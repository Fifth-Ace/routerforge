package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	dpiDetectorV5Confirm        = "ROUTERFORGE_DPI_DETECTOR_V5_RUN"
	dpiDetectorV5Timeout        = 10 * time.Minute
	dpiDetectorV5StdoutMax      = 2 << 20
	dpiDetectorV5TraceMax       = 512 << 10
	dpiDetectorV5MaxDomains     = 128
	dpiDetectorV5MaxConcurrency = 100
)

var dpiDetectorV5Fingerprints = map[string]struct{}{
	"rustls": {}, "chrome146": {}, "chrome131": {}, "chrome131_android": {},
	"chrome123": {}, "chrome116": {}, "chrome107": {}, "chrome99_android": {},
	"edge101": {}, "firefox147": {}, "firefox133": {}, "safari260": {},
	"safari260_ios": {}, "safari184_ios": {}, "safari180": {}, "safari172_ios": {},
	"safari170": {}, "safari155": {}, "safari153": {}, "tor145": {},
}

type dpiDetectorV5RunRequest struct {
	Tests                string   `json:"tests"`
	Domains              []string `json:"domains,omitempty"`
	DomainsFile          string   `json:"domains_file,omitempty"`
	TCP16File            string   `json:"tcp16_file,omitempty"`
	Concurrency          int      `json:"concurrency,omitempty"`
	Language             string   `json:"language,omitempty"`
	Profile              string   `json:"profile,omitempty"`
	Interface            string   `json:"interface,omitempty"`
	Proxy                string   `json:"proxy,omitempty"`
	Fingerprint          string   `json:"fingerprint,omitempty"`
	Burst                int      `json:"burst,omitempty"`
	BurstTimeout         int      `json:"burst_timeout,omitempty"`
	BurstProfiles        string   `json:"burst_profiles,omitempty"`
	BurstTLS             string   `json:"burst_tls,omitempty"`
	BurstALPN            string   `json:"burst_alpn,omitempty"`
	Trace                bool     `json:"trace,omitempty"`
	ExpectedConfigSHA256 string   `json:"expected_config_sha256"`
	Confirm              string   `json:"confirm"`
}

type dpiDetectorV5RunResponse struct {
	OK                        bool            `json:"ok"`
	Tests                     string          `json:"tests"`
	Path                      string          `json:"path"`
	Upstream                  string          `json:"upstream"`
	DurationMS                int64           `json:"duration_ms"`
	Report                    json.RawMessage `json:"report"`
	Stderr                    string          `json:"stderr,omitempty"`
	StderrCut                 bool            `json:"stderr_cut,omitempty"`
	Trace                     string          `json:"trace,omitempty"`
	TraceCut                  bool            `json:"trace_cut,omitempty"`
	NFQWS2Running             bool            `json:"nfqws2_running"`
	RawProviderTruth          bool            `json:"raw_provider_truth"`
	EnvironmentWarning        string          `json:"environment_warning"`
	PersistentMutation        bool            `json:"persistent_mutation"`
	ProductionConfigSHA256    string          `json:"production_config_sha256"`
	ProductionConfigUnchanged bool            `json:"production_config_unchanged"`
}

type dpiDetectorV5ReportHeader struct {
	SchemaVersion int    `json:"schema_version"`
	Version       string `json:"version"`
}

type dpiDetectorV5Validated struct {
	Tests         string
	Domains       []string
	DomainsFile   string
	TCP16File     string
	Concurrency   int
	Language      string
	Profile       string
	Interface     string
	Proxy         string
	Fingerprint   string
	Burst         int
	BurstTimeout  int
	BurstProfiles string
	BurstTLS      string
	BurstALPN     string
	Trace         bool
}

func normalizeDPIDetectorV5Tests(value string) (string, error) {
	seen := map[byte]bool{}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch < '0' || ch > '6' {
			return "", errors.New("tests must contain only digits 0-6; legend is a separate Web action")
		}
		seen[ch] = true
	}
	var out strings.Builder
	for ch := byte('0'); ch <= '6'; ch++ {
		if seen[ch] {
			out.WriteByte(ch)
		}
	}
	if out.Len() == 0 {
		return "", errors.New("select at least one DPI Detector test")
	}
	return out.String(), nil
}

