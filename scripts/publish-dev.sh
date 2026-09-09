#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"

TAG=routerforge-dev
TARGET=aarch64-3.10
BASE_CONFIG=release/channels/dev.json
BUILD_CONFIG=dist/routerforge-dev-build-config.json
CANDIDATE=dist/routerforge-dev-candidate-index.json
FINAL=dist/routerforge-dev-index.json
SUMS=dist/routerforge-dev-SHA256SUMS
BOOTSTRAP=dist/routerforge-dev-bootstrap.sh
CURRENT=/tmp/routerforge-dev-current-assets.txt

: "${GITHUB_SHA:?GITHUB_SHA is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_RUN_NUMBER:?GITHUB_RUN_NUMBER is required}"
: "${ROUTERFORGE_UPX:=0}"

case "$GITHUB_RUN_NUMBER" in
    ''|*[!0-9]*)
        echo "GITHUB_RUN_NUMBER must be a positive integer." >&2
        exit 1
        ;;
esac

[ "$ROUTERFORGE_UPX" = "0" ] || {
    echo "Dev channel must use plain binaries (ROUTERFORGE_UPX=0)." >&2
    exit 1
}

SHORT_SHA="$(printf '%.12s' "$GITHUB_SHA")"
# 0.7.0-dev.<sha> was not monotonic under opkg version ordering because the
# hexadecimal SHA participated directly in comparison. Burn that prerelease
# train and move to the next patch line. "~dev" sorts below the matching stable
# release while GITHUB_RUN_NUMBER provides a monotonic rolling Dev sequence.
VERSION="0.7.1~dev.r${GITHUB_RUN_NUMBER}.${SHORT_SHA}"

mkdir -p dist
rm -f "$BUILD_CONFIG" "$CANDIDATE" "$FINAL" "$SUMS" "$BOOTSTRAP" "$CURRENT"

python3 - "$BASE_CONFIG" "$BUILD_CONFIG" "$VERSION" <<'PY'
import json
import sys

source, output, version = sys.argv[1:4]

with open(source, "r", encoding="utf-8") as fh:
    doc = json.load(fh)

if doc.get("channel") != "dev":
    raise SystemExit("dev source config has wrong channel")

doc["release_version"] = version

for item in doc.get("components", []):
    item["version"] = version
    if item.get("id") != "routerforge-core":
        item["min_core_version"] = version

