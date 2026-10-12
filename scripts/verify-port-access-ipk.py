#!/usr/bin/env python3
"""P43: check the actual packaged IPK, not just source build declarations."""
import argparse
import io
import json
from pathlib import Path
import re
import tarfile

EXPECTED_UI = {'index.html','app.js','module.css','config-editor.js','odin-ui.js',
    'ui-rebuild.js','ui-rebuild.css','engine-wizard.js','operation-log.js',
    'p39-compact.js','p39-compact.css'}
ROOT = 'opt/share/routerforge/modules/port-access-manager/'
EXECUTABLES = {'opt/bin/routerforge-port-access-manager',
    'opt/bin/routerforge-port-access-watchdog',
    'opt/etc/init.d/S95routerforge-port-access-manager',
    ROOT + 'odin-engine.sh', ROOT + 'engine-gate.sh'}
REQUIRED = EXECUTABLES | {ROOT+'ui/'+name for name in EXPECTED_UI} | {
    ROOT+'manifest.json', 'opt/share/licenses/routerforge-port-access-manager/LICENSE'}


def unpack(tarbytes, label):
    with tarfile.open(fileobj=io.BytesIO(tarbytes), mode='r:gz') as tf:
        entries = {}
        for member in tf.getmembers():
            raw = member.name
            path = raw.removeprefix('./')
            if not path or path == '.':
                continue
            parts = path.split('/')
            if path.startswith('/') or '\\' in path or '..' in parts or any(not p for p in parts):
                raise ValueError(f'{label}: unsafe path {raw!r}')
            if member.issym() or member.islnk() or member.isdev() or member.isfifo():
                raise ValueError(f'{label}: unsafe entry {raw!r}')
            if path in entries:
                raise ValueError(f'{label}: duplicate entry {raw!r}')
            if not (member.isdir() or member.isfile()):
                raise ValueError(f'{label}: unsupported entry {raw!r}')
            if member.mode & 0o6000:
                raise ValueError(f'{label}: setuid/setgid {raw!r}')
            entries[path] = (member, tf.extractfile(member).read() if member.isfile() else None)
        return entries


def verify(path):
    with tarfile.open(path, 'r:gz') as outer:
        members = outer.getmembers()
        names = [x.name.removeprefix('./') for x in members]
        if set(names) != {'debian-binary','control.tar.gz','data.tar.gz'} or len(names) != 3:
            raise ValueError('IPK outer members differ from expected three files')
        if any(not x.isfile() for x in members):
            raise ValueError('IPK outer contains non-file member')
        archive = {x.name.removeprefix('./'): outer.extractfile(x).read() for x in members}
    if archive['debian-binary'] != b'2.0\n':
        raise ValueError('incorrect ipk format version')
    control = unpack(archive['control.tar.gz'], 'control')
    data = unpack(archive['data.tar.gz'], 'data')
    files = {k for k,(m,_) in data.items() if m.isfile()}
    missing = REQUIRED - files
    if missing:
        raise ValueError('missing packaged files: ' + ', '.join(sorted(missing)))
    for name in EXECUTABLES:
        m, payload = data[name]
        if not (m.mode & 0o111) or not payload:
            raise ValueError('missing executable permission or empty: ' + name)
    for name in ('opt/bin/routerforge-port-access-manager','opt/bin/routerforge-port-access-watchdog'):
        if data[name][1][:4] != b'\x7fELF':
            raise ValueError('invalid ELF magic: ' + name)
    for name in ('postinst','prerm'):
        if name not in control or not (control[name][0].mode & 0o111):
            raise ValueError('missing control executable: '+name)
    meta = control.get('control')
    if not meta or b'Package: routerforge-port-access-manager\n' not in meta[1] or b'Depends: routerforge-core\n' not in meta[1]:
        raise ValueError('wrong package metadata')
    manifest = json.loads(data[ROOT+'manifest.json'][1])
    if any(manifest.get(k) != v for k,v in {
        'id':'port-access-manager', 'socket':'/opt/var/run/routerforge-port-access-manager.sock',
        'api_base':'/api/modules/port-access-manager',
        'ui_entry':'/api/modules/port-access-manager/ui/index.html'}.items()):
        raise ValueError('manifest contract mismatch')
    html = data[ROOT+'ui/index.html'][1].decode('utf-8')
    scripts = re.findall(r'<script\s+src="([^\"]+)"', html)
    if not scripts or any('/' in x or x not in EXPECTED_UI for x in scripts):
        raise ValueError('unsafe/missing JS entry')
    if any(ROOT+'ui/'+x not in files for x in scripts):
        raise ValueError('JS asset missing')
    print(f'P43_IPK_CONTENTS: PASS; FILES={len(files)}; SCRIPTS={len(scripts)}',flush=True)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('package',type=Path)
    args = p.parse_args()
    try:
        verify(args.package)
    except (OSError, ValueError, tarfile.TarError, json.JSONDecodeError) as e:
        print('P43_IPK_CONTENTS: FAIL: '+str(e),flush=True)
        return 1
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
