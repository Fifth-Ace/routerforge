//go:build linux

package apptransaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedHistoryArchiveDetail(t *testing.T) {
	c := newCoordinatorFixture(t)
	p := fixture()
	h, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(c.root, "completed-"+p.ID)
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(workerRequest{Plan: p, PlanSHA256: h})
	if err != nil {
		t.Fatal(err)
	}
	if err = durableExclusive(s, "request.json", b); err != nil {
		t.Fatal(err)
	}
	if err = s.save(Journal{Schema: Schema, Plan: p, PlanSHA256: h, Phase: Completed, Next: len(p.Packages), StartedAt: 100, FinishedAt: 105}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "worker.log")
	if err = os.WriteFile(path, []byte("actual worker log\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := c.UpdateHistory(20)
	if err != nil || len(rows) != 1 || rows[0].StartedAt != 100 || rows[0].FinishedAt != 105 || rows[0].Version != p.Packages[0].Version || rows[0].VerifiedCount != len(p.Packages) {
		t.Fatal(rows, err)
	}
	d, err := c.HistoryDetail(p.ID)
	if err != nil || d.Slot.Plan.ID != p.ID || !d.LogAvailable || d.Log != "actual worker log\n" || d.LogTruncated {
		t.Fatal(d, err)
	}
	if _, err = c.HistoryDetail("../outside"); err == nil {
		t.Fatal("path accepted")
	}
	if _, err = c.HistoryDetail("missing"); err == nil {
		t.Fatal("missing accepted")
	}
	if err = os.WriteFile(path, []byte(strings.Repeat("x", (1<<20)+1)), 0600); err != nil {
		t.Fatal(err)
	}
	d, err = c.HistoryDetail(p.ID)
	if err != nil || !d.LogTruncated || len(d.Log) != 1<<20 {
		t.Fatal("log limit", err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(dir, "request.json"), path); err != nil {
		t.Fatal(err)
	}
	d, err = c.HistoryDetail(p.ID)
	if err != nil || d.LogAvailable || d.Log != "" {
		t.Fatal("unsafe log", err)
	}
}

func TestUnifiedHistoryTerminalActiveAndFailedArchive(t *testing.T) {
	c := newCoordinatorFixture(t)
	p := fixture()
	hash, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	dir := c.slot()
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	req, err := json.Marshal(workerRequest{Plan: p, PlanSHA256: hash})
	if err != nil {
		t.Fatal(err)
	}
	if err = durableExclusive(s, "request.json", req); err != nil {
		t.Fatal(err)
	}
	if err = s.save(Journal{Schema: Schema, Plan: p, PlanSHA256: hash, Phase: RecoveryRequired, StartedAt: 100, FinishedAt: 105, ErrorCode: "verify_failed"}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "worker.log"), []byte("verify failed"), 0600); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireMutationLease(c.root)
	if err != nil {
		t.Fatal(err)
	}
	_, busyErr := c.HistoryDetail(p.ID)
	lease.Close()
	if busyErr == nil {
		t.Fatal("busy history accepted")
	}
	detail, err := c.HistoryDetail(p.ID)
	if err != nil || detail.Log != "verify failed" || detail.Slot.Journal.ErrorCode != "verify_failed" {
		t.Fatal(detail, err)
	}
	archive := filepath.Join(c.root, "failed-"+p.ID)
	if err = os.Rename(dir, archive); err != nil {
		t.Fatal(err)
	}
	rows, err := c.UpdateHistory(20)
	if err != nil || len(rows) != 1 || rows[0].State != string(RecoveryRequired) || rows[0].ErrorCode != "verify_failed" {
		t.Fatal(rows, err)
	}
	detail, err = c.HistoryDetail(p.ID)
	if err != nil || detail.Log != "verify failed" {
		t.Fatal(detail, err)
	}
	if b, err := os.ReadFile(filepath.Join(archive, "request.json")); err != nil || string(b) != string(req) {
		t.Fatal("history mutated", err)
	}
}
