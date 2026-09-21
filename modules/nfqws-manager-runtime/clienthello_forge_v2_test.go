package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientHelloForgeRewritesSNIAndPreservesTLSShape(t *testing.T) {
	src, err := v2GenerateClientHelloWithOptions("example.com", []string{"h2", "http/1.1"}, 0x0303)
	if err != nil {
		t.Fatal(err)
	}
	before := v2ClientHelloForgeInfoFor(src)
	out, err := v2RewriteClientHelloSNI(src, "www.google.com")
	if err != nil {
		t.Fatal(err)
	}
	after := v2ClientHelloForgeInfoFor(out)
	if !after.Valid || after.SNI != "www.google.com" {
		t.Fatalf("after=%+v", after)
	}
	if strings.Join(before.ALPN, ",") != strings.Join(after.ALPN, ",") {
		t.Fatalf("ALPN changed: before=%v after=%v", before.ALPN, after.ALPN)
	}
	if before.ExtensionCount != after.ExtensionCount {
		t.Fatalf("extension count changed: %d -> %d", before.ExtensionCount, after.ExtensionCount)
	}
}

func TestClientHelloForgeSupportsLongerAndShorterSNI(t *testing.T) {
	src, err := v2GenerateClientHello("very-long-source-hostname.example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, sni := range []string{"a.co", "very-long-target-hostname-for-routerforge.example.net"} {
		out, err := v2RewriteClientHelloSNI(src, sni)
		if err != nil {
			t.Fatalf("%s: %v", sni, err)
		}
		info := v2ValidateClientHello(out)
		if !info.Valid || info.SNI != sni {
			t.Fatalf("%s: %+v", sni, info)
		}
	}
}

func TestClientHelloForgeHTTP(t *testing.T) {
	src, err := v2GenerateClientHello("example.com")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(v2ClientHelloForgeRequest{
		ContentBase64: base64.StdEncoding.EncodeToString(src),
		SNI:           "www.google.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/clienthello/forge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleV2ClientHelloForge(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["sni"] != "www.google.com" || got["valid"] != true {
		t.Fatalf("response=%v", got)
	}
}

func TestRobustCaptureAcceptsNon443TLS(t *testing.T) {
	hello, err := v2GenerateClientHello("example.com")
	if err != nil {
		t.Fatal(err)
	}
	pcap := v2TestPCAPWithClientHello(t, hello)
	const tcpDstOffset = 24 + 16 + 14 + 20 + 2
	binary.BigEndian.PutUint16(pcap[tcpDstOffset:tcpDstOffset+2], 8443)
	items, err := v2ParsePCAPClientHellosRobust(pcap)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DstPort != 8443 || !items[0].Valid {
		t.Fatalf("items=%+v", items)
	}
}

func TestClientHelloBlobIDAlwaysStartsSafe(t *testing.T) {
	if got := v2ClientHelloBlobID("4pda.to.bin"); got != "rf_4pda_to" {
		t.Fatalf("id=%q", got)
	}
}
