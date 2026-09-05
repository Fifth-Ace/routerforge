package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

var latencyBoundsMS = [...]float64{10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 8000, 16000}

type upstreamState struct {
	meta               UpstreamMeta
	requests           uint64
	responses          uint64
	lateResponses      uint64
	unmatchedResponses uint64
	nxdomain           uint64
	servfail           uint64
	refused            uint64
	otherErrors        uint64
	timeouts           uint64
	fallbacks          uint64
	latencySumMS       float64
	latencyCount       uint64
	maxLatencyMS       float64
	healthStatus       string
	healthLatencyMS    float64
	lastHealthCheck    time.Time
	lastHealthError    string
	consecutiveFails   int
	lastObserved       time.Time
	observedLatencyMS  float64
	lastRequest        time.Time
	diagnostic         DiagnosticView
}

type pendingKey struct {
	proxyPort  uint16
	clientPort uint16
	id         uint16
}

type pendingQuery struct {
	when      time.Time
	domain    string
	qtype     uint16
	profile   string
	port      uint16
	clientKey string
	clientTxn *clientQuery
}

type timedOutQuery struct {
	timeoutAt time.Time
	query     pendingQuery
}

type recentQuery struct {
	when       time.Time
	port       uint16
	clientPort uint16
	id         uint16
	domain     string
	qtype      uint16
	clientKey  string
	clientTxn  *clientQuery
}

type resolverMinuteStats struct {
	requests      uint64
	responses     uint64
	lateResponses uint64
	success       uint64
	errors        uint64
	timeouts      uint64
	fallbacks     uint64
	latencySum    float64
	latencyN      uint64
	maxLatency    float64
	latencyHist   [len(latencyBoundsMS)]uint64
}

type errorBurstKey struct {
	port uint16
	kind string
}

type errorBurstMinute struct {
	count   uint64
	domains map[string]struct{}
}

type minuteBucket struct {
	minute        int64
	requests      uint64
	responses     uint64
	lateResponses uint64
	fallbacks     uint64
	errors        uint64
	timeouts      uint64
	upstreams     map[uint16]*resolverMinuteStats
	edges         map[[2]uint16]uint64
	errorBursts   map[errorBurstKey]*errorBurstMinute
}

type Store struct {
	mu                      sync.RWMutex
	started                 time.Time
	upstreams               map[uint16]*upstreamState
	pending                 map[pendingKey]pendingQuery
	timedOut                map[pendingKey]timedOutQuery
	recentByName            map[string][]recentQuery
	flow                    []FlowEvent
	flowCap                 int
	flowPos                 int
	flowCount               int
	errors                  []ErrorEvent
	errorCap                int
	domainCounts            map[string]uint64
	fallbackEdges           map[[2]uint16]uint64
	errorDedup              map[string]time.Time
	history                 []minuteBucket
	totalRequests           uint64
	totalResponses          uint64
	totalLateResponses      uint64
	totalUnmatchedResponses uint64
	totalFallbacks          uint64
	totalTimeouts           uint64
	discoveryError          string
	captureError            string
	lastDiscovery           time.Time
	clientRegistry          map[string]ClientInfo
	clientStats             map[string]*clientState
	recentClients           map[string][]*clientQuery
	clientPending           map[string][]*clientQuery
	clientDedup             map[string]time.Time
	clientResponseDedup     map[string]time.Time
	clientFlow              []ClientFlowEvent
	clientFlowCap           int
	clientFlowPos           int
	clientFlowCount         int
	policyRoutes            map[string]PolicyRouteView
	clientRegistryError     string
	clientCaptureError      string
	lastClientRegistry      time.Time
}

func NewStore(flowCap, errorCap int) *Store {
	return &Store{
		started: time.Now(), upstreams: make(map[uint16]*upstreamState), pending: make(map[pendingKey]pendingQuery), timedOut: make(map[pendingKey]timedOutQuery),
		recentByName: make(map[string][]recentQuery), flow: make([]FlowEvent, maxInt(1, flowCap)), flowCap: maxInt(1, flowCap), errorCap: errorCap,
		domainCounts: make(map[string]uint64), fallbackEdges: make(map[[2]uint16]uint64), errorDedup: make(map[string]time.Time), history: make([]minuteBucket, 1440),
		clientRegistry: make(map[string]ClientInfo), clientStats: make(map[string]*clientState), recentClients: make(map[string][]*clientQuery), clientPending: make(map[string][]*clientQuery), clientDedup: make(map[string]time.Time), clientResponseDedup: make(map[string]time.Time),
		clientFlow: make([]ClientFlowEvent, maxInt(1, flowCap)), clientFlowCap: maxInt(1, flowCap), policyRoutes: make(map[string]PolicyRouteView),
	}
}

func (s *Store) UpdateDiscovery(list []UpstreamMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	present := make(map[uint16]bool)
	for _, m := range list {
		present[m.Port] = true
		if st, ok := s.upstreams[m.Port]; ok {
			st.meta = m
		} else {
			s.upstreams[m.Port] = &upstreamState{meta: m, healthStatus: "UNKNOWN"}
		}
	}
	for p := range s.upstreams {
		if !present[p] {
			delete(s.upstreams, p)
		}
	}
	s.discoveryError = ""
	s.lastDiscovery = time.Now()
}

