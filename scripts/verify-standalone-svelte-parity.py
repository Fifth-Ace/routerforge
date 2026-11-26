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
from urllib.parse import urlsplit
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
ANTISCAN_MOCK = """(d)=>{window.__api=d;window.__calls=[];window.fetch=async(url,opt={})=>{const base=location.protocol==='about:'?'http://routerforge.mock.invalid/api/modules/antiscan-manager/ui/index.html':location.href;const u=new URL(String(url),base);const key=u.pathname.split('/').pop()+u.search;window.__calls.push({key,method:opt.method||'GET',body:opt.body||''});return new Response(JSON.stringify(structuredClone(window.__api[key]??{})),{status:200,headers:{'content-type':'application/json'}});};window.confirm=()=>false;}"""

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

def fulfill_compiled_route(route, frontend:Path, module:str):
    """Feed real production assets to Chromium without a system HTTP proxy.

    Reject every URL outside this module's build; do not touch live API, router,
    arbitrary local files, or external domains.
    """
    url=urlsplit(route.request.url)
    prefix=f'/api/modules/{module}/ui/'
    if url.scheme not in ('http','https') or url.hostname not in ('127.0.0.1','localhost') or not url.path.startswith(prefix):
        route.abort('blockedbyclient')
        return
    relative=url.path[len(prefix):] or 'index.html'
    base=frontend.resolve()
    file=(base/relative).resolve()
    if not file.is_relative_to(base) or not file.is_file():
        route.abort('blockedbyclient')
        return
    mime={'html':'text/html','js':'text/javascript','css':'text/css','svg':'image/svg+xml','json':'application/json','woff2':'font/woff2'}.get(file.suffix.lstrip('.'),'application/octet-stream')
    route.fulfill(status=200,body=file.read_bytes(),content_type=mime)

