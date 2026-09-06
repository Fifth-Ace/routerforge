#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

TARGETS = {"aarch64-3.10", "mips-3.4", "mipsel-3.4"}

def load(path):
    if not path or not Path(path).is_file():
        return None
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--candidate", required=True)
    ap.add_argument("--previous")
    ap.add_argument("--output", required=True)
    ap.add_argument("--changed-list", required=True)
    ap.add_argument("--checksums", required=True)
    args = ap.parse_args()

    candidate = load(args.candidate)
    previous = load(args.previous)
    if not candidate:
        raise SystemExit("candidate index missing")
    channel = candidate["channel"]
    target = candidate.get("target")
    if target not in TARGETS:
        raise SystemExit(f"candidate index has unsupported target {target!r}")

    old_by_id = {}
    if previous and previous.get("schema_version") == 1 and previous.get("channel") == channel:
        previous_target = previous.get("target")
        if not previous_target and target == "aarch64-3.10":
            previous_target = target
        if previous_target != target:
            raise SystemExit(
                f"previous index target {previous.get('target')!r} does not match candidate target {target!r}"
            )
        old_by_id = {x["id"]: x for x in previous.get("components", [])}

    final = dict(candidate)
    merged = []
    changed = []
    for current in candidate.get("components", []):
        old = old_by_id.get(current["id"])
        if old and old.get("version") == current.get("version"):
            old_sha = str(old.get("sha256", "")).strip()
            current_sha = str(current.get("sha256", "")).strip()
            old_asset = str(old.get("asset", "")).strip()
            current_asset = str(current.get("asset", "")).strip()

            if old_sha != current_sha or old_asset != current_asset:
                raise SystemExit(
                    f"{current['id']}: package version {current.get('version')!r} "
                    "already exists with a different binary identity; "
                    "bump the component version before publishing"
                )

            preserved = dict(old)
            # Preserve the already-published same-version binary identity, but
            # normalize repository metadata from the current candidate. This
            # prevents legacy repository URLs from surviving channel merges.
            for key in ("url", "canonical_url"):
                if current.get(key):
                    preserved[key] = current[key]
            merged.append(preserved)
        else:
            merged.append(current)
            changed.append(current["asset"])
    final["components"] = merged

    Path(args.output).write_text(
        json.dumps(final, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    Path(args.changed_list).write_text(
        "".join(asset + "\n" for asset in changed),
        encoding="utf-8",
    )
    Path(args.checksums).write_text(
        "".join(f"{item['sha256']}  {item['asset']}\n" for item in merged),
        encoding="utf-8",
    )
    print(f"{channel}/{target}: {len(changed)} changed component(s)")

if __name__ == "__main__":
    main()
