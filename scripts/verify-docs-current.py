#!/usr/bin/env python3
"""Validate active public documentation without pinning yesterday's release."""
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

ACTIVE = [
    "README.md", "README_EN.md", "CHANGELOG.md", "CONTRIBUTING.md", "SECURITY.md",
    "THIRD_PARTY_NOTICES.md", "docs/README.md", "docs/ARCHITECTURE.md",
    "docs/ARCHITECTURES.md", "docs/INSTALLATION.md", "docs/MODULES.md",
    "docs/MARKETPLACE.md", "docs/APP_CENTER_DEVELOPER_GUIDE.md",
    "docs/MANAGEMENT_V2_API.md", "docs/MANAGEMENT_V2_FILES_API.md",
    "docs/TROUBLESHOOTING.md", "docs/VERSIONING.md",
    "marketplace/README.md", "marketplace/developer-kit/README.md",
]

for rel in ACTIVE:
    if not (ROOT / rel).is_file():
        raise SystemExit(f"missing public documentation: {rel}")

for rel in ("README.md", "README_EN.md"):
    text = (ROOT / rel).read_text(encoding="utf-8")
    for expected in ("routerforge-stable", "routerforge-beta", "routerforge-nfqws-manager",
                     "routerforge-port-access-manager", "routerforge-crowdsec-manager"):
        if expected not in text:
            raise SystemExit(f"{rel}: missing product marker {expected}")
    if "Stable 0.11.0" in text or "stable-0.11.0" in text:
        raise SystemExit(f"{rel}: obsolete hardcoded release version")

for channel in ("stable", "beta"):
    obj = json.loads((ROOT / "release" / "channels" / f"{channel}.json").read_text(encoding="utf-8"))
    version = str(obj.get("release_version", ""))
    if not re.fullmatch(r"\d+\.\d+\.\d+(?:-beta\.\d+)?", version):
        raise SystemExit(f"invalid {channel} release version: {version}")
    if obj.get("channel") != channel:
        raise SystemExit(f"wrong release channel: {channel}")

# Validate release topology without embedding stale package versions in docs.
required_base = [
    "routerforge-core", "routerforge-dns", "routerforge-admin",
    "routerforge-monitoring", "routerforge-network-tools",
    "routerforge-nfqws-manager", "routerforge-antiscan-manager",
    "routerforge-profiling",
]
required_security = ["routerforge-port-access-manager", "routerforge-crowdsec-manager"]
for channel in ("stable", "beta"):
    obj = json.loads((ROOT / "release" / "channels" / f"{channel}.json").read_text(encoding="utf-8"))
    packages = [str(c.get("package", "")) for c in obj.get("components", [])]
    if len(packages) != len(set(packages)):
        raise SystemExit(f"duplicate {channel} component package")
    if not set(required_base).issubset(packages) or not set(packages).issubset(required_base + required_security):
        raise SystemExit(f"unexpected {channel} component topology")
    if channel == "beta" and not set(required_security).issubset(packages):
        raise SystemExit("Beta security managers missing")
    for component in obj["components"]:
        version = str(component.get("version", ""))
        if not re.fullmatch(r"\d+\.\d+\.\d+(?:~(?:beta|dev)\.[A-Za-z0-9.]+)?", version):
            raise SystemExit(f"invalid {channel} component version: {component.get('package')}")

# Keep the existing safety checks for retired split modules and aggregate builds.
for rel in ("modules/system", "modules/thermal", "modules/storage", "modules/network"):
    if (ROOT / rel).exists():
        raise SystemExit(f"retired split source directory remains: {rel}")
for rel in ("marketplace/approvals/system.json", "marketplace/approvals/thermal.json",
            "marketplace/approvals/storage.json", "marketplace/approvals/network.json"):
    if (ROOT / rel).exists():
        raise SystemExit(f"orphan split-module approval remains: {rel}")
builder = (ROOT / "scripts" / "build-module-opkg.sh").read_text(encoding="utf-8")
for forbidden in ("build_runtime_module()", "system) build_runtime_module",
                  "thermal) build_runtime_module", "storage) build_runtime_module",
                  "network) build_runtime_module"):
    if forbidden in builder:
        raise SystemExit(f"retired split build path remains: {forbidden}")
aggregate = (ROOT / "scripts" / "build-modules-opkg.sh").read_text(encoding="utf-8")
for required in ('build-module-opkg.sh" dns', 'build-module-opkg.sh" monitoring',
                 'build-network-tools-opkg.sh"', 'build-module-opkg.sh" nfqws-manager',
                 'build-module-opkg.sh" antiscan-manager', 'build-module-opkg.sh" profiling'):
    if required not in aggregate:
        raise SystemExit(f"aggregate module build missing: {required}")

link_re = re.compile(r"\[[^\]]+\]\(([^)]+)\)")
for rel in ACTIVE:
    src = ROOT / rel
    for raw in link_re.findall(src.read_text(encoding="utf-8")):
        target = raw.strip().split()[0].strip("<>").split("#", 1)[0]
        if not target or target.startswith(("http://", "https://", "mailto:", "#")):
            continue
        resolved = (src.parent / target).resolve()
        try:
            resolved.relative_to(ROOT.resolve())
        except ValueError:
            raise SystemExit(f"{rel}: link escapes repository: {raw}")
        if not resolved.exists():
            raise SystemExit(f"{rel}: broken local link: {raw}")

for folder in ("archive",):
    if (ROOT / folder).exists():
        raise SystemExit(f"retired directory still present: {folder}")

for rel in (
    "docs/DEV_DELIVERY.md", "docs/P17_NETWORK_DOCTOR_ROUTE_INSPECTOR.md",
    "docs/P18_DNS_POLICY_ROUTER.md", "docs/P29_ECOSYSTEM_POOL_2026-10-02.md",
    "docs/RELEASE_PROCESS.md", "docs/VNEXT_MODULES_DEV_FOUNDATION.md",
):
    if (ROOT / rel).exists():
        raise SystemExit(f"internal development note still tracked: {rel}")

renderer = (ROOT / "scripts" / "render_release_notes.py").read_text(encoding="utf-8")
for marker in ('if args.channel == "stable":', 'RELEASE_NOTES_{release_version}.md', 'write_bytes(canonical.read_bytes())'):
    if marker not in renderer:
        raise SystemExit("canonical stable release notes renderer changed")

print("DOCS_CURRENT=PASS")
print("PUBLIC_DOCS=PASS")
print("LOCAL_LINKS=PASS")
print("CHANNEL_RELEASE_MANIFESTS=PASS")
