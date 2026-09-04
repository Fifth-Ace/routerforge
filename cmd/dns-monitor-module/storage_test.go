package main

import "testing"

func TestCounterDelta(t *testing.T) {
	if got := counterDelta(120, 100); got != 20 {
		t.Fatalf("delta=%v", got)
	}
	if got := counterDelta(10, 100); got != 0 {
		t.Fatalf("wrap delta=%v", got)
	}
}

func TestPseudoFilesystem(t *testing.T) {
	if !pseudoFilesystem("proc") {
		t.Fatal("proc must be pseudo")
	}
	if pseudoFilesystem("ext4") {
		t.Fatal("ext4 must not be pseudo")
	}
}

func TestUserMountsDeduplicatesDeviceAndPrefersOpt(t *testing.T) {
	all := []storageMount{
		{Device: "/dev/sda2", Mount: "/tmp/mnt/uuid", FSType: "ext4", TotalBytes: 100},
		{Device: "/dev/ubi0_0", Mount: "/storage", FSType: "ubifs", TotalBytes: 100},
		{Device: "/dev/sda2", Mount: "/opt", FSType: "ext4", TotalBytes: 100},
		{Device: "rootfs", Mount: "/", FSType: "squashfs", TotalBytes: 100},
	}
	got := userMounts(all)
	if len(got) != 2 {
		t.Fatalf("got %d mounts, want 2: %#v", len(got), got)
	}
	if got[0].Mount != "/opt" || got[0].Device != "/dev/sda2" {
		t.Fatalf("first mount=%#v, want /opt on /dev/sda2", got[0])
	}
	if got[1].Mount != "/storage" {
		t.Fatalf("second mount=%#v, want /storage", got[1])
	}
}

func TestSystemOnlyBlockDevice(t *testing.T) {
	for _, name := range []string{"mtdblock0", "mtdblock18", "ubiblock0_0", "zram0"} {
		if !systemOnlyBlockDevice(name) {
			t.Fatalf("%s must be system-only", name)
		}
	}
	for _, name := range []string{"sda", "nvme0n1", "mmcblk0"} {
		if systemOnlyBlockDevice(name) {
			t.Fatalf("%s must remain visible", name)
		}
	}
}
