package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestBenchQUICInitialKeyDerivationMatchesRFC9001(t *testing.T) {
	dcid, err := hex.DecodeString("8394c8f03e515708")
	if err != nil {
		t.Fatal(err)
	}
	initialSecret := benchQUICHKDFExtract(benchQUICV1InitialSalt, dcid)
	clientSecret, err := benchQUICHKDFExpandLabel(initialSecret, "client in", 32)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(clientSecret); got != "c00cf151ca5be075ed0ebfb5c80323c42d6b7db67881289af4008f1f6c357aea" {
		t.Fatalf("client initial secret=%s", got)
	}
	key, err := benchQUICHKDFExpandLabel(clientSecret, "quic key", 16)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(key); got != "1f369613dd76d5467730efcbe3b1a22d" {
		t.Fatalf("client key=%s", got)
	}
	iv, err := benchQUICHKDFExpandLabel(clientSecret, "quic iv", 12)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(iv); got != "fa044b2f42a3fd3b46fb255c" {
		t.Fatalf("client iv=%s", got)
	}
	hp, err := benchQUICHKDFExpandLabel(clientSecret, "quic hp", 16)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(hp); got != "9f50449e04a0e810283a1e9933adedd2" {
		t.Fatalf("client hp=%s", got)
	}
}

func TestBenchQUICBuildInitialHasV1LongHeaderAndMinimumSize(t *testing.T) {
	initial, err := benchQUICBuildInitial("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Packet) != benchQUICMinInitialSize {
		t.Fatalf("packet size=%d want=%d", len(initial.Packet), benchQUICMinInitialSize)
	}
	if initial.Packet[0]&0x80 == 0 {
		t.Fatalf("packet is not long header: %02x", initial.Packet[0])
	}
	if got := uint32(initial.Packet[1])<<24 | uint32(initial.Packet[2])<<16 | uint32(initial.Packet[3])<<8 | uint32(initial.Packet[4]); got != benchQUICVersion1 {
		t.Fatalf("version=%08x", got)
	}
	if initial.Packet[5] != benchQUICCIDLength {
		t.Fatalf("dcid length=%d", initial.Packet[5])
	}
	if got := hex.EncodeToString(initial.Packet[6 : 6+benchQUICCIDLength]); got != hex.EncodeToString(initial.DCID) {
		t.Fatalf("packet dcid=%s want=%s", got, hex.EncodeToString(initial.DCID))
	}
}

func TestBenchQUICBuildInitialDecryptsWithDerivedClientKeys(t *testing.T) {
	initial, err := benchQUICBuildInitial("example.com")
	if err != nil {
		t.Fatal(err)
	}
	packet := append([]byte{}, initial.Packet...)

	position := 5
	dcidLength := int(packet[position])
	position += 1 + dcidLength
	scidLength := int(packet[position])
	position += 1 + scidLength
	if packet[position] != 0 {
		t.Fatalf("token length byte=%02x want=00", packet[position])
	}
	position++
	if packet[position]>>6 != 1 {
		t.Fatalf("length varint is not 2 bytes: %02x", packet[position])
	}
	pnOffset := position + 2

	initialSecret := benchQUICHKDFExtract(benchQUICV1InitialSalt, initial.DCID)
	clientSecret, err := benchQUICHKDFExpandLabel(initialSecret, "client in", sha256.Size)
	if err != nil {
		t.Fatal(err)
	}
	hpKey, err := benchQUICHKDFExpandLabel(clientSecret, "quic hp", 16)
	if err != nil {
		t.Fatal(err)
	}
	hpBlock, err := aes.NewCipher(hpKey)
	if err != nil {
		t.Fatal(err)
	}
	mask := make([]byte, aes.BlockSize)
	hpBlock.Encrypt(mask, packet[pnOffset+4:pnOffset+4+aes.BlockSize])
	packet[0] ^= mask[0] & 0x0f
	pnLength := int(packet[0]&0x03) + 1
	if pnLength != 4 || packet[0] != 0xc3 {
		t.Fatalf("unprotected first=%02x pn_len=%d", packet[0], pnLength)
	}
	for i := 0; i < pnLength; i++ {
		packet[pnOffset+i] ^= mask[i+1]
	}

	key, err := benchQUICHKDFExpandLabel(clientSecret, "quic key", 16)
	if err != nil {
		t.Fatal(err)
	}
	iv, err := benchQUICHKDFExpandLabel(clientSecret, "quic iv", 12)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	headerEnd := pnOffset + pnLength
	plaintext, err := aead.Open(nil, iv, packet[headerEnd:], packet[:headerEnd])
	if err != nil {
		t.Fatalf("decrypt Initial: %v", err)
	}
	if len(plaintext) < 4 || plaintext[0] != 0x06 || plaintext[1] != 0x00 {
		t.Fatalf("plaintext does not start with CRYPTO offset=0: %x", plaintext[:4])
	}
}

