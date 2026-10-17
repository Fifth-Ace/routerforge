//go:build routerforge_dns_module

package main

import (
	"errors"
	"testing"
	"time"
)

func TestDNSBootReadinessImmediate(t *testing.T) {
	calls, slept := 0, time.Duration(0)
	n, err := waitDNSBootReadiness(func() error { calls++; return nil }, 60*time.Second, 3*time.Second, func(d time.Duration) { slept += d })
	if err != nil || n != 1 || calls != 1 || slept != 0 {
		t.Fatalf("n=%d calls=%d slept=%s err=%v", n, calls, slept, err)
	}
}
func TestDNSBootReadinessDelayed(t *testing.T) {
	calls, slept := 0, time.Duration(0)
	n, err := waitDNSBootReadiness(func() error {
		calls++
		if calls < 4 {
			return errors.New("not ready")
		}
		return nil
	}, 60*time.Second, 3*time.Second, func(d time.Duration) { slept += d })
	if err != nil || n != 4 || slept != 9*time.Second {
		t.Fatalf("n=%d slept=%s err=%v", n, slept, err)
	}
}
func TestDNSBootReadinessTimeout(t *testing.T) {
	slept := time.Duration(0)
	n, err := waitDNSBootReadiness(func() error { return errors.New("not ready") }, 60*time.Second, 3*time.Second, func(d time.Duration) { slept += d })
	if err == nil || n != 21 || slept != 60*time.Second {
		t.Fatalf("n=%d slept=%s err=%v", n, slept, err)
	}
}
