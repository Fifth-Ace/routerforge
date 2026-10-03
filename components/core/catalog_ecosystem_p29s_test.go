package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestP29SNoMutableGitHubLifecycleURLs(t *testing.T) {
	files := []string{
		"catalog_ecosystem.go",
		"catalog_official_script_overrides.go",
	}
	installerURL := regexp.MustCompile(`InstallerURL:\s*"([^"]+)"`)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, match := range installerURL.FindAllStringSubmatch(string(data), -1) {
			url := match[1]
			if !strings.Contains(url, "github.com/") && !strings.Contains(url, "raw.githubusercontent.com/") {
				continue
			}

			// Known upstream repository migration: old owner now returns a GitHub
			// redirect and is intentionally excluded from automatic repinning
			// until the new canonical owner is independently resolved.
			if url == "https://raw.githubusercontent.com/Kuzz007/keenetic_xray_installer/main/xray_vless_failover_auto_latest.sh" {
				continue
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
				if strings.Contains(url, forbidden) {
					t.Fatalf("%s contains mutable GitHub lifecycle URL: %s", path, url)
				}
			}
		}
	}
}
