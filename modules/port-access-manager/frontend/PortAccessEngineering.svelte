<script>
  import { rfUi } from './rf-localization.js';
  import { onMount } from 'svelte';
  import { createPortAccessReadSession } from './PortAccessReadSession.js';
  const readSession = createPortAccessReadSession();

  const kinds = [
    { id: 'knockd', title: 'knockd', description: rfUi('Классический Port Knocking',"Classic Port Knocking") },
    { id: 'fwknopd', title: 'fwknopd / SPA', description: 'Single Packet Authorization' },
    { id: 'iptables-recent', title: 'iptables recent', description: rfUi('Без демона',"Without daemon") }
  ];
  let selected = $state('knockd');
  let forms = $state({
    knockd: { sequence: '41001,41002,41003', target: '2222', window: '30', ttl: '120' },
    fwknopd: { target: '2222', ttl: '120' },
    'iptables-recent': { sequence: '41001,41002,41003', target: '2222', window: '30', ttl: '120' }
  });
  let wan = $state('');
  let externalPort = $state('50000');
  let natResult = $state(rfUi('Только чтение.',"Read-only."));
  let forwardResult = $state(rfUi('Только чтение, без изменений firewall.',"Read-only, no firewall changes."));
  let validation = $state('');
  let preview = $state('');
  let invalid = $state(false);
  let stagedId = $state('test-access');
  let stageMessage = $state(rfUi('Firewall не изменяется.',"Firewall settings are unchanged."));
  let stagedRules = $state([]);
  let stageAssessment = $state(rfUi('Выбери сохранённое правило и нажми «Проверить правило».',"Select a saved rule and choose Check rule."));
  let stageBusy = $state(false);
  let recentStatus = $state('');
  let recentEntries = $state([]);
  let recentBusy = $state(false);
  let previewSerial = 0;

  const apiBase = () => {
    const p = window.location.pathname;
    const core = p.match(/^(.*\/api\/modules\/port-access-manager)\/(?:v1\/)?ui(?:\/.*)?$/);
    if (core) return core[1];
    const direct = p.match(/^(.*)\/v1\/ui(?:\/.*)?$/);
    if (direct) return direct[1] + '/v1';
    return '/api/modules/port-access-manager';
  };
  const endpoint = suffix => apiBase() + '/' + suffix;
  async function jsonRequest(url, options = {}) {
    return readSession.consume(url, {
      credentials: 'same-origin', cache: 'no-store',
      headers: { Accept: 'application/json', ...options.headers }, ...options
    }, async response => {
      let data;
      try { data = await response.json(); }
      catch { throw Error(rfUi('Некорректный JSON (HTTP ',"Invalid JSON (HTTP") + response.status + ')'); }
      if (!response.ok) throw Error(data.error || 'HTTP ' + response.status);
      return data;
    });
  }
  const port = text => /^\d+$/.test(String(text)) && Number(text) >= 1 && Number(text) <= 65535;
  const range = (text, min, max) => /^\d+$/.test(String(text)) && Number(text) >= min && Number(text) <= max;
  function checkInput(id = selected) {
    const f = forms[id];
    const data = { engine: id, operation: 'preview-only', applied: false };
    const errors = [];
    if (id !== 'fwknopd') {
      const steps = f.sequence.split(',').map(part => part.trim());
      if (steps.length !== 3 || !steps.every(port) || new Set(steps).size !== steps.length)
        errors.push(rfUi('Укажи три разных TCP-порта в диапазоне 1–65535.',"Enter three distinct TCP ports in the range 1–65535."));
      else data.sequence = steps.map(Number);
    }
    if (!port(f.target)) errors.push('Защищаемый порт должен быть от 1 до 65535.');
    else data.target_port = Number(f.target);
    if (id !== 'fwknopd') {
      if (!range(f.window, 5, 300)) errors.push(rfUi('Окно: 5–300 секунд.',"Window: 5–300 seconds."));
      else data.window_seconds = Number(f.window);
    }
    if (!range(f.ttl, 30, 3600)) errors.push(rfUi('Время доступа: 30–3600 секунд.',"Access duration: 30–3600 seconds."));
    else data.ttl_seconds = Number(f.ttl);
    if (data.sequence?.includes(data.target_port)) errors.push(rfUi('Открываемый порт не должен совпадать с портами стуков.',"The protected port must not match any knock port."));
    return { data, errors };
  }
  async function renderPreview() {
    const requestId = ++previewSerial;
    const chosen = selected;
    const { data, errors } = checkInput();
    invalid = errors.length > 0;
    validation = invalid ? errors.join(' ') : rfUi('Параметры корректны для локального предпросмотра. Проверка сети и правил firewall не выполнялась.',"Parameters are valid for local preview. Network and firewall rules were not checked.");
    preview = invalid ? rfUi('Предпросмотр недоступен: исправь параметры.',"Preview unavailable: correct the parameters.") : JSON.stringify(data, null, 2);
    if (invalid || chosen !== 'iptables-recent') return;
    try {
      const query = new URLSearchParams({
        first: data.sequence[0], second: data.sequence[1], third: data.sequence[2],
        target: data.target_port, window: data.window_seconds, ttl: data.ttl_seconds
      });
      const plan = await jsonRequest(endpoint('knock-preview') + '?' + query);
      if (requestId !== previewSerial || selected !== chosen) return;
      if (plan.applied !== false || plan.hooked !== false || !Array.isArray(plan.commands)) throw Error(rfUi('Неверный dry-run контракт',"Invalid dry-run response"));
      preview = rfUi('Порядок команд (НЕ ПРИМЕНЕНО):\n',"Command sequence (NOT APPLIED):\\n") + plan.commands.map((args, i) => (i + 1) + '. ' + args.join(' ')).join('\n') + rfUi('\n\nЦепочка не подключена к NDM/FORWARD.',"\\n\\nChain is not attached to NDM/FORWARD.");
    } catch (e) {
      if (requestId === previewSerial && selected === chosen) preview = rfUi('Ошибка предпросмотра: ',"Preview error:") + e.message;
    }
  }
  function activate(id) {
    if (!kinds.some(k => k.id === id)) return;
    selected = id;
    void renderPreview();
  }
  function arrow(event, i) {
    if (!['ArrowLeft','ArrowRight'].includes(event.key)) return;
    event.preventDefault();
    const next = kinds[(i + (event.key === 'ArrowRight' ? 1 : -1) + kinds.length) % kinds.length];
    activate(next.id);
    document.getElementById('tab-' + next.id)?.focus();
  }
  const safeWan = () => /^[a-zA-Z][a-zA-Z0-9_.:-]{0,14}$/.test(wan.trim());
  async function checkForward() {
    const target = forms['iptables-recent'].target;
    if (!safeWan() || !port(target)) { forwardResult = 'Укажи WAN-интерфейс и корректный защищаемый порт.'; return; }
    forwardResult = rfUi('Читаем filter/FORWARD…',"Reading filter/FORWARD…");
    try {
      const query = new URLSearchParams({ wan: wan.trim(), port: target });
      const data = await jsonRequest(endpoint('forward-order') + '?' + query);
      if (data.module !== 'port-access-manager' || data.ready_for_apply !== false || data.mutation_api !== false || data.observed_only !== true) throw Error(rfUi('Не подтверждён read-only контракт',"Read-only safety contract not verified"));
      forwardResult = rfUi('FORWARD: планируемая позиция ',"FORWARD: planned position") + data.planned_position + rfUi('. Никаких правил не применено.',". No rules have been applied.");
    } catch (e) { forwardResult = rfUi('Проверка не пройдена: ',"Check failed:") + e.message; }
  }
  async function checkNat() {
    const target = forms['iptables-recent'].target;
    if (!safeWan() || !port(externalPort) || !port(target)) { natResult = 'Проверь WAN и оба TCP-порта.'; return; }
    natResult = rfUi('Читаем NAT…',"Reading NAT…");
    try {
      const query = new URLSearchParams({ wan: wan.trim(), public: externalPort, target });
      const data = await jsonRequest(endpoint('nat-evidence') + '?' + query);
      if (data.module !== 'port-access-manager' || data.ready_for_apply !== false || data.mutation_api !== false || data.observed_only !== true) throw Error(rfUi('Нарушен read-only контракт',"Read-only safety contract violated"));
      natResult = rfUi('DNAT найден: ',"DNAT found:") + data.destination_ip + ':' + data.target_port + ' · ' + data.path + rfUi('. Правила не применены.',". Rules have not been applied.");
    } catch (e) { natResult = rfUi('DNAT не подтверждён: ',"DNAT not verified:") + e.message; }
  }
  async function stagedRequest(method, body) {
    return jsonRequest(endpoint('staged-rules'), {
      method, headers: { 'Content-Type':'application/json', 'X-RouterForge-Action':'stage-rule', Accept:'application/json' },
      body: body ? JSON.stringify(body) : undefined
    });
  }
  async function loadStaged() {
    stageBusy = true; stageMessage = rfUi('Читаем сохранённые правила…',"Reading saved rules…");
    try {
      const data = await stagedRequest('GET');
      if (data.firewall_mutation !== false || data.ready_for_apply !== false || !Array.isArray(data.rules)) throw Error(rfUi('Небезопасный ответ API',"Unsafe API response"));
      stagedRules = data.rules;
      stageMessage = rfUi('Черновиков: ',"Drafts:") + data.rules.length + rfUi(' · firewall не изменён',"· firewall unchanged");
    } catch (e) { stageMessage = rfUi('Сохранение пока недоступно: ',"Saving is currently unavailable:") + e.message; stagedRules = []; }
    finally { stageBusy = false; }
  }
  async function saveStaged() {
    const f = forms['iptables-recent'];
    const { data, errors } = checkInput('iptables-recent');
    if (errors.length || !data.sequence) { stageMessage = errors.join(' ') || rfUi('Нужно ровно три порта',"Exactly three ports are required"); return; }
    stageBusy = true; stageMessage = rfUi('Сохраняем черновик…',"Saving draft…");
    try {
      const rule = { id: stagedId.trim(), engine:'iptables-recent', sequence:data.sequence,
        target: data.target_port, window_seconds:data.window_seconds, access_seconds:data.ttl_seconds,
        wan:wan.trim(), enabled:false };
      await stagedRequest('POST', { rule });
      await loadStaged();
    } catch (e) { stageMessage = rfUi('Не сохранено: ',"Not saved:") + e.message; }
    finally { stageBusy = false; }
  }
  async function removeStaged(rule) {
    if (!window.confirm(rfUi('Удалить черновик ',"Delete draft") + rule.id + '?')) return;
    stageBusy = true;
    try { await stagedRequest('DELETE', { id:rule.id }); await loadStaged(); }
    catch (e) { stageMessage = e.message; }
    finally { stageBusy = false; }
  }
  async function assessStaged(rule) {
    stageAssessment = rfUi('Проверяем ',"Checking") + rule.id + '…';
    try {
      const data = await jsonRequest(endpoint('staged-assessment') + '?id=' + encodeURIComponent(rule.id));
      if (data.rule_id !== rule.id || data.ready_for_apply !== false || data.firewall_mutation !== false || data.deployment_blocked !== true || !Array.isArray(data.deployment_blockers) || !Array.isArray(data.rollback_preview)) throw Error(rfUi('Небезопасный ответ API',"Unsafe API response"));
      stageAssessment = rfUi('Правило: ',"Rule:") + rule.id + rfUi('\nПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО\nПричины: ',"\\nAPPLYING BLOCKED\\nReasons:") + data.deployment_blockers.join('; ') + '\n\nRollback preview:\n' + data.rollback_preview.map(parts => parts.join(' ')).join('\n');
    } catch (e) { stageAssessment = rfUi('Проверка не выполнена: ',"Check not performed:") + e.message; }
  }
  async function refreshRecent() {
    recentBusy = true; recentStatus = rfUi('Читаем текущий список ядра…',"Reading current kernel list…");
    try {
      const data = await jsonRequest(endpoint('recent-access'));
      if (!data.available) { recentEntries = []; recentStatus = data.reason || rfUi('Активный список не найден',"Active list not found"); return; }
      if (!Array.isArray(data.entries)) throw Error(rfUi('Некорректный ответ',"Invalid response"));
      recentEntries = data.entries;
      recentStatus = rfUi('Авторизованных IP: ',"Authorized IPs:") + data.entries.length + rfUi(' · источник: knockd / xt_recent',"· source: knockd / xt_recent");
    } catch (e) { recentStatus = rfUi('Не удалось получить список: ',"Could not load list:") + e.message; recentEntries = []; }
    finally { recentBusy = false; }
  }
  async function revokeRecent(entry) {
    if (!window.confirm(rfUi('Отозвать авторизацию для ',"Revoke authorization for") + entry.ip + '?')) return;
    try {
      await jsonRequest(endpoint('recent-revoke'), { method:'POST',headers:{'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify({ip:entry.ip,confirm:'REVOKE'}) });
      await refreshRecent();
    } catch (e) { recentStatus = rfUi('Отзыв доступа: ',"Revoke access:") + e.message; }
  }
  onMount(() => {
    void renderPreview(); void loadStaged(); void refreshRecent();
    const tokens = ['--rf-bg','--rf-surface','--rf-surface-2','--rf-hover','--rf-text','--rf-muted','--rf-border','--rf-border-strong','--rf-accent','--rf-accent-soft','--rf-radius-panel','--rf-radius-control','--rf-radius-card'];
    function syncTheme() {
      if (window.parent === window) return;
      try {
        const host = window.parent.document.documentElement;
        const computed = window.parent.getComputedStyle(host);
        for (const token of tokens) {
          const value = computed.getPropertyValue(token).trim();
          if (value) document.documentElement.style.setProperty(token, value);
        }
        for (const key of ['theme','density','radius','uiScale']) {
          if (host.dataset[key]) document.documentElement.dataset[key] = host.dataset[key];
        }
      } catch { /* Cross-origin iframe: use native defaults. */ }
    }
    syncTheme();
    let observer;
    if (window.parent !== window && typeof MutationObserver !== 'undefined') {
      try {
        observer = new MutationObserver(syncTheme);
        observer.observe(window.parent.document.documentElement, { attributes:true, attributeFilter:['style','class','data-theme','data-density','data-radius','data-ui-scale'] });
      } catch { /* Cross-origin iframe */ }
    }
    return () => { ++previewSerial; readSession.close(); observer?.disconnect(); };
  });
</script>

<nav class="tabs" aria-label={rfUi("Способы защиты","Protection methods")} role="tablist">
  {#each kinds as kind, i (kind.id)}
    <button type="button" role="tab" id={'tab-' + kind.id} aria-controls={'panel-' + kind.id}
      aria-selected={selected === kind.id} tabindex={selected === kind.id ? 0 : -1}
      data-tab={kind.id} onclick={() => activate(kind.id)} onkeydown={e => arrow(e, i)}>{kind.title}</button>
  {/each}
</nav>
<section class="panel" id="panel-knockd" role="tabpanel" aria-labelledby="tab-knockd" hidden={selected !== 'knockd'}>
  <h2>{rfUi("Классический Port Knocking","Classic Port Knocking")}</h2>
  <p class="help">{rfUi("Последовательность TCP-портов открывает доступ только после подтверждённой цепочки стуков.","A sequence of TCP knocks grants access only when the full sequence has been verified.")}</p>
  <div class="form-grid">
    <label>{rfUi("Последовательность TCP-портов","TCP port sequence")}<input data-field="sequence" bind:value={forms.knockd.sequence} oninput={renderPreview} autocomplete="off" spellcheck="false"/></label>
    <label>{rfUi("Открываемый порт","Port to open")}<input data-field="target" type="number" min="1" max="65535" bind:value={forms.knockd.target} oninput={renderPreview}/></label>
    <label>{rfUi("Окно последовательности, сек.","Sequence window, sec")}<input data-field="window" type="number" min="5" max="300" bind:value={forms.knockd.window} oninput={renderPreview}/></label>
    <label>{rfUi("Время доступа, сек.","Access duration, sec")}<input data-field="ttl" type="number" min="30" max="3600" bind:value={forms.knockd.ttl} oninput={renderPreview}/></label>
  </div>
</section>
<section class="panel" id="panel-fwknopd" role="tabpanel" aria-labelledby="tab-fwknopd" hidden={selected !== 'fwknopd'}>
  <h2>Single Packet Authorization (SPA)</h2>
  <p class="help">{rfUi("Одно авторизованное зашифрованное сообщение. Секреты, ключи и параметры шифрования намеренно не обрабатываются в K2D.","A single authorized encrypted message. Secrets, keys, and encryption parameters are intentionally not handled in K2D.")}</p>
  <div class="form-grid">
    <label>{rfUi("Открываемый TCP-порт","TCP port to open")}<input data-field="target" type="number" min="1" max="65535" bind:value={forms.fwknopd.target} oninput={renderPreview}/></label>
    <label>{rfUi("Время доступа, сек.","Access duration, sec")}<input data-field="ttl" type="number" min="30" max="3600" bind:value={forms.fwknopd.ttl} oninput={renderPreview}/></label>
  </div>
</section>
<section class="panel" id="panel-iptables-recent" role="tabpanel" aria-labelledby="tab-iptables-recent" hidden={selected !== 'iptables-recent'}>
  <h2>{rfUi("Без демона: iptables recent","Daemonless: iptables recent")}</h2>
  <p class="help">{rfUi("Инженерный dry-run. Реальные правила NDM не меняются; для работающей защиты используй odin.","Engineering dry run. Actual NDM rules remain unchanged; use odin for active protection.")}</p>
  <div class="form-grid">
    <label>{rfUi("Последовательность TCP-портов","TCP port sequence")}<input data-field="sequence" autocomplete="off" spellcheck="false" bind:value={forms['iptables-recent'].sequence} oninput={renderPreview}/></label>
    <label>{rfUi("Защищаемый проброшенный порт","Protected forwarded port")}<input data-field="target" type="number" min="1" max="65535" bind:value={forms['iptables-recent'].target} oninput={renderPreview}/></label>
    <label>{rfUi("Окно последовательности, сек.","Sequence window, sec")}<input data-field="window" type="number" min="5" max="300" bind:value={forms['iptables-recent'].window} oninput={renderPreview}/></label>
    <label>{rfUi("Время доступа, сек.","Access duration, sec")}<input data-field="ttl" type="number" min="30" max="3600" bind:value={forms['iptables-recent'].ttl} oninput={renderPreview}/></label>
    <label>{rfUi("WAN-интерфейс для проверки","WAN interface to check")}<input id="knock-wan" bind:value={wan} placeholder={rfUi("например, eth3","for example, eth3")} autocomplete="off" spellcheck="false"/></label>
    <label>{rfUi("Внешний TCP-порт DNAT","External TCP DNAT port")}<input id="knock-public-port" type="number" min="1" max="65535" bind:value={externalPort}/></label>
  </div>
  <div class="refresh-bar"><button id="knock-nat-check" type="button" onclick={checkNat}>{rfUi("↻ Проверить DNAT","↻ Check DNAT")}</button><span id="knock-nat-result" role="status" class="muted">{natResult}</span></div>
  <div class="refresh-bar"><button id="knock-check-all" type="button" onclick={() => {void checkForward(); void checkNat();}}>{rfUi("↻ Проверить FORWARD и DNAT","↻ Check FORWARD and DNAT")}</button><span class="muted">{rfUi("Обе проверки только читают правила.","Both checks only read firewall rules.")}</span></div>
  <div class="refresh-bar"><button id="knock-forward-check" type="button" onclick={checkForward}>{rfUi("↻ Проверить порядок FORWARD","↻ Check FORWARD order")}</button><span id="knock-forward-result" role="status" class="muted">{forwardResult}</span></div>
  <p class="help">{rfUi("Предварительная модель: входящий FORWARD, защита неправильной последовательности, срок действия разрешения. Сохранность SSH требует отдельной аппаратной проверки.","Preview model: inbound FORWARD rules, protection against an incorrect knock sequence, and access expiry. SSH access safety requires a separate device-level check.")}</p>
</section>
<div class="preview">
  <div class="preview-top"><strong>{rfUi("Локальный предпросмотр","Local preview")}</strong><span>{rfUi("Не применяется на роутере","Not applied to the router")}</span></div>
  <p id="validation" role="status" class:invalid class:good={!invalid}>{validation}</p>
  <pre id="preview" aria-label={rfUi("Предпросмотр параметров","Settings preview")}>{preview}</pre>
</div>
<footer>{rfUi("Черновики и диагностика не применяют правила. Управляемый odin использует собственные цепочки и независимый watchdog.","Drafts and diagnostics do not apply rules. Managed odin uses its own chains and independent watchdog.")}</footer>
<section class="panel" id="staging-panel" aria-labelledby="staging-title">
  <h2 id="staging-title">{rfUi("Сохранённые правила (без применения)","Saved rules (not applied)")}</h2>
  <p class="help">{rfUi("Только Entware-черновики. Не включает firewall, не запускает службы и не меняет NDM.","Entware drafts only. Does not enable the firewall, start services, or change NDM.")}</p>
  <div class="form-grid"><label>{rfUi("ID правила","Rule ID")}<input id="staged-id" bind:value={stagedId} maxlength="40" pattern="[a-z][a-z0-9_-]*" autocomplete="off"/></label></div>
  <div class="refresh-bar"><button id="staged-save" type="button" disabled={stageBusy} onclick={saveStaged}>{rfUi("Сохранить черновик iptables recent","Save iptables recent draft")}</button><button id="staged-refresh" type="button" disabled={stageBusy} onclick={loadStaged}>{rfUi("Обновить список","Refresh list")}</button></div>
  <p id="staged-message" role="status" class="muted">{stageMessage}</p>
  <div id="staged-list" aria-live="polite">
    {#each stagedRules as rule (rule.id)}
      <div class="engine-card"><strong>{rule.id}</strong>
        <p class="muted">{rule.engine} · WAN {rule.wan} · {(rule.sequence || []).join(',')} → {rule.target} {rfUi("· выключено","· disabled")}</p>
        <button type="button" disabled={stageBusy} onclick={() => assessStaged(rule)}>{rfUi("Проверить правило","Check rule")}</button>
        <button type="button" disabled={stageBusy} onclick={() => removeStaged(rule)}>{rfUi("Удалить черновик","Delete draft")}</button>
      </div>
    {/each}
  </div>
  <pre id="staged-assessment" role="status" aria-live="polite" style="white-space:pre-wrap;overflow-wrap:anywhere">{stageAssessment}</pre>
</section>
<section class="panel" id="recent-live-panel" aria-labelledby="recent-live-title">
  <h2 id="recent-live-title">{rfUi("Авторизованные IP · управляемый knockd","Authorized IPs · managed knockd")}</h2>
  <p class="help">{rfUi("Записи ядра /proc/net/xt_recent/RFPA_K_AUTH при включённой управляемой защите knockd.","Kernel entries in /proc/net/xt_recent/RFPA_K_AUTH when managed knockd protection is enabled.")}</p>
  <div class="refresh-bar"><button id="recent-live-refresh" type="button" disabled={recentBusy} onclick={refreshRecent}>{rfUi("↻ Обновить авторизованные IP","↻ Refresh authorized IPs")}</button><span id="recent-live-status" role="status" class="muted">{recentStatus}</span></div>
  <div id="recent-live-list" aria-live="polite">
    {#each recentEntries as entry (entry.ip)}
      <div class="engine-card"><strong>{entry.ip}</strong><button type="button" onclick={() => revokeRecent(entry)}>{rfUi("Отозвать доступ","Revoke access")}</button></div>
    {/each}
  </div>
</section>
