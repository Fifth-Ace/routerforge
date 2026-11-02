'use strict';
const test=require('node:test'),a=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {JSDOM,VirtualConsole}=require('jsdom');
const root=path.resolve(__dirname,'../frontend');
const html=fs.readFileSync(path.join(root,'index.html'),'utf8');
const js=fs.readFileSync(path.join(root,'app.js'),'utf8');
const css=fs.readFileSync(path.join(root,'module.css'),'utf8');
const tick=()=>new Promise(r=>setTimeout(r,25));
const until=async(p)=>{for(let i=0;i<45;i++){if(p())return;await tick();}throw Error('UI wait exceeded');};
function fixture(){
  let config={schema_version:1,upstreams:[{id:'old',transport:'udp',address:'9.9.9.9:53'},{id:'cf',transport:'doh',address:'1.1.1.1:443',server_name:'cloudflare-dns.com',path:'/dns-query'}],rules:[{id:'r1',domain:'example.com',match:'suffix',upstream:'old',priority:100},{id:'r2',domain:'other.net',match:'suffix',upstream:'cf',priority:100}]};
  let revision='sha256:initial',active=false,gateEnabled=false,gateOverride=null,connection={state:'disconnected'},requests=0,denied=0;const calls=[],errors=[];
  const vc=new VirtualConsole();vc.on('jsdomError',e=>errors.push(e.message));
  const dom=new JSDOM(html,{url:'https://routerforge.test/ui/index.html',runScripts:'outside-only',pretendToBeVisual:true,virtualConsole:vc});
  const w=dom.window;w.confirm=()=>true;w.scrollTo=()=>{};
  w.fetch=async(url,opts={})=>{
    const p=new URL(url,w.location.href).pathname.replace(/^\/?ui\/\.\.\//,'').replace(/^\//,'');calls.push({path:p,opts});
    let status=200,body={};
    if(p==='config'){
      if(opts.method==='PUT'){a.equal(opts.headers['If-Match'],'"'+revision+'"');config=JSON.parse(opts.body);revision='sha256:next';}
      body={config:structuredClone(config),revision};
    }else if(p==='service/status')body={mode:'independent-dns',active,applied_revision:active?revision:'',listen:config.service?.listen,default_upstream:config.service?.default_upstream,requests,denied_requests:denied,cache_hits:0,failures:0,connection,system_gate:gateOverride||{state:gateEnabled?'ACTIVE':'STOCK',active:gateEnabled,desired:gateEnabled,lease_seconds:60}};
    else if(p==='service/traces')body={traces:active?[{name:'example.com',time:'2026-10-31T12:00:00Z',assigned:'old',used:'old',transport:'udp',rcode:0,success:true,latency_ms:5,type:1,cache:false}]:[]};
    else if(p==='certificates/lookup'){a.equal(opts.headers['X-RouterForge-Action'],'certificate-lookup');const q=JSON.parse(opts.body);body={address:'8.8.8.8:443',selected_ip:'8.8.8.8',ips:['8.8.8.8'],certificate:{verified:true,pem:'-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n',subject:'CN=dns.google',issuer:'Trusted CA',not_after:'2027-10-31',fingerprint_sha256:'ab',server_name:q.server_name}};}
    else if(p==='service/control'){a.equal(opts.headers['X-RouterForge-Action'],'service-control');const q=JSON.parse(opts.body);active=q.operation!=='stop';body={ok:true};}
    else if(p==='service/gate/control'){const q=JSON.parse(opts.body);if(q.operation==='enable')gateEnabled=true;else if(q.operation==='disable'){gateEnabled=false;gateOverride=null;}else if(q.operation==='retry'){gateEnabled=true;gateOverride=null;}body={ok:true};}
    else if(p==='service/upstreams/inspect'){const q=JSON.parse(opts.body);a.equal(q.operation,'inspect');body={status:'UP',dns_available:true,address:q.upstream.address,transport:q.upstream.transport,probe_latency_ms:24,tls_status:'VERIFIED_BY_TRANSPORT',saved:false,applied:false};}
    else if(p==='service/upstreams/health')body={source:'routerforge-direct',upstreams:[{id:'old',status:'UP',probe_latency_ms:14,last_checked:'2026-11-01T12:00:00Z',requests_5m:9,failures_5m:0,p95_latency_5m_ms:22},{id:'cf',status:'DOWN',probe_latency_ms:3000,last_checked:'2026-11-01T12:00:00Z',requests_5m:2,failures_5m:2,last_error:'timeout'}]};
    else if(p==='service/upstreams/check')body={source:'routerforge-direct',upstreams:[{id:'old',status:'UP',probe_latency_ms:14,last_checked:'2026-11-01T12:00:00Z',requests_5m:9,failures_5m:0,p95_latency_5m_ms:22},{id:'cf',status:'DOWN',probe_latency_ms:3000,last_checked:'2026-11-01T12:00:00Z',requests_5m:2,failures_5m:2,last_error:'timeout'}]};
    else if(p==='service/verify')body={state:'PASS',results:[{state:'PASS'}]};
    else if(p==='service/connect'){connection={state:'connected',mac:JSON.parse(opts.body).mac};body={ok:true};}
    else if(p==='service/disconnect'){connection={state:'disconnected'};body={ok:true};}
    else if(p==='service/migrate-native')body={ok:true};
    else status=404;
    return {ok:status===200,status,headers:{get:()=> 'application/json'},json:async()=>structuredClone(body)};
  };
  w.eval(js);return {w,dom,calls,errors,getConfig:()=>config,getActive:()=>active,setConnection:v=>{connection=v;},setRequests:v=>{requests=v;},setDenied:v=>{denied=v;},setGate:v=>{gateOverride=v;}};
}

test('P13 visual parity: two pages and split DNS/domain workspace, no P10 controls',()=>{
  for(const id of ['pageDns','pageLogs','navDns','navLogs','quickHelp','inspectDNS','certificateInfo','editorDomains','logRows','clientConnection','defaultDNS','dnsCards','editorEmpty','selectedDNSBadge','logUpdated'])a.equal((html.match(new RegExp('id="'+id+'"','g'))||[]).length,1,id);
  for(const id of ['drAdvancedWorkspace','drModeAdvanced','managedTrialN','shadowStartButton','drSimplePolicies','drP92Apply'])a.doesNotMatch(html,new RegExp('id="'+id+'"'));
  a.match(css,/--rf-accent/);a.match(css,/routing-workspace/);a.match(css,/dns-card\.is-selected/);a.match(js,/service\/traces/);a.doesNotMatch(js,/keenetic\/durable\/apply|shadow\/control/);
});

test('P13 split editor: DNS -> assigned domains, verified TLS, no unrequested activation',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    a.equal(d.querySelectorAll('.dns-card').length,2);
    a.equal(d.getElementById('defaultDNS').value,'');
    d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
    a.equal(d.getElementById('editorEmpty').hidden,true);
    a.equal(d.getElementById('listEditor').hidden,false);
    a.ok(d.querySelector('[data-upstream-id="cf"]').classList.contains('is-selected'));
    a.equal(d.getElementById('editorDomains').value,'other.net');
    d.getElementById('editorDomains').value='other.net\nyoutube.com';
    d.getElementById('saveDomains').click();
    await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён'));
    a.equal(d.querySelector('[data-upstream-id="cf"] .dns-preview').textContent.includes('youtube.com'),true);
    a.equal(d.getElementById('listEditor').hidden,false,'editor stays visible after saving draft');
    a.equal(d.getElementById('editorCount').textContent,'2 доменов');
    a.equal(d.querySelector('[data-upstream-id="old"] .dns-preview').textContent.includes('youtube.com'),false);
    d.getElementById('addDnsToggle').click();d.getElementById('newAddress').value='https://dns.google/dns-query';
    d.getElementById('inspectDNS').click();await until(()=>/TLS проверен/.test(d.getElementById('certificateInfo').textContent)&&!d.getElementById('commitDNS').disabled&&!d.getElementById('inspectDNS').disabled);
    a.match(d.getElementById('certificateInfo').textContent,/TLS проверен/);
    d.getElementById('commitDNS').click();await until(()=>d.querySelectorAll('.dns-card').length===3);
    d.getElementById('defaultDNS').value='cf';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
    a.equal(f.getActive(),false,'adding and editing does NOT activate DNS');
    d.getElementById('applyButton').click();await until(()=>f.getActive());
    a.equal(f.getConfig().service.default_upstream,'cf');
    a.equal(f.getConfig().rules.length,3);
    a.ok(f.getConfig().upstreams.find(u=>u.server_name==='dns.google').certificate_pem);
    a.equal(f.calls.some(c=>/keenetic\/durable|shadow\/control/.test(c.path)),false);
    a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});

