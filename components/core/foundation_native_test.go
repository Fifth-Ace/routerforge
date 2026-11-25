package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

func TestFoundationObserverPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "observer.id")
	ids := make(chan string, 6)
	errs := make(chan error, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); id, e := loadFoundationObserver(p); ids <- id; errs <- e }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	id := ""
	for v := range ids {
		if id == "" {
			id = v
		}
		if id != v || !foundationObserverPattern.MatchString(v) {
			t.Fatal("identity race", v, id)
		}
	}
	next, e := loadFoundationObserver(p)
	if e != nil || next != id {
		t.Fatal(next, e)
	}
	if e = os.WriteFile(p, []byte("invalid"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = loadFoundationObserver(p); e == nil {
		t.Fatal("invalid identity regenerated")
	}
	b, e := os.ReadFile(p)
	if e != nil || string(b) != "invalid" {
		t.Fatal("corrupt overwritten", e)
	}
	dir := t.TempDir()
	if _, e = loadFoundationObserver(dir); e == nil {
		t.Fatal("directory accepted")
	}
	if e = os.Symlink(p, filepath.Join(dir, "link")); e == nil {
		if _, e = loadFoundationObserver(filepath.Join(dir, "link")); e == nil {
			t.Fatal("symlink accepted")
		}
	}
}
func TestFoundationNativeReferenceHTTP(t *testing.T) {
	s := contract.NewStore(filepath.Join(t.TempDir(), "catalog.json"))
	mux := http.NewServeMux()
	content := "example.org\n"
	calls := 0
	read := func(_ context.Context, module, target string) ([]byte, error) {
		calls++
		if module != "nfqws-manager" || target != "/v1/list?name=user.list" {
			t.Fatalf("unexpected native request %s %s", module, target)
		}
		h := sha256.Sum256([]byte(content))
		return json.Marshal(map[string]any{"name": "user.list", "size": len(content), "content": content, "sha256": hex.EncodeToString(h[:]), "path": "secret-path"})
	}
	registerFoundationNative(mux, s, read)
	registerFoundationConsumersWithReader(mux, s, read)
	get := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, foundationPath+path, nil))
		return w
	}
	w := get("GET", "/native-reference?ref=nfqws.user.list")
	var info struct {
		Pin, Status string
		Count       int
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &info) != nil || info.Status != "unpinned" || info.Count != 1 || strings.Contains(w.Body.String(), "secret-path") || strings.Contains(w.Body.String(), "example.org") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, q := range []string{"ref=nfqws...%2Fuser.list", "ref=nfqws.user.list&url=foo", "ref=nfqws.user.list&ref=other"} {
		if w = get("GET", "/native-reference?"+q); w.Code != 400 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if w = get("POST", "/native-reference?ref=nfqws.user.list"); w.Code != 405 || calls != 1 {
		t.Fatal(w.Code, calls)
	}
	initial, e := s.Get()
	if e != nil {
		t.Fatal(e)
	}
	body := []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[{"id":"group","type":"domain-group","name":"Group","scope":"local","revision":1,"members":[{"kind":"list-ref","value":"` + info.Pin + `"}]}]}`)
	p, e := s.Preview(initial.Current.ETag, body)
	if e != nil {
		t.Fatal(e)
	}
	st, changed, e := s.Commit(initial.Current.ETag, p.Candidate.ETag, body)
	if e != nil || !changed {
		t.Fatal(e, changed)
	}
	q := "/consumers?consumer=network&id=group&etag=" + st.Current.ETag
	w = get("GET", q)
	var projection contract.ConsumerPreview
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &projection) != nil || len(projection.Values) != 1 || projection.Values[0] != "example.org" {
		t.Fatal(w.Code, w.Body.String())
	}
	content = "other.example\n"
	w = get("GET", q)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &projection) != nil || len(projection.Values) != 0 || len(projection.Blocked) != 1 || projection.Blocked[0] != "NATIVE_REFERENCE_STALE" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = get("GET", "/native-reference?ref="+url.QueryEscape(info.Pin))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"stale"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = get("GET", "/native-references")
	if w.Code != 200 || !strings.Contains(w.Body.String(), info.Pin) {
		t.Fatal(w.Code, w.Body.String())
	}
	after, e := s.Get()
	if e != nil || after.Current.ETag != st.Current.ETag {
		t.Fatal("GET mutated catalogue", e)
	}
}
func TestFoundationNativeEventProjection(t *testing.T) {
	id, e := platformevents.NewEventID()
	if e != nil {
		t.Fatal(e)
	}
	ring := platformevents.NewRing(5)
	ring.Add(platformevents.Event{Component: "policy", Type: "policy.catalog.references.changed", Severity: platformevents.Info, Message: "do not expose"})
	got := ring.Query(platformevents.Query{})
	if len(got) != 1 {
		t.Fatal(got)
	}
	wire, ok := platformevents.Normalize(got[0], id)
	if !ok || wire.ObserverID != id || wire.Type != "policy.catalog.references.changed" {
		t.Fatal(wire, ok)
	}
}
