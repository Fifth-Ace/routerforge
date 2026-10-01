package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanMutationAuthHeader  = "X-RouterForge-Module-Authorized"
	antiscanMutationAuthValue   = "core-authorized-v1"
	antiscanMutationBodyMax     = 8 << 10
	antiscanCustomListMaxBytes  = 2 << 20
	antiscanPersistenceMaxBytes = 16 << 20
	antiscanMutationTimeout     = 15 * time.Second
)

type antiscanUnbanRequest struct {
	Set     string `json:"set"`
	Entry   string `json:"entry"`
	Confirm string `json:"confirm"`
}

type antiscanListEntryRequest struct {
	List    string `json:"list"`
	Entry   string `json:"entry"`
	Confirm string `json:"confirm"`
}

type antiscanMutationResult struct {
	Action             string   `json:"action"`
	Set                string   `json:"set,omitempty"`
	List               string   `json:"list,omitempty"`
	Entry              string   `json:"entry"`
	Changed            bool     `json:"changed"`
	Verified           bool     `json:"verified"`
	Active             bool     `json:"active"`
	PersistenceUpdated bool     `json:"persistence_updated"`
	MutationAPI        bool     `json:"mutation_api"`
	Warnings           []string `json:"warnings"`
}

func mutationOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error":        "POST required",
				"mutation_api": true,
			})
			return
		}
		if r.Header.Get(antiscanMutationAuthHeader) != antiscanMutationAuthValue {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":        "authorized RouterForge Core request required",
				"mutation_api": true,
			})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func decodeAntiscanMutationJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, antiscanMutationBodyMax)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func handleAntiscanUnban(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanUnbanRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid unban request", "mutation_api": true})
			return
		}
		if request.Confirm != "UNBAN" {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal UNBAN", "mutation_api": true})
			return
		}
		result, status, err := unbanAntiscanEntry(r.Context(), cfg, request.Set, request.Entry)
		if err != nil {
			writeJSON(w, status, map[string]any{"error": err.Error(), "mutation_api": true})
			return
		}
		writeJSON(w, status, result)
	}
}

func handleAntiscanListEntry(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanListEntryRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list-entry request", "mutation_api": true})
			return
		}
		if request.Confirm != "ADD" {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal ADD", "mutation_api": true})
			return
		}
		result, status, err := addAntiscanCustomListEntry(r.Context(), cfg, request.List, request.Entry)
		if err != nil {
			writeJSON(w, status, map[string]any{"error": err.Error(), "mutation_api": true})
			return
		}
		writeJSON(w, status, result)
	}
}

func unbanAntiscanEntry(parent context.Context, cfg runtimeConfig, setName, rawEntry string) (antiscanMutationResult, int, error) {
	result := antiscanMutationResult{Action: "unban", Set: strings.TrimSpace(setName), MutationAPI: true}
	entry, err := normalizeAntiscanUnbanTarget(setName, rawEntry)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	result.Entry = entry

	config, err := readAntiscanConfig(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("read Antiscan config: %w", err)
	}
	if err := antiscanRuntimeMutationReady(cfg); err != nil {
		return result, http.StatusConflict, err
	}

	binary := findAntiscanIPSetBinary()
	if binary == "" {
		return result, http.StatusConflict, errors.New("ipset binary not found")
	}
	exists, err := antiscanSetExists(parent, binary, setName)
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("inspect ipset: %w", err)
	}
	if !exists {
		return result, http.StatusConflict, fmt.Errorf("%s does not exist", setName)
	}
	member, entryInfo, err := antiscanExactSetMember(parent, binary, setName, entry)
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("inspect target entry: %w", err)
	}
	if !member {
		result.Verified = true
		result.Warnings = append(result.Warnings, "Entry was already absent from the runtime set.")
		return result, http.StatusOK, nil
	}

	var persistencePath string
	if config.SaveIPSets {
		persistencePath, err = preflightAntiscanPersistence(config, setName)
		if err != nil {
			return result, http.StatusConflict, err
		}
	}

	ctx, cancel := context.WithTimeout(parent, antiscanMutationTimeout)
	defer cancel()
	if output, runErr := safety.RunCommand(ctx, 16<<10, binary, "del", setName, entry); runErr != nil {
		return result, http.StatusConflict, fmt.Errorf("ipset del failed: %s", compactMutationOutput(output, runErr))
	}
	result.Changed = true

	stillPresent, _, verifyErr := antiscanExactSetMember(parent, binary, setName, entry)
	if stillPresent {
		return result, http.StatusConflict, errors.New("unban verification failed: entry is still present")
	}
	if verifyErr != nil {
		rollbackErr := restoreAntiscanSetEntry(parent, binary, setName, entry, entryInfo)
		if rollbackErr != nil {
			return result, http.StatusInternalServerError, fmt.Errorf("unban verification failed and rollback failed: verify=%v rollback=%v", verifyErr, rollbackErr)
		}
		return result, http.StatusConflict, errors.New("unban verification failed; runtime state was restored")
	}

	if config.SaveIPSets {
		if err := persistAntiscanRuntimeSet(parent, binary, setName, persistencePath); err != nil {
			rollbackErr := restoreAntiscanSetEntry(parent, binary, setName, entry, entryInfo)
			if rollbackErr != nil {
				return result, http.StatusInternalServerError, fmt.Errorf("persistence update failed and runtime rollback failed: persist=%v rollback=%v", err, rollbackErr)
			}
			return result, http.StatusConflict, fmt.Errorf("persistence update failed; runtime state was restored: %w", err)
		}
		result.PersistenceUpdated = true
	}

	result.Verified = true
	return result, http.StatusOK, nil
}

