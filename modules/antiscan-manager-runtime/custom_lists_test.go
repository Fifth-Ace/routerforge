package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAntiscanManagedCustomListTarget(t *testing.T) {
	for _, tt := range []struct {
		list string
		file string
		set  string
	}{
		{list: "blacklist", file: "ascn_custom_blacklist.txt", set: "ascn_custom_blacklist"},
		{list: "whitelist", file: "ascn_custom_whitelist.txt", set: "ascn_custom_whitelist"},
		{list: "exclude", file: "ascn_custom_exclude.txt", set: "ascn_custom_exclude"},
	} {
		fileName, setName, err := antiscanManagedCustomListTarget(tt.list)
		if err != nil || fileName != tt.file || setName != tt.set {
			t.Fatalf("%s file=%q set=%q err=%v", tt.list, fileName, setName, err)
		}
	}
	if _, _, err := antiscanManagedCustomListTarget("geo"); err == nil {
		t.Fatal("unsupported list accepted")
	}
}

func TestParseAntiscanManagedCustomListEntriesCanonicalizesAndDeduplicates(t *testing.T) {
	entries, skipped, err := parseAntiscanManagedCustomListEntries([]byte("# keep\n192.0.2.1 note\n192.0.2.1\n198.51.100.7/24\nnot-an-ip\n"))
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 {
		t.Fatalf("skipped=%d want=1", skipped)
	}
	got := strings.Join(entries, ",")
	if got != "192.0.2.1,198.51.100.0/24" {
		t.Fatalf("entries=%q", got)
	}
}

func TestRemoveAntiscanManagedCustomListEntryPreservesOtherLines(t *testing.T) {
	original := []byte("# keep me\n192.0.2.1 note\n198.51.100.0/24\n\n")
	updated, changed, err := removeAntiscanManagedCustomListEntry(original, "192.0.2.1")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	text := string(updated)
	if strings.Contains(text, "192.0.2.1") || !strings.Contains(text, "# keep me") || !strings.Contains(text, "198.51.100.0/24") {
		t.Fatalf("updated=%q", text)
	}
}

func TestClearAntiscanManagedCustomListEntriesPreservesComments(t *testing.T) {
	original := []byte("# keep me\n192.0.2.1\n198.51.100.0/24 note\n\n")
	updated, changed, err := clearAntiscanManagedCustomListEntries(original)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if strings.Contains(string(updated), "192.0.2.1") || strings.Contains(string(updated), "198.51.100.0/24") || !strings.Contains(string(updated), "# keep me") {
		t.Fatalf("updated=%q", string(updated))
	}
}

func TestApplyAntiscanCustomListMutationInactiveCRUD(t *testing.T) {
	cfg := fakeAntiscanCustomListConfig(t, antiscanDefaultConfigFixture)
	listPath := filepath.Join(cfg.AntiscanDir, "ascn_custom_blacklist.txt")
	if err := os.WriteFile(listPath, []byte("# seed\n192.0.2.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	add, status, err := applyAntiscanCustomListMutation(context.Background(), cfg, "add", "blacklist", "198.51.100.7/24")
	if err != nil || status != 200 || !add.Changed || !add.Verified || add.Active || add.CountAfter != 2 {
		t.Fatalf("add status=%d err=%v result=%+v", status, err, add)
	}
	if add.Reloaded {
		t.Fatal("inactive add unexpectedly reloaded runtime")
	}

	del, status, err := applyAntiscanCustomListMutation(context.Background(), cfg, "delete", "blacklist", "192.0.2.1")
	if err != nil || status != 200 || !del.Changed || !del.Verified || del.CountAfter != 1 {
		t.Fatalf("delete status=%d err=%v result=%+v", status, err, del)
	}

	clear, status, err := applyAntiscanCustomListMutation(context.Background(), cfg, "clear", "blacklist", "")
	if err != nil || status != 200 || !clear.Changed || !clear.Verified || clear.CountAfter != 0 {
		t.Fatalf("clear status=%d err=%v result=%+v", status, err, clear)
	}
	data, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# seed") {
		t.Fatalf("comment was not preserved: %q", string(data))
	}
}

func TestApplyAntiscanCustomListMutationProtectsConfiguredEmptyList(t *testing.T) {
	fixture := strings.Replace(antiscanDefaultConfigFixture, `CUSTOM_LISTS_BLOCK_MODE="0"`, `CUSTOM_LISTS_BLOCK_MODE="whitelist"`, 1)
	cfg := fakeAntiscanCustomListConfig(t, fixture)
	listPath := filepath.Join(cfg.AntiscanDir, "ascn_custom_whitelist.txt")
	if err := os.WriteFile(listPath, []byte("192.0.2.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, status, err := applyAntiscanCustomListMutation(context.Background(), cfg, "delete", "whitelist", "192.0.2.1")
	if err == nil || status != 409 || result.Changed {
		t.Fatalf("delete status=%d err=%v result=%+v", status, err, result)
	}
	data, readErr := os.ReadFile(listPath)
	if readErr != nil || string(data) != "192.0.2.1\n" {
		t.Fatalf("configured list changed: data=%q err=%v", string(data), readErr)
	}

	result, status, err = applyAntiscanCustomListMutation(context.Background(), cfg, "clear", "whitelist", "")
	if err == nil || status != 409 || result.Changed {
		t.Fatalf("clear status=%d err=%v result=%+v", status, err, result)
	}
}

func TestApplyAntiscanCustomListReloadInactiveIsNoop(t *testing.T) {
	cfg := fakeAntiscanCustomListConfig(t, antiscanDefaultConfigFixture)
	result, status, err := applyAntiscanCustomListMutation(context.Background(), cfg, "reload", "exclude", "")
	if err != nil || status != 200 || result.Changed || !result.Verified || result.Reloaded {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected no-op warning")
	}
}

func fakeAntiscanCustomListConfig(t *testing.T, fixture string) runtimeConfig {
	t.Helper()
	dir := t.TempDir()
	antiscanDir := filepath.Join(dir, "antiscan")
	if err := os.MkdirAll(antiscanDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(antiscanDir, "ascn.conf"), []byte(fixture), 0644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "S99ascn")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return runtimeConfig{
		AntiscanDir:    antiscanDir,
		InitScript:     scriptPath,
		StatusFile:     filepath.Join(dir, "ascn.run"),
		ConfigLockFile: filepath.Join(dir, "ascn.lock"),
		GeoLockFile:    filepath.Join(dir, "ascn_geo.lock"),
	}
}
