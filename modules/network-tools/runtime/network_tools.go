package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/probe"
)

const (
	targetLimit      = 253
	commandOutputMax = 32 << 10
)

var (
	packetLossRE = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)%\s*packet loss`)
	pingAvgRE    = regexp.MustCompile(`(?i)(?:min/avg/max(?:/mdev)?|round-trip min/avg/max(?:/stddev)?)\s*=\s*[0-9.]+/([0-9.]+)/`)
)

type routeEntry struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Mask        string `json:"mask"`
	Prefix      int    `json:"prefix"`
	Metric      int    `json:"metric"`
	Flags       uint64 `json:"flags"`
	Table       string `json:"table"`
	Type        string `json:"type"`
}

type flowEntry struct {
	Protocol    string `json:"protocol"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	SourcePort  string `json:"source_port,omitempty"`
	DestPort    string `json:"destination_port,omitempty"`
	State       string `json:"state,omitempty"`
	Packets     uint64 `json:"packets,omitempty"`
	Bytes       uint64 `json:"bytes,omitempty"`
	Timeout     int64  `json:"timeout_seconds,omitempty"`
	SeenAt      string `json:"seen_at"`
}

type probeResult struct {
	OK         bool     `json:"ok"`
	Kind       string   `json:"kind"`
	Target     string   `json:"target"`
	Port       int      `json:"port,omitempty"`
	DurationMS int64    `json:"duration_ms"`
	RTTAvgMS   float64  `json:"rtt_avg_ms,omitempty"`
	LossPct    float64  `json:"loss_pct,omitempty"`
	Addresses  []string `json:"addresses,omitempty"`
	StatusCode int      `json:"status_code,omitempty"`
	Output     string   `json:"output,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type traceHop struct {
	Hop     int       `json:"hop"`
	Address string    `json:"address"`
	RTTMS   []float64 `json:"rtt_ms"`
	LossPct float64   `json:"loss_pct"`
	Label   string    `json:"label,omitempty"`
	Raw     string    `json:"raw,omitempty"`
}

func registerNetworkToolsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		routes, _ := readRoutes()
		flows, source := readFlows(512)
		ifaces, _ := net.Interfaces()
		writeJSON(w, http.StatusOK, map[string]any{
			"interfaces":      len(ifaces),
			"routes":          len(routes),
			"active_sessions": len(flows),
			"flow_source":     source,
			"probe_engine":    true,
			"mutation_api":    false,
		})
	}))
	mux.HandleFunc("/v1/interfaces", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"interfaces": interfaceSnapshot()})
	}))
	mux.HandleFunc("/v1/routes", getOnly(func(w http.ResponseWriter, r *http.Request) {
		routes, err := readRoutes()
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
			return
		}
		response := map[string]any{"routes": routes, "table": "main"}
		if target := strings.TrimSpace(r.URL.Query().Get("target")); target != "" {
			ip, resolution := targetIP(r.Context(), target)
			response["resolution"] = resolution
			if ip != nil {
				response["selected"] = selectRoute(routes, ip)
			}
		}
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("/v1/flows", getOnly(func(w http.ResponseWriter, r *http.Request) {
		limit := 256
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 512 {
				limit = n
			}
		}
		flows, source := readFlows(limit)
		writeJSON(w, http.StatusOK, map[string]any{"flows": flows, "source": source, "limit": limit})
	}))
	mux.HandleFunc("/v1/probe", getOnly(func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("target"))
		kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
		port := queryPort(r, defaultPort(kind))
		result := runProbe(r.Context(), kind, target, port)
		status := http.StatusOK
		if result.Error == "invalid target" || result.Error == "unsupported probe kind" || result.Error == "invalid port" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, result)
	}))
	mux.HandleFunc("/v1/traceroute", getOnly(func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("target"))
		if !validTarget(target) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid target"})
			return
		}
		hops, raw, err := traceRoute(r.Context(), target)
		response := map[string]any{"ok": err == nil, "target": target, "hops": hops, "raw": raw}
		if err != nil {
			response["error"] = err.Error()
		}
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("/v1/doctor", getOnly(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, networkDoctor(r))
	}))
}

func interfaceSnapshot() []map[string]any {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		addrStrings := make([]string, 0, len(addresses))
		for _, addr := range addresses {
			addrStrings = append(addrStrings, addr.String())
			if len(addrStrings) >= 16 {
				break
			}
		}
		out = append(out, map[string]any{
			"name":          iface.Name,
			"index":         iface.Index,
			"mtu":           iface.MTU,
			"hardware_addr": iface.HardwareAddr.String(),
			"flags":         iface.Flags.String(),
			"addresses":     addrStrings,
		})
	}
	return out
}

func readRoutes() ([]routeEntry, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	out := make([]routeEntry, 0, 32)
	scanner := bufio.NewScanner(file)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || len(out) >= 256 {
			continue
		}
		dst := routeHexIP(fields[1])
		gw := routeHexIP(fields[2])
		mask := routeHexIP(fields[7])
		if dst == nil || gw == nil || mask == nil {
			continue
		}
		metric, _ := strconv.Atoi(fields[6])
		flags, _ := strconv.ParseUint(fields[3], 16, 64)
		mask4 := net.IPMask(mask.To4())
		ones, _ := mask4.Size()
		out = append(out, routeEntry{
			Interface:   fields[0],
			Destination: dst.String(),
			Gateway:     gw.String(),
			Mask:        mask.String(),
			Prefix:      ones,
			Metric:      metric,
			Flags:       flags,
			Table:       "main",
			Type:        "unicast",
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Prefix != out[j].Prefix {
			return out[i].Prefix > out[j].Prefix
		}
		return out[i].Metric < out[j].Metric
	})
	return out, nil
}

func routeHexIP(raw string) net.IP {
	if len(raw) != 8 {
		return nil
	}
	bytes, err := hex.DecodeString(raw)
	if err != nil || len(bytes) != 4 {
		return nil
	}
	return net.IPv4(bytes[3], bytes[2], bytes[1], bytes[0])
}

func targetIP(ctx context.Context, target string) (net.IP, probe.Result) {
	if ip := net.ParseIP(target); ip != nil {
		result := probe.Result{OK: true, Addresses: []string{ip.String()}}
		return ip.To4(), result
	}
	result := probe.Resolve(ctx, target, 2*time.Second)
	if !result.OK || len(result.Addresses) == 0 {
		return nil, result
	}
	for _, raw := range result.Addresses {
		if ip := net.ParseIP(raw); ip != nil && ip.To4() != nil {
			return ip.To4(), result
		}
	}
	return nil, result
}

func selectRoute(routes []routeEntry, ip net.IP) *routeEntry {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	var best *routeEntry
	for i := range routes {
		route := &routes[i]
		dst := net.ParseIP(route.Destination).To4()
		maskIP := net.ParseIP(route.Mask).To4()
		if dst == nil || maskIP == nil {
			continue
		}
		mask := net.IPMask(maskIP)
		if !ip4.Mask(mask).Equal(dst.Mask(mask)) {
			continue
		}
		if best == nil || route.Prefix > best.Prefix || (route.Prefix == best.Prefix && route.Metric < best.Metric) {
			copy := *route
			best = &copy
		}
	}
	return best
}

func readFlows(limit int) ([]flowEntry, string) {
	paths := []string{"/proc/net/nf_conntrack", "/proc/net/ip_conntrack"}
	if limit <= 0 || limit > 512 {
		limit = 256
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		out := make([]flowEntry, 0, minInt(limit, 64))
		scanner := bufio.NewScanner(file)
		buffer := make([]byte, 4096)
		scanner.Buffer(buffer, 64<<10)
		for scanner.Scan() && len(out) < limit {
			if flow, ok := parseFlow(scanner.Text()); ok {
				out = append(out, flow)
			}
		}
		_ = file.Close()
		return out, path
	}
	return []flowEntry{}, "unavailable"
}

func parseFlow(line string) (flowEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return flowEntry{}, false
	}
	flow := flowEntry{SeenAt: time.Now().Format("15:04:05")}
	timeoutCaptured := false
	for i, field := range fields {
		if flow.Protocol == "" && (field == "tcp" || field == "udp" || field == "icmp" || field == "icmpv6") {
			flow.Protocol = strings.ToUpper(field)
			if i+2 < len(fields) {
				if timeout, err := strconv.ParseInt(fields[i+2], 10, 64); err == nil {
					flow.Timeout = timeout
					timeoutCaptured = true
				}
			}
			continue
		}
		if flow.State == "" && (field == "ESTABLISHED" || field == "TIME_WAIT" || field == "SYN_SENT" || field == "CLOSE" || field == "UNREPLIED") {
			flow.State = field
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "src":
			if flow.Source == "" {
				flow.Source = value
			}
		case "dst":
			if flow.Destination == "" {
				flow.Destination = value
			}
		case "sport":
			if flow.SourcePort == "" {
				flow.SourcePort = value
			}
		case "dport":
			if flow.DestPort == "" {
				flow.DestPort = value
			}
		case "packets":
			if flow.Packets == 0 {
				flow.Packets, _ = strconv.ParseUint(value, 10, 64)
			}
		case "bytes":
			if flow.Bytes == 0 {
				flow.Bytes, _ = strconv.ParseUint(value, 10, 64)
			}
		}
	}
	if !timeoutCaptured {
		flow.Timeout = 0
	}
	if flow.State == "" {
		flow.State = "ACTIVE"
	}
	return flow, flow.Protocol != "" && (flow.Source != "" || flow.Destination != "")
}

func validTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" || len(target) > targetLimit || strings.ContainsAny(target, " \t\r\n/\\\x00") {
		return false
	}
	if net.ParseIP(target) != nil {
		return true
	}
	if strings.HasPrefix(target, ".") || strings.HasSuffix(target, ".") || strings.Contains(target, "..") {
		return false
	}
	for _, label := range strings.Split(target, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func defaultPort(kind string) int {
	switch kind {
	case "http":
		return 80
	case "tcp":
		return 53
	default:
		return 0
	}
}

func queryPort(r *http.Request, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("port"))
	if raw == "" {
		return fallback
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return -1
	}
	return port
}

func runProbe(parent context.Context, kind, target string, port int) probeResult {
	start := time.Now()
	result := probeResult{Kind: kind, Target: target, Port: port}
	if !validTarget(target) {
		result.Error = "invalid target"
		return result
	}
	if (kind == "tcp" || kind == "http") && (port < 1 || port > 65535) {
		result.Error = "invalid port"
		return result
	}

	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()

	switch kind {
	case "dns":
		if ip := net.ParseIP(target); ip != nil {
			result.OK = true
			result.Addresses = []string{ip.String()}
		} else {
			resolved := probe.Resolve(ctx, target, 3*time.Second)
			result.OK = resolved.OK
			result.Addresses = resolved.Addresses
			result.Error = resolved.Error
		}
	case "tcp":
		address := net.JoinHostPort(target, strconv.Itoa(port))
		tcp := probe.TCP(ctx, address, 4*time.Second)
		result.OK = tcp.OK
		result.DurationMS = tcp.LatencyMS
		result.Error = tcp.Error
	case "http":
		result = httpProbe(ctx, target, port)
	case "ping":
		result = pingProbe(ctx, target)
	default:
		result.Error = "unsupported probe kind"
	}
	if result.DurationMS == 0 {
		result.DurationMS = time.Since(start).Milliseconds()
	}
	return result
}

func pingProbe(ctx context.Context, target string) probeResult {
	start := time.Now()
	result := probeResult{Kind: "ping", Target: target}
	binary, err := exec.LookPath("ping")
	if err != nil {
		result.Error = "ping command unavailable"
		return result
	}
	cmd := exec.CommandContext(ctx, binary, "-c", "4", "-W", "1", target)
	output, err := cmd.CombinedOutput()
	if len(output) > commandOutputMax {
		output = output[:commandOutputMax]
	}
	result.Output = string(output)
	result.DurationMS = time.Since(start).Milliseconds()
	result.OK = err == nil

	if match := packetLossRE.FindStringSubmatch(result.Output); len(match) == 2 {
		result.LossPct, _ = strconv.ParseFloat(match[1], 64)
	}
	if match := pingAvgRE.FindStringSubmatch(result.Output); len(match) == 2 {
		result.RTTAvgMS, _ = strconv.ParseFloat(match[1], 64)
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Error = "ping timed out"
		} else {
			result.Error = err.Error()
		}
	}
	return result
}

func httpProbe(ctx context.Context, target string, port int) probeResult {
	start := time.Now()
	result := probeResult{Kind: "http", Target: target, Port: port}
	host := target
	if net.ParseIP(target) != nil {
		host = "[" + target + "]"
		if net.ParseIP(target).To4() != nil {
			host = target
		}
	}
	url := fmt.Sprintf("http://%s:%d/", host, port)
	transport := &http.Transport{
		Proxy:             nil,
		DialContext:       (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	resp, err := client.Do(req)
	result.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	result.OK = resp.StatusCode > 0 && resp.StatusCode < 500
	return result
}

func traceRoute(parent context.Context, target string) ([]traceHop, string, error) {
	ctx, cancel := context.WithTimeout(parent, 14*time.Second)
	defer cancel()

	binary, err := exec.LookPath("traceroute")
	if err != nil {
		return nil, "", errors.New("traceroute command unavailable")
	}
	cmd := exec.CommandContext(ctx, binary, "-m", "12", "-w", "2", target)
	output, runErr := cmd.CombinedOutput()
	if len(output) > commandOutputMax {
		output = output[:commandOutputMax]
	}
	raw := string(output)
	hops := parseTraceroute(raw)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return hops, raw, errors.New("traceroute timed out")
	}
	if runErr != nil && len(hops) == 0 {
		return hops, raw, runErr
	}
	return hops, raw, nil
}

func parseTraceroute(raw string) []traceHop {
	lines := strings.Split(raw, "\n")
	out := make([]traceHop, 0, 12)
	msRE := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ms`)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hopNumber, err := strconv.Atoi(fields[0])
		if err != nil || hopNumber < 1 {
			continue
		}
		hop := traceHop{Hop: hopNumber, Raw: strings.TrimSpace(line)}
		for _, field := range fields[1:] {
			candidate := strings.Trim(field, "()")
			if ip := net.ParseIP(candidate); ip != nil {
				hop.Address = ip.String()
				break
			}
		}
		if hop.Address == "" && fields[1] != "*" {
			hop.Label = strings.Trim(fields[1], "()")
		}
		for _, match := range msRE.FindAllStringSubmatch(line, -1) {
			if len(match) == 2 {
				if value, err := strconv.ParseFloat(match[1], 64); err == nil {
					hop.RTTMS = append(hop.RTTMS, value)
				}
			}
		}
		stars := strings.Count(line, "*")
		if stars >= 3 {
			hop.LossPct = 100
		} else if stars > 0 {
			hop.LossPct = float64(stars) / 3.0 * 100
		}
		out = append(out, hop)
	}
	return out
}

