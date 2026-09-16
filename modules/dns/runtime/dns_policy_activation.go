package main

import (
	"fmt"

	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
)

type DNSPolicyRuntimeSnapshot struct {
	Identity  string `json:"identity"`
	RuleCount int    `json:"rule_count"`
}

type dnsPolicyActivationDriver interface {
	Precheck([]DNSPolicyRule) error
	Snapshot() (DNSPolicyRuntimeSnapshot, error)
	Validate([]DNSPolicyRule) error
	Apply([]DNSPolicyRule) error
	Verify([]DNSPolicyRule) error
	Rollback(DNSPolicyRuntimeSnapshot) error
	VerifyRollback(DNSPolicyRuntimeSnapshot) error
}

type DNSPolicyActivationResult struct {
	Manifest  transaction.Manifest     `json:"manifest"`
	Snapshot  DNSPolicyRuntimeSnapshot `json:"snapshot"`
	Activated bool                     `json:"activated"`
}

func runDNSPolicyActivation(txID string, rules []DNSPolicyRule, driver dnsPolicyActivationDriver) (DNSPolicyActivationResult, error) {
	manifest := transaction.New(txID, "dns-policy-router", "persisted-policy-rules")
	result := DNSPolicyActivationResult{Manifest: manifest}

	if driver == nil {
		err := fmt.Errorf("activation driver is nil")
		transaction.RecordFailure(&manifest, string(transaction.Precheck), err, nil)
		_ = transaction.Advance(&manifest, transaction.Failed)
		result.Manifest = transaction.Clone(manifest)
		return result, transaction.Wrap(err, manifest)
	}

	if err := driver.Precheck(rules); err != nil {
		return failDNSPolicyActivationBeforeApply(result, manifest, transaction.Precheck, err)
	}
	transaction.Record(&manifest, string(transaction.Precheck), transaction.EvidencePassed, "activation precheck passed", map[string]string{
		"rules": fmt.Sprintf("%d", len(rules)),
	})

	if err := transaction.Advance(&manifest, transaction.Snapshot); err != nil {
		return result, transaction.Wrap(err, manifest)
	}
	snapshot, err := driver.Snapshot()
	if err != nil {
		return failDNSPolicyActivationBeforeApply(result, manifest, transaction.Snapshot, err)
	}
	result.Snapshot = snapshot
	transaction.Record(&manifest, string(transaction.Snapshot), transaction.EvidencePassed, "pre-activation snapshot captured", map[string]string{
		"identity":   snapshot.Identity,
		"rule_count": fmt.Sprintf("%d", snapshot.RuleCount),
	})

	if err := transaction.Advance(&manifest, transaction.Validated); err != nil {
		return result, transaction.Wrap(err, manifest)
	}
	if err := driver.Validate(rules); err != nil {
		return failDNSPolicyActivationBeforeApply(result, manifest, transaction.Validated, err)
	}
	transaction.Record(&manifest, string(transaction.Validated), transaction.EvidencePassed, "activation input validated", map[string]string{
		"rules": fmt.Sprintf("%d", len(rules)),
	})

	// Enter Applied before invoking Apply. Apply may have partially changed runtime
	// state even when it returns an error; from this point every failure must pass
	// through rollback/rollback-verification or terminate as ambiguous.
	if err := transaction.Advance(&manifest, transaction.Applied); err != nil {
		return result, transaction.Wrap(err, manifest)
	}
	if err := driver.Apply(rules); err != nil {
		transaction.RecordFailure(&manifest, string(transaction.Applied), err, nil)
		return rollbackDNSPolicyActivation(result, manifest, snapshot, driver, err)
	}
	transaction.Record(&manifest, string(transaction.Applied), transaction.EvidencePassed, "activation apply completed", map[string]string{
		"rules": fmt.Sprintf("%d", len(rules)),
	})

	if err := transaction.Advance(&manifest, transaction.Verified); err != nil {
		return result, transaction.Wrap(err, manifest)
	}
	if err := driver.Verify(rules); err != nil {
		transaction.RecordFailure(&manifest, string(transaction.Verified), err, nil)
		return rollbackDNSPolicyActivation(result, manifest, snapshot, driver, err)
	}
	transaction.Record(&manifest, string(transaction.Verified), transaction.EvidencePassed, "active runtime verified", nil)

	if err := transaction.Advance(&manifest, transaction.Committed); err != nil {
		return result, transaction.Wrap(err, manifest)
	}
	transaction.Record(&manifest, string(transaction.Committed), transaction.EvidencePassed, "policy activation committed", nil)
	result.Manifest = transaction.Clone(manifest)
	result.Activated = true
	return result, nil
}

func failDNSPolicyActivationBeforeApply(
	result DNSPolicyActivationResult,
	manifest transaction.Manifest,
	stage transaction.State,
	cause error,
) (DNSPolicyActivationResult, error) {
	transaction.RecordFailure(&manifest, string(stage), cause, nil)
	if err := transaction.Advance(&manifest, transaction.Failed); err != nil {
		cause = fmt.Errorf("%w; terminal transition failed: %v", cause, err)
	}
	result.Manifest = transaction.Clone(manifest)
	result.Activated = false
	return result, transaction.Wrap(cause, manifest)
}

func rollbackDNSPolicyActivation(
	result DNSPolicyActivationResult,
	manifest transaction.Manifest,
	snapshot DNSPolicyRuntimeSnapshot,
	driver dnsPolicyActivationDriver,
	cause error,
) (DNSPolicyActivationResult, error) {
	if err := driver.Rollback(snapshot); err != nil {
		transaction.RecordFailure(&manifest, "rollback", err, map[string]string{"identity": snapshot.Identity})
		if advanceErr := transaction.Advance(&manifest, transaction.Ambiguous); advanceErr != nil {
			err = fmt.Errorf("%w; ambiguous transition failed: %v", err, advanceErr)
		}
		result.Manifest = transaction.Clone(manifest)
		result.Activated = false
		return result, transaction.Wrap(fmt.Errorf("%w; rollback failed: %v", cause, err), manifest)
	}
	transaction.Record(&manifest, "rollback", transaction.EvidenceRecovered, "pre-activation snapshot restored", map[string]string{
		"identity": snapshot.Identity,
	})

	if err := driver.VerifyRollback(snapshot); err != nil {
		transaction.RecordFailure(&manifest, "rollback-verify", err, map[string]string{"identity": snapshot.Identity})
		if advanceErr := transaction.Advance(&manifest, transaction.Ambiguous); advanceErr != nil {
			err = fmt.Errorf("%w; ambiguous transition failed: %v", err, advanceErr)
		}
		result.Manifest = transaction.Clone(manifest)
		result.Activated = false
		return result, transaction.Wrap(fmt.Errorf("%w; rollback verification failed: %v", cause, err), manifest)
	}
	transaction.Record(&manifest, "rollback-verify", transaction.EvidenceRecovered, "rollback verified", map[string]string{
		"identity": snapshot.Identity,
	})

	if err := transaction.Advance(&manifest, transaction.RolledBack); err != nil {
		result.Manifest = transaction.Clone(manifest)
		return result, transaction.Wrap(fmt.Errorf("%w; rolled-back transition failed: %v", cause, err), manifest)
	}
	result.Manifest = transaction.Clone(manifest)
	result.Activated = false
	return result, transaction.Wrap(cause, manifest)
}
