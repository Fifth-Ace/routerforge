package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanAuditDefaultLimit = 50
	antiscanAuditMaxLimit     = 100
	antiscanAuditMaxBytes     = 256 << 10
	antiscanAuditKeepLines    = 120
	antiscanAuditCaptureMax   = 64 << 10
)

var antiscanAuditMu sync.Mutex

type antiscanAuditEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	Action          string    `json:"action"`
	Target          string    `json:"target,omitempty"`
	Outcome         string    `json:"outcome"`
	HTTPStatus      int       `json:"http_status"`
	DurationMS      int64     `json:"duration_ms"`
	Changed         bool      `json:"changed"`
	Verified        bool      `json:"verified"`
	Rollback        bool      `json:"rollback"`
	RestartRequired bool      `json:"restart_required"`
	Summary         string    `json:"summary"`
	Warnings        []string  `json:"warnings,omitempty"`
}

type antiscanAuditPage struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Persistent  bool                 `json:"persistent"`
	Limit       int                  `json:"limit"`
	Events      []antiscanAuditEvent `json:"events"`
	Skipped     int                  `json:"skipped_invalid_lines"`
}

type antiscanAuditResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *antiscanAuditResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *antiscanAuditResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.body.Len() < antiscanAuditCaptureMax {
		remaining := antiscanAuditCaptureMax - w.body.Len()
		if len(data) > remaining {
			_, _ = w.body.Write(data[:remaining])
		} else {
			_, _ = w.body.Write(data)
		}
	}
	return w.ResponseWriter.Write(data)
}

func auditAntiscanMutation(cfg runtimeConfig, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestBody := []byte(nil)
		if r.Body != nil {
			requestBody, _ = io.ReadAll(io.LimitReader(r.Body, antiscanMutationBodyMax+1))
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(requestBody))
		}

		capture := &antiscanAuditResponseWriter{ResponseWriter: w}
		next(capture, r)
		if capture.status == 0 {
			capture.status = http.StatusOK
		}

		event := buildAntiscanAuditEvent(route, requestBody, capture.status, capture.body.Bytes(), time.Since(started))
		_ = recordAntiscanAudit(cfg, event)
	}
}

func buildAntiscanAuditEvent(route string, requestBody []byte, status int, responseBody []byte, elapsed time.Duration) antiscanAuditEvent {
	action, target := antiscanAuditRequestMetadata(route, requestBody)
	event := antiscanAuditEvent{
		Timestamp:  time.Now().UTC(),
		Action:     action,
		Target:     target,
		Outcome:    "success",
		HTTPStatus: status,
		DurationMS: elapsed.Milliseconds(),
		Warnings:   []string{},
	}
	if event.DurationMS < 0 {
		event.DurationMS = 0
	}
	if status >= http.StatusBadRequest {
		event.Outcome = "failed"
	}

	var payload map[string]any
	if json.Unmarshal(responseBody, &payload) == nil {
		event.Changed = auditBool(payload["changed"])
		event.Verified = auditBool(payload["verified"])
		event.Rollback = auditBool(payload["rollback_performed"])
		event.RestartRequired = auditBool(payload["restart_required"])
		event.Warnings = auditStrings(payload["warnings"], 5, 240)
		if rawError, ok := payload["error"].(string); ok && strings.TrimSpace(rawError) != "" {
			event.Summary = truncateAuditText(rawError, 500)
		}
	}
	if event.Summary == "" {
		event.Summary = antiscanAuditSuccessSummary(event)
	}
	return event
}

func antiscanAuditRequestMetadata(route string, body []byte) (string, string) {
	route = strings.TrimSpace(route)
	action := route
	target := ""
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return action, target
	}
	stringValue := func(key string) string {
		value, _ := payload[key].(string)
		return truncateAuditText(strings.TrimSpace(value), 160)
	}
	switch route {
	case "unban":
		setName, entry := stringValue("set"), stringValue("entry")
		target = strings.Trim(strings.Join([]string{setName, entry}, " · "), " ·")
	case "list-entry":
		listName, entry := stringValue("list"), stringValue("entry")
		target = strings.Trim(strings.Join([]string{listName, entry}, " · "), " ·")
	case "lifecycle":
		if sub := strings.ToLower(stringValue("action")); sub != "" {
			action = "lifecycle:" + sub
			target = sub
		}
	case "operation":
		if sub := strings.ToLower(stringValue("action")); sub != "" {
			action = "operation:" + sub
			target = sub
			if scope := strings.ToLower(stringValue("scope")); scope != "" {
				action += ":" + scope
				target += " " + scope
			}
		}
	case "config":
		target = "ascn.conf"
	}
	return action, target
}

