package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNF4DSelectRecentDistinctWorkingMemoryOnly(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	entry := func(host string, age time.Duration) v2TargetMemoryEntry {
		return v2TargetMemoryEntry{Target: host, Protocol: benchTransportHTTPS, ResultClass: "WORKING", VerifiedCount: 2,
			InfrastructureOK: true, CleanupProven: true, LastVerified: now.Add(-age).Format(time.RFC3339)}
	}
	stale := entry("old.example", 25*time.Hour)
	failed := entry("fail.example", 1*time.Minute)
	failed.ResultClass = "FAILED"
	unproven := entry("unproven.example", 1*time.Minute)
	unproven.VerifiedCount = 1
	entries := []v2TargetMemoryEntry{
		entry("site2.example", 2*time.Minute), entry("site1.example", time.Minute), entry("site1.example", time.Minute),
		entry("changing.example", time.Second), stale, failed, unproven, entry("site3.example", 3*time.Minute),
	}
	selected := nf4dSelectRegressionCandidates(entries, "changing.example", now)
	if len(selected) != 2 || selected[0] != "site1.example" || selected[1] != "site2.example" {
		t.Fatalf("unexpected regression candidates: %#v", selected)
	}
}

func TestNF4DLegacySmartApplyHasNoGuards(t *testing.T) {
	v, err := nf4dPrepareRegressionGuards(context.Background(), "", nil, nil, nil)
	if err != nil || len(v) != 0 {
		t.Fatalf("legacy behavior changed: %v / %#v", err, v)
	}
}

func TestNF4DRejectsUnboundedOrDuplicateInputs(t *testing.T) {
	resolve := func(context.Context, string) (string, error) { return "93.184.216.34", nil }
	probe := func(context.Context, string, string, int) (v2HTTPMetrics, error) {
		return v2HTTPMetrics{TLSComplete: true, ProgressProven: true, HTTPStatus: 200}, nil
	}
	for _, names := range [][]string{{"one.example", "two.example", "three.example"}, {"one.example", "one.example"}, {"changed.example"}, {"Example.com"}} {
		if _, err := nf4dPrepareRegressionGuards(context.Background(), "changed.example", names, resolve, probe); err == nil {
			t.Errorf("accepted bad request %#v", names)
		}
	}
}

func TestNF4DPrecheckOnlyGuardsActuallyWorkingSites(t *testing.T) {
	resolve := func(_ context.Context, target string) (string, error) {
		if target == "bad.example" {
			return "10.0.0.1", nil
		}
		return "93.184.216.34", nil
	}
	probe := func(_ context.Context, host, ip string, port int) (v2HTTPMetrics, error) {
		if ip != "93.184.216.34" || port != 0 {
			t.Fatal("unexpected probe destination")
		}
		return v2HTTPMetrics{TLSComplete: true, ProgressProven: true, HTTPStatus: 403}, nil
	}
	guards, err := nf4dPrepareRegressionGuards(context.Background(), "changed.example", []string{"good.example", "bad.example"}, resolve, probe)
	if err != nil || len(guards) != 1 || guards[0].Target != "good.example" || guards[0].IPv4 != "93.184.216.34" {
		t.Fatalf("incorrect live baseline: %v %#v", err, guards)
	}
}

func TestNF4DPostVerifyDetectsRegression(t *testing.T) {
	calls := 0
	probe := func(_ context.Context, host, ip string, port int) (v2HTTPMetrics, error) {
		calls++
		if host != "good.example" || ip != "93.184.216.34" || port != 0 {
			t.Fatal("guard identity drifted")
		}
		return v2HTTPMetrics{}, errors.New("new config broke site")
	}
	err := nf4dVerifyRegressionGuards(context.Background(), []nf4dRegressionGuard{{Target: "good.example", IPv4: "93.184.216.34"}}, probe)
	if err == nil || !strings.Contains(err.Error(), "good.example") || calls != 2 {
		t.Fatalf("regression not detected: %v calls=%d", err, calls)
	}
}

func TestNF4DPostVerifyPreservesSuccessfulGuard(t *testing.T) {
	probe := func(_ context.Context, _, _ string, port int) (v2HTTPMetrics, error) {
		if port != 0 {
			t.Fatal("sandbox port unexpectedly used")
		}
		return v2HTTPMetrics{TLSComplete: true, ProgressProven: true, HTTPStatus: 200}, nil
	}
	if err := nf4dVerifyRegressionGuards(context.Background(), []nf4dRegressionGuard{{Target: "good.example", IPv4: "93.184.216.34"}}, probe); err != nil {
		t.Fatal(err)
	}
}

func TestNF4DRegressionCancellationFailsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolve := func(context.Context, string) (string, error) { t.Fatal("resolved after cancel"); return "", nil }
	probe := func(context.Context, string, string, int) (v2HTTPMetrics, error) {
		t.Fatal("probed after cancel")
		return v2HTTPMetrics{}, nil
	}
	if _, err := nf4dPrepareRegressionGuards(ctx, "changed.example", []string{"good.example"}, resolve, probe); err == nil {
		t.Fatal("canceled guard was accepted")
	}
}
