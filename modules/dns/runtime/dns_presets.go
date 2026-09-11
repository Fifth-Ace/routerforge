package main

import (
	"net/url"
	"strconv"
	"strings"
)

const dnsPublicPresetCatalogRevision = "2026-09-12-r2"

type dnsPublicPresetFamily struct {
	Provider   string
	Variant    string
	DoTAddress string
	DoTSNI     string
	DoHURI     string
}

// dnsPublicPresetFamilies is intentionally curated rather than scraped at
// runtime. R2 keeps exactly one baseline encrypted profile per provider: no
// Family/Malware/Adult/Ads/ECS/etc. variants. Every family expands to one DoT
// and one DoH preset and remains inert until explicitly enabled by the user.
var dnsPublicPresetFamilies = []dnsPublicPresetFamily{
	// Large public resolvers: one baseline profile per provider.
	{Provider: "Cloudflare", Variant: "Standard", DoTAddress: "1.1.1.1", DoTSNI: "one.one.one.one", DoHURI: "https://cloudflare-dns.com/dns-query"},
	{Provider: "Google", Variant: "Standard", DoTAddress: "8.8.8.8", DoTSNI: "dns.google", DoHURI: "https://dns.google/dns-query"},
	{Provider: "Quad9", Variant: "Default", DoTAddress: "9.9.9.9", DoTSNI: "dns.quad9.net", DoHURI: "https://dns.quad9.net/dns-query"},
	{Provider: "AdGuard", Variant: "Default", DoTAddress: "94.140.14.14", DoTSNI: "dns.adguard-dns.com", DoHURI: "https://dns.adguard-dns.com/dns-query"},
	{Provider: "Yandex", Variant: "Basic", DoTAddress: "77.88.8.8", DoTSNI: "common.dot.dns.yandex.net", DoHURI: "https://common.dot.dns.yandex.net/dns-query"},
	{Provider: "Control D", Variant: "Base", DoTAddress: "76.76.2.0", DoTSNI: "p0.freedns.controld.com", DoHURI: "https://freedns.controld.com/p0"},
	{Provider: "DNS4EU", Variant: "Base", DoTAddress: "86.54.11.100", DoTSNI: "unfiltered.joindns4.eu", DoHURI: "https://unfiltered.joindns4.eu/dns-query"},
	{Provider: "Comss.one", Variant: "Default", DoTAddress: "195.133.25.16", DoTSNI: "dns.comss.one", DoHURI: "https://dns.comss.one/dns-query"},

	// Community/private public resolvers. Prefer hostnames for DoT so provider
	// side address changes do not require a RouterForge catalog update.
	{Provider: "Xbox DNS", Variant: "Base", DoTAddress: "xbox-dns.ru", DoTSNI: "xbox-dns.ru", DoHURI: "https://xbox-dns.ru/dns-query"},
	{Provider: "AstraCat", Variant: "Base", DoTAddress: "dns.astracat.ru", DoTSNI: "dns.astracat.ru", DoHURI: "https://dns.astracat.ru/dns-query"},
	{Provider: "MALW", Variant: "Base", DoTAddress: "dns.malw.link", DoTSNI: "dns.malw.link", DoHURI: "https://dns.malw.link/dns-query"},
	{Provider: "Mafioznik", Variant: "Base", DoTAddress: "dns.mafioznik.xyz", DoTSNI: "dns.mafioznik.xyz", DoHURI: "https://dns.mafioznik.xyz/dns-query"},
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

// appendDNSPublicPresetCatalog adds only presets that are not already present
// in native/disabled RouterForge state. ID matching handles exact built-in
// round-trips; endpoint matching also suppresses a virtual preset when the user
// configured the same resolver before RouterForge learned about the catalog or
// when scope/interface metadata makes the resolver ID different.
func appendDNSPublicPresetCatalog(out *DNSResolverList, seen map[string]struct{}) {
	if out == nil {
		return
	}
	out.PresetCount = len(dnsPublicPresets)
	out.PresetCatalogRevision = dnsPublicPresetCatalogRevision
	configured := append([]DNSResolverSpec(nil), out.Resolvers...)
	for _, preset := range dnsPublicPresets {
		if _, exists := seen[preset.ID]; exists {
			continue
		}
		if dnsPublicPresetEndpointAlreadyConfigured(preset, configured) {
			continue
		}
		out.Resolvers = append(out.Resolvers, preset)
		// Virtual presets are visibly disabled but are not persisted disabled
		// resolvers. Keep DisabledCount backward-compatible with the actual
		// disabled store and expose catalog availability separately.
		out.PresetAvailableCount++
	}
}

func dnsPublicPresetEndpointAlreadyConfigured(preset DNSResolverSpec, configured []DNSResolverSpec) bool {
	for _, existing := range configured {
		if dnsPublicPresetEndpointEquivalent(preset, existing) {
			return true
		}
	}
	return false
}

func dnsPublicPresetEndpointEquivalent(a, b DNSResolverSpec) bool {
	if !strings.EqualFold(strings.TrimSpace(a.Protocol), strings.TrimSpace(b.Protocol)) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(a.Protocol)) {
	case "dot":
		return dnsPublicDoTEndpointEquivalent(a, b)
	case "doh":
		left := dnsPublicCanonicalDoHURI(a.URI)
		right := dnsPublicCanonicalDoHURI(b.URI)
		return left != "" && left == right
	default:
		return false
	}
}

func dnsPublicDoTEndpointEquivalent(a, b DNSResolverSpec) bool {
	leftPort := a.Port
	if leftPort == 0 {
		leftPort = 853
	}
	rightPort := b.Port
	if rightPort == 0 {
		rightPort = 853
	}
	if leftPort != rightPort {
		return false
	}
	leftAddress := dnsPublicCanonicalHost(a.Address)
	rightAddress := dnsPublicCanonicalHost(b.Address)
	leftSNI := dnsPublicCanonicalHost(a.SNI)
	rightSNI := dnsPublicCanonicalHost(b.SNI)
	return dnsPublicNonEmptyEqual(leftAddress, rightAddress) ||
		dnsPublicNonEmptyEqual(leftSNI, rightSNI) ||
		dnsPublicNonEmptyEqual(leftAddress, rightSNI) ||
		dnsPublicNonEmptyEqual(leftSNI, rightAddress)
}

func dnsPublicCanonicalHost(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func dnsPublicNonEmptyEqual(a, b string) bool {
	return a != "" && b != "" && a == b
}

func dnsPublicCanonicalDoHURI(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" {
		return ""
	}
	host := dnsPublicCanonicalHost(parsed.Hostname())
	port := parsed.Port()
	if port == "443" {
		port = ""
	}
	authority := host
	if port != "" {
		if _, err := strconv.Atoi(port); err != nil {
			return ""
		}
		authority += ":" + port
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	} else if path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	canonical := "https://" + authority + path
	if parsed.RawQuery != "" {
		canonical += "?" + parsed.RawQuery
	}
	return canonical
}
