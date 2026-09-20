package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type dpiDetectorV5StreamEvent struct {
	Type                      string `json:"type"`
	Tests                     string `json:"tests,omitempty"`
	Data                      string `json:"data,omitempty"`
	Report                    string `json:"report,omitempty"`
	DurationMS                int64  `json:"duration_ms,omitempty"`
	ReportBytes               int    `json:"report_bytes,omitempty"`
	ReportCut                 bool   `json:"report_cut,omitempty"`
	ExitWarning               string `json:"exit_warning,omitempty"`
	NFQWS2Running             bool   `json:"nfqws2_running,omitempty"`
	EnvironmentWarning        string `json:"environment_warning,omitempty"`
	ProductionConfigSHA256    string `json:"production_config_sha256,omitempty"`
	ProductionConfigUnchanged bool   `json:"production_config_unchanged,omitempty"`
	Error                     string `json:"error,omitempty"`
	ErrorKind                 string `json:"error_kind,omitempty"`
	Detail                    string `json:"detail,omitempty"`
}

func handleDPIDetectorV5StreamRun(w http.ResponseWriter, r *http.Request) {
	var req dpiDetectorV5RunRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid DPI Detector stream request"})
		return
	}
	validated, err := validateDPIDetectorV5Request(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	statusBefore := dpiDetectorStatusReader()
	if !strings.EqualFold(statusBefore.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed before DPI Detector run", "current_sha256": statusBefore.ConfigSHA256})
		return
	}

	path, info := findDPIDetector(dpiDetectorCandidatePaths)
	if path == "" || info == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "dpi-detector is not installed"})
		return
	}
	compatibility := dpiDetectorCompatibilityReader(path)
	if !compatibility.ExecutionReady {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "dpi-detector binary is not executable on this rootfs", "compatibility_reason": compatibility.Reason})
		return
	}
	version, err := probeDPIDetectorVersion(path)
	if err != nil || !dpiDetectorV5VersionOK(version) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "DPI Detector v5.0.0+ is required", "version": version})
		return
	}
	if !atomic.CompareAndSwapInt32(&dpiDetectorRunActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another dpi-detector run is active"})
		return
	}
	defer atomic.StoreInt32(&dpiDetectorRunActive, 0)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "streaming response is unavailable"})
		return
	}

	tmpDir, err := os.MkdirTemp("", "routerforge-dpi-stream-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot create DPI Detector stream temporary directory"})
		return
	}
	defer os.RemoveAll(tmpDir)
	reportPath := filepath.Join(tmpDir, "report.txt")

	ctx, cancel := context.WithTimeout(r.Context(), dpiDetectorV5Timeout)
	defer cancel()

	cmd, err := safety.CommandContext(ctx, path, buildDPIDetectorV5ConsoleArgs(validated, reportPath)...)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "dpi-detector stream command rejected", "detail": err.Error()})
		return
	}
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		kind, detail := classifyDPIDetectorRunError(err)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": "dpi-detector stream start failed", "error_kind": kind, "detail": detail,
		})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	writeEvent := func(event dpiDetectorV5StreamEvent) bool {
		if err := encoder.Encode(event); err != nil {
			cancel()
			return false
		}
		flusher.Flush()
		return true
	}

	started := time.Now()
	if !writeEvent(dpiDetectorV5StreamEvent{Type: "start", Tests: validated.Tests}) {
		_ = reader.Close()
		_ = writer.Close()
		return
	}

	waitDone := make(chan error, 1)
	go func() {
		runErr := cmd.Wait()
		_ = writer.Close()
		waitDone <- runErr
	}()

	buffered := bufio.NewReaderSize(reader, 32<<10)
	var captured strings.Builder
	streamCut := false
	for {
		line, readErr := buffered.ReadString('\n')
		if line != "" {
			if !streamCut {
				if captured.Len()+len(line) <= dpiDetectorV5ConsoleOutputMax {
					captured.WriteString(line)
					if !writeEvent(dpiDetectorV5StreamEvent{Type: "chunk", Data: line}) {
						_ = reader.Close()
						return
					}
				} else {
					streamCut = true
					if !writeEvent(dpiDetectorV5StreamEvent{Type: "cut", ReportCut: true}) {
						_ = reader.Close()
						return
					}
				}
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				cancel()
			}
			break
		}
	}

	runErr := <-waitDone
	durationMS := time.Since(started).Milliseconds()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_ = writeEvent(dpiDetectorV5StreamEvent{
			Type: "error", Error: "dpi-detector stream timeout", DurationMS: durationMS,
		})
		return
	}

	reportData, reportCut, reportErr := readBoundedFile(reportPath, dpiDetectorV5ConsoleOutputMax)
	report := ""
	if reportErr == nil {
		report = normalizeDPIDetectorRunText(string(reportData))
	}
	if report == "" {
		report = normalizeDPIDetectorRunText(captured.String())
		reportData = []byte(report)
		reportCut = streamCut
	}
	if report == "" {
		kind, detail := classifyDPIDetectorRunError(runErr)
		_ = writeEvent(dpiDetectorV5StreamEvent{
			Type: "error", Error: "dpi-detector stream returned no report",
			ErrorKind: kind, Detail: detail, DurationMS: durationMS,
		})
		return
	}

	statusAfter := dpiDetectorStatusReader()
	if !strings.EqualFold(statusBefore.ConfigSHA256, statusAfter.ConfigSHA256) {
		_ = writeEvent(dpiDetectorV5StreamEvent{
			Type: "error", Error: "production config changed during DPI Detector run",
			Detail: statusAfter.ConfigSHA256, DurationMS: durationMS,
		})
		return
	}

	exitWarning := ""
	if runErr != nil {
		kind, detail := classifyDPIDetectorRunError(runErr)
		exitWarning = strings.TrimSpace(kind + ": " + detail)
	}

	nfqwsRunning := dpiDetectorNFQWS2Running()
	warning := "results describe the current router path; RouterForge does not disable VPN, proxy, nfqws2, or other bypass processing"
	if nfqwsRunning {
		warning = "nfqws2 is running; results describe the current router path and must not be interpreted as raw ISP truth"
	}

	if !writeEvent(dpiDetectorV5StreamEvent{
		Type: "snapshot", Tests: validated.Tests, Report: report,
		DurationMS: durationMS, ReportBytes: len(reportData), ReportCut: reportCut || streamCut,
	}) {
		return
	}
	_ = writeEvent(dpiDetectorV5StreamEvent{
		Type: "done", Tests: validated.Tests, DurationMS: durationMS,
		ReportBytes: len(reportData), ReportCut: reportCut || streamCut,
		ExitWarning: exitWarning, NFQWS2Running: nfqwsRunning,
		EnvironmentWarning: warning, ProductionConfigSHA256: statusAfter.ConfigSHA256,
		ProductionConfigUnchanged: true,
	})
}
