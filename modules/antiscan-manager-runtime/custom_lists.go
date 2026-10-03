package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type antiscanCustomListPage struct {
	GeneratedAt    time.Time `json:"generated_at"`
	List           string    `json:"list"`
	Set            string    `json:"set"`
	Configured     bool      `json:"configured"`
	Running        bool      `json:"running"`
	Active         bool      `json:"active"`
	RuntimeExists  bool      `json:"runtime_exists"`
	Count          int       `json:"count"`
	SkippedInvalid int       `json:"skipped_invalid"`
	Entries        []string  `json:"entries"`
	MutationAPI    bool      `json:"mutation_api"`
	Warnings       []string  `json:"warnings"`
	Error          string    `json:"error,omitempty"`
}

type antiscanCustomListRequest struct {
	Action  string `json:"action"`
	List    string `json:"list"`
	Entry   string `json:"entry,omitempty"`
	Confirm string `json:"confirm"`
}

type antiscanCustomListResult struct {
	Action            string   `json:"action"`
	List              string   `json:"list"`
	Set               string   `json:"set"`
	Entry             string   `json:"entry,omitempty"`
	Changed           bool     `json:"changed"`
	Verified          bool     `json:"verified"`
	Configured        bool     `json:"configured"`
	Active            bool     `json:"active"`
	Reloaded          bool     `json:"reloaded"`
	RollbackPerformed bool     `json:"rollback_performed"`
	RestartRequired   bool     `json:"restart_required"`
	CountBefore       int      `json:"count_before"`
	CountAfter        int      `json:"count_after"`
	MutationAPI       bool     `json:"mutation_api"`
	Warnings          []string `json:"warnings"`
	Error             string   `json:"error,omitempty"`
}

func handleAntiscanCustomListRead(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, status := readAntiscanManagedCustomList(r.Context(), cfg, r.URL.Query().Get("list"))
		writeJSON(w, status, page)
	}
}

func handleAntiscanCustomListMutation(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanCustomListRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, antiscanCustomListResult{
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "invalid custom-list request",
			})
			return
		}

		action := strings.ToLower(strings.TrimSpace(request.Action))
		expectedConfirm := strings.ToUpper(action)
		if action != "add" && action != "delete" && action != "clear" && action != "reload" {
			writeJSON(w, http.StatusBadRequest, antiscanCustomListResult{
				Action:      action,
				List:        strings.ToLower(strings.TrimSpace(request.List)),
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "custom list action must be add, delete, clear or reload",
			})
			return
		}
		if request.Confirm != expectedConfirm {
			writeJSON(w, http.StatusConflict, antiscanCustomListResult{
				Action:      action,
				List:        strings.ToLower(strings.TrimSpace(request.List)),
				MutationAPI: true,
				Warnings:    []string{},
				Error:       fmt.Sprintf("confirm must equal %s", expectedConfirm),
			})
			return
		}

		result, status, err := applyAntiscanCustomListMutation(r.Context(), cfg, action, request.List, request.Entry)
		if err != nil {
			result.Error = err.Error()
		}
		writeJSON(w, status, result)
	}
}

func readAntiscanManagedCustomList(parent context.Context, cfg runtimeConfig, rawList string) (antiscanCustomListPage, int) {
	listName := strings.ToLower(strings.TrimSpace(rawList))
	page := antiscanCustomListPage{
		GeneratedAt: time.Now().UTC(),
		List:        listName,
		Entries:     []string{},
		MutationAPI: true,
		Warnings:    []string{},
	}
	fileName, setName, err := antiscanManagedCustomListTarget(listName)
	if err != nil {
		page.Error = err.Error()
		return page, http.StatusBadRequest
	}
	page.Set = setName

	config, err := readAntiscanConfig(cfg.AntiscanDir + "/ascn.conf")
	if err != nil {
		page.Error = "read Antiscan config: " + err.Error()
		return page, http.StatusConflict
	}
	page.Configured = antiscanManagedCustomListConfigured(config, listName)
	page.Running = pathExists(cfg.StatusFile)
	page.Active = page.Configured && page.Running

	_, data, _, err := readAntiscanCustomList(cfg, fileName)
	if err != nil {
		page.Error = err.Error()
		return page, http.StatusConflict
	}
	entries, skipped, err := parseAntiscanManagedCustomListEntries(data)
	if err != nil {
		page.Error = err.Error()
		return page, http.StatusConflict
	}
	page.Entries = entries
	page.Count = len(entries)
	page.SkippedInvalid = skipped
	if skipped > 0 {
		page.Warnings = append(page.Warnings, fmt.Sprintf("Ignored %d invalid custom-list lines.", skipped))
	}

	if page.Active {
		binary := findAntiscanIPSetBinary()
		if binary == "" {
			page.Warnings = append(page.Warnings, "ipset binary is unavailable; runtime list state could not be verified.")
			return page, http.StatusOK
		}
		exists, inspectErr := antiscanSetExists(parent, binary, setName)
		if inspectErr != nil {
			page.Warnings = append(page.Warnings, "runtime list state could not be verified: "+inspectErr.Error())
			return page, http.StatusOK
		}
		page.RuntimeExists = exists
	}
	return page, http.StatusOK
}

