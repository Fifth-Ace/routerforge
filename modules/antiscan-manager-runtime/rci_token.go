package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanRCITokenTimeout  = 2 * time.Minute
	antiscanRCITokenFileMax  = 64 << 10
	antiscanRCITokenValueMax = 4096
)

var antiscanRCITokenPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

type antiscanRCITokenStatus struct {
	AuthState    string `json:"auth_state"`
	TokenPresent bool   `json:"token_present"`
	KeyPresent   bool   `json:"key_present"`
	Complete     bool   `json:"complete"`
	Supported    bool   `json:"supported"`
	IgnoreToken  bool   `json:"ignore_token"`
	Running      bool   `json:"running"`
	MutationAPI  bool   `json:"mutation_api"`
}

type antiscanRCITokenRequest struct {
	Action  string `json:"action"`
	Token   string `json:"token,omitempty"`
	Confirm string `json:"confirm"`
}

type antiscanRCITokenResult struct {
	Action            string   `json:"action"`
	AuthState         string   `json:"auth_state"`
	BeforeComplete    bool     `json:"before_complete"`
	AfterComplete     bool     `json:"after_complete"`
	RunningBefore     bool     `json:"running_before"`
	RunningAfter      bool     `json:"running_after"`
	Changed           bool     `json:"changed"`
	Verified          bool     `json:"verified"`
	Checked           bool     `json:"checked"`
	Valid             bool     `json:"valid"`
	Supported         bool     `json:"supported"`
	RollbackPerformed bool     `json:"rollback_performed"`
	MutationAPI       bool     `json:"mutation_api"`
	Warnings          []string `json:"warnings"`
	Error             string   `json:"error,omitempty"`
}

type antiscanRCIFileSnapshot struct {
	Path   string
	Exists bool
	Data   []byte
	Mode   os.FileMode
}

func handleAntiscanRCITokenStatus(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, readAntiscanRCITokenStatus(cfg))
	}
}

func handleAntiscanRCITokenMutation(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanRCITokenRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, antiscanRCITokenResult{
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "invalid RCI token request",
			})
			return
		}

		action, err := normalizeAntiscanRCITokenAction(request.Action)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, antiscanRCITokenResult{
				Action:      strings.ToLower(strings.TrimSpace(request.Action)),
				MutationAPI: true,
				Warnings:    []string{},
				Error:       err.Error(),
			})
			return
		}

		expectedConfirm := map[string]string{
			"set":    "SET_TOKEN",
			"check":  "CHECK_TOKEN",
			"delete": "DELETE_TOKEN",
		}[action]
		if request.Confirm != expectedConfirm {
			writeJSON(w, http.StatusConflict, antiscanRCITokenResult{
				Action:      action,
				MutationAPI: true,
				Warnings:    []string{},
				Error:       fmt.Sprintf("confirm must equal %s", expectedConfirm),
			})
			return
		}

		result, status, applyErr := applyAntiscanRCITokenMutation(r.Context(), cfg, action, request.Token)
		if applyErr != nil {
			result.Error = applyErr.Error()
		}
		writeJSON(w, status, result)
	}
}

func normalizeAntiscanRCITokenAction(raw string) (string, error) {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "set", "check", "delete":
		return action, nil
	default:
		return "", errors.New("RCI token action must be set, check or delete")
	}
}

func normalizeAntiscanRCITokenValue(raw string) (string, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", errors.New("RCI token must not be empty")
	}
	if len(token) > antiscanRCITokenValueMax {
		return "", errors.New("RCI token exceeds RouterForge safety limit")
	}
	if !antiscanRCITokenPattern.MatchString(token) {
		return "", errors.New("RCI token must contain only ASCII letters and digits")
	}
	return token, nil
}

func readAntiscanRCITokenStatus(cfg runtimeConfig) antiscanRCITokenStatus {
	tokenPath, keyPath := antiscanRCITokenPaths(cfg)
	tokenPresent := fileNonEmpty(tokenPath)
	keyPresent := fileNonEmpty(keyPath)
	ignoreToken := pathExists("/tmp/ascn_ignore_token")
	authState := readAntiscanRCIAuthState()
	if ignoreToken {
		authState = "not_required"
	}
	return antiscanRCITokenStatus{
		AuthState:    authState,
		TokenPresent: tokenPresent,
		KeyPresent:   keyPresent,
		Complete:     tokenPresent && keyPresent,
		Supported:    !ignoreToken,
		IgnoreToken:  ignoreToken,
		Running:      pathExists(cfg.StatusFile),
		MutationAPI:  true,
	}
}

