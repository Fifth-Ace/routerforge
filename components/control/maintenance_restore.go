package main

import (
	"archive/tar"
	"compress/gzip"
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
	adminMaintenanceConfigRoot       = "/opt/etc/routerforge"
	adminMaintenanceRestoreStageRoot = "/opt/tmp"
	adminMaintenanceArchiveMaxBytes  = 64 << 20
	adminMaintenanceArchiveMaxFiles  = 8192
	adminMaintenanceArchiveMaxEntry  = 16 << 20
)

type adminMaintenanceRestoreRequest struct {
	Path        string `json:"path"`
	ConfirmPath string `json:"confirm_path"`
	Confirm     string `json:"confirm"`
}

type adminMaintenanceBackupResult struct {
	OK         bool     `json:"ok"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	Sources    []string `json:"sources"`
	Files      int      `json:"files"`
	Bytes      int64    `json:"bytes"`
	ConfigOnly bool     `json:"config_only"`
}

type adminMaintenanceArchiveInspection struct {
	Valid          bool     `json:"valid"`
	ConfigEntries  int      `json:"config_entries"`
	ConfigBytes    int64    `json:"config_bytes"`
	IgnoredEntries int      `json:"ignored_entries"`
	Roots          []string `json:"roots"`
	Error          string   `json:"error,omitempty"`
}

type adminMaintenanceBackupInfo struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	ModifiedAt     string `json:"modified_at"`
	Valid          bool   `json:"valid"`
	ConfigEntries  int    `json:"config_entries"`
	ConfigBytes    int64  `json:"config_bytes"`
	IgnoredEntries int    `json:"ignored_entries"`
	Error          string `json:"error,omitempty"`
}

func handleAdminMaintenanceBackups(w http.ResponseWriter, _ *http.Request) {
	entries, err := os.ReadDir(adminMaintenanceBackupRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusOK, map[string]any{"backups": []adminMaintenanceBackupInfo{}})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	backups := make([]adminMaintenanceBackupInfo, 0)
	for _, entry := range entries {
		path := filepath.Join(adminMaintenanceBackupRoot, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !validMaintenanceBackupName(entry.Name()) {
			continue
		}
		inspection, inspectErr := inspectAdminMaintenanceArchive(path)
		item := adminMaintenanceBackupInfo{
			Path:       path,
			Name:       entry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		}
		if inspectErr != nil {
			item.Error = inspectErr.Error()
		} else {
			item.Valid = inspection.Valid
			item.ConfigEntries = inspection.ConfigEntries
			item.ConfigBytes = inspection.ConfigBytes
			item.IgnoredEntries = inspection.IgnoredEntries
			item.Error = inspection.Error
		}
		backups = append(backups, item)
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModifiedAt > backups[j].ModifiedAt
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"backups":      backups,
		"restore_mode": "config-only",
		"restore_root": adminMaintenanceConfigRoot,
	})
}

func createAdminMaintenanceConfigBackup() (adminMaintenanceBackupResult, error) {
	var result adminMaintenanceBackupResult
	sourceInfo, err := os.Stat(adminMaintenanceConfigRoot)
	if err != nil {
		return result, fmt.Errorf("config root unavailable: %w", err)
	}
	if !sourceInfo.IsDir() {
		return result, errors.New("config root is not a directory")
	}
	if err := os.MkdirAll(adminMaintenanceBackupRoot, 0700); err != nil {
		return result, err
	}

	name := fmt.Sprintf("routerforge-config-%d.tar.gz", time.Now().UnixNano())
	target := filepath.Join(adminMaintenanceBackupRoot, name)
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(target)
		}
	}()

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	var total int64
	files := 0

	walkErr := filepath.Walk(adminMaintenanceConfigRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup refuses symlink: %s", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("backup refuses special file: %s", path)
		}
		name := filepath.ToSlash(strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator)))
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if info.Size() > adminMaintenanceArchiveMaxEntry {
				return fmt.Errorf("backup entry exceeds %d bytes: %s", adminMaintenanceArchiveMaxEntry, path)
			}
			total += info.Size()
			files++
			if total > adminMaintenanceArchiveMaxBytes || files > adminMaintenanceArchiveMaxFiles {
				return errors.New("backup exceeds RouterForge safety limits")
			}
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tarWriter, input)
			closeErr := input.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	})
	if walkErr != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return result, walkErr
	}
	if err := tarWriter.Close(); err != nil {
		return result, err
	}
	if err := gzipWriter.Close(); err != nil {
		return result, err
	}
	if err := file.Sync(); err != nil {
		return result, err
	}
	if err := file.Close(); err != nil {
		return result, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return result, err
	}

	success = true
	return adminMaintenanceBackupResult{
		OK:         true,
		Path:       target,
		Size:       info.Size(),
		Sources:    []string{adminMaintenanceConfigRoot},
		Files:      files,
		Bytes:      total,
		ConfigOnly: true,
	}, nil
}

func handleAdminMaintenanceRestore(w http.ResponseWriter, r *http.Request) {
	var request adminMaintenanceRestoreRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid restore request"})
		return
	}
	path, err := resolveAdminMaintenanceBackupPath(request.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if request.ConfirmPath != path {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_path does not match restore path"})
		return
	}
	if request.Confirm != "RESTORE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal RESTORE"})
		return
	}

	inspection, err := inspectAdminMaintenanceArchive(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !inspection.Valid || inspection.ConfigEntries == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "archive has no valid RouterForge config payload"})
		return
	}

	safetyBackup, err := createAdminMaintenanceConfigBackup()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "pre-restore safety backup failed: " + err.Error()})
		return
	}

	stage, err := os.MkdirTemp(adminMaintenanceRestoreStageRoot, ".routerforge-restore-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer os.RemoveAll(stage)

	if err := extractAdminMaintenanceConfigArchive(path, stage); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "staging restore failed: " + err.Error()})
		return
	}

	stagedConfig := filepath.Join(stage, filepath.FromSlash("opt/etc/routerforge"))
	if info, err := os.Stat(stagedConfig); err != nil || !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "staged archive does not contain RouterForge config root"})
		return
	}

	rollbackPath := adminMaintenanceConfigRoot + fmt.Sprintf(".restore-old-%d", time.Now().UnixNano())
	hadCurrent := false
	if _, err := os.Stat(adminMaintenanceConfigRoot); err == nil {
		if err := os.Rename(adminMaintenanceConfigRoot, rollbackPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot stage current config for rollback: " + err.Error()})
			return
		}
		hadCurrent = true
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if err := os.Rename(stagedConfig, adminMaintenanceConfigRoot); err != nil {
		if hadCurrent {
			_ = os.Rename(rollbackPath, adminMaintenanceConfigRoot)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "restore swap failed: " + err.Error()})
		return
	}
	if hadCurrent {
		_ = os.RemoveAll(rollbackPath)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"restored_from":     path,
		"restored_root":     adminMaintenanceConfigRoot,
		"config_entries":    inspection.ConfigEntries,
		"config_bytes":      inspection.ConfigBytes,
		"ignored_entries":   inspection.IgnoredEntries,
		"safety_backup":     safetyBackup.Path,
		"restart_required":  true,
		"automatic_restart": false,
	})
}

func resolveAdminMaintenanceBackupPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || !filepath.IsAbs(clean) {
		return "", errors.New("backup path must be absolute")
	}
	if filepath.Dir(clean) != filepath.Clean(adminMaintenanceBackupRoot) {
		return "", errors.New("backup must be directly inside RouterForge backup directory")
	}
	if !validMaintenanceBackupName(filepath.Base(clean)) {
		return "", errors.New("invalid RouterForge backup name")
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("backup must be a regular non-symlink file")
	}
	return clean, nil
}

func validMaintenanceBackupName(name string) bool {
	return strings.HasPrefix(name, "routerforge-config-") && strings.HasSuffix(name, ".tar.gz") &&
		!strings.ContainsAny(name, `/\`)
}

func normalizeMaintenanceArchiveName(name string) (string, error) {
	name = filepath.ToSlash(strings.TrimSpace(name))
	for strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, "./")
	}
	if name == "" || strings.HasPrefix(name, "/") {
		return "", errors.New("archive contains absolute or empty path")
	}

	for _, component := range strings.Split(name, "/") {
		if component == ".." {
			return "", errors.New("archive contains path traversal")
		}
	}

	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", errors.New("archive contains path traversal")
	}
	return clean, nil
}

func isMaintenanceConfigArchivePath(name string) bool {
	return name == "opt/etc/routerforge" || strings.HasPrefix(name, "opt/etc/routerforge/")
}

func inspectAdminMaintenanceArchive(path string) (adminMaintenanceArchiveInspection, error) {
	var inspection adminMaintenanceArchiveInspection
	file, err := os.Open(path)
	if err != nil {
		return inspection, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return inspection, err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	roots := make(map[string]struct{})
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return inspection, err
		}
		name, err := normalizeMaintenanceArchiveName(header.Name)
		if err != nil {
			return inspection, err
		}
		if !isMaintenanceConfigArchivePath(name) {
			inspection.IgnoredEntries++
			continue
		}
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return inspection, fmt.Errorf("config archive contains unsupported entry type: %s", name)
		}
		if header.Size < 0 || header.Size > adminMaintenanceArchiveMaxEntry {
			return inspection, fmt.Errorf("config archive entry exceeds safety limit: %s", name)
		}
		inspection.ConfigEntries++
		inspection.ConfigBytes += header.Size
		if inspection.ConfigEntries > adminMaintenanceArchiveMaxFiles || inspection.ConfigBytes > adminMaintenanceArchiveMaxBytes {
			return inspection, errors.New("config archive exceeds RouterForge safety limits")
		}
		roots["opt/etc/routerforge"] = struct{}{}
	}
	for root := range roots {
		inspection.Roots = append(inspection.Roots, root)
	}
	sort.Strings(inspection.Roots)
	inspection.Valid = inspection.ConfigEntries > 0
	return inspection, nil
}

func extractAdminMaintenanceConfigArchive(path, stage string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	var total int64
	entries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name, err := normalizeMaintenanceArchiveName(header.Name)
		if err != nil {
			return err
		}
		if !isMaintenanceConfigArchivePath(name) {
			continue
		}
		entries++
		total += header.Size
		if entries > adminMaintenanceArchiveMaxFiles || total > adminMaintenanceArchiveMaxBytes ||
			header.Size < 0 || header.Size > adminMaintenanceArchiveMaxEntry {
			return errors.New("config archive exceeds RouterForge safety limits")
		}
		target := filepath.Join(stage, filepath.FromSlash(name))
		rel, err := filepath.Rel(stage, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return errors.New("restore target escaped staging root")
		}
		mode := os.FileMode(header.Mode) & 0777
		switch header.Typeflag {
		case tar.TypeDir:
			if mode == 0 {
				mode = 0755
			}
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if mode == 0 {
				mode = 0600
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(output, reader, header.Size)
			syncErr := output.Sync()
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if syncErr != nil {
				return syncErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported config archive entry type: %s", name)
		}
	}
}
