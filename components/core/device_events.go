package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

const (
	deviceObserverInterval        = 15 * time.Second
	deviceStorageWarningPct       = 80.0
	deviceStorageCriticalPct      = 90.0
	deviceThermalWarningC         = 80.0
	deviceThermalCriticalC        = 90.0
	deviceObserverModuleTimeout   = 3 * time.Second
)

type deviceProblem struct {
	ID        string
	Component string
	Severity  platformevents.Severity
	Message   string
	Since     time.Time
	Context   map[string]any
}

type deviceObserverRuntime struct {
	mu                sync.RWMutex
	network           map[string]bool
	networkSeenUp     map[string]bool
	storage           map[string]string
	thermal           map[string]string
	watchdogLast      map[string]time.Time
	active            map[string]deviceProblem
	started           bool
}

var deviceObserver = deviceObserverRuntime{
	network:       map[string]bool{},
	networkSeenUp: map[string]bool{},
	storage:       map[string]string{},
	thermal:       map[string]string{},
	watchdogLast:  map[string]time.Time{},
	active:        map[string]deviceProblem{},
}

type deviceStorageSample struct {
	Device    string
	Mount     string
	FSType    string
	UsedPct   float64
	FreeBytes uint64
	Total     uint64
}

type deviceThermalSample struct {
	ID    string
	Name  string
	TempC float64
}

type deviceWatchdogEnvelope struct {
	Watchdogs []struct {
		ID                string    `json:"id"`
		Name              string    `json:"name"`
		Enabled           bool      `json:"enabled"`
		Detected          bool      `json:"detected"`
		Running           bool      `json:"running"`
		ServiceID         string    `json:"service_id"`
		LastAttemptAt     time.Time `json:"last_attempt_at"`
		LastAttemptOK     bool      `json:"last_attempt_ok"`
	} `json:"watchdogs"`
}

func startDeviceEventObserver() {
	deviceObserver.mu.Lock()
	if deviceObserver.started {
		deviceObserver.mu.Unlock()
		return
	}
	deviceObserver.started = true
	deviceObserver.mu.Unlock()

	deviceObserver.observe(time.Now().UTC())

	go func() {
		ticker := time.NewTicker(deviceObserverInterval)
		defer ticker.Stop()
		for now := range ticker.C {
			deviceObserver.observe(now.UTC())
		}
	}()
}

func (runtime *deviceObserverRuntime) observe(now time.Time) {
	runtime.observeNetwork(now, readDeviceNetworkStates())
	runtime.observeStorage(now, readDeviceStorageSamples())
	runtime.observeThermal(now, readDeviceThermalSamples())
	runtime.observeWatchdogs(now, readDeviceWatchdogs())
}

func (runtime *deviceObserverRuntime) observeNetwork(now time.Time, current map[string]bool) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	for name, up := range current {
		previous, known := runtime.network[name]
		if !known {
			runtime.network[name] = up
			if up {
				runtime.networkSeenUp[name] = true
			}
			continue
		}
		if previous == up {
			continue
		}

		runtime.network[name] = up
		id := "network:" + name
		if up {
			runtime.networkSeenUp[name] = true
			delete(runtime.active, id)
			emitPlatformEvent(platformevents.Event{
				Time:      now,
				Component: "network",
				Severity:  platformevents.Info,
				Type:      "device.network.recovered",
				Message:   "Network interface " + name + " is up",
				ObjectID:  name,
				Recovery:  true,
				Context:   map[string]any{"interface": name, "state": "up"},
			})
			continue
		}

		if !runtime.networkSeenUp[name] {
			continue
		}
		problem := deviceProblem{
			ID:        id,
			Component: "network",
			Severity:  platformevents.Warning,
			Message:   "Network interface " + name + " went down",
			Since:     now,
			Context:   map[string]any{"interface": name, "state": "down"},
		}
		runtime.active[id] = problem
		emitPlatformEvent(platformevents.Event{
			Time:      now,
			Component: "network",
			Severity:  platformevents.Warning,
			Type:      "device.network.down",
			Message:   problem.Message,
			ObjectID:  name,
			Context:   problem.Context,
		})
	}

	for name := range runtime.network {
		if _, exists := current[name]; exists {
			continue
		}
		delete(runtime.network, name)
		if !runtime.networkSeenUp[name] {
			continue
		}
		id := "network:" + name
		problem := deviceProblem{
			ID:        id,
			Component: "network",
			Severity:  platformevents.Warning,
			Message:   "Network interface " + name + " disappeared",
			Since:     now,
			Context:   map[string]any{"interface": name, "state": "missing"},
		}
		runtime.active[id] = problem
		emitPlatformEvent(platformevents.Event{
			Time:      now,
			Component: "network",
			Severity:  platformevents.Warning,
			Type:      "device.network.missing",
			Message:   problem.Message,
			ObjectID:  name,
			Context:   problem.Context,
		})
	}
}

