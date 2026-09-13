#!/bin/sh
set -eu

VERSION="${1:?version required}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"

DIST="$ROOT/dist"
ARCH="$RF_OPKG_ARCH"
RUNTIME="$DIST/routerforge-vnext-runtime-${ARCH}"
mkdir -p "$DIST"

(
    cd "$ROOT"
    routerforge_go build -trimpath \
        -ldflags="-s -w -X main.version=$VERSION" \
        -o "$RUNTIME" ./modules/vnext-runtime
)
chmod 0755 "$RUNTIME"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$RUNTIME"

pack_ipk() {
    WORK="$1"
    OUTPUT="$2"
    printf '2.0\n' > "$WORK/debian-binary"
    (cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
    (cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
    rm -f "$OUTPUT"
    (cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUTPUT" ./debian-binary ./control.tar.gz ./data.tar.gz)
}

build_one() {
    ID="$1"
    PACKAGE="$2"
    BINARY="$3"
    SERVICE="$4"
    SOCKET="$5"
    DESCRIPTION="$6"
    WORK="$DIST/${PACKAGE}-work"
    OUTPUT="$DIST/${PACKAGE}_${VERSION}_${ARCH}.ipk"
    UI="$ROOT/modules/$ID/frontend"

    rm -rf "$WORK"
    mkdir -p \
        "$WORK/data/opt/bin" \
        "$WORK/data/opt/etc/init.d" \
        "$WORK/data/opt/share/routerforge/modules/$ID/ui" \
        "$WORK/data/opt/share/routerforge/modules/$ID" \
        "$WORK/data/opt/share/licenses/$PACKAGE" \
        "$WORK/control"

    cp "$RUNTIME" "$WORK/data/opt/bin/$BINARY"
    chmod 0755 "$WORK/data/opt/bin/$BINARY"
    cp "$ROOT/modules/$ID/packaging/$SERVICE" "$WORK/data/opt/etc/init.d/$SERVICE"
    chmod 0755 "$WORK/data/opt/etc/init.d/$SERVICE"
    cp "$ROOT/modules/vnext-ui/module.css" "$WORK/data/opt/share/routerforge/modules/$ID/ui/module.css"
    cp "$ROOT/modules/vnext-ui/runtime.js" "$WORK/data/opt/share/routerforge/modules/$ID/ui/runtime.js"
    cp "$UI/index.html" "$WORK/data/opt/share/routerforge/modules/$ID/ui/index.html"
    cp "$UI/app.js" "$WORK/data/opt/share/routerforge/modules/$ID/ui/app.js"

    cat > "$WORK/data/opt/share/routerforge/modules/$ID/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "$ID",
  "version": "$VERSION",
  "api_version": 1,
  "socket": "$SOCKET",
  "api_base": "/api/modules/$ID",
  "ui_entry": "/api/modules/$ID/ui/index.html",
  "mode": "read-only-foundation"
}
MANIFEST

    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 \
        "$WORK/data/opt/share/routerforge/modules/$ID/manifest.json" \
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
Description: $DESCRIPTION
CONTROL

    sed -e "s|@SERVICE@|$SERVICE|g" -e "s|@SOCKET@|$SOCKET|g" \
        "$ROOT/modules/vnext-runtime/packaging/postinst" > "$WORK/control/postinst"
    sed -e "s|@SERVICE@|$SERVICE|g" -e "s|@SOCKET@|$SOCKET|g" \
        "$ROOT/modules/vnext-runtime/packaging/prerm" > "$WORK/control/prerm"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

    pack_ipk "$WORK" "$OUTPUT"
    printf '%s\n' "$OUTPUT"
}

build_one maintenance routerforge-maintenance routerforge-maintenance \
    S96routerforge-maintenance /opt/var/run/routerforge-maintenance.sock \
    "RouterForge Maintenance foundation: Config Vault inventory, logs/tasks metadata and Storage Doctor checks."

build_one network-tools routerforge-network-tools routerforge-network-tools \
    S97routerforge-network-tools /opt/var/run/routerforge-network-tools.sock \
    "RouterForge Network Tools foundation: Network Doctor, Route Inspector and metadata-only Flow Explorer."

build_one integrations routerforge-integrations routerforge-integrations \
    S98routerforge-integrations /opt/var/run/routerforge-integrations.sock \
    "RouterForge Integrations foundation: installed-only third-party discovery and NFQWS2 Manager diagnostics."

build_one developer-tools routerforge-developer-tools routerforge-developer-tools \
    S99routerforge-developer-tools /opt/var/run/routerforge-developer-tools.sock \
    "RouterForge Developer Tools foundation: runtime diagnostics and Module ABI manifest validation."
