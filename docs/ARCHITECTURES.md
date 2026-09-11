# RouterForge architectures

| Target | Go target | Entware family | Status |
| --- | --- | --- | --- |
| `aarch64-3.10` | `GOARCH=arm64` | `aarch64-k3.10` | **Stable / hardware validated** |
| `mipsel-3.4` | `GOARCH=mipsle`, softfloat | `mipselsf-k3.4` | **experimental / partial KN-1010 validation** |
| `mips-3.4` | `GOARCH=mips`, softfloat | `mipssf-k3.4` | **experimental / no physical validation** |

ARM64 — production target.

MIPSel имеет physical evidence для fresh install/basic operation. Upgrade/rollback/uninstall, complete DNS/Management matrix и resource-stress остаются открыты.

MIPS big-endian не имеет hardware pass. Cross-build/QEMU/runtime probe не заменяют физическую проверку.

## Universal installer
Target определяется через `opkg print-architecture`.

Non-ARM64 opt-in:

```sh
ROUTERFORGE_MIPS_PREVIEW=1
```

Для `degraded`:

```sh
ROUTERFORGE_MIPS_ALLOW_DEGRADED=1
```

`blocked` не override'ится.

## CI proves
- cross-build;
- IPK architecture;
- MIPS/MIPSel QEMU runtime;
- compatibility/quarantine bootstrap;
- release package/index integrity;
- ARM64 production compression.

CI не доказывает отсутствующую hardware validation.
