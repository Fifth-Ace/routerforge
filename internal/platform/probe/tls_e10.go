package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"time"
)

type TLSObservation struct {
	OK          bool
	TCPMS       *int64
	HandshakeMS *int64
	ErrorClass  string
}

// Only the explicitly requested address; standard hostname/IP SAN and trust verification.
// No certificate bypass, HTTP request, redirect, credentials, or fallback port.
var tlsSlots = make(chan struct{}, 2)

func TLS(ctx context.Context, address, serverName string) TLSObservation {
	return tlsObserve(ctx, address, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
}
func tlsObserve(parent context.Context, address string, cfg *tls.Config) TLSObservation {
	select {
	case tlsSlots <- struct{}{}:
		defer func() { <-tlsSlots }()
	default:
		return TLSObservation{ErrorClass: "busy"}
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	started := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	connectMS := time.Since(started).Milliseconds()
	out := TLSObservation{TCPMS: &connectMS}
	if err != nil {
		out.ErrorClass = "tcp-" + tlsErrorClass(err)
		return out
	}
	defer conn.Close()
	start := time.Now()
	client := tls.Client(conn, cfg)
	err = client.HandshakeContext(ctx)
	ms := time.Since(start).Milliseconds()
	out.HandshakeMS = &ms
	if err != nil {
		out.ErrorClass = "tls-" + tlsErrorClass(err)
		return out
	}
	out.OK = true
	return out
}
func tlsErrorClass(e error) string {
	var cert *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var host x509.HostnameError
	var ne net.Error
	switch {
	case errors.As(e, &cert) || errors.As(e, &unknown) || errors.As(e, &host):
		return "certificate"
	case errors.Is(e, context.Canceled):
		return "cancelled"
	case errors.Is(e, context.DeadlineExceeded) || errors.As(e, &ne) && ne.Timeout():
		return "timeout"
	default:
		return "failed"
	}
}
