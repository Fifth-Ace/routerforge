package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// K3C collects evidence only. None of these observations authorize rule changes.
func appendPreflightEvidence(p *Preflight) {
	p.Checks = append(p.Checks, inspectNDM(), inspectSSH(), inspectWAN())
}

func evidence(id, state, detail string) PreflightCheck {
	return PreflightCheck{ID: id, State: state, Detail: detail}
}

func inspectNDM() PreflightCheck {
	paths := []string{"/opt/sbin/iptables", "/usr/sbin/iptables", "/sbin/iptables"}
	var binary string
	for _, path := range paths {
		if st, err := os.Stat(path); err == nil && !st.IsDir() && st.Mode()&0111 != 0 {
			binary = path
			break
		}
	}
	if binary == "" {
		return evidence("ndm_chain_evidence", "unknown", "iptables binary not found at trusted paths; chain order cannot be inspected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := safety.RunCommandOutput(ctx, binary, "-t", "filter", "-S")
	if err != nil {
		return evidence("ndm_chain_evidence", "unknown", "Read-only iptables listing failed; NDM chain order remains unverified")
	}
	if len(output) > 1<<20 {
		return evidence("ndm_chain_evidence", "unknown", "iptables listing exceeded evidence limit")
	}
	text := string(output)
	var chains []string
	for _, name := range []string{"_NDM_FORWARD", "_NDM_INPUT", "FORWARD", "INPUT"} {
		if strings.Contains(text, "-N "+name+"\n") || strings.Contains(text, "-P "+name+" ") || strings.Contains(text, "-A "+name+" ") {
			chains = append(chains, name)
		}
	}
	if len(chains) == 0 {
		return evidence("ndm_chain_evidence", "unknown", "No expected chains were identifiable in filter listing")
	}
	return evidence("ndm_chain_evidence", "observed", "Visible filter chains: "+strings.Join(chains, ", ")+"; insertion order and NDM persistence NOT verified")
}

func listenPorts(filename string) map[int]bool {
	result := map[int]bool{}
	data, err := os.ReadFile(filename)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] != "0A" {
			continue
		}
		parts := strings.Split(fields[1], ":")
		if len(parts) != 2 {
			continue
		}
		port, err := strconv.ParseUint(parts[1], 16, 16)
		if err == nil {
			result[int(port)] = true
		}
	}
	return result
}

func inspectSSH() PreflightCheck {
	ports := listenPorts("/proc/net/tcp")
	for port := range listenPorts("/proc/net/tcp6") {
		ports[port] = true
	}
	var active []string
	for _, port := range []int{22, 222, 2222} {
		if ports[port] {
			active = append(active, strconv.Itoa(port))
		}
	}
	if len(active) == 0 {
		return evidence("ssh_listener_evidence", "unknown", "No common SSH management ports observed in TCP LISTEN; custom ports/rescue not assessed")
	}
	return evidence("ssh_listener_evidence", "observed", "TCP LISTEN on port(s) "+strings.Join(active, ", ")+"; reachability, authenticated session and rescue NOT verified")
}

func inspectWAN() PreflightCheck {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return evidence("wan_route_evidence", "unknown", "Kernel route table is unavailable")
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&1 == 0 {
			continue
		}
		iface := fields[0]
		if iface == "" || strings.ContainsAny(iface, "/\\\x00\n") {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", iface)); err != nil {
			continue
		}
		return evidence("wan_route_evidence", "observed", fmt.Sprintf("Default IPv4 route interface: %s; physical WAN identity, NAT target and forwarding NOT verified", iface))
	}
	return evidence("wan_route_evidence", "unknown", "No verifiable default IPv4 route interface; WAN/NAT scope remains unknown")
}
