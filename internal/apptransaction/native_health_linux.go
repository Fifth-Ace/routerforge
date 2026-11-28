//go:build linux

package apptransaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

// NativeModuleHealth is evidence about the management endpoint only. It is
// deliberately NOT a LifecycleObservation: boot intent, positive absence,
// auxiliary processes and dataplane readiness require separate proofs.
type NativeModuleHealth struct {
	Module         string
	RuntimeVersion string
	PeerPID        int
}

var nativeHealthModules = map[string]string{
	"routerforge-admin": "admin", "routerforge-dns": "dns", "routerforge-monitoring": "monitoring",
	"routerforge-network-tools": "network-tools", "routerforge-nfqws-manager": "nfqws-manager",
	"routerforge-antiscan-manager": "antiscan-manager", "routerforge-port-access-manager": "port-access-manager",
	"routerforge-crowdsec-manager": "crowdsec-manager",
}

// ProbeRunningModuleHealth queries the existing root-owned Unix endpoint of
// an allowlisted running daemon; no fallback, remote HTTP, command or mutation.
// installedVersion must come from a separate authoritative package observation.
// Core, DNS Router and Profiling intentionally have no generic adapter.
func ProbeRunningModuleHealth(ctx context.Context, name, installedVersion string) (NativeModuleHealth, error) {
	module, ok := nativeHealthModules[name]
	if !ok {
		return NativeModuleHealth{}, errors.New("native health contract unsupported")
	}
	return probeNativeHealth(ctx, filepath.Join("/opt/var/run", name+".sock"), filepath.Join("/opt/bin", name), 0, module, installedVersion)
}

func probeNativeHealth(ctx context.Context, socket, binary string, uid uint32, module, version string) (NativeModuleHealth, error) {
	var out NativeModuleHealth
	if ctx == nil || version == "" || len(version) > 128 {
		return out, errors.New("missing native health identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sock, err := os.Lstat(socket)
	if err != nil || sock.Mode()&os.ModeSocket == 0 {
		return out, errors.New("native socket unavailable")
	}
	owner, ok := sock.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uid || sock.Mode().Perm()&0077 != 0 {
		return out, errors.New("unsafe native socket ownership or permissions")
	}
	exe, err := os.Lstat(binary)
	if err != nil || !exe.Mode().IsRegular() {
		return out, errors.New("native binary unavailable")
	}
	exeOwner, ok := exe.Sys().(*syscall.Stat_t)
	if !ok || exeOwner.Uid != uid || exe.Mode().Perm()&0022 != 0 {
		return out, errors.New("unsafe native binary")
	}
	d := net.Dialer{}
	raw, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return out, errors.New("native socket connect failed")
	}
	defer raw.Close()
	uc, ok := raw.(*net.UnixConn)
	if !ok {
		return out, errors.New("not a Unix connection")
	}
	sc, err := uc.SyscallConn()
	if err != nil {
		return out, err
	}
	var cred *syscall.Ucred
	var peerErr error
	if err = sc.Control(func(fd uintptr) {
		cred, peerErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || peerErr != nil || cred == nil || cred.Uid != uid || cred.Pid <= 0 {
		return out, errors.New("native socket peer identity rejected")
	}
	procExe := fmt.Sprintf("/proc/%d/exe", cred.Pid)
	check := func() error {
		cur, e := os.Stat(procExe)
		if e != nil || !os.SameFile(exe, cur) {
			return errors.New("native process executable differs from installed binary")
		}
		disk, e := os.Lstat(binary)
		if e != nil || !os.SameFile(exe, disk) {
			return errors.New("native binary changed during probe")
		}
		current, e := os.Lstat(socket)
		if e != nil || !os.SameFile(sock, current) || current.Mode() != sock.Mode() {
			return errors.New("native socket changed during probe")
		}
		return nil
	}
	if err = check(); err != nil {
		return out, err
	}
	// Use the already authenticated connection exactly once. No redirect and no
	// network fallback. The host is a dummy HTTP Host header, not a DNS lookup.
	if deadline, ok := ctx.Deadline(); ok {
		if err = raw.SetDeadline(deadline); err != nil {
			return out, err
		}
	}
	var used atomic.Bool
	tr := &http.Transport{DisableKeepAlives: true, DisableCompression: true, MaxResponseHeaderBytes: 16 << 10, DialContext: func(context.Context, string, string) (net.Conn, error) {
		if !used.CompareAndSwap(false, true) {
			return nil, errors.New("native health redial forbidden")
		}
		return raw, nil
	}}
	defer tr.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://routerforge.local/v1/health", nil)
	if err != nil {
		return out, err
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return out, errors.New("native health request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, errors.New("native health status rejected")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return out, errors.New("native health body rejected")
	}
	if err = validateNativeHealth(body, module, version); err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = check(); err != nil {
		return out, err
	}
	return NativeModuleHealth{Module: module, RuntimeVersion: version, PeerPID: int(cred.Pid)}, nil
}

func validateNativeHealth(body []byte, module, version string) error {
	// Reject duplicate top-level fields (including otherwise ignored fields).
	// Unknown fields are permitted for existing and future health payloads.
	d := json.NewDecoder(bytes.NewReader(body))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return errors.New("invalid native health object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		k, e := d.Token()
		if e != nil {
			return e
		}
		key, ok := k.(string)
		if !ok {
			return errors.New("invalid native health key")
		}
		if _, exists := fields[key]; exists {
			return errors.New("duplicate native health field")
		}
		var raw json.RawMessage
		if e = d.Decode(&raw); e != nil {
			return e
		}
		fields[key] = raw
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') {
		return errors.New("invalid native health object end")
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing native health data")
	}
	var good bool
	var m, v string
	if json.Unmarshal(fields["ok"], &good) != nil || !good || json.Unmarshal(fields["module"], &m) != nil || m != module || json.Unmarshal(fields["version"], &v) != nil || v != version {
		return errors.New("native health module/version/readiness mismatch")
	}
	if module == "dns" {
		var ready bool
		var phase string
		if json.Unmarshal(fields["data_ready"], &ready) != nil || !ready || json.Unmarshal(fields["phase"], &phase) != nil || phase != "ready" {
			return errors.New("DNS data not ready")
		}
	}
	return nil
}
