package main

import (
	"reflect"
	"testing"
)

func TestP195DNSRouterUpgradeDoesNotForceRemove(t *testing.T) {
	for _, tc := range []struct {
		action, pkg string
		want        []string
	}{
		{"update", "routerforge-dns-router", []string{"install", "/tmp/dns.ipk"}},
		{"install", "routerforge-dns-router", []string{"install", "/tmp/dns.ipk"}},
		{"update", "routerforge-monitoring", []string{"--force-reinstall", "install", "/tmp/dns.ipk"}},
	} {
		got := routerForgeIPKOpkgArgs(tc.action, tc.pkg, "/tmp/dns.ipk")
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%v %v: got %v", tc.action, tc.pkg, got)
		}
	}
}
