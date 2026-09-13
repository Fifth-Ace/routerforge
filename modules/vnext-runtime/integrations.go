package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type integrationDescriptor struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	InitPaths    []string `json:"-"`
	ProcessNames []string `json:"-"`
	ConfigPaths  []string `json:"config_paths,omitempty"`
	WebHints     []string `json:"web_hints,omitempty"`
}

type integrationState struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Installed   bool     `json:"installed"`
	Running     bool     `json:"running"`
	InitScript  string   `json:"init_script,omitempty"`
	Processes   []string `json:"processes,omitempty"`
	ConfigPaths []string `json:"config_paths,omitempty"`
	WebHints    []string `json:"web_hints,omitempty"`
	Mode        string   `json:"mode"`
}

var knownIntegrations = []integrationDescriptor{
	{ID: "nfqws2", Name: "nfqws2", Category: "Network & Routing", InitPaths: []string{"/opt/etc/init.d/S51nfqws2", "/opt/etc/init.d/S99nfqws2"}, ProcessNames: []string{"nfqws", "nfqws2"}, ConfigPaths: []string{"/opt/etc/nfqws2", "/opt/etc/nfqws2.conf"}},
	{ID: "awg-manager", Name: "AWG Manager", Category: "Network & Routing", InitPaths: []string{"/opt/etc/init.d/S99awg-manager"}, ProcessNames: []string{"awg-manager"}, ConfigPaths: []string{"/opt/etc/awg-manager"}},
	{ID: "adguard-home", Name: "AdGuard Home", Category: "DNS", InitPaths: []string{"/opt/etc/init.d/S99adguardhome", "/opt/etc/init.d/S99AdGuardHome"}, ProcessNames: []string{"AdGuardHome"}, ConfigPaths: []string{"/opt/etc/AdGuardHome.yaml", "/opt/etc/AdGuardHome"}, WebHints: []string{"http://router:3000"}},
	{ID: "x-ui", Name: "3x-ui / x-ui", Category: "Management", InitPaths: []string{"/opt/etc/init.d/S99x-ui", "/opt/etc/init.d/S99xui"}, ProcessNames: []string{"x-ui"}, ConfigPaths: []string{"/opt/etc/x-ui"}},
}

func registerIntegrationsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		states := integrationSnapshot()
		installed := 0
		running := 0
		for _, state := range states {
			if state.Installed {
				installed++
			}
			if state.Running {
				running++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"known": len(states), "installed": installed, "running": running,
			"distribution_policy": "installed-only/external", "mutation_api": false,
		})
	}))
	mux.HandleFunc("/v1/integrations", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"integrations": integrationSnapshot()})
	}))
	mux.HandleFunc("/v1/nfqws2", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		for _, state := range integrationSnapshot() {
			if state.ID == "nfqws2" {
				writeJSON(w, http.StatusOK, map[string]any{
					"integration":  state,
					"profiles":     boundedNamedFiles("/opt/etc/nfqws2", 64),
					"strategy_lab": "deferred",
					"safe_apply":   "not-enabled-until-transaction-engine-consumer-gate",
				})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"installed": false})
	}))
}

func integrationSnapshot() []integrationState {
	processes := processSnapshot(4096)
	out := make([]integrationState, 0, len(knownIntegrations))
	for _, desc := range knownIntegrations {
		state := integrationState{ID: desc.ID, Name: desc.Name, Category: desc.Category, Mode: "installed-only", WebHints: desc.WebHints}
		for _, path := range desc.InitPaths {
			if fileExists(path) {
				state.Installed = true
				state.InitScript = path
				break
			}
		}
		for _, path := range desc.ConfigPaths {
			if fileExists(path) {
				state.Installed = true
				state.ConfigPaths = append(state.ConfigPaths, path)
			}
		}
		for _, processName := range desc.ProcessNames {
			for _, actual := range processes {
				if strings.EqualFold(processName, actual) || strings.Contains(strings.ToLower(actual), strings.ToLower(processName)) {
					state.Running = true
					state.Processes = appendUnique(state.Processes, actual)
				}
			}
		}
		out = append(out, state)
	}
	return out
}

func processSnapshot(limit int) []string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	out := make([]string, 0, 128)
	seen := map[string]bool{}
	for _, entry := range entries {
		if len(out) >= limit || !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(data))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func boundedNamedFiles(root string, limit int) []fileMeta {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []fileMeta{}
	}
	out := make([]fileMeta, 0, minInt(limit, len(entries)))
	for _, entry := range entries {
		if len(out) >= limit || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, fileMeta{Path: filepath.Join(root, entry.Name()), Size: info.Size(), Mode: info.Mode().String(), ModTime: info.ModTime()})
	}
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
