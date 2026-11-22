package lite

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Verify probes the real listeners with uncached, tagged DNS queries and checks
// exact per-question upstream attribution. It deliberately proves only the
// service's direct ingress, never a selected LAN client's Keenetic profile.
func (m *Manager) Verify(expected string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, rev, e := m.Store.Load()
	if e != nil {
		return nil, e
	}
	if rev != expected || m.engine == nil || m.revision != rev {
		return nil, fmt.Errorf("exact saved DNS configuration must be active")
	}
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return nil, e
	}
	prefix := "rfdiag-" + hex.EncodeToString(nonce[:])
	type sample struct{ name, upstream string }
	samples := []sample{}
	seen := map[string]bool{}
	for _, r := range m.applied.Rules {
		if r.Match != "suffix" || seen[r.Upstream] || len(samples) >= 4 {
			continue
		}
		name := prefix + "." + r.Domain
		d, err := m.engine.policy.EvaluateDNSQuestion(name)
		if err == nil && d.Upstream == r.Upstream {
			samples = append(samples, sample{name, r.Upstream})
			seen[r.Upstream] = true
		}
	}
	defaultName := prefix + ".invalid"
	if d, _ := m.engine.policy.EvaluateDNSQuestion(defaultName); d.Matched {
		return nil, fmt.Errorf("diagnostic default name matches a policy")
	}
	samples = append(samples, sample{defaultName, m.applied.Service.DefaultUpstream})
	results := []map[string]any{}
	for _, s := range samples {
		for _, network := range []string{"udp", "tcp"} {
			name := network + "." + s.name
			q := new(dns.Msg)
			q.SetQuestion(name+".", dns.TypeA)
			a, _, err := (&dns.Client{Net: network, Timeout: 4 * time.Second}).Exchange(q, m.applied.Service.Listen)
			if err != nil {
				return nil, fmt.Errorf("%s DNS: %w", network, err)
			}
			if a.Truncated || (a.Rcode != dns.RcodeSuccess && a.Rcode != dns.RcodeNameError) {
				return nil, fmt.Errorf("DNS route %s returned RCODE %d", s.upstream, a.Rcode)
			}
			proven := false
			for _, t := range m.engine.Traces(name) {
				if !t.Cache && t.Success && t.Assigned == s.upstream && t.Used == s.upstream && t.Type == dns.TypeA && t.RCode == a.Rcode {
					proven = true
				}
			}
			if !proven {
				return nil, fmt.Errorf("ROUTE_NOT_PROVEN: %s via %s", name, s.upstream)
			}
			results = append(results, map[string]any{"name": name, "network": network, "upstream": s.upstream, "rcode": a.Rcode, "state": "PASS"})
		}
	}
	return map[string]any{"state": "PASS", "scope": "DIRECT_SERVICE_ONLY", "client_profile_verified": false, "revision": rev, "results": results, "note": strings.TrimSpace("Sampled primary upstreams and default; not every domain or LAN client.")}, nil
}
