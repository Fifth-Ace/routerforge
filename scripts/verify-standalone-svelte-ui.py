#!/usr/bin/env python3
"""Fail-closed native Svelte 5 UI / IPK assertions for three standalone modules.

No network, no router access, no mutations. The legacy CSS hashes are exact
Git blob identities from the pinned DEV base. HTML references are relative.
"""
from __future__ import annotations

import argparse
import hashlib
from html.parser import HTMLParser
import io
from pathlib import Path
import tarfile

BASE_CSS = {
    'crowdsec-manager': '12adf3b30250459065db78d9f179ff7bcf9a0441',
    'network-tools': '41d9e6e1bc1693cec7f443254d15078857965361',
    'antiscan-manager': '38a78616536d050a886c2c1750254ed976bb1e3d',
}

class Links(HTMLParser):
    def __init__(self):
        super().__init__()
        self.references = []
    def handle_starttag(self, tag, attrs):
        d = dict(attrs)
        if tag == 'script' and d.get('src'):
            self.references.append(('js', d['src']))
        if tag == 'link' and d.get('rel') == 'stylesheet' and d.get('href'):
            self.references.append(('css', d['href']))

def git_blob(raw: bytes) -> str:
    header = b'blob ' + str(len(raw)).encode() + b'\0'
    return hashlib.sha1(header + raw).hexdigest()

def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]

def verify_source(module: str, root: Path) -> None:
    front = root / 'modules' / module / 'frontend'
    if not front.is_dir():
        raise ValueError(f'FRONTEND_NOT_FOUND: {module}')
    if (front / 'app.js').exists():
        raise ValueError(f'LEGACY_CONTROLLER_PRESENT: {module}')
    css = (front / 'module.css').read_bytes()
    if git_blob(css) != BASE_CSS[module]:
        raise ValueError(f'BASE_CSS_DRIFT: {module}: {git_blob(css)}')
    source_name = {
        'crowdsec-manager': 'CrowdSecManagerApp.svelte',
        'network-tools': 'NetworkToolsApp.svelte',
        'antiscan-manager': 'AntiscanManagerApp.svelte',
    }[module]
    source = (front / source_name).read_text(encoding='utf-8')
    entry = (front / 'main.js').read_text(encoding='utf-8')
    html = (front / 'index.html').read_text(encoding='utf-8')
    if "from 'svelte'" not in entry or f"from './{source_name}'" not in entry:
        raise ValueError(f'SVELTE_MOUNT_CONTRACT_FAIL: {module}')
    if 'id="app"' not in html or 'app.js' in html:
        raise ValueError(f'SVELTE_HTML_CONTRACT_FAIL: {module}')
    if len(source) < 10000 or '$state' not in source or 'onMount' not in source:
        raise ValueError(f'SVELTE_COMPONENT_NOT_OWNING_UI: {module}')
    if 'eval(' in source or 'new Function(' in source or 'document.write(' in source:
        raise ValueError(f'UNSAFE_DYNAMIC_EXECUTION: {module}')
    print(f'SOURCE_SVELTE5_{module.upper()}: PASS / CSS_ORIGINAL / NO_OLD_APP_JS')

def verify_bundle(files: dict[str, bytes], module: str) -> None:
    if 'index.html' not in files or not files['index.html']:
        raise ValueError(f'INDEX_NOT_BUILT: {module}')
    if any(x in files for x in ('app.js', 'p22-visual.js', 'p25-settings.js')):
        raise ValueError(f'OLD_CONTROLLERS_IN_IPK: {module}')
    html = files['index.html'].decode('utf-8')
    if 'id="app"' not in html or 'app.js' in html:
        raise ValueError(f'LEGACY_INDEX_IN_BUILD: {module}')
    parser = Links()
    parser.feed(html)
    kinds = {'js': [], 'css': []}
    for typ, link in parser.references:
        if '://' in link or link.startswith(('/', '//', 'data:')):
            raise ValueError(f'NON_RELATIVE_ASSET: {module} {link}')
        asset = link.split('?', 1)[0].split('#', 1)[0].lstrip('./')
        if asset.startswith('../') or '..' in asset.split('/') or not asset.startswith('assets/'):
            raise ValueError(f'ASSET_OUT_OF_SCOPE: {module} {asset}')
        if asset not in files or not files[asset]:
            raise ValueError(f'MISSING_BUNDLED_ASSET: {module} {asset}')
        if typ == 'js' and not asset.endswith('.js'):
            raise ValueError(f'NON_JS_SCRIPT_ASSET: {module} {asset}')
        if typ == 'css' and not asset.endswith('.css'):
            raise ValueError(f'NON_CSS_STYLE_ASSET: {module} {asset}')
        kinds[typ].append(asset)
    if not kinds['js'] or not kinds['css']:
        raise ValueError(f'NO_COMPILED_SVELTE_ASSETS: {module} {kinds}')
    if not any(len(files[x]) > 2000 for x in kinds['js']):
        raise ValueError(f'TRIVIAL_JS_WRAPPER: {module}')
    print(f'SVELTE5_BUNDLE_{module.upper()}: PASS / assets={len(kinds["js"])+len(kinds["css"])}')

