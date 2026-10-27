#!/bin/sh
# Offline CI acceptance for DNS Router standalone lab packages.
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VERSION=0.14.0~dev.dpr2a
cd "$ROOT"
sh -n scripts/build-dns-router-experimental.sh
GOPROXY=off go test ./modules/dns-router-runtime/...
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
    print(f'DNS_ROUTER_IPK_{arch}: PASS ({p.stat().st_size} bytes)')
print('DNS_ROUTER_EXPERIMENTAL_MATRIX: PASS (3/3; no publish)')
PY
