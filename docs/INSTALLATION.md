# Установка и обновление RouterForge

## Stable 0.7.1

Текущий Stable train:

```text
routerforge-core        0.7.1
routerforge-dns         0.7.1
routerforge-admin       0.7.1
routerforge-monitoring  0.7.1
routerforge-profiling   0.7.1
```

Fresh bootstrap устанавливает Core. DNS, Management, Monitoring и Profiling выбираются через **Центр приложений**.

## Targets

| Target | Статус |
| --- | --- |
| `aarch64-3.10` | primary fully hardware-validated Stable target |
| `mipsel-3.4` | experimental, partial physical validation на KN-1010 |
| `mips-3.4` | experimental, no physical hardware validation |

Требуются Keenetic/Netcraze с KeeneticOS/NDMS, Entware в `/opt`, `opkg`, `sha256sum` и `curl`/`wget`.

## Установка Stable

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

После установки: `http://<ip-роутера>:2233`

Проверка:

```sh
wget -qO- http://127.0.0.1:2233/api/health
echo
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

## Upgrade с Stable 0.6.x

1. Запустите Stable bootstrap.
2. Откройте **Центр приложений**.
3. Нажмите **Проверить обновления**.
4. Обновите установленные capabilities.
5. Если были split monitoring packages, обновление заменит их на `routerforge-monitoring`.

Не удаляйте old monitoring files вручную. Migration должна пройти через package lifecycle.

После migration:

```sh
ROUTERFORGE_MONITORING_EXPECTED_VERSION='0.7.1' \
  sh scripts/verify-monitoring-migration.sh runtime
```

Подробнее: [MONITORING_MIGRATION.md](MONITORING_MIGRATION.md).

## MIPS / MIPSel

Non-ARM64 требует explicit experimental opt-in:

```sh
ROUTERFORGE_MIPS_PREVIEW=1
```

Для `degraded` probe:

```sh
ROUTERFORGE_MIPS_ALLOW_DEGRADED=1
```

`blocked` override не допускает. См. [ARCHITECTURES.md](ARCHITECTURES.md).

## Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Pre-release package versions используют `~`, например `0.7.1~beta.4`, чтобы сортироваться ниже Stable `0.7.1`.

## Stable ↔ Beta

При смене channel:
1. запустите bootstrap целевого channel;
2. форсируйте проверку в App Center;
3. приведите installed RouterForge packages к versions этого channel.

Stable → `routerforge-stable` + Registry `main`.
Beta → `routerforge-beta` + Registry `dev`.

## Management

`routerforge-admin` добавляет Processes/Services, File Manager, Maintenance, Entware Terminal и Keenetic NDM Console. Keenetic mode запускает фиксированный backend `ndmc`.

## Core service

```sh
/opt/etc/init.d/S90routerforge restart
tail -f /opt/var/log/routerforge.log
```

## Удаление

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```
