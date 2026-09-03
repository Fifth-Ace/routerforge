#!/bin/sh
set -eu

ID="${1:?module id required}"
VERSION="${2:?version required}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
ARCH="aarch64-3.10"

mkdir -p "$DIST"

build_runtime_module() {
    PACKAGE="$1"
    LEGACY="$2"
    BINARY="$3"
    SERVICE="$4"
    SOCKET="$5"
    DESCRIPTION="$6"

    WORK="$DIST/${PACKAGE}-channel-work"
    PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
        "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"

    (
      cd "$ROOT"
      CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
          -trimpath \
          -ldflags="-s -w -X main.version=$VERSION" \
          -o "$WORK/data/opt/bin/$BINARY" ./cmd/dns-monitor-module
    )

    chmod 0755 "$WORK/data/opt/bin/$BINARY"
    cp "$ROOT/package/modules/$SERVICE" "$WORK/data/opt/etc/init.d/$SERVICE"
    chmod 0755 "$WORK/data/opt/etc/init.d/$SERVICE"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

    cat > "$WORK/control/control" <<EOF
Package: $PACKAGE
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: routerforge-core
Provides: $LEGACY
Conflicts: $LEGACY
Replaces: $LEGACY
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

build_dns() {
    PACKAGE="routerforge-dns"
    WORK="$DIST/${PACKAGE}-channel-work"
    PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/etc/routerforge" "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
    : > "$WORK/data/opt/etc/routerforge/dns.enabled"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/etc/routerforge/dns.enabled" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

    cat > "$WORK/control/control" <<EOF
Package: $PACKAGE
Version: $VERSION
Section: net
Priority: optional
Architecture: $ARCH
Depends: routerforge-core
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/dns-monitor
Homepage: https://github.com/Fifth-Ace/dns-monitor
License: MIT
Description: RouterForge DNS observability module.
EOF
    cp "$ROOT/packaging/modules/dns.postinst" "$WORK/control/postinst"
    cp "$ROOT/packaging/modules/dns.prerm" "$WORK/control/prerm"
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
    PACKAGE="routerforge-profiling"
    LEGACY="dns-monitor-profiling"
    WORK="$DIST/${PACKAGE}-channel-work"
    PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/etc/routerforge" \
        "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"

    cp "$ROOT/packaging/modules/profiling.conf" "$WORK/data/opt/etc/routerforge/profiling.conf"
    : > "$WORK/data/opt/etc/routerforge/profiling.enabled"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/etc/routerforge/profiling.conf" \
        "$WORK/data/opt/etc/routerforge/profiling.enabled" \
        "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

    cat > "$WORK/control/control" <<EOF
Package: $PACKAGE
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: routerforge-core
Provides: $LEGACY
Conflicts: $LEGACY
Replaces: $LEGACY
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/dns-monitor
Homepage: https://github.com/Fifth-Ace/dns-monitor
License: MIT
Description: Optional loopback-only pprof and slow-request logging for RouterForge Core.
EOF
    cp "$ROOT/packaging/modules/profiling.postinst" "$WORK/control/postinst"
    cp "$ROOT/packaging/modules/profiling.prerm" "$WORK/control/prerm"
    printf '/opt/etc/routerforge/profiling.conf\n' > "$WORK/control/conffiles"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

    printf '2.0\n' > "$WORK/debian-binary"
    (cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
    (cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
    rm -f "$DIST/$PKGFILE"
    (cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$DIST/$PKGFILE" \
        ./debian-binary ./control.tar.gz ./data.tar.gz)
    printf '%s\n' "$DIST/$PKGFILE"
}

case "$ID" in
    dns)
        build_dns
        ;;
    system)
        build_runtime_module routerforge-system dns-monitor-system routerforge-system \
            S92routerforge-system /opt/var/run/routerforge-system.sock \
            "RouterForge read-only CPU, memory, load and uptime monitoring."
        ;;
    thermal)
        build_runtime_module routerforge-thermal dns-monitor-thermal routerforge-thermal \
            S93routerforge-thermal /opt/var/run/routerforge-thermal.sock \
            "RouterForge thermal and hwmon monitoring."
        ;;
    storage)
        build_runtime_module routerforge-storage dns-monitor-storage routerforge-storage \
            S94routerforge-storage /opt/var/run/routerforge-storage.sock \
            "RouterForge storage capacity and passive I/O monitoring."
        ;;
    network)
        build_runtime_module routerforge-network dns-monitor-network routerforge-network \
            S95routerforge-network /opt/var/run/routerforge-network.sock \
            "RouterForge interface, route and conntrack monitoring."
        ;;
    profiling)
        build_profiling
        ;;
    *)
        echo "unsupported RouterForge module id: $ID" >&2
        exit 2
        ;;
esac
