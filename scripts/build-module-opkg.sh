#!/bin/sh
set -eu

ID="${1:?module id required}"
VERSION="${2:?version required}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST="$ROOT/dist"
ARCH="aarch64-3.10"

mkdir -p "$DIST"

build_runtime_module() {
    PACKAGE="$1"; LEGACY="$2"; BINARY="$3"; SERVICE="$4"; SOCKET="$5"; DESCRIPTION="$6"
    WORK="$DIST/${PACKAGE}-channel-work"
    PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
    (
      cd "$ROOT"
      CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
          -ldflags="-s -w -X main.version=$VERSION" \
          -o "$WORK/data/opt/bin/$BINARY" ./cmd/dns-monitor-module
    )
    chmod 0755 "$WORK/data/opt/bin/$BINARY"
    cp "$ROOT/package/modules/$SERVICE" "$WORK/data/opt/etc/init.d/$SERVICE"
    chmod 0755 "$WORK/data/opt/etc/init.d/$SERVICE"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    cat > "$WORK/control/control" <<CONTROL
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
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: $DESCRIPTION
CONTROL
    sed -e "s|@SERVICE@|$SERVICE|g" "$ROOT/packaging/modules/postinst" > "$WORK/control/postinst"
    sed -e "s|@SERVICE@|$SERVICE|g" -e "s|@SOCKET@|$SOCKET|g" "$ROOT/packaging/modules/prerm" > "$WORK/control/prerm"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"
    pack_ipk "$WORK" "$DIST/$PKGFILE"
}

build_dns() {
    PACKAGE="routerforge-dns"
    WORK="$DIST/${PACKAGE}-channel-work"
    PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
    UI_BUILD="$ROOT/frontend/dns-module-build"
    [ -f "$UI_BUILD/index.html" ] || sh "$ROOT/scripts/build-dns-frontend.sh"

    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/bin" "$WORK/data/opt/etc/init.d" \
        "$WORK/data/opt/etc/routerforge" "$WORK/data/opt/share/routerforge/modules/dns/ui" \
        "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"

    DNS_SOURCES="
    dns_module_main.go
    dns_module_server.go
    dns_control.go
    dns_rci.go
    dns_policy.go
    capture_linux.go
    client_capture_linux.go
    client_registry.go
    clients_stats.go
    diagnostics.go
    discovery.go
    dns.go
    dns_info.go
    health.go
    logger.go
    model.go
    plain_dns.go
    plain_dns_nameserver.go
    routing_discovery.go
    stats.go
    system_info.go
    "
    (
      cd "$ROOT"
      # Explicit source list is the DNS side of Module ABI v1.
      # shellcheck disable=SC2086
      CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
          -tags routerforge_dns_module \
          -ldflags="-s -w -X main.version=$VERSION" \
          -o "$WORK/data/opt/bin/routerforge-dns" $DNS_SOURCES
    )
    chmod 0755 "$WORK/data/opt/bin/routerforge-dns"
    cp "$ROOT/package/modules/S91routerforge-dns" "$WORK/data/opt/etc/init.d/S91routerforge-dns"
    chmod 0755 "$WORK/data/opt/etc/init.d/S91routerforge-dns"
    cp -R "$UI_BUILD"/. "$WORK/data/opt/share/routerforge/modules/dns/ui/"
    : > "$WORK/data/opt/etc/routerforge/dns.enabled"
    cat > "$WORK/data/opt/share/routerforge/modules/dns/manifest.json" <<MANIFEST
{
  "schema_version": 1,
  "id": "dns",
  "version": "$VERSION",
  "api_version": 1,
  "socket": "/opt/var/run/routerforge-dns.sock",
  "api_base": "/api/modules/dns",
  "ui_entry": "/api/modules/dns/ui/index.html"
}
MANIFEST
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/etc/routerforge/dns.enabled" \
        "$WORK/data/opt/share/routerforge/modules/dns/manifest.json" \
        "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"

    cat > "$WORK/control/control" <<CONTROL
Package: $PACKAGE
Version: $VERSION
Section: net
Priority: optional
Architecture: $ARCH
Depends: routerforge-core
Maintainer: Fifth-Ace
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: RouterForge DNS runtime: observability, resolver control, native multi-domain rules and transactional RCI rollback.
CONTROL
    cp "$ROOT/packaging/modules/dns.postinst" "$WORK/control/postinst"
    cp "$ROOT/packaging/modules/dns.prerm" "$WORK/control/prerm"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"
    pack_ipk "$WORK" "$DIST/$PKGFILE"
}

build_profiling() {
    PACKAGE="routerforge-profiling"; LEGACY="dns-monitor-profiling"
    WORK="$DIST/${PACKAGE}-channel-work"; PKGFILE="${PACKAGE}_${VERSION}_${ARCH}.ipk"
    rm -rf "$WORK"
    mkdir -p "$WORK/data/opt/etc/routerforge" "$WORK/data/opt/share/licenses/$PACKAGE" "$WORK/control"
    cp "$ROOT/packaging/modules/profiling.conf" "$WORK/data/opt/etc/routerforge/profiling.conf"
    : > "$WORK/data/opt/etc/routerforge/profiling.enabled"
    cp "$ROOT/LICENSE" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    chmod 0644 "$WORK/data/opt/etc/routerforge/profiling.conf" "$WORK/data/opt/etc/routerforge/profiling.enabled" "$WORK/data/opt/share/licenses/$PACKAGE/LICENSE"
    cat > "$WORK/control/control" <<CONTROL
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
Source: https://github.com/Fifth-Ace/routerforge
Homepage: https://github.com/Fifth-Ace/routerforge
License: MIT
Description: Optional loopback-only pprof and slow-request logging for RouterForge Core.
CONTROL
    cp "$ROOT/packaging/modules/profiling.postinst" "$WORK/control/postinst"
    cp "$ROOT/packaging/modules/profiling.prerm" "$WORK/control/prerm"
    printf '/opt/etc/routerforge/profiling.conf\n' > "$WORK/control/conffiles"
    chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"
    pack_ipk "$WORK" "$DIST/$PKGFILE"
}

pack_ipk() {
    WORK="$1"; OUTPUT="$2"
    printf '2.0\n' > "$WORK/debian-binary"
    (cd "$WORK/data" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" .)
    (cd "$WORK/control" && tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" .)
    rm -f "$OUTPUT"
    (cd "$WORK" && tar --owner=0 --group=0 --numeric-owner -czf "$OUTPUT" ./debian-binary ./control.tar.gz ./data.tar.gz)
    printf '%s\n' "$OUTPUT"
}

case "$ID" in
    dns) build_dns ;;
    system) build_runtime_module routerforge-system dns-monitor-system routerforge-system S92routerforge-system /opt/var/run/routerforge-system.sock "RouterForge read-only CPU, memory, load and uptime monitoring." ;;
    thermal) build_runtime_module routerforge-thermal dns-monitor-thermal routerforge-thermal S93routerforge-thermal /opt/var/run/routerforge-thermal.sock "RouterForge thermal and hwmon monitoring." ;;
    storage) build_runtime_module routerforge-storage dns-monitor-storage routerforge-storage S94routerforge-storage /opt/var/run/routerforge-storage.sock "RouterForge storage capacity and passive I/O monitoring." ;;
    network) build_runtime_module routerforge-network dns-monitor-network routerforge-network S95routerforge-network /opt/var/run/routerforge-network.sock "RouterForge interface, route and conntrack monitoring." ;;
    profiling) build_profiling ;;
    *) echo "unsupported RouterForge module id: $ID" >&2; exit 2 ;;
esac