func addAntiscanCustomListEntry(parent context.Context, cfg runtimeConfig, listName, rawEntry string) (antiscanMutationResult, int, error) {
	result := antiscanMutationResult{Action: "add-list-entry", List: strings.TrimSpace(listName), MutationAPI: true}
	fileName, setName, err := antiscanCustomListTarget(listName)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	entry, err := normalizeAntiscanListEntry(rawEntry)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	result.Entry = entry
	result.Set = setName

	config, err := readAntiscanConfig(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("read Antiscan config: %w", err)
	}
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return result, http.StatusConflict, errors.New("Antiscan reload is in progress")
	}

	listPath, original, mode, err := readAntiscanCustomList(cfg, fileName)
	if err != nil {
		return result, http.StatusConflict, err
	}
	updated, changed, err := appendAntiscanListEntry(original, entry)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	active := antiscanCustomListActive(config, listName)
	result.Active = active && pathExists(cfg.StatusFile)
	if !changed {
		result.Verified = true
		result.Warnings = append(result.Warnings, "Entry already exists in the upstream custom list file.")
		return result, http.StatusOK, nil
	}

	if err := safety.WriteFileAtomic(listPath, updated, mode); err != nil {
		return result, http.StatusConflict, fmt.Errorf("write custom list: %w", err)
	}
	result.Changed = true

	if !result.Active {
		result.Verified = true
		if !pathExists(cfg.StatusFile) {
			result.Warnings = append(result.Warnings, "Entry is stored, but Antiscan is stopped; it will become effective when the matching list mode is loaded.")
		} else if listName == "exclude" {
			result.Warnings = append(result.Warnings, "Entry is stored, but USE_CUSTOM_EXCLUDE_LIST is disabled in ascn.conf.")
		} else {
			result.Warnings = append(result.Warnings, "Entry is stored, but CUSTOM_LISTS_BLOCK_MODE is not whitelist.")
		}
		return result, http.StatusOK, nil
	}

	if err := reloadAntiscanCustomLists(parent, cfg); err != nil {
		rollbackErr := rollbackAntiscanCustomList(parent, cfg, listPath, original, mode)
		if rollbackErr != nil {
			return result, http.StatusInternalServerError, fmt.Errorf("custom list reload failed and rollback failed: reload=%v rollback=%v", err, rollbackErr)
		}
		return result, http.StatusConflict, fmt.Errorf("custom list reload failed; file/runtime were restored: %w", err)
	}

	binary := findAntiscanIPSetBinary()
	if binary == "" {
		rollbackErr := rollbackAntiscanCustomList(parent, cfg, listPath, original, mode)
		if rollbackErr != nil {
			return result, http.StatusInternalServerError, fmt.Errorf("ipset verification unavailable and rollback failed: %v", rollbackErr)
		}
		return result, http.StatusConflict, errors.New("ipset verification unavailable; file/runtime were restored")
	}
	present, _, verifyErr := antiscanExactSetMember(parent, binary, setName, entry)
	if verifyErr != nil || !present {
		rollbackErr := rollbackAntiscanCustomList(parent, cfg, listPath, original, mode)
		if rollbackErr != nil {
			return result, http.StatusInternalServerError, fmt.Errorf("custom list verification failed and rollback failed: verify=%v rollback=%v", verifyErr, rollbackErr)
		}
		return result, http.StatusConflict, errors.New("custom list verification failed; file/runtime were restored")
	}

	result.Verified = true
	return result, http.StatusOK, nil
}

