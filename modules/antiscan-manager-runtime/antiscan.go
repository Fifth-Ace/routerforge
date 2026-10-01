package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanUpstreamURL      = "https://github.com/dimon27254/antiscan"
	antiscanCommandTimeout   = 3 * time.Second
	antiscanCommandOutputMax = 64 << 10
)

var antiscanKnownSets = []string{
	"ascn_candidates",
	"ascn_ips",
	"ascn_subnets",
	"ascn_custom_exclude",
	"ascn_custom_blacklist",
	"ascn_custom_whitelist",
	"ascn_geo_blacklist",
	"ascn_geo_whitelist",
	"ascn_geo_exclude",
	"ascn_ndm_lockout",
	"ascn_honeypot",
}

var antiscanIPSetCandidates = []string{
	"/opt/sbin/ipset",
	"/opt/bin/ipset",
	"/usr/sbin/ipset",
	"/sbin/ipset",
}

type antiscanConfig struct {
	Raw                             map[string]string `json:"raw"`
	ISPInterfaces                   []string          `json:"isp_interfaces"`
	Ports                           []string          `json:"ports"`
	ForwardedPorts                  []string          `json:"forwarded_ports"`
	EnableHoneypot                  bool              `json:"enable_honeypot"`
	HoneypotPorts                   []string          `json:"honeypot_ports"`
	HoneypotBanTime                 int64             `json:"honeypot_bantime_seconds"`
	EnableIPSBan                    bool              `json:"enable_ips_ban"`
	RulesMask                       string            `json:"rules_mask"`
	RecentConnectionsTime           int64             `json:"recent_connections_time_seconds"`
	RecentConnectionsHitCount       int64             `json:"recent_connections_hitcount"`
	RecentConnectionsLimit          int64             `json:"recent_connections_limit"`
	RecentConnectionsBanTime        int64             `json:"recent_connections_bantime_seconds"`
	DifferentIPCandidateStorageTime int64             `json:"different_ip_candidates_storage_seconds"`
	DifferentIPThreshold            int64             `json:"different_ip_threshold"`
	SubnetsBanTime                  int64             `json:"subnets_bantime_seconds"`
	SaveIPSets                      bool              `json:"save_ipsets"`
	SaveOnExit                      bool              `json:"save_on_exit"`
	UseCustomExcludeList            bool              `json:"use_custom_exclude_list"`
	CustomListsBlockMode            string            `json:"custom_lists_block_mode"`
	GeoBlockMode                    string            `json:"geoblock_mode"`
	GeoBlockCountries               []string          `json:"geoblock_countries"`
	GeoExcludeCountries             []string          `json:"geo_exclude_countries"`
	ReadNDMLockoutIPSets            bool              `json:"read_ndm_lockout_ipsets"`
	LockoutIPSetBanTime             int64             `json:"lockout_ipset_bantime_seconds"`
	IPSetsDirectory                 string            `json:"ipsets_directory"`
}

type antiscanSetInfo struct {
	Name       string `json:"name"`
	Exists     bool   `json:"exists"`
	Count      int64  `json:"count,omitempty"`
	CountKnown bool   `json:"count_known"`
	Error      string `json:"error,omitempty"`
}

type antiscanProtectionSummary struct {
	IPSBanEnabled             bool  `json:"ips_ban_enabled"`
	HoneypotEnabled           bool  `json:"honeypot_enabled"`
	DifferentIPThreshold      int64 `json:"different_ip_threshold"`
	CandidateStorageSeconds   int64 `json:"candidate_storage_seconds"`
	SubnetBanSeconds          int64 `json:"subnet_ban_seconds"`
	RecentWindowSeconds       int64 `json:"recent_window_seconds"`
	RecentHitCount            int64 `json:"recent_hitcount"`
	ConcurrentConnectionLimit int64 `json:"concurrent_connection_limit"`
	DirectIPBanSeconds        int64 `json:"direct_ip_ban_seconds"`
}

