package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const benchProductionPIDFile = "/opt/var/run/nfqws2.pid"

type benchStrategyDependency struct {
	Kind       string `json:"kind"`
	Argument   string `json:"argument"`
	Path       string `json:"path,omitempty"`
	Resolved   string `json:"resolved,omitempty"`
	Proven     bool   `json:"proven"`
	Resolution string `json:"resolution,omitempty"`
}

type benchStrategyProfile struct {
	Index             int      `json:"index"`
	Args              []string `json:"args"`
	TCPFilters        []string `json:"tcp_filters"`
	UDPFilters        []string `json:"udp_filters"`
	L7Filters         []string `json:"l7_filters"`
	Payloads          []string `json:"payloads"`
	HostlistDomains   []string `json:"hostlist_domains"`
	FileBoundFilters  []string `json:"file_bound_filters"`
	DesyncCount       int      `json:"desync_count"`
	StrategyTags      []int    `json:"strategy_tags"`
	CandidateEligible bool     `json:"candidate_eligible"`
	Reasons           []string `json:"reasons"`
}

type benchStrategyInventory struct {
	ReadOnly               bool                      `json:"read_only"`
	CompilerImplemented    bool                      `json:"compiler_implemented"`
	ExecutionEnabled       bool                      `json:"execution_enabled"`
	Source                 string                    `json:"source"`
	ProductionPID          int                       `json:"production_pid,omitempty"`
	ProductionExecutable   string                    `json:"production_executable,omitempty"`
	BaseArgs               []string                  `json:"base_args"`
	BaseDependencies       []benchStrategyDependency `json:"base_dependencies"`
	BaseDependenciesProven bool                      `json:"base_dependencies_proven"`
	Profiles               []benchStrategyProfile    `json:"profiles"`
	ProfileCount           int                       `json:"profile_count"`
	EligibleProfileCount   int                       `json:"eligible_profile_count"`
	Warnings               []string                  `json:"warnings"`
}

func registerBenchStrategyRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-strategies", getOnly(handleBenchStrategies))
	registerBenchStrategyPreflightRoute(mux)
	registerBenchTLSStrategySmokeRoute(mux)
}

func handleBenchStrategies(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readBenchStrategyInventory())
}

func readProductionNFQWSArgv() (int, string, []string, error) {
	data, err := os.ReadFile(benchProductionPIDFile)
	if err != nil {
		return 0, "", nil, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return 0, "", nil, errors.New("invalid production nfqws2 pidfile")
	}

	cmdRaw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return 0, "", nil, err
	}
	parts := strings.Split(string(cmdRaw), "\x00")
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			argv = append(argv, part)
		}
	}
	if len(argv) == 0 || !strings.Contains(strings.ToLower(filepath.Base(argv[0])), "nfqws2") {
		return 0, "", nil, errors.New("production pid does not point to nfqws2")
	}
	executable, _ := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if executable == "" {
		executable = argv[0]
	}
	return pid, executable, argv, nil
}

func isBenchStrategyBaseArg(arg string) bool {
	return strings.HasPrefix(arg, "--lua-init=") || strings.HasPrefix(arg, "--blob=")
}

func splitBenchStrategyArgv(argv []string) ([]string, [][]string) {
	if len(argv) <= 1 {
		return []string{}, [][]string{}
	}
	args := argv[1:]
	i := 0
	for i < len(args) {
		if args[i] == "--daemon" ||
			strings.HasPrefix(args[i], "--pidfile=") ||
			strings.HasPrefix(args[i], "--user=") ||
			strings.HasPrefix(args[i], "--qnum=") ||
			strings.HasPrefix(args[i], "--fwmark=") {
			i++
			continue
		}
		break
	}

	baseArgs := []string{}
	for i < len(args) && isBenchStrategyBaseArg(args[i]) {
		baseArgs = append(baseArgs, args[i])
		i++
	}

	profiles := [][]string{}
	current := []string{}
	for ; i < len(args); i++ {
		if args[i] == "--new" {
			if len(current) > 0 {
				profiles = append(profiles, current)
			}
			current = []string{}
			continue
		}
		current = append(current, args[i])
	}
	if len(current) > 0 {
		profiles = append(profiles, current)
	}
	return baseArgs, profiles
}

