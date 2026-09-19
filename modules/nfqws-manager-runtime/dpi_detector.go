package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	dpiDetectorProbeTimeout = 2 * time.Second
	dpiDetectorOutputMax    = 4 << 10
)

var dpiDetectorCandidatePaths = []string{
	"/opt/bin/dpi-detector",
	"/usr/local/bin/dpi-detector",
	"/usr/bin/dpi-detector",
}

type dpiDetectorSnapshot struct {
	Installed             bool   `json:"installed"`
	ReadOnly              bool   `json:"read_only"`
	ManagedByRouterForge  bool   `json:"managed_by_routerforge"`
	Path                  string `json:"path,omitempty"`
	Version               string `json:"version,omitempty"`
	SizeBytes             int64  `json:"size_bytes,omitempty"`
	HostArch              string `json:"host_arch"`
	Upstream              string `json:"upstream"`
	VersionProbeAvailable bool   `json:"version_probe_available"`
	Warning               string `json:"warning,omitempty"`
}

func registerDPIDetectorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/dpi-detector", getOnly(handleDPIDetectorSnapshot))
}

func handleDPIDetectorSnapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readDPIDetectorSnapshot())
}

func readDPIDetectorSnapshot() dpiDetectorSnapshot {
	snapshot := dpiDetectorSnapshot{
		ReadOnly:              true,
		ManagedByRouterForge:  false,
		HostArch:              runtime.GOARCH,
		Upstream:              "Runnin4ik/dpi-detector",
		VersionProbeAvailable: false,
	}

	path, info := findDPIDetector(dpiDetectorCandidatePaths)
	if path == "" || info == nil {
		return snapshot
	}

	snapshot.Installed = true
	snapshot.Path = path
	snapshot.SizeBytes = info.Size()
	version, err := probeDPIDetectorVersion(path)
	if err != nil {
		snapshot.Warning = err.Error()
		return snapshot
	}
	snapshot.VersionProbeAvailable = true
	snapshot.Version = version
	return snapshot
}

func findDPIDetector(paths []string) (string, os.FileInfo) {
	for _, candidate := range paths {
		clean := filepath.Clean(candidate)
		if clean != candidate || filepath.Base(clean) != "dpi-detector" || !filepath.IsAbs(clean) {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
			continue
		}
		return clean, info
	}
	return "", nil
}

func probeDPIDetectorVersion(path string) (string, error) {
	if filepath.Base(path) != "dpi-detector" || !filepath.IsAbs(path) {
		return "", errors.New("dpi-detector version probe path rejected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), dpiDetectorProbeTimeout)
	defer cancel()
	output, err := safety.RunCommand(ctx, dpiDetectorOutputMax, path, "--version")
	if ctx.Err() != nil {
		return "", errors.New("dpi-detector --version timeout")
	}
	if err != nil {
		return "", errors.New("dpi-detector --version failed")
	}
	version := normalizeDPIDetectorVersion(string(output))
	if version == "" {
		return "", errors.New("dpi-detector --version returned no version")
	}
	return version, nil
}

func normalizeDPIDetectorVersion(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if len(line) > 160 {
			line = line[:160]
		}
		return line
	}
	return ""
}
