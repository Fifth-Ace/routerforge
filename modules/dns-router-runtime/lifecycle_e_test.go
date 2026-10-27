package dnsrouter

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func lifecycleFixture(t *testing.T) (*ConfigStore, string, *LifecycleManager) {
	t.Helper()
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	_, rev, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	rev, err = store.Save(shadowConfig("udp"), rev)
	if err != nil {
		t.Fatal(err)
	}
	l := NewLifecycleManager(store, ShadowOptions{Listen: net.JoinHostPort("127.0.0.1", strconv.Itoa(freePort(t))), Timeout: time.Second})
	t.Cleanup(func() { _ = l.Close() })
	return store, rev, l
}
func TestDPR2ELifecycleStartStopAndRevision(t *testing.T) {
	store, rev, l := lifecycleFixture(t)
	if l.Status().Active {
		t.Fatal("autostarted")
	}
	if err := l.Start("sha256:stale"); err != ErrStale {
		t.Fatalf("stale start %v", err)
	}
	if l.Status().Active {
		t.Fatal("stale start changed runtime")
	}
	if err := l.Start(rev); err != nil {
		t.Fatal(err)
	}
	if !l.Status().Active || l.Status().AppliedRevision != rev {
		t.Fatal("not active after start")
	}
	if err := l.Start(rev); err != ErrAlreadyActive {
		t.Fatal("double start accepted", err)
	}
	cfg, next, _ := store.Load()
	cfg.Rules[0].ID = "newrule"
	next, err := store.Save(cfg, next)
	if err != nil {
		t.Fatal(err)
	}
	if l.Status().AppliedRevision != rev {
		t.Fatal("saved config changed live shadow without apply")
	}
	if outcome, err := l.Apply(rev); err != ErrStale || outcome != "stale" {
		t.Fatal(outcome, err)
	}
	outcome, err := l.Apply(next)
	if err != nil || outcome != "committed" {
		t.Fatal(outcome, err)
	}
	if !l.Status().Active || l.Status().AppliedRevision != next {
		t.Fatal("apply not activated")
	}
	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
	if l.Status().Active {
		t.Fatal("shadow still active after stop")
	}
	if err := l.Stop(); err != ErrInactive {
		t.Fatal("double stop accepted", err)
	}
	if status := ReadOnlyKeeneticDiscovery(); status.ChangesApplied || status.Mode != "read-only" {
		t.Fatalf("unsafe discovery %+v", status)
	}
	snap := CurrentResourceSnapshot()
	if snap.Goroutines < 1 || snap.HeapSysBytes < 1 {
		t.Fatalf("bad resources %+v", snap)
	}
}
func TestDPR2ELifecycleSocketConflictKeepsInactive(t *testing.T) {
	store, rev, _ := lifecycleFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	manager := NewLifecycleManager(store, ShadowOptions{Listen: listener.Addr().String()})
	if err := manager.Start(rev); err == nil {
		t.Fatal("accepted conflicting listener")
	}
	if manager.Status().Active {
		t.Fatal("stale active state")
	}
}
func TestDPR2EHTTPGuardAndDiscovery(t *testing.T) {
	store, rev, l := lifecycleFixture(t)
	a := NewAPI(store, "")
	a.Lifecycle = l
	h := a.Handler()
	req := func(op, etag, header string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"operation": op})
		r := httptest.NewRequest("POST", "/v1/shadow/control", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("If-Match", etag)
		if header != "" {
			r.Header.Set("X-RouterForge-Action", header)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := req("start", rev, ""); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	if w := req("start", "", "shadow-control"); w.Code != 428 {
		t.Fatal(w.Code, w.Body)
	}
	if w := req("start", rev, "shadow-control"); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := req("stop", "", "shadow-control"); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	for _, url := range []string{"/v1/keenetic/discovery", "/v1/runtime/resources"} {
		r := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "true") {
			t.Fatal(url, w.Code, w.Body)
		}
	}
}

// Explicit benchmark suite; reports allocations without pretending that the
// CI runner is equivalent to a physical low-memory Keenetic device.
func BenchmarkDPR2EEvaluate10K(b *testing.B) {
	cfg := Config{SchemaVersion: 1, Upstreams: []Upstream{{ID: "one", Transport: "udp", Address: "1.1.1.1:53"}}}
	for i := 0; i < 10000; i++ {
		cfg.Rules = append(cfg.Rules, Rule{ID: "rule" + strconv.Itoa(i), Domain: "host" + strconv.Itoa(i) + ".example.com", Match: "exact", Upstream: "one", Priority: 1})
	}
	engine, err := Compile(cfg)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Evaluate("host5555.example.com")
		if err != nil {
			b.Fatal(err)
		}
	}
}