func normalizeDPIDetectorV5Domain(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.TrimSuffix(value, ".")
	if value == "" || len(value) > 253 || net.ParseIP(value) != nil || !strings.Contains(value, ".") || value == "localhost" || strings.HasSuffix(value, ".local") || strings.HasSuffix(value, ".localhost") {
		return "", errors.New("invalid public-style detector domain")
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid public-style detector domain")
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
				return "", errors.New("invalid public-style detector domain")
			}
		}
	}
	return value, nil
}

func validateDPIDetectorV5Path(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	clean := filepath.Clean(value)
	if !filepath.IsAbs(clean) || clean != value || !(clean == "/opt" || strings.HasPrefix(clean, "/opt/") || clean == "/tmp" || strings.HasPrefix(clean, "/tmp/")) {
		return "", errors.New("custom input path must be an absolute clean path under /opt or /tmp")
	}
	info, err := os.Stat(clean)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("custom input path must name an existing regular file")
	}
	return clean, nil
}

func validateDPIDetectorV5Proxy(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" {
		return "", errors.New("proxy URL is invalid")
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h", "http", "https":
		return value, nil
	default:
		return "", errors.New("proxy scheme must be socks5, socks5h, http, or https")
	}
}

func validateDPIDetectorV5Request(req dpiDetectorV5RunRequest) (dpiDetectorV5Validated, error) {
	var out dpiDetectorV5Validated
	if req.Confirm != dpiDetectorV5Confirm {
		return out, errors.New("confirm must equal " + dpiDetectorV5Confirm)
	}
	if !validDPIDetectorSHA256(req.ExpectedConfigSHA256) {
		return out, errors.New("expected_config_sha256 must be SHA256")
	}
	tests, err := normalizeDPIDetectorV5Tests(strings.TrimSpace(req.Tests))
	if err != nil {
		return out, err
	}
	out.Tests = tests

	if len(req.Domains) > dpiDetectorV5MaxDomains {
		return out, errors.New("too many domains")
	}
	seenDomains := map[string]bool{}
	for _, raw := range req.Domains {
		domain, err := normalizeDPIDetectorV5Domain(raw)
		if err != nil {
			return out, err
		}
		if !seenDomains[domain] {
			seenDomains[domain] = true
			out.Domains = append(out.Domains, domain)
		}
	}

	out.DomainsFile, err = validateDPIDetectorV5Path(req.DomainsFile)
	if err != nil {
		return out, err
	}
	out.TCP16File, err = validateDPIDetectorV5Path(req.TCP16File)
	if err != nil {
		return out, err
	}

	out.Concurrency = req.Concurrency
	if out.Concurrency == 0 {
		out.Concurrency = 20
	}
	if out.Concurrency < 1 || out.Concurrency > dpiDetectorV5MaxConcurrency {
		return out, errors.New("concurrency must be in range 1-100")
	}

	out.Language = strings.ToLower(strings.TrimSpace(req.Language))
	if out.Language == "" {
		out.Language = "ru"
	}
	switch out.Language {
	case "ru", "en", "zh", "fa":
	default:
		return out, errors.New("language must be ru, en, zh, or fa")
	}

	out.Profile = strings.ToLower(strings.TrimSpace(req.Profile))
	if out.Profile == "" {
		out.Profile = "ru"
	}
	switch out.Profile {
	case "ru", "ir", "cn", "global":
	default:
		return out, errors.New("profile must be ru, ir, cn, or global")
	}

	out.Interface = strings.TrimSpace(req.Interface)
	if len(out.Interface) > 128 || strings.ContainsAny(out.Interface, "\r\n\x00") {
		return out, errors.New("interface value is invalid")
	}
	out.Proxy, err = validateDPIDetectorV5Proxy(req.Proxy)
	if err != nil {
		return out, err
	}

	out.Fingerprint = strings.ToLower(strings.TrimSpace(req.Fingerprint))
	if out.Fingerprint != "" {
		if _, ok := dpiDetectorV5Fingerprints[out.Fingerprint]; !ok {
			return out, errors.New("unknown TLS fingerprint")
		}
	}

	out.Burst = req.Burst
	if out.Burst == 0 {
		out.Burst = 5
	}
	if out.Burst < 1 || out.Burst > 100 {
		return out, errors.New("burst must be in range 1-100")
	}
	out.BurstTimeout = req.BurstTimeout
	if out.BurstTimeout == 0 {
		out.BurstTimeout = 8
	}
	if out.BurstTimeout < 1 || out.BurstTimeout > 120 {
		return out, errors.New("burst_timeout must be in range 1-120")
	}
	out.BurstProfiles = strings.TrimSpace(req.BurstProfiles)
	if len(out.BurstProfiles) > 1024 || strings.ContainsAny(out.BurstProfiles, "\r\n\x00") {
		return out, errors.New("burst_profiles is invalid")
	}
	out.BurstTLS = strings.TrimSpace(req.BurstTLS)
	if out.BurstTLS == "" {
		out.BurstTLS = "1.3+1.2"
	}
	switch out.BurstTLS {
	case "1.3+1.2", "1.3", "1.2":
	default:
		return out, errors.New("burst_tls must be 1.3+1.2, 1.3, or 1.2")
	}
	out.BurstALPN = strings.ToLower(strings.TrimSpace(req.BurstALPN))
	if out.BurstALPN == "" {
		out.BurstALPN = "h2"
	}
	switch out.BurstALPN {
	case "h2", "http/1.1":
	default:
		return out, errors.New("burst_alpn must be h2 or http/1.1")
	}
	out.Trace = req.Trace
	return out, nil
}

