#!/bin/sh
set -eu

PKG="${1:?companion ipk required}"

[ -s "$PKG" ] || {
    echo "companion package missing: $PKG" >&2
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
grep -Fq 'D1/D2 hardware validation only' "$CONTROL"

tar -tzf "$TMP/data.tar.gz" > "$TMP/data.list"

grep -Fxq './opt/bin/dpi-detector' "$TMP/data.list"
grep -Fxq './opt/libexec/routerforge/dpi-detector/dpi-detector.bin' "$TMP/data.list"
grep -Fxq './opt/share/routerforge/dpi-detector/build-info.txt' "$TMP/data.list"
grep -Fxq './opt/share/routerforge/dpi-detector/pip-freeze.txt' "$TMP/data.list"
grep -Fxq './opt/share/licenses/routerforge-dpi-detector/LICENSE.upstream' "$TMP/data.list"
grep -Fxq './opt/share/licenses/routerforge-dpi-detector/LICENSE.routerforge' "$TMP/data.list"

tar -xzOf "$TMP/data.tar.gz" ./opt/bin/dpi-detector > "$TMP/wrapper"
tar -xzOf "$TMP/data.tar.gz" \
    ./opt/libexec/routerforge/dpi-detector/dpi-detector.bin > "$TMP/payload"
tar -xzOf "$TMP/data.tar.gz" \
    ./opt/share/routerforge/dpi-detector/build-info.txt > "$TMP/build-info"

chmod 0755 "$TMP/wrapper" "$TMP/payload"

sh -n "$TMP/wrapper"

grep -Fq "PAYLOAD='/opt/libexec/routerforge/dpi-detector/dpi-detector.bin'" "$TMP/wrapper"
grep -Fq "LOADER_LINK='/opt/lib/ld-linux-aarch64.so.1'" "$TMP/wrapper"
grep -Fq 'ld-entware.so.1' "$TMP/wrapper"
grep -Fq -- '--library-path "$LIBPATH"' "$TMP/wrapper"
grep -Fq 'TMPDIR="$PYITMP"' "$TMP/wrapper"

grep -Fxq 'upstream_tag=v4.2.4' "$TMP/build-info"
grep -Fxq 'upstream_sha=13ddc49bf5279c7fc08d3f3cca0e974b9a555191' "$TMP/build-info"
grep -Fxq 'pyinstaller_version=6.22.3' "$TMP/build-info"

PAYLOAD_SHA="$(sha256sum "$TMP/payload" | awk '{print $1}')"
INFO_SHA="$(sed -n 's/^payload_sha256=//p' "$TMP/build-info")"

[ "$PAYLOAD_SHA" = "$INFO_SHA" ] || {
    echo "payload digest mismatch: $PAYLOAD_SHA != $INFO_SHA" >&2
    exit 1
}

PAYLOAD_SIZE="$(wc -c < "$TMP/payload" | tr -d ' ')"
INFO_SIZE="$(sed -n 's/^payload_size=//p' "$TMP/build-info")"

[ "$PAYLOAD_SIZE" = "$INFO_SIZE" ] || {
    echo "payload size mismatch: $PAYLOAD_SIZE != $INFO_SIZE" >&2
    exit 1
}

if grep -a -q 'UPX!' "$TMP/payload"; then
    echo "unexpected UPX marker in dpi-detector payload" >&2
    exit 1
fi

echo "D1P1_PACKAGE_CONTRACT=PASS"
echo "D1P1_WRAPPER_CONTRACT=PASS"
echo "D1P1_UPSTREAM_PIN=PASS"
echo "D1P1_PYINSTALLER_PIN=PASS"
echo "D1P1_PAYLOAD_SHA256=$PAYLOAD_SHA"
echo "D1P1_PAYLOAD_SIZE=$PAYLOAD_SIZE"