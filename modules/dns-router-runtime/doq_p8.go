package dnsrouter

// DPR2-P8.1: DNS over QUIC (RFC 9250). Upstream address is a literal IP:port,
// never a hostname, so no bootstrap resolver can be contacted accidentally.
// Each DNS transaction uses its own bidirectional QUIC stream. The connection
// is pooled by upstream ID, not across providers or certificate names.

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

type doqClientP8 struct {
	mu       sync.Mutex
	sessions map[string]quic.Connection
	closed   bool
}

func newDoQClientP8() *doqClientP8 { return &doqClientP8{sessions: make(map[string]quic.Connection)} }

func (p *doqClientP8) close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for id, c := range p.sessions {
		_ = c.CloseWithError(0, "shutdown")
		delete(p.sessions, id)
	}
}

func (p *doqClientP8) invalidate(id string, c quic.Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sessions[id] == c {
		delete(p.sessions, id)
		_ = c.CloseWithError(0, "transaction error")
	}
}

// The dialer is isolated for negative TLS tests and synthetic local servers.
var dialDoQP8 = quic.DialAddr

func (p *doqClientP8) session(ctx context.Context, u Upstream, opts ShadowOptions) (quic.Connection, error) {
	if p == nil {
		return nil, errors.New("DoQ client not initialized")
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("DoQ client closed")
	}
	if c := p.sessions[u.ID]; c != nil && c.Context().Err() == nil {
		p.mu.Unlock()
		return c, nil
	}
	delete(p.sessions, u.ID)
	p.mu.Unlock()

	cfg := &tls.Config{ServerName: u.ServerName, RootCAs: opts.RootCAs, MinVersion: tls.VersionTLS13, NextProtos: []string{"doq"}}
	conn, err := dialDoQP8(ctx, u.Address, cfg, &quic.Config{MaxIdleTimeout: 30 * time.Second, KeepAlivePeriod: 0, Allow0RTT: false})
	if err != nil {
		return nil, fmt.Errorf("DoQ QUIC/TLS handshake: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		_ = conn.CloseWithError(0, "shutdown")
		return nil, errors.New("DoQ client closed")
	}
	if current := p.sessions[u.ID]; current != nil && current.Context().Err() == nil {
		_ = conn.CloseWithError(0, "duplicate session")
		return current, nil
	}
	p.sessions[u.ID] = conn
	return conn, nil
}

func (p *doqClientP8) exchange(ctx context.Context, u Upstream, opts ShadowOptions, query []byte) ([]byte, error) {
	if len(query) < 12 || len(query) > 65535 {
		return nil, errors.New("invalid DNS message size for DoQ")
	}
	c, err := p.session(ctx, u, opts)
	if err != nil {
		return nil, err
	}
	stream, err := c.OpenStreamSync(ctx)
	if err != nil {
		p.invalidate(u.ID, c)
		return nil, fmt.Errorf("DoQ open stream: %w", err)
	}
	defer stream.CancelRead(3) // harmless after EOF, bounds abandoned transactions
	deadline := time.Now().Add(opts.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := stream.SetDeadline(deadline); err != nil {
		stream.CancelWrite(3)
		return nil, err
	}

	payload := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(payload[:2], uint16(len(query)))
	copy(payload[2:], query)
	// RFC 9250 §4.2.1: DoQ Message ID MUST be zero on wire.
	payload[2], payload[3] = 0, 0
	if _, err := stream.Write(payload); err != nil {
		stream.CancelWrite(3)
		p.invalidate(u.ID, c)
		return nil, err
	}
	if err := stream.Close(); err != nil {
		p.invalidate(u.ID, c)
		return nil, err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(stream, hdr[:]); err != nil {
		p.invalidate(u.ID, c)
		return nil, fmt.Errorf("DoQ response length: %w", err)
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n < 12 || n > maxDNSFrame {
		p.invalidate(u.ID, c)
		return nil, fmt.Errorf("DoQ invalid DNS response size: %d", n)
	}
	answer := make([]byte, n)
	if _, err := io.ReadFull(stream, answer); err != nil {
		p.invalidate(u.ID, c)
		return nil, fmt.Errorf("DoQ truncated answer: %w", err)
	}
	if answer[0] != 0 || answer[1] != 0 {
		p.invalidate(u.ID, c)
		return nil, errors.New("DoQ response Message ID must be zero")
	}
	var trailing [1]byte
	count, readErr := stream.Read(trailing[:])
	if count != 0 || readErr != io.EOF {
		p.invalidate(u.ID, c)
		return nil, errors.New("DoQ stream has trailing data or missing FIN")
	}
	answer[0], answer[1] = query[0], query[1]
	return answer, nil
}
