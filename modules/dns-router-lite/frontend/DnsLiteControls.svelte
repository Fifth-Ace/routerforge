<script>
  // One Svelte-root-owned command surface. No document-level click handlers,
  // permanent window bridges or implicit mutating operations on mount.
  let {onAction = () => {}, page = 'dns', cards = [], chosen = '', defaultValue = '', busy = false, controls = {}, dirty = false, logView = {}, logCounter = 0, logVisible = '0 / 0', form = {} } = $props();
  let clientMac = $state('');
  let clientPersist = $state(false);
  let logFilter = $state('');
  let logType = $state('');
  let logErrors = $state(false);
  const notifyFilter=()=>send('log-filter',{filter:logFilter,type:logType,errorsOnly:logErrors});
  const send = (action, payload) => onAction(action, payload);
  function portal(node, id) {
    const host = document.getElementById(id);
    if (!host) throw Error('DNS_LITE_CONTROL_PORTAL_MISSING: ' + id);
    host.replaceChildren(node);
    return {destroy() { if (node.parentNode === host) node.remove(); }};
  }
</script>

<div use:portal={'rf-lite-reload-slot'} style="display:contents">
  <button type="button" class="button subtle" id="reloadButton" disabled={busy} onclick={() => send('reload')}>↻ Обновить</button>
</div>
<div use:portal={'rf-lite-dns-nav-slot'} style="display:contents">
  <button id="navDns" type="button" class="tab" class:selected={page === 'dns'} aria-current={page === 'dns' ? 'page' : undefined} onclick={() => send('nav-dns')}>DNS и домены</button>
</div>
<div use:portal={'rf-lite-log-nav-slot'} style="display:contents">
  <button id="navLogs" type="button" class="tab" class:selected={page === 'logs'} aria-current={page === 'logs' ? 'page' : undefined} onclick={() => send('nav-logs')}>Журнал запросов <span id="logCounter" class="counter">{logCounter}</span></button>
