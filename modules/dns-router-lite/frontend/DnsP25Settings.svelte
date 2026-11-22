<script>
  import { rfUi } from './rf-localization.js';
  import {onMount, getContext} from 'svelte';
  import DnsP34Diagnostics from './DnsP34Diagnostics.svelte';

  // Svelte owns Settings navigation, section selection, read-only telemetry,
  // and every setting input. Existing Lite draft/CAS/apply remains authoritative.
  let active = $state(false);
  let section = $state('service');
  let advanced = $state(false);
  const fields = getContext('routerforge:dns-lite-settings').fields;
  let scope = $derived(fields.p243Scope);
  let ipEnabled = $derived(fields.p241IPEnabled);
  let ipMode = $derived(fields.p241IPMode);
  let ipStatus = $state(rfUi('Монитор выключен',"Monitoring disabled"));
  let ipRows = $state([]);
  let performanceStatus = $state(rfUi('Метрики: нет данных',"Metrics: no data"));
  let cacheClearBusy = $state(false);
  let cacheClearStatus = $state('');
  let english = $state(false);
  let app;
  let observer;
  let intervals = [];
  let disposed = false;
  const pending = new Set();
  let cleanups = [];
  const t = (ru, en) => english ? en : ru;
  const byId = id => document.getElementById(id);
  // Both basic and advanced layouts contain a Svelte-owned instance of each
  // service control. This action moves only OUR Svelte nodes, never a foreign
  // legacy form or an already-attached event handler.
  function ownSlot(node, targetId) {
    const place = id => {
      const target = document.getElementById(id);
      if (!target) throw Error('P25_NATIVE_SLOT_MISSING: '+id);
      target.appendChild(node);
    };
    place(targetId);
    return {update:place,destroy(){node.remove();}};
  }
  // The parent owns draft/CAS. Merely displaying settings is GET-only.
  function settingsInputChange(event) {
    if (disposed) return;
    const control=event?.currentTarget;
    if (!control?.id || !(control.id in fields)) return;
    // Svelte dispatches user listeners before updating bind:value/checked.
    // Capture the actual event value first, not a stale reactive snapshot.
    if (control.multiple) fields[control.id]=Array.from(control.selectedOptions,o=>o.value);
    else if (control.type==='checkbox') fields[control.id]=control.checked;
    else if (control.type==='number') fields[control.id]=control.value===''?NaN:Number(control.value);
    else fields[control.id]=control.value;
    settingsHooks.change(control.id);
  }
  const sections = [
    {id:'service',ru:rfUi('DNS-сервис',"DNS service"),en:'DNS service',status:'available'},
    {id:'cache',ru:rfUi('Кэш',"Cache"),en:'Cache',status:'available'},
    {id:'ip',ru:'IP Intelligence',en:'IP Intelligence',status:'experimental'},
    {id:'upstream',ru:rfUi('Выбор DNS',"Select DNS"),en:'Upstream selection',status:'available'},
    {id:'security',ru:rfUi('Защита запросов',"Query protection"),en:'Query protection',status:'available'},
    {id:'privacy',ru:'EDNS / ECS / DNSSEC',en:'EDNS / ECS / DNSSEC',status:'available'},
    {id:'dns64',ru:'DNS64 / rDNS',en:'DNS64 / rDNS',status:'available'},
    {id:'performance',ru:rfUi('Производительность',"Performance"),en:'Performance',status:'available'},
    {id:'local',ru:rfUi('Локальный DNS',"Local DNS"),en:'Local DNS',status:'available'},
    {id:'secure',ru:rfUi('Защищённый DNS',"Encrypted DNS"),en:'Secure DNS',status:'experimental'},
    {id:'diagnostics',ru:rfUi('Диагностика DNS',"DNS diagnostics"),en:'DNS diagnostics',status:'available'}
  ];
  const settingsHooks = getContext('routerforge:dns-lite-settings');
  const navigationHooks = getContext('routerforge:dns-lite-navigation');
  const inputChange = settingsInputChange;
  const notes = {
    original:[rfUi('Без изменения IP в DNS-ответе.',"Without changing IP addresses in DNS responses."),'No address changes.'],
    round_robin:[rfUi('Циклически меняет первый IP при каждом новом DNS-запросе, включая ответы из кэша.',"Rotates the first IP on every DNS query, including cached responses."),'Rotates the first IP on each DNS request, including cached responses.'],
    random:[rfUi('Случайно переставляет все IP; исходный DNS-ответ сохранён.',"Randomizes all IPs; original DNS response is preserved."),'Shuffles addresses; original DNS response is preserved.'],
    fastest:[rfUi('С проверками TCP/TLS сортирует по задержке; без них сохраняет исходный порядок.',"With TCP/TLS checks, sorts by latency; otherwise preserves original order."),'Sorts measured IPs by latency; otherwise preserves original order.'],
    health_aware:[rfUi('С проверками ставит доступные IP первыми; неизвестные адреса не считаются упавшими.',"With checks, reachable IPs come first; unknown addresses are not treated as down."),'Prefers healthy IPs; unknown is not down.'],
    adaptive:[rfUi('С измерениями учитывает доступность и задержку; без измерений использует ротацию.',"With measurements, selects by availability and latency; otherwise rotates."),'Adapts to measurements; rotates otherwise.'],
    sticky:[rfUi('Закрепляет порядок за проверенным MAC; для неизвестных устройств использует ротацию.',"Keeps IP order for a verified MAC; rotates for unknown devices."),'Keeps stable client ordering; rotates for unknown devices.'],
    weighted:[rfUi('С измерениями веса учитывают задержку; без них используются равные веса.',"With measurements, weights account for latency; otherwise equal weights are used."),'Weights measured latency; otherwise equal weights.'],
    healthy_only:[rfUi('Без проверок IP ничего не исключается. DNSSEC-защищённые наборы не сокращаются.',"Without IP checks, no addresses are excluded. DNSSEC-protected sets are not reduced."),'Without checks no IP is excluded. DNSSEC-protected RRsets remain intact.']
  };
  let modeNote = $derived(!ipEnabled ? t('Функция выключена. DNS-ответы не изменяются.','Feature disabled. DNS responses remain unchanged.') : t(...(notes[ipMode] || notes.original)));

  function monitorRowsFrom(data){
    return (Array.isArray(data.entries) ? data.entries : []).map(record => ({
      address:String(record.address||'—'), domain:String(record.domain||'—'),
      status:String(record.status||'UNKNOWN'),
      rtt:(record.rtt_ms ? String(record.rtt_ms)+' ms' : '—')+' · '+String(record.failures||0)+' '+t('ош.','err')+' / '+String(record.recoveries||0)+' '+t('усп.','ok'),
      last:record.last_success ? String(record.last_success).replace('T',' ').replace(/Z$/,' UTC') : '—',
      title:[record.last_checked ? t('Проверен: ','Checked: ')+record.last_checked : '',record.last_error ? t('Ошибка: ','Error: ')+record.last_error : '',record.last_down ? t('DOWN с: ','DOWN since: ')+record.last_down : ''].filter(Boolean).join(' · ')
    }));
  }
  async function refreshIPHealth(){
    if(disposed || !advanced || !active || section!=='ip' || document.hidden) return;
    const controller=new AbortController(); pending.add(controller);
    try{
      const response=await fetch('../service/ip-health',{method:'GET',cache:'no-store',signal:controller.signal});
      if(!response.ok) throw Error('HTTP '+response.status);
      const data=await response.json();
      if(disposed || controller.signal.aborted)return;
      ipRows=monitorRowsFrom(data);
      ipStatus=data.enabled ? t('Активен · записей: ','Active · entries: ')+ipRows.length : t('Монитор выключен','Monitor disabled');
    }catch(e){if(!disposed && !controller.signal.aborted)ipStatus=t('Ошибка мониторинга: ','Monitor error: ')+e.message;}
    finally{pending.delete(controller);}
  }
  async function clearP33Cache() {
    if(cacheClearBusy || !confirm(t('Очистить DNS-кэш работающего RouterForge? Это не меняет правила или Gate.','Clear the active RouterForge DNS cache? Routing and Gate will not change.')))return;
    cacheClearBusy=true;cacheClearStatus='';
    try {
      const statusResp=await fetch('../service/status',{method:'GET',cache:'no-store'});
      if(!statusResp.ok)throw Error('status HTTP '+statusResp.status);
      const status=await statusResp.json();
      if(!status.active || !status.applied_revision)throw Error(t('Сначала запусти DNS','Start DNS first'));
      const response=await fetch('../service/cache/control',{method:'POST',headers:{'Content-Type':'application/json','X-RouterForge-Action':'service-control','If-Match':'"'+status.applied_revision+'"'},body:JSON.stringify({operation:'clear'})});
      if(!response.ok)throw Error('cache HTTP '+response.status);
      cacheClearStatus=t('Кэш очищен','Cache cleared');
      await refreshP29Metrics();
    }catch(error){cacheClearStatus=String(error.message||error)}
    finally{cacheClearBusy=false}
  }
  async function refreshP29Metrics(){
    if(disposed || !advanced || !active || section!=='performance' || document.hidden) return;
    const controller=new AbortController(); pending.add(controller);
    try{
      const response=await fetch('../service/status',{method:'GET',cache:'no-store',signal:controller.signal});
      if(!response.ok) throw Error('HTTP '+response.status);
      const data=await response.json();
      if(disposed || controller.signal.aborted)return;
      performanceStatus=t('Запросы: ','Queries: ')+String(data.requests||0)+t(' · Кэш: ',' · Cache: ')+String(data.cache_hits||0)+t(' · Ошибки: ',' · Errors: ')+String(data.failures||0)+t(' · DNS-измерений: ',' · Measured upstreams: ')+Object.keys(data.upstream_selection_metrics?.upstreams||{}).length+' · '+t('Изолированный кэш: ','Policy cache: ')+(data.cache_mode==='isolated_device_policy_cache'?'ON':data.cache_mode==='disabled_for_device_policy_isolation'?'OFF':'—')+' · '+t('Записей: ','Entries: ')+String(data.policy_cache?.entries||0);
    }catch(e){if(!disposed && !controller.signal.aborted)performanceStatus=t('Метрики недоступны: ','Metrics unavailable: ')+e.message;}
    finally{pending.delete(controller);}
  }
  function selectSection(id){
    section=id;
    if(id==='ip') refreshIPHealth();
    if(id==='performance') refreshP29Metrics();
  }
  function placement(){
    const newMode=app?.dataset.dnsMode==='advanced';
    if (newMode===advanced) return;
    advanced=newMode;
    if (!newMode) {
      active=false;
      if (app) app.removeAttribute('data-p25-settings-active');
    }
  }
  function openSettings(){
    if(!advanced && app?.dataset.dnsMode==='advanced')advanced=true;
    if(!advanced)return;
    // The workspace Svelte owner controls section visibility and active nav.
    // Settings never moves or toggles another component's DOM nodes.
    navigationHooks.navigate('settings');
    active=true;
    if(app)app.dataset.p25SettingsActive='true';
    refreshIPHealth();
  }
  function closeSettings(){active=false;if(app)app.removeAttribute('data-p25-settings-active');}
  function watch(target,type,callback){
    if(!target)return;
    target.addEventListener(type,callback);
    cleanups.push(()=>target.removeEventListener(type,callback));
  }
  onMount(()=>{
    disposed=false;
    app=byId('dnsLiteApp');
    if(!app) return;
    english=document.documentElement.lang==='en'||new URLSearchParams(location.search).get('lang')==='en';
    watch(byId('p21Mode'),'change',placement);
    for(const id of ['navDns','navLogs','p21Nav','p22NavGroups','p22NavRules']) watch(byId(id),'click',closeSettings);
    settingsHooks.render=()=>{};
    observer=new MutationObserver(placement);
    observer.observe(app,{attributes:true,attributeFilter:['data-dns-mode']});
    placement();
    intervals=[setInterval(refreshIPHealth,10000),setInterval(refreshP29Metrics,10000)];
    return ()=>{
      disposed=true;
      for(const timer of intervals)clearInterval(timer);
      for(const dispose of cleanups)dispose();
      observer?.disconnect();
      for(const controller of pending)controller.abort();
      pending.clear();
      settingsHooks.render=()=>{};
    };
  });

