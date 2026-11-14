package lite

// P34 is in-memory, read-only DNS transport telemetry.  It neither probes
// external hosts nor changes the route chosen by the policy engine.
import (
	"sort"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

const p34BucketSeconds int64 = 15 * 60
const p34BucketCount = 96 // at most the current and 95 preceding 15-minute buckets

type p34Bucket struct {
	epoch          int64
	requests       uint64
	errors         uint64
	alternate      uint64
	totalLatencyMS uint64
	successes      uint64
}

type P34BucketView struct {
	Start     string  `json:"start"`
	Requests  uint64  `json:"requests"`
	Errors    uint64  `json:"errors"`
	Alternate uint64  `json:"alternate_choices"`
	AvgRTTMS  float64 `json:"avg_rtt_ms"`
}

type P34WindowView struct {
	Requests  uint64  `json:"requests"`
	Errors    uint64  `json:"errors"`
	Alternate uint64  `json:"alternate_choices"`
	AvgRTTMS  float64 `json:"avg_rtt_ms"`
}

type P34UpstreamView struct {
	ID              string          `json:"id"`
	Transport       string          `json:"transport"`
	Address         string          `json:"address"`
	LastFourBuckets P34WindowView   `json:"last_four_buckets"`
	Last96Buckets   P34WindowView   `json:"last_96_buckets"`
	History         []P34BucketView `json:"history"`
}

func p34Record(entry *upstreamHealthEntry, t Trace, at, now time.Time) {
	if entry == nil || at.After(now.Add(time.Minute)) || at.Before(now.Add(-24*time.Hour)) {
		return
	}
	epoch := at.Unix() / p34BucketSeconds
	slot := int(epoch % p34BucketCount)
	b := &entry.p34History[slot]
	if b.epoch != epoch || b.requests == 0 {
		*b = p34Bucket{epoch: epoch}
	}
	b.requests++
	if !t.Success {
		b.errors++
	} else {
		b.successes++
		if t.LatencyMS > 0 {
			b.totalLatencyMS += uint64(t.LatencyMS)
		}
	}
	// "Alternate" is deliberately NOT called "failover": load balancing
	// and parallel selection can choose a different upstream without failure.
	if t.Assigned != "" && t.Used != "" && t.Assigned != t.Used {
		b.alternate++
	}
}

func p34AddWindow(w *P34WindowView, b p34Bucket, successes *uint64, latency *uint64) {
	w.Requests += b.requests
	w.Errors += b.errors
	w.Alternate += b.alternate
	*successes += b.successes
	*latency += b.totalLatencyMS
}

func (h *UpstreamHealthTracker) DiagnosticsP34(upstreams []dr.Upstream, now time.Time) []P34UpstreamView {
	out := make([]P34UpstreamView, 0, len(upstreams))
	if h == nil {
		return out
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	current := now.Unix() / p34BucketSeconds
	for _, u := range upstreams {
		v := P34UpstreamView{ID: u.ID, Transport: u.Transport, Address: u.Address, History: []P34BucketView{}}
		entry := h.entries[u.ID]
		if entry != nil {
			var success4, success96, latency4, latency96 uint64
			for _, b := range entry.p34History {
				age := current - b.epoch
				if b.requests == 0 || age < 0 || age >= p34BucketCount {
					continue
				}
				p34AddWindow(&v.Last96Buckets, b, &success96, &latency96)
				if age < 4 {
					p34AddWindow(&v.LastFourBuckets, b, &success4, &latency4)
				}
				average := float64(0)
				if b.successes > 0 {
					average = float64(b.totalLatencyMS) / float64(b.successes)
				}
				v.History = append(v.History, P34BucketView{
					Start:    time.Unix(b.epoch*p34BucketSeconds, 0).UTC().Format(time.RFC3339),
					Requests: b.requests, Errors: b.errors, Alternate: b.alternate, AvgRTTMS: average,
				})
			}
			if success4 > 0 {
				v.LastFourBuckets.AvgRTTMS = float64(latency4) / float64(success4)
			}
			if success96 > 0 {
				v.Last96Buckets.AvgRTTMS = float64(latency96) / float64(success96)
			}
			sort.Slice(v.History, func(i, j int) bool { return v.History[i].Start < v.History[j].Start })
		}
		out = append(out, v)
	}
	return out
}
