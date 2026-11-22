<script>
  let { data = null, message = '', busy = false, lx, tr } = $props();
  const rtt = (hop,index) => hop.rtt_ms?.[index] == null ? '—' : Number(hop.rtt_ms[index]).toFixed(1)+' ms';
</script>
{#if message}<div class="rf-error">{message}</div>
{:else if busy}<div class="nt-empty">{tr('tracing')}</div>
{:else if !data}<div class="nt-empty">{tr('traceEmpty')}</div>
{:else if !(data.hops || []).length}<div class="nt-empty">{data.error || tr('traceEmpty')}</div>
{:else}<table><thead><tr><th>#</th><th>{tr('hop')}</th><th>RTT 1</th><th>RTT 2</th><th>RTT 3</th><th>{tr('lossWord')}</th><th>{tr('node')}</th></tr></thead><tbody>
  {#each data.hops as hop, i (i)}<tr><td><span class="hop-number">{hop.hop}</span></td><td class="mono">{hop.address || '*'}</td><td>{rtt(hop,0)}</td><td>{rtt(hop,1)}</td><td>{rtt(hop,2)}</td><td>{Number(hop.loss_pct || 0).toFixed(0)}%</td><td>{hop.label || ''}</td></tr>{/each}
</tbody></table>{/if}
