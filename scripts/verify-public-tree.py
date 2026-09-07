#!/usr/bin/env python3
import re
import subprocess
import sys

tracked = subprocess.check_output(
    ["git", "ls-files", "-z"],
    text=False,
).decode("utf-8").split("\0")

forbidden = []

root_private_doc = re.compile(
    r"^(?:DEVELOPMENT[^/]*|ACTION_PLAN[^/]*|HANDOFF[^/]*|WORKLOG[^/]*|"
    r"MASTER[^/]*|PRIVATE_[^/]*)\.(?:md|txt)$",
    re.IGNORECASE,
)

backup_suffix = re.compile(r"\.(?:backup|bak|orig|rej)$", re.IGNORECASE)

for path in tracked:
    if not path:
        continue

    low = path.lower()

    if low.startswith(".routerforge-"):
        forbidden.append(path)
        continue

    if low.startswith("routerforge-") and "/" in path:
        forbidden.append(path)
        continue

    if low.startswith("private/"):
        forbidden.append(path)
        continue

    if root_private_doc.fullmatch(path):
        forbidden.append(path)
        continue

    if backup_suffix.search(path):
        forbidden.append(path)
        continue

if forbidden:
    print("ERROR: private/local RouterForge work products are tracked:", file=sys.stderr)
    for path in sorted(forbidden):
        print(f"  {path}", file=sys.stderr)
    sys.exit(1)

print("Public repository hygiene: OK")
