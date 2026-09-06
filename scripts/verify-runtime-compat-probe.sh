#!/bin/sh
set -eu

PROBE="${ROUTERFORGE_PROBE_SCRIPT:-$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/probe-runtime-compat.sh}"
TMP="${TMPDIR:-/tmp}/routerforge-compat-probe-test.$$"
ROOT="$TMP/root"
BIN="$TMP/bin"

cleanup() {
    rm -rf "$TMP"
}

trap cleanup EXIT HUP INT TERM

mkdir -p \
    "$ROOT/opt" \
    "$ROOT/proc/net" \
    "$ROOT/sys/class/net/lo" \
    "$ROOT/sys/class/thermal/thermal_zone0" \
    "$ROOT/sys/class/hwmon" \
    "$ROOT/sys/block" \
    "$BIN"

for file in stat meminfo uptime loadavg mounts diskstats; do
    : > "$ROOT/proc/$file"
done

for file in unix dev route packet; do
    : > "$ROOT/proc/net/$file"
done

printf '42000\n' > "$ROOT/sys/class/thermal/thermal_zone0/temp"

cat > "$BIN/opkg" <<'OPKG'
#!/bin/sh
[ "$1" = "print-architecture" ] || exit 2

printf '%s\n' \
    'arch all 1' \
    "arch ${ROUTERFORGE_TEST_OPKG_ARCH:-mips-3.4} 10"
OPKG

cat > "$BIN/ndmc" <<'NDMC'
#!/bin/sh
[ "$1" = "-c" ] || exit 2
[ "$2" = "show version" ] || exit 2
printf '%s\n' 'release: synthetic'
NDMC

chmod 0755 "$BIN/opkg" "$BIN/ndmc"

run_probe() {
    profile="$1"
    expected="$2"
    actual="$3"
    output="$4"

    ROUTERFORGE_PROBE_ROOT="$ROOT" \
    ROUTERFORGE_PROBE_EXPECT_TARGET="$expected" \
    ROUTERFORGE_PROBE_OPKG="$BIN/opkg" \
    ROUTERFORGE_PROBE_NDMC="$BIN/ndmc" \
    ROUTERFORGE_PROBE_UID=0 \
    ROUTERFORGE_PROBE_KERNEL=3.4.113 \
    ROUTERFORGE_TEST_OPKG_ARCH="$actual" \
        sh "$PROBE" "$profile" > "$output"
}

run_probe \
    all \
    mips-3.4 \
    mips-3.4 \
    "$TMP/mips-ready.out"

grep -Fxq 'target=mips-3.4' "$TMP/mips-ready.out"
grep -Fxq 'module_dns=ready' "$TMP/mips-ready.out"
grep -Fxq 'overall=ready' "$TMP/mips-ready.out"
grep -Fxq 'selected_status=ready' "$TMP/mips-ready.out"

run_probe \
    all \
    mipsel-3.4 \
    mipsel-3.4 \
    "$TMP/mipsel-ready.out"

grep -Fxq 'target=mipsel-3.4' "$TMP/mipsel-ready.out"
grep -Fxq 'overall=ready' "$TMP/mipsel-ready.out"

rm -f \
    "$ROOT/proc/diskstats" \
    "$ROOT/sys/class/thermal/thermal_zone0/temp"

run_probe \
    all \
    mips-3.4 \
    mips-3.4 \
    "$TMP/degraded.out"

grep -Fxq 'module_storage=degraded' "$TMP/degraded.out"
grep -Fxq 'module_thermal=degraded' "$TMP/degraded.out"
grep -Fxq 'overall=degraded' "$TMP/degraded.out"
grep -Fxq 'selected_status=degraded' "$TMP/degraded.out"

: > "$ROOT/proc/diskstats"
printf '42000\n' > "$ROOT/sys/class/thermal/thermal_zone0/temp"
rm -f "$ROOT/proc/net/packet"

run_probe \
    dns \
    mips-3.4 \
    mips-3.4 \
    "$TMP/dns-degraded.out"

grep -Fxq 'module_dns=degraded' "$TMP/dns-degraded.out"
grep -Fxq 'selected_status=degraded' "$TMP/dns-degraded.out"

: > "$ROOT/proc/net/packet"
rm -f "$ROOT/proc/stat"

set +e

run_probe \
    admin \
    mips-3.4 \
    mips-3.4 \
    "$TMP/admin-blocked.out"

rc=$?

set -e

[ "$rc" -eq 2 ]
grep -Fxq 'module_admin=blocked' "$TMP/admin-blocked.out"
grep -Fxq 'selected_status=blocked' "$TMP/admin-blocked.out"

: > "$ROOT/proc/stat"

set +e

run_probe \
    all \
    mips-3.4 \
    mipsel-3.4 \
    "$TMP/abi-blocked.out"

rc=$?

set -e

[ "$rc" -eq 2 ]
grep -Fxq 'target_match=no' "$TMP/abi-blocked.out"
grep -Fxq 'overall=blocked' "$TMP/abi-blocked.out"
grep -Fxq 'selected_status=blocked' "$TMP/abi-blocked.out"

printf '%s\n' \
    'mips-3.4 ready: PASS' \
    'mipsel-3.4 ready: PASS' \
    'optional capability degradation: PASS' \
    'DNS AF_PACKET degradation: PASS' \
    'missing /proc/stat block: PASS' \
    'ABI mismatch block: PASS' \
    'RouterForge compatibility probe: PASS'