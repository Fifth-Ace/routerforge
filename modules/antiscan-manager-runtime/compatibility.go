package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	antiscanUpdateCachePath         = "/tmp/ascn_update.json"
	antiscanRCICachePath            = "/tmp/ascn_rci.json"
	antiscanNetfilterHookPath       = "/opt/etc/ndm/netfilter.d/099-ascn.sh"
	antiscanUpdateCacheFreshSeconds = int64(900)
)

type antiscanCompatibilityUpdate struct {
	CachePresent     bool   `json:"cache_present"`
	CacheFresh       bool   `json:"cache_fresh"`
	CacheAgeSeconds  int64  `json:"cache_age_seconds,omitempty"`
	CacheError       string `json:"cache_error,omitempty"`
	LegacyVersion    string `json:"legacy_version,omitempty"`
	MainVersion      string `json:"main_version,omitempty"`
	MainMinOS        string `json:"main_min_os,omitempty"`
	EligibilityKnown bool   `json:"eligibility_known"`
	Available        bool   `json:"available"`
	AvailableVersion string `json:"available_version,omitempty"`
	AvailableChannel string `json:"available_channel,omitempty"`
	Source           string `json:"source"`
}

type antiscanCompatibilityFirmware struct {
	CachePresent     bool   `json:"cache_present"`
	Version          string `json:"version,omitempty"`
	NetfilterKnown   bool   `json:"netfilter_known"`
	NetfilterPresent bool   `json:"netfilter_present"`
}

type antiscanCompatibilityHook struct {
	Path       string `json:"path"`
	Present    bool   `json:"present"`
	Executable bool   `json:"executable"`
}

type antiscanCompatibility struct {
	GeneratedAt         time.Time                     `json:"generated_at"`
	Detected            bool                          `json:"detected"`
	Running             bool                          `json:"running"`
	ScriptVersion       string                        `json:"script_version,omitempty"`
	PackageVersion      string                        `json:"package_version,omitempty"`
	PackageControlPath  string                        `json:"package_control_path,omitempty"`
	PinnedVersion       string                        `json:"pinned_version"`
	PinnedSHA           string                        `json:"pinned_sha"`
	ContractExact       bool                          `json:"contract_exact"`
	PackageRuntimeMatch bool                          `json:"package_runtime_match"`
	State               string                        `json:"state"`
	Summary             string                        `json:"summary"`
	Update              antiscanCompatibilityUpdate   `json:"update"`
	Firmware            antiscanCompatibilityFirmware `json:"firmware"`
	Hook                antiscanCompatibilityHook     `json:"hook"`
	UpdateOwner         string                        `json:"update_owner"`
	UpdateAPI           bool                          `json:"update_api"`
	AutoUpdate          bool                          `json:"auto_update"`
	Warnings            []string                      `json:"warnings"`
}

