package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// All file names originate from an existing manifest; no client paths or raw
// contents. Only a narrowly validated standalone setting is writable in E06.1.
const recoveryLimit = 2 << 20

var errRecoveryConflict = errors.New("RECOVERY_CONFLICT_OR_UNSUPPORTED")

type recoveryRequest struct {
	SnapshotID string `json:"snapshot_id"`
	ArtifactID string `json:"artifact_id"`
	Token      string `json:"token,omitempty"`
	Confirm    string `json:"confirm,omitempty"`
}
type recoveryItem struct {
	ID             string `json:"id"`
	Path           string `json:"path"`
	SnapshotSHA    string `json:"snapshot_sha256"`
	CurrentSHA     string `json:"current_sha256,omitempty"`
	SnapshotMode   uint32 `json:"snapshot_mode"`
	CurrentMode    uint32 `json:"current_mode,omitempty"`
	Size           int64  `json:"size"`
	Status         string `json:"status"`
	Integrity      string `json:"integrity"`
	RestoreAllowed bool   `json:"restore_allowed"`
	Reason         string `json:"reason"`
}
type recoveryPlan struct {
	SnapshotID string                 `json:"snapshot_id"`
	Item       recoveryItem           `json:"item"`
	Token      string                 `json:"token"`
	Runtime    string                 `json:"runtime"`
	Before     *recoveryLoggingValues `json:"before,omitempty"`
	After      *recoveryLoggingValues `json:"after,omitempty"`
}
type recoveryLoggingValues struct {
	MaxMB   int `json:"max_mb"`
	Backups int `json:"backups"`
}
type recoveryVault interface {
	Get(string) (configvault.Manifest, error)
	ReadArtifact(string, string) (configvault.ArtifactRecord, []byte, error)
	Capture(configvault.CaptureRequest) (configvault.Manifest, error)
}

