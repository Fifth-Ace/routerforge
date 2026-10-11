package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// This controls an existing Entware daemon; it never generates firewall rules.
var engineServiceNames = map[string]string{
	"knockd":  "knockd",
	"fwknopd": "fwknopd",
}

type engineService struct {
	Engine         string `json:"engine"`
	Installed      bool   `json:"installed"`
	Script         string `json:"script,omitempty"`
	Running        bool   `json:"running"`
	FirewallPolicy string `json:"firewall_policy"`
}

func findEngineInitScript(engine, dir string) (string, error) {
	name, ok := engineServiceNames[engine]
	if !ok {
		return "", errors.New("unknown engine")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	found := ""
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if !strings.HasPrefix(n, "s") || !strings.Contains(n, name) {
			continue
		}
		if len(n) < 3 || n[1] < '0' || n[1] > '9' || n[2] < '0' || n[2] > '9' {
			continue
		}
		path := filepath.Join(dir, e.Name())
		st, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !st.Mode().IsRegular() || st.Mode().Perm()&0111 == 0 {
			continue
		}
		if found != "" {
			return "", errors.New("multiple matching init scripts")
		}
		found = path
	}
	if found == "" {
		return "", errors.New("installed Entware init script not found")
	}
	return found, nil
}

func serviceSnapshot(engine, dir string) engineService {
	script, err := findEngineInitScript(engine, dir)
	result := engineService{Engine: engine, Installed: err == nil, Running: runningProcesses()[engine], FirewallPolicy: "preserve"}
	if err == nil {
		result.Script = filepath.Base(script)
	}
	return result
}

func runExistingEngineService(ctx context.Context, engine, action, dir string, invoke func(context.Context, string, string) error) error {
	if action != "start" && action != "stop" && action != "restart" {
		return errors.New("unsupported service action")
	}
	if invoke == nil {
		return errors.New("missing command runner")
	}
	script, err := findEngineInitScript(engine, dir)
	if err != nil {
		return err
	}
	if action != "stop" {
		cfg := map[string]string{"knockd": "/opt/etc/knockd.conf", "fwknopd": "/opt/etc/fwknop/access.conf"}[engine]
		info, err := os.Lstat(cfg)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("service configuration missing")
		}
	}
	return invoke(ctx, script, action)
}

func invokeEntwareService(ctx context.Context, path, action string) error {
	cmd, err := safety.CommandContext(ctx, path, action)
	if err != nil {
		return errors.New("Entware service operation failed")
	}
	cmd.Env = []string{"PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = out
		return errors.New("Entware service operation failed")
	}
	return nil
}

// Install only a fixed upstream package. Never implicitly start a daemon or modify
// RouterForge firewall chains. opkg package scripts remain upstream-controlled.
func installExistingEngine(ctx context.Context, engine string, invoke func(context.Context, string) error) error {
	name, ok := engineServiceNames[engine]
	if !ok {
		return errors.New("unknown engine")
	}
	if invoke == nil {
		return errors.New("missing package runner")
	}
	return invoke(ctx, name)
}

// Rebuild a temporary opkg source list with HTTPS for trusted Entware feeds.
// The system configuration and additional third-party feed addresses stay unchanged.
func entwareHTTPSConfigLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) != 3 || (fields[0] != "src/gz" && fields[0] != "src") {
		return line
	}
	if !strings.HasPrefix(fields[2], "http://bin.entware.net/") {
		return line
	}
	return strings.Replace(line, fields[2], "https://"+strings.TrimPrefix(fields[2], "http://"), 1)
}

