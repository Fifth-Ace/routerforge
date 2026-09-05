package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultSocket = "/opt/var/run/dns-monitor-admin.sock"

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

type adminSummary struct {
	GeneratedAt   time.Time  `json:"generated_at"`
	Hostname      string     `json:"hostname"`
	Kernel        string     `json:"kernel"`
	Architecture  string     `json:"architecture"`
	UptimeSeconds float64    `json:"uptime_seconds"`
	Load1         float64    `json:"load_1"`
	Load5         float64    `json:"load_5"`
	Load15        float64    `json:"load_15"`
	CPUCount      int        `json:"cpu_count"`
	ProcessCount  int        `json:"process_count"`
	Memory        memoryInfo `json:"memory"`
	MutationAPI   bool       `json:"mutation_api"`
	Mode          string     `json:"mode"`
}

type cpuSample struct {
	Name   string  `json:"name"`
	Usage  float64 `json:"usage_pct"`
	User   uint64  `json:"user"`
	System uint64  `json:"system"`
	Idle   uint64  `json:"idle"`
	Total  uint64  `json:"total"`
}

type processInfo struct {
	PID      int    `json:"pid"`
	Name     string `json:"name"`
	Command  string `json:"command"`
	State    string `json:"state"`
	User     string `json:"user"`
	UID      int    `json:"uid"`
	RSSKB    int64  `json:"rss_kb"`
	VmSizeKB int64  `json:"vmsize_kb"`
	Threads  int    `json:"threads"`
}

type portInfo struct {
	Protocol      string `json:"protocol"`
	LocalAddress  string `json:"local_address"`
	LocalPort     int    `json:"local_port"`
	RemoteAddress string `json:"remote_address,omitempty"`
	RemotePort    int    `json:"remote_port,omitempty"`
	State         string `json:"state"`
	PID           int    `json:"pid,omitempty"`
	Process       string `json:"process,omitempty"`
	Inode         string `json:"inode,omitempty"`
}

type serviceInfo struct {
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Executable    bool      `json:"executable"`
	Running       bool      `json:"running"`
	RunningSource string    `json:"running_source,omitempty"`
	ModifiedAt    time.Time `json:"modified_at"`
}

type packageInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture,omitempty"`
	Status       string `json:"status,omitempty"`
}

