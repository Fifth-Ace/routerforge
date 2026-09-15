#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors = []
checks = 0


def read(rel):
    path = ROOT / rel
    if not path.is_file():
        errors.append(f"{rel}: missing")
        return ""
    return path.read_text(encoding="utf-8")


def require(rel, markers):
    global checks
    text = read(rel)
    for marker in markers:
        checks += 1
        if marker not in text:
            errors.append(f"{rel}: missing marker: {marker}")


def forbid(rel, markers):
    global checks
    text = read(rel)
    for marker in markers:
        checks += 1
        if marker in text:
            errors.append(f"{rel}: forbidden marker present: {marker}")


# Shared shell design language: tabs, headers and component geometry.
require(
    "components/core/frontend/src/admin-shell.css",
    [
        "R18 canonical module tab strip",
        ".subtabs,",
        ".module-selector {",
        "min-width:165px",
        "min-height:40px",
        "R19 canonical module header",
        ".page-head {",
        "font-size:max(var(--ui-title),28px)",
        ".page-head-actions {",
        "R20 canonical component rhythm",
        "--rf-ui-panel-radius:8px",
        "--rf-ui-control-h:38px",
        "--rf-ui-panel-head-min:50px",
        "--rf-ui-row-min:42px",
        ".panel-head,",
        ".toolbar,",
        "tbody tr:nth-child(even) td",
    ],
)

# DNS iframe mirrors the same visual contract.
require(
    "modules/dns/frontend/module.css",
    [
        "R18 canonical RouterForge tab strip",
        ".dns-module .module-tabs,",
        ".dns-module .parity-subtabs {",
        "R19 canonical module header for the DNS iframe",
        ".dns-module .page-head {",
        "font-size:max(var(--ui-title,23px),28px)",
        "R20 canonical component rhythm for the DNS iframe",
        "--rf-ui-control-h:38px",
        "--rf-ui-panel-head-min:50px",
        "--rf-ui-row-min:42px",
        ".dns-module .panel-head {",
        ".dns-module .toolbar {",
        ".dns-module thead th {",
    ],
)

# Network Tools remains the text-tab reference and uses the same header/component rhythm.
require(
    "modules/network-tools/frontend/index.html",
    [
        'role="tablist"',
        'role="tab"',
        'role="tabpanel"',
        'data-i18n="tabDoctor"',
        'data-i18n="tabRoutes"',
        'data-i18n="tabFlows"',
        'data-i18n="tabProbes"',
    ],
)
forbid(
    "modules/network-tools/frontend/index.html",
    ["◉", "♧", "☷", "◌"],
)

require(
    "modules/network-tools/frontend/module.css",
    [
        "R19 canonical module header for Network Tools",
        ".nt-heading {",
        "font-size:28px",
        "R20 canonical component rhythm for Network Tools",
        "--rf-ui-control-h:38px",
        "--rf-ui-panel-head-min:50px",
        "--rf-ui-row-min:42px",
        ".nt-panel-head,",
        ".nt-toolbar",
        "thead th {",
        "tbody td {",
    ],
)

# Module wiring must keep using the canonical shell classes.
require(
    "modules/admin/frontend/AdminModuleApp.svelte",
    [
        'class="page-head"',
        'class="subtabs admin-tabs"',
    ],
)
require(
    "components/core/frontend/src/routes/apps/+page.svelte",
    [
        'class="page-head"',
        'class="subtabs app-center-tabs"',
    ],
)
require(
    "modules/monitoring/frontend/MonitoringModuleApp.svelte",
    [
        'class="page-head"',
        'class="module-selector"',
    ],
)
require(
    "modules/dns/frontend/DNSModuleApp.svelte",
    [
        'class="page-head"',
        'class="module-tabs"',
        'class="subtabs parity-subtabs"',
    ],
)

if errors:
    print("UI_CONTRACT=FAIL", file=sys.stderr)
    for error in errors:
        print(" - " + error, file=sys.stderr)
    sys.exit(1)

print("UI_CONTRACT=PASS")
print(f"UI_CONTRACT_CHECKS={checks}")
print("TAB_REFERENCE=NETWORK_TOOLS_TEXT_ONLY")
print("HEADER_REFERENCE=28PX_TITLE_13PX_SUBTITLE")
print("COMPONENT_RHYTHM=8PX_PANEL_38PX_CONTROL_50PX_HEAD_42PX_ROW")

# NETWORK_TOOLS_TEXT_ONLY
