package main

import (
	"encoding/json"
	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const foundationMaxInt = uint64(9007199254740991)

type foundationBytes struct {
	Total uint64 `json:"totalBytes"`
	Used  uint64 `json:"usedBytes"`
}

func foundationMemPairs(raw string) (memory, swap *foundationBytes) {
	vals := map[string]uint64{}
	for _, l := range strings.Split(raw, "\n") {
		f := strings.Fields(l)
		if len(f) != 3 || f[2] != "kB" {
			continue
		}
		k := strings.TrimSuffix(f[0], ":")
		switch k {
		case "MemTotal", "MemAvailable", "SwapTotal", "SwapFree":
		default:
			continue
		}
		n, e := strconv.ParseUint(f[1], 10, 64)
		if e == nil && n <= foundationMaxInt/1024 {
			vals[k] = n * 1024
		}
	}
	pair := func(total, avail string, allowZero bool) *foundationBytes {
		t, ok := vals[total]
		a, ok2 := vals[avail]
		if !ok || !ok2 || a > t || (!allowZero && t == 0) {
			return nil
		}
		return &foundationBytes{t, t - a}
	}
	return pair("MemTotal", "MemAvailable", false), pair("SwapTotal", "SwapFree", true)
}
func foundationUptime(raw string) any {
	f := strings.Fields(raw)
	if len(f) == 0 {
		return nil
	}
	x, e := strconv.ParseFloat(f[0], 64)
	if e != nil || math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > float64(foundationMaxInt) {
		return nil
	}
	return uint64(x)
}

// Local observation has no registered device/connection binding yet. It is not
// advertised as a full cross-product Snapshot, or as remote health evidence.
func registerFoundationMetrics(mux *http.ServeMux) {
	mux.HandleFunc(foundationPath+"/local-metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		if r.URL.RawQuery != "" {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		if foundationObserverID == "" {
			writeAuthJSON(w, 503, map[string]string{"error": "OBSERVER_UNAVAILABLE"})
			return
		}
		id, e := platformevents.NewEventID()
		if e != nil {
			writeAuthJSON(w, 503, map[string]string{"error": "SAMPLE_ID_UNAVAILABLE"})
			return
		}
		cpu, cpuOK := coreCPUUsageSampler.sample()
		mem, swap := foundationMemPairs(readPlatformText("/proc/meminfo"))
		fs := readPlatformStorage("/opt")
		metrics := map[string]any{"cpuPercent": nil, "load": nil, "memory": mem, "swap": swap, "filesystems": nil, "network": nil, "uptimeSec": foundationUptime(readPlatformText("/proc/uptime")), "system": nil, "tls": nil, "services": nil}
		if cpuOK && cpu >= 0 && cpu <= 100 {
			metrics["cpuPercent"] = cpu
		}
		if fs.TotalBytes > 0 && fs.TotalBytes <= foundationMaxInt && fs.UsedBytes <= fs.TotalBytes && fs.AvailableBytes <= fs.TotalBytes {
			metrics["filesystems"] = []map[string]any{{"mount": "/opt", "totalBytes": fs.TotalBytes, "usedBytes": fs.UsedBytes, "availableBytes": fs.AvailableBytes}}
		}
		b, e := json.Marshal(metrics)
		if e != nil || contract.ValidateMetrics(b) != nil {
			writeAuthJSON(w, 503, map[string]string{"error": "INVALID_METRICS"})
			return
		}
		writeAuthJSON(w, 200, map[string]any{"schemaVersion": 1, "sampleId": id, "observerId": foundationObserverID, "observerScope": "installation", "observedAt": time.Now().UTC(), "deviceId": nil, "connectionId": nil, "metrics": metrics})
	})
}
