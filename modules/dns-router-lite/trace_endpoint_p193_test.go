package lite

import (
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"testing"
)

func TestP193TraceEndpointIdentity(t *testing.T) {
	tests := []struct {
		u    dr.Upstream
		want string
	}{
		{dr.Upstream{ID: "primary", Transport: "udp", Address: "9.9.9.9:53"}, "9.9.9.9:53"},
		{dr.Upstream{ID: "backup", Transport: "dot", Address: "1.1.1.1:853", ServerName: "one.one.one.one"}, "one.one.one.one (1.1.1.1:853)"},
		{dr.Upstream{ID: "sec", Transport: "doh", Address: "8.8.8.8:443", ServerName: "dns.google"}, "dns.google (8.8.8.8:443)"},
	}
	for _, tc := range tests {
		if got := traceDNSLabel(tc.u); got != tc.want {
			t.Errorf("traceDNSLabel(%s)=%q want=%q", tc.u.ID, got, tc.want)
		}
	}
}
