import fs from 'node:fs';
import assert from 'node:assert/strict';
import net from 'node:net';
const load=n=>JSON.parse(fs.readFileSync(new URL(n,import.meta.url),'utf8'));
const schema=load('schema.json'), golden=load('golden.json');
const fail=(p,m)=>{throw Error(`${p}: ${m}`)};
// Dependency-free verifier for the exact subset used by this candidate schema.
// Not a general-purpose JSON Schema implementation; unsupported keywords fail closed.
const keys=new Set(['$schema','$id','$defs','$ref','type','properties','required','additionalProperties','items','minItems','maxItems','minLength','maxLength','pattern','format','minimum','maximum','enum','const','anyOf']);
function check(s,v,p='$'){
 for(const k of Object.keys(s)) if(!keys.has(k)) fail(p,'unsupported schema keyword '+k);
 if(s.$ref){const d=schema.$defs[s.$ref.split('/').at(-1)];if(!d)fail(p,'unknown ref');return check(d,v,p);}
 if(s.anyOf){if(!s.anyOf.some(a=>{try{check(a,v,p);return true}catch{return false}}))fail(p,'anyOf');return;}
 if('const'in s&&v!==s.const)fail(p,'const');
 if(s.enum&&!s.enum.includes(v))fail(p,'enum');
 if(s.type==='object'&&(!v||typeof v!=='object'||Array.isArray(v)))fail(p,'object');
 if(s.type==='array'&&!Array.isArray(v))fail(p,'array');
 if(s.type==='null'&&v!==null)fail(p,'null');
 if(s.type==='string'&&typeof v!=='string')fail(p,'string');
 if(s.type==='boolean'&&typeof v!=='boolean')fail(p,'boolean');
 if(s.type==='number'&&!(typeof v==='number'&&Number.isFinite(v)))fail(p,'number');
 if(s.type==='integer'&&!Number.isSafeInteger(v))fail(p,'integer');
 if(s.type==='object'){
  for(const k of s.required||[])if(!Object.hasOwn(v,k))fail(p,'missing '+k);
  for(const k of Object.keys(v)){if(!Object.hasOwn(s.properties,k))fail(p,'unknown '+k);check(s.properties[k],v[k],p+'.'+k);}
 }
 if(s.type==='array'){if(v.length<(s.minItems||0)||v.length>(s.maxItems??Infinity))fail(p,'array size');v.forEach((a,i)=>check(s.items,a,p+'['+i+']'));}
 if(s.type==='string'){
  const len=[...v].length;if(len<(s.minLength||0)||len>(s.maxLength??Infinity))fail(p,'string length');
  if(s.pattern&&!new RegExp(s.pattern).test(v))fail(p,'pattern');
  if(s.format==='date-time'&&(!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(v)||!Number.isFinite(Date.parse(v))||new Date(v).toISOString().slice(0,19)!==v.slice(0,19)))fail(p,'date-time');
 }
 if(typeof v==='number'&&(v<(s.minimum??-Infinity)||v>(s.maximum??Infinity)))fail(p,'range');
}
function unique(xs,key){let set=new Set(xs.map(x=>x[key]));assert.equal(set.size,xs.length,'duplicate '+key);return set;}
function semantic(f){
 const c=f.catalog, d=unique(c.devices,'id'),con=unique(c.connections,'id'),pol=unique(c.policyObjects,'id');
 unique(c.connections.flatMap(x=>x.endpoints),'id');unique(c.checkPolicies,'connectionId');unique(f.snapshots,'snapshotId');unique(f.events,'id');
 const byId=new Map(c.connections.map(x=>[x.id,x]));
 for(const x of c.connections){assert(d.has(x.deviceId),'unknown device');for(const e of x.endpoints){
  assert((!e.host.includes('%')&&net.isIP(e.host))||/^(?=.{1,253}$)(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*\.?$/.test(e.host),'invalid host');
  assert(e.scheme==='icmp'?e.port===undefined:Number.isInteger(e.port),'port/protocol');
  assert(e.scheme===(x.protocol==='sftp'?'ssh':x.protocol),'endpoint protocol');
  if(x.transport==='reverse-proxy')assert(['http','https'].includes(e.scheme),'proxy protocol');
  if(e.publicPath!==undefined){assert(['http','https'].includes(e.scheme)&&!e.publicPath.startsWith('//')&&!/[\\\s\x00-\x1f]/.test(e.publicPath),'invalid public path');}
 }}
 for(const x of c.checkPolicies)assert(con.has(x.connectionId),'unknown check connection');
 const groups=new Map(c.policyObjects.map(x=>[x.id,x]));
 const domain=s=>/^(?=.{1,253}$)(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*\.?$/.test(s);
 function ipbits(s){
  if(s.includes('%'))return null;const k=net.isIP(s);if(k===4)return [s.split('.').reduce((n,v)=>(n<<8n)|BigInt(v),0n),32];if(k!==6)return null;
  if(s.includes('.')){const i=s.lastIndexOf(':');const a=s.slice(i+1).split('.').map(Number);s=s.slice(0,i+1)+((a[0]<<8)|a[1]).toString(16)+':'+((a[2]<<8)|a[3]).toString(16)}
  const [left,right]=s.split('::');const a=left?left.split(':'):[],b=right?right.split(':'):[];const parts=right===undefined?a:[...a,...Array(8-a.length-b.length).fill('0'),...b];return [parts.reduce((n,v)=>(n<<16n)|BigInt('0x'+v),0n),128];
 }
 function cidr(s){const a=s.split('/');if(a.length!==2||!/^\d+$/.test(a[1]))return false;const b=ipbits(a[0]),n=Number(a[1]);return !!b&&n<=b[1]&&String(n)===a[1]&&((b[0]>>BigInt(b[1]-n))<<BigInt(b[1]-n))===b[0]}
 function port(s){const m=/^(tcp|udp):([1-9][0-9]{0,4})(?:-([1-9][0-9]{0,4}))?$/.exec(s);return !!m&&+m[2]<=65535&&(!m[3]||(+m[3]>=+m[2]&&+m[3]<=65535))}
 for(const g of groups.values()){
  assert(g.name.trim()&&Buffer.byteLength(g.name.trim())<=160,'policy name');const seen=new Set();
  for(const m of g.members){const key=JSON.stringify(m);assert(!seen.has(key),'duplicate member');seen.add(key);let ok=false;switch(m.kind){case 'object':ok=groups.has(m.value)&&groups.get(m.value).type===g.type;break;case 'device':ok=g.type==='device'&&d.has(m.value);break;case 'ip':ok=g.type==='network'&&!!ipbits(m.value);break;case 'cidr':ok=g.type==='network'&&cidr(m.value);break;case 'domain':ok=g.type==='domain-group'&&domain(m.value.replace(/^\*\./,''));break;case 'list-ref':ok=g.type==='domain-group'&&/^[A-Za-z0-9_.-]{1,96}$/.test(m.value);break;case 'port':ok=g.type==='port-group'&&port(m.value);break;}assert(ok,'invalid typed member');}
 }
 for(const id of pol){const done=new Set(),active=new Set(),leaves=new Set();function visit(id){assert(!active.has(id),'policy cycle');if(done.has(id))return;active.add(id);for(const m of groups.get(id).members){if(m.kind==='object')visit(m.value);else leaves.add(JSON.stringify(m));assert(leaves.size<=4096,'expansion limit');}active.delete(id);done.add(id);}visit(id);}

 for(const x of f.snapshots){const y=byId.get(x.connectionId);assert(y&&y.deviceId===x.deviceId&&y.adapterId===x.adapterId,'snapshot binding');assert((x.latencyMs===null)===(x.latencyKind===null),'latency kind');for(const b of [x.metrics.memory,x.metrics.swap,...(x.metrics.filesystems||[])])if(b)assert(b.usedBytes<=b.totalBytes,'byte pair');}
 for(const x of f.events)if(x.deviceId!==null)assert(d.has(x.deviceId),'unknown event device');
}
function validate(x){check(schema,x);semantic(x);}
validate(golden);assert.equal(golden.snapshots[0].metrics.memory.usedBytes/golden.snapshots[0].metrics.memory.totalBytes*100,25);assert.equal(golden.snapshots[0].metrics.swap.totalBytes,0);assert.equal(golden.snapshots[0].metrics.network[0].rxBps,null);
for(const {name,document} of load('negative.json')){assert.throws(()=>validate(document),undefined,name);console.log('NEGATIVE PASS / '+name)}
console.log('GOLDEN PASS / JS / shared v1 schema + typed cross-reference checks');