func (s *Store) UpdatePolicyRoutes(routes map[string]PolicyRouteView) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if routes == nil {
		return
	}
	s.policyRoutes = routes
}

func (s *Store) SetDiscoveryError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discoveryError = msg
}
func (s *Store) SetCaptureError(msg string) { s.mu.Lock(); defer s.mu.Unlock(); s.captureError = msg }

func (s *Store) MetaForPort(port uint16) (UpstreamMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.upstreams[port]
	if !ok {
		return UpstreamMeta{}, false
	}
	return st.meta, true
}

func recentKey(profile, domain string, qtype uint16) string {
	return profile + "\x00" + domain + "\x00" + qtypeName(qtype)
}

func (s *Store) historyBucketLocked(now time.Time) *minuteBucket {
	m := now.Unix() / 60
	idx := int(m % int64(len(s.history)))
	if idx < 0 {
		idx += len(s.history)
	}
	b := &s.history[idx]
	if b.minute != m {
		*b = minuteBucket{minute: m, upstreams: make(map[uint16]*resolverMinuteStats), edges: make(map[[2]uint16]uint64), errorBursts: make(map[errorBurstKey]*errorBurstMinute)}
	}
	if b.upstreams == nil {
		b.upstreams = make(map[uint16]*resolverMinuteStats)
	}
	if b.edges == nil {
		b.edges = make(map[[2]uint16]uint64)
	}
	if b.errorBursts == nil {
		b.errorBursts = make(map[errorBurstKey]*errorBurstMinute)
	}
	return b
}

func resolverBucketLocked(b *minuteBucket, port uint16) *resolverMinuteStats {
	r := b.upstreams[port]
	if r == nil {
		r = &resolverMinuteStats{}
		b.upstreams[port] = r
	}
	return r
}

func recordLatency(r *resolverMinuteStats, ms float64) {
	if ms < 0 {
		return
	}
	r.latencySum += ms
	r.latencyN++
	if ms > r.maxLatency {
		r.maxLatency = ms
	}
	idx := len(latencyBoundsMS) - 1
	for i, bound := range latencyBoundsMS {
		if ms <= bound {
			idx = i
			break
		}
	}
	r.latencyHist[idx]++
}

func (s *Store) recordBurstLocked(now time.Time, port uint16, kind, domain string) {
	b := s.historyBucketLocked(now)
	key := errorBurstKey{port: port, kind: kind}
	eb := b.errorBursts[key]
	if eb == nil {
		eb = &errorBurstMinute{domains: make(map[string]struct{})}
		b.errorBursts[key] = eb
	}
	eb.count++
	if domain != "" && len(eb.domains) < 20 {
		eb.domains[domain] = struct{}{}
	}
}

