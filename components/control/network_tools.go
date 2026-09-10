package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	adminNetworkHostLimit   = 253
	adminNetworkOutputLimit = 32 << 10
	adminNetworkTimeout     = 12 * time.Second
)

type adminNetworkToolRequest struct {
	Tool string `json:"tool"`
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
}

func registerAdminNetworkToolRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/network-tools/run", mutationOnly(handleAdminNetworkToolRun))
}

func handleAdminNetworkToolRun(w http.ResponseWriter, r *http.Request) {
	var request adminNetworkToolRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid network tool request"})
		return
	}
	host := strings.TrimSpace(request.Host)
	if !validNetworkTarget(host) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid host"})
		return
	}

	tool := strings.ToLower(strings.TrimSpace(request.Tool))
	ctx, cancel := context.WithTimeout(r.Context(), adminNetworkTimeout)
	defer cancel()

	switch tool {
	case "dns":
		start := time.Now()
		addresses, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "tool": tool, "host": host, "error": err.Error(),
				"duration_ms": time.Since(start).Milliseconds(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "tool": tool, "host": host, "addresses": addresses,
			"duration_ms": time.Since(start).Milliseconds(),
		})
	case "tcp":
		if request.Port < 1 || request.Port > 65535 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "port must be 1..65535"})
			return
		}
		start := time.Now()
		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(request.Port)))
		if conn != nil {
			_ = conn.Close()
		}
		response := map[string]any{
			"ok": err == nil, "tool": tool, "host": host, "port": request.Port,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			response["error"] = err.Error()
		}
		writeJSON(w, http.StatusOK, response)
	case "ping":
		runNetworkCommand(w, ctx, tool, host, "ping", "-c", "4", "-W", "2", host)
	case "traceroute":
		runNetworkCommand(w, ctx, tool, host, "traceroute", "-m", "12", "-w", "2", host)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported tool; allowed: ping, traceroute, dns, tcp"})
	}
}

func validNetworkTarget(host string) bool {
	if host == "" || len(host) > adminNetworkHostLimit || strings.ContainsAny(host, " \t\r\n/\\") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
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

func runNetworkCommand(w http.ResponseWriter, ctx context.Context, tool, host, binary string, args ...string) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	truncated := false
	if len(output) > adminNetworkOutputLimit {
		output = output[:adminNetworkOutputLimit]
		truncated = true
	}
	response := map[string]any{
		"ok":               err == nil,
		"tool":             tool,
		"host":             host,
		"output":           string(output),
		"duration_ms":      time.Since(start).Milliseconds(),
		"output_truncated": truncated,
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			response["exit_code"] = exitErr.ExitCode()
		} else {
			response["error"] = err.Error()
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		response["timed_out"] = true
	}
	writeJSON(w, http.StatusOK, response)
}
