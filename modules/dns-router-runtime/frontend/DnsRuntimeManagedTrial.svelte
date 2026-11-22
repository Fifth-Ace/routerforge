<script>
  import { rfUi } from './rf-localization.js';
  import {createManagedFlow} from './DnsRuntimeManagedLogic.js';
  import {readDraft,feedback} from './DnsRuntimeStore.svelte.js';
  let busy = $state(false);
  let status = $state(rfUi('Применение запрещено до проверки текущего плана',"Application blocked until the current plan is verified"));
  let error = $state('');

  async function request(path, options = {}) {
    const response = await fetch('../' + path, {cache:'no-store', credentials:'same-origin', ...options});
    let body;
    try {body = await response.json();} catch {throw Error(rfUi('Некорректный JSON · ',"Invalid JSON ·") + path);}
    if (!response.ok) throw Error('HTTP ' + response.status + ': ' + (body?.error || 'unknown error'));
    return body;
  }
  function notify(message) {
    feedback(message);
  }
  const flow = createManagedFlow({
    readDraft,
    request,
    confirmAction: message => window.confirm(message),
    promptAction: message => window.prompt(message),
    notify
  });
  async function perform(action) {
    if (busy) return;
    busy = true;
    error = '';
    try { await (action === 'prepare' ? flow.prepare() : flow.trial()); }
    catch (e) { error = String(e?.message || e); notify(rfUi('Ошибка временного DNS-теста: ','Temporary DNS test error: ') + error); }
    finally { status = flow.snapshot().status; busy = false; }
  }
</script>
<div class="panel-actions">
  <button type="button" class="button" id="managedPrepareN" disabled={busy} onclick={() => perform('prepare')}>{rfUi("Проверить план и конфликты","Check plan and conflicts")}</button>
  <button type="button" class="button danger" id="managedTrialN" disabled={busy} onclick={() => perform('trial')}>{rfUi("Испытать с автоматическим откатом","Test with automatic rollback")}</button>
</div>
<pre class="console" id="managedStatusN">{status}</pre>
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
