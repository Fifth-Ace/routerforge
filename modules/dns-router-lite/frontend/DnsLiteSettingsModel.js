/* DNS Lite native settings model. No DOM, network, or device side effects.
 * Svelte owns the editable fields; CAS/commit/apply remain explicit commands.
 */

export function createLiteSettingsFields() {
  return {
    defaultDNS:'', serviceListen:'192.168.10.1:55358', cacheSize:1,
    allowedNetworks:'127.0.0.0/8,192.168.10.0/24',
    p27RateLimit:0, p27RefuseAny:false, p27BogusIPs:'',
    p28Enabled:false, p28Mode:'sequential', p28Timeout:3000,
    p29Enabled:false, p29MinTTL:0, p29MaxTTL:0, p29Optimistic:false, p29Pending:'default', p29Concurrent:32,
    p30Enabled:false, p30ECSMode:'inherit', p30ECSAddress:'', p30DNSSECMode:'off',
    p31DNS64Enabled:false, p31DNS64Prefix:'64:ff9b::/96', p31NAT64Confirmed:false,
    p31ReverseEnabled:false, p31ReverseUpstream:'',
    p241IPEnabled:false, p241IPMode:'original', p243Scope:'all', p243Domains:'', p243ListIDs:[],
    p242HealthEnabled:false, p242HealthProtocol:'tcp', p242HealthPort:443,
    p242HealthInterval:60, p242HealthTimeout:1200, p242HealthFailures:2,
    p243Recoveries:2, p243MinDown:30, p243HTTPPath:'/',
    ipListOptions:[], reverseOptions:[]
  };
}

export const IP_MODES = Object.freeze([
  'original','round_robin','random','fastest','health_aware','adaptive','sticky','weighted','healthy_only'
]);
export const UPSTREAM_MODES = Object.freeze(['sequential','load_balance','parallel','fastest']);
export const HEALTH_PROTOCOLS = Object.freeze(['tcp','tls','http','https']);

export function parseLiteDomains(text) {
  if (typeof text !== 'string' || text.length > 256*1024) throw Error('Список слишком большой (максимум 256 КБ)');
  const names=[],seen=new Set();
  for (const line of text.split(/\r?\n/)) {
    const clean=line.split('#')[0].trim();
    if (!clean) continue;
    for (let raw of clean.split(/[\s,;]+/)) {
      if (!raw) continue;
      raw=raw.toLowerCase().replace(/\.$/,'');
      if (raw.length>253||!/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/.test(raw)) throw Error('Некорректный домен: '+raw);
      if (!seen.has(raw)){seen.add(raw);names.push(raw);}
      if (names.length>10000) throw Error('Список слишком большой: больше 10 000 доменов');
    }
  }
  return names;
}

