<script>
  import { rfUi } from './rf-localization.js';
  let {mode = 'basic', status = null, busy = false, onMode = () => {}, onRefresh = () => {}} = $props();
  let error = $derived(String(status?.last_error || ''));
  let text = $derived(!status ? rfUi('Статус ещё не проверен',"Status not checked yet") : status.active ? 'Сервис работает' : 'Сервис остановлен');
  let listener = $derived(status?.active ? String(status.listen || 'неизвестно') : rfUi('Не запущен',"Not running"));
</script>
<div class="dns-toolbar-mode">
  <span class="dns-toolbar-label">{rfUi("Режим интерфейса","Interface mode")}</span>
  <div class="p22-mode-switch rfds-segmented" role="group" aria-label={rfUi("Режим DNS Router","DNS Router mode")}>
    <button id="p22ModeBasic" type="button" class:selected={mode === 'basic'} aria-pressed={mode === 'basic'} onclick={() => onMode('basic')}>{rfUi("Обычный","Basic")}</button>
    <button id="p22ModeAdvanced" type="button" class:selected={mode === 'advanced'} aria-pressed={mode === 'advanced'} onclick={() => onMode('advanced')}>{rfUi("Расширенный","Advanced")}</button>
  </div>
</div>
<div class="dns-toolbar-runtime">
  <span class="dns-toolbar-label">{rfUi("DNS-сервис","DNS service")}</span>
  <div class="dns-toolbar-runtime-body">
    <span class="status" role="status" aria-live="polite" title={error || 'Только чтение, без изменения DNS'}>{text} · {listener}</span>
    <button class="button subtle dns-toolbar-check" type="button" disabled={busy} onclick={onRefresh} aria-label={rfUi("Проверить DNS без изменения настроек","Test DNS without changing settings")}>{rfUi("↻ Проверить","↻ Check")}</button>
  </div>
</div>
