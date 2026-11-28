//go:build linux

package apptransaction

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const coreEvidenceDir = "/opt/var/run/routerforge-core-evidence"
const coreEvidencePath = "/v1/core-lifecycle"

// CoreProfilingEvidence is a management-plane observation, not a complete
// lifecycle proof. The Core listener is bound and its local process answers;
// this does not prove that an external client can reach the web UI.
type CoreProfilingEvidence struct {
	RuntimeVersion             string
	PeerPID                    int
	Enabled, Running           bool
	Listen                     string
	SlowMS                     int
	MarkerSHA256, ConfigSHA256 string // empty means positively absent
}

type coreEvidenceServer struct {
	server   *http.Server
	listener *net.UnixListener
	lock     *os.File
	once     sync.Once
}

func (s *coreEvidenceServer) Close() error {
	s.once.Do(func() { _ = s.server.Close(); _ = s.listener.Close(); _ = s.lock.Close() })
	return nil
}
func coreEvidenceOwned(info os.FileInfo, uid uint32) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uid
}
func safeEvidenceDirectory(path string, uid uint32, private bool) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && coreEvidenceOwned(info, uid) && info.Mode().Perm()&0022 == 0 && (!private || info.Mode().Perm() == 0700)
}

// StartCoreLifecycleEvidence is called only after Core's existing HTTP listener
// has successfully bound, and only in Dev ARM64. No TCP listener is added,
// no worker is launched, and this root-only Unix endpoint exposes no secrets.
func StartCoreLifecycleEvidence(version string, snapshot func() map[string]any) (io.Closer, error) {
	if os.Geteuid() != 0 {
		return nil, errors.New("core evidence requires root")
	}
	return startCoreEvidence(coreEvidenceDir, 0, version, snapshot)
}
func startCoreEvidence(dir string, uid uint32, version string, snapshot func() map[string]any) (io.Closer, error) {
	if version == "" || len(version) > 128 || snapshot == nil || !safeEvidenceDirectory(filepath.Dir(dir), uid, false) {
		return nil, errors.New("unsafe core evidence parent or identity")
	}
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if !safeEvidenceDirectory(dir, uid, true) {
		return nil, errors.New("unsafe core evidence directory")
	}
	fd, err := syscall.Open(filepath.Join(dir, "listener.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	lock := os.NewFile(uintptr(fd), "core evidence lock")
	success := false
	defer func() {
		if !success {
			_ = lock.Close()
		}
	}()
	fi, err := lock.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm() != 0600 || !coreEvidenceOwned(fi, uid) || fi.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, errors.New("unsafe core evidence lock")
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	sock := filepath.Join(dir, "core.sock")
	if old, e := os.Lstat(sock); e == nil {
		if old.Mode()&os.ModeSocket == 0 || !coreEvidenceOwned(old, uid) || old.Mode().Perm() != 0600 {
			return nil, errors.New("unsafe stale core socket")
		}
		conn, e := net.DialTimeout("unix", sock, time.Second)
		if e == nil {
			_ = conn.Close()
			return nil, errors.New("core evidence already running")
		}
		if !errors.Is(e, syscall.ECONNREFUSED) {
			return nil, errors.New("ambiguous stale core socket")
		}
		current, e := os.Lstat(sock)
		if e != nil || !os.SameFile(old, current) {
			return nil, errors.New("core socket changed")
		}
		if e = os.Remove(sock); e != nil {
			return nil, e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock, Net: "unix"})
	if err != nil {
		return nil, err
	}
	defer func() {
		if !success {
			_ = l.Close()
		}
	}()
	if err = os.Chmod(sock, 0600); err != nil {
		return nil, err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != coreEvidencePath || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "version": version, "module_abi": "v1", "web_bound": true, "profiling": snapshot()})
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 3 * time.Second, MaxHeaderBytes: 8192}
	s := &coreEvidenceServer{server: server, listener: l, lock: lock}
	success = true
	go func() { _ = server.Serve(l) }()
	return s, nil
}