func (runtime *deviceObserverRuntime) observeStorage(now time.Time, samples []deviceStorageSample) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	seen := map[string]bool{}
	for _, sample := range samples {
		id := "storage:" + sample.Mount
		seen[id] = true
		state := classifyDeviceStorage(sample.UsedPct)
		previous, known := runtime.storage[id]
		runtime.storage[id] = state

		if state == "ok" {
			if known && previous != "ok" {
				delete(runtime.active, id)
				emitPlatformEvent(platformevents.Event{
					Time:      now,
					Component: "storage",
					Severity:  platformevents.Info,
					Type:      "device.storage.recovered",
					Message:   "Storage " + sample.Mount + " returned to normal",
					ObjectID:  sample.Mount,
					Recovery:  true,
					Context: map[string]any{
						"mount": sample.Mount, "device": sample.Device, "fs_type": sample.FSType,
						"used_pct": sample.UsedPct, "free_bytes": sample.FreeBytes,
					},
				})
			}
			continue
		}

		if known && previous == state {
			continue
		}
		severity := platformevents.Warning
		if state == "critical" {
			severity = platformevents.Critical
		}
		message := "Storage " + sample.Mount + " is low on free space"
		problem := deviceProblem{
			ID:        id,
			Component: "storage",
			Severity:  severity,
			Message:   message,
			Since:     now,
			Context: map[string]any{
				"mount": sample.Mount, "device": sample.Device, "fs_type": sample.FSType,
				"used_pct": sample.UsedPct, "free_bytes": sample.FreeBytes,
			},
		}
		runtime.active[id] = problem
		emitPlatformEvent(platformevents.Event{
			Time:      now,
			Component: "storage",
			Severity:  severity,
			Type:      "device.storage." + state,
			Message:   message,
			ObjectID:  sample.Mount,
			Context:   problem.Context,
		})
	}

	for id := range runtime.storage {
		if seen[id] {
			continue
		}
		delete(runtime.storage, id)
		delete(runtime.active, id)
	}
}

func (runtime *deviceObserverRuntime) observeThermal(now time.Time, samples []deviceThermalSample) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	seen := map[string]bool{}
	for _, sample := range samples {
		id := "thermal:" + sample.ID
		seen[id] = true
		state := classifyDeviceThermal(sample.TempC)
		previous, known := runtime.thermal[id]
		runtime.thermal[id] = state

		if state == "ok" {
			if known && previous != "ok" {
				delete(runtime.active, id)
				emitPlatformEvent(platformevents.Event{
					Time:      now,
					Component: "thermal",
					Severity:  platformevents.Info,
					Type:      "device.thermal.recovered",
					Message:   "Temperature " + sample.Name + " returned to normal",
					ObjectID:  sample.ID,
					Recovery:  true,
					Context:   map[string]any{"sensor": sample.Name, "temp_c": sample.TempC},
				})
			}
			continue
		}

		if known && previous == state {
			continue
		}
		severity := platformevents.Warning
		if state == "critical" {
			severity = platformevents.Critical
		}
		message := "High temperature on " + sample.Name
		problem := deviceProblem{
			ID:        id,
			Component: "thermal",
			Severity:  severity,
			Message:   message,
			Since:     now,
			Context:   map[string]any{"sensor": sample.Name, "temp_c": sample.TempC},
		}
		runtime.active[id] = problem
		emitPlatformEvent(platformevents.Event{
			Time:      now,
			Component: "thermal",
			Severity:  severity,
			Type:      "device.thermal." + state,
			Message:   message,
			ObjectID:  sample.ID,
			Context:   problem.Context,
		})
	}

	for id := range runtime.thermal {
		if seen[id] {
			continue
		}
		delete(runtime.thermal, id)
		delete(runtime.active, id)
	}
}