type antiscanSnapshot struct {
	GeneratedAt  time.Time                 `json:"generated_at"`
	Detected     bool                      `json:"detected"`
	Running      bool                      `json:"running"`
	Version      string                    `json:"version,omitempty"`
	MutationAPI  bool                      `json:"mutation_api"`
	Mode         string                    `json:"mode"`
	Upstream     string                    `json:"upstream"`
	InitScript   string                    `json:"init_script"`
	ConfigPath   string                    `json:"config_path"`
	ConfigSHA256 string                    `json:"config_sha256,omitempty"`
	StatusMarker string                    `json:"status_marker"`
	ConfigReload bool                      `json:"config_reload_in_progress"`
	GeoReload    bool                      `json:"geo_reload_in_progress"`
	Config       antiscanConfig            `json:"config"`
	Protection   antiscanProtectionSummary `json:"protection"`
	IPSetBinary  string                    `json:"ipset_binary,omitempty"`
	IPSets       []antiscanSetInfo         `json:"ipsets"`
	Warnings     []string                  `json:"warnings"`
	Errors       []string                  `json:"errors"`
}

type antiscanEvidence struct {
	Kind     string `json:"kind"`
	Set      string `json:"set,omitempty"`
	Matched  bool   `json:"matched"`
	Blocking bool   `json:"blocking"`
	Summary  string `json:"summary"`
}

type antiscanInspectResult struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	IP          string                    `json:"ip"`
	Detected    bool                      `json:"detected"`
	Running     bool                      `json:"running"`
	Conclusive  bool                      `json:"conclusive"`
	Blocked     bool                      `json:"blocked"`
	Verdict     string                    `json:"verdict"`
	Reason      string                    `json:"reason,omitempty"`
	Evidence    []antiscanEvidence        `json:"evidence"`
	Protection  antiscanProtectionSummary `json:"protection"`
	Warnings    []string                  `json:"warnings"`
	MutationAPI bool                      `json:"mutation_api"`
}

func buildAntiscanSnapshot(ctx context.Context, cfg runtimeConfig) antiscanSnapshot {
	configPath := filepath.Join(cfg.AntiscanDir, "ascn.conf")
	config, configErr := readAntiscanConfig(configPath)
	version := readAntiscanVersion(cfg)
	detected := pathExists(cfg.InitScript) || pathExists(configPath) || version != ""
	running := pathExists(cfg.StatusFile)

	snapshot := antiscanSnapshot{
		GeneratedAt:  time.Now().UTC(),
		Detected:     detected,
		Running:      running,
		Version:      version,
		MutationAPI:  true,
		Mode:         "guarded-control",
		Upstream:     antiscanUpstreamURL,
		InitScript:   cfg.InitScript,
		ConfigPath:   configPath,
		ConfigSHA256: hashAntiscanConfigFile(configPath),
		StatusMarker: cfg.StatusFile,
		ConfigReload: pathExists(cfg.ConfigLockFile),
		GeoReload:    pathExists(cfg.GeoLockFile),
		Config:       config,
		Protection:   protectionFromConfig(config),
		Warnings:     antiscanConfigWarnings(config),
		IPSets:       make([]antiscanSetInfo, 0, len(antiscanKnownSets)),
	}
	if configErr != nil && detected {
		snapshot.Errors = append(snapshot.Errors, "config: "+configErr.Error())
	}
	if !detected {
		return snapshot
	}

	binary := findAntiscanIPSetBinary()
	snapshot.IPSetBinary = binary
	if binary == "" {
		snapshot.Errors = append(snapshot.Errors, "ipset binary not found")
		for _, name := range antiscanKnownSets {
			snapshot.IPSets = append(snapshot.IPSets, antiscanSetInfo{Name: name})
		}
		return snapshot
	}

	sets, err := readAntiscanSetInventory(ctx, binary)
	if err != nil {
		snapshot.Errors = append(snapshot.Errors, "ipset inventory: "+err.Error())
	}
	for _, name := range antiscanKnownSets {
		info := sets[name]
		info.Name = name
		snapshot.IPSets = append(snapshot.IPSets, info)
	}
	return snapshot
}

