package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// NF5G1: preview is read-only. No file renames, config writes or service actions.
// Only canonical, literal NFQWS list selectors are eligible for a future
// transaction. A stray filename occurrence is ambiguous and fails closed.
var nf5gSelectorPattern = regexp.MustCompile(`--(?:hostlist|hostlist-exclude|hostlist-auto|ipset|ipset-exclude)=([^\s"'\\]+)`)

type nf5gConfigImpact struct {
	Reference    string `json:"reference"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	Selectors    int    `json:"selectors"`
}

type nf5gRenamePreview struct {
	OK                bool               `json:"ok"`
	PreviewOnly       bool               `json:"preview_only"`
	MutationPerformed bool               `json:"mutation_performed"`
	OldName           string             `json:"old_name"`
	NewName           string             `json:"new_name"`
	OldListSHA256     string             `json:"old_list_sha256"`
	NewNameAvailable  bool               `json:"new_name_available"`
	ReferenceCount    int                `json:"reference_count"`
	ConfigChanges     []nf5gConfigImpact `json:"config_changes"`
}

// Return the hypothetical new config and exact selector count. All unrelated
// bytes are preserved so their hashes can anchor the future rename phase.
func nf5gPlanConfigRename(data []byte, listRoot, oldName, newName string) ([]byte, int, error) {
	original := string(data)
	if !safeListSourceName(oldName) || !safeListSourceName(newName) || oldName == newName {
		return nil, 0, errors.New("invalid rename identifiers")
	}
	if nf5fExactListReference(data, newName) {
		return nil, 0, errors.New("new list name is already referenced by a config")
	}
	oldPath := filepath.Join(listRoot, oldName)
	newPath := filepath.Join(listRoot, newName)
	matches := nf5gSelectorPattern.FindAllStringSubmatchIndex(original, -1)
	var out strings.Builder
	out.Grow(len(original) + len(matches)*len(newName))
	cursor, selected := 0, 0
	for _, idx := range matches {
		value := original[idx[2]:idx[3]]
		if filepath.Base(value) != oldName {
			continue
		}
		lineStart := strings.LastIndex(original[:idx[0]], "\n") + 1
		linePrefix := original[lineStart:idx[0]]
		if strings.Contains(linePrefix, "#") || strings.HasPrefix(strings.TrimSpace(linePrefix), ";") {
			return nil, 0, errors.New("list selector appears in a comment or ambiguous line")
		}
		if value != oldPath {
			return nil, 0, fmt.Errorf("list selector uses unsupported path: %s", value)
		}
		out.WriteString(original[cursor:idx[2]])
		out.WriteString(newPath)
		cursor = idx[3]
		selected++
	}
	// Catch comments, shell expansions, unsupported flags and lookalike names.
	// An uncertain reference must never be silently excluded from the plan.
	if strings.Count(original, oldName) != selected {
		return nil, 0, errors.New("ambiguous or unsupported list-name occurrence in config")
	}
	out.WriteString(original[cursor:])
	return []byte(out.String()), selected, nil
}

func nf5gReadConfigImpact(path, label, root, oldName, newName string) (nf5gConfigImpact, error) {
	data, err := nf5bReadReferenceConfig(path)
	if err != nil {
		return nf5gConfigImpact{}, fmt.Errorf("inspect %s: %w", label, err)
	}
	next, n, err := nf5gPlanConfigRename(data, root, oldName, newName)
	if err != nil {
		return nf5gConfigImpact{}, fmt.Errorf("plan %s: %w", label, err)
	}
	return nf5gConfigImpact{Reference: label, BeforeSHA256: listSourceSHA256(data), AfterSHA256: listSourceSHA256(next), Selectors: n}, nil
}

func nf5gPreviewRenameInRoots(listRoot, activePath, libraryRoot, oldName, newName, expected string) (nf5gRenamePreview, error) {
	preview := nf5gRenamePreview{PreviewOnly: true, MutationPerformed: false, ConfigChanges: []nf5gConfigImpact{}}
	if !safeListSourceName(oldName) || !safeListSourceName(newName) || oldName == newName ||
		protectedListName(oldName) || protectedListName(newName) {
		return preview, errors.New("unsafe, protected or identical list name")
	}
	rootInfo, err := os.Lstat(listRoot)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return preview, errors.New("list root is missing or not a real directory")
	}
	oldPath, newPath := filepath.Join(listRoot, oldName), filepath.Join(listRoot, newName)
	data, _, err := nf5aReadRegularList(oldPath)
	if err != nil {
		return preview, fmt.Errorf("read source list: %w", err)
	}
	if err := nf5aRequireListPreimage(data, expected); err != nil {
		return preview, err
	}
	if _, err := os.Lstat(newPath); err == nil {
		return preview, errors.New("destination list filename already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return preview, fmt.Errorf("inspect destination: %w", err)
	}
	preview.OldName, preview.NewName = oldName, newName
	preview.OldListSHA256 = listSourceSHA256(data)
	preview.NewNameAvailable = true

	active, err := nf5gReadConfigImpact(activePath, "active:nfqws2.conf", listRoot, oldName, newName)
	if err != nil {
		return nf5gRenamePreview{}, err
	}
	if active.Selectors != 0 {
		preview.ConfigChanges = append(preview.ConfigChanges, active)
		preview.ReferenceCount += active.Selectors
	}
	libraryInfo, err := os.Lstat(libraryRoot)
	if errors.Is(err, os.ErrNotExist) {
		preview.OK = true
		return preview, nil
	}
	if err != nil || libraryInfo.Mode()&os.ModeSymlink != 0 || !libraryInfo.IsDir() {
		return nf5gRenamePreview{}, errors.New("saved config library is not a real directory")
	}
	entries, err := os.ReadDir(libraryRoot)
	if err != nil {
		return nf5gRenamePreview{}, fmt.Errorf("read saved config library: %w", err)
	}
	if len(entries) > 512 {
		return nf5gRenamePreview{}, errors.New("saved config library exceeds preview scan budget")
	}
	for _, entry := range entries {
		if !safeConfigLibraryName(entry.Name()) {
			continue
		}
		label := "library:" + entry.Name()
		item, err := nf5gReadConfigImpact(filepath.Join(libraryRoot, entry.Name()), label, listRoot, oldName, newName)
		if err != nil {
			return nf5gRenamePreview{}, err
		}
		if item.Selectors > 0 {
			preview.ConfigChanges = append(preview.ConfigChanges, item)
			preview.ReferenceCount += item.Selectors
		}
	}
	sort.Slice(preview.ConfigChanges, func(i, j int) bool { return preview.ConfigChanges[i].Reference < preview.ConfigChanges[j].Reference })
	preview.OK = true
	return preview, nil
}

func registerNF5GRenamePreviewRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/list/rename-preview", getOnly(handleNF5GRenamePreview))
}

func handleNF5GRenamePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	preview, err := nf5gPreviewRenameInRoots(listsRoot, configPath, configLibraryRoot,
		q.Get("old_name"), q.Get("new_name"), q.Get("expected_sha256"))
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "preview_only": true, "mutation_performed": false})
		return
	}
	writeJSON(w, http.StatusOK, preview)
}
