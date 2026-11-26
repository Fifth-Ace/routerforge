package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestE10HistoryBounded(t *testing.T) {
	var h probeHistoryE10
	for i := 0; i < 70; i++ {
		h.add(probeResult{Kind: "tcp", Target: "example.org", Port: 443, Output: "secret", Error: "secret", Addresses: []string{"secret"}})
	}
	r := h.snapshot()
	if len(r) != 64 || r[0].Result.Output != "" || r[0].Result.Error != "" || len(r[0].Result.Addresses) != 0 {
		t.Fatal(r)
	}
	r[0].Result.Target = "wrong"
	if h.snapshot()[0].Result.Target != "example.org" {
		t.Fatal("alias")
	}
}
func TestE10HistoryReadOnly(t *testing.T) {
	h := getOnly(handleProbeHistoryE10)
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/v1/probe-history", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/v1/probe-history", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "network-tools-process") {
		t.Fatal(w.Code, w.Body.String())
	}
}
