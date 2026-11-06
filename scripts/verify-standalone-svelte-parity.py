#!/usr/bin/env python3
"""Real Chromium parity: shipped Vite bundles vs exact original Git DEV baseline.

Run in CI after production builds. Uses only mocked API responses, never contacts
Keenetic, runs mutations, or reads live environment. Keeps browser offline except
for an ephemeral 127.0.0.1 HTTP server for the actual built frontend assets.
"""
from __future__ import annotations
import argparse
from contextlib import contextmanager
import json
import os
from pathlib import Path
import re
import subprocess
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from PIL import Image, ImageChops
from playwright.sync_api import sync_playwright

BASE = '280040812dcd97940bfe89de2ba8f6fae3afbd53'

NETWORK_API = {
'/health': {}, '/summary': {'active_sessions':5},
'/interfaces': {'interfaces':[{'name':'br0','operstate':'up','flags':'UP','speed_mbps':1000,'duplex':'full','mtu':1500,'addresses':['192.168.10.1/24'],'rx_bytes':123000,'tx_bytes':452000,'default_route':True,'default_gateway':'192.168.10.254','default_metric':10}]},
'/routes': {'target':'8.8.8.8','family':'ipv4','capabilities':{'ip_command':True,'ipv4_all_tables':True,'ipv6_all_tables':True,'ipv4_rules':True,'ipv6_rules':False,'route_get':True},'kernel_decision':{'available':True,'destination':'8.8.8.8','family':'ipv4','gateway':'192.168.10.254','interface':'br0','source':'192.168.10.1','table':'main','metric':0,'type':'unicast'},'routes_v4':[{'family':'ipv4','destination':'0.0.0.0/0','table':'main','gateway':'192.168.10.254','interface':'br0','source':'192.168.10.1','metric':10,'protocol':'static','scope':'global','type':'unicast'}], 'routes_v6':[], 'rules_v4':[], 'rules_v6':[], 'source':'ip-route','decision_scope':'router-originated','policy_state':'active','resolution':{'addresses':['8.8.8.8']}},
'/flow-explorer': {'source':'conntrack','generated_at':'2026-11-06T09:19:01Z','enrichment':{'socket_records':7,'owner_fds_scanned':40,'route_lookups':1,'route_lookup_budget':32},'flows':[{'protocol':'TCP','state':'ESTABLISHED','original':{'source':'192.168.10.120','source_port':'49800','destination':'8.8.8.8','destination_port':'443','packets':8,'bytes':2048},'reply':{'source':'8.8.8.8','source_port':'443','destination':'192.168.10.120','destination_port':'49800','packets':10,'bytes':4050},'timeout_seconds':55,'mark':'0x1','nat':{'detected':False,'types':[]},'policy':{'source':'routing','kernel_decision':{'available':True,'interface':'br0','table':'main'},'candidates':[]},'socket':{'available':True,'process':'curl','pid':1234}}]},
'/doctor': {'target':'example.com','checks':['ping','dns','tcp'],'probes':{'ping':{'ok':True,'rtt_avg_ms':10,'loss_pct':0},'dns':{'ok':True,'addresses':['1.1.1.1']},'tcp':{'ok':True,'duration_ms':20}},'diagnosis':{'verdict':{'code':'healthy','severity':'ok'},'stages':[{'id':'dns','status':'ok','detail':'resolution is working','duration_ms':12}], 'actions':[]}},
'/traceroute': {'hops':[{'hop':1,'address':'192.168.10.254','rtt_ms':[0.4,0.5,0.7],'loss_pct':0,'label':'gateway'}]},
'/probe': {'ok':True,'kind':'tcp','target':'8.8.8.8','duration_ms':15},
}
ANTISCAN_STATUS={'detected':True,'running':True,'version':'2.0.0','ipsets':[{'name':'ascn_ips','exists':True,'count_known':True,'count':12},{'name':'ascn_subnets','exists':True,'count_known':True,'count':2},{'name':'ascn_candidates','exists':True,'count_known':True,'count':3}], 'config_sha256':'abcdef123456', 'config':{'isp_interfaces':['eth2'],'ports':['80','443'],'forwarded_ports':['2222'],'enable_honeypot':False,'honeypot_ports':[],'honeypot_bantime_seconds':0,'enable_ips_ban':True,'rules_mask':'255.255.255.255','recent_connections_time_seconds':60,'recent_connections_hitcount':6,'recent_connections_limit':50,'recent_connections_bantime_seconds':600,'different_ip_candidates_storage_seconds':86400,'different_ip_threshold':5,'subnets_bantime_seconds':600,'ipsets_directory':'/opt/var/lib/antiscan','save_ipsets':True,'save_on_exit':True,'use_custom_exclude_list':False,'custom_lists_block_mode':'0','geoblock_mode':'0','geoblock_countries':[],'geo_exclude_countries':[],'read_ndm_lockout_ipsets':False,'lockout_ipset_bantime_seconds':0}}
ANTISCAN_API={'status':ANTISCAN_STATUS, 'schedule':{'task_count':2,'active_count':1,'valid':True,'synced':True,'source_sha256':'abc123','tasks':[{'task':'read_candidates','schedule':'*/10 * * * *','enabled':True,'automatic':False},{'task':'retry_load_geo','schedule':'0 * * * *','enabled':True,'automatic':True}]}, 'history?limit=50':{'events':[]}}

