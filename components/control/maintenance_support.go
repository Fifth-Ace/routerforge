package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	adminSupportBundleRoot      = "/tmp/routerforge-support"
	adminSupportBundleMaxFiles  = 8
	adminSupportBundleMaxBytes  = 4 << 20
	adminSupportBundleLogBytes  = 128 << 10
	adminSupportBundleMaxSource = 2 << 20
)

type adminSupportBundleRequest struct {
	Confirm string `json:"confirm"`
}

type adminSupportBundleInfo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type adminSupportStorageHealth struct {
	Device     string  `json:"device"`
	Mount      string  `json:"mount"`
	FSType     string  `json:"fs_type"`
	UsedPct    float64 `json:"used_pct"`
	FreeBytes  uint64  `json:"free_bytes"`
	TotalBytes uint64  `json:"total_bytes"`
	Status     string  `json:"status"`
}

type adminSupportStatus struct {
	GeneratedAt        time.Time                   `json:"generated_at"`
	Overall            string                      `json:"overall"`
	Storage            []adminSupportStorageHealth `json:"storage"`
	BackupCount        int                         `json:"backup_count"`
	LatestBackup       string                      `json:"latest_backup,omitempty"`
	SnapshotCount      int                         `json:"snapshot_count"`
	WatchdogsEnabled   int                         `json:"watchdogs_enabled"`
	WatchdogsAttention int                         `json:"watchdogs_attention"`
	SupportBundleCount int                         `json:"support_bundle_count"`
}

func registerAdminSupportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/support/status", getOnly(handleAdminSupportStatus))
	mux.HandleFunc("/v1/maintenance/support-bundles", getOnly(handleAdminSupportBundles))
	mux.HandleFunc("/v1/maintenance/support-bundle", mutationOnly(handleSupportV2Create))
	registerSupportV2Routes(mux)
	mux.HandleFunc("/v1/maintenance/support-bundle/download", handleAdminSupportBundleDownload)
}

func handleAdminSupportStatus(w http.ResponseWriter, _ *http.Request) {
	status, err := buildAdminSupportStatus(time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func handleAdminSupportBundles(w http.ResponseWriter, _ *http.Request) {
	bundles, err := listAdminSupportBundles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bundles":   bundles,
		"max_files": adminSupportBundleMaxFiles,
		"max_bytes": adminSupportBundleMaxBytes,
		"redacted":  false, // legacy inventory may contain unverified archives
		"privacy":   "mixed-legacy-and-v2",
	})
}

func handleAdminSupportBundleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or HEAD required"})
		return
	}
	path, err := resolveAdminSupportBundlePath(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	file, err := supportOpen(adminSupportBundleRoot, filepath.Base(path))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "support bundle not found"})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.Name()))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func buildAdminSupportStatus(now time.Time) (adminSupportStatus, error) {
	storage := readStorage()
	storageHealth := make([]adminSupportStorageHealth, 0, len(storage))
	overall := "ok"
	for _, item := range storage {
		state := classifyAdminSupportStorage(item.UsedPct)
		if !adminSupportStorageMonitored(item) {
			state = "ignored"
		}
		if state == "critical" {
			overall = "critical"
		} else if state == "warning" && overall == "ok" {
			overall = "warning"
		}
		storageHealth = append(storageHealth, adminSupportStorageHealth{
			Device:     item.Device,
			Mount:      item.Mount,
			FSType:     item.FSType,
			UsedPct:    item.UsedPct,
			FreeBytes:  item.FreeBytes,
			TotalBytes: item.TotalBytes,
			Status:     state,
		})
	}

	backupCount, latestBackup := supportBackupSummary()
	snapshots, err := listAdminSnapshots()
	if err != nil {
		return adminSupportStatus{}, err
	}
	bundles, err := listAdminSupportBundles()
	if err != nil {
		return adminSupportStatus{}, err
	}
	watchdogs := adminWatchdogs.statuses(now)
	enabled := 0
	attention := 0
	for _, watchdog := range watchdogs {
		if watchdog.Enabled {
			enabled++
			if watchdog.Detected && !watchdog.Running {
				attention++
			}
		}
	}
	if attention > 0 && overall == "ok" {
		overall = "warning"
	}

	return adminSupportStatus{
		GeneratedAt:        now,
		Overall:            overall,
		Storage:            storageHealth,
		BackupCount:        backupCount,
		LatestBackup:       latestBackup,
		SnapshotCount:      len(snapshots),
		WatchdogsEnabled:   enabled,
		WatchdogsAttention: attention,
		SupportBundleCount: len(bundles),
	}, nil
}

