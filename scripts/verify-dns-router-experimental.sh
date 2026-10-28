#!/bin/sh
# Offline CI acceptance for DNS Router standalone lab packages.
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VERSION=0.14.0~dev.dpr2o3
cd "$ROOT"
sh -n scripts/build-dns-router-experimental.sh
sh -n scripts/dpr2-f-hardware-readonly.sh
sh -n scripts/dpr2-g-hardware-readonly.sh
sh -n scripts/dpr2-h-dstest-readonly.sh
# Independent hardware report CLI requires no DNS config or listeners.
go build -o dist/dpr2f-local-report ./modules/dns-router-runtime/cmd/routerforge-dns-router
./dist/dpr2f-local-report --hardware-report | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["mode"]=="read-only" and x["changes_applied"] is False and x["integration_plan"]["activation_allowed"] is False'
# DPR2-G raw collector and offline CLI assessment, deliberately on local CI host
# (it does not provide hardware acceptance for Keenetic).
sh scripts/dpr2-g-hardware-readonly.sh > dist/dpr2g-ci-evidence.txt
./dist/dpr2f-local-report --assess-hardware-log dist/dpr2g-ci-evidence.txt | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["mode"]=="offline-read-only-assessment" and x["changes_applied"] is False and x["activation_allowed"] is False and x["hardware_accepted"] is False'
./dist/dpr2f-local-report --assess-hardware-log dist/dpr2g-ci-evidence.txt | grep -q 'INSUFFICIENT_EVIDENCE'
# H: the CI runner is not a Keenetic device, so this fixture must NEVER
# be presented as physical hardware acceptance.
cat > dist/dpr2h-fixture.txt <<'FIXTURE'
RF_DPR2H_EVIDENCE_BEGIN
SCHEMA=1
MODE=READ_ONLY
ARCH=aarch64
DSTEST_IP_MATCH=YES
OPT_DIR=YES
ENTWARE_OPKG=YES
MEM_AVAILABLE_KIB=128000
OPT_FREE_KIB=152000
PORT53_UDP_ENTRIES=2
PORT53_TCP_ENTRIES=1
SHADOW_55353_ENTRIES=0
NDMC_VERSION=OK
NDMC_DNS=OK
DNS_OVERRIDE=UNKNOWN
LOCAL_NAMES=NOT_TESTED
SHADOW_UDP_TCP=NOT_TESTED
LOOP_DETECTION=NOT_TESTED
ROLLBACK=NOT_TESTED
DNS_INTERCEPTION=NONE
NFQUEUE_300=NOT_TOUCHED
RF_DPR2H_EVIDENCE_END
FIXTURE
./dist/dpr2f-local-report --assess-h-report dist/dpr2h-fixture.txt | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["mode"]=="read-only-hardware-inventory" and x["activation_allowed"] is False and x["shadow_hardware_pass"] is False and x["hardware_accepted"] is False'
GOPROXY=off go test ./modules/dns-router-runtime/...
GOPROXY=off go vet ./modules/dns-router-runtime/...
# Same API acceptance tests run under race detection on the CI Linux runner.
GOPROXY=off go test -race ./modules/dns-router-runtime/...
GOPROXY=off go test -count=1 -v -run 'TestDPR2(D|E|F|G|K|L|M|N)' ./modules/dns-router-runtime/...
GOPROXY=off go test -run '^$' -bench '^BenchmarkDPR2EEvaluate10K$' -benchtime=100ms -benchmem ./modules/dns-router-runtime/...
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

# DPR2-L frontend API wiring and syntactic contract, no browser or router dependency.
python3 - <<'PY_VERIFY_L'
from pathlib import Path
html = Path('modules/dns-router-runtime/frontend/index.html').read_text()
js = Path('modules/dns-router-runtime/frontend/app.js').read_text()
for element in ('listenerForm','listenerMode','listenerListen','listenerACL','listenerPlanButton','listenerPlanStatus'):
    assert f'id="{element}"' in html and "el('"+element+"')" in js, element
assert 'keenetic/domain-plan' in js and 'shadow-lan-experimental' in html
assert 'no ip name-server' in Path('modules/dns-router-runtime/listener_l.go').read_text()
print('DPR2_L_FRONTEND_CONTRACT: PASS')
PY_VERIFY_L
if command -v node >/dev/null 2>&1; then node --check modules/dns-router-runtime/frontend/app.js; fi

# DPR2-M test-only ndmc transaction. All CI side effects use injected
# drivers or an ephemeral mock ndmc, never router configuration.
GOPROXY=off go test -count=1 -v -run '^TestDPR2M' ./modules/dns-router-runtime/...
python3 - <<'PY_VERIFY_M'
from pathlib import Path
p=Path('modules/dns-router-runtime/transaction_m.go').read_text()
c=Path('modules/dns-router-runtime/cmd/routerforge-dns-router/main.go').read_text()
a=Path('modules/dns-router-runtime/api.go').read_text()
for s in ('dpr2m.test','192.168.10.1:55358','192.168.10.1:55354','WATCHDOG_ROLLBACK_PASS','no ip name-server','exact saved revision'):
    assert s in p, s
for s in ('ndmc-rollback-worker','ndmc-transaction-experimental','NDMCTestHostGateM'):
    assert s in c, s
assert 'keenetic/domain-trial' in a and 'If-Match' in a
assert 'system configuration save' not in p.split('func ')[-1]
print('DPR2_M_TRANSACTION_GATES: PASS')
PY_VERIFY_M

