package supportbundle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestE09SavedDNSProjection(t *testing.T) {
	raw := []byte(`{"status":"inactive","dns_listener":false,"rules":2,"upstreams":1,"saved_dns_policy":{"lists":2,"dnssec_mode":"require_ad","dns64_enabled":true,"nat64_declared":true,"domain":"secret.example","token":"private"}}`)
	got := ProjectHealth("dns-router", raw)
	if got.SavedDNSPolicy == nil || *got.SavedDNSPolicy.Lists != 2 || got.SavedDNSPolicy.DNSSECMode != "require_ad" || !(*got.SavedDNSPolicy.DNS64Enabled) {
		t.Fatalf("%+v", got)
	}
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "secret.example") || strings.Contains(string(b), "private") {
		t.Fatal(string(b))
	}
	if ProjectHealth("dns", raw).SavedDNSPolicy != nil {
		t.Fatal("wrong module")
	}
	for _, s := range []string{`{"lists":65,"dnssec_mode":"off","dns64_enabled":false,"nat64_declared":false}`, `{"lists":0,"dnssec_mode":"secret","dns64_enabled":false,"nat64_declared":false}`, `{"lists":0,"dnssec_mode":"off","dns64_enabled":false,"nat64_declared":true}`, `{"lists":0,"dnssec_mode":"off"}`} {
		if ProjectHealth("dns-router", []byte(`{"saved_dns_policy":`+s+`}`)).SavedDNSPolicy != nil {
			t.Fatal(s)
		}
	}
}
