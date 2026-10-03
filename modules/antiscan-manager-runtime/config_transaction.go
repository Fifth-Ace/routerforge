package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanConfigFileMaxBytes = 256 << 10
	antiscanConfigApplyTimeout = 5 * time.Minute
	antiscanConfigOutputMax    = 64 << 10
)

var (
	antiscanInterfacePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{1,14}$`)
	antiscanCountryPattern   = regexp.MustCompile(`^[A-Z]{2}$`)
)

var antiscanConfigKeys = []string{
	"ISP_INTERFACES",
	"PORTS",
	"PORTS_FORWARDED",
	"ENABLE_HONEYPOT",
	"HONEYPOT_PORTS",
	"HONEYPOT_BANTIME",
	"ENABLE_IPS_BAN",
	"RULES_MASK",
	"RECENT_CONNECTIONS_TIME",
	"RECENT_CONNECTIONS_HITCOUNT",
	"RECENT_CONNECTIONS_LIMIT",
	"RECENT_CONNECTIONS_BANTIME",
	"DIFFERENT_IP_CANDIDATES_STORAGETIME",
	"DIFFERENT_IP_THRESHOLD",
	"SUBNETS_BANTIME",
	"IPSETS_DIRECTORY",
	"SAVE_IPSETS",
	"SAVE_ON_EXIT",
	"USE_CUSTOM_EXCLUDE_LIST",
	"CUSTOM_LISTS_BLOCK_MODE",
	"GEOBLOCK_MODE",
	"GEOBLOCK_COUNTRIES",
	"GEO_EXCLUDE_COUNTRIES",
	"READ_NDM_LOCKOUT_IPSETS",
	"LOCKOUT_IPSET_BANTIME",
}

var antiscanConfigKeySet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(antiscanConfigKeys))
	for _, key := range antiscanConfigKeys {
		out[key] = struct{}{}
	}
	return out
}()

type antiscanConfigUpdateRequest struct {
	BaseSHA256                      string `json:"base_sha256"`
	Confirm                         string `json:"confirm"`
	ISPInterfaces                   string `json:"isp_interfaces"`
	Ports                           string `json:"ports"`
	ForwardedPorts                  string `json:"forwarded_ports"`
	EnableHoneypot                  bool   `json:"enable_honeypot"`
	HoneypotPorts                   string `json:"honeypot_ports"`
	HoneypotBanTime                 int64  `json:"honeypot_bantime_seconds"`
	EnableIPSBan                    bool   `json:"enable_ips_ban"`
	RulesMask                       string `json:"rules_mask"`
	RecentConnectionsTime           int64  `json:"recent_connections_time_seconds"`
	RecentConnectionsHitCount       int64  `json:"recent_connections_hitcount"`
	RecentConnectionsLimit          int64  `json:"recent_connections_limit"`
	RecentConnectionsBanTime        int64  `json:"recent_connections_bantime_seconds"`
	DifferentIPCandidateStorageTime int64  `json:"different_ip_candidates_storage_seconds"`
	DifferentIPThreshold            int64  `json:"different_ip_threshold"`
	SubnetsBanTime                  int64  `json:"subnets_bantime_seconds"`
	IPSetsDirectory                 string `json:"ipsets_directory"`
	SaveIPSets                      bool   `json:"save_ipsets"`
	SaveOnExit                      bool   `json:"save_on_exit"`
	UseCustomExcludeList            bool   `json:"use_custom_exclude_list"`
	CustomListsBlockMode            string `json:"custom_lists_block_mode"`
	GeoBlockMode                    string `json:"geoblock_mode"`
	GeoBlockCountries               string `json:"geoblock_countries"`
	GeoExcludeCountries             string `json:"geo_exclude_countries"`
	ReadNDMLockoutIPSets            bool   `json:"read_ndm_lockout_ipsets"`
	LockoutIPSetBanTime             int64  `json:"lockout_ipset_bantime_seconds"`
}

type antiscanConfigApplyResult struct {
	Action            string   `json:"action"`
	BeforeSHA256      string   `json:"before_sha256"`
	AfterSHA256       string   `json:"after_sha256"`
	BackupPath        string   `json:"backup_path,omitempty"`
	Changed           bool     `json:"changed"`
	Verified          bool     `json:"verified"`
	RuntimeApplied    bool     `json:"runtime_applied"`
	RollbackPerformed bool     `json:"rollback_performed"`
	RestartRequired   bool     `json:"restart_required"`
	MutationAPI       bool     `json:"mutation_api"`
	Output            string   `json:"output,omitempty"`
	Warnings          []string `json:"warnings"`
	Error             string   `json:"error,omitempty"`
}

type antiscanConfigFileState struct {
	Path string
	Data []byte
	Mode os.FileMode
	SHA  string
}

func handleAntiscanConfigApply(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanConfigUpdateRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config request", "mutation_api": true})
			return
		}
		if request.Confirm != "APPLY_CONFIG" {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal APPLY_CONFIG", "mutation_api": true})
			return
		}

		result, status, err := applyAntiscanConfig(r.Context(), cfg, request)
		if err != nil {
			result.Error = err.Error()
		}
		writeJSON(w, status, result)
	}
}

func applyAntiscanConfig(parent context.Context, cfg runtimeConfig, request antiscanConfigUpdateRequest) (antiscanConfigApplyResult, int, error) {
	result := antiscanConfigApplyResult{
		Action:      "apply-config",
		MutationAPI: true,
		Warnings:    []string{},
	}
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return result, http.StatusConflict, errors.New("Antiscan reload is already in progress")
	}

	values, err := normalizeAntiscanConfigUpdate(request)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	if err := validateAntiscanIPSetsDirectory(values["IPSETS_DIRECTORY"], values); err != nil {
		return result, http.StatusBadRequest, err
	}

	state, err := readAntiscanConfigFileState(cfg)
	if err != nil {
		return result, http.StatusConflict, err
	}
	result.BeforeSHA256 = state.SHA
	result.AfterSHA256 = state.SHA

	baseSHA := strings.ToLower(strings.TrimSpace(request.BaseSHA256))
	if baseSHA == "" || baseSHA != state.SHA {
		return result, http.StatusConflict, errors.New("ascn.conf changed since it was loaded; refresh before applying")
	}

	candidate, err := mergeAntiscanConfigText(string(state.Data), values)
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("prepare ascn.conf: %w", err)
	}
	if err := validateAntiscanCandidateSyntax(parent, state.Path, candidate, state.Mode); err != nil {
		return result, http.StatusBadRequest, err
	}
	if err := verifyAntiscanConfigValues(candidate, values); err != nil {
		return result, http.StatusBadRequest, err
	}

	candidateSHA := sha256Hex(candidate)
	result.AfterSHA256 = candidateSHA
	if bytes.Equal(state.Data, candidate) {
		result.Verified = true
		result.Warnings = append(result.Warnings, "ascn.conf already matches the requested settings.")
		return result, http.StatusOK, nil
	}

	if err := verifyAntiscanConfigUnchanged(state.Path, state.SHA); err != nil {
		return result, http.StatusConflict, err
	}

	backupPath := state.Path + ".routerforge.bak"
	if err := safety.WriteFileAtomic(backupPath, state.Data, state.Mode); err != nil {
		return result, http.StatusConflict, fmt.Errorf("create config backup: %w", err)
	}
	result.BackupPath = backupPath

	if err := safety.WriteFileAtomic(state.Path, candidate, state.Mode); err != nil {
		return result, http.StatusConflict, fmt.Errorf("write ascn.conf: %w", err)
	}
	result.Changed = true

	running := pathExists(cfg.StatusFile)
	oldParsed, _ := parseAntiscanConfig(string(state.Data))
	newParsed, _ := parseAntiscanConfig(string(candidate))
	if strings.TrimSpace(oldParsed.IPSetsDirectory) != strings.TrimSpace(newParsed.IPSetsDirectory) && running {
		result.RestartRequired = true
		result.Warnings = append(result.Warnings, "IPSETS_DIRECTORY changed. Upstream Antiscan requires a stop/start cycle before the new storage directory is fully active.")
	}

	if !running {
		if err := verifyAntiscanConfigFile(state.Path, candidateSHA, values); err != nil {
			rollbackErr := safety.WriteFileAtomic(state.Path, state.Data, state.Mode)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, fmt.Errorf("config verification failed and rollback failed: verify=%v rollback=%v", err, rollbackErr)
			}
			return result, http.StatusConflict, fmt.Errorf("config verification failed; file was rolled back: %w", err)
		}
		result.Verified = true
		result.Warnings = append(result.Warnings, "Antiscan is stopped. Settings were stored and will become active on the next start.")
		return result, http.StatusOK, nil
	}

	if err := antiscanLifecycleReady(cfg); err != nil {
		rollbackErr := safety.WriteFileAtomic(state.Path, state.Data, state.Mode)
		result.RollbackPerformed = rollbackErr == nil
		if rollbackErr != nil {
			return result, http.StatusInternalServerError, fmt.Errorf("lifecycle preflight failed and config rollback failed: preflight=%v rollback=%v", err, rollbackErr)
		}
		return result, http.StatusConflict, fmt.Errorf("lifecycle preflight failed; config was rolled back: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, antiscanConfigApplyTimeout)
	output, reloadErr := safety.RunCommand(ctx, antiscanConfigOutputMax, cfg.InitScript, "reload")
	cancel()
	result.Output = strings.TrimSpace(string(output))
	if reloadErr != nil {
		return rollbackAntiscanConfigAfterFailure(parent, cfg, state, result, fmt.Errorf("upstream reload failed: %s", compactMutationOutput(output, reloadErr)))
	}
	if !pathExists(cfg.StatusFile) {
		return rollbackAntiscanConfigAfterFailure(parent, cfg, state, result, errors.New("reload verification failed: Antiscan stopped during config apply"))
	}
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return rollbackAntiscanConfigAfterFailure(parent, cfg, state, result, errors.New("reload verification failed: an upstream reload lock remains"))
	}
	if err := verifyAntiscanConfigFile(state.Path, candidateSHA, values); err != nil {
		return rollbackAntiscanConfigAfterFailure(parent, cfg, state, result, fmt.Errorf("config verification failed after reload: %w", err))
	}

	result.RuntimeApplied = true
	result.Verified = true
	return result, http.StatusOK, nil
}

func rollbackAntiscanConfigAfterFailure(parent context.Context, cfg runtimeConfig, state antiscanConfigFileState, result antiscanConfigApplyResult, cause error) (antiscanConfigApplyResult, int, error) {
	if err := safety.WriteFileAtomic(state.Path, state.Data, state.Mode); err != nil {
		return result, http.StatusInternalServerError, fmt.Errorf("%v; config rollback write failed: %w", cause, err)
	}
	result.RollbackPerformed = true
	result.AfterSHA256 = state.SHA

	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return result, http.StatusInternalServerError, fmt.Errorf("%v; config file was restored but upstream left a reload lock, so runtime rollback was not attempted", cause)
	}
	if !pathExists(cfg.StatusFile) {
		return result, http.StatusInternalServerError, fmt.Errorf("%v; config file was restored but Antiscan is not running, so runtime rollback could not be verified", cause)
	}

	ctx, cancel := context.WithTimeout(parent, antiscanConfigApplyTimeout)
	output, err := safety.RunCommand(ctx, antiscanConfigOutputMax, cfg.InitScript, "reload")
	cancel()
	if err != nil {
		return result, http.StatusInternalServerError, fmt.Errorf("%v; config file was restored but upstream rollback reload failed: %s", cause, compactMutationOutput(output, err))
	}
	if !pathExists(cfg.StatusFile) || pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return result, http.StatusInternalServerError, fmt.Errorf("%v; rollback reload returned without a healthy Antiscan runtime", cause)
	}
	if got, err := os.ReadFile(state.Path); err != nil || !bytes.Equal(got, state.Data) {
		return result, http.StatusInternalServerError, fmt.Errorf("%v; rollback reload completed but ascn.conf was not restored exactly", cause)
	}
	return result, http.StatusConflict, fmt.Errorf("%v; previous ascn.conf and runtime state were restored", cause)
}

func readAntiscanConfigFileState(cfg runtimeConfig) (antiscanConfigFileState, error) {
	resolver := safety.Resolver{Roots: []string{cfg.AntiscanDir}}
	resolved, err := resolver.ResolveExisting(filepath.Join(cfg.AntiscanDir, "ascn.conf"), nil)
	if err != nil {
		return antiscanConfigFileState{}, fmt.Errorf("resolve ascn.conf: %w", err)
	}
	info, err := os.Stat(resolved.Canonical)
	if err != nil {
		return antiscanConfigFileState{}, fmt.Errorf("stat ascn.conf: %w", err)
	}
	if !info.Mode().IsRegular() {
		return antiscanConfigFileState{}, errors.New("ascn.conf is not a regular file")
	}
	file, err := os.Open(resolved.Canonical)
	if err != nil {
		return antiscanConfigFileState{}, fmt.Errorf("open ascn.conf: %w", err)
	}
	defer file.Close()
	data, err := readBounded(file, antiscanConfigFileMaxBytes)
	if err != nil {
		return antiscanConfigFileState{}, fmt.Errorf("read ascn.conf: %w", err)
	}
	return antiscanConfigFileState{
		Path: resolved.Canonical,
		Data: data,
		Mode: info.Mode().Perm(),
		SHA:  sha256Hex(data),
	}, nil
}

func readBounded(file *os.File, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errors.New("file exceeds RouterForge safety limit")
	}
	return data, nil
}

func hashAntiscanConfigFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > antiscanConfigFileMaxBytes {
		return ""
	}
	return sha256Hex(data)
}

func verifyAntiscanConfigUnchanged(path, expectedSHA string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("re-read ascn.conf before write: %w", err)
	}
	if len(data) > antiscanConfigFileMaxBytes {
		return errors.New("ascn.conf exceeds RouterForge safety limit")
	}
	if sha256Hex(data) != expectedSHA {
		return errors.New("ascn.conf changed during transaction preflight; refresh before applying")
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeAntiscanConfigUpdate(request antiscanConfigUpdateRequest) (map[string]string, error) {
	interfaces, err := normalizeAntiscanInterfaces(request.ISPInterfaces)
	if err != nil {
		return nil, err
	}
	ports, err := normalizeAntiscanPorts(request.Ports, "PORTS", true)
	if err != nil {
		return nil, err
	}
	forwarded, err := normalizeAntiscanPorts(request.ForwardedPorts, "PORTS_FORWARDED", true)
	if err != nil {
		return nil, err
	}
	if ports == "" && forwarded == "" {
		return nil, errors.New("PORTS and PORTS_FORWARDED cannot both be empty")
	}
	honeypotPorts, err := normalizeAntiscanPorts(request.HoneypotPorts, "HONEYPOT_PORTS", true)
	if err != nil {
		return nil, err
	}
	if request.EnableHoneypot && honeypotPorts == "" {
		return nil, errors.New("HONEYPOT_PORTS is required when honeypot is enabled")
	}
	if err := intRange("HONEYPOT_BANTIME", request.HoneypotBanTime, 0, 2147483); err != nil {
		return nil, err
	}

	rulesMask := strings.TrimSpace(request.RulesMask)
	if rulesMask == "" {
		rulesMask = "255.255.255.255"
	}
	addr, err := netip.ParseAddr(rulesMask)
	if err != nil || !addr.Is4() {
		return nil, errors.New("RULES_MASK must be a dotted IPv4 mask")
	}

	if request.EnableIPSBan {
		for _, check := range []struct {
			name       string
			value, min int64
			max        int64
		}{
			{"RECENT_CONNECTIONS_TIME", request.RecentConnectionsTime, 2, 3600},
			{"RECENT_CONNECTIONS_HITCOUNT", request.RecentConnectionsHitCount, 2, 100},
			{"RECENT_CONNECTIONS_LIMIT", request.RecentConnectionsLimit, 3, 1000},
			{"RECENT_CONNECTIONS_BANTIME", request.RecentConnectionsBanTime, 0, 2147483},
			{"DIFFERENT_IP_CANDIDATES_STORAGETIME", request.DifferentIPCandidateStorageTime, 0, 2147483},
			{"DIFFERENT_IP_THRESHOLD", request.DifferentIPThreshold, 2, 254},
			{"SUBNETS_BANTIME", request.SubnetsBanTime, 0, 2147483},
		} {
			if err := intRange(check.name, check.value, check.min, check.max); err != nil {
				return nil, err
			}
		}
	}
	if err := intRange("LOCKOUT_IPSET_BANTIME", request.LockoutIPSetBanTime, 0, 2147483); err != nil {
		return nil, err
	}

	customMode, err := normalizeAntiscanMode(request.CustomListsBlockMode, "CUSTOM_LISTS_BLOCK_MODE")
	if err != nil {
		return nil, err
	}
	geoMode, err := normalizeAntiscanMode(request.GeoBlockMode, "GEOBLOCK_MODE")
	if err != nil {
		return nil, err
	}
	geoCountries, err := normalizeAntiscanCountries(request.GeoBlockCountries, "GEOBLOCK_COUNTRIES")
	if err != nil {
		return nil, err
	}
	geoExclude, err := normalizeAntiscanCountries(request.GeoExcludeCountries, "GEO_EXCLUDE_COUNTRIES")
	if err != nil {
		return nil, err
	}
	if geoMode != "0" && geoCountries == "" {
		return nil, errors.New("GEOBLOCK_COUNTRIES is required when GEOBLOCK_MODE is enabled")
	}

	ipsetsDir := strings.TrimSpace(request.IPSetsDirectory)
	if strings.ContainsAny(ipsetsDir, "\x00\r\n`$\\\"") {
		return nil, errors.New("IPSETS_DIRECTORY contains unsupported characters")
	}
	if (request.SaveIPSets || geoMode != "0" || geoExclude != "") && ipsetsDir == "" {
		return nil, errors.New("IPSETS_DIRECTORY is required for persistence or Geo lists")
	}

	values := map[string]string{
		"ISP_INTERFACES":                      interfaces,
		"PORTS":                               ports,
		"PORTS_FORWARDED":                     forwarded,
		"ENABLE_HONEYPOT":                     bool01(request.EnableHoneypot),
		"HONEYPOT_PORTS":                      honeypotPorts,
		"HONEYPOT_BANTIME":                    strconv.FormatInt(request.HoneypotBanTime, 10),
		"ENABLE_IPS_BAN":                      bool01(request.EnableIPSBan),
		"RULES_MASK":                          rulesMask,
		"RECENT_CONNECTIONS_TIME":             strconv.FormatInt(request.RecentConnectionsTime, 10),
		"RECENT_CONNECTIONS_HITCOUNT":         strconv.FormatInt(request.RecentConnectionsHitCount, 10),
		"RECENT_CONNECTIONS_LIMIT":            strconv.FormatInt(request.RecentConnectionsLimit, 10),
		"RECENT_CONNECTIONS_BANTIME":          strconv.FormatInt(request.RecentConnectionsBanTime, 10),
		"DIFFERENT_IP_CANDIDATES_STORAGETIME": strconv.FormatInt(request.DifferentIPCandidateStorageTime, 10),
		"DIFFERENT_IP_THRESHOLD":              strconv.FormatInt(request.DifferentIPThreshold, 10),
		"SUBNETS_BANTIME":                     strconv.FormatInt(request.SubnetsBanTime, 10),
		"IPSETS_DIRECTORY":                    ipsetsDir,
		"SAVE_IPSETS":                         bool01(request.SaveIPSets),
		"SAVE_ON_EXIT":                        bool01(request.SaveOnExit),
		"USE_CUSTOM_EXCLUDE_LIST":             bool01(request.UseCustomExcludeList),
		"CUSTOM_LISTS_BLOCK_MODE":             customMode,
		"GEOBLOCK_MODE":                       geoMode,
		"GEOBLOCK_COUNTRIES":                  geoCountries,
		"GEO_EXCLUDE_COUNTRIES":               geoExclude,
		"READ_NDM_LOCKOUT_IPSETS":             bool01(request.ReadNDMLockoutIPSets),
		"LOCKOUT_IPSET_BANTIME":               strconv.FormatInt(request.LockoutIPSetBanTime, 10),
	}
	return values, nil
}

