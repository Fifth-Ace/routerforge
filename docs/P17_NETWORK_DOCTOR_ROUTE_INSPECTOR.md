# P17A — Network Doctor + Route Inspector integration

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

## Следующий этап

P17B — explainability для default path и интерфейса: kernel-selected egress, gateway/interface consistency, route/source mismatch diagnostics и более точный fault-domain verdict.
