package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeAntiscanUnbanTarget(t *testing.T) {
	tests := []struct {
		set, in, want string
		ok            bool
	}{
		{set: "ascn_ips", in: "198.51.100.7", want: "198.51.100.7", ok: true},
		{set: "ascn_honeypot", in: "203.0.113.9", want: "203.0.113.9", ok: true},
		{set: "ascn_subnets", in: "100.64.10.77/24", want: "100.64.10.0/24", ok: true},
		{set: "ascn_subnets", in: "100.64.10.0/16", ok: false},
		{set: "ascn_ndm_lockout", in: "198.51.100.7", ok: false},
		{set: "ascn_ips", in: "2001:db8::1", ok: false},
	}
	for _, tt := range tests {
		got, err := normalizeAntiscanUnbanTarget(tt.set, tt.in)
		if tt.ok && (err != nil || got != tt.want) {
			t.Fatalf("%s %s: got=%q err=%v want=%q", tt.set, tt.in, got, err, tt.want)
		}
		if !tt.ok && err == nil {
			t.Fatalf("%s %s unexpectedly accepted as %q", tt.set, tt.in, got)
		}
	}
}

func TestNormalizeAntiscanListEntry(t *testing.T) {
	if got, err := normalizeAntiscanListEntry(" 100.64.10.77/24 "); err != nil || got != "100.64.10.0/24" {
		t.Fatalf("prefix got=%q err=%v", got, err)
	}
	if got, err := normalizeAntiscanListEntry("198.51.100.8"); err != nil || got != "198.51.100.8" {
		t.Fatalf("ip got=%q err=%v", got, err)
	}
	if _, err := normalizeAntiscanListEntry("example.com"); err == nil {
		t.Fatal("hostname accepted as Antiscan list entry")
	}
}

func TestAppendAntiscanListEntryIsIdempotent(t *testing.T) {
	original := []byte("# trusted phone\n198.51.100.8 note\n")
	got, changed, err := appendAntiscanListEntry(original, "198.51.100.8")
	if err != nil || changed || !bytes.Equal(got, original) {
		t.Fatalf("existing entry got=%q changed=%v err=%v", got, changed, err)
	}
	got, changed, err = appendAntiscanListEntry(original, "100.64.10.0/24")
	if err != nil || !changed || !strings.HasSuffix(string(got), "100.64.10.0/24\n") {
		t.Fatalf("append got=%q changed=%v err=%v", got, changed, err)
	}
}

func TestAntiscanCustomListActive(t *testing.T) {
	cfg := antiscanConfig{UseCustomExcludeList: true, CustomListsBlockMode: "whitelist"}
	if !antiscanCustomListActive(cfg, "exclude") || !antiscanCustomListActive(cfg, "whitelist") {
		t.Fatal("active custom list modes were not detected")
	}
	if antiscanCustomListActive(cfg, "blacklist") {
		t.Fatal("unsupported custom list mode reported active")
	}
}

func TestMutationOnlyRequiresCoreAuthorization(t *testing.T) {
	hit := false
	handler := mutationOnly(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/unban", strings.NewReader("{}"))
	handler(rec, req)
	if rec.Code != http.StatusForbidden || hit {
		t.Fatalf("unauthorized mutation status=%d hit=%v", rec.Code, hit)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/unban", strings.NewReader("{}"))
	req.Header.Set(antiscanMutationAuthHeader, antiscanMutationAuthValue)
	handler(rec, req)
	if rec.Code != http.StatusNoContent || !hit {
		t.Fatalf("authorized mutation status=%d hit=%v", rec.Code, hit)
	}
}

func TestReadAntiscanCustomListRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("198.51.100.1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "ascn_custom_exclude.txt")); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := readAntiscanCustomList(runtimeConfig{AntiscanDir: root}, "ascn_custom_exclude.txt")
	if err == nil {
		t.Fatal("symlink escape was accepted")
	}
}

func TestFindAntiscanSetEntry(t *testing.T) {
	fixture := `Name: ascn_ips
Type: hash:ip
Number of entries: 2
Members:
198.51.100.7 timeout 400 packets 8 bytes 960
203.0.113.9 timeout 20
`
	found, entry, err := findAntiscanSetEntry(strings.NewReader(fixture), "198.51.100.7")
	if err != nil || !found || entry.TimeoutSeconds != 400 || !entry.TimeoutKnown {
		t.Fatalf("found=%v entry=%+v err=%v", found, entry, err)
	}
	found, _, err = findAntiscanSetEntry(strings.NewReader(fixture), "192.0.2.1")
	if err != nil || found {
		t.Fatalf("missing entry found=%v err=%v", found, err)
	}
}
