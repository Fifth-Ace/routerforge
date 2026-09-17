package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
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
	mux.HandleFunc("/v1/maintenance/support-bundle", mutationOnly(handleAdminSupportBundleCreate))
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
		"redacted":  true,
	})
}

func handleAdminSupportBundleCreate(w http.ResponseWriter, r *http.Request) {
	var request adminSupportBundleRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid support bundle request"})
		return
	}
	if request.Confirm != "SUPPORT_BUNDLE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal SUPPORT_BUNDLE"})
		return
	}
	bundle, err := createAdminSupportBundle(time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"bundle":   bundle,
		"redacted": true,
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
	file, err := os.Open(path)
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

func createAdminSupportBundle(now time.Time) (adminSupportBundleInfo, error) {
	status, err := buildAdminSupportStatus(now)
	if err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := os.MkdirAll(adminSupportBundleRoot, 0700); err != nil {
		return adminSupportBundleInfo{}, err
	}
	name := fmt.Sprintf("routerforge-support-%d.zip", now.UnixNano())
	path := filepath.Join(adminSupportBundleRoot, name)
	file, err := safety.CreateExclusiveFile(path, 0600)
	if err != nil {
		return adminSupportBundleInfo{}, err
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()

	writer := zip.NewWriter(file)
	total := int64(0)
	addJSON := func(entry string, value any) error {
		content, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		content = append(content, '\n')
		return writeAdminSupportZipEntry(writer, entry, content, &total)
	}
	addText := func(entry, value string) error {
		return writeAdminSupportZipEntry(writer, entry, []byte(value), &total)
	}

	hostname, _ := os.Hostname()
	manifest := map[string]any{
		"schema_version": 1,
		"created_at":     now,
		"routerforge": map[string]any{
			"component": "admin",
			"version":   version,
		},
		"device": map[string]any{
			"hostname":     hostname,
			"architecture": readSummary().Architecture,
		},
		"privacy": map[string]any{
			"redacted":          true,
			"config_files":      false,
			"process_arguments": false,
			"cron_commands":     false,
		},
		"contents": []string{
			"summary.json",
			"storage.json",
			"thermal.json",
			"services.json",
			"integrations.json",
			"watchdogs.json",
			"status.json",
			"logs.txt",
			"README.txt",
		},
	}
	if err := addJSON("manifest.json", manifest); err != nil {
		return adminSupportBundleInfo{}, err
	}

	if err := addJSON("status.json", status); err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := addJSON("summary.json", readSummary()); err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := addJSON("storage.json", readStorage()); err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := addJSON("thermal.json", readThermals()); err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := addJSON("services.json", readServices()); err != nil {
		return adminSupportBundleInfo{}, err
	}

	processes := readProcesses()
	services := readServices()
	ports := readPorts()
	integrations := make([]adminIntegrationInfo, 0, len(adminIntegrationDefinitions))
	for _, definition := range adminIntegrationDefinitions {
		integrations = append(integrations, detectAdminIntegration(definition, processes, services, ports))
	}
	if err := addJSON("integrations.json", integrations); err != nil {
		return adminSupportBundleInfo{}, err
	}

	watchdogs := adminWatchdogs.statuses(now)
	for i := range watchdogs {
		watchdogs[i].LastAttemptOutput = redactAdminSupportText(watchdogs[i].LastAttemptOutput)
	}
	if err := addJSON("watchdogs.json", watchdogs); err != nil {
		return adminSupportBundleInfo{}, err
	}

	logSource, logContent := adminSupportLogTail()
	logHeader := "# RouterForge redacted support log\n# source: " + logSource + "\n\n"
	if err := addText("logs.txt", logHeader+redactAdminSupportText(logContent)); err != nil {
		return adminSupportBundleInfo{}, err
	}

	readme := "RouterForge support bundle\n\n" +
		"This archive contains bounded diagnostic metadata only.\n" +
		"RouterForge config files, process arguments and cron command bodies are intentionally excluded.\n" +
		"Known credential-like values and private-key blocks are redacted from included log text.\n"
	if err := addText("README.txt", readme); err != nil {
		return adminSupportBundleInfo{}, err
	}

	if err := writer.Close(); err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := file.Sync(); err != nil {
		return adminSupportBundleInfo{}, err
	}
	if err := file.Close(); err != nil {
		return adminSupportBundleInfo{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return adminSupportBundleInfo{}, err
	}
	if info.Size() > adminSupportBundleMaxBytes {
		return adminSupportBundleInfo{}, fmt.Errorf("support bundle exceeds %d bytes", adminSupportBundleMaxBytes)
	}
	if err := pruneAdminSupportBundles(); err != nil {
		return adminSupportBundleInfo{}, err
	}
	success = true
	return adminSupportBundleInfo{
		Path:       path,
		Name:       name,
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC(),
	}, nil
}

func writeAdminSupportZipEntry(writer *zip.Writer, name string, content []byte, total *int64) error {
	if int64(len(content)) > adminSupportBundleMaxSource {
		return fmt.Errorf("support bundle entry %s exceeds %d bytes", name, adminSupportBundleMaxSource)
	}
	*total += int64(len(content))
	if *total > adminSupportBundleMaxBytes {
		return fmt.Errorf("support bundle source data exceeds %d bytes", adminSupportBundleMaxBytes)
	}
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0600)
	header.Modified = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(content)
	return err
}

func adminSupportLogTail() (string, string) {
	for _, path := range []string{
		"/opt/var/log/routerforge.log",
		"/opt/var/log/messages",
		"/var/log/messages",
	} {
		content, err := readTailFile(path, adminSupportBundleLogBytes)
		if err == nil {
			return path, content
		}
	}
	return "", ""
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
	entries, err := os.ReadDir(adminSupportBundleRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []adminSupportBundleInfo{}, nil
		}
		return nil, err
	}
	result := make([]adminSupportBundleInfo, 0)
	for _, entry := range entries {
		if !validAdminSupportBundleName(entry.Name()) {
			continue
		}
		path := filepath.Join(adminSupportBundleRoot, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		result = append(result, adminSupportBundleInfo{
			Path:       path,
			Name:       entry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModifiedAt.After(result[j].ModifiedAt)
	})
	return result, nil
}

func pruneAdminSupportBundles() error {
	bundles, err := listAdminSupportBundles()
	if err != nil {
		return err
	}
	for len(bundles) > adminSupportBundleMaxFiles {
		oldest := bundles[len(bundles)-1]
		if err := os.Remove(oldest.Path); err != nil {
			return err
		}
		bundles = bundles[:len(bundles)-1]
	}
	return nil
}

func resolveAdminSupportBundlePath(path string) (string, error) {
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

func validAdminSupportBundleName(name string) bool {
	return strings.HasPrefix(name, "routerforge-support-") &&
		strings.HasSuffix(name, ".zip") &&
		!strings.ContainsAny(name, `/\`)
}

// Compile-time check that ServeContent's io.Seeker requirement stays explicit.
var _ io.Seeker = (*os.File)(nil)
