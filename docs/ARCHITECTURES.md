# Архитектуры RouterForge

## Текущий статус

| Target | Go target | Семейство Entware | Статус |
| --- | --- | --- | --- |
| `aarch64-3.10` | `GOARCH=arm64` | `aarch64-k3.10` | **Production / полностью аппаратно проверен** |
| `mipsel-3.4` | `GOARCH=mipsle`, softfloat | `mipselsf-k3.4` | **Experimental / частичная физическая проверка KN-1010** |
| `mips-3.4` | `GOARCH=mips`, softfloat | `mipssf-k3.4` | **Experimental / физической проверки нет** |

ARM64 — основная production-архитектура RouterForge.

## Аппаратная матрица ARM64

| Устройство | Каналы | Назначение |
| --- | --- | --- |
| **Keenetic Hopper KN-3811** | **Dev + Beta** | ежедневная разработка, функциональные аппаратные проверки, rolling Dev и Beta |
| **Keenetic Ultra KN-1812** | **Beta + Stable** | дополнительная/финальная Beta validation и проверка Stable-релиза |

Beta, таким образом, проверяется на **обеих ARM64-площадках**.

## MIPSel

**Keenetic Giga KN-1010** используется для частичной физической проверки `mipsel-3.4`.

Подтверждены:
- fresh install;
- базовая нормальная работа.

Пока не заявлены полностью закрытыми:
- upgrade;
- rollback;
- uninstall;
- полный DNS/Management matrix;
- resource-stress scenarios.

Поэтому MIPSel остаётся experimental.

## MIPS big-endian

`mips-3.4` проходит:
- cross-build;
- QEMU runtime checks;
- package/index/bootstrap validation.

Физической аппаратной проверки для MIPS big-endian пока нет, поэтому target остаётся experimental.

## Универсальный installer

Target определяется через:

```sh
opkg print-architecture
```

Для non-ARM64 требуется explicit opt-in:

```sh
ROUTERFORGE_MIPS_PREVIEW=1
```

Для runtime probe со статусом `degraded`:

```sh
ROUTERFORGE_MIPS_ALLOW_DEGRADED=1
```

Статус `blocked` не override'ится.

## Что проверяет CI

CI проверяет:
- cross-build;
- архитектуру IPK;
- MIPS/MIPSel runtime под QEMU;
- compatibility/quarantine bootstrap;
- release package/index integrity;
- production compression для ARM64;
- документацию и release metadata.

CI не заменяет физическую аппаратную проверку на реальном роутере.
