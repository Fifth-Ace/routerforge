# Установка и обновление RouterForge

## Поддерживаемая платформа

RouterForge рассчитан на:

- Keenetic / Netcraze с KeeneticOS/NDMS;
- ARM64 / aarch64 — основной аппаратно проверенный target;
- MIPS — experimental Stable/Beta preview без физической hardware validation;
- MIPSel — experimental Stable/Beta preview с частичной physical validation на Keenetic Giga KN-1010 (fresh install/basic operation);
- Entware в `/opt`;
- рабочий `/opt/bin/opkg` или `opkg` в `PATH`;
- `sha256sum`;
- `curl` или `wget`.

Web UI работает на порту **2233**.

Текущий Stable baseline:

```text
product           0.6.1
routerforge-core  0.6.1
routerforge-dns   0.4.20  (optional)
```

Компоненты версионируются независимо. Fresh Stable 0.6 bootstrap устанавливает только RouterForge Core; DNS и остальные официальные возможности выбираются после запуска через Центр приложений. Уже установленные optional-пакеты bootstrap сам по себе не удаляет.

## Stable — рекомендуемый канал

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

Для публичной HTTPS-установки в примерах намеренно используется **Entware curl** (`/opt/bin/curl`): это обходит несовместимости stock BusyBox `wget`/TLS, встречающиеся на отдельных сборках KeeneticOS.

Bootstrap генерируется CI из актуального `routerforge-stable-index.json`.

Он:

1. проверяет ARM64 и Entware;
2. определяет `opkg`;
3. скачивает **точный** Core asset из release-index;
4. сверяет SHA256 IPK;
5. устанавливает `routerforge-core`;
6. оставляет DNS и остальные capabilities на выбор пользователя через Центр приложений.

Core и модули могут иметь разные версии — bootstrap не предполагает общий номер версии.

После установки:

```text
http://<ip-роутера>:2233
```

Проверка:

```sh
echo "=== CORE ==="
wget -qO- http://127.0.0.1:2233/api/health
echo

echo "=== PACKAGES ==="
/opt/bin/opkg list-installed | grep '^routerforge-' | sort

echo "=== OPTIONAL DNS MODULE ==="
if /opt/bin/opkg list-installed | grep -q '^routerforge-dns '; then
    cat /opt/share/routerforge/modules/dns/manifest.json 2>/dev/null
    echo
    wget -qO- http://127.0.0.1:2233/api/modules/dns/health
    echo
else
    echo "routerforge-dns is not installed (optional)"
fi
```

Для Core 0.4.2+ loopback `GET/HEAD /api/modules/<id>/health` используется как readiness probe и остаётся доступен локально даже при включённой RouterForge-auth. Остальные защищённые API по-прежнему требуют сессию.

## MIPS / MIPSel experimental preview

Stable 0.6 и Beta публикуют target-specific пакеты для:

```text
aarch64-3.10
mips-3.4
mipsel-3.4
```

ARM64 — аппаратно проверенный production target.

MIPS/MipSel проходят cross-build, QEMU runtime smoke и встроенный runtime compatibility probe.

MIPSel дополнительно имеет partial physical validation на Keenetic Giga KN-1010: fresh installation и базовая штатная работа подтверждены. Полные upgrade/rollback/uninstall, DNS/Module ABI и resource-footprint сценарии ещё не закрыты.

MIPS big-endian физически не проверен. Оба non-ARM64 target пока остаются experimental и требуют явного подтверждения.

Для non-interactive установки:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | ROUTERFORGE_MIPS_PREVIEW=1 sh
```

Если runtime probe вернёт `degraded`, нужен отдельный явный override:

```sh
ROUTERFORGE_MIPS_ALLOW_DEGRADED=1
```

Статус `blocked` override'ом не обходится.

## Beta

Beta публикуется из `dev` и предназначена для проверки новых возможностей до Stable:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

GitHub release `RouterForge Beta` помечен как **Pre-release**.

Для Beta 0.7.1 fresh bootstrap по-прежнему устанавливает только `routerforge-core`.
Пакеты Beta используют prerelease-safe opkg versions `0.7.1~beta.1`, а GitHub asset/tag — форму `0.7.1-beta.1`.
Optional RouterForge modules, интеграции и generic Entware packages выбираются после запуска через **Центр приложений**. Уже установленные optional-пакеты bootstrap не удаляет.

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

После установки Core откройте **Центр приложений** и установите нужные возможности:

- RouterForge DNS;
- RouterForge Control;
- RouterForge Monitoring (System + Thermal + Storage + Network);
- Profiling.

В Beta 0.7.1 `routerforge-monitoring` заменяет четыре старых split monitoring package и сохраняет compatibility API для миграции. После реального upgrade путь проверяется read-only gate из [MONITORING_MIGRATION.md](MONITORING_MIGRATION.md).

Установка одного модуля не требует обновлять остальные.

## Обновления

### Через Центр приложений

Это основной способ.

Для каждого RouterForge-компонента показываются:

- `Installed`;
- `Available`;
- package;
- состояние сервиса;
- доступные actions.

Remote release-index и Центр приложений Registry автоматически обновляются примерно раз в час.

Кнопка **«Проверить обновления»**:

- игнорирует часовой таймер;
- немедленно загружает свежий Registry и release-index;
- ждёт результат;
- сразу пересчитывает доступные обновления.

При наличии обновлений кнопка подсвечивается и показывает их количество.

**«Обновить всё RouterForge»** обновляет официальные RouterForge-компоненты независимо: модули сначала, Core последним.

Lifecycle считается успешным только после повторного чтения локального package state. Для `update` Core дополнительно проверяет, что фактически установленная версия совпала с target version из release-index. Во время restart Module ABI runtime может кратковременно быть недоступен; UI показывает состояние перезапуска и сам повторяет health-check вместо вывода сырого proxy JSON.

`opkg` иногда оставляет старые stanza со статусом `not-installed`. Core 0.4.3+ не считает такие tombstone-записи установленным пакетом и не позволяет им подменять отображаемую версию.

### Проверка установленных версий

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

## Stable ↔ Beta

Не рекомендуется постоянно смешивать пакеты разных каналов.

При осознанной смене канала:

1. запустите bootstrap целевого канала;
2. откройте Центр приложений;
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
