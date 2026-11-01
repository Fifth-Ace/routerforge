package lite

import (
	"context"
	"errors"
	"strings"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

// DraftInspectP194 is independent of the saved DNS configuration and running listener.
// An upstream DNS failure is a reported DOWN result, not an API failure.
type DraftInspectP194 struct {
	Status         string `json:"status"`
	DNSAvailable   bool   `json:"dns_available"`
	Address        string `json:"address"`
	Transport      string `json:"transport"`
	ProbeLatencyMS int64  `json:"probe_latency_ms"`
	LastError      string `json:"last_error,omitempty"`
	TLSStatus      string `json:"tls_status"`
	Saved          bool   `json:"saved"`
	Applied        bool   `json:"applied"`
}

func InspectDraftP194(ctx context.Context, candidate dr.Upstream, factory Factory) (DraftInspectP194, error) {
	candidate.ID = "p194-draft"
	candidate.Fallbacks = nil
	if candidate.Transport == "native" {
		return DraftInspectP194{}, errors.New("native Keenetic resolver is not a draft endpoint")
	}
	// Do not allow submitting a leaf certificate as a trust override: the
	// actual transport still validates TLS using the normal trust store.
	if _, err := dr.Compile(dr.Config{
		SchemaVersion: dr.SchemaVersion,
		Upstreams:     []dr.Upstream{candidate},
		Rules:         []dr.Rule{},
	}); err != nil {
		return DraftInspectP194{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tracker := NewUpstreamHealthTracker() // isolated from runtime health and traces
	err := tracker.Probe(ctx, []dr.Upstream{candidate}, candidate.ID, factory)
	view := tracker.Snapshot([]dr.Upstream{candidate})[0]
	result := DraftInspectP194{
		Status: view.Status, DNSAvailable: view.Status == "UP",
		Address: candidate.Address, Transport: candidate.Transport,
		ProbeLatencyMS: view.ProbeLatencyMS, LastError: view.LastError,
		TLSStatus: "NOT_APPLICABLE", Saved: false, Applied: false,
	}
	if strings.EqualFold(candidate.Transport, "dot") || strings.EqualFold(candidate.Transport, "doh") || strings.EqualFold(candidate.Transport, "doq") {
		result.TLSStatus = "UNCONFIRMED"
		if result.DNSAvailable {
			result.TLSStatus = "VERIFIED_BY_TRANSPORT"
		}
	}
	// Expired context is represented as DOWN so the user can decide whether
	// to save the inaccessible draft explicitly without changing the listener.
	if err != nil && result.Status != "DOWN" {
		result.Status = "DOWN"
		result.DNSAvailable = false
		result.LastError = err.Error()
	}
	if result.LastError == "" && !result.DNSAvailable {
		result.LastError = "no valid DNS response"
	}
	return result, nil
}
