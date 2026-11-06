<script>
  import { onMount, tick } from 'svelte';

  // Svelte 5 owns DOM, state, requests and events. The existing runtime.js is a
  // theme/API/iframe-height utility only; the legacy app.js is not loaded or shipped.
  const U = window.RFNet;
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

  const dict = {
    ru:{title:'Сетевые инструменты',kicker:'СЕТЕВЫЕ ИНСТРУМЕНТЫ',subtitle:'Диагностика, анализ маршрутов, исследование трафика и активные проверки сети.',saveReport:'Сохранить отчёт',tabDoctor:'Сетевой доктор',tabRoutes:'Маршруты',tabFlows:'Потоки',tabProbes:'Активные проверки',availability:'Доступность сети',availabilityHint:'По последним активным проверкам',rtt:'Средняя задержка (RTT)',rttHint:'По последним активным проверкам',loss:'Потери пакетов',lossHint:'По последнему Ping',sessions:'Активные сессии',sessionsHint:'Текущий снимок conntrack',doctorTitle:'Быстрая проверка',doctorHint:'Проверьте доступность узла и основные сервисы.',profileStandard:'Профиль: Стандартный',profileWeb:'Профиль: Web',profileDns:'Профиль: DNS',run:'Запустить',copy:'Копировать',doctorEmpty:'Выберите проверки и нажмите «Запустить».',traceTitle:'Трассировка маршрута',traceHint:'Визуализация пути и диагностика на каждом хопе.',traceEmpty:'Трассировка ещё не запускалась.',routeTitle:'Информация о маршруте',routeHint:'Подробная информация о выбранном маршруте и решении ядра.',showRoute:'Показать маршрут',routeEmpty:'Маршрут ещё не выбран.',decisionEmpty:'Нет данных о решении.',flowTitle:'Анализ трафика',flowHint:'Активные соединения и потоки в реальном времени.',flowNow:'Текущий снимок',allInterfaces:'Все интерфейсы',flowSearch:'Поиск по IP, порту…',refresh:'Обновить',flowLoading:'Загрузка conntrack…',moreFlows:'Показать больше соединений →',probeTitle:'Активные проверки',probeHint:'Ручная ограниченная проверка без изменения конфигурации роутера.',probeReady:'Готово к запуску.',success:'Успешно',error:'Ошибка',noData:'нет данных',time:'Время',lossWord:'Потери',hop:'Хоп',node:'Узел',selectedRoute:'Выбранный маршрут',destination:'Назначение',nextHop:'Следующий хоп',interface:'Интерфейс',tableSource:'Источник таблицы',metric:'Метрика',routeType:'Тип маршрута',policy:'Политика',status:'Статус',decisionPath:'Путь принятия решения',lookupDestination:'Поиск назначения',lookupMain:'Поиск в таблице: main',policyCheck:'Проверка политики (PBR)',nextHopChoice:'Выбор следующего хопа',routeInstalled:'Установка маршрута',notConfirmed:'Отдельная policy-таблица этим read-only источником не подтверждена',routeActive:'Маршрут активен в kernel route table',noRoute:'Подходящий IPv4-маршрут не найден.',noResults:'Нет результатов.',tracing:'Трассировка выполняется…',probing:'Выполняются ограниченные проверки…',noFlows:'Нет соединений по текущему фильтру.',total:'Всего',sessionsWord:'сессий',source:'Источник',volume:'Объём',ports:'Порты',protocol:'Протокол',flowState:'Состояние'},
    en:{title:'Network Tools',kicker:'NETWORK TOOLS',subtitle:'Diagnostics, route analysis, traffic inspection, and active network probes.',saveReport:'Save report',tabDoctor:'Network Doctor',tabRoutes:'Routes',tabFlows:'Flows',tabProbes:'Active Probes',availability:'Network availability',availabilityHint:'Based on latest active checks',rtt:'Average latency (RTT)',rttHint:'Based on latest active checks',loss:'Packet loss',lossHint:'Based on the latest Ping',sessions:'Active sessions',sessionsHint:'Current conntrack snapshot',doctorTitle:'Quick check',doctorHint:'Check host reachability and core services.',profileStandard:'Profile: Standard',profileWeb:'Profile: Web',profileDns:'Profile: DNS',run:'Run',copy:'Copy',doctorEmpty:'Choose checks and press Run.',traceTitle:'Route trace',traceHint:'Visualize the path and inspect every hop.',traceEmpty:'Traceroute has not been run yet.',routeTitle:'Route information',routeHint:'Detailed selected-route and kernel decision information.',showRoute:'Show route',routeEmpty:'No route selected yet.',decisionEmpty:'No decision data.',flowTitle:'Traffic analysis',flowHint:'Active connections and flows in real time.',flowNow:'Current snapshot',allInterfaces:'All interfaces',flowSearch:'Search by IP or port…',refresh:'Refresh',flowLoading:'Loading conntrack…',moreFlows:'Show more connections →',probeTitle:'Active Probes',probeHint:'Run a bounded manual check without changing router configuration.',probeReady:'Ready to run.',success:'Success',error:'Error',noData:'no data',time:'Time',lossWord:'Loss',hop:'Hop',node:'Node',selectedRoute:'Selected route',destination:'Destination',nextHop:'Next hop',interface:'Interface',tableSource:'Table source',metric:'Metric',routeType:'Route type',policy:'Policy',status:'Status',decisionPath:'Decision path',lookupDestination:'Destination lookup',lookupMain:'Lookup in table: main',policyCheck:'Policy check (PBR)',nextHopChoice:'Next-hop selection',routeInstalled:'Route installation',notConfirmed:'A separate policy table is not confirmed by this read-only source',routeActive:'Route is active in the kernel route table',noRoute:'No matching IPv4 route was found.',noResults:'No results.',tracing:'Traceroute is running…',probing:'Running bounded probes…',noFlows:'No connections match the current filter.',total:'Total',sessionsWord:'sessions',source:'Source',volume:'Volume',ports:'Ports',protocol:'Protocol',flowState:'State'}
  };

  const tr = key => (dict[locale]?.[key] || key);
  const lx = (ru,en) => locale==='ru' ? ru : en;
  const escape = U.esc;
  const fmtBytes = U.fmtBytes;
  const badge = U.badge;
  const put = (key,html) => { output[key] = html; void height(); };
  const putText = (key,value) => put(key,escape(value));
  const error = (key,err) => put(key,`<div class="rf-error">${escape(err?.message || err)}</div>`);
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
  function spark(values,bad){
    if(!values?.length) return `<svg class="spark${bad?' bad':''}" viewBox="0 0 100 32"></svg>`;
    let v=values.slice();if(v.length===1)v=[v[0],v[0]];
    let min=Math.min(...v),max=Math.max(...v);if(max===min){min--;max++;}
    const pts=v.map((num,i)=>`${(i/(v.length-1)*100).toFixed(1)},${(28-((num-min)/(max-min)*22)).toFixed(1)}`);
    return `<svg class="spark${bad?' bad':''}" viewBox="0 0 100 32" preserveAspectRatio="none"><polygon class="area" points="0,32 ${pts.join(' ')} 100,32"></polygon><polyline class="line" points="${pts.join(' ')}"></polyline></svg>`;
  }
  function historyPoint(name,val){
    if(!Number.isFinite(+val))return;
    const values=state.histories[name];values.push(+val);if(values.length>24)values.shift();
    put('spark-'+name,spark(values,name==='loss'));
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
  const resultIcon=kind=>({ping:'⌁',dns:'◎',http:'▣',tcp:'◉'}[kind]||'•');
  const resultLabel=kind=>({ping:'Ping',dns:'DNS',http:'HTTP',tcp:'TCP'}[kind]||kind);
  function resultDetail(kind,r){if(!r)return tr('noData');
    if(kind==='ping')return r.rtt_avg_ms?'RTT '+Number(r.rtt_avg_ms).toFixed(1)+' ms':tr('time')+' '+r.duration_ms+' ms';
    if(kind==='dns')return (r.addresses||[]).join(', ')||tr('time')+' '+r.duration_ms+' ms';
    if(kind==='http')return r.status_code?'HTTP '+r.status_code+' · '+r.duration_ms+' ms':tr('time')+' '+r.duration_ms+' ms';
    return tr('time')+' '+r.duration_ms+' ms';
  }
  const statusLabel = s => ({ok:lx('НОРМА','OK'),warn:lx('ВНИМАНИЕ','WARN'),fail:lx('ОШИБКА','FAIL'),skipped:lx('ПРОПУЩЕНО','SKIPPED'),unavailable:'N/A'}[s] || s);
  function stageLabel(id){const values={default_route:['Маршрут по умолчанию','Default route'],interface:['Сетевой интерфейс','Network interface'],local_address:['Локальный адрес','Local address'],gateway:['Шлюз','Gateway'],internet:['Доступ в Интернет','Internet connectivity'],dns:['DNS','DNS'],target_route:['Маршрут к цели','Target route'],policy_routing:['Политика маршрутизации','Policy routing'],kernel_egress:['Выходной интерфейс ядра','Kernel-selected egress'],gateway_consistency:['Согласованность шлюза','Gateway/interface consistency'],source_consistency:['Исходный адрес маршрута','Route/source consistency'],ping:['Ping','Ping'],tcp:['TCP','TCP'],http:['HTTP','HTTP']}; const pair=values[id]||[id,id];return lx(...pair);}
  function verdictCopy(v){const labels={healthy:['Сетевой путь исправен','Network path is healthy'],degraded:['Сеть работает с ограничениями','Network is operating with warnings'],no_default_route:['Нет маршрута по умолчанию','No default route'],interface_down:['Выходной интерфейс недоступен','Default interface is down'],no_local_address:['Нет рабочего IP-адреса','No usable local address'],internet_unreachable:['Внешняя сеть недоступна','Internet connectivity failed'],dns_failure:['DNS не работает','DNS resolution failed'],target_route_failure:['Нет маршрута к цели','No route to target'],egress_interface_failure:['Выбранный ядром интерфейс недоступен','Kernel-selected egress is unavailable'],gateway_interface_mismatch:['Шлюз и выходной интерфейс не согласованы','Route gateway/interface mismatch'],route_source_mismatch:['Исходный адрес не принадлежит интерфейсу','Route source does not belong to selected interface'],target_unreachable:['Цель не отвечает на Ping','Target does not answer Ping'],tcp_failure:['TCP-сервис недоступен','TCP service is unreachable'],http_failure:['HTTP-проверка не прошла','HTTP check failed']};const code=v?.code||'healthy';return lx(...(labels[code]||[code,code]));}
  const actionLabel = id => lx(...({ 'review-warnings':['Проверить предупреждения','Review warnings'], 'inspect-default-route':['Проверить default route','Inspect default route'], 'inspect-policy-rules':['Проверить PBR / policy rules','Inspect PBR / policy rules'], 'inspect-default-interface':['Проверить выходной интерфейс','Inspect default interface'], 'inspect-interface-address':['Проверить адрес интерфейса','Inspect interface address'], 'inspect-kernel-egress':['Проверить kernel-selected egress','Inspect kernel-selected egress'], 'inspect-target-route':['Проверить маршрут к цели','Inspect target route'], 'inspect-source-address':['Проверить исходный адрес','Inspect source address'], 'verify-upstream':['Проверить upstream','Verify upstream'], 'inspect-dns':['Проверить DNS','Inspect DNS'], 'verify-service':['Проверить сервис','Verify service'], 'verify-target-reachability':['Проверить доступность цели','Verify target reachability'], 'review-diagnosis':['Проверить диагностику','Review diagnosis'] }[id]||[id,id]));
  function renderDoctor(data){
    const checks=data.checks||[],probes=data.probes||{},diagnosis=data.diagnosis||{},stages=diagnosis.stages||[],verdict=diagnosis.verdict||{},severity=verdict.severity||'ok',tone=severity==='fail'?'bad':severity==='warn'?'warn':'';
    const verdictHTML=`<div class="doctor-verdict ${escape(severity)}"><div><span class="doctor-verdict-kicker">${escape(lx('ВЕРДИКТ','VERDICT'))}</span><strong>${escape(verdictCopy(verdict))}</strong><small>${escape(verdict.fault_domain?'fault domain: '+verdict.fault_domain:lx('Критических отказов не обнаружено.','No critical failure detected.'))}</small></div>${badge(statusLabel(severity==='fail'?'fail':severity==='warn'?'warn':'ok'),tone)}</div>`;
    const chain=`<div class="doctor-chain">${stages.map(s=>{const t=s.status==='fail'?'bad':s.status==='warn'?'warn':s.status==='ok'?'':'neutral';const duration=Number(s.duration_ms)>0?`<span class="doctor-stage-time">${escape(s.duration_ms)} ms</span>`:'';return `<div class="doctor-stage ${escape(s.status||'')}"><span class="doctor-stage-dot"></span><div class="doctor-stage-copy"><strong>${escape(stageLabel(s.id))}</strong><small>${escape(s.detail||'')}</small></div>${duration}${badge(statusLabel(s.status),t)}</div>`;}).join('')}</div>`;
    const actions=(diagnosis.actions||[]).map(a=>`<div class="nt-result-row"><span class="nt-result-icon">›</span><strong>${escape(actionLabel(a.id))}</strong><span class="mono">${escape(a.fault_domain||'')}</span>${badge(String(a.priority||'').toUpperCase(),a.priority==='high'?'bad':a.priority==='medium'?'warn':'')}<span class="nt-result-meta">${escape(a.detail||'')}</span></div>`).join('');
    const actionBlock=actions?`<div class="doctor-selected-title">${escape(lx('Что проверить дальше','Next checks'))}</div>${actions}`:'';
    const selected=checks.map(k=>{const r=probes[k]||{},loss=k==='ping'&&Number.isFinite(+r.loss_pct)?tr('lossWord')+' '+Number(r.loss_pct).toFixed(1)+'%':'';return `<div class="nt-result-row"><span class="nt-result-icon">${resultIcon(k)}</span><strong>${escape(resultLabel(k))}</strong><span class="mono">${escape(data.target||'')}</span>${badge(r.ok?tr('success'):tr('error'),r.ok?'':'bad')}<span class="nt-result-meta">${escape(resultDetail(k,r))}</span><span class="nt-result-meta" title="${escape(r.error||loss||'')}">›</span></div>`;}).join('');
    return verdictHTML+chain+actionBlock+(checks.length?`<div class="doctor-selected-title">${escape(lx('Выбранные активные проверки','Selected active probes'))}</div>${selected}`:'');
  }
  function profileChanged(){const p=input.doctorProfile;check.ping=p!=='dns';check.dns=true;check.http=p==='web';check.tcp=p!=='dns';}
  async function runDoctor(){
    const target=input.doctorTarget.trim(),checks=Object.keys(check).filter(k=>check[k]);
    if(!checks.length){error('doctor-results',tr('doctorEmpty'));return;}
    doctorBusy=true;put('doctor-results',`<div class="nt-empty">${escape(lx('Выполняется полная диагностика…','Running full diagnosis…'))}</div>`);
    try{const data=await U.request('/doctor',{target,checks:checks.join(','),tcp_port:443,http_port:80});state.doctor=data;
      put('doctor-results',renderDoctor(data));updatedKPIs(data);input.routeTarget=target;input.traceTarget=target;input.probeTarget=target;
      await Promise.all([runRoute(target),loadInterfaces()]);
    } catch(e){error('doctor-results',e);}finally{doctorBusy=false;}void height();
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
  function interfaceUp(i){const flags=String(i.flags||'').toLowerCase(),s=String(i.operstate||'').toLowerCase();return s!=='down'&&s!=='lowerlayerdown'&&s!=='dormant'&&(s==='up'||flags.includes('up'));}
  function interfaceLink(i){let p=[];if(+i.speed_mbps>0)p.push(Number(i.speed_mbps).toLocaleString(locale==='ru'?'ru-RU':'en-US')+' Mbps');if(i.duplex)p.push(i.duplex);p.push('MTU '+Number(i.mtu||0));return p.join(' · ');}
  function renderInterfaces(data){const items=(data?.interfaces||[]).slice().sort((a,b)=>+Boolean(b.default_route)-+Boolean(a.default_route)||String(a.name||'').localeCompare(String(b.name||'')));
    if(!items.length)return `<div class="nt-empty">${escape(lx('Интерфейсы не обнаружены.','Interfaces are unavailable.'))}</div>`;
    const head=`<table class="interfaces-table"><thead><tr><th>${escape(lx('Интерфейс','Interface'))}</th><th>${escape(lx('Состояние','State'))}</th><th>${escape(lx('Адреса','Addresses'))}</th><th>${escape(lx('Линк','Link'))}</th><th>RX / TX</th><th>${escape(lx('Ошибки / потери','Errors / drops'))}</th><th>${escape(lx('Маршрут','Route'))}</th></tr></thead><tbody>`;
    return head+items.map(i=>{
      const up=interfaceUp(i),name=`<strong>${escape(i.name||'—')}</strong>`+(i.alias?`<small>${escape(i.alias)}</small>`:'')+(i.hardware_addr?`<small class="mono">${escape(i.hardware_addr)}</small>`:'');
      const addresses=(i.addresses||[]).map(a=>`<span class="mono interface-address">${escape(a)}</span>`).join('')||'—';
      const traffic=fmtBytes(i.rx_bytes||0)+' / '+fmtBytes(i.tx_bytes||0),errs=Number(i.rx_errors||0)+Number(i.tx_errors||0),drops=Number(i.rx_dropped||0)+Number(i.tx_dropped||0);
      const route=i.default_route?`<span class="interface-default">${escape(lx('По умолчанию','Default'))}</span><small class="mono">${escape(i.default_gateway||'—')} · metric ${escape(i.default_metric||0)}</small>`:'—';
      return `<tr><td class="interface-name">${name}</td><td>${badge(up?lx('В сети','UP'):lx('Не в сети','DOWN'),up?'':'bad')}<small>${escape(i.operstate||'unknown')}</small></td><td>${addresses}</td><td>${escape(interfaceLink(i))}</td><td class="mono">${escape(traffic)}</td><td class="mono">${escape(errs+' / '+drops)}</td><td>${route}</td></tr>`;
    }).join('')+'</tbody></table>';
  }
  async function loadInterfaces(){
    try{const data=await U.request('/interfaces');state.interfaces=data;put('interface-results',renderInterfaces(data));}
    catch(e){stampError('interface-results',e);}finally{stamp('interfaces-updated');}
  }
  function renderTrace(data){const hops=data.hops||[];
    if(!hops.length)return `<div class="nt-empty">${escape(data.error||tr('traceEmpty'))}</div>`;
    const rows=hops.map(h=>{const r=h.rtt_ms||[];return `<tr><td><span class="hop-number">${escape(h.hop)}</span></td><td class="mono">${escape(h.address||'*')}</td><td>${escape(r[0]!==null&&r[0]!==undefined?Number(r[0]).toFixed(1)+' ms':'—')}</td><td>${escape(r[1]!==null&&r[1]!==undefined?Number(r[1]).toFixed(1)+' ms':'—')}</td><td>${escape(r[2]!==null&&r[2]!==undefined?Number(r[2]).toFixed(1)+' ms':'—')}</td><td>${escape(Number(h.loss_pct||0).toFixed(0)+'%')}</td><td>${escape(h.label||'')}</td></tr>`;}).join('');
    return `<table><thead><tr><th>#</th><th>${tr('hop')}</th><th>RTT 1</th><th>RTT 2</th><th>RTT 3</th><th>${tr('lossWord')}</th><th>${tr('node')}</th></tr></thead><tbody>${rows}</tbody></table>`;
  }
  async function runTrace(){put('trace-results',`<div class="nt-empty">${escape(tr('tracing'))}</div>`);
    try{const data=await U.request('/traceroute',{target:input.traceTarget.trim()});state.traceroute=data;put('trace-results',renderTrace(data));}
    catch(e){stampError('trace-results',e);}
  }
  const routePolicyLabel = name => ({active:lx('Активна','Active'),'default-only':lx('Только базовые правила','Default rules only'),unavailable:lx('Недоступно','Unavailable')}[name]||name||'—');
  function routeFacts(data){const dec=data.kernel_decision?.available?data.kernel_decision:data.selected;
    if(!dec)return `<div class="nt-empty">${escape(lx('Маршрут не определён.','Route decision is unavailable.'))}</div>`;
    const rows=[[lx('Адрес цели','Target address'),dec.destination||data.target_address||data.target||'—'],[lx('Семейство','Family'),dec.family||data.family||'—'],[lx('Следующий хоп','Next hop'),dec.gateway||'direct'],[lx('Интерфейс','Interface'),dec.interface||'—'],[lx('Исходный адрес','Source address'),dec.source||'—'],[lx('Таблица','Table'),dec.table||'main'],[lx('Метрика','Metric'),Number(dec.metric||0)],[lx('Тип','Type'),dec.type||'unicast'],['PBR',routePolicyLabel(data.policy_state)]];
    return `<div class="route-title">${escape(lx('Решение ядра','Kernel decision'))}</div>`+rows.map(r=>`<div class="fact-row"><span>${escape(r[0])}</span><strong>${escape(r[1])}</strong></div>`).join('');
  }
  function routeDecision(data,target){const res=data.resolution||{},addresses=res.addresses||[],dec=data.kernel_decision||{},caps=data.capabilities||{},rules=(data.family==='ipv6'?data.rules_v6:data.rules_v4)||[];
    const lookup=caps.route_get&&dec.available?lx('Точный ip route get','Exact ip route get'):dec.available?lx('Резервный longest-prefix lookup','Fallback longest-prefix lookup'):lx('Решение недоступно','Decision unavailable');
    const next=dec.available?((dec.gateway||'direct')+' · '+(dec.interface||'—')):'—';const source=dec.source||lx('Ядро не сообщило source address','Kernel did not report a source address');
    const steps=[[lx('Разрешение цели','Target resolution'),addresses.length?addresses.join(', '):('Target: '+target)],[lx('Поиск маршрута','Route lookup'),lookup],[lx('Политика PBR','Policy routing'),routePolicyLabel(data.policy_state)+' · '+rules.length+' '+lx('правил','rules')],[lx('Выбор next-hop','Next-hop selection'),next],[lx('Выбор source address','Source selection'),source]];
    return `<div class="route-title">${escape(lx('Путь принятия решения','Decision path'))}</div>`+steps.map((s,i)=>`<div class="decision-row"><span class="decision-number">${i+1}</span><div class="decision-copy"><strong>${escape(s[0])}</strong><small>${escape(s[1])}</small></div></div>`).join('');
  }
  function renderCapabilities(data){const c=data.capabilities||{};
    const cap=(name,v)=>`<span class="route-cap ${v?'ok':'off'}"><b>${escape(name)}</b>${escape(v?'OK':'N/A')}</span>`;
    return cap('ip',c.ip_command)+cap('IPv4 tables',c.ipv4_all_tables)+cap('IPv6 tables',c.ipv6_all_tables)+cap('IPv4 rules',c.ipv4_rules)+cap('IPv6 rules',c.ipv6_rules)+cap('route get',c.route_get)+`<span class="route-source">${escape((data.source||'unknown')+' · '+(data.decision_scope||'router-originated'))}</span>`;
  }
  function filteredRoutes(){const data=state.route||{};return [...(data.routes_v4||[]),...(data.routes_v6||[])].filter(r=>{
    if(input.routeFamily && String(r.family||'').toLowerCase()!==input.routeFamily)return false;
    if(input.routeTable && String(r.table||'main')!==input.routeTable)return false;
    const q=input.routeSearch.toLowerCase().trim();if(!q)return true;
    return [r.destination,r.gateway,r.interface,r.source,r.table,r.protocol,r.scope,r.type,r.metric].join(' ').toLowerCase().includes(q);
  });}
  function renderRoutes(){if(!state.route)return;const all=[...(state.route.routes_v4||[]),...(state.route.routes_v6||[])];
    routeTables=unique(all.map(r=>r.table||'main'));const filtered=filteredRoutes();const visible=filtered.slice(0,200);
    putText('route-routes-count',filtered.length===all.length?String(all.length):filtered.length+' / '+all.length);
    if(!visible.length){put('route-routes',`<div class="nt-empty">${escape(lx('Нет маршрутов по текущему фильтру.','No routes match the current filter.'))}</div>`);return;}
    const mode=routeFit==='grid'?'fit-grid':'fit-values';
    put('route-routes',`<table class="route-data-table ${mode}"><thead><tr><th class="c-family">IP</th><th class="c-destination">${escape(lx('Назначение','Destination'))}</th><th class="c-table">${escape(lx('Таблица','Table'))}</th><th class="c-via">Via</th><th class="c-dev">Dev</th><th class="c-src">Src</th><th class="c-metric">Metric</th><th class="c-proto">Proto / Scope</th></tr></thead><tbody>`+visible.map(r=>{
      const destination=r.destination||'—';const rowClass=['default','0.0.0.0/0','::/0'].includes(destination)?' class="is-default"':'';
      return `<tr${rowClass}><td>${escape(r.family||'—')}</td><td class="mono">${escape(destination)}</td><td class="mono">${escape(r.table||'main')}</td><td class="mono">${escape(r.gateway||'direct')}</td><td>${escape(r.interface||'—')}</td><td class="mono">${escape(r.source||'—')}</td><td class="num">${escape(r.metric||0)}</td><td>${escape([r.protocol,r.scope,r.type].filter(Boolean).join(' / ')||'—')}</td></tr>`;
    }).join('')+'</tbody></table>');
  }
  function toggleRouteFit(){routeFit=routeFit==='values'?'grid':'values';renderRoutes();}
  function filteredRules(){const data=state.route||{},q=input.rulesSearch.trim().toLowerCase();const rules=[...(data.rules_v4||[]),...(data.rules_v6||[])];return q?rules.filter(r=>JSON.stringify(r).toLowerCase().includes(q)):rules;}
  function renderRules(){if(!state.route)return;const all=[...(state.route.rules_v4||[]),...(state.route.rules_v6||[])],filtered=filteredRules(),visible=filtered.slice(0,160);
    putText('route-rules-count',filtered.length===all.length?String(all.length):filtered.length+' / '+all.length);
    if(!visible.length){put('route-rules',`<div class="nt-empty">${escape(lx('Нет PBR-правил по текущему фильтру.','No policy rules match the current filter.'))}</div>`);return;}
    put('route-rules',`<table class="policy-table"><thead><tr><th>IP</th><th>Priority</th><th>From</th><th>To</th><th>Mark</th><th>IIF / OIF</th><th>Table / Action</th></tr></thead><tbody>`+
    visible.map(r=>`<tr><td>${escape(r.family||'—')}</td><td class="num">${escape(r.priority)}</td><td class="mono">${escape(r.from||'all')}</td><td class="mono">${escape(r.to||'all')}</td><td class="mono">${escape(r.mark||'—')}</td><td>${escape([r.iif,r.oif].filter(Boolean).join(' / ')||'—')}</td><td class="mono">${escape(r.table||r.action||'—')}</td></tr>`).join('')+'</tbody></table>');
  }
  async function runRoute(target){const dest=target||input.routeTarget.trim();
    try{const data=await U.request('/routes',{target:dest});state.route=data;
      put('route-facts',routeFacts(data));put('route-decision',routeDecision(data,dest));put('route-capabilities',renderCapabilities(data));renderRoutes();renderRules();}
    catch(e){for(const id of ['route-facts','route-decision','route-capabilities','route-routes','route-rules'])stampError(id,e);}
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
  function renderFlowSummary(list){let total=0,tcp=0,udp=0,owned=0,nat=0;
    for(const f of list){total+=Number(f.bytes||0);const p=String(f.protocol||'').toUpperCase();if(p==='TCP')tcp++;if(p==='UDP')udp++;if(f.pid)owned++;if(f._raw?.nat?.detected)nat++;}
    const e=state.flowEnrichment||{};
    return `<span><b>${escape(list.length)}</b>${escape(lx(' потоков',' flows'))}</span>`+`<span>TCP <b>${tcp}</b></span><span>UDP <b>${udp}</b></span><span>NAT <b>${nat}</b></span><span>PID <b>${owned}</b></span>`+
      `<span>${escape(lx('Объём','Volume'))} <b>${escape(fmtBytes(total))}</b></span>`+`<span class="mono">${escape(state.flowSource||'unavailable')}</span>`+
      `<span class="mono">sockets ${escape(e.socket_records||0)}/4096</span>`+`<span class="mono">fd ${escape(e.owner_fds_scanned||0)}/16384</span>`+`<span class="mono">route-get ${escape(e.route_lookups||0)}/${escape(e.route_lookup_budget||32)}</span>`;
  }
  function renderFlowRow(f,pos){const source=f.source+(f.source_port?':'+f.source_port:''),dest=f.destination+(f.destination_port?':'+f.destination_port:'');
    const s=f.state||'ACTIVE',tone=s==='ESTABLISHED'||s==='ACTIVE'?'':'neutral',selected=selectedFlow&&flowKey(selectedFlow)===flowKey(f);
    const owner=f.pid?((f.process||'pid')+' · '+f.pid):'—';
    return `<tr class="flow-row-selectable${selected?' selected':''}" data-flow-pos="${pos}"><td class="mono">${escape(source||'—')}</td><td class="mono">${escape(dest||'—')}</td><td>${escape(f.protocol||'—')}</td><td>${badge(s,tone)}</td><td class="mono">${escape(f.mark||'—')}</td><td class="mono">${escape(owner)}</td><td class="mono">${escape(f.route_interface||'—')}</td><td class="num">${escape(f.packets||0)}</td><td class="num">${escape(fmtBytes(f.bytes||0))}</td><td class="num">${escape(f.timeout_seconds||0)} s</td></tr>`;
  }
  function renderFlowDetail(){
    if(!selectedFlow){put('flow-detail',`<div class="flow-detail-empty">${escape(lx('Выберите строку потока для деталей.','Select a flow row to inspect details.'))}</div>`);return;}
    const raw=selectedFlow._raw||{},original=raw.original||{},reply=raw.reply||{},nat=raw.nat||{},socket=raw.socket||{},policy=raw.policy||{};
    const route=policy.kernel_decision?.available?policy.kernel_decision:raw.route||{},candidates=policy.candidates||[];
    const pairs=[
      [lx('Исходный tuple','Original tuple'),(original.source||'—')+(original.source_port?':'+original.source_port:'')+' → '+(original.destination||'—')+(original.destination_port?':'+original.destination_port:'')],
      [lx('Ответный tuple','Reply tuple'),(reply.source||'—')+(reply.source_port?':'+reply.source_port:'')+' → '+(reply.destination||'—')+(reply.destination_port?':'+reply.destination_port:'')],
      ['NAT',nat.detected?(nat.types||[]).join(' + ')||'detected':'none'],
      [lx('Эффективное назначение','Effective destination'),raw.effective_destination||original.destination||'—'],
      ['Mark',raw.mark||'—'],['PID / Process',socket.available?((socket.process||'—')+' · '+(socket.pid||'—')):(socket.reason||'unattributed')],
      [lx('Socket evidence','Socket evidence'),socket.match||socket.source||'—'],
      [lx('Маршрут','Route'),route.available?((route.interface||'—')+' · table '+(route.table||'main')+(route.gateway?' · via '+route.gateway:'')):(route.reason||route.error||'unavailable')],
      [lx('Policy candidates','Policy candidates'),String(candidates.length)],['Policy source',policy.source||'unavailable']
    ];
    let html='<div class="flow-detail-grid">'+pairs.map(p=>`<div class="flow-detail-item"><span>${escape(p[0])}</span><strong class="mono">${escape(p[1])}</strong></div>`).join('')+'</div>';
    if(candidates.length)html+='<div class="flow-policy-list">'+candidates.slice(0,16).map(r=>`<span class="route-cap ${r.match==='matched-known-fields'?'ok':'off'}">${escape(String(r.priority)+' · '+(r.mark||'no-mark')+' → '+(r.table||r.match||'—'))}</span>`).join('')+'</div>';
    put('flow-detail',html);
  }
  function renderFlows(){const list=flowList(),visible=list.slice(0,flowLimit);
    putText('flow-count',lx('Показано: ','Showing: ')+visible.length+' / '+list.length);
    put('flow-summary',renderFlowSummary(list));
    if(!visible.length){put('flow-results',`<div class="nt-empty">${escape(tr('noFlows'))}</div>`);renderFlowDetail();return;}
    put('flow-results',`<table class="flow-data-table"><thead><tr><th>${tr('source')}</th><th>${tr('destination')}</th><th>${tr('protocol')}</th><th>${tr('flowState')}</th><th>Mark</th><th>PID / Process</th><th>${escape(lx('Маршрут','Route'))}</th><th>${escape(lx('Пакеты','Packets'))}</th><th>${tr('volume')}</th><th>TTL</th></tr></thead><tbody>`+
      visible.map(renderFlowRow).join('')+'</tbody></table>');renderFlowDetail();
  }
  function flowKeydown(event){
    if(event.key!=='Enter' && event.key!==' ') return;
    const row=event.target.closest?.('[data-flow-pos]');if(!row)return;
    event.preventDefault();selectFlowRow(event);
  }
  function selectFlowRow(event){const row=event.target.closest?.('[data-flow-pos]');if(!row)return;
    const pos=Number(row.getAttribute('data-flow-pos'));selectedFlow=flowList()[pos]||null;renderFlows();}
  function moreFlows(){flowLimit+=80;renderFlows();}
  async function refreshFlows(){if(flowBusy)return;flowBusy=true;
    try{const params={limit:256};if(input.flowProtocol)params.protocol=input.flowProtocol;
      if(input.sourceFilter.trim())params.source=input.sourceFilter.trim();if(input.destinationFilter.trim())params.destination=input.destinationFilter.trim();
      const data=await U.request('/flow-explorer',params);
      state.flowGeneratedAt=data.generated_at?new Date(data.generated_at).toLocaleTimeString(locale==='ru'?'ru-RU':'en-US'):'—';
      state.flows=(data.flows||[]).map(normalizeFlow);state.flowSource=data.source||'';state.flowEnrichment=data.enrichment||{};
      flowProtocols=unique(state.flows.map(f=>String(f.protocol||'').toUpperCase()));
      flowStates=unique(state.flows.map(f=>String(f.state||'ACTIVE').toUpperCase()));
      if(input.flowProtocol&&!flowProtocols.includes(input.flowProtocol))input.flowProtocol='';
      if(input.flowState&&!flowStates.includes(input.flowState))input.flowState='';
      renderFlows();putText('kpi-sessions',state.flows.length.toLocaleString(locale==='ru'?'ru-RU':'en-US'));
      historyPoint('sessions',state.flows.length);
    }catch(e){stampError('flow-results',e);}finally{flowBusy=false;stamp('flow-updated');}
  }
  function clearFlowFilters(){Object.assign(input,{flowProtocol:'',flowState:'',flowSort:'recent',flowSearch:'',sourceFilter:'',destinationFilter:''});selectedFlow=null;flowLimit=80;void refreshFlows();}
  function windowChanged(){if(autoTimer)clearInterval(autoTimer);autoTimer=null;
    const n=+input.flowWindow||0;if(n>0)autoTimer=setInterval(()=>{if(!document.hidden)void refreshFlows();},n*1000);void refreshFlows();}

  let needsProbePort = $derived(input.probeKind==='tcp'||input.probeKind==='http');
  function probeKindChanged(){input.probePort=input.probeKind==='tcp'?'443':input.probeKind==='http'?'80':'';}
  function renderProbeHistory(){const list=state.probeHistory.slice().reverse();
    if(!list.length){put('probe-history',`<div class="nt-empty">${escape(lx('История проверок пока пуста.','Probe history is empty.'))}</div>`);return;}
    const rows=list.map(item=>{const data=item.result||{};return `<tr><td class="mono">${escape(item.at)}</td><td>${escape(resultLabel(data.kind||''))}</td><td class="mono">${escape(data.target||'—')}</td><td>${badge(data.ok?tr('success'):tr('error'),data.ok?'':'bad')}</td><td>${escape(resultDetail(data.kind||'',data))}</td></tr>`;}).join('');
    put('probe-history',`<table class="probe-history-table"><thead><tr><th>${tr('time')}</th><th>${escape(lx('Тип','Kind'))}</th><th>${escape(lx('Цель','Target'))}</th><th>${tr('status')}</th><th>${escape(lx('Результат','Result'))}</th></tr></thead><tbody>${rows}</tbody></table>`);
  }
  async function runProbe(){if(probeBusy)return;probeBusy=true;put('probe-summary',`<div class="nt-empty">${escape(lx('Проверка выполняется…','Probe is running…'))}</div>`);
    try{const data=await U.request('/probe',{kind:input.probeKind,target:input.probeTarget.trim(),port:+input.probePort||0});state.activeProbe=data;
      state.probeHistory.push({at:new Date().toLocaleTimeString(locale==='ru'?'ru-RU':'en-US'),result:data});if(state.probeHistory.length>12)state.probeHistory.shift();
      const kind=data.kind||'',detail=resultDetail(kind,data),ok=!!data.ok;
      put('probe-summary',`<div class="probe-summary-card ${ok?'ok':'fail'}"><div><span>${escape(resultIcon(kind))}</span><strong>${escape(resultLabel(kind))}</strong><small class="mono">${escape(data.target||'—')}</small></div>${badge(ok?tr('success'):tr('error'),ok?'':'bad')}<div class="probe-summary-detail">${escape(detail)}</div></div>`);
      putText('probe-output',JSON.stringify(data,null,2));renderProbeHistory();
    }catch(e){stampError('probe-summary',e);putText('probe-output','FAIL: '+e.message);}finally{probeBusy=false;stamp('probe-updated');}
  }
  function clearProbeHistory(){state.probeHistory=[];renderProbeHistory();}
  function saveReport(){const report={generated_at:new Date().toISOString(),module:'network-tools',health:state.health,summary:state.summary,doctor:state.doctor,interfaces:state.interfaces,traceroute:state.traceroute,route:state.route,flow_source:state.flowSource,flow_enrichment:state.flowEnrichment,flows:state.flows,selected_flow:selectedFlow,active_probe:state.activeProbe,probe_history:state.probeHistory};
    const url=URL.createObjectURL(new Blob([JSON.stringify(report,null,2)+'\n'],{type:'application/json'}));
    const a=document.createElement('a');a.href=url;a.download='network-tools-report-'+new Date().toISOString().replace(/[:.]/g,'-')+'.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
  }
  async function refreshAll(){if(refreshBusy)return;refreshBusy=true;
    try{await Promise.all([refreshFlows(),runRoute(input.routeTarget.trim()||'8.8.8.8'),loadInterfaces()]);}
    finally{refreshBusy=false;void height();}
  }
  onMount(()=>{
    mounted=true;
    document.documentElement.lang=locale;document.title=tr('title');
    if(!params.get('view'))try{const s=sessionStorage.getItem('rf-network-tools-view');if(views.includes(s))view=s;}catch(_){}
    probeKindChanged();renderProbeHistory();renderFlowDetail();
    for(const k of ['availability','rtt','loss','sessions'])put('spark-'+k,spark(state.histories[k],k==='loss'));
    putText('trace-mode',lx('Режим: ICMP traceroute','Mode: ICMP traceroute'));
    void (async()=>{try{state.health=await U.request('/health');state.summary=await U.request('/summary');
      putText('kpi-sessions',Number(state.summary?.active_sessions||0).toLocaleString(locale==='ru'?'ru-RU':'en-US'));
      historyPoint('sessions',Number(state.summary?.active_sessions||0));
      await refreshAll();}catch(e){lastError=String(e.message||e);}})();
    const resized=()=>void height();window.addEventListener('resize',resized);
    if(typeof ResizeObserver!=='undefined'){resizeObserver=new ResizeObserver(resized);resizeObserver.observe(document.querySelector('.nt-page')||document.body);}
    return()=>{mounted=false;if(autoTimer)clearInterval(autoTimer);resizeObserver?.disconnect();window.removeEventListener('resize',resized);};
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
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="availability">{tr("availability")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-availability">{@html output["kpi-availability"] ?? "—"}</div><div class="nt-trend">LIVE</div><div id="spark-availability">{@html output["spark-availability"] ?? ""}</div></div><small data-i18n="availabilityHint">{tr("availabilityHint")}</small></article>
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="rtt">{tr("rtt")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-rtt">{@html output["kpi-rtt"] ?? "—"}</div><div class="nt-trend">LIVE</div><div id="spark-rtt">{@html output["spark-rtt"] ?? ""}</div></div><small data-i18n="rttHint">{tr("rttHint")}</small></article>
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="loss">{tr("loss")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-loss">{@html output["kpi-loss"] ?? "—"}</div><div class="nt-trend bad">LIVE</div><div id="spark-loss">{@html output["spark-loss"] ?? ""}</div></div><small data-i18n="lossHint">{tr("lossHint")}</small></article>
      <article class="nt-kpi"><div class="nt-kpi-label" data-i18n="sessions">{tr("sessions")}</div><div class="nt-kpi-main"><div class="nt-kpi-value" id="kpi-sessions">{@html output["kpi-sessions"] ?? "—"}</div><div class="nt-trend">LIVE</div><div id="spark-sessions">{@html output["spark-sessions"] ?? ""}</div></div><small data-i18n="sessionsHint">{tr("sessionsHint")}</small></article>
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
      <div class="nt-result-list" id="doctor-results">{@html output["doctor-results"] ?? "<div class=\"nt-empty\" data-i18n=\"doctorEmpty\">Выберите проверки и нажмите «Запустить».</div>"}</div>
    </article>

    <article class="nt-panel" id="interface-inspector">
      <div class="nt-panel-head">
        <div>
          <strong id="interfaces-title">{lx("Интерфейсы","Interfaces")}</strong>
          <small id="interfaces-hint">{lx("Состояние линка, IP-адреса, счетчики и владение маршрутом по умолчанию.","Link state, IP addresses, counters and default-route ownership.")}</small>
        </div>
        <div class="nt-panel-head-actions"><span class="panel-updated" id="interfaces-updated">{@html output["interfaces-updated"] ?? "—"}</span><button class="nt-button compact" id="interfaces-refresh" onclick={loadInterfaces}>{lx("Обновить","Refresh")}</button></div>
      </div>
      <div class="nt-table-wrap" id="interface-results">{@html output["interface-results"] ?? "\n        <div class=\"nt-empty\">Loading interfaces…</div>\n      "}</div>
    </article>
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-routes" role="tabpanel" aria-labelledby="tab-routes" data-panel="routes" class:active={view==="routes"} aria-hidden={view!=="routes"}>
    <div class="nt-stack">
      <article class="nt-panel">
        <div class="nt-panel-head"><div><strong data-i18n="traceTitle">{tr("traceTitle")}</strong><small data-i18n="traceHint">{tr("traceHint")}</small></div></div>
        <div class="nt-toolbar four"><input id="trace-target" bind:value={input.traceTarget}><div class="nt-static-mode" id="trace-mode">{@html output["trace-mode"] ?? "Mode: ICMP traceroute"}</div><button class="nt-button primary" id="trace-run" onclick={runTrace}><span data-i18n="run">{tr("run")}</span></button><button class="nt-button" id="trace-copy" onclick={copyTrace}><span>▣</span><span data-i18n="copy">{tr("copy")}</span></button></div>
        <div class="nt-table-wrap" id="trace-results">{@html output["trace-results"] ?? "<div class=\"nt-empty\" data-i18n=\"traceEmpty\">Трассировка ещё не запускалась.</div>"}</div>
      </article>

      <article class="nt-panel">
        <div class="nt-panel-head"><div><strong data-i18n="routeTitle">{tr("routeTitle")}</strong><small data-i18n="routeHint">{tr("routeHint")}</small></div><div class="nt-panel-head-actions"><span class="panel-updated" id="route-updated">{@html output["route-updated"] ?? "—"}</span></div></div>
        <div class="nt-toolbar"><input id="route-target" bind:value={input.routeTarget}><span></span><button class="nt-button primary" id="route-run" data-i18n="showRoute" onclick={runRoute}>{tr("showRoute")}</button></div>
        <div class="route-capabilities" id="route-capabilities">{@html output["route-capabilities"] ?? "<span class=\"route-source\">Loading capabilities...</span>"}</div>
        <div class="route-card-grid"><div class="route-facts" id="route-facts">{@html output["route-facts"] ?? "<div class=\"nt-empty\" data-i18n=\"routeEmpty\">Route not selected.</div>"}</div><div class="route-decision" id="route-decision">{@html output["route-decision"] ?? "<div class=\"nt-empty\" data-i18n=\"decisionEmpty\">No decision data.</div>"}</div></div>
        <div class="route-inspector-grid">
          <section class="route-subpanel">
            <div class="route-subpanel-head"><div><strong id="route-inventory-title">{lx("Таблицы маршрутизации","Routing tables")}</strong><small id="route-inventory-hint">{lx("IPv4/IPv6 маршруты из всех доступных kernel tables.","IPv4/IPv6 routes from every kernel table available to this runtime.")}</small></div><div class="route-subpanel-actions"><span class="route-count" id="route-routes-count">{@html output["route-routes-count"] ?? "0"}</span><button class="nt-button compact" id="route-copy-routes" type="button" onclick={copyRoutes}>{lx("Копировать","Copy")}</button></div></div>
            <div class="route-filter-bar">
              <select id="route-family-filter" aria-label="IP family" bind:value={input.routeFamily} onchange={() => renderRoutes()}><option value="">{lx('Все IP','All IP')}</option><option value="ipv4">IPv4</option><option value="ipv6">IPv6</option></select>
              <select id="route-table-filter" aria-label="Routing table" bind:value={input.routeTable} onchange={() => renderRoutes()}><option value="">{lx('Все таблицы','All tables')}</option>{#each routeTables as table (table)}<option value={table}>{table}</option>{/each}</select>
              <input id="route-search" aria-label="Route search" placeholder={lx("Назначение, via, dev, src…","Destination, via, dev, src…")} bind:value={input.routeSearch} oninput={(e) => { input.routeSearch = e.currentTarget.value; renderRoutes(); }}>
              <button class="nt-button compact" id="route-fit-toggle" type="button" onclick={toggleRouteFit}>{routeFit === "values" ? lx("Колонки: по данным","Columns: fit values") : lx("Колонки: по сетке","Columns: fixed grid")}</button>
            </div>
            <div class="nt-table-wrap" id="route-routes">{@html output["route-routes"] ?? "<div class=\"nt-empty\">Loading routes...</div>"}</div>
          </section>
          <section class="route-subpanel">
            <div class="route-subpanel-head"><div><strong id="route-rules-title">{lx("Правила PBR","Policy rules")}</strong><small id="route-rules-hint">{lx("Read-only ip rule snapshot; route get показывает решение для трафика самого роутера.","This is a read-only ip rule snapshot; route get describes router-originated traffic.")}</small></div><div class="route-subpanel-actions"><span class="route-count" id="route-rules-count">{@html output["route-rules-count"] ?? "0"}</span><button class="nt-button compact" id="route-copy-rules" type="button" onclick={copyRules}>{lx("Копировать","Copy")}</button></div></div><div class="route-rules-filter"><input id="route-rules-search" aria-label="Policy rule search" placeholder={lx("Поиск по priority, from, to, mark, table…","Search priority, from, to, mark, table…")} bind:value={input.rulesSearch} oninput={(e) => { input.rulesSearch = e.currentTarget.value; renderRules(); }}></div>
            <div class="nt-table-wrap" id="route-rules">{@html output["route-rules"] ?? "<div class=\"nt-empty\">Loading policy rules...</div>"}</div>
          </section>
        </div>
      </article>
    </div>
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-flows" role="tabpanel" aria-labelledby="tab-flows" data-panel="flows" class:active={view==="flows"} aria-hidden={view!=="flows"}>
    <article class="nt-panel">
      <div class="nt-panel-head"><div><strong data-i18n="flowTitle">{tr("flowTitle")}</strong><small data-i18n="flowHint">{tr("flowHint")}</small></div><div class="nt-panel-head-actions"><span class="panel-updated" id="flow-updated">{@html output["flow-updated"] ?? "—"}</span><button class="nt-button compact" id="flow-clear" type="button" onclick={clearFlowFilters}>{lx("Сбросить фильтры","Reset filters")}</button><button class="nt-button compact" id="flow-copy-selected" type="button" onclick={copySelectedFlow} disabled={!selectedFlow}>{lx("Копировать поток","Copy flow")}</button></div></div>
      <div class="nt-toolbar flow">
        <select id="flow-window" aria-label="Flow refresh mode" bind:value={input.flowWindow} onchange={() => windowChanged()}><option value="manual">{lx('Ручное обновление','Manual refresh')}</option><option value="5">{lx('Авто · 5 с','Auto · 5 s')}</option><option value="15">{lx('Авто · 15 с','Auto · 15 s')}</option></select>
        <select id="flow-protocol" aria-label="Protocol" bind:value={input.flowProtocol} onchange={() => renderFlows()}><option value="">{lx('Все протоколы','All protocols')}</option>{#each flowProtocols as protocol (protocol)}<option value={protocol}>{protocol}</option>{/each}</select>
        <select id="flow-state" aria-label="State" bind:value={input.flowState} onchange={() => renderFlows()}><option value="">{lx('Все состояния','All states')}</option>{#each flowStates as state (state)}<option value={state}>{state}</option>{/each}</select>
        <select id="flow-sort" aria-label="Sort" bind:value={input.flowSort} onchange={() => renderFlows()}><option value="recent">{lx('Сначала новые','Newest first')}</option><option value="bytes">{lx('По объёму','By volume')}</option><option value="packets">{lx('По пакетам','By packets')}</option><option value="destination">{lx('По назначению','By destination')}</option></select>
        <input id="flow-source-filter" aria-label="Exact source IP" placeholder="Source IP (exact)" bind:value={input.sourceFilter}>
        <input id="flow-destination-filter" aria-label="Exact destination IP" placeholder="Destination IP (exact)" bind:value={input.destinationFilter}>
        <input id="flow-search" data-i18n-placeholder="flowSearch" placeholder={lx("Поиск: IP, порт, состояние…","Search: IP, port, state…")} bind:value={input.flowSearch} oninput={(e) => { input.flowSearch = e.currentTarget.value; renderFlows(); }}>
        <button class="nt-button primary" id="flow-refresh" data-i18n="refresh" onclick={refreshFlows} disabled={flowBusy}>{tr("refresh")}</button>
      </div>
      <div class="flow-summary" id="flow-summary">{@html output["flow-summary"] ?? "<span>Waiting for conntrack snapshot...</span>"}</div>
      <div class="flow-detail" id="flow-detail">{@html output["flow-detail"] ?? "<div class=\"flow-detail-empty\">Select a flow row to inspect details.</div>"}</div>
      <div class="nt-table-wrap" id="flow-results" role="grid" tabindex="0" onclick={selectFlowRow} onkeydown={flowKeydown}>{@html output["flow-results"] ?? "<div class=\"nt-empty\" data-i18n=\"flowLoading\">Загрузка conntrack…</div>"}</div>
      <div class="flow-footer"><button id="flow-more" data-i18n="moreFlows" onclick={moreFlows}>{tr("moreFlows")}</button><span id="flow-count">{@html output["flow-count"] ?? "—"}</span></div>
    </article>
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section class="nt-view" id="panel-probes" role="tabpanel" aria-labelledby="tab-probes" data-panel="probes" class:active={view==="probes"} aria-hidden={view!=="probes"}>
    <article class="nt-panel">
      <div class="nt-panel-head"><div><strong data-i18n="probeTitle">{tr("probeTitle")}</strong><small data-i18n="probeHint">{tr("probeHint")}</small></div><div class="nt-panel-head-actions"><span class="panel-updated" id="probe-updated">{@html output["probe-updated"] ?? "—"}</span><button class="nt-button compact" id="probe-clear-history" type="button" onclick={clearProbeHistory}>{lx("Очистить историю","Clear history")}</button></div></div>
      <div class="active-grid">
        <select id="probe-kind" bind:value={input.probeKind} onchange={() => probeKindChanged()}><option value="ping">Ping</option><option value="dns">DNS</option><option value="tcp">TCP</option><option value="http">HTTP</option></select>
        <input id="probe-target" bind:value={input.probeTarget}>
        <input id="probe-port" inputmode="numeric" placeholder="n/a" bind:value={input.probePort} disabled={!needsProbePort}>
        <button class="nt-button primary" id="probe-run" onclick={runProbe} disabled={probeBusy}><span data-i18n="run">{tr("run")}</span>&nbsp;&#9654;</button>
        <button class="nt-button" id="probe-copy" type="button" onclick={copyProbe}>{lx("Копировать","Copy")}</button>
      </div>
      <div class="probe-summary" id="probe-summary">{@html output["probe-summary"] ?? "<div class=\"nt-empty\">Ready to run.</div>"}</div><div class="probe-history-title">Recent probes</div><div class="probe-history" id="probe-history">{@html output["probe-history"] ?? "<div class=\"nt-empty\">Probe history is empty.</div>"}</div><details class="probe-raw" id="probe-raw"><summary id="probe-raw-summary">{lx("Технические данны JSON","Raw JSON details")}</summary><pre class="nt-console" id="probe-output" data-i18n="probeReady">{tr("probeReady")}</pre></details>
    </article>
  </section>
</main>