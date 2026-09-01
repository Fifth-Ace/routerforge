#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
WORK="$DIST/opkg-work"
ARCH="aarch64-3.10"
PKGFILE="dns-monitor_${VERSION}-1_${ARCH}.ipk"

rm -rf "$WORK"
mkdir -p "$DIST" "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
    "$WORK/data/opt/share/licenses/dns-monitor" "$WORK/control"

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o "$WORK/data/opt/bin/dns-monitor" "$ROOT"
chmod 0755 "$WORK/data/opt/bin/dns-monitor"
cp "$ROOT/package/S90dns-monitor" "$WORK/data/opt/etc/init.d/S90dns-monitor"
chmod 0755 "$WORK/data/opt/etc/init.d/S90dns-monitor"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/dns-monitor/LICENSE"
chmod 0644 "$WORK/data/opt/share/licenses/dns-monitor/LICENSE"

sed "s/@VERSION@/$VERSION/g" "$ROOT/packaging/opkg/control.template" > "$WORK/control/control"
cp "$ROOT/packaging/opkg/postinst" "$WORK/control/postinst"
cp "$ROOT/packaging/opkg/prerm" "$WORK/control/prerm"
cp "$ROOT/packaging/opkg/conffiles" "$WORK/control/conffiles"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

printf '2.0\n' > "$WORK/debian-binary"
(
  cd "$WORK/data"
  tar -czf "$WORK/data.tar.gz" .
)
(
  cd "$WORK/control"
  tar -czf "$WORK/control.tar.gz" .
)
rm -f "$DIST/$PKGFILE"
(
  cd "$WORK"
  ar r "$DIST/$PKGFILE" debian-binary control.tar.gz data.tar.gz >/dev/null
)

printf '%s\n' "$DIST/$PKGFILE"
