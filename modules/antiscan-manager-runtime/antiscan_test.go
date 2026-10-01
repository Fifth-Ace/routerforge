package main

import (
	"strings"
	"testing"
)

const antiscanDefaultConfigFixture = `
ISP_INTERFACES="eth3"
PORTS="22,80,443"
PORTS_FORWARDED="22,80,443"
ENABLE_HONEYPOT="0"
HONEYPOT_PORTS=""
HONEYPOT_BANTIME="864000"
ENABLE_IPS_BAN="1"
RULES_MASK="255.255.255.255"
RECENT_CONNECTIONS_TIME="30"
RECENT_CONNECTIONS_HITCOUNT="15"
RECENT_CONNECTIONS_LIMIT="20"
RECENT_CONNECTIONS_BANTIME="864000"
DIFFERENT_IP_CANDIDATES_STORAGETIME="864000"
DIFFERENT_IP_THRESHOLD="5"
SUBNETS_BANTIME="864000"
IPSETS_DIRECTORY=""
SAVE_IPSETS="0"
SAVE_ON_EXIT="0"
USE_CUSTOM_EXCLUDE_LIST="0"
CUSTOM_LISTS_BLOCK_MODE="0"
GEOBLOCK_MODE="0"
GEOBLOCK_COUNTRIES=""
GEO_EXCLUDE_COUNTRIES=""
READ_NDM_LOCKOUT_IPSETS="0"
LOCKOUT_IPSET_BANTIME="864000"
`

func TestParseAntiscanConfigDefaults(t *testing.T) {
	cfg, err := parseAntiscanConfig(antiscanDefaultConfigFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ISPInterfaces) != 1 || cfg.ISPInterfaces[0] != "eth3" {
		t.Fatalf("interfaces=%v", cfg.ISPInterfaces)
	}
	if !cfg.EnableIPSBan || cfg.EnableHoneypot {
		t.Fatalf("unexpected protection toggles: %+v", cfg)
	}
	if cfg.DifferentIPThreshold != 5 || cfg.DifferentIPCandidateStorageTime != 864000 {
		t.Fatalf("unexpected distributed protection: %+v", cfg)
	}
	if cfg.RecentConnectionsTime != 30 || cfg.RecentConnectionsHitCount != 15 || cfg.RecentConnectionsLimit != 20 {
		t.Fatalf("unexpected direct protection: %+v", cfg)
	}
	if got := strings.Join(cfg.Ports, ","); got != "22,80,443" {
		t.Fatalf("ports=%q", got)
	}
}

func TestParseAntiscanConfigRejectsShellSyntax(t *testing.T) {
	_, err := parseAntiscanConfig("ISP_INTERFACES=\"eth3$(touch /tmp/nope)\"\n")
	if err == nil {
		t.Fatal("shell-style config value was accepted")
	}
}

func TestAntiscanConfigWarnsAboutMobilePoolFalsePositiveRisk(t *testing.T) {
	cfg, err := parseAntiscanConfig(antiscanDefaultConfigFixture)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(antiscanConfigWarnings(cfg), "\n")
	if !strings.Contains(warnings, "rotating/mobile address pools") {
		t.Fatalf("mobile pool warning missing: %q", warnings)
	}
}

func TestClassifyAntiscanMembershipDistributedSubnet(t *testing.T) {
	cfg, err := parseAntiscanConfig(antiscanDefaultConfigFixture)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := antiscanSnapshot{
		Detected:    true,
		Running:     true,
		Config:      cfg,
		Protection:  protectionFromConfig(cfg),
		MutationAPI: true,
	}
	existing := map[string]bool{"ascn_subnets": true}
	matches := map[string]bool{"ascn_subnets": true}

	got := classifyAntiscanMembership(snapshot, "100.64.10.77", existing, matches, false)
	if !got.Conclusive || !got.Blocked || got.Reason != "distributed-subnet" {
		t.Fatalf("unexpected verdict: %+v", got)
	}
	if len(got.Evidence) != 1 || got.Evidence[0].Set != "ascn_subnets" {
		t.Fatalf("unexpected evidence: %+v", got.Evidence)
	}
	if !strings.Contains(strings.Join(got.Warnings, "\n"), "Mobile carrier address pools") {
		t.Fatalf("mobile false-positive warning missing: %+v", got.Warnings)
	}
}

func TestClassifyAntiscanMembershipExclusionWins(t *testing.T) {
	snapshot := antiscanSnapshot{Detected: true, Running: true}
	existing := map[string]bool{
		"ascn_custom_exclude": true,
		"ascn_ips":            true,
	}
	matches := map[string]bool{
		"ascn_custom_exclude": true,
		"ascn_ips":            true,
	}

	got := classifyAntiscanMembership(snapshot, "203.0.113.10", existing, matches, false)
	if !got.Conclusive || got.Blocked || got.Verdict != "excluded" || got.Reason != "custom-exclude" {
		t.Fatalf("unexpected exclusion verdict: %+v", got)
	}
}

func TestClassifyAntiscanMembershipDirectIPDoesNotInventExactTrigger(t *testing.T) {
	snapshot := antiscanSnapshot{Detected: true, Running: true}
	existing := map[string]bool{"ascn_ips": true}
	matches := map[string]bool{"ascn_ips": true}

	got := classifyAntiscanMembership(snapshot, "198.51.100.25", existing, matches, false)
	if !got.Blocked || got.Reason != "direct-ip" {
		t.Fatalf("unexpected direct verdict: %+v", got)
	}
	if len(got.Evidence) != 1 || !strings.Contains(got.Evidence[0].Summary, "does not persist") {
		t.Fatalf("exact-trigger limitation missing: %+v", got.Evidence)
	}
}

func TestClassifyAntiscanMembershipCandidateIsNotBlocked(t *testing.T) {
	snapshot := antiscanSnapshot{Detected: true, Running: true}
	existing := map[string]bool{"ascn_candidates": true}
	matches := map[string]bool{"ascn_candidates": true}

	got := classifyAntiscanMembership(snapshot, "192.0.2.44", existing, matches, false)
	if !got.Conclusive || got.Blocked || got.Verdict != "candidate" || got.Reason != "candidate-only" {
		t.Fatalf("unexpected candidate verdict: %+v", got)
	}
}

func TestParseIPSetCount(t *testing.T) {
	got, err := parseIPSetCount("Name: ascn_ips\nType: hash:ip\nNumber of entries: 17\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != 17 {
		t.Fatalf("count=%d", got)
	}
}

func TestParseAntiscanVersions(t *testing.T) {
	if got := parsePackageVersion("Package: antiscan\nVersion: 1.10.6\n"); got != "1.10.6" {
		t.Fatalf("package version=%q", got)
	}
	if got := parseScriptVersion("ASCN_TEMP_FILE=\"/tmp/ascn.run\"\nASCN_VERSION=\"1.10.6\"\n"); got != "1.10.6" {
		t.Fatalf("script version=%q", got)
	}
}
