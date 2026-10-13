package main

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// This catalog describes functions found in the router's installed Lua files.
// It never loads/executes Lua and never interprets a submitted shell config.
// Descriptions are advisory, not a substitute for nfqws2 --dry-run.
type nf1bLuaArgument struct {
	Name string `json:"name"`
	Help string `json:"help"`
}

type nf1bLuaEntry struct {
	Name        string            `json:"name"`
	File        string            `json:"file"`
	Standard    string            `json:"standard,omitempty"`
	Description []string          `json:"description"`
	Arguments   []nf1bLuaArgument `json:"arguments"`
	Legacy      string            `json:"legacy,omitempty"`
}

type nf1bLuaCatalog struct {
	ReadOnly     bool           `json:"read_only"`
	Source       string         `json:"source"`
	FilesScanned int            `json:"files_scanned"`
	FilesSkipped int            `json:"files_skipped"`
	Functions    []nf1bLuaEntry `json:"functions"`
}

var nf1bLuaFnRE = regexp.MustCompile(`^\s*function\s+([a-z_][a-z0-9_]*)\s*\(\s*ctx\s*,\s*desync\s*\)`)
var nf1bLuaArgRE = regexp.MustCompile(`^arg\s*:\s*([a-z][a-z0-9_]*)(.*)$`)
var nf1bLuaStdRE = regexp.MustCompile(`^standard\s+args\s*:\s*(.+)$`)
var nf1bLuaOldRE = regexp.MustCompile(`^nfqws1\s*:\s*(.+)$`)

func nf1bLuaShort(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) > limit {
		s = s[:limit]
	}
	return strings.ToValidUTF8(s, "")
}

func nf1bLuaExtract(data string, file string) []nf1bLuaEntry {
	entries := []nf1bLuaEntry{}
	doc := []string{}
	blockComment := false
	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "--[[") {
			blockComment = true
			doc = nil
			continue
		}
		if blockComment {
			if strings.Contains(line, "]] ") || strings.HasPrefix(line, "]]") {
				blockComment = false
			}
			continue
		}
		if strings.HasPrefix(line, "--") {
			if len(doc) < 36 {
				doc = append(doc, nf1bLuaShort(strings.TrimSpace(strings.TrimPrefix(line, "--")), 240))
			}
			continue
		}
		if line == "" {
			continue
		}
		if m := nf1bLuaFnRE.FindStringSubmatch(line); m != nil {
			entry := nf1bLuaEntry{Name: m[1], File: file, Description: []string{}, Arguments: []nf1bLuaArgument{}}
			for _, d := range doc {
				switch {
				case nf1bLuaArgRE.MatchString(d):
					p := nf1bLuaArgRE.FindStringSubmatch(d)
					if len(entry.Arguments) < 36 {
						entry.Arguments = append(entry.Arguments, nf1bLuaArgument{Name: p[1], Help: nf1bLuaShort(p[1]+p[2], 180)})
					}
				case nf1bLuaStdRE.MatchString(d):
					entry.Standard = nf1bLuaShort(nf1bLuaStdRE.FindStringSubmatch(d)[1], 220)
				case nf1bLuaOldRE.MatchString(d):
					entry.Legacy = nf1bLuaShort(nf1bLuaOldRE.FindStringSubmatch(d)[1], 180)
				case d != "":
					if len(entry.Description) < 6 {
						entry.Description = append(entry.Description, nf1bLuaShort(d, 180))
					}
				}
			}
			if len(entries) < 512 {
				entries = append(entries, entry)
			}
		}
		doc = nil
	}
	return entries
}

func nf1bLuaInstalledCatalog() nf1bLuaCatalog {
	out := nf1bLuaCatalog{ReadOnly: true, Source: "installed_lua", Functions: []nf1bLuaEntry{}}
	found := discoverScripts()
	keys := make([]string, 0, len(found))
	for key := range found {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seen := map[string]bool{}
	for _, key := range keys {
		if out.FilesScanned >= 64 {
			out.FilesSkipped++
			continue
		}
		src := found[key]
		raw, truncated, err := readScriptData(src)
		if err != nil || truncated {
			out.FilesSkipped++
			continue
		}
		out.FilesScanned++
		for _, f := range nf1bLuaExtract(string(raw), src.Name) {
			if seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			out.Functions = append(out.Functions, f)
		}
	}
	sort.Slice(out.Functions, func(i, j int) bool { return out.Functions[i].Name < out.Functions[j].Name })
	return out
}

func handleNF1BLuaCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, nf1bLuaInstalledCatalog())
}