func buildAntiscanCompatibility(cfg runtimeConfig) antiscanCompatibility {
	scriptVersion := parseScriptVersion(string(readFile(cfg.InitScript)))
	packageVersion, packagePath := readAntiscanPackageVersion()
	firmware := readAntiscanFirmwareCompatibility()
	hook := readAntiscanHookCompatibility()
	update := readAntiscanCachedUpdate(scriptVersion, firmware)

	detected := pathExists(cfg.InitScript) || pathExists(filepath.Join(cfg.AntiscanDir, "ascn.conf")) || scriptVersion != "" || packageVersion != ""
	contractExact := scriptVersion != "" && scriptVersion == antiscanUpstreamPinnedVersion
	packageRuntimeMatch := false
	if scriptVersion != "" && packageVersion != "" {
		packageRuntimeMatch = antiscanVersionUpstreamPart(packageVersion) == scriptVersion
	}

	result := antiscanCompatibility{
		GeneratedAt:         time.Now().UTC(),
		Detected:            detected,
		Running:             pathExists(cfg.StatusFile),
		ScriptVersion:       scriptVersion,
		PackageVersion:      packageVersion,
		PackageControlPath:  packagePath,
		PinnedVersion:       antiscanUpstreamPinnedVersion,
		PinnedSHA:           antiscanUpstreamPinnedSHA,
		ContractExact:       contractExact,
		PackageRuntimeMatch: packageRuntimeMatch,
		State:               "pass",
		Summary:             "Установленная версия Antiscan совпадает с проверенным контрактом RouterForge.",
		Update:              update,
		Firmware:            firmware,
		Hook:                hook,
		UpdateOwner:         "upstream-opkg",
		UpdateAPI:           false,
		AutoUpdate:          false,
		Warnings:            []string{},
	}

	if !detected {
		result.State = "info"
		result.Summary = "Antiscan не обнаружен; совместимость проверить нельзя."
		return result
	}
	if scriptVersion == "" {
		result.State = "fail"
		result.Summary = "Не удалось определить runtime-версию Antiscan из S99ascn."
		result.Warnings = append(result.Warnings, "RouterForge не выполняет обновление или восстановление пакета автоматически.")
		return result
	}
	if !contractExact {
		result.State = "warn"
		result.Summary = "Runtime-версия Antiscan отличается от проверенного RouterForge контракта."
		result.Warnings = append(result.Warnings, "Совместимость этой runtime-версии не подтверждена pinned upstream-контрактом RouterForge.")
	}
	if packageVersion == "" {
		if result.State == "pass" {
			result.State = "warn"
			result.Summary = "Runtime-версия совместима, но OPKG metadata Antiscan не найдена."
		}
		result.Warnings = append(result.Warnings, "OPKG metadata отсутствует; соответствие пакета и runtime проверить нельзя.")
	} else if !packageRuntimeMatch {
		if result.State == "pass" {
			result.State = "warn"
			result.Summary = "Версия OPKG-пакета Antiscan не совпадает с runtime S99ascn."
		}
		result.Warnings = append(result.Warnings, "Версия пакета и ASCN_VERSION расходятся; обновление должно оставаться под управлением upstream OPKG.")
	}

	if !update.CachePresent {
		result.Warnings = append(result.Warnings, "Upstream update cache отсутствует; RouterForge не обращается в сеть для его создания.")
	} else if update.CacheError != "" {
		result.Warnings = append(result.Warnings, "Upstream update cache не удалось безопасно разобрать: "+update.CacheError)
	} else if !update.CacheFresh {
		result.Warnings = append(result.Warnings, "Upstream update cache старше 15 минут; RouterForge показывает его как устаревший и не обновляет по сети.")
	}
	if update.CachePresent && update.MainVersion != "" && !update.EligibilityKnown && compareAntiscanVersions(update.MainVersion, scriptVersion) > 0 {
		result.Warnings = append(result.Warnings, "В upstream cache есть новая main-версия, но совместимость прошивки нельзя подтвердить по текущему RCI cache.")
	}

	if !firmware.CachePresent {
		result.Warnings = append(result.Warnings, "Upstream RCI cache отсутствует; RouterForge не опрашивает прошивку повторно ради этой страницы.")
	} else if firmware.NetfilterKnown && !firmware.NetfilterPresent {
		result.State = "fail"
		result.Summary = "Upstream RCI cache не подтверждает обязательный компонент Netfilter."
	}
	if !hook.Present {
		result.State = "fail"
		result.Summary = "Netfilter hook Antiscan не найден."
	} else if !hook.Executable {
		result.State = "fail"
		result.Summary = "Netfilter hook Antiscan найден, но не исполняем."
	}

	return result
}

func readAntiscanPackageVersion() (string, string) {
	for _, path := range []string{
		"/opt/lib/opkg/info/antiscan.control",
		"/opt/var/opkg/info/antiscan.control",
	} {
		if version := parsePackageVersion(string(readFile(path))); version != "" {
			return version, path
		}
	}
	return "", ""
}

func readAntiscanFirmwareCompatibility() antiscanCompatibilityFirmware {
	data, err := os.ReadFile(antiscanRCICachePath)
	if err != nil || len(data) == 0 {
		return antiscanCompatibilityFirmware{}
	}
	result := antiscanCompatibilityFirmware{
		CachePresent:     true,
		NetfilterKnown:   true,
		NetfilterPresent: strings.Contains(string(data), "opkg-kmod-netfilter"),
	}
	var payload map[string]any
	if json.Unmarshal(data, &payload) == nil {
		result.Version = strings.TrimSpace(compatString(payload["release"]))
	}
	return result
}

func readAntiscanHookCompatibility() antiscanCompatibilityHook {
	result := antiscanCompatibilityHook{Path: antiscanNetfilterHookPath}
	info, err := os.Stat(antiscanNetfilterHookPath)
	if err != nil {
		return result
	}
	result.Present = info.Mode().IsRegular()
	result.Executable = result.Present && info.Mode().Perm()&0111 != 0
	return result
}

