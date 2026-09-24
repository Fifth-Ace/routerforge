#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TMP="${TMPDIR:-/tmp}/routerforge-universal-bootstrap-test.$$"
BIN="$TMP/bin"
BETA="$TMP/routerforge-beta-bootstrap.sh"
STABLE="$TMP/routerforge-stable-bootstrap.sh"

cleanup() {
    rm -rf "$TMP"
}

trap cleanup EXIT HUP INT TERM

mkdir -p "$BIN"

python3 "$ROOT/scripts/render_universal_bootstrap.py" \
    --channel beta \
    --target aarch64-3.10 \
    --target mips-3.4 \
    --target mipsel-3.4 \
    --output "$BETA"

python3 "$ROOT/scripts/render_universal_bootstrap.py" \
    --channel stable \
    --target aarch64-3.10 \
    --output "$STABLE"

sh -n "$BETA"
sh -n "$STABLE"

grep -Fq '1) ARM64  тАФ aarch64-3.10' "$BETA"
grep -Fq '2) MIPS   тАФ mips-3.4' "$BETA"
grep -Fq '3) MIPSel тАФ mipsel-3.4' "$BETA"
grep -Fq 'ROUTERFORGE_TTY:-/dev/tty' "$BETA"
grep -Fq 'bootstrap-${TARGET}.sh' "$BETA"
grep -Fq 'normalize_input() {' "$BETA"
grep -Fq 'tty_ready() {' "$BETA"
grep -Fq '/opt/bin/curl -q -fL' "$BETA"

if grep -Fq 'ROUTERFORGE_MIPS_PREVIEW' "$BETA"; then
    echo "retired MIPS preview gate is still present" >&2
    exit 1
fi

if grep -Fq 'Continue /' "$BETA"; then
    echo "retired MIPS confirmation prompt is still present" >&2
    exit 1
fi

make_opkg() {
    file="$1"
    shift

    {
        printf '%s\n' '#!/bin/sh'
        printf '%s\n' '[ "$1" = "print-architecture" ] || exit 2'
        printf '%s\n' "printf '%s\\n' 'arch all 1' \\"
        last=$#

        i=1
        for target in "$@"; do
            if [ "$i" -eq "$last" ]; then
                printf "  'arch %s 10'\n" "$target"
            else
                printf "  'arch %s 10' \\\\\n" "$target"
            fi
            i=$((i + 1))
        done
    } > "$file"

    chmod 0755 "$file"
}

make_opkg "$BIN/opkg-arm" aarch64-3.10
make_opkg "$BIN/opkg-mips" mips-3.4
make_opkg "$BIN/opkg-mipsel" mipsel-3.4
make_opkg "$BIN/opkg-unknown" unknown-1.0
make_opkg "$BIN/opkg-ambiguous" mips-3.4 mipsel-3.4

run_ok() {
    name="$1"
    script="$2"
    opkg="$3"
    shift 3

    env \
        ROUTERFORGE_OPKG="$opkg" \
        ROUTERFORGE_DISPATCH_ONLY=1 \
        "$@" \
        sh "$script" \
        > "$TMP/$name.out" 2>&1
}

run_ok \
    auto-arm \
    "$BETA" \
    "$BIN/opkg-arm"

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: aarch64-3.10 (auto)' \
    "$TMP/auto-arm.out"

run_ok \
    auto-mips \
    "$BETA" \
    "$BIN/opkg-mips" \

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: mips-3.4 (auto)' \
    "$TMP/auto-mips.out"

run_ok \
    auto-mipsel \
    "$BETA" \
    "$BIN/opkg-mipsel" \

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: mipsel-3.4 (auto)' \
    "$TMP/auto-mipsel.out"

run_ok \
    manual-arm \
    "$BETA" \
    "$BIN/opkg-unknown" \
    ROUTERFORGE_TARGET_CHOICE=1

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: aarch64-3.10 (manual)' \
    "$TMP/manual-arm.out"

run_ok \
    manual-mips \
    "$BETA" \
    "$BIN/opkg-unknown" \
    ROUTERFORGE_TARGET_CHOICE=2 \

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: mips-3.4 (manual)' \
    "$TMP/manual-mips.out"

run_ok \
    manual-mipsel \
    "$BETA" \
    "$BIN/opkg-ambiguous" \
    ROUTERFORGE_TARGET_CHOICE=3 \

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: mipsel-3.4 (manual)' \
    "$TMP/manual-mipsel.out"

run_ok \
    explicit-target \
    "$BETA" \
    "$BIN/opkg-unknown" \
    ROUTERFORGE_TARGET=mips-3.4 \

grep -Fq \
    'Selected target / ╨Т╤Л╨▒╤А╨░╨╜╨╜╨░╤П ╨░╤А╤Е╨╕╤В╨╡╨║╤В╤Г╤А╨░: mips-3.4 (override)' \
    "$TMP/explicit-target.out"

set +e

env \
    ROUTERFORGE_OPKG="$BIN/opkg-arm" \
    ROUTERFORGE_DISPATCH_ONLY=1 \
    ROUTERFORGE_TARGET=mips-3.4 \
    sh "$BETA" \
    > "$TMP/conflict.out" 2>&1

rc=$?

set -e

[ "$rc" -eq 1 ]

grep -Fq \
    'conflicts with detected Entware target aarch64-3.10' \
    "$TMP/conflict.out"

set +e

env \
    ROUTERFORGE_OPKG="$BIN/opkg-unknown" \
    ROUTERFORGE_DISPATCH_ONLY=1 \
    ROUTERFORGE_TTY="$TMP/no-such-tty" \
    sh "$BETA" \
    > "$TMP/no-tty.out" 2>&1

rc=$?

set -e

[ "$rc" -eq 1 ]

grep -Fq \
    'Architecture selection is required.' \
    "$TMP/no-tty.out"

grep -Fq \
    'ROUTERFORGE_TARGET=mips-3.4' \
    "$TMP/no-tty.out"

set +e

env \
    ROUTERFORGE_OPKG="$BIN/opkg-unknown" \
    ROUTERFORGE_DISPATCH_ONLY=1 \
    ROUTERFORGE_TARGET_CHOICE=2 \
    sh "$STABLE" \
    > "$TMP/stable-mips.out" 2>&1

rc=$?

set -e

[ "$rc" -eq 1 ]

grep -Fq \
    'RouterForge stable does not publish packages for mips-3.4.' \
    "$TMP/stable-mips.out"

printf '%s\n' \
    'ARM64 automatic detection: PASS' \
    'MIPS automatic detection: PASS' \
    'MIPSel automatic detection: PASS' \
    'manual 1/2/3 architecture selection: PASS' \
    'ambiguous architecture fallback: PASS' \
    'non-interactive target override: PASS' \
    'conflicting override rejected: PASS' \
    'no-TTY guidance: PASS' \
    'unpublished target rejected: PASS' \
    'MIPS/MIPSel confirmation gate retired: PASS' \
    'Universal bootstrap dispatcher: PASS'
