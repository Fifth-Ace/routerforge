#!/bin/sh
set -eu

TAG="${1:-routerforge-beta}"
TMP="${ROUTERFORGE_VERIFY_TMP:-/tmp/routerforge-beta-verify-$$}"

cleanup() {
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

mkdir -p "$TMP"

ASSETS="$TMP/assets.txt"

gh release view "$TAG" --json assets --jq '.assets[].name' |
    sort -u > "$ASSETS"

require_asset() {
    asset="$1"
    if ! grep -Fxq "$asset" "$ASSETS"; then
        echo "missing release asset: $asset" >&2
        exit 1
    fi
}

for asset in \
    routerforge-beta-index.json \
    routerforge-beta-index-mips-3.4.json \
    routerforge-beta-index-mipsel-3.4.json \
    routerforge-beta-SHA256SUMS \
    routerforge-beta-bootstrap.sh \
    routerforge-beta-bootstrap-aarch64-3.10.sh \
    routerforge-beta-bootstrap-mips-3.4.sh \
    routerforge-beta-bootstrap-mipsel-3.4.sh
do
    require_asset "$asset"
done

gh release download "$TAG" \
    -D "$TMP" \
    -p 'routerforge-beta-index*.json' \
    -p 'routerforge-beta-SHA256SUMS' \
    -p 'routerforge-beta-bootstrap*.sh' \
    >/dev/null

python3 - "$TMP" "$ASSETS" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
asset_list = pathlib.Path(sys.argv[2])
published = {
    line.strip()
    for line in asset_list.read_text(encoding="utf-8").splitlines()
    if line.strip()
}

targets = {
    "aarch64-3.10": root / "routerforge-beta-index.json",
    "mips-3.4": root / "routerforge-beta-index-mips-3.4.json",
    "mipsel-3.4": root / "routerforge-beta-index-mipsel-3.4.json",
}

all_ipks = set()

for target, path in targets.items():
    with path.open("r", encoding="utf-8") as fh:
        doc = json.load(fh)

    if doc.get("schema_version") != 1:
        raise SystemExit(f"{path.name}: unexpected schema_version")
    if doc.get("channel") != "beta":
        raise SystemExit(f"{path.name}: unexpected channel")
    if doc.get("target") != target:
        raise SystemExit(
            f"{path.name}: target={doc.get('target')!r}, expected {target!r}"
        )

    components = doc.get("components")
    if not isinstance(components, list) or len(components) != 8:
        raise SystemExit(
            f"{path.name}: expected 8 components, got "
            f"{len(components) if isinstance(components, list) else 'invalid'}"
        )

    seen_packages = set()

    for item in components:
        package = item.get("package")
        asset = item.get("asset")
        sha256 = item.get("sha256")
        url = item.get("url")
        canonical = item.get("canonical_url")

        if not package or package in seen_packages:
            raise SystemExit(f"{path.name}: invalid/duplicate package {package!r}")
        seen_packages.add(package)

        expected_suffix = f"_{target}.ipk"
        if not isinstance(asset, str) or not asset.endswith(expected_suffix):
            raise SystemExit(
                f"{path.name}: asset {asset!r} does not match target {target}"
            )

        if asset not in published:
            raise SystemExit(f"{path.name}: referenced IPK not published: {asset}")

        if not isinstance(sha256, str) or len(sha256) != 64:
            raise SystemExit(f"{path.name}: invalid sha256 for {package}")

        for key, value in (("url", url), ("canonical_url", canonical)):
            if value and not value.endswith("/" + asset):
                raise SystemExit(
                    f"{path.name}: {key} does not end with asset for {package}"
                )

        all_ipks.add(asset)

if len(all_ipks) != 24:
    raise SystemExit(f"expected 24 target-specific IPKs, got {len(all_ipks)}")

sums_path = root / "routerforge-beta-SHA256SUMS"
sum_assets = set()
for raw in sums_path.read_text(encoding="utf-8").splitlines():
    line = raw.strip()
    if not line:
        continue
    parts = line.split()
    if len(parts) < 2:
        raise SystemExit("invalid SHA256SUMS line")
    digest, asset = parts[0], parts[-1].lstrip("*")
    if len(digest) != 64:
        raise SystemExit(f"invalid SHA256SUMS digest for {asset}")
    sum_assets.add(asset)

if sum_assets != all_ipks:
    missing = sorted(all_ipks - sum_assets)
    extra = sorted(sum_assets - all_ipks)
    raise SystemExit(
        f"SHA256SUMS asset mismatch: missing={missing}, extra={extra}"
    )

print("Published beta indexes/assets: OK")
print("Published target IPKs: 24")
PY

for bootstrap in \
    "$TMP/routerforge-beta-bootstrap.sh" \
    "$TMP/routerforge-beta-bootstrap-aarch64-3.10.sh" \
    "$TMP/routerforge-beta-bootstrap-mips-3.4.sh" \
    "$TMP/routerforge-beta-bootstrap-mipsel-3.4.sh"
do
    sh -n "$bootstrap"
done

grep -Fq 'aarch64-3.10' "$TMP/routerforge-beta-bootstrap.sh"
grep -Fq 'mips-3.4' "$TMP/routerforge-beta-bootstrap.sh"
grep -Fq 'mipsel-3.4' "$TMP/routerforge-beta-bootstrap.sh"

grep -Fq "TARGET='aarch64-3.10'" \
    "$TMP/routerforge-beta-bootstrap-aarch64-3.10.sh"
grep -Fq "TARGET='mips-3.4'" \
    "$TMP/routerforge-beta-bootstrap-mips-3.4.sh"
grep -Fq "TARGET='mipsel-3.4'" \
    "$TMP/routerforge-beta-bootstrap-mipsel-3.4.sh"

echo "Published beta bootstrap syntax/targets: OK"
echo "PUBLISHED BETA MULTIARCH VERIFY: PASS"
