package lite

import (
	"testing"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func TestP32TransportFactoryHTTP3AndBootstrap(t *testing.T) {
	for _, mode := range []string{"off", "auto", "only"} {
		u, err := transportFactory(nil)(dr.Upstream{
			ID: "p32-h3", Transport: "doh", Address: "9.9.9.9:443",
			ServerName: "dns.quad9.net", Path: "/dns-query",
			HTTP3Mode: mode, BootstrapIPs: []string{"1.1.1.1"},
		})
		if err != nil {
			t.Fatalf("mode=%s: %v", mode, err)
		}
		if err := u.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

func TestP32TransportFactoryDNSCryptStamp(t *testing.T) {
	const stamp = "sdns://AQMAAAAAAAAAETk0LjE0MC4xNC4xNDo1NDQzINErR_JS3PLCu_iZEIbq95zkSV2LFsigxDIuUso_OQhzIjIuZG5zY3J5cHQuZGVmYXVsdC5uczEuYWRndWFyZC5jb20"
	u, err := transportFactory(nil)(dr.Upstream{
		ID: "p32-crypt", Transport: "stamp", Address: "94.140.14.14:5443", Stamp: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Close(); err != nil {
		t.Fatal(err)
	}
}
