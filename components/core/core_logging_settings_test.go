package main

import "testing"

func TestP56CoreLoggingSettings(t *testing.T) {
	s := defaultCoreLoggingSettings()
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	if path, err := resolveCoreLogPath(s, "/opt/var/log/routerforge.log", false); err != nil || path != "/opt/var/log/routerforge.log" {
		t.Fatalf("entware: %q %v", path, err)
	}
	s.Mode = "ram"
	if _, err := resolveCoreLogPath(s, "original", false); err == nil {
		t.Fatal("non-tmpfs RAM mode accepted")
	}
	if path, err := resolveCoreLogPath(s, "original", true); err != nil || path != coreRAMLogPath {
		t.Fatalf("ram: %q %v", path, err)
	}
	s.MaxMB = 17
	if s.validate() == nil {
		t.Fatal("unbounded log size accepted")
	}
	s.MaxMB = 1
	s.Backups = 0
	if s.validate() == nil {
		t.Fatal("missing rotation accepted")
	}
	s.Backups = 2
	s.Mode = "custom"
	if s.validate() == nil {
		t.Fatal("unimplemented mode accepted")
	}
}
