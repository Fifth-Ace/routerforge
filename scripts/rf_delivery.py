#!/usr/bin/env python3
"""RouterForge Delivery v3. Fail-closed commit/push/CI workflow.

Use: python scripts/rf_delivery.py doctor|check|ship|resume [--repo PATH]
Patch workflow: git apply --check first, then apply patch before `ship`.
"""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time

EXPECTED_REMOTE = 'https://github.com/Fifth-Ace/routerforge.git'
REPO_NAME = 'Fifth-Ace/routerforge'
STATE = '.git/rf-delivery-state.json'

class DeliveryError(Exception):
    pass

def run(argv, cwd=None, capture=True, check=True):
    p = subprocess.run(argv, cwd=cwd, text=True, encoding='utf-8', errors='replace',
                       stdout=subprocess.PIPE if capture else None,
                       stderr=subprocess.STDOUT if capture else None)
    if check and p.returncode:
        if capture and p.stdout: print(p.stdout.rstrip())
        raise DeliveryError(f'COMMAND_FAIL rc={p.returncode}: {" ".join(argv[:4])}')
    return p.stdout.rstrip('\r\n') if capture else p.returncode

def git(repo, *args):
    return run(['git', '-C', str(repo), *args])

def precheck(repo):
    print('=== PRECHECK ===', flush=True)
    for exe in ('git', 'gh', sys.executable):
        if exe != sys.executable and not shutil.which(exe): raise DeliveryError('MISSING_TOOL: '+exe)
    if not (repo / '.git').exists(): raise DeliveryError('NOT_GIT_WORKTREE: '+str(repo))
    if git(repo, 'branch', '--show-current') != 'dev': raise DeliveryError('BRANCH_NOT_DEV')
    origin = git(repo, 'remote', 'get-url', 'origin')
    if origin not in (EXPECTED_REMOTE, 'git@github.com:Fifth-Ace/routerforge.git'):
        raise DeliveryError('ORIGIN_MISMATCH: '+origin)
    auth = run(['gh', 'api', 'user', '--jq', '.login'])
    if not auth: raise DeliveryError('GH_AUTH_REQUIRED')
    print('GIT/GH/PYTHON: PASS; AUTH: '+auth, flush=True)

def remote_sha():
    data = run(['git', 'ls-remote', EXPECTED_REMOTE, 'refs/heads/dev'])
    match = re.fullmatch(r'([a-f0-9]{40})\s+refs/heads/dev', data)
    if not match: raise DeliveryError('REMOTE_SHA_PARSE_FAIL')
    return match.group(1)

def changed_paths(repo):
    rows = git(repo, 'status', '--porcelain=v1', '--untracked-files=all').splitlines()
    paths = []
    for row in rows:
        if not row: continue
        item = row[3:]
        if ' -> ' in item: item = item.split(' -> ', 1)[-1]
        paths.append(item.strip('"'))
    return paths

def gates(repo, allow):
    print('=== GATES ===', flush=True)
    base = git(repo, 'rev-parse', 'HEAD')
    remote = remote_sha()
    if base != remote: raise DeliveryError('REMOTE_DEV_MOVED: '+remote)
    paths = changed_paths(repo)
    if not paths: raise DeliveryError('NO_CHANGES')
    if allow:
        illegal = [p for p in paths if p not in allow]
        if illegal: raise DeliveryError('SCOPE_VIOLATION: '+', '.join(illegal))
    for x in paths:
        if x.startswith('.git/') or x.startswith('../') or x.startswith('/'):
            raise DeliveryError('UNSAFE_PATH: '+x)
    git(repo, 'diff', '--check')
    git(repo, 'diff', '--cached', '--check')
    print('BASE: '+base+'; PATHS: '+str(len(paths))+'; PASS', flush=True)
    return base, paths

def state_file(repo): return repo / STATE

def write_state(repo, info):
    dest=state_file(repo)
    temp=dest.with_suffix('.tmp')
    temp.write_text(json.dumps(info, indent=2), encoding='utf-8')
    temp.replace(dest)

def find_run(sha):
    # gh --jq emits ONE scalar only. Never stringify a collection of run IDs.
    expr = f'[.[] | select(.headSha == "{sha}" and .event == "push") | .databaseId] | unique | if length == 1 then .[0] else empty end'
    for index in range(40):
        out=run(['gh', 'run', 'list', '-R', REPO_NAME, '--workflow', 'ci.yml',
                 '--event', 'push', '--branch', 'dev', '--limit', '30',
                 '--json', 'databaseId,headSha,event', '--jq', expr])
        if re.fullmatch('[0-9]+', out): return out
        print(f'WAIT_CI: {index+1}/40', flush=True)
        time.sleep(3)
    raise DeliveryError('CI_RUN_NOT_FOUND')

