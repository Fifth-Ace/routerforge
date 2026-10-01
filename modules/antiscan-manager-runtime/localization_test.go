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
		"Управление Antiscan",
		"РИСК ЛОЖНЫХ БЛОКИРОВОК",
		"ГОТОВНОСТЬ ANTISCAN",
		"RCI-токен",
		"Токен передаётся upstream только через stdin",
		"Расписание Antiscan",
		"Автоматическая задача retry_load_geo",
		"ШТАТНАЯ ОЧИСТКА / ВОССТАНОВЛЕНИЕ",
		"Совместимость и обновления",
		"RouterForge не устанавливает обновления Antiscan",
		"data-tab=\"geo\">Geo</button>",
		"Добавить адрес",
		"Применение настроек",
		"Учтённый функционал upstream",
		"ДИАГНОСТИКА БЛОКИРОВКИ",
		"БЕЗОПАСНАЯ НАСТРОЙКА",
		"ИСТОРИЯ ОПЕРАЦИЙ",
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
		"function renderDiagnostics(payload)",
		"function loadDiagnostics()",
		"function renderCompatibility(payload)",
		"function loadCompatibility()",
		"function renderRCITokenStatus(payload)",
		"function loadRCITokenStatus()",
		"function performRCITokenAction(action)",
		"function renderScheduler(payload, force = false)",
		"function loadScheduler()",
		"function applyScheduler()",
		"function renderFlushPreview(payload)",
		"function configureListSetMode(mode)",
		"function loadFlushPreview()",
		"function performFlush()",
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

func TestAntiscanFrontendRejectsRemovedOrDeveloperCopy(t *testing.T) {
	combined := readLocalizationFixture(t, "../antiscan-manager/frontend/index.html") + "\n" +
		readLocalizationFixture(t, "../antiscan-manager/frontend/app.js")

	forbidden := []string{
		"ЗАЩИЩЁННОЕ УПРАВЛЕНИЕ",
		"P26E · Локализация и UX",
		"ГРАНИЦЫ МОДУЛЯ",
		"GUARDED CONTROL",
		"FALSE POSITIVE RISK",
		"UPSTREAM LIFECYCLE",
		"Guarded lifecycle",
		"WHY BLOCKED?",
		"LIVE BLOCK STATE",
		"TRANSACTIONAL CONFIG",
		"data-tab=\"inspect\"",
		"data-tab=\"protection\"",
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
			t.Fatalf("removed/developer-facing copy leaked into Antiscan UI: %q", needle)
		}
	}
}
