package lite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

// UpstreamHealthTracker is isolated from the DNS data plane. Its probes do not
// traverse the RouterForge listener, fill its cache, or touch Keenetic state.
type UpstreamHealthTracker struct {
	mu      sync.Mutex
	entries map[string]*upstreamHealthEntry
	busy    atomic.Bool
}

type upstreamHealthEntry struct {
	lastCheck      time.Time
	probeLatencyMS int64
	probeOK        bool
	probeError     string
	observations   []healthObservation
}

type healthObservation struct {
	at        time.Time
	latencyMS int64
	success   bool
}

type UpstreamHealthView struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	ProbeLatencyMS int64   `json:"probe_latency_ms"`
	LastChecked    string  `json:"last_checked,omitempty"`
	LastError      string  `json:"last_error,omitempty"`
	Requests5m     int     `json:"requests_5m"`
	Failures5m     int     `json:"failures_5m"`
	AvgLatency5mMS float64 `json:"avg_latency_5m_ms"`
	P95Latency5mMS int64   `json:"p95_latency_5m_ms"`
}

func NewUpstreamHealthTracker() *UpstreamHealthTracker {
	return &UpstreamHealthTracker{entries: make(map[string]*upstreamHealthEntry)}
}

func (h *UpstreamHealthTracker) entryLocked(id string) *upstreamHealthEntry {
	if h.entries == nil {
		h.entries = make(map[string]*upstreamHealthEntry)
	}
	e := h.entries[id]
	if e == nil {
		e = &upstreamHealthEntry{}
		h.entries[id] = e
	}
	return e
}

func (h *UpstreamHealthTracker) Observe(t Trace) {
	if h == nil || t.Internal || t.Cache || t.Used == "" || t.Used == "cache" || t.Used == "client-acl" || t.Used == "system-local" {
		return
	}
	at := t.Time
	if at.IsZero() {
		at = time.Now().UTC()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.entryLocked(t.Used)
	e.observations = append(e.observations, healthObservation{at: at, latencyMS: t.LatencyMS, success: t.Success})
	if len(e.observations) > 1024 {
		e.observations = append([]healthObservation(nil), e.observations[len(e.observations)-1024:]...)
	}
}

func (h *UpstreamHealthTracker) Snapshot(upstreams []dr.Upstream) []UpstreamHealthView {
	if h == nil {
		h = NewUpstreamHealthTracker()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now().UTC()
	out := make([]UpstreamHealthView, 0, len(upstreams))
	for _, u := range upstreams {
		v := UpstreamHealthView{ID: u.ID, Status: "UNKNOWN"}
		e := h.entries[u.ID]
		if e != nil {
			if !e.lastCheck.IsZero() {
				v.LastChecked = e.lastCheck.Format(time.RFC3339)
				v.ProbeLatencyMS = e.probeLatencyMS
				v.LastError = e.probeError
				if now.Sub(e.lastCheck) > 3*time.Minute {
					v.Status = "STALE"
				} else if e.probeOK {
					v.Status = "UP"
				} else {
					v.Status = "DOWN"
				}
			}
			cutoff := now.Add(-5 * time.Minute)
			values := make([]int64, 0, len(e.observations))
			var sum int64
			for _, item := range e.observations {
				if item.at.Before(cutoff) || item.at.After(now.Add(time.Minute)) {
					continue
				}
				v.Requests5m++
				if !item.success {
					v.Failures5m++
					continue
				}
				values = append(values, item.latencyMS)
				sum += item.latencyMS
			}
			if len(values) > 0 {
				v.AvgLatency5mMS = float64(sum) / float64(len(values))
				sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
				rank := (95*len(values)+99)/100 - 1
				v.P95Latency5mMS = values[rank]
			}
		}
		if u.Transport == "native" {
			v.Status = "UNSUPPORTED"
			v.LastError = "native selector belongs to Keenetic"
		}
		out = append(out, v)
	}
	return out
}

func (h *UpstreamHealthTracker) probeOne(ctx context.Context, u dr.Upstream, factory Factory) {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()
	var err error
	var ok bool
	if u.Transport == "native" {
		err = errors.New("native selector is not a direct DNS upstream")
	} else {
		var s interface {
			Exchange(context.Context, *dns.Msg) (*dns.Msg, error)
			Close() error
		}
		s, err = factory(u)
		if err == nil {
			defer s.Close()
			req := new(dns.Msg)
			req.SetQuestion("example.org.", dns.TypeA)
			req.RecursionDesired = true
			var answer *dns.Msg
			answer, err = s.Exchange(timeoutCtx, req)
			if err == nil {
				if answer == nil || !answer.Response || answer.Id != req.Id || len(answer.Question) != 1 || !strings.EqualFold(answer.Question[0].Name, req.Question[0].Name) || answer.Question[0].Qtype != req.Question[0].Qtype {
					err = errors.New("invalid DNS response for probe question")
				} else if answer.Rcode != dns.RcodeSuccess && answer.Rcode != dns.RcodeNameError {
					err = fmt.Errorf("DNS RCODE %d", answer.Rcode)
				} else {
					ok = true
				}
			}
		}
	}
	duration := time.Since(start).Milliseconds()
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.entryLocked(u.ID)
	e.lastCheck = time.Now().UTC()
	e.probeOK = ok
	e.probeLatencyMS = duration
	e.probeError = ""
	if err != nil {
		e.probeError = err.Error()
		if len(e.probeError) > 240 {
			e.probeError = e.probeError[:240]
		}
	}
}

// Probe does at most two independent network requests concurrently. An ongoing
// manual/automatic batch is never duplicated by a concurrent request.
func (h *UpstreamHealthTracker) Probe(ctx context.Context, upstreams []dr.Upstream, id string, factory Factory) error {
	if h == nil {
		return errors.New("health tracker not initialized")
	}
	if !h.busy.CompareAndSwap(false, true) {
		return errors.New("DNS upstream check is already running")
	}
	defer h.busy.Store(false)
	if factory == nil {
		factory = transportFactory(nil)
	}
	selected := make([]dr.Upstream, 0, len(upstreams))
	for _, u := range upstreams {
		if id == "" || u.ID == id {
			selected = append(selected, u)
		}
	}
	if id != "" && len(selected) == 0 {
		return errors.New("upstream not found")
	}
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for _, u := range selected {
		if u.Transport == "native" {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(u dr.Upstream) { defer wg.Done(); defer func() { <-sem }(); h.probeOne(ctx, u, factory) }(u)
	}
	wg.Wait()
	return ctx.Err()
}

// RunHealthChecks observes only active RouterForge configuration. It never
// starts DNS, enables the Gate or changes saved configuration.
func (m *Manager) RunHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		m.mu.Lock()
		active := m.engine != nil
		c := m.applied
		factory := m.Factory
		h := m.healthLocked()
		m.mu.Unlock()
		if active {
			_ = h.Probe(ctx, c.Upstreams, "", factory)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
