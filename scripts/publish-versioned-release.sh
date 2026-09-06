#!/bin/sh
set -eu

CHANNEL="${1:-}"
MODE="${2:-publish}"
case "$CHANNEL" in
    beta|stable) ;;
    *) echo "usage: $0 beta|stable [--preflight-only]" >&2; exit 64 ;;
esac
case "$MODE" in
    publish|--preflight-only) ;;
    *) echo "usage: $0 beta|stable [--preflight-only]" >&2; exit 64 ;;
esac

CONFIG="release/channels/${CHANNEL}.json"
DIST=dist

VERSION="$(
    python3 - "$CONFIG" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    doc = json.load(fh)

version = str(doc.get("release_version", "")).strip()
if not version:
    raise SystemExit("release_version is missing")
print(version)
PY
)"

TAG="routerforge-v${VERSION}"
SNAP="$DIST/versioned-${CHANNEL}-${VERSION}"
NOTES="$SNAP/RELEASE_NOTES.md"

rm -rf "$SNAP"
mkdir -p "$SNAP"

if gh release view "$TAG" >/dev/null 2>&1; then
    echo "Immutable release already exists: $TAG" >&2
    echo "Bump release_version before publishing another snapshot." >&2
    exit 1
fi

if [ "$MODE" = "--preflight-only" ]; then
    echo "Immutable release preflight available: $TAG"
    exit 0
fi

case "$CHANNEL" in
    beta)
        TARGETS="aarch64-3.10 mips-3.4 mipsel-3.4"
        ;;
    stable)
        TARGETS="aarch64-3.10"
        ;;
esac

for target in $TARGETS; do
    case "$target" in
        aarch64-3.10)
            SRC_INDEX="$DIST/routerforge-${CHANNEL}-index.json"
            ;;
        *)
            SRC_INDEX="$DIST/routerforge-${CHANNEL}-index-${target}.json"
            ;;
    esac

    INDEX_NAME="$(basename "$SRC_INDEX")"
    DST_INDEX="$SNAP/$INDEX_NAME"
    TARGET_BOOTSTRAP="$SNAP/routerforge-${CHANNEL}-bootstrap-${target}.sh"

    test -s "$SRC_INDEX"

    python3 - "$SRC_INDEX" "$DST_INDEX" "$CHANNEL" "$TAG" <<'PY'
import json
import sys
from pathlib import Path

src, dst, channel, tag = sys.argv[1:5]
with open(src, "r", encoding="utf-8") as fh:
    doc = json.load(fh)

prefix = f"https://github.com/Fifth-Ace/routerforge/releases/download/{tag}/"

for item in doc.get("components", []):
    asset = item.get("asset")
    if not asset:
        raise SystemExit(f"{src}: component without asset")
    immutable_url = prefix + asset
    item["url"] = immutable_url
    item["canonical_url"] = immutable_url

Path(dst).write_text(
    json.dumps(doc, ensure_ascii=False, indent=2) + "\n",
    encoding="utf-8",
)
PY

    python3 scripts/render_bootstrap.py \
        --channel "$CHANNEL" \
        --release-tag "$TAG" \
        --final "$DST_INDEX" \
        --output "$TARGET_BOOTSTRAP"

    sh -n "$TARGET_BOOTSTRAP"

    python3 - "$DST_INDEX" "$DIST" "$SNAP" <<'PY'
import json
import shutil
import sys
from pathlib import Path

index, dist, snap = map(Path, sys.argv[1:4])
with index.open("r", encoding="utf-8") as fh:
    doc = json.load(fh)

for item in doc.get("components", []):
    asset = item.get("asset")
    if not asset:
        raise SystemExit(f"{index}: component without asset")
    src = dist / asset
    dst = snap / asset
    if not src.is_file():
        raise SystemExit(f"missing package for immutable snapshot: {asset}")
    if not dst.exists():
        shutil.copy2(src, dst)
PY
done

if [ "$CHANNEL" = "beta" ]; then
    python3 scripts/render_universal_bootstrap.py \
        --channel beta \
        --release-tag "$TAG" \
        --target aarch64-3.10 \
        --target mips-3.4 \
        --target mipsel-3.4 \
        --output "$SNAP/routerforge-beta-bootstrap.sh"
else
    python3 scripts/render_universal_bootstrap.py \
        --channel stable \
        --release-tag "$TAG" \
        --target aarch64-3.10 \
        --output "$SNAP/routerforge-stable-bootstrap.sh"
fi

sh -n "$SNAP/routerforge-${CHANNEL}-bootstrap.sh"

python3 - "$SNAP" "$CHANNEL" > "$SNAP/routerforge-${CHANNEL}-SHA256SUMS" <<'PY'
import hashlib
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
channel = sys.argv[2]

for path in sorted(root.glob("routerforge-*.ipk")):
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    print(f"{digest}  {path.name}")
PY

python3 scripts/render_release_notes.py \
    --channel "$CHANNEL" \
    --config "$CONFIG" \
    --release-tag "$TAG" \
    --final "$SNAP/routerforge-${CHANNEL}-index.json" \
    --output "$NOTES" \
    --commit "$GITHUB_SHA"

test -s "$NOTES"
test -s "$SNAP/routerforge-${CHANNEL}-SHA256SUMS"

# Validate that every versioned index points only at this immutable tag.
python3 - "$SNAP" "$CHANNEL" "$TAG" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
channel, tag = sys.argv[2:4]
prefix = f"https://github.com/Fifth-Ace/routerforge/releases/download/{tag}/"

indexes = sorted(root.glob(f"routerforge-{channel}-index*.json"))
if not indexes:
    raise SystemExit("no versioned release indexes generated")

for path in indexes:
    with path.open("r", encoding="utf-8") as fh:
        doc = json.load(fh)
    for item in doc.get("components", []):
        for key in ("url", "canonical_url"):
            value = item.get(key)
            if value and not value.startswith(prefix):
                raise SystemExit(
                    f"{path.name}: {key} escapes immutable tag: {value}"
                )
print(f"Immutable snapshot preflight: {tag}: OK")
PY

CREATE_ARGS=""
if [ "$CHANNEL" = "beta" ]; then
    CREATE_ARGS="--prerelease"
fi

# No --clobber is ever used for immutable releases.
# Existing tag was rejected above, so a published snapshot cannot be overwritten.
# shellcheck disable=SC2086
gh release create "$TAG" \
    --target "$GITHUB_SHA" \
    --title "RouterForge ${VERSION}" \
    --notes-file "$NOTES" \
    $CREATE_ARGS \
    "$SNAP"/routerforge-*.ipk \
    "$SNAP"/routerforge-"$CHANNEL"-index*.json \
    "$SNAP"/routerforge-"$CHANNEL"-SHA256SUMS \
    "$SNAP"/routerforge-"$CHANNEL"-bootstrap*.sh

echo "Immutable RouterForge release published: $TAG"
