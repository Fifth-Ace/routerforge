<script>
  import { onDestroy } from 'svelte';
  import { markUnauthorized } from '$lib/stores/auth.js';
  export let locale = 'ru';
  let busy=false, sample=null, error='', controller=null, alive=true;
  $: ru=locale!=='en';
  const bytes = v => v==null?'—':`${(v/1048576).toFixed(1)} MiB`;
  async function refresh(){if(busy)return;busy=true;error='';controller=new AbortController();const timer=setTimeout(()=>controller.abort(),10000);
    try{const r=await fetch('/api/platform/foundation/local-metrics',{cache:'no-store',redirect:'error',signal:controller.signal});if(r.status===401)markUnauthorized();if(!r.ok)throw Error('HTTP '+r.status);const b=await r.json();if(b.schemaVersion!==1||b.observerScope!=='installation'||!b.metrics)throw Error('INVALID_METRICS');if(alive)sample=b;}
    catch(e){if(alive){error=String(e.message||e);sample=null;}}finally{clearTimeout(timer);if(alive)busy=false;}}
  onDestroy(()=>{alive=false;controller?.abort()});
</script>
<section class="panel foundation-local">
  <div class="local-head"><strong>{ru?'Общий формат метрик Core':'Core metrics contract'}</strong><button type="button" class="button" onclick={refresh} disabled={busy}>{ru?'Получить снимок':'Read sample'}</button></div>
  <small>{ru?'Локальное наблюдение, без фонового опроса. ID наблюдателя хранится локально и сохраняется после перезапуска Core.':'Local observation, no background polling. Observer ID is stored locally and persists across Core restarts.'}</small>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if sample}<div class="local-values"><span>CPU: {sample.metrics.cpuPercent==null?'—':sample.metrics.cpuPercent.toFixed(1)+'%'}</span><span>RAM: {bytes(sample.metrics.memory?.usedBytes)} / {bytes(sample.metrics.memory?.totalBytes)}</span><span>Swap: {bytes(sample.metrics.swap?.usedBytes)} / {bytes(sample.metrics.swap?.totalBytes)}</span><span>/opt: {bytes(sample.metrics.filesystems?.[0]?.usedBytes)} / {bytes(sample.metrics.filesystems?.[0]?.totalBytes)}</span><span>Uptime: {sample.metrics.uptimeSec??'—'} s</span></div><small>{sample.observedAt} · {ru?'наблюдатель':'observer'} {sample.observerId}</small>{/if}
</section>
<style>.foundation-local{margin:0 0 12px;padding:12px}.local-head,.local-values{display:flex;flex-wrap:wrap;gap:12px;align-items:center}.local-head{justify-content:space-between}.local-values{margin:8px 0}.foundation-local small{overflow-wrap:anywhere}</style>
