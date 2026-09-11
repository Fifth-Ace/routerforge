#!/bin/sh
set -eu

SHA="${1:-${GITHUB_SHA:-}}"
[ -n "$SHA" ] || {
    echo "usage: $0 <commit-sha>" >&2
    exit 64
}

for target in aarch64-3.10 mips-3.4 mipsel-3.4; do
    case "$target" in
        aarch64-3.10)
            INDEX=dist/routerforge-stable-candidate-index.json
            ;;
        *)
            INDEX="dist/routerforge-stable-candidate-index-${target}.json"
            ;;
    esac

    test -s "$INDEX"

    python3 - "$INDEX" "$target" "$SHA" dist <<'PY'
import hashlib
import io
import json
import sys
import tarfile
from pathlib import Path

index_path, target, sha, dist_path = sys.argv[1:5]
dist = Path(dist_path)

with open(index_path, "r", encoding="utf-8") as fh:
    doc = json.load(fh)

assert doc["schema_version"] == 1
assert doc["channel"] == "stable"
assert doc["target"] == target
assert doc["commit"] == sha

components = doc.get("components") or []
assert len(components) == 5, (target, len(components))

expected_ids = {
    "routerforge-core",
    "dns",
    "admin",
    "monitoring",
    "profiling",
}
actual_ids = {item.get("id") for item in components}
assert actual_ids == expected_ids, (target, actual_ids)

for item in components:
    asset = item["asset"]
    expected_sha = item["sha256"].lower()
    path = dist / asset

    assert path.is_file(), f"{target}: missing {asset}"

    actual_sha = hashlib.sha256(path.read_bytes()).hexdigest()
    assert actual_sha == expected_sha, (
        f"{target}: {asset}: sha256 mismatch: "
        f"{actual_sha} != {expected_sha}"
    )

    with tarfile.open(path, "r:gz") as outer:
        member = next(
            (m for m in outer.getmembers()
             if m.name.lstrip("./") == "control.tar.gz"),
            None,
        )
        assert member is not None, f"{asset}: control.tar.gz missing"
        raw_control = outer.extractfile(member).read()

    with tarfile.open(fileobj=io.BytesIO(raw_control), mode="r:gz") as control_tar:
        member = next(
            (m for m in control_tar.getmembers()
             if m.name.lstrip("./") == "control"),
            None,
        )
        assert member is not None, f"{asset}: control file missing"
        control = control_tar.extractfile(member).read().decode("utf-8", "replace")

    architecture = ""
    for line in control.splitlines():
        if line.startswith("Architecture:"):
            architecture = line.split(":", 1)[1].strip()
            break

    assert architecture == target, (
        f"{asset}: Architecture={architecture!r}, expected {target!r}"
    )

print(f"Stable promotion candidate verified: {target}")
PY
done

echo "Stable multiarch promotion candidate verified: $SHA"
