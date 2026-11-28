package apptransaction

import (
	"os/exec"
	"testing"
)

func TestDevicePackageLeaseScope(t *testing.T) {
	for _, c := range []struct {
		os, arch, version string
		want              bool
	}{
		{"linux", "arm64", "1~dev.r1459.aaaaaaaaaaaa", true},
		{"windows", "arm64", "1~dev.r1459.aaaaaaaaaaaa", false},
		{"linux", "mips", "1~dev.r1459.aaaaaaaaaaaa", false},
		{"linux", "arm64", "1.0.0", false},
		{"linux", "arm64", "1~beta.r1.aaaaaaaaaaaa", false},
		{"linux", "arm64", "dev", false},
	} {
		if got := devicePackageLeaseRequired(c.os, c.arch, c.version); got != c.want {
			t.Fatalf("%+v got %v", c, got)
		}
	}
	l, err := AcquireDevicePackageLease("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	cmd := &exec.Cmd{}
	if err = l.Attach(cmd); err != nil || len(cmd.ExtraFiles) != 0 {
		t.Fatal(err)
	}
	l.Close()
	l.Close()
	if l.Attach(cmd) == nil {
		t.Fatal("closed lease accepted")
	}
}