export function hydrateLiteSettings(fields, config) {
  if (!fields || !config || !Array.isArray(config.upstreams) || !Array.isArray(config.lists || [])) {
    throw Error('DNS_LITE_SETTINGS_CONFIG_INVALID');
  }
  const d=createLiteSettingsFields(), svc=config.service||{};
  d.defaultDNS=svc.default_upstream||'';
  d.serviceListen=svc.listen || d.serviceListen;
  d.cacheSize=svc.cache_bytes==null?d.cacheSize:svc.cache_bytes/(1024*1024);
  d.allowedNetworks=(svc.allowed_networks||[]).join(',');
  const protection=svc.protection||{};
  d.p27RateLimit=protection.rate_limit_rps||0;d.p27RefuseAny=Boolean(protection.refuse_any);
  d.p27BogusIPs=(protection.bogus_nxdomain||[]).join('\n');
  const selection=svc.upstream_selection||{};
  d.p28Enabled=Boolean(selection.enabled);d.p28Mode=selection.mode||d.p28Mode;
  d.p28Timeout=selection.timeout_ms||d.p28Timeout;
  const perf=svc.performance||{};
  d.p29Enabled=Boolean(perf.enabled);d.p29MinTTL=perf.cache_min_ttl||0;d.p29MaxTTL=perf.cache_max_ttl||0;
  d.p29Optimistic=Boolean(perf.optimistic_cache);d.p29Pending=perf.pending_requests||'default';
  d.p29Concurrent=perf.max_concurrent||32;
  const privacy=svc.privacy||{};
  d.p30Enabled=Boolean(privacy.enabled);d.p30ECSMode=privacy.ecs_mode||d.p30ECSMode;
  d.p30ECSAddress=privacy.ecs_address||'';d.p30DNSSECMode=privacy.dnssec_mode||'off';
  const dns64=svc.dns64||{};
  d.p31DNS64Enabled=Boolean(dns64.enabled);d.p31DNS64Prefix=dns64.prefix||d.p31DNS64Prefix;
  d.p31NAT64Confirmed=Boolean(dns64.nat64_confirmed);
  const reverse=svc.reverse_dns||{};
  d.p31ReverseEnabled=Boolean(reverse.enabled);d.p31ReverseUpstream=reverse.upstream||'';
  const ip=svc.ip_intelligence||{},health=ip.health||{};
  d.p241IPEnabled=Boolean(ip.enabled);d.p241IPMode=ip.mode||d.p241IPMode;
  d.p243Scope=ip.scope||'all';d.p243Domains=(ip.domains||[]).join('\n');
  d.p243ListIDs=[...(ip.list_ids||[])];
  d.p242HealthEnabled=Boolean(health.enabled);d.p242HealthProtocol=health.protocol||d.p242HealthProtocol;
  d.p242HealthPort=health.port||443;d.p242HealthInterval=health.interval_seconds||60;
  d.p242HealthTimeout=health.timeout_ms||1200;d.p242HealthFailures=health.failure_threshold||2;
  d.p243Recoveries=health.recovery_threshold||2;
  d.p243MinDown=health.min_down_seconds??30;d.p243HTTPPath=health.http_path||'/';
  d.ipListOptions=(config.lists||[]).map(x=>({id:x.id,label:(x.name||x.id)+' · '+x.id}));
  d.reverseOptions=config.upstreams.filter(x=>x.transport!=='native').map(x=>({id:x.id,label:(x.server_name||x.address)+' · '+x.transport+' ['+x.id+']'}));
  Object.assign(fields,d);
  return fields;
}

export function refreshLiteSettingsOptions(fields, config) {
  const selected = fields.p243ListIDs;
  fields.ipListOptions=(config.lists||[]).map(x=>({id:x.id,label:(x.name||x.id)+' · '+x.id}));
  fields.reverseOptions=(config.upstreams||[]).filter(x=>x.transport!=='native').map(x=>({id:x.id,label:(x.server_name||x.address)+' · '+x.transport+' ['+x.id+']'}));
  // Preserve user selections while draft options are re-rendered. Invalid IDs
  // are refused when persisting, rather than silently changing another list.
  fields.p243ListIDs=selected;
}

function integer(value,id,min,max) {
  const n=Number(value);
  if (!Number.isInteger(n)||n<min||n>max) throw Error(id+': допустимо '+min+'..'+max);
  return n;
}
function member(value, options, id) {
  if (!options.includes(value)) throw Error(id+': неизвестный режим');
  return value;
}

