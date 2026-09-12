#!/usr/bin/env python3
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

ACTIVE = [
    "README.md",
    "README_EN.md",
    "CONTRIBUTING.md",
    "SECURITY.md",
    "components/README.md",
    "modules/README.md",
    "marketplace/README.md",
    "release/channels/README.md",
    "docs/README.md",
    "docs/INSTALLATION.md",
    "docs/ARCHITECTURE.md",
    "docs/ARCHITECTURES.md",
    "docs/MODULES.md",
    "docs/MARKETPLACE.md",
    "docs/MANAGEMENT_V2_API.md",
    "docs/MANAGEMENT_V2_FILES_API.md",
    "docs/MONITORING_MIGRATION.md",
    "docs/RELEASE_PROCESS.md",
    "docs/REPOSITORY_LAYOUT.md",
    "docs/TROUBLESHOOTING.md",
    "docs/FRONTEND_ARCHITECTURE.md",
    "docs/EXECUTABLE_COMPRESSION.md",
    "docs/APP_CENTER_RELEASE_FEED_ADR.md",
    "docs/RELEASE_NOTES_0.7.1.md",
    "docs/RELEASE_NOTES_0.7.2.md",
]

for rel in ACTIVE:
    if not (ROOT / rel).is_file():
        raise SystemExit(f"missing active documentation: {rel}")

required = {
    "README.md": [
        "Stable 0.7.2",
        "routerforge-dns",
        "routerforge-monitoring",
        "Keenetic NDM Console",
        "docs/RELEASE_NOTES_0.7.2.md",
    ],
    "README_EN.md": [
        "Stable 0.7.2",
        "routerforge-dns",
        "routerforge-monitoring",
        "Keenetic NDM Console",
        "docs/RELEASE_NOTES_0.7.2.md",
    ],
    "docs/MANAGEMENT_V2_API.md": [
        "mode=<entware|keenetic>",
        "ndmc",
    ],
    "docs/RELEASE_PROCESS.md": [
        "publish_beta=false",
        "routerforge-stable-promotion",
    ],
    "docs/RELEASE_NOTES_0.7.1.md": [
        "RouterForge 0.7.1",
        "Keenetic NDM Console",
    ],
    "docs/RELEASE_NOTES_0.7.2.md": [
        "RouterForge 0.7.2",
        "routerforge-dns",
        "DNS hotfix",
    ],
}

for rel, needles in required.items():
    text = (ROOT / rel).read_text(encoding="utf-8")
    for needle in needles:
        if needle not in text:
            raise SystemExit(f"{rel}: missing current marker {needle}")

forbidden = [
    "Stable 0.6 baseline",
    "Current Dev/Beta package",
    "The current Management UI remains read-only",
    "0.7.1~beta.1",
]

for rel in ACTIVE:
    text = (ROOT / rel).read_text(encoding="utf-8")
    for needle in forbidden:
        if needle in text:
            raise SystemExit(f"{rel}: stale text {needle}")

stable = json.loads(
    (ROOT / "release/channels/stable.json").read_text(encoding="utf-8")
)

expected_packages = [
    "routerforge-core",
    "routerforge-dns",
    "routerforge-admin",
    "routerforge-monitoring",
    "routerforge-profiling",
]

if stable.get("release_version") != "0.7.2":
    raise SystemExit("stable.json release_version mismatch")

components = stable.get("components", [])
if [item.get("package") for item in components] != expected_packages:
    raise SystemExit("stable.json topology mismatch")

expected_versions = {
    "routerforge-core": "0.7.1",
    "routerforge-dns": "0.7.2",
    "routerforge-admin": "0.7.1",
    "routerforge-monitoring": "0.7.1",
    "routerforge-profiling": "0.7.1",
}

for item in components:
    package = item.get("package")
    expected_version = expected_versions.get(package)
    if expected_version is None:
        raise SystemExit(f"unexpected stable package {package}")
    if item.get("version") != expected_version:
        raise SystemExit(
            f"stable component version mismatch: "
            f"{package}={item.get('version')!r}, expected {expected_version!r}"
        )

    if package != "routerforge-core":
        if item.get("min_core_version") != "0.7.1":
            raise SystemExit(
                f"stable min_core_version mismatch: "
                f"{package}={item.get('min_core_version')!r}"
            )

link_re = re.compile(r"\[[^\]]+\]\(([^)]+)\)")

for rel in ACTIVE:
    src = ROOT / rel
    for raw in link_re.findall(src.read_text(encoding="utf-8")):
        target = raw.strip().split()[0].strip("<>").split("#", 1)[0]
        if not target or target.startswith(
            ("http://", "https://", "mailto:", "#")
        ):
            continue

        resolved = (src.parent / target).resolve()
        try:
            resolved.relative_to(ROOT.resolve())
        except ValueError:
            raise SystemExit(f"{rel}: link escapes repository: {raw}")

        if not resolved.exists():
            raise SystemExit(f"{rel}: broken local link: {raw}")

print("DOCS_CURRENT=PASS")
print(f"ACTIVE_DOCS={len(ACTIVE)}")
print("STABLE_RELEASE_VERSION=0.7.2")
print(
    "STABLE_COMPONENT_VERSIONS="
    "core:0.7.1,dns:0.7.2,admin:0.7.1,"
    "monitoring:0.7.1,profiling:0.7.1"
)
