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
	for _, needle := range []string{"source(repeater)", "source(src)", "syslog-entware.log", "syslog-192.168.10.1.log", "${R_YEAR}/${R_MONTH}/${R_DAY}", "netmask("} {
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

func TestP454RotationAndDeviceNames(t *testing.T) {
	s := rfSyslogDefaults()
	s.RotateHours = 6
	s.RemoteSource = "repeater"
	s.DeviceNames = map[string]string{"192.168.10.1": "Главный_Keenetic", "192.168.10.2": "Spalnya"}
	parent := []byte("@version: current\nsource repeater { network(ip(\"0.0.0.0\") port(514) transport(\"udp\")); };\n")
	snippet, _, _, err := rfSyslogPreflight(s, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"syslog-Главный_Keenetic-00-06.log", "syslog-Spalnya-18-24.log", "${R_YEAR}/${R_MONTH}/${R_DAY}", "R_HOUR", "create_dirs(no)", "${SOURCEIP}-06-12.log"} {
		if !strings.Contains(snippet, needle) {
			t.Fatalf("missing rotation component %q", needle)
		}
	}
	if strings.Contains(snippet, "source s_routerforge_net") {
		t.Fatal("duplicate listener")
	}
	if strings.Count(snippet, "destination d_routerforge_named_0_") != 4 {
		t.Fatal("wrong number of 6h files")
	}
}
func TestP454IntervalsAndNamesRejectUnsafeInputs(t *testing.T) {
	for _, hours := range []int{1, 6, 12, 24} {
		s := rfSyslogDefaults()
		s.RotateHours = hours
		if err := rfSyslogValidate(s); err != nil {
			t.Fatal(hours, err)
		}
		snippet := rfSyslogSnippetEasy(s)
		want := 24 / hours
		if strings.Count(snippet, "destination d_routerforge_other_") != want {
			t.Fatalf("interval %d: expected %d buckets", hours, want)
		}
	}
	s := rfSyslogDefaults()
	s.RotateHours = 5
	if rfSyslogValidate(s) == nil {
		t.Fatal("unsupported bucket allowed")
	}
	s = rfSyslogDefaults()
	s.DeviceNames = map[string]string{"192.168.10.2": "../../etc/passwd"}
	if rfSyslogValidate(s) == nil {
		t.Fatal("unsafe filename allowed")
	}
	s = rfSyslogDefaults()
	s.DeviceNames = map[string]string{"192.168.10.2": "Bedroom", "192.168.10.3": "bedroom"}
	if rfSyslogValidate(s) == nil {
		t.Fatal("case-colliding names allowed")
	}
}
func TestP454RejectKernelAsNetworkSource(t *testing.T) {
	parent := []byte("@version: current\nsource kernel { file(\"/proc/kmsg\"); };\nsource src { internal(); unix-dgram(\"/dev/log\"); };\nsource net { network(ip(\"0.0.0.0\") port(514)); };\n")
	kind := map[string]string{}
	for _, item := range rfSyslogSourceDetails(parent) {
		kind[item.Name] = item.Kind
	}
	if kind["kernel"] != "kernel" || kind["src"] != "local" || kind["net"] != "network" {
		t.Fatal(kind)
	}
	s := rfSyslogDefaults()
	s.RemoteSource = "kernel"
	_, _, _, err := rfSyslogPreflight(s, parent, nil)
	if err == nil {
		t.Fatal("kernel was accepted as network listener")
	}
	s.RemoteSource = "net"
	if _, _, _, err = rfSyslogPreflight(s, parent, nil); err != nil {
		t.Fatal(err)
	}
}
func TestP454LogFilenameSecurity(t *testing.T) {
	for _, filename := range []string{"syslog-Главный_Keenetic-00-06.log", "syslog-192.168.10.5-21-22.log", "syslog-entware.log"} {
		if !rfSyslogFile.MatchString(filename) {
			t.Fatalf("legitimate filename refused: %s", filename)
		}
	}
	for _, filename := range []string{"../syslog-evil.log", "syslog-hello/../../bad.log", "syslog-hi;rm.log"} {
		if rfSyslogFile.MatchString(filename) {
			t.Fatalf("unsafe filename accepted: %s", filename)
		}
	}
}

// Changing user-facing release labels must not orphan pre-existing sidecar configs.
func TestSyslogVersionlessMarkerAndLegacyCompatibility(t *testing.T) {
	if strings.Contains(rfSyslogSidecarMarker, "P45") {
		t.Fatal("release label leaked into generated config")
	}
	for _, marker := range []string{rfSyslogSidecarMarker, rfSyslogLegacySidecarMarker} {
		if !rfSyslogManagedSidecar([]byte(marker + "\nsource example { internal(); };\n")) {
			t.Fatalf("owned sidecar rejected: %q", marker)
		}
	}
	if rfSyslogManagedSidecar([]byte("# THIRD PARTY\n")) {
		t.Fatal("foreign sidecar accepted")
	}
	s := rfSyslogDefaults()
	if !strings.HasPrefix(rfSyslogSnippet(s), rfSyslogSidecarMarker+"\n") {
		t.Fatal("new marker not emitted")
	}
}

func TestSyslogInitAutostartStatus(t *testing.T) {
	cases := []struct{ id, script, want string }{
		{"S01syslog-ng", "#!/bin/sh\nENABLED=yes\n. /opt/etc/init.d/rc.func", "enabled"},
		{"S01syslog-ng", "ENABLED='no' # disabled\n", "disabled"},
		{"S01syslog-ng", "ENABLED=true\n", "enabled"},
		{"S01syslog-ng", "#!/bin/sh\n# ENABLED=yes\n", "unknown"},
		{"K01syslog-ng", "ENABLED=yes\n", "disabled"},
		{"", "ENABLED=yes\n", "unknown"},
	}
	for _, c := range cases {
		if got := rfSyslogAutostartFromScript(c.id, []byte(c.script)); got != c.want {
			t.Errorf("%s: got %s want %s", c.id, got, c.want)
		}
	}
}
