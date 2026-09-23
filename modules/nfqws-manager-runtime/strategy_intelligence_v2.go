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
	Diagnostic         v2DiagnosticVerdict      `json:"diagnostic"`
}

type v2HTTPMetrics struct {
	TLSComplete            bool   `json:"tls_complete"`
	HTTPStatus             int    `json:"http_status,omitempty"`
	TTFBMS                 int64  `json:"ttfb_ms,omitempty"`
	DurationMS             int64  `json:"duration_ms,omitempty"`
	Bytes                  int64  `json:"bytes"`
	ThroughputBPS          int64  `json:"throughput_bps,omitempty"`
	ResponseComplete       bool   `json:"response_complete"`
	ProgressProven         bool   `json:"progress_proven"`
	Cutoff16KSuspected     bool   `json:"cutoff_16k_suspected"`
	ReadError              string `json:"read_error,omitempty"`
	UDPWriteBytes          int64  `json:"udp_write_bytes,omitempty"`
	UDPReadBytes           int64  `json:"udp_read_bytes,omitempty"`
	UDPReadDatagrams       int    `json:"udp_read_datagrams,omitempty"`
	QUICLongHeader         bool   `json:"quic_long_header,omitempty"`
	QUICCIDMatched         bool   `json:"quic_cid_matched,omitempty"`
	QUICVersion            string `json:"quic_version,omitempty"`
	QUICResponseType       string `json:"quic_response_type,omitempty"`
	QUICResponseProven     bool   `json:"quic_response_proven,omitempty"`
	STUNTransactionMatched bool   `json:"stun_transaction_matched,omitempty"`
	STUNMessageType        string `json:"stun_message_type,omitempty"`
	STUNMappedAddress      string `json:"stun_mapped_address,omitempty"`
	STUNMappedPort         int    `json:"stun_mapped_port,omitempty"`
	STUNResponseProven     bool   `json:"stun_response_proven,omitempty"`
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
	Transport            string   `json:"transport,omitempty"`
	ProfileIndex         *int     `json:"profile_index,omitempty"`
	Args                 []string `json:"args,omitempty"`
	Attempts             int      `json:"attempts,omitempty"`
	ExpectedConfigSHA256 string   `json:"expected_config_sha256"`
	Confirm              string   `json:"confirm"`
}

type v2BenchAttempt struct {
	OK                    bool          `json:"ok"`
	InfrastructureOK      bool          `json:"infrastructure_ok"`
	StrategyPathExercised bool          `json:"strategy_path_exercised"`
	ResultClass           string        `json:"result_class"`
	SessionID             string        `json:"session_id"`
	DestinationIPv4       string        `json:"destination_ipv4"`
	Transport             string        `json:"transport"`
	Network               string        `json:"network"`
	RemotePort            int           `json:"remote_port"`
	LocalPort             int           `json:"local_port"`
	Queue                 int           `json:"queue"`
	OutboundQueuePackets  uint64        `json:"outbound_queue_packets"`
	InboundQueuePackets   uint64        `json:"inbound_queue_packets"`
	CleanupProven         bool          `json:"cleanup_proven"`
	Metrics               v2HTTPMetrics `json:"metrics"`
	Error                 string        `json:"error,omitempty"`
}

type v2CandidateResult struct {
	Baseline           bool             `json:"baseline"`
	CandidateID        string           `json:"candidate_id,omitempty"`
	CandidateName      string           `json:"candidate_name,omitempty"`
	CandidateSource    string           `json:"candidate_source,omitempty"`
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
	InfrastructureOK   bool             `json:"infrastructure_ok"`
}

type v2BenchResponse struct {
	OK                   bool              `json:"ok"`
	Target               string            `json:"target"`
	Transport            string            `json:"transport"`
	Network              string            `json:"network"`
	RemotePort           int               `json:"remote_port"`
	MetricScope          string            `json:"metric_scope"`
	ConfigSHA256         string            `json:"config_sha256"`
	Source               string            `json:"source"`
	ProfileIndex         int               `json:"profile_index,omitempty"`
	Result               v2CandidateResult `json:"result"`
	CleanupBaselineAfter bool              `json:"cleanup_baseline_after"`
}

type v2SelectorCandidateInput struct {
	ID     string   `json:"id,omitempty"`
	Name   string   `json:"name,omitempty"`
	Source string   `json:"source,omitempty"`
	Args   []string `json:"args"`
}

type v2SelectorRequest struct {
	Mode                       string                     `json:"mode"`
	ServerName                 string                     `json:"server_name"`
	ExpectedConfigSHA256       string                     `json:"expected_config_sha256"`
	Concurrency                int                        `json:"concurrency,omitempty"`
	SessionID                  string                     `json:"session_id,omitempty"`
	IncludeProduction          *bool                      `json:"include_production,omitempty"`
	AutoPool                   *bool                      `json:"auto_pool,omitempty"`
	Candidates                 []v2SelectorCandidateInput `json:"candidates,omitempty"`
	DiagnosticCode             string                     `json:"diagnostic_code,omitempty"`
	DiagnosticFaultDomain      string                     `json:"diagnostic_fault_domain,omitempty"`
	DiagnosticStrategyRelevant bool                       `json:"diagnostic_strategy_relevant,omitempty"`
	PropertyVector             *v2DPIPropertyVector       `json:"property_vector,omitempty"`
	Confirm                    string                     `json:"confirm"`
}

