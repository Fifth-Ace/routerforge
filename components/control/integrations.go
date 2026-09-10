package main

import (
	"net/http"
	"os"
	"strings"
)

type adminIntegrationInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Detected    bool     `json:"detected"`
	Running     bool     `json:"running"`
	ServiceID   string   `json:"service_id,omitempty"`
	ProcessPIDs []int    `json:"process_pids,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	Ports       []int    `json:"ports,omitempty"`
}

type adminIntegrationDefinition struct {
	ID             string
	Name           string
	PathCandidates []string
	ProcessTokens  []string
	ServiceTokens  []string
}

var adminIntegrationDefinitions = []adminIntegrationDefinition{
	{
		ID:   "nfqws2",
		Name: "nfqws2",
		PathCandidates: []string{
			"/opt/etc/init.d/S51nfqws2",
			"/opt/usr/bin/nfqws2",
			"/opt/bin/nfqws2",
			"/opt/etc/nfqws2",
		},
		ProcessTokens: []string{"nfqws2", "nfqws"},
		ServiceTokens: []string{"nfqws2"},
	},
	{
		ID:   "awg-manager",
		Name: "AWG Manager",
		PathCandidates: []string{
			"/opt/etc/init.d/S99awg-manager",
			"/opt/bin/awg-manager",
			"/opt/etc/awg-manager",
		},
		ProcessTokens: []string{"awg-manager"},
		ServiceTokens: []string{"awg-manager"},
	},
	{
		ID:   "adguard-home",
		Name: "AdGuard Home",
		PathCandidates: []string{
			"/opt/etc/init.d/S99adguardhome",
			"/opt/etc/init.d/S99AdGuardHome",
			"/opt/AdGuardHome/AdGuardHome",
			"/opt/bin/AdGuardHome",
			"/opt/etc/AdGuardHome.yaml",
		},
		ProcessTokens: []string{"adguardhome", "adguard home"},
		ServiceTokens: []string{"adguardhome", "adguard-home"},
	},
}

func registerAdminIntegrationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/integrations", getOnly(handleAdminIntegrations))
}

func handleAdminIntegrations(w http.ResponseWriter, _ *http.Request) {
	processes := readProcesses()
	services := readServices()
	ports := readPorts()

	result := make([]adminIntegrationInfo, 0, len(adminIntegrationDefinitions))
	for _, definition := range adminIntegrationDefinitions {
		result = append(result, detectAdminIntegration(definition, processes, services, ports))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"integrations": result,
		"mode":         "read-only-detection",
	})
}

func detectAdminIntegration(
	definition adminIntegrationDefinition,
	processes []processInfo,
	services []serviceInfo,
	ports []portInfo,
) adminIntegrationInfo {
	info := adminIntegrationInfo{
		ID:   definition.ID,
		Name: definition.Name,
	}

	for _, candidate := range definition.PathCandidates {
		if _, err := os.Stat(candidate); err == nil {
			info.Paths = append(info.Paths, candidate)
		}
	}

	processNames := make(map[string]struct{})
	for _, process := range processes {
		corpus := strings.ToLower(process.Name + " " + process.Command)
		if containsAnyToken(corpus, definition.ProcessTokens) {
			info.ProcessPIDs = append(info.ProcessPIDs, process.PID)
			processNames[strings.ToLower(process.Name)] = struct{}{}
			info.Running = true
		}
	}

	for _, service := range services {
		corpus := strings.ToLower(service.ID + " " + service.Name + " " + service.Path)
		if containsAnyToken(corpus, definition.ServiceTokens) {
			if info.ServiceID == "" {
				info.ServiceID = service.ID
			}
			if service.Running {
				info.Running = true
			}
		}
	}

	seenPorts := make(map[int]struct{})
	for _, port := range ports {
		if port.LocalPort <= 0 {
			continue
		}
		process := strings.ToLower(port.Process)
		if _, ok := processNames[process]; !ok && !containsAnyToken(process, definition.ProcessTokens) {
			continue
		}
		if _, ok := seenPorts[port.LocalPort]; ok {
			continue
		}
		seenPorts[port.LocalPort] = struct{}{}
		info.Ports = append(info.Ports, port.LocalPort)
	}

	info.Detected = len(info.Paths) > 0 || len(info.ProcessPIDs) > 0 || info.ServiceID != ""
	return info
}

func containsAnyToken(corpus string, tokens []string) bool {
	corpus = strings.ToLower(corpus)
	for _, token := range tokens {
		if token != "" && strings.Contains(corpus, strings.ToLower(token)) {
			return true
		}
	}
	return false
}
