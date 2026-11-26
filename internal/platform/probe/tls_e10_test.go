package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTLSVerifiedAndCertificate(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") }))
	defer s.Close()
	addr := s.Listener.Addr().String()
	p := x509.NewCertPool()
	p.AddCert(s.Certificate())
	good := tlsObserve(context.Background(), addr, &tls.Config{RootCAs: p, ServerName: "example.com", MinVersion: tls.VersionTLS12})
	if !good.OK || good.HandshakeMS == nil {
		t.Fatal(good)
	}
	bad := tlsObserve(context.Background(), addr, &tls.Config{RootCAs: p, ServerName: "wrong.example", MinVersion: tls.VersionTLS12})
	if bad.OK || bad.ErrorClass != "tls-certificate" {
		t.Fatal(bad)
	}
}
func TestTLSBusy(t *testing.T) {
	tlsSlots <- struct{}{}
	tlsSlots <- struct{}{}
	defer func() { <-tlsSlots; <-tlsSlots }()
	if r := TLS(context.Background(), "127.0.0.1:1", "localhost"); r.OK || r.ErrorClass != "busy" {
		t.Fatal(r)
	}
}
