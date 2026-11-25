package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
)

func TestFoundationHTTPBoundary(t *testing.T) {
	p := filepath.Join(t.TempDir(), "catalog.json")
	mux := http.NewServeMux()
	registerFoundationStore(mux, contract.NewStore(p))
	call := func(method, path, origin, header, etag, confirm, preview, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://router.test"+foundationPath+path, strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-RF-Foundation", header)
		r.Header.Set("If-Match", etag)
		r.Header.Set("X-RF-Confirm", confirm)
		r.Header.Set("X-RF-Preview", preview)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	w := call("GET", "/catalog", "", "", "", "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var st contract.State
	if e := json.Unmarshal(w.Body.Bytes(), &st); e != nil {
		t.Fatal(e)
	}
	etag := `"` + st.Current.ETag + `"`
	body := `{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[{"id":"net","type":"network","name":"Test","revision":1,"scope":"local","members":[{"kind":"ip","value":"192.0.2.1"}]}]}`
	if w = call("POST", "/preview", "http://evil.test", "1", etag, "", "", body); w.Code != 403 {
		t.Fatal("cross origin", w.Code)
	}
	if w = call("POST", "/preview", "http://router.test", "", etag, "", "", body); w.Code != 403 {
		t.Fatal("missing header", w.Code)
	}
	w = call("POST", "/preview", "http://router.test", "1", etag, "", "", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var preview contract.Preview
	if e := json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("preview wrote file")
	}
	if w = call("POST", "/save", "http://router.test", "1", etag, "", "", body); w.Code != 400 {
		t.Fatal("unconfirmed save", w.Code)
	}
	w = call("POST", "/save", "http://router.test", "1", etag, "SAVE_DRAFT", preview.Candidate.ETag, body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = call("POST", "/save", "http://router.test", "1", etag, "SAVE_DRAFT", preview.Candidate.ETag, body); w.Code != 409 {
		t.Fatal("stale save", w.Code)
	}
}
