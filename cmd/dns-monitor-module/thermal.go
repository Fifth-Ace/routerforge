package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type thermalSensor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Source    string    `json:"source"`
	TempC     float64   `json:"temp_c"`
	Status    string    `json:"status"`
	WarnC     float64   `json:"warn_c"`
	CriticalC float64   `json:"critical_c"`
	UpdatedAt time.Time `json:"updated_at"`
}

type thermalCollector struct {
	mu       sync.RWMutex
	interval time.Duration
	sensors  []thermalSensor
	scanned  time.Time
}

func newThermalCollector(interval time.Duration) *thermalCollector {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	t := &thermalCollector{interval: interval}
	t.refresh()
	return t
}

func (t *thermalCollector) snapshot() ([]thermalSensor, time.Time) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]thermalSensor(nil), t.sensors...), t.scanned
}

func (t *thermalCollector) maybeRefresh() {
	t.mu.RLock()
	stale := time.Since(t.scanned) >= t.interval
	t.mu.RUnlock()
	if stale {
		t.refresh()
	}
}

func (t *thermalCollector) refresh() {
	now := time.Now()
	sensors := collectThermalSensors(now)
	t.mu.Lock()
	t.sensors = sensors
	t.scanned = now
	t.mu.Unlock()
}

func (s *moduleServer) registerThermal(mux *http.ServeMux) {
	mux.HandleFunc("/v1/sensors", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		s.thermal.maybeRefresh()
		sensors, scanned := s.thermal.snapshot()
		writeJSON(w, http.StatusOK, map[string]any{
			"sensors":           sensors,
			"sensor_count":      len(sensors),
			"scanned_at":        scanned,
			"cache_seconds":     int(s.thermal.interval.Seconds()),
			"optional_smartctl": commandExists("smartctl"),
		})
	}))
}

func collectThermalSensors(now time.Time) []thermalSensor {
	var sensors []thermalSensor
	seen := map[string]bool{}

	add := func(sensor thermalSensor) {
		key := thermalDedupKey(sensor)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		sensors = append(sensors, sensor)
	}

	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, zone := range zones {
		tempPath := filepath.Join(zone, "temp")
		temp, ok := readTemperature(tempPath)
		if !ok {
			continue
		}
		name := firstNonEmpty(readTrimmed(filepath.Join(zone, "type")), filepath.Base(zone))
		category := thermalCategory(name, tempPath)
		add(makeThermalSensor(filepath.Base(zone), name, category, tempPath, temp, now))
	}

	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hwmon := range hwmons {
		hwName := firstNonEmpty(readTrimmed(filepath.Join(hwmon, "name")), filepath.Base(hwmon))
		inputs, _ := filepath.Glob(filepath.Join(hwmon, "temp*_input"))
		for _, input := range inputs {
			temp, ok := readTemperature(input)
			if !ok {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(input), "_input")
			label := readTrimmed(filepath.Join(hwmon, base+"_label"))
			name := hwName
			if label != "" {
				name += " · " + label
			} else {
				name += " · " + base
			}
			category := thermalCategory(name, input)
			add(makeThermalSensor(filepath.Base(hwmon)+"/"+base, name, category, input, temp, now))
		}
	}

	for _, sensor := range collectDebugFSThermals(now) {
		add(sensor)
	}
	for _, sensor := range collectSmartctlThermals(now) {
		add(sensor)
	}

	sort.Slice(sensors, func(i, j int) bool {
		if sensors[i].Category != sensors[j].Category {
			return thermalCategoryOrder(sensors[i].Category) < thermalCategoryOrder(sensors[j].Category)
		}
		if sensors[i].TempC != sensors[j].TempC {
			return sensors[i].TempC > sensors[j].TempC
		}
		return sensors[i].Name < sensors[j].Name
	})
	return sensors
}

func thermalDedupKey(sensor thermalSensor) string {
	source := sensor.Source
	if strings.HasPrefix(source, "/sys/") {
		if real, err := filepath.EvalSymlinks(source); err == nil {
			source = real
		}
		return "sys:" + source
	}
	return strings.ToLower(sensor.Category + ":" + sensor.Name + ":" + source)
}

func makeThermalSensor(id, name, category, source string, temp float64, now time.Time) thermalSensor {
	warn, critical := thermalThresholds(category)
	status := "ok"
	if temp >= critical {
		status = "critical"
	} else if temp >= warn {
		status = "warn"
	}
	return thermalSensor{
		ID: id, Name: name, Category: category, Source: source, TempC: temp,
		Status: status, WarnC: warn, CriticalC: critical, UpdatedAt: now,
	}
}