func applyAntiscanCustomListMutation(parent context.Context, cfg runtimeConfig, actionRaw, listRaw, entryRaw string) (antiscanCustomListResult, int, error) {
	action := strings.ToLower(strings.TrimSpace(actionRaw))
	listName := strings.ToLower(strings.TrimSpace(listRaw))
	result := antiscanCustomListResult{
		Action:      action,
		List:        listName,
		MutationAPI: true,
		Warnings:    []string{},
	}

	fileName, setName, err := antiscanManagedCustomListTarget(listName)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	result.Set = setName

	if pathExists(cfg.ConfigLockFile) {
		return result, http.StatusConflict, errors.New("Antiscan config reload is in progress")
	}
	config, err := readAntiscanConfig(cfg.AntiscanDir + "/ascn.conf")
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("read Antiscan config: %w", err)
	}
	result.Configured = antiscanManagedCustomListConfigured(config, listName)
	result.Active = result.Configured && pathExists(cfg.StatusFile)

	listPath, original, mode, err := readAntiscanCustomList(cfg, fileName)
	if err != nil {
		return result, http.StatusConflict, err
	}
	beforeEntries, _, err := parseAntiscanManagedCustomListEntries(original)
	if err != nil {
		return result, http.StatusConflict, err
	}
	result.CountBefore = len(beforeEntries)
	result.CountAfter = result.CountBefore

	if action == "reload" {
		if strings.TrimSpace(entryRaw) != "" {
			return result, http.StatusBadRequest, errors.New("reload does not accept an entry")
		}
		if !result.Configured {
			result.Verified = true
			result.Warnings = append(result.Warnings, "Custom list is not configured; there is nothing to reload.")
			return result, http.StatusOK, nil
		}
		if !pathExists(cfg.StatusFile) {
			result.Verified = true
			result.Warnings = append(result.Warnings, "Antiscan is stopped; reload was not executed.")
			return result, http.StatusOK, nil
		}
		if listName == "whitelist" && len(beforeEntries) == 0 {
			return result, http.StatusConflict, errors.New("empty active custom whitelist reload is blocked to prevent an empty-whitelist lockout")
		}
		if err := reloadAntiscanCustomLists(parent, cfg); err != nil {
			return result, http.StatusConflict, err
		}
		result.Reloaded = true
		result.Changed = true
		if err := verifyAntiscanManagedCustomRuntime(parent, setName, beforeEntries, "", true); err != nil {
			return result, http.StatusConflict, err
		}
		result.Verified = true
		return result, http.StatusOK, nil
	}

	var entry string
	if action == "add" || action == "delete" {
		entry, err = normalizeAntiscanListEntry(entryRaw)
		if err != nil {
			return result, http.StatusBadRequest, err
		}
		result.Entry = entry
	} else if action != "clear" {
		return result, http.StatusBadRequest, errors.New("custom list action must be add, delete, clear or reload")
	}

	var updated []byte
	var changed bool
	switch action {
	case "add":
		updated, changed, err = appendAntiscanListEntry(original, entry)
		if err != nil {
			return result, http.StatusBadRequest, err
		}
		if !changed {
			result.Verified = true
			result.Warnings = append(result.Warnings, "Entry already exists in the custom list.")
			return result, http.StatusOK, nil
		}
	case "delete":
		updated, changed, err = removeAntiscanManagedCustomListEntry(original, entry)
		if err != nil {
			return result, http.StatusBadRequest, err
		}
		if !changed {
			result.Verified = true
			result.Warnings = append(result.Warnings, "Entry was already absent from the custom list.")
			return result, http.StatusOK, nil
		}
	case "clear":
		if strings.TrimSpace(entryRaw) != "" {
			return result, http.StatusBadRequest, errors.New("clear does not accept an entry")
		}
		updated, changed, err = clearAntiscanManagedCustomListEntries(original)
		if err != nil {
			return result, http.StatusBadRequest, err
		}
		if !changed && !result.Active {
			result.Verified = true
			return result, http.StatusOK, nil
		}
	}

	afterEntries, _, err := parseAntiscanManagedCustomListEntries(updated)
	if err != nil {
		return result, http.StatusConflict, err
	}
	if len(afterEntries) == 0 {
		updated = []byte{}
	}
	result.CountAfter = len(afterEntries)

	if antiscanManagedCustomListNeedsFlush(result.Active, afterEntries) {
		flushTarget, targetErr := antiscanManagedCustomListFlushTarget(listName)
		if targetErr != nil {
			return result, http.StatusBadRequest, targetErr
		}
		flushResult, status, flushErr := applyAntiscanFlush(parent, cfg, flushTarget)
		result.Changed = flushResult.Changed
		result.Verified = flushResult.Verified
		result.RestartRequired = flushResult.RestartRequired
		result.Warnings = append(result.Warnings, flushResult.Warnings...)
		if flushErr != nil {
			return result, status, flushErr
		}
		return result, status, nil
	}

	if err := safety.WriteFileAtomic(listPath, updated, mode); err != nil {
		return result, http.StatusConflict, fmt.Errorf("write custom list: %w", err)
	}
	result.Changed = true

	if result.Active {
		if err := reloadAntiscanCustomLists(parent, cfg); err != nil {
			rollbackErr := rollbackAntiscanCustomList(parent, cfg, listPath, original, mode)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, fmt.Errorf("custom list reload failed and rollback failed: reload=%v rollback=%v", err, rollbackErr)
			}
			return result, http.StatusConflict, fmt.Errorf("custom list reload failed; file/runtime were restored: %w", err)
		}
		result.Reloaded = true
		if err := verifyAntiscanManagedCustomRuntime(parent, setName, afterEntries, entry, action == "add"); err != nil {
			rollbackErr := rollbackAntiscanCustomList(parent, cfg, listPath, original, mode)
			result.RollbackPerformed = rollbackErr == nil
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, fmt.Errorf("custom list verification failed and rollback failed: verify=%v rollback=%v", err, rollbackErr)
			}
			return result, http.StatusConflict, fmt.Errorf("custom list verification failed; file/runtime were restored: %w", err)
		}
	} else {
		if result.Configured {
			result.Warnings = append(result.Warnings, "Antiscan is stopped; source file was updated without runtime reload.")
		} else {
			result.Warnings = append(result.Warnings, "Custom list is not configured; source file was updated without runtime reload.")
		}
	}

	_, verifyData, _, verifyErr := readAntiscanCustomList(cfg, fileName)
	if verifyErr != nil {
		_ = safety.WriteFileAtomic(listPath, original, mode)
		result.RollbackPerformed = true
		return result, http.StatusConflict, errors.New("custom list file verification failed; original file was restored")
	}
	verifiedEntries, _, verifyErr := parseAntiscanManagedCustomListEntries(verifyData)
	if verifyErr != nil || !equalAntiscanManagedEntries(afterEntries, verifiedEntries) {
		_ = safety.WriteFileAtomic(listPath, original, mode)
		result.RollbackPerformed = true
		return result, http.StatusConflict, errors.New("custom list file verification failed; original file was restored")
	}

	result.Verified = true
	return result, http.StatusOK, nil
}

