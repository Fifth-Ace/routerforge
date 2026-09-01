package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	soMark         = 36
	soBindToDevice = 25
)

type diagnosticRoute struct {
	mode          string
	mark          uint32
	table         int
	interfaceName string
	scope         string
}

func diagnosticLoop(store *Store, log *EventLogger) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	run := func() {
		candidates := store.DiagnosticCandidates(time.Now())
		sem := make(chan struct{}, 2)
		var wg sync.WaitGroup
		for _, u := range candidates {
			u := u
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				d := diagnoseUpstream(u)
				d.LastTrigger = "health-down"
				store.SetDiagnostic(u.Port, d)
				if d.Status == "FAIL" {
					log.Event("DIAG_FAIL", fmt.Sprintf("%s / %s / route=%s / stage=%s / %s", u.Profile, u.Name, d.RouteMode, d.Stage, d.Error))
				}
			}()
		}
		wg.Wait()
	}
	time.Sleep(10 * time.Second)
	run()
	for range t.C {
		run()
	}
}

func routeForUpstream(u UpstreamView) diagnosticRoute {
	if u.PolicyMark != 0 {
		return diagnosticRoute{
			mode: "policy-mark", mark: u.PolicyMark, table: u.PolicyTable,
			scope: fmt.Sprintf("policy=%s mark=0x%x table=%d", u.Profile, u.PolicyMark, u.PolicyTable),
		}
	}
	if strings.TrimSpace(u.LinuxInterface) != "" {
		return diagnosticRoute{
			mode: "interface", interfaceName: u.LinuxInterface,
			scope: fmt.Sprintf("configured-interface=%s linux-interface=%s", u.Interface, u.LinuxInterface),
		}
	}
	return diagnosticRoute{mode: "default", scope: "default-route"}
}

func defaultDiagnosticRoute() diagnosticRoute {
	return diagnosticRoute{mode: "default", scope: "default-route"}
}

func diagnoseUpstream(u UpstreamView) DiagnosticView {
	route := routeForUpstream(u)
	d := diagnoseOnce(u, route)
	d.RouteMode = route.mode
	d.PolicyMark = route.mark
	d.PolicyTable = route.table
	d.LinuxInterface = route.interfaceName

	if route.mode != "default" && d.Status == "FAIL" {
		fallback := diagnoseOnce(u, defaultDiagnosticRoute())
		d.DefaultStatus = fallback.Status
		d.DefaultStage = fallback.Stage
		d.DefaultError = fallback.Error
		if fallback.Status == "OK" {
			if route.mode == "policy-mark" {
				d.Assessment = "POLICY_ROUTE_FAIL"
			} else {
				d.Assessment = "INTERFACE_ROUTE_FAIL"
			}
			return d
		}
	}

	if d.Status == "OK" {
		if u.HealthStatus == "DOWN" {
			if route.mode == "policy-mark" || route.mode == "interface" {
				d.Assessment = "UPSTREAM_OK_LOCAL_PROXY_FAIL"
			} else {
				d.Assessment = "UPSTREAM_OK_LOCAL_PATH_FAIL"
			}
		} else {
			d.Assessment = "UPSTREAM_OK"
		}
	}
	return d
}

func dialerForRoute(r diagnosticRoute, timeout time.Duration) *net.Dialer {
	d := &net.Dialer{Timeout: timeout}
	if r.mark == 0 && r.interfaceName == "" {
		return d
	}
	d.Control = func(network, address string, c syscall.RawConn) error {
		var sockErr error
		if err := c.Control(func(fd uintptr) {
			if r.mark != 0 {
				if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, soMark, int(r.mark)); err != nil {
					sockErr = err
					return
				}
			}
			if r.interfaceName != "" {
				if err := syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, soBindToDevice, r.interfaceName); err != nil {
					sockErr = err
				}
			}
		}); err != nil {
			return err
		}
		return sockErr
	}
	return d
}

