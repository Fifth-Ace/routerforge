<script>
  import { rfUi } from './rf-localization.js';
  import { onMount } from 'svelte';
  import { createPortAccessReadSession } from './PortAccessReadSession.js';
  const readSession = createPortAccessReadSession();

  const endpoint = '/api/modules/port-access-manager/operation-logs';
  let lines = $state([]);
  let halted = $state(false);
  let busy = false;
  let seen = 0;
  let connection = $state(rfUi('Подключаемся…',"Connecting…"));
  let listNode;
  let scrollFrame = 0;

  async function refresh() {
    if (halted || busy || document.hidden) return;
    busy = true;
    try {
      const data = await readSession.consume(endpoint, {
        credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' }
      }, async response => {
        if (!response.ok) throw new Error('HTTP ' + response.status);
        return response.json();
      });
      if (!Array.isArray(data.entries) || data.scope !== 'operations') throw new Error(rfUi('Неверный формат API',"Invalid API response format"));
      const next = [];
      for (const entry of data.entries) {
        if (!Number.isSafeInteger(entry.id) || entry.id <= seen) continue;
        seen = entry.id;
        next.push([entry.at, entry.engine, entry.action, entry.status].join('  '));
      }
      if (next.length) lines = [...lines, ...next].slice(-500);
      connection = rfUi('Live · 2 сек. · записей: ',"Live · 2 s · entries:") + data.entries.length;
    } catch (error) {
      connection = rfUi('Ошибка связи: ',"Connection error:") + error.message;
    } finally {
      busy = false;
    }
  }
  function togglePause() {
    halted = !halted;
    connection = halted ? rfUi('Пауза',"Pause") : rfUi('Live · 2 сек.',"Live · 2 s");
    if (!halted) void refresh();
  }
  function clearScreen() { lines = []; }
  async function copyScreen() {
    if (!navigator.clipboard?.writeText) {
      connection = rfUi('Копирование недоступно в этом контексте',"Copying is unavailable in this context");
      return;
    }
    try {
      await navigator.clipboard.writeText(lines.join('\n'));
      connection = rfUi('Скопировано',"Copied");
    } catch (_) { connection = rfUi('Копирование отклонено браузером',"Copying was denied by the browser"); }
  }
  onMount(() => {
    void refresh();
    const interval = window.setInterval(refresh, 2000);
    return () => { window.clearInterval(interval); readSession.close(); cancelAnimationFrame(scrollFrame); };
  });
  $effect(() => {
    lines;
    if (listNode) {
      // Keep the old auto-scroll contract after Svelte flushes the text.
      cancelAnimationFrame(scrollFrame);
      scrollFrame = requestAnimationFrame(() => { if (listNode?.isConnected) listNode.scrollTop = listNode.scrollHeight; });
    }
  });
</script>

<section class="panel rf-pa-operation-log" aria-label={rfUi("Журнал Port Access Manager","Port Access Manager activity log")}>
  <h2>{rfUi("Журнал операций · Live","Operation log · Live")}</h2>
  <p class="help">{rfUi("Операции RouterForge: HTTP PASS/FAIL и попытки применения. SPA-ключи и конфигурации не записываются. Журнал хранится в памяти до перезапуска модуля, это не захват сетевых пакетов.","RouterForge records operation results and attempted changes. SPA keys and configuration values are never logged. This in-memory log is cleared when the module restarts; it does not capture network packets.")}</p>
  <div class="refresh-bar">
    <button type="button" id="pa-log-pause" onclick={togglePause}>{halted ? 'Продолжить' : 'Пауза'}</button>
    <button type="button" id="pa-log-copy" onclick={copyScreen}>{rfUi("Копировать","Copy")}</button>
    <button type="button" id="pa-log-clear" onclick={clearScreen}>{rfUi("Очистить экран","Clear screen")}</button>
    <span id="pa-log-state" role="status">{connection}</span>
  </div>
  <pre id="pa-log-lines" bind:this={listNode} tabindex="0" aria-label={rfUi("Журнал операций","Operation log")} style="height:250px;max-height:45vh;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.5 monospace;border:1px solid var(--rf-border,#6665);border-radius:8px;padding:12px;">{lines.length ? lines.join('\n') : 'Событий пока нет.'}</pre>
</section>
