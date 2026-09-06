#!/usr/bin/env python3
import argparse
import json
import os
from pathlib import Path

NAMES = {
    "routerforge-core": ("RouterForge Core", "RouterForge Core"),
    "routerforge-dns": ("RouterForge DNS", "RouterForge DNS"),
    "routerforge-admin": ("RouterForge Control", "RouterForge Control"),
    "routerforge-system": ("System Monitor", "System Monitor"),
    "routerforge-thermal": ("Thermal Monitor", "Thermal Monitor"),
    "routerforge-storage": ("Storage Monitor", "Storage Monitor"),
    "routerforge-network": ("Network Monitor", "Network Monitor"),
    "routerforge-profiling": ("Profiling", "Profiling"),
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

ALLOWED_REPOSITORIES = {"Fifth-Ace/routerforge", "Fifth-Ace/dns-monitor"}


def repository_name():
    value = os.environ.get("GITHUB_REPOSITORY", "Fifth-Ace/routerforge").strip()
    return value if value in ALLOWED_REPOSITORIES else "Fifth-Ace/routerforge"


def load(path):
    p = Path(path)
    if not p.is_file() or p.stat().st_size == 0:
        raise SystemExit(f"missing or empty JSON file: {path}")
    with p.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def by_package(doc):
    return {
        str(item.get("package", "")): item
        for item in doc.get("components", [])
        if item.get("package")
    }


def name(package, lang):
    pair = NAMES.get(package, (package, package))
    return pair[0] if lang == "ru" else pair[1]


def notes_for(config, lang):
    notes = config.get("release_notes") or {}
    doc = notes.get(lang) or {}
    if not isinstance(doc, dict):
        raise SystemExit(f"release_notes.{lang} must be an object")
    return doc


def append_items(lines, heading, items):
    if not items:
        return
    lines += [heading, ""]
    for item in items:
        text = " ".join(str(item).split())
        if text:
            lines.append(f"- {text}")
    lines.append("")


def append_versions(lines, current, lang):
    heading = "## Текущие версии компонентов" if lang == "ru" else "## Current component versions"
    component_label = "Компонент" if lang == "ru" else "Component"
    version_label = "Версия" if lang == "ru" else "Version"

    lines += [
        heading,
        "",
        f"| {component_label} | {version_label} |",
        "| --- | --- |",
    ]
    for package in ORDER:
        item = current.get(package)
        if item:
            lines.append(f"| {name(package, lang)} | `{item.get('version', '—')}` |")
    lines.append("")


def append_install(lines, channel, release_tag, lang):
    channel_name = "Stable" if channel == "stable" else "Beta"
    heading = "## Установка" if lang == "ru" else "## Installation"
    intro = (
        f"Свежая установка **RouterForge {channel_name}**:"
        if lang == "ru"
        else f"Fresh **RouterForge {channel_name}** install:"
    )
    repo = repository_name()
    tag = release_tag or f"routerforge-{channel}"

    lines += [
        heading,
        "",
        intro,
        "",
        "```sh",
        f"/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/{repo}/releases/download/{tag}/routerforge-{channel}-bootstrap.sh | sh",
        "```",
        "",
    ]


def append_build(lines, channel, commit, release_version, lang):
    heading = "## Сборка и проверка" if lang == "ru" else "## Build and verification"
    commit_label = "Коммит" if lang == "ru" else "Commit"
    release_label = "Версия RouterForge" if lang == "ru" else "RouterForge release"
    archive_label = "Архивный релиз" if lang == "ru" else "Immutable release"
    repo = repository_name()
    short = commit[:7]
    tag = f"routerforge-v{release_version}"

    lines += [
        heading,
        "",
        f"- {release_label}: `{release_version}`",
        f"- {archive_label}: `{tag}`",
        f"- {commit_label}: [`{short}`](https://github.com/{repo}/commit/{commit})",
        f"- Release index: `routerforge-{channel}-index.json`",
        f"- Bootstrap: `routerforge-{channel}-bootstrap.sh`",
        "",
    ]


def append_language(lines, config, final, channel, commit, release_tag, lang):
    release_version = str(config.get("release_version", "")).strip()
    if not release_version:
        raise SystemExit("release_version is missing")

    notes = notes_for(config, lang)
    current = by_package(final)

    if lang == "ru":
        lines += [
            "## 🇷🇺 Русский",
            "",
            f"**RouterForge {release_version}**",
            "",
        ]
        append_items(lines, "## Что нового", notes.get("new"))
        append_items(lines, "## Исправления", notes.get("fixes"))
        append_items(lines, "## Совместимость", notes.get("compatibility"))
        append_items(lines, "## Технические изменения", notes.get("technical"))
    else:
        lines += [
            "## 🇬🇧 English",
            "",
            f"**RouterForge {release_version}**",
            "",
        ]
        append_items(lines, "## What's new", notes.get("new"))
        append_items(lines, "## Fixes", notes.get("fixes"))
        append_items(lines, "## Compatibility", notes.get("compatibility"))
        append_items(lines, "## Technical changes", notes.get("technical"))

    append_versions(lines, current, lang)
    append_install(lines, channel, release_tag, lang)
    append_build(lines, channel, commit, release_version, lang)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--channel", choices=("beta", "stable"), required=True)
    parser.add_argument("--config", required=True)
    parser.add_argument("--release-tag")
    parser.add_argument("--final", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--commit", required=True)
    args = parser.parse_args()

    config = load(args.config)
    final = load(args.final)

    if config.get("channel") != args.channel:
        raise SystemExit("release config channel mismatch")
    if final.get("channel") != args.channel:
        raise SystemExit("release index channel mismatch")

    release_version = str(config.get("release_version", "")).strip()
    if not release_version:
        raise SystemExit("release_version is missing")

    lines = [
        f"# RouterForge {release_version}",
        "",
    ]

    append_language(lines, config, final, args.channel, args.commit, args.release_tag, "ru")
    lines += ["---", ""]
    append_language(lines, config, final, args.channel, args.commit, args.release_tag, "en")

    Path(args.output).write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
