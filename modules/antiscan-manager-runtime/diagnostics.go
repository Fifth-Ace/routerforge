package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanUpstreamPinnedSHA     = "f8a052b39d47c86fc3c00983a5502903a1cc2cd9"
	antiscanUpstreamPinnedVersion = "1.10.6"
)

var antiscanUpstreamConfigKeys = []string{
	"ISP_INTERFACES",
	"PORTS",
	"PORTS_FORWARDED",
	"ENABLE_HONEYPOT",
	"HONEYPOT_PORTS",
	"HONEYPOT_BANTIME",
	"ENABLE_IPS_BAN",
	"RULES_MASK",
	"RECENT_CONNECTIONS_TIME",
	"RECENT_CONNECTIONS_HITCOUNT",
	"RECENT_CONNECTIONS_LIMIT",
	"RECENT_CONNECTIONS_BANTIME",
	"DIFFERENT_IP_CANDIDATES_STORAGETIME",
	"DIFFERENT_IP_THRESHOLD",
	"SUBNETS_BANTIME",
	"IPSETS_DIRECTORY",
	"SAVE_IPSETS",
	"SAVE_ON_EXIT",
	"USE_CUSTOM_EXCLUDE_LIST",
	"CUSTOM_LISTS_BLOCK_MODE",
	"GEOBLOCK_MODE",
	"GEOBLOCK_COUNTRIES",
	"GEO_EXCLUDE_COUNTRIES",
	"READ_NDM_LOCKOUT_IPSETS",
	"LOCKOUT_IPSET_BANTIME",
}

var antiscanUpstreamIPSets = []string{
	"ascn_candidates",
	"ascn_ips",
	"ascn_subnets",
	"ascn_custom_exclude",
	"ascn_custom_blacklist",
	"ascn_custom_whitelist",
	"ascn_geo_blacklist",
	"ascn_geo_whitelist",
	"ascn_geo_exclude",
	"ascn_ndm_lockout",
	"ascn_honeypot",
}

var antiscanUpstreamCommands = []string{
	"start",
	"stop",
	"restart",
	"status",
	"list",
	"reload",
	"flush",
	"update_rules",
	"read_candidates",
	"read",
	"read_ndm_ipsets",
	"read_ndm",
	"save_ipsets",
	"save",
	"update_ipsets",
	"update_crontab",
	"retry_load_geo",
	"version",
	"version_opkg",
	"edit",
	"config",
	"conf",
	"crontab",
	"cron",
	"token",
}

var antiscanUpstreamValidTasks = []string{
	"read_candidates",
	"save_ipsets",
	"update_ipsets geo",
	"read_ndm_ipsets",
	"retry_load_geo",
}

var antiscanUpstreamRuntimeFiles = []string{
	"scripts/checks.sh",
	"scripts/config.sh",
	"scripts/crontab.sh",
	"scripts/geo.sh",
	"scripts/ipsets.sh",
	"scripts/iptables.sh",
	"scripts/log.sh",
	"scripts/rci.sh",
	"scripts/show_data.sh",
	"scripts/valid_params",
	"ascn_crontab.conf",
	"ascn_custom_blacklist.txt",
	"ascn_custom_exclude.txt",
	"ascn_custom_whitelist.txt",
}

type antiscanDiagnosticCheck struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	State   string   `json:"state"`
	Summary string   `json:"summary"`
	Details []string `json:"details,omitempty"`
}

type antiscanUpstreamContract struct {
	Repository string   `json:"repository"`
	PinnedSHA  string   `json:"pinned_sha"`
	Version    string   `json:"version"`
	ConfigKeys []string `json:"config_keys"`
	IPSets     []string `json:"ipsets"`
	Commands   []string `json:"commands"`
	ValidTasks []string `json:"valid_tasks"`
}

type antiscanDiagnostics struct {
	GeneratedAt      time.Time                 `json:"generated_at"`
	Detected         bool                      `json:"detected"`
	Running          bool                      `json:"running"`
	InstalledVersion string                    `json:"installed_version,omitempty"`
	Overall          string                    `json:"overall"`
	Contract         antiscanUpstreamContract  `json:"contract"`
	Checks           []antiscanDiagnosticCheck `json:"checks"`
}

