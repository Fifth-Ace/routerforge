#!/bin/sh
set -u

MODE="${1:-runtime}"

fail() {
    echo
    echo "=== RESULT ==="
    echo "RESULT: FAIL"
    echo "$1"
    exit 1
}

field_value() {
    field="$1"
    awk -F ': *' -v key="$field" '$1 == key { print $2; exit }'
}

require_token() {
    field="$1"
    token="$2"
    value="$3"

    printf '%s\n' "$value" |
        tr ',' '\n' |
        sed 's/^[[:space:]]*//;s/[[:space:]]*$//' |
        grep -Fx "$token" >/dev/null ||
        fail "$field missing token: $token"
}

verify_package() {
    pkg="$1"
    [ -s "$pkg" ] || fail "package missing/empty: $pkg"

    control="$(
        tar -xzOf "$pkg" ./control.tar.gz 2>/dev/null |
        tar -xzOf - ./control 2>/dev/null
    )" || fail "cannot read control metadata: $pkg"

    package_name="$(printf '%s\n' "$control" | field_value Package)"
    [ "$package_name" = 'routerforge-monitoring' ] ||
        fail "$pkg Package=$package_name"

    provides="$(printf '%s\n' "$control" | field_value Provides)"
    conflicts="$(printf '%s\n' "$control" | field_value Conflicts)"
    replaces="$(printf '%s\n' "$control" | field_value Replaces)"

    legacy_packages='
routerforge-system
routerforge-thermal
routerforge-storage
routerforge-network
dns-monitor-system
dns-monitor-thermal
dns-monitor-storage
dns-monitor-network
'

    for legacy in $legacy_packages; do
        require_token Provides "$legacy" "$provides"
        require_token Conflicts "$legacy" "$conflicts"
        require_token Replaces "$legacy" "$replaces"
    done

    payload="$(
        tar -xzOf "$pkg" ./data.tar.gz 2>/dev/null |
        tar -tzf - 2>/dev/null
    )" || fail "cannot read data payload: $pkg"

    for required in \
        './opt/bin/routerforge-monitoring' \
        './opt/etc/init.d/S92routerforge-monitoring' \
        './opt/share/routerforge/modules/monitoring/manifest.json' \
        './opt/share/routerforge/modules/monitoring/ui/index.html'
    do
        printf '%s\n' "$payload" | grep -Fx "$required" >/dev/null ||
            fail "$pkg missing payload: $required"
    done

    for forbidden in \
        './opt/bin/routerforge-system' \
        './opt/bin/routerforge-thermal' \
        './opt/bin/routerforge-storage' \
        './opt/bin/routerforge-network' \
        './opt/etc/init.d/S92routerforge-system' \
        './opt/etc/init.d/S93routerforge-thermal' \
        './opt/etc/init.d/S94routerforge-storage' \
        './opt/etc/init.d/S95routerforge-network'
    do
        if printf '%s\n' "$payload" | grep -Fx "$forbidden" >/dev/null; then
            fail "$pkg contains legacy split payload: $forbidden"
        fi
    done

    postinst="$(
        tar -xzOf "$pkg" ./control.tar.gz 2>/dev/null |
        tar -xzOf - ./postinst 2>/dev/null
    )" || fail "cannot read postinst: $pkg"

    for marker in \
        'S92routerforge-system' \
        'S93routerforge-thermal' \
        'S94routerforge-storage' \
        'S95routerforge-network' \
        'S92routerforge-monitoring'
    do
        printf '%s\n' "$postinst" | grep -F "$marker" >/dev/null ||
            fail "$pkg postinst missing migration marker: $marker"
    done

    echo "PACKAGE_CONTRACT=PASS $pkg"
}

is_installed() {
    package="$1"
    "$OPKG" status "$package" 2>/dev/null |
        awk '
            $1 == "Status:" &&
            $2 == "install" &&
            $3 == "ok" &&
            $4 == "installed" {
                found=1
            }
            END { exit found ? 0 : 1 }
        '
}

package_version() {
    package="$1"
    "$OPKG" status "$package" 2>/dev/null |
        awk -F ': *' '$1 == "Version" { print $2; exit }'
}

check_health() {
    id="$1"
    url="$2"
    tmp="$TMP/$id.json"

    "$CURL" -q --proxy '' --noproxy '*' \
        -fsS --max-time 10 \
        "$url" > "$tmp" ||
        fail "$id health failed: $url"

    grep -E '"ok"[[:space:]]*:[[:space:]]*true' "$tmp" >/dev/null ||
        fail "$id health missing ok=true"

    echo "$id=PASS"
}

