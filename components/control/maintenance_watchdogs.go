package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	adminWatchdogConfigPath     = "/opt/etc/routerforge/watchdogs.json"
	adminWatchdogInterval       = 30 * time.Second
	adminWatchdogCooldown       = 5 * time.Minute
	adminWatchdogAttemptWindow  = time.Hour
	adminWatchdogMaxAttempts    = 3
	adminWatchdogOutputMaxBytes = 4096
	adminWatchdogStartTimeout   = 15 * time.Second
)

type adminWatchdogConfig struct {
	Version int                          `json:"version"`
	Items   map[string]adminWatchdogItem `json:"items"`
}

type adminWatchdogItem struct {
	Enabled bool `json:"enabled"`
}

type adminWatchdogConfigureRequest struct {
	ID        string `json:"id"`
	Enabled   bool   `json:"enabled"`
	ConfirmID string `json:"confirm_id"`
}

type adminWatchdogStatus struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Enabled            bool      `json:"enabled"`
	Detected           bool      `json:"detected"`
	Running            bool      `json:"running"`
	ServiceID          string    `json:"service_id,omitempty"`
	ServicePath        string    `json:"service_path,omitempty"`
	AttemptsLastHour   int       `json:"attempts_last_hour"`
	LastAttemptAt      time.Time `json:"last_attempt_at,omitempty"`
	LastAttemptOK      bool      `json:"last_attempt_ok,omitempty"`
	LastAttemptOutput  string    `json:"last_attempt_output,omitempty"`
	NextAttemptAfter   time.Time `json:"next_attempt_after,omitempty"`
	AutoStartAvailable bool      `json:"auto_start_available"`
}

type adminWatchdogRuntime struct {
	mu          sync.Mutex
	config      adminWatchdogConfig
	attempts    map[string][]time.Time
	lastAttempt map[string]adminWatchdogAttempt
	started     bool
}

type adminWatchdogAttempt struct {
	At     time.Time
	OK     bool
	Output string
}

var adminWatchdogs = adminWatchdogRuntime{
	config: adminWatchdogConfig{
		Version: 1,
		Items:   map[string]adminWatchdogItem{},
	},
	attempts:    map[string][]time.Time{},
	lastAttempt: map[string]adminWatchdogAttempt{},
}

func registerAdminWatchdogRoutes(mux *http.ServeMux) {
	adminWatchdogs.start()
	mux.HandleFunc("/v1/maintenance/watchdogs", getOnly(handleAdminWatchdogs))
	mux.HandleFunc("/v1/maintenance/watchdogs/configure", mutationOnly(handleAdminWatchdogConfigure))
}

func (runtime *adminWatchdogRuntime) start() {
	runtime.mu.Lock()
	if runtime.started {
		runtime.mu.Unlock()
		return
	}
	runtime.started = true
	runtime.config = loadAdminWatchdogConfig()
	runtime.mu.Unlock()

	go func() {
		ticker := time.NewTicker(adminWatchdogInterval)
		defer ticker.Stop()
		for range ticker.C {
			runtime.tick(time.Now())
		}
	}()
}

func loadAdminWatchdogConfig() adminWatchdogConfig {
	config := adminWatchdogConfig{Version: 1, Items: map[string]adminWatchdogItem{}}
	content, err := os.ReadFile(adminWatchdogConfigPath)
	if err != nil {
		return config
	}
	if err := json.Unmarshal(content, &config); err != nil || config.Version != 1 {
		return adminWatchdogConfig{Version: 1, Items: map[string]adminWatchdogItem{}}
	}
	if config.Items == nil {
		config.Items = map[string]adminWatchdogItem{}
	}
	filtered := map[string]adminWatchdogItem{}
	for id, item := range config.Items {
		if _, ok := adminWatchdogDefinition(id); ok {
			filtered[id] = item
		}
	}
	config.Items = filtered
	return config
}

func saveAdminWatchdogConfig(config adminWatchdogConfig) error {
	if err := os.MkdirAll(filepath.Dir(adminWatchdogConfigPath), 0700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	temp, err := os.CreateTemp(filepath.Dir(adminWatchdogConfigPath), ".watchdogs-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	success := false
	defer func() {
		_ = temp.Close()
		if !success {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0600); err != nil {
		return err
	}
	if _, err := temp.Write(content); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, adminWatchdogConfigPath); err != nil {
		return err
	}
	success = true
	return nil
}

func adminWatchdogDefinition(id string) (adminIntegrationDefinition, bool) {
	for _, definition := range adminIntegrationDefinitions {
		if definition.ID == id {
			return definition, true
		}
	}
	return adminIntegrationDefinition{}, false
}

func handleAdminWatchdogs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"watchdogs":             adminWatchdogs.statuses(time.Now()),
		"interval_seconds":      int(adminWatchdogInterval / time.Second),
		"cooldown_seconds":      int(adminWatchdogCooldown / time.Second),
		"max_attempts_per_hour": adminWatchdogMaxAttempts,
		"default_enabled":       false,
	})
}

