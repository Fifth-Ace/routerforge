#!/bin/sh
# RouterForge P18H — Keenetic policy RCI discovery probe.
#
# READ-ONLY BY DESIGN:
# - no RCI POST
# - no RCI DELETE
# - no configuration save
# - no ndmc mutation command
#
# Usage:
#   sh keenetic-policy-rci-discovery.sh [label] [output-dir]
#
# Example:
#   sh keenetic-policy-rci-discovery.sh baseline /opt/tmp
#
# Run twice around ONE reversible policy change made through the normal
# Keenetic UI/CLI to obtain before/after evidence. The probe itself never
# performs that change.

LABEL="${1:-snapshot}"
OUT_ROOT="${2:-/opt/tmp}"
STAMP="$(date '+%Y%m%d-%H%M%S' 2>/dev/null)"
[ -n "$STAMP" ] || STAMP="unknown-time"

SAFE_LABEL="$(printf '%s' "$LABEL" | tr -c 'A-Za-z0-9._-' '_' | cut -c1-48)"
[ -n "$SAFE_LABEL" ] || SAFE_LABEL="snapshot"

OUT="$OUT_ROOT/routerforge-policy-discovery-${SAFE_LABEL}-${STAMP}"
mkdir -p "$OUT" || {
    echo "FAIL: cannot create $OUT"
    exit 20
}

RCI_BASE="http://127.0.0.1:79/rci"
RESULT="PASS"

capture_cmd() {
    name="$1"
    shift
    "$@" >"$OUT/$name.stdout" 2>"$OUT/$name.stderr"
    rc=$?
    printf '%s\n' "$rc" >"$OUT/$name.rc"
    return 0
}

capture_shell() {
    name="$1"
    command="$2"
    sh -c "$command" >"$OUT/$name.stdout" 2>"$OUT/$name.stderr"
    rc=$?
    printf '%s\n' "$rc" >"$OUT/$name.rc"
    return 0
}

echo "=== PRECHECK ==="
echo "LABEL: $SAFE_LABEL"
echo "OUT: $OUT"

{
    echo "label=$SAFE_LABEL"
    echo "timestamp=$STAMP"
    echo "uname=$(uname -a 2>/dev/null)"
    echo "kernel=$(uname -r 2>/dev/null)"
    echo "machine=$(uname -m 2>/dev/null)"
    echo "probe_mode=read-only"
    echo "rci_base=$RCI_BASE"
} >"$OUT/meta.txt"

if command -v ndmc >/dev/null 2>&1; then
    echo "NDMC: FOUND"
    capture_cmd "ndmc-show-version" ndmc -c "show version"
    capture_cmd "ndmc-show-system" ndmc -c "show system"
    capture_cmd "ndmc-show-ip-policy" ndmc -c "show ip policy"

    # Never persist the complete running-config because it may contain secrets.
    # Filter only policy/route-related lines before writing evidence.
    capture_shell "ndmc-running-config-policy-filtered" \
        "ndmc -c 'show running-config' 2>/dev/null | grep -Ei '(^|[[:space:]])(ip[[:space:]]+policy|policy|proxy|route)([[:space:]]|$)'"
else
    echo "NDMC: MISSING"
    printf '%s\n' "127" >"$OUT/ndmc-show-ip-policy.rc"
    printf '%s\n' "ndmc not found" >"$OUT/ndmc-show-ip-policy.stderr"
fi

if command -v curl >/dev/null 2>&1; then
    echo "CURL: FOUND"

    # Known read endpoint already used by RouterForge DNS runtime.
    curl -q --proxy '' --noproxy '*' \
        --connect-timeout 3 --max-time 10 \
        -sS -D "$OUT/rci-show-ip-policy.headers" \
        -o "$OUT/rci-show-ip-policy.body" \
        -w '%{http_code}\n' \
        "$RCI_BASE/show/ip/policy" \
        >"$OUT/rci-show-ip-policy.http" \
        2>"$OUT/rci-show-ip-policy.stderr"
    printf '%s\n' "$?" >"$OUT/rci-show-ip-policy.rc"

    # Confirmed narrow configuration reads. Do not query the RCI root:
    # on real hardware it can expose unrelated sensitive configuration.
    for item in \
        "rci-ip-policy|/ip/policy" \
        "rci-ip-hotspot-host|/ip/hotspot/host"
    do
        name="${item%%|*}"
        path="${item#*|}"
        curl -q --proxy '' --noproxy '*' \
            --connect-timeout 3 --max-time 10 \
            -sS -D "$OUT/$name.headers" \
            -o "$OUT/$name.body" \
            -w '%{http_code}\n' \
            "$RCI_BASE$path" \
            >"$OUT/$name.http" \
            2>"$OUT/$name.stderr"
        printf '%s\n' "$?" >"$OUT/$name.rc"
    done
