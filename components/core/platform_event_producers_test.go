package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

func resetPlatformEventProducerTestState() {
	coreEventEngine = platformevents.NewRing(eventEngineCapacity)
	moduleHealthProducerState.Lock()
	moduleHealthProducerState.Modules = make(map[string]moduleHealthObservation)
	moduleHealthProducerState.Unlock()
}

func TestModuleHealthProducerTransitionsAndDeduplicates(t *testing.T) {
	resetPlatformEventProducerTestState()

	observeModuleHealth("dns", false, "dial failed")
	observeModuleHealth("dns", false, "dial failed again")
	observeModuleHealth("dns", true, "200 OK")
	observeModuleHealth("dns", true, "200 OK")

	events := coreEventEngine.Query(platformevents.Query{Component: "dns", Limit: 10})
	if len(events) != 2 {
		t.Fatalf("expected exactly two transition events, got %d: %#v", len(events), events)
	}
	if events[0].Type != "module.health.recovered" || !events[0].Recovery {
		t.Fatalf("unexpected recovery event: %#v", events[0])
	}
	if events[1].Type != "module.health.down" || events[1].Recovery {
		t.Fatalf("unexpected down event: %#v", events[1])
	}
}

func TestNetworkToolsHealthUsesGenericModuleProducer(t *testing.T) {
	resetPlatformEventProducerTestState()

	resp := &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: http.NoBody}
	observeModuleProxyResponse("network-tools", "/v1/health", http.MethodGet, resp)

	events := coreEventEngine.Query(platformevents.Query{Component: "network-tools", Limit: 10})
	if len(events) != 1 || events[0].Type != "module.health.up" {
		t.Fatalf("unexpected network-tools health event: %#v", events)
	}
}

func TestAdminServiceActionProducerPreservesBodyAndCorrelatesTransaction(t *testing.T) {
	resetPlatformEventProducerTestState()

	payload := `{"ok":true,"action":"restart","id":"demo","transaction":{"id":"tx-123","component":"admin-service","state":"committed","started_at":"2026-09-17T20:00:00Z","updated_at":"2026-09-17T20:00:01Z"}}`
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		ContentLength: int64(len(payload)),
		Body:          io.NopCloser(strings.NewReader(payload)),
	}

	observeModuleProxyResponse("admin", "/v1/services/demo/action", http.MethodPost, resp)

	restored, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != payload {
		t.Fatalf("response body changed: got %q want %q", restored, payload)
	}

	events := coreEventEngine.Query(platformevents.Query{ObjectID: "demo", Limit: 10})
	if len(events) != 2 {
		t.Fatalf("expected service and transaction events, got %d: %#v", len(events), events)
	}
	if events[0].Type != "transaction.committed" || events[0].TransactionID != "tx-123" {
		t.Fatalf("unexpected transaction event: %#v", events[0])
	}
	if events[1].Type != "service.action.committed" || events[1].TransactionID != "tx-123" {
		t.Fatalf("unexpected service event: %#v", events[1])
	}
}

func TestAdminRejectedServiceActionProducesBoundedServiceEventWithoutTransaction(t *testing.T) {
	resetPlatformEventProducerTestState()

	payload := `{"error":"confirm_id does not match target service"}`
	resp := &http.Response{
		StatusCode:    http.StatusConflict,
		Status:        "409 Conflict",
		ContentLength: int64(len(payload)),
		Body:          io.NopCloser(strings.NewReader(payload)),
	}

	observeModuleProxyResponse("admin", "/v1/services/demo/action", http.MethodPost, resp)

	events := coreEventEngine.Query(platformevents.Query{ObjectID: "demo", Limit: 10})
	if len(events) != 1 {
		t.Fatalf("expected one rejection event, got %d: %#v", len(events), events)
	}
	if events[0].Type != "service.action.rejected" || events[0].Severity != platformevents.Warning {
		t.Fatalf("unexpected rejection event: %#v", events[0])
	}
}

func TestProducerSkipsOversizedServiceResponseWithoutConsumingBody(t *testing.T) {
	resetPlatformEventProducerTestState()

	payload := strings.Repeat("x", int(platformEventProducerResponseLimit)+1)
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		ContentLength: int64(len(payload)),
		Body:          io.NopCloser(strings.NewReader(payload)),
	}

	observeModuleProxyResponse("admin", "/v1/services/demo/action", http.MethodPost, resp)

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != payload {
		t.Fatalf("oversized response body changed")
	}
	if coreEventEngine.Len() != 0 {
		t.Fatalf("oversized response should not emit events")
	}
}
