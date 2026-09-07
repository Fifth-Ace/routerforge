package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type catalogWebProbeDoerFunc func(*http.Request) (*http.Response, error)

func (f catalogWebProbeDoerFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCatalogWebProbeUsesLoopbackWithoutCredentials(t *testing.T) {
	item := catalogItem{
		ID:        "nfqws-web",
		Installed: true,
		Trust:     catalogTrust{Status: "verified"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   90,
			Path:   "/ui",
			Mode:   "external-only",
			Embed:  false,
		},
	}

	var seen *http.Request
	doer := catalogWebProbeDoerFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.Clone(r.Context())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"X-Frame-Options":         []string{"SAMEORIGIN"},
				"Content-Security-Policy": []string{"default-src 'self'; frame-ancestors 'self'"},
				"Set-Cookie":              []string{"session=secret; HttpOnly; SameSite=Lax"},
			},
			Body: io.NopCloser(strings.NewReader("ignored")),
		}, nil
	})

	result := probeCatalogWeb(context.Background(), item, doer)

	if seen == nil {
		t.Fatal("probe request was not sent")
	}
	if seen.Method != http.MethodHead {
		t.Fatalf("unexpected method: %s", seen.Method)
	}
	if seen.URL.Scheme != "http" || seen.URL.Host != "127.0.0.1:90" || seen.URL.Path != "/ui" {
		t.Fatalf("unexpected loopback target: %s", seen.URL.String())
	}
	if seen.Header.Get("Cookie") != "" || seen.Header.Get("Authorization") != "" {
		t.Fatalf("probe leaked credentials: %#v", seen.Header)
	}
	if !result.Reachable || result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected reachability result: %#v", result)
	}
	if result.FrameHeaderPolicy != "blocked" {
		t.Fatalf("expected blocked frame policy, got %q", result.FrameHeaderPolicy)
	}
	if result.CSPFrameAncestors != "'self'" {
		t.Fatalf("unexpected frame-ancestors: %q", result.CSPFrameAncestors)
	}
	if result.SetCookieCount != 1 {
		t.Fatalf("expected one Set-Cookie header summary, got %d", result.SetCookieCount)
	}
}

func TestCatalogWebProbeEligibility(t *testing.T) {
	probeWeb := &catalogWebMetadata{
		Scheme: "http",
		Port:   2222,
		Path:   "/",
		Mode:   "probe-required",
		Embed:  true,
	}

	tests := []struct {
		name string
		item catalogItem
		want bool
	}{
		{
			name: "verified registry metadata",
			item: catalogItem{
				Trust: catalogTrust{Status: "verified"},
				Web:   probeWeb,
			},
			want: true,
		},
		{
			name: "official registry metadata",
			item: catalogItem{
				Trust: catalogTrust{Status: "official"},
				Web:   probeWeb,
			},
			want: true,
		},
		{
			name: "built-in legacy fallback probe metadata",
			item: catalogItem{
				Trust:          catalogTrust{Status: "unverified"},
				RegistrySource: "legacy-fallback",
				WebPortSource:  "project-default",
				Web:            probeWeb,
			},
			want: true,
		},
		{
			name: "remote unverified metadata",
			item: catalogItem{
				Trust:          catalogTrust{Status: "unverified"},
				RegistrySource: "routerforge-community",
				WebPortSource:  "project-default",
				Web:            probeWeb,
			},
			want: false,
		},
		{
			name: "legacy fallback external-only metadata",
			item: catalogItem{
				Trust:          catalogTrust{Status: "unverified"},
				RegistrySource: "legacy-fallback",
				WebPortSource:  "project-default",
				Web: &catalogWebMetadata{
					Scheme: "http",
					Port:   2222,
					Path:   "/",
					Mode:   "external-only",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := catalogWebProbeAllowed(tt.item); got != tt.want {
				t.Fatalf("got %v, want %v for %#v", got, tt.want, tt.item)
			}
		})
	}
}

func TestCatalogOrderWebHostsPrefersCurrentInterface(t *testing.T) {
	got := catalogOrderWebHosts(
		[]string{"192.168.1.252", "192.168.1.252"},
		[]string{"192.168.10.1", "172.20.12.1", "192.168.1.252", "192.168.10.1"},
	)
	want := []string{"192.168.1.252", "172.20.12.1", "192.168.10.1"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestCatalogOrderWebHostsRejectsNonLocalPreferredHost(t *testing.T) {
	got := catalogOrderWebHosts(
		[]string{"203.0.113.8"},
		[]string{"192.168.10.1"},
	)
	if len(got) != 1 || got[0] != "192.168.10.1" {
		t.Fatalf("unexpected host order: %#v", got)
	}
}

func TestCatalogWebExactHostClientRejectsDifferentDialTarget(t *testing.T) {
	client, err := newCatalogWebExactHostClient("192.168.10.1")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type: %T", client.Transport)
	}
	conn, err := transport.DialContext(context.Background(), "tcp", "192.168.1.252:2222")
	if conn != nil {
		conn.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "blocked unexpected dial") {
		t.Fatalf("expected exact-host dial rejection, got %v", err)
	}
}

func TestCatalogWebProbeClientRejectsRedirects(t *testing.T) {
	client := newCatalogWebProbeClient()
	err := client.CheckRedirect(
		&http.Request{},
		[]*http.Request{{}},
	)
	if !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy must stop at first response: %v", err)
	}
}

func TestCatalogWebProbeTransportRejectsNonLoopbackDial(t *testing.T) {
	client := newCatalogWebProbeClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type: %T", client.Transport)
	}
	conn, err := transport.DialContext(context.Background(), "tcp", "example.com:80")
	if conn != nil {
		conn.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "blocked non-loopback dial") {
		t.Fatalf("expected non-loopback dial rejection, got %v", err)
	}
}

