<script>
  import {onDestroy} from 'svelte';
  import {rfUi} from './rf-localization.js';
  let domain=$state(''), mac=$state(''), report=$state(null), busy=$state(false), error=$state(''), bindings=$state(null), bindingError=$state('');
  let ctl=null, alive=true;
  const text=(ru,en)=>rfUi(ru,en);
  const val=v=>v===null||v===undefined?'—':String(v);
  const yes=v=>v===null||v===undefined?'—':v?text('Да','Yes'):text('Нет','No');
  function reason(r){return ({WINNER:text('Первое по приоритету','Highest precedence'),LESS_SPECIFIC:text('Менее точный домен','Less specific domain'),MATCH_RANK:text('Тип совпадения','Match type'),PRIORITY:text('Числовой приоритет','Numeric priority'),RULE_ID_TIEBREAK:text('Порядок ID','Rule ID tiebreak')})[r]||r;}
  function dnssec(v){return ({off:text('Выключено','Off'),request_do:text('Запрос DO','Request DO'),require_ad:text('Требовать AD от upstream','Require upstream AD')})[v]||v;}
  async function load(explain=false){if(busy)return;busy=true;error='';report=null;bindings=null;bindingError='';const c=new AbortController();ctl=c;const timer=setTimeout(()=>c.abort(),15000);
    try{const q=new URLSearchParams();if(explain){q.set('domain',domain.trim());if(mac.trim())q.set('mac',mac.trim().toLowerCase());}const r=await fetch('../service/policy-audit'+(q.size?'?'+q.toString():''),{method:'GET',cache:'no-store',credentials:'same-origin',redirect:'error',signal:c.signal});if(!r.ok)throw Error('HTTP '+r.status);const v=await r.json();if(v.schema_version!==1||v.dry_run!==true)throw Error('INVALID_RESPONSE');if(alive&&!c.signal.aborted)report=v;
      // Separate read of registry metadata: do not equate it with live state.
      try {const br=await fetch('/api/platform/foundation/bindings',{method:'GET',cache:'no-store',credentials:'same-origin',redirect:'error',signal:c.signal});if(!br.ok)throw Error('HTTP '+br.status);const b=await br.json();if(b.schemaVersion!==1||!Array.isArray(b.bindings)||b.bindings.length>16)throw Error('INVALID_RESPONSE');if(alive&&!c.signal.aborted)bindings={generation:b.generation,rows:b.bindings.filter(x=>typeof x.target==='string'&&x.target.startsWith('dns.'))};}catch(e){if(alive)bindingError=String(e.message||e);}}catch(e){if(alive)error=String(e.message||e);}finally{clearTimeout(timer);if(ctl===c)ctl=null;if(alive)busy=false;}}
  onDestroy(()=>{alive=false;ctl?.abort()});
