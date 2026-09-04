package main

import "testing"

func TestLogicalResolversKeepDistinctGoogleSNIAndGroupYandexDomains(t *testing.T) {
	entries := []map[string]any{
		{"address": "8.8.8.8", "fqdn": "dns.google.com"},
		{"address": "8.8.8.8", "fqdn": "dns.google"},
		{"address": "common.dot.dns.yandex.net", "domain": "ru"},
		{"address": "common.dot.dns.yandex.net", "domain": "su"},
		{"address": "common.dot.dns.yandex.net", "domain": "xn--p1ai"},
	}
	logical := logicalResolversFromRaw("DoT", entries)
	if len(logical) != 3 {
		t.Fatalf("logical resolvers = %d, want 3", len(logical))
	}
	var googleA, googleB, yandex *dnsLogicalResolver
	for _, item := range logical {
		switch item.Spec.SNI {
		case "dns.google.com":
			googleA = item
		case "dns.google":
			googleB = item
		default:
			if item.Spec.Address == "common.dot.dns.yandex.net" {
				yandex = item
			}
		}
	}
	if googleA == nil || googleB == nil || googleA.Spec.ID == googleB.Spec.ID {
		t.Fatalf("Google SNI identities were not kept distinct: %#v %#v", googleA, googleB)
	}
	if yandex == nil || len(yandex.Spec.Domains) != 3 || yandex.Spec.PhysicalCount != 3 {
		t.Fatalf("Yandex domains were not grouped: %#v", yandex)
	}
}

func TestPreviewNativeMultiDomainExpandsToPhysicalEntries(t *testing.T) {
	preview, err := previewDNSResolver(DNSResolverSpec{
		Protocol: "DoT",
		Address:  "common.dot.dns.yandex.net",
		Domains:  []string{"ru", "su", "xn--p1ai"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.PhysicalCount != 3 {
		t.Fatalf("physical count = %d, want 3", preview.PhysicalCount)
	}
	for _, entry := range preview.PhysicalEntries {
		if entry["address"] != "common.dot.dns.yandex.net" {
			t.Fatalf("unexpected entry: %#v", entry)
		}
	}
}

func TestDynamicResolverIsReadOnlyShape(t *testing.T) {
	items := dynamicResolvers([]dnsActiveNameServer{{
		Address: "192.168.1.1", Service: "Dhcp::Client-GigabitEthernet1", Interface: "GigabitEthernet1",
	}}, map[string]*dnsLogicalResolver{})
	if len(items) != 1 || !items[0].Dynamic || items[0].ReadOnlyReason == "" {
		t.Fatalf("dynamic resolver shape = %#v", items)
	}
}
