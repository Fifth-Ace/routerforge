package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	v2DetectTimeout        = 10 * time.Second
	v2HTTPProbeTimeout     = 12 * time.Second
	v2HTTPReadLimit        = 256 << 10
	v2WorkingProgressBytes = 32 << 10
	v2CutoffLowBytes       = 12 << 10
	v2CutoffHighBytes      = 20 << 10
	v2BenchConfirm         = "ROUTERFORGE_V2_BENCH"
	v2SelectorConfirm      = "ROUTERFORGE_V2_SELECTOR"
	v2MaxTargets           = 64
	v2MaxAttempts          = 5
	v2MaxConcurrency       = 4
)

type v2StageResult struct {
	State      string `json:"state"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	Detail     string `json:"detail,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
}

type v2DetectRequest struct {
	Target string `json:"target"`
}

type v2DetectResponse struct {
	OK                 bool                     `json:"ok"`
	Target             string                   `json:"target"`
	DestinationIPv4    string                   `json:"destination_ipv4,omitempty"`
	Stages             map[string]v2StageResult `json:"stages"`
	Classification     string                   `json:"classification"`
	ClassificationText string                   `json:"classification_text"`
	ResponseComplete   bool                     `json:"response_complete"`
	ProgressProven     bool                     `json:"progress_proven"`
	Cutoff16KSuspected bool                     `json:"cutoff_16k_suspected"`
	TTFBMS             int64                    `json:"ttfb_ms,omitempty"`
	DurationMS         int64                    `json:"duration_ms,omitempty"`
	ThroughputBPS      int64                    `json:"throughput_bps,omitempty"`
}

type v2HTTPMetrics struct {
	TLSComplete        bool   `json:"tls_complete"`
	HTTPStatus         int    `json:"http_status,omitempty"`
	TTFBMS             int64  `json:"ttfb_ms,omitempty"`
	DurationMS         int64  `json:"duration_ms,omitempty"`
	Bytes              int64  `json:"bytes"`
	ThroughputBPS      int64  `json:"throughput_bps,omitempty"`
	ResponseComplete   bool   `json:"response_complete"`
	ProgressProven     bool   `json:"progress_proven"`
	Cutoff16KSuspected bool   `json:"cutoff_16k_suspected"`
	ReadError          string `json:"read_error,omitempty"`
}

type v2InspectorProfile struct {
	Index          int      `json:"index"`
	StrategyTags   []int    `json:"strategy_tags,omitempty"`
	Args           []string `json:"args"`
	MatchReason    string   `json:"match_reason"`
	CandidateReady bool     `json:"candidate_ready"`
}

type v2ListMatch struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Relation string `json:"relation"`
}

type v2InspectRequest struct {
	Target string `json:"target"`
}

type v2InspectResponse struct {
	OK                       bool                 `json:"ok"`
	Target                   string               `json:"target"`
	RuntimeRunning           bool                 `json:"runtime_running"`
	ConfigSHA256             string               `json:"config_sha256"`
	ProductionSource         string               `json:"production_source"`
	ProductionPID            int                  `json:"production_pid,omitempty"`
	ProfileCount             int                  `json:"profile_count"`
	EligibleProfileCount     int                  `json:"eligible_profile_count"`
	ListMatches              []v2ListMatch        `json:"list_matches"`
	ProductionProfileMatches []v2InspectorProfile `json:"production_profile_matches"`
	BindingVerdict           string               `json:"binding_verdict"`
}

type v2TargetSource struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Count int    `json:"count"`
}

type v2TargetsResolveRequest struct {
	Source   string   `json:"source"`
	ListName string   `json:"list_name,omitempty"`
	Targets  []string `json:"targets,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

type v2TargetsResolveResponse struct {
	OK      bool     `json:"ok"`
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
	Count   int      `json:"count"`
}

type v2BenchRequest struct {
	Target               string   `json:"target"`
	ProfileIndex         *int     `json:"profile_index,omitempty"`
	Args                 []string `json:"args,omitempty"`
	Attempts             int      `json:"attempts,omitempty"`
	ExpectedConfigSHA256 string   `json:"expected_config_sha256"`
	Confirm              string   `json:"confirm"`
}

type v2BenchAttempt struct {
	OK                   bool          `json:"ok"`
	ResultClass          string        `json:"result_class"`
	SessionID            string        `json:"session_id"`
	DestinationIPv4      string        `json:"destination_ipv4"`
	LocalPort            int           `json:"local_port"`
	Queue                int           `json:"queue"`
	OutboundQueuePackets uint64        `json:"outbound_queue_packets"`
	InboundQueuePackets  uint64        `json:"inbound_queue_packets"`
	CleanupProven        bool          `json:"cleanup_proven"`
	Metrics              v2HTTPMetrics `json:"metrics"`
	Error                string        `json:"error,omitempty"`
}

type v2CandidateResult struct {
	Baseline           bool             `json:"baseline"`
	SourceProfileIndex int              `json:"source_profile_index"`
	StrategyTags       []int            `json:"strategy_tags,omitempty"`
	Args               []string         `json:"args,omitempty"`
	Attempts           []v2BenchAttempt `json:"attempts"`
	Successes          int              `json:"successes"`
	Failures           int              `json:"failures"`
	SuccessRate        float64          `json:"success_rate"`
	CompleteRate       float64          `json:"complete_rate"`
	MedianTTFBMS       int64            `json:"median_ttfb_ms,omitempty"`
	MedianDurationMS   int64            `json:"median_duration_ms,omitempty"`
	MedianThroughput   int64            `json:"median_throughput_bps,omitempty"`
	ResultClass        string           `json:"result_class"`
	CleanupProven      bool             `json:"cleanup_proven"`
}

type v2BenchResponse struct {
	OK                   bool              `json:"ok"`
	Target               string            `json:"target"`
	ConfigSHA256         string            `json:"config_sha256"`
	Source               string            `json:"source"`
	ProfileIndex         int               `json:"profile_index,omitempty"`
	Result               v2CandidateResult `json:"result"`
	CleanupBaselineAfter bool              `json:"cleanup_baseline_after"`
}

type v2SelectorRequest struct {
	Mode                 string `json:"mode"`
	ServerName           string `json:"server_name"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Concurrency          int    `json:"concurrency,omitempty"`
	Confirm              string `json:"confirm"`
}

