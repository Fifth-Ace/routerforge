package main

import (
	"context"
	"debug/elf"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	dpiDetectorProbeTimeout        = 2 * time.Second
	dpiDetectorOutputMax           = 4 << 10
	dpiDetectorInterpreterMaxBytes = 4 << 10
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
	ExecutionReady        bool   `json:"execution_ready"`
	Path                  string `json:"path,omitempty"`
	Version               string `json:"version,omitempty"`
	SizeBytes             int64  `json:"size_bytes,omitempty"`
	HostArch              string `json:"host_arch"`
	Upstream              string `json:"upstream"`
	VersionProbeAvailable bool   `json:"version_probe_available"`
	RequiredInterpreter   string `json:"required_interpreter,omitempty"`
	InterpreterAvailable  bool   `json:"interpreter_available"`
	CompatibilityReason   string `json:"compatibility_reason,omitempty"`
	Warning               string `json:"warning,omitempty"`
}

func registerDPIDetectorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/dpi-detector", getOnly(handleDPIDetectorSnapshot))
	mux.HandleFunc("/v1/dpi-detector/run", mutationOnly(handleDPIDetectorRun))
	mux.HandleFunc("/v1/dpi-detector/v5/run", mutationOnly(handleDPIDetectorV5Run))
	mux.HandleFunc("/v1/dpi-detector/v5/console", mutationOnly(handleDPIDetectorV5ConsoleRun))
	mux.HandleFunc("/v1/dpi-detector/v5/stream", mutationOnly(handleDPIDetectorV5StreamRun))
	mux.HandleFunc("/v1/dpi-detector/v5/legend", getOnly(handleDPIDetectorV5Legend))
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

	compatibility := readDPIDetectorCompatibility(path)
	snapshot.ExecutionReady = compatibility.ExecutionReady
	snapshot.RequiredInterpreter = compatibility.RequiredInterpreter
	snapshot.InterpreterAvailable = compatibility.InterpreterAvailable
	snapshot.CompatibilityReason = compatibility.Reason
	if !compatibility.ExecutionReady {
		snapshot.Warning = dpiDetectorCompatibilityWarning(compatibility)
		return snapshot
	}

	version, err := probeDPIDetectorVersion(path)
	if err != nil {
		snapshot.Warning = err.Error()
		return snapshot
	}
	snapshot.VersionProbeAvailable = true
	snapshot.Version = version
	return snapshot
}

type dpiDetectorCompatibility struct {
	ExecutionReady       bool
	RequiredInterpreter  string
	InterpreterAvailable bool
	Reason               string
}

func readDPIDetectorCompatibility(path string) dpiDetectorCompatibility {
	compatibility := dpiDetectorCompatibility{ExecutionReady: true}

	file, err := elf.Open(path)
	if err != nil {
		return compatibility
	}
	defer file.Close()

	for _, program := range file.Progs {
		if program.Type != elf.PT_INTERP {
			continue
		}

		data, err := io.ReadAll(io.LimitReader(program.Open(), dpiDetectorInterpreterMaxBytes))
		if err != nil {
			compatibility.ExecutionReady = false
			compatibility.Reason = "elf_interpreter_unreadable"
			return compatibility
		}

		interpreter := strings.TrimSpace(strings.TrimRight(string(data), "\x00"))
		compatibility.RequiredInterpreter = interpreter
		if interpreter == "" {
			compatibility.ExecutionReady = false
			compatibility.Reason = "elf_interpreter_empty"
			return compatibility
		}

		compatibility.InterpreterAvailable = dpiDetectorInterpreterAvailable(interpreter)
		if !compatibility.InterpreterAvailable {
			compatibility.ExecutionReady = false
			compatibility.Reason = "required_elf_interpreter_unavailable"
		}
		return compatibility
	}

	return compatibility
}

func dpiDetectorInterpreterAvailable(path string) bool {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0
}

func dpiDetectorCompatibilityWarning(compatibility dpiDetectorCompatibility) string {
	switch compatibility.Reason {
	case "required_elf_interpreter_unavailable":
		return "required ELF interpreter is unavailable: " + compatibility.RequiredInterpreter
	case "elf_interpreter_unreadable":
		return "ELF interpreter metadata is unreadable"
	case "elf_interpreter_empty":
		return "ELF interpreter metadata is empty"
	default:
		return compatibility.Reason
	}
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
