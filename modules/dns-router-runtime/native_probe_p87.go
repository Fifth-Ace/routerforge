package dnsrouter

// P8.7: one explicitly requested DNS A query to ONE freshly discovered
// loopback ndnproxy endpoint. No changes to Keenetic, DNS zones or listeners.

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"time"
)

type NativeDNSProbeP87 struct {
	Mode             string `json:"mode"`
	Instance         string `json:"instance"`
	Hint             string `json:"upstream_hint"`
	Query            string `json:"query"`
	Target           string `json:"target,omitempty"`
	Attempted        bool   `json:"attempted"`
	DNSResponded     bool   `json:"dns_responded"`
	DNSSuccess       bool   `json:"dns_success"`
	RCode            int    `json:"rcode"`
	LatencyMS        int64  `json:"latency_ms"`
	Error            string `json:"error,omitempty"`
	EgressVerified   bool   `json:"egress_verified"`
	EncryptionProven bool   `json:"encryption_proven"`
	ChangesApplied   bool   `json:"changes_applied"`
}

func probeWireP87() ([]byte, [2]byte, error) {
	var id [2]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, id, err
	}
	q := []byte{id[0], id[1], 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	// example.com, QTYPE A, QCLASS IN (fixed: no free-form DNS injection)
	q = append(q, 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0, 0, 1, 0, 1)
	return q, id, nil
}

func ProbeNativeDNSP87(parent context.Context, inv NativeDNSInventoryP82, instance, hint string) NativeDNSProbeP87 {
	out := NativeDNSProbeP87{Mode: "explicit-loopback-dns-probe", Instance: instance, Hint: hint, Query: "example.com", RCode: -1}
	if instance != "System" && !nativeInstanceP83.MatchString(instance) {
		out.Error = "invalid policy selector"
		return out
	}
	if !nativeHintP83.MatchString(hint) {
		out.Error = "invalid native upstream hint"
		return out
	}
	u := Upstream{ID: "p87-probe", Transport: "native", Address: "native://" + instance + "/./" + hint}
	addr, err := ResolveNativeDestinationP83(u, "example.com", inv, "")
	if err != nil {
		out.Error = "native binding unavailable or ambiguous"
		return out
	}
	out.Target = addr
	ctx, cancel := context.WithTimeout(parent, 2200*time.Millisecond)
	defer cancel()
	dialer := net.Dialer{}
	started := time.Now()
	conn, err := dialer.DialContext(ctx, "udp4", addr)
	if err != nil {
		out.Error = "local DNS endpoint unavailable"
		return out
	}
	defer conn.Close()
	out.Attempted = true
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		out.Error = "cannot set query timeout"
		return out
	}
	query, id, err := probeWireP87()
	if err != nil {
		out.Error = "DNS request ID unavailable"
		return out
	}
	if _, err = conn.Write(query); err != nil {
		out.Error = "cannot send DNS query"
		return out
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	out.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		out.Error = "DNS response timeout or transport failure"
		return out
	}
	if n < 12 || buf[0] != id[0] || buf[1] != id[1] || buf[2]&0x80 == 0 || !sameDNSQuestion(query, buf[:n]) {
		out.Error = "DNS response rejected (ID, question or header mismatch)"
		return out
	}
	out.DNSResponded = true
	out.RCode = int(binary.BigEndian.Uint16(buf[2:4]) & 0xf)
	out.DNSSuccess = out.RCode == 0 && binary.BigEndian.Uint16(buf[6:8]) > 0
	if !out.DNSSuccess {
		if out.RCode == 2 {
			out.Error = "native upstream returned SERVFAIL"
		} else if out.RCode != 0 {
			out.Error = "native upstream returned DNS error"
		} else {
			out.Error = "DNS answer is empty"
		}
	}
	return out
}