def verify_directory(path: Path, module: str) -> None:
    if not path.is_dir():
        raise ValueError(f'BUILD_DIR_NOT_FOUND: {path}')
    files = {str(p.relative_to(path)).replace('\\', '/'): p.read_bytes()
             for p in path.rglob('*') if p.is_file()}
    verify_bundle(files, module)

def verify_ipk(path: Path, module: str) -> None:
    with tarfile.open(path, 'r:gz') as outer:
        entries = [m for m in outer if m.isfile() and m.name.lstrip('./') == 'data.tar.gz']
        if len(entries) != 1:
            raise ValueError('IPK_PAYLOAD_MISSING')
        raw = outer.extractfile(entries[0]).read()
    prefix = f'opt/share/routerforge/modules/{module}/ui/'
    files = {}
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:gz') as inner:
        for item in inner:
            name = item.name.lstrip('./')
            if item.isfile() and name.startswith(prefix):
                logical = name[len(prefix):]
                if not logical or '..' in logical.split('/') or logical in files:
                    raise ValueError('UNSAFE_OR_DUPLICATE_IPK_FILE: ' + name)
                files[logical] = inner.extractfile(item).read()
    verify_bundle(files, module)
    print(f'SVELTE5_IPK_{module.upper()}: PASS / files={len(files)}')

def selftest() -> None:
    sample = {
        'index.html': b'<html><head><link rel="stylesheet" href="./assets/app.css"></head><body><div id="app"></div><script type="module" src="./assets/index-abc123.js"></script></body></html>',
        'assets/index-abc123.js': b'const native_svelte5=true;'+b'x'*4000,
        'assets/app.css': b'.as{color:inherit}',
    }
    verify_bundle(sample, 'antiscan-manager')
    for desc, poisoned in [
        ('legacy', dict(sample, **{'app.js': b'old legacy'})),
        ('missing', {k:v for k,v in sample.items() if k != 'assets/index-abc123.js'}),
        ('remote', dict(sample, **{'index.html': sample['index.html'].replace(b'./assets/index-abc123.js', b'https://example.com/a.js')})),
    ]:
        try:
            verify_bundle(poisoned, 'antiscan-manager')
        except ValueError:
            continue
        raise ValueError(f'SELFTEST_FAIL_OPEN: {desc}')
    print('SVELTE5_ASSET_SELFTEST: PASS')

def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--module', choices=list(BASE_CSS))
    parser.add_argument('--dir', type=Path)
    parser.add_argument('--ipk', type=Path)
    parser.add_argument('--source', action='store_true')
    parser.add_argument('--selftest', action='store_true')
    args = parser.parse_args()
    if args.selftest:
        if args.module or args.dir or args.ipk or args.source:
            parser.error('selftest accepts no other options')
        selftest()
        return
    if not args.module:
        parser.error('--module is required')
    if not args.source and args.dir is None and args.ipk is None:
        parser.error('use --source, --dir or --ipk')
    verify_source(args.module, repo_root())
    if args.dir:
        verify_directory(args.dir, args.module)
    if args.ipk:
        verify_ipk(args.ipk, args.module)

if __name__ == '__main__':
    main()
