package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanFlushTimeout         = 90 * time.Second
	antiscanFlushSetCountTimeout = 10 * time.Second
)

var antiscanFlushTargets = []string{
	"candidates",
	"ips",
	"subnets",
	"custom_whitelist",
	"custom_blacklist",
	"custom_exclude",
	"geo",
	"ndm_lockout",
	"honeypot",
	"all",
}

type antiscanFlushSetPreview struct {
	Name       string `json:"name"`
	Exists     bool   `json:"exists"`
	Count      int64  `json:"count"`
	CountKnown bool   `json:"count_known"`
}

type antiscanFlushPreview struct {
	Target          string                    `json:"target"`
	Running         bool                      `json:"running"`
	Allowed         bool                      `json:"allowed"`
	BlockReason     string                    `json:"block_reason,omitempty"`
	Confirm         string                    `json:"confirm"`
	AffectedSets    []antiscanFlushSetPreview `json:"affected_sets"`
	BeforeEntries   int64                     `json:"before_entries"`
	DestructiveFile bool                      `json:"destructive_file"`
	RestartRequired bool                      `json:"restart_required"`
	Warnings        []string                  `json:"warnings"`
	MutationAPI     bool                      `json:"mutation_api"`
}

type antiscanFlushRequest struct {
	Target  string `json:"target"`
	Confirm string `json:"confirm"`
}

type antiscanFlushResult struct {
	Action            string   `json:"action"`
	Target            string   `json:"target"`
	BeforeEntries     int64    `json:"before_entries"`
	AfterEntries      int64    `json:"after_entries"`
	Changed           bool     `json:"changed"`
	Verified          bool     `json:"verified"`
	RollbackPerformed bool     `json:"rollback_performed"`
	RestartRequired   bool     `json:"restart_required"`
	MutationAPI       bool     `json:"mutation_api"`
	Warnings          []string `json:"warnings"`
	Error             string   `json:"error,omitempty"`
}

type antiscanFlushFileEffect struct {
	Path    string
	Kind    string
	Existed bool
	Size    int64
}

func handleAntiscanFlushPreview(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, err := normalizeAntiscanFlushTarget(r.URL.Query().Get("target"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":        err.Error(),
				"mutation_api": false,
			})
			return
		}
		preview, err := buildAntiscanFlushPreview(r.Context(), cfg, target)
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":        err.Error(),
				"mutation_api": false,
			})
			return
		}
		writeJSON(w, http.StatusOK, preview)
	}
}

func handleAntiscanFlushMutation(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanFlushRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, antiscanFlushResult{
				Action:      "flush",
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "invalid flush request",
			})
			return
		}

		target, err := normalizeAntiscanFlushTarget(request.Target)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, antiscanFlushResult{
				Action:      "flush",
				Target:      strings.ToLower(strings.TrimSpace(request.Target)),
				MutationAPI: true,
				Warnings:    []string{},
				Error:       err.Error(),
			})
			return
		}
		if request.Confirm != antiscanFlushConfirm(target) {
			writeJSON(w, http.StatusConflict, antiscanFlushResult{
				Action:      "flush",
				Target:      target,
				MutationAPI: true,
				Warnings:    []string{},
				Error:       "flush confirmation does not match target",
			})
			return
		}

		result, status, applyErr := applyAntiscanFlush(r.Context(), cfg, target)
		if applyErr != nil {
			result.Error = applyErr.Error()
		}
		writeJSON(w, status, result)
	}
}

func normalizeAntiscanFlushTarget(raw string) (string, error) {
	target := strings.ToLower(strings.TrimSpace(raw))
	for _, allowed := range antiscanFlushTargets {
		if target == allowed {
			return target, nil
		}
	}
	return "", errors.New("flush target must be candidates, ips, subnets, custom_whitelist, custom_blacklist, custom_exclude, geo, ndm_lockout, honeypot or all")
}

func antiscanFlushConfirm(target string) string {
	return "FLUSH_" + strings.ToUpper(target)
}

