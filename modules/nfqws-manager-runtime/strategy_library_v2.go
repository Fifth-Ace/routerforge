package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	v2StrategyLibraryRoot = "/opt/var/lib/routerforge/nfqws-manager"
	v2StrategyLibraryPath = v2StrategyLibraryRoot + "/strategy-library.json"
	v2StrategyLibraryMax  = 64
	v2StrategyArgsMax     = 256
)

type v2StoredStrategy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Args        []string `json:"args"`
	Fingerprint string   `json:"fingerprint"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type v2StrategyLibraryDocument struct {
	Version    int                `json:"version"`
	Strategies []v2StoredStrategy `json:"strategies"`
}

type v2StrategySaveRequest struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Source  string   `json:"source,omitempty"`
	Args    []string `json:"args"`
	Confirm string   `json:"confirm"`
}

type v2StrategyDeleteRequest struct {
	ID      string `json:"id"`
	Confirm string `json:"confirm"`
}

func registerStrategyLibraryV2Routes(mux *http.ServeMux) {
	registerStrategyImportV2Routes(mux)
	mux.HandleFunc("/v1/v2/strategies", getOnly(handleV2StrategiesList))
	mux.HandleFunc("/v1/v2/strategies/save", mutationOnly(handleV2StrategySave))
	mux.HandleFunc("/v1/v2/strategies/delete", mutationOnly(handleV2StrategyDelete))
}

func v2StrategySafeID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 || filepath.Base(value) != value || strings.HasPrefix(value, ".") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func v2StrategySafeName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 120 {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func v2StrategySource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "catalog":
		return "catalog"

	case "custom":
		return "custom"
	case "import":
		return "import"
	case "builtin":
		return "builtin"
	case "memory":
		return "memory"
	default:
		return "custom"
	}
}

func v2StrategyFingerprint(args []string) string {
	h := sha256.New()
	for _, arg := range args {
		_, _ = h.Write([]byte(arg))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func v2ValidateStoredStrategyArgs(args []string) error {
	if len(args) == 0 || len(args) > v2StrategyArgsMax {
		return errors.New("strategy args are empty or exceed the candidate limit")
	}
	if _, err := v2CustomProfile(args, "example.com"); err != nil {
		return err
	}
	return nil
}

func ensureV2StrategyLibraryRoot() error {
	if err := os.MkdirAll(v2StrategyLibraryRoot, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(v2StrategyLibraryRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("strategy library root is not a real directory")
	}
	return nil
}

func readV2StrategyLibrary() (v2StrategyLibraryDocument, error) {
	doc := v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{}}
	data, err := os.ReadFile(v2StrategyLibraryPath)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	if len(data) > 256<<10 {
		return doc, errors.New("strategy library exceeds safety limit")
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, errors.New("decode strategy library: " + err.Error())
	}
	if doc.Version != 1 {
		return doc, errors.New("unsupported strategy library version")
	}
	if len(doc.Strategies) > v2StrategyLibraryMax {
		return doc, errors.New("strategy library exceeds entry limit")
	}
	for i := range doc.Strategies {
		doc.Strategies[i].Source = v2StrategySource(doc.Strategies[i].Source)
		item := doc.Strategies[i]
		if !v2StrategySafeID(item.ID) || !v2StrategySafeName(item.Name) ||
			item.Fingerprint != v2StrategyFingerprint(item.Args) ||
			v2ValidateStoredStrategyArgs(item.Args) != nil {
			return doc, errors.New("strategy library contains an invalid entry")
		}
	}
	sort.Slice(doc.Strategies, func(i, j int) bool {
		return strings.ToLower(doc.Strategies[i].Name) < strings.ToLower(doc.Strategies[j].Name)
	})
	return doc, nil
}

func writeV2StrategyLibrary(doc v2StrategyLibraryDocument) error {
	if err := ensureV2StrategyLibraryRoot(); err != nil {
		return err
	}
	if len(doc.Strategies) > v2StrategyLibraryMax {
		return errors.New("strategy library entry limit exceeded")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if old, readErr := os.ReadFile(v2StrategyLibraryPath); readErr == nil && len(old) > 0 {
		_, _ = recordPersistentBackupTarget(
			"strategy-library", "strategy-library", v2StrategyLibraryPath, old, 0600,
		)
	}
	return safety.WriteFileAtomic(v2StrategyLibraryPath, data, 0600)
}

func handleV2StrategiesList(w http.ResponseWriter, _ *http.Request) {
	doc, err := readV2StrategyLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": doc.Version, "count": len(doc.Strategies),
		"max": v2StrategyLibraryMax, "strategies": doc.Strategies,
	})
}

func handleV2StrategySave(w http.ResponseWriter, r *http.Request) {
	var req v2StrategySaveRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy save request"})
		return
	}
	if req.Confirm != "ROUTERFORGE_V2_STRATEGY_SAVE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy save confirmation mismatch"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !v2StrategySafeName(req.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "strategy name is empty, too long or contains control characters"})
		return
	}
	if err := v2ValidateStoredStrategyArgs(req.Args); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy is not bench-eligible: " + err.Error()})
		return
	}
	fingerprint := v2StrategyFingerprint(req.Args)
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = "s-" + fingerprint[:16]
	}
	if !v2StrategySafeID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy id"})
		return
	}
	doc, err := readV2StrategyLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	found := -1
	for i := range doc.Strategies {
		if doc.Strategies[i].ID == id {
			found = i
			break
		}
	}
	item := v2StoredStrategy{
		ID: id, Name: req.Name, Source: v2StrategySource(req.Source),
		Args: append([]string{}, req.Args...), Fingerprint: fingerprint,
		CreatedAt: now, UpdatedAt: now,
	}
	if found >= 0 {
		item.CreatedAt = doc.Strategies[found].CreatedAt
		doc.Strategies[found] = item
	} else {
		if len(doc.Strategies) >= v2StrategyLibraryMax {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy library is full"})
			return
		}
		doc.Strategies = append(doc.Strategies, item)
	}
	if err := writeV2StrategyLibrary(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write strategy library: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "strategy": item})
}

func handleV2StrategyDelete(w http.ResponseWriter, r *http.Request) {
	var req v2StrategyDeleteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy delete request"})
		return
	}
	if req.Confirm != "ROUTERFORGE_V2_STRATEGY_DELETE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy delete confirmation mismatch"})
		return
	}
	id := strings.TrimSpace(req.ID)
	if !v2StrategySafeID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy id"})
		return
	}
	doc, err := readV2StrategyLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	next := make([]v2StoredStrategy, 0, len(doc.Strategies))
	deleted := false
	for _, item := range doc.Strategies {
		if item.ID == id {
			deleted = true
			continue
		}
		next = append(next, item)
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "strategy not found"})
		return
	}
	doc.Strategies = next
	if err := writeV2StrategyLibrary(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write strategy library: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}