</script>
<section class="audit-e09">
  <h4>{text('Аудит политики и конфигурации','Policy and configuration audit')}</h4>
  <p>{text('Сравнение сохранённой и применённой политики. Только чтение: без DNS-запросов, активации, изменения Gate и очистки кэша.','Compare saved and applied policy. Read only: no DNS requests, activation, Gate changes or cache clearing.')}</p>
  <div class="actions"><button class="button" disabled={busy} onclick={()=>load(false)}>{text('Собрать аудит','Collect audit')}</button><span role="status">{busy?text('Сбор…','Collecting…'):error}</span></div>
  <div class="fields two"><label class="field"><span>{text('Домен (ASCII / IDNA)','Domain (ASCII / IDNA)')}</span><input bind:value={domain} maxlength="253" placeholder="example.org" disabled={busy} onchange={()=>report=null}/></label><label class="field"><span>{text('MAC для моделирования (необязательно)','Hypothetical MAC (optional)')}</span><input bind:value={mac} maxlength="17" placeholder="02:11:22:33:44:55" disabled={busy} onchange={()=>report=null}/></label></div>
  <button class="button" disabled={busy||!domain.trim()} onclick={()=>load(true)}>{text('Объяснить обе редакции','Explain both revisions')}</button>
  {#if report}
    <p><strong>{report.active?(report.same_revision?text('Редакции совпадают','Revisions match'):text('Сохранённая политика отличается от активной','Saved policy differs from active')):text('Активная политика отсутствует','No active policy')}</strong></p>
    <div class="revisions"><small>{text('Сохранена','Saved')}: {report.saved_revision}</small><small>{text('Применена','Applied')}: {report.active?report.applied_revision:'—'}</small></div>
    {#each [{label:text('Сохранённая','Saved'),config:report.saved,explain:report.saved_explain},{label:text('Применённая','Applied'),config:report.applied,explain:report.applied_explain}] as row}
      {#if row.config}
        {@const c=row.config}
        <details open><summary>{row.label} · {c.rules}/{c.rule_limit} {text('правил','rules')} · {c.conflicts.total} {text('конфликтов равного приоритета','equal-precedence conflicts')}</summary>
          <div class="audit-grid"><span>DNSSEC: <b>{dnssec(c.dnssec_mode)}</b></span><span>DNS64: <b>{yes(c.dns64_enabled)} {c.dns64_prefix||''}</b></span><span>NAT64: <b>{c.nat64_declared?text('Заявлен пользователем','User declared'):text('Не подтверждён','Not confirmed')}</b></span><span>{text('Кэш, байт','Cache, bytes')}: <b>{c.cache_bytes}</b></span><span>TTL min / max: <b>{val(c.cache_min_ttl)} / {val(c.cache_max_ttl)}</b></span><span>{text('Кэш политик / optimistic','Policy / optimistic cache')}: <b>{yes(c.policy_cache_configured)} / {yes(c.optimistic_cache_configured)}</b></span></div>
          <p class="muted">{text('DO и AD не являются локальной DNSSEC-валидацией. NAT64 и сквозная IPv6-доступность не проверяются этим аудитом. TTL — настройки, а не возраст записей.','DO and AD are not local DNSSEC validation. This audit does not test NAT64 or end-to-end IPv6. TTL values are settings, not entry ages.')}</p>
          {#if row.explain}{@const x=row.explain}<div class="decision"><strong>{x.decision.domain} · {x.decision.layer}</strong><span>{text('Выбрано правило','Selected rule')}: {x.decision.rule_id||'—'} · {x.decision.upstream||'—'} {x.decision.group_id||''}</span><span>{text('Разрешённые upstream','Allowed upstreams')}: {x.allowed_upstreams.join(' → ')||'—'}</span><small>{text('Цепочка разрешённых серверов не является прогнозом фактического выбора или успешного ответа. MAC моделируется, а не определяется. ACL, локальные записи и кэш не оцениваются.','Allowed servers are not a prediction of actual selection or successful response. MAC is hypothetical. ACL, local records and cache are not evaluated.')}</small></div>
            {#each [{title:text('Общие правила','Shared rules'),r:x.shared},{title:text('Правила устройства','Device rules'),r:x.device}] as g}
              <details><summary>{g.title} ({g.r.rows.length}/{g.r.total})</summary><div class="scroll"><table><thead><tr><th>ID</th><th>{text('Домен','Domain')}</th><th>{text('Тип / приоритет','Type / priority')}</th><th>DNS</th><th>{text('Место в слое','Within-layer rank')}</th></tr></thead><tbody>{#each g.r.rows as v}<tr><td>{v.id}</td><td>{v.domain}</td><td>{v.match} / {v.priority}</td><td>{v.upstream}</td><td>{reason(v.reason)}</td></tr>{/each}</tbody></table></div></details>
            {/each}
          {/if}
          {#if c.conflicts.total}<details><summary>{text('Конфликтующие цели в общих правилах','Conflicting targets in shared rules')} ({c.conflicts.rows.length}/{c.conflicts.total})</summary>{#each c.conflicts.rows as v}<div>{v.domain} · {v.match} · {v.priority} · {v.rule_count} {text('правил','rules')} · {text('первый ID','first ID')}: {v.winner_id}</div>{/each}</details>{/if}
          <details><summary>{text('Списки доменов','Domain lists')} ({c.lists.length})</summary><div class="scroll"><table><thead><tr><th>{text('Список','List')}</th><th>{text('Источник','Source')}</th><th>{text('Правил','Rules')}</th><th>{text('Обновлён UTC','Updated UTC')}</th><th>{text('Авто, мин','Auto, min')}</th></tr></thead><tbody>{#each c.lists as l}<tr><td>{l.name} · {l.id}</td><td>{l.kind}</td><td>{l.rules}</td><td>{l.updated_at||'—'}</td><td>{l.auto_update_minutes||text('Нет','Off')}</td></tr>{/each}</tbody></table></div></details>
        </details>
      {/if}
    {/each}
    <details><summary>{text('Постоянные привязки каталога','Persistent catalogue bindings')}</summary>
      {#if bindings}<small>{text('Редакция реестра','Registry generation')}: {bindings.generation} · {text('Отдельное наблюдение, не проверка синхронности','Separate observation, not a synchronization check')}</small>
        {#each bindings.rows as b}<div class="binding-row"><strong>{b.target} → {b.groupId}</strong><span>{text('Сохранённый checkpoint','Saved checkpoint')}: {b.count} · {b.pending?text('Ожидается сверка','Reconciliation pending'):text('Без pending','No pending')}</span><span>{text('Список в сохранённой / активной редакции','List in saved / applied revision')}: {yes(report.saved.lists.some(l=>'dns.'+l.id===b.target))} / {report.applied?yes(report.applied.lists.some(l=>'dns.'+l.id===b.target)):'—'}</span><small>{text('Активность привязки не подтверждена','Binding runtime not verified')}</small></div>{:else}<p>{text('Нет DNS-привязок','No DNS bindings')}</p>{/each}
      {:else}<p>{text('Реестр недоступен','Registry unavailable')} {bindingError}</p>{/if}
      <a href="/policy-objects" target="_top">{text('Объекты политик и привязки ↗','Policy objects and bindings ↗')}</a>
      <span> · </span><a href="/manage" target="_top">{text('Управление: Recovery Center / Support Bundle ↗','Admin: Recovery Center / Support Bundle ↗')}</a>
    </details>
    <details open><summary>{text('Активный runtime / Gate' ,'Active runtime / Gate')}</summary><div class="audit-grid"><span>{text('Запросов','Requests')}: {val(report.requests)}</span><span>{text('Попаданий в кэш','Cache hits')}: {val(report.cache_hits)}</span><span>{text('Ошибок','Failures')}: {val(report.failures)}</span><span>{text('IPv6 listener','IPv6 listener')}: {yes(report.listener_ipv6)}</span></div>
      {#if report.policy_cache?.enabled}<div class="audit-grid"><span>{text('Записей','Entries')}: {val(report.policy_cache.entries)}</span><span>{text('Память (оценка) / лимит','Estimated bytes / budget')}: {val(report.policy_cache.estimated_bytes)} / {val(report.policy_cache.budget_bytes)}</span><span>{text('Попадания / промахи','Hits / misses')}: {val(report.policy_cache.hits)} / {val(report.policy_cache.misses)}</span><span>{text('Вытеснения','Evictions')}: {val(report.policy_cache.evictions)}</span></div>{/if}
      {#each Object.entries(report.gate_coverage||{}) as [key,value]}<div>{key}: {value}</div>{/each}
      <small>{text('Gate отражает последнее наблюдение контроллера; аппаратная проверка отдельно.','Gate reflects the controller’s last observation; hardware verification is separate.')}</small>
    </details>
  {/if}
</section>
<style>
.audit-e09{display:grid;gap:9px;padding:12px;border:1px solid var(--rf-border,#38414b);border-radius:6px;margin-bottom:12px;min-width:0}.audit-e09 p,.audit-e09 h4{margin:0}.audit-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(185px,1fr));gap:8px;padding:8px 0}.revisions,.decision,.binding-row{display:grid;gap:4px;overflow-wrap:anywhere}.decision,.binding-row{padding:8px;border:1px solid var(--rf-border,#38414b)}.scroll{overflow-x:auto}.scroll table{width:100%;min-width:460px;text-align:left}details{border-top:1px solid var(--rf-border,#38414b);padding:8px 0}summary{cursor:pointer}.audit-e09 small{overflow-wrap:anywhere}
</style>
