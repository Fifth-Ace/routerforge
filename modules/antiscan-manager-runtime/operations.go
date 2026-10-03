package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanOperationTimeout    = 90 * time.Second
	antiscanGeoOperationTimeout = 12 * time.Minute
	antiscanOperationOutputMax  = 64 << 10
)

type antiscanOperationRequest struct {
	Action  string `json:"action"`
	Scope   string `json:"scope,omitempty"`
	Confirm string `json:"confirm"`
}

type antiscanOperationResult struct {
	Action        string   `json:"action"`
	Scope         string   `json:"scope,omitempty"`
	BeforeRunning bool     `json:"before_running"`
	AfterRunning  bool     `json:"after_running"`
	Changed       bool     `json:"changed"`
	Verified      bool     `json:"verified"`
	MutationAPI   bool     `json:"mutation_api"`
	Output        string   `json:"output,omitempty"`
	Warnings      []string `json:"warnings"`
	Error         string   `json:"error,omitempty"`
}

type antiscanOperationSpec struct {
	Action          string
	Scope           string
	Args            []string
	RequiresRunning bool
	Timeout         time.Duration
}

func handleAntiscanOperation(cfg runtimeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request antiscanOperationRequest
		if err := decodeAntiscanMutationJSON(w, r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid operation request", "mutation_api": true})
			return
		}
		if request.Confirm != "RUN" {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal RUN", "mutation_api": true})
			return
		}

		result, status, runErr := applyAntiscanOperation(r.Context(), cfg, request.Action, request.Scope)
		if runErr != nil {
			result.Error = runErr.Error()
		}
		writeJSON(w, status, result)
	}
}

func normalizeAntiscanOperation(actionRaw, scopeRaw string) (antiscanOperationSpec, error) {
	action := strings.ToLower(strings.TrimSpace(actionRaw))
	scope := strings.ToLower(strings.TrimSpace(scopeRaw))
	spec := antiscanOperationSpec{
		Action:          action,
		Scope:           scope,
		RequiresRunning: true,
		Timeout:         antiscanOperationTimeout,
	}

	switch action {
	case "update_rules":
		if scope != "" {
			return antiscanOperationSpec{}, errors.New("update_rules does not accept a scope")
		}
		spec.Args = []string{"update_rules"}
	case "read_candidates":
		if scope != "" {
			return antiscanOperationSpec{}, errors.New("read_candidates does not accept a scope")
		}
		spec.Args = []string{"read_candidates"}
	case "read_ndm_ipsets":
		if scope != "" {
			return antiscanOperationSpec{}, errors.New("read_ndm_ipsets does not accept a scope")
		}
		spec.Args = []string{"read_ndm_ipsets"}
	case "save_ipsets":
		if scope != "" {
			return antiscanOperationSpec{}, errors.New("save_ipsets does not accept a scope")
		}
		spec.Args = []string{"save_ipsets"}
	case "update_ipsets":
		if scope != "custom" && scope != "geo" {
			return antiscanOperationSpec{}, errors.New("update_ipsets scope must be custom or geo")
		}
		spec.Args = []string{"update_ipsets", scope}
		if scope == "geo" {
			spec.Timeout = antiscanGeoOperationTimeout
		}
	case "retry_load_geo":
		if scope != "" {
			return antiscanOperationSpec{}, errors.New("retry_load_geo does not accept a scope")
		}
		spec.Args = []string{"retry_load_geo"}
		spec.Timeout = antiscanGeoOperationTimeout
	case "update_crontab":
		if scope != "" {
			return antiscanOperationSpec{}, errors.New("update_crontab does not accept a scope")
		}
		spec.Args = []string{"update_crontab"}
		spec.RequiresRunning = false
	default:
		return antiscanOperationSpec{}, errors.New("unsupported Antiscan operation")
	}
	return spec, nil
}

