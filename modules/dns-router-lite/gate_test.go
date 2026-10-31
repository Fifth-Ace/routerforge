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
