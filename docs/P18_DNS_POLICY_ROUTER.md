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

## P18D — persisted-rule evaluation + activation preview

P18D связывает persisted store с evaluator, но по-прежнему не активирует rules в live DNS runtime.

### Persisted evaluation

`POST /v1/policy-rules/evaluate`

Тело содержит только request context:

- `client_ip`;
- `domain`;
- `qtype`.

Rules из запроса не принимаются. Runtime:

1. загружает versioned persisted document;
2. читает live Keenetic policy inventory;
3. повторно валидирует сохранённые rules;
4. запускает тот же deterministic evaluator из P18B.

Ответ содержит:

- `source=persisted`;
- `document_version`;
- `updated_at`;
- `persisted=true`;
- `activated=false`;
- explainable evaluation result.

Если сохранённая policy больше не существует в live inventory, endpoint возвращает conflict и не пытается оценивать устаревший rule set.

### Activation preview

`GET /v1/policy-rules/activation-preview`

Preview показывает только уже доказуемую границу будущей активации:

- persisted rule count;
- current active policy-router rule count = `0`;
- policy identities, используемые сохранёнными rules;
- readiness после live-policy validation;
- `activated=false`;
- структурированный `changes` и `evidence`.

Current state `0 active policy-router rules` следует из текущего P18C/P18D контракта: сохранённые policy rules ещё не подключены к live DNS traffic path.

Preview **не симулирует неизвестный будущий dataplane** и не делает RCI mutation.

### Safety boundary

P18D остаётся preview-only:

- сохранённый rule set можно dry-run'ить;
- readiness зависит от текущего Keenetic policy inventory;
- live DNS traffic не меняется;
- resolver configuration не меняется;
- Keenetic policy bindings не меняются;
- activation endpoint отсутствует;
- activation preview не требует mutation header.

## P18E — activation transaction design

P18E определяет будущий activation transaction поверх общего RouterForge transaction contract, но **не добавляет runtime activation**.

`GET /v1/policy-rules/activation-transaction-design`

Endpoint:

1. загружает persisted document;
2. читает live Keenetic policy inventory;
3. валидирует сохранённые rules;
4. возвращает immutable design contract.

### Shared transaction states

Используются существующие RouterForge states без отдельной DNS-специфичной state machine:

`precheck → snapshot → validated → applied → verified → committed`

Terminal recovery/error states:

- `rolled-back`;
- `ambiguous`;
- `failed`.

### Required evidence by stage

`precheck`:

- persisted document loaded;
- schema version accepted;
- all rules valid against current Keenetic policy inventory.

`snapshot`:

- exact pre-activation runtime policy-router snapshot;
- desired persisted document identity;
- rollback artifact readable before apply.

`validated`:

- canonical desired rules frozen;
- policy inventory rechecked;
- activation input unchanged since precheck.

`applied`:

- future atomic apply reports success;
- runtime accepts staged policy-router configuration.

`verified`:

- active runtime identity equals staged desired identity;
- runtime health passes;
- policy-router evaluation probes match staged rules.

`committed`:

- verification evidence complete;
- no unresolved rollback/ambiguous state.

### Rollback contract

If failure happens after apply, future implementation must restore the **exact pre-activation runtime snapshot** and verify recovery.

If rollback cannot be proven, transaction must end as `ambiguous`, never as successful.

Required artifacts:

- exact pre-activation runtime policy-router snapshot;
- desired persisted policy document identity;
- transaction evidence manifest.

### Commit gate

A future activation may enter `committed` only when:

- active runtime identity equals staged desired identity;
- runtime health is good;
- policy-router evaluation probes pass;
- no unresolved rollback or ambiguous evidence exists.

### Safety boundary

P18E remains design-only:

- `activation_available=false`;
- `activated=false`;
- `mutation_api=false` for the design endpoint;
- no apply endpoint;
- no enable endpoint;
- no live DNS traffic mutation;
- no RCI mutation.

## P18F — activation engine foundation behind tests/fakes

P18F реализует внутренний activation transaction executor, но не подключает его к HTTP/API и не создаёт production runtime driver.

Новый internal engine принимает только абстрактный `dnsPolicyActivationDriver`:

- `Precheck`;
- `Snapshot`;
- `Validate`;
- `Apply`;
- `Verify`;
- `Rollback`;
- `VerifyRollback`.

Production driver отсутствует. В P18F engine вызывается только тестами с fake driver.

### State-machine execution

Успешный путь:

`precheck → snapshot → validated → applied → verified → committed`

Engine использует существующий `internal/platform/transaction`.

Критически важно: engine входит в state `applied` **до** вызова `Apply`.

Причина: apply может частично изменить runtime и вернуть error. Поэтому любой error после входа в applied обязан пройти через rollback path, а не может завершиться обычным `failed` с неизвестным состоянием runtime.

### Failure semantics

До apply:

- precheck/snapshot/validation failure → `failed`;
- runtime mutation ещё не разрешена.

После входа в applied:

