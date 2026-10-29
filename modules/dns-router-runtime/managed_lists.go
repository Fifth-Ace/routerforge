package dnsrouter

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// DomainList is metadata for an explicitly imported set of DNS policy rules.
// Sources never change the native Keenetic resolver or an active shadow listener.
type DomainList struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Kind              string `json:"kind"` // paste, file, url
	URL               string `json:"url,omitempty"`
	Upstream          string `json:"upstream"`
	Match             string `json:"match"`
	Priority          int    `json:"priority"`
	AutoUpdateMinutes int    `json:"auto_update_minutes,omitempty"` // explicit opt-in: 0, 60, 360, 1440
	UpdatedAt         string `json:"updated_at,omitempty"`
}

type ManagedListRequest struct {
	List DomainList `json:"list"`
	Text string     `json:"text"`
}

type ListLineProblem struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type ManagedListPreview struct {
	Config     Config            `json:"config"`
	Added      int               `json:"added"`
	Duplicates int               `json:"duplicates"`
	Ignored    int               `json:"ignored"`
	Invalid    int               `json:"invalid"`
	Errors     []ListLineProblem `json:"errors"`
	TotalRules int               `json:"total_rules"`
	ListRules  int               `json:"list_rules"`
}

func validateManagedLists(cfg Config) error {
	if len(cfg.Lists) > 64 {
		return fmt.Errorf("too many domain lists: %d > 64", len(cfg.Lists))
	}
	upstreams := make(map[string]bool, len(cfg.Upstreams))
	for _, u := range cfg.Upstreams {
		upstreams[u.ID] = true
	}
	lists := make(map[string]DomainList, len(cfg.Lists))
	for _, list := range cfg.Lists {
		if !validID(list.ID) || len(list.ID) > 40 {
			return fmt.Errorf("invalid list id %q", list.ID)
		}
		if _, exists := lists[list.ID]; exists {
			return fmt.Errorf("duplicate list %q", list.ID)
		}
		if !utf8.ValidString(list.Name) || len([]rune(strings.TrimSpace(list.Name))) < 1 || len([]rune(list.Name)) > 80 {
			return fmt.Errorf("invalid name for list %q", list.ID)
		}
		if list.Kind != "paste" && list.Kind != "file" && list.Kind != "url" {
			return fmt.Errorf("invalid source kind for list %q", list.ID)
		}
		if !upstreams[list.Upstream] {
			return fmt.Errorf("unknown upstream for list %q", list.ID)
		}
		if list.Match != "suffix" && list.Match != "exact" && list.Match != "subdomains" {
			return fmt.Errorf("invalid match for list %q", list.ID)
		}
		if list.Priority < 0 || list.Priority > 1000000 {
			return fmt.Errorf("invalid priority for list %q", list.ID)
		}
		switch list.AutoUpdateMinutes {
		case 0, 60, 360, 1440:
		default:
			return fmt.Errorf("invalid auto update interval for list %q", list.ID)
		}
		if list.Kind == "url" {
			if _, err := validatePublicListURL(list.URL); err != nil {
				return fmt.Errorf("list %q URL: %w", list.ID, err)
			}
		} else if list.URL != "" || list.AutoUpdateMinutes != 0 {
			return fmt.Errorf("list %q: local sources cannot have URLs or auto update", list.ID)
		}
		if list.UpdatedAt != "" {
			if _, err := time.Parse(time.RFC3339, list.UpdatedAt); err != nil {
				return fmt.Errorf("list %q: bad updated_at", list.ID)
			}
		}
		lists[list.ID] = list
	}
	for _, r := range cfg.Rules {
		if r.ListID == "" {
			continue
		}
		list, ok := lists[r.ListID]
		if !ok {
			return fmt.Errorf("rule %q references missing list %q", r.ID, r.ListID)
		}
		if r.Upstream != list.Upstream || r.Match != list.Match || r.Priority != list.Priority {
			return fmt.Errorf("rule %q conflicts with list %q policy", r.ID, r.ListID)
		}
	}
	return nil
}

func listRuleID(listID, domain string) string {
	digest := sha256.Sum256([]byte(listID + "/" + domain))
	return "lst-" + hex.EncodeToString(digest[:12])
}

func normalizeListLine(line string) ([]string, bool, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "@@") {
		return nil, true, nil
	}
	// Common AdGuard DNS blocking syntax, not full cosmetic/URL-filter grammar.
	if strings.HasPrefix(trimmed, "||") && strings.HasSuffix(trimmed, "^") {
		trimmed = strings.TrimSuffix(strings.TrimPrefix(trimmed, "||"), "^")
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return nil, true, nil
	}
	if len(fields) >= 2 && len(fields) <= 12 {
		ip := net.ParseIP(fields[0])
		if ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
			fields = fields[1:]
		}
	}
	domains := make([]string, 0, len(fields))
	for _, d := range fields {
		if strings.HasPrefix(d, "#") {
			break
		}
		if strings.HasPrefix(d, "||") || strings.ContainsAny(d, "/^*?$|@[]") {
			return nil, false, errors.New("unsupported filtering expression")
		}
		normalized, err := normalizedDomain(d)
		if err != nil {
			return nil, false, fmt.Errorf("invalid domain %q", d)
		}
		if isProtected(normalized) {
			return nil, false, fmt.Errorf("protected local domain %q", normalized)
		}
		domains = append(domains, normalized)
	}
	if len(domains) == 0 {
		return nil, true, nil
	}
	return domains, false, nil
}

