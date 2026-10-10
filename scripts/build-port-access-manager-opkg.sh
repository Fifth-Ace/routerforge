#!/bin/sh
set -eu
VERSION="${1:?version required}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"
DIST="$ROOT/dist"
ARCH="$RF_OPKG_ARCH"
PACKAGE=routerforge-port-access-manager
WORK="$DIST/${PACKAGE}-work"
OUTPUT="$DIST/${PACKAGE}_${VERSION}_${ARCH}.ipk"
mkdir -p "$DIST"
rm -rf "$WORK"
mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
  "$WORK/data/opt/share/routerforge/modules/port-access-manager/ui" \
  "$WORK/data/opt/share/routerforge/modules/port-access-manager" \
  "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
(
  cd "$ROOT"
  routerforge_go build -trimpath -ldflags="-s -w -X main.version=$VERSION" \
    -o "$WORK/data/opt/bin/$PACKAGE" ./modules/port-access-manager/runtime
)
chmod 0755 "$WORK/data/opt/bin/$PACKAGE"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/$PACKAGE"
cp "$ROOT/modules/port-access-manager/packaging/S95routerforge-port-access-manager" "$WORK/data/opt/etc/init.d/S95routerforge-port-access-manager"
chmod 0755 "$WORK/data/opt/etc/init.d/S95routerforge-port-access-manager"
for file in index.html app.js module.css; do
  cp "$ROOT/modules/port-access-manager/frontend/$file" "$WORK/data/opt/share/routerforge/modules/port-access-manager/ui/$file"
done
cat > "$WORK/data/opt/share/routerforge/modules/port-access-manager/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "port-access-manager",
  "version": "$VERSION",
  "api_version": 1,
  "socket": "/opt/var/run/routerforge-port-access-manager.sock",
  "api_base": "/api/modules/port-access-manager",
  "ui_entry": "/api/modules/port-access-manager/ui/index.html",
  "mode": "read-only-discovery"
}
MANIFEST
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
chmod 0644 "$WORK/data/opt/share/routerforge/modules/port-access-manager/manifest.json" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
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
Description: Read-only RouterForge Port Access Manager for knockd, fwknopd and iptables recent detection.
CONTROL
cp "$ROOT/modules/port-access-manager/packaging/postinst" "$WORK/control/postinst"
cp "$ROOT/modules/port-access-manager/packaging/prerm" "$WORK/control/prerm"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"
printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
rm -f "$OUTPUT"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUTPUT" ./debian-binary ./control.tar.gz ./data.tar.gz)
printf '%s\n' "$OUTPUT"