type v2SelectorResponse struct {
	OK                         bool                   `json:"ok"`
	SessionID                  string                 `json:"session_id,omitempty"`
	Mode                       benchAutoTuneMode      `json:"mode"`
	ServerName                 string                 `json:"server_name"`
	DestinationIPv4            string                 `json:"destination_ipv4"`
	MetricScope                string                 `json:"metric_scope"`
	Baseline                   v2CandidateResult      `json:"baseline"`
	Candidates                 []v2CandidateResult    `json:"candidates"`
	RecommendationAvailable    bool                   `json:"recommendation_available"`
	RecommendedProfileIndex    int                    `json:"recommended_profile_index"`
	RecommendedCandidateID     string                 `json:"recommended_candidate_id,omitempty"`
	RecommendedCandidateName   string                 `json:"recommended_candidate_name,omitempty"`
	RecommendedCandidateSource string                 `json:"recommended_candidate_source,omitempty"`
	StrategyNeeded             bool                   `json:"strategy_needed"`
	RecommendationReason       string                 `json:"recommendation_reason"`
	CleanupBaselineAfter       bool                   `json:"cleanup_baseline_after"`
	BenchEnabled               bool                   `json:"bench_enabled"`
	SafeToBench                bool                   `json:"safe_to_bench"`
	ApplyEnabled               bool                   `json:"apply_enabled"`
	ApplyGateEligible          bool                   `json:"apply_gate_eligible"`
	ApplyGateToken             string                 `json:"apply_gate_token,omitempty"`
	ApplyGateExpiresAt         string                 `json:"apply_gate_expires_at,omitempty"`
	ApplyGateReason            string                 `json:"apply_gate_reason"`
	Concurrency                int                    `json:"concurrency"`
	CandidateSource            string                 `json:"candidate_source"`
	AutoPoolEnabled            bool                   `json:"auto_pool_enabled"`
	AutoPoolAdded              int                    `json:"auto_pool_added"`
	PoolSources                []string               `json:"pool_sources"`
	PoolWarnings               []string               `json:"pool_warnings"`
	HistoricalPlanning         bool                   `json:"historical_planning"`
	HistoricalHints            int                    `json:"historical_hints"`
	HistoricalPromoted         int                    `json:"historical_promoted"`
	PlannerVersion             int                    `json:"planner_version"`
	PlannerDiagnosticCode      string                 `json:"planner_diagnostic_code,omitempty"`
	PlannerFaultDomain         string                 `json:"planner_fault_domain,omitempty"`
	PlannerStrategyRelevant    bool                   `json:"planner_strategy_relevant"`
	PlannerCompatibleCount     int                    `json:"planner_compatible_count"`
	PlannerPromotedCount       int                    `json:"planner_promoted_count"`
	PlannerAdmittedRegistry    int                    `json:"planner_admitted_registry_count"`
	PlannerPlan                []v2CandidatePoolItem  `json:"planner_plan,omitempty"`
	PropertyVector             *v2DPIPropertyVector   `json:"property_vector,omitempty"`

	MemoryUpdated bool   `json:"memory_updated"`
	MemoryWarning string `json:"memory_warning,omitempty"`
}

