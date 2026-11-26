package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	so "github.com/Fifth-Ace/routerforge/internal/platform/serviceobserve"
	sb "github.com/Fifth-Ace/routerforge/internal/platform/supportbundle"
)

type serviceObservation struct {
	ObservedAt        time.Time       `json:"observed_at"`
	Module            string          `json:"module,omitempty"`
	ProcessState      string          `json:"process_state"`
	CPUPercent        *float64        `json:"cpu_percent"`
	CPUWindowMS       int64           `json:"cpu_window_ms,omitempty"`
	BootEligibility   string          `json:"boot_eligibility"`
	Dependencies      []so.Dependency `json:"dependencies"`
	DependenciesKnown bool            `json:"dependencies_known"`
}

var serviceObservationSampler so.Sampler
var serviceHealthSlot = make(chan struct{}, 1)

func serviceObservationExcluded(item serviceInspectorItem) bool {
	// The three maintenance-only security modules retain the previous view.
	s := strings.ToLower(item.ID + " " + item.Name)
	if item.Package != nil {
		s += " " + strings.ToLower(item.Package.Name)
	}
	for _, x := range []string{"antiscan", "crowdsec", "port-access"} {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
func serviceMarker(path string) *bool {
	st, e := os.Lstat(path)
	if os.IsNotExist(e) {
		v := false
		return &v
	}
	if e != nil || !st.Mode().IsRegular() {
		return nil
	}
	v := true
	return &v
}
func serviceBoot(item serviceInspectorItem, abi *bool) string {
	t, ok := so.KnownTemplate(item.ID)
	if !ok || item.InitScript != filepath.Join(serviceInitDir, item.ID) {
		return "unknown"
	}
	b, e := supportRead(item.InitScript, 16<<10, false)
	if e != nil {
		return "unknown"
	}
	// Check the exact packaged script, not the existence of a process or S prefix.
	var disabled *bool
	if t.DisableMarker {
		disabled = serviceMarker(filepath.Join(adminConfigVaultManagedRoot, "modules-disabled", t.Module))
	}
	return so.Boot(t, b, item.Executable, abi, disabled)
}
func serviceDependencyInventory() map[string][]so.Dependency {
	for _, path := range []string{"/opt/lib/opkg/status", "/opt/var/opkg/status"} {
		if raw, e := supportRead(path, 2<<20, false); e == nil {
			if d := so.Dependencies(raw); len(d) > 0 {
				return d
			}
		}
	}
	return nil
}
func servicePIDList(item serviceInspectorItem) []int {
	p := make([]int, 0, len(item.Processes))
	for _, v := range item.Processes {
		if v.PID > 0 {
			p = append(p, v.PID)
		}
	}
	return p
}
func addServiceObservations(items []serviceInspectorItem) {
	// Reuse the inspector's existing process correlations, never scan all of /proc
	// again. Limit the extra reads even on a router with many packages/services.
	ids := map[int]bool{}
	overflow := false
	for _, item := range items {
		if serviceObservationExcluded(item) {
			continue
		}
		for _, p := range servicePIDList(item) {
			if len(ids) >= so.MaxPIDs && !ids[p] {
				overflow = true
				break
			}
			ids[p] = true
		}
		if overflow {
			break
		}
	}
	sample := so.Sample{Procs: map[int]so.Proc{}}
	if !overflow {
		if b, e := supportRead("/proc/stat", 64<<10, false); e == nil {
			sample.Total, _ = so.ParseTotal(string(b))
		}
		for pid := range ids {
			if b, e := supportRead(filepath.Join("/proc", strconv.Itoa(pid), "stat"), 8192, false); e == nil {
				if p, ok := so.ParseProc(string(b), pid); ok {
					sample.Procs[pid] = p
				}
			}
		}
	}
	sample.At = time.Now().UTC()
	delta := serviceObservationSampler.Observe(sample)
	deps := serviceDependencyInventory()
	abi := serviceMarker(filepath.Join(adminConfigVaultManagedRoot, "module-abi-v1"))
	for i := range items {
		item := &items[i]
		if serviceObservationExcluded(*item) {
			continue
		}
		pids := servicePIDList(*item)
		o := &serviceObservation{ObservedAt: sample.At, Module: so.Module(item.ID), ProcessState: so.Presence(pids, sample.Procs), BootEligibility: serviceBoot(*item, abi), Dependencies: []so.Dependency{}}
		if item.Package != nil {
			if d, ok := deps[item.Package.Name]; ok {
				o.Dependencies = d
				o.DependenciesKnown = true
			}
		}
		if !overflow {
			o.CPUPercent = delta.Percent(pids)
			if o.CPUPercent != nil {
				o.CPUWindowMS = delta.WindowMS
			}
		}
		item.Observation = o
	}
}

// Native health is on demand, using E07's fixed Unix socket adapter only. The
// user cannot pass a path, URL, command or arbitrary module. A separate service
// action API already exists and is unchanged.
func serviceHealthModule(id string) string {
	m := so.Module(id)
	for _, v := range sb.ProbeIDs() {
		if m == v {
			return m
		}
	}
	return ""
}
func serviceHealthHandler(probe func(context.Context, string) sb.Health, exists func(string) bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeJSON(w, 405, map[string]string{"error": "GET_REQUIRED"})
			return
		}
		q := r.URL.Query()
		id := q.Get("id")
		if len(q) != 1 || len(q["id"]) != 1 || len(id) > 96 || !validServiceID(id) {
			writeJSON(w, 400, map[string]string{"error": "INVALID_SERVICE"})
			return
		}
		module := serviceHealthModule(id)
		if module == "" {
			writeJSON(w, 409, map[string]string{"error": "NATIVE_HEALTH_UNSUPPORTED"})
			return
		}
		if !exists(id) {
			writeJSON(w, 404, map[string]string{"error": "SERVICE_NOT_FOUND"})
			return
		}
		select {
		case serviceHealthSlot <- struct{}{}:
			defer func() { <-serviceHealthSlot }()
		default:
			writeJSON(w, 429, map[string]string{"error": "HEALTH_BUSY"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		observed := probe(ctx, module)
		writeJSON(w, 200, map[string]any{"service_id": id, "observed_at": time.Now().UTC(), "scope": "native-management-endpoint", "health": observed})
	}
}
func serviceHealthExists(id string) bool {
	st, e := os.Lstat(filepath.Join(serviceInitDir, id))
	return e == nil && st.Mode().IsRegular()
}
func registerServiceObservationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/service-inspector/health", serviceHealthHandler(supportModuleHealth, serviceHealthExists))
}
