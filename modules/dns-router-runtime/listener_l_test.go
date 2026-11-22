package dnsrouter

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func lConfig() Config {
	c := shadowConfig("udp")
	c.Listener = &ListenerConfig{Mode: "lan", Listen: "192.168.10.1:55358", AllowedClients: []string{"192.168.10.1", "127.0.0.1"}}
	c.Rules = []Rule{{ID: "demo", Domain: "dpr2l.test", Match: "suffix", Upstream: c.Upstreams[0].ID, Priority: 100}}
	return c
}
func TestDPR2LListenerValidation(t *testing.T) {
	valid := lConfig()
	if _, err := Compile(valid); err != nil {
		t.Fatal(err)
	}
	cap := ShadowOptions{Listen: "127.0.0.1:55353", AllowLAN: true}
	opts, err := ResolveListenerOptionsL(valid, cap)
	if err != nil || opts.Listen != "192.168.10.1:55358" || len(opts.AllowedClients) != 2 {
		t.Fatalf("opts=%+v err=%v", opts, err)
	}
	if _, err := ResolveListenerOptionsL(valid, ShadowOptions{Listen: "127.0.0.1:55353"}); err == nil {
		t.Fatal("missing explicit LAN capability accepted")
	}
	cases := []ListenerConfig{
		{Mode: "lan", Listen: "0.0.0.0:55358", AllowedClients: []string{"127.0.0.1"}},
		{Mode: "lan", Listen: "8.8.8.8:55358", AllowedClients: []string{"127.0.0.1"}},
		{Mode: "lan", Listen: "192.168.10.1:53", AllowedClients: []string{"127.0.0.1"}},
		{Mode: "lan", Listen: "192.168.10.1:55358"},
		{Mode: "lan", Listen: "192.168.10.1:55358", AllowedClients: []string{"192.168.10.0/24"}},
		{Mode: "lan", Listen: "192.168.10.1:55358", AllowedClients: []string{"127.0.0.1", "127.0.0.1"}},
		{Mode: "lan", Listen: "192.168.10.1:55358", AllowedClients: []string{"8.8.8.8"}},
		{Mode: "loopback", Listen: "192.168.10.1:55358"},
		{Mode: "loopback", Listen: "127.0.0.1:55353", AllowedClients: []string{"127.0.0.1"}},
		{Mode: "lan", Listen: "[::1]:55358", AllowedClients: []string{"127.0.0.1"}},
		{Mode: "oops", Listen: "127.0.0.1:55353"},
	}
	for _, bad := range cases {
		x := valid
		x.Listener = &bad
		if _, err := Compile(x); err == nil {
			t.Errorf("accepted unsafe listener: %+v", bad)
		}
	}
	c := shadowConfig("udp")
	if _, err := Compile(c); err != nil {
		t.Fatalf("legacy config rejected: %v", err)
	}
}
func TestDPR2LDomainSplitPlanReadOnly(t *testing.T) {
	c := lConfig()
	c.Rules = append(c.Rules, Rule{ID: "child", Domain: "sub.dpr2l.test", Match: "suffix", Upstream: c.Upstreams[0].ID, Priority: 50}, Rule{ID: "exact", Domain: "example.org", Match: "exact", Upstream: c.Upstreams[0].ID, Priority: 30})
	plan := BuildDomainSplitPlanL(c)
	if !plan.Ready || plan.ChangesApplied || len(plan.Zones) != 1 || plan.Zones[0].Domain != "dpr2l.test" {
		t.Fatalf("plan=%+v", plan)
	}
	if plan.Zones[0].Add != "ip name-server 192.168.10.1:55358 dpr2l.test" {
		t.Fatal(plan.Zones[0].Add)
	}
	c.Listener.AllowedClients = []string{"192.168.10.1"}
	if BuildDomainSplitPlanL(c).Ready {
		t.Fatal("no loopback ACL but plan accepted")
	}
	c = lConfig()
	c.Upstreams[0].Address = "192.168.10.1:53"
	if BuildDomainSplitPlanL(c).Ready {
		t.Fatal("recursive ndnproxy upstream accepted")
	}
	c = lConfig()
	c.Rules[0].Domain = "com"
	if BuildDomainSplitPlanL(c).Ready {
		t.Fatal("TLD-only domain interception accepted")
	}
	c = lConfig()
	c.Listener = nil
	if BuildDomainSplitPlanL(c).Ready {
		t.Fatal("no LAN listener but plan accepted")
	}
}
func TestDPR2LAPIPlanAndCAS(t *testing.T) {
	dir := t.TempDir()
	store := NewConfigStore(filepath.Join(dir, "config.json"))
	_, revision, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := lConfig()
	revision, err = store.Save(cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPI(store, "").Handler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/keenetic/domain-plan", nil)
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Revision       string           `json:"revision"`
		Plan           DomainSplitPlanL `json:"plan"`
		RuntimeChanged bool             `json:"runtime_changed"`
		NdmcInvoked    bool             `json:"ndmc_invoked"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Revision != revision || !body.Plan.Ready || body.RuntimeChanged || body.NdmcInvoked {
		t.Fatalf("response %+v", body)
	}
	marshalled, _ := json.Marshal(cfg)
	bad := httptest.NewRecorder()
	mutate := httptest.NewRequest("PUT", "/v1/config", bytes.NewReader(marshalled))
	mutate.Header.Set("Content-Type", "application/json")
	mutate.Header.Set("X-RouterForge-Action", "save")
	mutate.Header.Set("If-Match", `"sha256:stale"`)
	handler.ServeHTTP(bad, mutate)
	if bad.Code != 409 {
		t.Fatalf("stale CAS code=%d", bad.Code)
	}
	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest("POST", "/v1/keenetic/domain-plan", strings.NewReader("{}")))
	if method.Code != 405 {
		t.Fatal("read-only route accepted mutation")
	}
}
