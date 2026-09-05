package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type dnsModuleServer struct {
	store   *Store
	version string
	socket  string
	uiPath  string
	started time.Time
	control *dnsControlManager
}

func newDNSModuleServer(store *Store, version, socket, uiPath string) *dnsModuleServer {
	return &dnsModuleServer{
		store:   store,
		version: version,
		socket:  socket,
		uiPath:  uiPath,
		started: time.Now(),
		control: newDNSControlManager(newDNSRCIClient("http://127.0.0.1:79/rci"), "/opt/etc/routerforge/dns-disabled.json"),
	}
}

func (s *dnsModuleServer) Serve() error {
	if err := os.MkdirAll(filepath.Dir(s.socket), 0755); err != nil {
		return err
	}
	_ = os.Remove(s.socket)
	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(s.socket)
	_ = os.Chmod(s.socket, 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", s.getOnly(func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":             true,
			"module":         "dns",
			"version":        s.version,
			"api_version":    1,
			"mode":           "control",
			"mutation_api":   true,
			"uptime_seconds": time.Since(s.started).Seconds(),
		})
	}))
	mux.HandleFunc("/v1/snapshot", s.getOnly(func(w http.ResponseWriter, _ *http.Request) {
		data := s.store.Snapshot(200, 30, 80)
		data["version"] = s.version
		data["server_time"] = time.Now()
		s.writeJSON(w, http.StatusOK, data)
	}))
	mux.HandleFunc("/v1/history", s.getOnly(func(w http.ResponseWriter, r *http.Request) {
		minutes := boundedInt(r.URL.Query().Get("minutes"), 60, 1, 1440)
		coverage := s.store.HistoryCoverage(minutes)
		step := 1
		if minutes > 180 {
			step = 15
		} else if minutes > 60 {
			step = 3
		}
		s.writeJSON(w, http.StatusOK, map[string]any{
			"minutes": minutes, "coverage": coverage, "sufficient": coverage >= 0.5,
			"step_minutes": step, "points": s.store.History(minutes),
		})
	}))
	mux.HandleFunc("/v1/quality", s.getOnly(func(w http.ResponseWriter, r *http.Request) {
		minutes := boundedInt(r.URL.Query().Get("minutes"), 5, 1, 1440)
		s.writeJSON(w, http.StatusOK, map[string]any{"minutes": minutes, "upstreams": s.store.Quality(minutes)})
	}))
	mux.HandleFunc("/v1/fallbacks", s.getOnly(func(w http.ResponseWriter, r *http.Request) {
		minutes := boundedInt(r.URL.Query().Get("minutes"), 60, 1, 1440)
		s.writeJSON(w, http.StatusOK, map[string]any{"minutes": minutes, "edges": s.store.FallbackEdges(minutes)})
	}))
	mux.HandleFunc("/v1/error-bursts", s.getOnly(func(w http.ResponseWriter, r *http.Request) {
		minutes := boundedInt(r.URL.Query().Get("minutes"), 60, 1, 1440)
		s.writeJSON(w, http.StatusOK, map[string]any{"minutes": minutes, "bursts": s.store.ErrorBursts(minutes)})
	}))
	mux.HandleFunc("/v1/clients", s.getOnly(func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusOK, map[string]any{"clients": s.store.Clients(200)})
	}))
	mux.HandleFunc("/v1/client", s.getOnly(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.TrimSpace(r.URL.Query().Get("ip"))
		limit := boundedInt(r.URL.Query().Get("limit"), 500, 1, 2000)
		client, events, ok := s.store.ClientDetail(ip, limit)
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "client not found"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"client": client, "events": events})
	}))
	mux.HandleFunc("/v1/interfaces", s.getOnly(func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusOK, map[string]any{"interfaces": s.store.Interfaces()})
	}))
	mux.HandleFunc("/v1/system", s.getOnly(func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusOK, readSystemInfo())
	}))
	mux.HandleFunc("/v1/plain-dns", s.getOnly(func(w http.ResponseWriter, r *http.Request) {
		limit := boundedInt(r.URL.Query().Get("limit"), 100, 1, 500)
		s.writeJSON(w, http.StatusOK, plainDNS.Snapshot(limit))
	}))
	mux.HandleFunc("/v1/info", s.getOnly(func(w http.ResponseWriter, _ *http.Request) {
		info, err := readDNSInfo()
		if err != nil {
			s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, info)
	}))
	mux.HandleFunc("/v1/resolvers", s.handleResolvers)
	mux.HandleFunc("/v1/resolvers/", s.handleResolverAction)
	mux.HandleFunc("/v1/preview", s.handlePreview)

	uiFS := http.FileServer(http.Dir(s.uiPath))
	mux.HandleFunc("/v1/ui", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
			return
		}
		http.Redirect(w, r, "/v1/ui/index.html", http.StatusTemporaryRedirect)
	})
	mux.Handle("/v1/ui/", http.StripPrefix("/v1/ui/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		uiFS.ServeHTTP(w, r)
	})))

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return server.Serve(listener)
}

