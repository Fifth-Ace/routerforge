package dnsrouter

// RT-C4: isolated in-memory NDMC fixture and real filesystem ledger/config.
// These are backend restart-contract tests, NOT opkg install/upgrade tests.
// Never contact a real router or run ndmc.

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

// restartDurableO2 emulates a new backend process reading the exact same
// on-disk config and ownership ledger. The fake NDMC inventory is deliberately
// shared, and no mutation is allowed during construction.
func restartDurableO2(previous *DurableManagerO2) *DurableManagerO2 {
	next := *previous
	next.Store = NewConfigStore(previous.Store.path)
	next.Ledger = NewLedgerStoreO(previous.Ledger.path)
	next.Ready = func(rev, listen string) bool {
		_, current, err := next.Store.Load()
		return err == nil && current == rev && listen == "192.168.10.1:55358"
	}
	return &next
}

func TestRTC4OfflineRestartStaysOfflineAndPreservesRevision(t *testing.T) {
	manager, ndmc := newDurableO2(t)
	_, before, err := manager.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	next := restartDurableO2(manager)
	_, after, err := next.Store.Load()
	if err != nil || after != before {
		t.Fatalf("config revision changed on restart: %q -> %q / %v", before, after, err)
	}
	ledger, _, err := next.Ledger.Load()
	if err != nil || ledger != nil {
		t.Fatalf("offline restart unexpectedly owns DNS: %#v / %v", ledger, err)
	}
	if len(ndmc.zones) != 0 {
		t.Fatalf("offline restart added zones: %#v", ndmc.zones)
	}
	// Mount/read-only backend GET must not activate a listener or manufacture ownership.
	api := NewAPI(next.Store, "").Handler()
	for _, url := range []string{"/v1/health", "/v1/config", "/v1/shadow/status", "/v1/status"} {
		resp := httptest.NewRecorder()
		api.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))
		if resp.Code != 200 {
			t.Fatalf("GET %s: %d %s", url, resp.Code, resp.Body.String())
		}
	}
	if len(ndmc.zones) != 0 {
		t.Fatal("read-only mount changed DNS")
	}
	if result, err := next.Disable(context.Background(), after, ""); err == nil || result.DurableChange {
		t.Fatalf("without ownership disable must refuse: %+v / %v", result, err)
	}
	ledger, _, err = next.Ledger.Load()
	if err != nil || ledger != nil {
		t.Fatal("failed disable introduced ownership")
	}
}

func TestRTC4ActiveRestartPreservesOwnedLedgerAndCanRestoreBaseline(t *testing.T) {
	manager, ndmc := newDurableO2(t)
	ctx := context.Background()
	rev := revisionO2(t, manager)
	plan, err := manager.Prepare(ctx, rev)
	if err != nil || !plan.Ready {
		t.Fatalf("prepare: %+v / %v", plan, err)
	}
	applied, err := manager.Apply(ctx, rev, plan.Token)
	if err != nil || applied.State != "active" || !applied.WatchdogArmed {
		t.Fatalf("apply: %+v / %v", applied, err)
	}
	before, expectedLedgerSHA, err := manager.Ledger.Load()
	if err != nil || before == nil || before.State != "active" {
		t.Fatalf("ledger: %+v / %v", before, err)
	}
	next := restartDurableO2(manager)
	_, actualRev, err := next.Store.Load()
	if err != nil || actualRev != rev {
		t.Fatalf("restart changed revision: %v %s", err, actualRev)
	}
	after, sha, err := next.Ledger.Load()
	if err != nil || after == nil || after.State != "active" || sha != expectedLedgerSHA {
		t.Fatalf("ownership ledger not preserved: %+v %s / %v", after, sha, err)
	}
	if !ndmc.zones["alpha.dpr2n.test"] || !ndmc.zones["beta.dpr2n.test"] {
		t.Fatalf("owned zones lost on restart: %+v", ndmc.zones)
	}
	if _, err := next.Prepare(ctx, rev); err == nil {
		t.Fatal("active ownership was incorrectly treated as unowned")
	}
	if _, err := next.Disable(ctx, rev, "sha256:stale"); err == nil {
		t.Fatal("stale ledger revision accepted")
	}
	after, sha, err = next.Ledger.Load()
	if err != nil || after == nil || after.State != "active" || sha != expectedLedgerSHA {
		t.Fatal("stale attempt modified ledger")
	}
	restored, err := next.Disable(ctx, rev, sha)
	if err != nil || restored.State != "disabled" || !restored.BaselineRestored {
		t.Fatalf("disable: %+v / %v", restored, err)
	}
	after, _, err = next.Ledger.Load()
	if err != nil || after != nil || len(ndmc.zones) != 0 {
		t.Fatalf("baseline not restored: %#v / %v / %+v", after, err, ndmc.zones)
	}
}

func TestRTC4RestartForeignDriftRequiresManualReview(t *testing.T) {
	manager, ndmc := newDurableO2(t)
	ctx := context.Background()
	rev := revisionO2(t, manager)
	plan, err := manager.Prepare(ctx, rev)
	if err != nil || !plan.Ready {
		t.Fatal(err)
	}
	applied, err := manager.Apply(ctx, rev, plan.Token)
	if err != nil || applied.State != "active" {
		t.Fatalf("apply: %+v / %v", applied, err)
	}
	next := restartDurableO2(manager)
	_, originalLedgerSHA, err := next.Ledger.Load()
	if err != nil {
		t.Fatal(err)
	}
	ndmc.foreign = true // same domain, different DNS owner: never remove that entry.
	restored, err := next.Disable(ctx, rev, originalLedgerSHA)
	if err == nil || !strings.Contains(err.Error(), "zone") || restored.BaselineRestored {
		t.Fatalf("foreign DNS ownership must hard-fail without restoration: %+v / %v", restored, err)
	}
	ledger, sha, err := next.Ledger.Load()
	if err != nil || ledger == nil || sha != originalLedgerSHA || ledger.State != "active" {
		t.Fatalf("foreign drift silently removed ownership ledger: %+v / %v", ledger, err)
	}
	if !ndmc.zones["alpha.dpr2n.test"] {
		t.Fatal("foreign DNS zone was removed")
	}
}
