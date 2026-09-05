#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
RELEASE="${2:-}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"

DIST="$ROOT/dist"
WORK="$DIST/admin-opkg-work"
ARCH="$RF_OPKG_ARCH"

PKG_VERSION="$VERSION"
if [ -n "$RELEASE" ]; then
    PKG_VERSION="${VERSION}-${RELEASE}"
fi
PKGFILE="routerforge-admin_${PKG_VERSION}_${ARCH}.ipk"

rm -rf "$WORK"
mkdir -p "$DIST" "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
    "$WORK/data/opt/share/licenses/routerforge-admin" "$WORK/control"

(
  cd "$ROOT"
  routerforge_go build \
      -trimpath \
      -ldflags='-s -w' \
      -o "$WORK/data/opt/bin/routerforge-admin" ./components/control
)

chmod 0755 "$WORK/data/opt/bin/routerforge-admin"
sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/routerforge-admin"
cp "$ROOT/components/control/packaging/S91routerforge-admin" "$WORK/data/opt/etc/init.d/S91routerforge-admin"
chmod 0755 "$WORK/data/opt/etc/init.d/S91routerforge-admin"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/routerforge-admin/LICENSE"
chmod 0644 "$WORK/data/opt/share/licenses/routerforge-admin/LICENSE"

sed -e "s/@VERSION@/$PKG_VERSION/g" \
    -e "s/@ARCH@/$ARCH/g" \
    "$ROOT/components/control/packaging/control.template" > "$WORK/control/control"
cp "$ROOT/components/control/packaging/postinst" "$WORK/control/postinst"
cp "$ROOT/components/control/packaging/prerm" "$WORK/control/prerm"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

rm -f "$DIST/$PKGFILE"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
    ./debian-binary ./control.tar.gz ./data.tar.gz)

printf '%s\n' "$DIST/$PKGFILE"
