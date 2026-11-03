package lite

// P24.3: direct-to-address HTTP(S) health probes. The HTTP Host and TLS SNI
// use the DNS question name but the outbound socket is pinned to its public IP.
// No proxy, DNS re-resolution, cookies, redirects, or unbounded response reads.
import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func httpProbeRequest(ctx context.Context, target ipProbeTarget, path string, client *http.Client) error {
	if path == "" {
		path = "/"
	}
	u := url.URL{Scheme: target.Protocol, Host: net.JoinHostPort(target.Domain, strconv.Itoa(target.Port)), Path: path}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "RouterForge-IP-Health/1")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Redirects are not followed by the real client. 3xx and 4xx establish
	// reachability; 5xx are server failures. This is not an application SLA.
	if resp.StatusCode < 200 || resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	_, err = io.CopyN(io.Discard, resp.Body, 1024)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (m *IPHealthMonitor) dialHTTP(ctx context.Context, target ipProbeTarget, start time.Time) (time.Duration, error) {
	port := net.JoinHostPort(target.IP.String(), strconv.Itoa(target.Port))
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", port)
		},
		TLSClientConfig:        &tls.Config{ServerName: target.Domain, MinVersion: tls.VersionTLS12},
		DisableKeepAlives:      true,
		DisableCompression:     true,
		MaxResponseHeaderBytes: 8 << 10,
		ForceAttemptHTTP2:      false,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	err := httpProbeRequest(ctx, target, m.cfg.HTTPPath, client)
	return time.Since(start), err
}
