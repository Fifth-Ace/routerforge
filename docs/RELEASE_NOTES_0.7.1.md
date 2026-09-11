# RouterForge 0.7.1 — полный патчноут

Дата релиза: **11 сентября 2026**

Этот документ описывает изменения **Stable 0.7.1 относительно Stable 0.6.1**.

0.7.1 — большой функциональный релиз. За один цикл RouterForge получил полноценный Management v2, файловый менеджер, два интерактивных терминала, объединённый Monitoring, усиленный DNS runtime, более зрелый Центр приложений, безопасное обнаружение локальных Web UI и более строгую цепочку сборки и публикации релизов.

> [!NOTE]
> RouterForge — независимый некоммерческий проект сообщества и не является официальным продуктом Keenetic, Netcraze, Entware или других упомянутых компаний/проектов.

## Базовая точка сравнения: Stable 0.6.1

Immutable release `routerforge-v0.6.1` содержал следующую основную package topology:

| Компонент | Версия в Stable 0.6.1 |
| --- | ---: |
| `routerforge-core` | `0.6.1` |
| `routerforge-dns` | `0.4.20` |
| `routerforge-admin` | `0.3.1` |
| `routerforge-system` | `0.3.1` |
| `routerforge-thermal` | `0.3.2` |
| `routerforge-storage` | `0.3.2` |
| `routerforge-network` | `0.3.3` |
| `routerforge-profiling` | `0.3.0` |

В Stable 0.7.1 релизный train сведён к пяти пакетам:

```text
routerforge-core
routerforge-dns
routerforge-admin
routerforge-monitoring
routerforge-profiling
```

Для этого Stable-релиза все пять пакетов имеют version `0.7.1`.

> [!TIP]
> Это выравнивание версий относится именно к релизному train 0.7.1. Архитектура RouterForge по-прежнему допускает независимое версионирование Core и модулей в отдельных component releases.

## Главное в 0.7.1

- Management стал полноценной пользовательской областью, а не только read-only helper.
- Появился File Manager с Commander/Explorer, редактором, деревом, томами и guarded mutations.
- Появились Entware Terminal и Keenetic NDM Console.
- Четыре Monitoring-пакета объединены в один `routerforge-monitoring`.
- DNS получил дополнительные hardening/performance changes и компактные event rings.
- App Center стал полноценной общей точкой управления RouterForge/Integrations/Entware.
- Появилось безопасное обнаружение локальных Web UI без слепого LAN-сканирования.
- Усилены auth/runtime/network boundaries.
- Stable promotion теперь привязан к exact SHA и заранее проверенному promotion artifact.
- ARM64 проходит физические проверки на двух моделях: KN-3811 и KN-1812.
- MIPSel получил частичную физическую проверку на KN-1010.

# Сильные стороны RouterForge

## Нативная интеграция с Keenetic

RouterForge не ограничивается generic Linux-информацией. Он умеет работать с KeeneticOS/NDMS и использует:

- `ndmc`;
- RCI;
- DNS configuration/runtime;
- policy routing;
- сведения об интерфейсах и маршрутах;
- Entware services и OPKG;
- системные данные устройства.

За счёт этого RouterForge может показывать и изменять Keenetic-специфичные сущности, которые универсальная Linux-панель обычно не понимает.

## Модульность с единым Web UI

Core остаётся единой пользовательской точкой входа на `:2233`.

DNS, Management и Monitoring:
- устанавливаются отдельно;
- имеют собственные runtime-модули;
- общаются с Core через root-owned Unix sockets;
- не требуют отдельного внешнего Web-порта.

## Ограниченные контракты для привилегированных действий

RouterForge не превращает каждую кнопку в произвольную root-shell строку.

Привилегированные операции используют:
- проверку root-session;
- same-origin boundary;
- whitelists/фиксированные действия;
- точную валидацию цели;
- canonical path containment;
- Core-injected internal markers;
- rollback/readback там, где операция может изменить системную конфигурацию.

## Оптимизация под постоянно работающий роутер

0.7.1 продолжает курс на ограниченное потребление ресурсов:
- bounded caches;
- bounded history;
- bounded auth/job state;
- последовательный polling;
- request timeouts;
- failure backoff;
- compact DNS event storage;
- production executable compression;
- отсутствие Node.js в router runtime.

## Центр приложений как единая точка управления

Один интерфейс объединяет:
- официальные RouterForge packages;
- Integrations;
- Entware;
- Installed;
- Updates.