func (s *Store) RecordQuery(now time.Time, transport string, proxyPort, clientPort, id uint16, d DNSMessage) {
	if d.QName == "" || isHealthDomain(d.QName) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.upstreams[proxyPort]
	if !ok {
		st = &upstreamState{meta: UpstreamMeta{Port: proxyPort, Profile: "Unknown", Protocol: "?", Target: "unknown", Name: "Unknown local DNS proxy"}, healthStatus: "UNKNOWN"}
		s.upstreams[proxyPort] = st
	}
	st.requests++
	st.lastRequest = now
	s.totalRequests++
	b := s.historyBucketLocked(now)
	b.requests++
	resolverBucketLocked(b, proxyPort).requests++
	if _, exists := s.domainCounts[d.QName]; exists || len(s.domainCounts) < 20000 {
		s.domainCounts[d.QName]++
	}
	clientKey, clientInfo, clientTxn := s.matchClientLocked(now, st.meta.Profile, d.QName, d.QType, d.ID, proxyPort)
	pkey := pendingKey{proxyPort: proxyPort, clientPort: clientPort, id: id}
	s.pending[pkey] = pendingQuery{when: now, domain: d.QName, qtype: d.QType, profile: st.meta.Profile, port: proxyPort, clientKey: clientKey, clientTxn: clientTxn}

	fallback := false
	var fallbackFrom uint16
	k := recentKey(st.meta.Profile, d.QName, d.QType)
	proceed := st.meta.ProceedMS
	if proceed <= 0 {
		proceed = 500
	}
	old := s.recentByName[k]
	keep := old[:0]
	for _, rq := range old {
		if now.Sub(rq.when) <= 2*time.Second {
			keep = append(keep, rq)
		}
	}
	for i := len(keep) - 1; i >= 0; i-- {
		rq := keep[i]
		age := now.Sub(rq.when)
		_, sourceStillPending := s.pending[pendingKey{proxyPort: rq.port, clientPort: rq.clientPort, id: rq.id}]
		if rq.port != proxyPort && sourceStillPending && age >= time.Duration(maxInt(100, proceed-180))*time.Millisecond && age <= time.Duration(proceed+350)*time.Millisecond {
			fallback = true
			fallbackFrom = rq.port
			if clientKey == "" && rq.clientKey != "" {
				clientKey = rq.clientKey
				clientTxn = rq.clientTxn
				if cs := s.clientStats[clientKey]; cs != nil {
					clientInfo = cs.info
					cs.resolvers[proxyPort]++
				}
				pq := s.pending[pkey]
				pq.clientKey = clientKey
				pq.clientTxn = clientTxn
				s.pending[pkey] = pq
			}
			if rq.clientTxn != nil {
				rq.clientTxn.fallback = true
				rq.clientTxn.resolverPort = proxyPort
			}
			if rq.clientKey != "" {
				s.recordClientFallbackLocked(rq.clientKey)
			}
			if prev, exists := s.upstreams[rq.port]; exists {
				prev.fallbacks++
			}
			s.totalFallbacks++
			s.fallbackEdges[[2]uint16{rq.port, proxyPort}]++
			b.fallbacks++
			b.edges[[2]uint16{rq.port, proxyPort}]++
			// Resolver quality is cohort-based: attribute fallback to the original
			// request minute, not to the later fallback attempt minute.
			sourceBucket := s.historyBucketLocked(rq.when)
			resolverBucketLocked(sourceBucket, rq.port).fallbacks++
			break
		}
	}
	s.recentByName[k] = append(keep, recentQuery{when: now, port: proxyPort, clientPort: clientPort, id: id, domain: d.QName, qtype: d.QType, clientKey: clientKey, clientTxn: clientTxn})

	e := FlowEvent{Time: now, Profile: st.meta.Profile, Protocol: st.meta.Protocol, Upstream: st.meta.Name, Port: proxyPort, Domain: d.QName, QType: qtypeName(d.QType), Transport: transport, Fallback: fallback,
		ClientIP: clientInfo.IP, ClientMAC: clientInfo.MAC, ClientName: clientInfo.Name, ClientHostname: clientInfo.Hostname, ClientPolicy: clientInfo.Policy, ClientNetwork: clientInfo.Network, ClientAccess: clientInfo.Access, ClientSSID: clientInfo.SSID, ClientAP: clientInfo.AP}
	_ = fallbackFrom
	s.addFlowLocked(e)
}

func (s *Store) shouldEmitErrorLocked(now time.Time, kind string, port uint16, domain string) bool {
	key := fmt.Sprintf("%s|%d|%s", kind, port, domain)
	if last, ok := s.errorDedup[key]; ok && now.Sub(last) < 60*time.Second {
		return false
	}
	s.errorDedup[key] = now
	return true
}

func (s *Store) RecordResponse(now time.Time, proxyPort, clientPort, id uint16, d DNSMessage, log *EventLogger) {
	if isHealthDomain(d.QName) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.upstreams[proxyPort]
	if !ok {
		return
	}
	key := pendingKey{proxyPort: proxyPort, clientPort: clientPort, id: id}

	// A response that arrives after the query was already declared TIMEOUT is
	// useful telemetry, but it must not become a second successful outcome.
	if late, exists := s.timedOut[key]; exists {
		delete(s.timedOut, key)
		st.lateResponses++
		s.totalLateResponses++
		wireBucket := s.historyBucketLocked(now)
		wireBucket.lateResponses++
		queryBucket := s.historyBucketLocked(late.query.when)
		resolverBucketLocked(queryBucket, proxyPort).lateResponses++
		return
	}

	p, exists := s.pending[key]
	if !exists {
		// Duplicate/unsolicited packet. Keep it visible as a diagnostic counter,
		// but never let it distort resolver response/quality statistics.
		st.unmatchedResponses++
		s.totalUnmatchedResponses++
		return
	}
	delete(s.pending, key)

	st.responses++
	s.totalResponses++
	wireBucket := s.historyBucketLocked(now)
	wireBucket.responses++
	// Resolver window metrics are request-cohort based. This prevents a reply
	// crossing a 5m/1h boundary from appearing without its originating request.
	queryBucket := s.historyBucketLocked(p.when)
	rb := resolverBucketLocked(queryBucket, proxyPort)
	rb.responses++

	ms := float64(now.Sub(p.when).Microseconds()) / 1000
	st.latencySumMS += ms
	st.latencyCount++
	if ms > st.maxLatencyMS {
		st.maxLatencyMS = ms
	}
	recordLatency(rb, ms)
	if p.clientTxn != nil {
		p.clientTxn.upstreamRCode = rcodeName(d.RCode)
		p.clientTxn.upstreamLatencyMS = ms
	}

	if d.RCode == 0 || d.RCode == 3 {
		oldHealth := st.healthStatus
		st.lastObserved = now
		st.observedLatencyMS = ms
		st.healthStatus = "UP"
		st.consecutiveFails = 0
		st.lastHealthError = ""
		if oldHealth == "DOWN" {
			s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: proxyPort, Kind: "RECOVERED", Message: "Resolver recovered on live DNS traffic"})
			log.Event("RECOVERED", st.meta.Profile+" / "+st.meta.Name+" / live DNS traffic")
		}
	}

	if d.RCode != 0 && d.RCode != 3 && p.clientKey != "" {
		s.recordClientErrorLocked(p.clientKey)
	}

	switch d.RCode {
	case 0:
		rb.success++
	case 3:
		st.nxdomain++
		rb.success++
	case 2:
		st.servfail++
		rb.errors++
		wireBucket.errors++
		s.recordBurstLocked(now, proxyPort, "SERVFAIL", d.QName)
		if s.shouldEmitErrorLocked(now, "SERVFAIL", proxyPort, d.QName) {
			s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: proxyPort, Domain: d.QName, Kind: "SERVFAIL", Message: "DNS response SERVFAIL"})
			log.DNSError("SERVFAIL", st.meta.Profile, st.meta.Name, d.QName)
		}
	case 5:
		st.refused++
		rb.errors++
		wireBucket.errors++
		s.recordBurstLocked(now, proxyPort, "REFUSED", d.QName)
		if s.shouldEmitErrorLocked(now, "REFUSED", proxyPort, d.QName) {
			s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: proxyPort, Domain: d.QName, Kind: "REFUSED", Message: "DNS response REFUSED"})
			log.DNSError("REFUSED", st.meta.Profile, st.meta.Name, d.QName)
		}
	default:
		st.otherErrors++
		rb.errors++
		wireBucket.errors++
		kind := rcodeName(d.RCode)
		s.recordBurstLocked(now, proxyPort, kind, d.QName)
		if s.shouldEmitErrorLocked(now, kind, proxyPort, d.QName) {
			s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: proxyPort, Domain: d.QName, Kind: kind, Message: "DNS response " + kind})
			log.DNSError(kind, st.meta.Profile, st.meta.Name, d.QName)
		}
	}
}

