package supportbundle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestE07AllowlistedProjection(t *testing.T) {
	const secret = "secret-xYz-123"
	raw := []byte("Package: routerforge-core\nVersion: 1~dev.r1430.0fe1c2f9c1e1\nStatus: install ok installed\nDescription: " + secret + "\n\nPackage: " + secret + "\nVersion: " + secret + "\nStatus: install ok installed\n\nPackage: routerforge-admin\nVersion: " + secret + "\nStatus: install ok installed\n")
	p := Packages(raw)
	if len(p) != 2 || p[0].Version != "1~dev.r1430.0fe1c2f9c1e1" || p[1].Version != "unknown" {
		t.Fatalf("packages: %+v", p)
	}
	h := ProjectHealth("nfqws-manager", []byte(`{"ok":true,"version":"1.0.0","target_running":false,"available":true,"target_id":"`+secret+`","config_sha256":"`+secret+`","status":"`+secret+`"}`))
	if h.State != "ok" || h.Running == nil || *h.Running {
		t.Fatal(h)
	}
	l := SummarizeLogs([]byte("ERROR password=" + secret + "\nWARN bearer " + secret + "\nINFO localhost\ntimeout conflict\n-----BEGIN PRIVATE KEY-----\n" + secret))
	if l.Error != 1 || l.Warning != 1 || l.Info != 1 || l.Timeout != 1 || l.Conflict != 1 {
		t.Fatal(l)
	}
	for _, v := range []any{p, h, l} {
		b, _ := json.Marshal(v)
		if bytes.Contains(b, []byte(secret)) || bytes.Contains(b, []byte("localhost")) || bytes.Contains(b, []byte("PRIVATE KEY")) {
			t.Fatalf("raw leak: %s", b)
		}
	}
	if Model("Keenetic Giga (KN-1011)") != "KN-1011" || Model(secret) != "unknown" || Kernel("5.10.123-private-host") != "5.10.123" {
		t.Fatal("model/kernel")
	}
}
func TestE07SelectionAndObservation(t *testing.T) {
	for _, v := range [][]string{nil, {"logs", "logs"}, {"../../etc/passwd"}} {
		if _, e := Selection(v); e == nil {
			t.Fatal(v)
		}
	}
	if _, e := Modules([]string{"crowdsec-manager"}); e == nil {
		t.Fatal("protected runtime")
	}
	if _, e := SelectedModules([]string{"logs"}, []string{"dns"}); e == nil {
		t.Fatal("unselected module")
	}
	if a, e := SelectedModules([]string{"logs"}, nil); e != nil || len(a) != 0 {
		t.Fatal(a, e)
	}
	for _, s := range []string{`{"available":true,"info":513}`, `{"available":true,"password":"secret"}`, `{"available":true} {}`, `{"available":false,"critical":1}`} {
		if ParseObservation(s).Available || ParseObservation(s).Critical != 0 {
			t.Fatal(s)
		}
	}
	if o := ParseObservation(`{"available":true,"info":1,"warning":2,"critical":3,"core_version":"1.0.0","cpu_percent":5.5}`); !o.Available || o.Critical != 3 || o.CPUPercent == nil {
		t.Fatal(o)
	}
	if ProjectHealth("dns", []byte("null")).State != "unavailable" {
		t.Fatal("null health")
	}
}
func TestE07ZipManifestAndTamper(t *testing.T) {
	b, m, e := Build(time.Unix(10, 0), []string{"logs", "packages"}, nil, []Entry{{ID: "logs", State: "ok", Data: JSON(SummarizeLogs([]byte("error secret")))}, {ID: "packages", State: "skipped", Reason: "NOT_AVAILABLE"}})
	if e != nil {
		t.Fatal(e)
	}
	if e = Verify(b, m); e != nil {
		t.Fatal(e)
	}
	if len(m.Modules) != 0 || len(m.Sources) != 2 || m.Sources[0].Name != "" || m.Sources[1].SHA256 == "" {
		t.Fatal(m)
	}
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	names := []string{}
	for _, f := range z.File {
		names = append(names, f.Name)
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		raw, e := io.ReadAll(r)
		r.Close()
		if e != nil || bytes.Contains(raw, []byte("error secret")) {
			t.Fatal("raw log")
		}
	}
	if strings.Join(names, ",") != "logs.json,manifest.json,README.txt" {
		t.Fatal(names)
	}
	m.Sources[1].SHA256 = strings.Repeat("0", 64)
	if Verify(b, m) == nil {
		t.Fatal("manifest mismatch accepted")
	}
	b[len(b)/2] ^= 1
	if Verify(b, m) == nil {
		t.Fatal("tamper accepted")
	}
	if _, _, e := Build(time.Now(), []string{"logs"}, nil, []Entry{{ID: "logs", State: "ok", Data: JSON(strings.Repeat("x", MaxEntry))}}); e == nil {
		t.Fatal("oversized source")
	}
}
