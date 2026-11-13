<script>
  import { onMount, tick } from 'svelte';
  import U from './runtime.js';
  import NetworkSparkline from './NetworkSparkline.svelte';
  import NetworkDoctorResults from './NetworkDoctorResults.svelte';
  import NetworkInterfaces from './NetworkInterfaces.svelte';
  import NetworkTrace from './NetworkTrace.svelte';
  import NetworkRouteViews from './NetworkRouteViews.svelte';
  import NetworkFlowViews from './NetworkFlowViews.svelte';
  import NetworkProbeViews from './NetworkProbeViews.svelte';

  // Svelte owns DOM, theme mounting, resize and network adapter lifecycle.
  // runtime.js exports a pure adapter; no global RFNet and no document listeners.
  const params = U.params();
  const locale = params.get('locale') === 'en' ? 'en' : 'ru';
  const views = ['doctor', 'routes', 'flows', 'probes'];
  let view = $state(views.includes(params.get('view')) ? params.get('view') : 'doctor');
  let output = $state({});
  let input = $state({
    doctorTarget: 'example.com', doctorProfile: 'standard', traceTarget:'8.8.8.8', routeTarget:'8.8.8.8',
    routeFamily:'',routeTable:'',routeSearch:'',rulesSearch:'',flowWindow:'manual',flowProtocol:'',
    flowState:'',flowSort:'recent',sourceFilter:'',destinationFilter:'',flowSearch:'',
    probeKind:'ping',probeTarget:'8.8.8.8',probePort:''
  });
  let check = $state({ping:true,dns:true,http:false,tcp:true});
  let state = $state({health:null,summary:null,doctor:null,interfaces:null,traceroute:null,route:null,flows:[],
    flowSource:'',flowEnrichment:{},flowGeneratedAt:'—',activeProbe:null,probeHistory:[],histories:{availability:[],rtt:[],loss:[],sessions:[]}});
  let flowLimit = $state(80);
  let flowBusy = $state(false);
  let doctorBusy = $state(false);
  let probeBusy = $state(false);
  let refreshBusy = $state(false);
  let selectedFlow = $state(null);
  let routeFit = $state('values');
  let routeTables = $state([]);
  let flowProtocols = $state([]);
  let flowStates = $state([]);
  let lastError = $state('');
  let autoTimer;
  let resizeObserver;
  let mounted = false;
  let viewAbort = null;
  // One Svelte mount owns read-only requests; abort pending fetches on unmount.
  const read = async (path,query) => {
    const owner = viewAbort;
    if (!owner || owner.signal.aborted) throw Object.assign(new Error('View unmounted'), {name:'AbortError'});
    const data = await U.request(path,query,owner.signal);
    if (viewAbort !== owner || owner.signal.aborted) throw Object.assign(new Error('View unmounted'), {name:'AbortError'});
    return data;
  };
  const failIfMounted = (key,e) => { if(mounted && e?.name !== 'AbortError') stampError(key,e); };

  const dict = {
    ru:{title:'Сетевые инструменты',kicker:'СЕТЕВЫЕ ИНСТРУМЕНТЫ',subtitle:'Диагностика, анализ маршрутов, исследование трафика и активные проверки сети.',saveReport:'Сохранить отчёт',tabDoctor:'Сетевой доктор',tabRoutes:'Маршруты',tabFlows:'Потоки',tabProbes:'Активные проверки',availability:'Доступность сети',availabilityHint:'По последним активным проверкам',rtt:'Средняя задержка (RTT)',rttHint:'По последним активным проверкам',loss:'Потери пакетов',lossHint:'По последнему Ping',sessions:'Активные сессии',sessionsHint:'Текущий снимок conntrack',doctorTitle:'Быстрая проверка',doctorHint:'Проверьте доступность узла и основные сервисы.',profileStandard:'Профиль: Стандартный',profileWeb:'Профиль: Web',profileDns:'Профиль: DNS',run:'Запустить',copy:'Копировать',doctorEmpty:'Выберите проверки и нажмите «Запустить».',traceTitle:'Трассировка маршрута',traceHint:'Визуализация пути и диагностика на каждом хопе.',traceEmpty:'Трассировка ещё не запускалась.',routeTitle:'Информация о маршруте',routeHint:'Подробная информация о выбранном маршруте и решении ядра.',showRoute:'Показать маршрут',routeEmpty:'Маршрут ещё не выбран.',decisionEmpty:'Нет данных о решении.',flowTitle:'Анализ трафика',flowHint:'Активные соединения и потоки в реальном времени.',flowNow:'Текущий снимок',allInterfaces:'Все интерфейсы',flowSearch:'Поиск по IP, порту…',refresh:'Обновить',flowLoading:'Загрузка conntrack…',moreFlows:'Показать больше соединений →',probeTitle:'Активные проверки',probeHint:'Ручная ограниченная проверка без изменения конфигурации роутера.',probeReady:'Готово к запуску.',success:'Успешно',error:'Ошибка',noData:'нет данных',time:'Время',lossWord:'Потери',hop:'Хоп',node:'Узел',selectedRoute:'Выбранный маршрут',destination:'Назначение',nextHop:'Следующий хоп',interface:'Интерфейс',tableSource:'Источник таблицы',metric:'Метрика',routeType:'Тип маршрута',policy:'Политика',status:'Статус',decisionPath:'Путь принятия решения',lookupDestination:'Поиск назначения',lookupMain:'Поиск в таблице: main',policyCheck:'Проверка политики (PBR)',nextHopChoice:'Выбор следующего хопа',routeInstalled:'Установка маршрута',notConfirmed:'Отдельная policy-таблица этим read-only источником не подтверждена',routeActive:'Маршрут активен в kernel route table',noRoute:'Подходящий IPv4-маршрут не найден.',noResults:'Нет результатов.',tracing:'Трассировка выполняется…',probing:'Выполняются ограниченные проверки…',noFlows:'Нет соединений по текущему фильтру.',total:'Всего',sessionsWord:'сессий',source:'Источник',volume:'Объём',ports:'Порты',protocol:'Протокол',flowState:'Состояние'},
    en:{title:'Network Tools',kicker:'NETWORK TOOLS',subtitle:'Diagnostics, route analysis, traffic inspection, and active network probes.',saveReport:'Save report',tabDoctor:'Network Doctor',tabRoutes:'Routes',tabFlows:'Flows',tabProbes:'Active Probes',availability:'Network availability',availabilityHint:'Based on latest active checks',rtt:'Average latency (RTT)',rttHint:'Based on latest active checks',loss:'Packet loss',lossHint:'Based on the latest Ping',sessions:'Active sessions',sessionsHint:'Current conntrack snapshot',doctorTitle:'Quick check',doctorHint:'Check host reachability and core services.',profileStandard:'Profile: Standard',profileWeb:'Profile: Web',profileDns:'Profile: DNS',run:'Run',copy:'Copy',doctorEmpty:'Choose checks and press Run.',traceTitle:'Route trace',traceHint:'Visualize the path and inspect every hop.',traceEmpty:'Traceroute has not been run yet.',routeTitle:'Route information',routeHint:'Detailed selected-route and kernel decision information.',showRoute:'Show route',routeEmpty:'No route selected yet.',decisionEmpty:'No decision data.',flowTitle:'Traffic analysis',flowHint:'Active connections and flows in real time.',flowNow:'Current snapshot',allInterfaces:'All interfaces',flowSearch:'Search by IP or port…',refresh:'Refresh',flowLoading:'Loading conntrack…',moreFlows:'Show more connections →',probeTitle:'Active Probes',probeHint:'Run a bounded manual check without changing router configuration.',probeReady:'Ready to run.',success:'Success',error:'Error',noData:'no data',time:'Time',lossWord:'Loss',hop:'Hop',node:'Node',selectedRoute:'Selected route',destination:'Destination',nextHop:'Next hop',interface:'Interface',tableSource:'Table source',metric:'Metric',routeType:'Route type',policy:'Policy',status:'Status',decisionPath:'Decision path',lookupDestination:'Destination lookup',lookupMain:'Lookup in table: main',policyCheck:'Policy check (PBR)',nextHopChoice:'Next-hop selection',routeInstalled:'Route installation',notConfirmed:'A separate policy table is not confirmed by this read-only source',routeActive:'Route is active in the kernel route table',noRoute:'No matching IPv4 route was found.',noResults:'No results.',tracing:'Traceroute is running…',probing:'Running bounded probes…',noFlows:'No connections match the current filter.',total:'Total',sessionsWord:'sessions',source:'Source',volume:'Volume',ports:'Ports',protocol:'Protocol',flowState:'State'}
  };

  const tr = key => (dict[locale]?.[key] || key);
  const lx = (ru,en) => locale==='ru' ? ru : en;
  const fmtBytes = U.fmtBytes;
  // Plain text only. HTML and API objects are rendered by Svelte view owners below.
  const putText = (key,value) => { output[key] = String(value ?? ''); void height(); };
  const error = (key,err) => putText(key,err?.message || err);
  async function height(){ if (!mounted) return; await tick(); U.notifyHeight(); }
  const stamp = key => putText(key,lx('Обновлено ','Updated ')+new Date().toLocaleTimeString(locale==='ru'?'ru-RU':'en-US'));
  function chooseView(next){
    if(!views.includes(next)) next='doctor';
    view=next;
    try { sessionStorage.setItem('rf-network-tools-view',next);
      const u=new URL(location.href);u.searchParams.set('view',next);history.replaceState(null,'',u); } catch(_){}
    void height();
  }
  function tabKey(event,current){
    const i=views.indexOf(current);let n=-1;
    if(event.key==='ArrowRight')n=(i+1)%views.length;
    else if(event.key==='ArrowLeft')n=(i+views.length-1)%views.length;
    else if(event.key==='Home')n=0;
    else if(event.key==='End')n=views.length-1;
    if(n>=0){event.preventDefault();chooseView(views[n]);
      void tick().then(()=>document.getElementById('tab-'+views[n])?.focus());}
  }

  const unique = a => [...new Set(a.map(x=>String(x||'').trim()).filter(Boolean))].sort((a,b)=>a.localeCompare(b));
  function historyPoint(name,val){
    if(!Number.isFinite(+val))return;
    const values=state.histories[name];values.push(+val);if(values.length>24)values.shift();
  }
  function updatedKPIs(data){
    const checks=data.checks||[],probes=data.probes||{};
    let ok=0,rtts=[],loss=null;
    for(const kind of checks){const r=probes[kind]||{};if(r.ok)ok++;
      if(Number(r.rtt_avg_ms)>0)rtts.push(Number(r.rtt_avg_ms));
      else if(r.ok&&Number(r.duration_ms)>0&&(kind==='tcp'||kind==='http'))rtts.push(Number(r.duration_ms));
      if(kind==='ping'&&Number.isFinite(Number(r.loss_pct)))loss=Number(r.loss_pct);
    }
    const availability=checks.length?ok/checks.length*100:0;
    const rtt=rtts.length?rtts.reduce((a,b)=>a+b,0)/rtts.length:0;
    putText('kpi-availability',availability.toFixed(1)+'%');
    putText('kpi-rtt',rtt>0?rtt.toFixed(0)+' ms':'—');
    putText('kpi-loss',loss===null?'—':loss.toFixed(1)+'%');
    historyPoint('availability',availability);if(rtt>0)historyPoint('rtt',rtt);if(loss!==null)historyPoint('loss',loss);
  }
  function profileChanged(){const p=input.doctorProfile;check.ping=p!=='dns';check.dns=true;check.http=p==='web';check.tcp=p!=='dns';}
  async function runDoctor(){
    const target=input.doctorTarget.trim(),checks=Object.keys(check).filter(k=>check[k]);
    if(!checks.length){error('doctor-results',tr('doctorEmpty'));return;}
    doctorBusy=true;output['doctor-results']='';
    try{const data=await read('/doctor',{target,checks:checks.join(','),tcp_port:443,http_port:80});state.doctor=data;
      output['doctor-results']='';updatedKPIs(data);input.routeTarget=target;input.traceTarget=target;input.probeTarget=target;
      await Promise.all([runRoute(target),loadInterfaces()]);
    } catch(e){if (mounted && e?.name !== 'AbortError') error('doctor-results',e);}finally{doctorBusy=false;}void height();
  }
  function copyText(s){ if(navigator.clipboard?.writeText) void navigator.clipboard.writeText(s).catch(()=>{}); }
  function copyDoctor(){copyText(JSON.stringify(state.doctor||{},null,2));}
  function copyTrace(){copyText(state.traceroute?(state.traceroute.raw||JSON.stringify(state.traceroute,null,2)):'');}
  function copyRoutes(){copyText(JSON.stringify(filteredRoutes(),null,2));}
  function copyRules(){copyText(JSON.stringify(filteredRules(),null,2));}
  function copySelectedFlow(){if(selectedFlow)copyText(JSON.stringify(selectedFlow,null,2));}
  function copyProbe(){copyText(state.activeProbe?JSON.stringify(state.activeProbe,null,2):'');}
  function stampError(section,e){error(section,e);lastError=String(e?.message||e);}
  // All Network Doctor / Route Inspector values are rendered from owned state,
  // never written to DOM through the former imperative controller.
  async function loadInterfaces(){
    try{const data=await read('/interfaces');state.interfaces=data;output['interface-results']='';}
    catch(e){failIfMounted('interface-results',e);}finally{stamp('interfaces-updated');}
  }
  let traceBusy = $state(false);
  async function runTrace(){traceBusy=true;output['trace-results']='';
    try{const data=await read('/traceroute',{target:input.traceTarget.trim()});state.traceroute=data;output['trace-results']='';}
    catch(e){failIfMounted('trace-results',e);}finally{traceBusy=false;}
  }
  function filteredRoutes(){const data=state.route||{};return [...(data.routes_v4||[]),...(data.routes_v6||[])].filter(r=>{
    if(input.routeFamily && String(r.family||'').toLowerCase()!==input.routeFamily)return false;
    if(input.routeTable && String(r.table||'main')!==input.routeTable)return false;
    const q=input.routeSearch.toLowerCase().trim();if(!q)return true;
    return [r.destination,r.gateway,r.interface,r.source,r.table,r.protocol,r.scope,r.type,r.metric].join(' ').toLowerCase().includes(q);
  });}
  function toggleRouteFit(){routeFit=routeFit==='values'?'grid':'values';}
  function filteredRules(){const data=state.route||{},q=input.rulesSearch.trim().toLowerCase();const rules=[...(data.rules_v4||[]),...(data.rules_v6||[])];return q?rules.filter(r=>JSON.stringify(r).toLowerCase().includes(q)):rules;}
  function routeCount(){const d=state.route||{},n=(d.routes_v4||[]).length+(d.routes_v6||[]).length,v=filteredRoutes().length;return v===n?String(v):v+' / '+n;}
  function ruleCount(){const d=state.route||{},n=(d.rules_v4||[]).length+(d.rules_v6||[]).length,v=filteredRules().length;return v===n?String(v):v+' / '+n;}
  async function runRoute(target){const dest=target||input.routeTarget.trim();
    try{const data=await read('/routes',{target:dest});state.route=data;
      routeTables=unique([...(data.routes_v4||[]),...(data.routes_v6||[])].map(r=>r.table||'main'));output['route']='';}
    catch(e){failIfMounted('route',e);}
    finally{stamp('route-updated');}
  }
  function normalizeFlow(raw){const original=raw.original||{},reply=raw.reply||{},socket=raw.socket||{},policy=raw.policy||{},decision=policy.kernel_decision||{},route=raw.route||{};
    return {protocol:raw.protocol||'',state:raw.state||'ACTIVE',source:original.source||'',source_port:original.source_port||'',destination:original.destination||'',destination_port:original.destination_port||'',
      packets:Number(original.packets||0)+Number(reply.packets||0),bytes:Number(original.bytes||0)+Number(reply.bytes||0),timeout_seconds:raw.timeout_seconds||0,
      seen_at:state.flowGeneratedAt||'—',mark:raw.mark||'',process:socket.available?socket.process||'':'',pid:socket.available?Number(socket.pid||0):0,
      route_interface:decision.available?decision.interface||'':route.available?route.interface||'':'',_raw:raw};
  }
  function flowKey(f){const raw=f?._raw||{},original=raw.original||{};
    return [raw.protocol||f.protocol||'',original.source||f.source||'',original.source_port||f.source_port||'',original.destination||f.destination||'',original.destination_port||f.destination_port||'',raw.mark||f.mark||''].join('|');
  }
  function flowList(){const search=input.flowSearch.trim().toLowerCase();const arr=state.flows.filter(f=>
      (!input.flowProtocol||String(f.protocol||'').toUpperCase()===input.flowProtocol)&&
      (!input.flowState||String(f.state||'ACTIVE').toUpperCase()===input.flowState)&&
      (!search||JSON.stringify(f).toLowerCase().includes(search)) ).slice();
    if(input.flowSort==='bytes')arr.sort((a,b)=>Number(b.bytes||0)-Number(a.bytes||0));
    else if(input.flowSort==='packets')arr.sort((a,b)=>Number(b.packets||0)-Number(a.packets||0));
    else if(input.flowSort==='destination')arr.sort((a,b)=>String(a.destination||'').localeCompare(String(b.destination||'')));
    return arr;
  }
  async function refreshFlows(){if(flowBusy)return;flowBusy=true;
    try{const params={limit:256};if(input.flowProtocol)params.protocol=input.flowProtocol;
      if(input.sourceFilter.trim())params.source=input.sourceFilter.trim();if(input.destinationFilter.trim())params.destination=input.destinationFilter.trim();
      const data=await read('/flow-explorer',params);
      state.flowGeneratedAt=data.generated_at?new Date(data.generated_at).toLocaleTimeString(locale==='ru'?'ru-RU':'en-US'):'—';
      state.flows=(data.flows||[]).map(normalizeFlow);state.flowSource=data.source||'';state.flowEnrichment=data.enrichment||{};
      flowProtocols=unique(state.flows.map(f=>String(f.protocol||'').toUpperCase()));
      flowStates=unique(state.flows.map(f=>String(f.state||'ACTIVE').toUpperCase()));
      if(input.flowProtocol&&!flowProtocols.includes(input.flowProtocol))input.flowProtocol='';
      if(input.flowState&&!flowStates.includes(input.flowState))input.flowState='';
      output['flow-results']='';putText('kpi-sessions',state.flows.length.toLocaleString(locale==='ru'?'ru-RU':'en-US'));
      historyPoint('sessions',state.flows.length);
    }catch(e){failIfMounted('flow-results',e);}finally{flowBusy=false;stamp('flow-updated');}
  }
  function clearFlowFilters(){Object.assign(input,{flowProtocol:'',flowState:'',flowSort:'recent',flowSearch:'',sourceFilter:'',destinationFilter:''});selectedFlow=null;flowLimit=80;void refreshFlows();}
  function moreFlows(){flowLimit+=80;}
  function windowChanged(){if(autoTimer)clearInterval(autoTimer);autoTimer=null;
    const n=+input.flowWindow||0;if(n>0)autoTimer=setInterval(()=>{if(!document.hidden)void refreshFlows();},n*1000);void refreshFlows();}

  let needsProbePort = $derived(input.probeKind==='tcp'||input.probeKind==='http');
  function probeKindChanged(){input.probePort=input.probeKind==='tcp'?'443':input.probeKind==='http'?'80':'';}
  async function runProbe(){if(probeBusy)return;probeBusy=true;output['probe-summary']='';
    try{const data=await read('/probe',{kind:input.probeKind,target:input.probeTarget.trim(),port:+input.probePort||0});state.activeProbe=data;
      state.probeHistory.push({at:new Date().toLocaleTimeString(locale==='ru'?'ru-RU':'en-US'),result:data});if(state.probeHistory.length>12)state.probeHistory.shift();
      putText('probe-output',JSON.stringify(data,null,2));
    }catch(e){if (mounted && e?.name !== 'AbortError') {stampError('probe-summary',e);putText('probe-output','FAIL: '+e.message);}}finally{probeBusy=false;stamp('probe-updated');}
  }
  function clearProbeHistory(){state.probeHistory=[];}
  function saveReport(){const report={generated_at:new Date().toISOString(),module:'network-tools',health:state.health,summary:state.summary,doctor:state.doctor,interfaces:state.interfaces,traceroute:state.traceroute,route:state.route,flow_source:state.flowSource,flow_enrichment:state.flowEnrichment,flows:state.flows,selected_flow:selectedFlow,active_probe:state.activeProbe,probe_history:state.probeHistory};
    const url=URL.createObjectURL(new Blob([JSON.stringify(report,null,2)+'\n'],{type:'application/json'}));
    const a=document.createElement('a');a.href=url;a.download='network-tools-report-'+new Date().toISOString().replace(/[:.]/g,'-')+'.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
  }
  async function refreshAll(){if(refreshBusy)return;refreshBusy=true;
    try{await Promise.all([refreshFlows(),runRoute(input.routeTarget.trim()||'8.8.8.8'),loadInterfaces()]);}
    finally{refreshBusy=false;void height();}
  }
  onMount(()=>{
    mounted=true;viewAbort = new AbortController();
    U.applyTheme();
    document.documentElement.lang=locale;document.title=tr('title');
    if(!params.get('view'))try{const s=sessionStorage.getItem('rf-network-tools-view');if(views.includes(s))view=s;}catch(_){}
    probeKindChanged();
    void (async()=>{try{state.health=await read('/health');state.summary=await read('/summary');
      putText('kpi-sessions',Number(state.summary?.active_sessions||0).toLocaleString(locale==='ru'?'ru-RU':'en-US'));
      historyPoint('sessions',Number(state.summary?.active_sessions||0));
      await refreshAll();}catch(e){if(mounted && e?.name !== 'AbortError') lastError=String(e.message||e);}})();
    const resized=()=>void height();window.addEventListener('resize',resized);
    if(typeof ResizeObserver!=='undefined'){resizeObserver=new ResizeObserver(resized);resizeObserver.observe(document.querySelector('.nt-page')||document.body);}
    return()=>{mounted=false;viewAbort?.abort();viewAbort=null;if(autoTimer)clearInterval(autoTimer);resizeObserver?.disconnect();window.removeEventListener('resize',resized);};
  });
