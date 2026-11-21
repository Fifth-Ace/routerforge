import { rfUi } from './rf-localization.js';
// Read-only pure native endpoint catalog, extracted from the R15 legacy runtime.

  const hintRE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$/;
  const policyRE = /^(System|Policy(?:0|[1-9][0-9]{0,2}))$/;
  const zoneRE = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/;
  const loopbackRE = /^127\.0\.0\.1:([0-9]{4,5})$/;
  function zoneValid(zone) {
    return zone === '.' || (typeof zone === 'string' && zone.length <= 253 && zoneRE.test(zone));
  }
  function entries(inventory, policy) {
    if (!policyRE.test(String(policy || '')) || !inventory || !Array.isArray(inventory.instances)) return [];
    const complete = inventory.available === true && inventory.parsed_instances === inventory.instances.length;
    const instance = inventory.instances.find(x => x && x.name === policy);
    if (!instance || !Array.isArray(instance.endpoints)) return [];
    const counts = new Map();
    for (const ep of instance.endpoints) {
      if (!ep) continue;
      const key = String(ep.domain) + '\u0000' + String(ep.upstream_hint || '');
      counts.set(key, (counts.get(key) || 0) + 1);
    }
    return instance.endpoints.map((ep, index) => {
      const zone = ep && ep.domain;
      const hint = ep && ep.upstream_hint || '';
      const selectorHint = ep && ep.selector_hint || (hintRE.test(hint) ? hint : '');
      const isDoH = ep && ep.transport_hint==='doh' && /^https:\/\/[^\s]+@dnsm$/.test(hint);
      const loopback = ep && typeof ep.address === 'string' && loopbackRE.exec(ep.address);
      const numericPort = loopback ? Number(loopback[1]) : 0;
      let reason = '';
      if (!complete) reason = rfUi('Инвентаризация Keenetic неполная',"Keenetic inventory is incomplete");
      else if (!instance.valid) reason = rfUi('DNS-инстанс содержит ошибки',"DNS instance has errors");
      else if (!zoneValid(zone)) reason = rfUi('Неизвестная DNS-зона',"Unknown DNS zone");
      else if (!loopback || numericPort < 1025 || numericPort > 65535) reason = rfUi('Некорректный локальный endpoint',"Invalid local endpoint");
      else if (!hintRE.test(selectorHint)) reason = 'Нет устойчивого идентификатора DNS-сервера';
      else if (counts.get(String(zone)+'\u0000'+hint) !== 1) reason = 'Неоднозначный DNS-сервер';
      const usable = reason === '';
      return {
        index, zone: String(zone || ''), hint: String(hint),
        address: String(ep && ep.address || ''),
        selector: usable ? 'native://' + policy + '/' + zone + '/' + selectorHint : '',
        usable, reason,
        // Do NOT infer transport from a stubby port or a comment hint.
        transport: isDoH ? 'doh' : 'unknown', encryptionProven: false, source: 'ndnproxy',
        label: (isDoH ? hint.slice(0,-5) : (hint || ('DNS #' + (index+1)))) + ' · ' + (zone === '.' ? rfUi('все домены',"all domains") : rfUi('зона ',"zone") + zone)
      };
    });
  }
  function acceptsDomains(entry, domains) {
    if (!entry || !entry.usable) return {ok:false, reason:entry && entry.reason || rfUi('DNS недоступен для выбора',"DNS cannot be selected")};
    if (!Array.isArray(domains) || !domains.length) return {ok:false, reason:rfUi('Нет доменов',"No domains")};
    const zone = entry.zone;
    if (zone === '.') return {ok:true, reason:''};
    for (const d of domains) {
      if (typeof d !== 'string' || (d !== zone && !d.endsWith('.' + zone))) {
        return {ok:false, reason:rfUi('DNS привязан в Keenetic к зоне ',"DNS is bound in Keenetic to zone") + zone + rfUi('. Домен ',". Domain") + d + rfUi(' вне этой зоны. Для других доменов нужен DNS зоны «.» либо отдельно настроенный DoT/DoH.',"outside this zone. Other domains require a root-zone DNS (.) or a separately configured DoT/DoH server.")};
      }
    }
    return {ok:true, reason:''};
  }

export {entries, acceptsDomains};
