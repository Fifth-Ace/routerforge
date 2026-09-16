# P18 — DNS Policy Router

## P18A — policy inventory contract

P18A создаёт read-only основу для DNS Policy Router поверх уже существующей интеграции RouterForge DNS с Keenetic RCI.

До P18A runtime уже умел читать `/show/ip/policy` и сопоставлять технические proxy-идентификаторы `PolicyN` с человекочитаемыми именами. Это использовалось как внутренний helper, но не было стабильного API-контракта для будущего policy routing.

### Новый контракт

`GET /v1/policies`

Ответ:

- `policies` — стабильный список policy identities;
- `source` — `keenetic-rci`;
- `mutation_api` — `false`.

Элемент `policies`:

- `proxy` — стабильный технический идентификатор (`System`, `Policy1`, `Policy2`, ...);
- `display_name` — человекочитаемое имя из Keenetic;
- `system` — системная политика;
- `ordinal` — числовой индекс для `PolicyN`.

### Порядок

Список детерминирован:

1. `System`;
2. `Policy1`;
3. `Policy2`;
4. ...
5. неизвестные/нестандартные идентификаторы — после известных policy identities.

Это важно для UI, diff/evidence и будущего policy evaluator: порядок не зависит от map iteration.

### Совместимость

Существующий `readDNSPolicyNames()` сохранён и теперь строится поверх того же inventory contract, поэтому старые consumers продолжают получать `map[proxy]display_name`.

### Безопасность

P18A полностью read-only:

- policy assignment не меняется;
- resolver configuration не меняется;
- DNS traffic не перенаправляется;
- mutation API не добавляется;
- источник — существующий bounded RCI client.

## P18B — policy selection/evaluation dry-run

P18B добавляет pure evaluator без хранения правил и без применения конфигурации.

`POST /v1/policies/evaluate`

Контекст запроса:

- `client_ip` — optional IPv4/IPv6 адрес клиента;
- `domain` — обязательное DNS-имя;
- `qtype` — optional тип запроса;
- `rules` — временный candidate rule set только для этого dry-run.

Rule:

- `id` — стабильный идентификатор;
- `priority` — целое `0..1000000`, меньшее значение выигрывает при одинаковой specificity;
- `policy` — существующий `System` либо `PolicyN`;
- `match.client_cidr`;
- `match.domain_suffix`;
- `match.qtype`.

### Детерминированный выбор

Совпавшие правила ранжируются:

1. specificity bit score: client CIDR = 4, domain suffix = 2, qtype = 1;
2. меньшее `priority`;
3. лексикографически меньший `id`.

Так client+domain rule предсказуемо сильнее только domain rule независимо от порядка JSON-массива.

Если ни одно правило не совпало, evaluator возвращает `System` с `fallback_to_system=true`.

Ответ также содержит `reasons`, `rule_id`, `rule_priority`, `specificity` и число просмотренных правил.

### Safety boundary

P18B остаётся read-only:

- rules не записываются на диск;
- Keenetic policy не меняется;
- resolver configuration не меняется;
- endpoint не требует mutation header, потому что выполняет только dry-run;
- policy identities сверяются с текущим `/show/ip/policy`;
- body ограничен 128 KiB, rules — максимум 128;
- unknown JSON fields отклоняются.

## P18C — persisted policy model + validation boundary

P18C добавляет сохранение policy rules, но намеренно **не активирует** их в DNS runtime.

### API

`GET /v1/policy-rules`

Возвращает versioned document:

- `version`;
- `updated_at`;
- `rules`.

Ответ явно содержит:

- `persisted=true`;
- `activated=false`.

`PUT /v1/policy-rules`

Требует стандартный RouterForge DNS mutation header и `application/json`.

Перед записью:

1. каждое правило проходит нормализацию P18B;
2. duplicate `id` отклоняются;
3. policy должна существовать в live Keenetic RCI inventory;
4. максимум 128 rules;
5. правила канонически сортируются по `priority`, затем `id`.

### Storage contract

Файл:

`/opt/etc/routerforge/dns-policy-rules.json`

Схема versioned (`version=1`).

Запись выполняется через temporary file в том же каталоге:

- mode `0600`;
- write;
- `fsync`;
- close;
- atomic rename.

Отсутствующий файл трактуется как валидный пустой rule set.

### Safety boundary

P18C — **persist-only**:

- правила сохраняются;
- правила можно валидировать и dry-run'ить;
- resolver configuration не меняется;
- policy bindings в Keenetic не меняются;
- live DNS traffic не использует сохранённые rules;
- `activated=false` является частью API-контракта.

Это отделяет безопасную конфигурационную mutation boundary от будущей runtime activation.

## Следующий этап

P18D — persisted-rule evaluation + activation preview: dry-run должен уметь брать сохранённый rule set и показывать diff/evidence будущей runtime activation, всё ещё без изменения живого DNS-трафика.