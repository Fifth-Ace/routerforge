package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRegisterOncePresenceAndDisable(t *testing.T) {
	var mu sync.Mutex
	var received []Event
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e Event
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accepted":true}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	m := NewManager(filepath.Join(dir, "conf.json"), filepath.Join(dir, "state.json"), srv.URL+"/v1/events", "0.13.0~beta.9", func() (float64, float64, float64, bool, bool, bool) { return 25, 65, 55, true, true, true }, srv.Client())
	clock := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }
	m.Tick(context.Background())
	if len(received) != 2 || received[0].Kind != "register" || received[1].Kind != "diagnostic" {
		t.Fatalf("events %#v", received)
	}
	m.Tick(context.Background())
	if len(received) != 2 {
		t.Fatalf("duplicate events %d", len(received))
	}
	if err := m.SetDiagnostics(false); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(25 * time.Hour)
	m.Tick(context.Background())
	if len(received) != 3 || received[2].Kind != "presence" {
		t.Fatalf("after disable %#v", received)
	}
	st := m.Status()
	if !st.RegistrationConfirmed || st.DiagnosticsEnabled {
		t.Fatalf("bad state %#v", st)
	}
	// simulated Core upgrade/restart: same state on disk, no new register
	n := NewManager(m.configPath, m.statePath, m.endpoint, "0.14.0", nil, srv.Client())
	n.now = func() time.Time { return clock }
	n.Tick(context.Background())
	if len(received) != 3 {
		t.Fatalf("restart registered twice %#v", received)
	}
}
func TestOfflineRetryUsesStableEventID(t *testing.T) {
	d := t.TempDir()
	m := NewManager(filepath.Join(d, "config"), filepath.Join(d, "state"), "https://example.invalid/v1/events", "0.1.0", nil, &http.Client{Transport: brokenTransport{}})
	m.Tick(context.Background())
	a, e := os.ReadFile(m.statePath)
	if e != nil {
		t.Fatal(e)
	}
	m.Tick(context.Background())
	b, e := os.ReadFile(m.statePath)
	if e != nil {
		t.Fatal(e)
	}
	if string(a) != string(b) {
		t.Fatal("unstable registration event after retry")
	}
}

type brokenTransport struct{}

func (brokenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}
func TestNeverSendToHTTP(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("request to HTTP") }))
	defer s.Close()
	d := t.TempDir()
	m := NewManager(filepath.Join(d, "config"), filepath.Join(d, "state"), s.URL+"/v1/events", "0.1.0", nil, nil)
	m.Tick(context.Background())
	if m.Status().CollectorConfigured {
		t.Fatal("HTTP accepted")
	}
}
func TestRejectInvalidStateWithoutRegenerating(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "state")
	os.WriteFile(p, []byte(`{"installation_id":"broken"}`), 0600)
	m := NewManager(filepath.Join(d, "config"), p, "https://example.invalid/v1/events", "0.1.0", nil, nil)
	m.httpClient = &http.Client{Transport: brokenTransport{}}
	m.Tick(context.Background())
	if m.Status().LastError != "state-unavailable" {
		t.Fatal("corrupt state not surfaced")
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != `{"installation_id":"broken"}` {
		t.Fatal("corrupt state overwritten")
	}
}
func TestBadCertificateRejected(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"accepted":true}`)) }))
	defer srv.Close()
	d := t.TempDir()
	m := NewManager(filepath.Join(d, "config"), filepath.Join(d, "state"), srv.URL+"/v1/events", "0.1.0", nil, nil)
	m.Tick(context.Background())
	if m.Status().RegistrationConfirmed {
		t.Fatal("bad certificate accepted")
	}
}
