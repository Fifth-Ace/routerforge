package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRFParseProcNetTCPIPv4ListenersOnly(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:08B9 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 12345 1 0000000000000000
   1: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 23456 1 0000000000000000
   2: 0100007F:0050 0100007F:ABCD 01 00000000:00000000 00:00000000 00000000 0 0 34567 1 0000000000000000
`
	listeners, err := rfParseProcNetTCP(strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(listeners) != 2 {
		t.Fatalf("listeners=%d %#v", len(listeners), listeners)
	}
	if listeners[0].Address != "127.0.0.1" || listeners[0].Port != 2233 || listeners[0].Inode != 12345 {
		t.Fatalf("first=%#v", listeners[0])
	}
	if listeners[1].Address != "0.0.0.0" || listeners[1].Port != 8080 || listeners[1].Inode != 23456 {
		t.Fatalf("second=%#v", listeners[1])
	}
}

func TestRFDecodeProcTCPAddressIPv6(t *testing.T) {
	host, port, err := rfDecodeProcTCPAddress("0000000000000000FFFF00000100007F:08B9", true)
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" || port != 2233 {
		t.Fatalf("host=%q port=%d", host, port)
	}
}

func TestRFAttachLocalTCPListenerOwnersPrefersAppScriptPackage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix-like filesystem semantics")
	}
	root := t.TempDir()
	procRoot := filepath.Join(root, "proc")
	infoDir := filepath.Join(root, "info")
	if err := os.MkdirAll(filepath.Join(procRoot, "321", "fd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(infoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:[777]", filepath.Join(procRoot, "321", "fd", "5")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/opt/bin/python3", filepath.Join(procRoot, "321", "exe")); err != nil {
		t.Fatal(err)
	}
	cmd := []byte("/opt/bin/python3\x00/opt/share/my-web-ui/server.py\x00--port\x002222\x00")
	if err := os.WriteFile(filepath.Join(procRoot, "321", "cmdline"), cmd, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(infoDir, "python3-base.list"), []byte("/opt/bin/python3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(infoDir, "my-web-ui.list"), []byte("/opt/share/my-web-ui/server.py\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := rfAttachLocalTCPListenerOwners(procRoot, infoDir, []rfWebListener{{Address: "0.0.0.0", Port: 2222, Inode: 777}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Owners) != 1 {
		t.Fatalf("got=%#v", got)
	}
	owner := got[0].Owners[0]
	if owner.PID != 321 || owner.Process != "python3" || owner.Package != "my-web-ui" {
		t.Fatalf("owner=%#v", owner)
	}
}

func TestRFSocketInode(t *testing.T) {
	inode, ok := rfSocketInode("socket:[123456]")
	if !ok || inode != 123456 {
		t.Fatalf("inode=%d ok=%v", inode, ok)
	}
	if _, ok := rfSocketInode("pipe:[123456]"); ok {
		t.Fatal("pipe accepted as socket")
	}
}
