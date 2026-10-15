package logpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	s := Settings{Mode: "entware", MaxMB: 1, Backups: 2}
	got, e := resolve(s, "dns", "/opt/var/log/routerforge-dns.log", false)
	if e != nil || got != "/opt/var/log/routerforge-dns.log" {
		t.Fatalf("entware %q %v", got, e)
	}
	s.Mode = "ram"
	if _, e = resolve(s, "dns", "/opt/var/log/routerforge-dns.log", false); e == nil {
		t.Fatal("missing ram fs accepted")
	}
	got, e = resolve(s, "dns", "/opt/var/log/routerforge-dns.log", true)
	if e != nil || got != "/tmp/routerforge/logs/routerforge-dns.log" {
		t.Fatalf("ram %q %v", got, e)
	}
	s.Mode = "custom"
	s.CustomPath = "/opt/mnt/usb/routerforge.log"
	got, e = resolve(s, "antiscan-audit", "/opt/var/log/routerforge/antiscan-manager.jsonl", false)
	if e != nil || got != "/opt/mnt/usb/routerforge-antiscan-audit.jsonl" {
		t.Fatalf("audit %q %v", got, e)
	}
	for _, bad := range []string{"/etc/passwd.log", "/opt/mnt/usb/../cfg.log", "/opt/etc/routerforge/conf.log"} {
		s.CustomPath = bad
		if _, e = resolve(s, "dns", "/opt/var/log/a.log", false); e == nil {
			t.Fatalf("unsafe custom accepted %s", bad)
		}
	}
	s.Mode = "entware"
	s.CustomPath = ""
	if _, e = resolve(s, "../../escape", "/opt/var/log/a.log", false); e == nil {
		t.Fatal("unsafe module accepted")
	}
}
func TestAppendRetentionAndNoFollow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "module.log")
	payload := []byte(strings.Repeat("a", 4096))
	// Test the underlying guarded file operations with an isolated directory.
	for i := 0; i < 2; i++ {
		f, e := openAppendNoFollow(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(payload); e != nil {
			t.Fatal(e)
		}
		if e = f.Close(); e != nil {
			t.Fatal(e)
		}
	}
	if e := renameIfRegular(path, path+".1"); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(dir, "elsewhere"), path); e != nil {
		t.Skipf("no symlinks: %v", e)
	}
	if _, e := openAppendNoFollow(path); e == nil {
		t.Fatal("symlink followed")
	}
	if e := removeIfRegular(path); e == nil {
		t.Fatal("symlink removed by cleanup")
	}
	if e := removeIfRegular(path + ".1"); e != nil {
		t.Fatal(e)
	}
}
func TestBrokenConfigDoesNotFallBack(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if e := os.WriteFile(p, []byte(`{"mode":"ram","max_mb":0,"backups":2}`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readSettings(p); e == nil {
		t.Fatal("invalid RAM config accepted")
	}
	if _, e := readSettings(p + "-missing"); e != nil {
		t.Fatal(e)
	}
}

func TestRealAppendRotateAndGuard(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "dns.log")
	s := Settings{Mode: "entware", MaxMB: 1, Backups: 2}
	first := []byte(strings.Repeat("x", 1024*1024-4))
	if err := os.WriteFile(file, first, 0600); err != nil {
		t.Fatal(err)
	}
	if err := appendWithSettings(s, "dns", file, []byte("new line\n"), false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new line\n" {
		t.Fatalf("post-rotation active=%q", b)
	}
	fi, err := os.Stat(file + ".1")
	if err != nil || fi.Size() != int64(len(first)) {
		t.Fatalf("backup: %v %v", fi, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "secret"), file); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := appendWithSettings(s, "dns", file, []byte("should-not-write"), false); err == nil {
		t.Fatal("symlink accepted")
	}
}
