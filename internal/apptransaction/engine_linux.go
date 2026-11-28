//go:build linux

// Package apptransaction implements a fail-closed, durable transaction engine.
// It has no package-manager or HTTP entry point. Callers must supply trusted
// release metadata and adapters, and share the same exclusion protocol.
package apptransaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const Schema = 1
const maxJournal = 128 << 10

var ErrBusy = errors.New("transaction executor busy")
var ErrRecovery = errors.New("transaction needs explicit recovery")
var token = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var commit = regexp.MustCompile(`^[a-f0-9]{40}$`)
var official = map[string]bool{"routerforge-core": true, "routerforge-dns": true, "routerforge-admin": true, "routerforge-monitoring": true, "routerforge-network-tools": true, "routerforge-nfqws-manager": true, "routerforge-antiscan-manager": true, "routerforge-port-access-manager": true, "routerforge-crowdsec-manager": true, "routerforge-dns-router": true, "routerforge-profiling": true}

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Asset   string `json:"asset"`
	Size    int64  `json:"size"`
}
type Plan struct {
	ID       string    `json:"id"`
	Channel  string    `json:"channel"`
	Target   string    `json:"target"`
	Commit   string    `json:"commit"`
	Packages []Package `json:"packages"`
}

func (p Plan) validate() error {
	if !token.MatchString(p.ID) || !commit.MatchString(p.Commit) || p.Channel != "dev" || p.Target != "aarch64-3.10" || len(p.Packages) == 0 || len(p.Packages) > 11 {
		return errors.New("invalid dev plan identity")
	}
	seen := map[string]bool{}
	last := 0
	version := p.Packages[0].Version
	// All packages must be from the same exact rolling Dev release.
	if !regexp.MustCompile(`^1~dev\.r[0-9]+\.[a-f0-9]{12}$`).MatchString(version) || !strings.HasSuffix(version, "."+p.Commit[:12]) {
		return errors.New("invalid dev release version")
	}
	for _, v := range p.Packages {
		priority := 0
		if v.Name == "routerforge-profiling" {
			priority = 1
		}
		if v.Name == "routerforge-core" {
			priority = 2
		}
		expected := v.Name + "_" + strings.ReplaceAll(version, "~", "-") + "_" + p.Target + ".ipk"
		if !official[v.Name] || seen[v.Name] || v.Version != version || v.Asset != expected || !digest.MatchString(v.SHA256) || v.Size <= 0 || v.Size > 64<<20 || priority < last {
			return fmt.Errorf("invalid package plan: %s", v.Name)
		}
		seen[v.Name] = true
		last = priority
	}
	return nil
}
func (p Plan) Hash() (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	b, e := json.Marshal(p)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

type Phase string

const (
	Preparing        Phase = "preparing"
	Prepared         Phase = "prepared"
	Applying         Phase = "applying"
	Verifying        Phase = "verifying"
	Completed        Phase = "completed"
	Failed           Phase = "failed"
	Cancelled        Phase = "cancelled"
	RecoveryRequired Phase = "recovery_required"
)

type Journal struct {
	Schema     int    `json:"schema"`
	Plan       Plan   `json:"plan"`
	PlanSHA256 string `json:"plan_sha256"`
	Phase      Phase  `json:"phase"`
	Next       int    `json:"next"`
	Downloaded int    `json:"downloaded,omitempty"`
	StartedAt  int64  `json:"started_at,omitempty"`
	FinishedAt int64  `json:"finished_at,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

// Hooks must stage verified immutable assets in a private directory, verify
// again before each install, use a shared cross-process mutation exclusion
// protocol with all legacy package paths, and check actual package/lifecycle
// state. An error after Install begins is ambiguous and never auto-retried.
type Hooks interface {
	Prepare(context.Context, Package) error
	Check(context.Context, Package) error
	Install(context.Context, Package) error
	Verify(context.Context, Package) error
}
type Store struct{ dir string }

// Open requires a pre-existing, root/service-owned private directory. It does
// not create or modify the device's configuration or package database.
func Open(dir string) (*Store, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, errors.New("invalid journal directory")
	}
	resolved, e := filepath.EvalSymlinks(dir)
	if e != nil || resolved != dir {
		return nil, errors.New("unsafe journal directory")
	}
	st, e := os.Lstat(dir)
	if e != nil {
		return nil, e
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.IsDir() || st.Mode().Perm() != 0700 || sys.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("journal directory must be private and owned")
	}
	return &Store{dir}, nil
}
func privateRegular(st os.FileInfo) bool {
	if st == nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 {
		return false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	return ok && sys.Uid == uint32(os.Geteuid()) && sys.Nlink == 1
}
func (s *Store) lock() (*os.File, error) {
	fd, e := syscall.Open(filepath.Join(s.dir, "executor.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "executor.lock")
	st, e := f.Stat()
	if e != nil || !privateRegular(st) {
		f.Close()
		return nil, errors.New("unsafe executor lock")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		if e == syscall.EWOULDBLOCK {
			return nil, ErrBusy
		}
		return nil, e
	}
	return f, nil
}
func (s *Store) Load() (Journal, error) {
	var j Journal
	fd, e := syscall.Open(filepath.Join(s.dir, "current.json"), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if e != nil {
		return j, e
	}
	f := os.NewFile(uintptr(fd), "current.json")
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !privateRegular(st) || st.Size() > maxJournal {
		return j, errors.New("unsafe journal")
	}
	d := json.NewDecoder(io.LimitReader(f, maxJournal+1))
	d.DisallowUnknownFields()
	if e = d.Decode(&j); e != nil {
		return j, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return j, errors.New("trailing journal data")
	}
	h, e := j.Plan.Hash()
	if e != nil || j.Schema != Schema || h != j.PlanSHA256 || j.Next < 0 || j.Next > len(j.Plan.Packages) || j.Downloaded < 0 || j.Downloaded > len(j.Plan.Packages) {
		return j, errors.New("invalid journal integrity")
	}
	switch j.Phase {
	case Preparing, Prepared, Applying, Verifying, Completed, Failed, Cancelled, RecoveryRequired:
	default:
		return j, errors.New("invalid journal phase")
	}
	if (j.Phase == Completed && j.Next != len(j.Plan.Packages)) || ((j.Phase == Applying || j.Phase == Verifying) && j.Next == len(j.Plan.Packages)) {
		return j, errors.New("invalid checkpoint")
	}
	return j, nil
}
func (s *Store) save(j Journal) error {
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	if len(b) > maxJournal {
		return errors.New("journal too large")
	}
	f, e := os.CreateTemp(s.dir, ".checkpoint-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(name, filepath.Join(s.dir, "current.json")); e != nil {
		return e
	}
	dir, e := os.Open(s.dir)
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}

// Execute is synchronous in the executor process, not an HTTP request. No
// automatic resume or replay is permitted, including after a crash. A store
// deliberately supports one transaction; archival/recovery belongs to the
// future owner API. Cancellation is honored only before the next mutation.
func (s *Store) Execute(ctx context.Context, p Plan, h Hooks) (Journal, error) {
	var j Journal
	p.Packages = append([]Package(nil), p.Packages...)
	hash, e := p.Hash()
	if e != nil {
		return j, e
	}
	if h == nil {
		return j, errors.New("missing hooks")
	}
	l, e := s.lock()
	if e != nil {
		return j, e
	}
	defer l.Close()
	if old, err := s.Load(); err == nil {
		return old, ErrRecovery
	} else if !errors.Is(err, os.ErrNotExist) {
		return j, err
	}
	j = Journal{Schema: Schema, Plan: p, PlanSHA256: hash, Phase: Preparing, StartedAt: time.Now().Unix()}
	if e = s.save(j); e != nil {
		return j, e
	}
	stop := func(phase Phase, code string, err error) (Journal, error) {
		j.Phase = phase
		j.ErrorCode = code
		j.FinishedAt = time.Now().Unix()
		if e := s.save(j); e != nil {
			return j, fmt.Errorf("checkpoint failed: %w", e)
		}
		return j, err
	}
	for _, v := range p.Packages {
		if e = ctx.Err(); e != nil {
			return stop(Cancelled, "cancelled_before_mutation", e)
		}
		if e = h.Prepare(ctx, v); e != nil {
			return stop(Failed, "prepare_failed", e)
		}
		j.Downloaded++
		if e = s.save(j); e != nil {
			return j, e
		}
	}
	j.Phase = Prepared
	if e = s.save(j); e != nil {
		return j, e
	}
	for _, v := range p.Packages {
		if e = ctx.Err(); e != nil {
			return stop(Cancelled, "cancelled_at_boundary", e)
		}
		if e = h.Check(ctx, v); e != nil {
			return stop(Failed, "asset_check_failed", e)
		}
		if e = ctx.Err(); e != nil {
			return stop(Cancelled, "cancelled_at_boundary", e)
		}
		j.Phase = Applying
		if e = s.save(j); e != nil {
			return j, e
		}
		// Cancellation must not terminate opkg mid-mutation. The trusted adapter
		// owns its bounded process supervision and returns ambiguity as an error.
		stable := context.WithoutCancel(ctx)
		if e = h.Install(stable, v); e != nil {
			return stop(RecoveryRequired, "install_uncertain", e)
		}
		j.Phase = Verifying
		if e = s.save(j); e != nil {
			return j, e
		}
		if e = h.Verify(stable, v); e != nil {
			return stop(RecoveryRequired, "verify_failed", e)
		}
		j.Next++
		j.Phase = Prepared
		if e = s.save(j); e != nil {
			return j, e
		}
	}
	j.Phase = Completed
	j.FinishedAt = time.Now().Unix()
	if e = s.save(j); e != nil {
		return j, e
	}
	return j, nil
}
