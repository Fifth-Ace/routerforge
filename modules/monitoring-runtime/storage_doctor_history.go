package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// A bounded, best-effort internal history file. Its contents never control device operations.
const doctorHistoryFile = "/opt/var/lib/routerforge/monitoring/storage-health.json"
const doctorHistoryLimit = 256
const doctorKernelScanInterval = 60 * time.Second

// Observed timestamps are detection times, not reconstructed kernel event times.
type doctorHealthEvent struct {
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	Kind     string    `json:"kind"`
	Severity string    `json:"severity"`
	Device   string    `json:"device,omitempty"`
	Summary  string    `json:"summary"`
	Detail   string    `json:"detail,omitempty"`
	Recovery bool      `json:"recovery,omitempty"`
	Origin   string    `json:"origin"`
}
type doctorTemperatureReading struct {
	Celsius float64   `json:"celsius"`
	At      time.Time `json:"at"`
	Source  string    `json:"source"`
}
type doctorTemperaturePoint struct {
	Celsius float64   `json:"celsius"`
	At      time.Time `json:"at"`
}
type doctorSmartBaseline struct {
	Counters map[string]uint64 `json:"counters"`
	Health   string            `json:"health"`
}
type doctorHistoryDocument struct {
	Version        int                                 `json:"version"`
	Events         []doctorHealthEvent                 `json:"events"`
	Seen           []string                            `json:"seen"`
	Temperatures   map[string][]doctorTemperaturePoint `json:"temperatures"`
	Smart          map[string]doctorSmartBaseline      `json:"smart"`
	DiskIdentities map[string]string                   `json:"disk_identities,omitempty"`
}

var doctorHistory = struct {
	sync.Mutex
	loaded          bool
	document        doctorHistoryDocument
	lastKernel      time.Time
	kernelAvailable bool
	lastReadings    map[string]doctorTemperatureReading
}{lastReadings: map[string]doctorTemperatureReading{}}

func doctorHistoryLoadLocked() {
	if doctorHistory.loaded {
		return
	}
	doctorHistory.loaded = true
	doctorHistory.document = doctorHistoryDocument{Version: 1, Events: []doctorHealthEvent{}, Seen: []string{}, Temperatures: map[string][]doctorTemperaturePoint{}, Smart: map[string]doctorSmartBaseline{}, DiskIdentities: map[string]string{}}
	info, err := os.Stat(doctorHistoryFile)
	if err != nil || info.Size() > 1024*1024 || info.Size() < 0 {
		return
	}
	content, err := os.ReadFile(doctorHistoryFile)
	if err != nil {
		return
	}
	var loaded doctorHistoryDocument
	if json.Unmarshal(content, &loaded) != nil || loaded.Version != 1 {
		return
	}
	if len(loaded.Events) > doctorHistoryLimit {
		loaded.Events = loaded.Events[:doctorHistoryLimit]
	}
	if len(loaded.Seen) > 512 {
		loaded.Seen = loaded.Seen[len(loaded.Seen)-512:]
	}
	if loaded.Temperatures == nil {
		loaded.Temperatures = map[string][]doctorTemperaturePoint{}
	}
	if loaded.Smart == nil {
		loaded.Smart = map[string]doctorSmartBaseline{}
	}
	if loaded.DiskIdentities == nil {
		loaded.DiskIdentities = map[string]string{}
	}
	if len(loaded.DiskIdentities) > 32 {
		loaded.DiskIdentities = map[string]string{}
	}
	// Bound untrusted persisted JSON after decode; only devices with simple disk IDs are retained.
	for device, points := range loaded.Temperatures {
		name := strings.TrimPrefix(device, "/dev/")
		if !doctorDeviceName.MatchString(name) {
			delete(loaded.Temperatures, device)
			continue
		}
		if len(points) > 144 {
			loaded.Temperatures[device] = points[len(points)-144:]
		}
	}
	if len(loaded.Temperatures) > 16 {
		loaded.Temperatures = map[string][]doctorTemperaturePoint{}
	}
	if len(loaded.Smart) > 16 {
		loaded.Smart = map[string]doctorSmartBaseline{}
	}
	doctorHistory.document = loaded
}

