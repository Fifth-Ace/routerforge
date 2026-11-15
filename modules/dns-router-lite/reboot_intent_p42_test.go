package lite

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression: Entware shutdown uses init "stop". That must not rewrite the
// user's persisted ON/ON choice; permanent OFF is only an explicit disable.
func TestP42EntwareShutdownDoesNotPersistDNSOff(t *testing.T) {
	initPath := "../dns-router-runtime/packaging/S95routerforge-dns-router-dev"
	raw, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatal(err)
	}
	marker := "\ncase \"${1:-}\" in\n"
	start := strings.LastIndex(string(raw), marker)
	if start < 0 {
		t.Fatal("init script dispatch not found")
	}
	dispatch := string(raw)[start+1:]
	stub := "stop_service() { printf 'STOP:%s\\n' \"$1\"; }\nstart_service() { echo START; }\nowned() { return 1; }\n"
	for _, tc := range []struct{ action, want string }{
		{"stop", "STOP:maintenance"},
		{"maintenance", "STOP:maintenance"},
		{"disable", "STOP:stop"},
		{"restart", "STOP:maintenance\nSTART"},
		{"start", "START"},
	} {
		t.Run("init_"+tc.action, func(t *testing.T) {
			cmd := exec.Command("sh", "-c", stub+dispatch, "rf-init", tc.action)
			out, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("init %s: output=%q err=%v want=%q", tc.action, out, err, tc.want)
			}
		})
	}

	prerm, err := os.ReadFile("../dns-router-runtime/packaging/prerm-dev")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	fake := filepath.Join(tmp, "init")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"$1\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	removeScript := strings.ReplaceAll(string(prerm), "/opt/etc/init.d/S95routerforge-dns-router", fake)
	for _, tc := range []struct{ action, want string }{
		{"remove", "disable"},
		{"upgrade", "maintenance"},
		{"deconfigure", "maintenance"},
	} {
		t.Run("prerm_"+tc.action, func(t *testing.T) {
			out, err := exec.Command("sh", "-c", removeScript, "rf-prerm", tc.action).CombinedOutput()
			if err != nil || !strings.HasPrefix(string(out), tc.want+"\n") {
				t.Fatalf("prerm %s: %q err=%v", tc.action, out, err)
			}
		})
	}

	manager, rev, _ := lifecycleFixture(t)
	if err := manager.control("start", rev); err != nil {
		t.Fatal(err)
	}
	if err := manager.gateControl(context.Background(), "enable", rev); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manager.intentStore().Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.control("maintenance", ""); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(manager.intentStore().Path())
	if err != nil || string(before) != string(after) {
		t.Fatal("shutdown/maintenance rewrote persistent ON intent", err)
	}
	resumed := resumedManager(t, manager)
	if err := resumed.Resume(); err != nil {
		t.Fatal(err)
	}
	if !resumed.ActiveState() || resumed.Gate.Status()["active"] != true {
		t.Fatal("ON/ON intent not restored after shutdown")
	}
	if err := resumed.control("stop", ""); err != nil {
		t.Fatal(err)
	}
	intent, err := resumed.intentStore().Load()
	if err != nil || intent.ServiceEnabled || intent.InterceptionEnabled {
		t.Fatal("explicit user disable did not persist OFF/OFF", err)
	}
}
