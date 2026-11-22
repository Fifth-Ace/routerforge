package lite

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

func TestP243MinDownHoldAndRecoveryThreshold(t *testing.T) {
	m := p242Monitor()
	m.cfg.RecoveryThreshold = 3
	m.cfg.MinDownSeconds = 90
	q, a := p242Answer("1.1.1.1")
	m.Observe(q, a)
	target := ipProbeTarget{IP: netip.MustParseAddr("1.1.1.1"), Domain: "www.example.org", Port: 443, Protocol: "tcp"}
	fails := true
	m.probe = func(context.Context, ipProbeTarget) (time.Duration, error) {
		if fails {
			return 0, errors.New("offline")
		}
		return 10 * time.Millisecond, nil
	}
	m.measure(context.Background(), target)
	m.measure(context.Background(), target)
	if p242ViewFor(t, m, "1.1.1.1").Status != "DOWN" {
		t.Fatal("DOWN not reached")
	}
	fails = false
	for i := 0; i < 3; i++ {
		m.measure(context.Background(), target)
	}
	if p242ViewFor(t, m, "1.1.1.1").Status != "DOWN" {
		t.Fatal("bypassed hold")
	}
	m.mu.Lock()
	m.entries[target].DownAt = time.Now().Add(-91 * time.Second)
	m.mu.Unlock()
	m.measure(context.Background(), target)
	if p242ViewFor(t, m, "1.1.1.1").Status != "UP" {
		t.Fatal("did not recover after hold")
	}
}
