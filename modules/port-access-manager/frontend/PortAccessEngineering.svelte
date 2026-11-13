<script>
  import { onMount } from 'svelte';
  import { createPortAccessReadSession } from './PortAccessReadSession.js';
  const readSession = createPortAccessReadSession();

  const kinds = [
    { id: 'knockd', title: 'knockd', description: 'Классический Port Knocking' },
    { id: 'fwknopd', title: 'fwknopd / SPA', description: 'Single Packet Authorization' },
    { id: 'iptables-recent', title: 'iptables recent', description: 'Без демона' }
  ];
  let selected = $state('knockd');
  let forms = $state({
    knockd: { sequence: '41001,41002,41003', target: '2222', window: '30', ttl: '120' },
    fwknopd: { target: '2222', ttl: '120' },
    'iptables-recent': { sequence: '41001,41002,41003', target: '2222', window: '30', ttl: '120' }
  });
  let wan = $state('');
  let externalPort = $state('50000');
  let natResult = $state('Только чтение.');
  let forwardResult = $state('Только чтение, без изменений firewall.');
  let validation = $state('');
  let preview = $state('');
  let invalid = $state(false);
  let stagedId = $state('test-access');
  let stageMessage = $state('Firewall не изменяется.');
  let stagedRules = $state([]);
  let stageAssessment = $state('Выбери сохранённое правило и нажми «Проверить правило».');
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
      catch { throw Error('Некорректный JSON (HTTP ' + response.status + ')'); }
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
        errors.push('Укажи три разных TCP-порта в диапазоне 1–65535.');
      else data.sequence = steps.map(Number);
    }
    if (!port(f.target)) errors.push('Защищаемый порт должен быть от 1 до 65535.');
    else data.target_port = Number(f.target);
    if (id !== 'fwknopd') {
      if (!range(f.window, 5, 300)) errors.push('Окно: 5–300 секунд.');
      else data.window_seconds = Number(f.window);
    }
    if (!range(f.ttl, 30, 3600)) errors.push('Время доступа: 30–3600 секунд.');
    else data.ttl_seconds = Number(f.ttl);
    if (data.sequence?.includes(data.target_port)) errors.push('Открываемый порт не должен совпадать с портами стуков.');
    return { data, errors };
  }
  async function renderPreview() {
    const requestId = ++previewSerial;
    const chosen = selected;
    const { data, errors } = checkInput();
    invalid = errors.length > 0;
    validation = invalid ? errors.join(' ') : 'Параметры корректны для локального предпросмотра. Проверка сети и правил firewall не выполнялась.';
    preview = invalid ? 'Предпросмотр недоступен: исправь параметры.' : JSON.stringify(data, null, 2);
    if (invalid || chosen !== 'iptables-recent') return;
    try {
      const query = new URLSearchParams({
        first: data.sequence[0], second: data.sequence[1], third: data.sequence[2],
        target: data.target_port, window: data.window_seconds, ttl: data.ttl_seconds
      });
      const plan = await jsonRequest(endpoint('knock-preview') + '?' + query);
      if (requestId !== previewSerial || selected !== chosen) return;
      if (plan.applied !== false || plan.hooked !== false || !Array.isArray(plan.commands)) throw Error('Неверный dry-run контракт');
      preview = 'Порядок команд (НЕ ПРИМЕНЕНО):\n' + plan.commands.map((args, i) => (i + 1) + '. ' + args.join(' ')).join('\n') + '\n\nЦепочка не подключена к NDM/FORWARD.';
    } catch (e) {
      if (requestId === previewSerial && selected === chosen) preview = 'Ошибка предпросмотра: ' + e.message;
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
    forwardResult = 'Читаем filter/FORWARD…';
    try {
      const query = new URLSearchParams({ wan: wan.trim(), port: target });
      const data = await jsonRequest(endpoint('forward-order') + '?' + query);
      if (data.module !== 'port-access-manager' || data.ready_for_apply !== false || data.mutation_api !== false || data.observed_only !== true) throw Error('Не подтверждён read-only контракт');
      forwardResult = 'FORWARD: планируемая позиция ' + data.planned_position + '. Никаких правил не применено.';
    } catch (e) { forwardResult = 'Проверка не пройдена: ' + e.message; }
  }
  async function checkNat() {
    const target = forms['iptables-recent'].target;
    if (!safeWan() || !port(externalPort) || !port(target)) { natResult = 'Проверь WAN и оба TCP-порта.'; return; }
    natResult = 'Читаем NAT…';
    try {
      const query = new URLSearchParams({ wan: wan.trim(), public: externalPort, target });
      const data = await jsonRequest(endpoint('nat-evidence') + '?' + query);
      if (data.module !== 'port-access-manager' || data.ready_for_apply !== false || data.mutation_api !== false || data.observed_only !== true) throw Error('Нарушен read-only контракт');
      natResult = 'DNAT найден: ' + data.destination_ip + ':' + data.target_port + ' · ' + data.path + '. Правила не применены.';
    } catch (e) { natResult = 'DNAT не подтверждён: ' + e.message; }
  }
  async function stagedRequest(method, body) {
    return jsonRequest(endpoint('staged-rules'), {
      method, headers: { 'Content-Type':'application/json', 'X-RouterForge-Action':'stage-rule', Accept:'application/json' },
      body: body ? JSON.stringify(body) : undefined
    });
  }
  async function loadStaged() {
    stageBusy = true; stageMessage = 'Читаем сохранённые правила…';
    try {
      const data = await stagedRequest('GET');
      if (data.firewall_mutation !== false || data.ready_for_apply !== false || !Array.isArray(data.rules)) throw Error('Небезопасный ответ API');
      stagedRules = data.rules;
      stageMessage = 'Черновиков: ' + data.rules.length + ' · firewall не изменён';
    } catch (e) { stageMessage = 'Сохранение пока недоступно: ' + e.message; stagedRules = []; }
    finally { stageBusy = false; }
  }
  async function saveStaged() {
    const f = forms['iptables-recent'];
    const { data, errors } = checkInput('iptables-recent');
    if (errors.length || !data.sequence) { stageMessage = errors.join(' ') || 'Нужно ровно три порта'; return; }
    stageBusy = true; stageMessage = 'Сохраняем черновик…';
    try {
      const rule = { id: stagedId.trim(), engine:'iptables-recent', sequence:data.sequence,
        target: data.target_port, window_seconds:data.window_seconds, access_seconds:data.ttl_seconds,
        wan:wan.trim(), enabled:false };
      await stagedRequest('POST', { rule });
      await loadStaged();
    } catch (e) { stageMessage = 'Не сохранено: ' + e.message; }
    finally { stageBusy = false; }
  }
  async function removeStaged(rule) {
    if (!window.confirm('Удалить черновик ' + rule.id + '?')) return;
    stageBusy = true;
    try { await stagedRequest('DELETE', { id:rule.id }); await loadStaged(); }
    catch (e) { stageMessage = e.message; }
    finally { stageBusy = false; }
  }
  async function assessStaged(rule) {
    stageAssessment = 'Проверяем ' + rule.id + '…';
    try {
      const data = await jsonRequest(endpoint('staged-assessment') + '?id=' + encodeURIComponent(rule.id));
      if (data.rule_id !== rule.id || data.ready_for_apply !== false || data.firewall_mutation !== false || data.deployment_blocked !== true || !Array.isArray(data.deployment_blockers) || !Array.isArray(data.rollback_preview)) throw Error('Небезопасный ответ API');
      stageAssessment = 'Правило: ' + rule.id + '\nПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО\nПричины: ' + data.deployment_blockers.join('; ') + '\n\nRollback preview:\n' + data.rollback_preview.map(parts => parts.join(' ')).join('\n');
    } catch (e) { stageAssessment = 'Проверка не выполнена: ' + e.message; }
  }
  async function refreshRecent() {
    recentBusy = true; recentStatus = 'Читаем текущий список ядра…';
    try {
      const data = await jsonRequest(endpoint('recent-access'));
      if (!data.available) { recentEntries = []; recentStatus = data.reason || 'Активный список не найден'; return; }
      if (!Array.isArray(data.entries)) throw Error('Некорректный ответ');
      recentEntries = data.entries;
      recentStatus = 'Авторизованных IP: ' + data.entries.length + ' · источник: knockd / xt_recent';
    } catch (e) { recentStatus = 'Не удалось получить список: ' + e.message; recentEntries = []; }
    finally { recentBusy = false; }
  }
  async function revokeRecent(entry) {
    if (!window.confirm('Отозвать авторизацию для ' + entry.ip + '?')) return;
    try {
      await jsonRequest(endpoint('recent-revoke'), { method:'POST',headers:{'Content-Type':'application/json',Accept:'application/json'},body:JSON.stringify({ip:entry.ip,confirm:'REVOKE'}) });
      await refreshRecent();
    } catch (e) { recentStatus = 'Отзыв доступа: ' + e.message; }
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

<nav class="tabs" aria-label="Способы защиты" role="tablist">
  {#each kinds as kind, i (kind.id)}
    <button type="button" role="tab" id={'tab-' + kind.id} aria-controls={'panel-' + kind.id}
      aria-selected={selected === kind.id} tabindex={selected === kind.id ? 0 : -1}
      data-tab={kind.id} onclick={() => activate(kind.id)} onkeydown={e => arrow(e, i)}>{kind.title}</button>
  {/each}
</nav>
<section class="panel" id="panel-knockd" role="tabpanel" aria-labelledby="tab-knockd" hidden={selected !== 'knockd'}>
  <h2>Классический Port Knocking</h2>
  <p class="help">Последовательность TCP-портов открывает доступ только после подтверждённой цепочки стуков.</p>
  <div class="form-grid">
    <label>Последовательность TCP-портов<input data-field="sequence" bind:value={forms.knockd.sequence} oninput={renderPreview} autocomplete="off" spellcheck="false"/></label>
    <label>Открываемый порт<input data-field="target" type="number" min="1" max="65535" bind:value={forms.knockd.target} oninput={renderPreview}/></label>
    <label>Окно последовательности, сек.<input data-field="window" type="number" min="5" max="300" bind:value={forms.knockd.window} oninput={renderPreview}/></label>
    <label>Время доступа, сек.<input data-field="ttl" type="number" min="30" max="3600" bind:value={forms.knockd.ttl} oninput={renderPreview}/></label>
  </div>
</section>
<section class="panel" id="panel-fwknopd" role="tabpanel" aria-labelledby="tab-fwknopd" hidden={selected !== 'fwknopd'}>
  <h2>Single Packet Authorization (SPA)</h2>
  <p class="help">Одно авторизованное зашифрованное сообщение. Секреты, ключи и параметры шифрования намеренно не обрабатываются в K2D.</p>
  <div class="form-grid">
    <label>Открываемый TCP-порт<input data-field="target" type="number" min="1" max="65535" bind:value={forms.fwknopd.target} oninput={renderPreview}/></label>
    <label>Время доступа, сек.<input data-field="ttl" type="number" min="30" max="3600" bind:value={forms.fwknopd.ttl} oninput={renderPreview}/></label>
  </div>
</section>
<section class="panel" id="panel-iptables-recent" role="tabpanel" aria-labelledby="tab-iptables-recent" hidden={selected !== 'iptables-recent'}>
  <h2>Без демона: iptables recent</h2>
  <p class="help">Инженерный dry-run. Реальные правила NDM не меняются; для работающей защиты используй odin.</p>
  <div class="form-grid">
    <label>Последовательность TCP-портов<input data-field="sequence" autocomplete="off" spellcheck="false" bind:value={forms['iptables-recent'].sequence} oninput={renderPreview}/></label>
    <label>Защищаемый проброшенный порт<input data-field="target" type="number" min="1" max="65535" bind:value={forms['iptables-recent'].target} oninput={renderPreview}/></label>
    <label>Окно последовательности, сек.<input data-field="window" type="number" min="5" max="300" bind:value={forms['iptables-recent'].window} oninput={renderPreview}/></label>
    <label>Время доступа, сек.<input data-field="ttl" type="number" min="30" max="3600" bind:value={forms['iptables-recent'].ttl} oninput={renderPreview}/></label>
    <label>WAN-интерфейс для проверки<input id="knock-wan" bind:value={wan} placeholder="например, eth3" autocomplete="off" spellcheck="false"/></label>
    <label>Внешний TCP-порт DNAT<input id="knock-public-port" type="number" min="1" max="65535" bind:value={externalPort}/></label>
  </div>
  <div class="refresh-bar"><button id="knock-nat-check" type="button" onclick={checkNat}>↻ Проверить DNAT</button><span id="knock-nat-result" role="status" class="muted">{natResult}</span></div>
  <div class="refresh-bar"><button id="knock-check-all" type="button" onclick={() => {void checkForward(); void checkNat();}}>↻ Проверить FORWARD и DNAT</button><span class="muted">Обе проверки только читают правила.</span></div>
  <div class="refresh-bar"><button id="knock-forward-check" type="button" onclick={checkForward}>↻ Проверить порядок FORWARD</button><span id="knock-forward-result" role="status" class="muted">{forwardResult}</span></div>
  <p class="help">Предварительная модель: входящий FORWARD, защита неправильной последовательности, срок действия разрешения. Сохранность SSH требует отдельной аппаратной проверки.</p>
</section>
<div class="preview">
  <div class="preview-top"><strong>Локальный предпросмотр</strong><span>Не применяется на роутере</span></div>
  <p id="validation" role="status" class:invalid class:good={!invalid}>{validation}</p>
  <pre id="preview" aria-label="Предпросмотр параметров">{preview}</pre>
</div>
<footer>Черновики и диагностика не применяют правила. Управляемый odin использует собственные цепочки и независимый watchdog.</footer>
<section class="panel" id="staging-panel" aria-labelledby="staging-title">
  <h2 id="staging-title">Сохранённые правила (без применения)</h2>
  <p class="help">Только Entware-черновики. Не включает firewall, не запускает службы и не меняет NDM.</p>
  <div class="form-grid"><label>ID правила<input id="staged-id" bind:value={stagedId} maxlength="40" pattern="[a-z][a-z0-9_-]*" autocomplete="off"/></label></div>
  <div class="refresh-bar"><button id="staged-save" type="button" disabled={stageBusy} onclick={saveStaged}>Сохранить черновик iptables recent</button><button id="staged-refresh" type="button" disabled={stageBusy} onclick={loadStaged}>Обновить список</button></div>
  <p id="staged-message" role="status" class="muted">{stageMessage}</p>
  <div id="staged-list" aria-live="polite">
    {#each stagedRules as rule (rule.id)}
      <div class="engine-card"><strong>{rule.id}</strong>
        <p class="muted">{rule.engine} · WAN {rule.wan} · {(rule.sequence || []).join(',')} → {rule.target} · выключено</p>
        <button type="button" disabled={stageBusy} onclick={() => assessStaged(rule)}>Проверить правило</button>
        <button type="button" disabled={stageBusy} onclick={() => removeStaged(rule)}>Удалить черновик</button>
      </div>
    {/each}
  </div>
  <pre id="staged-assessment" role="status" aria-live="polite" style="white-space:pre-wrap;overflow-wrap:anywhere">{stageAssessment}</pre>
</section>
<section class="panel" id="recent-live-panel" aria-labelledby="recent-live-title">
  <h2 id="recent-live-title">Авторизованные IP · управляемый knockd</h2>
  <p class="help">Записи ядра /proc/net/xt_recent/RFPA_K_AUTH при включённой управляемой защите knockd.</p>
  <div class="refresh-bar"><button id="recent-live-refresh" type="button" disabled={recentBusy} onclick={refreshRecent}>↻ Обновить авторизованные IP</button><span id="recent-live-status" role="status" class="muted">{recentStatus}</span></div>
  <div id="recent-live-list" aria-live="polite">
    {#each recentEntries as entry (entry.ip)}
      <div class="engine-card"><strong>{entry.ip}</strong><button type="button" onclick={() => revokeRecent(entry)}>Отозвать доступ</button></div>
    {/each}
  </div>
</section>
