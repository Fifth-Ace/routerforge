package main

import (
	"context"
	sb "github.com/Fifth-Ace/routerforge/internal/platform/supportbundle"
	"net/http/httptest"
	"testing"
)

func TestServiceObservationHealthGuard(t *testing.T) {
	calls := 0
	h := serviceHealthHandler(func(_ context.Context, id string) sb.Health { calls++; return sb.Health{ID: id, State: "responded"} }, func(id string) bool { return id == "S91routerforge-dns" })
	for _, x := range []struct {
		method, q string
		code      int
	}{{"POST", "id=S91routerforge-dns", 405}, {"GET", "id=S91routerforge-dns&id=S91routerforge-dns", 400}, {"GET", "id=S91routerforge-dns&path=/etc/passwd", 400}, {"GET", "id=S98routerforge-antiscan-manager", 409}, {"GET", "id=S90routerforge", 409}, {"GET", "id=S91routerforge-admin", 404}, {"GET", "id=S91routerforge-dns", 200}} {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(x.method, "/v1/service-inspector/health?"+x.q, nil))
		if w.Code != x.code {
			t.Fatalf("%+v got %d %s", x, w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestServiceObservationProtectedExclusion(t *testing.T) {
	for _, id := range []string{"S98routerforge-antiscan-manager", "S99routerforge-crowdsec-manager", "S95routerforge-port-access-manager"} {
		if !serviceObservationExcluded(serviceInspectorItem{ID: id}) || serviceHealthModule(id) != "" {
			t.Fatal(id)
		}
	}
}
