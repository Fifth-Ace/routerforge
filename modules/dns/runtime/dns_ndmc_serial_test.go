package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDNSNDMCSerializesCalls(t *testing.T) {
	old := dnsNDMCRunner
	defer func() { dnsNDMCRunner = old }()
	var active, peak int32
	dnsNDMCRunner = func(ctx context.Context, program string, args ...string) ([]byte, error) {
		n := atomic.AddInt32(&active, 1)
		if n > 1 {
			atomic.StoreInt32(&peak, n)
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return []byte("ok"), nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := dnsNDMCCommand(context.Background(), "show dns-proxy")
			if err != nil || string(out) != "ok" {
				t.Errorf("out=%q err=%v", out, err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&peak); got != 0 {
		t.Fatalf("concurrent ndmc invocation detected: %d", got)
	}
}

func TestDNSNDMCWaitObeysContext(t *testing.T) {
	dnsNDMCGate <- struct{}{}
	defer func() { <-dnsNDMCGate }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	_, err := dnsNDMCCommand(ctx, "show dns-proxy")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}

func TestDNSNDMCNonExitErrorIsNotRetried(t *testing.T) {
	old := dnsNDMCRunner
	defer func() { dnsNDMCRunner = old }()
	count := 0
	dnsNDMCRunner = func(ctx context.Context, program string, args ...string) ([]byte, error) {
		count++
		return nil, errors.New("executor failure")
	}
	_, err := dnsNDMCCommand(context.Background(), "show dns-proxy")
	if err == nil || count != 1 {
		t.Fatalf("unexpected retry: count=%d err=%v", count, err)
	}
}
