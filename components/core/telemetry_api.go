package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/telemetry"
)

// Audited RouterForge Collector endpoint (P10E). The installation identity is
// independent of optional diagnostics; never transmit over plain HTTP.
var telemetryCollectorEndpoint = "https://telemetry.fifthace.ru/v1/events"
var routerForgeTelemetryManager *telemetry.Manager

func startRouterForgeTelemetry(coreVersion string) {
	manager := telemetry.NewManager(
		"/opt/etc/routerforge/telemetry.json",
		"/opt/etc/routerforge/telemetry-state.json",
		telemetryCollectorEndpoint,
		coreVersion,
		func() (float64, float64, float64, bool, bool, bool) {
			tel := readCoreTelemetry()
			return tel.CPUUsagePct, tel.RAMUsagePct, tel.CPUTempC, tel.CPUUsageAvailable, tel.RAMUsageAvailable, tel.CPUTempAvailable
		}, nil)
	routerForgeTelemetryManager = manager
	go manager.Run(context.Background())
}

func registerRouterForgeTelemetryHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/settings/telemetry", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if routerForgeTelemetryManager == nil {
			http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(routerForgeTelemetryManager.Status())
		case http.MethodPost:
			if !sameOriginRequest(r) {
				http.Error(w, `{"error":"cross-origin rejected"}`, http.StatusForbidden)
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, `{"error":"json required"}`, http.StatusUnsupportedMediaType)
				return
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
			dec.DisallowUnknownFields()
			var request struct {
				DiagnosticsEnabled *bool `json:"diagnostics_enabled"`
			}
			if dec.Decode(&request) != nil || request.DiagnosticsEnabled == nil {
				http.Error(w, `{"error":"invalid diagnostics setting"}`, http.StatusBadRequest)
				return
			}
			var trailing any
			if dec.Decode(&trailing) != io.EOF {
				http.Error(w, `{"error":"invalid trailing data"}`, http.StatusBadRequest)
				return
			}
			if err := routerForgeTelemetryManager.SetDiagnostics(*request.DiagnosticsEnabled); err != nil {
				http.Error(w, `{"error":"settings unavailable"}`, http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(routerForgeTelemetryManager.Status())
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/settings/telemetry/preview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if routerForgeTelemetryManager == nil {
			http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(routerForgeTelemetryManager.Preview())
	})
}