- apply failure → rollback + rollback verification;
- verify failure → rollback + rollback verification;
- успешный rollback + verification → `rolled-back`;
- rollback failure → `ambiguous`;
- rollback verification failure → `ambiguous`.

`committed` достигается только после успешного `Verify`.

### Evidence

Engine записывает shared transaction evidence:

- precheck result;
- snapshot identity/rule count;
- validated rule count;
- apply result;
- verify result;
- rollback/recovery evidence;
- rollback verification evidence;
- committed terminal evidence.

Ошибки возвращаются через `transaction.Wrap`, поэтому manifest доступен через `transaction.Extract`.

### Build ABI

`dns_policy_activation.go` включён в explicit DNS Module ABI source list в `scripts/build-module-opkg.sh`.

Таким образом production-like Dev package build проверяет compile совместимость engine даже при отсутствии production driver/API wiring.

### Safety boundary

P18F всё ещё не активирует routing:

- public activation endpoint отсутствует;
- production activation driver отсутствует;
- server не вызывает executor;
- RCI mutation отсутствует;
- live DNS traffic не меняется;
- persisted rules остаются inactive.

## P18G — runtime adapter discovery + contract tests

P18G не создаёт production driver. Этап фиксирует, какие primitives уже существуют в RouterForge и какие знания о Keenetic policy dataplane всё ещё отсутствуют.

### Уже доступные primitives

Подтверждены текущим DNS mutation stack:

- bounded context-aware RCI GET через `dnsRCIClient.getJSON`;
- structured RCI POST/DELETE без shell interpolation;
- native configuration save через `/system/configuration/save`;
- exact canonical RCI readback verification;
- restore exact captured native state + verification;
- DNS runtime health probe before transaction commit.

Эти primitives уже используются resolver mutation path и являются подходящими строительными блоками для будущего policy activation driver.

### Blocking unknowns

Production policy driver остаётся **заблокирован**, пока аппаратно не определены:

1. exact Keenetic RCI read path и response schema для реально активного policy-router rule state;
2. exact RCI mutation path и payload schema для установки policy-router rules;
3. stable runtime identity/readback contract, которым можно доказать, что active state идентичен desired rules.

`discoverDNSPolicyRuntimeAdapter()` поэтому возвращает:

`production_driver_ready=false`.

Ни один unknown не заменяется предположением о CLI/RCI schema.

### Adapter contract

Добавлен internal `dnsPolicyActivationAdapter`, который связывает P18F engine с абстрактным `dnsPolicyRuntimePrimitive`.

Primitive contract:

- `SnapshotRuntime`;
- `ApplyCanonicalRules`;
- `VerifyCanonicalRules`;
- `RestoreRuntime`;
- `VerifyRuntimeSnapshot`;
- `RuntimeHealth`.

Rule validation передаётся отдельно и выполняется на precheck/validated boundary.

### Contract tests

Fake primitive доказывает:

- P18F engine корректно проходит через adapter до `committed`;
- validation выполняется до mutation;
- runtime health проверяется до apply и после desired readback;
- verify failure вызывает restore exact snapshot;
- rollback verification проверяет snapshot identity и runtime health;
- health failure до apply не вызывает mutation.

### Build ABI

`dns_policy_adapter.go` включён в explicit DNS Module ABI source list.

### Safety boundary

P18G остаётся non-production:

- production runtime primitive отсутствует;
- exact policy RCI write path нигде не задан;
- server/API не создаёт adapter;
- public activation endpoint отсутствует;
- live DNS traffic не меняется;
- RCI mutation policy-router rules отсутствует.

## P18H — hardware discovery protocol

P18H добавляет воспроизводимый hardware probe:

`scripts/keenetic-policy-rci-discovery.sh`

Probe предназначен для реального Keenetic/Entware и **не выполняет mutation**.

### Phase A — read-only snapshot

Один запуск собирает:

- firmware/system metadata через `ndmc`;
- CLI `show ip policy`;
- raw HTTP GET `RCI /show/ip/policy` вместе с headers/status/body;
- narrow config GET `RCI /ip/policy`;
- narrow host binding GET `RCI /ip/hotspot/host`;
- только отфильтрованные policy/proxy/route строки `show running-config`;
- SHA256 для evidence-файлов;
- tar.gz evidence bundle.

Полный `running-config` намеренно не сохраняется, чтобы не утащить секреты/ключи/пароли в диагностический bundle.

После hardware baseline было подтверждено, что GET RCI root может раскрывать несвязанные чувствительные настройки. Поэтому root GET удалён из probe полностью. Используются только подтверждённые narrow reads `/ip/policy` и `/ip/hotspot/host`.

Probe явно гарантирует:

- RCI POST не выполняется;
- RCI DELETE не выполняется;
- `/system/configuration/save` не вызывается;
- mutating `ndmc` command отсутствует.

### Phase B — controlled delta capture

Для определения runtime identity и будущего write contract probe запускается дважды:

1. `baseline` — до изменения;
2. `after` — после **одного** заведомо обратимого изменения существующей Keenetic policy через штатный UI/CLI.

Сам probe изменение не выполняет.

Сравниваются:

