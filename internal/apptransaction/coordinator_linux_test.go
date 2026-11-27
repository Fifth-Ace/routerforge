//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func newCoordinatorFixture(t *testing.T) *Coordinator {
	t.Helper()
	s := store(t)
	c, e := OpenCoordinator(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func awaitCoordinatorState(t *testing.T, c *Coordinator, state string) {
	t.Helper()
	until := time.Now().Add(15 * time.Second)
	for time.Now().Before(until) {
		v, e := c.Status()
		if e == nil && v.State == state {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	v, e := c.Status()
	t.Fatalf("state %s: %+v / %v", state, v, e)
}
func TestCoordinatorProcess(t *testing.T) {
	if len(os.Args) < 5 || os.Args[len(os.Args)-3] != "RF_COORD_TEST" {
		return
	}
	root, mode := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	c, e := OpenCoordinator(root)
	if e != nil {
		t.Fatal(e)
	}
	switch mode {
	case "launcher":
		exe, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		_, e = c.Start(exe, []string{"-test.run=^TestCoordinatorProcess$", "--", "RF_COORD_TEST", root, "worker"}, fixture())
		if e != nil {
			t.Fatal(e)
		}
	case "worker", "failworker":
		j, e := c.RunWorker(context.Background(), func(dir string, p Plan, lease *os.File) (Hooks, error) {
			f := &fake{}
			f.callback = func(call string, _ context.Context) {
				if call == "install:"+p.Packages[0].Name {
					if mode == "failworker" {
						os.Exit(44)
					}
					if e := os.WriteFile(filepath.Join(root, "entered"), []byte("yes"), 0600); e != nil {
						t.Fatal(e)
					}
					until := time.Now().Add(12 * time.Second)
					for time.Now().Before(until) {
						if _, e := os.Stat(filepath.Join(root, "continue")); e == nil {
							return
						}
						time.Sleep(20 * time.Millisecond)
					}
					t.Fatal("worker gate timed out")
				}
			}
			return f, nil
		})
		if e != nil || j.Phase != Completed {
			t.Fatal(j, e)
		}
	default:
		t.Fatal(mode)
	}
}
func TestCoordinatorDetachedAdmission(t *testing.T) {
	c := newCoordinatorFixture(t)
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	if b, e := exec.Command(exe, "-test.run=^TestCoordinatorProcess$", "--", "RF_COORD_TEST", c.root, "launcher").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	awaitCoordinatorState(t, c, string(Applying))
	if _, e = c.AcquireLegacy(); !errors.Is(e, ErrBusy) {
		t.Fatal("legacy not excluded", e)
	}
	if _, e = c.Start(exe, nil, fixture()); !errors.Is(e, ErrBusy) {
		t.Fatal("duplicate worker accepted", e)
	}
	if e = os.WriteFile(filepath.Join(c.root, "continue"), []byte("yes"), 0600); e != nil {
		t.Fatal(e)
	}
	awaitCoordinatorState(t, c, string(Completed))
	until := time.Now().Add(10 * time.Second)
	for {
		l, e := c.AcquireLegacy()
		if e == nil {
			l.Close()
			break
		}
		if !errors.Is(e, ErrBusy) || time.Now().After(until) {
			t.Fatal(e)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, e = c.Start(exe, nil, fixture()); !errors.Is(e, ErrRecovery) {
		t.Fatal("completed ID replay", e)
	}
	p := fixture()
	p.ID = "second"
	if _, e = c.Start(exe, []string{"-test.run=^TestCoordinatorProcess$", "--", "RF_COORD_TEST", c.root, "worker"}, p); e != nil {
		t.Fatal(e)
	}
	awaitCoordinatorState(t, c, string(Completed))
	if _, e = Open(filepath.Join(c.root, "completed-test")); e != nil {
		t.Fatal(e)
	}
}
func TestCoordinatorInterruptedBlocksLegacyAndReplay(t *testing.T) {
	c := newCoordinatorFixture(t)
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.Start(exe, []string{"-test.run=^TestCoordinatorProcess$", "--", "RF_COORD_TEST", c.root, "failworker"}, fixture())
	if e != nil {
		t.Fatal(e)
	}
	awaitCoordinatorState(t, c, string(RecoveryRequired))
	v, e := c.Status()
	if e != nil || v.Journal == nil || v.Journal.Phase != Applying {
		t.Fatal(v, e)
	}
	if _, e = c.AcquireLegacy(); !errors.Is(e, ErrRecovery) {
		t.Fatal(e)
	}
	p := fixture()
	p.ID = "other"
	if _, e = c.Start(exe, nil, p); !errors.Is(e, ErrRecovery) {
		t.Fatal(e)
	}
}
func TestCoordinatorSpawnFailure(t *testing.T) {
	c := newCoordinatorFixture(t)
	if _, e := c.Start("/does-not-exist/rf-worker", nil, fixture()); e == nil {
		t.Fatal("spawn accepted")
	}
	awaitCoordinatorState(t, c, string(RecoveryRequired))
	if _, e := c.AcquireLegacy(); !errors.Is(e, ErrRecovery) {
		t.Fatal(e)
	}
}
func TestCoordinatorUnsafeSlot(t *testing.T) {
	c := newCoordinatorFixture(t)
	if e := os.Symlink(t.TempDir(), c.slot()); e != nil {
		t.Fatal(e)
	}
	if _, e := c.AcquireLegacy(); !errors.Is(e, ErrRecovery) {
		t.Fatal(e)
	}
	if v, e := c.Status(); e == nil || v.State != string(RecoveryRequired) {
		t.Fatal(v, e)
	}
}
func TestCoordinatorCanonicalEntwareParent(t *testing.T) {
	d := t.TempDir()
	real := filepath.Join(d, "mounted")
	if e := os.Mkdir(real, 0700); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(d, "opt")
	if e := os.Symlink(real, alias); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(real, "routerforge"), 0755); e != nil {
		t.Fatal(e)
	}
	c, e := createCoordinatorUnder(filepath.Join(alias, "routerforge"))
	if e != nil {
		t.Fatal(e)
	}
	if c.root != filepath.Join(real, "routerforge", "routerforge-app-transactions") {
		t.Fatal(c.root)
	}
	if _, e = createCoordinatorUnder(alias); e == nil {
		t.Fatal("accepted symlink config parent")
	}
}