</script>

<main class="nt-page">
  <header class="nt-heading">
    <div>
      <span class="nt-kicker" data-i18n="kicker">{tr("kicker")}</span>
      <h1 data-i18n="title">{tr("title")}</h1>
      <p data-i18n="subtitle">{tr("subtitle")}</p>
    </div>
    <div class="nt-heading-actions">
      <button class="nt-button" id="refresh-all" type="button" onclick={refreshAll} disabled={refreshBusy}><span>↻</span><span id="refresh-all-label">{lx("Обновить данны","Refresh data")}</span></button>
      <button class="nt-button" id="save-report" onclick={saveReport}><span>▣</span><span data-i18n="saveReport">{tr("saveReport")}</span></button>
    </div>
  </header>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <nav class="nt-tabs" role="tablist" aria-label="Network Tools" data-contract="Network Doctor | Route Inspector | Flow Explorer | Active Probes">
    <button class="nt-tab" id="tab-doctor" role="tab" aria-controls="panel-doctor" data-view="doctor" class:active={view==="doctor"} aria-selected={view==="doctor"} tabindex={view==="doctor"?0:-1} onclick={() => chooseView("doctor")} onkeydown={(event) => tabKey(event,"doctor")}><span data-i18n="tabDoctor">{tr("tabDoctor")}</span></button>
    <button class="nt-tab" id="tab-routes" role="tab" aria-controls="panel-routes" data-view="routes" class:active={view==="routes"} aria-selected={view==="routes"} tabindex={view==="routes"?0:-1} onclick={() => chooseView("routes")} onkeydown={(event) => tabKey(event,"routes")}><span data-i18n="tabRoutes">{tr("tabRoutes")}</span></button>
    <button class="nt-tab" id="tab-flows" role="tab" aria-controls="panel-flows" data-view="flows" class:active={view==="flows"} aria-selected={view==="flows"} tabindex={view==="flows"?0:-1} onclick={() => chooseView("flows")} onkeydown={(event) => tabKey(event,"flows")}><span data-i18n="tabFlows">{tr("tabFlows")}</span></button>
    <button class="nt-tab" id="tab-probes" role="tab" aria-controls="panel-probes" data-view="probes" class:active={view==="probes"} aria-selected={view==="probes"} tabindex={view==="probes"?0:-1} onclick={() => chooseView("probes")} onkeydown={(event) => tabKey(event,"probes")}><span data-i18n="tabProbes">{tr("tabProbes")}</span></button>
  </nav>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-doctor" role="tabpanel" aria-labelledby="tab-doctor" data-panel="doctor" class:active={view==="doctor"} aria-hidden={view!=="doctor"}>
    <section class="nt-kpis" aria-label="Network summary">
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="availability">{tr("availability")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-availability">{output["kpi-availability"] || "—"}</div><div class="nt-trend">LIVE</div><div id="spark-availability"><NetworkSparkline values={state.histories.availability}/></div></div><small data-i18n="availabilityHint">{tr("availabilityHint")}</small></article>
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="rtt">{tr("rtt")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-rtt">{output["kpi-rtt"] || "—"}</div><div class="nt-trend">LIVE</div><div id="spark-rtt"><NetworkSparkline values={state.histories.rtt}/></div></div><small data-i18n="rttHint">{tr("rttHint")}</small></article>
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="loss">{tr("loss")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-loss">{output["kpi-loss"] || "—"}</div><div class="nt-trend bad">LIVE</div><div id="spark-loss"><NetworkSparkline values={state.histories.loss} bad={true}/></div></div><small data-i18n="lossHint">{tr("lossHint")}</small></article>
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="sessions">{tr("sessions")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-sessions">{output["kpi-sessions"] || "—"}</div><div class="nt-trend">LIVE</div><div id="spark-sessions"><NetworkSparkline values={state.histories.sessions}/></div></div><small data-i18n="sessionsHint">{tr("sessionsHint")}</small></article>
    </section>

    <article class="nt-panel">
      <div class="nt-panel-head"><div><strong data-i18n="doctorTitle">{tr("doctorTitle")}</strong><small data-i18n="doctorHint">{tr("doctorHint")}</small></div></div>
      <div class="nt-toolbar four">
        <input id="doctor-target" aria-label="Target" bind:value={input.doctorTarget}>
        <select id="doctor-profile" bind:value={input.doctorProfile} onchange={() => profileChanged()}><option value="standard" data-i18n="profileStandard">{tr("profileStandard")}</option><option value="web" data-i18n="profileWeb">{tr("profileWeb")}</option><option value="dns" data-i18n="profileDns">{tr("profileDns")}</option></select>
        <button class="nt-button primary" id="doctor-run" onclick={runDoctor} disabled={doctorBusy}><span data-i18n="run">{tr("run")}</span>&nbsp;▶</button>
        <button class="nt-button" id="doctor-copy" type="button" onclick={copyDoctor}><span>▣</span><span id="doctor-copy-label">Копировать</span></button>
      </div>
      <div class="nt-checks"><label class="nt-check"><input type="checkbox" id="check-ping" bind:checked={check.ping}> Ping</label><label class="nt-check"><input type="checkbox" id="check-dns" bind:checked={check.dns}> DNS</label><label class="nt-check"><input type="checkbox" id="check-http" bind:checked={check.http}> HTTP</label><label class="nt-check"><input type="checkbox" id="check-tcp" bind:checked={check.tcp}> TCP</label></div>
      <div class="nt-result-list" id="doctor-results"><NetworkDoctorResults data={state.doctor} busy={doctorBusy} message={output["doctor-results"]} {lx} {tr}/></div>
    </article>

    <article class="nt-panel" id="interface-inspector">
      <div class="nt-panel-head">
        <div>
          <strong id="interfaces-title">{lx("Интерфейсы","Interfaces")}</strong>
          <small id="interfaces-hint">{lx("Состояние линка, IP-адреса, счетчики и владение маршрутом по умолчанию.","Link state, IP addresses, counters and default-route ownership.")}</small>
        </div>
        <div class="nt-panel-head-actions"><span class="panel-updated" id="interfaces-updated">{output["interfaces-updated"] || "—"}</span><button class="nt-button compact" id="interfaces-refresh" onclick={loadInterfaces}>{lx("Обновить","Refresh")}</button></div>
      </div>
      <div class="nt-table-wrap" id="interface-results"><NetworkInterfaces data={state.interfaces} message={output["interface-results"]} {lx} {fmtBytes}/></div>
    </article>
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-routes" role="tabpanel" aria-labelledby="tab-routes" data-panel="routes" class:active={view==="routes"} aria-hidden={view!=="routes"}>
    <div class="nt-stack">
      <article class="nt-panel">
        <div class="nt-panel-head"><div><strong data-i18n="traceTitle">{tr("traceTitle")}</strong><small data-i18n="traceHint">{tr("traceHint")}</small></div></div>
        <div class="nt-toolbar four"><input id="trace-target" bind:value={input.traceTarget}><div class="nt-static-mode" id="trace-mode">{lx('Режим: ICMP traceroute','Mode: ICMP traceroute')}</div><button class="nt-button primary" id="trace-run" onclick={runTrace}><span data-i18n="run">{tr("run")}</span></button><button class="nt-button" id="trace-copy" onclick={copyTrace}><span>▣</span><span data-i18n="copy">{tr("copy")}</span></button></div>
        <div class="nt-table-wrap" id="trace-results"><NetworkTrace data={state.traceroute} message={output["trace-results"]} busy={traceBusy} {lx} {tr}/></div>
      </article>

      <article class="nt-panel">
        <div class="nt-panel-head"><div><strong data-i18n="routeTitle">{tr("routeTitle")}</strong><small data-i18n="routeHint">{tr("routeHint")}</small></div><div class="nt-panel-head-actions"><span class="panel-updated" id="route-updated">{output["route-updated"] || "—"}</span></div></div>
        <div class="nt-toolbar"><input id="route-target" bind:value={input.routeTarget}><span></span><button class="nt-button primary" id="route-run" data-i18n="showRoute" onclick={() => runRoute()}>{tr("showRoute")}</button></div>
        <div class="route-capabilities" id="route-capabilities"><NetworkRouteViews section="capabilities" data={state.route} message={output["route"]} {lx}/></div>
        <div class="route-card-grid"><div class="route-facts" id="route-facts"><NetworkRouteViews section="facts" data={state.route} message={output["route"]} {lx}/></div><div class="route-decision" id="route-decision"><NetworkRouteViews section="decision" data={state.route} target={input.routeTarget} message={output["route"]} {lx}/></div></div>
        <div class="route-inspector-grid">
          <section class="route-subpanel">
            <div class="route-subpanel-head"><div><strong id="route-inventory-title">{lx("Таблицы маршрутизации","Routing tables")}</strong><small id="route-inventory-hint">{lx("IPv4/IPv6 маршруты из всех доступных kernel tables.","IPv4/IPv6 routes from every kernel table available to this runtime.")}</small></div><div class="route-subpanel-actions"><span class="route-count" id="route-routes-count">{state.route ? routeCount() : "0"}</span><button class="nt-button compact" id="route-copy-routes" type="button" onclick={copyRoutes}>{lx("Копировать","Copy")}</button></div></div>
            <div class="route-filter-bar">
              <select id="route-family-filter" aria-label="IP family" bind:value={input.routeFamily}><option value="">{lx('Все IP','All IP')}</option><option value="ipv4">IPv4</option><option value="ipv6">IPv6</option></select>
              <select id="route-table-filter" aria-label="Routing table" bind:value={input.routeTable}><option value="">{lx('Все таблицы','All tables')}</option>{#each routeTables as table (table)}<option value={table}>{table}</option>{/each}</select>
              <input id="route-search" aria-label="Route search" placeholder={lx("Назначение, via, dev, src…","Destination, via, dev, src…")} bind:value={input.routeSearch}>
              <button class="nt-button compact" id="route-fit-toggle" type="button" onclick={toggleRouteFit}>{routeFit === "values" ? lx("Колонки: по данным","Columns: fit values") : lx("Колонки: по сетке","Columns: fixed grid")}</button>
            </div>
            <div class="nt-table-wrap" id="route-routes"><NetworkRouteViews section="routes" data={state.route} family={input.routeFamily} table={input.routeTable} search={input.routeSearch} fit={routeFit} message={output["route"]} {lx}/></div>
          </section>
          <section class="route-subpanel">
            <div class="route-subpanel-head"><div><strong id="route-rules-title">{lx("Правила PBR","Policy rules")}</strong><small id="route-rules-hint">{lx("Read-only ip rule snapshot; route get показывает решение для трафика самого роутера.","This is a read-only ip rule snapshot; route get describes router-originated traffic.")}</small></div><div class="route-subpanel-actions"><span class="route-count" id="route-rules-count">{state.route ? ruleCount() : "0"}</span><button class="nt-button compact" id="route-copy-rules" type="button" onclick={copyRules}>{lx("Копировать","Copy")}</button></div></div><div class="route-rules-filter"><input id="route-rules-search" aria-label="Policy rule search" placeholder={lx("Поиск по priority, from, to, mark, table…","Search priority, from, to, mark, table…")} bind:value={input.rulesSearch}></div>
            <div class="nt-table-wrap" id="route-rules"><NetworkRouteViews section="rules" data={state.route} rulesSearch={input.rulesSearch} message={output["route"]} {lx}/></div>
          </section>
        </div>
      </article>
    </div>
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-flows" role="tabpanel" aria-labelledby="tab-flows" data-panel="flows" class:active={view==="flows"} aria-hidden={view!=="flows"}>
    <article class="nt-panel">
      <div class="nt-panel-head"><div><strong data-i18n="flowTitle">{tr("flowTitle")}</strong><small data-i18n="flowHint">{tr("flowHint")}</small></div><div class="nt-panel-head-actions"><span class="panel-updated" id="flow-updated">{output["flow-updated"] || "—"}</span><button class="nt-button compact" id="flow-clear" type="button" onclick={clearFlowFilters}>{lx("Сбросить фильтры","Reset filters")}</button><button class="nt-button compact" id="flow-copy-selected" type="button" onclick={copySelectedFlow} disabled={!selectedFlow}>{lx("Копировать поток","Copy flow")}</button></div></div>
      <div class="nt-toolbar flow">
        <select id="flow-window" aria-label="Flow refresh mode" bind:value={input.flowWindow} onchange={() => windowChanged()}><option value="manual">{lx('Ручное обновление','Manual refresh')}</option><option value="5">{lx('Авто · 5 с','Auto · 5 s')}</option><option value="15">{lx('Авто · 15 с','Auto · 15 s')}</option></select>
        <select id="flow-protocol" aria-label="Protocol" bind:value={input.flowProtocol}><option value="">{lx('Все протоколы','All protocols')}</option>{#each flowProtocols as protocol (protocol)}<option value={protocol}>{protocol}</option>{/each}</select>
        <select id="flow-state" aria-label="State" bind:value={input.flowState}><option value="">{lx('Все состояния','All states')}</option>{#each flowStates as state (state)}<option value={state}>{state}</option>{/each}</select>
        <select id="flow-sort" aria-label="Sort" bind:value={input.flowSort}><option value="recent">{lx('Сначала новые','Newest first')}</option><option value="bytes">{lx('По объёму','By volume')}</option><option value="packets">{lx('По пакетам','By packets')}</option><option value="destination">{lx('По назначению','By destination')}</option></select>
        <input id="flow-source-filter" aria-label="Exact source IP" placeholder="Source IP (exact)" bind:value={input.sourceFilter}>
        <input id="flow-destination-filter" aria-label="Exact destination IP" placeholder="Destination IP (exact)" bind:value={input.destinationFilter}>
        <input id="flow-search" data-i18n-placeholder="flowSearch" placeholder={lx("Поиск: IP, порт, состояние…","Search: IP, port, state…")} bind:value={input.flowSearch}>
        <button class="nt-button primary" id="flow-refresh" data-i18n="refresh" onclick={refreshFlows} disabled={flowBusy}>{tr("refresh")}</button>
      </div>
      <div class="flow-summary" id="flow-summary"><NetworkFlowViews section="summary" list={flowList()} source={state.flowSource} enrichment={state.flowEnrichment} {lx} {tr} {fmtBytes} {flowKey}/></div>
      <div class="flow-detail" id="flow-detail"><NetworkFlowViews section="detail" selected={selectedFlow} {lx} {tr} {fmtBytes} {flowKey}/></div>
      <div class="nt-table-wrap" id="flow-results" role="grid"><NetworkFlowViews section="table" list={flowList()} selected={selectedFlow} limit={flowLimit} message={output["flow-results"]} {lx} {tr} {fmtBytes} {flowKey} onSelect={(flow) => { selectedFlow=flow; }}/></div>
      <div class="flow-footer"><button id="flow-more" data-i18n="moreFlows" onclick={moreFlows}>{tr("moreFlows")}</button><span id="flow-count">{`${lx('Показано: ','Showing: ')}${Math.min(flowLimit,flowList().length)} / ${flowList().length}`}</span></div>
    </article>
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-probes" role="tabpanel" aria-labelledby="tab-probes" data-panel="probes" class:active={view==="probes"} aria-hidden={view!=="probes"}>
    <article class="nt-panel">
      <div class="nt-panel-head"><div><strong data-i18n="probeTitle">{tr("probeTitle")}</strong><small data-i18n="probeHint">{tr("probeHint")}</small></div><div class="nt-panel-head-actions"><span class="panel-updated" id="probe-updated">{output["probe-updated"] || "—"}</span><button class="nt-button compact" id="probe-clear-history" type="button" onclick={clearProbeHistory}>{lx("Очистить историю","Clear history")}</button></div></div>
      <div class="active-grid">
        <select id="probe-kind" bind:value={input.probeKind} onchange={() => probeKindChanged()}><option value="ping">Ping</option><option value="dns">DNS</option><option value="tcp">TCP</option><option value="http">HTTP</option></select>
        <input id="probe-target" bind:value={input.probeTarget}>
        <input id="probe-port" inputmode="numeric" placeholder="n/a" bind:value={input.probePort} disabled={!needsProbePort}>
        <button class="nt-button primary" id="probe-run" onclick={runProbe} disabled={probeBusy}><span data-i18n="run">{tr("run")}</span>&nbsp;&#9654;</button>
        <button class="nt-button" id="probe-copy" type="button" onclick={copyProbe}>{lx("Копировать","Copy")}</button>
      </div>
      <div class="probe-summary" id="probe-summary"><NetworkProbeViews section="summary" data={state.activeProbe} busy={probeBusy} message={output["probe-summary"]} {lx} {tr}/></div><div class="probe-history-title">Recent probes</div><div class="probe-history" id="probe-history"><NetworkProbeViews section="history" history={state.probeHistory} {lx} {tr}/></div><details class="probe-raw" id="probe-raw"><summary id="probe-raw-summary">{lx("Технические данны JSON","Raw JSON details")}</summary><pre class="nt-console" id="probe-output" data-i18n="probeReady">{output["probe-output"] || tr("probeReady")}</pre></details>
    </article>
  </section>
</main>