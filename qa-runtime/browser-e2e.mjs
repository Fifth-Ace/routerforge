// Runtime RT-C3: actual compiled Svelte and real Chrome CDP, LOCALHOST FIXTURES ONLY.
// No dependencies beyond Node >=22 and project-pinned npm modules used for the build.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import http from 'node:http';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import crypto from 'node:crypto';
import {isMockOriginRequest, auditBrowserBoundary, redactNetworkUrl} from './browser-network-boundary.mjs';
import {pointExpression} from './browser-interaction-expression.mjs';
const here=path.dirname(fileURLToPath(import.meta.url));
const root=path.resolve(here,'..');
const ui=path.join(root,'modules/dns-router-runtime/frontend');
const componentDir=process.env.RF_RUNTIME_COMPONENT_GATE_OUTDIR||path.join(root,'qa-runtime/component-build');
const evidence=process.env.RF_RTC3_EVIDENCE||path.join(root,'qa-runtime/evidence');
const selfTest=process.argv.includes('--self-test');
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const sha=p=>crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
fs.mkdirSync(evidence,{recursive:true});
const txt=(name,val)=>fs.writeFileSync(path.join(evidence,name),typeof val==='string'?val:JSON.stringify(val,null,2));
if(!selfTest){
 for(const p of [path.join(ui,'build/svelte-mode.js'),path.join(componentDir,'component-gate.js'),path.join(ui,'index.html')]){
  if(!fs.existsSync(p)||fs.statSync(p).size===0)throw Error('BLOCKED_COMPILED_SVELTE_ASSET_MISSING: '+p);
 }
}
const fixture={};
function resetFixture(){
 fixture.calls=[];fixture.errors=[];fixture.unknownWrites=[];fixture.revision='r1';fixture.config={schema_version:1,upstreams:[{id:'u1',transport:'dot',address:'9.9.9.9:853'}],rules:[]};
 fixture.shadow={lifecycle:{active:false,state:'stopped',applied_revision:null}};
 fixture.putStatus=200;fixture.putInvalidMime=false;fixture.shadowStatus=200;fixture.configGetStatus=200;
 fixture.confirmDefault=false;fixture.holdConfigPut=null;fixture.holdShadowGet=null;fixture.shadowControlStatus=200;
 fixture.blockUnknown=true;
 fixture.putStatusBody=null;fixture.holdConfigGet=null;fixture.holdNativeGet=null;fixture.holdShadowControl=null;
 fixture.holdImportPreview=null;fixture.importBaseRevision=null;fixture.importBadPreview=false;
 fixture.managedReady=false;fixture.managedToken='trial-token-test-only';fixture.trialStatus=200;fixture.trialConsumed=false;
 fixture.ownership={state:'disabled',owned_zones:[]};fixture.prepareToken='durable-token-test-only';
 fixture.durableReady=false;fixture.durableApplyStatus=200;fixture.durableDisableStatus=200;
 fixture.diagnosticsCalls=0;
}
resetFixture();
function sendJSON(res,status,body,headers={}){if(res.destroyed)return;res.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'no-store',...headers});res.end(JSON.stringify(body));}
async function requestBody(req){let b='';for await(const chunk of req){b+=String(chunk);if(b.length>1024*1024)throw Error('FIXTURE_BODY_TOO_LARGE');}return b;}
const server=http.createServer(async (req,res)=>{
 try{
  const u=new URL(req.url,'http://127.0.0.1'); const pathname=u.pathname;
  if(pathname.startsWith('/v1/ui/')){
   const asset=pathname.slice('/v1/ui/'.length);
   if(asset==='component-test.html'){
    res.writeHead(200,{'Content-Type':'text/html; charset=utf-8'});
    res.end('<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="module.css"></head><body><div id="app"></div><script defer src="component-gate.js"></script></body></html>');return;
   }
   if(asset==='self-test.html'){
    // Test-only CDP click/focus preflight. Never shipped as a production asset.
    res.writeHead(200,{'Content-Type':'text/html; charset=utf-8'});
    res.end(`<!doctype html><title>CDP CHECK</title><main id="cdp-smoke">PASS</main><button id="testVisible" onclick="document.getElementById('clickCount').textContent=String(Number(document.getElementById('clickCount').textContent)+1)">Visible</button><div hidden><button id="testHidden">Hidden</button></div><button id="testDisabled" disabled>Disabled</button><output id="clickCount">0</output>`);return;
   }
   let file=null;
   if(asset==='component-gate.js')file=path.join(componentDir,'component-gate.js');
   else if(asset==='svelte-mode.js')file=path.join(ui,'build/svelte-mode.js');
   else if(asset==='index.html'||asset==='')file=path.join(ui,'index.html');
   else if(asset==='module.css')file=path.join(ui,'module.css');
   else if(/^[A-Za-z0-9_.-]+\.css$/.test(asset))file=path.join(ui,'build',asset);
   if(!file||!fs.existsSync(file)){sendJSON(res,404,{error:'UI_ASSET_NOT_FOUND'});return;}
   const type=asset.endsWith('.js')?'text/javascript; charset=utf-8':asset.endsWith('.css')?'text/css; charset=utf-8':'text/html; charset=utf-8';
   res.writeHead(200,{'Content-Type':type,'Cache-Control':'no-store'});fs.createReadStream(file).pipe(res);return;
  }
  const body=await requestBody(req);
  const call={method:req.method,path:pathname,query:u.search,headers:{'if-match':req.headers['if-match']||null,'x-routerforge-action':req.headers['x-routerforge-action']||null,'content-type':req.headers['content-type']||null,'x-routerforge-ledger-match':req.headers['x-routerforge-ledger-match']||null},body};
  fixture.calls.push(call);
  if(pathname==='/v1/config'&&req.method==='GET'){
   if(fixture.holdConfigGet)await fixture.holdConfigGet.promise;
   sendJSON(res,fixture.configGetStatus,fixture.configGetStatus===200?{revision:fixture.revision,config:fixture.config}:{error:'denied'});return;
  }
  if(pathname==='/v1/config'&&req.method==='PUT'){
   if(fixture.holdConfigPut)await fixture.holdConfigPut.promise;
   if(fixture.putStatus!==200){sendJSON(res,fixture.putStatus,{error:'TEST_DENIED'});return;}
   if(call.headers['if-match']!==`"${fixture.revision}"` || call.headers['x-routerforge-action']!=='save'){sendJSON(res,409,{error:'CAS_MISMATCH'});return;}
   fixture.config=JSON.parse(body);fixture.revision='r2';
   if(fixture.putInvalidMime){res.writeHead(200,{'Content-Type':'text/plain'});res.end('not-json');return;}
   sendJSON(res,200,{revision:'r2'});return;
  }
  if(pathname==='/v1/service/status'&&req.method==='GET'){sendJSON(res,404,{error:'independent-dns absent'});return;}
  if(pathname==='/v1/keenetic/native-dns'&&req.method==='GET'){
   if(fixture.holdNativeGet)await fixture.holdNativeGet.promise;
   sendJSON(res,200,{available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[{address:'127.0.0.1:40500',domain:'.',upstream_hint:'9.9.9.9'}]}]});return;
  }
  if(pathname==='/v1/keenetic/native-policies'&&req.method==='GET'){
   sendJSON(res,200,{available:true,policies:[{name:'System',description:'Системная'}]});return;
  }
  if(pathname==='/v1/keenetic/managed-ownership/status'&&req.method==='GET'){
   sendJSON(res,200,fixture.ownership);return;
  }
  if(pathname==='/v1/shadow/status'&&req.method==='GET'){
   const shadowSnapshot=structuredClone(fixture.shadow);const statusSnapshot=fixture.shadowStatus;
   if(fixture.holdShadowGet)await fixture.holdShadowGet.promise;
   sendJSON(res,statusSnapshot,statusSnapshot===200?shadowSnapshot:{error:'TEST_OFFLINE'});return;
  }
  if(pathname==='/v1/shadow/control'&&req.method==='POST'){
   if(fixture.holdShadowControl)await fixture.holdShadowControl.promise;
   if(fixture.shadowControlStatus!==200){sendJSON(res,fixture.shadowControlStatus,{error:'TEST_DENIED'});return;}
   if(call.headers['if-match']!==`"${fixture.revision}"` || call.headers['x-routerforge-action']!=='shadow-control'){
    sendJSON(res,409,{error:'SHADOW_CAS_MISMATCH'});return;
   }
   const op=JSON.parse(body).operation;
   if(!['start','apply','stop'].includes(op)){sendJSON(res,400,{error:'INVALID_OPERATION'});return;}
   fixture.shadow={lifecycle:{active:op!=='stop',state:op==='stop'?'stopped':'running',applied_revision:fixture.revision}};
   sendJSON(res,200,{outcome:op==='stop'?'stopped':'started'});return;
  }
  if(pathname==='/v1/runtime/resources'&&req.method==='GET'){fixture.diagnosticsCalls++;sendJSON(res,200,{rss_bytes:0,pss_bytes:0});return;}
  if(pathname==='/v1/keenetic/discovery'&&req.method==='GET'){sendJSON(res,200,{available:false});return;}
  if(pathname==='/v1/import-preview'&&req.method==='POST'){
   const revisionAtRequest=fixture.revision;
   if(fixture.holdImportPreview)await fixture.holdImportPreview.promise;
   const proposed=structuredClone(fixture.config);
   proposed.rules.push({id:'import-r1',domain:'import.example',match:'suffix',upstream:'u1',priority:100});
   const preview=fixture.importBadPreview?{config:proposed,added:'INVALID',duplicates:0,ignored:0,result_rules:proposed.rules.length}
     :{config:proposed,added:1,duplicates:0,ignored:0,result_rules:proposed.rules.length};
   sendJSON(res,200,{base_revision:fixture.importBaseRevision||revisionAtRequest,preview});return;
  }
  if(pathname==='/v1/keenetic/domain-managed/prepare'&&req.method==='POST'){
   if(call.headers['if-match']!==`"${fixture.revision}"` || call.headers['x-routerforge-action']!=='managed-prepare') {sendJSON(res,409,{error:'MANAGED_CAS_MISMATCH'});return;}
   sendJSON(res,200,fixture.managedReady?{ready:true,revision:fixture.revision,token:fixture.managedToken,zones:[{domain:'test.invalid'}]}:{ready:false,revision:fixture.revision,zones:[],conflicts:['mock-disabled']});return;
  }
  if(pathname==='/v1/keenetic/domain-managed/trial'&&req.method==='POST'){
   const t=JSON.parse(body||'{}');
   if(call.headers['if-match']!==`"${fixture.revision}"` || call.headers['x-routerforge-action']!=='managed-trial'||t.token!==fixture.managedToken||fixture.trialConsumed){sendJSON(res,409,{error:'TOKEN_CONSUMED_OR_CAS'});return;}
   fixture.trialConsumed=true;
   if(fixture.trialStatus!==200){sendJSON(res,fixture.trialStatus,{error:'MOCK_TRIAL_DENIED'});return;}
   sendJSON(res,200,{state:'pass',baseline_restored:true});return;
  }
  if(pathname==='/v1/keenetic/durable/prepare'&&req.method==='POST'){
   if(call.headers['if-match']!==`"${fixture.revision}"`||call.headers['x-routerforge-action']!=='durable-prepare') {sendJSON(res,409,{error:'DURABLE_CAS'});return;}
   sendJSON(res,200,fixture.durableReady?{ready:true,token:fixture.prepareToken,zones:[{domain:'test.invalid'}]}:{ready:false,conflicts:['mock-conflict']});return;
  }
  if(pathname==='/v1/keenetic/durable/apply'&&req.method==='POST'){
   if(!fixture.durableReady||call.headers['if-match']!==`"${fixture.revision}"`||call.headers['x-routerforge-action']!=='durable-apply'||JSON.parse(body).token!==fixture.prepareToken){sendJSON(res,409,{error:'DURABLE_CAS'});return;}
   if(fixture.durableApplyStatus!==200){sendJSON(res,fixture.durableApplyStatus,{error:'MOCK_APPLY_DENIED'});return;}
   fixture.ownership={state:'verified_active',ledger_hash:'ledger-mock',owned_zones:['test.invalid']};
   sendJSON(res,200,{state:'active',durable_change:true});return;
  }
  if(pathname==='/v1/keenetic/durable/disable'&&req.method==='POST'){
   if(fixture.durableDisableStatus!==200){sendJSON(res,fixture.durableDisableStatus,{error:'MOCK_ROLLBACK_DENIED'});return;}
   if(call.headers['x-routerforge-action']!=='durable-disable'||call.headers['x-routerforge-ledger-match']!=='ledger-mock') {sendJSON(res,409,{error:'LEDGER_CAS'});return;}
   fixture.ownership={state:'disabled',owned_zones:[]};sendJSON(res,200,{state:'disabled',baseline_restored:true});return;
  }
  // No public network/proxy and no accidental router write in any mock scenario.
  if(req.method!=='GET'&&req.method!=='HEAD')fixture.unknownWrites.push(call);
  sendJSON(res,404,{error:'MOCK_DENIES_UNKNOWN_ENDPOINT'});
 }catch(e){fixture.errors.push(String(e));sendJSON(res,500,{error:'FIXTURE_ERROR'});}
});
const chromeCandidates=()=>{
 const result=[];if(process.env.RF_CHROME_PATH)result.push(process.env.RF_CHROME_PATH);
 if(process.platform==='win32'){
  for(const root of [process.env.PROGRAMFILES,process.env['PROGRAMFILES(X86)'],process.env.LOCALAPPDATA]){
   if(root){result.push(path.join(root,'Google/Chrome/Application/chrome.exe'),path.join(root,'Microsoft/Edge/Application/msedge.exe'));}
  }
 } else result.push('/usr/bin/chromium','/usr/bin/google-chrome','/usr/bin/chromium-browser');
 return result.filter(Boolean).find(p=>fs.existsSync(p));
};
async function waitFor(fn,label,timeoutMs=14000){const began=Date.now();let last;while(Date.now()-began<timeoutMs){try{const v=await fn();if(v)return v;}catch(e){last=e;}await sleep(65);}throw Error('WAIT_TIMEOUT: '+label+(last?': '+last.message:''));}
class CDP {
 constructor(ws){this.ws=ws;this.seq=0;this.pending=new Map();this.events=[];this.exceptions=[];this.blockedExternal=[];this.interceptErrors=[];
  ws.addEventListener('message',event=>{
   let msg;try{msg=JSON.parse(String(event.data))}catch{return}
   if(msg.id){const pending=this.pending.get(msg.id);if(!pending)return;this.pending.delete(msg.id);if(msg.error)pending.reject(Error(JSON.stringify(msg.error)));else pending.resolve(msg.result||{});return;}
   this.events.push(msg);
   if(msg.method==='Runtime.exceptionThrown')this.exceptions.push(msg.params?.exceptionDetails?.text||'EXCEPTION');
   if(msg.method==='Fetch.requestPaused') {
    const u=msg.params?.request?.url||'';
    const allow=isMockOriginRequest(u,baseUrl);
    if(!allow)this.blockedExternal.push({url:u,reason:'BlockedByClient'});
    const method=allow?'Fetch.continueRequest':'Fetch.failRequest';
    const params=allow?{requestId:msg.params.requestId}:{requestId:msg.params.requestId,errorReason:'BlockedByClient'};
    void this.send(method,params).catch(error=>{this.interceptErrors.push({url:redactNetworkUrl(u),error:String(error)});});
   }
   if(this.events.length>3000)this.events.shift();
  });
 }
 send(method,params={}){
  const id=++this.seq;
  return new Promise((resolve,reject)=>{
   const timeout=setTimeout(()=>{this.pending.delete(id);reject(Error('CDP_COMMAND_TIMEOUT '+method));},16000);
   this.pending.set(id,{resolve:r=>{clearTimeout(timeout);resolve(r)},reject:e=>{clearTimeout(timeout);reject(e)}});
   this.ws.send(JSON.stringify({id,method,params}));
  });
 }
 async eval(expr){const r=await this.send('Runtime.evaluate',{expression:expr,returnByValue:true,awaitPromise:true});
  if(r.exceptionDetails)throw Error('BROWSER_JS: '+(r.exceptionDetails.exception?.description||r.exceptionDetails.text||'exception'));
  return r.result?.value;
 }
 async load(url,selector='#dnsRouterWorkspace'){
  await this.send('Page.navigate',{url});
  if(selector)await waitFor(()=>this.eval(`!!document.querySelector(${JSON.stringify(selector)})`),'DOM '+selector);
  await sleep(250);
 }
 async screenshot(name){const r=await this.send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});const f=path.join(evidence,name+'.png');fs.writeFileSync(f,Buffer.from(r.data,'base64'));return {path:f,sha256:sha(f)};}
 async viewport(width,height,mobile=false){await this.send('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile});}
 // Real CDP pointer/keyboard input. A hidden or disabled element cannot pass by
 // triggering element.click() from JavaScript while inaccessible to a user.
 async point(selector,expectDisabled=false){
  return this.eval(pointExpression(selector,expectDisabled));
 }
 async click(selector,options={}){
  const point=await this.point(selector,options.expectDisabled===true);
  await this.send('Input.dispatchMouseEvent',{type:'mouseMoved',x:point.x,y:point.y});
  await this.send('Input.dispatchMouseEvent',{type:'mousePressed',x:point.x,y:point.y,button:'left',clickCount:1});
  await this.send('Input.dispatchMouseEvent',{type:'mouseReleased',x:point.x,y:point.y,button:'left',clickCount:1});
 }
 async fill(selector,value){
  await this.focus(selector);
  await this.send('Input.dispatchKeyEvent',{type:'keyDown',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:2});
  await this.send('Input.dispatchKeyEvent',{type:'keyUp',key:'a',code:'KeyA',windowsVirtualKeyCode:65,modifiers:2});
  await this.send('Input.insertText',{text:String(value)});
  const actual=await this.eval(`document.querySelector(${JSON.stringify(selector)})?.value`);
  assert.equal(actual,String(value),'REAL_INPUT_VALUE_MISMATCH: '+selector);
 }
 async focus(selector){
  await this.point(selector);
  const ok=await this.eval(`(()=>{const e=document.querySelector(${JSON.stringify(selector)});e.focus();return document.activeElement===e})()`);
  assert.equal(ok,true,'VISIBLE_ELEMENT_FOCUS_FAILED: '+selector);
 }
 async key(key){await this.send('Input.dispatchKeyEvent',{type:'keyDown',key,code:key==='ArrowRight'?'ArrowRight':key,windowsVirtualKeyCode:key==='ArrowRight'?39:36});await this.send('Input.dispatchKeyEvent',{type:'keyUp',key,code:key==='ArrowRight'?'ArrowRight':key,windowsVirtualKeyCode:key==='ArrowRight'?39:36});}
 close(){this.ws.close();}
}
let chrome,cdp,port,profile,baseUrl;
const results=[];
async function launch(){
 const executable=chromeCandidates();if(!executable)throw Error('BLOCKED_CHROMIUM_BINARY_NOT_FOUND');
 profile=fs.mkdtempSync(path.join(os.tmpdir(),'rf-rtc3-cdp-'));
 const args=['--headless=new','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-extensions','--disable-sync','--no-proxy-server','--proxy-bypass-list=127.0.0.1;localhost','--disable-features=MediaRouter','--disable-dev-shm-usage','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'];
 if(process.platform!=='win32'&&process.getuid?.()===0)args.push('--no-sandbox');
 const chromeLog=fs.openSync(path.join(evidence,'chromium.log'),'w');
 chrome=spawn(executable,args,{stdio:['ignore',chromeLog,chromeLog],windowsHide:true});
 const f=path.join(profile,'DevToolsActivePort');
 const debugPort=await waitFor(()=>fs.existsSync(f)?Number(fs.readFileSync(f,'utf8').split(/\r?\n/)[0]):false,'DevToolsActivePort',18000);
 const pages=await waitFor(async()=>{const r=await fetch(`http://127.0.0.1:${debugPort}/json/list`);return r.ok?await r.json():false;},'Chrome pages');
 const page=pages.find(p=>p.type==='page');if(!page)throw Error('NO_BROWSER_PAGE');
 const ws=new WebSocket(page.webSocketDebuggerUrl);
 await new Promise((res,rej)=>{ws.addEventListener('open',res,{once:true});ws.addEventListener('error',rej,{once:true});});
 cdp=new CDP(ws);
 await cdp.send('Page.enable');await cdp.send('Runtime.enable');await cdp.send('Network.enable');
 await cdp.send('Fetch.enable',{patterns:[{urlPattern:'*',requestStage:'Request'}]});
 await cdp.viewport(1440,900,false);
}
async function saveCase(id,action){
 const start=Date.now();resetFixture();cdp.events=[];cdp.exceptions=[];cdp.blockedExternal=[];cdp.interceptErrors=[];
 try{
  await action();
  // Allow enough time for injected content-scripts to attempt their requests.
  // All non-mock requests were blocked at Fetch.requestPaused before any wire IO.
  await sleep(100);
  assert.deepEqual(cdp.interceptErrors,[],'CDP_NETWORK_INTERCEPTION_FAILURE');
  const audit=auditBrowserBoundary(cdp.events,cdp.blockedExternal,baseUrl);
  assert.deepEqual(audit.bypassed,[],'CDP_NETWORK_BOUNDARY_BYPASSED');
  assert.deepEqual(audit.unknown,[],'UNEXPECTED_EXTERNAL_BROWSER_REQUEST_BLOCKED');
  const noise=audit.environmentBlocked;
  if(noise)console.log('ENV_INJECTION_BLOCKED / ADGUARD: '+noise);
  assert.deepEqual(cdp.exceptions,[],'BROWSER_UNCAUGHT_EXCEPTION');
  assert.deepEqual(fixture.errors,[],'MOCK_SERVER_ERROR');
  assert.deepEqual(fixture.unknownWrites,[],'UNMOCKED_MUTATION_ATTEMPT');
  results.push({id,status:'PASS',elapsedMs:Date.now()-start,requestCount:fixture.calls.length,environmentBlocked:noise});
  txt(id+'.json',{status:'PASS',calls:fixture.calls,exceptions:cdp.exceptions,environmentBlocked:noise,networkBlocked:audit.blockedLog,requests:cdp.events.filter(x=>x.method==='Network.requestWillBeSent').map(x=>redactNetworkUrl(x.params?.request?.url||''))});
  console.log('PASS '+id);
 }catch(error){
  let screenshot=null;try{screenshot=await cdp.screenshot(id+'_FAIL');}catch{}
  let bodyText='';try{bodyText=await cdp.eval(`document.body?.innerText?.slice(0,800) || ''`);}catch{}
  const status=/is blocked|doesn.t allow you to view this site|administrator has blocked/i.test(bodyText)?'BLOCKED':'FAIL';
  results.push({id,status,error:String(error.stack||error),screenshot});
  txt(id+'.json',{status,error:String(error.stack||error),calls:fixture.calls,events:cdp.events.filter(x=>['Network.requestWillBeSent','Fetch.requestPaused','Runtime.exceptionThrown','Network.loadingFailed'].includes(x.method)).map(x=>({method:x.method,url:redactNetworkUrl(x.params?.request?.url||''),reason:x.params?.errorText||'',exception:x.method==='Runtime.exceptionThrown'?x.params?.exceptionDetails?.text:''})),networkBlocked:cdp.blockedExternal.map(x=>({url:redactNetworkUrl(x.url),reason:x.reason})),interceptErrors:cdp.interceptErrors,exceptions:cdp.exceptions,screenshot,bodyText});
  throw Error((status==='BLOCKED'?'BROWSER_POLICY_BLOCKED ':'FIRST_REAL_FAIL ')+id+': '+error.message);
 }
}
async function app(){await cdp.load(baseUrl+'/v1/ui/index.html');await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'config loaded');}
async function advanced(){
 await cdp.click('#drModeAdvanced');
 await waitFor(()=>cdp.eval(`document.querySelector('#drAdvancedWorkspace')?.hidden === false && document.querySelector('#drModeAdvanced')?.getAttribute('aria-pressed') === 'true'`),'advanced mode is user-visible');
}
async function appAdvanced(){await app();await advanced();}
async function selectTab(name){await cdp.click('#dr-tab-'+name);await waitFor(()=>cdp.eval(`document.querySelector('#dr-tab-${name}')?.getAttribute('aria-selected')==='true'`),'selected tab '+name);}
async function stageDomain(domain){
 await waitFor(()=>cdp.eval(`!document.querySelector('#drSimpleAdd')?.disabled && !!document.querySelector('#drSimpleDNS option')`),'usable DNS choice');
 await cdp.fill('#drSimpleDomains',domain);
 await cdp.click('#drSimpleAdd');
 await waitFor(()=>cdp.eval(`document.getElementById('metricFooter')?.textContent?.includes('несохранённые')`),'domain staged in draft');
}
async function importForm(text){await selectTab('testing');await waitFor(()=>cdp.eval(`!document.querySelector('#importForm button[type="submit"]')?.disabled`),'import form ready');await cdp.fill('#importForm textarea[name="text"]',text);await cdp.click('#importForm button[type="submit"]');}
async function mounted(){await component();await cdp.eval('window.__rfRuntimeComponentTest.mount()');await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'component config loaded');}
function requests(method,path){return fixture.calls.filter(c=>c.method===method&&c.path===path);}
function assertNoSystemMutations(){assert.equal(fixture.calls.filter(c=>c.method!=='GET'&&c.method!=='HEAD' && !['/v1/config','/v1/import-preview'].includes(c.path)).length,0,'UNEXPECTED_SYSTEM_MUTATION');}

