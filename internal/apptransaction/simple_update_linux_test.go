//go:build linux

package apptransaction

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimpleCoordinatorEntwareFallback(t *testing.T) {
	root := t.TempDir()
	v := filepath.Join(root, "var")
	cfg := filepath.Join(root, "config")
	for _, p := range []string{v, cfg} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	// An untrusted parent must not force changes to Entware permissions.
	if err := os.Chmod(v, 0777); err != nil {
		t.Fatal(err)
	}
	c, err := deviceCoordinatorAt(v, cfg)
	if err != nil || c.root != filepath.Join(cfg, "routerforge-app-transactions") {
		t.Fatal(c, err)
	}
	if st, _ := os.Stat(v); st.Mode().Perm() != 0777 {
		t.Fatal("var changed")
	}
	if err := os.Chmod(v, 0700); err != nil {
		t.Fatal(err)
	}
	next, err := deviceCoordinatorAt(v, cfg)
	if err != nil || next.root != c.root {
		t.Fatal("lock path drift", err)
	}
}
func TestSimpleUpdateDoesNotBlockOnOldFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	c, err := OpenCoordinator(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(c.slot(), 0700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(c.slot(), "old-log")
	if err = os.WriteFile(old, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	l, err := c.AcquirePackageMutation()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 40)
	v := "1~dev.r2." + sha[:12]
	p := Plan{ID: "new-plan", Channel: "dev", Target: "aarch64-3.10", Commit: sha, Packages: []Package{{Name: "routerforge-admin", Version: v, SHA256: strings.Repeat("b", 64), Asset: "routerforge-admin_" + strings.ReplaceAll(v, "~", "-") + "_aarch64-3.10.ipk", Size: 1}}}
	check := func() error { return nil }
	launch := func(string, *os.File) (int, error) { return 42, nil }
	if _, err = c.startSimpleUpdate(p, check, launch); !errors.Is(err, ErrBusy) {
		t.Fatal("concurrent update", err)
	}
	l.Close()
	if id, err := c.startSimpleUpdate(p, check, launch); err != nil || id != 42 {
		t.Fatal(id, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "interrupted-new-plan", "old-log")); err != nil || string(data) != "preserve" {
		t.Fatal("old evidence lost", err)
	}
	if _, err = c.startSimpleUpdate(p, check, launch); !errors.Is(err, ErrRecovery) {
		t.Fatal("replay", err)
	}
}

func TestSimpleCoordinatorForeignOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to model Keenetic UID 1001")
	}
	root := t.TempDir()
	v := filepath.Join(root, "var")
	cfg := filepath.Join(root, "config")
	for _, p := range []string{v, cfg} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chown(v, 1001, 1001); err != nil {
		t.Skip(err)
	}
	c, err := deviceCoordinatorAt(v, cfg)
	if err != nil || c.root != filepath.Join(cfg, "routerforge-app-transactions") {
		t.Fatal(c, err)
	}
}