func normalizeAntiscanInterfaces(raw string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 || len(parts) > 10 {
		return "", errors.New("ISP_INTERFACES must contain between 1 and 10 interface names")
	}
	for _, part := range parts {
		if !antiscanInterfacePattern.MatchString(part) {
			return "", fmt.Errorf("invalid ISP interface %q", part)
		}
	}
	return strings.Join(parts, " "), nil
}

func normalizeAntiscanPorts(raw, name string, allowEmpty bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("%s cannot be empty", name)
	}
	items := strings.Split(raw, ",")
	endpointCount := 0
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			return "", fmt.Errorf("%s contains an empty port item", name)
		}
		bounds := strings.Split(item, ":")
		if len(bounds) > 2 {
			return "", fmt.Errorf("%s contains an invalid port range", name)
		}
		numbers := make([]int, 0, len(bounds))
		for _, bound := range bounds {
			value, err := strconv.Atoi(bound)
			if err != nil || value < 1 || value > 65535 {
				return "", fmt.Errorf("%s ports must be between 1 and 65535", name)
			}
			numbers = append(numbers, value)
			endpointCount++
		}
		if len(numbers) == 2 && numbers[0] > numbers[1] {
			return "", fmt.Errorf("%s port range start must not exceed end", name)
		}
		if len(numbers) == 1 {
			out = append(out, strconv.Itoa(numbers[0]))
		} else {
			out = append(out, strconv.Itoa(numbers[0])+":"+strconv.Itoa(numbers[1]))
		}
	}
	if endpointCount > 15 {
		return "", fmt.Errorf("%s exceeds the upstream 15-value limit", name)
	}
	return strings.Join(out, ","), nil
}

