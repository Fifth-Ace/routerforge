package main

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type processInfo struct {
	PID       int     `json:"pid"`
	PPID      int     `json:"ppid"`
	Name      string  `json:"name"`
	Command   string  `json:"command"`
	State     string  `json:"state"`
	User      string  `json:"user"`
	UID       int     `json:"uid"`
	RSSKB     int64   `json:"rss_kb"`
	VmSizeKB  int64   `json:"vmsize_kb"`
	Threads   int     `json:"threads"`
	CPUPct    float64 `json:"cpu_pct"`
	MemoryPct float64 `json:"memory_pct"`
}

type processRaw struct {
	PID       int
	PPID      int
	Name      string
	State     string
	CPUTime   uint64
	StartTime uint64
	VmSizeKB  int64
	RSSKB     int64
	Threads   int
}

type processCPUState struct {
	CPUTime   uint64
	StartTime uint64
}

type processIdentity struct {
	StartTime uint64
	Name      string
	Command   string
	UID       int
	User      string
}

type processCollector struct {
	mu sync.RWMutex

	interval      time.Duration
	previousTotal uint64
	previous      map[int]processCPUState
	identities    map[int]processIdentity
	users         map[int]string
	latest        []processInfo
	sampledAt     time.Time
	ready         bool

	stop chan struct{}
	done chan struct{}
}

func newProcessCollector(interval time.Duration) (*processCollector, error) {
	if interval <= 0 {
		interval = time.Second
	}

	total, err := readTotalCPUJiffies()
	if err != nil {
		return nil, err
	}

	users := readProcessUsers()
	raw := readProcessStats()
	previous := make(map[int]processCPUState, len(raw))
	identities := make(map[int]processIdentity, len(raw))
	latest := make([]processInfo, 0, len(raw))
	memoryTotalKB := readMemory().TotalKB

	for _, item := range raw {
		previous[item.PID] = processCPUState{CPUTime: item.CPUTime, StartTime: item.StartTime}
		identity := readProcessIdentity(item, users)
		identities[item.PID] = identity
		latest = append(latest, processInfoFromRaw(item, identity, memoryTotalKB, 0))
	}
	sortProcessInfos(latest)

	collector := &processCollector{
		interval:      interval,
		previousTotal: total,
		previous:      previous,
		identities:    identities,
		users:         users,
		latest:        latest,
		sampledAt:     time.Now(),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go collector.run()
	return collector, nil
}

func (c *processCollector) Close() {
	select {
	case <-c.stop:
		return
	default:
		close(c.stop)
	}
	<-c.done
}

func (c *processCollector) run() {
	defer close(c.done)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.sample()
		}
	}
}

func (c *processCollector) sample() {
	currentTotal, err := readTotalCPUJiffies()
	if err != nil {
		return
	}
	current := readProcessStats()
	memoryTotalKB := readMemory().TotalKB

	c.mu.RLock()
	previousTotal := c.previousTotal
	previous := c.previous
	identities := c.identities
	users := c.users
	c.mu.RUnlock()

	nextPrevious := make(map[int]processCPUState, len(current))
	nextIdentities := make(map[int]processIdentity, len(current))
	latest := make([]processInfo, 0, len(current))
	cpuCount := runtime.NumCPU()
	if cpuCount < 1 {
		cpuCount = 1
	}

	for _, item := range current {
		identity, ok := identities[item.PID]
		if !ok || identity.StartTime != item.StartTime || identity.Name != item.Name {
			identity = readProcessIdentity(item, users)
		}

		cpuPct := 0.0
		if prev, ok := previous[item.PID]; ok && prev.StartTime == item.StartTime {
			cpuPct = processCPUPercent(prev.CPUTime, item.CPUTime, previousTotal, currentTotal, cpuCount)
		}

		nextPrevious[item.PID] = processCPUState{CPUTime: item.CPUTime, StartTime: item.StartTime}
		nextIdentities[item.PID] = identity
		latest = append(latest, processInfoFromRaw(item, identity, memoryTotalKB, cpuPct))
	}
	sortProcessInfos(latest)

	c.mu.Lock()
	c.previousTotal = currentTotal
	c.previous = nextPrevious
	c.identities = nextIdentities
	c.latest = latest
	c.sampledAt = time.Now()
	c.ready = true
	c.mu.Unlock()
}

func (c *processCollector) snapshot() ([]processInfo, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]processInfo(nil), c.latest...), c.sampledAt, c.ready
}

func (c *processCollector) intervalMillis() int64 {
	return c.interval.Milliseconds()
}

func readTotalCPUJiffies() (uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var total uint64
		for i := 1; i < len(fields) && i <= 8; i++ {
			value, parseErr := strconv.ParseUint(fields[i], 10, 64)
			if parseErr != nil {
				return 0, parseErr
			}
			total += value
		}
		return total, nil
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("aggregate cpu line missing from /proc/stat")
}

