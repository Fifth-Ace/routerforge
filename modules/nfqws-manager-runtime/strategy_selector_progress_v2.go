package main

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type v2SelectorProgress struct {
	SessionID string `json:"session_id"`
	Phase     string `json:"phase"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Message   string `json:"message"`
	Done      bool   `json:"done"`
	Failed    bool   `json:"failed"`
	UpdatedAt string `json:"updated_at"`
}

var v2SelectorProgressState = struct {
	sync.Mutex
	items map[string]v2SelectorProgress
}{items: map[string]v2SelectorProgress{}}

func registerSelectorProgressV2Route(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/selector-progress", getOnly(handleV2SelectorProgress))
}

func v2SelectorSessionValid(id string) bool {
	return benchSessionIDPattern.MatchString(strings.TrimSpace(id))
}

func setV2SelectorProgress(id, phase string, completed, total int, message string, done, failed bool) {
	if !v2SelectorSessionValid(id) {
		return
	}
	now := time.Now().UTC()
	v2SelectorProgressState.Lock()
	defer v2SelectorProgressState.Unlock()
	for key, item := range v2SelectorProgressState.items {
		if stamp, err := time.Parse(time.RFC3339Nano, item.UpdatedAt); err != nil || now.Sub(stamp) > 15*time.Minute {
			delete(v2SelectorProgressState.items, key)
		}
	}
	if len(v2SelectorProgressState.items) >= 32 {
		var oldestKey string
		var oldest time.Time
		for key, item := range v2SelectorProgressState.items {
			stamp, _ := time.Parse(time.RFC3339Nano, item.UpdatedAt)
			if oldestKey == "" || stamp.Before(oldest) {
				oldestKey, oldest = key, stamp
			}
		}
		if oldestKey != "" && oldestKey != id {
			delete(v2SelectorProgressState.items, oldestKey)
		}
	}
	v2SelectorProgressState.items[id] = v2SelectorProgress{
		SessionID: id, Phase: phase, Completed: completed, Total: total,
		Message: message, Done: done, Failed: failed, UpdatedAt: now.Format(time.RFC3339Nano),
	}
}

func handleV2SelectorProgress(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("session"))
	if !v2SelectorSessionValid(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid selector session id"})
		return
	}
	v2SelectorProgressState.Lock()
	item, ok := v2SelectorProgressState.items[id]
	v2SelectorProgressState.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "selector session not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}