func recoveryHash(data []byte) string { d := sha256.Sum256(data); return hex.EncodeToString(d[:]) }
func recoveryRead(root, path string) ([]byte, uint32, error) {
	clean, e := validateAdminConfigVaultRestorePath(root, path)
	if e != nil || clean != path {
		return nil, 0, errRecoveryConflict
	}
	// Resolve the parent even for an absent file; reject symlink traversal.
	parent := filepath.Dir(path)
	resolved, e := filepath.EvalSymlinks(parent)
	if e != nil || resolved != parent {
		return nil, 0, errRecoveryConflict
	}
	info, e := os.Lstat(path)
	if e != nil {
		return nil, 0, e
	}
	if !info.Mode().IsRegular() || info.Size() > recoveryLimit {
		return nil, 0, errRecoveryConflict
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, 0, e
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(info, actual) || !actual.Mode().IsRegular() {
		return nil, 0, errRecoveryConflict
	}
	data, e := io.ReadAll(io.LimitReader(f, recoveryLimit+1))
	if e != nil || len(data) > recoveryLimit {
		return nil, 0, errRecoveryConflict
	}
	latest, e := os.Lstat(path)
	if e != nil || !os.SameFile(actual, latest) || latest.Size() != int64(len(data)) || !actual.ModTime().Equal(latest.ModTime()) || actual.Mode() != latest.Mode() {
		return nil, 0, errRecoveryConflict
	}
	return data, uint32(actual.Mode().Perm()), nil
}

// Do not restore security, service intent, module config, arbitrary paths,
// or coupled Foundation state. The safe first adapter is standard Entware
// logging settings; no custom path or RAM mode that requires live validation.
func recoveryAllowed(root, path string, data []byte, mode uint32) bool {
	if path != filepath.Join(root, "logging.json") || mode != 0600 || len(data) > 4096 {
		return false
	}
	var v struct {
		Mode       string `json:"mode"`
		CustomPath string `json:"custom_path,omitempty"`
		MaxMB      int    `json:"max_mb"`
		Backups    int    `json:"backups"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return false
	}
	// Reject duplicate top-level keys (json.Unmarshal alone accepts duplicates).
	d = json.NewDecoder(bytes.NewReader(data))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for d.More() {
		k, e := d.Token()
		key, ok := k.(string)
		if e != nil || !ok || seen[key] || (key != "mode" && key != "custom_path" && key != "max_mb" && key != "backups") {
			return false
		}
		seen[key] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return false
		}
	}
	return v.Mode == "entware" && v.CustomPath == "" && v.MaxMB >= 1 && v.MaxMB <= 16 && v.Backups >= 1 && v.Backups <= 5
}
func recoveryInspect(v recoveryVault, root, snapshot, id string) (recoveryPlan, []byte, error) {
	var p recoveryPlan
	m, e := v.Get(snapshot)
	if e != nil {
		return p, nil, e
	}
	if e = validateAdminConfigVaultRestoreManifest(root, m); e != nil {
		return p, nil, e
	}
	var a *configvault.ArtifactRecord
	for i := range m.Artifacts {
		if m.Artifacts[i].ID == id {
			a = &m.Artifacts[i]
			break
		}
	}
	if a == nil {
		return p, nil, errRecoveryConflict
	}
	p.SnapshotID = m.ID
	p.Runtime = "NOT_APPLIED"
	p.Item = recoveryItem{ID: a.ID, Path: a.SourcePath, SnapshotSHA: a.SHA256, SnapshotMode: a.Mode, Size: a.Size, Status: "unavailable", Integrity: "failed", Reason: "UNSUPPORTED_TARGET"}
	rec, desired, e := v.ReadArtifact(snapshot, id)
	if e != nil || rec != *a || int64(len(desired)) != a.Size || recoveryHash(desired) != a.SHA256 {
		return p, nil, errRecoveryConflict
	}
	p.Item.Integrity = "verified"
	current, mode, e := recoveryRead(root, a.SourcePath)
	if os.IsNotExist(e) {
		p.Item.Status = "missing"
		p.Item.Reason = "MISSING_TARGET"
		return p, nil, nil
	}
	if e != nil {
		return p, nil, e
	}
	p.Item.CurrentSHA = recoveryHash(current)
	p.Item.CurrentMode = mode
	p.Item.Status = "changed"
	if p.Item.CurrentSHA == a.SHA256 && mode == a.Mode {
		p.Item.Status = "unchanged"
		p.Item.Reason = "NO_CHANGE"
		return p, nil, nil
	}
	// Also validate the preimage so the safety snapshot is itself restorable.
	p.Item.RestoreAllowed = recoveryAllowed(root, a.SourcePath, desired, a.Mode) && recoveryAllowed(root, a.SourcePath, current, mode)
	if p.Item.RestoreAllowed {
		p.Item.Reason = "READY"
		p.Before = &recoveryLoggingValues{}
		p.After = &recoveryLoggingValues{}
		if json.Unmarshal(current, p.Before) != nil || json.Unmarshal(desired, p.After) != nil {
			return p, nil, errRecoveryConflict
		}
		raw, _ := json.Marshal(p)
		p.Token = recoveryHash(raw)
	}
	return p, desired, nil
}

type recoveryResult struct {
	OK               bool   `json:"ok"`
	SafetySnapshot   string `json:"safety_snapshot,omitempty"`
	ReadbackVerified bool   `json:"readback_verified"`
	Runtime          string `json:"runtime"`
	State            string `json:"state"`
}

func recoveryApply(v recoveryVault, root string, q recoveryRequest) (recoveryResult, error) {
	out := recoveryResult{Runtime: "NOT_APPLIED", State: "BLOCKED"}
	p, desired, e := recoveryInspect(v, root, q.SnapshotID, q.ArtifactID)
	if e != nil || !p.Item.RestoreAllowed || q.Token == "" || q.Token != p.Token {
		return out, errRecoveryConflict
	}
	// Same Config Vault, single-file preimage, never evict earlier snapshots.
	before, e := v.Capture(configvault.CaptureRequest{Component: "admin", Reason: "Recovery Center pre-restore " + q.SnapshotID, Artifacts: []configvault.ArtifactSpec{{ID: q.ArtifactID, Path: p.Item.Path}}, NoPrune: true, Version: version})
	if e != nil {
		return out, e
	}
	out.SafetySnapshot = before.ID
	if len(before.Artifacts) != 1 || before.Artifacts[0].SHA256 != p.Item.CurrentSHA || before.Artifacts[0].Mode != p.Item.CurrentMode {
		return out, errRecoveryConflict
	}
	// Recheck exact revision immediately before publishing. This serializes Admin
	// mutations, not arbitrary external writers; no claim of global atomic CAS.
	recheck, _, e := recoveryInspect(v, root, q.SnapshotID, q.ArtifactID)
	if e != nil || recheck.Token != p.Token {
		return out, errRecoveryConflict
	}
	out.State = "WRITE_UNCERTAIN"
	if e = safety.WriteFileAtomic(p.Item.Path, desired, os.FileMode(p.Item.SnapshotMode)); e != nil {
		return out, e
	}
	data, mode, e := recoveryRead(root, p.Item.Path)
	if e != nil || recoveryHash(data) != p.Item.SnapshotSHA || mode != p.Item.SnapshotMode {
		return out, errRecoveryConflict
	}
	out.OK = true
	out.ReadbackVerified = true
	out.State = "SAVED"
	return out, nil
}
func registerAdminRecoveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/recovery-inspect", getOnly(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if len(q) != 2 || len(q["id"]) != 1 || len(q["artifact"]) != 1 || len(q.Get("id")) > 80 || len(q.Get("artifact")) > 64 {
			writeJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		unlock, _ := lockAdminConfigVaultMutation(adminConfigVaultManagedRoot)
		defer unlock()
		v, e := newAdminConfigVault()
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "VAULT_UNAVAILABLE"})
			return
		}
		p, _, e := recoveryInspect(v, adminConfigVaultManagedRoot, q.Get("id"), q.Get("artifact"))
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "RECOVERY_INSPECT_FAILED"})
			return
		}
		writeJSON(w, 200, p)
	}))
	mux.HandleFunc("/v1/maintenance/recovery-restore", packageMutationOnly(func(w http.ResponseWriter, r *http.Request) {
		var q recoveryRequest
		if decodeMutationJSON(w, r, &q) != nil || q.Confirm != "RECOVERY_RESTORE_ONE" || len(q.SnapshotID) > 80 || len(q.ArtifactID) > 64 || len(q.Token) != 64 {
			writeJSON(w, 400, map[string]string{"error": "INVALID_CONFIRMATION"})
			return
		}
		unlock, _ := lockAdminConfigVaultMutation(adminConfigVaultManagedRoot)
		defer unlock()
		v, e := newAdminConfigVault()
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "VAULT_UNAVAILABLE"})
			return
		}
		out, e := recoveryApply(v, adminConfigVaultManagedRoot, q)
		status := 200
		if e != nil {
			status = 409
		}
		log.Printf("routerforge-admin recovery snapshot=%q artifact=%q safety=%q state=%q", q.SnapshotID, q.ArtifactID, out.SafetySnapshot, out.State)
		writeJSON(w, status, out)
	}))
}