func boundedInt(raw string, fallback, min, max int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return fallback
	}
	return value
}

func (s *dnsModuleServer) getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func (s *dnsModuleServer) requireMutationHeader(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-RouterForge-Action") != "dns-control" {
		s.writeJSON(w, http.StatusForbidden, map[string]any{
			"error": "missing RouterForge mutation header",
		})
		return false
	}
	contentType := r.Header.Get("Content-Type")
	if r.Method != http.MethodDelete && !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		s.writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "application/json required"})
		return false
	}
	return true
}

func decodeResolverSpec(r *http.Request) (DNSResolverSpec, error) {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var spec DNSResolverSpec
	if err := decoder.Decode(&spec); err != nil {
		return DNSResolverSpec{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return DNSResolverSpec{}, errors.New("only one JSON object is allowed")
	}
	return spec, nil
}

func (s *dnsModuleServer) handleResolvers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		snapshot, err := s.control.List(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, snapshot)
	case http.MethodPost:
		if !s.requireMutationHeader(w, r) {
			return
		}
		spec, err := decodeResolverSpec(r)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		result, err := s.control.Create(r.Context(), spec)
		if err != nil {
			s.writeMutationError(w, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, result)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func (s *dnsModuleServer) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !s.requireMutationHeader(w, r) {
		return
	}
	spec, err := decodeResolverSpec(r)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	preview, err := previewDNSResolver(spec)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, preview)
}

func (s *dnsModuleServer) handleResolverAction(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/resolvers/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if !s.requireMutationHeader(w, r) {
		return
	}

	var (
		result DNSMutationResult
		err    error
	)
	switch {
	case action == "" && r.Method == http.MethodPatch:
		var spec DNSResolverSpec
		spec, err = decodeResolverSpec(r)
		if err == nil {
			result, err = s.control.Update(r.Context(), id, spec)
		}
	case action == "" && r.Method == http.MethodDelete:
		result, err = s.control.Delete(r.Context(), id)
	case action == "disable" && r.Method == http.MethodPost:
		result, err = s.control.Disable(r.Context(), id)
	case action == "enable" && r.Method == http.MethodPost:
		result, err = s.control.Enable(r.Context(), id)
	default:
		w.Header().Set("Allow", "PATCH, DELETE, POST")
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "unsupported resolver action"})
		return
	}
	if err != nil {
		if errors.Is(err, errDNSResolverNotFound) {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		if errors.Is(err, errDNSResolverReadOnly) || errors.Is(err, errDNSResolverConflict) {
			s.writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			return
		}
		if errors.Is(err, errDNSResolverInvalid) {
			s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		s.writeMutationError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *dnsModuleServer) writeMutationError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if errors.Is(err, errDNSResolverInvalid) {
		status = http.StatusBadRequest
	} else if errors.Is(err, errDNSResolverConflict) || errors.Is(err, errDNSResolverReadOnly) {
		status = http.StatusConflict
	}
	s.writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (s *dnsModuleServer) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *dnsModuleServer) String() string {
	return fmt.Sprintf("dns module %s on %s", s.version, s.socket)
}