func buildDPIDetectorV5Args(req dpiDetectorV5Validated, tracePath string) []string {
	args := []string{"--json", "-t", req.Tests, "-l", req.Language, "--profile", req.Profile, "-c", strconv.Itoa(req.Concurrency)}
	if req.Interface != "" {
		args = append(args, "--iface", req.Interface)
	}
	if req.Proxy != "" {
		args = append(args, "--proxy", req.Proxy)
	}
	if req.Fingerprint != "" {
		args = append(args, "--fingerprint", req.Fingerprint)
	}
	for _, domain := range req.Domains {
		args = append(args, "-d", domain)
	}
	if req.DomainsFile != "" {
		args = append(args, "--domains", req.DomainsFile)
	}
	if req.TCP16File != "" {
		args = append(args, "--tcp16", req.TCP16File)
	}
	if strings.Contains(req.Tests, "6") {
		args = append(args,
			"--burst", strconv.Itoa(req.Burst),
			"--burst-timeout", strconv.Itoa(req.BurstTimeout),
			"--burst-tls", req.BurstTLS,
			"--burst-alpn", req.BurstALPN,
		)
		if req.BurstProfiles != "" {
			args = append(args, "--burst-profiles", req.BurstProfiles)
		}
		if req.Trace && tracePath != "" {
			args = append(args, "--trace", tracePath)
		}
	}
	return args
}

func dpiDetectorV5VersionOK(version string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(version)))
	for _, field := range fields {
		value := strings.TrimPrefix(field, "v")
		if strings.HasPrefix(value, "5.") {
			return true
		}
	}
	return false
}

func runDPIDetectorV5Command(ctx context.Context, path string, args ...string) ([]byte, string, bool, error) {
	output, err := safety.RunCommandOutput(ctx, path, args...)
	if len(output) > dpiDetectorV5StdoutMax {
		output = output[:dpiDetectorV5StdoutMax]
	}
	return output, "", false, err
}

