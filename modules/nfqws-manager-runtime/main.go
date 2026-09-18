package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

var version = "dev"

const (
	configPath       = "/opt/etc/nfqws2/nfqws2.conf"
	initPath         = "/opt/etc/init.d/S51nfqws2"
	listsRoot        = "/opt/etc/nfqws2/lists"
	logPath          = "/opt/var/log/nfqws2.log"
	backupRoot       = "/tmp/routerforge-nfqws-manager-backups"
	configMaxBytes   = 128 << 10
	previewMaxBytes  = 32 << 10
	listFileMaxBytes = 2 << 20
	logTailMaxBytes  = 64 << 10
	backupMaxFiles   = 8
	actionTimeout    = 20 * time.Second
	authHeader       = "X-RouterForge-Module-Authorized"
	authValue        = "core-authorized-v1"
	defaultSocket    = "/opt/var/run/routerforge-nfqws-manager.sock"
)

type filePreview struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified_at,omitempty"`
	Preview   string `json:"preview,omitempty"`
	Truncated bool   `json:"truncated"`
}

type managerStatus struct {
	Available     bool          `json:"available"`
	TargetID      string        `json:"target_id"`
	TargetName    string        `json:"target_name"`
	Running       bool          `json:"running"`
	InitScript    string        `json:"init_script"`
	ConfigPath    string        `json:"config_path"`
	ConfigExists  bool          `json:"config_exists"`
	ConfigSize    int64         `json:"config_size"`
	ConfigSHA256  string        `json:"config_sha256,omitempty"`
	Config        string        `json:"config,omitempty"`
	ConfigCut     bool          `json:"config_truncated"`
	ListsRoot     string        `json:"lists_root"`
	Lists         []filePreview `json:"lists"`
	LogPath       string        `json:"log_path"`
	LogTail       string        `json:"log_tail,omitempty"`
	LogCut        bool          `json:"log_truncated"`
	BackupCount   int           `json:"backup_count"`
	MutationReady bool          `json:"mutation_ready"`
}

type actionRequest struct {
	Action  string `json:"action"`
	Confirm string `json:"confirm"`
}

type configRequest struct {
	Content string `json:"content"`
	Confirm string `json:"confirm"`
}

type listRequest struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
	Confirm string `json:"confirm"`
}

type checkRequest struct {
	URL string `json:"url"`
}

type checkResponse struct {
	Reachable  bool   `json:"reachable"`
	StatusCode int    `json:"status_code,omitempty"`
	URL        string `json:"url"`
	Error      string `json:"error,omitempty"`
}

func main() {
	socket := flagValue("-socket", defaultSocket)
	uiPath := flagValue("-ui", "/opt/share/routerforge/modules/nfqws-manager/ui")

	if err := os.MkdirAll(filepath.Dir(socket), 0755); err != nil {
		panic(err)
	}
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	defer os.Remove(socket)
	_ = os.Chmod(socket, 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		status := readStatus()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":             true,
			"module":         "nfqws-manager",
			"version":        version,
			"api_version":    1,
			"mode":           "integration-manager",
			"mutation_api":   true,
			"mutation_auth":  "core-guarded",
			"available":      status.Available,
			"target_id":      status.TargetID,
			"target_running": status.Running,
		})
	}))
	mux.HandleFunc("/v1/status", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, readStatus())
	}))
	mux.HandleFunc("/v1/action", mutationOnly(handleAction))
	mux.HandleFunc("/v1/config", mutationOnly(handleConfig))
	mux.HandleFunc("/v1/list", getOnly(handleListRead))
	mux.HandleFunc("/v1/list/create", mutationOnly(handleListCreate))
	mux.HandleFunc("/v1/list/save", mutationOnly(handleListSave))
	mux.HandleFunc("/v1/list/delete", mutationOnly(handleListDelete))
	mux.HandleFunc("/v1/check", mutationOnly(handleCheck))

	ui := http.FileServer(http.Dir(uiPath))
	mux.HandleFunc("/v1/ui", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/v1/ui/index.html", http.StatusTemporaryRedirect)
	})
	mux.Handle("/v1/ui/", http.StripPrefix("/v1/ui/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		ui.ServeHTTP(w, r)
	})))

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func flagValue(name, fallback string) string {
	for i := 1; i+1 < len(os.Args); i++ {
		if os.Args[i] == name && strings.TrimSpace(os.Args[i+1]) != "" {
			return os.Args[i+1]
		}
	}
	return fallback
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
			return
		}
		next(w, r)
	}
}

func mutationOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required", "mutation_api": true})
			return
		}
		if r.Header.Get(authHeader) != authValue {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "authorized RouterForge Core request required", "mutation_api": true})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, listFileMaxBytes+(64<<10))
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func readStatus() managerStatus {
	status := managerStatus{
		TargetID: "nfqws2", TargetName: "nfqws2-keenetic",
		InitScript: initPath, ConfigPath: configPath, ListsRoot: listsRoot,
		Lists: []filePreview{}, LogPath: logPath,
	}
	initInfo, initErr := os.Stat(initPath)
	configInfo, configErr := os.Stat(configPath)
	status.Available = initErr == nil && !initInfo.IsDir() && configErr == nil && !configInfo.IsDir()
	status.Running = nfqws2Running()
	status.MutationReady = status.Available && initInfo.Mode()&0111 != 0

	if data, cut, err := readBoundedFile(configPath, configMaxBytes); err == nil {
		status.ConfigExists = true
		status.ConfigSize = int64(len(data))
		sum := sha256.Sum256(data)
		status.ConfigSHA256 = hex.EncodeToString(sum[:])
		status.Config = string(data)
		status.ConfigCut = cut
	}
	status.Lists = readListPreviews()
	if data, cut, err := readTailBounded(logPath, logTailMaxBytes); err == nil {
		status.LogTail = string(data)
		status.LogCut = cut
	}
	status.BackupCount = countBackups()
	return status
}

func nfqws2Running() bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, file := range []string{"comm", "cmdline"} {
			data, err := os.ReadFile(filepath.Join("/proc", name, file))
			if err != nil {
				continue
			}
			text := strings.ToLower(strings.ReplaceAll(string(data), "\x00", " "))
			if strings.Contains(text, "nfqws2") {
				return true
			}
		}
	}
	return false
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
	reader := io.LimitReader(file, limit+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, info.Size() > limit, nil
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
	data, err := io.ReadAll(io.LimitReader(file, limit))
	return data, cut, err
}

func readListPreviews() []filePreview {
	entries, err := os.ReadDir(listsRoot)
	if err != nil {
		return []filePreview{}
	}
	out := make([]filePreview, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !safeListName(entry.Name()) {
			continue
		}
		path := filepath.Join(listsRoot, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		data, cut, err := readBoundedFile(path, previewMaxBytes)
		if err != nil {
			continue
		}
		out = append(out, filePreview{
			Name: entry.Name(), Path: path, Size: info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
			Preview:  string(data), Truncated: cut,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func safeListName(name string) bool {
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

func editableListName(name string) bool {
	if !safeListName(name) {
		return false
	}
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".list") ||
		strings.HasSuffix(lower, ".list-opkg") ||
		strings.HasSuffix(lower, ".list-old")
}

func protectedListName(name string) bool {
	switch strings.ToLower(name) {
	case "user.list", "exclude.list", "auto.list", "ipset.list", "ipset_exclude.list":
		return true
	default:
		return false
	}
}

func validateListContent(content string) error {
	if len(content) > listFileMaxBytes {
		return fmt.Errorf("list content exceeds %d bytes", listFileMaxBytes)
	}
	if strings.IndexByte(content, 0) >= 0 {
		return errors.New("list content contains NUL byte")
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) > 8192 {
			return fmt.Errorf("list line %d exceeds 8192 bytes", line)
		}
	}
	return scanner.Err()
}

var checkHostPattern = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func normalizeCheckURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("URL is required")
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value + "/"
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", errors.New("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("only http and https are allowed")
	}
	if parsed.User != nil {
		return "", errors.New("userinfo is not allowed")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if !checkHostPattern.MatchString(host) || net.ParseIP(host) != nil {
		return "", errors.New("a public DNS hostname is required")
	}
	if parsed.Port() != "" {
		return "", errors.New("custom ports are not allowed")
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateConfig(content string) error {
	if content == "" {
		return errors.New("nfqws2 config must not be empty")
	}
	if len(content) > configMaxBytes {
		return fmt.Errorf("nfqws2 config exceeds %d bytes", configMaxBytes)
	}
	if strings.IndexByte(content, 0) >= 0 {
		return errors.New("nfqws2 config contains NUL byte")
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) > 8192 {
			return fmt.Errorf("nfqws2 config line %d exceeds 8192 bytes", line)
		}
	}
	return scanner.Err()
}

func handleAction(w http.ResponseWriter, r *http.Request) {
	var request actionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid nfqws action request"})
		return
	}
	if request.Confirm != "NFQWS" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS"})
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "reload" && action != "restart" && action != "start" && action != "stop" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "allowed actions: start, stop, reload, restart"})
		return
	}
	if !readStatus().MutationReady {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 runtime is not ready for mutation"})
		return
	}
	output, err := runInit(action)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "output": output})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": action, "output": output, "status": readStatus()})
}

func handleListRead(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if !editableListName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list filename"})
		return
	}
	path := filepath.Join(listsRoot, name)
	data, cut, err := readBoundedFile(path, listFileMaxBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "list file not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read list: " + err.Error()})
		return
	}
	if cut {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "list file exceeds editor limit"})
		return
	}
	info, _ := os.Stat(path)
	writeJSON(w, http.StatusOK, map[string]any{
		"name": name, "path": path, "content": string(data),
		"size": len(data), "protected": protectedListName(name),
		"modified_at": func() string {
			if info == nil {
				return ""
			}
			return info.ModTime().UTC().Format(time.RFC3339)
		}(),
	})
}