export function buildLiteServicePrefs(f, config, requireDefault=true) {
  if (!f||!config||!Array.isArray(config.upstreams)||!Array.isArray(config.lists||[])) throw Error('DNS_LITE_SETTINGS_CONFIG_INVALID');
  const primary=String(f.defaultDNS||'');
  if (requireDefault && (!primary || !config.upstreams.some(u=>u.id===primary))) throw Error('Выбери DNS по умолчанию');
  if (primary && !config.upstreams.some(u=>u.id===primary)) throw Error('Выбранный DNS недоступен');
  const cache=integer(f.cacheSize,'cacheSize',0,8);
  const prefs={listen:String(f.serviceListen||'').trim(),default_upstream:primary,
    cache_bytes:cache*1024*1024,allowed_networks:String(f.allowedNetworks||'').split(',').map(s=>s.trim()).filter(Boolean)};
  if (!prefs.listen) throw Error('Адрес DNS-сервиса не задан');
  const rate=integer(f.p27RateLimit,'Защита DNS',0,2000);
  const bogus=String(f.p27BogusIPs||'').split(/[\r\n,]+/).map(s=>s.trim()).filter(Boolean);
  if (bogus.length>16) throw Error('Защита DNS: максимум 16 IP/CIDR');
  if (rate>0||f.p27RefuseAny||bogus.length) prefs.protection={rate_limit_rps:rate,refuse_any:Boolean(f.p27RefuseAny),bogus_nxdomain:bogus};
  if (f.p28Enabled) prefs.upstream_selection={enabled:true,mode:member(f.p28Mode,UPSTREAM_MODES,'p28Mode'),timeout_ms:integer(f.p28Timeout,'p28Timeout',250,5000)};
  if (f.p29Enabled) {
    prefs.performance={enabled:true,cache_min_ttl:integer(f.p29MinTTL,'p29MinTTL',0,3600),
      cache_max_ttl:integer(f.p29MaxTTL,'p29MaxTTL',0,86400),optimistic_cache:Boolean(f.p29Optimistic),
      pending_requests:member(f.p29Pending,['default','on','off'],'p29Pending'),
      max_concurrent:integer(f.p29Concurrent,'p29Concurrent',8,64)};
  }
  if (f.p30Enabled) {
    prefs.privacy={enabled:true,ecs_mode:member(f.p30ECSMode,['inherit','strip','fixed'],'p30ECSMode'),
      ecs_address:f.p30ECSMode==='fixed'?String(f.p30ECSAddress||'').trim():'',
      dnssec_mode:member(f.p30DNSSECMode,['off','request_do','require_ad'],'p30DNSSECMode')};
  }
  if (f.p31DNS64Enabled) prefs.dns64={enabled:true,prefix:String(f.p31DNS64Prefix||'').trim(),nat64_confirmed:Boolean(f.p31NAT64Confirmed)};
  if (f.p31ReverseEnabled) {
    if (!f.p31ReverseUpstream||!config.upstreams.some(x=>x.id===f.p31ReverseUpstream&&x.transport!=='native')) throw Error('Выбери DNS для приватного rDNS');
    prefs.reverse_dns={enabled:true,upstream:f.p31ReverseUpstream};
  }
  if (f.p242HealthEnabled&&!f.p241IPEnabled) throw Error('Сначала включи IP Intelligence');
  if (f.p241IPEnabled) {
    prefs.ip_intelligence={enabled:true,mode:member(f.p241IPMode,IP_MODES,'p241IPMode')};
    const scope=member(f.p243Scope,['all','domains','lists'],'p243Scope');
    if (scope==='domains') {
      const domains=parseLiteDomains(String(f.p243Domains||''));
      if (!domains.length||domains.length>128) throw Error('IP Intelligence: укажи 1..128 доменов');
      prefs.ip_intelligence.scope='domains';prefs.ip_intelligence.domains=domains;
    } else if (scope==='lists') {
      const ids=Array.isArray(f.p243ListIDs)?[...f.p243ListIDs]:[];
      if (!ids.length) throw Error('IP Intelligence: выбери хотя бы один список');
      const available=new Set((config.lists||[]).map(x=>x.id));
      if (new Set(ids).size!==ids.length||ids.some(id=>!available.has(id))) throw Error('IP Intelligence: выбранный список изменился');
      prefs.ip_intelligence.scope='lists';prefs.ip_intelligence.list_ids=ids;
    }
    if (f.p242HealthEnabled) prefs.ip_intelligence.health={
      enabled:true,protocol:member(f.p242HealthProtocol,HEALTH_PROTOCOLS,'p242HealthProtocol'),
      port:integer(f.p242HealthPort,'p242HealthPort',80,65535),
      interval_seconds:integer(f.p242HealthInterval,'p242HealthInterval',30,3600),
      timeout_ms:integer(f.p242HealthTimeout,'p242HealthTimeout',250,3000),
      failure_threshold:integer(f.p242HealthFailures,'p242HealthFailures',1,5),
      recovery_threshold:integer(f.p243Recoveries,'p243Recoveries',1,5),
      min_down_seconds:integer(f.p243MinDown,'p243MinDown',0,3600),
      http_path:String(f.p243HTTPPath||'').trim()||'/'
    };
  }
  return prefs;
}

export function mergeLiteServicePrefs(previous, next) {
  const merged={...(previous||{}),...next};
  for(const key of ['ip_intelligence','protection','upstream_selection','performance','privacy','dns64','reverse_dns']) {
    if (!next[key]) delete merged[key];
  }
  if(previous?.default_upstream!==next.default_upstream) delete merged.default_group;
  return merged;
}
