#!/usr/bin/env python3
"""Read-only source inventory for the Svelte 5 migration (not a build/parity proof)."""
from __future__ import annotations
import argparse
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
FRONTENDS = {
    'core': 'components/core/frontend',
    'admin': 'modules/admin/frontend',
    'monitoring': 'modules/monitoring/frontend',
    'dns': 'modules/dns/frontend',
    'crowdsec-manager': 'modules/crowdsec-manager/frontend',
    'network-tools': 'modules/network-tools/frontend',
    'antiscan-manager': 'modules/antiscan-manager/frontend',
    'dns-router-lite': 'modules/dns-router-lite/frontend',
    'dns-router-runtime': 'modules/dns-router-runtime/frontend',
    'nfqws-manager': 'modules/nfqws-manager/frontend',
    'port-access-manager': 'modules/port-access-manager/frontend',
}
LEGACY_CONTROLLERS = {
    'dns-router-lite': ('app.js', 'p22-visual.js', 'p25-settings.js'),
    'dns-router-runtime': ('app.js',),
    'port-access-manager': ('app.js', 'ui-rebuild.js', 'odin-ui.js', 'engine-wizard.js',
                            'operation-log.js', 'config-editor.js', 'p39-compact.js',
                            'p53-ux.js', 'p54-workspace.js', 'p55-antiscan.js'),
}


def inspect(name: str, root: Path) -> list[str]:
    d = root / FRONTENDS[name]
    if not d.is_dir():
        return ['FRONTEND_DIRECTORY_MISSING']
    errors = []
    components = tuple(d.rglob('*.svelte'))
    if not components:
        errors.append('SVELTE_COMPONENT_MISSING')
    package = d / 'package.json'
    if not package.is_file() or '"svelte"' not in package.read_text(encoding='utf-8'):
        errors.append('SVELTE_PACKAGE_MISSING')
    if name != 'core':
        # Port Access uses an IIFE library build from status-main.js, not main.js.
        entry = d / ('status-main.js' if name == 'port-access-manager' else 'main.js')
        if not entry.is_file():
            errors.append('SVELTE_ENTRY_MISSING')
        else:
            source = entry.read_text(encoding='utf-8')
            if 'from \'svelte\'' not in source and 'from "svelte"' not in source:
                errors.append('SVELTE_IMPORT_MISSING')
            if not any(p.name in source for p in components):
                errors.append('SVELTE_COMPONENT_NOT_MOUNTED')
    for file in LEGACY_CONTROLLERS.get(name, ()):
        if (d / file).is_file():
            errors.append('LEGACY_CONTROLLER:' + file)
    if name == 'nfqws-manager' and (d / 'index.html').is_file():
        if (d / 'index.html').stat().st_size > 100_000:
            errors.append('LEGACY_MONOLITHIC_HTML')
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument('--report-only', action='store_true', help='report incomplete migration without failing CI')
    parser.add_argument('--root', type=Path, default=ROOT)
    args = parser.parse_args()
    good = 0
    for name in FRONTENDS:
        failures = inspect(name, args.root)
        good += not failures
        print(f'{name}: {"PASS" if not failures else "INCOMPLETE"}')
        for issue in failures:
            print('  ' + issue)
    print(f'SVELTE_SOURCE_INVENTORY={good}/{len(FRONTENDS)}')
    if good != len(FRONTENDS):
        print('FULL_SVELTE_MIGRATION=NOT_VERIFIED')
        return 0 if args.report_only else 1
    print('SVELTE_SOURCE_INVENTORY=PASS (BUILD_AND_PARITY_STILL_REQUIRED)')
    return 0


if __name__ == '__main__':
    sys.exit(main())
