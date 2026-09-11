#!/usr/bin/env python3
import json
import re
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
ACTIVE = [
 "README.md","README_EN.md","CONTRIBUTING.md","SECURITY.md",
 "components/README.md","modules/README.md","marketplace/README.md","release/channels/README.md",
 "docs/README.md","docs/INSTALLATION.md","docs/ARCHITECTURE.md","docs/ARCHITECTURES.md",
 "docs/MODULES.md","docs/MARKETPLACE.md","docs/MANAGEMENT_V2_API.md","docs/MANAGEMENT_V2_FILES_API.md",
 "docs/MONITORING_MIGRATION.md","docs/RELEASE_PROCESS.md","docs/REPOSITORY_LAYOUT.md",
 "docs/TROUBLESHOOTING.md","docs/FRONTEND_ARCHITECTURE.md","docs/EXECUTABLE_COMPRESSION.md",
 "docs/APP_CENTER_RELEASE_FEED_ADR.md","docs/RELEASE_NOTES_0.7.1.md"
]
for rel in ACTIVE:
    if not (ROOT/rel).is_file():
        raise SystemExit(f"missing active documentation: {rel}")
required={
 "README.md":["Stable 0.7.1","routerforge-monitoring","Keenetic NDM Console"],
 "README_EN.md":["Stable 0.7.1","routerforge-monitoring","Keenetic NDM Console"],
 "docs/MANAGEMENT_V2_API.md":["mode=<entware|keenetic>","ndmc"],
 "docs/RELEASE_PROCESS.md":["publish_beta=false","routerforge-stable-promotion"],
 "docs/RELEASE_NOTES_0.7.1.md":["RouterForge 0.7.1","Keenetic NDM Console"],
}
for rel, needles in required.items():
    text=(ROOT/rel).read_text(encoding="utf-8")
    for needle in needles:
        if needle not in text:
            raise SystemExit(f"{rel}: missing current marker {needle}")
forbidden=["Stable 0.6 baseline","Current Dev/Beta package","The current Management UI remains read-only","0.7.1~beta.1"]
for rel in ACTIVE:
    text=(ROOT/rel).read_text(encoding="utf-8")
    for needle in forbidden:
        if needle in text:
            raise SystemExit(f"{rel}: stale text {needle}")
stable=json.loads((ROOT/"release/channels/stable.json").read_text(encoding="utf-8"))
expected=["routerforge-core","routerforge-dns","routerforge-admin","routerforge-monitoring","routerforge-profiling"]
if stable.get("release_version")!="0.7.1" or [x.get("package") for x in stable.get("components",[])]!=expected:
    raise SystemExit("stable.json topology/version mismatch")
for item in stable["components"]:
    if item.get("version")!="0.7.1":
        raise SystemExit("stable component version mismatch")
    if item["package"]!="routerforge-core" and item.get("min_core_version")!="0.7.1":
        raise SystemExit("stable min_core_version mismatch")
link_re=re.compile(r"\[[^\]]+\]\(([^)]+)\)")
for rel in ACTIVE:
    src=ROOT/rel
    for raw in link_re.findall(src.read_text(encoding="utf-8")):
        target=raw.strip().split()[0].strip("<>").split("#",1)[0]
        if not target or target.startswith(("http://","https://","mailto:","#")):
            continue
        resolved=(src.parent/target).resolve()
        try:
            resolved.relative_to(ROOT.resolve())
        except ValueError:
            raise SystemExit(f"{rel}: link escapes repository: {raw}")
        if not resolved.exists():
            raise SystemExit(f"{rel}: broken local link: {raw}")
print("DOCS_CURRENT=PASS")
print(f"ACTIVE_DOCS={len(ACTIVE)}")
print("STABLE_RELEASE_VERSION=0.7.1")
