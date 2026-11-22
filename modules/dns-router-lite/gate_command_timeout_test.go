package lite

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// The helper is an isolated test process. No iptables, ipset or router files
// are accessed. Production uses the same bounded executor implementation.
func TestGateFirewallCommandTimeoutReported(t *testing.T) {
	if os.Getenv("RF_FIREWALL_TIMEOUT_HELPER") == "1" {
		time.Sleep(4 * time.Second)
		return
	}
	t.Setenv("RF_FIREWALL_TIMEOUT_HELPER", "1")
	_, err := runFirewallIOBounded(context.Background(), 60*time.Millisecond, os.Args[0], "-test.run=^TestGateFirewallCommandTimeoutReported$")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "RF_FIREWALL_COMMAND_TIMEOUT") {
		t.Fatalf("timeout must preserve deadline and diagnostic reason: %v", err)
	}
	if _, err := runFirewallIOBounded(context.Background(), 0, os.Args[0]); err == nil {
		t.Fatal("unbounded command budget accepted")
	}
}

func TestGateFirewallParentCancellationReported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runFirewallIOBounded(ctx, time.Second, os.Args[0], "-test.run=^TestGateFirewallParentCancellationReported$")
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "RF_FIREWALL_COMMAND_CANCELLED") {
		t.Fatalf("parent cancellation lost: %v", err)
	}
}