func applyAntiscanOperation(parent context.Context, cfg runtimeConfig, actionRaw, scopeRaw string) (antiscanOperationResult, int, error) {
	spec, err := normalizeAntiscanOperation(actionRaw, scopeRaw)
	result := antiscanOperationResult{
		Action:      strings.ToLower(strings.TrimSpace(actionRaw)),
		Scope:       strings.ToLower(strings.TrimSpace(scopeRaw)),
		MutationAPI: true,
		Warnings:    []string{},
	}
	if err != nil {
		return result, http.StatusBadRequest, err
	}
	result.Action = spec.Action
	result.Scope = spec.Scope
	result.BeforeRunning = pathExists(cfg.StatusFile)
	result.AfterRunning = result.BeforeRunning

	if err := antiscanOperationCommandReady(cfg); err != nil {
		return result, http.StatusConflict, err
	}

	config, configErr := readAntiscanConfig(filepath.Join(cfg.AntiscanDir, "ascn.conf"))
	if configErr != nil {
		return result, http.StatusConflict, fmt.Errorf("read Antiscan config: %w", configErr)
	}

	ctx, cancel := context.WithTimeout(parent, spec.Timeout)
	defer cancel()
	output, runErr := safety.RunCommand(ctx, antiscanOperationOutputMax, cfg.InitScript, spec.Args...)
	result.Output = sanitizeAntiscanOperationOutput(output)
	result.AfterRunning = pathExists(cfg.StatusFile)

	if runErr != nil {
		return result, http.StatusConflict, fmt.Errorf("upstream %s failed: %s", antiscanOperationLabel(spec), compactMutationOutput(output, runErr))
	}
	if spec.RequiresRunning && !result.AfterRunning {
		return result, http.StatusConflict, errors.New("Antiscan stopped during the operation")
	}
	if err := verifyAntiscanOperation(parent, cfg, config, spec); err != nil {
		return result, http.StatusConflict, err
	}

	result.Changed = true
	result.Verified = true
	return result, http.StatusOK, nil
}

func antiscanOperationCommandReady(cfg runtimeConfig) error {
	info, err := os.Stat(cfg.InitScript)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return errors.New("Antiscan init script is unavailable or not executable")
	}
	return nil
}

func verifyAntiscanOperation(parent context.Context, cfg runtimeConfig, config antiscanConfig, spec antiscanOperationSpec) error {
	switch spec.Action {
	case "update_rules":
		binary := findAntiscanExecutable(
			"/opt/sbin/iptables",
			"/opt/bin/iptables",
			"/usr/sbin/iptables",
			"/sbin/iptables",
			"iptables",
		)
		check := diagnoseAntiscanIPTables(parent, binary, true)
		if check.State != "pass" {
			return errors.New("update_rules verification failed: ANTISCAN chain is unavailable")
		}
	case "read_candidates":
		if !config.EnableIPSBan {
			return nil
		}
		binary := findAntiscanIPSetBinary()
		if binary == "" {
			return errors.New("ipset binary not found after candidate processing")
		}
		for _, setName := range []string{"ascn_candidates", "ascn_ips", "ascn_subnets"} {
			exists, err := antiscanSetExists(parent, binary, setName)
			if err != nil {
				return fmt.Errorf("verify %s: %w", setName, err)
			}
			if !exists {
				return fmt.Errorf("read_candidates verification failed: %s is absent", setName)
			}
		}
	case "read_ndm_ipsets":
		if !config.ReadNDMLockoutIPSets {
			return nil
		}
		binary := findAntiscanIPSetBinary()
		if binary == "" {
			return errors.New("ipset binary not found after Keenetic lockout import")
		}
		exists, err := antiscanSetExists(parent, binary, "ascn_ndm_lockout")
		if err != nil {
			return fmt.Errorf("verify ascn_ndm_lockout: %w", err)
		}
		if !exists {
			return errors.New("read_ndm_ipsets verification failed: ascn_ndm_lockout is absent")
		}
	case "update_ipsets":
		if spec.Scope == "custom" && config.CustomListsBlockMode == "0" && !config.UseCustomExcludeList {
			return nil
		}
		if spec.Scope == "geo" && config.GeoBlockMode == "0" && len(config.GeoExcludeCountries) == 0 {
			return nil
		}
		binary := findAntiscanIPSetBinary()
		if binary == "" {
			return errors.New("ipset binary not found after update")
		}
		if spec.Scope == "custom" {
			if err := verifyConfiguredCustomIPSets(parent, binary, config); err != nil {
				return err
			}
		} else {
			if err := verifyConfiguredGeoIPSets(parent, binary, config); err != nil {
				return err
			}
		}
	case "retry_load_geo":
		if pathExists(cfg.GeoLockFile) {
			return errors.New("retry_load_geo verification failed: Geo lock is still present")
		}
		if config.GeoBlockMode == "0" && len(config.GeoExcludeCountries) == 0 {
			return nil
		}
		binary := findAntiscanIPSetBinary()
		if binary == "" {
			return errors.New("ipset binary not found after Geo retry")
		}
		if err := verifyConfiguredGeoIPSets(parent, binary, config); err != nil {
			return err
		}
	case "update_crontab":
		if err := verifyAntiscanCronOperation(parent, cfg); err != nil {
			return err
		}
	}
	return nil
}

