package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNatEvidenceHandlerReadOnly(t *testing.T) {
	h := routes(t.TempDir())
	for _, tc := range []struct {
		method, url string
		want        int
	}{
		{"POST", "/v1/nat-evidence?wan=eth3&public=50000&target=2222", http.StatusMethodNotAllowed},
		{"PUT", "/v1/nat-evidence?wan=eth3&public=50000&target=2222", http.StatusMethodNotAllowed},
		{"GET", "/v1/nat-evidence?wan=eth3&public=0&target=2222", http.StatusBadRequest},
		{"GET", "/v1/nat-evidence?wan=eth3&public=50000&target=65536", http.StatusBadRequest},
		{"GET", "/v1/nat-evidence?wan=eth3%20-j%20ACCEPT&public=50000&target=2222", http.StatusBadRequest},
		{"GET", "/v1/nat-evidence?wan=eth3&public=50000&target=2222&extra=1", http.StatusBadRequest},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.url, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %s got %d want %d", tc.method, tc.url, w.Code, tc.want)
		}
	}
}
