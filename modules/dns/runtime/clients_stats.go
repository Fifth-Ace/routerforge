package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type clientState struct {
	info              ClientInfo
	requests          uint64
	matched           uint64
	forwarded         uint64
	cacheLocal        uint64
	clientResponses   uint64
	clientErrors      uint64
	clientTimeouts    uint64
	errors            uint64 // upstream DNS errors attributed to this client
	timeouts          uint64 // upstream resolver timeouts attributed to this client
	fallbacks         uint64
	lastSeen          time.Time
	domains           map[string]uint64
	resolvers         map[uint16]uint64
	clientLatencySum  float64
	clientLatencyN    uint64
	clientMaxLatency  float64
	clientLatencyHist [len(latencyBoundsMS)]uint64
}

type clientQuery struct {
	when              time.Time
	id                uint16
	domain            string
	qtype             uint16
	transport         string
	key               string
	clientIP          string
	matched           bool
	resolverPort      uint16
	fallback          bool
	upstreamRCode     string
	upstreamLatencyMS float64
	upstreamTimeout   bool
	terminal          bool
}

func clientStateKey(c ClientInfo) string {
	if strings.TrimSpace(c.MAC) != "" {
		return strings.ToLower(c.MAC)
	}
	return c.IP
}

func clientQueryKey(profile, domain string, qtype uint16) string {
	return normalizeClientPolicy(profile) + "\x00" + strings.ToLower(domain) + "\x00" + qtypeName(qtype)
}

func clientResponseKey(ip, transport string, id uint16, domain string, qtype uint16) string {
	return strings.ToLower(ip) + "\x00" + strings.ToUpper(transport) + "\x00" + fmt.Sprint(id) + "\x00" + strings.ToLower(domain) + "\x00" + qtypeName(qtype)
}

func (s *Store) UpdateClientRegistry(clients []ClientInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]ClientInfo, len(clients))
	for _, c := range clients {
		c.Policy = normalizeClientPolicy(c.Policy)
		next[c.IP] = c
		key := clientStateKey(c)
		if st := s.clientStats[key]; st != nil {
			st.info = c
		} else {
			// Keep active Keenetic clients visible even before they emit their first
			// DNS packet after a daemon restart. This also makes client drill-down
			// available for devices assigned to dedicated VPN policies while idle.
			s.clientStats[key] = &clientState{
				info:      c,
				domains:   make(map[string]uint64),
				resolvers: make(map[uint16]uint64),
			}
		}
	}
	s.clientRegistry = next
	s.clientRegistryError = ""
	s.lastClientRegistry = time.Now()
}

func (s *Store) SetClientRegistryError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientRegistryError = msg
}

func (s *Store) SetClientCaptureError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientCaptureError = msg
}

func (s *Store) ensureClientStateLocked(srcIP, srcMAC string) (*clientState, ClientInfo, string) {
	info, ok := s.clientRegistry[srcIP]
	if !ok {
		info = ClientInfo{IP: srcIP, MAC: strings.ToLower(srcMAC), Policy: "System", Access: "Unknown", Active: true}
	} else if info.MAC == "" && srcMAC != "" {
		info.MAC = strings.ToLower(srcMAC)
	}
	info.Policy = normalizeClientPolicy(info.Policy)
	key := clientStateKey(info)
	st := s.clientStats[key]
	if st == nil {
		st = &clientState{info: info, domains: make(map[string]uint64), resolvers: make(map[uint16]uint64)}
		s.clientStats[key] = st
	} else {
		st.info = info
	}
	return st, info, key
}

