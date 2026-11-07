<script>
  import {onMount} from 'svelte';
  import {flushSync} from 'svelte';

  // Svelte owns Settings navigation, section selection, read-only telemetry,
  // and every setting input. Existing Lite draft/CAS/apply remains authoritative.
  let active = $state(false);
  let section = $state('service');
  let advanced = $state(false);
  let scope = $state('all');
  let ipEnabled = $state(false);
  let ipMode = $state('original');
  let ipStatus = $state('Монитор выключен');
  let ipRows = $state([]);
  let performanceStatus = $state('Метрики: нет данных');
  let english = $state(false);
  let app, card, tab, workspace, additional, cacheField;
  let originalAdditional, originalCache;
  let originalOpen = false;
  let observer;
  let intervals = [];
  let cleanups = [];
  const t = (ru, en) => english ? en : ru;
  const byId = id => document.getElementById(id);
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
  const inputChange = () => document.dispatchEvent(new Event('routerforge:p28-p31-change'));
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
    if(!advanced || !active || section!=='ip' || document.hidden) return;
    try{
      const response=await fetch('../service/ip-health',{method:'GET',cache:'no-store'});
      if(!response.ok) throw Error('HTTP '+response.status);
      const data=await response.json();
      ipRows=monitorRowsFrom(data);
      ipStatus=data.enabled ? t('Активен · записей: ','Active · entries: ')+ipRows.length : t('Монитор выключен','Monitor disabled');
    }catch(e){ipStatus=t('Ошибка мониторинга: ','Monitor error: ')+e.message;}
  }
  async function refreshP29Metrics(){
    if(!advanced || !active || section!=='performance' || document.hidden) return;
    try{
      const response=await fetch('../service/status',{method:'GET',cache:'no-store'});
      if(!response.ok) throw Error('HTTP '+response.status);
      const data=await response.json();
      performanceStatus=t('Запросы: ','Queries: ')+String(data.requests||0)+t(' · Кэш: ',' · Cache: ')+String(data.cache_hits||0)+t(' · Ошибки: ',' · Errors: ')+String(data.failures||0)+t(' · DNS-измерений: ',' · Measured upstreams: ')+Object.keys(data.upstream_selection_metrics?.upstreams||{}).length;
    }catch(e){performanceStatus=t('Метрики недоступны: ','Metrics unavailable: ')+e.message;}
  }
  function selectSection(id){
    section=id;
    if(id==='ip') refreshIPHealth();
    if(id==='performance') refreshP29Metrics();
  }
  function returnTo(element,position){
    if(!element || !position?.parent) return;
    if(position.next?.parentNode===position.parent) position.parent.insertBefore(element,position.next);
    else position.parent.appendChild(element);
  }
  function placement(){
    const newMode=app?.dataset.dnsMode==='advanced';
    if(newMode===advanced && !newMode)return;
    advanced=newMode;
    if(newMode){
      byId('p25Settings_service')?.appendChild(additional);
      if(additional) additional.open=true;
      byId('p25Settings_cache')?.appendChild(cacheField);
      byId('p25Settings_ip')?.appendChild(byId('p241IPControls'));
      byId('p25Settings_security')?.appendChild(byId('p27SecurityControls'));
      if(byId('p241IPControls'))byId('p241IPControls').hidden=false;
      if(byId('p27SecurityControls'))byId('p27SecurityControls').hidden=false;
    }else{
      returnTo(cacheField,originalCache);
      returnTo(additional,originalAdditional);
      if(additional)additional.open=originalOpen;
      active=false;
      if(app) app.removeAttribute('data-p25-settings-active');
    }
    flushSync();
  }
  function openSettings(){
    if(!advanced) return;
    byId('p21Nav')?.click();
    for(const id of ['p22DevicesCard','p22GroupsCard','p22RulesCard']){const node=byId(id);if(node)node.hidden=true;}
    active=true;
    for(const id of ['navDns','navLogs','p21Nav','p22NavGroups','p22NavRules']){
      const node=byId(id);if(!node)continue;node.classList.remove('selected');node.removeAttribute('aria-current');
    }
    app.dataset.p25SettingsActive='true';
    const label=byId('p23DraftToolbar')?.querySelector('.p22-draft-toolbar-label');
    if(label)label.textContent=t('Настройки DNS · черновик','DNS settings · draft');
    refreshIPHealth();
  }
  function closeSettings(){active=false;if(app)app.removeAttribute('data-p25-settings-active');}
  function watch(target,type,callback){
    if(!target)return;
    target.addEventListener(type,callback);
    cleanups.push(()=>target.removeEventListener(type,callback));
  }
  function syncNotes(){
    ipEnabled=!!byId('p241IPEnabled')?.checked;
    ipMode=byId('p241IPMode')?.value || 'original';
    scope=byId('p243Scope')?.value || 'all';
    // These legacy inputs are still owned by the existing P21 editor. Mirror
    // their derived visibility and note until that editor is migrated.
    const domains=byId('p241IPControls')?.querySelector('.p243-domains');
    const lists=byId('p241IPControls')?.querySelector('.p243-lists');
    if(domains)domains.hidden=scope!=='domains';
    if(lists)lists.hidden=scope!=='lists';
    const note=byId('p241IPModeNote');
    if(note)note.textContent=ipEnabled ? t(...(notes[ipMode]||notes.original)) : t('Функция выключена. DNS-ответы не изменяются.','Feature disabled. DNS responses remain unchanged.');
  }
  onMount(()=>{
    app=byId('dnsLiteApp');workspace=byId('p21Page');
    tab=byId('p25SettingsNav');card=byId('p25SettingsCard');
    additional=byId('additionalSettings');
    const cacheInput=byId('cacheSize');
    cacheField=cacheInput?.closest('.field');
    if(!app||!workspace||!tab||!card||!additional||!cacheField) return;
    english=document.documentElement.lang==='en'||new URLSearchParams(location.search).get('lang')==='en';
    const nextAdditional=additional.nextSibling,nextCache=cacheField.nextSibling;
    originalAdditional={parent:additional.parentNode,next:nextAdditional};
    originalCache={parent:cacheField.parentNode,next:nextCache};
    originalOpen=additional.open;
    byId('navLogs')?.before(tab);workspace.append(card);
    watch(byId('p21Mode'),'change',placement);
    for(const id of ['navDns','navLogs','p21Nav','p22NavGroups','p22NavRules']) watch(byId(id),'click',closeSettings);
    watch(document,'routerforge:p243-render',syncNotes);
    watch(document,'routerforge:p241-render',syncNotes);
    for(const id of ['p243Scope','p241IPEnabled','p241IPMode']) watch(byId(id),'change',syncNotes);
    observer=new MutationObserver(placement);
    observer.observe(app,{attributes:true,attributeFilter:['data-dns-mode']});
    syncNotes();placement();
    intervals=[setInterval(refreshIPHealth,10000),setInterval(refreshP29Metrics,10000)];
    return ()=>{
      for(const timer of intervals)clearInterval(timer);
      for(const dispose of cleanups)dispose();
      observer?.disconnect();
      returnTo(cacheField,originalCache);
      returnTo(additional,originalAdditional);
      if(additional)additional.open=originalOpen;
    };
  });
