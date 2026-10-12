#!/usr/bin/env python3
"""P40D fail-closed source/package release gates for Port Access Manager.

Static checks complement (not replace) the CI-built IPK and actual Keenetic acceptance.
"""
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
FRONT = ROOT / 'modules/port-access-manager/frontend'
RUNTIME = ROOT / 'modules/port-access-manager/runtime'
PKG = ROOT / 'modules/port-access-manager/packaging'
BUILD = ROOT / 'scripts/build-port-access-manager-opkg.sh'
CHECKS = []


def gate(name, condition):
    CHECKS.append((name, bool(condition)))
    print(('PASS' if condition else 'FAIL') + ': ' + name, flush=True)


def main():
    build = BUILD.read_text(encoding='utf-8')
    html = (FRONT / 'index.html').read_text(encoding='utf-8')
    script_src = re.findall(r'<script\s+src="([^"]+)"', html)
    shipped = re.search(r'for file in ([^\n]+); do\s*\n\s*cp ', build)
    shipped_names = set(shipped.group(1).split()) if shipped else set()
    gate('all frontend scripts shipped', bool(script_src) and all(s in shipped_names for s in script_src))
    gate('frontend CSS shipped', all(s in shipped_names for s in ('module.css','ui-rebuild.css','p39-compact.css')))
    gate('operation-log.js shipped', 'operation-log.js' in shipped_names and (FRONT/'operation-log.js').is_file())
    gate('no missing packaged frontend asset', all((FRONT / name).is_file() for name in shipped_names))
    gate('runtime and watchdog are built into IPK', all(x in build for x in ('./modules/port-access-manager/runtime','./modules/port-access-manager/watchdog','routerforge-port-access-watchdog')))
    gate('manifest and dependency declared', 'manifest.json' in build and 'Depends: routerforge-core' in build and 'routerforge-port-access-manager.sock' in build)
    gate('startup and uninstall scripts shipped', all(x in build for x in ('S95routerforge-port-access-manager','packaging/postinst','packaging/prerm')))
    gate('both managed engine helpers shipped', all(x in build for x in ('packaging/odin-engine.sh','packaging/engine-gate.sh')))
    server = (RUNTIME/'server.go').read_text(encoding='utf-8')
    gate('read-only logs endpoint registered', 'mux.HandleFunc("/v1/operation-logs", operationLogsHandler)' in server)
    gate('Core guarded mutation routes registered', all(x in server for x in ('/v1/engine-deploy','/v1/odin','/v1/recent-revoke')))
    gate('Unix socket refuses existing path', 'socket already exists; refusing to remove it' in server)
    gate('Unix socket restrictive permissions', 'os.Chmod(socket, 0600)' in server)
    log = (RUNTIME/'operation_log.go').read_text(encoding='utf-8')
    gate('bounded synchronized operation log', 'operationLogLimit = 320' in log and 'operationLog.Lock()' in log and 'operationLog.Unlock()' in log)
    gate('operation log is explicitly non-persistent', '"persistent": false' in log)
    gate('API and race regression sources exist', all((RUNTIME/name).is_file() for name in ('api_safety_contract_test.go','operation_log_reliability_test.go','operation_log_test.go')))
    gate('watchdog rollback smoke script exists', (ROOT/'scripts/verify-port-access-watchdog-package.sh').is_file())
    gate('watchdog package smoke checks both binaries', all(x in (ROOT/'scripts/verify-port-access-watchdog-package.sh').read_text(encoding='utf-8') for x in ('routerforge-port-access-manager routerforge-port-access-watchdog','TIMEOUT_ROLLBACK: PASS','CONFIRMATION_SKIPS_ROLLBACK: PASS')))
    gate('no release or firewall mutation by gate script', True)  # pure file reads above
    fails=[name for name, ok in CHECKS if not ok]
    print('P40D_RELEASE_GATES: '+('FAIL: '+', '.join(fails) if fails else 'PASS'),flush=True)
    return 1 if fails else 0

if __name__=='__main__': sys.exit(main())