func (s *Store) RecordClientQuery(now time.Time, transport, srcIP, srcMAC string, id uint16, d DNSMessage) {
	if d.QName == "" || d.QR || isHealthDomain(d.QName) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dedup := fmt.Sprintf("Q|%s|%s|%d|%s|%d", transport, srcIP, id, strings.ToLower(d.QName), d.QType)
	if last, ok := s.clientDedup[dedup]; ok && now.Sub(last) < 250*time.Millisecond {
		return
	}
	s.clientDedup[dedup] = now

	st, info, key := s.ensureClientStateLocked(srcIP, srcMAC)
	st.requests++
	st.lastSeen = now
	if _, exists := st.domains[d.QName]; exists || len(st.domains) < 1000 {
		st.domains[d.QName]++
	}

	q := &clientQuery{when: now, id: id, domain: d.QName, qtype: d.QType, transport: transport, key: key, clientIP: info.IP}
	k := clientQueryKey(info.Policy, d.QName, d.QType)
	items := s.recentClients[k]
	keep := items[:0]
	for _, old := range items {
		if old != nil && now.Sub(old.when) <= 2*time.Second {
			keep = append(keep, old)
		}
	}
	s.recentClients[k] = append(keep, q)
	rk := clientResponseKey(info.IP, transport, id, d.QName, d.QType)
	s.clientPending[rk] = append(s.clientPending[rk], q)
}

func (s *Store) matchClientLocked(now time.Time, profile, domain string, qtype, id, resolverPort uint16) (string, ClientInfo, *clientQuery) {
	k := clientQueryKey(profile, domain, qtype)
	items := s.recentClients[k]
	if len(items) == 0 {
		return "", ClientInfo{}, nil
	}
	best := -1
	for i := len(items) - 1; i >= 0; i-- {
		q := items[i]
		if q == nil || q.matched || now.Sub(q.when) > 2*time.Second {
			continue
		}
		if q.id == id {
			best = i
			break
		}
		if best < 0 {
			best = i
		}
	}
	if best < 0 {
		return "", ClientInfo{}, nil
	}
	q := items[best]
	q.matched = true
	q.resolverPort = resolverPort
	clientKey := q.key
	st := s.clientStats[clientKey]
	if st == nil {
		return "", ClientInfo{}, q
	}
	st.matched++
	st.resolvers[resolverPort]++
	return clientKey, st.info, q
}

func (s *Store) recordClientErrorLocked(key string) {
	if st := s.clientStats[key]; st != nil {
		st.errors++
	}
}
func (s *Store) recordClientTimeoutLocked(key string) {
	if st := s.clientStats[key]; st != nil {
		st.timeouts++
	}
}
func (s *Store) recordClientFallbackLocked(key string) {
	if st := s.clientStats[key]; st != nil {
		st.fallbacks++
	}
}

func recordClientLatency(st *clientState, ms float64) {
	if st == nil || ms < 0 {
		return
	}
	st.clientLatencySum += ms
	st.clientLatencyN++
	if ms > st.clientMaxLatency {
		st.clientMaxLatency = ms
	}
	idx := len(latencyBoundsMS) - 1
	for i, bound := range latencyBoundsMS {
		if ms <= bound {
			idx = i
			break
		}
	}
	st.clientLatencyHist[idx]++
}

func clientP95(st *clientState) float64 {
	if st == nil || st.clientLatencyN == 0 {
		return 0
	}
	want := (st.clientLatencyN*95 + 99) / 100
	var seen uint64
	for i, c := range st.clientLatencyHist {
		seen += c
		if seen >= want {
			v := latencyBoundsMS[i]
			if v > st.clientMaxLatency {
				return st.clientMaxLatency
			}
			return v
		}
	}
	return st.clientMaxLatency
}

