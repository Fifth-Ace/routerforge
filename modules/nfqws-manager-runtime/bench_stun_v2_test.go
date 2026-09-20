package main

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

func TestBenchSTUNBuildBindingRequest(t *testing.T) {
	request, err := benchSTUNBuildBindingRequest()
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Packet) != benchSTUNHeaderSize {
		t.Fatalf("packet size=%d want=%d", len(request.Packet), benchSTUNHeaderSize)
	}
	if len(request.TransactionID) != benchSTUNTransactionIDLen {
		t.Fatalf("transaction id size=%d", len(request.TransactionID))
	}
	if got := binary.BigEndian.Uint16(request.Packet[0:2]); got != benchSTUNBindingRequest {
		t.Fatalf("message type=%04x", got)
	}
	if got := binary.BigEndian.Uint16(request.Packet[2:4]); got != 0 {
		t.Fatalf("message length=%d", got)
	}
	if got := binary.BigEndian.Uint32(request.Packet[4:8]); got != benchSTUNMagicCookie {
		t.Fatalf("magic cookie=%08x", got)
	}
	if string(request.Packet[8:20]) != string(request.TransactionID) {
		t.Fatal("packet transaction id mismatch")
	}
}

func buildBenchSTUNXORMappedResponse(transactionID []byte, ip net.IP, port int) []byte {
	value := make([]byte, 8)
	value[1] = 0x01
	binary.BigEndian.PutUint16(value[2:4], uint16(port)^uint16(benchSTUNMagicCookie>>16))
	cookie := make([]byte, 4)
	binary.BigEndian.PutUint32(cookie, benchSTUNMagicCookie)
	v4 := ip.To4()
	for i := 0; i < 4; i++ {
		value[4+i] = v4[i] ^ cookie[i]
	}

	packet := make([]byte, benchSTUNHeaderSize+12)
	binary.BigEndian.PutUint16(packet[0:2], benchSTUNBindingSuccess)
	binary.BigEndian.PutUint16(packet[2:4], 12)
	binary.BigEndian.PutUint32(packet[4:8], benchSTUNMagicCookie)
	copy(packet[8:20], transactionID)
	binary.BigEndian.PutUint16(packet[20:22], benchSTUNAttrXORMapped)
	binary.BigEndian.PutUint16(packet[22:24], 8)
	copy(packet[24:32], value)
	return packet
}

func TestBenchSTUNParseBindingResponseRequiresTransactionAndMappedAddress(t *testing.T) {
	transactionID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	packet := buildBenchSTUNXORMappedResponse(transactionID, net.IPv4(203, 0, 113, 7), 54321)

	response, err := benchSTUNParseBindingResponse(packet, transactionID)
	if err != nil {
		t.Fatal(err)
	}
	if response.MessageType != "binding-success" || !response.TransactionMatched {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.MappedAddress != "203.0.113.7" || response.MappedPort != 54321 {
		t.Fatalf("mapped=%s:%d", response.MappedAddress, response.MappedPort)
	}

	wrong := append([]byte{}, transactionID...)
	wrong[0] ^= 0xff
	if _, err := benchSTUNParseBindingResponse(packet, wrong); err == nil || !strings.Contains(err.Error(), "transaction ID mismatch") {
		t.Fatalf("expected transaction mismatch, got %v", err)
	}
}

func TestBenchSTUNTransportProfileAndEligibility(t *testing.T) {
	transport, err := normalizeBenchTransport("stun")
	if err != nil {
		t.Fatal(err)
	}
	if transport.Network != "udp" || transport.RemotePort != 3478 || transport.MetricScope != "stun-binding-response" || transport.ProbeKind != "stun-binding" {
		t.Fatalf("unexpected STUN transport: %+v", transport)
	}

	profile := analyzeBenchStrategyProfile(0, []string{
		"--filter-udp=3478",
		"--filter-l7=stun",
		"--payload=stun",
		"--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2",
	})
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if !profile.CandidateEligible {
		t.Fatalf("STUN profile should be eligible: %v", profile.Reasons)
	}

	quic := analyzeBenchStrategyProfile(0, []string{
		"--filter-udp=443",
		"--filter-l7=quic",
		"--payload=quic_initial",
		"--lua-desync=fake:blob=fake_default_quic:repeats=2",
	})
	quic = prepareBenchStrategyProfileForTransport(quic, transport)
	if quic.CandidateEligible {
		t.Fatal("QUIC-only profile must not be eligible for STUN")
	}
}

func TestBenchSTUNCustomProfileIsDomainless(t *testing.T) {
	transport, err := normalizeBenchTransport("stun")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := v2CustomProfileForTransport([]string{
		"--filter-udp=3478",
		"--filter-l7=stun",
		"--payload=stun",
		"--hostlist=/tmp/not-portable.list",
		"--hostlist-domains=wrong.example",
		"--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2",
	}, "stun.cloudflare.com", transport)
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range profile.Args {
		if strings.HasPrefix(arg, "--hostlist") || strings.HasPrefix(arg, "--ipset") {
			t.Fatalf("domainless STUN profile retained selection filter %q", arg)
		}
	}
}

func TestBenchTransportProfilesIncludeSTUNWithoutChangingDefault(t *testing.T) {
	if len(benchTransportProfiles) != 4 {
		t.Fatalf("profile count=%d want=4", len(benchTransportProfiles))
	}
	defaultTransport, err := normalizeBenchTransport("")
	if err != nil {
		t.Fatal(err)
	}
	if defaultTransport.ID != benchTransportHTTPS {
		t.Fatalf("default transport=%q", defaultTransport.ID)
	}
}
