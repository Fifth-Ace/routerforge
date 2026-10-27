package dnsrouter

// DPR2-C: opt-in loopback-only shadow DNS forwarder. This file deliberately has
// no Keenetic, iptables, resolver configuration, listener :53 or daemon hooks.
// A query without an explicit matching rule fails closed (REFUSED).

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
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
	Listen  string
	Timeout time.Duration
	Workers int
	Dial    DialContextFunc // injectable for tests; production uses net.Dialer
}

type ShadowStats struct {
	Requests  uint64 `json:"requests"`
	Forwarded uint64 `json:"forwarded"`
	Refused   uint64 `json:"refused"`
	Failures  uint64 `json:"failures"`
	UDP       uint64 `json:"udp"`
	TCP       uint64 `json:"tcp"`
}

type ShadowForwarder struct {
	engine   *Engine
	opts     ShadowOptions
	sem      chan struct{}
	active   atomic.Bool
	req      atomic.Uint64
	ok       atomic.Uint64
	refused  atomic.Uint64
	failures atomic.Uint64
	udp      atomic.Uint64
	tcp      atomic.Uint64
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
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("shadow listener must bind an explicit loopback IP")
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
	return &ShadowForwarder{engine: engine, opts: opts, sem: make(chan struct{}, opts.Workers)}, nil
}

func (f *ShadowForwarder) Stats() ShadowStats {
	return ShadowStats{Requests: f.req.Load(), Forwarded: f.ok.Load(), Refused: f.refused.Load(), Failures: f.failures.Load(), UDP: f.udp.Load(), TCP: f.tcp.Load()}
}
func (f *ShadowForwarder) Active() bool { return f.active.Load() }

func (f *ShadowForwarder) Serve(ctx context.Context) error {
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
	ctx, cancel := context.WithTimeout(parent, f.opts.Timeout)
	defer cancel()
	targetNetwork := decision.Transport
	// A selected upstream transport applies to both UDP and TCP clients.
	if targetNetwork != "udp" && targetNetwork != "tcp" {
		f.failures.Add(1)
		return makeDNSError(query, 2)
	}
	conn, err := f.opts.Dial(ctx, targetNetwork, decision.Address)
	if err != nil {
		f.failures.Add(1)
		return makeDNSError(query, 2)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(f.opts.Timeout))
	var response []byte
	if targetNetwork == "udp" {
		if _, err = conn.Write(query); err == nil {
			buf := make([]byte, maxDNSFrame)
			var n int
			n, err = conn.Read(buf)
			if err == nil {
				response = append([]byte(nil), buf[:n]...)
			}
		}
	} else {
		frame := make([]byte, len(query)+2)
		binary.BigEndian.PutUint16(frame[:2], uint16(len(query)))
		copy(frame[2:], query)
		if _, err = conn.Write(frame); err == nil {
			var h [2]byte
			if _, err = io.ReadFull(conn, h[:]); err == nil {
				n := int(binary.BigEndian.Uint16(h[:]))
				if n < 12 {
					err = errors.New("short TCP DNS response")
				} else {
					response = make([]byte, n)
					_, err = io.ReadFull(conn, response)
				}
			}
		}
	}
	if err != nil || !sameDNSQuestion(query, response) {
		f.failures.Add(1)
		return makeDNSError(query, 2)
	}
	f.ok.Add(1)
	// UDP clients receive whatever the upstream returned; TCP retry on TC is
	// a subsequent gate. Never silently change upstream for a selected rule.
	return response
}
