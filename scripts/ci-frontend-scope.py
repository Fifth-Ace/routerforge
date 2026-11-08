#!/usr/bin/env python3
"""P0.1: exact, testable CI scope for the three unfinished frontends.

This does not declare a Svelte migration complete. Classification only;
no build, deploy or publication occurs here.
"""
from __future__ import annotations

import argparse
from pathlib import Path
import sys

FLAGS = ('dns_lite_frontend', 'dns_runtime_frontend', 'nfqws_frontend')
PREFIXES = {
    'dns_lite_frontend': (
        'modules/dns-router-lite/frontend/',
        'modules/dns-router-lite/frontend-tests/',
    ),
    'dns_runtime_frontend': (
        'modules/dns-router-runtime/frontend/',
        'modules/dns-router-runtime/frontend-tests/',
    ),
    'nfqws_frontend': (
        'modules/nfqws-manager/frontend/',
    ),
}
SHARED_BUILDERS = {
    'scripts/build-dns-router-experimental.sh': ('dns_lite_frontend', 'dns_runtime_frontend'),
    'scripts/verify-beta-dns-router-package.py': ('dns_lite_frontend', 'dns_runtime_frontend'),
    'scripts/build-module-opkg.sh': ('nfqws_frontend',),
    'scripts/build-modules-opkg.sh': ('nfqws_frontend',),
    'scripts/target-env.sh': FLAGS,
    'scripts/ci-frontend-scope.py': FLAGS,
    'tests/test_ci_frontend_scope.py': FLAGS,
    '.github/workflows/ci.yml': FLAGS,
}


def classify(paths: list[str], full_release: bool = False) -> dict[str, bool]:
    result = {key: bool(full_release) for key in FLAGS}
    for raw in paths:
        # Git diff --name-only emits repo-relative forward slash paths.
        path = raw.strip()
        if not path:
            continue
        if path.startswith('/') or path.startswith('../') or '\\' in path:
            raise ValueError('invalid repository path: ' + repr(path))
        for key in FLAGS:
            if path.startswith(PREFIXES[key]):
                result[key] = True
        for key in SHARED_BUILDERS.get(path, ()):
            result[key] = True
    return result


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--changed-files', type=Path, required=True)
    parser.add_argument('--github-output', type=Path, required=True)
    parser.add_argument('--full-release', choices=('true', 'false'), required=True)
    args = parser.parse_args(argv)
    full = args.full_release == 'true'
    if args.changed_files.is_file():
        paths = args.changed_files.read_text(encoding='utf-8').splitlines()
    elif full:
        paths = []
    else:
        print('FRONTEND_SCOPE_CHANGED_FILES_MISSING: ' + str(args.changed_files), file=sys.stderr)
        return 2
    try:
        flags = classify(paths, full_release=full)
    except ValueError as exc:
        print('FRONTEND_SCOPE_INVALID_INPUT: ' + str(exc), file=sys.stderr)
        return 2
    output = ''.join(f'{key}={str(value).lower()}\n' for key, value in flags.items())
    # Deliberately append rather than replace other step outputs.
    with args.github_output.open('a', encoding='utf-8', newline='\n') as handle:
        handle.write(output)
    print('RF_FRONTEND_SCOPE_P01=PASS')
    print(output, end='')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
