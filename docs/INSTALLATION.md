# Установка и обновление RouterForge

## Stable 0.9.0

Текущий Stable train после promotion:

```text
routerforge-core           0.9.0
routerforge-dns            0.8.1   (min Core 0.9.0)
routerforge-admin          0.8.1   (min Core 0.9.0)
routerforge-monitoring     0.8.0   (min Core 0.9.0)
routerforge-network-tools  0.9.0   (min Core 0.9.0)
routerforge-profiling      0.7.1   (min Core 0.7.1)
```

Компоненты версионируются независимо; см. [VERSIONING.md](VERSIONING.md).

Fresh bootstrap устанавливает Core. DNS, Management, Monitoring, Network Tools и Profiling
выбираются через **Центр приложений**.

## Targets

| Target | Статус |
| --- | --- |
| `aarch64-3.10` | primary fully hardware-validated Stable target |
| `mipsel-3.4` | experimental, partial physical validation на KN-1010 |
| `mips-3.4` | experimental, no physical hardware validation |

Требуются Keenetic/Netcraze с KeeneticOS/NDMS, Entware в `/opt`, `opkg`, `sha256sum`
и `curl`/`wget`.

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

## Upgrade с Stable 0.8.0

1. Запустите Stable bootstrap.
2. Откройте **Центр приложений**.
3. Нажмите **Проверить обновления**.
4. Обновите установленные RouterForge capabilities.
5. App Center применит обычные модули до Core, а Core — последним, затем восстановит shell.

Не удаляйте package-owned files вручную.

## Monitoring migration

Исторический переход split System/Thermal/Storage/Network → `routerforge-monitoring`
остаётся поддерживаемым migration contract.

После Stable 0.9.0 ожидаемая версия consolidated package:

```sh
ROUTERFORGE_MONITORING_EXPECTED_VERSION='0.8.0' \
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

`blocked` override не допускается. См. [ARCHITECTURES.md](ARCHITECTURES.md).

## Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Pre-release package versions используют `~`, чтобы сортироваться ниже соответствующего Stable.

## Stable ↔ Beta

При смене channel:
1. запустите bootstrap целевого channel;
2. форсируйте проверку в App Center;
3. приведите installed RouterForge packages к versions целевого release-index.

Stable → `routerforge-stable` + Registry `main`.
Beta → `routerforge-beta` + Registry `dev`.

## Management

`routerforge-admin` добавляет Processes/Services, File Manager, Maintenance,
Entware Terminal и Keenetic NDM Console. Keenetic mode запускает фиксированный backend `ndmc`.

## Core service

```sh
/opt/etc/init.d/S90routerforge restart
tail -f /opt/var/log/routerforge.log
```

## Удаление

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```
