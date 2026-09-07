#!/bin/sh
set -eu

MODE="${1:-}"
case "$MODE" in
    prepare|publish) ;;
    *)
        echo "usage: $0 prepare|publish" >&2
        exit 64
        ;;
esac

SHA="${GITHUB_SHA:-}"
[ -n "$SHA" ] || {
    echo "GITHUB_SHA is required" >&2
    exit 64
}

TAG=routerforge-stable
SUMS=dist/routerforge-stable-SHA256SUMS
BOOTSTRAP=dist/routerforge-stable-bootstrap.sh
NOTES=dist/routerforge-stable-release-notes.md
ALL_CHANGED=/tmp/routerforge-stable-changed-assets.txt
CURRENT_ASSETS=/tmp/routerforge-stable-current-assets.txt
PREPARED=dist/.routerforge-stable-multiarch-prepared

target_index_name() {
    case "$1" in
        aarch64-3.10) printf '%s\n' 'routerforge-stable-index.json' ;;
        *) printf 'routerforge-stable-index-%s.json\n' "$1" ;;
    esac
}

candidate_index_name() {
    case "$1" in
        aarch64-3.10) printf '%s\n' 'routerforge-stable-candidate-index.json' ;;
        *) printf 'routerforge-stable-candidate-index-%s.json\n' "$1" ;;
    esac
}

prepare_release() {
    sh scripts/verify-stable-promotion.sh "$SHA"

    # Immutable tag must be free before any rolling Stable mutation is allowed.
    sh scripts/publish-versioned-release.sh stable --preflight-only

    : > "$ALL_CHANGED"
    rm -f "$SUMS" "$BOOTSTRAP" "$NOTES" "$CURRENT_ASSETS" "$PREPARED"

    for target in aarch64-3.10 mips-3.4 mipsel-3.4; do
        INDEX="$(target_index_name "$target")"
        CANDIDATE="dist/$(candidate_index_name "$target")"
        PREVIOUS="/tmp/$INDEX"
        FINAL="dist/$INDEX"
        CHANGED="dist/routerforge-stable-changed-assets-${target}.txt"
        TARGET_SUMS="dist/routerforge-stable-SHA256SUMS-${target}"
        TARGET_BOOTSTRAP="dist/routerforge-stable-bootstrap-${target}.sh"

        rm -f "$PREVIOUS" "$FINAL" "$CHANGED" "$TARGET_SUMS" "$TARGET_BOOTSTRAP"

        if gh release view "$TAG" >/dev/null 2>&1; then
            gh release download "$TAG" \
                -p "$INDEX" \
                -D /tmp >/dev/null 2>&1 || true
        fi

        python3 scripts/merge_release_index.py \
            --candidate "$CANDIDATE" \
            --previous "$PREVIOUS" \
            --output "$FINAL" \
            --changed-list "$CHANGED" \
            --checksums "$TARGET_SUMS"

        python3 scripts/render_bootstrap.py \
            --channel stable \
            --final "$FINAL" \
            --output "$TARGET_BOOTSTRAP"

        sh -n "$TARGET_BOOTSTRAP"
        test -s "$FINAL"
        test -s "$TARGET_SUMS"
        test -s "$TARGET_BOOTSTRAP"

        cat "$CHANGED" >> "$ALL_CHANGED"
    done

    sort -u "$ALL_CHANGED" -o "$ALL_CHANGED"

    cat \
        dist/routerforge-stable-SHA256SUMS-aarch64-3.10 \
        dist/routerforge-stable-SHA256SUMS-mips-3.4 \
        dist/routerforge-stable-SHA256SUMS-mipsel-3.4 |
        sort -k2,2 > "$SUMS"

    python3 scripts/render_universal_bootstrap.py \
        --channel stable \
        --target aarch64-3.10 \
        --target mips-3.4 \
        --target mipsel-3.4 \
        --output "$BOOTSTRAP"

    sh -n "$BOOTSTRAP"

    python3 scripts/render_release_notes.py \
        --channel stable \
        --config release/channels/stable.json \
        --final dist/routerforge-stable-index.json \
        --output "$NOTES" \
        --commit "$SHA"

    python3 - \
        dist/routerforge-stable-index.json \
        dist/routerforge-stable-index-mips-3.4.json \
        dist/routerforge-stable-index-mipsel-3.4.json \
        > "$CURRENT_ASSETS" <<'PY'
import json
import sys

seen = set()

for path in sys.argv[1:]:
    with open(path, "r", encoding="utf-8") as fh:
        doc = json.load(fh)

    assert doc["schema_version"] == 1
    assert doc["channel"] == "stable"

    for item in doc.get("components", []):
        asset = item.get("asset")
        if asset and asset not in seen:
            seen.add(asset)
            print(asset)
PY

    test -s "$SUMS"
    test -s "$BOOTSTRAP"
    test -s "$NOTES"
    test -s "$CURRENT_ASSETS"

    while IFS= read -r asset; do
        [ -n "$asset" ] || continue
        test -s "dist/$asset"
    done < "$ALL_CHANGED"

    printf '%s\n' "$SHA" > "$PREPARED"

    echo "Stable multiarch release prepared locally: $SHA"
    echo "AArch64: hardware validated"
    echo "MIPS/MIPSel: experimental, not physically hardware-tested"
}

