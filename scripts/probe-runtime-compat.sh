#!/bin/sh
set -u

PROFILE="${1:-all}"
ROOT="${ROUTERFORGE_PROBE_ROOT:-}"
EXPECTED_TARGET="${ROUTERFORGE_PROBE_EXPECT_TARGET:-}"
OPKG="${ROUTERFORGE_PROBE_OPKG:-}"
NDMC="${ROUTERFORGE_PROBE_NDMC:-}"
UID_VALUE="${ROUTERFORGE_PROBE_UID:-}"
KERNEL_VALUE="${ROUTERFORGE_PROBE_KERNEL:-}"

case "$PROFILE" in
    all|core|admin|system|network|storage|thermal|dns) ;;
    *)
        echo "usage: $0 [all|core|admin|system|network|storage|thermal|dns]" >&2
        exit 64
        ;;
esac

p() {
    printf '%s%s\n' "$ROOT" "$1"
}

yn_file() {
    if [ -r "$(p "$1")" ]; then
        printf 'yes\n'
    else
        printf 'no\n'
    fi
}

yn_dir() {
    if [ -d "$(p "$1")" ]; then
        printf 'yes\n'
    else
        printf 'no\n'
    fi
}

worst() {
    left="$1"
    right="$2"

    case "$left:$right" in
        *blocked*)  printf 'blocked\n' ;;
        *degraded*) printf 'degraded\n' ;;
        *)          printf 'ready\n' ;;
    esac
}

find_opkg() {
    if [ -n "$OPKG" ]; then
        [ -x "$OPKG" ] && {
            printf '%s\n' "$OPKG"
            return 0
        }
        return 1
    fi

    for candidate in "$(p /opt/bin/opkg)" /opt/bin/opkg; do
        [ -x "$candidate" ] && {
            printf '%s\n' "$candidate"
            return 0
        }
    done

    command -v opkg 2>/dev/null || return 1
}

find_ndmc() {
    if [ -n "$NDMC" ]; then
        [ -x "$NDMC" ] && {
            printf '%s\n' "$NDMC"
            return 0
        }
        return 1
    fi

    command -v ndmc 2>/dev/null || return 1
}

has_thermal_sensor() {
    for sensor in \
        "$(p /sys/class/thermal)"/thermal_zone*/temp \
        "$(p /sys/class/hwmon)"/hwmon*/temp*_input
    do
        [ -r "$sensor" ] && return 0
    done

    return 1
}

if [ -z "$UID_VALUE" ]; then
    UID_VALUE="$(id -u 2>/dev/null || printf 'unknown')"
fi

if [ -z "$KERNEL_VALUE" ]; then
    KERNEL_VALUE="$(uname -r 2>/dev/null || printf 'unknown')"
fi

OPKG_PATH="$(find_opkg 2>/dev/null || true)"
OPKG_OUTPUT=""
TARGET=""
TARGET_OK=no

if [ -n "$OPKG_PATH" ]; then
    OPKG_OUTPUT="$("$OPKG_PATH" print-architecture 2>/dev/null || true)"

    if [ -n "$EXPECTED_TARGET" ]; then
        TARGET="$EXPECTED_TARGET"

        if printf '%s\n' "$OPKG_OUTPUT" |
            awk -v want="$EXPECTED_TARGET" '
                $1 == "arch" && $2 == want { found=1 }
                END { exit(found ? 0 : 1) }
            '
        then
            TARGET_OK=yes
        fi
    else
        TARGET="$(
            printf '%s\n' "$OPKG_OUTPUT" |
                awk '
                    $1 == "arch" &&
                    ($2 == "aarch64-3.10" ||
                     $2 == "mips-3.4" ||
                     $2 == "mipsel-3.4") {
                        print $2
                        exit
                    }
                '
        )"

        [ -n "$TARGET" ] && TARGET_OK=yes
    fi
fi

case "$TARGET" in
    aarch64-3.10|mips-3.4|mipsel-3.4)
        TARGET_SUPPORTED=yes
        ;;
    *)
        TARGET_SUPPORTED=no
        ;;
esac

if NDMC_PATH="$(find_ndmc 2>/dev/null)"; then
    if "$NDMC_PATH" -c 'show version' >/dev/null 2>&1; then
        NDMC_READ=yes
    else
        NDMC_READ=no
    fi
else
    NDMC_PATH=""
    NDMC_READ=no
fi

OPT_DIR="$(yn_dir /opt)"

if [ "$OPT_DIR" = yes ] && [ -w "$(p /opt)" ]; then
    OPT_WRITABLE=yes
else
    OPT_WRITABLE=no
fi

PROC_UNIX="$(yn_file /proc/net/unix)"
PROC_STAT="$(yn_file /proc/stat)"
PROC_MEMINFO="$(yn_file /proc/meminfo)"
PROC_UPTIME="$(yn_file /proc/uptime)"
PROC_LOADAVG="$(yn_file /proc/loadavg)"
PROC_NET_DEV="$(yn_file /proc/net/dev)"
PROC_NET_ROUTE="$(yn_file /proc/net/route)"
PROC_NET_PACKET="$(yn_file /proc/net/packet)"
PROC_MOUNTS="$(yn_file /proc/mounts)"
PROC_DISKSTATS="$(yn_file /proc/diskstats)"
SYS_CLASS_NET="$(yn_dir /sys/class/net)"
SYS_BLOCK="$(yn_dir /sys/block)"

