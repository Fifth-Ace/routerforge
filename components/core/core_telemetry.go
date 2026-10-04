package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type coreTel struct {
	CPUUsagePct           float64 `json:"cpu_usage_pct"`
	CPUUsageAvailable     bool    `json:"cpu_usage_available"`
	RAMUsagePct           float64 `json:"ram_usage_pct"`
	RAMUsageAvailable     bool    `json:"ram_usage_available"`
	CPUTempC              float64 `json:"cpu_temp_c"`
	CPUTempAvailable      bool    `json:"cpu_temp_available"`
	ProcessCount          int     `json:"process_count"`
	ProcessCountAvailable bool    `json:"process_count_available"`
}

type coreCPUCounter struct {
	Idle   uint64
	IOWait uint64
	Total  uint64
}

type coreCPUUsageState struct {
	mu       sync.Mutex
	previous coreCPUCounter
	ready    bool
}

var coreCPUUsageSampler = newCoreCPUUsageState()

func newCoreCPUUsageState() *coreCPUUsageState {
	current, ok := readCoreCPUCounter()
	return &coreCPUUsageState{previous: current, ready: ok}
}

func (s *coreCPUUsageState) sample() (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := readCoreCPUCounter()
	if !ok {
		return 0, false
	}
	if !s.ready {
		s.previous = current
		s.ready = true
		return 0, false
	}

	previous := s.previous
	s.previous = current
	return coreCPUUsagePercent(previous, current)
}

func readCoreTelemetry() coreTel {
	cpuUsage, cpuOK := coreCPUUsageSampler.sample()
	ramUsage, ramOK := readCoreRAMUsage()
	cpuTemp, tempOK := readCoreCPUTemperature()
	processes, processesOK := readCoreProcessCount()

	return coreTel{
		CPUUsagePct:           cpuUsage,
		CPUUsageAvailable:     cpuOK,
		RAMUsagePct:           ramUsage,
		RAMUsageAvailable:     ramOK,
		CPUTempC:              cpuTemp,
		CPUTempAvailable:      tempOK,
		ProcessCount:          processes,
		ProcessCountAvailable: processesOK,
	}
}

func readCoreCPUCounter() (coreCPUCounter, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return coreCPUCounter{}, false
	}
	return parseCoreCPUCounter(string(data))
}

func parseCoreCPUCounter(raw string) (coreCPUCounter, bool) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}

		values := make([]uint64, 8)
		for i := 1; i < len(fields) && i <= 8; i++ {
			value, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return coreCPUCounter{}, false
			}
			values[i-1] = value
		}

		total := uint64(0)
		for _, value := range values {
			total += value
		}
		if total == 0 {
			return coreCPUCounter{}, false
		}
		return coreCPUCounter{Idle: values[3], IOWait: values[4], Total: total}, true
	}
	return coreCPUCounter{}, false
}

func coreCPUUsagePercent(previous, current coreCPUCounter) (float64, bool) {
	if previous.Total == 0 || current.Total <= previous.Total {
		return 0, false
	}
	totalDelta := current.Total - previous.Total
	previousIdle := previous.Idle + previous.IOWait
	currentIdle := current.Idle + current.IOWait
	if currentIdle < previousIdle {
		return 0, false
	}
	idleDelta := currentIdle - previousIdle
	if idleDelta >= totalDelta {
		return 0, true
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100, true
}

func readCoreRAMUsage() (float64, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	return parseCoreRAMUsage(string(data))
}

func parseCoreRAMUsage(raw string) (float64, bool) {
	values := map[string]int64{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = value
	}

	total := values["MemTotal"]
	if total <= 0 {
		return 0, false
	}
	available := values["MemAvailable"]
	used := int64(0)
	if available > 0 {
		used = total - available
	} else {
		used = total - values["MemFree"] - values["Buffers"] - values["Cached"] - values["SReclaimable"]
	}
	if used < 0 {
		used = 0
	}
	return float64(used) / float64(total) * 100, true
}

func readCoreProcessCount() (int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, false
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			count++
		}
	}
	return count, true
}

func readCoreCPUTemperature() (float64, bool) {
	var thermalZoneTemps []float64
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, zone := range zones {
		if value, ok := readCoreTemperature(filepath.Join(zone, "temp")); ok {
			thermalZoneTemps = append(thermalZoneTemps, value)
		}
	}
	if value, ok := maxCoreTemperature(thermalZoneTemps); ok {
		return value, true
	}

	var preferred []float64
	var fallback []float64
	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, hwmon := range hwmons {
		name := strings.ToLower(readPlatformText(filepath.Join(hwmon, "name")))
		inputs, _ := filepath.Glob(filepath.Join(hwmon, "temp*_input"))
		for _, input := range inputs {
			value, ok := readCoreTemperature(input)
			if !ok {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(input), "_input")
			label := strings.ToLower(readPlatformText(filepath.Join(hwmon, base+"_label")))
			identity := name + " " + label
			if strings.Contains(identity, "cpu") || strings.Contains(identity, "soc") || strings.Contains(identity, "mcusys") {
				preferred = append(preferred, value)
			} else {
				fallback = append(fallback, value)
			}
		}
	}
	if value, ok := maxCoreTemperature(preferred); ok {
		return value, true
	}
	return maxCoreTemperature(fallback)
}

func readCoreTemperature(path string) (float64, bool) {
	raw := readPlatformText(path)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return normalizeCoreTemperature(value)
}

func normalizeCoreTemperature(value float64) (float64, bool) {
	absolute := value
	if absolute < 0 {
		absolute = -absolute
	}
	if absolute > 200 {
		value /= 1000
	}
	if value < -50 || value > 200 {
		return 0, false
	}
	return value, true
}

func maxCoreTemperature(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max, true
}
