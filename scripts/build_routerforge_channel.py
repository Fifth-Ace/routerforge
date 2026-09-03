#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
DIST_DEFAULT = ROOT / "dist"

LEGACY_REPOSITORY = "Fifth-Ace/dns-monitor"
CANONICAL_REPOSITORY = "Fifth-Ace/routerforge"
ALLOWED_REPOSITORIES = {LEGACY_REPOSITORY, CANONICAL_REPOSITORY}

def load(path):
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)

def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

def run(cmd, env=None):
    subprocess.run(cmd, cwd=ROOT, env=env, check=True)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--config", required=True)
    ap.add_argument("--dist", default=str(DIST_DEFAULT))
    args = ap.parse_args()

    config = load(args.config)
    if config.get("schema_version") != 1:
        raise SystemExit("channel config schema_version must be 1")
    channel = config.get("channel")
    if channel not in {"beta", "stable"}:
        raise SystemExit("channel must be beta or stable")

    repository = os.environ.get("GITHUB_REPOSITORY", CANONICAL_REPOSITORY).strip()
    if repository not in ALLOWED_REPOSITORIES:
        raise SystemExit(f"unexpected GITHUB_REPOSITORY {repository!r}")

    components = config.get("components") or []
    ids = [c.get("id") for c in components]
    required = ["routerforge-core", "dns", "admin", "system", "thermal", "storage", "network", "profiling"]
    if ids != required:
        raise SystemExit(f"components must be ordered exactly as {required}")

    dist = Path(args.dist)
    dist.mkdir(parents=True, exist_ok=True)
    for old in dist.glob("routerforge-*.ipk"):
        old.unlink()

    env = dict(os.environ)
    env["ROUTERFORGE_CHANNEL"] = channel

    for component in components:
        cid = component["id"]
        version = component["version"]
        if not version or "/" in version or ".." in version:
            raise SystemExit(f"{cid}: invalid version")
        if cid == "routerforge-core":
            run(["./scripts/build-opkg.sh", version], env=env)
        elif cid == "admin":
            run(["./scripts/build-admin-opkg.sh", version], env=env)
        else:
            run(["./scripts/build-module-opkg.sh", cid, version], env=env)

    tag = f"routerforge-{channel}"
    legacy_base = f"https://github.com/{LEGACY_REPOSITORY}/releases/download/{tag}"
    canonical_base = f"https://github.com/{repository}/releases/download/{tag}"

    entries = []
    for component in components:
        pkg = component["package"]
        version = component["version"]
        asset = f"{pkg}_{version}_aarch64-3.10.ipk"
        path = dist / asset
        if not path.is_file():
            raise SystemExit(f"missing built asset {asset}")
        entries.append({
            "id": component["id"],
            "package": pkg,
            "version": version,
            # Keep url on the historical repository indefinitely so pre-bridge
            # Core versions can still consume new indexes after the rename.
            "url": f"{legacy_base}/{asset}",
            # New Core versions prefer canonical_url. Before rename this equals
            # url; after rename CI automatically emits Fifth-Ace/routerforge.
            "canonical_url": f"{canonical_base}/{asset}",
            "asset": asset,
            "sha256": sha256(path),
            "min_core_version": component.get("min_core_version", ""),
        })

    candidate = {
        "schema_version": 1,
        "channel": channel,
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "commit": os.environ.get("GITHUB_SHA", ""),
        "components": entries,
    }
    out = dist / f"routerforge-{channel}-candidate-index.json"
    out.write_text(json.dumps(candidate, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(out)

if __name__ == "__main__":
    main()
