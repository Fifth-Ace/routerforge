package main

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
	sb "github.com/Fifth-Ace/routerforge/internal/platform/supportbundle"
)

// Aggregate only normalized in-memory events; identifiers, messages and
// context are never included in the Admin support observation header.
func supportFoundationObservationHeader() string {
	o := sb.Observation{Available: foundationObserverID != "", CoreVersion: sb.Version(version)}
	if o.Available {
		for _, event := range coreEventEngine.Snapshot(512) {
			v, ok := platformevents.Normalize(event, foundationObserverID)
			if !ok {
				continue
			}
			switch v.Severity {
			case platformevents.Info:
				o.Info++
			case platformevents.Warning:
				o.Warning++
			case platformevents.Critical:
				o.Critical++
			}
		}
	}
	if cpu, ok := coreCPUUsageSampler.sample(); ok && cpu >= 0 && cpu <= 100 {
		o.CPUPercent = &cpu
	}
	raw, _ := json.Marshal(o)
	return string(raw)
}

const foundationPath = "/api/platform/foundation"

var foundationObserverID string

func registerFoundationHandlers(mux *http.ServeMux, bindingAuth ...func(*http.Request) bool) {
	// Same Core auth middleware as existing /api/platform endpoints. Only the
	// installation observer metadata may be created on startup; catalogue and
	// module settings remain untouched. Invalid identity fails closed for exports.
	foundationObserverID, _ = loadFoundationObserver(filepath.Join(filepath.Dir(securityConfigPath), "observer-v1.id"))
	store := contract.NewStore(filepath.Join(filepath.Dir(securityConfigPath), "policy-catalog-v1.json"))
	if len(bindingAuth) == 1 {
		bindings := &foundationBindingService{store: store, path: filepath.Join(filepath.Dir(securityConfigPath), "policy-bindings-v1.json"), read: foundationReadNative, call: foundationBindingCall}
		registerFoundationStore(mux, store, bindings)
		registerFoundationBindings(mux, bindings, bindingAuth[0])
	} else {
		registerFoundationStore(mux, store)
	}
	registerFoundationConsumers(mux, store)
	registerFoundationNative(mux, store, foundationReadNative)
	registerFoundationMetrics(mux)
	mux.HandleFunc(foundationPath+"/events", handleFoundationEvents)
}
func registerFoundationStore(mux *http.ServeMux, s *contract.Store, bindingGuard ...*foundationBindingService) {
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
			// Serialize catalogue preview/save with the binding registry; check both
			// direct saves and imported historical revisions against active dependencies.
			if len(bindingGuard) == 1 {
				g := bindingGuard[0]
				g.mu.Lock()
				defer g.mu.Unlock()
				candidate, e := s.Preview(etag, b)
				if e != nil {
					foundationError(w, e)
					return
				}
				if e = g.guard(candidate.Candidate); e != nil {
					if errors.Is(e, contract.ErrStorage) {
						foundationError(w, e)
					} else {
						writeAuthJSON(w, 409, map[string]string{"error": "BINDING_DEPENDENCY_CONFLICT"})
					}
					return
				}
				if !commit {
					writeAuthJSON(w, http.StatusOK, candidate)
					return
				}
			}
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
				if len(st.History) > 0 {
					before, e1 := contract.NativeReferenceIndex(st.History[0])
					after, e2 := contract.NativeReferenceIndex(st.Current)
					if e1 == nil && e2 == nil && !reflect.DeepEqual(before, after) {
						emitPlatformEvent(platformevents.Event{Component: "policy", Severity: platformevents.Info, Type: "policy.catalog.references.changed", Message: "Native catalogue references changed"})
					}
				}
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