type v2SelectorResponse struct {
	OK                      bool                `json:"ok"`
	Mode                    benchAutoTuneMode   `json:"mode"`
	ServerName              string              `json:"server_name"`
	DestinationIPv4         string              `json:"destination_ipv4"`
	MetricScope             string              `json:"metric_scope"`
	Baseline                v2CandidateResult   `json:"baseline"`
	Candidates              []v2CandidateResult `json:"candidates"`
	RecommendationAvailable bool                `json:"recommendation_available"`
	RecommendedProfileIndex int                 `json:"recommended_profile_index"`
	StrategyNeeded          bool                `json:"strategy_needed"`
	RecommendationReason    string              `json:"recommendation_reason"`
	CleanupBaselineAfter    bool                `json:"cleanup_baseline_after"`
	BenchEnabled            bool                `json:"bench_enabled"`
	SafeToBench             bool                `json:"safe_to_bench"`
	ApplyEnabled            bool                `json:"apply_enabled"`
	ApplyGateEligible       bool                `json:"apply_gate_eligible"`
	ApplyGateToken          string              `json:"apply_gate_token,omitempty"`
	ApplyGateExpiresAt      string              `json:"apply_gate_expires_at,omitempty"`
	ApplyGateReason         string              `json:"apply_gate_reason"`
	Concurrency             int                 `json:"concurrency"`
	CandidateSource         string              `json:"candidate_source"`
}

func registerStrategyIntelligenceV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/inspect-target", mutationOnly(handleV2InspectTarget))
	mux.HandleFunc("/v1/v2/detect", mutationOnly(handleV2Detect))
	mux.HandleFunc("/v1/v2/target-sources", getOnly(handleV2TargetSources))
	mux.HandleFunc("/v1/v2/targets/resolve", mutationOnly(handleV2TargetsResolve))
	mux.HandleFunc("/v1/v2/bench", mutationOnly(handleV2Bench))
	mux.HandleFunc("/v1/v2/selector", mutationOnly(handleV2Selector))
}

func v2NormalizeTarget(raw string) (string, error) {
	return normalizeBenchServerName(raw)
}

func v2DomainMatches(target, candidate string) bool {
	target = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(target)), ".")
	candidate = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(candidate)), ".")
	candidate = strings.TrimPrefix(candidate, "*.")
	candidate = strings.TrimPrefix(candidate, ".")
	return candidate != "" && (target == candidate || strings.HasSuffix(target, "."+candidate))
}

func v2ExtractDomains(text string, limit int) []string {
	if limit <= 0 || limit > v2MaxTargets {
		limit = v2MaxTargets
	}
	seen := map[string]bool{}
	out := []string{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if cut := strings.IndexAny(line, " \t#;"); cut >= 0 {
			line = strings.TrimSpace(line[:cut])
		}
		line = strings.TrimPrefix(line, "||")
		line = strings.TrimSuffix(line, "^")
		line = strings.TrimPrefix(line, "*.")
		line = strings.TrimPrefix(line, ".")
		host, err := v2NormalizeTarget(line)
		if err != nil || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func v2ListContainsTarget(text, target string) bool {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if cut := strings.IndexAny(line, " \t#;"); cut >= 0 {
			line = strings.TrimSpace(line[:cut])
		}
		line = strings.TrimPrefix(line, "||")
		line = strings.TrimSuffix(line, "^")
		line = strings.TrimPrefix(line, "*.")
		line = strings.TrimPrefix(line, ".")
		if host, err := v2NormalizeTarget(line); err == nil && v2DomainMatches(target, host) {
			return true
		}
	}
	return false
}

func v2CountDomains(text string) int {
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if cut := strings.IndexAny(line, " \t#;"); cut >= 0 {
			line = strings.TrimSpace(line[:cut])
		}
		line = strings.TrimPrefix(line, "||")
		line = strings.TrimSuffix(line, "^")
		line = strings.TrimPrefix(line, "*.")
		line = strings.TrimPrefix(line, ".")
		if host, err := v2NormalizeTarget(line); err == nil {
			seen[host] = true
		}
	}
	return len(seen)
}

func v2ReadListByName(name string) ([]byte, string, error) {
	status := readStatus()
	for _, item := range status.Lists {
		if item.Name != name {
			continue
		}
		path := filepath.Join(listsRoot, item.Name)
		data, cut, err := readBoundedFile(path, listFileMaxBytes)
		if err != nil {
			return nil, "", err
		}
		if cut {
			return nil, "", errors.New("list exceeds read limit")
		}
		return data, path, nil
	}
	return nil, "", errors.New("list not found in current runtime inventory")
}