func antiscanFlushSetNames(target string) []string {
	switch target {
	case "candidates":
		return []string{"ascn_candidates"}
	case "ips":
		return []string{"ascn_ips"}
	case "subnets":
		return []string{"ascn_subnets"}
	case "custom_whitelist":
		return []string{"ascn_custom_whitelist"}
	case "custom_blacklist":
		return []string{"ascn_custom_blacklist"}
	case "custom_exclude":
		return []string{"ascn_custom_exclude"}
	case "geo":
		return []string{"ascn_geo_whitelist", "ascn_geo_blacklist", "ascn_geo_exclude"}
	case "ndm_lockout":
		return []string{"ascn_ndm_lockout"}
	case "honeypot":
		return []string{"ascn_honeypot"}
	case "all":
		// This intentionally mirrors upstream 1.10.6. The empty flush target does
		// not include custom lists or ascn_geo_exclude.
		return []string{
			"ascn_candidates",
			"ascn_ips",
			"ascn_subnets",
			"ascn_geo_whitelist",
			"ascn_geo_blacklist",
			"ascn_ndm_lockout",
			"ascn_honeypot",
		}
	default:
		return nil
	}
}

func buildAntiscanFlushPreview(parent context.Context, cfg runtimeConfig, target string) (antiscanFlushPreview, error) {
	target, err := normalizeAntiscanFlushTarget(target)
	if err != nil {
		return antiscanFlushPreview{}, err
	}
	config, err := readAntiscanConfig(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if err != nil {
		return antiscanFlushPreview{}, fmt.Errorf("read Antiscan config before flush: %w", err)
	}
	binary := findAntiscanIPSetBinary()
	if binary == "" {
		return antiscanFlushPreview{}, errors.New("ipset binary not found")
	}
	inventory, err := readAntiscanSetInventory(parent, binary)
	if err != nil {
		return antiscanFlushPreview{}, fmt.Errorf("read ipset inventory before flush: %w", err)
	}

	preview := antiscanFlushPreview{
		Target:          target,
		Running:         pathExists(cfg.StatusFile),
		Allowed:         true,
		Confirm:         antiscanFlushConfirm(target),
		AffectedSets:    []antiscanFlushSetPreview{},
		DestructiveFile: antiscanFlushTouchesSourceFiles(target),
		RestartRequired: antiscanFlushRestartRequired(config, target),
		Warnings:        antiscanFlushWarnings(config, target),
		MutationAPI:     true,
	}
	for _, name := range antiscanFlushSetNames(target) {
		info, countErr := resolveAntiscanFlushSetCount(parent, binary, inventory[name])
		if countErr != nil {
			return antiscanFlushPreview{}, fmt.Errorf("cannot verify %s before flush: %w", name, countErr)
		}
		preview.AffectedSets = append(preview.AffectedSets, antiscanFlushSetPreview{
			Name:       name,
			Exists:     info.Exists,
			Count:      info.Count,
			CountKnown: info.CountKnown,
		})
		if info.CountKnown {
			preview.BeforeEntries += info.Count
		}
	}

	if !preview.Running {
		preview.Allowed = false
		preview.BlockReason = "Antiscan must be running before flush"
	} else if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		preview.Allowed = false
		preview.BlockReason = "Antiscan reload is already in progress"
	} else if reason := antiscanFlushSafetyBlock(config, target); reason != "" {
		preview.Allowed = false
		preview.BlockReason = reason
	}
	return preview, nil
}

func antiscanFlushSafetyBlock(config antiscanConfig, target string) string {
	if config.SaveIPSets {
		return ""
	}
	if target == "custom_whitelist" && strings.EqualFold(config.CustomListsBlockMode, "whitelist") {
		return "flush blocked because active custom whitelist with SAVE_IPSETS=0 could lock out access"
	}
	if (target == "geo" || target == "all") && strings.EqualFold(config.GeoBlockMode, "whitelist") {
		return "flush blocked because active Geo whitelist with SAVE_IPSETS=0 could lock out access"
	}
	return ""
}

