package lite

// P28: selection is constrained to the explicit chain returned by the compiled
// RouterForge policy.  It never falls back to Keenetic or to an unlisted DNS.
import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

type upstreamSelectionP28 struct {
	mu        sync.Mutex
	round     map[string]uint64
	latency   map[string]float64
	failures  map[string]uint64
	successes map[string]uint64
}

func newUpstreamSelectionP28() *upstreamSelectionP28 {
	return &upstreamSelectionP28{round: make(map[string]uint64), latency: make(map[string]float64), failures: make(map[string]uint64), successes: make(map[string]uint64)}
}
func (s *upstreamSelectionP28) order(key, mode string, chain []dr.Upstream) []dr.Upstream {
	out := append([]dr.Upstream(nil), chain...)
	if len(out) < 2 || mode == "" || mode == "sequential" || mode == "parallel" {
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch mode {
	case "load_balance":
		pos := int(s.round[key] % uint64(len(out)))
		s.round[key]++
		out = append(out[pos:], out[:pos]...)
	case "fastest":
		// Never label an unmeasured server as failed.  Give each member a
		// chance to receive actual traffic, then prefer measured latency.
		var unknown int = -1
		for i, u := range out {
			if s.successes[u.ID]+s.failures[u.ID] == 0 {
				unknown = i
				break
			}
		}
		if unknown >= 0 {
			out[0], out[unknown] = out[unknown], out[0]
		} else {
			sort.SliceStable(out, func(i, j int) bool {
				ai := s.latency[out[i].ID] + float64(s.failures[out[i].ID])*10000
				aj := s.latency[out[j].ID] + float64(s.failures[out[j].ID])*10000
				return ai < aj
			})
		}
	}
	return out
}
func (s *upstreamSelectionP28) observe(id string, duration time.Duration, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ok {
		ms := float64(duration.Milliseconds())
		if ms < 1 {
			ms = 1
		}
		if prev, exists := s.latency[id]; exists {
			s.latency[id] = prev*0.75 + ms*0.25
		} else {
			s.latency[id] = ms
		}
		s.successes[id]++
		if s.failures[id] > 0 {
			s.failures[id]--
		}
	} else {
		s.failures[id]++
	}
}
func (s *upstreamSelectionP28) snapshot() map[string]any {
	if s == nil {
		return map[string]any{"enabled": false}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := make(map[string]any, len(s.successes)+len(s.failures))
	for id, ms := range s.latency {
		r[id] = map[string]any{"avg_rtt_ms": ms, "successes": s.successes[id], "recent_failures": s.failures[id]}
	}
	return map[string]any{"enabled": true, "upstreams": r}
}

func p28ResponseValid(q, a *dns.Msg) error {
	if a == nil || !a.Response || a.Id != q.Id || len(a.Question) != 1 || len(q.Question) != 1 || !strings.EqualFold(a.Question[0].Name, q.Question[0].Name) || a.Question[0].Qtype != q.Question[0].Qtype || a.Question[0].Qclass != q.Question[0].Qclass {
		return errors.New("DNS response does not match the question")
	}
	return nil
}

type p28ExchangeResult struct {
	answer  *dns.Msg
	err     error
	server  dr.Upstream
	latency time.Duration
}

func (e *Engine) exchangeOneP28(ctx context.Context, q *dns.Msg, u dr.Upstream, timeout time.Duration) p28ExchangeResult {
	start := time.Now()
	server := e.servers[u.ID]
	if server == nil {
		return p28ExchangeResult{server: u, err: errors.New("selected server is unavailable")}
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request := e.upstreamRequestP30(q)
	answer, err := server.Exchange(attemptCtx, request)
	if err == nil {
		err = p28ResponseValid(request, answer)
	}
	if err == nil {
		err = e.upstreamDNSSECCheckP30(request, answer)
	}
	if err == nil {
		answer, _ = e.protection.convertBogusNXDomainP27(request, answer)
	}
	return p28ExchangeResult{answer: answer, err: err, server: u, latency: time.Since(start)}
}
func (e *Engine) recordSelectionP28(ctx context.Context, q *dns.Msg, d dr.Decision, primary dr.Upstream, r p28ExchangeResult) {
	internal, _ := ctx.Value(internalProbeKey{}).(bool)
	// Losing parallel requests are cancelled deliberately; they are not
	// reported as unreachable servers or poison the latency estimator.
	if errors.Is(r.err, context.Canceled) {
		return
	}
	if e.upstreamSelect != nil {
		e.upstreamSelect.observe(r.server.ID, r.latency, r.err == nil)
	}
	client, _ := ctx.Value(requestClientKey{}).(string)
	name := strings.TrimSuffix(strings.ToLower(q.Question[0].Name), ".")
	t := Trace{Internal: internal, Time: time.Now().UTC(), Client: client, Name: name, Type: q.Question[0].Qtype, Rule: d.RuleID, Assigned: d.Upstream, Used: r.server.ID, AssignedEndpoint: traceDNSLabel(primary), UsedEndpoint: traceDNSLabel(r.server), Transport: r.server.Transport, Success: r.err == nil, LatencyMS: r.latency.Milliseconds(), RCode: -1}
	if r.answer != nil && r.err == nil {
		t.RCode = r.answer.Rcode
	}
	attachClientRoute(&t, ctx)
	e.addTrace(t)
}
func (e *Engine) exchangeSelectedP28(ctx context.Context, q *dns.Msg, d dr.Decision) (*dns.Msg, error) {
	chain, err := e.policy.UpstreamChainForDecision(d)
	if err != nil {
		return nil, err
	}
	if len(chain) == 0 {
		return nil, errors.New("empty explicit upstream chain")
	}
	mode := "sequential"
	timeout := 3 * time.Second
	if settings := e.cfg.Service.UpstreamSelection; settings != nil && settings.Enabled {
		if settings.Mode != "" {
			mode = settings.Mode
		}
		if settings.TimeoutMS != 0 {
			timeout = time.Duration(settings.TimeoutMS) * time.Millisecond
		}
	}
	key := d.GroupID + "|" + d.Upstream
	selected := e.upstreamSelect.order(key, mode, chain)
	if mode == "parallel" && len(selected) > 1 {
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		results := make(chan p28ExchangeResult, len(selected))
		for _, u := range selected {
			go func(u dr.Upstream) { results <- e.exchangeOneP28(runCtx, q, u, timeout) }(u)
		}
		var last error
		for i := 0; i < len(selected); i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case r := <-results:
				if r.err == nil {
					cancel()
					e.recordSelectionP28(ctx, q, d, chain[0], r)
					return r.answer, nil // includes NXDOMAIN: no cross-resolver leakage
				}
				if !errors.Is(r.err, context.Canceled) {
					e.recordSelectionP28(ctx, q, d, chain[0], r)
					last = r.err
				}
			}
		}
		if last == nil {
			last = errors.New("no upstream returned a matching DNS answer")
		}
		e.failures.Add(1)
		return nil, fmt.Errorf("explicit parallel DNS chain failed: %w", last)
	}
	var last error
	for _, u := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := e.exchangeOneP28(ctx, q, u, timeout)
		e.recordSelectionP28(ctx, q, d, chain[0], r)
		if r.err == nil {
			return r.answer, nil
		}
		last = r.err
	}
	e.failures.Add(1)
	return nil, fmt.Errorf("explicit DNS chain failed: %w", last)
}
