# RouterForge 0.9.0 — полный патчноут

Дата подготовки: 2026-09-16
База сравнения: **Stable 0.8.0** (`89c172f33e464d058f795aeb993f51ef14e56209`)
Release-prep база: `9e2e2f43d11cb5dccfabc857065b3af18687f021`
Диапазон: **70 коммитов** после Stable 0.8.0.

RouterForge использует независимое версионирование компонентов. Название платформенного
релиза — **0.9.0**, но версия повышается только у компонентов, чей продуктовый код
действительно изменился.

## Версии компонентов

| Компонент | Stable 0.8.0 | Stable 0.9.0 | Причина |
| --- | ---: | ---: | --- |
| `routerforge-core` | 0.8.0 | **0.9.0** | Registry/Manifest Platform, App Center Jobs/History, trust/security и channel-transition изменения |
| `routerforge-dns` | 0.8.0 | **0.8.1** | rolling health, explainability, settings/readiness/UI fixes без смены основной DNS-архитектуры |
| `routerforge-admin` | 0.8.0 | **0.8.1** | guarded actions/auth semantics и Shared Safety adoption без нового Management ABI |
| `routerforge-monitoring` | 0.7.1 | **0.8.0** | существенная переработка thermal discovery/runtime на Keenetic |
| `routerforge-network-tools` | 0.8.0 | **0.9.0** | крупное расширение Route Inspector, runtime и UI |
| `routerforge-profiling` | 0.7.1 | **0.7.1** | продуктовый код не менялся |

`DNS`, `Admin`, `Monitoring` и `Network Tools` в этом Stable train требуют Core **0.9.0**.
`Profiling` не менялся и сохраняет `min_core_version=0.7.1`.

## Core / App Center / Registry

- Завершена **PHASE 13 Registry / Manifest Platform**.
- Bundled manifest registry стал источником истины для интеграций; старый Go fallback удалён.
- Manifest schema и registry builder расширены и прикрыты постоянными CI-contract checks.
- Local/private sources получили namespaced identity и не могут тихо подменять public ID.
- Добавление стороннего источника стало двухшаговым: read-only preview → явный `ADD_SOURCE`
  с exact SHA-256 preview; изменившийся между шагами источник блокируется.
- Detection сторонних sources остаётся пассивным; trust-state drift не выдаёт arbitrary
  lifecycle/web-probe authority.
- Добавлены manifests для AWG Manager, AdGuardHome Keenetic, NFQWS, XKeen, KVAS,
  Keen PBR, sing-box UI и других поддерживаемых community integrations.
- Installed user software может открывать локальный Web UI при валидной локальной metadata
  и успешном bounded probe, не получая при этом права на произвольный install/lifecycle.
- App Center получил **Jobs + History**: до 100 backend records, progressive UI, фильтры,
  persisted details и полный технический лог.
- Active conflict adoption позволяет UI привязаться к уже выполняющемуся job вместо
  создания дубликата.
- History переведён в workspace-sheet: общий header/sidebar остаются видимыми.
- Ошибки jobs включаются в единый technical log вместо отдельного огромного banner.

## Обновления и channel transitions

- Batch update выполняет обычные модули до Core; **Core обновляется последним**.
- После восстановления Core shell немедленно перезагружается.
- Catalog reads получили retry/recovery semantics.
- Пустые и повреждённые JSON-ответы во время рестарта Core теперь различаются явно вместо
  `Unexpected end of JSON input`.
- Module HTML/UI документы отдаются `no-store`; fingerprinted assets остаются immutable.
- Module iframe получает revision key, поэтому смена Dev/Beta больше не должна оставлять
  старую страницу/старые assets в браузере.

## Private Forgejo

- Завершена **PHASE 14 Private Forgejo**.
- Зафиксирована модель authority: **GitHub primary, Forgejo hot backup**.
- Нормальное направление replication: GitHub → Forgejo.
- Forgejo → GitHub никогда не выполняется автоматически.
- `scripts/routerforge-failover.ps1` поддерживает STATUS, guarded PROMOTE_FORGEJO и
  RESTORE_GITHUB с fail-closed parity checks.
- Promotion требует явного подтверждения, что внешняя autosync-задача остановлена.
- Restore требует exact parity; divergence переводит процесс в manual reconciliation.
- Destructive hot-backup restore drill не заявляется выполненным там, где серверный
  autosync нельзя безопасно приостановить.

## Shared Safety Engines

- Завершена **PHASE 15 Shared Safety Engines**.
- Добавлены общие `internal/safety` primitives:
  - canonical/path validation;
  - atomic file/config write;
  - atomic swap/rollback;
  - bounded command runner.
- Management file/config/maintenance paths переведены на общие safety primitives там,
  где это не меняет пользовательский контракт.
- Сохранены намеренные low-level exceptions: interactive Terminal/PTTY, App Center
  streaming execution, File Manager rename через `os.Rename`, log/history append/rotation
  и внутренний `exec.CommandContext` самого shared runner.
- CI получил отдельный `Verify shared safety contract`.

## DNS 0.8.1

