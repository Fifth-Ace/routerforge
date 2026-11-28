//go:build linux

package apptransaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type finalizationRoundTripper func(*http.Request) (*http.Response, error)

func (f finalizationRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadFinalByteIsNotCompletion(t *testing.T) {
	var got DownloadProgress
	w := &downloadWriter{w: io.Discard, p: Package{Size: 4}, started: time.Now().Add(-time.Second), report: func(_ Package, v DownloadProgress) { got = v }}
	if _, err := w.Write([]byte("abcd")); err != nil || got.State != "finalizing" || got.Bytes != 4 || got.Rate != 0 || w.lastByte.IsZero() {
		t.Fatal(got, err)
	}
}

func TestTransientStagingFinalizationAndIntegrity(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data := []byte("verified package bytes")
			sha := strings.Repeat("a", 40)
			v := "1~dev.r2." + sha[:12]
			pkg := Package{Name: "routerforge-admin", Version: v, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Asset: "routerforge-admin_" + strings.ReplaceAll(v, "~", "-") + "_aarch64-3.10.ipk", Size: int64(len(data))}
			p := Plan{ID: "test", Channel: "dev", Target: "aarch64-3.10", Commit: sha, Packages: []Package{pkg}}
			a, err := NewStaging(dir, p)
			if err != nil {
				t.Fatal(err)
			}
			var states []string
			a.report = func(_ Package, v DownloadProgress) { states = append(states, v.State) }
			body := append([]byte(nil), data...)
			if corrupt {
				body[0] ^= 1
			}
			a.client = &http.Client{Transport: finalizationRoundTripper(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header), Request: r}, nil
			})}
			err = a.Prepare(context.Background(), pkg)
			if corrupt {
				if err == nil {
					t.Fatal("bad hash accepted")
				}
				if _, err = os.Stat(filepath.Join(dir, pkg.Asset)); !os.IsNotExist(err) {
					t.Fatal("corrupt file published", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(states, ",") != "downloading,finalizing,checking,staging" {
				t.Fatal(states)
			}
			f, err := a.OpenVerified(pkg)
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			if err = os.WriteFile(filepath.Join(dir, pkg.Asset), body[:len(body)-1], 0600); err != nil {
				t.Fatal(err)
			}
			if f, err = a.OpenVerified(pkg); err == nil {
				f.Close()
				t.Fatal("truncated cache accepted")
			}
		})
	}
}
