package main

import (
	"github.com/Fifth-Ace/routerforge/internal/platform/storageobserve"
	"os"
	"path/filepath"
	"time"
)

var doctorObserveE10 storageobserve.Tracker

func augmentStorageE10(view *doctorView, now time.Time) {
	ids := map[string]string{}
	mounts := map[string][]storageobserve.Mount{}
	for _, d := range view.Disks {
		name := filepath.Base(d.Device)
		ids[name] = d.ID
		for _, m := range d.Mounts {
			mounts[name] = append(mounts[name], storageobserve.Mount{Key: m.Device + "|" + m.Mount, Total: m.TotalBytes, Available: m.AvailableBytes})
		}
	}
	var stats map[string]storageobserve.Counters
	if f, e := os.Open("/proc/diskstats"); e == nil {
		stats = storageobserve.Parse(f)
		f.Close()
	}
	for name, id := range ids {
		if doctorDiskID(name) != id {
			ids[name] = "volatile:" + name
		}
	}
	observations := doctorObserveE10.Observe(now, ids, stats, mounts)
	for i := range view.Disks {
		d := &view.Disks[i]
		if o, ok := observations[filepath.Base(d.Device)]; ok {
			d.Observation = &o
		}
	}
}