type storageInfo struct {
	Device         string  `json:"device"`
	Mount          string  `json:"mount"`
	FSType         string  `json:"fs_type"`
	TotalBytes     uint64  `json:"total_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPct        float64 `json:"used_pct"`
	Reads          uint64  `json:"reads,omitempty"`
	ReadSectors    uint64  `json:"read_sectors,omitempty"`
	ReadMS         uint64  `json:"read_ms,omitempty"`
	Writes         uint64  `json:"writes,omitempty"`
	WriteSectors   uint64  `json:"write_sectors,omitempty"`
	WriteMS        uint64  `json:"write_ms,omitempty"`
}

type thermalInfo struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Source string  `json:"source"`
	TempC  float64 `json:"temp_c"`
}

func main() {
	socket := flag.String("socket", defaultSocket, "Unix socket path")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*socket), 0755); err != nil {
		panic(err)
	}
	_ = os.Remove(*socket)

	listener, err := net.Listen("unix", *socket)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	_ = os.Chmod(*socket, 0600)

	cpuSampler, err := newCPUSampler(1*time.Second, 5)
	if err != nil {
		panic(err)
	}
	defer cpuSampler.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"mode":         "read-only",
			"mutation_api": false,
		})
	}))
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, readSummary())
	}))
	mux.HandleFunc("/v1/cpu", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		cpus, sampledAt, ready := cpuSampler.Snapshot()
		writeJSON(w, http.StatusOK, map[string]any{
			"cpus":           cpus,
			"sampled_at":     sampledAt,
			"ready":          ready,
			"window_seconds": cpuSampler.WindowSeconds(),
		})
	}))
	mux.HandleFunc("/v1/processes", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"processes": readProcesses()})
	}))
	mux.HandleFunc("/v1/ports", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ports": readPorts()})
	}))
	mux.HandleFunc("/v1/services", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"services": readServices()})
	}))
	mux.HandleFunc("/v1/packages", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"packages": readPackages()})
	}))
	mux.HandleFunc("/v1/storage", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"storage": readStorage()})
	}))
	mux.HandleFunc("/v1/thermal", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"sensors": readThermals()})
	}))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error":        "read-only module",
				"mutation_api": false,
			})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
}

func readSummary() adminSummary {
	load := readLoadAverage()
	uptime := readFloatField("/proc/uptime", 0)
	processes := 0
	if entries, err := os.ReadDir("/proc"); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				if _, err := strconv.Atoi(entry.Name()); err == nil {
					processes++
				}
			}
		}
	}

	return adminSummary{
		GeneratedAt:   time.Now(),
		Hostname:      firstNonEmpty(readTrimmed("/proc/sys/kernel/hostname"), hostname()),
		Kernel:        readTrimmed("/proc/sys/kernel/osrelease"),
		Architecture:  runtime.GOARCH,
		UptimeSeconds: uptime,
		Load1:         load[0],
		Load5:         load[1],
		Load15:        load[2],
		CPUCount:      runtime.NumCPU(),
		ProcessCount:  processes,
		Memory:        readMemory(),
		MutationAPI:   false,
		Mode:          "read-only",
	}
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

func readLoadAverage() [3]float64 {
	var out [3]float64
	fields := strings.Fields(readTrimmed("/proc/loadavg"))
	for i := 0; i < len(fields) && i < 3; i++ {
		out[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return out
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
		TotalKB:     values["MemTotal"],
		FreeKB:      values["MemFree"],
		AvailableKB: values["MemAvailable"],
		BuffersKB:   values["Buffers"],
		CachedKB:    values["Cached"] + values["SReclaimable"],
		SwapTotalKB: values["SwapTotal"],
		SwapFreeKB:  values["SwapFree"],
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
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		if fields[0] == "cpu" {
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

type cpuSampler struct {
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

func newCPUSampler(interval time.Duration, window int) (*cpuSampler, error) {
	if interval <= 0 {
		interval = time.Second
	}
	if window < 1 {
		window = 1
	}

	raw, err := readCPURaw()
	if err != nil {
		return nil, err
	}

	s := &cpuSampler{
		interval: interval,
		window:   window,
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

func (s *cpuSampler) run() {
	defer close(s.done)

	// First result becomes available after one full interval, then the endpoint
	// serves cached rolling data without sleeping inside HTTP requests.
	timer := time.NewTimer(s.interval)
	defer timer.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-timer.C:
			s.sample()
			timer.Reset(s.interval)
		}
	}
}

func (s *cpuSampler) sample() {
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
			Name:   item.Name,
			Usage:  rolling,
			User:   item.User,
			System: item.System,
			Idle:   item.Idle,
			Total:  item.Total,
		})
	}

	sort.Slice(latest, func(i, j int) bool { return latest[i].Name < latest[j].Name })
	if len(latest) > 0 {
		s.latest = latest
		s.sampledAt = time.Now()
		s.ready = true
	}
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

func (s *cpuSampler) Snapshot() ([]cpuSample, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]cpuSample(nil), s.latest...)
	return out, s.sampledAt, s.ready
}

func (s *cpuSampler) WindowSeconds() int {
	seconds := int(s.interval.Seconds()) * s.window
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (s *cpuSampler) Close() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	<-s.done
}

func readProcesses() []processInfo {
	users := readUsers()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	out := make([]processInfo, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		base := filepath.Join("/proc", entry.Name())
		status := readColonFile(filepath.Join(base, "status"))
		name := strings.TrimSpace(status["Name"])
		if name == "" {
			name = readTrimmed(filepath.Join(base, "comm"))
		}
		uid := firstInt(strings.Fields(status["Uid"]))
		command := name
		cmdline := strings.Split(string(readBytes(filepath.Join(base, "cmdline"))), "\x00")
		if len(cmdline) > 0 && strings.TrimSpace(cmdline[0]) != "" {
			command = cmdline[0]
		}
		out = append(out, processInfo{
			PID:      pid,
			Name:     name,
			Command:  command,
			State:    firstField(status["State"]),
			User:     firstNonEmpty(users[uid], strconv.Itoa(uid)),
			UID:      uid,
			RSSKB:    parseKB(status["VmRSS"]),
			VmSizeKB: parseKB(status["VmSize"]),
			Threads:  firstInt(strings.Fields(status["Threads"])),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RSSKB != out[j].RSSKB {
			return out[i].RSSKB > out[j].RSSKB
		}
		return out[i].PID < out[j].PID
	})
	if len(out) > 300 {
		out = out[:300]
	}
	return out
}

func readUsers() map[int]string {
	out := map[int]string{}
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ":")
		if len(fields) < 3 {
			continue
		}
		uid, _ := strconv.Atoi(fields[2])
		out[uid] = fields[0]
	}
	return out
}

func readPorts() []portInfo {
	inodes, processes := socketOwners()
	var out []portInfo
	out = append(out, parseProcNet("/proc/net/tcp", "tcp4", inodes, processes, true)...)
	out = append(out, parseProcNet("/proc/net/tcp6", "tcp6", inodes, processes, true)...)
	out = append(out, parseProcNet("/proc/net/udp", "udp4", inodes, processes, false)...)
	out = append(out, parseProcNet("/proc/net/udp6", "udp6", inodes, processes, false)...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].LocalPort != out[j].LocalPort {
			return out[i].LocalPort < out[j].LocalPort
		}
		return out[i].Protocol < out[j].Protocol
	})
	return out
}

func socketOwners() (map[string]int, map[int]string) {
	owners := map[string]int{}
	names := map[int]string{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return owners, names
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		base := filepath.Join("/proc", entry.Name())
		names[pid] = readTrimmed(filepath.Join(base, "comm"))
		fds, err := os.ReadDir(filepath.Join(base, "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(base, "fd", fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
				inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
				if _, exists := owners[inode]; !exists {
					owners[inode] = pid
				}
			}
		}
	}
	return owners, names
}

func parseProcNet(file, protocol string, owners map[string]int, names map[int]string, tcp bool) []portInfo {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	first := true
	var out []portInfo
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		state := fields[3]
		if tcp && state != "0A" {
			continue
		}
		localAddr, localPort := decodeEndpoint(fields[1])
		remoteAddr, remotePort := decodeEndpoint(fields[2])
		inode := fields[9]
		pid := owners[inode]
		out = append(out, portInfo{
			Protocol:      protocol,
			LocalAddress:  localAddr,
			LocalPort:     localPort,
			RemoteAddress: remoteAddr,
			RemotePort:    remotePort,
			State:         socketState(state, tcp),
			PID:           pid,
			Process:       names[pid],
			Inode:         inode,
		})
	}
	return out
}

func decodeEndpoint(raw string) (string, int) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return raw, 0
	}
	port64, _ := strconv.ParseUint(parts[1], 16, 16)
	host := parts[0]
	if len(host) == 8 {
		b, err := hex.DecodeString(host)
		if err == nil && len(b) == 4 {
			host = net.IPv4(b[3], b[2], b[1], b[0]).String()
		}
	} else if len(host) == 32 {
		b, err := hex.DecodeString(host)
		if err == nil && len(b) == 16 {
			for i := 0; i < 16; i += 4 {
				b[i], b[i+3] = b[i+3], b[i]
				b[i+1], b[i+2] = b[i+2], b[i+1]
			}
			host = net.IP(b).String()
		}
	}
	return host, int(port64)
}

func socketState(state string, tcp bool) string {
	if !tcp {
		if state == "07" {
			return "UNCONN"
		}
		return state
	}
	states := map[string]string{
		"01": "ESTABLISHED", "02": "SYN_SENT", "03": "SYN_RECV", "04": "FIN_WAIT1",
		"05": "FIN_WAIT2", "06": "TIME_WAIT", "07": "CLOSE", "08": "CLOSE_WAIT",
		"09": "LAST_ACK", "0A": "LISTEN", "0B": "CLOSING",
	}
	if name := states[state]; name != "" {
		return name
	}
	return state
}

func readServices() []serviceInfo {
	entries, err := os.ReadDir("/opt/etc/init.d")
	if err != nil {
		return nil
	}
	processes := processSearchCorpus()
	var out []serviceInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "S") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join("/opt/etc/init.d", entry.Name())
		serviceName := trimServicePrefix(entry.Name())
		needle := strings.ToLower(strings.ReplaceAll(serviceName, "-", ""))
		running := false
		if needle != "" {
			for _, corpus := range processes {
				normalized := strings.ToLower(strings.ReplaceAll(corpus, "-", ""))
				if strings.Contains(normalized, needle) {
					running = true
					break
				}
			}
		}
		out = append(out, serviceInfo{
			Name:          serviceName,
			Path:          path,
			Executable:    info.Mode()&0111 != 0,
			Running:       running,
			RunningSource: "process-match",
			ModifiedAt:    info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func trimServicePrefix(name string) string {
	i := 1
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	return strings.TrimLeft(name[i:], "-_")
}

func processSearchCorpus() []string {
	var out []string
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		base := filepath.Join("/proc", entry.Name())
		comm := readTrimmed(filepath.Join(base, "comm"))
		cmd := strings.ReplaceAll(string(readBytes(filepath.Join(base, "cmdline"))), "\x00", " ")
		out = append(out, comm+" "+cmd)
	}
	return out
}

func readPackages() []packageInfo {
	paths := []string{"/opt/lib/opkg/status", "/opt/var/opkg/status"}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		out := parsePackages(f)
		_ = f.Close()
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func parsePackages(f *os.File) []packageInfo {
	var out []packageInfo
	sc := bufio.NewScanner(f)
	current := packageInfo{}
	commit := func() {
		if current.Name != "" {
			out = append(out, current)
		}
		current = packageInfo{}
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			commit()
			continue
		}
		switch {
		case strings.HasPrefix(line, "Package:"):
			current.Name = strings.TrimSpace(strings.TrimPrefix(line, "Package:"))
		case strings.HasPrefix(line, "Version:"):
			current.Version = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		case strings.HasPrefix(line, "Architecture:"):
			current.Architecture = strings.TrimSpace(strings.TrimPrefix(line, "Architecture:"))
		case strings.HasPrefix(line, "Status:"):
			current.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	commit()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type diskStat struct {
	Reads        uint64
	ReadSectors  uint64
	ReadMS       uint64
	Writes       uint64
	WriteSectors uint64
	WriteMS      uint64
}

func readDiskStats() map[string]diskStat {
	out := map[string]diskStat{}
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
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			return v
		}
		out[fields[2]] = diskStat{
			Reads: n(3), ReadSectors: n(5), ReadMS: n(6),
			Writes: n(7), WriteSectors: n(9), WriteMS: n(10),
		}
	}
	return out
}

func readStorage() []storageInfo {
	stats := readDiskStats()
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []storageInfo
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		device := unescapeMount(fields[0])
		mount := unescapeMount(fields[1])
		fsType := fields[2]
		if seen[mount] {
			continue
		}
		seen[mount] = true
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		blockSize := uint64(st.Bsize)
		total := uint64(st.Blocks) * blockSize
		free := uint64(st.Bfree) * blockSize
		avail := uint64(st.Bavail) * blockSize
		used := total - free
		pct := 0.0
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		item := storageInfo{
			Device: device, Mount: mount, FSType: fsType,
			TotalBytes: total, FreeBytes: free, AvailableBytes: avail,
			UsedBytes: used, UsedPct: pct,
		}
		if strings.HasPrefix(device, "/dev/") {
			devName := filepath.Base(device)
			if ds, ok := stats[devName]; ok {
				item.Reads, item.ReadSectors, item.ReadMS = ds.Reads, ds.ReadSectors, ds.ReadMS
				item.Writes, item.WriteSectors, item.WriteMS = ds.Writes, ds.WriteSectors, ds.WriteMS
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	return out
}

func unescapeMount(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(value)
}

func readThermals() []thermalInfo {
	var out []thermalInfo

	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, zone := range zones {
		temp, ok := readTemperature(filepath.Join(zone, "temp"))
		if !ok {
			continue
		}
		name := firstNonEmpty(readTrimmed(filepath.Join(zone, "type")), filepath.Base(zone))
		out = append(out, thermalInfo{
			ID: filepath.Base(zone), Name: name, Source: zone, TempC: temp,
		})
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
			out = append(out, thermalInfo{
				ID:   filepath.Base(hwmon) + "/" + base,
				Name: name, Source: input, TempC: temp,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].TempC != out[j].TempC {
			return out[i].TempC > out[j].TempC
		}
		return out[i].Name < out[j].Name
	})
	return out
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
	if value > 1000 {
		value /= 1000
	}
	if value < -100 || value > 200 {
		return 0, false
	}
	return value, true
}

func readColonFile(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		out[line[:idx]] = strings.TrimSpace(line[idx+1:])
	}
	return out
}

func parseKB(value string) int64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.ParseInt(fields[0], 10, 64)
	return n
}

func firstInt(fields []string) int {
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[0])
	return n
}

func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func readFloatField(path string, index int) float64 {
	fields := strings.Fields(readTrimmed(path))
	if index < 0 || index >= len(fields) {
		return 0
	}
	n, _ := strconv.ParseFloat(fields[index], 64)
	return n
}

func readTrimmed(path string) string {
	return strings.TrimSpace(string(readBytes(path)))
}

func readBytes(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
