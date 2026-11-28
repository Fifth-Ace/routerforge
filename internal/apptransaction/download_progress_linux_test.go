//go:build linux

package apptransaction

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelDownloadsBounded(t *testing.T) {
	var active, peak int32
	gate := make(chan struct{})
	entered := make(chan struct{}, 6)
	result := make(chan error, 1)
	packages := make([]Package, 6)
	go func() {
		n, err := prepareParallel(context.Background(), packages, func(ctx context.Context, p Package) error {
			v := atomic.AddInt32(&active, 1)
			defer atomic.AddInt32(&active, -1)
			for {
				old := atomic.LoadInt32(&peak)
				if old >= v || atomic.CompareAndSwapInt32(&peak, old, v) {
					break
				}
			}
			entered <- struct{}{}
			select {
			case <-gate:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err == nil && n != 6 {
			err = errors.New("wrong completion count")
		}
		result <- err
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			close(gate)
			t.Fatal("downloads did not overlap")
		}
	}
	close(gate)
	if err := <-result; err != nil || peak != 3 {
		t.Fatalf("peak %d: %v", peak, err)
	}
}
func TestParallelDownloadsFailure(t *testing.T) {
	n, err := prepareParallel(context.Background(), []Package{{Name: "bad"}}, func(context.Context, Package) error { return errors.New("network") })
	if n != 0 || err == nil {
		t.Fatal(n, err)
	}
}
func TestDownloadWriterActualBytes(t *testing.T) {
	var buf bytes.Buffer
	var latest DownloadProgress
	w := &downloadWriter{w: &buf, p: Package{Size: 4}, started: time.Now(), source: "GitHub", report: func(_ Package, v DownloadProgress) { latest = v }}
	if _, err := w.Write([]byte("abcd")); err != nil || latest.Bytes != 4 || latest.Source != "GitHub" || buf.String() != "abcd" {
		t.Fatal(latest, err)
	}
}
func TestReadOnlyOPKGStatusRetriesBusy(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "opkg")
	count := filepath.Join(dir, "count")
	body := "#!/bin/sh\n[ \"$1\" = status ] || exit 7\nif [ ! -e '" + count + "' ]; then touch '" + count + "'; echo 'Could not lock /opt/tmp/opkg.lock: Resource temporarily unavailable.' >&2; exit 255; fi\nprintf 'Package: routerforge-profiling\\nVersion: 1~dev.r1474.28e2140501bb\\nStatus: install user installed\\nArchitecture: aarch64-3.10\\n'\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	got, err := readInstalledDevVersion(context.Background(), script, "routerforge-profiling")
	if err != nil || got != "1~dev.r1474.28e2140501bb" {
		t.Fatal(got, err)
	}
}
