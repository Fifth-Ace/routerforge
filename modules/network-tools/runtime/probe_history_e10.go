package main

import (
	"net/http"
	"sync"
	"time"
)

const probeHistoryLimitE10 = 64

type probeHistoryRowE10 struct {
	At     time.Time   `json:"at"`
	Result probeResult `json:"result"`
}
type probeHistoryE10 struct {
	mu   sync.Mutex
	rows []probeHistoryRowE10
}

var recentProbesE10 probeHistoryE10

func (h *probeHistoryE10) add(r probeResult) {
	if !validTarget(r.Target) || r.Error == "unsupported probe kind" || r.Error == "invalid port" {
		return
	}
	if !r.OK && r.ErrorClass == "" {
		r.ErrorClass = "unclassified"
	}
	if r.Kind == "dns" || r.Kind == "ping" {
		r.Port = 0
	}
	r.Output = ""
	r.Addresses = nil
	r.Error = ""
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rows) >= probeHistoryLimitE10 {
		copy(h.rows, h.rows[1:])
		h.rows = h.rows[:probeHistoryLimitE10-1]
	}
	h.rows = append(h.rows, probeHistoryRowE10{time.Now().UTC(), r})
}
func (h *probeHistoryE10) snapshot() []probeHistoryRowE10 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]probeHistoryRowE10{}, h.rows...)
}
func handleProbeHistoryE10(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeJSON(w, 400, map[string]string{"error": "invalid query"})
		return
	}
	writeJSON(w, 200, map[string]any{"scope": "network-tools-process", "limit": probeHistoryLimitE10, "rows": recentProbesE10.snapshot()})
}

func (h *probeHistoryE10) summary() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	ok := 0
	for _, row := range h.rows {
		if row.Result.OK {
			ok++
		}
	}
	return map[string]int{"samples": len(h.rows), "successful": ok, "failed": len(h.rows) - ok}
}
