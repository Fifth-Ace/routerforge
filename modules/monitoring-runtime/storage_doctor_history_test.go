package main

import (
	"strings"
	"testing"
	"time"
)

func TestDoctorIdentitySeparatesCacheByVisibleDiskMetadata(t *testing.T) {
	original := blockDevice{Name: "sdb", SizeBytes: 100, Vendor: "A", Model: "USB", Transport: "usb"}
	replacement := original
	replacement.Model = "USB-B"
	if doctorSmartFallbackIdentity(original) == doctorSmartFallbackIdentity(replacement) {
		t.Fatal("SMART cache can confuse distinct USB disks")
	}
	replacement = original
	replacement.SizeBytes = 200
	if doctorSmartFallbackIdentity(original) == doctorSmartFallbackIdentity(replacement) {
		t.Fatal("SMART cache can confuse different disk sizes")
	}
}

func TestDoctorDiskIdentityResetDropsOnlyReplacedHistory(t *testing.T) {
	doc := doctorHistoryDocument{
		Temperatures:   map[string][]doctorTemperaturePoint{"/dev/sdb": {{Celsius: 30}}, "/dev/sdc": {{Celsius: 40}}},
		DiskIdentities: map[string]string{"/dev/sdb": "hw:old", "/dev/sdc": "hw:other"},
	}
	if doctorHistoryBindDiskIdentity(&doc, "/dev/sdb", "hw:old") {
		t.Fatal("identical disk triggered mutation")
	}
	if !doctorHistoryBindDiskIdentity(&doc, "/dev/sdb", "hw:new") {
		t.Fatal("replacement was not detected")
	}
	if len(doc.Temperatures["/dev/sdb"]) != 0 || len(doc.Temperatures["/dev/sdc"]) != 1 {
		t.Fatal("wrong temperature history reset")
	}
	if doc.DiskIdentities["/dev/sdb"] != "hw:new" {
		t.Fatal("identity not updated")
	}
}

func TestDoctorOldHistoryIsAdoptedWithoutErasure(t *testing.T) {
	doc := doctorHistoryDocument{Temperatures: map[string][]doctorTemperaturePoint{"/dev/sdb": {{Celsius: 30}}}}
	if !doctorHistoryBindDiskIdentity(&doc, "/dev/sdb", "hw:new") {
		t.Fatal("identity not saved")
	}
	if len(doc.Temperatures["/dev/sdb"]) != 1 {
		t.Fatal("legacy history erased")
	}
}

func TestDoctorKernelEvidenceClassification(t *testing.T) {
	for _, tc := range []struct{ line, kind string }{
		{"EXT4-fs error (device sda1): ext4_find_entry", "fs_error"},
		{"Buffer I/O error on dev sda1, logical block 1", "io_error"},
		{"usb 1-1: USB disconnect, device number 2", "usb_disconnect"},
		{"usb 1-1: reset high-speed USB device number 2", "usb_reset"},
		{"EXT4-fs (sda1): remounting filesystem read-only", "read_only"},
		{"ordinary device connected successfully", ""},
	} {
		kind, _, ok := doctorKernelKind(tc.line)
		if kind != tc.kind || ok != (tc.kind != "") {
			t.Errorf("%q: kind=%q, detected=%v", tc.line, kind, ok)
		}
	}
}
func TestDoctorKernelEventsDeterministicWithinBoot(t *testing.T) {
	now := time.Now().UTC()
	sample := "[123.456] Buffer I/O error on dev sda1\n[124.456] ordinary boot message\n"
	first := doctorKernelEvents(sample, "boot-A", now)
	second := doctorKernelEvents(sample, "boot-A", now.Add(time.Minute))
	changedBoot := doctorKernelEvents(sample, "boot-B", now)
	if len(first) != 1 || len(second) != 1 || len(changedBoot) != 1 {
		t.Fatalf("unexpected scan results: %v, %v, %v", first, second, changedBoot)
	}
	if first[0].ID != second[0].ID {
		t.Fatal("same dmesg message changed identity")
	}
	if first[0].ID == changedBoot[0].ID {
		t.Fatal("different boot did not change identity")
	}
	if first[0].Device != "sda1" {
		t.Fatalf("unexpected disk %q", first[0].Device)
	}
	if !strings.HasPrefix(first[0].ID, "kernel:") {
		t.Fatal("unexpected ID namespace")
	}
}
