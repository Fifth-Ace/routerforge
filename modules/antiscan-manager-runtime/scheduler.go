package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanSchedulerFileMax = 64 << 10
	antiscanSchedulerTimeout = 10 * time.Second
)

var (
	antiscanSchedulerFieldPattern = regexp.MustCompile(`^[0-9*/]+$`)
	antiscanSchedulerManagedTasks = []string{
		"read_candidates",
		"read_ndm_ipsets",
		"save_ipsets",
		"update_ipsets geo",
	}
	antiscanSchedulerAllTasks = []string{
		"read_candidates",
		"read_ndm_ipsets",
		"save_ipsets",
		"update_ipsets geo",
		"retry_load_geo",
	}
	antiscanSchedulerDefaults = map[string]string{
		"read_candidates":   "*/1 * * * *",
		"read_ndm_ipsets":   "*/2 * * * *",
		"save_ipsets":       "0 0 */5 * *",
		"update_ipsets geo": "0 5 */15 * *",
		"retry_load_geo":    "0 */1 * * *",
	}
	antiscanSchedulerFindCrontab = func() string {
		return findAntiscanExecutable("/opt/bin/crontab", "/usr/bin/crontab", "crontab")
	}
)

type antiscanSchedulerTask struct {
	Task      string `json:"task"`
	Enabled   bool   `json:"enabled"`
	Schedule  string `json:"schedule"`
	Automatic bool   `json:"automatic"`
}

type antiscanSchedulerStatus struct {
	SourceSHA256 string                  `json:"source_sha256"`
	Valid        bool                    `json:"valid"`
	Synced       bool                    `json:"synced"`
	TaskCount    int                     `json:"task_count"`
	ActiveCount  int                     `json:"active_count"`
	Tasks        []antiscanSchedulerTask `json:"tasks"`
	Errors       []string                `json:"errors"`
	MutationAPI  bool                    `json:"mutation_api"`
}

type antiscanSchedulerTaskRequest struct {
	Task     string `json:"task"`
	Enabled  bool   `json:"enabled"`
	Schedule string `json:"schedule"`
}

type antiscanSchedulerRequest struct {
	Tasks      []antiscanSchedulerTaskRequest `json:"tasks"`
	BaseSHA256 string                         `json:"base_sha256"`
	Confirm    string                         `json:"confirm"`
}

type antiscanSchedulerResult struct {
	Action            string   `json:"action"`
	Changed           bool     `json:"changed"`
	Verified          bool     `json:"verified"`
	Synced            bool     `json:"synced"`
	RollbackPerformed bool     `json:"rollback_performed"`
	SourceSHA256      string   `json:"source_sha256,omitempty"`
	TaskCount         int      `json:"task_count"`
	ActiveCount       int      `json:"active_count"`
	AutoRetry         bool     `json:"auto_retry"`
	MutationAPI       bool     `json:"mutation_api"`
	Warnings          []string `json:"warnings"`
	Error             string   `json:"error,omitempty"`
}

type antiscanSchedulerSource struct {
	Path      string
	Data      []byte
	Mode      os.FileMode
	SHA256    string
	Tasks     map[string]string
	TaskLines []string
	Errors    []string
	AutoRetry bool
	Valid     bool
}

func handleAntiscanSchedulerRead(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := readAntiscanSchedulerStatus(r.Context(), cfg)
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":        err.Error(),
				"mutation_api": false,
			})
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func handleAntiscanSchedulerMutation(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanSchedulerRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, antiscanSchedulerResult{
				Action:      "apply",
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "invalid scheduler request",
			})
			return
		}
		if request.Confirm != "APPLY_SCHEDULE" {
			writeJSON(w, http.StatusConflict, antiscanSchedulerResult{
				Action:      "apply",
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "confirm must equal APPLY_SCHEDULE",
			})
			return
		}

		result, status, err := applyAntiscanScheduler(r.Context(), cfg, request)
		if err != nil {
			result.Error = err.Error()
		}
		writeJSON(w, status, result)
	}
}

