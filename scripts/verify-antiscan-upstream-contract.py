#!/usr/bin/env python3
import argparse
import base64
import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST_PATH = ROOT / "scripts" / "contracts" / "antiscan-upstream.json"
DIAGNOSTICS_GO = ROOT / "modules" / "antiscan-manager-runtime" / "diagnostics.go"

SEMANTIC_PATHS = {
    "init": "etc/init.d/S99ascn",
    "config": "etc/antiscan/ascn.conf",
    "cron": "etc/antiscan/ascn_crontab.conf",
    "ipsets": "etc/antiscan/scripts/ipsets.sh",
    "valid": "etc/antiscan/scripts/valid_params",
}

def fail(message):
    print("ANTISCAN_UPSTREAM_DRIFT_GUARD=FAIL")
    print("ERROR=" + message)
    raise SystemExit(1)

def load_manifest():
    try:
        data = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
    except Exception as exc:
        fail("cannot read upstream contract manifest: " + str(exc))
    if data.get("schema_version") != 1:
        fail("unsupported upstream contract manifest schema")
    if data.get("repository") != "dimon27254/antiscan":
        fail("unexpected upstream repository")
    if data.get("branch") != "main":
        fail("unexpected upstream branch")
    if not re.fullmatch(r"[0-9a-f]{40}", str(data.get("pinned_commit", ""))):
        fail("invalid pinned upstream commit")
    if not re.fullmatch(r"[0-9]+(?:\.[0-9]+)+", str(data.get("pinned_version", ""))):
        fail("invalid pinned upstream version")
    blobs = data.get("critical_blobs")
    if not isinstance(blobs, dict) or len(blobs) < 10:
        fail("critical upstream blob manifest is incomplete")
    for path, sha in blobs.items():
        if not isinstance(path, str) or not path or ".." in path:
            fail("invalid critical upstream path")
        if not re.fullmatch(r"[0-9a-f]{40}", str(sha)):
            fail("invalid critical blob sha for " + path)
    return data

def extract_go_const(text, name):
    match = re.search(r"\b" + re.escape(name) + r'\s*=\s*"([^"]+)"', text)
    if not match:
        fail("missing Go contract constant " + name)
    return match.group(1)

def extract_go_slice(text, name):
    match = re.search(
        r"\bvar\s+" + re.escape(name) + r"\s*=\s*\[\]string\s*\{(.*?)\n\}",
        text,
        re.S,
    )
    if not match:
        fail("missing Go contract slice " + name)
    return re.findall(r'"([^"]*)"', match.group(1))

def check_local_contract(manifest):
    try:
        diagnostics = DIAGNOSTICS_GO.read_text(encoding="utf-8")
    except Exception as exc:
        fail("cannot read diagnostics.go: " + str(exc))

    pairs = [
        ("pinned_commit", extract_go_const(diagnostics, "antiscanUpstreamPinnedSHA")),
        ("pinned_version", extract_go_const(diagnostics, "antiscanUpstreamPinnedVersion")),
    ]
    for key, actual in pairs:
        if manifest[key] != actual:
            fail("manifest/Go mismatch for " + key + ": manifest=" + manifest[key] + " go=" + actual)

    list_pairs = [
        ("config_keys", "antiscanUpstreamConfigKeys"),
        ("ipsets", "antiscanUpstreamIPSets"),
        ("commands", "antiscanUpstreamCommands"),
        ("valid_tasks", "antiscanUpstreamValidTasks"),
    ]
    for manifest_key, go_name in list_pairs:
        actual = extract_go_slice(diagnostics, go_name)
        expected = manifest.get(manifest_key)
        if actual != expected:
            fail("manifest/Go list mismatch for " + manifest_key)

    runtime_files = extract_go_slice(diagnostics, "antiscanUpstreamRuntimeFiles")
    critical = manifest["critical_blobs"]
    for rel in runtime_files:
        full = "etc/antiscan/" + rel
        if full not in critical:
            fail("runtime contract file missing from critical blob guard: " + full)

    for path in SEMANTIC_PATHS.values():
        if path not in critical:
            fail("semantic upstream path missing from critical blob guard: " + path)
    for path in ("etc/ndm/netfilter.d/099-ascn.sh",):
        if path not in critical:
            fail("required hook missing from critical blob guard: " + path)

    print("ANTISCAN_UPSTREAM_STATIC_CONTRACT=PASS")
    print("PINNED_SHA=" + manifest["pinned_commit"])
    print("PINNED_VERSION=" + manifest["pinned_version"])
    print("CONFIG_KEYS=" + str(len(manifest["config_keys"])))
    print("IPSETS=" + str(len(manifest["ipsets"])))
    print("COMMANDS=" + str(len(manifest["commands"])))
    print("VALID_TASKS=" + str(len(manifest["valid_tasks"])))
    print("CRITICAL_BLOBS=" + str(len(manifest["critical_blobs"])))

