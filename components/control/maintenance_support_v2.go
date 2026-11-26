package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	sb "github.com/Fifth-Ace/routerforge/internal/platform/supportbundle"
	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const supportPreviewTTL = 2 * time.Minute

var supportWorkerSlot = make(chan struct{}, 1)
var supportMu sync.Mutex
var supportPending *supportPrepared
var supportNameRE = regexp.MustCompile(`^routerforge-support-(v2-)?[0-9]{1,20}\.zip$`)
var supportTokenRE = regexp.MustCompile(`^[a-f0-9]{48}$`)
var supportErrBusy = errors.New("SUPPORT_BUSY")
var supportErrFull = errors.New("SUPPORT_RETENTION_FULL")
var supportErrExpired = errors.New("SUPPORT_PREVIEW_EXPIRED")

type supportSelectionRequest struct {
	Sections []string `json:"sections"`
	Modules  []string `json:"modules"`
	Confirm  string   `json:"confirm"`
}
type supportPrepared struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	Manifest  sb.Manifest `json:"manifest"`
	Size      int         `json:"size"`
	SHA256    string      `json:"sha256"`
	raw       []byte
}
type supportDeleteRequest struct {
	Name    string `json:"name"`
	Confirm string `json:"confirm"`
}

type supportLimitedOutput struct {
	buf   bytes.Buffer
	limit int
}

func (w *supportLimitedOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buf.Len() {
		return 0, sb.ErrInvalid
	}
	return w.buf.Write(p)
}

func supportCollect(ctx context.Context, q supportWorkerRequest) ([]sb.Entry, error) {
	select {
	case supportWorkerSlot <- struct{}{}:
	default:
		return nil, supportErrBusy
	}
	raw, e := json.Marshal(q)
	if e != nil {
		<-supportWorkerSlot
		return nil, e
	}
	exe, e := os.Executable()
	if e != nil {
		<-supportWorkerSlot
		return nil, e
	}
	workerCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	cmd, e := safety.CommandContext(workerCtx, exe, "--support-collect-worker")
	if e != nil {
		<-supportWorkerSlot
		return nil, e
	}
	cmd.Stdin = bytes.NewReader(raw)
	out := &supportLimitedOutput{limit: sb.MaxArchive}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	done := make(chan error, 1)
	go func() { e := cmd.Run(); <-supportWorkerSlot; done <- e }()
	// If a kernel I/O call ignores cancellation, the slot stays occupied until
	// the child exits; further previews fail fast rather than accumulate workers.
	select {
	case e = <-done:
		if e != nil {
			return nil, errors.New("SUPPORT_COLLECT_FAILED")
		}
	case <-workerCtx.Done():
		return nil, errors.New("SUPPORT_COLLECT_TIMEOUT")
	}
	var entries []sb.Entry
	if json.Unmarshal(out.buf.Bytes(), &entries) != nil {
		return nil, sb.ErrInvalid
	}
	return entries, nil
}
func supportMakePrepared(now time.Time, sel, mods []string, entries []sb.Entry) (*supportPrepared, error) {
	raw, m, e := sb.Build(now, sel, mods, entries)
	if e != nil {
		return nil, e
	}
	if e = sb.Verify(raw, m); e != nil {
		return nil, e
	}
	var token [24]byte
	if _, e = rand.Read(token[:]); e != nil {
		return nil, e
	}
	return &supportPrepared{hex.EncodeToString(token[:]), now.Add(supportPreviewTTL), m, len(raw), sb.Hash(raw), raw}, nil
}
func supportPrepare(ctx context.Context, sel, mods []string, obs sb.Observation) (*supportPrepared, error) {
	entries, e := supportCollect(ctx, supportWorkerRequest{sel, mods, obs})
	if e != nil {
		return nil, e
	}
	return supportMakePrepared(time.Now().UTC(), sel, mods, entries)
}
func supportTakePreview(now time.Time, p *supportPrepared, token string) (*supportPrepared, error) {
	if p == nil || !supportTokenRE.MatchString(token) || token != p.Token || !now.Before(p.ExpiresAt) {
		return nil, supportErrExpired
	}
	return p, nil
}

