package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	benchTransportHTTPS = "https"
	benchTransportHTTP  = "http"
	benchTransportQUIC  = "quic"
)

type benchTransportProfile struct {
	ID                  string `json:"id"`
	Network             string `json:"network"`
	RemotePort          int    `json:"remote_port"`
	MetricScope         string `json:"metric_scope"`
	ProbeKind           string `json:"probe_kind"`
	ProbeTimeoutSeconds int    `json:"probe_timeout_seconds"`
	IPv4                bool   `json:"ipv4"`
	Implemented         bool   `json:"implemented"`
}

var benchTransportProfiles = []benchTransportProfile{
	{
		ID:                  benchTransportHTTPS,
		Network:             "tcp",
		RemotePort:          443,
		MetricScope:         "https-full-response",
		ProbeKind:           "tls-http1",
		ProbeTimeoutSeconds: 12,
		IPv4:                true,
		Implemented:         true,
	},
	{
		ID:                  benchTransportHTTP,
		Network:             "tcp",
		RemotePort:          80,
		MetricScope:         "http-full-response",
		ProbeKind:           "http1",
		ProbeTimeoutSeconds: 12,
		IPv4:                true,
		Implemented:         true,
	},
	{
		ID:                  benchTransportQUIC,
		Network:             "udp",
		RemotePort:          443,
		MetricScope:         "quic-initial-response",
		ProbeKind:           "quic-v1-initial",
		ProbeTimeoutSeconds: 8,
		IPv4:                true,
		Implemented:         true,
	},
}

func registerBenchTransportV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/bench-profiles", getOnly(handleV2BenchProfiles))
}

func handleV2BenchProfiles(w http.ResponseWriter, _ *http.Request) {
	profiles := append([]benchTransportProfile{}, benchTransportProfiles...)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"default":  benchTransportHTTPS,
		"profiles": profiles,
		"count":    len(profiles),
	})
}

func normalizeBenchTransport(raw string) (benchTransportProfile, error) {
	id := strings.ToLower(strings.TrimSpace(raw))
	if id == "" {
		id = benchTransportHTTPS
	}
	for _, profile := range benchTransportProfiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return benchTransportProfile{}, errors.New("transport must be https, http or quic")
}

func normalizeBenchTransactionTransport(spec benchTransactionSpec) (string, int, error) {
	network := strings.ToLower(strings.TrimSpace(spec.Network))
	remotePort := spec.RemotePort

	if network == "" {
		network = "tcp"
	}
	if remotePort == 0 {
		remotePort = 443
	}
	if network != "tcp" && network != "udp" {
		return "", 0, errors.New("bench network must be tcp or udp")
	}
	if remotePort < 1 || remotePort > 65535 {
		return "", 0, errors.New("bench remote port must be in range 1-65535")
	}
	return network, remotePort, nil
}

func allocateBenchLocalPortForNetwork(network string) (int, error) {
	switch network {
	case "", "tcp":
		return allocateBenchLocalPort()
	case "udp":
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
		if err != nil {
			return 0, err
		}
		port := conn.LocalAddr().(*net.UDPAddr).Port
		if err := conn.Close(); err != nil {
			return 0, err
		}
		if port < 1024 || port > 65535 {
			return 0, errors.New("allocated UDP local port is outside allowed range")
		}
		return port, nil
	default:
		return 0, errors.New("unsupported bench network")
	}
}

func benchTransportTimeout(profile benchTransportProfile) time.Duration {
	seconds := profile.ProbeTimeoutSeconds
	if seconds <= 0 {
		seconds = int(v2HTTPProbeTimeout / time.Second)
	}
	return time.Duration(seconds) * time.Second
}

