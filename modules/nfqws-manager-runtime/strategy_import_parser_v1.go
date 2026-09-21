package main

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const v2StrategyImportParseMax = 256 << 10

type v2StrategyImportWarning struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

type v2StrategyImportTransform struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type v2StrategyImportDependency struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type v2StrategyImportLogSummary struct {
	Format     string `json:"format,omitempty"`
	Strategies int    `json:"strategies,omitempty"`
	Targets    int    `json:"targets,omitempty"`
	Tests      int    `json:"tests,omitempty"`
	Passed     int    `json:"passed,omitempty"`
	Failed     int    `json:"failed,omitempty"`
}

type v2StrategyImportParseRequest struct {
	Content string `json:"content"`
}

type v2StrategyImportParseResponse struct {
	OK         bool                 `json:"ok"`
	Type       string               `json:"type"`
	Commands   []string             `json:"commands"`
	Profiles   []string             `json:"profiles"`
	TCP        string               `json:"tcp,omitempty"`
	UDP        string               `json:"udp,omitempty"`
	Transforms []v2StrategyImportTransform  `json:"transforms"`
	Warnings   []v2StrategyImportWarning    `json:"warnings"`
	Deps       []v2StrategyImportDependency `json:"deps"`
	Preview    string               `json:"preview,omitempty"`
	Ready      bool                 `json:"ready"`
	Log        v2StrategyImportLogSummary   `json:"log,omitempty"`

	ProductionMutation bool `json:"production_mutation"`
}

var (
	v2StrategyImportSetRE     = regexp.MustCompile(`(?i)^set\s+"?([A-Za-z_][A-Za-z0-9_]*)=([^"]*)"?\s*$`)
	v2StrategyImportVarRE     = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)
	v2StrategyImportDepRE     = regexp.MustCompile(`(--[A-Za-z0-9_-]+)=([^\s]+)`)
	v2StrategyImportFormat1RE = regexp.MustCompile(`(?i)^Config:\s+(.+?)\s+\(Type:`)
	v2StrategyImportFormat2RE = regexp.MustCompile(`^\[(\d+)/(\d+)\]\s+(.+)$`)
	v2StrategyImportTarget1RE = regexp.MustCompile(`^\s*Target:\s+(.+?)\s+\((.+?)\)$`)
	v2StrategyImportTarget2RE = regexp.MustCompile(`^===\s+(.+?)\s+\[(.+?)\]\s+===$`)
	v2StrategyImportResult1RE = regexp.MustCompile(`(?i)^\s+(HTTP|TLS1\.2|TLS1\.3):\s+code=(\d+)\s+size=([\d.]+)\s+(KB|MB|bytes?)\s+status=(OK|FAIL|LIKELY_BLOCKED)$`)
	v2StrategyImportResult2RE = regexp.MustCompile(`^\[(.+?)\]\[(.+?)\]\s+code=(\d+)\s+size=(\d+)\s+bytes.*status=(OK|FAIL)$`)
)

func v2StrategyImportDetectType(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r", ""), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if v2StrategyImportFormat1RE.MatchString(line) || v2StrategyImportFormat2RE.MatchString(line) {
			return "log"
		}
	}
	lower := strings.ToLower(content)
	hasExec := strings.Contains(lower, "winws.exe") || strings.Contains(lower, "nfqws.exe") || strings.Contains(lower, "nfqws ")
	if hasExec {
		for _, raw := range lines {
			line := strings.TrimSpace(strings.ToLower(raw))
			if strings.HasPrefix(line, "@echo") || strings.HasPrefix(line, "set ") || strings.HasPrefix(line, "start ") ||
				strings.HasPrefix(line, "call ") || strings.HasPrefix(line, "chcp ") || strings.Contains(line, "%~dp0") {
				return "batch"
			}
		}
		return "command"
	}
	if strings.Contains(lower, "--filter-") || strings.Contains(lower, "--dpi-desync") || strings.Contains(lower, "--wf-") {
		return "command"
	}
	return "unknown"
}

