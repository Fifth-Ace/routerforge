package main

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	dpiDetectorRunConfirm   = "ROUTERFORGE_DPI_DETECTOR_RUN"
	dpiDetectorRunTimeout   = 90 * time.Second
	dpiDetectorRunOutputMax = 32 << 10
	dpiDetectorReportMax    = 128 << 10
	dpiDetectorRunMaxConc   = 5
)

var (
	dpiDetectorRunActive     int32
	dpiDetectorCommandRunner = safety.RunCommand
	dpiDetectorStatusReader  = readStatus
	dpiDetectorNFQWS2Running = nfqws2Running
)

type dpiDetectorRunRequest struct {
	Test                 string `json:"test"`
	Domain               string `json:"domain"`
	Concurrency          int    `json:"concurrency,omitempty"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type dpiDetectorRunResponse struct {
	OK                        bool   `json:"ok"`
	Test                      string `json:"test"`
	UpstreamTest              string `json:"upstream_test"`
	Domain                    string `json:"domain"`
	Concurrency               int    `json:"concurrency"`
	Batch                     bool   `json:"batch"`
	Path                      string `json:"path"`
	Upstream                  string `json:"upstream"`
	DurationMS                int64  `json:"duration_ms"`
	Report                    string `json:"report"`
	ReportBytes               int    `json:"report_bytes"`
	ReportCut                 bool   `json:"report_cut"`
	NFQWS2Running             bool   `json:"nfqws2_running"`
	RawProviderTruth          bool   `json:"raw_provider_truth"`
	EnvironmentWarning        string `json:"environment_warning"`
	PersistentMutation        bool   `json:"persistent_mutation"`
	ManagedByRouterForge      bool   `json:"managed_by_routerforge"`
	ProductionConfigSHA256    string `json:"production_config_sha256"`
	ProductionConfigUnchanged bool   `json:"production_config_unchanged"`
}

func validateDPIDetectorRunRequest(req dpiDetectorRunRequest) (string, int, error) {
	if req.Confirm != dpiDetectorRunConfirm {
		return "", 0, errors.New("confirm must equal " + dpiDetectorRunConfirm)
	}
	if strings.TrimSpace(req.Test) != "domains" {
		return "", 0, errors.New("test must equal domains in D1")
	}
	if !validDPIDetectorSHA256(req.ExpectedConfigSHA256) {
		return "", 0, errors.New("expected_config_sha256 must be SHA256")
	}
	domain, err := v2NormalizeTarget(req.Domain)
	if err != nil {
		return "", 0, errors.New("invalid detector domain")
	}
	if net.ParseIP(domain) != nil || !strings.Contains(domain, ".") || strings.HasSuffix(domain, ".local") || strings.HasSuffix(domain, ".localhost") || domain == "localhost" {
		return "", 0, errors.New("detector domain must be a public-style hostname")
	}
	concurrency := req.Concurrency
	if concurrency == 0 {
		concurrency = 2
	}
	if concurrency < 1 || concurrency > dpiDetectorRunMaxConc {
		return "", 0, errors.New("concurrency must be in range 1-5")
	}
	return domain, concurrency, nil
}

func validDPIDetectorSHA256(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func buildDPIDetectorRunArgs(domain string, concurrency int, reportPath string) []string {
	return []string{
		"-t", "2",
		"-d", domain,
		"-c", strconv.Itoa(concurrency),
		"-o", reportPath,
		"--batch",
	}
}

func normalizeDPIDetectorRunText(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimSpace(value)
}

func handleDPIDetectorRun(w http.ResponseWriter, r *http.Request) {
	var req dpiDetectorRunRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid dpi-detector run request"})
		return
	}
	domain, concurrency, err := validateDPIDetectorRunRequest(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	statusBefore := dpiDetectorStatusReader()
	if !strings.EqualFold(statusBefore.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed before dpi-detector run",
			"current_sha256": statusBefore.ConfigSHA256,
		})
		return
	}

	path, info := findDPIDetector(dpiDetectorCandidatePaths)
	if path == "" || info == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":     "dpi-detector is not installed",
			"installed": false,
			"upstream":  "Runnin4ik/dpi-detector",
		})
		return
	}
	if !atomic.CompareAndSwapInt32(&dpiDetectorRunActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another dpi-detector run is active"})
		return
	}
	defer atomic.StoreInt32(&dpiDetectorRunActive, 0)

	tmpDir, err := os.MkdirTemp("", "routerforge-dpi-detector-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create dpi-detector temporary directory"})
		return
	}
	defer os.RemoveAll(tmpDir)
	reportPath := filepath.Join(tmpDir, "report.txt")

	ctx, cancel := context.WithTimeout(r.Context(), dpiDetectorRunTimeout)
	defer cancel()
	started := time.Now()
	output, runErr := dpiDetectorCommandRunner(ctx, dpiDetectorRunOutputMax, path, buildDPIDetectorRunArgs(domain, concurrency, reportPath)...)
	durationMS := time.Since(started).Milliseconds()
	if ctx.Err() != nil {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{
			"error":       "dpi-detector run timeout",
			"test":        "domains",
			"domain":      domain,
			"duration_ms": durationMS,
		})
		return
	}
	if runErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":       "dpi-detector run failed",
			"test":        "domains",
			"domain":      domain,
			"duration_ms": durationMS,
			"output":      normalizeDPIDetectorRunText(string(output)),
		})
		return
	}

	reportBytes, reportCut, err := readBoundedFile(reportPath, dpiDetectorReportMax)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  "dpi-detector completed without readable report",
			"test":   "domains",
			"domain": domain,
		})
		return
	}
	report := normalizeDPIDetectorRunText(string(reportBytes))
	if report == "" {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  "dpi-detector completed with empty report",
			"test":   "domains",
			"domain": domain,
		})
		return
	}

	statusAfter := dpiDetectorStatusReader()
	configUnchanged := strings.EqualFold(statusBefore.ConfigSHA256, statusAfter.ConfigSHA256)
	if !configUnchanged {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed during dpi-detector run",
			"before_sha256":  statusBefore.ConfigSHA256,
			"current_sha256": statusAfter.ConfigSHA256,
		})
		return
	}

	nfqwsRunning := dpiDetectorNFQWS2Running()
	warning := "results describe the current router path; RouterForge does not disable VPN, proxy, nfqws2, or other bypass processing for this run"
	if nfqwsRunning {
		warning = "nfqws2 is running; upstream warns that bypass processing can distort provider-DPI measurements, so this result is not raw ISP truth"
	}

	writeJSON(w, http.StatusOK, dpiDetectorRunResponse{
		OK:                        true,
		Test:                      "domains",
		UpstreamTest:              "2",
		Domain:                    domain,
		Concurrency:               concurrency,
		Batch:                     true,
		Path:                      path,
		Upstream:                  "Runnin4ik/dpi-detector",
		DurationMS:                durationMS,
		Report:                    report,
		ReportBytes:               len(reportBytes),
		ReportCut:                 reportCut,
		NFQWS2Running:             nfqwsRunning,
		RawProviderTruth:          false,
		EnvironmentWarning:        warning,
		PersistentMutation:        false,
		ManagedByRouterForge:      false,
		ProductionConfigSHA256:    statusAfter.ConfigSHA256,
		ProductionConfigUnchanged: true,
	})
}