func doctorHistorySaveLocked(canWrite bool) {
	if !canWrite {
		return
	}
	dir := filepath.Dir(doctorHistoryFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	data, err := json.Marshal(doctorHistory.document)
	if err != nil || len(data) > 1024*1024 {
		return
	}
	_ = safety.WriteFileAtomic(doctorHistoryFile, data, 0600)
}
func doctorHistoryAddLocked(event doctorHealthEvent) bool {
	if event.ID == "" {
		return false
	}
	for _, key := range doctorHistory.document.Seen {
		if key == event.ID {
			return false
		}
	}
	doctorHistory.document.Seen = append(doctorHistory.document.Seen, event.ID)
	if len(doctorHistory.document.Seen) > 512 {
		doctorHistory.document.Seen = doctorHistory.document.Seen[len(doctorHistory.document.Seen)-512:]
	}
	doctorHistory.document.Events = append([]doctorHealthEvent{event}, doctorHistory.document.Events...)
	if len(doctorHistory.document.Events) > doctorHistoryLimit {
		doctorHistory.document.Events = doctorHistory.document.Events[:doctorHistoryLimit]
	}
	return true
}

func doctorKernelKind(line string) (string, string, bool) {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "ext4-fs error"), strings.Contains(l, "ext4-fs warning"), strings.Contains(l, "jbd2") && strings.Contains(l, "error"):
		return "fs_error", "critical", true
	case strings.Contains(l, "remounting filesystem read-only"), strings.Contains(l, "aborting journal"):
		return "read_only", "critical", true
	case strings.Contains(l, "buffer i/o error"), strings.Contains(l, "i/o error"), strings.Contains(l, "blk_update_request") && strings.Contains(l, "error"), strings.Contains(l, "critical medium error"):
		return "io_error", "critical", true
	case strings.Contains(l, "usb") && strings.Contains(l, "disconnect"):
		return "usb_disconnect", "warning", true
	case strings.Contains(l, "reset") && strings.Contains(l, "usb device"):
		return "usb_reset", "warning", true
	default:
		return "", "", false
	}
}

var doctorKernelDevicePattern = regexp.MustCompile(`\b(?:sd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(?:p[0-9]+)?)\b`)

func doctorKernelEvents(raw string, bootID string, observed time.Time) []doctorHealthEvent {
	events := []doctorHealthEvent{}
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 1024), 4096)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		kind, severity, ok := doctorKernelKind(line)
		if !ok {
			continue
		}
		if len(line) > 280 {
			line = line[:280]
		}
		// IDs bind to boot to avoid duplicate entries on restart or 60-second rescans.
		sum := sha256.Sum256([]byte(bootID + "\x00" + line))
		event := doctorHealthEvent{ID: "kernel:" + hex.EncodeToString(sum[:12]), At: observed, Kind: kind, Severity: severity, Device: doctorKernelDevicePattern.FindString(line), Summary: kind, Detail: line, Origin: "kernel"}
		events = append(events, event)
	}
	return events
}
func doctorKernelRead(now time.Time) ([]doctorHealthEvent, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Bounded read-only command. Never run fsck, mount, smartctl, or SMART self-tests.
	out, err := safety.RunCommand(ctx, 512*1024, "dmesg")
	if err != nil || ctx.Err() != nil || len(out) == 0 {
		return nil, false
	}
	bootID := strings.TrimSpace(readTrimmed("/proc/sys/kernel/random/boot_id"))
	return doctorKernelEvents(string(out), bootID, now), true
}

func doctorTemperatureForDisk(name string, now time.Time) (doctorTemperatureReading, bool) {
	paths := []string{
		"/sys/block/" + name + "/device/hwmon/hwmon*/temp*_input",
		"/sys/block/" + name + "/device/device/hwmon/hwmon*/temp*_input",
	}
	// Prefer device-addressed hwmon; do not misattribute generic SoC/board sensors.
	for _, pattern := range paths {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			temp, ok := readTemperature(path)
			if ok {
				return doctorTemperatureReading{Celsius: temp, At: now, Source: "hwmon"}, true
			}
		}
	}
	doctorSmartState.Lock()
	defer doctorSmartState.Unlock()
	for key, cached := range doctorSmartState.cache {
		if !strings.HasPrefix(key, doctorSmartIdentity(name)+"/") || now.Sub(cached.At) > 5*time.Minute {
			continue
		}
		if cached.Result.Status == "available" && cached.Result.Temperature != nil {
			return doctorTemperatureReading{Celsius: *cached.Result.Temperature, At: cached.At, Source: "SMART (manual)"}, true
		}
	}
	return doctorTemperatureReading{}, false
}

