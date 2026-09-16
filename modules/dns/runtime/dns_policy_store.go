package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const dnsPolicyRulesSchemaVersion = 1

type DNSPolicyRulesDocument struct {
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
	Rules     []DNSPolicyRule `json:"rules"`
}

type dnsPolicyStore struct {
	path string
}

func newDNSPolicyStore(path string) *dnsPolicyStore {
	return &dnsPolicyStore{path: path}
}

func (s *dnsPolicyStore) Load() (DNSPolicyRulesDocument, error) {
	if s == nil || s.path == "" {
		return DNSPolicyRulesDocument{}, errors.New("policy store path is empty")
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return DNSPolicyRulesDocument{
			Version: dnsPolicyRulesSchemaVersion,
			Rules:   []DNSPolicyRule{},
		}, nil
	}
	if err != nil {
		return DNSPolicyRulesDocument{}, fmt.Errorf("read policy rules: %w", err)
	}
	var doc DNSPolicyRulesDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return DNSPolicyRulesDocument{}, fmt.Errorf("decode policy rules: %w", err)
	}
	if doc.Version != dnsPolicyRulesSchemaVersion {
		return DNSPolicyRulesDocument{}, fmt.Errorf("unsupported policy rules schema version %d", doc.Version)
	}
	if doc.Rules == nil {
		doc.Rules = []DNSPolicyRule{}
	}
	return doc, nil
}

func (s *dnsPolicyStore) Save(rules []DNSPolicyRule) (DNSPolicyRulesDocument, error) {
	if s == nil || s.path == "" {
		return DNSPolicyRulesDocument{}, errors.New("policy store path is empty")
	}
	doc := DNSPolicyRulesDocument{
		Version:   dnsPolicyRulesSchemaVersion,
		UpdatedAt: time.Now().UTC(),
		Rules:     append([]DNSPolicyRule(nil), rules...),
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return DNSPolicyRulesDocument{}, fmt.Errorf("encode policy rules: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return DNSPolicyRulesDocument{}, fmt.Errorf("create policy store directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".dns-policy-rules-*.tmp")
	if err != nil {
		return DNSPolicyRulesDocument{}, fmt.Errorf("create policy rules temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0600); err != nil {
		cleanup()
		return DNSPolicyRulesDocument{}, fmt.Errorf("chmod policy rules temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return DNSPolicyRulesDocument{}, fmt.Errorf("write policy rules temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return DNSPolicyRulesDocument{}, fmt.Errorf("sync policy rules temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return DNSPolicyRulesDocument{}, fmt.Errorf("close policy rules temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return DNSPolicyRulesDocument{}, fmt.Errorf("replace policy rules: %w", err)
	}
	return doc, nil
}