func readAntiscanSchedulerStatus(parent context.Context, cfg runtimeConfig) (antiscanSchedulerStatus, error) {
	source, err := readAntiscanSchedulerSource(cfg)
	if err != nil {
		return antiscanSchedulerStatus{}, err
	}
	active, err := readAntiscanActiveCron(parent)
	if err != nil {
		return antiscanSchedulerStatus{}, err
	}

	status := antiscanSchedulerStatus{
		SourceSHA256: source.SHA256,
		Valid:        source.Valid,
		Synced:       source.Valid && sameStringSet(source.TaskLines, active),
		TaskCount:    len(source.TaskLines),
		ActiveCount:  len(active),
		Tasks:        make([]antiscanSchedulerTask, 0, len(antiscanSchedulerAllTasks)),
		Errors:       append([]string(nil), source.Errors...),
		MutationAPI:  true,
	}
	for _, task := range antiscanSchedulerAllTasks {
		schedule, enabled := source.Tasks[task]
		if schedule == "" {
			schedule = antiscanSchedulerDefaults[task]
		}
		status.Tasks = append(status.Tasks, antiscanSchedulerTask{
			Task:      task,
			Enabled:   enabled,
			Schedule:  schedule,
			Automatic: task == "retry_load_geo",
		})
	}
	return status, nil
}

func readAntiscanSchedulerSource(cfg runtimeConfig) (antiscanSchedulerSource, error) {
	path := filepath.Join(cfg.AntiscanDir, "ascn_crontab.conf")
	info, err := os.Lstat(path)
	if err != nil {
		return antiscanSchedulerSource{}, fmt.Errorf("inspect ascn_crontab.conf: %w", err)
	}
	if !info.Mode().IsRegular() {
		return antiscanSchedulerSource{}, errors.New("ascn_crontab.conf is not a regular file")
	}
	if info.Size() > antiscanSchedulerFileMax {
		return antiscanSchedulerSource{}, errors.New("ascn_crontab.conf exceeds RouterForge safety limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return antiscanSchedulerSource{}, fmt.Errorf("read ascn_crontab.conf: %w", err)
	}
	tasks, taskLines, parseErrors := parseAntiscanSchedulerSource(string(data))
	return antiscanSchedulerSource{
		Path:      path,
		Data:      data,
		Mode:      info.Mode().Perm(),
		SHA256:    antiscanSchedulerSHA(data),
		Tasks:     tasks,
		TaskLines: taskLines,
		Errors:    parseErrors,
		AutoRetry: tasks["retry_load_geo"] != "",
		Valid:     len(parseErrors) == 0,
	}, nil
}

func parseAntiscanSchedulerSource(text string) (map[string]string, []string, []string) {
	tasks := make(map[string]string, len(antiscanSchedulerAllTasks))
	lines := []string{}
	problems := []string{}
	for index, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		task, schedule, ok, err := parseAntiscanSchedulerLine(line)
		if err != nil {
			problems = append(problems, fmt.Sprintf("line %d: %v", index+1, err))
			continue
		}
		if !ok {
			problems = append(problems, fmt.Sprintf("line %d: unsupported scheduler line", index+1))
			continue
		}
		if _, exists := tasks[task]; exists {
			problems = append(problems, fmt.Sprintf("line %d: duplicate scheduler task %q", index+1, task))
			continue
		}
		tasks[task] = schedule
		lines = append(lines, canonicalAntiscanSchedulerLine(task, schedule))
	}
	return tasks, lines, problems
}

func parseAntiscanSchedulerLine(line string) (string, string, bool, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 8 {
		return "", "", false, errors.New("invalid cron syntax")
	}
	if fields[len(fields)-1] != "&" {
		return "", "", false, errors.New("scheduler command must end with &")
	}
	for _, field := range fields[:5] {
		if len(field) > 32 || !antiscanSchedulerFieldPattern.MatchString(field) {
			return "", "", false, errors.New("cron field contains characters unsupported by upstream")
		}
	}
	command := strings.Join(fields[5:len(fields)-1], " ")
	const prefix = "/opt/etc/init.d/S99ascn "
	if !strings.HasPrefix(command, prefix) {
		return "", "", false, nil
	}
	task := strings.TrimSpace(strings.TrimPrefix(command, prefix))
	if !antiscanSchedulerTaskAllowed(task) {
		return "", "", false, fmt.Errorf("unsupported scheduler task %q", task)
	}
	return task, strings.Join(fields[:5], " "), true, nil
}

func antiscanSchedulerTaskAllowed(task string) bool {
	for _, allowed := range antiscanSchedulerAllTasks {
		if task == allowed {
			return true
		}
	}
	return false
}

