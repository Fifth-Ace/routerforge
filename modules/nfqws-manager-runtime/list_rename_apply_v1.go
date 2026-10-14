package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// NF5G2 is a bounded best-effort multi-file transaction. Only the confirmed
// exact NF5G1 plan may be applied. The original list is retained until all
// configuration writes and the optional service restart have succeeded.
// The transaction cannot be made crash-atomic across multiple files.
const nf5g2MaxConfigChanges = 6 // source list + up to six configs, safety backup budget <= 8

var nf5g2RenameMu sync.Mutex

type nf5g2Request struct {
	OldName         string             `json:"old_name"`
	NewName         string             `json:"new_name"`
	ExpectedSHA256  string             `json:"expected_sha256"`
	ExpectedChanges []nf5gConfigImpact `json:"expected_config_changes"`
	Confirm         string             `json:"confirm"`
}

type nf5g2Result struct {
	OK               bool     `json:"ok"`
	OldName          string   `json:"old_name"`
	NewName          string   `json:"new_name"`
	ChangedConfigs   int      `json:"changed_configs"`
	UpdatedSelectors int      `json:"updated_selectors"`
	ServiceRestarted bool     `json:"service_restarted"`
	RolledBack       bool     `json:"rolled_back"`
	RollbackError    string   `json:"rollback_error,omitempty"`
	Backups          []string `json:"backups"`
}

type nf5g2Ops struct {
	Backup      func(label string, data []byte, mode os.FileMode) (string, error)
	WriteConfig func(path string, data []byte, mode os.FileMode) error
	Restart     func() error
	Running     bool
}

type nf5g2ConfigSnap struct {
	path     string
	original []byte
	updated  []byte
	info     os.FileInfo
	isActive bool
	modified bool
}

func nf5g2EqualImpacts(a, b []nf5gConfigImpact) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]nf5gConfigImpact(nil), a...)
	bb := append([]nf5gConfigImpact(nil), b...)
	sort.Slice(aa, func(i, j int) bool { return aa[i].Reference < aa[j].Reference })
	sort.Slice(bb, func(i, j int) bool { return bb[i].Reference < bb[j].Reference })
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func nf5g2ExclusiveCopy(path string, payload []byte, mode os.FileMode) (os.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return nil, err
	}
	info, statErr := f.Stat()
	complete := false
	defer func() {
		if complete || statErr != nil {
			return
		}
		if current, err := os.Lstat(path); err == nil && os.SameFile(info, current) {
			_ = os.Remove(path)
		}
	}()
	n, writeErr := f.Write(payload)
	if writeErr == nil && n != len(payload) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if statErr != nil {
		return nil, statErr
	}
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	complete = true
	return info, nil
}

// Re-read the exact plan and every byte immediately before the first mutation.
// Every reference must remain both path-exact and digest-exact.
func nf5g2MakeSnapshots(listRoot, activePath, libRoot string, req nf5g2Request) ([]nf5g2ConfigSnap, nf5gRenamePreview, []byte, os.FileInfo, error) {
	preview, err := nf5gPreviewRenameInRoots(listRoot, activePath, libRoot, req.OldName, req.NewName, req.ExpectedSHA256)
	if err != nil {
		return nil, preview, nil, nil, err
	}
	if !preview.OK || !nf5g2EqualImpacts(req.ExpectedChanges, preview.ConfigChanges) {
		return nil, preview, nil, nil, errors.New("rename plan or config SHA changed since Preview; refresh Preview")
	}
	if len(preview.ConfigChanges) > nf5g2MaxConfigChanges {
		return nil, preview, nil, nil, fmt.Errorf("rename touches %d configs; maximum %d per transaction", len(preview.ConfigChanges), nf5g2MaxConfigChanges)
	}
	listPath := filepath.Join(listRoot, req.OldName)
	listData, listInfo, err := nf5aReadRegularList(listPath)
	if err != nil {
		return nil, preview, nil, nil, err
	}
	if err = nf5aRequireListPreimage(listData, req.ExpectedSHA256); err != nil {
		return nil, preview, nil, nil, err
	}
	snaps := make([]nf5g2ConfigSnap, 0, len(preview.ConfigChanges))
	for _, impact := range preview.ConfigChanges {
		p := activePath
		active := impact.Reference == "active:nfqws2.conf"
		if !active {
			if !strings.HasPrefix(impact.Reference, "library:") {
				return nil, preview, nil, nil, errors.New("unknown config reference")
			}
			name := strings.TrimPrefix(impact.Reference, "library:")
			if !safeConfigLibraryName(name) {
				return nil, preview, nil, nil, errors.New("invalid saved config identity")
			}
			p = filepath.Join(libRoot, name)
		}
		data, err := nf5bReadReferenceConfig(p)
		if err != nil {
			return nil, preview, nil, nil, err
		}
		updated, count, err := nf5gPlanConfigRename(data, listRoot, req.OldName, req.NewName)
		if err != nil {
			return nil, preview, nil, nil, err
		}
		if count != impact.Selectors || listSourceSHA256(data) != impact.BeforeSHA256 || listSourceSHA256(updated) != impact.AfterSHA256 {
			return nil, preview, nil, nil, errors.New("config plan identity changed: " + impact.Reference)
		}
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, preview, nil, nil, errors.New("config file identity changed: " + impact.Reference)
		}
		snaps = append(snaps, nf5g2ConfigSnap{path: p, original: data, updated: updated, info: info, isActive: active})
	}
	sort.Slice(snaps, func(i, j int) bool {
		if snaps[i].isActive != snaps[j].isActive {
			return !snaps[i].isActive
		}
		return snaps[i].path < snaps[j].path
	})
	return snaps, preview, listData, listInfo, nil
}