func thermalCategory(name, source string) string {
	text := strings.ToLower(name + " " + source)
	switch {
	case strings.Contains(text, "nvme"), strings.Contains(text, "ssd"),
		strings.Contains(text, "hdd"), strings.Contains(text, "disk"),
		strings.Contains(text, "ata"):
		return "storage"
	case strings.Contains(text, "wifi"), strings.Contains(text, "wlan"),
		strings.Contains(text, "phy"), strings.Contains(text, "radio"),
		strings.Contains(text, "mt76"), strings.Contains(text, "ath"):
		return "wifi"
	case strings.Contains(text, "pmic"), strings.Contains(text, "board"),
		strings.Contains(text, "pcb"), strings.Contains(text, "ambient"),
		strings.Contains(text, "switch"):
		return "board"
	case strings.Contains(text, "cpu"), strings.Contains(text, "soc"),
		strings.Contains(text, "thermal_zone"), strings.Contains(text, "cpu-thermal"):
		return "soc"
	default:
		return "other"
	}
}

func thermalThresholds(category string) (float64, float64) {
	switch category {
	case "storage":
		return 55, 65
	case "wifi":
		return 75, 90
	case "soc":
		return 75, 90
	case "board":
		return 70, 85
	default:
		return 70, 85
	}
}

func thermalCategoryOrder(category string) int {
	switch category {
	case "soc":
		return 1
	case "wifi":
		return 2
	case "board":
		return 3
	case "storage":
		return 4
	default:
		return 5
	}
}

func readTemperature(path string) (float64, bool) {
	raw := readTrimmed(path)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return normalizeTemperature(value)
}

func normalizeTemperature(value float64) (float64, bool) {
	abs := value
	if abs < 0 {
		abs = -abs
	}
	if abs > 200 {
		value /= 1000
	}
	if value < -50 || value > 200 {
		return 0, false
	}
	return value, true
}

func collectDebugFSThermals(now time.Time) []thermalSensor {
	root := "/sys/kernel/debug/ieee80211"
	var out []thermalSensor
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := strings.ToLower(info.Name())
		if !strings.Contains(base, "temp") && !strings.Contains(base, "thermal") {
			return nil
		}
		if info.Size() > 1024 {
			return nil
		}

		raw := strings.TrimSpace(string(readBytes(path)))
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			return nil
		}
		for _, field := range fields {
			clean := strings.Trim(field, "°Cc:=,")
			value, parseErr := strconv.ParseFloat(clean, 64)
			if parseErr != nil {
				continue
			}
			temp, ok := normalizeTemperature(value)
			if !ok {
				continue
			}
			name := strings.TrimPrefix(path, root+"/")
			out = append(out, makeThermalSensor(
				"debugfs:"+name, "Wi-Fi · "+name, "wifi", path, temp, now,
			))
			break
		}
		return nil
	})
	return out
}

var smartTemperatureRE = regexp.MustCompile(`(?i)(?:temperature(?: sensor)?)[^0-9-]*(-?[0-9]+(?:\.[0-9]+)?)`)

func collectSmartctlThermals(now time.Time) []thermalSensor {
	if !commandExists("smartctl") {
		return nil
	}

	var devices []string
	for _, pattern := range []string{"/dev/sd?", "/dev/nvme?n1"} {
		matches, _ := filepath.Glob(pattern)
		devices = append(devices, matches...)
	}

	var out []thermalSensor
	for _, device := range devices {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, "smartctl", "-A", device)
		output, _ := cmd.CombinedOutput()
		cancel()
		if ctx.Err() != nil || len(output) == 0 {
			continue
		}
		temp, ok := parseSmartctlTemperature(string(output))
		if !ok {
			continue
		}
		name := filepath.Base(device)
		out = append(out, makeThermalSensor(
			"smartctl:"+name, "Storage · "+name, "storage", "smartctl:"+device, temp, now,
		))
	}
	return out
}

func parseSmartctlTemperature(output string) (float64, bool) {
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := sc.Text()
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "temp") {
			continue
		}

		fields := strings.Fields(line)
		if strings.Contains(lower, "temperature_celsius") && len(fields) > 0 {
			last := strings.Trim(fields[len(fields)-1], "°Cc")
			if value, err := strconv.ParseFloat(last, 64); err == nil {
				if normalized, ok := normalizeTemperature(value); ok {
					return normalized, true
				}
			}
		}

		match := smartTemperatureRE.FindStringSubmatch(line)
		if len(match) > 1 {
			value, err := strconv.ParseFloat(match[1], 64)
			if err == nil {
				if normalized, ok := normalizeTemperature(value); ok {
					return normalized, true
				}
			}
		}
	}
	return 0, false
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
