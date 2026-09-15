#!/usr/bin/env python3
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

ACTIVE = [
    "README.md", "README_EN.md", "CONTRIBUTING.md", "SECURITY.md",
    "components/README.md", "modules/README.md", "marketplace/README.md",
    "release/channels/README.md", "docs/README.md", "docs/INSTALLATION.md",
    "docs/ARCHITECTURE.md", "docs/ARCHITECTURES.md", "docs/MODULES.md",
    "docs/MARKETPLACE.md", "docs/MANAGEMENT_V2_API.md",
    "docs/MANAGEMENT_V2_FILES_API.md", "docs/MONITORING_MIGRATION.md",
    "docs/NETWORK_TOOLS_CONSOLIDATION.md", "docs/RELEASE_PROCESS.md",
    "docs/REPOSITORY_LAYOUT.md", "docs/TROUBLESHOOTING.md",
    "docs/FRONTEND_ARCHITECTURE.md", "docs/EXECUTABLE_COMPRESSION.md",
    "docs/APP_CENTER_RELEASE_FEED_ADR.md", "docs/RELEASE_NOTES_0.7.1.md",
    "docs/RELEASE_NOTES_0.7.2.md", "docs/RELEASE_NOTES_0.8.0.md",
    "docs/VNEXT_MODULES_DEV_FOUNDATION.md", "docs/FORGEJO_FAILOVER.md",
]

for rel in ACTIVE:
    if not (ROOT / rel).is_file():
        raise SystemExit(f"missing active documentation: {rel}")

required = {
    "README.md": [
        "Stable 0.8.0", "routerforge-network-tools", "routerforge-monitoring",
        "Keenetic NDM Console", "docs/RELEASE_NOTES_0.8.0.md",
    ],
    "README_EN.md": [
        "Stable 0.8.0", "routerforge-network-tools", "routerforge-monitoring",
        "Keenetic NDM Console", "docs/RELEASE_NOTES_0.8.0.md",
    ],
    "modules/README.md": [
        "Current Stable 0.8.0 topology", "network-tools/",
        "Legacy split source directories",
    ],
    "docs/REPOSITORY_LAYOUT.md": [
        "Stable 0.8.0 release topology", "modules/network-tools/",
        "not build targets",
    ],
    "docs/VNEXT_MODULES_DEV_FOUNDATION.md": [
        "Historical / superseded", "Maintenance remains under Management",
        "Network Tools is the only new top-level module",
    ],
    "docs/NETWORK_TOOLS_CONSOLIDATION.md": [
        "routerforge-network-tools", "sole first-class network diagnostics module",
    ],
    "docs/MANAGEMENT_V2_API.md": ["mode=<entware|keenetic>", "ndmc"],
    "docs/RELEASE_PROCESS.md": ["publish_beta=false", "routerforge-stable-promotion"],
    "docs/RELEASE_NOTES_0.7.1.md": ["RouterForge 0.7.1", "Keenetic NDM Console"],
    "docs/RELEASE_NOTES_0.7.2.md": ["RouterForge 0.7.2", "routerforge-dns", "DNS hotfix"],
    "docs/RELEASE_NOTES_0.8.0.md": [
        "RouterForge 0.8.0", "routerforge-network-tools",
        "routerforge-monitoring", "publish_beta=false",
    ],
}

for rel, needles in required.items():
    text = (ROOT / rel).read_text(encoding="utf-8")
    for needle in needles:
        if needle not in text:
            raise SystemExit(f"{rel}: missing current marker {needle}")

stable = json.loads((ROOT / "release/channels/stable.json").read_text(encoding="utf-8"))
if stable.get("release_version") != "0.8.0":
    raise SystemExit("stable.json release_version mismatch")

expected = [
    "routerforge-core", "routerforge-dns", "routerforge-admin",
    "routerforge-monitoring", "routerforge-network-tools", "routerforge-profiling",
]
components = stable.get("components", [])
if [x.get("package") for x in components] != expected:
    raise SystemExit("stable.json topology mismatch")

versions = {
    "routerforge-core": ("0.8.0", ""),
    "routerforge-dns": ("0.8.0", "0.8.0"),
    "routerforge-admin": ("0.8.0", "0.8.0"),
    "routerforge-monitoring": ("0.7.1", "0.7.1"),
    "routerforge-network-tools": ("0.8.0", "0.8.0"),
    "routerforge-profiling": ("0.7.1", "0.7.1"),
}
for item in components:
    version, min_core = versions[item["package"]]
    if item.get("version") != version:
        raise SystemExit(f"stable version mismatch: {item['package']}")
    if min_core and item.get("min_core_version") != min_core:
        raise SystemExit(f"stable min_core mismatch: {item['package']}")

for rel in ("modules/system", "modules/thermal", "modules/storage", "modules/network"):
    if (ROOT / rel).exists():
        raise SystemExit(f"legacy split source directory must be absent: {rel}")

for rel in (
    "marketplace/approvals/system.json",
    "marketplace/approvals/thermal.json",
    "marketplace/approvals/storage.json",
    "marketplace/approvals/network.json",
):
    if (ROOT / rel).exists():
        raise SystemExit(f"orphan marketplace approval must be absent: {rel}")

builder = (ROOT / "scripts/build-module-opkg.sh").read_text(encoding="utf-8")
for forbidden in (
    "build_runtime_module()",
    "system) build_runtime_module",
    "thermal) build_runtime_module",
    "storage) build_runtime_module",
    "network) build_runtime_module",
):
    if forbidden in builder:
        raise SystemExit(f"legacy split build path remains: {forbidden}")

aggregate = (ROOT / "scripts/build-modules-opkg.sh").read_text(encoding="utf-8")
for required_call in (
    'build-module-opkg.sh" dns',
    'build-module-opkg.sh" monitoring',
    'build-network-tools-opkg.sh"',
    'build-module-opkg.sh" profiling',
):
    if required_call not in aggregate:
        raise SystemExit(f"aggregate module build missing: {required_call}")

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

print("DOCS_CURRENT=PASS")
print("STABLE_RELEASE_VERSION=0.8.0")
print("STABLE_COMPONENT_VERSIONS=core:0.8.0,dns:0.8.0,admin:0.8.0,monitoring:0.7.1,network-tools:0.8.0,profiling:0.7.1")
print("LEGACY_SPLIT_SOURCE_DIRS=ABSENT")
print("ORPHAN_SPLIT_APPROVALS=ABSENT")
print("ACTIVE_BUILD_TOPOLOGY=dns,monitoring,network-tools,profiling")