func normalizeAntiscanUnbanTarget(setName, raw string) (string, error) {
	setName = strings.TrimSpace(setName)
	switch setName {
	case "ascn_ips", "ascn_honeypot":
		addr, err := netip.ParseAddr(strings.TrimSpace(raw))
		if err != nil || !addr.Is4() {
			return "", errors.New("unban target must be an IPv4 address")
		}
		return addr.String(), nil
	case "ascn_subnets":
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() != 24 {
			return "", errors.New("ascn_subnets unban target must be an IPv4 /24 prefix")
		}
		return prefix.Masked().String(), nil
	default:
		return "", errors.New("single-entry unban is allowed only for ascn_ips, ascn_subnets and ascn_honeypot")
	}
}

func normalizeAntiscanListEntry(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if addr, err := netip.ParseAddr(value); err == nil && addr.Is4() {
		return addr.String(), nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() {
		return "", errors.New("custom list entry must be an IPv4 address or CIDR prefix")
	}
	return prefix.Masked().String(), nil
}

func antiscanCustomListTarget(listName string) (string, string, error) {
	switch strings.TrimSpace(listName) {
	case "exclude":
		return "ascn_custom_exclude.txt", "ascn_custom_exclude", nil
	case "whitelist":
		return "ascn_custom_whitelist.txt", "ascn_custom_whitelist", nil
	default:
		return "", "", errors.New("list must be exclude or whitelist")
	}
}

func antiscanCustomListActive(cfg antiscanConfig, listName string) bool {
	switch strings.TrimSpace(listName) {
	case "exclude":
		return cfg.UseCustomExcludeList
	case "whitelist":
		return strings.EqualFold(cfg.CustomListsBlockMode, "whitelist")
	default:
		return false
	}
}

func appendAntiscanListEntry(original []byte, entry string) ([]byte, bool, error) {
	if len(original) > antiscanCustomListMaxBytes {
		return nil, false, errors.New("custom list exceeds RouterForge safety limit")
	}
	if _, err := normalizeAntiscanListEntry(entry); err != nil {
		return nil, false, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(original))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		first := strings.Fields(line)
		if len(first) == 0 {
			continue
		}
		normalized, err := normalizeAntiscanListEntry(first[0])
		if err == nil && normalized == entry {
			return append([]byte(nil), original...), false, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	out := append([]byte(nil), original...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, []byte(entry+"\n")...)
	if len(out) > antiscanCustomListMaxBytes {
		return nil, false, errors.New("custom list would exceed RouterForge safety limit")
	}
	return out, true, nil
}

func readAntiscanCustomList(cfg runtimeConfig, fileName string) (string, []byte, os.FileMode, error) {
	resolver := safety.Resolver{Roots: []string{cfg.AntiscanDir}}
	raw := filepath.Join(cfg.AntiscanDir, fileName)
	resolved, err := resolver.ResolveCreatable(raw, nil, nil)
	if err != nil {
		return "", nil, 0, fmt.Errorf("resolve custom list: %w", err)
	}
	mode := os.FileMode(0644)
	data := []byte{}
	info, statErr := os.Stat(resolved.Canonical)
	if statErr == nil {
		if !info.Mode().IsRegular() {
			return "", nil, 0, errors.New("custom list is not a regular file")
		}
		mode = info.Mode().Perm()
		file, openErr := os.Open(resolved.Canonical)
		if openErr != nil {
			return "", nil, 0, openErr
		}
		defer file.Close()
		data, openErr = io.ReadAll(io.LimitReader(file, antiscanCustomListMaxBytes+1))
		if openErr != nil {
			return "", nil, 0, openErr
		}
		if len(data) > antiscanCustomListMaxBytes {
			return "", nil, 0, errors.New("custom list exceeds RouterForge safety limit")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", nil, 0, statErr
	}
	return resolved.Canonical, data, mode, nil
}

func antiscanRuntimeMutationReady(cfg runtimeConfig) error {
	if !pathExists(cfg.StatusFile) {
		return errors.New("Antiscan is not running")
	}
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return errors.New("Antiscan reload is in progress")
	}
	info, err := os.Stat(cfg.InitScript)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return errors.New("Antiscan init script is unavailable")
	}
	return nil
}

func antiscanExactSetMember(parent context.Context, binary, setName, entry string) (bool, antiscanSetEntry, error) {
	if !knownAntiscanSet(setName) {
		return false, antiscanSetEntry{}, errors.New("unknown Antiscan ipset")
	}
	ctx, cancel := context.WithTimeout(parent, antiscanSetListTimeout)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, binary, "list", setName)
	if err != nil {
		return false, antiscanSetEntry{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, antiscanSetEntry{}, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return false, antiscanSetEntry{}, err
	}
	found, info, parseErr := findAntiscanSetEntry(stdout, entry)
	waitErr := cmd.Wait()
	if parseErr != nil {
		return false, antiscanSetEntry{}, parseErr
	}
	if ctx.Err() != nil {
		return false, antiscanSetEntry{}, ctx.Err()
	}
	if waitErr != nil {
		return false, antiscanSetEntry{}, waitErr
	}
	return found, info, nil
}

func findAntiscanSetEntry(r io.Reader, expected string) (bool, antiscanSetEntry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	inMembers := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !inMembers {
			if line == "Members:" {
				inMembers = true
			}
			continue
		}
		entry, ok := parseAntiscanSetEntry(line)
		if ok && entry.Value == expected {
			return true, entry, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, antiscanSetEntry{}, err
	}
	return false, antiscanSetEntry{}, nil
}

func preflightAntiscanPersistence(cfg antiscanConfig, setName string) (string, error) {
	dir := strings.TrimSpace(cfg.IPSetsDirectory)
	if dir == "" {
		return "", errors.New("SAVE_IPSETS is enabled but IPSETS_DIRECTORY is empty")
	}
	resolver := safety.Resolver{Roots: []string{"/opt", "/tmp"}}
	raw := filepath.Join(dir, "ipset_"+setName+".txt")
	resolved, err := resolver.ResolveCreatable(raw, nil, nil)
	if err != nil {
		return "", fmt.Errorf("resolve persistence file: %w", err)
	}
	parent := filepath.Dir(resolved.Canonical)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return "", errors.New("IPSETS_DIRECTORY is unavailable")
	}
	probe, err := safety.NewAtomicFile(parent, ".routerforge-antiscan-probe-*")
	if err != nil {
		return "", fmt.Errorf("persistence directory is not writable: %w", err)
	}
	_ = probe.Cleanup()
	return resolved.Canonical, nil
}

