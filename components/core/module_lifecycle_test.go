package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleLifecycleDoesNotExposeCore(t *testing.T) {
	if _, ok := moduleLifecycleSpecFor("routerforge-core"); ok {
		t.Fatal("RouterForge Core must not be controllable through module lifecycle")
	}
}

func TestModuleLifecycleDisableEnableMarkers(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "actions.log")
	servicePath := filepath.Join(root, "service.sh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$1\" >> " + logPath + "\nexit 0\n"
	if err := os.WriteFile(servicePath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	oldSpecs := moduleLifecycleSpecs
	oldRoot := moduleLifecycleDisabledRoot
	moduleLifecycleDisabledRoot = filepath.Join(root, "disabled")
	moduleLifecycleSpecs = map[string]moduleLifecycleSpec{
		"test-module": {
			ID:       "test-module",
			Services: []string{servicePath},
		},
	}
	t.Cleanup(func() {
		moduleLifecycleSpecs = oldSpecs
		moduleLifecycleDisabledRoot = oldRoot
	})

	spec, ok := moduleLifecycleSpecFor("test-module")
	if !ok {
		t.Fatal("test lifecycle spec missing")
	}

	if output, err := moduleLifecycleRunInit(context.Background(), servicePath, "stop"); err != nil {
		t.Fatalf("stop: %v output=%q", err, output)
	}
	if err := moduleLifecycleSetDisabled("test-module", true); err != nil {
		t.Fatal(err)
	}
	if marker := moduleLifecycleDisabledMarker("test-module"); marker == "" {
		t.Fatal("disable marker path missing")
	} else if _, err := os.Stat(marker); err != nil {
		t.Fatalf("disable marker not created: %v", err)
	}

	if err := moduleLifecycleSetDisabled("test-module", false); err != nil {
		t.Fatal(err)
	}
	if marker := moduleLifecycleDisabledMarker("test-module"); marker != "" {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("disable marker still exists: %v", err)
		}
	}
	if output, err := moduleLifecycleRunInit(context.Background(), servicePath, "start"); err != nil {
		t.Fatalf("start: %v output=%q", err, output)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != "stop\nstart" {
		t.Fatalf("actions=%q", got)
	}

	_ = spec
}

func TestCatalogLifecycleOverrideTracksDisabledAndRuntime(t *testing.T) {
	root := t.TempDir()
	oldSpecs := moduleLifecycleSpecs
	oldRoot := moduleLifecycleDisabledRoot
	moduleLifecycleDisabledRoot = root
	moduleLifecycleSpecs = map[string]moduleLifecycleSpec{
		"test-module": {
			ID:           "test-module",
			Services:     []string{"/opt/etc/init.d/Stest"},
			ProcessNames: []string{"test-runtime"},
		},
	}
	t.Cleanup(func() {
		moduleLifecycleSpecs = oldSpecs
		moduleLifecycleDisabledRoot = oldRoot
	})

	marker := moduleLifecycleDisabledMarker("test-module")
	if err := os.WriteFile(marker, []byte("disabled\n"), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalogItem{
		ID:                   "test-module",
		Kind:                 "module",
		Name:                 "Test Module",
		Source:               "routerforge-official",
		Managed:              true,
		PackageAuthoritative: true,
		Detection: catalogDetection{
			Packages: []string{"routerforge-test"},
		},
	}

	exists := func(path string) bool {
		return path == marker || path == "/opt/etc/init.d/Stest"
	}
	finalizeCatalogItem(
		&item,
		map[string]string{"routerforge-test": "1.0.0"},
		map[string]bool{"test-runtime": true},
		exists,
	)

	if !item.LifecycleManaged {
		t.Fatal("lifecycle-managed flag missing")
	}
	if !item.Disabled {
		t.Fatal("disabled state missing")
	}
	if !item.ServiceRunning {
		t.Fatal("runtime process state was lost")
	}
	if item.Enabled {
		t.Fatal("disabled module must not report enabled")
	}
	if item.Service != "/opt/etc/init.d/Stest" {
		t.Fatalf("service=%q", item.Service)
	}
}
