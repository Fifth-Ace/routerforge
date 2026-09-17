package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	nfqws2ConfigPath      = "/opt/etc/nfqws2/nfqws2.conf"
	nfqws2InitPath        = "/opt/etc/init.d/S51nfqws2"
	nfqws2ListsRoot       = "/opt/etc/nfqws2/lists"
	nfqws2LogPath         = "/opt/var/log/nfqws2.log"
	nfqws2BackupRoot      = "/tmp/routerforge-nfqws2-backups"
	nfqws2ConfigMaxBytes  = 128 << 10
	nfqws2PreviewMaxBytes = 32 << 10
	nfqws2LogTailMaxBytes = 64 << 10
	nfqws2BackupMaxFiles  = 8
	nfqws2ActionTimeout   = 20 * time.Second
)

type nfqws2FilePreview struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified_at,omitempty"`
	Preview   string `json:"preview,omitempty"`
	Truncated bool   `json:"truncated"`
}

type nfqws2ManagerStatus struct {
	Detected      bool                `json:"detected"`
	Running       bool                `json:"running"`
	ServiceID     string              `json:"service_id,omitempty"`
	InitScript    string              `json:"init_script"`
	ConfigPath    string              `json:"config_path"`
	ConfigExists  bool                `json:"config_exists"`
	ConfigSize    int64               `json:"config_size"`
	ConfigSHA256  string              `json:"config_sha256,omitempty"`
	Config        string              `json:"config,omitempty"`
	ConfigCut     bool                `json:"config_truncated"`
	ListsRoot     string              `json:"lists_root"`
	Lists         []nfqws2FilePreview `json:"lists"`
	LogPath       string              `json:"log_path"`
	LogTail       string              `json:"log_tail,omitempty"`
	LogCut        bool                `json:"log_truncated"`
	BackupCount   int                 `json:"backup_count"`
	MutationReady bool                `json:"mutation_ready"`
}

type nfqws2ActionRequest struct {
	Action  string `json:"action"`
	Confirm string `json:"confirm"`
}

type nfqws2ConfigRequest struct {
	Content string `json:"content"`
	Confirm string `json:"confirm"`
}

func registerNFQWS2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/integrations/nfqws2", getOnly(handleNFQWS2Status))
	mux.HandleFunc("/v1/integrations/nfqws2/action", mutationOnly(handleNFQWS2Action))
	mux.HandleFunc("/v1/integrations/nfqws2/config", mutationOnly(handleNFQWS2Config))
}

func handleNFQWS2Status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readNFQWS2ManagerStatus())
}

func readNFQWS2ManagerStatus() nfqws2ManagerStatus {
	processes := readProcesses()
	services := readServices()
	ports := readPorts()
	var definition adminIntegrationDefinition
	for _, candidate := range adminIntegrationDefinitions {
		if candidate.ID == "nfqws2" {
			definition = candidate
			break
		}
	}
	info := detectAdminIntegration(definition, processes, services, ports)

	status := nfqws2ManagerStatus{
		Detected:   info.Detected,
		Running:    info.Running,
		ServiceID:  info.ServiceID,
		InitScript: nfqws2InitPath,
		ConfigPath: nfqws2ConfigPath,
		ListsRoot:  nfqws2ListsRoot,
		Lists:      []nfqws2FilePreview{},
		LogPath:    nfqws2LogPath,
	}

	if data, cut, err := readBoundedFile(nfqws2ConfigPath, nfqws2ConfigMaxBytes); err == nil {
		status.ConfigExists = true
		status.ConfigSize = int64(len(data))
		sum := sha256.Sum256(data)
		status.ConfigSHA256 = hex.EncodeToString(sum[:])
		status.Config = string(data)
		status.ConfigCut = cut
	}
	status.Lists = readNFQWS2ListPreviews()
	if data, cut, err := readTailBounded(nfqws2LogPath, nfqws2LogTailMaxBytes); err == nil {
		status.LogTail = string(data)
		status.LogCut = cut
	}
	status.BackupCount = countNFQWS2Backups()
	status.MutationReady = nfqws2MutationReady()
	return status
}

func nfqws2MutationReady() bool {
	info, err := os.Stat(nfqws2InitPath)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return false
	}
	configInfo, err := os.Stat(nfqws2ConfigPath)
	return err == nil && !configInfo.IsDir()
}

func readBoundedFile(path string, limit int64) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Size() > limit {
		data := make([]byte, limit)
		n, readErr := file.Read(data)
		if readErr != nil && n == 0 {
			return nil, false, readErr
		}
		return data[:n], true, nil
	}
	data, err := os.ReadFile(path)
	return data, false, err
}

func readTailBounded(path string, limit int64) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	start := int64(0)
	cut := false
	if info.Size() > limit {
		start = info.Size() - limit
		cut = true
	}
	if _, err := file.Seek(start, 0); err != nil {
		return nil, false, err
	}
	data := make([]byte, info.Size()-start)
	n, err := file.Read(data)
	if err != nil && n == 0 {
		return nil, false, err
	}
	return data[:n], cut, nil
}

