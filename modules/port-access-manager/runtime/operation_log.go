package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type operationLogEntry struct {
	ID     uint64 `json:"id"`
	At     string `json:"at"`
	Engine string `json:"engine"`
	Action string `json:"action"`
	Status string `json:"status"`
}

var operationLog = struct {
	sync.Mutex
	Next    uint64
	Entries []operationLogEntry
}{}

const operationLogLimit = 320

func recordOperation(engine, action, status string) {
	operationLog.Lock()
	defer operationLog.Unlock()
	operationLog.Next++
	entry := operationLogEntry{
		ID:     operationLog.Next,
		At:     time.Now().UTC().Format(time.RFC3339),
		Engine: engine,
		Action: action,
		Status: status,
	}
	operationLog.Entries = append(operationLog.Entries, entry)
	if len(operationLog.Entries) > operationLogLimit {
		operationLog.Entries = append([]operationLogEntry(nil), operationLog.Entries[len(operationLog.Entries)-operationLogLimit:]...)
	}
}

func operationLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		jsonReply(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET required"})
		return
	}
	operationLog.Lock()
	entries := append([]operationLogEntry{}, operationLog.Entries...)
	next := operationLog.Next
	operationLog.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries, "last_id": next, "scope": "operations", "persistent": false})
}

// Only known, fixed labels are recorded. Request bodies, keys and configs never enter the log.
func auditOperation(engine string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next(w, r)
			return
		}
		if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
			next(w, r)
			return
		}
		recordOperation(engine, "request", "START")
		rw := &operationStatusWriter{ResponseWriter: w, status: http.StatusOK}
		next(rw, r)
		if rw.status >= 400 {
			recordOperation(engine, "request", "FAIL HTTP "+http.StatusText(rw.status))
		} else {
			recordOperation(engine, "request", "PASS")
		}
	}
}

type operationStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *operationStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