func normalizeAntiscanMode(raw, name string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		mode = "0"
	}
	switch mode {
	case "0", "blacklist", "whitelist":
		return mode, nil
	default:
		return "", fmt.Errorf("%s must be 0, blacklist or whitelist", name)
	}
}

func normalizeAntiscanCountries(raw, name string) (string, error) {
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(raw)))
	if len(parts) > 8 {
		return "", fmt.Errorf("%s supports at most 8 country codes", name)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if !antiscanCountryPattern.MatchString(part) {
			return "", fmt.Errorf("%s contains invalid country code %q", name, part)
		}
		if !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	return strings.Join(out, " "), nil
}

func intRange(name string, value, min, max int64) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return nil
}

func bool01(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func validateAntiscanIPSetsDirectory(raw string, values map[string]string) error {
	path := strings.TrimSpace(raw)
	required := values["SAVE_IPSETS"] == "1" || values["GEOBLOCK_MODE"] != "0" || values["GEO_EXCLUDE_COUNTRIES"] != ""
	if path == "" {
		if required {
			return errors.New("IPSETS_DIRECTORY is required by the selected persistence/Geo settings")
		}
		return nil
	}
	if !filepath.IsAbs(path) || safety.HasParentTraversal(path) {
		return errors.New("IPSETS_DIRECTORY must be an absolute path without parent traversal")
	}
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("IPSETS_DIRECTORY contains an invalid path segment")
		}
	}

	clean := filepath.Clean(path)
	if clean == "/opt/etc" {
		return errors.New("IPSETS_DIRECTORY must not be /opt/etc")
	}
	if clean != "/opt" && clean != "/tmp" &&
		!strings.HasPrefix(clean, "/opt/") && !strings.HasPrefix(clean, "/tmp/") {
		return errors.New("IPSETS_DIRECTORY must be under /opt or /tmp")
	}
	return nil
}