func (s *Store) SetHealth(port uint16, now time.Time, latency float64, err error, log *EventLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.upstreams[port]
	if !ok {
		return
	}
	old := st.healthStatus
	st.lastHealthCheck = now
	if err == nil {
		st.healthLatencyMS = latency
		st.lastHealthError = ""
		st.consecutiveFails = 0
		st.healthStatus = "UP"
		if old == "DOWN" {
			s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: port, Kind: "RECOVERED", Message: "Resolver recovered"})
			log.Event("RECOVERED", st.meta.Profile+" / "+st.meta.Name)
		}
		return
	}
	st.consecutiveFails++
	st.lastHealthError = err.Error()
	if !st.lastObserved.IsZero() && now.Sub(st.lastObserved) <= 90*time.Second {
		st.healthStatus = "UP"
		return
	}
	if st.consecutiveFails >= 2 {
		st.healthStatus = "DOWN"
	} else {
		st.healthStatus = "DEGRADED"
	}
	if st.healthStatus == "DOWN" && old != "DOWN" {
		b := s.historyBucketLocked(now)
		b.errors++
		s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: port, Kind: "DOWN", Message: err.Error()})
		log.Event("DOWN", st.meta.Profile+" / "+st.meta.Name+" / "+err.Error())
	}
}

func (s *Store) SetDiagnostic(port uint16, d DiagnosticView) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.upstreams[port]; ok {
		st.diagnostic = d
	}
}

func (s *Store) DiagnosticCandidates(now time.Time) []UpstreamView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UpstreamView, 0)
	for _, st := range s.upstreams {
		if st.healthStatus != "DOWN" {
			continue
		}
		if st.diagnostic.LastRun != nil && now.Sub(*st.diagnostic.LastRun) < 5*time.Minute {
			continue
		}
		out = append(out, UpstreamView{UpstreamMeta: st.meta, HealthStatus: st.healthStatus, LastHealthError: st.lastHealthError, Diagnostic: st.diagnostic})
	}
	return out
}

func (s *Store) addErrorLocked(e ErrorEvent) { s.errors = appendRing(s.errors, e, s.errorCap) }

func healthCause(st *upstreamState) string {
	if st == nil || st.healthStatus != "DOWN" {
		return ""
	}
	switch st.diagnostic.Assessment {
	case "POLICY_ROUTE_FAIL":
		return "POLICY_ROUTE"
	case "INTERFACE_ROUTE_FAIL":
		return "INTERFACE_ROUTE"
	case "UPSTREAM_OK_LOCAL_PROXY_FAIL":
		return "LOCAL_PROXY"
	case "UPSTREAM_OK_LOCAL_PATH_FAIL":
		return "LOCAL_PATH"
	}
	if st.diagnostic.Status == "OK" {
		return "LOCAL_PATH"
	}
	if st.diagnostic.Status == "FAIL" {
		stage := st.diagnostic.Stage
		if stage == "" {
			stage = "UNKNOWN"
		}
		return "UPSTREAM_" + stage
	}
	return "UNKNOWN"
}

