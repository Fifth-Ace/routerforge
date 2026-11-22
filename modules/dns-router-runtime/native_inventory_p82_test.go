package dnsrouter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeP82(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestP82RealDSTestThreeInstances(t *testing.T) {
	dir := t.TempDir()
	for idx, name := range []string{"ndnproxymain.conf", "ndnproxy_Policy0.conf", "ndnproxy_Policy1.conf"} {
		p := 40500 + idx*16
		var b strings.Builder
		for j, zone := range []string{".", ".", ".", "ru", "su", "xn--p1ai"} {
			b.WriteString("dns_server = 127.0.0.1:" + itoaP82(p+j) + " " + zone + " # common.dot.dns.yandex.net\n")
		}
		writeP82(t, dir, name, b.String())
	}
	r := ReadOnlyNativeDNSInventoryP82(dir)
	if !r.Available || r.ParsedInstances != 3 || r.LocalEndpoints != 18 {
		t.Fatalf("inventory: %+v", r)
	}
	if r.ReuseVerified || r.ActivationAllowed || r.NDMCInvoked || r.ChangesApplied {
		t.Fatal("read-only result improperly elevated to apply permission")
	}
	if r.Instances[0].Name != "System" || r.Instances[1].Name != "Policy0" || r.Instances[2].Name != "Policy1" {
		t.Fatalf("identity drift: %+v", r.Instances)
	}
	if r.Instances[1].Endpoints[3].Address != "127.0.0.1:40519" || r.Instances[2].Endpoints[5].Domain != "xn--p1ai" {
		t.Fatal("real port or domain mapping wrong")
	}
}

func TestP82RejectForeignOrLowPort(t *testing.T) {
	for _, bad := range []string{"192.168.10.1:40500", "127.0.0.1:53", "127.0.0.1:65536", "[::1]:40500"} {
		t.Run(bad, func(t *testing.T) {
			i := parseNativeConfigP82("System", "test", []byte("dns_server = "+bad+" . # upstream\n"))
			if i.Valid {
				t.Fatalf("accepted forbidden endpoint %q", bad)
			}
		})
	}
}

func TestP82RejectDuplicateCrossPolicy(t *testing.T) {
	dir := t.TempDir()
	writeP82(t, dir, "ndnproxymain.conf", "dns_server = 127.0.0.1:40500 .\n")
	writeP82(t, dir, "ndnproxy_Policy0.conf", "dns_server = 127.0.0.1:40500 ru\n")
	r := ReadOnlyNativeDNSInventoryP82(dir)
	if r.Available || r.Instances[1].Valid {
		t.Fatalf("reused port accepted: %+v", r)
	}
}

func TestP82MissingSystemAndSymlink(t *testing.T) {
	dir := t.TempDir()
	r := ReadOnlyNativeDNSInventoryP82(dir)
	if r.Available || r.ReuseVerified {
		t.Fatal("absent inventory accepted")
	}
	writeP82(t, dir, "target", "dns_server = 127.0.0.1:40500 .\n")
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "ndnproxymain.conf")); err != nil {
		t.Skip(err)
	}
	r = ReadOnlyNativeDNSInventoryP82(dir)
	if r.Available {
		t.Fatal("symlink config accepted")
	}
}

func TestP82InvalidLinesAndSafeHints(t *testing.T) {
	i := parseNativeConfigP82("System", "test", []byte("dns_server = 127.0.0.1:40500 ru # upstream\ndns_server = 127.0.0.1:40500 ru\n"))
	if i.Valid || len(i.Endpoints) != 1 {
		t.Fatal("duplicate exact DNS entry was accepted")
	}
	i = parseNativeConfigP82("System", "test", []byte("dns_server = 127.0.0.1:40500 ru # valid.name\n"))
	if !i.Valid || i.Endpoints[0].UpstreamHint != "valid.name" {
		t.Fatalf("expected safe native hint: %+v", i)
	}
	i = parseNativeConfigP82("System", "test", []byte("dns_server = 127.0.0.1:40500 Ru # bad\n"))
	if i.Valid {
		t.Fatal("accepted noncanonical zone")
	}
}

func TestP82SharedLocalServerDifferentZonesAllowed(t *testing.T) {
	i := parseNativeConfigP82("System", "test", []byte("dns_server = 127.0.0.1:40500 ru\ndns_server = 127.0.0.1:40500 su\n"))
	if !i.Valid || len(i.Endpoints) != 2 {
		t.Fatalf("one native DNS listener can serve multiple zones: %+v", i)
	}
}

func TestP82NeverReturnsRawConfig(t *testing.T) {
	dir := t.TempDir()
	writeP82(t, dir, "ndnproxymain.conf", "password = secret-never-expose\ndns_server = 127.0.0.1:40500 . # 9.9.9.9\n")
	r := ReadOnlyNativeDNSInventoryP82(dir)
	if !r.Available {
		t.Fatal("valid endpoint refused")
	}
	if strings.Contains(r.Instances[0].Source, "secret") || len(r.Instances[0].Endpoints) != 1 {
		t.Fatal("unexpected inventory result")
	}
}

func itoaP82(n int) string {
	// Fixed-width test fixture: all observed values are between 40500 and 40537.
	const digits = "0123456789"
	return string([]byte{digits[n/10000], digits[(n/1000)%10], digits[(n/100)%10], digits[(n/10)%10], digits[n%10]})
}
