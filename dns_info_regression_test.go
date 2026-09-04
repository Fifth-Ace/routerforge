package main

import "testing"

func TestDNSInfoMetadataUsesDistinctTLSRowsAndDedupesStaticFlags(t *testing.T) {
	text := `proxy-name: System
proxy-config:
 dns_tcp_port = 53
 dns_udp_port = 53
 dns_server = 127.0.0.1:40500 . # 8.8.8.8:853@dns.google.com
 dns_server = 127.0.0.1:40501 . # 8.8.8.8:853@dns.google
 static_a = example.test 192.0.2.1 1
 static_a = example.test 192.0.2.1 0
proxy-tls:
 server-tls:
  address: 8.8.8.8
  port: 853
  sni: dns.google.com
 server-tls:
  address: 8.8.8.8
  port: 853
  sni: dns.google
`
	got := parseDNSInfo(text)
	if len(got.Proxies) != 1 || len(got.Proxies[0].Upstreams) != 2 {
		t.Fatalf("unexpected proxy shape: %#v", got.Proxies)
	}
	first := got.Proxies[0].Upstreams[0].SNI
	second := got.Proxies[0].Upstreams[1].SNI
	if first != "dns.google.com" || second != "dns.google" {
		t.Fatalf("SNI matching = %q / %q", first, second)
	}
	if len(got.StaticRecords) != 1 {
		t.Fatalf("static dedupe count = %d, want 1", len(got.StaticRecords))
	}
}
