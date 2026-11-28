//go:build linux

package apptransaction

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagementPreviewAdvisory(t *testing.T) {
	_, p, pkg := lifecycleFixture(t)
	old := "1~dev.r1458.000000000000"
	for _, mode := range []string{"ready", "unavailable", "drift", "current", "status_failed", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			reads := 0
			probes := 0
			result, err := previewManagement(ctx, p, func(context.Context, string) (string, error) {
				reads++
				if mode == "status_failed" {
					return "", errors.New("status failed")
				}
				if mode == "current" || (mode == "drift" && reads > 1) {
					return pkg.Version, nil
				}
				return old, nil
			}, func(_ context.Context, name, installed, core string) error {
				probes++
				if name != pkg.Name || installed != old {
					t.Fatal(name, installed)
				}
				if mode == "unavailable" {
					return errors.New("not ready")
				}
				return nil
			})
			want := map[string]string{"ready": "management_ready", "unavailable": "management_unavailable", "drift": "version_conflict", "current": "version_conflict", "status_failed": "installed_state_unavailable", "cancelled": "not_checked"}[mode]
			if err != nil || len(result) != 1 || result[0].State != want {
				t.Fatal(result, err)
			}
			if (mode == "current" || mode == "status_failed" || mode == "cancelled") && probes != 0 {
				t.Fatal("unexpected probe")
			}
		})
	}
}

func TestCompletedHistoryReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	c, err := OpenCoordinator(root)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := c.CompletedHistory(10); err != nil || len(v) != 0 {
		t.Fatal(v, err)
	}
	_, p, _ := lifecycleFixture(t)
	dir := filepath.Join(root, "completed-"+p.ID)
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	// Use the exact durable request encoding and complete checkpoint.
	if err = writeHistoryFixture(s, p, h); err != nil {
		t.Fatal(err)
	}
	v, err := c.CompletedHistory(10)
	if err != nil || len(v) != 1 || v[0].ID != p.ID || v[0].PackageCount != len(p.Packages) {
		t.Fatal(v, err)
	}
	if err = os.Symlink(dir, filepath.Join(root, "completed-other")); err != nil {
		t.Fatal(err)
	}
	if _, err = c.CompletedHistory(10); err == nil {
		t.Fatal("symlink history accepted")
	}
}

func writeHistoryFixture(s *Store, p Plan, h string) error {
	b, e := json.Marshal(workerRequest{p, h})
	if e != nil {
		return e
	}
	if e = durableExclusive(s, "request.json", b); e != nil {
		return e
	}
	return s.save(Journal{Schema: Schema, Plan: p, PlanSHA256: h, Phase: Completed, Next: len(p.Packages)})
}

func TestManagementPreviewProfilingCoreDrift(t *testing.T) {
	_, p, _ := lifecycleFixture(t)
	pkg := p.Packages[0]
	pkg.Name = "routerforge-profiling"
	pkg.Asset = pkg.Name + "_" + strings.ReplaceAll(pkg.Version, "~", "-") + "_" + p.Target + ".ipk"
	p.Packages[0] = pkg
	old := "1~dev.r1458.000000000000"
	core := "1~dev.r1457.111111111111"
	reads := 0
	result, err := previewManagement(context.Background(), p, func(_ context.Context, name string) (string, error) {
		if name == "routerforge-core" {
			reads++
			if reads > 1 {
				return old, nil
			}
			return core, nil
		}
		return old, nil
	}, func(_ context.Context, name, installed, coreVersion string) error {
		if name != pkg.Name || installed != old || coreVersion != core {
			t.Fatal(name, installed, coreVersion)
		}
		return nil
	})
	if err != nil || len(result) != 1 || result[0].State != "version_conflict" {
		t.Fatal(result, err)
	}
}

func TestManagementPreviewStatusCommandOnly(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "opkg-fixture")
	raw := `#!/bin/sh
[ "$#" = 2 ] && [ "$1" = status ] && [ "$2" = routerforge-dns ] || exit 9
printf 'Package: routerforge-dns\nVersion: 1~dev.r1458.000000000000\nArchitecture: aarch64-3.10\nStatus: install user installed\n\n'
`
	if err := os.WriteFile(bin, []byte(raw), 0700); err != nil {
		t.Fatal(err)
	}
	v, err := readInstalledDevVersion(context.Background(), bin, "routerforge-dns")
	if err != nil || v != "1~dev.r1458.000000000000" {
		t.Fatal(v, err)
	}
}
