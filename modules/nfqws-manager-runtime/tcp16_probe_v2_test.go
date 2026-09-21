package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTCP16SelectTargetsDefaultsAreBounded(t *testing.T) {
	got, err := v2TCP16SelectTargets(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || len(got) > v2TCP16MaxTargets {
		t.Fatalf("default target count=%d", len(got))
	}
}

func TestTCP16SelectTargetsRejectsUnknown(t *testing.T) {
	if _, err := v2TCP16SelectTargets([]string{"NOPE"}); err == nil {
		t.Fatal("unknown target was accepted")
	}
}

func TestTCP16SNICandidatesNormalizeAndDeduplicate(t *testing.T) {
	got, err := v2TCP16NormalizeSNICandidates([]string{"VK.com", "vk.com", "300.ya.ru"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "vk.com" || got[1] != "300.ya.ru" {
		t.Fatalf("candidates=%v", got)
	}
}

func TestTCP16FinishFailureRequiresThreshold(t *testing.T) {
	target := v2TCP16ProbeTarget{ID: "x", IP: "192.0.2.1", Port: 443}
	early := v2TCP16FinishFailure(v2TCP16ProbeAttempt{Target: target}, 2, 7, errTestTCP16)
	if early.Detected {
		t.Fatal("early failure was misclassified as tcp16")
	}
	late := v2TCP16FinishFailure(v2TCP16ProbeAttempt{Target: target}, 4, 15, errTestTCP16)
	if !late.Detected || !late.Alive || late.DiedAtKB != 15 {
		t.Fatalf("late=%+v", late)
	}
}

var errTestTCP16 = &tcp16TestError{}

type tcp16TestError struct{}

func (*tcp16TestError) Error() string { return "test" }

func TestTCP16AttemptPassesSNIRequiresFullCompletion(t *testing.T) {
	if v2TCP16AttemptPassesSNI(v2TCP16ProbeAttempt{Alive: true, Completed: false}) {
		t.Fatal("partial attempt passed SNI gate")
	}
	if !v2TCP16AttemptPassesSNI(v2TCP16ProbeAttempt{Alive: true, Completed: true}) {
		t.Fatal("completed clear attempt did not pass SNI gate")
	}
}

func TestTCP16ProbeResultPersistsProviderAndRevalidatedSNI(t *testing.T) {
	oldPath := v2TCP16MemoryPath
	oldNow := v2TCP16MemoryNow
	oldRoot := v2TargetMemoryRoot
	t.Cleanup(func() {
		v2TCP16MemoryPath = oldPath
		v2TCP16MemoryNow = oldNow
		v2TargetMemoryRoot = oldRoot
	})
	root := t.TempDir()
	v2TargetMemoryRoot = root
	v2TCP16MemoryPath = filepath.Join(root, "tcp16-network-memory.json")
	v2TCP16MemoryNow = func() time.Time {
		return time.Date(2026, 9, 21, 10, 30, 0, 0, time.UTC)
	}

	target := v2TCP16ProbeTarget{
		ID: "HE-01", ASN: 24940, Provider: "Hetzner",
		IP: "91.98.156.82", Port: 443,
	}
	baseline := v2TCP16ProbeAttempt{Target: target, Alive: true, Detected: true, DiedAtKB: 15}
	if err := v2RecordTCP16ProbeResult(target, baseline, "300.ya.ru"); err != nil {
		t.Fatal(err)
	}
	doc, err := readV2TCP16Memory()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 1 {
		t.Fatalf("entries=%d", len(doc.Entries))
	}
	got := doc.Entries[0]
	if got.ASN != 24940 || got.Provider != "Hetzner" || got.SuspectedCount != 1 {
		t.Fatalf("entry=%+v", got)
	}
	if got.WorkingSNI != "300.ya.ru" || !strings.Contains(got.WorkingSNISource, "revalidated") {
		t.Fatalf("working SNI not persisted as revalidated: %+v", got)
	}
}

func TestTCP16ProbeSummary(t *testing.T) {
	run := v2TCP16ProbeRun{
		Target:     v2TCP16ProbeTarget{ID: "HE-01"},
		Baseline:   v2TCP16ProbeAttempt{Alive: true, Detected: true},
		WorkingSNI: "300.ya.ru",
	}
	if got := v2TCP16ProbeSummary(run); got != "HE-01:detected:sni=300.ya.ru" {
		t.Fatalf("summary=%q", got)
	}
}
