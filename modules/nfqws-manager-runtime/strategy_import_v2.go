package main

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const v2StrategyImportMax = 32

type v2StrategyImportItem struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

type v2StrategyImportRequest struct {
	Items   []v2StrategyImportItem `json:"items"`
	Confirm string                 `json:"confirm"`
}

type v2StrategyImportResult struct {
	Added    int                `json:"added"`
	Existing int                `json:"existing"`
	Items    []v2StoredStrategy `json:"items"`
}

func registerStrategyImportV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/strategies/import", mutationOnly(handleV2StrategyImport))
}

func v2ImportStrategies(doc v2StrategyLibraryDocument, items []v2StrategyImportItem, now string) (v2StrategyLibraryDocument, v2StrategyImportResult, error) {
	result := v2StrategyImportResult{Items: []v2StoredStrategy{}}
	if len(items) == 0 {
		return doc, result, errors.New("import items are empty")
	}
	if len(items) > v2StrategyImportMax {
		return doc, result, errors.New("import item limit exceeded")
	}

	byFingerprint := map[string]v2StoredStrategy{}
	byID := map[string]v2StoredStrategy{}
	for _, item := range doc.Strategies {
		byFingerprint[item.Fingerprint] = item
		byID[item.ID] = item
	}

	requestSeen := map[string]bool{}
	toAdd := make([]v2StoredStrategy, 0, len(items))
	for _, input := range items {
		name := strings.TrimSpace(input.Name)
		if !v2StrategySafeName(name) {
			return doc, result, errors.New("strategy name is empty, too long or contains control characters")
		}
		if err := v2ValidateStoredStrategyArgs(input.Args); err != nil {
			return doc, result, errors.New("strategy is not bench-eligible: " + err.Error())
		}
		args := append([]string{}, input.Args...)
		fingerprint := v2StrategyFingerprint(args)
		if requestSeen[fingerprint] {
			continue
		}
		requestSeen[fingerprint] = true

		if existing, ok := byFingerprint[fingerprint]; ok {
			result.Existing++
			result.Items = append(result.Items, existing)
			continue
		}
		id := "s-" + fingerprint[:16]
		if existing, ok := byID[id]; ok && existing.Fingerprint != fingerprint {
			return doc, result, errors.New("strategy id collision")
		}
		item := v2StoredStrategy{
			ID: id, Name: name, Source: "zapret", Args: args, Fingerprint: fingerprint,
			CreatedAt: now, UpdatedAt: now,
		}
		toAdd = append(toAdd, item)
		byFingerprint[fingerprint] = item
		byID[id] = item
	}
	if len(doc.Strategies)+len(toAdd) > v2StrategyLibraryMax {
		return doc, result, errors.New("strategy library capacity exceeded")
	}
	doc.Strategies = append(doc.Strategies, toAdd...)
	result.Added = len(toAdd)
	result.Items = append(result.Items, toAdd...)
	return doc, result, nil
}

func handleV2StrategyImport(w http.ResponseWriter, r *http.Request) {
	var req v2StrategyImportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy import request"})
		return
	}
	if req.Confirm != "ROUTERFORGE_V2_STRATEGY_IMPORT" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy import confirmation mismatch"})
		return
	}
	doc, err := readV2StrategyLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	next, result, err := v2ImportStrategies(doc, req.Items, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	if result.Added > 0 {
		if err := writeV2StrategyLibrary(next); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write strategy library: " + err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "added": result.Added, "existing": result.Existing,
		"count": len(next.Strategies), "strategies": result.Items,
		"source": "zapret", "production_mutation": false,
	})
}