func splitCSVValue(arg, prefix string) []string {
	if !strings.HasPrefix(arg, prefix) {
		return nil
	}
	value := strings.TrimSpace(strings.TrimPrefix(arg, prefix))
	if value == "" {
		return nil
	}
	out := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func appendUniqueStrings(dst []string, values ...string) []string {
	seen := map[string]bool{}
	for _, item := range dst {
		seen[item] = true
	}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			dst = append(dst, value)
		}
	}
	return dst
}

func parseBenchStrategyTag(arg string) (int, bool) {
	pos := strings.Index(arg, ":strategy=")
	if pos < 0 {
		return 0, false
	}
	value := arg[pos+len(":strategy="):]
	if cut := strings.IndexByte(value, ':'); cut >= 0 {
		value = value[:cut]
	}
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0
}

func benchPortListContains(portValues []string, target int) bool {
	for _, value := range portValues {
		if n, err := strconv.Atoi(value); err == nil {
			if n == target {
				return true
			}
			continue
		}
		parts := strings.SplitN(value, "-", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(value, ":", 2)
		}
		if len(parts) != 2 {
			continue
		}
		a, errA := strconv.Atoi(parts[0])
		b, errB := strconv.Atoi(parts[1])
		if errA == nil && errB == nil && a <= target && target <= b {
			return true
		}
	}
	return false
}

func analyzeBenchStrategyProfile(index int, args []string) benchStrategyProfile {
	profile := benchStrategyProfile{
		Index:            index,
		Args:             append([]string{}, args...),
		TCPFilters:       []string{},
		UDPFilters:       []string{},
		L7Filters:        []string{},
		Payloads:         []string{},
		HostlistDomains:  []string{},
		FileBoundFilters: []string{},
		StrategyTags:     []int{},
		Reasons:          []string{},
	}
	tagSet := map[int]bool{}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--filter-tcp="):
			profile.TCPFilters = appendUniqueStrings(profile.TCPFilters, splitCSVValue(arg, "--filter-tcp=")...)
		case strings.HasPrefix(arg, "--filter-udp="):
			profile.UDPFilters = appendUniqueStrings(profile.UDPFilters, splitCSVValue(arg, "--filter-udp=")...)
		case strings.HasPrefix(arg, "--filter-l7="):
			profile.L7Filters = appendUniqueStrings(profile.L7Filters, splitCSVValue(arg, "--filter-l7=")...)
		case strings.HasPrefix(arg, "--payload="):
			profile.Payloads = appendUniqueStrings(profile.Payloads, splitCSVValue(arg, "--payload=")...)
		case strings.HasPrefix(arg, "--hostlist-domains="):
			profile.HostlistDomains = appendUniqueStrings(profile.HostlistDomains, splitCSVValue(arg, "--hostlist-domains=")...)
		case strings.HasPrefix(arg, "--hostlist="),
			strings.HasPrefix(arg, "--hostlist-auto="),
			strings.HasPrefix(arg, "--hostlist-exclude="),
			strings.HasPrefix(arg, "--ipset="),
			strings.HasPrefix(arg, "--ipset-exclude="):
			profile.FileBoundFilters = appendUniqueStrings(profile.FileBoundFilters, arg)
		}
		if strings.HasPrefix(arg, "--lua-desync=") {
			profile.DesyncCount++
			if tag, ok := parseBenchStrategyTag(arg); ok {
				tagSet[tag] = true
			}
		}
	}
	for tag := range tagSet {
		profile.StrategyTags = append(profile.StrategyTags, tag)
	}
	sort.Ints(profile.StrategyTags)

	hasTLS := false
	for _, value := range profile.L7Filters {
		if value == "tls" {
			hasTLS = true
			break
		}
	}
	hasClientHello := false
	for _, value := range profile.Payloads {
		if value == "tls_client_hello" {
			hasClientHello = true
			break
		}
	}

	if !benchPortListContains(profile.TCPFilters, 443) {
		profile.Reasons = append(profile.Reasons, "profile does not explicitly filter TCP/443")
	}
	if len(profile.UDPFilters) > 0 {
		profile.Reasons = append(profile.Reasons, "profile includes UDP and is outside first TLS bench scope")
	}
	if !hasTLS {
		profile.Reasons = append(profile.Reasons, "profile does not explicitly filter TLS")
	}
	if !hasClientHello {
		profile.Reasons = append(profile.Reasons, "profile does not explicitly target tls_client_hello")
	}
	if profile.DesyncCount == 0 {
		profile.Reasons = append(profile.Reasons, "profile has no lua-desync actions")
	}
	if len(profile.FileBoundFilters) > 0 {
		profile.Reasons = append(profile.Reasons, "profile depends on hostlist/ipset files")
	}
	if len(profile.StrategyTags) > 1 {
		profile.Reasons = append(profile.Reasons, "profile contains multiple strategy tags")
	}

	profile.CandidateEligible = len(profile.Reasons) == 0
	return profile
}