// Track the last physical identity observed at each Linux device path.
// A replacement at /dev/sdb should not inherit the previous disk's samples.
func doctorHistoryBindDiskIdentity(doc *doctorHistoryDocument, device, identity string) bool {
	if doc == nil || device == "" || identity == "" {
		return false
	}
	if doc.DiskIdentities == nil {
		doc.DiskIdentities = map[string]string{}
	}
	previous := doc.DiskIdentities[device]
	if previous == identity {
		return false
	}
	if previous != "" {
		delete(doc.Temperatures, device)
	}
	doc.DiskIdentities[device] = identity
	return true
}

func doctorHistoryAugment(view *doctorView, now time.Time) {
	doctorHistory.Lock()
	doctorHistoryLoadLocked()
	scanKernel := now.Sub(doctorHistory.lastKernel) >= doctorKernelScanInterval
	if scanKernel {
		doctorHistory.lastKernel = now
	}
	doctorHistory.Unlock()
	var kernelEvents []doctorHealthEvent
	var kernelOK bool
	if scanKernel {
		kernelEvents, kernelOK = doctorKernelRead(now)
	}
	// Read volatile mount ring without holding history lock.
	doctorTracker.Lock()
	observedMounts := append([]doctorIncident(nil), doctorTracker.events...)
	doctorTracker.Unlock()
	readings := make(map[string]doctorTemperatureReading, len(view.Disks))
	for _, disk := range view.Disks {
		name := strings.TrimPrefix(disk.Device, "/dev/")
		if temp, ok := doctorTemperatureForDisk(name, now); ok {
			readings[disk.Device] = temp
		}
	}
	doctorHistory.Lock()
	defer doctorHistory.Unlock()
	doctorHistoryLoadLocked()
	dirty := false
	for _, disk := range view.Disks {
		name := strings.TrimPrefix(disk.Device, "/dev/")
		if doctorHistoryBindDiskIdentity(&doctorHistory.document, disk.Device, doctorSmartIdentity(name)) {
			dirty = true
		}
	}
	for _, m := range observedMounts {
		event := doctorHealthEvent{ID: "mount:" + m.At.UTC().Format(time.RFC3339Nano) + ":" + m.Kind + ":" + m.Mount, At: m.At, Kind: m.Kind, Severity: "warning", Device: m.Mount, Summary: m.Kind, Detail: m.Detail, Origin: "mount"}
		if m.Kind == "appeared" || m.Kind == "writable_again" {
			event.Severity = "info"
			event.Recovery = true
		}
		if m.Kind == "read_only" {
			event.Severity = "critical"
		}
		if doctorHistoryAddLocked(event) {
			dirty = true
		}
	}
	if scanKernel {
		doctorHistory.kernelAvailable = kernelOK
		for _, event := range kernelEvents {
			if doctorHistoryAddLocked(event) {
				dirty = true
			}
		}
	}
	for dev, temp := range readings {
		doctorHistory.lastReadings[dev] = temp
		points := doctorHistory.document.Temperatures[dev]
		if len(points) == 0 || now.Sub(points[len(points)-1].At) >= 10*time.Minute {
			cutoff := now.Add(-24 * time.Hour)
			kept := points[:0]
			for _, p := range points {
				if p.At.After(cutoff) {
					kept = append(kept, p)
				}
			}
			points = kept
			points = append(points, doctorTemperaturePoint{Celsius: temp.Celsius, At: temp.At})
			if len(points) > 144 {
				points = points[len(points)-144:]
			}
			doctorHistory.document.Temperatures[dev] = points
			dirty = true
		}
		if temp.Celsius >= 65 {
			// A warning or recovery is a state transition, not one event per poll.
			severity := "warning"
			if temp.Celsius >= 75 {
				severity = "critical"
			}
			stateKey := "temp-state:" + dev
			previous := doctorHistory.document.Smart[stateKey].Health
			if previous != severity {
				doctorHistory.document.Smart[stateKey] = doctorSmartBaseline{Health: severity}
				event := doctorHealthEvent{ID: fmt.Sprintf("temp:%s:%s:%d", dev, severity, now.Unix()), At: now, Kind: "overheat", Severity: severity, Device: dev, Summary: "overheat", Detail: fmt.Sprintf("%.1f C", temp.Celsius), Origin: "thermal"}
				if doctorHistoryAddLocked(event) {
					dirty = true
				}
				dirty = true
			}
		} else {
			stateKey := "temp-state:" + dev
			previous := doctorHistory.document.Smart[stateKey].Health
			if previous != "" && previous != "normal" {
				doctorHistory.document.Smart[stateKey] = doctorSmartBaseline{Health: "normal"}
				event := doctorHealthEvent{ID: fmt.Sprintf("temp:%s:normal:%d", dev, now.Unix()), At: now, Kind: "cooled", Severity: "info", Device: dev, Summary: "cooled", Detail: fmt.Sprintf("%.1f C", temp.Celsius), Recovery: true, Origin: "thermal"}
				if doctorHistoryAddLocked(event) {
					dirty = true
				}
				dirty = true
			}
		}
	}
	view.History = append([]doctorHealthEvent(nil), doctorHistory.document.Events...)
	if view.History == nil {
		view.History = []doctorHealthEvent{}
	}
	view.Temperatures = readings
	view.TemperatureHistory = map[string][]doctorTemperaturePoint{}
	for dev, points := range doctorHistory.document.Temperatures {
		view.TemperatureHistory[dev] = append([]doctorTemperaturePoint(nil), points...)
	}
	view.KernelHistoryAvailable = doctorHistory.kernelAvailable
	view.HistoryScope = "best_effort_persistent_observation"
	if !view.Entware.Mounted || !view.Entware.Writable {
		view.HistoryScope = "memory_only_entware_not_writable"
	}
	if dirty {
		doctorHistorySaveLocked(view.Entware.Mounted && view.Entware.Writable)
	}
}

