#!/bin/sh
set -eu

VERSION="${1:-0.7.0-phase7b}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"

DIST="$ROOT/dist"
ARCH="$RF_OPKG_ARCH"
PACKAGE="routerforge-monitoring"
WORK="$DIST/${PACKAGE}-prototype-work"
PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
OUTPUT="$DIST/$PKGFILE"

rm -rf "$WORK"
mkdir -p \
    "$WORK/data/opt/bin" \
    "$WORK/data/opt/etc/init.d" \
    "$WORK/data/opt/etc/routerforge" \
    "$WORK/data/opt/share/licenses/$PACKAGE" \
    "$WORK/control"

(
    cd "$ROOT"
    routerforge_go build -trimpath \
        -ldflags="-s -w -X main.version=$VERSION" \
        -o "$WORK/data/opt/bin/routerforge-monitoring" \
        ./modules/monitoring-runtime
)
chmod 0755 "$WORK/data/opt/bin/routerforge-monitoring"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/routerforge-monitoring"

cp "$ROOT/modules/monitoring-runtime/packaging/S92routerforge-monitoring" \
   "$WORK/data/opt/etc/init.d/S92routerforge-monitoring"
cp "$ROOT/modules/monitoring-runtime/packaging/standalone-postinst" \
   "$WORK/control/postinst"
cp "$ROOT/modules/monitoring-runtime/packaging/standalone-prerm" \
   "$WORK/control/prerm"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
chmod 0755 \
    "$WORK/data/opt/etc/init.d/S92routerforge-monitoring" \
    "$WORK/control/postinst" \
    "$WORK/control/prerm"
chmod 0644 "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

cat > "$WORK/control/control" <<CONTROL
Package: routerforge-monitoring
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: routerforge-core
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: Standalone RouterForge monitoring runtime serving system, thermal, storage and network Module ABI v1 sockets from one process.
CONTROL

printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
rm -f "$OUTPUT"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUTPUT" ./debian-binary ./control.tar.gz ./data.tar.gz)

BYTES="$(wc -c < "$OUTPUT" | tr -d ' ')"
SHA256="$(sha256sum "$OUTPUT" | awk '{print $1}')"

printf 'PACKAGE:    routerforge-monitoring\n'
printf 'PROCESS:    routerforge-monitoring\n'
printf 'SERVICE:    S92routerforge-monitoring\n'
printf 'MODE:       all\n'
printf 'SOCKETS:    routerforge-system.sock routerforge-thermal.sock routerforge-storage.sock routerforge-network.sock\n'
printf 'FILE:       %s\n' "$OUTPUT"
printf 'VERSION:    %s\n' "$VERSION"
printf 'TARGET:     %s\n' "$TARGET"
printf 'ARCH:       %s\n' "$ARCH"
printf 'BYTES:      %s\n' "$BYTES"
printf 'SHA256:     %s\n' "$SHA256"
printf 'NOTE:       prototype package; no channel/release cutover yet\n'
