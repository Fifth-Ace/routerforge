#!/bin/sh
# Mandatory Linux/WSL gates for the later user-run P21 publisher. No remote writes.
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"
for tool in go node npm python3 gcc tar readelf; do
 command -v "$tool" >/dev/null || { echo "MISSING_TEST_TOOL=$tool" >&2; exit 1; }
done
[ "$(uname -s)" = Linux ] || { echo 'Linux/WSL required for firewall/process tests and race detector.' >&2; exit 1; }
# A restricted host must fail here, before a user-run push; no skipped socket gate.
python3 - <<'PY'
import socket,tempfile,os
try:
    with tempfile.TemporaryDirectory(prefix='rf-p21-') as d:
        s=socket.socket(socket.AF_UNIX)
        try: s.bind(os.path.join(d,'probe.sock'))
        finally: s.close()
except OSError as e:
    raise SystemExit('AF_UNIX_GATE=BLOCKED: %s; full Linux tests required before push' % e)
print('AF_UNIX_GATE=PASS')
PY
export CGO_ENABLED=1
export GOTOOLCHAIN=go1.22.12
go test -count=1 ./...
go test -race -count=1 ./modules/dns-router-runtime/... ./components/core/...
go vet ./...
node --check modules/dns-router-lite/frontend/app.js
npm ci --prefix modules/dns-router-lite/frontend-tests --ignore-scripts --no-audit --no-fund
npm test --prefix modules/dns-router-lite/frontend-tests
(
 cd modules/dns-router-lite
 export GOTOOLCHAIN=go1.26.8
 binary="$(mktemp /tmp/rf-p21-test.XXXXXX)"
 trap 'rm -f "$binary"' EXIT HUP INT TERM
 go build -tags dns_router_lite -o "$binary" ./cmd/routerforge-dns-router
 GITHUB_ACTIONS=true RF_DNS_LITE_TEST_BINARY="$binary" go test -tags dns_router_lite -race -count=1 ./...
 go test -tags dns_router_lite -race -run 'TestP21|TestP19(Status|Supervisor|Intent|Listener)' -count=10 ./...
 go vet -tags dns_router_lite ./...
)
export GOTOOLCHAIN=auto
for target in aarch64-3.10 mips-3.4 mipsel-3.4; do
 ROUTERFORGE_CHANNEL=dev sh scripts/build-dns-router-experimental.sh '1~dev.local.p21' "$target"
 python3 scripts/verify-beta-dns-router-package.py "dist/routerforge-dns-router_1~dev.local.p21_${target}.ipk" "$target" '1~dev.local.p21'
done
git diff --check
echo 'P21_LOCAL_GATES=PASS / HARDWARE_PENDING'
