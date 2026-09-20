package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestTCP16NetworkIdentityUsesStableIPv4Prefix(t *testing.T) {
	key, cidr, err := v2TCP16NetworkIdentity("91.98.156.82")
	if err != nil {
		t.Fatal(err)
	}
	if key != "cidr:91.98.156.0/24" || cidr != "91.98.156.0/24" {
		t.Fatalf("key=%q cidr=%q", key, cidr)
	}
}

func TestTCP16ObservationAggregatesByNetwork(t *testing.T) {
	oldPath := v2TCP16MemoryPath
	oldNow := v2TCP16MemoryNow
	oldRoot := v2TargetMemoryRoot
	t.Cleanup(func() {
		v2TCP16MemoryPath = oldPath
		v2TCP16MemoryNow = oldNow
		v2TargetMemoryRoot = oldRoot
	})

	root := t.TempDir()
	v2TargetMemoryRoot = root
	v2TCP16MemoryPath = filepath.Join(root, "tcp16-network-memory.json")
	now := time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)
	v2TCP16MemoryNow = func() time.Time { return now }

	if err := v2RecordTCP16Observation("one.example", "91.98.156.82", v2HTTPMetrics{Cutoff16KSuspected: true}); err != nil {
		t.Fatal(err)
	}
	if err := v2RecordTCP16Observation("two.example", "91.98.156.99", v2HTTPMetrics{Cutoff16KSuspected: true}); err != nil {
		t.Fatal(err)
	}

	doc, err := readV2TCP16Memory()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 1 {
		t.Fatalf("entries=%d", len(doc.Entries))
	}
	got := doc.Entries[0]
	if got.SuspectedCount != 2 || got.LastTarget != "two.example" {
		t.Fatalf("entry=%+v", got)
	}
	if got.Confidence != "OBSERVED" {
		t.Fatalf("confidence=%q", got.Confidence)
	}
}

func TestTCP16MemoryRouteIsGetOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerTCP16NetworkMemoryV1Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/v2/tcp16-memory", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d body=%s", rr.Code, rr.Body.String())
	}
}
