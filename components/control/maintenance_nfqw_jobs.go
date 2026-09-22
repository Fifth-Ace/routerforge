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
	adminNFQWSJobsRetryMax          = 1
	adminNFQWSJobsSocket            = "/opt/var/run/routerforge-nfqws-manager.sock"
	adminNFQWSJobRecheckResponseMax = 128 << 10
)

const (
	adminNFQWSJobOutcomeOK           = "OK"
	adminNFQWSJobOutcomeInconclusive = "INCONCLUSIVE"
	adminNFQWSJobOutcomeFailed       = "FAILED"
	adminNFQWSJobOutcomeUnhealthy    = "UNHEALTHY"
)

var (
	adminNFQWSJobTargetPattern   = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	adminNFQWSTCP16TargetPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,31}$`)
)

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
	Running        bool      `json:"running"`
	LastRunAt      time.Time `json:"last_run_at,omitempty"`
	LastOK         bool      `json:"last_ok,omitempty"`
	LastStatus     int       `json:"last_status,omitempty"`
	LastOutput     string    `json:"last_output,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	Outcome        string    `json:"outcome,omitempty"`
	RetryCount     int       `json:"retry_count,omitempty"`
	RunCount       int       `json:"run_count,omitempty"`
	LastCadence    string    `json:"last_cadence,omitempty"`
	LastGoodAt     time.Time `json:"last_good_at,omitempty"`
	LastGoodStatus int       `json:"last_good_status,omitempty"`
	LastGoodOutput string    `json:"last_good_output,omitempty"`
	NextRunAt      time.Time `json:"next_run_at,omitempty"`
}

type adminNFQWSJobRuntime struct {
	mu          sync.Mutex
	config      adminNFQWSJobsConfig
	state       map[string]adminNFQWSJobState
	runningKeys map[string]bool
	started     bool
	now         func() time.Time
	run         func(context.Context, adminNFQWSJob) (int, string, error)
}

