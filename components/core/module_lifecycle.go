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

const moduleLifecycleCommandOutputLimit = 16 << 10

type moduleLifecycleSpec struct {
	ID            string
	Services      []string
	ProcessNames  []string
	EnableMarkers []string
}

var moduleLifecycleSpecs = map[string]moduleLifecycleSpec{
	"admin": {
		ID: "admin",
		Services: []string{
			"/opt/etc/init.d/S91routerforge-admin",
			"/opt/etc/init.d/S91dns-monitor-admin",
		},
		ProcessNames: []string{"routerforge-admin", "dnsmon-admin"},
	},
	"dns": {
		ID:            "dns",
		Services:      []string{"/opt/etc/init.d/S91routerforge-dns"},
		ProcessNames:  []string{"routerforge-dns"},
		EnableMarkers: []string{"/opt/etc/routerforge/dns.enabled"},
	},
	"monitoring": {
		ID:           "monitoring",
		Services:     []string{"/opt/etc/init.d/S92routerforge-monitoring"},
		ProcessNames: []string{"routerforge-monitoring"},
	},
	"network-tools": {
		ID:           "network-tools",
		Services:     []string{"/opt/etc/init.d/S97routerforge-network-tools"},
		ProcessNames: []string{"routerforge-network-tools"},
	},
	"nfqws-manager": {
		ID:           "nfqws-manager",
		Services:     []string{"/opt/etc/init.d/S96routerforge-nfqws-manager"},
		ProcessNames: []string{"routerforge-nfqws-manager"},
	},
	"antiscan-manager": {
		ID:           "antiscan-manager",
		Services:     []string{"/opt/etc/init.d/S98routerforge-antiscan-manager"},
		ProcessNames: []string{"routerforge-antiscan-manager"},
	},
}

var moduleLifecycleDisabledRoot = "/opt/etc/routerforge/modules-disabled"

type moduleLifecycleFailure struct {
	Status int
	Detail string
}

func (e *moduleLifecycleFailure) Error() string {
	return e.Detail
}

func moduleLifecycleSpecFor(id string) (moduleLifecycleSpec, bool) {
	spec, ok := moduleLifecycleSpecs[strings.ToLower(strings.TrimSpace(id))]
	return spec, ok
}

func moduleLifecycleDisabledMarker(id string) string {
	if _, ok := moduleLifecycleSpecFor(id); !ok {
		return ""
	}
	return filepath.Join(moduleLifecycleDisabledRoot, strings.ToLower(strings.TrimSpace(id)))
}

func moduleLifecycleServicePath(spec moduleLifecycleSpec, exists func(string) bool) string {
	for _, service := range spec.Services {
		if exists(service) {
			return service
		}
	}
	if len(spec.Services) > 0 {
		return spec.Services[0]
	}
	return ""
}

func moduleLifecycleRuntimeState(spec moduleLifecycleSpec, processes map[string]bool) bool {
	for _, name := range spec.ProcessNames {
		if processes[strings.ToLower(strings.TrimSpace(name))] {
			return true
		}
	}
	return false
}

