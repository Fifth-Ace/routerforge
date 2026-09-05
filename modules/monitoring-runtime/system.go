package main

import (
	"bufio"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type memoryInfo struct {
	TotalKB     int64   `json:"total_kb"`
	FreeKB      int64   `json:"free_kb"`
	AvailableKB int64   `json:"available_kb"`
	BuffersKB   int64   `json:"buffers_kb"`
	CachedKB    int64   `json:"cached_kb"`
	SwapTotalKB int64   `json:"swap_total_kb"`
	SwapFreeKB  int64   `json:"swap_free_kb"`
	UsedKB      int64   `json:"used_kb"`
	UsedPct     float64 `json:"used_pct"`
}

type cpuRaw struct {
	Name   string
	User   uint64
	Nice   uint64
	System uint64
	Idle   uint64
	IOWait uint64
	IRQ    uint64
	Soft   uint64
	Steal  uint64
	Total  uint64
}

type cpuSample struct {
	Name     string  `json:"name"`
	UsagePct float64 `json:"usage_pct"`
	User     uint64  `json:"user"`
	System   uint64  `json:"system"`
	Idle     uint64  `json:"idle"`
	Total    uint64  `json:"total"`
}

type systemCollector struct {
	mu        sync.RWMutex
	interval  time.Duration
	window    int
	previous  map[string]cpuRaw
	history   map[string][]float64
	latest    []cpuSample
	sampledAt time.Time
	ready     bool
	stop      chan struct{}
	done      chan struct{}
}

func newSystemCollector() (*systemCollector, error) {
	raw, err := readCPURaw()
	if err != nil {
		return nil, err
	}
	s := &systemCollector{
		interval: time.Second,
		window:   5,
		previous: make(map[string]cpuRaw, len(raw)),
		history:  make(map[string][]float64, len(raw)),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	for _, item := range raw {
		s.previous[item.Name] = item
	}
	go s.run()
	return s, nil
}

func (s *systemCollector) Close() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	<-s.done
}

func (s *systemCollector) run() {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.sample()
		}
	}
}

func (s *systemCollector) sample() {
	current, err := readCPURaw()
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	latest := make([]cpuSample, 0, len(current))
	for _, item := range current {
		prev, ok := s.previous[item.Name]
		s.previous[item.Name] = item
		if !ok || item.Total <= prev.Total {
			continue
		}

		usage := cpuUsage(prev, item)
		values := append(s.history[item.Name], usage)
		if len(values) > s.window {
			values = values[len(values)-s.window:]
		}
		s.history[item.Name] = values

		sum := 0.0
		for _, value := range values {
			sum += value
		}
		rolling := sum / float64(len(values))

		latest = append(latest, cpuSample{
			Name: item.Name, UsagePct: rolling, User: item.User,
			System: item.System, Idle: item.Idle, Total: item.Total,
		})
	}

	sort.Slice(latest, func(i, j int) bool { return latest[i].Name < latest[j].Name })
	if len(latest) > 0 {
		s.latest = latest
		s.sampledAt = time.Now()
		s.ready = true
	}
}

func (s *systemCollector) cpuSnapshot() ([]cpuSample, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]cpuSample(nil), s.latest...), s.sampledAt, s.ready
}

func (s *moduleServer) registerSystem(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		load := readLoadAverage()
		mem := readMemory()
		writeJSON(w, http.StatusOK, map[string]any{
			"generated_at":   time.Now(),
			"hostname":       firstNonEmpty(readTrimmed("/proc/sys/kernel/hostname"), hostname()),
			"kernel":         readTrimmed("/proc/sys/kernel/osrelease"),
			"architecture":   runtime.GOARCH,
			"uptime_seconds": readFloatField("/proc/uptime", 0),
			"load_1":         load[0],
			"load_5":         load[1],
			"load_15":        load[2],
			"cpu_count":      runtime.NumCPU(),
			"process_count":  processCount(),
			"memory":         mem,
			"mode":           "read-only",
		})
	}))

	mux.HandleFunc("/v1/cpu", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		cpus, sampledAt, ready := s.system.cpuSnapshot()
		writeJSON(w, http.StatusOK, map[string]any{
			"cpus":           cpus,
			"sampled_at":     sampledAt,
			"ready":          ready,
			"window_seconds": s.system.window,
		})
	}))

	mux.HandleFunc("/v1/memory", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, readMemory())
	}))
}

func readCPURaw() ([]cpuRaw, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []cpuRaw
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") || fields[0] == "cpu" {
			continue
		}
		nums := make([]uint64, 8)
		for i := 1; i < len(fields) && i <= 8; i++ {
			nums[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
		}
		raw := cpuRaw{
			Name: fields[0], User: nums[0], Nice: nums[1], System: nums[2],
			Idle: nums[3], IOWait: nums[4], IRQ: nums[5], Soft: nums[6], Steal: nums[7],
		}
		raw.Total = raw.User + raw.Nice + raw.System + raw.Idle + raw.IOWait + raw.IRQ + raw.Soft + raw.Steal
		out = append(out, raw)
	}
	return out, sc.Err()
}

func cpuUsage(prev, current cpuRaw) float64 {
	if current.Total <= prev.Total {
		return 0
	}
	totalDelta := current.Total - prev.Total
	prevIdle := prev.Idle + prev.IOWait
	currentIdle := current.Idle + current.IOWait
	if currentIdle < prevIdle {
		return 0
	}
	idleDelta := currentIdle - prevIdle
	if idleDelta >= totalDelta {
		return 0
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}

func readMemory() memoryInfo {
	values := map[string]int64{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return memoryInfo{}
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		n, _ := strconv.ParseInt(fields[1], 10, 64)
		values[key] = n
	}

	out := memoryInfo{
		TotalKB: values["MemTotal"], FreeKB: values["MemFree"], AvailableKB: values["MemAvailable"],
		BuffersKB: values["Buffers"], CachedKB: values["Cached"] + values["SReclaimable"],
		SwapTotalKB: values["SwapTotal"], SwapFreeKB: values["SwapFree"],
	}
	if out.AvailableKB > 0 {
		out.UsedKB = out.TotalKB - out.AvailableKB
	} else {
		out.UsedKB = out.TotalKB - out.FreeKB - out.BuffersKB - out.CachedKB
	}
	if out.UsedKB < 0 {
		out.UsedKB = 0
	}
	if out.TotalKB > 0 {
		out.UsedPct = float64(out.UsedKB) / float64(out.TotalKB) * 100
	}
	return out
}

func processCount() int {
	count := 0
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			count++
		}
	}
	return count
}
