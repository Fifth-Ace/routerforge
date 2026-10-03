package main

import (
	"os"
	"strings"
	"testing"
)

func TestP29SNoMutableGitHubLifecycleURLs(t *testing.T) {
	files := []string{
		"catalog_ecosystem.go",
		"catalog_official_script_overrides.go",
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)

		for _, marker := range []string{
			"raw.githubusercontent.com/",
			"github.com/",
		} {
			if !strings.Contains(text, marker) {
				continue
			}
		}

		for _, forbidden := range []string{
			"/main/",
			"/master/",
			"/develop/",
			"/stable/",
			"/refs/heads/main/",
			"/refs/heads/master/",
			"/refs/heads/develop/",
		} {
			if !strings.Contains(text, forbidden) {
				continue
			}

			// Known upstream repository migration: old owner now returns a GitHub
			// redirect and is intentionally excluded from automatic repinning
			// until the new canonical owner is independently resolved.
			remaining := strings.ReplaceAll(
				text,
				"https://raw.githubusercontent.com/Kuzz007/keenetic_xray_installer/main/xray_vless_failover_auto_latest.sh",
				"",
			)
			if strings.Contains(remaining, forbidden) {
				t.Fatalf("%s still contains mutable GitHub lifecycle ref %q", path, forbidden)
			}
		}
	}
}
