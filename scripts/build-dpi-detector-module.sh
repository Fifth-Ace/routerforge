#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
META="$ROOT/modules/dpi-detector/upstream.json"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
VERSION="${ROUTERFORGE_DPI_PACKAGE_VERSION:?ROUTERFORGE_DPI_PACKAGE_VERSION is required}"

[ -s "$META" ] || {
    echo "DPI Detector upstream metadata missing: $META" >&2
    exit 2
}

meta_value() {
    key="$1"
    python3 - "$META" "$key" <<'PY'
import json
import sys

path, key = sys.argv[1:3]
doc = json.load(open(path, encoding="utf-8"))
value = doc
for part in key.split("."):
    value = value[part]
if isinstance(value, (dict, list)):
    raise SystemExit("metadata value is not scalar: " + key)
print(value)
PY
}

UPSTREAM_REPO="$(meta_value upstream.repository)"
UPSTREAM_TAG="$(meta_value upstream.release_tag)"
UPSTREAM_SHA="$(meta_value upstream.commit_sha)"
UPSTREAM_AUTHOR="$(meta_value upstream.author)"
PYINSTALLER_VERSION="$(meta_value build.pyinstaller)"
PYTHON_IMAGE="$(meta_value build.python_image)"
CERTIFI_VERSION="$(meta_value build.certifi)"
HTTPX_VERSION="$(meta_value build.httpx)"
H2_VERSION="$(meta_value build.h2)"
HPACK_VERSION="$(meta_value build.hpack)"
SOCKSIO_VERSION="$(meta_value build.socksio)"
RICH_VERSION="$(meta_value build.rich)"
PYYAML_VERSION="$(meta_value build.pyyaml)"

PACKAGE='routerforge-dpi-detector'
ARCH='aarch64-3.10'
ASSET_VERSION="$(printf '%s' "$VERSION" | tr '~' '-')"
PKGFILE="${PACKAGE}_${ASSET_VERSION}_${ARCH}.ipk"

case "$TARGET" in
    aarch64-3.10) ;;
    *)
        echo "routerforge-dpi-detector currently supports only aarch64-3.10" >&2
        exit 2
        ;;
esac

case "$VERSION" in
    ''|*[!A-Za-z0-9._~+-]*)
        echo "unsafe DPI Detector package version: $VERSION" >&2
        exit 2
        ;;
esac

for tool in docker git python3 tar sha256sum; do
    command -v "$tool" >/dev/null 2>&1 || {
        echo "$tool is required" >&2
        exit 2
    }
done

mkdir -p "$DIST"

WORK="$DIST/${PACKAGE}-module-work"
SRC="$WORK/upstream"
PKG="$WORK/ipk"
BUILD_INFO="$WORK/build-info.txt"

rm -rf "$WORK"
mkdir -p \
    "$WORK" \
    "$PKG/data/opt/bin" \
    "$PKG/data/opt/libexec/routerforge/dpi-detector" \
    "$PKG/data/opt/share/licenses/$PACKAGE" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector" \
    "$PKG/control"

git init -q "$SRC"
git -C "$SRC" remote add origin "${UPSTREAM_REPO}.git"
git -C "$SRC" fetch -q --depth=1 \
    origin "refs/tags/$UPSTREAM_TAG:refs/tags/$UPSTREAM_TAG"
git -C "$SRC" checkout -q --detach "refs/tags/$UPSTREAM_TAG"

ACTUAL_UPSTREAM_SHA="$(git -C "$SRC" rev-parse HEAD)"
[ "$ACTUAL_UPSTREAM_SHA" = "$UPSTREAM_SHA" ] || {
    echo "upstream $UPSTREAM_TAG resolved to $ACTUAL_UPSTREAM_SHA, expected $UPSTREAM_SHA" >&2
    exit 1
}

BIN_NAME='dpi-detector.bin'

