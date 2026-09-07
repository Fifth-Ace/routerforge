#!/bin/sh
set -eu

SHA="${1:-${GITHUB_SHA:-}}"
[ -n "$SHA" ] || {
    echo "usage: $0 <commit-sha>" >&2
    exit 64
}

rm -f \
    dist/routerforge-*.ipk \
    dist/routerforge-stable-candidate-index*.json

for target in aarch64-3.10 mips-3.4 mipsel-3.4; do
    python3 scripts/build_routerforge_channel.py \
        --config release/channels/stable.json \
        --dist dist \
        --target "$target"
done

test -s dist/routerforge-stable-candidate-index.json
test -s dist/routerforge-stable-candidate-index-mips-3.4.json
test -s dist/routerforge-stable-candidate-index-mipsel-3.4.json

sh scripts/verify-stable-promotion.sh "$SHA"

echo "Stable multiarch promotion candidate built: $SHA"
