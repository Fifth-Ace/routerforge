<script>
  let { section = 'table', list = [], selected = null, limit = 80, source = '', enrichment = {}, message = '', lx, tr, fmtBytes, flowKey, onSelect = () => {} } = $props();
  let visible = $derived(list.slice(0,limit));
  let totalBytes = $derived(list.reduce((sum, f) => sum+Number(f.bytes||0),0));
  let tcp = $derived(list.filter(f => String(f.protocol||'').toUpperCase()==='TCP').length);
  let udp = $derived(list.filter(f => String(f.protocol||'').toUpperCase()==='UDP').length);
  let natCount = $derived(list.filter(f => f._raw?.nat?.detected).length);
  let ownedCount = $derived(list.filter(f => f.pid).length);
  let raw = $derived(selected?._raw || {});
  let original = $derived(raw.original || {});
  let reply = $derived(raw.reply || {});
  let socket = $derived(raw.socket || {});
  let policy = $derived(raw.policy || {});
  let route = $derived(policy.kernel_decision?.available ? policy.kernel_decision : raw.route || {});
  let nat = $derived(raw.nat || {});
  let candidates = $derived(policy.candidates || []);
  let details = $derived(selected ? [
    [lx('Исходный tuple','Original tuple'),(original.source||'—')+(original.source_port?':'+original.source_port:'')+' → '+(original.destination||'—')+(original.destination_port?':'+original.destination_port:'')],
    [lx('Ответный tuple','Reply tuple'),(reply.source||'—')+(reply.source_port?':'+reply.source_port:'')+' → '+(reply.destination||'—')+(reply.destination_port?':'+reply.destination_port:'')],
    ['NAT',nat.detected?(nat.types||[]).join(' + ')||'detected':'none'],
    [lx('Эффективное назначение','Effective destination'),raw.effective_destination||original.destination||'—'],
    ['Mark',raw.mark||'—'],['PID / Process',socket.available?(socket.process||'—')+' · '+(socket.pid||'—'):(socket.reason||'unattributed')],
    [lx('Socket evidence','Socket evidence'),socket.match||socket.source||'—'],
    [lx('Маршрут','Route'),route.available?(route.interface||'—')+' · table '+(route.table||'main')+(route.gateway?' · via '+route.gateway:''):(route.reason||route.error||'unavailable')],
    [lx('Policy candidates','Policy candidates'),String(candidates.length)],['Policy source',policy.source||'unavailable']
  ] : []);
  function selectKeydown(event, flow) { if(event.key==='Enter' || event.key===' ') {event.preventDefault(); onSelect(flow);} }
</script>
{#if section === 'summary'}
  <span><b>{list.length}</b>{lx(' потоков',' flows')}</span><span>TCP <b>{tcp}</b></span><span>UDP <b>{udp}</b></span><span>NAT <b>{natCount}</b></span><span>PID <b>{ownedCount}</b></span>
  <span>{lx('Объём','Volume')} <b>{fmtBytes(totalBytes)}</b></span><span class="mono">{source||'unavailable'}</span>
  <span class="mono">sockets {enrichment.socket_records||0}/4096</span><span class="mono">fd {enrichment.owner_fds_scanned||0}/16384</span>
  <span class="mono">route-get {enrichment.route_lookups||0}/{enrichment.route_lookup_budget||32}</span>
{:else if section === 'detail'}
  {#if !selected}<div class="flow-detail-empty">{lx('Выберите строку потока для деталей.','Select a flow row to inspect details.')}</div>
  {:else}<div class="flow-detail-grid">{#each details as item, i (i)}<div class="flow-detail-item"><span>{item[0]}</span><strong class="mono">{item[1]}</strong></div>{/each}</div>
    {#if candidates.length}<div class="flow-policy-list">{#each candidates.slice(0,16) as candidate, i (i)}<span class:ok={candidate.match==='matched-known-fields'} class:off={candidate.match!=='matched-known-fields'} class="route-cap">{String(candidate.priority)+' · '+(candidate.mark||'no-mark')+' → '+(candidate.table||candidate.match||'—')}</span>{/each}</div>{/if}
  {/if}
{:else}
  {#if message}<div class="rf-error">{message}</div>
  {:else if !visible.length}<div class="nt-empty">{tr('noFlows')}</div>
  {:else}<table class="flow-data-table"><thead><tr><th>{tr('source')}</th><th>{tr('destination')}</th><th>{tr('protocol')}</th><th>{tr('flowState')}</th><th>Mark</th><th>PID / Process</th><th>{lx('Маршрут','Route')}</th><th>{lx('Пакеты','Packets')}</th><th>{tr('volume')}</th><th>TTL</th></tr></thead><tbody>
    {#each visible as flow, i (i)}<!-- svelte-ignore a11y_no_noninteractive_element_interactions a11y_no_noninteractive_tabindex --><tr class:selected={selected && flowKey(selected)===flowKey(flow)} class="flow-row-selectable" tabindex="0" onclick={() => onSelect(flow)} onkeydown={event => selectKeydown(event,flow)} aria-selected={Boolean(selected && flowKey(selected)===flowKey(flow))}>
      <td class="mono">{flow.source || '—'}{flow.source_port ? ':'+flow.source_port : ''}</td><td class="mono">{flow.destination || '—'}{flow.destination_port ? ':'+flow.destination_port : ''}</td><td>{flow.protocol||'—'}</td><td><span class:neutral={flow.state !== 'ESTABLISHED' && flow.state !== 'ACTIVE'} class="rf-badge">{flow.state||'ACTIVE'}</span></td>
      <td class="mono">{flow.mark||'—'}</td><td class="mono">{flow.pid?(flow.process||'pid')+' · '+flow.pid:'—'}</td><td class="mono">{flow.route_interface||'—'}</td><td class="num">{flow.packets||0}</td><td class="num">{fmtBytes(flow.bytes||0)}</td><td class="num">{flow.timeout_seconds||0} s</td>
    </tr>{/each}
  </tbody></table>{/if}
{/if}
