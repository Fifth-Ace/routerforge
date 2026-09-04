package main

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type diskCounters struct {
	Reads        uint64
	ReadSectors  uint64
	Writes       uint64
	WriteSectors uint64
}

type diskRate struct {
	ReadBPS  float64 `json:"read_bps"`
	WriteBPS float64 `json:"write_bps"`
	ReadOPS  float64 `json:"read_ops"`
	WriteOPS float64 `json:"write_ops"`
}

type storageMount struct {
	Device         string  `json:"device"`
	Mount          string  `json:"mount"`
	FSType         string  `json:"fs_type"`
	TotalBytes     uint64  `json:"total_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPct        float64 `json:"used_pct"`
	ReadBPS        float64 `json:"read_bps,omitempty"`
	WriteBPS       float64 `json:"write_bps,omitempty"`
	ReadOPS        float64 `json:"read_ops,omitempty"`
	WriteOPS       float64 `json:"write_ops,omitempty"`
}

type blockDevice struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Model      string   `json:"model,omitempty"`
	Vendor     string   `json:"vendor,omitempty"`
	Transport  string   `json:"transport,omitempty"`
	SizeBytes  uint64   `json:"size_bytes"`
	Removable  bool     `json:"removable"`
	Rotational bool     `json:"rotational"`
	ReadOnly   bool     `json:"read_only"`
	Rate       diskRate `json:"rate"`
}

type storageCollector struct {
	mu       sync.RWMutex
	interval time.Duration
	previous map[string]diskCounters
	rates    map[string]diskRate
	sampled  time.Time
	stop     chan struct{}
	done     chan struct{}
}

func newStorageCollector(interval time.Duration) *storageCollector {
	if interval < time.Second {
		interval = 2 * time.Second
	}
	s := &storageCollector{
		interval: interval,
		previous: readDiskCounters(),
		rates:    map[string]diskRate{},
		sampled:  time.Now(),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *storageCollector) Close() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	<-s.done
}

func (s *storageCollector) run() {
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

func (s *storageCollector) sample() {
	now := time.Now()
	current := readDiskCounters()

	s.mu.Lock()
	defer s.mu.Unlock()

	elapsed := now.Sub(s.sampled).Seconds()
	if elapsed <= 0 {
		elapsed = s.interval.Seconds()
	}

	rates := make(map[string]diskRate, len(current))
	for name, cur := range current {
		prev, ok := s.previous[name]
		if !ok {
			continue
		}
		rates[name] = diskRate{
			ReadBPS:  counterDelta(cur.ReadSectors, prev.ReadSectors) * 512 / elapsed,
			WriteBPS: counterDelta(cur.WriteSectors, prev.WriteSectors) * 512 / elapsed,
			ReadOPS:  counterDelta(cur.Reads, prev.Reads) / elapsed,
			WriteOPS: counterDelta(cur.Writes, prev.Writes) / elapsed,
		}
	}
	s.previous = current
	s.rates = rates
	s.sampled = now
}

func counterDelta(current, previous uint64) float64 {
	if current < previous {
		return 0
	}
	return float64(current - previous)
}

func (s *storageCollector) rateSnapshot() (map[string]diskRate, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]diskRate, len(s.rates))
	for key, value := range s.rates {
		out[key] = value
	}
	return out, s.sampled
}

func (s *moduleServer) registerStorage(mux *http.ServeMux) {
	mux.HandleFunc("/v1/storage", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		rates, sampled := s.storage.rateSnapshot()
		allMounts := readAllMounts(rates)
		allDisks := readAllBlockDevices(rates)
		writeJSON(w, http.StatusOK, map[string]any{
			"mounts":         userMounts(allMounts),
			"system_mounts":  allMounts,
			"disks":          userBlockDevices(allDisks),
			"system_disks":   allDisks,
			"sampled_at":     sampled,
			"sample_seconds": int(s.storage.interval.Seconds()),
			"benchmarking":   false,
		})
	}))
}

func readDiskCounters() map[string]diskCounters {
	out := map[string]diskCounters{}
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 14 {
			continue
		}
		n := func(i int) uint64 {
			value, _ := strconv.ParseUint(fields[i], 10, 64)
			return value
		}
		out[fields[2]] = diskCounters{
			Reads: n(3), ReadSectors: n(5), Writes: n(7), WriteSectors: n(9),
		}
	}
	return out
}

func readAllMounts(rates map[string]diskRate) []storageMount {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := map[string]bool{}
	var out []storageMount
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		device := unescapeMount(fields[0])
		mount := unescapeMount(fields[1])
		fsType := fields[2]

		if seen[mount] || pseudoFilesystem(fsType) {
			continue
		}
		seen[mount] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}
		blockSize := uint64(stat.Bsize)
		total := uint64(stat.Blocks) * blockSize
		free := uint64(stat.Bfree) * blockSize
		available := uint64(stat.Bavail) * blockSize
		used := total - free

		usedPct := 0.0
		if total > 0 {
			usedPct = float64(used) / float64(total) * 100
		}
		item := storageMount{
			Device: device, Mount: mount, FSType: fsType,
			TotalBytes: total, FreeBytes: free, AvailableBytes: available,
			UsedBytes: used, UsedPct: usedPct,
		}
		if strings.HasPrefix(device, "/dev/") {
			if rate, ok := rates[filepath.Base(device)]; ok {
				item.ReadBPS, item.WriteBPS = rate.ReadBPS, rate.WriteBPS
				item.ReadOPS, item.WriteOPS = rate.ReadOPS, rate.WriteOPS
			}
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		pi, pj := mountPriority(out[i].Mount), mountPriority(out[j].Mount)
		if pi != pj {
			return pi < pj
		}
		return out[i].Mount < out[j].Mount
	})
	return out
}

func userMounts(all []storageMount) []storageMount {
	bestByDevice := map[string]storageMount{}
	deviceOrder := []string{}

	for _, item := range all {
		if !eligibleUserMount(item) {
			continue
		}
		key := strings.TrimSpace(item.Device)
		if key == "" {
			key = "mount:" + item.Mount
		}
		current, exists := bestByDevice[key]
		if !exists {
			bestByDevice[key] = item
			deviceOrder = append(deviceOrder, key)
			continue
		}
		if mountPriority(item.Mount) < mountPriority(current.Mount) {
			bestByDevice[key] = item
		}
	}

	out := make([]storageMount, 0, len(bestByDevice))
	for _, key := range deviceOrder {
		if item, ok := bestByDevice[key]; ok {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi, pj := mountPriority(out[i].Mount), mountPriority(out[j].Mount)
		if pi != pj {
			return pi < pj
		}
		return out[i].Mount < out[j].Mount
	})
	return out
}

func eligibleUserMount(item storageMount) bool {
	if item.Mount == "/" {
		return false
	}
	if item.FSType == "squashfs" {
		return false
	}
	if item.TotalBytes == 0 {
		return false
	}
	device := strings.TrimSpace(item.Device)
	if device == "" || device == "rootfs" || device == "none" {
		return false
	}
	return true
}

func pseudoFilesystem(fsType string) bool {
	switch fsType {
	case "proc", "sysfs", "devtmpfs", "devpts", "tmpfs", "cgroup", "cgroup2",
		"debugfs", "tracefs", "securityfs", "pstore", "mqueue", "fusectl",
		"configfs", "overlay":
		return true
	default:
		return false
	}
}

func mountPriority(mount string) int {
	switch mount {
	case "/opt":
		return 1
	case "/storage":
		return 2
	case "/":
		return 3
	default:
		if strings.HasPrefix(mount, "/tmp/mnt/") {
			return 20
		}
		return 10
	}
}

func readAllBlockDevices(rates map[string]diskRate) []blockDevice {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}

	var out []blockDevice
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		base := filepath.Join("/sys/block", name)
		sectors := parseUint(readTrimmed(filepath.Join(base, "size")))
		out = append(out, blockDevice{
			Name: name, Path: "/dev/" + name,
			Model:      readTrimmed(filepath.Join(base, "device/model")),
			Vendor:     readTrimmed(filepath.Join(base, "device/vendor")),
			Transport:  blockTransport(base),
			SizeBytes:  sectors * 512,
			Removable:  readTrimmed(filepath.Join(base, "removable")) == "1",
			Rotational: readTrimmed(filepath.Join(base, "queue/rotational")) == "1",
			ReadOnly:   readTrimmed(filepath.Join(base, "ro")) == "1",
			Rate:       rates[name],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func userBlockDevices(all []blockDevice) []blockDevice {
	out := make([]blockDevice, 0, len(all))
	for _, device := range all {
		if systemOnlyBlockDevice(device.Name) || device.SizeBytes == 0 {
			continue
		}
		out = append(out, device)
	}
	return out
}

func systemOnlyBlockDevice(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "mtdblock") ||
		strings.HasPrefix(lower, "ubiblock") ||
		strings.HasPrefix(lower, "zram")
}

func blockTransport(base string) string {
	real, err := filepath.EvalSymlinks(filepath.Join(base, "device"))
	if err != nil {
		return ""
	}
	lower := strings.ToLower(real)
	switch {
	case strings.Contains(lower, "/usb"):
		return "usb"
	case strings.Contains(lower, "/nvme"):
		return "nvme"
	case strings.Contains(lower, "/mmc"):
		return "mmc"
	case strings.Contains(lower, "/ata") || strings.Contains(lower, "/sata"):
		return "ata"
	default:
		return ""
	}
}

func unescapeMount(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(value)
}
