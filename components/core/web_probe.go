package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const catalogWebProbeTimeout = 2500 * time.Millisecond

type catalogWebProbeRequest struct {
	ID string `json:"id"`
}

type catalogWebProbeDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type catalogWebProbeResult struct {
	ID                     string    `json:"id"`
	Mode                   string    `json:"mode"`
	Embed                  bool      `json:"embed"`
	Scheme                 string    `json:"scheme"`
	Port                   int       `json:"port"`
	Path                   string    `json:"path"`
	BrowserURLs            []string  `json:"browser_urls,omitempty"`
	Reachable              bool      `json:"reachable"`
	StatusCode             int       `json:"status_code,omitempty"`
	Redirect               bool      `json:"redirect"`
	XFrameOptions          string    `json:"x_frame_options,omitempty"`
	CSPFrameAncestors      string    `json:"csp_frame_ancestors,omitempty"`
	AccessControlAllowOrig string    `json:"access_control_allow_origin,omitempty"`
	SetCookieCount         int       `json:"set_cookie_count"`
	FrameHeaderPolicy      string    `json:"frame_header_policy"`
	Error                  string    `json:"error,omitempty"`
	ProbedAt               time.Time `json:"probed_at"`
}

func registerCatalogWebProbeHandler(mux *http.ServeMux) {
	mux.HandleFunc("/api/catalog/web-probe", handleCatalogWebProbe)
}

