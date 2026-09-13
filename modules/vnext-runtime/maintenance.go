package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type fileMeta struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size_bytes"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"modified_at"`
}

func registerMaintenanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, maintenanceSummary())
	}))
	mux.HandleFunc("/v1/snapshots", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"snapshots": boundedFileMetadata("/opt/etc/routerforge/vault", 64)})
	}))
	mux.HandleFunc("/v1/logs", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"logs": routerForgeLogs()})
	}))
	mux.HandleFunc("/v1/tasks", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, maintenanceTasks())
	}))
}

func maintenanceSummary() map[string]any {
	var fs syscall.Statfs_t
	statErr := syscall.Statfs("/opt", &fs)
	mount := map[string]any{"path": "/opt", "available": statErr == nil}
	if statErr == nil {
		blockSize := uint64(fs.Bsize)
		mount["total_bytes"] = fs.Blocks * blockSize
		mount["free_bytes"] = fs.Bavail * blockSize
		mount["total_inodes"] = fs.Files
		mount["free_inodes"] = fs.Ffree
	}

	checks := []map[string]any{
		pathCheck("/opt/etc/init.d", "Entware init scripts"),
		pathCheck("/opt/lib/opkg/status", "OPKG database"),
		pathCheck("/opt/etc/routerforge", "RouterForge configuration"),
		pathCheck("/opt/var/log", "RouterForge logs"),
	}
	return map[string]any{
		"ok":             statErr == nil,
		"mount":          mount,
		"integrity":      checks,
		"snapshot_count": len(boundedFileMetadata("/opt/etc/routerforge/vault", 64)),
		"log_count":      len(routerForgeLogs()),
		"safe_mode":      "read-only",
	}
}

func pathCheck(path, label string) map[string]any {
	info, err := os.Stat(path)
	result := map[string]any{"label": label, "path": path, "present": err == nil}
	if err == nil {
		result["directory"] = info.IsDir()
		result["mode"] = info.Mode().String()
	}
	return result
}

func boundedFileMetadata(root string, limit int) []fileMeta {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []fileMeta{}
	}
	if limit <= 0 || limit > 256 {
		limit = 256
	}
	out := make([]fileMeta, 0, minInt(limit, len(entries)))
	for _, entry := range entries {
		if len(out) >= limit || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, fileMeta{
			Path: filepath.Join(root, entry.Name()), Size: info.Size(), Mode: info.Mode().String(), ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}

func routerForgeLogs() []fileMeta {
	patterns := []string{
		"/opt/var/log/routerforge*.log",
		"/opt/var/log/*routerforge*.log",
	}
	seen := map[string]bool{}
	out := make([]fileMeta, 0, 16)
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			if len(out) >= 16 || seen[path] {
				continue
			}
			seen[path] = true
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			out = append(out, fileMeta{Path: path, Size: info.Size(), Mode: info.Mode().String(), ModTime: info.ModTime()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}

func maintenanceTasks() map[string]any {
	paths := []string{"/opt/etc/crontab", "/opt/etc/cron.d", "/opt/etc/init.d"}
	checks := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		checks = append(checks, pathCheck(path, filepath.Base(path)))
	}
	watchdogs := make([]string, 0, 16)
	entries, _ := os.ReadDir("/opt/etc/init.d")
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if strings.Contains(name, "watch") || strings.Contains(name, "health") {
			watchdogs = append(watchdogs, entry.Name())
			if len(watchdogs) >= 16 {
				break
			}
		}
	}
	return map[string]any{"sources": checks, "watchdog_candidates": watchdogs, "mutation_api": false}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
