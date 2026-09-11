#!/usr/bin/env python3
import argparse
import hashlib
import io
import json
import os
import re
from pathlib import Path
import subprocess
import tarfile
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
DIST_DEFAULT = ROOT / "dist"

LEGACY_REPOSITORY = "Fifth-Ace/dns-monitor"
CANONICAL_REPOSITORY = "Fifth-Ace/routerforge"
ALLOWED_REPOSITORIES = {LEGACY_REPOSITORY, CANONICAL_REPOSITORY}
TARGETS = {"aarch64-3.10", "mips-3.4", "mipsel-3.4"}
SAFE_ASSET_VERSION = re.compile(r"^[A-Za-z0-9._+-]+$")


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


def candidate_index_name(channel, target):
    if target == "aarch64-3.10":
        return f"routerforge-{channel}-candidate-index.json"
    return f"routerforge-{channel}-candidate-index-{target}.json"


def release_asset_version(version):
    # opkg needs "~" for prerelease ordering, but GitHub release asset names
    # must stay on the portable filename subset used by RouterForge.
    value = str(version).replace("~", "-")
    if not value or not SAFE_ASSET_VERSION.fullmatch(value):
        raise SystemExit(f"unsafe release asset version {version!r}")
    return value


BETA_RELEASE_VERSION = re.compile(r"^(.+)-beta\.([0-9]+)$")


def validate_release_train(config, channel, components):
    if channel != "beta":
        return

    release_version = str(config.get("release_version", "")).strip()
    match = BETA_RELEASE_VERSION.fullmatch(release_version)
    if not match:
        raise SystemExit(
            "beta release_version %r must match <base>-beta.<n>" % release_version
        )

    expected = "%s~beta.%s" % (match.group(1), match.group(2))
    for component in components:
        cid = component.get("id")
        version = str(component.get("version", "")).strip()
        if version != expected:
            raise SystemExit(
                "beta release train mismatch: %s version=%r, " % (cid, version)
                + "expected %r from release_version=%r" % (expected, release_version)
            )

        min_core = str(component.get("min_core_version", "")).strip()
        if min_core and min_core != expected:
            raise SystemExit(
                "beta release train mismatch: %s min_core_version=%r, " % (cid, min_core)
                + "expected %r from release_version=%r" % (expected, release_version)
            )

def parse_control_fields(raw):
    fields = {}
    current = None
    for line in raw.splitlines():
        if not line:
            current = None
            continue
        if line[:1].isspace() and current:
            fields[current] = fields[current] + "\n" + line.strip()
            continue
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        current = key.strip()
        fields[current] = value.strip()
    return fields


def package_names(raw):
    out = []
    seen = set()
    for clause in (raw or "").split(","):
        for alternative in clause.split("|"):
            token = alternative.strip()
            if not token:
                continue
            token = token.split()[0].strip("()")
            if token and token not in seen:
                seen.add(token)
                out.append(token)
    return out


def _tar_member(archive, basename):
    for member in archive.getmembers():
        if member.name.lstrip("./") == basename:
            return member
    raise SystemExit(f"IPK is missing {basename}")


