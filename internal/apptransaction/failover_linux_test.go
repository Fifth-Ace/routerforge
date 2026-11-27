//go:build linux

package apptransaction

import (
	"bytes"
	"context"
	"errors"
	"github.com/Fifth-Ace/routerforge/internal/releasetransport"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func fixtureResponse(body []byte, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header)}
}

type brokenBody struct{ read bool }

func (b *brokenBody) Read(p []byte) (int, error) {
	if b.read {
		return 0, errors.New("connection dropped")
	}
	b.read = true
	return copy(p, []byte("partial")), nil
}
func (b *brokenBody) Close() error { return nil }
func TestStageMirrorFailover(t *testing.T) {
	for _, kind := range []string{"transport", "HTTP", "partial", "hash", "both_bad", "primary"} {
		t.Run(kind, func(t *testing.T) {
			data := []byte("correct exact bytes")
			a, p := stagedFixture(t, data)
			var seen []string
			a.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				seen = append(seen, r.URL.String())
				if len(seen) == 1 {
					switch kind {
					case "transport":
						return nil, errors.New("timeout")
					case "HTTP", "both_bad":
						return fixtureResponse(nil, 503), nil
					case "partial":
						return &http.Response{StatusCode: 200, Body: &brokenBody{}, ContentLength: int64(len(data)), Header: make(http.Header)}, nil
					case "hash":
						return fixtureResponse(bytes.Repeat([]byte{'x'}, len(data)), 200), nil
					}
				}
				if kind == "both_bad" {
					return fixtureResponse([]byte("bad"), 200), nil
				}
				return fixtureResponse(data, 200), nil
			})
			e := a.Prepare(context.Background(), p)
			path, _ := a.path(p)
			if kind == "both_bad" {
				if e == nil {
					t.Fatal("accepted invalid mirror")
				}
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				b, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(b, data) {
					t.Fatal(err, b)
				}
			}
			want := releasetransport.Candidates("routerforge-dev", p.Asset)
			if kind == "primary" {
				want = want[:1]
			}
			if !reflect.DeepEqual(seen, want) {
				t.Fatal(seen, want)
			}
		})
	}
}
func TestMirrorIndexExpectedCommit(t *testing.T) {
	idx := indexFixture()
	var seen []string
	c := releaseClient()
	c.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.URL.String())
		if r.URL.Host == "github.com" {
			return nil, errors.New("unavailable")
		}
		return fixtureResponse(encodeIndex(t, idx), 200), nil
	})
	if _, err := fetchDevPlan(context.Background(), c, "frozen", idx.Commit, []string{"routerforge-core"}, encodeIndex(t, idx)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, releasetransport.Candidates("routerforge-dev", "routerforge-dev-index.json")) {
		t.Fatal(seen)
	}
	other := strings.Repeat("c", 40)
	if other == idx.Commit {
		other = strings.Repeat("d", 40)
	}
	if _, err := fetchDevPlan(context.Background(), c, "frozen", other, []string{"routerforge-core"}, encodeIndex(t, idx)); err == nil {
		t.Fatal("stale mirror accepted")
	}
}

func TestMirrorIndexRequiresExactTrustedBytes(t *testing.T) {
	idx := indexFixture()
	trusted := encodeIndex(t, idx)
	idx.Components[0].SHA256 = strings.Repeat("f", 64)
	changed := encodeIndex(t, idx)
	c := releaseClient()
	c.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.com" {
			return nil, errors.New("unavailable")
		}
		return fixtureResponse(changed, 200), nil
	})
	for _, snapshot := range [][]byte{nil, trusted} {
		if _, err := fetchDevPlan(context.Background(), c, "frozen", idx.Commit, []string{"routerforge-core"}, snapshot); err == nil {
			t.Fatal("accepted uncorroborated mirror index")
		}
	}
}
