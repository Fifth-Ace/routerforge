package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeMaintenanceArchiveNameRejectsTraversal(t *testing.T) {
	bad := []string{"../etc/passwd", "/opt/etc/routerforge/x", "opt/etc/routerforge/../../tmp/x", ""}
	for _, name := range bad {
		if _, err := normalizeMaintenanceArchiveName(name); err == nil {
			t.Fatalf("expected rejection for %q", name)
		}
	}
}

func TestMaintenanceConfigArchivePathScope(t *testing.T) {
	good := []string{"opt/etc/routerforge", "opt/etc/routerforge/config.json"}
	for _, name := range good {
		if !isMaintenanceConfigArchivePath(name) {
			t.Fatalf("expected config path: %q", name)
		}
	}
	bad := []string{"opt/etc/routerforge-old/x", "opt/share/routerforge/ui", "etc/routerforge/x"}
	for _, name := range bad {
		if isMaintenanceConfigArchivePath(name) {
			t.Fatalf("unexpected config path: %q", name)
		}
	}
}

func TestInspectMaintenanceArchiveIgnoresLegacyShareButAcceptsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	entries := []struct {
		name string
		body string
	}{
		{"opt/etc/routerforge/config.json", "{}"},
		{"opt/share/routerforge/ui/index.html", "legacy"},
	}
	for _, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	inspection, err := inspectAdminMaintenanceArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Valid || inspection.ConfigEntries != 1 || inspection.IgnoredEntries != 1 {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
}

func TestInspectMaintenanceArchiveRejectsConfigSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "opt/etc/routerforge/link",
		Mode:     0777,
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := inspectAdminMaintenanceArchive(path); err == nil {
		t.Fatal("expected symlink archive rejection")
	}
}