test('P13 live log shows actual assigned and used upstream, cache and status, with filter',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
    d.getElementById('applyButton').click();await until(()=>f.getActive());
    d.getElementById('navLogs').click();
    await until(()=>d.querySelectorAll('.log-row').length>0);
    a.match(d.getElementById('logRows').textContent,/example\.com/);
    a.match(d.getElementById('logRows').textContent,/old/);
    d.getElementById('logFilter').value='not-existing';d.getElementById('logFilter').dispatchEvent(new f.w.Event('input'));
    a.equal(d.querySelectorAll('.log-row').length,0);
    a.equal(d.getElementById('pageDns').hidden,true);
    a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});

test('P13 legacy native entries are not silently activated or dropped',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    a.equal(f.calls.filter(c=>/migrate-native/.test(c.path)).length,0);
    a.equal(f.calls.some(c=>c.opts.method==='POST'),false);
  }finally{f.dom.window.close();}
});


test('P13 prevents accidental loss of unsaved domain text on apply or switching DNS',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.querySelectorAll('.dns-card').length===2);
    d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
    d.getElementById('editorDomains').value='other.net\nunsaved.example';
    d.getElementById('editorDomains').dispatchEvent(new f.w.Event('input'));
    a.match(d.getElementById('editorCount').textContent,/2 строк/);
    d.querySelector('[data-upstream-id="old"] .dns-edit').click();
    a.equal(d.getElementById('editorDomains').value,'example.com','confirm permits explicit discard when switching DNS');
    d.getElementById('editorDomains').value='example.com\nimportant.net';
    d.getElementById('defaultDNS').value='old';
    d.getElementById('applyButton').click();
    await until(()=>d.getElementById('feedback').textContent.includes('Сначала нажми'));
    a.equal(f.getActive(),false,'no DNS activation while unsaved domains exist');
    a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});


