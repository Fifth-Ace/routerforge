# RouterForge — Центр приложений

**Центр приложений** — единая точка управления официальными компонентами RouterForge,
curated-интеграциями и пакетами Entware/OPKG.

> Compatibility note: имя файла `docs/MARKETPLACE.md`, корневой каталог `marketplace/`
> и старые `/api/catalog*` маршруты пока сохраняются как compatibility surface для старых
> Core и существующих ссылок. Пользовательское название продукта — **Центр приложений**.

## Вкладки

```text
Центр приложений
├── RouterForge
├── Интеграции
├── Entware
├── Установлено
└── Обновления
```

- **RouterForge** — Core и optional-пакеты платформы.
- **Интеграции** — curated сторонние проекты, которые RouterForge умеет обнаруживать.
- **Entware** — generic OPKG browser.
- **Установлено** — агрегированное состояние RouterForge, интеграций и Entware.
- **Обновления** — агрегированный список доступных обновлений.

## Источники данных

Core объединяет:

1. bundled Registry;
2. remote Registry из GitHub;
3. локальное состояние Entware/opkg;
4. target-specific RouterForge release-index текущего канала.

Для Stable remote Registry читается из `main`, для Beta — из `dev`.

## Что является источником истины

### Installed

Установленная версия package определяется по локальной базе `opkg`.

Core учитывает только stanza, у которых `Status` действительно заканчивается состоянием
`installed`. Старые записи вида `install prefer not-installed` считаются tombstone и
игнорируются.

### Available для RouterForge

Для официальных RouterForge package authoritative source — target-specific release-index:

```text
routerforge-stable-index.json
routerforge-beta-index.json
```

Release-index содержит, когда metadata доступна:

- component id;
- package;
- version;
- architecture;
- exact asset filename;
- exact GitHub release URL;
- SHA256;
- minimum Core version;
- download size;
- estimated installed payload size;
- dependencies;
- conflicts.

Core не строит имя IPK из собственной версии и не выдумывает package metadata.

### OPKG feed

`scripts/build-feed.sh` уже формирует `Packages` и `Packages.gz`.
Для RouterForge 0.6 release-index остаётся authoritative source для собственного lifecycle.
`Packages.gz` существует как параллельное OPKG-представление и не заменяет release-index
без отдельной миграции. Решение зафиксировано в `APP_CENTER_RELEASE_FEED_ADR.md`.

## Entware / OPKG read path

Центр приложений использует реальные OPKG команды и кэширует тяжёлые списки:

```text
opkg list
opkg list-installed
opkg list-upgradable
opkg info <pkg>
opkg status <pkg>
```

Показываются:

- package/name/description;
- installed и available version;
- architecture;
- section/maintainer/source;
- download и installed size, когда OPKG их сообщает;
- dependencies и reverse dependency context;
- installed/upgradable state.

Поиск не запускает полный `opkg list` на каждый keystroke. Кэш инвалидируется после
update/install/remove/upgrade и имеет TTL/manual refresh.

## Preflight и действия

Перед mutation выполняется preflight. Он может показать:

- свободное место в `/opt`;
- зависимости;
- reverse dependencies/removal consequences;
- source/feed;
- ожидаемое действие.

Package name валидируется. Команда запускается как executable + argv, без построения
произвольной shell-строки.

Для non-Core действий используется job model:

- один глобальный package-manager lock;
- bounded output;
- timeout;
- SSE live output;
- cancellation;
- post-action refresh;
- post-action verification;
- persistent action history.

Текущий App Center action API:

```text
POST   /api/apps/preflight
GET    /api/apps/actions
POST   /api/apps/actions
GET    /api/apps/actions/<id>
GET    /api/apps/actions/<id>/events
DELETE /api/apps/actions/<id>
```

История действий хранится в:

```text
/opt/var/log/routerforge-app-center.jsonl
```

с ограниченной ротацией.

### Core self-update

Самообновление `routerforge-core` намеренно не выполняется как обычный in-process async job:
успешный update перезапускает сам Core. Для него сохраняется отдельный restart-safe lifecycle,
а batch update выполняет Core последним.

### Generic OPKG rollback

Для generic Entware package RouterForge не обещает собственный rollback поверх семантики
`opkg`. UI должен отличать:

```text
RouterForge managed action -> exact release metadata + verification
Generic OPKG action        -> opkg semantics, rollback не гарантирован
```

## Проверка обновлений

Remote Registry и release-index периодически обновляются с throttling/cache.
Ручная **«Проверить обновления»** обходит обычный interval и немедленно пересчитывает
доступные версии.

RouterForge-компоненты версионируются независимо. Если component version не менялась,
release pipeline сохраняет уже опубликованный same-version asset вместо молчаливой
подмены бинарника.

## Проверка IPK RouterForge

Для официального lifecycle:

1. Core получает exact URL + SHA256 из release-index;
2. разрешает ожидаемый HTTPS GitHub release source;
3. скачивает IPK во временный каталог;
4. вычисляет SHA256;
5. сравнивает его с release-index;
6. вызывает `opkg install`;
7. повторно читает package state/version.

Успешный exit code `opkg` сам по себе не является достаточным доказательством update.

## Trust model интеграций

Типичные статусы:

- `OFFICIAL`;
- `VERIFIED`;
- `UNVERIFIED`;
- `CHANGED`;
- `BLOCKED`;
- `DEPRECATED`.

`CHANGED`, `BLOCKED` и `DEPRECATED` не получают автоматический executable lifecycle.
Approval привязан к проверяемой версии manifest.

Manifest не является root-shell script. Неизвестный lifecycle method не выполняется.
Upstream install shell script никогда не запускается автоматически только потому, что URL
присутствует в Registry.

## Сторонние проекты

Интеграция может быть:

- обнаружена по package/path/process/service;
- показана как installed external;
- снабжена version/service/project URL;
- иметь compatibility и conflict hints;
- получить typed package action, только если действие однозначно и разрешено.

Typed `web` metadata (`scheme`, `port`, `path`, `embed`) в 0.6 является **только schema
groundwork** для следующего этапа. Она не включает generic reverse proxy, произвольный SSRF
surface или автоматическое iframe-embedding.

Полный embedded workspace остаётся отдельным security gate: per-session request token,
strict Origin/CORS и изоляция внешнего iframe должны быть реализованы до массового embedding.

## Package management marker

Официальный Core package включает:

```text
/opt/etc/routerforge/package-management.enabled
```

Legacy marker читается только для миграционной совместимости.

## Release channels

| Channel | Branch | GitHub release | Назначение |
| --- | --- | --- | --- |
| Stable | `main` | `routerforge-stable` | production channel |
| Beta | `dev` | `routerforge-beta` | pre-release validation |

Stable продвигается только после exact Beta verification и требуемого hardware pass.
