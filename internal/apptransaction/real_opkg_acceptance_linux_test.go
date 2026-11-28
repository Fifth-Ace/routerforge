//go:build linux && rf_opkg_acceptance

package apptransaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const labRoot = "/opt/var/rf-acceptance"
const labConfig = "/opt/etc/routerforge/profiling.conf"
const labOldVersion = "1~dev.r1.bf8c6d5899a9"
const labCore = "routerforge-core"
const labProfile = "routerforge-profiling"
const labGate = "/opt/var/rf-acceptance/entered"
const labReady = "/opt/var/rf-acceptance/ready"
const labContinue = "/opt/var/rf-acceptance/continue"
const labService = "/opt/etc/init.d/S90routerforge"
const labConfigText = "listen=127.0.0.1:6061\nslow_ms=913\n"

func labMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func labWrite(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	labMust(t, os.WriteFile(path, content, mode))
}
func labRead(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	labMust(t, e)
	return b
}
func labExists(path string) bool { _, e := os.Stat(path); return e == nil }
func labWait(t *testing.T, name string, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if b, e := os.ReadFile(filepath.Join(labRoot, "coordinator/active/worker.log")); e == nil {
		t.Logf("worker log: %s", b)
	}
	if b, e := os.ReadFile(filepath.Join(labRoot, "service.log")); e == nil {
		t.Logf("service log: %s", b)
	}
	t.Fatalf("timeout: %s", name)
}

// Never run with the live device's /opt. This test is build-tagged and only
// accepts the explicitly provisioned, empty ephemeral container mount.
func labGuard(t *testing.T) {
	t.Helper()
	if os.Getenv("RF_OPKG_ACCEPTANCE") != "isolated-ci-v1" {
		t.Skip("isolated real-OPKG acceptance only")
	}
	if os.Geteuid() != 0 {
		t.Fatal("lab must run as root in its private container")
	}
	raw := string(labRead(t, "/proc/self/mountinfo"))
	found := false
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) > 6 && f[4] == "/opt" && strings.Contains(line, " - tmpfs ") {
			found = true
		}
	}
	if !found || !labExists("/lab/rf-isolated-lab-v1") {
		t.Fatal("no disposable /opt tmpfs or lab marker")
	}
}
func labOPKG(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd, e := safety.CommandContext(ctx, "/opt/bin/opkg", args...)
	labMust(t, e)
	b, e := cmd.CombinedOutput()
	t.Logf("opkg %v: %s", args, b)
	labMust(t, e)
	return string(b)
}
func labVersion(t *testing.T, name string) string {
	t.Helper()
	v, e := installedDevVersion(labOPKG(t, "status", name), name)
	labMust(t, e)
	return v
}
func labPlan(t *testing.T) Plan {
	t.Helper()
	sha := os.Getenv("RF_LAB_COMMIT")
	if len(sha) != 40 {
		t.Fatal("invalid lab commit")
	}
	version := "1~dev.r2." + sha[:12]
	raw := labRead(t, "/lab/new.ipk")
	sum := sha256.Sum256(raw)
	p := Plan{ID: "real-opkg-acceptance", Channel: "dev", Target: "aarch64-3.10", Commit: sha, Packages: []Package{{Name: labProfile, Version: version, SHA256: hex.EncodeToString(sum[:]), Asset: labProfile + "_" + strings.ReplaceAll(version, "~", "-") + "_aarch64-3.10.ipk", Size: int64(len(raw))}}}
	_, e := p.Hash()
	labMust(t, e)
	return p
}

