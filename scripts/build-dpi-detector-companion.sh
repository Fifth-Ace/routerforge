#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
TARGET="${ROUTERFORGE_TARGET:-aarch64-3.10}"
VERSION="${ROUTERFORGE_DPI_PACKAGE_VERSION:?ROUTERFORGE_DPI_PACKAGE_VERSION is required}"

UPSTREAM_REPO='https://github.com/Runnin4ik/dpi-detector.git'
UPSTREAM_TAG='v4.2.4'
UPSTREAM_SHA='13ddc49bf5279c7fc08d3f3cca0e974b9a555191'
PYINSTALLER_VERSION='6.22.3'
PYTHON_IMAGE='python:3.11-slim-bookworm'

PACKAGE='routerforge-dpi-detector'
ARCH='aarch64-3.10'
ASSET_VERSION="$(printf '%s' "$VERSION" | tr '~' '-')"
PKGFILE="${PACKAGE}_${ASSET_VERSION}_${ARCH}.ipk"

case "$TARGET" in
    aarch64-3.10)
        ;;
    *)
        echo "DPI detector companion prototype supports only aarch64-3.10." >&2
        exit 2
        ;;
esac

case "$VERSION" in
    ''|*[!A-Za-z0-9._~+-]*)
        echo "unsafe companion package version: $VERSION" >&2
        exit 2
        ;;
esac

command -v docker >/dev/null 2>&1 || {
    echo "docker is required" >&2
    exit 2
}

command -v git >/dev/null 2>&1 || {
    echo "git is required" >&2
    exit 2
}

mkdir -p "$DIST"

WORK="$DIST/${PACKAGE}-d1p1-work"
SRC="$WORK/upstream"
PKG="$WORK/ipk"
BUILD_INFO="$WORK/build-info.txt"

rm -rf "$WORK"
mkdir -p "$WORK" "$PKG/data/opt/bin" \
    "$PKG/data/opt/libexec/routerforge/dpi-detector" \
    "$PKG/data/opt/share/licenses/$PACKAGE" \
    "$PKG/data/opt/share/routerforge/dpi-detector" \
    "$PKG/control"

git init -q "$SRC"
git -C "$SRC" remote add origin "$UPSTREAM_REPO"
git -C "$SRC" fetch -q --depth=1     origin "refs/tags/$UPSTREAM_TAG:refs/tags/$UPSTREAM_TAG"
git -C "$SRC" checkout -q --detach "refs/tags/$UPSTREAM_TAG"

ACTUAL_UPSTREAM_SHA="$(git -C "$SRC" rev-parse HEAD)"
[ "$ACTUAL_UPSTREAM_SHA" = "$UPSTREAM_SHA" ] || {
    echo "upstream tag $UPSTREAM_TAG resolved to unexpected SHA: $ACTUAL_UPSTREAM_SHA" >&2
    exit 1
}

BIN_NAME='dpi-detector.bin'

docker run --rm --platform linux/arm64 \
    -v "$SRC:/src" \
    -w /src \
    -e BIN_NAME="$BIN_NAME" \
    -e PYINSTALLER_VERSION="$PYINSTALLER_VERSION" \
    "$PYTHON_IMAGE" \
    bash -ceu '
        export DEBIAN_FRONTEND=noninteractive

        apt-get update
        apt-get install -y --no-install-recommends \
            binutils \
            gcc \
            libc6-dev \
            zlib1g-dev
        rm -rf /var/lib/apt/lists/*

        python -m pip install --no-cache-dir --upgrade pip

        python -m pip install --no-cache-dir \
            "PyInstaller==${PYINSTALLER_VERSION}" \
            "httpx[socks,http2]==0.28.1" \
            "h2==4.3.0" \
            "hpack==4.1.0" \
            "socksio==1.0.0" \
            "rich==15.0.0" \
            "PyYAML==6.0.3" \
            "certifi==2025.8.3"

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
    echo "PyInstaller version gate failed." >&2
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
    /opt/lib/ld-*.so)
        ;;
    *)
        fail "unexpected Entware loader realpath: ${LOADER_REAL:-<empty>}"
        ;;
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

cp "$SRC/LICENSE" "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.upstream"
cp "$ROOT/LICENSE" "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.routerforge"
chmod 0644 \
    "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.upstream" \
    "$PKG/data/opt/share/licenses/$PACKAGE/LICENSE.routerforge"

PAYLOAD_SHA="$(sha256sum "$PAYLOAD" | awk '{print $1}')"
PAYLOAD_SIZE="$(wc -c < "$PAYLOAD" | tr -d ' ')"

{
    echo "schema_version=1"
    echo "package=$PACKAGE"
    echo "package_version=$VERSION"
    echo "target=$TARGET"
    echo "upstream_repo=$UPSTREAM_REPO"
    echo "upstream_tag=$UPSTREAM_TAG"
    echo "upstream_sha=$UPSTREAM_SHA"
    echo "pyinstaller_version=$PYINSTALLER_VERSION"
    echo "python_image=$PYTHON_IMAGE"
    echo "payload_sha256=$PAYLOAD_SHA"
    echo "payload_size=$PAYLOAD_SIZE"
} > "$BUILD_INFO"

cp "$BUILD_INFO" \
    "$PKG/data/opt/share/routerforge/dpi-detector/build-info.txt"

cp "$SRC/pip-freeze.txt" \
    "$PKG/data/opt/share/routerforge/dpi-detector/pip-freeze.txt"

chmod 0644 \
    "$PKG/data/opt/share/routerforge/dpi-detector/build-info.txt" \
    "$PKG/data/opt/share/routerforge/dpi-detector/pip-freeze.txt"

cat > "$PKG/control/control" <<CONTROL
Package: $PACKAGE
Version: $VERSION
Section: net
Priority: optional
Architecture: $ARCH
Depends: libc, libgcc, zlib
Maintainer: Fifth-Ace
Source: $UPSTREAM_REPO
Homepage: https://github.com/Runnin4ik/dpi-detector
License: MIT
Description: Experimental RouterForge Entware compatibility companion for dpi-detector $UPSTREAM_TAG. D1/D2 hardware validation only; not part of the RouterForge channel index.
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