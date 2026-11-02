package lite

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

// Actual UDP/TCP listener and real independent DNS upstreams. Only LAN identity
// discovery is injected, since a CI host has no Keenetic LAN/DHCP interfaces.
func TestP21TwoClientsSameDomainNoCacheOrPendingLeak(t *testing.T) {
	var a, b atomic.Int32
	ad := dnsFixture(t, "DEVICE_A", &a)
	bd := dnsFixture(t, "DEVICE_B", &b)
	c := fixtureConfig(t)
	c.Rules = nil
	c.Advanced = &dr.AdvancedDNS{Devices: []dr.DevicePolicy{{MAC: testMACA, Name: "Laptop A", Target: &dr.DeviceTarget{Upstream: "chosen"}}, {MAC: testMACB, Target: &dr.DeviceTarget{Upstream: "other"}}}}
	factory := func(u dr.Upstream) (upstream.Upstream, error) {
		dest := ad
		if u.ID == "other" {
			dest = bd
		}
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	e, err := NewEngine(c, factory)
	if err != nil {
		t.Fatal(err)
	}
	e.identify = func(ip string) DeviceIdentity {
		mac := ""
		if ip == "127.0.0.2" {
			mac = testMACA
		}
		if ip == "127.0.0.3" {
			mac = testMACB
		}
		return DeviceIdentity{IP: ip, MAC: mac, Trusted: mac != "", Source: "injected-LAN-neighbor", Reason: "test LAN fixture"}
	}
	if err = e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Close(context.Background())
	if e.effectiveCacheBytes() != 0 || e.cacheMode() != "disabled_for_device_policy_isolation" {
		t.Fatal("shared cache enabled")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for _, network := range []string{"udp", "tcp"} {
		for round := 0; round < 8; round++ {
			for _, client := range []struct{ ip, marker string }{{"127.0.0.2", "DEVICE_A"}, {"127.0.0.3", "DEVICE_B"}} {
				wg.Add(1)
				go func(network, ip, marker string) {
					defer wg.Done()
					q := new(dns.Msg)
					q.SetQuestion("same-policy-test.example.", dns.TypeTXT)
					dialer := &net.Dialer{Timeout: 2 * time.Second}
					if network == "udp" {
						dialer.LocalAddr = &net.UDPAddr{IP: net.ParseIP(ip)}
					} else {
						dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(ip)}
					}
					answer, _, err := (&dns.Client{Net: network, Timeout: 3 * time.Second, Dialer: dialer}).Exchange(q, c.Service.Listen)
					if err != nil {
						errs <- err
						return
					}
					if len(answer.Answer) != 1 || !strings.Contains(answer.Answer[0].String(), marker) {
						errs <- fmt.Errorf("%s %s leaked answer: %v", network, ip, answer)
					}
				}(network, client.ip, client.marker)
			}
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if e.cacheHits.Load() != 0 || a.Load() != 16 || b.Load() != 16 {
		t.Fatalf("cross-policy cache/coalescing: a=%d b=%d hits=%d", a.Load(), b.Load(), e.cacheHits.Load())
	}
	seen := map[string]bool{}
	for _, trace := range e.Traces("same-policy-test.example") {
		if trace.DeviceMAC == "" || trace.PolicyLayer != "device" || trace.Assigned != trace.Used || trace.IdentitySource != "injected-LAN-neighbor" {
			t.Fatal(trace)
		}
		seen[trace.DeviceMAC] = true
	}
	if len(seen) != 2 {
		t.Fatal("missing device traces", seen)
	}
}
func TestP21DeviceFailureOnlyExplicitFallbackAndNegativeNoLeak(t *testing.T) {
	var count atomic.Int32
	dest := dnsFixture(t, "BACKUP", &count)
	c := fixtureConfig(t)
	c.Rules = nil
	c.Upstreams[1].Fallbacks = []string{"default"}
	c.Advanced = &dr.AdvancedDNS{Devices: []dr.DevicePolicy{{MAC: testMACA, Target: &dr.DeviceTarget{Upstream: "chosen"}}}}
	failing := true
	factory := func(u dr.Upstream) (upstream.Upstream, error) {
		if u.ID == "chosen" && failing {
			return &brokenUpstream{calls: new(atomic.Int32), id: u.ID}, nil
		}
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	e, err := NewEngine(c, factory)
	if err != nil {
		t.Fatal(err)
	}
	e.identify = func(ip string) DeviceIdentity {
		return DeviceIdentity{MAC: testMACA, Trusted: true, Source: "injected"}
	}
	if err = e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	answer := ask(t, c.Service.Listen, "udp", "fallback.example")
	if len(answer.Answer) != 1 || !strings.Contains(answer.Answer[0].String(), "BACKUP") {
		t.Fatal(answer)
	}
	trace := e.Traces("fallback.example")
	if len(trace) != 2 || trace[1].Used != "default" || trace[1].Assigned != "chosen" {
		t.Fatal(trace)
	}
	e.Close(context.Background())
	failing = false
	e, err = NewEngine(c, factory)
	if err != nil {
		t.Fatal(err)
	}
	e.identify = func(ip string) DeviceIdentity { return DeviceIdentity{MAC: testMACA, Trusted: true} }
	if err = e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Close(context.Background())
	before := count.Load()
	answer = ask(t, c.Service.Listen, "tcp", "nx.same.example")
	if answer.Rcode != dns.RcodeNameError || count.Load() != before+1 {
		t.Fatal("negative response leaked", answer)
	}
}
func TestP21AppliedSnapshotAndReadOnlyRouteAPI(t *testing.T) {
	var n atomic.Int32
	dest := dnsFixture(t, "OK", &n)
	c := fixtureConfig(t)
	c.Advanced = &dr.AdvancedDNS{Devices: []dr.DevicePolicy{{MAC: testMACA, Target: &dr.DeviceTarget{Upstream: "chosen"}}}}
	path := filepath.Join(t.TempDir(), "config.json")
	store := dr.NewConfigStore(path)
	_, rev, _ := store.Load()
	rev, err := store.Save(c, rev)
	if err != nil {
		t.Fatal(err)
	}
	factory := func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	directory := NewDeviceDirectory()
	directory.Read = func(context.Context) (DeviceSnapshot, error) {
		return DeviceSnapshot{Interfaces: deviceTopology()}, nil
	}
	m := &Manager{Store: store, Path: path, Factory: factory, Devices: directory}
	if err = m.control("start", rev); err != nil {
		t.Fatal(err)
	}
	c.Advanced.Devices[0].Target.Upstream = "default"
	draft, err := store.Save(c, rev)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	m = &Manager{Store: store, Path: path, Factory: factory, Devices: directory}
	defer m.Close()
	if err = m.Resume(); err != nil {
		t.Fatal(err)
	}
	if m.intent.InterceptionEnabled || !m.intent.ServiceEnabled {
		t.Fatal("service ON / Gate OFF intent lost", m.intent)
	}
	if m.revision != rev || m.applied.Advanced.Devices[0].Target.Upstream != "chosen" {
		t.Fatal("draft activated")
	}
	if m.engine.effectiveCacheBytes() != 0 {
		t.Fatal("cache after resume")
	}
	for _, tc := range []struct {
		applied            bool
		expected, revision string
	}{{true, "chosen", rev}, {false, "default", draft}} {
		body := fmt.Sprintf(`{"operation":"evaluate","domain":"outside.example","mac":%q,"use_applied":%v}`, testMACA, tc.applied)
		req := httptest.NewRequest("POST", "/v1/service/devices/evaluate", strings.NewReader(body))
		req.Header.Set("X-RouterForge-Action", "service-control")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("If-Match", `"`+draft+`"`)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, req)
		var result struct {
			Scope, Revision string
			NetworkVerified bool `json:"network_verified"`
			Decision        dr.ClientDecision
		}
		if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result.Decision.Upstream != tc.expected || result.Revision != tc.revision || result.Scope != "CONFIGURATION_ONLY" || result.NetworkVerified {
			t.Fatal(w.Code, w.Body.String(), err)
		}
	}
	req := httptest.NewRequest("POST", "/v1/service/devices/evaluate", strings.NewReader(`{"operation":"evaluate","domain":"outside.example"}`))
	req.Header.Set("X-RouterForge-Action", "service-control")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"stale"`)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Discovery and explanation never call the legacy native-profile manager.
	if m.Profile != nil {
		t.Fatal("native profile manager created")
	}
	if err = m.control("stop", draft); err != nil {
		t.Fatal(err)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	m = &Manager{Store: store, Path: path, Factory: factory, Devices: directory}
	if err = m.Resume(); err != nil {
		t.Fatal(err)
	}
	if m.engine != nil {
		t.Fatal("explicit OFF lost")
	}
}

func TestP21DevicePoliciesSurviveUpgradeGateChoices(t *testing.T) {
	for _, choice := range []struct {
		name          string
		service, gate bool
	}{{"OFF_OFF", false, false}, {"ON_OFF", true, false}, {"ON_ON", true, true}} {
		t.Run(choice.name, func(t *testing.T) {
			m, rev, _ := lifecycleFixture(t)
			cfg, _, err := m.Store.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Advanced = &dr.AdvancedDNS{Devices: []dr.DevicePolicy{{MAC: testMACA, Name: "persisted", Target: &dr.DeviceTarget{Upstream: "chosen"}}}}
			rev, err = m.Store.Save(cfg, rev)
			if err != nil {
				t.Fatal(err)
			}
			directory := NewDeviceDirectory()
			directory.Read = func(context.Context) (DeviceSnapshot, error) {
				return DeviceSnapshot{Interfaces: deviceTopology()}, nil
			}
			m.Devices = directory
			if choice.service {
				err = m.control("start", rev)
			} else {
				err = m.control("stop", rev)
			}
			if err != nil {
				t.Fatal(err)
			}
			if choice.gate {
				if err = m.gateControl(context.Background(), "enable", rev); err != nil {
					t.Fatal(err)
				}
			}
			before := m.intent
			if err = m.control("maintenance", rev); err != nil {
				t.Fatal(err)
			}
			if err = m.Close(); err != nil {
				t.Fatal(err)
			}
			next := resumedManager(t, m)
			next.Devices = directory
			if err = next.Resume(); err != nil {
				t.Fatal(err)
			}
			if next.intent != before || next.ActiveState() != choice.service {
				t.Fatal("upgrade changed user intent", before, next.intent)
			}
			status := next.Gate.Status()
			if status["active"] != choice.gate {
				t.Fatal("upgrade changed Gate", status, choice.gate)
			}
			if choice.service && (next.revision != rev || next.applied.Advanced.Devices[0].Name != "persisted" || next.engine.effectiveCacheBytes() != 0) {
				t.Fatal("applied device snapshot changed")
			}
			if next.Profile != nil {
				t.Fatal("native profile manager created")
			}
		})
	}
}
