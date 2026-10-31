#!/usr/bin/env python3
"""Collect full license notices for modules actually compiled into DNS Lite."""
import json
import pathlib
import subprocess
import sys

source, output = map(pathlib.Path, sys.argv[1:])
raw = subprocess.check_output(['go', 'list', '-buildvcs=false', '-tags', 'dns_router_lite', '-deps', '-json', './cmd/routerforge-dns-router'], cwd=source, text=True)
decoder = json.JSONDecoder()
modules = {}
while raw.strip():
    raw = raw.lstrip()
    item, end = decoder.raw_decode(raw)
    raw = raw[end:]
    module = item.get('Module', {})
    if module.get('Path') and module.get('Dir'):
        modules[module['Path']] = module
output.mkdir(parents=True, exist_ok=True)
goroot = pathlib.Path(subprocess.check_output(['go', 'env', 'GOROOT'], cwd=source, text=True).strip())
(output / 'Go-LICENSE').write_bytes((goroot / 'LICENSE').read_bytes())
manifest = []
for name, module in sorted(modules.items()):
    directory = pathlib.Path(module.get('Replace', module).get('Dir', module['Dir']))
    notices = [p for p in directory.iterdir() if p.is_file() and (p.name.upper().startswith(('LICENSE', 'COPYING', 'NOTICE')) or p.name.upper() == 'PATENTS')]
    if not notices:
        raise SystemExit('license notice missing: ' + name)
    destination = output / name.replace('/', '__')
    destination.mkdir(exist_ok=True)
    for path in notices:
        (destination / path.name).write_bytes(path.read_bytes())
    manifest.append({'module': name, 'version': module.get('Version', 'local'), 'notices': [p.name for p in notices]})
(output / 'MODULES.json').write_text(json.dumps(manifest, indent=2) + '\n')
print('DNS_LITE_LICENSES: PASS /', len(manifest), 'compiled modules')
