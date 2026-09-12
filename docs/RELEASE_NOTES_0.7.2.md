# RouterForge 0.7.2 — DNS hotfix

Дата: **12 сентября 2026**

Stable 0.7.2 — точечный релиз DNS-модуля поверх Stable 0.7.1.

## Версии пакетов

| Компонент | Версия |
| --- | ---: |
| `routerforge-core` | `0.7.1` |
| `routerforge-dns` | **`0.7.2`** |
| `routerforge-admin` | `0.7.1` |
| `routerforge-monitoring` | `0.7.1` |
| `routerforge-profiling` | `0.7.1` |

`routerforge-dns 0.7.2` сохраняет `min_core_version=0.7.1`.

## Что исправлено

- Устранено загрязнение DNS telemetry при повторном использовании NDMS/Keenetic одного локального secure-порта другим resolver identity.
- Добавлен curated baseline catalog публичных/частных DoT/DoH presets, отключённых по умолчанию.
- Semantic endpoint dedup не показывает лишний virtual preset рядом с уже настроенным эквивалентным resolver.
- Resolver UI разделён на `Keenetic / свои`, `Публичные DNS`, `Частные DNS`.
- Отключённые ручные известные DoT/DoH-провайдеры классифицируются в provider sections, оставаясь редактируемыми manual entries.
- AstraCat распознаётся по `dns.astracat.ru` и `dns.astracat.network`; Comss.one классифицируется как Private.
- Сворачивание секций получило прямые Svelte state bindings: мгновенный отклик + плавная анимация.
- На вкладке Resolvers убран тяжёлый фоновый full refresh каждые 5 секунд; ручное и post-mutation обновление сохранены.
- Disabled preset status приведён к общей визуальной семантике.

## Hardware evidence

DoT port-reuse isolation проверена на реальном Keenetic без перезапуска `routerforge-dns`: при возврате исходного resolver на переиспользованный порт cumulative/history начинались с нуля.

DoH использует тот же общий semantic identity механизм, но отдельный DoH-specific hardware port-reuse сценарий в этом релизе не заявляется.

## Release scope

Этот hotfix не поднимает версии Core/Admin/Monitoring/Profiling и не меняет Beta channel. Immutable `routerforge-v0.7.1` остаётся нетронутым; Stable 0.7.2 публикуется отдельным immutable snapshot.
