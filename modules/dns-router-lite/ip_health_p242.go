package lite

// P24.2: opt-in, asynchronous and bounded IP address health sampling.
// Never resolve names in probes, traverse the RouterForge listener, or dial
// private/special addresses. DNS requests only enqueue; no synchronous I/O.

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/modules/dns-router-lite/internal/ipintel"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

const (
	maxIPHealthEntries     = 512
	maxIPHealthQueued      = 128
	maxIPHealthShown       = 100
	ipHealthGlobalInterval = time.Second // <= one dial initiation per second
)

type ipProbeTarget struct {
	IP       netip.Addr
	Domain   string
	Port     int
	Protocol string
}

type ipHealthEntry struct {
	LastSeen    time.Time
	CheckedAt   time.Time
	LastSuccess time.Time
	DownAt      time.Time
	LastError   string
	RTT         time.Duration
	Failures    int
	Recoveries  int
	Checked     bool
	Healthy     bool
	Pending     bool
}

type IPHealthView struct {
	Address     string `json:"address"`
	Domain      string `json:"domain"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	RTTMS       int64  `json:"rtt_ms"`
	Failures    int    `json:"failures"`
	LastChecked string `json:"last_checked,omitempty"`
	LastSuccess string `json:"last_success,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	Recoveries  int    `json:"recoveries"`
	LastDown    string `json:"last_down,omitempty"`
	LastSeen    string `json:"last_seen,omitempty"`
}

type IPHealthMonitor struct {
	cfg       dr.IPHealthConfig
	mu        sync.Mutex
	entries   map[ipProbeTarget]*ipHealthEntry
	queue     chan ipProbeTarget
	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	probe     func(context.Context, ipProbeTarget) (time.Duration, error)
}

func newIPHealthMonitor(cfg *dr.IPHealthConfig) *IPHealthMonitor {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	chosen := *cfg
	if chosen.Protocol == "" {
		chosen.Protocol = "tcp"
	}
	if chosen.Port == 0 {
		chosen.Port = 443
	}
	if chosen.IntervalSeconds == 0 {
		chosen.IntervalSeconds = 60
	}
	if chosen.TimeoutMS == 0 {
		chosen.TimeoutMS = 1200
	}
	if chosen.FailureThreshold == 0 {
		chosen.FailureThreshold = 2
	}
	if chosen.RecoveryThreshold == 0 {
		chosen.RecoveryThreshold = 2
	}
	if chosen.HTTPPath == "" {
		chosen.HTTPPath = "/"
	}
	m := &IPHealthMonitor{cfg: chosen, entries: make(map[ipProbeTarget]*ipHealthEntry), queue: make(chan ipProbeTarget, maxIPHealthQueued)}
	m.probe = m.dial
	return m
}

var deniedIPv4 = func() []netip.Prefix {
	raw := []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"}
	out := make([]netip.Prefix, 0, len(raw))
	for _, s := range raw {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()
var deniedIPv6 = func() []netip.Prefix {
	raw := []string{"::/128", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8"}
	out := make([]netip.Prefix, 0, len(raw))
	for _, s := range raw {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// Filter *before* queueing and again immediately before dialing. Public IPs
// are allowed, not hostnames; this prevents DNS rebinding / resolver loops.
func ipProbePermitted(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" || ip.Is4In6() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is4() {
		for _, p := range deniedIPv4 {
			if p.Contains(ip) {
				return false
			}
		}
		return true
	}
	if ip.Is6() {
		// Only globally routable 2000::/3; exclude reserved/documentation.
		if !netip.MustParsePrefix("2000::/3").Contains(ip) {
			return false
		}
		for _, p := range deniedIPv6 {
			if p.Contains(ip) {
				return false
			}
		}
		return true
	}
	return false
}

func ipProbeNameValid(name string) bool {
	if len(name) < 1 || len(name) > 253 || strings.HasSuffix(name, ".") {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func (m *IPHealthMonitor) dial(ctx context.Context, target ipProbeTarget) (time.Duration, error) {
	if !ipProbePermitted(target.IP) {
		return 0, errors.New("address not permitted")
	}
	if target.Port < 80 || target.Port > 65535 || !ipProbeNameValid(target.Domain) {
		return 0, errors.New("invalid probe target")
	}
	start := time.Now()
	if target.Protocol == "http" || target.Protocol == "https" {
		return m.dialHTTP(ctx, target, start)
	}
	address := net.JoinHostPort(target.IP.String(), strconv.Itoa(target.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return time.Since(start), err
	}
	defer conn.Close()
	if target.Protocol == "tls" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: target.Domain, MinVersion: tls.VersionTLS12})
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return time.Since(start), err
		}
	} else if target.Protocol != "tcp" {
		return 0, errors.New("unsupported probe protocol")
	}
	return time.Since(start), nil
}

func (m *IPHealthMonitor) Start() {
	if m == nil {
		return
	}
	m.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		ticks := time.NewTicker(ipHealthGlobalInterval)
		// One shared ticker globally caps outbound initiation to 1/s.
		for i := 0; i < 2; i++ {
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticks.C:
					}
					select {
					case <-ctx.Done():
						return
					case target := <-m.queue:
						m.measure(ctx, target)
					default:
					}
				}
			}()
		}
		// Ticker must stop after all workers, and never be orphaned.
		m.wg.Add(1)
		go func() { defer m.wg.Done(); <-ctx.Done(); ticks.Stop() }()
	})
}
func (m *IPHealthMonitor) Stop() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
		m.wg.Wait()
	})
}

