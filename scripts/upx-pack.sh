#!/bin/sh
set -eu

TARGET="${1:?RouterForge target required}"
BINARY="${2:?binary path required}"

# Emergency/debug escape hatch.
if [ "${ROUTERFORGE_UPX:-1}" = "0" ]; then
    exit 0
fi

# Release policy: UPX ultra-brute on all three supported architectures.
# DEV opts out explicitly with ROUTERFORGE_UPX=0.
# An unsupported release target must FAIL; never silently ship plain ELF.
case "$TARGET" in
    aarch64-3.10|mips-3.4|mipsel-3.4)
        ;;
    *)
        echo "RouterForge UPX unsupported target: $TARGET" >&2
        exit 2
        ;;
esac

if [ ! -f "$BINARY" ]; then
    echo "RouterForge UPX input does not exist: $BINARY" >&2
    exit 2
fi

UPX_BIN="${ROUTERFORGE_UPX_BIN:-}"

if [ -z "$UPX_BIN" ]; then
    if command -v upx >/dev/null 2>&1; then
        UPX_BIN="$(command -v upx)"
    elif command -v upx.exe >/dev/null 2>&1; then
        UPX_BIN="$(command -v upx.exe)"
    else
        echo "UPX is required for RouterForge AArch64 production builds." >&2
        echo "Install pinned UPX 5.2.1 or set ROUTERFORGE_UPX_BIN." >&2
        echo "Set ROUTERFORGE_UPX=0 only for an intentional plain diagnostic build." >&2
        exit 3
    fi
fi

"$UPX_BIN" --ultra-brute --no-progress "$BINARY"
"$UPX_BIN" -t "$BINARY"
