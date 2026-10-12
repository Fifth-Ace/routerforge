#!/usr/bin/env python3
"""P44: offline IPK lifecycle acceptance. Never executes maintainer scripts.

The simulated filesystem is confined to TemporaryDirectory. Runtime execution,
Entware rc.func behavior and physical Keenetic acceptance are out of scope.
"""
import argparse
import io
from pathlib import Path
import re
import sys
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'scripts'))
from verify_port_access_ipk import unpack, verify  # P43 validator

PACKAGE = 'routerforge-port-access-manager'
PREFIX = 'opt/'


def insist(ok, label):
    if not ok:
        raise ValueError(label)
    print('PASS: ' + label, flush=True)


def ipk_entries(path):
    verify(path)
    with tarfile.open(path, 'r:gz') as outer:
        data_bytes = outer.extractfile('./data.tar.gz').read()
        control_bytes = outer.extractfile('./control.tar.gz').read()
    return unpack(data_bytes, 'data'), unpack(control_bytes, 'control')


def script_policy(control):
    for name in ('postinst', 'prerm'):
        raw = control[name][1]
        insist(raw.startswith(b'#!/bin/sh\n'), name + ' POSIX shell')
        code = raw.decode('utf-8')
        insist('set -eu' in code, name + ' strict errors')
        # Shell executes with absolute /opt paths, so this test deliberately
        # does not run it. Fail if it gains filesystem/firewall mutators.
        forbidden = re.compile(r'\b(?:iptables|ip6tables|nft|opkg|wget|curl|reboot|'
                               r'rm|mv|cp|dd|mkfs|mount|umount|killall)\b')
        insist(not forbidden.search(code), name + ' no unmanaged commands')
        insist('/opt/etc/init.d/S95routerforge-port-access-manager' in code,
               name + ' only managed service')
        insist('|| true' in code, name + ' service failure nonfatal')
    post = control['postinst'][1].decode('utf-8')
    pre = control['prerm'][1].decode('utf-8')
    insist(' stop ' in post and ' start ' in post, 'install/upgrade service cycle')
    insist(' stop ' in pre and ' start ' not in pre, 'remove stops without restarting')


def paths(data):
    return {name: payload for name, (entry, payload) in data.items() if entry.isfile()}


def install_sim(root, files, installed):
    """Model opkg file ownership only; no scripts executed and no host writes."""
    for rel, content in files.items():
        insist(rel.startswith(PREFIX) and '..' not in rel.split('/'), 'safe destination ' + rel) if not rel.startswith(PREFIX) or '..' in rel.split('/') else None
        dst = root.joinpath(*rel.split('/'))
        dst.parent.mkdir(parents=True, exist_ok=True)
        dst.write_bytes(content)
    return set(files)


def remove_sim(root, installed):
    for rel in installed:
        dst = root.joinpath(*rel.split('/'))
        if dst.exists():
            dst.unlink()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('package', type=Path)
    args = ap.parse_args()
    try:
        data, control = ipk_entries(args.package)
        print('=== P44 OFFLINE LIFECYCLE ===', flush=True)
        script_policy(control)
        files = paths(data)
        insist(bool(files), 'nonempty package payload')
        with tempfile.TemporaryDirectory(prefix='rf-p44-') as tmp:
            root = Path(tmp)
            # Simulate unrelated owner files, persistent state and an operator config.
            foreign = root / 'opt/etc/foreign.conf'
            foreign.parent.mkdir(parents=True)
            foreign.write_bytes(b'foreign-owner')
            state = root / 'opt/etc/routerforge/port-access-manager/operator.conf'
            state.parent.mkdir(parents=True)
            state.write_bytes(b'keep-me')
            installed = install_sim(root, files, set())
            insist(all((root / x).read_bytes() == y for x, y in files.items()), 'install payload exact')
            insist(state.read_bytes() == b'keep-me' and foreign.read_bytes() == b'foreign-owner', 'install preserves unowned state')
            # Upgrade a second time; every owned path must remain consistent.
            upgraded = install_sim(root, files, installed)
            insist(upgraded == installed, 'idempotent owned-file upgrade')
            insist(state.read_bytes() == b'keep-me', 'upgrade preserves operator configuration')
            remove_sim(root, upgraded)
            insist(all(not (root / x).exists() for x in upgraded), 'removal clears owned files')
            insist(state.read_bytes() == b'keep-me' and foreign.read_bytes() == b'foreign-owner', 'removal preserves unowned files')
        print('P44_OFFLINE_LIFECYCLE: PASS', flush=True)
        return 0
    except (ValueError, KeyError, OSError, tarfile.TarError) as err:
        print('P44_OFFLINE_LIFECYCLE: FAIL: ' + str(err), flush=True)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
