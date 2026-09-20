#!/usr/bin/env python3
import argparse
import json
import urllib.request
from pathlib import Path

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--metadata", required=True)
    ap.add_argument("--json-out")
    args = ap.parse_args()

    meta = json.loads(Path(args.metadata).read_text(encoding="utf-8"))
    upstream = meta["upstream"]
    repo = upstream["repository"].rstrip("/").split("github.com/", 1)[-1]
    current = upstream["release_tag"]

    req = urllib.request.Request(
        f"https://api.github.com/repos/{repo}/releases/latest",
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": "RouterForge-DPI-Detector-Upstream-Check",
        },
    )
    with urllib.request.urlopen(req, timeout=20) as response:
        latest_doc = json.load(response)

    latest = str(latest_doc.get("tag_name") or "").strip()
    if not latest:
        raise SystemExit("latest upstream release has no tag_name")

    result = {
        "repository": upstream["repository"],
        "current_tag": current,
        "latest_tag": latest,
        "update_available": latest != current,
        "latest_release_url": latest_doc.get("html_url", ""),
        "latest_published_at": latest_doc.get("published_at", ""),
    }

    if args.json_out:
        Path(args.json_out).write_text(
            json.dumps(result, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )

    print("DPI_UPSTREAM_REPOSITORY=" + result["repository"])
    print("DPI_UPSTREAM_CURRENT=" + current)
    print("DPI_UPSTREAM_LATEST=" + latest)
    print("DPI_UPSTREAM_UPDATE_AVAILABLE=" + ("true" if result["update_available"] else "false"))

if __name__ == "__main__":
    main()