func handleListCreate(w http.ResponseWriter, r *http.Request) {
	var request listRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list create request"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Confirm != "NFQWS_LIST_CREATE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_LIST_CREATE"})
		return
	}
	if !editableListName(request.Name) || !strings.HasSuffix(strings.ToLower(request.Name), ".list") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "new list filename must end with .list and contain only safe characters"})
		return
	}
	path := filepath.Join(listsRoot, request.Name)
	if _, err := os.Stat(path); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "list file already exists"})
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat list: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(path, []byte{}, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create list: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.Name, "status": readStatus()})
}

func handleListSave(w http.ResponseWriter, r *http.Request) {
	var request listRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list save request"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Confirm != "NFQWS_LIST_SAVE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_LIST_SAVE"})
		return
	}
	if !editableListName(request.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list filename"})
		return
	}
	if err := validateListContent(request.Content); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	path := filepath.Join(listsRoot, request.Name)
	before, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "read current list: " + err.Error()})
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat current list: " + err.Error()})
		return
	}
	backup, err := createNamedBackup("list-"+request.Name, before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create list backup: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(path, []byte(request.Content), info.Mode().Perm()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "save list: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.Name, "backup": backup, "status": readStatus()})
}

func handleListDelete(w http.ResponseWriter, r *http.Request) {
	var request listRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list delete request"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Confirm != "NFQWS_LIST_DELETE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_LIST_DELETE"})
		return
	}
	if !editableListName(request.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list filename"})
		return
	}
	if protectedListName(request.Name) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "core nfqws list is protected from deletion"})
		return
	}
	path := filepath.Join(listsRoot, request.Name)
	before, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "read list before delete: " + err.Error()})
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat list before delete: " + err.Error()})
		return
	}
	backup, err := createNamedBackup("deleted-"+request.Name, before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create delete backup: " + err.Error()})
		return
	}
	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete list: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.Name, "backup": backup, "status": readStatus()})
}

func handleCheck(w http.ResponseWriter, r *http.Request) {
	var request checkRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid check request"})
		return
	}
	target, err := normalizeCheckURL(request.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "build check request: " + err.Error()})
		return
	}
	req.Header.Set("User-Agent", "RouterForge-NFQWS/1")
	client := &http.Client{
		Timeout: 6 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, checkResponse{Reachable: false, URL: target, Error: err.Error()})
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
	writeJSON(w, http.StatusOK, checkResponse{Reachable: true, StatusCode: resp.StatusCode, URL: target})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	var request configRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid nfqws config request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG"})
		return
	}
	if err := validateConfig(request.Content); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	status := readStatus()
	if !status.MutationReady {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 runtime is not ready for mutation"})
		return
	}

	before, err := os.ReadFile(configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read current config: " + err.Error()})
		return
	}
	info, err := os.Stat(configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat current config: " + err.Error()})
		return
	}
	backup, err := createBackup(before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create safety backup: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(configPath, []byte(request.Content), info.Mode().Perm()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "publish config: " + err.Error(), "backup": backup})
		return
	}
	output, reloadErr := runInit("reload")
	if reloadErr != nil {
		rollbackErr := safety.WriteFileAtomic(configPath, before, info.Mode().Perm())
		rollbackOutput := ""
		if rollbackErr == nil {
			rollbackOutput, _ = runInit("reload")
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": reloadErr.Error(), "output": output, "backup": backup,
			"rolled_back": rollbackErr == nil, "rollback_error": errorString(rollbackErr),
			"rollback_output": rollbackOutput,
		})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "backup": backup, "reload_output": output, "status": readStatus(),
	})
}

func runInit(action string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
	defer cancel()
	output, err := safety.RunCommand(ctx, logTailMaxBytes, initPath, action)
	text := strings.TrimSpace(string(output))
	if ctx.Err() != nil {
		return text, fmt.Errorf("nfqws2 %s timed out", action)
	}
	if err != nil {
		return text, fmt.Errorf("nfqws2 %s failed: %w", action, err)
	}
	return text, nil
}

func createBackup(data []byte, mode os.FileMode) (string, error) {
	return createNamedBackup("config", data, mode)
}

func createNamedBackup(label string, data []byte, mode os.FileMode) (string, error) {
	if err := os.MkdirAll(backupRoot, 0700); err != nil {
		return "", err
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, label)
	path := filepath.Join(backupRoot, fmt.Sprintf("nfqws-%d-%s.bak", time.Now().UTC().UnixNano(), safe))
	if err := safety.WriteFileAtomic(path, data, mode); err != nil {
		return "", err
	}
	return path, nil
}

func countBackups() int {
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "nfqws-") {
			count++
		}
	}
	return count
}

func pruneBackups() error {
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	type item struct {
		path string
		mod  time.Time
	}
	var items []item
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "nfqws-") {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			items = append(items, item{filepath.Join(backupRoot, entry.Name()), info.ModTime()})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	for i := backupMaxFiles; i < len(items); i++ {
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