func readNFQWS2ListPreviews() []nfqws2FilePreview {
	entries, err := os.ReadDir(nfqws2ListsRoot)
	if err != nil {
		return []nfqws2FilePreview{}
	}
	out := make([]nfqws2FilePreview, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !safeNFQWS2ListName(name) {
			continue
		}
		path := filepath.Join(nfqws2ListsRoot, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}
		data, cut, err := readBoundedFile(path, nfqws2PreviewMaxBytes)
		if err != nil {
			continue
		}
		out = append(out, nfqws2FilePreview{
			Name: name, Path: path, Size: info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
			Preview:  string(data), Truncated: cut,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func safeNFQWS2ListName(name string) bool {
	if name == "" || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validateNFQWS2Config(content string) error {
	if content == "" {
		return errors.New("nfqws2 config must not be empty")
	}
	if len(content) > nfqws2ConfigMaxBytes {
		return fmt.Errorf("nfqws2 config exceeds %d bytes", nfqws2ConfigMaxBytes)
	}
	if strings.IndexByte(content, 0) >= 0 {
		return errors.New("nfqws2 config contains NUL byte")
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	line := 0
	for scanner.Scan() {
		line++
		if len(scanner.Bytes()) > 8192 {
			return fmt.Errorf("nfqws2 config line %d exceeds 8192 bytes", line)
		}
	}
	return scanner.Err()
}

func handleNFQWS2Action(w http.ResponseWriter, r *http.Request) {
	var request nfqws2ActionRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid nfqws2 action request"})
		return
	}
	if request.Confirm != "NFQWS2" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS2"})
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "reload" && action != "restart" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "allowed actions: reload, restart"})
		return
	}
	if !nfqws2MutationReady() {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 installed runtime is not ready for mutation"})
		return
	}
	output, err := runNFQWS2Init(action)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "output": output})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "action": action, "output": output, "status": readNFQWS2ManagerStatus(),
	})
}

func handleNFQWS2Config(w http.ResponseWriter, r *http.Request) {
	var request nfqws2ConfigRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid nfqws2 config request"})
		return
	}
	if request.Confirm != "NFQWS2_CONFIG" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS2_CONFIG"})
		return
	}
	if err := validateNFQWS2Config(request.Content); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !nfqws2MutationReady() {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 installed runtime is not ready for mutation"})
		return
	}

	before, err := os.ReadFile(nfqws2ConfigPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read current nfqws2 config: " + err.Error()})
		return
	}
	info, err := os.Stat(nfqws2ConfigPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat current nfqws2 config: " + err.Error()})
		return
	}
	backup, err := createNFQWS2Backup(before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create nfqws2 safety backup: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(nfqws2ConfigPath, []byte(request.Content), info.Mode().Perm()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "publish nfqws2 config: " + err.Error(), "backup": backup})
		return
	}
	output, reloadErr := runNFQWS2Init("reload")
	if reloadErr != nil {
		rollbackErr := safety.WriteFileAtomic(nfqws2ConfigPath, before, info.Mode().Perm())
		rollbackOutput := ""
		if rollbackErr == nil {
			rollbackOutput, _ = runNFQWS2Init("reload")
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": reloadErr.Error(), "output": output, "backup": backup,
			"rolled_back": rollbackErr == nil, "rollback_error": errorString(rollbackErr),
			"rollback_output": rollbackOutput,
		})
		return
	}
	_ = pruneNFQWS2Backups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "backup": backup, "reload_output": output, "status": readNFQWS2ManagerStatus(),
	})
}

func runNFQWS2Init(action string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nfqws2ActionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, nfqws2InitPath, action)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if ctx.Err() != nil {
		return text, fmt.Errorf("nfqws2 %s timed out", action)
	}
	if err != nil {
		return text, fmt.Errorf("nfqws2 %s failed: %w", action, err)
	}
	return text, nil
}

func createNFQWS2Backup(data []byte, mode os.FileMode) (string, error) {
	if err := os.MkdirAll(nfqws2BackupRoot, 0700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("nfqws2-%d.conf", time.Now().UTC().UnixNano())
	path := filepath.Join(nfqws2BackupRoot, name)
	if err := safety.WriteFileAtomic(path, data, mode); err != nil {
		return "", err
	}
	return path, nil
}

func countNFQWS2Backups() int {
	entries, err := os.ReadDir(nfqws2BackupRoot)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "nfqws2-") && strings.HasSuffix(entry.Name(), ".conf") {
			count++
		}
	}
	return count
}

func pruneNFQWS2Backups() error {
	entries, err := os.ReadDir(nfqws2BackupRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	type item struct {
		path string
		time time.Time
	}
	var items []item
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "nfqws2-") || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, item{path: filepath.Join(nfqws2BackupRoot, entry.Name()), time: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].time.After(items[j].time) })
	for i := nfqws2BackupMaxFiles; i < len(items); i++ {
		if err := os.Remove(items[i].path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