func timeString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func qualityStatus(w WindowStats) string {
	if w.Requests == 0 && w.Responses == 0 && w.Timeouts == 0 {
		return "NO_DATA"
	}
	if w.QualityPct < 90 || w.TimeoutPct >= 5 {
		return "BAD"
	}
	// Fallbacks are not terminal DNS errors, but a sustained fallback rate is a
	// strong signal that the preferred resolver is too slow or unreliable.
	if w.QualityPct < 98 || w.FallbackPct >= 5 || w.P95LatencyMS >= 1000 {
		return "WARN"
	}
	return "GOOD"
}

func (s *Store) windowStatsLocked(port uint16, minutes int, now time.Time) WindowStats {
	if minutes < 1 {
		minutes = 1
	}
	if minutes > len(s.history) {
		minutes = len(s.history)
	}
	w := WindowStats{Minutes: minutes, QualityPct: 100}
	var latSum float64
	var latN uint64
	var hist [len(latencyBoundsMS)]uint64
	nowMinute := now.Unix() / 60
	for m := nowMinute - int64(minutes) + 1; m <= nowMinute; m++ {
		idx := int(m % int64(len(s.history)))
		if idx < 0 {
			idx += len(s.history)
		}
		b := s.history[idx]
		if b.minute != m {
			continue
		}
		r := b.upstreams[port]
		if r == nil {
			continue
		}
		w.Requests += r.requests
		w.Responses += r.responses
		w.LateResponses += r.lateResponses
		w.Success += r.success
		w.Errors += r.errors
		w.Timeouts += r.timeouts
		w.Fallbacks += r.fallbacks
		latSum += r.latencySum
		latN += r.latencyN
		if r.maxLatency > w.MaxLatencyMS {
			w.MaxLatencyMS = r.maxLatency
		}
		for i := range hist {
			hist[i] += r.latencyHist[i]
		}
	}
	if latN > 0 {
		w.AvgLatencyMS = latSum / float64(latN)
	}
	if latN > 0 {
		want := (latN*95 + 99) / 100
		var seen uint64
		for i, c := range hist {
			seen += c
			if seen >= want {
				w.P95LatencyMS = latencyBoundsMS[i]
				break
			}
		}
		if w.P95LatencyMS == 0 || w.P95LatencyMS > w.MaxLatencyMS {
			w.P95LatencyMS = w.MaxLatencyMS
		}
	}
	attempts := w.Success + w.Errors + w.Timeouts
	if attempts > 0 {
		w.QualityPct = float64(w.Success) / float64(attempts) * 100
		w.ErrorPct = float64(w.Errors) / float64(attempts) * 100
		w.TimeoutPct = float64(w.Timeouts) / float64(attempts) * 100
	}
	if w.Requests > 0 {
		w.FallbackPct = float64(w.Fallbacks) / float64(w.Requests) * 100
	}
	terminal := w.Success + w.Errors + w.Timeouts
	if w.Requests > terminal {
		w.Pending = w.Requests - terminal
	}
	w.QualityStatus = qualityStatus(w)
	return w
}

func (s *Store) fallbackEdgesWindowLocked(minutes int, now time.Time) []FallbackEdge {
	if minutes < 1 {
		minutes = 1
	}
	if minutes > len(s.history) {
		minutes = len(s.history)
	}
	counts := make(map[[2]uint16]uint64)
	nowMinute := now.Unix() / 60
	for m := nowMinute - int64(minutes) + 1; m <= nowMinute; m++ {
		idx := int(m % int64(len(s.history)))
		if idx < 0 {
			idx += len(s.history)
		}
		b := s.history[idx]
		if b.minute != m {
			continue
		}
		for pair, c := range b.edges {
			counts[pair] += c
		}
	}
	return s.edgesFromCountsLocked(counts, 50)
}

func (s *Store) edgesFromCountsLocked(counts map[[2]uint16]uint64, limit int) []FallbackEdge {
	edges := make([]FallbackEdge, 0, len(counts))
	for pair, count := range counts {
		from, fok := s.upstreams[pair[0]]
		to, tok := s.upstreams[pair[1]]
		if !fok || !tok {
			continue
		}
		edges = append(edges, FallbackEdge{FromProfile: from.meta.Profile, FromUpstream: from.meta.Name, FromPort: pair[0], ToProfile: to.meta.Profile, ToUpstream: to.meta.Name, ToPort: pair[1], Count: count})
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].Count > edges[j].Count })
	if limit > 0 && len(edges) > limit {
		edges = edges[:limit]
	}
	return edges
}

