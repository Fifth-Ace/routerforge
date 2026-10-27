package dnsrouter

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func queryBytes(domain string, id uint16) []byte {
	b := []byte{0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[:2], id)
	for _, part := range strings.Split(domain, ".") {
		b = append(b, byte(len(part)))
		b = append(b, part...)
	}
	return append(b, 0, 0, 1, 0, 1)
}
func successReply(q []byte) []byte {
	r := append([]byte(nil), q...)
	r[2] |= 0x80
	r[3] |= 0x80
	return r
}
func shadowConfig(transport string) Config {
	return Config{SchemaVersion: 1, Upstreams: []Upstream{{ID: "pool", Transport: transport, Address: "192.0.2.44:5300"}}, Rules: []Rule{{ID: "r1", Domain: "example.com", Match: "suffix", Upstream: "pool", Priority: 1}}}
}
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func TestShadowRejectsRiskyListeners(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:55353", "192.168.1.1:55353", "127.0.0.1:53", "127.0.0.1:1024", "[::]:55353", "localhost:55353"} {
		if _, err := NewShadowForwarder(shadowConfig("udp"), ShadowOptions{Listen: addr}); err == nil {
			t.Fatalf("unsafe listener accepted: %s", addr)
		}
	}
	if _, err := NewShadowForwarder(shadowConfig("udp"), ShadowOptions{Listen: "127.0.0.1:55353"}); err != nil {
		t.Fatal(err)
	}
}
func TestShadowRefusesUnmatchedAndLocalNamesWithoutDial(t *testing.T) {
	calls := 0
	f, err := NewShadowForwarder(shadowConfig("udp"), ShadowOptions{Listen: "127.0.0.1:55353", Dial: func(context.Context, string, string) (net.Conn, error) { calls++; return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"unmatched.example", "host.lan", "localhost", "home.arpa", "1.168.192.in-addr.arpa"} {
		r := f.resolve(context.Background(), queryBytes(domain, 88), "udp")
		if len(r) < 12 || r[3]&0x0f != 5 {
			t.Errorf("%s expected REFUSED, got %v", domain, r)
		}
	}
	if calls != 0 {
		t.Fatal("DNS leak to upstream")
	}
	s := f.Stats()
	if s.Refused != 5 || s.Forwarded != 0 {
		t.Fatalf("bad stats %+v", s)
	}
}
func TestShadowRejectsForgedResponse(t *testing.T) {
	u, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()
	go func() {
		buf := make([]byte, 512)
		n, a, e := u.ReadFrom(buf)
		if e == nil {
			r := successReply(buf[:n])
			r[1] ^= 1
			_, _ = u.WriteTo(r, a)
		}
	}()
	f, err := NewShadowForwarder(shadowConfig("udp"), ShadowOptions{Listen: "127.0.0.1:55353", Timeout: time.Second, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, u.LocalAddr().String())
	}})
	if err != nil {
		t.Fatal(err)
	}
	r := f.resolve(context.Background(), queryBytes("example.com", 33), "udp")
	if len(r) < 12 || r[3]&15 != 2 {
		t.Fatalf("invalid DNS answer not rejected %x", r)
	}
	if f.Stats().Failures != 1 {
		t.Fatal("not counted")
	}
}
func TestShadowUDPToUDPAndTCPToTCP(t *testing.T) {
	for _, proto := range []string{"udp", "tcp"} {
		t.Run(proto, func(t *testing.T) {
			var upstreamAddr string
			closeUpstream := func() {}
			if proto == "udp" {
				l, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				upstreamAddr = l.LocalAddr().String()
				closeUpstream = func() { _ = l.Close() }
				go func() {
					buf := make([]byte, 4096)
					n, a, e := l.ReadFrom(buf)
					if e == nil {
						_, _ = l.WriteTo(successReply(buf[:n]), a)
					}
				}()
			} else {
				l, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				upstreamAddr = l.Addr().String()
				closeUpstream = func() { _ = l.Close() }
				go func() {
					c, e := l.Accept()
					if e != nil {
						return
					}
					defer c.Close()
					var hdr [2]byte
					if _, e = io.ReadFull(c, hdr[:]); e != nil {
						return
					}
					q := make([]byte, int(binary.BigEndian.Uint16(hdr[:])))
					if _, e = io.ReadFull(c, q); e != nil {
						return
					}
					resp := successReply(q)
					binary.BigEndian.PutUint16(hdr[:], uint16(len(resp)))
					_, _ = c.Write(append(hdr[:], resp...))
				}()
			}
			defer closeUpstream()
			port := freePort(t)
			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			f, err := NewShadowForwarder(shadowConfig(proto), ShadowOptions{Listen: addr, Timeout: 2 * time.Second, Dial: func(ctx context.Context, network, server string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, upstreamAddr)
			}})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- f.Serve(ctx) }()
			deadline := time.Now().Add(time.Second)
			for !f.Active() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if !f.Active() {
				t.Fatal("listener not active")
			}
			q := queryBytes("www.example.com", 124)
			var r []byte
			if proto == "udp" {
				c, e := net.DialTimeout("udp", addr, time.Second)
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(time.Second))
				_, _ = c.Write(q)
				buf := make([]byte, 4096)
				n, e := c.Read(buf)
				if e != nil {
					t.Fatal(e)
				}
				r = buf[:n]
			} else {
				c, e := net.DialTimeout("tcp", addr, time.Second)
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(time.Second))
				h := []byte{byte(len(q) >> 8), byte(len(q))}
				_, _ = c.Write(append(h, q...))
				if _, e = io.ReadFull(c, h); e != nil {
					t.Fatal(e)
				}
				r = make([]byte, int(binary.BigEndian.Uint16(h)))
				if _, e = io.ReadFull(c, r); e != nil {
					t.Fatal(e)
				}
			}
			if !sameDNSQuestion(q, r) {
				t.Fatalf("unexpected response %x", r)
			}
			if r[3]&15 != 0 {
				t.Fatalf("upstream failure %x", r)
			}
			s := f.Stats()
			if s.Requests != 1 || s.Forwarded != 1 {
				t.Fatalf("bad stats %+v", s)
			}
			cancel()
			select {
			case e := <-done:
				if e != nil {
					t.Fatal(e)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("server did not stop")
			}
		})
	}
}
