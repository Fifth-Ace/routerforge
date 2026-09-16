package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const dnsRuntimeProbeBodyLimit int64 = 16 << 10

func validateDNSMutationDesired(desired *dnsConfigState, changed map[string]bool) error {
	if desired == nil {
		return errors.New("DNS mutation validator: desired state is nil")
	}
	if len(changed) == 0 {
		return errors.New("DNS mutation validator: changed protocol set is empty")
	}
	for protocol, enabled := range changed {
		if !enabled {
			continue
		}
		switch protocol {
		case "DNS", "DoT", "DoH":
		default:
			return fmt.Errorf("DNS mutation validator: unsupported protocol %q", protocol)
		}
	}
	if err := validateDNSPhysicalLimits(desired, changed); err != nil {
		return err
	}
	return nil
}

func (m *dnsControlManager) runRuntimeProbe(ctx context.Context) error {
	if m == nil || m.runtimeProbe == nil {
		return nil
	}
	return m.runtimeProbe(ctx)
}

func newDNSRuntimeProbe(socket string) func(context.Context) error {
	socket = strings.TrimSpace(socket)
	return func(ctx context.Context) error {
		return probeDNSModuleRuntime(ctx, socket)
	}
}

func probeDNSModuleRuntime(ctx context.Context, socket string) error {
	socket = strings.TrimSpace(socket)
	if socket == "" {
		return errors.New("DNS runtime probe socket is empty")
	}

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
	}
	defer transport.CloseIdleConnections()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://routerforge/v1/health", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("DNS module health request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("DNS module health returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, dnsRuntimeProbeBodyLimit+1))
	if err != nil {
		return fmt.Errorf("DNS module health read failed: %w", err)
	}
	if int64(len(body)) > dnsRuntimeProbeBodyLimit {
		return errors.New("DNS module health response exceeds safety limit")
	}

	var health struct {
		OK          bool   `json:"ok"`
		Module      string `json:"module"`
		MutationAPI bool   `json:"mutation_api"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("DNS module health response is invalid JSON: %w", err)
	}
	if !health.OK || health.Module != "dns" || !health.MutationAPI {
		return fmt.Errorf(
			"DNS module health contract mismatch: ok=%t module=%q mutation_api=%t",
			health.OK,
			health.Module,
			health.MutationAPI,
		)
	}
	return nil
}