func readAntiscanCachedUpdate(installedVersion string, firmware antiscanCompatibilityFirmware) antiscanCompatibilityUpdate {
	result := antiscanCompatibilityUpdate{Source: "upstream-cache"}
	data, err := os.ReadFile(antiscanUpdateCachePath)
	if err != nil || len(data) == 0 {
		return result
	}
	result.CachePresent = true
	if info, statErr := os.Stat(antiscanUpdateCachePath); statErr == nil {
		age := time.Since(info.ModTime()).Seconds()
		if age < 0 {
			age = 0
		}
		result.CacheAgeSeconds = int64(age)
		result.CacheFresh = result.CacheAgeSeconds <= antiscanUpdateCacheFreshSeconds
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		result.CacheError = err.Error()
		return result
	}
	result.LegacyVersion, _, _ = extractAntiscanUpdateChannel(payload, "legacy")
	result.MainVersion, _, result.MainMinOS = extractAntiscanUpdateChannel(payload, "main")

	if installedVersion == "" {
		return result
	}
	if result.LegacyVersion != "" && compareAntiscanVersions(result.LegacyVersion, installedVersion) > 0 {
		result.EligibilityKnown = true
		result.Available = true
		result.AvailableVersion = result.LegacyVersion
		result.AvailableChannel = "legacy"
		return result
	}
	if result.MainVersion == "" || compareAntiscanVersions(result.MainVersion, installedVersion) <= 0 {
		result.EligibilityKnown = true
		return result
	}
	if result.MainMinOS == "" || firmware.Version == "" {
		return result
	}
	result.EligibilityKnown = true
	if compareAntiscanVersions(firmware.Version, result.MainMinOS) >= 0 {
		result.Available = true
		result.AvailableVersion = result.MainVersion
		result.AvailableChannel = "main"
	}
	return result
}

func extractAntiscanUpdateChannel(payload map[string]any, name string) (string, string, string) {
	channel, ok := payload[name].(map[string]any)
	if !ok {
		return "", "", ""
	}
	minOS := strings.TrimSpace(compatString(channel["min_os"]))
	for _, item := range compatObjects(channel["packages"]) {
		antiscan, ok := item["antiscan"].(map[string]any)
		if !ok {
			continue
		}
		version := strings.TrimSpace(compatString(antiscan["version"]))
		if version == "" {
			continue
		}
		return version, strings.TrimSpace(compatString(antiscan["update_info"])), minOS
	}
	return "", "", minOS
}

func compatObjects(value any) []map[string]any {
	out := []map[string]any{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if object, ok := item.(map[string]any); ok {
				out = append(out, object)
			}
		}
	case map[string]any:
		for _, item := range typed {
			if object, ok := item.(map[string]any); ok {
				out = append(out, object)
			}
		}
	}
	return out
}

func compatString(value any) string {
	text, _ := value.(string)
	return text
}

type antiscanVersionParts struct {
	epoch    int64
	upstream string
	revision string
}

func splitAntiscanVersion(value string) antiscanVersionParts {
	value = strings.TrimSpace(value)
	parts := antiscanVersionParts{}
	if idx := strings.IndexByte(value, ':'); idx > 0 {
		if epoch, err := strconv.ParseInt(value[:idx], 10, 64); err == nil {
			parts.epoch = epoch
			value = value[idx+1:]
		}
	}
	if idx := strings.LastIndexByte(value, '-'); idx >= 0 {
		parts.upstream = value[:idx]
		parts.revision = value[idx+1:]
	} else {
		parts.upstream = value
	}
	return parts
}

func antiscanVersionUpstreamPart(value string) string {
	return splitAntiscanVersion(value).upstream
}

func compareAntiscanVersions(left, right string) int {
	a := splitAntiscanVersion(left)
	b := splitAntiscanVersion(right)
	if a.epoch < b.epoch {
		return -1
	}
	if a.epoch > b.epoch {
		return 1
	}
	if result := compareAntiscanVersionPart(a.upstream, b.upstream); result != 0 {
		return result
	}
	return compareAntiscanVersionPart(a.revision, b.revision)
}

func compareAntiscanVersionPart(left, right string) int {
	for left != "" || right != "" {
		leftNonDigit, leftRest := takeAntiscanVersionRun(left, false)
		rightNonDigit, rightRest := takeAntiscanVersionRun(right, false)
		if result := compareAntiscanNonDigits(leftNonDigit, rightNonDigit); result != 0 {
			return result
		}
		var leftDigits, rightDigits string
		leftDigits, left = takeAntiscanVersionRun(leftRest, true)
		rightDigits, right = takeAntiscanVersionRun(rightRest, true)
		if result := compareAntiscanDigits(leftDigits, rightDigits); result != 0 {
			return result
		}
	}
	return 0
}

func takeAntiscanVersionRun(value string, digits bool) (string, string) {
	index := 0
	for index < len(value) {
		isDigit := value[index] >= '0' && value[index] <= '9'
		if isDigit != digits {
			break
		}
		index++
	}
	return value[:index], value[index:]
}

func compareAntiscanNonDigits(left, right string) int {
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}
	for i := 0; i < maxLen; i++ {
		leftOrder := antiscanVersionCharOrder(left, i)
		rightOrder := antiscanVersionCharOrder(right, i)
		if leftOrder < rightOrder {
			return -1
		}
		if leftOrder > rightOrder {
			return 1
		}
	}
	return 0
}

func antiscanVersionCharOrder(value string, index int) int {
	if index >= len(value) {
		return -3000
	}
	char := value[index]
	if char == '~' {
		return -4000
	}
	if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
		return int(char)
	}
	return int(char) + 10000
}

func compareAntiscanDigits(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
