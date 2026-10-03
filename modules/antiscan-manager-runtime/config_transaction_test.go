package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeAntiscanConfigUpdate(t *testing.T) {
	req := validAntiscanConfigUpdate()
	req.ISPInterfaces = "eth3  ppp0"
	req.Ports = "22,443,1000:1010"
	req.GeoBlockCountries = "fi de FI"
	values, err := normalizeAntiscanConfigUpdate(req)
	if err != nil {
		t.Fatal(err)
	}
	if values["ISP_INTERFACES"] != "eth3 ppp0" {
		t.Fatalf("interfaces=%q", values["ISP_INTERFACES"])
	}
	if values["PORTS"] != "22,443,1000:1010" {
		t.Fatalf("ports=%q", values["PORTS"])
	}
	if values["GEOBLOCK_COUNTRIES"] != "FI DE" {
		t.Fatalf("countries=%q", values["GEOBLOCK_COUNTRIES"])
	}
}

func TestNormalizeAntiscanConfigUpdateRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*antiscanConfigUpdateRequest)
	}{
		{"interface", func(r *antiscanConfigUpdateRequest) { r.ISPInterfaces = "eth3;reboot" }},
		{"port", func(r *antiscanConfigUpdateRequest) { r.Ports = "70000" }},
		{"range", func(r *antiscanConfigUpdateRequest) { r.Ports = "9000:8000" }},
		{"threshold", func(r *antiscanConfigUpdateRequest) { r.DifferentIPThreshold = 1 }},
		{"geo-mode", func(r *antiscanConfigUpdateRequest) { r.GeoBlockMode = "drop-everything" }},
		{"mask", func(r *antiscanConfigUpdateRequest) { r.RulesMask = "999.1.1.1" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validAntiscanConfigUpdate()
			tc.edit(&req)
			if _, err := normalizeAntiscanConfigUpdate(req); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}

func TestValidateAntiscanIPSetsDirectoryMatchesUpstreamPathContract(t *testing.T) {
	optional := map[string]string{"SAVE_IPSETS": "0", "GEOBLOCK_MODE": "0", "GEO_EXCLUDE_COUNTRIES": ""}
	for _, path := range []string{
		"",
		"/opt",
		"/tmp",
		"/opt/antiscan",
		"/tmp/antiscan",
		"/opt/etc/antiscan",
		"/opt/etc/antiscan/ipsets",
		"/opt/routerforge-parity-path-does-not-need-to-exist",
	} {
		if err := validateAntiscanIPSetsDirectory(path, optional); err != nil {
			t.Fatalf("upstream-valid path %q rejected: %v", path, err)
		}
	}

	required := map[string]string{"SAVE_IPSETS": "1", "GEOBLOCK_MODE": "0", "GEO_EXCLUDE_COUNTRIES": ""}
	if err := validateAntiscanIPSetsDirectory("", required); err == nil {
		t.Fatal("empty required IPSETS_DIRECTORY accepted")
	}

	for _, path := range []string{
		"/opt/etc",
		"/opt/etc/",
		"/etc/antiscan",
		"/var/lib/antiscan",
		"relative/path",
		"/opt/../tmp/antiscan",
		"/opt/./antiscan",
		"/opt//antiscan",
	} {
		if err := validateAntiscanIPSetsDirectory(path, optional); err == nil {
			t.Fatalf("upstream-invalid path %q accepted", path)
		}
	}
}

func TestMergeAntiscanConfigTextPreservesComments(t *testing.T) {
	req := validAntiscanConfigUpdate()
	values, err := normalizeAntiscanConfigUpdate(req)
	if err != nil {
		t.Fatal(err)
	}
	original := "# keep me\nISP_INTERFACES=\"eth9\"\n\nPORTS=\"80\"\n"
	merged, err := mergeAntiscanConfigText(original, values)
	if err != nil {
		t.Fatal(err)
	}
	text := string(merged)
	if !strings.Contains(text, "# keep me") || !strings.Contains(text, `ISP_INTERFACES="eth3"`) {
		t.Fatalf("merged config=%s", text)
	}
	if !strings.Contains(text, "# Added by RouterForge Antiscan Manager") {
		t.Fatalf("missing appended-key marker: %s", text)
	}
	if err := verifyAntiscanConfigValues(merged, values); err != nil {
		t.Fatal(err)
	}
}

func TestApplyAntiscanConfigStoppedStoresVerifiedConfig(t *testing.T) {
	cfg, original := fakeAntiscanConfigEnvironment(t, false)
	req := validAntiscanConfigUpdate()
	req.BaseSHA256 = sha256Hex(original)
	req.ISPInterfaces = "ppp0"

	result, status, err := applyAntiscanConfig(context.Background(), cfg, req)
	if err != nil || status != http.StatusOK || !result.Changed || !result.Verified || result.RuntimeApplied || result.RollbackPerformed {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	if result.BackupPath == "" {
		t.Fatal("backup path missing")
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || string(backup) != string(original) {
		t.Fatalf("backup err=%v data=%q", err, backup)
	}
	parsed, err := readAntiscanConfig(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if err != nil || strings.Join(parsed.ISPInterfaces, " ") != "ppp0" {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
}

func TestApplyAntiscanConfigRunningReloadsAndVerifies(t *testing.T) {
	cfg, original := fakeAntiscanConfigEnvironment(t, true)
	req := validAntiscanConfigUpdate()
	req.BaseSHA256 = sha256Hex(original)
	req.DifferentIPThreshold = 6

	result, status, err := applyAntiscanConfig(context.Background(), cfg, req)
	if err != nil || status != http.StatusOK || !result.Changed || !result.Verified || !result.RuntimeApplied || result.RollbackPerformed {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	logData, err := os.ReadFile(filepath.Join(cfg.AntiscanDir, "reload.log"))
	if err != nil || strings.Count(string(logData), "reload\n") != 1 {
		t.Fatalf("reload log=%q err=%v", logData, err)
	}
}

func TestApplyAntiscanConfigRollbackOnReloadFailure(t *testing.T) {
	cfg, original := fakeAntiscanConfigEnvironment(t, true)
	req := validAntiscanConfigUpdate()
	req.BaseSHA256 = sha256Hex(original)
	req.DifferentIPThreshold = 3

	result, status, err := applyAntiscanConfig(context.Background(), cfg, req)
	if err == nil || status != http.StatusConflict || !result.Changed || !result.RollbackPerformed || result.Verified {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	current, readErr := os.ReadFile(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if readErr != nil || string(current) != string(original) {
		t.Fatalf("rollback file mismatch err=%v\ncurrent=%s\noriginal=%s", readErr, current, original)
	}
	logData, logErr := os.ReadFile(filepath.Join(cfg.AntiscanDir, "reload.log"))
	if logErr != nil || strings.Count(string(logData), "reload\n") != 2 {
		t.Fatalf("reload log=%q err=%v", logData, logErr)
	}
}

func TestApplyAntiscanConfigRejectsStaleHash(t *testing.T) {
	cfg, original := fakeAntiscanConfigEnvironment(t, false)
	req := validAntiscanConfigUpdate()
	req.BaseSHA256 = strings.Repeat("0", 64)
	result, status, err := applyAntiscanConfig(context.Background(), cfg, req)
	if err == nil || status != http.StatusConflict || result.Changed {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	current, _ := os.ReadFile(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if string(current) != string(original) {
		t.Fatal("stale-hash request changed config")
	}
}

func validAntiscanConfigUpdate() antiscanConfigUpdateRequest {
	return antiscanConfigUpdateRequest{
		Confirm:                         "APPLY_CONFIG",
		ISPInterfaces:                   "eth3",
		Ports:                           "22,80,443",
		ForwardedPorts:                  "22,80,443",
		EnableHoneypot:                  false,
		HoneypotPorts:                   "",
		HoneypotBanTime:                 864000,
		EnableIPSBan:                    true,
		RulesMask:                       "255.255.255.255",
		RecentConnectionsTime:           30,
		RecentConnectionsHitCount:       15,
		RecentConnectionsLimit:          20,
		RecentConnectionsBanTime:        864000,
		DifferentIPCandidateStorageTime: 864000,
		DifferentIPThreshold:            5,
		SubnetsBanTime:                  864000,
		IPSetsDirectory:                 "",
		SaveIPSets:                      false,
		SaveOnExit:                      false,
		UseCustomExcludeList:            false,
		CustomListsBlockMode:            "0",
		GeoBlockMode:                    "0",
		GeoBlockCountries:               "",
		GeoExcludeCountries:             "",
		ReadNDMLockoutIPSets:            false,
		LockoutIPSetBanTime:             864000,
	}
}

func fakeAntiscanConfigEnvironment(t *testing.T, running bool) (runtimeConfig, []byte) {
	t.Helper()
	dir := t.TempDir()
	cfg := runtimeConfig{
		AntiscanDir:    dir,
		InitScript:     filepath.Join(dir, "S99ascn"),
		StatusFile:     filepath.Join(dir, "ascn.run"),
		ConfigLockFile: filepath.Join(dir, "ascn.lock"),
		GeoLockFile:    filepath.Join(dir, "ascn_geo.lock"),
	}
	values, err := normalizeAntiscanConfigUpdate(validAntiscanConfigUpdate())
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{"# original fixture"}
	for _, key := range antiscanConfigKeys {
		lines = append(lines, fmt.Sprintf(`%s="%s"`, key, values[key]))
	}
	original := []byte(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(dir, "ascn.conf"), original, 0644); err != nil {
		t.Fatal(err)
	}
	if running {
		if err := os.WriteFile(cfg.StatusFile, []byte("1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	script := fmt.Sprintf(`#!/bin/sh
CONFIG=%q
LOG=%q
STATUS=%q
case "$1" in
  reload)
    printf 'reload\n' >> "$LOG"
    if grep -q 'DIFFERENT_IP_THRESHOLD="3"' "$CONFIG"; then
      printf 'fixture rejects threshold 3\n'
      exit 7
    fi
    [ -f "$STATUS" ] || exit 8
    printf 'reloaded\n'
    ;;
  *)
    exit 9
    ;;
esac
`, filepath.Join(dir, "ascn.conf"), filepath.Join(dir, "reload.log"), cfg.StatusFile)
	if err := os.WriteFile(cfg.InitScript, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return cfg, original
}