// Only restore bytes written by THIS transaction. If an outside actor changed
// a file after our write, report a rollback conflict instead of clobbering it.
func nf5g2RestoreConfigs(snaps []nf5g2ConfigSnap, write func(string, []byte, os.FileMode) error) error {
	problems := []string{}
	for i := len(snaps) - 1; i >= 0; i-- {
		s := snaps[i]
		if !s.modified {
			continue
		}
		current, info, err := nf5aReadRegularList(s.path) // bounded: configs are smaller than list max
		if err != nil || !os.SameFile(s.info, info) {     // atomic rename changes inode: compare the exact written bytes below instead
			// Inode replacement is expected after WriteFileAtomic; only reject invalid/missing reads.
			if err != nil {
				problems = append(problems, "read rollback "+s.path+": "+err.Error())
				continue
			}
		}
		if listSourceSHA256(current) == listSourceSHA256(s.original) {
			continue
		}
		if listSourceSHA256(current) != listSourceSHA256(s.updated) {
			problems = append(problems, "external config modification: "+s.path)
			continue
		}
		if err := write(s.path, s.original, s.info.Mode().Perm()); err != nil {
			problems = append(problems, "restore "+s.path+": "+err.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func nf5g2Execute(listRoot, activePath, libRoot string, req nf5g2Request, ops nf5g2Ops) (nf5g2Result, error) {
	out := nf5g2Result{OldName: req.OldName, NewName: req.NewName, Backups: []string{}}
	if req.Confirm != "ROUTERFORGE_LIST_RENAME_APPLY" {
		return out, errors.New("explicit rename confirmation is required")
	}
	if ops.Backup == nil || ops.WriteConfig == nil {
		return out, errors.New("transaction services unavailable")
	}
	snaps, plan, oldBytes, oldInfo, err := nf5g2MakeSnapshots(listRoot, activePath, libRoot, req)
	if err != nil {
		return out, err
	}
	oldPath, newPath := filepath.Join(listRoot, req.OldName), filepath.Join(listRoot, req.NewName)
	activeChanged := false
	for _, s := range snaps {
		if s.isActive {
			activeChanged = true
		}
	}
	if activeChanged && ops.Running && ops.Restart == nil {
		return out, errors.New("restart required but unavailable")
	}
	for _, s := range snaps {
		// Take all backups BEFORE a single mutation. Never prune before success.
		b, err := ops.Backup("list-rename-config-"+filepath.Base(s.path), s.original, s.info.Mode().Perm())
		if err != nil {
			return out, fmt.Errorf("backup config: %w", err)
		}
		out.Backups = append(out.Backups, b)
	}
	b, err := ops.Backup("list-rename-source-"+req.OldName, oldBytes, oldInfo.Mode().Perm())
	if err != nil {
		return out, fmt.Errorf("backup source list: %w", err)
	}
	out.Backups = append(out.Backups, b)
	// Revalidate all preimages after backups, before any mutation.
	if _, _, _, _, err = nf5g2MakeSnapshots(listRoot, activePath, libRoot, req); err != nil {
		return out, fmt.Errorf("changed after backup: %w", err)
	}
	newInfo, err := nf5g2ExclusiveCopy(newPath, oldBytes, oldInfo.Mode().Perm())
	if err != nil {
		return out, fmt.Errorf("exclusive target list creation: %w", err)
	}
	// Fail closed when any write, restart or source-removal precheck fails.
	rollback := func(cause error) (nf5g2Result, error) {
		issues := []string{}
		if e := nf5g2RestoreConfigs(snaps, ops.WriteConfig); e != nil {
			issues = append(issues, e.Error())
		}
		restartNeeded := false
		for _, snap := range snaps {
			if snap.isActive && snap.modified {
				restartNeeded = true
			}
		}
		if restartNeeded && ops.Running && ops.Restart != nil {
			if e := ops.Restart(); e != nil {
				issues = append(issues, "restore previous runtime: "+e.Error())
			}
		}
		current, statErr := os.Lstat(newPath)
		if statErr == nil && os.SameFile(current, newInfo) {
			fresh, e := os.ReadFile(newPath)
			if e != nil || listSourceSHA256(fresh) != listSourceSHA256(oldBytes) {
				issues = append(issues, "new list changed externally; not removing")
			} else if e := os.Remove(newPath); e != nil {
				issues = append(issues, "remove created list: "+e.Error())
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			issues = append(issues, "created list identity changed; not removing")
		}
		out.RolledBack = len(issues) == 0
		if len(issues) > 0 {
			out.RollbackError = strings.Join(issues, "; ")
		}
		return out, fmt.Errorf("rename failed: %w; rollback_ok=%t; rollback_error=%s", cause, out.RolledBack, out.RollbackError)
	}
	for i := range snaps {
		s := &snaps[i]
		before, e := nf5bReadReferenceConfig(s.path)
		var info os.FileInfo
		if e == nil {
			info, e = os.Lstat(s.path)
		}
		if e != nil || !os.SameFile(info, s.info) || listSourceSHA256(before) != listSourceSHA256(s.original) {
			return rollback(errors.New("config changed before write: " + s.path))
		}
		s.modified = true // also inspect/restore when an atomic write reports an error
		if e := ops.WriteConfig(s.path, s.updated, s.info.Mode().Perm()); e != nil {
			return rollback(fmt.Errorf("write %s: %w", s.path, e))
		}
	}
	if activeChanged && ops.Running {
		if err := ops.Restart(); err != nil {
			return rollback(fmt.Errorf("restart new config: %w", err))
		}
		out.ServiceRestarted = true
	}
	source, info, e := nf5aReadRegularList(oldPath)
	if e != nil || !os.SameFile(info, oldInfo) || listSourceSHA256(source) != listSourceSHA256(oldBytes) {
		return rollback(errors.New("source list changed before final removal"))
	}
	dest, info, e := nf5aReadRegularList(newPath)
	if e != nil || !os.SameFile(info, newInfo) || listSourceSHA256(dest) != listSourceSHA256(oldBytes) {
		return rollback(errors.New("new list changed before final removal"))
	}
	if err := os.Remove(oldPath); err != nil {
		return rollback(fmt.Errorf("remove old list: %w", err))
	}
	out.OK = true
	out.ChangedConfigs = len(snaps)
	out.UpdatedSelectors = plan.ReferenceCount
	return out, nil
}

func registerNF5G2RenameApplyRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/list/rename-apply", mutationOnly(handleNF5G2RenameApply))
}

func handleNF5G2RenameApply(w http.ResponseWriter, r *http.Request) {
	var request nf5g2Request
	if err := decodeSmartApplyJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid rename apply request"})
		return
	}
	nf5g2RenameMu.Lock()
	defer nf5g2RenameMu.Unlock()
	status := readStatus()
	if !status.MutationReady || !status.ConfigExists || status.ConfigCut {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 runtime not ready for rename"})
		return
	}
	restart := func() error {
		output, err := runInit("restart")
		if err != nil {
			return fmt.Errorf("nfqws2 restart: %v: %s", err, output)
		}
		next := readStatus()
		if !next.Running {
			return errors.New("nfqws2 is not running after restart")
		}
		return nil
	}
	result, err := nf5g2Execute(listsRoot, configPath, configLibraryRoot, request, nf5g2Ops{
		Backup: createNamedBackup,
		WriteConfig: func(path string, data []byte, mode os.FileMode) error {
			return safety.WriteFileAtomic(path, data, mode)
		},
		Restart: restart,
		Running: status.Running,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "rolled_back": result.RolledBack, "rollback_error": result.RollbackError, "backups": result.Backups})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "old_name": request.OldName, "new_name": request.NewName, "changed_configs": result.ChangedConfigs, "updated_selectors": result.UpdatedSelectors, "service_restarted": result.ServiceRestarted, "rolled_back": false, "backups": result.Backups, "status": readStatus()})
}
