package main

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	profilingMarker = "/opt/etc/dns-monitor/profiling.enabled"
	profilingConfig = "/opt/etc/dns-monitor/profiling.conf"
)

type profilingRuntime struct {
	mu        sync.RWMutex
	Enabled   bool      `json:"enabled"`
	Running   bool      `json:"running"`
	Listen    string    `json:"listen"`
	SlowMS    int       `json:"slow_ms"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

var profilingState = profilingRuntime{
	Listen: "127.0.0.1:6061",
	SlowMS: 750,
}

func startOptionalProfiling() {
	if _, err := os.Stat(profilingMarker); err != nil {
		return
	}

	listen, slowMS := readProfilingConfig()
	profilingState.mu.Lock()
	profilingState.Enabled = true
	profilingState.Listen = listen
	profilingState.SlowMS = slowMS
	profilingState.mu.Unlock()

	if !loopbackListenAddress(listen) {
		profilingState.mu.Lock()
		profilingState.Error = "profiling listen address must be loopback-only"
		profilingState.mu.Unlock()
		log.Printf("profiling disabled: %s", profilingState.Error)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("/debug/pprof/"+name, pprof.Handler(name))
	}

	server := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		profilingState.mu.Lock()
		profilingState.Running = true
		profilingState.StartedAt = time.Now()
		profilingState.mu.Unlock()

		log.Printf("profiling listener enabled on %s", listen)
		err := server.ListenAndServe()

		profilingState.mu.Lock()
		profilingState.Running = false
		if err != nil && err != http.ErrServerClosed {
			profilingState.Error = err.Error()
			log.Printf("profiling listener failed: %v", err)
		}
		profilingState.mu.Unlock()
	}()
}

func readProfilingConfig() (string, int) {
	listen := "127.0.0.1:6061"
	slowMS := 750

	f, err := os.Open(profilingConfig)
	if err != nil {
		return listen, slowMS
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "listen":
			if value != "" {
				listen = value
			}
		case "slow_ms":
			if n, parseErr := strconv.Atoi(value); parseErr == nil && n >= 50 && n <= 60000 {
				slowMS = n
			}
		}
	}
	return listen, slowMS
}

func loopbackListenAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func profilingStatusSnapshot() map[string]any {
	profilingState.mu.RLock()
	defer profilingState.mu.RUnlock()
	return map[string]any{
		"enabled":    profilingState.Enabled,
		"running":    profilingState.Running,
		"listen":     profilingState.Listen,
		"slow_ms":    profilingState.SlowMS,
		"error":      profilingState.Error,
		"started_at": profilingState.StartedAt,
		"marker":     profilingMarker,
		"mode":       "loopback-only",
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

// Preserve SSE streaming when profiling middleware is enabled.
func (r *statusRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func profiledHTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profilingState.mu.RLock()
		enabled := profilingState.Enabled
		slowMS := profilingState.SlowMS
		profilingState.mu.RUnlock()

		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		elapsed := time.Since(start)

		if elapsed >= time.Duration(slowMS)*time.Millisecond {
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			// Query strings may contain private domains/client values, so never
			// print RawQuery in slow-request logs.
			log.Printf("slow request method=%s path=%s status=%d duration=%s",
				r.Method, r.URL.Path, status, elapsed)
		}
	})
}
