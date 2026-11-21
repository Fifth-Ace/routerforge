<script>
  import { rfUi } from './rf-localization.js';
  import {readDraft,feedback as showFeedback} from './DnsRuntimeStore.svelte.js';
  let {kind} = $props();
  let busy = $state(false);
  let output = $state('');
  let secondary = $state('');
  let evidence = $state('');
  let error = $state('');
  const initial = {
    listener: rfUi('Сначала сохрани конфигурацию.',"Save the configuration first."),
    ingress: rfUi('Сначала сохрани конфигурацию, затем запусти аудит.',"Save the configuration before running the audit."),
    ownership: rfUi('Постоянные зоны не включены',"Permanent zones not enabled"),
    discovery: rfUi('Нет измерений',"No measurements"),
    hardware: rfUi('Аппаратная совместимость не подтверждена',"Hardware compatibility not verified"),
    upstreams: rfUi('Нет измерений',"No measurements")
  };
  // Re-evaluate defaults only if the mounted component's kind prop changes.
  // Explicit probe results are never overwritten by unrelated draft updates.
  $effect(() => {
    output = initial[kind] || '';
    secondary = kind === 'upstreams'
      ? rfUi('Native Bridge: статус ещё не запрошен. Режим read-only; готовность привязки не означает доступный интернет.',"Native bridge: status not requested. Read-only mode; a ready binding does not guarantee internet access.")
      : '';
    error = '';
  });

  function feedback(message) {
    showFeedback(message);
  }
  function revisionGate() {
    const state = readDraft();
    if (!state || typeof state.revision !== 'string' || !state.revision) {
      throw Error(rfUi('Конфигурация ещё не загружена',"Configuration has not loaded yet"));
    }
    if (state.dirty) throw Error(rfUi('Для проверки сначала сохрани конфигурацию',"Save the configuration before testing"));
    return state.revision;
  }
  async function request(path, options = {}) {
    const response = await fetch('../' + path, {cache:'no-store', credentials:'same-origin', ...options});
    let body;
    try {body = await response.json();} catch {throw Error(rfUi('Некорректный JSON · ',"Invalid JSON ·") + path);}
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
        if (data.revision !== revision) throw Error(rfUi('Конфигурация обновлена другим клиентом',"Configuration was updated by another client"));
        output = JSON.stringify(data.plan, null, 2);
        feedback(rfUi('План read-only готов · DNS Keenetic не изменён',"Read-only plan ready · Keenetic DNS unchanged"));
      } else if (kind === 'ingress') {
        const revision = revisionGate();
        output = action === 'live' ? rfUi('Чтение текущих записей Keenetic…',"Reading current Keenetic entries…") : rfUi('Анализ сохранённых правил…',"Analyzing saved rules…");
        const data = await request('keenetic/ingress-audit' + (action === 'live' ? '?live=1' : ''));
        if (data.revision !== revision) throw Error(rfUi('Ревизия изменилась: повтори проверку',"Revision changed: repeat the check"));
        output = JSON.stringify(data.audit, null, 2);
        const audit = data.audit;
        feedback(rfUi('Покрыто DNS-правил: ','Covered DNS rules: ')+audit.covered+'/'+audit.rule_count+rfUi('; вне маршрутов ','; not routed ')+audit.unmapped+rfUi('; конфликты ','; conflicts ')+audit.conflicts.length+rfUi('; DNS не изменён','; DNS unchanged'));
      } else if (kind === 'ownership') {
        output = JSON.stringify(await request('keenetic/managed-ownership/status'), null, 2);
        feedback(rfUi('Реестр проверен без изменения DNS Keenetic',"Registry checked without changing Keenetic DNS"));
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
          if (!evidence.trim()) throw Error(rfUi('Требуется диагностический отчёт',"A diagnostic report is required"));
          const result = await request('keenetic/assess-hardware', {
            method: 'POST',
            headers: {'Content-Type':'application/json', 'X-RouterForge-Action':'assess-hardware-readonly'},
            body: JSON.stringify({evidence})
          });
          output = JSON.stringify(result, null, 2);
          feedback(rfUi('Отчёт обработан только в памяти; запуск DNS Proxy запрещён','The report was processed in memory only; DNS Proxy activation is blocked'));
        } else {
          output = JSON.stringify(await request('keenetic/rehearsal?failure=verify'), null, 2);
          feedback(rfUi('Только моделирование в памяти, конфигурация Keenetic не менялась','In-memory simulation only; Keenetic configuration was not changed'));
        }
      } else if (kind === 'upstreams') {
        if (action === 'status') {
          output = JSON.stringify(await request('upstreams/status'), null, 2);
        } else {
          secondary = JSON.stringify(await request('keenetic/native-bridge/status'), null, 2);
          feedback(rfUi('Keenetic Native: capability и привязки (RO); egress не подтверждён',"Keenetic native support and bindings (read-only); outbound connectivity not verified"));
        }
      } else {
        throw Error('READ_ONLY_KIND_UNKNOWN');
      }
    } catch (e) {
      error = String(e?.message || e);
      feedback(rfUi('Диагностика: ',"Diagnostics:") + error);
      if (kind === 'ingress') output = rfUi('Ошибка интеграции DNS: ',"DNS integration error:") + error;
    } finally {
      busy = false;
    }
  }
