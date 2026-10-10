package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForwardOrderLiveRejectsUnsafeRequests(t *testing.T) {
	h := routes(t.TempDir())
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"POST", "/v1/forward-order?wan=eth3&port=2222", http.StatusMethodNotAllowed},
		{"PUT", "/v1/forward-order?wan=eth3&port=2222", http.StatusMethodNotAllowed},
		{"GET", "/v1/forward-order?wan=eth3&port=0", http.StatusBadRequest},
		{"GET", "/v1/forward-order?wan=eth3%20-j%20ACCEPT&port=2222", http.StatusBadRequest},
		{"GET", "/v1/forward-order?wan=eth3&port=2222&other=1", http.StatusBadRequest},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %s code=%d want=%d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}
