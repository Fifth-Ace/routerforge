#!/usr/bin/env python3
"""Real Linux process + Unix HTTP, isolated tempfile, no Keenetic/ndmc.

Requires an explicitly supplied `dns_router_lite` test-tag binary. This is
RT-C4 partial evidence and MUST NOT be labeled full production QUIC or IPK.
"""
import hashlib
import http.client
import json
import os
import pathlib
import signal
import socket
import stat
import subprocess
import sys
import tempfile
import time


def check(condition, message):
    if not condition:
        raise AssertionError(message)


class UnixHTTP(http.client.HTTPConnection):
    def __init__(self, address):
        super().__init__('localhost', timeout=3)
        self.address = address

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.connect(self.address)


def request(path, method='GET', headers=None, body=None):
    client = UnixHTTP(str(socket_path))
    try:
        client.request(method, path, body=body, headers=headers or {})
        response = client.getresponse()
        raw = response.read()
        return response.status, json.loads(raw) if raw else None
    finally:
        client.close()


def start_process():
    global child
    child = subprocess.Popen([
        str(executable), '--config', str(config_path), '--serve',
        '--socket', str(socket_path), '--ui', str(work / 'ui-nonexistent')
    ], stdin=subprocess.DEVNULL, stdout=log_file,
       stderr=subprocess.STDOUT, cwd=work)
    for _ in range(150):
        if child.poll() is not None:
            raise AssertionError('runtime process exited before Unix HTTP startup')
        if socket_path.exists():
            return
        time.sleep(.02)
    raise AssertionError('Unix HTTP startup timeout')


def stop_process():
    global child
    if child is None:
        return
    if child.poll() is None:
        child.send_signal(signal.SIGTERM)
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait(timeout=5)
            raise AssertionError('runtime failed to stop after SIGTERM')
    check(child.returncode == 0, f'unexpected daemon exit: {child.returncode}')
    child = None
    check(not socket_path.exists(), 'shutdown left the Unix socket behind')


if len(sys.argv) != 3:
    raise SystemExit('usage: rt4_offline_process_e2e.py <TEST-ONLY-LITE-BINARY> <EVIDENCE-DIR>')
executable = pathlib.Path(sys.argv[1]).resolve()
evidence = pathlib.Path(sys.argv[2]).resolve()
evidence.mkdir(parents=True, exist_ok=True)
check(executable.is_file(), 'test-only Linux executable missing')
check(os.name == 'posix' and sys.platform.startswith('linux'), 'native Linux required')
work = pathlib.Path(tempfile.mkdtemp(prefix='rf-rtc4-process-'))
# No router address or actual management socket is used.
config_path = work / 'config.json'
socket_path = work / 'rf-test.sock'
child = None
log_file = (evidence / 'rt4-process.log').open('w')
result = {'mode': 'dns_router_lite test-only process', 'full_production_quic': False,
          'router_or_system_services_touched': False, 'checks': []}

def ok(name):
    result['checks'].append({'name': name, 'result': 'PASS'})
    print('PASS', name, flush=True)

try:
    start_process()
    status, payload = request('/v1/health')
    check(status == 200 and payload['status'] == 'ok', 'health GET')
    status, payload = request('/v1/status')
    check(status == 200 and payload['dns_listener'] is False and payload['dns_interception'] is False,
          'OFF start unexpectedly enabled DNS')
    status, payload = request('/v1/config')
    check(status == 200 and payload['active'] is False, 'config GET not offline')
    original_rev = payload['revision']
    check(not config_path.exists(), 'read-only startup created a config')
    check(not (work / 'config.json.managed-state.json').exists(), 'read-only startup created ownership ledger')
    ok('RTC4-PROCESS-01 / clean install reads are OFF and read-only')

    new_cfg = {'schema_version': 1,
               'upstreams': [{'id': 'cf', 'transport': 'udp', 'address': '1.1.1.1:53'}],
               'rules': []}
    data = json.dumps(new_cfg).encode()
    base_headers = {'Content-Type': 'application/json'}
    denied, _ = request('/v1/config', 'PUT', {**base_headers, 'If-Match': original_rev}, data)
    missing, _ = request('/v1/config', 'PUT', {**base_headers, 'X-RouterForge-Action': 'save'}, data)
    check(denied == 403 and missing == 428 and not config_path.exists(),
          f'failed 403/428 protect config; statuses {denied}/{missing}')
    ok('RTC4-PROCESS-02 / 403 and 428 do not write config')

    headers = {**base_headers, 'X-RouterForge-Action': 'save', 'If-Match': f'"{original_rev}"'}
    saved, payload = request('/v1/config', 'PUT', headers, data)
    check(saved == 200 and isinstance(payload.get('revision'), str), f'config save refused: {saved} {payload}')
    new_rev = payload['revision']
    check(new_rev != original_rev, 'saved config did not update revision')
    stale, _ = request('/v1/config', 'PUT', headers, data)
    check(stale == 409, f'stale CAS returned {stale}, not 409')
    check(stat.S_IMODE(config_path.stat().st_mode) == 0o600, 'config permissions not 0600')
    blob_before = hashlib.sha256(config_path.read_bytes()).hexdigest()
    ok('RTC4-PROCESS-03 / CAS write 200 and stale write 409')
    stop_process()

    # A new process and the same persistent on-disk config emulate the backend
    # restart segment of an OFF->OFF package upgrade (not a package upgrade).
    start_process()
    status, payload = request('/v1/status')
    check(status == 200 and payload['dns_listener'] is False and payload['dns_interception'] is False,
          'restart auto-enabled DNS')
    status, payload = request('/v1/config')
    check(status == 200 and payload['revision'] == new_rev and payload['active'] is False,
          'restart changed config or active state')
    check(hashlib.sha256(config_path.read_bytes()).hexdigest() == blob_before,
          'config bytes changed on restart')
    check(not (work / 'config.json.managed-state.json').exists(), 'restart invented DNS ownership')
    ok('RTC4-PROCESS-04 / OFF->OFF backend restart preserves config and ownership')
    stop_process()
    result['status'] = 'PASS'
finally:
    try:
        if child is not None:
            if child.poll() is None:
                child.terminate()
            child.wait(timeout=5)
    finally:
        log_file.close()
        if child is not None and child.poll() is None:
            child.kill()
        # Temp config may be important if a test fails; no automatic deletion of evidence.
        result.setdefault('status', 'FAIL')
        result['source_config_sha256'] = hashlib.sha256(config_path.read_bytes()).hexdigest() if config_path.exists() else None
        (evidence / 'rt4-process-result.json').write_text(json.dumps(result, indent=2), encoding='utf-8')
        import shutil
        shutil.rmtree(work, ignore_errors=True)
print('RT4_LOCAL_PROCESS_E2E: PASS / test-only binary / no QUIC signoff')
