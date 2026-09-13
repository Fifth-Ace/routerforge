package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type manifestCheck struct {
	Path       string `json:"path"`
	Valid      bool   `json:"valid"`
	ID         string `json:"id,omitempty"`
	Version    string `json:"version,omitempty"`
	APIVersion int    `json:"api_version,omitempty"`
	Error      string `json:"error,omitempty"`
}

func registerDeveloperToolsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		checks := moduleManifestChecks()
		valid := 0
		for _, check := range checks {
			if check.Valid {
				valid++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"manifest_count": len(checks), "manifest_valid": valid,
			"go_version": runtime.Version(), "architecture": runtime.GOARCH,
			"profiling_configured": fileExists("/opt/etc/routerforge/profiling.enabled"),
			"mutation_api":         false,
		})
	}))
	mux.HandleFunc("/v1/runtime", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		writeJSON(w, http.StatusOK, map[string]any{
			"go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
			"goroutines": runtime.NumGoroutine(), "gomaxprocs": runtime.GOMAXPROCS(0),
			"heap_alloc_bytes": mem.HeapAlloc, "heap_sys_bytes": mem.HeapSys,
			"stack_inuse_bytes": mem.StackInuse, "gc_cycles": mem.NumGC,
			"sampled_at": time.Now(),
		})
	}))
	mux.HandleFunc("/v1/manifests", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"manifests": moduleManifestChecks(), "limit": 32})
	}))
	mux.HandleFunc("/v1/platform", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"paths": []map[string]any{
				pathCheck("/opt/etc/routerforge", "configuration"),
				pathCheck("/opt/share/routerforge/modules", "module manifests"),
				pathCheck("/opt/var/run", "runtime sockets"),
				pathCheck("/opt/var/log", "logs"),
			},
			"profiling": map[string]any{
				"enabled": fileExists("/opt/etc/routerforge/profiling.enabled"),
				"config":  fileExists("/opt/etc/routerforge/profiling.conf"),
				"note":    "profiling remains loopback-only and is not proxied through this module",
			},
		})
	}))
}

func moduleManifestChecks() []manifestCheck {
	matches, _ := filepath.Glob("/opt/share/routerforge/modules/*/manifest.json")
	sort.Strings(matches)
	if len(matches) > 32 {
		matches = matches[:32]
	}
	out := make([]manifestCheck, 0, len(matches))
	for _, path := range matches {
		out = append(out, checkModuleManifest(path))
	}
	return out
}

func checkModuleManifest(path string) manifestCheck {
	check := manifestCheck{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		check.Error = "manifest is unavailable"
		return check
	}
	if info.Size() <= 0 || info.Size() > 64<<10 {
		check.Error = "manifest size is outside 1..65536 bytes"
		return check
	}
	data, err := os.ReadFile(path)
	if err != nil {
		check.Error = "manifest cannot be read"
		return check
	}
	var raw struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		Version       string `json:"version"`
		APIVersion    int    `json:"api_version"`
		Socket        string `json:"socket"`
		APIBase       string `json:"api_base"`
		UIEntry       string `json:"ui_entry"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		check.Error = "invalid JSON"
		return check
	}
	check.ID = strings.TrimSpace(raw.ID)
	check.Version = strings.TrimSpace(raw.Version)
	check.APIVersion = raw.APIVersion
	if raw.SchemaVersion != 1 || check.ID == "" || check.Version == "" || raw.APIVersion != 1 {
		check.Error = "required Module ABI v1 fields are missing or invalid"
		return check
	}
	if !strings.HasPrefix(raw.Socket, "/opt/var/run/routerforge-") || !strings.HasSuffix(raw.Socket, ".sock") {
		check.Error = "module socket is outside canonical RouterForge runtime namespace"
		return check
	}
	if raw.APIBase != "/api/modules/"+check.ID {
		check.Error = "api_base does not match module id"
		return check
	}
	if raw.UIEntry != raw.APIBase+"/ui/index.html" {
		check.Error = "ui_entry does not match module api_base"
		return check
	}
	check.Valid = true
	return check
}
