package dnsrouter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxAPIBytes int64 = MaxConfigBytes

type API struct {
	Store     *ConfigStore
	UI        string
	Shadow    *ShadowForwarder
	Lifecycle *LifecycleManager
	TrialM    *NDMCTrialM
	ManagedN  *ManagedTrialN
}

func NewAPI(store *ConfigStore, ui string) *API { return &API{Store: store, UI: ui} }

func respondJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, map[string]any{"error": err.Error()})
}
func requireMutation(w http.ResponseWriter, r *http.Request, action string) bool {
	if r.Header.Get("X-RouterForge-Action") != action {
		fail(w, http.StatusForbidden, errors.New("X-RouterForge-Action header required"))
		return false
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		fail(w, http.StatusUnsupportedMediaType, errors.New("application/json required"))
		return false
	}
	return true
}
func decodeBody(r *http.Request, dest any, limit int64) error {
	if r.ContentLength > limit {
		return errors.New("payload too large")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, limit+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON objects are not permitted")
		}
		return err
	}
	return nil
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		cfg, rev, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		active := a.Shadow != nil && a.Shadow.Active()
		if a.Lifecycle != nil {
			active = a.Lifecycle.Status().Active
		}
		state := "inactive"
		if active {
			state = "shadow-loopback"
		}
		respondJSON(w, 200, map[string]any{"component": "dns-router", "status": state, "dns_listener": active, "dns_interception": false, "installed_dns_manager_required": false, "config_revision": rev, "rules": len(cfg.Rules), "upstreams": len(cfg.Upstreams), "time": time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("/v1/shadow/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		if a.Lifecycle != nil {
			status := a.Lifecycle.Status()
			var stats any = nil
			if f := a.Lifecycle.Forwarder(); f != nil {
				stats = f.Stats()
			}
			respondJSON(w, 200, map[string]any{"lifecycle": status, "stats": stats, "interception": false})
			return
		}
		if a.Shadow == nil {
			respondJSON(w, 200, map[string]any{"active": false, "interception": false})
			return
		}
		respondJSON(w, 200, map[string]any{"active": a.Shadow.Active(), "interception": false, "stats": a.Shadow.Stats()})
	})
	mux.HandleFunc("/v1/upstreams/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		cfg, rev, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		ids := make([]string, 0, len(cfg.Upstreams))
		for _, u := range cfg.Upstreams {
			ids = append(ids, u.ID)
		}
		var usage any = map[string]UpstreamStats{}
		if a.Shadow != nil {
			usage = a.Shadow.Stats().Upstreams
		}
		if a.Lifecycle != nil {
			if f := a.Lifecycle.Forwarder(); f != nil {
				usage = f.Stats().Upstreams
			}
		}
		respondJSON(w, 200, map[string]any{"upstream_ids": ids, "stats": usage, "revision": rev, "passive_only": true, "probe_network": false, "dns_interception": false})
	})
	// Runtime mutations are opt-in, revision-pinned, and affect loopback only.
	mux.HandleFunc("/v1/shadow/control", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		if a.Lifecycle == nil {
			fail(w, 503, errors.New("manual lifecycle is unavailable"))
			return
		}
		var req struct {
			Operation string `json:"operation"`
		}
		if !requireMutation(w, r, "shadow-control") {
			return
		}
		if err := decodeBody(r, &req, 1024); err != nil {
			fail(w, 400, err)
			return
		}
		expected := strings.Trim(r.Header.Get("If-Match"), "\"")
		if req.Operation != "stop" && expected == "" {
			fail(w, 428, errors.New("If-Match required"))
			return
		}
		var err error
		outcome := ""
		switch req.Operation {
		case "start":
			err = a.Lifecycle.Start(expected)
			outcome = "started"
		case "stop":
			err = a.Lifecycle.Stop()
			outcome = "stopped"
		case "apply":
			outcome, err = a.Lifecycle.Apply(expected)
		default:
			fail(w, 400, errors.New("invalid lifecycle operation"))
			return
		}
		if err != nil {
			code := 400
			if errors.Is(err, ErrStale) {
				code = 409
			}
			if errors.Is(err, ErrAlreadyActive) || errors.Is(err, ErrInactive) {
				code = 409
			}
			respondJSON(w, code, map[string]any{"error": err.Error(), "outcome": outcome, "lifecycle": a.Lifecycle.Status(), "interception": false})
			return
		}
		respondJSON(w, 200, map[string]any{"outcome": outcome, "lifecycle": a.Lifecycle.Status(), "interception": false})
	})
	// DPR2-N: explicit one-shot multi-zone trial; read-only prepare, followed
	// by a separately confirmed CAS+token trial. A trial ALWAYS rolls back.
	for _, spec := range []struct{ path, action, operation string }{
		{"/v1/keenetic/domain-managed/prepare", "managed-prepare", "prepare"},
		{"/v1/keenetic/domain-managed/trial", "managed-trial", "trial"},
	} {
		spec := spec
		mux.HandleFunc(spec.path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				w.WriteHeader(405)
				return
			}
			if !requireMutation(w, r, spec.action) {
				return
			}
			if a.ManagedN == nil {
				fail(w, 503, errors.New("DPR2-N managed trial is disabled"))
				return
			}
			var req struct {
				Operation string `json:"operation"`
				Token     string `json:"token,omitempty"`
			}
			if err := decodeBody(r, &req, 1024); err != nil {
				fail(w, 400, err)
				return
			}
			if req.Operation != spec.operation {
				fail(w, 400, errors.New("invalid managed operation"))
				return
			}
			rev := strings.Trim(r.Header.Get("If-Match"), "\"")
			if rev == "" {
				fail(w, 428, errors.New("If-Match required"))
				return
			}
			if spec.operation == "prepare" {
				result, err := a.ManagedN.Prepare(r.Context(), rev)
				if err != nil {
					code := 422
					if errors.Is(err, ErrStale) {
						code = 409
					}
					fail(w, code, err)
					return
				}
				respondJSON(w, 200, result)
				return
			}
			result, err := a.ManagedN.Run(r.Context(), rev, req.Token)
			if err != nil {
				code := 422
				if errors.Is(err, ErrStale) || errors.Is(err, ErrManagedTokenN) {
					code = 409
				}
				respondJSON(w, code, map[string]any{"error": err.Error(), "trial": result})
				return
			}
			respondJSON(w, 200, result)
		})
	}
	// DPR2-M: explicit one-shot DSTest-only ndmc transaction. It always
	// removes its test zone before returning, with an independent watchdog.
	mux.HandleFunc("/v1/keenetic/domain-trial", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		if !requireMutation(w, r, "ndmc-trial") {
			return
		}
		if a.TrialM == nil {
			fail(w, http.StatusServiceUnavailable, errors.New("experimental ndmc transaction capability disabled"))
			return
		}
		var req struct {
			Operation string `json:"operation"`
		}
		if err := decodeBody(r, &req, 1024); err != nil {
			fail(w, 400, err)
			return
		}
		if req.Operation != "trial" {
			fail(w, 400, errors.New("only bounded trial supported"))
			return
		}
		expected := strings.Trim(r.Header.Get("If-Match"), "\"")
		if expected == "" {
			fail(w, 428, errors.New("If-Match required"))
			return
		}
		result, err := a.TrialM.Run(r.Context(), expected)
		if err != nil {
			code := 422
			if errors.Is(err, ErrStale) {
				code = 409
			}
			respondJSON(w, code, map[string]any{"error": err.Error(), "trial": result})
			return
		}
		respondJSON(w, 200, result)
	})
	// DPR2-L: no side effects; uses only the CAS-persisted policy snapshot.
	mux.HandleFunc("/v1/keenetic/domain-plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		cfg, rev, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf("%q", rev))
		respondJSON(w, 200, map[string]any{"revision": rev, "plan": BuildDomainSplitPlanL(cfg), "runtime_changed": false, "ndmc_invoked": false})
	})
	mux.HandleFunc("/v1/runtime/resources", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		respondJSON(w, 200, map[string]any{"resources": CurrentResourceSnapshot(), "passive_only": true})
	})
	// DPR2-G offline-only evidence assessment: never writes the report, never
	// changes DNS Proxy, and never elevates an observation to approval.
	mux.HandleFunc("/v1/keenetic/assess-hardware", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !requireMutation(w, r, "assess-hardware-readonly") {
			return
		}
		var request struct {
			Evidence string `json:"evidence"`
		}
		if err := decodeBody(r, &request, maxHardwareEvidenceBytes); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		assessment, err := ParseHardwareReadOnlyEvidence(request.Evidence)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		respondJSON(w, http.StatusOK, assessment)
	})
	mux.HandleFunc("/v1/keenetic/rehearsal", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		scenario := r.URL.Query().Get("failure")
		if scenario == "" {
			scenario = "verify"
		}
		result := RehearseKeeneticTransaction(scenario)
		if result.State == "INVALID_SCENARIO" {
			fail(w, http.StatusBadRequest, errors.New("invalid simulated failure scenario"))
			return
		}
		respondJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("/v1/keenetic/integration-plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		respondJSON(w, http.StatusOK, BuildKeeneticIntegrationPlan(ReadOnlyKeeneticDiscovery()))
	})
	mux.HandleFunc("/v1/hardware/evidence", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		discovery := ReadOnlyKeeneticDiscovery()
		respondJSON(w, http.StatusOK, map[string]any{
			"mode": "read-only", "resources": CurrentResourceSnapshot(),
			"keenetic": discovery, "integration": BuildKeeneticIntegrationPlan(discovery),
			"changes_applied": false, "hardware_accepted": false,
		})
	})
	mux.HandleFunc("/v1/keenetic/discovery", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		respondJSON(w, 200, ReadOnlyKeeneticDiscovery())
	})
	mux.HandleFunc("/v1/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, rev, err := a.Store.Load()
			if err != nil {
				fail(w, 500, err)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("%q", rev))
			respondJSON(w, 200, map[string]any{"config": cfg, "revision": rev, "active": a.Lifecycle != nil && a.Lifecycle.Status().Active})
		case http.MethodPut:
			if !requireMutation(w, r, "save") {
				return
			}
			var cfg Config
			if err := decodeBody(r, &cfg, maxAPIBytes); err != nil {
				fail(w, 400, err)
				return
			}
			expected := strings.Trim(r.Header.Get("If-Match"), "\"")
			if expected == "" {
				fail(w, 428, errors.New("If-Match required"))
				return
			}
			rev, err := a.Store.Save(cfg, expected)
			if errors.Is(err, ErrStale) {
				fail(w, 409, err)
				return
			}
			if err != nil {
				fail(w, 400, err)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("%q", rev))
			respondJSON(w, 200, map[string]any{"revision": rev, "active": false, "saved": true})
		default:
			w.Header().Set("Allow", "GET, PUT")
			w.WriteHeader(405)
		}
	})
	mux.HandleFunc("/v1/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		if !requireMutation(w, r, "evaluate") {
			return
		}
		var req struct {
			Domain string `json:"domain"`
		}
		if err := decodeBody(r, &req, 4096); err != nil {
			fail(w, 400, err)
			return
		}
		cfg, _, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		e, err := Compile(cfg)
		if err != nil {
			fail(w, 500, err)
			return
		}
		result, err := e.Evaluate(req.Domain)
		if err != nil {
			fail(w, 400, err)
			return
		}
		respondJSON(w, 200, map[string]any{"decision": result, "dry_run": true})
	})
	mux.HandleFunc("/v1/import-preview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		if !requireMutation(w, r, "import-preview") {
			return
		}
		var req ImportRequest
		if err := decodeBody(r, &req, 2<<20); err != nil {
			fail(w, 400, err)
			return
		}
		cfg, rev, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		p, err := PreviewImport(cfg, req)
		if err != nil {
			fail(w, 400, err)
			return
		}
		respondJSON(w, 200, map[string]any{"preview": p, "base_revision": rev, "saved": false})
	})
	mux.HandleFunc("/v1/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		cfg, _, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=routerforge-dns-router.json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(ExportContent(cfg))
	})
	serveUI := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		name := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v1"), "/ui/")
		if name == "" {
			name = "index.html"
		}
		if name != "index.html" && name != "app.js" && name != "module.css" {
			http.NotFound(w, r)
			return
		}
		if a.UI == "" {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(a.UI, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		switch name {
		case "index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case "app.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case "module.css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		_, _ = io.Copy(w, f)
	}
	mux.HandleFunc("/ui/", serveUI)
	mux.HandleFunc("/v1/ui/", serveUI)
	return mux
}
