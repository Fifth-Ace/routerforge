package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
)

const recoveryDNSRelative = "dns-router/config.json"
const recoveryPolicyRelative = "policy-catalog-v1.json"

var recoveryRevisionRE = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var recoveryIDRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

// Only exact local module endpoints. Never proxy user-supplied paths or headers.
type recoveryModuleCall func(context.Context, string, string, string, string, []byte) ([]byte, error)

func recoveryCallModule(ctx context.Context, module, method, path, rev string, body []byte) ([]byte, error) {
	var socket, action string
	switch module {
	case "dns":
		socket = "/opt/var/run/routerforge-dns-router.sock"
		switch {
		case method == "GET" && path == "/v1/config":
		case method == "POST" && path == "/v1/recovery-policy-preview":
			action = "recovery-policy-preview"
		case method == "PUT" && path == "/v1/recovery-policy-save":
			action = "recovery-policy-save"
		default:
			return nil, errRecoveryConflict
		}
	default:
		return nil, errRecoveryConflict
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	client := http.Client{Transport: tr, Timeout: 7 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, method, "http://unix"+path, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/json")
	if method != "GET" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-RouterForge-Action", action)
		req.Header.Set("If-Match", "\""+rev+"\"")
		if method == "PUT" {
			req.Header.Set("X-RouterForge-Confirm", "RECOVERY_POLICY_SAVE")
		}
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, errRecoveryConflict
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errRecoveryConflict
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, recoveryLimit+65537))
	if e != nil || len(raw) > recoveryLimit+65536 {
		return nil, errRecoveryConflict
	}
	return raw, nil
}
func recoveryNativeArtifact(v recoveryVault, root, snapshot, id, relative string) (configvault.ArtifactRecord, []byte, error) {
	var empty configvault.ArtifactRecord
	m, e := v.Get(snapshot)
	if e != nil || validateAdminConfigVaultRestoreManifest(root, m) != nil {
		return empty, nil, errRecoveryConflict
	}
	for _, a := range m.Artifacts {
		if a.ID == id && a.SourcePath == filepath.Join(root, filepath.FromSlash(relative)) {
			rec, b, e := v.ReadArtifact(snapshot, id)
			if e != nil || rec != a || int64(len(b)) != a.Size || recoveryHash(b) != a.SHA256 {
				return empty, nil, errRecoveryConflict
			}
			return rec, b, nil
		}
	}
	return empty, nil, errRecoveryConflict
}

type recoveryDNSPreview struct {
	CurrentRevision string `json:"current_revision"`
	TargetRevision  string `json:"target_revision"`
	Changed         bool   `json:"changed"`
	Upstreams       int    `json:"upstreams"`
	Rules           int    `json:"rules"`
	Lists           int    `json:"lists"`
	Runtime         string `json:"runtime"`
}
type recoveryDNSPlan struct {
	SnapshotID  string             `json:"snapshot_id"`
	ArtifactID  string             `json:"artifact_id"`
	ArtifactSHA string             `json:"artifact_sha256"`
	Native      recoveryDNSPreview `json:"native"`
	Token       string             `json:"token"`
	CanCommit   bool               `json:"can_commit"`
}