func diagnoseOnce(u UpstreamView, route diagnosticRoute) DiagnosticView {
	now := time.Now()
	d := DiagnosticView{Ran: true, LastRun: &now, Status: "RUNNING", Stage: "RESOLVE", RouteScope: route.scope, RouteMode: route.mode, PolicyMark: route.mark, PolicyTable: route.table, LinuxInterface: route.interfaceName}
	host, port, endpoint, err := diagnosticEndpoint(u)
	if err != nil {
		d.Status = "FAIL"
		d.Assessment = "UPSTREAM_" + d.Stage
		d.Error = err.Error()
		return d
	}

	resolveStart := time.Now()
	ips, err := diagnosticIPs(host)
	d.ResolveMS = float64(time.Since(resolveStart).Microseconds()) / 1000
	if err != nil {
		d.Status = "FAIL"
		d.Stage = "RESOLVE"
		d.Assessment = "UPSTREAM_RESOLVE"
		d.Error = err.Error()
		return d
	}

	dialer := dialerForRoute(route, 3*time.Second)
	var conn net.Conn
	var chosen net.IP
	var lastErr error
	for _, ip := range ips {
		start := time.Now()
		c, e := dialer.Dial("tcp", net.JoinHostPort(ip.String(), port))
		if e == nil {
			conn = c
			chosen = ip
			d.TCPMS = float64(time.Since(start).Microseconds()) / 1000
			break
		}
		lastErr = e
	}
	if conn == nil {
		d.Status = "FAIL"
		d.Stage = "TCP"
		d.Assessment = "UPSTREAM_TCP"
		d.Error = fmt.Sprintf("connect %s:%s: %v", host, port, lastErr)
		return d
	}
	d.TargetIP = chosen.String()

	sni := strings.TrimSpace(u.SNI)
	if sni == "" {
		sni = host
	}
	if parsed := net.ParseIP(sni); parsed != nil && net.ParseIP(host) == nil {
		sni = host
	}
	tlsStart := time.Now()
	tlsConn := tls.Client(conn, &tls.Config{ServerName: sni, MinVersion: tls.VersionTLS12})
	_ = tlsConn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := tlsConn.Handshake(); err != nil {
		_ = conn.Close()
		d.Status = "FAIL"
		d.Stage = "TLS"
		d.Assessment = "UPSTREAM_TLS"
		d.Error = err.Error()
		return d
	}
	d.TLSMS = float64(time.Since(tlsStart).Microseconds()) / 1000

	if u.Protocol == "DoT" {
		d.Stage = "DNS"
		start := time.Now()
		rcode, err := diagnosticDoT(tlsConn)
		d.ProtocolMS = float64(time.Since(start).Microseconds()) / 1000
		_ = tlsConn.Close()
		if err != nil {
			d.Status = "FAIL"
			d.Assessment = "UPSTREAM_" + d.Stage
			d.Error = err.Error()
			return d
		}
		d.DNSRCode = rcode
		if rcode == "SERVFAIL" || rcode == "REFUSED" {
			d.Status = "FAIL"
			d.Assessment = "UPSTREAM_DNS"
			d.Error = "direct resolver returned " + rcode
			return d
		}
		d.Status = "OK"
		d.Stage = "DNS"
		return d
	}
	_ = tlsConn.Close()

	if u.Protocol == "DoH" {
		d.Stage = "HTTP"
		start := time.Now()
		status, rcode, err := diagnosticDoH(endpoint, host, port, chosen, sni, route)
		d.ProtocolMS = float64(time.Since(start).Microseconds()) / 1000
		d.HTTPStatus = status
		d.DNSRCode = rcode
		if err != nil {
			d.Status = "FAIL"
			d.Assessment = "UPSTREAM_" + d.Stage
			d.Error = err.Error()
			return d
		}
		d.Status = "OK"
		d.Stage = "DNS"
		return d
	}
	d.Status = "FAIL"
	d.Stage = "PROTOCOL"
	d.Assessment = "UPSTREAM_PROTOCOL"
	d.Error = "unsupported protocol " + u.Protocol
	return d
}

func diagnosticEndpoint(u UpstreamView) (host, port, endpoint string, err error) {
	if u.Protocol == "DoH" {
		parsed, e := url.Parse(u.Target)
		if e != nil || parsed.Hostname() == "" {
			return "", "", "", fmt.Errorf("invalid DoH URL: %s", u.Target)
		}
		host = parsed.Hostname()
		port = parsed.Port()
		if port == "" {
			port = "443"
		}
		return host, port, u.Target, nil
	}
	target := strings.TrimSpace(u.Target)
	if h, p, e := net.SplitHostPort(target); e == nil {
		return h, p, "", nil
	}
	if ip := net.ParseIP(target); ip != nil {
		return target, "853", "", nil
	}
	if strings.Count(target, ":") == 0 && target != "" {
		return target, "853", "", nil
	}
	return "", "", "", fmt.Errorf("invalid DoT target: %s", target)
}

func diagnosticIPs(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		if a.IP.To4() != nil {
			ips = append(ips, a.IP)
		}
	}
	for _, a := range addrs {
		if a.IP.To4() == nil {
			ips = append(ips, a.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	return ips, nil
}

func diagnosticDoT(c net.Conn) (string, error) {
	id := uint16(rand.Intn(65535))
	q := buildDNSQuery(id, fmt.Sprintf("%s%x.example.com", healthPrefix, rand.Uint64()), 1)
	frame := make([]byte, 2+len(q))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(q)))
	copy(frame[2:], q)
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write(frame); err != nil {
		return "", err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return "", err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n < 12 || n > 65535 {
		return "", fmt.Errorf("invalid DoT frame length %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		return "", err
	}
	d, ok := parseDNSMessage(buf)
	if !ok || !d.QR || d.ID != id {
		return "", fmt.Errorf("invalid DoT DNS response")
	}
	return rcodeName(d.RCode), nil
}

func diagnosticDoH(endpoint, host, port string, ip net.IP, sni string, route diagnosticRoute) (int, string, error) {
	id := uint16(rand.Intn(65535))
	q := buildDNSQuery(id, fmt.Sprintf("%s%x.example.com", healthPrefix, rand.Uint64()), 1)
	dialer := dialerForRoute(route, 3*time.Second)
	tr := &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{ServerName: sni, MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
		},
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 7 * time.Second}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(q))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	req.Host = host
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, "", fmt.Errorf("DoH HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return resp.StatusCode, "", err
	}
	d, ok := parseDNSMessage(body)
	if !ok || !d.QR || d.ID != id {
		return resp.StatusCode, "", fmt.Errorf("invalid DoH DNS response")
	}
	rcode := rcodeName(d.RCode)
	if d.RCode == 2 || d.RCode == 5 {
		return resp.StatusCode, rcode, fmt.Errorf("direct resolver returned %s", rcode)
	}
	return resp.StatusCode, rcode, nil
}
