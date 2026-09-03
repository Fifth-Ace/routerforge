#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

NAMES = {
    "routerforge-core": "RouterForge Core",
    "routerforge-dns": "RouterForge DNS",
    "routerforge-admin": "RouterForge Control",
    "routerforge-system": "System Monitor",
    "routerforge-thermal": "Thermal Monitor",
    "routerforge-storage": "Storage Monitor",
    "routerforge-network": "Network Monitor",
    "routerforge-profiling": "Profiling",
}

ORDER = [
    "routerforge-core",
    "routerforge-dns",
    "routerforge-admin",
    "routerforge-system",
    "routerforge-thermal",
    "routerforge-storage",
    "routerforge-network",
    "routerforge-profiling",
]


def load_index(path):
    if not path:
        return None
    p = Path(path)
    if not p.is_file() or p.stat().st_size == 0:
        return None
    with p.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def by_package(doc):
    if not doc:
        return {}
    return {
        str(item.get("package", "")): item
        for item in doc.get("components", [])
        if item.get("package")
    }


def display_name(package):
    return NAMES.get(package, package)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--channel", choices=("beta", "stable"), required=True)
    parser.add_argument("--previous")
    parser.add_argument("--final", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--commit", required=True)
    args = parser.parse_args()

    previous = load_index(args.previous)
    final = load_index(args.final)
    if not final:
        raise SystemExit("final release index is missing or empty")

    current = by_package(final)
    old = by_package(previous)

    changed = []
    for package in ORDER:
        item = current.get(package)
        if not item:
            continue
        before = old.get(package, {}).get("version")
        after = item.get("version", "")
        if before != after:
            changed.append((package, before, after))

    title = "RouterForge Beta" if args.channel == "beta" else "RouterForge Stable"
    channel_text = (
        "Pre-release testing channel. Packages here are intended for validation before promotion to stable."
        if args.channel == "beta"
        else "Production channel. Packages here have passed the RouterForge beta validation flow."
    )

    lines = [
        f"# {title}",
        "",
        channel_text,
        "",
        "This is a rolling channel with independently versioned RouterForge components.",
        "",
        "## Changes in this publish",
        "",
    ]

    if changed:
        for package, before, after in changed:
            name = display_name(package)
            if before:
                lines.append(f"- **{name}:** `{before}` → `{after}`")
            else:
                lines.append(f"- **{name}:** initial channel version `{after}`")
    else:
        lines.append("- No component version changes. Channel metadata and release index were refreshed.")

    lines += [
        "",
        "## Current component versions",
        "",
        "| Component | Version |",
        "| --- | --- |",
    ]
    for package in ORDER:
        item = current.get(package)
        if item:
            lines.append(f"| {display_name(package)} | `{item.get('version', '—')}` |")

    short = args.commit[:7]
    index_name = f"routerforge-{args.channel}-index.json"
    sums_name = f"routerforge-{args.channel}-SHA256SUMS"
    lines += [
        "",
        "## Build",
        "",
        f"- Commit: [`{short}`](https://github.com/Fifth-Ace/dns-monitor/commit/{args.commit})",
        f"- Release index: `{index_name}`",
        f"- Checksums: `{sums_name}`",
        "",
        "RouterForge verifies the exact release asset and SHA256 from the channel index before package installation.",
        "",
    ]

    Path(args.output).write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
