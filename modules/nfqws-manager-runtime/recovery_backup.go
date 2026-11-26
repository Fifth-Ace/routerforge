package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

var errNFRecovery = errors.New("NFQWS_RECOVERY_CONFLICT_OR_UNSUPPORTED")

type nfRecoveryRequest struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Confirm string `json:"confirm"`
}
type nfRecoveryPlan struct {
	ID          string `json:"id"`
	Target      string `json:"target"`
	CurrentSHA  string `json:"current_sha256"`
	TargetSHA   string `json:"target_sha256"`
	CurrentMode uint32 `json:"current_mode"`
	Size        int    `json:"size"`
	CanCommit   bool   `json:"can_commit"`
	Token       string `json:"token"`
	Runtime     string `json:"runtime"`
}

func nfRecoveryRead(path string, limit int64) ([]byte, os.FileInfo, error) {
	parent := filepath.Dir(path)
	resolved, e := filepath.EvalSymlinks(parent)
	if e != nil || resolved != parent {
		return nil, nil, errNFRecovery
	}
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, nil, errNFRecovery
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, e
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(info, actual) || !actual.Mode().IsRegular() {
		return nil, nil, errNFRecovery
	}
	data, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(data)) > limit {
		return nil, nil, errNFRecovery
	}
	latest, e := os.Lstat(path)
	if e != nil || !os.SameFile(actual, latest) || latest.Size() != int64(len(data)) || !actual.ModTime().Equal(latest.ModTime()) || actual.Mode() != latest.Mode() {
		return nil, nil, errNFRecovery
	}
	return data, actual, nil
}
func nfRecoveryListTarget(root, kind, target string) bool {
	name := filepath.Base(target)
	lower := strings.ToLower(name)
	return kind == "list" && target == filepath.Join(root, name) && editableListName(name) && strings.HasSuffix(lower, ".list") && !protectedListName(name) && !strings.HasPrefix(lower, "rf-binding-")
}
func nfRecoveryPlanFor(id string) (nfRecoveryPlan, []byte, []byte, error) {
	var p nfRecoveryPlan
	if len(id) > 128 || id != strings.TrimSpace(id) || !safeBackupID(id) {
		return p, nil, nil, errNFRecovery
	}
	dir, e := persistentBackupDir(id)
	if e != nil {
		return p, nil, nil, e
	}
	raw, _, e := nfRecoveryRead(filepath.Join(dir, "manifest.json"), 64<<10)
	if e != nil {
		return p, nil, nil, e
	}
	var m persistentBackupManifest
	if json.Unmarshal(raw, &m) != nil || m.ID != id || !nfRecoveryListTarget(listsRoot, m.Kind, m.Target) {
		return p, nil, nil, errNFRecovery
	}
	desired, _, e := nfRecoveryRead(filepath.Join(dir, "payload.bin"), listFileMaxBytes)
	if e != nil || int64(len(desired)) != m.Size || blobSHA256(desired) != m.SHA256 || validateListContent(string(desired)) != nil {
		return p, nil, nil, errNFRecovery
	}
	current, info, e := nfRecoveryRead(m.Target, listFileMaxBytes)
	if e != nil || validateListContent(string(current)) != nil {
		return p, nil, nil, errNFRecovery
	}
	p = nfRecoveryPlan{ID: id, Target: filepath.Base(m.Target), CurrentSHA: blobSHA256(current), TargetSHA: m.SHA256, CurrentMode: uint32(info.Mode().Perm()), Size: len(desired), Runtime: "NOT_VERIFIED"}
	p.CanCommit = p.CurrentSHA != p.TargetSHA
	if p.CanCommit {
		raw, _ := json.Marshal(p)
		p.Token = blobSHA256(raw)
	}
	return p, desired, current, nil
}
func registerNFRecoveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/backups/recovery-preview", getOnly(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if len(q) != 1 || len(q["id"]) != 1 {
			writeJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		p, _, _, e := nfRecoveryPlanFor(q.Get("id"))
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "NFQWS_RECOVERY_BLOCKED"})
			return
		}
		writeJSON(w, 200, p)
	}))
	mux.HandleFunc("/v1/backups/recovery-save", mutationOnly(func(w http.ResponseWriter, r *http.Request) {
		var q nfRecoveryRequest
		if decodeJSON(w, r, &q) != nil || q.Confirm != "NFQWS_RECOVERY_LIST_SAVE" || len(q.Token) != 64 {
			writeJSON(w, 400, map[string]string{"error": "INVALID_CONFIRMATION"})
			return
		}
		p, desired, current, e := nfRecoveryPlanFor(q.ID)
		if e != nil || !p.CanCommit || p.Token != q.Token {
			writeJSON(w, 409, map[string]string{"error": "NFQWS_RECOVERY_CONFLICT"})
			return
		}
		path := filepath.Join(listsRoot, p.Target)
		// Capture a native persistent preimage without pruning. No temporary safety
		// backup rotation, no config/library/blob restoration, no service action.
		backup, e := recordPersistentBackupTargetWithRetention("recovery-before-list-"+p.Target, "list", path, current, os.FileMode(p.CurrentMode), true)
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "NFQWS_RECOVERY_SAFETY_BACKUP_BLOCKED"})
			return
		}
		fail := func(state string) {
			log.Printf("recovery-nfqws id=%q safety=%q state=%q", q.ID, backup, state)
			writeJSON(w, 409, map[string]any{"error": state, "safety_backup": backup, "runtime": "NOT_VERIFIED"})
		}
		// Check source and destination again after backup. Native editors are not
		// globally serialized against external writers; this is a preimage gate.
		recheck, _, _, e := nfRecoveryPlanFor(q.ID)
		if e != nil || recheck.Token != p.Token {
			fail("NFQWS_RECOVERY_PREIMAGE_CHANGED")
			return
		}
		if e = safety.WriteFileAtomic(path, desired, os.FileMode(p.CurrentMode)); e != nil {
			fail("NFQWS_RECOVERY_WRITE_UNCERTAIN")
			return
		}
		observed, info, e := nfRecoveryRead(path, listFileMaxBytes)
		if e != nil || blobSHA256(observed) != p.TargetSHA || uint32(info.Mode().Perm()) != p.CurrentMode {
			fail("NFQWS_RECOVERY_READBACK_UNCERTAIN")
			return
		}
		log.Printf("recovery-nfqws id=%q safety=%q state=verified", q.ID, backup)
		writeJSON(w, 200, map[string]any{"ok": true, "safety_backup": backup, "sha256": p.TargetSHA, "readback_verified": true, "service_restarted": false, "runtime": "NOT_VERIFIED"})
	}))
}
