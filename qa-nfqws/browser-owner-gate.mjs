// NFQWS remaining owners: real compiled Svelte, real Chromium CDP, local HTTP fixtures.
// Network boundary: localhost HTTP or CDP in-memory fixture when managed Chromium
// blocks loopback. Never touches router / real GitHub or active config.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFileSync,existsSync,mkdtempSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {spawn} from 'node:child_process';
import {waitForRenderedState} from '../modules/nfqws-manager/frontend/NfqwsBrowserReadinessModel.js';
import {strategyFixtureReply,STRATEGY_LIBRARY_FIXTURE} from './browser-strategy-fixture.mjs';
const cwd=resolve(import.meta.dirname,'../modules/nfqws-manager/frontend');
const bundle=join(cwd,'build/svelte-mode.js');
if(!existsSync(bundle))throw Error('BLOCKED_COMPILED_NFQWS_BUNDLE_NOT_AVAILABLE');
if(typeof WebSocket!=='function')throw Error('BLOCKED_NODE22_WEBSOCKET_REQUIRED');
const chrome=process.env.RF_CHROMIUM|| (process.platform==='win32'?process.env.PROGRAMFILES+'\\Google\\Chrome\\Application\\chrome.exe':'/usr/bin/chromium');
if(!existsSync(chrome))throw Error('BLOCKED_CHROMIUM_BINARY_MISSING: '+chrome);
const requests=[],errors=[],blockedExternal=[],consoleErrors=[];
let browserStage='PRECHECK';
let fixtureMode='http',navigationResult=null;
const sha='a'.repeat(64);
const config='ISP_INTERFACE="eth0"\nNFQWS_ARGS="--filter-tcp=443"\nNFQWS_ARGS_QUIC=""\nNFQWS_ARGS_UDP=""\nNFQWS_ARGS_IPSET=""\nNFQWS_BASE_ARGS=""\nIPV6_ENABLED="0"\nTCP_PORTS="443"\nUDP_PORTS="443"\nNFQUEUE_NUM="200"\nUSER="root"\nCONFIG_VERSION="1"\n';
let responseMode=409;
async function requestLog(){return fixtureMode==='cdp'?(await evaluate('globalThis.__rfRequests||[]')):requests;}
async function setResponseMode(status){responseMode=status;if(fixtureMode==='cdp')await evaluate(`(globalThis.__rfResponseMode=${status})`);}
const json=(res,status,obj)=>{res.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'no-store'});res.end(JSON.stringify(obj));};
const server=createServer(async(req,res)=>{
 const url=new URL(req.url,'http://127.0.0.1');
 if(req.method==='GET'&&['/','/index.html','/ui/'].includes(url.pathname)){
  res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store'});
  res.end(readFileSync(join(cwd,'index.html')));return;
 }
 if(req.method==='GET'&&url.pathname==='/svelte-mode.js'){
  res.writeHead(200,{'Content-Type':'text/javascript; charset=utf-8','Cache-Control':'no-store'});res.end(readFileSync(bundle));return;
 }
 if(req.method==='GET'&&url.pathname==='/module.css'){
  res.writeHead(200,{'Content-Type':'text/css; charset=utf-8','Cache-Control':'no-store'});res.end(readFileSync(join(cwd,'module.css')));return;
 }
 let body='';for await(const chunk of req){body+=chunk;if(body.length>2_000_000){json(res,413,{error:'LOCAL_FIXTURE_BODY_TOO_LARGE'});return;}}
 requests.push({method:req.method,path:url.pathname,body});
 const p=url.pathname.replace(/^\/api\/modules\/nfqws-manager/,'');
 if(req.method==='GET'){
  if(p==='/status'){json(res,200,{available:true,running:false,mutation_ready:true,target_name:'fixture-nfqws',config_path:'/opt/etc/nfqws2/nfqws2.conf',config_sha256:sha,config,lists:[],backup_count:0});return;}
  if(p==='/configs'){json(res,200,{files:[]});return;}
  if(p==='/blobs'){json(res,200,{root:'/opt/etc/nfqws2/blobs',files:[{name:'sample.bin',size:3,sha256:'b'.repeat(64),referenced:false,references:[]}]});return;}
  if(p==='/keenetic'){json(res,200,{detected:true,ndmc_available:true,model:'Fixture Keenetic',interfaces:[{name:'eth0',default_v4:true,up:true}],default_ipv4_interface:'eth0',config_isp_interface:'eth0'});return;}
  if(p==='/v2/nfqueue-health'){json(res,200,{state:'UNKNOWN',ok:false,production_queues:[],firewall_queues:[],kernel_queues:[],read_only:true});return;}
  if(p==='/v2/bench-profiles'){json(res,200,{profiles:[]});return;}
  if(p==='/v2/policy-automation-gate'){json(res,200,{automation_enabled:false,reason:'TEST_ONLY'});return;}
  if(p==='/backups'){json(res,200,{files:[]});return;}
  if(p==='/v2/strategies'){json(res,200,STRATEGY_LIBRARY_FIXTURE);return;}
  json(res,404,{error:'LOCAL_FIXTURE_UNSUPPORTED_GET '+p});return;
 }
 const strategyReply=strategyFixtureReply(req.method,p,body);
 if(strategyReply){json(res,strategyReply.status,strategyReply.payload);return;}
 if(req.method==='POST'&&p==='/config'){
  const payload=JSON.parse(body);if(payload.confirm!=='NFQWS_CONFIG'){json(res,403,{error:'CONFIRM_REQUIRED'});return;}
  json(res,responseMode,{error:responseMode===409?'CAS_CONFLICT':'DENIED'});return;
 }
 if(req.method==='POST'&&p==='/blobs/delete'){
  const payload=JSON.parse(body);if(payload.confirm!=='NFQWS_BLOB_DELETE'){json(res,403,{error:'CONFIRM_REQUIRED'});return;}
  json(res,403,{error:'FIXTURE_FORBIDDEN'});return;
 }
 json(res,403,{error:'TEST_ONLY_NO_UNEXPECTED_MUTATION'});
});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
async function until(fn,what,ms=8000){browserStage=what;return waitForRenderedState(fn,{label:what,timeoutMs:ms,beforeProbe:()=>{if(errors.length)throw Error('BROWSER_JS_EXCEPTION_AT_'+what+': '+errors[0]);}})}
async function start(){await new Promise((yes,no)=>server.once('error',no).listen(0,'127.0.0.1',yes));return server.address().port;}
const page=(expr)=>`(()=>{${expr}})()`;
let browser=null,ws=null,profile=null,seq=0,listen=new Map();
async function send(method,params={},timeout=8000){const id=++seq;return new Promise((resolve,reject)=>{const timer=setTimeout(()=>{listen.delete(id);reject(Error('CDP_TIMEOUT '+method))},timeout);listen.set(id,{resolve,reject,timer});ws.send(JSON.stringify({id,method,params}));});}
async function evaluate(source,timeout=8000){const d=await send('Runtime.evaluate',{expression:source,returnByValue:true,awaitPromise:true},timeout);if(d.exceptionDetails)throw Error('JS_EVAL: '+(d.exceptionDetails.exception?.description||d.exceptionDetails.text));return d.result?.value;}
async function click(selector){browserStage='CLICK '+selector;const expr=page(`const x=document.querySelector(${JSON.stringify(selector)});if(!x)return false;x.click();return true;`);assert.equal(await evaluate(expr),true,'ELEMENT_MISSING '+selector);}
async function gate(label,predicate){browserStage=label;assert.equal(await evaluate(predicate),true,label);console.log('BROWSER_GATE PASS / '+label)}
async function navigate(tab){await click(`[data-tab="${tab}"]`);await until(async()=>await evaluate(`document.querySelector('#view-${tab}')?.classList.contains('active')`),tab);}
let passed=0;
try{
 const port=await start();
 profile=mkdtempSync(join(tmpdir(),'rf-nfqws-owner-cdp-'));
 const flags=['--headless=new','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-extensions','--disable-sync','--no-proxy-server','--proxy-bypass-list=127.0.0.1;localhost','--disable-dev-shm-usage','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'];
 if(process.platform!=='win32'&&process.getuid?.()===0)flags.push('--no-sandbox');
 browser=spawn(chrome,flags,{stdio:'ignore',windowsHide:true});
 const file=join(profile,'DevToolsActivePort');await until(()=>existsSync(file),'DEVTOOLS_START',18000);
 const remotePort=Number(readFileSync(file,'utf8').split(/\r?\n/)[0]);
 const tabs=await (await fetch(`http://127.0.0.1:${remotePort}/json/list`)).json();
 const tab=tabs.find(x=>x.type==='page');if(!tab)throw Error('CHROMIUM_NO_PAGE');
 ws=new WebSocket(tab.webSocketDebuggerUrl);
 await new Promise((res,rej)=>{ws.addEventListener('open',res,{once:true});ws.addEventListener('error',rej,{once:true});});
 ws.addEventListener('message',event=>{const msg=JSON.parse(event.data);if(msg.id){const job=listen.get(msg.id);if(job){clearTimeout(job.timer);listen.delete(msg.id);msg.error?job.reject(Error(msg.error.message)):job.resolve(msg.result);}return;}
  if(msg.method==='Runtime.exceptionThrown')errors.push(JSON.stringify({text:msg.params.exceptionDetails.text,exception:msg.params.exceptionDetails.exception?.description,stack:msg.params.exceptionDetails.stackTrace?.callFrames?.slice(0,5)}));
  if(msg.method==='Runtime.consoleAPICalled'&&msg.params.type==='error')consoleErrors.push(msg.params.args?.map(x=>x.value||x.description||'').join(' ')||'console.error');
  if(msg.method==='Fetch.requestPaused'){
   const req=msg.params.request,uri=req.url,allowed=uri.startsWith(`http://127.0.0.1:${port}/`);
   if(allowed)void send('Fetch.continueRequest',{requestId:msg.params.requestId}).catch(e=>errors.push('CONTINUE '+e.message));
   else{blockedExternal.push(uri);void send('Fetch.failRequest',{requestId:msg.params.requestId,errorReason:'BlockedByClient'}).catch(e=>errors.push('BLOCK '+e.message));}
  }
 });
 await send('Page.enable');await send('Runtime.enable');await send('Fetch.enable',{patterns:[{urlPattern:'*',requestStage:'Request'}]});
 navigationResult=await send('Page.navigate',{url:`http://127.0.0.1:${port}/index.html`});
 if(navigationResult.errorText){
  if(!/ERR_BLOCKED_BY_(?:ADMINISTRATOR|CLIENT)/.test(navigationResult.errorText))throw Error('CHROMIUM_LOCAL_NAVIGATION_FAILED: '+navigationResult.errorText);
  // Some managed Chromium installations forbid even 127.0.0.1. Keep the
  // exact compiled Svelte bundle and real browser, but fulfil strict local
  // API fixtures inside the page; never access router or external network.
  fixtureMode='cdp';
  await send('Page.navigate',{url:'about:blank'});
  const html=readFileSync(join(cwd,'index.html'),'utf8')
    .replace(/<script\s+defer\s+src="svelte-mode\.js"><\/script>/i,'')
    .replace(/<link\s+rel="stylesheet"\s+href="module\.css">/i,'');
  if(/<script\b[^>]*svelte-mode\.js/i.test(html))throw Error('CDP_FIXTURE_SCRIPT_TAG_NOT_STRIPPED');
  const css=readFileSync(join(cwd,'module.css'),'utf8');
  await evaluate(`(function(){document.open();document.write(${JSON.stringify(html)});document.close();const style=document.createElement('style');style.textContent=${JSON.stringify(css)};document.head.appendChild(style);return true;})()`);
  const fixture=`(function(){
    globalThis.__rfRequests=[];globalThis.__rfResponseMode=409;
    const store=new Map();Object.defineProperty(globalThis,'localStorage',{configurable:true,value:{getItem:k=>store.get(k)||null,setItem:(k,v)=>store.set(String(k),String(v)),removeItem:k=>store.delete(k)}});
    const sha='a'.repeat(64),config=${JSON.stringify(config)};
    globalThis.fetch=async function(url,opts={}){
      const u=String(url),method=String(opts.method||'GET').toUpperCase(),prefix='/api/modules/nfqws-manager';
      if(!u.startsWith(prefix+'/'))throw Error('TEST_ONLY_EXTERNAL_FETCH_BLOCKED: '+u.slice(0,150));
      const p=u.slice(prefix.length).split('?')[0],body=String(opts.body||'');
      globalThis.__rfRequests.push({method,path:u.split('?')[0],body});
      let code=200,payload={};
      if(method==='GET'){
        switch(p){
          case '/status':payload={available:true,running:false,mutation_ready:true,target_name:'fixture-nfqws',config_path:'/opt/etc/nfqws2/nfqws2.conf',config_sha256:sha,config,lists:[],backup_count:0};break;
          case '/configs':case '/backups':payload={files:[]};break;
          case '/blobs':payload={root:'/opt/etc/nfqws2/blobs',files:[{name:'sample.bin',size:3,sha256:'b'.repeat(64),referenced:false,references:[]}]};break;
          case '/keenetic':payload={detected:true,ndmc_available:true,model:'Fixture Keenetic',interfaces:[{name:'eth0',default_v4:true,up:true}],default_ipv4_interface:'eth0',config_isp_interface:'eth0'};break;
          case '/v2/nfqueue-health':payload={state:'UNKNOWN',ok:false,production_queues:[],firewall_queues:[],kernel_queues:[],read_only:true};break;
          case '/v2/bench-profiles':payload={profiles:[]};break;
          case '/v2/policy-automation-gate':payload={automation_enabled:false,reason:'TEST_ONLY'};break;
          case '/v2/strategies':payload=${JSON.stringify(STRATEGY_LIBRARY_FIXTURE)};break;
          default:code=404;payload={error:'LOCAL_FIXTURE_UNSUPPORTED_GET '+p};
        }
      } else if(method==='POST'&&(p==='/v2/strategies/save'||p==='/v2/strategies/delete')){
        const required=p.endsWith('/save')?'ROUTERFORGE_V2_STRATEGY_SAVE':'ROUTERFORGE_V2_STRATEGY_DELETE';
        let confirm='';try{confirm=JSON.parse(body)?.confirm}catch{}
        code=403;payload={error:confirm===required?'TEST_ONLY_NO_PERSISTENCE':'CONFIRM_REQUIRED'};
      } else if(method==='POST'&&p==='/config'){
        const sent=JSON.parse(body);code=sent.confirm==='NFQWS_CONFIG'?globalThis.__rfResponseMode:403;
        payload={error:code===409?'CAS_CONFLICT':'DENIED'};
      } else if(method==='POST'&&p==='/blobs/delete'){
        code=403;payload={error:'FIXTURE_FORBIDDEN'};
      } else {code=403;payload={error:'TEST_ONLY_NO_UNEXPECTED_MUTATION'};}
      return new Response(JSON.stringify(payload),{status:code,headers:{'content-type':'application/json; charset=utf-8','cache-control':'no-store'}});
    };
    return true;
  })()`;
  await evaluate(fixture);
  const compiled=readFileSync(bundle,'utf8');
  await evaluate(compiled+'\n//# sourceURL=rf-nfqws-compiled-svelte.js',16000);
  console.log('BROWSER_FIXTURE_MODE: CDP_IN_MEMORY / policy denied localhost; external network blocked');
 }else console.log('BROWSER_FIXTURE_MODE: LOOPBACK_HTTP / external network blocked');
 await until(()=>evaluate("!!document.querySelector('[data-svelte-owned=\"nfqws-blobs\"]')"),'REAL_SVELTE_ROOT');
 await until(async()=>(await requestLog()).some(x=>x.method==='GET'&&x.path.endsWith('/status')),'INITIAL_STATUS');
 console.log('=== BROWSER / REAL CHROMIUM + COMPILED SVELTE ===');
 const essential=['nfqws-blobs','nfqws-catalog','nfqws-run-history','nfqws-domain-batch-check','nfqws-setup-diff','nfqws-setup-diagnostics','nfqws-keenetic','nfqws-runtime-health'];
 for(const owner of essential){await gate('EXACT_SINGLE_OWNER '+owner,`document.querySelectorAll('[data-svelte-owned="${owner}"]').length===1`);passed++;}
 assert.equal((await requestLog()).some(x=>x.method==='POST'),false,'MOUNT_TRIGGERED_MUTATION');passed++;console.log('BROWSER_GATE PASS / MOUNT_NO_POST');
 await navigate('blobs');
 // A GET request can be visible in the fixture before its JSON reaches the
 // controller and before Svelte publishes a state update. Wait for BOTH.
 await until(async()=>{
   const getObserved=(await requestLog()).some(x=>x.path.endsWith('/blobs')&&x.method==='GET');
   if(!getObserved)return false;
   return await evaluate("document.querySelector('#view-blobs.active #blobInstalledCount')?.textContent?.trim()==='1'");
 },'BLOB_INVENTORY_DOM_READY');
 await gate('BLOB_INVENTORY_REACTIVE',"document.querySelector('#view-blobs.active #blobInstalledCount')?.textContent?.trim()==='1'");passed++;
 await navigate('runs');await gate('HISTORY_BROWSER_LOCAL',"document.querySelector('[data-svelte-owned=\"nfqws-run-history\"]')?.textContent.includes('BROWSER LOCAL')");passed++;
 await navigate('keenetic');await until(async()=>(await requestLog()).some(x=>x.path.endsWith('/keenetic')),'KEENETIC_GET');
 await gate('KEENETIC_READ_ONLY',"!!document.querySelector('[data-svelte-owned=\"nfqws-keenetic\"]')");passed++;
 await navigate('runtime-health-v2');await until(async()=>(await requestLog()).some(x=>x.path.endsWith('/v2/nfqueue-health')),'RUNTIME_HEALTH_GET');
 assert.equal((await requestLog()).some(x=>x.method==='POST'),false,'READ_ONLY_TABS_MUTATED');passed++;console.log('BROWSER_GATE PASS / READ_ONLY_TABS_NO_POST');
 // The primary group 'strategies' and the Configuration leaf 'strategies'
 // are distinct. Prove both in Chromium, and the authoritative GET envelope.
 await click('[data-primary-tab="strategies"]');
 await until(()=>evaluate("document.querySelector('#view-strategy-library-v2.active #v2LibraryState')?.textContent?.trim()==='READY'"),'STRATEGY_LIBRARY_READY');
 assert.equal((await requestLog()).some(x=>x.method==='GET'&&x.path.endsWith('/v2/strategies')),true,'STRATEGY_LIBRARY_GET_MISSING');
 assert.equal((await requestLog()).some(x=>x.method==='POST'),false,'STRATEGY_LIBRARY_OPEN_MUTATED');
 passed++;console.log('BROWSER_GATE PASS / STRATEGY_LIBRARY_READ_ONLY');
 await click('[data-primary-tab="configuration"]');
 await until(()=>evaluate("!!document.querySelector('#view-strategies.active')"),'CONFIG_PRIMARY_ROUTE');
 passed++;console.log('BROWSER_GATE PASS / CONFIG_PRIMARY_ROUTE');
 await click('[data-primary-tab="strategies"]');
 await until(()=>evaluate("!!document.querySelector('#view-strategy-library-v2.active')"),'STRATEGY_PRIMARY_RESTORE');
 await click('[data-primary-tab="configuration"]');
 await until(()=>evaluate("!!document.querySelector('#view-strategies.active')"),'CONFIG_PRIMARY_RESTORE');
 await click('[data-tab="strategies"]');
 await until(()=>evaluate("!!document.querySelector('#view-strategies.active')"),'CONFIG_LEAF_DIRECT_ROUTE');
 passed++;console.log('BROWSER_GATE PASS / CONFIG_LEAF_DIRECT_ROUTE');
 await gate('SETUP_DIAGNOSTICS_SVELTE',"!!document.querySelector('[data-svelte-owned=\"nfqws-setup-diagnostics\"]')");passed++;
 const input=await evaluate("!!document.querySelector('#flowInterface')");assert.equal(input,true,'SETUP_INTERFACE_INPUT_MISSING');
 await evaluate(page("const e=document.querySelector('#flowInterface');e.value='eth9';e.dispatchEvent(new Event('input',{bubbles:true}));return true;"));
 await until(()=>evaluate("document.querySelector('.setup-state')?.textContent?.includes('UNSAVED')"),'DRAFT_DIRTY');passed++;console.log('BROWSER_GATE PASS / DRAFT_REACTIVE');
 await click('.setup-sticky button:nth-child(2)');await until(()=>evaluate("document.querySelector('#diffModal')?.classList.contains('open')"),'DIFF_OPEN');
 await gate('DIFF_NO_POST',"!!document.querySelector('#diffModal pre.diff')");passed++;
 assert.equal((await requestLog()).some(x=>x.method==='POST'),false,'DIFF_OPEN_MUTATED');passed++;
 await click('#diffModal .dialog-head button');await gate('DIFF_CLOSE',"!document.querySelector('#diffModal')?.classList.contains('open')");passed++;
 // Deliberate 403/409 with a browser confirmation: fixture only; no router.
 await evaluate('window.confirm=()=>true');
 for(const status of [409,403]){
  await setResponseMode(status);
  const beforeConfigPosts=(await requestLog()).filter(x=>x.method==='POST'&&x.path.endsWith('/config')).length;
  await click('.setup-sticky button:nth-child(2)');await until(()=>evaluate("document.querySelector('#diffModal')?.classList.contains('open')"),'DIFF_OPEN_AGAIN');
  await click('#diffModal .dialog-foot button.primary');
  await until(async()=>(await requestLog()).filter(x=>x.method==='POST'&&x.path.endsWith('/config')).length>beforeConfigPosts,'PROTECTED_CONFIG_POST');
  const last=(await requestLog()).filter(x=>x.method==='POST'&&x.path.endsWith('/config')).at(-1);
  assert.equal(JSON.parse(last.body).confirm,'NFQWS_CONFIG','CONFIRM_TOKEN_LOST');
  assert.equal(await evaluate("document.querySelector('.notice')?.textContent?.includes('успешно')"),false,'FALSE_SUCCESS_ON_DENIED');
  passed++;console.log('BROWSER_GATE PASS / CONFIG_'+status+'_DENIAL');
  await sleep(150);
 }
 assert.equal(errors.length,0,'BROWSER_EXCEPTIONS '+JSON.stringify(errors));
 assert.equal(consoleErrors.length,0,'BROWSER_CONSOLE_ERRORS '+JSON.stringify(consoleErrors));
 assert.equal((await requestLog()).some(x=>x.path.includes('/action')||x.path.includes('/smart-apply')||x.path.includes('/backups/restore')),false,'SURPRISE_MUTATION');
 passed++;console.log('BROWSER_GATE PASS / NO_SURPRISE_MUTATIONS');
 console.log('=== RESULT / BROWSER PASS / '+passed+' / external requests blocked '+blockedExternal.length+' ===');
}catch(error){
 console.error('=== RESULT / BROWSER FAIL / '+error.message+' / passed '+passed+' ===');
 console.error('BROWSER_DIAGNOSTIC / FAILED_STAGE '+browserStage);
 console.error('BROWSER_DIAGNOSTIC / MODE '+fixtureMode+' / NAV '+JSON.stringify(navigationResult));
 console.error('BROWSER_DIAGNOSTIC / SCRIPT '+JSON.stringify({bundleExists:existsSync(bundle),bundleBytes:existsSync(bundle)?readFileSync(bundle).byteLength:0}));
 console.error('BROWSER_DIAGNOSTIC / JS_EXCEPTIONS '+JSON.stringify(errors.slice(0,12)));
 console.error('BROWSER_DIAGNOSTIC / CONSOLE_ERRORS '+JSON.stringify(consoleErrors.slice(0,12)));
 console.error('BROWSER_DIAGNOSTIC / BLOCKED_EXTERNAL '+JSON.stringify(blockedExternal.slice(0,12)));
 try{console.error('BROWSER_DIAGNOSTIC / REQUESTS '+JSON.stringify((await requestLog()).map(x=>({method:x.method,path:x.path})).slice(0,30)));}catch(e){console.error('BROWSER_DIAGNOSTIC / REQUEST_LOG '+e.message)}
 try{console.error('BROWSER_DIAGNOSTIC / PAGE '+JSON.stringify(await evaluate('({url:location.href,ready:document.readyState,title:document.title,root:!!document.querySelector(".shell"),activeView:document.querySelector(".view.active")?.id,blobCount:document.querySelector("#blobInstalledCount")?.textContent?.trim(),owned:document.querySelectorAll("[data-svelte-owned]").length,body:document.body?.innerText?.slice(0,800),scripts:Array.from(document.scripts||[],s=>s.src)})')))}catch(e){console.error('BROWSER_DIAGNOSTIC / PAGE_EVAL '+e.message)}
 process.exitCode=1;
}
finally{
 for(const job of listen.values()){clearTimeout(job.timer);job.reject(Error('BROWSER_END'));}listen.clear();
 try{ws?.close()}catch{}
 try{browser?.kill()}catch{}
 server.closeAllConnections?.();await new Promise(resolve=>server.close(()=>resolve()));
 if(profile)try{rmSync(profile,{recursive:true,force:true})}catch{}
}
