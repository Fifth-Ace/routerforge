package lite

import (
	"context"
	"errors"
	"testing"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func TestP194DraftInspectReportsUnavailableWithoutMutation(t *testing.T) {
	calls := 0
	factory := func(u dr.Upstream) (upstream.Upstream, error) {
		calls++
		if u.ID != "p194-draft" || u.Address != "9.9.9.9:53" {
			t.Fatal("draft identity changed incorrectly")
		}
		return nil, errors.New("offline test upstream")
	}
	result, err := InspectDraftP194(context.Background(), dr.Upstream{ID: "original-id", Transport: "udp", Address: "9.9.9.9:53"}, factory)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || result.Status != "DOWN" || result.DNSAvailable || result.LastError == "" || result.Saved || result.Applied || result.TLSStatus != "NOT_APPLICABLE" {
		t.Fatalf("unexpected result: %+v, factory calls=%d", result, calls)
	}
}

func TestP194DraftRejectsInvalidAddressBeforeNetwork(t *testing.T) {
	calls := 0
	factory := func(u dr.Upstream) (upstream.Upstream, error) { calls++; return nil, errors.New("should not run") }
	_, err := InspectDraftP194(context.Background(), dr.Upstream{Transport: "dot", Address: "127.0.0.1:853", ServerName: "dns.example.com"}, factory)
	if err == nil || calls != 0 {
		t.Fatalf("must reject invalid endpoint without network calls: %v, %d", err, calls)
	}
}
