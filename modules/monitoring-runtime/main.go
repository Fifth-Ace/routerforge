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

var monitoringModuleIDs = []string{"system", "thermal", "storage", "network"}

var monitoringSocketNames = map[string]string{
	"system":  "routerforge-system.sock",
	"thermal": "routerforge-thermal.sock",
	"storage": "routerforge-storage.sock",
	"network": "routerforge-network.sock",
}

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
	moduleID := flag.String("module", "", "module id: system|thermal|storage|network|all")
	socket := flag.String("socket", "", "Unix socket path")
	socketDir := flag.String("socket-dir", "/opt/var/run", "Unix socket directory for -module all")
	flag.Parse()

	id := strings.ToLower(strings.TrimSpace(*moduleID))
	if !validModuleMode(id) {
		panic("invalid -module: expected system|thermal|storage|network|all")
	}
	if id == "all" {
		if strings.TrimSpace(*socket) != "" {
			panic("invalid -socket: -module all uses -socket-dir")
		}
		if err := serveAllModules(*socketDir); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
		return
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

func validModuleMode(id string) bool {
	return id == "all" || validModuleID(id)
}

func validModuleID(id string) bool {
	switch id {
	case "system", "thermal", "storage", "network":
		return true
	default:
		return false
	}
}

func monitoringSocketPath(socketDir, id string) string {
	name, ok := monitoringSocketNames[id]
	if !ok {
		return ""
	}
	return filepath.Join(strings.TrimSpace(socketDir), name)
}

func unixSocketResponding(socket string) (bool, error) {
	info, err := os.Lstat(socket)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return false, fmt.Errorf("monitoring socket path exists and is not a Unix socket: %s", socket)
	}
	conn, err := net.DialTimeout("unix", socket, 150*time.Millisecond)
	if err != nil {
		return false, nil
	}
	_ = conn.Close()
	return true, nil
}

func serveAllModules(socketDir string) error {
	socketDir = strings.TrimSpace(socketDir)
	if socketDir == "" {
		return errors.New("invalid -socket-dir: directory is required")
	}

	for _, id := range monitoringModuleIDs {
		socket := monitoringSocketPath(socketDir, id)
		active, err := unixSocketResponding(socket)
		if err != nil {
			return err
		}
		if active {
			return fmt.Errorf("monitoring socket already active: %s", socket)
		}
	}

	servers := make([]*moduleServer, 0, len(monitoringModuleIDs))
	for _, id := range monitoringModuleIDs {
		server, err := newModuleServer(id, monitoringSocketPath(socketDir, id))
		if err != nil {
			for _, created := range servers {
				created.Close()
			}
			return err
		}
		servers = append(servers, server)
	}
	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	errCh := make(chan error, len(servers))
	for _, server := range servers {
		go func(s *moduleServer) {
			errCh <- s.Serve()
		}(server)
	}
	return <-errCh
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