test('P14: status distinguishes an active listener from a verified client path',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    a.match(d.getElementById('routeAdvice').textContent,/DNS Router остановлен/);
    d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
    d.getElementById('applyButton').click();await until(()=>f.getActive() && d.getElementById('routeAdvice').textContent.includes('0 запросов'));
    await tick();
    a.match(d.getElementById('routeAdvice').textContent,/0 запросов/);
    a.match(d.getElementById('routeAdvice').textContent,/53/);
    f.setConnection({state:'pending',mac:'be:56:a6:9a:70:ee'});
    d.getElementById('refreshLogs').click();
    await until(()=>d.getElementById('routeAdvice').textContent.includes('pending'));
    a.match(d.getElementById('logGuide').textContent,/не завершено/);
    f.setConnection({state:'connected',mac:'be:56:a6:9a:70:ee'});f.setRequests(4);
    d.getElementById('refreshLogs').click();
    await until(()=>d.getElementById('routeAdvice').textContent.includes('4 запросов'));
    a.doesNotMatch(d.getElementById('routeAdvice').textContent,/0 запросов/);
    f.setRequests(4);f.setDenied(4);d.getElementById('refreshLogs').click();
    await until(()=>d.getElementById('routeAdvice').textContent.includes('ВСЕ отклонены'));
    a.match(d.getElementById('routeAdvice').textContent,/REFUSED/);
    a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});