func inspectAntiscanIP(ctx context.Context, cfg runtimeConfig, rawIP string) (antiscanInspectResult, int) {
	addr, err := netip.ParseAddr(strings.TrimSpace(rawIP))
	if err != nil || !addr.Is4() {
		return antiscanInspectResult{
			GeneratedAt: time.Now().UTC(),
			IP:          strings.TrimSpace(rawIP),
			Conclusive:  false,
			Verdict:     "invalid-ip",
			Warnings:    []string{"IPv4 address required"},
			MutationAPI: true,
		}, http.StatusBadRequest
	}

	snapshot := buildAntiscanSnapshot(ctx, cfg)
	base := baseAntiscanInspectResult(snapshot, addr.String())
	if !snapshot.Detected {
		base.Verdict = "not-detected"
		base.Warnings = append(base.Warnings, "Antiscan is not detected on this device")
		return base, http.StatusOK
	}
	if !snapshot.Running {
		base.Verdict = "stopped"
		base.Warnings = append(base.Warnings, "Antiscan runtime marker is absent; live blocking state is not authoritative")
		return base, http.StatusOK
	}
	if snapshot.IPSetBinary == "" {
		base.Warnings = append(base.Warnings, "ipset is unavailable; live membership cannot be checked")
		return base, http.StatusOK
	}

	existing := make(map[string]bool, len(snapshot.IPSets))
	for _, set := range snapshot.IPSets {
		existing[set.Name] = set.Exists
	}
	matches := make(map[string]bool, len(antiscanKnownSets))
	membershipErrors := false
	for _, name := range antiscanKnownSets {
		if !existing[name] {
			continue
		}
		matched, testErr := antiscanIPSetContains(ctx, snapshot.IPSetBinary, name, addr.String())
		if testErr != nil {
			membershipErrors = true
			base.Warnings = append(base.Warnings, name+": "+testErr.Error())
			continue
		}
		matches[name] = matched
	}

	result := classifyAntiscanMembership(snapshot, addr.String(), existing, matches, membershipErrors)
	result.Warnings = append(base.Warnings, result.Warnings...)
	return result, http.StatusOK
}

func baseAntiscanInspectResult(snapshot antiscanSnapshot, ip string) antiscanInspectResult {
	return antiscanInspectResult{
		GeneratedAt: snapshot.GeneratedAt,
		IP:          ip,
		Detected:    snapshot.Detected,
		Running:     snapshot.Running,
		Conclusive:  false,
		Verdict:     "unknown",
		Protection:  snapshot.Protection,
		Warnings:    append([]string(nil), snapshot.Warnings...),
		MutationAPI: true,
	}
}

