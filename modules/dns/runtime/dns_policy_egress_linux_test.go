//go:build linux

package main

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

type fakeDNSPolicyRawConn struct {
	fd uintptr
}

func (f fakeDNSPolicyRawConn) Control(fn func(uintptr)) error {
	fn(f.fd)
	return nil
}
func (fakeDNSPolicyRawConn) Read(func(uintptr) bool) error  { return errors.New("unused") }
func (fakeDNSPolicyRawConn) Write(func(uintptr) bool) error { return errors.New("unused") }

func TestDNSPolicyMarkedDialerSetsExpectedSO_MARK(t *testing.T) {
	original := dnsPolicySetSocketMark
	defer func() { dnsPolicySetSocketMark = original }()

	var gotFD int
	var gotMark uint32
	dnsPolicySetSocketMark = func(fd int, mark uint32) error {
		gotFD = fd
		gotMark = mark
		return nil
	}

	target := DNSPolicyEgressTarget{Policy: "Policy1", Mark: 0x0ffffaab, Table: 4098, HasDefault: true}
	dialer, err := newDNSPolicyMarkedDialer(target, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if dialer.Control == nil {
		t.Fatal("marked dialer must install Control")
	}
	if err := dialer.Control("udp4", "1.1.1.1:53", fakeDNSPolicyRawConn{fd: 42}); err != nil {
		t.Fatal(err)
	}
	if gotFD != 42 || gotMark != 0x0ffffaab {
		t.Fatalf("setsockopt got fd=%d mark=%#x", gotFD, gotMark)
	}
}

func TestDNSPolicyMarkedDialerPropagatesSetMarkFailure(t *testing.T) {
	original := dnsPolicySetSocketMark
	defer func() { dnsPolicySetSocketMark = original }()

	want := syscall.EPERM
	dnsPolicySetSocketMark = func(int, uint32) error { return want }

	target := DNSPolicyEgressTarget{Policy: "Policy1", Mark: 0x0ffffaab, Table: 4098}
	dialer, err := newDNSPolicyMarkedDialer(target, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := dialer.Control("tcp4", "1.1.1.1:53", fakeDNSPolicyRawConn{fd: 9}); !errors.Is(err, want) {
		t.Fatalf("control error = %v, want EPERM", err)
	}
}

func TestDNSPolicySystemDialerHasNoMarkControl(t *testing.T) {
	dialer, err := newDNSPolicyMarkedDialer(DNSPolicyEgressTarget{Policy: "System", System: true}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if dialer.Control != nil {
		t.Fatal("System dialer must not set SO_MARK")
	}
}
