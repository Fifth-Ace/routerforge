'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {JSDOM,VirtualConsole}=require('jsdom');
const root=path.resolve(__dirname,'../../dns-router-runtime/frontend');
const wait=()=>new Promise(r=>setTimeout(r,20));
test('shipped UI applies independent DNS settings and connects a client without P10 zone requests',async()=>{
  let config={schema_version:1,upstreams:[{id:'one',transport:'udp',address:'9.9.9.9:53'},{id:'two',transport:'doh',address:'1.1.1.1:443',server_name:'cloudflare-dns.com',path:'/dns-query'}],rules:[{id:'r',domain:'example.com',match:'suffix',upstream:'one',priority:1}]};
  let revision='sha256:old',active=false,connection={state:'disconnected'};const calls=[],errors=[];
  const vc=new VirtualConsole();vc.on('jsdomError',e=>errors.push(e.message));
  const dom=new JSDOM(fs.readFileSync(path.join(root,'index.html'),'utf8'),{url:'http://routerforge.test/ui/index.html',runScripts:'outside-only',pretendToBeVisual:true,virtualConsole:vc});
  const w=dom.window;w.confirm=()=>true;w.prompt=()=>null;
  w.fetch=async(url,opts={})=>{
    const p=new URL(url,w.location.href).pathname.replace(/^\//,'');calls.push([p,opts]);let body={};let status=200;
    if(p==='config'){
      if(opts.method==='PUT'){assert.equal(opts.headers['If-Match'],'"'+revision+'"');config=JSON.parse(opts.body);revision='sha256:new';}
      body={config:structuredClone(config),revision};
    }else if(p==='service/status')body={mode:'independent-dns',active,applied_revision:active?revision:'',listen:config.service?.listen,default_upstream:config.service?.default_upstream,requests:0,cache_hits:0,connection};
    else if(p==='service/traces')body={traces:[]};
    else if(p==='service/control'){const action=JSON.parse(opts.body);assert.equal(opts.headers['X-RouterForge-Action'],'service-control');active=action.operation!=='stop';body={ok:true};}
    else if(p==='service/connect'){const action=JSON.parse(opts.body);assert.equal(action.mac,'02:11:22:33:44:55');connection={state:'connected',mac:action.mac};body={ok:true};}
    else if(p==='service/disconnect'){connection={state:'disconnected'};body={ok:true};}
    else if(p==='keenetic/native-dns')body={available:true,instances:[{name:'System',valid:true,endpoints:[]}]};
    else if(p==='keenetic/native-policies')body={available:true,policies:[]};
    else if(p==='shadow/status')body={lifecycle:{active:false}};
    else if(p==='keenetic/native-bridge/status')body={bindings:[]};
    else if(p==='keenetic/managed-ownership/status')body={state:'disabled',owned_zones:[]};
    return {ok:status===200,status,json:async()=>structuredClone(body)};
  };
  try{
    w.eval(fs.readFileSync(path.join(root,'app.js'),'utf8'));
    for(let i=0;i<30&&w.document.getElementById('drLiteDefault').options.length<2;i++)await wait();
    assert.equal(w.document.getElementById('drLitePanel').hidden,false);
    const select=w.document.getElementById('drLiteDefault');select.value='two';select.dispatchEvent(new w.Event('change'));
    const cache=w.document.getElementById('drLiteCache');cache.value='2';cache.dispatchEvent(new w.Event('change'));
    w.document.getElementById('drLiteApply').click();
    for(let i=0;i<30&&!active;i++)await wait();assert.equal(active,true,w.document.getElementById('feedback').textContent);
    assert.equal(config.service.default_upstream,'two');assert.equal(config.service.cache_bytes,2*1024*1024);
    await wait();w.document.getElementById('drLiteMAC').value='02:11:22:33:44:55';w.document.getElementById('drLiteConnect').click();
    for(let i=0;i<30&&(connection.state!=='connected'||w.document.getElementById('drLiteDisconnect').disabled);i++)await wait();assert.equal(connection.state,'connected');
    w.document.getElementById('drLiteDisconnect').click();for(let i=0;i<30&&connection.state!=='disconnected';i++)await wait();assert.equal(connection.state,'disconnected');
    assert.equal(calls.some(([p])=>p.startsWith('keenetic/durable/')),false);
    await wait(); // Allow the disconnect handler's final status refresh to finish.
    assert.deepEqual(errors,[]);
  }finally{w.close();}
});