func classifyAntiscanMembership(
	snapshot antiscanSnapshot,
	ip string,
	existing map[string]bool,
	matches map[string]bool,
	membershipErrors bool,
) antiscanInspectResult {
	result := baseAntiscanInspectResult(snapshot, ip)
	result.Warnings = nil
	appendEvidence := func(kind, set, summary string, blocking bool) {
		result.Evidence = append(result.Evidence, antiscanEvidence{
			Kind: kind, Set: set, Matched: true, Blocking: blocking, Summary: summary,
		})
	}

	// Antiscan inserts exclusions first, so an exclusion wins even if the IP
	// remains present in a lower-priority blocking set.
	if matches["ascn_custom_exclude"] {
		appendEvidence("custom-exclude", "ascn_custom_exclude", "Matched the user exclusion list; Antiscan returns before blocking rules.", false)
		result.Conclusive = true
		result.Blocked = false
		result.Verdict = "excluded"
		result.Reason = "custom-exclude"
		return result
	}
	if matches["ascn_geo_exclude"] {
		appendEvidence("geo-exclude", "ascn_geo_exclude", "Matched the country exclusion set; Antiscan returns before blocking rules.", false)
		result.Conclusive = true
		result.Blocked = false
		result.Verdict = "excluded"
		result.Reason = "geo-exclude"
		return result
	}

	modeCustom := strings.ToLower(snapshot.Config.CustomListsBlockMode)
	modeGeo := strings.ToLower(snapshot.Config.GeoBlockMode)

	if modeCustom == "blacklist" && matches["ascn_custom_blacklist"] {
		appendEvidence("custom-blacklist", "ascn_custom_blacklist", "Matched the active custom blacklist.", true)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "custom-blacklist"
		return result
	}
	if modeCustom == "whitelist" {
		if !existing["ascn_custom_whitelist"] {
			result.Warnings = append(result.Warnings, "custom whitelist mode is configured but the runtime ipset is absent")
			return result
		}
		if !matches["ascn_custom_whitelist"] {
			appendEvidence("custom-whitelist-miss", "ascn_custom_whitelist", "Address is absent from the active custom whitelist.", true)
			result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "custom-whitelist-miss"
			return result
		}
		appendEvidence("custom-whitelist", "ascn_custom_whitelist", "Address is present in the active custom whitelist.", false)
	}

	if modeGeo == "blacklist" && matches["ascn_geo_blacklist"] {
		appendEvidence("geo-blacklist", "ascn_geo_blacklist", "Matched the active geographic blacklist.", true)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "geo-blacklist"
		return result
	}
	if modeGeo == "whitelist" {
		if !existing["ascn_geo_whitelist"] {
			result.Warnings = append(result.Warnings, "geo whitelist mode is configured but the runtime ipset is absent")
			return result
		}
		if !matches["ascn_geo_whitelist"] {
			appendEvidence("geo-whitelist-miss", "ascn_geo_whitelist", "Address is absent from the active geographic whitelist.", true)
			result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "geo-whitelist-miss"
			return result
		}
		appendEvidence("geo-whitelist", "ascn_geo_whitelist", "Address is present in the active geographic whitelist.", false)
	}

	if matches["ascn_ndm_lockout"] {
		appendEvidence("ndm-lockout", "ascn_ndm_lockout", "Matched an address imported from Keenetic ip lockout-policy.", true)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "ndm-lockout"
		return result
	}
	if matches["ascn_honeypot"] {
		appendEvidence("honeypot", "ascn_honeypot", "Matched the Antiscan honeypot block set.", true)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "honeypot"
		return result
	}
	if matches["ascn_subnets"] {
		appendEvidence("distributed-subnet", "ascn_subnets", "Address belongs to a /24 promoted by distributed-address detection.", true)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "distributed-subnet"
		result.Warnings = append(result.Warnings, "Mobile carrier address pools can legitimately rotate through many addresses in one /24; review candidate retention and subnet threshold before changing policy.")
		return result
	}
	if matches["ascn_ips"] {
		appendEvidence("direct-ip", "ascn_ips", "Matched the direct IP block set. Upstream does not persist whether connlimit or recent-hitcount was the exact trigger.", true)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = true, true, "blocked", "direct-ip"
		return result
	}
	if matches["ascn_candidates"] {
		appendEvidence("candidate", "ascn_candidates", "Address is currently a subnet-block candidate but is not blocked by this set alone.", false)
		result.Conclusive, result.Blocked, result.Verdict, result.Reason = !membershipErrors, false, "candidate", "candidate-only"
		return result
	}

	if membershipErrors {
		result.Verdict = "unknown"
		return result
	}
	result.Conclusive = true
	result.Blocked = false
	result.Verdict = "not-blocked"
	result.Reason = "no-active-set-match"
	return result
}

func readAntiscanConfig(path string) (antiscanConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return antiscanConfig{Raw: map[string]string{}}, err
	}
	return parseAntiscanConfig(string(data))
}