func (runtime *deviceObserverRuntime) observeWatchdogs(now time.Time, envelope deviceWatchdogEnvelope) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	for _, item := range envelope.Watchdogs {
		if item.LastAttemptAt.IsZero() {
			continue
		}
		previous, known := runtime.watchdogLast[item.ID]
		if known && !item.LastAttemptAt.After(previous) {
			continue
		}
		runtime.watchdogLast[item.ID] = item.LastAttemptAt
		if !known {
			continue
		}

		id := "watchdog:" + item.ID
		context := map[string]any{
			"watchdog": item.ID,
			"name": item.Name,
			"service_id": item.ServiceID,
		}
		if item.LastAttemptOK {
			delete(runtime.active, id)
			emitPlatformEvent(platformevents.Event{
				Time:      item.LastAttemptAt,
				Component: "watchdog",
				Severity:  platformevents.Info,
				Type:      "device.watchdog.recovered",
				Message:   "Watchdog recovered " + item.Name,
				ObjectID:  item.ID,
				Recovery:  true,
				Context:   context,
			})
			continue
		}

		problem := deviceProblem{
			ID:        id,
			Component: "watchdog",
			Severity:  platformevents.Warning,
			Message:   "Watchdog could not recover " + item.Name,
			Since:     item.LastAttemptAt,
			Context:   context,
		}
		runtime.active[id] = problem
		emitPlatformEvent(platformevents.Event{
			Time:      item.LastAttemptAt,
			Component: "watchdog",
			Severity:  platformevents.Warning,
			Type:      "device.watchdog.failed",
			Message:   problem.Message,
			ObjectID:  item.ID,
			Context:   context,
		})
	}
}

func snapshotDeviceHealthAlerts(now time.Time) []healthAlert {
	deviceObserver.mu.RLock()
	items := make([]deviceProblem, 0, len(deviceObserver.active))
	for _, item := range deviceObserver.active {
		items = append(items, item)
	}
	deviceObserver.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		if items[i].Severity != items[j].Severity {
			return items[i].Severity == platformevents.Critical
		}
		return items[i].ID < items[j].ID
	})

	alerts := make([]healthAlert, 0, len(items))
	for _, item := range items {
		age := int64(now.Sub(item.Since).Seconds())
		if age < 0 {
			age = 0
		}
		alerts = append(alerts, healthAlert{
			ID:         item.ID,
			Component:  item.Component,
			Severity:   item.Severity,
			State:      "active",
			Message:    item.Message,
			Since:      item.Since,
			AgeSeconds: age,
			Context:    item.Context,
		})
	}
	return alerts
}

func readDeviceNetworkStates() map[string]bool {
	result := map[string]bool{}
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return result
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" || strings.TrimSpace(name) == "" {
			continue
		}
		content, err := os.ReadFile(filepath.Join("/sys/class/net", name, "operstate"))
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(content)) {
		case "up":
			result[name] = true
		case "down", "lowerlayerdown":
			result[name] = false
		}
	}
	return result
}