func readAntiscanRCIAuthState() string {
	switch strings.TrimSpace(string(readFile("/tmp/ascn_rci_auth"))) {
	case "1":
		return "required"
	case "0":
		return "not_required"
	default:
		return "unknown"
	}
}

func antiscanRCITokenPaths(cfg runtimeConfig) (string, string) {
	return filepath.Join(cfg.AntiscanDir, ".tkn"), filepath.Join(cfg.AntiscanDir, ".k")
}

func applyAntiscanRCITokenMutation(parent context.Context, cfg runtimeConfig, actionRaw, tokenRaw string) (antiscanRCITokenResult, int, error) {
	action, err := normalizeAntiscanRCITokenAction(actionRaw)
	statusBefore := readAntiscanRCITokenStatus(cfg)
	result := antiscanRCITokenResult{
		Action:         action,
		AuthState:      statusBefore.AuthState,
		BeforeComplete: statusBefore.Complete,
		AfterComplete:  statusBefore.Complete,
		RunningBefore:  statusBefore.Running,
		RunningAfter:   statusBefore.Running,
		Supported:      statusBefore.Supported,
		MutationAPI:    true,
		Warnings:       []string{},
	}
	if err != nil {
		return result, http.StatusBadRequest, err
	}

	if err := antiscanLifecycleReady(cfg); err != nil {
		return result, http.StatusConflict, err
	}

	tokenPath, keyPath := antiscanRCITokenPaths(cfg)
	switch action {
	case "check":
		if strings.TrimSpace(tokenRaw) != "" {
			return result, http.StatusBadRequest, errors.New("RCI token check does not accept a token value")
		}
		if !statusBefore.Supported {
			return result, http.StatusConflict, errors.New("RCI token flow is not supported by this firmware")
		}
		if !statusBefore.Complete {
			return result, http.StatusConflict, errors.New("RCI token/key pair is incomplete")
		}
		if runErr := runAntiscanRCITokenCommand(parent, cfg, "check", "", statusBefore.TokenPresent); runErr != nil {
			result.Checked = true
			result.Valid = false
			return result, http.StatusConflict, errors.New("upstream RCI token check failed")
		}
		result.Checked = true
		result.Valid = true
		result.Verified = true
		return result, http.StatusOK, nil

	case "set":
		if !statusBefore.Supported {
			return result, http.StatusConflict, errors.New("RCI token flow is not supported by this firmware")
		}
		token, normalizeErr := normalizeAntiscanRCITokenValue(tokenRaw)
		if normalizeErr != nil {
			return result, http.StatusBadRequest, normalizeErr
		}

		tokenSnapshot, snapErr := snapshotAntiscanRCIFile(tokenPath)
		if snapErr != nil {
			return result, http.StatusConflict, snapErr
		}
		keySnapshot, snapErr := snapshotAntiscanRCIFile(keyPath)
		if snapErr != nil {
			return result, http.StatusConflict, snapErr
		}

		if runErr := runAntiscanRCITokenCommand(parent, cfg, "set", token, statusBefore.TokenPresent); runErr != nil {
			rollbackErr := restoreAntiscanRCIFiles(tokenSnapshot, keySnapshot)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, errors.New("upstream RCI token set failed and encrypted token files could not be restored")
			}
			return result, http.StatusConflict, errors.New("upstream RCI token set failed; previous encrypted token state was restored")
		}

		statusAfterSet := readAntiscanRCITokenStatus(cfg)
		if !statusAfterSet.Complete {
			rollbackErr := restoreAntiscanRCIFiles(tokenSnapshot, keySnapshot)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, errors.New("RCI token set verification failed and encrypted token files could not be restored")
			}
			return result, http.StatusConflict, errors.New("RCI token set verification failed; previous encrypted token state was restored")
		}

		if runErr := runAntiscanRCITokenCommand(parent, cfg, "check", "", true); runErr != nil {
			rollbackErr := restoreAntiscanRCIFiles(tokenSnapshot, keySnapshot)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, errors.New("RCI token validation failed and encrypted token files could not be restored")
			}
			return result, http.StatusConflict, errors.New("RCI token validation failed; previous encrypted token state was restored")
		}

		statusAfter := readAntiscanRCITokenStatus(cfg)
		result.AfterComplete = statusAfter.Complete
		result.RunningAfter = statusAfter.Running
		result.AuthState = statusAfter.AuthState
		result.Changed = true
		result.Verified = true
		result.Checked = true
		result.Valid = true
		return result, http.StatusOK, nil

	case "delete":
		if strings.TrimSpace(tokenRaw) != "" {
			return result, http.StatusBadRequest, errors.New("RCI token delete does not accept a token value")
		}
		if !statusBefore.TokenPresent && !statusBefore.KeyPresent {
			result.Verified = true
			result.Warnings = append(result.Warnings, "RCI token/key files are already absent.")
			return result, http.StatusOK, nil
		}

		tokenSnapshot, snapErr := snapshotAntiscanRCIFile(tokenPath)
		if snapErr != nil {
			return result, http.StatusConflict, snapErr
		}
		keySnapshot, snapErr := snapshotAntiscanRCIFile(keyPath)
		if snapErr != nil {
			return result, http.StatusConflict, snapErr
		}

		if runErr := runAntiscanRCITokenCommand(parent, cfg, "delete", "", statusBefore.TokenPresent); runErr != nil {
			rollbackErr := restoreAntiscanRCIFiles(tokenSnapshot, keySnapshot)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, errors.New("upstream RCI token delete failed and encrypted token files could not be restored")
			}
			return result, http.StatusConflict, errors.New("upstream RCI token delete failed; encrypted token files were restored")
		}

		statusAfter := readAntiscanRCITokenStatus(cfg)
		result.AfterComplete = statusAfter.Complete
		result.RunningAfter = statusAfter.Running
		result.AuthState = statusAfter.AuthState
		if statusAfter.TokenPresent || statusAfter.KeyPresent {
			rollbackErr := restoreAntiscanRCIFiles(tokenSnapshot, keySnapshot)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, errors.New("RCI token delete verification failed and encrypted token files could not be restored")
			}
			return result, http.StatusConflict, errors.New("RCI token delete verification failed; encrypted token files were restored")
		}

		result.Changed = true
		result.Verified = true
		if result.RunningBefore && !result.RunningAfter {
			result.Warnings = append(result.Warnings, "Antiscan stopped because this firmware requires RCI authentication after token deletion.")
		}
		return result, http.StatusOK, nil
	}

	return result, http.StatusBadRequest, errors.New("unsupported RCI token action")
}