func safeCatalogWebProbeID(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func catalogWebProbeAllowed(item catalogItem) bool {
	switch strings.ToLower(strings.TrimSpace(item.Trust.Status)) {
	case "official", "verified":
		return true
	}
	return item.RegistrySource == "legacy-fallback" &&
		item.WebPortSource != "" &&
		item.Web != nil &&
		item.Web.Mode == "probe-required"
}

func handleCatalogWebProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin web probe request rejected"})
		return
	}

	var request catalogWebProbeRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid web probe request"})
		return
	}

	request.ID = strings.TrimSpace(request.ID)
	if !safeCatalogWebProbeID(request.ID) {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "valid catalog item id is required"})
		return
	}

	item, ok := catalogItemByID(request.ID)
	if !ok {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "catalog item not found"})
		return
	}
	if item.Web == nil {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "catalog item has no typed web metadata"})
		return
	}
	if err := validateCatalogWebMetadata(item.Web); err != nil {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{"error": "catalog web metadata failed validation"})
		return
	}

	if !catalogWebProbeAllowed(item) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "web probe requires trusted or built-in local web metadata"})
		return
	}

	if !item.Installed {
		writeCatalogJSON(w, http.StatusConflict, map[string]any{"error": "catalog item is not installed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), catalogWebProbeTimeout)
	defer cancel()

	result := probeCatalogWeb(ctx, item, nil)
	if result.Reachable &&
		result.StatusCode >= http.StatusOK &&
		result.StatusCode < http.StatusMultipleChoices &&
		!result.Redirect &&
		result.FrameHeaderPolicy == "no-blocking-header-detected" {
		result.BrowserURLs = catalogWebBrowserURLs(ctx, item, r)
	}
	writeCatalogJSON(w, http.StatusOK, result)
}

func probeCatalogWeb(ctx context.Context, item catalogItem, doer catalogWebProbeDoer) catalogWebProbeResult {
	meta := item.Web
	result := catalogWebProbeResult{
		ID:                item.ID,
		Mode:              meta.Mode,
		Embed:             meta.Embed,
		Scheme:            meta.Scheme,
		Port:              meta.Port,
		Path:              meta.Path,
		FrameHeaderPolicy: "unknown",
		ProbedAt:          time.Now().UTC(),
	}

	if result.Scheme == "" {
		result.Scheme = "http"
	}
	if result.Path == "" {
		result.Path = "/"
	}

	target := &url.URL{
		Scheme: result.Scheme,
		Host:   net.JoinHostPort("127.0.0.1", strconv.Itoa(result.Port)),
		Path:   result.Path,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Accept", "text/html,*/*;q=0.1")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "RouterForge/"+version+" web-probe")

	if doer == nil {
		doer = newCatalogWebProbeClient()
	}

	resp, err := doer.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	result.Reachable = true
	result.StatusCode = resp.StatusCode
	result.Redirect = resp.StatusCode >= 300 && resp.StatusCode <= 399
	result.XFrameOptions = strings.TrimSpace(strings.Join(resp.Header.Values("X-Frame-Options"), ", "))
	frameAncestors := catalogFrameAncestorsPolicies(resp.Header.Values("Content-Security-Policy"))
	result.CSPFrameAncestors = strings.Join(frameAncestors, " | ")
	result.AccessControlAllowOrig = strings.TrimSpace(resp.Header.Get("Access-Control-Allow-Origin"))
	result.SetCookieCount = len(resp.Header.Values("Set-Cookie"))
	result.FrameHeaderPolicy = catalogFrameHeaderPolicy(result.XFrameOptions, frameAncestors)

	return result
}

const (
	catalogWebBrowserCandidateTimeout = 600 * time.Millisecond
	catalogWebBrowserCandidateLimit   = 16
)

func catalogWebBrowserURLs(ctx context.Context, item catalogItem, r *http.Request) []string {
	if item.Web == nil {
		return nil
	}

	hosts := catalogLocalWebHosts(r)
	if len(hosts) > catalogWebBrowserCandidateLimit {
		hosts = hosts[:catalogWebBrowserCandidateLimit]
	}

	urls := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if candidate, ok := probeCatalogWebBrowserURL(ctx, item, host); ok {
			urls = append(urls, candidate)
		}
	}
	return urls
}

func catalogLocalWebHosts(r *http.Request) []string {
	discovered := make([]string, 0, 8)
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			addresses, addrErr := iface.Addrs()
			if addrErr != nil {
				continue
			}
			for _, address := range addresses {
				var ip net.IP
				switch value := address.(type) {
				case *net.IPNet:
					ip = value.IP
				case *net.IPAddr:
					ip = value.IP
				}
				if ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
					continue
				}
				discovered = append(discovered, ip.String())
			}
		}
	}

	preferred := make([]string, 0, 2)
	if r != nil {
		if host := catalogWebIPHost(r.Host); host != "" {
			preferred = append(preferred, host)
		}
		if local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && local != nil {
			if host := catalogWebIPHost(local.String()); host != "" {
				preferred = append(preferred, host)
			}
		}
	}
	return catalogOrderWebHosts(preferred, discovered)
}

func catalogWebIPHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	ip := net.ParseIP(value)
	if ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
		return ""
	}
	return ip.String()
}

func catalogOrderWebHosts(preferred, discovered []string) []string {
	available := make(map[string]struct{}, len(discovered))
	for _, raw := range discovered {
		host := catalogWebIPHost(raw)
		if host != "" {
			available[host] = struct{}{}
		}
	}

	rest := make([]string, 0, len(available))
	for host := range available {
		rest = append(rest, host)
	}
	sort.Strings(rest)

	out := make([]string, 0, len(rest))
	used := make(map[string]struct{}, len(rest))
	for _, raw := range preferred {
		host := catalogWebIPHost(raw)
		if host == "" {
			continue
		}
		if _, ok := available[host]; !ok {
			continue
		}
		if _, ok := used[host]; ok {
			continue
		}
		used[host] = struct{}{}
		out = append(out, host)
	}
	for _, host := range rest {
		if _, ok := used[host]; ok {
			continue
		}
		used[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func probeCatalogWebBrowserURL(parent context.Context, item catalogItem, host string) (string, bool) {
	if item.Web == nil {
		return "", false
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
		return "", false
	}

	scheme := item.Web.Scheme
	if scheme == "" {
		scheme = "http"
	}
	path := item.Web.Path
	if path == "" {
		path = "/"
	}

	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(ip.String(), strconv.Itoa(item.Web.Port)),
		Path:   path,
	}

	ctx, cancel := context.WithTimeout(parent, catalogWebBrowserCandidateTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		return "", false
	}
	request.Header.Set("Accept", "text/html,*/*;q=0.1")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("User-Agent", "RouterForge/"+version+" web-browser-candidate")

	client, err := newCatalogWebExactHostClient(ip.String())
	if err != nil {
		return "", false
	}

	response, err := client.Do(request)
	if err != nil {
		return "", false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", false
	}

	frameAncestors := catalogFrameAncestorsPolicies(response.Header.Values("Content-Security-Policy"))
	if catalogFrameHeaderPolicy(
		strings.TrimSpace(strings.Join(response.Header.Values("X-Frame-Options"), ", ")),
		frameAncestors,
	) != "no-blocking-header-detected" {
		return "", false
	}

	return target.String(), true
}

func newCatalogWebExactHostClient(host string) (*http.Client, error) {
	expected := net.ParseIP(strings.Trim(host, "[]"))
	if expected == nil {
		return nil, errors.New("web candidate requires an IP host")
	}

	transport := &http.Transport{
		Proxy:                  nil,
		DisableKeepAlives:      true,
		ResponseHeaderTimeout:  catalogWebBrowserCandidateTimeout,
		TLSHandshakeTimeout:    catalogWebBrowserCandidateTimeout,
		MaxResponseHeaderBytes: 64 << 10,
	}
	dialer := &net.Dialer{Timeout: catalogWebBrowserCandidateTimeout}

	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialHost, dialPort, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		actual := net.ParseIP(strings.Trim(dialHost, "[]"))
		if actual == nil || !actual.Equal(expected) {
			return nil, errors.New("web candidate blocked unexpected dial")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(expected.String(), dialPort))
	}

	return &http.Client{
		Transport: transport,
		Timeout:   catalogWebBrowserCandidateTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func newCatalogWebProbeClient() *http.Client {
	base := &http.Transport{
		Proxy:                  nil,
		DisableKeepAlives:      true,
		ResponseHeaderTimeout:  2 * time.Second,
		TLSHandshakeTimeout:    2 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}

	dialer := &net.Dialer{
		Timeout: 1500 * time.Millisecond,
	}

	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("web probe blocked non-loopback dial")
		}
		return dialer.DialContext(ctx, network, address)
	}

	return &http.Client{
		Transport: base,
		Timeout:   catalogWebProbeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func catalogFrameAncestors(csp string) string {
	policies := catalogFrameAncestorsPolicies([]string{csp})
	if len(policies) == 0 {
		return ""
	}
	return policies[0]
}

func catalogFrameAncestorsPolicies(values []string) []string {
	policies := make([]string, 0, len(values))
	for _, csp := range values {
		for _, directive := range strings.Split(csp, ";") {
			fields := strings.Fields(strings.TrimSpace(directive))
			if len(fields) == 0 || !strings.EqualFold(fields[0], "frame-ancestors") {
				continue
			}
			policies = append(policies, strings.Join(fields[1:], " "))
			break
		}
	}
	return policies
}

func catalogFrameHeaderPolicy(xFrameOptions string, frameAncestors []string) string {
	xfo := strings.ToLower(strings.TrimSpace(xFrameOptions))
	if strings.Contains(xfo, "deny") || strings.Contains(xfo, "sameorigin") {
		return "blocked"
	}
	if xfo != "" {
		return "restricted"
	}
	if len(frameAncestors) == 0 {
		return "no-blocking-header-detected"
	}

	restricted := false
	for _, policy := range frameAncestors {
		tokens := strings.Fields(strings.ToLower(strings.TrimSpace(policy)))
		if len(tokens) == 0 {
			return "blocked"
		}
		wildcardAll := false
		for _, token := range tokens {
			if token == "'none'" {
				return "blocked"
			}
			if token == "*" {
				wildcardAll = true
			}
		}
		if !wildcardAll {
			restricted = true
		}
	}
	if restricted {
		return "restricted"
	}
	return "no-blocking-header-detected"
}
