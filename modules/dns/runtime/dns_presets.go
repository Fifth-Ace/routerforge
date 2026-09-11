package main

import "strings"

const dnsPublicPresetCatalogRevision = "2026-09-12"

type dnsPublicPresetFamily struct {
	Provider   string
	Variant    string
	DoTAddress string
	DoTSNI     string
	DoHURI     string
}

// dnsPublicPresetFamilies is intentionally curated rather than scraped at
// runtime. Every entry is a public, no-account encrypted DNS endpoint verified
// against the operator's published documentation for the catalog revision
// above. Keeping the catalog compiled-in makes package installation inert:
// presets exist only in RouterForge's control view until the user explicitly
// enables one.
var dnsPublicPresetFamilies = []dnsPublicPresetFamily{
	// Cloudflare 1.1.1.1
	{Provider: "Cloudflare", Variant: "Standard", DoTAddress: "1.1.1.1", DoTSNI: "one.one.one.one", DoHURI: "https://cloudflare-dns.com/dns-query"},
	{Provider: "Cloudflare", Variant: "Malware", DoTAddress: "1.1.1.2", DoTSNI: "security.cloudflare-dns.com", DoHURI: "https://security.cloudflare-dns.com/dns-query"},
	{Provider: "Cloudflare", Variant: "Family", DoTAddress: "1.1.1.3", DoTSNI: "family.cloudflare-dns.com", DoHURI: "https://family.cloudflare-dns.com/dns-query"},

	// Google Public DNS
	{Provider: "Google", Variant: "Standard", DoTAddress: "8.8.8.8", DoTSNI: "dns.google", DoHURI: "https://dns.google/dns-query"},

	// Quad9
	{Provider: "Quad9", Variant: "Secure", DoTAddress: "9.9.9.9", DoTSNI: "dns.quad9.net", DoHURI: "https://dns.quad9.net/dns-query"},
	{Provider: "Quad9", Variant: "Secure + ECS", DoTAddress: "9.9.9.11", DoTSNI: "dns11.quad9.net", DoHURI: "https://dns11.quad9.net/dns-query"},
	{Provider: "Quad9", Variant: "Unfiltered", DoTAddress: "9.9.9.10", DoTSNI: "dns10.quad9.net", DoHURI: "https://dns10.quad9.net/dns-query"},

	// AdGuard Public DNS
	{Provider: "AdGuard", Variant: "Default", DoTAddress: "94.140.14.14", DoTSNI: "dns.adguard-dns.com", DoHURI: "https://dns.adguard-dns.com/dns-query"},
	{Provider: "AdGuard", Variant: "Unfiltered", DoTAddress: "94.140.14.140", DoTSNI: "unfiltered.adguard-dns.com", DoHURI: "https://unfiltered.adguard-dns.com/dns-query"},
	{Provider: "AdGuard", Variant: "Family", DoTAddress: "94.140.14.15", DoTSNI: "family.adguard-dns.com", DoHURI: "https://family.adguard-dns.com/dns-query"},

	// Yandex DNS
	{Provider: "Yandex", Variant: "Basic", DoTAddress: "77.88.8.8", DoTSNI: "common.dot.dns.yandex.net", DoHURI: "https://common.dot.dns.yandex.net/dns-query"},
	{Provider: "Yandex", Variant: "Safe", DoTAddress: "77.88.8.88", DoTSNI: "safe.dot.dns.yandex.net", DoHURI: "https://safe.dot.dns.yandex.net/dns-query"},
	{Provider: "Yandex", Variant: "Family", DoTAddress: "77.88.8.7", DoTSNI: "family.dot.dns.yandex.net", DoHURI: "https://family.dot.dns.yandex.net/dns-query"},

	// CleanBrowsing free filters
	{Provider: "CleanBrowsing", Variant: "Security", DoTAddress: "185.228.168.9", DoTSNI: "security-filter-dns.cleanbrowsing.org", DoHURI: "https://doh.cleanbrowsing.org/doh/security-filter/"},
	{Provider: "CleanBrowsing", Variant: "Adult", DoTAddress: "185.228.168.10", DoTSNI: "adult-filter-dns.cleanbrowsing.org", DoHURI: "https://doh.cleanbrowsing.org/doh/adult-filter/"},
	{Provider: "CleanBrowsing", Variant: "Family", DoTAddress: "185.228.168.168", DoTSNI: "family-filter-dns.cleanbrowsing.org", DoHURI: "https://doh.cleanbrowsing.org/doh/family-filter/"},

	// Control D free native filters. Third-party blocklist variants are omitted
	// deliberately: they are much more volatile than the provider's native
	// public service and would turn this stable catalog into a moving target.
	{Provider: "Control D", Variant: "Unfiltered", DoTAddress: "76.76.2.0", DoTSNI: "p0.freedns.controld.com", DoHURI: "https://freedns.controld.com/p0"},
	{Provider: "Control D", Variant: "Malware", DoTAddress: "76.76.2.1", DoTSNI: "p1.freedns.controld.com", DoHURI: "https://freedns.controld.com/p1"},
	{Provider: "Control D", Variant: "Ads & Tracking", DoTAddress: "76.76.2.2", DoTSNI: "p2.freedns.controld.com", DoHURI: "https://freedns.controld.com/p2"},
	{Provider: "Control D", Variant: "Social", DoTAddress: "76.76.2.3", DoTSNI: "p3.freedns.controld.com", DoHURI: "https://freedns.controld.com/p3"},
	{Provider: "Control D", Variant: "Family", DoTAddress: "76.76.2.4", DoTSNI: "family.freedns.controld.com", DoHURI: "https://freedns.controld.com/family"},
	{Provider: "Control D", Variant: "Uncensored", DoTAddress: "76.76.2.5", DoTSNI: "uncensored.freedns.controld.com", DoHURI: "https://freedns.controld.com/uncensored"},

	// DNS4EU Public Service
	{Provider: "DNS4EU", Variant: "Protective", DoTAddress: "86.54.11.1", DoTSNI: "protective.joindns4.eu", DoHURI: "https://protective.joindns4.eu/dns-query"},
	{Provider: "DNS4EU", Variant: "Protective + Child", DoTAddress: "86.54.11.12", DoTSNI: "child.joindns4.eu", DoHURI: "https://child.joindns4.eu/dns-query"},
	{Provider: "DNS4EU", Variant: "Protective + Ads", DoTAddress: "86.54.11.13", DoTSNI: "noads.joindns4.eu", DoHURI: "https://noads.joindns4.eu/dns-query"},
	{Provider: "DNS4EU", Variant: "Protective + Child + Ads", DoTAddress: "86.54.11.11", DoTSNI: "child-noads.joindns4.eu", DoHURI: "https://child-noads.joindns4.eu/dns-query"},
	{Provider: "DNS4EU", Variant: "Unfiltered", DoTAddress: "86.54.11.100", DoTSNI: "unfiltered.joindns4.eu", DoHURI: "https://unfiltered.joindns4.eu/dns-query"},

	// Comss.one DNS
	{Provider: "Comss.one", Variant: "Default", DoTAddress: "195.133.25.16", DoTSNI: "dns.comss.one", DoHURI: "https://dns.comss.one/dns-query"},
}

