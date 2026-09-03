#!/bin/sh
set -eu

VERSION="${1:-0.1.0}"
RELEASE="${2:-}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
ARCH="aarch64-3.10"

PKG_VERSION="$VERSION"
if [ -n "$RELEASE" ]; then
    PKG_VERSION="${VERSION}-${RELEASE}"
fi

mkdir -p "$DIST"
COMMON="$DIST/modules-common"
rm -rf "$COMMON"
mkdir -p "$COMMON"

(
  cd "$ROOT"
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -trimpath \
      -ldflags="-s -w -X main.version=$PKG_VERSION" \
      -o "$COMMON/dns-monitor-module" ./cmd/dns-monitor-module
)
chmod 0755 "$COMMON/dns-monitor-module"

build_module() {
    ID="$1"
    PACKAGE="$2"
    BINARY="$3"
    SERVICE="$4"
    SOCKET="$5"
    DESCRIPTION="$6"

    WORK="$DIST/${PACKAGE}-work"
    PKGFILE="${PACKAGE}_${PKG_VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
        "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"

    cp "$COMMON/dns-monitor-module" "$WORK/data/opt/bin/$BINARY"
    chmod 0755 "$WORK/data/opt/bin/$BINARY"
    cp "$ROOT/package/modules/$SERVICE" "$WORK/data/opt/etc/init.d/$SERVICE"
    chmod 0755 "$WORK/data/opt/etc/init.d/$SERVICE"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

    cat > "$WORK/control/control" <<EOF
Package: $PACKAGE
Version: $PKG_VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: dns-monitor
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/dns-monitor
Homepage: https://github.com/Fifth-Ace/dns-monitor
License: MIT
Description: $DESCRIPTION
EOF

    sed -e "s|@SERVICE@|$SERVICE|g" \
        "$ROOT/packaging/modules/postinst" > "$WORK/control/postinst"
    sed -e "s|@SERVICE@|$SERVICE|g" -e "s|@SOCKET@|$SOCKET|g" \
        "$ROOT/packaging/modules/prerm" > "$WORK/control/prerm"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

    printf '2.0\n' > "$WORK/debian-binary"
    (cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
    (cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

    rm -f "$DIST/$PKGFILE"
    (cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
        ./debian-binary ./control.tar.gz ./data.tar.gz)
    printf '%s\n' "$DIST/$PKGFILE"
}

build_profiling() {
    PACKAGE="dns-monitor-profiling"
    WORK="$DIST/${PACKAGE}-work"
    PKGFILE="${PACKAGE}_${PKG_VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/etc/dns-monitor" \
        "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"

    cp "$ROOT/packaging/modules/profiling.conf" "$WORK/data/opt/etc/dns-monitor/profiling.conf"
    : > "$WORK/data/opt/etc/dns-monitor/profiling.enabled"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/etc/dns-monitor/profiling.conf" \
        "$WORK/data/opt/etc/dns-monitor/profiling.enabled" \
        "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

    cat > "$WORK/control/control" <<EOF
Package: $PACKAGE
Version: $PKG_VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: dns-monitor
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/dns-monitor
Homepage: https://github.com/Fifth-Ace/dns-monitor
License: MIT
Description: Optional loopback-only pprof and slow-request logging for DNS Monitor Core.
EOF
    cp "$ROOT/packaging/modules/profiling.postinst" "$WORK/control/postinst"
    cp "$ROOT/packaging/modules/profiling.prerm" "$WORK/control/prerm"
    printf '/opt/etc/dns-monitor/profiling.conf\n' > "$WORK/control/conffiles"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

    printf '2.0\n' > "$WORK/debian-binary"
    (cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
    (cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)

    rm -f "$DIST/$PKGFILE"
    (cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
        ./debian-binary ./control.tar.gz ./data.tar.gz)
    printf '%s\n' "$DIST/$PKGFILE"
}

build_module system dns-monitor-system dnsmon-system S92dns-monitor-system \
    /opt/var/run/dns-monitor-system.sock \
    "Optional read-only CPU, memory, load and uptime monitoring for DNS Monitor."
build_module thermal dns-monitor-thermal dnsmon-thermal S93dns-monitor-thermal \
    /opt/var/run/dns-monitor-thermal.sock \
    "Optional read-only thermal and hwmon monitoring for DNS Monitor."
build_module storage dns-monitor-storage dnsmon-storage S94dns-monitor-storage \
    /opt/var/run/dns-monitor-storage.sock \
    "Optional read-only storage capacity and passive I/O monitoring for DNS Monitor."
build_module network dns-monitor-network dnsmon-network S95dns-monitor-network \
    /opt/var/run/dns-monitor-network.sock \
    "Optional read-only network interface, route and conntrack monitoring for DNS Monitor."
build_profiling
