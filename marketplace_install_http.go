package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type catalogInstallRequest struct {
	ID string `json:"id"`
}

func handleCatalogInstallTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST required"})
		return
	}

	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "cross-origin install request rejected"})
			return
		}
	}

	if !marketplaceTestInstallEnabled() {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":  "marketplace test install mode is disabled",
			"marker": marketplaceTestInstallMarker,
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request catalogInstallRequest
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.ID) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "valid catalog module id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	result, err := installCatalogModuleTest(ctx, request.ID)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		detail := ""
		if installErr, ok := err.(*catalogInstallFailure); ok {
			if installErr.Status > 0 {
				status = installErr.Status
			}
			message = installErr.Message
			detail = installErr.Detail
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":  message,
			"detail": detail,
			"result": result,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(result)
}