func buildAntiscanDiagnostics(parent context.Context, cfg runtimeConfig) antiscanDiagnostics {
	configPath := filepath.Join(cfg.AntiscanDir, "ascn.conf")
	detected := pathExists(cfg.InitScript) || pathExists(configPath) || readAntiscanVersion(cfg) != ""
	running := pathExists(cfg.StatusFile)
	result := antiscanDiagnostics{
		GeneratedAt:      time.Now().UTC(),
		Detected:         detected,
		Running:          running,
		InstalledVersion: readAntiscanVersion(cfg),
		Contract: antiscanUpstreamContract{
			Repository: "dimon27254/antiscan",
			PinnedSHA:  antiscanUpstreamPinnedSHA,
			Version:    antiscanUpstreamPinnedVersion,
			ConfigKeys: append([]string(nil), antiscanUpstreamConfigKeys...),
			IPSets:     append([]string(nil), antiscanUpstreamIPSets...),
			Commands:   append([]string(nil), antiscanUpstreamCommands...),
			ValidTasks: append([]string(nil), antiscanUpstreamValidTasks...),
		},
	}

	result.Checks = append(result.Checks, diagnoseAntiscanContract())
	result.Checks = append(result.Checks, diagnoseAntiscanRuntimeFiles(cfg))
	result.Checks = append(result.Checks, diagnoseAntiscanConfig(configPath))
	result.Checks = append(result.Checks, diagnoseAntiscanInit(cfg.InitScript))
	result.Checks = append(result.Checks, diagnoseAntiscanIPSet())

	iptablesBinary := findAntiscanExecutable(
		"/opt/sbin/iptables",
		"/opt/bin/iptables",
		"/usr/sbin/iptables",
		"/sbin/iptables",
		"iptables",
	)
	result.Checks = append(result.Checks, diagnoseAntiscanIPTables(parent, iptablesBinary, running))
	result.Checks = append(result.Checks, diagnoseAntiscanNetfilter(cfg, running, iptablesBinary))
	result.Checks = append(result.Checks, diagnoseAntiscanHook())
	result.Checks = append(result.Checks, diagnoseAntiscanCron(parent, cfg))
	result.Checks = append(result.Checks, diagnoseAntiscanRCI(cfg))
	result.Checks = append(result.Checks, diagnoseAntiscanLocks(cfg))
	result.Checks = append(result.Checks, diagnoseAntiscanSupportTools())
	result.Overall = diagnosticOverall(result.Checks)
	return result
}

func diagnoseAntiscanContract() antiscanDiagnosticCheck {
	configOK := sameStringSet(antiscanConfigKeys, antiscanUpstreamConfigKeys)
	setsOK := sameStringSet(antiscanKnownSets, antiscanUpstreamIPSets)
	if configOK && setsOK {
		return antiscanDiagnosticCheck{
			ID:      "upstream-contract",
			Label:   "Контракт Antiscan",
			State:   "pass",
			Summary: fmt.Sprintf("Зафиксирован Antiscan %s: %d параметров, %d ipset.", antiscanUpstreamPinnedVersion, len(antiscanUpstreamConfigKeys), len(antiscanUpstreamIPSets)),
			Details: []string{"upstream " + antiscanUpstreamPinnedSHA},
		}
	}
	var details []string
	if !configOK {
		details = append(details, "список параметров RouterForge не совпадает с pinned upstream")
	}
	if !setsOK {
		details = append(details, "список ipset RouterForge не совпадает с pinned upstream")
	}
	return antiscanDiagnosticCheck{ID: "upstream-contract", Label: "Контракт Antiscan", State: "fail", Summary: "Внутренний контракт RouterForge разошёлся с pinned upstream.", Details: details}
}

func diagnoseAntiscanRuntimeFiles(cfg runtimeConfig) antiscanDiagnosticCheck {
	missing := make([]string, 0)
	for _, rel := range antiscanUpstreamRuntimeFiles {
		if !pathExists(filepath.Join(cfg.AntiscanDir, rel)) {
			missing = append(missing, rel)
		}
	}
	if len(missing) == 0 {
		return antiscanDiagnosticCheck{ID: "runtime-files", Label: "Файлы Antiscan", State: "pass", Summary: "Все ожидаемые runtime-файлы Antiscan на месте."}
	}
	return antiscanDiagnosticCheck{ID: "runtime-files", Label: "Файлы Antiscan", State: "fail", Summary: fmt.Sprintf("Не хватает runtime-файлов: %d.", len(missing)), Details: missing}
}