func benchProfileHas(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func benchProfileTransportReasons(profile benchStrategyProfile, transport benchTransportProfile) []string {
	reasons := []string{}

	switch transport.ID {
	case benchTransportHTTPS:
		if !benchPortListContains(profile.TCPFilters, 443) {
			reasons = append(reasons, "profile does not explicitly filter TCP/443")
		}
		if len(profile.UDPFilters) > 0 {
			reasons = append(reasons, "profile includes UDP and is outside HTTPS bench scope")
		}
		if !benchProfileHas(profile.L7Filters, "tls") {
			reasons = append(reasons, "profile does not explicitly filter TLS")
		}
		if !benchProfileHas(profile.Payloads, "tls_client_hello") {
			reasons = append(reasons, "profile does not explicitly target tls_client_hello")
		}
	case benchTransportHTTP:
		if !benchPortListContains(profile.TCPFilters, 80) {
			reasons = append(reasons, "profile does not explicitly filter TCP/80")
		}
		if len(profile.UDPFilters) > 0 {
			reasons = append(reasons, "profile includes UDP and is outside HTTP bench scope")
		}
		if !benchProfileHas(profile.L7Filters, "http") {
			reasons = append(reasons, "profile does not explicitly filter HTTP")
		}
		if !benchProfileHas(profile.Payloads, "http_req") {
			reasons = append(reasons, "profile does not explicitly target http_req")
		}
	case benchTransportQUIC:
		if !benchPortListContains(profile.UDPFilters, 443) {
			reasons = append(reasons, "profile does not explicitly filter UDP/443")
		}
		if len(profile.TCPFilters) > 0 {
			reasons = append(reasons, "profile includes TCP and is outside QUIC bench scope")
		}
		if !benchProfileHas(profile.L7Filters, "quic") {
			reasons = append(reasons, "profile does not explicitly filter QUIC")
		}
		if !benchProfileHas(profile.Payloads, "quic_initial") {
			reasons = append(reasons, "profile does not explicitly target quic_initial")
		}
	default:
		reasons = append(reasons, "unsupported bench transport")
	}

	if profile.DesyncCount == 0 {
		reasons = append(reasons, "profile has no lua-desync actions")
	}
	if len(profile.FileBoundFilters) > 0 {
		reasons = append(reasons, "profile depends on hostlist/ipset files")
	}
	if len(profile.StrategyTags) > 1 {
		reasons = append(reasons, "profile contains multiple strategy tags")
	}
	return reasons
}

func prepareBenchStrategyProfileForTransport(profile benchStrategyProfile, transport benchTransportProfile) benchStrategyProfile {
	profile.Reasons = benchProfileTransportReasons(profile, transport)
	profile.CandidateEligible = len(profile.Reasons) == 0
	return profile
}

func findBenchStrategyProfileForTransport(inventory benchStrategyInventory, index int, transport benchTransportProfile) (benchStrategyProfile, error) {
	for _, profile := range inventory.Profiles {
		if profile.Index != index {
			continue
		}
		profile = prepareBenchStrategyProfileForTransport(profile, transport)
		if !profile.CandidateEligible {
			return benchStrategyProfile{}, errors.New("selected profile is not eligible for " + transport.ID + " bench: " + strings.Join(profile.Reasons, "; "))
		}
		return profile, nil
	}
	return benchStrategyProfile{}, errors.New("strategy profile index not found")
}

func retargetBenchStrategyProfileForTransport(profile benchStrategyProfile, serverName string, transport benchTransportProfile) (benchStrategyProfile, error) {
	serverName, err := normalizeBenchServerName(serverName)
	if err != nil {
		return benchStrategyProfile{}, err
	}
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if !profile.CandidateEligible {
		return benchStrategyProfile{}, errors.New("profile is not eligible for " + transport.ID + " bench: " + strings.Join(profile.Reasons, "; "))
	}

	out := profile
	out.Args = make([]string, 0, len(profile.Args)+1)
	out.Args = append(out.Args, "--hostlist-domains="+serverName)
	for _, arg := range profile.Args {
		if strings.HasPrefix(arg, "--hostlist-domains=") {
			continue
		}
		out.Args = append(out.Args, arg)
	}
	out = analyzeBenchStrategyProfile(out.Index, out.Args)
	out = prepareBenchStrategyProfileForTransport(out, transport)
	if !out.CandidateEligible {
		return benchStrategyProfile{}, errors.New("retargeted profile is not eligible for " + transport.ID + " bench: " + strings.Join(out.Reasons, "; "))
	}
	return out, nil
}

func v2CustomProfileForTransport(args []string, target string, transport benchTransportProfile) (benchStrategyProfile, error) {
	normalizedTarget, err := v2NormalizeTarget(target)
	if err != nil {
		return benchStrategyProfile{}, err
	}
	target = normalizedTarget

	if len(args) == 0 || len(args) > 256 {
		return benchStrategyProfile{}, errors.New("custom candidate args are empty or too large")
	}

	compiled := make([]string, 0, len(args)+1)
	compiled = append(compiled, "--hostlist-domains="+target)

	for _, arg := range args {
		switch {
		case arg == "--new",
			strings.HasPrefix(arg, "--daemon"),
			strings.HasPrefix(arg, "--pidfile"),
			strings.HasPrefix(arg, "--user"),
			strings.HasPrefix(arg, "--qnum"),
			strings.HasPrefix(arg, "--fwmark"):
			return benchStrategyProfile{}, errors.New("custom candidate contains reserved runtime argument: " + arg)
		case strings.HasPrefix(arg, "--hostlist-domains="),
			strings.HasPrefix(arg, "--hostlist="),
			strings.HasPrefix(arg, "--hostlist-auto="),
			strings.HasPrefix(arg, "--hostlist-exclude="),
			strings.HasPrefix(arg, "--ipset="),
			strings.HasPrefix(arg, "--ipset-exclude="):
			continue
		default:
			compiled = append(compiled, arg)
		}
	}

	profile := analyzeBenchStrategyProfile(0, compiled)
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if !profile.CandidateEligible {
		return benchStrategyProfile{}, errors.New("custom candidate is not eligible for " + transport.ID + " bench: " + strings.Join(profile.Reasons, "; "))
	}
	return retargetBenchStrategyProfileForTransport(profile, target, transport)
}

func buildBenchStrategyCandidateArgsForTransport(spec benchTransactionSpec, inventory benchStrategyInventory, profile benchStrategyProfile, transport benchTransportProfile) ([]string, error) {
	if !inventory.BaseDependenciesProven {
		return nil, errors.New("live strategy base dependencies are not proven")
	}
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if !profile.CandidateEligible {
		return nil, errors.New("strategy profile is not eligible for " + transport.ID + " bench: " + strings.Join(profile.Reasons, "; "))
	}

	args := append([]string{}, benchCandidateArgs(spec)...)
	args = append(args, inventory.BaseArgs...)
	args = append(args, profile.Args...)
	return args, nil
}

func v2ReadPlainHTTP(ctx context.Context, host, ip string, localPort, remotePort int) (v2HTTPMetrics, error) {
	metrics := v2HTTPMetrics{}
	start := time.Now()

	dialer := net.Dialer{Timeout: 5 * time.Second}
	if localPort > 0 {
		dialer.LocalAddr = &net.TCPAddr{IP: net.IPv4zero, Port: localPort}
	}

	conn, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ip, strconv.Itoa(remotePort)))
	if err != nil {
		return metrics, fmt.Errorf("tcp connect: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(v2HTTPProbeTimeout))

	request := "GET / HTTP/1.1\r\nHost: " + host + "\r\nUser-Agent: RouterForge-NFQWS-V2/1\r\nAccept: */*\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		return metrics, fmt.Errorf("http write: %w", err)
	}

	reader := bufio.NewReader(conn)
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

func v2ProbeTransport(ctx context.Context, transport benchTransportProfile, host, ip string, localPort int) (v2HTTPMetrics, error) {
	switch transport.ID {
	case benchTransportHTTPS:
		return v2ReadHTTPS(ctx, host, ip, localPort)
	case benchTransportHTTP:
		return v2ReadPlainHTTP(ctx, host, ip, localPort, transport.RemotePort)
	case benchTransportQUIC:
		return v2ProbeQUIC(ctx, host, ip, localPort, transport.RemotePort)
	default:
		return v2HTTPMetrics{}, errors.New("unsupported bench transport")
	}
}

func v2TransportAttemptSucceeded(transport benchTransportProfile, metrics v2HTTPMetrics) bool {
	switch transport.ID {
	case benchTransportHTTPS:
		return metrics.TLSComplete && metrics.ProgressProven
	case benchTransportHTTP:
		return metrics.ProgressProven
	case benchTransportQUIC:
		return metrics.QUICResponseProven && metrics.QUICCIDMatched
	default:
		return false
	}
}

func v2TransportResultClass(attempt v2BenchAttempt, transport benchTransportProfile) string {
	if attempt.OK && attempt.Metrics.ResponseComplete {
		return "WORKING"
	}
	if attempt.OK && attempt.Metrics.ProgressProven {
		return "WORKING"
	}
	if attempt.Metrics.Cutoff16KSuspected {
		return "PARTIAL"
	}
	if transport.ID == benchTransportHTTPS && attempt.CleanupProven && attempt.Metrics.TLSComplete {
		return "INCONCLUSIVE"
	}
	return "FAILED"
}