- `rci-show-ip-policy.body`;
- `ndmc-show-ip-policy.stdout`;
- `ndmc-running-config-policy-filtered.stdout`;
- HTTP metadata;
- SHA256 manifests.

Из diff можно принимать только фактически наблюдаемые поля/identity. Нельзя выводить mutation path из названия read endpoint по аналогии.

### Write-schema gate

P18H **не разблокирует production driver автоматически**.

Для `production_driver_ready=true` всё ещё нужны доказательства:

1. exact active-state read path/schema;
2. exact mutation path/payload;
3. controlled apply evidence;
4. exact readback equality after apply;
5. exact rollback evidence;
6. stable identity across repeated reads.

До появления этих данных:

- production driver отсутствует;
- public activation API отсутствует;
- policy mutation в RouterForge отсутствует.

### Hardware output

Probe печатает секции:

`PRECHECK / GATES / ACTION / VERIFY / RESULT`

И завершает:

- `STATE: PASS`, если известный RCI read contract доступен и evidence сохранён;
- `STATE: PARTIAL`, если часть read-only источников недоступна.

`PARTIAL` не является разрешением переходить к write implementation.

## P18I — Keenetic-backed policy state primitive

Hardware discovery подтвердил на реальном Keenetic:

- narrow GET `/ip/policy`;
- narrow GET `/ip/hotspot/host`;
- runtime GET `/show/ip/policy`;
- structured POST transport через `/rci/`;
- payload shape для безопасного controlled field: `ip -> policy -> Policy0 -> description`;
- exact readback после apply;
- structured RCI rollback тем же transport;
- exact restoration `/ip/policy`, `/ip/hotspot/host` и `/show/ip/policy` по SHA256;
- experiment не требовал `/system/configuration/save`.

### Production state primitive

P18I добавляет `dnsPolicyKeeneticPrimitive`.

Production-backed операции:

- `SnapshotRuntime()` читает только три доказанных narrow/runtime endpoint;
- состояние канонизируется;
- host array сортируется по MAC, чтобы порядок перечисления не менял identity;
- identity = SHA256 от canonical `/ip/policy` + `/ip/hotspot/host` + `/show/ip/policy`;
- `VerifyRuntimeSnapshot()` повторно читает state и требует exact identity equality;
- `RuntimeHealth()` использует существующий health callback.

### Fail-closed mutation boundary

P18I **не материализует `DNSPolicyRule` в Keenetic dataplane**.

Следующие методы production primitive намеренно возвращают `errDNSPolicyDataplaneMappingUnknown`:

- `ApplyCanonicalRules`;
- `VerifyCanonicalRules`;
- `RestoreRuntime`.

Причина: hardware evidence доказал structured RCI transport и rollback на `description`, но ещё не доказал exact mapping наших match dimensions (`client_cidr`, `domain_suffix`, `qtype`) в native Keenetic objects.

Поэтому `dnsPolicyKeeneticPrimitive` уже реализует interface compile-time, но activation через него fail-closed и не может случайно изменить router state.

### Discovery contract

Старые unknowns "read path/write transport/identity" заменены более точными блокерами:

1. mapping `DNSPolicyRule` -> Keenetic dataplane objects;
2. structured payload schema именно для этих objects;
3. partial-apply rollback ordering/artifacts для финального mapping.

`production_driver_ready=false` сохраняется.

### Tests

Contract tests доказывают:

- primitive ходит только на `/ip/policy`, `/ip/hotspot/host`, `/show/ip/policy`;
- RCI root GET не используется;
- host enumeration order не меняет snapshot identity;
- runtime change ломает exact snapshot verification;
- mutation methods fail closed с dedicated error.

### Build ABI

`dns_policy_runtime_keenetic.go` добавлен в explicit DNS Module ABI source list и проходит production-like Linux ARM64 build до commit.

### Safety boundary

- public activation API отсутствует;
- production rule apply отсутствует;
- live DNS traffic не меняется;
- RouterForge не вызывает structured RCI policy mutation;
- hardware-proven read/identity primitive можно развивать дальше без догадок.

## P18J — hybrid dataplane mapping + marked egress contract

Hardware discovery показал, что direct native mapping `client_cidr + domain_suffix + qtype -> PolicyN` не требуется для первого dataplane design.

Keenetic уже предоставляет policy-routing primitive:

- `Policy0`: mark `0xffffaaa`, table4 `4096`;
- `Policy1`: mark `0xffffaab`, table4 `4098`;
- Linux `ip rule` связывает эти marks с exact routing tables;
- explicit route lookup with `mark 0xffffaab` selects table `4098`;
- Policy0 has no default route, and explicit marked lookup fails instead of silently falling through;
- therefore mark selection is a fail-closed egress primitive.

### Hybrid model

Policy matching остаётся внутри RouterForge DNS:

`client_cidr / domain_suffix / qtype -> PolicyN`

После evaluator выбранная `PolicyN` разрешается в текущий Keenetic route identity:

`PolicyN -> mark + table`

