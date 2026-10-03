package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func validateLocalScriptPlan(plan catalogInstallPlan) error {
	if plan.Method != "local-script" {
		return nil
	}
	path := strings.TrimSpace(plan.ScriptPath)
	if path == "" || strings.ContainsAny(path, "\x00\r\n") {
		return fmt.Errorf("local-script requires a safe script_path")
	}
	clean := filepath.Clean(path)
	if clean != path || (!strings.HasPrefix(clean, "/opt/etc/") && !strings.HasPrefix(clean, "/opt/share/")) {
		return fmt.Errorf("local-script path must be a canonical file under /opt/etc or /opt/share")
	}
	if len(plan.Args) > 32 {
		return fmt.Errorf("local-script has too many arguments")
	}
	for _, arg := range plan.Args {
		if len(arg) > 256 || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("local-script contains an unsafe argument")
		}
	}
	return nil
}

func readStableLocalScript(path string, maxBytes int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("local-script unavailable: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("local-script refuses symlinks")
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("local-script path is not a regular file")
	}
	if before.Size() > maxBytes {
		return nil, fmt.Errorf("local-script exceeds size limit")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("local-script open failed: %w", err)
	}
	defer file.Close()

	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("local-script stat failed: %w", err)
	}
	if !os.SameFile(before, opened) {
		return nil, fmt.Errorf("local-script changed while opening")
	}
	if !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("local-script path is not a regular file")
	}

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("local-script read failed: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("local-script is empty")
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("local-script exceeds size limit")
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return nil, fmt.Errorf("local-script contains NUL bytes")
	}
	return data, nil
}

func snapshotLocalScript(data []byte) (string, error) {
	if err := os.MkdirAll(marketplaceDownloadDir, 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(marketplaceDownloadDir, "local-script-*.sh")
	if err != nil {
		return "", err
	}
	path := file.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func runLocalScriptPlan(
	ctx context.Context,
	item catalogItem,
	action string,
	plan catalogInstallPlan,
	result *catalogActionResult,
	log *catalogActionLog,
) error {
	status := strings.ToLower(strings.TrimSpace(item.Trust.Status))
	if status != "official" && status != "verified" {
		return fmt.Errorf("local-script requires official or verified catalog trust")
	}
	if plan.PreviewOnly {
		return fmt.Errorf("preview-only local-script cannot execute")
	}
	if action != "update" && action != "remove" {
		return fmt.Errorf("local-script does not implement %q", action)
	}
	if err := validateLocalScriptPlan(plan); err != nil {
		return err
	}

	data, err := readStableLocalScript(plan.ScriptPath, officialScriptMaxBytes)
	if err != nil {
		return err
	}
	snapshot, err := snapshotLocalScript(data)
	if err != nil {
		return err
	}
	defer os.Remove(snapshot)

	shell, err := officialScriptShell()
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	digestHex := hex.EncodeToString(digest[:])
	fmt.Fprintf(log, "[local-script] %s %s\n", action, plan.ScriptPath)
	fmt.Fprintf(log, "[local-script] snapshot-sha256=%s\n", digestHex)
	result.Sources = append(result.Sources, "local-script:"+plan.ScriptPath+"#sha256="+digestHex)

	args := append([]string{snapshot}, plan.Args...)
	output, runErr := runCommandStreaming(ctx, shell, args, log.EmitLine)
	if runErr != nil {
		return fmt.Errorf("local-script %s failed: %s", action, strings.TrimSpace(output))
	}
	return nil
}
