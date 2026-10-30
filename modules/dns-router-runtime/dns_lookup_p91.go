package dnsrouter

// P9.1: AWG-style explicit DNS address lookup: user supplies a hostname,
// server returns connectable IPs and, for DoT/DoH, a verified public leaf.
// Discovery is on demand. No DNS configuration, system CA or route is changed.
import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"time"
)

type DNSLookupRequestP91 struct {
	Transport   string `json:"transport"`
	ServerName  string `json:"server_name"`
	PreferredIP string `json:"preferred_ip,omitempty"`
	TLSName     string `json:"tls_name,omitempty"`
}

type DNSLookupResultP91 struct {
	IPs            []string                     `json:"ips"`
	SelectedIP     string                       `json:"selected_ip"`
	Address        string                       `json:"address"`
	ServerName     string                       `json:"server_name"`
	Certificate    *CertificateInspectResultP90 `json:"certificate,omitempty"`
	ChangesApplied bool                         `json:"changes_applied"`
}

var lookupIPsP91 = func(ctx context.Context, name string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", name)
}
var inspectTLSP91 = InspectCertificateP90

func dnsPortP91(transport string) (string, error) {
	switch transport {
	case "udp":
		return "53", nil
	case "doh":
		return "443", nil
	case "dot", "doq":
		return "853", nil
	default:
		return "", errors.New("unsupported DNS type")
	}
}

func LookupDNSServerP91(parent context.Context, req DNSLookupRequestP91) (DNSLookupResultP91, error) {
	out := DNSLookupResultP91{IPs: []string{}, ServerName: req.ServerName}
	port, err := dnsPortP91(req.Transport)
	if err != nil {
		return out, err
	}
	name := strings.TrimSpace(req.ServerName)
	if name == "" || name != req.ServerName {
		return out, errors.New("invalid DNS server hostname")
	}
	tlsName := name
	if req.TLSName != "" {
		tlsName = strings.ToLower(strings.TrimSpace(req.TLSName))
		norm, e := normalizedDomain(tlsName)
		if e != nil || norm != tlsName || !strings.Contains(tlsName, ".") {
			return out, errors.New("invalid TLS name")
		}
	}
	parsedIP := net.ParseIP(name)
	if parsedIP == nil {
		normalized, e := normalizedDomain(name)
		if e != nil || normalized != name || !strings.Contains(name, ".") {
			return out, errors.New("invalid DNS server hostname")
		}
	} else if req.Transport == "doh" || req.Transport == "dot" || req.Transport == "doq" {
		return out, errors.New("encrypted DNS needs a hostname for TLS SNI")
	}
	ctx, cancel := context.WithTimeout(parent, 9*time.Second)
	defer cancel()
	ips := []net.IP{}
	if parsedIP != nil {
		ips = append(ips, parsedIP)
	} else {
		ips, err = lookupIPsP91(ctx, name)
		if err != nil {
			return out, fmt.Errorf("DNS lookup %s: %w", name, err)
		}
	}
	seen := map[string]bool{}
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		addr, e := netip.ParseAddr(ip.String())
		if e != nil || addr.Is4In6() {
			continue
		}
		str := addr.String()
		if seen[str] {
			continue
		}
		if req.Transport == "dot" || req.Transport == "doh" {
			if publicDialAddressP90(net.JoinHostPort(str, port)) != nil {
				continue
			}
		} else if !addr.IsGlobalUnicast() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
			continue
		}
		seen[str] = true
		out.IPs = append(out.IPs, str)
	}
	// Stable presentation, IPv4 first; never persist the DNS response as a trust decision.
	sort.Slice(out.IPs, func(i, j int) bool {
		a := strings.Contains(out.IPs[i], ":")
		b := strings.Contains(out.IPs[j], ":")
		if a != b {
			return !a
		}
		return out.IPs[i] < out.IPs[j]
	})
	if len(out.IPs) > 16 {
		out.IPs = out.IPs[:16]
	}
	if len(out.IPs) == 0 {
		return out, errors.New("DNS server has no usable public IP")
	}
	if req.PreferredIP != "" {
		found := false
		for _, ip := range out.IPs {
			if ip == req.PreferredIP {
				found = true
				break
			}
		}
		if !found {
			return out, errors.New("selected IP is not in current DNS lookup")
		}
		out.IPs = append([]string{req.PreferredIP}, append([]string{}, out.IPs...)...)
		// Deduplicate preferred IP after putting it first.
		for i := 1; i < len(out.IPs); i++ {
			if out.IPs[i] == req.PreferredIP {
				out.IPs = append(out.IPs[:i], out.IPs[i+1:]...)
				break
			}
		}
	}
	if req.Transport == "udp" || req.Transport == "doq" {
		out.SelectedIP = out.IPs[0]
		out.Address = net.JoinHostPort(out.SelectedIP, port)
		return out, nil
	}
	// A resolved IP is not enough: inspect each candidate until one presents
	// a valid certificate for SNI. Failure does not disable verification.
	var lastErr error
	for _, ip := range out.IPs {
		if ctx.Err() != nil {
			break
		}
		address := net.JoinHostPort(ip, port)
		cert, e := inspectTLSP91(ctx, CertificateInspectRequestP90{
			Transport: req.Transport, Address: address, ServerName: tlsName,
		})
		if e != nil {
			lastErr = e
			continue
		}
		if !cert.Verified || cert.PEM == "" {
			lastErr = errors.New("TLS certificate not verified")
			continue
		}
		out.SelectedIP = ip
		out.Address = address
		out.Certificate = &cert
		return out, nil
	}
	if lastErr == nil {
		lastErr = ctx.Err()
	}
	return out, fmt.Errorf("no DNS IP passed TLS verification: %w", lastErr)
}
