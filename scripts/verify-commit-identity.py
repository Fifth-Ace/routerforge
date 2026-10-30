#!/usr/bin/env python3
"""Fail-closed RouterForge commit attribution gate for new commits and release ranges.

This gate does not rewrite history. Its release range mode deliberately rejects
old DEV commits with non-owner identities, requiring an explicit clean release
integration method instead of merging unsafe ancestry into main.
"""
from __future__ import annotations

import re
import subprocess
import sys

OWNER_NAME = 'Fifth-Ace'
OWNER_EMAIL = '248300012+Fifth-Ace@users.noreply.github.com'
HEX40 = re.compile(r'^[a-fA-F0-9]{40}$')


def git(*args: str) -> str:
    p = subprocess.run(['git', *args], stdout=subprocess.PIPE,
                       stderr=subprocess.PIPE, text=True, encoding='utf-8',
                       errors='replace', check=False)
    if p.returncode:
        raise ValueError('GIT_FAILED: ' + ' '.join(args[:3]) + ': ' + p.stderr.strip()[:300])
    return p.stdout.strip()


def resolve_commit(ref: str) -> str:
    if not ref or ref.startswith('-') or ref == '0' * 40:
        raise ValueError('INVALID_COMMIT_REF')
    result = git('rev-parse', '--verify', ref + '^{commit}')
    if not HEX40.fullmatch(result):
        raise ValueError('INVALID_COMMIT_SHA')
    return result.lower()


def check_commit(sha: str) -> None:
    raw = git('show', '-s', '--format=%an%x00%ae%x00%cn%x00%ce', sha)
    fields = raw.split('\x00')
    if len(fields) != 4:
        raise ValueError('COMMIT_IDENTITY_UNPARSEABLE: ' + sha)
    an, ae, cn, ce = fields
    if (an, ae, cn, ce) != (OWNER_NAME, OWNER_EMAIL, OWNER_NAME, OWNER_EMAIL):
        raise ValueError(f'COMMIT_IDENTITY_DENIED: {sha}: '
                         f'author={an!r} <{ae}> committer={cn!r} <{ce}>')
    # GitHub also attributes commits to Co-authored-by trailers. No implicit
    # collaborators are allowed in this repository's delivery history.
    body = git('show', '-s', '--format=%B', sha)
    if re.search(r'(?im)^Co-authored-by\s*:', body):
        raise ValueError('COAUTHOR_TRAILER_DENIED: ' + sha)


def main(argv: list[str]) -> int:
    if len(argv) == 3 and argv[1] == 'head':
        sha = resolve_commit(argv[2])
        check_commit(sha)
        print('HEAD_AUTHOR_COMMITTER: PASS')
        print('CHECKED_SHA: ' + sha)
        return 0
    if len(argv) == 4 and argv[1] == 'range':
        base = resolve_commit(argv[2])
        head = resolve_commit(argv[3])
        # Reject unrelated lines or history rewrites instead of silently checking an arbitrary set.
        check = subprocess.run(['git', 'merge-base', '--is-ancestor', base, head],
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
        if check.returncode != 0:
            raise ValueError('PROMOTION_BASE_NOT_ANCESTOR')
        commits = [x for x in git('rev-list', '--reverse', base + '..' + head).splitlines() if x]
        if not commits:
            raise ValueError('EMPTY_PROMOTION_RANGE')
        for sha in commits:
            check_commit(sha)
        print(f'PROMOTION_RANGE_IDENTITIES: PASS ({len(commits)})')
        print('PROMOTION_BASE: ' + base)
        print('PROMOTION_HEAD: ' + head)
        return 0
    print('usage: verify-commit-identity.py head <sha> | range <ancestor> <sha>', file=sys.stderr)
    return 64


if __name__ == '__main__':
    try:
        sys.exit(main(sys.argv))
    except ValueError as exc:
        print('IDENTITY_GATE: FAIL', file=sys.stderr)
        print(str(exc), file=sys.stderr)
        sys.exit(1)
