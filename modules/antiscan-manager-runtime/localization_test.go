package main

import (
	"os"
	"strings"
	"testing"
)

func readLocalizationFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestAntiscanFrontendRussianLocalizationContracts(t *testing.T) {
	html := readLocalizationFixture(t, "../antiscan-manager/frontend/index.html")
	js := readLocalizationFixture(t, "../antiscan-manager/frontend/app.js")

	requiredHTML := []string{
		"ЗАЩИЩЁННОЕ УПРАВЛЕНИЕ",
		"РИСК ЛОЖНЫХ БЛОКИРОВОК",
		"ДИАГНОСТИКА БЛОКИРОВКИ",
		"БЕЗОПАСНАЯ НАСТРОЙКА",
		"ИСТОРИЯ ОПЕРАЦИЙ",
		"P26E · Локализация и UX",
	}
	for _, needle := range requiredHTML {
		if !strings.Contains(html, needle) {
			t.Fatalf("localized HTML contract missing %q", needle)
		}
	}

	requiredJS := []string{
		"function localizeMessage(value)",
		"function localizeMessages(values)",
		"function reasonLabel(value)",
		"function evidenceSummary(item)",
		"function mutationSuccessMessage(context, payload)",
		"Предыдущее состояние восстановлено",
		"Защита отключена до следующего запуска",
		"нужен перезапуск",
	}
	for _, needle := range requiredJS {
		if !strings.Contains(js, needle) {
			t.Fatalf("localized UI contract missing %q", needle)
		}
	}
}

func TestAntiscanFrontendRejectsDeveloperEnglishCopy(t *testing.T) {
	combined := readLocalizationFixture(t, "../antiscan-manager/frontend/index.html") + "\n" +
		readLocalizationFixture(t, "../antiscan-manager/frontend/app.js")

	forbidden := []string{
		"GUARDED CONTROL",
		"FALSE POSITIVE RISK",
		"UPSTREAM LIFECYCLE",
		"Guarded lifecycle",
		"WHY BLOCKED?",
		"LIVE BLOCK STATE",
		"TRANSACTIONAL CONFIG",
		"Bounded persistent audit",
		"AUDIT BOUNDARY",
		"Guarded action failed.",
		"Single-entry unban",
		"Custom list update",
		"Transactional config",
		"runtime member",
		"UPSTREAM BUSY",
		"APPLYING…",
		"STALE",
		"SYNCED",
		"threshold/retention",
		"risk-profile",
	}
	for _, needle := range forbidden {
		if strings.Contains(combined, needle) {
			t.Fatalf("developer-facing English copy leaked into Antiscan UI: %q", needle)
		}
	}
}