test('P15 Gate remains OFF by default, requires explicit activation and returns to STOCK',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    a.equal(d.getElementById('gateBadge').textContent,'Выключен');
    a.equal(d.getElementById('gateEnable').disabled,true,'Gate cannot enable before service start');
    a.equal(f.calls.filter(c=>c.path==='service/gate/control').length,0,'no implicit Gate requests');
    d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
    d.getElementById('applyButton').click();await until(()=>f.getActive());
    a.equal(d.getElementById('gateEnable').disabled,false,'Gate becomes available after apply');
    d.getElementById('gatePreflight').click();
    await until(()=>f.calls.some(c=>c.path==='service/gate/control'&&JSON.parse(c.opts.body).operation==='preflight'));
    await until(()=>!d.getElementById('gateEnable').disabled);
    d.getElementById('gateEnable').click();
    await until(()=>d.getElementById('gateBadge').textContent==='Активен');
    a.match(d.getElementById('routeAdvice').textContent,/Основной DNS для LAN активен/);
    a.doesNotMatch(d.getElementById('routeAdvice').textContent,/клиентский профиль не подключён/);
    d.getElementById('gateDisable').click();
    await until(()=>d.getElementById('gateBadge').textContent==='Выключен');
    const gateCalls=f.calls.filter(c=>c.path==='service/gate/control').map(c=>JSON.parse(c.opts.body).operation);
    a.deepEqual(gateCalls,['preflight','enable','disable']);a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});

test('P16 LAN-first layout uses the shared Gate panel and keeps legacy device setup optional',async()=>{
  const f=fixture(),d=f.w.document;
  try{
    await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
    const gate=d.getElementById('systemGatePanel');
    a.ok(gate.contains(d.getElementById('defaultDNS')),'default upstream belongs to the LAN-first panel');
    for(const id of ['applyButton','gatePreflight','gateEnable','gateDisable','runStatus','routeAdvice'])a.ok(gate.contains(d.getElementById(id)),id+' belongs to the primary workflow');
    a.equal(d.querySelector('.apply-panel'),null,'obsolete Step 3 panel removed');
    a.doesNotMatch(html,/ШАГ 3|Запуск и подключение/);
    a.equal(d.getElementById('clientConnection').open,false,'legacy test-device section is collapsed');
    a.equal(d.getElementById('additionalSettings').open,false,'advanced settings are collapsed');
    a.equal(d.getElementById('serviceTools').open,false,'service maintenance actions are secondary');
    a.match(d.getElementById('gateAdvice').textContent,/Тестовое устройство не требуется/);
    a.match(d.getElementById('quickHelp').textContent,/отдельное устройство подключать не нужно/);
    a.ok(d.querySelector('.routing-workspace'),'two-column DNS/domain workspace retained');
    a.ok(d.querySelector('.dns-card'),'DNS cards render in the left pane');
    a.match(css,/--rf-ui-control-radius/);
    a.match(css,/grid-template-columns:minmax\(300px,420px\) minmax\(0,1fr\)/);
    a.match(css,/@media\(max-width:980px\)/);
    a.doesNotMatch(css,/\.gate-panel\{border-color:var\(--accent\)\}/,'no decorative accent frame around the Gate');
    a.deepEqual(f.calls.filter(c=>/service\/connect|service\/disconnect/.test(c.path)),[],'LAN workflow does not require a test-device profile');
    a.deepEqual(f.errors,[]);
  }finally{f.dom.window.close();}
});