func networkDoctor(r *http.Request) map[string]any {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = "8.8.8.8"
	}
	if !validTarget(target) {
		return map[string]any{"ok": false, "first_failure": "target-validation", "error": "invalid target"}
	}

	rawChecks := strings.TrimSpace(r.URL.Query().Get("checks"))
	if rawChecks == "" {
		rawChecks = "ping,dns,tcp"
	}
	checks := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, raw := range strings.Split(rawChecks, ",") {
		kind := strings.ToLower(strings.TrimSpace(raw))
		if kind == "" || seen[kind] {
			continue
		}
		if kind != "ping" && kind != "dns" && kind != "http" && kind != "tcp" {
			return map[string]any{"ok": false, "first_failure": "check-validation", "error": "unsupported check " + kind}
		}
		seen[kind] = true
		checks = append(checks, kind)
	}
	if len(checks) == 0 {
		return map[string]any{"ok": false, "first_failure": "check-validation", "error": "no checks selected"}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()

	ip, resolution := targetIP(ctx, target)
	routes, _ := readRoutes()
	var selected *routeEntry
	if ip != nil {
		selected = selectRoute(routes, ip)
	}

	probes := map[string]probeResult{}
	allOK := true
	firstFailure := ""
	for _, kind := range checks {
		port := defaultPort(kind)
		result := runProbe(ctx, kind, target, port)
		probes[kind] = result
		if !result.OK {
			allOK = false
			if firstFailure == "" {
				firstFailure = kind
			}
		}
	}
	if selected == nil && ip != nil {
		allOK = false
		if firstFailure == "" {
			firstFailure = "route"
		}
	}

	return map[string]any{
		"ok":            allOK,
		"target":        target,
		"checks":        checks,
		"resolution":    resolution,
		"route":         selected,
		"probes":        probes,
		"first_failure": firstFailure,
		"mutation_api":  false,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
