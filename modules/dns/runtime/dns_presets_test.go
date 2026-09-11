package main

import "testing"

func TestDNSPublicPresetCatalogIsBaselineOnlyValidUniqueAndDisabled(t *testing.T) {
	wantFamilies := map[string]string{
		"Cloudflare": "Standard",
		"Google":     "Standard",
		"Quad9":      "Default",
		"AdGuard":    "Default",
		"Yandex":     "Basic",
		"Control D":  "Base",
		"DNS4EU":     "Base",
		"Comss.one":  "Default",
		"Xbox DNS":   "Base",
		"AstraCat":   "Base",
		"MALW":       "Base",
		"Mafioznik":  "Base",
	}
	if len(dnsPublicPresets) != len(wantFamilies)*2 {
		t.Fatalf("public preset count = %d, want %d", len(dnsPublicPresets), len(wantFamilies)*2)
	}
	seen := map[string]struct{}{}
	providers := map[string]map[string]int{}
	for _, preset := range dnsPublicPresets {
		if preset.ID == "" || preset.Provider == "" || preset.Variant == "" {
			t.Fatalf("incomplete public preset: %#v", preset)
		}
		if !preset.Preset || !preset.Disabled || preset.Source != "preset" {
			t.Fatalf("public preset must be inert by default: %#v", preset)
		}
		if preset.Protocol != "DoT" && preset.Protocol != "DoH" {
			t.Fatalf("unexpected public preset protocol: %#v", preset)
		}
		wantVariant, ok := wantFamilies[preset.Provider]
		if !ok {
			t.Fatalf("unexpected provider %q leaked into baseline catalog", preset.Provider)
		}
		if preset.Variant != wantVariant {
			t.Fatalf("provider %s variant = %q, want %q", preset.Provider, preset.Variant, wantVariant)
		}
		if _, exists := seen[preset.ID]; exists {
			t.Fatalf("duplicate public preset ID: %s", preset.ID)
		}
		seen[preset.ID] = struct{}{}
		if providers[preset.Provider] == nil {
			providers[preset.Provider] = map[string]int{}
		}
		providers[preset.Provider][preset.Protocol]++
		entries, err := buildDNSResolverEntries(preset)
		if err != nil {
			t.Fatalf("preset %s is invalid: %v", preset.Name, err)
		}
		if len(entries) != 1 {
			t.Fatalf("preset %s physical entries = %d, want 1", preset.Name, len(entries))
		}
	}
	for provider := range wantFamilies {
		if providers[provider]["DoT"] != 1 || providers[provider]["DoH"] != 1 {
			t.Fatalf("provider %s protocols = %#v, want exactly one DoT and one DoH", provider, providers[provider])
		}
	}
}

func TestDNSPublicPresetCatalogAppendDoesNotConsumeNativeSlots(t *testing.T) {
	out := DNSResolverList{}
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{})
	if len(out.Resolvers) != len(dnsPublicPresets) {
		t.Fatalf("visible presets = %d, want %d", len(out.Resolvers), len(dnsPublicPresets))
	}
	if out.GeneratedSlots != 0 || out.DoTPhysicalEntries != 0 || out.DoHPhysicalEntries != 0 || out.SecurePhysicalEntries != 0 {
		t.Fatalf("inert catalog consumed native slots: %#v", out)
	}
	if out.DisabledCount != 0 || out.PresetAvailableCount != len(dnsPublicPresets) {
		t.Fatalf("inert catalog counts are wrong: %#v", out)
	}
	if out.PresetCatalogRevision != dnsPublicPresetCatalogRevision {
		t.Fatalf("catalog revision = %q, want %q", out.PresetCatalogRevision, dnsPublicPresetCatalogRevision)
	}
}

func TestDNSPublicPresetCatalogSkipsConfiguredIDs(t *testing.T) {
	configured := dnsPublicPresets[0]
	out := DNSResolverList{}
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{configured.ID: {}})
	if len(out.Resolvers) != len(dnsPublicPresets)-1 {
		t.Fatalf("visible presets = %d, want %d", len(out.Resolvers), len(dnsPublicPresets)-1)
	}
	assertPresetIDAbsent(t, out.Resolvers, configured.ID)
}