FIXED_DATE = """() => { const NativeDate=Date; Date=class extends NativeDate { constructor(...a){super(...(a.length?a:['2026-11-06T09:19:01.000Z']));} static now(){return new NativeDate('2026-11-06T09:19:01.000Z').getTime();} }; }"""
NETWORK_MOCK = """(d)=>{window.__api=d;window.__calls=[];window.fetch=async(url,opt={})=>{let u=new URL(String(url),location.href);let key=u.pathname.split('/').pop();const data=window.__api['/'+key]??{};window.__calls.push({path:'/'+key,query:Object.fromEntries([...u.searchParams.entries()].sort())});return new Response(JSON.stringify(structuredClone(data)),{status:200,headers:{'content-type':'application/json'}});};}"""
ANTISCAN_MOCK = """(d)=>{window.__api=d;window.__calls=[];window.fetch=async(url,opt={})=>{const u=new URL(String(url),location.href);const key=u.pathname.split('/').pop()+u.search;window.__calls.push({key,method:opt.method||'GET',body:opt.body||''});return new Response(JSON.stringify(structuredClone(window.__api[key]??{})),{status:200,headers:{'content-type':'application/json'}});};window.confirm=()=>false;}"""

class StaticHandler(BaseHTTPRequestHandler):
    front: Path
    module: str
    def log_message(self, *_): pass
    def do_GET(self):
        prefix=f'/api/modules/{self.module}/ui/'
        if not self.path.startswith(prefix):self.send_error(404);return
        relative=self.path[len(prefix):].split('?',1)[0].split('#',1)[0]
        if not relative:relative='index.html'
        file=(self.front/relative).resolve()
        if not file.is_file() or not file.is_relative_to(self.front.resolve()):self.send_error(404);return
        raw=file.read_bytes()
        self.send_response(200)
        self.send_header('Content-Type',('text/javascript' if file.suffix=='.js' else 'text/css' if file.suffix=='.css' else 'text/html')+'; charset=utf-8')
        self.send_header('Content-Length',str(len(raw)))
        self.end_headers();self.wfile.write(raw)

@contextmanager
def server(front:Path,module:str):
    handler=type('FrontendHandler',(StaticHandler,),{'front':front,'module':module})
    http=ThreadingHTTPServer(('127.0.0.1',0),handler)
    thread=threading.Thread(target=http.serve_forever,daemon=True);thread.start()
    try: yield f'http://127.0.0.1:{http.server_port}/api/modules/{module}/ui/index.html'
    finally:http.shutdown();http.server_close();thread.join(timeout=5)

