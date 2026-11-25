package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

// Run against the actual native DNS config/preview/save handlers with a temporary
// store. No listener, lifecycle manager, external fetch, or router integration.
func TestFoundationBindingDNSRealNativeHandlers(t *testing.T) {
	ref, ok := contract.BindingTarget("dns.rf-binding-demo")
	if !ok {
		t.Fatal("target")
	}
	store := dr.NewConfigStore(filepath.Join(t.TempDir(), "dns.json"))
	cfg, rev, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Upstreams = []dr.Upstream{{ID: "test", Transport: "udp", Address: "192.0.2.53:53"}}
	preview, e := dr.PreviewManagedList(cfg, dr.ManagedListRequest{List: dr.DomainList{ID: ref.Name, Name: "Demo", Kind: "paste", Upstream: "test", Match: "suffix", Priority: 10}, Text: "example.org\n"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.Save(preview.Config, rev); e != nil {
		t.Fatal(e)
	}
	native := dr.NewAPI(store, "").Handler()
	writes := 0
	call := func(ctx context.Context, module, method, path string, headers http.Header, body []byte) ([]byte, error) {
		if module != "dns-router" {
			t.Fatal(module)
		}
		if method == "PUT" {
			writes++
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(ctx)
		if headers != nil {
			r.Header = headers.Clone()
		}
		w := httptest.NewRecorder()
		native.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Log(w.Code, w.Body.String())
			return nil, contract.ErrNativeList
		}
		return w.Body.Bytes(), nil
	}
	ctx := context.Background()
	before, e := bindingReadTarget(ctx, call, ref)
	if e != nil {
		t.Fatal(e)
	}
	after, write, e := bindingPrepare(ctx, call, ref, before, []string{"example.net"})
	if e != nil || writes != 0 {
		t.Fatal(e, writes)
	}
	if _, e = call(ctx, write.Module, write.Method, write.Path, write.Headers, write.Body); e != nil {
		t.Fatal(e)
	}
	current, e := bindingReadTarget(ctx, call, ref)
	if e != nil || !reflect.DeepEqual(current.Snapshot, after) {
		t.Fatal(current.Snapshot, after, e)
	}
	rollback, write, e := bindingPrepare(ctx, call, ref, current, before.Snapshot.Values)
	if e != nil || !reflect.DeepEqual(rollback, before.Snapshot) {
		t.Fatal(rollback, before.Snapshot, e)
	}
	if _, e = call(ctx, write.Module, write.Method, write.Path, write.Headers, write.Body); e != nil {
		t.Fatal(e)
	}
	restored, e := bindingReadTarget(ctx, call, ref)
	if e != nil || !reflect.DeepEqual(restored.Snapshot, before.Snapshot) || writes != 2 {
		t.Fatal(restored.Snapshot, before.Snapshot, e, writes)
	}
}
