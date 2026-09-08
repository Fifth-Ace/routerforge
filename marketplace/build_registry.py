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
EMBEDDED_OUT = ROOT.parent / "components" / "core" / "embedded" / "marketplace-index.json"

ALLOWED_KINDS = {"module", "integration"}
ALLOWED_METHODS = {"routerforge-release", "opkg", "structured", "manual", "official-script", "release-deploy"}
ALLOWED_STEPS = {"opkg-update", "opkg-install", "opkg-upgrade", "opkg-remove", "write-opkg-feed"}
ALLOWED_APPROVALS = {"official", "verified", "blocked", "deprecated"}
ALLOWED_WEB_MODES = {"external-only", "probe-required", "embedded-supported", "unsupported-version"}


def canonical(obj):
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def digest_manifest(obj):
    return hashlib.sha256(canonical(obj)).hexdigest()


def load_json(path):
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def validate_string_list(value, where):
    if not isinstance(value, list) or any(not isinstance(item, str) for item in value):
        raise ValueError(f"{where}: must be an array of strings")


def validate_package(value, where):
    if not value or any(ch not in "abcdefghijklmnopqrstuvwxyz0123456789-+._" for ch in value):
        raise ValueError(f"{where}: unsafe package {value!r}")


def validate_web(web, where):
    if web is None:
        return
    if not isinstance(web, dict):
        raise ValueError(f"{where}: web must be an object")
    unknown = set(web) - {"scheme", "port", "path", "mode", "embed"}
    if unknown:
        raise ValueError(f"{where}: unknown web keys {sorted(unknown)!r}")
    port = web.get("port")
    if isinstance(port, bool) or not isinstance(port, int) or not 1 <= port <= 65535:
        raise ValueError(f"{where}: web.port must be an integer between 1 and 65535")
    scheme = web.get("scheme", "")
    if scheme not in {"", "http", "https"}:
        raise ValueError(f"{where}: web.scheme must be http or https")
    path = web.get("path", "")
    if not isinstance(path, str):
        raise ValueError(f"{where}: web.path must be a string")
    if path and (not path.startswith("/") or "\r" in path or "\n" in path):
        raise ValueError(f"{where}: web.path must be an absolute local path")
    mode = web.get("mode")
    if mode not in ALLOWED_WEB_MODES:
        raise ValueError(f"{where}: invalid or missing web.mode {mode!r}")
    embed = web.get("embed", False)
    if not isinstance(embed, bool):
        raise ValueError(f"{where}: web.embed must be boolean")
    if embed and mode not in {"embedded-supported", "probe-required"}:
        raise ValueError(f"{where}: web.embed requires mode=embedded-supported or probe-required")


def validate_plan(plan, where):
    if not plan:
        return
    if not isinstance(plan, dict):
        raise ValueError(f"{where}: plan must be an object")
    method = plan.get("method", "")
    if method not in ALLOWED_METHODS:
        raise ValueError(f"{where}: unsupported method {method!r}")

    if "packages" in plan:
        validate_string_list(plan["packages"], f"{where}.packages")
    if "notes" in plan:
        validate_string_list(plan["notes"], f"{where}.notes")
    for pkg in plan.get("packages", []):
        validate_package(pkg, where)

    if method == "structured":
        steps = plan.get("steps", [])
        if not isinstance(steps, list) or not steps:
            raise ValueError(f"{where}: structured plan has no steps")
        for idx, step in enumerate(steps):
            if not isinstance(step, dict):
                raise ValueError(f"{where}.steps[{idx}]: step must be an object")
            step_type = step.get("type")
            if step_type not in ALLOWED_STEPS:
                raise ValueError(f"{where}.steps[{idx}]: unsupported type {step_type!r}")
            if "packages" in step:
                validate_string_list(step["packages"], f"{where}.steps[{idx}].packages")
            if "args" in step:
                validate_string_list(step["args"], f"{where}.steps[{idx}].args")
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
    validate_web(obj.get("web"), obj["id"])
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
            embedded = EMBEDDED_OUT.read_text(encoding="utf-8") if EMBEDDED_OUT.exists() else ""
            if current != text:
                print("marketplace/registry/index.json is out of date", file=sys.stderr)
                return 1
            if embedded != text:
                print("components/core/embedded/marketplace-index.json is out of date", file=sys.stderr)
                return 1
            print(f"RouterForge registry OK: {len(doc['entries'])} entries · {doc['revision'][:12]}")
            return 0
        OUT.parent.mkdir(parents=True, exist_ok=True)
        EMBEDDED_OUT.parent.mkdir(parents=True, exist_ok=True)
        OUT.write_text(text, encoding="utf-8", newline="\n")
        EMBEDDED_OUT.write_text(text, encoding="utf-8", newline="\n")
        print(f"wrote {OUT} and {EMBEDDED_OUT}: {len(doc['entries'])} entries · {doc['revision'][:12]}")
        return 0
    except Exception as exc:
        print(f"registry validation failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