func (s *Store) Snapshot(flowN, topN, errorN int) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	ups := make([]UpstreamView, 0, len(s.upstreams))
	healthy, down, degraded := 0, 0, 0
	activeDown, activeDegraded, inactiveDown := 0, 0, 0
	activeQualityBad, activeQualityWarn := 0, 0

	hasSystem := false
	for _, st := range s.upstreams {
		if st.meta.Profile == "System" {
			hasSystem = true
			break
		}
	}
	primaryHealthy, primaryDown, primaryDegraded := 0, 0, 0
	primaryActiveDown, primaryActiveDegraded := 0, 0
	primaryActiveQualityBad, primaryActiveQualityWarn := 0, 0
	primaryUpstreamCount := 0

	for _, st := range s.upstreams {
		avg := 0.0
		if st.latencyCount > 0 {
			avg = st.latencySumMS / float64(st.latencyCount)
		}
		active := !st.lastRequest.IsZero() && now.Sub(st.lastRequest) <= 5*time.Minute
		w5 := s.windowStatsLocked(st.meta.Port, 5, now)
		w1 := s.windowStatsLocked(st.meta.Port, 60, now)
		w24 := s.windowStatsLocked(st.meta.Port, 1440, now)
		v := UpstreamView{UpstreamMeta: st.meta, Requests: st.requests, Responses: st.responses, LateResponses: st.lateResponses, UnmatchedResponses: st.unmatchedResponses, NXDomain: st.nxdomain, ServFail: st.servfail, Refused: st.refused, OtherErrors: st.otherErrors, Timeouts: st.timeouts, Fallbacks: st.fallbacks, AvgLatencyMS: avg, MaxLatencyMS: st.maxLatencyMS, HealthStatus: st.healthStatus, HealthLatencyMS: st.healthLatencyMS, LastHealthCheck: st.lastHealthCheck, LastHealthError: st.lastHealthError, ConsecutiveFails: st.consecutiveFails, LastObserved: st.lastObserved, ObservedLatencyMS: st.observedLatencyMS, LastRequest: st.lastRequest, Active: active, HealthCause: healthCause(st), Stats5m: w5, Stats1h: w1, Stats24h: w24, Diagnostic: st.diagnostic}
		if active {
			switch w5.QualityStatus {
			case "BAD":
				activeQualityBad++
			case "WARN":
				activeQualityWarn++
			}
		}
		ups = append(ups, v)
		switch st.healthStatus {
		case "UP":
			healthy++
		case "DOWN":
			down++
			if active {
				activeDown++
			} else {
				inactiveDown++
			}
		case "DEGRADED":
			degraded++
			if active {
				activeDegraded++
			}
		}

		if st.meta.Profile == "System" || !hasSystem {
			primaryUpstreamCount++
			if active {
				switch w5.QualityStatus {
				case "BAD":
					primaryActiveQualityBad++
				case "WARN":
					primaryActiveQualityWarn++
				}
			}
			switch st.healthStatus {
			case "UP":
				primaryHealthy++
			case "DOWN":
				primaryDown++
				if active {
					primaryActiveDown++
				}
			case "DEGRADED":
				primaryDegraded++
				if active {
					primaryActiveDegraded++
				}
			}
		}
	}
	sort.Slice(ups, func(i, j int) bool {
		if ups[i].Profile == ups[j].Profile {
			return ups[i].Port < ups[j].Port
		}
		return ups[i].Profile < ups[j].Profile
	})
	flow := s.flowTailLocked(flowN)
	errs := tailCopy(s.errors, errorN)
	tops := make([]DomainCount, 0, len(s.domainCounts))
	for d, c := range s.domainCounts {
		tops = append(tops, DomainCount{Domain: d, Count: c})
	}
	sort.Slice(tops, func(i, j int) bool { return tops[i].Count > tops[j].Count })
	if topN > 0 && len(tops) > topN {
		tops = tops[:topN]
	}
	edges := s.edgesFromCountsLocked(s.fallbackEdges, 30)
	return map[string]any{
		"started": s.started, "uptime_seconds": int64(time.Since(s.started).Seconds()), "total_requests": s.totalRequests, "total_responses": s.totalResponses, "total_late_responses": s.totalLateResponses, "total_unmatched_responses": s.totalUnmatchedResponses, "total_fallbacks": s.totalFallbacks, "total_timeouts": s.totalTimeouts,
		"healthy": healthy, "down": down, "degraded": degraded, "active_down": activeDown, "active_degraded": activeDegraded, "inactive_down": inactiveDown, "active_quality_bad": activeQualityBad, "active_quality_warn": activeQualityWarn,
		"primary_healthy": primaryHealthy, "primary_down": primaryDown, "primary_degraded": primaryDegraded, "primary_active_down": primaryActiveDown, "primary_active_degraded": primaryActiveDegraded, "primary_active_quality_bad": primaryActiveQualityBad, "primary_active_quality_warn": primaryActiveQualityWarn, "primary_upstream_count": primaryUpstreamCount,
		"upstream_count": len(ups), "upstreams": ups, "flow": flow, "top_domains": tops, "errors": errs, "fallback_edges": edges, "fallback_edges_5m": s.fallbackEdgesWindowLocked(5, now), "fallback_edges_1h": s.fallbackEdgesWindowLocked(60, now), "fallback_edges_24h": s.fallbackEdgesWindowLocked(1440, now),
		"error_bursts": s.errorBurstsLocked(60, now), "last_discovery": s.lastDiscovery, "discovery_error": s.discoveryError, "capture_error": s.captureError,
		"last_client_registry": s.lastClientRegistry, "client_registry_error": s.clientRegistryError, "client_capture_error": s.clientCaptureError,
	}
}

