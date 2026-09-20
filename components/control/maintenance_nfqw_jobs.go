package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	adminNFQWSJobsConfigPath = "/opt/etc/routerforge/nfqws-jobs.json"
	adminNFQWSJobsVersion    = 1
	adminNFQWSJobsMax        = 32
	adminNFQWSJobsOutputMax  = 4096
	adminNFQWSJobsMinMinutes = 15
	adminNFQWSJobsMaxMinutes = 1440
	adminNFQWSJobsSocket     = "/opt/var/run/routerforge-nfqws-manager.sock"
)

var adminNFQWSJobTargetPattern = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

type adminNFQWSJob struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Target          string `json:"target,omitempty"`
	IntervalMinutes int    `json:"interval_minutes"`
	Enabled         bool   `json:"enabled"`
}

type adminNFQWSJobsConfig struct {
	Version int                      `json:"version"`
	Items   map[string]adminNFQWSJob `json:"items"`
}

type adminNFQWSJobConfigureRequest struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Target          string `json:"target,omitempty"`
	IntervalMinutes int    `json:"interval_minutes"`
	Enabled         bool   `json:"enabled"`
	ConfirmID       string `json:"confirm_id"`
}

type adminNFQWSJobRunRequest struct {
	ID        string `json:"id"`
	ConfirmID string `json:"confirm_id"`
}

