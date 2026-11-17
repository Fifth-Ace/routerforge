#!/usr/bin/env python3
"""P51: prevent Stable metadata regressions and accidental republish of an old version.

This is a release-train *source* check; it does not create a release or mutate Git.
"""
from __future__ import annotations
import argparse
import json
import re
import subprocess
from pathlib import Path

FLOOR = (0, 14, 0)
IDS = frozenset({'routerforge-core', 'dns', 'admin', 'monitoring',
                 'network-tools', 'nfqws-manager', 'antiscan-manager',
                 'port-access-manager', 'crowdsec-manager', 'profiling'})
SEMVER = re.compile(r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)\Z')
TAG = re.compile(r'routerforge-v(\d+\.\d+\.\d+)\Z')


def parse_version(value: str) -> tuple[int, int, int]:
    m = SEMVER.fullmatch(value or '')
    if m is None:
        raise ValueError('INVALID_STABLE_SEMVER: ' + str(value))
    return tuple(map(int, m.groups()))


def check_config(cfg: dict, candidate: bool = False, tags: list[str] | None = None) -> tuple[int, int, int]:
    if cfg.get('schema_version') != 1 or cfg.get('channel') != 'stable':
        raise ValueError('INVALID_STABLE_CHANNEL_SCHEMA')
    version = parse_version(cfg.get('release_version'))
    if version < FLOOR:
        raise ValueError('STABLE_VERSION_REGRESSION_BELOW_0_14_0')
    rows = cfg.get('components', [])
    if not isinstance(rows, list) or len(rows) != len(IDS) or {r.get('id') for r in rows} != IDS:
        raise ValueError('STABLE_TEN_COMPONENT_POLICY_MISMATCH')
    packages = [r.get('package') for r in rows]
    if len(set(packages)) != len(packages) or any(not p for p in packages):
        raise ValueError('DUPLICATE_STABLE_PACKAGE')
    if rows[0].get('id') != 'routerforge-core' or parse_version(rows[0].get('version')) != version:
        raise ValueError('STABLE_CORE_RELEASE_VERSION_MISMATCH')
    for item in rows:
        parse_version(item.get('version'))
        if item.get('min_core_version') and parse_version(item['min_core_version']) > version:
            raise ValueError('STABLE_MIN_CORE_TOO_NEW: ' + item['id'])
    if candidate:
        if tags is None:
            raise ValueError('STABLE_CANDIDATE_REQUIRES_TAG_EVIDENCE')
        releases = [parse_version(m.group(1)) for t in tags if (m := TAG.fullmatch(t.strip()))]
        if not releases or max(releases) < FLOOR:
            raise ValueError('STABLE_RELEASE_TAG_FLOOR_MISSING')
        if version <= max(releases):
            raise ValueError('STABLE_CANDIDATE_NOT_NEWER_THAN_PUBLISHED_TAG')
    return version


def main() -> int:
    ap = argparse.ArgumentParser()
    group = ap.add_mutually_exclusive_group(required=True)
    group.add_argument('--baseline', action='store_true')
    group.add_argument('--candidate', action='store_true')
    args = ap.parse_args()
    try:
        cfg = json.loads(Path('release/channels/stable.json').read_text(encoding='utf-8'))
        tags = None
        if args.candidate:
            tags = subprocess.check_output(['git', 'tag', '--list', 'routerforge-v*'], text=True).splitlines()
        version = check_config(cfg, candidate=args.candidate, tags=tags)
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        print('STABLE_RELEASE_TRAIN: FAIL / ' + str(exc))
        return 1
    print('STABLE_RELEASE_TRAIN: PASS / ' + '.'.join(map(str, version)) +
          (' / NEW VERSION GATE' if args.candidate else ' / CURRENT FLOOR'))
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
