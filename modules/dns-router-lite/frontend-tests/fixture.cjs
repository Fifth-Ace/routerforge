'use strict';
const a=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {JSDOM,VirtualConsole}=require('jsdom');
const root=path.resolve(__dirname,'../frontend');
const html=fs.readFileSync(path.join(root,'index.html'),'utf8');
const js=fs.readFileSync(path.join(root,'build/svelte-mode.js'),'utf8');
const tick=()=>new Promise(r=>setTimeout(r,25));
const until=async(p)=>{for(let i=0;i<45;i++){if(p())return;await tick();}throw Error('UI wait exceeded');};
function fixture(options={}){
  let config={schema_version:1,upstreams:[{id:'old',transport:'udp',address:'9.9.9.9:53'},{id:'cf',transport:'doh',address:'1.1.1.1:443',server_name:'cloudflare-dns.com',path:'/dns-query'}],rules:[{id:'r1',domain:'example.com',match:'suffix',upstream:'old',priority:100},{id:'r2',domain:'other.net',match:'suffix',upstream:'cf',priority:100}]};
  if(options.advanced)config.advanced=structuredClone(options.advanced);
  if(options.dnsGroups)config.dns_groups=structuredClone(options.dnsGroups);
  let applied=options.active?structuredClone(config):null;
  let revision='sha256:initial',active=Boolean(options.active),gateEnabled=false,gateOverride=null,connection={state:'disconnected'},requests=Number(options.requests||0),denied=Number(options.denied||0),aclDenied=Number(options.aclDenied||0),protectionDenied=Number(options.protectionDenied||0);const calls=[],errors=[],pendingDevices=[];
  const vc=new VirtualConsole();vc.on('jsdomError',e=>errors.push(e.message));
  const dom=new JSDOM(html,{url:'https://routerforge.test/ui/index.html',runScripts:'outside-only',pretendToBeVisual:true,virtualConsole:vc});
  const w=dom.window;if(options.mode)w.localStorage.setItem('routerforge.dnsRouter.mode.v1',options.mode);w.confirm=typeof options.confirm==='function'?options.confirm:()=>true;w.scrollTo=()=>{};
  w.fetch=async(url,opts={})=>{
    const p=new URL(url,w.location.href).pathname.replace(/^\/?ui\/\.\.\//,'').replace(/^\//,'');calls.push({path:p,opts});
    let status=200,body={};
    if(p==='config'){
      if(opts.method==='PUT'){a.equal(opts.headers['If-Match'],'"'+revision+'"');config=JSON.parse(opts.body);revision='sha256:next';}
      body={config:structuredClone(config),revision};
    }else if(p==='service/status')body={mode:'independent-dns',active,applied_revision:active?revision:'',listen:config.service?.listen,default_upstream:config.service?.default_upstream,requests,denied_requests:denied,acl_denied_requests:aclDenied,protection_denied_requests:protectionDenied,cache_hits:0,failures:0,device_policies_active:Boolean(active&&applied?.advanced?.devices?.some(d=>d.target)),cache_mode:active&&applied?.advanced?.devices?.some(d=>d.target)?'disabled_for_device_policy_isolation':'shared_global_policy',connection,system_gate:gateOverride||{state:gateEnabled?'ACTIVE':'STOCK',active:gateEnabled,desired:gateEnabled,lease_seconds:60}};
    else if(p==='service/traces')body={traces:active?(options.traces||[{name:'example.com',time:'2026-10-31T12:00:00Z',assigned:'old',used:'old',transport:'udp',rcode:0,success:true,latency_ms:5,type:1,cache:false}]):[]};
    else if(p==='certificates/lookup'){a.equal(opts.headers['X-RouterForge-Action'],'certificate-lookup');const q=JSON.parse(opts.body);body={address:'8.8.8.8:443',selected_ip:'8.8.8.8',ips:['8.8.8.8'],certificate:{verified:true,pem:'-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n',subject:'CN=dns.google',issuer:'Trusted CA',not_after:'2027-10-31',fingerprint_sha256:'ab',server_name:q.server_name}};}
    else if(p==='service/control'){a.equal(opts.headers['X-RouterForge-Action'],'service-control');const q=JSON.parse(opts.body);active=q.operation!=='stop';applied=active?structuredClone(config):null;body={ok:true};}
    else if(p==='service/gate/control'){const q=JSON.parse(opts.body);if(q.operation==='enable')gateEnabled=true;else if(q.operation==='disable'){gateEnabled=false;gateOverride=null;}else if(q.operation==='retry'){gateEnabled=true;gateOverride=null;}body={ok:true};}
    else if(p==='service/upstreams/inspect'){const q=JSON.parse(opts.body);a.equal(q.operation,'inspect');body={status:'UP',dns_available:true,address:q.upstream.address,transport:q.upstream.transport,probe_latency_ms:24,tls_status:'VERIFIED_BY_TRANSPORT',saved:false,applied:false};}
    else if(p==='service/upstreams/health')body={source:'routerforge-direct',upstreams:[{id:'old',status:'UP',probe_latency_ms:14,last_checked:'2026-11-01T12:00:00Z',requests_5m:9,failures_5m:0,p95_latency_5m_ms:22},{id:'cf',status:'DOWN',probe_latency_ms:3000,last_checked:'2026-11-01T12:00:00Z',requests_5m:2,failures_5m:2,last_error:'timeout'}]};
    else if(p==='service/upstreams/check')body={source:'routerforge-direct',upstreams:[{id:'old',status:'UP',probe_latency_ms:14,last_checked:'2026-11-01T12:00:00Z',requests_5m:9,failures_5m:0,p95_latency_5m_ms:22},{id:'cf',status:'DOWN',probe_latency_ms:3000,last_checked:'2026-11-01T12:00:00Z',requests_5m:2,failures_5m:2,last_error:'timeout'}]};
    else if(p==='service/devices'){if(options.deferDevices)await new Promise(resolve=>pendingDevices.push(resolve));body={devices:[{ip:'192.168.10.22',mac:'02:11:22:33:44:55',name:'Laptop',trusted:true,state:'REACHABLE'}],issues:[],applied:applied?.advanced||null,applied_revision:active?'sha256:applied':''};}
    else if(p==='service/devices/evaluate'){const q=JSON.parse(opts.body);body={scope:'CONFIGURATION_ONLY',network_verified:false,identity_verified:false,source:q.use_applied?'applied':'saved_draft',decision:{layer:'device'},allowed_chain:[]};}
    else if(p==='service/verify')body={state:'PASS',results:[{state:'PASS'}]};
    else if(p==='service/connect'){connection={state:'connected',mac:JSON.parse(opts.body).mac};body={ok:true};}
    else if(p==='service/disconnect'){connection={state:'disconnected'};body={ok:true};}
    else if(p==='service/migrate-native')body={ok:true};
    else status=404;
    return {ok:status===200,status,headers:{get:()=> 'application/json'},json:async()=>structuredClone(body)};
  };
  const originalClose=w.close.bind(w);
  let closed=false;
  w.close=()=>{if(closed)return;closed=true;w.dispatchEvent(new w.Event('pagehide'));originalClose();};
  w.eval(js);return {w,dom,calls,errors,getConfig:()=>config,getActive:()=>active,setConnection:v=>{connection=v;},setRequests:v=>{requests=v;},setDenied:v=>{denied=v;},setAclDenied:v=>{aclDenied=v;},setProtectionDenied:v=>{protectionDenied=v;},setGate:v=>{gateOverride=v;},releaseDevices:()=>pendingDevices.splice(0).forEach(resolve=>resolve())};
}


module.exports={fixture,tick,until,html,js};
