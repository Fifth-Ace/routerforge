package main

import (
	"strings"
	"testing"
)

func TestSyslogDefaultSettings(t *testing.T) {
	s := rfSyslogDefaults()
	if err := rfSyslogValidate(s); err != nil {
		t.Fatal(err)
	}
	out := rfSyslogConfigText(s)
	for _, word := range []string{rfSyslogMarker, "network(", "filter f_rf_allowed", "syslog-${SOURCEIP}.log"} {
		if !strings.Contains(out, word) {
			t.Fatalf("missing %s", word)
		}
	}
	if strings.Contains(out, "/dev/log") {
		t.Fatal("local log capture is enabled without consent")
	}
}
func TestSyslogInputs(t *testing.T) {
	s := rfSyslogDefaults()
	for _, root := range []string{"/etc/passwd", "/opt/mnt/a/../../routerforge-syslog", "/tmp/routerforge-syslog"} {
		s.Root = root
		if rfSyslogValidate(s) == nil {
			t.Fatalf("accepted unsafe root %q", root)
		}
	}
	s = rfSyslogDefaults()
	s.Allowed = []string{"0.0.0.0/0;include \"evil\""}
	if rfSyslogValidate(s) == nil {
		t.Fatal("accepted injection")
	}
}
func TestSyslogNoDuplicateNetworkLog(t *testing.T) {
	s := rfSyslogDefaults()
	s.TCP = true
	out := rfSyslogConfigText(s)
	if strings.Count(out, "destination(d_rf_remote)") != 1 {
		t.Fatal("network messages can be duplicated")
	}
	if strings.Count(out, "network(") != 2 {
		t.Fatal("UDP/TCP missing")
	}
}
func TestSyslogManagedMarker(t *testing.T) {
	if rfSyslogOwned([]byte("@version: current")) {
		t.Fatal("unmanaged accepted")
	}
	if !rfSyslogOwned([]byte("@version: current\n" + rfSyslogMarker + "\n")) {
		t.Fatal("managed not accepted")
	}
}
func TestSyslogSafeFileNames(t *testing.T) {
	s := rfSyslogDefaults()
	for _, name := range []string{"../../passwd", "1.2.3.4/evil", "..", "bad$name"} {
		_, err := rfSyslogSafeFile(s, map[string][]string{"date": {"2026-11-15"}, "source": {name}})
		if err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}
