package lite

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func TestP36DisabledDoesNotOpenTLSOrHTTP(t *testing.T) {
	cfg := &proxy.Config{}
	if e := configureInboundP36(cfg, &dr.ServiceConfig{Listen: "127.0.0.1:55358"}); e != nil {
		t.Fatal(e)
	}
	if cfg.TLSConfig != nil || cfg.HTTPConfig != nil || len(cfg.TLSListenAddr) != 0 || len(cfg.QUICListenAddr) != 0 {
		t.Fatal("P36 disabled altered DNS listener")
	}
}
func TestP36RequiresRealCredentialFilesAndNoSymlinks(t *testing.T) {
	cfg := &proxy.Config{}
	c := &dr.SecureDNSConfig{Enabled: true, DoTListen: "127.0.0.1:10853",
		CertificateFile: dr.P36CertificateRoot + "/no-cert.pem", PrivateKeyFile: dr.P36CertificateRoot + "/no-key.pem", ClientCAFile: dr.P36CertificateRoot + "/no-ca.pem"}
	if e := configureInboundP36(cfg, &dr.ServiceConfig{Listen: "127.0.0.1:55358", SecureDNS: c}); e == nil {
		t.Fatal("enabled without cert files")
	}
	if cfg.TLSConfig != nil {
		t.Fatal("half-configured TLS listener")
	}
	if runtime.GOOS != "windows" {
		tmp := t.TempDir()
		a := filepath.Join(tmp, "a")
		b := filepath.Join(tmp, "b")
		if err := os.WriteFile(a, []byte("key"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(a, b); err != nil {
			t.Fatal(err)
		}
		if _, err := p36RegularFile(b, true); err == nil {
			t.Fatal("symlinked key accepted")
		}
		if err := os.Chmod(a, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := p36RegularFile(a, true); err == nil {
			t.Fatal("group-readable private key accepted")
		}
	}
}