func normalizeAntiscanSchedulerRequest(tasks []antiscanSchedulerTaskRequest) (map[string]antiscanSchedulerTaskRequest, error) {
	if len(tasks) != len(antiscanSchedulerManagedTasks) {
		return nil, fmt.Errorf("scheduler request must contain exactly %d managed tasks", len(antiscanSchedulerManagedTasks))
	}
	allowed := make(map[string]bool, len(antiscanSchedulerManagedTasks))
	for _, task := range antiscanSchedulerManagedTasks {
		allowed[task] = true
	}
	result := make(map[string]antiscanSchedulerTaskRequest, len(tasks))
	enabled := 0
	for _, item := range tasks {
		item.Task = strings.TrimSpace(item.Task)
		item.Schedule = strings.Join(strings.Fields(item.Schedule), " ")
		if !allowed[item.Task] {
			return nil, fmt.Errorf("scheduler task %q is not user-managed", item.Task)
		}
		if _, exists := result[item.Task]; exists {
			return nil, fmt.Errorf("duplicate scheduler task %q", item.Task)
		}
		if item.Enabled {
			if err := validateAntiscanSchedulerExpression(item.Schedule); err != nil {
				return nil, fmt.Errorf("%s: %w", item.Task, err)
			}
			enabled++
		} else if item.Schedule != "" {
			if err := validateAntiscanSchedulerExpression(item.Schedule); err != nil {
				return nil, fmt.Errorf("%s: %w", item.Task, err)
			}
		}
		result[item.Task] = item
	}
	for _, task := range antiscanSchedulerManagedTasks {
		if _, ok := result[task]; !ok {
			return nil, fmt.Errorf("scheduler request is missing task %q", task)
		}
	}
	if enabled == 0 {
		return nil, errors.New("at least one managed scheduler task must remain enabled")
	}
	return result, nil
}

func validateAntiscanSchedulerExpression(expression string) error {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) != 5 {
		return errors.New("cron expression must contain exactly five fields")
	}
	for _, field := range fields {
		if len(field) > 32 || !antiscanSchedulerFieldPattern.MatchString(field) {
			return errors.New("cron expression contains characters unsupported by upstream")
		}
	}
	return nil
}

func renderAntiscanSchedulerSource(original string, requested map[string]antiscanSchedulerTaskRequest) (string, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(original, "\r\n", "\n"), "\r", "\n")
	rows := strings.Split(normalized, "\n")
	out := make([]string, 0, len(rows)+len(antiscanSchedulerManagedTasks))
	emitted := make(map[string]bool, len(antiscanSchedulerManagedTasks))
	for _, raw := range rows {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			out = append(out, raw)
			continue
		}
		task, _, ok, err := parseAntiscanSchedulerLine(line)
		if err != nil || !ok {
			if err != nil {
				return "", err
			}
			return "", errors.New("unsupported scheduler line")
		}
		if task == "retry_load_geo" {
			out = append(out, raw)
			continue
		}
		item, exists := requested[task]
		if !exists {
			return "", fmt.Errorf("missing requested scheduler task %q", task)
		}
		emitted[task] = true
		if item.Enabled {
			out = append(out, canonicalAntiscanSchedulerLine(task, item.Schedule))
		}
	}
	for _, task := range antiscanSchedulerManagedTasks {
		if emitted[task] {
			continue
		}
		item := requested[task]
		if item.Enabled {
			out = append(out, canonicalAntiscanSchedulerLine(task, item.Schedule))
		}
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n") + "\n", nil
}

func canonicalAntiscanSchedulerLine(task, schedule string) string {
	return strings.Join(strings.Fields(schedule), " ") + " /opt/etc/init.d/S99ascn " + task + " &"
}

