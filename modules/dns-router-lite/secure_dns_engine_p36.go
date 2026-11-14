package lite

// P36 encrypted inbound listeners reuse the existing DNSProxy handler, ACL,
// explicit upstream policy, cache and trace path. No secondary resolver.
import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func p36RegularFile(path string, private bool) ([]byte, error) {
	if filepath.Clean(path) != path {
		return nil, fmt.Errorf("P36 unsafe credential path")
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || real != path {
		return nil, fmt.Errorf("P36 credential path must exist without symlinks: %s", path)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("P36 credential file missing or not regular: %s", path)
	}
	if private && info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("P36 private key must not be accessible to group or others")
	}
	if info.Size() <= 0 || info.Size() > 1<<20 {
		return nil, fmt.Errorf("P36 credential size must be 1..1048576 bytes")
	}
	return os.ReadFile(path)
}

func p36MutualTLS(c *dr.SecureDNSConfig) (*tls.Config, error) {
	if c == nil || !c.Enabled {
		return nil, nil
	}
	certBytes, err := p36RegularFile(c.CertificateFile, false)
	if err != nil {
		return nil, err
	}
	keyBytes, err := p36RegularFile(c.PrivateKeyFile, true)
	if err != nil {
		return nil, err
	}
	caBytes, err := p36RegularFile(c.ClientCAFile, false)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("P36 server certificate and key mismatch: %w", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil || time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
		return nil, fmt.Errorf("P36 server certificate not currently valid")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("P36 invalid/empty client CA")
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}, nil
}

func configureInboundP36(dst *proxy.Config, s *dr.ServiceConfig) error {
	if s == nil || s.SecureDNS == nil || !s.SecureDNS.Enabled {
		return nil
	}
	c := s.SecureDNS
	if err := dr.ValidateSecureDNSP36(c, s.Listen); err != nil {
		return err
	}
	tlsCfg, err := p36MutualTLS(c)
	if err != nil {
		return err
	}
	dst.TLSConfig = tlsCfg
	if c.DoTListen != "" {
		addr, e := net.ResolveTCPAddr("tcp", c.DoTListen)
		if e != nil {
			return e
		}
		dst.TLSListenAddr = []*net.TCPAddr{addr}
	}
	if c.DoQListen != "" {
		addr, e := net.ResolveUDPAddr("udp", c.DoQListen)
		if e != nil {
			return e
		}
		dst.QUICListenAddr = []*net.UDPAddr{addr}
	}
	if c.DoHListen != "" {
		addr, e := netip.ParseAddrPort(c.DoHListen)
		if e != nil {
			return e
		}
		dst.HTTPConfig = &proxy.HTTPConfig{
			ListenAddresses: []netip.AddrPort{addr},
			Routes:          []string{"/dns-query"},
			ReadTimeout:     5 * time.Second, WriteTimeout: 8 * time.Second,
			InsecureEnabled: false, HTTP3Enabled: false,
		}
	}
	return nil
}
