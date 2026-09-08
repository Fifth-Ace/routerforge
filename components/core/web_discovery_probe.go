package main

import (
	"context"
	"crypto/tls"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
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

var rfWebTitlePattern = regexp.MustCompile(`(?is)<title(?:\s[^>]*)?>(.*?)</title>`)

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

func rfDetectWebSurface(ctx context.Context, listener rfWebListener) (rfDetectedWebSurface, bool) {
	host, ok := rfWebDiscoveryProbeHost(listener.Address)
	if !ok {
		return rfDetectedWebSurface{}, false
	}
	for _, scheme := range rfWebDiscoverySchemeOrder(listener.Port) {
		observation := rfProbeWebListener(ctx, listener, host, scheme)
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