func (m *IPHealthMonitor) measure(ctx context.Context, target ipProbeTarget) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(m.cfg.TimeoutMS)*time.Millisecond)
	defer cancel()
	var rtt time.Duration
	var err error
	if !ipProbePermitted(target.IP) {
		err = errors.New("address not permitted")
	} else {
		rtt, err = m.probe(timeoutCtx, target)
	}
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[target]
	if e == nil {
		return
	}
	e.Pending = false
	if ctx.Err() != nil {
		return
	} // cancellations do not mark IP down
	e.CheckedAt = now
	if err != nil {
		e.Recoveries = 0
		e.Failures++
		e.LastError = err.Error()
		if len(e.LastError) > 160 {
			e.LastError = e.LastError[:160]
		}
		if e.Failures >= m.cfg.FailureThreshold {
			if !e.Checked || e.Healthy {
				e.DownAt = now
			}
			e.Checked = true
			e.Healthy = false
		}
	} else {
		e.Failures = 0
		e.LastSuccess = now
		e.LastError = ""
		e.Recoveries++
		if !e.Checked || e.Healthy || (e.Recoveries >= m.cfg.RecoveryThreshold && (e.DownAt.IsZero() || now.Sub(e.DownAt) >= time.Duration(m.cfg.MinDownSeconds)*time.Second)) {
			e.Checked = true
			e.Healthy = true
			e.DownAt = time.Time{}
		}
		if rtt > 0 {
			if e.RTT == 0 {
				e.RTT = rtt
			} else {
				e.RTT = (3*e.RTT + rtt) / 4
			}
		}
	}
}

func (m *IPHealthMonitor) Observe(q, a *dns.Msg) map[netip.Addr]ipintel.Health {
	if m == nil || q == nil || a == nil || len(q.Question) != 1 || a.Rcode != dns.RcodeSuccess || !a.Response {
		return nil
	}
	question := q.Question[0]
	if question.Qclass != dns.ClassINET || question.Qtype != dns.TypeA && question.Qtype != dns.TypeAAAA {
		return nil
	}
	domain := strings.TrimSuffix(strings.ToLower(question.Name), ".")
	if !ipProbeNameValid(domain) {
		return nil
	}
	now := time.Now().UTC()
	result := make(map[netip.Addr]ipintel.Health)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rr := range a.Answer {
		ip, ok := responseIP(rr, question.Qtype)
		if !ok || !ipProbePermitted(ip) {
			continue
		}
		target := ipProbeTarget{IP: ip, Domain: domain, Port: m.cfg.Port, Protocol: m.cfg.Protocol}
		e := m.entries[target]
		if e == nil {
			if len(m.entries) >= maxIPHealthEntries {
				// Never evict an in-flight target; discard only stale data.
				for key, v := range m.entries {
					if !v.Pending && now.Sub(v.LastSeen) > 15*time.Minute {
						delete(m.entries, key)
					}
				}
			}
			if len(m.entries) >= maxIPHealthEntries {
				continue
			}
			e = &ipHealthEntry{}
			m.entries[target] = e
		}
		e.LastSeen = now
		freshness := time.Duration(m.cfg.IntervalSeconds*3) * time.Second
		if e.Checked && !e.CheckedAt.IsZero() && now.Sub(e.CheckedAt) < freshness {
			// Unknown samples never become false DOWN observations.
			weight := 1
			if e.Healthy && e.RTT > 0 {
				weight = int((200 * time.Millisecond) / e.RTT)
				if weight < 1 {
					weight = 1
				}
				if weight > 100 {
					weight = 100
				}
			}
			result[ip] = ipintel.Health{Checked: true, Healthy: e.Healthy, RTT: e.RTT, Weight: weight}
		}
		if !e.Pending && (e.CheckedAt.IsZero() || now.Sub(e.CheckedAt) >= time.Duration(m.cfg.IntervalSeconds)*time.Second) {
			select {
			case m.queue <- target:
				e.Pending = true
			default:
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (m *IPHealthMonitor) Snapshot() []IPHealthView {
	if m == nil {
		return []IPHealthView{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	entries := make([]struct {
		target ipProbeTarget
		entry  ipHealthEntry
	}, 0, len(m.entries))
	for k, e := range m.entries {
		entries = append(entries, struct {
			target ipProbeTarget
			entry  ipHealthEntry
		}{k, *e})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].entry.LastSeen.After(entries[j].entry.LastSeen) })
	if len(entries) > maxIPHealthShown {
		entries = entries[:maxIPHealthShown]
	}
	out := make([]IPHealthView, 0, len(entries))
	for _, item := range entries {
		e := item.entry
		status := "UNKNOWN"
		if e.Pending {
			status = "PENDING"
		}
		if e.Checked {
			freshness := time.Duration(m.cfg.IntervalSeconds*3) * time.Second
			if now.Sub(e.CheckedAt) >= freshness {
				status = "STALE"
			} else if e.Healthy {
				status = "UP"
			} else {
				status = "DOWN"
			}
		}
		view := IPHealthView{Address: item.target.IP.String(), Domain: item.target.Domain, Port: item.target.Port, Protocol: item.target.Protocol, Status: status, RTTMS: e.RTT.Milliseconds(), Failures: e.Failures, Recoveries: e.Recoveries}
		if !e.DownAt.IsZero() {
			view.LastDown = e.DownAt.Format(time.RFC3339)
		}
		if !e.LastSeen.IsZero() {
			view.LastSeen = e.LastSeen.Format(time.RFC3339)
		}
		if !e.CheckedAt.IsZero() {
			view.LastChecked = e.CheckedAt.Format(time.RFC3339)
		}
		if !e.LastSuccess.IsZero() {
			view.LastSuccess = e.LastSuccess.Format(time.RFC3339)
		}
		view.LastError = e.LastError
		out = append(out, view)
	}
	return out
}
