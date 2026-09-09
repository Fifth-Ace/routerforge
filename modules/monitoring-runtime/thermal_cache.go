package main

import (
	"sync"
	"time"
)

type thermalSensor struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Role        string    `json:"role"`
	SensorIndex int       `json:"sensor_index"`
	Detail      string    `json:"detail,omitempty"`
	Source      string    `json:"source"`
	TempC       float64   `json:"temp_c"`
	Status      string    `json:"status"`
	WarnC       float64   `json:"warn_c"`
	CriticalC   float64   `json:"critical_c"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type thermalCollector struct {
	mu         sync.RWMutex
	interval   time.Duration
	sensors    []thermalSensor
	scanned    time.Time
	refreshing bool
	collect    func(time.Time) []thermalSensor
}

func newThermalCollectorWith(interval time.Duration, collect func(time.Time) []thermalSensor) *thermalCollector {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	if collect == nil {
		panic("thermal collector function is required")
	}
	t := &thermalCollector{
		interval: interval,
		collect:  collect,
	}
	t.refresh()
	return t
}

func (t *thermalCollector) snapshot() ([]thermalSensor, time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]thermalSensor(nil), t.sensors...), t.scanned
}

// snapshotForRequest implements stale-while-revalidate for HTTP reads.
// The caller always receives the current cached snapshot. If it is stale,
// exactly one background refresh is started after the snapshot is captured.
func (t *thermalCollector) snapshotForRequest() ([]thermalSensor, time.Time) {
	sensors, scanned := t.snapshot()
	t.maybeRefresh()
	return sensors, scanned
}

func (t *thermalCollector) maybeRefresh() bool {
	t.mu.Lock()
	stale := time.Since(t.scanned) >= t.interval
	if !stale || t.refreshing {
		t.mu.Unlock()
		return false
	}
	t.refreshing = true
	t.mu.Unlock()

	go t.refresh()
	return true
}

func (t *thermalCollector) refresh() {
	now := time.Now()
	sensors := t.collect(now)

	t.mu.Lock()
	t.sensors = sensors
	t.scanned = now
	t.refreshing = false
	t.mu.Unlock()
}