func applyAntiscanScheduler(parent context.Context, cfg runtimeConfig, request antiscanSchedulerRequest) (antiscanSchedulerResult, int, error) {
	result := antiscanSchedulerResult{
		Action:      "apply",
		MutationAPI: true,
		Warnings:    []string{},
	}
	requested, err := normalizeAntiscanSchedulerRequest(request.Tasks)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	current, err := readAntiscanSchedulerSource(cfg)
	if err != nil {
		return result, http.StatusConflict, err
	}
	if !current.Valid {
		return result, http.StatusConflict, errors.New("ascn_crontab.conf contains lines outside the pinned upstream scheduler contract")
	}
	if strings.TrimSpace(request.BaseSHA256) == "" || request.BaseSHA256 != current.SHA256 {
		return result, http.StatusConflict, errors.New("ascn_crontab.conf changed since it was loaded; refresh before applying")
	}
	beforeActive, err := readAntiscanActiveCron(parent)
	if err != nil {
		return result, http.StatusConflict, err
	}

	rendered, err := renderAntiscanSchedulerSource(string(current.Data), requested)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	newData := []byte(rendered)
	newTasks, newLines, parseErrors := parseAntiscanSchedulerSource(rendered)
	if len(parseErrors) > 0 {
		return result, http.StatusBadRequest, errors.New("rendered scheduler configuration failed validation")
	}
	if len(newLines) == 0 {
		return result, http.StatusBadRequest, errors.New("scheduler configuration must contain at least one Antiscan task")
	}

	result.AutoRetry = newTasks["retry_load_geo"] != ""
	result.TaskCount = len(newLines)
	result.Changed = !bytes.Equal(current.Data, newData)
	if result.AutoRetry {
		result.Warnings = append(result.Warnings, "retry_load_geo is managed automatically by upstream Antiscan and was preserved unchanged.")
	}

	beforeSynced := sameStringSet(current.TaskLines, beforeActive)
	if !result.Changed && beforeSynced {
		result.Verified = true
		result.Synced = true
		result.SourceSHA256 = current.SHA256
		result.ActiveCount = len(beforeActive)
		return result, http.StatusOK, nil
	}

	if result.Changed {
		if err := safety.WriteFileAtomic(current.Path, newData, current.Mode); err != nil {
			return result, http.StatusInternalServerError, fmt.Errorf("write ascn_crontab.conf: %w", err)
		}
	}

	updateOutput, updateErr := runAntiscanSchedulerUpdate(parent, cfg)
	if updateErr != nil {
		rollbackOK := rollbackAntiscanScheduler(parent, cfg, current, beforeActive)
		result.RollbackPerformed = rollbackOK
		if !rollbackOK {
			return result, http.StatusInternalServerError, errors.New("upstream update_crontab failed and previous scheduler state could not be restored")
		}
		return result, http.StatusConflict, fmt.Errorf("upstream update_crontab failed; previous scheduler state was restored: %s", compactAntiscanSchedulerOutput(updateOutput, updateErr))
	}

	afterActive, err := readAntiscanActiveCron(parent)
	if err != nil || !sameStringSet(newLines, afterActive) {
		rollbackOK := rollbackAntiscanScheduler(parent, cfg, current, beforeActive)
		result.RollbackPerformed = rollbackOK
		if !rollbackOK {
			return result, http.StatusInternalServerError, errors.New("scheduler verification failed and previous scheduler state could not be restored")
		}
		return result, http.StatusConflict, errors.New("scheduler verification failed; previous scheduler state was restored")
	}

	published, err := readAntiscanSchedulerSource(cfg)
	if err != nil {
		return result, http.StatusInternalServerError, err
	}
	result.SourceSHA256 = published.SHA256
	result.ActiveCount = len(afterActive)
	result.Verified = true
	result.Synced = true
	return result, http.StatusOK, nil
}

func rollbackAntiscanScheduler(parent context.Context, cfg runtimeConfig, source antiscanSchedulerSource, beforeActive []string) bool {
	if err := safety.WriteFileAtomic(source.Path, source.Data, source.Mode); err != nil {
		return false
	}
	active, err := readAntiscanActiveCron(parent)
	if err == nil && sameStringSet(beforeActive, active) {
		return true
	}
	if _, err := runAntiscanSchedulerUpdate(parent, cfg); err != nil {
		return false
	}
	active, err = readAntiscanActiveCron(parent)
	return err == nil && sameStringSet(beforeActive, active)
}

func runAntiscanSchedulerUpdate(parent context.Context, cfg runtimeConfig) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, antiscanSchedulerTimeout)
	defer cancel()
	return safety.RunCommand(ctx, 16<<10, cfg.InitScript, "update_crontab")
}

func readAntiscanActiveCron(parent context.Context) ([]string, error) {
	binary := antiscanSchedulerFindCrontab()
	if binary == "" {
		return nil, errors.New("crontab binary not found")
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	output, err := safety.RunCommand(ctx, 64<<10, binary, "-l")
	if err != nil {
		text := strings.ToLower(strings.TrimSpace(string(output)))
		if strings.Contains(text, "no crontab") || strings.Contains(text, "no crontab for") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read active crontab: %s", compactAntiscanSchedulerOutput(output, err))
	}
	return extractAntiscanSchedulerLines(string(output)), nil
}

func extractAntiscanSchedulerLines(text string) []string {
	lines := []string{}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || !strings.Contains(line, "/opt/etc/init.d/S99ascn ") {
			continue
		}
		task, schedule, ok, err := parseAntiscanSchedulerLine(line)
		if err != nil || !ok {
			lines = append(lines, strings.Join(strings.Fields(line), " "))
			continue
		}
		lines = append(lines, canonicalAntiscanSchedulerLine(task, schedule))
	}
	sort.Strings(lines)
	return lines
}

func antiscanSchedulerSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func compactAntiscanSchedulerOutput(output []byte, err error) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 500 {
		text = text[:500] + "…"
	}
	if text != "" {
		return text
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}