</script>
<div use:ownSlot={advanced ? 'p25Settings_service' : 'rf-dns-lite-basic-service-slot'} class="p25-native-service-fields">
  <div class="fields two">
    <label class="field"><span>{rfUi("Адрес DNS-сервиса","DNS service address")}</span><input id="serviceListen" onchange={settingsInputChange} bind:value={fields.serviceListen}/></label>
    <label class="field"><span>{rfUi("Разрешённые сети","Allowed networks")}</span><input id="allowedNetworks" spellcheck="false" onchange={settingsInputChange} bind:value={fields.allowedNetworks}/></label>
  </div>
</div>
<div use:ownSlot={advanced ? 'p25Settings_cache' : 'rf-dns-lite-basic-cache-slot'} class="p25-native-cache-field">
  <label class="field"><span>{rfUi("Кэш, МиБ","Cache, MiB")}</span><input id="cacheSize" type="number" min="0" max="8" step="1" onchange={settingsInputChange} bind:value={fields.cacheSize}/></label>
</div>

<button use:ownSlot={'rf-dns-lite-settings-nav-slot'} id="p25SettingsNav" class="tab p22-advanced-tab" type="button" aria-controls="p25SettingsCard" class:selected={active} aria-current={active?'page':undefined} onclick={openSettings}>{t('Настройки','Settings')}</button>
<section use:ownSlot={'rf-dns-lite-settings-page-slot'} id="p25SettingsCard" class="panel rfds-card p25-settings-card" hidden={!active || !advanced} aria-label={t('Настройки DNS Router','DNS Router settings')}>
  <div class="p25-settings-heading"><h2>{t('Настройки DNS Router','DNS Router settings')}</h2><p class="muted">{t('Настройки сервиса и DNSProxy в одном месте.','DNS service and DNSProxy settings in one place.')}</p></div>
  <div class="p25-settings-layout">
    <nav class="p25-settings-sidebar" aria-label={t('Категории настроек','Settings categories')}>
      {#each sections as item (item.id)}
        <button type="button" class="p25-settings-link" class:selected={section===item.id} aria-pressed={section===item.id} data-settings-section={item.id} onclick={()=>selectSection(item.id)}><span class="p25-settings-link-label">{t(item.ru,item.en)}</span><small class="p25-settings-link-state">{item.status==='experimental'?t('Экспериментально','Experimental'):t('Доступно','Available')}</small></button>
      {/each}
    </nav>
    <div class="p25-settings-content">
      <section id="p25Settings_service" class="p25-settings-section" hidden={section!=='service'} aria-label={t('DNS-сервис','DNS service')}><h3>{t('DNS-сервис','DNS service')}</h3><p class="p25-settings-info">{t('Параметры DNS-сервиса. Сохранение и применение выполняются только через общую панель.','DNS service settings. Save and apply only through the shared toolbar.')}</p></section>
      <section id="p25Settings_cache" class="p25-settings-section" hidden={section!=='cache'} aria-label={t('Кэш','Cache')}><h3>{t('Кэш','Cache')}</h3><p class="p25-settings-info">{t('Кэш изменяется только после явного сохранения и применения.','Cache changes require an explicit save and apply.')}</p></section>
      <section id="p25Settings_ip" class="p25-settings-section" hidden={section!=='ip'} aria-label="IP Intelligence"><h3>IP Intelligence</h3><p class="p25-settings-info">{t('Доступны девять способов выбора IP-адресов. Дополнительная проверка TCP/TLS запускается по желанию; отсутствие ответа не означает, что узел недоступен.','Choose from nine IP-selection strategies. Optional TCP/TLS checks can be enabled; an unknown result does not mean the host is down.')}</p>
        <p class="p25-settings-info">{modeNote}</p>
        <p class="p25-settings-info" id="p25IPHealthState">{ipStatus}</p>
            <div id="p241IPControls" class="p241-ip-controls">
      <label class="check p241-ip-toggle"><input type="checkbox" id="p241IPEnabled" onchange={(event)=>{settingsInputChange(event);}} bind:checked={fields.p241IPEnabled}><span>{rfUi("Включить IP Intelligence","Enable IP Intelligence")}</span></label>
      <label class="field p241-ip-mode"><span>{rfUi("Алгоритм выбора IP","IP selection algorithm")}</span>
        <select id="p241IPMode" onchange={(event)=>{settingsInputChange(event);}} aria-label={rfUi("Алгоритм IP Intelligence","IP Intelligence algorithm")} bind:value={fields.p241IPMode}>
          <option value="original">{rfUi("Original · без изменений","Original · unchanged")}</option>
          <option value="round_robin">{rfUi("Round-Robin · по кругу","Round-Robin · rotating order")}</option>
          <option value="random">{rfUi("Random · случайный порядок","Random · randomized order")}</option>
          <option value="fastest">{rfUi("Fastest IP · быстрые первыми","Fastest IP · fastest first")}</option>
          <option value="health_aware">{rfUi("Health-Aware · доступные первыми","Health-Aware · reachable first")}</option>
          <option value="adaptive">{rfUi("Adaptive · адаптивная ротация","Adaptive · adaptive rotation")}</option>
          <option value="sticky">{rfUi("Sticky IP · закрепить за устройством","Sticky IP · pin to device")}</option>
          <option value="weighted">{rfUi("Weighted · распределение по весам","Weighted · weighted distribution")}</option>
          <option value="healthy_only">{rfUi("Healthy Only · исключать недоступные","Healthy Only · exclude unreachable")}</option>
        </select>
      </label>
      <p class="p25-settings-info" id="p241IPModeNote" role="status">{modeNote}</p>
      <div class="p243-scope-settings">
        <label class="field"><span>{rfUi("Область применения","Scope")}</span><select id="p243Scope" onchange={(event)=>{settingsInputChange(event);}} bind:value={fields.p243Scope}><option value="all">{rfUi("Все внешние домены","All external domains")}</option><option value="domains">{rfUi("Выбранные домены","Selected domains")}</option><option value="lists">{rfUi("Выбранные списки","Selected lists")}</option></select></label>
        <label class="field p243-domains" hidden={scope!=='domains'}><span>{rfUi("Домены, по одному в строке (включая поддомены)","Domains, one per line (including subdomains)")}</span><textarea id="p243Domains" onchange={settingsInputChange} rows="3" spellcheck="false" placeholder="example.org" bind:value={fields.p243Domains}></textarea></label>
        <label class="field p243-lists" hidden={scope!=='lists'}><span>{rfUi("Списки доменов (Ctrl/Shift для нескольких)","Domain lists (Ctrl/Shift to select multiple)")}</span><select id="p243ListIDs" onchange={settingsInputChange} multiple size="4" bind:value={fields.p243ListIDs}>{#each fields.ipListOptions as option (option.id)}<option value={option.id}>{option.label}</option>{/each}</select></label>
      </div>
      <div class="p242-health-settings">
        <label class="check"><input type="checkbox" id="p242HealthEnabled" onchange={settingsInputChange} bind:checked={fields.p242HealthEnabled}> {rfUi("Фоновая проверка IP-адресов (экспериментально)","Background IP checks (experimental)")}</label>
        <div class="p242-health-fields">
          <label class="field"><span>{rfUi("Проверка","Check")}</span><select id="p242HealthProtocol" onchange={settingsInputChange} bind:value={fields.p242HealthProtocol}><option value="tcp">{rfUi("TCP: соединение с портом","TCP: connect to port")}</option><option value="tls">{rfUi("TLS: проверка сертификата","TLS: verify certificate")}</option><option value="http">{rfUi("HTTP: ответ приложения","HTTP: application response")}</option><option value="https">{rfUi("HTTPS: ответ приложения + TLS","HTTPS: application response + TLS")}</option></select></label>
          <label class="field"><span>{rfUi("Порт","Port")}</span><input id="p242HealthPort" onchange={settingsInputChange} type="number" min="80" max="65535" bind:value={fields.p242HealthPort}></label>
          <label class="field"><span>{rfUi("Интервал, секунд","Interval, seconds")}</span><input id="p242HealthInterval" onchange={settingsInputChange} type="number" min="30" max="3600" bind:value={fields.p242HealthInterval}></label>
          <label class="field"><span>{rfUi("Тайм-аут, мс","Timeout, ms")}</span><input id="p242HealthTimeout" onchange={settingsInputChange} type="number" min="250" max="3000" bind:value={fields.p242HealthTimeout}></label>
          <label class="field"><span>{rfUi("Неудач до DOWN","Failures before marking down")}</span><input id="p242HealthFailures" onchange={settingsInputChange} type="number" min="1" max="5" bind:value={fields.p242HealthFailures}></label>
          <label class="field"><span>{rfUi("Успехов до UP","Successes before marking up")}</span><input id="p243Recoveries" onchange={settingsInputChange} type="number" min="1" max="5" bind:value={fields.p243Recoveries}></label>
          <label class="field"><span>{rfUi("Выдержка DOWN, сек","Down-state hold, sec")}</span><input id="p243MinDown" onchange={settingsInputChange} type="number" min="0" max="3600" bind:value={fields.p243MinDown}></label>
          <label class="field"><span>{rfUi("HTTP-путь","HTTP path")}</span><input id="p243HTTPPath" onchange={settingsInputChange} maxlength="128" placeholder="/health" bind:value={fields.p243HTTPPath}></label>
        </div>
        <p class="p25-settings-info">{rfUi("Проверяются только публичные IP из реальных ответов A/AAAA, не чаще одного подключения в секунду. Приватные и служебные адреса исключены. HTTP/HTTPS проверяют ответ сервиса, 5xx считаются ошибкой; редиректы не выполняются. TCP/TLS подтверждают только соединение.","Only public IPs from real A/AAAA responses are checked, at most once per second. Private and reserved addresses are excluded. HTTP/HTTPS checks assess the service response; 5xx responses are failures, and redirects are not followed. TCP/TLS checks confirm connectivity only.")}</p>
        <div class="p242-monitor"><div class="p242-monitor-heading"><strong>{rfUi("Монитор IP","IP monitor")}</strong><small id="p242MonitorStatus">{rfUi("Нет измерений","No measurements")}</small></div>
          <div id="p242MonitorRows" class="p242-monitor-rows" aria-live="polite"><span class="muted">{rfUi("Записи появятся после запросов и фоновых проверок.","Entries appear after queries and background checks.")}</span></div>
        </div>
      </div>
    </div>

        <div class="p242-monitor p25-native-monitor" aria-live="polite">
          {#if ipRows.length===0}<span class="muted">{t('Нет измерений. Дождись DNS-запросов.','No measurements yet.')}</span>{/if}
          {#each ipRows as record, i (record.address+'|'+record.domain+'|'+i)}
            <div class="p242-monitor-row" title={record.title}>
              <span class="p242-monitor-address">{record.address}</span><span class="p242-monitor-domain">{record.domain}</span><span class="p242-monitor-state">{record.status}</span><span class="p242-monitor-rtt">{record.rtt}</span><span class="p242-monitor-last">{record.last}</span>
            </div>
          {/each}
        </div>
      </section>
      <section id="p25Settings_upstream" class="p25-settings-section" hidden={section!=='upstream'} aria-label={t('Выбор DNS','Upstream selection')}><h3>{t('Выбор DNS','Upstream selection')}</h3><p class="p25-settings-info">{t('Для работы требуется явная DNS-группа или резервная цепочка.','Requires an explicit upstream group or fallback chain.')}</p>
        <label class="check"><span>{t('Включить выбор DNS','Enable upstream selection')}</span><input id="p28Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p28Enabled}/></label>
        <label class="field"><span>{t('Алгоритм','Mode')}</span><select id="p28Mode" onchange={inputChange} bind:value={fields.p28Mode}><option value="sequential">Failover / Sequential</option><option value="load_balance">Load Balance</option><option value="parallel">Parallel</option><option value="fastest">Fastest DNS</option></select></label>
        <label class="field"><span>{t('Тайм-аут на DNS, мс','Timeout per DNS, ms')}</span><input id="p28Timeout" type="number" min="250" max="5000" onchange={inputChange} bind:value={fields.p28Timeout}/></label>
      </section>
      <section id="p25Settings_security" class="p25-settings-section" hidden={section!=='security'} aria-label={t('Защита запросов','Query protection')}><h3>{t('Защита запросов','Query protection')}</h3><p class="p25-settings-info">{t('Частотные ограничения, DNS ANY и Bogus NXDOMAIN действуют только после сохранения и явного применения.','Rate limits, ANY refusal and Bogus NXDOMAIN require explicit save and apply.')}</p>    <div id="p27SecurityControls" class="p27-security-controls">
      <p class="p25-settings-info">{rfUi("Защита применяется к независимому DNS Router, не меняет правила Keenetic. Все параметры по умолчанию выключены.","Protection applies to the standalone DNS Router and does not alter Keenetic rules. All options are disabled by default.")}</p>
      <div class="fields two">
        <label class="field"><span>{rfUi("Лимит запросов одного клиента в секунду (0 — выкл.)","Per-client request limit per second (0 = disabled)")}</span><input id="p27RateLimit" onchange={settingsInputChange} type="number" min="0" max="2000" bind:value={fields.p27RateLimit}></label>
        <label class="check"><input type="checkbox" id="p27RefuseAny" onchange={settingsInputChange} bind:checked={fields.p27RefuseAny}> {rfUi("Отказывать на DNS ANY (NOTIMP)","Reject DNS ANY (NOTIMP)")}</label>
      </div>
      <label class="field"><span>{rfUi("Bogus NXDOMAIN — IP-адреса или CIDR (по одному в строке)","Bogus NXDOMAIN — IP addresses or CIDR ranges (one per line)")}</span><textarea id="p27BogusIPs" onchange={settingsInputChange} rows="4" spellcheck="false" placeholder="0.0.0.0" bind:value={fields.p27BogusIPs}></textarea></label>
      <p class="p25-settings-info">{rfUi("Если DNS вернул такой IP, RouterForge отправит NXDOMAIN. Не включает блокировку доменов и не проверяет криптографически DNSSEC. CIDR: IPv4 от /24, IPv6 от /64.","When a DNS answer contains a matching IP, RouterForge returns NXDOMAIN. This does not enable domain blocking or cryptographically verify DNSSEC. CIDR: IPv4 /24 or larger, IPv6 /64 or larger.")}</p>
    </div>
</section>
      <section id="p25Settings_privacy" class="p25-settings-section" hidden={section!=='privacy'} aria-label="EDNS / ECS / DNSSEC"><h3>EDNS / ECS / DNSSEC</h3><p class="p25-settings-info">{t('Require AD доверяет только шифрованным DNS; локальная проверка DNSSEC не выполняется.','Require AD trusts only encrypted upstreams; there is no local DNSSEC validation.')}</p>
        <label class="check"><span>{t('Включить настройки EDNS/ECS/DNSSEC','Enable EDNS/ECS/DNSSEC controls')}</span><input id="p30Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p30Enabled}/></label>
        <label class="field"><span>EDNS Client Subnet</span><select id="p30ECSMode" onchange={inputChange} bind:value={fields.p30ECSMode}><option value="inherit">{t('Сохранять запрос клиента','Preserve client option')}</option><option value="strip">{t('Удалять ECS','Strip ECS')}</option><option value="fixed">{t('Фиксированный публичный адрес','Fixed public IP')}</option></select></label>
        <label class="field"><span>{t('Публичный IP ECS','Public ECS address')}</span><input id="p30ECSAddress" type="text" onchange={inputChange} bind:value={fields.p30ECSAddress}/></label>
        <label class="field"><span>DNSSEC</span><select id="p30DNSSECMode" onchange={inputChange} bind:value={fields.p30DNSSECMode}><option value="off">Off</option><option value="request_do">Request DO</option><option value="require_ad">Require AD (encrypted DNS only)</option></select></label>
      </section>
      <section id="p25Settings_dns64" class="p25-settings-section" hidden={section!=='dns64'} aria-label="DNS64 / rDNS"><h3>DNS64 / rDNS</h3><p class="p25-settings-info">{t('DNS64 требует заранее настроенного NAT64; RouterForge не меняет маршруты.','DNS64 requires preconfigured NAT64; RouterForge does not change routing.')}</p>
        <label class="check"><span>{t('Включить DNS64','Enable DNS64')}</span><input id="p31DNS64Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p31DNS64Enabled}/></label>
        <label class="field"><span>{t('Префикс NAT64 /96','NAT64 /96 prefix')}</span><input id="p31DNS64Prefix" type="text" onchange={inputChange} bind:value={fields.p31DNS64Prefix}/></label>
        <label class="check"><span>{t('Подтверждаю, что NAT64 уже настроен','I confirm NAT64 already works')}</span><input id="p31NAT64Confirmed" type="checkbox" onchange={inputChange} bind:checked={fields.p31NAT64Confirmed}/></label>
        <label class="check"><span>{t('Отдельный DNS для приватных PTR','Custom private PTR resolver')}</span><input id="p31ReverseEnabled" type="checkbox" onchange={inputChange} bind:checked={fields.p31ReverseEnabled}/></label>
        <label class="field"><span>{t('DNS для rDNS','rDNS upstream')}</span><select id="p31ReverseUpstream" onchange={inputChange} bind:value={fields.p31ReverseUpstream}><option value="">{rfUi("Выбери прямой DNS…","Select a direct DNS…")}</option>{#each fields.reverseOptions as option (option.id)}<option value={option.id}>{option.label}</option>{/each}</select></label>
      </section>
      <section id="p25Settings_diagnostics" class="p25-settings-section" hidden={section!=='diagnostics'} aria-label={t('Диагностика DNS','DNS diagnostics')}>
        <h3>{t('Диагностика DNS','DNS diagnostics')}</h3>
        {#if section==='diagnostics'}<DnsP34Diagnostics/>{/if}
      </section>
      <section id="p25Settings_secure" class="p25-settings-section" hidden={section!=='secure'} aria-label={t('Защищённый DNS','Secure DNS')}>
        <h3>{t('Защищённый DNS','Secure DNS')}</h3>
        <p class="p25-settings-info">{t('Входящие DoT, DoH и DoQ: TLS 1.3 и ОБЯЗАТЕЛЬНЫЙ клиентский сертификат (mTLS). Доступ только по IP интерфейса DNS Router, через разрешённые сети. Не открывает порты Keenetic и ничего не запускает без общего Save и Apply.','Inbound DoT, DoH and DoQ: TLS 1.3 with MANDATORY client certificates (mTLS). Explicit DNS Router IP, existing allowed networks, no Keenetic port changes; requires Save then Apply.')}</p>
        <label class="check"><input id="p36Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p36Enabled}/><span>{t('Включить защищённые входящие DNS','Enable encrypted inbound DNS')}</span></label>
        <div class="fields two">
          <label class="field"><span>DoT · TCP IP:port</span><input id="p36DoT" spellcheck="false" placeholder="192.168.10.1:10853" onchange={inputChange} bind:value={fields.p36DoT}/></label>
          <label class="field"><span>DoH · HTTPS IP:port</span><input id="p36DoH" spellcheck="false" placeholder="192.168.10.1:10443" onchange={inputChange} bind:value={fields.p36DoH}/></label>
          <label class="field"><span>DoQ · UDP IP:port</span><input id="p36DoQ" spellcheck="false" placeholder="192.168.10.1:10854" onchange={inputChange} bind:value={fields.p36DoQ}/></label>
          <label class="field"><span>{t('PEM сертификат сервера','Server certificate PEM path')}</span><input id="p36CertificateFile" spellcheck="false" placeholder="/opt/etc/routerforge/secure-dns/server.pem" onchange={inputChange} bind:value={fields.p36CertificateFile}/></label>
          <label class="field"><span>{t('Приватный ключ сервера, 0600','Server private key, mode 0600')}</span><input id="p36PrivateKeyFile" spellcheck="false" placeholder="/opt/etc/routerforge/secure-dns/server.key" onchange={inputChange} bind:value={fields.p36PrivateKeyFile}/></label>
          <label class="field"><span>{t('CA клиентских сертификатов','Client certificate CA path')}</span><input id="p36ClientCAFile" spellcheck="false" placeholder="/opt/etc/routerforge/secure-dns/clients-ca.pem" onchange={inputChange} bind:value={fields.p36ClientCAFile}/></label>
        </div>
        <p class="p25-settings-info">{t('В конфигурации хранятся только пути. Ключи не загружаются через UI. Клиент без доверенного сертификата отклоняется на TLS-рукопожатии. DoH: /dns-query.','Only file paths are saved, never key material. Clients without trusted certificates fail TLS authentication. DoH: /dns-query.')}</p>
      </section>
      <section id="p25Settings_local" class="p25-settings-section" hidden={section!=='local'} aria-label={t('Локальный DNS','Local DNS')}>
        <h3>{t('Локальный DNS','Local DNS')}</h3>
        <p class="p25-settings-info">{t('Записи обрабатываются только для *.rf.home.arpa. Штатные имена Keenetic, *.lan, *.local и чужие *.home.arpa не изменяются. Изменения требуют общего сохранения и отдельного применения.','Only *.rf.home.arpa is served locally. Keenetic stock names and other home.arpa names remain untouched; explicit Save and Apply are required.')}</p>
        <label class="check"><input id="p35Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p35Enabled}/><span>{t('Включить локальную зону RouterForge','Enable RouterForge local zone')}</span></label>
        <label class="field"><span>{t('Записи: NAME | TYPE | VALUE | TTL (30–86400)','Records: NAME | TYPE | VALUE | TTL (30–86400)')}</span>
          <textarea id="p35Records" spellcheck="false" rows="8" maxlength="32768" placeholder="nas.rf.home.arpa | A | 192.168.10.8 | 300&#10;alias.rf.home.arpa | CNAME | nas.rf.home.arpa | 300" onchange={inputChange} bind:value={fields.p35Records}></textarea>
        </label>
        <p class="p25-settings-info">{t('Поддержка A, AAAA, CNAME, PTR; максимум 128 записей. PTR разрешён только для приватного IP, уже указанного в локальной A/AAAA записи.','A, AAAA, CNAME and PTR; up to 128 records. PTR must match an explicitly configured private A/AAAA record.')}</p>
      </section>
      <section id="p25Settings_performance" class="p25-settings-section" hidden={section!=='performance'} aria-label={t('Производительность','Performance')}><h3>{t('Производительность','Performance')}</h3><p class="p25-settings-info">{performanceStatus}</p>
        <label class="check"><span>{t('Включить расширенные настройки','Enable performance settings')}</span><input id="p29Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p29Enabled}/></label>
        <label class="field"><span>{t('Минимум TTL, с','Min cache TTL, s')}</span><input id="p29MinTTL" type="number" min="0" max="3600" onchange={inputChange} bind:value={fields.p29MinTTL}/></label>
        <label class="field"><span>{t('Максимум TTL, с (0 — без ограничения)','Max cache TTL, s (0 = no cap)')}</span><input id="p29MaxTTL" type="number" min="0" max="86400" onchange={inputChange} bind:value={fields.p29MaxTTL}/></label>
        <label class="field"><span>{t('Параллельные обработчики','Concurrent handlers')}</span><input id="p29Concurrent" type="number" min="8" max="64" onchange={inputChange} bind:value={fields.p29Concurrent}/></label>
        <label class="check"><span>Optimistic cache</span><input id="p29Optimistic" type="checkbox" onchange={inputChange} bind:checked={fields.p29Optimistic}/></label>
        <label class="field"><span>{t('Максимальный возраст устаревшей записи, с (0 — стандарт DNSProxy)','Max expired-entry age, seconds (0 = DNSProxy default)')}</span><input id="p33MaxAge" type="number" min="0" max="86400" onchange={inputChange} bind:value={fields.p33MaxAge}/></label>
        <label class="field"><span>{t('TTL ответа из устаревшего кэша, с (0 — стандарт DNSProxy)','Expired-answer TTL, seconds (0 = DNSProxy default)')}</span><input id="p33AnswerTTL" type="number" min="0" max="300" onchange={inputChange} bind:value={fields.p33AnswerTTL}/></label>
        <label class="check"><span>{t('Изолированный кэш для политик устройств (экспериментально)','Isolated device-policy cache (experimental)')}</span><input id="p33DeviceCache" type="checkbox" onchange={inputChange} bind:checked={fields.p33DeviceCache}/></label>
        <p class="p25-settings-info">{t('Изолированный кэш хранит только обычные A/AAAA без ECS, DO и DNSSEC. Общий кэш при правилах устройств по-прежнему отключён. Лимит памяти общий: максимум 8 МиБ.','Policy cache holds only ordinary A/AAAA without ECS, DO or DNSSEC. Global cache remains disabled with device rules; shared memory limit <=8 MiB.')}</p>
        <div class="actions"><button type="button" class="button" disabled={cacheClearBusy} onclick={clearP33Cache}>{cacheClearBusy?t('Очищаем…','Clearing…'):t('Очистить кэш DNS','Clear DNS cache')}</button><span role="status">{cacheClearStatus}</span></div>
        <label class="field"><span>{t('Объединять одинаковые запросы','Deduplicate pending queries')}</span><select id="p29Pending" onchange={inputChange} bind:value={fields.p29Pending}><option value="default">{t('По умолчанию (вкл.)','Default (on)')}</option><option value="on">{t('Включено','On')}</option><option value="off">{t('Выключено','Off')}</option></select></label>
      </section>
    </div>
  </div>
</section>
