package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	v2DirectPortBase       = 40000
	v2DirectPortsPerWorker = 64
	v2DirectReadCap        = int64(128 << 10)
	v2DirectMinBytes       = int64(16 << 10)
	v2DirectProcMark       = "0x40000000/0x40000000"
)

type v2DirectSandbox struct {
	capabilities benchCapabilities
	inventory    benchStrategyInventory
	worker       int
	queue        int
	portLo       int
	portHi       int
	target       string
	ip           string
	writableDir  string
	postChain    string
	preChain     string

	mu     sync.Mutex
	cmd    *exec.Cmd
	done   chan struct{}
	log    string
	next   int
	active bool
}

type v2DirectProbeResult struct {
	OK        bool
	Metrics   v2HTTPMetrics
	Error     string
	LocalPort int
}

func newV2DirectSandbox(capabilities benchCapabilities, inventory benchStrategyInventory, worker, queue int, target, ip string) *v2DirectSandbox {
	lo := v2DirectPortBase + worker*v2DirectPortsPerWorker
	return &v2DirectSandbox{
		capabilities: capabilities,
		inventory:    inventory,
		worker:       worker,
		queue:        queue,
		portLo:       lo,
		portHi:       lo + v2DirectPortsPerWorker - 1,
		target:       target,
		ip:           ip,
		writableDir:  filepath.Join("/tmp/routerforge-direct-selector", fmt.Sprintf("w%d", worker)),
		postChain:    fmt.Sprintf("RFDS_POST_%d", worker),
		preChain:     fmt.Sprintf("RFDS_PRE_%d", worker),
	}
}

func (s *v2DirectSandbox) ipt(args ...string) error {
	full := append([]string{"-w"}, args...)
	out, err := runBenchCommand(context.Background(), s.capabilities.IPTablesPath, full...)
	if err != nil {
		return fmt.Errorf("iptables %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *v2DirectSandbox) iptQuiet(args ...string) {
	full := append([]string{"-w"}, args...)
	_, _ = runBenchCommand(context.Background(), s.capabilities.IPTablesPath, full...)
}

func (s *v2DirectSandbox) RulesUp(queue bool) error {
	s.RulesDown()
	if err := s.ipt("-t", "mangle", "-N", s.postChain); err != nil && !strings.Contains(err.Error(), "Chain already exists") {
		return err
	}
	if err := s.ipt("-t", "mangle", "-F", s.postChain); err != nil {
		return err
	}
	if err := s.ipt("-t", "mangle", "-N", s.preChain); err != nil && !strings.Contains(err.Error(), "Chain already exists") {
		return err
	}
	if err := s.ipt("-t", "mangle", "-F", s.preChain); err != nil {
		return err
	}

	sport := fmt.Sprintf("%d:%d", s.portLo, s.portHi)
	q := strconv.Itoa(s.queue)

	// nfqws-generated packets keep the process mark and must never loop back
	// through the candidate queue.
	if err := s.ipt("-t", "mangle", "-A", s.postChain, "-m", "mark", "--mark", v2DirectProcMark, "-j", "RETURN"); err != nil {
		return err
	}

	outTuple := []string{"-p", "tcp", "-d", s.ip, "--sport", sport, "--dport", "443"}
	inTuple := []string{"-p", "tcp", "-s", s.ip, "--sport", "443", "--dport", sport}

	// Mark every probe packet before the normal production nfqws chains see it.
	// RouterForge production already treats the nfqws process mark as excluded;
	// this gives the direct selector the same isolation semantics without the old
	// anchor/order transaction machinery.
	if err := s.ipt("-t", "mangle", "-A", s.postChain, append(append([]string{}, outTuple...), "-j", "MARK", "--set-xmark", v2DirectProcMark)...); err != nil {
		return err
	}
	if queue {
		args := append(append([]string{}, outTuple...),
			"-m", "connbytes", "--connbytes", "1:16", "--connbytes-mode", "packets", "--connbytes-dir", "original",
			"-j", "NFQUEUE", "--queue-num", q, "--queue-bypass")
		if err := s.ipt("-t", "mangle", "-A", s.postChain, args...); err != nil {
			return err
		}
	}
	if err := s.ipt("-t", "mangle", "-A", s.postChain, append(append([]string{}, outTuple...), "-j", "RETURN")...); err != nil {
		return err
	}

	if err := s.ipt("-t", "mangle", "-A", s.preChain, append(append([]string{}, inTuple...), "-j", "MARK", "--set-xmark", v2DirectProcMark)...); err != nil {
		return err
	}
	if queue {
		args := append(append([]string{}, inTuple...),
			"-m", "connbytes", "--connbytes", "1:16", "--connbytes-mode", "packets", "--connbytes-dir", "reply",
			"-j", "NFQUEUE", "--queue-num", q, "--queue-bypass")
		if err := s.ipt("-t", "mangle", "-A", s.preChain, args...); err != nil {
			return err
		}
	}
	if err := s.ipt("-t", "mangle", "-A", s.preChain, append(append([]string{}, inTuple...), "-j", "RETURN")...); err != nil {
		return err
	}

	if err := s.ipt("-t", "mangle", "-I", "POSTROUTING", "1", "-j", s.postChain); err != nil {
		return err
	}
	if err := s.ipt("-t", "mangle", "-I", "PREROUTING", "1", "-j", s.preChain); err != nil {
		s.iptQuiet("-t", "mangle", "-D", "POSTROUTING", "-j", s.postChain)
		return err
	}
	s.active = true
	return nil
}

func (s *v2DirectSandbox) RulesDown() {
	s.iptQuiet("-t", "mangle", "-D", "POSTROUTING", "-j", s.postChain)
	s.iptQuiet("-t", "mangle", "-D", "PREROUTING", "-j", s.preChain)
	s.iptQuiet("-t", "mangle", "-F", s.postChain)
	s.iptQuiet("-t", "mangle", "-X", s.postChain)
	s.iptQuiet("-t", "mangle", "-F", s.preChain)
	s.iptQuiet("-t", "mangle", "-X", s.preChain)
	s.active = false
}

func v2DirectStripRuntimeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "--daemon", arg == "-D",
			strings.HasPrefix(arg, "--pidfile="),
			strings.HasPrefix(arg, "--qnum="),
			strings.HasPrefix(arg, "--fwmark="),
			strings.HasPrefix(arg, "--user="):
			continue
		default:
			out = append(out, arg)
		}
	}
	return out
}