func diagnoseAntiscanConfig(path string) antiscanDiagnosticCheck {
	data, err := os.ReadFile(path)
	if err != nil {
		return antiscanDiagnosticCheck{ID: "config", Label: "ascn.conf", State: "fail", Summary: "Не удалось прочитать ascn.conf.", Details: []string{err.Error()}}
	}
	cfg, err := parseAntiscanConfig(string(data))
	if err != nil {
		return antiscanDiagnosticCheck{ID: "config", Label: "ascn.conf", State: "fail", Summary: "ascn.conf не проходит безопасный разбор RouterForge.", Details: []string{err.Error()}}
	}
	known := make(map[string]bool, len(antiscanUpstreamConfigKeys))
	for _, key := range antiscanUpstreamConfigKeys {
		known[key] = true
	}
	var missing, unknown []string
	for _, key := range antiscanUpstreamConfigKeys {
		if _, ok := cfg.Raw[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range cfg.Raw {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return antiscanDiagnosticCheck{ID: "config", Label: "ascn.conf", State: "fail", Summary: "В ascn.conf есть параметры, которых нет в pinned Antiscan contract.", Details: prefixDetails("неизвестный параметр: ", unknown)}
	}
	if len(missing) > 0 {
		return antiscanDiagnosticCheck{ID: "config", Label: "ascn.conf", State: "warn", Summary: fmt.Sprintf("Файл читается, но отсутствуют %d параметров текущего pinned contract.", len(missing)), Details: prefixDetails("отсутствует: ", missing)}
	}
	return antiscanDiagnosticCheck{ID: "config", Label: "ascn.conf", State: "pass", Summary: fmt.Sprintf("Все %d параметров pinned contract присутствуют.", len(antiscanUpstreamConfigKeys))}
}

func diagnoseAntiscanInit(path string) antiscanDiagnosticCheck {
	info, err := os.Stat(path)
	if err != nil {
		return antiscanDiagnosticCheck{ID: "init", Label: "S99ascn", State: "fail", Summary: "Штатный init-скрипт Antiscan не найден.", Details: []string{err.Error()}}
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return antiscanDiagnosticCheck{ID: "init", Label: "S99ascn", State: "fail", Summary: "S99ascn найден, но не является исполняемым обычным файлом."}
	}
	return antiscanDiagnosticCheck{ID: "init", Label: "S99ascn", State: "pass", Summary: "Штатный init-скрипт Antiscan доступен и исполняем."}
}

func diagnoseAntiscanIPSet() antiscanDiagnosticCheck {
	binary := findAntiscanIPSetBinary()
	if binary == "" {
		return antiscanDiagnosticCheck{ID: "ipset", Label: "ipset", State: "fail", Summary: "Исполняемый файл ipset не найден."}
	}
	return antiscanDiagnosticCheck{ID: "ipset", Label: "ipset", State: "pass", Summary: "ipset доступен.", Details: []string{binary}}
}

func diagnoseAntiscanIPTables(parent context.Context, binary string, running bool) antiscanDiagnosticCheck {
	if binary == "" {
		return antiscanDiagnosticCheck{ID: "iptables", Label: "iptables", State: "fail", Summary: "Исполняемый файл iptables не найден."}
	}
	if !running {
		return antiscanDiagnosticCheck{ID: "iptables", Label: "iptables", State: "pass", Summary: "iptables доступен; цепочка ANTISCAN будет проверена после запуска.", Details: []string{binary}}
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	output, err := safety.RunCommand(ctx, 8<<10, binary, "-w", "-t", "filter", "-L", "ANTISCAN", "-n")
	if err != nil {
		return antiscanDiagnosticCheck{ID: "iptables", Label: "iptables", State: "fail", Summary: "Antiscan отмечен как запущенный, но цепочка ANTISCAN не читается.", Details: compactDiagnosticOutput(output, err)}
	}
	return antiscanDiagnosticCheck{ID: "iptables", Label: "iptables", State: "pass", Summary: "Цепочка ANTISCAN существует и читается.", Details: []string{binary}}
}

func diagnoseAntiscanNetfilter(cfg runtimeConfig, running bool, iptablesBinary string) antiscanDiagnosticCheck {
	cache := "/tmp/ascn_rci.json"
	if data, err := os.ReadFile(cache); err == nil && len(data) > 0 {
		if strings.Contains(string(data), "opkg-kmod-netfilter") {
			return antiscanDiagnosticCheck{ID: "netfilter", Label: "Netfilter", State: "pass", Summary: "Upstream RCI cache подтверждает компонент Netfilter."}
		}
		return antiscanDiagnosticCheck{ID: "netfilter", Label: "Netfilter", State: "fail", Summary: "В upstream RCI cache нет компонента opkg-kmod-netfilter."}
	}
	if running && iptablesBinary != "" && pathExists(cfg.StatusFile) {
		return antiscanDiagnosticCheck{ID: "netfilter", Label: "Netfilter", State: "pass", Summary: "Antiscan запущен; upstream-проверка прошивки уже была пройдена при старте.", Details: []string{"RCI cache сейчас отсутствует, поэтому компонент не перечитывался повторно"}}
	}
	return antiscanDiagnosticCheck{ID: "netfilter", Label: "Netfilter", State: "info", Summary: "RCI cache отсутствует. Upstream проверит компонент Netfilter при следующем запуске Antiscan."}
}

func diagnoseAntiscanHook() antiscanDiagnosticCheck {
	path := "/opt/etc/ndm/netfilter.d/099-ascn.sh"
	info, err := os.Stat(path)
	if err != nil {
		return antiscanDiagnosticCheck{ID: "netfilter-hook", Label: "Netfilter hook", State: "fail", Summary: "099-ascn.sh не найден.", Details: []string{path}}
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return antiscanDiagnosticCheck{ID: "netfilter-hook", Label: "Netfilter hook", State: "fail", Summary: "099-ascn.sh найден, но не исполняем.", Details: []string{path}}
	}
	return antiscanDiagnosticCheck{ID: "netfilter-hook", Label: "Netfilter hook", State: "pass", Summary: "Hook восстановления правил Antiscan установлен.", Details: []string{path}}
}

func diagnoseAntiscanCron(parent context.Context, cfg runtimeConfig) antiscanDiagnosticCheck {
	configPath := filepath.Join(cfg.AntiscanDir, "ascn_crontab.conf")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return antiscanDiagnosticCheck{ID: "cron", Label: "Cron", State: "fail", Summary: "Не удалось прочитать ascn_crontab.conf.", Details: []string{err.Error()}}
	}
	tasks, invalid := parseAntiscanCronTasks(string(data))
	if len(invalid) > 0 {
		return antiscanDiagnosticCheck{ID: "cron", Label: "Cron", State: "fail", Summary: "ascn_crontab.conf содержит задачи вне pinned VALID_TASKS.", Details: invalid}
	}
	binary := findAntiscanExecutable("/opt/bin/crontab", "/usr/bin/crontab", "crontab")
	if binary == "" {
		return antiscanDiagnosticCheck{ID: "cron", Label: "Cron", State: "fail", Summary: "crontab не найден.", Details: []string{fmt.Sprintf("конфигурация содержит %d задач Antiscan", len(tasks))}}
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	output, runErr := safety.RunCommand(ctx, 32<<10, binary, "-l")
	if runErr != nil {
		text := strings.ToLower(string(output))
		if strings.Contains(text, "no crontab") || strings.Contains(text, "no crontab for") {
			return antiscanDiagnosticCheck{ID: "cron", Label: "Cron", State: "warn", Summary: "Файл расписания Antiscan валиден, но пользовательский crontab пуст.", Details: []string{fmt.Sprintf("в ascn_crontab.conf задач: %d", len(tasks))}}
		}
		return antiscanDiagnosticCheck{ID: "cron", Label: "Cron", State: "warn", Summary: "Файл расписания Antiscan валиден, но текущий crontab прочитать не удалось.", Details: compactDiagnosticOutput(output, runErr)}
	}
	installed := countAntiscanCronLines(string(output))
	state := "pass"
	summary := fmt.Sprintf("ascn_crontab.conf валиден: %d задач; в crontab установлено %d строк Antiscan.", len(tasks), installed)
	if len(tasks) > 0 && installed == 0 {
		state = "warn"
		summary = "ascn_crontab.conf валиден, но задачи Antiscan не найдены в активном crontab."
	}
	return antiscanDiagnosticCheck{ID: "cron", Label: "Cron", State: state, Summary: summary, Details: []string{binary}}
}

func diagnoseAntiscanRCI(cfg runtimeConfig) antiscanDiagnosticCheck {
	tokenPath := filepath.Join(cfg.AntiscanDir, ".tkn")
	keyPath := filepath.Join(cfg.AntiscanDir, ".k")
	token := fileNonEmpty(tokenPath)
	key := fileNonEmpty(keyPath)
	if pathExists("/tmp/ascn_ignore_token") {
		return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "pass", Summary: "Upstream пометил прошивку как не требующую token-flow.", Details: []string{"значение токена RouterForge не читает"}}
	}
	authState := strings.TrimSpace(string(readFile("/tmp/ascn_rci_auth")))
	switch authState {
	case "1":
		if token && key {
			return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "pass", Summary: "RCI требует токен; зашифрованные token/key файлы присутствуют.", Details: []string{"значение токена RouterForge не читает"}}
		}
		return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "fail", Summary: "RCI требует токен, но token/key комплект Antiscan неполный.", Details: []string{fmt.Sprintf("token=%t key=%t", token, key)}}
	case "0":
		return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "pass", Summary: "Upstream cache сообщает: RCI-токен для этой прошивки не требуется."}
	default:
		if token != key {
			return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "warn", Summary: "Требование RCI ещё не закэшировано, а token/key комплект неполный.", Details: []string{fmt.Sprintf("token=%t key=%t", token, key)}}
		}
		if token && key {
			return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "info", Summary: "Token/key файлы присутствуют; требование RCI будет подтверждено upstream при обращении к роутеру.", Details: []string{"значение токена RouterForge не читает"}}
		}
		return antiscanDiagnosticCheck{ID: "rci", Label: "RCI / token", State: "info", Summary: "Требование RCI-токена ещё не определено upstream. Это будет проверено при запуске Antiscan."}
	}
}

