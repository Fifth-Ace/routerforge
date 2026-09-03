package main

import "testing"

func TestParseIPNameServers(t *testing.T) {
	input := `
server:
  address: 192.168.1.1
  domain:
  global: 64510
  service: Dhcp::Client-GigabitEthernet1
  interface: GigabitEthernet1

server:
  address: 8.8.4.4
  domain: example.com
  global: yes

server:
  address: 8.8.4.4
  domain: example.net
  global: yes
`

	got := parseIPNameServers(input)
	if len(got) != 2 {
		t.Fatalf("got %d name servers: %#v", len(got), got)
	}

	var gateway *PlainDNSMeta
	var google *PlainDNSMeta
	for i := range got {
		switch got[i].Address {
		case "192.168.1.1":
			gateway = &got[i]
		case "8.8.4.4":
			google = &got[i]
		}
	}

	if gateway == nil || gateway.Port != 53 {
		t.Fatalf("DHCP name server not parsed: %#v", got)
	}
	if google == nil {
		t.Fatalf("8.8.4.4 not parsed: %#v", got)
	}
	if len(google.Domains) != 2 ||
		google.Domains[0] != "example.com" ||
		google.Domains[1] != "example.net" {
		t.Fatalf("domains=%#v", google.Domains)
	}
}