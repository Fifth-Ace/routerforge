package main

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dnsServerRE = regexp.MustCompile(`^dns_server\s*=\s*127\.0\.0\.1:(\d+)\s+(\S+)\s+#\s+(.+)$`)

type tlsEndpointMeta struct {
	address       string
	port          string
	sni           string
	interfaceName string
	domain        string
}

type httpsEndpointMeta struct {
	uri           string
	interfaceName string
	domain        string
}

type profileParse struct {
	name      string
	proceedMS int
	timeoutMS int
	dnsPort   uint16
	entries   []UpstreamMeta
	tls       []tlsEndpointMeta
	https     []httpsEndpointMeta
}

const (
	dnsAuxiliaryTTL       = 5 * time.Minute
	dnsAuxiliaryColdRetry = time.Minute
)

type dnsDiscoveryCommand func(string, time.Duration) (string, error)

type dnsAuxiliaryState struct {
	text        string
	hasValue    bool
	nextAttempt time.Time
}

type dnsDiscoveryCollector struct {
	auxiliaryTTL time.Duration
	now          func() time.Time
	run          dnsDiscoveryCommand

	auxiliary    map[string]dnsAuxiliaryState
	havePrimary  bool
	lastUpstream []UpstreamMeta
	lastPlain    []PlainDNSMeta
}

func newDNSDiscoveryCollector(auxiliaryTTL time.Duration) *dnsDiscoveryCollector {
	return &dnsDiscoveryCollector{
		auxiliaryTTL: auxiliaryTTL,
		now:          time.Now,
		run:          ndmcOutput,
		auxiliary:    make(map[string]dnsAuxiliaryState),
	}
}

func (c *dnsDiscoveryCollector) primaryChanged(ups []UpstreamMeta, plain []PlainDNSMeta) bool {
	changed := !c.havePrimary ||
		!reflect.DeepEqual(c.lastUpstream, ups) ||
		!reflect.DeepEqual(c.lastPlain, plain)

	c.havePrimary = true
	c.lastUpstream = append([]UpstreamMeta(nil), ups...)
	c.lastPlain = append([]PlainDNSMeta(nil), plain...)
	return changed
}

func (c *dnsDiscoveryCollector) auxiliaryText(command string, timeout time.Duration, force bool) string {
	now := c.now()
	state := c.auxiliary[command]

	due := force || c.auxiliaryTTL <= 0 || state.nextAttempt.IsZero() || !now.Before(state.nextAttempt)
	if !due {
		return state.text
	}

	text, err := c.run(command, timeout)
	if err == nil {
		state.text = text
		state.hasValue = true
		if c.auxiliaryTTL > 0 {
			state.nextAttempt = now.Add(c.auxiliaryTTL)
		}
		c.auxiliary[command] = state
		return state.text
	}

	if c.auxiliaryTTL > 0 {
		retry := c.auxiliaryTTL
		if !state.hasValue && dnsAuxiliaryColdRetry < retry {
			retry = dnsAuxiliaryColdRetry
		}
		state.nextAttempt = now.Add(retry)
	}
	c.auxiliary[command] = state
	return state.text
}

func (c *dnsDiscoveryCollector) discover() ([]UpstreamMeta, []PlainDNSMeta, map[string]PolicyRouteView, error) {
	dnsProxyText, err := c.run("show dns-proxy", 10*time.Second)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ndmc show dns-proxy: %w", err)
	}

	ups := parseDNSProxy(dnsProxyText)
	primaryPlain := parsePlainDNSProxy(dnsProxyText)
	forceAuxiliary := c.primaryChanged(ups, primaryPlain)

	plain := append([]PlainDNSMeta(nil), primaryPlain...)
	nameServerText := c.auxiliaryText("show ip name-server", 6*time.Second, forceAuxiliary)
	plain = append(plain, parseIPNameServers(nameServerText)...)

	policyText := c.auxiliaryText("show ip policy", 6*time.Second, forceAuxiliary)
	policies := parseIPPolicies(policyText)

	interfaceText := c.auxiliaryText("show interface", 8*time.Second, forceAuxiliary)
	linuxIfs := keeneticLinuxInterfacesFromText(interfaceText)
	ifaceIndex := routeInterfaceIndexFromText(interfaceText)

	for i := range ups {
		if p, ok := policies[ups[i].Profile]; ok {
			ups[i].PolicyMark = p.Mark
			ups[i].PolicyTable = p.Table
			ups[i].PolicyDescription = p.Description
			ups[i].PolicyHasDefault = p.HasDefault
		}
		if ups[i].Interface != "" {
			ups[i].LinuxInterface = linuxIfs[ups[i].Interface]
		}
	}

	return ups, plain, buildPolicyRouteViews(policies, ifaceIndex), nil
}