</script>

{#if kind === 'listener'}
  <div class="panel-actions"><button type="button" class="button" id="listenerPlanButton" disabled={busy} onclick={() => run('plan')}>{rfUi("План доменных зон (read-only)","DNS zone plan (read only)")}</button></div>
  <pre class="console" id="listenerPlanStatus">{output}</pre>
{:else if kind === 'ingress'}
  <div class="panel-actions">
    <button type="button" class="button" id="ingressOfflineP7" disabled={busy} onclick={() => run('offline')}>{rfUi("Проверить покрытие DNS-зон","Check DNS zone coverage")}</button>
    <button type="button" class="button" id="ingressLiveP7" disabled={busy} onclick={() => run('live')}>{rfUi("Сверить с Keenetic (только чтение)","Compare with Keenetic (read only)")}</button>
  </div>
  <pre class="console" id="ingressAuditStatusP7">{output}</pre>
{:else if kind === 'ownership'}
  <div class="panel-actions"><button type="button" class="button" id="managedOwnershipStatusO" disabled={busy} onclick={() => run('status')}>{rfUi("Проверить реестр владения","Check ownership registry")}</button></div>
  <pre class="console" id="managedOwnershipResultO">{output}</pre>
{:else if kind === 'discovery'}
  <div class="panel-actions">
    <button type="button" id="resourcesButton" class="button" disabled={busy} onclick={() => run('resources')}>{rfUi("Память / PSS / RSS","Memory / PSS / RSS")}</button>
    <button type="button" id="keeneticButton" class="button" disabled={busy} onclick={() => run('keenetic')}>{rfUi("Обнаружение Keenetic","Keenetic discovery")}</button>
    <button type="button" id="integrationButton" class="button" disabled={busy} onclick={() => run('integration')}>{rfUi("План DNS Proxy","DNS Proxy plan")}</button>
    <button type="button" id="hardwareButton" class="button" disabled={busy} onclick={() => run('hardware')}>Hardware evidence</button>
  </div>
  <pre class="console" id="discoveryStatus">{output}</pre>
{:else if kind === 'hardware'}
  <form class="form-grid" id="evidenceForm" onsubmit={(e) => {e.preventDefault(); void run('analyze');}}>
    <label class="span-all"><span>{rfUi("Текст диагностического отчёта","Diagnostic report text")}</span><textarea name="evidence" rows="9" maxlength="15000" required bind:value={evidence} placeholder={'RF_DPR2G_EVIDENCE_BEGIN\nSCHEMA=1\nMODE=READ_ONLY\n...\nRF_DPR2G_EVIDENCE_END'}></textarea></label>
    <div class="form-actions"><button type="submit" class="button primary" disabled={busy}>{rfUi("Разобрать отчёт","Parse report")}</button></div>
  </form>
  <div class="panel-actions"><button type="button" class="button" id="rehearsalButton" disabled={busy} onclick={() => run('rehearsal')}>{rfUi("Репетиция отказа / отката (in-memory)","Failure and rollback simulation (in memory)")}</button></div>
  <pre class="console" id="assessmentStatus">{output}</pre>
{:else if kind === 'upstreams'}
  <div class="panel-actions">
    <button type="button" id="upstreamStatusButton" class="button" disabled={busy} onclick={() => run('status')}>{rfUi("Статистика upstream (без проб)","Upstream statistics (no probes)")}</button>
    <button type="button" id="nativeBridgePreviewButton" class="button" disabled={busy} onclick={() => run('native')}>{rfUi("Keenetic Native: проверить привязки (RO)","Keenetic Native: check bindings (read only)")}</button>
  </div>
  <pre class="console" id="upstreamStatus">{output}</pre>
  <pre class="console" id="p85NativeStatus">{secondary}</pre>
{/if}
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
