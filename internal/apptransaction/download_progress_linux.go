//go:build linux

package apptransaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Advisory progress is separate from the durable journal. No fsync per chunk.
// Exact downloaded bytes are reported, not an estimate of OPKG's internal work.
type DownloadProgress struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Bytes  int64  `json:"bytes"`
	Source string `json:"source,omitempty"`
	Rate   int64  `json:"rate,omitempty"`
}

type downloadWriter struct {
	w                       io.Writer
	p                       Package
	report                  func(Package, DownloadProgress)
	source                  string
	started, last, lastByte time.Time
	n                       int64
}

func (w *downloadWriter) emit(state string) {
	rate := int64(0)
	if elapsed := time.Since(w.started).Seconds(); state == "downloading" && elapsed > 0.1 {
		rate = int64(float64(w.n) / elapsed)
	}
	w.report(w.p, DownloadProgress{State: state, Bytes: w.n, Source: w.source, Rate: rate})
	w.last = time.Now()
}
func (w *downloadWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	if err == nil && w.n == w.p.Size {
		w.lastByte = time.Now()
		w.emit("finalizing")
	} else if time.Since(w.last) >= 250*time.Millisecond {
		w.emit("downloading")
	}
	return n, err
}
func writeDownloadProgress(dir, id string, p Package, v DownloadProgress) {
	v.ID = id
	v.Name = p.Name
	if v.Bytes < 0 || v.Bytes > p.Size {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	path := filepath.Join(dir, "progress-"+p.Name+".json")
	// Each download has exactly one writer. Atomic replacement keeps polling cheap.
	temp, err := os.CreateTemp(dir, ".progress-")
	if err != nil {
		return
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(b); err != nil {
		temp.Close()
		return
	}
	if err = temp.Close(); err != nil {
		return
	}
	_ = os.Rename(temp.Name(), path)
}
func (c *Coordinator) UpdateDownloads(p *Plan) []DownloadProgress {
	out := []DownloadProgress{}
	if p == nil {
		return out
	}
	if _, err := p.Hash(); err != nil {
		return out
	}
	for _, pkg := range p.Packages {
		path := filepath.Join(c.slot(), "progress-"+pkg.Name+".json")
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, 4096))
		f.Close()
		if err != nil {
			continue
		}
		var v DownloadProgress
		if json.Unmarshal(b, &v) == nil && v.ID == p.ID && v.Name == pkg.Name && v.Bytes >= 0 && v.Bytes <= pkg.Size {
			out = append(out, v)
		}
	}
	return out
}

// Bounded three-way download concurrency only. Wait for all writers to stop
// before returning, so the installation phase can never overlap downloads.
func prepareParallel(ctx context.Context, packages []Package, prepare func(context.Context, Package) error) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan Package)
	var wg sync.WaitGroup
	var mu sync.Mutex
	count := 0
	var first error
	for i := 0; i < 3 && i < len(packages); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if ctx.Err() != nil {
					return
				}
				err := prepare(ctx, p)
				mu.Lock()
				if err == nil {
					count++
				} else if first == nil {
					first = fmt.Errorf("download %s: %w", p.Name, err)
					cancel()
				}
				mu.Unlock()
			}
		}()
	}
send:
	for _, p := range packages {
		select {
		case <-ctx.Done():
			break send
		case jobs <- p:
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return count, first
	}
	return count, ctx.Err()
}
