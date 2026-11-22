package dnsrouter

// P9.0: explicit, bounded TLS certificate inspection for a manually configured
// DoT/DoH server. No DNS bootstrap, trust-store mutation or HTTP request.
// Always uses the ordinary verified TLS handshake; a failed identity or chain
// returns an error, not a trusted certificate. The returned PEM is public.
import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const maxCertificatePEMP90 = 16384

type CertificateInspectRequestP90 struct {
	Transport  string `json:"transport"`
	Address    string `json:"address"`
	ServerName string `json:"server_name"`
}

type CertificateInspectResultP90 struct {
	Verified          bool     `json:"verified"`
	Transport         string   `json:"transport"`
	ServerName        string   `json:"server_name"`
	Subject           string   `json:"subject"`
	Issuer            string   `json:"issuer"`
	DNSNames          []string `json:"dns_names"`
	NotBefore         string   `json:"not_before"`
	NotAfter          string   `json:"not_after"`
	FingerprintSHA256 string   `json:"fingerprint_sha256"`
	PEM               string   `json:"pem"`
	TLSVersion        string   `json:"tls_version"`
	ChangesApplied    bool     `json:"changes_applied"`
}

// Reject non-public ranges even where IsGlobalUnicast returns true (RFC 6890).
func publicDialAddressP90(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("requires explicit IP:port, no hostname bootstrap")
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || addr.Zone() != "" || !addr.Is4() {
		return errors.New("requires a literal public IPv4 address without scope")
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1 || n > 65535 {
		return errors.New("invalid TCP port")
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return errors.New("non-public IP address is not permitted")
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001:db8::/32", "2001::/23", "64:ff9b:1::/48"} {
		block := netip.MustParsePrefix(cidr)
		if block.Contains(addr) {
			return errors.New("reserved or special-use IP is not permitted")
		}
	}
	return nil
}

func validateCertRequestP90(req CertificateInspectRequestP90) error {
	if req.Transport != "dot" && req.Transport != "doh" {
		return errors.New("only DoT and DoH certificate inspection is supported")
	}
	if name, err := normalizedDomain(req.ServerName); err != nil || name != req.ServerName || !strings.Contains(req.ServerName, ".") {
		return errors.New("invalid TLS server name")
	}
	if err := publicDialAddressP90(req.Address); err != nil {
		return err
	}
	_, port, _ := net.SplitHostPort(req.Address)
	if (req.Transport == "doh" && port != "443") || (req.Transport == "dot" && port != "853") {
		return errors.New("only standard DoH 443 / DoT 853 endpoints are supported")
	}
	return nil
}

func InspectCertificateP90(parent context.Context, req CertificateInspectRequestP90) (CertificateInspectResultP90, error) {
	out := CertificateInspectResultP90{Transport: req.Transport, ServerName: req.ServerName, DNSNames: []string{}, ChangesApplied: false}
	if err := validateCertRequestP90(req); err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", req.Address)
	if err != nil {
		return out, fmt.Errorf("TLS TCP connect: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	peer := tls.Client(conn, &tls.Config{ServerName: req.ServerName, MinVersion: tls.VersionTLS12})
	defer peer.Close()
	// This handshake validates CA chain, hostname/SAN and validity using the system roots.
	if err := peer.HandshakeContext(ctx); err != nil {
		return out, fmt.Errorf("TLS certificate verification failed: %w", err)
	}
	state := peer.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return out, errors.New("server presented no certificate")
	}
	leaf := state.PeerCertificates[0]
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})
	if len(pemBytes) > maxCertificatePEMP90 {
		return out, errors.New("certificate exceeds size limit")
	}
	sum := sha256.Sum256(leaf.Raw)
	out.Verified = true
	out.Subject = leaf.Subject.String()
	out.Issuer = leaf.Issuer.String()
	out.DNSNames = append([]string{}, leaf.DNSNames...)
	if len(out.DNSNames) > 64 {
		out.DNSNames = out.DNSNames[:64]
	}
	out.NotBefore = leaf.NotBefore.UTC().Format(time.RFC3339)
	out.NotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
	out.FingerprintSHA256 = hex.EncodeToString(sum[:])
	out.PEM = string(pemBytes)
	out.TLSVersion = tls.VersionName(state.Version)
	return out, nil
}

// The stored public leaf is a snapshot for display/export, not an alternate CA.
func validateSavedCertificateP90(transport, serverName, value string) error {
	if value == "" {
		return nil
	}
	if transport != "dot" && transport != "doh" {
		return errors.New("saved certificate supported for DoT/DoH only")
	}
	if len(value) > maxCertificatePEMP90 {
		return errors.New("saved certificate is too large")
	}
	block, extra := pem.Decode([]byte(value))
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(extra))) != 0 || len(block.Headers) != 0 {
		return errors.New("saved PEM must contain exactly one public certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("saved certificate invalid: %w", err)
	}
	if err = cert.VerifyHostname(serverName); err != nil {
		return errors.New("saved certificate does not match TLS server name")
	}
	return nil
}
