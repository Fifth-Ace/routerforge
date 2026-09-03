#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

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

    old_by_id = {}
    if previous and previous.get("schema_version") == 1 and previous.get("channel") == channel:
        old_by_id = {x["id"]: x for x in previous.get("components", [])}

    final = dict(candidate)
    merged = []
    changed = []
    for current in candidate.get("components", []):
        old = old_by_id.get(current["id"])
        if old and old.get("version") == current.get("version"):
            preserved = dict(old)
            # Metadata can move to the renamed repository without rebuilding or
            # replacing a same-version binary. Legacy url remains untouched.
            if current.get("canonical_url"):
                preserved["canonical_url"] = current["canonical_url"]
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
    print(f"{channel}: {len(changed)} changed component(s)")

if __name__ == "__main__":
    main()
