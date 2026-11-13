<script>
  let { data = null, busy = false, message = '', lx, tr } = $props();
  const icons = {ping:'⌁',dns:'◎',http:'▣',tcp:'◉'};
  const names = {ping:'Ping',dns:'DNS',http:'HTTP',tcp:'TCP'};
  const verdictLabels = {
    healthy:['Сетевой путь исправен','Network path is healthy'], degraded:['Сеть работает с ограничениями','Network is operating with warnings'],
    no_default_route:['Нет маршрута по умолчанию','No default route'],interface_down:['Выходной интерфейс недоступен','Default interface is down'],
    no_local_address:['Нет рабочего IP-адреса','No usable local address'],internet_unreachable:['Внешняя сеть недоступна','Internet connectivity failed'],
    dns_failure:['DNS не работает','DNS resolution failed'],target_route_failure:['Нет маршрута к цели','No route to target'],
    egress_interface_failure:['Выбранный ядром интерфейс недоступен','Kernel-selected egress is unavailable'],
    gateway_interface_mismatch:['Шлюз и выходной интерфейс не согласованы','Route gateway/interface mismatch'],
    route_source_mismatch:['Исходный адрес не принадлежит интерфейсу','Route source does not belong to selected interface'],
    target_unreachable:['Цель не отвечает на Ping','Target does not answer Ping'],tcp_failure:['TCP-сервис недоступен','TCP service is unreachable'],http_failure:['HTTP-проверка не прошла','HTTP check failed']
  };
  const stages = {
    default_route:['Маршрут по умолчанию','Default route'],interface:['Сетевой интерфейс','Network interface'],
    local_address:['Локальный адрес','Local address'],gateway:['Шлюз','Gateway'],internet:['Доступ в Интернет','Internet connectivity'],
    dns:['DNS','DNS'],target_route:['Маршрут к цели','Target route'],policy_routing:['Политика маршрутизации','Policy routing'],
    kernel_egress:['Выходной интерфейс ядра','Kernel-selected egress'],gateway_consistency:['Согласованность шлюза','Gateway/interface consistency'],
    source_consistency:['Исходный адрес маршрута','Route/source consistency'],ping:['Ping','Ping'],tcp:['TCP','TCP'],http:['HTTP','HTTP']
  };
  const actionNames = {
    'review-warnings':['Проверить предупреждения','Review warnings'], 'inspect-default-route':['Проверить default route','Inspect default route'],
    'inspect-policy-rules':['Проверить PBR / policy rules','Inspect PBR / policy rules'], 'inspect-default-interface':['Проверить выходной интерфейс','Inspect default interface'],
    'inspect-interface-address':['Проверить адрес интерфейса','Inspect interface address'],'inspect-kernel-egress':['Проверить kernel-selected egress','Inspect kernel-selected egress'],
    'inspect-target-route':['Проверить маршрут к цели','Inspect target route'],'inspect-source-address':['Проверить исходный адрес','Inspect source address'],
    'verify-upstream':['Проверить upstream','Verify upstream'],'inspect-dns':['Проверить DNS','Inspect DNS'],
    'verify-service':['Проверить сервис','Verify service'],'verify-target-reachability':['Проверить доступность цели','Verify target reachability'],
    'review-diagnosis':['Проверить диагностику','Review diagnosis']
  };
  const statusNames = {ok:['НОРМА','OK'],warn:['ВНИМАНИЕ','WARN'],fail:['ОШИБКА','FAIL'],skipped:['ПРОПУЩЕНО','SKIPPED'],unavailable:['N/A','N/A']};
  const localized = (map,key) => lx(...(map[key] || [String(key),String(key)]));
  const tone = status => status === 'fail' ? 'bad' : status === 'warn' ? 'warn' : status === 'ok' ? '' : 'neutral';
  const detail = (kind,r) => {
    if (!r) return tr('noData');
    if (kind === 'ping') return r.rtt_avg_ms ? 'RTT '+Number(r.rtt_avg_ms).toFixed(1)+' ms' : tr('time')+' '+r.duration_ms+' ms';
    if (kind === 'dns') return (r.addresses||[]).join(', ') || tr('time')+' '+r.duration_ms+' ms';
    if (kind === 'http') return r.status_code ? 'HTTP '+r.status_code+' · '+r.duration_ms+' ms' : tr('time')+' '+r.duration_ms+' ms';
    return tr('time')+' '+r.duration_ms+' ms';
  };
  let diagnosis = $derived(data?.diagnosis || {});
  let verdict = $derived(diagnosis.verdict || {});
  let severity = $derived(verdict.severity || 'ok');
</script>
{#if message}<div class="rf-error">{message}</div>
{:else if busy}<div class="nt-empty">{lx('Выполняется полная диагностика…','Running full diagnosis…')}</div>
{:else if !data}<div class="nt-empty">{tr('doctorEmpty')}</div>
{:else}
  <div class={`doctor-verdict ${severity}`}><div><span class="doctor-verdict-kicker">{lx('ВЕРДИКТ','VERDICT')}</span>
    <strong>{localized(verdictLabels,verdict.code || 'healthy')}</strong>
    <small>{verdict.fault_domain ? 'fault domain: '+verdict.fault_domain : lx('Критических отказов не обнаружено.','No critical failure detected.')}</small>
  </div><span class={`rf-badge ${tone(severity)}`}>{localized(statusNames,severity)}</span></div>
  <div class="doctor-chain">
    {#each diagnosis.stages || [] as stage, i (i)}
      <div class={`doctor-stage ${stage.status || ''}`}><span class="doctor-stage-dot"></span><div class="doctor-stage-copy"><strong>{localized(stages,stage.id)}</strong><small>{stage.detail || ''}</small></div>
        {#if Number(stage.duration_ms) > 0}<span class="doctor-stage-time">{stage.duration_ms} ms</span>{/if}
        <span class={`rf-badge ${tone(stage.status)}`}>{localized(statusNames,stage.status)}</span>
      </div>
    {/each}
  </div>
  {#if (diagnosis.actions || []).length}<div class="doctor-selected-title">{lx('Что проверить дальше','Next checks')}</div>
    {#each diagnosis.actions as action, i (i)}
      <div class="nt-result-row"><span class="nt-result-icon">›</span><strong>{localized(actionNames,action.id)}</strong><span class="mono">{action.fault_domain || ''}</span><span class={`rf-badge ${action.priority === 'high' ? 'bad' : action.priority === 'medium' ? 'warn' : ''}`}>{String(action.priority || '').toUpperCase()}</span><span class="nt-result-meta">{action.detail || ''}</span></div>
    {/each}
  {/if}
  {#if (data.checks || []).length}<div class="doctor-selected-title">{lx('Выбранные активные проверки','Selected active probes')}</div>
    {#each data.checks as kind, i (i)}
      {@const result = data.probes?.[kind] || {}}
      <div class="nt-result-row"><span class="nt-result-icon">{icons[kind] || '•'}</span><strong>{names[kind] || kind}</strong><span class="mono">{data.target || ''}</span><span class:bad={!result.ok} class="rf-badge">{result.ok ? tr('success') : tr('error')}</span><span class="nt-result-meta">{detail(kind,result)}</span><span class="nt-result-meta" title={result.error || (kind === 'ping' && Number.isFinite(+result.loss_pct) ? tr('lossWord')+' '+Number(result.loss_pct).toFixed(1)+'%' : '')}>›</span></div>
    {/each}
  {/if}
{/if}
