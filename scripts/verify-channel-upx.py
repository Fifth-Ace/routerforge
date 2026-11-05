#!/usr/bin/env python3
"""Fail-closed RouterForge ELF compression contract for DEV/Beta/Stable IPK.

DEV: ARM64 only, all shipped ELF binaries plain.
Beta/Stable: ARM64 + MIPS + MIPSEL, every shipped ELF binary UPX packed
and validated with the pinned UPX host tool. Profiling is config-only.
"""
import argparse
import io
from pathlib import Path
import subprocess
import tarfile
import tempfile

TARGETS = {'aarch64-3.10', 'mips-3.4', 'mipsel-3.4'}
EXECUTABLE_PACKAGES = {
    'routerforge-core', 'routerforge-dns', 'routerforge-admin',
    'routerforge-monitoring', 'routerforge-network-tools',
    'routerforge-nfqws-manager', 'routerforge-antiscan-manager',
    'routerforge-port-access-manager', 'routerforge-crowdsec-manager',
    'routerforge-dns-router',
}


def _member(tar, name):
    matches = [m for m in tar.getmembers() if m.isfile() and m.name.lstrip('./') == name]
    if len(matches) != 1:
        raise ValueError('missing or duplicate IPK member: ' + name)
    handle = tar.extractfile(matches[0])
    if handle is None:
        raise ValueError('unreadable IPK member: ' + name)
    return handle.read()


def _check_payload(payload, filename, require_upx, verify_upx):
    is_upx = b'UPX!' in payload
    if is_upx != require_upx:
        raise ValueError('%s: expected %s, found %s' %
                         (filename, 'UPX' if require_upx else 'plain',
                          'UPX' if is_upx else 'plain'))
    if require_upx and verify_upx:
        with tempfile.TemporaryDirectory(prefix='rf-upx-package-gate-') as work:
            binary = Path(work) / 'binary'
            binary.write_bytes(payload)
            subprocess.run(['upx', '-t', str(binary)], check=True, timeout=120)


def verify(ipk, channel, target, verify_upx=True):
    if channel not in {'dev', 'beta', 'stable'} or target not in TARGETS:
        raise ValueError('invalid channel or target')
    if channel == 'dev' and target != 'aarch64-3.10':
        raise ValueError('DEV must be ARM64 only')
    if not ipk.name.endswith('_' + target + '.ipk'):
        raise ValueError('IPK filename target mismatch: ' + ipk.name)
    package = ipk.name.split('_', 1)[0]
    if not package.startswith('routerforge-'):
        raise ValueError('unexpected package name')
    if package not in EXECUTABLE_PACKAGES and package != 'routerforge-profiling':
        raise ValueError('unknown release package: ' + package)
    if not ipk.is_file() or ipk.stat().st_size < 1024:
        raise ValueError('IPK missing or empty')
    with tarfile.open(ipk, 'r:gz') as outer:
        raw = _member(outer, 'data.tar.gz')
    names = []
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:gz') as data:
        for item in data.getmembers():
            name = item.name.lstrip('./')
            if not item.isfile() or not name.startswith('opt/bin/'):
                continue
            if name.count('/') != 2 or name.startswith('opt/bin/.'):
                raise ValueError('unexpected executable path: ' + name)
            src = data.extractfile(item)
            if src is None:
                raise ValueError('unreadable executable: ' + name)
            blob = src.read()
            if not blob.startswith(b'\x7fELF'):
                continue  # scripts may be installed alongside Go ELF
            names.append(name)
            _check_payload(blob, name, channel != 'dev', verify_upx)
    if package in EXECUTABLE_PACKAGES and not names:
        raise ValueError('missing ELF executable: ' + package)
    if package == 'routerforge-profiling' and names:
        raise ValueError('unexpected profiling ELF executable')
    print('CHANNEL_UPX_PACKAGE_GATE: PASS / %s / %s / %s / ELF=%d / %s' %
          (channel, target, package, len(names), 'PLAIN' if channel == 'dev' else 'UPX_ULTRA_BRUTE'))
    return len(names)


def _fake_package(root, package, blob, target='aarch64-3.10'):
    out = root / (package + '_1_' + target + '.ipk')
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode='w:gz') as data:
        info = tarfile.TarInfo('./opt/bin/' + package)
        info.size = len(blob)
        data.addfile(info, io.BytesIO(blob))
    contents = raw.getvalue()
    with tarfile.open(out, mode='w:gz') as outer:
        info = tarfile.TarInfo('./data.tar.gz')
        info.size = len(contents)
        outer.addfile(info, io.BytesIO(contents))
    return out


def selftest():
    # Big enough to satisfy the nonempty package check; no real UPX call.
    import hashlib
    plain = b'\x7fELF' + b''.join(hashlib.sha256(str(i).encode()).digest() for i in range(160))
    packed = plain + b'UPX!' + b'\x00' * 16
    with tempfile.TemporaryDirectory(prefix='rf-upx-test-') as work:
        root = Path(work)
        for payload, channel, should_pass in (
            (plain, 'dev', True), (plain, 'beta', False),
            (plain, 'stable', False), (packed, 'dev', False),
            (packed, 'beta', True), (packed, 'stable', True),
        ):
            p = _fake_package(root, 'routerforge-dns-router', payload)
            try:
                verify(p, channel, 'aarch64-3.10', verify_upx=False)
                result = True
            except ValueError:
                result = False
            if result != should_pass:
                raise AssertionError('%s: expected=%s result=%s' % (channel, should_pass, result))
        for target in ('mips-3.4', 'mipsel-3.4'):
            other = _fake_package(root, 'routerforge-dns-router', packed, target)
            verify(other, 'beta', target, verify_upx=False)
            verify(other, 'stable', target, verify_upx=False)
            try:
                verify(other, 'dev', target, verify_upx=False)
                raise AssertionError('non-ARM64 DEV accepted')
            except ValueError:
                pass
            plain_other = _fake_package(root, 'routerforge-dns-router', plain, target)
            try:
                verify(plain_other, 'beta', target, verify_upx=False)
                raise AssertionError('plain MIPS Beta accepted')
            except ValueError:
                pass
    print('CHANNEL_UPX_FIXTURES: PASS / DEV PLAIN / BETA+STABLE PACKED / ALL TARGETS')


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('ipk', type=Path, nargs='?')
    ap.add_argument('--channel', choices=['dev', 'beta', 'stable'])
    ap.add_argument('--target', choices=sorted(TARGETS))
    ap.add_argument('--selftest', action='store_true')
    args = ap.parse_args()
    if args.selftest:
        if args.ipk or args.channel or args.target:
            raise ValueError('--selftest cannot be combined with IPK arguments')
        selftest()
        return
    if not args.ipk or not args.channel or not args.target:
        ap.error('IPK, --channel and --target required')
    verify(args.ipk, args.channel, args.target)


if __name__ == '__main__':
    main()
