package main

import "testing"

func TestParseDNSProxyProfilePort(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
dns_tcp_port = 53
dns_udp_port = 53
dns_server = 127.0.0.1:40500 . # 8.8.8.8@dns.google.com

proxy-status:
  proxy-name: Policy0
proxy-config:
dns_tcp_port = 41100
dns_udp_port = 41100
dns_server = 127.0.0.1:40516 . # 8.8.8.8@dns.google.com
`
	got := parseDNSProxy(input)
	if len(got) != 2 {
		t.Fatalf("got=%#v", got)
	}
	if got[0].Profile != "System" || got[0].ProfileDNSPort != 53 {
		t.Fatalf("system=%#v", got[0])
	}
	if got[1].Profile != "Policy0" || got[1].ProfileDNSPort != 41100 {
		t.Fatalf("policy=%#v", got[1])
	}
}