func parseAntiscanConfig(text string) (antiscanConfig, error) {
	raw := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			return antiscanConfig{Raw: raw}, fmt.Errorf("line %d: expected KEY=\"VALUE\"", lineNo)
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if !validAntiscanConfigKey(key) || len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return antiscanConfig{Raw: raw}, fmt.Errorf("line %d: invalid Antiscan config syntax", lineNo)
		}
		value = value[1 : len(value)-1]
		if strings.ContainsAny(value, "\x00\r\n`$\\\"") {
			return antiscanConfig{Raw: raw}, fmt.Errorf("line %d: unsupported config value", lineNo)
		}
		raw[key] = value
	}
	if err := scanner.Err(); err != nil {
		return antiscanConfig{Raw: raw}, err
	}

	cfg := antiscanConfig{Raw: raw}
	cfg.ISPInterfaces = fields(raw["ISP_INTERFACES"])
	cfg.Ports = csv(raw["PORTS"])
	cfg.ForwardedPorts = csv(raw["PORTS_FORWARDED"])
	cfg.EnableHoneypot = truthy(raw["ENABLE_HONEYPOT"])
	cfg.HoneypotPorts = csv(raw["HONEYPOT_PORTS"])
	cfg.HoneypotBanTime = integer(raw["HONEYPOT_BANTIME"])
	cfg.EnableIPSBan = raw["ENABLE_IPS_BAN"] == "" || truthy(raw["ENABLE_IPS_BAN"])
	cfg.RulesMask = firstNonEmpty(raw["RULES_MASK"], "255.255.255.255")
	cfg.RecentConnectionsTime = integer(raw["RECENT_CONNECTIONS_TIME"])
	cfg.RecentConnectionsHitCount = integer(raw["RECENT_CONNECTIONS_HITCOUNT"])
	cfg.RecentConnectionsLimit = integer(raw["RECENT_CONNECTIONS_LIMIT"])
	cfg.RecentConnectionsBanTime = integer(raw["RECENT_CONNECTIONS_BANTIME"])
	cfg.DifferentIPCandidateStorageTime = integer(raw["DIFFERENT_IP_CANDIDATES_STORAGETIME"])
	cfg.DifferentIPThreshold = integer(raw["DIFFERENT_IP_THRESHOLD"])
	cfg.SubnetsBanTime = integer(raw["SUBNETS_BANTIME"])
	cfg.SaveIPSets = truthy(raw["SAVE_IPSETS"])
	cfg.SaveOnExit = truthy(raw["SAVE_ON_EXIT"])
	cfg.UseCustomExcludeList = truthy(raw["USE_CUSTOM_EXCLUDE_LIST"])
	cfg.CustomListsBlockMode = firstNonEmpty(raw["CUSTOM_LISTS_BLOCK_MODE"], "0")
	cfg.GeoBlockMode = firstNonEmpty(raw["GEOBLOCK_MODE"], "0")
	cfg.GeoBlockCountries = fields(raw["GEOBLOCK_COUNTRIES"])
	cfg.GeoExcludeCountries = fields(raw["GEO_EXCLUDE_COUNTRIES"])
	cfg.ReadNDMLockoutIPSets = truthy(raw["READ_NDM_LOCKOUT_IPSETS"])
	cfg.LockoutIPSetBanTime = integer(raw["LOCKOUT_IPSET_BANTIME"])
	cfg.IPSetsDirectory = raw["IPSETS_DIRECTORY"]
	return cfg, nil
}

func validAntiscanConfigKey(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func protectionFromConfig(cfg antiscanConfig) antiscanProtectionSummary {
	return antiscanProtectionSummary{
		IPSBanEnabled:             cfg.EnableIPSBan,
		HoneypotEnabled:           cfg.EnableHoneypot,
		DifferentIPThreshold:      cfg.DifferentIPThreshold,
		CandidateStorageSeconds:   cfg.DifferentIPCandidateStorageTime,
		SubnetBanSeconds:          cfg.SubnetsBanTime,
		RecentWindowSeconds:       cfg.RecentConnectionsTime,
		RecentHitCount:            cfg.RecentConnectionsHitCount,
		ConcurrentConnectionLimit: cfg.RecentConnectionsLimit,
		DirectIPBanSeconds:        cfg.RecentConnectionsBanTime,
	}
}

func antiscanConfigWarnings(cfg antiscanConfig) []string {
	var out []string
	if cfg.EnableIPSBan && cfg.DifferentIPThreshold > 0 && cfg.DifferentIPThreshold <= 5 && cfg.DifferentIPCandidateStorageTime >= 86400 {
		out = append(out, "Distributed subnet detection is aggressive for rotating/mobile address pools: a small /24 threshold is combined with long candidate retention.")
	}
	if cfg.EnableIPSBan && cfg.RecentConnectionsTime > 0 && cfg.RecentConnectionsHitCount > 0 && cfg.RecentConnectionsHitCount <= 15 {
		out = append(out, "Burst-sensitive direct IP protection is enabled; browsers and mobile apps can open many TCP connections in a short window.")
	}
	return out
}

func readAntiscanVersion(cfg runtimeConfig) string {
	for _, path := range []string{
		"/opt/lib/opkg/info/antiscan.control",
		"/opt/var/opkg/info/antiscan.control",
	} {
		if version := parsePackageVersion(string(readFile(path))); version != "" {
			return version
		}
	}
	return parseScriptVersion(string(readFile(cfg.InitScript)))
}

func parsePackageVersion(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Version:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		}
	}
	return ""
}

