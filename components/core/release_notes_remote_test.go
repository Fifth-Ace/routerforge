package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestNotesIndexRejectsUntrustedAndCrossChannelAssets(t *testing.T) {
	hash := sha256.Sum256([]byte("release notes"))
	goodHash := hex.EncodeToString(hash[:])
	cases := []struct {
		name  string
		data  string
		valid bool
	}{
		{"good", fmt.Sprintf(`{"schema_version":2,"channel":"beta","releases":[{"version":"0.15.0-beta.4","title":"Beta 4","summary":{"asset":"routerforge-notes-beta-0.15.0-beta.4-summary.md","sha256":"%s"}}]}`, goodHash), true},
		{"bad asset", fmt.Sprintf(`{"schema_version":2,"channel":"beta","releases":[{"version":"0.15.0-beta.4","summary":{"asset":"../../injected.md","sha256":"%s"}}]}`, goodHash), false},
		{"bad hash", `{"schema_version":2,"channel":"beta","releases":[{"version":"0.15.0-beta.4","summary":{"asset":"routerforge-notes-beta-0.15.0-beta.4-summary.md","sha256":"bad"}}]}`, false},
		{"stable in beta", fmt.Sprintf(`{"schema_version":2,"channel":"beta","releases":[{"version":"0.14.0","summary":{"asset":"routerforge-notes-beta-0.14.0-summary.md","sha256":"%s"}}]}`, goodHash), false},
		{"duplicate", fmt.Sprintf(`{"schema_version":2,"channel":"beta","releases":[{"version":"0.15.0-beta.4","summary":{"asset":"routerforge-notes-beta-0.15.0-beta.4-summary.md","sha256":"%s"}},{"version":"0.15.0-beta.4","summary":{"asset":"routerforge-notes-beta-0.15.0-beta.4-summary.md","sha256":"%s"}}]}`, goodHash, goodHash), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseNotesIndex([]byte(tc.data), "beta")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t err=%v", tc.valid, err)
			}
		})
	}
}