def git_read(repo:Path,filename:str):
    p=subprocess.run(['git','show',BASE+':'+filename],cwd=repo,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if p.returncode:raise RuntimeError('ORIGINAL_SOURCE_GIT_MISSING: '+filename+' '+p.stderr.decode()[:250])
    return p.stdout.decode('utf-8')

def pixels(path1:Path,path2:Path)->int:
    a=Image.open(path1).convert('RGB');b=Image.open(path2).convert('RGB')
    if a.size!=b.size: raise AssertionError(f'DIMENSIONS_DIFFER: {a.size} != {b.size}')
    box=ImageChops.difference(a,b)
    if not box.getbbox():return 0
    # Chromium on different hosts may vary by 1-2 RGB values at antialiased edges.
    # Preserve strict geometry and the existing limit of 50 SIGNIFICANT pixels.
    return sum(
        max(abs(before[channel] - after[channel]) for channel in range(3)) > 2
        for before, after in zip(a.getdata(), b.getdata())
    )

def app_body(html:str,module:str)->str:
    klass='nt-page' if module=='network-tools' else 'as-page'
    match=re.search(r'<main\s+class="'+klass+r'"',html)
    if not match:raise ValueError('LEGACY_MAIN_MISSING: '+module)
    return html[match.start():html.index('</main>',match.start())+7]


# E02 intentionally adds a closed catalogue picker to the Doctor and Probes
# panels. Extend the pinned historical reference with independently specified
# markup/CSS for that one control. Do not copy DOM/styles from the Svelte app,
# remove/mask production elements, or change the pixel/API parity thresholds.
def extend_network_reference(page):
    # E14.7: approved closed catalogue control, independent of production CSS.
    page.add_style_tag(content="""
        .rf-policy-picker {
            display:flex; align-items:center; flex-wrap:wrap; gap:8px;
            min-width:0; max-width:100%; margin:0 12px 10px; padding:2px 0;
            color:var(--rf-text,var(--text,#f5f7fa)); font-size:12px;
        }
        .rf-policy-picker > button {
            appearance:none; display:inline-flex; align-items:center; justify-content:center;
            gap:6px; box-sizing:border-box; min-height:34px; max-width:100%;
            padding:7px 12px; border:1px solid var(--rf-accent,var(--accent,#38bdf8));
            border-radius:var(--rf-radius,6px);
            background:var(--rf-accent-soft,var(--accent-soft,rgba(56,189,248,.10)));
            color:var(--rf-accent,var(--accent,#38bdf8)); font:inherit; font-weight:600;
            line-height:1.35; white-space:normal; overflow-wrap:anywhere; cursor:pointer;
        }
    """)
    page.evaluate("""() => {
        for (const selector of ['#panel-doctor .nt-checks', '#panel-probes #probe-summary']) {
            const matches = document.querySelectorAll(selector);
            if (matches.length !== 1) throw Error('E02_REFERENCE_ANCHOR: '+selector);
            const row = document.createElement('div'); row.className = 'rf-policy-picker';
            const button = document.createElement('button'); button.type = 'button';
            button.textContent = 'Из общего каталога'; row.append(button);
            matches[0].before(row);
        }
    }""")


# E10 extends the historical reference with the approved controls. This is
# independently authored static expected HTML, not DOM copied from the app.
# All original screenshots, geometry, API comparisons and tolerances remain.
def extend_network_e10_reference(page):
    page.evaluate("""() => {
        const one = (selector) => {
            const nodes = document.querySelectorAll(selector);
            if (nodes.length !== 1) throw Error('E10_REFERENCE_ANCHOR: '+selector);
            return nodes[0];
        };
        one('#panel-doctor [data-i18n="rttHint"]').textContent = 'Только ICMP RTT';
        const label = document.createElement('label'); label.className='nt-check';
        const input = document.createElement('input'); input.type='checkbox'; input.id='check-tls';
        label.append(input, document.createTextNode(' TLS'));
        one('#panel-doctor .nt-checks').append(label);
        one('#probe-kind').append(new Option('TLS','tls'));
        const heading = one('#panel-probes .probe-history-title');
        heading.textContent='Проверки текущего окна';
        const history = document.createElement('details');
        const summary = document.createElement('summary'); summary.textContent='История Network Tools (до 64)';
        history.append(summary); heading.after(history);
    }""")


def check_network_e10(page, scenario, viewport):
    # Exercise the production bundle only, after unmodified parity evidence.
    # No real network is involved; the original fixture intercepts every fetch.
    if scenario == 'doctor':
        tls = page.locator('#check-tls')
        assert tls.count() == 1 and tls.is_visible() and not tls.is_checked(), 'E10_TLS_OPT_IN'
        page.evaluate("""() => {
            window.__calls=[];
            window.__api['/doctor']={target:'tls-fixture.example',checks:['ping','dns','tcp','tls'],
              probes:{ping:{ok:true,rtt_measured_ms:10,loss_measured_pct:0},dns:{ok:true,addresses:['192.0.2.1']},tcp:{ok:true,duration_ms:90},tls:{ok:false,tcp_ms:15,tls_ms:200,error_class:'timeout'}},
              diagnosis:{verdict:{code:'tls_failure',severity:'fail',fault_domain:'service'},stages:[{id:'tls',status:'fail',detail:'TLS fixture timeout'}],actions:[]}};
        }""")
        page.locator('#doctor-target').fill('tls-fixture.example')
        tls.check()
        page.locator('#doctor-run').click()
        page.locator('#doctor-results').get_by_text('TLS-проверка не прошла',exact=True).wait_for(state='visible')
        assert page.locator('#kpi-rtt').inner_text().strip() == '10 ms', 'E10_ICMP_RTT_MIXED'
        assert page.locator('#kpi-loss').inner_text().strip() == '0.0%', 'E10_MEASURED_ZERO_LOSS'
        calls = page.evaluate('window.__calls.filter(c=>c.path==="/doctor")')
        assert calls == [{'path':'/doctor','query':{'target':'tls-fixture.example','checks':'ping,dns,tcp,tls','tcp_port':'443','http_port':'80'}}], f'E10_DOCTOR_CALL: {calls}'
    else:
        summary = page.locator('#panel-probes details > summary').filter(has_text='История Network Tools (до 64)')
        assert summary.count() == 1 and summary.is_visible(), 'E10_HISTORY_MISSING'
        assert not page.evaluate("window.__calls.some(c=>c.path==='/probe-history')"), 'E10_HISTORY_AUTO_FETCH'
        page.evaluate("""() => {
            window.__calls=[];
            window.__api['/probe']={ok:false,kind:'tls',target:'tls-fixture.example',port:443,tcp_ms:12,tls_ms:80,error_class:'timeout'};
            window.__api['/probe-history']={scope:'network-tools-process',limit:64,rows:[{at:'2026-11-06T09:19:01Z',result:{ok:true,kind:'tls',target:'history-fixture.example',port:443,tcp_ms:7,tls_ms:9}}]};
        }""")
        page.locator('#probe-kind').select_option('tls')
        assert page.locator('#probe-port').is_enabled() and page.locator('#probe-port').input_value() == '443', 'E10_TLS_PORT'
        page.locator('#probe-target').fill('tls-fixture.example')
        page.locator('#probe-run').click()
        page.locator('#probe-summary').get_by_text('TCP 12 ms · TLS 80 ms · timeout',exact=True).wait_for(state='visible')
        assert not page.evaluate("window.__calls.some(c=>c.path==='/probe-history')"), 'E10_HISTORY_IMPLICIT_FETCH'
        summary.click()
        page.get_by_role('button',name='Загрузить историю',exact=True).click()
        page.get_by_text('history-fixture.example',exact=True).wait_for(state='visible')
        calls = page.evaluate('window.__calls.filter(c=>["/probe","/probe-history"].includes(c.path))')
        assert calls == [{'path':'/probe','query':{'kind':'tls','target':'tls-fixture.example','port':'443'}},{'path':'/probe-history','query':{}}], f'E10_PROBE_CALLS: {calls}'
    print(f'E10_NETWORK_BROWSER: PASS / {scenario} / {viewport}',flush=True)


def check_network_picker(page, scenario, viewport):
    # Run after the unmodified production screenshot and original API call
    # capture. The fixture uses reserved names and mocked same-origin GETs.
    picker = page.locator('#panel-'+scenario+' .rf-policy-picker')
    target = page.locator('#'+('doctor' if scenario == 'doctor' else 'probe')+'-target')
    assert picker.count() == 1 and picker.is_visible(), 'E02_PICKER_MISSING'
    assert picker.locator('button').count() == 1, 'E02_PICKER_NOT_CLOSED'
    before = target.input_value()
    assert before != 'catalog-target.example', 'E02_FIXTURE_TARGET_COLLISION'
    page.evaluate("""() => {
        const original = window.fetch;
        window.__catalogCalls = []; window.__catalogStale = true;
        window.__catalogUnexpected = [];
        const etag = 'a'.repeat(64);
        const group = {id:'browser-fixture',name:'Fixture catalogue',type:'domain-group',revision:1};
        window.fetch = async (url, opt={}) => {
            const u = new URL(String(url), location.href);
            if (u.pathname !== '/api/platform/foundation/consumers') {
                if (!['GET','HEAD'].includes(opt.method || 'GET') || /\\/(doctor|probe|traceroute)$/.test(u.pathname)) window.__catalogUnexpected.push(u.pathname);
                return original(url, opt);
            }
            const params = Object.fromEntries(u.searchParams);
            window.__catalogCalls.push({method:opt.method || 'GET',params});
            if (opt.method !== 'GET' || opt.credentials !== 'same-origin' || opt.redirect !== 'error' || params.consumer !== 'network') throw Error('E02_UNSAFE_CATALOGUE_REQUEST');
            const response = (body,status=200) => new Response(JSON.stringify(body),{status,headers:{'content-type':'application/json'}});
            if (Object.keys(params).length === 1) return response({schemaVersion:1,etag,groups:[group]});
            if (Object.keys(params).length !== 3 || params.id !== group.id || params.etag !== etag) throw Error('E02_INVALID_PREVIEW_QUERY');
            if (window.__catalogStale && window.__catalogCalls.length === 3) return response({error:'REVISION_CONFLICT'},409);
            return response({schemaVersion:1,etag,consumer:'network',group,values:['catalog-target.example'],blocked:[]});
        };
    }""")
    def prepare():
        picker.get_by_role('button', name='Из общего каталога', exact=True).click()
        picker.locator('select').first.locator('option[value="browser-fixture"]').wait_for(state='attached')
        picker.locator('select').first.select_option('browser-fixture')
        picker.get_by_role('button', name='Предпросмотр', exact=True).click()
        picker.locator('select').nth(1).locator('option[value="catalog-target.example"]').wait_for(state='attached')
        picker.locator('select').nth(1).select_option('catalog-target.example')
        assert target.input_value() == before, 'E02_PREVIEW_CHANGED_TARGET'
    prepare()
    picker.get_by_role('button',name='Подставить узел',exact=True).click()
    picker.get_by_role('alert').filter(has_text='HTTP 409').wait_for(state='visible')
    assert target.input_value() == before, 'E02_STALE_CHANGED_TARGET'
    picker.get_by_role('button',name='Закрыть',exact=True).click()
    page.evaluate('window.__catalogStale = false')
    prepare()
    picker.get_by_role('button',name='Подставить узел',exact=True).click()
    picker.get_by_role('button',name='Закрыть',exact=True).wait_for(state='detached')
    assert target.input_value() == 'catalog-target.example', 'E02_TARGET_NOT_INSERTED'
    calls = page.evaluate('window.__catalogCalls')
    expected = [{'method':'GET','params':{'consumer':'network'}},
                {'method':'GET','params':{'consumer':'network','id':'browser-fixture','etag':'a'*64}},
                {'method':'GET','params':{'consumer':'network','id':'browser-fixture','etag':'a'*64}}]
    assert calls == expected*2, f'E02_CATALOGUE_CALL_DIFF: {calls}'
    assert page.evaluate('window.__catalogUnexpected') == [], 'E02_AUTOMATIC_PROBE_OR_MUTATION'
    print(f'E02_PICKER_BROWSER: PASS / {scenario} / {viewport} / stale rejected + explicit insert / GET only',flush=True)

def check(module:str,repo:Path,browser,out:Path,viewport:int,scenario:str,url:str,old:dict):
    files={k:git_read(repo,f'modules/{module}/frontend/{k}') for k in ['index.html','app.js','module.css']}
    if module=='network-tools':files['runtime.js']=git_read(repo,'modules/network-tools/frontend/runtime.js')
    shots={};calls={};contents={};layouts={}
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
            page.route('**/*',lambda route:fulfill_compiled_route(route,repo/'modules'/module/'frontend/build',module))
            page.goto(url,wait_until='load')
        page.wait_for_timeout(450)
        if module=='network-tools' and scenario!='doctor':
            page.locator('#tab-'+scenario).click()
        if module=='antiscan-manager' and scenario!='overview':
            page.locator('[data-tab="'+scenario+'"]:visible').first.click()
        page.wait_for_timeout(160)
        if module=='network-tools' and which=='legacy':
            extend_network_reference(page)
        if module=='network-tools' and which=='legacy':
            extend_network_e10_reference(page)
        if errors:raise AssertionError(f'JAVASCRIPT_ERRORS: {module} {which} {scenario} {errors[:2]}')
        if module=='antiscan-manager':
            contents[which]=page.locator('#configState').inner_text()
        # One intentional i18n improvement since the pinned DEV baseline:
        # legacy hardcodes English in the idle Probes summary; Svelte 5
        # correctly renders Russian on an RU page. Check exact translations
        # first, save the unmodified visual evidence, then normalize only
        # this ONE text node for pixel-geometry parity. Any other drift still
        # fails at the unchanged 50-pixel/2-RGB limits.
        if module=='network-tools' and scenario=='probes':
            empty=page.locator('#probe-summary .nt-empty')
            if empty.count()!=1:
                raise AssertionError('PROBES_READY_TEXT_MISSING: '+which)
            actual=empty.inner_text().strip()
            expected='Ready to run.' if which=='legacy' else 'Готово к запуску.'
            if actual!=expected:
                raise AssertionError(f'PROBES_READY_LOCALIZATION_REGRESSION: {which} {actual!r}')
            raw=out/f'{module}-{scenario}-{viewport}-{which}-unmodified.png'
            page.screenshot(path=str(raw),full_page=True,animations='disabled',caret='hide')
            if which=='svelte':
                empty.evaluate("node => { if(node.textContent.trim() !== 'Готово к запуску.') throw Error('PROBES_I18N_CHANGED'); node.textContent='Ready to run.'; }")
        shots[which]=out/f'{module}-{scenario}-{viewport}-{which}.png'
        page.evaluate("() => document.fonts.ready")
        page.screenshot(path=str(shots[which]),full_page=True,animations='disabled',caret='hide')
        layouts[which]=page.evaluate("""() => {
          const box=(e)=>{let r=e.getBoundingClientRect(),c=getComputedStyle(e);return {
            tag:e.tagName.toLowerCase(),id:e.id||'',classes:String(e.className||''),
            section:e.getAttribute('data-page')||'',hidden:e.hidden,
            display:c.display,visibility:c.visibility,
            y:Math.round(r.y),height:Math.round(r.height),bottom:Math.round(r.bottom),
            scrollHeight:e.scrollHeight};};
          return {rootSection:document.documentElement.dataset.section||'',
            documentHeight:document.documentElement.scrollHeight,bodyHeight:document.body.scrollHeight,
            allMainChildren:[...document.querySelectorAll('.as-page > *')].map(box),
            allTabPages:[...document.querySelectorAll('.tab-page')].map(box),
            overviewItems:[...document.querySelectorAll('.tab-page[data-page="overview"] > *')].map(box),
            visibleNotices:[...document.querySelectorAll('.notice')].map(box)};
        }""")
        calls[which]=page.evaluate('window.__calls')
        if module=='network-tools' and which=='svelte' and scenario in ('doctor','probes'):
            check_network_picker(page, scenario, viewport)
            check_network_e10(page, scenario, viewport)
            if errors:raise AssertionError(f'E02_PICKER_JAVASCRIPT_ERRORS: {errors[:2]}')
        page.close()
    if Image.open(shots['legacy']).size != Image.open(shots['svelte']).size:
        data={'module':module,'scenario':scenario,'viewport':viewport,'layouts':layouts,
              'shots':{key:str(value) for key,value in shots.items()}}
        path=out/f'{module}-{scenario}-{viewport}-layout-diff.json'
        path.write_text(json.dumps(data,indent=2,ensure_ascii=False),encoding='utf-8')
        print('PARITY_DIMENSIONS_DIAGNOSTIC: '+str(path),flush=True)
        for name,entry in layouts.items():
            print('PARITY_LAYOUT_'+name.upper()+': '+json.dumps(entry,ensure_ascii=False),flush=True)
    diff=pixels(shots['legacy'],shots['svelte'])
    if calls['legacy']!=calls['svelte']:
        raise AssertionError(f'API_CALL_DIFF: {module} {scenario} {calls}')
    if contents.get('legacy')!=contents.get('svelte'):
        raise AssertionError(f'RENDER_STATE_DIFF: {module} {scenario} {contents}')
    if diff>50:
        a=Image.open(shots['legacy']).convert('RGB')
        b=Image.open(shots['svelte']).convert('RGB')
        delta=ImageChops.difference(a,b)
        region=delta.getbbox()
        path=out/f'{module}-{scenario}-{viewport}-diff.png'
        delta.save(path)
        # Record exact evidence; never relax pixel threshold or mask mismatching elements.
        bucket={}
        width=a.width
        for i,(p,q) in enumerate(zip(a.getdata(),b.getdata())):
            if max(abs(before-after) for before,after in zip(p,q))>2:
                x,y=i%width,i//width
                tile=(x//80*80,y//40*40)
                bucket[tile]=bucket.get(tile,0)+1
        top=sorted(bucket.items(),key=lambda kv:-kv[1])[:12]
        report={'module':module,'scenario':scenario,'viewport':viewport,
                'changed_pixels':diff,'threshold':50,'rgb_noise_tolerance':2,'bbox':region,
                'hotspots':[{'x':x,'y':y,'changed_pixels':n} for (x,y),n in top],
                'shots':{key:str(value) for key,value in shots.items()},
                'diff':str(path),'layouts':layouts}
        report_path=out/f'{module}-{scenario}-{viewport}-pixel-diff.json'
        report_path.write_text(json.dumps(report,indent=2,ensure_ascii=False),encoding='utf-8')
        print('PIXEL_DRIFT_EVIDENCE: '+str(report_path),flush=True)
        print('PIXEL_DRIFT_BBOX: '+str(region),flush=True)
        print('PIXEL_DRIFT_HOTSPOTS: '+json.dumps(report['hotspots']),flush=True)
        raise AssertionError(f'PIXEL_DRIFT: {module} {scenario} {viewport}: {diff} pixels')
    print(f'BROWSER_{module}_{scenario}_{viewport}: PASS / significant_pixels={diff} / rgb_tolerance=2/255 / API_EQUAL')

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
            browser=p.chromium.launch(headless=True,executable_path=args.chromium or None,args=['--no-sandbox','--disable-extensions','--no-proxy-server'])
            failures=[]
            try:
                for scenario in scenarios:
                    for viewport in [1280,375]:
                        try:
                            check(args.module,args.repo,browser,args.out,viewport,scenario,url,api)
                        except AssertionError as error:
                            failures.append((scenario,viewport,str(error)))
                            print(f'PARITY_SCENARIO_FAIL / {args.module} / {scenario} / {viewport}: {error}',flush=True)
            finally:browser.close()
    if failures:
        print('PARITY_FULL_MATRIX: FAIL / '+str(len(failures))+' scenarios',flush=True)
        for scenario,viewport,error in failures:
            print(f'PARITY_FAILURE_DETAIL / {args.module} / {scenario} / {viewport}: {error}',flush=True)
        raise AssertionError('PARITY_SCENARIOS_FAILED: '+str(len(failures)))
    print('BROWSER_STANDALONE_SVELTE5_ALL_SCENARIOS: PASS')
if __name__=='__main__':main()