// This probe deliberately observes a test service, not the real Core HTTP
// listener or data plane. The package versions and OPKG are real. The actual
// production lifecycle candidates have their own regression tests.
func labProbe(_ context.Context, p Package) (LifecycleObservation, error) {
	if p.Name != labProfile {
		return LifecycleObservation{}, errors.New("unexpected package")
	}
	if !labExists(labReady) {
		return LifecycleObservation{}, errors.New("test service not ready")
	}
	cfg, e := os.ReadFile(labConfig)
	if e != nil {
		return LifecycleObservation{}, e
	}
	if string(cfg) != labConfigText {
		return LifecycleObservation{}, errors.New("config changed")
	}
	marker, _, e := readCoreFile(profilingMarkerPath, uint32(os.Geteuid()))
	if e != nil {
		return LifecycleObservation{}, e
	}
	sum := sha256.Sum256(append([]byte(marker+"\x00"), cfg...))
	return LifecycleObservation{Mode: "running", Enabled: marker != "", Ready: true, RuntimeVersion: labOldVersion, IntentSHA256: hex.EncodeToString(sum[:])}, nil
}
func TestRealOPKGChild(t *testing.T) {
	role := os.Getenv("RF_LAB_ROLE")
	if role == "" {
		return
	}
	labGuard(t)
	c, e := OpenCoordinator(filepath.Join(labRoot, "coordinator"))
	labMust(t, e)
	p := labPlan(t)
	switch role {
	case "launcher":
		exe, e := os.Executable()
		labMust(t, e)
		// The child inherits the real Coordinator FD. Its role is selected by an
		// argument, not by exposing a production CLI or accepting arbitrary input.
		pid, e := c.Start(exe, []string{"-test.run=^TestRealOPKGWorker$", "-test.v"}, p)
		labMust(t, e)
		labWrite(t, filepath.Join(labRoot, "worker-pid"), []byte(fmt.Sprint(pid)), 0600)
		// Parent is intentionally killed by the test while OPKG is in progress.
		time.Sleep(60 * time.Second)
	default:
		t.Fatal("invalid child role")
	}
}
func TestRealOPKGWorker(t *testing.T) {
	if os.Getenv("RF_LAB_ROLE") != "launcher" {
		return
	}
	labGuard(t)
	c, e := OpenCoordinator(filepath.Join(labRoot, "coordinator"))
	labMust(t, e)
	p := labPlan(t)
	b := labRead(t, "/lab/new.ipk")
	if os.Getenv("RF_LAB_CASE") == "corrupt" {
		b[0] ^= 0xff
	}
	// Prestage the exact local test IPK; Staging.Prepare must re-hash it and
	// cannot fetch anything (container network is disabled).
	path := filepath.Join(c.slot(), p.Packages[0].Asset)
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	labMust(t, e)
	_, e = f.Write(b)
	labMust(t, e)
	labMust(t, f.Sync())
	labMust(t, f.Close())
	j, e := c.RunVerifiedWorker(context.Background(), labProbe)
	if e != nil {
		t.Logf("worker stopped: phase=%s err=%v", j.Phase, e)
		return
	}
	if j.Phase != Completed {
		t.Fatal(j)
	}
}
func TestRealOPKGAcceptance(t *testing.T) {
	if os.Getenv("RF_LAB_ROLE") != "" {
		return
	}
	labGuard(t)
	mode := os.Getenv("RF_LAB_CASE")
	switch mode {
	case "on", "off", "worker-crash", "corrupt", "failed-start":
	default:
		t.Fatal("invalid acceptance case")
	}
	entries, e := os.ReadDir("/opt")
	labMust(t, e)
	if len(entries) != 0 {
		t.Fatal("/opt is not empty")
	}
	for _, d := range []string{"/opt/bin", "/opt/etc/init.d", "/opt/etc/routerforge", "/opt/var/lock", "/opt/lib/opkg", "/opt/lib/opkg/info", "/opt/var/opkg-lists", "/opt/tmp", labRoot} {
		labMust(t, os.MkdirAll(d, 0700))
	}
	labWrite(t, "/opt/bin/opkg", labRead(t, "/lab/opkg"), 0700)
	labWrite(t, "/opt/etc/opkg.conf", []byte("dest root /\nlists_dir ext /opt/var/opkg-lists\narch all 1\narch aarch64-3.10 100\n"), 0600)
	labOPKG(t, "--version")
	labOPKG(t, "install", "/lab/core-fixture.ipk")
	labOPKG(t, "install", "/lab/old.ipk")
	if labVersion(t, labCore) != labOldVersion || labVersion(t, labProfile) != labOldVersion {
		t.Fatal("wrong baseline versions")
	}
	labWrite(t, labConfig, []byte(labConfigText), 0644)
	on := mode == "on"
	if on {
		labWrite(t, profilingMarkerPath, nil, 0644)
	} else if e = os.Remove(profilingMarkerPath); e != nil && !errors.Is(e, os.ErrNotExist) {
		t.Fatal(e)
	}
	labWrite(t, labReady, []byte("baseline\n"), 0600)
	labWrite(t, labService, labRead(t, "/lab/service-fixture.sh"), 0700)
	defer func() {
		if b, e := os.ReadFile(filepath.Join(labRoot, "service.log")); e == nil {
			t.Logf("observed real maintainer-script service invocations:\n%s", b)
		}
		if b, e := os.ReadFile(filepath.Join(labRoot, "coordinator/active/worker.log")); e == nil {
			t.Logf("detached worker:\n%s", b)
		}
	}()
	dir := filepath.Join(labRoot, "coordinator")
	labMust(t, os.Mkdir(dir, 0700))
	c, e := OpenCoordinator(dir)
	labMust(t, e)
	exe, e := os.Executable()
	labMust(t, e)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd, e := safety.CommandContext(ctx, exe, "-test.run=^TestRealOPKGChild$", "-test.v")
	labMust(t, e)
	cmd.Env = append(os.Environ(), "RF_LAB_ROLE=launcher")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	labMust(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	pidPath := filepath.Join(labRoot, "worker-pid")
	labWait(t, "worker PID", func() bool { return labExists(pidPath) })
	if mode == "corrupt" {
		labWait(t, "corrupt stage rejection", func() bool {
			v, e := c.Status()
			return e == nil && v.Journal != nil && v.Journal.Phase == Failed && v.State == string(RecoveryRequired)
		})
		if labExists(labGate) || labVersion(t, labProfile) != labOldVersion || labExists(filepath.Join(c.slot(), "profiling-marker-before.json")) {
			t.Fatal("corrupt IPK reached OPKG")
		}
		if f, e := c.AcquireLegacy(); !errors.Is(e, ErrRecovery) {
			if f != nil {
				f.Close()
			}
			t.Fatal("replay guard", e)
		}
		t.Log("REAL_OPKG_CORRUPTION_REJECTED_BEFORE_INSTALL=PASS")
		return
	}
	labWait(t, "real OPKG postinst gate", func() bool { return labExists(labGate) })
	status, e := c.Status()
	labMust(t, e)
	if status.Journal == nil || status.Journal.Phase != Applying {
		t.Fatal("wrong in-flight phase", status)
	}
	if f, e := c.AcquireLegacy(); !errors.Is(e, ErrBusy) {
		if f != nil {
			f.Close()
		}
		t.Fatal("parallel legacy mutation allowed", e)
	}
	// Kill the launching process during the actual OPKG call, not just before it.
	labMust(t, cmd.Process.Kill())
	_ = cmd.Wait()
	if f, e := c.AcquireLegacy(); !errors.Is(e, ErrBusy) {
		if f != nil {
			f.Close()
		}
		t.Fatal("lease lost after launcher termination", e)
	}
	if mode == "worker-crash" {
		var pid int
		_, e = fmt.Sscan(string(labRead(t, pidPath)), &pid)
		labMust(t, e)
		if pid <= 1 {
			t.Fatal("unsafe worker pid")
		}
		worker, e := os.FindProcess(pid)
		labMust(t, e)
		labMust(t, worker.Kill())
		// After the worker is gone, the real OPKG process must still own the lock.
		labWait(t, "worker exited", func() bool {
			b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
			if errors.Is(e, os.ErrNotExist) {
				return true
			}
			if e != nil {
				return false
			}
			s := string(b)
			i := strings.LastIndex(s, ") ")
			return i >= 0 && len(s) > i+2 && s[i+2] == 'Z'
		})
		if f, e := AcquireMutationLease(dir); !errors.Is(e, ErrBusy) {
			if f != nil {
				f.Close()
			}
			t.Fatal("OPKG did not retain lease after worker death", e)
		}
	}
	labWrite(t, labContinue, []byte("continue\n"), 0600)
	want := Completed
	if mode == "worker-crash" || mode == "failed-start" {
		want = RecoveryRequired
	}
	labWait(t, "final transaction state", func() bool {
		v, e := c.Status()
		if e != nil || v.State != string(want) || v.Journal == nil {
			return false
		}
		if mode == "worker-crash" {
			return v.Journal.Phase == Applying
		}
		return v.Journal.Phase == want
	})
	// A terminal checkpoint may precede process exit. Wait for the lock, without
	// ever unlocking a foreign descriptor. Unresolved journal remains blocking.
	labWait(t, "lease released by real OPKG and worker", func() bool {
		f, e := AcquireMutationLease(dir)
		if e != nil {
			return false
		}
		f.Close()
		return true
	})
	p := labPlan(t)
	record := labRead(t, filepath.Join(c.slot(), "profiling-marker-before.json"))
	var r profilingMigrationRecord
	labMust(t, json.Unmarshal(record, &r))
	expect := "off"
	if on {
		expect = "on"
	}
	if r.Intent != expect || r.From != labOldVersion || r.To != p.Packages[0].Version || r.AssetSHA256 != p.Packages[0].SHA256 {
		t.Fatal("wrong migration evidence", r)
	}
	if labExists(profilingMarkerPath) != on || string(labRead(t, labConfig)) != labConfigText {
		t.Fatal("profiling intent or config lost")
	}
	if mode == "on" || mode == "off" {
		if labVersion(t, labProfile) != p.Packages[0].Version || !labExists(labReady) || !labExists(filepath.Join(c.slot(), "lifecycle-after-"+labProfile+".json")) {
			t.Fatal("missing verified success")
		}
	}
	if mode == "worker-crash" {
		if labVersion(t, labProfile) != p.Packages[0].Version || !labExists(labReady) || labExists(filepath.Join(c.slot(), "lifecycle-after-"+labProfile+".json")) {
			t.Fatal("OPKG did not complete independently, or worker incorrectly recorded post-proof")
		}
	}
	if mode == "on" || mode == "off" || mode == "worker-crash" {
		var daemon int
		_, e = fmt.Sscan(string(labRead(t, filepath.Join(labRoot, "daemon-pid"))), &daemon)
		labMust(t, e)
		if daemon <= 1 || !labExists(fmt.Sprintf("/proc/%d", daemon)) {
			t.Fatal("long-lived service fixture did not survive package installation")
		}
	}
	if mode == "failed-start" && labExists(labReady) {
		t.Fatal("failed service marked ready")
	}
	if _, e = c.Start(exe, nil, p); !errors.Is(e, ErrRecovery) {
		t.Fatal("replay accepted", e)
	}
	if want == RecoveryRequired {
		if f, e := c.AcquireLegacy(); !errors.Is(e, ErrRecovery) {
			if f != nil {
				f.Close()
			}
			t.Fatal("unresolved journal allowed legacy", e)
		}
	}
	t.Logf("REAL_OPKG_CASE=%s RESULT=%s / EXPECTED=%s / PASS", mode, want, want)
}
