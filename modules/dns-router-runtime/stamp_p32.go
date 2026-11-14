package dnsrouter

// P32 DNS stamps are accepted only with an embedded, literal server address.
// This prevents dnsproxy from silently consulting the system resolver.
import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

func DNSStampEndpointP32(stamp string) (string, error) {
	if !strings.HasPrefix(stamp, "sdns://") || len(stamp) > 4096 {
		return "", errors.New("DNS stamp requires a bounded sdns:// URL")
	}
	encoded := strings.TrimPrefix(stamp, "sdns://")
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(encoded, "="))
	if err != nil || len(b) < 11 {
		return "", errors.New("invalid DNS stamp encoding")
	}
	// DNS stamps: protocol byte, eight property bytes, length-prefixed address.
	port := "443"
	switch b[0] {
	case 0x01, 0x02: // DNSCrypt, DoH
	case 0x03, 0x04: // DoT, DoQ
		port = "853"
	default:
		return "", errors.New("DNS stamp protocol is not supported")
	}
	n := int(b[9])
	if n == 0 || 10+n >= len(b) { // also require provider/protocol material after address
		return "", errors.New("DNS stamp must contain a pinned server IP")
	}
	// DNSProxy v0.86.0 does not pass stamp TLS hashes to newDoH/DoT/DoQ.
	// Reject hashed stamps rather than silently dropping their pinning policy.
	if b[0] != 0x01 && b[10+n] != 0 {
		return "", errors.New("TLS certificate-pinned stamps need explicit pin verification")
	}
	endpoint := string(b[10 : 10+n])
	host, suppliedPort, e := net.SplitHostPort(endpoint)
	if e != nil {
		host, suppliedPort = endpoint, port
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		}
	}
	ip, e := netip.ParseAddr(host)
	if e != nil || !p32PublicIP(ip) {
		return "", errors.New("DNS stamp server must be a public unicast IP")
	}
	p, e := strconv.Atoi(suppliedPort)
	if e != nil || p <= 0 || p > 65535 {
		return "", errors.New("invalid DNS stamp port")
	}
	return net.JoinHostPort(ip.String(), fmt.Sprint(p)), nil
}

func validateDNSStampP32(stamp, address string) error {
	endpoint, err := DNSStampEndpointP32(stamp)
	if err != nil {
		return err
	}
	if endpoint != address {
		return errors.New("DNS stamp address and pinned upstream address differ")
	}
	return nil
}

func validateP32Options(u Upstream) error {
	if u.Transport == "stamp" {
		if err := validateDNSStampP32(u.Stamp, u.Address); err != nil {
			return err
		}
		if u.ServerName != "" || u.Path != "" || u.HTTP3Mode != "" || len(u.BootstrapIPs) > 0 {
			return errors.New("stamp owns its identity, protocol, and pinned bootstrap")
		}
	} else if u.Stamp != "" {
		return errors.New("DNS stamp is only allowed for stamp transport")
	}
	if u.Transport != "doh" && u.HTTP3Mode != "" {
		return errors.New("HTTP/3 mode is only allowed for DoH")
	}
	switch u.HTTP3Mode {
	case "", "off", "auto", "only":
	default:
		return errors.New("unsupported HTTP/3 mode")
	}
	if len(u.BootstrapIPs) > 4 {
		return errors.New("at most four extra bootstrap addresses are allowed")
	}
	if len(u.BootstrapIPs) > 0 && u.Transport != "doh" && u.Transport != "dot" && u.Transport != "doq" {
		return errors.New("bootstrap IPs are only allowed for encrypted DNS")
	}
	seen := make(map[netip.Addr]bool)
	for _, raw := range u.BootstrapIPs {
		ip, err := netip.ParseAddr(raw)
		if err != nil || ip.String() != raw || !p32PublicIP(ip) {
			return errors.New("bootstrap address must be a canonical public IP")
		}
		if seen[ip] {
			return errors.New("duplicate bootstrap IP")
		}
		seen[ip] = true
	}
	return nil
}

// Explicitly deny RFC 6890 special ranges that Go marks global unicast.
func p32PublicIP(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.Is4In6() {
		return false
	}
	v4 := []string{"0.0.0.0/8", "100.64.0.0/10", "169.254.0.0/16", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"}
	v6 := []string{"64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8"}
	ranges := v4
	if ip.Is6() {
		ranges = v6
	}
	for _, raw := range ranges {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}
