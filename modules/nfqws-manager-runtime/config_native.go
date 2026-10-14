package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// nf1bNativeReport describes a read-only validation of a candidate config.
// No shell config is sourced or evaluated; the nfqws2 binary receives argv only.
type nf1bNativeReport struct {
	State           string `json:"state"` // passed, failed, not_performed
	Detail          string `json:"detail,omitempty"`
	CheckedProfiles int    `json:"checked_profiles,omitempty"`
}

var nf1bNativeAssignment = regexp.MustCompile(`^([A-Z][A-Z0-9_]*)=(.*)$`)
var nf1bNativeReference = regexp.MustCompile(`\$(?:\{([A-Z][A-Z0-9_]*)\}|([A-Z][A-Z0-9_]*))`)

var nf1bNativeLocateBinary = func() string {
	for _, path := range []string{"/opt/usr/bin/nfqws2", "/usr/bin/nfqws2"} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path
		}
	}
	return ""
}

// Parse only assignment-only files. Control flow, shell functions, command
// substitutions, escaped quotes, and arbitrary expansions are not supported.
// If the input isn't provably representable as argv, no native command runs.
func nf1bNativeParseAssignments(content string) (map[string]string, error) {
	result := map[string]string{}
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := nf1bNativeAssignment.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("строка %d содержит неподдерживаемую shell-конструкцию", i+1)
		}
		name, rhs := match[1], match[2]
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("повторное присваивание %s", name)
		}
		var value string
		if strings.HasPrefix(rhs, `"`) || strings.HasPrefix(rhs, "'") {
			quote := rhs[:1]
			rhs = rhs[1:]
			var parts []string
			for {
				index := strings.Index(rhs, quote)
				if index >= 0 {
					fragment := rhs[:index]
					if strings.Contains(fragment, `\`) {
						return nil, fmt.Errorf("%s: escape-последовательности не поддерживаются", name)
					}
					parts = append(parts, fragment)
					suffix := strings.TrimSpace(rhs[index+1:])
					if suffix != "" && !strings.HasPrefix(suffix, "#") {
						return nil, fmt.Errorf("%s: сложное присваивание", name)
					}
					break
				}
				if strings.Contains(rhs, `\`) {
					return nil, fmt.Errorf("%s: escape-последовательности не поддерживаются", name)
				}
				parts = append(parts, rhs)
				i++
				if i >= len(lines) {
					return nil, fmt.Errorf("%s: незакрытая кавычка", name)
				}
				rhs = lines[i]
			}
			value = strings.Join(parts, "\n")
		} else {
			// Comments in unquoted values are allowed only after whitespace.
			if k := strings.Index(rhs, " #"); k >= 0 {
				rhs = rhs[:k]
			}
			value = strings.TrimSpace(rhs)
			if strings.ContainsAny(value, " \t\r'\"\\;|&()`") {
				return nil, fmt.Errorf("%s: неподдерживаемое присваивание", name)
			}
		}
		if strings.ContainsAny(value, "`\\") || strings.Contains(value, "$(") || strings.Contains(value, "${!") {
			return nil, fmt.Errorf("%s: динамические shell-конструкции не поддерживаются", name)
		}
		result[name] = value
	}
	return result, nil
}

func nf1bNativeResolve(values map[string]string, name string, resolving map[string]bool) (string, error) {
	value, ok := values[name]
	if !ok {
		return "", fmt.Errorf("неизвестная переменная %s", name)
	}
	if resolving[name] {
		return "", fmt.Errorf("цикл переменных: %s", name)
	}
	resolving[name] = true
	defer delete(resolving, name)
	var failure error
	value = nf1bNativeReference.ReplaceAllStringFunc(value, func(token string) string {
		if failure != nil {
			return ""
		}
		key := strings.TrimPrefix(token, "$")
		key = strings.Trim(key, "{}")
		if !strings.HasPrefix(key, "MODE_") {
			failure = fmt.Errorf("неразрешённая ссылка %s", token)
			return ""
		}
		resolved, err := nf1bNativeResolve(values, key, resolving)
		if err != nil {
			failure = err
			return ""
		}
		return resolved
	})
	if failure != nil {
		return "", failure
	}
	if strings.ContainsAny(value, "$`\\") {
		return "", fmt.Errorf("%s: осталась непроверенная shell-подстановка", name)
	}
	return value, nil
}

func nf1bNativeTokens(value string) []string {
	// nfqws2-keenetic passes expanded string values to argv using word splitting.
	// Comments inside quoted strings are literal words, not shell comments.
	return strings.Fields(value)
}