func verifyAntiscanCronOperation(parent context.Context, cfg runtimeConfig) error {
	data, err := os.ReadFile(filepath.Join(cfg.AntiscanDir, "ascn_crontab.conf"))
	if err != nil {
		return fmt.Errorf("update_crontab verification failed: %w", err)
	}
	tasks, invalid := parseAntiscanCronTasks(string(data))
	if len(invalid) > 0 {
		return fmt.Errorf("update_crontab verification failed: invalid tasks remain")
	}
	binary := findAntiscanExecutable("/opt/bin/crontab", "/usr/bin/crontab", "crontab")
	if binary == "" {
		return errors.New("update_crontab verification failed: crontab not found")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	output, runErr := safety.RunCommand(ctx, 32<<10, binary, "-l")
	if runErr != nil {
		return fmt.Errorf("update_crontab verification failed: %s", compactMutationOutput(output, runErr))
	}
	installed := countAntiscanCronLines(string(output))
	if installed != len(tasks) {
		return fmt.Errorf("update_crontab verification failed: installed %d Antiscan tasks, expected %d", installed, len(tasks))
	}
	return nil
}

func verifyConfiguredCustomIPSets(parent context.Context, binary string, config antiscanConfig) error {
	expected := []string{}
	if config.CustomListsBlockMode == "blacklist" || config.CustomListsBlockMode == "whitelist" {
		expected = append(expected, "ascn_custom_"+config.CustomListsBlockMode)
	}
	if config.UseCustomExcludeList {
		expected = append(expected, "ascn_custom_exclude")
	}
	for _, setName := range expected {
		exists, err := antiscanSetExists(parent, binary, setName)
		if err != nil {
			return fmt.Errorf("verify %s: %w", setName, err)
		}
		if !exists {
			return fmt.Errorf("update_ipsets custom verification failed: %s is absent", setName)
		}
	}
	return nil
}

func verifyConfiguredGeoIPSets(parent context.Context, binary string, config antiscanConfig) error {
	expected := []string{}
	if config.GeoBlockMode == "blacklist" || config.GeoBlockMode == "whitelist" {
		expected = append(expected, "ascn_geo_"+config.GeoBlockMode)
	}
	if len(config.GeoExcludeCountries) > 0 {
		expected = append(expected, "ascn_geo_exclude")
	}
	for _, setName := range expected {
		exists, err := antiscanSetExists(parent, binary, setName)
		if err != nil {
			return fmt.Errorf("verify %s: %w", setName, err)
		}
		if !exists {
			return fmt.Errorf("update_ipsets geo verification failed: %s is absent", setName)
		}
	}
	return nil
}

func antiscanOperationLabel(spec antiscanOperationSpec) string {
	if spec.Action == "update_ipsets" {
		return spec.Action + " " + spec.Scope
	}
	return spec.Action
}

var antiscanANSIEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func sanitizeAntiscanOperationOutput(output []byte) string {
	text := antiscanANSIEscape.ReplaceAllString(string(output), "")
	text = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r >= 32 {
			return r
		}
		return -1
	}, text)
	return strings.TrimSpace(text)
}
