//go:build linux

package apptransaction

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestC7ProfilingMigrationRecord(t *testing.T) {
	for _, mode := range []string{"on", "off", "custom", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			s, _, p := lifecycleFixture(t)
			p.Name = "routerforge-profiling"
			p.Asset = strings.Replace(p.Asset, "routerforge-dns", "routerforge-profiling", 1)
			marker := filepath.Join(s.dir, "marker")
			uid := uint32(os.Geteuid())
			if mode == "on" || mode == "custom" {
				b := []byte{}
				if mode == "custom" {
					b = []byte("custom")
				}
				if e := os.WriteFile(marker, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "symlink" {
				if e := os.Symlink("missing", marker); e != nil {
					t.Fatal(e)
				}
			}
			intent, e := recordProfilingMigration(s, p, "1~dev.r1458.000000000000", marker, uid)
			if mode == "custom" || mode == "symlink" {
				if e == nil {
					t.Fatal("unsafe marker accepted")
				}
				return
			}
			if e != nil || intent != mode {
				t.Fatal(intent, e)
			}
			b, e := os.ReadFile(filepath.Join(s.dir, "profiling-marker-before.json"))
			if e != nil {
				t.Fatal(e)
			}
			var rec profilingMigrationRecord
			if e = json.Unmarshal(b, &rec); e != nil || rec.Intent != mode || rec.AssetSHA256 != p.SHA256 || rec.To != p.Version {
				t.Fatal(rec, e)
			}
			if _, e = recordProfilingMigration(s, p, "1~dev.r1458.000000000000", marker, uid); e == nil {
				t.Fatal("replay accepted")
			}
		})
	}
}
func TestC7ExecutionEnvironment(t *testing.T) {
	base := []string{"PATH=/bin", leaseEnv + "=99", profilingIntentEnv + "=on", profilingIntentEnv + "=off"}
	for _, intent := range []string{"", "on", "off"} {
		env, e := executionEnvironment(base, intent)
		if e != nil {
			t.Fatal(e)
		}
		expected := "PATH=/bin," + leaseEnv + "=3"
		if intent != "" {
			expected += "," + profilingIntentEnv + "=" + intent
		}
		if strings.Join(env, ",") != expected {
			t.Fatal(env)
		}
	}
	if _, e := executionEnvironment(base, "invalid"); e == nil {
		t.Fatal("invalid intent accepted")
	}
}
func TestC7ForeignLeaseHolder(t *testing.T) {
	d := t.TempDir()
	if e := os.Chmod(d, 0700); e != nil {
		t.Fatal(e)
	}
	lease, e := AcquireMutationLease(d)
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	proc := filepath.Join(d, "proc")
	fds := filepath.Join(proc, "321", "fd")
	if e = os.MkdirAll(fds, 0700); e != nil {
		t.Fatal(e)
	}
	uid := uint32(os.Geteuid())
	if pid, e := foreignLeaseHolder(proc, lease, uid, 123); e != nil || pid != 0 {
		t.Fatal(pid, e)
	}
	if e = os.Symlink(filepath.Join(d, mutationLockName), filepath.Join(fds, "19")); e != nil {
		t.Fatal(e)
	}
	if pid, e := foreignLeaseHolder(proc, lease, uid, 123); e != nil || pid != 321 {
		t.Fatal(pid, e)
	}
	if pid, e := foreignLeaseHolder(proc, lease, uid, 321); e != nil || pid != 0 {
		t.Fatal(pid, e)
	}
}

// Real child holds FD 3. Restrict the observer to that child via a private
// fixture view, so unrelated CI/container /proc visibility cannot fake a failure.
func TestC7LeaseChild(t *testing.T) {
	if os.Getenv("RF_C7_TEST_LEASE_CHILD") != "1" {
		return
	}
	f := os.NewFile(3, "inherited-lease")
	if f == nil {
		os.Exit(8)
	}
	if _, e := f.Stat(); e != nil {
		os.Exit(9)
	}
	if _, e := os.Stdout.Write([]byte("READY\n")); e != nil {
		os.Exit(10)
	}
	var b [1]byte
	_, _ = os.Stdin.Read(b[:])
	_ = f.Close()
	os.Exit(0)
}
func TestC7RealInheritedLeaseHolder(t *testing.T) {
	d := t.TempDir()
	if e := os.Chmod(d, 0700); e != nil {
		t.Fatal(e)
	}
	lease, e := AcquireMutationLease(d)
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestC7LeaseChild$")
	cmd.Env = append(os.Environ(), "RF_C7_TEST_LEASE_CHILD=1", "GORACE=atexit_sleep_ms=0")
	cmd.ExtraFiles = []*os.File{lease}
	out, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	in, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_ = in.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	line, e := bufio.NewReader(out).ReadString('\n')
	if e != nil || line != "READY\n" {
		t.Fatal(line, e)
	}
	proc := filepath.Join(d, "proc")
	pid := strconv.Itoa(cmd.Process.Pid)
	if e = os.MkdirAll(filepath.Join(proc, pid), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(filepath.Join("/proc", pid, "fd"), filepath.Join(proc, pid, "fd")); e != nil {
		t.Fatal(e)
	}
	if got, e := foreignLeaseHolder(proc, lease, uint32(os.Geteuid()), os.Getpid()); e != nil || got != cmd.Process.Pid {
		t.Fatal(got, e)
	}
	// Parent closes without unlocking; child still owns the kernel flock.
	if e = lease.Close(); e != nil {
		t.Fatal(e)
	}
	if f, e := AcquireMutationLease(d); !errors.Is(e, ErrBusy) {
		if f != nil {
			f.Close()
		}
		t.Fatal("lease was lost", e)
	}
	if e = in.Close(); e != nil {
		t.Fatal(e)
	}
	if e = cmd.Wait(); e != nil {
		t.Fatal(e)
	}
	f, e := AcquireMutationLease(d)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if got, e := foreignLeaseHolder(proc, f, uint32(os.Geteuid()), os.Getpid()); e != nil || got != 0 {
		t.Fatal(got, e)
	}
}
