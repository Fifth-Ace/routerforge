#!/usr/bin/env python3
import argparse
import json
import re
import subprocess
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

HEX40 = re.compile(r"^[0-9a-f]{40}$")


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


def name(package, lang):
    pair = NAMES.get(package, (package, package))
    return pair[0] if lang == "ru" else pair[1]


def commit_history(previous, current):
    previous_sha = str((previous or {}).get("commit", "")).lower()
    current_sha = str(current).lower()
    if not HEX40.fullmatch(previous_sha) or not HEX40.fullmatch(current_sha):
        return []
    if previous_sha == current_sha:
        return []

    result = subprocess.run(
        [
            "git", "log", "--reverse", "--no-merges",
            "--format=%H%x09%s",
            f"{previous_sha}..{current_sha}",
        ],
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        return []

    commits = []
    for raw in result.stdout.splitlines():
        if "\t" not in raw:
            continue
        sha, subject = raw.split("\t", 1)
        sha = sha.strip().lower()
        subject = " ".join(subject.strip().split())
        if HEX40.fullmatch(sha) and subject:
            commits.append((sha, subject))
    # Rolling release notes should stay readable even after a long gap.
    return commits[-30:]


def append_component_changes(lines, changed, lang):
    heading = "## Изменения компонентов" if lang == "ru" else "## Component changes"
    lines += [heading, ""]
    if changed:
        for package, before, after in changed:
            label = name(package, lang)
            if before:
                lines.append(f"- **{label}:** `{before}` → `{after}`")
            else:
                initial = "первая версия в канале" if lang == "ru" else "initial channel version"
                lines.append(f"- **{label}:** {initial} `{after}`")
    else:
        lines.append(
            "- Версии пакетов в этой публикации не менялись."
            if lang == "ru"
            else "- No package versions changed in this publish."
        )
    lines.append("")


def append_commit_notes(lines, commits, lang):
    heading = "## Что изменилось с прошлого publish" if lang == "ru" else "## Changes since the previous publish"
    lines += [heading, ""]
    if commits:
        for sha, subject in commits:
            short = sha[:7]
            lines.append(
                f"- [`{short}`](https://github.com/Fifth-Ace/dns-monitor/commit/{sha}) — {subject}"
            )
    else:
        lines.append(
            "- Нет новых commit notes для этого rolling publish."
            if lang == "ru"
            else "- No additional commit notes for this rolling publish."
        )
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


def append_install(lines, channel, lang):
    channel_name = "Stable" if channel == "stable" else "Beta"
    heading = "## Установка" if lang == "ru" else "## Installation"
    intro = (
        f"Свежая установка **RouterForge {channel_name}** (Core + DNS, версии берутся из release-index):"
        if lang == "ru"
        else f"Fresh **RouterForge {channel_name}** install (Core + DNS, versions resolved from the release index):"
    )
    lines += [
        heading,
        "",
        intro,
        "",
        "```sh",
        f"wget -qO- https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-{channel}/routerforge-{channel}-bootstrap.sh | sh",
        "```",
        "",
    ]


def append_build(lines, channel, commit, lang):
    heading = "## Сборка и проверка" if lang == "ru" else "## Build and verification"
    commit_label = "Коммит" if lang == "ru" else "Commit"
    index_label = "Release index" if lang == "ru" else "Release index"
    sums_label = "Контрольные суммы" if lang == "ru" else "Checksums"
    bootstrap_label = "Bootstrap установки" if lang == "ru" else "Fresh-install bootstrap"
    verify = (
        "Перед установкой RouterForge сверяет точный release asset и SHA256 из release-index."
        if lang == "ru"
        else "RouterForge verifies the exact release asset and SHA256 from the channel index before package installation."
    )
    short = commit[:7]
    lines += [
        heading,
        "",
        f"- {commit_label}: [`{short}`](https://github.com/Fifth-Ace/dns-monitor/commit/{commit})",
        f"- {index_label}: `routerforge-{channel}-index.json`",
        f"- {sums_label}: `routerforge-{channel}-SHA256SUMS`",
        f"- {bootstrap_label}: `routerforge-{channel}-bootstrap.sh`",
        "",
        verify,
        "",
    ]


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

    commits = commit_history(previous, args.commit)
    channel_name = "Beta" if args.channel == "beta" else "Stable"

    ru_channel = (
        "Тестовый pre-release канал. Здесь изменения проверяются перед переносом в Stable."
        if args.channel == "beta"
        else "Production-канал RouterForge. Сюда попадают изменения после проверки в Beta."
    )
    en_channel = (
        "Pre-release testing channel. Changes are validated here before promotion to Stable."
        if args.channel == "beta"
        else "RouterForge production channel. Changes arrive here after validation in Beta."
    )

    lines = [
        f"# RouterForge {channel_name}",
        "",
        "## 🇷🇺 Русский",
        "",
        ru_channel,
        "",
        "Это rolling-канал с независимыми версиями компонентов RouterForge.",
        "",
    ]
    append_component_changes(lines, changed, "ru")
    append_commit_notes(lines, commits, "ru")
    append_versions(lines, current, "ru")
    append_install(lines, args.channel, "ru")
    append_build(lines, args.channel, args.commit, "ru")

    lines += [
        "---",
        "",
        "## 🇬🇧 English",
        "",
        en_channel,
        "",
        "This is a rolling channel with independently versioned RouterForge components.",
        "",
    ]
    append_component_changes(lines, changed, "en")
    append_commit_notes(lines, commits, "en")
    append_versions(lines, current, "en")
    append_install(lines, args.channel, "en")
    append_build(lines, args.channel, args.commit, "en")

    Path(args.output).write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
