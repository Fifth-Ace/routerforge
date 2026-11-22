package lite

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A successful early boot activation must not cancel the bounded grace period:
// NDM may rebuild its NAT/filter rules after the DNS listener first becomes ready.
func TestP44BootGraceSurvivesSuccessAndLateNDMReload(t *testing.T) {
	g, model, kernel := gateFixture(t)
	start := time.Now()
	var clock atomic.Int64
	clock.Store(start.UnixNano())
	g.now = func() time.Time { return time.Unix(0, clock.Load()) }

	if err := g.Restore(context.Background(), "192.168.10.1:55358"); err != nil {
		t.Fatalf("early Gate activation failed: %v", err)
	}
	g.mu.Lock()
	initialDeadline := g.bootRetryUntil
	g.mu.Unlock()
	if initialDeadline.IsZero() || !initialDeadline.After(start) {
		t.Fatal("first successful activation canceled the cold-boot recovery deadline")
	}

	original := kernel.Tables.Exec
	var reload atomic.Bool
	kernel.Tables.Exec = func(ctx context.Context, name string, args ...string) (string, error) {
		if reload.Load() && name == "iptables" && strings.Contains(" "+strings.Join(args, " ")+" ", " -t nat -S ") {
			return "iptables: No chain/target/match by that name.", errors.New("exit status 1")
		}
		return original(ctx, name, args...)
	}

	// NDM briefly replaces the table AFTER the initial successful activation.
	reload.Store(true)
	clock.Store(start.Add(4 * time.Second).UnixNano())
	if err := g.Reconcile(context.Background(), false); err == nil {
		t.Fatal("an injected NDM reload unexpectedly succeeded")
	}
	if state := g.Status(); state["state"] != "DEGRADED" || state["recovery_attempts"] != 1 {
		t.Fatalf("transient post-success reload incorrectly failed closed: %#v", state)
	}

	// The table stabilizes and Gate recovers, still inside the same boot window.
	reload.Store(false)
	clock.Store(start.Add(8 * time.Second).UnixNano())
	if err := g.Reconcile(context.Background(), false); err != nil {
		t.Fatalf("Gate did not recover after post-success NDM reload: %v", err)
	}
	g.mu.Lock()
	afterDeadline := g.bootRetryUntil
	g.mu.Unlock()
	if !afterDeadline.Equal(initialDeadline) {
		t.Fatalf("recovery success reset the original boot deadline: %s -> %s", initialDeadline, afterDeadline)
	}

	// Another short reload in the original grace period remains recoverable.
	reload.Store(true)
	clock.Store(start.Add(12 * time.Second).UnixNano())
	if err := g.Reconcile(context.Background(), false); err == nil {
		t.Fatal("second injected NDM reload unexpectedly succeeded")
	}
	if state := g.Status(); state["state"] != "DEGRADED" || state["recovery_attempts"] != 1 {
		t.Fatalf("second boot reload incorrectly exhausted recovery budget: %#v", state)
	}
	reload.Store(false)
	clock.Store(start.Add(16 * time.Second).UnixNano())
	if err := g.Reconcile(context.Background(), false); err != nil {
		t.Fatalf("Gate did not recover after second NDM reload: %v", err)
	}
	if state := g.Status(); state["state"] != "ACTIVE" || state["active"] != true {
		t.Fatalf("Gate not active after two recoverable boot reloads: %#v", state)
	}
	if got := model.packet("inet", "br0", "192.168.10.55", "9.9.9.9", "udp", 53); got != "ROUTERFORGE:55358" {
		t.Fatalf("LAN DNS capture missing after recovery: %q", got)
	}
}

func TestP44BootGraceStillRejectsForeignRulesAndExpires(t *testing.T) {
	g, _, _ := gateFixture(t)
	start := time.Now()
	var clock atomic.Int64
	clock.Store(start.UnixNano())
	g.now = func() time.Time { return time.Unix(0, clock.Load()) }
	if err := g.Restore(context.Background(), "192.168.10.1:55358"); err != nil {
		t.Fatal(err)
	}
	g.failed(&GateConflict{Evidence: "foreign rule in iptables/RF_DNS_LITE_GATE"})
	if s := g.Status(); s["state"] != "CONFLICT" || s["recovery_attempts"] != 3 {
		t.Fatalf("foreign rules must always fail closed: %#v", s)
	}

	g2, _, _ := gateFixture(t)
	g2.now = func() time.Time { return time.Unix(0, clock.Load()) }
	if err := g2.Restore(context.Background(), "192.168.10.1:55358"); err != nil {
		t.Fatal(err)
	}
	clock.Store(start.Add(bootGateRetryWindow + time.Second).UnixNano())
	g2.failed(&GateConflict{Evidence: "topology changed while enabling iptables"})
	if s := g2.Status(); s["state"] != "CONFLICT" || s["recovery_attempts"] != 3 {
		t.Fatalf("boot deadline must bound retries after initial success: %#v", s)
	}
}