func adminSupportStorageMonitored(item storageInfo) bool {
	if item.TotalBytes == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(item.FSType)) {
	case "squashfs", "proc", "sysfs", "devpts", "debugfs", "tmpfs", "devtmpfs",
		"ramfs", "overlay", "cgroup", "cgroup2", "pstore", "securityfs", "tracefs",
		"configfs", "fusectl", "mqueue", "autofs", "bpf":
		return false
	default:
		return true
	}
}

func classifyAdminSupportStorage(usedPct float64) string {
	switch {
	case usedPct >= 90:
		return "critical"
	case usedPct >= 80:
		return "warning"
	default:
		return "ok"
	}
}

func supportBackupSummary() (int, string) {
	entries, err := os.ReadDir(adminMaintenanceBackupRoot)
	if err != nil {
		return 0, ""
	}
	count := 0
	var latest time.Time
	for _, entry := range entries {
		if !validMaintenanceBackupName(entry.Name()) {
			continue
		}
		path := filepath.Join(adminMaintenanceBackupRoot, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		count++
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	if latest.IsZero() {
		return count, ""
	}
	return count, latest.UTC().Format(time.RFC3339)
}

func redactAdminSupportText(input string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	result := make([]string, 0, len(lines))
	inPrivateKey := false

	for _, line := range lines {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "-----BEGIN") && strings.Contains(upper, "PRIVATE KEY-----") {
			inPrivateKey = true
			result = append(result, "[REDACTED PRIVATE KEY BLOCK]")
			continue
		}
		if inPrivateKey {
			if strings.Contains(upper, "-----END") && strings.Contains(upper, "PRIVATE KEY-----") {
				inPrivateKey = false
			}
			continue
		}

		redacted := line
		lower := strings.ToLower(redacted)
		for _, key := range []string{
			"password", "passwd", "token", "secret", "api_key", "api-key",
			"privatekey", "private_key", "private-key",
			"presharedkey", "preshared_key", "preshared-key", "authorization",
		} {
			index := strings.Index(lower, key)
			if index < 0 {
				continue
			}
			tail := redacted[index+len(key):]
			separator := strings.IndexAny(tail, ":=")
			if separator < 0 {
				continue
			}
			end := index + len(key) + separator + 1
			redacted = redacted[:end] + " [REDACTED]"
			lower = strings.ToLower(redacted)
		}
		if bearer := strings.Index(strings.ToLower(redacted), "bearer "); bearer >= 0 {
			redacted = redacted[:bearer] + "Bearer [REDACTED]"
		}
		result = append(result, redacted)
	}
	return strings.Join(result, "\n")
}

func listAdminSupportBundles() ([]adminSupportBundleInfo, error) {
	rows, err := supportInventory(adminSupportBundleRoot)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ModifiedAt.After(rows[j].ModifiedAt) })
	return rows, nil
}

func resolveAdminSupportBundlePath(path string) (string, error) {
	if err := supportRootCheck(adminSupportBundleRoot, false); err != nil {
		return "", err
	}
	clean := filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(clean) || filepath.Dir(clean) != filepath.Clean(adminSupportBundleRoot) {
		return "", errors.New("support bundle must be directly inside RouterForge support directory")
	}
	if !validAdminSupportBundleName(filepath.Base(clean)) {
		return "", errors.New("invalid RouterForge support bundle name")
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("support bundle must be a regular non-symlink file")
	}
	return clean, nil
}

func validAdminSupportBundleName(name string) bool { return supportNameRE.MatchString(name) }

// Compile-time check that ServeContent's io.Seeker requirement stays explicit.
var _ io.Seeker = (*os.File)(nil)
