package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const dpiDetectorV5ConsoleOutputMax = 4 << 20

type dpiDetectorV5ConsoleResponse struct {
	OK                        bool   `json:"ok"`
	Tests                     string `json:"tests"`
	Path                      string `json:"path"`
	Upstream                  string `json:"upstream"`
	DurationMS                int64  `json:"duration_ms"`
	ConsoleReport             string `json:"console_report"`
	ReportBytes               int    `json:"report_bytes"`
	ReportCut                 bool   `json:"report_cut"`
	ExitWarning               string `json:"exit_warning,omitempty"`
	NFQWS2Running             bool   `json:"nfqws2_running"`
	RawProviderTruth          bool   `json:"raw_provider_truth"`
	EnvironmentWarning        string `json:"environment_warning"`
	PersistentMutation        bool   `json:"persistent_mutation"`
	ProductionConfigSHA256    string `json:"production_config_sha256"`
	ProductionConfigUnchanged bool   `json:"production_config_unchanged"`
}

func buildDPIDetectorV5ConsoleArgs(req dpiDetectorV5Validated, reportPath string) []string {
	jsonArgs := buildDPIDetectorV5Args(req, "")
	args := make([]string, 0, len(jsonArgs)+3)
	for _, arg := range jsonArgs {
		if arg == "--json" {
			continue
		}
		args = append(args, arg)
	}
	args = append(args, "-o", reportPath)
	return args
}

func handleDPIDetectorV5ConsoleRun(w http.ResponseWriter, r *http.Request) {
	var req dpiDetectorV5RunRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid DPI Detector console request"})
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

	tmpDir, err := os.MkdirTemp("", "routerforge-dpi-console-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot create DPI Detector console temporary directory"})
		return
	}
	defer os.RemoveAll(tmpDir)
	reportPath := filepath.Join(tmpDir, "report.txt")

	ctx, cancel := context.WithTimeout(r.Context(), dpiDetectorV5Timeout)
	defer cancel()
	started := time.Now()
	output, runErr := dpiDetectorCommandRunner(ctx, dpiDetectorV5ConsoleOutputMax, path, buildDPIDetectorV5ConsoleArgs(validated, reportPath)...)
	durationMS := time.Since(started).Milliseconds()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{"error": "dpi-detector console run timeout", "duration_ms": durationMS})
		return
	}

	reportData, reportCut, reportErr := readBoundedFile(reportPath, dpiDetectorV5ConsoleOutputMax)
	report := ""
	if reportErr == nil {
		report = normalizeDPIDetectorRunText(string(reportData))
	}
	if report == "" {
		reportData = output
		reportCut = len(output) >= dpiDetectorV5ConsoleOutputMax
		report = normalizeDPIDetectorRunText(string(output))
	}

	if report == "" {
		kind, detail := classifyDPIDetectorRunError(runErr)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": "dpi-detector console run failed", "error_kind": kind, "detail": detail,
			"duration_ms": durationMS, "output": normalizeDPIDetectorRunText(string(output)),
		})
		return
	}

	exitWarning := ""
	if runErr != nil {
		kind, detail := classifyDPIDetectorRunError(runErr)
		exitWarning = strings.TrimSpace(kind + ": " + detail)
	}

	statusAfter := dpiDetectorStatusReader()
	if !strings.EqualFold(statusBefore.ConfigSHA256, statusAfter.ConfigSHA256) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed during DPI Detector run", "before_sha256": statusBefore.ConfigSHA256, "current_sha256": statusAfter.ConfigSHA256})
		return
	}

	nfqwsRunning := dpiDetectorNFQWS2Running()
	warning := "results describe the current router path; RouterForge does not disable VPN, proxy, nfqws2, or other bypass processing"
	if nfqwsRunning {
		warning = "nfqws2 is running; results describe the current router path and must not be interpreted as raw ISP truth"
	}

	writeJSON(w, http.StatusOK, dpiDetectorV5ConsoleResponse{
		OK: true, Tests: validated.Tests, Path: path, Upstream: "Runnin4ik/dpi-detector",
		DurationMS: durationMS, ConsoleReport: report, ReportBytes: len(reportData),
		ReportCut: reportCut, ExitWarning: exitWarning,
		NFQWS2Running: nfqwsRunning, RawProviderTruth: false, EnvironmentWarning: warning,
		PersistentMutation: false, ProductionConfigSHA256: statusAfter.ConfigSHA256,
		ProductionConfigUnchanged: true,
	})
}
