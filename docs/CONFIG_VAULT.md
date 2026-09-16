# Config Vault — фундамент P16A

Статус: **backend foundation / без пользовательского UI**
Этап: **P16A — Config Vault Core**
Схема хранения: **v1**

## Зачем это нужно

Config Vault — будущая общая история конфигураций RouterForge. Его задача — перед любым управляемым изменением уметь сохранить точное предыдущее состояние, однозначно идентифицировать содержимое, ограниченно хранить историю и не потерять последнюю подтверждённо рабочую точку.

Это **не резервная копия всего `/opt`** и не замена существующим диагностическим snapshots Management.

Существующий `/tmp/routerforge-snapshots` хранит снимки *состояния системы* — процессы, службы, порты, storage, thermal и integrations. Config Vault хранит *управляемые конфигурационные артефакты* и будет использоваться для безопасного rollback.

## Что реализовано в P16A

Новый пакет:

`internal/platform/configvault`

Он предоставляет файловое хранилище snapshots со следующими свойствами:

- schema version `1`;
- явный `Root` хранилища и явный список разрешённых исходных корней;
- snapshot manifest: component, reason, transaction ID, timestamp и список артефактов;
- для каждого артефакта: ID, исходный путь, SHA-256, размер, mode и mtime;
- content-addressed objects по SHA-256;
- одинаковое содержимое физически хранится один раз;
- snapshot manifests записываются атомарно через `internal/safety`;
- исходные файлы должны быть обычными файлами и не могут проходить через symlink;
- путь вне разрешённых корней отклоняется;
- bounded limits на размер одного артефакта, snapshot, всего store и число snapshots;
- `LAST WORKING` хранится отдельно и защищается от обычной retention-очистки;
- после удаления старых manifests выполняется сборка неиспользуемых content objects;
- при невозможности соблюсти retention без удаления защищённого snapshot операция завершается ошибкой.

## Базовые лимиты

Если интеграция не задаёт свои более строгие ограничения:

- один артефакт: до **2 MiB**;
- суммарный payload snapshot: до **8 MiB**;
- весь Config Vault: до **32 MiB**;
- не более **32 snapshots**.

Это намеренно консервативные значения для роутера. Конкретный consumer может задавать меньшие лимиты.

## Формат snapshot

Пример структуры manifest:

```json
{
  "schema_version": 1,
  "id": "cfg-...",
  "component": "admin",
  "reason": "before-change",
  "transaction_id": "tx-...",
  "created_at": "2026-09-16T18:00:00Z",
  "artifacts": [
    {
      "id": "config",
      "source_path": "/opt/etc/example.conf",
      "sha256": "...",
      "size": 123,
      "mode": 384,
      "modified_at": "2026-09-16T17:59:00Z"
    }
  ],
  "total_bytes": 123
}
```

Фактический контент лежит отдельно в `objects/<sha256>`. Snapshot manifest содержит только ссылки на content-addressed objects и metadata.

## Что намеренно НЕ входит в P16A

P16A не угадывает, какие конфиги RouterForge обязан сохранять. Managed artifacts должен объявлять конкретный consumer.

На этом этапе нет:

- HTTP API;
- UI;
- diff;
- restore;
- automatic pre-change snapshot wiring;
- component validators;
- post-restore probe;
- export/import;
- жёстко выбранного production path вроде `/opt/var/lib/routerforge/config-vault`.

Так мы сначала фиксируем маленький, тестируемый и безопасный storage contract, а уже потом подключаем его к Management/DNS и реальным mutation flows.

## Следующий этап — P16B

P16B должен подключить Config Vault к Management / Maintenance:

1. выбрать и создать постоянный storage root в `/opt`;
2. определить первый curated набор managed config artifacts;
3. добавить read-only API: list/get/state;
4. добавить explicit create snapshot;
5. добавить `LAST WORKING`;
6. сделать diff metadata/content для выбранного snapshot;
7. подготовить restore preview без самого destructive apply.

После этого P16C добавит component restore с validate → apply → readback/probe → rollback/verify.
## P16B — API и интеграция с Maintenance

P16B подключает фундамент Config Vault к модулю Management / Maintenance.

