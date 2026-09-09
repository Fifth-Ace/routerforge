#!/bin/sh
set -eu

ROOT="${ROUTERFORGE_ROOT:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}"
DIST="${ROUTERFORGE_DIST:-$ROOT/dist}"
TMP="${TMPDIR:-/tmp}/routerforge-mips-qemu.$$"

ACTIVE_PID=""
ACTIVE_SOCKET=""

cleanup_active() {
    if [ -n "$ACTIVE_PID" ]; then
        kill "$ACTIVE_PID" 2>/dev/null || true
        wait "$ACTIVE_PID" 2>/dev/null || true
        ACTIVE_PID=""
    fi

    if [ -n "$ACTIVE_SOCKET" ]; then
        rm -f "$ACTIVE_SOCKET"
        ACTIVE_SOCKET=""
    fi
}

cleanup_all() {
    cleanup_active
    rm -rf "$TMP"
}

trap cleanup_all EXIT HUP INT TERM

command -v curl >/dev/null 2>&1 || {
    echo "curl is required for MIPS QEMU smoke tests" >&2
    exit 1
}

command -v qemu-mips >/dev/null 2>&1 || {
    echo "qemu-mips is required for MIPS QEMU smoke tests" >&2
    exit 1
}

command -v qemu-mipsel >/dev/null 2>&1 || {
    echo "qemu-mipsel is required for MIPSel QEMU smoke tests" >&2
    exit 1
}

mkdir -p "$TMP"

extract_binary() {
    package="$1"
    binary="$2"
    output="$3"

    test -s "$package"

    tar -xzOf "$package" ./data.tar.gz |
        tar -xzOf - "./opt/bin/$binary" > "$output"

    test -s "$output"
    chmod 0755 "$output"
}

smoke_help() {
    emulator="$1"
    binary="$2"
    label="$3"
    log="$TMP/${label}.help.log"

    if ! "$emulator" "$binary" -h >"$log" 2>&1; then
        echo "$label: executable failed under $emulator" >&2
        cat "$log" >&2
        return 1
    fi

    grep -Fq 'Usage of ' "$log" || {
        echo "$label: Go flag help was not produced" >&2
        cat "$log" >&2
        return 1
    }
}

smoke_server() {
    emulator="$1"
    binary="$2"
    socket="$3"
    expected_arch="$4"
    expected_module="$5"
    expected_mode="$6"
    shift 6

    log="$TMP/$(basename "$binary").server.log"

    cleanup_active

    ACTIVE_SOCKET="$socket"
    rm -f "$socket"

    "$emulator" "$binary" "$@" >"$log" 2>&1 &
    ACTIVE_PID=$!

    i=0
    while [ ! -S "$socket" ] && [ "$i" -lt 50 ]; do
        if ! kill -0 "$ACTIVE_PID" 2>/dev/null; then
            echo "$(basename "$binary"): exited before creating $socket" >&2
            cat "$log" >&2
            wait "$ACTIVE_PID" 2>/dev/null || true
            ACTIVE_PID=""
            return 1
        fi

        sleep 0.1
        i=$((i + 1))
    done

    if [ ! -S "$socket" ]; then
        echo "$(basename "$binary"): socket timeout: $socket" >&2
        cat "$log" >&2
        return 1
    fi

    health="$(
        curl --fail --silent --show-error \
            --unix-socket "$socket" \
            http://unix/v1/health
    )"

    printf '%s\n' "$health" |
        grep -Fq '"ok":true'

    printf '%s\n' "$health" |
        grep -Fq "\"mode\":\"$expected_mode\""

    if [ -n "$expected_module" ]; then
        printf '%s\n' "$health" |
            grep -Fq "\"module\":\"$expected_module\""
    fi

    summary="$(
        curl --fail --silent --show-error \
            --unix-socket "$socket" \
            http://unix/v1/summary
    )"

    printf '%s\n' "$summary" |
        grep -Fq "\"architecture\":\"$expected_arch\""

    cleanup_active
}

for target in mips-3.4 mipsel-3.4; do

    case "$target" in
        mips-3.4)
            emulator=qemu-mips
            goarch=mips
            ;;

        mipsel-3.4)
            emulator=qemu-mipsel
            goarch=mipsle
            ;;

        *)
            echo "unsupported QEMU target: $target" >&2
            exit 2
            ;;
    esac

    target_tmp="$TMP/$target"
    mkdir -p "$target_tmp"

    core="$target_tmp/routerforge"
    admin="$target_tmp/routerforge-admin"
    dns="$target_tmp/routerforge-dns"
    system="$target_tmp/routerforge-system"

    extract_binary \
        "$DIST/routerforge-core_0.0.0-ci_${target}.ipk" \
        routerforge \
        "$core"

    extract_binary \
        "$DIST/routerforge-admin_0.0.0-ci_${target}.ipk" \
        routerforge-admin \
        "$admin"

    extract_binary \
        "$DIST/routerforge-dns_0.0.0-ci_${target}.ipk" \
        routerforge-dns \
        "$dns"

    extract_binary \
        "$DIST/routerforge-system_0.0.0-ci_${target}.ipk" \
        routerforge-system \
        "$system"

    # Core and DNS require root/router-specific runtime resources.
    # -h still executes the actual cross-built ELF but exits before those checks.
    smoke_help \
        "$emulator" \
        "$core" \
        "${target}-core"

    smoke_help \
        "$emulator" \
        "$dns" \
        "${target}-dns"

    # Admin smoke only reads health/summary against the CI host /proc.
    # The runtime mode is "control" because Management v2 exposes guarded mutations.
    admin_socket="$TMP/${target}-admin.sock"

    smoke_server \
        "$emulator" \
        "$admin" \
        "$admin_socket" \
        "$goarch" \
        "" \
        control \
        -socket "$admin_socket"

    # System uses the shared monitoring runtime and exposes read-only health/summary.
    system_socket="$TMP/${target}-system.sock"

    smoke_server \
        "$emulator" \
        "$system" \
        "$system_socket" \
        "$goarch" \
        system \
        read-only \
        -module system \
        -socket "$system_socket"

    echo "$target QEMU runtime smoke: PASS"
done

echo "MIPS/MIPSel QEMU runtime smoke: PASS"