test('P18: DNS cards show direct RTT separately from observed p95, read-only overview and manual checks',async()=>{
 const f=fixture(),d=f.w.document;
 try{
   await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
   await until(()=>d.getElementById('healthRequests').textContent==='11');
   a.equal(d.querySelector('[data-upstream-id="old"] .dns-rtt').textContent,'DNS RTT: 14 мс');
   a.equal(d.querySelector('[data-upstream-id="old"] .dns-p95').textContent,'p95: 22 мс');
   a.match(d.querySelector('[data-upstream-id="cf"] .dns-health-status').textContent,/Недоступен/);
   a.equal(d.getElementById('healthServers').textContent,'1/2');
   a.match(d.getElementById('nativeDNSHealth').textContent,/недоступен/);
   a.equal(f.calls.filter(x=>x.path==='service/upstreams/check').length,0,'health GET must not trigger a probe');
   d.querySelector('[data-upstream-id="old"] .dns-check').click();
   await until(()=>f.calls.some(x=>x.path==='service/upstreams/check')&&!d.getElementById('applyButton').disabled);
   a.deepEqual(JSON.parse(f.calls.find(x=>x.path==='service/upstreams/check').opts.body),{operation:'check',upstream:'old'});
   d.getElementById('checkAllDNS').click();
   // A request is recorded before the async handler has finished. Closing
   // jsdom here leaves renderHealth() running against a destroyed document.
   // Wait for the second check to finish and the UI busy guard to clear.
   await until(()=>f.calls.filter(x=>x.path==='service/upstreams/check').length===2 &&
     d.getElementById('feedback').textContent.includes('Проверка всех DNS завершена') &&
     !d.getElementById('checkAllDNS').disabled);
   await tick(); // allow task()'s final refreshStatus() promise to settle
   a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});

test('P19 recovery shows desired vs observed and explicit retry without inventing a hardware PASS',async()=>{
 const f=fixture(),d=f.w.document;
 try{
  await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
  d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));d.getElementById('applyButton').click();await until(()=>f.getActive());
  for(const state of ['DEGRADED','RECOVERY_REQUIRED','CONFLICT']){
   f.setGate({state,active:false,desired:true,last_error:'xtables lock budget exhausted',lease_seconds:60});d.getElementById('refreshLogs').click();await until(()=>d.getElementById('gateAdvice').textContent.includes('xtables lock')&&!d.getElementById('gatePreflight').disabled&&d.getElementById('gateBadge').textContent===(state==='DEGRADED'?'Восстановление':state==='CONFLICT'?'Конфликт':'Требуется проверка'));
   a.match(d.getElementById('gateAdvice').textContent,/Желаемое состояние: ВКЛ/);a.equal(d.getElementById('gatePreflight').textContent,'Повторить восстановление');a.equal(d.getElementById('gateEnable').disabled,true);a.equal(d.getElementById('gateDisable').disabled,false);
  }
  d.getElementById('gatePreflight').click();await until(()=>d.getElementById('gateBadge').textContent==='Активен');
  a.ok(f.calls.some(c=>c.path==='service/gate/control'&&JSON.parse(c.opts.body).operation==='retry'));
  a.match(d.getElementById('gateAdvice').textContent,/не этим счётчиком/);a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});

