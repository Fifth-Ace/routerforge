package main

import (
	"sort"
	"strings"
)

type v2CandidatePoolItem struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Source              string   `json:"source"`
	Family              string   `json:"family"`
	Protocol            string   `json:"protocol"`
	Args                []string `json:"args"`
	Fingerprint         string   `json:"fingerprint"`
	MemoryClass         string   `json:"memory_class,omitempty"`
	MemoryConfidence    string   `json:"memory_confidence,omitempty"`
	MemoryAgeSeconds    int64    `json:"memory_age_seconds,omitempty"`
	MemoryVerifiedCount int      `json:"memory_verified_count,omitempty"`
	MemorySuccessStreak int      `json:"memory_success_streak,omitempty"`
	Stage               string   `json:"stage,omitempty"`
}


type v2SelectorAutoPoolMeta struct {
	Enabled           bool
	Added             int
	MemoryCandidates  int
	LibraryCandidates int
	BuiltinCandidates int
	Sources           []string
	Warnings          []string
}

type v2BuiltinCandidate struct {
	ID       string
	Name     string
	Family   string
	Protocol string
	Args     []string
}

var v2BuiltinHTTPSCandidates = []v2BuiltinCandidate{
	{
		ID: "builtin-tls-multisplit-midsld", Name: "TLS multisplit midsld",
		Family: "split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "builtin-tls-multisplit-sniext", Name: "TLS multisplit sniext",
		Family: "split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,sniext+1"},
	},
	{
		ID: "builtin-tls-multisplit-host", Name: "TLS multisplit host",
		Family: "split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,host+1"},
	},
	{
		ID: "builtin-tls-multidisorder-midsld", Name: "TLS multidisorder midsld",
		Family: "disorder", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "builtin-tls-multidisorder-sniext", Name: "TLS multidisorder sniext",
		Family: "disorder", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,sniext+1"},
	},
	{
		ID: "builtin-tls-fakedsplit-midsld", Name: "TLS fakedsplit midsld",
		Family: "fake-split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld"},
	},
	{
		ID: "builtin-tls-fakeddisorder-midsld", Name: "TLS fakeddisorder midsld",
		Family: "fake-disorder", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld"},
	},
	{
		ID: "builtin-tls-inline-fake-multisplit", Name: "TLS inline fake + multisplit",
		Family: "fake+split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=0x00000000:tcp_ack=-66000:repeats=2", "--lua-desync=multisplit:pos=1,midsld"},
	},
}

var v2BuiltinHTTPCandidates = []v2BuiltinCandidate{
	{
		ID: "builtin-http-hostcase", Name: "HTTP hostcase",
		Family: "http-case", Protocol: "http",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_hostcase"},
	},
	{
		ID: "builtin-http-domcase", Name: "HTTP domcase",
		Family: "http-case", Protocol: "http",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_domcase"},
	},
	{
		ID: "builtin-http-methodeol", Name: "HTTP method EOL",
		Family: "http-method", Protocol: "http",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_methodeol"},
	},
}

