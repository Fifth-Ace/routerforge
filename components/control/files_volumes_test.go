package main

import "testing"

func TestAdminFileStorageMountCandidate(t *testing.T) {
	cases := []struct {
		mount  string
		fsType string
		want   bool
	}{
		{"/tmp/mnt/usb", "ext4", true},
		{"/mnt/usb", "vfat", true},
		{"/media/disk", "exfat", true},
		{"/storage", "ubifs", true},
		{"/proc", "proc", false},
		{"/sys", "sysfs", false},
		{"/tmp/cache", "ext4", false},
	}
	for _, tc := range cases {
		if got := adminFileStorageMountCandidate(tc.mount, tc.fsType); got != tc.want {
			t.Fatalf("candidate(%q,%q)=%v want %v", tc.mount, tc.fsType, got, tc.want)
		}
	}
}

func TestAdminFileForbiddenSystemPath(t *testing.T) {
	for _, path := range []string{"/proc", "/proc/cpuinfo", "/sys/class", "/dev/null", "/run/lock"} {
		if !adminFileForbiddenSystemPath(path) {
			t.Fatalf("expected %q to be forbidden", path)
		}
	}
	for _, path := range []string{"/", "/etc", "/opt", "/tmp", "/storage"} {
		if adminFileForbiddenSystemPath(path) {
			t.Fatalf("did not expect %q to be forbidden", path)
		}
	}
}

func TestMountOptionsReadOnly(t *testing.T) {
	if !mountOptionsReadOnly("rw,nosuid,ro,nodev") {
		t.Fatal("expected ro option to be detected")
	}
	if mountOptionsReadOnly("rw,nosuid,nodev") {
		t.Fatal("did not expect rw mount to be read-only")
	}
}
