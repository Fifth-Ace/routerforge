package dnsrouter

// DPR2-C/K: opt-in loopback shadow DNS; explicit private-LAN trial requires source ACL. This file has
// no Keenetic, iptables, resolver configuration, listener :53 or daemon hooks.
// A query without an explicit matching rule fails closed (REFUSED).

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxDNSFrame          = 65535
	defaultShadowWorkers = 32
)

type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type ShadowOptions struct {
	Listen         string
	AllowLAN       bool     // experimental explicit private IPv4 bind only
	AllowedClients []string // exact source IPv4 allowlist, required for LAN
	Timeout        time.Duration
	Workers        int
	Dial           DialContextFunc // injectable for tests; production uses net.Dialer
	RootCAs        *x509.CertPool  // test-only override via constructor; CLI always uses system roots
}

type ShadowStats struct {
	Requests  uint64                   `json:"requests"`
	Forwarded uint64                   `json:"forwarded"`
	Refused   uint64                   `json:"refused"`
	Failures  uint64                   `json:"failures"`
	UDP       uint64                   `json:"udp"`
	TCP       uint64                   `json:"tcp"`
	Failovers uint64                   `json:"failovers"`
	Upstreams map[string]UpstreamStats `json:"upstreams"`
}

type UpstreamStats struct {
	Attempts      uint64 `json:"attempts"`
	Successes     uint64 `json:"successes"`
	Failures      uint64 `json:"failures"`
	LastLatencyMS int64  `json:"last_latency_ms"`
}

type ShadowForwarder struct {
	allowedClients map[string]bool
	engine         *Engine
	opts           ShadowOptions
	sem            chan struct{}
	active         atomic.Bool
	req            atomic.Uint64
	ok             atomic.Uint64
	refused        atomic.Uint64
	failures       atomic.Uint64
	udp            atomic.Uint64
	tcp            atomic.Uint64
	failovers      atomic.Uint64
	statsMu        sync.Mutex
	upstreamStats  map[string]UpstreamStats
	doqP8          *doqClientP8
}

