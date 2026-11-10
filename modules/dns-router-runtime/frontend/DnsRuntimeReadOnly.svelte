<script>
  import {readDraft,feedback as showFeedback} from './DnsRuntimeStore.svelte.js';
  let {kind} = $props();
  let busy = $state(false);
  let output = $state('');
  let secondary = $state('');
  let evidence = $state('');
  let error = $state('');
  const initial = {
    listener: 'Сначала сохрани конфигурацию.',
    ingress: 'Сначала сохрани конфигурацию, затем запусти аудит.',
    ownership: 'Постоянные зоны не включены',
    discovery: 'Нет измерений',
    hardware: 'Аппаратная совместимость не подтверждена',
    upstreams: 'Нет измерений'
  };
  // Re-evaluate defaults only if the mounted component's kind prop changes.
  // Explicit probe results are never overwritten by unrelated draft updates.
  $effect(() => {
    output = initial[kind] || '';
    secondary = kind === 'upstreams'
      ? 'Native Bridge: статус ещё не запрошен. Режим read-only; готовность привязки не означает доступный интернет.'
      : '';
    error = '';
  });

  function feedback(message) {
    showFeedback(message);
  }
  function revisionGate() {
    const state = readDraft();
    if (!state || typeof state.revision !== 'string' || !state.revision) {
      throw Error('Конфигурация ещё не загружена');
    }
    if (state.dirty) throw Error('Для проверки сначала сохрани конфигурацию');
    return state.revision;
  }
  async function request(path, options = {}) {
    const response = await fetch('../' + path, {cache:'no-store', credentials:'same-origin', ...options});
    let body;
    try {body = await response.json();} catch {throw Error('Некорректный JSON · ' + path);}
    if (!response.ok) throw Error('HTTP ' + response.status + ': ' + (body?.error || 'unknown error'));
    return body;
  }
  async function run(action) {
    if (busy) return;
    busy = true; error = '';
    try {
      if (kind === 'listener') {
        const revision = revisionGate();
        const data = await request('keenetic/domain-plan');
        if (data.revision !== revision) throw Error('Конфигурация обновлена другим клиентом');
        output = JSON.stringify(data.plan, null, 2);
        feedback('План read-only готов · DNS Keenetic не изменён');
      } else if (kind === 'ingress') {
        const revision = revisionGate();
        output = action === 'live' ? 'Чтение текущих записей Keenetic…' : 'Анализ сохранённых правил…';
        const data = await request('keenetic/ingress-audit' + (action === 'live' ? '?live=1' : ''));
        if (data.revision !== revision) throw Error('Ревизия изменилась: повтори проверку');
        output = JSON.stringify(data.audit, null, 2);
        const audit = data.audit;
        feedback('P7: покрыто ' + audit.covered + '/' + audit.rule_count + '; вне маршрутов ' + audit.unmapped + '; конфликты ' + audit.conflicts.length + '; DNS не изменён');
      } else if (kind === 'ownership') {
        output = JSON.stringify(await request('keenetic/managed-ownership/status'), null, 2);
        feedback('Реестр проверен без изменения DNS Keenetic');
      } else if (kind === 'discovery') {
        const paths = {
          resources: 'runtime/resources',
          keenetic: 'keenetic/discovery',
          integration: 'keenetic/integration-plan',
          hardware: 'hardware/evidence'
        };
        if (!Object.prototype.hasOwnProperty.call(paths, action)) throw Error('READ_ONLY_ACTION_UNKNOWN');
        output = JSON.stringify(await request(paths[action]), null, 2);
      } else if (kind === 'hardware') {
        if (action === 'analyze') {
          if (!evidence.trim()) throw Error('Требуется диагностический отчёт');
          const result = await request('keenetic/assess-hardware', {
            method: 'POST',
            headers: {'Content-Type':'application/json', 'X-RouterForge-Action':'assess-hardware-readonly'},
            body: JSON.stringify({evidence})
          });
          output = JSON.stringify(result, null, 2);
          feedback('DPR2-G: отчёт обработан только в памяти; активация DNS Proxy запрещена');
        } else {
          output = JSON.stringify(await request('keenetic/rehearsal?failure=verify'), null, 2);
          feedback('DPR2-G: исключительно симуляция in-memory, конфигурация Keenetic не менялась');
        }
      } else if (kind === 'upstreams') {
        if (action === 'status') {
          output = JSON.stringify(await request('upstreams/status'), null, 2);
        } else {
          secondary = JSON.stringify(await request('keenetic/native-bridge/status'), null, 2);
          feedback('Keenetic Native: capability и привязки (RO); egress не подтверждён');
        }
      } else {
        throw Error('READ_ONLY_KIND_UNKNOWN');
      }
    } catch (e) {
      error = String(e?.message || e);
      feedback('Диагностика: ' + error);
      if (kind === 'ingress') output = 'Ошибка P7: ' + error;
    } finally {
      busy = false;
    }
  }
