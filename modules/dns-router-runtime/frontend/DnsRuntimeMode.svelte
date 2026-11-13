<script>
  import { onMount } from 'svelte';
  import {shell} from './DnsRuntimeStore.svelte.js';
  let mode = $derived(shell.mode);
  let busy = $state(false);
  let status = $state('Статус ещё не проверен');
  let error = $state('');
  let destroyed = false;
  let serial = 0;
  let activeRead = null;
  function selectMode(next) {
    if(next==='simple'||next==='advanced')shell.mode=next;
  }
  async function refresh() {
    if (busy) return;
    const n = ++serial;
    activeRead?.abort();
    const controller = new AbortController();
    activeRead = controller;
    busy = true;
    status = 'Читаем shadow DNS…'; error = '';
    try {
      const response = await fetch('../shadow/status', { credentials:'same-origin', cache:'no-store', headers:{Accept:'application/json'}, signal: controller.signal });
      if (!response.ok) throw Error('HTTP '+response.status);
      if (!(response.headers.get('content-type') || '').includes('application/json')) throw Error('Сервер не вернул JSON');
      const data = await response.json();
      if (!data || typeof data !== 'object' || Array.isArray(data)) throw Error('Неверный ответ shadow DNS');
      if (destroyed || n !== serial) return;
      status = 'Shadow DNS: ответ получен (без изменения настроек)';
    } catch(e) {if (!destroyed && n === serial) {status='Shadow DNS: состояние не подтверждено';error=e.message;}}
    finally {if (!destroyed && n === serial) busy=false; if (activeRead === controller) activeRead = null;}
  }
  onMount(() => {
    // No implicit mode change or router mutation at mount.

    void refresh();
    const interval=window.setInterval(()=>{if (!document.hidden) void refresh();},15000);
    return ()=>{destroyed=true;++serial;window.clearInterval(interval);activeRead?.abort();activeRead=null;};
  });
</script>
<div class="dr-mode-toggle" aria-label="Режим интерфейса">
  <button type="button" id="drModeSimple" aria-pressed={mode === 'simple'} onclick={() => selectMode('simple')}>Простой</button>
  <button type="button" id="drModeAdvanced" aria-pressed={mode === 'advanced'} onclick={() => selectMode('advanced')}>Расширенный</button>
</div>
<div class="state neutral" role="status" aria-live="polite" style="max-width:320px;white-space:normal">
  {status}
  {#if error}<small>{error}</small>{/if}
  <button type="button" disabled={busy} onclick={refresh} aria-label="Проверить состояние shadow DNS">↻</button>
</div>