</div>
<div use:portal={'rf-lite-default-dns-slot'} style="display:contents">
  <select id="defaultDNS" value={defaultValue} onchange={event => send('default-change',event.currentTarget.value)}>
    <option value="">Выбери основной DNS…</option>
    {#each cards as item (item.id)}
      <option value={item.id}>{item.title}</option>
    {/each}
  </select>
</div>
<div use:portal={'rf-lite-gate-actions-slot'} class="gate-actions">
  <button type="button" class="button primary" id="applyButton" disabled={busy} onclick={() => send('apply')}>{dirty ? '✓ Сохранить и запустить DNS' : '▶ Запустить DNS'}</button>
  <button type="button" class="button" id="gatePreflight" disabled={busy || controls.preflightDisabled} onclick={() => send('gate-preflight')}>{controls.preflightLabel || 'Проверить готовность'}</button>
  <button type="button" class="button primary" id="gateEnable" disabled={busy || controls.gateEnableDisabled} onclick={() => send('gate-enable')}>Включить для LAN</button>
  <button type="button" class="button danger" id="gateDisable" disabled={busy || controls.gateDisableDisabled} onclick={() => send('gate-disable')}>Вернуть штатный DNS</button>
</div>
<div use:portal={'rf-lite-service-actions-slot'} class="actions">
  <button type="button" class="button" id="verifyButton" disabled={busy} onclick={() => send('verify')}>Проверить DNS</button>
  <button type="button" class="button" id="saveButton" disabled={busy} onclick={() => send('save')}>Сохранить без запуска</button>
  <button type="button" class="button danger" id="stopButton" disabled={busy || controls.stopDisabled} onclick={() => send('stop')}>Остановить DNS</button>
</div>
<div use:portal={'rf-lite-client-fields-slot'} style="display:contents">
  <div class="fields two">
    <label class="field"><span>MAC-адрес устройства</span><input id="clientMAC" placeholder="02:11:22:33:44:55" spellcheck="false" bind:value={clientMac}/></label>
    <div class="field"><span>DNS-профиль Keenetic</span><strong id="clientState">Не подключён</strong><small>Подключаем только выбранное устройство, не всю сеть</small></div>
  </div>
  <label class="check"><input type="checkbox" id="persistClient" bind:checked={clientPersist}/> Сохранить назначение после перезагрузки Keenetic (изменяет его общую running-config)</label>
</div>
<div use:portal={'rf-lite-client-actions-slot'} class="actions">
  <button class="button primary" type="button" id="connectButton" disabled={busy || controls.connectDisabled} onclick={() => send('connect',{mac:clientMac.trim(),persist:clientPersist})}>Подключить</button>
  <button class="button" type="button" id="disconnectButton" disabled={busy || controls.disconnectDisabled} onclick={() => send('disconnect')}>Вернуть прежний DNS</button>
</div>
<div use:portal={'rf-lite-dns-form-slot'} style="display:contents">
  <section id="dnsAddPanel" class="panel" hidden={!form.opened}>
    <div class="section-title"><div><h2 id="dnsAddPanelTitle">{form.title || 'Добавить DNS'}</h2><p>Укажи адрес провайдера, а IP и сертификат мы найдём сами.</p></div><button type="button" id="cancelDNS" class="button subtle" onclick={() => send('dns-cancel')}>Закрыть</button></div>
    <div class="fields two">
      <label class="field"><span>Протокол</span><select id="newProtocol" value={form.protocol} onchange={event => send('dns-draft-change',{protocol:event.currentTarget.value})}>
        <option value="doh">DoH · защищённый HTTPS</option><option value="dot">DoT · защищённый TLS</option><option value="udp">UDP · обычный</option><option value="tcp">TCP · обычный</option><option value="doq">DoQ · QUIC (без предварительного просмотра сертификата)</option>
      </select></label>
      <label class="field"><span>Адрес DNS</span><input id="newAddress" type="text" maxlength="300" spellcheck="false" placeholder={form.protocol==='doh'?'https://dns.example.com/dns-query':form.protocol==='udp'||form.protocol==='tcp'?'9.9.9.9':'dns.example.com'} autocomplete="off" value={form.address} oninput={event => send('dns-draft-change',{address:event.currentTarget.value})}/></label>
    </div>
    <div class="actions"><button type="button" class="button" id="inspectDNS" disabled={busy} onclick={() => send('dns-inspect')}>Проверить DNS</button><button type="button" class="button primary" id="commitDNS" disabled={busy || !form.ready} onclick={() => send('dns-commit')}>{form.button||'Добавить DNS'}</button></div>
    <div class="draft-check-grid" aria-label="Результаты проверки чернового DNS">
      {#each [['draftAddressState','Адрес'],['draftIPState','IP / порт'],['draftTLSState','TLS'],['draftCertificateState','Сертификат'],['draftDNSState','DNS'],['draftRTTState','RTT']] as item (item[0])}
        <div><span>{item[1]}</span><strong id={item[0]} data-state={form.checks?.[item[0]]?.state||''}>{form.checks?.[item[0]]?.text||'Не проверено'}</strong></div>
      {/each}
    </div>
    <p class="draft-probe-error" id="draftProbeError" role="status" hidden={!form.error}>{form.error||''}</p>
    <p id="certificateInfo" class="result muted">{form.certificateInfo||'Проверка ещё не выполнялась.'}</p>
    <details id="certificateDetails" hidden={!form.certificateVisible}><summary>Сведения о сертификате</summary><pre id="certificateDetailsText" class="cert-details">{form.certificateText||''}</pre></details>
  </section>
</div>
<div use:portal={'rf-lite-dns-buttons-slot'} style="display:contents">
  <button id="checkAllDNS" type="button" class="button" disabled={busy} onclick={() => send('dns-check-all')}>↻ Проверить все</button>
  <span class="counter" id="serverCount">{cards.length} DNS</span>
  <button type="button" id="addDnsToggle" class="button primary" disabled={busy} onclick={() => send('dns-add')}>＋ Добавить DNS</button>
</div>
<div use:portal={'rf-lite-native-migrate-slot'} style="display:contents">
  <button class="button" id="migrateNative" type="button" disabled={busy} onclick={() => send('dns-migrate')}>Преобразовать DNS</button>
</div>
<div use:portal={'rf-lite-editor-choice-slot'} style="display:contents">
  <select id="editorDNSChoice" aria-label="DNS-сервер для списков" value={chosen} disabled={!cards.some(c=>!c.native)} onchange={event => send('editor-choice',event.currentTarget.value)}>
    <option value="">Выбери DNS…</option>
    {#each cards.filter(c=>!c.native) as item (item.id)}
      <option value={item.id}>{item.title}</option>
    {/each}
  </select>
</div>
<div use:portal={'rf-lite-log-toolbar-slot'} class="log-toolbar">
  <label class="field search"><span>Фильтр по домену или DNS</span><input id="logFilter" placeholder="Например, youtube или Cloudflare" autocomplete="off" bind:value={logFilter} oninput={event=>{logFilter=event.currentTarget.value;notifyFilter();}}/></label>
  <label class="check"><input type="checkbox" id="onlyErrors" bind:checked={logErrors} onchange={event=>{if(event.currentTarget.id==='onlyErrors')logErrors=event.currentTarget.checked;else logType=event.currentTarget.value;notifyFilter();}}/> Ошибки и отказы</label>
  <label class="field log-type-field"><span>Тип</span><select id="logType" bind:value={logType} onchange={event=>{if(event.currentTarget.id==='onlyErrors')logErrors=event.currentTarget.checked;else logType=event.currentTarget.value;notifyFilter();}}><option value="">Все типы</option><option>A</option><option>AAAA</option><option>HTTPS</option><option>TXT</option><option>PTR</option><option>MX</option><option>ANY</option></select></label>
  <span id="logVisible" class="counter">{logVisible}</span>
  <button type="button" class="button" id="logPause" onclick={() => send('log-pause')}>{logView.paused ? '▶ Продолжить' : '⏸ Пауза'}</button>
  <button type="button" class="button" id="refreshLogs" onclick={() => send('log-refresh')}>↻ Обновить</button>
  <button type="button" class="button" id="copyLogs" onclick={() => send('log-copy')}>Копировать</button>
</div>
