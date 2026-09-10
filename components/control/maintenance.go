package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	adminMaintenanceLogLimit   = 64 << 10
	adminMaintenanceTaskLimit  = 512
	adminMaintenanceBackupRoot = "/tmp/routerforge-backups"
)

type adminMaintenanceBackupRequest struct {
	Confirm string `json:"confirm"`
}

func registerAdminMaintenanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/logs", getOnly(handleAdminMaintenanceLogs))
	mux.HandleFunc("/v1/maintenance/tasks", getOnly(handleAdminMaintenanceTasks))
	mux.HandleFunc("/v1/maintenance/backups", getOnly(handleAdminMaintenanceBackups))
	mux.HandleFunc("/v1/maintenance/backup", mutationOnly(handleAdminMaintenanceBackup))
	mux.HandleFunc("/v1/maintenance/restore", mutationOnly(handleAdminMaintenanceRestore))
	registerAdminWatchdogRoutes(mux)
	registerAdminSnapshotRoutes(mux)
}

func handleAdminMaintenanceLogs(w http.ResponseWriter, _ *http.Request) {
	candidates := []string{
		"/opt/var/log/routerforge.log",
		"/opt/var/log/messages",
		"/var/log/messages",
	}
	for _, path := range candidates {
		content, err := readTailFile(path, adminMaintenanceLogLimit)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"source":    path,
				"content":   content,
				"max_bytes": adminMaintenanceLogLimit,
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":    "",
		"content":   "",
		"max_bytes": adminMaintenanceLogLimit,
	})
}

func readTailFile(path string, limit int64) (string, error) {
	if limit <= 0 {
		return "", nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	offset := info.Size() - limit
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return "", err
	}

	content, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func handleAdminMaintenanceTasks(w http.ResponseWriter, _ *http.Request) {
	paths := []string{
		"/opt/etc/crontab",
		"/opt/etc/cron.d",
		"/opt/var/spool/cron/crontabs",
	}
	type task struct {
		Source string `json:"source"`
		Line   string `json:"line"`
	}
	result := make([]task, 0)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		files := []string{}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.Type().IsRegular() {
					files = append(files, filepath.Join(path, entry.Name()))
				}
			}
		} else if info.Mode().IsRegular() {
			files = append(files, path)
		}
		sort.Strings(files)
		for _, filePath := range files {
			file, err := os.Open(filePath)
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(file)
			for scanner.Scan() && len(result) < adminMaintenanceTaskLimit {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				result = append(result, task{Source: filePath, Line: line})
			}
			file.Close()
			if len(result) >= adminMaintenanceTaskLimit {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tasks":       result,
		"max_entries": adminMaintenanceTaskLimit,
	})
}

func handleAdminMaintenanceBackup(w http.ResponseWriter, r *http.Request) {
	var request adminMaintenanceBackupRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid backup request"})
		return
	}
	if request.Confirm != "BACKUP" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal BACKUP"})
		return
	}

	result, err := createAdminMaintenanceConfigBackup()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
