#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
RELEASE="${2:-}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
WORK="$DIST/opkg-work"
ARCH="aarch64-3.10"

PKG_VERSION="$VERSION"
if [ -n "$RELEASE" ]; then
    PKG_VERSION="${VERSION}-${RELEASE}"
fi
PKGFILE="dns-monitor_${PKG_VERSION}_${ARCH}.ipk"

if [ ! -f "$ROOT/frontend/build/index.html" ]; then
    sh "$ROOT/scripts/build-frontend.sh"
fi

rm -rf "$WORK"
mkdir -p "$DIST" "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
    "$WORK/data/opt/share/licenses/dns-monitor" "$WORK/control"

(
    cd "$ROOT"
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
        -tags embed_frontend \
        -trimpath \
        -ldflags="-s -w -X main.version=$PKG_VERSION" \
        -o "$WORK/data/opt/bin/dns-monitor" .
)

chmod 0755 "$WORK/data/opt/bin/dns-monitor"
cp "$ROOT/package/S90dns-monitor" "$WORK/data/opt/etc/init.d/S90dns-monitor"
chmod 0755 "$WORK/data/opt/etc/init.d/S90dns-monitor"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/dns-monitor/LICENSE"
chmod 0644 "$WORK/data/opt/share/licenses/dns-monitor/LICENSE"

sed -e "s/@VERSION@/$PKG_VERSION/g" \
    "$ROOT/packaging/opkg/control.template" > "$WORK/control/control"
cp "$ROOT/packaging/opkg/postinst" "$WORK/control/postinst"
cp "$ROOT/packaging/opkg/prerm" "$WORK/control/prerm"
cp "$ROOT/packaging/opkg/conffiles" "$WORK/control/conffiles"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

rm -f "$DIST/$PKGFILE"
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
    ./debian-binary ./control.tar.gz ./data.tar.gz)

printf '%s\n' "$DIST/$PKGFILE"
