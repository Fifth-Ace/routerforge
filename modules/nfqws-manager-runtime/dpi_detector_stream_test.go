package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDPIDetectorV5StreamRouteIsPostOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerDPIDetectorRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/dpi-detector/v5/stream", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET stream status=%d body=%s", rr.Code, rr.Body.String())
	}
}
