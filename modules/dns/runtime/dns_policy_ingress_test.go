package main

import "testing"

func TestValidateDNSPolicyIngressConfig(t *testing.T) {
	cfg, err := validateDNSPolicyIngressConfig(DNSPolicyIngressConfig{Interface: "br0", ListenAddr: "192.168.10.1:55355"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interface != "br0" || cfg.ListenAddr != "192.168.10.1:55355" {
		t.Fatalf("cfg=%#v", cfg)
	}
}

func TestValidateDNSPolicyIngressConfigRejectsUnsafeTargets(t *testing.T) {
	for _, tc := range []DNSPolicyIngressConfig{
		{Interface: "", ListenAddr: "192.168.10.1:55355"},
		{Interface: "br0", ListenAddr: "127.0.0.1:55355"},
		{Interface: "br0", ListenAddr: "0.0.0.0:55355"},
		{Interface: "br0", ListenAddr: "192.168.10.1:53"},
	} {
		if _, err := validateDNSPolicyIngressConfig(tc); err == nil {
			t.Fatalf("unsafe cfg accepted: %#v", tc)
		}
	}
}

func TestDNSPolicyIngressRuleSpecsAreExact(t *testing.T) {
	jump := dnsPolicyIngressJumpArgs("br0", "udp")
	wantJump := []string{"-t", "nat", "PREROUTING", "-i", "br0", "-p", "udp", "--dport", "53", "-j", dnsPolicyIngressChain}
	if len(jump) != len(wantJump) {
		t.Fatalf("jump=%#v", jump)
	}
	for i := range wantJump {
		if jump[i] != wantJump[i] {
			t.Fatalf("jump[%d]=%q want=%q", i, jump[i], wantJump[i])
		}
	}
	redirect := dnsPolicyIngressRedirectArgs("tcp", 55355)
	wantRedirect := []string{"-t", "nat", dnsPolicyIngressChain, "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", "55355"}
	for i := range wantRedirect {
		if redirect[i] != wantRedirect[i] {
			t.Fatalf("redirect[%d]=%q want=%q", i, redirect[i], wantRedirect[i])
		}
	}
}