func handleV2TargetSources(w http.ResponseWriter, _ *http.Request) {
	status := readStatus()
	out := make([]v2TargetSource, 0, len(status.Lists))
	for _, item := range status.Lists {
		data, _, err := v2ReadListByName(item.Name)
		count := 0
		if err == nil {
			count = v2CountDomains(string(data))
		}
		out = append(out, v2TargetSource{Name: item.Name, Size: item.Size, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sources": out, "max_targets": v2MaxTargets})
}

func handleV2TargetsResolve(w http.ResponseWriter, r *http.Request) {
	var req v2TargetsResolveRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid target resolve request"})
		return
	}
	limit := req.Limit
	if limit <= 0 || limit > v2MaxTargets {
		limit = v2MaxTargets
	}
	source := strings.ToLower(strings.TrimSpace(req.Source))
	var targets []string
	switch source {
	case "manual":
		seen := map[string]bool{}
		for _, raw := range req.Targets {
			target, err := v2NormalizeTarget(raw)
			if err != nil || seen[target] {
				continue
			}
			seen[target] = true
			targets = append(targets, target)
			if len(targets) >= limit {
				break
			}
		}
	case "list":
		data, _, err := v2ReadListByName(strings.TrimSpace(req.ListName))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		targets = v2ExtractDomains(string(data), limit)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "source must be manual or list"})
		return
	}
	if len(targets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no valid DNS targets resolved"})
		return
	}
	writeJSON(w, http.StatusOK, v2TargetsResolveResponse{OK: true, Source: source, Targets: targets, Count: len(targets)})
}

func v2ProfileFileRelation(arg string) (string, string) {
	for _, spec := range []struct {
		prefix   string
		relation string
	}{
		{"--hostlist=", "include"},
		{"--hostlist-auto=", "include-auto"},
		{"--hostlist-exclude=", "exclude"},
		{"--ipset=", "ipset"},
		{"--ipset-exclude=", "ipset-exclude"},
	} {
		if strings.HasPrefix(arg, spec.prefix) {
			value := strings.Trim(strings.TrimPrefix(arg, spec.prefix), "\"'")
			value = strings.TrimPrefix(value, "@")
			return filepath.Base(value), spec.relation
		}
	}
	return "", ""
}

func handleV2InspectTarget(w http.ResponseWriter, r *http.Request) {
	var req v2InspectRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid inspect request"})
		return
	}
	target, err := v2NormalizeTarget(req.Target)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	status := readStatus()
	inventory := readBenchStrategyInventory()
	listMatches := []v2ListMatch{}
	matchedLists := map[string]bool{}
	for _, item := range status.Lists {
		data, path, readErr := v2ReadListByName(item.Name)
		if readErr != nil {
			continue
		}
		if v2ListContainsTarget(string(data), target) {
			matchedLists[item.Name] = true
			listMatches = append(listMatches, v2ListMatch{Name: item.Name, Path: path, Relation: "present"})
		}
	}
	profiles := []v2InspectorProfile{}
	for _, profile := range inventory.Profiles {
		reasons := []string{}
		for _, domain := range profile.HostlistDomains {
			if v2DomainMatches(target, domain) {
				reasons = append(reasons, "hostlist-domains="+domain)
			}
		}
		for _, arg := range profile.FileBoundFilters {
			name, relation := v2ProfileFileRelation(arg)
			if name != "" && matchedLists[name] {
				reasons = append(reasons, relation+" "+name)
			}
		}
		if len(reasons) == 0 {
			continue
		}
		profiles = append(profiles, v2InspectorProfile{
			Index: profile.Index, StrategyTags: append([]int{}, profile.StrategyTags...),
			Args: append([]string{}, profile.Args...), MatchReason: strings.Join(reasons, "; "),
			CandidateReady: profile.CandidateEligible,
		})
	}
	verdict := "NO_ACTIVE_BINDING_PROVEN"
	switch len(profiles) {
	case 1:
		verdict = "ONE_PRODUCTION_PROFILE_MATCH"
	default:
		if len(profiles) > 1 {
			verdict = "MULTIPLE_PRODUCTION_PROFILE_MATCHES"
		} else if len(listMatches) > 0 {
			verdict = "LIST_MATCH_WITHOUT_PROFILE_BINDING"
		}
	}
	writeJSON(w, http.StatusOK, v2InspectResponse{
		OK: true, Target: target, RuntimeRunning: status.Running, ConfigSHA256: status.ConfigSHA256,
		ProductionSource: inventory.Source, ProductionPID: inventory.ProductionPID,
		ProfileCount: inventory.ProfileCount, EligibleProfileCount: inventory.EligibleProfileCount,
		ListMatches: listMatches, ProductionProfileMatches: profiles, BindingVerdict: verdict,
	})
}

func v2ResolvePublicIPv4(ctx context.Context, host string) (string, time.Duration, error) {
	start := time.Now()
	ip, err := resolveBenchServerIPv4(ctx, host)
	return ip, time.Since(start), err
}

func v2DirectTCP(ctx context.Context, ip string) (time.Duration, error) {
	start := time.Now()
	d := net.Dialer{Timeout: 4 * time.Second}
	conn, err := d.DialContext(ctx, "tcp4", net.JoinHostPort(ip, "443"))
	if err == nil {
		_ = conn.Close()
	}
	return time.Since(start), err
}

func v2DirectTLS(ctx context.Context, host, ip string) (time.Duration, error) {
	start := time.Now()
	d := net.Dialer{Timeout: 5 * time.Second}
	raw, err := d.DialContext(ctx, "tcp4", net.JoinHostPort(ip, "443"))
	if err != nil {
		return time.Since(start), err
	}
	defer raw.Close()
	tlsConn := tls.Client(raw, &tls.Config{
		ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: true,
		NextProtos: []string{"http/1.1"},
	})
	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = tlsConn.HandshakeContext(hctx)
	cancel()
	return time.Since(start), err
}

