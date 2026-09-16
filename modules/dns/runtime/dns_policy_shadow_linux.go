//go:build linux

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type dnsPolicyShadowServer struct {
	cfg           DNSPolicyShadowConfig
	routeProvider func() map[string]policyRoute
}

func newDNSPolicyShadowServer(cfg DNSPolicyShadowConfig, routeProvider func() map[string]policyRoute) (*dnsPolicyShadowServer, error) {
	validated, err := validateDNSPolicyShadowConfig(cfg)
	if err != nil {
		return nil, err
	}
	if routeProvider == nil {
		return nil, fmt.Errorf("shadow route provider is required")
	}
	return &dnsPolicyShadowServer{cfg: validated, routeProvider: routeProvider}, nil
}

func (s *dnsPolicyShadowServer) Serve(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return fmt.Errorf("shadow UDP listen: %w", err)
	}
	defer udpConn.Close()

	tcpLn, err := net.Listen("tcp4", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("shadow TCP listen: %w", err)
	}
	defer tcpLn.Close()

	go func() {
		<-ctx.Done()
		_ = udpConn.Close()
		_ = tcpLn.Close()
	}()

	errCh := make(chan error, 2)
	go func() { errCh <- s.serveUDP(ctx, udpConn) }()
	go func() { errCh <- s.serveTCP(ctx, tcpLn) }()

	err = <-errCh
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (s *dnsPolicyShadowServer) serveUDP(ctx context.Context, conn *net.UDPConn) error {
	buf := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if ctx.Err() != nil {
					return nil
				}
				continue
			}
			return err
		}
		query := append([]byte(nil), buf[:n]...)
		go func() {
			response, err := s.forward(ctx, "udp4", client.IP.String(), query)
			if err == nil {
				_, _ = conn.WriteToUDP(response, client)
			}
		}()
	}
}

func (s *dnsPolicyShadowServer) serveTCP(ctx context.Context, ln net.Listener) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(s.cfg.Timeout))
			var hdr [2]byte
			if _, err := io.ReadFull(c, hdr[:]); err != nil {
				return
			}
			n := int(binary.BigEndian.Uint16(hdr[:]))
			if n < 12 || n > 65535 {
				return
			}
			query := make([]byte, n)
			if _, err := io.ReadFull(c, query); err != nil {
				return
			}
			host, _, _ := net.SplitHostPort(c.RemoteAddr().String())
			response, err := s.forward(ctx, "tcp4", host, query)
			if err != nil {
				return
			}
			frame := make([]byte, 2+len(response))
			binary.BigEndian.PutUint16(frame[:2], uint16(len(response)))
			copy(frame[2:], response)
			_, _ = c.Write(frame)
		}(conn)
	}
}

func (s *dnsPolicyShadowServer) forward(parent context.Context, network, clientIP string, query []byte) ([]byte, error) {
	plan, err := planDNSPolicyShadowQuery(query, clientIP, s.cfg, s.routeProvider())
	if err != nil {
		return nil, err
	}
	dialer, err := newDNSPolicyMarkedDialer(plan.Target, s.cfg.Timeout)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, s.cfg.Timeout)
	defer cancel()
	conn, err := dialer.DialContext(ctx, network, s.cfg.Upstream)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.cfg.Timeout))

	if network == "udp4" {
		if _, err := conn.Write(query); err != nil {
			return nil, err
		}
		buf := make([]byte, 65535)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		response := append([]byte(nil), buf[:n]...)
		return validateDNSPolicyShadowResponse(query, response)
	}

	frame := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(query)))
	copy(frame[2:], query)
	if _, err := conn.Write(frame); err != nil {
		return nil, err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n < 12 || n > 65535 {
		return nil, fmt.Errorf("invalid shadow TCP DNS response length %d", n)
	}
	response := make([]byte, n)
	if _, err := io.ReadFull(conn, response); err != nil {
		return nil, err
	}
	return validateDNSPolicyShadowResponse(query, response)
}

func validateDNSPolicyShadowResponse(query, response []byte) ([]byte, error) {
	q, qok := parseDNSMessage(query)
	r, rok := parseDNSMessage(response)
	if !qok || !rok || !r.QR || q.ID != r.ID {
		return nil, fmt.Errorf("invalid shadow DNS response")
	}
	return response, nil
}
