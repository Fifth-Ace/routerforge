package storageobserve

import (
	"fmt"
	"testing"
	"time"
)

func TestE10ObservationBounds(t *testing.T) {
	var tracker Tracker
	start := time.Unix(1000, 0)
	devices := map[string]string{}
	mounts := map[string][]Mount{}
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("disk%02d", i)
		devices[name] = "hw:" + name
		for j := 0; j < 6; j++ {
			mounts[name] = append(mounts[name], Mount{Key: fmt.Sprintf("mount%d", j), Total: 1000, Available: 800})
		}
	}
	var observations map[string]Observation
	for i := 0; i < 100; i++ {
		observations = tracker.Observe(start.Add(time.Duration(i)*15*time.Minute), devices, nil, mounts)
	}
	if len(observations) != 8 || tracker.Summary()["tracked_disks"] != 8 {
		t.Fatal("disk bound exceeded")
	}
	for _, item := range observations {
		if len(item.Capacity) != 4 {
			t.Fatal("mount bound exceeded")
		}
		for _, cap := range item.Capacity {
			if len(cap.Points) != 96 || cap.EstimatedDays != nil || cap.NetConsumptionBPS != nil {
				t.Fatal("history limit or false forecast", cap)
			}
		}
	}
}

func TestE10ShortPollPreservesBaseline(t *testing.T) {
	var tracker Tracker
	start := time.Unix(1000, 0)
	devices := map[string]string{"sda": "hw:fixture"}
	tracker.Observe(start, devices, map[string]Counters{"sda": {}}, nil)
	first := tracker.Observe(start.Add(500*time.Millisecond), devices, map[string]Counters{"sda": {Reads: 1, ReadSectors: 2}}, nil)
	if first["sda"].IO != nil {
		t.Fatal("subsecond rate was reported")
	}
	second := tracker.Observe(start.Add(2*time.Second), devices, map[string]Counters{"sda": {Reads: 4, ReadSectors: 8}}, nil)
	io := second["sda"].IO
	if io == nil || io.WindowSeconds != 2 || io.ReadOPS != 2 || io.ReadBPS != 2048 {
		t.Fatal("subsecond poll consumed baseline", io)
	}
}
