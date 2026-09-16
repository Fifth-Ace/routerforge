package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
)

const dnsPolicyRulesSchemaVersion = 1

type DNSPolicyRulesDocument struct {
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
	Rules     []DNSPolicyRule `json:"rules"`
}

type DNSPersistedPolicyEvaluationRequest struct {
	ClientIP  string `json:"client_ip,omitempty"`
	Domain    string `json:"domain"`
	QueryType string `json:"qtype,omitempty"`
}

type DNSPolicyActivationChange struct {
	Field   string `json:"field"`
	Current string `json:"current"`
	Desired string `json:"desired"`
}

type DNSPolicyActivationPreview struct {
	DocumentVersion int                         `json:"document_version"`
	UpdatedAt       time.Time                   `json:"updated_at"`
	PersistedRules  int                         `json:"persisted_rules"`
	ActiveRules     int                         `json:"active_rules"`
	Policies        []string                    `json:"policies"`
	Ready           bool                        `json:"ready"`
	Activated       bool                        `json:"activated"`
	Changes         []DNSPolicyActivationChange `json:"changes"`
	Evidence        []string                    `json:"evidence"`
}

func buildDNSPolicyActivationPreview(doc DNSPolicyRulesDocument, rules []DNSPolicyRule) DNSPolicyActivationPreview {
	names := map[string]string{}
	for _, rule := range rules {
		names[rule.Policy] = rule.Policy
	}
	inventory := dnsPolicyInventoryFromNames(names)
	policies := make([]string, 0, len(inventory))
	for _, item := range inventory {
		policies = append(policies, item.Proxy)
	}
	sort.Strings(policies)
	if len(policies) > 1 {
		sort.SliceStable(policies, func(i, j int) bool {
			if policies[i] == "System" {
				return true
			}
			if policies[j] == "System" {
				return false
			}
			oi, iok := policyProxyOrdinal(policies[i])
			oj, jok := policyProxyOrdinal(policies[j])
			if iok && jok && oi != oj {
				return oi < oj
			}
			if iok != jok {
				return iok
			}
			return policies[i] < policies[j]
		})
	}

	return DNSPolicyActivationPreview{
		DocumentVersion: doc.Version,
		UpdatedAt:       doc.UpdatedAt,
		PersistedRules:  len(rules),
		ActiveRules:     0,
		Policies:        policies,
		Ready:           true,
		Activated:       false,
		Changes: []DNSPolicyActivationChange{
			{
				Field:   "policy_rules",
				Current: "inactive (0 active policy-router rules)",
				Desired: fmt.Sprintf("persisted (%d validated rules)", len(rules)),
			},
		},
		Evidence: []string{
			"persisted document loaded",
			fmt.Sprintf("schema version %d accepted", doc.Version),
			"rules validated against live Keenetic policy inventory",
			"runtime activation remains disabled",
		},
	}
}

type DNSPolicyActivationStageDesign struct {
	State            transaction.State `json:"state"`
	Purpose          string            `json:"purpose"`
	RequiredEvidence []string          `json:"required_evidence"`
	OnFailure        string            `json:"on_failure"`
}

type DNSPolicyActivationTransactionDesign struct {
	Component           string                           `json:"component"`
	DocumentVersion     int                              `json:"document_version"`
	PersistedRules      int                              `json:"persisted_rules"`
	ActivationAvailable bool                             `json:"activation_available"`
	MutatesRuntime      bool                             `json:"mutates_runtime"`
	Stages              []DNSPolicyActivationStageDesign `json:"stages"`
	TerminalStates      []transaction.State              `json:"terminal_states"`
	RollbackArtifacts   []string                         `json:"rollback_artifacts"`
	CommitGate          []string                         `json:"commit_gate"`
}

func buildDNSPolicyActivationTransactionDesign(doc DNSPolicyRulesDocument, rules []DNSPolicyRule) DNSPolicyActivationTransactionDesign {
	return DNSPolicyActivationTransactionDesign{
		Component:           "dns-policy-router",
		DocumentVersion:     doc.Version,
		PersistedRules:      len(rules),
		ActivationAvailable: false,
		MutatesRuntime:      false,
		Stages: []DNSPolicyActivationStageDesign{
			{
				State:   transaction.Precheck,
				Purpose: "prove the persisted rule set and live policy inventory are usable before any runtime change",
				RequiredEvidence: []string{
					"persisted document loaded",
					"schema version accepted",
					"all rules validate against current Keenetic policy inventory",
				},
				OnFailure: "failed; no runtime mutation is permitted",
			},
			{
				State:   transaction.Snapshot,
				Purpose: "capture exact pre-activation runtime policy-router state and desired persisted document identity",
				RequiredEvidence: []string{
					"pre-activation runtime snapshot captured",
					"desired persisted document identity captured",
					"rollback artifact is readable before apply",
				},
				OnFailure: "failed; no runtime mutation is permitted",
			},
			{
				State:   transaction.Validated,
				Purpose: "freeze the canonical desired rule set and activation plan after snapshot creation",
				RequiredEvidence: []string{
					"canonical validated rules frozen",
					"policy inventory rechecked after snapshot",
					"activation input is unchanged since precheck",
				},
				OnFailure: "failed; no runtime mutation is permitted",
			},
			{
				State:   transaction.Applied,
				Purpose: "future activation implementation atomically installs the staged runtime policy-router state",
				RequiredEvidence: []string{
					"apply operation returned success",
					"runtime accepted the staged policy-router configuration",
				},
				OnFailure: "attempt rollback to the exact pre-activation snapshot; unresolved state becomes ambiguous",
			},
			{
				State:   transaction.Verified,
				Purpose: "prove the active runtime state matches the staged desired configuration before commit",
				RequiredEvidence: []string{
					"active runtime identity matches staged desired identity",
					"runtime health check passes",
					"policy-router evaluation probes match the staged rule set",
				},
				OnFailure: "rollback to the exact pre-activation snapshot and verify recovery; unresolved state becomes ambiguous",
			},
			{
				State:   transaction.Committed,
				Purpose: "declare activation successful only after verification evidence is complete",
				RequiredEvidence: []string{
					"all required verification evidence passed",
					"transaction manifest is terminal and immutable",
				},
				OnFailure: "not applicable; commit is reached only after verified state",
			},
		},
		TerminalStates: []transaction.State{
			transaction.Committed,
			transaction.RolledBack,
			transaction.Ambiguous,
			transaction.Failed,
		},
		RollbackArtifacts: []string{
			"exact pre-activation runtime policy-router snapshot",
			"desired persisted policy document identity",
			"transaction evidence manifest",
		},
		CommitGate: []string{
			"active runtime identity equals staged desired identity",
			"runtime health is good",
			"policy-router evaluation probes pass",
			"no unresolved rollback or ambiguous evidence exists",
		},
	}
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