var dnsPublicPresets = mustBuildDNSPublicPresets()

func mustBuildDNSPublicPresets() []DNSResolverSpec {
	out := make([]DNSResolverSpec, 0, len(dnsPublicPresetFamilies)*2)
	seen := make(map[string]struct{}, len(dnsPublicPresetFamilies)*2)
	for _, family := range dnsPublicPresetFamilies {
		name := family.Provider + " — " + family.Variant
		if family.DoTAddress != "" {
			spec, err := normalizeDNSResolverSpec(DNSResolverSpec{
				Protocol: "DoT",
				Address:  family.DoTAddress,
				Port:     853,
				SNI:      family.DoTSNI,
			})
			if err != nil {
				panic("invalid built-in DoT preset " + name + ": " + err.Error())
			}
			spec.ID = dnsResolverID(spec)
			spec.Name = name
			spec.Disabled = true
			spec.Source = "preset"
			spec.Preset = true
			spec.Provider = family.Provider
			spec.Variant = family.Variant
			if _, exists := seen[spec.ID]; exists {
				panic("duplicate built-in DNS preset ID: " + spec.ID)
			}
			seen[spec.ID] = struct{}{}
			out = append(out, spec)
		}
		if family.DoHURI != "" {
			spec, err := normalizeDNSResolverSpec(DNSResolverSpec{
				Protocol: "DoH",
				URI:      family.DoHURI,
				Format:   "dnsm",
			})
			if err != nil {
				panic("invalid built-in DoH preset " + name + ": " + err.Error())
			}
			spec.ID = dnsResolverID(spec)
			spec.Name = name
			spec.Disabled = true
			spec.Source = "preset"
			spec.Preset = true
			spec.Provider = family.Provider
			spec.Variant = family.Variant
			if _, exists := seen[spec.ID]; exists {
				panic("duplicate built-in DNS preset ID: " + spec.ID)
			}
			seen[spec.ID] = struct{}{}
			out = append(out, spec)
		}
	}
	return out
}

func dnsPublicPresetByID(id string) (DNSResolverSpec, bool) {
	id = strings.TrimSpace(id)
	for _, preset := range dnsPublicPresets {
		if preset.ID == id {
			return preset, true
		}
	}
	return DNSResolverSpec{}, false
}

func applyDNSPublicPresetMetadata(spec DNSResolverSpec) DNSResolverSpec {
	preset, ok := dnsPublicPresetByID(spec.ID)
	if !ok {
		return spec
	}
	spec.Preset = true
	spec.Provider = preset.Provider
	spec.Variant = preset.Variant
	spec.Name = preset.Name
	spec.Source = "preset"
	return spec
}

func appendDNSPublicPresetCatalog(out *DNSResolverList, seen map[string]struct{}) {
	if out == nil {
		return
	}
	out.PresetCount = len(dnsPublicPresets)
	out.PresetCatalogRevision = dnsPublicPresetCatalogRevision
	for _, preset := range dnsPublicPresets {
		if _, exists := seen[preset.ID]; exists {
			continue
		}
		out.Resolvers = append(out.Resolvers, preset)
		// Virtual presets are visibly disabled but are not persisted disabled
		// resolvers. Keep DisabledCount backward-compatible with the actual
		// disabled store and expose catalog availability separately.
		out.PresetAvailableCount++
	}
}