def watch(repo, info):
    print('=== LIVE ===', flush=True)
    sha=info['sha']
    run_id=info.get('run_id') or find_run(sha)
    info['run_id']=run_id
    write_state(repo, info)
    print('EXACT_SHA: '+sha+'; EXACT_RUN_ID: '+run_id, flush=True)
    run(['gh','run','watch',run_id,'-R',REPO_NAME,'--exit-status'], capture=False)
    details=json.loads(run(['gh','run','view',run_id,'-R',REPO_NAME,'--json','headSha,status,conclusion,event']))
    if details.get('headSha') != sha or details.get('event') != 'push' or details.get('status') != 'completed' or details.get('conclusion') != 'success':
        raise DeliveryError('CI_EXACT_SHA_OR_RESULT_FAIL')
    info['phase']='verified'
    write_state(repo, info)
    print('=== VERIFY ===\nCI_EXACT_SHA: PASS\n=== RESULT ===\nSTATE: PASS\nCOMMIT: '+sha+'\nCI_RUN_ID: '+run_id,flush=True)

def ship(repo, message, allow):
    precheck(repo)
    _, paths=gates(repo,allow)
    print('=== ACTION ===', flush=True)
    # Verify the committed identity locally; no global Git identity changes.
    git(repo, 'add', '--', *paths)
    git(repo, 'diff', '--cached', '--check')
    name=git(repo, 'log', '-1', '--format=%an')
    email=git(repo, 'log', '-1', '--format=%ae')
    run(['git','-C',str(repo),'-c','user.name='+name,'-c','user.email='+email,
         'commit','-m',message],capture=False)
    sha=git(repo,'rev-parse','HEAD')
    info={'sha':sha,'phase':'committed','run_id':None}
    write_state(repo,info)
    run(['git','-C',str(repo),'push','origin','HEAD:refs/heads/dev'],capture=False)
    info['phase']='pushed'
    write_state(repo,info)
    watch(repo,info)

def resume(repo):
    precheck(repo)
    path=state_file(repo)
    if not path.exists(): raise DeliveryError('NO_DELIVERY_STATE')
    info=json.loads(path.read_text(encoding='utf-8'))
    sha=info.get('sha','')
    if not re.fullmatch('[a-f0-9]{40}',sha) or git(repo,'rev-parse','HEAD')!=sha:
        raise DeliveryError('STATE_SHA_MISMATCH')
    remote = remote_sha()
    phase = info.get('phase')
    if phase == 'committed' and remote != sha:
        parent = git(repo, 'rev-parse', 'HEAD^')
        if remote != parent:
            raise DeliveryError('REMOTE_DEV_MOVED_BEFORE_RESUME_PUSH: '+remote)
        if changed_paths(repo):
            raise DeliveryError('DIRTY_WORKTREE_BEFORE_RESUME_PUSH')
        print('=== ACTION: RESUME EXACT PUSH ===', flush=True)
        run(['git', '-C', str(repo), 'push', 'origin', 'HEAD:refs/heads/dev'], capture=False)
        remote = remote_sha()
    if remote != sha:
        raise DeliveryError('PUSH_NOT_CONFIRMED_OR_REMOTE_MOVED')
    if phase=='verified':
        print('=== RESULT ===\nSTATE: PASS (ALREADY VERIFIED)\nCOMMIT: '+sha+'\nCI_RUN_ID: '+str(info['run_id']))
        return
    info['phase']='pushed'
    watch(repo,info)

def main():
    p=argparse.ArgumentParser()
    p.add_argument('action', choices=['doctor','check','ship','resume'])
    p.add_argument('--repo',default='.')
    p.add_argument('--message',default='chore(routerforge): apply validated update')
    p.add_argument('--allow',action='append',default=[])
    args=p.parse_args()
    repo=Path(args.repo).resolve()
    try:
        if args.action=='doctor': precheck(repo);print('=== RESULT ===\nSTATE: PASS')
        elif args.action=='check': precheck(repo);gates(repo,args.allow);print('=== RESULT ===\nSTATE: PASS')
        elif args.action=='ship': ship(repo,args.message,args.allow)
        else: resume(repo)
    except (DeliveryError, OSError, ValueError) as e:
        print('=== RESULT ===\nSTATE: FAIL\nREASON: '+str(e),flush=True)
        sys.exit(1)
if __name__=='__main__':main()
