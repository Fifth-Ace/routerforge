package dnsrouter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p83Fixture(t *testing.T) (string, Upstream) {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []struct{ name, lines string }{
		{"ndnproxymain.conf", "dns_server = 127.0.0.1:40500 . # 9.9.9.9\ndns_server = 127.0.0.1:40503 ru # common.dot.dns.yandex.net\n"},
		{"ndnproxy_Policy0.conf", "dns_server = 127.0.0.1:40516 . # 9.9.9.9\n"},
		{"ndnproxy_Policy1.conf", "dns_server = 127.0.0.1:40532 . # 9.9.9.9\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.lines), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, Upstream{ID: "native-one", Transport: "native", Address: "native://System/./9.9.9.9"}
}
func p83Inventory(t *testing.T, dir string) NativeDNSInventoryP82 {
	t.Helper()
	v := ReadOnlyNativeDNSInventoryP82(dir)
	if !v.Available {
		t.Fatalf("fixture invalid: %+v", v)
	}
	return v
}
func TestP83ResolvePoliciesWithoutHardcodedPorts(t *testing.T) {
	dir, u := p83Fixture(t)
	inv := p83Inventory(t, dir)
	for _, tc := range []struct{ policy, want string }{{"System", "127.0.0.1:40500"}, {"Policy0", "127.0.0.1:40516"}, {"Policy1", "127.0.0.1:40532"}} {
		u.Address = "native://" + tc.policy + "/./9.9.9.9"
		got, err := ResolveNativeDestinationP83(u, "example.org", inv, "192.168.10.1:55353")
		if err != nil || got != tc.want {
			t.Fatalf("%s: %q %v", tc.policy, got, err)
		}
	}
}
func TestP83ReReadsMovedNativePort(t *testing.T) {
	dir, u := p83Fixture(t)
	old, _ := ResolveNativeDestinationP83(u, "example.org", p83Inventory(t, dir), "127.0.0.1:55353")
	if err := os.WriteFile(filepath.Join(dir, "ndnproxymain.conf"), []byte("dns_server = 127.0.0.1:49123 . # 9.9.9.9\n"), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := ResolveNativeDestinationP83(u, "example.org", p83Inventory(t, dir), "127.0.0.1:55353")
	if err != nil || next != "127.0.0.1:49123" || old == next {
		t.Fatalf("old=%s next=%s err=%v", old, next, err)
	}
}
func TestP83NeverContactsUnknownOrAmbiguousBinding(t *testing.T) {
	dir, u := p83Fixture(t)
	u.Address = "native://System/./not-present"
	if _, err := ResolveNativeDestinationP83(u, "example.org", p83Inventory(t, dir), ""); err == nil {
		t.Fatal("unknown hint accepted")
	}
	u.Address = "native://System/./9.9.9.9"
	if err := os.WriteFile(filepath.Join(dir, "ndnproxymain.conf"), []byte("dns_server = 127.0.0.1:40500 . # 9.9.9.9\ndns_server = 127.0.0.1:40501 . # 9.9.9.9\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveNativeDestinationP83(u, "example.org", p83Inventory(t, dir), ""); err == nil {
		t.Fatal("ambiguous hint accepted")
	}
}
func TestP83DomainIsolationAndProtectedNames(t *testing.T) {
	dir, u := p83Fixture(t)
	u.Address = "native://System/ru/common.dot.dns.yandex.net"
	inv := p83Inventory(t, dir)
	if _, err := ResolveNativeDestinationP83(u, "example.org", inv, ""); err == nil {
		t.Fatal("wrong zone accepted")
	}
	if got, err := ResolveNativeDestinationP83(u, "site.ru", inv, ""); err != nil || got != "127.0.0.1:40503" {
		t.Fatalf("zone %+v %v", got, err)
	}
	u.Address = "native://System/./common.dot.dns.yandex.net"
	if _, err := ResolveNativeDestinationP83(u, "router.lan", inv, ""); err == nil {
		t.Fatal("protected local name accepted")
	}
}
func TestP83RejectsSelfLoopAndInvalidInventory(t *testing.T) {
	dir, u := p83Fixture(t)
	inv := p83Inventory(t, dir)
	if _, err := ResolveNativeDestinationP83(u, "example.org", inv, "127.0.0.1:40500"); err == nil {
		t.Fatal("loop accepted")
	}
	inv.Available = false
	if _, err := ResolveNativeDestinationP83(u, "example.org", inv, ""); err == nil {
		t.Fatal("invalid inventory accepted")
	}
}
func TestP83StrictSchemaAndNoLegacyFallback(t *testing.T) {
	_, u := p83Fixture(t)
	for _, mutation := range []func(*Upstream){
		func(x *Upstream) { x.Address = "native://PolicyBad/./9.9.9.9" }, func(x *Upstream) { x.Address = "native://System/lan/9.9.9.9" },
		func(x *Upstream) { x.Address = "native://System/./bad hint" }, func(x *Upstream) { x.Address = "127.0.0.1:40500" },
		func(x *Upstream) { x.Fallbacks = []string{"other"} }, func(x *Upstream) { x.ServerName = "dns.example" },
	} {
		copy := u
		mutation(&copy)
		if _, err := parseNativeSelectorP83(copy); err == nil {
			t.Fatalf("accepted %+v", copy)
		}
	}
}
func TestP83CompilerAndEncryptedOnlyGroupContract(t *testing.T) {
	_, u := p83Fixture(t)
	cfg := Config{SchemaVersion: 1, Upstreams: []Upstream{u}, Rules: []Rule{{ID: "r", Domain: "example.org", Match: "suffix", Upstream: u.ID}}}
	if _, err := Compile(cfg); err != nil {
		t.Fatalf("valid native rejected: %v", err)
	}
	cfg.Groups = []DNSGroup{{ID: "group", Name: "Strict", Servers: []string{u.ID}, EncryptedOnly: true}}
	cfg.Rules[0].GroupID = "group"
	if _, err := Compile(cfg); err == nil {
		t.Fatal("unproven encrypted native was admitted to encrypted-only group")
	}
}
func TestP83ReadOnlyPreviewCannotActivate(t *testing.T) {
	dir, u := p83Fixture(t)
	cfg := Config{SchemaVersion: 1, Upstreams: []Upstream{u}}
	p := BuildNativeBridgePreviewP83(cfg, p83Inventory(t, dir))
	if p.ActivationAllowed || p.ChangesApplied || p.EncryptionProven || len(p.Bindings) != 1 || !p.Bindings[0].Ready {
		t.Fatalf("preview unexpectedly grants capability: %+v", p)
	}
	if !strings.HasPrefix(p.Bindings[0].Address, "127.0.0.1:") {
		t.Fatal("external address in preview")
	}
}
func TestP83NoNativeWithoutOptIn(t *testing.T) {
	_, u := p83Fixture(t)
	cfg := Config{SchemaVersion: 1, Upstreams: []Upstream{u}}
	_, err := NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55353"})
	if err == nil {
		t.Fatal("native accepted without opt-in")
	}
	_, err = NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55353", NativeBridge: true})
	if err != nil {
		t.Fatalf("explicit native option rejected: %v", err)
	}
}