func recoveryDNSRevision(ctx context.Context, call recoveryModuleCall) (string, error) {
	raw, e := call(ctx, "dns", "GET", "/v1/config", "", nil)
	if e != nil {
		return "", e
	}
	var p struct {
		Revision string `json:"revision"`
	}
	if json.Unmarshal(raw, &p) != nil || !recoveryRevisionRE.MatchString(p.Revision) {
		return "", errRecoveryConflict
	}
	return p.Revision, nil
}
func recoveryDNSNativePreview(ctx context.Context, call recoveryModuleCall, rev string, data []byte) (recoveryDNSPreview, error) {
	var p recoveryDNSPreview
	raw, e := call(ctx, "dns", "POST", "/v1/recovery-policy-preview", rev, data)
	if e != nil || json.Unmarshal(raw, &p) != nil || p.CurrentRevision != rev || !recoveryRevisionRE.MatchString(p.TargetRevision) || p.Changed != (p.CurrentRevision != p.TargetRevision) || p.Runtime != "NOT_APPLIED" {
		return p, errRecoveryConflict
	}
	return p, nil
}
func recoveryDNSPlanFor(ctx context.Context, v recoveryVault, root string, q recoveryRequest, call recoveryModuleCall) (recoveryDNSPlan, []byte, error) {
	var p recoveryDNSPlan
	a, data, e := recoveryNativeArtifact(v, root, q.SnapshotID, q.ArtifactID, recoveryDNSRelative)
	if e != nil || a.Mode != 0600 || len(data) > recoveryLimit {
		return p, nil, errRecoveryConflict
	}
	// Native policy restore may not silently invalidate persistent saved-list bindings.
	// Fail closed on a nonempty registry, regardless of module. This is an
	// observation gate, not a cross-process lock; native editor CAS is authoritative.
	bindings, e := contract.ReadBindingState(filepath.Join(root, "policy-bindings-v1.json"))
	if e != nil || len(bindings.Bindings) != 0 {
		return p, nil, errRecoveryConflict
	}
	rev, e := recoveryDNSRevision(ctx, call)
	if e != nil {
		return p, nil, e
	}
	native, e := recoveryDNSNativePreview(ctx, call, rev, data)
	if e != nil {
		return p, nil, e
	}
	p = recoveryDNSPlan{SnapshotID: q.SnapshotID, ArtifactID: q.ArtifactID, ArtifactSHA: a.SHA256, Native: native, CanCommit: native.Changed}
	if p.CanCommit {
		raw, _ := json.Marshal(p)
		p.Token = recoveryHash(raw)
	}
	return p, data, nil
}
func recoveryDNSApply(ctx context.Context, v recoveryVault, root string, q recoveryRequest, call recoveryModuleCall) (recoveryResult, error) {
	out := recoveryResult{Runtime: "NOT_APPLIED", State: "BLOCKED"}
	p, data, e := recoveryDNSPlanFor(ctx, v, root, q, call)
	if e != nil || !p.CanCommit || q.Token == "" || p.Token != q.Token {
		return out, errRecoveryConflict
	}
	// Capture exactly the saved policy preimage, no unrelated config or pruning.
	path := filepath.Join(root, filepath.FromSlash(recoveryDNSRelative))
	before, e := v.Capture(configvault.CaptureRequest{Component: "admin", Reason: "DNS policy recovery preimage for " + q.SnapshotID, Version: version, NoPrune: true, Artifacts: []configvault.ArtifactSpec{{ID: q.ArtifactID, Path: path}}})
	if e != nil {
		return out, e
	}
	out.SafetySnapshot = before.ID
	rec, prior, e := v.ReadArtifact(before.ID, q.ArtifactID)
	if e != nil || rec.SourcePath != path || rec.Mode != 0600 {
		return out, errRecoveryConflict
	}
	priorPlan, e := recoveryDNSNativePreview(ctx, call, p.Native.CurrentRevision, prior)
	if e != nil || priorPlan.Changed {
		return out, errRecoveryConflict
	}
	recheck, _, e := recoveryDNSPlanFor(ctx, v, root, q, call)
	if e != nil || recheck.Token != q.Token {
		return out, errRecoveryConflict
	}
	out.State = "WRITE_UNCERTAIN"
	raw, e := call(ctx, "dns", "PUT", "/v1/recovery-policy-save", p.Native.CurrentRevision, data)
	if e != nil {
		return out, e
	}
	var result struct {
		Saved    bool   `json:"saved"`
		Revision string `json:"revision"`
		Runtime  string `json:"runtime"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.Saved || result.Revision != p.Native.TargetRevision || result.Runtime != "NOT_APPLIED" {
		return out, errRecoveryConflict
	}
	rev, e := recoveryDNSRevision(ctx, call)
	if e != nil || rev != result.Revision {
		return out, errRecoveryConflict
	}
	out.OK = true
	out.ReadbackVerified = true
	out.State = "SAVED"
	return out, nil
}
func recoveryCatalogDraft(v recoveryVault, root string, q recoveryRequest) (json.RawMessage, error) {
	_, data, e := recoveryNativeArtifact(v, root, q.SnapshotID, q.ArtifactID, recoveryPolicyRelative)
	if e != nil {
		return nil, e
	}
	if _, e = contract.Decode(data); e != nil {
		return nil, errRecoveryConflict
	}
	var state contract.State
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&state) != nil || len(state.Current.Catalog) == 0 || contract.ValidateCatalog(state.Current.Catalog) != nil {
		return nil, errRecoveryConflict
	}
	return state.Current.Catalog, nil
}
func registerRecoveryNativeRoutes(mux *http.ServeMux) {
	for _, op := range []string{"dns-preview", "dns-restore", "policy-draft"} {
		op := op
		mux.HandleFunc("/v1/maintenance/recovery-"+op, mutationOnly(func(w http.ResponseWriter, r *http.Request) {
			var q recoveryRequest
			confirm := "RECOVERY_" + strings.ToUpper(strings.ReplaceAll(op, "-", "_"))
			if decodeMutationJSON(w, r, &q) != nil || q.Confirm != confirm || len(q.SnapshotID) > 80 || !recoveryIDRE.MatchString(q.ArtifactID) || (op == "dns-restore" && len(q.Token) != 64) || (op != "dns-restore" && q.Token != "") {
				writeJSON(w, 400, map[string]string{"error": "INVALID_RECOVERY_CONFIRMATION"})
				return
			}
			unlock, _ := lockAdminConfigVaultMutation(adminConfigVaultManagedRoot)
			defer unlock()
			v, e := newAdminConfigVault()
			if e != nil {
				writeJSON(w, 503, map[string]string{"error": "VAULT_UNAVAILABLE"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
			defer cancel()
			switch op {
			case "dns-preview":
				p, _, e := recoveryDNSPlanFor(ctx, v, adminConfigVaultManagedRoot, q, recoveryCallModule)
				if e != nil {
					writeJSON(w, 409, map[string]string{"error": "DNS_RECOVERY_BLOCKED_CHECK_COMPATIBILITY_AND_BINDINGS"})
					return
				}
				writeJSON(w, 200, p)
			case "dns-restore":
				out, e := recoveryDNSApply(ctx, v, adminConfigVaultManagedRoot, q, recoveryCallModule)
				status := 200
				if e != nil {
					status = 409
				}
				log.Printf("recovery-dns snapshot=%q artifact=%q safety=%q state=%q", q.SnapshotID, q.ArtifactID, out.SafetySnapshot, out.State)
				writeJSON(w, status, out)
			case "policy-draft":
				draft, e := recoveryCatalogDraft(v, adminConfigVaultManagedRoot, q)
				if e != nil {
					writeJSON(w, 409, map[string]string{"error": "CATALOGUE_DRAFT_BLOCKED"})
					return
				}
				writeJSON(w, 200, map[string]any{"catalog": draft, "runtime": "NOT_APPLIED"})
			}
		}))
	}
}
