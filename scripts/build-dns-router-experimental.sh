#!/bin/sh
# Standalone DNS Router package builder: legacy experimental or independent DEV/Beta service. Stable unchanged.
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
  DNS_DESCRIPTION='Optional DNS Router independent service with domain lists, explicit default and fallback DNS, bounded cache and diagnostics. Client DNS connection is explicit; fresh installation never changes Keenetic DNS.'
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
# Only channel builds set UPX explicitly: DEV=0, Beta=1.
# Keep standalone experimental and DEV smoke builds fast and plain.
if [ "${ROUTERFORGE_UPX:-0}" = "1" ]; then
  sh "$ROOT/scripts/upx-pack.sh" "$TARGET" "$WORK/data/opt/bin/$PACKAGE"
fi
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
# Both DEV Lite and legacy experimental Runtime compile native Svelte 5 mode/status.
# Dependencies are local to the disposable checkout or CI workspace only.
command -v npm >/dev/null 2>&1 || { echo 'DNS_SVELTE_NPM_REQUIRED' >&2; exit 1; }
if [ ! -d "$UI_SOURCE/node_modules" ]; then
  (cd "$UI_SOURCE" && npm install --no-save --package-lock=false --ignore-scripts --no-audit --no-fund)
fi
(cd "$UI_SOURCE" && npm run build)
test -s "$UI_SOURCE/build/svelte-mode.js"
for file in index.html module.css; do
  if [ "$ROUTERFORGE_CHANNEL" = experimental ]; then
    : # experimental app.js remains a separately tested runtime entry
  fi
  cp "$UI_SOURCE/$file" \
     "$WORK/data/opt/share/routerforge/modules/dns-router/ui/$file"
done
if [ "$ROUTERFORGE_CHANNEL" = experimental ]; then
  cp "$UI_SOURCE/app.js" "$WORK/data/opt/share/routerforge/modules/dns-router/ui/app.js"
fi
cp "$UI_SOURCE/build/svelte-mode.js" "$WORK/data/opt/share/routerforge/modules/dns-router/ui/svelte-mode.js"
test -s "$WORK/data/opt/share/routerforge/modules/dns-router/ui/svelte-mode.js"
# P22 FIX2: native Svelte bundles the visual enhancer; CSS remains separate.
# The visual enhancer is referenced by the DEV HTML and MUST ship
# in the same IPK. Experimental/Beta legacy UI remains unchanged.
if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
  for file in p22-visual.css p25-settings.css; do
    test -s "$UI_SOURCE/$file" || { echo "DNS_UI_SOURCE_MISSING: $file" >&2; exit 1; }
    cp "$UI_SOURCE/$file" "$WORK/data/opt/share/routerforge/modules/dns-router/ui/$file"
    test -s "$WORK/data/opt/share/routerforge/modules/dns-router/ui/$file"
  done
fi
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
  mkdir -p "$WORK/data/opt/etc/init.d" "$WORK/data/opt/etc/ndm/netfilter.d"
  cp "$ROOT/modules/dns-router-runtime/packaging/S95routerforge-dns-router-dev" "$WORK/data/opt/etc/init.d/S95routerforge-dns-router"
  cp "$ROOT/modules/dns-router-runtime/packaging/40-routerforge-dns-controller.sh" "$WORK/data/opt/etc/ndm/netfilter.d/40-routerforge-dns-controller.sh"
  chmod 0755 "$WORK/data/opt/etc/ndm/netfilter.d/40-routerforge-dns-controller.sh"
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
# Fail closed on actual packaged Svelte bundle (DEV Lite and experimental Runtime).
python3 - "$OUT" <<'PY_DNS_SVELTE_IPK'
import io, pathlib, sys, tarfile
path=pathlib.Path(sys.argv[1]); prefix='opt/share/routerforge/modules/dns-router/ui/'
with tarfile.open(path,'r:gz') as archive:
    data=next(m for m in archive if m.name.lstrip('./')=='data.tar.gz')
    raw=archive.extractfile(data).read()
with tarfile.open(fileobj=io.BytesIO(raw), mode='r:gz') as content:
    scripts={m.name.lstrip('./'):content.extractfile(m).read() for m in content if m.isfile() and m.name.lstrip('./').startswith(prefix)}
html=scripts.get(prefix+'index.html',b'')
asset=scripts.get(prefix+'svelte-mode.js',b'')
if b'src="svelte-mode.js"' not in html or len(asset)<3000:
    raise SystemExit('DNS_NATIVE_SVELTE_IPK: FAIL: compiled entry missing')
if b'DNS_LITE_SVELTE_MODE_MOUNT_MISSING' not in asset and b'DNS_RUNTIME_SVELTE_MODE_MOUNT_MISSING' not in asset:
    raise SystemExit('DNS_NATIVE_SVELTE_IPK: FAIL: bundle mount marker missing')
print('DNS_NATIVE_SVELTE_IPK: PASS')
PY_DNS_SVELTE_IPK
# Verify the ACTUAL DEV IPK, not just source files. Every local script/style
# referenced in the shipped HTML must exist and be nonempty in data.tar.gz.
if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
  python3 "$ROOT/scripts/verify-beta-dns-router-package.py" "$OUT" "$TARGET" "$VERSION"
fi
echo "EXPERIMENTAL_DNS_ROUTER_IPK=$OUT"
