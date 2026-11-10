<script>
  // One Svelte-root-owned command surface. No document-level click handlers,
  // permanent window bridges or implicit mutating operations on mount.
  let {onAction = () => {}, page = 'dns', cards = [], defaultValue = ''} = $props();
  const send = (action, payload) => onAction(action, payload);
  function portal(node, id) {
    const host = document.getElementById(id);
    if (!host) throw Error('DNS_LITE_CONTROL_PORTAL_MISSING: ' + id);
    host.replaceChildren(node);
    return {destroy() { if (node.parentNode === host) node.remove(); }};
  }
</script>

<div use:portal={'rf-lite-reload-slot'} style="display:contents">
  <button type="button" class="button subtle" id="reloadButton" onclick={() => send('reload')}>↻ Обновить</button>
</div>
<div use:portal={'rf-lite-dns-nav-slot'} style="display:contents">
  <button id="navDns" type="button" class="tab" class:selected={page === 'dns'} aria-current={page === 'dns' ? 'page' : undefined} onclick={() => send('nav-dns')}>DNS и домены</button>
</div>
<div use:portal={'rf-lite-log-nav-slot'} style="display:contents">
  <button id="navLogs" type="button" class="tab" class:selected={page === 'logs'} aria-current={page === 'logs' ? 'page' : undefined} onclick={() => send('nav-logs')}>Журнал запросов <span id="logCounter" class="counter">0</span></button>
</div>
<div use:portal={'rf-lite-default-dns-slot'} style="display:contents">
  <select id="defaultDNS" value={defaultValue} onchange={() => send('default-change')}>
    <option value="">Выбери основной DNS…</option>
    {#each cards as item (item.id)}
      <option value={item.id}>{item.title}</option>
    {/each}
  </select>
</div>
<div use:portal={'rf-lite-gate-actions-slot'} class="gate-actions">
  <button type="button" class="button primary" id="applyButton" onclick={() => send('apply')}>✓ Сохранить и запустить DNS</button>
  <button type="button" class="button" id="gatePreflight" onclick={() => send('gate-preflight')}>Проверить готовность</button>
  <button type="button" class="button primary" id="gateEnable" disabled onclick={() => send('gate-enable')}>Включить для LAN</button>
  <button type="button" class="button danger" id="gateDisable" disabled onclick={() => send('gate-disable')}>Вернуть штатный DNS</button>
</div>
<div use:portal={'rf-lite-service-actions-slot'} class="actions">
  <button type="button" class="button" id="verifyButton" onclick={() => send('verify')}>Проверить DNS</button>
  <button type="button" class="button" id="saveButton" onclick={() => send('save')}>Сохранить без запуска</button>
  <button type="button" class="button danger" id="stopButton" disabled onclick={() => send('stop')}>Остановить DNS</button>
</div>
<div use:portal={'rf-lite-client-actions-slot'} class="actions">
  <button class="button primary" type="button" id="connectButton" onclick={() => send('connect')}>Подключить</button>
  <button class="button" type="button" id="disconnectButton" onclick={() => send('disconnect')}>Вернуть прежний DNS</button>
</div>
<div use:portal={'rf-lite-dns-form-slot'} style="display:contents">
  <section id="dnsAddPanel" class="panel" hidden>
    <div class="section-title"><div><h2 id="dnsAddPanelTitle">Добавить DNS</h2><p>Укажи адрес провайдера, а IP и сертификат мы найдём сами.</p></div><button type="button" id="cancelDNS" class="button subtle" onclick={() => send('dns-cancel')}>Закрыть</button></div>
    <div class="fields two">
      <label class="field"><span>Протокол</span><select id="newProtocol" onchange={() => {send('dns-draft-change'); send('dns-protocol-change');}}>
        <option value="doh">DoH · защищённый HTTPS</option><option value="dot">DoT · защищённый TLS</option><option value="udp">UDP · обычный</option><option value="tcp">TCP · обычный</option><option value="doq">DoQ · QUIC (без предварительного просмотра сертификата)</option>
      </select></label>
      <label class="field"><span>Адрес DNS</span><input id="newAddress" type="text" maxlength="300" spellcheck="false" placeholder="https://dns.example.com/dns-query" autocomplete="off" oninput={() => send('dns-draft-change')}/></label>
    </div>
    <div class="actions"><button type="button" class="button" id="inspectDNS" onclick={() => send('dns-inspect')}>Проверить DNS</button><button type="button" class="button primary" id="commitDNS" disabled onclick={() => send('dns-commit')}>Добавить DNS</button></div>
    <div class="draft-check-grid" aria-label="Результаты проверки чернового DNS">
      <div><span>Адрес</span><strong id="draftAddressState">Не проверено</strong></div>
      <div><span>IP / порт</span><strong id="draftIPState">Не проверено</strong></div>
      <div><span>TLS</span><strong id="draftTLSState">Не проверено</strong></div>
      <div><span>Сертификат</span><strong id="draftCertificateState">Не проверено</strong></div>
      <div><span>DNS</span><strong id="draftDNSState">Не проверено</strong></div>
      <div><span>RTT</span><strong id="draftRTTState">Не проверено</strong></div>
    </div>
    <p class="draft-probe-error" id="draftProbeError" role="status" hidden></p>
    <p id="certificateInfo" class="result muted">Проверка ещё не выполнялась.</p>
    <details id="certificateDetails" hidden><summary>Сведения о сертификате</summary><pre id="certificateDetailsText" class="cert-details"></pre></details>
  </section>
</div>
<div use:portal={'rf-lite-dns-buttons-slot'} style="display:contents">
  <button id="checkAllDNS" type="button" class="button" onclick={() => send('dns-check-all')}>↻ Проверить все</button>
  <span class="counter" id="serverCount">0</span>
  <button type="button" id="addDnsToggle" class="button primary" onclick={() => send('dns-add')}>＋ Добавить DNS</button>
</div>
<div use:portal={'rf-lite-native-migrate-slot'} style="display:contents">
  <button class="button" id="migrateNative" type="button" onclick={() => send('dns-migrate')}>Преобразовать DNS</button>
</div>
<div use:portal={'rf-lite-editor-choice-slot'} style="display:contents">
  <select id="editorDNSChoice" aria-label="DNS-сервер для списков" onchange={() => send('editor-choice')}><option value="">Выбери DNS…</option></select>
</div>
<div use:portal={'rf-lite-log-toolbar-slot'} class="log-toolbar">
  <label class="field search"><span>Фильтр по домену или DNS</span><input id="logFilter" placeholder="Например, youtube или Cloudflare" autocomplete="off" oninput={() => send('log-filter')}/></label>
  <label class="check"><input type="checkbox" id="onlyErrors" onchange={() => send('log-filter')}/> Ошибки и отказы</label>
  <label class="field log-type-field"><span>Тип</span><select id="logType" onchange={() => send('log-filter')}><option value="">Все типы</option><option>A</option><option>AAAA</option><option>HTTPS</option><option>TXT</option><option>PTR</option><option>MX</option><option>ANY</option></select></label>
  <span id="logVisible" class="counter">0 / 0</span>
  <button type="button" class="button" id="logPause" onclick={() => send('log-pause')}>⏸ Пауза</button>
  <button type="button" class="button" id="refreshLogs" onclick={() => send('log-refresh')}>↻ Обновить</button>
  <button type="button" class="button" id="copyLogs" onclick={() => send('log-copy')}>Копировать</button>
</div>
