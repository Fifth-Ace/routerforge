package main

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var keeneticModelPattern = regexp.MustCompile(`(?i)\bKN-\d{4}\b`)

type platformStorage struct {
	Mount          string  `json:"mount"`
	TotalBytes     uint64  `json:"total_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPct        float64 `json:"used_pct"`
}

type platformInfo struct {
	Hostname      string          `json:"hostname"`
	Model         string          `json:"model"`
	ModelFull     string          `json:"model_full,omitempty"`
	Architecture  string          `json:"architecture"`
	Target        string          `json:"target,omitempty"`
	UptimeSeconds int64           `json:"uptime_seconds"`
	Opt           platformStorage `json:"opt"`
}

func registerPlatformHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/platform", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, `{"error":"GET required"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(readPlatformInfo())
	})
}

func readPlatformInfo() platformInfo {
	full := readDeviceModel()
	return platformInfo{
		Hostname:      firstPlatformValue(readPlatformText("/proc/sys/kernel/hostname"), "RouterForge"),
		Model:         shortDeviceModel(full),
		ModelFull:     full,
		Architecture:  runtime.GOARCH,
		Target:        releaseTarget,
		UptimeSeconds: readPlatformUptimeSeconds(),
		Opt:           readPlatformStorage("/opt"),
	}
}

func readDeviceModel() string {
	for _, path := range []string{
		"/proc/device-tree/model",
		"/sys/firmware/devicetree/base/model",
		"/tmp/ndm/device-model",
	} {
		if value := readPlatformText(path); value != "" {
			return value
		}
	}
	return ""
}

func shortDeviceModel(value string) string {
	if match := keeneticModelPattern.FindString(value); match != "" {
		return strings.ToUpper(match)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "Keenetic"
	}
	fields := strings.Fields(value)
	if len(fields) > 3 {
		return strings.Join(fields[len(fields)-3:], " ")
	}
	return value
}

func parsePlatformUptimeSeconds(value string) int64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return int64(seconds)
}

func readPlatformUptimeSeconds() int64 {
	return parsePlatformUptimeSeconds(readPlatformText("/proc/uptime"))
}

func readPlatformText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(string(data), "\x00"))
}

func firstPlatformValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
