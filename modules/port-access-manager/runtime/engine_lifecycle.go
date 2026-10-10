package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// This controls an existing Entware daemon; it never generates firewall rules.
var engineServiceNames = map[string]string{
	"knockd":  "knockd",
	"fwknopd": "fwknopd",
}

type engineService struct {
	Engine    string `json:"engine"`
	Installed bool   `json:"installed"`
	Script    string `json:"script,omitempty"`
	Running   bool   `json:"running"`
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
	result := engineService{Engine: engine, Installed: err == nil, Running: runningProcesses()[engine]}
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
	cmd := exec.CommandContext(ctx, path, action)
	cmd.Env = []string{"PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = out
		return errors.New("Entware service operation failed")
	}
	return nil
}

func engineServiceHandler(w http.ResponseWriter, r *http.Request) {
	engine := r.URL.Query().Get("engine")
	if _, ok := engineServiceNames[engine]; !ok {
		jsonReply(w, 400, map[string]string{"error": "unknown engine"})
		return
	}
	if r.Method == http.MethodGet {
		jsonReply(w, 200, serviceSnapshot(engine, "/opt/etc/init.d"))
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
		Action  string `json:"action"`
		Confirm string `json:"confirm"`
	}
	if err := decodeEngineAction(r.Body, &request); err != nil {
		jsonReply(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if request.Confirm != "APPLY" {
		jsonReply(w, 400, map[string]string{"error": "confirmation required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := runExistingEngineService(ctx, engine, request.Action, "/opt/etc/init.d", invokeEntwareService); err != nil {
		jsonReply(w, 409, map[string]string{"error": err.Error()})
		return
	}
	jsonReply(w, 200, map[string]any{"ok": true, "engine": engine, "action": request.Action, "service": serviceSnapshot(engine, "/opt/etc/init.d")})
}
