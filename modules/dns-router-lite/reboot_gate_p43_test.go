package lite

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// The boot-only grace is bounded and only admits known transient NDM reload
// signatures. It never relaxes foreign-rule/ownership preflight checks.
func TestP43BootGraceRetriesMissingGateChainThenRecovers(t *testing.T) {
	g, model, kernel := gateFixture(t)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	broken := true
	original := kernel.Tables.Exec
	kernel.Tables.Exec = func(ctx context.Context, name string, args ...string) (string, error) {
		if broken && name == "iptables" && strings.Contains(" "+strings.Join(args, " ")+" ", " -N RF_DNS_LITE_GATE ") {
			return "iptables: No chain/target/match by that name.", errors.New("exit status 1")
		}
		return original(ctx, name, args...)
	}
	if err := g.Restore(context.Background(), "192.168.10.1:55358"); err == nil {
		t.Fatal("transient boot failure was silently accepted")
	}
	for i := 0; i < 5; i++ {
		now = now.Add(4 * time.Second)
		if err := g.Reconcile(context.Background(), false); err == nil {
			t.Fatal("transient failure unexpectedly cleared")
		}
		status := g.Status()
		if status["state"] != "DEGRADED" || status["recovery_attempts"] != i+2 {
			t.Fatalf("bounded boot grace lost at %d: %#v", i, status)
		}
	}
	broken = false
	if err := g.Reconcile(context.Background(), false); err != nil {
		t.Fatalf("boot stabilization did not restore Gate: %v", err)
	}
	status := g.Status()
	if status["state"] != "ACTIVE" || status["active"] != true || status["recovery_attempts"] != 0 {
		t.Fatalf("Gate not healthy after delayed restore: %#v", status)
	}
	if got := model.packet("inet", "br0", "192.168.10.55", "9.9.9.9", "udp", 53); got != "ROUTERFORGE:55358" {
		t.Fatalf("LAN DNS flow not restored: %q", got)
	}
}

func TestP43BootGraceNeverRetriesForeignConflict(t *testing.T) {
	g, _, _ := gateFixture(t)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	g.bootRetryUntil = now.Add(bootGateRetryWindow)
	g.failed(&GateConflict{Evidence: "foreign rule in iptables/RF_DNS_LITE_GATE"})
	if s := g.Status(); s["state"] != "CONFLICT" || s["recovery_attempts"] != 3 {
		t.Fatalf("foreign-rule conflict must fail closed immediately: %#v", s)
	}
}

func TestP43BootGraceExpiresAndRequiresExplicitRetry(t *testing.T) {
	g, _, _ := gateFixture(t)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	g.bootRetryUntil = now.Add(bootGateRetryWindow)
	transient := errors.New("iptables -t nat -N RF_DNS_LITE_GATE: exit status 1: No chain/target/match by that name")
	for i := 0; i < bootGateRetryLimit; i++ {
		g.failed(transient)
		if s := g.Status(); s["state"] != "DEGRADED" {
			t.Fatalf("boot grace ended too early: %#v", s)
		}
	}
	for i := 0; i < 3; i++ {
		g.failed(transient)
	}
	if s := g.Status(); s["state"] != "RECOVERY_REQUIRED" || s["recovery_attempts"] != 3 {
		t.Fatalf("boot retry budget not bounded: %#v", s)
	}
}

func TestP43FirewallIOReportsExactFailingCommand(t *testing.T) {
	arg := "-test.p43-intentionally-invalid"
	_, err := runFirewallIOBounded(context.Background(), time.Second, os.Args[0], arg)
	if err == nil || !strings.Contains(err.Error(), arg) {
		t.Fatalf("firewall operation lost exact argument on failure: %v", err)
	}
}

func TestP43BootGraceDeadlineAndSpecificConflict(t *testing.T) {
	g, _, _ := gateFixture(t)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	g.bootRetryUntil = now.Add(bootGateRetryWindow)
	g.failed(&GateConflict{Evidence: "topology changed while enabling iptables"})
	if s := g.Status(); s["state"] != "DEGRADED" || s["recovery_attempts"] != 1 {
		t.Fatalf("temporary NDM rule replacement not retried: %#v", s)
	}
	now = now.Add(bootGateRetryWindow + time.Second)
	g.failed(errors.New("iptables: exit status 1: No chain/target/match by that name"))
	if s := g.Status(); s["recovery_attempts"] != 1 || s["state"] != "DEGRADED" {
		t.Fatalf("deadline not enforced: %#v", s)
	}
	for i := 0; i < 2; i++ {
		g.failed(errors.New("iptables: exit status 1: No chain/target/match by that name"))
	}
	if s := g.Status(); s["state"] != "RECOVERY_REQUIRED" || s["recovery_attempts"] != 3 {
		t.Fatalf("persistent error did not fail closed: %#v", s)
	}
}
