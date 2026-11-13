<script>
  let { section = 'summary', data = null, history = [], busy = false, message = '', lx, tr } = $props();
  const icons={ping:'⌁',dns:'◎',http:'▣',tcp:'◉'};
  const names={ping:'Ping',dns:'DNS',http:'HTTP',tcp:'TCP'};
  const detail=(kind,r) => {
    if (!r) return tr('noData');
    if (kind==='ping') return r.rtt_avg_ms?'RTT '+Number(r.rtt_avg_ms).toFixed(1)+' ms':tr('time')+' '+r.duration_ms+' ms';
    if (kind==='dns') return (r.addresses||[]).join(', ')||tr('time')+' '+r.duration_ms+' ms';
    if (kind==='http') return r.status_code?'HTTP '+r.status_code+' · '+r.duration_ms+' ms':tr('time')+' '+r.duration_ms+' ms';
    return tr('time')+' '+r.duration_ms+' ms';
  };
</script>
{#if section === 'history'}
  {#if !history.length}<div class="nt-empty">{lx('История проверок пока пуста.','Probe history is empty.')}</div>
  {:else}<table class="probe-history-table"><thead><tr><th>{tr('time')}</th><th>{lx('Тип','Kind')}</th><th>{lx('Цель','Target')}</th><th>{tr('status')}</th><th>{lx('Результат','Result')}</th></tr></thead><tbody>
    {#each [...history].reverse() as item, i (i)}{@const kind = item.result?.kind || ''}<tr><td class="mono">{item.at}</td><td>{names[kind]||kind}</td><td class="mono">{item.result?.target||'—'}</td><td><span class:bad={!item.result?.ok} class="rf-badge">{item.result?.ok?tr('success'):tr('error')}</span></td><td>{detail(kind,item.result)}</td></tr>{/each}
  </tbody></table>{/if}
{:else if message}<div class="rf-error">{message}</div>
{:else if busy}<div class="nt-empty">{lx('Проверка выполняется…','Probe is running…')}</div>
{:else if !data}<div class="nt-empty">{lx('Готово к запуску.','Ready to run.')}</div>
{:else}<div class:ok={Boolean(data.ok)} class:fail={!data.ok} class="probe-summary-card"><div><span>{icons[data.kind] || '•'}</span><strong>{names[data.kind] || data.kind}</strong><small class="mono">{data.target || '—'}</small></div><span class:bad={!data.ok} class="rf-badge">{data.ok?tr('success'):tr('error')}</span><div class="probe-summary-detail">{detail(data.kind,data)}</div></div>
{/if}
