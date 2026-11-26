// Package serviceobserve projects bounded, read-only service evidence. A process
// match or a package dependency is not a runtime health or ordering guarantee.
package serviceobserve

import (
	"crypto/sha1" // Git blob identity for exact known init-script templates only.
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxPIDs = 256

type Proc struct {
	Start, Ticks uint64
	State        string
}

func ParseProc(raw string, pid int) (Proc, bool) {
	var p Proc
	if len(raw) > 8192 || pid <= 0 {
		return p, false
	}
	i, j := strings.Index(raw, " ("), strings.LastIndex(raw, ") ")
	if i < 1 || j <= i {
		return p, false
	}
	n, e := strconv.Atoi(raw[:i])
	if e != nil || n != pid {
		return p, false
	}
	f := strings.Fields(raw[j+2:])
	if len(f) < 20 || len(f[0]) != 1 || !strings.Contains("RSDZTWtXxKWPIN", f[0]) {
		return p, false
	}
	u, e := strconv.ParseUint(f[11], 10, 64)
	if e != nil {
		return p, false
	}
	s, e := strconv.ParseUint(f[12], 10, 64)
	if e != nil || u > math.MaxUint64-s {
		return p, false
	}
	start, e := strconv.ParseUint(f[19], 10, 64)
	if e != nil || start == 0 {
		return p, false
	}
	return Proc{start, u + s, f[0]}, true
}
func ParseTotal(raw string) (uint64, bool) {
	f := strings.Fields(strings.SplitN(raw, "\n", 2)[0])
	if len(f) < 5 || f[0] != "cpu" {
		return 0, false
	}
	var n uint64
	// Guest counters are already included in user/nice. Use the first 8 fields.
	for _, v := range f[1:min(len(f), 9)] {
		x, e := strconv.ParseUint(v, 10, 64)
		if e != nil || x > math.MaxUint64-n {
			return 0, false
		}
		n += x
	}
	return n, n > 0
}

type Sample struct {
	At    time.Time
	Total uint64
	Procs map[int]Proc
}
type Delta struct {
	WindowMS int64
	total    uint64
	ticks    map[int]uint64
}
type Sampler struct {
	mu    sync.Mutex
	prior Sample
}

// A baseline exists only within the current Admin process. No background work.
// A new/reused PID, missing counter or discontinuity returns unknown, not zero.
func (s *Sampler) Observe(now Sample) Delta {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.Total == 0 || now.At.IsZero() || len(now.Procs) > MaxPIDs {
		return Delta{}
	}
	if !s.prior.At.IsZero() && !now.At.After(s.prior.At) {
		return Delta{}
	}
	old := s.prior
	if !old.At.IsZero() && now.At.Sub(old.At) < time.Second {
		return Delta{}
	}
	s.prior = now
	age := now.At.Sub(old.At)
	if old.At.IsZero() || age > 5*time.Minute || now.Total <= old.Total {
		return Delta{}
	}
	d := Delta{WindowMS: age.Milliseconds(), total: now.Total - old.Total, ticks: map[int]uint64{}}
	for id, p := range now.Procs {
		if q, ok := old.Procs[id]; ok && q.Start == p.Start && p.Ticks >= q.Ticks {
			d.ticks[id] = p.Ticks - q.Ticks
		}
	}
	return d
}
func (d Delta) Percent(ids []int) *float64 {
	if len(ids) == 0 || d.total == 0 {
		return nil
	}
	seen := map[int]bool{}
	var n uint64
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		x, ok := d.ticks[id]
		if !ok || x > d.total-n {
			return nil
		}
		n += x
	}
	v := 100 * float64(n) / float64(d.total)
	return &v
}
func Presence(ids []int, p map[int]Proc) string {
	if len(ids) == 0 {
		return "not_detected"
	}
	live, blocked := false, false
	for _, id := range ids {
		v, ok := p[id]
		if !ok {
			return "unknown"
		}
		switch v.State {
		case "Z", "X", "x":
		default:
			live = true
			blocked = blocked || v.State == "D"
		}
	}
	if !live {
		return "zombie_only"
	}
	if blocked {
		return "blocked_observed"
	}
	return "observed"
}

type Dependency struct {
	Kind                string   `json:"kind"`
	Clause              string   `json:"clause"`
	InstalledCandidates []string `json:"installed_candidates"`
}
type packageRow struct{ depends, pre, status string }

var packageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.-]{0,127}$`)
var clauseRE = regexp.MustCompile(`^[A-Za-z0-9+_.:~<>=()|\- \t]+$`)

// Parse only the bounded opkg status snapshot. Alternatives and constraints are
// preserved. Candidate presence is not a version check or a Provides resolver.
func Dependencies(raw []byte) map[string][]Dependency {
	out := map[string][]Dependency{}
	if len(raw) > 2<<20 {
		return out
	}
	rows := map[string]packageRow{}
	for _, block := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n\n") {
		name := ""
		row := packageRow{}
		duplicate := false
		seen := map[string]bool{}
		previous := ""
		for _, l := range strings.Split(block, "\n") {
			if strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") {
				if previous == "Depends" || previous == "Pre-Depends" || previous == "Package" || previous == "Status" {
					duplicate = true
				}
				continue
			}
			k, v, ok := strings.Cut(l, ":")
			previous = k
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch k {
			case "Package", "Status", "Depends", "Pre-Depends":
				if seen[k] {
					duplicate = true
				}
				seen[k] = true
			default:
				continue
			}
			switch k {
			case "Package":
				name = v
			case "Status":
				row.status = v
			case "Depends":
				row.depends = v
			case "Pre-Depends":
				row.pre = v
			}
		}
		if duplicate || !packageName.MatchString(name) {
			continue
		}
		if _, exists := rows[name]; exists {
			return map[string][]Dependency{}
		}
		rows[name] = row
		if len(rows) > 4096 {
			return map[string][]Dependency{}
		}
	}
	installed := map[string]bool{}
	for n, r := range rows {
		f := strings.Fields(r.status)
		if len(f) == 3 && f[2] == "installed" {
			installed[n] = true
		}
	}
	for name, row := range rows {
		if !installed[name] {
			continue
		}
		deps := []Dependency{}
		valid := true
		for _, field := range []struct{ k, v string }{{"pre-depends", row.pre}, {"depends", row.depends}} {
			if field.v == "" {
				continue
			}
			for _, s := range strings.Split(field.v, ",") {
				s = strings.TrimSpace(s)
				if s == "" || len(s) > 256 || !clauseRE.MatchString(s) || len(deps) >= 64 {
					valid = false
					break
				}
				d := Dependency{Kind: field.k, Clause: s, InstalledCandidates: []string{}}
				for _, alt := range strings.Split(s, "|") {
					f := strings.Fields(strings.TrimSpace(alt))
					if len(f) > 0 && packageName.MatchString(f[0]) && installed[f[0]] {
						d.InstalledCandidates = append(d.InstalledCandidates, f[0])
					}
				}
				deps = append(deps, d)
			}
			if !valid {
				break
			}
		}
		if valid {
			out[name] = deps
		}
	}
	return out
}

type BootTemplate struct {
	Module, Hash       string
	ABI, DisableMarker bool
}

var templates = map[string]BootTemplate{
	"S90routerforge":               {"core", "1194f543518079d619bca80f82cbeac5b59f84d6", false, false},
	"S91routerforge-admin":         {"admin", "d950b9d66fc27660752b1e90cf77d2e92474da5b", false, true},
	"S91routerforge-dns":           {"dns", "f4c6b0bfe25e465153eef3c909a8110999e9d09f", true, true},
	"S92routerforge-monitoring":    {"monitoring", "c0ab7a0d2f40d8c8f3d391c42ddc8faa5da5b983", true, true},
	"S97routerforge-network-tools": {"network-tools", "645d1eebef1430ebfd99c5fd81c07747b26716e7", true, true},
	"S96routerforge-nfqws-manager": {"nfqws-manager", "145298d0c506253d298211dd9b265db24e878393", true, true},
}

func KnownTemplate(id string) (BootTemplate, bool) { v, ok := templates[id]; return v, ok }
func Module(id string) string {
	if t, ok := KnownTemplate(id); ok {
		return t.Module
	}
	if id == "S95routerforge-dns-router" {
		return "dns-router"
	}
	return ""
}
func BlobHash(raw []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(raw))
	_, _ = h.Write(raw)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Only known byte-identical rc.func templates get an enabled/disabled result.
// The result describes configured boot eligibility, not a successful boot.
func Boot(t BootTemplate, raw []byte, executable bool, abi, disabled *bool) string {
	if t.Hash == "" || BlobHash(raw) != t.Hash {
		return "unknown"
	}
	if !executable {
		return "not_executable"
	}
	if t.DisableMarker {
		if disabled == nil {
			return "unknown"
		}
		if *disabled {
			return "disabled"
		}
	}
	if t.ABI {
		if abi == nil {
			return "unknown"
		}
		if !*abi {
			return "abi_missing"
		}
	}
	return "eligible"
}
