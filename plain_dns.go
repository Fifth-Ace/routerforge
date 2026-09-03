package main

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	plainDNSTimeout         = 8 * time.Second
	plainDNSForwardSuppress = 2 * time.Second
)

type PlainDNSMeta struct {
	Address   string   `json:"address"`
	Port      uint16   `json:"port"`
	Name      string   `json:"name"`
	Profiles  []string `json:"profiles,omitempty"`
	Domains   []string `json:"domains,omitempty"`
	Source    string   `json:"source,omitempty"`
	Interface string   `json:"interface,omitempty"`
}

type plainDNSPendingKey struct {
	Protocol   string
	RemoteIP   string
	RemotePort uint16
	LocalPort  uint16
	ID         uint16
}

type plainDNSPending struct {
	Time   time.Time
	Domain string
	QType  uint16
}

type plainDNSForwardKey struct {
	Protocol   string
	RemoteIP   string
	RemotePort uint16
	ID         uint16
	Domain     string
	QType      uint16
}

type plainDNSForwardMark struct {
	SeenAt time.Time
	Count  uint32
}

type PlainDNSEvent struct {
	Time      time.Time `json:"time"`
	Protocol  string    `json:"protocol"`
	Resolver  string    `json:"resolver"`
	Port      uint16    `json:"port"`
	Domain    string    `json:"domain"`
	QType     string    `json:"qtype"`
	RCode     string    `json:"rcode,omitempty"`
	LatencyMS float64   `json:"latency_ms,omitempty"`
	Status    string    `json:"status"`
}

type plainDNSResolverState struct {
	Meta         PlainDNSMeta
	Requests     uint64
	Responses    uint64
	Errors       uint64
	Timeouts     uint64
	NXDomain     uint64
	LatencySumMS float64
	LatencyCount uint64
	LatenciesMS  []float64
	LastRequest  time.Time
	LastResponse time.Time
	LastLatency  float64
}

type PlainDNSResolverView struct {
	PlainDNSMeta
	Requests     uint64    `json:"requests"`
	Responses    uint64    `json:"responses"`
	Errors       uint64    `json:"errors"`
	Timeouts     uint64    `json:"timeouts"`
	NXDomain     uint64    `json:"nxdomain"`
	AvgLatencyMS float64   `json:"avg_latency_ms"`
	P95LatencyMS float64   `json:"p95_latency_ms"`
	LastLatency  float64   `json:"last_latency_ms"`
	LastRequest  time.Time `json:"last_request,omitempty"`
	LastResponse time.Time `json:"last_response,omitempty"`
}

type PlainDNSSnapshot struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Resolvers   []PlainDNSResolverView `json:"resolvers"`
	Recent      []PlainDNSEvent        `json:"recent"`
	Pending     int                    `json:"pending"`
	Mode        string                 `json:"mode"`
	Note        string                 `json:"note"`
}

type plainDNSTracker struct {
	mu        sync.RWMutex
	resolvers map[string]*plainDNSResolverState
	pending   map[plainDNSPendingKey]plainDNSPending
	forwarded map[plainDNSForwardKey]plainDNSForwardMark
	recent    []PlainDNSEvent
	recentCap int
}

var plainDNS = newPlainDNSTracker(500)

func newPlainDNSTracker(recentCap int) *plainDNSTracker {
	if recentCap < 1 {
		recentCap = 100
	}
	return &plainDNSTracker{
		resolvers: make(map[string]*plainDNSResolverState),
		pending:   make(map[plainDNSPendingKey]plainDNSPending),
		forwarded: make(map[plainDNSForwardKey]plainDNSForwardMark),
		recentCap: recentCap,
	}
}

func plainDNSEndpoint(address string, port uint16) string {
	return net.JoinHostPort(canonicalIP(address), strconv.Itoa(int(port)))
}

