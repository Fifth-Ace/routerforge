#!/bin/sh
set -eu

PKG="${1:?routerforge-dpi-detector ipk required}"

[ -s "$PKG" ] || {
    echo "DPI Detector package missing: $PKG" >&2
    exit 1
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' 0

tar -xzOf "$PKG" ./control.tar.gz > "$TMP/control.tar.gz"
tar -xzOf "$PKG" ./data.tar.gz > "$TMP/data.tar.gz"

tar -xzf "$TMP/control.tar.gz" -C "$TMP"
CONTROL="$TMP/control"

grep -Fxq 'Package: routerforge-dpi-detector' "$CONTROL"
grep -Fxq 'Architecture: aarch64-3.10' "$CONTROL"
grep -Fq 'Depends: libc, libgcc, zlib' "$CONTROL"
grep -Fq 'Upstream DPI Detector by Runnin4ik' "$CONTROL"

tar -tzf "$TMP/data.tar.gz" > "$TMP/data.list"

for member in \
    './opt/bin/dpi-detector' \
    './opt/libexec/routerforge/dpi-detector/dpi-detector.bin' \
    './opt/share/routerforge/modules/dpi-detector/manifest.json' \
    './opt/share/routerforge/modules/dpi-detector/upstream.json' \
    './opt/share/routerforge/modules/dpi-detector/build-info.txt' \
    './opt/share/routerforge/modules/dpi-detector/pip-freeze.txt' \
    './opt/share/licenses/routerforge-dpi-detector/LICENSE.upstream' \
    './opt/share/licenses/routerforge-dpi-detector/LICENSE.routerforge'
do
    grep -Fxq "$member" "$TMP/data.list"
done

tar -xzOf "$TMP/data.tar.gz" ./opt/bin/dpi-detector > "$TMP/wrapper"
tar -xzOf "$TMP/data.tar.gz" \
    ./opt/libexec/routerforge/dpi-detector/dpi-detector.bin > "$TMP/payload"
tar -xzOf "$TMP/data.tar.gz" \
    ./opt/share/routerforge/modules/dpi-detector/build-info.txt > "$TMP/build-info"
tar -xzOf "$TMP/data.tar.gz" \
    ./opt/share/routerforge/modules/dpi-detector/manifest.json > "$TMP/manifest.json"
tar -xzOf "$TMP/data.tar.gz" \
    ./opt/share/routerforge/modules/dpi-detector/upstream.json > "$TMP/upstream.json"

chmod 0755 "$TMP/wrapper" "$TMP/payload"
sh -n "$TMP/wrapper"

grep -Fq "PAYLOAD='/opt/libexec/routerforge/dpi-detector/dpi-detector.bin'" "$TMP/wrapper"
grep -Fq "LOADER_LINK='/opt/lib/ld-linux-aarch64.so.1'" "$TMP/wrapper"
grep -Fq 'ld-entware.so.1' "$TMP/wrapper"
grep -Fq -- '--library-path "$LIBPATH"' "$TMP/wrapper"
grep -Fq 'TMPDIR="$PYITMP"' "$TMP/wrapper"

python3 - "$TMP/upstream.json" "$TMP/manifest.json" <<'PY'
import json
import sys

upstream = json.load(open(sys.argv[1], encoding="utf-8"))
manifest = json.load(open(sys.argv[2], encoding="utf-8"))

assert upstream["upstream"]["author"] == "Runnin4ik"
assert upstream["upstream"]["repository"] == "https://github.com/Runnin4ik/dpi-detector"
assert upstream["upstream"]["release_tag"] == manifest["upstream_tag"]
assert upstream["upstream"]["commit_sha"] == manifest["upstream_sha"]
assert upstream["build"]["pyinstaller"] == "6.22.3"
assert manifest["integration_host"] == "nfqws-manager"
assert manifest["console_command"] == "/opt/bin/dpi-detector"

print("DPI_MODULE_METADATA=PASS")
PY

PAYLOAD_SHA="$(sha256sum "$TMP/payload" | awk '{print $1}')"
INFO_SHA="$(sed -n 's/^payload_sha256=//p' "$TMP/build-info")"
[ "$PAYLOAD_SHA" = "$INFO_SHA" ]

PAYLOAD_SIZE="$(wc -c < "$TMP/payload" | tr -d ' ')"
INFO_SIZE="$(sed -n 's/^payload_size=//p' "$TMP/build-info")"
[ "$PAYLOAD_SIZE" = "$INFO_SIZE" ]

if grep -a -q 'UPX!' "$TMP/payload"; then
    echo "unexpected UPX marker in DPI Detector payload" >&2
    exit 1
fi

echo "DPI_MODULE_PACKAGE_CONTRACT=PASS"
echo "DPI_MODULE_CONSOLE_WRAPPER=PASS"
echo "DPI_MODULE_ATTRIBUTION=PASS"
echo "DPI_MODULE_UPSTREAM_PIN=PASS"
echo "DPI_MODULE_PYINSTALLER_PIN=PASS"
echo "DPI_MODULE_PAYLOAD_SHA256=$PAYLOAD_SHA"
echo "DPI_MODULE_PAYLOAD_SIZE=$PAYLOAD_SIZE"