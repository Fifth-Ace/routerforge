package lite

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestP15GatePortAndNoImplicitActivation(t *testing.T) {
	for _, bad := range []string{"0.0.0.0:53", "192.168.10.1:53", "localhost:55358", "192.168.10.1:abc"} {
		if _, e := gatePort(bad); e == nil {
			t.Fatalf("unsafe gateway accepted %q", bad)
		}
	}
	g := NewGateManager()
	if g.Status()["active"] != false || g.Status()["state"] != "STOCK" {
		t.Fatal("gate automatically activated")
	}
}
func TestP15GateExplicitDualStackAndCleanup(t *testing.T) {
	listener, e := net.Listen("tcp6", "[::1]:0")
	if e != nil {
		t.Skip("IPv6 disabled in CI environment")
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	g := NewGateManager()
	g.candidates = func() (string, []string, error) { return "192.168.10.0/24", []string{"fe80::/64"}, nil }
	g.locate = func(string) (string, error) { return "/mock", nil }
	calls := []string{}
	g.read = func(_ context.Context, _ string, _ ...string) (string, error) { return "Name: owned", nil }
	g.run = func(_ context.Context, b string, a ...string) error {
		cmd := b + " " + strings.Join(a, " ")
		calls = append(calls, cmd)
		if (strings.Contains(cmd, "-S RF_DNS_LITE_GATE") || strings.Contains(cmd, "list RF_DNS_LITE")) && !strings.Contains(cmd, "-A") {
			return errors.New("not found")
		}
		return nil
	}
	if e := g.Enable(context.Background(), net.JoinHostPort("192.168.10.1", itoa(port))); e != nil {
		t.Fatal(e)
	}
	if g.Status()["active"] != true {
		t.Fatal("gate not active")
	}
	seen4, seen6, lease := false, false, false
	for _, c := range calls {
		if strings.Contains(c, "iptables -t nat -I PREROUTING") {
			seen4 = true
		}
		if strings.Contains(c, "ip6tables -t nat -I PREROUTING") {
			seen6 = true
		}
		if strings.Contains(c, "ipset add RF_DNS_LITE4") {
			lease = true
		}
	}
	if !seen4 || !seen6 || !lease {
		t.Fatalf("IPv4/IPv6 gate missing: %v", calls)
	}
	if e := g.Disable(context.Background()); e != nil {
		t.Fatal(e)
	}
	if g.Status()["active"] != false {
		t.Fatal("gate not disabled")
	}
	flushed := false
	for _, c := range calls {
		if c == "ipset flush RF_DNS_LITE4" {
			flushed = true
		}
	}
	if !flushed {
		t.Fatal("lease was not removed before firewall shutdown")
	}
}
func itoa(n int) string { return strconv.Itoa(n) }

// P16 R1: removal is idempotent if firmware already removed all owned objects.
func TestP16R1DisableWithAllGateObjectsAbsent(t *testing.T) {
	g := NewGateManager()
	g.lastError = "old cleanup failed on missing ipset and chains"
	g.active = false
	g.port = ""
	mutations := 0
	g.run = func(_ context.Context, _ string, _ ...string) error {
		mutations++
		return errors.New("no mutation permitted in already-clean state")
	}
	g.read = func(_ context.Context, binary string, args ...string) (string, error) {
		for _, arg := range args {
			if arg == "PREROUTING" {
				return "-P PREROUTING ACCEPT\n-A PREROUTING -j _NDM_DNS_REDIRECT\n", nil
			}
		}
		if binary == "ipset" {
			return "ipset: The set with the given name does not exist", errors.New("exit status 1")
		}
		return "iptables: No chain/target/match by that name.", errors.New("exit status 1")
	}
	if err := g.Disable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := g.Status()["state"]; got != "STOCK" {
		t.Fatalf("expected STOCK, got %v", got)
	}
	if got := g.Status()["last_error"]; got != "" {
		t.Fatalf("stale error retained: %v", got)
	}
	if err := g.Disable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if mutations != 0 {
		t.Fatalf("unexpected kernel mutations: %d", mutations)
	}
}

func TestP16R1DisableRefusesOrphanedJump(t *testing.T) {
	g := NewGateManager()
	g.lastError = "old cleanup error"
	g.run = func(_ context.Context, _ string, _ ...string) error {
		t.Fatal("no kernel mutation allowed")
		return nil
	}
	g.read = func(_ context.Context, binary string, args ...string) (string, error) {
		for _, arg := range args {
			if arg == "PREROUTING" {
				return "-P PREROUTING ACCEPT\n-A PREROUTING -i br0 -j RF_DNS_LITE_GATE\n", nil
			}
		}
		if binary == "ipset" {
			return "The set with the given name does not exist", errors.New("exit status 1")
		}
		return "No chain/target/match by that name", errors.New("exit status 1")
	}
	if err := g.Disable(context.Background()); err == nil || !strings.Contains(err.Error(), "orphaned") {
		t.Fatalf("orphaned jump must fail closed: %v", err)
	}
	if g.Status()["state"] != "RECOVERY_REQUIRED" {
		t.Fatal("lost recovery state")
	}
}

func TestP16R1DisableDoesNotConfusePermissionWithMissing(t *testing.T) {
	g := NewGateManager()
	g.lastError = "previous cleanup error"
	g.run = func(_ context.Context, _ string, _ ...string) error {
		t.Fatal("no kernel mutation allowed")
		return nil
	}
	g.read = func(_ context.Context, _ string, _ ...string) (string, error) {
		return "Permission denied", errors.New("exit status 1")
	}
	if err := g.Disable(context.Background()); err == nil {
		t.Fatal("permission error silently ignored")
	}
	if g.Status()["state"] != "RECOVERY_REQUIRED" {
		t.Fatal("cleared error without a valid inspection")
	}
}
