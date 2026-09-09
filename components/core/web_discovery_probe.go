package main

import (
	"context"
	"crypto/tls"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	rfWebDiscoveryProbeTimeout  = 450 * time.Millisecond
	rfWebDiscoveryBodyLimit     = 32 << 10
	rfWebDiscoveryHeaderLimit   = 64 << 10
	rfWebDiscoveryMaxListeners  = 48
	rfWebDiscoveryWorkerCount   = 6
	rfWebDiscoveryMaxTitleRunes = 160
)

var (
	rfWebTitlePattern      = regexp.MustCompile(`(?is)<title(?:\s[^>]*)?>(.*?)</title>`)
	rfProbeWebListenerFunc = rfProbeWebListener
)

type rfDetectedWebSurface struct {
	Address                string               `json:"address"`
	Port                   int                  `json:"port"`
	Scheme                 string               `json:"scheme"`
	ProbeURL               string               `json:"probe_url"`
	Path                   string               `json:"path"`
	StatusCode             int                  `json:"status_code"`
	ContentType            string               `json:"content_type,omitempty"`
	Title                  string               `json:"title,omitempty"`
	Redirect               string               `json:"redirect,omitempty"`
	TLSVerificationSkipped bool                 `json:"tls_verification_skipped,omitempty"`
	Owners                 []rfWebListenerOwner `json:"owners,omitempty"`
}

type rfWebProbeObservation struct {
	Surface   rfDetectedWebSurface
	HTTP      bool
	HTML      bool
	LikelyUI  bool
	TLSHint   bool
	BodyBytes int
}

