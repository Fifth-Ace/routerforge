package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	appActionHistoryPath = "/opt/var/log/routerforge-app-center.jsonl"
	appActionHistoryMax  = 512 << 10
	appActionMaxLines    = 320
	appActionLineMax     = 2048
	appActionOutputMax   = 64 << 10
)

type appActionStartRequest struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Action  string `json:"action"`
	Confirm string `json:"confirm,omitempty"`
}

type appActionPreflight struct {
	Kind              string   `json:"kind"`
	Target            string   `json:"target"`
	Action            string   `json:"action"`
	Allowed           bool     `json:"allowed"`
	Reason            string   `json:"reason,omitempty"`
	Method            string   `json:"method,omitempty"`
	Packages          []string `json:"packages,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
	OptAvailableBytes uint64   `json:"opt_available_bytes,omitempty"`
	OptTotalBytes     uint64   `json:"opt_total_bytes,omitempty"`
	Entware           any      `json:"entware,omitempty"`
}

type appActionView struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Target      string    `json:"target"`
	Action      string    `json:"action"`
	State       string    `json:"state"`
	Error       string    `json:"error,omitempty"`
	Lines       []string  `json:"lines,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type appActionJob struct {
	mu          sync.Mutex
	ID          string
	Kind        string
	Target      string
	Action      string
	State       string
	Error       string
	Lines       []string
	StartedAt   time.Time
	CompletedAt time.Time
	cancel      context.CancelFunc
}

var appActionSeq uint64

var appActions = struct {
	sync.Mutex
	active string
	jobs   map[string]*appActionJob
}{
	jobs: map[string]*appActionJob{},
}

type actionLineCapture struct {
	mu       sync.Mutex
	pending  []byte
	captured bytes.Buffer
	emit     func(string)
	truncate bool
}

func (w *actionLineCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.captured.Len() < appActionOutputMax {
		remaining := appActionOutputMax - w.captured.Len()
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = w.captured.Write(p[:remaining])
		if remaining < len(p) {
			w.truncate = true
		}
	} else {
		w.truncate = true
	}

	w.pending = append(w.pending, p...)
	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimRight(string(w.pending[:index]), "\r")
		w.pending = w.pending[index+1:]
		w.emitLine(line)
	}
	for len(w.pending) > appActionLineMax {
		line := string(w.pending[:appActionLineMax])
		w.pending = w.pending[appActionLineMax:]
		w.emitLine(line)
	}
	return len(p), nil
}

func (w *actionLineCapture) emitLine(line string) {
	if w.emit == nil {
		return
	}
	line = strings.TrimSpace(line)
	if line != "" {
		w.emit(line)
	}
}

func (w *actionLineCapture) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.emitLine(strings.TrimRight(string(w.pending), "\r"))
		w.pending = nil
	}
}

func (w *actionLineCapture) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	value := w.captured.String()
	if w.truncate {
		value += "\n[output truncated]\n"
	}
	return value
}

func runCommandStreaming(ctx context.Context, program string, args []string, emit func(string)) (string, error) {
	if emit != nil {
		emit("$ " + program + " " + strings.Join(args, " "))
	}
	capture := &actionLineCapture{emit: emit}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Stdout = capture
	cmd.Stderr = capture
	err := cmd.Run()
	capture.Flush()
	return capture.String(), err
}

type catalogActionLog struct {
	mu      sync.Mutex
	builder strings.Builder
	emit    func(string)
}

func (l *catalogActionLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	_, _ = l.builder.Write(p)
	l.mu.Unlock()
	if l.emit != nil {
		for _, line := range strings.Split(string(p), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				l.emit(line)
			}
		}
	}
	return len(p), nil
}

func (l *catalogActionLog) EmitLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	l.mu.Lock()
	_, _ = l.builder.WriteString(line)
	_, _ = l.builder.WriteString("\n")
	l.mu.Unlock()
	if l.emit != nil {
		l.emit(line)
	}
}

func (l *catalogActionLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.builder.String()
}

func registerAppActionHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/apps/preflight", handleAppActionPreflight)
	mux.HandleFunc("/api/apps/actions", handleAppActions)
	mux.HandleFunc("/api/apps/actions/", handleAppActionPath)
}

func handleAppActionPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin App Center preflight rejected"})
		return
	}

	var request appActionStartRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		return
	}
	request.Kind = strings.ToLower(strings.TrimSpace(request.Kind))
	request.Target = strings.TrimSpace(request.Target)
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	preflight, err := buildAppActionPreflight(ctx, request)
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	writeCatalogJSON(w, http.StatusOK, preflight)
}

func buildAppActionPreflight(ctx context.Context, request appActionStartRequest) (appActionPreflight, error) {
	out := appActionPreflight{
		Kind:   request.Kind,
		Target: request.Target,
		Action: request.Action,
	}
	opt := readPlatformStorage("/opt")
	out.OptAvailableBytes = opt.AvailableBytes
	out.OptTotalBytes = opt.TotalBytes
	if opt.TotalBytes > 0 && opt.AvailableBytes < entwareDiskFloorBytes {
		out.Warnings = append(out.Warnings, "low free space on /opt")
	}

	switch request.Kind {
	case "entware":
		if !safeCatalogPackageName(request.Target) {
			out.Reason = "invalid package name"
			return out, nil
		}
		action := normalizeEntwareAction(request.Action)
		if action == "" {
			out.Reason = "unsupported Entware action"
			return out, nil
		}
		preflight, err := buildEntwarePreflight(ctx, request.Target, action)
		if err != nil {
			return out, err
		}
		out.Allowed = preflight.Allowed
		out.Reason = preflight.Reason
		out.Packages = []string{request.Target}
		out.Warnings = append(out.Warnings, preflight.Warnings...)
		out.Entware = preflight
		return out, nil

	case "catalog":
		item, ok := catalogItemByID(request.Target)
		if !ok {
			out.Reason = "catalog item not found"
			return out, nil
		}
		if item.ID == "routerforge-core" {
			out.Reason = "RouterForge Core self-update uses the synchronous replacement path"
			return out, nil
		}
		if request.Action != "install" && request.Action != "update" && request.Action != "remove" {
			out.Reason = "unsupported catalog action"
			return out, nil
		}
		if !catalogActionAllowed(item, request.Action) {
			out.Reason = item.Actions.Reason
			if out.Reason == "" {
				out.Reason = "lifecycle action is not approved or not declared"
			}
			return out, nil
		}
		plan := catalogPlanForAction(item, request.Action)
		if !executableCatalogPlan(plan) {
			out.Reason = "catalog lifecycle has no executable plan"
			return out, nil
		}
		out.Method = plan.Method
		out.Packages = append([]string(nil), plan.Packages...)
		if len(out.Packages) == 0 {
			out.Packages = append([]string(nil), item.Detection.Packages...)
		}
		if request.Action == "remove" && request.Confirm != item.Name {
			out.Reason = "typed removal confirmation does not match item name"
			return out, nil
		}
		for _, step := range plan.Steps {
			switch step.Type {
			case "write-opkg-feed":
				out.Warnings = append(out.Warnings, "lifecycle writes an opkg feed definition")
			case "opkg-remove":
				out.Warnings = append(out.Warnings, "lifecycle removes one or more packages")
			}
		}
		out.Allowed = marketplaceTestInstallEnabled()
		if !out.Allowed {
			out.Reason = "RouterForge package management is disabled"
		}
		return out, nil
	default:
		out.Reason = "unsupported App Center action kind"
		return out, nil
	}
}

func handleAppActions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeCatalogJSON(w, http.StatusOK, map[string]any{"items": listAppActionViews()})
		return
	case http.MethodPost:
		if !sameOriginRequest(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin App Center action rejected"})
			return
		}
		if !marketplaceTestInstallEnabled() {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{
				"error":  "RouterForge package management is disabled",
				"marker": marketplaceTestInstallMarker,
			})
			return
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or POST required"})
		return
	}

	var request appActionStartRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		return
	}
	request.Kind = strings.ToLower(strings.TrimSpace(request.Kind))
	request.Target = strings.TrimSpace(request.Target)
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	preflight, err := buildAppActionPreflight(ctx, request)
	cancel()
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	if !preflight.Allowed {
		writeCatalogJSON(w, http.StatusConflict, map[string]any{
			"error":     "App Center action rejected by preflight",
			"detail":    preflight.Reason,
			"preflight": preflight,
		})
		return
	}

	appActions.Lock()
	if appActions.active != "" {
		active := appActions.active
		appActions.Unlock()
		writeCatalogJSON(w, http.StatusConflict, map[string]any{
			"error":     "another App Center action is already running",
			"active_id": active,
		})
		return
	}

	id := fmt.Sprintf("%d-%d", time.Now().Unix(), atomic.AddUint64(&appActionSeq, 1))
	runCtx, runCancel := context.WithTimeout(context.Background(), 180*time.Second)
	job := &appActionJob{
		ID:        id,
		Kind:      request.Kind,
		Target:    request.Target,
		Action:    request.Action,
		State:     "queued",
		StartedAt: time.Now(),
		cancel:    runCancel,
	}
	appActions.jobs[id] = job
	appActions.active = id
	appActions.Unlock()

	go runAppActionJob(runCtx, job, request)
	writeCatalogJSON(w, http.StatusAccepted, job.snapshot())
}

