package main

import (
	"errors"
	"fmt"
	"testing"
)

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

func TestDNSRCIPayloadUsesFQDNForDoTSNI(t *testing.T) {
	raw := map[string]any{
		"address": "1.0.0.1",
		"sni":     "cloudflare-dns.com",
		"domain":  "routerforge-test.invalid",
	}
	payload := dnsRCIPayload("DoT", raw)
	if payload["fqdn"] != "cloudflare-dns.com" {
		t.Fatalf("fqdn = %#v, want cloudflare-dns.com; payload=%#v", payload["fqdn"], payload)
	}
	if _, exists := payload["sni"]; exists {
		t.Fatalf("Keenetic RCI payload must not emit sni: %#v", payload)
	}

	want := canonicalProtocolEntries("DoT", []map[string]any{raw})
	got := canonicalProtocolEntries("DoT", []map[string]any{payload})
	if len(want) != 1 || len(got) != 1 || want[0] != got[0] {
		t.Fatalf("sni/fqdn canonical readback mismatch: want=%v got=%v", want, got)
	}
}

func TestDNSPhysicalLimitAcceptsEightDoTSlots(t *testing.T) {
	entries := make([]map[string]any, dnsKeeneticDoTSlotLimit)
	for i := range entries {
		entries[i] = map[string]any{"address": "1.0.0.1", "domain": fmt.Sprintf("slot-%d.invalid", i)}
	}
	state := &dnsConfigState{Logical: map[string]*dnsLogicalResolver{
		"test": {Spec: DNSResolverSpec{Protocol: "DoT"}, RawEntries: entries},
	}}
	if err := validateDNSPhysicalLimits(state, map[string]bool{"DoT": true}); err != nil {
		t.Fatalf("8 DoT slots must be accepted: %v", err)
	}
}

func TestDNSPhysicalLimitRejectsNineDoTSlotsBeforeWrite(t *testing.T) {
	entries := make([]map[string]any, dnsKeeneticDoTSlotLimit+1)
	for i := range entries {
		entries[i] = map[string]any{"address": "1.0.0.1", "domain": fmt.Sprintf("slot-%d.invalid", i)}
	}
	state := &dnsConfigState{Logical: map[string]*dnsLogicalResolver{
		"test": {Spec: DNSResolverSpec{Protocol: "DoT"}, RawEntries: entries},
	}}
	err := validateDNSPhysicalLimits(state, map[string]bool{"DoT": true})
	if err == nil {
		t.Fatal("9 DoT slots must be rejected")
	}
	if !errors.Is(err, errDNSResolverConflict) {
		t.Fatalf("slot overflow must be a conflict: %v", err)
	}
}
