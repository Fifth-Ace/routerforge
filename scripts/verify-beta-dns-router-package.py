#!/usr/bin/env python3
"""Validate Beta DNS Router Lite IPK metadata, architecture and shipped service.

No network, router, system configuration or write to installed paths.
Optional --qemu checks only `--version` in a disposable local directory.
"""
import argparse
from html.parser import HTMLParser
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile

TARGETS = {
    'aarch64-3.10': (183, 1, None),
    'mips-3.4': (8, 2, 'qemu-mips'),
    'mipsel-3.4': (8, 1, 'qemu-mipsel'),
}


def tar_payload(archive, name):
    matches = [m for m in archive.getmembers() if m.name.lstrip('./') == name and m.isfile()]
    if len(matches) != 1:
        raise ValueError('IPK archive missing or duplicate: ' + name)
    handle = archive.extractfile(matches[0])
    if handle is None:
        raise ValueError('IPK member unreadable: ' + name)
    return handle.read()


class LocalUIAssets(HTMLParser):
    """Collect only relative script/style assets; global Core CSS is external."""

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.names = set()

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag == 'script':
            raw = a.get('src', '')
        elif tag == 'link' and 'stylesheet' in a.get('rel', '').lower().split():
            raw = a.get('href', '')
        else:
            return
        if not raw or raw.startswith('/') or '://' in raw or raw.startswith('data:'):
            return
        # Only local filename references are supported by the DNS UI API.
        # Reject nested or escaped references rather than silently dropping them.
        if raw.startswith('./'):
            raw = raw[2:]
        if not raw or raw.startswith('.') or '/' in raw or '\\' in raw or '?' in raw or '#' in raw:
            raise ValueError('invalid local UI asset reference: ' + raw)
        if not raw.endswith(('.js', '.css')):
            raise ValueError('unexpected local UI asset reference: ' + raw)
        self.names.add(raw)


def verify(ipk, target, version, qemu=False):
    if target not in TARGETS:
        raise ValueError('unknown target')
    if not ipk.is_file() or ipk.stat().st_size < 1024:
        raise ValueError('IPK missing or empty: ' + str(ipk))
    with tarfile.open(ipk, 'r:gz') as outer:
        raw_control = tar_payload(outer, 'control.tar.gz')
        raw_data = tar_payload(outer, 'data.tar.gz')
    with tarfile.open(fileobj=io.BytesIO(raw_control), mode='r:gz') as control:
        metadata = tar_payload(control, 'control').decode('utf-8', errors='strict')
        fields = dict(line.split(': ', 1) for line in metadata.splitlines() if ': ' in line)
        if fields.get('Package') != 'routerforge-dns-router':
            raise ValueError('incorrect package identity')
        if fields.get('Version') != version:
            raise ValueError('incorrect package version: ' + str(fields.get('Version')))
        if fields.get('Architecture') != target:
            raise ValueError('incorrect control architecture')
        dependencies = {part.strip().split(' ')[0] for part in fields.get('Depends', '').split(',')}
        if not {'routerforge-core', 'ca-bundle'}.issubset(dependencies):
            raise ValueError('DNS Router service dependencies missing')
        for name in ('postinst', 'prerm'):
            if not tar_payload(control, name).startswith(b'#!'):
                raise ValueError('missing executable lifecycle file: ' + name)
    with tarfile.open(fileobj=io.BytesIO(raw_data), mode='r:gz') as data:
        binary = tar_payload(data, 'opt/bin/routerforge-dns-router')
        machine, endian, emulator = TARGETS[target]
        if not binary.startswith(b'\x7fELF') or len(binary) < 52:
            raise ValueError('missing ELF executable')
        if binary[5] != endian:
            raise ValueError('wrong ELF endian for ' + target)
        byteorder = 'little' if endian == 1 else 'big'
        actual_machine = int.from_bytes(binary[18:20], byteorder)
        if actual_machine != machine:
            raise ValueError('wrong ELF machine: ' + str(actual_machine))
        if not (int.from_bytes(binary[16:18], byteorder) == 2):
            raise ValueError('binary is not executable ELF')
        prefix = 'opt/share/routerforge/modules/dns-router/'
        manifest = json.loads(tar_payload(data, prefix + 'manifest.json').decode('utf-8'))
        if manifest.get('id') != 'dns-router' or manifest.get('mode') != 'independent-dns':
            raise ValueError('wrong DNS Router engine or manifest mode')
        if manifest.get('version') != version:
            raise ValueError('manifest version mismatch')
        for required in (
            prefix + 'ui/index.html', prefix + 'ui/svelte-mode.js', prefix + 'ui/module.css',
            'opt/etc/init.d/S95routerforge-dns-router',
            'opt/etc/ndm/netfilter.d/40-routerforge-dns-controller.sh',
            'opt/share/licenses/routerforge-dns-router/LICENSE',
        ):
            if not tar_payload(data, required):
                raise ValueError('empty required payload: ' + required)
        # P24.4: verify EVERY shipped local CSS/JS referenced by the actual HTML.
        # Source-only DOM tests do not prove that an IPK contains the assets.
        ui_html = tar_payload(data, prefix + 'ui/index.html').decode('utf-8')
        referenced = LocalUIAssets()
        referenced.feed(ui_html)
        for ui_asset in sorted(referenced.names):
            if not tar_payload(data, prefix + 'ui/' + ui_asset):
                raise ValueError('empty referenced UI asset: ' + ui_asset)
        if manifest.get('mode') == 'independent-dns':
            for required in ('svelte-mode.js', 'p25-settings.css'):
                if required not in referenced.names:
                    raise ValueError('DEV settings UI not referenced: ' + required)
        print('DNS_ROUTER_UI_IPK_REFERENCED_ASSETS: PASS / ' + ', '.join(sorted(referenced.names)))
    if qemu:
        if emulator is None:
            raise ValueError('QEMU MIPS smoke applies to MIPS or MIPSEL only')
        with tempfile.TemporaryDirectory(prefix='rf-beta-dns-mips-') as folder:
            exe = Path(folder) / 'dns-router'
            exe.write_bytes(binary)
            exe.chmod(0o700)
            outcome = subprocess.run([emulator, str(exe), '--version'],
                                     capture_output=True, text=True, timeout=90, check=False)
            if outcome.returncode != 0 or outcome.stdout.strip() != version:
                raise ValueError('QEMU version smoke FAIL: returncode=%s stdout=%r stderr=%r' %
                                 (outcome.returncode, outcome.stdout[:300], outcome.stderr[:500]))
    print('DNS_ROUTER_BETA_PACKAGE: PASS / %s / %s / ELF:%s%s' %
          (target, version, machine, ' / QEMU:PASS' if qemu else ''))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('ipk', type=Path)
    parser.add_argument('target', choices=sorted(TARGETS))
    parser.add_argument('version')
    parser.add_argument('--qemu', action='store_true')
    args = parser.parse_args()
    verify(args.ipk, args.target, args.version, args.qemu)


if __name__ == '__main__':
    main()
