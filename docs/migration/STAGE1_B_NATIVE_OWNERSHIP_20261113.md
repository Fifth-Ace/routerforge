# RouterForge ALL11 — Stage 1, крупный функциональный пакет B

**Дата:** 13.11.2026. **Source parent:** `RouterForge_STAGE1_A.zip` (SHA256 `9788a7ffeaa41b31032f2b3973a3014e84f911579e22a7d97fba9817c84c04d9`).

## Что перенесено в этом исходном срезе

### Antiscan Manager — Svelte native read-only views + direct actions

- Удалён `new Proxy` и совместимый `node(id)` — прямые изменения реактивного `$state(cells)` теперь не имитируют HTMLElement.
- Убраны **все** `{@html}` и `.innerHTML` из исходного интерфейса Antiscan. Неиспользуемые HTML-ветки исходной разметки заменены на обычный Svelte текст / встроенную разметку.
- Вынесены **8** Svelte-компонентов: `AntiscanKeyValueRows`, `AntiscanWarningItems`, `AntiscanDiagnosticChecks`, `AntiscanHistoryRows`, `AntiscanRiskSummary`, `AntiscanIPSetRows`, `AntiscanSetEntries`, `AntiscanInspectorResult`.
- Под Svelte отданы десять read-only областей: runtime, состояние защиты, оценка /24, предупреждения, перечень ipset, диагностика, контракт upstream, совместимость, история, инспектор IP.
- Под Svelte отданы активные ipset/Geo списки, исходные пользовательские списки, ошибки расписания, предупреждения flush, варианты runtime-set селектора.
- Динамические кнопки IP inspect/exclude/unban/custom delete переведены с HTML-строк и документного делегированного слушателя на `onclick` компонентов. Сохранены исходные `performUnban`, `performListEntry`, `performCustomListMutation` и серверные confirmation/CAS/rollback контракты.
- Задержанный iframe height `requestAnimationFrame` теперь отменяется при unmount; resize/observer cleanup оставлен.

**Ограничение:** крупный `INITIAL`/`cells` — всё ещё единое DOM-shaped представление state в корневом файле, хотя больше не является поддельным DOM и не использует raw HTML. Осталось разбить настройки/config form, расписание, команды/жизненный цикл по владельцам. Формальная полнота пакета B **PENDING**, пока это не сверено по коду.

### NFQWS Manager — root lifecycle

- Добавлен `NfqwsHostBridge.js` для iframe theme/height (read-only), монтирование и удаление в `NFQWSManagerApp.svelte`.
- Root Svelte владеет глобальными `keydown` handlers, очищает их через `AbortController` при unmount. Legacy runtime предоставляет только командные обработчики `handleShortcut`/`handleEscape`.
- Высота iframe коалесцируется через `requestAnimationFrame` в адаптере, наблюдатель отключается при unmount; лишние вложенные rAF в таб-навигации удалены.
- Убрана устаревшая пометка «В разработке» у рабочего Svelte Geo UI.
- Защищённые runtime API, Smart Apply, tokens, rollback, no autostart не переписаны.

## Текущая последовательность

1. **Этап 1 / A:** NFQWS Geo + Lite polling — исходники внесены в предыдущем checkpoint.
2. **Этап 1 / B:** крупный перенос Antiscan + NFQWS lifecycle — исходники внесены в данном checkpoint; осталась финальная архитектурная развязка Antiscan form/controllers.
3. **Этап 1 / C:** сверка 11 feature→Svelte owner→API, предотвращение дублей в остальных модулях, завершение UI transfer.
4. **Этап 1 / D:** source closeout и **только после него** Stage 2 (Vite/Chromium/Go/security/IPK) и Stage 3 (регрессионные исправления).

## Что не выполнялось

- **Новых Node/Vite/Chromium/Go тестов не запускалось по запрету до завершения этапа 1**. Даже compile / browser данного checkpoint не подтверждены; `MODULE_DONE=PENDING`.
- Git commit/push, CI dispatch, main/tag/release и реальные операции на роутере не выполнялись.
- Сборочные зависимости не устанавливались; runtime modules и protected API не вызывались.

## Принцип завершения

Исходный JS сервис с API не считается legacy UI только за наличие `.js`. UI завершён, когда state, handler и lifecycle принадлежат одному Svelte owner и отсутствует работающий DOM-renderer. Stage 1 COMPLETE возможен только после source-инвентаризации **всех 11**, включая Antiscan forms, а не по одному успешному тесту.
