//go:build linux

package apptransaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const releaseBase = "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-dev/"
const legacyReleaseBase = "https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-dev/"
const indexLimit = 512 << 10

type releaseEntry struct {
	Package      string `json:"package"`
	Version      string `json:"version"`
	SHA256       string `json:"sha256"`
	Asset        string `json:"asset"`
	Size         int64  `json:"size_bytes"`
	Architecture string `json:"architecture"`
	URL          string `json:"url"`
	CanonicalURL string `json:"canonical_url"`
}
type releaseIndex struct {
	Schema     int            `json:"schema_version"`
	Channel    string         `json:"channel"`
	Target     string         `json:"target"`
	Commit     string         `json:"commit"`
	Components []releaseEntry `json:"components"`
}

// FreezeDevPlan accepts only an exact, trusted publisher snapshot. The caller
// must obtain it through the authenticated release transport, never from an
// HTTP request body. expectedCommit is the release commit, not the dev head.
func FreezeDevPlan(data []byte, id, expectedCommit string, selected []string) (Plan, error) {
	var empty Plan
	if len(data) > indexLimit || len(selected) == 0 || len(selected) > 11 {
		return empty, errors.New("invalid release selection")
	}
	var idx releaseIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return empty, err
	}
	if idx.Schema != 1 || idx.Channel != "dev" || idx.Target != "aarch64-3.10" || idx.Commit != expectedCommit || len(idx.Components) != 11 {
		return empty, errors.New("unexpected official release snapshot")
	}
	all := Plan{ID: id, Channel: idx.Channel, Target: idx.Target, Commit: idx.Commit}
	entries := map[string]Package{}
	for _, e := range idx.Components {
		if e.Architecture != idx.Target || e.CanonicalURL != releaseBase+e.Asset || (e.URL != legacyReleaseBase+e.Asset && e.URL != releaseBase+e.Asset) {
			return empty, errors.New("unexpected release provenance")
		}
		p := Package{e.Package, e.Version, e.SHA256, e.Asset, e.Size}
		if _, ok := entries[p.Name]; ok {
			return empty, errors.New("duplicate release package")
		}
		entries[p.Name] = p
		all.Packages = append(all.Packages, p)
	}
	orderPackages(all.Packages)
	if _, err := all.Hash(); err != nil {
		return empty, err
	}
	out := all
	out.Packages = nil
	for _, n := range selected {
		p, ok := entries[n]
		if !ok {
			return empty, errors.New("unknown selected package")
		}
		out.Packages = append(out.Packages, p)
	}
	orderPackages(out.Packages)
	if _, err := out.Hash(); err != nil {
		return empty, err
	}
	return out, nil
}
func orderPackages(p []Package) {
	rank := func(n string) int {
		switch n {
		case "routerforge-profiling":
			return 1
		case "routerforge-core":
			return 2
		}
		return 0
	}
	sort.Slice(p, func(i, j int) bool {
		a, b := rank(p[i].Name), rank(p[j].Name)
		if a != b {
			return a < b
		}
		return p[i].Name < p[j].Name
	})
}
func allowedReleaseRedirect(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	// GitHub's release asset CDN uses signed query strings. Only these exact
	// hosts may receive redirects; never accept arbitrary github.io or suffixes.
	switch u.Host {
	case "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}
func releaseClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !allowedReleaseRedirect(req.URL) {
			return errors.New("untrusted release redirect")
		}
		return nil
	}}
}
func fetchRelease(ctx context.Context, c *http.Client, asset string, limit int64, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseBase+asset, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "RouterForge-AppTransaction/1")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength > limit || resp.Uncompressed {
		return 0, errors.New("unexpected release HTTP response")
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, errors.New("release size limit exceeded")
	}
	return n, nil
}

// FetchDevPlan never accepts a user-supplied URL, and validates the entire
// official index before returning a selected immutable plan.
func FetchDevPlan(ctx context.Context, id, expectedCommit string, selected []string) (Plan, error) {
	var b boundedBytes
	if _, err := fetchRelease(ctx, releaseClient(), "routerforge-dev-index.json", indexLimit, &b); err != nil {
		return Plan{}, err
	}
	return FreezeDevPlan(b, id, expectedCommit, selected)
}

type boundedBytes []byte

func (b *boundedBytes) Write(p []byte) (int, error) { *b = append(*b, p...); return len(p), nil }

// Staging is per transaction, owned and private. It cannot install packages.
// The adapter retains the exact plan, rejects caller substitutions and hashes
// staged bytes again immediately before passing a verified descriptor onward.
type Staging struct {
	store    *Store
	packages map[string]Package
	client   *http.Client
}

func NewStaging(dir string, p Plan) (*Staging, error) {
	if _, err := p.Hash(); err != nil {
		return nil, err
	}
	s, err := Open(dir)
	if err != nil {
		return nil, err
	}
	a := &Staging{store: s, packages: map[string]Package{}, client: releaseClient()}
	for _, v := range p.Packages {
		a.packages[v.Name] = v
	}
	return a, nil
}
func (a *Staging) path(p Package) (string, error) {
	if v, ok := a.packages[p.Name]; !ok || v != p {
		return "", errors.New("package not in frozen plan")
	}
	return filepath.Join(a.store.dir, p.Asset), nil
}
func (a *Staging) OpenVerified(p Package) (*os.File, error) {
	path, err := a.path(p)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	st, err := f.Stat()
	if err != nil || !privateRegular(st) || st.Size() != p.Size {
		return nil, errors.New("invalid staged asset metadata")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, p.Size+1))
	if err != nil || n != p.Size || hex.EncodeToString(h.Sum(nil)) != p.SHA256 {
		return nil, errors.New("staged asset digest mismatch")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return f, nil
}
func (a *Staging) Check(ctx context.Context, p Package) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := a.OpenVerified(p)
	if err != nil {
		return err
	}
	return f.Close()
}
func (a *Staging) Prepare(ctx context.Context, p Package) error {
	path, err := a.path(p)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if _, err = os.Lstat(path); err == nil {
		return a.Check(ctx, p)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(a.store.dir, ".asset-")
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(f.Name())
	h := sha256.New()
	n, err := fetchRelease(ctx, a.client, p.Asset, p.Size, io.MultiWriter(f, h))
	if err != nil {
		return err
	}
	if n != p.Size || hex.EncodeToString(h.Sum(nil)) != p.SHA256 {
		return fmt.Errorf("asset integrity mismatch: %s", p.Name)
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Link rather than rename: never overwrite a pre-existing path.
	if err = os.Link(f.Name(), path); err != nil {
		return err
	}
	if err = os.Remove(f.Name()); err != nil {
		return err
	}
	d, err := os.Open(a.store.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err = d.Sync(); err != nil {
		return err
	}
	return a.Check(ctx, p)
}
