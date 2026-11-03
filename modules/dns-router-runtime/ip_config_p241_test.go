package dnsrouter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestP241IPIntelligenceModeValidation(t *testing.T) {
	accepted := []string{"", "original", "round_robin", "random", "fastest", "health_aware", "adaptive", "sticky", "weighted", "healthy_only"}
	for _, mode := range accepted {
		if err := validateIPIntelligence(&IPIntelligenceConfig{Enabled: true, Mode: mode}); err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
	}
	for _, mode := range []string{"unsafe", "fastest_ip", "ALL", "original;echo"} {
		if err := validateIPIntelligence(&IPIntelligenceConfig{Enabled: true, Mode: mode}); err == nil {
			t.Fatalf("invalid mode %q accepted", mode)
		}
	}
	if err := validateIPIntelligence(nil); err != nil {
		t.Fatal("old config must remain valid", err)
	}
}
func TestP241OldServiceConfigHasNoNewKey(t *testing.T) {
	b, err := json.Marshal(ServiceConfig{Listen: "127.0.0.1:5301"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "ip_intelligence") {
		t.Fatal("default adds a migration key")
	}
	var restored ServiceConfig
	if err := json.Unmarshal(b, &restored); err != nil || restored.IPIntelligence != nil {
		t.Fatal("old config upgraded unexpectedly", err)
	}
	modern := ServiceConfig{IPIntelligence: &IPIntelligenceConfig{Enabled: true, Mode: "round_robin"}}
	b, err = json.Marshal(modern)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"ip_intelligence":{"enabled":true,"mode":"round_robin"}`) {
		t.Fatal(string(b))
	}
}