func mergeAntiscanConfigText(original string, values map[string]string) ([]byte, error) {
	for _, key := range antiscanConfigKeys {
		value, ok := values[key]
		if !ok {
			return nil, fmt.Errorf("missing config value %s", key)
		}
		if strings.ContainsAny(value, "\x00\r\n`$\\\"") {
			return nil, fmt.Errorf("%s contains unsupported characters", key)
		}
	}

	lines := strings.Split(strings.ReplaceAll(original, "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("line %d: expected KEY=\"VALUE\"", i+1)
		}
		key := strings.TrimSpace(trimmed[:idx])
		if _, ok := antiscanConfigKeySet[key]; !ok {
			return nil, fmt.Errorf("line %d: unsupported Antiscan config key %s", i+1, key)
		}
		if seen[key] {
			return nil, fmt.Errorf("line %d: duplicate Antiscan config key %s", i+1, key)
		}
		seen[key] = true
		lines[i] = key + "=\"" + values[key] + "\""
	}

	missing := make([]string, 0)
	for _, key := range antiscanConfigKeys {
		if !seen[key] {
			missing = append(missing, key+"=\""+values[key]+"\"")
		}
	}
	if len(missing) > 0 {
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, "", "# Added by RouterForge Antiscan Manager")
		lines = append(lines, missing...)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func validateAntiscanCandidateSyntax(parent context.Context, configPath string, candidate []byte, mode os.FileMode) error {
	if len(candidate) == 0 || len(candidate) > antiscanConfigFileMaxBytes {
		return errors.New("generated ascn.conf exceeds RouterForge safety limit")
	}
	dir := filepath.Dir(configPath)
	atomicFile, err := safety.NewAtomicFile(dir, ".routerforge-ascn-validate-*")
	if err != nil {
		return fmt.Errorf("create config validation file: %w", err)
	}
	defer atomicFile.Cleanup()
	if err := atomicFile.File().Chmod(mode); err != nil {
		return err
	}
	if _, err := atomicFile.File().Write(candidate); err != nil {
		return err
	}
	if err := atomicFile.Sync(); err != nil {
		return err
	}
	if err := atomicFile.Close(); err != nil {
		return err
	}

	shell := findAntiscanValidationShell()
	if shell == "" {
		return errors.New("no shell is available for ascn.conf syntax validation")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	output, err := safety.RunCommand(ctx, 8<<10, shell, "-n", atomicFile.Path())
	if err != nil {
		return fmt.Errorf("ascn.conf shell syntax validation failed: %s", compactMutationOutput(output, err))
	}
	return nil
}

func findAntiscanValidationShell() string {
	for _, candidate := range []string{"/opt/bin/sh", "/bin/sh", "/usr/bin/sh"} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return candidate
		}
	}
	return ""
}

func verifyAntiscanConfigValues(candidate []byte, values map[string]string) error {
	cfg, err := parseAntiscanConfig(string(candidate))
	if err != nil {
		return fmt.Errorf("parse generated config: %w", err)
	}
	for _, key := range antiscanConfigKeys {
		if cfg.Raw[key] != values[key] {
			return fmt.Errorf("generated config verification mismatch for %s", key)
		}
	}
	return nil
}

func verifyAntiscanConfigFile(path, expectedSHA string, values map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if sha256Hex(data) != expectedSHA {
		return errors.New("ascn.conf checksum changed unexpectedly")
	}
	return verifyAntiscanConfigValues(data, values)
}
