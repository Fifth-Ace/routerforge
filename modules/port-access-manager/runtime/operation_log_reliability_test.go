package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// P40C exercises the real in-memory operation-log implementation without
// touching kernel, firewall, engine configuration, or daemon processes.
func p40cFreshLog(t *testing.T) {
	t.Helper()
	operationLog.Lock()
	savedEntries, savedNext := operationLog.Entries, operationLog.Next
	operationLog.Entries, operationLog.Next = nil, 0
	operationLog.Unlock()
	t.Cleanup(func() {
		operationLog.Lock()
		defer operationLog.Unlock()
		operationLog.Entries, operationLog.Next = savedEntries, savedNext
	})
}

func p40cGet(t *testing.T) (struct {
	Entries    []operationLogEntry `json:"entries"`
	LastID     uint64              `json:"last_id"`
	Scope      string              `json:"scope"`
	Persistent bool                `json:"persistent"`
}, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	operationLogsHandler(w, httptest.NewRequest(http.MethodGet, "/v1/operation-logs", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET logs: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Entries    []operationLogEntry `json:"entries"`
		LastID     uint64              `json:"last_id"`
		Scope      string              `json:"scope"`
		Persistent bool                `json:"persistent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response, w
}

func TestP40CConcurrentBoundedOrderedLog(t *testing.T) {
	p40cFreshLog(t)
	const writers, perWriter = 16, 80
	var wg sync.WaitGroup
	errors := make(chan error, writers+4)
	// Readers and writers overlap; run this test with -race in CI.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 100; k++ {
				w := httptest.NewRecorder()
				operationLogsHandler(w, httptest.NewRequest(http.MethodGet, "/v1/operation-logs", nil))
				if w.Code != http.StatusOK {
					errors <- fmt.Errorf("read status %d", w.Code)
					return
				}
				var v struct {
					Entries []operationLogEntry `json:"entries"`
					LastID  uint64              `json:"last_id"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
					errors <- err
					return
				}
				if len(v.Entries) > operationLogLimit {
					errors <- fmt.Errorf("unbounded read: %d", len(v.Entries))
					return
				}
				for i := 1; i < len(v.Entries); i++ {
					if v.Entries[i].ID != v.Entries[i-1].ID+1 {
						errors <- fmt.Errorf("non-contiguous snapshot IDs")
						return
					}
				}
				if len(v.Entries) > 0 && v.Entries[len(v.Entries)-1].ID != v.LastID {
					errors <- fmt.Errorf("last_id disagrees with snapshot")
					return
				}
			}
		}()
	}
	for n := 0; n < writers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				recordOperation("odin", "request", "PASS")
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	response, _ := p40cGet(t)
	want := uint64(writers * perWriter)
	if response.LastID != want || len(response.Entries) != operationLogLimit {
		t.Fatalf("last=%d length=%d; want last=%d length=%d", response.LastID, len(response.Entries), want, operationLogLimit)
	}
	if response.Entries[0].ID != want-operationLogLimit+1 || response.Entries[len(response.Entries)-1].ID != want {
		t.Fatal("ring tail lost ordering or bounded retention")
	}
	for i, entry := range response.Entries {
		if entry.ID != want-operationLogLimit+1+uint64(i) || entry.At == "" {
			t.Fatalf("invalid entry at %d: %+v", i, entry)
		}
	}
	if response.Scope != "operations" || response.Persistent {
		t.Fatal("unexpected log persistence contract")
	}
}

func TestP40CMethodAndCacheContract(t *testing.T) {
	p40cFreshLog(t)
	_, w := p40cGet(t)
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing response protections")
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		w = httptest.NewRecorder()
		operationLogsHandler(w, httptest.NewRequest(method, "/v1/operation-logs", nil))
		if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET" {
			t.Fatalf("%s accepted or missing Allow: %d", method, w.Code)
		}
	}
}

func TestP40CAuthAuditAndSecretRedaction(t *testing.T) {
	p40cFreshLog(t)
	secret := "P40C_NEVER_LOG_THIS_SPA_SECRET"
	wrapped := auditOperation("fwknopd", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	unauthorized := httptest.NewRequest(http.MethodPost, "/v1/engine-deploy", strings.NewReader(secret))
	wrapped(httptest.NewRecorder(), unauthorized)
	empty, _ := p40cGet(t)
	if empty.LastID != 0 || len(empty.Entries) != 0 {
		t.Fatal("unauthorized request entered operation log")
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/engine-deploy", strings.NewReader(secret))
	request.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")
	wrapped(httptest.NewRecorder(), request)
	result, w := p40cGet(t)
	if result.LastID != 2 || len(result.Entries) != 2 || result.Entries[0].Status != "START" || result.Entries[1].Status != "FAIL HTTP Service Unavailable" {
		t.Fatalf("unexpected audit transitions: %+v", result.Entries)
	}
	if strings.Contains(w.Body.String(), secret) {
		t.Fatal("SPA secret leaked into operation log")
	}
}
