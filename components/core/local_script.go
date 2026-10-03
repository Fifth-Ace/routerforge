package main

import (
	"context"
	"fmt"
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

	info, err := os.Lstat(plan.ScriptPath)
	if err != nil {
		return fmt.Errorf("local-script unavailable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("local-script refuses symlinks")
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("local-script path is not a regular file")
	}
	if info.Size() > officialScriptMaxBytes {
		return fmt.Errorf("local-script exceeds size limit")
	}

	shell, err := officialScriptShell()
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "[local-script] %s %s\n", action, plan.ScriptPath)
	result.Sources = append(result.Sources, "local-script:"+plan.ScriptPath)

	args := append([]string{plan.ScriptPath}, plan.Args...)
	output, runErr := runCommandStreaming(ctx, shell, args, log.EmitLine)
	if runErr != nil {
		return fmt.Errorf("local-script %s failed: %s", action, strings.TrimSpace(output))
	}
	return nil
}