func discoverDNSConfiguration() ([]UpstreamMeta, []PlainDNSMeta, map[string]PolicyRouteView, error) {
	return newDNSDiscoveryCollector(0).discover()
}

// Keep the old helper for tests/internal callers that only care about
// encrypted local proxy upstreams.
func discoverUpstreams() ([]UpstreamMeta, map[string]PolicyRouteView, error) {
	ups, _, routes, err := discoverDNSConfiguration()
	return ups, routes, err
}

func parseDNSProxy(text string) []UpstreamMeta {
	s := bufio.NewScanner(strings.NewReader(text))
	profiles := make([]*profileParse, 0, 4)
	var cur *profileParse
	var curTLS *tlsEndpointMeta
	var curHTTPS *httpsEndpointMeta

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.HasPrefix(line, "proxy-name:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "proxy-name:"))
			cur = &profileParse{name: name, proceedMS: 500, timeoutMS: 7000}
			profiles = append(profiles, cur)
			curTLS, curHTTPS = nil, nil
			continue
		}
		if cur == nil {
			continue
		}

		if strings.HasPrefix(line, "proceed =") {
			if n, e := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "proceed ="))); e == nil {
				cur.proceedMS = n
			}
			continue
		}
		if strings.HasPrefix(line, "timeout =") {
			if n, e := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "timeout ="))); e == nil {
				cur.timeoutMS = n
			}
			continue
		}
		if strings.HasPrefix(line, "dns_tcp_port =") {
			if n, e := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "dns_tcp_port ="))); e == nil && n > 0 && n <= 65535 {
				cur.dnsPort = uint16(n)
			}
			continue
		}

		if line == "server-tls:" {
			cur.tls = append(cur.tls, tlsEndpointMeta{})
			curTLS = &cur.tls[len(cur.tls)-1]
			curHTTPS = nil
			continue
		}
		if line == "server-https:" {
			cur.https = append(cur.https, httpsEndpointMeta{})
			curHTTPS = &cur.https[len(cur.https)-1]
			curTLS = nil
			continue
		}

		if curTLS != nil {
			switch {
			case strings.HasPrefix(line, "address:"):
				curTLS.address = strings.TrimSpace(strings.TrimPrefix(line, "address:"))
			case strings.HasPrefix(line, "port:"):
				curTLS.port = strings.TrimSpace(strings.TrimPrefix(line, "port:"))
			case strings.HasPrefix(line, "sni:"):
				curTLS.sni = strings.TrimSpace(strings.TrimPrefix(line, "sni:"))
			case strings.HasPrefix(line, "interface:"):
				curTLS.interfaceName = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			case strings.HasPrefix(line, "domain:"):
				curTLS.domain = strings.TrimSpace(strings.TrimPrefix(line, "domain:"))
			}
		}
		if curHTTPS != nil {
			switch {
			case strings.HasPrefix(line, "uri:"):
				curHTTPS.uri = strings.TrimSpace(strings.TrimPrefix(line, "uri:"))
			case strings.HasPrefix(line, "interface:"):
				curHTTPS.interfaceName = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			case strings.HasPrefix(line, "domain:"):
				curHTTPS.domain = strings.TrimSpace(strings.TrimPrefix(line, "domain:"))
			}
		}

		m := dnsServerRE.FindStringSubmatch(line)
		if len(m) != 4 {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err != nil || port < 1 || port > 65535 {
			continue
		}

		domain := m[2]
		if domain == "." {
			domain = ""
		}
		comment := strings.TrimSpace(m[3])
		meta := UpstreamMeta{
			Port: uint16(port), Profile: cur.name, Domain: domain,
			ProceedMS: cur.proceedMS, TimeoutMS: cur.timeoutMS,
		}

		if strings.HasPrefix(comment, "https://") || strings.HasPrefix(comment, "http://") {
			meta.Protocol = "DoH"
			raw := comment
			if i := strings.LastIndex(raw, "@dnsm"); i >= 0 {
				raw = raw[:i]
			}
			meta.Target = raw
			if u, e := url.Parse(raw); e == nil {
				meta.SNI = u.Hostname()
				meta.Name = friendlyResolverName(u.Hostname(), "DoH")
			}
		} else {
			meta.Protocol = "DoT"
			left, right, ok := strings.Cut(comment, "@")
			meta.Target = left
			if ok {
				meta.SNI = right
			}
			if meta.SNI == "" {
				meta.SNI = strings.Split(left, ":")[0]
			}
			meta.Name = friendlyResolverName(meta.SNI, "DoT")
		}
		if meta.Name == "" {
			meta.Name = meta.Target
		}
		cur.entries = append(cur.entries, meta)
	}

	var out []UpstreamMeta
	seen := make(map[uint16]bool)
	for _, profile := range profiles {
		for _, entry := range profile.entries {
			entry = enrichEndpointMeta(entry, profile)
			entry.ProfileDNSPort = profile.dnsPort
			if seen[entry.Port] {
				continue
			}
			seen[entry.Port] = true
			out = append(out, entry)
		}
	}
	return out
}

