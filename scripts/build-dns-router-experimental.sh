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
: "${ROUTERFORGE_CHANNEL:=experimental}"
case "$ROUTERFORGE_CHANNEL" in experimental|dev) ;; *) echo 'Invalid DNS Router channel' >&2; exit 1 ;; esac
DNS_MODE=manual-shadow-dns-opt-in
DNS_DEPENDS=routerforge-core
DNS_DESCRIPTION='Experimental optional DNS Router with revision-pinned manual loopback-only shadow UDP/TCP/DoT/DoH lifecycle, explicit fallback and passive diagnostics. No autostart, :53 listener, DNS takeover or router changes.'
if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
  DNS_MODE=independent-dns
  DNS_DEPENDS='routerforge-core, ca-bundle'
  DNS_DESCRIPTION='Optional DNS Router DEV service with domain lists, explicit default and fallback DNS, bounded cache and diagnostics. Client DNS connection is explicit; fresh installation never changes Keenetic DNS.'
fi
rm -rf "$WORK"
mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/share/routerforge/dns-router" \
  "$WORK/data/opt/share/routerforge/dns-router/service" \
  "$WORK/data/opt/share/routerforge/modules/dns-router/ui" \
  "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
(
  cd "$ROOT"
  if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
    cd modules/dns-router-lite
    routerforge_go build -buildvcs=false -tags dns_router_lite -trimpath \
      -ldflags="-s -w -X main.version=$VERSION" \
      -o "$WORK/data/opt/bin/$PACKAGE" ./cmd/routerforge-dns-router
  else
  routerforge_go build -trimpath \
    -ldflags="-s -w -X main.version=$VERSION" \
    -o "$WORK/data/opt/bin/$PACKAGE" ./modules/dns-router-runtime/cmd/routerforge-dns-router
  fi
)
chmod 0755 "$WORK/data/opt/bin/$PACKAGE"
cp "$ROOT/modules/dns-router-runtime/packaging/example.json" \
  "$WORK/data/opt/share/routerforge/dns-router/example.json"
# O3C: service template is inert on install. NEVER install into init.d.
cp "$ROOT/modules/dns-router-runtime/packaging/S95routerforge-dns-router" \
  "$WORK/data/opt/share/routerforge/dns-router/service/S95routerforge-dns-router"
chmod 0755 "$WORK/data/opt/share/routerforge/dns-router/service/S95routerforge-dns-router"
UI_SOURCE="$ROOT/modules/dns-router-runtime/frontend"
if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
  UI_SOURCE="$ROOT/modules/dns-router-lite/frontend"
fi
for file in index.html app.js module.css; do
  cp "$UI_SOURCE/$file" \
     "$WORK/data/opt/share/routerforge/modules/dns-router/ui/$file"
done
cat > "$WORK/data/opt/share/routerforge/modules/dns-router/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "dns-router",
  "version": "$VERSION",
  "api_version": 1,
  "socket": "/opt/var/run/routerforge-dns-router.sock",
  "api_base": "/api/modules/dns-router",
  "ui_entry": "/api/modules/dns-router/ui/index.html",
  "mode": "$DNS_MODE"
}
MANIFEST
# DEV_PACKAGE_FILES_BEGIN
if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
  mkdir -p "$WORK/data/opt/etc/init.d"
  cp "$ROOT/modules/dns-router-runtime/packaging/S95routerforge-dns-router-dev" "$WORK/data/opt/etc/init.d/S95routerforge-dns-router"
  cp "$ROOT/modules/dns-router-runtime/packaging/postinst-dev" "$WORK/control/postinst"
  cp "$ROOT/modules/dns-router-runtime/packaging/prerm-dev" "$WORK/control/prerm"
  chmod 0755 "$WORK/data/opt/etc/init.d/S95routerforge-dns-router" "$WORK/control/postinst" "$WORK/control/prerm"
  python3 "$ROOT/scripts/collect-dns-lite-licenses.py" "$ROOT/modules/dns-router-lite" "$WORK/data/opt/share/licenses/$PACKAGE/dependencies"
fi
# DEV_PACKAGE_FILES_END
cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
cp "$ROOT/modules/dns-router-runtime/packaging/QUIC-GO-LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/QUIC-GO-LICENSE"
cat > "$WORK/control/control" <<CONTROL
Package: $PACKAGE
Version: $VERSION
Section: net
Priority: optional
Architecture: $RF_OPKG_ARCH
Depends: $DNS_DEPENDS
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: $DNS_DESCRIPTION
CONTROL
printf '2.0\n' > "$WORK/debian-binary"
(cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
(cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUT" ./debian-binary ./control.tar.gz ./data.tar.gz)
test -s "$OUT"
echo "EXPERIMENTAL_DNS_ROUTER_IPK=$OUT"