func (s *Store) RecordClientResponse(now time.Time, transport, clientIP, clientMAC string, id uint16, d DNSMessage) {
	if d.QName == "" || !d.QR || isHealthDomain(d.QName) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dedup := fmt.Sprintf("R|%s|%s|%d|%s|%d|%d", transport, clientIP, id, strings.ToLower(d.QName), d.QType, d.RCode)
	if last, ok := s.clientResponseDedup[dedup]; ok && now.Sub(last) < 250*time.Millisecond {
		return
	}
	s.clientResponseDedup[dedup] = now

	rk := clientResponseKey(clientIP, transport, id, d.QName, d.QType)
	items := s.clientPending[rk]
	var q *clientQuery
	for i := len(items) - 1; i >= 0; i-- {
		if items[i] != nil && !items[i].terminal && now.Sub(items[i].when) <= 15*time.Second {
			q = items[i]
			break
		}
	}
	if q == nil {
		return
	}
	q.terminal = true
	st := s.clientStats[q.key]
	if st == nil {
		return
	}
	st.clientResponses++
	st.lastSeen = now
	ms := float64(now.Sub(q.when).Microseconds()) / 1000
	recordClientLatency(st, ms)

	outcome := "CACHE_LOCAL"
	if d.RCode != 0 && d.RCode != 3 {
		outcome = "ERROR"
		st.clientErrors++
	} else if q.matched {
		outcome = "FORWARDED"
		st.forwarded++
	} else {
		st.cacheLocal++
	}

	resolver := ""
	if q.resolverPort != 0 {
		if up := s.upstreams[q.resolverPort]; up != nil {
			resolver = up.meta.Name
		} else {
			resolver = fmt.Sprintf(":%d", q.resolverPort)
		}
	}
	s.addClientFlowLocked(ClientFlowEvent{
		Time: q.when, CompletedAt: now,
		ClientIP: st.info.IP, ClientMAC: st.info.MAC, ClientName: st.info.Name, ClientHostname: st.info.Hostname,
		ClientPolicy: st.info.Policy, ClientAccess: st.info.Access,
		Domain: q.domain, QType: qtypeName(q.qtype), Transport: q.transport,
		Outcome: outcome, RCode: rcodeName(d.RCode), LatencyMS: ms,
		Resolver: resolver, ResolverPort: q.resolverPort, Fallback: q.fallback,
		UpstreamRCode: q.upstreamRCode, UpstreamLatencyMS: q.upstreamLatencyMS, UpstreamTimeout: q.upstreamTimeout,
	})

	keep := items[:0]
	for _, it := range items {
		if it != nil && !it.terminal {
			keep = append(keep, it)
		}
	}
	if len(keep) == 0 {
		delete(s.clientPending, rk)
	} else {
		s.clientPending[rk] = keep
	}
}

func (s *Store) finalizeClientTimeoutLocked(now time.Time, q *clientQuery) {
	if q == nil || q.terminal {
		return
	}
	q.terminal = true
	st := s.clientStats[q.key]
	if st == nil {
		return
	}
	st.clientTimeouts++
	st.lastSeen = now
	resolver := ""
	if q.resolverPort != 0 {
		if up := s.upstreams[q.resolverPort]; up != nil {
			resolver = up.meta.Name
		}
	}
	s.addClientFlowLocked(ClientFlowEvent{
		Time: q.when, CompletedAt: now,
		ClientIP: st.info.IP, ClientMAC: st.info.MAC, ClientName: st.info.Name, ClientHostname: st.info.Hostname,
		ClientPolicy: st.info.Policy, ClientAccess: st.info.Access,
		Domain: q.domain, QType: qtypeName(q.qtype), Transport: q.transport,
		Outcome: "CLIENT_TIMEOUT", Resolver: resolver, ResolverPort: q.resolverPort, Fallback: q.fallback,
		UpstreamRCode: q.upstreamRCode, UpstreamLatencyMS: q.upstreamLatencyMS, UpstreamTimeout: q.upstreamTimeout,
	})
}

