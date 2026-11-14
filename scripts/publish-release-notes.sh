#!/bin/sh
# Only called by a verified release workflow after the release package indexes
# and installer are already coherent. The public notes index is switched LAST.
set -eu
CHANNEL="${1:-}"
case "$CHANNEL" in beta|stable) ;; *) echo 'channel must be beta or stable' >&2; exit 64;; esac
TAG="routerforge-$CHANNEL"
python3 scripts/build-release-notes-catalog.py --channel "$CHANNEL" --dist dist
INDEX="routerforge-notes-$CHANNEL-index.json"
# gh release upload can replace notes bytes, but existing immutable source
# documents must not be rewritten accidentally during future releases.
for path in dist/routerforge-notes-"$CHANNEL"-*.md; do
    [ -f "$path" ] || continue
    gh release upload "$TAG" --clobber "$path"
done
# Important: readers never observe an index pointing at missing documents.
gh release upload "$TAG" --clobber "dist/$INDEX"
echo "NOTES_PUBLISHED: PASS / $CHANNEL / INDEX_LAST"
