package main

import "testing"

func TestParseDNSInfo(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
dns_tcp_port = 53
dns_udp_port = 53
dns_server = 127.0.0.1:40500 . # 9.9.9.9@dns.quad9.net
dns_server = 127.0.0.1:40508 ru # https://dns.google/dns-query@dnsm
static_a = router.test 192.0.2.1 0
static_aaaa = router.test 2001:db8::1 0
norebind_ctl = on
norebind_ip4net = 192.168.0.0/16
norebind_exclude = safe.example
proxy-stat:
Total incoming requests: 1000
Proxy requests sent: 750
Cache hits ratio: 0.250 (250)
Memory usage: 2.5M
Ip Port RSent ARcvd NXRcvd MedResp AvgResp Rank
127.0.0.1 40500 400 390 2 12ms 15ms 1
127.0.0.1 40508 350 340 4 20ms 24ms 2
proxy-tls:
server-tls:
  address: 9.9.9.9
  sni: dns.quad9.net
  interface: GigabitEthernet1
  domain:
proxy-https:
server-https:
  uri: https://dns.google/dns-query
  format: dnsm
  interface: GigabitEthernet1
  domain: ru
proxy-status:
  proxy-name: Policy1
proxy-config:
dns_tcp_port = 40541
dns_udp_port = 40541
dns_server = 1.1.1.1 example.org
proxy-stat:
Total incoming requests: 10
Proxy requests sent: 10
Cache hits ratio: 0 (0)
Memory usage: 128K
Ip Port RSent ARcvd NXRcvd MedResp AvgResp Rank
1.1.1.1 53 10 10 0 8ms 9ms 1
`

	info := parseDNSInfo(input)
	if len(info.Proxies) != 2 {
		t.Fatalf("got %d proxies, want 2: %#v", len(info.Proxies), info.Proxies)
	}
	system := info.Proxies[0]
	if system.Name != "System" || system.TCPPort != 53 || system.UDPPort != 53 {
		t.Fatalf("bad System proxy: %#v", system)
	}
	if system.Stat.TotalRequests != 1000 || system.Stat.ProxyRequestsSent != 750 || system.Stat.CacheHits != 250 {
		t.Fatalf("bad System stats: %#v", system.Stat)
	}
	if system.Stat.CacheHitRatio != 0.25 || system.Stat.Memory != "2.5M" {
		t.Fatalf("bad cache/memory: %#v", system.Stat)
	}
	if len(system.Upstreams) != 2 {
		t.Fatalf("got %d System upstreams, want 2: %#v", len(system.Upstreams), system.Upstreams)
	}
	if system.Upstreams[0].Protocol != "DoT" || system.Upstreams[0].Address != "9.9.9.9" || system.Upstreams[0].Interface != "GigabitEthernet1" {
		t.Fatalf("bad DoT upstream: %#v", system.Upstreams[0])
	}
	if system.Upstreams[0].RSent != 400 || system.Upstreams[0].ARcvd != 390 || system.Upstreams[0].Rank != 1 {
		t.Fatalf("bad DoT stats: %#v", system.Upstreams[0])
	}
	if system.Upstreams[1].Protocol != "DoH" || system.Upstreams[1].Domain != "ru" || system.Upstreams[1].Port != 443 {
		t.Fatalf("bad DoH upstream: %#v", system.Upstreams[1])
	}
	if len(info.StaticRecords) != 2 || info.StaticRecords[0].Host != "router.test" {
		t.Fatalf("bad static records: %#v", info.StaticRecords)
	}
	if !info.Rebind.Enabled || len(info.Rebind.Nets) != 1 || len(info.Rebind.Excludes) != 1 {
		t.Fatalf("bad rebind: %#v", info.Rebind)
	}

	policy := info.Proxies[1]
	if len(policy.Upstreams) != 1 || policy.Upstreams[0].Protocol != "DNS" || policy.Upstreams[0].Address != "1.1.1.1" {
		t.Fatalf("bad plain policy upstream: %#v", policy.Upstreams)
	}
	if policy.Upstreams[0].Domain != "example.org" || policy.Upstreams[0].RSent != 10 {
		t.Fatalf("bad plain policy scope/stats: %#v", policy.Upstreams[0])
	}
}

func TestParseDNSInfoDeduplicatesSharedState(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
static_a = same.test 192.0.2.10 0
norebind_ip4net = 10.0.0.0/8
proxy-status:
  proxy-name: Policy1
proxy-config:
static_a = same.test 192.0.2.10 0
norebind_ip4net = 10.0.0.0/8
`
	info := parseDNSInfo(input)
	if len(info.StaticRecords) != 1 {
		t.Fatalf("static records not deduplicated: %#v", info.StaticRecords)
	}
	if len(info.Rebind.Nets) != 1 {
		t.Fatalf("rebind nets not deduplicated: %#v", info.Rebind.Nets)
	}
}

func TestParseDNSInfoServerDoesNotInventDoTSNI(t *testing.T) {
	upstream, ok := parseDNSInfoServer("dns_server = 127.0.0.1:40500 . # 8.8.8.8")
	if !ok {
		t.Fatal("DoT upstream was not parsed")
	}
	if upstream.SNI != "" {
		t.Fatalf("SNI = %q, want empty for native DoT without SNI", upstream.SNI)
	}
	if upstream.Address != "8.8.8.8" || upstream.Port != 853 {
		t.Fatalf("unexpected DoT endpoint: %#v", upstream)
	}
}

func TestParseDNSInfoAcceptsFQDNMetadataAlias(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
dns_server = 127.0.0.1:40500 . # 8.8.8.8
proxy-tls:
server-tls:
  address: 8.8.8.8
  fqdn: dns.google
  domain:
`
	info := parseDNSInfo(input)
	if len(info.Proxies) != 1 || len(info.Proxies[0].Upstreams) != 1 {
		t.Fatalf("unexpected proxies: %#v", info.Proxies)
	}
	if got := info.Proxies[0].Upstreams[0].SNI; got != "dns.google" {
		t.Fatalf("SNI = %q, want dns.google from fqdn metadata", got)
	}
}