func (t *plainDNSTracker) UpdateResolvers(items []PlainDNSMeta) {
	t.mu.Lock()
	defer t.mu.Unlock()

	next := make(map[string]*plainDNSResolverState, len(items))
	for _, meta := range items {
		if meta.Port == 0 {
			meta.Port = 53
		}
		meta.Address = canonicalIP(meta.Address)
		if meta.Address == "" {
			continue
		}
		meta.Profiles = uniqueSorted(meta.Profiles)
		meta.Domains = uniqueSorted(meta.Domains)
		if meta.Name == "" {
			meta.Name = friendlyPlainDNSName(meta.Address)
		}

		key := plainDNSEndpoint(meta.Address, meta.Port)
		if existing, ok := next[key]; ok {
			existing.Meta.Profiles = uniqueSorted(append(existing.Meta.Profiles, meta.Profiles...))
			existing.Meta.Domains = uniqueSorted(append(existing.Meta.Domains, meta.Domains...))
			if existing.Meta.Source == "" {
				existing.Meta.Source = meta.Source
			}
			if existing.Meta.Interface == "" {
				existing.Meta.Interface = meta.Interface
			}
			continue
		}
		if previous, ok := t.resolvers[key]; ok {
			clone := *previous
			clone.Meta = meta
			next[key] = &clone
		} else {
			next[key] = &plainDNSResolverState{Meta: meta}
		}
	}
	t.resolvers = next

	for key := range t.pending {
		if _, ok := next[plainDNSEndpoint(key.RemoteIP, key.RemotePort)]; !ok {
			delete(t.pending, key)
		}
	}
}

