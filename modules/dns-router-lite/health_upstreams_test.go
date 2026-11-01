package lite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

type fakeHealthDNS struct {
	err      error
	mismatch bool
}

func (f *fakeHealthDNS) Address() string { return "health-test" }
func (f *fakeHealthDNS) Close() error    { return nil }
func (f *fakeHealthDNS) Exchange(_ context.Context, q *dns.Msg) (*dns.Msg, error) {
	if f.err != nil {
		return nil, f.err
	}
	a := new(dns.Msg)
	a.SetReply(q)
	if f.mismatch {
		a.Id++
	}
	return a, nil
}
func TestP18HealthProbeAndRollingView(t *testing.T) {
	h := NewUpstreamHealthTracker()
	ups := []dr.Upstream{{ID: "a", Transport: "udp", Address: "9.9.9.9:53"}, {ID: "b", Transport: "doh", Address: "1.1.1.1:443"}}
	factory := func(u dr.Upstream) (upstream.Upstream, error) {
		if u.ID == "b" {
			return &fakeHealthDNS{err: errors.New("simulated timeout")}, nil
		}
		return &fakeHealthDNS{}, nil
	}
	if err := h.Probe(context.Background(), ups, "", factory); err != nil {
		t.Fatal(err)
	}
	h.Observe(Trace{Time: time.Now(), Used: "a", Success: true, LatencyMS: 12})
	h.Observe(Trace{Time: time.Now(), Used: "a", Success: true, LatencyMS: 21})
	h.Observe(Trace{Time: time.Now(), Used: "a", Success: false, LatencyMS: 35})
	h.Observe(Trace{Time: time.Now().Add(-6 * time.Minute), Used: "a", Success: true, LatencyMS: 3000})
	h.Observe(Trace{Time: time.Now(), Used: "a", Cache: true, Success: true, LatencyMS: 0})
	out := h.Snapshot(ups)
	if out[0].Status != "UP" || out[0].Requests5m != 3 || out[0].Failures5m != 1 || out[0].P95Latency5mMS != 21 {
		t.Fatalf("unexpected healthy view: %+v", out[0])
	}
	if out[1].Status != "DOWN" || !strings.Contains(out[1].LastError, "simulated timeout") {
		t.Fatalf("unexpected down view: %+v", out[1])
	}
	if err := h.Probe(context.Background(), ups, "missing", factory); err == nil {
		t.Fatal("unknown upstream accepted")
	}
}
func TestP18HealthBadResponseAndNoFalseHealthy(t *testing.T) {
	h := NewUpstreamHealthTracker()
	ups := []dr.Upstream{{ID: "a", Transport: "udp", Address: "127.0.0.1:53"}}
	if s := h.Snapshot(ups)[0]; s.Status != "UNKNOWN" {
		t.Fatalf("status before probe = %s", s.Status)
	}
	if err := h.Probe(context.Background(), ups, "a", func(dr.Upstream) (upstream.Upstream, error) { return &fakeHealthDNS{mismatch: true}, nil }); err != nil {
		t.Fatal(err)
	}
	if s := h.Snapshot(ups)[0]; s.Status != "DOWN" || !strings.Contains(s.LastError, "invalid DNS") {
		t.Fatalf("mismatch accepted: %+v", s)
	}
}