func v2ReadHTTPS(ctx context.Context, host, ip string, localPort int) (v2HTTPMetrics, error) {
	metrics := v2HTTPMetrics{}
	start := time.Now()
	d := net.Dialer{Timeout: 5 * time.Second}
	if localPort > 0 {
		d.LocalAddr = &net.TCPAddr{IP: net.IPv4zero, Port: localPort}
	}
	raw, err := d.DialContext(ctx, "tcp4", net.JoinHostPort(ip, "443"))
	if err != nil {
		return metrics, fmt.Errorf("tcp connect: %w", err)
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(v2HTTPProbeTimeout))
	tlsConn := tls.Client(raw, &tls.Config{
		ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: true,
		NextProtos: []string{"http/1.1"},
	})
	hctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	err = tlsConn.HandshakeContext(hctx)
	cancel()
	if err != nil {
		return metrics, fmt.Errorf("tls handshake: %w", err)
	}
	metrics.TLSComplete = tlsConn.ConnectionState().HandshakeComplete

	request := "GET / HTTP/1.1\r\nHost: " + host + "\r\nUser-Agent: RouterForge-NFQWS-V2/1\r\nAccept: */*\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(tlsConn, request); err != nil {
		return metrics, fmt.Errorf("http write: %w", err)
	}
	reader := bufio.NewReader(tlsConn)
	if _, err := reader.Peek(1); err != nil {
		return metrics, fmt.Errorf("http first byte: %w", err)
	}
	metrics.TTFBMS = time.Since(start).Milliseconds()
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		return metrics, fmt.Errorf("http response: %w", err)
	}
	defer resp.Body.Close()
	metrics.HTTPStatus = resp.StatusCode

	n, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, v2HTTPReadLimit+1))
	metrics.Bytes = n
	metrics.DurationMS = time.Since(start).Milliseconds()
	if metrics.DurationMS > 0 {
		metrics.ThroughputBPS = (metrics.Bytes * 1000) / metrics.DurationMS
	}
	metrics.ResponseComplete = errors.Is(readErr, io.EOF) || (readErr == nil && n <= v2HTTPReadLimit)
	metrics.ProgressProven = metrics.ResponseComplete || metrics.Bytes >= v2WorkingProgressBytes
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		metrics.ReadError = readErr.Error()
	}
	if !metrics.ResponseComplete && metrics.Bytes >= v2CutoffLowBytes && metrics.Bytes <= v2CutoffHighBytes {
		metrics.Cutoff16KSuspected = true
	}
	if !metrics.ProgressProven {
		if metrics.ReadError != "" {
			return metrics, errors.New(metrics.ReadError)
		}
		return metrics, errors.New("HTTP response did not prove sufficient progress")
	}
	return metrics, nil
}

func v2ClassifyDetect(stages map[string]v2StageResult, m v2HTTPMetrics) (string, string) {
	if stages["dns"].State == "fail" {
		return "dns_failure", "DNS не смог получить публичный IPv4-адрес."
	}
	if stages["tcp"].State == "fail" {
		return "tcp_failure", "TCP/443 не устанавливается. Причина может быть в маршруте, адресной блокировке или удалённой стороне."
	}
	if stages["tls"].State == "fail" {
		return "tls_failure", "TCP соединяется, но TLS handshake не завершается. Это подходящий случай для подбора TCP/TLS стратегии."
	}
	if m.Cutoff16KSuspected {
		return "partial_16k_suspected", "TLS и HTTP начинаются, но поток обрывается примерно в районе 16 КБ. Нужна отдельная проверка раннего обрыва."
	}
	if stages["http"].State == "fail" {
		return "http_failure", "TLS проходит, но HTTP-проверка не подтверждает рабочий ответ."
	}
	if stages["http"].State == "warn" {
		return "http_restricted", "Сеть и TLS работают, но сервер вернул ограничивающий HTTP-ответ."
	}
	if stages["http"].State == "pass" {
		return "clear", "С роутера DNS, TCP, TLS и HTTP проходят."
	}
	return "inconclusive", "Сигналов недостаточно для однозначного вывода."
}

func handleV2Detect(w http.ResponseWriter, r *http.Request) {
	var req v2DetectRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid detect request"})
		return
	}
	target, err := v2NormalizeTarget(req.Target)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), v2DetectTimeout)
	defer cancel()
	stages := map[string]v2StageResult{
		"dns": {State: "skipped"}, "tcp": {State: "skipped"},
		"tls": {State: "skipped"}, "http": {State: "skipped"},
	}
	ip, d, err := v2ResolvePublicIPv4(ctx, target)
	if err != nil {
		stages["dns"] = v2StageResult{State: "fail", LatencyMS: d.Milliseconds(), Detail: err.Error()}
		class, text := v2ClassifyDetect(stages, v2HTTPMetrics{})
		writeJSON(w, http.StatusOK, v2DetectResponse{OK: false, Target: target, Stages: stages, Classification: class, ClassificationText: text})
		return
	}
	stages["dns"] = v2StageResult{State: "pass", LatencyMS: d.Milliseconds(), Detail: ip}
	td, err := v2DirectTCP(ctx, ip)
	if err != nil {
		stages["tcp"] = v2StageResult{State: "fail", LatencyMS: td.Milliseconds(), Detail: err.Error()}
		class, text := v2ClassifyDetect(stages, v2HTTPMetrics{})
		writeJSON(w, http.StatusOK, v2DetectResponse{OK: false, Target: target, DestinationIPv4: ip, Stages: stages, Classification: class, ClassificationText: text})
		return
	}
	stages["tcp"] = v2StageResult{State: "pass", LatencyMS: td.Milliseconds(), Detail: ip + ":443"}
	tld, err := v2DirectTLS(ctx, target, ip)
	if err != nil {
		stages["tls"] = v2StageResult{State: "fail", LatencyMS: tld.Milliseconds(), Detail: err.Error()}
		class, text := v2ClassifyDetect(stages, v2HTTPMetrics{})
		writeJSON(w, http.StatusOK, v2DetectResponse{OK: false, Target: target, DestinationIPv4: ip, Stages: stages, Classification: class, ClassificationText: text})
		return
	}
	stages["tls"] = v2StageResult{State: "pass", LatencyMS: tld.Milliseconds(), Detail: "TLS handshake complete"}
	metrics, httpErr := v2ReadHTTPS(ctx, target, ip, 0)
	state := "pass"
	detail := "HTTP response received"
	if httpErr != nil {
		state = "fail"
		detail = httpErr.Error()
	} else if metrics.HTTPStatus >= 400 {
		state = "warn"
		detail = fmt.Sprintf("HTTP %d", metrics.HTTPStatus)
	}
	stages["http"] = v2StageResult{State: state, LatencyMS: metrics.DurationMS, Detail: detail, StatusCode: metrics.HTTPStatus, Bytes: metrics.Bytes}
	class, text := v2ClassifyDetect(stages, metrics)
	writeJSON(w, http.StatusOK, v2DetectResponse{
		OK: state != "fail", Target: target, DestinationIPv4: ip, Stages: stages,
		Classification: class, ClassificationText: text,
		ResponseComplete: metrics.ResponseComplete, ProgressProven: metrics.ProgressProven,
		Cutoff16KSuspected: metrics.Cutoff16KSuspected, TTFBMS: metrics.TTFBMS,
		DurationMS: metrics.DurationMS, ThroughputBPS: metrics.ThroughputBPS,
	})
}