func readDeviceStorageSamples() []deviceStorageSample {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer file.Close()

	byDevice := map[string]deviceStorageSample{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		device := decodeProcMountField(fields[0])
		mount := decodeProcMountField(fields[1])
		fsType := fields[2]
		options := fields[3]
		if !deviceStorageFilesystemMonitored(fsType, options) {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil || stat.Blocks == 0 {
			continue
		}
		blockSize := uint64(stat.Bsize)
		total := stat.Blocks * blockSize
		free := stat.Bavail * blockSize
		used := total - free
		usedPct := float64(used) * 100 / float64(total)

		sample := deviceStorageSample{
			Device: device, Mount: mount, FSType: fsType,
			UsedPct: usedPct, FreeBytes: free, Total: total,
		}
		key := device
		if key == "" || key == "none" {
			key = mount
		}
		existing, exists := byDevice[key]
		if !exists || preferDeviceStorageMount(sample.Mount, existing.Mount) {
			byDevice[key] = sample
		}
	}

	result := make([]deviceStorageSample, 0, len(byDevice))
	for _, sample := range byDevice {
		result = append(result, sample)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Mount < result[j].Mount })
	return result
}

func deviceStorageFilesystemMonitored(fsType, options string) bool {
	fsType = strings.ToLower(strings.TrimSpace(fsType))
	for _, ignored := range []string{
		"squashfs", "proc", "sysfs", "devpts", "debugfs", "tmpfs", "devtmpfs",
		"ramfs", "overlay", "cgroup", "cgroup2", "pstore", "securityfs", "tracefs",
		"configfs", "fusectl", "mqueue", "autofs", "bpf",
	} {
		if fsType == ignored {
			return false
		}
	}
	for _, option := range strings.Split(options, ",") {
		if option == "ro" {
			return false
		}
	}
	return true
}

func preferDeviceStorageMount(candidate, existing string) bool {
	priority := func(value string) int {
		switch value {
		case "/opt":
			return 0
		case "/storage":
			return 1
		case "/":
			return 2
		default:
			return 10 + strings.Count(strings.Trim(value, "/"), "/")
		}
	}
	pc, pe := priority(candidate), priority(existing)
	if pc != pe {
		return pc < pe
	}
	if len(candidate) != len(existing) {
		return len(candidate) < len(existing)
	}
	return candidate < existing
}

func decodeProcMountField(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(value)
}

func classifyDeviceStorage(usedPct float64) string {
	switch {
	case usedPct >= deviceStorageCriticalPct:
		return "critical"
	case usedPct >= deviceStorageWarningPct:
		return "warning"
	default:
		return "ok"
	}
}

func readDeviceThermalSamples() []deviceThermalSample {
	paths, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	result := make([]deviceThermalSample, 0, len(paths))
	for _, path := range paths {
		rawTemp, err := os.ReadFile(filepath.Join(path, "temp"))
		if err != nil {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(string(rawTemp)), 64)
		if err != nil {
			continue
		}
		if value > 1000 {
			value /= 1000
		}
		name := filepath.Base(path)
		if rawType, err := os.ReadFile(filepath.Join(path, "type")); err == nil {
			if candidate := strings.TrimSpace(string(rawType)); candidate != "" {
				name = candidate
			}
		}
		result = append(result, deviceThermalSample{
			ID: filepath.Base(path), Name: name, TempC: value,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func classifyDeviceThermal(tempC float64) string {
	switch {
	case tempC >= deviceThermalCriticalC:
		return "critical"
	case tempC >= deviceThermalWarningC:
		return "warning"
	default:
		return "ok"
	}
}

func readDeviceWatchdogs() deviceWatchdogEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), deviceObserverModuleTimeout)
	defer cancel()
	raw, err := readModuleRaw(ctx, "admin", "/v1/maintenance/watchdogs", deviceObserverModuleTimeout)
	if err != nil {
		return deviceWatchdogEnvelope{}
	}
	var result deviceWatchdogEnvelope
	if json.Unmarshal(raw, &result) != nil {
		return deviceWatchdogEnvelope{}
	}
	return result
}
