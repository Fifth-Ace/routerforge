package main

import "testing"

func TestDNSPublicPresetCatalogIsValidUniqueAndDisabled(t *testing.T) {
	if len(dnsPublicPresets) != 56 {
		t.Fatalf("public preset count = %d, want 56", len(dnsPublicPresets))
	}
	seen := map[string]struct{}{}
	providers := map[string]int{}
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
		if _, exists := seen[preset.ID]; exists {
			t.Fatalf("duplicate public preset ID: %s", preset.ID)
		}
		seen[preset.ID] = struct{}{}
		providers[preset.Provider]++
		entries, err := buildDNSResolverEntries(preset)
		if err != nil {
			t.Fatalf("preset %s is invalid: %v", preset.Name, err)
		}
		if len(entries) != 1 {
			t.Fatalf("preset %s physical entries = %d, want 1", preset.Name, len(entries))
		}
	}
	for _, provider := range []string{
		"Cloudflare", "Google", "Quad9", "AdGuard", "Yandex", "CleanBrowsing", "Control D", "DNS4EU", "Comss.one",
	} {
		if providers[provider] == 0 {
			t.Fatalf("provider %s missing from public preset catalog", provider)
		}
	}
	if providers["Mullvad"] != 0 || providers["DNS0.EU"] != 0 {
		t.Fatalf("retired/retiring provider leaked into catalog: %#v", providers)
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
	appendDNSPublicPresetCatalog(&out, map[string]struct{}{configured.ID: struct{}{}})
	if len(out.Resolvers) != len(dnsPublicPresets)-1 {
		t.Fatalf("visible presets = %d, want %d", len(out.Resolvers), len(dnsPublicPresets)-1)
	}
	for _, preset := range out.Resolvers {
		if preset.ID == configured.ID {
			t.Fatalf("configured preset %s was duplicated", configured.ID)
		}
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
