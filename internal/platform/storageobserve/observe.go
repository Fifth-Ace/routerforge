package storageobserve

import (
	"bufio"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Linux diskstats sectors are always 512 bytes, irrespective of hardware block size.
type Counters struct{ Reads, ReadSectors, ReadMS, Writes, WriteSectors, WriteMS, BusyMS uint64 }

func Parse(r io.Reader) map[string]Counters {
	out := map[string]Counters{}
	s := bufio.NewScanner(io.LimitReader(r, 256<<10))
	s.Buffer(make([]byte, 4096), 8192)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 14 {
			continue
		}
		v := make([]uint64, 7)
		ok := true
		for i, j := range []int{3, 5, 6, 7, 9, 10, 12} {
			n, e := strconv.ParseUint(f[j], 10, 64)
			if e != nil {
				ok = false
				break
			}
			v[i] = n
		}
		if ok {
			out[f[2]] = Counters{v[0], v[1], v[2], v[3], v[4], v[5], v[6]}
		}
	}
	if s.Err() != nil {
		return nil
	}
	return out
}

type IO struct {
	ReadBPS       float64  `json:"read_bps"`
	WriteBPS      float64  `json:"write_bps"`
	ReadOPS       float64  `json:"read_ops"`
	WriteOPS      float64  `json:"write_ops"`
	ReadAwaitMS   *float64 `json:"read_await_ms"`
	WriteAwaitMS  *float64 `json:"write_await_ms"`
	BusyPct       float64  `json:"busy_pct"`
	WindowSeconds float64  `json:"window_seconds"`
}

func Delta(a, b Counters, elapsed time.Duration) *IO {
	if elapsed < time.Second || elapsed > 5*time.Minute {
		return nil
	}
	if b.Reads < a.Reads || b.ReadSectors < a.ReadSectors || b.ReadMS < a.ReadMS || b.Writes < a.Writes || b.WriteSectors < a.WriteSectors || b.WriteMS < a.WriteMS || b.BusyMS < a.BusyMS {
		return nil
	}
	t := elapsed.Seconds()
	rd, wr := b.Reads-a.Reads, b.Writes-a.Writes
	x := &IO{ReadBPS: float64(b.ReadSectors-a.ReadSectors) * 512 / t, WriteBPS: float64(b.WriteSectors-a.WriteSectors) * 512 / t, ReadOPS: float64(rd) / t, WriteOPS: float64(wr) / t, BusyPct: math.Min(100, float64(b.BusyMS-a.BusyMS)/(10*t)), WindowSeconds: t}
	if rd > 0 {
		v := float64(b.ReadMS-a.ReadMS) / float64(rd)
		x.ReadAwaitMS = &v
	}
	if wr > 0 {
		v := float64(b.WriteMS-a.WriteMS) / float64(wr)
		x.WriteAwaitMS = &v
	}
	return x
}

type Mount struct {
	Key              string
	Total, Available uint64
}
type Point struct {
	At        time.Time `json:"at"`
	Available uint64    `json:"available_bytes"`
}
type Capacity struct {
	Points            []Point  `json:"points"`
	NetConsumptionBPS *float64 `json:"net_consumption_bps"`
	EstimatedDays     *float64 `json:"estimated_days"`
}
type mountHistory struct {
	total  uint64
	points []Point
}
type diskState struct {
	id       string
	at       time.Time
	counters *Counters
	mounts   map[string]mountHistory
}
type Observation struct {
	IO             *IO                 `json:"io"`
	Capacity       map[string]Capacity `json:"capacity"`
	IdentityStable bool                `json:"identity_stable"`
}
type Tracker struct {
	mu     sync.Mutex
	states map[string]diskState
}

// Retain only devices in the current observation, never join unknown identities.
func (t *Tracker) Observe(now time.Time, devices map[string]string, stats map[string]Counters, mounts map[string][]Mount) map[string]Observation {
	t.mu.Lock()
	defer t.mu.Unlock()
	next := map[string]diskState{}
	out := map[string]Observation{}
	ids := map[string]int{}
	for _, id := range devices {
		ids[id]++
	}
	keys := make([]string, 0, len(devices))
	for dev := range devices {
		keys = append(keys, dev)
	}
	sort.Strings(keys)
	for _, dev := range keys {
		id := devices[dev]
		if len(next) >= 8 {
			break
		}
		stable := strings.HasPrefix(id, "hw:") && len(id) > 3 && ids[id] == 1
		o := Observation{Capacity: map[string]Capacity{}, IdentityStable: stable}
		old := t.states[dev]
		if !stable || old.id != id || now.Before(old.at) {
			old = diskState{}
		}
		s := diskState{id: id, at: now, mounts: map[string]mountHistory{}}
		if c, ok := stats[dev]; ok && stable {
			s.counters = &c
			if old.counters != nil {
				if now.Sub(old.at) < time.Second {
					s.at = old.at
					s.counters = old.counters
				} else {
					o.IO = Delta(*old.counters, c, now.Sub(old.at))
				}
			}
		}
		if stable {
			for _, m := range mounts[dev] {
				if len(s.mounts) >= 4 {
					break
				}
				if m.Total == 0 || m.Available > m.Total {
					continue
				}
				h := old.mounts[m.Key]
				if h.total != m.Total {
					h = mountHistory{total: m.Total}
				}
				p := []Point{}
				for _, v := range h.points {
					if !v.At.Before(now.Add(-24*time.Hour)) && !v.At.After(now) {
						p = append(p, v)
					}
				}
				if len(p) == 0 || now.Sub(p[len(p)-1].At) >= 15*time.Minute {
					p = append(p, Point{now, m.Available})
				}
				if len(p) > 96 {
					p = p[len(p)-96:]
				}
				h.points = p
				s.mounts[m.Key] = h
				cap := Capacity{Points: append([]Point{}, p...)}
				if len(p) >= 4 {
					first, last := p[0], p[len(p)-1]
					span := last.At.Sub(first.At)
					if span >= 30*time.Minute && first.Available > last.Available {
						rate := float64(first.Available-last.Available) / span.Seconds()
						days := float64(m.Available) / rate / 86400
						cap.NetConsumptionBPS = &rate
						if days <= 365 {
							cap.EstimatedDays = &days
						}
					}
				}
				o.Capacity[m.Key] = cap
			}
		}
		next[dev] = s
		out[dev] = o
	}
	t.states = next
	return out
}

// Counts only. No disk IDs, mount paths or I/O data leave the storage observer.
func (t *Tracker) Summary() map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.states == nil {
		return nil
	}
	stable := 0
	for _, s := range t.states {
		if s.counters != nil {
			stable++
		}
	}
	return map[string]int{"tracked_disks": len(t.states), "counter_baselines": stable}
}