func antiscanManagedCustomListTarget(listName string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(listName)) {
	case "blacklist":
		return "ascn_custom_blacklist.txt", "ascn_custom_blacklist", nil
	case "whitelist":
		return "ascn_custom_whitelist.txt", "ascn_custom_whitelist", nil
	case "exclude":
		return "ascn_custom_exclude.txt", "ascn_custom_exclude", nil
	default:
		return "", "", errors.New("list must be blacklist, whitelist or exclude")
	}
}

func antiscanManagedCustomListFlushTarget(listName string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(listName)) {
	case "blacklist":
		return "custom_blacklist", nil
	case "whitelist":
		return "custom_whitelist", nil
	case "exclude":
		return "custom_exclude", nil
	default:
		return "", errors.New("list must be blacklist, whitelist or exclude")
	}
}

func antiscanManagedCustomListNeedsFlush(active bool, entries []string) bool {
	return active && len(entries) == 0
}

func antiscanManagedCustomListConfigured(cfg antiscanConfig, listName string) bool {
	switch strings.ToLower(strings.TrimSpace(listName)) {
	case "blacklist":
		return strings.EqualFold(cfg.CustomListsBlockMode, "blacklist")
	case "whitelist":
		return strings.EqualFold(cfg.CustomListsBlockMode, "whitelist")
	case "exclude":
		return cfg.UseCustomExcludeList
	default:
		return false
	}
}

