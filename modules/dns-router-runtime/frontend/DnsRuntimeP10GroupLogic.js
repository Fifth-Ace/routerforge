import { rfUi } from './rf-localization.js';
// R14: pure DNS group edit planner. No HTTP, DOM, service, Keenetic, or mutations.
// The Svelte action owns confirmation; the CAS bridge alone commits a prepared plan.
const simpleRule = r => r && r.match === 'suffix' && r.priority === 100 &&
  typeof r.domain === 'string' &&
  Object.keys(r).every(k => ['id', 'domain', 'match', 'upstream', 'priority'].includes(k));

export function planGroupReplacement(config, upstreamId, selected, chosenDNS, domains, acceptsDomains, revision) {
  if (!config || !Array.isArray(config.upstreams) || !Array.isArray(config.rules))
    throw Error(rfUi('Черновик DNS недоступен. Перезагрузи настройки.',"DNS draft unavailable. Reload settings."));
  if (!upstreamId || typeof upstreamId !== 'string' || !chosenDNS || !selected ||
      typeof acceptsDomains !== 'function' || !Array.isArray(domains))
    throw Error(rfUi('Не подтверждён текущий DNS или список доменов.',"The current DNS or domain list is not verified."));
  const upstream = config.upstreams.find(u => u.id === upstreamId);
  if (!upstream) throw Error(rfUi('DNS для редактирования больше не существует.',"The DNS entry being edited no longer exists."));
  const expectedSelector = upstream.transport === 'native' ? upstream.address : 'manual:' + upstreamId;
  const chosenId = chosenDNS.type === 'manual' ? chosenDNS.upstreamId :
    config.upstreams.find(u => u.transport === 'native' && u.address === selected)?.id;
  if (selected !== expectedSelector || chosenDNS.value !== selected || chosenId !== upstreamId)
    throw Error(rfUi('Выбранный DNS изменился. Замена списка остановлена.',"The selected DNS changed. List replacement was stopped."));
  const before = config.rules.filter(r => r.upstream === upstreamId);
  if (before.length > 200 || before.some(r => !simpleRule(r)) || domains.length > 200)
    throw Error(rfUi('Расширенные правила и списки более 200 доменов через простой редактор не заменяются.',"Advanced rules and lists with more than 200 domains cannot be replaced in the basic editor."));
  if (new Set(domains).size !== domains.length ||
      domains.some(d => typeof d !== 'string' || !/^[a-z0-9.-]+$/.test(d)))
    throw Error(rfUi('Список должен содержать уникальные проверенные домены.',"The list must contain unique, verified domains."));
  const scope = acceptsDomains({usable: true, zone: chosenDNS.zone}, domains);
  if (!scope || !scope.ok) throw Error(scope?.reason || rfUi('Домены вне зоны выбранного DNS.',"Domains are outside the selected DNS zone."));
  const outside = new Set(config.rules.filter(r => r.upstream !== upstreamId)
    .map(r => String(r.domain).toLowerCase()));
  const collision = domains.find(d => outside.has(d.toLowerCase()));
  if (collision) throw Error(rfUi('Домен ',"Domain") + collision + rfUi(' уже принадлежит другому DNS. Ничего не заменено.',"is already assigned to another DNS. Nothing was replaced."));
  const desired = new Set(domains), original = new Set(before.map(r => r.domain));
  const removed = before.filter(r => !desired.has(r.domain)).length;
  const addedDomains = domains.filter(d => !original.has(d));
  const ids = new Set(config.rules.map(r => r.id));
  const additions = addedDomains.map(domain => {
    let n = 1; while (ids.has('domain-' + n)) n++;
    const id = 'domain-' + n; ids.add(id);
    return {id, domain, match: 'suffix', upstream: upstreamId, priority: 100};
  });
  const rules = config.rules.filter(r => r.upstream !== upstreamId || desired.has(r.domain)).concat(additions);
  return {
    upstreamId, revision: String(revision || ''), baseRules: JSON.stringify(config.rules), rules,
    added: additions.length, removed, count: domains.length, changed: Boolean(removed || additions.length)
  };
}
