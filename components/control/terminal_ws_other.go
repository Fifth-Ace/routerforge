//go:build !linux

package main

import "net/http"

func registerAdminTerminalWebSocketRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/terminal/ws", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": "interactive terminal WebSocket requires Linux",
		})
	})
}
