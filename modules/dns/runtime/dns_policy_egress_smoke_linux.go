//go:build linux

package main

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
)

type DNSPolicyEgressSmokeResult struct {
	Policy        string
	Address       string
	RequestedMark uint32
	ActualMark    uint32
	Table         int
	HasDefault    bool
	Connected     bool
}

func runDNSPolicyEgressSmoke(policy, address string, timeout time.Duration) (DNSPolicyEgressSmokeResult, error) {
	routes := discoverPolicyRoutes()
	target, err := resolveDNSPolicyEgressTarget(policy, routes)
	if err != nil {
		return DNSPolicyEgressSmokeResult{}, err
	}
	result := DNSPolicyEgressSmokeResult{
		Policy:        target.Policy,
		Address:       address,
		RequestedMark: target.Mark,
		Table:         target.Table,
		HasDefault:    target.HasDefault,
	}

	originalSetter := dnsPolicySetSocketMark
	defer func() { dnsPolicySetSocketMark = originalSetter }()

	dnsPolicySetSocketMark = func(fd int, mark uint32) error {
		if err := originalSetter(fd, mark); err != nil {
			return err
		}
		got, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK)
		if err != nil {
			return fmt.Errorf("verify SO_MARK: %w", err)
		}
		result.ActualMark = uint32(got)
		if result.ActualMark != mark {
			return fmt.Errorf("SO_MARK mismatch: got %#x want %#x", result.ActualMark, mark)
		}
		return nil
	}

	dialer, err := newDNSPolicyMarkedDialer(target, timeout)
	if err != nil {
		return result, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp4", address)
	if err != nil {
		return result, err
	}
	result.Connected = true
	_ = conn.(*net.TCPConn).SetLinger(0)
	_ = conn.Close()
	return result, nil
}