func NewShadowForwarder(cfg Config, opts ShadowOptions) (*ShadowForwarder, error) {
	engine, err := Compile(cfg)
	if err != nil {
		return nil, err
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(opts.Listen))
	if err != nil {
		return nil, fmt.Errorf("shadow listen: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return nil, errors.New("shadow listener must bind an explicit unicast IP")
	}
	isLAN := !ip.IsLoopback()
	if isLAN && (!opts.AllowLAN || ip.To4() == nil || !ip.IsPrivate()) {
		return nil, errors.New("private IPv4 shadow LAN listener requires explicit experimental opt-in")
	}
	if !isLAN && (opts.AllowLAN || len(opts.AllowedClients) != 0) {
		return nil, errors.New("loopback shadow must not use LAN options")
	}
	allowlist := make(map[string]bool)
	if isLAN {
		if len(opts.AllowedClients) == 0 || len(opts.AllowedClients) > 8 {
			return nil, errors.New("LAN shadow requires 1..8 exact source IPs")
		}
		for _, item := range opts.AllowedClients {
			client := net.ParseIP(item)
			if client == nil || client.To4() == nil || client.IsUnspecified() ||
				(!client.IsPrivate() && !client.IsLoopback()) || client.String() != item || allowlist[item] {
				return nil, errors.New("invalid or duplicate LAN shadow source IP")
			}
			allowlist[item] = true
		}
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1025 || n > 65535 || n == 53 {
		return nil, errors.New("shadow listener must use an unprivileged high port")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 3 * time.Second
	}
	if opts.Timeout < 200*time.Millisecond || opts.Timeout > 30*time.Second {
		return nil, errors.New("shadow timeout must be 200ms..30s")
	}
	if opts.Workers == 0 {
		opts.Workers = defaultShadowWorkers
	}
	if opts.Workers < 1 || opts.Workers > 128 {
		return nil, errors.New("shadow workers must be between 1 and 128")
	}
	if opts.Dial == nil {
		dialer := &net.Dialer{Timeout: opts.Timeout}
		opts.Dial = dialer.DialContext
	}
	return &ShadowForwarder{engine: engine, opts: opts, allowedClients: allowlist, sem: make(chan struct{}, opts.Workers), upstreamStats: make(map[string]UpstreamStats), doqP8: newDoQClientP8()}, nil
}

// allowSource drops unauthorized DNS packets before they reach policy/upstream logic.
// A LAN bind is never a general-purpose recursive resolver.
func (f *ShadowForwarder) allowSource(ip net.IP) bool {
	if !f.opts.AllowLAN {
		return ip.IsLoopback()
	}
	return f.allowedClients[ip.String()]
}

func (f *ShadowForwarder) Stats() ShadowStats {
	f.statsMu.Lock()
	defer f.statsMu.Unlock()
	upstreams := make(map[string]UpstreamStats, len(f.upstreamStats))
	for id, stat := range f.upstreamStats {
		upstreams[id] = stat
	}
	return ShadowStats{Requests: f.req.Load(), Forwarded: f.ok.Load(), Refused: f.refused.Load(), Failures: f.failures.Load(), UDP: f.udp.Load(), TCP: f.tcp.Load(), Failovers: f.failovers.Load(), Upstreams: upstreams}
}
func (f *ShadowForwarder) Active() bool { return f.active.Load() }

func (f *ShadowForwarder) Serve(ctx context.Context) error {
	defer f.doqP8.close()
	// Listening is always an explicitly invoked operation, never automatic.
	addr, err := net.ResolveUDPAddr("udp", f.opts.Listen)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("shadow udp listen: %w", err)
	}
	defer udpConn.Close()
	tcpLn, err := net.Listen("tcp", f.opts.Listen)
	if err != nil {
		return fmt.Errorf("shadow tcp listen: %w", err)
	}
	defer tcpLn.Close()
	f.active.Store(true)
	defer f.active.Store(false)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = udpConn.Close()
			_ = tcpLn.Close()
		case <-done:
		}
	}()
	defer close(done)
	var wg sync.WaitGroup
	errorsCh := make(chan error, 2)
	go func() { errorsCh <- f.serveUDP(ctx, udpConn, &wg) }()
	go func() { errorsCh <- f.serveTCP(ctx, tcpLn, &wg) }()
	first := <-errorsCh
	_ = udpConn.Close()
	_ = tcpLn.Close()
	<-errorsCh
	wg.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return first
}

func (f *ShadowForwarder) acquire(ctx context.Context) bool {
	select {
	case f.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}
func (f *ShadowForwarder) release() { <-f.sem }

func (f *ShadowForwarder) serveUDP(ctx context.Context, c *net.UDPConn, wg *sync.WaitGroup) error {
	buf := make([]byte, maxDNSFrame)
	for {
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		n, addr, err := c.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			return err
		}
		if !f.allowSource(addr.IP) {
			continue
		}
		data := append([]byte(nil), buf[:n]...)
		if !f.acquire(ctx) {
			return nil
		}
		wg.Add(1)
		go func(q []byte, peer *net.UDPAddr) {
			defer wg.Done()
			defer f.release()
			f.udp.Add(1)
			resp := f.resolve(ctx, q, "udp")
			if len(resp) > 0 {
				_, _ = c.WriteToUDP(resp, peer)
			}
		}(data, addr)
	}
}
func (f *ShadowForwarder) serveTCP(ctx context.Context, l net.Listener, wg *sync.WaitGroup) error {
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		peer, ok := c.RemoteAddr().(*net.TCPAddr)
		if !ok || !f.allowSource(peer.IP) {
			_ = c.Close()
			continue
		}
		if !f.acquire(ctx) {
			_ = c.Close()
			return nil
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer f.release()
			defer c.Close()
			f.tcp.Add(1)
			_ = c.SetDeadline(time.Now().Add(f.opts.Timeout))
			var hdr [2]byte
			if _, err := io.ReadFull(c, hdr[:]); err != nil {
				return
			}
			n := int(binary.BigEndian.Uint16(hdr[:]))
			if n < 12 {
				return
			}
			q := make([]byte, n)
			if _, err := io.ReadFull(c, q); err != nil {
				return
			}
			resp := f.resolve(ctx, q, "tcp")
			if len(resp) > 0 {
				out := make([]byte, 2+len(resp))
				binary.BigEndian.PutUint16(out[:2], uint16(len(resp)))
				copy(out[2:], resp)
				_, _ = c.Write(out)
			}
		}()
	}
}