if [ -d "$(p /sys/class/net/lo)" ]; then
    LOOPBACK=yes
else
    LOOPBACK=no
fi

if has_thermal_sensor; then
    THERMAL_SENSOR=yes
else
    THERMAL_SENSOR=no
fi

CORE=ready

[ "$TARGET_OK" = yes ] || CORE=blocked
[ "$TARGET_SUPPORTED" = yes ] || CORE=blocked
[ "$UID_VALUE" = 0 ] || CORE=blocked
[ "$OPT_DIR" = yes ] || CORE=blocked
[ "$OPT_WRITABLE" = yes ] || CORE=blocked
[ "$PROC_UNIX" = yes ] || CORE=blocked

ADMIN="$CORE"
SYSTEM="$CORE"

if [ "$CORE" != blocked ]; then
    if [ "$PROC_STAT" != yes ]; then
        ADMIN=blocked
        SYSTEM=blocked
    else
        for value in "$PROC_MEMINFO" "$PROC_UPTIME" "$PROC_LOADAVG"; do
            if [ "$value" != yes ]; then
                ADMIN=degraded
                SYSTEM=degraded
            fi
        done
    fi
fi

NETWORK="$CORE"

if [ "$NETWORK" != blocked ]; then
    for value in \
        "$PROC_NET_DEV" \
        "$PROC_NET_ROUTE" \
        "$SYS_CLASS_NET" \
        "$NDMC_READ"
    do
        [ "$value" = yes ] || NETWORK=degraded
    done
fi

STORAGE="$CORE"

if [ "$STORAGE" != blocked ]; then
    for value in \
        "$PROC_MOUNTS" \
        "$PROC_DISKSTATS" \
        "$SYS_BLOCK"
    do
        [ "$value" = yes ] || STORAGE=degraded
    done
fi

THERMAL="$CORE"

if [ "$THERMAL" != blocked ] &&
   [ "$THERMAL_SENSOR" != yes ]; then

    THERMAL=degraded
fi

DNS="$CORE"

if [ "$DNS" != blocked ]; then
    if [ "$NDMC_READ" != yes ]; then
        DNS=blocked
    else
        for value in "$PROC_NET_PACKET" "$LOOPBACK"; do
            [ "$value" = yes ] || DNS=degraded
        done
    fi
fi

ALL="$CORE"

for value in \
    "$ADMIN" \
    "$SYSTEM" \
    "$NETWORK" \
    "$STORAGE" \
    "$THERMAL" \
    "$DNS"
do
    ALL="$(worst "$ALL" "$value")"
done

case "$PROFILE" in
    all)     SELECTED="$ALL" ;;
    core)    SELECTED="$CORE" ;;
    admin)   SELECTED="$ADMIN" ;;
    system)  SELECTED="$SYSTEM" ;;
    network) SELECTED="$NETWORK" ;;
    storage) SELECTED="$STORAGE" ;;
    thermal) SELECTED="$THERMAL" ;;
    dns)     SELECTED="$DNS" ;;
esac

printf '%s\n' \
    'probe_version=1' \
    'probe_mode=read-only' \
    "profile=$PROFILE" \
    "kernel=$KERNEL_VALUE" \
    "expected_target=$EXPECTED_TARGET" \
    "target=$TARGET" \
    "target_match=$TARGET_OK" \
    "target_supported=$TARGET_SUPPORTED" \
    "root=$([ "$UID_VALUE" = 0 ] && printf yes || printf no)" \
    "opt_dir=$OPT_DIR" \
    "opt_writable=$OPT_WRITABLE" \
    "proc_unix=$PROC_UNIX" \
    "proc_stat=$PROC_STAT" \
    "proc_meminfo=$PROC_MEMINFO" \
    "proc_uptime=$PROC_UPTIME" \
    "proc_loadavg=$PROC_LOADAVG" \
    "proc_net_dev=$PROC_NET_DEV" \
    "proc_net_route=$PROC_NET_ROUTE" \
    "proc_net_packet=$PROC_NET_PACKET" \
    "proc_mounts=$PROC_MOUNTS" \
    "proc_diskstats=$PROC_DISKSTATS" \
    "sys_class_net=$SYS_CLASS_NET" \
    "sys_block=$SYS_BLOCK" \
    "loopback=$LOOPBACK" \
    "ndmc_read=$NDMC_READ" \
    "thermal_sensor=$THERMAL_SENSOR" \
    "module_core=$CORE" \
    "module_admin=$ADMIN" \
    "module_system=$SYSTEM" \
    "module_network=$NETWORK" \
    "module_storage=$STORAGE" \
    "module_thermal=$THERMAL" \
    "module_dns=$DNS" \
    "overall=$ALL" \
    "selected_status=$SELECTED"

[ "$SELECTED" != blocked ] || exit 2
exit 0