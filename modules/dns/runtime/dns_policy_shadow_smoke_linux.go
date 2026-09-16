//go:build linux

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

type DNSPolicyShadowSmokeCase struct {
	Name      string
	Domain    string
	Policy    string
	WantMark  uint32
	WantReply bool
}

type DNSPolicyShadowSmokeCaseResult struct {
	Name      string
	Transport string
	Policy    string
	WantMark  uint32
	Reply     bool
	Error     string
}

type DNSPolicyShadowSmokeResult struct {
	ListenAddr string
	Upstream   string
	Cases      []DNSPolicyShadowSmokeCaseResult
	Marks      []uint32
}

func runDNSPolicyShadowSmoke(listenAddr, upstream string, timeout time.Duration) (DNSPolicyShadowSmokeResult, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	cases := []DNSPolicyShadowSmokeCase{
		{Name: "system", Domain: "routerforge-shadow-system.invalid", Policy: "System", WantReply: true},
		{Name: "policy1", Domain: "routerforge-shadow-policy1.invalid", Policy: "Policy1", WantMark: 0x0ffffaab, WantReply: true},
		{Name: "policy0", Domain: "routerforge-shadow-policy0.invalid", Policy: "Policy0", WantMark: 0x0ffffaaa, WantReply: false},
	}
	allowed := map[string]bool{"System": true, "Policy0": true, "Policy1": true}
	rules := []DNSPolicyRule{
		{ID: "shadow-policy1", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: cases[1].Domain, QueryType: "A"}},
		{ID: "shadow-policy0", Priority: 20, Policy: "Policy0", Match: DNSPolicyMatch{DomainSuffix: cases[2].Domain, QueryType: "A"}},
	}
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr: listenAddr,
		Upstream:   upstream,
		Rules:      rules,
		Allowed:    allowed,
		Timeout:    timeout,
	})
	if err != nil {
		return DNSPolicyShadowSmokeResult{}, err
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
			return fmt.Errorf("shadow smoke verify SO_MARK: %w", err)
		}
		if uint32(got) != mark {
			return fmt.Errorf("shadow smoke SO_MARK mismatch: got %#x want %#x", uint32(got), mark)
		}
		markMu.Lock()
		marks = append(marks, mark)
		markMu.Unlock()
		return nil
	}

	server, err := newDNSPolicyShadowServer(cfg, discoverPolicyRoutes)
	if err != nil {
		return DNSPolicyShadowSmokeResult{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(ctx) }()

	if err := waitDNSPolicyShadowReady(ctx, cfg.ListenAddr, 2*time.Second); err != nil {
		cancel()
		<-serverErr
		return DNSPolicyShadowSmokeResult{}, err
	}

	result := DNSPolicyShadowSmokeResult{ListenAddr: cfg.ListenAddr, Upstream: cfg.Upstream}
	id := uint16(0x6100)
	for _, tc := range cases {
		for _, transport := range []string{"udp", "tcp"} {
			id++
			query := buildDNSQuery(id, tc.Domain, 1)
			var response []byte
			var queryErr error
			switch transport {
			case "udp":
				response, queryErr = dnsPolicyShadowSmokeUDP(cfg.ListenAddr, query, timeout)
			case "tcp":
				response, queryErr = dnsPolicyShadowSmokeTCP(cfg.ListenAddr, query, timeout)
			}
			gotReply := queryErr == nil
			caseResult := DNSPolicyShadowSmokeCaseResult{
				Name: tc.Name, Transport: transport, Policy: tc.Policy,
				WantMark: tc.WantMark, Reply: gotReply,
			}
			if queryErr != nil {
				caseResult.Error = queryErr.Error()
			}
			result.Cases = append(result.Cases, caseResult)

			if tc.WantReply && !gotReply {
				cancel()
				<-serverErr
				return result, fmt.Errorf("%s/%s expected DNS reply: %v", tc.Name, transport, queryErr)
			}
			if !tc.WantReply && gotReply {
				cancel()
				<-serverErr
				return result, fmt.Errorf("%s/%s unexpectedly received DNS reply", tc.Name, transport)
			}
			if gotReply {
				q, qok := parseDNSMessage(query)
				r, rok := parseDNSMessage(response)
				if !qok || !rok || !r.QR || q.ID != r.ID {
					cancel()
					<-serverErr
					return result, fmt.Errorf("%s/%s invalid DNS response", tc.Name, transport)
				}
			}
		}
	}

	cancel()
	if err := <-serverErr; err != nil {
		return result, fmt.Errorf("shadow server exit: %w", err)
	}
	markMu.Lock()
	result.Marks = append(result.Marks, marks...)
	markMu.Unlock()

	if countDNSPolicyShadowMark(result.Marks, 0x0ffffaab) < 2 {
		return result, fmt.Errorf("Policy1 mark was not observed for both UDP and TCP")
	}
	if countDNSPolicyShadowMark(result.Marks, 0x0ffffaaa) < 2 {
		return result, fmt.Errorf("Policy0 mark was not observed for both UDP and TCP")
	}
	return result, nil
}

func waitDNSPolicyShadowReady(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp4", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return fmt.Errorf("shadow listener did not become ready")
}

func dnsPolicyShadowSmokeUDP(addr string, query []byte, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialTimeout("udp4", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(query); err != nil {
		return nil, err
	}
	buf := make([]byte, 65535)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), buf[:n]...), nil
}

func dnsPolicyShadowSmokeTCP(addr string, query []byte, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialTimeout("tcp4", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	frame := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(query)))
	copy(frame[2:], query)
	if _, err := conn.Write(frame); err != nil {
		return nil, err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n < 12 || n > 65535 {
		return nil, fmt.Errorf("invalid shadow smoke TCP frame length %d", n)
	}
	response := make([]byte, n)
	if _, err := io.ReadFull(conn, response); err != nil {
		return nil, err
	}
	return response, nil
}

func countDNSPolicyShadowMark(marks []uint32, target uint32) int {
	count := 0
	for _, mark := range marks {
		if mark == target {
			count++
		}
	}
	return count
}
