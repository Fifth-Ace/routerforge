// Pure NFQWS Strategy Import projection and inactive Config Library candidate.
// No IO, DOM or active-configuration mutation belongs in this module.
export const IMPORT_CONFIRM='ROUTERFORGE_V2_STRATEGY_IMPORT';
export const CONFIG_CREATE_CONFIRM='NFQWS_CONFIG_LIBRARY_CREATE';
export const IMPORT_LIMIT=262144;
const STRATEGY_SLOTS=['NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET'];
const REQUIRED=[...STRATEGY_SLOTS,'NFQWS_ARGS_CUSTOM','TCP_PORTS','UDP_PORTS'];

export function normalizeImportedResult(body){
  if(!body||typeof body!=='object'||Array.isArray(body))throw Error('NFQWS_IMPORT_RESPONSE_INVALID');
  const arrays=['commands','profiles','transforms','warnings','deps'];
  for(const key of arrays)if(body[key]!==undefined&&!Array.isArray(body[key]))throw Error(`NFQWS_IMPORT_${key.toUpperCase()}_INVALID`);
  return {type:String(body.type||'unknown'),commands:body.commands||[],profiles:body.profiles||[],tcp:String(body.tcp||''),udp:String(body.udp||''),transforms:(body.transforms||[]).map(x=>[String(x?.kind||''),String(x?.text||'')]),warnings:(body.warnings||[]).map(x=>[String(x?.level||'warn'),String(x?.text||'')]),deps:(body.deps||[]).map(x=>[String(x?.kind||''),String(x?.path||'')]),preview:String(body.preview||''),ready:body.ready===true,log:body.log||null};
}
export function normalizeImportName(raw){
  let name=String(raw||'').trim();if(!name)return'';if(!/\.conf$/i.test(name))name+='.conf';
  return /^[A-Za-z0-9][A-Za-z0-9._-]*\.conf$/i.test(name)&&name.length<=128?name:'';
}
export function importedArgTokens(value){
  const raw=String(value||'').match(/(?:[^\s"'\\]+|"(?:\\.|[^"])*"|'(?:\\.|[^'])*')+/g)||[];
  return raw.map(x=>{let y=x;if((y[0]==='"'&&y.at(-1)==='"')||(y[0]==="'"&&y.at(-1)==="'"))y=y.slice(1,-1);return y;});
}
export function confirmedStrategyImport(response,requestedCount){
  if(!response||typeof response!=='object'||Array.isArray(response)||response.ok!==true||response.production_mutation!==false||response.source!=='import')throw Error('NFQWS_IMPORT_ACK_INVALID');
  const count=Number(requestedCount),added=response.added,existing=response.existing,total=response.count;
  if(!Number.isInteger(count)||count<1||count>32||!Number.isInteger(added)||added<0||!Number.isInteger(existing)||existing<0||added+existing>count||!Number.isInteger(total)||total<added+existing)throw Error('NFQWS_IMPORT_COUNTS_INVALID');
  if(!Array.isArray(response.strategies)||response.strategies.length!==added+existing||response.strategies.some(item=>!item||typeof item.id!=='string'||!item.id))throw Error('NFQWS_IMPORT_ITEMS_INVALID');
  return {added,existing,count:total};
}
export function importStrategyItems(result){
  if(!result?.ready||!Array.isArray(result.profiles)||!result.profiles.length)throw Error('NFQWS_IMPORT_NOT_READY');
  const items=result.profiles.map((profile,i)=>({name:`Импортированный профиль ${i+1}`,args:importedArgTokens(profile)}));
  if(items.some(x=>!x.args.length))throw Error('NFQWS_IMPORT_EMPTY_PROFILE');return items;
}
function scan(config,key){
  const re=new RegExp(`^([\\t ]*)${key}\\s*=\\s*`,'m'),m=re.exec(config);if(!m)return null;
  const start=m.index;let i=start+m[0].length,quote='';if(config[i]==='"'||config[i]==="'"){quote=config[i];i++}
  const valueStart=i;let escaped=false;
  if(quote){for(;i<config.length;i++){const c=config[i];if(c===quote&&!escaped)break;escaped=c==='\\'&&!escaped;if(c!=='\\')escaped=false}if(i>=config.length)return null;return{start,end:i+1,quote,indent:m[1],raw:config.slice(valueStart,i)}}
  for(;i<config.length;i++){const c=config[i];if((c==='\n'||c==='\r')&&!escaped)break;escaped=c==='\\'&&!escaped;if(c!=='\\')escaped=false}
  return{start,end:i,quote:'',indent:m[1],raw:config.slice(valueStart,i)};
}
function formatted(key,value,a){
  const quote=a.quote||((String(value).includes(' ')||String(value).includes('\n')||String(value)==='')?'"':'');
  const logical=(quote==='"'?String(value).replace(/\\/g,'\\\\').replace(/"/g,'\\"'):quote==="'"?String(value).replace(/'/g,"'\\''"):String(value)).replace(/\r\n/g,'\n');
  if(!logical.includes('\n'))return `${a.indent}${key}=${quote}${logical}${quote}`;
  const continuation=a.indent+' '.repeat(key.length+2);
  return `${a.indent}${key}=${quote}${logical.split('\n').map((line,i)=>i===0?line.trim():line.trim()?continuation+line.trim():'').join('\n')}${quote}`;
}
function replace(config,key,value){const a=scan(config,key);if(!a)return config;return config.slice(0,a.start)+formatted(key,value,a)+config.slice(a.end)}
export function candidateFromImport(result,active){
  if(!result?.ready)return{ok:false,error:'Требуется READY preview.'};
  if(typeof active!=='string'||!active.trim())return{ok:false,error:'Active canonical config недоступен.'};
  const missing=REQUIRED.filter(key=>!scan(active,key));if(missing.length)return{ok:false,error:`Отсутствуют обязательные assignments: ${missing.join(', ')}`};
  const custom=(result.profiles||[]).map(x=>String(x||'').trim()).filter(Boolean).join('\n--new\n');
  if(!custom)return{ok:false,error:'Импорт не содержит NFQWS_ARGS_CUSTOM profiles.'};
  let cfg=active;for(const key of STRATEGY_SLOTS)cfg=replace(cfg,key,'');
  cfg=replace(cfg,'NFQWS_ARGS_CUSTOM',custom);
  if(result.tcp?.trim())cfg=replace(cfg,'TCP_PORTS',result.tcp.trim());
  if(result.udp?.trim())cfg=replace(cfg,'UDP_PORTS',result.udp.trim());
  return{ok:true,content:cfg};
}
export function importDiff(before,after){const a=String(before).split(/\r?\n/),b=String(after).split(/\r?\n/),out=[];for(let i=0;i<Math.max(a.length,b.length);i++){if(a[i]===b[i]){if(a[i]!==undefined)out.push({kind:'same',text:`  ${a[i]}`});continue}if(a[i]!==undefined)out.push({kind:'del',text:`- ${a[i]}`});if(b[i]!==undefined)out.push({kind:'add',text:`+ ${b[i]}`})}return out;}
export function defaultImportName(date=new Date()){const p=n=>String(n).padStart(2,'0');return `strategy-import-${date.getFullYear()}${p(date.getMonth()+1)}${p(date.getDate())}-${p(date.getHours())}${p(date.getMinutes())}.conf`}
