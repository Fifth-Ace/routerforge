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
