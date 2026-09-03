package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var version = "dev"

type moduleServer struct {
	id      string
	socket  string
	started time.Time

	system  *systemCollector
	thermal *thermalCollector
	storage *storageCollector
	network *networkCollector
}

func main() {
	moduleID := flag.String("module", "", "module id: system|thermal|storage|network")
	socket := flag.String("socket", "", "Unix socket path")
	flag.Parse()

	id := strings.ToLower(strings.TrimSpace(*moduleID))
	if !validModuleID(id) {
		panic("invalid -module: expected system|thermal|storage|network")
	}
	if strings.TrimSpace(*socket) == "" {
		*socket = fmt.Sprintf("/opt/var/run/dns-monitor-%s.sock", id)
	}

	server, err := newModuleServer(id, *socket)
	if err != nil {
		panic(err)
	}
	defer server.Close()

	if err := server.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func validModuleID(id string) bool {
	switch id {
	case "system", "thermal", "storage", "network":
		return true
	default:
		return false
	}
}

func newModuleServer(id, socket string) (*moduleServer, error) {
	s := &moduleServer{id: id, socket: socket, started: time.Now()}

	switch id {
	case "system":
		collector, err := newSystemCollector()
		if err != nil {
			return nil, err
		}
		s.system = collector
	case "thermal":
		s.thermal = newThermalCollector(30 * time.Second)
	case "storage":
		s.storage = newStorageCollector(2 * time.Second)
	case "network":
		s.network = newNetworkCollector(2 * time.Second)
	}

	return s, nil
}

func (s *moduleServer) Close() {
	if s.system != nil {
		s.system.Close()
	}
	if s.storage != nil {
		s.storage.Close()
	}
	if s.network != nil {
		s.network.Close()
	}
}

func (s *moduleServer) Serve() error {
	if err := os.MkdirAll(filepath.Dir(s.socket), 0755); err != nil {
		return err
	}
	_ = os.Remove(s.socket)

	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	_ = os.Chmod(s.socket, 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":             true,
			"module":         s.id,
			"version":        version,
			"mode":           "read-only",
			"mutation_api":   false,
			"uptime_seconds": time.Since(s.started).Seconds(),
		})
	}))

	switch s.id {
	case "system":
		s.registerSystem(mux)
	case "thermal":
		s.registerThermal(mux)
	case "storage":
		s.registerStorage(mux)
	case "network":
		s.registerNetwork(mux)
	}

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	return httpServer.Serve(listener)
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error":        "module is read-only",
				"mutation_api": false,
			})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