func diagnoseAntiscanLocks(cfg runtimeConfig) antiscanDiagnosticCheck {
	var locks []string
	if pathExists(cfg.ConfigLockFile) {
		locks = append(locks, cfg.ConfigLockFile)
	}
	if pathExists(cfg.GeoLockFile) {
		locks = append(locks, cfg.GeoLockFile)
	}
	if len(locks) == 0 {
		return antiscanDiagnosticCheck{ID: "locks", Label: "Runtime locks", State: "pass", Summary: "Зависших lock-файлов Antiscan нет."}
	}
	return antiscanDiagnosticCheck{ID: "locks", Label: "Runtime locks", State: "warn", Summary: "Antiscan сейчас выполняет конфигурационную или Geo-операцию.", Details: locks}
}

func diagnoseAntiscanSupportTools() antiscanDiagnosticCheck {
	var missing, found []string
	for _, name := range []string{"curl", "jq"} {
		path := findAntiscanExecutable(filepath.Join("/opt/bin", name), filepath.Join("/opt/sbin", name), name)
		if path == "" {
			missing = append(missing, name)
		} else {
			found = append(found, name+"="+path)
		}
	}
	if len(missing) > 0 {
		return antiscanDiagnosticCheck{ID: "support-tools", Label: "curl / jq", State: "fail", Summary: "Не хватает обязательных утилит Antiscan.", Details: prefixDetails("не найден: ", missing)}
	}
	return antiscanDiagnosticCheck{ID: "support-tools", Label: "curl / jq", State: "pass", Summary: "Обязательные пользовательские утилиты доступны.", Details: found}
}

