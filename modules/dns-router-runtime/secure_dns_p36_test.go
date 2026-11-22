package dnsrouter

import (
	"strings"
	"testing"
)

func TestP36DisabledAndMutualTLSRequirements(t *testing.T) {
	base := "192.168.10.1:55358"
	if err := validateSecureDNSP36(nil, base); err != nil {
		t.Fatal(err)
	}
	if err := validateSecureDNSP36(&SecureDNSConfig{}, base); err != nil {
		t.Fatal(err)
	}
	if err := validateSecureDNSP36(&SecureDNSConfig{DoHListen: "192.168.10.1:10443"}, base); err == nil {
		t.Fatal("disabled listener accepted")
	}
	good := &SecureDNSConfig{Enabled: true, DoTListen: "192.168.10.1:10853", DoHListen: "192.168.10.1:10443", DoQListen: "192.168.10.1:10854",
		CertificateFile: P36CertificateRoot + "/server.crt", PrivateKeyFile: P36CertificateRoot + "/server.key", ClientCAFile: P36CertificateRoot + "/clients-ca.crt"}
	if err := validateSecureDNSP36(good, base); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*SecureDNSConfig){
		func(x *SecureDNSConfig) { x.DoHListen = ""; x.DoTListen = ""; x.DoQListen = "" },
		func(x *SecureDNSConfig) { x.DoHListen = "0.0.0.0:10443" },
		func(x *SecureDNSConfig) { x.DoHListen = "8.8.8.8:10443" },
		func(x *SecureDNSConfig) { x.DoTListen = "192.168.10.2:10853" },
		func(x *SecureDNSConfig) { x.DoHListen = base },
		func(x *SecureDNSConfig) { x.DoHListen = x.DoTListen },
		func(x *SecureDNSConfig) { x.DoHListen = "192.168.10.1:443" },
		func(x *SecureDNSConfig) { x.PrivateKeyFile = "/tmp/private.key" },
		func(x *SecureDNSConfig) { x.ClientCAFile = "" },
		func(x *SecureDNSConfig) { x.PrivateKeyFile = x.CertificateFile },
		func(x *SecureDNSConfig) { x.CertificateFile = P36CertificateRoot + "/../elsewhere.pem" },
	} {
		x := *good
		mutate(&x)
		if err := validateSecureDNSP36(&x, base); err == nil {
			t.Fatalf("accepted unsafe P36 config: %+v", x)
		}
	}
	for _, v := range []string{"", "/opt/keys/key", P36CertificateRoot + "/../oops", P36CertificateRoot + "/foo\\bar", P36CertificateRoot + "/"} {
		if p36SafeCertPath(v) {
			t.Fatalf("accepted path %q", v)
		}
	}
	if !strings.HasPrefix(P36CertificateRoot, "/opt/etc/") {
		t.Fatal("P36 path root changed")
	}
}