Пользователю не требуется вручную собирать состояние из нескольких независимых package/UI surfaces.

## Проверяемый supply/release path

Release-index содержит exact versions/assets/URLs/SHA256.

Stable:
- строится для трёх targets;
- проходит CI/QEMU и package validation;
- создаёт отдельный promotion artifact;
- продвигается только тем же exact SHA;
- публикует rolling Stable и versioned snapshot.

---

# 1. Management v2

В 0.6.1 `routerforge-admin` был существенно более ранним Management/Control helper. В 0.7.1 он превращён в полноценную пользовательскую capability.

## Processes

Добавлены защищённые действия над процессами:

```text
TERM
HUP
INT
KILL
```

Backend проверяет конкретную цель. Browser не передаёт произвольную команду оболочки.

## Entware Services

Добавлены действия:

```text
start
stop
restart
```

Они применяются к разрешённым init scripts и требуют live root-session.

## Security contract

Для Management mutations используются:
- live Entware-root session;
- same-origin checks;
- confirmation/whitelist;
- Core-injected internal Admin marker.

Это отделяет read-only monitoring от действительно привилегированных действий.

# 2. File Manager

В 0.7.1 появился полноценный File Manager.

## Интерфейс

Реализованы:
- **Commander**;
- **Explorer**;
- дерево каталогов;
- список доступных томов;
- навигация;
- UTF-8 editor;
- create directory;
- rename/move;
- download;
- non-recursive delete;
- properties;
- visual `chmod`;
- responsive/compact toolbar;
- leaf-aware tree navigation.

## Filesystem boundary

Разрешённые корни для mutations:

```text
/opt
/tmp
```

Защита включает:
- запрет явного `..`;
- canonical path resolution;
- symlink containment;
- повторную проверку цели перед destructive operation;
- size/mtime preconditions для edit;
- atomic same-filesystem replacement.

## Что намеренно не обещается в Stable 0.7.1

Остаются за пределами заявленного готового набора:
- полноценное recursive directory copy/delete;
- arbitrary binary upload;
- archive/extract;
- `chown`;
- произвольный write-доступ по всей системной `/`.

# 3. Entware Terminal

Добавлен полноценный browser terminal для Entware:

- WebSocket;
- PTY;
- фиксированный `/opt/bin/sh -il`;
- интерактивный resize;
- reconnect/switch semantics.

Terminal использует существующую RouterForge auth boundary.

# 4. Keenetic NDM Console

В Management добавлена отдельная Keenetic NDM Console.

Backend:
- server-side resolve'ит `ndmc`;
- запускает фиксированный `ndmc`;
- не принимает executable или произвольный argv из browser request;
- использует PTY/WebSocket transport.

Browser выбирает только:

```text
mode=entware
mode=keenetic
```

Аппаратно проверены:
- Entware PTY;
- Keenetic PTY;
- `ndmc show version`;
- повторное переключение Entware ↔ Keenetic.

# 5. Monitoring: миграция 4 → 1

Stable 0.6.1 публиковал четыре отдельных monitoring package:

```text
routerforge-system
routerforge-thermal
routerforge-storage
routerforge-network
```

Stable 0.7.1 использует единый:

```text
routerforge-monitoring
```

Он обслуживает:
- System;
- Thermal;
- Storage;
- Network.

## Зачем объединение

- меньше отдельных runtime-процессов;
- один package lifecycle;
- один UI;
- меньше дублирования;
- проще установка и обновление;
- единая точка дальнейшей оптимизации.

## Миграция старых установок

`routerforge-monitoring` использует `Provides/Conflicts/Replaces`.

Post-install logic:
- останавливает старые split services;
- удаляет stale sockets;
- запускает consolidated runtime.

Сохранены intentional compatibility sockets/API для System/Thermal/Storage/Network, поэтому наличие legacy-named socket после migration само по себе **не означает**, что старый процесс всё ещё работает.

На ARM64 проверены:
- package database cleanup;
- отсутствие старых binaries/init scripts;
- consolidated process;
- expected sockets;
- reboot;
- autostart.

# 6. DNS: безопасные изменения

`routerforge-dns` остаётся отдельным Module ABI v1 runtime.

Для resolver mutations используется цепочка:

```text
snapshot
  -> validation
  -> mutation
  -> save
  -> readback
  -> semantic compare
  -> verified rollback при mismatch
```