func parseAntiscanCronTasks(text string) ([]string, []string) {
	valid := make(map[string]bool, len(antiscanUpstreamValidTasks))
	for _, task := range antiscanUpstreamValidTasks {
		valid[task] = true
	}
	var tasks, invalid []string
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			invalid = append(invalid, fmt.Sprintf("строка %d: некорректный cron", i+1))
			continue
		}
		command := strings.Join(fields[5:], " ")
		const prefix = "/opt/etc/init.d/S99ascn "
		idx := strings.Index(command, prefix)
		if idx < 0 {
			invalid = append(invalid, fmt.Sprintf("строка %d: не S99ascn", i+1))
			continue
		}
		task := strings.TrimSpace(command[idx+len(prefix):])
		task = strings.TrimSpace(strings.TrimSuffix(task, "&"))
		if !valid[task] {
			invalid = append(invalid, fmt.Sprintf("строка %d: неподдерживаемая задача %q", i+1, task))
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, invalid
}

func countAntiscanCronLines(text string) int {
	count := 0
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" && !strings.HasPrefix(line, "#") && strings.Contains(line, "/opt/etc/init.d/S99ascn ") {
			count++
		}
	}
	return count
}

func findAntiscanExecutable(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, "/") {
			info, err := os.Stat(candidate)
			if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func diagnosticOverall(checks []antiscanDiagnosticCheck) string {
	overall := "pass"
	for _, check := range checks {
		switch check.State {
		case "fail":
			return "fail"
		case "warn":
			overall = "warn"
		}
	}
	return overall
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func prefixDetails(prefix string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, prefix+value)
	}
	return out
}

func fileNonEmpty(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func compactDiagnosticOutput(output []byte, err error) []string {
	text := strings.TrimSpace(string(output))
	if len(text) > 500 {
		text = text[:500] + "…"
	}
	var details []string
	if text != "" {
		details = append(details, text)
	}
	if err != nil {
		details = append(details, err.Error())
	}
	return details
}