func (s *Store) History(minutes int) []HistoryPoint {
	if minutes < 1 {
		minutes = 1
	}
	if minutes > len(s.history) {
		minutes = len(s.history)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	nowMinute := time.Now().Unix() / 60
	requestedStart := nowMinute - int64(minutes) + 1
	startedMinute := s.started.Unix() / 60
	if requestedStart < startedMinute {
		requestedStart = startedMinute
	}
	if requestedStart > nowMinute {
		requestedStart = nowMinute
	}

	// Keep long-range graphs light enough for a router UI. Five minutes and one
	// hour stay minute-granular; 3h becomes 3-minute buckets; 24h becomes
	// 15-minute buckets. This also cuts JSON/DOM work dramatically.
	step := int64(1)
	if minutes > 180 {
		step = 15
	} else if minutes > 60 {
		step = 3
	}

	out := make([]HistoryPoint, 0, int((nowMinute-requestedStart)/step)+1)
	for start := requestedStart; start <= nowMinute; start += step {
		end := start + step - 1
		if end > nowMinute {
			end = nowMinute
		}
		p := HistoryPoint{Time: time.Unix(start*60, 0)}
		for m := start; m <= end; m++ {
			idx := int(m % int64(len(s.history)))
			if idx < 0 {
				idx += len(s.history)
			}
			b := s.history[idx]
			if b.minute != m {
				continue
			}
			p.Requests += b.requests
			p.Responses += b.responses
			p.LateResponses += b.lateResponses
			p.Fallbacks += b.fallbacks
			p.Errors += b.errors
			p.Timeouts += b.timeouts
		}
		out = append(out, p)
	}
	return out
}

func (s *Store) Quality(minutes int) []QualityUpstreamView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	out := make([]QualityUpstreamView, 0, len(s.upstreams))
	for _, st := range s.upstreams {
		out = append(out, QualityUpstreamView{
			UpstreamMeta:     st.meta,
			HealthStatus:     st.healthStatus,
			HealthCause:      healthCause(st),
			HealthLatencyMS:  st.healthLatencyMS,
			LastHealthCheck:  timeString(st.lastHealthCheck),
			LastHealthError:  st.lastHealthError,
			ConsecutiveFails: st.consecutiveFails,
			LastRequest:      timeString(st.lastRequest),
			Active:           !st.lastRequest.IsZero() && now.Sub(st.lastRequest) <= 5*time.Minute,
			Window:           s.windowStatsLocked(st.meta.Port, minutes, now),
			Diagnostic:       st.diagnostic,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Profile == out[j].Profile {
			return out[i].Port < out[j].Port
		}
		return out[i].Profile < out[j].Profile
	})
	return out
}

func (s *Store) FallbackEdges(minutes int) []FallbackEdge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fallbackEdgesWindowLocked(minutes, time.Now())
}

