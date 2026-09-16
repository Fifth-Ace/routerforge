# DNS mutation safety — P16E

Статус: **component validator + control-plane runtime probe**

P16E усиливает существующий transactional DNS mutation flow. RouterForge уже делал native snapshot конфигурации Keenetic, записывал только затронутые DNS-секции, проверял RCI readback и откатывал native state при несовпадении. P16E добавляет два явных обязательных gate перед успешным завершением mutation.

## 1. Component validator

До первой native-записи проверяется mutation envelope:

- desired state обязан существовать;
- список изменяемых протоколов не может быть пустым;
- разрешены только `DNS`, `DoT`, `DoH`;
- продолжают действовать существующие Keenetic physical limits;
- resolver spec по-прежнему проходит protocol-specific normalization и validation до вызова apply.

Validator работает **до write** и therefore fail-closed.

## 2. Native readback

После записи сохраняется прежний обязательный RCI readback:

`WRITE → RCI READBACK → exact canonical comparison`

Если readback не совпал, RouterForge восстанавливает native DNS-секции из снимка, сделанного непосредственно перед mutation.

## 3. Runtime probe

После успешного readback и RouterForge metadata-write DNS module делает локальный probe собственного Unix control socket:

`GET /v1/health`

Probe имеет короткие timeout и проверяет contract:

- HTTP 200;
- `ok = true`;
- `module = "dns"`;
- `mutation_api = true`.

Если probe не проходит, mutation считается неуспешной и native DNS configuration откатывается через уже существующий rollback path. Для disable/enable RouterForge-only metadata caller также возвращает предыдущий disabled-store при ошибке apply flow.

## Что именно доказывает P16E

P16E доказывает:

`VALID CONFIG ENVELOPE → NATIVE WRITE → EXACT RCI READBACK → DNS CONTROL-PLANE ALIVE`

Это **не** является сетевым тестом конкретного upstream DNS-провайдера. Например, недоступность внешнего DoH endpoint из-за WAN/DPI сама по себе не превращает корректно сохранённый resolver в invalid config.

Такое разделение намеренное: configuration correctness и control-plane liveness проверяются синхронно в mutation transaction, а внешнее качество upstream остаётся задачей существующего rolling DNS health/quality monitoring.

## Rollback

При ошибке write, readback или runtime probe:

- native sections возвращаются к pre-change state;
- rollback failure не маскируется;
- вызывающий API получает ошибку;
- success не возвращается до прохождения runtime probe.

P16D Config Vault продолжает защищать файлы `/opt/etc/routerforge`. Native Keenetic DNS state не притворяется Config Vault artifact: для него используется собственный RCI snapshot/readback/rollback механизм DNS module.

## Следующий этап

Следующий шаг P16F — распространить общий validator/probe contract на другие component mutation flow и добавить единый transaction evidence/reporting слой без дублирования low-level safety logic.