func TestCatalogFrameHeaderPolicy(t *testing.T) {
	tests := []struct {
		name      string
		xfo       string
		ancestors []string
		want      string
	}{
		{name: "none", want: "no-blocking-header-detected"},
		{name: "xfo deny", xfo: "DENY", want: "blocked"},
		{name: "xfo sameorigin", xfo: "SAMEORIGIN", want: "blocked"},
		{name: "xfo unknown", xfo: "ALLOW-FROM http://router.local", ancestors: []string{"*"}, want: "restricted"},
		{name: "csp none", ancestors: []string{"'none'"}, want: "blocked"},
		{name: "csp wildcard", ancestors: []string{"*"}, want: "no-blocking-header-detected"},
		{name: "csp wildcard host", ancestors: []string{"https://*.example.com"}, want: "restricted"},
		{name: "csp self", ancestors: []string{"'self'"}, want: "restricted"},
		{name: "csp explicit", ancestors: []string{"http://router.local:2233"}, want: "restricted"},
		{name: "multiple wildcard", ancestors: []string{"*", "*"}, want: "no-blocking-header-detected"},
		{name: "multiple block", ancestors: []string{"*", "'none'"}, want: "blocked"},
		{name: "multiple restrict", ancestors: []string{"*", "'self'"}, want: "restricted"},
		{name: "empty directive", ancestors: []string{""}, want: "blocked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := catalogFrameHeaderPolicy(tt.xfo, tt.ancestors); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCatalogFrameAncestorsExtraction(t *testing.T) {
	csp := "default-src 'self'; img-src *; frame-ancestors 'self' http://router.local:2233; script-src 'self'"
	got := catalogFrameAncestors(csp)
	want := "'self' http://router.local:2233"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCatalogFrameAncestorsPoliciesKeepAllHeaders(t *testing.T) {
	got := catalogFrameAncestorsPolicies([]string{
		"default-src 'self'; frame-ancestors *",
		"img-src *; frame-ancestors 'none'",
	})
	if len(got) != 2 || got[0] != "*" || got[1] != "'none'" {
		t.Fatalf("unexpected frame-ancestors policies: %#v", got)
	}
}

func TestCatalogWebProbeMultipleCSPHeadersAreConservative(t *testing.T) {
	item := catalogItem{
		ID:        "nfqws-web",
		Installed: true,
		Trust:     catalogTrust{Status: "verified"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   90,
			Path:   "/",
			Mode:   "embedded-supported",
			Embed:  true,
		},
	}

	doer := catalogWebProbeDoerFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Security-Policy": []string{
					"default-src 'self'; frame-ancestors *",
					"frame-ancestors 'none'",
				},
			},
			Body: io.NopCloser(strings.NewReader("")),
		}, nil
	})

	result := probeCatalogWeb(context.Background(), item, doer)
	if result.CSPFrameAncestors != "* | 'none'" {
		t.Fatalf("unexpected CSP summary: %q", result.CSPFrameAncestors)
	}
	if result.FrameHeaderPolicy != "blocked" {
		t.Fatalf("multiple enforced CSP headers must block, got %q", result.FrameHeaderPolicy)
	}
}
