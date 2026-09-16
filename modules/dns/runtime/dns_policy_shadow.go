package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDNSPolicyShadowConcurrency = 32
	maxDNSPolicyShadowConcurrency     = 256
)

type DNSPolicyShadowConfig struct {
	ListenAddr    string
	Upstream      string
	Rules         []DNSPolicyRule
	Allowed       map[string]bool
	Timeout       time.Duration
	MaxConcurrent int
}

type DNSPolicyShadowPlan struct {
	Message    DNSMessage
	Evaluation DNSPolicyEvaluation
	Target     DNSPolicyEgressTarget
}

type DNSPolicyShadowStats struct {
	Requests          uint64
	Successes         uint64
	Failures          uint64
	ServfailResponses uint64
	InFlight          int
	PeakInFlight      int
	PolicySelections  map[string]uint64
}

func validateDNSPolicyShadowConfig(cfg DNSPolicyShadowConfig) (DNSPolicyShadowConfig, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(cfg.ListenAddr))
	if err != nil {
		return DNSPolicyShadowConfig{}, fmt.Errorf("invalid shadow listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return DNSPolicyShadowConfig{}, fmt.Errorf("shadow listener must use an explicit loopback IP")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return DNSPolicyShadowConfig{}, fmt.Errorf("invalid shadow listen port")
	}
	if port == 53 {
		return DNSPolicyShadowConfig{}, fmt.Errorf("shadow listener must not bind port 53")
	}
	upHost, upPort, err := net.SplitHostPort(strings.TrimSpace(cfg.Upstream))
	if err != nil || net.ParseIP(upHost) == nil {
		return DNSPolicyShadowConfig{}, fmt.Errorf("shadow upstream must be an explicit IP:port")
	}
	if p, err := strconv.Atoi(upPort); err != nil || p <= 0 || p > 65535 {
		return DNSPolicyShadowConfig{}, fmt.Errorf("invalid shadow upstream port")
	}
	if cfg.Allowed == nil {
		return DNSPolicyShadowConfig{}, fmt.Errorf("shadow policy inventory is required")
	}
	rules, err := validateDNSPolicyRules(cfg.Rules, cfg.Allowed)
	if err != nil {
		return DNSPolicyShadowConfig{}, fmt.Errorf("validate shadow rules: %w", err)
	}
	cfg.ListenAddr = net.JoinHostPort(ip.String(), portText)
	cfg.Upstream = net.JoinHostPort(upHost, upPort)
	cfg.Rules = rules
	if cfg.Timeout <= 0 {
		cfg.Timeout = 4 * time.Second
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = defaultDNSPolicyShadowConcurrency
	}
	if cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > maxDNSPolicyShadowConcurrency {
		return DNSPolicyShadowConfig{}, fmt.Errorf("shadow max concurrency must be between 1 and %d", maxDNSPolicyShadowConcurrency)
	}
	return cfg, nil
}

func planDNSPolicyShadowQuery(query []byte, clientIP string, cfg DNSPolicyShadowConfig, routes map[string]policyRoute) (DNSPolicyShadowPlan, error) {
	msg, ok := parseDNSMessage(query)
	if !ok || msg.QR || msg.QDCount != 1 || strings.TrimSpace(msg.QName) == "" {
		return DNSPolicyShadowPlan{}, fmt.Errorf("invalid shadow DNS query")
	}
	evaluation, err := evaluateDNSPolicy(DNSPolicyEvaluationRequest{
		ClientIP:  clientIP,
		Domain:    msg.QName,
		QueryType: qtypeName(msg.QType),
		Rules:     cfg.Rules,
	}, cfg.Allowed)
	if err != nil {
		return DNSPolicyShadowPlan{}, err
	}
	target, err := resolveDNSPolicyEgressTarget(evaluation.Policy, routes)
	if err != nil {
		return DNSPolicyShadowPlan{}, err
	}
	return DNSPolicyShadowPlan{Message: msg, Evaluation: evaluation, Target: target}, nil
}

func buildDNSPolicyShadowFailureResponse(query []byte, rcode uint8) ([]byte, error) {
	msg, ok := parseDNSMessage(query)
	if !ok || msg.QR || msg.QDCount != 1 {
		return nil, fmt.Errorf("cannot build shadow DNS failure response")
	}
	_, off, ok := parseDNSName(query, 12)
	if !ok || off+4 > len(query) {
		return nil, fmt.Errorf("cannot locate shadow DNS question")
	}
	response := append([]byte(nil), query[:off+4]...)
	flags := binary.BigEndian.Uint16(response[2:4])
	flags |= 0x8000
	flags &^= 0x000f
	flags |= uint16(rcode & 0x0f)
	binary.BigEndian.PutUint16(response[2:4], flags)
	binary.BigEndian.PutUint16(response[6:8], 0)
	binary.BigEndian.PutUint16(response[8:10], 0)
	binary.BigEndian.PutUint16(response[10:12], 0)
	return response, nil
}
