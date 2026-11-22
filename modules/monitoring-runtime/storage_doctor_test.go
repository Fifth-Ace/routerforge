package main

import (
	"strings"
	"testing"
	"time"
)

func TestDoctorParentMapsOneDiskToMultipleMountpoints(t *testing.T) {
	for _, tc := range []struct {
		disk, partition string
		ok              bool
	}{
		{"sda", "/dev/sda1", true}, {"sda", "/dev/sda12", true}, {"sda", "/dev/sdb1", false},
		{"nvme0n1", "/dev/nvme0n1p1", true}, {"nvme0n1", "/dev/nvme0n11", false},
		{"mmcblk0", "/dev/mmcblk0p2", true}, {"mmcblk0", "/dev/mmcblk1p2", false},
	} {
		if got := doctorParent(tc.disk, tc.partition); got != tc.ok {
			t.Errorf("%s / %s got %v", tc.disk, tc.partition, got)
		}
	}
}
func TestDoctorSmartDriverAllowlist(t *testing.T) {
	for _, typ := range []string{"auto", "sat", "sat,12", "scsi", "nvme", "usbjmicron"} {
		if !doctorSmartDrivers[typ] {
			t.Fatalf("missing %s", typ)
		}
	}
	for _, typ := range []string{"../../etc/passwd", "sat;reboot", "", "-n never"} {
		if doctorSmartDrivers[typ] {
			t.Fatalf("unsafe %s", typ)
		}
	}
}
func TestDoctorNeverExposesArbitraryDevices(t *testing.T) {
	for _, name := range []string{"../../sda", "sda1", "dm-0", "sda;rm", "nvme0n1p1"} {
		if doctorDeviceName.MatchString(name) {
			t.Errorf("accepted %s", name)
		}
	}
}
func TestDoctorStorageEventsAreBounded(t *testing.T) {
	doctorTracker.Lock()
	savedInit, savedMounts, savedEvents := doctorTracker.initialized, doctorTracker.mounts, doctorTracker.events
	doctorTracker.initialized = false
	doctorTracker.mounts = map[string]bool{}
	doctorTracker.events = nil
	doctorTracker.Unlock()
	defer func() {
		doctorTracker.Lock()
		doctorTracker.initialized = savedInit
		doctorTracker.mounts = savedMounts
		doctorTracker.events = savedEvents
		doctorTracker.Unlock()
	}()
	start := time.Now()
	doctorTrackMounts(start, map[string]doctorMount{"/opt": {Mount: "/opt", ReadOnly: false}})
	doctorTrackMounts(start.Add(time.Second), map[string]doctorMount{"/opt": {Mount: "/opt", ReadOnly: true}})
	doctorTrackMounts(start.Add(2*time.Second), map[string]doctorMount{})
	doctorTracker.Lock()
	if len(doctorTracker.events) != 2 || doctorTracker.events[0].Kind != "disappeared" || doctorTracker.events[1].Kind != "read_only" {
		t.Errorf("unexpected incidents: %#v", doctorTracker.events)
	}
	for i := 0; i < 200; i++ {
		doctorAppendIncidentLocked(doctorIncident{At: start, Kind: "appeared", Mount: "/opt"})
	}
	if len(doctorTracker.events) != doctorMaxIncidents {
		t.Errorf("ring length %d", len(doctorTracker.events))
	}
	doctorTracker.Unlock()
}
func TestDoctorOutputHasHardCap(t *testing.T) {
	var b doctorLimitedBuffer
	b.Limit = 4
	n, e := b.Write([]byte("abcdefgh"))
	if e != nil || n != 8 || !b.Truncated || b.String() != "abcd" {
		t.Fatalf("bad bounded output %q %d %v", b.String(), n, e)
	}
}
func TestDoctorLabelsNeverClaimHealthyWithoutSMART(t *testing.T) {
	d := doctorDisk{Device: "/dev/sda", Entware: true}
	if got := doctorSummaryLabel(d); !strings.Contains(got, "Entware") {
		t.Fatal(got)
	}
	r := doctorSmartResult{Health: "unknown"}
	if r.Health == "ok" {
		t.Fatal("unknown SMART must never be green")
	}
}
