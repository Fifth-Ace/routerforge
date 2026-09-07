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
		ancestors string
		want      string
	}{
		{name: "none", want: "no-blocking-header-detected"},
		{name: "xfo deny", xfo: "DENY", want: "blocked"},
		{name: "xfo sameorigin", xfo: "SAMEORIGIN", want: "blocked"},
		{name: "csp none", ancestors: "'none'", want: "blocked"},
		{name: "csp wildcard", ancestors: "*", want: "no-blocking-header-detected"},
		{name: "csp self", ancestors: "'self'", want: "restricted"},
		{name: "csp explicit", ancestors: "http://router.local:2233", want: "restricted"},
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
