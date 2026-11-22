package dnsrouter

import "testing"

func TestP242IPHealthConfigValidation(t *testing.T) {
	good := &IPHealthConfig{Enabled: true, Protocol: "tcp", Port: 443, IntervalSeconds: 60, TimeoutMS: 1200, FailureThreshold: 2}
	if e := validateIPHealth(good); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []*IPHealthConfig{
		{Enabled: true, Protocol: "icmp"},
		{Enabled: true, Protocol: "tcp", Port: 22},
		{Enabled: true, Protocol: "tcp", Port: 65536},
		{Enabled: true, Protocol: "tcp", IntervalSeconds: 29},
		{Enabled: true, Protocol: "tcp", IntervalSeconds: 3601},
		{Enabled: true, Protocol: "tcp", TimeoutMS: 249},
		{Enabled: true, Protocol: "tcp", TimeoutMS: 3001},
		{Enabled: true, Protocol: "tcp", FailureThreshold: 6},
	} {
		if e := validateIPHealth(bad); e == nil {
			t.Fatalf("invalid health accepted: %+v", bad)
		}
	}
	if e := validateIPIntelligence(&IPIntelligenceConfig{Enabled: false, Mode: "original", Health: &IPHealthConfig{Enabled: true}}); e == nil {
		t.Fatal("health activated without IP Intelligence")
	}
	if e := validateIPIntelligence(&IPIntelligenceConfig{Enabled: true, Mode: "fastest", Health: good}); e != nil {
		t.Fatal(e)
	}
}