func TestDNSPublicPresetCatalogSkipsEquivalentConfiguredDoTWithDifferentID(t *testing.T) {
	preset := mustFindDNSPublicPreset(t, "Cloudflare", "DoT")
	configured := preset
	configured.ID = "custom-dot-id"
	configured.Preset = false
	configured.Disabled = false
	configured.Source = "static"
	configured.Domains = []string{"example.com"}
	configured.Interface = "Wireguard0"

	out := DNSResolverList{Resolvers: []DNSResolverSpec{configured}}
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{configured.ID: {}})
	assertPresetIDAbsent(t, out.Resolvers, preset.ID)

	doh := mustFindDNSPublicPreset(t, "Cloudflare", "DoH")
	assertPresetIDPresent(t, out.Resolvers, doh.ID)
}

func TestDNSPublicPresetCatalogSkipsEquivalentConfiguredDoTBySNI(t *testing.T) {
	preset := mustFindDNSPublicPreset(t, "Cloudflare", "DoT")
	configured := DNSResolverSpec{
		ID:       "custom-hostname-dot",
		Protocol: "DoT",
		Address:  "one.one.one.one",
		Port:     853,
		Source:   "static",
	}
	out := DNSResolverList{Resolvers: []DNSResolverSpec{configured}}
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{configured.ID: {}})
	assertPresetIDAbsent(t, out.Resolvers, preset.ID)
}

func TestDNSPublicPresetCatalogSkipsEquivalentConfiguredDoHWithTrailingSlash(t *testing.T) {
	preset := mustFindDNSPublicPreset(t, "Xbox DNS", "DoH")
	configured := preset
	configured.ID = "custom-doh-id"
	configured.Preset = false
	configured.Disabled = false
	configured.Source = "static"
	configured.URI = preset.URI + "/"
	configured.Domains = []string{"example.com"}

	out := DNSResolverList{Resolvers: []DNSResolverSpec{configured}}
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{configured.ID: {}})
	assertPresetIDAbsent(t, out.Resolvers, preset.ID)

	dot := mustFindDNSPublicPreset(t, "Xbox DNS", "DoT")
	assertPresetIDPresent(t, out.Resolvers, dot.ID)
}

func TestDNSPublicPresetCatalogSkipsEquivalentPersistedDisabledResolver(t *testing.T) {
	preset := mustFindDNSPublicPreset(t, "MALW", "DoH")
	configured := preset
	configured.ID = "legacy-disabled-malw"
	configured.Preset = false
	configured.Disabled = true
	configured.Source = "disabled"

	out := DNSResolverList{Resolvers: []DNSResolverSpec{configured}, DisabledCount: 1}
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{configured.ID: {}})
	assertPresetIDAbsent(t, out.Resolvers, preset.ID)
	if out.DisabledCount != 1 {
		t.Fatalf("semantic preset de-dup changed disabled count: %d", out.DisabledCount)
	}
}

func TestDNSPublicPresetMetadataOverlay(t *testing.T) {
	preset := dnsPublicPresets[0]
	active := preset
	active.Name = "native readback name"
	active.Source = "static"
	active.Disabled = false
	active.Preset = false
	active.Provider = ""
	active.Variant = ""

	got := applyDNSPublicPresetMetadata(active)
	if !got.Preset || got.Provider != preset.Provider || got.Variant != preset.Variant || got.Name != preset.Name || got.Source != "preset" {
		t.Fatalf("preset metadata overlay mismatch: %#v", got)
	}
	if got.Disabled {
		t.Fatalf("metadata overlay changed active state: %#v", got)
	}
}

func mustFindDNSPublicPreset(t *testing.T, provider, protocol string) DNSResolverSpec {
	t.Helper()
	for _, preset := range dnsPublicPresets {
		if preset.Provider == provider && preset.Protocol == protocol {
			return preset
		}
	}
	t.Fatalf("preset %s/%s not found", provider, protocol)
	return DNSResolverSpec{}
}

func assertPresetIDAbsent(t *testing.T, resolvers []DNSResolverSpec, id string) {
	t.Helper()
	for _, resolver := range resolvers {
		if resolver.ID == id {
			t.Fatalf("preset %s was duplicated", id)
		}
	}
}

func assertPresetIDPresent(t *testing.T, resolvers []DNSResolverSpec, id string) {
	t.Helper()
	for _, resolver := range resolvers {
		if resolver.ID == id {
			return
		}
	}
	t.Fatalf("preset %s unexpectedly missing", id)
}