### Постоянное хранилище

Production root:

`/opt/var/lib/routerforge/config-vault`

Управляемый конфигурационный root первого этапа:

`/opt/etc/routerforge`

Config Vault не сканирует весь `/opt` и не пытается сохранять произвольные пользовательские файлы. На P16B область намеренно ограничена конфигурациями RouterForge.

### Что считается управляемым артефактом

Maintenance рекурсивно обнаруживает обычные файлы внутри `/opt/etc/routerforge`.

Правила:

- symlink игнорируются;
- special files игнорируются;
- не более 128 файлов за один discovery;
- каждый файл получает стабильный artifact ID от относительного пути;
- полный исходный путь остаётся в manifest;
- сами snapshots по-прежнему проходят ограничения Config Vault P16A.

### API P16B

Read-only:

- `GET /v1/maintenance/config-vault` — состояние Vault, snapshots, retention, `LAST WORKING` и текущие managed artifacts;
- `GET /v1/maintenance/config-vault/<id>` — manifest одного snapshot;
- `GET /v1/maintenance/config-vault-diff?id=<id>` — сравнение snapshot с текущей конфигурацией;
- `GET /v1/maintenance/config-vault-restore-preview?id=<id>` — предварительный просмотр будущего restore без применения изменений.

Guarded mutations через существующий Core authorization contract:

- `POST /v1/maintenance/config-vault-capture` — ручной snapshot текущих RouterForge configs;
- `POST /v1/maintenance/config-vault-last-working` — назначение подтверждённого snapshot как `LAST WORKING`.

Capture требует явное подтверждение `SNAPSHOT`.

Назначение `LAST WORKING` требует явное подтверждение `LAST_WORKING`.

### Diff

Diff имеет четыре состояния:

- `unchanged` — текущий SHA-256 совпадает со snapshot;
- `changed` — путь существует, но содержимое изменилось;
- `removed` — файл был в snapshot, но сейчас отсутствует;
- `added` — файл появился после snapshot.

P16B не раскрывает содержимое файлов через API: наружу идут только metadata и SHA-256.

### Restore preview

Restore preview уже показывает, какие файлы изменятся, но **не умеет применять snapshot**.

Это намеренный safety gate:

`apply_enabled = false`

Реальный component restore переносится в P16C, где появится полный цикл:

`precheck → safety snapshot → validation → apply → readback/probe → commit`

При доказанном FAIL:

`rollback → rollback verification`

### Что изменилось относительно P16A

P16A был библиотечным storage contract.

P16B делает его частью Maintenance:

- выбран постоянный storage root;
- определена первая managed область;
- появились list/get/state API;
- появился ручной capture;
- `LAST WORKING` доступен через guarded mutation;
- появился diff;
- появился restore preview.

### Следующий этап — P16C

P16C добавит реальное безопасное восстановление одного snapshot с проверкой до и после применения. До завершения P16C Config Vault остаётся read/capture/preview системой и не выполняет destructive restore.

## P16C — безопасное восстановление snapshot

P16C включает реальный restore для управляемой области `/opt/etc/routerforge`.

Restore доступен только через guarded mutation API:

`POST /v1/maintenance/config-vault-restore`

Запрос обязан содержать:

- `snapshot_id`;
- точно совпадающий `confirm_snapshot_id`;
- `confirm = "RESTORE"`;
- необязательный `transaction_id`.

### Цепочка восстановления

RouterForge не копирует snapshot поверх текущих файлов напрямую. Операция проходит общий безопасный контур:

`PRECHECK → SAFETY SNAPSHOT → VALIDATION → APPLY → SHA-256 READBACK → COMMIT`

Перед первым изменением Config Vault обязательно делает отдельный safety snapshot текущей управляемой конфигурации.

Target snapshot повторно проверяется перед apply:

- schema и component;
- отсутствие дублирующихся artifact ID и путей;
- каждый путь обязан находиться внутри `/opt/etc/routerforge`;
- backing object читается через Config Vault с проверкой размера и SHA-256;
- symlink в целевом файле или родительском каталоге запрещает restore.

### Apply