func antiscanFlushTouchesSourceFiles(target string) bool {
	switch target {
	case "custom_whitelist", "custom_blacklist", "custom_exclude", "geo", "all":
		return true
	default:
		return false
	}
}

func antiscanFlushRestartRequired(config antiscanConfig, target string) bool {
	if !config.SaveIPSets {
		return false
	}
	switch target {
	case "custom_whitelist":
		return strings.EqualFold(config.CustomListsBlockMode, "whitelist")
	case "custom_blacklist":
		return strings.EqualFold(config.CustomListsBlockMode, "blacklist")
	case "geo", "all":
		return strings.EqualFold(config.GeoBlockMode, "blacklist") ||
			strings.EqualFold(config.GeoBlockMode, "whitelist") || len(config.GeoExcludeCountries) > 0
	default:
		return false
	}
}

func antiscanFlushWarnings(config antiscanConfig, target string) []string {
	warnings := []string{}
	switch target {
	case "candidates", "ips", "subnets", "honeypot":
		warnings = append(warnings, "Dynamic protection sets can repopulate while Antiscan keeps running.")
	case "ndm_lockout":
		warnings = append(warnings, "Keenetic lockout entries can be imported again by read_ndm_ipsets.")
	case "custom_whitelist", "custom_blacklist":
		warnings = append(warnings, "This flush clears the upstream source file, not only the runtime ipset.")
	case "custom_exclude":
		warnings = append(warnings,
			"This flush clears the upstream source file, not only the runtime ipset.",
			"Custom exclusion entries will be erased from the source file; removed exceptions may expose addresses to blocking rules.",
		)
	case "geo":
		warnings = append(warnings, "Geo flush removes downloaded Geo files and empties all three Geo runtime sets.")
	case "all":
		warnings = append(warnings,
			"Dynamic protection sets can repopulate while Antiscan keeps running.",
			"The upstream all flush does not clear custom lists or ascn_geo_exclude.",
			"Geo files are removed because upstream all flush includes geo blacklist and whitelist sets.",
		)
	}
	if target == "custom_whitelist" && strings.EqualFold(config.CustomListsBlockMode, "whitelist") {
		warnings = append(warnings, "Active custom whitelist protection will be removed by upstream; repopulate the list or change mode before restart.")
	}
	if (target == "geo" || target == "all") && strings.EqualFold(config.GeoBlockMode, "whitelist") {
		warnings = append(warnings, "Active Geo whitelist protection will be removed by upstream; reload Geo or change mode before restart.")
	}
	return warnings
}