func handleDPIDetectorV5Run(w http.ResponseWriter, r *http.Request) {
	var req dpiDetectorV5RunRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid DPI Detector v5 request"})
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

	tmpDir, err := os.MkdirTemp("", "routerforge-dpi-v5-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot create DPI Detector temporary directory"})
		return
	}
	defer os.RemoveAll(tmpDir)
	tracePath := ""
	if validated.Trace && strings.Contains(validated.Tests, "6") {
		tracePath = filepath.Join(tmpDir, "trace.txt")
	}

	ctx, cancel := context.WithTimeout(r.Context(), dpiDetectorV5Timeout)
	defer cancel()
	started := time.Now()
	stdout, stderrText, stderrCut, runErr := runDPIDetectorV5Command(ctx, path, buildDPIDetectorV5Args(validated, tracePath)...)
	durationMS := time.Since(started).Milliseconds()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{"error": "dpi-detector v5 run timeout", "duration_ms": durationMS})
		return
	}
	if runErr != nil {
		kind, detail := classifyDPIDetectorRunError(runErr)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "dpi-detector v5 run failed", "error_kind": kind, "detail": detail, "stderr": stderrText, "duration_ms": durationMS})
		return
	}

	var header dpiDetectorV5ReportHeader
	if err := json.Unmarshal(stdout, &header); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "dpi-detector returned invalid JSON", "detail": err.Error()})
		return
	}
	if header.SchemaVersion != 1 || !dpiDetectorV5VersionOK(header.Version) {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "unsupported DPI Detector JSON schema/version", "schema_version": header.SchemaVersion, "version": header.Version})
		return
	}

	statusAfter := dpiDetectorStatusReader()
	if !strings.EqualFold(statusBefore.ConfigSHA256, statusAfter.ConfigSHA256) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed during DPI Detector run", "before_sha256": statusBefore.ConfigSHA256, "current_sha256": statusAfter.ConfigSHA256})
		return
	}

	traceText := ""
	traceCut := false
	if tracePath != "" {
		if data, cut, readErr := readBoundedFile(tracePath, dpiDetectorV5TraceMax); readErr == nil {
			traceText = normalizeDPIDetectorRunText(string(data))
			traceCut = cut
		}
	}

	nfqwsRunning := dpiDetectorNFQWS2Running()
	warning := "results describe the current router path; RouterForge does not disable VPN, proxy, nfqws2, or other bypass processing"
	if nfqwsRunning {
		warning = "nfqws2 is running; results describe the current router path and must not be interpreted as raw ISP truth"
	}

	writeJSON(w, http.StatusOK, dpiDetectorV5RunResponse{
		OK:                        true,
		Tests:                     validated.Tests,
		Path:                      path,
		Upstream:                  "Runnin4ik/dpi-detector",
		DurationMS:                durationMS,
		Report:                    json.RawMessage(stdout),
		Stderr:                    stderrText,
		StderrCut:                 stderrCut,
		Trace:                     traceText,
		TraceCut:                  traceCut,
		NFQWS2Running:             nfqwsRunning,
		RawProviderTruth:          false,
		EnvironmentWarning:        warning,
		PersistentMutation:        false,
		ProductionConfigSHA256:    statusAfter.ConfigSHA256,
		ProductionConfigUnchanged: true,
	})
}

func handleDPIDetectorV5Legend(w http.ResponseWriter, r *http.Request) {
	path, info := findDPIDetector(dpiDetectorCandidatePaths)
	if path == "" || info == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "dpi-detector is not installed"})
		return
	}
	language := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang")))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en", "zh", "fa":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "lang must be ru, en, zh, or fa"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	output, err := dpiDetectorCommandRunner(ctx, 128<<10, path, "--legend", "--ascii", "-l", language)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "dpi-detector legend failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"upstream": "Runnin4ik/dpi-detector", "language": language, "legend": normalizeDPIDetectorRunText(string(output))})
}

func dpiDetectorV5FingerprintList() []string {
	out := make([]string, 0, len(dpiDetectorV5Fingerprints))
	for name := range dpiDetectorV5Fingerprints {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