func (t *plainDNSTracker) IsResolver(address string, port uint16) bool {
	address = canonicalIP(address)
	if address == "" {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.resolvers[plainDNSEndpoint(address, port)]
	return ok
}

func (t *plainDNSTracker) Observe(now time.Time, protocol, srcIP, dstIP string, srcPort, dstPort uint16, outgoing bool, msg DNSMessage) bool {
	if outgoing && dstPort == 53 && !msg.QR && t.IsResolver(dstIP, dstPort) {
		t.RecordQuery(now, protocol, dstIP, dstPort, srcPort, msg)
		return true
	}
	if !outgoing && srcPort == 53 && msg.QR && t.IsResolver(srcIP, srcPort) {
		t.RecordResponse(now, protocol, srcIP, srcPort, dstPort, msg)
		return true
	}
	return false
}

// ObserveDirectClientQuery marks a client-originated query that targets one of
// the same public resolver addresses configured in Keenetic. AF_PACKET sees the
// NATed egress packet too; the marker prevents that forwarded client flow from
// being miscounted as a router DNS-proxy upstream query.
func (t *plainDNSTracker) ObserveDirectClientQuery(now time.Time, protocol, remoteIP string, remotePort uint16, msg DNSMessage) {
	remoteIP = canonicalIP(remoteIP)
	if remoteIP == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.resolvers[plainDNSEndpoint(remoteIP, remotePort)]; !ok {
		return
	}
	key := plainDNSForwardKey{
		Protocol: strings.ToUpper(protocol), RemoteIP: remoteIP,
		RemotePort: remotePort, ID: msg.ID, Domain: msg.QName, QType: msg.QType,
	}
	mark := t.forwarded[key]
	mark.SeenAt = now
	if mark.Count < ^uint32(0) {
		mark.Count++
	}
	t.forwarded[key] = mark
}

func (t *plainDNSTracker) RecordQuery(now time.Time, protocol, remoteIP string, remotePort, localPort uint16, msg DNSMessage) bool {
	remoteIP = canonicalIP(remoteIP)
	endpoint := plainDNSEndpoint(remoteIP, remotePort)

	t.mu.Lock()
	defer t.mu.Unlock()

	resolver, ok := t.resolvers[endpoint]
	if !ok {
		return false
	}

	forwardKey := plainDNSForwardKey{
		Protocol: strings.ToUpper(protocol), RemoteIP: remoteIP,
		RemotePort: remotePort, ID: msg.ID, Domain: msg.QName, QType: msg.QType,
	}
	if mark, ok := t.forwarded[forwardKey]; ok && now.Sub(mark.SeenAt) <= plainDNSForwardSuppress && mark.Count > 0 {
		mark.Count--
		if mark.Count == 0 {
			delete(t.forwarded, forwardKey)
		} else {
			t.forwarded[forwardKey] = mark
		}
		return false
	}

	key := plainDNSPendingKey{
		Protocol: strings.ToUpper(protocol), RemoteIP: remoteIP,
		RemotePort: remotePort, LocalPort: localPort, ID: msg.ID,
	}
	if previous, exists := t.pending[key]; exists &&
		previous.Domain == msg.QName && previous.QType == msg.QType && now.Sub(previous.Time) <= 250*time.Millisecond {
		// AF_PACKET may expose the same egress frame through more than one
		// logical bridge/netdev view. Do not double-count an immediate duplicate.
		return false
	}
	t.pending[key] = plainDNSPending{Time: now, Domain: msg.QName, QType: msg.QType}
	resolver.Requests++
	resolver.LastRequest = now
	return true
}

func (t *plainDNSTracker) RecordResponse(now time.Time, protocol, remoteIP string, remotePort, localPort uint16, msg DNSMessage) bool {
	remoteIP = canonicalIP(remoteIP)
	endpoint := plainDNSEndpoint(remoteIP, remotePort)

	t.mu.Lock()
	defer t.mu.Unlock()

	resolver, ok := t.resolvers[endpoint]
	if !ok {
		return false
	}

	key := plainDNSPendingKey{
		Protocol: strings.ToUpper(protocol), RemoteIP: remoteIP,
		RemotePort: remotePort, LocalPort: localPort, ID: msg.ID,
	}
	pending, ok := t.pending[key]
	if !ok {
		return false
	}
	delete(t.pending, key)

	latency := now.Sub(pending.Time).Seconds() * 1000
	if latency < 0 {
		latency = 0
	}

	resolver.Responses++
	resolver.LastResponse = now
	resolver.LastLatency = latency
	resolver.LatencySumMS += latency
	resolver.LatencyCount++
	resolver.LatenciesMS = append(resolver.LatenciesMS, latency)
	if len(resolver.LatenciesMS) > 256 {
		resolver.LatenciesMS = resolver.LatenciesMS[len(resolver.LatenciesMS)-256:]
	}

	if msg.RCode == 3 {
		resolver.NXDomain++
	} else if msg.RCode != 0 {
		resolver.Errors++
	}

	t.appendRecentLocked(PlainDNSEvent{
		Time: now, Protocol: strings.ToUpper(protocol), Resolver: remoteIP,
		Port: remotePort, Domain: pending.Domain, QType: qtypeName(pending.QType),
		RCode: rcodeName(msg.RCode), LatencyMS: latency, Status: "ANSWER",
	})
	return true
}

func (t *plainDNSTracker) Sweep(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for key, pending := range t.pending {
		if now.Sub(pending.Time) < plainDNSTimeout {
			continue
		}
		delete(t.pending, key)
		if resolver := t.resolvers[plainDNSEndpoint(key.RemoteIP, key.RemotePort)]; resolver != nil {
			resolver.Timeouts++
		}
		t.appendRecentLocked(PlainDNSEvent{
			Time: now, Protocol: key.Protocol, Resolver: key.RemoteIP,
			Port: key.RemotePort, Domain: pending.Domain, QType: qtypeName(pending.QType),
			Status: "TIMEOUT",
		})
	}

	for key, mark := range t.forwarded {
		if now.Sub(mark.SeenAt) > plainDNSForwardSuppress {
			delete(t.forwarded, key)
		}
	}
}

func (t *plainDNSTracker) Snapshot(limit int) PlainDNSSnapshot {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	resolvers := make([]PlainDNSResolverView, 0, len(t.resolvers))
	for _, state := range t.resolvers {
		view := PlainDNSResolverView{
			PlainDNSMeta: state.Meta,
			Requests:     state.Requests, Responses: state.Responses,
			Errors: state.Errors, Timeouts: state.Timeouts, NXDomain: state.NXDomain,
			LastLatency: state.LastLatency, LastRequest: state.LastRequest, LastResponse: state.LastResponse,
		}
		if state.LatencyCount > 0 {
			view.AvgLatencyMS = state.LatencySumMS / float64(state.LatencyCount)
		}
		view.P95LatencyMS = percentile95(state.LatenciesMS)
		resolvers = append(resolvers, view)
	}

	sort.Slice(resolvers, func(i, j int) bool {
		if resolvers[i].Address != resolvers[j].Address {
			return resolvers[i].Address < resolvers[j].Address
		}
		return resolvers[i].Port < resolvers[j].Port
	})

	recent := append([]PlainDNSEvent(nil), t.recent...)
	if len(recent) > limit {
		recent = recent[len(recent)-limit:]
	}
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}

	return PlainDNSSnapshot{
		GeneratedAt: time.Now(), Resolvers: resolvers, Recent: recent,
		Pending: len(t.pending),
		Mode:    "configured-upstream-egress-observation",
		Note:    "UDP/TCP :53 отслеживается для resolver-адресов из show dns-proxy и show ip name-server. Прямые клиентские запросы к тем же resolver подавляются best-effort до NAT.",
	}
}