// Same existing local directory. Do not follow a replaced root or permit an
// accessible directory. Only an explicit delete can release retention space.
func supportRootCheck(root string, create bool) error {
	st, e := os.Lstat(root)
	if errors.Is(e, os.ErrNotExist) && create {
		if e = os.Mkdir(root, 0700); e != nil && !errors.Is(e, os.ErrExist) {
			return e
		}
		st, e = os.Lstat(root)
	}
	if e != nil {
		return e
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || sys.Uid != uint32(os.Geteuid()) {
		return sb.ErrInvalid
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 {
		return sb.ErrInvalid
	}
	return nil
}
func supportOpen(root, name string) (*os.File, error) {
	if !supportNameRE.MatchString(name) || supportRootCheck(root, false) != nil {
		return nil, sb.ErrInvalid
	}
	path := filepath.Join(root, name)
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Size() < 1 || st.Size() > adminSupportBundleMaxBytes {
		f.Close()
		return nil, sb.ErrInvalid
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || sys.Uid != uint32(os.Geteuid()) {
		f.Close()
		return nil, sb.ErrInvalid
	}
	return f, nil
}
func supportInventory(root string) ([]adminSupportBundleInfo, error) {
	if e := supportRootCheck(root, false); e != nil {
		if errors.Is(e, os.ErrNotExist) {
			return []adminSupportBundleInfo{}, nil
		}
		return nil, e
	}
	dir, e := os.Open(root)
	if e != nil {
		return nil, e
	}
	defer dir.Close()
	entries, e := dir.ReadDir(256)
	if e != nil && e != io.EOF {
		return nil, e
	}
	if len(entries) == 256 {
		return nil, sb.ErrInvalid
	}
	rows := []adminSupportBundleInfo{}
	for _, entry := range entries {
		if !supportNameRE.MatchString(entry.Name()) {
			continue
		}
		f, e := supportOpen(root, entry.Name())
		if e != nil {
			continue
		}
		st, e := f.Stat()
		f.Close()
		if e != nil {
			continue
		}
		rows = append(rows, adminSupportBundleInfo{Path: filepath.Join(root, entry.Name()), Name: entry.Name(), Size: st.Size(), ModifiedAt: st.ModTime().UTC()})
	}
	return rows, nil
}
func supportPublish(root string, p *supportPrepared, now time.Time) (adminSupportBundleInfo, error) {
	var info adminSupportBundleInfo
	if p == nil || p.SHA256 != sb.Hash(p.raw) || len(p.raw) != p.Size || sb.Verify(p.raw, p.Manifest) != nil {
		return info, sb.ErrInvalid
	}
	if e := supportRootCheck(root, true); e != nil {
		return info, e
	}
	rows, e := supportInventory(root)
	if e != nil {
		return info, e
	}
	if len(rows) >= adminSupportBundleMaxFiles {
		return info, supportErrFull
	}
	name := fmt.Sprintf("routerforge-support-v2-%d.zip", now.UnixNano())
	path := filepath.Join(root, name)
	// Write an invisible staging file first; hard-link publish is exclusive and
	// never replaces an existing archive. /tmp may not support hardlinks: fail closed.
	staged, e := safety.NewAtomicFile(root, ".support-preview-*")
	if e != nil {
		return info, e
	}
	defer staged.Cleanup()
	f := staged.File()
	if e = f.Chmod(0600); e != nil {
		return info, e
	}
	if _, e = f.Write(p.raw); e != nil {
		return info, e
	}
	if e = staged.Sync(); e != nil {
		return info, e
	}
	if e = staged.Close(); e != nil {
		return info, e
	}
	if e = os.Link(staged.Path(), path); e != nil {
		return info, e
	}
	dir, e := os.Open(root)
	if e != nil {
		return info, errors.New("SUPPORT_WRITE_UNCERTAIN")
	}
	e = dir.Sync()
	ce := dir.Close()
	if e != nil || ce != nil {
		return info, errors.New("SUPPORT_WRITE_UNCERTAIN")
	}
	readback, e := supportOpen(root, name)
	if e != nil {
		return info, e
	}
	defer readback.Close()
	b, e := io.ReadAll(io.LimitReader(readback, sb.MaxArchive+1))
	if e != nil || sb.Hash(b) != p.SHA256 {
		return info, errors.New("SUPPORT_WRITE_UNCERTAIN")
	}
	st, e := readback.Stat()
	if e != nil {
		return info, e
	}
	return adminSupportBundleInfo{Path: path, Name: name, Size: st.Size(), ModifiedAt: st.ModTime().UTC()}, nil
}
func supportFailure(w http.ResponseWriter, e error) {
	status := http.StatusConflict
	switch e {
	case supportErrBusy:
		status = http.StatusTooManyRequests
	}
	code := "SUPPORT_OPERATION_FAILED"
	if e == supportErrBusy || e == supportErrFull || e == supportErrExpired {
		code = e.Error()
	} else if e != nil && (e.Error() == "SUPPORT_COLLECT_FAILED" || e.Error() == "SUPPORT_COLLECT_TIMEOUT" || e.Error() == "SUPPORT_WRITE_UNCERTAIN") {
		code = e.Error()
	}
	writeJSON(w, status, map[string]string{"error": code})
}
func registerSupportV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/support-preview", mutationOnly(func(w http.ResponseWriter, r *http.Request) {
		var q supportSelectionRequest
		if decodeMutationJSON(w, r, &q) != nil || q.Confirm != "SUPPORT_PREVIEW" {
			writeJSON(w, 400, map[string]string{"error": "INVALID_PREVIEW"})
			return
		}
		sel, e := sb.Selection(q.Sections)
		if e != nil {
			writeJSON(w, 400, map[string]string{"error": "INVALID_SELECTION"})
			return
		}
		mods, e := sb.SelectedModules(sel, q.Modules)
		if e != nil {
			writeJSON(w, 400, map[string]string{"error": "INVALID_MODULES"})
			return
		}
		if !supportMu.TryLock() {
			supportFailure(w, supportErrBusy)
			return
		}
		defer supportMu.Unlock()
		supportPending = nil
		p, e := supportPrepare(r.Context(), sel, mods, sb.ParseObservation(r.Header.Get(sb.Header)))
		if e != nil {
			supportFailure(w, e)
			return
		}
		supportPending = p
		writeJSON(w, 200, p)
	}))
	mux.HandleFunc("/v1/maintenance/support-delete", mutationOnly(func(w http.ResponseWriter, r *http.Request) {
		var q supportDeleteRequest
		if decodeMutationJSON(w, r, &q) != nil || q.Confirm != "DELETE_SUPPORT_BUNDLE" || !supportNameRE.MatchString(q.Name) {
			writeJSON(w, 400, map[string]string{"error": "INVALID_CONFIRMATION"})
			return
		}
		if !supportMu.TryLock() {
			supportFailure(w, supportErrBusy)
			return
		}
		defer supportMu.Unlock()
		f, e := supportOpen(adminSupportBundleRoot, q.Name)
		if e != nil {
			supportFailure(w, e)
			return
		}
		f.Close()
		if e = os.Remove(filepath.Join(adminSupportBundleRoot, q.Name)); e != nil {
			supportFailure(w, e)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
}
func handleSupportV2Create(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Confirm      string `json:"confirm"`
		PreviewToken string `json:"preview_token,omitempty"`
	}
	if decodeMutationJSON(w, r, &q) != nil || q.Confirm != "SUPPORT_BUNDLE" {
		writeJSON(w, 400, map[string]string{"error": "INVALID_CONFIRMATION"})
		return
	}
	if !supportMu.TryLock() {
		supportFailure(w, supportErrBusy)
		return
	}
	defer supportMu.Unlock()
	if q.PreviewToken == "" {
		writeJSON(w, 409, map[string]string{"error": "SUPPORT_PREVIEW_REQUIRED"})
		return
	}
	p, e := supportTakePreview(time.Now(), supportPending, q.PreviewToken)
	if e != nil {
		supportFailure(w, e)
		return
	}
	supportPending = nil // single use, never retry an uncertain write
	bundle, e := supportPublish(adminSupportBundleRoot, p, time.Now().UTC())
	if e != nil {
		supportFailure(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "bundle": bundle, "schema_version": 2, "sha256": p.SHA256, "redacted": true})
}
