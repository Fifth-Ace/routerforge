# P17A–P17C — Network Doctor + Route Inspector

P17A связывает уже существующие Network Doctor и Route Inspector в один маршрутный диагностический контур.

## Что было до P17A

Route Inspector уже умел:

- читать IPv4/IPv6 routes из `ip route table all`;
- читать IPv4/IPv6 policy rules;
- выполнять bounded `ip route get`;
- показывать kernel decision, table, gateway, interface, source address;
- fallback на `/proc/net/route` для IPv4, когда `ip` недоступен.

Network Doctor при этом определял маршрут к цели отдельно через `/proc/net/route` и longest-prefix match. Это означало, что Doctor мог не видеть:

- policy routing table;
- IPv6 kernel decision;
- `blackhole`, `unreachable`, `prohibit`, `throw`;
- реальный table/interface/source, выбранный `ip route get`.

## Что меняется

Этап `target_route` в Network Doctor теперь использует **тот же Route Inspector**, что и отдельный экран маршрутов.

Doctor получает и возвращает:

- `route_decision`;
- `policy_state`;
- legacy `route` для обратной совместимости, если доступен IPv4 procfs fallback.

Добавляется отдельный диагностический этап `policy_routing`.

### target_route

`ok` — kernel decision доступен и не является блокирующим route type.

`fail` — нет route decision либо kernel выбрал:

- `blackhole`;
- `unreachable`;
- `prohibit`;
- `throw`.

`skipped` — адрес цели не удалось получить.

### policy_routing

`ok` — policy state известен (`active` либо `default-only`).

`unavailable` — firmware/runtime не дал достаточных данных по policy rules.

Активный PBR сам по себе **не ошибка**. Doctor сообщает, что policy routing участвует в выборе пути, и показывает фактически выбранную таблицу.

## Почему так

Network Doctor теперь отвечает не на вопрос «какой маршрут похож на подходящий в main table», а на более полезный вопрос:

**«Какой путь реально выбрало ядро для этой цели и участвовала ли policy routing?»**

Это особенно важно для Keenetic с несколькими uplink/VPN/PBR таблицами.

## Безопасность

P17A остаётся полностью read-only:

- mutation API отсутствует;
- shell-команды ограничены существующим `safety.RunCommand`;
- `ip` работает с фиксированными аргументами;
- пользовательский target проходит существующую validation;
- timeout/output bounds сохраняются.

## P17B — path explainability

P17B делает результат Network Doctor объяснимым на уровне фактически выбранного ядром пути.

После `target_route` и `policy_routing` Doctor добавляет:

- `kernel_egress` — существует ли выбранный `ip route get` интерфейс и находится ли он в рабочем состоянии;
- `gateway_consistency` — какой gateway/interface выбрало ядро, совпадает ли main-table путь с IPv4 default path либо цель использует отдельный target-specific/PBR путь;
- `source_consistency` — принадлежит ли выбранный ядром `src` фактическому egress-интерфейсу.

Ответ `/v1/doctor` также содержит `path_explainability` с:

- `egress_interface`;
- `gateway`;
- `source`;
- `table`;
- `egress_exists`;
- `egress_up`;
- `source_checked`;
- `source_matches_egress`.

Различие target path и default path само по себе не считается ошибкой: более специфичный маршрут либо PBR могут законно выбрать другой gateway/interface.

Ошибка фиксируется только для фактической несогласованности:

- kernel-selected интерфейс отсутствует или down;
- gateway присутствует, но egress interface отсутствует;
- kernel-selected source не принадлежит выбранному интерфейсу.

Fault-domain verdict теперь отдаёт более точные коды:

- `egress_interface_failure` → `local`;
- `gateway_interface_mismatch` → `routing`;
- `route_source_mismatch` → `local`.

P17B остаётся read-only: mutation API не добавляется, маршруты/правила не изменяются.
## P17C — actionable diagnosis

P17C завершает диагностический контур Network Doctor: итоговый verdict теперь сопровождается безопасным списком следующих проверок.

`diagnosis.actions` содержит machine-readable элементы:

- `id` — стабильный идентификатор следующей проверки;
- `priority` — `high` или `medium`;
- `fault_domain` — область, к которой относится проверка;
- `detail` — краткое объяснение.

Примеры:

- routing failure → проверить target route и policy rules;
- kernel egress failure → проверить фактически выбранный интерфейс;
- source mismatch → проверить назначение source address и source-based PBR;
- DNS failure → проверить resolver/DNS policy;
- upstream failure → проверить gateway и upstream;
- service failure → проверить порт, сервис и удалённую фильтрацию.

Для `healthy` список действий пустой: Doctor не предлагает бессмысленные изменения, когда критическая проблема не обнаружена.

UI показывает блок «Что проверить дальше / Next checks» непосредственно под диагностической цепочкой.

Все действия остаются **рекомендациями**, а не mutation API. P17C ничего не меняет в маршрутах, интерфейсах, DNS, firewall или policy rules.

## P17 status

P17 Network Doctor + Route Inspector завершён:

- P17A — единый kernel-aware route decision;
- P17B — path explainability и consistency diagnostics;
- P17C — actionable diagnosis.

Следующий workstream: **P18 DNS Policy Router**.