var adminNFQWSJobs = adminNFQWSJobRuntime{
	config:      adminNFQWSJobsConfig{Version: adminNFQWSJobsVersion, Items: map[string]adminNFQWSJob{}},
	state:       map[string]adminNFQWSJobState{},
	runningKeys: map[string]bool{},
	now:         time.Now,
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
	job.Target = strings.TrimSpace(job.Target)
	if job.ID == "" || filepath.Base(job.ID) != job.ID || len(job.ID) > 64 {
		return job, errors.New("invalid job id")
	}
	for _, r := range job.ID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return job, errors.New("invalid job id")
	}
	switch job.Kind {
	case "detect-target", "strategy-health-recheck":
		job.Target = strings.ToLower(strings.TrimSuffix(job.Target, "."))
		if !adminNFQWSJobTargetPattern.MatchString(job.Target) || net.ParseIP(job.Target) != nil {
			return job, fmt.Errorf("%s requires a DNS hostname", job.Kind)
		}
	case "tcp16-revalidate":
		job.Target = strings.ToUpper(job.Target)
		if job.Target != "" && !adminNFQWSTCP16TargetPattern.MatchString(job.Target) {
			return job, errors.New("tcp16-revalidate target must be an optional TCP16 target id")
		}
	case "nfqueue-health", "backup-cleanup":
		job.Target = ""
	case "list-refresh":
		return job, errors.New("list-refresh is not available: no refresh-capable list source provider is installed")
	default:
		return job, errors.New("unsupported NFQWS job kind")
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
	if runtime.runningKeys == nil {
		runtime.runningKeys = map[string]bool{}
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

func adminNFQWSJobKey(job adminNFQWSJob) string {
	return job.Kind + "|" + strings.ToLower(strings.TrimSpace(job.Target))
}

func (runtime *adminNFQWSJobRuntime) tick(now time.Time) {
	runtime.mu.Lock()
	due := []adminNFQWSJob{}
	if runtime.runningKeys == nil {
		runtime.runningKeys = map[string]bool{}
	}
	for id, job := range runtime.config.Items {
		state := runtime.state[id]
		key := adminNFQWSJobKey(job)
		if !job.Enabled || state.Running || runtime.runningKeys[key] {
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
		runtime.runningKeys[key] = true
		due = append(due, job)
	}
	runtime.mu.Unlock()

	for _, job := range due {
		go runtime.execute(job)
	}
}

func adminNFQWSJobTimeout(job adminNFQWSJob) time.Duration {
	switch job.Kind {
	case "strategy-health-recheck":
		return 120 * time.Second
	case "tcp16-revalidate":
		return 90 * time.Second
	default:
		return 25 * time.Second
	}
}

func shouldRetryAdminNFQWSJob(status int, err error) bool {
	return err != nil || status >= http.StatusInternalServerError
}

func classifyAdminNFQWSJobOutcome(job adminNFQWSJob, status int, output string, err error) string {
	if err != nil || status < 200 || status >= 300 {
		return adminNFQWSJobOutcomeFailed
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(output), &body)
	switch job.Kind {
	case "detect-target":
		if strings.EqualFold(fmt.Sprint(body["classification"]), "inconclusive") {
			return adminNFQWSJobOutcomeInconclusive
		}
	case "strategy-health-recheck":
		ok, _ := body["ok"].(bool)
		cleanup, _ := body["cleanup_baseline_after"].(bool)
		strategyNeeded, _ := body["strategy_needed"].(bool)
		recommendationAvailable, _ := body["recommendation_available"].(bool)
		if !ok || !cleanup {
			return adminNFQWSJobOutcomeUnhealthy
		}
		if strategyNeeded && !recommendationAvailable {
			return adminNFQWSJobOutcomeInconclusive
		}
	case "tcp16-revalidate":
		ok, _ := body["ok"].(bool)
		if !ok {
			return adminNFQWSJobOutcomeInconclusive
		}
	case "nfqueue-health":
		switch strings.ToUpper(strings.TrimSpace(fmt.Sprint(body["state"]))) {
		case "HEALTHY":
			return adminNFQWSJobOutcomeOK
		case "UNHEALTHY":
			return adminNFQWSJobOutcomeUnhealthy
		default:
			return adminNFQWSJobOutcomeInconclusive
		}
	}
	return adminNFQWSJobOutcomeOK
}

func (runtime *adminNFQWSJobRuntime) execute(job adminNFQWSJob) {
	status, output, runErr := 0, "", error(nil)
	retries := 0
	for attempt := 0; attempt <= adminNFQWSJobsRetryMax; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), adminNFQWSJobTimeout(job))
		status, output, runErr = runtime.executeRun(ctx, job)
		cancel()
		if attempt >= adminNFQWSJobsRetryMax || !shouldRetryAdminNFQWSJob(status, runErr) {
			break
		}
		retries++
	}
	now := runtime.now()
	outcome := classifyAdminNFQWSJobOutcome(job, status, output, runErr)

	runtime.mu.Lock()
	state := runtime.state[job.ID]
	state.adminNFQWSJob = job
	state.Running = false
	state.LastRunAt = now
	state.LastOK = outcome == adminNFQWSJobOutcomeOK
	state.LastStatus = status
	state.LastOutput = truncateAdminNFQWSJobOutput(output)
	state.LastError = ""
	state.Outcome = outcome
	state.RetryCount = retries
	if state.RunCount == 0 {
		state.LastCadence = "first"
	} else {
		state.LastCadence = "periodic"
	}
	state.RunCount++
	if runErr != nil {
		state.LastError = runErr.Error()
	}
	if outcome == adminNFQWSJobOutcomeOK {
		state.LastGoodAt = now
		state.LastGoodStatus = status
		state.LastGoodOutput = state.LastOutput
	}
	state.NextRunAt = now.Add(time.Duration(job.IntervalMinutes) * time.Minute)
	runtime.state[job.ID] = state
	delete(runtime.runningKeys, adminNFQWSJobKey(job))
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

func adminNFQWSManagerRequestLimit(ctx context.Context, method, path string, body []byte, limit int64) (int, string, error) {
	if limit <= 0 {
		limit = adminNFQWSJobsOutputMax
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://routerforge"+path, reader)
	if err != nil {
		return 0, "", err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
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
	data, readErr := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if readErr != nil {
		return response.StatusCode, string(data), readErr
	}
	if int64(len(data)) > limit {
		return response.StatusCode, string(data[:limit]), errors.New("nfqws-manager response exceeds job limit")
	}
	output := string(data)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, output, fmt.Errorf("nfqws-manager returned HTTP %d", response.StatusCode)
	}
	return response.StatusCode, output, nil
}

func adminNFQWSManagerRequest(ctx context.Context, method, path string, body []byte) (int, string, error) {
	status, output, err := adminNFQWSManagerRequestLimit(ctx, method, path, body, adminNFQWSJobsOutputMax)
	return status, truncateAdminNFQWSJobOutput(output), err
}

func parseAdminNFQWSManagerConfigSHA(output string) (string, error) {
	var body struct {
		ConfigSHA256 string `json:"config_sha256"`
	}
	if json.Unmarshal([]byte(output), &body) != nil || strings.TrimSpace(body.ConfigSHA256) == "" {
		return "", errors.New("nfqws-manager health did not return config_sha256")
	}
	return strings.TrimSpace(body.ConfigSHA256), nil
}

func adminNFQWSManagerConfigSHA(ctx context.Context) (string, error) {
	status, output, err := adminNFQWSManagerRequest(ctx, http.MethodGet, "/v1/health", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("nfqws-manager health returned HTTP %d", status)
	}
	return parseAdminNFQWSManagerConfigSHA(output)
}

func runAdminNFQWSJob(ctx context.Context, job adminNFQWSJob) (int, string, error) {
	normalized, err := normalizeAdminNFQWSJob(job)
	if err != nil {
		return 0, "", err
	}
	switch normalized.Kind {
	case "detect-target":
		before, err := adminNFQWSManagerConfigSHA(ctx)
		if err != nil {
			return 0, "", err
		}
		body, _ := json.Marshal(map[string]string{"target": normalized.Target})
		status, output, runErr := adminNFQWSManagerRequest(ctx, http.MethodPost, "/v1/v2/detect", body)
		if runErr != nil {
			return status, output, runErr
		}
		after, err := adminNFQWSManagerConfigSHA(ctx)
		if err != nil {
			return status, output, err
		}
		if !strings.EqualFold(before, after) {
			return http.StatusConflict, output, errors.New("production config changed during NFQWS job; result is stale")
		}
		return status, output, nil
	case "strategy-health-recheck":
		expectedSHA, err := adminNFQWSManagerConfigSHA(ctx)
		if err != nil {
			return 0, "", err
		}
		body, _ := json.Marshal(map[string]any{
			"mode":                   "fast",
			"server_name":            normalized.Target,
			"transport":              "https",
			"expected_config_sha256": expectedSHA,
			"concurrency":            1,
			"confirm":                "ROUTERFORGE_V2_PROGRESSIVE_SELECTOR",
		})
		status, output, runErr := adminNFQWSManagerRequestLimit(
			ctx, http.MethodPost, "/v1/v2/selector-progressive", body, adminNFQWSJobRecheckResponseMax,
		)
		if runErr != nil {
			return status, truncateAdminNFQWSJobOutput(output), runErr
		}
		after, err := adminNFQWSManagerConfigSHA(ctx)
		if err != nil {
			return status, truncateAdminNFQWSJobOutput(output), err
		}
		if !strings.EqualFold(expectedSHA, after) {
			return http.StatusConflict, truncateAdminNFQWSJobOutput(output), errors.New("production config changed during strategy health recheck")
		}
		return status, output, nil
	case "tcp16-revalidate":
		expectedSHA, err := adminNFQWSManagerConfigSHA(ctx)
		if err != nil {
			return 0, "", err
		}
		targetIDs := []string{}
		if normalized.Target != "" {
			targetIDs = append(targetIDs, normalized.Target)
		}
		body, _ := json.Marshal(map[string]any{
			"target_ids":             targetIDs,
			"scan_sni":               true,
			"expected_config_sha256": expectedSHA,
			"confirm":                "ROUTERFORGE_TCP16_PROBE_V2",
		})
		return adminNFQWSManagerRequest(ctx, http.MethodPost, "/v1/v2/tcp16-probe", body)
	case "nfqueue-health":
		return adminNFQWSManagerRequest(ctx, http.MethodGet, "/v1/v2/nfqueue-health", nil)
	case "backup-cleanup":
		body, _ := json.Marshal(map[string]string{"confirm": "NFQWS_BACKUP_PRUNE"})
		return adminNFQWSManagerRequest(ctx, http.MethodPost, "/v1/backups/prune", body)
	default:
		return 0, "", errors.New("unsupported NFQWS job kind")
	}
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
		"jobs": adminNFQWSJobs.statuses(),
		"kinds": []string{
			"detect-target",
			"tcp16-revalidate",
			"strategy-health-recheck",
			"nfqueue-health",
			"backup-cleanup",
		},
		"kind_capabilities": []map[string]any{
			{"kind": "detect-target", "available": true, "production_mutation": false},
			{"kind": "tcp16-revalidate", "available": true, "production_mutation": false},
			{"kind": "strategy-health-recheck", "available": true, "production_mutation": false},
			{"kind": "nfqueue-health", "available": true, "production_mutation": false},
			{"kind": "backup-cleanup", "available": true, "production_mutation": false},
			{"kind": "list-refresh", "available": false, "reason": "no refresh-capable list source provider is installed", "production_mutation": false},
		},
		"min_interval_minutes": adminNFQWSJobsMinMinutes,
		"max_interval_minutes": adminNFQWSJobsMaxMinutes,
		"default_enabled":      false,
		"retry_max":            adminNFQWSJobsRetryMax,
		"single_flight":        "kind+target",
		"restart_storm_guard":  true,
		"last_known_good":      true,
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
	if adminNFQWSJobs.runningKeys == nil {
		adminNFQWSJobs.runningKeys = map[string]bool{}
	}
	key := adminNFQWSJobKey(job)
	if state.Running || adminNFQWSJobs.runningKeys[key] {
		adminNFQWSJobs.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "NFQWS job kind/target is already running"})
		return
	}
	state.adminNFQWSJob = job
	state.Running = true
	adminNFQWSJobs.state[req.ID] = state
	adminNFQWSJobs.runningKeys[key] = true
	adminNFQWSJobs.mu.Unlock()

	go adminNFQWSJobs.execute(job)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": job.ID, "state": "running", "production_mutation": false})
}

func resetAdminNFQWSJobsForTest(now time.Time) {
	adminNFQWSJobs.mu.Lock()
	defer adminNFQWSJobs.mu.Unlock()
	adminNFQWSJobs.config = adminNFQWSJobsConfig{Version: adminNFQWSJobsVersion, Items: map[string]adminNFQWSJob{}}
	adminNFQWSJobs.state = map[string]adminNFQWSJobState{}
	adminNFQWSJobs.runningKeys = map[string]bool{}
	adminNFQWSJobs.started = false
	adminNFQWSJobs.now = func() time.Time { return now }
	adminNFQWSJobs.run = nil
}
