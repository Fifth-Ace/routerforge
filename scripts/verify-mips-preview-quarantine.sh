#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TMP="${TMPDIR:-/tmp}/routerforge-mips-preview-test.$$"
ROOTFS="$TMP/root"
BIN="$TMP/bin"

cleanup() {
    rm -rf "$TMP"
}

trap cleanup EXIT HUP INT TERM

mkdir -p \
    "$ROOTFS/opt" \
    "$ROOTFS/proc/net" \
    "$ROOTFS/sys/class/net/lo" \
    "$ROOTFS/sys/class/thermal/thermal_zone0" \
    "$ROOTFS/sys/class/hwmon" \
    "$ROOTFS/sys/block" \
    "$BIN"

for file in stat meminfo uptime loadavg mounts diskstats; do
    : > "$ROOTFS/proc/$file"
done

for file in unix dev route packet; do
    : > "$ROOTFS/proc/net/$file"
done

printf '42000\n' > "$ROOTFS/sys/class/thermal/thermal_zone0/temp"

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

cat > "$TMP/meminfo" <<'MEM'
MemTotal:       262144 kB
MemFree:        131072 kB
SwapTotal:      0 kB
SwapFree:       0 kB
MEM

cat > "$TMP/cpuinfo" <<'CPU'
processor       : 0
model name      : RouterForge synthetic CPU

processor       : 1
model name      : RouterForge synthetic CPU
CPU

make_index() {
    target="$1"
    output="$2"

    cat > "$output" <<JSON
{
  "schema_version": 1,
  "channel": "beta",
  "target": "$target",
  "components": [
    {
      "package": "routerforge-core",
      "version": "0.0.0-ci",
      "asset": "routerforge-core_0.0.0-ci_${target}.ipk",
      "sha256": "0000000000000000000000000000000000000000000000000000000000000000",
      "url": "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-core_0.0.0-ci_${target}.ipk",
      "canonical_url": "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-core_0.0.0-ci_${target}.ipk"
    },
    {
      "package": "routerforge-dns",
      "version": "0.0.0-ci",
      "asset": "routerforge-dns_0.0.0-ci_${target}.ipk",
      "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
      "url": "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-dns_0.0.0-ci_${target}.ipk",
      "canonical_url": "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-dns_0.0.0-ci_${target}.ipk"
    }
  ]
}
JSON
}

render_target() {
    target="$1"
    index="$TMP/${target}.json"
    bootstrap="$TMP/${target}.sh"

    make_index "$target" "$index"

    python3 "$ROOT/scripts/render_bootstrap.py" \
        --channel beta \
        --final "$index" \
        --output "$bootstrap"

    sh -n "$bootstrap"
    printf '%s\n' "$bootstrap"
}

run_case() {
    name="$1"
    target="$2"
    bootstrap="$3"
    preview="$4"
    allow_degraded="$5"
    output="$TMP/${name}.out"
    work="$TMP/work-${name}"

    rm -rf "$work"

    set +e

    ROUTERFORGE_MIPS_PREVIEW="$preview" \
    ROUTERFORGE_MIPS_ALLOW_DEGRADED="$allow_degraded" \
    ROUTERFORGE_PREINSTALL_ONLY=1 \
    ROUTERFORGE_TMP="$work" \
    ROUTERFORGE_OPKG="$BIN/opkg" \
    ROUTERFORGE_PROC_MEMINFO="$TMP/meminfo" \
    ROUTERFORGE_PROC_CPUINFO="$TMP/cpuinfo" \
    ROUTERFORGE_PROBE_ROOT="$ROOTFS" \
    ROUTERFORGE_PROBE_EXPECT_TARGET="$target" \
    ROUTERFORGE_PROBE_OPKG="$BIN/opkg" \
    ROUTERFORGE_PROBE_NDMC="$BIN/ndmc" \
    ROUTERFORGE_PROBE_UID=0 \
    ROUTERFORGE_PROBE_KERNEL=3.4.113 \
    ROUTERFORGE_TEST_OPKG_ARCH="$target" \
        sh "$bootstrap" > "$output" 2>&1

    CASE_RC=$?

    set -e
}

