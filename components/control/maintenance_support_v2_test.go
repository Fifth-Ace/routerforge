package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	sb "github.com/Fifth-Ace/routerforge/internal/platform/supportbundle"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func supportFixturePrepared(t *testing.T, now time.Time) *supportPrepared {
	t.Helper()
	p, e := supportMakePrepared(now, []string{"logs"}, nil, []sb.Entry{{ID: "logs", State: "ok", Data: sb.JSON(sb.SummarizeLogs([]byte("error secret-value")))}})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestE07PreviewTokenAndExactArchive(t *testing.T) {
	now := time.Now()
	p := supportFixturePrepared(t, now)
	if _, e := supportTakePreview(now, p, p.Token); e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		at    time.Time
		token string
	}{{now, "bad"}, {now.Add(supportPreviewTTL), p.Token}} {
		if _, e := supportTakePreview(x.at, p, x.token); !errors.Is(e, supportErrExpired) {
			t.Fatal(e)
		}
	}
	root := filepath.Join(t.TempDir(), "private")
	row, e := supportPublish(root, p, now)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(row.Path)
	if e != nil || !bytes.Equal(b, p.raw) || sb.Hash(b) != p.SHA256 {
		t.Fatal("exact preview not saved", e)
	}
	if _, e = supportPublish(root, p, now); e == nil {
		t.Fatal("collision accepted")
	}
	rows, e := supportInventory(root)
	if e != nil || len(rows) != 1 {
		t.Fatal(rows, e)
	}
	if _, e = supportTakePreview(now, nil, p.Token); e == nil {
		t.Fatal("consumed preview accepted")
	}
}
func TestE07RetentionAndSymlinks(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	for i := 1; i <= adminSupportBundleMaxFiles; i++ {
		if e := os.WriteFile(filepath.Join(root, fmt.Sprintf("routerforge-support-%d.zip", i)), []byte("legacy"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	p := supportFixturePrepared(t, time.Now())
	if _, e := supportPublish(root, p, time.Now()); !errors.Is(e, supportErrFull) {
		t.Fatal(e)
	}
	rows, e := supportInventory(root)
	if e != nil || len(rows) != adminSupportBundleMaxFiles {
		t.Fatal(rows, e)
	}
	for _, r := range rows {
		b, e := os.ReadFile(r.Path)
		if e != nil || string(b) != "legacy" {
			t.Fatal(r, e)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if e = os.WriteFile(outside, []byte("secret"), 0600); e != nil {
		t.Fatal(e)
	}
	name := "routerforge-support-v2-99.zip"
	if e = os.Symlink(outside, filepath.Join(root, name)); e != nil {
		t.Fatal(e)
	}
	if f, e := supportOpen(root, name); e == nil {
		f.Close()
		t.Fatal("followed symlink")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if e = os.Symlink(root, alias); e != nil {
		t.Fatal(e)
	}
	if e = supportRootCheck(alias, false); e == nil {
		t.Fatal("accepted root symlink")
	}
}
func TestE07FoundationAndUnknownMetrics(t *testing.T) {
	c := supportCatalogue([]byte(`{"current":{"generation":2,"catalog":{"schemaVersion":1,"devices":[{"name":"PRIVATE"}],"connections":[],"checkPolicies":[],"policyObjects":[{"domain":"PRIVATE"}]}},"history":[]}`))
	if c == nil || c.Devices != 1 || c.Objects != 1 {
		t.Fatal(c)
	}
	b := supportBindings([]byte(`{"schemaVersion":1,"generation":3,"bindings":[{"target":"PRIVATE","pending":{"secret":"PRIVATE"}}]}`))
	if b == nil || b.Count != 1 || b.Pending != 1 {
		t.Fatal(b)
	}
	raw, _ := json.Marshal([]any{c, b})
	if strings.Contains(string(raw), "PRIVATE") {
		t.Fatal("leaked catalogue")
	}
	if supportCatalogue([]byte(`{"current":{"generation":1,"catalog":{"schemaVersion":1}}}`)) != nil {
		t.Fatal("missing collection")
	}
	m, s := supportMemoryInfo([]byte("MemTotal: 1024 kB\nMemAvailable: 512 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"))
	if m == nil || m.Used != 524288 || s == nil || s.Total != 0 {
		t.Fatal(m, s)
	}
	m, s = supportMemoryInfo([]byte("MemTotal: 1 kB\nMemAvailable: 2 kB\n"))
	if m != nil || s != nil {
		t.Fatal("unknown must be nil")
	}
}
func TestE07WorkerBounds(t *testing.T) {
	var out bytes.Buffer
	for _, r := range []string{strings.Repeat("x", 8193), `{"sections":["../etc"],"modules":[]}`, `{"sections":["modules"],"modules":["antiscan-manager"]}`} {
		if supportCollectWorker(strings.NewReader(r), &out) == nil {
			t.Fatal("unsafe worker request")
		}
	}
	w := supportLimitedOutput{limit: 4}
	if n, e := w.Write([]byte("1234")); e != nil || n != 4 {
		t.Fatal(n, e)
	}
	if _, e := w.Write([]byte("5")); e == nil {
		t.Fatal("unbounded output")
	}
}

func TestE07HTTPGuards(t *testing.T) {
	mux := http.NewServeMux()
	registerSupportV2Routes(mux)
	mux.HandleFunc("/v1/maintenance/support-bundle", mutationOnly(handleSupportV2Create))
	cases := []struct {
		path, method, body string
		auth               bool
		status             int
		code               string
	}{
		{"support-preview", "GET", "", false, 405, ""},
		{"support-preview", "POST", `{"confirm":"SUPPORT_PREVIEW","sections":["logs"]}`, false, 403, ""},
		{"support-preview", "POST", `{"confirm":"SUPPORT_PREVIEW","sections":["modules"],"modules":["crowdsec-manager"]}`, true, 400, "INVALID_MODULES"},
		{"support-preview", "POST", `{"confirm":"SUPPORT_PREVIEW","sections":["logs"],"path":"/etc/shadow"}`, true, 400, "INVALID_PREVIEW"},
		{"support-bundle", "POST", `{"confirm":"SUPPORT_BUNDLE"}`, true, 409, "SUPPORT_PREVIEW_REQUIRED"},
		{"support-delete", "POST", `{"confirm":"DELETE_SUPPORT_BUNDLE","name":"../etc/shadow"}`, true, 400, "INVALID_CONFIRMATION"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "/v1/maintenance/"+c.path, strings.NewReader(c.body))
		if c.auth {
			r.Header.Set("X-RouterForge-Admin-Authorized", "core-authorized-v1")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != c.status || !strings.Contains(w.Body.String(), c.code) {
			t.Fatalf("%s %s: %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
}
