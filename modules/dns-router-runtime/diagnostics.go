package dnsrouter

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type ResourceSnapshot struct {
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
	Goroutines     int    `json:"goroutines"`
	RSSBytes       uint64 `json:"rss_bytes,omitempty"`
	PSSBytes       uint64 `json:"pss_bytes,omitempty"`
	RSSAvailable   bool   `json:"rss_available"`
	PSSAvailable   bool   `json:"pss_available"`
}

func readKBField(path, key string) (uint64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 2 {
			continue
		}
		if strings.TrimSuffix(parts[0], ":") == key {
			n, e := strconv.ParseUint(parts[1], 10, 64)
			if e == nil {
				return n * 1024, true
			}
		}
	}
	return 0, false
}
func CurrentResourceSnapshot() ResourceSnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	v := ResourceSnapshot{HeapAllocBytes: m.HeapAlloc, HeapSysBytes: m.HeapSys, Goroutines: runtime.NumGoroutine()}
	v.RSSBytes, v.RSSAvailable = readKBField("/proc/self/status", "VmRSS")
	v.PSSBytes, v.PSSAvailable = readKBField("/proc/self/smaps_rollup", "Pss")
	return v
}
