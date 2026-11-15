<script>
  let {mode = 'basic', status = null, busy = false, onMode = () => {}, onRefresh = () => {}} = $props();
  let error = $derived(String(status?.last_error || ''));
  let text = $derived(!status ? 'Статус ещё не проверен' : status.active ? 'Сервис работает' : 'Сервис остановлен');
  let listener = $derived(status?.active ? String(status.listen || 'неизвестно') : 'Не запущен');
</script>
<div class="dns-toolbar-mode">
  <span class="dns-toolbar-label">Режим интерфейса</span>
  <div class="p22-mode-switch rfds-segmented" role="group" aria-label="Режим DNS Router">
    <button id="p22ModeBasic" type="button" class:selected={mode === 'basic'} aria-pressed={mode === 'basic'} onclick={() => onMode('basic')}>Обычный</button>
    <button id="p22ModeAdvanced" type="button" class:selected={mode === 'advanced'} aria-pressed={mode === 'advanced'} onclick={() => onMode('advanced')}>Расширенный</button>
  </div>
</div>
<div class="dns-toolbar-runtime">
  <span class="dns-toolbar-label">DNS-сервис</span>
  <div class="dns-toolbar-runtime-body">
    <span class="status" role="status" aria-live="polite" title={error || 'Только чтение, без изменения DNS'}>{text} · {listener}</span>
    <button class="button subtle dns-toolbar-check" type="button" disabled={busy} onclick={onRefresh} aria-label="Проверить DNS без изменения настроек">↻ Проверить</button>
  </div>
</div>
