package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	mux.HandleFunc("/v1/maintenance/backup", mutationOnly(handleAdminMaintenanceBackup))
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
	content, err := os.ReadFile(path)
	if err == nil && int64(len(content)) <= limit {
		return string(content), nil
	}

	buf := make([]byte, limit)
	n, err := file.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
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
	if err := os.MkdirAll(adminMaintenanceBackupRoot, 0700); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	name := fmt.Sprintf("routerforge-config-%d.tar.gz", time.Now().Unix())
	target := filepath.Join(adminMaintenanceBackupRoot, name)

	args := []string{"-czf", target}
	sources := make([]string, 0, 2)
	for _, candidate := range []string{"/opt/etc/routerforge", "/opt/share/routerforge"} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			args = append(args, candidate)
			sources = append(sources, candidate)
		}
	}
	if len(sources) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no RouterForge configuration directories found"})
		return
	}
	cmd := exec.Command("tar", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "backup command failed",
			"output": string(output[:minInt(len(output), adminMutationResponseOutputLimit)]),
		})
		return
	}
	if err := os.Chmod(target, 0600); err != nil {
		_ = os.Remove(target)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"path":    target,
		"size":    info.Size(),
		"sources": sources,
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
