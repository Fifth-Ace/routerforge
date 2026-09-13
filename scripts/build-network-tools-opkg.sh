#!/bin/sh
set -eu

VERSION="${1:?version required}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"

DIST="$ROOT/dist"
ARCH="$RF_OPKG_ARCH"
PACKAGE=routerforge-network-tools
BINARY=routerforge-network-tools
SERVICE=S97routerforge-network-tools
SOCKET=/opt/var/run/routerforge-network-tools.sock
WORK="$DIST/${PACKAGE}-work"
OUTPUT="$DIST/${PACKAGE}_${VERSION}_${ARCH}.ipk"

mkdir -p "$DIST"
rm -rf "$WORK"
mkdir -p \
    "$WORK/data/opt/bin" \
    "$WORK/data/opt/etc/init.d" \
    "$WORK/data/opt/share/routerforge/modules/network-tools/ui" \
    "$WORK/data/opt/share/routerforge/modules/network-tools" \
    "$WORK/data/opt/share/licenses/$PACKAGE" \
    "$WORK/control"

(
    cd "$ROOT"
    routerforge_go build -trimpath \
        -ldflags="-s -w -X main.version=$VERSION" \
        -o "$WORK/data/opt/bin/$BINARY" ./modules/network-tools/runtime
)
chmod 0755 "$WORK/data/opt/bin/$BINARY"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/$BINARY"

cp "$ROOT/modules/network-tools/packaging/$SERVICE" "$WORK/data/opt/etc/init.d/$SERVICE"
chmod 0755 "$WORK/data/opt/etc/init.d/$SERVICE"

for file in index.html app.js runtime.js module.css; do
    cp "$ROOT/modules/network-tools/frontend/$file" \
        "$WORK/data/opt/share/routerforge/modules/network-tools/ui/$file"
done

cat > "$WORK/data/opt/share/routerforge/modules/network-tools/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "network-tools",
  "version": "$VERSION",
  "api_version": 1,
  "socket": "$SOCKET",
  "api_base": "/api/modules/network-tools",
  "ui_entry": "/api/modules/network-tools/ui/index.html",
  "mode": "read-only-diagnostics"
}
MANIFEST

cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
chmod 0644 \
    "$WORK/data/opt/share/routerforge/modules/network-tools/manifest.json" \
    "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

cat > "$WORK/control/control" <<CONTROL
Package: $PACKAGE
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: routerforge-core
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: RouterForge Network Tools: Network Doctor, traceroute, Route Inspector, Flow Explorer and bounded active probes.
CONTROL

cp "$ROOT/modules/network-tools/packaging/postinst" "$WORK/control/postinst"
cp "$ROOT/modules/network-tools/packaging/prerm" "$WORK/control/prerm"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

rm -f "$OUTPUT"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUTPUT" \
    ./debian-binary ./control.tar.gz ./data.tar.gz)

printf '%s\n' "$OUTPUT"