func buildAntiscanRCITokenCommand(action, token string, tokenPresent bool) ([]string, string, error) {
	switch action {
	case "set":
		normalized, err := normalizeAntiscanRCITokenValue(token)
		if err != nil {
			return nil, "", err
		}
		stdin := normalized + "\n"
		if tokenPresent {
			stdin = "Y\n" + normalized + "\n"
		}
		return []string{"token", "set"}, stdin, nil
	case "check":
		return []string{"token", "check"}, "", nil
	case "delete":
		return []string{"token", "delete"}, "Y\n", nil
	default:
		return nil, "", errors.New("unsupported RCI token command")
	}
}

func runAntiscanRCITokenCommand(parent context.Context, cfg runtimeConfig, action, token string, tokenPresent bool) error {
	args, stdin, err := buildAntiscanRCITokenCommand(action, token, tokenPresent)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, antiscanRCITokenTimeout)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, cfg.InitScript, args...)
	if err != nil {
		return err
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func snapshotAntiscanRCIFile(path string) (antiscanRCIFileSnapshot, error) {
	snapshot := antiscanRCIFileSnapshot{Path: path, Mode: 0600}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, fmt.Errorf("inspect encrypted RCI token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return snapshot, errors.New("encrypted RCI token path is not a regular file")
	}
	if info.Size() > antiscanRCITokenFileMax {
		return snapshot, errors.New("encrypted RCI token file exceeds RouterForge safety limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot, fmt.Errorf("read encrypted RCI token file: %w", err)
	}
	snapshot.Exists = true
	snapshot.Data = data
	snapshot.Mode = info.Mode().Perm()
	return snapshot, nil
}

func restoreAntiscanRCIFiles(snapshots ...antiscanRCIFileSnapshot) error {
	var restoreErrors []string
	for _, snapshot := range snapshots {
		if snapshot.Exists {
			if err := safety.WriteFileAtomic(snapshot.Path, snapshot.Data, snapshot.Mode); err != nil {
				restoreErrors = append(restoreErrors, filepath.Base(snapshot.Path)+": "+err.Error())
			}
			continue
		}
		if err := os.Remove(snapshot.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			restoreErrors = append(restoreErrors, filepath.Base(snapshot.Path)+": "+err.Error())
		}
	}
	if len(restoreErrors) > 0 {
		return errors.New(strings.Join(restoreErrors, "; "))
	}
	return nil
}
