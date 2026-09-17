package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const dnsPolicyIngressChain = "RF_DNS_INGRESS"

type DNSPolicyIngressConfig struct {
	Interface  string
	ListenAddr string
}

func validateDNSPolicyIngressConfig(cfg DNSPolicyIngressConfig) (DNSPolicyIngressConfig, error) {
	cfg.Interface = strings.TrimSpace(cfg.Interface)
	if cfg.Interface == "" || strings.ContainsAny(cfg.Interface, " \t\r\n/") {
		return DNSPolicyIngressConfig{}, fmt.Errorf("invalid DNS ingress interface")
	}
	host, portText, err := net.SplitHostPort(strings.TrimSpace(cfg.ListenAddr))
	if err != nil {
		return DNSPolicyIngressConfig{}, fmt.Errorf("invalid DNS ingress listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return DNSPolicyIngressConfig{}, fmt.Errorf("DNS ingress requires an explicit non-loopback IPv4 address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 || port == 53 {
		return DNSPolicyIngressConfig{}, fmt.Errorf("DNS ingress requires a non-53 listen port")
	}
	cfg.ListenAddr = net.JoinHostPort(ip.String(), strconv.Itoa(port))
	return cfg, nil
}

func dnsPolicyIngressJumpArgs(iface, proto string) []string {
	return []string{"-t", "nat", "PREROUTING", "-i", iface, "-p", proto, "--dport", "53", "-j", dnsPolicyIngressChain}
}

func dnsPolicyIngressRedirectArgs(proto string, port int) []string {
	return []string{"-t", "nat", dnsPolicyIngressChain, "-p", proto, "--dport", "53", "-j", "REDIRECT", "--to-ports", strconv.Itoa(port)}
}
