//go:build linux

package apptransaction

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executionFixture(t *testing.T) (Plan, CooperativeVersionSet, acceptedExecution) {
	t.Helper()
	_, p, pkg := lifecycleFixture(t)
	core := pkg
	core.Name = "routerforge-core"
	core.Asset = "routerforge-core" + pkg.Asset[len(pkg.Name):]
	p.Packages = append(p.Packages, core)
	old := "1~dev.r1458.000000000000"
	initial := CooperativeVersionSet{}
	for _, name := range devicePackageNames() {
		initial[name] = old
	}
	intermediate := copyVersionSet(initial)
	intermediate[pkg.Name] = pkg.Version
	final := copyVersionSet(intermediate)
	final[core.Name] = core.Version
	return p, initial, acceptedExecution{old, p.Commit, append([]Package(nil), p.Packages...), copyVersionSet(initial), []CooperativeVersionSet{copyVersionSet(initial), intermediate, final}}
}
func TestExecutionPolicyExactAssetsAndIntermediate(t *testing.T) {
	p, initial, e := executionFixture(t)
	if DeviceExecutionConfigured(e.WorkerVersion) {
		t.Fatal("unaccepted policy shipped")
	}
	if _, err := selectExecutionPolicy(e.WorkerVersion, p, initial, nil); !errors.Is(err, ErrExecutionLocked) {
		t.Fatal(err)
	}
	seq, err := selectExecutionPolicy(e.WorkerVersion, p, initial, []acceptedExecution{e})
	if err != nil || len(seq) != 3 {
		t.Fatal(seq, err)
	}
	if _, err = selectExecutionPolicy(e.WorkerVersion, p, initial, []acceptedExecution{e, e}); err == nil {
		t.Fatal("ambiguous policy accepted")
	}
	changed := p
	changed.Packages = append([]Package(nil), p.Packages...)
	changed.Packages[0].SHA256 = strings.Repeat("f", 64)
	if _, err = selectExecutionPolicy(e.WorkerVersion, changed, initial, []acceptedExecution{e}); err == nil {
		t.Fatal("changed asset accepted")
	}
	e.Accepted = []CooperativeVersionSet{e.Accepted[0], e.Accepted[2]}
	if _, err = selectExecutionPolicy(e.WorkerVersion, p, initial, []acceptedExecution{e}); err == nil {
		t.Fatal("missing intermediate accepted")
	}
}
func TestExecutionInstalledListRejectsAmbiguity(t *testing.T) {
	_, initial, _ := executionFixture(t)
	var lines []string
	for _, name := range devicePackageNames() {
		lines = append(lines, name+" - "+initial[name])
	}
	raw := strings.Join(lines, "\n") + "\n"
	if got, err := parseDeviceInstalledList(raw + "busybox - 1.0\n"); err != nil || !equalCooperativeSets(initial, got) {
		t.Fatal(got, err)
	}
	for _, bad := range []string{raw + lines[0] + "\n", raw + "dns-monitor - 1.0\n", raw + "routerforge-maintenance - 1.0\n", strings.Join(lines[1:], "\n"), raw + "bad line\n"} {
		if _, err := parseDeviceInstalledList(bad); err == nil {
			t.Fatal("invalid inventory accepted")
		}
	}
}

type executionTestHooks struct {
	install func(Package)
	calls   int
}