func antiscanAuditSuccessSummary(event antiscanAuditEvent) string {
	base := map[string]string{
		"unban":             "Точечный unban завершён.",
		"list-entry":        "Пользовательский список обновлён.",
		"lifecycle:start":   "Antiscan запущен штатной командой upstream.",
		"lifecycle:stop":    "Antiscan остановлен штатной командой upstream.",
		"lifecycle:reload":  "Конфигурация Antiscan перечитана штатной командой upstream.",
		"lifecycle:restart": "Antiscan перезапущен штатной командой upstream.",
		"config":            "Транзакция ascn.conf завершена.",
	}[event.Action]
	if base == "" && strings.HasPrefix(event.Action, "operation:") {
		base = "Штатная сервисная операция Antiscan завершена."
	}
	if base == "" {
		base = "Guarded operation завершена."
	}
	if event.Rollback {
		return "Операция завершилась rollback предыдущего состояния."
	}
	if event.RestartRequired {
		return base + " Для полного применения требуется stop/start."
	}
	return base
}

func parseAntiscanAuditLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return antiscanAuditDefaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > antiscanAuditMaxLimit {
		return 0, errors.New("history limit must be between 1 and 100")
	}
	return limit, nil
}

func recordAntiscanAudit(cfg runtimeConfig, event antiscanAuditEvent) error {
	path := strings.TrimSpace(cfg.AuditLog)
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return errors.New("Antiscan audit log path must be absolute")
	}
	event.Action = truncateAuditText(event.Action, 80)
	event.Target = truncateAuditText(event.Target, 160)
	event.Summary = truncateAuditText(event.Summary, 500)
	event.Warnings = auditStrings(event.Warnings, 5, 240)
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(line) > 4096 {
		return errors.New("Antiscan audit event exceeds safety limit")
	}

	antiscanAuditMu.Lock()
	defer antiscanAuditMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create audit directory: %w", err)
	}
	if err := compactAntiscanAuditLocked(path); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("chmod audit log: %w", err)
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("append audit log: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync audit log: %w", err)
	}
	return file.Close()
}

func compactAntiscanAuditLocked(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat audit log: %w", err)
	}
	if info.Size() <= antiscanAuditMaxBytes {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read audit log for compaction: %w", err)
	}
	lines := bytes.Split(data, []byte{'\n'})
	kept := make([][]byte, 0, antiscanAuditKeepLines)
	for i := len(lines) - 1; i >= 0 && len(kept) < antiscanAuditKeepLines; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		copyLine := append([]byte(nil), line...)
		kept = append(kept, copyLine)
	}
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	compacted := bytes.Join(kept, []byte{'\n'})
	if len(compacted) > 0 {
		compacted = append(compacted, '\n')
	}
	if err := safety.WriteFileAtomic(path, compacted, 0600); err != nil {
		return fmt.Errorf("compact audit log: %w", err)
	}
	return nil
}

func readAntiscanAudit(cfg runtimeConfig, limit int) (antiscanAuditPage, error) {
	page := antiscanAuditPage{
		GeneratedAt: time.Now().UTC(),
		Persistent:  strings.TrimSpace(cfg.AuditLog) != "",
		Limit:       limit,
		Events:      []antiscanAuditEvent{},
	}
	if limit < 1 || limit > antiscanAuditMaxLimit {
		return page, errors.New("invalid Antiscan history limit")
	}
	path := strings.TrimSpace(cfg.AuditLog)
	if path == "" {
		return page, nil
	}

	antiscanAuditMu.Lock()
	defer antiscanAuditMu.Unlock()

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return page, nil
	}
	if err != nil {
		return page, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	ring := make([]antiscanAuditEvent, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event antiscanAuditEvent
		if err := json.Unmarshal(line, &event); err != nil {
			page.Skipped++
			continue
		}
		if len(ring) < limit {
			ring = append(ring, event)
		} else {
			copy(ring, ring[1:])
			ring[len(ring)-1] = event
		}
	}
	if err := scanner.Err(); err != nil {
		return page, fmt.Errorf("scan audit log: %w", err)
	}
	for left, right := 0, len(ring)-1; left < right; left, right = left+1, right-1 {
		ring[left], ring[right] = ring[right], ring[left]
	}
	page.Events = ring
	return page, nil
}

func auditBool(value any) bool {
	result, _ := value.(bool)
	return result
}

func auditStrings(value any, maxItems, maxLen int) []string {
	out := []string{}
	switch items := value.(type) {
	case []string:
		for _, item := range items {
			if len(out) >= maxItems {
				break
			}
			item = truncateAuditText(item, maxLen)
			if item != "" {
				out = append(out, item)
			}
		}
	case []any:
		for _, item := range items {
			if len(out) >= maxItems {
				break
			}
			text, _ := item.(string)
			text = truncateAuditText(text, maxLen)
			if text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}

func truncateAuditText(value string, max int) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	if max < 1 || len(value) <= max {
		return value
	}
	return strings.TrimSpace(value[:max]) + "…"
}
