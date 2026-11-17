package main

import "testing"

func TestP52SmartCacheRequiresHardwareIdentity(t *testing.T) {
	if !doctorSmartCacheAllowed("hw:some-unique-disk") {
		t.Fatal("stable hardware ID must be cacheable")
	}
	if doctorSmartCacheAllowed("fallback:sdb|123|USB") {
		t.Fatal("weak ID must not reuse cached SMART")
	}
}

func TestP52HistoryIdentityClearsOldAlerts(t *testing.T) {
	doc := doctorHistoryDocument{
		Temperatures:   map[string][]doctorTemperaturePoint{"/dev/sdb": {{Celsius: 72}}},
		Smart:          map[string]doctorSmartBaseline{"temp-state:/dev/sdb": {Health: "warning"}},
		DiskIdentities: map[string]string{"/dev/sdb": "hw:old"},
	}
	if !doctorHistoryBindDiskIdentity(&doc, "/dev/sdb", "hw:new") {
		t.Fatal("replacement not observed")
	}
	if len(doc.Temperatures["/dev/sdb"]) != 0 {
		t.Fatal("old temperature points retained")
	}
	if _, ok := doc.Smart["temp-state:/dev/sdb"]; ok {
		t.Fatal("old heat state retained")
	}
	if doctorHistoryBindDiskIdentity(&doc, "/dev/sdb", "hw:new") {
		t.Fatal("same identity should be unchanged")
	}
}
