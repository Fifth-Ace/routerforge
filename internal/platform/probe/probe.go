package probe

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

type Result struct {
	Kind       string   `json:"kind"`
	Target     string   `json:"target"`
	OK         bool     `json:"ok"`
	LatencyMS  int64    `json:"latency_ms"`
	ErrorClass string   `json:"error_class,omitempty"`
	Error      string   `json:"error,omitempty"`
	Addresses  []string `json:"addresses,omitempty"`
}

func Resolve(ctx context.Context, target string, timeout time.Duration) Result {
	result := Result{Kind: "dns", Target: target}
	target = strings.TrimSpace(target)
	if target == "" || len(target) > 253 {
		result.ErrorClass = "invalid-target"
		result.Error = "target is empty or too long"
		return result
	}
	if ip := net.ParseIP(target); ip != nil {
		result.OK = true
		result.Addresses = []string{ip.String()}
		return result
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	addrs, err := net.DefaultResolver.LookupIPAddr(probeCtx, target)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.ErrorClass = classify(err)
		result.Error = boundedError(err)
		return result
	}
	for _, addr := range addrs {
		result.Addresses = append(result.Addresses, addr.IP.String())
		if len(result.Addresses) >= 16 {
			break
		}
	}
	result.OK = len(result.Addresses) > 0
	if !result.OK {
		result.ErrorClass = "empty-answer"
	}
	return result
}

func TCP(ctx context.Context, address string, timeout time.Duration) Result {
	result := Result{Kind: "tcp", Target: address}
	address = strings.TrimSpace(address)
	if address == "" || len(address) > 320 {
		result.ErrorClass = "invalid-target"
		result.Error = "address is empty or too long"
		return result
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	var dialer net.Dialer
	conn, err := dialer.DialContext(probeCtx, "tcp", address)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.ErrorClass = classify(err)
		result.Error = boundedError(err)
		return result
	}
	_ = conn.Close()
	result.OK = true
	return result
}

func classify(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "refused"):
		return "refused"
	case strings.Contains(lower, "no such host"):
		return "dns-not-found"
	case strings.Contains(lower, "network is unreachable") || strings.Contains(lower, "no route"):
		return "no-route"
	default:
		return "network-error"
	}
}

func boundedError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 240 {
		msg = msg[:240]
	}
	return msg
}
