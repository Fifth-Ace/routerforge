package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeAntiscanOperationContract(t *testing.T) {
	tests := []struct {
		action          string
		scope           string
		args            []string
		requiresRunning bool
	}{
		{action: "update_rules", args: []string{"update_rules"}, requiresRunning: true},
		{action: "read_candidates", args: []string{"read_candidates"}, requiresRunning: true},
		{action: "read_ndm_ipsets", args: []string{"read_ndm_ipsets"}, requiresRunning: true},
		{action: "save_ipsets", args: []string{"save_ipsets"}, requiresRunning: true},
		{action: "update_ipsets", scope: "custom", args: []string{"update_ipsets", "custom"}, requiresRunning: true},
		{action: "update_ipsets", scope: "geo", args: []string{"update_ipsets", "geo"}, requiresRunning: true},
		{action: "retry_load_geo", args: []string{"retry_load_geo"}, requiresRunning: true},
		{action: "update_crontab", args: []string{"update_crontab"}, requiresRunning: false},
	}

	for _, tt := range tests {
		spec, err := normalizeAntiscanOperation(tt.action, tt.scope)
		if err != nil {
			t.Fatalf("%s/%s rejected: %v", tt.action, tt.scope, err)
		}
		if spec.Action != tt.action || spec.Scope != tt.scope || spec.RequiresRunning != tt.requiresRunning {
			t.Fatalf("%s/%s unexpected spec: %+v", tt.action, tt.scope, spec)
		}
		if len(spec.Args) != len(tt.args) {
			t.Fatalf("%s/%s args=%v want=%v", tt.action, tt.scope, spec.Args, tt.args)
		}
		for i := range tt.args {
			if spec.Args[i] != tt.args[i] {
				t.Fatalf("%s/%s args=%v want=%v", tt.action, tt.scope, spec.Args, tt.args)
			}
		}
	}
}

func TestNormalizeAntiscanOperationRejectsUnsupportedSurface(t *testing.T) {
	for _, tt := range []struct {
		action string
		scope  string
	}{
		{action: ""},
		{action: "flush"},
		{action: "token"},
		{action: "update_ipsets"},
		{action: "update_ipsets", scope: "other"},
		{action: "save_ipsets", scope: "geo"},
	} {
		if _, err := normalizeAntiscanOperation(tt.action, tt.scope); err == nil {
			t.Fatalf("%s/%s accepted", tt.action, tt.scope)
		}
	}
}

func TestApplyAntiscanOperationDelegatesDisabledFeatureCommandsToUpstream(t *testing.T) {
	cfg := fakeAntiscanOperationConfig(t)
	if err := os.WriteFile(cfg.StatusFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		action string
		scope  string
		want   string
	}{
		{action: "read_candidates", want: "read_candidates"},
		{action: "read_ndm_ipsets", want: "read_ndm_ipsets"},
		{action: "save_ipsets", want: "save_ipsets"},
		{action: "update_ipsets", scope: "custom", want: "update_ipsets custom"},
		{action: "retry_load_geo", want: "retry_load_geo"},
	}

	for _, tt := range tests {
		if err := os.WriteFile(filepath.Join(filepath.Dir(cfg.InitScript), "operation.log"), nil, 0644); err != nil {
			t.Fatal(err)
		}
		result, status, err := applyAntiscanOperation(context.Background(), cfg, tt.action, tt.scope)
		if err != nil {
			t.Fatalf("%s/%s error=%v", tt.action, tt.scope, err)
		}
		if status != 200 || !result.Changed || !result.Verified || !result.AfterRunning {
			t.Fatalf("%s/%s status=%d result=%+v", tt.action, tt.scope, status, result)
		}
		logged, readErr := os.ReadFile(filepath.Join(filepath.Dir(cfg.InitScript), "operation.log"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.TrimSpace(string(logged)) != tt.want {
			t.Fatalf("%s/%s upstream invocation=%q want=%q", tt.action, tt.scope, logged, tt.want)
		}
	}
}

func TestApplyAntiscanOperationPropagatesDisabledGeoUpdateFromUpstream(t *testing.T) {
	cfg := fakeAntiscanOperationConfig(t)
	if err := os.WriteFile(cfg.StatusFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, status, err := applyAntiscanOperation(context.Background(), cfg, "update_ipsets", "geo")
	if err == nil || status != 409 || result.Verified {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	if !strings.Contains(err.Error(), "upstream update_ipsets geo failed") {
		t.Fatalf("unexpected upstream error: %v", err)
	}
	logged, readErr := os.ReadFile(filepath.Join(filepath.Dir(cfg.InitScript), "operation.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(logged)) != "update_ipsets geo" {
		t.Fatalf("upstream invocation=%q", logged)
	}
}

func TestApplyAntiscanOperationDoesNotGloballyBlockGeoLock(t *testing.T) {
	cfg := fakeAntiscanOperationConfig(t)
	if err := os.WriteFile(cfg.StatusFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.GeoLockFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, status, err := applyAntiscanOperation(context.Background(), cfg, "read_candidates", "")
	if err != nil || status != 200 || !result.Verified {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	logged, readErr := os.ReadFile(filepath.Join(filepath.Dir(cfg.InitScript), "operation.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(logged)) != "read_candidates" {
		t.Fatalf("read_candidates was not delegated during Geo load: %q", logged)
	}
}

func TestApplyAntiscanOperationStoppedStateIsDecidedByUpstream(t *testing.T) {
	cfg := fakeAntiscanOperationConfig(t)
	result, status, err := applyAntiscanOperation(context.Background(), cfg, "read_candidates", "")
	if err == nil || status != 409 || result.Verified {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	if !strings.Contains(err.Error(), "upstream read_candidates failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	logged, readErr := os.ReadFile(filepath.Join(filepath.Dir(cfg.InitScript), "operation.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(logged)) != "read_candidates" {
		t.Fatalf("stopped-state command was not delegated: %q", logged)
	}
}

func fakeAntiscanOperationConfig(t *testing.T) runtimeConfig {
	t.Helper()
	dir := t.TempDir()
	antiscanDir := filepath.Join(dir, "antiscan")
	if err := os.MkdirAll(antiscanDir, 0755); err != nil {
		t.Fatal(err)
	}

	fixture := strings.Replace(antiscanDefaultConfigFixture, `ENABLE_IPS_BAN="1"`, `ENABLE_IPS_BAN="0"`, 1)
	if fixture == antiscanDefaultConfigFixture {
		t.Fatal("test fixture did not disable ENABLE_IPS_BAN")
	}
	if err := os.WriteFile(filepath.Join(antiscanDir, "ascn.conf"), []byte(fixture), 0644); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(dir, "S99ascn")
	logPath := filepath.Join(dir, "operation.log")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> " + logPath + "\n" +
		"if [ \"$1 $2\" = \"update_ipsets geo\" ]; then exit 7; fi\n" +
		"if [ \"$1\" != \"update_crontab\" ] && [ ! -f " + filepath.Join(dir, "ascn.run") + " ]; then exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
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
