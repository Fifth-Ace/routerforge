package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
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

// P51: plain config archives must never contain SMB reconnect plaintext.
func TestP51SMBPrivateArchiveFilter(t *testing.T) {
	for _, rel := range []string{"syslog-smb/credentials", "syslog-smb/connection.json"} {
		if !adminMaintenanceSMBPrivateRelative(rel) || !adminMaintenanceSMBPrivateArchive("opt/etc/routerforge/"+rel) {
			t.Fatalf("private file not excluded: %s", rel)
		}
	}
	for _, rel := range []string{"syslog-smb", "syslog-smb/settings.json", "security.json", "syslog-smb-other/credentials"} {
		if adminMaintenanceSMBPrivateRelative(rel) {
			t.Fatalf("ordinary file unexpectedly excluded: %s", rel)
		}
	}
}

func TestP51LegacyArchiveSecretsNeverImported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	entries := map[string]string{
		"opt/etc/routerforge/security.json":              "{}",
		"opt/etc/routerforge/syslog-smb/credentials":     "username=admin\npassword=plaintext-secret\n",
		"opt/etc/routerforge/syslog-smb/connection.json": `{"remember":true}`,
	}
	for name, data := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	inspection, err := inspectAdminMaintenanceArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.ConfigEntries != 1 || inspection.IgnoredEntries != 2 {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
	stage := t.TempDir()
	if err := extractAdminMaintenanceConfigArchive(path, stage); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stage, "opt/etc/routerforge/syslog-smb/credentials")); !os.IsNotExist(err) {
		t.Fatalf("legacy credentials extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stage, "opt/etc/routerforge/syslog-smb/connection.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy reconnect settings extracted: %v", err)
	}
}

func TestP51RestorePreservesOnlyCurrentRouterSMBPair(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission contract validated by Linux CI")
	}
	source := t.TempDir()
	staged := t.TempDir()
	secretDir := filepath.Join(source, "syslog-smb")
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secretDir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"credentials": "username=current\npassword=current-only\n", "connection.json": `{"remember":true}`} {
		if err := os.WriteFile(filepath.Join(secretDir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := adminMaintenancePreserveLocalSMBPair(source, staged); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"credentials", "connection.json"} {
		got, err := os.ReadFile(filepath.Join(staged, "syslog-smb", name))
		if err != nil {
			t.Fatal(err)
		}
		want, _ := os.ReadFile(filepath.Join(source, "syslog-smb", name))
		if string(got) != string(want) {
			t.Fatalf("local SMB secret changed: %s", name)
		}
	}
	if err := os.Remove(filepath.Join(source, "syslog-smb", "connection.json")); err != nil {
		t.Fatal(err)
	}
	if err := adminMaintenancePreserveLocalSMBPair(source, t.TempDir()); err == nil {
		t.Fatal("partial secret pair accepted")
	}
}
