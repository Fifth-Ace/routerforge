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

func TestDNSPhysicalLimitAcceptsEightMixedSecureSlots(t *testing.T) {
	dot := make([]map[string]any, 5)
	for i := range dot {
		dot[i] = map[string]any{"address": "1.0.0.1", "domain": fmt.Sprintf("dot-%d.invalid", i)}
	}
	doh := make([]map[string]any, 3)
	for i := range doh {
		doh[i] = map[string]any{"uri": "https://cloudflare-dns.com/dns-query", "domain": fmt.Sprintf("doh-%d.invalid", i)}
	}
	state := &dnsConfigState{Logical: map[string]*dnsLogicalResolver{
		"dot": {Spec: DNSResolverSpec{Protocol: "DoT"}, RawEntries: dot},
		"doh": {Spec: DNSResolverSpec{Protocol: "DoH"}, RawEntries: doh},
	}}
	if err := validateDNSPhysicalLimits(state, map[string]bool{"DoH": true}); err != nil {
		t.Fatalf("8 combined DoT/DoH slots must be accepted: %v", err)
	}
}

func TestDNSPhysicalLimitRejectsNinthMixedSecureSlotBeforeWrite(t *testing.T) {
	dot := make([]map[string]any, 6)
	for i := range dot {
		dot[i] = map[string]any{"address": "1.0.0.1", "domain": fmt.Sprintf("dot-%d.invalid", i)}
	}
	doh := make([]map[string]any, 3)
	for i := range doh {
		doh[i] = map[string]any{"uri": "https://cloudflare-dns.com/dns-query", "domain": fmt.Sprintf("doh-%d.invalid", i)}
	}
	state := &dnsConfigState{Logical: map[string]*dnsLogicalResolver{
		"dot": {Spec: DNSResolverSpec{Protocol: "DoT"}, RawEntries: dot},
		"doh": {Spec: DNSResolverSpec{Protocol: "DoH"}, RawEntries: doh},
	}}
	err := validateDNSPhysicalLimits(state, map[string]bool{"DoH": true})
	if err == nil {
		t.Fatal("9 combined DoT/DoH slots must be rejected")
	}
	if !errors.Is(err, errDNSResolverConflict) {
		t.Fatalf("secure slot overflow must be a conflict: %v", err)
	}
}

func TestPreviewDoHMultiDomainExpandsToPhysicalEntries(t *testing.T) {
	preview, err := previewDNSResolver(DNSResolverSpec{
		Protocol: "DoH",
		URI:      "https://cloudflare-dns.com/dns-query",
		Format:   "dnsm",
		Domains:  []string{"a.invalid", "b.invalid", "c.invalid"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.PhysicalCount != 3 {
		t.Fatalf("DoH physical count = %d, want 3", preview.PhysicalCount)
	}
	for _, entry := range preview.PhysicalEntries {
		if entry["uri"] != "https://cloudflare-dns.com/dns-query" || entry["format"] != "dnsm" {
			t.Fatalf("unexpected DoH entry: %#v", entry)
		}
	}
}

func TestPlainDNSAcceptsSixteenDomainsAndRejectsSeventeen(t *testing.T) {
	domains := make([]string, dnsKeeneticPlainDomainLimit)
	for i := range domains {
		domains[i] = fmt.Sprintf("zone-%d.invalid", i)
	}
	spec, err := normalizeDNSResolverSpec(DNSResolverSpec{
		Protocol: "DNS",
		Address:  "1.1.1.1",
		Domains:  domains,
	})
	if err != nil {
		t.Fatalf("16 plain DNS domains must be accepted: %v", err)
	}
	if spec.PhysicalCount != dnsKeeneticPlainDomainLimit {
		t.Fatalf("physical count = %d, want %d", spec.PhysicalCount, dnsKeeneticPlainDomainLimit)
	}
	_, err = normalizeDNSResolverSpec(DNSResolverSpec{
		Protocol: "DNS",
		Address:  "1.1.1.1",
		Domains:  append(domains, "overflow.invalid"),
	})
	if err == nil || !errors.Is(err, errDNSResolverInvalid) {
		t.Fatalf("17 plain DNS domains must be rejected, got %v", err)
	}
}

func TestDoHPortComesFromURLAndFormatIsValidated(t *testing.T) {
	spec, err := normalizeDNSResolverSpec(DNSResolverSpec{
		Protocol: "DoH",
		URI:      "https://dns.example:8443/dns-query",
		Port:     1234,
		Format:   "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Port != 8443 {
		t.Fatalf("DoH port = %d, want URL port 8443", spec.Port)
	}
	_, err = normalizeDNSResolverSpec(DNSResolverSpec{
		Protocol: "DoH",
		URI:      "https://dns.example/dns-query",
		Format:   "bogus",
	})
	if err == nil || !errors.Is(err, errDNSResolverInvalid) {
		t.Fatalf("invalid DoH format must be rejected, got %v", err)
	}
}