docker run --rm --platform linux/arm64 \
    -v "$SRC:/src" \
    -w /src \
    -e BIN_NAME="$BIN_NAME" \
    -e PYINSTALLER_VERSION="$PYINSTALLER_VERSION" \
    -e CERTIFI_VERSION="$CERTIFI_VERSION" \
    -e HTTPX_VERSION="$HTTPX_VERSION" \
    -e H2_VERSION="$H2_VERSION" \
    -e HPACK_VERSION="$HPACK_VERSION" \
    -e SOCKSIO_VERSION="$SOCKSIO_VERSION" \
    -e RICH_VERSION="$RICH_VERSION" \
    -e PYYAML_VERSION="$PYYAML_VERSION" \
    "$PYTHON_IMAGE" \
    bash -ceu '
        export DEBIAN_FRONTEND=noninteractive

        apt-get update
        apt-get install -y --no-install-recommends \
            binutils gcc libc6-dev zlib1g-dev
        rm -rf /var/lib/apt/lists/*

        python -m pip install --no-cache-dir --upgrade pip
        python -m pip install --no-cache-dir \
            "PyInstaller==${PYINSTALLER_VERSION}" \
            "httpx[socks,http2]==${HTTPX_VERSION}" \
            "h2==${H2_VERSION}" \
            "hpack==${HPACK_VERSION}" \
            "socksio==${SOCKSIO_VERSION}" \
            "rich==${RICH_VERSION}" \
            "PyYAML==${PYYAML_VERSION}" \
            "certifi==${CERTIFI_VERSION}"

        python -m PyInstaller --version > /src/pyinstaller-version.txt
        python -m pip freeze > /src/pip-freeze.txt

        python -m PyInstaller \
            --onefile \
            --console \
            --clean \
            --noconfirm \
            --collect-all rich \
            --collect-all httpx \
            --collect-all certifi \
            --collect-all yaml \
            --add-data "domains.txt:." \
            --add-data "tcp16.json:." \
            --add-data "whitelist_sni.txt:." \
            --add-data "config.yml:." \
            --name "$BIN_NAME" \
            dpi_detector.py

        test -s "dist/$BIN_NAME"
        chmod 0755 "dist/$BIN_NAME"

        "./dist/$BIN_NAME" --help > /src/direct-help.txt 2>&1
        grep -Eq "DPI Detector|--tests|--batch|--domain" /src/direct-help.txt
    '

[ "$(tr -d '\r\n' < "$SRC/pyinstaller-version.txt")" = "$PYINSTALLER_VERSION" ] || {
    echo "PyInstaller pin gate failed" >&2
    exit 1
}

PAYLOAD="$PKG/data/opt/libexec/routerforge/dpi-detector/dpi-detector.bin"
cp "$SRC/dist/$BIN_NAME" "$PAYLOAD"
chmod 0755 "$PAYLOAD"

cat > "$PKG/data/opt/bin/dpi-detector" <<'WRAPPER'
#!/bin/sh
set -eu

PAYLOAD='/opt/libexec/routerforge/dpi-detector/dpi-detector.bin'
LOADER_LINK='/opt/lib/ld-linux-aarch64.so.1'
LIBPATH='/opt/lib'

fail() {
    printf '%s\n' "routerforge-dpi-detector: $*" >&2
    exit 126
}

[ -x "$PAYLOAD" ] || fail "payload missing or not executable"
[ -x "$LOADER_LINK" ] || fail "Entware AArch64 loader missing: $LOADER_LINK"
[ -d "$LIBPATH" ] || fail "Entware library directory missing: $LIBPATH"

LOADER_REAL="$(readlink -f "$LOADER_LINK" 2>/dev/null || true)"

case "$LOADER_REAL" in
    /opt/lib/ld-*.so) ;;
    *) fail "unexpected Entware loader realpath: ${LOADER_REAL:-<empty>}" ;;
esac

TMP="/tmp/routerforge-dpi-loader.$$"

umask 077
mkdir "$TMP" || fail "cannot create private loader directory"

cleanup() {
    rm -rf "$TMP" 2>/dev/null || true
}

trap cleanup 0
trap 'exit 130' 2
trap 'exit 143' 15

ALIAS="$TMP/ld-entware.so.1"
PYITMP="$TMP/pyi"

mkdir "$PYITMP" || fail "cannot create PyInstaller temp directory"
cp "$LOADER_REAL" "$ALIAS" || fail "cannot stage Entware loader alias"
chmod 0700 "$ALIAS" || fail "cannot chmod Entware loader alias"

LD_LIBRARY_PATH="$LIBPATH" \
TMPDIR="$PYITMP" \
    "$ALIAS" \
    --library-path "$LIBPATH" \
    "$PAYLOAD" \
    "$@"
WRAPPER

chmod 0755 "$PKG/data/opt/bin/dpi-detector"

cp "$SRC/LICENSE" \
    "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.upstream"
cp "$ROOT/LICENSE" \
    "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.routerforge"

cp "$META" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/upstream.json"
cp "$SRC/pip-freeze.txt" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/pip-freeze.txt"

PAYLOAD_SHA="$(sha256sum "$PAYLOAD" | awk '{print $1}')"
PAYLOAD_SIZE="$(wc -c < "$PAYLOAD" | tr -d ' ')"

cat > "$PKG/data/opt/share/routerforge/modules/dpi-detector/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "dpi-detector",
  "package": "$PACKAGE",
  "package_version": "$VERSION",
  "integration_host": "nfqws-manager",
  "ui_location": "NFQWS / DPI Detector",
  "console_command": "/opt/bin/dpi-detector",
  "upstream_repository": "$UPSTREAM_REPO",
  "upstream_author": "$UPSTREAM_AUTHOR",
  "upstream_tag": "$UPSTREAM_TAG",
  "upstream_sha": "$UPSTREAM_SHA",
  "payload_sha256": "$PAYLOAD_SHA",
  "payload_size": $PAYLOAD_SIZE
}
MANIFEST

{
    echo "schema_version=1"
    echo "package=$PACKAGE"
    echo "package_version=$VERSION"
    echo "target=$TARGET"
    echo "upstream_repository=$UPSTREAM_REPO"
    echo "upstream_author=$UPSTREAM_AUTHOR"
    echo "upstream_tag=$UPSTREAM_TAG"
    echo "upstream_sha=$UPSTREAM_SHA"
    echo "pyinstaller_version=$PYINSTALLER_VERSION"
    echo "python_image=$PYTHON_IMAGE"
    echo "payload_sha256=$PAYLOAD_SHA"
    echo "payload_size=$PAYLOAD_SIZE"
} > "$BUILD_INFO"

cp "$BUILD_INFO" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/build-info.txt"

chmod 0644 \
    "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.upstream" \
    "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.routerforge" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/upstream.json" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/pip-freeze.txt" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/manifest.json" \
    "$PKG/data/opt/share/routerforge/modules/dpi-detector/build-info.txt"

cat > "$PKG/control/control" <<CONTROL
Package: $PACKAGE
Version: $VERSION
Section: net
Priority: optional
Architecture: $ARCH
Depends: libc, libgcc, zlib
Maintainer: Fifth-Ace
Source: $UPSTREAM_REPO
Homepage: $UPSTREAM_REPO
License: MIT
Description: RouterForge DPI Detector module. Upstream DPI Detector by $UPSTREAM_AUTHOR; RouterForge provides Keenetic/Entware compatibility packaging and NFQWS web integration.
CONTROL

cat > "$PKG/control/prerm" <<'PRERM'
#!/bin/sh
rm -rf /tmp/routerforge-dpi-loader.* 2>/dev/null || true
exit 0
PRERM
chmod 0755 "$PKG/control/prerm"

printf '2.0\n' > "$PKG/debian-binary"

(
    cd "$PKG/data"
    tar --owner=0 --group=0 --numeric-owner -czf "$PKG/data.tar.gz" .
)

(
    cd "$PKG/control"
    tar --owner=0 --group=0 --numeric-owner -czf "$PKG/control.tar.gz" .
)

OUTPUT="$DIST/$PKGFILE"
rm -f "$OUTPUT"

(
    cd "$PKG"
    tar --owner=0 --group=0 --numeric-owner -czf \
        "$OUTPUT" \
        ./debian-binary \
        ./control.tar.gz \
        ./data.tar.gz
)

test -s "$OUTPUT"
printf '%s\n' "$OUTPUT"