</script>

<button id="p25SettingsNav" class="tab p22-advanced-tab" type="button" aria-controls="p25SettingsCard" class:selected={active} aria-current={active?'page':undefined} onclick={openSettings}>{t('Настройки','Settings')}</button>
<section id="p25SettingsCard" class="panel rfds-card p25-settings-card" hidden={!active || !advanced} aria-label={t('Настройки DNS Router','DNS Router settings')}>
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
        <label class="check"><span>{t('Включить выбор DNS','Enable upstream selection')}</span><input id="p28Enabled" type="checkbox" onchange={inputChange}/></label>
        <label class="field"><span>{t('Алгоритм','Mode')}</span><select id="p28Mode" onchange={inputChange}><option value="sequential">Failover / Sequential</option><option value="load_balance">Load Balance</option><option value="parallel">Parallel</option><option value="fastest">Fastest DNS</option></select></label>
        <label class="field"><span>{t('Тайм-аут на DNS, мс','Timeout per DNS, ms')}</span><input id="p28Timeout" type="number" min="250" max="5000" value="3000" onchange={inputChange}/></label>
      </section>
      <section id="p25Settings_security" class="p25-settings-section" hidden={section!=='security'} aria-label={t('Защита запросов','Query protection')}><h3>{t('Защита запросов','Query protection')}</h3><p class="p25-settings-info">{t('Частотные ограничения, DNS ANY и Bogus NXDOMAIN действуют только после сохранения и явного применения.','Rate limits, ANY refusal and Bogus NXDOMAIN require explicit save and apply.')}</p></section>
      <section id="p25Settings_privacy" class="p25-settings-section" hidden={section!=='privacy'} aria-label="EDNS / ECS / DNSSEC"><h3>EDNS / ECS / DNSSEC</h3><p class="p25-settings-info">{t('Require AD доверяет только шифрованным DNS; локальная проверка DNSSEC не выполняется.','Require AD trusts only encrypted upstreams; there is no local DNSSEC validation.')}</p>
        <label class="check"><span>{t('Включить настройки EDNS/ECS/DNSSEC','Enable EDNS/ECS/DNSSEC controls')}</span><input id="p30Enabled" type="checkbox" onchange={inputChange}/></label>
        <label class="field"><span>EDNS Client Subnet</span><select id="p30ECSMode" onchange={inputChange}><option value="inherit">{t('Сохранять запрос клиента','Preserve client option')}</option><option value="strip">{t('Удалять ECS','Strip ECS')}</option><option value="fixed">{t('Фиксированный публичный адрес','Fixed public IP')}</option></select></label>
        <label class="field"><span>{t('Публичный IP ECS','Public ECS address')}</span><input id="p30ECSAddress" type="text" onchange={inputChange}/></label>
        <label class="field"><span>DNSSEC</span><select id="p30DNSSECMode" onchange={inputChange}><option value="off">Off</option><option value="request_do">Request DO</option><option value="require_ad">Require AD (encrypted DNS only)</option></select></label>
      </section>
      <section id="p25Settings_dns64" class="p25-settings-section" hidden={section!=='dns64'} aria-label="DNS64 / rDNS"><h3>DNS64 / rDNS</h3><p class="p25-settings-info">{t('DNS64 требует заранее настроенного NAT64; RouterForge не меняет маршруты.','DNS64 requires preconfigured NAT64; RouterForge does not change routing.')}</p>
        <label class="check"><span>{t('Включить DNS64','Enable DNS64')}</span><input id="p31DNS64Enabled" type="checkbox" onchange={inputChange}/></label>
        <label class="field"><span>{t('Префикс NAT64 /96','NAT64 /96 prefix')}</span><input id="p31DNS64Prefix" type="text" value="64:ff9b::/96" onchange={inputChange}/></label>
        <label class="check"><span>{t('Подтверждаю, что NAT64 уже настроен','I confirm NAT64 already works')}</span><input id="p31NAT64Confirmed" type="checkbox" onchange={inputChange}/></label>
        <label class="check"><span>{t('Отдельный DNS для приватных PTR','Custom private PTR resolver')}</span><input id="p31ReverseEnabled" type="checkbox" onchange={inputChange}/></label>
        <label class="field"><span>{t('DNS для rDNS','rDNS upstream')}</span><select id="p31ReverseUpstream" onchange={inputChange}></select></label>
      </section>
      <section id="p25Settings_performance" class="p25-settings-section" hidden={section!=='performance'} aria-label={t('Производительность','Performance')}><h3>{t('Производительность','Performance')}</h3><p class="p25-settings-info">{performanceStatus}</p>
        <label class="check"><span>{t('Включить расширенные настройки','Enable performance settings')}</span><input id="p29Enabled" type="checkbox" onchange={inputChange}/></label>
        <label class="field"><span>{t('Минимум TTL, с','Min cache TTL, s')}</span><input id="p29MinTTL" type="number" min="0" max="3600" value="0" onchange={inputChange}/></label>
        <label class="field"><span>{t('Максимум TTL, с (0 — без ограничения)','Max cache TTL, s (0 = no cap)')}</span><input id="p29MaxTTL" type="number" min="0" max="86400" value="0" onchange={inputChange}/></label>
        <label class="field"><span>{t('Параллельные обработчики','Concurrent handlers')}</span><input id="p29Concurrent" type="number" min="8" max="64" value="32" onchange={inputChange}/></label>
        <label class="check"><span>Optimistic cache</span><input id="p29Optimistic" type="checkbox" onchange={inputChange}/></label>
        <label class="field"><span>{t('Объединять одинаковые запросы','Deduplicate pending queries')}</span><select id="p29Pending" onchange={inputChange}><option value="default">{t('По умолчанию (вкл.)','Default (on)')}</option><option value="on">{t('Включено','On')}</option><option value="off">{t('Выключено','Off')}</option></select></label>
      </section>
    </div>
  </div>
</section>
