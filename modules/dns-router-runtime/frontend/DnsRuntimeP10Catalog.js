// R15: pure read-only P10 DNS group catalog. No fetch, DOM, mutation, or activation.
// A native Svelte entry imports this module before the legacy draft bridge initializes.
export function isSimpleRule(rule) {
  return !!rule && rule.match === 'suffix' && rule.priority === 100 &&
    typeof rule.domain === 'string' &&
    Object.keys(rule).every(k => ['id', 'domain', 'match', 'upstream', 'priority'].includes(k));
}

export function dnsName(upstream, nativeEntries=[]) {
  if (!upstream) return {title:'НЕИЗВЕСТНЫЙ DNS',detail:'Ссылка на DNS-сервер отсутствует',scope:'broken'};
  if (upstream.transport === 'native') {
    const entries = Array.isArray(nativeEntries) ? nativeEntries : [];
    const current = entries.find(x => x.selector === upstream.address);
    const instance = String(upstream.address || '').split('/')[2] || '?';
    return {title:current ? current.label : 'DNS Keenetic · ' + instance + ' (не подтверждён)',
      detail:current ? 'Keenetic · зона ' + current.zone + ' · ' + (current.address || '') :
        'Сохранённый селектор: ' + String(upstream.address || ''), scope:'native'};
  }
  const transport = String(upstream.transport || 'unknown').toUpperCase();
  const name = String(upstream.server_name || upstream.address || upstream.id);
  const target = upstream.transport === 'doh' && upstream.server_name ?
    'https://' + upstream.server_name + (upstream.path || '/dns-query') : String(upstream.address || '');
  return {title:transport + ' · ' + name,
    detail:'Endpoint: ' + target + (upstream.address && upstream.address !== name ? ' · IP: ' + upstream.address : ''),
    scope:'manual'};
}

export function dnsGroups(config, policy='System', nativeEntries=[]) {
  const upstreams = Array.isArray(config?.upstreams) ? config.upstreams : [];
  const rules = Array.isArray(config?.rules) ? config.rules : [];
  const groups = [];
  for (const u of upstreams) {
    if (u.transport === 'native' && !String(u.address).startsWith('native://' + policy + '/')) continue;
    groups.push({id:u.id, u, info:dnsName(u,nativeEntries), rules:rules.filter(r=>r.upstream===u.id)});
  }
  const known = new Set(upstreams.map(u=>u.id));
  for (const id of new Set(rules.filter(r=>!known.has(r.upstream)).map(r=>r.upstream))) {
    groups.push({id,u:null,info:dnsName(null,nativeEntries),rules:rules.filter(r=>r.upstream===id)});
  }
  groups.sort((a,b) => (b.rules.length>0)-(a.rules.length>0) ||
    (a.info.scope==='native'?0:a.info.scope==='manual'?1:2)-
    (b.info.scope==='native'?0:b.info.scope==='manual'?1:2) ||
    a.info.title.localeCompare(b.info.title,'ru'));
  return groups;
}

export function dnsChoice(group) {
  if (!group?.u) return '';
  return group.u.transport === 'native' ? group.u.address : 'manual:' + group.id;
}

// Preserve the original P10 apply summary: all configured rules, including other
// policies and broken references, are reported, never silently omitted.
export function dnsSummary(config, nativeEntries=[]) {
  const upstreams = Array.isArray(config?.upstreams) ? config.upstreams : [];
  const rules = Array.isArray(config?.rules) ? config.rules : [];
  const byId = new Map(upstreams.map(u=>[u.id,u]));
  const counts = new Map();
  for (const r of rules) counts.set(r.upstream,(counts.get(r.upstream)||0)+1);
  return [...counts].map(([id,count])=>{
    const u=byId.get(id),label=dnsName(u,nativeEntries);
    const domains=rules.filter(r=>r.upstream===id).slice(0,3).map(r=>r.domain).join(', ');
    return '• '+label.title+' ['+id+']: '+count+' доменов ('+domains+(count>3?', …':'')+')';
  }).join('\n');
}