func nf1bNativeArgv(content string) ([]string, int, error) {
	vals, err := nf1bNativeParseAssignments(content)
	if err != nil {
		return nil, 0, err
	}
	get := func(key string) (string, error) {
		if _, ok := vals[key]; !ok {
			return "", nil
		}
		return nf1bNativeResolve(vals, key, map[string]bool{})
	}
	keys := []string{"NFQWS_BASE_ARGS", "NFQWS_ARGS", "NFQWS_ARGS_CUSTOM", "NFQWS_ARGS_UDP", "NFQWS_ARGS_QUIC", "NFQWS_ARGS_IPSET", "NFQWS_EXTRA_ARGS", "USER", "NFQUEUE_NUM", "ISP_INTERFACE", "IPV6_ENABLED", "LOG_LEVEL", "LOG_DEBUG_PATH"}
	resolved := map[string]string{}
	for _, key := range keys {
		v, e := get(key)
		if e != nil {
			return nil, 0, e
		}
		resolved[key] = v
	}
	if strings.TrimSpace(resolved["NFQWS_ARGS"]) == "" {
		return nil, 0, errors.New("нет NFQWS_ARGS — полный аргументный тест невозможен")
	}
	user := resolved["USER"]
	if user == "" {
		user = "nobody"
	}
	if !regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`).MatchString(user) {
		return nil, 0, errors.New("USER имеет неподдерживаемое значение")
	}
	q := resolved["NFQUEUE_NUM"]
	if q == "" {
		q = "300"
	}
	qnum, e := strconv.Atoi(q)
	if e != nil || qnum < 1 || qnum > 65535 {
		return nil, 0, errors.New("NFQUEUE_NUM вне допустимого диапазона")
	}
	args := []string{"--dry-run", "--user=" + user, "--qnum=" + q}
	if strings.TrimSpace(resolved["LOG_LEVEL"]) == "1" {
		logPath := strings.TrimSpace(resolved["LOG_DEBUG_PATH"])
		if logPath == "" {
			logPath = "syslog"
		}
		args = append(args, "--debug="+logPath)
	}
	if len(strings.Fields(resolved["ISP_INTERFACE"])) > 1 {
		args = append(args, "--bind-fix4")
		if strings.TrimSpace(resolved["IPV6_ENABLED"]) != "0" {
			args = append(args, "--bind-fix6")
		}
	}
	base := resolved["NFQWS_BASE_ARGS"]
	args = append(args, nf1bNativeTokens(base)...)
	profiles := 0
	appendProfile := func(parts ...string) {
		list := []string{}
		for _, part := range parts {
			list = append(list, nf1bNativeTokens(part)...)
		}
		if len(list) == 0 {
			return
		}
		if profiles > 0 {
			args = append(args, "--new")
		}
		args = append(args, list...)
		profiles++
	}
	// Model the installed nfqws2-keenetic startup order without invoking its shell.
	custom := nf1bNativeTokens(resolved["NFQWS_ARGS_CUSTOM"])
	if len(custom) > 0 {
		// The upstream custom value may itself contain --new.
		for _, token := range custom {
			if token == "--new" {
				args = append(args, "--new")
				profiles++
				continue
			}
			args = append(args, token)
		}
		profiles++
	}
	appendProfile(resolved["NFQWS_ARGS_UDP"])
	if resolved["NFQWS_ARGS_QUIC"] != "" {
		if resolved["NFQWS_ARGS_IPSET"] != "" {
			appendProfile(resolved["NFQWS_ARGS_QUIC"], resolved["NFQWS_ARGS_IPSET"], "--ipset-ip=0.0.0.0")
		}
		appendProfile(resolved["NFQWS_ARGS_QUIC"], resolved["NFQWS_EXTRA_ARGS"])
	}
	if resolved["NFQWS_ARGS_IPSET"] != "" {
		appendProfile(resolved["NFQWS_ARGS"], resolved["NFQWS_ARGS_IPSET"], "--ipset-ip=0.0.0.0")
	}
	appendProfile(resolved["NFQWS_ARGS"], resolved["NFQWS_EXTRA_ARGS"])
	if len(args) > 2000 {
		return nil, 0, errors.New("слишком много аргументов для нативной проверки")
	}
	return args, profiles, nil
}

func nf1bNativeDryRun(content string) nf1bNativeReport {
	args, profiles, err := nf1bNativeArgv(content)
	if err != nil {
		return nf1bNativeReport{State: "not_performed", Detail: err.Error()}
	}
	path := nf1bNativeLocateBinary()
	if path == "" {
		return nf1bNativeReport{State: "not_performed", Detail: "бинарный nfqws2 не установлен на этом устройстве"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := safety.RunCommand(ctx, 4096, filepath.Clean(path), args...)
	msg := strings.TrimSpace(string(output))
	if len(msg) > 1800 {
		msg = msg[:1800]
	}
	if ctx.Err() != nil {
		return nf1bNativeReport{State: "not_performed", Detail: "native dry-run превышает лимит времени", CheckedProfiles: profiles}
	}
	if err != nil {
		if msg == "" {
			msg = err.Error()
		}
		return nf1bNativeReport{State: "failed", Detail: msg, CheckedProfiles: profiles}
	}
	return nf1bNativeReport{State: "passed", CheckedProfiles: profiles}
}
