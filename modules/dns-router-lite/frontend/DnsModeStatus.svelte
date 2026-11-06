<script>
  import { onMount } from 'svelte';
  let mode = $state('basic');
  let status = $state('Статус ещё не проверен');
  let busy = $state(false);
  let active = $state(false);
  let actualMode = $state('неизвестен');
  let listener = $state('—');
  let error = $state('');
  let destroyed = false;
  let serial = 0;
  const native = () => document.getElementById('p21Mode');
  const app = () => document.getElementById('dnsLiteApp');

  function syncMode() {
    const m = native()?.value || app()?.dataset.dnsMode || 'basic';
    mode = m === 'advanced' ? 'advanced' : 'basic';
  }
  function selectMode(next) {
    const select = native();
    if (!select || (next !== 'basic' && next !== 'advanced')) return;
    select.value = next;
    select.dispatchEvent(new Event('change', { bubbles:true }));
    syncMode();
  }
  async function refresh() {
    if (busy) return;
    const n = ++serial;
    busy = true;
    status = 'Читаем состояние DNS…';
    error = '';
    try {
      const response = await fetch('/api/modules/dns-router/service/status', {
        method:'GET', credentials:'same-origin', cache:'no-store', headers:{Accept:'application/json'}
      });
      if (!response.ok) throw new Error('HTTP ' + response.status);
      if (!(response.headers.get('content-type') || '').includes('application/json')) throw new Error('Сервер не вернул JSON');
      const data = await response.json();
      if (data.mode !== 'independent-dns' || typeof data.active !== 'boolean') throw new Error('Неверный контракт DNS Lite');
      if (destroyed || n !== serial) return;
      active = data.active;
      actualMode = data.mode;
      listener = data.active ? String(data.listen || 'неизвестно') : 'Не запущен';
      status = data.active ? 'Сервис работает' : 'Сервис остановлен';
      error = String(data.last_error || '');
    } catch (e) {
      if (destroyed || n !== serial) return;
      active = false;
      actualMode = 'не подтверждён';
      listener = '—';
      status = 'Статус недоступен';
      error = e.message;
    } finally { if (!destroyed && n === serial) busy = false; }
  }
  onMount(() => {
    syncMode();
    const select = native();
    const root = app();
    select?.addEventListener('change', syncMode);
    const observer = typeof MutationObserver !== 'undefined' && root ? new MutationObserver(syncMode) : null;
    observer?.observe(root, { attributes:true, attributeFilter:['data-dns-mode'] });
    void refresh();
    const interval = window.setInterval(() => { if (!document.hidden) void refresh(); }, 15000);
    return () => {destroyed = true; ++serial; select?.removeEventListener('change', syncMode); observer?.disconnect();window.clearInterval(interval);};
  });
</script>
<div class="p22-mode-switch rfds-segmented" role="group" aria-label="Режим DNS Router">
  <button id="p22ModeBasic" type="button" class:selected={mode === 'basic'} aria-pressed={mode === 'basic'} onclick={() => selectMode('basic')}>Обычный</button>
  <button id="p22ModeAdvanced" type="button" class:selected={mode === 'advanced'} aria-pressed={mode === 'advanced'} onclick={() => selectMode('advanced')}>Расширенный</button>
</div>
<span class="status" role="status" aria-live="polite" title={error || 'Только чтение, без изменения DNS'}>{status} · {listener}</span>
<button class="button subtle" type="button" disabled={busy} onclick={refresh} aria-label="Проверить DNS без изменения настроек">↻ DNS</button>