Каждый файл публикуется через атомарную запись в том же каталоге.

После восстановления файлов snapshot удаляются только те текущие **обычные managed-файлы**, которых в snapshot нет. Symlink и special files restore не удаляет и не заменяет.

Права доступа восстанавливаются из manifest. Нулевой mode нормализуется в `0600`.

### Проверка после применения

После apply RouterForge строит новый diff между target snapshot и фактическим состоянием `/opt/etc/routerforge`.

Успех возможен только если:

`changed = false`

Это является обязательным SHA-256 readback probe.

P16C пока не перезапускает сервисы и не утверждает, что конкретный DNS/VPN/другой runtime функционально здоров. Runtime-specific validators и probes должны подключаться consumer-ами отдельно. Текущая гарантия P16C — **точное восстановление управляемых файлов**.

### Автоматический rollback

Если apply или post-apply readback завершается ошибкой:

1. RouterForge применяет pre-restore safety snapshot;
2. повторно сравнивает фактическое состояние с safety snapshot;
3. только после `changed = false` rollback считается подтверждённым.

Результат транзакции:

- `committed` — target snapshot применён и подтверждён readback;
- `rolled-back` — target не прошёл, исходное состояние восстановлено и подтверждено;
- `ambiguous` — не удалось доказать успешный rollback; требуется ручная диагностика.

`LAST WORKING` не меняется автоматически после restore. Он остаётся отдельным явным действием пользователя.

### Ограничение первого P16C

Restore требует, чтобы перед операцией существовал хотя бы один текущий managed-файл. Это позволяет всегда создать доказуемый safety snapshot до изменения системы.

### Следующий этап

После P16C фундамент Config Vault считается пригодным для подключения к реальным mutation flow отдельных компонентов: автоматический pre-change snapshot, component-specific validation и runtime probes.

## P16D — автоматические pre-change snapshots

P16D подключает Config Vault к реальным изменениям управляемой конфигурации.

### Что защищается автоматически

Если Admin File Manager собирается изменить **обычный файл** внутри `/opt/etc/routerforge`, RouterForge перед первым изменением автоматически создаёт Config Vault snapshot.

Покрыты file-level операции:

- create / edit через `/v1/files/write`;
- copy, если destination находится внутри managed root;
- move обычного файла, если source или destination затрагивает managed root;
- delete обычного managed-файла;
- chmod обычного managed-файла;
- legacy Maintenance restore из `routerforge-config-*.tar.gz`.

P16C Config Vault restore также входит в общий serialized mutation guard, но отдельный дополнительный pre-change snapshot ему не нужен: сам P16C уже обязательно создаёт собственный safety snapshot.

### Сериализация

Все mutation flow, затрагивающие `/opt/etc/routerforge`, проходят через один in-process mutex.

Критическая секция включает:

`FINAL RECHECK → PRE-CHANGE SNAPSHOT → MUTATION → VERIFY`

Это не позволяет двум запросам Admin одновременно получить один и тот же baseline и затем перетереть изменения друг друга между snapshot и apply.

### Empty baseline

Schema v1 теперь допускает snapshot с нулём artifacts.

Это нужно для корректной защиты самого первого managed-файла. Если `/opt/etc/routerforge` ещё не содержит обычных файлов, RouterForge всё равно создаёт доказуемый empty-baseline snapshot перед create/copy/restore.

Такой snapshot можно восстановить: restore удалит managed regular files, появившиеся после пустого baseline, и подтвердит результат через обычный SHA-256 diff.

### Fail closed

Если путь относится к managed root, а automatic pre-change snapshot создать не удалось, файловая mutation **не должна выполняться**.

Операции вне `/opt/etc/routerforge` продолжают работать без Config Vault snapshot.

### Что пока не входит

Config Vault хранит и восстанавливает файлы, а не состояние каталогов. Поэтому чисто directory-only операции File Manager (`mkdir`, chmod/delete/move пустого каталога) в P16D не объявляются защищёнными snapshot-механизмом.

Runtime-specific restart/health probe также остаётся отдельным следующим слоем: P16D защищает конфигурационное состояние до мутации, а P16C отвечает за доказуемое файловое восстановление.