</script>

{#if kind === 'listener'}
  <div class="panel-actions"><button type="button" class="button" id="listenerPlanButton" disabled={busy} onclick={() => run('plan')}>План доменных зон (read-only)</button></div>
  <pre class="console" id="listenerPlanStatus">{output}</pre>
{:else if kind === 'ingress'}
  <div class="panel-actions">
    <button type="button" class="button" id="ingressOfflineP7" disabled={busy} onclick={() => run('offline')}>Проверить покрытие DNS-зон</button>
    <button type="button" class="button" id="ingressLiveP7" disabled={busy} onclick={() => run('live')}>Сверить с Keenetic (только чтение)</button>
  </div>
  <pre class="console" id="ingressAuditStatusP7">{output}</pre>
{:else if kind === 'ownership'}
  <div class="panel-actions"><button type="button" class="button" id="managedOwnershipStatusO" disabled={busy} onclick={() => run('status')}>Проверить реестр владения</button></div>
  <pre class="console" id="managedOwnershipResultO">{output}</pre>
{:else if kind === 'discovery'}
  <div class="panel-actions">
    <button type="button" id="resourcesButton" class="button" disabled={busy} onclick={() => run('resources')}>Память / PSS / RSS</button>
    <button type="button" id="keeneticButton" class="button" disabled={busy} onclick={() => run('keenetic')}>Обнаружение Keenetic</button>
    <button type="button" id="integrationButton" class="button" disabled={busy} onclick={() => run('integration')}>План DNS Proxy</button>
    <button type="button" id="hardwareButton" class="button" disabled={busy} onclick={() => run('hardware')}>Hardware evidence</button>
  </div>
  <pre class="console" id="discoveryStatus">{output}</pre>
{:else if kind === 'hardware'}
  <form class="form-grid" id="evidenceForm" onsubmit={(e) => {e.preventDefault(); void run('analyze');}}>
    <label class="span-all"><span>Текст диагностического отчёта</span><textarea name="evidence" rows="9" maxlength="15000" required bind:value={evidence} placeholder={'RF_DPR2G_EVIDENCE_BEGIN\nSCHEMA=1\nMODE=READ_ONLY\n...\nRF_DPR2G_EVIDENCE_END'}></textarea></label>
    <div class="form-actions"><button type="submit" class="button primary" disabled={busy}>Разобрать отчёт</button></div>
  </form>
  <div class="panel-actions"><button type="button" class="button" id="rehearsalButton" disabled={busy} onclick={() => run('rehearsal')}>Репетиция отказа / отката (in-memory)</button></div>
  <pre class="console" id="assessmentStatus">{output}</pre>
{:else if kind === 'upstreams'}
  <div class="panel-actions">
    <button type="button" id="upstreamStatusButton" class="button" disabled={busy} onclick={() => run('status')}>Статистика upstream (без проб)</button>
    <button type="button" id="nativeBridgePreviewButton" class="button" disabled={busy} onclick={() => run('native')}>Keenetic Native: проверить привязки (RO)</button>
  </div>
  <pre class="console" id="upstreamStatus">{output}</pre>
  <pre class="console" id="p85NativeStatus">{secondary}</pre>
{/if}
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