Исходящий DNS socket получает `SO_MARK`, а native Keenetic Linux policy routing выбирает уже существующую table.

Таким образом RouterForge не создаёт domain/qtype rules внутри Keenetic и не дублирует его policy configuration.

### Internal contract

P18J добавляет:

- `DNSPolicyEgressTarget`;
- `resolveDNSPolicyEgressTarget`;
- Linux-only `newDNSPolicyMarkedDialer`.

`System` использует обычный unmarked dialer.

`PolicyN` требует:

- существующую policy;
- ненулевой mark;
- положительный table id.

Отсутствие default route не маскируется: target сохраняет `HasDefault=false`, а kernel marked route остаётся fail-closed.

### Linux SO_MARK

Linux dialer ставит:

`SOL_SOCKET / SO_MARK = policy.Mark`

через `net.Dialer.Control` до connect/send path.

Ошибки `SO_MARK` возвращаются вызывающему коду и не допускают unmarked fallback.

### Tests

Unit/contract tests проверяют:

- `System` остаётся unmarked;
- `Policy1` maps to exact `0xffffaab / 4098`;
- policy without default route remains representable as fail-closed;
- unknown/zero-mark/zero-table policies rejected;
- Linux dialer sets exact mark;
- setsockopt error is propagated;
- System dialer has no mark Control.

### Safety boundary

P18J ещё не подключает marked dialer к live DNS resolver path:

- public activation API отсутствует;
- persisted rules не активируются;
- live DNS traffic не меняется;
- `ApplyCanonicalRules` остаётся fail-closed;
- production driver ready остаётся false.

Hardware mark mapping доказан, но перед live wiring нужен отдельный marked-socket smoke на реальном Keenetic.

## P18K — marked socket hardware smoke harness

P18K добавляет explicit CLI-only диагностический режим в production DNS binary:

`routerforge-dns --policy-egress-smoke PolicyN`

Дополнительные параметры:

- `--policy-egress-smoke-address` — TCP destination, default `1.1.1.1:443`;
- `--policy-egress-smoke-timeout` — bounded timeout, default `4s`.

Диагностический режим запускается до DNS server/capture loops и сразу завершается.

### Exact primitive reuse

Smoke использует production P18J contract без отдельной реализации:

1. `discoverPolicyRoutes()`;
2. `resolveDNSPolicyEgressTarget()`;
3. `newDNSPolicyMarkedDialer()`;
4. тот же `dnsPolicySetSocketMark`.

Для hardware evidence setter временно оборачивается:

- production setter устанавливает `SO_MARK`;
- `getsockopt(SO_MARK)` читает mark обратно на том же fd;
- mismatch считается ошибкой;
- затем выполняется реальный bounded TCP connect.

Таким образом evidence связывает:

`PolicyN -> requested mark -> actual socket mark -> kernel route/connect result`.

### Safety boundary

Smoke mode:

- не стартует DNS server;
- не запускает packet capture;
- не читает persisted policy rules;
- не изменяет Keenetic configuration;
- не вызывает RCI POST/DELETE;
- не вызывает configuration save;
- не меняет live DNS traffic;
- выполняет только один диагностический outbound TCP socket.

P18K считается hardware-complete только после проверки на реальном Keenetic:

- Policy1: requested/actual `0xffffaab`, TCP connect PASS;
- Policy0: requested/actual `0xffffaaa`, TCP connect ожидаемо FAIL из-за отсутствия default route;
- никакого unmarked fallback.

## P18L — live wiring design boundary + egress primitive unification

P18K hardware smoke подтвердил production egress primitive на реальном Keenetic:

- Policy1 requested mark `0xffffaab`;
- actual socket mark `0xffffaab`;
- table `4098`;
- bounded TCP connect успешно прошёл;
- Policy0 requested/actual mark `0xffffaaa`;
- table `4096` без default route;
- connect завершился fail-closed;
- unmarked fallback не наблюдался;
- установка пакета, restart сервиса и config mutation не выполнялись.

### Найденная runtime boundary

Текущий RouterForge DNS не является authoritative/live forwarding DNS proxy.

Текущие client/proxy paths:

- `capture_linux.go` пассивно наблюдает loopback proxy traffic через `AF_PACKET`;
- `client_capture_linux.go` пассивно наблюдает client/plain DNS traffic;
- runtime сохраняет telemetry, client attribution и health state;
- production listener на UDP/TCP :53 отсутствует;
- live client DNS request не проходит через RouterForge evaluator перед отправкой upstream.

Следовательно, подключить `newDNSPolicyMarkedDialer()` к существующему live request path невозможно: такого forwarding path пока нет.

### Single egress primitive

До P18L diagnostics имел отдельную реализацию `SO_MARK`.

P18L устраняет это расхождение:

- diagnostic `policy-mark` route теперь использует `newDNSPolicyMarkedDialer`;
- `dnsPolicySetSocketMark` становится единственной реализацией policy socket mark;
- interface-only diagnostics продолжает использовать `SO_BINDTODEVICE`;
- default diagnostics остаётся unmarked.

