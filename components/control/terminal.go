package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	adminTerminalCommandLimit = 4096
	adminTerminalOutputLimit  = 64 << 10
	adminTerminalTimeout      = 15 * time.Second
)

type adminTerminalRunRequest struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
	Confirm string `json:"confirm"`
}

func registerAdminTerminalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/terminal/run", mutationOnly(handleAdminTerminalRun))
}

func handleAdminTerminalRun(w http.ResponseWriter, r *http.Request) {
	var request adminTerminalRunRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid terminal request"})
		return
	}

	command := strings.TrimSpace(request.Command)
	if command == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "command is required"})
		return
	}
	if len(command) > adminTerminalCommandLimit {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":             "terminal command exceeds limit",
			"max_command_bytes": adminTerminalCommandLimit,
		})
		return
	}
	if request.Confirm != "RUN" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal RUN"})
		return
	}

	cwd := strings.TrimSpace(request.Cwd)
	if cwd == "" {
		cwd = "/opt"
	}
	resolved, err := resolveExistingAdminFilePath(cwd)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	info, err := os.Stat(resolved.Canonical)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "terminal cwd is not a directory"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), adminTerminalTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", command)
	cmd.Dir = resolved.Canonical
	output, runErr := cmd.CombinedOutput()
	truncated := false
	if len(output) > adminTerminalOutputLimit {
		output = output[:adminTerminalOutputLimit]
		truncated = true
	}

	exitCode := 0
	if runErr != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	if timedOut {
		exitCode = 124
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               runErr == nil && !timedOut,
		"command":          command,
		"cwd":              resolved.Lexical,
		"output":           string(output),
		"exit_code":        exitCode,
		"timed_out":        timedOut,
		"output_truncated": truncated,
		"timeout_seconds":  int(adminTerminalTimeout / time.Second),
	})
}
