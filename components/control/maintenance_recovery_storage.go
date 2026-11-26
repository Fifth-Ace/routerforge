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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// Only existing mounts. No credentials, mount/unmount, or fallback paths.
type recoveryMount struct {
	ID      string `json:"id"`
	Point   string `json:"point"`
	Kind    string `json:"kind"`
	FS      string `json:"fs"`
	mountID string
}

var recoveryFileRE = regexp.MustCompile(`^rfvault-[0-9a-f]{32}\.rfv$`)
var recoveryMountRE = regexp.MustCompile(`^[a-f0-9]{64}$`)
var recoveryStorageSlot = make(chan struct{}, 1)
var recoveryStorageRequestSlot = make(chan struct{}, 1)
var errRecoveryStorage = errors.New("RECOVERY_STORAGE_UNAVAILABLE_OR_UNSAFE")

func recoveryUnescapeMount(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}
func recoveryMounts(raw string) []recoveryMount {
	out := []recoveryMount{}
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) < 10 {
			continue
		}
		sep := -1
		for i := 6; i < len(f); i++ {
			if f[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+3 >= len(f) {
			continue
		}
		p := recoveryUnescapeMount(f[4])
		if filepath.Clean(p) != p || strings.ContainsAny(p, "\r\n\t\x00") || len(p) > 512 || (filepath.Dir(p) != "/opt/mnt" && filepath.Dir(p) != "/tmp/mnt") {
			continue
		}
		fs := f[sep+1]
		kind := ""
		switch fs {
		case "cifs", "smb3":
			kind = "smb"
		case "ext2", "ext3", "ext4", "vfat", "exfat", "fuseblk", "ntfs", "ntfs3", "f2fs", "btrfs", "xfs":
			kind = "usb"
		}
		if kind == "" {
			continue
		}
		if _, e := strconv.ParseUint(f[0], 10, 64); e != nil {
			continue
		}
		if seen[p] { // stacked mounts: retain none, fail closed
			for i := range out {
				if out[i].Point == p {
					out[i].ID = ""
				}
			}
			continue
		}
		seen[p] = true
		// identity binds mount ID, device number, root, mountpoint, fs, and source.
		// The source itself can contain private server/share data; never expose it.
		id := recoveryHash([]byte(strings.Join([]string{f[0], f[2], f[3], f[4], fs, f[sep+2]}, "\x00")))
		out = append(out, recoveryMount{ID: id, Point: p, Kind: kind, FS: fs, mountID: f[0]})
	}
	filtered := out[:0]
	for _, m := range out {
		if m.ID != "" {
			filtered = append(filtered, m)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Point < filtered[j].Point })
	return filtered
}
func recoveryReadMounts() ([]recoveryMount, error) {
	f, e := os.Open("/proc/self/mountinfo")
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		return nil, errRecoveryStorage
	}
	return recoveryMounts(string(b)), nil
}
func recoveryFDOnMount(f *os.File, m recoveryMount) bool {
	raw, e := os.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", f.Fd()))
	if e != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "mnt_id:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "mnt_id:")) == m.mountID
		}
	}
	return false
}
func recoveryOpenDir(path string, m recoveryMount) (*os.File, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	if !recoveryFDOnMount(f, m) {
		f.Close()
		return nil, errRecoveryStorage
	}
	return f, nil
}
func recoveryFDPath(f *os.File) string { return fmt.Sprintf("/proc/self/fd/%d", f.Fd()) }
func recoveryStorageDir(id string, create bool) (*os.File, recoveryMount, error) {
	mounts, e := recoveryReadMounts()
	if e != nil {
		return nil, recoveryMount{}, e
	}
	var m recoveryMount
	for _, x := range mounts {
		if x.ID == id {
			m = x
			break
		}
	}
	if m.ID == "" {
		return nil, m, errRecoveryStorage
	}
	root, e := recoveryOpenDir(m.Point, m)
	if e != nil {
		return nil, m, e
	}
	defer root.Close()
	p := filepath.Join(recoveryFDPath(root), "routerforge-recovery")
	if create {
		if e = os.Mkdir(p, 0700); e != nil && !errors.Is(e, os.ErrExist) {
			return nil, m, e
		}
	}
	dir, e := recoveryOpenDir(p, m)
	return dir, m, e
}
func recoveryReadExternal(dir *os.File, m recoveryMount, name string) ([]byte, error) {
	if !recoveryFileRE.MatchString(name) {
		return nil, errRecoveryStorage
	}
	p := filepath.Join(recoveryFDPath(dir), name)
	fd, e := syscall.Open(p, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), p)
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() <= 0 || st.Size() > configvault.ExternalLimit || !recoveryFDOnMount(f, m) {
		return nil, errRecoveryStorage
	}
	b, e := io.ReadAll(io.LimitReader(f, configvault.ExternalLimit+1))
	if e != nil || int64(len(b)) != st.Size() {
		return nil, errRecoveryStorage
	}
	return b, nil
}
func recoveryWriteExternal(dir *os.File, m recoveryMount, name string, data []byte) error {
	if !recoveryFileRE.MatchString(name) || len(data) == 0 || len(data) > configvault.ExternalLimit {
		return errRecoveryStorage
	}
	var fs syscall.Statfs_t
	if e := syscall.Statfs(recoveryFDPath(dir), &fs); e != nil {
		return e
	}
	if fs.Bsize <= 0 || uint64(fs.Bavail) < (uint64(len(data))+uint64(fs.Bsize)-1)/uint64(fs.Bsize) {
		return errors.New("external storage space limit")
	}
	p := filepath.Join(recoveryFDPath(dir), name)
	fd, e := syscall.Open(p, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if e != nil {
		return e
	}
	f := os.NewFile(uintptr(fd), p)
	if !recoveryFDOnMount(f, m) {
		f.Close()
		return errRecoveryStorage
	}
	n, e := f.Write(data)
	if e == nil && n != len(data) {
		e = io.ErrShortWrite
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = dir.Sync(); e != nil {
		return e
	}
	// No auto-delete or overwrite on uncertain write; the name is returned to UI.
	actual, e := recoveryReadExternal(dir, m, name)
	if e != nil || recoveryHash(actual) != recoveryHash(data) {
		return errRecoveryStorage
	}
	return nil
}

type recoveryStorageListing struct {
	Files      []string `json:"files"`
	FreeBytes  uint64   `json:"free_bytes"`
	TotalBytes uint64   `json:"total_bytes"`
	Limited    bool     `json:"limited"`
}

// Invoked only in a child process of Admin, before any service initialization.
// No shell and no key on argv/stdin; the child sees encrypted bytes only.
func recoveryStorageWorker(args []string, in io.Reader, out io.Writer) error {
	if len(args) != 3 || !recoveryMountRE.MatchString(args[1]) {
		return errRecoveryStorage
	}
	op, id, name := args[0], args[1], args[2]
	if op != "put" && op != "get" && op != "list" {
		return errRecoveryStorage
	}
	if op == "list" && name != "-" || op != "list" && !recoveryFileRE.MatchString(name) {
		return errRecoveryStorage
	}
	dir, m, e := recoveryStorageDir(id, op == "put")
	if e != nil {
		if op == "list" && errors.Is(e, os.ErrNotExist) { // only if mount itself is still valid; no fallback
			mounts, me := recoveryReadMounts()
			if me != nil {
				return me
			}
			for _, x := range mounts {
				if x.ID == id {
					root, re := recoveryOpenDir(x.Point, x)
					if re != nil {
						return re
					}
					defer root.Close()
					var fs syscall.Statfs_t
					if re = syscall.Statfs(recoveryFDPath(root), &fs); re != nil {
						return re
					}
					return json.NewEncoder(out).Encode(recoveryStorageListing{Files: []string{}, FreeBytes: uint64(fs.Bavail) * uint64(fs.Bsize), TotalBytes: uint64(fs.Blocks) * uint64(fs.Bsize)})
				}
			}
		}
		return e
	}
	defer dir.Close()
	switch op {
	case "put":
		b, e := io.ReadAll(io.LimitReader(in, configvault.ExternalLimit+1))
		if e != nil || len(b) > configvault.ExternalLimit {
			return errRecoveryStorage
		}
		return recoveryWriteExternal(dir, m, name, b)
	case "get":
		b, e := recoveryReadExternal(dir, m, name)
		if e != nil {
			return e
		}
		_, e = out.Write(b)
		return e
	default:
		var fs syscall.Statfs_t
		if e = syscall.Statfs(recoveryFDPath(dir), &fs); e != nil {
			return e
		}
		r := recoveryStorageListing{Files: []string{}, FreeBytes: uint64(fs.Bavail) * uint64(fs.Bsize), TotalBytes: uint64(fs.Blocks) * uint64(fs.Bsize)}
		entries, e := dir.ReadDir(257)
		if e != nil && e != io.EOF {
			return e
		}
		r.Limited = len(entries) > 256
		if r.Limited {
			entries = entries[:256]
		}
		for _, v := range entries {
			if recoveryFileRE.MatchString(v.Name()) && v.Type().IsRegular() {
				r.Files = append(r.Files, v.Name())
			}
		}
		sort.Strings(r.Files)
		return json.NewEncoder(out).Encode(r)
	}
}

type recoveryLimitedOutput struct {
	b   bytes.Buffer
	max int
}

func (w *recoveryLimitedOutput) Write(p []byte) (int, error) {
	if len(p) > w.max-w.b.Len() {
		return 0, errRecoveryStorage
	}
	return w.b.Write(p)
}
func recoveryStorageCall(ctx context.Context, op, id, name string, data []byte) ([]byte, error) {
	select {
	case recoveryStorageSlot <- struct{}{}:
	default:
		return nil, errors.New("RECOVERY_STORAGE_BUSY")
	}
	executable, e := os.Executable()
	if e != nil {
		<-recoveryStorageSlot
		return nil, e
	}
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	type reply struct {
		data []byte
		err  error
	}
	done := make(chan reply, 1)
	go func() {
		defer func() { <-recoveryStorageSlot }()
		cmd, e := safety.CommandContext(ctx, executable, "--recovery-storage-worker", op, id, name)
		if e != nil {
			done <- reply{nil, e}
			return
		}
		cmd.WaitDelay = 2 * time.Second
		cmd.Stdin = bytes.NewReader(data)
		output := &recoveryLimitedOutput{max: configvault.ExternalLimit}
		cmd.Stdout = output
		cmd.Stderr = io.Discard
		e = cmd.Run()
		done <- reply{output.b.Bytes(), e}
	}()
	select {
	case x := <-done:
		return x.data, x.err
	case <-ctx.Done():
		return nil, errors.New("RECOVERY_STORAGE_TIMEOUT_UNCERTAIN")
	}
}

type recoveryStorageRequest struct {
	MountID    string `json:"mount_id"`
	SnapshotID string `json:"snapshot_id,omitempty"`
	File       string `json:"file,omitempty"`
	Key        string `json:"key,omitempty"`
	Confirm    string `json:"confirm"`
}

func registerRecoveryStorageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/recovery-storage", getOnly(func(w http.ResponseWriter, r *http.Request) {
		m, e := recoveryReadMounts()
		if e != nil {
			writeJSON(w, 503, map[string]string{"error": "MOUNT_TABLE_UNAVAILABLE"})
			return
		}
		writeJSON(w, 200, map[string]any{"mounts": m})
	}))
	for _, op := range []string{"list", "export", "import"} {
		op := op
		mux.HandleFunc("/v1/maintenance/recovery-storage-"+op, mutationOnly(func(w http.ResponseWriter, r *http.Request) {
			var q recoveryStorageRequest
			if decodeMutationJSON(w, r, &q) != nil || !recoveryMountRE.MatchString(q.MountID) || q.Confirm != "RECOVERY_STORAGE_"+strings.ToUpper(op) {
				writeJSON(w, 400, map[string]string{"error": "INVALID_REQUEST"})
				return
			}
			select {
			case recoveryStorageRequestSlot <- struct{}{}:
				defer func() { <-recoveryStorageRequestSlot }()
			default:
				writeJSON(w, 409, map[string]string{"error": "RECOVERY_STORAGE_BUSY"})
				return
			}
			fail := func(status int, code, file string) {
				log.Printf("recovery-storage operation=%s state=%s file=%q", op, code, file)
				writeJSON(w, status, map[string]string{"error": code, "file": file})
			}
			if op == "list" {
				if q.Key != "" || q.File != "" || q.SnapshotID != "" {
					fail(400, "INVALID_REQUEST", "")
					return
				}
				b, e := recoveryStorageCall(r.Context(), "list", q.MountID, "-", nil)
				if e != nil {
					fail(503, "STORAGE_UNAVAILABLE_OR_BUSY", "")
					return
				}
				var listing recoveryStorageListing
				if json.Unmarshal(b, &listing) != nil {
					fail(503, "INVALID_STORAGE_RESPONSE", "")
					return
				}
				writeJSON(w, 200, listing)
				return
			}
			if len(q.Key) != 64 {
				fail(400, "INVALID_ENCRYPTION_KEY", "")
				return
			}
			if _, e := hex.DecodeString(q.Key); e != nil {
				fail(400, "INVALID_ENCRYPTION_KEY", "")
				return
			}
			if op == "export" {
				if q.File != "" || q.SnapshotID == "" || len(q.SnapshotID) > 80 {
					fail(400, "INVALID_REQUEST", "")
					return
				}
				unlock, _ := lockAdminConfigVaultMutation(adminConfigVaultManagedRoot)
				v, e := newAdminConfigVault()
				var data []byte
				if e == nil {
					data, e = v.ExportEncrypted(q.SnapshotID, q.Key)
				}
				unlock()
				if e != nil {
					fail(409, "SNAPSHOT_EXPORT_BLOCKED", "")
					return
				}
				random := make([]byte, 16)
				if _, e = rand.Read(random); e != nil {
					fail(500, "RANDOM_UNAVAILABLE", "")
					return
				}
				name := "rfvault-" + hex.EncodeToString(random) + ".rfv"
				if _, e = recoveryStorageCall(r.Context(), "put", q.MountID, name, data); e != nil {
					fail(503, "EXPORT_UNCERTAIN_INSPECT_STORAGE_DO_NOT_RETRY", name)
					return
				}
				log.Printf("recovery-storage operation=export state=verified snapshot=%q file=%q", q.SnapshotID, name)
				writeJSON(w, 200, map[string]any{"ok": true, "file": name, "sha256": recoveryHash(data), "readback_verified": true})
				return
			}
			if q.SnapshotID != "" || !recoveryFileRE.MatchString(q.File) {
				fail(400, "INVALID_REQUEST", "")
				return
			}
			data, e := recoveryStorageCall(r.Context(), "get", q.MountID, q.File, nil)
			if e != nil {
				fail(503, "STORAGE_READ_FAILED", q.File)
				return
			}
			unlock, _ := lockAdminConfigVaultMutation(adminConfigVaultManagedRoot)
			defer unlock()
			v, e := newAdminConfigVault()
			if e != nil {
				fail(503, "VAULT_UNAVAILABLE", q.File)
				return
			}
			m, e := v.ImportEncrypted(data, q.Key)
			if e != nil {
				fail(409, "IMPORT_BLOCKED_KEY_INTEGRITY_OR_CAPACITY", q.File)
				return
			}
			log.Printf("recovery-storage operation=import state=verified snapshot=%q file=%q", m.ID, q.File)
			writeJSON(w, 200, map[string]any{"ok": true, "snapshot": m, "runtime": "NOT_APPLIED"})
		}))
	}
}
