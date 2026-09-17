package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

func TestEventTimelineFilteringAndOrder(t *testing.T) {
	base := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	ring := platformevents.NewRing(8)
	ring.Add(platformevents.Event{Time: base, Component: "core", Severity: platformevents.Info, Type: "core.ready", Message: "ready"})
	ring.Add(platformevents.Event{Time: base.Add(time.Minute), Component: "dns", Severity: platformevents.Warning, Type: "dns.timeout", ObjectID: "policy1", Message: "timeout"})
	ring.Add(platformevents.Event{Time: base.Add(2 * time.Minute), Component: "dns", Severity: platformevents.Critical, Type: "dns.down", ObjectID: "policy1", Message: "down"})

	request := httptest.NewRequest(http.MethodGet, "/api/platform/events?component=dns&severity=warning,critical&limit=2", nil)
	response := httptest.NewRecorder()
	handleEventTimelineWithRing(response, request, ring)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", response.Code, response.Body.String())
	}

	var payload eventTimelineResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.APIVersion != 1 || payload.Mode != "read-only" || payload.Order != "newest-first" {
		t.Fatalf("unexpected contract: %#v", payload)
	}
	if payload.Capacity != 8 || payload.TotalBuffered != 3 || payload.Count != 2 || payload.Limit != 2 {
		t.Fatalf("unexpected metadata: %#v", payload)
	}
	if payload.Events[0].Type != "dns.down" || payload.Events[1].Type != "dns.timeout" {
		t.Fatalf("unexpected event order: %#v", payload.Events)
	}
	if payload.SeverityCount["critical"] != 1 || payload.SeverityCount["warning"] != 1 {
		t.Fatalf("unexpected severity counts: %#v", payload.SeverityCount)
	}
}

func TestEventTimelineRejectsMutationAndBadFilters(t *testing.T) {
	ring := platformevents.NewRing(4)

	post := httptest.NewRequest(http.MethodPost, "/api/platform/events", nil)
	postResponse := httptest.NewRecorder()
	handleEventTimelineWithRing(postResponse, post, ring)
	if postResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", postResponse.Code)
	}

	badSeverity := httptest.NewRequest(http.MethodGet, "/api/platform/events?severity=banana", nil)
	badSeverityResponse := httptest.NewRecorder()
	handleEventTimelineWithRing(badSeverityResponse, badSeverity, ring)
	if badSeverityResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad severity, got %d", badSeverityResponse.Code)
	}

	badLimit := httptest.NewRequest(http.MethodGet, "/api/platform/events?limit=0", nil)
	badLimitResponse := httptest.NewRecorder()
	handleEventTimelineWithRing(badLimitResponse, badLimit, ring)
	if badLimitResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad limit, got %d", badLimitResponse.Code)
	}
}

func TestEventTimelineLimitIsCapped(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/platform/events?limit=9999", nil)
	query, limit, err := eventQueryFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if query.Limit != 0 {
		t.Fatalf("parser should leave query limit to caller, got %d", query.Limit)
	}
	if limit != eventEngineMaxLimit {
		t.Fatalf("expected max limit %d, got %d", eventEngineMaxLimit, limit)
	}
}
