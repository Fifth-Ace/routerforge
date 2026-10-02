package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const officialScriptMaxBytes = 2 << 20

var officialScriptAllowedHosts = map[string]bool{
	"github.com":                true,
	"raw.githubusercontent.com": true,
	"git.zerrolabs.org":         true,
}

func validateOfficialScriptPlan(plan catalogInstallPlan) error {
	if plan.Method != "official-script" {
		return nil
	}

	// Preview-only entries are metadata, not execution authority. Keep legacy
	// upstream contracts visible even when they use HTTP or another source that
	// RouterForge would refuse to execute automatically.
	if plan.PreviewOnly {
		return nil
	}

	if !validOfficialScriptURL(plan.InstallerURL) {
		return fmt.Errorf("official-script requires an approved HTTPS installer URL")
	}
	if len(plan.Args) > 32 {
		return fmt.Errorf("official-script has too many arguments")
	}
	for _, arg := range plan.Args {
		if len(arg) > 256 || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("official-script contains an unsafe argument")
		}
	}
	return nil
}

func validOfficialScriptURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return false
	}
	return officialScriptAllowedHosts[strings.ToLower(u.Hostname())]
}

func officialScriptShell() (string, error) {
	for _, candidate := range []string{"/opt/bin/sh", "/bin/sh"} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	path, err := exec.LookPath("sh")
	if err != nil {
		return "", fmt.Errorf("sh interpreter not found")
	}
	return path, nil
}

func fetchOfficialScript(ctx context.Context, rawURL string) ([]byte, error) {
	if !validOfficialScriptURL(rawURL) {
		return nil, fmt.Errorf("official-script URL is not allowed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RouterForge/"+version+" official-script")

	client := &http.Client{
		Timeout: 45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if !validOfficialScriptURL(req.URL.String()) {
				return fmt.Errorf("official-script redirect left the approved HTTPS source set")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("official-script download: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, officialScriptMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("official-script download is empty")
	}
	if len(data) > officialScriptMaxBytes {
		return nil, fmt.Errorf("official-script exceeds size limit")
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return nil, fmt.Errorf("official-script contains NUL bytes")
	}
	return data, nil
}

func runOfficialScriptPlan(
	ctx context.Context,
	item catalogItem,
	action string,
	plan catalogInstallPlan,
	result *catalogActionResult,
	log *catalogActionLog,
) error {
	status := strings.ToLower(strings.TrimSpace(item.Trust.Status))
	if status != "official" && status != "verified" {
		return fmt.Errorf("official-script requires official or verified catalog trust")
	}
	if plan.PreviewOnly {
		return fmt.Errorf("preview-only official-script cannot execute")
	}
	if err := validateOfficialScriptPlan(plan); err != nil {
		return err
	}
	if err := os.MkdirAll(marketplaceDownloadDir, 0755); err != nil {
		return err
	}

	data, err := fetchOfficialScript(ctx, plan.InstallerURL)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	digestHex := hex.EncodeToString(digest[:])

	file, err := os.CreateTemp(marketplaceDownloadDir, "official-script-*.sh")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)

	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	shell, err := officialScriptShell()
	if err != nil {
		return err
	}

	fmt.Fprintf(log, "[official-script] %s %s\n", action, plan.InstallerURL)
	fmt.Fprintf(log, "[official-script] sha256=%s\n", digestHex)
	result.Sources = append(result.Sources, "official-script:"+plan.InstallerURL+"#sha256="+digestHex)

	args := append([]string{filepath.Clean(path)}, plan.Args...)
	output, runErr := runCommandStreaming(ctx, shell, args, log.EmitLine)
	if runErr != nil {
		return fmt.Errorf("official-script %s failed: %s", action, strings.TrimSpace(output))
	}
	return nil
}
