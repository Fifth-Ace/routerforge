//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"sync"
	"time"
)

type dnsPolicyProductionTakeoverPrimitive struct {
	cfg      DNSPolicyShadowConfig
	ingress  *dnsPolicyIngressManager
	server   *dnsPolicyShadowServer
	cancel   context.CancelFunc
	serverCh chan error
	mu       sync.Mutex
}

func newDNSPolicyProductionTakeoverPrimitive(
	iface, listenAddr, upstream string,
	timeout time.Duration,
) (*dnsPolicyProductionTakeoverPrimitive, error) {
	manager, err := newDNSPolicyIngressManager(DNSPolicyIngressConfig{Interface: iface, ListenAddr: listenAddr})
	if err != nil {
		return nil, err
	}
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr: listenAddr, ListenInterface: iface, Upstream: upstream,
		Allowed: map[string]bool{"System": true}, Timeout: timeout, MaxConcurrent: 32,
	})
	if err != nil {
		return nil, err
	}
	return &dnsPolicyProductionTakeoverPrimitive{cfg: cfg, ingress: manager}, nil
}

func (p *dnsPolicyProductionTakeoverPrimitive) PrecheckIngress(rules []DNSPolicyRule) error {
	if p == nil || p.ingress == nil {
		return fmt.Errorf("production DNS ingress primitive is unavailable")
	}
	if err := p.ingress.Precheck(); err != nil {
		return err
	}
	inventory, err := readDNSPolicyInventory()
	if err != nil {
		return err
	}
	allowed := dnsPolicyAllowedFromInventory(inventory)
	if _, err := validateDNSPolicyRules(rules, allowed); err != nil {
		return err
	}
	return nil
}

func (p *dnsPolicyProductionTakeoverPrimitive) SnapshotIngress() (DNSPolicyIngressSnapshot, error) {
	if err := p.ingress.VerifyRemoved(); err != nil {
		return DNSPolicyIngressSnapshot{}, err
	}
	host, _, _ := net.SplitHostPort(p.cfg.ListenAddr)
	if err := verifyDNSPolicyNativeDNS(net.JoinHostPort(host, "53"), p.cfg.Timeout); err != nil {
		return DNSPolicyIngressSnapshot{}, err
	}
	raw := fmt.Sprintf("native|%s|%s|udp+tcp|rf-chain-absent", p.ingress.cfg.Interface, host)
	sum := sha256.Sum256([]byte(raw))
	return DNSPolicyIngressSnapshot{
		Identity:        fmt.Sprintf("%x", sum),
		NativeOwner:     "ndnproxy",
		NativePort53:    true,
		RouterForgeAddr: p.cfg.ListenAddr,
	}, nil
}

func (p *dnsPolicyProductionTakeoverPrimitive) StartRouterForgeProxy(rules []DNSPolicyRule) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server != nil {
		return fmt.Errorf("RouterForge policy proxy already running")
	}
	inventory, err := readDNSPolicyInventory()
	if err != nil {
		return err
	}
	cfg := p.cfg
	cfg.Allowed = dnsPolicyAllowedFromInventory(inventory)
	cfg.Rules, err = validateDNSPolicyRules(rules, cfg.Allowed)
	if err != nil {
		return err
	}
	server, err := newDNSPolicyShadowServer(cfg, discoverPolicyRoutes)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan error, 1)
	go func() { ch <- server.Serve(ctx) }()
	if err := waitDNSPolicyShadowReady(ctx, cfg.ListenAddr, 2*time.Second); err != nil {
		cancel()
		select {
		case <-ch:
		case <-time.After(time.Second):
		}
		return err
	}
	p.cfg = cfg
	p.server = server
	p.cancel = cancel
	p.serverCh = ch
	return nil
}

func (p *dnsPolicyProductionTakeoverPrimitive) VerifyRouterForgeProxy(_ []DNSPolicyRule) error {
	query := buildDNSQuery(0x7a01, "routerforge-activation-ready.invalid", 1)
	response, err := dnsPolicyShadowSmokeUDP(p.cfg.ListenAddr, query, p.cfg.Timeout)
	if err != nil {
		return err
	}
	msg, ok := parseDNSMessage(response)
	if !ok || !msg.QR || msg.ID != 0x7a01 {
		return fmt.Errorf("RouterForge policy proxy readiness response invalid")
	}
	return nil
}

func (p *dnsPolicyProductionTakeoverPrimitive) SwitchIngressToRouterForge() error {
	return p.ingress.Install()
}

func (p *dnsPolicyProductionTakeoverPrimitive) VerifyIngressOnRouterForge() error {
	return p.ingress.VerifyInstalled()
}

func (p *dnsPolicyProductionTakeoverPrimitive) RestoreNativeIngress(_ DNSPolicyIngressSnapshot) error {
	if err := p.ingress.Remove(); err != nil {
		return err
	}
	return p.ingress.VerifyRemoved()
}

func (p *dnsPolicyProductionTakeoverPrimitive) StopRouterForgeProxy() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server == nil {
		return nil
	}
	p.cancel()
	select {
	case err := <-p.serverCh:
		if err != nil {
			return err
		}
	case <-time.After(2 * time.Second):
		return fmt.Errorf("RouterForge policy proxy did not stop")
	}
	p.server = nil
	p.cancel = nil
	p.serverCh = nil
	return nil
}

func (p *dnsPolicyProductionTakeoverPrimitive) VerifyNativeIngress(_ DNSPolicyIngressSnapshot) error {
	host, _, _ := net.SplitHostPort(p.cfg.ListenAddr)
	return verifyDNSPolicyNativeDNS(net.JoinHostPort(host, "53"), p.cfg.Timeout)
}

func verifyDNSPolicyNativeDNS(addr string, timeout time.Duration) error {
	for index, transport := range []string{"udp", "tcp"} {
		query := buildDNSQuery(uint16(0x7b00+index), "example.com", 1)
		var response []byte
		var err error
		if transport == "udp" {
			response, err = dnsPolicyShadowSmokeUDP(addr, query, timeout)
		} else {
			response, err = dnsPolicyShadowSmokeTCP(addr, query, timeout)
		}
		if err != nil {
			return fmt.Errorf("native DNS %s verification: %w", transport, err)
		}
		msg, ok := parseDNSMessage(response)
		if !ok || !msg.QR || msg.ID != uint16(0x7b00+index) {
			return fmt.Errorf("native DNS %s verification returned invalid response", transport)
		}
	}
	return nil
}