func benchDependencyPath(arg string) (kind, path string, ok bool) {
	if strings.HasPrefix(arg, "--lua-init=@") {
		return "lua", strings.TrimPrefix(arg, "--lua-init=@"), true
	}
	if strings.HasPrefix(arg, "--blob=") {
		value := strings.TrimPrefix(arg, "--blob=")
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "@") {
			return "", "", false
		}
		return "blob", strings.TrimPrefix(parts[1], "@"), true
	}
	return "", "", false
}

func resolveBenchStrategyDependency(arg string) benchStrategyDependency {
	dep := benchStrategyDependency{Argument: arg}
	kind, path, ok := benchDependencyPath(arg)
	if !ok {
		dep.Resolution = "inline"
		dep.Proven = true
		return dep
	}
	dep.Kind = kind
	dep.Path = path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dep.Proven = true
		dep.Resolved = path
		dep.Resolution = "exact"
		return dep
	}
	if kind == "lua" {
		gz := path + ".gz"
		if info, err := os.Stat(gz); err == nil && !info.IsDir() {
			dep.Proven = true
			dep.Resolved = gz
			dep.Resolution = "gzip-sibling-used-by-live-runtime"
			return dep
		}
	}
	dep.Resolution = "missing"
	return dep
}

func readBenchStrategyInventory() benchStrategyInventory {
	out := benchStrategyInventory{
		ReadOnly:            true,
		CompilerImplemented: true,
		ExecutionEnabled:    false,
		Source:              "live-production-proc-cmdline",
		BaseArgs:            []string{},
		BaseDependencies:    []benchStrategyDependency{},
		Profiles:            []benchStrategyProfile{},
		Warnings:            []string{},
	}
	pid, executable, argv, err := readProductionNFQWSArgv()
	if err != nil {
		out.Warnings = append(out.Warnings, err.Error())
		return out
	}
	out.ProductionPID = pid
	out.ProductionExecutable = executable
	baseArgs, profiles := splitBenchStrategyArgv(argv)
	out.BaseArgs = baseArgs
	out.BaseDependenciesProven = true
	for _, arg := range baseArgs {
		dep := resolveBenchStrategyDependency(arg)
		out.BaseDependencies = append(out.BaseDependencies, dep)
		if !dep.Proven {
			out.BaseDependenciesProven = false
		}
	}
	for i, args := range profiles {
		profile := analyzeBenchStrategyProfile(i, args)
		out.Profiles = append(out.Profiles, profile)
		if profile.CandidateEligible {
			out.EligibleProfileCount++
		}
	}
	out.ProfileCount = len(out.Profiles)
	if !out.BaseDependenciesProven {
		out.Warnings = append(out.Warnings, "one or more live production base dependencies could not be proven")
	}
	if out.EligibleProfileCount == 0 {
		out.Warnings = append(out.Warnings, "no first-scope TCP/TLS candidate profile was found")
	}
	return out
}