func parseScriptVersion(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ASCN_VERSION=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "ASCN_VERSION="))
		return strings.Trim(value, "\"'")
	}
	return ""
}

func findAntiscanIPSetBinary() string {
	for _, path := range antiscanIPSetCandidates {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return path
		}
	}
	return ""
}

func readAntiscanSetInventory(parent context.Context, binary string) (map[string]antiscanSetInfo, error) {
	out := make(map[string]antiscanSetInfo, len(antiscanKnownSets))
	ctx, cancel := context.WithTimeout(parent, antiscanCommandTimeout)
	defer cancel()
	data, err := safety.RunCommand(ctx, antiscanCommandOutputMax, binary, "-n", "list")
	if err != nil {
		return out, fmt.Errorf("list sets: %w", err)
	}
	present := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			present[name] = true
		}
	}

	for _, name := range antiscanKnownSets {
		info := antiscanSetInfo{Name: name, Exists: present[name]}
		if !info.Exists {
			out[name] = info
			continue
		}
		count, countErr := readAntiscanSetCount(parent, binary, name)
		if countErr != nil {
			info.Error = countErr.Error()
		} else {
			info.Count = count
			info.CountKnown = true
		}
		out[name] = info
	}
	return out, nil
}

func readAntiscanSetCount(parent context.Context, binary, setName string) (int64, error) {
	if !knownAntiscanSet(setName) {
		return 0, errors.New("unknown Antiscan ipset")
	}
	ctx, cancel := context.WithTimeout(parent, antiscanCommandTimeout)
	defer cancel()
	data, err := safety.RunCommand(ctx, antiscanCommandOutputMax, binary, "list", setName, "-terse")
	if err != nil {
		return 0, err
	}
	return parseIPSetCount(string(data))
}

func parseIPSetCount(text string) (int64, error) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Number of entries:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "Number of entries:"))
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil || count < 0 {
			return 0, errors.New("invalid ipset entry count")
		}
		return count, nil
	}
	return 0, errors.New("ipset entry count missing")
}

func antiscanIPSetContains(parent context.Context, binary, setName, ip string) (bool, error) {
	if !knownAntiscanSet(setName) {
		return false, errors.New("unknown Antiscan ipset")
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return false, errors.New("invalid IPv4 address")
	}
	ctx, cancel := context.WithTimeout(parent, antiscanCommandTimeout)
	defer cancel()
	data, runErr := safety.RunCommand(ctx, 4096, binary, "test", setName, addr.String())
	if runErr == nil {
		return true, nil
	}
	text := strings.ToLower(string(data))
	if strings.Contains(text, "is not in set") || strings.Contains(text, "not in set") {
		return false, nil
	}
	return false, runErr
}

func knownAntiscanSet(name string) bool {
	for _, candidate := range antiscanKnownSets {
		if name == candidate {
			return true
		}
	}
	return false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

func truthy(value string) bool {
	return strings.TrimSpace(value) == "1"
}

func integer(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}

func fields(value string) []string {
	return append([]string(nil), strings.Fields(value)...)
}

func csv(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
