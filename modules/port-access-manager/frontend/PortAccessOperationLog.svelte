<script>
  import { onMount } from 'svelte';

  const endpoint = '/api/modules/port-access-manager/operation-logs';
  let lines = $state([]);
  let halted = $state(false);
  let busy = false;
  let seen = 0;
  let connection = $state('Подключаемся…');
  let listNode;

  async function refresh() {
    if (halted || busy || document.hidden) return;
    busy = true;
    try {
      const response = await fetch(endpoint, {
        credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' }
      });
      if (!response.ok) throw new Error('HTTP ' + response.status);
      const data = await response.json();
      if (!Array.isArray(data.entries) || data.scope !== 'operations') throw new Error('Неверный формат API');
      const next = [];
      for (const entry of data.entries) {
        if (!Number.isSafeInteger(entry.id) || entry.id <= seen) continue;
        seen = entry.id;
        next.push([entry.at, entry.engine, entry.action, entry.status].join('  '));
      }
      if (next.length) lines = [...lines, ...next].slice(-500);
      connection = 'Live · 2 сек. · записей: ' + data.entries.length;
    } catch (error) {
      connection = 'Ошибка связи: ' + error.message;
    } finally {
      busy = false;
    }
  }
  function togglePause() {
    halted = !halted;
    connection = halted ? 'Пауза' : 'Live · 2 сек.';
    if (!halted) void refresh();
  }
  function clearScreen() { lines = []; }
  async function copyScreen() {
    if (!navigator.clipboard?.writeText) {
      connection = 'Копирование недоступно в этом контексте';
      return;
    }
    try {
      await navigator.clipboard.writeText(lines.join('\n'));
      connection = 'Скопировано';
    } catch (_) { connection = 'Копирование отклонено браузером'; }
  }
  onMount(() => {
    void refresh();
    const interval = window.setInterval(refresh, 2000);
    return () => window.clearInterval(interval);
  });
  $effect(() => {
    lines;
    if (listNode) {
      // Keep the old auto-scroll contract after Svelte flushes the text.
      requestAnimationFrame(() => { if (listNode) listNode.scrollTop = listNode.scrollHeight; });
    }
  });
</script>

<section class="panel rf-pa-operation-log" aria-label="Журнал Port Access Manager">
  <h2>Журнал операций · Live</h2>
  <p class="help">Операции RouterForge: HTTP PASS/FAIL и попытки применения. SPA-ключи и конфигурации не записываются. Журнал хранится в памяти до перезапуска модуля, это не захват сетевых пакетов.</p>
  <div class="refresh-bar">
    <button type="button" id="pa-log-pause" onclick={togglePause}>{halted ? 'Продолжить' : 'Пауза'}</button>
    <button type="button" id="pa-log-copy" onclick={copyScreen}>Копировать</button>
    <button type="button" id="pa-log-clear" onclick={clearScreen}>Очистить экран</button>
    <span id="pa-log-state" role="status">{connection}</span>
  </div>
  <pre id="pa-log-lines" bind:this={listNode} tabindex="0" aria-label="Журнал операций" style="height:250px;max-height:45vh;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.5 monospace;border:1px solid var(--rf-border,#6665);border-radius:8px;padding:12px;">{lines.length ? lines.join('\n') : 'Событий пока нет.'}</pre>
</section>
