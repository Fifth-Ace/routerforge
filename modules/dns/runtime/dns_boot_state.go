package main

import (
	"context"
	"sync"
	"time"
)

// The optional cold-boot delay is shared by every DNS ndmc caller.
// Manual service restarts use zero delay. It does not block the service socket.
var dnsStartup = struct {
	mu        sync.RWMutex
	notBefore time.Time
	probing   bool
}{}

func dnsConfigureColdBoot(delay time.Duration) {
	dnsStartup.mu.Lock()
	defer dnsStartup.mu.Unlock()
	if delay <= 0 {
		dnsStartup.notBefore = time.Time{}
		dnsStartup.probing = false
		return
	}
	dnsStartup.notBefore = time.Now().Add(delay)
	dnsStartup.probing = true
}

func dnsBootStatus() (string, int) {
	dnsStartup.mu.RLock()
	until, probing := dnsStartup.notBefore, dnsStartup.probing
	dnsStartup.mu.RUnlock()
	if remaining := time.Until(until); remaining > 0 {
		return "initializing", int((remaining + time.Second - 1) / time.Second)
	}
	if probing {
		return "probing", 0
	}
	return "ready", 0
}

func dnsWaitColdBoot(ctx context.Context) error {
	dnsStartup.mu.RLock()
	until := dnsStartup.notBefore
	dnsStartup.mu.RUnlock()
	remaining := time.Until(until)
	if remaining <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func dnsEndBootProbe() {
	dnsStartup.mu.Lock()
	dnsStartup.probing = false
	dnsStartup.mu.Unlock()
}
