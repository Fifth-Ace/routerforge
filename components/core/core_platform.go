package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var keeneticModelPattern = regexp.MustCompile(`(?i)\bKN-\d{4}\b`)

var deviceModelCache = struct {
	sync.Once
	value string
}{}

type platformStorage struct {
	Mount          string  `json:"mount"`
	TotalBytes     uint64  `json:"total_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPct        float64 `json:"used_pct"`
}

type platformInfo struct {
	Hostname         string          `json:"hostname"`
	Model            string          `json:"model"`
	ModelFull        string          `json:"model_full,omitempty"`
	Architecture     string          `json:"architecture"`
	Target           string          `json:"target,omitempty"`
	TargetStatus     string          `json:"target_status,omitempty"`
	TargetSource     string          `json:"target_source,omitempty"`
	TargetCandidates []string        `json:"target_candidates,omitempty"`
	UptimeSeconds    int64           `json:"uptime_seconds"`
	Opt              platformStorage `json:"opt"`
}

type platformTargetResolution struct {
	Status     string
	Target     string
	Source     string
	Candidates []string
}

func parseEntwareTargets(raw string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "arch" {
			continue
		}
		var target string
		switch strings.ToLower(fields[1]) {
		case "aarch64-3.10":
			target = "aarch64-3.10"
		case "mips-3.4":
			target = "mips-3.4"
		case "mipsel-3.4":
			target = "mipsel-3.4"
		default:
			continue
		}
		if !seen[target] {
			seen[target] = true
			out = append(out, target)
		}
	}
	return out
}

func targetResolutionFromArchitectureOutput(raw string) platformTargetResolution {
	candidates := parseEntwareTargets(raw)
	result := platformTargetResolution{
		Status:     "unknown",
		Source:     "opkg print-architecture",
		Candidates: candidates,
	}
	if len(candidates) == 1 {
		result.Status = "resolved"
		result.Target = candidates[0]
	} else if len(candidates) > 1 {
		result.Status = "ambiguous"
	}
	return result
}

func readOpkgPrintArchitecture() (string, error) {
	path := "/opt/bin/opkg"
	if _, err := os.Stat(path); err != nil {
		resolved, lookupErr := exec.LookPath("opkg")
		if lookupErr != nil {
			return "", lookupErr
		}
		path = resolved
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "print-architecture").Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func resolvePlatformTarget() platformTargetResolution {
	raw, err := readOpkgPrintArchitecture()
	if err != nil {
		return platformTargetResolution{
			Status: "unknown",
			Source: "opkg print-architecture",
		}
	}
	return targetResolutionFromArchitectureOutput(raw)
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
	target := resolvePlatformTarget()
	return platformInfo{
		Hostname:         firstPlatformValue(readPlatformText("/proc/sys/kernel/hostname"), "RouterForge"),
		Model:            shortDeviceModel(full),
		ModelFull:        full,
		Architecture:     runtime.GOARCH,
		Target:           target.Target,
		TargetStatus:     target.Status,
		TargetSource:     target.Source,
		TargetCandidates: append([]string(nil), target.Candidates...),
		UptimeSeconds:    readPlatformUptimeSeconds(),
		Opt:              readPlatformStorage("/opt"),
	}
}

func readDeviceModel() string {
	deviceModelCache.Do(func() {
		deviceModelCache.value = discoverDeviceModel()
	})
	return deviceModelCache.value
}

func discoverDeviceModel() string {
	for _, path := range []string{
		"/proc/device-tree/model",
		"/sys/firmware/devicetree/base/model",
		"/tmp/ndm/device-model",
	} {
		if value := readPlatformText(path); value != "" {
			return value
		}
	}
	return readNDMCDeviceModel()
}

func parseNDMCDeviceModel(raw string) string {
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if keeneticModelPattern.FindString(line) == "" {
			continue
		}
		if index := strings.IndexByte(line, ':'); index >= 0 {
			value := strings.TrimSpace(line[index+1:])
			if keeneticModelPattern.FindString(value) != "" {
				return value
			}
		}
		return line
	}
	return ""
}

func readNDMCDeviceModel() string {
	if _, err := exec.LookPath("ndmc"); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ndmc", "-c", "show version").Output()
	if err != nil {
		return ""
	}
	return parseNDMCDeviceModel(string(output))
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
