package serviceobserve

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func procLine(pid int, state string, u, s, start uint64) string {
	f := make([]string, 21)
	for i := range f {
		f[i] = "0"
	}
	f[0] = state
	f[11] = fmt.Sprint(u)
	f[12] = fmt.Sprint(s)
	f[19] = fmt.Sprint(start)
	return fmt.Sprintf("%d (name ) odd) %s", pid, strings.Join(f, " "))
}
func TestProcAndTotal(t *testing.T) {
	p, ok := ParseProc(procLine(12, "S", 8, 3, 7), 12)
	if !ok || p.Ticks != 11 || p.Start != 7 {
		t.Fatal(p, ok)
	}
	if _, ok := ParseProc(procLine(12, "S", 8, 3, 7), 13); ok {
		t.Fatal("wrong pid")
	}
	if v, ok := ParseTotal("cpu 1 2 3 4 5 6 7 8 100 200\ncpu0 1"); !ok || v != 36 {
		t.Fatal(v, ok)
	}
	if _, ok := ParseTotal("cpu 1 x 2 3"); ok {
		t.Fatal("invalid total")
	}
}
func TestCPUIdentityAndWindow(t *testing.T) {
	var s Sampler
	now := time.Now()
	a := Sample{now, 100, map[int]Proc{1: {4, 10, "S"}, 2: {9, 11, "S"}}}
	if d := s.Observe(a); d.Percent([]int{1}) != nil {
		t.Fatal("first sample")
	}
	b := Sample{now.Add(2 * time.Second), 200, map[int]Proc{1: {4, 30, "S"}, 2: {10, 50, "S"}}}
	d := s.Observe(b)
	if v := d.Percent([]int{1, 1}); v == nil || *v != 20 {
		t.Fatal(v)
	}
	if d.Percent([]int{1, 2}) != nil {
		t.Fatal("reused pid")
	}
	if d.Percent([]int{9}) != nil {
		t.Fatal("missing pid")
	}
	if s.Observe(Sample{now.Add(2500 * time.Millisecond), 205, b.Procs}).Percent([]int{1}) != nil {
		t.Fatal("short window")
	}
	if s.Observe(Sample{now.Add(6 * time.Minute), 300, b.Procs}).Percent([]int{1}) != nil {
		t.Fatal("stale")
	}
}
func TestPresence(t *testing.T) {
	p := map[int]Proc{1: {1, 1, "Z"}, 2: {1, 1, "D"}, 3: {1, 1, "S"}}
	for _, x := range []struct {
		ids  []int
		want string
	}{{nil, "not_detected"}, {[]int{1}, "zombie_only"}, {[]int{1, 2}, "blocked_observed"}, {[]int{1, 3}, "observed"}, {[]int{3, 9}, "unknown"}} {
		if got := Presence(x.ids, p); got != x.want {
			t.Fatal(got, x.want)
		}
	}
}
func TestDeclaredDependencies(t *testing.T) {
	raw := []byte("Package: app\nStatus: install ok installed\nDepends: liba (>= 2), libb | libc\nPre-Depends: core\n\nPackage: liba\nStatus: install ok installed\n\nPackage: libc\nStatus: deinstall ok config-files\n")
	d := Dependencies(raw)["app"]
	if len(d) != 3 || d[0].Kind != "pre-depends" || len(d[1].InstalledCandidates) != 1 || d[1].InstalledCandidates[0] != "liba" || len(d[2].InstalledCandidates) != 0 {
		t.Fatal(d)
	}
	if len(Dependencies(append(raw, []byte("\nPackage: app\nStatus: install ok installed\n")...))) != 0 {
		t.Fatal("duplicate package")
	}
}
func TestBootEvidence(t *testing.T) {
	raw := []byte("fixture\n")
	v := BootTemplate{"dns", BlobHash(raw), true, true}
	yes, no := true, false
	for _, x := range []struct {
		raw  []byte
		ex   bool
		a, b *bool
		want string
	}{{raw, true, &yes, &no, "eligible"}, {raw, true, &no, &no, "abi_missing"}, {raw, true, &yes, &yes, "disabled"}, {raw, true, nil, &no, "unknown"}, {raw, false, &yes, &no, "not_executable"}, {[]byte("changed"), true, &yes, &no, "unknown"}} {
		if got := Boot(v, x.raw, x.ex, x.a, x.b); got != x.want {
			t.Fatal(got, x.want)
		}
	}
	if Module("S98routerforge-antiscan-manager") != "" || Module("S95routerforge-dns-router") != "dns-router" {
		t.Fatal("module mapping")
	}
}
