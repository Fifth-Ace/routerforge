# Установка и обновление RouterForge

## Поддерживаемая платформа

RouterForge рассчитан на:

- Keenetic / Netcraze с KeeneticOS/NDMS;
- ARM64 / aarch64;
- Entware в `/opt`;
- рабочий `/opt/bin/opkg` или `opkg` в `PATH`;
- `sha256sum`;
- `curl` или `wget`.

Web UI работает на порту **2233**.

## Stable — рекомендуемый канал

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

Для публичной HTTPS-установки в примерах намеренно используется **Entware curl** (`/opt/bin/curl`): это обходит несовместимости stock BusyBox `wget`/TLS, встречающиеся на отдельных сборках KeeneticOS.

Bootstrap генерируется CI из актуального `routerforge-stable-index.json`.

Он:

1. проверяет ARM64 и Entware;
2. определяет `opkg`;
3. скачивает **точные** Core и DNS assets из release-index;
4. сверяет SHA256 каждого IPK;
5. устанавливает `routerforge-core`;
6. устанавливает `routerforge-dns`;
7. оставляет остальные capabilities на выбор пользователя через Marketplace.

Core и DNS могут иметь разные версии — bootstrap не предполагает общий номер версии.

После установки:

```text
http://<ip-роутера>:2233
```

Проверка:

```sh
wget -qO- http://127.0.0.1:2233/api/health
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

## Beta

Beta публикуется из `dev` и предназначена для проверки новых возможностей до Stable:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

GitHub release `RouterForge Beta` помечен как **Pre-release**.

## Compatibility launcher

Для совместимости в репозитории остаётся:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/install-repo.sh | sh
```

По умолчанию он запускает Stable bootstrap.

Beta через launcher:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/install-repo.sh | \
  ROUTERFORGE_CHANNEL=beta sh
```

Рекомендуется использовать прямую команду соответствующего release channel.

## Что устанавливать дальше

После Core + DNS откройте **Marketplace** и установите нужные возможности:

- RouterForge Control;
- System Monitor;
- Thermal Monitor;
- Storage Monitor;
- Network Monitor;
- Profiling.

Установка одного модуля не требует обновлять остальные.

## Обновления

### Через Marketplace

Это основной способ.

Для каждого RouterForge-компонента показываются:

- `Installed`;
- `Available`;
- package;
- состояние сервиса;
- доступные actions.

Remote release-index и Marketplace Registry автоматически обновляются примерно раз в час.

Кнопка **«Проверить обновления»**:

- игнорирует часовой таймер;
- немедленно загружает свежий Registry и release-index;
- ждёт результат;
- сразу пересчитывает доступные обновления.

При наличии обновлений кнопка подсвечивается и показывает их количество.

**«Обновить всё RouterForge»** обновляет официальные RouterForge-компоненты независимо: модули сначала, Core последним.

### Проверка установленных версий

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

## Stable ↔ Beta

Не рекомендуется постоянно смешивать пакеты разных каналов.

При осознанной смене канала:

1. запустите bootstrap целевого канала;
2. откройте Marketplace;
3. выполните ручную проверку;
4. обновите остальные установленные RouterForge-компоненты до доступных версий этого канала.

Канал Core определяет, какой release-index и какую ветку Registry использует платформа:

- Stable → `routerforge-stable` + Registry из `main`;
- Beta → `routerforge-beta` + Registry из `dev`.

## Сервис Core

```sh
/opt/etc/init.d/S90routerforge stop
/opt/etc/init.d/S90routerforge start
/opt/etc/init.d/S90routerforge restart
```

Log:

```sh
tail -f /opt/var/log/routerforge.log
```

## Удаление

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```

Удаляются RouterForge-пакеты и legacy `dns-monitor-*`, если они остались установленными.

Каталоги конфигурации намеренно сохраняются.

## Основные пути

```text
/opt/bin/routerforge
/opt/etc/routerforge/
/opt/etc/init.d/S90routerforge
/opt/var/log/routerforge.log
/opt/var/run/routerforge-*.sock
/opt/var/cache/routerforge/
```

## Миграция с DNS Monitor

RouterForge использует новый package namespace `routerforge-*`.

Пакеты содержат migration metadata для legacy `dns-monitor*`, а Core переносит совместимые настройки безопасности из старого namespace при первом обновлении.

Канонический репозиторий — `Fifth-Ace/routerforge`. Исторические ссылки на прежнее имя поддерживаются GitHub redirect и compatibility fallback в Core.