func applyAntiscanFlush(parent context.Context, cfg runtimeConfig, target string) (antiscanFlushResult, int, error) {
	result := antiscanFlushResult{
		Action:      "flush",
		Target:      target,
		MutationAPI: true,
		Warnings:    []string{},
	}
	preview, err := buildAntiscanFlushPreview(parent, cfg, target)
	if err != nil {
		return result, http.StatusConflict, err
	}
	result.BeforeEntries = preview.BeforeEntries
	result.RestartRequired = preview.RestartRequired
	result.Warnings = append(result.Warnings, preview.Warnings...)
	if !preview.Allowed {
		return result, http.StatusConflict, errors.New(preview.BlockReason)
	}
	if err := antiscanLifecycleReady(cfg); err != nil {
		return result, http.StatusConflict, err
	}

	config, err := readAntiscanConfig(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if err != nil {
		return result, http.StatusConflict, fmt.Errorf("read Antiscan config before flush: %w", err)
	}
	fileEffects, err := snapshotAntiscanFlushFileEffects(cfg, config, target)
	if err != nil {
		return result, http.StatusConflict, err
	}
	changedByFiles := false
	for _, effect := range fileEffects {
		if effect.Existed && (effect.Kind == "remove" || effect.Size > 0) {
			changedByFiles = true
		}
	}

	args, stdin, err := buildAntiscanFlushCommand(target)
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	ctx, cancel := context.WithTimeout(parent, antiscanFlushTimeout)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, cfg.InitScript, args...)
	if err != nil {
		return result, http.StatusConflict, err
	}
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return result, http.StatusConflict, errors.New("upstream Antiscan flush command failed")
	}
	if !pathExists(cfg.StatusFile) {
		return result, http.StatusConflict, errors.New("Antiscan stopped during flush")
	}
	if pathExists(cfg.ConfigLockFile) || pathExists(cfg.GeoLockFile) {
		return result, http.StatusConflict, errors.New("upstream flush returned while an Antiscan reload lock is still present")
	}

	binary := findAntiscanIPSetBinary()
	if binary == "" {
		return result, http.StatusConflict, errors.New("ipset binary not found after flush")
	}
	afterEntries, err := verifyAntiscanFlushSets(parent, binary, target)
	if err != nil {
		return result, http.StatusConflict, err
	}
	if err := verifyAntiscanFlushFileEffects(fileEffects); err != nil {
		return result, http.StatusConflict, err
	}

	result.AfterEntries = afterEntries
	result.Changed = result.BeforeEntries > 0 || changedByFiles
	result.Verified = true
	if !result.Changed {
		result.Warnings = append(result.Warnings, "Selected Antiscan sets were already empty; upstream flush completed and verification passed.")
	}
	return result, http.StatusOK, nil
}

func buildAntiscanFlushCommand(target string) ([]string, string, error) {
	target, err := normalizeAntiscanFlushTarget(target)
	if err != nil {
		return nil, "", err
	}
	args := []string{"flush"}
	if target != "all" {
		args = append(args, target)
	}
	return args, "Y\n", nil
}

func verifyAntiscanFlushSets(parent context.Context, binary, target string) (int64, error) {
	inventory, err := readAntiscanSetInventory(parent, binary)
	if err != nil {
		return 0, fmt.Errorf("read ipset inventory after flush: %w", err)
	}
	var total int64
	for _, name := range antiscanFlushSetNames(target) {
		info, countErr := resolveAntiscanFlushSetCount(parent, binary, inventory[name])
		if countErr != nil {
			return 0, fmt.Errorf("flush verification failed: cannot read %s: %w", name, countErr)
		}
		if !info.Exists {
			continue
		}
		if info.Count != 0 {
			return 0, fmt.Errorf("flush verification failed: %s still contains %d entries", name, info.Count)
		}
		total += info.Count
	}
	return total, nil
}

func resolveAntiscanFlushSetCount(parent context.Context, binary string, info antiscanSetInfo) (antiscanSetInfo, error) {
	if !info.Exists {
		return info, nil
	}
	if info.CountKnown && info.Error == "" {
		return info, nil
	}
	count, err := readAntiscanFlushSetCount(parent, binary, info.Name)
	if err != nil {
		return info, err
	}
	info.Count = count
	info.CountKnown = true
	info.Error = ""
	return info, nil
}