class GitHub:
    def __init__(self):
        self.token = os.environ.get("GITHUB_TOKEN", "").strip()

    def json(self, url):
        headers = {
            "Accept": "application/vnd.github+json",
            "User-Agent": "RouterForge-Antiscan-Drift-Guard",
            "X-GitHub-Api-Version": "2022-11-28",
        }
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        request = urllib.request.Request(url, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                return json.load(response)
        except urllib.error.HTTPError as exc:
            body = exc.read(1024).decode("utf-8", "replace")
            fail("GitHub API HTTP %d: %s" % (exc.code, body))
        except Exception as exc:
            fail("GitHub API request failed: " + str(exc))

    def blob_text(self, repository, sha):
        payload = self.json(
            "https://api.github.com/repos/%s/git/blobs/%s" % (repository, sha)
        )
        if payload.get("encoding") != "base64":
            fail("unexpected GitHub blob encoding for " + sha)
        try:
            return base64.b64decode(payload["content"]).decode("utf-8")
        except Exception as exc:
            fail("cannot decode upstream blob " + sha + ": " + str(exc))

def shell_assignment(text, name):
    marker = name + '="'
    start = text.find(marker)
    if start < 0:
        fail("missing upstream shell assignment " + name)
    start += len(marker)
    end = text.find('"', start)
    if end < 0:
        fail("unterminated upstream shell assignment " + name)
    return text[start:end].replace("\\\n", "")

def parse_main_commands(text):
    marker = 'case "$1" in'
    start = text.find(marker)
    if start < 0:
        fail("upstream main command dispatcher missing")
    end = text.find("\nesac", start)
    if end < 0:
        fail("upstream main command dispatcher is unterminated")
    block = text[start:end]
    commands = []
    for label in re.findall(r"(?m)^([A-Za-z0-9_ ]+(?:\s*\|\s*[A-Za-z0-9_ ]+)*)\)\s*$", block):
        for item in label.split("|"):
            item = item.strip()
            if item:
                commands.append(item)
    return commands

def parse_config_keys(text):
    return re.findall(r"(?m)^([A-Z][A-Z0-9_]*)=", text)

def parse_default_cron(text):
    return [
        line.strip()
        for line in text.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]

def parse_full_ipset_list(text):
    candidates = []
    for raw in re.findall(r'ipsets_list="([^"]+)"', text):
        items = raw.split()
        if items and all(item.startswith("ascn_") for item in items):
            candidates.append(items)
    if not candidates:
        fail("cannot locate upstream full ipset list")
    return max(candidates, key=len)

def check_remote_contract(manifest):
    gh = GitHub()
    repository = manifest["repository"]
    branch = manifest["branch"]
    pinned = manifest["pinned_commit"]

    branch_data = gh.json(
        "https://api.github.com/repos/%s/branches/%s" % (repository, branch)
    )
    head = str(branch_data.get("commit", {}).get("sha", ""))
    if not re.fullmatch(r"[0-9a-f]{40}", head):
        fail("cannot resolve upstream branch head")

    tree_data = gh.json(
        "https://api.github.com/repos/%s/git/trees/%s?recursive=1" % (repository, head)
    )
    if tree_data.get("truncated"):
        fail("upstream recursive tree was truncated")
    tree = {
        item.get("path"): item.get("sha")
        for item in tree_data.get("tree", [])
        if item.get("type") == "blob"
    }

    changed = []
    for path, expected_sha in manifest["critical_blobs"].items():
        current_sha = tree.get(path)
        if current_sha != expected_sha:
            changed.append((path, expected_sha, current_sha or "MISSING"))

    print("UPSTREAM_HEAD_SHA=" + head)
    print("PINNED_SHA=" + pinned)

    if changed:
        print("UPSTREAM_CRITICAL_BLOBS=DRIFT")
        for path, expected_sha, current_sha in changed:
            print("DRIFT_FILE=%s pinned=%s current=%s" % (path, expected_sha, current_sha))
        fail("critical Antiscan upstream contract changed; manual review and repin required")

    print("UPSTREAM_CRITICAL_BLOBS=UNCHANGED")
    print("UPSTREAM_HEAD_CHANGED_NONCONTRACT=" + ("YES" if head != pinned else "NO"))

    texts = {}
    for label, path in SEMANTIC_PATHS.items():
        texts[label] = gh.blob_text(repository, tree[path])

    version_match = re.search(r'(?m)^ASCN_VERSION="([^"]+)"$', texts["init"])
    if not version_match:
        fail("cannot read ASCN_VERSION from upstream S99ascn")
    if version_match.group(1) != manifest["pinned_version"]:
        fail("upstream ASCN_VERSION differs from pinned version")

    valid_params = shell_assignment(texts["valid"], "VALID_PARAMS").split("|")
    if valid_params != manifest["config_keys"]:
        fail("upstream VALID_PARAMS differs from RouterForge contract")

    valid_tasks = shell_assignment(texts["valid"], "VALID_TASKS").split("|")
    if valid_tasks != manifest["valid_tasks"]:
        fail("upstream VALID_TASKS differs from RouterForge contract")

    if parse_config_keys(texts["config"]) != manifest["config_keys"]:
        fail("upstream default ascn.conf keys differ from RouterForge contract")

    if parse_main_commands(texts["init"]) != manifest["commands"]:
        fail("upstream S99ascn command surface differs from RouterForge contract")

    if parse_default_cron(texts["cron"]) != manifest["default_cron"]:
        fail("upstream default crontab differs from RouterForge contract")

    if parse_full_ipset_list(texts["ipsets"]) != manifest["ipsets"]:
        fail("upstream ipset inventory differs from RouterForge contract")

    print("UPSTREAM_VERSION_SEMANTICS=PASS")
    print("UPSTREAM_CONFIG_KEYS_SEMANTICS=PASS")
    print("UPSTREAM_COMMAND_SURFACE_SEMANTICS=PASS")
    print("UPSTREAM_IPSET_SURFACE_SEMANTICS=PASS")
    print("UPSTREAM_CRON_SURFACE_SEMANTICS=PASS")
    print("ANTISCAN_UPSTREAM_DRIFT_GUARD=PASS")

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--remote",
        action="store_true",
        help="also compare the live upstream main branch against the pinned contract",
    )
    args = parser.parse_args()

    manifest = load_manifest()
    check_local_contract(manifest)
    if args.remote:
        check_remote_contract(manifest)
    else:
        print("REMOTE_CHECK=SKIPPED")
        print("ANTISCAN_UPSTREAM_DRIFT_GUARD=PASS")

if __name__ == "__main__":
    main()
