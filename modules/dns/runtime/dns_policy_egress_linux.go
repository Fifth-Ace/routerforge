//go:build linux

package main

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

var dnsPolicySetSocketMark = func(fd int, mark uint32) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, int(mark))
}

func newDNSPolicyMarkedDialer(target DNSPolicyEgressTarget, timeout time.Duration) (*net.Dialer, error) {
	if target.System {
		return &net.Dialer{Timeout: timeout}, nil
	}
	if target.Mark == 0 {
		return nil, fmt.Errorf("DNS policy %q has zero routing mark", target.Policy)
	}
	dialer := &net.Dialer{Timeout: timeout}
	dialer.Control = func(_, _ string, raw syscall.RawConn) error {
		var markErr error
		if err := raw.Control(func(fd uintptr) {
			markErr = dnsPolicySetSocketMark(int(fd), target.Mark)
		}); err != nil {
			return fmt.Errorf("DNS policy %q socket control: %w", target.Policy, err)
		}
		if markErr != nil {
			return fmt.Errorf("DNS policy %q set SO_MARK %#x: %w", target.Policy, target.Mark, markErr)
		}
		return nil
	}
	return dialer, nil
}
