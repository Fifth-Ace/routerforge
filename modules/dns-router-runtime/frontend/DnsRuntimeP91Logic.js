/* RouterForge R11: guided manual DNS lookup, validation and CAS draft staging.
 * No fetch, timer or activation on import/construction. Backend is the only
 * certificate authority; UI never marks unverified DoQ or UDP as TLS-verified.
 */
const input = v=>String(v??'').trim();
const tlsTypes = new Set(['dot','doh']);
const protocols = new Set(['dot','doh','doq','udp']);
const domain = /^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/;
function validIP(value){
  if(/^[0-9.]+$/.test(value)){
    const octets=value.split('.');
    return octets.length===4&&octets.every(x=>x.length>0&&x.length<=3&&Number(x)>=0&&Number(x)<=255);
  }
  if(!/^[a-fA-F0-9:]+$/.test(value)||!value.includes(':'))return false;
  try{return new URL('http://['+value+']/').hostname.length>0;}catch{return false;}
}
function fail(message){throw Error(message);}
function canonical(fields){
  return JSON.stringify(['transport','endpoint','ip','sni'].map(k=>input(fields?.[k])));
}
export function parseDnsFields(fields,requireIP=false){
  const transport=input(fields?.transport);
  if(!protocols.has(transport))fail('Неизвестный тип DNS');
  const raw=input(fields?.endpoint).replace(/@dnsm$/,'');
  if(raw.includes('/../')||raw.includes('/./'))fail('Недопустимые сегменты URL DNS');
  const ip=input(fields?.ip);
  const sni=input(fields?.sni).toLowerCase();
  let server_name='',path='',port=transport==='doh'?'443':transport==='udp'?'53':'853';
  if(transport==='doh'){
    let url;
    try{url=new URL(/^https:\/\//i.test(raw)?raw:'https://'+raw);}catch{fail('Некорректный DoH URL');}
    if(url.protocol!=='https:'||url.username||url.password||url.search||url.hash||
       !url.hostname||url.pathname.startsWith('//')||url.pathname.includes('..')||
       (url.port&&url.port!=='443'))fail('Некорректный DoH URL или порт');
    server_name=url.hostname.toLowerCase();
    path=url.pathname==='/'?'/dns-query':url.pathname;
  }else if(transport==='dot'||transport==='doq'){
    let url;
    try{url=new URL(raw.startsWith('tls://')?raw:'tls://'+raw);}catch{fail('Укажи DNS-имя сервера');}
    if(url.protocol!=='tls:'||!url.hostname||(url.port&&url.port!=='853')||
      (url.pathname&&url.pathname!=='/')||url.search||url.hash||url.username||url.password)
      fail('Укажи DNS-имя сервера без URL-пути');
    server_name=url.hostname.toLowerCase();
  }else{
    if(!raw||raw.includes('://')||raw.includes('/')||raw.includes('@'))fail('Для UDP укажи IP или имя DNS-сервера');
    server_name=raw.toLowerCase();
  }
  if(!domain.test(server_name)||server_name.length>253||server_name.includes('..'))fail('Некорректное имя DNS');
  if(sni&&(!domain.test(sni)||sni.length>253||sni.includes('..')))fail('Некорректное имя SNI');
  if(ip&&(ip.length>45||!validIP(ip)))fail('Некорректный IP подключения');
  if(requireIP&&!ip)fail('Сначала нажми «Получить IP и сертификаты»');
  const address=ip?(ip.includes(':')?'['+ip+']':ip)+':'+port:'';
  return {transport,server_name,tls_name:sni||server_name,path,address,selected_ip:ip};
}
export function createP91Flow({lookup,readDraft,commit,notify,refresh,getFields,signal}){
  for(const fn of [lookup,readDraft,commit,notify,refresh,getFields,signal])
    if(typeof fn!=='function')fail('DNS_P91_DEPENDENCY_MISSING');
  let verified=null,busy=false,generation=0;
  function invalidate(){verified=null;generation++;signal({verified:null});}
  async function inspect(){
    if(busy)fail('Проверка DNS уже выполняется');
    const initial=getFields(),key=canonical(initial),requestId=++generation;
    invalidate();
    const fields=parseDnsFields(initial,false);
    busy=true;
    try{
      const result=await lookup({transport:fields.transport,server_name:fields.server_name,
        tls_name:fields.tls_name,...(fields.selected_ip?{preferred_ip:fields.selected_ip}:{})});
      if(requestId!==generation-1||key!==canonical(getFields()))fail('Адрес изменился во время проверки. Повтори запрос');
      if(!result||!result.address||!result.selected_ip||!Array.isArray(result.ips)||!result.ips.length)
        fail('IP не найден');
      if(!result.ips.includes(result.selected_ip))fail('Сервер вернул IP вне списка результатов');
      const encrypted=tlsTypes.has(fields.transport);
      if(encrypted&&(!result.certificate?.verified||!result.certificate.pem))
        fail('TLS-сертификат не подтверждён');
      const next={...initial,ip:input(initial.ip)||result.selected_ip};
      if(parseDnsFields(next,true).address!==result.address)fail('IP/порт результата не совпадают с запрошенным DNS');
      if(key!==canonical(getFields()))fail('Адрес изменился во время проверки. Повтори запрос');
      verified={key:canonical(next),fields:parseDnsFields(next,true),result,encrypted};
      signal({verified:{ip:next.ip,ips:result.ips,encrypted,certificate:encrypted?result.certificate:null}});
      notify(encrypted?'IP найден, TLS-сертификат проверен · DNS в черновик ещё не добавлен':
        'IP найден. Шифрование для выбранного транспорта не подтверждалось');
      return verified;
    }catch(error){verified=null;signal({verified:null});throw error;}
    finally{busy=false;}
  }
  function store(){
    if(busy)fail('Проверка DNS ещё выполняется');
    if(!verified||verified.key!==canonical(getFields()))fail('Сначала получи IP и сертификат для текущего адреса');
    const state=readDraft();
    if(!state||!state.revision||!Number.isSafeInteger(state.generation)||
       !state.config||!Array.isArray(state.config.upstreams))fail('Черновик DNS ещё не загружен');
    const {fields,result,encrypted}=verified;
    const config=structuredClone(state.config);
    if(config.upstreams.some(x=>x.transport===fields.transport&&x.address===fields.address&&
        x.server_name===fields.server_name&&(x.path||'')===fields.path))fail('Такой DNS уже добавлен');
    let n=1;while(config.upstreams.some(x=>x.id==='manual-'+n))n++;
    if(n>100000)fail('Слишком много DNS-серверов');
    const server={id:'manual-'+n,transport:fields.transport,address:fields.address,
      ...(fields.transport!=='udp'?{server_name:fields.tls_name}:{}),
      ...(fields.path?{path:fields.path}:{})};
    if(encrypted&&result.certificate?.verified)server.certificate_pem=result.certificate.pem;
    config.upstreams.push(server);
    commit('edited',{previousRevision:state.revision,generation:state.generation,config});
    invalidate();
    refresh();
    notify('DNS-пул добавлен только в черновик. Работающий DNS не изменён');
    return server;
  }
  return {inspect,store,invalidate,isBusy:()=>busy};
}