Это гарантирует, что hardware smoke, diagnostics и будущий live proxy используют один и тот же mark primitive.

### Future live proxy contract

Будущий forwarding path должен появиться отдельно и сначала только в shadow mode.

Минимальный contract:

1. отдельный explicit listener, не системный `:53`;
2. UDP и TCP parity;
3. client IP должен быть известен до evaluation;
4. raw query парсится тем же DNS parser;
5. persisted rules загружаются и валидируются;
6. evaluator выбирает `System` либо `PolicyN`;
7. `PolicyN` повторно разрешается в live `mark/table`;
8. upstream socket создаётся через единственный policy egress primitive;
9. отсутствие policy/default route и SO_MARK error — fail closed, без silent System fallback;
10. response возвращается исходному diagnostic/shadow client;
11. telemetry фиксирует selected rule, policy, mark/table и outcome.

### Shadow-first rollout

Первый live forwarding этап не должен:

- bind системный UDP/TCP :53;
- менять Keenetic nameserver config;
- менять DHCP DNS;
- ставить redirect/NAT rules;
- выключать native DNS proxy;
- активировать persisted rules для обычных клиентов.

Hardware gate сначала должен доказать на отдельном loopback/temporary port:

`synthetic client query -> evaluator -> PolicyN -> SO_MARK -> upstream -> response`.

Только после этого можно проектировать takeover/redirect transaction.

### Activation semantics

Существующий P18F transaction нельзя считать готовым к live proxy takeover без дополнительных artifacts.

Будущий activation snapshot должен включать минимум:

- proxy listener state;
- selected listen addresses/ports;
- persisted rule document identity;
- current Keenetic policy state identity;
- upstream inventory identity;
- current native DNS/redirect state, если takeover когда-либо будет разрешён.

Rollback должен сначала вернуть native DNS traffic path, затем остановить RouterForge live listener. Если recovery нельзя доказать, состояние `ambiguous`.

### Safety boundary

P18L:

- не добавляет DNS listener;
- не bind'ит :53;
- не меняет Keenetic config;
- не активирует persisted policy rules;
- не добавляет public activation API;
- не меняет live DNS traffic.

`production_driver_ready=false` сохраняется.

## P18M — shadow DNS forwarder foundation

P18M добавляет internal shadow forwarder, но не подключает его к `main()` и не запускает listener в обычном DNS runtime.

### Listener safety contract

Shadow config принимает только:

- explicit loopback IP (`127.0.0.0/8`);
- explicit non-zero port;
- port `53` запрещён;
- explicit upstream в форме IP:port;
- validated policy inventory;
- существующий `DNSPolicyRule` document model.

Wildcard/LAN bind и системный DNS port отвергаются до `Listen`.

### Existing policy engine reuse

Каждый synthetic query проходит существующий production contract:

1. raw DNS packet разбирается `parseDNSMessage`;
2. `client_ip`, normalized domain и qtype передаются в `evaluateDNSPolicy`;
3. evaluator выбирает `System` или `PolicyN`;
4. `PolicyN` разрешается через `resolveDNSPolicyEgressTarget` в current mark/table;
5. Linux transport использует `newDNSPolicyMarkedDialer`;
6. response проверяется как DNS response с тем же transaction ID.

Отдельного shadow evaluator или второго SO_MARK implementation нет.

### UDP/TCP parity

Foundation содержит оба listener path:

- UDP: datagram query -> marked UDP upstream -> datagram response;
- TCP: RFC-style 2-byte DNS frame -> marked TCP upstream -> framed response.

Оба транспорта используют один `planDNSPolicyShadowQuery` и один egress contract.

### Dynamic route identity

`dnsPolicyShadowServer` получает route provider function. Mark/table читаются заново для каждого запроса, поэтому будущий shadow smoke не закрепляет stale route identity на момент старта listener.

### Fail-closed rules

- invalid query -> no upstream dial;
- unknown policy -> no upstream dial;
- missing/zero mark or invalid table -> no upstream dial;
- SO_MARK error -> no unmarked fallback;
- PolicyN without usable kernel route naturally fails on marked socket;
- malformed/mismatched upstream DNS response не возвращается client.

### Runtime boundary

P18M намеренно **не** меняет `dns_module_main.go`.

Следовательно после сборки/publish:

- shadow listener не стартует автоматически;
- :53 не bind'ится;
- Keenetic config не меняется;
- persisted rules не становятся live;
- обычный RouterForge DNS остаётся passive observer.

`production_driver_ready=false` сохраняется.

## P18N — explicit CLI shadow-forwarder smoke harness

P18N добавляет CLI-only режим:

`routerforge-dns --policy-shadow-smoke`

Default параметры:

- listener: `127.0.0.1:55353`;
- upstream: `1.1.1.1:53`;
- per-query timeout: `3s`.

Listener остаётся под P18M validation: только explicit loopback и port != 53.

### Synthetic rules only

Smoke не читает и не изменяет persisted policy document.

Внутри процесса создаются два synthetic rules:

- dedicated domain -> `Policy1`;
- dedicated domain -> `Policy0`.