func PreviewManagedList(existing Config, req ManagedListRequest) (ManagedListPreview, error) {
	out := ManagedListPreview{Errors: make([]ListLineProblem, 0)}
	list := req.List
	if len(req.Text) > 1<<20 {
		return out, errors.New("list text exceeds 1 MiB")
	}
	if !utf8.ValidString(req.Text) {
		return out, errors.New("list text must be UTF-8")
	}
	// Match the validation contract before any existing list is replaced.
	seed := EmptyConfig()
	seed.Upstreams = append(seed.Upstreams, existing.Upstreams...)
	seed.Lists = []DomainList{list}
	if err := validateManagedLists(seed); err != nil {
		return out, err
	}
	cfg := existing
	cfg.Rules = make([]Rule, 0, len(existing.Rules))
	cfg.Lists = make([]DomainList, 0, len(existing.Lists)+1)
	found := false
	for _, old := range existing.Lists {
		if old.ID == list.ID {
			found = true
			continue
		}
		cfg.Lists = append(cfg.Lists, old)
	}
	_ = found
	cfg.Lists = append(cfg.Lists, list)
	used := map[string]bool{}
	usedID := map[string]bool{}
	for _, r := range existing.Rules {
		if r.ListID == list.ID {
			continue
		}
		cfg.Rules = append(cfg.Rules, r)
		used[r.Domain+"|"+r.Match+"|"+r.Upstream] = true
		usedID[r.ID] = true
	}
	scanner := bufio.NewScanner(strings.NewReader(req.Text))
	scanner.Buffer(make([]byte, 4096), 65536)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		domains, ignored, err := normalizeListLine(scanner.Text())
		if ignored {
			out.Ignored++
			continue
		}
		if err != nil {
			out.Invalid++
			if len(out.Errors) < 30 {
				out.Errors = append(out.Errors, ListLineProblem{Line: lineNo, Reason: err.Error()})
			}
			continue
		}
		for _, domain := range domains {
			key := domain + "|" + list.Match + "|" + list.Upstream
			if used[key] {
				out.Duplicates++
				continue
			}
			if len(cfg.Rules) >= MaxRules {
				return out, fmt.Errorf("list exceeds %d rules", MaxRules)
			}
			id := listRuleID(list.ID, domain)
			if usedID[id] {
				return out, errors.New("internal rule id collision")
			}
			used[key] = true
			usedID[id] = true
			cfg.Rules = append(cfg.Rules, Rule{ID: id, Domain: domain, Match: list.Match, Upstream: list.Upstream, Priority: list.Priority, ListID: list.ID})
			out.Added++
		}
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	if out.Added == 0 {
		return out, errors.New("list has no new valid domains; existing list unchanged")
	}
	if _, err := Compile(cfg); err != nil {
		return out, err
	}
	out.TotalRules = len(cfg.Rules)
	out.ListRules = out.Added
	out.Config = cfg
	return out, nil
}

// HTTPS only; no IP literals, credentials, redirects, local or special-use DNS destinations.
func validatePublicListURL(raw string) (*url.URL, error) {
	if len(raw) > 2048 {
		return nil, errors.New("URL too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return nil, errors.New("invalid URL")
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("HTTPS URL with public hostname required")
	}
	if u.Port() != "" && u.Port() != "443" {
		return nil, errors.New("only HTTPS port 443 allowed")
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".home.arpa") || strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") || strings.HasSuffix(host, ".example") || strings.HasSuffix(host, ".") {
		return nil, errors.New("public DNS hostname required")
	}
	if strings.ContainsAny(u.Host, "%@\\") {
		return nil, errors.New("unsafe authority")
	}
	if _, err := normalizedDomain(host); err != nil {
		return nil, errors.New("invalid hostname")
	}
	return u, nil
}

func publicListIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	ranges := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "fc00::/7", "fe80::/10", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48", "ff00::/8",
	}
	for _, s := range ranges {
		if netip.MustParsePrefix(s).Contains(addr) {
			return false
		}
	}
	return true
}

func FetchManagedListText(ctx context.Context, raw string) (string, error) {
	u, err := validatePublicListURL(raw)
	if err != nil {
		return "", err
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	ips, err := net.DefaultResolver.LookupIPAddr(lookupCtx, u.Hostname())
	cancel()
	if err != nil || len(ips) == 0 {
		return "", errors.New("public hostname resolution failed")
	}
	for _, ip := range ips {
		if !publicListIP(ip.IP) {
			return "", errors.New("DNS resolution includes private/special IP; blocked")
		}
	}
	// Resolve exactly once and pin the connection. Disable proxies and redirects.
	selected := ips[0].IP.String()
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = 5 * time.Second
			return d.DialContext(ctx, "tcp", net.JoinHostPort(selected, "443"))
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		DisableKeepAlives:     true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 15 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error { return errors.New("redirects are not allowed") }}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain, application/octet-stream;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > 1<<20 {
		return "", errors.New("list exceeds 1 MiB")
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") {
		return "", errors.New("HTML sources are not supported")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return "", err
	}
	if len(body) > 1<<20 {
		return "", errors.New("list exceeds 1 MiB")
	}
	if !utf8.Valid(body) {
		return "", errors.New("list source must be UTF-8")
	}
	return string(body), nil
}
