package lite

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func p192Mangle(dnsRules ...string) string {
	return "-P PREROUTING ACCEPT\n-N traffic_pre\n-A PREROUTING -j traffic_pre\n" + strings.Join(dnsRules, "\n") + "\n"
}

func TestP192ArbitraryWANDoesNotConflict(t *testing.T) {
	p := GatePlan{Interface: "br0"}
	f := GateFamily{Family: "inet", Set: gateSet4, Prefixes: []string{"192.168.10.0/24"}}
	for _, wan := range []string{"eth3", "ppp0", "enp9s0", "wwan1"} {
		t.Run(wan, func(t *testing.T) {
			dump := p192Mangle(
				fmt.Sprintf("-A traffic_pre -i %s -m mark --mark 0xffffaab/0xfffffff -j RETURN", wan),
				fmt.Sprintf("-A traffic_pre -i %s -p udp -m mark ! --mark 0x40000000/0x40000000 -m multiport --sports 443,590:600,49152:65535 -m connbytes --connbytes 1:15 --connbytes-mode packets --connbytes-dir reply -j NFQUEUE --queue-num 300 --queue-bypass", wan),
				fmt.Sprintf("-A traffic_pre -i %s -p tcp -m multiport --sports 80,443,5222,8443 -j NFQUEUE --queue-num 300 --queue-bypass", wan),
			)
			if err := inspectMangleDNS(dump, p, f); err != nil {
				t.Fatalf("WAN %s incorrectly blocks br0 DNS: %v", wan, err)
			}
		})
	}
}

func TestP192RealDNSInterceptionStillDenied(t *testing.T) {
	p := GatePlan{Interface: "br0"}
	f := GateFamily{Family: "inet", Set: gateSet4, Prefixes: []string{"192.168.10.0/24"}}
	for _, tc := range []struct {
		name string
		rule string
	}{
		{"udp tproxy", "-A traffic_pre -i br0 -p udp --dport 53 -j TPROXY --on-port 1601"},
		{"tcp nfqueue", "-A traffic_pre -i br0 -p tcp --dport 53 -j NFQUEUE --queue-num 300 --queue-bypass"},
		{"udp queue source port", "-A traffic_pre -i br0 -p udp -m multiport --sports 443,49152:65535 -j NFQUEUE --queue-num 300 --queue-bypass"},
		{"udp redirect", "-A traffic_pre -i br0 -p udp --dports 53,5353 -j REDIRECT --to-ports 53"},
		{"tcp drop", "-A traffic_pre -i br0 -p tcp --dport 53 -j DROP"},
		{"uncertain other", "-A traffic_pre -i br0 -p udp --dport 53 -j UNRECOGNISED_INTERCEPTOR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := inspectMangleDNS(p192Mangle(tc.rule), p, f); err == nil {
				t.Fatal("DNS-capable mangle interference accepted")
			}
		})
	}
}

func TestP192PortsProtocolsNegationAndScopes(t *testing.T) {
	p := GatePlan{Interface: "br0"}
	f := GateFamily{Family: "inet", Set: gateSet4, Prefixes: []string{"192.168.10.0/24"}}
	for _, tc := range []struct{ name, rule string }{
		{"different port", "-A traffic_pre -i br0 -p udp --dports 443,590:600,49152:65535 -j NFQUEUE --queue-num 300"},
		{"other protocol", "-A traffic_pre -i br0 -p icmp -j NFQUEUE --queue-num 300"},
		{"negated dns port", "-A traffic_pre -i br0 -p udp ! --dport 53 -j NFQUEUE --queue-num 300"},
		{"different client source", "-A traffic_pre -i br0 -s 172.16.0.0/16 -p udp --dport 53 -j NFQUEUE --queue-num 300"},
		{"different interface", "-A traffic_pre -i wan+ -p udp --dport 53 -j NFQUEUE --queue-num 300"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := inspectMangleDNS(p192Mangle(tc.rule), p, f); err != nil {
				t.Fatalf("proven non-overlap reported as DNS conflict: %v", err)
			}
		})
	}
	for _, tc := range []struct{ name, rule string }{
		{"wildcard LAN", "-A traffic_pre -i br+ -p udp --dport 53 -j NFQUEUE --queue-num 300"},
		{"inverted other interface", "-A traffic_pre ! -i eth3 -p udp --dport 53 -j NFQUEUE --queue-num 300"},
		{"inverted non dns port", "-A traffic_pre -i br0 -p tcp ! --dports 443,8443 -j NFQUEUE --queue-num 300"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := inspectMangleDNS(p192Mangle(tc.rule), p, f); err == nil {
				t.Fatal("matching DNS packet incorrectly cleared")
			}
		})
	}
}

func TestP192NoFalseCycleFromRepeatedNFQUEUE(t *testing.T) {
	p := GatePlan{Interface: "br0"}
	f := GateFamily{Family: "inet", Set: gateSet4, Prefixes: []string{"192.168.10.0/24"}}
	dump := p192Mangle(
		"-A traffic_pre -i ppp0 -p udp -m multiport --sports 443 -j NFQUEUE --queue-num 300",
		"-A traffic_pre -i ppp0 -p tcp -m multiport --sports 443 -j NFQUEUE --queue-num 300",
	)
	if err := inspectMangleDNS(dump, p, f); err != nil {
		t.Fatal("repeated terminal action incorrectly treated as chain cycle:", err)
	}
}

func TestP192GoGateE2EPreflight(t *testing.T) {
	g, m, _ := gateFixture(t)
	m.mu.Lock()
	m.mangle["iptables"]["PREROUTING"] = [][]string{{"-A", "PREROUTING", "-j", "traffic_pre"}}
	m.mangle["iptables"]["traffic_pre"] = [][]string{
		{"-A", "traffic_pre", "-i", "ppp0", "-p", "udp", "-m", "multiport", "--sports", "443,49152:65535", "-j", "NFQUEUE", "--queue-num", "300"},
		{"-A", "traffic_pre", "-i", "ppp0", "-p", "tcp", "-m", "multiport", "--sports", "443", "-j", "NFQUEUE", "--queue-num", "300"},
	}
	m.mu.Unlock()
	if err := g.Enable(context.Background(), "192.168.10.1:55358"); err != nil {
		t.Fatal("nonoverlapping traffic wrongly rejects DNS Gate:", err)
	}
	if g.Status()["state"] != "ACTIVE" {
		t.Fatal("Gate did not activate in simulated firewall")
	}
	if err := g.Disable(context.Background()); err != nil {
		t.Fatal("simulated cleanup failed:", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.mangle["iptables"]["traffic_pre"]) != 2 {
		t.Fatal("foreign mangle rules changed")
	}
}