def git_read(repo:Path,filename:str):
    p=subprocess.run(['git','show',BASE+':'+filename],cwd=repo,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if p.returncode:raise RuntimeError('ORIGINAL_SOURCE_GIT_MISSING: '+filename+' '+p.stderr.decode()[:250])
    return p.stdout.decode('utf-8')

def pixels(path1:Path,path2:Path)->int:
    a=Image.open(path1).convert('RGB');b=Image.open(path2).convert('RGB')
    if a.size!=b.size: raise AssertionError(f'DIMENSIONS_DIFFER: {a.size} != {b.size}')
    box=ImageChops.difference(a,b)
    if not box.getbbox():return 0
    return sum(a_px!=b_px for a_px,b_px in zip(a.getdata(),b.getdata()))

def app_body(html:str,module:str)->str:
    klass='nt-page' if module=='network-tools' else 'as-page'
    match=re.search(r'<main\s+class="'+klass+r'"',html)
    if not match:raise ValueError('LEGACY_MAIN_MISSING: '+module)
    return html[match.start():html.index('</main>',match.start())+7]

def check(module:str,repo:Path,browser,out:Path,viewport:int,scenario:str,url:str,old:dict):
    files={k:git_read(repo,f'modules/{module}/frontend/{k}') for k in ['index.html','app.js','module.css']}
    if module=='network-tools':files['runtime.js']=git_read(repo,'modules/network-tools/frontend/runtime.js')
    shots={};calls={};contents={}
    for which in ['legacy','svelte']:
        page=browser.new_page(viewport={'width':viewport,'height':900},device_scale_factor=1)
        errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
        mock=NETWORK_MOCK if module=='network-tools' else ANTISCAN_MOCK
        if which=='legacy':
            page.goto('about:blank')
            page.evaluate(FIXED_DATE)
            page.set_content('<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>'+files['module.css']+'</style></head><body><div id="app" style="display:contents"></div></body></html>')
            page.evaluate(mock,old)
            page.evaluate('(b)=>document.body.innerHTML=b',app_body(files['index.html'],module))
            if module=='network-tools':
                page.add_script_tag(content=files['runtime.js'])
                page.evaluate("() => { window.RFNet.request=async(path,query)=>{window.__calls.push({path,query:Object.fromEntries(Object.entries(query||{}).filter(([k,v])=>v!==''&&v!=null).map(([k,v])=>[k,String(v)]).sort())});if(window.__api[path]===undefined)throw Error('UNMOCKED_PATH '+path);return structuredClone(window.__api[path]);}; }")
            page.add_script_tag(content=files['app.js'])
        else:
            page.add_init_script(script='('+FIXED_DATE+')()')
            page.add_init_script(script='('+mock+')('+json.dumps(old, ensure_ascii=False)+')')
            page.goto(url,wait_until='load')
        page.wait_for_timeout(450)
        if module=='network-tools' and scenario!='doctor':
            page.locator('#tab-'+scenario).click()
        if module=='antiscan-manager' and scenario!='overview':
            page.locator('[data-tab="'+scenario+'"]:visible').first.click()
        page.wait_for_timeout(160)
        if errors:raise AssertionError(f'JAVASCRIPT_ERRORS: {module} {which} {scenario} {errors[:2]}')
        if module=='antiscan-manager':
            contents[which]=page.locator('#configState').inner_text()
        shots[which]=out/f'{module}-{scenario}-{viewport}-{which}.png'
        page.screenshot(path=str(shots[which]),full_page=True)
        calls[which]=page.evaluate('window.__calls')
        page.close()
    diff=pixels(shots['legacy'],shots['svelte'])
    if calls['legacy']!=calls['svelte']:
        raise AssertionError(f'API_CALL_DIFF: {module} {scenario} {calls}')
    if contents.get('legacy')!=contents.get('svelte'):
        raise AssertionError(f'RENDER_STATE_DIFF: {module} {scenario} {contents}')
    if diff>50:raise AssertionError(f'PIXEL_DRIFT: {module} {scenario} {viewport}: {diff} pixels')
    print(f'BROWSER_{module}_{scenario}_{viewport}: PASS / pixels={diff} / API_EQUAL')

def main():
    p=argparse.ArgumentParser()
    p.add_argument('--repo',type=Path,required=True)
    p.add_argument('--module',choices=['network-tools','antiscan-manager'],required=True)
    p.add_argument('--out',type=Path,required=True)
    p.add_argument('--chromium',default='')
    args=p.parse_args()
    args.out.mkdir(parents=True,exist_ok=True)
    frontend=args.repo/'modules'/args.module/'frontend/build'
    if not (frontend/'index.html').is_file():raise ValueError('PRODUCTION_FRONTEND_MISSING')
    api=NETWORK_API if args.module=='network-tools' else ANTISCAN_API
    scenarios=['doctor','routes','flows','probes'] if args.module=='network-tools' else ['overview','diagnostics','blocked','lists','geo','schedule','history','config']
    with server(frontend,args.module) as url:
        with sync_playwright() as p:
            browser=p.chromium.launch(headless=True,executable_path=args.chromium or None,args=['--no-sandbox'])
            try:
                for scenario in scenarios:
                    for viewport in [1280,375]:check(args.module,args.repo,browser,args.out,viewport,scenario,url,api)
            finally:browser.close()
    print('BROWSER_STANDALONE_SVELTE5_ALL_SCENARIOS: PASS')
if __name__=='__main__':main()
