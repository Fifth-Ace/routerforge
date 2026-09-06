#!/usr/bin/env python3
import argparse
import json
import re
from pathlib import Path

HEX64 = re.compile(r"^[0-9a-f]{64}$")
SAFE_ASSET = re.compile(r"^[A-Za-z0-9._+-]+$")
SAFE_PACKAGE = re.compile(r"^[a-z0-9._+-]+$")

ALLOWED_REPOSITORIES = (
    "Fifth-Ace/routerforge",
    "Fifth-Ace/dns-monitor",
)

def load(path):
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)

def shell_single(value):
    if "'" in value or "\n" in value or "\r" in value:
        raise SystemExit(f"unsafe shell value: {value!r}")
    return "'" + value + "'"

def valid_release_url(url, channel, asset):
    for repository in ALLOWED_REPOSITORIES:
        prefix = f"https://github.com/{repository}/releases/download/routerforge-{channel}/"
        if url.startswith(prefix) and url.endswith("/" + asset):
            return True
    return False

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--channel", choices=("stable", "beta"), required=True)
    ap.add_argument("--final", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args()

    doc = load(args.final)
    if doc.get("schema_version") != 1 or doc.get("channel") != args.channel:
        raise SystemExit("release index channel/schema mismatch")

    target = str(doc.get("target", ""))
    supported_targets = {
        "aarch64-3.10",
        "mips-3.4",
        "mipsel-3.4",
    }
    if target not in supported_targets:
        raise SystemExit(f"unsupported release target {target!r}")

    by_package = {x.get("package"): x for x in doc.get("components", [])}
    required = ["routerforge-core", "routerforge-dns"]
    entries = []

    for package in required:
        item = by_package.get(package)
        if not item:
            raise SystemExit(f"release index is missing {package}")
        asset = str(item.get("asset", ""))
        sha = str(item.get("sha256", "")).lower()
        legacy_url = str(item.get("url", ""))
        canonical_url = str(item.get("canonical_url", "") or legacy_url)
        if not SAFE_PACKAGE.fullmatch(package):
            raise SystemExit(f"unsafe package {package}")
        if not SAFE_ASSET.fullmatch(asset):
            raise SystemExit(f"unsafe asset {asset}")
        if not HEX64.fullmatch(sha):
            raise SystemExit(f"invalid sha256 for {package}")
        if not valid_release_url(legacy_url, args.channel, asset):
            raise SystemExit(f"unexpected legacy release URL for {package}")
        if not valid_release_url(canonical_url, args.channel, asset):
            raise SystemExit(f"unexpected canonical release URL for {package}")
        fallback_url = legacy_url if legacy_url != canonical_url else ""
        entries.append((
            package,
            asset,
            sha,
            canonical_url,
            fallback_url,
            str(item.get("version", "")),
        ))

    lines = [
        "#!/bin/sh",
        "set -eu",
        "",
        f"CHANNEL={shell_single(args.channel)}",
        f"TARGET={shell_single(target)}",
        'PREFLIGHT_ONLY="${ROUTERFORGE_PREFLIGHT_ONLY:-0}"',
        'PROC_MEMINFO="${ROUTERFORGE_PROC_MEMINFO:-/proc/meminfo}"',
        'PROC_CPUINFO="${ROUTERFORGE_PROC_CPUINFO:-/proc/cpuinfo}"',
        'TMP="/opt/tmp/routerforge-bootstrap.$$"',
        "",
        "say() { printf '%s\\n' \"$*\"; }",
        "fail() { printf 'ERROR: %s\\n' \"$*\" >&2; exit 1; }",
        "",
        'if [ -n "${ROUTERFORGE_OPKG:-}" ]; then OPKG="$ROUTERFORGE_OPKG"',
        'elif [ -x /opt/bin/opkg ]; then OPKG=/opt/bin/opkg',
        'elif command -v opkg >/dev/null 2>&1; then OPKG="$(command -v opkg)"',
        'else fail "opkg was not found."; fi',
        "",
        "target_preflight() {",
        '    if ! "$OPKG" print-architecture 2>/dev/null |',
        '        awk ''$1 == "arch" { print $2 }'' |',
        '        grep -Fxq "$TARGET"; then',
        '        say ""',
        '        say "Target package architecture: $TARGET"',
        '        say "Entware architectures:"',
        '        "$OPKG" print-architecture 2>/dev/null || true',
        '        fail "This RouterForge build is not compatible with the installed Entware architecture."',
        '    fi',
        "}",
        "",
        "target_preflight",
        "",
        "detect_mem_mib() {",
        "    awk '/^MemTotal:/ { printf \"%d\\n\", $2 / 1024; found=1; exit } END { if (!found) print 0 }' \"$PROC_MEMINFO\" 2>/dev/null || printf '0\\n'",
        "}",
        "",
        "detect_swap_mib() {",
        "    awk '/^SwapTotal:/ { printf \"%d\\n\", $2 / 1024; found=1; exit } END { if (!found) print 0 }' \"$PROC_MEMINFO\" 2>/dev/null || printf '0\\n'",
        "}",
        "",
        "detect_cpu_cores() {",
        "    cores=\"$(awk '/^processor[[:space:]]*:/ { n++ } END { print n + 0 }' \"$PROC_CPUINFO\" 2>/dev/null || printf '0\\n')\"",
        "    case \"$cores\" in ''|*[!0-9]*) cores=0 ;; esac",
        '    if [ "$cores" -gt 0 ]; then',
        "        printf '%s\\n' \"$cores\"",
        '        return 0',
        '    fi',
        '    if command -v nproc >/dev/null 2>&1; then',
        '        nproc 2>/dev/null && return 0',
        '    fi',
        "    printf '0\\n'",
        "}",
        "",
        "hardware_preflight() {",
        '    ram_mib="$(detect_mem_mib)"',
        '    swap_mib="$(detect_swap_mib)"',
        '    cpu_cores="$(detect_cpu_cores)"',
        "",
        "    case \"$ram_mib\" in ''|*[!0-9]*) ram_mib=0 ;; esac",
        "    case \"$swap_mib\" in ''|*[!0-9]*) swap_mib=0 ;; esac",
        "    case \"$cpu_cores\" in ''|*[!0-9]*) cpu_cores=0 ;; esac",
        "",
        '    say ""',
        '    say "Detected hardware / Обнаруженное оборудование:"',
        '    say "RAM: ${ram_mib} MiB"',
        '    say "CPU cores: ${cpu_cores}"',
        '    say "Swap: ${swap_mib} MiB"',
        "",
        '    low_ram=0',
        '    single_core=0',
        "",
        '    if [ "$ram_mib" -gt 0 ] && [ "$ram_mib" -le 128 ]; then',
        '        low_ram=1',
        '    fi',
        '    if [ "$cpu_cores" -eq 1 ]; then',
        '        single_core=1',
        '    fi',
        "",
        '    if [ "$low_ram" -eq 1 ] && [ "$single_core" -eq 1 ]; then',
        '        say ""',
        '        say "ПРЕДУПРЕЖДЕНИЕ: обнаружено устройство с 128 МБ ОЗУ или меньше и одноядерным процессором."',
        '        say "Установка RouterForge разрешена, но стабильная работа и производительность не гарантируются."',
        '        if [ "$swap_mib" -gt 0 ]; then',
        '            say "Активный swap обнаружен: ${swap_mib} MiB."',
        '        else',
        '            say "Настоятельно рекомендуется настроить swap-раздел или swap-файл на USB-накопителе."',
        '        fi',
        '        say ""',
        '        say "WARNING: a device with 128 MiB of RAM or less and a single-core CPU was detected."',
        '        say "RouterForge installation is allowed, but stable operation and performance are not guaranteed."',
        '        if [ "$swap_mib" -gt 0 ]; then',
        '            say "Active swap detected: ${swap_mib} MiB."',
        '        else',
        '            say "Configuring a swap partition or swap file on USB storage is strongly recommended."',
        '        fi',
        "",
        '    elif [ "$low_ram" -eq 1 ]; then',
        '        say ""',
        '        say "ПРЕДУПРЕЖДЕНИЕ: обнаружено 128 МБ ОЗУ или меньше."',
        '        if [ "$swap_mib" -gt 0 ]; then',
        '            say "Активный swap обнаружен: ${swap_mib} MiB."',
        '        else',
        '            say "Для стабильной работы RouterForge рекомендуется настроить swap-раздел или swap-файл на USB-накопителе."',
        '        fi',
        '        say ""',
        '        say "WARNING: 128 MiB of RAM or less detected."',
        '        if [ "$swap_mib" -gt 0 ]; then',
        '            say "Active swap detected: ${swap_mib} MiB."',
        '        else',
        '            say "For stable RouterForge operation, configuring a swap partition or swap file on USB storage is recommended."',
        '        fi',
        "",
        '    elif [ "$single_core" -eq 1 ]; then',
        '        say ""',
        '        say "ПРЕДУПРЕЖДЕНИЕ: обнаружен одноядерный процессор."',
        '        say "RouterForge можно установить, но стабильная работа и производительность на таком устройстве не гарантируются."',
        '        say ""',
        '        say "WARNING: single-core CPU detected."',
        '        say "RouterForge can be installed, but stable operation and performance on this device are not guaranteed."',
        '    fi',
        "",
        '    say ""',
        "}",
        "",
        "hardware_preflight",
        "",
        'if [ "$PREFLIGHT_ONLY" = "1" ]; then',
        '    exit 0',
        'fi',
        "",
        '[ -d /opt ] || fail "Entware /opt was not found."',
        'command -v sha256sum >/dev/null 2>&1 || fail "sha256sum was not found."',
        "",
        "fetch() {",
        '    if command -v curl >/dev/null 2>&1; then curl -fL --connect-timeout 10 --max-time 120 -o "$2" "$1"',
        '    elif [ -x /opt/bin/curl ]; then /opt/bin/curl -fL --connect-timeout 10 --max-time 120 -o "$2" "$1"',
        '    elif command -v wget >/dev/null 2>&1; then wget -q -O "$2" "$1"',
        '    elif [ -x /opt/bin/wget ]; then /opt/bin/wget -q -O "$2" "$1"',
        '    else fail "curl/wget was not found."; fi',
        "}",
        "",
        'fetch_compatible() {',
        '    primary="$1"; fallback="$2"; output="$3"',
        '    if fetch "$primary" "$output"; then return 0; fi',
        '    if [ -n "$fallback" ] && [ "$fallback" != "$primary" ]; then',
        '        say "Primary repository URL unavailable; trying compatibility URL..."',
        '        fetch "$fallback" "$output"',
        '        return $?',
        '    fi',
        '    return 1',
        '}',
        "",
        'mkdir -p "$TMP"',
        "trap 'rm -rf \"$TMP\"' EXIT HUP INT TERM",
        "",
        'install_verified() {',
        '    package="$1"; asset="$2"; expected="$3"; primary="$4"; fallback="$5"',
        '    fetch_compatible "$primary" "$fallback" "$TMP/$asset" || fail "Download failed for $asset."',
        '    actual="$(sha256sum "$TMP/$asset" | awk \'{print $1}\')"',
        '    [ "$actual" = "$expected" ] || fail "SHA256 mismatch for $asset."',
        '    say "Installing $package..."',
        '    "$OPKG" install "$TMP/$asset"',
        "}",
        "",
        'say "RouterForge $CHANNEL bootstrap"',
    ]

    for package, asset, sha, primary, fallback, version in entries:
        lines += [
            f"say {shell_single(package + ' ' + version)}",
            "install_verified "
            + " ".join(shell_single(v) for v in (package, asset, sha, primary, fallback)),
        ]

    lines += [
        "",
        'say ""',
        'say "RouterForge is ready."',
        'say "Web UI: http://<router-ip>:2233"',
        'say "Core service: /opt/etc/init.d/S90routerforge"',
        'say "Log: /opt/var/log/routerforge.log"',
        'say "Install optional capabilities from Marketplace."',
        "",
    ]

    Path(args.output).write_text("\n".join(lines), encoding="utf-8")

if __name__ == "__main__":
    main()
