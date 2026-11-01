//go:build linux

package lite

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestP19ProcessLockRejectsDuplicateAndSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.lock")
	release, e := AcquireProcessLock(path)
	if e != nil {
		t.Fatal(e)
	}
	if second, e := AcquireProcessLock(path); e == nil {
		second()
		t.Fatal("two supervisors acquired the same lock")
	}
	release()
	again, e := AcquireProcessLock(path)
	if e != nil {
		t.Fatal("stale lock cannot be reused", e)
	}
	again()
	link := filepath.Join(t.TempDir(), "link")
	if e = os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if done, e := AcquireProcessLock(link); e == nil {
		done()
		t.Fatal("symlink lock accepted")
	}
}
func TestP19SocketReceiptLifecycle(t *testing.T) {
	if os.Getenv("RF_P19_SOCKET_HELPER") == "1" {
		path := os.Getenv("RF_P19_SOCKET_PATH")
		listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
		if e != nil {
			os.Exit(2)
		}
		listener.SetUnlinkOnClose(false)
		if e = RecordSocketOwner(path); e != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	path := filepath.Join(t.TempDir(), "management.sock")
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatalf("CI prerequisite missing: AF_UNIX: %v", e)
		}
		t.Skipf("BLOCKED: host forbids AF_UNIX; live socket recovery must run in source CI: %v", e)
	}
	if e = RecordSocketOwner(path); e != nil {
		t.Fatal(e)
	}
	if e = RecoverOwnedSocket(path); e == nil {
		t.Fatal("live management socket removed")
	}
	listener.Close()
	os.Remove(path + ".owner.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestP19SocketReceiptLifecycle$")
	cmd.Env = append(os.Environ(), "RF_P19_SOCKET_HELPER=1", "RF_P19_SOCKET_PATH="+path)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("socket helper: %v %s", e, out)
	}
	if e = RecoverOwnedSocket(path); e != nil {
		t.Fatal("proven dead owner socket not recovered", e)
	}
	if _, e = os.Lstat(path); !os.IsNotExist(e) {
		t.Fatal("stale socket remains")
	}
	if e = os.WriteFile(path, []byte("foreign"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = RecoverOwnedSocket(path); e == nil {
		t.Fatal("foreign file removed")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "foreign" {
		t.Fatal("foreign file changed")
	}
}