Поддерживаются:
- plain DNS;
- DoT;
- DoH;
- Add/Edit/Delete;
- временный Disable/Enable;
- logical multi-domain grouping;
- read-only защита DHCP/service entries.

Такая модель важнее обычного «команда вернула 0»: RouterForge проверяет, что Keenetic действительно сохранил ожидаемое состояние.

# 7. DNS: наблюдаемость и диагностика

В 0.7.1 сохранена и расширена DNS observability:

- запросы и история;
- clients;
- LAN/Wi-Fi attribution;
- domains/QTYPEs;
- upstream/fallback;
- timeout/error;
- latency;
- quality windows;
- error bursts;
- runtime/system health;
- policy-routing-aware upstream diagnostics;
- локальный/cache path.

# 8. DNS: производительность и hardening

Добавлены/усилены:

- kernel BPF filtering перед userspace processing;
- bounded TTL/last-good caches;
- failure backoff;
- bounded frontend read timeouts;
- compact internal event representation;
- отсутствие постоянной event-записи на flash в горячем пути.

Логическая глубина DNS event history сохранена на **10 000** событий.

Это структурная оптимизация памяти и allocation pressure. Релиз не заявляет неподтверждённую «магическую» цифру экономии RSS.

# 9. Центр приложений

App Center в 0.7.1 объединяет:

```text
RouterForge
Integrations
Entware
Installed
Updates
```

## Lifecycle jobs

Для package actions используются:
- preflight;
- dependencies;
- download/installed sizes;
- guarded async jobs;
- global package-manager lock;
- SSE output;
- timeout;
- cancel;
- post-action refresh;
- installed-version verification;
- bounded completed-job history.

## Bulk update

При массовом обновлении:
1. обновляются modules;
2. Core идёт последним.

Это снижает риск оборвать оставшиеся операции перезапуском Core.

# 10. Generic Runtime Web UI Discovery

В 0.7.1 local Web UI discovery строится по цепочке:

```text
LISTEN socket
  -> PID/process
  -> package
  -> bounded local HTTP/HTTPS probe
```

RouterForge **не сканирует вслепую всю LAN/subnet**.

Используются fail-closed boundaries для:
- redirects;
- X-Frame-Options;
- CSP;
- SSRF;
- infrastructure/platform services;
- duplicate known integrations.

# 11. Core runtime

## Один внешний Web listener

Core остаётся единственным пользовательским RouterForge LAN listener:

```text
:2233
```

DNS/Admin/Monitoring работают через root-owned Unix sockets.

Profiling, если установлен, остаётся loopback-only:

```text
127.0.0.1:6061
```

## HTTP hardening

Добавлены/уточнены:
- read timeout;
- header timeout;
- idle timeout;
- header-size limit.

Глобальный `WriteTimeout` намеренно не включён, чтобы не ломать долгоживущие SSE streams.

## Module proxy

Mutation request bodies ограничиваются до Unix-socket forwarding, чтобы oversized request не превращался в неограниченную нагрузку на privileged module.

# 12. Frontend и polling

В Core/Admin/Monitoring:
- periodic reads переведены на serial scheduling;
- устранено накопление overlapping async `setInterval`;
- read requests используют bounded AbortController timeouts;
- slow request не должен бесконечно накапливать следующие.

Для DNS mutation semantics timeout применяется осторожно: потенциально уже выполненная системная мутация не должна ложно объявляться «не выполненной» только из-за frontend timeout.

# 13. Thermal/Storage/ndmc hot paths

Для дорогих или часто вызываемых collectors используются:
- TTL caches;
- last-good value;
- singleflight;
- stale-while-revalidate;
- failure backoff.

Storage `Statfs` разделён по platform-specific implementation для лучшей portability/cross-build дисциплины.

# 14. Auth и bounded state

В 0.7.1:
- failed-login clients имеют ограниченный размер;
- stale entries очищаются;
- tracking rate-limited;
- session token остаётся in-memory;
- password RouterForge не сохраняет;
- cookie использует `HttpOnly` + `SameSite=Strict`.

App Center completed-job history также bounded.

# 15. UI и визуальная согласованность

Management, Maintenance, File Properties и Terminal приведены к общей semantic RouterForge theme-модели.

Исправлялись:
- поверхности;
- borders;
- muted/text states;
- toolbar density;
- responsive layout;
- terminal presentation;
- active/hover/focus states.

Цель — чтобы standalone modules визуально воспринимались как части одной системы.