func rfDetectLocalWebSurfaces(ctx context.Context, listeners []rfWebListener) []rfDetectedWebSurface {
	if len(listeners) == 0 {
		return nil
	}
	listeners = rfFilterActiveProbeWebListeners(listeners)
	if len(listeners) == 0 {
		return nil
	}
	if len(listeners) > rfWebDiscoveryMaxListeners {
		listeners = listeners[:rfWebDiscoveryMaxListeners]
	}

	workerCount := rfWebDiscoveryWorkerCount
	if workerCount > len(listeners) {
		workerCount = len(listeners)
	}

	jobs := make(chan rfWebListener)
	results := make(chan rfDetectedWebSurface, len(listeners))
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for listener := range jobs {
				if surface, ok := rfDetectWebSurface(ctx, listener); ok {
					select {
					case results <- surface:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, listener := range listeners {
			select {
			case jobs <- listener:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	close(results)

	surfaces := make([]rfDetectedWebSurface, 0, len(results))
	seen := make(map[string]struct{})
	for surface := range results {
		key := surface.Scheme + "|" + surface.ProbeURL
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(i, j int) bool {
		if surfaces[i].Port != surfaces[j].Port {
			return surfaces[i].Port < surfaces[j].Port
		}
		if surfaces[i].Address != surfaces[j].Address {
			return surfaces[i].Address < surfaces[j].Address
		}
		return surfaces[i].Scheme < surfaces[j].Scheme
	})
	return surfaces
}

func rfFilterActiveProbeWebListeners(listeners []rfWebListener) []rfWebListener {
	if len(listeners) == 0 {
		return nil
	}
	filtered := make([]rfWebListener, 0, len(listeners))
	for _, listener := range listeners {
		if rfShouldActiveProbeWebListener(listener) {
			filtered = append(filtered, listener)
		}
	}
	return filtered
}

func rfShouldActiveProbeWebListener(listener rfWebListener) bool {
	if listener.Port < 1 || listener.Port > 65535 {
		return false
	}
	if rfWebDiscoveryKnownNonWebPort(listener.Port) {
		return false
	}
	if rfWebDiscoveryKnownWebPort(listener.Port) {
		return true
	}
	return rfWebDiscoveryOwnerAllowsActiveProbe(listener.Owners)
}

func rfWebDiscoveryKnownNonWebPort(port int) bool {
	switch port {
	case 1, 7, 9, 13, 17, 19,
		20, 21, 22, 23, 25, 37, 53, 67, 68, 69,
		79, 88, 109, 110, 111, 113, 119, 123,
		135, 137, 138, 139, 143, 161, 162, 389,
		445, 465, 514, 515, 587, 631, 636, 873,
		989, 990, 993, 995, 1194, 1723, 1812, 1813,
		3306, 3389, 5432, 5900, 6379, 11211, 27017:
		return true
	default:
		return false
	}
}

func rfWebDiscoveryKnownWebPort(port int) bool {
	switch port {
	case 80, 81, 82, 443,
		8000, 8008, 8080, 8081, 8088, 8181,
		8443, 8888, 9000, 9090, 9443, 10443,
		2222, 2233:
		return true
	default:
		return false
	}
}

func rfWebDiscoveryOwnerAllowsActiveProbe(owners []rfWebListenerOwner) bool {
	for _, owner := range owners {
		if rfWebDiscoveryOwnerLooksNonWebInfrastructure(owner) {
			continue
		}
		if rfWebDiscoveryOwnerLooksWebCapable(owner) {
			return true
		}
	}
	return false
}

func rfWebDiscoveryOwnerLooksNonWebInfrastructure(owner rfWebListenerOwner) bool {
	corpus := rfWebDiscoveryOwnerCorpus(owner)
	for _, marker := range []string{
		"dropbear", "openssh", "sshd", "ssh",
		"telnet", "telnetd",
		"samba", "smbd", "nmbd", "wsdd", "ksmbd", "tsmb",
		"dnsmasq", "named", "unbound", "stubby", "smartdns",
		"pppd", "xl2tpd", "openvpn", "wireguard", "amneziawg",
		"postgres", "mysql", "mariadb", "redis", "memcached", "mongodb",
	} {
		if strings.Contains(corpus, marker) {
			return true
		}
	}
	return false
}

func rfWebDiscoveryOwnerLooksWebCapable(owner rfWebListenerOwner) bool {
	corpus := rfWebDiscoveryOwnerCorpus(owner)
	for _, marker := range []string{
		"routerforge", "awg-manager",
		"http", "https", "web", "ui", "panel", "dashboard", "admin",
		"lighttpd", "nginx", "uhttpd", "apache", "caddy", "traefik",
		"node", "npm", "python", "gunicorn", "uvicorn",
	} {
		if strings.Contains(corpus, marker) {
			return true
		}
	}
	return rfWebDiscoveryOwnerIsOptPackage(owner)
}

func rfWebDiscoveryOwnerCorpus(owner rfWebListenerOwner) string {
	parts := []string{owner.Package, owner.Process, owner.Executable}
	parts = append(parts, owner.Command...)
	return strings.ToLower(strings.Join(parts, " "))
}

func rfWebDiscoveryOwnerIsOptPackage(owner rfWebListenerOwner) bool {
	pkg := strings.TrimSpace(owner.Package)
	if pkg == "" {
		return false
	}
	executable := filepath.Clean(strings.TrimSpace(owner.Executable))
	if executable == "." || executable == string(filepath.Separator) {
		return false
	}
	return executable == "/opt" || strings.HasPrefix(executable, "/opt/")
}

func rfDetectWebSurface(ctx context.Context, listener rfWebListener) (rfDetectedWebSurface, bool) {
	if !rfShouldActiveProbeWebListener(listener) {
		return rfDetectedWebSurface{}, false
	}
	host, ok := rfWebDiscoveryProbeHost(listener.Address)
	if !ok {
		return rfDetectedWebSurface{}, false
	}
	for _, scheme := range rfWebDiscoverySchemeOrder(listener.Port) {
		observation := rfProbeWebListenerFunc(ctx, listener, host, scheme)
		if observation.LikelyUI {
			return observation.Surface, true
		}
	}
	return rfDetectedWebSurface{}, false
}

func rfWebDiscoveryProbeHost(address string) (string, bool) {
	address = strings.TrimSpace(strings.Trim(address, "[]"))
	ip := net.ParseIP(address)
	if ip == nil {
		return "", false
	}
	if ip.IsUnspecified() {
		if ip.To4() != nil {
			return "127.0.0.1", true
		}
		return "::1", true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return "", false
	}
	return ip.String(), true
}

func rfWebDiscoverySchemeOrder(port int) []string {
	switch port {
	case 443, 4443, 8443, 9443, 10443:
		return []string{"https", "http"}
	default:
		return []string{"http", "https"}
	}
}

func rfProbeWebListener(parent context.Context, listener rfWebListener, host, scheme string) rfWebProbeObservation {
	result := rfWebProbeObservation{}
	if listener.Port < 1 || listener.Port > 65535 {
		return result
	}

	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, strconv.Itoa(listener.Port)),
		Path:   "/",
	}
	ctx, cancel := context.WithTimeout(parent, rfWebDiscoveryProbeTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return result
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("User-Agent", "RouterForge web-discovery")

	client := rfNewWebDiscoveryClient(host, scheme == "https")
	response, err := client.Do(request)
	if err != nil {
		return result
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, rfWebDiscoveryBodyLimit))
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	htmlBody := rfLooksLikeHTML(contentType, body)
	redirect := rfSafeWebDiscoveryRedirect(target, response.Header.Get("Location"))
	tlsHint := scheme == "http" && rfLooksLikeTLSRequired(body)
	authUI := rfLooksLikeAuthWebUI(response.StatusCode, htmlBody, body)

	result.HTTP = true
	result.HTML = htmlBody
	result.TLSHint = tlsHint
	result.BodyBytes = len(body)
	result.LikelyUI = rfLooksLikeWebUIResponse(response.StatusCode, htmlBody, redirect, tlsHint, authUI)
	result.Surface = rfDetectedWebSurface{
		Address:                listener.Address,
		Port:                   listener.Port,
		Scheme:                 scheme,
		ProbeURL:               target.String(),
		Path:                   "/",
		StatusCode:             response.StatusCode,
		ContentType:            contentType,
		Title:                  rfWebDiscoveryTitle(body),
		Redirect:               redirect,
		TLSVerificationSkipped: scheme == "https",
		Owners:                 append([]rfWebListenerOwner(nil), listener.Owners...),
	}
	if redirect != "" {
		if parsed, err := url.Parse(redirect); err == nil && parsed.Path != "" {
			result.Surface.Path = parsed.Path
		}
	}
	return result
}

func rfNewWebDiscoveryClient(host string, tlsProbe bool) *http.Client {
	expected := net.ParseIP(strings.Trim(host, "[]"))
	transport := &http.Transport{
		Proxy:                  nil,
		DisableKeepAlives:      true,
		ResponseHeaderTimeout:  rfWebDiscoveryProbeTimeout,
		TLSHandshakeTimeout:    rfWebDiscoveryProbeTimeout,
		MaxResponseHeaderBytes: rfWebDiscoveryHeaderLimit,
	}
	if tlsProbe {
		transport.TLSClientConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, // Discovery only: no credentials/cookies are sent.
		}
	}
	dialer := &net.Dialer{Timeout: rfWebDiscoveryProbeTimeout}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialHost, dialPort, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		actual := net.ParseIP(strings.Trim(dialHost, "[]"))
		if expected == nil || actual == nil || !actual.Equal(expected) {
			return nil, &net.AddrError{Err: "web discovery blocked unexpected dial", Addr: address}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(expected.String(), dialPort))
	}
	return &http.Client{
		Transport: transport,
		Timeout:   rfWebDiscoveryProbeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func rfLooksLikeHTML(contentType string, body []byte) bool {
	lowerType := strings.ToLower(contentType)
	if strings.Contains(lowerType, "text/html") || strings.Contains(lowerType, "application/xhtml+xml") {
		return true
	}
	trimmed := strings.TrimSpace(strings.ToLower(string(body)))
	if strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html") {
		return true
	}
	return strings.Contains(trimmed, "<html") && (strings.Contains(trimmed, "<head") || strings.Contains(trimmed, "<body"))
}

func rfLooksLikeTLSRequired(body []byte) bool {
	text := strings.ToLower(string(body))
	for _, marker := range []string{
		"plain http request was sent to https port",
		"client sent an http request to an https server",
		"http request to an https server",
		"speaking plain http to an ssl-enabled server port",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func rfLooksLikeAuthWebUI(status int, htmlBody bool, body []byte) bool {
	if !htmlBody || (status != http.StatusUnauthorized && status != http.StatusForbidden) {
		return false
	}

	text := strings.ToLower(string(body))
	for _, marker := range []string{
		`type="password"`,
		`type='password'`,
		`autocomplete="current-password"`,
		`autocomplete='current-password'`,
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}

	title := strings.ToLower(rfWebDiscoveryTitle(body))
	for _, marker := range []string{
		"login",
		"log in",
		"sign in",
		"authentication",
		"authorization",
		"username",
		"password",
	} {
		if strings.Contains(title, marker) {
			return true
		}
	}
	return false
}

func rfLooksLikeWebUIResponse(status int, htmlBody bool, redirect string, tlsHint bool, authUI bool) bool {
	if tlsHint {
		return false
	}
	if redirect != "" && status >= 300 && status < 400 {
		return true
	}
	if !htmlBody {
		return false
	}
	return (status >= 200 && status < 300) || authUI
}

func rfSafeWebDiscoveryRedirect(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n") {
		return ""
	}
	target, err := base.Parse(raw)
	if err != nil || target.User != nil {
		return ""
	}
	if !strings.EqualFold(target.Scheme, base.Scheme) || !strings.EqualFold(target.Host, base.Host) {
		return ""
	}
	if target.Path == "" || !strings.HasPrefix(target.Path, "/") {
		return ""
	}
	return target.String()
}

func rfWebDiscoveryTitle(body []byte) string {
	match := rfWebTitlePattern.FindSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	title := html.UnescapeString(string(match[1]))
	title = strings.Join(strings.Fields(title), " ")
	runes := []rune(title)
	if len(runes) > rfWebDiscoveryMaxTitleRunes {
		title = string(runes[:rfWebDiscoveryMaxTitleRunes])
	}
	return title
}
