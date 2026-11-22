'use strict';
// RouterForge Runtime: real Svelte component mount/update/unmount, isolated localhost.
// This test-only runner uses the V3 CDP protocol that previously passed on Windows.
// No package installation, no Git, no external API, no router access.
const fs=require('node:fs');
const path=require('node:path');
const http=require('node:http');
const crypto=require('node:crypto');
const {spawn}=require('node:child_process');
const os=require('node:os');
const args=process.argv.slice(2);
function option(name){const at=args.indexOf(name);if(at===-1||at===args.length-1)throw Error('ARG_MISSING: '+name);return args[at+1];}
const source=path.resolve(option('--source'));
const browser=path.resolve(option('--browser'));
const expected=option('--sha256').toLowerCase();
const componentBundlePath=path.resolve(option('--bundle'));
const ui=path.join(source,'modules','dns-router-runtime','frontend');
const htmlPath=path.join(ui,'index.html');
const bundlePath=componentBundlePath;
const cssPath=path.join(ui,'module.css');
const html=fs.readFileSync(htmlPath,'utf8');
const bundle=fs.readFileSync(bundlePath);
if(crypto.createHash('sha256').update(bundle).digest('hex')!==expected)throw Error('COMPILED_BUNDLE_SHA256_MISMATCH');
if(!/src="svelte-mode\.js"/.test(html)||/src="app\.js"/.test(html))throw Error('FRONTEND_ENTRY_NOT_NATIVE');
if(!fs.existsSync(browser)||!fs.statSync(browser).isFile())throw Error('CHROMIUM_BINARY_NOT_FOUND: '+browser);
const cfg={schema_version:1,upstreams:[{id:'u1',transport:'dot',address:'9.9.9.9:853',server_name:'dns.quad9.net'}],rules:[]};
const copy=x=>JSON.parse(JSON.stringify(x));
let savedCfg=copy(cfg),revision='r0',puts=0,unexpectedMutation=0;
let firstConfigPending=true,releaseConfig=null;
const requests=[];
const resources={html:0,bundle:0,css:0,gate:0};
const gateScript=String.raw`
(async()=>{
 const report=[];const errors=[...(window.__rfGateEarlyErrors||[])];
 window.__rfBrowserGateProgress=report;window.__rfBrowserGatePhase='BOOT';
 window.addEventListener('error',e=>errors.push(String(e.message||e.error||'error')));
 window.addEventListener('unhandledrejection',e=>errors.push(String(e.reason||'unhandled')));
 const sleep=ms=>new Promise(r=>setTimeout(r,ms));
 const until=async(fn,label,max=7000)=>{
  window.__rfBrowserGatePhase=label;const end=Date.now()+max;
  while(Date.now()<end){if(await fn())return;await sleep(35);}
  throw Error('WAIT_TIMEOUT: '+label);
 };
 const check=(condition,name)=>{
  if(!condition)throw Error('ASSERT: '+name);
  report.push('PASS / '+name);window.__rfBrowserGatePhase=name;
 };
 const requests=async()=>{const response=await fetch('/__requests',{cache:'no-store'});return response.json();};
 let verdict='FAIL',error='';
 try{
  await until(()=>typeof window.__rfRuntimeComponentTest?.mount==='function','HARNESS_READY');
  const gate=window.__rfRuntimeComponentTest;
  await gate.mount();
  await until(async()=> (await requests()).some(x=>x.path==='/v1/config'),'INITIAL_GET_STARTED');
  check(gate.snapshot().mounted && !!document.getElementById('dnsRouterWorkspace'),'REAL_SVELTE_MOUNT');
  check(!gate.snapshot().loaded,'INITIAL_GET_DELAYED');
  let network=await requests();
  check(network.every(x=>['GET','HEAD'].includes(x.method)),'NO_MUTATION_ON_FIRST_MOUNT');
  const afterUnmount=await gate.unmount();
  check(!afterUnmount.mounted&&afterUnmount.domChildren===0,'SVELTE_UNMOUNT_CLEANS_DOM');
  await fetch('/__release-config',{cache:'no-store'});
  await sleep(125);
  check(!gate.snapshot().loaded&&gate.snapshot().revision==='','LATE_GET_CANNOT_COMMIT_AFTER_UNMOUNT');
  await gate.mount();
  await until(()=>gate.snapshot().loaded,'REMOUNT_CONFIG_GET');
  check(gate.snapshot().revision==='r0','REMOUNT_LOADS_CURRENT_REVISION');
  check(document.querySelectorAll('#dnsRouterWorkspace').length===1,'SINGLE_RUNTIME_OWNER_AFTER_REMOUNT');
  const field=document.getElementById('drSimpleDomains');
  check(!!field,'DOMAIN_EDITOR_MOUNTED');
  field.value='lifecycle.example.com';field.dispatchEvent(new Event('input',{bubbles:true}));
  await until(()=>gate.snapshot().domains==='lifecycle.example.com','DOM_BINDING_UPDATE');
  check(gate.snapshot().domains==='lifecycle.example.com','REAL_REACTIVE_INPUT');
  await gate.unmount();
  check(!gate.snapshot().mounted&&!document.getElementById('dnsRouterWorkspace'),'SECOND_UNMOUNT_CLEANS_DOM');
  await gate.mount();
  await until(()=>document.querySelectorAll('#dnsRouterWorkspace').length===1,'SECOND_REMOUNT');
  check(document.querySelectorAll('#dnsRouterWorkspace').length===1,'NO_DUPLICATE_ROOTS');
  await gate.unmount();
  network=await requests();
  check(network.every(x=>['GET','HEAD'].includes(x.method)),'ALL_MOUNT_ACTIONS_READ_ONLY');
  check(errors.length===0,'NO_UNHANDLED_BROWSER_ERRORS');
  verdict='PASS';window.__rfBrowserGatePhase='COMPLETE';
 }catch(e){error=String(e?.stack||e);window.__rfBrowserGatePhase='FAIL';}
 const el=document.createElement('pre');el.id='RF_BROWSER_GATE_RESULT';el.setAttribute('data-state',verdict);
 el.textContent=verdict+'\n'+report.join('\n')+(error?'\nERROR: '+error:'');document.body.appendChild(el);
})().catch(e=>{const el=document.createElement('pre');el.id='RF_BROWSER_GATE_RESULT';el.setAttribute('data-state','FAIL');el.textContent='FAIL / COMPONENT GATE: '+String(e?.stack||e);document.body.appendChild(el);});`;
const seen=(method,url,req)=>({method,path:url.pathname,action:req.headers['x-routerforge-action']||'',ifMatch:req.headers['if-match']||''});
function respond(res,status,data){const raw=Buffer.from(JSON.stringify(data));res.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Content-Length':raw.length});res.end(raw);}
function respondAsset(res,data,contentType,extra={}){const raw=Buffer.isBuffer(data)?data:Buffer.from(data);res.writeHead(200,{'Content-Type':contentType,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Content-Length':raw.length,...extra});res.end(raw);}
const server=http.createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://127.0.0.1');
  if(url.pathname==='/favicon.ico'){res.writeHead(204);res.end();return;}
  if(req.method==='GET'&&url.pathname==='/__requests'){respond(res,200,requests);return;}
  const record=seen(req.method,url,req);
  if(req.method==='GET'&&url.pathname==='/v1/ui/index.html'){resources.html++;
   const early=`<script>window.__rfGateEarlyErrors=[];window.addEventListener('error',e=>window.__rfGateEarlyErrors.push(String(e.message||e.error||'error')));window.addEventListener('unhandledrejection',e=>window.__rfGateEarlyErrors.push(String(e.reason||'unhandled')));</script>`;
   const transformed=html.replace('<script defer src="svelte-mode.js"></script>','<script defer src="component-gate.js"></script>').replace('</head>',early+'</head>').replace('</body>','<script defer src="/__gate.js"></script></body>');
   respondAsset(res,transformed,'text/html; charset=utf-8',{'Content-Security-Policy':"default-src 'self'; connect-src 'self'; img-src 'self' data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; font-src 'self' data:; object-src 'none'; base-uri 'self'"});return;
  }
  if(req.method==='GET'&&url.pathname==='/__gate.js'){resources.gate++;respondAsset(res,gateScript,'application/javascript; charset=utf-8');return;}
  if(req.method==='GET'&&url.pathname==='/v1/ui/component-gate.js'){resources.bundle++;respondAsset(res,bundle,'application/javascript; charset=utf-8');return;}
  if(req.method==='GET'&&url.pathname==='/v1/ui/module.css'){resources.css++;respondAsset(res,fs.readFileSync(cssPath),'text/css; charset=utf-8');return;}
  if(req.method==='GET'&&url.pathname==='/__release-config'){
   if(releaseConfig){const release=releaseConfig;releaseConfig=null;release();}
   respond(res,200,{released:true});return;
  }
  if(req.method==='PUT'&&url.pathname==='/v1/config'){
   let raw='';for await (const chunk of req){raw+=chunk;if(raw.length>2000000){respond(res,413,{error:'too large'});return;}}
   try{record.body=JSON.parse(raw);}catch{record.body=null;}
   requests.push(record); puts++;
   if(puts===1){respond(res,409,{error:'revision conflict'});return;}
   if(puts===2){respond(res,403,{error:'forbidden'});return;}
   if(record.ifMatch!=='"'+revision+'"'||record.action!=='save'||!record.body?.rules?.length){respond(res,409,{error:'invalid CAS'});return;}
   savedCfg=copy(record.body);revision='r1';respond(res,200,{revision});return;
  }
  requests.push(record);
  if(req.method!=='GET'&&req.method!=='HEAD'){unexpectedMutation++;respond(res,403,{error:'blocked by isolated test'});return;}
  if(url.pathname==='/v1/config'){
    if(firstConfigPending){
      firstConfigPending=false;
      await new Promise(resolve=>{releaseConfig=()=>{
        if(!res.destroyed)respond(res,200,{config:copy(savedCfg),revision});
        resolve();
      };});
      return;
    }
    respond(res,200,{config:copy(savedCfg),revision});return;
  }
  if(url.pathname==='/v1/keenetic/native-dns'){respond(res,200,{available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[]}]});return;}
  if(url.pathname==='/v1/keenetic/native-policies'){respond(res,200,{available:true,policies:[{name:'System',description:'Системная',ipv4_route_status:'default_present'}]});return;}
  if(url.pathname==='/v1/shadow/status'){respond(res,200,{active:false,lifecycle:{active:false,state:'stopped'}});return;}
  respond(res,200,{available:false,mode:false,active:false,status:'inactive',state:'disabled',items:[],policies:[]});
 } catch(e){respond(res,500,{error:String(e.message||e)});}
});
function closeServer(){
 return new Promise(resolve=>{
  if(!server.listening){resolve();return;}
  server.close(()=>resolve());
  if(typeof server.closeAllConnections==='function')server.closeAllConnections();
  const t=setTimeout(resolve,1200);t.unref();
 });
}
function sleep(ms){return new Promise(r=>setTimeout(r,ms));}
function errorExcerpt(s){return String(s||'').slice(-2500).replace(/\r/g,'');}
let stage='PRECHECK';
function gate(name){stage=name;console.log('=== '+name+' ===');}
function fatal(code,message=''){throw Error(code+(message?' / '+message:''));}
async function waitFor(predicate,timeout,label,interval=120){
 const start=Date.now();
 while(Date.now()-start<timeout){
  const value=await predicate();if(value)return value;
  await sleep(interval);
 }
 fatal(label+'_TIMEOUT');
}
function startBrowser(profile){
 const chromeArgs=[
  '--headless=new','--disable-gpu','--disable-dev-shm-usage',
  '--disable-background-networking','--no-first-run','--no-default-browser-check',
  '--disable-extensions','--disable-sync','--disable-component-update',
  '--disable-default-apps','--disable-translate','--no-proxy-server',
  '--remote-debugging-address=127.0.0.1','--remote-debugging-port=0',
  '--user-data-dir='+profile,'about:blank'
 ];
 if(process.platform==='linux'&&typeof process.getuid==='function'&&process.getuid()===0)
  chromeArgs.unshift('--no-sandbox');
 const child=spawn(browser,chromeArgs,{
  windowsHide:true,stdio:['ignore','ignore','pipe'],
  detached:process.platform!=='win32'
 });
 let stderr='',exit=null;
 child.stderr.on('data',d=>{stderr+=String(d);if(stderr.length>12000)stderr=stderr.slice(-12000);});
 child.on('error',e=>{exit='SPAWN_ERROR_'+e.message;});
 child.on('exit',(code,signal)=>{exit='EXIT_'+code+'_'+signal;});
 return {child,get stderr(){return stderr;},get exit(){return exit;}};
}
async function terminateBrowser(proc){
 if(!proc||!proc.child.pid)return;
 const pid=proc.child.pid;
 if(process.platform==='win32'){
  await new Promise(resolve=>{
   const t=spawn('taskkill.exe',['/PID',String(pid),'/T','/F'],{
    stdio:'ignore',windowsHide:true
   });
   const timer=setTimeout(()=>{t.kill();resolve();},3500);
   t.once('error',()=>{clearTimeout(timer);resolve();});
   t.once('close',()=>{clearTimeout(timer);resolve();});
  });
 }else{
  try{process.kill(-pid,'SIGKILL');}catch{
   try{proc.child.kill('SIGKILL');}catch{}
  }
 }
}
async function connectCDP(url){
 if(typeof WebSocket!=='function')fatal('NODE_NATIVE_WEBSOCKET_UNAVAILABLE',process.version);
 const ws=new WebSocket(url);
 let sequence=0,closed=false;
 const pending=new Map();
 const listeners=new Map();
 function emit(method,params){for(const fn of listeners.get(method)||[]){try{fn(params||{});}catch(e){console.error('CDP_LISTENER_ERROR: '+method+' / '+e.message);}}}
 function rejectAll(e){for(const [,p] of pending){clearTimeout(p.timer);p.reject(e);}pending.clear();}
 ws.addEventListener('message',event=>{
  let packet;
  try{packet=JSON.parse(String(event.data));}catch{return;}
  if(packet.method){emit(packet.method,packet.params);return;}
  const p=pending.get(packet.id);if(!p)return;
  pending.delete(packet.id);clearTimeout(p.timer);
  if(packet.error)p.reject(Error('CDP_'+p.method+': '+JSON.stringify(packet.error)));
  else p.resolve(packet.result||{});
 });
 ws.addEventListener('close',()=>{closed=true;rejectAll(Error('CDP_CONNECTION_CLOSED'));});
 ws.addEventListener('error',()=>{rejectAll(Error('CDP_WEBSOCKET_ERROR'));});
 await new Promise((resolve,reject)=>{
  const timer=setTimeout(()=>reject(Error('CDP_WS_CONNECT_TIMEOUT')),7000);
  ws.addEventListener('open',()=>{clearTimeout(timer);resolve();},{once:true});
  ws.addEventListener('error',()=>{clearTimeout(timer);reject(Error('CDP_WS_CONNECT_FAILED'));},{once:true});
 });
 return {
  async send(method,params={},timeout=5000){
   if(closed||ws.readyState!==WebSocket.OPEN)throw Error('CDP_NOT_CONNECTED: '+method);
   const id=++sequence;
   return new Promise((resolve,reject)=>{
    const timer=setTimeout(()=>{
     pending.delete(id);reject(Error('CDP_COMMAND_TIMEOUT: '+method));
    },timeout);
    pending.set(id,{resolve,reject,timer,method});
    ws.send(JSON.stringify({id,method,params}));
   });
  },
  on(method,handler){const handlers=listeners.get(method)||[];handlers.push(handler);listeners.set(method,handlers);},
  close(){closed=true;rejectAll(Error('CDP_CLOSED_BY_GATE'));ws.close();}
 };
}
async function main(){
 let proc=null,cdp=null,profile=null;
 try{
  gate('PRECHECK / LOCAL BUNDLE AND NODE');
  if(typeof WebSocket!=='function')fatal('NODE_WEBSOCKET_UNAVAILABLE',process.version);
  await new Promise((ok,bad)=>{server.once('error',bad);server.listen(0,'127.0.0.1',ok);});
  const port=server.address().port;
  const url='http://127.0.0.1:'+port+'/v1/ui/index.html';
  profile=fs.mkdtempSync(path.join(os.tmpdir(),'rf-runtime-chromium-cdp-'));
  gate('LIVE / CHROMIUM START / DEVTOOLS');
  console.log('CHROMIUM: '+browser+' / LOCALHOST_ONLY / NO ROUTER');
  proc=startBrowser(profile);
  const portFile=path.join(profile,'DevToolsActivePort');
  let devtoolsPort;
  try{
   devtoolsPort=await waitFor(()=>{
    if(proc.exit)fatal('CHROMIUM_EARLY_EXIT',proc.exit+' / '+errorExcerpt(proc.stderr));
    if(!fs.existsSync(portFile))return null;
    const value=Number(fs.readFileSync(portFile,'utf8').split(/\r?\n/)[0]);
    return Number.isInteger(value)&&value>0&&value<=65535?value:null;
   },16000,'CHROMIUM_DEVTOOLS_START');
  }catch(e){fatal(e.message,errorExcerpt(proc.stderr));}
  console.log('CHROMIUM_DEVTOOLS: PASS / 127.0.0.1');
  gate('LIVE / DEVTOOLS CONNECTION');
  const list=await waitFor(async()=>{
   try{
    const res=await fetch('http://127.0.0.1:'+devtoolsPort+'/json/list',{
     signal:AbortSignal.timeout(1200)
    });
    if(!res.ok)return null;
    const pages=await res.json();return pages.find(p=>p.type==='page'&&p.webSocketDebuggerUrl)||null;
   }catch{return null;}
  },7000,'CHROMIUM_PAGE_DISCOVERY');
  cdp=await connectCDP(list.webSocketDebuggerUrl);
  const cdplog={requests:[],responses:[],failed:[],exceptions:[],consoleErrors:[],paused:[],lifecycle:[],contexts:0};
  function cdplogExcerpt(){
   const short=list=>JSON.stringify(list.slice(-8)).slice(-2400);
   return 'RESOURCES='+JSON.stringify(resources)+
    ' / CDP_REQUESTS='+short(cdplog.requests)+
    ' / CDP_RESPONSES='+short(cdplog.responses)+
    ' / CDP_FAILED='+short(cdplog.failed)+
    ' / LIFECYCLE='+short(cdplog.lifecycle)+
    ' / CONTEXTS='+cdplog.contexts+
    ' / JS_ERRORS='+short(cdplog.exceptions)+
    ' / CONSOLE_ERRORS='+short(cdplog.consoleErrors)+
    ' / DEBUGGER_PAUSED='+short(cdplog.paused)+
    ' / THIRD_PARTY_ADGUARD='+cdplog.responses.some(r=>/local\.adguard\.org/i.test(String(r.url)))+
    ' / SERVER_REQUESTS='+short(requests);
  }
  cdp.on('Network.requestWillBeSent',p=>cdplog.requests.push({url:p.request?.url,method:p.request?.method,type:p.type}));
  cdp.on('Network.responseReceived',p=>cdplog.responses.push({url:p.response?.url,status:p.response?.status,type:p.type}));
  cdp.on('Network.loadingFailed',p=>cdplog.failed.push({error:p.errorText,type:p.type,canceled:p.canceled}));
  cdp.on('Runtime.exceptionThrown',p=>{
   const x=p.exceptionDetails||{};
   cdplog.exceptions.push({message:x.exception?.description||x.exception?.value||x.text||'exception',
    url:x.url||'',line:x.lineNumber,column:x.columnNumber,
    stack:(x.stackTrace?.callFrames||[]).slice(0,5).map(f=>({name:f.functionName,url:f.url,line:f.lineNumber}))});
  });
  cdp.on('Runtime.consoleAPICalled',p=>{
   if(['error','warning','assert'].includes(p.type))cdplog.consoleErrors.push({type:p.type,
    args:(p.args||[]).slice(0,3).map(a=>a.value||a.description||a.type),
    stack:(p.stackTrace?.callFrames||[]).slice(0,3).map(f=>({url:f.url,line:f.lineNumber,name:f.functionName}))});
  });
  cdp.on('Debugger.paused',p=>{
   cdplog.paused.push({reason:p.reason,callFrames:(p.callFrames||[]).slice(0,8).map(f=>({name:f.functionName,
    url:f.url,line:f.location?.lineNumber,column:f.location?.columnNumber}))});
  });
  cdp.on('Runtime.executionContextCreated',()=>{cdplog.contexts++;});
  cdp.on('Page.domContentEventFired',()=>cdplog.lifecycle.push('DOMContentLoaded'));
  cdp.on('Page.loadEventFired',()=>cdplog.lifecycle.push('load'));
  await cdp.send('Page.enable');
  await cdp.send('Runtime.enable');
  await cdp.send('Network.enable');
  await cdp.send('Debugger.enable');
  console.log('CHROMIUM_CDP: PASS');
  gate('GATES / CDP EVALUATE ABOUT:BLANK PRE-FLIGHT');
  try{
   const probe=await cdp.send('Runtime.evaluate',{expression:'1+1',returnByValue:true,awaitPromise:false},8000);
   if(probe.result?.value!==2)fatal('CDP_ABOUT_BLANK_EVALUATE_BAD_RESULT',JSON.stringify(probe).slice(0,800));
   console.log('CDP_PRENAV_EVALUATE: PASS / 2');
  }catch(e){fatal('CDP_PRENAV_EVALUATE_FAILED',e.message+' / '+cdplogExcerpt());}
  gate('LIVE / NAVIGATE TEST-ONLY SVELTE COMPONENT HARNESS');
  const navigation=await cdp.send('Page.navigate',{url},7000);
  if(navigation.errorText)fatal('CHROMIUM_NAVIGATION_COMMAND_FAIL',navigation.errorText);
  console.log('NAVIGATION_COMMAND: PASS / '+url+' / AWAITING DOCUMENT+JS+CSS');
  gate('LIVE / DOCUMENT + BUNDLE + GATE RESOURCE LOADING');
  const resourceDeadline=Date.now()+25000;
  let resourceHeartbeat=0;
  while(Date.now()<resourceDeadline){
   if(proc.exit)fatal('CHROMIUM_EXIT_BEFORE_RESOURCES',proc.exit+' / '+errorExcerpt(proc.stderr));
   if(resources.html>0&&resources.css>0&&resources.bundle>0&&resources.gate>0){
    console.log('HTTP_RESOURCES: PASS / '+JSON.stringify(resources));break;
   }
   if(Date.now()-resourceHeartbeat>=3000){
    console.log('RESOURCE_HEARTBEAT: '+cdplogExcerpt());
    resourceHeartbeat=Date.now();
   }
   await sleep(200);
  }
  if(!resources.bundle||!resources.gate){
   const stalled=resources.html&&resources.css&&!resources.bundle&&!resources.gate;
   fatal(stalled?'HTML_PARSER_OR_STYLESHEET_STALL':'CHROMIUM_RESOURCE_LOADING_STALL',cdplogExcerpt()+' / CHROME_STDERR='+errorExcerpt(proc.stderr));
  }
  gate('LIVE / COMPONENT MOUNT-UPDATE-UNMOUNT + SECURITY');
  const deadline=Date.now()+45000;
  let lastProgress=0,lastPhase='',lastStatus=0;
  while(Date.now()<deadline){
   if(proc.exit)fatal('CHROMIUM_EXIT_DURING_GATE',proc.exit+' / '+errorExcerpt(proc.stderr));
   let snapshot;
   try{
    const response=await cdp.send('Runtime.evaluate',{
     expression:`(()=>({ready:document.readyState, phase:window.__rfBrowserGatePhase||'PREBOOT',progress:window.__rfBrowserGateProgress||[],result:(()=>{const el=document.getElementById('RF_BROWSER_GATE_RESULT');return el?{state:el.dataset.state,text:el.textContent}:null})()}))()`,
     returnByValue:true,awaitPromise:false
    },8500);
    snapshot=response.result?.value;
    if(response.exceptionDetails)throw Error('CDP_EVALUATION_EXCEPTION: '+JSON.stringify(response.exceptionDetails).slice(0,1200));
   }catch(e){
    if(/context.*destroyed|Cannot find context|Inspected target navigated/i.test(e.message)){await sleep(200);continue;}
    if(/CDP_COMMAND_TIMEOUT: Runtime\.evaluate/.test(e.message)){
     console.error('CDP_MAIN_THREAD_STALL: DETECTED / REQUESTING DEBUGGER STACK');
     try{await cdp.send('Debugger.pause',{},2600);await sleep(500);}catch(x){console.error('DEBUGGER_PAUSE: UNAVAILABLE / '+x.message);}
     console.error('CDP_JAVASCRIPT_DIAGNOSTICS: '+cdplogExcerpt());
    }
    fatal('CDP_RUNTIME_EVALUATE_AFTER_RESOURCES_FAILED',e.message+' / '+cdplogExcerpt()+' / CHROME_STDERR='+errorExcerpt(proc.stderr));
   }
   if(snapshot){
    const progress=Array.isArray(snapshot.progress)?snapshot.progress:[];
    for(const line of progress.slice(lastProgress))console.log(line);
    lastProgress=progress.length;
    if(snapshot.phase!==lastPhase){lastPhase=snapshot.phase;console.log('BROWSER_STAGE: '+lastPhase);}
    if(snapshot.result){
     console.log('=== CHROMIUM DOM VERDICT ===');
     console.log(snapshot.result.text);
     if(snapshot.result.state!=='PASS')fatal('REAL_CHROMIUM_PARITY_FAILED');
     if(unexpectedMutation!==0)fatal('UNEXPECTED_API_MUTATION',String(unexpectedMutation));
     if(puts!==0)fatal('UNEXPECTED_MUTATING_REQUESTS',String(puts));
     gate('RESULT');
     console.log('BROWSER_PARITY: PASS');
     console.log('MOCK_API_MUTATION_GUARD: PASS');
     console.log('SVELTE_MOUNT_UPDATE_UNMOUNT: PASS');
     console.log('STALE_AFTER_UNMOUNT: PASS');
     return;
    }
    if(Date.now()-lastStatus>=3500){
     console.log('HEARTBEAT: READY='+snapshot.ready+' PHASE='+snapshot.phase+' GET/PUT='+requests.length+'/'+puts+' RESOURCES='+JSON.stringify(resources));
     lastStatus=Date.now();
    }
   }
   await sleep(250);
  }
  fatal('BROWSER_DOM_VERDICT_TIMEOUT', 'last_phase='+lastPhase+' / resources='+JSON.stringify(resources)+' / requests='+JSON.stringify(requests.slice(-5)).slice(0,1400)+' / '+cdplogExcerpt()+' / chrome_stderr='+errorExcerpt(proc.stderr));
 } finally{
  if(cdp)try{cdp.close();}catch{}
  await terminateBrowser(proc);
  if(releaseConfig){const release=releaseConfig;releaseConfig=null;release();}
  await closeServer();
  if(profile){
   try{
    fs.rmSync(profile,{recursive:true,force:true,maxRetries:5,retryDelay:200});
    console.log('ISOLATED_CHROME_PROFILE_CLEANUP: PASS');
   }catch(e){console.error('CHROME_PROFILE_CLEANUP_WARNING: '+profile+' / '+e.message);}
  }
 }
}
main().catch(e=>{
 console.error('=== FIRST REAL FAIL / STOP ===');
 console.error('STAGE: '+stage);
 console.error('BROWSER_PARITY: FAIL / '+e.message);
 process.exitCode=1;
});
