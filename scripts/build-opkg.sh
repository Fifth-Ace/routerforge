#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
RELEASE="${2:-}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"

DIST="$ROOT/dist"
WORK="$DIST/opkg-work"
ARCH="$RF_OPKG_ARCH"
CHANNEL="${ROUTERFORGE_CHANNEL:-beta}"

PKG_VERSION="$VERSION"
if [ -n "$RELEASE" ]; then
    PKG_VERSION="${VERSION}-${RELEASE}"
fi
PKGFILE="routerforge-core_${PKG_VERSION}_${ARCH}.ipk"

if [ ! -f "$ROOT/components/core/frontend/build/index.html" ]; then
    sh "$ROOT/scripts/build-frontend.sh"
fi

rm -rf "$WORK"
mkdir -p "$DIST" "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
    "$WORK/data/opt/etc/routerforge" "$WORK/data/opt/share/licenses/routerforge-core" "$WORK/control"

# Module ABI v1 boundary: Core is built from an explicit platform source list.
# DNS capture/discovery/control sources are intentionally absent, so changing
# routerforge-dns cannot silently change the Core binary.
CORE_SOURCES="
main.go
core_log.go
web.go
admin_proxy.go
module_proxy.go
auth.go
auth_crypt.go
catalog.go
marketplace_install.go
marketplace_install_http.go
profiling.go
routerforge_registry.go
routerforge_release.go
frontend_assets_embed.go
"

(
    cd "$ROOT/components/core"
    # shellcheck disable=SC2086
    routerforge_go build \
        -tags embed_frontend \
        -trimpath \
        -ldflags="-s -w -X main.version=$PKG_VERSION -X main.releaseChannel=$CHANNEL" \
        -o "$WORK/data/opt/bin/routerforge" $CORE_SOURCES
)

chmod 0755 "$WORK/data/opt/bin/routerforge"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/routerforge"
cp "$ROOT/components/core/packaging/S90routerforge" "$WORK/data/opt/etc/init.d/S90routerforge"
chmod 0755 "$WORK/data/opt/etc/init.d/S90routerforge"
: > "$WORK/data/opt/etc/routerforge/package-management.enabled"
: > "$WORK/data/opt/etc/routerforge/module-abi-v1"
chmod 0644 "$WORK/data/opt/etc/routerforge/package-management.enabled" \
    "$WORK/data/opt/etc/routerforge/module-abi-v1"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/routerforge-core/LICENSE"
cp "$ROOT/THIRD_PARTY_NOTICES.md" "$WORK/data/opt/share/licenses/routerforge-core/THIRD_PARTY_NOTICES.md"
chmod 0644 "$WORK/data/opt/share/licenses/routerforge-core/LICENSE" \
    "$WORK/data/opt/share/licenses/routerforge-core/THIRD_PARTY_NOTICES.md"

sed -e "s/@VERSION@/$PKG_VERSION/g" \
    -e "s/@ARCH@/$ARCH/g" \
    "$ROOT/components/core/packaging/control.template" > "$WORK/control/control"
cp "$ROOT/components/core/packaging/postinst" "$WORK/control/postinst"
cp "$ROOT/components/core/packaging/prerm" "$WORK/control/prerm"
cp "$ROOT/components/core/packaging/conffiles" "$WORK/control/conffiles"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

rm -f "$DIST/$PKGFILE"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
    ./debian-binary ./control.tar.gz ./data.tar.gz)

printf '%s\n' "$DIST/$PKGFILE"
