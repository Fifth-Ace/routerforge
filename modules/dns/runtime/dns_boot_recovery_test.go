package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDNSSystemNDMCEnv(t *testing.T) {
	in := []string{"PATH=/opt/bin", "LD_LIBRARY_PATH=/opt/lib", "LD_PRELOAD=/opt/lib/evil.so", "LD_AUDIT=evil", "HOME=/opt/root"}
	got := strings.Join(systemNDMCEnv(in), "\n")
	if strings.Contains(got, "evil") || strings.Contains(got, "/opt/lib") || !strings.Contains(got, "LD_LIBRARY_PATH=/lib:/usr/lib") {
		t.Fatalf("unsafe ndmc environment: %s", got)
	}
	if !strings.Contains(got, "HOME=/opt/root") {
		t.Fatal("non-loader environment lost")
	}
}

func TestDNSBootGateReadinessAndCancellation(t *testing.T) {
	dnsConfigureColdBoot(time.Hour)
	defer dnsConfigureColdBoot(0)
	phase, remaining := dnsBootStatus()
	if phase != "initializing" || remaining == 0 {
		t.Fatalf("phase=%q remaining=%d", phase, remaining)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := dnsWaitColdBoot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	dnsConfigureColdBoot(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	phase, _ = dnsBootStatus()
	if phase != "probing" {
		t.Fatalf("phase=%q, expected probing", phase)
	}
	dnsEndBootProbe()
	phase, _ = dnsBootStatus()
	if phase != "ready" {
		t.Fatalf("phase=%q, expected ready", phase)
	}
}

func TestDNSInfoRecoversWithoutNewRequest(t *testing.T) {
	var calls atomic.Int32
	cache := newDNSInfoCache(25*time.Millisecond, func() (RouterDNSInfoSnapshot, error) {
		if calls.Add(1) < 3 {
			return RouterDNSInfoSnapshot{}, errors.New("temporary ndmc initialization failure")
		}
		return RouterDNSInfoSnapshot{Proxies: []RouterDNSProxyInfo{{Name: "System"}}}, nil
	})
	cache.retryDelays = []time.Duration{5 * time.Millisecond, 10 * time.Millisecond}
	if _, err := cache.snapshotForRequest(); err == nil {
		t.Fatal("cold failure must not return fake 200")
	}
	deadline := time.Now().Add(time.Second)
	for {
		cache.mu.Lock()
		ready := !cache.scanned.IsZero()
		cache.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("automatic background recovery did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected exactly three attempts, got %d", got)
	}
}