func doctorRememberSMART(res doctorSmartResult) {
	if res.Status != "available" {
		return
	}
	counters := map[string]uint64{}
	for _, a := range res.Attributes {
		if a.ID != 5 && a.ID != 197 && a.ID != 198 {
			continue
		}
		var raw struct {
			Value uint64 `json:"value"`
		}
		if json.Unmarshal(a.Raw, &raw) == nil {
			counters[fmt.Sprintf("ata_%d", a.ID)] = raw.Value
		}
	}
	// NVMe health counters are exposed as JSON float64, bounded to integer range.
	for _, key := range []string{"media_errors", "num_err_log_entries", "percentage_used"} {
		if value, ok := res.NVMe[key].(float64); ok && value >= 0 && value < 1e15 {
			counters[key] = uint64(value)
		}
	}
	device := res.Device
	baselineID := "smart:" + doctorSmartIdentity(strings.TrimPrefix(device, "/dev/"))
	doctorHistory.Lock()
	defer doctorHistory.Unlock()
	doctorHistoryLoadLocked()
	previous, exists := doctorHistory.document.Smart[baselineID]
	dirty := false
	if exists {
		for key, value := range counters {
			if before, ok := previous.Counters[key]; ok && value > before {
				e := doctorHealthEvent{ID: fmt.Sprintf("smart:%s:%s:%d", device, key, value), At: res.CheckedAt, Kind: "smart_degradation", Severity: "warning", Device: device, Summary: "smart_degradation", Detail: fmt.Sprintf("%s increased from %d to %d", key, before, value), Origin: "SMART"}
				if doctorHistoryAddLocked(e) {
					dirty = true
				}
			}
		}
	}
	if res.Health == "critical" && (!exists || previous.Health != "critical") {
		e := doctorHealthEvent{ID: fmt.Sprintf("smart:%s:critical:%d", device, res.CheckedAt.Unix()), At: res.CheckedAt, Kind: "smart_critical", Severity: "critical", Device: device, Summary: "smart_critical", Origin: "SMART"}
		if doctorHistoryAddLocked(e) {
			dirty = true
		}
	}
	doctorHistory.document.Smart[baselineID] = doctorSmartBaseline{Counters: counters, Health: res.Health}
	dirty = true
	if res.Temperature != nil {
		points := doctorHistory.document.Temperatures[device]
		if len(points) == 0 || res.CheckedAt.Sub(points[len(points)-1].At) >= 10*time.Minute {
			points = append(points, doctorTemperaturePoint{Celsius: *res.Temperature, At: res.CheckedAt})
			if len(points) > 144 {
				points = points[len(points)-144:]
			}
			doctorHistory.document.Temperatures[device] = points
			dirty = true
		}
	}
	if dirty {
		doctorHistorySaveLocked(doctorExistsDir("/opt/etc/init.d"))
	}
}

func doctorBackground(stop <-chan struct{}) {
	// Start collecting without a user visiting the screen; no automatic SMART I/O.
	_ = doctorSnapshot(time.Now())
	ticker := time.NewTicker(doctorKernelScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			_ = doctorSnapshot(now)
		}
	}
}
