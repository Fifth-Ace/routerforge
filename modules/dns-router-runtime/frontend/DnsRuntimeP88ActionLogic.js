import { rfUi } from './rf-localization.js';
// R13: P88 domain draft and probe guards. No IO and no mutation of input config.
export function planDomainAddition(config,selected,choice,domains,acceptsDomains) {
  if (!config || !Array.isArray(config.upstreams) || !Array.isArray(config.rules))
    throw Error(rfUi('Конфигурация DNS неполная. Обнови страницу.',"DNS configuration is incomplete. Refresh the page."));
  if (!choice || choice.value!==selected || !['native','manual'].includes(choice.type))
    throw Error(rfUi('Выбери DNS-сервер из текущего списка Keenetic.',"Select a DNS server from the current Keenetic list."));
  if (!Array.isArray(domains) || domains.length===0 || domains.length>200 ||
      domains.some(d=>typeof d!=='string' || !d || d!==d.toLowerCase()))
    throw Error(rfUi('Добавь корректный список доменов (не более 200).',"Add a valid domain list (up to 200 domains)."));
  if(new Set(domains).size!==domains.length)throw Error(rfUi('В списке есть дубликаты.',"This list contains duplicate entries."));
  if(typeof acceptsDomains!=='function')throw Error(rfUi('Проверка зоны DNS недоступна.',"DNS zone testing is unavailable."));
  const scope=acceptsDomains({usable:true,zone:choice.zone},domains);
  if(!scope || scope.ok!==true)throw Error(scope?.reason||rfUi('DNS не подходит для выбранных доменов.',"DNS is not suitable for the selected domains."));
  const existing=new Set(config.rules.map(r=>String(r.domain||'').toLowerCase()));
  const duplicate=domains.find(d=>existing.has(d));
  if(duplicate)throw Error(rfUi('Домен ',"Domain")+duplicate+rfUi(' уже есть в правилах. Открой расширенные настройки, чтобы изменить его DNS.',"already exists in the rules. Open advanced settings to change its DNS."));
  const upstreams=config.upstreams.slice(),rules=config.rules.slice();
  let upstream;
  if(choice.type==='manual') {
    upstream=upstreams.find(u=>u.id===choice.upstreamId);
    if(!upstream||!['doh','dot','udp','doq'].includes(upstream.transport))
      throw Error(rfUi('Ручной DNS больше не найден. Обнови настройки.',"The custom DNS entry no longer exists. Refresh settings."));
  } else {
    if(!selected.startsWith('native://') || choice.zone===undefined)
      throw Error(rfUi('Неподтверждённый DNS Keenetic.',"Unverified Keenetic DNS."));
    upstream=upstreams.find(u=>u.transport==='native'&&u.address===selected);
    if(!upstream){
      const used=new Set(upstreams.map(u=>u.id));
      let n=1;while(used.has('native-'+n))n++;
      upstream={id:'native-'+n,transport:'native',address:selected};upstreams.push(upstream);
    }
  }
  const ids=new Set(rules.map(r=>r.id));
  for(const domain of domains){
    let n=1;while(ids.has('domain-'+n))n++;
    const id='domain-'+n;ids.add(id);
    rules.push({id,domain,match:'suffix',upstream:upstream.id,priority:100});
  }
  return {upstreams,rules,count:domains.length};
}

export function nativeProbeTarget(selected,choices) {
  if(typeof selected!=='string'||!Array.isArray(choices)||
     !choices.some(x=>x.value===selected&&x.zone==='.'&&x.type==='native'))
     throw Error(rfUi('Проверка example.com доступна только для DNS зоны «.». DNS других зон не проверяется чужим доменом.',"The example.com test is available only for the root DNS zone (.). DNS for other zones is not tested with an unrelated domain."));
  if(!selected.startsWith('native://'))throw Error(rfUi('Неподтверждённый DNS Keenetic.',"Unverified Keenetic DNS."));
  const parts=selected.slice('native://'.length).split('/');
  if(parts.length!==3 || parts[1]!=='.' ||
     !/^(System|Policy(?:0|[1-9][0-9]{0,2}))$/.test(parts[0]) ||
     !/^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$/.test(parts[2]))
     throw Error(rfUi('Недопустимый идентификатор DNS Keenetic.',"Invalid Keenetic DNS identifier."));
  return 'keenetic/native-dns/probe?'+new URLSearchParams({instance:parts[0],hint:parts[2]}).toString();
}

export function probeMessage(result){
  if(!result || typeof result!=='object')throw Error(rfUi('Некорректный ответ DNS-пробы.',"Invalid DNS probe response."));
  if(result.dns_success===true && Number.isFinite(result.latency_ms))
    return rfUi('DNS отвечает · ',"DNS responding ·")+result.latency_ms+rfUi(' мс · проверка шифрования не выполнялась',"ms · encryption not checked");
  return rfUi('DNS не отвечает корректно · ',"DNS response invalid ·")+String(result.error||('RCODE '+(result.rcode??'?')));
}
