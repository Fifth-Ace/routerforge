#!/bin/sh
# Experimental stand-alone DNS Router IPK. NOT part of Stable/Beta/App Center.
set -eu
VERSION="${1:?version required}"
TARGET="${2:-aarch64-3.10}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"
PACKAGE=routerforge-dns-router
WORK="$ROOT/dist/${PACKAGE}-experimental-${TARGET}-work"
OUT="$ROOT/dist/${PACKAGE}_${VERSION}_${RF_OPKG_ARCH}.ipk"
mkdir -p "$ROOT/dist"
rm -rf "$WORK"
mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/share/routerforge/dns-router" \
  "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
(
  cd "$ROOT"
  routerforge_go build -trimpath \
    -ldflags="-s -w -X main.version=$VERSION" \
    -o "$WORK/data/opt/bin/$PACKAGE" ./modules/dns-router-runtime/cmd/routerforge-dns-router
)
chmod 0755 "$WORK/data/opt/bin/$PACKAGE"
cp "$ROOT/modules/dns-router-runtime/packaging/example.json" \
  "$WORK/data/opt/share/routerforge/dns-router/example.json"
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
cat > "$WORK/control/control" <<CONTROL
Package: $PACKAGE
Version: $VERSION
Section: net
Priority: optional
Architecture: $RF_OPKG_ARCH
Depends: routerforge-core
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: Inert development-only DNS Router policy evaluator; never enables DNS interception or opens a listener.
CONTROL
printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUT" ./debian-binary ./control.tar.gz ./data.tar.gz)
test -s "$OUT"
echo "EXPERIMENTAL_DNS_ROUTER_IPK=$OUT"
