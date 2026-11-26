package dnsrouter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestE062PolicyRecovery(t *testing.T) {
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	a := NewAPI(store, "")
	mux := http.NewServeMux()
	a.registerRecoveryPolicy(mux)
	_, rev, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	desired := EmptyConfig()
	desired.Upstreams = append(desired.Upstreams, Upstream{ID: "test", Transport: "udp", Address: "1.1.1.1:53"})
	desired.Rules = append(desired.Rules, Rule{ID: "r", Domain: "example.org", Match: "exact", Upstream: "test"})
	body, e := json.Marshal(desired)
	if e != nil {
		t.Fatal(e)
	}
	run := func(path, method, action, match string, confirm bool) int {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-RouterForge-Action", action)
		r.Header.Set("If-Match", "\""+match+"\"")
		if confirm {
			r.Header.Set("X-RouterForge-Confirm", "RECOVERY_POLICY_SAVE")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	if got := run("/v1/recovery-policy-preview", "POST", "recovery-policy-preview", rev, false); got != 200 {
		t.Fatalf("preview=%d", got)
	}
	_, observed, _ := store.Load()
	if observed != rev {
		t.Fatal("preview mutated")
	}
	if got := run("/v1/recovery-policy-save", "PUT", "recovery-policy-save", rev, false); got != 409 {
		t.Fatalf("confirmation=%d", got)
	}
	if got := run("/v1/recovery-policy-save", "PUT", "recovery-policy-save", rev, true); got != 200 {
		t.Fatalf("save=%d", got)
	}
	_, observed, _ = store.Load()
	if observed != revision(desired) {
		t.Fatal("wrong saved revision")
	}
	if got := run("/v1/recovery-policy-save", "PUT", "recovery-policy-save", rev, true); got != 409 {
		t.Fatalf("stale=%d", got)
	}
}
func TestE062PolicyRecoveryBlocksAutoList(t *testing.T) {
	a, b := EmptyConfig(), EmptyConfig()
	b.Lists = []DomainList{{Kind: "url", URL: "https://example.org/list"}}
	if recoveryPolicyCompatible(a, b) || recoveryPolicyCompatible(b, a) {
		t.Fatal("automatic list accepted")
	}
}
