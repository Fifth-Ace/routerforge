#!/usr/bin/env python3
import argparse
import json
import re
from pathlib import Path

HEX64 = re.compile(r"^[0-9a-f]{64}$")
SAFE_ASSET = re.compile(r"^[A-Za-z0-9._+-]+$")
SAFE_PACKAGE = re.compile(r"^[a-z0-9._+-]+$")

def load(path):
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)

def shell_single(value):
    if "'" in value or "\n" in value or "\r" in value:
        raise SystemExit(f"unsafe shell value: {value!r}")
    return "'" + value + "'"

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--channel", choices=("stable", "beta"), required=True)
    ap.add_argument("--final", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args()

    doc = load(args.final)
    if doc.get("schema_version") != 1 or doc.get("channel") != args.channel:
        raise SystemExit("release index channel/schema mismatch")

    by_package = {x.get("package"): x for x in doc.get("components", [])}
    required = ["routerforge-core", "routerforge-dns"]
    entries = []
    expected_prefix = (
        "https://github.com/Fifth-Ace/dns-monitor/releases/download/"
        f"routerforge-{args.channel}/"
    )

    for package in required:
        item = by_package.get(package)
        if not item:
            raise SystemExit(f"release index is missing {package}")
        asset = str(item.get("asset", ""))
        sha = str(item.get("sha256", "")).lower()
        url = str(item.get("url", ""))
        if not SAFE_PACKAGE.fullmatch(package):
            raise SystemExit(f"unsafe package {package}")
        if not SAFE_ASSET.fullmatch(asset):
            raise SystemExit(f"unsafe asset {asset}")
        if not HEX64.fullmatch(sha):
            raise SystemExit(f"invalid sha256 for {package}")
        if not url.startswith(expected_prefix) or not url.endswith("/" + asset):
            raise SystemExit(f"unexpected release URL for {package}")
        entries.append((package, asset, sha, url, str(item.get("version", ""))))

    lines = [
        "#!/bin/sh",
        "set -eu",
        "",
        f"CHANNEL={shell_single(args.channel)}",
        'TMP="/opt/tmp/routerforge-bootstrap.$$"',
        "",
        "say() { printf '%s\\n' \"$*\"; }",
        "fail() { printf 'ERROR: %s\\n' \"$*\" >&2; exit 1; }",
        "",
        'case "$(uname -m 2>/dev/null || true)" in',
        "    aarch64|arm64) ;;",
        '    *) fail "RouterForge supports ARM64/aarch64 Keenetic/Netcraze routers." ;;',
        "esac",
        "",
        '[ -d /opt ] || fail "Entware /opt was not found."',
        'if [ -x /opt/bin/opkg ]; then OPKG=/opt/bin/opkg',
        'elif command -v opkg >/dev/null 2>&1; then OPKG="$(command -v opkg)"',
        'else fail "opkg was not found."; fi',
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
        'mkdir -p "$TMP"',
        "trap 'rm -rf \"$TMP\"' EXIT HUP INT TERM",
        "",
        'install_verified() {',
        '    package="$1"; asset="$2"; expected="$3"; url="$4"',
        '    fetch "$url" "$TMP/$asset"',
        '    actual="$(sha256sum "$TMP/$asset" | awk \'{print $1}\')"',
        '    [ "$actual" = "$expected" ] || fail "SHA256 mismatch for $asset."',
        '    say "Installing $package..."',
        '    "$OPKG" install "$TMP/$asset"',
        "}",
        "",
        'say "RouterForge $CHANNEL bootstrap"',
    ]

    for package, asset, sha, url, version in entries:
        lines += [
            f"say {shell_single(package + ' ' + version)}",
            "install_verified "
            + " ".join(shell_single(v) for v in (package, asset, sha, url)),
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
