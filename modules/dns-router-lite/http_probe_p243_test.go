package lite

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type p243HTTPRoundTrip func(*http.Request) (*http.Response, error)

func (fn p243HTTPRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestP243HTTPReachabilityAndBoundedBody(t *testing.T) {
	target := ipProbeTarget{IP: netip.MustParseAddr("8.8.8.8"), Domain: "example.org", Port: 8443, Protocol: "https"}
	var calls int
	client := &http.Client{Transport: p243HTTPRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Host != "example.org:8443" || r.URL.Path != "/health" || r.URL.Scheme != "https" {
			t.Fatalf("wrong HTTP probe: %v", r)
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("alive")), Header: make(http.Header)}, nil
	})}
	if err := httpProbeRequest(context.Background(), target, "/health", client); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("unexpected HTTP requests", calls)
	}
}
func TestP243HTTPStatusAndCancellation(t *testing.T) {
	target := ipProbeTarget{IP: netip.MustParseAddr("8.8.8.8"), Domain: "example.org", Port: 443, Protocol: "https"}
	for _, status := range []int{200, 301, 403, 429, 503} {
		client := &http.Client{Transport: p243HTTPRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("hello")), Header: make(http.Header)}, nil
		})}
		err := httpProbeRequest(context.Background(), target, "/", client)
		if (err == nil) != (status < 500) {
			t.Fatalf("status %d err %v", status, err)
		}
	}
	cancel, cfn := context.WithCancel(context.Background())
	cfn()
	client := &http.Client{Transport: p243HTTPRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() == nil {
			t.Fatal("missing cancellation")
		}
		return nil, r.Context().Err()
	})}
	if err := httpProbeRequest(cancel, target, "/", client); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}
