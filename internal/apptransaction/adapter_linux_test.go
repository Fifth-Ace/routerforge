//go:build linux

package apptransaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func indexFixture() releaseIndex {
	p := fixture()
	x := releaseIndex{Schema: 1, Channel: p.Channel, Target: p.Target, Commit: p.Commit}
	for n := range official {
		a := n + "_" + strings.ReplaceAll(p.Packages[0].Version, "~", "-") + "_" + p.Target + ".ipk"
		x.Components = append(x.Components, releaseEntry{n, p.Packages[0].Version, strings.Repeat("b", 64), a, 123, p.Target, legacyReleaseBase + a, releaseBase + a})
	}
	return x
}
func encodeIndex(t *testing.T, x releaseIndex) []byte {
	t.Helper()
	b, e := json.Marshal(x)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestFreezeDevPlan(t *testing.T) {
	x := indexFixture()
	p, e := FreezeDevPlan(encodeIndex(t, x), "frozen", x.Commit, []string{"routerforge-core", "routerforge-profiling", "routerforge-dns-router"})
	if e != nil {
		t.Fatal(e)
	}
	if p.Packages[0].Name != "routerforge-dns-router" || p.Packages[1].Name != "routerforge-profiling" || p.Packages[2].Name != "routerforge-core" {
		t.Fatal(p)
	}
	h, _ := p.Hash()
	x.Components[0].SHA256 = strings.Repeat("c", 64)
	h2, _ := p.Hash()
	if h != h2 {
		t.Fatal("plan mutated")
	}
	for _, v := range []struct {
		name string
		edit func(*releaseIndex)
	}{
		{"wrong schema", func(x *releaseIndex) { x.Schema = 2 }}, {"wrong channel", func(x *releaseIndex) { x.Channel = "stable" }}, {"wrong architecture", func(x *releaseIndex) { x.Components[0].Architecture = "mips" }}, {"incomplete index", func(x *releaseIndex) { x.Components = x.Components[:10] }}, {"foreign URL", func(x *releaseIndex) { x.Components[0].CanonicalURL = "https://evil.example/" + x.Components[0].Asset }}, {"query injection", func(x *releaseIndex) { x.Components[0].CanonicalURL += "?foo=bar" }}, {"duplicate package", func(x *releaseIndex) { x.Components[0] = x.Components[1] }}, {"mixed versions", func(x *releaseIndex) { x.Components[0].Version = "1~dev.r1.aaaaaaaaaaaa" }}, {"size zero", func(x *releaseIndex) { x.Components[0].Size = 0 }}, {"invalid SHA", func(x *releaseIndex) { x.Components[0].SHA256 = "broken" }},
	} {
		t.Run(v.name, func(t *testing.T) {
			x := indexFixture()
			v.edit(&x)
			if _, e := FreezeDevPlan(encodeIndex(t, x), "frozen", x.Commit, []string{"routerforge-core"}); e == nil {
				t.Fatal("accepted invalid release")
			}
		})
	}
	x = indexFixture()
	for _, sel := range [][]string{{}, {"routerforge-core", "routerforge-core"}, {"routerforge-external"}} {
		if _, e := FreezeDevPlan(encodeIndex(t, x), "frozen", x.Commit, sel); e == nil {
			t.Fatal("accepted invalid selection")
		}
	}
	if _, e := FreezeDevPlan(encodeIndex(t, x), "frozen", strings.Repeat("c", 40), []string{"routerforge-core"}); e == nil {
		t.Fatal("accepted stale release")
	}
}
func TestRedirectPolicy(t *testing.T) {
	for _, v := range []struct {
		u  string
		ok bool
	}{{"https://release-assets.githubusercontent.com/a?sig=x", true}, {"https://objects.githubusercontent.com/a", true}, {"http://objects.githubusercontent.com/a", false}, {"https://objects.githubusercontent.com.evil.test/a", false}, {"https://user@objects.githubusercontent.com/a", false}, {"https://objects.githubusercontent.com:443/a", false}, {"https://evil.github.io/a", false}} {
		u, e := url.Parse(v.u)
		if e != nil || allowedReleaseRedirect(u) != v.ok {
			t.Fatal(v, e)
		}
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func stagedFixture(t *testing.T, data []byte) (*Staging, Package) {
	t.Helper()
	p := fixture()
	p.Packages = p.Packages[:1]
	h := sha256.Sum256(data)
	p.Packages[0].SHA256 = hex.EncodeToString(h[:])
	p.Packages[0].Size = int64(len(data))
	s := store(t)
	a, e := NewStaging(s.dir, p)
	if e != nil {
		t.Fatal(e)
	}
	return a, p.Packages[0]
}
func setBody(t *testing.T, a *Staging, p Package, body []byte, code int, requests *int) {
	t.Helper()
	a.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		*requests++
		if r.URL.String() != releaseBase+p.Asset || r.Method != "GET" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header)}, nil
	})
}
func TestStageVerifiedBytes(t *testing.T) {
	data := []byte("exact package fixture")
	a, p := stagedFixture(t, data)
	n := 0
	setBody(t, a, p, data, 200, &n)
	ctx := context.Background()
	if e := a.Prepare(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := a.Prepare(ctx, p); e != nil || n != 1 {
		t.Fatal(e, n)
	}
	f, e := a.OpenVerified(p)
	if e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(f)
	f.Close()
	if e != nil || !bytes.Equal(b, data) {
		t.Fatal(e, b)
	}
	p2 := p
	p2.SHA256 = strings.Repeat("0", 64)
	if e = a.Check(ctx, p2); e == nil {
		t.Fatal("substitution")
	}
	path, _ := a.path(p)
	if e = os.WriteFile(path, bytes.Repeat([]byte{'x'}, len(data)), 0600); e != nil {
		t.Fatal(e)
	}
	if e = a.Prepare(ctx, p); e == nil || n != 1 {
		t.Fatal("corrupt asset reused or overwritten", e, n)
	}
}
func TestStageFailures(t *testing.T) {
	data := []byte("good")
	for _, v := range []struct {
		name string
		body []byte
		code int
	}{{"wrong SHA", []byte("evil"), 200}, {"oversize", []byte("good!"), 200}, {"short", []byte("go"), 200}, {"HTTP failure", data, 404}} {
		t.Run(v.name, func(t *testing.T) {
			a, p := stagedFixture(t, data)
			n := 0
			setBody(t, a, p, v.body, v.code, &n)
			if e := a.Prepare(context.Background(), p); e == nil {
				t.Fatal("accepted bad asset")
			}
			path, _ := a.path(p)
			if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("published bad asset", e)
			}
		})
	}
}
func TestStageLinks(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "mode"} {
		t.Run(kind, func(t *testing.T) {
			a, p := stagedFixture(t, []byte("good"))
			path, _ := a.path(p)
			other := filepath.Join(a.store.dir, "other")
			if e := os.WriteFile(other, []byte("good"), 0600); e != nil {
				t.Fatal(e)
			}
			var e error
			switch kind {
			case "symlink":
				e = os.Symlink(other, path)
			case "hardlink":
				e = os.Link(other, path)
			case "mode":
				e = os.WriteFile(path, []byte("good"), 0644)
			}
			if e != nil {
				t.Fatal(e)
			}
			if e = a.Check(context.Background(), p); e == nil {
				t.Fatal("accepted unsafe file")
			}
		})
	}
}
func TestMutationLease(t *testing.T) {
	d := store(t).dir
	f, e := AcquireMutationLease(d)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = AcquireMutationLease(d); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	f.Close()
	f, e = AcquireMutationLease(d)
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	path := filepath.Join(d, mutationLockName)
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink("/dev/null", path); e != nil {
		t.Fatal(e)
	}
	if _, e = AcquireMutationLease(d); e == nil {
		t.Fatal("accepted symlink")
	}
}