func readAntiscanFlushSetCount(parent context.Context, binary, setName string) (int64, error) {
	if !knownAntiscanSet(setName) {
		return 0, errors.New("unknown Antiscan ipset")
	}
	ctx, cancel := context.WithTimeout(parent, antiscanFlushSetCountTimeout)
	defer cancel()

	cmd, err := safety.CommandContext(ctx, binary, "save", setName)
	if err != nil {
		return 0, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	count, parseErr := parseAntiscanFlushSaveCount(stdout, setName)
	waitErr := cmd.Wait()
	if parseErr != nil {
		return 0, parseErr
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if waitErr != nil {
		return 0, waitErr
	}
	return count, nil
}

func parseAntiscanFlushSaveCount(r io.Reader, setName string) (int64, error) {
	if !knownAntiscanSet(setName) {
		return 0, errors.New("unknown Antiscan ipset")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	createSeen := false
	var count int64
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "create":
			if fields[1] == setName {
				createSeen = true
			}
		case "add":
			if fields[1] == setName {
				count++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if !createSeen {
		return 0, errors.New("ipset save set header missing")
	}
	return count, nil
}

func snapshotAntiscanFlushFileEffects(cfg runtimeConfig, config antiscanConfig, target string) ([]antiscanFlushFileEffect, error) {
	effects := []antiscanFlushFileEffect{}
	addFile := func(path, kind string, requireRegular bool) error {
		if strings.TrimSpace(path) == "" {
			return nil
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			effects = append(effects, antiscanFlushFileEffect{Path: path, Kind: kind})
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect flush file %s: %w", path, err)
		}
		if requireRegular && !info.Mode().IsRegular() {
			return fmt.Errorf("flush source file is not a regular file: %s", path)
		}
		effects = append(effects, antiscanFlushFileEffect{Path: path, Kind: kind, Existed: true, Size: info.Size()})
		return nil
	}

	switch target {
	case "custom_whitelist", "custom_blacklist", "custom_exclude":
		if err := addFile(filepath.Join(cfg.AntiscanDir, "ascn_"+target+".txt"), "truncate", true); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(config.IPSetsDirectory) != "" {
		cleanDir := filepath.Clean(config.IPSetsDirectory)
		if !filepath.IsAbs(cleanDir) || cleanDir == string(filepath.Separator) {
			return nil, errors.New("IPSETS_DIRECTORY is unsafe for verified flush")
		}
		for _, setName := range antiscanFlushPersistedSetNames(target) {
			if err := addFile(filepath.Join(cleanDir, "ipset_"+setName+".txt"), "remove", true); err != nil {
				return nil, err
			}
		}
		if target == "geo" || target == "all" {
			geoDir := filepath.Join(cleanDir, "geo")
			entries, err := os.ReadDir(geoDir)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("read Geo persistence directory before flush: %w", err)
			}
			if err == nil {
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".") || entry.IsDir() {
						continue
					}
					path := filepath.Join(geoDir, entry.Name())
					info, statErr := os.Lstat(path)
					if statErr != nil {
						return nil, fmt.Errorf("inspect Geo persistence file before flush: %w", statErr)
					}
					effects = append(effects, antiscanFlushFileEffect{Path: path, Kind: "remove", Existed: true, Size: info.Size()})
				}
			}
		}
	}
	return effects, nil
}

func antiscanFlushPersistedSetNames(target string) []string {
	switch target {
	case "candidates":
		return []string{"ascn_candidates"}
	case "ips":
		return []string{"ascn_ips"}
	case "subnets":
		return []string{"ascn_subnets"}
	case "ndm_lockout":
		return []string{"ascn_ndm_lockout"}
	case "honeypot":
		return []string{"ascn_honeypot"}
	case "all":
		return []string{"ascn_candidates", "ascn_ips", "ascn_subnets", "ascn_ndm_lockout", "ascn_honeypot"}
	default:
		return nil
	}
}

func verifyAntiscanFlushFileEffects(effects []antiscanFlushFileEffect) error {
	for _, effect := range effects {
		switch effect.Kind {
		case "truncate":
			info, err := os.Lstat(effect.Path)
			if effect.Existed {
				if err != nil {
					return fmt.Errorf("flush verification failed: source file disappeared: %s", effect.Path)
				}
				if !info.Mode().IsRegular() || info.Size() != 0 {
					return fmt.Errorf("flush verification failed: source file was not emptied: %s", effect.Path)
				}
			} else if err == nil || !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("flush verification failed: absent source file changed unexpectedly: %s", effect.Path)
			}
		case "remove":
			if !effect.Existed {
				continue
			}
			if _, err := os.Lstat(effect.Path); err == nil || !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("flush verification failed: persisted file still exists: %s", effect.Path)
			}
		}
	}
	return nil
}