async function interactionPreflight(){
 await cdp.load(baseUrl+'/v1/ui/self-test.html','#cdp-smoke');
 await cdp.click('#testVisible');
 assert.equal(await cdp.eval(`document.getElementById('clickCount').textContent`),'1','CDP_REAL_MOUSE_CLICK_NOT_DISPATCHED');
 await cdp.focus('#testVisible');
 await assert.rejects(()=>cdp.click('#testHidden'),/ELEMENT_HIDDEN/);
 await assert.rejects(()=>cdp.focus('#testHidden'),/ELEMENT_HIDDEN/);
 await assert.rejects(()=>cdp.click('#testDisabled'),/ELEMENT_DISABLED/);
 assert.equal(await cdp.eval(`document.getElementById('clickCount').textContent`),'1');
 console.log('CDP_INTERACTION_PREFLIGHT: PASS / real pointer, focus, hidden and disabled guards');
}
async function component(){await cdp.load(baseUrl+'/v1/ui/component-test.html',null);await waitFor(()=>cdp.eval(`!!window.__rfRuntimeComponentTest`),'component gate');}
const writes=()=>fixture.calls.filter(c=>c.method!=='GET'&&c.method!=='HEAD');
function deferred(){let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve};}
try{
 await new Promise((res,rej)=>{server.listen(0,'127.0.0.1',res);server.on('error',rej)});
 port=server.address().port;baseUrl=`http://127.0.0.1:${port}`;
 await launch();
 txt('versions.json',{node:process.version,chrome:await cdp.send('Browser.getVersion'),source:'d069c1f09f95392368ab0e128857bc5a67e96c75',productionBundle:selfTest?null:sha(path.join(ui,'build/svelte-mode.js')),componentBundle:selfTest?null:sha(path.join(componentDir,'component-gate.js'))});
 if(selfTest){
  await saveCase('CDP-SELFTEST',async()=>{await interactionPreflight();assert.equal(await cdp.eval('document.querySelector("#cdp-smoke").textContent'),'PASS');await cdp.screenshot('cdp_selftest');});
 }else{
  await interactionPreflight();
  await saveCase('RTC3-01',async()=>{await app();assert.equal(writes().length,0);assert.ok(fixture.calls.some(x=>x.path==='/v1/config'&&x.method==='GET'));assert.ok(fixture.calls.some(x=>x.path==='/v1/shadow/status'&&x.method==='GET'));await cdp.screenshot('desktop_1440x900');});
  await saveCase('RTC3-02',async()=>{await appAdvanced();await cdp.click('#dr-tab-policies');assert.equal(await cdp.eval(`document.querySelector('#dr-tab-policies').getAttribute('aria-selected')`),'true');await cdp.focus('#dr-tab-policies');await cdp.key('ArrowRight');await waitFor(()=>cdp.eval(`document.activeElement?.id==='dr-tab-testing'`),'keyboard focus dr-tab-testing');assert.equal(await cdp.eval(`document.querySelector('#dr-tab-testing').getAttribute('aria-selected')`),'true');assert.equal(await cdp.eval(`document.querySelector('#dr-panel-testing').getAttribute('aria-hidden')`),'false');assert.equal(writes().length,0);});
  await saveCase('RTC3-03',async()=>{fixture.config.upstreams=[];await app();assert.equal(writes().length,0);assert.ok(await cdp.eval(`!!document.getElementById('dnsRouterWorkspace')`));});
  await saveCase('RTC3-04',async()=>{fixture.shadowStatus=503;await app();await waitFor(()=>cdp.eval(`document.querySelector('#rf-dns-runtime-svelte-mode [role="status"]')?.textContent?.includes('503')`),'visible shadow 503');assert.equal(writes().length,0);});
  await saveCase('RTC3-06',async()=>{await cdp.viewport(390,844,true);await app();assert.ok(await cdp.eval(`!!document.getElementById('dnsRouterWorkspace')`));await cdp.screenshot('mobile_390x844');await advanced();await cdp.click('#dr-tab-testing');assert.equal(await cdp.eval(`document.querySelector('#dr-tab-testing').getAttribute('aria-selected')`),'true');await cdp.screenshot('mobile_advanced_390x844');await cdp.viewport(1440,900,false);assert.equal(writes().length,0);});
  await saveCase('RTC3-09',async()=>{await appAdvanced();await cdp.click('#saveButton');await waitFor(()=>fixture.calls.some(x=>x.method==='PUT'),'PUT');await waitFor(()=>cdp.eval(`document.getElementById('revision')?.textContent?.includes('r2')`),'saved rev');const req=writes()[0];assert.equal(writes().length,1);assert.equal(req.path,'/v1/config');assert.equal(req.method,'PUT');assert.equal(req.headers['if-match'],'"r1"');assert.equal(req.headers['x-routerforge-action'],'save');});
  await saveCase('RTC3-10',async()=>{fixture.holdConfigPut=deferred();await appAdvanced();await cdp.click('#saveButton');await waitFor(()=>writes().length===1,'initial PUT');await waitFor(()=>cdp.eval(`document.querySelector('#saveButton')?.disabled===true`),'save disabled while PUT pending');await cdp.click('#saveButton',{expectDisabled:true});await sleep(200);assert.equal(writes().length,1);fixture.holdConfigPut.resolve();await waitFor(()=>cdp.eval(`document.getElementById('revision')?.textContent?.includes('r2')`),'saved');});
  await saveCase('RTC3-12',async()=>{fixture.putStatus=403;await appAdvanced();await cdp.click('#saveButton');await waitFor(()=>cdp.eval(`document.querySelector('#rf-dns-runtime-svelte-save [role="alert"]')?.textContent?.includes('403')`),'403 visible');assert.equal(writes().length,1);assert.ok(await cdp.eval(`document.getElementById('revision')?.textContent?.includes('r1')`));});
  await saveCase('RTC3-13',async()=>{fixture.putStatus=409;await appAdvanced();await cdp.click('#saveButton');await waitFor(()=>cdp.eval(`document.querySelector('#rf-dns-runtime-svelte-save [role="alert"]')?.textContent?.includes('409')`),'409 visible');assert.equal(writes().length,1);assert.ok(await cdp.eval(`document.getElementById('revision')?.textContent?.includes('r1')`));});
  await saveCase('RTC3-14',async()=>{fixture.putInvalidMime=true;await appAdvanced();await cdp.click('#saveButton');await waitFor(()=>cdp.eval(`document.querySelector('#rf-dns-runtime-svelte-save [role="alert"]')?.textContent?.includes('JSON')`),'invalid JSON visible');assert.equal(writes().length,1);assert.ok(await cdp.eval(`document.getElementById('revision')?.textContent?.includes('r1')`));});
  await saveCase('RTC3-05',async()=>{await component();const info=await cdp.eval(`window.__rfRuntimeComponentTest.mount()`);assert.equal(info.mounted,true);const removed=await cdp.eval(`window.__rfRuntimeComponentTest.unmount()`);assert.equal(removed.mounted,false);assert.equal(removed.domChildren,0);const remount=await cdp.eval(`window.__rfRuntimeComponentTest.mount()`);assert.equal(remount.mounted,true);assert.equal(writes().length,0);});
  await saveCase('RTC3-24',async()=>{await appAdvanced();await cdp.eval('window.confirm=()=>false');await cdp.click('#shadowStartButton');await sleep(150);assert.equal(writes().length,0);});
  await saveCase('RTC3-25',async()=>{await appAdvanced();await cdp.eval('window.confirm=()=>true');await cdp.click('#shadowStartButton');await waitFor(()=>writes().length>0,'shadow POST');assert.equal(writes().length,1);const r=writes()[0];assert.equal(r.path,'/v1/shadow/control');assert.equal(r.headers['if-match'],'"r1"');assert.equal(r.headers['x-routerforge-action'],'shadow-control');});
  await saveCase('RTC3-26',async()=>{await appAdvanced();await cdp.eval('window.confirm=()=>true');fixture.shadowControlStatus=403;await cdp.click('#shadowApplyButton');await waitFor(()=>writes().length>0,'shadow denied');await waitFor(()=>cdp.eval(`document.querySelector('#feedback')?.textContent?.includes('403')`),'shadow denied feedback');assert.equal(writes().length,1);assert.equal(fixture.shadow.lifecycle.active,false);});
  // The following 20 cases execute against the *compiled* Svelte application,
  // with real CDP pointer/keyboard events and a fail-closed localhost API.
  await saveCase('RTC3-07',async()=>{
   await app();await waitFor(()=>cdp.eval(`!document.querySelector('#drSimpleAdd')?.disabled`),'native DNS choice');
   await stageDomain('alpha.example');
   assert.equal(requests('PUT','/v1/config').length,0);
   assert.equal(writes().length,0);
   assert.ok(await cdp.eval(`document.getElementById('drSimpleRuleCount')?.textContent?.includes('1 домен')`));
   assert.ok(await cdp.eval(`document.getElementById('revision')?.textContent?.includes('r1')`));
  });
  await saveCase('RTC3-08',async()=>{
   fixture.config.rules=[{id:'existing',domain:'duplicate.example',match:'suffix',upstream:'u1',priority:100}];
   await app();await waitFor(()=>cdp.eval(`!document.querySelector('#drSimpleAdd')?.disabled`),'native DNS choice');
   await cdp.fill('#drSimpleDomains','duplicate.example');await cdp.click('#drSimpleAdd');
   await waitFor(()=>cdp.eval(`document.querySelector('#drSimpleState')?.textContent?.includes('уже есть')`),'duplicate rejection');
   assert.ok(!(await cdp.eval(`document.getElementById('metricFooter')?.textContent?.includes('несохранённые')`)));
   assert.equal(writes().length,0);
  });
  await saveCase('RTC3-11',async()=>{
   fixture.holdConfigPut=deferred();await appAdvanced();await cdp.click('#saveButton');
   await waitFor(()=>requests('PUT','/v1/config').length===1,'held CAS PUT');
   await cdp.click('#drModeSimple');await stageDomain('changed-mid-save.example');
   fixture.holdConfigPut.resolve();
   await waitFor(()=>cdp.eval(`document.getElementById('revision')?.textContent?.includes('r2')`),'server revision committed');
   assert.ok(await cdp.eval(`document.getElementById('metricFooter')?.textContent?.includes('несохранённые')`));
   assert.equal(requests('PUT','/v1/config').length,1);
   assertNoSystemMutations();
  });
  await saveCase('RTC3-15',async()=>{
   fixture.putStatus=428;await appAdvanced();await cdp.click('#saveButton');
   await waitFor(()=>cdp.eval(`document.querySelector('#rf-dns-runtime-svelte-save [role="alert"]')?.textContent?.includes('428')`),'If-Match refusal visible');
   assert.equal(requests('PUT','/v1/config').length,1);
   assert.ok(await cdp.eval(`document.getElementById('revision')?.textContent?.includes('r1')`));
  });
  await saveCase('RTC3-16',async()=>{
   await app();await stageDomain('unsaved-reload.example');
   await cdp.eval('window.confirm=()=>false');const before=requests('GET','/v1/config').length;
   await cdp.click('#reloadButton');await sleep(125);
   assert.equal(requests('GET','/v1/config').length,before,'RELOAD_REQUEST_ISSUED_DESPITE_REFUSED_CONFIRM');
   assert.ok(await cdp.eval(`document.getElementById('metricFooter')?.textContent?.includes('несохранённые')`));
   assert.equal(writes().length,0);
  });
  await saveCase('RTC3-17',async()=>{
   await mounted();fixture.holdConfigGet=deferred();await cdp.click('#reloadButton');
   await waitFor(()=>requests('GET','/v1/config').length>=2,'held config reload');
   const before=await cdp.eval('window.__rfRuntimeComponentTest.snapshot()');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');fixture.holdConfigGet.resolve();
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'remounted config');
   const after=await cdp.eval('window.__rfRuntimeComponentTest.snapshot()');
   assert.equal(after.revision,before.revision);assert.equal(after.dirty,false);
   assert.equal(writes().length,0);
  });
  await saveCase('RTC3-18',async()=>{
   await appAdvanced();await cdp.eval('window.confirm=()=>true');await importForm('import.example');
   await waitFor(()=>cdp.eval(`document.querySelector('#importResult')?.textContent?.includes('Добавлено: 1')`),'import preview applied locally');
   assert.equal(requests('POST','/v1/import-preview').length,1);
   assert.equal(requests('PUT','/v1/config').length,0);
   assert.equal(fixture.config.rules.length,0,'MOCK_PERSISTED_CONFIG_CHANGED_DURING_PREVIEW');
   assert.ok(await cdp.eval(`document.getElementById('metricFooter')?.textContent?.includes('несохранённые')`));
   assert.equal(requests('POST','/v1/import-preview')[0].headers['x-routerforge-action'],'import-preview');
  });
  await saveCase('RTC3-19',async()=>{
   await mounted();await advanced();await cdp.eval('window.confirm=()=>true');
   fixture.holdImportPreview=deferred();await importForm('import.example');
   await waitFor(()=>requests('POST','/v1/import-preview').length===1,'held preview');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');fixture.holdImportPreview.resolve();
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'remount after import');
   const state=await cdp.eval('window.__rfRuntimeComponentTest.snapshot()');
   assert.equal(state.dirty,false,'ABANDONED_IMPORT_STAGED_AFTER_UNMOUNT');
   assert.equal(requests('PUT','/v1/config').length,0);
  });
  await saveCase('RTC3-20',async()=>{
   fixture.importBaseRevision='r-stale';await appAdvanced();await cdp.eval('window.confirm=()=>true');
   await importForm('import.example');
   await waitFor(()=>cdp.eval(`document.querySelector('#rf-dns-runtime-svelte-import [role="alert"]')?.textContent?.includes('изменилась')`),'stale import denied');
   assert.ok(!(await cdp.eval(`document.getElementById('metricFooter')?.textContent?.includes('несохранённые')`)));
   assert.equal(requests('POST','/v1/import-preview').length,1);assert.equal(requests('PUT','/v1/config').length,0);
  });
  await saveCase('RTC3-21',async()=>{
   await mounted();fixture.holdNativeGet=deferred();await cdp.click('#drSimpleScan');
   await waitFor(()=>requests('GET','/v1/keenetic/native-dns').length>=2,'native scan pending');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');fixture.holdNativeGet.resolve();
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'scan remount');
   assert.equal(writes().length,0);
   assert.ok(await cdp.eval(`!!document.querySelector('#drSimpleScan')`));
  });
  await saveCase('RTC3-22',async()=>{
   fixture.shadowControlStatus=409;await appAdvanced();await cdp.eval('window.confirm=()=>true');
   await cdp.click('#shadowApplyButton');
   await waitFor(()=>cdp.eval(`document.querySelector('#feedback')?.textContent?.includes('409')`),'shadow CAS 409 visible');
   assert.equal(requests('POST','/v1/shadow/control').length,1);
   assert.equal(fixture.shadow.lifecycle.active,false);
  });
  await saveCase('RTC3-23',async()=>{
   await mounted();await advanced();fixture.holdShadowGet=deferred();await cdp.click('#shadowStatusButton');
   await waitFor(()=>requests('GET','/v1/shadow/status').length>=2,'held shadow GET');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');fixture.holdShadowGet.resolve();
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'shadow view remount');
   assert.equal(writes().length,0);
   assert.equal(await cdp.eval(`document.querySelector('#shadowBadge')?.textContent?.includes('АКТИВЕН')`),false);
  });
  await saveCase('RTC3-27',async()=>{
   await mounted();await advanced();await cdp.eval('window.confirm=()=>true');
   fixture.holdShadowControl=deferred();await cdp.click('#shadowStartButton');
   await waitFor(()=>requests('POST','/v1/shadow/control').length===1,'held shadow POST');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');fixture.holdShadowControl.resolve();
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'post teardown remount');
   await sleep(200);
   assert.equal(requests('POST','/v1/shadow/control').length,1,'UNCERTAIN_WRITE_WAS_RETRIED');
   assert.equal(await cdp.eval(`document.querySelector('#feedback')?.textContent?.includes('Shadow: started')`),false,'STALE_SUCCESS_AFTER_UNMOUNT');
  });
  await saveCase('RTC3-28',async()=>{
   await mounted();await advanced();fixture.holdShadowGet=deferred();await cdp.click('#shadowStatusButton');
   await waitFor(()=>requests('GET','/v1/shadow/status').length>=2,'old shadow status pending');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');
   fixture.shadow={lifecycle:{active:true,state:'running',applied_revision:'r1'}};
   fixture.holdShadowGet.resolve();await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#shadowBadge')?.textContent?.includes('АКТИВЕН')`),'current remount shadow status');
   await sleep(150);assert.ok(await cdp.eval(`document.querySelector('#shadowBadge')?.textContent?.includes('АКТИВЕН')`));
   assert.equal(writes().length,0);
  });
  await saveCase('RTC3-29',async()=>{
   await mounted();await advanced();fixture.holdConfigPut=deferred();await cdp.click('#saveButton');
   await waitFor(()=>requests('PUT','/v1/config').length===1,'held unmount save');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');fixture.holdConfigPut.resolve();
   await waitFor(()=>fixture.revision==='r2','server applied released save');
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#revision')?.textContent?.includes('r2')`),'new mount gets saved revision');
   assert.equal(requests('PUT','/v1/config').length,1);
   assert.equal(await cdp.eval(`document.querySelector('#feedback')?.textContent?.includes('Сохранено атомарно')`),false,'OLD_SAVE_PUBLISHED_SUCCESS');
  });
  await saveCase('RTC3-30',async()=>{
   fixture.putStatus=403;await appAdvanced();await cdp.click('#saveButton');
   await waitFor(()=>cdp.eval(`document.getElementById('feedback')?.textContent?.includes('HTTP 403')`),'reactive 403 feedback');
   assert.equal(await cdp.eval(`document.querySelectorAll('#feedback').length`),1);
   assert.equal(await cdp.eval(`document.querySelector('#feedback').getAttribute('role')`),'status');
   assert.equal(requests('PUT','/v1/config').length,1);
  });
  await saveCase('RTC3-31',async()=>{
   fixture.managedReady=true;await appAdvanced();await selectTab('keenetic');
   await cdp.click('#managedPrepareN');
   await waitFor(()=>cdp.eval(`document.querySelector('#managedStatusN')?.textContent?.includes('"ready": true')`),'managed plan');
   await cdp.eval(`window.confirm=()=>true;window.prompt=()=>"TRIAL"`);
   await cdp.click('#managedTrialN');
   await waitFor(()=>requests('POST','/v1/keenetic/domain-managed/trial').length===1,'trial POST');
   await waitFor(()=>cdp.eval(`document.querySelector('#managedStatusN')?.textContent?.includes('baseline_restored')`),'trial rollback response');
   assert.equal(fixture.trialConsumed,true);assert.equal(requests('POST','/v1/keenetic/domain-managed/prepare').length,1);
   assert.equal(requests('POST','/v1/keenetic/domain-managed/trial')[0].headers['if-match'],'"r1"');
   await waitFor(()=>cdp.eval(`!document.querySelector('#managedTrialN')?.disabled`),'trial button available');
   await cdp.click('#managedTrialN');await sleep(150);
   assert.equal(requests('POST','/v1/keenetic/domain-managed/trial').length,1,'ONE_SHOT_TOKEN_REUSED');
  });
  await saveCase('RTC3-32',async()=>{
   fixture.config.rules=[{id:'r1',domain:'test.invalid',match:'suffix',upstream:'u1',priority:100}];
   fixture.durableReady=true;await app();await cdp.eval('window.confirm=()=>true');
   await waitFor(()=>cdp.eval(`!document.querySelector('#drP92Apply')?.disabled`),'durable activation available');
   await cdp.click('#drP92Apply');
   await waitFor(()=>requests('POST','/v1/keenetic/durable/apply').length===1,'durable apply POST');
   await waitFor(()=>cdp.eval(`document.querySelector('#drP10Runtime')?.textContent?.includes('DNS ACTIVE')`),'managed DNS active');
   assert.equal(requests('POST','/v1/keenetic/durable/prepare').length,1);
   assert.equal(requests('POST','/v1/keenetic/durable/apply')[0].headers['if-match'],'"r2"');
   assert.equal(fixture.ownership.state,'verified_active');
  });
  await saveCase('RTC3-33',async()=>{
   fixture.config.rules=[{id:'r1',domain:'test.invalid',match:'suffix',upstream:'u1',priority:100}];
   fixture.durableReady=true;await app();await cdp.eval('window.confirm=()=>true');
   await waitFor(()=>cdp.eval(`!document.querySelector('#drP92Apply')?.disabled`),'durable activation available');
   await cdp.click('#drP92Apply');
   await waitFor(()=>cdp.eval(`document.querySelector('#drP10Runtime')?.textContent?.includes('DNS ACTIVE')`),'DNS active for rollback');
   fixture.durableDisableStatus=409;
   await waitFor(()=>cdp.eval(`!document.querySelector('#drP10Disable')?.disabled`),'disable enabled');
   const before=requests('POST','/v1/shadow/control').length;
   await cdp.click('#drP10Disable');
   await waitFor(()=>requests('POST','/v1/keenetic/durable/disable').length===1,'ledger rollback denial');
   assert.equal(requests('POST','/v1/shadow/control').length,before,'SHADOW_STOPPED_BEFORE_LEDGER_RESTORATION');
   assert.equal(fixture.ownership.state,'verified_active');
   fixture.durableDisableStatus=200;
   await waitFor(()=>cdp.eval(`!document.querySelector('#drP10Disable')?.disabled`),'retry after denied rollback');
   await cdp.click('#drP10Disable');
   await waitFor(()=>requests('POST','/v1/keenetic/durable/disable').length===2,'successful ledger rollback');
   await waitFor(()=>requests('POST','/v1/shadow/control').some(x=>JSON.parse(x.body).operation==='stop'),'shadow stop after authoritative restore');
   assert.equal(fixture.ownership.state,'disabled');
  });
  await saveCase('RTC3-34',async()=>{
   await component();
   await cdp.eval(`(()=>{const oldSet=window.setInterval,oldClear=window.clearInterval;window.__rtIntervals=new Set();window.setInterval=function(fn,delay,...rest){const id=oldSet.call(window,fn,delay,...rest);window.__rtIntervals.add(id);return id};window.clearInterval=function(id){window.__rtIntervals.delete(id);return oldClear.call(window,id)}})()`);
   await cdp.eval('window.__rfRuntimeComponentTest.mount()');
   await waitFor(()=>cdp.eval(`document.querySelector('#metricConfig')?.textContent?.includes('СОХРАНЕНО')`),'diag mount');
   await advanced();await selectTab('diagnostics');
   await waitFor(()=>fixture.diagnosticsCalls>=1,'diagnostics GET');
   const intervals=await cdp.eval('window.__rtIntervals.size');assert.ok(intervals>0,'DIAGNOSTICS_POLL_TIMER_NOT_INSTALLED');
   await cdp.eval('window.__rfRuntimeComponentTest.unmount()');
   assert.equal(await cdp.eval('window.__rtIntervals.size'),0,'POLL_TIMER_LEAK_AFTER_UNMOUNT');
   const before=fixture.diagnosticsCalls;await sleep(200);assert.equal(fixture.diagnosticsCalls,before,'POLLING_CONTINUES_AFTER_UNMOUNT');
   assert.equal(writes().length,0);
  });

 }
 txt('RESULT.json',{base:'d069c1f09f95392368ab0e128857bc5a67e96c75',cases:results,pass:results.filter(x=>x.status==='PASS').length,fail:results.filter(x=>x.status==='FAIL').length,scope:selfTest?'CDP harness selftest':'REAL_COMPILED_SVELTE_LOCALHOST_BROWSER'});
 console.log('=== RESULT ===');console.log('STATE: PASS / '+results.length+' BROWSER CASES');
}catch(e){const blocked=String(e).includes('BROWSER_POLICY_BLOCKED')||String(e).includes('BLOCKED_');console.error('=== '+(blocked?'BLOCKED':'FIRST REAL FAIL')+' / STOP ===\n'+String(e.stack||e));txt('RESULT.json',{base:'d069c1f09f95392368ab0e128857bc5a67e96c75',cases:results,state:blocked?'BLOCKED':'FAIL',error:String(e.stack||e)});process.exitCode=blocked?2:1;}
finally{
 try{cdp?.close();}catch{}
 try{server.close();}catch{}
 try{chrome?.kill('SIGTERM');}catch{}
 if(chrome&&process.platform==='win32')try{spawn('taskkill',['/PID',String(chrome.pid),'/T','/F'],{windowsHide:true,stdio:'ignore'}).unref();}catch{}
 if(profile)try{fs.rmSync(profile,{recursive:true,force:true,maxRetries:4,retryDelay:250});}catch{}
}
