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
	"github.com/Fifth-Ace/routerforge/internal/safety"
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
		routes, _ := readRoutes()
		writeJSON(w, http.StatusOK, map[string]any{
			"interfaces":    interfaceSnapshot(),
			"default_route": selectDefaultRoute(routes),
			"source":        "kernel+sysfs",
			"mutation_api":  false,
		})
	}))
	mux.HandleFunc("/v1/routes", getOnly(func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("target"))
		if target != "" && !validTarget(target) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid target"})
			return
		}
		writeJSON(w, http.StatusOK, routeInspector(r.Context(), target))
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

type doctorStage struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

type doctorVerdict struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	FaultDomain string `json:"fault_domain,omitempty"`
}

type doctorAction struct {
	ID          string `json:"id"`
	Priority    string `json:"priority"`
	FaultDomain string `json:"fault_domain,omitempty"`
	Detail      string `json:"detail"`
}

func readSysfsText(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readSysfsUint(path string) uint64 {
	raw := readSysfsText(path)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func readSysfsInt(path string) int {
	raw := readSysfsText(path)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func selectDefaultRoute(routes []routeEntry) *routeEntry {
	var best *routeEntry
	for i := range routes {
		route := &routes[i]
		if route.Prefix != 0 || route.Destination != "0.0.0.0" {
			continue
		}
		if best == nil || route.Metric < best.Metric {
			copy := *route
			best = &copy
		}
	}
	return best
}

func interfaceAddresses(iface *net.Interface) []string {
	if iface == nil {
		return []string{}
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		out = append(out, addr.String())
		if len(out) >= 16 {
			break
		}
	}
	return out
}

func primaryInterfaceAddress(addresses []string) string {
	for _, raw := range addresses {
		ip, _, err := net.ParseCIDR(raw)
		if err != nil || ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		return raw
	}
	if len(addresses) > 0 {
		return addresses[0]
	}
	return ""
}

func interfaceSnapshot() []map[string]any {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []map[string]any{}
	}

	routes, _ := readRoutes()
	defaultRoute := selectDefaultRoute(routes)
	out := make([]map[string]any, 0, len(interfaces))

	for _, iface := range interfaces {
		name := iface.Name
		base := "/sys/class/net/" + name
		addresses := interfaceAddresses(&iface)
		operstate := readSysfsText(base + "/operstate")
		carrierRaw := readSysfsText(base + "/carrier")
		speed := readSysfsInt(base + "/speed")
		if speed < 0 {
			speed = 0
		}

		item := map[string]any{
			"name":          name,
			"index":         iface.Index,
			"mtu":           iface.MTU,
			"hardware_addr": iface.HardwareAddr.String(),
			"flags":         iface.Flags.String(),
			"addresses":     addresses,
			"alias":         readSysfsText(base + "/ifalias"),
			"operstate":     operstate,
			"carrier":       carrierRaw == "1",
			"speed_mbps":    speed,
			"duplex":        readSysfsText(base + "/duplex"),
			"rx_bytes":      readSysfsUint(base + "/statistics/rx_bytes"),
			"tx_bytes":      readSysfsUint(base + "/statistics/tx_bytes"),
			"rx_packets":    readSysfsUint(base + "/statistics/rx_packets"),
			"tx_packets":    readSysfsUint(base + "/statistics/tx_packets"),
			"rx_errors":     readSysfsUint(base + "/statistics/rx_errors"),
			"tx_errors":     readSysfsUint(base + "/statistics/tx_errors"),
			"rx_dropped":    readSysfsUint(base + "/statistics/rx_dropped"),
			"tx_dropped":    readSysfsUint(base + "/statistics/tx_dropped"),
			"default_route": false,
		}

		if defaultRoute != nil && defaultRoute.Interface == name {
			item["default_route"] = true
			item["default_gateway"] = defaultRoute.Gateway
			item["default_metric"] = defaultRoute.Metric
		}

		out = append(out, item)
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
	output, err := safety.RunCommand(ctx, commandOutputMax, binary, "-c", "4", "-W", "1", target)
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
	output, runErr := safety.RunCommand(ctx, commandOutputMax, binary, "-m", "12", "-w", "2", target)
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

func doctorRouteStages(decision kernelRouteDecision, policyState string, targetResolved bool) (doctorStage, doctorStage) {
	routeStage := doctorStage{ID: "target_route"}
	policyStage := doctorStage{ID: "policy_routing"}

	if !targetResolved {
		routeStage.Status = "skipped"
		routeStage.Detail = "target address unavailable"
	} else if !decision.Available {
		routeStage.Status = "fail"
		routeStage.Detail = "kernel route decision unavailable"
		if decision.Error != "" {
			routeStage.Detail = decision.Error
		}
	} else {
		switch decision.Type {
		case "blackhole", "unreachable", "prohibit", "throw":
			routeStage.Status = "fail"
			routeStage.Detail = fmt.Sprintf("%s route selected for %s", decision.Type, decision.Destination)
		default:
			routeStage.Status = "ok"
			parts := []string{}
			if decision.Table != "" {
				parts = append(parts, "table "+decision.Table)
			}
			if decision.Gateway != "" {
				parts = append(parts, "via "+decision.Gateway)
			}
			if decision.Interface != "" {
				parts = append(parts, "dev "+decision.Interface)
			}
			if decision.Source != "" {
				parts = append(parts, "src "+decision.Source)
			}
			routeStage.Detail = strings.Join(parts, " ")
			if routeStage.Detail == "" {
				routeStage.Detail = decision.Raw
			}
		}
	}

	switch policyState {
	case "active":
		policyStage.Status = "ok"
		policyStage.Detail = "policy routing active"
		if decision.Table != "" {
			policyStage.Detail += "; kernel selected table " + decision.Table
		}
	case "default-only":
		policyStage.Status = "ok"
		policyStage.Detail = "default policy rules only"
		if decision.Table != "" {
			policyStage.Detail += "; kernel selected table " + decision.Table
		}
	default:
		policyStage.Status = "unavailable"
		policyStage.Detail = "policy routing state unavailable"
	}
	return routeStage, policyStage
}

type doctorPathFacts struct {
	EgressExists  bool
	EgressUp      bool
	SourceChecked bool
	SourceMatches bool
}

func sourceBelongsToInterface(addresses []net.Addr, source string) bool {
	ip := net.ParseIP(strings.TrimSpace(source))
	if ip == nil {
		return false
	}
	for _, address := range addresses {
		var candidate net.IP
		switch value := address.(type) {
		case *net.IPNet:
			candidate = value.IP
		case *net.IPAddr:
			candidate = value.IP
		default:
			host := strings.SplitN(address.String(), "/", 2)[0]
			candidate = net.ParseIP(host)
		}
		if candidate != nil && candidate.Equal(ip) {
			return true
		}
	}
	return false
}

func inspectDoctorPathFacts(decision kernelRouteDecision) doctorPathFacts {
	facts := doctorPathFacts{}
	if !decision.Available || decision.Interface == "" {
		return facts
	}
	iface, err := net.InterfaceByName(decision.Interface)
	if err != nil {
		return facts
	}
	facts.EgressExists = true
	operstate := readSysfsText("/sys/class/net/" + iface.Name + "/operstate")
	facts.EgressUp = iface.Flags&net.FlagUp != 0
	if operstate != "" && operstate != "up" && operstate != "unknown" {
		facts.EgressUp = false
	}
	if decision.Source == "" {
		return facts
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return facts
	}
	facts.SourceChecked = true
	facts.SourceMatches = sourceBelongsToInterface(addresses, decision.Source)
	return facts
}

func doctorPathStages(decision kernelRouteDecision, defaultRoute *routeEntry, facts doctorPathFacts) (doctorStage, doctorStage, doctorStage) {
	egress := doctorStage{ID: "kernel_egress"}
	gateway := doctorStage{ID: "gateway_consistency"}
	source := doctorStage{ID: "source_consistency"}

	if !decision.Available {
		egress.Status = "skipped"
		egress.Detail = "kernel route decision unavailable"
		gateway.Status = "skipped"
		gateway.Detail = "kernel route decision unavailable"
		source.Status = "skipped"
		source.Detail = "kernel route decision unavailable"
		return egress, gateway, source
	}

	switch decision.Type {
	case "blackhole", "unreachable", "prohibit", "throw":
		egress.Status = "skipped"
		egress.Detail = decision.Type + " route has no usable egress"
		gateway.Status = "skipped"
		gateway.Detail = "blocking route selected"
		source.Status = "skipped"
		source.Detail = "blocking route selected"
		return egress, gateway, source
	}

	if decision.Interface == "" {
		egress.Status = "fail"
		egress.Detail = "kernel decision has no egress interface"
	} else if !facts.EgressExists {
		egress.Status = "fail"
		egress.Detail = "kernel-selected interface " + decision.Interface + " does not exist"
	} else if !facts.EgressUp {
		egress.Status = "fail"
		egress.Detail = "kernel-selected interface " + decision.Interface + " is not up"
	} else {
		egress.Status = "ok"
		egress.Detail = "kernel selected dev " + decision.Interface
		if decision.Table != "" {
			egress.Detail += " table " + decision.Table
		}
	}

	if decision.Gateway == "" {
		gateway.Status = "ok"
		gateway.Detail = "direct/on-link route via " + decision.Interface
	} else if decision.Interface == "" {
		gateway.Status = "fail"
		gateway.Detail = "gateway " + decision.Gateway + " selected without an egress interface"
	} else {
		gateway.Status = "ok"
		gateway.Detail = "via " + decision.Gateway + " dev " + decision.Interface
		if defaultRoute != nil && (decision.Table == "" || decision.Table == "main") {
			if defaultRoute.Interface == decision.Interface && defaultRoute.Gateway == decision.Gateway {
				gateway.Detail += "; matches IPv4 default path"
			} else {
				gateway.Detail += fmt.Sprintf(
					"; target path differs from IPv4 default via %s dev %s",
					defaultRoute.Gateway,
					defaultRoute.Interface,
				)
			}
		} else if decision.Table != "" && decision.Table != "main" {
			gateway.Detail += "; policy table " + decision.Table + " is authoritative"
		}
	}

	if decision.Source == "" {
		source.Status = "unavailable"
		source.Detail = "kernel did not expose a source address"
	} else if !facts.EgressExists {
		source.Status = "skipped"
		source.Detail = "egress interface unavailable"
	} else if !facts.SourceChecked {
		source.Status = "unavailable"
		source.Detail = "could not enumerate addresses on " + decision.Interface
	} else if !facts.SourceMatches {
		source.Status = "fail"
		source.Detail = "kernel source " + decision.Source + " is not assigned to " + decision.Interface
	} else {
		source.Status = "ok"
		source.Detail = "src " + decision.Source + " belongs to " + decision.Interface
	}

	return egress, gateway, source
}

func doctorStageIndex(stages []doctorStage, id string) int {
	for i := range stages {
		if stages[i].ID == id {
			return i
		}
	}
	return -1
}

func doctorVerdictFor(stages []doctorStage) doctorVerdict {
	priority := []struct {
		id     string
		code   string
		domain string
	}{
		{"default_route", "no_default_route", "routing"},
		{"interface", "interface_down", "local"},
		{"local_address", "no_local_address", "local"},
		{"target_route", "target_route_failure", "routing"},
		{"kernel_egress", "egress_interface_failure", "local"},
		{"gateway_consistency", "gateway_interface_mismatch", "routing"},
		{"source_consistency", "route_source_mismatch", "local"},
		{"internet", "internet_unreachable", "upstream"},
		{"dns", "dns_failure", "dns"},
		{"tcp", "tcp_failure", "service"},
		{"http", "http_failure", "service"},
		{"ping", "target_unreachable", "target"},
	}

	for _, candidate := range priority {
		if i := doctorStageIndex(stages, candidate.id); i >= 0 && stages[i].Status == "fail" {
			return doctorVerdict{
				Code:        candidate.code,
				Severity:    "fail",
				FaultDomain: candidate.domain,
			}
		}
	}

	for _, stage := range stages {
		if stage.Status == "warn" {
			return doctorVerdict{
				Code:        "degraded",
				Severity:    "warn",
				FaultDomain: "mixed",
			}
		}
	}

	return doctorVerdict{Code: "healthy", Severity: "ok"}
}

func doctorActionsFor(verdict doctorVerdict) []doctorAction {
	action := func(id, priority, domain, detail string) doctorAction {
		return doctorAction{ID: id, Priority: priority, FaultDomain: domain, Detail: detail}
	}
	switch verdict.Code {
	case "healthy":
		return []doctorAction{}
	case "degraded":
		return []doctorAction{
			action("review-warnings", "medium", "mixed", "Review warning stages before changing configuration."),
		}
	case "no_default_route":
		return []doctorAction{
			action("inspect-default-route", "high", "routing", "Inspect default routes and their metrics."),
			action("inspect-policy-rules", "medium", "routing", "Check whether policy rules intentionally bypass the main table."),
		}
	case "interface_down":
		return []doctorAction{
			action("inspect-default-interface", "high", "local", "Check link state and carrier on the default-route interface."),
		}
	case "no_local_address":
		return []doctorAction{
			action("inspect-interface-address", "high", "local", "Check address assignment on the default-route interface."),
		}
	case "egress_interface_failure":
		return []doctorAction{
			action("inspect-kernel-egress", "high", "local", "Check the interface selected by the kernel route decision."),
			action("inspect-policy-rules", "medium", "routing", "Verify that policy routing selects an existing interface."),
		}
	case "gateway_interface_mismatch":
		return []doctorAction{
			action("inspect-target-route", "high", "routing", "Inspect the target-specific route, gateway, and selected interface."),
			action("inspect-policy-rules", "medium", "routing", "Verify the policy table that selected this path."),
		}
	case "route_source_mismatch":
		return []doctorAction{
			action("inspect-source-address", "high", "local", "Verify that the kernel-selected source address is assigned to the selected egress."),
			action("inspect-policy-rules", "medium", "routing", "Check source-based routing and policy rules."),
		}
	case "internet_unreachable":
		return []doctorAction{
			action("verify-upstream", "high", "upstream", "Verify gateway reachability and upstream connectivity."),
		}
	case "dns_failure":
		return []doctorAction{
			action("inspect-dns", "high", "dns", "Verify resolver availability and DNS policy for this target."),
		}
	case "target_route_failure":
		return []doctorAction{
			action("inspect-target-route", "high", "routing", "Inspect the kernel route decision and blocking route types for this target."),
			action("inspect-policy-rules", "medium", "routing", "Check policy rules and the selected routing table."),
		}
	case "tcp_failure", "http_failure":
		return []doctorAction{
			action("verify-service", "medium", "service", "Verify the target service, listening port, and remote filtering."),
		}
	case "target_unreachable":
		return []doctorAction{
			action("verify-target-reachability", "medium", "target", "Retry with a transport check because ICMP may be filtered."),
		}
	default:
		return []doctorAction{
			action("review-diagnosis", "medium", verdict.FaultDomain, "Review failed and warning stages before changing configuration."),
		}
	}
}

func doctorPort(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return -1
	}
	return port
}

func networkDoctor(r *http.Request) map[string]any {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = "example.com"
	}
	if !validTarget(target) {
		return map[string]any{
			"ok":            false,
			"first_failure": "target-validation",
			"error":         "invalid target",
		}
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
			return map[string]any{
				"ok":            false,
				"first_failure": "check-validation",
				"error":         "unsupported check " + kind,
			}
		}
		seen[kind] = true
		checks = append(checks, kind)
	}
	if len(checks) == 0 {
		return map[string]any{
			"ok":            false,
			"first_failure": "check-validation",
			"error":         "no checks selected",
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 22*time.Second)
	defer cancel()

	stages := make([]doctorStage, 0, 10)
	routes, routeErr := readRoutes()
	defaultRoute := selectDefaultRoute(routes)

	switch {
	case routeErr != nil:
		stages = append(stages, doctorStage{
			ID:     "default_route",
			Status: "unavailable",
			Detail: routeErr.Error(),
		})
	case defaultRoute == nil:
		stages = append(stages, doctorStage{
			ID:     "default_route",
			Status: "fail",
			Detail: "no IPv4 default route",
		})
	default:
		stages = append(stages, doctorStage{
			ID:     "default_route",
			Status: "ok",
			Detail: fmt.Sprintf(
				"via %s dev %s metric %d",
				defaultRoute.Gateway,
				defaultRoute.Interface,
				defaultRoute.Metric,
			),
		})
	}

	var defaultInterface map[string]any
	if defaultRoute != nil {
		if iface, err := net.InterfaceByName(defaultRoute.Interface); err != nil {
			stages = append(stages, doctorStage{
				ID:     "interface",
				Status: "fail",
				Detail: err.Error(),
			})
		} else {
			operstate := readSysfsText("/sys/class/net/" + iface.Name + "/operstate")
			up := iface.Flags&net.FlagUp != 0
			if operstate != "" && operstate != "up" && operstate != "unknown" {
				up = false
			}

			status := "ok"
			if !up {
				status = "fail"
			}
			stages = append(stages, doctorStage{
				ID:     "interface",
				Status: status,
				Detail: fmt.Sprintf("%s (%s)", iface.Name, operstate),
			})

			addresses := interfaceAddresses(iface)
			primary := primaryInterfaceAddress(addresses)
			addressStatus := "ok"
			if primary == "" {
				addressStatus = "fail"
				primary = "no usable address"
			}
			stages = append(stages, doctorStage{
				ID:     "local_address",
				Status: addressStatus,
				Detail: primary,
			})

			defaultInterface = map[string]any{
				"name":            iface.Name,
				"flags":           iface.Flags.String(),
				"operstate":       operstate,
				"addresses":       addresses,
				"primary_address": primary,
				"mtu":             iface.MTU,
			}
		}
	} else {
		stages = append(stages,
			doctorStage{
				ID:     "interface",
				Status: "skipped",
				Detail: "default route unavailable",
			},
			doctorStage{
				ID:     "local_address",
				Status: "skipped",
				Detail: "default interface unavailable",
			},
		)
	}

	gatewayResult := probeResult{}
	if defaultRoute != nil && defaultRoute.Gateway != "" && defaultRoute.Gateway != "0.0.0.0" {
		gatewayResult = pingProbe(ctx, defaultRoute.Gateway)
		status := "ok"
		if !gatewayResult.OK {
			status = "fail"
		}
		detail := defaultRoute.Gateway
		if gatewayResult.RTTAvgMS > 0 {
			detail = fmt.Sprintf(
				"%s %.1f ms",
				defaultRoute.Gateway,
				gatewayResult.RTTAvgMS,
			)
		}
		if gatewayResult.Error != "" {
			detail += " - " + gatewayResult.Error
		}
		stages = append(stages, doctorStage{
			ID:         "gateway",
			Status:     status,
			Detail:     detail,
			DurationMS: gatewayResult.DurationMS,
		})
	} else {
		stages = append(stages, doctorStage{
			ID:     "gateway",
			Status: "skipped",
			Detail: "direct default route or no gateway",
		})
	}

	internetPing := pingProbe(ctx, "1.1.1.1")
	internetTCP := probeResult{}
	internetOK := internetPing.OK
	internetDetail := "1.1.1.1"
	internetDuration := internetPing.DurationMS

	if internetPing.RTTAvgMS > 0 {
		internetDetail = fmt.Sprintf(
			"1.1.1.1 %.1f ms ICMP",
			internetPing.RTTAvgMS,
		)
	}

	if !internetOK {
		internetTCP = runProbe(ctx, "tcp", "1.1.1.1", 443)
		internetDuration = internetTCP.DurationMS
		if internetTCP.OK {
			internetOK = true
			internetDetail = fmt.Sprintf(
				"1.1.1.1:443 reachable in %d ms; ICMP unavailable",
				internetTCP.DurationMS,
			)
		} else {
			internetDetail = "1.1.1.1 unreachable by ICMP and TCP/443"
		}
	}

	internetStatus := "ok"
	if !internetOK {
		internetStatus = "fail"
	}
	stages = append(stages, doctorStage{
		ID:         "internet",
		Status:     internetStatus,
		Detail:     internetDetail,
		DurationMS: internetDuration,
	})

	_, resolution := targetIP(ctx, target)
	if net.ParseIP(target) == nil {
		status := "ok"
		detail := strings.Join(resolution.Addresses, ", ")
		if !resolution.OK || len(resolution.Addresses) == 0 {
			status = "fail"
			detail = resolution.Error
			if detail == "" {
				detail = "no addresses returned"
			}
		}
		stages = append(stages, doctorStage{
			ID:     "dns",
			Status: status,
			Detail: detail,
		})
	} else {
		stages = append(stages, doctorStage{
			ID:     "dns",
			Status: "skipped",
			Detail: "target is already an IP address",
		})
	}

	routeReport := routeInspector(ctx, target)
	decision, _ := routeReport["kernel_decision"].(kernelRouteDecision)
	policyState, _ := routeReport["policy_state"].(string)
	targetAddress, _ := routeReport["target_address"].(string)

	var selected *routeEntry
	if legacySelected, ok := routeReport["selected"].(*routeEntry); ok {
		selected = legacySelected
	}

	targetRouteStage, policyRouteStage := doctorRouteStages(decision, policyState, targetAddress != "")
	pathFacts := inspectDoctorPathFacts(decision)
	egressStage, gatewayConsistencyStage, sourceConsistencyStage := doctorPathStages(decision, defaultRoute, pathFacts)
	stages = append(stages, targetRouteStage, policyRouteStage, egressStage, gatewayConsistencyStage, sourceConsistencyStage)

	probes := map[string]probeResult{}
	anyServiceOK := false

	for _, kind := range checks {
		port := 0
		switch kind {
		case "tcp":
			port = doctorPort(r, "tcp_port", 443)
		case "http":
			port = doctorPort(r, "http_port", 80)
		}

		result := runProbe(ctx, kind, target, port)
		probes[kind] = result

		if result.OK && (kind == "tcp" || kind == "http") {
			anyServiceOK = true
		}

		if kind == "dns" {
			continue
		}

		status := "ok"
		if !result.OK {
			status = "fail"
		}

		detail := fmt.Sprintf("%d ms", result.DurationMS)
		if kind == "ping" && result.RTTAvgMS > 0 {
			detail = fmt.Sprintf(
				"%.1f ms avg, %.1f%% loss",
				result.RTTAvgMS,
				result.LossPct,
			)
		}
		if kind == "http" && result.StatusCode > 0 {
			detail = fmt.Sprintf(
				"HTTP %d in %d ms",
				result.StatusCode,
				result.DurationMS,
			)
		}
		if result.Error != "" {
			detail = result.Error
		}

		stages = append(stages, doctorStage{
			ID:         kind,
			Status:     status,
			Detail:     detail,
			DurationMS: result.DurationMS,
		})
	}

	if internetOK || anyServiceOK {
		if i := doctorStageIndex(stages, "gateway"); i >= 0 && stages[i].Status == "fail" {
			stages[i].Status = "warn"
			stages[i].Detail += "; upstream still reachable"
		}
	}

	if anyServiceOK {
		if i := doctorStageIndex(stages, "internet"); i >= 0 && stages[i].Status == "fail" {
			stages[i].Status = "warn"
			stages[i].Detail += "; selected target service is reachable"
		}
		if i := doctorStageIndex(stages, "ping"); i >= 0 && stages[i].Status == "fail" {
			stages[i].Status = "warn"
			stages[i].Detail += "; TCP/HTTP target service is reachable"
		}
	}

	verdict := doctorVerdictFor(stages)
	actions := doctorActionsFor(verdict)
	firstFailure := ""
	for _, stage := range stages {
		if stage.Status == "fail" {
			firstFailure = stage.ID
			break
		}
	}

	return map[string]any{
		"ok":             verdict.Severity != "fail",
		"target":         target,
		"checks":         checks,
		"resolution":     resolution,
		"route":          selected,
		"route_decision": decision,
		"policy_state":   policyState,
		"path_explainability": map[string]any{
			"egress_interface":      decision.Interface,
			"gateway":               decision.Gateway,
			"source":                decision.Source,
			"table":                 decision.Table,
			"egress_exists":         pathFacts.EgressExists,
			"egress_up":             pathFacts.EgressUp,
			"source_checked":        pathFacts.SourceChecked,
			"source_matches_egress": pathFacts.SourceMatches,
		},
		"default_route":         defaultRoute,
		"default_interface":     defaultInterface,
		"probes":                probes,
		"gateway_probe":         gatewayResult,
		"internet_probe":        internetPing,
		"internet_tcp_fallback": internetTCP,
		"diagnosis": map[string]any{
			"stages":  stages,
			"verdict": verdict,
			"actions": actions,
		},
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
