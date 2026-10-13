package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// NF1B1 is deliberately static: it never evaluates shell expressions and does
// not claim that nfqws2 accepted this configuration. Native dry-run is separate.
type nf1bConfigIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Variable string `json:"variable,omitempty"`
	Message  string `json:"message"`
}

type nf1bConfigReport struct {
	OK         bool              `json:"ok"`
	Scope      string            `json:"scope"`
	Issues     []nf1bConfigIssue `json:"issues"`
	Inspected  int               `json:"inspected_variables"`
	Unverified int               `json:"unverified_variables"`
}

var nf1bAssignment = regexp.MustCompile(`^([A-Z][A-Z0-9_]*)=(.*)$`)
var nf1bNumericPort = regexp.MustCompile(`^(\d+)(?:(?:-|:)(\d+))?$`)

func nf1bArgsVariable(name string) bool {
	return name == "NFQWS_BASE_ARGS" || name == "NFQWS_ARGS" || strings.HasPrefix(name, "NFQWS_ARGS_")
}

// nf1bLiteralAssignments extracts only direct literal values of shell
// assignments. A value with expansions is not evaluated or interpreted.
// This intentionally does not attempt to support every shell grammar form.
func nf1bLiteralAssignments(content string) (map[string]string, map[string]bool) {
	variables := map[string]string{}
	unknown := map[string]bool{}
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		raw := strings.TrimSpace(lines[i])
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		matches := nf1bAssignment.FindStringSubmatch(raw)
		if matches == nil {
			continue
		}
		name, value := matches[1], matches[2]
		if strings.HasPrefix(value, "'") || strings.HasPrefix(value, `"`) {
			quote := value[0]
			var b strings.Builder
			escaped := false
			closed := false
			for {
				for p := 1; p < len(value); p++ {
					c := value[p]
					if quote == '"' && escaped {
						escaped = false
						b.WriteByte(c)
						continue
					}
					if quote == '"' && c == '\\' {
						escaped = true
						continue
					}
					if c == quote {
						// Only a full literal assignment is eligible. Anything
						// after the closing quote must be whitespace or a comment.
						suffix := strings.TrimSpace(value[p+1:])
						if suffix != "" && !strings.HasPrefix(suffix, "#") {
							unknown[name] = true
						}
						closed = true
						break
					}
					b.WriteByte(c)
				}
				if closed || i+1 >= len(lines) {
					break
				}
				i++
				b.WriteByte('\n')
				value = `"` + lines[i] // skip synthetic first byte above
			}
			if !closed || strings.ContainsAny(b.String(), "$`") {
				unknown[name] = true
				continue
			}
			if !unknown[name] {
				variables[name] = b.String()
			}
			continue
		}
		value = strings.TrimSpace(strings.SplitN(value, "#", 2)[0])
		if strings.ContainsAny(value, "$`\\;&|(){}") {
			unknown[name] = true
			continue
		}
		variables[name] = value
	}
	return variables, unknown
}

func nf1bPortInvalid(value string) bool {
	if value == "" {
		return true
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		m := nf1bNumericPort.FindStringSubmatch(part)
		if m == nil {
			// Unknown nfqws2 port syntax: do not assert an invalid port.
			continue
		}
		low, _ := strconv.Atoi(m[1])
		if low < 1 || low > 65535 {
			return true
		}
		if m[2] != "" {
			high, _ := strconv.Atoi(m[2])
			if high < 1 || high > 65535 || high < low {
				return true
			}
		}
	}
	return false
}

func nf1bConfigSemantics(content string) nf1bConfigReport {
	report := nf1bConfigReport{OK: true, Scope: "static_literals_only", Issues: []nf1bConfigIssue{}}
	vars, unknown := nf1bLiteralAssignments(content)
	add := func(severity, code, name, message string) {
		if len(report.Issues) >= 80 {
			return
		}
		report.Issues = append(report.Issues, nf1bConfigIssue{Severity: severity, Code: code, Variable: name, Message: message})
		if severity == "error" {
			report.OK = false
		}
	}
	for name := range unknown {
		if nf1bArgsVariable(name) || name == "TCP_PORTS" || name == "UDP_PORTS" {
			report.Unverified++
			add("info", "SHELL_DYNAMIC_UNVERIFIED", name, "Значение содержит shell-подстановку или неподдерживаемую конструкцию; семантика не проверялась")
		}
	}
	for name, value := range vars {
		if !nf1bArgsVariable(name) && name != "TCP_PORTS" && name != "UDP_PORTS" {
			continue
		}
		report.Inspected++
		if name == "TCP_PORTS" || name == "UDP_PORTS" {
			if nf1bPortInvalid(value) {
				add("error", "INVALID_PORT_RANGE", name, "Список портов содержит недопустимый номер или диапазон")
			}
			continue
		}
		// nfqws2 variables contain whitespace-separated command-line options.
		// Comments inside a multiline literal are intentionally ignored here.
		var tokens []string
		for _, line := range strings.Split(value, "\n") {
			fields := strings.Fields(line)
			for _, token := range fields {
				if strings.HasPrefix(token, "#") {
					break
				}
				tokens = append(tokens, token)
			}
		}
		for start := 0; start < len(tokens); {
			end := start
			for end < len(tokens) && tokens[end] != "--new" {
				end++
			}
			profile := tokens[start:end]
			payloadUnknown := false
			hasLuaPayload := false
			luaCandidates := []string{}
			for _, token := range profile {
				if strings.HasPrefix(token, "--dpi-desync") {
					add("error", "LEGACY_NFQWS_V1", name, "Обнаружен --dpi-desync из nfqws первой версии; nfqws2 использует --lua-desync")
				}
				for _, flag := range []string{"--filter-tcp=", "--filter-udp="} {
					if strings.HasPrefix(token, flag) && nf1bPortInvalid(strings.TrimPrefix(token, flag)) {
						add("error", "INVALID_FILTER_PORT", name, fmt.Sprintf("Недопустимый номер или диапазон в %s", flag))
					}
				}
				for _, flag := range []string{"--hostlist=", "--ipset=", "--lua-desync=", "--lua-init="} {
					if token == flag {
						add("error", "EMPTY_OPTION_VALUE", name, "Пустой аргумент "+flag)
					}
				}
				if token == "--payload=unknown" || token == "--payload=empty" {
					payloadUnknown = true
				}
				if strings.HasPrefix(token, "--lua-desync=") {
					luaCandidates = append(luaCandidates, strings.TrimPrefix(token, "--lua-desync="))
				}
			}
			if payloadUnknown {
				for _, lua := range luaCandidates {
					namePart := strings.SplitN(lua, ":", 2)[0]
					if namePart != "fake" && namePart != "multisplit" && namePart != "hostfakesplit" && namePart != "multidisorder" {
						continue
					}
					hasLuaPayload = strings.Contains(lua, "payload=")
					if !hasLuaPayload {
						add("warning", "UNKNOWN_PAYLOAD_LUA", name, "При --payload=unknown/empty функция "+namePart+" без payload= может не обработать пакет; проверьте payload=all")
					}
				}
			}
			if end == len(tokens) {
				break
			}
			start = end + 1
		}
	}
	return report
}