func v2StrategyImportLogicalLines(content string) []string {
	var out []string
	buf := ""
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r", ""), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" && buf == "" {
			continue
		}
		cont := strings.HasSuffix(line, "^")
		if cont {
			line = strings.TrimSpace(strings.TrimSuffix(line, "^"))
		}
		if buf == "" {
			buf = line
		} else if line != "" {
			buf += " " + line
		}
		if !cont {
			if strings.TrimSpace(buf) != "" {
				out = append(out, strings.TrimSpace(buf))
			}
			buf = ""
		}
	}
	if strings.TrimSpace(buf) != "" {
		out = append(out, strings.TrimSpace(buf))
	}
	return out
}

func v2StrategyImportVariables(content string) map[string]string {
	out := map[string]string{}
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r", ""), "\n") {
		if m := v2StrategyImportSetRE.FindStringSubmatch(strings.TrimSpace(raw)); len(m) == 3 {
			out[strings.ToUpper(m[1])] = strings.TrimSpace(m[2])
		}
	}
	return out
}

func v2StrategyImportExtractCommands(content string) []string {
	var out []string
	for _, line := range v2StrategyImportLogicalLines(content) {
		lower := strings.ToLower(line)
		idx, n := strings.Index(lower, "winws.exe"), len("winws.exe")
		if idx < 0 {
			idx, n = strings.Index(lower, "nfqws.exe"), len("nfqws.exe")
		}
		if idx < 0 {
			idx, n = strings.Index(lower, "nfqws"), len("nfqws")
		}
		if idx < 0 {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[idx+n:]), `"`))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func v2StrategyImportAppendTransform(dst *[]v2StrategyImportTransform, kind, text string) {
	for _, item := range *dst {
		if item.Kind == kind && item.Text == text {
			return
		}
	}
	*dst = append(*dst, v2StrategyImportTransform{Kind: kind, Text: text})
}

func v2StrategyImportAppendWarning(dst *[]v2StrategyImportWarning, level, text string) {
	for _, item := range *dst {
		if item.Level == level && item.Text == text {
			return
		}
	}
	*dst = append(*dst, v2StrategyImportWarning{Level: level, Text: text})
}

