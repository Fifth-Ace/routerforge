#!/bin/sh
# Standalone DNS Router package builder: legacy experimental or independent DEV/Beta service. Stable unchanged.
set -eu
VERSION="${1:?version required}"
TARGET="${2:-aarch64-3.10}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
. "$ROOT/scripts/target-env.sh"
routerforge_target_init "$TARGET"
PACKAGE=routerforge-dns-router
OUT="$ROOT/dist/${PACKAGE}_${VERSION}_${RF_OPKG_ARCH}.ipk"
mkdir -p "$ROOT/dist"
: "${ROUTERFORGE_CHANNEL:=experimental}"
case "$ROUTERFORGE_CHANNEL" in experimental|dev) ;; *) echo 'Invalid DNS Router channel' >&2; exit 1 ;; esac
# P0.2: fail closed if an output name can escape dist or change lock identity.
case "$VERSION" in ''|*[!a-zA-Z0-9.+~:_-]*) echo 'DNS_ROUTER_UNSAFE_VERSION' >&2; exit 1 ;; esac
command -v flock >/dev/null 2>&1 || { echo 'DNS_ROUTER_FLOCK_REQUIRED' >&2; exit 1; }
# One lock protects a final package path; the other protects the shared Vite
# build directory for the selected channel. Different UI channels do not block
# each other. Locks live in dist and are NOT unlinked (avoids inode lock races).
OUTPUT_LOCK="$ROOT/dist/.${PACKAGE}_${VERSION}_${RF_OPKG_ARCH}.ipk.lock"
UI_LOCK="$ROOT/dist/.${PACKAGE}_${ROUTERFORGE_CHANNEL}.ui.lock"
[ ! -L "$OUTPUT_LOCK" ] && [ ! -L "$UI_LOCK" ] && [ ! -L "$OUT" ] || {
  echo 'DNS_ROUTER_SYMLINKED_BUILD_PATH_REJECTED' >&2; exit 1;
}
# Do not change the caller's umask: package assets must retain normal
# web-readable permissions. mktemp creates private staging and output files.
exec 8>>"$OUTPUT_LOCK"
echo "DNS_ROUTER_LOCK_WAIT: output ${VERSION} ${TARGET}"
flock -x -w 300 8 || { echo 'DNS_ROUTER_OUTPUT_LOCK_TIMEOUT' >&2; exit 1; }
exec 9>>"$UI_LOCK"
echo "DNS_ROUTER_LOCK_WAIT: frontend ${ROUTERFORGE_CHANNEL}"
flock -x -w 300 9 || { echo 'DNS_ROUTER_UI_LOCK_TIMEOUT' >&2; exit 1; }
echo "DNS_ROUTER_LOCKS_ACQUIRED: ${ROUTERFORGE_CHANNEL} ${TARGET}"
# Each call receives isolated data, control and tar inputs; no invocation can
# delete another call's staging, even when the channels/targets overlap.
WORK=''
OUT_TMP=''
cleanup() {
  if [ -n "$WORK" ]; then rm -rf -- "$WORK"; fi
  if [ -n "$OUT_TMP" ]; then rm -f -- "$OUT_TMP"; fi
}
trap cleanup 0
trap 'exit 129' 1
trap 'exit 130' 2
trap 'exit 143' 15
WORK="$(mktemp -d "$ROOT/dist/.${PACKAGE}-${ROUTERFORGE_CHANNEL}-work.XXXXXX")"
OUT_TMP="$(mktemp "$ROOT/dist/.${PACKAGE}-package.XXXXXX")"
DNS_MODE=manual-shadow-dns-opt-in
DNS_DEPENDS=routerforge-core
DNS_DESCRIPTION='Experimental optional DNS Router with revision-pinned manual loopback-only shadow UDP/TCP/DoT/DoH lifecycle, explicit fallback and passive diagnostics. No autostart, :53 listener, DNS takeover or router changes.'
if [ "$ROUTERFORGE_CHANNEL" = dev ]; then
  DNS_MODE=independent-dns
  DNS_DEPENDS='routerforge-core, ca-bundle'
  DNS_DESCRIPTION='Optional DNS Router independent service with domain lists, explicit default and fallback DNS, bounded cache and diagnostics. Client DNS connection is explicit; fresh installation never changes Keenetic DNS.'
fi
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
# P0.3: deterministic frontend dependency tree; guarded by the existing per-UI lock.
[ -s "$UI_SOURCE/package-lock.json" ] || { echo 'DNS_SVELTE_LOCKFILE_REQUIRED' >&2; exit 1; }
(cd "$UI_SOURCE" && npm ci --ignore-scripts --no-audit --no-fund)
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
(cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUT_TMP" ./debian-binary ./control.tar.gz ./data.tar.gz)
test -s "$OUT_TMP"
# Fail closed on actual packaged Svelte bundle (DEV Lite and experimental Runtime).
python3 - "$OUT_TMP" <<'PY_DNS_SVELTE_IPK'
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
  python3 "$ROOT/scripts/verify-beta-dns-router-package.py" "$OUT_TMP" "$TARGET" "$VERSION"
fi
# A failed validation must never destroy a previously valid destination IPK.
# The rename is atomic because the validated temporary file shares dist's FS.
mv -f "$OUT_TMP" "$OUT"
OUT_TMP=''
test -s "$OUT"
echo "DNS_ROUTER_STAGING_ISOLATION: PASS"
echo "EXPERIMENTAL_DNS_ROUTER_IPK=$OUT"
