package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeAntiscanFlushTargetPinnedContract(t *testing.T) {
	for _, target := range antiscanFlushTargets {
		got, err := normalizeAntiscanFlushTarget("  " + strings.ToUpper(target) + "  ")
		if err != nil {
			t.Fatalf("normalize %q: %v", target, err)
		}
		if got != target {
			t.Fatalf("normalize %q = %q", target, got)
		}
	}
	for _, raw := range []string{"", "everything", "ips;rm -rf /", "custom", "geo_exclude"} {
		if _, err := normalizeAntiscanFlushTarget(raw); err == nil {
			t.Fatalf("unsafe/unsupported target %q was accepted", raw)
		}
	}
}

func TestAntiscanFlushAllMirrorsUpstreamExactSetList(t *testing.T) {
	got := antiscanFlushSetNames("all")
	want := []string{
		"ascn_candidates",
		"ascn_ips",
		"ascn_subnets",
		"ascn_geo_whitelist",
		"ascn_geo_blacklist",
		"ascn_ndm_lockout",
		"ascn_honeypot",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("all sets = %#v, want %#v", got, want)
	}
	for _, forbidden := range []string{"ascn_geo_exclude", "ascn_custom_whitelist", "ascn_custom_blacklist", "ascn_custom_exclude"} {
		for _, name := range got {
			if name == forbidden {
				t.Fatalf("upstream all contract unexpectedly contains %s", forbidden)
			}
		}
	}
}

func TestAntiscanFlushWhitelistSafetyGuard(t *testing.T) {
	custom := antiscanConfig{CustomListsBlockMode: "whitelist", SaveIPSets: false}
	if got := antiscanFlushSafetyBlock(custom, "custom_whitelist"); !strings.Contains(got, "could lock out access") {
		t.Fatalf("custom whitelist guard missing: %q", got)
	}
	custom.SaveIPSets = true
	if got := antiscanFlushSafetyBlock(custom, "custom_whitelist"); got != "" {
		t.Fatalf("custom whitelist with SAVE_IPSETS=1 blocked: %q", got)
	}

	geo := antiscanConfig{GeoBlockMode: "whitelist", SaveIPSets: false}
	for _, target := range []string{"geo", "all"} {
		if got := antiscanFlushSafetyBlock(geo, target); !strings.Contains(got, "could lock out access") {
			t.Fatalf("geo whitelist guard missing for %s: %q", target, got)
		}
	}
	geo.SaveIPSets = true
	if got := antiscanFlushSafetyBlock(geo, "geo"); got != "" {
		t.Fatalf("geo whitelist with SAVE_IPSETS=1 blocked: %q", got)
	}
}

func TestBuildAntiscanFlushCommandNeverUsesShellText(t *testing.T) {
	args, stdin, err := buildAntiscanFlushCommand("ips")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"flush", "ips"}) || stdin != "Y\n" {
		t.Fatalf("ips command = %#v stdin=%q", args, stdin)
	}
	args, stdin, err = buildAntiscanFlushCommand("all")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"flush"}) || stdin != "Y\n" {
		t.Fatalf("all command = %#v stdin=%q", args, stdin)
	}
	if _, _, err := buildAntiscanFlushCommand("ips && touch /tmp/pwned"); err == nil {
		t.Fatal("shell-like target was accepted")
	}
}

func TestAntiscanFlushRestartRequirementsMatchUpstreamPersistenceBranch(t *testing.T) {
	cfg := antiscanConfig{SaveIPSets: true, CustomListsBlockMode: "blacklist"}
	if !antiscanFlushRestartRequired(cfg, "custom_blacklist") {
		t.Fatal("active custom blacklist should require recovery before restart")
	}
	if antiscanFlushRestartRequired(cfg, "custom_whitelist") {
		t.Fatal("inactive custom whitelist should not request restart")
	}
	cfg = antiscanConfig{SaveIPSets: true, GeoBlockMode: "whitelist"}
	if !antiscanFlushRestartRequired(cfg, "geo") || !antiscanFlushRestartRequired(cfg, "all") {
		t.Fatal("active Geo mode should report restart requirement")
	}
	cfg.SaveIPSets = false
	if antiscanFlushRestartRequired(cfg, "geo") {
		t.Fatal("SAVE_IPSETS=0 should not claim upstream destroy/restart branch")
	}
}

func TestParseAntiscanFlushSaveCount(t *testing.T) {
	fixture := `create ascn_candidates hash:ip family inet hashsize 1024 maxelem 65536 timeout 864000
add ascn_candidates 198.51.100.7 timeout 60
add ascn_candidates 203.0.113.9 timeout 120
`
	count, err := parseAntiscanFlushSaveCount(strings.NewReader(fixture), "ascn_candidates")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d, want 2", count)
	}

	empty := `create ascn_candidates hash:ip family inet hashsize 1024 maxelem 65536 timeout 864000
`
	count, err = parseAntiscanFlushSaveCount(strings.NewReader(empty), "ascn_candidates")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("empty count=%d, want 0", count)
	}

	if _, err := parseAntiscanFlushSaveCount(strings.NewReader("add ascn_candidates 192.0.2.1\n"), "ascn_candidates"); err == nil {
		t.Fatal("save output without create header was accepted")
	}
}

func TestVerifyAntiscanFlushFileEffects(t *testing.T) {
	dir := t.TempDir()
	truncated := filepath.Join(dir, "custom.txt")
	removed := filepath.Join(dir, "persisted.txt")
	if err := os.WriteFile(truncated, nil, 0600); err != nil {
		t.Fatal(err)
	}
	effects := []antiscanFlushFileEffect{
		{Path: truncated, Kind: "truncate", Existed: true, Size: 12},
		{Path: removed, Kind: "remove", Existed: true, Size: 12},
	}
	if err := verifyAntiscanFlushFileEffects(effects); err != nil {
		t.Fatalf("expected verified effects: %v", err)
	}
	if err := os.WriteFile(removed, []byte("still here"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyAntiscanFlushFileEffects(effects); err == nil {
		t.Fatal("remaining persisted file was not detected")
	}
}