// Only read bounded root-owned regular files. Missing files are not errors;
// permissions, symlinks, unstable contents and unreadable files are errors.
func readCoreFile(path string, uid uint32) (string, []byte, error) {
	if !safeEvidenceDirectory(filepath.Dir(path), uid, false) {
		return "", nil, errors.New("unsafe profiling state parent")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || !coreEvidenceOwned(st, uid) || st.Mode().Perm()&0022 != 0 || st.Size() > 64<<10 {
		return "", nil, errors.New("unsafe profiling state file")
	}
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return "", nil, errors.New("profiling state read failed")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(st, after) || st.Size() != after.Size() || int64(len(b)) != st.Size() || st.ModTime() != after.ModTime() || st.Mode() != after.Mode() {
		return "", nil, errors.New("profiling state changed during read")
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), b, nil
}
func expectedProfilingConfig(raw []byte) (string, int, error) {
	listen, slow := "127.0.0.1:6061", 750
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "listen":
			if value != "" {
				listen = value
			}
		case "slow_ms":
			if n, e := strconv.Atoi(value); e == nil && n >= 50 && n <= 60000 {
				slow = n
			}
		}
	}
	return listen, slow, sc.Err()
}
func decodeCoreEvidence(body []byte, version string) (CoreProfilingEvidence, error) {
	var out CoreProfilingEvidence
	f, e := nativeObject(body)
	if e != nil {
		return out, e
	}
	ok, e1 := requiredNativeField[bool](f, "ok")
	v, e2 := requiredNativeField[string](f, "version")
	abi, e3 := requiredNativeField[string](f, "module_abi")
	bound, e4 := requiredNativeField[bool](f, "web_bound")
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || !ok || !bound || v != version || abi != "v1" {
		return out, errors.New("core version or bound listener not proven")
	}
	p, e := nativeObject(f["profiling"])
	if e != nil {
		return out, e
	}
	enabled, e1 := requiredNativeField[bool](p, "enabled")
	running, e2 := requiredNativeField[bool](p, "running")
	listen, e3 := requiredNativeField[string](p, "listen")
	slow, e4 := requiredNativeField[int](p, "slow_ms")
	last, e5 := requiredNativeField[string](p, "error")
	mode, e6 := requiredNativeField[string](p, "mode")
	host, port, addrErr := net.SplitHostPort(listen)
	ip := net.ParseIP(host)
	n, portErr := strconv.Atoi(port)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil || enabled != running || last != "" || mode != "loopback-only" || slow < 50 || slow > 60000 || addrErr != nil || ip == nil || !ip.IsLoopback() || portErr != nil || n < 1 || n > 65535 {
		return out, errors.New("profiling runtime not proven healthy")
	}
	return CoreProfilingEvidence{RuntimeVersion: v, Enabled: enabled, Running: running, Listen: listen, SlowMS: slow}, nil
}

func ProbeCoreProfilingEvidence(ctx context.Context, version string) (CoreProfilingEvidence, error) {
	return probeCoreProfilingEvidence(ctx, filepath.Join(coreEvidenceDir, "core.sock"), "/opt/bin/routerforge", "/opt/etc/routerforge/profiling.enabled", "/opt/etc/routerforge/profiling.conf", 0, version)
}
func probeCoreProfilingEvidence(ctx context.Context, socket, binary, marker, config string, uid uint32, version string) (CoreProfilingEvidence, error) {
	var out CoreProfilingEvidence
	if ctx == nil || version == "" || len(version) > 128 || !safeEvidenceDirectory(filepath.Dir(socket), uid, true) {
		return out, errors.New("missing or unsafe core evidence identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	m, _, e := readCoreFile(marker, uid)
	if e != nil {
		return out, e
	}
	c, raw, e := readCoreFile(config, uid)
	if e != nil {
		return out, e
	}
	var a, b CoreProfilingEvidence
	p, e := probeNativeEndpoint(ctx, socket, binary, uid, coreEvidencePath, func(raw []byte) error { var err error; a, err = decodeCoreEvidence(raw, version); return err })
	if e != nil {
		return out, e
	}
	q, e := probeNativeEndpoint(ctx, socket, binary, uid, coreEvidencePath, func(raw []byte) error { var err error; b, err = decodeCoreEvidence(raw, version); return err })
	if e != nil {
		return out, e
	}
	m2, _, e := readCoreFile(marker, uid)
	if e != nil {
		return out, e
	}
	c2, _, e := readCoreFile(config, uid)
	if e != nil {
		return out, e
	}
	if !p.same(q) || a != b || m != m2 || c != c2 || a.Enabled != (m != "") || ctx.Err() != nil {
		return out, errors.New("core or profiling intent drift")
	}
	if a.Enabled {
		listen, slow, err := expectedProfilingConfig(raw)
		if err != nil || a.Listen != listen || a.SlowMS != slow {
			return out, errors.New("profiling effective configuration differs from disk")
		}
	}
	a.PeerPID = p.pid
	a.MarkerSHA256 = m
	a.ConfigSHA256 = c
	return a, nil
}

// Compare only already validated observations. Do not infer profiling package
// version from Core's version: its package status must be checked independently.
func CompareCoreProfilingIntent(before, after CoreProfilingEvidence, targetCoreVersion string) error {
	if targetCoreVersion == "" || before.RuntimeVersion == "" || after.RuntimeVersion != targetCoreVersion || before.PeerPID <= 0 || after.PeerPID <= 0 || before.Enabled != after.Enabled || before.Running != after.Running || before.Listen != after.Listen || before.SlowMS != after.SlowMS || before.MarkerSHA256 != after.MarkerSHA256 || before.ConfigSHA256 != after.ConfigSHA256 {
		return fmt.Errorf("core version or profiling intent changed")
	}
	return nil
}
