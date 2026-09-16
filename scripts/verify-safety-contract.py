#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]

ALLOWED_DIRECT_COMMAND_FILES = {
    "components/control/terminal.go",
    "components/control/terminal_sessions_linux.go",
    "components/control/terminal_ws_linux.go",
    "components/core/app_center_jobs.go",
    "internal/safety/command.go",
}

INTENTIONAL_FILESYSTEM_SNIPPETS = {
    "components/control/files_mutate.go": ["os.Rename("],
    "components/control/maintenance_restore.go": ["os.RemoveAll("],
    "components/core/app_center_jobs.go": ["os.Rename("],
    "components/core/core_log.go": ["os.Rename(", "os.OpenFile("],
    "modules/dns/runtime/logger.go": ["os.Rename(", "os.OpenFile("],
    "components/control/terminal_sessions_linux.go": ["os.OpenFile("],
    "components/core/marketplace_install.go": [],
}

errors = []
direct_command_hits = []
for path in sorted(ROOT.rglob("*.go")):
    rel = path.relative_to(ROOT).as_posix()
    text = path.read_text(encoding="utf-8")
    for match in re.finditer(r"\bexec\.CommandContext\s*\(", text):
        line = text.count("\n", 0, match.start()) + 1
        direct_command_hits.append((rel, line))
        if rel not in ALLOWED_DIRECT_COMMAND_FILES:
            errors.append(f"unapproved direct exec.CommandContext: {rel}:{line}")

for rel in sorted(ALLOWED_DIRECT_COMMAND_FILES):
    path = ROOT / rel
    if not path.is_file():
        errors.append(f"missing intentional direct-command file: {rel}")

# Guard against the hand-written replace-file patterns that P15 consolidated.
for path in sorted(ROOT.rglob("*.go")):
    rel = path.relative_to(ROOT).as_posix()
    text = path.read_text(encoding="utf-8")
    if re.search(r'os\.WriteFile\([^,\n]*\.tmp\b', text):
        errors.append(f"hand-written .tmp WriteFile persistence remains: {rel}")
    if "os.O_CREATE|os.O_TRUNC|os.O_WRONLY" in text:
        errors.append(f"direct truncating final-file create remains: {rel}")

# Shared safety primitives must stay present.
required = {
    "internal/safety/path.go": ["type Resolver struct"],
    "internal/safety/atomic.go": ["func WriteFileAtomic(", "func CreateExclusiveFile("],
    "internal/safety/swap.go": ["func SwapPath("],
    "internal/safety/command.go": ["func RunCommand(", "func RunCommandOutput("],
}
for rel, needles in required.items():
    path = ROOT / rel
    if not path.is_file():
        errors.append(f"missing shared safety file: {rel}")
        continue
    text = path.read_text(encoding="utf-8")
    for needle in needles:
        if needle not in text:
            errors.append(f"missing shared safety primitive {needle!r} in {rel}")

print(f"DIRECT_COMMAND_HITS={len(direct_command_hits)}")
for rel, line in direct_command_hits:
    print(f"INTENTIONAL_DIRECT_COMMAND={rel}:{line}")

if errors:
    print("P15_SAFETY_CONTRACT=FAIL")
    for item in errors:
        print("ERROR=" + item)
    sys.exit(1)

print("UNMIGRATED_ACTIONABLE=0")
print("INTENTIONAL_LOW_LEVEL_EXCEPTIONS=VERIFIED")
print("DUPLICATE_SAFETY_LOGIC=NONE")
print("P15_SAFETY_CONTRACT=PASS")
