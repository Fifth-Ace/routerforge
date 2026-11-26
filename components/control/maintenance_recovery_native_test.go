package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
)

// The vault is real, the fixed native socket transport is injected to test
// ordering, preimage capture and refusal of stale plans without a live router.
func TestE062DNSRecoveryOrchestration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	path := filepath.Join(root, "dns-router", "config.json")
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	v, e := configvault.New(configvault.Options{Root: filepath.Join(filepath.Dir(root), "vault"), AllowedRoots: []string{root}})
	if e != nil {
		t.Fatal(e)
	}
	m, e := v.Capture(configvault.CaptureRequest{Component: "admin", Artifacts: []configvault.ArtifactSpec{{ID: "dns", Path: path}}})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte("current"), 0600); e != nil {
		t.Fatal(e)
	}
	calls := 0
	current := []byte("current")
	rev := func(b []byte) string { return "sha256:" + recoveryHash(b) }
	call := func(ctx context.Context, module, method, endpoint, expected string, b []byte) ([]byte, error) {
		if module != "dns" {
			t.Fatal("wrong module")
		}
		switch method {
		case "GET":
			return json.Marshal(map[string]string{"revision": rev(current)})
		case "POST":
			if expected != rev(current) {
				return nil, errRecoveryConflict
			}
			return json.Marshal(recoveryDNSPreview{CurrentRevision: rev(current), TargetRevision: rev(b), Changed: rev(current) != rev(b), Runtime: "NOT_APPLIED"})
		case "PUT":
			if expected != rev(current) {
				return nil, errRecoveryConflict
			}
			calls++
			list, e := v.List()
			if e != nil || len(list) != 2 {
				t.Fatal("safety snapshot not created before native write")
			}
			current = append([]byte(nil), b...)
			return json.Marshal(map[string]any{"saved": true, "revision": rev(current), "runtime": "NOT_APPLIED"})
		default:
			t.Fatal("unexpected method")
			return nil, errRecoveryConflict
		}
	}
	q := recoveryRequest{SnapshotID: m.ID, ArtifactID: "dns"}
	p, _, e := recoveryDNSPlanFor(context.Background(), v, root, q, call)
	if e != nil || !p.CanCommit {
		t.Fatalf("preview failed: %v", e)
	}
	if calls != 0 {
		t.Fatal("preview wrote")
	}
	q.Token = p.Token + "x"
	if _, e = recoveryDNSApply(context.Background(), v, root, q, call); e == nil || calls != 0 {
		t.Fatal("stale token accepted")
	}
	q.Token = p.Token
	out, e := recoveryDNSApply(context.Background(), v, root, q, call)
	if e != nil || !out.OK || !out.ReadbackVerified || calls != 1 || out.SafetySnapshot == "" {
		t.Fatalf("restore failed: %+v %v", out, e)
	}
	_, preimage, e := v.ReadArtifact(out.SafetySnapshot, "dns")
	if e != nil || string(preimage) != "current" {
		t.Fatal("safety preimage mismatch")
	}
}
