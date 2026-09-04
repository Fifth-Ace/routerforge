# RouterForge Marketplace

Marketplace — каталог официальных RouterForge capabilities и поддерживаемых внешних проектов для Keenetic/Netcraze Entware.

## Источники данных

Core объединяет:

1. bundled Registry;
2. remote Registry из GitHub;
3. локальное состояние Entware/opkg;
4. RouterForge release-index текущего канала.

Для Stable remote Registry читается из `main`, для Beta — из `dev`.

## Что является источником истины

### Installed

Установленная версия RouterForge package определяется по локальной базе `opkg`.

Core учитывает только stanza, у которых `Status` действительно заканчивается состоянием `installed`. Старые записи вида `install prefer not-installed`, которые `opkg` может оставить после upgrade, считаются tombstone и игнорируются. Поэтому старая версия не должна перекрывать текущую установленную версию в Marketplace.

### Available

Доступная версия официального RouterForge package берётся из release-index:

```text
routerforge-stable-index.json
routerforge-beta-index.json
```

Release-index содержит:

- component id;
- package;
- version;
- exact asset filename;
- exact GitHub release URL;
- SHA256;
- compatibility metadata.

Core не строит имя IPK из собственной версии.

## Проверка обновлений

Автоматическая remote-проверка Registry и release-index выполняется примерно **раз в час**.

Browser может чаще перечитывать локальный `/api/catalog`; это не означает постоянные запросы к GitHub.

Кнопка **«Проверить обновления»** выполняет force refresh и ждёт результат.

Если доступны executable updates, кнопка подсвечивается и показывает количество.

## Независимые обновления

RouterForge-модули не обязаны иметь версию Core.

CI публикует новый asset только для компонента, version которого изменился.

Если version не менялся, release pipeline сохраняет предыдущий asset/URL/SHA256 и не заменяет бинарник под тем же номером.

При batch update:

1. обновляются optional modules;
2. Core обновляется последним.

После lifecycle action Core повторно строит catalog из локального package state. `update` считается успешным только если package установлен **и** его фактическая версия совпадает с target version из release-index. Один только успешный exit code `opkg` недостаточен.

Для Module ABI runtime package-installed и runtime-ready — независимые состояния. Во время restart proxy может кратковременно отдавать `503`, а UI выполняет health preflight/retry и восстанавливает iframe после готовности socket.

## Проверка IPK

Для официального RouterForge lifecycle:

1. Core получает exact URL + SHA256 из release-index;
2. разрешает только HTTPS GitHub release URL проекта;
3. скачивает IPK во временный каталог;
4. вычисляет SHA256;
5. сравнивает с release-index;
6. только затем вызывает `opkg install`;
7. временный IPK удаляется.

## Trust model

Marketplace различает доверие к источнику каталога и право выполнить lifecycle action.

Типичные статусы:

- `OFFICIAL` — официальный RouterForge component;
- `VERIFIED` — внешний manifest прошёл review/approval;
- `UNVERIFIED` — metadata есть, но lifecycle не должен считаться доверенным;
- `CHANGED` — manifest изменился после approval;
- `BLOCKED` — выполнение запрещено;
- `DEPRECATED` — устаревший entry.

Approval привязан к manifest SHA256.

## Никакого arbitrary shell из Registry

Manifest не является root-shell script.

Executable lifecycle ограничен поддерживаемыми Core типами, например:

- RouterForge verified release install/update;
- ограниченный `opkg` lifecycle;
- structured operations из разрешённого набора.

Неизвестный method не выполняется.

## Сторонние проекты

Сторонний проект может быть:

- обнаружен;
- показан как installed external;
- снабжён project URL;
- иметь compatibility/status metadata;
- иметь lifecycle только если он явно реализован и разрешён.

RouterForge не должен молча присваивать себе вручную установленный внешний проект.

## Удаление

Remove требует отдельного разрешённого plan.

Для destructive lifecycle UI использует явное подтверждение; backend повторно проверяет разрешение и ожидаемое имя.

## Package management marker

Официальный Core package включает:

```text
/opt/etc/routerforge/package-management.enabled
```

Это включает RouterForge package-management mode.

Legacy marker читается только для миграционной совместимости.

## Release channels

| Channel | Branch | GitHub release | Назначение |
| --- | --- | --- | --- |
| Stable | `main` | `routerforge-stable` | публичный production channel |
| Beta | `dev` | `routerforge-beta` | тестирование перед promotion |

Beta release намеренно помечен GitHub как **Pre-release**.
Stable должен быть обычным **Latest** release.