func (s *v2DirectSandbox) StartNfqws(profile benchStrategyProfile) error {
	s.StopNfqws()
	if err := os.MkdirAll(s.writableDir, 0o755); err != nil {
		return err
	}

	args := []string{
		"--qnum=" + strconv.Itoa(s.queue),
		"--fwmark=0x40000000",
		"--writable=" + s.writableDir,
	}
	args = append(args, v2DirectStripRuntimeArgs(s.inventory.BaseArgs)...)
	args = append(args, v2DirectStripRuntimeArgs(profile.Args)...)

	logPath := filepath.Join(s.writableDir, "launch.log")
	lf, err := os.Create(logPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(s.capabilities.CandidateBinary, args...)
	cmd.Stdout = lf
	cmd.Stderr = lf
	if err := cmd.Start(); err != nil {
		_ = lf.Close()
		return err
	}
	_ = lf.Close()

	done := make(chan struct{})
	s.mu.Lock()
	s.cmd = cmd
	s.done = done
	s.log = logPath
	s.mu.Unlock()

	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		bound, qerr := benchQueueIsBound(s.queue)
		if qerr == nil && bound {
			return nil
		}
		select {
		case <-done:
			data, _ := os.ReadFile(logPath)
			return fmt.Errorf("nfqws2 exited before queue %d bind: %s", s.queue, strings.TrimSpace(string(data)))
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.StopNfqws()
	return fmt.Errorf("nfqws2 queue %d bind timeout", s.queue)
}

func (s *v2DirectSandbox) StopNfqws() error {
	s.mu.Lock()
	cmd := s.cmd
	done := s.done
	s.cmd = nil
	s.done = nil
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	if done != nil {
		select {
		case <-done:
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	_ = cmd.Process.Kill()
	if done != nil {
		select {
		case <-done:
		case <-time.After(time.Second):
			return errors.New("nfqws2 did not stop after SIGKILL")
		}
	}
	return nil
}

func (s *v2DirectSandbox) pickPort() int {
	span := s.portHi - s.portLo + 1
	if span < 1 {
		span = 1
	}
	port := s.portLo + (s.next % span)
	s.next++
	return port
}

func (s *v2DirectSandbox) Probe(ctx context.Context) v2DirectProbeResult {
	port := s.pickPort()
	result := v2DirectProbeResult{LocalPort: port}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	dialer := &net.Dialer{
		Timeout:   6 * time.Second,
		LocalAddr: &net.TCPAddr{Port: port},
		Control: func(_, _ string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			_, remotePort, err := net.SplitHostPort(addr)
			if err != nil {
				remotePort = "443"
			}
			return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(s.ip, remotePort))
		},
		TLSClientConfig:     &tls.Config{ServerName: s.target, InsecureSkipVerify: true},
		TLSHandshakeTimeout: 6 * time.Second,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   true,
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}

	start := time.Now()
	var firstByte time.Time
	trace := &httptrace.ClientTrace{GotFirstResponseByte: func() { firstByte = time.Now() }}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(probeCtx, trace), http.MethodGet, "https://"+s.target+"/", nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (RouterForge direct selector)")

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Metrics.DurationMS = time.Since(start).Milliseconds()
		return result
	}
	defer resp.Body.Close()
	result.Metrics.TLSComplete = true
	result.Metrics.HTTPStatus = resp.StatusCode
	if !firstByte.IsZero() {
		result.Metrics.TTFBMS = firstByte.Sub(start).Milliseconds()
	}

	buf := make([]byte, 32*1024)
	var total int64
	var readErr error
	for total < v2DirectReadCap {
		n, e := resp.Body.Read(buf)
		total += int64(n)
		if e != nil {
			readErr = e
			break
		}
	}
	result.Metrics.Bytes = total
	result.Metrics.DurationMS = time.Since(start).Milliseconds()
	if result.Metrics.DurationMS > 0 {
		result.Metrics.ThroughputBPS = total * 1000 / result.Metrics.DurationMS
	}
	reachedCap := total >= v2DirectReadCap
	result.Metrics.ResponseComplete = readErr == io.EOF || reachedCap
	if readErr != nil && readErr != io.EOF && !reachedCap {
		result.Metrics.ReadError = readErr.Error()
		if total > 0 && total <= 17408 {
			result.Metrics.Cutoff16KSuspected = true
		}
	}
	result.Metrics.ProgressProven = resp.StatusCode >= 200 && resp.StatusCode < 400 && total > v2DirectMinBytes && !result.Metrics.Cutoff16KSuspected && result.Metrics.ResponseComplete
	result.OK = result.Metrics.ProgressProven
	if !result.OK {
		if result.Metrics.ReadError != "" {
			result.Error = result.Metrics.ReadError
		} else {
			result.Error = fmt.Sprintf("HTTP %d bytes=%d complete=%t truncated=%t", resp.StatusCode, total, result.Metrics.ResponseComplete, result.Metrics.Cutoff16KSuspected)
		}
	}
	return result
}

func (s *v2DirectSandbox) RunCandidate(ctx context.Context, profile benchStrategyProfile) v2BenchAttempt {
	attempt := v2BenchAttempt{
		Transport:        benchTransportHTTPS,
		Network:          "tcp",
		RemotePort:       443,
		DestinationIPv4:  s.ip,
		Queue:            s.queue,
		InfrastructureOK: false,
		CleanupProven:    false,
	}
	if err := s.StartNfqws(profile); err != nil {
		// A candidate that cannot start is a candidate failure, not a reason to
		// abort the whole catalog. The worker sandbox itself is still healthy.
		attempt.InfrastructureOK = true
		attempt.CleanupProven = true
		attempt.Error = err.Error()
		attempt.ResultClass = "FAILED"
		return attempt
	}
	attempt.InfrastructureOK = true
	attempt.StrategyPathExercised = true
	probe := s.Probe(ctx)
	attempt.LocalPort = probe.LocalPort
	attempt.Metrics = probe.Metrics
	attempt.OK = probe.OK
	attempt.Error = probe.Error
	if err := s.StopNfqws(); err != nil {
		attempt.CleanupProven = false
		if attempt.Error == "" {
			attempt.Error = err.Error()
		} else {
			attempt.Error += "; cleanup: " + err.Error()
		}
	} else {
		attempt.CleanupProven = true
	}
	if attempt.OK && attempt.CleanupProven {
		attempt.ResultClass = "WORKING"
	} else if attempt.InfrastructureOK && attempt.CleanupProven {
		attempt.ResultClass = "FAILED"
	} else {
		attempt.ResultClass = "INCONCLUSIVE"
	}
	return attempt
}

func v2RunDirectBaseline(ctx context.Context, capabilities benchCapabilities, inventory benchStrategyInventory, worker int, target, ip string) v2BenchAttempt {
	s := newV2DirectSandbox(capabilities, inventory, worker, capabilities.RecommendedQueue, target, ip)
	attempt := v2BenchAttempt{
		Transport:       benchTransportHTTPS,
		Network:         "tcp",
		RemotePort:      443,
		DestinationIPv4: ip,
		Queue:           0,
	}
	if err := s.RulesUp(false); err != nil {
		attempt.Error = "baseline rules: " + err.Error()
		attempt.ResultClass = "INCONCLUSIVE"
		return attempt
	}
	attempt.InfrastructureOK = true
	attempt.StrategyPathExercised = true
	probe := s.Probe(ctx)
	attempt.LocalPort = probe.LocalPort
	attempt.Metrics = probe.Metrics
	attempt.OK = probe.OK
	attempt.Error = probe.Error
	s.RulesDown()
	attempt.CleanupProven = true
	if attempt.OK {
		attempt.ResultClass = "WORKING"
	} else {
		attempt.ResultClass = "FAILED"
	}
	return attempt
}
