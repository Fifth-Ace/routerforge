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
