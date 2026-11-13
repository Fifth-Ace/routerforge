<script>
  import {onMount, getContext} from 'svelte';

  // Svelte owns Settings navigation, section selection, read-only telemetry,
  // and every setting input. Existing Lite draft/CAS/apply remains authoritative.
  let active = $state(false);
  let section = $state('service');
  let advanced = $state(false);
  const fields = getContext('routerforge:dns-lite-settings').fields;
  let scope = $derived(fields.p243Scope);
  let ipEnabled = $derived(fields.p241IPEnabled);
  let ipMode = $derived(fields.p241IPMode);
  let ipStatus = $state('Монитор выключен');
  let ipRows = $state([]);
  let performanceStatus = $state('Метрики: нет данных');
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
    {id:'service',ru:'DNS-сервис',en:'DNS service',status:'available'},
    {id:'cache',ru:'Кэш',en:'Cache',status:'available'},
    {id:'ip',ru:'IP Intelligence',en:'IP Intelligence',status:'experimental'},
    {id:'upstream',ru:'Выбор DNS',en:'Upstream selection',status:'available'},
    {id:'security',ru:'Защита запросов',en:'Query protection',status:'available'},
    {id:'privacy',ru:'EDNS / ECS / DNSSEC',en:'EDNS / ECS / DNSSEC',status:'available'},
    {id:'dns64',ru:'DNS64 / rDNS',en:'DNS64 / rDNS',status:'available'},
    {id:'performance',ru:'Производительность',en:'Performance',status:'available'}
  ];
  const settingsHooks = getContext('routerforge:dns-lite-settings');
  const navigationHooks = getContext('routerforge:dns-lite-navigation');
  const inputChange = settingsInputChange;
  const notes = {
    original:['Без изменения IP в DNS-ответе.','No address changes.'],
    round_robin:['Циклически меняет первый IP при каждом новом DNS-запросе, включая ответы из кэша.','Rotates the first IP on each DNS request, including cached responses.'],
    random:['Случайно переставляет все IP; исходный DNS-ответ сохранён.','Shuffles addresses; original DNS response is preserved.'],
    fastest:['С проверками TCP/TLS сортирует по задержке; без них сохраняет исходный порядок.','Sorts measured IPs by latency; otherwise preserves original order.'],
    health_aware:['С проверками ставит доступные IP первыми; неизвестные адреса не считаются упавшими.','Prefers healthy IPs; unknown is not down.'],
    adaptive:['С измерениями учитывает доступность и задержку; без измерений использует ротацию.','Adapts to measurements; rotates otherwise.'],
    sticky:['Закрепляет порядок за проверенным MAC; для неизвестных устройств использует ротацию.','Keeps stable client ordering; rotates for unknown devices.'],
    weighted:['С измерениями веса учитывают задержку; без них используются равные веса.','Weights measured latency; otherwise equal weights.'],
    healthy_only:['Без проверок IP ничего не исключается. DNSSEC-защищённые наборы не сокращаются.','Without checks no IP is excluded. DNSSEC-protected RRsets remain intact.']
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
  async function refreshP29Metrics(){
    if(disposed || !advanced || !active || section!=='performance' || document.hidden) return;
    const controller=new AbortController(); pending.add(controller);
    try{
      const response=await fetch('../service/status',{method:'GET',cache:'no-store',signal:controller.signal});
      if(!response.ok) throw Error('HTTP '+response.status);
      const data=await response.json();
      if(disposed || controller.signal.aborted)return;
      performanceStatus=t('Запросы: ','Queries: ')+String(data.requests||0)+t(' · Кэш: ',' · Cache: ')+String(data.cache_hits||0)+t(' · Ошибки: ',' · Errors: ')+String(data.failures||0)+t(' · DNS-измерений: ',' · Measured upstreams: ')+Object.keys(data.upstream_selection_metrics?.upstreams||{}).length;
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
    <label class="field"><span>Адрес DNS-сервиса</span><input id="serviceListen" onchange={settingsInputChange} bind:value={fields.serviceListen}/></label>
    <label class="field"><span>Разрешённые сети</span><input id="allowedNetworks" spellcheck="false" onchange={settingsInputChange} bind:value={fields.allowedNetworks}/></label>
  </div>
</div>
<div use:ownSlot={advanced ? 'p25Settings_cache' : 'rf-dns-lite-basic-cache-slot'} class="p25-native-cache-field">
  <label class="field"><span>Кэш, МиБ</span><input id="cacheSize" type="number" min="0" max="8" step="1" onchange={settingsInputChange} bind:value={fields.cacheSize}/></label>
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
      <section id="p25Settings_ip" class="p25-settings-section" hidden={section!=='ip'} aria-label="IP Intelligence"><h3>IP Intelligence</h3><p class="p25-settings-info">P24.0 contains nine IP ordering strategies. P24.2 adds opt-in public-IP TCP/TLS checks; unknown never means down.</p>
        <p class="p25-settings-info">{modeNote}</p>
        <p class="p25-settings-info" id="p25IPHealthState">{ipStatus}</p>
            <div id="p241IPControls" class="p241-ip-controls">
      <label class="check p241-ip-toggle"><input type="checkbox" id="p241IPEnabled" onchange={(event)=>{settingsInputChange(event);}} bind:checked={fields.p241IPEnabled}><span>Включить IP Intelligence</span></label>
      <label class="field p241-ip-mode"><span>Алгоритм выбора IP</span>
        <select id="p241IPMode" onchange={(event)=>{settingsInputChange(event);}} aria-label="Алгоритм IP Intelligence" bind:value={fields.p241IPMode}>
          <option value="original">Original · без изменений</option>
          <option value="round_robin">Round-Robin · по кругу</option>
          <option value="random">Random · случайный порядок</option>
          <option value="fastest">Fastest IP · быстрые первыми</option>
          <option value="health_aware">Health-Aware · доступные первыми</option>
          <option value="adaptive">Adaptive · адаптивная ротация</option>
          <option value="sticky">Sticky IP · закрепить за устройством</option>
          <option value="weighted">Weighted · распределение по весам</option>
          <option value="healthy_only">Healthy Only · исключать недоступные</option>
        </select>
      </label>
      <p class="p25-settings-info" id="p241IPModeNote" role="status">{modeNote}</p>
      <div class="p243-scope-settings">
        <label class="field"><span>Область применения</span><select id="p243Scope" onchange={(event)=>{settingsInputChange(event);}} bind:value={fields.p243Scope}><option value="all">Все внешние домены</option><option value="domains">Выбранные домены</option><option value="lists">Выбранные списки</option></select></label>
        <label class="field p243-domains" hidden={scope!=='domains'}><span>Домены, по одному в строке (включая поддомены)</span><textarea id="p243Domains" onchange={settingsInputChange} rows="3" spellcheck="false" placeholder="example.org" bind:value={fields.p243Domains}></textarea></label>
        <label class="field p243-lists" hidden={scope!=='lists'}><span>Списки доменов (Ctrl/Shift для нескольких)</span><select id="p243ListIDs" onchange={settingsInputChange} multiple size="4" bind:value={fields.p243ListIDs}>{#each fields.ipListOptions as option (option.id)}<option value={option.id}>{option.label}</option>{/each}</select></label>
      </div>
      <div class="p242-health-settings">
        <label class="check"><input type="checkbox" id="p242HealthEnabled" onchange={settingsInputChange} bind:checked={fields.p242HealthEnabled}> Фоновая проверка IP-адресов (экспериментально)</label>
        <div class="p242-health-fields">
          <label class="field"><span>Проверка</span><select id="p242HealthProtocol" onchange={settingsInputChange} bind:value={fields.p242HealthProtocol}><option value="tcp">TCP: соединение с портом</option><option value="tls">TLS: проверка сертификата</option><option value="http">HTTP: ответ приложения</option><option value="https">HTTPS: ответ приложения + TLS</option></select></label>
          <label class="field"><span>Порт</span><input id="p242HealthPort" onchange={settingsInputChange} type="number" min="80" max="65535" bind:value={fields.p242HealthPort}></label>
          <label class="field"><span>Интервал, секунд</span><input id="p242HealthInterval" onchange={settingsInputChange} type="number" min="30" max="3600" bind:value={fields.p242HealthInterval}></label>
          <label class="field"><span>Тайм-аут, мс</span><input id="p242HealthTimeout" onchange={settingsInputChange} type="number" min="250" max="3000" bind:value={fields.p242HealthTimeout}></label>
          <label class="field"><span>Неудач до DOWN</span><input id="p242HealthFailures" onchange={settingsInputChange} type="number" min="1" max="5" bind:value={fields.p242HealthFailures}></label>
          <label class="field"><span>Успехов до UP</span><input id="p243Recoveries" onchange={settingsInputChange} type="number" min="1" max="5" bind:value={fields.p243Recoveries}></label>
          <label class="field"><span>Выдержка DOWN, сек</span><input id="p243MinDown" onchange={settingsInputChange} type="number" min="0" max="3600" bind:value={fields.p243MinDown}></label>
          <label class="field"><span>HTTP-путь</span><input id="p243HTTPPath" onchange={settingsInputChange} maxlength="128" placeholder="/health" bind:value={fields.p243HTTPPath}></label>
        </div>
        <p class="p25-settings-info">Проверяются только публичные IP из реальных ответов A/AAAA, не чаще одного подключения в секунду. Приватные и служебные адреса исключены. HTTP/HTTPS проверяют ответ сервиса, 5xx считаются ошибкой; редиректы не выполняются. TCP/TLS подтверждают только соединение.</p>
        <div class="p242-monitor"><div class="p242-monitor-heading"><strong>Монитор IP</strong><small id="p242MonitorStatus">Нет измерений</small></div>
          <div id="p242MonitorRows" class="p242-monitor-rows" aria-live="polite"><span class="muted">Записи появятся после запросов и фоновых проверок.</span></div>
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
      <p class="p25-settings-info">Защита применяется к независимому DNS Router, не меняет правила Keenetic. Все параметры по умолчанию выключены.</p>
      <div class="fields two">
        <label class="field"><span>Лимит запросов одного клиента в секунду (0 — выкл.)</span><input id="p27RateLimit" onchange={settingsInputChange} type="number" min="0" max="2000" bind:value={fields.p27RateLimit}></label>
        <label class="check"><input type="checkbox" id="p27RefuseAny" onchange={settingsInputChange} bind:checked={fields.p27RefuseAny}> Отказывать на DNS ANY (NOTIMP)</label>
      </div>
      <label class="field"><span>Bogus NXDOMAIN — IP-адреса или CIDR (по одному в строке)</span><textarea id="p27BogusIPs" onchange={settingsInputChange} rows="4" spellcheck="false" placeholder="0.0.0.0" bind:value={fields.p27BogusIPs}></textarea></label>
      <p class="p25-settings-info">Если DNS вернул такой IP, RouterForge отправит NXDOMAIN. Не включает блокировку доменов и не проверяет криптографически DNSSEC. CIDR: IPv4 от /24, IPv6 от /64.</p>
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
        <label class="field"><span>{t('DNS для rDNS','rDNS upstream')}</span><select id="p31ReverseUpstream" onchange={inputChange} bind:value={fields.p31ReverseUpstream}><option value="">Выбери прямой DNS…</option>{#each fields.reverseOptions as option (option.id)}<option value={option.id}>{option.label}</option>{/each}</select></label>
      </section>
      <section id="p25Settings_performance" class="p25-settings-section" hidden={section!=='performance'} aria-label={t('Производительность','Performance')}><h3>{t('Производительность','Performance')}</h3><p class="p25-settings-info">{performanceStatus}</p>
        <label class="check"><span>{t('Включить расширенные настройки','Enable performance settings')}</span><input id="p29Enabled" type="checkbox" onchange={inputChange} bind:checked={fields.p29Enabled}/></label>
        <label class="field"><span>{t('Минимум TTL, с','Min cache TTL, s')}</span><input id="p29MinTTL" type="number" min="0" max="3600" onchange={inputChange} bind:value={fields.p29MinTTL}/></label>
        <label class="field"><span>{t('Максимум TTL, с (0 — без ограничения)','Max cache TTL, s (0 = no cap)')}</span><input id="p29MaxTTL" type="number" min="0" max="86400" onchange={inputChange} bind:value={fields.p29MaxTTL}/></label>
        <label class="field"><span>{t('Параллельные обработчики','Concurrent handlers')}</span><input id="p29Concurrent" type="number" min="8" max="64" onchange={inputChange} bind:value={fields.p29Concurrent}/></label>
        <label class="check"><span>Optimistic cache</span><input id="p29Optimistic" type="checkbox" onchange={inputChange} bind:checked={fields.p29Optimistic}/></label>
        <label class="field"><span>{t('Объединять одинаковые запросы','Deduplicate pending queries')}</span><select id="p29Pending" onchange={inputChange} bind:value={fields.p29Pending}><option value="default">{t('По умолчанию (вкл.)','Default (on)')}</option><option value="on">{t('Включено','On')}</option><option value="off">{t('Выключено','Off')}</option></select></label>
      </section>
    </div>
  </div>
</section>
