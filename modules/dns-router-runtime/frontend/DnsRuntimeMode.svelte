<script>
  import { rfUi } from './rf-localization.js';
  import { onMount } from 'svelte';
  import {shell} from './DnsRuntimeStore.svelte.js';
  let mode = $derived(shell.mode);
  let busy = $state(false);
  let status = $state(rfUi('Статус ещё не проверен',"Status not checked yet"));
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
    status = rfUi('Читаем shadow DNS…',"Loading test DNS service…"); error = '';
    try {
      const response = await fetch('../shadow/status', { credentials:'same-origin', cache:'no-store', headers:{Accept:'application/json'}, signal: controller.signal });
      if (!response.ok) throw Error('HTTP '+response.status);
      if (!(response.headers.get('content-type') || '').includes('application/json')) throw Error('Сервер не вернул JSON');
      const data = await response.json();
      if (!data || typeof data !== 'object' || Array.isArray(data)) throw Error(rfUi('Неверный ответ shadow DNS',"Invalid test DNS service response"));
      if (destroyed || n !== serial) return;
      status = rfUi('Shadow DNS: ответ получен (без изменения настроек)',"Test DNS: response received (no settings changed)");
    } catch(e) {if (!destroyed && n === serial) {status=rfUi('Shadow DNS: состояние не подтверждено',"Test DNS service: state not verified");error=e.message;}}
    finally {if (!destroyed && n === serial) busy=false; if (activeRead === controller) activeRead = null;}
  }
  onMount(() => {
    // No implicit mode change or router mutation at mount.

    void refresh();
    const interval=window.setInterval(()=>{if (!document.hidden) void refresh();},15000);
    return ()=>{destroyed=true;++serial;window.clearInterval(interval);activeRead?.abort();activeRead=null;};
  });
</script>
<div class="dr-mode-toggle" aria-label={rfUi("Режим интерфейса","Interface mode")}>
  <button type="button" id="drModeSimple" aria-pressed={mode === 'simple'} onclick={() => selectMode('simple')}>{rfUi("Простой","Basic")}</button>
  <button type="button" id="drModeAdvanced" aria-pressed={mode === 'advanced'} onclick={() => selectMode('advanced')}>{rfUi("Расширенный","Advanced")}</button>
</div>
<div class="state neutral" role="status" aria-live="polite" style="max-width:320px;white-space:normal">
  {status}
  {#if error}<small>{error}</small>{/if}
  <button type="button" disabled={busy} onclick={refresh} aria-label={rfUi("Проверить состояние shadow DNS","Check test DNS service status")}>↻</button>
</div>