Третий dedicated domain не имеет match и обязан пройти через evaluator fallback `System`.

### End-to-end matrix

Harness запускает один production `dnsPolicyShadowServer` и последовательно проверяет:

- System / UDP -> reply;
- System / TCP -> reply;
- Policy1 / UDP -> reply;
- Policy1 / TCP -> reply;
- Policy0 / UDP -> no reply, fail-closed;
- Policy0 / TCP -> no reply, fail-closed.

Ответ считается валидным только если это DNS response с исходным transaction ID.

### Exact mark evidence

На время CLI smoke production `dnsPolicySetSocketMark` оборачивается без изменения его поведения:

1. исходный setter ставит `SO_MARK`;
2. `getsockopt(SO_MARK)` проверяет actual mark на том же fd;
3. observed marks записываются в evidence.

PASS требует минимум:

- два observed `0xffffaab` — Policy1 UDP + TCP;
- два observed `0xffffaaa` — Policy0 UDP + TCP.

System не вызывает mark setter.

### Lifetime

CLI smoke:

- стартует listener только внутри текущего процесса;
- после матрицы отменяет context;
- закрывает UDP/TCP listener;
- завершается;
- обычный daemon/runtime не запускается.

### Safety boundary

- package install не требуется;
- native DNS :53 не bind'ится;
- Keenetic config не меняется;
- DHCP/DNS redirect не меняется;
- persisted rules не меняются;
- service restart не требуется;
- public activation отсутствует;
- production auto-start shadow listener отсутствует.

`production_driver_ready=false` сохраняется.

## P18O — shadow runtime hardening

P18N hardware smoke доказал на реальном Keenetic полный synthetic path для UDP и TCP:

- System получил DNS response по UDP и TCP без SO_MARK;
- Policy1 получил DNS response по UDP и TCP с actual SO_MARK `0xffffaab`;
- Policy0 выставил actual SO_MARK `0xffffaaa` по UDP и TCP и не получил unmarked fallback;
- loopback listener закрылся после smoke;
- native :53, persisted rules, service state и Keenetic config не менялись.

### Bounded concurrency

Shadow config теперь содержит `MaxConcurrent`:

- default `32`;
- допустимый диапазон `1..256`;
- один общий semaphore ограничивает суммарные UDP/TCP in-flight handlers.

Это не даёт diagnostic/shadow listener бесконтрольно создавать goroutines при burst нагрузке.

### Explicit DNS failure semantics

Ошибки policy/evaluation/marked upstream path больше не превращаются в silent UDP timeout или TCP EOF для валидного DNS query.

Для валидного одиночного DNS question shadow listener строит локальный response:

- тот же transaction ID;
- тот же question;
- `QR=1`;
- `RCODE=SERVFAIL`;
- answer/authority/additional counts = 0.

Критически важно: SERVFAIL создаётся **после** fail-closed ошибки. Он не делает System fallback и не отправляет второй unmarked upstream request.

### Telemetry/evidence

Каждый shadow server ведёт process-local snapshot:

- requests;
- successful upstream responses;
- forwarding failures;
- emitted SERVFAIL responses;
- current in-flight;
- peak in-flight;
- policy selection counts по `System/PolicyN`.

Telemetry не пишется на диск и не экспортируется public API на P18O.

### Hardened smoke contract

P18N CLI smoke автоматически адаптирован к новым failure semantics.

Теперь Policy0 UDP/TCP обязан:

- выставить exact mark `0xffffaaa`;
- получить marked-path failure;
- получить локальный DNS `SERVFAIL` вместо timeout/EOF;
- не использовать System fallback.

Smoke также требует exact counters:

- requests = 6;
- successes = 4;
- failures = 2;
- SERVFAIL = 2;
- System selections = 2;
- Policy1 selections = 2;
- Policy0 selections = 2.

### Persisted-rule preparation boundary

P18O не включает persisted rules в listener автоматически.

Но runtime теперь имеет необходимые deterministic primitives для следующего controlled shadow stage:

- bounded listener lifetime/concurrency;
- explainable policy selection;
- exact egress primitive;
- client-visible fail-closed response;
- process-local evidence snapshot.

### Safety boundary

P18O по-прежнему:

- не запускает shadow listener в normal daemon mode;
- не bind'ит native :53;
- не меняет Keenetic config/DHCP/redirect;
- не активирует persisted rules;
- не добавляет public activation API;
- не меняет live client DNS traffic.

`production_driver_ready=false` сохраняется.

## P18P — controlled persisted-rule shadow mode

P18P добавляет CLI-only controlled shadow mode поверх реального persisted policy document.

### Read-only persisted input

Default source:

`/opt/etc/routerforge/dns-policy-rules.json`

Mode:

1. требует существующий readable persisted file;
2. читает raw bytes и фиксирует SHA256 identity;
3. загружает document через существующий `dnsPolicyStore.Load()`;
4. проверяет schema version;
5. читает live Keenetic policy inventory;
6. повторно запускает `validateDNSPolicyRules` против live inventory;
7. только после успешных gates строит P18O shadow config.