AARCH64_BOOTSTRAP="$(render_target aarch64-3.10)"

if grep -Fq 'ROUTERFORGE_MIPS_PREVIEW' "$AARCH64_BOOTSTRAP"; then
    echo "AArch64 bootstrap unexpectedly contains MIPS preview quarantine" >&2
    exit 1
fi

MIPS_BOOTSTRAP="$(render_target mips-3.4)"
MIPSEL_BOOTSTRAP="$(render_target mipsel-3.4)"

grep -Fq 'ROUTERFORGE_MIPS_PREVIEW' "$MIPS_BOOTSTRAP"
grep -Fq 'runtime-compat-probe.sh' "$MIPS_BOOTSTRAP"
grep -Fq 'ROUTERFORGE_MIPS_PREVIEW' "$MIPSEL_BOOTSTRAP"

run_case no-optin mips-3.4 "$MIPS_BOOTSTRAP" 0 0
[ "$CASE_RC" -eq 1 ]
grep -Fq \
    'Set ROUTERFORGE_MIPS_PREVIEW=1 to continue.' \
    "$TMP/no-optin.out"

run_case mips-ready mips-3.4 "$MIPS_BOOTSTRAP" 1 0
[ "$CASE_RC" -eq 0 ]
grep -Fxq 'selected_status=ready' "$TMP/mips-ready.out"
grep -Fq \
    'MIPS/MIPSel compatibility: ready.' \
    "$TMP/mips-ready.out"

run_case mipsel-ready mipsel-3.4 "$MIPSEL_BOOTSTRAP" 1 0
[ "$CASE_RC" -eq 0 ]
grep -Fxq 'selected_status=ready' "$TMP/mipsel-ready.out"
grep -Fq \
    'MIPS/MIPSel compatibility: ready.' \
    "$TMP/mipsel-ready.out"

rm -f \
    "$ROOTFS/proc/diskstats" \
    "$ROOTFS/sys/class/thermal/thermal_zone0/temp"

run_case degraded-rejected mips-3.4 "$MIPS_BOOTSTRAP" 1 0
[ "$CASE_RC" -eq 1 ]
grep -Fxq 'selected_status=degraded' "$TMP/degraded-rejected.out"
grep -Fq \
    'Set ROUTERFORGE_MIPS_ALLOW_DEGRADED=1' \
    "$TMP/degraded-rejected.out"

run_case degraded-accepted mips-3.4 "$MIPS_BOOTSTRAP" 1 1
[ "$CASE_RC" -eq 0 ]
grep -Fxq 'selected_status=degraded' "$TMP/degraded-accepted.out"
grep -Fq \
    'Degraded experimental operation explicitly accepted.' \
    "$TMP/degraded-accepted.out"

: > "$ROOTFS/proc/diskstats"
printf '42000\n' > "$ROOTFS/sys/class/thermal/thermal_zone0/temp"
rm -f "$ROOTFS/proc/stat"

run_case blocked mips-3.4 "$MIPS_BOOTSTRAP" 1 1
[ "$CASE_RC" -eq 1 ]
grep -Fxq 'selected_status=blocked' "$TMP/blocked.out"
grep -Fq \
    'MIPS/MIPSel compatibility is blocked on this router.' \
    "$TMP/blocked.out"

printf '%s\n' \
    'AArch64 quarantine exclusion: PASS' \
    'MIPS preview explicit opt-in: PASS' \
    'mips-3.4 ready: PASS' \
    'mipsel-3.4 ready: PASS' \
    'degraded default reject: PASS' \
    'degraded explicit override: PASS' \
    'blocked cannot be overridden: PASS' \
    'MIPS/MIPSel preview quarantine: PASS'