func moduleLifecycleSetEnableMarkers(spec moduleLifecycleSpec, enabled bool) error {
	for _, marker := range spec.EnableMarkers {
		if enabled {
			if err := os.MkdirAll(filepath.Dir(marker), 0755); err != nil {
				return err
			}
			if err := safety.WriteFileAtomic(marker, nil, 0644); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func moduleLifecycleSetDisabled(id string, disabled bool) error {
	marker := moduleLifecycleDisabledMarker(id)
	if marker == "" {
		return fmt.Errorf("module lifecycle is not supported for %q", id)
	}
	if disabled {
		if err := os.MkdirAll(moduleLifecycleDisabledRoot, 0755); err != nil {
			return err
		}
		return safety.WriteFileAtomic(marker, []byte("disabled\n"), 0644)
	}
	if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func moduleLifecycleRunInit(ctx context.Context, service string, action string) (string, error) {
	info, err := os.Stat(service)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Mode()&0111 == 0 {
		return "", fmt.Errorf("service script is not executable: %s", service)
	}
	output, runErr := safety.RunCommand(ctx, moduleLifecycleCommandOutputLimit, service, action)
	return strings.TrimSpace(string(output)), runErr
}

func moduleLifecycleWait(spec moduleLifecycleSpec, running bool, timeout time.Duration) bool {
	if len(spec.ProcessNames) == 0 {
		return running
	}
	deadline := time.Now().Add(timeout)
	for {
		current := moduleLifecycleRuntimeState(spec, readProcessNames())
		if current == running {
			return current
		}
		if time.Now().After(deadline) {
			return current
		}
		time.Sleep(125 * time.Millisecond)
	}
}

func runModuleLifecycleAction(ctx context.Context, id, action string) (map[string]any, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	action = strings.ToLower(strings.TrimSpace(action))

	spec, ok := moduleLifecycleSpecFor(id)
	if !ok {
		return nil, &moduleLifecycleFailure{Status: http.StatusBadRequest, Detail: "module lifecycle is not supported"}
	}
	if id == "routerforge-core" {
		return nil, &moduleLifecycleFailure{Status: http.StatusForbidden, Detail: "RouterForge Core lifecycle cannot be controlled from App Center"}
	}

	snapshot := refreshCatalog()
	var item *catalogItem
	for i := range snapshot.Modules {
		if snapshot.Modules[i].ID == id {
			item = &snapshot.Modules[i]
			break
		}
	}
	if item == nil || !item.Installed {
		return nil, &moduleLifecycleFailure{Status: http.StatusNotFound, Detail: "module is not installed"}
	}

	service := moduleLifecycleServicePath(spec, pathExists)
	if service == "" {
		return nil, &moduleLifecycleFailure{Status: http.StatusNotFound, Detail: "module service script is not available"}
	}

	disabledMarker := moduleLifecycleDisabledMarker(id)
	disabled := disabledMarker != "" && pathExists(disabledMarker)

	var (
		output string
		err    error
	)

	switch action {
	case "start":
		if disabled {
			return nil, &moduleLifecycleFailure{Status: http.StatusConflict, Detail: "module is disabled; enable it first"}
		}
		if markerErr := moduleLifecycleSetEnableMarkers(spec, true); markerErr != nil {
			return nil, markerErr
		}
		output, err = moduleLifecycleRunInit(ctx, service, "start")
		if err == nil {
			moduleLifecycleWait(spec, true, 4*time.Second)
		}

	case "restart":
		if disabled {
			return nil, &moduleLifecycleFailure{Status: http.StatusConflict, Detail: "module is disabled; enable it first"}
		}
		if markerErr := moduleLifecycleSetEnableMarkers(spec, true); markerErr != nil {
			return nil, markerErr
		}
		output, err = moduleLifecycleRunInit(ctx, service, "restart")
		if err == nil {
			moduleLifecycleWait(spec, true, 4*time.Second)
		}

	case "stop":
		output, err = moduleLifecycleRunInit(ctx, service, "stop")
		if err == nil {
			moduleLifecycleWait(spec, false, 4*time.Second)
		}

	case "disable":
		output, err = moduleLifecycleRunInit(ctx, service, "stop")
		if err != nil {
			break
		}
		moduleLifecycleWait(spec, false, 4*time.Second)
		if err = moduleLifecycleSetDisabled(id, true); err != nil {
			break
		}
		err = moduleLifecycleSetEnableMarkers(spec, false)

	case "enable":
		if err = moduleLifecycleSetDisabled(id, false); err != nil {
			break
		}
		if err = moduleLifecycleSetEnableMarkers(spec, true); err != nil {
			break
		}
		output, err = moduleLifecycleRunInit(ctx, service, "start")
		if err == nil {
			moduleLifecycleWait(spec, true, 4*time.Second)
		}

	default:
		return nil, &moduleLifecycleFailure{Status: http.StatusBadRequest, Detail: "unsupported module lifecycle action"}
	}

	if err != nil {
		detail := err.Error()
		if output != "" {
			detail += ": " + output
		}
		return nil, &moduleLifecycleFailure{Status: http.StatusInternalServerError, Detail: detail}
	}

	next := refreshCatalog()
	var current *catalogItem
	for i := range next.Modules {
		if next.Modules[i].ID == id {
			current = &next.Modules[i]
			break
		}
	}

	result := map[string]any{
		"ok":      true,
		"id":      id,
		"action":  action,
		"service": service,
		"output":  output,
		"catalog": next,
	}
	if current != nil {
		result["running"] = current.ServiceRunning
		result["disabled"] = current.Disabled
	}
	return result, nil
}

func registerModuleLifecycleHandlers(mux *http.ServeMux, auth *authManager) {
	mux.HandleFunc("/api/apps/modules/lifecycle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
			return
		}
		if !sameOriginRequest(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin module lifecycle request rejected"})
			return
		}
		if auth.authRequired() {
			user, authenticated := auth.sessionUser(r)
			if !authenticated || user != "root" {
				writeCatalogJSON(w, http.StatusUnauthorized, map[string]any{
					"error":         "authenticated RouterForge root session required for module lifecycle",
					"auth_required": true,
				})
				return
			}
		}

		var request struct {
			ID        string `json:"id"`
			Action    string `json:"action"`
			ConfirmID string `json:"confirm_id"`
		}
		if err := decodeSmallJSON(w, r, &request); err != nil {
			return
		}
		request.ID = strings.ToLower(strings.TrimSpace(request.ID))
		request.Action = strings.ToLower(strings.TrimSpace(request.Action))
		if request.ID == "" || request.ConfirmID != request.ID {
			writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "exact module confirmation is required"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		result, err := runModuleLifecycleAction(ctx, request.ID, request.Action)
		if err != nil {
			status := http.StatusInternalServerError
			if failure, ok := err.(*moduleLifecycleFailure); ok && failure.Status > 0 {
				status = failure.Status
			}
			writeCatalogJSON(w, status, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusOK, result)
	})
}
