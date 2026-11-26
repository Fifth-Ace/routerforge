package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestE062MountSelection(t *testing.T) {
	raw := "42 1 8:1 / /tmp/mnt/USB\\040Disk rw - ext4 /dev/sda1 rw\n43 1 0:1 / /opt/mnt/nas rw - cifs //private/share rw\n44 1 0:1 / /opt rw - ext4 /dev/sda1 rw\n45 1 0:2 / /opt/mnt/hidden rw - tmpfs tmpfs rw\n"
	m := recoveryMounts(raw)
	if len(m) != 2 || m[0].Kind != "smb" || m[1].Point != "/tmp/mnt/USB Disk" {
		t.Fatalf("wrong mounts: %#v", m)
	}
	if m[0].ID == m[1].ID || len(m[0].ID) != 64 {
		t.Fatal("identity")
	}
	if len(recoveryMounts(raw+"46 1 8:2 / /tmp/mnt/USB\\040Disk rw - ext4 /dev/sdb1 rw\n")) != 1 {
		t.Fatal("stacked mount")
	}
	if recoveryMounts(strings.Replace(raw, "42 1", "52 1", 1))[1].ID == m[1].ID {
		t.Fatal("mount identity did not change")
	}
}
func TestE062MountFDGate(t *testing.T) {
	f, e := os.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if recoveryFDOnMount(f, recoveryMount{mountID: "0"}) {
		t.Fatal("wrong mount accepted")
	}
}
func TestE062ExternalFileNames(t *testing.T) {
	for _, x := range []string{"../rfvault-" + strings.Repeat("a", 32) + ".rfv", "file.rfv", "rfvault-" + strings.Repeat("a", 32) + ".rfv.partial"} {
		if recoveryFileRE.MatchString(x) {
			t.Fatal("unsafe name")
		}
	}
	if !recoveryFileRE.MatchString("rfvault-" + strings.Repeat("a", 32) + ".rfv") {
		t.Fatal("valid name rejected")
	}
}

func TestE062ExternalExclusiveWrite(t *testing.T) {
	root := t.TempDir()
	dir, e := os.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer dir.Close()
	raw, e := os.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", dir.Fd()))
	if e != nil {
		t.Fatal(e)
	}
	var m recoveryMount
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(l, "mnt_id:") {
			m.mountID = strings.TrimSpace(strings.TrimPrefix(l, "mnt_id:"))
		}
	}
	if m.mountID == "" {
		t.Skip("kernel fdinfo mount ID unavailable")
	}
	name := "rfvault-" + strings.Repeat("f", 32) + ".rfv"
	data := []byte("encrypted fixture only")
	if e = recoveryWriteExternal(dir, m, name, data); e != nil {
		t.Fatal(e)
	}
	if e = recoveryWriteExternal(dir, m, name, []byte("overwrite")); e == nil {
		t.Fatal("overwrite allowed")
	}
	observed, e := recoveryReadExternal(dir, m, name)
	if e != nil || !bytes.Equal(observed, data) {
		t.Fatal("readback mismatch")
	}
}