func persistAntiscanRuntimeSet(parent context.Context, binary, setName, destination string) error {
	ctx, cancel := context.WithTimeout(parent, antiscanMutationTimeout)
	defer cancel()
	atomicFile, err := safety.NewAtomicFile(filepath.Dir(destination), "."+filepath.Base(destination)+".tmp-*")
	if err != nil {
		return err
	}
	defer atomicFile.Cleanup()
	if err := atomicFile.File().Chmod(0644); err != nil {
		return err
	}
	cmd, err := safety.CommandContext(ctx, binary, "save", setName)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stdout = atomicFile.File()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ipset save failed: %s", compactMutationOutput(stderr.Bytes(), err))
	}
	info, err := atomicFile.File().Stat()
	if err != nil {
		return err
	}
	if info.Size() > antiscanPersistenceMaxBytes {
		return errors.New("persisted ipset exceeds RouterForge safety limit")
	}
	if err := atomicFile.Sync(); err != nil {
		return err
	}
	if err := atomicFile.Close(); err != nil {
		return err
	}
	return atomicFile.Publish(destination)
}

func restoreAntiscanSetEntry(parent context.Context, binary, setName, entry string, info antiscanSetEntry) error {
	args := []string{"add", setName, entry}
	if info.TimeoutKnown && info.TimeoutSeconds > 0 {
		args = append(args, "timeout", fmt.Sprintf("%d", info.TimeoutSeconds))
	}
	if info.PacketsKnown {
		args = append(args, "packets", fmt.Sprintf("%d", info.Packets))
	}
	if info.BytesKnown {
		args = append(args, "bytes", fmt.Sprintf("%d", info.Bytes))
	}
	ctx, cancel := context.WithTimeout(parent, antiscanMutationTimeout)
	defer cancel()
	output, err := safety.RunCommand(ctx, 16<<10, binary, args...)
	if err != nil {
		return fmt.Errorf("restore runtime entry: %s", compactMutationOutput(output, err))
	}
	return nil
}

func reloadAntiscanCustomLists(parent context.Context, cfg runtimeConfig) error {
	ctx, cancel := context.WithTimeout(parent, antiscanMutationTimeout)
	defer cancel()
	output, err := safety.RunCommand(ctx, 32<<10, cfg.InitScript, "update_ipsets", "custom")
	if err != nil {
		return fmt.Errorf("upstream update_ipsets custom failed: %s", compactMutationOutput(output, err))
	}
	return nil
}

func rollbackAntiscanCustomList(parent context.Context, cfg runtimeConfig, path string, original []byte, mode os.FileMode) error {
	if err := safety.WriteFileAtomic(path, original, mode); err != nil {
		return err
	}
	if pathExists(cfg.StatusFile) {
		if err := reloadAntiscanCustomLists(parent, cfg); err != nil {
			return err
		}
	}
	return nil
}

func compactMutationOutput(output []byte, err error) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 512 {
		text = text[:512]
	}
	if text != "" {
		return text
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}
