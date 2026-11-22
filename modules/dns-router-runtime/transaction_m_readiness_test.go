package dnsrouter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDPR2MReadinessRetriesWithFreshNames(t *testing.T) {
	var seen = map[string]bool{}
	attempts := 0
	start := time.Now()
	err := probeNDNReadyWithQueryM(context.Background(), "udp", "127.0.0.1:53", 0, func(_ context.Context, network, dest, name string, id uint16) error {
		attempts++
		if network != "udp" || dest != "127.0.0.1:53" || !strings.HasSuffix(name, ".dpr2m.test") || seen[name] || id == 0 {
			return errors.New("invalid attempt")
		}
		seen[name] = true
		if attempts < 3 {
			return fmt.Errorf("synthetic transient rcode=3")
		}
		return nil
	})
	if err != nil || attempts != 3 || len(seen) != 3 {
		t.Fatalf("err=%v attempts=%d names=%v", err, attempts, seen)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("readiness retry unexpectedly slow")
	}
}
func TestDPR2MReadinessPermanentNegativeRejects(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err := probeNDNReadyWithQueryM(ctx, "tcp", "192.168.10.1:53", 3, func(_ context.Context, network, dest, name string, id uint16) error {
		attempts++
		return errors.New("permanent rcode=5")
	})
	if err == nil || attempts != 12 || !strings.Contains(err.Error(), "permanent rcode=5") {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}
func TestDPR2MProbeRejectsWrongTargetAndZone(t *testing.T) {
	ctx := context.Background()
	if e := probeDNSM(ctx, "udp", "192.168.10.1:53", "probe.example.com", 0x1222); e == nil {
		t.Fatal("accepted untrusted zone")
	}
	if e := probeDNSM(ctx, "udp", "127.0.0.1:5353", "p0-0.probe.dpr2m.test", 0x1222); e == nil {
		t.Fatal("accepted untrusted target")
	}
	if e := probeDNSM(ctx, "icmp", "127.0.0.1:53", "p0-0.probe.dpr2m.test", 0x1222); e == nil {
		t.Fatal("accepted invalid protocol")
	}
}
