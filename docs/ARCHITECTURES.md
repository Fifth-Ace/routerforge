# RouterForge architectures

RouterForge packages are Entware IPKs. Package architecture и Go target связаны, но это
разные строки.

## Release status

| RouterForge target | Go target | Entware feed family | Release status |
| --- | --- | --- | --- |
| `aarch64-3.10` | `GOARCH=arm64` | `aarch64-k3.10` | **Stable / Beta — hardware validated** |
| `mips-3.4` | `GOARCH=mips`, `GOMIPS=softfloat` | `mipssf-k3.4` | **Stable / Beta experimental preview — NOT hardware tested** |
| `mipsel-3.4` | `GOARCH=mipsle`, `GOMIPS=softfloat` | `mipselsf-k3.4` | **Stable / Beta experimental preview — partial hardware validation on KN-1010** |

AArch64 — основной полностью аппаратно проверенный target.

MIPS/MIPSel cross-build'ятся, проходят runtime smoke под QEMU и runtime compatibility probe.
Начиная со Stable 0.6 они публикуются и в Stable release как **experimental preview**.

Для MIPSel получена первая физическая проверка на Keenetic Giga KN-1010: fresh installation
и базовая штатная работа RouterForge подтверждены на реальном устройстве. Это частичная
валидация: upgrade/rollback/uninstall, полный DNS/Module ABI сценарий и resource footprint
на MIPSel ещё не закрыты. MIPS big-endian физически не проверен.

Публикация пакета не означает production support. MIPS/MipSel installer сохраняет explicit
experimental opt-in и fail-closed probe: `blocked` нельзя обойти, а `degraded` требует
отдельного явного подтверждения.

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

Для MIPS/MIPSel Stable/Beta требуется явное подтверждение experimental режима:

```sh
ROUTERFORGE_MIPS_PREVIEW=1
```

Если runtime compatibility probe возвращает `degraded`, продолжение требует отдельного
явного согласия:

```sh
ROUTERFORGE_MIPS_ALLOW_DEGRADED=1
```

Статус `blocked` не может быть overridden.

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
- MIPS/MipSel builds;
- MIPS runtime under QEMU;
- universal bootstrap quarantine/compatibility rules;
- Stable multiarch promotion candidate checksums and package architecture.

CI/QEMU do **not** prove:

- real KeeneticOS startup on a target that has no physical evidence;
- complete Module ABI Unix-socket behavior on each hardware generation;
- DNS capture/control against each real MIPS/MIPSel Keenetic firmware family;
- real update/remove/rollback behavior;
- acceptable resource footprint on representative low-memory MIPS/MIPSel hardware.

## Physical validation backlog for MIPS/MipSel

MIPS remains fully unvalidated on physical hardware. MIPSel has a partial KN-1010 fresh-install/basic-operation pass, but both targets remain explicitly experimental until the remaining matrix is covered:

1. Core starts and `/api/health` is healthy.
2. Module ABI Unix sockets work.
3. DNS discovery/capture/control is verified.
4. Центр приложений selects the correct target asset.
5. Universal bootstrap detects the local architecture correctly.
6. Install, update, rollback and uninstall are exercised.
7. Resource footprint is acceptable on representative low-memory hardware.

Until that matrix is complete, release notes and documentation must distinguish:

- MIPS big-endian: **not hardware tested**;
- MIPSel: **partially hardware validated on KN-1010**, still experimental;
- AArch64: primary fully hardware-validated target.