test('P19.1: IPv4-only coverage never claims full or link-local IPv6 interception',async()=>{
 const f=fixture(),d=f.w.document;
 try{
  await until(()=>d.getElementById('serverCount').textContent.includes('2 DNS'));
  d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
  d.getElementById('applyButton').click();await until(()=>f.getActive()&&!d.getElementById('applyButton').disabled);
  f.setGate({state:'ACTIVE',active:true,desired:true,coverage:{ipv4:'active_last_verified',ipv6_routed:'excluded_no_managed_routed_prefix',ipv6_link_local:'excluded_unproven_interface_scope'}});
  d.getElementById('refreshLogs').click();await until(()=>d.getElementById('gateAdvice').textContent.includes('только IPv4'));
  a.match(d.getElementById('gateAdvice').textContent,/link-local source исключены/);
  a.doesNotMatch(d.getElementById('gateAdvice').textContent,/обе настроенные|FULL|IPv6 E2E PASS/);
  f.setGate({state:'ACTIVE',active:true,desired:true,coverage:{ipv4:'active_last_verified',ipv6_routed:'active_last_verified',ipv6_link_local:'excluded_unproven_interface_scope'}});
  d.getElementById('refreshLogs').click();await until(()=>d.getElementById('gateAdvice').textContent.includes('IPv4 и IPv6 global/ULA'));
  a.match(d.getElementById('gateAdvice').textContent,/link-local source исключены/);
  a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});


test('P19.3: existing DNS can refresh TLS without ID or domain loss',async()=>{
 const f=fixture(),d=f.w.document;
 try{
  await until(()=>d.querySelectorAll('.dns-card').length===2);
  d.querySelector('[data-upstream-id="cf"] .dns-change').click();
  a.equal(d.getElementById('dnsAddPanelTitle').textContent,'Изменить DNS / обновить TLS');
  d.getElementById('inspectDNS').click();await until(()=>/TLS проверен/.test(d.getElementById('certificateInfo').textContent)&&!d.getElementById('commitDNS').disabled&&!d.getElementById('inspectDNS').disabled);
  d.getElementById('commitDNS').click();await until(()=>d.getElementById('feedback').textContent.includes('DNS/TLS обновлены')&&!d.getElementById('saveButton').disabled);
  d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
  d.getElementById('saveButton').click();await until(()=>f.getConfig().upstreams[1].address==='8.8.8.8:443');
  a.equal(f.getConfig().upstreams[1].id,'cf');
  a.ok(f.getConfig().upstreams[1].certificate_pem);
  a.equal(d.querySelectorAll('.dns-card').length,2);
  a.equal(d.querySelector('[data-upstream-id="cf"] .dns-preview').textContent.includes('other.net'),true);
  a.equal(f.getActive(),false);
  await until(()=>!d.getElementById('saveButton').disabled);
  a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});

test('P19.3: two independent named lists bind to one DNS and keep legacy rules',async()=>{
 const f=fixture(),d=f.w.document;
 try{
  await until(()=>d.querySelectorAll('.dns-card').length===2);
  d.querySelector('[data-upstream-id="cf"] .dns-edit').click();
  d.getElementById('newListName').value='Видео';d.getElementById('newListButton').click();
  await until(()=>!d.getElementById('newListButton').disabled);
  d.getElementById('editorDomains').value='youtube.com';d.getElementById('saveDomains').click();
  await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён')&&!d.getElementById('newListButton').disabled);
  d.getElementById('newListName').value='Игры';d.getElementById('newListButton').click();
  await until(()=>!d.getElementById('newListButton').disabled);
  d.getElementById('editorDomains').value='steamcommunity.com';d.getElementById('saveDomains').click();
  await until(()=>d.getElementById('feedback').textContent.includes('Список обновлён')&&!d.getElementById('newListButton').disabled);
  d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
  d.getElementById('saveButton').click();await until(()=>f.getConfig().lists?.length===2);
  const c=f.getConfig();a.equal(c.rules.length,4);a.equal(c.lists.length,2);
  a.ok(c.rules.find(r=>r.domain==='other.net'&&!r.list_id),'old ungrouped rules stay unchanged');
  a.ok(c.rules.find(r=>r.domain==='youtube.com'&&r.list_id===c.lists[0].id));
  a.ok(c.rules.find(r=>r.domain==='steamcommunity.com'&&r.list_id===c.lists[1].id));
  await until(()=>!d.getElementById('saveButton').disabled);
  a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});

test('P19.3: log shows real DNS endpoint, client, type, pause and filter',async()=>{
 const f=fixture(),d=f.w.document;
 try{
  await until(()=>d.querySelectorAll('.dns-card').length===2);
  d.getElementById('defaultDNS').value='old';d.getElementById('defaultDNS').dispatchEvent(new f.w.Event('change'));
  d.getElementById('applyButton').click();await until(()=>f.getActive());
  d.getElementById('navLogs').click();await until(()=>d.querySelectorAll('.log-row').length>0);
  a.match(d.getElementById('logRows').textContent,/9\.9\.9\.9:53/);
  a.match(d.getElementById('logRows').textContent,/example\.com/);
  d.getElementById('logPause').click();a.match(d.getElementById('logPause').textContent,/Продолжить/);
  d.getElementById('logType').value='AAAA';d.getElementById('logType').dispatchEvent(new f.w.Event('change'));
  a.equal(d.querySelectorAll('.log-row').length,0);
  a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});


test('P19.4: independent draft probe visibly verifies address, TLS, DNS and RTT',async()=>{
 const f=fixture(),d=f.w.document;
 try {
  await until(()=>d.querySelectorAll('.dns-card').length===2);
  d.querySelector('[data-upstream-id="cf"] .dns-change').click();
  d.getElementById('inspectDNS').click();
  await until(()=>d.getElementById('draftDNSState').textContent==='Доступен'&&!d.getElementById('inspectDNS').disabled);
  a.match(d.getElementById('draftIPState').textContent,/8\.8\.8\.8:443/);
  a.match(d.getElementById('draftTLSState').textContent,/Проверен/);
  a.match(d.getElementById('draftRTTState').textContent,/24 мс/);
  a.ok(f.calls.some(x=>x.path==='service/upstreams/inspect'));
  a.equal(f.getActive(),false,'draft probe must not start DNS');
  await tick();a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});

test('P19.4: compact DNS cards preserve actions and list/domain counters',async()=>{
 const f=fixture(),d=f.w.document;
 try{
  await until(()=>d.querySelectorAll('.dns-card').length===2);
  const c=d.querySelector('[data-upstream-id="cf"]');
  a.match(c.querySelector('.dns-count').textContent,/сп\. · 1 дом\./);
  a.ok(c.querySelector('.dns-card-actions .dns-edit'));
  a.ok(c.querySelector('.dns-card-actions .dns-change'));
  a.ok(c.querySelector('.dns-card-actions .dns-check'));
  a.ok(c.querySelector('.dns-card-actions details summary'));
  a.match(css,/--rf-p194-pad/);
  a.deepEqual(f.errors,[]);
 }finally{f.dom.window.close();}
});

test('P19.5b: initial DNS selection and safe cross-DNS list editing',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('editorDNSChoice').value==='old' && d.getElementById('editorDomains').value==='example.com');
    a.equal(d.getElementById('editorEmpty').hidden,true,'first eligible DNS is selected');
    a.equal(d.querySelector('[data-upstream-id="old"]').classList.contains('is-selected'),true);
    a.equal(d.getElementById('defaultDNS').value,'','display selection must not change default upstream');
    a.equal(f.getActive(),false,'auto-selection must not start the DNS service');
    a.equal(f.calls.filter(c=>['POST','PUT','PATCH','DELETE'].includes(c.opts.method)).length,0,'auto-selection must be read only');
    d.getElementById('editorDNSChoice').value='cf';
    d.getElementById('editorDNSChoice').dispatchEvent(new f.w.Event('change'));
    a.equal(d.getElementById('editorDomains').value,'other.net');
    a.equal(d.querySelector('[data-upstream-id="cf"]').classList.contains('is-selected'),true);
    d.getElementById('editorDomains').value='other.net\nunsaved.example';
    f.w.confirm=()=>false;
    d.getElementById('editorDNSChoice').value='old';
    d.getElementById('editorDNSChoice').dispatchEvent(new f.w.Event('change'));
    a.equal(d.getElementById('editorDNSChoice').value,'cf','cancel restores selector');
    a.equal(d.getElementById('editorDomains').value,'other.net\nunsaved.example','unsaved domain draft survives cancelled switch');
    a.equal(f.getActive(),false);
    a.deepEqual(f.errors,[]);
  } finally {f.dom.window.close();}
});
