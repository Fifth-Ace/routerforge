package dnsrouter

import "testing"

func TestP27ConfigProtectionValidation(t *testing.T) {
	for _, good := range []*QueryProtectionConfig{
		nil, {}, {RefuseAny: true}, {RateLimitRPS: 25}, {BogusNXDomain: []string{"0.0.0.0", "::/128"}},
	} {
		if err := validateQueryProtection(good); err != nil {
			t.Fatalf("good protection rejected: %+v %v", good, err)
		}
	}
	for _, bad := range []*QueryProtectionConfig{
		{RateLimitRPS: -1}, {RateLimitRPS: 2001}, {BogusNXDomain: []string{"not-ip"}},
		{BogusNXDomain: []string{"0.0.0.0/0"}}, {BogusNXDomain: []string{"192.168.0.0/16"}},
		{BogusNXDomain: []string{"8.8.8.8", "8.8.8.8/32"}},
	} {
		if err := validateQueryProtection(bad); err == nil {
			t.Fatalf("invalid protection accepted: %+v", bad)
		}
	}
}