func handleAppActionPath(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/apps/actions/")
	suffix = strings.Trim(suffix, "/")
	if suffix == "" {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "action id required"})
		return
	}
	parts := strings.Split(suffix, "/")
	id := parts[0]
	if len(parts) == 2 && parts[1] == "events" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
			return
		}
		streamAppActionEvents(w, r, id)
		return
	}
	if len(parts) != 1 {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "unknown App Center action path"})
		return
	}

	job := findAppActionJob(id)
	if job == nil {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "action not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeCatalogJSON(w, http.StatusOK, job.snapshot())
	case http.MethodDelete:
		if !sameOriginRequest(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin App Center cancellation rejected"})
			return
		}
		view := job.snapshot()
		if !terminalAppActionState(view.State) && job.cancel != nil {
			job.cancel()
		}
		writeCatalogJSON(w, http.StatusAccepted, job.snapshot())
	default:
		w.Header().Set("Allow", "GET, DELETE")
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or DELETE required"})
	}
}

func runAppActionJob(ctx context.Context, job *appActionJob, request appActionStartRequest) {
	job.setState("running", "")
	job.appendLine("App Center action started")

	var runErr error
	switch request.Kind {
	case "catalog":
		_, runErr = runCatalogModuleActionWithLogger(
			ctx,
			request.Target,
			request.Action,
			request.Confirm,
			job.appendLine,
		)
	case "entware":
		_, runErr = runEntwareJob(ctx, request, job.appendLine)
	default:
		runErr = fmt.Errorf("unsupported action kind")
	}

	switch {
	case ctx.Err() == context.Canceled:
		job.setState("cancelled", "cancelled")
	case ctx.Err() == context.DeadlineExceeded:
		job.setState("failed", "action timeout")
	case runErr != nil:
		job.setState("failed", runErr.Error())
	default:
		job.setState("succeeded", "")
	}

	view := job.snapshot()
	_ = appendAppActionHistory(view)

	appActions.Lock()
	if appActions.active == job.ID {
		appActions.active = ""
	}
	appActions.Unlock()
}

func runEntwareJob(ctx context.Context, request appActionStartRequest, emit func(string)) (entwareActionResult, error) {
	marketplaceInstallMu.Lock()
	defer marketplaceInstallMu.Unlock()

	action := normalizeEntwareAction(request.Action)
	preflight, err := buildEntwarePreflight(ctx, request.Target, action)
	if err != nil {
		return entwareActionResult{}, err
	}
	if !preflight.Allowed {
		return entwareActionResult{}, fmt.Errorf("preflight rejected action: %s", preflight.Reason)
	}
	if action == "remove" && request.Confirm != request.Target {
		return entwareActionResult{}, fmt.Errorf("typed removal confirmation does not match package name")
	}

	opkg, err := opkgExecutable()
	if err != nil {
		return entwareActionResult{}, err
	}
	args := []string{entwareOpkgVerb(action), request.Target}
	output, runErr := runCommandStreaming(ctx, opkg, args, emit)
	result := entwareActionResult{
		Package: request.Target,
		Action:  action,
		Output:  truncateCatalogInstallOutput(output, entwareOutputMaxBytes),
	}
	if runErr != nil {
		return result, fmt.Errorf("opkg %s failed: %s", action, strings.TrimSpace(result.Output))
	}

	installed, err := loadOpkgInstalled(ctx, opkg)
	if err != nil {
		return result, fmt.Errorf("post-action verification failed: %w", err)
	}
	result.Version, result.Installed = installed[request.Target]
	result.CompletedAt = time.Now()

	if action == "remove" && result.Installed {
		return result, fmt.Errorf("opkg remove completed but package is still installed")
	}
	if action != "remove" && !result.Installed {
		return result, fmt.Errorf("opkg action completed but package is not installed")
	}
	invalidateEntwareCatalog()
	refreshCatalog()
	return result, nil
}

