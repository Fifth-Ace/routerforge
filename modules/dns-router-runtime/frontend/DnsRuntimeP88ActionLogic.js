// R13: P88 domain draft and probe guards. No IO and no mutation of input config.
export function planDomainAddition(config,selected,choice,domains,acceptsDomains) {
  if (!config || !Array.isArray(config.upstreams) || !Array.isArray(config.rules))
    throw Error('Конфигурация DNS неполная. Обнови страницу.');
  if (!choice || choice.value!==selected || !['native','manual'].includes(choice.type))
    throw Error('Выбери DNS-сервер из текущего списка Keenetic.');
  if (!Array.isArray(domains) || domains.length===0 || domains.length>200 ||
      domains.some(d=>typeof d!=='string' || !d || d!==d.toLowerCase()))
    throw Error('Добавь корректный список доменов (не более 200).');
  if(new Set(domains).size!==domains.length)throw Error('В списке есть дубликаты.');
  if(typeof acceptsDomains!=='function')throw Error('Проверка зоны DNS недоступна.');
  const scope=acceptsDomains({usable:true,zone:choice.zone},domains);
  if(!scope || scope.ok!==true)throw Error(scope?.reason||'DNS не подходит для выбранных доменов.');
  const existing=new Set(config.rules.map(r=>String(r.domain||'').toLowerCase()));
  const duplicate=domains.find(d=>existing.has(d));
  if(duplicate)throw Error('Домен '+duplicate+' уже есть в правилах. Открой расширенные настройки, чтобы изменить его DNS.');
  const upstreams=config.upstreams.slice(),rules=config.rules.slice();
  let upstream;
  if(choice.type==='manual') {
    upstream=upstreams.find(u=>u.id===choice.upstreamId);
    if(!upstream||!['doh','dot','udp','doq'].includes(upstream.transport))
      throw Error('Ручной DNS больше не найден. Обнови настройки.');
  } else {
    if(!selected.startsWith('native://') || choice.zone===undefined)
      throw Error('Неподтверждённый DNS Keenetic.');
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
     throw Error('Проверка example.com доступна только для DNS зоны «.». DNS других зон не проверяется чужим доменом.');
  if(!selected.startsWith('native://'))throw Error('Неподтверждённый DNS Keenetic.');
  const parts=selected.slice('native://'.length).split('/');
  if(parts.length!==3 || parts[1]!=='.' ||
     !/^(System|Policy(?:0|[1-9][0-9]{0,2}))$/.test(parts[0]) ||
     !/^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$/.test(parts[2]))
     throw Error('Недопустимый идентификатор DNS Keenetic.');
  return 'keenetic/native-dns/probe?'+new URLSearchParams({instance:parts[0],hint:parts[2]}).toString();
}

export function probeMessage(result){
  if(!result || typeof result!=='object')throw Error('Некорректный ответ DNS-пробы.');
  if(result.dns_success===true && Number.isFinite(result.latency_ms))
    return 'DNS отвечает · '+result.latency_ms+' мс · проверка шифрования не выполнялась';
  return 'DNS не отвечает корректно · '+String(result.error||('RCODE '+(result.rcode??'?')));
}