func (s *Store) ErrorBursts(minutes int) []ErrorBurstView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorBurstsLocked(minutes, time.Now())
}
func (s *Store) errorBurstsLocked(minutes int, now time.Time) []ErrorBurstView {
	if minutes < 1 {
		minutes = 1
	}
	if minutes > len(s.history) {
		minutes = len(s.history)
	}
	out := make([]ErrorBurstView, 0)
	nowMinute := now.Unix() / 60
	for m := nowMinute - int64(minutes) + 1; m <= nowMinute; m++ {
		idx := int(m % int64(len(s.history)))
		if idx < 0 {
			idx += len(s.history)
		}
		b := s.history[idx]
		if b.minute != m {
			continue
		}
		for key, eb := range b.errorBursts {
			st := s.upstreams[key.port]
			if st == nil {
				continue
			}
			domains := make([]string, 0, len(eb.domains))
			for d := range eb.domains {
				domains = append(domains, d)
			}
			sort.Strings(domains)
			if len(domains) > 8 {
				domains = domains[:8]
			}
			out = append(out, ErrorBurstView{Minute: time.Unix(m*60, 0), Profile: st.meta.Profile, Upstream: st.meta.Name, Port: key.port, Kind: key.kind, Count: eb.count, Domains: domains})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Minute.Equal(out[j].Minute) {
			return out[i].Count > out[j].Count
		}
		return out[i].Minute.After(out[j].Minute)
	})
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

func (s *Store) addFlowLocked(e FlowEvent) {
	if s.flowCap <= 0 {
		return
	}
	s.flow[s.flowPos] = e
	s.flowPos = (s.flowPos + 1) % s.flowCap
	if s.flowCount < s.flowCap {
		s.flowCount++
	}
}
func (s *Store) flowTailLocked(n int) []FlowEvent {
	if n <= 0 || s.flowCount == 0 {
		return []FlowEvent{}
	}
	if n > s.flowCount {
		n = s.flowCount
	}
	out := make([]FlowEvent, 0, n)
	start := (s.flowPos - n + s.flowCap) % s.flowCap
	for i := 0; i < n; i++ {
		out = append(out, s.flow[(start+i)%s.flowCap])
	}
	return out
}

func (s *Store) CleanupTransient(now time.Time, log *EventLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, p := range s.pending {
		limit := 8 * time.Second
		if st, ok := s.upstreams[p.port]; ok && st.meta.TimeoutMS > 0 {
			limit = time.Duration(st.meta.TimeoutMS+750) * time.Millisecond
		}
		if now.Sub(p.when) > limit {
			if st, ok := s.upstreams[p.port]; ok {
				st.timeouts++
				s.totalTimeouts++
				// Global history records when the timeout was observed; resolver
				// quality attributes the terminal outcome to the request cohort.
				wireBucket := s.historyBucketLocked(now)
				wireBucket.timeouts++
				wireBucket.errors++
				queryBucket := s.historyBucketLocked(p.when)
				rb := resolverBucketLocked(queryBucket, p.port)
				rb.timeouts++
				s.recordBurstLocked(now, p.port, "TIMEOUT", p.domain)
				if p.clientKey != "" {
					s.recordClientTimeoutLocked(p.clientKey)
				}
				if p.clientTxn != nil {
					p.clientTxn.upstreamTimeout = true
				}
				if s.shouldEmitErrorLocked(now, "TIMEOUT", p.port, p.domain) {
					s.addErrorLocked(ErrorEvent{Time: now, Profile: st.meta.Profile, Upstream: st.meta.Name, Port: p.port, Domain: p.domain, Kind: "TIMEOUT", Message: "No DNS response before resolver timeout"})
					if log != nil {
						log.DNSError("TIMEOUT", st.meta.Profile, st.meta.Name, p.domain)
					}
				}
			}
			s.timedOut[k] = timedOutQuery{timeoutAt: now, query: p}
			delete(s.pending, k)
		}
	}
	for k, items := range s.recentByName {
		keep := items[:0]
		for _, it := range items {
			if now.Sub(it.when) <= 2*time.Second {
				keep = append(keep, it)
			}
		}
		if len(keep) == 0 {
			delete(s.recentByName, k)
		} else {
			s.recentByName[k] = keep
		}
	}
	for k, t := range s.timedOut {
		if now.Sub(t.timeoutAt) > time.Minute {
			delete(s.timedOut, k)
		}
	}
	for k, t := range s.errorDedup {
		if now.Sub(t) > 10*time.Minute {
			delete(s.errorDedup, k)
		}
	}
	for k, items := range s.recentClients {
		keep := items[:0]
		for _, it := range items {
			if it != nil && now.Sub(it.when) <= 2*time.Second {
				keep = append(keep, it)
			}
		}
		if len(keep) == 0 {
			delete(s.recentClients, k)
		} else {
			s.recentClients[k] = keep
		}
	}
	for k, items := range s.clientPending {
		keep := items[:0]
		for _, it := range items {
			if it == nil || it.terminal {
				continue
			}
			if now.Sub(it.when) > 10*time.Second {
				s.finalizeClientTimeoutLocked(now, it)
				continue
			}
			keep = append(keep, it)
		}
		if len(keep) == 0 {
			delete(s.clientPending, k)
		} else {
			s.clientPending[k] = keep
		}
	}
	for k, t := range s.clientDedup {
		if now.Sub(t) > 10*time.Second {
			delete(s.clientDedup, k)
		}
	}
	for k, t := range s.clientResponseDedup {
		if now.Sub(t) > 10*time.Second {
			delete(s.clientResponseDedup, k)
		}
	}
}

func (s *Store) addClientFlowLocked(e ClientFlowEvent) {
	if s.clientFlowCap <= 0 {
		return
	}
	s.clientFlow[s.clientFlowPos] = e
	s.clientFlowPos = (s.clientFlowPos + 1) % s.clientFlowCap
	if s.clientFlowCount < s.clientFlowCap {
		s.clientFlowCount++
	}
}

func (s *Store) clientFlowTailLocked(n int) []ClientFlowEvent {
	if n <= 0 || s.clientFlowCount == 0 {
		return []ClientFlowEvent{}
	}
	if n > s.clientFlowCount {
		n = s.clientFlowCount
	}
	out := make([]ClientFlowEvent, 0, n)
	start := (s.clientFlowPos - n + s.clientFlowCap) % s.clientFlowCap
	for i := 0; i < n; i++ {
		out = append(out, s.clientFlow[(start+i)%s.clientFlowCap])
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func appendRing[T any](s []T, v T, capN int) []T {
	if capN <= 0 {
		return s
	}
	if len(s) < capN {
		return append(s, v)
	}
	copy(s, s[1:])
	s[len(s)-1] = v
	return s
}
func tailCopy[T any](s []T, n int) []T {
	if n <= 0 || len(s) == 0 {
		return []T{}
	}
	if n > len(s) {
		n = len(s)
	}
	out := make([]T, n)
	copy(out, s[len(s)-n:])
	return out
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Store) HistoryCoverage(minutes int) float64 {
	if minutes < 1 {
		minutes = 1
	}
	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()
	covered := time.Since(started).Minutes()
	if covered < 0 {
		covered = 0
	}
	v := covered / float64(minutes)
	if v > 1 {
		v = 1
	}
	return v
}