func topNamed(counts map[string]uint64, limit int) []NamedCount {
	out := make([]NamedCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, NamedCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) clientViewLocked(st *clientState) ClientView {
	resolverNames := make(map[string]uint64)
	for port, count := range st.resolvers {
		name := fmt.Sprintf(":%d", port)
		if u := s.upstreams[port]; u != nil {
			name = u.meta.Name
		}
		resolverNames[name] += count
	}
	avg := 0.0
	if st.clientLatencyN > 0 {
		avg = st.clientLatencySum / float64(st.clientLatencyN)
	}
	route := s.policyRoutes[normalizeClientPolicy(st.info.Policy)]
	return ClientView{
		ClientInfo: st.info, Requests: st.requests, Matched: st.matched,
		Forwarded: st.forwarded, CacheLocal: st.cacheLocal, ClientResponses: st.clientResponses, ClientErrors: st.clientErrors, ClientTimeouts: st.clientTimeouts,
		Errors: st.errors, Timeouts: st.timeouts, Fallbacks: st.fallbacks,
		AvgClientLatencyMS: avg, P95ClientLatencyMS: clientP95(st), MaxClientLatencyMS: st.clientMaxLatency,
		LastSeen: st.lastSeen, TopDomains: topNamed(st.domains, 5), TopResolvers: topNamed(resolverNames, 5), Route: route,
	}
}

func (s *Store) Clients(limit int) []ClientView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ClientView, 0, len(s.clientStats))
	for _, st := range s.clientStats {
		// Show every currently active Keenetic client. Also keep inactive clients
		// that already produced DNS data during this daemon session so their
		// captured history does not disappear as soon as they disconnect.
		if !st.info.Active && st.requests == 0 {
			continue
		}
		out = append(out, s.clientViewLocked(st))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		nameI := strings.ToLower(strings.TrimSpace(out[i].Name))
		if nameI == "" {
			nameI = strings.ToLower(strings.TrimSpace(out[i].Hostname))
		}
		if nameI == "" {
			nameI = out[i].IP
		}
		nameJ := strings.ToLower(strings.TrimSpace(out[j].Name))
		if nameJ == "" {
			nameJ = strings.ToLower(strings.TrimSpace(out[j].Hostname))
		}
		if nameJ == "" {
			nameJ = out[j].IP
		}
		if nameI == nameJ {
			return out[i].IP < out[j].IP
		}
		return nameI < nameJ
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) ClientDetail(ip string, limit int) (ClientView, []ClientFlowEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st *clientState
	for _, candidate := range s.clientStats {
		if candidate.info.IP == ip {
			st = candidate
			break
		}
	}
	if st == nil {
		return ClientView{}, []ClientFlowEvent{}, false
	}
	view := s.clientViewLocked(st)
	events := s.clientFlowTailLocked(s.clientFlowCount)
	filtered := make([]ClientFlowEvent, 0, minInt(limit, len(events)))
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].ClientIP != ip {
			continue
		}
		filtered = append(filtered, events[i])
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return view, filtered, true
}

func (s *Store) Interfaces() []InterfaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type agg struct {
		v       InterfaceView
		clients map[string]uint64
	}
	m := make(map[string]*agg)
	for _, st := range s.clientStats {
		if st.requests == 0 {
			continue
		}
		accessKey := st.info.Access
		if accessKey == "" {
			accessKey = "Unknown"
		}
		a := m[accessKey]
		if a == nil {
			a = &agg{v: InterfaceView{Key: accessKey, Name: accessKey, Network: st.info.Network, SSID: st.info.SSID, AP: st.info.AP}, clients: make(map[string]uint64)}
			m[accessKey] = a
		}
		a.v.Requests += st.requests
		a.v.Errors += st.errors
		a.v.Timeouts += st.timeouts
		a.v.Fallbacks += st.fallbacks
		name := st.info.Name
		if name == "" {
			name = st.info.Hostname
		}
		if name == "" {
			name = st.info.IP
		}
		a.clients[name] += st.requests
	}
	out := make([]InterfaceView, 0, len(m))
	for _, a := range m {
		a.v.Devices = len(a.clients)
		a.v.TopClients = topNamed(a.clients, 5)
		out = append(out, a.v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Requests > out[j].Requests })
	return out
}