type v2BenchOps struct {
	*benchTLSStrategyOps
	productionPID  int
	productionExec string
}

func (o *v2BenchOps) Probe(ctx context.Context, spec benchTransactionSpec) error {
	outComment := benchRuleComment(spec.SessionID, "out-queue")
	inComment := benchRuleComment(spec.SessionID, "in-queue")
	outBefore, err := o.readRulePackets(ctx, outComment)
	if err != nil {
		return fmt.Errorf("read outbound queue before probe: %w", err)
	}
	inBefore, err := o.readRulePackets(ctx, inComment)
	if err != nil {
		return fmt.Errorf("read inbound queue before probe: %w", err)
	}
	metrics, probeErr := v2ReadHTTPS(ctx, o.serverName, spec.DestinationIPv4, spec.LocalPort)
	o.tlsHandshakeComplete = metrics.TLSComplete
	o.probeDurationMS = metrics.DurationMS

	outAfter, outErr := o.readRulePackets(ctx, outComment)
	inAfter, inErr := o.readRulePackets(ctx, inComment)
	if outErr != nil {
		return fmt.Errorf("read outbound queue after probe: %w", outErr)
	}
	if inErr != nil {
		return fmt.Errorf("read inbound queue after probe: %w", inErr)
	}
	if outAfter > outBefore {
		o.outboundQueuePackets = outAfter - outBefore
	}
	if inAfter > inBefore {
		o.inboundQueuePackets = inAfter - inBefore
	}
	o.strategyPathExercised = metrics.TLSComplete && o.outboundQueuePackets > 0 && o.inboundQueuePackets > 0
	v2StoreMetrics(spec.SessionID, metrics)
	if !o.strategyPathExercised {
		return errors.New("candidate NFQUEUE path was not proven")
	}
	if probeErr != nil {
		return probeErr
	}
	return nil
}

var v2MetricsStore sync.Map

func v2StoreMetrics(sessionID string, metrics v2HTTPMetrics) {
	v2MetricsStore.Store(sessionID, metrics)
}

func v2TakeMetrics(sessionID string) v2HTTPMetrics {
	value, ok := v2MetricsStore.LoadAndDelete(sessionID)
	if !ok {
		return v2HTTPMetrics{}
	}
	metrics, _ := value.(v2HTTPMetrics)
	return metrics
}

