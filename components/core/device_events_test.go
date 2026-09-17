package main

import (
	"testing"
	"time"
)

func TestDeviceStorageFilesystemMonitoring(t *testing.T) {
	cases := []struct {
		fs      string
		options string
		want    bool
	}{
		{"ext4", "rw,relatime", true},
		{"ubifs", "rw,relatime", true},
		{"squashfs", "ro,relatime", false},
		{"proc", "rw,nosuid", false},
		{"tmpfs", "rw,nosuid", false},
		{"ext4", "ro,relatime", false},
	}
	for _, tc := range cases {
		if got := deviceStorageFilesystemMonitored(tc.fs, tc.options); got != tc.want {
			t.Fatalf("%s %s: got %v want %v", tc.fs, tc.options, got, tc.want)
		}
	}
}

func TestDeviceStorageSeverity(t *testing.T) {
	if classifyDeviceStorage(79.99) != "ok" {
		t.Fatal("79.99 must be ok")
	}
	if classifyDeviceStorage(80) != "warning" {
		t.Fatal("80 must be warning")
	}
	if classifyDeviceStorage(90) != "critical" {
		t.Fatal("90 must be critical")
	}
}

func TestDeviceThermalSeverity(t *testing.T) {
	if classifyDeviceThermal(79.9) != "ok" {
		t.Fatal("79.9 must be ok")
	}
	if classifyDeviceThermal(80) != "warning" {
		t.Fatal("80 must be warning")
	}
	if classifyDeviceThermal(90) != "critical" {
		t.Fatal("90 must be critical")
	}
}

func TestNetworkOnlyAlertsAfterObservedUp(t *testing.T) {
	runtime := deviceObserverRuntime{
		network:       map[string]bool{},
		networkSeenUp: map[string]bool{},
		storage:       map[string]string{},
		thermal:       map[string]string{},
		watchdogLast:  map[string]time.Time{},
		active:        map[string]deviceProblem{},
	}
	now := time.Date(2026, 9, 18, 1, 0, 0, 0, time.UTC)

	runtime.observeNetwork(now, map[string]bool{"eth9": false})
	if len(runtime.active) != 0 {
		t.Fatalf("initially-down interface must not alert: %#v", runtime.active)
	}

	runtime.observeNetwork(now.Add(time.Second), map[string]bool{"eth9": true})
	if len(runtime.active) != 0 {
		t.Fatalf("recovery to first observed up must not leave alert: %#v", runtime.active)
	}

	runtime.observeNetwork(now.Add(2*time.Second), map[string]bool{"eth9": false})
	if _, ok := runtime.active["network:eth9"]; !ok {
		t.Fatalf("interface that went down after being up must alert: %#v", runtime.active)
	}

	runtime.observeNetwork(now.Add(3*time.Second), map[string]bool{"eth9": true})
	if len(runtime.active) != 0 {
		t.Fatalf("recovered interface must clear alert: %#v", runtime.active)
	}
}

func TestPreferDeviceStorageMount(t *testing.T) {
	if !preferDeviceStorageMount("/opt", "/tmp/mnt/uuid") {
		t.Fatal("/opt must win over duplicate removable mount")
	}
	if preferDeviceStorageMount("/tmp/mnt/uuid", "/opt") {
		t.Fatal("duplicate removable mount must not replace /opt")
	}
}
