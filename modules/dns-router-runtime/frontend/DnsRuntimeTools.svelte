<script>
  import {evaluateDomain, exportSavedConfig} from './DnsRuntimeToolsLogic.js';
  let {kind} = $props();
  let busy = $state(false);
  let domain = $state('');
  let output = $state('Ожидание запроса');
  let error = $state('');
  async function request(path, options = {}) {
    const response = await fetch('../' + path, {cache:'no-store', credentials:'same-origin', ...options});
    let body;
    try {body = await response.json();} catch {throw Error('Некорректный JSON · ' + path);}
    if (!response.ok) throw Error('HTTP ' + response.status + ': ' + (body?.error || 'unknown error'));
    return body;
  }
  function notify(message) {
    const element = document.getElementById('feedback');
    if (element) element.textContent = message;
  }
  async function perform() {
    if (busy) return;
    busy = true; error = '';
    try {
      if (kind === 'evaluate') {
        output = JSON.stringify(await evaluateDomain(domain, request), null, 2);
      } else if (kind === 'export') {
        await exportSavedConfig(request, {BlobConstructor: Blob, URLObject: URL, documentObject: document});
        notify('Экспортирована сохранённая конфигурация');
      } else { throw Error('UNKNOWN_TOOL'); }
    } catch(e) {error = String(e?.message || e);notify(error);}
    finally {busy = false;}
  }
</script>
{#if kind === 'evaluate'}
  <form id="evaluateForm" class="form-grid form-search" onsubmit={(e) => {e.preventDefault(); void perform();}}>
    <label><span>Домен для проверки</span><input name="domain" required placeholder="api.example.com" bind:value={domain}></label>
    <div class="form-actions"><button class="button primary" type="submit" disabled={busy}>Проверить правило</button></div>
  </form>
  <pre class="console" id="decision">{output}</pre>
{:else if kind === 'export'}
  <button id="exportButton" type="button" class="button" disabled={busy} onclick={perform}>Экспорт JSON</button>
{/if}
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
