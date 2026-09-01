package main

import "testing"

func TestParseDNSProxy(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
proceed = 500
timeout = 7000
dns_server = 127.0.0.1:40500 . # 9.9.9.9@dns.quad9.net
dns_server = 127.0.0.1:40508 . # https://dns.google/dns-query@dnsm
proxy-status:
  proxy-name: Policy1
proxy-config:
proceed = 500
dns_server = 127.0.0.1:40541 ntc.party # https://dns.astracat.ru/dns-query@dnsm
`
	got := parseDNSProxy(input)
	if len(got) != 3 {
		t.Fatalf("got %d upstreams, want 3: %#v", len(got), got)
	}
	if got[0].Port != 40500 || got[0].Protocol != "DoT" || got[0].Profile != "System" {
		t.Fatalf("bad DoT: %#v", got[0])
	}
	if got[1].Port != 40508 || got[1].Protocol != "DoH" || got[1].Name != "Google DoH" {
		t.Fatalf("bad DoH: %#v", got[1])
	}
	if got[2].Domain != "ntc.party" || got[2].Profile != "Policy1" {
		t.Fatalf("bad policy: %#v", got[2])
	}
}

func TestParseDNSProxyInterfaces(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
proceed = 500
timeout = 7000
dns_server = 127.0.0.1:40500 . # 9.9.9.9@dns.quad9.net
dns_server = 127.0.0.1:40504 . # 77.88.8.8:853@common.dot.dns.yandex.net
dns_server = 127.0.0.1:40508 . # https://dns.google/dns-query@dnsm
proxy-tls:
server-tls:
  address: 9.9.9.9
  sni: dns.quad9.net
  interface: GigabitEthernet1
  domain:
server-tls:
  address: 77.88.8.8
  port: 853
  sni: common.dot.dns.yandex.net
  interface: WifiMaster1/AccessPoint1
  domain:
proxy-https:
server-https:
  uri: https://dns.google/dns-query
  format: dnsm
  interface: GigabitEthernet1
  domain:
`
	got := parseDNSProxy(input)
	if len(got) != 3 {
		t.Fatalf("got %d upstreams: %#v", len(got), got)
	}
	byPort := map[uint16]UpstreamMeta{}
	for _, u := range got {
		byPort[u.Port] = u
	}
	if byPort[40500].Interface != "GigabitEthernet1" {
		t.Fatalf("quad9 interface=%q", byPort[40500].Interface)
	}
	if byPort[40504].Interface != "WifiMaster1/AccessPoint1" {
		t.Fatalf("yandex interface=%q", byPort[40504].Interface)
	}
	if byPort[40508].Interface != "GigabitEthernet1" {
		t.Fatalf("DoH interface=%q", byPort[40508].Interface)
	}
}
