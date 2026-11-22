<script>
  let { data = null, message = '', lx, fmtBytes } = $props();
  let interfaces = $derived((data?.interfaces || []).slice().sort((a,b) => Number(Boolean(b.default_route)) - Number(Boolean(a.default_route)) || String(a.name||'').localeCompare(String(b.name||''))));
  const isUp = i => { const flags=String(i.flags||'').toLowerCase(),s=String(i.operstate||'').toLowerCase(); return s!=='down' && s!=='lowerlayerdown' && s!=='dormant' && (s==='up' || flags.includes('up')); };
  const link = i => [Number(i.speed_mbps)>0 ? Number(i.speed_mbps).toLocaleString()+' Mbps' : '',i.duplex || '','MTU '+Number(i.mtu||0)].filter(Boolean).join(' · ');
</script>
{#if message}<div class="rf-error">{message}</div>
{:else if !data}<div class="nt-empty">Loading interfaces…</div>
{:else if !interfaces.length}<div class="nt-empty">{lx('Интерфейсы не обнаружены.','Interfaces are unavailable.')}</div>
{:else}
  <table class="interfaces-table"><thead><tr><th>{lx('Интерфейс','Interface')}</th><th>{lx('Состояние','State')}</th><th>{lx('Адреса','Addresses')}</th><th>{lx('Линк','Link')}</th><th>RX / TX</th><th>{lx('Ошибки / потери','Errors / drops')}</th><th>{lx('Маршрут','Route')}</th></tr></thead><tbody>
    {#each interfaces as iface, i (i)}
      <tr><td class="interface-name"><strong>{iface.name || '—'}</strong>{#if iface.alias}<small>{iface.alias}</small>{/if}{#if iface.hardware_addr}<small class="mono">{iface.hardware_addr}</small>{/if}</td>
        <td><span class:bad={!isUp(iface)} class="rf-badge">{isUp(iface) ? lx('В сети','UP') : lx('Не в сети','DOWN')}</span><small>{iface.operstate || 'unknown'}</small></td>
        <td>{#if (iface.addresses || []).length}{#each iface.addresses as addr, j (j)}<span class="mono interface-address">{addr}</span>{/each}{:else}—{/if}</td>
        <td>{link(iface)}</td><td class="mono">{fmtBytes(iface.rx_bytes||0)} / {fmtBytes(iface.tx_bytes||0)}</td>
        <td class="mono">{Number(iface.rx_errors||0) + Number(iface.tx_errors||0)} / {Number(iface.rx_dropped||0) + Number(iface.tx_dropped||0)}</td>
        <td>{#if iface.default_route}<span class="interface-default">{lx('По умолчанию','Default')}</span><small class="mono">{iface.default_gateway || '—'} · metric {iface.default_metric || 0}</small>{:else}—{/if}</td>
      </tr>
    {/each}
  </tbody></table>
{/if}
