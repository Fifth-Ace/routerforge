package dnsrouter

import (
	"bufio"
	"fmt"
	"strings"
)

type ImportRequest struct {
	Text     string `json:"text"`
	Upstream string `json:"upstream"`
	Match    string `json:"match"`
	Priority int    `json:"priority"`
	Mode     string `json:"mode"` // append or replace
}

type ImportPreview struct {
	Config      Config `json:"config"`
	Added       int    `json:"added"`
	Duplicates  int    `json:"duplicates"`
	Ignored     int    `json:"ignored"`
	ResultRules int    `json:"result_rules"`
}

func PreviewImport(existing Config, req ImportRequest) (ImportPreview, error) {
	if len(req.Text) > 1<<20 {
		return ImportPreview{}, fmt.Errorf("import exceeds 1 MiB")
	}
	if req.Mode != "append" && req.Mode != "replace" {
		return ImportPreview{}, fmt.Errorf("mode must be append or replace")
	}
	if req.Match != "exact" && req.Match != "suffix" && req.Match != "subdomains" {
		return ImportPreview{}, fmt.Errorf("invalid match mode")
	}
	if req.Priority < 0 || req.Priority > 1000000 {
		return ImportPreview{}, fmt.Errorf("invalid priority")
	}
	pool := false
	for _, u := range existing.Upstreams {
		if u.ID == req.Upstream {
			pool = true
			break
		}
	}
	if !pool {
		return ImportPreview{}, fmt.Errorf("unknown upstream %q", req.Upstream)
	}
	result := EmptyConfig()
	result.Upstreams = append(result.Upstreams, existing.Upstreams...)
	if req.Mode == "append" {
		result.Rules = append(result.Rules, existing.Rules...)
	}
	usedIDs := make(map[string]bool, len(result.Rules))
	usedRules := make(map[string]bool, len(result.Rules))
	for _, r := range result.Rules {
		usedIDs[r.ID] = true
		usedRules[r.Domain+"|"+r.Match+"|"+r.Upstream] = true
	}
	p := ImportPreview{}
	sc := bufio.NewScanner(strings.NewReader(req.Text))
	sc.Buffer(make([]byte, 4096), 65536)
	next := 1
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			p.Ignored++
			continue
		}
		if len(strings.Fields(line)) != 1 {
			return ImportPreview{}, fmt.Errorf("line has whitespace: %q", line)
		}
		domain, err := normalizedDomain(line)
		if err != nil {
			return ImportPreview{}, fmt.Errorf("invalid domain %q: %w", line, err)
		}
		if isProtected(domain) {
			return ImportPreview{}, fmt.Errorf("protected local domain %q", domain)
		}
		key := domain + "|" + req.Match + "|" + req.Upstream
		if usedRules[key] {
			p.Duplicates++
			continue
		}
		if len(result.Rules) >= MaxRules {
			return ImportPreview{}, fmt.Errorf("import exceeds %d rules", MaxRules)
		}
		var id string
		for {
			id = fmt.Sprintf("import-%d", next)
			next++
			if !usedIDs[id] {
				break
			}
		}
		usedIDs[id], usedRules[key] = true, true
		result.Rules = append(result.Rules, Rule{ID: id, Domain: domain, Match: req.Match, Upstream: req.Upstream, Priority: req.Priority})
		p.Added++
	}
	if err := sc.Err(); err != nil {
		return ImportPreview{}, err
	}
	if _, err := Compile(result); err != nil {
		return ImportPreview{}, err
	}
	p.Config = result
	p.ResultRules = len(result.Rules)
	return p, nil
}