func registerStrategyIntelligenceV2Routes(mux *http.ServeMux) {
	registerStrategyLibraryV2Routes(mux)
	registerCandidatePoolV2Routes(mux)
	registerTargetMemoryV2Routes(mux)
	registerObservedTargetsV1Routes(mux)
	registerStrategyRegistryV1Routes(mux)
	registerTCP16NetworkMemoryV1Routes(mux)
	registerSelectorProgressV2Route(mux)
	registerDPIPropertyProbeV1Route(mux)
	registerProgressiveSelectorV2Route(mux)
	registerBenchTransportV2Routes(mux)
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

func v2InspectProfileMatchReasons(
	target string,
	profile benchStrategyProfile,
	domainMatches map[string]bool,
	ipMatches map[string]bool,
) []string {
	if !v2ProfileMatchesTargetWithLists(target, profile, domainMatches, ipMatches) {
		return nil
	}
	reasons := []string{}
	for _, domain := range profile.HostlistDomains {
		if v2DomainMatches(target, domain) {
			reasons = append(reasons, "hostlist-domains="+domain)
		}
	}
	for _, arg := range profile.FileBoundFilters {
		name, relation := v2ProfileFileRelation(arg)
		if name == "" {
			continue
		}
		switch relation {
		case "include", "include-auto":
			if domainMatches[name] {
				reasons = append(reasons, relation+" "+name)
			}
		case "ipset":
			if ipMatches[name] {
				reasons = append(reasons, relation+" "+name)
			}
		}
	}
	if len(reasons) > 0 {
		return reasons
	}
	if len(profile.HostlistDomains) == 0 && len(profile.FileBoundFilters) == 0 {
		return []string{"profile has no target selector"}
	}
	return []string{"target is not excluded by profile filters"}
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

	destinationIPv4 := ""
	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 4*time.Second)
	if resolved, resolveErr := resolveBenchServerIPv4(resolveCtx, target); resolveErr == nil {
		destinationIPv4 = resolved
	}
	cancelResolve()

	status := readStatus()
	inventory := readBenchStrategyInventory()
	listMatches := []v2ListMatch{}
	domainMatches := map[string]bool{}
	ipMatches := map[string]bool{}
	for _, item := range status.Lists {
		data, path, readErr := v2ReadListByName(item.Name)
		if readErr != nil {
			continue
		}
		text := string(data)
		domainMatch := v2ListContainsTarget(text, target)
		ipMatch := destinationIPv4 != "" && v2ListContainsIPv4(text, destinationIPv4)
		if domainMatch {
			domainMatches[item.Name] = true
		}
		if ipMatch {
			ipMatches[item.Name] = true
		}
		if domainMatch || ipMatch {
			listMatches = append(listMatches, v2ListMatch{Name: item.Name, Path: path, Relation: "present"})
		}
	}

	profiles := []v2InspectorProfile{}
	for _, profile := range inventory.Profiles {
		if !v2ProductionSourceProfileEligible(profile) {
			continue
		}
		reasons := v2InspectProfileMatchReasons(target, profile, domainMatches, ipMatches)
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
		diagnostic := v2ClassifyDiagnostic(stages, v2HTTPMetrics{})
		writeJSON(w, http.StatusOK, v2DetectResponse{OK: false, Target: target, Stages: stages, Classification: class, ClassificationText: text, Diagnostic: diagnostic})
		return
	}
	stages["dns"] = v2StageResult{State: "pass", LatencyMS: d.Milliseconds(), Detail: ip}
	td, err := v2DirectTCP(ctx, ip)
	if err != nil {
		stages["tcp"] = v2StageResult{State: "fail", LatencyMS: td.Milliseconds(), Detail: err.Error()}
		class, text := v2ClassifyDetect(stages, v2HTTPMetrics{})
		diagnostic := v2ClassifyDiagnostic(stages, v2HTTPMetrics{})
		writeJSON(w, http.StatusOK, v2DetectResponse{OK: false, Target: target, DestinationIPv4: ip, Stages: stages, Classification: class, ClassificationText: text, Diagnostic: diagnostic})
		return
	}
	stages["tcp"] = v2StageResult{State: "pass", LatencyMS: td.Milliseconds(), Detail: ip + ":443"}
	tld, err := v2DirectTLS(ctx, target, ip)
	if err != nil {
		stages["tls"] = v2StageResult{State: "fail", LatencyMS: tld.Milliseconds(), Detail: err.Error()}
		class, text := v2ClassifyDetect(stages, v2HTTPMetrics{})
		diagnostic := v2ClassifyDiagnostic(stages, v2HTTPMetrics{})
		writeJSON(w, http.StatusOK, v2DetectResponse{OK: false, Target: target, DestinationIPv4: ip, Stages: stages, Classification: class, ClassificationText: text, Diagnostic: diagnostic})
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
	diagnostic := v2ClassifyDiagnostic(stages, metrics)
	_ = v2RecordTCP16Observation(target, ip, metrics)
	writeJSON(w, http.StatusOK, v2DetectResponse{
		OK: state != "fail", Target: target, DestinationIPv4: ip, Stages: stages,
		Classification: class, ClassificationText: text,
		ResponseComplete: metrics.ResponseComplete, ProgressProven: metrics.ProgressProven,
		Cutoff16KSuspected: metrics.Cutoff16KSuspected, TTFBMS: metrics.TTFBMS,
		DurationMS: metrics.DurationMS, ThroughputBPS: metrics.ThroughputBPS,
		Diagnostic: diagnostic,
	})
}

type v2BenchOps struct {
	*benchTLSStrategyOps
	productionPID  int
	productionExec string
	transport      benchTransportProfile
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
	metrics, probeErr := v2ProbeTransport(ctx, o.transport, o.serverName, spec.DestinationIPv4, spec.LocalPort)
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
	// Infrastructure proof and candidate success are deliberately separate.
	// An outbound queue counter increase proves that the isolated candidate path
	// actually saw the probe even when the candidate subsequently fails TLS/HTTP.
	o.strategyPathExercised = o.outboundQueuePackets > 0
	v2StoreMetrics(spec.SessionID, metrics)
	if !o.strategyPathExercised {
		return errors.New("candidate outbound NFQUEUE path was not proven")
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
	transport, err := normalizeBenchTransport(attempt.Transport)
	if err != nil {
		transport, _ = normalizeBenchTransport("")
	}
	return v2TransportResultClass(attempt, transport)
}

func v2RunAttempt(ctx context.Context, capabilities benchCapabilities, configSHA, target, ip string, inventory benchStrategyInventory, profile *benchStrategyProfile, queue int) v2BenchAttempt {
	transport, _ := normalizeBenchTransport("")
	return v2RunTransportAttempt(ctx, capabilities, configSHA, target, ip, inventory, profile, queue, transport)
}

func v2RunTransportAttempt(ctx context.Context, capabilities benchCapabilities, configSHA, target, ip string, inventory benchStrategyInventory, profile *benchStrategyProfile, queue int, transport benchTransportProfile) v2BenchAttempt {
	localPort, err := allocateBenchLocalPortForNetwork(transport.Network)
	if err != nil {
		return v2BenchAttempt{Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, Error: "allocate local port: " + err.Error(), ResultClass: "FAILED"}
	}
	sessionID, err := newBenchSessionID()
	if err != nil {
		return v2BenchAttempt{Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, Error: "create session: " + err.Error(), ResultClass: "FAILED"}
	}
	spec := benchTransactionSpec{
		SessionID: sessionID, DestinationIPv4: ip, LocalPort: localPort, Queue: queue,
		Network: transport.Network, RemotePort: transport.RemotePort,
	}
	var candidateArgs []string
	if profile == nil {
		candidateArgs = benchCandidateArgs(spec)
	} else {
		candidateArgs, err = buildBenchStrategyCandidateArgsForTransport(spec, inventory, *profile, transport)
		if err != nil {
			return v2BenchAttempt{
				SessionID: sessionID, DestinationIPv4: ip,
				Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort,
				LocalPort: localPort, Queue: queue, Error: err.Error(), ResultClass: "FAILED",
			}
		}
	}
	pid, executable, _, prodErr := readProductionNFQWSArgv()
	if prodErr != nil {
		return v2BenchAttempt{
			SessionID: sessionID, DestinationIPv4: ip,
			Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort,
			LocalPort: localPort, Queue: queue, Error: "production process: " + prodErr.Error(), ResultClass: "FAILED",
		}
	}
	baselineProcesses, _ := readBenchProcesses()
	systemOps := &benchSystemOps{
		iptablesPath: capabilities.IPTablesPath, iptablesSavePath: capabilities.IPTablesSavePath,
		candidateBinary: capabilities.CandidateBinary, baselineConfigSHA: configSHA,
		baselineProcesses: baselineProcesses,
	}
	tlsOps := &benchTLSStrategyOps{benchSystemOps: systemOps, candidateArgs: candidateArgs, serverName: target}
	ops := &v2BenchOps{
		benchTLSStrategyOps: tlsOps, productionPID: pid, productionExec: executable,
		transport: transport,
	}
	result := runBenchTransaction(ctx, ops, spec)
	metrics := v2TakeMetrics(sessionID)
	afterConfig := readStatus().ConfigSHA256
	cleanup := result.CleanupProven && afterConfig == configSHA
	infrastructureOK := cleanup && ops.strategyPathExercised
	ok := result.Error == "" && result.State == benchLifecycleStateClean && infrastructureOK &&
		v2TransportAttemptSucceeded(transport, metrics)
	attempt := v2BenchAttempt{
		OK: ok, InfrastructureOK: infrastructureOK, StrategyPathExercised: ops.strategyPathExercised,
		SessionID: sessionID, DestinationIPv4: ip,
		Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort,
		LocalPort: localPort, Queue: queue,
		OutboundQueuePackets: ops.outboundQueuePackets, InboundQueuePackets: ops.inboundQueuePackets,
		CleanupProven: cleanup, Metrics: metrics,
	}
	if result.Error != "" {
		attempt.Error = result.Error
	}
	attempt.ResultClass = v2TransportResultClass(attempt, transport)
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
	c.CleanupProven = len(c.Attempts) > 0
	c.InfrastructureOK = len(c.Attempts) > 0
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
		if !a.InfrastructureOK {
			c.InfrastructureOK = false
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
	case !c.InfrastructureOK:
		c.ResultClass = "INCONCLUSIVE"
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
	// Imported and catalog candidates can carry hostlist/ipset selectors.
	// The isolated bench already narrows traffic to one exact destination + local
	// source port, so these selection-only file filters are compiled away rather
	// than copied into the temporary candidate. This keeps the strategy technique
	// while avoiding hidden dependence on production list files.
	compiled := make([]string, 0, len(args)+1)
	hasTargetDomain := false
	for _, arg := range args {
		switch {
		case arg == "--new",
			strings.HasPrefix(arg, "--daemon"),
			strings.HasPrefix(arg, "--pidfile"),
			strings.HasPrefix(arg, "--user"),
			strings.HasPrefix(arg, "--qnum"),
			strings.HasPrefix(arg, "--fwmark"):
			return benchStrategyProfile{}, errors.New("custom candidate contains reserved runtime argument: " + arg)
		case strings.HasPrefix(arg, "--hostlist="),
			strings.HasPrefix(arg, "--hostlist-auto="),
			strings.HasPrefix(arg, "--hostlist-exclude="),
			strings.HasPrefix(arg, "--ipset="),
			strings.HasPrefix(arg, "--ipset-exclude="):
			continue
		case strings.HasPrefix(arg, "--hostlist-domains="):
			hasTargetDomain = true
		}
		compiled = append(compiled, arg)
	}
	if !hasTargetDomain {
		compiled = append([]string{"--hostlist-domains=" + target}, compiled...)
	}
	profile := analyzeBenchStrategyProfile(0, compiled)
	if !profile.CandidateEligible {
		return benchStrategyProfile{}, errors.New("custom candidate is not eligible: " + strings.Join(profile.Reasons, "; "))
	}
	return retargetBenchStrategyProfile(profile, target)
}

func validateV2BenchRequest(req v2BenchRequest) error {
	if _, err := v2NormalizeTarget(req.Target); err != nil {
		return err
	}
	if _, err := normalizeBenchTransport(req.Transport); err != nil {
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
	transport, _ := normalizeBenchTransport(req.Transport)
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
		profile, err = findBenchStrategyProfileForTransport(inventory, *req.ProfileIndex, transport)
		if err == nil {
			profile, err = retargetBenchStrategyProfileForTransport(profile, target, transport)
		}
	} else {
		source = "custom"
		profile, err = v2CustomProfileForTransport(req.Args, target, transport)
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
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(attempts)*benchTransportTimeout(transport)+10*time.Second)
	defer cancel()
	result := v2CandidateResult{
		SourceProfileIndex: profile.Index, StrategyTags: append([]int{}, profile.StrategyTags...),
		Args: append([]string{}, profile.Args...), CleanupProven: true,
	}
	for i := 0; i < attempts; i++ {
		a := v2RunTransportAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, &profile, queue, transport)
		result.Attempts = append(result.Attempts, a)
		if !a.CleanupProven {
			break
		}
	}
	v2FinalizeCandidate(&result)
	after := readBenchCapabilities()
	ok := result.CleanupProven && result.InfrastructureOK && after.CleanupBaselineProven
	resp := v2BenchResponse{
		OK: ok, Target: target,
		Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, MetricScope: transport.MetricScope,
		ConfigSHA256: status.ConfigSHA256, Source: source, ProfileIndex: profile.Index,
		Result: result, CleanupBaselineAfter: after.CleanupBaselineProven,
	}
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

func v2SelectorBenchTimeout(mode benchAutoTuneMode, concurrency int) time.Duration {
	if concurrency < 1 {
		concurrency = 1
	}
	baselineConcurrency := v2DefaultConcurrency()
	if baselineConcurrency < 1 {
		baselineConcurrency = 1
	}
	scale := (baselineConcurrency + concurrency - 1) / concurrency
	if scale < 1 {
		scale = 1
	}
	return time.Duration(mode.TimeoutSec*scale) * time.Second
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
	if req.SessionID != "" && !v2SelectorSessionValid(req.SessionID) {
		return errors.New("invalid selector session_id")
	}
	if _, err := v2NormalizeDPIPropertyVector(req.PropertyVector, benchTransportHTTPS); err != nil {
		return err
	}
	if len(req.Candidates) > v2DirectSelectorMaxCandidates {
		return errors.New("selector candidates exceed limit")
	}
	for _, candidate := range req.Candidates {
		if len(candidate.Args) == 0 || len(candidate.Args) > v2StrategyArgsMax {
			return errors.New("selector candidate args are empty or exceed limit")
		}
		if candidate.ID != "" && !v2StrategySafeID(candidate.ID) {
			return errors.New("selector candidate id is invalid")
		}
		if candidate.Name != "" && !v2StrategySafeName(candidate.Name) {
			return errors.New("selector candidate name is invalid")
		}
	}
	return nil
}

func v2ChooseRecommendation(baseline v2CandidateResult, candidates []v2CandidateResult) (bool, *v2CandidateResult, bool, string) {
	if baseline.ResultClass == "WORKING" && baseline.SuccessRate == 1 {
		return false, nil, false, "baseline completed reliably; bypass strategy is not required for this target"
	}
	var best *v2CandidateResult
	for i := range candidates {
		c := &candidates[i]
		if !c.CleanupProven || !c.InfrastructureOK || c.Successes == 0 || c.ResultClass == "PARTIAL" {
			continue
		}
		if best == nil || v2CandidateBetter(*c, *best) {
			best = c
		}
	}
	if best == nil {
		return false, nil, false, "no candidate proved a working HTTP response through isolated NFQUEUE"
	}
	if best.SuccessRate <= baseline.SuccessRate && best.CompleteRate <= baseline.CompleteRate {
		return false, nil, false, "best candidate did not improve reliability/completeness over baseline"
	}
	copyBest := *best
	return true, &copyBest, true, "candidate improved verified HTTP reachability over baseline"
}

func handleV2Selector(w http.ResponseWriter, r *http.Request) {
	var req v2SelectorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid v2 selector request"})
		return
	}
	autoPoolMeta, poolErr := populateV2SelectorCandidates(&req)
	if poolErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "candidate pool: " + poolErr.Error()})
		return
	}
	if err := validateV2SelectorRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		generated, err := newBenchSessionID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create selector session: " + err.Error()})
			return
		}
		sessionID = generated
	}
	setV2SelectorProgress(sessionID, "PLAN", 0, 0, "building direct catalog", false, false)

	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "another bench session is active", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active", "session_id": sessionID})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	mode, _ := v2SelectorMode(req.Mode)
	mode.Attempts = 1
	target, _ := v2NormalizeTarget(req.ServerName)
	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "production config changed before selector", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed before selector", "current_sha256": status.ConfigSHA256, "session_id": sessionID})
		return
	}
	capabilities := readBenchCapabilities()
	if !benchExecutionReady(capabilities) {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "selector capability gates are not proven", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "selector capability gates are not proven", "session_id": sessionID})
		return
	}
	inventory := readBenchStrategyInventory()
	if !inventory.BaseDependenciesProven {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "strategy base dependencies are not proven", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy base dependencies are not proven", "session_id": sessionID})
		return
	}

	type selectorTemplate struct {
		profile    benchStrategyProfile
		id         string
		name       string
		source     string
		production bool
	}
	templates := []selectorTemplate{}
	seen := map[string]bool{}
	includeProduction := true
	if req.IncludeProduction != nil {
		includeProduction = *req.IncludeProduction
	}

	// P26 direct catalog model: execute the actual corpus/library candidates first.
	// Production profiles are fallback candidates, not a reservation that can
	// starve the portable strategy corpus.
	for i, candidate := range req.Candidates {
		if len(templates) >= v2DirectSelectorMaxCandidates {
			break
		}
		profile, err := v2CustomProfile(candidate.Args, target)
		if err != nil {
			continue
		}
		profile.Index = -1
		fp := v2StrategyFingerprint(profile.Args)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		id := strings.TrimSpace(candidate.ID)
		if id == "" {
			id = fmt.Sprintf("direct-%d", i+1)
		}
		name := strings.TrimSpace(candidate.Name)
		if name == "" {
			name = fmt.Sprintf("Direct candidate %d", i+1)
		}
		templates = append(templates, selectorTemplate{
			profile: profile, id: id, name: name,
			source: v2StrategySource(candidate.Source), production: false,
		})
	}

	if includeProduction && len(templates) < v2DirectSelectorMaxCandidates {
		for _, p := range inventory.Profiles {
			if !p.CandidateEligible {
				continue
			}
			retargeted, err := retargetBenchStrategyProfile(p, target)
			if err != nil {
				continue
			}
			fp := v2StrategyFingerprint(retargeted.Args)
			if fp == "" || seen[fp] {
				continue
			}
			seen[fp] = true
			templates = append(templates, selectorTemplate{
				profile: retargeted, id: fmt.Sprintf("production-%d", p.Index),
				name:   fmt.Sprintf("Production profile %d", p.Index),
				source: "production", production: true,
			})
			if len(templates) >= v2DirectSelectorMaxCandidates {
				break
			}
		}
	}
	if len(templates) == 0 {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "no retargetable strategy candidates", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no retargetable strategy candidates", "session_id": sessionID})
		return
	}

	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 4*time.Second)
	ip, err := resolveBenchServerIPv4(resolveCtx, target)
	cancelResolve()
	if err != nil {
		setV2SelectorProgress(sessionID, "FAILED", 0, len(templates), "resolve target failed", true, true)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve target: " + err.Error(), "session_id": sessionID})
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
		setV2SelectorProgress(sessionID, "FAILED", 0, len(templates), "no free reserved NFQUEUE", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no free reserved NFQUEUE for selector", "session_id": sessionID})
		return
	}

	baselineCtx, cancelBaseline := context.WithTimeout(r.Context(), time.Duration(mode.TimeoutSec)*time.Second)

	setV2SelectorProgress(sessionID, "BASELINE", 0, len(templates), "testing baseline without desync strategy", false, false)
	baseline := v2CandidateResult{
		Baseline: true, CandidateID: "baseline", CandidateName: "Baseline",
		CandidateSource: "baseline", SourceProfileIndex: -1, CleanupProven: true,
	}
	a := v2RunDirectBaseline(baselineCtx, capabilities, inventory, concurrency, target, ip)
	baseline.Attempts = append(baseline.Attempts, a)
	v2FinalizeCandidate(&baseline)
	cancelBaseline()
	if !baseline.CleanupProven || !baseline.InfrastructureOK {
		reason := "baseline infrastructure proof failed; selector stopped fail-closed"
		if !baseline.CleanupProven {
			reason = "baseline cleanup proof failed; selector stopped fail-closed"
		}
		setV2SelectorProgress(sessionID, "FAILED", 0, len(templates), reason, true, true)
		afterFailure := readBenchCapabilities()
		writeJSON(w, http.StatusBadGateway, v2SelectorResponse{
			OK: false, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip, MetricScope: "https-full-response",
			Baseline: baseline, RecommendationReason: reason,
			CleanupBaselineAfter: afterFailure.CleanupBaselineProven, BenchEnabled: afterFailure.BenchEnabled,
			SafeToBench: afterFailure.SafeToBench, Concurrency: concurrency, CandidateSource: "mixed",
			AutoPoolEnabled: autoPoolMeta.Enabled, AutoPoolAdded: autoPoolMeta.Added,
			PoolSources: append([]string{}, autoPoolMeta.Sources...), PoolWarnings: append([]string{}, autoPoolMeta.Warnings...),
			HistoricalPlanning: autoPoolMeta.RecommendationAware, HistoricalHints: autoPoolMeta.RecommendationHints,
			HistoricalPromoted: autoPoolMeta.RecommendationAdded,
			PlannerVersion:     autoPoolMeta.PlannerVersion, PlannerDiagnosticCode: autoPoolMeta.PlannerDiagnosticCode,
			PlannerFaultDomain: autoPoolMeta.PlannerFaultDomain, PlannerStrategyRelevant: autoPoolMeta.PlannerStrategyRelevant,
			PlannerCompatibleCount: autoPoolMeta.PlannerCompatibleCount, PlannerPromotedCount: autoPoolMeta.PlannerPromotedCount,
			PlannerAdmittedRegistry: autoPoolMeta.PlannerAdmittedRegistryCount, PlannerPlan: append([]v2CandidatePoolItem{}, autoPoolMeta.PlannerPlan...),
		})
		return
	}

	benchCtx, cancelBench := context.WithTimeout(r.Context(), v2DirectBenchTimeout(mode, len(templates), concurrency))

	type job struct {
		index int
		item  selectorTemplate
	}
	type jobResult struct {
		index  int
		result v2CandidateResult
	}
	jobs := make(chan job)
	results := make(chan jobResult, len(templates))
	var wg sync.WaitGroup
	setV2SelectorProgress(sessionID, "BENCH", 0, len(templates), "testing direct worker sandboxes", false, false)
	for worker := 0; worker < concurrency; worker++ {
		queue := queues[worker]
		sb := newV2DirectSandbox(capabilities, inventory, worker, queue, target, ip)
		ruleErr := sb.RulesUp(true)
		wg.Add(1)
		go func(sandbox *v2DirectSandbox, setupErr error) {
			defer wg.Done()
			defer sandbox.RulesDown()
			for j := range jobs {
				c := v2CandidateResult{
					CandidateID: j.item.id, CandidateName: j.item.name, CandidateSource: j.item.source,
					SourceProfileIndex: j.item.profile.Index,
					StrategyTags:       append([]int{}, j.item.profile.StrategyTags...),
					Args:               append([]string{}, j.item.profile.Args...), CleanupProven: true,
				}
				if setupErr != nil {
					c.Attempts = append(c.Attempts, v2BenchAttempt{
						InfrastructureOK: false,
						CleanupProven:    false,
						Transport:        benchTransportHTTPS,
						Network:          "tcp",
						RemotePort:       443,
						Queue:            sandbox.queue,
						DestinationIPv4:  ip,
						ResultClass:      "INCONCLUSIVE",
						Error:            "direct worker rules: " + setupErr.Error(),
					})
				} else {
					c.Attempts = append(c.Attempts, sandbox.RunCandidate(benchCtx, j.item.profile))
				}
				v2FinalizeCandidate(&c)
				results <- jobResult{index: j.index, result: c}
			}
		}(sb, ruleErr)
	}

	candidateSlots := make([]v2CandidateResult, len(templates))
	tested := make([]bool, len(templates))
	completed := 0
	working := 0
	next := 0
	benchTimedOut := false

executionLoop:
	for next < len(templates) {
		remaining := len(templates) - next
		batchSize := v2DirectBatchSize(mode.Name, remaining)
		if batchSize <= 0 {
			break
		}
		batchEnd := next + batchSize

		for i := next; i < batchEnd; i++ {
			select {
			case jobs <- job{index: i, item: templates[i]}:
			case <-benchCtx.Done():
				benchTimedOut = true
				break executionLoop
			}
		}

		for received := 0; received < batchSize; received++ {
			select {
			case jr := <-results:
				candidateSlots[jr.index] = jr.result
				tested[jr.index] = true
				completed++
				if jr.result.ResultClass == "WORKING" {
					working++
				}
				setV2SelectorProgress(
					sessionID, "BENCH", completed, len(templates),
					fmt.Sprintf("completed %d of %d candidates · working %d", completed, len(templates), working), false, false,
				)
			case <-benchCtx.Done():
				benchTimedOut = true
				break executionLoop
			}
		}

		next = batchEnd
		if v2DirectShouldStopAfterBatch(mode.Name, working, next, len(templates)) {
			break
		}
	}

	close(jobs)
	wg.Wait()
	cancelBench()

	if benchCtx.Err() != nil {
		benchTimedOut = true
	}
	if benchTimedOut {
		reason := fmt.Sprintf("selector candidate time budget exhausted after %d of %d candidates", completed, len(templates))
		setV2SelectorProgress(sessionID, "FAILED", completed, len(templates), reason, true, true)
		afterFailure := readBenchCapabilities()
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{
			"error": reason, "session_id": sessionID, "completed": completed, "planned": len(templates),
			"cleanup_baseline_after": afterFailure.CleanupBaselineProven,
		})
		return
	}

	candidates := make([]v2CandidateResult, 0, completed)
	for i := range candidateSlots {
		if tested[i] {
			candidates = append(candidates, candidateSlots[i])
		}
	}
	for _, c := range candidates {
		if !c.CleanupProven {
			reason := "candidate cleanup proof failed; selector stopped"
			setV2SelectorProgress(sessionID, "FAILED", completed, len(templates), reason, true, true)
			afterFailure := readBenchCapabilities()
			writeJSON(w, http.StatusBadGateway, v2SelectorResponse{
				OK: false, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip, MetricScope: "https-full-response",
				Baseline: baseline, Candidates: candidates,
				RecommendationReason: reason,
				CleanupBaselineAfter: afterFailure.CleanupBaselineProven, BenchEnabled: afterFailure.BenchEnabled,
				SafeToBench: afterFailure.SafeToBench, Concurrency: concurrency, CandidateSource: "mixed",
				AutoPoolEnabled: autoPoolMeta.Enabled, AutoPoolAdded: autoPoolMeta.Added,
				PoolSources: append([]string{}, autoPoolMeta.Sources...), PoolWarnings: append([]string{}, autoPoolMeta.Warnings...),
				HistoricalPlanning: autoPoolMeta.RecommendationAware, HistoricalHints: autoPoolMeta.RecommendationHints,
				HistoricalPromoted: autoPoolMeta.RecommendationAdded,
				PlannerVersion:     autoPoolMeta.PlannerVersion, PlannerDiagnosticCode: autoPoolMeta.PlannerDiagnosticCode,
				PlannerFaultDomain: autoPoolMeta.PlannerFaultDomain, PlannerStrategyRelevant: autoPoolMeta.PlannerStrategyRelevant,
				PlannerCompatibleCount: autoPoolMeta.PlannerCompatibleCount, PlannerPromotedCount: autoPoolMeta.PlannerPromotedCount,
				PlannerAdmittedRegistry: autoPoolMeta.PlannerAdmittedRegistryCount, PlannerPlan: append([]v2CandidatePoolItem{}, autoPoolMeta.PlannerPlan...),
			})
			return
		}
	}

	setV2SelectorProgress(sessionID, "RANK", completed, len(candidates), "ranking direct worker live results", false, false)
	recommend, best, needed, reason := v2ChooseRecommendation(baseline, candidates)
	after := readBenchCapabilities()
	ok := after.CleanupBaselineProven && strings.EqualFold(readStatus().ConfigSHA256, status.ConfigSHA256)

	profileIndex := -1
	if best != nil {
		profileIndex = best.SourceProfileIndex
	}
	applyEligible := false
	token, expires, applyReason := "", "", "no selector recommendation is available"
	clearBenchAutoTuneApplyPlan()
	if ok && recommend && needed && best != nil {
		var sourceProfile *benchStrategyProfile
		if best.CandidateSource == "production" && best.SourceProfileIndex >= 0 {
			for i := range inventory.Profiles {
				if inventory.Profiles[i].Index == best.SourceProfileIndex && inventory.Profiles[i].CandidateEligible {
					candidate := inventory.Profiles[i]
					sourceProfile = &candidate
					break
				}
			}
		} else {
			matches := v2ProductionProfilesMatchingTarget(target, ip, inventory)
			if len(matches) == 1 {
				candidate := matches[0]
				sourceProfile = &candidate
			} else if len(matches) == 0 {
				applyReason = "verified candidate has no unique production profile binding for this target"
			} else {
				applyReason = "verified candidate matches multiple production profiles; explicit slot selection is required"
			}
		}
		if sourceProfile != nil {
			boundArgs, bindErr := v2BindCandidateToSourceProfile(*sourceProfile, best.Args)
			if bindErr != nil {
				applyReason = "bind candidate to production profile: " + bindErr.Error()
			} else {
				plan, planErr := v2StoreGenericCandidateApplyPlan(
					status.ConfigSHA256, target, ip, benchTransportHTTPS, sessionID,
					*sourceProfile, boundArgs, best,
				)
				if planErr != nil {
					applyReason = "create apply gate: " + planErr.Error()
				} else {
					applyEligible = true
					token = plan.Token
					expires = plan.ExpiresAt.Format(time.RFC3339)
					applyReason = "live-verified candidate is bound to one production profile and eligible for deterministic preview"
				}
			}
		}
	}
	sourceKind := "mixed"
	if len(req.Candidates) == 0 {
		sourceKind = "live-production-proc-cmdline"
	} else if !includeProduction {
		sourceKind = "external"
	}
	recommendedID, recommendedName, recommendedSource := "", "", ""
	if best != nil {
		recommendedID, recommendedName, recommendedSource = best.CandidateID, best.CandidateName, best.CandidateSource
	}
	memoryUpdated := false
	memoryWarning := ""
	if updated, memoryErr := v2RecordSelectorEvidence(target, status.ConfigSHA256, candidates); memoryErr != nil {
		memoryWarning = memoryErr.Error()
	} else {
		memoryUpdated = updated
	}
	resp := v2SelectorResponse{
		OK: ok, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip, MetricScope: "https-full-response",
		Baseline: baseline, Candidates: candidates,
		RecommendationAvailable: recommend, RecommendedProfileIndex: profileIndex,
		RecommendedCandidateID: recommendedID, RecommendedCandidateName: recommendedName, RecommendedCandidateSource: recommendedSource,
		StrategyNeeded:       needed,
		RecommendationReason: reason, CleanupBaselineAfter: after.CleanupBaselineProven,
		BenchEnabled: after.BenchEnabled, SafeToBench: after.SafeToBench, ApplyEnabled: false,
		ApplyGateEligible: applyEligible, ApplyGateToken: token, ApplyGateExpiresAt: expires,
		ApplyGateReason: applyReason, Concurrency: concurrency, CandidateSource: sourceKind,
		AutoPoolEnabled: autoPoolMeta.Enabled, AutoPoolAdded: autoPoolMeta.Added,
		PoolSources: append([]string{}, autoPoolMeta.Sources...), PoolWarnings: append([]string{}, autoPoolMeta.Warnings...),
		HistoricalPlanning: autoPoolMeta.RecommendationAware, HistoricalHints: autoPoolMeta.RecommendationHints,
		HistoricalPromoted: autoPoolMeta.RecommendationAdded,
		PlannerVersion:     autoPoolMeta.PlannerVersion, PlannerDiagnosticCode: autoPoolMeta.PlannerDiagnosticCode,
		PlannerFaultDomain: autoPoolMeta.PlannerFaultDomain, PlannerStrategyRelevant: autoPoolMeta.PlannerStrategyRelevant,
		PlannerCompatibleCount: autoPoolMeta.PlannerCompatibleCount, PlannerPromotedCount: autoPoolMeta.PlannerPromotedCount,
		PlannerAdmittedRegistry: autoPoolMeta.PlannerAdmittedRegistryCount, PlannerPlan: append([]v2CandidatePoolItem{}, autoPoolMeta.PlannerPlan...),
		PropertyVector: req.PropertyVector,

		MemoryUpdated: memoryUpdated, MemoryWarning: memoryWarning,
	}
	if !ok {
		setV2SelectorProgress(sessionID, "FAILED", completed, len(templates), applyReason, true, true)
		writeJSON(w, http.StatusBadGateway, resp)
		return
	}
	setV2SelectorProgress(sessionID, "DONE", completed, len(templates), reason, true, false)
	writeJSON(w, http.StatusOK, resp)
}