case "$MODE" in
    package)
        shift
        [ "$#" -gt 0 ] || fail "package mode requires at least one IPK"
        echo "=== VERIFY / PACKAGE MIGRATION CONTRACT ==="
        for pkg in "$@"; do
            verify_package "$pkg"
        done
        echo
        echo "=== RESULT ==="
        echo "RESULT: PASS"
        echo "MONITORING_PACKAGE_MIGRATION_CONTRACT=PASS"
        echo "PACKAGES=$#"
        ;;
    runtime)
        OPKG=/opt/bin/opkg
        [ -x "$OPKG" ] || OPKG="$(command -v opkg 2>/dev/null || true)"
        [ -n "${OPKG:-}" ] || fail "opkg not found"

        CURL=/opt/bin/curl
        [ -x "$CURL" ] || CURL="$(command -v curl 2>/dev/null || true)"
        [ -n "${CURL:-}" ] || fail "curl not found"

        EXPECTED_VERSION="${ROUTERFORGE_MONITORING_EXPECTED_VERSION:-}"
        BASE_URL="${ROUTERFORGE_CORE_URL:-http://127.0.0.1:2233}"
        TMP="/opt/tmp/routerforge-monitoring-migration.$$"

        cleanup() {
            rm -rf "$TMP"
        }
        trap cleanup 0 1 2 15

        mkdir -p "$TMP" || fail "cannot create temp directory"

        echo "=== PRECHECK / PACKAGE DB ==="

        is_installed routerforge-monitoring ||
            fail "routerforge-monitoring is not installed"

        current_version="$(package_version routerforge-monitoring)"
        echo "routerforge-monitoring=$current_version"

        if [ -n "$EXPECTED_VERSION" ] &&
           [ "$current_version" != "$EXPECTED_VERSION" ]; then
            fail "monitoring version=$current_version expected=$EXPECTED_VERSION"
        fi

        legacy_packages='
routerforge-system
routerforge-thermal
routerforge-storage
routerforge-network
dns-monitor-system
dns-monitor-thermal
dns-monitor-storage
dns-monitor-network
'
        for legacy in $legacy_packages; do
            if is_installed "$legacy"; then
                fail "legacy package still installed: $legacy"
            fi
        done

        echo "PASS: no legacy monitoring packages installed"

        echo
        echo "=== VERIFY / FILESYSTEM ==="

        [ -x /opt/bin/routerforge-monitoring ] ||
            fail "new monitoring binary missing"
        [ -x /opt/etc/init.d/S92routerforge-monitoring ] ||
            fail "new monitoring init script missing"
        [ -s /opt/share/routerforge/modules/monitoring/manifest.json ] ||
            fail "new monitoring manifest missing"
        [ -s /opt/share/routerforge/modules/monitoring/ui/index.html ] ||
            fail "new monitoring UI missing"

        for old in \
            /opt/bin/routerforge-system \
            /opt/bin/routerforge-thermal \
            /opt/bin/routerforge-storage \
            /opt/bin/routerforge-network \
            /opt/etc/init.d/S92routerforge-system \
            /opt/etc/init.d/S93routerforge-thermal \
            /opt/etc/init.d/S94routerforge-storage \
            /opt/etc/init.d/S95routerforge-network
        do
            [ ! -e "$old" ] || fail "legacy file remains: $old"
        done

        echo "PASS: no legacy split RouterForge files"

        echo
        echo "=== VERIFY / PROCESSES ==="

        new_pids="$(pidof routerforge-monitoring 2>/dev/null || true)"
        [ -n "$new_pids" ] || fail "routerforge-monitoring process not running"
        echo "routerforge-monitoring pid(s)=$new_pids"

        for old_process in \
            routerforge-system \
            routerforge-thermal \
            routerforge-storage \
            routerforge-network
        do
            old_pids="$(pidof "$old_process" 2>/dev/null || true)"
            [ -z "$old_pids" ] ||
                fail "legacy process still running: $old_process pid(s)=$old_pids"
        done

        echo "PASS: no legacy split RouterForge processes"

        echo
        echo "=== VERIFY / SOCKETS ==="

        for socket in \
            /opt/var/run/routerforge-monitoring.sock \
            /opt/var/run/routerforge-system.sock \
            /opt/var/run/routerforge-thermal.sock \
            /opt/var/run/routerforge-storage.sock \
            /opt/var/run/routerforge-network.sock
        do
            [ -S "$socket" ] || fail "expected Unix socket missing: $socket"
            echo "socket=PASS $socket"
        done

        echo
        echo "=== VERIFY / HEALTH ==="

        check_health monitoring "$BASE_URL/api/modules/monitoring/health"
        check_health system "$BASE_URL/api/modules/system/health"
        check_health thermal "$BASE_URL/api/modules/thermal/health"
        check_health storage "$BASE_URL/api/modules/storage/health"
        check_health network "$BASE_URL/api/modules/network/health"

        echo
        echo "=== RESULT ==="
        echo "RESULT: PASS"
        echo "MONITORING_4_TO_1_MIGRATION=PASS"
        echo "VERSION=$current_version"
        echo "LEGACY_PACKAGES=ABSENT"
        echo "LEGACY_ROUTERFORGE_FILES=ABSENT"
        echo "LEGACY_ROUTERFORGE_PROCESSES=ABSENT"
        echo "CONSOLIDATED_PROCESS=RUNNING"
        echo "PRIMARY_SOCKET=PASS"
        echo "COMPATIBILITY_SOCKETS=PASS"
        echo "MODULE_HEALTH=PASS"
        echo "FILES_REMOVED_MANUALLY=NO"
        ;;
    *)
        echo "usage: $0 package <ipk...> | runtime" >&2
        exit 64
        ;;
esac