# 16. Release pipeline: Dev, Beta и Stable разделены

## Dev

Push в `dev` предназначен для development train и rolling Dev.

## Beta

Beta публикуется только explicit FULL RELEASE на проверенном SHA.

Beta version train имеет fail-closed consistency guard между:
- `release_version`;
- component versions;
- `min_core_version`.

Guard появился после обнаружения реального класса ошибки, когда versioned Beta tag мог не совпасть с package train.

## Stable

Stable promotion:
1. строится на exact Dev SHA;
2. проходит полный release validation;
3. формирует `routerforge-stable-promotion`;
4. тот же exact SHA продвигается в `main`;
5. main скачивает именно этот promotion artifact;
6. artifact повторно проверяется;
7. после этого обновляется rolling Stable;
8. создаётся versioned release.

# 17. Stable 0.7.1 multiarch release

Stable 0.7.1 публикует:

- 5 packages;
- 3 targets;
- **15 IPK**;
- 3 target release-index;
- SHA256SUMS;
- universal bootstrap;
- target-specific bootstraps.

Targets:

```text
aarch64-3.10
mips-3.4
mipsel-3.4
```

ARM64 — production/hardware-validated target.

MIPSel и MIPS остаются experimental.

# 18. Аппаратная матрица разработки и релиза

## Keenetic Hopper KN-3811 — Dev + Beta

KN-3811 используется как основная ARM64-площадка для:
- текущей разработки;
- функциональных hardware checks;
- rolling Dev;
- Beta validation.

## Keenetic Ultra KN-1812 — Beta + Stable

KN-1812 используется для:
- дополнительной/финальной Beta validation;
- проверки релизной версии;
- Stable hardware checks.

Именно на Ultra, в частности, проверялись browser PTY для Entware/Keenetic и переключение режимов терминала.

## Keenetic Giga KN-1010 — MIPSel experimental

На KN-1010 есть частичная физическая проверка MIPSel:
- fresh install;
- базовая нормальная работа.

Полная MIPSel matrix — upgrade/rollback/uninstall, полный DNS/Management coverage и resource stress — пока не заявляется закрытой.

## MIPS big-endian

Physical hardware validation отсутствует.

Cross-build/QEMU/runtime probes важны, но не приравниваются к реальному hardware PASS.

# 19. Что изменилось для пользователя 0.6.1 → 0.7.1

Наиболее заметные изменения:

- вместо раннего Control — полноценный Management v2;
- появился File Manager;
- появился browser Entware Terminal;
- появилась Keenetic NDM Console;
- четыре monitoring package заменены одним;
- App Center стал шире и надёжнее;
- DNS runtime получил дополнительные performance/hardening changes;
- multiarch/release pipeline стал строже;
- Beta и Stable проходят более формализованную hardware validation.

# 20. Обновление с Stable 0.6.1

Рекомендуемый путь:

1. Запустить актуальный Stable bootstrap.
2. Открыть **Центр приложений**.
3. Нажать **«Проверить обновления»**.
4. Обновить установленные RouterForge packages.
5. Проверить migration split monitoring → `routerforge-monitoring`.
6. Проверить health Core и нужных modules.

Не рекомендуется вручную удалять старые package-owned monitoring files, чтобы «починить» migration: штатный lifecycle должен сам провести замену.

# 21. Что сознательно остаётся за пределами обещаний 0.7.1

Stable 0.7.1 не объявляет завершёнными:

- recursive directory copy/delete во всех File Manager сценариях;
- arbitrary binary upload;
- archive/extract;
- `chown`;
- MIPS big-endian hardware validation;
- полную MIPSel hardware matrix;
- неподтверждённые RSS-рекорды от DNS compact rings.

# 22. Итог

По сравнению с Stable 0.6.1 RouterForge 0.7.1 заметно меняет сам класс продукта:

- Management превращается из вспомогательного read-only слоя в реальный инструмент управления;
- Monitoring становится компактнее по topology;
- DNS получает более строгие safety/performance boundaries;
- App Center становится центральной точкой управления пакетами и локальными приложениями;
- терминалы дают доступ и к Entware, и к Keenetic NDM в одном UI;
- релизная цепочка становится exact-SHA и fail-closed;
- hardware validation распределена между KN-3811 и KN-1812, а MIPSel получает отдельную физическую проверку.

Stable 0.7.1 — это уже не просто набор диагностических экранов, а модульная платформа управления и наблюдения для Keenetic/Netcraze с Entware.
