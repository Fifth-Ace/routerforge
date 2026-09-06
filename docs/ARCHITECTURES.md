# RouterForge architectures

RouterForge packages are Entware IPKs. Package architecture и Go target связаны, но это
разные строки.

## Release status

| RouterForge target | Go target | Entware feed family | Release status |
| --- | --- | --- | --- |
| `aarch64-3.10` | `GOARCH=arm64` | `aarch64-k3.10` | **Stable / Beta supported** |
| `mips-3.4` | `GOARCH=mips`, `GOMIPS=softfloat` | `mipssf-k3.4` | **Beta experimental preview only** |
| `mipsel-3.4` | `GOARCH=mipsle`, `GOMIPS=softfloat` | `mipselsf-k3.4` | **Beta experimental preview only** |

AArch64 — основной аппаратно проверяемый target.

MIPS/MIPSel уже cross-build'ятся, проходят runtime smoke под QEMU и могут публиковаться в
Beta как experimental preview. Это **не аппаратная валидация**: у проекта нет физического
MIPS/MIPSel test router, поэтому Stable для этих target остаётся заблокированным.

## Universal installer

Один bootstrap URL определяет target через:

```text
opkg print-architecture
```

Правила выбора:

- однозначный target выбирается автоматически;
- конфликт/неизвестная архитектура fail-closed;
- при неоднозначности интерактивный TTY может показать выбор;
- без TTY target можно задать через `ROUTERFORGE_TARGET`.

Для MIPS/MIPSel Beta требуется явное подтверждение experimental режима:

```sh
ROUTERFORGE_MIPS_PREVIEW=1
```

Если runtime compatibility probe возвращает `degraded`, продолжение требует отдельного
явного согласия:

```sh
ROUTERFORGE_MIPS_ALLOW_DEGRADED=1
```

Stable bootstrap MIPS/MIPSel не открывает.

## Build target selection

Default:

```sh
./scripts/build-opkg.sh 0.0.0-dev
```

Explicit AArch64:

```sh
ROUTERFORGE_TARGET=aarch64-3.10 ./scripts/build-opkg.sh 0.0.0-dev
```

MIPS:

```sh
ROUTERFORGE_TARGET=mips-3.4 ./scripts/build-opkg.sh 0.0.0-dev
ROUTERFORGE_TARGET=mips-3.4 ./scripts/build-module-opkg.sh dns 0.0.0-dev
```

MIPSel:

```sh
ROUTERFORGE_TARGET=mipsel-3.4 ./scripts/build-opkg.sh 0.0.0-dev
ROUTERFORGE_TARGET=mipsel-3.4 ./scripts/build-module-opkg.sh dns 0.0.0-dev
```

Target mapping lives in `scripts/target-env.sh`.

## What CI proves

CI currently checks, depending on scope/full-release mode:

- Core/Control/DNS/monitoring cross-build;
- target-specific IPK architecture;
- planned MIPS/MIPSel builds;
- MIPS runtime under QEMU;
- universal bootstrap quarantine/compatibility rules.

CI/QEMU do **not** prove:

- real KeeneticOS startup on MIPS hardware;
- Module ABI Unix-socket behavior on that hardware generation;
- DNS capture/control against real MIPS Keenetic firmware;
- real install/update/remove/rollback behavior.

## Before Stable MIPS/MIPSel

Stable support requires physical validation for each target:

1. Core starts and `/api/health` is healthy.
2. Module ABI Unix sockets work.
3. DNS discovery/capture/control is verified.
4. Центр приложений selects the correct target asset.
5. Universal bootstrap detects the local architecture correctly.
6. Install, update, rollback and uninstall are exercised.
7. Resource footprint is acceptable on representative low-memory hardware.

Until then MIPS/MipSel remain Beta experimental preview only.
