<script>
  import {createManagedFlow} from './DnsRuntimeManagedLogic.js';
  import {readDraft,feedback} from './DnsRuntimeStore.svelte.js';
  let busy = $state(false);
  let status = $state('Применение запрещено до проверки текущего плана');
  let error = $state('');

  async function request(path, options = {}) {
    const response = await fetch('../' + path, {cache:'no-store', credentials:'same-origin', ...options});
    let body;
    try {body = await response.json();} catch {throw Error('Некорректный JSON · ' + path);}
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
    catch (e) { error = String(e?.message || e); notify('DPR2-N: ' + error); }
    finally { status = flow.snapshot().status; busy = false; }
  }
</script>
<div class="panel-actions">
  <button type="button" class="button" id="managedPrepareN" disabled={busy} onclick={() => perform('prepare')}>Проверить план и конфликты</button>
  <button type="button" class="button danger" id="managedTrialN" disabled={busy} onclick={() => perform('trial')}>Испытать с автоматическим откатом</button>
</div>
<pre class="console" id="managedStatusN">{status}</pre>
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
