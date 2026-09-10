//go:build !linux

package main

import (
	"net/http"
)

func registerAdminTerminalSessionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/terminal/session", mutationOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": "interactive terminal sessions require Linux",
		})
	}))
}