func enrichEndpointMeta(e UpstreamMeta, p *profileParse) UpstreamMeta {
	if e.Protocol == "DoH" {
		for _, h := range p.https {
			if h.uri == e.Target {
				e.Interface = h.interfaceName
				if e.Domain == "" {
					e.Domain = h.domain
				}
				return e
			}
		}
		return e
	}

	host := e.Target
	port := "853"
	if h, po, err := net.SplitHostPort(e.Target); err == nil {
		host, port = h, po
	}
	for _, t := range p.tls {
		tp := t.port
		if tp == "" {
			tp = "853"
		}
		if t.address == host && tp == port {
			e.Interface = t.interfaceName
			if e.SNI == "" {
				e.SNI = t.sni
			}
			if e.Domain == "" {
				e.Domain = t.domain
			}
			return e
		}
	}
	return e
}

func friendlyResolverName(host, proto string) string {
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "quad9"):
		return "Quad9 " + proto
	case h == "dns.google" || strings.Contains(h, "google"):
		return "Google " + proto
	case strings.Contains(h, "cloudflare"):
		return "Cloudflare " + proto
	case strings.Contains(h, "yandex"):
		return "Yandex " + proto
	case strings.Contains(h, "comss"):
		return "Comss " + proto
	case strings.Contains(h, "xbox-dns"):
		return "Xbox DNS " + proto
	case strings.Contains(h, "astracat"):
		return "Astracat " + proto
	}
	if host != "" {
		return host + " " + proto
	}
	return proto
}

const dnsBackgroundMaxBackoff = 5 * time.Minute

type pollBackoff struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

func newPollBackoff(base, max time.Duration) pollBackoff {
	if base <= 0 {
		base = time.Second
	}
	if max < base {
		max = base
	}
	return pollBackoff{base: base, max: max, current: base}
}

func (b *pollBackoff) next(success bool) time.Duration {
	if success {
		b.current = b.base
		return b.current
	}
	if b.current < b.base {
		b.current = b.base
	}
	if b.current >= b.max {
		b.current = b.max
		return b.current
	}
	next := b.current * 2
	if next < b.current || next > b.max {
		next = b.max
	}
	b.current = next
	return b.current
}

func runAdaptivePoll(base, max time.Duration, refresh func() bool) {
	backoff := newPollBackoff(base, max)
	for {
		success := refresh()
		time.Sleep(backoff.next(success))
	}
}

func discoveryLoop(store *Store, interval time.Duration, log *EventLogger) {
	collector := newDNSDiscoveryCollector(dnsAuxiliaryTTL)
	refresh := func() bool {
		ups, plain, routes, err := collector.discover()
		if err != nil {
			store.SetDiscoveryError(err.Error())
			log.Event("DISCOVERY_ERROR", err.Error())
			return false
		}
		store.UpdateDiscovery(ups)
		store.UpdatePolicyRoutes(routes)
		plainDNS.UpdateResolvers(plain)
		return true
	}

	runAdaptivePoll(interval, dnsBackgroundMaxBackoff, refresh)
}