func handleAdminWatchdogConfigure(w http.ResponseWriter, r *http.Request) {
	var request adminWatchdogConfigureRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid watchdog configuration request"})
		return
	}
	request.ID = strings.TrimSpace(request.ID)
	if request.ConfirmID != request.ID {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_id does not match watchdog id"})
		return
	}
	if _, ok := adminWatchdogDefinition(request.ID); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported watchdog id"})
		return
	}

	adminWatchdogs.mu.Lock()
	next := adminWatchdogConfig{Version: 1, Items: map[string]adminWatchdogItem{}}
	for id, item := range adminWatchdogs.config.Items {
		next.Items[id] = item
	}
	next.Items[request.ID] = adminWatchdogItem{Enabled: request.Enabled}
	if err := saveAdminWatchdogConfig(next); err != nil {
		adminWatchdogs.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	adminWatchdogs.config = next
	adminWatchdogs.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"id":      request.ID,
		"enabled": request.Enabled,
	})
}

func (runtime *adminWatchdogRuntime) statuses(now time.Time) []adminWatchdogStatus {
	services := readServices()

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.pruneAttemptsLocked(now)

	statuses := make([]adminWatchdogStatus, 0, len(adminIntegrationDefinitions))
	for _, definition := range adminIntegrationDefinitions {
		service, detected := findAdminWatchdogService(definition, services)
		item := runtime.config.Items[definition.ID]
		attempts := runtime.attempts[definition.ID]
		last := runtime.lastAttempt[definition.ID]
		status := adminWatchdogStatus{
			ID:               definition.ID,
			Name:             definition.Name,
			Enabled:          item.Enabled,
			Detected:         detected,
			AttemptsLastHour: len(attempts),
		}
		if detected {
			status.Running = service.Running
			status.ServiceID = service.ID
			status.ServicePath = service.Path
			status.AutoStartAvailable = service.Executable && safeAdminWatchdogServicePath(service.Path)
		}
		if !last.At.IsZero() {
			status.LastAttemptAt = last.At
			status.LastAttemptOK = last.OK
			status.LastAttemptOutput = last.Output
			status.NextAttemptAfter = last.At.Add(adminWatchdogCooldown)
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (runtime *adminWatchdogRuntime) tick(now time.Time) {
	services := readServices()

	for _, definition := range adminIntegrationDefinitions {
		service, detected := findAdminWatchdogService(definition, services)
		if !detected || service.Running || !service.Executable || !safeAdminWatchdogServicePath(service.Path) {
			continue
		}
		if !runtime.reserveAttempt(definition.ID, now) {
			continue
		}

		attempt := runAdminWatchdogServiceStart(service.Path, now, adminWatchdogStartTimeout)
		runtime.mu.Lock()
		runtime.lastAttempt[definition.ID] = attempt
		runtime.mu.Unlock()
	}
}

func (runtime *adminWatchdogRuntime) reserveAttempt(id string, now time.Time) bool {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	runtime.pruneAttemptsLocked(now)
	if !runtime.config.Items[id].Enabled {
		return false
	}
	attempts := runtime.attempts[id]
	if len(attempts) >= adminWatchdogMaxAttempts {
		return false
	}
	last := runtime.lastAttempt[id]
	if !last.At.IsZero() && now.Sub(last.At) < adminWatchdogCooldown {
		return false
	}

	runtime.attempts[id] = append(attempts, now)
	runtime.lastAttempt[id] = adminWatchdogAttempt{
		At:     now,
		Output: "start in progress",
	}
	return true
}

func runAdminWatchdogServiceStart(path string, now time.Time, timeout time.Duration) adminWatchdogAttempt {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, path, "start").CombinedOutput()
	if len(output) > adminWatchdogOutputMaxBytes {
		output = output[:adminWatchdogOutputMaxBytes]
	}
	text := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		if text != "" {
			text += "\n"
		}
		text += "start timed out"
	}

	return adminWatchdogAttempt{
		At:     now,
		OK:     err == nil && ctx.Err() == nil,
		Output: text,
	}
}

func (runtime *adminWatchdogRuntime) pruneAttemptsLocked(now time.Time) {
	cutoff := now.Add(-adminWatchdogAttemptWindow)
	for id, attempts := range runtime.attempts {
		keep := attempts[:0]
		for _, attempt := range attempts {
			if !attempt.Before(cutoff) {
				keep = append(keep, attempt)
			}
		}
		runtime.attempts[id] = keep
	}
}

func findAdminWatchdogService(definition adminIntegrationDefinition, services []serviceInfo) (serviceInfo, bool) {
	candidates := make([]serviceInfo, 0)
	for _, service := range services {
		corpus := strings.ToLower(service.ID + " " + service.Name + " " + service.Path)
		if containsAnyToken(corpus, definition.ServiceTokens) {
			candidates = append(candidates, service)
		}
	}
	if len(candidates) == 0 {
		return serviceInfo{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], true
}

func safeAdminWatchdogServicePath(path string) bool {
	clean := filepath.Clean(path)
	root := filepath.Clean(serviceInitDir)
	if filepath.Dir(clean) != root {
		return false
	}
	info, err := os.Lstat(clean)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return info.Mode()&0111 != 0
}

func resetAdminWatchdogsForTest() {
	adminWatchdogs.mu.Lock()
	defer adminWatchdogs.mu.Unlock()
	adminWatchdogs.config = adminWatchdogConfig{Version: 1, Items: map[string]adminWatchdogItem{}}
	adminWatchdogs.attempts = map[string][]time.Time{}
	adminWatchdogs.lastAttempt = map[string]adminWatchdogAttempt{}
}
