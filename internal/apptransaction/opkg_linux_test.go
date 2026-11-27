//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func opkgStatus(p Package, v, status, arch string) string {
	return fmt.Sprintf("Package: %s\nVersion: %s\nStatus: %s\nArchitecture: %s\n\n", p.Name, v, status, arch)
}
func TestInstalledDevVersion(t *testing.T) {
	p := fixture().Packages[0]
	good := opkgStatus(p, p.Version, "install user installed", "aarch64-3.10")
	if v, e := installedDevVersion(good, p.Name); e != nil || v != p.Version {
		t.Fatal(v, e)
	}
	for _, raw := range []string{"", good + good, strings.Replace(good, "Architecture: aarch64-3.10", "Architecture: mips-3.4", 1), strings.Replace(good, "install user installed", "install user unpacked", 1), strings.Replace(good, "install user installed", "install hold installed", 1), strings.Replace(good, "Version: ", "Version: bad\nVersion: ", 1)} {
		if _, e := installedDevVersion(raw, p.Name); e == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestAllowUpgrade(t *testing.T) {
	v := "1~dev.r1458.af90aae4fb68"
	for _, old := range []string{"1~dev.r1456.778961ab5c93", v} {
		if e := allowUpgrade(old, v); e != nil {
			t.Fatal(e)
		}
	}
	for _, old := range []string{"1.0.0", "1~dev.r1459.778961ab5c93", "1~dev.r1458.778961ab5c93", "1~dev.r+1457.778961ab5c93", "1~dev.r1457.XXXX61ab5c93"} {
		if e := allowUpgrade(old, v); e == nil {
			t.Fatal(old)
		}
	}
}
func TestVerifiedOPKG(t *testing.T) {
	for _, mode := range []string{"success", "already", "bad-sha", "install-failure", "wrong-version", "lifecycle-failure", "downgrade", "lease-closed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			data := []byte("exact staged IPK fixture")
			st, p := stagedFixture(t, data)
			n := 0
			setBody(t, st, p, data, 200, &n)
			if err := st.Prepare(ctx, p); err != nil {
				t.Fatal(err)
			}
			lock := store(t)
			lease, e := AcquireMutationLease(lock.dir)
			if e != nil {
				t.Fatal(e)
			}
			defer lease.Close()
			dir := t.TempDir()
			status := filepath.Join(dir, "status")
			calls := filepath.Join(dir, "calls")
			next := filepath.Join(dir, "next")
			bin := filepath.Join(dir, "opkg")
			current := "1~dev.r0.000000000000"
			if mode == "already" {
				current = p.Version
			}
			if mode == "downgrade" {
				current = "1~dev.r999999999.000000000000"
			}
			put := func(path, s string, perm os.FileMode) {
				t.Helper()
				if e := os.WriteFile(path, []byte(s), perm); e != nil {
					t.Fatal(e)
				}
			}
			put(status, opkgStatus(p, current, "install user installed", "aarch64-3.10"), 0600)
			after := p.Version
			if mode == "wrong-version" {
				after = current
			}
			put(next, opkgStatus(p, after, "install user installed", "aarch64-3.10"), 0600)
			code := 0
			if mode == "install-failure" {
				code = 9
			}
			put(bin, fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\nstatus) cat '%s';;\ninstall) [ \"$#\" = 2 ] && [ -f \"$2\" ] && [ -e /proc/self/fd/3 ] || exit 8; printf '%%s\\n' \"$2\" >> '%s'; cp '%s' '%s'; exit %d;;\n*) exit 7;;\nesac\n", status, calls, next, status, code), 0700)
			checked := false
			a, e := newVerifiedOPKG(st, lock.dir, lease, bin, func(context.Context, Package) error {
				checked = true
				if mode == "lifecycle-failure" {
					return errors.New("not ready")
				}
				return nil
			})
			if e != nil {
				t.Fatal(e)
			}
			if mode == "bad-sha" {
				path, _ := st.path(p)
				put(path, strings.Repeat("x", len(data)), 0600)
			}
			if mode == "lease-closed" {
				lease.Close()
			}
			e = a.Install(ctx, p)
			wantInstall := mode == "success" || mode == "install-failure" || mode == "wrong-version" || mode == "lifecycle-failure"
			_, statErr := os.Stat(calls)
			if (statErr == nil) != wantInstall {
				t.Fatal("mutation mismatch", mode, statErr)
			}
			if mode == "install-failure" || mode == "downgrade" || mode == "lease-closed" || mode == "bad-sha" {
				if e == nil {
					t.Fatal("expected error")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			e = a.Verify(ctx, p)
			wantVerify := mode == "success" || mode == "already"
			if (e == nil) != wantVerify {
				t.Fatal(e)
			}
			if checked != (mode != "wrong-version") {
				t.Fatal("lifecycle mismatch")
			}
		})
	}
}
