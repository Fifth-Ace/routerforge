//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture() Plan {
	c := strings.Repeat("a", 40)
	v := "1~dev.r1458." + c[:12]
	p := Plan{ID: "test", Channel: "dev", Target: "aarch64-3.10", Commit: c}
	for _, n := range []string{"routerforge-dns-router", "routerforge-profiling", "routerforge-core"} {
		p.Packages = append(p.Packages, Package{n, v, strings.Repeat("b", 64), n + "_" + strings.ReplaceAll(v, "~", "-") + "_" + p.Target + ".ipk", 123})
	}
	return p
}
func store(t *testing.T) *Store {
	t.Helper()
	d := t.TempDir()
	if e := os.Chmod(d, 0700); e != nil {
		t.Fatal(e)
	}
	s, e := Open(d)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

type fake struct {
	calls    []string
	fail     string
	callback func(string, context.Context)
}

func (f *fake) call(k string, c context.Context, p Package) error {
	v := k + ":" + p.Name
	f.calls = append(f.calls, v)
	if f.callback != nil {
		f.callback(v, c)
	}
	if f.fail == v {
		return errors.New("injected")
	}
	return nil
}
func (f *fake) Prepare(c context.Context, p Package) error { return f.call("prepare", c, p) }
func (f *fake) Check(c context.Context, p Package) error   { return f.call("check", c, p) }
func (f *fake) Install(c context.Context, p Package) error { return f.call("install", c, p) }
func (f *fake) Verify(c context.Context, p Package) error  { return f.call("verify", c, p) }
func TestPlanValidation(t *testing.T) {
	for _, tc := range []struct {
		n    string
		edit func(*Plan)
	}{{"unknown", func(p *Plan) { p.Packages[0].Name = "evil" }}, {"mixed", func(p *Plan) { p.Packages[1].Version = "1~dev.r1.aaaaaaaaaaaa" }}, {"asset traversal", func(p *Plan) { p.Packages[0].Asset = "../file" }}, {"hash", func(p *Plan) { p.Packages[0].SHA256 = "wrong" }}, {"core first", func(p *Plan) { p.Packages[0], p.Packages[2] = p.Packages[2], p.Packages[0] }}, {"duplicate", func(p *Plan) { p.Packages[1] = p.Packages[0] }}, {"channel", func(p *Plan) { p.Channel = "stable" }}, {"commit mismatch", func(p *Plan) { p.Commit = strings.Repeat("c", 40) }}} {
		t.Run(tc.n, func(t *testing.T) {
			p := fixture()
			tc.edit(&p)
			if _, e := p.Hash(); e == nil {
				t.Fatal("accepted invalid plan")
			}
		})
	}
}
func TestExecuteOrderAndCheckpoint(t *testing.T) {
	s := store(t)
	p := fixture()
	f := &fake{}
	f.callback = func(v string, _ context.Context) {
		j, e := s.Load()
		if e != nil {
			t.Fatal(e)
		}
		if strings.HasPrefix(v, "install:") && j.Phase != Applying {
			t.Fatal(j.Phase)
		}
		if strings.HasPrefix(v, "verify:") && j.Phase != Verifying {
			t.Fatal(j.Phase)
		}
	}
	j, e := s.Execute(context.Background(), p, f)
	if e != nil || j.Phase != Completed || j.Next != 3 {
		t.Fatal(j, e)
	}
	var want []string
	for _, v := range p.Packages {
		want = append(want, "prepare:"+v.Name)
	}
	for _, v := range p.Packages {
		for _, k := range []string{"check", "install", "verify"} {
			want = append(want, k+":"+v.Name)
		}
	}
	if !reflect.DeepEqual(want, f.calls) {
		t.Fatal(f.calls)
	}
	loaded, e := s.Load()
	if e != nil || !reflect.DeepEqual(j, loaded) {
		t.Fatal(loaded, e)
	}
	if _, e = s.Execute(context.Background(), p, &fake{}); !errors.Is(e, ErrRecovery) {
		t.Fatal(e)
	}
}
func TestFirstFailureStops(t *testing.T) {
	for _, tc := range []struct {
		stage string
		phase Phase
		next  int
	}{{"prepare", Failed, 0}, {"check", Failed, 1}, {"install", RecoveryRequired, 1}, {"verify", RecoveryRequired, 1}} {
		t.Run(tc.stage, func(t *testing.T) {
			s := store(t)
			f := &fake{fail: tc.stage + ":routerforge-profiling"}
			j, e := s.Execute(context.Background(), fixture(), f)
			if e == nil || j.Phase != tc.phase || j.Next != tc.next {
				t.Fatal(j, e)
			}
			if f.calls[len(f.calls)-1] != f.fail {
				t.Fatal(f.calls)
			}
			if _, e = s.Execute(context.Background(), fixture(), &fake{}); !errors.Is(e, ErrRecovery) {
				t.Fatal(e)
			}
		})
	}
}
func TestCancelDuringInstallAtBoundary(t *testing.T) {
	s := store(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fake{callback: func(v string, c context.Context) {
		if v == "install:routerforge-dns-router" {
			cancel()
			if c.Err() != nil {
				t.Fatal("mutation cancelled")
			}
		}
	}}
	j, e := s.Execute(ctx, fixture(), f)
	if !errors.Is(e, context.Canceled) || j.Phase != Cancelled || j.Next != 1 || f.calls[len(f.calls)-1] != "verify:routerforge-dns-router" {
		t.Fatal(j, e, f.calls)
	}
}
func TestBusyAndInterrupted(t *testing.T) {
	s := store(t)
	l, e := s.lock()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Execute(context.Background(), fixture(), &fake{}); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	l.Close()
	p := fixture()
	h, _ := p.Hash()
	if e = s.save(Journal{Schema: Schema, Plan: p, PlanSHA256: h, Phase: Applying}); e != nil {
		t.Fatal(e)
	}
	f := &fake{}
	if _, e = s.Execute(context.Background(), p, f); !errors.Is(e, ErrRecovery) || len(f.calls) != 0 {
		t.Fatal(e, f.calls)
	}
}
func TestUnsafeAndTamperedJournal(t *testing.T) {
	s := store(t)
	p := fixture()
	h, _ := p.Hash()
	j := Journal{Schema: Schema, Plan: p, PlanSHA256: h, Phase: Prepared}
	if e := s.save(j); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(s.dir, "current.json")
	b, _ := os.ReadFile(path)
	b = []byte(strings.Replace(string(b), "test", "tampered", 1))
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Load(); e == nil {
		t.Fatal("tamper accepted")
	}
	if e := os.Remove(path); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("/dev/null", path); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Load(); e == nil {
		t.Fatal("symlink accepted")
	}
	if _, e := s.Execute(context.Background(), p, &fake{}); e == nil {
		t.Fatal("unsafe journal overwritten")
	}
}
func TestPrivateDirectory(t *testing.T) {
	s := store(t)
	if e := os.Chmod(s.dir, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := Open(s.dir); e == nil {
		t.Fatal("public dir")
	}
	if _, e := Open(fmt.Sprintf("%s/..", s.dir)); e == nil {
		t.Fatal("unclean dir")
	}
}

func TestActualExecutorExit(t *testing.T) {
	if dir := os.Getenv("RF_TXN_TEST_EXIT_DIR"); dir != "" {
		s, e := Open(dir)
		if e != nil {
			t.Fatal(e)
		}
		f := &fake{callback: func(v string, _ context.Context) {
			if strings.HasPrefix(v, "install:") {
				os.Exit(77)
			}
		}}
		_, _ = s.Execute(context.Background(), fixture(), f)
		t.Fatal("expected process exit")
	}
	s := store(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestActualExecutorExit$")
	cmd.Env = append(os.Environ(), "RF_TXN_TEST_EXIT_DIR="+s.dir)
	out, e := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(e, &exit) || exit.ExitCode() != 77 {
		t.Fatalf("expected 77: %v %s", e, out)
	}
	j, e := s.Load()
	if e != nil || j.Phase != Applying || j.Next != 0 {
		t.Fatal(j, e)
	}
	f := &fake{}
	if _, e = s.Execute(context.Background(), fixture(), f); !errors.Is(e, ErrRecovery) || len(f.calls) != 0 {
		t.Fatal(e, f.calls)
	}
}