type dnsQuestion struct {
	name   string
	qtype  uint16
	qclass uint16
	end    int
}

func parseDNSQuestion(data []byte) (dnsQuestion, error) {
	if len(data) < 17 || data[2]&0x80 != 0 || (data[2]>>3)&15 != 0 || binary.BigEndian.Uint16(data[4:6]) != 1 {
		return dnsQuestion{}, errors.New("expected standard single-question DNS query")
	}
	var labels []string
	at := 12
	for {
		if at >= len(data) {
			return dnsQuestion{}, errors.New("truncated qname")
		}
		length := int(data[at])
		at++
		if length == 0 {
			break
		}
		if length > 63 || at+length > len(data) {
			return dnsQuestion{}, errors.New("invalid/compressed qname")
		}
		labels = append(labels, string(data[at:at+length]))
		at += length
		if at > 267 {
			return dnsQuestion{}, errors.New("overlong qname")
		}
	}
	if at+4 > len(data) {
		return dnsQuestion{}, errors.New("truncated question")
	}
	d, err := normalizedDomain(strings.Join(labels, "."))
	if err != nil {
		return dnsQuestion{}, err
	}
	return dnsQuestion{name: d, qtype: binary.BigEndian.Uint16(data[at:]), qclass: binary.BigEndian.Uint16(data[at+2:]), end: at + 4}, nil
}
func makeDNSError(query []byte, rcode uint16) []byte {
	if len(query) < 12 {
		return nil
	}
	end := 12
	if q, err := parseDNSQuestion(query); err == nil {
		end = q.end
	}
	out := append([]byte(nil), query[:end]...)
	flags := binary.BigEndian.Uint16(out[2:4])
	flags = (flags | 0x8000) &^ uint16(0x000f)
	flags |= rcode
	binary.BigEndian.PutUint16(out[2:4], flags)
	if end == 12 {
		binary.BigEndian.PutUint16(out[4:6], 0)
	}
	for i := 6; i < 12; i++ {
		out[i] = 0
	}
	return out
}
func sameDNSQuestion(query, resp []byte) bool {
	if len(resp) < 17 || len(query) < 17 || resp[2]&0x80 == 0 ||
		binary.BigEndian.Uint16(query[:2]) != binary.BigEndian.Uint16(resp[:2]) ||
		binary.BigEndian.Uint16(resp[4:6]) != 1 {
		return false
	}
	// Some compliant replies can compress the question, but the first baseline
	// explicitly refuses such responses rather than trusting ambiguous packets.
	normalized := append([]byte(nil), resp...)
	normalized[2] &^= 0x80
	q, err := parseDNSQuestion(query)
	if err != nil {
		return false
	}
	r, err := parseDNSQuestion(normalized)
	if err != nil {
		return false
	}
	return q.name == r.name && q.qtype == r.qtype && q.qclass == r.qclass
}

func (f *ShadowForwarder) resolve(parent context.Context, query []byte, network string) []byte {
	f.req.Add(1)
	q, err := parseDNSQuestion(query)
	if err != nil {
		f.failures.Add(1)
		return makeDNSError(query, 1)
	}
	if q.qclass != 1 {
		f.refused.Add(1)
		return makeDNSError(query, 5)
	}
	decision, err := f.engine.Evaluate(q.name)
	if err != nil {
		f.failures.Add(1)
		return makeDNSError(query, 2)
	}
	if !decision.Matched || decision.Source != "policy" {
		// No implicit default DNS fallback and no local hostname leakage.
		f.refused.Add(1)
		return makeDNSError(query, 5)
	}
	chain, err := f.engine.UpstreamChainForDecision(decision)
	if err != nil {
		f.failures.Add(1)
		return makeDNSError(query, 2)
	}
	for index, upstream := range chain {
		if parent.Err() != nil {
			break
		}
		if index > 0 {
			f.failovers.Add(1)
		}
		ctx, cancel := context.WithTimeout(parent, f.opts.Timeout)
		started := time.Now()
		response, exchangeErr := f.exchange(ctx, upstream, query)
		cancel()
		success := exchangeErr == nil && sameDNSQuestion(query, response)
		if !success && exchangeErr == nil {
			exchangeErr = errors.New("DNS response validation failed")
		}
		f.recordUpstream(upstream.ID, success, time.Since(started))
		if success {
			f.ok.Add(1)
			return response
		}
		// Failover is only attempted on transport, HTTP, TLS or malformed DNS
		// failures. A valid NXDOMAIN/SERVFAIL answer must not leak to fallback.
		_ = exchangeErr
	}
	f.failures.Add(1)
	return makeDNSError(query, 2)
}

