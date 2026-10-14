package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNF4CPostVerifyRequiresPublicExactIdentity(t *testing.T) {
	for _, bad := range [][2]string{
		{"example.com", ""}, {"", "93.184.216.34"},
		{"Example.com", "93.184.216.34"}, {"example.com", "127.0.0.1"},
		{"example.com", "10.0.0.1"}, {"example.com", "169.254.1.2"},
		{"example.com", "::1"}, {"example.com", "garbage"},
	} {
		if err := nf4cValidatePostApplyTarget(bad[0], bad[1]); err == nil {
			t.Errorf("accepted invalid post-apply identity: %q / %q", bad[0], bad[1])
		}
	}
	if err := nf4cValidatePostApplyTarget("example.com", "93.184.216.34"); err != nil {
		t.Fatal(err)
	}
	if err := nf4cValidatePostApplyTarget("", ""); err != nil {
		t.Fatalf("legacy Smart Apply must remain valid: %v", err)
	}
}

func TestNF4CPostVerifyRealProgress(t *testing.T) {
	called := 0
	probe := func(_ context.Context, h, ip string, port int) (v2HTTPMetrics, error) {
		called++
		if h != "example.com" || ip != "93.184.216.34" || port != 0 {
			t.Fatal("probe target or production routing drifted")
		}
		return v2HTTPMetrics{TLSComplete: true, ProgressProven: true, HTTPStatus: 200}, nil
	}
	if err := nf4cVerifyHTTPSWithProbe(context.Background(), "example.com", "93.184.216.34", probe); err != nil || called != 1 {
		t.Fatalf("verify=%v calls=%d", err, called)
	}
}

func TestNF4CPostVerifyRejectsTLSOnly(t *testing.T) {
	probe := func(context.Context, string, string, int) (v2HTTPMetrics, error) {
		return v2HTTPMetrics{TLSComplete: true, ProgressProven: false}, nil
	}
	if err := nf4cVerifyHTTPSWithProbe(context.Background(), "example.com", "93.184.216.34", probe); err == nil {
		t.Fatal("TLS handshake alone cannot count as target verification")
	}
}

func TestNF4CPostVerifyRetriesTransientFailure(t *testing.T) {
	calls := 0
	probe := func(context.Context, string, string, int) (v2HTTPMetrics, error) {
		calls++
		if calls == 1 {
			return v2HTTPMetrics{}, errors.New("temporary failure")
		}
		return v2HTTPMetrics{TLSComplete: true, ProgressProven: true, HTTPStatus: 403}, nil
	}
	if err := nf4cVerifyHTTPSWithProbe(context.Background(), "example.com", "93.184.216.34", probe); err != nil || calls != 2 {
		t.Fatalf("retry not performed: %v, %d", err, calls)
	}
}

func TestNF4CPostVerifyFailsClosed(t *testing.T) {
	calls := 0
	probe := func(context.Context, string, string, int) (v2HTTPMetrics, error) {
		calls++
		return v2HTTPMetrics{}, errors.New("blocked")
	}
	err := nf4cVerifyHTTPSWithProbe(context.Background(), "example.com", "93.184.216.34", probe)
	if err == nil || !strings.Contains(err.Error(), "blocked") || calls != 2 {
		t.Fatalf("fail closed expected: %v calls=%d", err, calls)
	}
}

func TestNF4CPostVerifyHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	probe := func(context.Context, string, string, int) (v2HTTPMetrics, error) {
		calls++
		return v2HTTPMetrics{}, nil
	}
	if err := nf4cVerifyHTTPSWithProbe(ctx, "example.com", "93.184.216.34", probe); err == nil || calls != 0 {
		t.Fatalf("cancel not honoured: %v calls=%d", err, calls)
	}
}