func (o *v2BenchOps) VerifyCleanup(ctx context.Context, spec benchTransactionSpec) error {
	var problems []string
	if present, err := o.sessionCommentPresent(ctx, spec.SessionID); err != nil {
		problems = append(problems, "firewall inventory: "+err.Error())
	} else if present {
		problems = append(problems, "session firewall rules remain")
	}
	if bound, err := benchQueueIsBound(spec.Queue); err != nil {
		problems = append(problems, "kernel queue inventory: "+err.Error())
	} else if bound {
		problems = append(problems, "reserved queue remains bound")
	}
	if _, err := os.Stat(benchCandidatePIDFile(spec.SessionID)); err == nil {
		problems = append(problems, "candidate pidfile remains")
	} else if !errors.Is(err, os.ErrNotExist) {
		problems = append(problems, "candidate pidfile stat: "+err.Error())
	}
	if current := readStatus().ConfigSHA256; current != o.baselineConfigSHA {
		problems = append(problems, "production config SHA changed")
	}
	pid, executable, _, err := readProductionNFQWSArgv()
	if err != nil {
		problems = append(problems, "production process unavailable: "+err.Error())
	} else if pid != o.productionPID || executable != o.productionExec {
		problems = append(problems, "production process identity changed")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func v2FreeBenchQueues(occupied []int, count int) []int {
	if count < 1 {
		count = 1
	}
	if count > v2MaxConcurrency {
		count = v2MaxConcurrency
	}
	used := map[int]bool{}
	for _, q := range occupied {
		used[q] = true
	}
	out := []int{}
	for q := benchQueueMin; q <= benchQueueMax && len(out) < count; q++ {
		if !used[q] {
			out = append(out, q)
		}
	}
	return out
}

func v2ResultClass(attempt v2BenchAttempt) string {
	if attempt.OK && attempt.Metrics.ResponseComplete {
		return "WORKING"
	}
	if attempt.OK && attempt.Metrics.ProgressProven {
		return "WORKING"
	}
	if attempt.Metrics.Cutoff16KSuspected {
		return "PARTIAL"
	}
	if attempt.CleanupProven && attempt.Metrics.TLSComplete {
		return "INCONCLUSIVE"
	}
	return "FAILED"
}

func v2RunAttempt(ctx context.Context, capabilities benchCapabilities, configSHA, target, ip string, inventory benchStrategyInventory, profile *benchStrategyProfile, queue int) v2BenchAttempt {
	localPort, err := allocateBenchLocalPort()
	if err != nil {
		return v2BenchAttempt{Error: "allocate local port: " + err.Error(), ResultClass: "FAILED"}
	}
	sessionID, err := newBenchSessionID()
	if err != nil {
		return v2BenchAttempt{Error: "create session: " + err.Error(), ResultClass: "FAILED"}
	}
	spec := benchTransactionSpec{SessionID: sessionID, DestinationIPv4: ip, LocalPort: localPort, Queue: queue}
	var candidateArgs []string
	if profile == nil {
		candidateArgs = benchCandidateArgs(spec)
	} else {
		candidateArgs, err = buildBenchStrategyCandidateArgs(spec, inventory, *profile)
		if err != nil {
			return v2BenchAttempt{SessionID: sessionID, DestinationIPv4: ip, LocalPort: localPort, Queue: queue, Error: err.Error(), ResultClass: "FAILED"}
		}
	}
	pid, executable, _, prodErr := readProductionNFQWSArgv()
	if prodErr != nil {
		return v2BenchAttempt{SessionID: sessionID, DestinationIPv4: ip, LocalPort: localPort, Queue: queue, Error: "production process: " + prodErr.Error(), ResultClass: "FAILED"}
	}
	baselineProcesses, _ := readBenchProcesses()
	systemOps := &benchSystemOps{
		iptablesPath: capabilities.IPTablesPath, iptablesSavePath: capabilities.IPTablesSavePath,
		candidateBinary: capabilities.CandidateBinary, baselineConfigSHA: configSHA,
		baselineProcesses: baselineProcesses,
	}
	tlsOps := &benchTLSStrategyOps{benchSystemOps: systemOps, candidateArgs: candidateArgs, serverName: target}
	ops := &v2BenchOps{benchTLSStrategyOps: tlsOps, productionPID: pid, productionExec: executable}
	result := runBenchTransaction(ctx, ops, spec)
	metrics := v2TakeMetrics(sessionID)
	afterConfig := readStatus().ConfigSHA256
	cleanup := result.CleanupProven && afterConfig == configSHA
	ok := result.Error == "" && result.State == benchLifecycleStateClean && cleanup &&
		metrics.TLSComplete && metrics.ProgressProven &&
		ops.outboundQueuePackets > 0 && ops.inboundQueuePackets > 0
	attempt := v2BenchAttempt{
		OK: ok, SessionID: sessionID, DestinationIPv4: ip, LocalPort: localPort, Queue: queue,
		OutboundQueuePackets: ops.outboundQueuePackets, InboundQueuePackets: ops.inboundQueuePackets,
		CleanupProven: cleanup, Metrics: metrics,
	}
	if result.Error != "" {
		attempt.Error = result.Error
	}
	attempt.ResultClass = v2ResultClass(attempt)
	return attempt
}

func v2MedianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]int64{}, values...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func v2FinalizeCandidate(c *v2CandidateResult) {
	c.Successes, c.Failures = 0, 0
	c.CleanupProven = true
	ttfb, duration, throughput := []int64{}, []int64{}, []int64{}
	complete := 0
	partial := false
	for _, a := range c.Attempts {
		if a.OK {
			c.Successes++
		} else {
			c.Failures++
		}
		if !a.CleanupProven {
			c.CleanupProven = false
		}
		if a.Metrics.ResponseComplete {
			complete++
		}
		if a.Metrics.Cutoff16KSuspected {
			partial = true
		}
		if a.OK && a.Metrics.TTFBMS > 0 {
			ttfb = append(ttfb, a.Metrics.TTFBMS)
		}
		if a.OK && a.Metrics.DurationMS > 0 {
			duration = append(duration, a.Metrics.DurationMS)
		}
		if a.OK && a.Metrics.ThroughputBPS > 0 {
			throughput = append(throughput, a.Metrics.ThroughputBPS)
		}
	}
	if len(c.Attempts) > 0 {
		c.SuccessRate = float64(c.Successes) / float64(len(c.Attempts))
		c.CompleteRate = float64(complete) / float64(len(c.Attempts))
	}
	c.MedianTTFBMS = v2MedianInt64(ttfb)
	c.MedianDurationMS = v2MedianInt64(duration)
	c.MedianThroughput = v2MedianInt64(throughput)
	switch {
	case c.SuccessRate == 1 && c.CleanupProven:
		c.ResultClass = "WORKING"
	case partial:
		c.ResultClass = "PARTIAL"
	case c.Successes > 0:
		c.ResultClass = "UNSTABLE"
	case c.CleanupProven:
		c.ResultClass = "FAILED"
	default:
		c.ResultClass = "INCONCLUSIVE"
	}
}

func v2CandidateBetter(a, b v2CandidateResult) bool {
	if a.SuccessRate != b.SuccessRate {
		return a.SuccessRate > b.SuccessRate
	}
	if a.CompleteRate != b.CompleteRate {
		return a.CompleteRate > b.CompleteRate
	}
	if a.CleanupProven != b.CleanupProven {
		return a.CleanupProven
	}
	if a.MedianTTFBMS > 0 && b.MedianTTFBMS > 0 && a.MedianTTFBMS != b.MedianTTFBMS {
		return a.MedianTTFBMS < b.MedianTTFBMS
	}
	if a.MedianThroughput != b.MedianThroughput {
		return a.MedianThroughput > b.MedianThroughput
	}
	return a.SourceProfileIndex < b.SourceProfileIndex
}

func v2CustomProfile(args []string, target string) (benchStrategyProfile, error) {
	if len(args) == 0 || len(args) > 256 {
		return benchStrategyProfile{}, errors.New("custom candidate args are empty or too large")
	}
	for _, arg := range args {
		switch {
		case arg == "--new",
			strings.HasPrefix(arg, "--daemon"),
			strings.HasPrefix(arg, "--pidfile"),
			strings.HasPrefix(arg, "--user"),
			strings.HasPrefix(arg, "--qnum"),
			strings.HasPrefix(arg, "--fwmark"):
			return benchStrategyProfile{}, errors.New("custom candidate contains reserved runtime argument: " + arg)
		}
	}
	profile := analyzeBenchStrategyProfile(0, args)
	if !profile.CandidateEligible {
		return benchStrategyProfile{}, errors.New("custom candidate is not eligible: " + strings.Join(profile.Reasons, "; "))
	}
	return retargetBenchStrategyProfile(profile, target)
}

func validateV2BenchRequest(req v2BenchRequest) error {
	if _, err := v2NormalizeTarget(req.Target); err != nil {
		return err
	}
	if req.Confirm != v2BenchConfirm {
		return errors.New("confirm must equal " + v2BenchConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(req.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	if req.ProfileIndex == nil && len(req.Args) == 0 {
		return errors.New("profile_index or args is required")
	}
	if req.ProfileIndex != nil && len(req.Args) > 0 {
		return errors.New("profile_index and args are mutually exclusive")
	}
	return nil
}

func handleV2Bench(w http.ResponseWriter, r *http.Request) {
	var req v2BenchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid v2 bench request"})
		return
	}
	if err := validateV2BenchRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)
	target, _ := v2NormalizeTarget(req.Target)
	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed before v2 bench", "current_sha256": status.ConfigSHA256})
		return
	}
	capabilities := readBenchCapabilities()
	if !benchExecutionReady(capabilities) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "v2 bench capability gates are not proven"})
		return
	}
	inventory := readBenchStrategyInventory()
	var profile benchStrategyProfile
	var err error
	source := "production"
	if req.ProfileIndex != nil {
		profile, err = findBenchStrategyProfile(inventory, *req.ProfileIndex)
		if err == nil {
			profile, err = retargetBenchStrategyProfile(profile, target)
		}
	} else {
		source = "custom"
		profile, err = v2CustomProfile(req.Args, target)
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 4*time.Second)
	ip, err := resolveBenchServerIPv4(resolveCtx, target)
	cancelResolve()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve target: " + err.Error()})
		return
	}
	attempts := req.Attempts
	if attempts <= 0 {
		attempts = 1
	}
	if attempts > v2MaxAttempts {
		attempts = v2MaxAttempts
	}
	queue := capabilities.RecommendedQueue
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(attempts)*v2HTTPProbeTimeout+10*time.Second)
	defer cancel()
	result := v2CandidateResult{
		SourceProfileIndex: profile.Index, StrategyTags: append([]int{}, profile.StrategyTags...),
		Args: append([]string{}, profile.Args...), CleanupProven: true,
	}
	for i := 0; i < attempts; i++ {
		a := v2RunAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, &profile, queue)
		result.Attempts = append(result.Attempts, a)
		if !a.CleanupProven {
			break
		}
	}
	v2FinalizeCandidate(&result)
	after := readBenchCapabilities()
	ok := result.CleanupProven && after.CleanupBaselineProven
	resp := v2BenchResponse{OK: ok, Target: target, ConfigSHA256: status.ConfigSHA256, Source: source, ProfileIndex: profile.Index, Result: result, CleanupBaselineAfter: after.CleanupBaselineProven}
	if !ok {
		writeJSON(w, http.StatusBadGateway, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func v2SelectorMode(raw string) (benchAutoTuneMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fast":
		return benchAutoTuneMode{Name: "fast", Attempts: 1, MaxCandidates: 8, TimeoutSec: 90}, nil
	case "normal":
		return benchAutoTuneMode{Name: "normal", Attempts: 2, MaxCandidates: 16, TimeoutSec: 180}, nil
	case "thorough":
		return benchAutoTuneMode{Name: "thorough", Attempts: 3, MaxCandidates: 32, TimeoutSec: 300}, nil
	default:
		return benchAutoTuneMode{}, errors.New("mode must be fast, normal or thorough")
	}
}

func v2DefaultConcurrency() int {
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "amd64" {
		return 2
	}
	return 1
}

func validateV2SelectorRequest(req v2SelectorRequest) error {
	if _, err := v2SelectorMode(req.Mode); err != nil {
		return err
	}
	if _, err := v2NormalizeTarget(req.ServerName); err != nil {
		return err
	}
	if req.Confirm != v2SelectorConfirm {
		return errors.New("confirm must equal " + v2SelectorConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(req.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	return nil
}

func v2ChooseRecommendation(baseline v2CandidateResult, candidates []v2CandidateResult) (bool, int, bool, string) {
	if baseline.ResultClass == "WORKING" && baseline.SuccessRate == 1 {
		return false, 0, false, "baseline completed reliably; bypass strategy is not required for this target"
	}
	var best *v2CandidateResult
	for i := range candidates {
		c := &candidates[i]
		if !c.CleanupProven || c.Successes == 0 || c.ResultClass == "PARTIAL" {
			continue
		}
		if best == nil || v2CandidateBetter(*c, *best) {
			best = c
		}
	}
	if best == nil {
		return false, 0, false, "no candidate proved a working HTTP response through isolated NFQUEUE"
	}
	if best.SuccessRate <= baseline.SuccessRate && best.CompleteRate <= baseline.CompleteRate {
		return false, 0, false, "best candidate did not improve reliability/completeness over baseline"
	}
	return true, best.SourceProfileIndex, true, "candidate improved verified HTTP reachability over baseline"
}

func handleV2Selector(w http.ResponseWriter, r *http.Request) {
	var req v2SelectorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid v2 selector request"})
		return
	}
	if err := validateV2SelectorRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)
	mode, _ := v2SelectorMode(req.Mode)
	target, _ := v2NormalizeTarget(req.ServerName)
	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed before selector", "current_sha256": status.ConfigSHA256})
		return
	}
	capabilities := readBenchCapabilities()
	if !benchExecutionReady(capabilities) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "selector capability gates are not proven"})
		return
	}
	inventory := readBenchStrategyInventory()
	if !inventory.BaseDependenciesProven {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy base dependencies are not proven"})
		return
	}
	templates := []benchStrategyProfile{}
	for _, p := range inventory.Profiles {
		if !p.CandidateEligible {
			continue
		}
		retargeted, err := retargetBenchStrategyProfile(p, target)
		if err == nil {
			templates = append(templates, retargeted)
		}
		if len(templates) >= mode.MaxCandidates {
			break
		}
	}
	if len(templates) == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no retargetable production strategy candidates"})
		return
	}
	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 4*time.Second)
	ip, err := resolveBenchServerIPv4(resolveCtx, target)
	cancelResolve()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve target: " + err.Error()})
		return
	}
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = v2DefaultConcurrency()
	}
	if concurrency > v2MaxConcurrency {
		concurrency = v2MaxConcurrency
	}
	if concurrency > len(templates) {
		concurrency = len(templates)
	}
	queues := v2FreeBenchQueues(capabilities.OccupiedQueues, concurrency)
	if len(queues) < concurrency {
		concurrency = len(queues)
	}
	if concurrency < 1 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no free reserved NFQUEUE for selector"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(mode.TimeoutSec)*time.Second)
	defer cancel()

	baseline := v2CandidateResult{Baseline: true, CleanupProven: true}
	for i := 0; i < mode.Attempts; i++ {
		a := v2RunAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, nil, queues[0])
		baseline.Attempts = append(baseline.Attempts, a)
		if !a.CleanupProven {
			break
		}
	}
	v2FinalizeCandidate(&baseline)
	if !baseline.CleanupProven {
		writeJSON(w, http.StatusBadGateway, v2SelectorResponse{
			OK: false, Mode: mode, ServerName: target, DestinationIPv4: ip, MetricScope: "https-full-response",
			Baseline: baseline, RecommendationReason: "baseline cleanup proof failed; selector stopped fail-closed",
			CleanupBaselineAfter: false, Concurrency: concurrency, CandidateSource: "live-production-proc-cmdline",
		})
		return
	}

	type job struct {
		index   int
		profile benchStrategyProfile
	}
	type jobResult struct {
		index  int
		result v2CandidateResult
	}
	jobs := make(chan job)
	results := make(chan jobResult, len(templates))
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		queue := queues[worker]
		wg.Add(1)
		go func(q int) {
			defer wg.Done()
			for j := range jobs {
				c := v2CandidateResult{
					SourceProfileIndex: j.profile.Index,
					StrategyTags:       append([]int{}, j.profile.StrategyTags...),
					Args:               append([]string{}, j.profile.Args...), CleanupProven: true,
				}
				for i := 0; i < mode.Attempts; i++ {
					a := v2RunAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, &j.profile, q)
					c.Attempts = append(c.Attempts, a)
					if !a.CleanupProven {
						break
					}
				}
				v2FinalizeCandidate(&c)
				results <- jobResult{index: j.index, result: c}
			}
		}(queue)
	}
	go func() {
		for i, profile := range templates {
			jobs <- job{index: i, profile: profile}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	candidates := make([]v2CandidateResult, len(templates))
	for jr := range results {
		candidates[jr.index] = jr.result
	}
	for _, c := range candidates {
		if !c.CleanupProven {
			writeJSON(w, http.StatusBadGateway, v2SelectorResponse{
				OK: false, Mode: mode, ServerName: target, DestinationIPv4: ip, MetricScope: "https-full-response",
				Baseline: baseline, Candidates: candidates,
				RecommendationReason: "candidate cleanup proof failed; selector stopped fail-closed",
				CleanupBaselineAfter: false, Concurrency: concurrency, CandidateSource: "live-production-proc-cmdline",
			})
			return
		}
	}
	recommend, profileIndex, needed, reason := v2ChooseRecommendation(baseline, candidates)
	after := readBenchCapabilities()
	ok := after.CleanupBaselineProven && strings.EqualFold(readStatus().ConfigSHA256, status.ConfigSHA256)

	applyEligible := false
	token, expires, applyReason := "", "", "no selector recommendation is available"
	clearBenchAutoTuneApplyPlan()
	if ok && recommend && needed {
		for _, p := range templates {
			if p.Index != profileIndex {
				continue
			}
			plan, planErr := storeBenchAutoTuneApplyPlan(status.ConfigSHA256, target, ip, p)
			if planErr != nil {
				ok = false
				applyReason = "create apply gate: " + planErr.Error()
				break
			}
			applyEligible = true
			token = plan.Token
			expires = plan.ExpiresAt.Format(time.RFC3339)
			applyReason = "verified selector recommendation is eligible for deterministic preview"
			break
		}
	}
	resp := v2SelectorResponse{
		OK: ok, Mode: mode, ServerName: target, DestinationIPv4: ip, MetricScope: "https-full-response",
		Baseline: baseline, Candidates: candidates,
		RecommendationAvailable: recommend, RecommendedProfileIndex: profileIndex, StrategyNeeded: needed,
		RecommendationReason: reason, CleanupBaselineAfter: after.CleanupBaselineProven,
		BenchEnabled: after.BenchEnabled, SafeToBench: after.SafeToBench, ApplyEnabled: false,
		ApplyGateEligible: applyEligible, ApplyGateToken: token, ApplyGateExpiresAt: expires,
		ApplyGateReason: applyReason, Concurrency: concurrency, CandidateSource: "live-production-proc-cmdline",
	}
	if !ok {
		writeJSON(w, http.StatusBadGateway, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