func parseAntiscanManagedCustomListEntries(data []byte) ([]string, int, error) {
	if len(data) > antiscanCustomListMaxBytes {
		return nil, 0, errors.New("custom list exceeds RouterForge safety limit")
	}
	entries := []string{}
	seen := map[string]struct{}{}
	skipped := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		entry, err := normalizeAntiscanListEntry(fields[0])
		if err != nil {
			skipped++
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, err
	}
	return entries, skipped, nil
}

func removeAntiscanManagedCustomListEntry(original []byte, target string) ([]byte, bool, error) {
	if len(original) > antiscanCustomListMaxBytes {
		return nil, false, errors.New("custom list exceeds RouterForge safety limit")
	}
	if _, err := normalizeAntiscanListEntry(target); err != nil {
		return nil, false, err
	}
	lines := splitAntiscanManagedListLines(original)
	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && len(fields) > 0 {
			entry, err := normalizeAntiscanListEntry(fields[0])
			if err == nil && entry == target {
				changed = true
				continue
			}
		}
		out = append(out, line)
	}
	return joinAntiscanManagedListLines(out), changed, nil
}

func clearAntiscanManagedCustomListEntries(original []byte) ([]byte, bool, error) {
	if len(original) > antiscanCustomListMaxBytes {
		return nil, false, errors.New("custom list exceeds RouterForge safety limit")
	}
	if len(original) == 0 {
		return []byte{}, false, nil
	}
	return []byte{}, true, nil
}

func splitAntiscanManagedListLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

func joinAntiscanManagedListLines(lines []string) []byte {
	if len(lines) == 0 {
		return []byte{}
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func verifyAntiscanManagedCustomRuntime(parent context.Context, setName string, expected []string, focus string, wantFocus bool) error {
	binary := findAntiscanIPSetBinary()
	if binary == "" {
		return errors.New("ipset binary not found after custom-list reload")
	}
	exists, err := antiscanSetExists(parent, binary, setName)
	if err != nil {
		return fmt.Errorf("verify %s: %w", setName, err)
	}
	if !exists {
		return fmt.Errorf("custom-list verification failed: %s is absent", setName)
	}
	_, count, countKnown, _, err := readAntiscanSetEntries(parent, binary, setName, 1)
	if err != nil {
		return fmt.Errorf("verify %s count: %w", setName, err)
	}
	if !countKnown || count != int64(len(expected)) {
		return fmt.Errorf("custom-list verification failed: %s has %d entries, expected %d", setName, count, len(expected))
	}
	if focus != "" {
		present, _, memberErr := antiscanExactSetMember(parent, binary, setName, focus)
		if memberErr != nil {
			return fmt.Errorf("verify %s entry: %w", setName, memberErr)
		}
		if present != wantFocus {
			return fmt.Errorf("custom-list verification failed: entry membership mismatch in %s", setName)
		}
	}
	return nil
}

func equalAntiscanManagedEntries(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