with open(output, "w", encoding="utf-8") as fh:
    json.dump(doc, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
PY

echo "DEV_SOURCE_SHA=$GITHUB_SHA"
echo "DEV_VERSION=$VERSION"
echo "DEV_TARGET=$TARGET"
echo "DEV_COMPRESSION=none"

ROUTERFORGE_UPX=0 \
ROUTERFORGE_TARGET="$TARGET" \
python3 scripts/build_routerforge_channel.py \
    --config "$BUILD_CONFIG" \
    --dist dist \
    --target "$TARGET"

test -s "$CANDIDATE"
cp "$CANDIDATE" "$FINAL"

python3 scripts/render_bootstrap.py \
    --channel dev \
    --final "$FINAL" \
    --output "$BOOTSTRAP"

sh -n "$BOOTSTRAP"

python3 - "$FINAL" "$SUMS" "$CURRENT" <<'PY'
import json
import sys

index_path, sums_path, current_path = sys.argv[1:4]

with open(index_path, "r", encoding="utf-8") as fh:
    doc = json.load(fh)

if doc.get("channel") != "dev":
    raise SystemExit("generated index is not dev")

if doc.get("target") != "aarch64-3.10":
    raise SystemExit("generated dev index is not ARM64")

components = doc.get("components", [])
if not components:
    raise SystemExit("generated dev index is empty")

with open(sums_path, "w", encoding="utf-8") as sums, \
     open(current_path, "w", encoding="utf-8") as current:
    for item in components:
        asset = item["asset"]
        digest = item["sha256"]
        sums.write(f"{digest}  {asset}\n")
        current.write(asset + "\n")
PY

verify_plain() {
    package="$1"
    binary="$2"
    output="/tmp/routerforge-dev-plain-$binary"

    tar -xzOf "$package" ./data.tar.gz |
        tar -xzOf - "./opt/bin/$binary" > "$output"

    test -s "$output"
    readelf -S "$output" | grep -Fq '.text'

    if grep -a -q 'UPX!' "$output"; then
        echo "$package: UPX marker detected in Dev binary $binary" >&2
        exit 1
    fi

    rm -f "$output"
}

verify_plain "dist/routerforge-core_${VERSION}_${TARGET}.ipk" routerforge
verify_plain "dist/routerforge-admin_${VERSION}_${TARGET}.ipk" routerforge-admin
verify_plain "dist/routerforge-dns_${VERSION}_${TARGET}.ipk" routerforge-dns
verify_plain "dist/routerforge-monitoring_${VERSION}_${TARGET}.ipk" routerforge-monitoring

echo "DEV_PLAIN_BINARY_GATE=PASS"

if ! gh release view "$TAG" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
    gh release create "$TAG" \
        --repo "$GITHUB_REPOSITORY" \
        --target "$GITHUB_SHA" \
        --prerelease \
        --title "RouterForge Dev" \
        --notes "Rolling ARM64 development channel. Plain binaries; test-router use only."
fi

while IFS= read -r asset; do
    [ -n "$asset" ] || continue
    gh release upload "$TAG" --repo "$GITHUB_REPOSITORY" --clobber "dist/$asset"
done < "$CURRENT"

gh release upload "$TAG" --repo "$GITHUB_REPOSITORY" --clobber \
    "$SUMS" \
    "$BOOTSTRAP"

# The index is the App Center switch point; publish it after every referenced asset exists.
gh release upload "$TAG" --repo "$GITHUB_REPOSITORY" --clobber "$FINAL"

# Remove stale package assets only after the new index is live.
gh release view "$TAG" --repo "$GITHUB_REPOSITORY" --json assets --jq '.assets[].name' |
while IFS= read -r asset; do
    case "$asset" in
        routerforge-*.ipk)
            if ! grep -Fxq "$asset" "$CURRENT"; then
                gh release delete-asset "$TAG" "$asset" --repo "$GITHUB_REPOSITORY" --yes
            fi
            ;;
    esac
done

verify_remote_digest() {
    asset="$1"
    path="$2"
    local_sha="$(sha256sum "$path" | awk '{print $1}')"
    remote="$(
        gh api "repos/$GITHUB_REPOSITORY/releases/tags/$TAG" \
            --jq ".assets[] | select(.name == \"$asset\") | .digest"
    )"

    [ "$remote" = "sha256:$local_sha" ] || {
        echo "$asset: remote digest mismatch: $remote" >&2
        exit 1
    }
}

while IFS= read -r asset; do
    [ -n "$asset" ] || continue
    verify_remote_digest "$asset" "dist/$asset"
done < "$CURRENT"

verify_remote_digest "$(basename "$SUMS")" "$SUMS"
verify_remote_digest "$(basename "$BOOTSTRAP")" "$BOOTSTRAP"
verify_remote_digest "$(basename "$FINAL")" "$FINAL"

# The rolling Dev tag is the source identity. Move it only after all published assets
# and their remote digests have been verified successfully.
gh api \
    --method PATCH \
    "repos/$GITHUB_REPOSITORY/git/refs/tags/$TAG" \
    -f sha="$GITHUB_SHA" \
    -F force=true >/dev/null

DEV_TAG_SHA="$(
    gh api \
        "repos/$GITHUB_REPOSITORY/git/ref/tags/$TAG" \
        --jq '.object.sha'
)"

[ "$DEV_TAG_SHA" = "$GITHUB_SHA" ] || {
    echo "Dev tag mismatch: expected $GITHUB_SHA, got $DEV_TAG_SHA" >&2
    exit 1
}

echo "DEV_TAG_SHA=$DEV_TAG_SHA"
echo "DEV_TAG_GATE=PASS"

echo "DEV_RELEASE=$TAG"
echo "DEV_INDEX=$(basename "$FINAL")"
echo "DEV_VERSION=$VERSION"
echo "DEV_TARGET=$TARGET"
echo "DEV_COMPRESSION=none"
echo "DEV_REMOTE_DIGESTS=PASS"
