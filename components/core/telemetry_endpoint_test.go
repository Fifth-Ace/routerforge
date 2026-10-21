package main

import (
	"net/url"
	"testing"
)

// This endpoint is deliberately pinned for the T4 dev rollout. Changing the
// destination requires a new review, not an unexamined packaging side effect.
func TestTelemetryT4CollectorEndpoint(t *testing.T) {
	const want = "https://telemetry.fifthace.ru/v1/events"
	if telemetryCollectorEndpoint != want {
		t.Fatalf("collector endpoint changed unexpectedly: %q", telemetryCollectorEndpoint)
	}
	u, err := url.Parse(telemetryCollectorEndpoint)
	if err != nil || u.Scheme != "https" || u.Host != "telemetry.fifthace.ru" ||
		u.Path != "/v1/events" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		t.Fatalf("unsafe collector endpoint: %q (%v)", telemetryCollectorEndpoint, err)
	}
}
