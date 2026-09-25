#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
RELEASE="${2:-}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

PKG_VERSION="$VERSION"
if [ -n "$RELEASE" ]; then
    PKG_VERSION="${VERSION}-${RELEASE}"
fi

"$ROOT/scripts/build-module-opkg.sh" dns "$PKG_VERSION"
"$ROOT/scripts/build-module-opkg.sh" monitoring "$PKG_VERSION"
"$ROOT/scripts/build-network-tools-opkg.sh" "$PKG_VERSION"
"$ROOT/scripts/build-module-opkg.sh" nfqws-manager "$PKG_VERSION"
"$ROOT/scripts/build-module-opkg.sh" profiling "$PKG_VERSION"