Отсутствующий file является ошибкой controlled mode. P18P не трактует missing store как доказанно активный empty rule set.

### Listener contract

CLI mode использует тот же P18M/P18O server:

- loopback-only;
- port 53 запрещён;
- UDP/TCP;
- bounded concurrency;
- fail-closed SERVFAIL;
- exact `PolicyN -> SO_MARK` egress;
- bounded lifetime, default 15 seconds and hard maximum 10 minutes.

Normal daemon mode listener не запускает.

### Evidence manifest

После завершения CLI process печатает:

- store path;
- raw document SHA256;
- schema version;
- `updated_at`;
- validated rule count;
- live inventory count;
- listener/upstream;
- bounded duration;
- requests/success/failure/SERVFAIL/peak-in-flight counters.

P18P не объявляет конкретное persisted rule hardware-proven, если за окно не было запроса, который это правило реально matched.

### Safety boundary

P18P:

- store не изменяет;
- persisted rules не копирует и не переписывает;
- native :53 не bind'ит;
- DHCP/DNS redirect не меняет;
- normal DNS daemon path не меняет;
- public activation API не добавляет;
- system DNS takeover не выполняет.

`production_driver_ready=false` сохраняется.

## P18Q — hardware persisted-shadow evidence

Тестовый Keenetic не имел production store `/opt/etc/routerforge/dns-policy-rules.json`, и P18P корректно завершился fail-closed с `PERSISTED_STORE_NOT_FOUND`.

Чтобы не создавать production state только ради smoke, P18Q использовал временный document в `/opt/tmp` и exact production P18P path.

Hardware evidence:

- temporary persisted document SHA256 verified;
- schema version `1`;
- 2 rules validated against live inventory;
- live inventory count = 3;
- persisted loader/listener lifecycle PASS;
- real production store остался absent;
- System UDP/TCP PASS;
- Policy1 UDP/TCP PASS с exact `SO_MARK=0xffffaab`;
- Policy0 UDP/TCP получил local SERVFAIL после exact `SO_MARK=0xffffaaa`;
- requests=6, successes=4, failures=2, SERVFAIL=2;
- no unmarked fallback;
- listener cleanup PASS;
- installed RouterForge DNS service remained alive;
- native port 53, config and service state не менялись.

P18Q закрывает shadow/persisted hardware workstream.

## P18R — activation target pivot

P18G предполагал, что future activation должен транслировать `DNSPolicyRule` в неизвестные Keenetic policy dataplane objects.

P18J-P18Q доказали другую production architecture:

`DNSPolicyRule -> RouterForge evaluator/forwarder -> SO_MARK -> native Keenetic policy routing`

Следовательно отдельный Keenetic rule compiler не нужен и старые dataplane-mapping blockers больше не являются правильной production boundary.

### Новый activation target

Activation теперь означает lifecycle двух частей:

1. RouterForge DNS proxy/forwarder с validated persisted rules;
2. DNS ingress ownership — controlled transfer client DNS traffic с native Keenetic path на RouterForge и обратно.

Keenetic policy routing остаётся backend dataplane и не получает DNSPolicyRule objects.

### Takeover transaction driver

P18R добавляет `dnsPolicyTakeoverActivationDriver` поверх существующего P18F engine.

Apply ordering:

1. start RouterForge proxy;
2. verify RouterForge proxy readiness;
3. switch DNS ingress to RouterForge;
4. P18F Verify доказывает, что ingress действительно обслуживается RouterForge.

Rollback ordering намеренно обратный только частично:

1. **сначала restore native DNS ingress**;
2. затем stop RouterForge proxy;
3. затем verify native ingress identity.

Так RouterForge proxy не выключается раньше, чем восстановлен native DNS path.

### Failure semantics

- precheck/snapshot failure: no mutation;
- proxy start/verify failure after Applied boundary: rollback path;
- ingress switch/verify failure: rollback path;
- native ingress restore failure: `ambiguous`;
- native restore verification failure: `ambiguous`.

### Production readiness

`production_driver_ready=false` остаётся.

Теперь реальные blockers только три:

1. exact crash-safe ingress takeover primitive;
2. exact native ingress snapshot/readback identity;
3. hardware-proven restore ordering/ownership.

Policy evaluator, persisted rules, UDP/TCP forwarder и SO_MARK dataplane уже не blockers.

### Safety boundary

P18R не реализует production takeover primitive:

- native :53 не меняется;
- daemon auto-start policy routing не включается;
- public activation API отсутствует;
- persisted rules остаются inactive для обычных клиентов;
- никакой RCI/iptables mutation из нового driver пока нет.

## P18S — hardware ingress discovery

Read-only hardware discovery на Keenetic Hopper установил exact ingress topology:

- UDP `0.0.0.0:53` и `:::53` принадлежат `ndnproxy` PID 684;
- TCP `0.0.0.0:53` и `:::53` также принадлежат тому же `ndnproxy`;
- native command: `/usr/sbin/ndnproxy -c /var/ndnproxymain.conf ...`;
- LAN interface `br0` имеет IPv4 `192.168.10.1/24`;
- native DNS отвечает локально на 127.0.0.1;
- saved static name-server list пуст, active upstream приходит по DHCP на GigabitEthernet1;
- RouterForge DNS service остаётся отдельным процессом и native :53 не занимает.

