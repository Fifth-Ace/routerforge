package main

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

const foundationPath = "/api/platform/foundation"

var foundationObserverID, _ = platformevents.NewEventID()

func registerFoundationHandlers(mux *http.ServeMux) {
	// Same Core auth middleware as existing /api/platform endpoints. No write on startup.
	store := contract.NewStore(filepath.Join(filepath.Dir(securityConfigPath), "policy-catalog-v1.json"))
	registerFoundationStore(mux, store)
	registerFoundationConsumers(mux, store)
	registerFoundationMetrics(mux)
	mux.HandleFunc(foundationPath+"/events", handleFoundationEvents)
}
func registerFoundationStore(mux *http.ServeMux, s *contract.Store) {
	mux.HandleFunc(foundationPath+"/schema", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(contract.Schema())
	})
	mux.HandleFunc(foundationPath+"/catalog", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		st, e := s.Get()
		if e != nil {
			foundationError(w, e)
			return
		}
		writeAuthJSON(w, http.StatusOK, st)
	})
	action := func(commit bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				methodNotAllowedJSON(w, http.MethodPost)
				return
			}
			if !sameOriginRequest(r) || r.Header.Get("X-RF-Foundation") != "1" {
				writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin or missing action header"})
				return
			}
			media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if e != nil || media != "application/json" {
				writeAuthJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "JSON required"})
				return
			}
			b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, contract.MaxCatalogBytes))
			if e != nil {
				writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or oversized body"})
				return
			}
			etag := r.Header.Get("If-Match")
			if len(etag) != 66 || !strings.HasPrefix(etag, "\"") || !strings.HasSuffix(etag, "\"") {
				writeAuthJSON(w, http.StatusPreconditionRequired, map[string]string{"error": "If-Match required"})
				return
			}
			etag = strings.Trim(etag, "\"")
			if !commit {
				p, e := s.Preview(etag, b)
				if e != nil {
					foundationError(w, e)
					return
				}
				writeAuthJSON(w, http.StatusOK, p)
				return
			}
			if r.Header.Get("X-RF-Confirm") != "SAVE_DRAFT" {
				writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "explicit confirmation required"})
				return
			}
			st, changed, e := s.Commit(etag, r.Header.Get("X-RF-Preview"), b)
			if e != nil {
				foundationError(w, e)
				return
			}
			if changed {
				emitPlatformEvent(platformevents.Event{Component: "policy", Severity: platformevents.Info, Type: "policy.catalog.saved", Message: "Policy draft catalog saved"})
			}
			writeAuthJSON(w, http.StatusOK, st)
		}
	}
	mux.HandleFunc(foundationPath+"/preview", action(false))
	mux.HandleFunc(foundationPath+"/save", action(true))
}
func foundationError(w http.ResponseWriter, e error) {
	status := http.StatusInternalServerError
	code := "CATALOG_STORAGE_ERROR"
	if errors.Is(e, contract.ErrConflict) {
		status = http.StatusConflict
		code = "REVISION_CONFLICT"
	} else if errors.Is(e, contract.ErrInvalid) {
		status = http.StatusBadRequest
		code = "INVALID_CATALOG"
	}
	writeAuthJSON(w, status, map[string]string{"error": code})
}
func handleFoundationEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowedJSON(w, http.MethodGet)
		return
	}
	if foundationObserverID == "" {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OBSERVER_UNAVAILABLE"})
		return
	}
	q, limit, e := eventQueryFromRequest(r)
	if e != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_QUERY"})
		return
	}
	if q.ObjectID != "" || q.TransactionID != "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "FILTER_NOT_SUPPORTED"})
		return
	}
	q.Limit = 0
	out := []platformevents.WireEvent{}
	for _, e := range coreEventEngine.Query(q) {
		if v, ok := platformevents.Normalize(e, foundationObserverID); ok {
			out = append(out, v)
			if len(out) == limit {
				break
			}
		}
	}
	writeAuthJSON(w, http.StatusOK, struct {
		SchemaVersion int                        `json:"schemaVersion"`
		ObserverID    string                     `json:"observerId"`
		Events        []platformevents.WireEvent `json:"events"`
	}{1, foundationObserverID, out})
}
