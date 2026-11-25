package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
)

func TestFoundationBindingLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux directory durability")
	}
	dir := t.TempDir()
	store := contract.NewStore(filepath.Join(dir, "catalog.json"))
	initial, e := store.Get()
	if e != nil {
		t.Fatal(e)
	}
	raw := []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[{"id":"root","type":"domain-group","name":"Group","scope":"local","revision":1,"members":[{"kind":"domain","value":"new.example"}]}]}`)
	cp, e := store.Preview(initial.Current.ETag, raw)
	if e != nil {
		t.Fatal(e)
	}
	saved, _, e := store.Commit(initial.Current.ETag, cp.Candidate.ETag, raw)
	if e != nil {
		t.Fatal(e)
	}
	cat := saved.Current
	path := filepath.Join(dir, "bindings.json")
	values := []string{"old.example"}
	writes := 0
	lost := false
	name := "rf-binding-demo.list"
	svc := &foundationBindingService{store: store, path: path, call: func(_ context.Context, module, method, target string, headers http.Header, body []byte) ([]byte, error) {
		if module != "nfqws-manager" {
			t.Fatal(module)
		}
		switch method + " " + target {
		case "GET /v1/list?name=" + name:
			return json.Marshal(map[string]any{"name": name, "content": contract.BindingText(values), "sha256": contract.BindingTextDigest(values), "size": len(contract.BindingText(values)), "protected": false, "path": "PRIVATE_PATH"})
		case "POST /v1/list/save":
			writes++
			st, e := contract.ReadBindingState(path)
			if e != nil || st.Bindings[0].Pending == nil {
				t.Fatal("no durable journal", e)
			}
			var q map[string]string
			if json.Unmarshal(body, &q) != nil || q["expected_sha256"] != contract.BindingTextDigest(values) || q["confirm"] != "NFQWS_LIST_SAVE" || headers.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
				t.Fatal(q, headers)
			}
			values = strings.Split(strings.TrimSuffix(q["content"], "\n"), "\n")
			if lost {
				return nil, errors.New("lost reply")
			}
			return []byte(`{"ok":true}`), nil
		default:
			t.Fatal("unexpected call", method, target)
			return nil, nil
		}
	}}
	mux := http.NewServeMux()
	registerFoundationStore(mux, store, svc)
	registerFoundationBindings(mux, svc, func(r *http.Request) bool { return r.Header.Get("Test-Root") == "yes" })
	req := func(op, token string, commit bool, root string) *httptest.ResponseRecorder {
		q := bindingRequest{Operation: op, Target: "nfqws." + name, GroupID: "root", CatalogueETag: cat.ETag, Token: token}
		b, _ := json.Marshal(q)
		end := "preview"
		if commit {
			end = "commit"
		}
		r := httptest.NewRequest("POST", foundationPath+"/bindings/"+end, strings.NewReader(string(b)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-RF-Foundation", "1")
		r.Header.Set("Test-Root", root)
		r.Header.Set("X-RF-Confirm", "BINDING_COMMIT")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	preview := func(op string) bindingPlan {
		w := req(op, "", false, "yes")
		var p bindingPlan
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil || strings.Contains(w.Body.String(), "PRIVATE_PATH") {
			t.Fatal(op, w.Code, w.Body.String())
		}
		return p
	}
	commit := func(op string, p bindingPlan) {
		w := req(op, p.Token, true, "yes")
		if w.Code != 200 {
			t.Fatal(op, w.Code, w.Body.String())
		}
	}
	if w := req("attach", "", false, "no"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	p := preview("attach")
	if !p.CanCommit || p.Status != "needs_sync" || writes != 0 {
		t.Fatal(p, writes)
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("preview wrote state")
	}
	if w := req("attach", strings.Repeat("0", 64), true, "yes"); w.Code != 409 {
		t.Fatal(w.Code)
	}
	commit("attach", p)
	p = preview("sync")
	values = []string{"manual.example"}
	if w := req("sync", p.Token, true, "yes"); w.Code != 409 || writes != 0 {
		t.Fatal(w.Code, writes)
	}
	if p = preview("sync"); p.Status != "native_conflict" || p.CanCommit {
		t.Fatal(p)
	}
	values = []string{"old.example"}
	p = preview("sync")
	lost = true
	if w := req("sync", p.Token, true, "yes"); w.Code != 409 || writes != 1 {
		t.Fatal(w.Code, writes)
	}
	p = preview("reconcile")
	if p.Status != "pending_after_observed" || !p.CanCommit {
		t.Fatal(p)
	}
	commit("reconcile", p)
	if writes != 1 {
		t.Fatal(writes)
	}
	lost = false
	p = preview("rollback")
	commit("rollback", p)
	if writes != 2 || !reflect.DeepEqual(values, []string{"old.example"}) {
		t.Fatal(writes, values)
	}
	empty := []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[]}`)
	candidate, e := store.Preview(cat.ETag, empty)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", foundationPath+"/save", strings.NewReader(string(empty)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-RF-Foundation", "1")
	r.Header.Set("If-Match", `"`+cat.ETag+`"`)
	r.Header.Set("X-RF-Confirm", "SAVE_DRAFT")
	r.Header.Set("X-RF-Preview", candidate.Candidate.ETag)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	p = preview("detach")
	commit("detach", p)
	if writes != 2 {
		t.Fatal(writes)
	}
}
func TestFoundationBindingDNSPrepare(t *testing.T) {
	ref, ok := contract.BindingTarget("dns.rf-binding-demo")
	if !ok {
		t.Fatal(ref)
	}
	cfg := func(domain, kind string) map[string]any {
		return map[string]any{"schema_version": 1, "upstreams": []any{map[string]string{"id": "u", "transport": "udp", "address": "192.0.2.53:53"}}, "lists": []any{map[string]any{"id": ref.Name, "kind": kind, "name": "Demo", "match": "suffix", "priority": 10, "upstream": "u"}}, "rules": []any{map[string]any{"id": "lst-old", "list_id": ref.Name, "domain": domain, "match": "suffix", "priority": 10, "upstream": "u"}}}
	}
	raw, _ := json.Marshal(map[string]any{"revision": "sha256:base", "config": cfg("old.example", "paste")})
	before, e := bindingDNSView(raw, ref)
	if e != nil {
		t.Fatal(e)
	}
	change := false
	call := func(_ context.Context, module, method, path string, h http.Header, b []byte) ([]byte, error) {
		if module != "dns-router" || method != "POST" || path != "/v1/lists/preview" || h.Get("X-RouterForge-Action") != "lists-preview" {
			t.Fatal(module, method, path, h)
		}
		n := cfg("new.example", "paste")
		if change {
			n["upstreams"] = []any{}
		}
		return json.Marshal(map[string]any{"base_revision": "sha256:base", "saved": false, "runtime_applied": false, "preview": map[string]any{"config": n, "invalid": 0, "duplicates": 0}})
	}
	after, write, e := bindingPrepare(context.Background(), call, ref, before, []string{"new.example"})
	if e != nil || after.SHA256 == before.Snapshot.SHA256 || write.Method != "PUT" || write.Headers.Get("If-Match") != `"sha256:base"` {
		t.Fatal(after, write, e)
	}
	change = true
	if _, _, e = bindingPrepare(context.Background(), call, ref, before, []string{"new.example"}); e == nil {
		t.Fatal("unrelated changes accepted")
	}
	raw, _ = json.Marshal(map[string]any{"revision": "sha256:base", "config": cfg("old.example", "url")})
	if _, e = bindingDNSView(raw, ref); e == nil {
		t.Fatal("URL accepted")
	}
}