func v2StrategyImportExpand(raw string, vars map[string]string, transforms *[]v2StrategyImportTransform, warnings *[]v2StrategyImportWarning) string {
	out := raw
	replacements := []struct{ old, new, label string }{
		{`%~dp0bin\`, `/opt/etc/nfqws2/blobs/`, `%~dp0bin\ -> /opt/etc/nfqws2/blobs/`},
		{`%~dp0lists\`, `/opt/etc/nfqws2/lists/`, `%~dp0lists\ -> /opt/etc/nfqws2/lists/`},
		{`%BIN%`, `/opt/etc/nfqws2/blobs/`, `%BIN% -> /opt/etc/nfqws2/blobs/`},
		{`%LISTS%`, `/opt/etc/nfqws2/lists/`, `%LISTS% -> /opt/etc/nfqws2/lists/`},
		{`%GameFilter%`, `1024-65535`, `%GameFilter% -> 1024-65535`},
	}
	for _, rep := range replacements {
		before := out
		out = strings.ReplaceAll(out, rep.old, rep.new)
		out = strings.ReplaceAll(out, strings.ToLower(rep.old), rep.new)
		if out != before {
			v2StrategyImportAppendTransform(transforms, "PATH", rep.label)
		}
	}
	out = v2StrategyImportVarRE.ReplaceAllStringFunc(out, func(all string) string {
		name := strings.ToUpper(strings.Trim(all, "%"))
		if value, ok := vars[name]; ok && !strings.Contains(strings.ToLower(value), "%~dp0") {
			v2StrategyImportAppendTransform(transforms, "VAR", all+" -> "+value)
			return value
		}
		v2StrategyImportAppendWarning(warnings, "bad", "Unresolved variable "+all)
		return all
	})
	if strings.Contains(out, "^!") {
		out = strings.ReplaceAll(out, "^!", "!")
		v2StrategyImportAppendTransform(transforms, "CMD", "^! -> !")
	}
	if strings.Contains(out, "^^") {
		out = strings.ReplaceAll(out, "^^", "^")
		v2StrategyImportAppendTransform(transforms, "CMD", "^^ -> ^")
	}
	out = strings.ReplaceAll(out, `\`, "/")
	out = strings.ReplaceAll(out, `"`, "")
	compat := []struct{ old, new string }{
		{"quic_initial_www_google_com.bin", "quic_initial.bin"},
		{"quic_initial_max_ru.bin", "quic_initial.bin"},
		{"tls_clienthello_www_google_com.bin", "tls_clienthello.bin"},
		{"tls_clienthello_max_ru.bin", "tls_clienthello.bin"},
	}
	for _, rep := range compat {
		if strings.Contains(strings.ToLower(out), strings.ToLower(rep.old)) {
			re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(rep.old))
			out = re.ReplaceAllString(out, rep.new)
			v2StrategyImportAppendTransform(transforms, "COMPAT", rep.old+" -> "+rep.new)
		}
	}
	return strings.Join(strings.Fields(out), " ")
}

func v2StrategyImportSplitProfiles(args string) []string {
	re := regexp.MustCompile(`(?i)\s+--new\s+`)
	parts := re.Split(strings.TrimSpace(args), -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func v2StrategyImportDependencyKind(flag, value string) string {
	flag = strings.ToLower(flag)
	switch flag {
	case "--hostlist", "--hostlist-exclude", "--hostlist-auto", "--ipset", "--ipset-exclude":
		return "LIST"
	case "--dpi-desync-split-seqovl-pattern":
		return "BLOB"
	}
	if strings.HasPrefix(flag, "--dpi-desync-fake-") {
		lower := strings.ToLower(value)
		if strings.HasPrefix(lower, "0x") || lower == "!" || lower == "?" || lower == "none" || lower == "rnd" {
			return ""
		}
		if strings.ContainsAny(value, `/\`) || strings.HasSuffix(lower, ".bin") || strings.HasSuffix(lower, ".dat") || strings.HasSuffix(lower, ".raw") {
			return "BLOB"
		}
	}
	return ""
}

func v2StrategyImportDependencies(args string, warnings *[]v2StrategyImportWarning) []v2StrategyImportDependency {
	seen := map[string]bool{}
	out := []v2StrategyImportDependency{}
	for _, m := range v2StrategyImportDepRE.FindAllStringSubmatch(args, -1) {
		kind := v2StrategyImportDependencyKind(m[1], m[2])
		if kind == "" {
			continue
		}
		key := kind + "\x00" + m[2]
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v2StrategyImportDependency{Kind: kind, Path: m[2]})
		if strings.Contains(m[2], "%") || strings.Contains(m[2], `\`) || regexp.MustCompile(`^[A-Za-z]:`).MatchString(m[2]) {
			v2StrategyImportAppendWarning(warnings, "warn", "Windows/path variable remains in dependency: "+m[2])
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func v2StrategyImportParseLog(content string) v2StrategyImportLogSummary {
	lines := strings.Split(strings.ReplaceAll(content, "\r", ""), "\n")
	s := v2StrategyImportLogSummary{}
	targets := map[string]bool{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case v2StrategyImportFormat1RE.MatchString(line):
			s.Format = "format1"
			s.Strategies++
		case v2StrategyImportFormat2RE.MatchString(line):
			if s.Format == "" {
				s.Format = "format2"
			}
			s.Strategies++
		case v2StrategyImportTarget1RE.MatchString(raw):
			m := v2StrategyImportTarget1RE.FindStringSubmatch(raw)
			if len(m) > 1 {
				targets[m[1]] = true
			}
		case v2StrategyImportTarget2RE.MatchString(line):
			m := v2StrategyImportTarget2RE.FindStringSubmatch(line)
			if len(m) > 1 {
				targets[m[1]] = true
			}
		case v2StrategyImportResult1RE.MatchString(raw):
			m := v2StrategyImportResult1RE.FindStringSubmatch(raw)
			s.Tests++
			if strings.EqualFold(m[5], "OK") {
				s.Passed++
			} else {
				s.Failed++
			}
		case v2StrategyImportResult2RE.MatchString(line):
			m := v2StrategyImportResult2RE.FindStringSubmatch(line)
			s.Tests++
			if m[5] == "OK" {
				s.Passed++
			} else {
				s.Failed++
			}
		}
	}
	s.Targets = len(targets)
	return s
}

func v2StrategyImportParse(content string) (v2StrategyImportParseResponse, error) {
	if len(content) == 0 || len(content) > v2StrategyImportParseMax {
		return v2StrategyImportParseResponse{}, errors.New("strategy source is empty or exceeds 256 KiB")
	}
	resp := v2StrategyImportParseResponse{
		OK: true, Type: v2StrategyImportDetectType(content),
		Commands: []string{}, Profiles: []string{}, Transforms: []v2StrategyImportTransform{},
		Warnings: []v2StrategyImportWarning{}, Deps: []v2StrategyImportDependency{}, ProductionMutation: false,
	}
	if resp.Type == "log" {
		resp.Log = v2StrategyImportParseLog(content)
		v2StrategyImportAppendWarning(&resp.Warnings, "warn", "Test log recognized; a .bat/command is still required to build an NFQWS candidate.")
		return resp, nil
	}
	resp.Commands = v2StrategyImportExtractCommands(content)
	if len(resp.Commands) == 0 {
		v2StrategyImportAppendWarning(&resp.Warnings, "bad", "No winws.exe/nfqws strategy command found.")
		return resp, nil
	}
	if len(resp.Commands) > 1 {
		v2StrategyImportAppendWarning(&resp.Warnings, "bad", "Multiple strategy process launches found ("+strconv.Itoa(len(resp.Commands))+"); RouterForge will not merge them automatically.")
	}
	args := v2StrategyImportExpand(resp.Commands[0], v2StrategyImportVariables(content), &resp.Transforms, &resp.Warnings)
	wfTCP := regexp.MustCompile(`(?i)--wf-tcp=([^\s]+)`)
	wfUDP := regexp.MustCompile(`(?i)--wf-udp=([^\s]+)`)
	if m := wfTCP.FindStringSubmatch(args); len(m) == 2 {
		resp.TCP = m[1]
		args = wfTCP.ReplaceAllString(args, "")
		v2StrategyImportAppendTransform(&resp.Transforms, "WF", "--wf-tcp -> TCP_PORTS preview")
	}
	if m := wfUDP.FindStringSubmatch(args); len(m) == 2 {
		resp.UDP = m[1]
		args = wfUDP.ReplaceAllString(args, "")
		v2StrategyImportAppendTransform(&resp.Transforms, "WF", "--wf-udp -> UDP_PORTS preview")
	}
	args = strings.Join(strings.Fields(args), " ")
	resp.Profiles = v2StrategyImportSplitProfiles(args)
	if len(resp.Profiles) > 1 {
		v2StrategyImportAppendTransform(&resp.Transforms, "PROFILE", "Preserved "+strconv.Itoa(len(resp.Profiles))+" profile order through --new")
	}
	resp.Deps = v2StrategyImportDependencies(args, &resp.Warnings)
	if v2StrategyImportVarRE.MatchString(resp.TCP + " " + resp.UDP + " " + args) {
		for _, m := range v2StrategyImportVarRE.FindAllString(resp.TCP+" "+resp.UDP+" "+args, -1) {
			v2StrategyImportAppendWarning(&resp.Warnings, "bad", "Unresolved variable "+m)
		}
	}
	var preview []string
	preview = append(preview, "# RouterForge strategy import preview", "# PREVIEW ONLY - not saved or applied")
	if resp.TCP != "" {
		preview = append(preview, `TCP_PORTS="`+resp.TCP+`"`)
	} else {
		preview = append(preview, "# TCP_PORTS: not present in source")
	}
	if resp.UDP != "" {
		preview = append(preview, `UDP_PORTS="`+resp.UDP+`"`)
	} else {
		preview = append(preview, "# UDP_PORTS: not present in source")
	}
	preview = append(preview, "", `NFQWS_ARGS_CUSTOM="`)
	for i, profile := range resp.Profiles {
		if i < len(resp.Profiles)-1 {
			preview = append(preview, profile+" --new")
		} else {
			preview = append(preview, profile)
		}
	}
	preview = append(preview, `"`)
	resp.Preview = strings.Join(preview, "\n")
	resp.Ready = len(resp.Commands) == 1 && len(resp.Profiles) > 0
	for _, warning := range resp.Warnings {
		if warning.Level == "bad" {
			resp.Ready = false
		}
	}
	return resp, nil
}

func handleV2StrategyImportParse(w http.ResponseWriter, r *http.Request) {
	var req v2StrategyImportParseRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy import parse request"})
		return
	}
	resp, err := v2StrategyImportParse(req.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