func (f *ShadowForwarder) recordUpstream(id string, success bool, elapsed time.Duration) {
	f.statsMu.Lock()
	defer f.statsMu.Unlock()
	s := f.upstreamStats[id]
	s.Attempts++
	if success {
		s.Successes++
	} else {
		s.Failures++
	}
	s.LastLatencyMS = elapsed.Milliseconds()
	f.upstreamStats[id] = s
}

func (f *ShadowForwarder) exchange(ctx context.Context, upstream Upstream, query []byte) ([]byte, error) {
	switch upstream.Transport {
	case "udp", "tcp":
		conn, err := f.opts.Dial(ctx, upstream.Transport, upstream.Address)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		if err = conn.SetDeadline(time.Now().Add(f.opts.Timeout)); err != nil {
			return nil, err
		}
		return exchangeDNSConn(conn, upstream.Transport, query)
	case "dot":
		conn, err := f.opts.Dial(ctx, "tcp", upstream.Address)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: upstream.ServerName, RootCAs: f.opts.RootCAs, MinVersion: tls.VersionTLS12})
		defer tlsConn.Close()
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return nil, fmt.Errorf("DoT TLS handshake: %w", err)
		}
		if err = tlsConn.SetDeadline(time.Now().Add(f.opts.Timeout)); err != nil {
			return nil, err
		}
		return exchangeDNSConn(tlsConn, "tcp", query)
	case "doh":
		return f.exchangeDoH(ctx, upstream, query)
	case "doq":
		return f.doqP8.exchange(ctx, upstream, f.opts, query)
	default:
		return nil, errors.New("unsupported upstream transport")
	}
}

func exchangeDNSConn(conn net.Conn, transport string, query []byte) ([]byte, error) {
	if transport == "udp" {
		if _, err := conn.Write(query); err != nil {
			return nil, err
		}
		buf := make([]byte, maxDNSFrame)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), buf[:n]...), nil
	}
	frame := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(query)))
	copy(frame[2:], query)
	if _, err := conn.Write(frame); err != nil {
		return nil, err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(hdr[:]))
	if length < 12 {
		return nil, errors.New("short DNS TCP response")
	}
	resp := make([]byte, length)
	_, err := io.ReadFull(conn, resp)
	return resp, err
}

func (f *ShadowForwarder) exchangeDoH(ctx context.Context, upstream Upstream, query []byte) ([]byte, error) {
	endpoint := &url.URL{Scheme: "https", Host: upstream.ServerName, Path: upstream.Path}
	transport := &http.Transport{
		Proxy: nil, ForceAttemptHTTP2: true, DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{ServerName: upstream.ServerName, RootCAs: f.opts.RootCAs, MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			// Ignore URI hostname for dialing: it is only used for SNI and
			// certificate validation; the explicit configured IP prevents
			// DNS bootstrap recursion and unexpected egress.
			return f.opts.Dial(ctx, "tcp", upstream.Address)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: f.opts.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("DoH redirect prohibited") }}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("Content-Type", "application/dns-message")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH unexpected HTTP status %d", resp.StatusCode)
	}
	if strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])) != "application/dns-message" {
		return nil, errors.New("DoH unexpected Content-Type")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxDNSFrame+1))
	if err != nil {
		return nil, err
	}
	if len(raw) < 12 || len(raw) > maxDNSFrame {
		return nil, errors.New("DoH DNS response size invalid")
	}
	return raw, nil
}
