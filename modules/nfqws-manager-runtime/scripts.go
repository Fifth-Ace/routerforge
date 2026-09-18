package main

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	scriptsRoot        = "/opt/etc/nfqws2/lua"
	scriptFileMaxBytes = 512 << 10
)

var scriptRoots = []string{
	scriptsRoot,
	"/opt/share/zapret2/lua",
	"/opt/usr/share/zapret2/lua",
	"/opt/share/nfqws2/lua",
	"/opt/usr/share/nfqws2/lua",
}

var scriptPathPattern = regexp.MustCompile(`/opt/[A-Za-z0-9_./-]+\.lua(?:\.gz)?`)

type scriptSource struct {
	Name       string
	Logical    string
	Resolved   string
	Compressed bool
	Info       os.FileInfo
}

func safeScriptName(name string) bool {
	if !safeListName(name) {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".lua")
}

func scriptDisplayName(name string) (string, bool) {
	base := filepath.Base(name)
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, ".lua.gz"):
		return base[:len(base)-3], true
	case strings.HasSuffix(lower, ".lua"):
		return base, true
	default:
		return "", false
	}
}

func scriptTargetAllowed(path string) bool {
	clean := filepath.Clean(path)
	lower := strings.ToLower(clean)
	if !strings.HasSuffix(lower, ".lua") && !strings.HasSuffix(lower, ".lua.gz") {
		return false
	}
	rel, err := filepath.Rel("/opt", clean)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func resolveScriptFile(logical string) (scriptSource, error) {
	name, ok := scriptDisplayName(logical)
	if !ok || !safeScriptName(name) {
		return scriptSource{}, errors.New("invalid lua script filename")
	}
	resolved, err := filepath.EvalSymlinks(logical)
	if err != nil {
		return scriptSource{}, err
	}
	resolved = filepath.Clean(resolved)
	if !scriptTargetAllowed(resolved) {
		return scriptSource{}, errors.New("lua script target must stay below /opt and end with .lua or .lua.gz")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return scriptSource{}, err
	}
	if !info.Mode().IsRegular() {
		return scriptSource{}, errors.New("lua script target must be a regular file")
	}
	return scriptSource{
		Name:       name,
		Logical:    filepath.Clean(logical),
		Resolved:   resolved,
		Compressed: strings.HasSuffix(strings.ToLower(resolved), ".gz"),
		Info:       info,
	}, nil
}

func addScriptSource(dst map[string]scriptSource, logical string) {
	source, err := resolveScriptFile(logical)
	if err != nil {
		return
	}
	key := strings.ToLower(source.Name)
	current, exists := dst[key]
	if !exists || (current.Compressed && !source.Compressed) {
		dst[key] = source
	}
}

func configScriptPaths() []string {
	data, _, err := readBoundedFile(configPath, configMaxBytes)
	if err != nil {
		return nil
	}
	matches := scriptPathPattern.FindAllString(string(data), -1)
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, path := range matches {
		path = filepath.Clean(path)
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

func discoverScripts() map[string]scriptSource {
	found := map[string]scriptSource{}
	for _, root := range scriptRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if _, ok := scriptDisplayName(entry.Name()); !ok {
				continue
			}
			addScriptSource(found, filepath.Join(root, entry.Name()))
		}
	}
	for _, path := range configScriptPaths() {
		addScriptSource(found, path)
		if strings.HasSuffix(strings.ToLower(path), ".lua") {
			addScriptSource(found, path+".gz")
		}
	}
	return found
}

func readScriptData(source scriptSource) ([]byte, bool, error) {
	file, err := os.Open(source.Resolved)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	var reader io.Reader = file
	var gz *gzip.Reader
	if source.Compressed {
		gz, err = gzip.NewReader(file)
		if err != nil {
			return nil, false, err
		}
		defer gz.Close()
		reader = gz
	}

	data, err := io.ReadAll(io.LimitReader(reader, scriptFileMaxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > scriptFileMaxBytes {
		return data[:scriptFileMaxBytes], true, nil
	}
	return data, false, nil
}

func readScriptInventory() []filePreview {
	found := discoverScripts()
	out := make([]filePreview, 0, len(found))
	for _, source := range found {
		out = append(out, filePreview{
			Name:      source.Name,
			Path:      source.Logical,
			Size:      source.Info.Size(),
			Modified:  source.Info.ModTime().UTC().Format(time.RFC3339),
			Truncated: source.Info.Size() > scriptFileMaxBytes,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func handleScripts(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	found := discoverScripts()
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"root":      scriptsRoot,
			"roots":     scriptRoots,
			"files":     readScriptInventory(),
			"read_only": true,
		})
		return
	}
	if !safeScriptName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid lua script filename"})
		return
	}
	source, ok := found[strings.ToLower(name)]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "lua script not found"})
		return
	}
	data, cut, err := readScriptData(source)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read lua script: " + err.Error()})
		return
	}
	if cut {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "lua script exceeds viewer limit"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        source.Name,
		"path":        source.Logical,
		"content":     string(data),
		"size":        len(data),
		"read_only":   true,
		"compressed":  source.Compressed,
		"modified_at": source.Info.ModTime().UTC().Format(time.RFC3339),
	})
}