publish_release() {
    test -s "$PREPARED"
    test "$(cat "$PREPARED")" = "$SHA"

    test -s dist/routerforge-stable-index.json
    test -s dist/routerforge-stable-index-mips-3.4.json
    test -s dist/routerforge-stable-index-mipsel-3.4.json
    test -s "$SUMS"
    test -s "$BOOTSTRAP"
    test -s "$NOTES"
    test -s "$CURRENT_ASSETS"

    # Re-check immutable tag immediately before the first remote mutation.
    sh scripts/publish-versioned-release.sh stable --preflight-only

    if ! gh release view "$TAG" >/dev/null 2>&1; then
        gh release create "$TAG" \
            --latest \
            --title "RouterForge Stable" \
            --notes "RouterForge stable channel is being initialized."
    fi

    # 1. Upload changed component assets first.
    while IFS= read -r asset; do
        [ -n "$asset" ] || continue
        gh release upload "$TAG" --clobber "dist/$asset"
    done < "$ALL_CHANGED"

    # 2. Publish all target indexes and target bootstraps only after packages exist.
    gh release upload "$TAG" --clobber \
        dist/routerforge-stable-index.json \
        dist/routerforge-stable-index-mips-3.4.json \
        dist/routerforge-stable-index-mipsel-3.4.json \
        "$SUMS" \
        dist/routerforge-stable-bootstrap-aarch64-3.10.sh \
        dist/routerforge-stable-bootstrap-mips-3.4.sh \
        dist/routerforge-stable-bootstrap-mipsel-3.4.sh

    # 3. Switch the public one-link installer last.
    gh release upload "$TAG" --clobber "$BOOTSTRAP"

    # 4. Stale package cleanup uses the union of all current target indexes.
    gh release view "$TAG" --json assets --jq '.assets[].name' |
    while IFS= read -r asset; do
        case "$asset" in
            routerforge-*.ipk)
                if ! grep -Fxq "$asset" "$CURRENT_ASSETS"; then
                    gh release delete-asset "$TAG" "$asset" --yes
                fi
                ;;
        esac
    done

    # 5. Update release notes after the rolling Stable assets are coherent.
    gh release edit "$TAG" \
        --latest \
        --title "RouterForge Stable" \
        --notes-file "$NOTES"

    # 6. Preserve immutable multiarch snapshot.
    sh scripts/publish-versioned-release.sh stable

    echo "Stable multiarch promotion complete: $SHA"
}

case "$MODE" in
    prepare) prepare_release ;;
    publish) publish_release ;;
esac