func (j *appActionJob) appendLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if len(line) > appActionLineMax {
		line = line[:appActionLineMax] + "..."
	}
	j.mu.Lock()
	j.Lines = append(j.Lines, line)
	if len(j.Lines) > appActionMaxLines {
		j.Lines = append([]string(nil), j.Lines[len(j.Lines)-appActionMaxLines:]...)
	}
	j.mu.Unlock()
}

func (j *appActionJob) setState(state, errorText string) {
	j.mu.Lock()
	j.State = state
	j.Error = truncateCatalogInstallOutput(strings.TrimSpace(errorText), 4000)
	if terminalAppActionState(state) {
		j.CompletedAt = time.Now()
	}
	j.mu.Unlock()
}

func (j *appActionJob) snapshot() appActionView {
	j.mu.Lock()
	defer j.mu.Unlock()
	return appActionView{
		ID:          j.ID,
		Kind:        j.Kind,
		Target:      j.Target,
		Action:      j.Action,
		State:       j.State,
		Error:       j.Error,
		Lines:       append([]string(nil), j.Lines...),
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
	}
}

func findAppActionJob(id string) *appActionJob {
	appActions.Lock()
	defer appActions.Unlock()
	return appActions.jobs[id]
}

func terminalAppActionState(state string) bool {
	return state == "succeeded" || state == "failed" || state == "cancelled"
}

func listAppActionViews() []appActionView {
	viewsByID := map[string]appActionView{}
	for _, view := range readAppActionHistory() {
		viewsByID[view.ID] = view
	}

	appActions.Lock()
	for id, job := range appActions.jobs {
		viewsByID[id] = job.snapshot()
	}
	appActions.Unlock()

	views := make([]appActionView, 0, len(viewsByID))
	for _, view := range viewsByID {
		if len(view.Lines) > 20 {
			view.Lines = append([]string(nil), view.Lines[len(view.Lines)-20:]...)
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool {
		return views[i].StartedAt.After(views[j].StartedAt)
	})
	if len(views) > 50 {
		views = views[:50]
	}
	return views
}

func streamAppActionEvents(w http.ResponseWriter, r *http.Request, id string) {
	job := findAppActionJob(id)
	if job == nil {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "action not found"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{"error": "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	lastLine := 0
	lastState := ""
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	send := func(value any) bool {
		data, err := json.Marshal(value)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	for {
		view := job.snapshot()
		for lastLine < len(view.Lines) {
			if !send(map[string]any{"type": "line", "line": view.Lines[lastLine]}) {
				return
			}
			lastLine++
		}
		if view.State != lastState {
			if !send(map[string]any{
				"type":         "state",
				"id":           view.ID,
				"state":        view.State,
				"error":        view.Error,
				"completed_at": view.CompletedAt,
			}) {
				return
			}
			lastState = view.State
		}
		if terminalAppActionState(view.State) {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func appendAppActionHistory(view appActionView) error {
	if len(view.Lines) > 80 {
		view.Lines = append([]string(nil), view.Lines[len(view.Lines)-80:]...)
	}
	if err := os.MkdirAll(filepath.Dir(appActionHistoryPath), 0755); err != nil {
		return err
	}
	if info, err := os.Stat(appActionHistoryPath); err == nil && info.Size() >= appActionHistoryMax {
		_ = os.Remove(appActionHistoryPath + ".1")
		_ = os.Rename(appActionHistoryPath, appActionHistoryPath+".1")
	}
	file, err := os.OpenFile(appActionHistoryPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(view)
}

func readAppActionHistory() []appActionView {
	data, err := os.ReadFile(appActionHistoryPath)
	if err != nil {
		return nil
	}
	if len(data) > appActionHistoryMax {
		data = data[len(data)-appActionHistoryMax:]
		if index := bytes.IndexByte(data, '\n'); index >= 0 {
			data = data[index+1:]
		}
	}
	var out []appActionView
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var view appActionView
		if json.Unmarshal(line, &view) == nil && view.ID != "" {
			out = append(out, view)
		}
	}
	return out
}
