#!/usr/bin/env python3
"""Basic public-tree hygiene without touching source or App Center metadata."""
import re
import subprocess
import sys

tracked = subprocess.check_output(["git", "ls-files", "-z"], text=False).decode("utf-8").split("\0")
forbidden = []
private_doc = re.compile(r"^(?:DEVELOPMENT[^/]*|ACTION_PLAN[^/]*|HANDOFF[^/]*|WORKLOG[^/]*|MASTER[^/]*|PRIVATE_[^/]*)\.(?:md|txt)$", re.I)
backup_suffix = re.compile(r"\.(?:backup|bak|orig|rej)$", re.I)
internal_doc_names = {
    "APP_CENTER_RELEASE_FEED_ADR.md", "CONFIG_VAULT.md", "DEV_DELIVERY.md",
    "DNS_MUTATION_SAFETY.md", "EXECUTABLE_COMPRESSION.md", "FORGEJO_FAILOVER.md",
    "FRONTEND_ARCHITECTURE.md", "MONITORING_MIGRATION.md",
    "NETWORK_TOOLS_CONSOLIDATION.md", "NFQWS2_MANAGER.md",
    "P17_NETWORK_DOCTOR_ROUTE_INSPECTOR.md", "P18_DNS_POLICY_ROUTER.md",
    "P29_ECOSYSTEM_POOL_2026-10-02.md", "PORT_ACCESS_K1.md",
    "RELEASE_PROCESS.md", "REPOSITORY_LAYOUT.md", "SUPPORT_RECOVERY_STORAGE.md",
    "THIRD_PARTY_SOURCES.md", "TRANSACTION_EVIDENCE.md", "UI_CONTRACT.md",
    "VNEXT_MODULES_DEV_FOUNDATION.md",
}
for path in tracked:
    if not path:
        continue
    low = path.lower()
    if (low.startswith((".routerforge-", "private/", "archive/"))
        or (low.startswith("routerforge-") and "/" in path)
        or private_doc.fullmatch(path)
        or backup_suffix.search(path)
        or (path.startswith("docs/") and path.rsplit("/", 1)[-1] in internal_doc_names)
    ):
        forbidden.append(path)
if forbidden:
    print("ERROR: internal/retired files in public tree:", file=sys.stderr)
    for path in sorted(forbidden):
        print(f"  {path}", file=sys.stderr)
    sys.exit(1)
print("Public repository hygiene: OK")
