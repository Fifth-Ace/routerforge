package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// Wizard validation is side-effect-free. In particular, preview never claims
// to install a firewall hook or expose SPA material.
func engineWizardPreviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		jsonReply(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
		jsonReply(w, http.StatusForbidden, map[string]string{"error": "Core authorization required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var req EngineWizardRequest
	if err := d.Decode(&req); err != nil {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid wizard request"})
		return
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "trailing data"})
		return
	}
	plan, err := describeWizard(req)
	if err != nil {
		jsonReply(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	files, err := renderWizard(req)
	if err != nil {
		jsonReply(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	jsonReply(w, http.StatusOK, map[string]any{"valid": true, "preview_only": true, "applied": false, "engine": req.Engine, "plan": plan, "candidate": files})
}
