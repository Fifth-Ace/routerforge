package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type EngineStatus struct {
	ID                   string `json:"id"`
	Installed            bool   `json:"installed"`
	Binary               string `json:"binary,omitempty"`
	PackageInstalled     bool   `json:"package_installed"`
	Running              bool   `json:"running"`
	ConfigurationPresent bool   `json:"configuration_present"`
	ServicePresent       bool   `json:"service_present"`
}

type Status struct {
	Module      string         `json:"module"`
	Version     string         `json:"version"`
	Mode        string         `json:"mode"`
	MutationAPI bool           `json:"mutation_api"`
	Engines     []EngineStatus `json:"engines"`
}

type engineSpec struct{ id, binary, packageName, configuration string }

var specs = []engineSpec{
	{"knockd", "knockd", "knockd", "/opt/etc/knockd.conf"},
	{"fwknopd", "fwknopd", "fwknopd", "/opt/etc/fwknop/access.conf"},
}

// Only read installed package records. Do not update package feeds, launch daemons,
// inspect secret values, or make firewall changes in K1.
func packageSet() map[string]bool {
	out := map[string]bool{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := safety.RunCommandOutput(ctx, "/opt/bin/opkg", "list-installed")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "-" {
			out[fields[0]] = true
		}
	}
	return out
}

func serviceExists(prefix string) bool {
	entries, err := os.ReadDir("/opt/etc/init.d")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name()), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func runningProcesses() map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/proc", name, "comm"))
		if err != nil {
			continue
		}
		out[strings.TrimSpace(string(b))] = true
	}
	return out
}

// recent is a kernel match, not an Entware daemon. Check capability without
// loading modules or touching the firewall. No supported engine is activated.
func kernelMatchPresent(path, match string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, field := range strings.Fields(string(b)) {
		if field == match {
			return true
		}
	}
	return false
}

func snapshot() Status {
	pkgs := packageSet()
	procs := runningProcesses()
	result := Status{Module: "port-access-manager", Version: version, Mode: "read-only-discovery", MutationAPI: false}
	for _, spec := range specs {
		binary, err := exec.LookPath(spec.binary)
		_, confErr := os.Stat(spec.configuration)
		result.Engines = append(result.Engines, EngineStatus{
			ID: spec.id, Installed: err == nil || pkgs[spec.packageName], Binary: binary,
			PackageInstalled: pkgs[spec.packageName], Running: procs[spec.id],
			ConfigurationPresent: confErr == nil, ServicePresent: serviceExists(spec.id),
		})
	}
	// Expose the daemon-free community approach as a separate, read-only adapter.
	// "Installed" means kernel match capability, not active protection.
	result.Engines = append(result.Engines, EngineStatus{
		ID: "iptables-recent", Installed: kernelMatchPresent("/proc/net/ip_tables_matches", "recent"),
	})
	return result
}
