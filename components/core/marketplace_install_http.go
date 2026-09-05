package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type catalogInstallRequest struct {
	ID string `json:"id"`
}

type catalogActionRequest struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Confirm string `json:"confirm,omitempty"`
}

func handleCatalogInstallTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin install request rejected"})
		return
	}
	var request catalogInstallRequest
	if err := decodeSmallJSON(w, r, &request); err != nil || strings.TrimSpace(request.ID) == "" {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "valid catalog item id is required"})
		return
	}
	runCatalogHTTPAction(w, r, request.ID, "install", "")
}

func handleCatalogActionTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin package-management request rejected"})
		return
	}
	var request catalogActionRequest
	if err := decodeSmallJSON(w, r, &request); err != nil || strings.TrimSpace(request.ID) == "" {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "valid catalog action request is required"})
		return
	}
	runCatalogHTTPAction(w, r, request.ID, request.Action, request.Confirm)
}

func runCatalogHTTPAction(w http.ResponseWriter, r *http.Request, id, action, confirmation string) {
	if !marketplaceTestInstallEnabled() {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{
			"error":  "marketplace test package management is disabled",
			"marker": marketplaceTestInstallMarker,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	result, err := runCatalogModuleAction(ctx, id, action, confirmation)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		detail := ""
		if actionErr, ok := err.(*catalogInstallFailure); ok {
			if actionErr.Status > 0 {
				status = actionErr.Status
			}
			message = actionErr.Message
			detail = actionErr.Detail
		}
		writeCatalogJSON(w, status, map[string]any{
			"error":  message,
			"detail": detail,
			"result": result,
		})
		return
	}
	writeCatalogJSON(w, http.StatusOK, result)
}

func writeCatalogJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
