package dnsrouter

// P36: encrypted inbound DNS. This is a declaration, never a router or
// firewall permission. The service remains disabled until explicit Apply.
import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
)

const P36CertificateRoot = "/opt/etc/routerforge/secure-dns"

type SecureDNSConfig struct {
	Enabled         bool   `json:"enabled"`
	DoTListen       string `json:"dot_listen,omitempty"`
	DoHListen       string `json:"doh_listen,omitempty"`
	DoQListen       string `json:"doq_listen,omitempty"`
	CertificateFile string `json:"certificate_file,omitempty"`
	PrivateKeyFile  string `json:"private_key_file,omitempty"`
	ClientCAFile    string `json:"client_ca_file,omitempty"`
}

func p36SafeCertPath(v string) bool {
	if v == "" || filepath.Clean(v) != v || !filepath.IsAbs(v) || strings.ContainsAny(v, "\x00\r\n") {
		return false
	}
	return strings.HasPrefix(v, P36CertificateRoot+"/") && !strings.Contains(v, "\\")
}

func validateSecureDNSP36(c *SecureDNSConfig, plainListen string) error {
	if c == nil {
		return nil
	}
	if !c.Enabled {
		if c.DoTListen != "" || c.DoHListen != "" || c.DoQListen != "" || c.CertificateFile != "" || c.PrivateKeyFile != "" || c.ClientCAFile != "" {
			return fmt.Errorf("P36 disabled secure DNS must not declare listeners or credentials")
		}
		return nil
	}
	if !p36SafeCertPath(c.CertificateFile) || !p36SafeCertPath(c.PrivateKeyFile) || !p36SafeCertPath(c.ClientCAFile) {
		return fmt.Errorf("P36 certificate, private key, and client CA paths must be direct files under %s", P36CertificateRoot)
	}
	if c.CertificateFile == c.PrivateKeyFile || c.CertificateFile == c.ClientCAFile || c.PrivateKeyFile == c.ClientCAFile {
		return fmt.Errorf("P36 credential files must be distinct")
	}
	plainHost, _, e := net.SplitHostPort(plainListen)
	if e != nil {
		return fmt.Errorf("P36 requires valid plain DNS listen IP: %w", e)
	}
	plainIP, e := netip.ParseAddr(plainHost)
	if e != nil || plainIP.Is4In6() {
		return fmt.Errorf("P36 plain listen must use canonical IP")
	}
	seen := map[string]bool{plainListen: true}
	count := 0
	for _, addr := range []string{c.DoTListen, c.DoHListen, c.DoQListen} {
		if addr == "" {
			continue
		}
		count++
		host, port, e := net.SplitHostPort(addr)
		if e != nil {
			return fmt.Errorf("P36 secure DNS listener must be IP:port: %w", e)
		}
		ip, e := netip.ParseAddr(host)
		portNumber, portErr := strconv.Atoi(port)
		if e != nil || portErr != nil || portNumber < 1025 || portNumber > 65535 || ip.String() != host || ip.Is4In6() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || (!ip.IsPrivate() && !ip.IsLoopback()) {
			return fmt.Errorf("P36 only canonical private/loopback IP and high port allowed: %q", addr)
		}
		if ip != plainIP {
			return fmt.Errorf("P36 encrypted DNS must bind the same local interface as plain DNS")
		}
		if seen[addr] {
			return fmt.Errorf("P36 duplicate or conflicting DNS listen address: %q", addr)
		}
		seen[addr] = true
	}
	if count == 0 {
		return fmt.Errorf("P36 requires at least one explicit DoH/DoT/DoQ listener")
	}
	return nil
}

// ValidateSecureDNSP36 is used by the Lite listener setup after config validation.
func ValidateSecureDNSP36(c *SecureDNSConfig, plainListen string) error {
	return validateSecureDNSP36(c, plainListen)
}
