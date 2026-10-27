#!/bin/sh
# Offline CI acceptance for DNS Router standalone lab packages.
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VERSION=0.14.0~dev.dpr2d
cd "$ROOT"
sh -n scripts/build-dns-router-experimental.sh
GOPROXY=off go test ./modules/dns-router-runtime/...
GOPROXY=off go vet ./modules/dns-router-runtime/...
# Same API acceptance tests run under race detection on the CI Linux runner.
GOPROXY=off go test -race ./modules/dns-router-runtime/...
GOPROXY=off go test -count=1 -v -run 'TestDPR2D' ./modules/dns-router-runtime/...
for TARGET in aarch64-3.10 mips-3.4 mipsel-3.4; do
  sh scripts/build-dns-router-experimental.sh "$VERSION" "$TARGET"
done
python3 - "$VERSION" <<'PY'
import io, pathlib, sys, tarfile
v=sys.argv[1]
root=pathlib.Path('dist')
for arch in ('aarch64-3.10','mips-3.4','mipsel-3.4'):
    p=root/f'routerforge-dns-router_{v}_{arch}.ipk'
    assert p.is_file() and p.stat().st_size > 10000, p
    with tarfile.open(p,'r:gz') as outer:
        raw=outer.extractfile(next(m for m in outer if m.name.lstrip('./')=='control.tar.gz')).read()
    with tarfile.open(fileobj=io.BytesIO(raw),mode='r:gz') as ctl:
        control=ctl.extractfile(next(m for m in ctl if m.name.lstrip('./')=='control')).read().decode()
    assert f'Package: routerforge-dns-router\n' in control
    assert f'Architecture: {arch}\n' in control
    assert f'Version: {v}\n' in control
    assert 'Depends: routerforge-core\n' in control
    assert 'routerforge-dns,' not in control
    assert 'No autostart' in control
    with tarfile.open(p,'r:gz') as outer:
        payload=outer.extractfile(next(m for m in outer if m.name.lstrip('./')=='data.tar.gz')).read()
    with tarfile.open(fileobj=io.BytesIO(payload),mode='r:gz') as data:
        names={m.name.lstrip('./') for m in data}
    prefix='opt/share/routerforge/modules/dns-router/'
    for required in (prefix+'manifest.json', prefix+'ui/index.html', prefix+'ui/app.js', prefix+'ui/module.css'):
        assert required in names, required
    assert not any(n.endswith('/postinst') or n.endswith('/S99routerforge-dns-router') for n in names), 'unexpected autostart'
    assert 'opt/bin/routerforge-dns-router' in names
    import json
    with tarfile.open(fileobj=io.BytesIO(payload),mode='r:gz') as data:
        m=data.extractfile(next(x for x in data if x.name.lstrip('./')==prefix+'manifest.json'))
        manifest=json.load(m)
    assert manifest['id']=='dns-router'
    assert manifest['mode']=='manual-shadow-dns-opt-in'
    print(f'DNS_ROUTER_IPK_{arch}: PASS ({p.stat().st_size} bytes)')
print('DNS_ROUTER_EXPERIMENTAL_MATRIX: PASS (3/3; manual loopback shadow DNS; no publish)')
PY
