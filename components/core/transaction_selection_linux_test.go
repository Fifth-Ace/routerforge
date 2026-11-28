//go:build linux

package main

import (
	"testing"
)

func TestCoreTransactionEligibleSelection(t *testing.T) {
	_, doc, status, items := coreTransactionFixture()
	commit, rows := coreTransactionEligibleSelection(doc, status, items)
	if commit != doc.Commit || len(rows) != 11 {
		t.Fatal(commit, rows)
	}
	for _, row := range rows {
		if row.Package == "" || row.InstalledVersion == "" || row.TargetVersion == "" || row.InstalledVersion == row.TargetVersion {
			t.Fatal(row)
		}
	}
	items[0].UpdateAvailable = false
	_, rows = coreTransactionEligibleSelection(doc, status, items)
	if len(rows) != 10 {
		t.Fatal(rows)
	}
	status.Source = "cache"
	commit, rows = coreTransactionEligibleSelection(doc, status, items)
	if commit != "" || len(rows) != 0 {
		t.Fatal(commit, rows)
	}
}
