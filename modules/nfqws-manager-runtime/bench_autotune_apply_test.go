package main

import (
	"strings"
	"testing"
	"time"
)

func TestValidateBenchAutoTuneApplyRequest(t *testing.T) {
	good := benchAutoTuneApplyRequest{
		ReceiptToken:            "rf-test",
		ExpectedActiveSHA256:    strings.Repeat("a", 64),
		ExpectedCandidateSHA256: strings.Repeat("b", 64),
		Confirm:                 benchAutoTuneApplyConfirm,
	}
	if err := validateBenchAutoTuneApplyRequest(good); err != nil {
		t.Fatalf("good request rejected: %v", err)
	}
	bad := good
	bad.Confirm = "WRONG"
	if err := validateBenchAutoTuneApplyRequest(bad); err == nil {
		t.Fatal("wrong confirmation accepted")
	}
	bad = good
	bad.ReceiptToken = ""
	if err := validateBenchAutoTuneApplyRequest(bad); err == nil {
		t.Fatal("empty receipt token accepted")
	}
}

func TestStoreBenchAutoTuneApplyReceiptPreservesPreviewIdentity(t *testing.T) {
	clearBenchAutoTuneApplyReceipt()
	defer clearBenchAutoTuneApplyReceipt()

	plan := &benchAutoTuneApplyPlan{
		Token:              "rf-receipt-test",
		ExpiresAt:          time.Now().UTC().Add(time.Minute),
		ConfigSHA256:       strings.Repeat("a", 64),
		ServerName:         "www.example.com",
		SourceProfileIndex: 2,
	}
	candidate := "NFQWS_ARGS_CUSTOM=\"--hostlist-domains=www.example.com\"\n"
	candidateSHA := smartApplySHA256([]byte(candidate))
	if err := storeBenchAutoTuneApplyReceipt(plan, candidate, candidateSHA); err != nil {
		t.Fatal(err)
	}

	benchAutoTuneApplyReceiptState.Lock()
	got := *benchAutoTuneApplyReceiptState.receipt
	benchAutoTuneApplyReceiptState.Unlock()

	if got.Token != plan.Token ||
		got.ActiveConfigSHA256 != plan.ConfigSHA256 ||
		got.CandidateSHA256 != candidateSHA ||
		got.CandidateConfig != candidate ||
		got.ServerName != plan.ServerName ||
		got.SourceProfileIndex != plan.SourceProfileIndex {
		t.Fatalf("receipt identity mismatch: %+v", got)
	}
}
