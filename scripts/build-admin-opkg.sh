#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
RELEASE="${2:-}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
WORK="$DIST/admin-opkg-work"
ARCH="aarch64-3.10"

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
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -trimpath \
      -ldflags='-s -w' \
      -o "$WORK/data/opt/bin/routerforge-admin" ./cmd/dns-monitor-admin
)

chmod 0755 "$WORK/data/opt/bin/routerforge-admin"
cp "$ROOT/package/S91routerforge-admin" "$WORK/data/opt/etc/init.d/S91routerforge-admin"
chmod 0755 "$WORK/data/opt/etc/init.d/S91routerforge-admin"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/routerforge-admin/LICENSE"
chmod 0644 "$WORK/data/opt/share/licenses/routerforge-admin/LICENSE"

sed -e "s/@VERSION@/$PKG_VERSION/g" \
    "$ROOT/packaging/admin/control.template" > "$WORK/control/control"
cp "$ROOT/packaging/admin/postinst" "$WORK/control/postinst"
cp "$ROOT/packaging/admin/prerm" "$WORK/control/prerm"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

rm -f "$DIST/$PKGFILE"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
    ./debian-binary ./control.tar.gz ./data.tar.gz)

printf '%s\n' "$DIST/$PKGFILE"
