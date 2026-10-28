package dnsrouter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDNSRouterModuleFrameHealthContract(t *testing.T) {
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	api := NewAPI(store, "").Handler()
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest(method, "/v1/health", nil))
		if w.Code != 200 {
			t.Fatalf("%s /v1/health: status=%d body=%s", method, w.Code, w.Body.String())
		}
		if method == "GET" {
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode health response: %v", err)
			}
			if body["component"] != "dns-router" || body["status"] != "ok" {
				t.Fatalf("unexpected health response: %v", body)
			}
		}
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/health", nil))
	if w.Code != 405 {
		t.Fatalf("mutation allowed at health endpoint: %d", w.Code)
	}
}

func TestAPIConfigCRUDAndCAS(t *testing.T) {
	st := NewConfigStore(filepath.Join(t.TempDir(), "cfg.json"))
	api := NewAPI(st, "").Handler()
	request := func(method, path, body, action, etag string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if action != "" {
			req.Header.Set("X-RouterForge-Action", action)
		}
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		return w
	}
	w := request("GET", "/v1/config", "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var initial struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	fixture := fixture()
	raw, _ := json.Marshal(fixture)
	if response := request("PUT", "/v1/config", string(raw), "", initial.Revision); response.Code != 403 {
		t.Fatalf("header gate: %d", response.Code)
	}
	if response := request("PUT", "/v1/config", string(raw), "save", ""); response.Code != 428 {
		t.Fatalf("revision gate: %d", response.Code)
	}
	response := request("PUT", "/v1/config", string(raw), "save", initial.Revision)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	if response := request("PUT", "/v1/config", string(raw), "save", initial.Revision); response.Code != 409 {
		t.Fatalf("stale allowed: %d", response.Code)
	}
	dec := request("POST", "/v1/evaluate", `{"domain":"api.example.com"}`, "evaluate", "")
	if dec.Code != 200 || !strings.Contains(dec.Body.String(), `"upstream":"two"`) {
		t.Fatal(dec.Code, dec.Body)
	}
	exp := request("GET", "/v1/export", "", "", "")
	if exp.Code != 200 || !strings.Contains(exp.Body.String(), "example.com") {
		t.Fatal(exp.Code, exp.Body)
	}
	info, err := os.Stat(st.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions: %v %v", info, err)
	}
	w = request("GET", "/v1/status", "", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"dns_interception":false`) {
		t.Fatal(w.Code, w.Body)
	}
}

func TestImportPreviewAndInvalidIsolation(t *testing.T) {
	st := NewConfigStore(filepath.Join(t.TempDir(), "cfg.json"))
	_, rev, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Save(Config{SchemaVersion: 1, Upstreams: []Upstream{{ID: "cf", Transport: "udp", Address: "1.1.1.1:53"}}, Rules: []Rule{}}, rev)
	if err != nil {
		t.Fatal(err)
	}
	api := NewAPI(st, "").Handler()
	req := httptest.NewRequest("POST", "/v1/import-preview", strings.NewReader(`{"text":"example.com\nexample.com\n# comment\nsub.example.org","upstream":"cf","match":"suffix","mode":"append","priority":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RouterForge-Action", "import-preview")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"added":2`) || !strings.Contains(w.Body.String(), `"duplicates":1`) {
		t.Fatal(w.Body)
	}
	cfg, _, err := st.Load()
	if err != nil || len(cfg.Rules) != 0 {
		t.Fatal("preview persisted changes", err)
	}
	invalid := httptest.NewRequest("PUT", "/v1/config", strings.NewReader(`{"schema_version":1,"upstreams":[],"rules":[{"id":"bad","domain":"local","match":"suffix","upstream":"missing","priority":1}]}`))
	invalid.Header.Set("Content-Type", "application/json")
	invalid.Header.Set("X-RouterForge-Action", "save")
	_, current, _ := st.Load()
	invalid.Header.Set("If-Match", current)
	out := httptest.NewRecorder()
	api.ServeHTTP(out, invalid)
	if out.Code != 400 {
		t.Fatal(out.Code, out.Body)
	}
	cfg, _, _ = st.Load()
	if len(cfg.Rules) != 0 {
		t.Fatal("invalid update changed config")
	}
}

func TestAtomicCASConcurrency(t *testing.T) {
	st := NewConfigStore(filepath.Join(t.TempDir(), "cfg.json"))
	_, rev, _ := st.Load()
	cfg := fixture()
	var wg sync.WaitGroup
	wins := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := st.Save(cfg, rev); wins <- err == nil }()
	}
	wg.Wait()
	close(wins)
	n := 0
	for win := range wins {
		if win {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("concurrent update winners: %d", n)
	}
}

func TestUIStaticAllowlist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("test-ui"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := NewAPI(NewConfigStore(filepath.Join(t.TempDir(), "cfg.json")), dir).Handler()
	for path, want := range map[string]int{"/ui/index.html": 200, "/v1/ui/index.html": 200, "/ui/../../secret": 301, "/ui/not-allowed.txt": 404} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%s => %d", path, w.Code)
		}
		if want == 200 {
			all, _ := io.ReadAll(w.Body)
			if !bytes.Contains(all, []byte("test-ui")) {
				t.Fatal("wrong UI payload")
			}
		}
	}
}

func TestShadowStatusAPI(t *testing.T) {
	a := NewAPI(NewConfigStore(filepath.Join(t.TempDir(), "config.json")), "")
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/shadow/status", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"active":false`) {
		t.Fatal(w.Code, w.Body)
	}
}
