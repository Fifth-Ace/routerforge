package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

const (
	eventEngineCapacity     = 512
	eventEngineDefaultLimit = 100
	eventEngineMaxLimit     = 256
)

var coreEventEngine = platformevents.NewRing(eventEngineCapacity)

type eventTimelineResponse struct {
	APIVersion    int                    `json:"api_version"`
	Mode          string                 `json:"mode"`
	Order         string                 `json:"order"`
	Capacity      int                    `json:"capacity"`
	TotalBuffered int                    `json:"total_buffered"`
	Count         int                    `json:"count"`
	Limit         int                    `json:"limit"`
	SeverityCount map[string]int         `json:"severity_count"`
	Events        []platformevents.Event `json:"events"`
}

func emitPlatformEvent(event platformevents.Event) {
	coreEventEngine.Add(event)
}

func registerEventEngineHandlers(mux *http.ServeMux, version string) {
	emitPlatformEvent(platformevents.Event{
		Component: "core",
		Severity:  platformevents.Info,
		Type:      "core.ready",
		Message:   "RouterForge Core event engine ready",
		ObjectID:  "core",
		Context: map[string]any{
			"version": version,
		},
	})
	startDeviceEventObserver()
	mux.HandleFunc("/api/platform/events", handleEventTimeline)
	mux.HandleFunc("/api/platform/alerts", handleHealthAlerts)
}

func handleEventTimeline(w http.ResponseWriter, r *http.Request) {
	handleEventTimelineWithRing(w, r, coreEventEngine)
}

func handleEventTimelineWithRing(w http.ResponseWriter, r *http.Request, ring *platformevents.Ring) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeEventTimelineError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	query, limit, err := eventQueryFromRequest(r)
	if err != nil {
		writeEventTimelineError(w, http.StatusBadRequest, err.Error())
		return
	}
	query.Limit = limit
	items := ring.Query(query)

	severityCount := map[string]int{
		string(platformevents.Info):     0,
		string(platformevents.Warning):  0,
		string(platformevents.Critical): 0,
	}
	for _, event := range items {
		severityCount[string(event.Severity)]++
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(eventTimelineResponse{
		APIVersion:    1,
		Mode:          "read-only",
		Order:         "newest-first",
		Capacity:      ring.Capacity(),
		TotalBuffered: ring.Len(),
		Count:         len(items),
		Limit:         limit,
		SeverityCount: severityCount,
		Events:        items,
	})
}

type healthAlert struct {
	ID         string                  `json:"id"`
	Component  string                  `json:"component"`
	Severity   platformevents.Severity `json:"severity"`
	State      string                  `json:"state"`
	Message    string                  `json:"message"`
	Since      time.Time               `json:"since"`
	AgeSeconds int64                   `json:"age_seconds"`
	Context    map[string]any          `json:"context,omitempty"`
}

type healthAlertsResponse struct {
	APIVersion    int           `json:"api_version"`
	Mode          string        `json:"mode"`
	Status        string        `json:"status"`
	ActiveCount   int           `json:"active_count"`
	WarningCount  int           `json:"warning_count"`
	CriticalCount int           `json:"critical_count"`
	Alerts        []healthAlert `json:"alerts"`
}

func handleHealthAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeEventTimelineError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	now := time.Now().UTC()
	alerts := snapshotActiveModuleHealthAlerts(now)
	alerts = append(alerts, snapshotDeviceHealthAlerts(now)...)
	response := healthAlertsResponse{
		APIVersion:  1,
		Mode:        "read-only",
		Status:      "ok",
		ActiveCount: len(alerts),
		Alerts:      alerts,
	}
	if len(alerts) > 0 {
		response.Status = "degraded"
	}
	for _, alert := range alerts {
		switch alert.Severity {
		case platformevents.Critical:
			response.CriticalCount++
		default:
			response.WarningCount++
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(response)
}

func eventQueryFromRequest(r *http.Request) (platformevents.Query, int, error) {
	values := r.URL.Query()
	limit := eventEngineDefaultLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return platformevents.Query{}, 0, fmt.Errorf("limit must be a positive integer")
		}
		if parsed > eventEngineMaxLimit {
			parsed = eventEngineMaxLimit
		}
		limit = parsed
	}

	severities, err := parseEventSeverities(values.Get("severity"))
	if err != nil {
		return platformevents.Query{}, 0, err
	}

	var recovery *bool
	if raw := strings.TrimSpace(values.Get("recovery")); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return platformevents.Query{}, 0, fmt.Errorf("recovery must be true or false")
		}
		recovery = &parsed
	}

	since, err := parseOptionalEventTime(values.Get("since"), "since")
	if err != nil {
		return platformevents.Query{}, 0, err
	}
	until, err := parseOptionalEventTime(values.Get("until"), "until")
	if err != nil {
		return platformevents.Query{}, 0, err
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		return platformevents.Query{}, 0, fmt.Errorf("since must not be after until")
	}

	return platformevents.Query{
		Component:     strings.TrimSpace(values.Get("component")),
		Severity:      severities,
		Type:          strings.TrimSpace(values.Get("type")),
		ObjectID:      strings.TrimSpace(values.Get("object_id")),
		TransactionID: strings.TrimSpace(values.Get("transaction_id")),
		Recovery:      recovery,
		Since:         since,
		Until:         until,
	}, limit, nil
}

func parseEventSeverities(raw string) ([]platformevents.Severity, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	seen := map[platformevents.Severity]bool{}
	out := make([]platformevents.Severity, 0, 3)
	for _, part := range strings.Split(raw, ",") {
		severity := platformevents.Severity(strings.ToLower(strings.TrimSpace(part)))
		switch severity {
		case platformevents.Info, platformevents.Warning, platformevents.Critical:
		default:
			return nil, fmt.Errorf("unsupported severity %q", part)
		}
		if !seen[severity] {
			seen[severity] = true
			out = append(out, severity)
		}
	}
	return out, nil
}

func parseOptionalEventTime(raw string, name string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339", name)
	}
	return value, nil
}

func writeEventTimelineError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
