package dnsrouter

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestP90CertificateInspectionTargets(t *testing.T) {
	good := CertificateInspectRequestP90{Transport: "doh", Address: "1.1.1.1:443", ServerName: "cloudflare-dns.com"}
	if err := validateCertRequestP90(good); err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{"127.0.0.1:443", "192.168.1.1:443", "10.0.0.1:443", "100.64.1.1:443", "169.254.2.3:443", "198.18.1.1:443", "203.0.113.1:443", "[::1]:443", "[fc00::1]:443", "[2606:4700:4700::1111]:443", "cloudflare-dns.com:443", "1.1.1.1:0"} {
		c := good
		c.Address = addr
		if err := validateCertRequestP90(c); err == nil {
			t.Errorf("unsafe target accepted %q", addr)
		}
	}
	bad := good
	bad.Transport = "udp"
	if validateCertRequestP90(bad) == nil {
		t.Fatal("UDP allowed")
	}
	bad = good
	bad.ServerName = "localhost"
	if validateCertRequestP90(bad) == nil {
		t.Fatal("local SNI allowed")
	}
}
func TestP90StoredPublicPEMOnly(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "dns.example.com"}, DNSNames: []string{"dns.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "dns.example.com"}, DNSNames: []string{"dns.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	if err := validateSavedCertificateP90("doh", "dns.example.com", leaf); err != nil {
		t.Fatal(err)
	}
	if validateSavedCertificateP90("doh", "wrong.example.com", leaf) == nil {
		t.Fatal("wrong SNI allowed")
	}
	if validateSavedCertificateP90("native", "dns.example.com", leaf) == nil {
		t.Fatal("native cert allowed")
	}
	if validateSavedCertificateP90("doh", "dns.example.com", leaf+leaf) == nil {
		t.Fatal("chain injection allowed")
	}
	if validateSavedCertificateP90("doh", "dns.example.com", strings.Repeat("A", maxCertificatePEMP90+1)) == nil {
		t.Fatal("oversized PEM allowed")
	}
}
