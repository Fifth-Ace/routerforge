package events

import (
	"sync"
	"time"
)

type Severity string

const (
	Info     Severity = "info"
	Warning  Severity = "warning"
	Critical Severity = "critical"
)

type Event struct {
	Time          time.Time      `json:"time"`
	Component     string         `json:"component"`
	Severity      Severity       `json:"severity"`
	Type          string         `json:"type"`
	Message       string         `json:"message"`
	ObjectID      string         `json:"object_id,omitempty"`
	TransactionID string         `json:"transaction_id,omitempty"`
	Recovery      bool           `json:"recovery,omitempty"`
	Context       map[string]any `json:"context,omitempty"`
}

type Ring struct {
	mu    sync.RWMutex
	items []Event
	cap   int
	next  int
	full  bool
}

func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{items: make([]Event, capacity), cap: capacity}
}

func (r *Ring) Add(event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	r.mu.Lock()
	r.items[r.next] = event
	r.next = (r.next + 1) % r.cap
	if r.next == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

func (r *Ring) Snapshot(limit int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := r.next
	if r.full {
		count = r.cap
	}
	if limit <= 0 || limit > count {
		limit = count
	}
	out := make([]Event, 0, limit)
	start := 0
	if r.full {
		start = r.next
	}
	for i := count - limit; i < count; i++ {
		idx := (start + i) % r.cap
		out = append(out, r.items[idx])
	}
	return out
}
