package events

import (
	"strings"
	"time"
)

// Query describes a bounded, read-only timeline view. Results are returned
// newest-first so callers can render an incident timeline without reversing it.
type Query struct {
	Limit         int
	Component     string
	Severity      []Severity
	Type          string
	ObjectID      string
	TransactionID string
	Recovery      *bool
	Since         time.Time
	Until         time.Time
}

func (r *Ring) Capacity() int {
	if r == nil {
		return 0
	}
	return r.cap
}

func (r *Ring) Len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.full {
		return r.cap
	}
	return r.next
}

func (r *Ring) Query(query Query) []Event {
	if r == nil {
		return nil
	}
	items := r.Snapshot(0)
	limit := query.Limit
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}

	severitySet := make(map[Severity]struct{}, len(query.Severity))
	for _, severity := range query.Severity {
		severitySet[severity] = struct{}{}
	}

	out := make([]Event, 0, limit)
	for i := len(items) - 1; i >= 0; i-- {
		event := items[i]
		if !eventMatchesQuery(event, query, severitySet) {
			continue
		}
		out = append(out, event)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func eventMatchesQuery(event Event, query Query, severitySet map[Severity]struct{}) bool {
	if query.Component != "" && !strings.EqualFold(event.Component, query.Component) {
		return false
	}
	if len(severitySet) > 0 {
		if _, ok := severitySet[event.Severity]; !ok {
			return false
		}
	}
	if query.Type != "" && !strings.EqualFold(event.Type, query.Type) {
		return false
	}
	if query.ObjectID != "" && event.ObjectID != query.ObjectID {
		return false
	}
	if query.TransactionID != "" && event.TransactionID != query.TransactionID {
		return false
	}
	if query.Recovery != nil && event.Recovery != *query.Recovery {
		return false
	}
	if !query.Since.IsZero() && event.Time.Before(query.Since) {
		return false
	}
	if !query.Until.IsZero() && event.Time.After(query.Until) {
		return false
	}
	return true
}