- Health-модель переведена с lifetime counters на **rolling window**.
- Исторические ошибки больше не держат resolver в вечном `DEGRADED`.
- Базовый профиль: **«Нестабильный интернет»**.
- Базовые пороги:
  - health window 5 минут;
  - DEGRADED по ошибкам только при `>=10` errors и `>=2%`;
  - slow DNS при p95 `>=1500 ms` и `>=20` responses;
  - DOWN при `>=5` attempts, `0` successes в коротком 60-second окне;
  - historical incident marker 60 минут.
- `NXDOMAIN` считается информационным событием и не ухудшает здоровье resolver.
- Overview показывает число **текущих проблемных resolver**, а не накопительный счётчик.
- `DEGRADED` больше не одновременно считается healthy.
- Добавлена resolver-specific problem attribution и понятные `?`-подсказки для status,
  p95, quality, current problems и secure runtime health.
- DNS health settings находятся в общих RouterForge Settings и показываются только когда
  DNS установлен.
- Package readiness больше не зависит от Core health: используется DNS process + Unix socket.
- Resolver details drawer и card presentation приведены к общему UI contract.

## Management / Admin 0.8.1

- Исправлена авторизация mutation path:
  - при выключенной RouterForge auth разрешены same-origin Management actions;
  - при включённой auth требуется authenticated root session;
  - cross-origin mutations по-прежнему блокируются.
- Browser не может подделать internal authorization marker; canonical marker выставляет Core.
- Exact target confirmations, body limits, PID/service checks и protected RouterForge
  processes сохранены.
- Processes сохраняют ограниченный набор `TERM/HUP/INT/KILL`.
- Services сохраняют только `start/stop/restart`.
- File Manager path restrictions и destructive confirmations не ослаблены.
- Старый дублирующий Management network diagnostics endpoint/tab удалён: active diagnostics
  принадлежат только `routerforge-network-tools`.

## Monitoring 0.8.0

- Thermal collector переработан под реальные Keenetic/NDMS/sysfs источники.
- Добавлена нормализация и dedup сенсоров, более дружелюбные имена и fallback logic.
- Уточнено отображение температуры на KN-3811/NDMS.
- Сохранена consolidated topology System/Thermal/Storage/Network в одном
  `routerforge-monitoring`.
- Legacy split package names/sockets остаются только как migration compatibility surface.

## Network Tools 0.9.0

- `routerforge-network-tools` закреплён как единственный first-class модуль активной
  сетевой диагностики.
- Существенно расширены runtime и UI.
- Добавлен/расширен **Route Inspector** с отдельным runtime implementation и tests.
- Network Doctor, routes, flows и active probes приведены к общей структуре UI.
- Старый Management network diagnostics duplicate и dead split `modules/network` source
  удалены.
- Monitoring продолжает владеть только read-only network telemetry.

## UI / UX

- Зафиксирован общий RouterForge UI Contract для Management, Monitoring, DNS,
  App Center и Network Tools.
- Унифицированы tab strip, module headers, panel/control/table rhythm и keyboard focus.
- App Center history drawer приведён к геометрии общего workspace.
- Убрана отдельная Private Forgejo tile из App Center; sidebar сохраняет нужные внешние
  ссылки.
- Исправлены stale module UI после смены channel.
- DNS и thermal presentation получили отдельный polish без повторного broad redesign.

## CI / release tooling

- Добавлены постоянные gates:
  - public repository hygiene;
  - documentation currency;
  - UI contract;
  - marketplace contract;
  - legacy fallback removal;
  - private registry completion;
  - Forgejo failover contract;
  - shared safety contract.
- Network Tools workflow расширен на consolidation/runtime invariants.
- FULL RELEASE продолжает проверять multiarch build, IPK containers/payloads, MIPS/MIPSel
  QEMU/runtime, bootstraps, release indexes и package integrity.
- Stable promotion остаётся exact-SHA: сначала FULL RELEASE `publish_beta=false`,
  затем тот же SHA fast-forward в `main`, после чего main workflow проверяет и публикует
  exact `routerforge-stable-promotion` artifact.

## Архитектуры

- `aarch64-3.10` — production, полностью hardware-validated.
- `mipsel-3.4` — experimental, частичная physical validation на KN-1010.
- `mips-3.4` — experimental, без физической аппаратной проверки.
- Stable topology: 6 компонентов × 3 target = 18 IPK.

## Совместимость и обновление

- Fresh install по-прежнему начинается с Core; остальные возможности выбираются через
  App Center.
- External RouterForge UI port остаётся `:2233`.
- Runtime modules используют root-owned Unix sockets.
- Обновление с Stable 0.8.0 должно выполняться через Stable bootstrap/App Center, а не
  ручной заменой package-owned файлов.
- После promotion рекомендуется обновить Core первым bootstrap'ом канала, затем дать
  App Center привести остальные установленные RouterForge packages к Stable index.

## Что намеренно не менялось

- Profiling 0.7.1 остаётся без изменений (`routerforge-profiling`); косметический bump не выполняется.
- Stable/Beta/Dev остаются отдельными rolling channels.
- GitHub остаётся primary release/CI authority.
- Maintenance остаётся внутри Management, а не отдельным top-level module.
- Network telemetry остаётся в Monitoring; active diagnostics — в Network Tools.