type adminNFQWSJobState struct {
	adminNFQWSJob
	Running    bool      `json:"running"`
	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	LastOK     bool      `json:"last_ok,omitempty"`
	LastStatus int       `json:"last_status,omitempty"`
	LastOutput string    `json:"last_output,omitempty"`
	NextRunAt  time.Time `json:"next_run_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

type adminNFQWSJobRuntime struct {
	mu      sync.Mutex
	config  adminNFQWSJobsConfig
	state   map[string]adminNFQWSJobState
	started bool
	now     func() time.Time
	run     func(context.Context, adminNFQWSJob) (int, string, error)
}

var adminNFQWSJobs = adminNFQWSJobRuntime{
	config: adminNFQWSJobsConfig{Version: adminNFQWSJobsVersion, Items: map[string]adminNFQWSJob{}},
	state:  map[string]adminNFQWSJobState{},
	now:    time.Now,
}

func registerAdminNFQWSJobRoutes(mux *http.ServeMux) {
	adminNFQWSJobs.start()
	mux.HandleFunc("/v1/maintenance/nfqws-jobs", getOnly(handleAdminNFQWSJobs))
	mux.HandleFunc("/v1/maintenance/nfqws-jobs/configure", mutationOnly(handleAdminNFQWSJobConfigure))
	mux.HandleFunc("/v1/maintenance/nfqws-jobs/run", mutationOnly(handleAdminNFQWSJobRun))
}

func normalizeAdminNFQWSJob(job adminNFQWSJob) (adminNFQWSJob, error) {
	job.ID = strings.TrimSpace(job.ID)
	job.Kind = strings.ToLower(strings.TrimSpace(job.Kind))
	job.Target = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(job.Target), "."))
	if job.ID == "" || filepath.Base(job.ID) != job.ID || len(job.ID) > 64 {
		return job, errors.New("invalid job id")
	}
	for _, r := range job.ID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return job, errors.New("invalid job id")
	}
	if job.Kind != "detect-target" {
		return job, errors.New("unsupported NFQWS job kind")
	}
	if !adminNFQWSJobTargetPattern.MatchString(job.Target) || net.ParseIP(job.Target) != nil {
		return job, errors.New("detect-target requires a DNS hostname")
	}
	if job.IntervalMinutes < adminNFQWSJobsMinMinutes || job.IntervalMinutes > adminNFQWSJobsMaxMinutes {
		return job, fmt.Errorf("interval_minutes must be between %d and %d", adminNFQWSJobsMinMinutes, adminNFQWSJobsMaxMinutes)
	}
	return job, nil
}

func loadAdminNFQWSJobsConfig() adminNFQWSJobsConfig {
	out := adminNFQWSJobsConfig{Version: adminNFQWSJobsVersion, Items: map[string]adminNFQWSJob{}}
	data, err := os.ReadFile(adminNFQWSJobsConfigPath)
	if err != nil {
		return out
	}
	var raw adminNFQWSJobsConfig
	if json.Unmarshal(data, &raw) != nil || raw.Version != adminNFQWSJobsVersion {
		return out
	}
	for id, item := range raw.Items {
		item.ID = id
		normalized, err := normalizeAdminNFQWSJob(item)
		if err == nil && len(out.Items) < adminNFQWSJobsMax {
			out.Items[normalized.ID] = normalized
		}
	}
	return out
}

func saveAdminNFQWSJobsConfig(config adminNFQWSJobsConfig) error {
	if len(config.Items) > adminNFQWSJobsMax {
		return errors.New("NFQWS job limit exceeded")
	}
	if err := os.MkdirAll(filepath.Dir(adminNFQWSJobsConfigPath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return safety.WriteFileAtomic(adminNFQWSJobsConfigPath, data, 0600)
}

func (runtime *adminNFQWSJobRuntime) start() {
	runtime.mu.Lock()
	if runtime.started {
		runtime.mu.Unlock()
		return
	}
	runtime.started = true
	runtime.config = loadAdminNFQWSJobsConfig()
	if runtime.state == nil {
		runtime.state = map[string]adminNFQWSJobState{}
	}
	now := runtime.now()
	for id, job := range runtime.config.Items {
		runtime.state[id] = adminNFQWSJobState{
			adminNFQWSJob: job,
			LastRunAt:     now,
			NextRunAt:     now.Add(time.Duration(job.IntervalMinutes) * time.Minute),
		}
	}
	runtime.mu.Unlock()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			runtime.tick(now)
		}
	}()
}

func (runtime *adminNFQWSJobRuntime) tick(now time.Time) {
	runtime.mu.Lock()
	due := []adminNFQWSJob{}
	for id, job := range runtime.config.Items {
		state := runtime.state[id]
		if !job.Enabled || state.Running {
			continue
		}
		next := state.NextRunAt
		if next.IsZero() {
			next = state.LastRunAt.Add(time.Duration(job.IntervalMinutes) * time.Minute)
		}
		if next.IsZero() || now.Before(next) {
			continue
		}
		state.adminNFQWSJob = job
		state.Running = true
		runtime.state[id] = state
		due = append(due, job)
	}
	runtime.mu.Unlock()

	for _, job := range due {
		go runtime.execute(job)
	}
}

func (runtime *adminNFQWSJobRuntime) execute(job adminNFQWSJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, output, err := runtime.executeRun(ctx, job)
	now := runtime.now()

	runtime.mu.Lock()
	state := runtime.state[job.ID]
	state.adminNFQWSJob = job
	state.Running = false
	state.LastRunAt = now
	state.LastOK = err == nil && status >= 200 && status < 300
	state.LastStatus = status
	state.LastOutput = truncateAdminNFQWSJobOutput(output)
	state.LastError = ""
	if err != nil {
		state.LastError = err.Error()
	}
	state.NextRunAt = now.Add(time.Duration(job.IntervalMinutes) * time.Minute)
	runtime.state[job.ID] = state
	runtime.mu.Unlock()
}

func (runtime *adminNFQWSJobRuntime) executeRun(ctx context.Context, job adminNFQWSJob) (int, string, error) {
	if runtime.run != nil {
		return runtime.run(ctx, job)
	}
	return runAdminNFQWSJob(ctx, job)
}

func truncateAdminNFQWSJobOutput(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > adminNFQWSJobsOutputMax {
		return value[:adminNFQWSJobsOutputMax] + "..."
	}
	return value
}

func runAdminNFQWSJob(ctx context.Context, job adminNFQWSJob) (int, string, error) {
	normalized, err := normalizeAdminNFQWSJob(job)
	if err != nil {
		return 0, "", err
	}
	body, _ := json.Marshal(map[string]string{"target": normalized.Target})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://routerforge/v1/v2/detect", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", adminNFQWSJobsSocket)
		},
	}
	client := &http.Client{Transport: transport}
	response, err := client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, adminNFQWSJobsOutputMax+1))
	output := truncateAdminNFQWSJobOutput(string(data))
	if readErr != nil {
		return response.StatusCode, output, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, output, fmt.Errorf("nfqws-manager returned HTTP %d", response.StatusCode)
	}
	return response.StatusCode, output, nil
}

func (runtime *adminNFQWSJobRuntime) statuses() []adminNFQWSJobState {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	out := make([]adminNFQWSJobState, 0, len(runtime.config.Items))
	now := runtime.now()
	for id, job := range runtime.config.Items {
		state := runtime.state[id]
		state.adminNFQWSJob = job
		if state.NextRunAt.IsZero() {
			state.NextRunAt = now.Add(time.Duration(job.IntervalMinutes) * time.Minute)
		}
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func handleAdminNFQWSJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":                 adminNFQWSJobs.statuses(),
		"kinds":                []string{"detect-target"},
		"min_interval_minutes": adminNFQWSJobsMinMinutes,
		"max_interval_minutes": adminNFQWSJobsMaxMinutes,
		"default_enabled":      false,
		"production_mutation":  false,
	})
}

func handleAdminNFQWSJobConfigure(w http.ResponseWriter, r *http.Request) {
	var req adminNFQWSJobConfigureRequest
	if err := decodeMutationJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid NFQWS job request"})
		return
	}
	if strings.TrimSpace(req.ConfirmID) != strings.TrimSpace(req.ID) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_id does not match job id"})
		return
	}
	job, err := normalizeAdminNFQWSJob(adminNFQWSJob{
		ID: req.ID, Kind: req.Kind, Target: req.Target,
		IntervalMinutes: req.IntervalMinutes, Enabled: req.Enabled,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	adminNFQWSJobs.mu.Lock()
	next := adminNFQWSJobsConfig{Version: adminNFQWSJobsVersion, Items: map[string]adminNFQWSJob{}}
	for id, item := range adminNFQWSJobs.config.Items {
		next.Items[id] = item
	}
	if _, exists := next.Items[job.ID]; !exists && len(next.Items) >= adminNFQWSJobsMax {
		adminNFQWSJobs.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "NFQWS job limit exceeded"})
		return
	}
	next.Items[job.ID] = job
	if err := saveAdminNFQWSJobsConfig(next); err != nil {
		adminNFQWSJobs.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	adminNFQWSJobs.config = next
	state := adminNFQWSJobs.state[job.ID]
	state.adminNFQWSJob = job
	state.NextRunAt = adminNFQWSJobs.now().Add(time.Duration(job.IntervalMinutes) * time.Minute)
	adminNFQWSJobs.state[job.ID] = state
	adminNFQWSJobs.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": job, "production_mutation": false})
}

func handleAdminNFQWSJobRun(w http.ResponseWriter, r *http.Request) {
	var req adminNFQWSJobRunRequest
	if err := decodeMutationJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid NFQWS job run request"})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ConfirmID != req.ID {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_id does not match job id"})
		return
	}

	adminNFQWSJobs.mu.Lock()
	job, ok := adminNFQWSJobs.config.Items[req.ID]
	state := adminNFQWSJobs.state[req.ID]
	if !ok {
		adminNFQWSJobs.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "NFQWS job not found"})
		return
	}
	if state.Running {
		adminNFQWSJobs.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "NFQWS job is already running"})
		return
	}
	state.adminNFQWSJob = job
	state.Running = true
	adminNFQWSJobs.state[req.ID] = state
	adminNFQWSJobs.mu.Unlock()

	go adminNFQWSJobs.execute(job)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": job.ID, "state": "running", "production_mutation": false})
}

func resetAdminNFQWSJobsForTest(now time.Time) {
	adminNFQWSJobs.mu.Lock()
	defer adminNFQWSJobs.mu.Unlock()
	adminNFQWSJobs.config = adminNFQWSJobsConfig{Version: adminNFQWSJobsVersion, Items: map[string]adminNFQWSJob{}}
	adminNFQWSJobs.state = map[string]adminNFQWSJobState{}
	adminNFQWSJobs.started = false
	adminNFQWSJobs.now = func() time.Time { return now }
	adminNFQWSJobs.run = nil
}
