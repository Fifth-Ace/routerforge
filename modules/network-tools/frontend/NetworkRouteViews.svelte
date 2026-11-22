<script>
  let { data = null, target = '', section = 'facts', family = '', table = '', search = '', rulesSearch = '', fit = 'values', message = '', lx } = $props();
  const policy = name => ({active:lx('Активна','Active'),'default-only':lx('Только базовые правила','Default rules only'),unavailable:lx('Недоступно','Unavailable')}[name] || name || '—');
  let decision = $derived(data?.kernel_decision?.available ? data.kernel_decision : data?.selected);
  let capabilities = $derived(data?.capabilities || {});
  let resolved = $derived(data?.resolution?.addresses || []);
  let rules = $derived([...(data?.rules_v4 || []), ...(data?.rules_v6 || [])]);
  let routes = $derived([...(data?.routes_v4 || []), ...(data?.routes_v6 || [])]);
  let filtered = $derived(routes.filter(r => (!family || String(r.family||'').toLowerCase()===family) && (!table || String(r.table||'main')===table) && (!search.trim() || [r.destination,r.gateway,r.interface,r.source,r.table,r.protocol,r.scope,r.type,r.metric].join(' ').toLowerCase().includes(search.trim().toLowerCase()))));
  let filteredRules = $derived(rules.filter(r => !rulesSearch.trim() || JSON.stringify(r).toLowerCase().includes(rulesSearch.trim().toLowerCase())));
  let facts = $derived(decision ? [[lx('Адрес цели','Target address'),decision.destination||data?.target_address||data?.target||'—'],[lx('Семейство','Family'),decision.family||data?.family||'—'],[lx('Следующий хоп','Next hop'),decision.gateway||'direct'],[lx('Интерфейс','Interface'),decision.interface||'—'],[lx('Исходный адрес','Source address'),decision.source||'—'],[lx('Таблица','Table'),decision.table||'main'],[lx('Метрика','Metric'),Number(decision.metric||0)],[lx('Тип','Type'),decision.type||'unicast'],['PBR',policy(data?.policy_state)]] : []);
  let steps = $derived.by(() => {
    if (!data) return [];
    const dec=data.kernel_decision||{}, cap=data.capabilities||{}, grouped=(data.family==='ipv6'?data.rules_v6:data.rules_v4)||[];
    const lookup=cap.route_get&&dec.available?lx('Точный ip route get','Exact ip route get'):dec.available?lx('Резервный longest-prefix lookup','Fallback longest-prefix lookup'):lx('Решение недоступно','Decision unavailable');
    return [[lx('Разрешение цели','Target resolution'),resolved.length ? resolved.join(', ') : 'Target: '+target],
      [lx('Поиск маршрута','Route lookup'),lookup],
      [lx('Политика PBR','Policy routing'),policy(data.policy_state)+' · '+grouped.length+' '+lx('правил','rules')],
      [lx('Выбор next-hop','Next-hop selection'),dec.available ? (dec.gateway||'direct')+' · '+(dec.interface||'—') : '—'],
      [lx('Выбор source address','Source selection'),dec.source||lx('Ядро не сообщило source address','Kernel did not report a source address')]];
  });
  const caps = [['ip','ip_command'],['IPv4 tables','ipv4_all_tables'],['IPv6 tables','ipv6_all_tables'],['IPv4 rules','ipv4_rules'],['IPv6 rules','ipv6_rules'],['route get','route_get']];
</script>
{#if message}<div class="rf-error">{message}</div>
{:else if !data}<div class="nt-empty">{section === 'facts' ? lx('Маршрут ещё не выбран.','No route selected yet.') : lx('Загрузка данных…','Loading data...')}</div>
{:else if section === 'capabilities'}
  {#each caps as cap (cap[1])}<span class:ok={Boolean(capabilities[cap[1]])} class:off={!capabilities[cap[1]]} class="route-cap"><b>{cap[0]}</b>{capabilities[cap[1]] ? 'OK' : 'N/A'}</span>{/each}
  <span class="route-source">{data.source||'unknown'} · {data.decision_scope||'router-originated'}</span>
{:else if section === 'facts'}
  {#if !decision}<div class="nt-empty">{lx('Маршрут не определён.','Route decision is unavailable.')}</div>{:else}
    <div class="route-title">{lx('Решение ядра','Kernel decision')}</div>
    {#each facts as item, i (i)}<div class="fact-row"><span>{item[0]}</span><strong>{item[1]}</strong></div>{/each}
  {/if}
{:else if section === 'decision'}
  <div class="route-title">{lx('Путь принятия решения','Decision path')}</div>
  {#each steps as item, i (i)}<div class="decision-row"><span class="decision-number">{i+1}</span><div class="decision-copy"><strong>{item[0]}</strong><small>{item[1]}</small></div></div>{/each}
{:else if section === 'routes'}
  {#if !filtered.length}<div class="nt-empty">{lx('Нет маршрутов по текущему фильтру.','No routes match the current filter.')}</div>
  {:else}<table class={fit==='grid' ? 'route-data-table fit-grid' : 'route-data-table fit-values'}><thead><tr><th class="c-family">IP</th><th class="c-destination">{lx('Назначение','Destination')}</th><th class="c-table">{lx('Таблица','Table')}</th><th class="c-via">Via</th><th class="c-dev">Dev</th><th class="c-src">Src</th><th class="c-metric">Metric</th><th class="c-proto">Proto / Scope</th></tr></thead><tbody>
    {#each filtered.slice(0,200) as route, i (i)}<tr class:is-default={['default','0.0.0.0/0','::/0'].includes(route.destination)}><td>{route.family||'—'}</td><td class="mono">{route.destination||'—'}</td><td class="mono">{route.table||'main'}</td><td class="mono">{route.gateway||'direct'}</td><td>{route.interface||'—'}</td><td class="mono">{route.source||'—'}</td><td class="num">{route.metric||0}</td><td>{[route.protocol,route.scope,route.type].filter(Boolean).join(' / ')||'—'}</td></tr>{/each}
  </tbody></table>{/if}
{:else if section === 'rules'}
  {#if !filteredRules.length}<div class="nt-empty">{lx('Нет PBR-правил по текущему фильтру.','No policy rules match the current filter.')}</div>
  {:else}<table class="policy-table"><thead><tr><th>IP</th><th>Priority</th><th>From</th><th>To</th><th>Mark</th><th>IIF / OIF</th><th>Table / Action</th></tr></thead><tbody>
    {#each filteredRules.slice(0,160) as rule, i (i)}<tr><td>{rule.family||'—'}</td><td class="num">{rule.priority}</td><td class="mono">{rule.from||'all'}</td><td class="mono">{rule.to||'all'}</td><td class="mono">{rule.mark||'—'}</td><td>{[rule.iif,rule.oif].filter(Boolean).join(' / ')||'—'}</td><td class="mono">{rule.table||rule.action||'—'}</td></tr>{/each}
  </tbody></table>{/if}
{/if}