func createEntwareHTTPSConfig(root, tmp string) (string, error) {
	paths := []string{filepath.Join(root, "opkg.conf")}
	fragments, err := filepath.Glob(filepath.Join(root, "opkg", "*.conf"))
	if err != nil {
		return "", err
	}
	paths = append(paths, fragments...)
	var result strings.Builder
	found := false
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) && i != 0 {
			continue
		}
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "http://bin.entware.net/") && entwareHTTPSConfigLine(line) != line {
				found = true
			}
			result.WriteString(entwareHTTPSConfigLine(line))
			result.WriteByte('\n')
		}
	}
	if !found {
		return "", errors.New("Entware HTTP source not found: HTTPS installation unavailable")
	}
	file, err := os.CreateTemp(tmp, "routerforge-opkg-https-*.conf")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err = file.Chmod(0600); err == nil {
		_, err = file.WriteString(result.String())
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func invokeEntwareInstall(ctx context.Context, name string) error {
	config, err := createEntwareHTTPSConfig("/opt/etc", "/opt/tmp")
	if err != nil {
		return fmt.Errorf("Entware HTTPS configuration: %w", err)
	}
	defer os.Remove(config)
	cmd, err := safety.CommandContext(ctx, "/opt/bin/opkg", "-f", config, "install", name)
	if err != nil {
		return errors.New("unable to launch Entware package manager")
	}
	cmd.Env = []string{"PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Entware HTTPS installation failed: %s", limitedInstallError(out))
	}
	return nil
}

func limitedInstallError(out []byte) string {
	// Do not send arbitrary package output or private configuration into the UI.
	if len(out) == 0 {
		return "check Entware package feeds and logs"
	}
	return "check Entware package feeds, available storage and logs"
}

func engineServiceHandler(w http.ResponseWriter, r *http.Request) {
	engine := r.URL.Query().Get("engine")
	if _, ok := engineServiceNames[engine]; !ok {
		jsonReply(w, 400, map[string]string{"error": "unknown engine"})
		return
	}
	if r.Method == http.MethodGet {
		result := serviceSnapshot(engine, "/opt/etc/init.d")
		install := engineInstalls.status(engine)
		jsonReply(w, 200, map[string]any{"engine": result.Engine, "installed": result.Installed, "script": result.Script, "running": result.Running, "firewall_policy": result.FirewallPolicy, "installation": install})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		jsonReply(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
		jsonReply(w, 403, map[string]string{"error": "Core authorization required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var request struct {
		Action         string `json:"action"`
		Confirm        string `json:"confirm"`
		FirewallPolicy string `json:"firewall_policy"`
	}
	if err := decodeEngineAction(r.Body, &request); err != nil {
		jsonReply(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if request.Confirm != "APPLY" {
		jsonReply(w, 400, map[string]string{"error": "confirmation required"})
		return
	}
	// Existing upstream init scripts may mutate firewall internally. RouterForge never
	// performs extra revocation on stop; cleanup requires separate ownership proofs.
	if request.FirewallPolicy != "" && request.FirewallPolicy != "preserve" {
		jsonReply(w, 409, map[string]string{"error": "firewall cleanup is unavailable: ownership of daemon rules is not verified"})
		return
	}
	if request.Action == "install" {
		if request.FirewallPolicy != "" && request.FirewallPolicy != "preserve" {
			jsonReply(w, 409, map[string]string{"error": "firewall cleanup is unavailable"})
			return
		}
		if serviceSnapshot(engine, "/opt/etc/init.d").Installed {
			jsonReply(w, 409, map[string]string{"error": "service already installed"})
			return
		}
		if err := engineInstalls.start(engine, invokeEntwareInstall); err != nil {
			jsonReply(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonReply(w, 202, map[string]any{"ok": true, "engine": engine, "action": "install", "installation": engineInstalls.status(engine), "postinst_may_start_service": true})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := runExistingEngineService(ctx, engine, request.Action, "/opt/etc/init.d", invokeEntwareService); err != nil {
		jsonReply(w, 409, map[string]string{"error": err.Error()})
		return
	}
	jsonReply(w, 200, map[string]any{"ok": true, "engine": engine, "action": request.Action, "firewall_policy": "preserve", "service": serviceSnapshot(engine, "/opt/etc/init.d")})
}
