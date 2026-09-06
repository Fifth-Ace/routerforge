#!/bin/sh
set -eu

TAG=routerforge-beta
SUMS=dist/routerforge-beta-SHA256SUMS
BOOTSTRAP=dist/routerforge-beta-bootstrap.sh
NOTES=dist/routerforge-beta-release-notes.md
ALL_CHANGED=/tmp/routerforge-beta-changed-assets.txt
CURRENT_ASSETS=/tmp/routerforge-beta-current-assets.txt
ARM_PREVIOUS=/tmp/routerforge-beta-index.json

: > "$ALL_CHANGED"
rm -f "$SUMS" "$CURRENT_ASSETS"

if ! gh release view "$TAG" >/dev/null 2>&1; then
    echo "Beta release anchor '$TAG' does not exist." >&2
    echo "Create the anchor manually once before FULL RELEASE." >&2
    exit 1
fi

for target in aarch64-3.10 mips-3.4 mipsel-3.4; do
    case "$target" in
        aarch64-3.10)
            INDEX=routerforge-beta-index.json
            CANDIDATE=dist/routerforge-beta-candidate-index.json
            ;;
        *)
            INDEX="routerforge-beta-index-${target}.json"
            CANDIDATE="dist/routerforge-beta-candidate-index-${target}.json"
            ;;
    esac

    PREVIOUS="/tmp/$INDEX"
    FINAL="dist/$INDEX"
    CHANGED="dist/routerforge-beta-changed-assets-${target}.txt"
    TARGET_SUMS="dist/routerforge-beta-SHA256SUMS-${target}"
    TARGET_BOOTSTRAP="dist/routerforge-beta-bootstrap-${target}.sh"

    rm -f "$PREVIOUS"

    gh release download "$TAG" -p "$INDEX" -D /tmp >/dev/null 2>&1 || true

    python3 scripts/merge_release_index.py \
        --candidate "$CANDIDATE" \
        --previous "$PREVIOUS" \
        --output "$FINAL" \
        --changed-list "$CHANGED" \
        --checksums "$TARGET_SUMS"

    python3 scripts/render_bootstrap.py \
        --channel beta \
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
    dist/routerforge-beta-SHA256SUMS-aarch64-3.10 \
    dist/routerforge-beta-SHA256SUMS-mips-3.4 \
    dist/routerforge-beta-SHA256SUMS-mipsel-3.4 |
    sort -k2,2 > "$SUMS"

python3 scripts/render_universal_bootstrap.py \
    --channel beta \
    --target aarch64-3.10 \
    --target mips-3.4 \
    --target mipsel-3.4 \
    --output "$BOOTSTRAP"

sh -n "$BOOTSTRAP"

python3 scripts/render_release_notes.py \
    --channel beta \
    --config release/channels/beta.json \
    --final dist/routerforge-beta-index.json \
    --output "$NOTES" \
    --commit "$GITHUB_SHA"

test -s "$SUMS"
test -s "$BOOTSTRAP"
test -s "$NOTES"

python3 - \
    dist/routerforge-beta-index.json \
    dist/routerforge-beta-index-mips-3.4.json \
    dist/routerforge-beta-index-mipsel-3.4.json \
    > "$CURRENT_ASSETS" <<'PY'
import json
import sys

seen = set()
for path in sys.argv[1:]:
    with open(path, "r", encoding="utf-8") as fh:
        doc = json.load(fh)
    for item in doc.get("components", []):
        asset = item.get("asset")
        if asset and asset not in seen:
            seen.add(asset)
            print(asset)
PY

# 1. Upload all changed package binaries first.
while IFS= read -r asset; do
    [ -n "$asset" ] || continue
    gh release upload "$TAG" --clobber "dist/$asset"
done < "$ALL_CHANGED"

# 2. Publish target-specific indexes/bootstrap assets only after all packages exist.
gh release upload "$TAG" --clobber \
    dist/routerforge-beta-index.json \
    dist/routerforge-beta-index-mips-3.4.json \
    dist/routerforge-beta-index-mipsel-3.4.json \
    "$SUMS" \
    dist/routerforge-beta-bootstrap-aarch64-3.10.sh \
    dist/routerforge-beta-bootstrap-mips-3.4.sh \
    dist/routerforge-beta-bootstrap-mipsel-3.4.sh

# 3. Public one-link installer is switched last.
gh release upload "$TAG" --clobber "$BOOTSTRAP"

# 4. Stale package cleanup uses the union of all three current indexes.
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

# 5. Release notes are updated after the release assets are coherent.
gh release edit "$TAG" \
    --prerelease \
    --title "RouterForge Beta" \
    --notes-file "$NOTES"

# Preserve an immutable rollback snapshot after the rolling Beta alias is coherent.
sh scripts/publish-versioned-release.sh beta
