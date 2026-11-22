<script>
  import { rfUi } from './rf-localization.js';
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
  <button type="button" class="button subtle" id="reloadButton" disabled={busy} onclick={() => send('reload')}>{rfUi("↻ Обновить","↻ Refresh")}</button>
</div>
<div use:portal={'rf-lite-dns-nav-slot'} style="display:contents">
  <button id="navDns" type="button" class="tab" class:selected={page === 'dns'} aria-current={page === 'dns' ? 'page' : undefined} onclick={() => send('nav-dns')}>{rfUi("DNS и домены","DNS and domains")}</button>
</div>
<div use:portal={'rf-lite-log-nav-slot'} style="display:contents">
  <button id="navLogs" type="button" class="tab" class:selected={page === 'logs'} aria-current={page === 'logs' ? 'page' : undefined} onclick={() => send('nav-logs')}>{rfUi("Журнал запросов","Query log")} <span id="logCounter" class="counter">{logCounter}</span></button>
</div>
<div use:portal={'rf-lite-default-dns-slot'} style="display:contents">
  <select id="defaultDNS" value={defaultValue} onchange={event => send('default-change',event.currentTarget.value)}>
    <option value="">{rfUi("Выбери основной DNS…","Select primary DNS…")}</option>
    {#each cards as item (item.id)}
      <option value={item.id}>{item.title}</option>
    {/each}
  </select>
</div>
<div use:portal={'rf-lite-gate-actions-slot'} class="gate-actions">
  <button type="button" class="button primary" id="applyButton" disabled={busy} onclick={() => send('apply')}>{dirty ? '✓ Сохранить и запустить DNS' : '▶ Запустить DNS'}</button>
  <button type="button" class="button" id="gatePreflight" disabled={busy || controls.preflightDisabled} onclick={() => send('gate-preflight')}>{controls.preflightLabel || 'Проверить готовность'}</button>
  <button type="button" class="button primary" id="gateEnable" disabled={busy || controls.gateEnableDisabled} onclick={() => send('gate-enable')}>{rfUi("Включить для LAN","Enable for LAN")}</button>
  <button type="button" class="button danger" id="gateDisable" disabled={busy || controls.gateDisableDisabled} onclick={() => send('gate-disable')}>{rfUi("Вернуть штатный DNS","Restore Keenetic DNS")}</button>
</div>
<div use:portal={'rf-lite-service-actions-slot'} class="actions">
  <button type="button" class="button" id="verifyButton" disabled={busy} onclick={() => send('verify')}>{rfUi("Проверить DNS","Test DNS")}</button>
  <button type="button" class="button" id="saveButton" disabled={busy} onclick={() => send('save')}>{rfUi("Сохранить без запуска","Save without starting")}</button>
  <button type="button" class="button danger" id="stopButton" disabled={busy || controls.stopDisabled} onclick={() => send('stop')}>{rfUi("Остановить DNS","Stop DNS")}</button>
</div>
<div use:portal={'rf-lite-client-fields-slot'} style="display:contents">
  <div class="fields two">
    <label class="field"><span>{rfUi("MAC-адрес устройства","Device MAC address")}</span><input id="clientMAC" placeholder="02:11:22:33:44:55" spellcheck="false" bind:value={clientMac}/></label>
    <div class="field"><span>{rfUi("DNS-профиль Keenetic","Keenetic DNS profile")}</span><strong id="clientState">{rfUi("Не подключён","Not connected")}</strong><small>{rfUi("Подключаем только выбранное устройство, не всю сеть","Connect only the selected device, not the entire network")}</small></div>
  </div>
  <label class="check"><input type="checkbox" id="persistClient" bind:checked={clientPersist}/> {rfUi("Сохранить назначение после перезагрузки Keenetic (изменяет его общую running-config)","Persist the assignment across Keenetic reboots (changes its running configuration)")}</label>
</div>
<div use:portal={'rf-lite-client-actions-slot'} class="actions">
  <button class="button primary" type="button" id="connectButton" disabled={busy || controls.connectDisabled} onclick={() => send('connect',{mac:clientMac.trim(),persist:clientPersist})}>{rfUi("Подключить","Connect")}</button>
  <button class="button" type="button" id="disconnectButton" disabled={busy || controls.disconnectDisabled} onclick={() => send('disconnect')}>{rfUi("Вернуть прежний DNS","Restore previous DNS")}</button>
</div>
<div use:portal={'rf-lite-dns-form-slot'} style="display:contents">
  <section id="dnsAddPanel" class="panel" hidden={!form.opened}>
    <div class="section-title"><div><h2 id="dnsAddPanelTitle">{form.title || 'Добавить DNS'}</h2><p>{rfUi("Укажи адрес провайдера, а IP и сертификат мы найдём сами.","Enter the provider address; RouterForge will discover IP and certificate information.")}</p></div><button type="button" id="cancelDNS" class="button subtle" onclick={() => send('dns-cancel')}>{rfUi("Закрыть","Close")}</button></div>
    <div class="fields two">
      <label class="field"><span>{rfUi("Протокол","Protocol")}</span><select id="newProtocol" value={form.protocol} onchange={event => send('dns-draft-change',{protocol:event.currentTarget.value})}>
        <option value="doh">{rfUi("DoH · защищённый HTTPS","DoH · encrypted HTTPS")}</option><option value="dot">{rfUi("DoT · защищённый TLS","DoT · encrypted TLS")}</option><option value="udp">{rfUi("UDP · обычный","UDP · unencrypted")}</option><option value="tcp">{rfUi("TCP · обычный","TCP · unencrypted")}</option><option value="doq">{rfUi("DoQ · QUIC (без предварительного просмотра сертификата)","DoQ · QUIC (certificate preview unavailable)")}</option><option value="stamp">DNS Stamp · DNSCrypt / DoH / DoT / DoQ</option>
      </select></label>
      <label class="field"><span>{rfUi("Адрес DNS","DNS address")}</span><input id="newAddress" type="text" maxlength={form.protocol==='stamp'?4096:300} spellcheck="false" placeholder={form.protocol==='doh'?'https://dns.example.com/dns-query':form.protocol==='udp'||form.protocol==='tcp'?'9.9.9.9':'dns.example.com'} autocomplete="off" value={form.address} oninput={event => send('dns-draft-change',{address:event.currentTarget.value})}/></label>
    </div>
    {#if form.protocol==='doh'}
      <label class="field"><span>HTTP/3</span><select id="p32Http3" value={form.http3Mode||'off'} onchange={event=>send('dns-draft-change',{http3Mode:event.currentTarget.value})}><option value="off">{rfUi("Выключен (HTTP/1.1, HTTP/2)","Disabled (HTTP/1.1, HTTP/2)")}</option><option value="auto">{rfUi("Авто (HTTP/1.1, HTTP/2, HTTP/3)","Automatic (HTTP/1.1, HTTP/2, HTTP/3)")}</option><option value="only">{rfUi("Только HTTP/3","HTTP/3 only")}</option></select></label>
    {/if}
    {#if ['doh','dot','doq'].includes(form.protocol)}
      <label class="field"><span>{rfUi("Дополнительные Bootstrap IP (до 4, по одному в строке)","Additional bootstrap IPs (up to 4, one per line)")}</span><textarea id="p32Bootstrap" rows="2" value={form.bootstrapIPs||''} oninput={event=>send('dns-draft-change',{bootstrapIPs:event.currentTarget.value})} placeholder="1.1.1.1"></textarea></label>
    {/if}
    {#if form.protocol==='stamp'}<p class="muted">{rfUi("Вставь sdns:// с явным публичным IP. Импорт не использует системный DNS.","Paste sdns:// with an explicit public IP. Import does not use system DNS.")}</p>{/if}
    <div class="actions"><button type="button" class="button" id="inspectDNS" disabled={busy} onclick={() => send('dns-inspect')}>{rfUi("Проверить DNS","Test DNS")}</button><button type="button" class="button primary" id="commitDNS" disabled={busy || !form.ready} onclick={() => send('dns-commit')}>{form.button||'Добавить DNS'}</button></div>
    <div class="draft-check-grid" aria-label={rfUi("Результаты проверки чернового DNS","DNS draft test results")}>
      {#each [['draftAddressState','Адрес'],['draftIPState','IP / порт'],['draftTLSState','TLS'],['draftCertificateState','Сертификат'],['draftDNSState','DNS'],['draftRTTState','RTT']] as item (item[0])}
        <div><span>{item[1]}</span><strong id={item[0]} data-state={form.checks?.[item[0]]?.state||''}>{form.checks?.[item[0]]?.text||'Не проверено'}</strong></div>
      {/each}
    </div>
    <p class="draft-probe-error" id="draftProbeError" role="status" hidden={!form.error}>{form.error||''}</p>
    <p id="certificateInfo" class="result muted">{form.certificateInfo||'Проверка ещё не выполнялась.'}</p>
    <details id="certificateDetails" hidden={!form.certificateVisible}><summary>{rfUi("Сведения о сертификате","Certificate information")}</summary><pre id="certificateDetailsText" class="cert-details">{form.certificateText||''}</pre></details>
  </section>
</div>
<div use:portal={'rf-lite-dns-buttons-slot'} style="display:contents">
  <button id="checkAllDNS" type="button" class="button" disabled={busy} onclick={() => send('dns-check-all')}>{rfUi("↻ Проверить все","↻ Check all")}</button>
  <span class="counter" id="serverCount">{cards.length} DNS</span>
  <button type="button" id="addDnsToggle" class="button primary" disabled={busy} onclick={() => send('dns-add')}>{rfUi("＋ Добавить DNS","＋ Add DNS")}</button>
</div>
<div use:portal={'rf-lite-native-migrate-slot'} style="display:contents">
  <button class="button" id="migrateNative" type="button" disabled={busy} onclick={() => send('dns-migrate')}>{rfUi("Преобразовать DNS","Convert DNS")}</button>
</div>
<div use:portal={'rf-lite-editor-choice-slot'} style="display:contents">
  <select id="editorDNSChoice" aria-label={rfUi("DNS-сервер для списков","DNS server for lists")} value={chosen} disabled={!cards.some(c=>!c.native)} onchange={event => {const prior=chosen;if(send('editor-choice',event.currentTarget.value)===false)event.currentTarget.value=prior;}}>
    <option value="">{rfUi("Выбери DNS…","Select DNS…")}</option>
    {#each cards.filter(c=>!c.native) as item (item.id)}
      <option value={item.id}>{item.title}</option>
    {/each}
  </select>
</div>
<div use:portal={'rf-lite-log-toolbar-slot'} class="log-toolbar">
  <label class="field search"><span>{rfUi("Фильтр по домену или DNS","Filter by domain or DNS")}</span><input id="logFilter" placeholder={rfUi("Например, youtube или Cloudflare","For example, YouTube or Cloudflare")} autocomplete="off" bind:value={logFilter} oninput={event=>{logFilter=event.currentTarget.value;notifyFilter();}}/></label>
  <label class="check"><input type="checkbox" id="onlyErrors" bind:checked={logErrors} onchange={event=>{if(event.currentTarget.id==='onlyErrors')logErrors=event.currentTarget.checked;else logType=event.currentTarget.value;notifyFilter();}}/> {rfUi("Ошибки и отказы","Errors and failures")}</label>
  <label class="field log-type-field"><span>{rfUi("Тип","Type")}</span><select id="logType" bind:value={logType} onchange={event=>{if(event.currentTarget.id==='onlyErrors')logErrors=event.currentTarget.checked;else logType=event.currentTarget.value;notifyFilter();}}><option value="">{rfUi("Все типы","All types")}</option><option>A</option><option>AAAA</option><option>HTTPS</option><option>TXT</option><option>PTR</option><option>MX</option><option>ANY</option></select></label>
  <span id="logVisible" class="counter">{logVisible}</span>
  <button type="button" class="button" id="logPause" onclick={() => send('log-pause')}>{logView.paused ? '▶ Продолжить' : '⏸ Пауза'}</button>
  <button type="button" class="button" id="refreshLogs" onclick={() => send('log-refresh')}>{rfUi("↻ Обновить","↻ Refresh")}</button>
  <button type="button" class="button" id="copyLogs" onclick={() => send('log-copy')}>{rfUi("Копировать","Copy")}</button>
</div>