var v2BuiltinQUICCandidates = []v2BuiltinCandidate{
	{ID: "builtin-quic-fake-1", Name: "QUIC fake x1", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=1"}},
	{ID: "builtin-quic-fake-2", Name: "QUIC fake x2", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=2"}},
	{ID: "builtin-quic-fake-3", Name: "QUIC fake x3", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=3"}},
	{ID: "builtin-quic-fake-5", Name: "QUIC fake x5", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=5"}},
}

var v2BuiltinSTUNCandidates = []v2BuiltinCandidate{
	{ID: "builtin-stun-fake-1", Name: "STUN fake x1", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=1"}},
	{ID: "builtin-stun-fake-2", Name: "STUN fake x2", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2"}},
	{ID: "builtin-stun-fake-3", Name: "STUN fake x3", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=3"}},
	{ID: "builtin-stun-fake-5", Name: "STUN fake x5", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=5"}},
}

func v2BuiltinCandidatesForTransport(transport benchTransportProfile) []v2BuiltinCandidate {
	var items []v2BuiltinCandidate
	switch transport.ID {
	case benchTransportHTTPS:
		items = v2BuiltinHTTPSCandidates
	case benchTransportHTTP:
		items = v2BuiltinHTTPCandidates
	case benchTransportQUIC:
		items = v2BuiltinQUICCandidates
	case benchTransportSTUN:
		items = v2BuiltinSTUNCandidates
	default:
		return nil
	}
	return append([]v2BuiltinCandidate{}, items...)
}

func v2PortableCandidateArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--hostlist-domains="),
			strings.HasPrefix(arg, "--hostlist="),
			strings.HasPrefix(arg, "--hostlist-auto="),
			strings.HasPrefix(arg, "--hostlist-exclude="),
			strings.HasPrefix(arg, "--ipset="),
			strings.HasPrefix(arg, "--ipset-exclude="):
			continue
		}
		out = append(out, arg)
	}
	return out
}

func v2CandidateTechniqueFingerprint(args []string) string {
	return v2StrategyFingerprint(v2PortableCandidateArgs(args))
}

func v2CandidateSourceList(items []v2CandidatePoolItem) []string {
	set := map[string]bool{}
	for _, item := range items {
		if item.Source != "" {
			set[item.Source] = true
		}
	}
	out := make([]string, 0, len(set))
	for source := range set {
		out = append(out, source)
	}
	sort.Strings(out)
	return out
}

func populateV2SelectorCandidates(req *v2SelectorRequest) (v2SelectorAutoPoolMeta, error) {
	meta := v2SelectorAutoPoolMeta{Sources: []string{}, Warnings: []string{}}
	enabled := true
	if req.AutoPool != nil {
		enabled = *req.AutoPool
	}
	meta.Enabled = enabled
	if !enabled {
		return meta, nil
	}

	if _, err := v2NormalizeTarget(req.ServerName); err != nil {
		return meta, err
	}
	mode, err := v2SelectorMode(req.Mode)
	if err != nil {
		return meta, err
	}
	transport, err := normalizeBenchTransport(benchTransportHTTPS)
	if err != nil {
		return meta, err
	}

	// P26A direct execution model:
	// Auto Pool is the portable corpus itself. No planner, synthesis, historical
	// promotion or adaptive mutation is allowed to replace real candidate execution.
	external := append([]v2SelectorCandidateInput{}, req.Candidates...)
	req.Candidates = []v2SelectorCandidateInput{}
	seen := map[string]bool{}
	sourceSet := map[string]bool{}

	corpus := v2CorpusCandidatesForTransport(transport)
	autoLimit := v2DirectAutoPoolLimit(mode.Name, len(corpus))
	for _, item := range corpus {
		if len(req.Candidates) >= autoLimit {
			break
		}
		fp := v2CandidateTechniqueFingerprint(item.Args)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		req.Candidates = append(req.Candidates, v2SelectorCandidateInput{
			ID: item.ID, Name: item.Name, Source: item.Source, Args: append([]string{}, item.Args...),
		})
		meta.Added++
		meta.LibraryCandidates++
		if item.Source != "" {
			sourceSet[item.Source] = true
		}
	}

	for _, existing := range external {
		if len(req.Candidates) >= v2DirectSelectorMaxCandidates {
			break
		}
		fp := v2CandidateTechniqueFingerprint(existing.Args)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		req.Candidates = append(req.Candidates, existing)
		if existing.Source != "" {
			sourceSet[v2StrategySource(existing.Source)] = true
		}
	}

	for source := range sourceSet {
		meta.Sources = append(meta.Sources, source)
	}
	sort.Strings(meta.Sources)
	meta.Warnings = append(meta.Warnings, "P26 direct catalog execution: portable corpus candidates are executed directly")
	return meta, nil
}