func TestBenchQUICParseVersionNegotiationRequiresCIDMatch(t *testing.T) {
	clientSCID := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	packet := []byte{0x80, 0, 0, 0, 0, byte(len(clientSCID))}
	packet = append(packet, clientSCID...)
	packet = append(packet, 8, 9, 9, 9, 9, 9, 9, 9, 9)
	packet = append(packet, 0, 0, 0, 1, 0x6b, 0x33, 0x43, 0xcf)

	responseType, version, matched, err := benchQUICParseLongHeaderResponse(packet, clientSCID)
	if err != nil {
		t.Fatal(err)
	}
	if responseType != "version-negotiation" || version != "0x00000000" || !matched {
		t.Fatalf("type=%q version=%q matched=%v", responseType, version, matched)
	}

	wrongSCID := []byte{8, 7, 6, 5, 4, 3, 2, 1}
	_, _, matched, err = benchQUICParseLongHeaderResponse(packet, wrongSCID)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("mismatched response DCID was accepted")
	}
}

func TestBenchQUICTransportProfileAndEligibility(t *testing.T) {
	transport, err := normalizeBenchTransport("quic")
	if err != nil {
		t.Fatal(err)
	}
	if transport.Network != "udp" || transport.RemotePort != 443 || transport.MetricScope != "quic-initial-response" {
		t.Fatalf("unexpected QUIC transport: %+v", transport)
	}
	profile := analyzeBenchStrategyProfile(0, []string{
		"--hostlist-domains=example.com",
		"--filter-udp=443",
		"--filter-l7=quic",
		"--payload=quic_initial",
		"--lua-desync=fake:blob=fake_default_quic:repeats=2",
	})
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if !profile.CandidateEligible {
		t.Fatalf("QUIC profile unexpectedly rejected: %v", profile.Reasons)
	}
}

func TestBenchQUICRejectsTLSOnlyCandidate(t *testing.T) {
	transport, err := normalizeBenchTransport("quic")
	if err != nil {
		t.Fatal(err)
	}
	profile := analyzeBenchStrategyProfile(0, []string{
		"--hostlist-domains=example.com",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if profile.CandidateEligible {
		t.Fatal("TLS-only profile was accepted for QUIC/UDP443")
	}
}

func TestV2CustomQUICProfileCompilesSelectionFree(t *testing.T) {
	transport, err := normalizeBenchTransport("quic")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := v2CustomProfileForTransport([]string{
		"--hostlist=/opt/etc/nfqws2/lists/example.list",
		"--filter-udp=443",
		"--filter-l7=quic",
		"--payload=quic_initial",
		"--lua-desync=fake:blob=fake_default_quic:repeats=2",
	}, "example.com", transport)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(profile.Args, " ")
	if strings.Contains(joined, "--hostlist=") {
		t.Fatalf("file-bound selector leaked into custom QUIC profile: %q", joined)
	}
	if !strings.Contains(joined, "--hostlist-domains=example.com") {
		t.Fatalf("exact target selector missing: %q", joined)
	}
}

func TestAllocateBenchUDPPort(t *testing.T) {
	port, err := allocateBenchLocalPortForNetwork("udp")
	if err != nil {
		t.Fatal(err)
	}
	if port < 1024 || port > 65535 {
		t.Fatalf("UDP port=%d", port)
	}
}

func TestV2BenchRequestAcceptsQUICTransport(t *testing.T) {
	req := v2BenchRequest{
		Target:    "example.com",
		Transport: "quic",
		Args: []string{
			"--filter-udp=443",
			"--filter-l7=quic",
			"--payload=quic_initial",
			"--lua-desync=fake:blob=fake_default_quic:repeats=2",
		},
		ExpectedConfigSHA256: strings.Repeat("a", 64),
		Confirm:              v2BenchConfirm,
	}
	if err := validateV2BenchRequest(req); err != nil {
		t.Fatalf("QUIC bench request rejected: %v", err)
	}
}
