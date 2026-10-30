package dnsrouter

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	quic "github.com/quic-go/quic-go"
)

func doqCertificateP8(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "dns.test"}, DNSNames: []string{"dns.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	roots := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots.AddCert(parsed)
	return cert, roots
}
func TestP8DoQLocalRFC9250(t *testing.T) {
	cert, roots := doqCertificateP8(t)
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"doq"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	serverRelease := make(chan struct{})
	defer close(serverRelease)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		conn, err := listener.Accept(ctx)
		if err != nil {
			done <- err
			return
		}
		defer conn.CloseWithError(0, "done")
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			done <- err
			return
		}
		var h [2]byte
		if _, err = io.ReadFull(stream, h[:]); err != nil {
			done <- err
			return
		}
		q := make([]byte, int(binary.BigEndian.Uint16(h[:])))
		if _, err = io.ReadFull(stream, q); err != nil {
			done <- err
			return
		}
		if q[0] != 0 || q[1] != 0 {
			done <- io.ErrUnexpectedEOF
			return
		}
		response := append([]byte(nil), q...)
		response[2] = 0x81
		response[3] = 0x80
		binary.BigEndian.PutUint16(h[:], uint16(len(response)))
		_, err = stream.Write(append(h[:], response...))
		if err == nil {
			err = stream.Close()
		}
		done <- err
		if err == nil {
			<-serverRelease
		}
	}()
	addr := listener.Addr().(*net.UDPAddr)
	p := newDoQClientP8()
	defer p.close()
	q := []byte{0x12, 0x34, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	answer, err := p.exchange(ctx, Upstream{ID: "up", Transport: "doq", Address: addr.String(), ServerName: "dns.test"}, ShadowOptions{Timeout: 3 * time.Second, RootCAs: roots}, q)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(answer[:2], q[:2]) || answer[2] != 0x81 {
		t.Fatalf("unexpected DoQ response %x", answer)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestP8DoQRejectsBadCert(t *testing.T) {
	cert, _ := doqCertificateP8(t)
	ln, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"doq"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p := newDoQClientP8()
	defer p.close()
	_, err = p.exchange(ctx, Upstream{ID: "up", Transport: "doq", Address: ln.Addr().String(), ServerName: "wrong.test"}, ShadowOptions{Timeout: time.Second}, []byte{0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	if err == nil {
		t.Fatal("DoQ unexpectedly accepted untrusted server")
	}
}
func TestP8EncryptedOnlyGroup(t *testing.T) {
	cfg := Config{SchemaVersion: 1, Upstreams: []Upstream{{ID: "clear", Transport: "udp", Address: "9.9.9.9:53"}, {ID: "secure", Transport: "dot", Address: "1.1.1.1:853", ServerName: "dns.test"}}, Groups: []DNSGroup{{ID: "strict", Name: "Secure", Servers: []string{"secure", "clear"}, EncryptedOnly: true}}}
	if _, err := Compile(cfg); err == nil {
		t.Fatal("plaintext upstream accepted by encrypted-only group")
	}
	cfg.Groups[0].EncryptedOnly = false
	if _, err := Compile(cfg); err != nil {
		t.Fatalf("legacy mixed group must remain compatible: %v", err)
	}
}