Выбран один takeover primitive: **runtime-only IPv4 iptables NAT PREROUTING redirect**, без остановки `ndnproxy`.

## P18T — reversible ingress takeover smoke

R105 добавляет CLI-only primitive и hardware smoke.

### Listener safety

Обычные shadow/persisted modes сохраняют loopback contract. Non-loopback listener разрешён только если одновременно:

- указан explicit IPv4, не wildcard и не loopback;
- указан explicit interface;
- Linux runtime доказывает, что IPv4 реально принадлежит этому UP interface;
- port 53 запрещён.

### Ingress ownership

RouterForge резервирует отдельную NAT chain `RF_DNS_INGRESS`.

Activation primitive:

1. reconcile only stale RouterForge chain/jumps;
2. start/verify RouterForge proxy on explicit LAN high-port;
3. create `RF_DNS_INGRESS`;
4. add UDP/TCP dport 53 REDIRECT внутри chain;
5. insert exact PREROUTING jumps только для выбранного LAN interface;
6. verify every jump/redirect through `iptables -C`.

Native `ndnproxy :53` остаётся запущен всё время.

Rollback:

1. delete exact RouterForge PREROUTING jumps;
2. flush/delete only `RF_DNS_INGRESS`;
3. verify chain/jumps absent;
4. direct UDP/TCP query to native LAN-IP:53 must succeed;
5. only after native recovery proxy process may stop.

Rules runtime-only: никакого iptables-save/persistence.

### Hardware-smoke boundary

CLI smoke доказывает:

- concrete non-loopback RouterForge listener;
- exact Policy1/Policy0 marked dataplane on that listener;
- exact iptables install/readback/removal;
- native UDP/TCP DNS recovery after rollback.

Local CLI smoke не заявляет packet-counter proof конкретного external LAN frame through PREROUTING; такой proof можно добавить финальным client-side acceptance probe, если требуется перед public activation.

`production_driver_ready=false` сохраняется до hardware PASS R105 и final public activation hardening.

## P18U — production transaction wiring and final acceptance

R105 hardware PASS доказал reversible ingress primitive:

- exact LAN listener ownership;
- runtime-only IPv4 PREROUTING redirect;
- UDP/TCP rule install/readback;
- Policy1 exact mark `0xffffaab`;
- Policy0 exact mark `0xffffaaa` with fail-closed SERVFAIL;
- rollback removed all RouterForge firewall state;
- native `ndnproxy` recovered UDP/TCP without restart.

R106 wires that proven primitive into the P18R transaction driver.

Production transaction now uses:

`precheck -> snapshot native ingress -> start proxy -> verify proxy -> install ingress -> verify ingress -> committed`

Acceptance CLI then deliberately exercises post-commit recovery:

`restore native ingress -> stop RouterForge proxy -> verify native UDP/TCP`

The persistent/public activation API remains disabled until this exact acceptance passes on hardware.

### Hardening debt closed in R106

- zero-valued `ordinal`, `rule_priority`, and `specificity` are explicit in JSON;
- persisted store `Load()` performs semantic rule validation while preserving stored rule order;
- repeated atomic store overwrite is covered by regression test;
- HEAD for generic GET-only API and `/v1/policy-rules` suppresses response bodies;
- PUT content-type gate was already enforced by the existing mutation-header helper and is retained unchanged.

## P18V — final external ingress acceptance and closeout

Final hardware acceptance is complete.

The transaction-driver acceptance reached `committed`, then deliberate post-commit rollback restored native UDP/TCP DNS without restarting `ndnproxy`; all RouterForge runtime firewall state was absent afterwards.

The real external client proof used the nested test topology:

`Windows 192.168.1.2 -> main router 192.168.1.1 -> test Keenetic -> 192.168.10.1:53`

Because the client is upstream of the test Keenetic, the physical ingress is `eth3`, not `br0`. A real TCP DNS query produced 3 packets / 180 bytes on the exact NAT PREROUTING rule, and the temporary chain was removed afterwards.

`production_driver_ready=true`.

This means the internal production driver, transaction ordering, marked egress, interface-scoped IPv4 ingress takeover and verified rollback have completed hardware acceptance.

It does not enable persistent/public activation by itself:

- public activation endpoint remains disabled;
- daemon autostart takeover remains disabled;
- no persistent iptables rules are installed;
- activation remains explicit and transaction-gated;
- the accepted ingress primitive is IPv4 only and must use the actual client ingress interface;
- IPv6 DNS takeover remains outside P18 scope.

The UDP timeout from the upstream Windows client is tracked separately as a nested test-network path issue; TCP/53 external ingress was directly proven through PREROUTING.

## P18 result

**P18 DNS Policy Router: COMPLETE.**

All P18 blockers are closed. No additional P18 discovery or hardware stage is required.