func (h *executionTestHooks) Prepare(context.Context, Package) error { return nil }
func (h *executionTestHooks) Check(context.Context, Package) error   { return nil }
func (h *executionTestHooks) Install(_ context.Context, p Package) error {
	h.calls++
	h.install(p)
	return nil
}
func (h *executionTestHooks) Verify(context.Context, Package) error { return nil }
func TestExecutionBoundaryAndUnselectedDrift(t *testing.T) {
	for _, mode := range []string{"success", "before", "after", "unselected_intent"} {
		t.Run(mode, func(t *testing.T) {
			s, _, _ := lifecycleFixture(t)
			p, initial, entry := executionFixture(t)
			seq, err := selectExecutionPolicy(entry.WorkerVersion, p, initial, []acceptedExecution{entry})
			if err != nil {
				t.Fatal(err)
			}
			current := copyVersionSet(initial)
			changed := false
			h := &executionTestHooks{install: func(pkg Package) {
				current[pkg.Name] = pkg.Version
				if mode == "after" {
					current["routerforge-admin"] = "1~dev.r1.aaaaaaaaaaaa"
				}
				if mode == "unselected_intent" {
					changed = true
				}
			}}
			observe := func(_ context.Context, pkg Package) (lifecycleEvidence, error) {
				intent := "baseline"
				if changed && pkg.Name == "routerforge-admin" {
					intent = "changed"
				}
				return lifecycleEvidence{Package: pkg.Name, InstalledVersion: current[pkg.Name], Observation: LifecycleObservation{Mode: "running", Ready: true, Enabled: true, IntentSHA256: intent}}, nil
			}
			e, err := newExecutionBoundary(context.Background(), p, h, seq, func(context.Context) (CooperativeVersionSet, error) { return copyVersionSet(current), nil }, observe)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "before" {
				current["routerforge-admin"] = "1~dev.r1.aaaaaaaaaaaa"
			}
			journal, err := s.Execute(context.Background(), p, e)
			switch mode {
			case "success":
				if err != nil || journal.Phase != Completed || h.calls != 2 {
					t.Fatal(journal, err, h.calls)
				}
			case "before":
				if err == nil || h.calls != 0 {
					t.Fatal(journal, err, h.calls)
				}
			default:
				if err == nil || journal.Phase != RecoveryRequired {
					t.Fatal(journal, err)
				}
			}
		})
	}
}
func TestStartCheckedGateNoSlotMutation(t *testing.T) {
	s, p, _ := lifecycleFixture(t)
	c, err := OpenCoordinator(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	_, err = c.startChecked("/not-executed", nil, p, func() error {
		called = true
		if l, e := c.AcquireLegacy(); !errors.Is(e, ErrBusy) {
			if l != nil {
				l.Close()
			}
			t.Fatal("lease not held", e)
		}
		return ErrExecutionLocked
	})
	if !called || !errors.Is(err, ErrExecutionLocked) {
		t.Fatal(called, err)
	}
	if status, err := c.Status(); err != nil || status.State != "idle" {
		t.Fatal(status, err)
	}
}

func TestAcceptanceDocumentRootOwnedCanonicalAndExplicit(t *testing.T) {
	s, _, _ := lifecycleFixture(t)
	_, _, e := executionFixture(t)
	if _, err := readAcceptancePolicy(s.dir); err == nil {
		t.Fatal("missing policy accepted")
	}
	p := deviceAcceptancePolicy{1, "RF_HARDWARE_ACCEPTED", strings.Repeat("a", 64), e.WorkerVersion, strings.Repeat("b", 64), []acceptedExecution{e}}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, "accepted-execution-policy.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readAcceptancePolicy(s.dir); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = readAcceptancePolicy(s.dir); err == nil {
		t.Fatal("public policy accepted")
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(strings.Replace(string(raw), `"schema":1`, `"schema":1,"schema":1`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readAcceptancePolicy(s.dir); err == nil {
		t.Fatal("duplicate field accepted")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink("/etc/passwd", path); err != nil {
		t.Fatal(err)
	}
	if _, err = readAcceptancePolicy(s.dir); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestDisableMarkerAbsentAndUnsafe(t *testing.T) {
	root := t.TempDir()
	uid := uint32(os.Geteuid())
	if h, err := readDisabledMarker(root, "admin", uid); err != nil || h != "" {
		t.Fatal(h, err)
	}
	d := filepath.Join(root, "modules-disabled")
	if err := os.Mkdir(d, 0700); err != nil {
		t.Fatal(err)
	}
	if h, err := readDisabledMarker(root, "admin", uid); err != nil || h != "" {
		t.Fatal(h, err)
	}
	if err := os.WriteFile(filepath.Join(d, "admin"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if h, err := readDisabledMarker(root, "admin", uid); err != nil || h == "" {
		t.Fatal(h, err)
	}
	if err := os.Remove(filepath.Join(d, "admin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(d); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, d); err != nil {
		t.Fatal(err)
	}
	if _, err := readDisabledMarker(root, "admin", uid); err == nil {
		t.Fatal("unsafe parent accepted")
	}
}
