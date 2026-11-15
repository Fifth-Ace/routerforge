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

func TestSyslogConfigPreviewBoundedUTF8(t *testing.T) {
	original := strings.Repeat("a", 65535) + "Ж" + "suffix"
	content, truncated := rfSyslogPreview([]byte(original), 65536)
	if !truncated || len(content) != 65535 || !strings.HasSuffix(content, "a") {
		t.Fatal("preview must not split UTF-8 rune")
	}
	content, truncated = rfSyslogPreview([]byte("existing configuration"), 65536)
	if truncated || content != "existing configuration" {
		t.Fatal("short config must be preserved without changes")
	}
	content, truncated = rfSyslogPreview([]byte("nonempty"), 0)
	if !truncated || content != "" {
		t.Fatal("zero-length preview must be empty")
	}
}

func TestSidecarReusesExistingSourcesAndPreservesParent(t *testing.T) {
	parent := []byte("@version: current\nsource src { internal(); unix-dgram(\"/dev/log\"); };\nsource repeater { udp(port(514)); };\n")
	s := rfSyslogDefaults()
	s.RemoteSource = "repeater"
	s.Local = true
	s.LocalSource = "src"
	s.HeadIP = "192.168.10.1"
	snippet, updated, action, err := rfSyslogPreflight(s, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if action != "append-include" || !strings.HasPrefix(string(updated), string(parent)) {
		t.Fatal("foreign config overwritten")
	}
	if strings.Contains(snippet, "unix-dgram(") || strings.Contains(snippet, "network(") {
		t.Fatal("duplicate listener")
	}
	for _, needle := range []string{"source(repeater)", "source(src)", "syslog-entware.log", "syslog-192.168.10.1.log", "${YEAR}/${MONTH}/${DAY}", "netmask("} {
		if !strings.Contains(snippet, needle) {
			t.Fatalf("missing %q", needle)
		}
	}
	_, updated2, action2, err := rfSyslogPreflight(s, updated, []byte(snippet))
	if err != nil || action2 != "unchanged" || string(updated2) != string(updated) {
		t.Fatal("include insertion must be idempotent", err)
	}
}
func TestSidecarRefusesNetworkListenerCollision(t *testing.T) {
	s := rfSyslogDefaults()
	parent := []byte("@version: current\nsource existing { udp(port(514)); };\n")
	_, _, _, err := rfSyslogPreflight(s, parent, nil)
	if err == nil {
		t.Fatal("double UDP listener allowed")
	}
	s.RemoteSource = "existing"
	_, _, _, err = rfSyslogPreflight(s, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
}
func TestSidecarRejectsForeignSidecarAndBadLocalSource(t *testing.T) {
	s := rfSyslogDefaults()
	s.RemoteSource = "foo"
	_, _, _, err := rfSyslogPreflight(s, []byte("@version: current\n"), nil)
	if err == nil {
		t.Fatal("missing existing source allowed")
	}
	s.RemoteSource = ""
	s.Local = true
	s.LocalSource = ""
	if rfSyslogValidate(s) == nil {
		t.Fatal("unbound local source allowed")
	}
	s.LocalSource = "src"
	_, _, _, err = rfSyslogPreflight(s, []byte("@version: current\nsource src { internal(); };\n"), []byte("# foreign"))
	if err == nil {
		t.Fatal("foreign sidecar overwrite allowed")
	}
}
