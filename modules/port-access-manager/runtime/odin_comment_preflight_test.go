package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOdinCommentPreflight(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "matches")
	for _, tc := range []struct {
		name      string
		contents  string
		wantError bool
	}{
		{"loaded", "recent\ncomment\nstate\n", false},
		{"missing", "recent\nstate\n", true},
		{"not_substring", "another_comment\nrecent\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(fixture, []byte(tc.contents), 0600); err != nil {
				t.Fatal(err)
			}
			err := odinRequireComment(fixture)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, wantError=%v", err, tc.wantError)
			}
			if tc.wantError && !strings.Contains(err.Error(), "xt_comment") {
				t.Fatalf("missing actionable message: %v", err)
			}
		})
	}
	if err := odinRequireComment(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("unreadable kernel registry must fail closed")
	}
}