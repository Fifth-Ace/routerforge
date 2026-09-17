//go:build linux

package main

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func validateDNSPolicyShadowListenOwnership(cfg DNSPolicyShadowConfig) error {
	host, _, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("invalid shadow listen IP")
	}
	if ip.IsLoopback() {
		return nil
	}
	iface, err := net.InterfaceByName(cfg.ListenInterface)
	if err != nil {
		return fmt.Errorf("shadow listen interface %s: %w", cfg.ListenInterface, err)
	}
	if iface.Flags&net.FlagUp == 0 {
		return fmt.Errorf("shadow listen interface %s is down", cfg.ListenInterface)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return fmt.Errorf("shadow listen interface addresses: %w", err)
	}
	for _, addr := range addrs {
		var candidate net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			candidate = value.IP
		case *net.IPAddr:
			candidate = value.IP
		}
		if candidate != nil && candidate.Equal(ip) {
			return nil
		}
	}
	return fmt.Errorf("shadow listen IP %s does not belong to interface %s", ip.String(), cfg.ListenInterface)
}

type dnsPolicyIngressManager struct {
	cfg  DNSPolicyIngressConfig
	port int
}

func newDNSPolicyIngressManager(cfg DNSPolicyIngressConfig) (*dnsPolicyIngressManager, error) {
	validated, err := validateDNSPolicyIngressConfig(cfg)
	if err != nil {
		return nil, err
	}
	shadowCfg := DNSPolicyShadowConfig{ListenAddr: validated.ListenAddr, ListenInterface: validated.Interface, Upstream: "1.1.1.1:53", Allowed: map[string]bool{"System": true}}
	shadowCfg, err = validateDNSPolicyShadowConfig(shadowCfg)
	if err != nil {
		return nil, err
	}
	if err := validateDNSPolicyShadowListenOwnership(shadowCfg); err != nil {
		return nil, err
	}
	_, portText, _ := net.SplitHostPort(validated.ListenAddr)
	port, _ := strconv.Atoi(portText)
	return &dnsPolicyIngressManager{cfg: validated, port: port}, nil
}

func (m *dnsPolicyIngressManager) run(args ...string) ([]byte, error) {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return nil, fmt.Errorf("iptables unavailable: %w", err)
	}
	cmd := exec.Command(path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("iptables %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (m *dnsPolicyIngressManager) exists(args ...string) bool {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return false
	}
	return exec.Command(path, args...).Run() == nil
}

func (m *dnsPolicyIngressManager) ReconcileStale() error {
	for _, proto := range []string{"udp", "tcp"} {
		spec := dnsPolicyIngressJumpArgs(m.cfg.Interface, proto)
		for i := 0; i < 8; i++ {
			deleteArgs := append([]string{"-t", "nat", "-D", "PREROUTING"}, spec[3:]...)
			if _, err := m.run(deleteArgs...); err != nil {
				break
			}
		}
	}
	if m.exists("-t", "nat", "-S", dnsPolicyIngressChain) {
		_, _ = m.run("-t", "nat", "-F", dnsPolicyIngressChain)
		if _, err := m.run("-t", "nat", "-X", dnsPolicyIngressChain); err != nil {
			return err
		}
	}
	return nil
}

func (m *dnsPolicyIngressManager) Precheck() error {
	if _, err := exec.LookPath("iptables"); err != nil {
		return fmt.Errorf("iptables unavailable: %w", err)
	}
	if _, err := m.run("-t", "nat", "-S", "PREROUTING"); err != nil {
		return err
	}
	if m.exists("-t", "nat", "-S", dnsPolicyIngressChain) {
		return fmt.Errorf("stale RouterForge DNS ingress chain exists")
	}
	return nil
}

func (m *dnsPolicyIngressManager) Install() error {
	if _, err := m.run("-t", "nat", "-N", dnsPolicyIngressChain); err != nil {
		return err
	}
	installed := false
	defer func() {
		if !installed {
			_ = m.Remove()
		}
	}()
	for _, proto := range []string{"udp", "tcp"} {
		spec := dnsPolicyIngressRedirectArgs(proto, m.port)
		args := append([]string{"-t", "nat", "-A", dnsPolicyIngressChain}, spec[3:]...)
		if _, err := m.run(args...); err != nil {
			return err
		}
	}
	for _, proto := range []string{"udp", "tcp"} {
		spec := dnsPolicyIngressJumpArgs(m.cfg.Interface, proto)
		args := append([]string{"-t", "nat", "-I", "PREROUTING", "1"}, spec[3:]...)
		if _, err := m.run(args...); err != nil {
			return err
		}
	}
	installed = true
	return nil
}

func (m *dnsPolicyIngressManager) VerifyInstalled() error {
	for _, proto := range []string{"udp", "tcp"} {
		jumpSpec := dnsPolicyIngressJumpArgs(m.cfg.Interface, proto)
		jump := append([]string{"-t", "nat", "-C", "PREROUTING"}, jumpSpec[3:]...)
		if _, err := m.run(jump...); err != nil {
			return fmt.Errorf("verify %s ingress jump: %w", proto, err)
		}
		redirectSpec := dnsPolicyIngressRedirectArgs(proto, m.port)
		redirect := append([]string{"-t", "nat", "-C", dnsPolicyIngressChain}, redirectSpec[3:]...)
		if _, err := m.run(redirect...); err != nil {
			return fmt.Errorf("verify %s ingress redirect: %w", proto, err)
		}
	}
	return nil
}

func (m *dnsPolicyIngressManager) Remove() error {
	var first error
	for _, proto := range []string{"udp", "tcp"} {
		spec := dnsPolicyIngressJumpArgs(m.cfg.Interface, proto)
		checkArgs := append([]string{"-t", "nat", "-C", "PREROUTING"}, spec[3:]...)
		deleteArgs := append([]string{"-t", "nat", "-D", "PREROUTING"}, spec[3:]...)
		for m.exists(checkArgs...) {
			if _, err := m.run(deleteArgs...); err != nil && first == nil {
				first = err
				break
			}
		}
	}
	if m.exists("-t", "nat", "-S", dnsPolicyIngressChain) {
		if _, err := m.run("-t", "nat", "-F", dnsPolicyIngressChain); err != nil && first == nil {
			first = err
		}
		if _, err := m.run("-t", "nat", "-X", dnsPolicyIngressChain); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *dnsPolicyIngressManager) VerifyRemoved() error {
	if m.exists("-t", "nat", "-S", dnsPolicyIngressChain) {
		return fmt.Errorf("RouterForge DNS ingress chain still exists")
	}
	for _, proto := range []string{"udp", "tcp"} {
		jumpSpec := dnsPolicyIngressJumpArgs(m.cfg.Interface, proto)
		jump := append([]string{"-t", "nat", "-C", "PREROUTING"}, jumpSpec[3:]...)
		if m.exists(jump...) {
			return fmt.Errorf("RouterForge %s ingress jump still exists", proto)
		}
	}
	return nil
}

func (m *dnsPolicyIngressManager) Dump() string {
	out, err := m.run("-t", "nat", "-L", dnsPolicyIngressChain, "-v", "-n", "-x")
	if err != nil {
		return err.Error()
	}
	return string(out)
}
