package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	v2GeoRoot          = "/opt/var/lib/routerforge/nfqws-manager/geo"
	v2GeoMetaPath      = v2GeoRoot + "/metadata.json"
	v2GeoAssetMaxBytes = 24 << 20
	v2GeoPreviewMax    = 20000
)

type v2GeoCategory struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type v2GeoAssetMeta struct {
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Size       int64           `json:"size"`
	SHA256     string          `json:"sha256"`
	ModifiedAt string          `json:"modified_at,omitempty"`
	Categories []v2GeoCategory `json:"categories"`
	Source     string          `json:"source,omitempty"`
	SourceURL  string          `json:"source_url,omitempty"`
	SourceRef  string          `json:"source_ref,omitempty"`
}

type v2GeoMetaDocument struct {
	Version int                       `json:"version"`
	Assets  map[string]v2GeoAssetMeta `json:"assets"`
}

type v2GeoUploadRequest struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	ContentBase64  string `json:"content_base64"`
	ExpectedSHA256 string `json:"expected_sha256"`
	Source         string `json:"source,omitempty"`
	SourceURL      string `json:"source_url,omitempty"`
	SourceRef      string `json:"source_ref,omitempty"`
	Confirm        string `json:"confirm"`
}

type v2GeoSourceRequest struct {
	Name      string `json:"name"`
	Source    string `json:"source,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
	Confirm   string `json:"confirm"`
}

type v2GeoPreviewRequest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Limit    int    `json:"limit,omitempty"`
}

type v2GeoSaveListRequest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	ListName string `json:"list_name"`
	Limit    int    `json:"limit,omitempty"`
	Replace  bool   `json:"replace,omitempty"`
	Confirm  string `json:"confirm"`
}

func registerGeoListIntelligenceV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/geo/assets", getOnly(handleV2GeoAssets))
	mux.HandleFunc("/v1/v2/geo/upload", mutationOnly(handleV2GeoUpload))
	mux.HandleFunc("/v1/v2/geo/source", mutationOnly(handleV2GeoSource))
	mux.HandleFunc("/v1/v2/geo/preview", mutationOnly(handleV2GeoPreview))
	mux.HandleFunc("/v1/v2/geo/save-list", mutationOnly(handleV2GeoSaveList))
	mux.HandleFunc("/v1/v2/geo/autohostlist", getOnly(handleV2GeoAutohostlist))
}

func v2GeoSafeName(name string) bool {
	name = strings.TrimSpace(name)
	if !safeListName(name) {
		return false
	}
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".dat") || strings.HasSuffix(lower, ".txt")
}

func v2GeoKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "geosite":
		return "geosite", nil
	case "geoip":
		return "geoip", nil
	case "text":
		return "text", nil
	default:
		return "", errors.New("kind must be geosite, geoip or text")
	}
}

func v2GeoReadMeta() v2GeoMetaDocument {
	doc := v2GeoMetaDocument{Version: 1, Assets: map[string]v2GeoAssetMeta{}}
	data, err := os.ReadFile(v2GeoMetaPath)
	if err != nil {
		return doc
	}
	if json.Unmarshal(data, &doc) != nil || doc.Version != 1 || doc.Assets == nil {
		return v2GeoMetaDocument{Version: 1, Assets: map[string]v2GeoAssetMeta{}}
	}
	return doc
}

func v2GeoWriteMeta(doc v2GeoMetaDocument) error {
	if err := os.MkdirAll(v2GeoRoot, 0755); err != nil {
		return err
	}
	doc.Version = 1
	if doc.Assets == nil {
		doc.Assets = map[string]v2GeoAssetMeta{}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return safety.WriteFileAtomic(v2GeoMetaPath, data, 0600)
}

func v2GeoPBWalk(data []byte, fn func(num, wire int, v uint64, b []byte)) {
	for i := 0; i < len(data); {
		key, n := binary.Uvarint(data[i:])
		if n <= 0 {
			return
		}
		i += n
		num, wire := int(key>>3), int(key&7)
		switch wire {
		case 0:
			v, n := binary.Uvarint(data[i:])
			if n <= 0 {
				return
			}
			i += n
			fn(num, 0, v, nil)
		case 1:
			if i+8 > len(data) {
				return
			}
			i += 8
		case 2:
			l, n := binary.Uvarint(data[i:])
			if n <= 0 {
				return
			}
			i += n
			if l > uint64(len(data)-i) {
				return
			}
			fn(num, 2, 0, data[i:i+int(l)])
			i += int(l)
		case 5:
			if i+4 > len(data) {
				return
			}
			i += 4
		default:
			return
		}
	}
}

func v2GeoParseSite(data []byte) map[string][]string {
	out := map[string][]string{}
	v2GeoPBWalk(data, func(num, wire int, _ uint64, b []byte) {
		if num != 1 || wire != 2 {
			return
		}
		var code string
		var items []string
		v2GeoPBWalk(b, func(n2, w2 int, _ uint64, b2 []byte) {
			switch {
			case n2 == 1 && w2 == 2:
				code = strings.ToLower(string(b2))
			case n2 == 2 && w2 == 2:
				var typ uint64
				var value string
				v2GeoPBWalk(b2, func(n3, w3 int, v3 uint64, b3 []byte) {
					if n3 == 1 && w3 == 0 {
						typ = v3
					}
					if n3 == 2 && w3 == 2 {
						value = string(b3)
					}
				})
				if value != "" && typ != 1 {
					items = append(items, value)
				}
			}
		})
		if code != "" {
			out[code] = append(out[code], items...)
		}
	})
	return out
}

func v2GeoParseIP(data []byte) map[string][]string {
	out := map[string][]string{}
	v2GeoPBWalk(data, func(num, wire int, _ uint64, b []byte) {
		if num != 1 || wire != 2 {
			return
		}
		var code string
		var items []string
		v2GeoPBWalk(b, func(n2, w2 int, _ uint64, b2 []byte) {
			switch {
			case n2 == 1 && w2 == 2:
				code = strings.ToLower(string(b2))
			case n2 == 2 && w2 == 2:
				var ip net.IP
				var prefix uint64
				v2GeoPBWalk(b2, func(n3, w3 int, v3 uint64, b3 []byte) {
					if n3 == 1 && w3 == 2 {
						ip = net.IP(append([]byte(nil), b3...))
					}
					if n3 == 2 && w3 == 0 {
						prefix = v3
					}
				})
				if len(ip) == 4 || len(ip) == 16 {
					bits := 32
					if len(ip) == 16 {
						bits = 128
					}
					p := int(prefix)
					if p == 0 || p > bits {
						p = bits
					}
					items = append(items, (&net.IPNet{IP: ip, Mask: net.CIDRMask(p, bits)}).String())
				}
			}
		})
		if code != "" {
			out[code] = append(out[code], items...)
		}
	})
	return out
}

func v2GeoParseText(data []byte) map[string][]string {
	var items []string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		items = append(items, line)
	}
	return map[string][]string{"all": items}
}

func v2GeoParse(kind string, data []byte) map[string][]string {
	switch kind {
	case "geosite":
		return v2GeoParseSite(data)
	case "geoip":
		return v2GeoParseIP(data)
	default:
		return v2GeoParseText(data)
	}
}

func v2GeoDedupe(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func v2GeoCategories(parsed map[string][]string) []v2GeoCategory {
	out := make([]v2GeoCategory, 0, len(parsed))
	for name, items := range parsed {
		out = append(out, v2GeoCategory{Name: name, Count: len(v2GeoDedupe(items))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func v2GeoAssetPath(name string) (string, error) {
	if !v2GeoSafeName(name) {
		return "", errors.New("invalid geo asset filename")
	}
	return filepath.Join(v2GeoRoot, strings.TrimSpace(name)), nil
}

func v2GeoReadAsset(name string) ([]byte, v2GeoAssetMeta, error) {
	doc := v2GeoReadMeta()
	meta, ok := doc.Assets[name]
	if !ok {
		return nil, v2GeoAssetMeta{}, os.ErrNotExist
	}
	path, err := v2GeoAssetPath(name)
	if err != nil {
		return nil, v2GeoAssetMeta{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, v2GeoAssetMeta{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, v2GeoAssetMeta{}, errors.New("geo asset is not a regular file")
	}
	if info.Size() > v2GeoAssetMaxBytes {
		return nil, v2GeoAssetMeta{}, errors.New("geo asset exceeds safety limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, v2GeoAssetMeta{}, err
	}
	return data, meta, nil
}

func v2GeoInventory() []v2GeoAssetMeta {
	doc := v2GeoReadMeta()
	out := make([]v2GeoAssetMeta, 0, len(doc.Assets))
	for name, meta := range doc.Assets {
		path, err := v2GeoAssetPath(name)
		if err != nil {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		meta.Size = info.Size()
		meta.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func v2GeoDecodeRequest(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, int64(v2GeoAssetMaxBytes*4/3)+(2<<20))
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func handleV2GeoAssets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "root": v2GeoRoot, "assets": v2GeoInventory(),
		"max_asset_bytes": v2GeoAssetMaxBytes, "production_mutation": false,
	})
}

func handleV2GeoUpload(w http.ResponseWriter, r *http.Request) {
	var req v2GeoUploadRequest
	if err := v2GeoDecodeRequest(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid geo upload request"})
		return
	}
	if req.Confirm != "ROUTERFORGE_GEO_UPLOAD" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "geo upload confirmation mismatch"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	kind, err := v2GeoKind(req.Kind)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	path, err := v2GeoAssetPath(req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content_base64 is invalid"})
		return
	}
	if len(data) == 0 || len(data) > v2GeoAssetMaxBytes {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "geo asset is empty or exceeds safety limit"})
		return
	}
	if !listSourceHashPattern.MatchString(strings.TrimSpace(req.ExpectedSHA256)) ||
		!strings.EqualFold(listSourceSHA256(data), strings.TrimSpace(req.ExpectedSHA256)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "geo asset SHA256 mismatch"})
		return
	}
	parsed := v2GeoParse(kind, data)
	categories := v2GeoCategories(parsed)
	if len(categories) == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "geo asset contains no supported categories"})
		return
	}
	if err := os.MkdirAll(v2GeoRoot, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "existing geo asset is not a regular file"})
			return
		}
		before, readErr := os.ReadFile(path)
		if readErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": readErr.Error()})
			return
		}
		if _, backupErr := createNamedBackup("geo-"+req.Name, before, info.Mode().Perm()); backupErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create geo backup: " + backupErr.Error()})
			return
		}
	}
	if err := safety.WriteFileAtomic(path, data, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write geo asset: " + err.Error()})
		return
	}
	doc := v2GeoReadMeta()
	doc.Assets[req.Name] = v2GeoAssetMeta{
		Name: req.Name, Kind: kind, Size: int64(len(data)), SHA256: listSourceSHA256(data),
		ModifiedAt: time.Now().UTC().Format(time.RFC3339), Categories: categories,
		Source: strings.TrimSpace(req.Source), SourceURL: strings.TrimSpace(req.SourceURL), SourceRef: strings.TrimSpace(req.SourceRef),
	}
	if err := v2GeoWriteMeta(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write geo metadata: " + err.Error()})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "asset": doc.Assets[req.Name], "production_mutation": false,
		"runtime_restarted": false, "config_sha256": readStatus().ConfigSHA256,
	})
}

func handleV2GeoSource(w http.ResponseWriter, r *http.Request) {
	var req v2GeoSourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid geo source request"})
		return
	}
	if req.Confirm != "ROUTERFORGE_GEO_SOURCE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "geo source confirmation mismatch"})
		return
	}
	doc := v2GeoReadMeta()
	meta, ok := doc.Assets[strings.TrimSpace(req.Name)]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "geo asset not found"})
		return
	}
	meta.Source = strings.TrimSpace(req.Source)
	meta.SourceURL = strings.TrimSpace(req.SourceURL)
	meta.SourceRef = strings.TrimSpace(req.SourceRef)
	doc.Assets[meta.Name] = meta
	if err := v2GeoWriteMeta(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "asset": meta, "production_mutation": false})
}

func v2GeoCategoryItems(name, category string, limit int) ([]string, int, bool, error) {
	data, meta, err := v2GeoReadAsset(name)
	if err != nil {
		return nil, 0, false, err
	}
	parsed := v2GeoParse(meta.Kind, data)
	items, ok := parsed[strings.ToLower(strings.TrimSpace(category))]
	if !ok {
		return nil, 0, false, errors.New("geo category not found")
	}
	items = v2GeoDedupe(items)
	total := len(items)
	if limit <= 0 || limit > v2GeoPreviewMax {
		limit = v2GeoPreviewMax
	}
	truncated := total > limit
	if truncated {
		items = items[:limit]
	}
	return items, total, truncated, nil
}

func handleV2GeoPreview(w http.ResponseWriter, r *http.Request) {
	var req v2GeoPreviewRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid geo preview request"})
		return
	}
	items, total, truncated, err := v2GeoCategoryItems(strings.TrimSpace(req.Name), req.Category, req.Limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "name": strings.TrimSpace(req.Name), "category": strings.ToLower(strings.TrimSpace(req.Category)),
		"count": len(items), "total": total, "truncated": truncated, "entries": items,
		"production_mutation": false,
	})
}

func handleV2GeoSaveList(w http.ResponseWriter, r *http.Request) {
	var req v2GeoSaveListRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid geo save-list request"})
		return
	}
	expected := "ROUTERFORGE_GEO_LIST_CREATE"
	if req.Replace {
		expected = "ROUTERFORGE_GEO_LIST_REPLACE"
	}
	if req.Confirm != expected {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "geo save-list confirmation mismatch"})
		return
	}
	req.ListName = strings.TrimSpace(req.ListName)
	if !safeListSourceName(req.ListName) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "list_name must be a safe .list filename"})
		return
	}
	items, total, truncated, err := v2GeoCategoryItems(strings.TrimSpace(req.Name), req.Category, req.Limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	content := strings.Join(items, "\n") + "\n"
	if err := validateListContent(content); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	beforeConfig := readStatus().ConfigSHA256
	path := filepath.Join(listsRoot, req.ListName)
	backup := ""
	replaced := false
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "existing list path is not a regular file"})
			return
		}
		if !req.Replace {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "list already exists; explicit replace required"})
			return
		}
		before, readErr := os.ReadFile(path)
		if readErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": readErr.Error()})
			return
		}
		backup, err = createNamedBackup("geo-list-"+req.ListName, before, info.Mode().Perm())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create list backup: " + err.Error()})
			return
		}
		replaced = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": statErr.Error()})
		return
	}
	if err := ensureListSourceRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(path, []byte(content), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write extracted list: " + err.Error(), "backup": backup})
		return
	}
	afterConfig := readStatus().ConfigSHA256
	if afterConfig != beforeConfig {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "production config changed unexpectedly", "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "list_name": req.ListName, "count": len(items), "category_total": total, "truncated": truncated,
		"sha256": listSourceSHA256([]byte(content)), "replaced": replaced, "backup": backup,
		"runtime_restarted": false, "production_config_unchanged": true, "production_mutation": false,
	})
}

func v2GeoAutohostlistCapability() map[string]any {
	status := readStatus()
	configured := strings.Contains(status.Config, "--hostlist-auto") || strings.Contains(status.Config, "auto.list")
	supported := false
	source := "unknown"
	pid, executable, _, err := readProductionNFQWSArgv()
	if err == nil && pid > 0 && executable != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		out, _ := safety.RunCommand(ctx, 256<<10, executable, "--help")
		if strings.Contains(string(out), "--hostlist-auto") {
			supported = true
			source = "installed-binary-help"
		}
	}
	if !supported && configured {
		supported = true
		source = "active-config"
	}
	return map[string]any{
		"supported": supported, "configured": configured, "evidence_source": source,
		"engine": "installed-nfqws2", "routerforge_engine": false, "production_mutation": false,
	}
}

func handleV2GeoAutohostlist(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, v2GeoAutohostlistCapability())
}