# DPR2-N bounded multi-zone transactional trial, no real ndmc on CI.
GOPROXY=off go test -count=1 -v -run '^TestDPR2N' ./modules/dns-router-runtime/...
python3 - <<'PY_VERIFY_N'
from pathlib import Path
p = Path('modules/dns-router-runtime/transaction_n.go').read_text()
a = Path('modules/dns-router-runtime/api.go').read_text()
c = Path('modules/dns-router-runtime/cmd/routerforge-dns-router/main.go').read_text()
h = Path('modules/dns-router-runtime/frontend/index.html').read_text()
j = Path('modules/dns-router-runtime/frontend/app.js').read_text()
for needle in ('managedMaxZonesN = 8', 'ManagedWorkerN', 'trialSnapshotM', 'BaselineRestored', 'syscall.Flock'):
    assert needle in p, needle
assert 'ndmc-managed-trial-experimental' in c
assert 'domain-managed/prepare' in a and 'domain-managed/trial' in a
for v in ('managedPrepareN','managedTrialN','managedStatusN'):
    assert f'id="{v}"' in h and f"el('{v}')" in j
assert 'system configuration save' not in p
print('DPR2_N_MULTIZONE_TRANSACTION_AND_UI: PASS')
PY_VERIFY_N

# DPR2-O1 durable ownership ledger and read-only reconciliation. No ndmc
# mutations, no daemon autostart and no persistent apply in this stage.
GOPROXY=off go test -count=1 -v -run '^TestDPR2O1' ./modules/dns-router-runtime/...
python3 - <<'PY_VERIFY_O1'
from pathlib import Path
p=Path('modules/dns-router-runtime/managed_o.go').read_text()
a=Path('modules/dns-router-runtime/api.go').read_text()
c=Path('modules/dns-router-runtime/cmd/routerforge-dns-router/main.go').read_text()
h=Path('modules/dns-router-runtime/frontend/index.html').read_text()
j=Path('modules/dns-router-runtime/frontend/app.js').read_text()
for needle in ('SaveCAS', 'safety.WriteFileAtomic', 'pending_recovery', 'stale_policy', 'ReconcileLedgerO'):
    assert needle in p, needle
assert 'managed-ownership/status' in a and 'managed-state.json' in c
for x in ('managedOwnershipStatusO', 'managedOwnershipResultO'):
    assert f'id="{x}"' in h and f"el('{x}')" in j
assert 'ndmc-managed-trial-experimental' in c
print('DPR2_O1_PERSISTENCE_AND_READONLY_UI: PASS')
PY_VERIFY_O1

# DPR2-O2: persistent DNS changes are opt-in and only possible on DSTest.
# CI never issues real ndmc mutations: tests use injected executors/guards.
GOPROXY=off go test -count=1 -v -run '^TestDPR2O2' ./modules/dns-router-runtime/...
python3 - <<'PY_VERIFY_O2'
from pathlib import Path
p=Path('modules/dns-router-runtime/managed_o2.go').read_text()
l=Path('modules/dns-router-runtime/managed_o.go').read_text()
a=Path('modules/dns-router-runtime/api.go').read_text()
c=Path('modules/dns-router-runtime/cmd/routerforge-dns-router/main.go').read_text()
for needle in ('lockNDMCForO2', 'pending', 'deactivating', 'ensureExactOwnedO2', 'ArmDisableO2', 'DurableDisableWorkerO2', 'HostGate'):
    assert needle in p, needle
assert 'safety.CommandContext' in p and 'WATCHDOG_O2_RESTORE_PASS' in p
assert 'DeleteCAS' in l and 'safety.WriteFileAtomic' in l
for endpoint in ('durable/prepare','durable/apply','durable/disable'):
    assert endpoint in a
assert 'ndmc-durable-experimental' in c and 'ndmc-durable-disable-worker' in c
assert '--shadow-forward' in c and 'NDMCTestHostGateM' in c
assert 'system configuration save' not in p.lower()
print('DPR2_O2_DURABLE_TRANSACTION_AND_REVERSE_WATCHDOG: PASS')
PY_VERIFY_O2

# DPR2-O3: startup reconciliation remains strictly explicit on DSTest.
# CI uses injected ndmc and watchdog fixtures, never real router mutations.
GOPROXY=off go test -count=1 -v -run '^TestDPR2O3' ./modules/dns-router-runtime/...
python3 - <<'PY_VERIFY_O3'
from pathlib import Path
p=Path('modules/dns-router-runtime/managed_o3.go').read_text()
t=Path('modules/dns-router-runtime/managed_o3_test.go').read_text()
a=Path('modules/dns-router-runtime/api.go').read_text()
c=Path('modules/dns-router-runtime/cmd/routerforge-dns-router/main.go').read_text()
assert 'lockNDMCForO2' in p and 'checkGuardsO3' in p
assert 'classifyStartupO3' in p and 'ReconcileLedgerO' not in p
assert 'rollbackManagedN' in p and 'guard.Finish()' in p
assert 'pending_recovery_required' not in p # state is composed from ledger, not guessed
assert 'if ledger.State != "active"' in p
assert 'm.Probe' in p and 'safety.CommandContext' not in p
assert 'TestDPR2O3RebootRestoresOwnedZones' in t
assert 'TestDPR2O3RegistrationFailureRollsBackAndPreservesLedger' in t
assert 'TestDPR2O3PendingLedgerBlocked' in t
assert '/v1/keenetic/startup/status' in a and 'http.MethodGet' in a
assert 'ndmc-startup-recovery-experimental' in c
assert 'startupOptIn && !*durableOptIn' in c
print('DPR2_O3_EXPLICIT_STARTUP_RECONCILIATION: PASS')
PY_VERIFY_O3
