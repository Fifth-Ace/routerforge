package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

var errDNSPolicyDataplaneMappingUnknown = errors.New("DNS policy dataplane mapping is not proven on hardware")

type dnsPolicyKeeneticPrimitive struct {
	rci           *dnsRCIClient
	runtimeHealth func() error
}

func newDNSPolicyKeeneticPrimitive(rci *dnsRCIClient, runtimeHealth func() error) *dnsPolicyKeeneticPrimitive {
	return &dnsPolicyKeeneticPrimitive{rci: rci, runtimeHealth: runtimeHealth}
}

var _ dnsPolicyRuntimePrimitive = (*dnsPolicyKeeneticPrimitive)(nil)

func (p *dnsPolicyKeeneticPrimitive) SnapshotRuntime() (DNSPolicyRuntimeSnapshot, error) {
	identity, err := p.readIdentity()
	if err != nil {
		return DNSPolicyRuntimeSnapshot{}, err
	}
	return DNSPolicyRuntimeSnapshot{Identity: identity, RuleCount: 0}, nil
}

func (p *dnsPolicyKeeneticPrimitive) ApplyCanonicalRules(_ []DNSPolicyRule) error {
	return errDNSPolicyDataplaneMappingUnknown
}

func (p *dnsPolicyKeeneticPrimitive) VerifyCanonicalRules(_ []DNSPolicyRule) error {
	return errDNSPolicyDataplaneMappingUnknown
}

func (p *dnsPolicyKeeneticPrimitive) RestoreRuntime(_ DNSPolicyRuntimeSnapshot) error {
	return errDNSPolicyDataplaneMappingUnknown
}

func (p *dnsPolicyKeeneticPrimitive) VerifyRuntimeSnapshot(snapshot DNSPolicyRuntimeSnapshot) error {
	if snapshot.Identity == "" {
		return fmt.Errorf("empty DNS policy runtime snapshot identity")
	}
	identity, err := p.readIdentity()
	if err != nil {
		return err
	}
	if identity != snapshot.Identity {
		return fmt.Errorf("DNS policy runtime identity mismatch: got %s want %s", identity, snapshot.Identity)
	}
	return nil
}

func (p *dnsPolicyKeeneticPrimitive) RuntimeHealth() error {
	if p == nil || p.runtimeHealth == nil {
		return fmt.Errorf("DNS policy runtime health probe is unavailable")
	}
	return p.runtimeHealth()
}

func (p *dnsPolicyKeeneticPrimitive) readIdentity() (string, error) {
	if p == nil || p.rci == nil {
		return "", fmt.Errorf("DNS policy RCI client is unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var policy any
	if err := p.rci.getJSON(ctx, "/ip/policy", &policy); err != nil {
		return "", fmt.Errorf("read /ip/policy: %w", err)
	}
	var hosts any
	if err := p.rci.getJSON(ctx, "/ip/hotspot/host", &hosts); err != nil {
		return "", fmt.Errorf("read /ip/hotspot/host: %w", err)
	}
	var runtime any
	if err := p.rci.getJSON(ctx, "/show/ip/policy", &runtime); err != nil {
		return "", fmt.Errorf("read /show/ip/policy: %w", err)
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("canonicalize /ip/policy: %w", err)
	}
	hostJSON, err := canonicalDNSPolicyHosts(hosts)
	if err != nil {
		return "", fmt.Errorf("canonicalize /ip/hotspot/host: %w", err)
	}
	runtimeJSON, err := json.Marshal(runtime)
	if err != nil {
		return "", fmt.Errorf("canonicalize /show/ip/policy: %w", err)
	}

	hash := sha256.New()
	for _, part := range []struct {
		name string
		body []byte
	}{
		{name: "/ip/policy", body: policyJSON},
		{name: "/ip/hotspot/host", body: hostJSON},
		{name: "/show/ip/policy", body: runtimeJSON},
	} {
		_, _ = hash.Write([]byte(part.name))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(part.body)
		_, _ = hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func canonicalDNSPolicyHosts(value any) ([]byte, error) {
	items, ok := value.([]any)
	if !ok {
		return json.Marshal(value)
	}
	type entry struct {
		key  string
		body []byte
	}
	entries := make([]entry, 0, len(items))
	for _, item := range items {
		body, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		key := string(body)
		if object, ok := item.(map[string]any); ok {
			if mac, ok := object["mac"].(string); ok {
				key = mac + "\x00" + key
			}
		}
		entries = append(entries, entry{key: key, body: body})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	ordered := make([]json.RawMessage, 0, len(entries))
	for _, item := range entries {
		ordered = append(ordered, json.RawMessage(item.body))
	}
	return json.Marshal(ordered)
}
