package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanLifecycleTimeout   = 90 * time.Second
	antiscanRestartTimeout     = 12 * time.Minute
	antiscanLifecycleOutputMax = 32 << 10
)

type antiscanLifecycleRequest struct {
	Action  string `json:"action"`
	Confirm string `json:"confirm"`
}

type antiscanLifecycleResult struct {
	Action        string   `json:"action"`
	BeforeRunning bool     `json:"before_running"`
	AfterRunning  bool     `json:"after_running"`
	Changed       bool     `json:"changed"`
	Verified      bool     `json:"verified"`
	MutationAPI   bool     `json:"mutation_api"`
	Output        string   `json:"output,omitempty"`
	Warnings      []string `json:"warnings"`
	Error         string   `json:"error,omitempty"`
}

func handleAntiscanLifecycle(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanLifecycleRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid lifecycle request", "mutation_api": true})
			return
		}
		action, err := normalizeAntiscanLifecycleAction(request.Action)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "mutation_api": true})
			return
		}
		expectedConfirm := strings.ToUpper(action)
		if request.Confirm != expectedConfirm {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":        fmt.Sprintf("confirm must equal %s", expectedConfirm),
				"mutation_api": true,
			})
			return
		}

		result, status, runErr := applyAntiscanLifecycle(r.Context(), cfg, action)
		if runErr != nil {
			result.Error = runErr.Error()
		}
		writeJSON(w, status, result)
	}
}

func normalizeAntiscanLifecycleAction(raw string) (string, error) {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "start", "stop", "reload", "restart":
		return action, nil
	default:
		return "", errors.New("lifecycle action must be start, stop, reload or restart")
	}
}

func applyAntiscanLifecycle(parent context.Context, cfg runtimeConfig, action string) (antiscanLifecycleResult, int, error) {
	action, err := normalizeAntiscanLifecycleAction(action)
	result := antiscanLifecycleResult{
		Action:      action,
		MutationAPI: true,
		Warnings:    []string{},
	}
	if err != nil {
		return result, http.StatusBadRequest, err
	}

	result.BeforeRunning = pathExists(cfg.StatusFile)
	result.AfterRunning = result.BeforeRunning

	if action == "start" && result.BeforeRunning {
		result.Verified = true
		result.Warnings = append(result.Warnings, "Antiscan is already running; no lifecycle command was executed.")
		return result, http.StatusOK, nil
	}
	if action == "stop" && !result.BeforeRunning {
		result.Verified = true
		result.Warnings = append(result.Warnings, "Antiscan is already stopped; no lifecycle command was executed.")
		return result, http.StatusOK, nil
	}
	if action == "reload" && !result.BeforeRunning {
		return result, http.StatusConflict, errors.New("Antiscan must be running before reload")
	}
	if err := antiscanLifecycleReady(cfg); err != nil {
		return result, http.StatusConflict, err
	}

	timeout := antiscanLifecycleTimeout
	if action == "restart" {
		timeout = antiscanRestartTimeout
		if !result.BeforeRunning {
			result.Warnings = append(result.Warnings, "Antiscan was stopped; upstream restart will start it.")
		}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	output, runErr := safety.RunCommand(ctx, antiscanLifecycleOutputMax, cfg.InitScript, action)
	result.Output = strings.TrimSpace(string(output))
	result.AfterRunning = pathExists(cfg.StatusFile)

	if runErr != nil {
		return result, http.StatusConflict, fmt.Errorf("upstream %s failed: %s", action, compactMutationOutput(output, runErr))
	}
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return result, http.StatusConflict, errors.New("upstream lifecycle command returned while an Antiscan reload lock is still present")
	}

	switch action {
	case "start":
		if !result.AfterRunning {
			return result, http.StatusConflict, errors.New("start verification failed: /tmp/ascn.run is absent")
		}
		result.Changed = !result.BeforeRunning
	case "stop":
		if result.AfterRunning {
			return result, http.StatusConflict, errors.New("stop verification failed: /tmp/ascn.run is still present")
		}
		result.Changed = result.BeforeRunning
	case "reload":
		if !result.AfterRunning {
			return result, http.StatusConflict, errors.New("reload verification failed: Antiscan stopped during reload")
		}
		result.Changed = true
	case "restart":
		if !result.AfterRunning {
			return result, http.StatusConflict, errors.New("restart verification failed: /tmp/ascn.run is absent")
		}
		result.Changed = true
	}

	result.Verified = true
	return result, http.StatusOK, nil
}

func antiscanLifecycleReady(cfg runtimeConfig) error {
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return errors.New("Antiscan reload is already in progress")
	}
	info, err := os.Stat(cfg.InitScript)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return errors.New("Antiscan init script is unavailable or not executable")
	}
	return nil
}
