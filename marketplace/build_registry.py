#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parent
SUBMISSIONS = ROOT / "submissions"
APPROVALS = ROOT / "approvals"
OUT = ROOT / "registry" / "index.json"

ALLOWED_KINDS = {"module", "integration"}
ALLOWED_METHODS = {"routerforge-release", "opkg", "structured", "manual", "official-script", "release-deploy"}
ALLOWED_STEPS = {"opkg-update", "opkg-install", "opkg-upgrade", "opkg-remove", "write-opkg-feed"}
ALLOWED_APPROVALS = {"official", "verified", "blocked", "deprecated"}


def canonical(obj):
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def digest_manifest(obj):
    return hashlib.sha256(canonical(obj)).hexdigest()


def load_json(path):
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def validate_package(value, where):
    if not value or any(ch not in "abcdefghijklmnopqrstuvwxyz0123456789-+._" for ch in value):
        raise ValueError(f"{where}: unsafe package {value!r}")


def validate_plan(plan, where):
    if not plan:
        return
    method = plan.get("method", "")
    if method not in ALLOWED_METHODS:
        raise ValueError(f"{where}: unsupported method {method!r}")
    for pkg in plan.get("packages", []):
        validate_package(pkg, where)
    if method == "structured":
        steps = plan.get("steps", [])
        if not steps:
            raise ValueError(f"{where}: structured plan has no steps")
        for idx, step in enumerate(steps):
            step_type = step.get("type")
            if step_type not in ALLOWED_STEPS:
                raise ValueError(f"{where}.steps[{idx}]: unsupported type {step_type!r}")
            for pkg in step.get("packages", []):
                validate_package(pkg, f"{where}.steps[{idx}]")
            if step_type == "write-opkg-feed":
                path = step.get("path", "")
                content = step.get("content", "").strip()
                if not path.startswith("/opt/etc/opkg/") or ".." in path:
                    raise ValueError(f"{where}.steps[{idx}]: unsafe feed path")
                if not content.startswith("src/gz ") or "https://" not in content:
                    raise ValueError(f"{where}.steps[{idx}]: invalid feed content")


def validate_manifest(obj, path):
    required = ["schema_version", "id", "kind", "name", "publisher"]
    for key in required:
        if key not in obj:
            raise ValueError(f"{path.name}: missing {key}")
    if obj["schema_version"] != 1:
        raise ValueError(f"{path.name}: schema_version must be 1")
    if obj["kind"] not in ALLOWED_KINDS:
        raise ValueError(f"{path.name}: invalid kind")
    if path.stem != obj["id"]:
        raise ValueError(f"{path.name}: filename must match id")
    if not obj["publisher"].get("id") or not obj["publisher"].get("name"):
        raise ValueError(f"{path.name}: publisher.id/name required")
    for key in ("install", "update", "remove"):
        validate_plan(obj.get(key), f"{obj['id']}.{key}")


def build():
    entries = []
    seen = set()
    approvals = {}
    for path in sorted(APPROVALS.glob("*.json")):
        approval = load_json(path)
        if approval.get("status") not in ALLOWED_APPROVALS:
            raise ValueError(f"{path.name}: invalid approval status")
        approvals[approval.get("id")] = approval

    for path in sorted(SUBMISSIONS.glob("*.json")):
        manifest = load_json(path)
        validate_manifest(manifest, path)
        mid = manifest["id"]
        if mid in seen:
            raise ValueError(f"duplicate id {mid}")
        seen.add(mid)
        digest = digest_manifest(manifest)
        approval = approvals.get(mid)
        if not approval:
            trust = {"status": "unverified", "note": "Manifest прошёл schema validation, но ещё не прошёл review RouterForge."}
        elif approval.get("manifest_sha256") != digest:
            trust = {"status": "changed", "reviewed_by": approval.get("reviewed_by", ""), "note": "Manifest изменён после последнего approval."}
        else:
            trust = {
                "status": approval["status"],
                "reviewed_by": approval.get("reviewed_by", ""),
                "note": approval.get("note", ""),
            }
        entry = dict(manifest)
        entry.pop("schema_version", None)
        entry["manifest_id"] = mid
        entry["manifest_sha256"] = digest
        entry["manifest_source"] = f"marketplace/submissions/{path.name}"
        entry["registry_source"] = "routerforge-community"
        entry["trust"] = trust
        entries.append(entry)

    entries.sort(key=lambda item: item["id"])
    revision = hashlib.sha256(canonical(entries)).hexdigest()
    return {
        "schema_version": 1,
        "registry_id": "routerforge-community",
        "brand": "RouterForge",
        "revision": revision,
        "entries": entries,
    }


def render(doc):
    return json.dumps(doc, ensure_ascii=False, indent=2, sort_keys=False) + "\n"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    try:
        doc = build()
        text = render(doc)
        if args.check:
            current = OUT.read_text(encoding="utf-8") if OUT.exists() else ""
            if current != text:
                print("marketplace/registry/index.json is out of date", file=sys.stderr)
                return 1
            print(f"RouterForge registry OK: {len(doc['entries'])} entries · {doc['revision'][:12]}")
            return 0
        OUT.parent.mkdir(parents=True, exist_ok=True)
        OUT.write_text(text, encoding="utf-8", newline="\n")
        print(f"wrote {OUT}: {len(doc['entries'])} entries · {doc['revision'][:12]}")
        return 0
    except Exception as exc:
        print(f"registry validation failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