func (t *plainDNSTracker) appendRecentLocked(event PlainDNSEvent) {
	t.recent = append(t.recent, event)
	if len(t.recent) > t.recentCap {
		t.recent = append([]PlainDNSEvent(nil), t.recent[len(t.recent)-t.recentCap:]...)
	}
}

func percentile95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	index := (95*len(copyValues) + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(copyValues) {
		index = len(copyValues)
	}
	return copyValues[index-1]
}

func canonicalIP(value string) string {
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func friendlyPlainDNSName(address string) string {
	switch canonicalIP(address) {
	case "8.8.8.8", "8.8.4.4", "2001:4860:4860::8888", "2001:4860:4860::8844":
		return "Google DNS"
	case "1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001":
		return "Cloudflare DNS"
	case "9.9.9.9", "149.112.112.112", "2620:fe::fe", "2620:fe::9":
		return "Quad9 DNS"
	case "77.88.8.8", "77.88.8.1":
		return "Yandex DNS"
	default:
		return fmt.Sprintf("%s DNS", address)
	}
}

func parsePlainDNSEndpoint(token string) (string, uint16, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", 0, false
	}

	if ip := net.ParseIP(strings.Trim(token, "[]")); ip != nil {
		if ip.IsLoopback() {
			return "", 0, false
		}
		return ip.String(), 53, true
	}

	host, portText, err := net.SplitHostPort(token)
	if err != nil {
		return "", 0, false
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || ip.IsLoopback() {
		return "", 0, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}
	return ip.String(), uint16(port), true
}

func parsePlainDNSProxy(text string) []PlainDNSMeta {
	lines := strings.Split(text, "\n")
	byEndpoint := map[string]*PlainDNSMeta{}
	profile := ""

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "proxy-name:") {
			profile = strings.TrimSpace(strings.TrimPrefix(line, "proxy-name:"))
			continue
		}
		if !strings.HasPrefix(line, "dns_server") {
			continue
		}

		left := line
		if idx := strings.Index(left, "#"); idx >= 0 {
			left = strings.TrimSpace(left[:idx])
		}
		parts := strings.Fields(left)
		// Keenetic output: dns_server = ENDPOINT DOMAIN
		if len(parts) < 4 || parts[0] != "dns_server" || parts[1] != "=" {
			continue
		}

		address, port, ok := parsePlainDNSEndpoint(parts[2])
		if !ok {
			continue
		}
		domain := parts[3]
		if domain == "." {
			domain = ""
		}

		key := plainDNSEndpoint(address, port)
		item := byEndpoint[key]
		if item == nil {
			item = &PlainDNSMeta{
				Address: address, Port: port, Name: friendlyPlainDNSName(address),
			}
			byEndpoint[key] = item
		}
		if profile != "" {
			item.Profiles = append(item.Profiles, profile)
		}
		if domain != "" {
			item.Domains = append(item.Domains, domain)
		}
	}

	out := make([]PlainDNSMeta, 0, len(byEndpoint))
	for _, item := range byEndpoint {
		item.Profiles = uniqueSorted(item.Profiles)
		item.Domains = uniqueSorted(item.Domains)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Address != out[j].Address {
			return out[i].Address < out[j].Address
		}
		return out[i].Port < out[j].Port
	})
	return out
}