def read_ipk_metadata(path):
    with tarfile.open(path, "r:gz") as outer:
        control_member = _tar_member(outer, "control.tar.gz")
        data_member = _tar_member(outer, "data.tar.gz")

        control_payload = outer.extractfile(control_member)
        data_payload = outer.extractfile(data_member)
        if control_payload is None or data_payload is None:
            raise SystemExit(f"{path.name}: invalid IPK archive")

        with tarfile.open(fileobj=io.BytesIO(control_payload.read()), mode="r:gz") as control_tar:
            control_file = _tar_member(control_tar, "control")
            handle = control_tar.extractfile(control_file)
            if handle is None:
                raise SystemExit(f"{path.name}: missing control metadata")
            fields = parse_control_fields(handle.read().decode("utf-8", errors="strict"))

        with tarfile.open(fileobj=io.BytesIO(data_payload.read()), mode="r:gz") as data_tar:
            installed_size = sum(
                member.size
                for member in data_tar.getmembers()
                if member.isfile()
            )

    return {
        "package": fields.get("Package", ""),
        "version": fields.get("Version", ""),
        "architecture": fields.get("Architecture", ""),
        "depends": package_names(fields.get("Depends", "")),
        "conflicts": package_names(fields.get("Conflicts", "")),
        "installed_size_bytes": installed_size,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--config", required=True)
    ap.add_argument("--dist", default=str(DIST_DEFAULT))
    ap.add_argument(
        "--target",
        default=os.environ.get("ROUTERFORGE_TARGET", "aarch64-3.10"),
        choices=sorted(TARGETS),
    )
    args = ap.parse_args()
    target = args.target

    config = load(args.config)
    if config.get("schema_version") != 1:
        raise SystemExit("channel config schema_version must be 1")
    channel = config.get("channel")
    if channel not in {"dev", "beta", "stable"}:
        raise SystemExit("channel must be dev, beta or stable")
    if channel == "dev" and target != "aarch64-3.10":
        raise SystemExit("dev channel is ARM64-only")

    repository = os.environ.get("GITHUB_REPOSITORY", CANONICAL_REPOSITORY).strip()
    if repository not in ALLOWED_REPOSITORIES:
        raise SystemExit(f"unexpected GITHUB_REPOSITORY {repository!r}")

    components = config.get("components") or []
    ids = [c.get("id") for c in components]
    if channel in {"dev", "beta"}:
        required = ["routerforge-core", "dns", "admin", "monitoring", "profiling"]
    else:
        required = ["routerforge-core", "dns", "admin", "monitoring", "profiling"]
    if ids != required:
        raise SystemExit(f"components for {channel} must be ordered exactly as {required}")

    validate_release_train(config, channel, components)

    dist = Path(args.dist)
    dist.mkdir(parents=True, exist_ok=True)
    for old in dist.glob(f"routerforge-*_{target}.ipk"):
        old.unlink()

    env = dict(os.environ)
    env["ROUTERFORGE_CHANNEL"] = channel
    env["ROUTERFORGE_TARGET"] = target

    for component in components:
        cid = component["id"]
        pkg = component["package"]
        version = component["version"]
        if not version or "/" in version or ".." in version:
            raise SystemExit(f"{cid}: invalid version")
        asset_version = release_asset_version(version)
        if cid == "routerforge-core":
            run(["./scripts/build-opkg.sh", version], env=env)
        elif cid == "admin":
            run(["./scripts/build-admin-opkg.sh", version], env=env)
        else:
            run(["./scripts/build-module-opkg.sh", cid, version], env=env)

        raw_asset = f"{pkg}_{version}_{target}.ipk"
        asset = f"{pkg}_{asset_version}_{target}.ipk"
        raw_path = dist / raw_asset
        path = dist / asset

        if not raw_path.is_file():
            raise SystemExit(f"missing built asset {raw_asset}")
        if raw_path != path:
            if path.exists():
                raise SystemExit(f"safe release asset already exists {asset}")
            raw_path.rename(path)

    tag = f"routerforge-{channel}"
    legacy_base = f"https://github.com/{LEGACY_REPOSITORY}/releases/download/{tag}"
    canonical_base = f"https://github.com/{repository}/releases/download/{tag}"

    entries = []
    for component in components:
        pkg = component["package"]
        version = component["version"]
        asset_version = release_asset_version(version)
        asset = f"{pkg}_{asset_version}_{target}.ipk"
        path = dist / asset
        if not path.is_file():
            raise SystemExit(f"missing built asset {asset}")

        metadata = read_ipk_metadata(path)
        if metadata["package"] != pkg:
            raise SystemExit(f"{asset}: control package {metadata['package']!r} != {pkg!r}")
        if metadata["version"] != version:
            raise SystemExit(f"{asset}: control version {metadata['version']!r} != {version!r}")
        if metadata["architecture"] != target:
            raise SystemExit(f"{asset}: control architecture {metadata['architecture']!r} != {target!r}")

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
            "architecture": metadata["architecture"],
            "size_bytes": path.stat().st_size,
            "installed_size_bytes": metadata["installed_size_bytes"],
            "depends": metadata["depends"],
            "conflicts": metadata["conflicts"],
        })

    candidate = {
        "schema_version": 1,
        "channel": channel,
        "target": target,
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "commit": os.environ.get("GITHUB_SHA", ""),
        "components": entries,
    }
    out = dist / candidate_index_name(channel, target)
    out.write_text(json.dumps(candidate, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(out)


if __name__ == "__main__":
    main()
