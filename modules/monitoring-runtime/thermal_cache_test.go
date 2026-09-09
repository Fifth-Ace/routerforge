package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestThermalSnapshotForRequestStaleWhileRevalidateSingleflight(t *testing.T) {
	staleAt := time.Now().Add(-time.Minute).UTC()
	started := make(chan struct{})
	release := make(chan struct{})
	var runs atomic.Int32

	collector := &thermalCollector{
		interval: 10 * time.Second,
		sensors: []thermalSensor{{
			ID:        "old",
			Name:      "old",
			Category:  "soc",
			Source:    "test",
			TempC:     42,
			Status:    "ok",
			UpdatedAt: staleAt,
		}},
		scanned: staleAt,
		collect: func(now time.Time) []thermalSensor {
			if runs.Add(1) == 1 {
				close(started)
			}
			<-release
			return []thermalSensor{{
				ID:        "new",
				Name:      "new",
				Category:  "soc",
				Source:    "test",
				TempC:     43,
				Status:    "ok",
				UpdatedAt: now,
			}}
		},
	}

	sensors, scanned := collector.snapshotForRequest()
	if len(sensors) != 1 || sensors[0].ID != "old" {
		close(release)
		t.Fatalf("first stale snapshot=%+v, want cached old snapshot", sensors)
	}
	if !scanned.Equal(staleAt) {
		close(release)
		t.Fatalf("first scanned_at=%s want=%s", scanned, staleAt)
	}

	select {
	case <-started:
	case <-time.After(250 * time.Millisecond):
		close(release)
		t.Fatal("background thermal refresh did not start")
	}

	const callers = 16
	start := make(chan struct{})
	errs := make(chan string, callers)
	var wg sync.WaitGroup
	wg.Add(callers)

	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			<-start
			gotSensors, gotScanned := collector.snapshotForRequest()
			if len(gotSensors) != 1 || gotSensors[0].ID != "old" {
				errs <- "concurrent caller did not receive cached old snapshot"
				return
			}
			if !gotScanned.Equal(staleAt) {
				errs <- "concurrent caller received unexpected scanned_at"
			}
		}()
	}

	close(start)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		close(release)
		t.Fatal("concurrent stale callers blocked on background refresh")
	}
	close(errs)

	for msg := range errs {
		close(release)
		t.Fatal(msg)
	}

	if got := runs.Load(); got != 1 {
		close(release)
		t.Fatalf("background refresh runs=%d want=1", got)
	}

	close(release)

	deadline := time.Now().Add(time.Second)
	for {
		gotSensors, gotScanned := collector.snapshot()

		collector.mu.RLock()
		refreshing := collector.refreshing
		collector.mu.RUnlock()

		if len(gotSensors) == 1 &&
			gotSensors[0].ID == "new" &&
			gotScanned.After(staleAt) &&
			!refreshing {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"refresh did not publish new snapshot: sensors=%+v scanned=%s refreshing=%v",
				gotSensors,
				gotScanned,
				refreshing,
			)
		}
		time.Sleep(time.Millisecond)
	}

	if got := runs.Load(); got != 1 {
		t.Fatalf("completed refresh runs=%d want=1", got)
	}

	gotSensors, _ := collector.snapshotForRequest()
	if len(gotSensors) != 1 || gotSensors[0].ID != "new" {
		t.Fatalf("fresh snapshot=%+v, want new snapshot", gotSensors)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("fresh request started another refresh; runs=%d want=1", got)
	}
}

func TestNewThermalCollectorWithKeepsStartupRefreshSynchronous(t *testing.T) {
	var runs atomic.Int32
	collector := newThermalCollectorWith(30*time.Second, func(now time.Time) []thermalSensor {
		runs.Add(1)
		return []thermalSensor{{
			ID:        "startup",
			Name:      "startup",
			Category:  "soc",
			Source:    "test",
			TempC:     41,
			Status:    "ok",
			UpdatedAt: now,
		}}
	})

	if got := runs.Load(); got != 1 {
		t.Fatalf("startup refresh runs=%d want=1", got)
	}
	sensors, scanned := collector.snapshot()
	if len(sensors) != 1 || sensors[0].ID != "startup" {
		t.Fatalf("startup snapshot=%+v", sensors)
	}
	if scanned.IsZero() {
		t.Fatal("startup scanned_at is zero")
	}
}