else
    echo "CURL: MISSING"
    printf '%s\n' "127" >"$OUT/rci-show-ip-policy.rc"
    printf '%s\n' "curl not found" >"$OUT/rci-show-ip-policy.stderr"
    RESULT="PARTIAL"
fi

echo "=== GATES ==="

RCI_HTTP=""
if [ -f "$OUT/rci-show-ip-policy.http" ]; then
    RCI_HTTP="$(tr -d '\r\n' <"$OUT/rci-show-ip-policy.http")"
fi

case "$RCI_HTTP" in
    200)
        echo "RCI_SHOW_IP_POLICY: PASS"
        ;;
    *)
        echo "RCI_SHOW_IP_POLICY: FAIL http=${RCI_HTTP:-none}"
        RESULT="PARTIAL"
        ;;
esac

if [ -s "$OUT/rci-show-ip-policy.body" ]; then
    echo "RCI_BODY_NONEMPTY: PASS"
else
    echo "RCI_BODY_NONEMPTY: FAIL"
    RESULT="PARTIAL"
fi

if [ -f "$OUT/ndmc-show-ip-policy.rc" ] && [ "$(cat "$OUT/ndmc-show-ip-policy.rc" 2>/dev/null)" = "0" ]; then
    echo "NDMC_SHOW_IP_POLICY: PASS"
else
    echo "NDMC_SHOW_IP_POLICY: PARTIAL"
fi

echo "=== ACTION ==="
echo "ACTION: READ_ONLY_EVIDENCE_CAPTURE"
echo "RCI_POST: NOT_PERFORMED"
echo "RCI_DELETE: NOT_PERFORMED"
echo "CONFIG_SAVE: NOT_PERFORMED"
echo "NDMC_MUTATION: NOT_PERFORMED"

echo "=== VERIFY ==="

if command -v sha256sum >/dev/null 2>&1; then
    (
        cd "$OUT" || exit 1
        find . -type f ! -name SHA256SUMS.txt -print | sort | while IFS= read -r f; do
            sha256sum "$f"
        done >SHA256SUMS.txt
    )
    echo "SHA256_EVIDENCE: PASS"
else
    echo "SHA256_EVIDENCE: SKIPPED (sha256sum missing)"
    RESULT="PARTIAL"
fi

MANIFEST="$OUT/MANIFEST.txt"
{
    echo "RouterForge P18H Keenetic policy discovery evidence"
    echo "label=$SAFE_LABEL"
    echo "timestamp=$STAMP"
    echo "mode=read-only"
    echo "known_rci_read=/show/ip/policy"
    echo "rci_post_performed=false"
    echo "rci_delete_performed=false"
    echo "configuration_save_performed=false"
    echo "ndmc_mutation_performed=false"
    echo "result=$RESULT"
    echo
    echo "Evidence interpretation:"
    echo "- rci-show-ip-policy.* captures the exact known RCI read contract."
    echo "- ndmc-show-ip-policy.* captures the CLI view of the same policy inventory."
    echo "- ndmc-running-config-policy-filtered.* contains only policy/route-related lines."
    echo "- rci-ip-policy.* captures the narrow configured policy subtree."
    echo "- rci-ip-hotspot-host.* captures the narrow host-to-policy binding subtree."
    echo "- RCI root is intentionally never queried because it may expose sensitive configuration."
    echo "- this probe does NOT establish a write schema by itself."
    echo "- compare baseline/after snapshots around one reversible UI/CLI policy change."
} >"$MANIFEST"

ARCHIVE="$OUT.tar.gz"
if command -v tar >/dev/null 2>&1; then
    tar -C "$(dirname "$OUT")" -czf "$ARCHIVE" "$(basename "$OUT")" 2>/dev/null
    TAR_RC=$?
    if [ "$TAR_RC" -eq 0 ]; then
        echo "EVIDENCE_ARCHIVE: $ARCHIVE"
    else
        echo "EVIDENCE_ARCHIVE: FAIL rc=$TAR_RC"
        RESULT="PARTIAL"
    fi
else
    echo "EVIDENCE_ARCHIVE: SKIPPED (tar missing)"
    RESULT="PARTIAL"
fi

echo "=== RESULT ==="
echo "STATE: $RESULT"
echo "MODE: READ_ONLY"
echo "OUTPUT_DIR: $OUT"
[ -f "$ARCHIVE" ] && echo "ARCHIVE: $ARCHIVE"
echo "MUTATION: NONE"
echo "NEXT: compare baseline/after evidence; do not implement production write path until exact schema is proven"

[ "$RESULT" = "PASS" ] && exit 0
exit 10