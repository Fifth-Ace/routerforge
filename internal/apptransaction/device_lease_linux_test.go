//go:build linux

package apptransaction

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/Fifth-Ace/routerforge/internal/safety"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDevicePackageLeaseSharedCoordinator(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	c, err := OpenCoordinator(dir)
	if err != nil {
		t.Fatal(err)
	}
	l, err := acquireCoordinatorPackageLease(c)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if f, e := c.AcquireLegacy(); !errors.Is(e, ErrBusy) {
		if f != nil {
			f.Close()
		}
		t.Fatalf("core admission: %v", e)
	}
	if next, e := acquireCoordinatorPackageLease(c); !errors.Is(e, ErrBusy) {
		if next != nil {
			next.Close()
		}
		t.Fatalf("module admission: %v", e)
	}
	cmd := &exec.Cmd{}
	if err = l.Attach(cmd); err != nil || len(cmd.ExtraFiles) != 1 {
		t.Fatal(err)
	}
	if l.Attach(cmd) == nil {
		t.Fatal("duplicate descriptor accepted")
	}
	l.Close()
	f, e := c.AcquireLegacy()
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	// A historical slot does not keep the package manager permanently locked.
	if e = os.Mkdir(c.slot(), 0700); e != nil {
		t.Fatal(e)
	}
	if next, e := acquireCoordinatorPackageLease(c); e != nil {
		t.Fatal(e)
	} else {
		next.Close()
	}
}

func TestDevicePackageLeaseInheritedChild(t *testing.T) {
	if os.Getenv("RF_TEST_DEVICE_LEASE_CHILD") == "1" {
		f := os.NewFile(3, "test-inherited-lease")
		if f == nil {
			t.Fatal("missing fd")
		}
		defer f.Close()
		if _, err := f.Stat(); err != nil {
			t.Fatal(err)
		}
		fmt.Println("LEASE_CHILD_READY")
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			t.Fatal(err)
		}
		return
	}
	d := t.TempDir()
	if err := os.Chmod(d, 0700); err != nil {
		t.Fatal(err)
	}
	c, err := OpenCoordinator(d)
	if err != nil {
		t.Fatal(err)
	}
	l, err := acquireCoordinatorPackageLease(c)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, os.Args[0], "-test.run=^TestDevicePackageLeaseInheritedChild$")
	if err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(os.Environ(), "RF_TEST_DEVICE_LEASE_CHILD=1")
	cmd.Stderr = os.Stderr
	if err = l.Attach(cmd); err != nil {
		t.Fatal(err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = in.Close()
			cancel()
			_ = cmd.Wait()
		}
	}()
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "LEASE_CHILD_READY" {
		t.Fatal(line, err)
	}
	l.Close()
	if f, e := c.AcquireLegacy(); !errors.Is(e, ErrBusy) {
		if f != nil {
			f.Close()
		}
		t.Fatalf("child lease lost: %v", e)
	}
	if err = in.Close(); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatal(err)
	}
	f, err := c.AcquireLegacy()
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}
