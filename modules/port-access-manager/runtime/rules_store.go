package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Staging only: storing configuration NEVER executes or schedules firewall operations.
const rulesStorePath = "/opt/etc/routerforge/port-access-manager/rules.json"
const rulesMaxBody = 8192

var ruleIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)
var rulesMu sync.Mutex

type managedKnockRule struct {
	ID            string `json:"id"`
	Engine        string `json:"engine"`
	Sequence      [3]int `json:"sequence"`
	Target        int    `json:"target"`
	WindowSeconds int    `json:"window_seconds"`
	AccessSeconds int    `json:"access_seconds"`
	WAN           string `json:"wan"`
	Enabled       bool   `json:"enabled"`
}

type managedRuleFile struct {
	Schema int                `json:"schema"`
	Rules  []managedKnockRule `json:"rules"`
}

func validateManagedRule(r managedKnockRule) error {
	if !ruleIDPattern.MatchString(r.ID) {
		return errors.New("invalid rule id")
	}
	if r.Engine != "iptables-recent" {
		return errors.New("engine not supported for staging")
	}
	if r.Enabled {
		return errors.New("activation unavailable until hardware acceptance")
	}
	if !wanInterfacePattern.MatchString(r.WAN) {
		return errors.New("invalid WAN interface")
	}
	plan, err := buildKnockRules(KnockOptions{Sequence: r.Sequence, Target: r.Target, WindowSeconds: r.WindowSeconds, AccessSeconds: r.AccessSeconds})
	if err != nil {
		return err
	}
	if !plan.DeploymentBlocked || plan.Applied || plan.Hooked || plan.StrictSequenceVerified {
		return errors.New("unsafe deployment contract")
	}
	return nil
}

func readManagedRules(path string) (managedRuleFile, error) {
	file := managedRuleFile{Schema: 1, Rules: []managedKnockRule{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return file, nil
	}
	if err != nil {
		return file, err
	}
	if len(data) > 262144 {
		return file, errors.New("rule store exceeds limit")
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return file, err
	}
	if file.Schema != 1 || len(file.Rules) > 32 {
		return file, errors.New("unsupported or excessive rule store")
	}
	seen := make(map[string]bool)
	for _, rule := range file.Rules {
		if err := validateManagedRule(rule); err != nil {
			return file, err
		}
		if seen[rule.ID] {
			return file, errors.New("duplicate rule id")
		}
		seen[rule.ID] = true
	}
	sort.Slice(file.Rules, func(i, j int) bool { return file.Rules[i].ID < file.Rules[j].ID })
	return file, nil
}

func writeManagedRules(path string, file managedRuleFile) error {
	// Never create or follow a symlink at the destination.
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("rule store is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid rule store directory")
	}
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".rules-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(payload, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func stagedRulesHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rulesMu.Lock()
		defer rulesMu.Unlock()
		file, err := readManagedRules(path)
		if err != nil {
			jsonReply(w, 500, map[string]string{"error": "staged rule store unavailable"})
			return
		}
		if r.Method == http.MethodGet {
			jsonReply(w, 200, map[string]any{"schema": 1, "mode": "staging-only", "mutation_api": true, "firewall_mutation": false, "ready_for_apply": false, "rules": file.Rules})
			return
		}
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			w.Header().Set("Allow", "GET, POST, DELETE")
			jsonReply(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		if r.Header.Get("X-RouterForge-Action") != "stage-rule" || r.Header.Get("Content-Type") != "application/json" {
			jsonReply(w, 403, map[string]string{"error": "explicit staging action and JSON required"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, rulesMaxBody)
		var req struct {
			Rule managedKnockRule `json:"rule"`
			ID   string           `json:"id"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			jsonReply(w, 400, map[string]string{"error": "invalid JSON body"})
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			jsonReply(w, 400, map[string]string{"error": "trailing JSON"})
			return
		}
		if r.Method == http.MethodPost {
			if err := validateManagedRule(req.Rule); err != nil {
				jsonReply(w, 400, map[string]string{"error": err.Error()})
				return
			}
			updated := false
			for i := range file.Rules {
				if file.Rules[i].ID == req.Rule.ID {
					file.Rules[i] = req.Rule
					updated = true
					break
				}
			}
			if !updated {
				if len(file.Rules) >= 32 {
					jsonReply(w, 400, map[string]string{"error": "rule limit reached"})
					return
				}
				file.Rules = append(file.Rules, req.Rule)
			}
		} else {
			if !ruleIDPattern.MatchString(req.ID) {
				jsonReply(w, 400, map[string]string{"error": "invalid rule id"})
				return
			}
			next := file.Rules[:0]
			for _, rule := range file.Rules {
				if rule.ID != req.ID {
					next = append(next, rule)
				}
			}
			file.Rules = next
		}
		sort.Slice(file.Rules, func(i, j int) bool { return strings.Compare(file.Rules[i].ID, file.Rules[j].ID) < 0 })
		if err := writeManagedRules(path, file); err != nil {
			jsonReply(w, 500, map[string]string{"error": "storage failure"})
			return
		}
		jsonReply(w, 200, map[string]any{"ok": true, "stored": len(file.Rules), "firewall_mutation": false})
	}
}
