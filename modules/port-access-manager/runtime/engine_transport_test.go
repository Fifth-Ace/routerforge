package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntwareHTTPSConfigLineScope(t *testing.T) {
	cases := []struct{ in, want string }{
		{"src/gz entware http://bin.entware.net/aarch64-k3.10", "src/gz entware https://bin.entware.net/aarch64-k3.10"},
		{"src/gz keendev http://bin.entware.net/aarch64-k3.10/keenetic", "src/gz keendev https://bin.entware.net/aarch64-k3.10/keenetic"},
		{"src/gz third http://repo.hoaxisr.ru/aarch64-k3.10", "src/gz third http://repo.hoaxisr.ru/aarch64-k3.10"},
		{"src/gz antiscan https://example.com/all", "src/gz antiscan https://example.com/all"},
		{"option http://bin.entware.net/foo", "option http://bin.entware.net/foo"},
	}
	for _, tc := range cases {
		if got := entwareHTTPSConfigLine(tc.in); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
func TestCreateEntwareHTTPSConfig(t *testing.T) {
	root := t.TempDir()
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "opkg"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "opkg.conf"), []byte("dest root /opt\noption lists_dir /opt/var/opkg-lists\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "opkg", "feeds.conf"), []byte("src/gz entware http://bin.entware.net/aarch64-k3.10\nsrc/gz third http://repo.example.net/foo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := createEntwareHTTPSConfig(root, tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dest root /opt", "src/gz entware https://bin.entware.net/aarch64-k3.10", "src/gz third http://repo.example.net/foo"}
	for _, s := range want {
		if !strings.Contains(string(content), s) {
			t.Fatalf("missing %q in %s", s, content)
		}
	}
	original, _ := os.ReadFile(filepath.Join(root, "opkg", "feeds.conf"))
	if !strings.Contains(string(original), "http://bin.entware.net/") {
		t.Fatal("original feed changed")
	}
}
