#!/bin/sh
set -eu
VERSION="${1:?version required}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"
DIST="$ROOT/dist"
ARCH="$RF_OPKG_ARCH"
PACKAGE=routerforge-crowdsec-manager
WORK="$DIST/${PACKAGE}-work"
OUTPUT="$DIST/${PACKAGE}_${VERSION}_${ARCH}.ipk"
mkdir -p "$DIST"
rm -rf "$WORK"
mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" "$WORK/data/opt/share/routerforge/modules/crowdsec-manager/ui" "$WORK/data/opt/share/routerforge/modules/crowdsec-manager" "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
(cd "$ROOT" && routerforge_go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$WORK/data/opt/bin/$PACKAGE" ./modules/crowdsec-manager/runtime)
chmod 0755 "$WORK/data/opt/bin/$PACKAGE"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/$PACKAGE"
cp "$ROOT/modules/crowdsec-manager/packaging/S99routerforge-crowdsec-manager" "$WORK/data/opt/etc/init.d/S99routerforge-crowdsec-manager"
chmod 0755 "$WORK/data/opt/etc/init.d/S99routerforge-crowdsec-manager"
for file in index.html app.js module.css; do cp "$ROOT/modules/crowdsec-manager/frontend/$file" "$WORK/data/opt/share/routerforge/modules/crowdsec-manager/ui/$file"; done
cat > "$WORK/data/opt/share/routerforge/modules/crowdsec-manager/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "crowdsec-manager",
  "version": "$VERSION",
  "api_version": 1,
  "socket": "/opt/var/run/routerforge-crowdsec-manager.sock",
  "api_base": "/api/modules/crowdsec-manager",
  "ui_entry": "/api/modules/crowdsec-manager/ui/index.html",
  "mode": "bouncer-management"
}
MANIFEST
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
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
Description: RouterForge CrowdSec bouncer manager with secure configuration, NDM recovery and firewall integrity status.
CONTROL
cp "$ROOT/modules/crowdsec-manager/packaging/postinst" "$WORK/control/postinst"
cp "$ROOT/modules/crowdsec-manager/packaging/prerm" "$WORK/control/prerm"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"
printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUTPUT" ./debian-binary ./control.tar.gz ./data.tar.gz)
printf '%s\n' "$OUTPUT"
