package main

import (
	"net/http"
	"sort"
	"strings"
)

type v2CandidatePoolItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Family      string   `json:"family"`
	Protocol    string   `json:"protocol"`
	Args        []string `json:"args"`
	Fingerprint string   `json:"fingerprint"`
	MemoryClass string   `json:"memory_class,omitempty"`
}

type v2CandidatePoolResponse struct {
	OK         bool                  `json:"ok"`
	Target     string                `json:"target"`
	Mode       string                `json:"mode"`
	ConfigSHA  string                `json:"config_sha256"`
	Candidates []v2CandidatePoolItem `json:"candidates"`
	Count      int                   `json:"count"`
	Sources    []string              `json:"sources"`
	Warnings   []string              `json:"warnings"`
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

func registerCandidatePoolV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/candidates", getOnly(handleV2CandidatePool))
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

func v2AppendPoolItem(out []v2CandidatePoolItem, seen map[string]bool, item v2CandidatePoolItem) []v2CandidatePoolItem {
	item.Args = v2PortableCandidateArgs(item.Args)
	if len(item.Args) == 0 {
		return out
	}
	item.Fingerprint = v2CandidateTechniqueFingerprint(item.Args)
	if item.Fingerprint == "" || seen[item.Fingerprint] {
		return out
	}
	seen[item.Fingerprint] = true
	return append(out, item)
}

func v2BuildCandidatePool(target, mode string) (v2CandidatePoolResponse, error) {
	target, err := v2NormalizeTarget(target)
	if err != nil {
		return v2CandidatePoolResponse{}, err
	}
	m, err := v2SelectorMode(mode)
	if err != nil {
		return v2CandidatePoolResponse{}, err
	}
	status := readStatus()
	resp := v2CandidatePoolResponse{
		OK: true, Target: target, Mode: m.Name, ConfigSHA: status.ConfigSHA256,
		Candidates: []v2CandidatePoolItem{}, Warnings: []string{},
	}
	seen := map[string]bool{}

	memory, memoryErr := v2TargetMemoryCandidates(target, status.ConfigSHA256, m.MaxCandidates)
	if memoryErr != nil {
		resp.Warnings = append(resp.Warnings, "memory: "+memoryErr.Error())
	} else {
		for _, item := range memory {
			resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, item)
			if len(resp.Candidates) >= m.MaxCandidates {
				break
			}
		}
	}

	if len(resp.Candidates) < m.MaxCandidates {
		doc, libErr := readV2StrategyLibrary()
		if libErr != nil {
			resp.Warnings = append(resp.Warnings, "library: "+libErr.Error())
		} else {
			for _, item := range doc.Strategies {
				poolItem := v2CandidatePoolItem{
					ID: item.ID, Name: item.Name, Source: item.Source, Family: "saved",
					Protocol: "https", Args: append([]string{}, item.Args...),
				}
				resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, poolItem)
				if len(resp.Candidates) >= m.MaxCandidates {
					break
				}
			}
		}
	}

	if len(resp.Candidates) < m.MaxCandidates {
		for _, item := range v2BuiltinHTTPSCandidates {
			if _, compileErr := v2CustomProfile(item.Args, target); compileErr != nil {
				resp.Warnings = append(resp.Warnings, item.ID+": "+compileErr.Error())
				continue
			}
			poolItem := v2CandidatePoolItem{
				ID: item.ID, Name: item.Name, Source: "builtin", Family: item.Family,
				Protocol: item.Protocol, Args: append([]string{}, item.Args...),
			}
			resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, poolItem)
			if len(resp.Candidates) >= m.MaxCandidates {
				break
			}
		}
	}

	resp.Count = len(resp.Candidates)
	resp.Sources = v2CandidateSourceList(resp.Candidates)
	return resp, nil
}

func handleV2CandidatePool(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = "normal"
	}
	resp, err := v2BuildCandidatePool(target, mode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
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

	target, err := v2NormalizeTarget(req.ServerName)
	if err != nil {
		return meta, err
	}
	pool, err := v2BuildCandidatePool(target, req.Mode)
	if err != nil {
		return meta, err
	}
	meta.Sources = append(meta.Sources, pool.Sources...)
	meta.Warnings = append(meta.Warnings, pool.Warnings...)

	seen := map[string]bool{}
	for _, existing := range req.Candidates {
		seen[v2CandidateTechniqueFingerprint(existing.Args)] = true
	}

	for _, item := range pool.Candidates {
		if len(req.Candidates) >= 32 {
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
		switch item.Source {
		case "memory":
			meta.MemoryCandidates++
		case "builtin":
			meta.BuiltinCandidates++
		default:
			meta.LibraryCandidates++
		}
	}
	return meta, nil
}