func readProcessStats() []processRaw {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	pageKB := int64(os.Getpagesize() / 1024)
	if pageKB < 1 {
		pageKB = 4
	}

	out := make([]processRaw, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		item, ok := parseProcessStatLine(pid, readTrimmed(filepath.Join("/proc", entry.Name(), "stat")))
		if !ok {
			continue
		}
		item.RSSKB *= pageKB
		out = append(out, item)
	}
	return out
}

func parseProcessStatLine(expectedPID int, line string) (processRaw, bool) {
	line = strings.TrimSpace(line)
	open := strings.IndexByte(line, '(')
	close := strings.LastIndexByte(line, ')')
	if open <= 0 || close <= open || close+1 >= len(line) {
		return processRaw{}, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(line[:open]))
	if err != nil || pid <= 0 || (expectedPID > 0 && pid != expectedPID) {
		return processRaw{}, false
	}
	fields := strings.Fields(strings.TrimSpace(line[close+1:]))
	if len(fields) < 22 {
		return processRaw{}, false
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return processRaw{}, false
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return processRaw{}, false
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return processRaw{}, false
	}
	threads, err := strconv.Atoi(fields[17])
	if err != nil {
		return processRaw{}, false
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return processRaw{}, false
	}
	vsizeBytes, err := strconv.ParseUint(fields[20], 10, 64)
	if err != nil {
		return processRaw{}, false
	}
	rssPages, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return processRaw{}, false
	}
	if rssPages < 0 {
		rssPages = 0
	}

	return processRaw{
		PID:       pid,
		PPID:      ppid,
		Name:      strings.TrimSpace(line[open+1 : close]),
		State:     fields[0],
		CPUTime:   utime + stime,
		StartTime: startTime,
		VmSizeKB:  int64(vsizeBytes / 1024),
		RSSKB:     rssPages,
		Threads:   threads,
	}, true
}

func readProcessIdentity(item processRaw, users map[int]string) processIdentity {
	status := scanKeyValueFile(filepath.Join("/proc", strconv.Itoa(item.PID), "status"), ':')
	uid := 0
	if fields := strings.Fields(status["Uid"]); len(fields) > 0 {
		uid, _ = strconv.Atoi(fields[0])
	}
	command := firstProcessCommand(readBytes(filepath.Join("/proc", strconv.Itoa(item.PID), "cmdline")))
	if command == "" {
		command = item.Name
	}
	return processIdentity{
		StartTime: item.StartTime,
		Name:      item.Name,
		Command:   command,
		UID:       uid,
		User:      firstNonEmpty(users[uid], strconv.Itoa(uid)),
	}
}

func firstProcessCommand(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if idx := strings.IndexByte(string(raw), 0); idx >= 0 {
		raw = raw[:idx]
	}
	command := strings.TrimSpace(string(raw))
	if len(command) > 512 {
		command = command[:512]
	}
	return command
}

func readProcessUsers() map[int]string {
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
		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		out[uid] = fields[0]
	}
	return out
}

func processCPUPercent(previousProcess, currentProcess, previousTotal, currentTotal uint64, cpuCount int) float64 {
	if currentProcess < previousProcess || currentTotal <= previousTotal {
		return 0
	}
	processDelta := currentProcess - previousProcess
	totalDelta := currentTotal - previousTotal
	if processDelta == 0 || totalDelta == 0 {
		return 0
	}
	if cpuCount < 1 {
		cpuCount = 1
	}
	value := float64(processDelta) / float64(totalDelta) * float64(cpuCount) * 100
	maxValue := float64(cpuCount * 100)
	if value > maxValue {
		return maxValue
	}
	return value
}

func processInfoFromRaw(item processRaw, identity processIdentity, memoryTotalKB int64, cpuPct float64) processInfo {
	memoryPct := 0.0
	if memoryTotalKB > 0 && item.RSSKB > 0 {
		memoryPct = float64(item.RSSKB) / float64(memoryTotalKB) * 100
	}
	return processInfo{
		PID:       item.PID,
		PPID:      item.PPID,
		Name:      item.Name,
		Command:   identity.Command,
		State:     item.State,
		User:      identity.User,
		UID:       identity.UID,
		RSSKB:     item.RSSKB,
		VmSizeKB:  item.VmSizeKB,
		Threads:   item.Threads,
		CPUPct:    cpuPct,
		MemoryPct: memoryPct,
	}
}

func sortProcessInfos(items []processInfo) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].CPUPct != items[j].CPUPct {
			return items[i].CPUPct > items[j].CPUPct
		}
		if items[i].RSSKB != items[j].RSSKB {
			return items[i].RSSKB > items[j].RSSKB
		}
		return items[i].PID < items[j].PID
	})
}