// A real process, not a goroutine simulation. Launcher exits while its
// detached worker still holds the inherited lease and waits on a test gate.
func TestWorkerProcess(t *testing.T) {
	if len(os.Args) < 5 || os.Args[len(os.Args)-4] != "RF_WORKER_TEST" {
		return
	}
	mode, dir, lockDir := os.Args[len(os.Args)-3], os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "launcher" {
		exe, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		_, e = StartDetached(dir, lockDir, exe, []string{"-test.run=^TestWorkerProcess$", "--", "RF_WORKER_TEST", "worker", dir, lockDir}, fixture())
		if e != nil {
			t.Fatal(e)
		}
		return
	}
	if mode != "worker" {
		t.Fatal(mode)
	}
	lease, e := AdoptMutationLease(lockDir, MutationLeaseFD)
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	p, e := ReadWorkerRequest(dir)
	if e != nil || !reflect.DeepEqual(p, fixture()) {
		t.Fatal(e, p)
	}
	if e = os.WriteFile(filepath.Join(dir, "worker-ready"), []byte("ready"), 0600); e != nil {
		t.Fatal(e)
	}
	end := time.Now().Add(12 * time.Second)
	for {
		if _, e = os.Stat(filepath.Join(dir, "test-release")); e == nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("worker gate timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	j, e := s.Execute(context.Background(), p, &fake{})
	if e != nil || j.Phase != Completed {
		t.Fatal(j, e)
	}
}
func until(t *testing.T, fn func() bool) {
	t.Helper()
	end := time.Now().Add(8 * time.Second)
	for !fn() {
		if time.Now().After(end) {
			t.Fatal("condition timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func TestDetachedWorkerHandoff(t *testing.T) {
	s := store(t)
	d := store(t).dir
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(exe, "-test.run=^TestWorkerProcess$", "--", "RF_WORKER_TEST", "launcher", s.dir, d)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	defer os.WriteFile(filepath.Join(s.dir, "test-release"), nil, 0600)
	until(t, func() bool { _, e := os.Stat(filepath.Join(s.dir, "worker-ready")); return e == nil })
	if _, e = AcquireMutationLease(d); !errors.Is(e, ErrBusy) {
		t.Fatal("worker lost inherited lease", e)
	}
	if _, e = StartDetached(s.dir, d, exe, nil, fixture()); !errors.Is(e, ErrBusy) {
		t.Fatal("duplicate launch not rejected", e)
	}
	if e = os.WriteFile(filepath.Join(s.dir, "test-release"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	until(t, func() bool {
		f, e := AcquireMutationLease(d)
		if e != nil {
			return false
		}
		f.Close()
		return true
	})
	j, e := s.Load()
	if e != nil || j.Phase != Completed {
		b, _ := os.ReadFile(filepath.Join(s.dir, "worker.log"))
		t.Fatal(j, e, string(b))
	}
	if _, e = StartDetached(s.dir, d, exe, nil, fixture()); !errors.Is(e, ErrRecovery) {
		t.Fatal("replayed completed worker", e)
	}
}
func TestWorkerSpawnFailureNoReplay(t *testing.T) {
	s := store(t)
	d := store(t).dir
	missing := filepath.Join(s.dir, "no-executable")
	if _, e := StartDetached(s.dir, d, missing, nil, fixture()); e == nil {
		t.Fatal("spawn should fail")
	}
	p, e := ReadWorkerRequest(s.dir)
	if e != nil || !reflect.DeepEqual(p, fixture()) {
		t.Fatal(e, p)
	}
	if _, e = StartDetached(s.dir, d, missing, nil, fixture()); !errors.Is(e, os.ErrExist) {
		t.Fatal("spawn failure replayed", e)
	}
	path := filepath.Join(s.dir, "request.json")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	b = bytes.Replace(b, []byte("1458"), []byte("1459"), -1)
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWorkerRequest(s.dir); e == nil {
		t.Fatal("accepted modified request")
	}
}
