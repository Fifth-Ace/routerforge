//go:build linux

package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"
)

type DNSPolicyIngressSmokeResult struct {
	ListenAddr       string
	Interface        string
	IngressInstalled bool
	IngressVerified  bool
	IngressRemoved   bool
	NativeUDPAfter   bool
	NativeTCPAfter   bool
	Policy1Marks     int
	Policy0Marks     int
	Stats            DNSPolicyShadowStats
	RuleDump         string
}

func runDNSPolicyIngressSmoke(iface, listenAddr, upstream string, timeout time.Duration) (result DNSPolicyIngressSmokeResult, err error) {
	result.Interface = iface
	result.ListenAddr = listenAddr
	if timeout <= 0 {
		timeout = 4 * time.Second
	}

	manager, err := newDNSPolicyIngressManager(DNSPolicyIngressConfig{Interface: iface, ListenAddr: listenAddr})
	if err != nil {
		return result, err
	}
	if err := manager.ReconcileStale(); err != nil {
		return result, fmt.Errorf("reconcile stale ingress: %w", err)
	}
	if err := manager.Precheck(); err != nil {
		return result, err
	}

	cases := []DNSPolicyShadowSmokeCase{
		{Name: "system", Domain: "routerforge-ingress-system.invalid", Policy: "System"},
		{Name: "policy1", Domain: "routerforge-ingress-policy1.invalid", Policy: "Policy1", WantMark: 0x0ffffaab},
		{Name: "policy0", Domain: "routerforge-ingress-policy0.invalid", Policy: "Policy0", WantMark: 0x0ffffaaa, WantRCode: "SERVFAIL"},
	}
	allowed := map[string]bool{"System": true, "Policy0": true, "Policy1": true}
	rules := []DNSPolicyRule{
		{ID: "ingress-policy1", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: cases[1].Domain, QueryType: "A"}},
		{ID: "ingress-policy0", Priority: 20, Policy: "Policy0", Match: DNSPolicyMatch{DomainSuffix: cases[2].Domain, QueryType: "A"}},
	}
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr: listenAddr, ListenInterface: iface, Upstream: upstream, Rules: rules, Allowed: allowed, Timeout: timeout, MaxConcurrent: 8,
	})
	if err != nil {
		return result, err
	}

	var markMu sync.Mutex
	marks := make([]uint32, 0, 4)
	originalSetter := dnsPolicySetSocketMark
	defer func() { dnsPolicySetSocketMark = originalSetter }()
	dnsPolicySetSocketMark = func(fd int, mark uint32) error {
		if err := originalSetter(fd, mark); err != nil {
			return err
		}
		got, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK)
		if err != nil {
			return err
		}
		if uint32(got) != mark {
			return fmt.Errorf("SO_MARK mismatch got=%#x want=%#x", uint32(got), mark)
		}
		markMu.Lock()
		marks = append(marks, mark)
		markMu.Unlock()
		return nil
	}

	server, err := newDNSPolicyShadowServer(cfg, discoverPolicyRoutes)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(ctx) }()
	defer func() {
		_ = manager.Remove()
		cancel()
		select {
		case <-serverErr:
		case <-time.After(2 * time.Second):
		}
	}()
	if err := waitDNSPolicyShadowReady(ctx, cfg.ListenAddr, 2*time.Second); err != nil {
		return result, err
	}

	if err := manager.Install(); err != nil {
		return result, err
	}
	result.IngressInstalled = true
	if err := manager.VerifyInstalled(); err != nil {
		return result, err
	}
	result.IngressVerified = true
	result.RuleDump = manager.Dump()

	id := uint16(0x7100)
	for _, tc := range cases {
		for _, transport := range []string{"udp", "tcp"} {
			id++
			query := buildDNSQuery(id, tc.Domain, 1)
			var response []byte
			if transport == "udp" {
				response, err = dnsPolicyShadowSmokeUDP(cfg.ListenAddr, query, timeout)
			} else {
				response, err = dnsPolicyShadowSmokeTCP(cfg.ListenAddr, query, timeout)
			}
			if err != nil {
				return result, fmt.Errorf("direct proxy %s/%s: %w", tc.Name, transport, err)
			}
			msg, ok := parseDNSMessage(response)
			if !ok || !msg.QR || msg.ID != id {
				return result, fmt.Errorf("invalid direct proxy response %s/%s", tc.Name, transport)
			}
			if tc.WantRCode != "" && rcodeName(msg.RCode) != tc.WantRCode {
				return result, fmt.Errorf("direct proxy %s/%s rcode=%s want=%s", tc.Name, transport, rcodeName(msg.RCode), tc.WantRCode)
			}
		}
	}

	markMu.Lock()
	result.Policy1Marks = countDNSPolicyShadowMark(marks, 0x0ffffaab)
	result.Policy0Marks = countDNSPolicyShadowMark(marks, 0x0ffffaaa)
	markMu.Unlock()
	result.Stats = server.Stats()
	if result.Policy1Marks < 2 || result.Policy0Marks < 2 {
		return result, fmt.Errorf("marked proxy evidence incomplete")
	}

	if err := manager.Remove(); err != nil {
		return result, fmt.Errorf("remove ingress: %w", err)
	}
	if err := manager.VerifyRemoved(); err != nil {
		return result, err
	}
	result.IngressRemoved = true

	host, _, _ := net.SplitHostPort(cfg.ListenAddr)
	query := buildDNSQuery(0x7201, "example.com", 1)
	if response, qerr := dnsPolicyShadowSmokeUDP(net.JoinHostPort(host, "53"), query, timeout); qerr == nil {
		if msg, ok := parseDNSMessage(response); ok && msg.QR && msg.ID == 0x7201 {
			result.NativeUDPAfter = true
		}
	}
	query = buildDNSQuery(0x7202, "example.com", 1)
	if response, qerr := dnsPolicyShadowSmokeTCP(net.JoinHostPort(host, "53"), query, timeout); qerr == nil {
		if msg, ok := parseDNSMessage(response); ok && msg.QR && msg.ID == 0x7202 {
			result.NativeTCPAfter = true
		}
	}
	if !result.NativeUDPAfter || !result.NativeTCPAfter {
		return result, fmt.Errorf("native DNS did not recover after ingress rollback")
	}

	cancel()
	if serr := <-serverErr; serr != nil {
		return result, serr
	}
	return result, nil
}
