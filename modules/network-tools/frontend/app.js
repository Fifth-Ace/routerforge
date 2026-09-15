(function () {
  'use strict';
  var U = window.RFNet;
  var locale = (U.params().get('locale') === 'en') ? 'en' : 'ru';
  var activeView = U.params().get('view') || 'doctor';
  var dict = {
    ru:{title:'Сетевые инструменты',kicker:'СЕТЕВЫЕ ИНСТРУМЕНТЫ',subtitle:'Диагностика, анализ маршрутов, исследование трафика и активные проверки сети.',saveReport:'Сохранить отчёт',tabDoctor:'Сетевой доктор',tabRoutes:'Маршруты',tabFlows:'Потоки',tabProbes:'Активные проверки',availability:'Доступность сети',availabilityHint:'По последним активным проверкам',rtt:'Средняя задержка (RTT)',rttHint:'По последним активным проверкам',loss:'Потери пакетов',lossHint:'По последнему Ping',sessions:'Активные сессии',sessionsHint:'Текущий снимок conntrack',doctorTitle:'Быстрая проверка',doctorHint:'Проверьте доступность узла и основные сервисы.',profileStandard:'Профиль: Стандартный',profileWeb:'Профиль: Web',profileDns:'Профиль: DNS',run:'Запустить',copy:'Копировать',doctorEmpty:'Выберите проверки и нажмите «Запустить».',traceTitle:'Трассировка маршрута',traceHint:'Визуализация пути и диагностика на каждом хопе.',traceEmpty:'Трассировка ещё не запускалась.',routeTitle:'Информация о маршруте',routeHint:'Подробная информация о выбранном маршруте и решении ядра.',showRoute:'Показать маршрут',routeEmpty:'Маршрут ещё не выбран.',decisionEmpty:'Нет данных о решении.',flowTitle:'Анализ трафика',flowHint:'Активные соединения и потоки в реальном времени.',flowNow:'Текущий снимок',allInterfaces:'Все интерфейсы',flowSearch:'Поиск по IP, порту…',refresh:'Обновить',flowLoading:'Загрузка conntrack…',moreFlows:'Показать больше соединений →',probeTitle:'Активные проверки',probeHint:'Ручная ограниченная проверка без изменения конфигурации роутера.',probeReady:'Готово к запуску.',success:'Успешно',error:'Ошибка',noData:'нет данных',time:'Время',lossWord:'Потери',hop:'Хоп',node:'Узел',selectedRoute:'Выбранный маршрут',destination:'Назначение',nextHop:'Следующий хоп',interface:'Интерфейс',tableSource:'Источник таблицы',metric:'Метрика',routeType:'Тип маршрута',policy:'Политика',status:'Статус',decisionPath:'Путь принятия решения',lookupDestination:'Поиск назначения',lookupMain:'Поиск в таблице: main',policyCheck:'Проверка политики (PBR)',nextHopChoice:'Выбор следующего хопа',routeInstalled:'Установка маршрута',notConfirmed:'Отдельная policy-таблица этим read-only источником не подтверждена',routeActive:'Маршрут активен в kernel route table',noRoute:'Подходящий IPv4-маршрут не найден.',noResults:'Нет результатов.',tracing:'Трассировка выполняется…',probing:'Выполняются ограниченные проверки…',noFlows:'Нет соединений по текущему фильтру.',total:'Всего',sessionsWord:'сессий',source:'Источник',volume:'Объём',ports:'Порты',protocol:'Протокол',flowState:'Состояние'},
    en:{title:'Network Tools',kicker:'NETWORK TOOLS',subtitle:'Diagnostics, route analysis, traffic inspection, and active network probes.',saveReport:'Save report',tabDoctor:'Network Doctor',tabRoutes:'Routes',tabFlows:'Flows',tabProbes:'Active Probes',availability:'Network availability',availabilityHint:'Based on latest active checks',rtt:'Average latency (RTT)',rttHint:'Based on latest active checks',loss:'Packet loss',lossHint:'Based on the latest Ping',sessions:'Active sessions',sessionsHint:'Current conntrack snapshot',doctorTitle:'Quick check',doctorHint:'Check host reachability and core services.',profileStandard:'Profile: Standard',profileWeb:'Profile: Web',profileDns:'Profile: DNS',run:'Run',copy:'Copy',doctorEmpty:'Choose checks and press Run.',traceTitle:'Route trace',traceHint:'Visualize the path and inspect every hop.',traceEmpty:'Traceroute has not been run yet.',routeTitle:'Route information',routeHint:'Detailed selected-route and kernel decision information.',showRoute:'Show route',routeEmpty:'No route selected yet.',decisionEmpty:'No decision data.',flowTitle:'Traffic analysis',flowHint:'Active connections and flows in real time.',flowNow:'Current snapshot',allInterfaces:'All interfaces',flowSearch:'Search by IP or port…',refresh:'Refresh',flowLoading:'Loading conntrack…',moreFlows:'Show more connections →',probeTitle:'Active Probes',probeHint:'Run a bounded manual check without changing router configuration.',probeReady:'Ready to run.',success:'Success',error:'Error',noData:'no data',time:'Time',lossWord:'Loss',hop:'Hop',node:'Node',selectedRoute:'Selected route',destination:'Destination',nextHop:'Next hop',interface:'Interface',tableSource:'Table source',metric:'Metric',routeType:'Route type',policy:'Policy',status:'Status',decisionPath:'Decision path',lookupDestination:'Destination lookup',lookupMain:'Lookup in table: main',policyCheck:'Policy check (PBR)',nextHopChoice:'Next-hop selection',routeInstalled:'Route installation',notConfirmed:'A separate policy table is not confirmed by this read-only source',routeActive:'Route is active in the kernel route table',noRoute:'No matching IPv4 route was found.',noResults:'No results.',tracing:'Traceroute is running…',probing:'Running bounded probes…',noFlows:'No connections match the current filter.',total:'Total',sessionsWord:'sessions',source:'Source',volume:'Volume',ports:'Ports',protocol:'Protocol',flowState:'State'}
  };
  var T = dict[locale];
  var state={health:null,summary:null,doctor:null,interfaces:null,traceroute:null,route:null,flows:[],flowSource:'',activeProbe:null,histories:{availability:[],rtt:[],loss:[],sessions:[]}};
  var flowLimit=80;
  function q(id){return document.getElementById(id);} function tr(k){return T[k]||k;} function lx(ru,en){return locale==='ru'?ru:en;}
  function localize(){document.documentElement.lang=locale;document.title=tr('title');document.querySelectorAll('[data-i18n]').forEach(function(n){n.textContent=tr(n.getAttribute('data-i18n'));});document.querySelectorAll('[data-i18n-placeholder]').forEach(function(n){n.setAttribute('placeholder',tr(n.getAttribute('data-i18n-placeholder')));});}
  function setView(view){if(!['doctor','routes','flows','probes'].includes(view))view='doctor';activeView=view;document.querySelectorAll('.nt-tab').forEach(function(b){b.classList.toggle('active',b.getAttribute('data-view')===view);});document.querySelectorAll('.nt-view').forEach(function(p){p.classList.toggle('active',p.getAttribute('data-panel')===view);});U.notifyHeight();}
  function bindTabs(){document.querySelectorAll('.nt-tab').forEach(function(b){b.addEventListener('click',function(){setView(b.getAttribute('data-view'));});});setView(activeView);}
  function pushHistory(name,value){if(!Number.isFinite(Number(value)))return;var list=state.histories[name];list.push(Number(value));if(list.length>24)list.splice(0,list.length-24);}
  function spark(values,bad){if(!values||!values.length)return '<svg class="spark'+(bad?' bad':'')+'" viewBox="0 0 100 32"></svg>';var vals=values.slice();if(vals.length===1)vals=[vals[0],vals[0]];var min=Math.min.apply(null,vals),max=Math.max.apply(null,vals);if(max===min){max+=1;min-=1;}var pts=vals.map(function(v,i){var x=i/(vals.length-1)*100;var y=28-((v-min)/(max-min)*22);return x.toFixed(1)+','+y.toFixed(1);});return '<svg class="spark'+(bad?' bad':'')+'" viewBox="0 0 100 32" preserveAspectRatio="none"><polygon class="area" points="0,32 '+pts.join(' ')+' 100,32"></polygon><polyline class="line" points="'+pts.join(' ')+'"></polyline></svg>';}
  function updateKpis(){q('spark-availability').innerHTML=spark(state.histories.availability,false);q('spark-rtt').innerHTML=spark(state.histories.rtt,false);q('spark-loss').innerHTML=spark(state.histories.loss,true);q('spark-sessions').innerHTML=spark(state.histories.sessions,false);}
  function selectedChecks(){var out=[];if(q('check-ping').checked)out.push('ping');if(q('check-dns').checked)out.push('dns');if(q('check-http').checked)out.push('http');if(q('check-tcp').checked)out.push('tcp');return out;}
  function resultLabel(kind){return {ping:'Ping',dns:'DNS',http:'HTTP',tcp:'TCP'}[kind]||kind;}
  function resultIcon(kind){return {ping:'⌁',dns:'◎',http:'▣',tcp:'◉'}[kind]||'•';}
  function resultDetail(kind,r){if(!r)return tr('noData');if(kind==='ping')return r.rtt_avg_ms?('RTT '+Number(r.rtt_avg_ms).toFixed(1)+' ms'):(tr('time')+' '+r.duration_ms+' ms');if(kind==='dns')return (r.addresses||[]).join(', ')||(tr('time')+' '+r.duration_ms+' ms');if(kind==='http')return r.status_code?('HTTP '+r.status_code+' · '+r.duration_ms+' ms'):(tr('time')+' '+r.duration_ms+' ms');return tr('time')+' '+r.duration_ms+' ms';}
  function stageLabel(id){var labels={default_route:[`\u041c\u0430\u0440\u0448\u0440\u0443\u0442 \u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e`,'Default route'],interface:[`\u0421\u0435\u0442\u0435\u0432\u043e\u0439 \u0438\u043d\u0442\u0435\u0440\u0444\u0435\u0439\u0441`,'Network interface'],local_address:[`\u041b\u043e\u043a\u0430\u043b\u044c\u043d\u044b\u0439 \u0430\u0434\u0440\u0435\u0441`,'Local address'],gateway:[`\u0428\u043b\u044e\u0437`,'Gateway'],internet:[`\u0414\u043e\u0441\u0442\u0443\u043f \u0432 \u0418\u043d\u0442\u0435\u0440\u043d\u0435\u0442`,'Internet connectivity'],dns:['DNS','DNS'],target_route:[`\u041c\u0430\u0440\u0448\u0440\u0443\u0442 \u043a \u0446\u0435\u043b\u0438`,'Target route'],ping:['Ping','Ping'],tcp:['TCP','TCP'],http:['HTTP','HTTP']};var pair=labels[id]||[id,id];return lx(pair[0],pair[1]);}
  function statusLabel(status){return {ok:lx('\u041d\u041e\u0420\u041c\u0410','OK'),warn:lx('\u0412\u041d\u0418\u041c\u0410\u041d\u0418\u0415','WARN'),fail:lx('\u041e\u0428\u0418\u0411\u041a\u0410','FAIL'),skipped:lx('\u041f\u0420\u041e\u041f\u0423\u0429\u0415\u041d\u041e','SKIPPED'),unavailable:'N/A'}[status]||status;}
  function verdictCopy(v){var code=(v&&v.code)||'healthy';var map={healthy:[`\u0421\u0435\u0442\u0435\u0432\u043e\u0439 \u043f\u0443\u0442\u044c \u0438\u0441\u043f\u0440\u0430\u0432\u0435\u043d`,'Network path is healthy'],degraded:[`\u0421\u0435\u0442\u044c \u0440\u0430\u0431\u043e\u0442\u0430\u0435\u0442 \u0441 \u043e\u0433\u0440\u0430\u043d\u0438\u0447\u0435\u043d\u0438\u044f\u043c\u0438`,'Network is operating with warnings'],no_default_route:[`\u041d\u0435\u0442 \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u0430 \u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e`,'No default route'],interface_down:[`\u0412\u044b\u0445\u043e\u0434\u043d\u043e\u0439 \u0438\u043d\u0442\u0435\u0440\u0444\u0435\u0439\u0441 \u043d\u0435 \u0433\u043e\u0442\u043e\u0432`,'Default interface is down'],no_local_address:[`\u041d\u0435\u0442 \u0440\u0430\u0431\u043e\u0447\u0435\u0433\u043e IP-\u0430\u0434\u0440\u0435\u0441\u0430`,'No usable local address'],internet_unreachable:[`\u0412\u043d\u0435\u0448\u043d\u044f\u044f \u0441\u0435\u0442\u044c \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u0430`,'Internet connectivity failed'],dns_failure:[`DNS \u043d\u0435 \u0440\u0430\u0431\u043e\u0442\u0430\u0435\u0442`,'DNS resolution failed'],target_route_failure:[`\u041d\u0435\u0442 \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u0430 \u043a \u0446\u0435\u043b\u0438`,'No route to target'],target_unreachable:[`\u0426\u0435\u043b\u044c \u043d\u0435 \u043e\u0442\u0432\u0435\u0447\u0430\u0435\u0442 \u043d\u0430 Ping`,'Target does not answer Ping'],tcp_failure:[`TCP-\u0441\u0435\u0440\u0432\u0438\u0441 \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u0435\u043d`,'TCP service is unreachable'],http_failure:[`HTTP-\u043f\u0440\u043e\u0432\u0435\u0440\u043a\u0430 \u043d\u0435 \u043f\u0440\u043e\u0448\u043b\u0430`,'HTTP check failed']};var pair=map[code]||[code,code];return lx(pair[0],pair[1]);}
  function stageRow(s){var tone=s.status==='fail'?'bad':s.status==='warn'?'warn':s.status==='ok'?'':'neutral';var duration=Number(s.duration_ms)>0?('<span class="doctor-stage-time">'+U.esc(s.duration_ms)+' ms</span>'):'';return '<div class="doctor-stage '+U.esc(s.status||'')+'"><span class="doctor-stage-dot"></span><div class="doctor-stage-copy"><strong>'+U.esc(stageLabel(s.id))+'</strong><small>'+U.esc(s.detail||'')+'</small></div>'+duration+U.badge(statusLabel(s.status),tone)+'</div>';}
  function renderDoctor(d){var checks=d.checks||[],probes=d.probes||{},diagnosis=d.diagnosis||{},stages=diagnosis.stages||[],verdict=diagnosis.verdict||{},severity=verdict.severity||'ok',tone=severity==='fail'?'bad':severity==='warn'?'warn':'';var verdict='<div class="doctor-verdict '+U.esc(severity)+'"><div><span class="doctor-verdict-kicker">'+U.esc(lx('\u0412\u0415\u0420\u0414\u0418\u041a\u0422','VERDICT'))+'</span><strong>'+U.esc(verdictCopy(verdict))+'</strong><small>'+U.esc(verdict.fault_domain?('fault domain: '+verdict.fault_domain):lx('\u041a\u0440\u0438\u0442\u0438\u0447\u0435\u0441\u043a\u0438\u0445 \u043e\u0442\u043a\u0430\u0437\u043e\u0432 \u043d\u0435 \u043e\u0431\u043d\u0430\u0440\u0443\u0436\u0435\u043d\u043e.','No critical failure detected.'))+'</small></div>'+U.badge(statusLabel(severity==='fail'?'fail':severity==='warn'?'warn':'ok'),tone)+'</div>';var chain='<div class="doctor-chain">'+stages.map(stageRow).join('')+'</div>';var selected=checks.map(function(kind){var r=probes[kind]||{},tone=r.ok?'':'bad',status=r.ok?tr('success'):tr('error'),loss=kind==='ping'&&Number.isFinite(Number(r.loss_pct))?(tr('lossWord')+' '+Number(r.loss_pct).toFixed(1)+'%'):'';return '<div class="nt-result-row"><span class="nt-result-icon">'+resultIcon(kind)+'</span><strong>'+U.esc(resultLabel(kind))+'</strong><span class="mono">'+U.esc(d.target||'')+'</span>'+U.badge(status,tone)+'<span class="nt-result-meta">'+U.esc(resultDetail(kind,r))+'</span><span class="nt-result-meta" title="'+U.esc(r.error||loss||'')+'">\u203a</span></div>';}).join('');var selectedTitle='<div class="doctor-selected-title">'+U.esc(lx('\u0412\u044b\u0431\u0440\u0430\u043d\u043d\u044b\u0435 \u0430\u043a\u0442\u0438\u0432\u043d\u044b\u0435 \u043f\u0440\u043e\u0432\u0435\u0440\u043a\u0438','Selected active probes'))+'</div>';return verdict+chain+(checks.length?selectedTitle+selected:'');}  function updateDoctorKpis(d){var checks=d.checks||[],probes=d.probes||{},ok=0,rtts=[],loss=null;checks.forEach(function(kind){var r=probes[kind]||{};if(r.ok)ok++;if(Number(r.rtt_avg_ms)>0)rtts.push(Number(r.rtt_avg_ms));else if(r.ok&&Number(r.duration_ms)>0&&(kind==='tcp'||kind==='http'))rtts.push(Number(r.duration_ms));if(kind==='ping'&&Number.isFinite(Number(r.loss_pct)))loss=Number(r.loss_pct);});var availability=checks.length?ok/checks.length*100:0;var rtt=rtts.length?rtts.reduce(function(a,b){return a+b;},0)/rtts.length:0;q('kpi-availability').textContent=availability.toFixed(1)+'%';q('kpi-rtt').textContent=rtt>0?rtt.toFixed(0)+' ms':'—';q('kpi-loss').textContent=loss==null?'—':loss.toFixed(1)+'%';pushHistory('availability',availability);if(rtt>0)pushHistory('rtt',rtt);if(loss!=null)pushHistory('loss',loss);updateKpis();}
  async function runDoctor(){var target=q('doctor-target').value.trim(),checks=selectedChecks(),button=q('doctor-run');if(!checks.length){U.error(q('doctor-results'),new Error(tr('doctorEmpty')));return;}button.disabled=true;q('doctor-results').innerHTML='<div class="nt-empty">'+U.esc(lx('\u0412\u044b\u043f\u043e\u043b\u043d\u044f\u0435\u0442\u0441\u044f \u043f\u043e\u043b\u043d\u0430\u044f \u0434\u0438\u0430\u0433\u043d\u043e\u0441\u0442\u0438\u043a\u0430\u2026','Running full diagnosis\u2026'))+'</div>';try{var d=await U.request('/doctor',{target:target,checks:checks.join(','),tcp_port:443,http_port:80});state.doctor=d;q('doctor-results').innerHTML=renderDoctor(d);updateDoctorKpis(d);q('route-target').value=target;q('trace-target').value=target;q('probe-target').value=target;await Promise.all([inspectRoute(target),loadInterfaces()]);}catch(e){U.error(q('doctor-results'),e);}finally{button.disabled=false;}U.notifyHeight();}
  function interfaceUp(i){var flags=String(i.flags||'').toLowerCase(),stateName=String(i.operstate||'').toLowerCase();if(stateName==='down'||stateName==='lowerlayerdown'||stateName==='dormant')return false;return stateName==='up'||flags.indexOf('up')>=0;}
  function interfaceLink(i){var parts=[];if(Number(i.speed_mbps)>0)parts.push(Number(i.speed_mbps).toLocaleString(locale==='ru'?'ru-RU':'en-US')+' Mbps');if(i.duplex)parts.push(String(i.duplex));parts.push('MTU '+Number(i.mtu||0));return parts.join(' \u00b7 ');}
  function renderInterfaces(data){var list=(data&&data.interfaces)||[];list=list.slice().sort(function(a,b){return Number(Boolean(b.default_route))-Number(Boolean(a.default_route))||String(a.name||'').localeCompare(String(b.name||''));});if(!list.length)return '<div class="nt-empty">'+U.esc(lx('\u0418\u043d\u0442\u0435\u0440\u0444\u0435\u0439\u0441\u044b \u043d\u0435 \u043e\u0431\u043d\u0430\u0440\u0443\u0436\u0435\u043d\u044b.','No interfaces found.'))+'</div>';var head='<table class="interfaces-table"><thead><tr><th>'+U.esc(lx('\u0418\u043d\u0442\u0435\u0440\u0444\u0435\u0439\u0441','Interface'))+'</th><th>'+U.esc(lx('\u0421\u043e\u0441\u0442\u043e\u044f\u043d\u0438\u0435','State'))+'</th><th>'+U.esc(lx('\u0410\u0434\u0440\u0435\u0441\u0430','Addresses'))+'</th><th>'+U.esc(lx('\u041b\u0438\u043d\u043a','Link'))+'</th><th>RX / TX</th><th>'+U.esc(lx('\u041e\u0448\u0438\u0431\u043a\u0438 / \u043f\u043e\u0442\u0435\u0440\u0438','Errors / drops'))+'</th><th>'+U.esc(lx('\u041c\u0430\u0440\u0448\u0440\u0443\u0442','Route'))+'</th></tr></thead><tbody>';var rows=list.map(function(i){var up=interfaceUp(i),tone=up?'':'bad',name='<strong>'+U.esc(i.name||'\u2014')+'</strong>'+(i.alias?'<small>'+U.esc(i.alias)+'</small>':'')+(i.hardware_addr?'<small class="mono">'+U.esc(i.hardware_addr)+'</small>':'');var addresses=(i.addresses||[]).map(function(a){return '<span class="mono interface-address">'+U.esc(a)+'</span>';}).join('')||'\u2014';var traffic=U.fmtBytes(i.rx_bytes||0)+' / '+U.fmtBytes(i.tx_bytes||0);var errors=Number(i.rx_errors||0)+Number(i.tx_errors||0),drops=Number(i.rx_dropped||0)+Number(i.tx_dropped||0);var route=i.default_route?('<span class="interface-default">'+U.esc(lx('\u041f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e','Default'))+'</span><small class="mono">'+U.esc(i.default_gateway||'\u2014')+' \u00b7 metric '+U.esc(i.default_metric||0)+'</small>'):'\u2014';return '<tr><td class="interface-name">'+name+'</td><td>'+U.badge(up?lx('\u0412 \u0441\u0435\u0442\u0438','UP'):lx('\u041d\u0435 \u0432 \u0441\u0435\u0442\u0438','DOWN'),tone)+'<small>'+U.esc(i.operstate||'unknown')+'</small></td><td>'+addresses+'</td><td>'+U.esc(interfaceLink(i))+'</td><td class="mono">'+U.esc(traffic)+'</td><td class="mono">'+U.esc(errors+' / '+drops)+'</td><td>'+route+'</td></tr>';}).join('');return head+rows+'</tbody></table>';}
  async function loadInterfaces(){try{var d=await U.request('/interfaces');state.interfaces=d;q('interface-results').innerHTML=renderInterfaces(d);}catch(e){U.error(q('interface-results'),e);}U.notifyHeight();}  function renderTrace(data){var hops=data.hops||[];if(!hops.length)return '<div class="nt-empty">'+U.esc(data.error||tr('traceEmpty'))+'</div>';var rows=hops.map(function(h){var r=h.rtt_ms||[];return '<tr><td><span class="hop-number">'+U.esc(h.hop)+'</span></td><td class="mono">'+U.esc(h.address||'*')+'</td><td>'+U.esc(r[0]!=null?Number(r[0]).toFixed(1)+' ms':'—')+'</td><td>'+U.esc(r[1]!=null?Number(r[1]).toFixed(1)+' ms':'—')+'</td><td>'+U.esc(r[2]!=null?Number(r[2]).toFixed(1)+' ms':'—')+'</td><td>'+U.esc(Number(h.loss_pct||0).toFixed(0)+'%')+'</td><td>'+U.esc(h.label||'')+'</td></tr>';}).join('');return '<table><thead><tr><th>#</th><th>'+tr('hop')+'</th><th>RTT 1</th><th>RTT 2</th><th>RTT 3</th><th>'+tr('lossWord')+'</th><th>'+tr('node')+'</th></tr></thead><tbody>'+rows+'</tbody></table>';}
  async function runTrace(){q('trace-results').innerHTML='<div class="nt-empty">'+U.esc(tr('tracing'))+'</div>';try{var d=await U.request('/traceroute',{target:q('trace-target').value.trim()});state.traceroute=d;q('trace-results').innerHTML=renderTrace(d);}catch(e){U.error(q('trace-results'),e);}U.notifyHeight();}
  function routePolicyLabel(stateName){return {active:lx('\u0410\u043a\u0442\u0438\u0432\u043d\u0430','Active'),'default-only':lx('\u0422\u043e\u043b\u044c\u043a\u043e \u0431\u0430\u0437\u043e\u0432\u044b\u0435 \u043f\u0440\u0430\u0432\u0438\u043b\u0430','Default rules only'),unavailable:lx('\u041d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u043e','Unavailable')}[stateName]||stateName||'\u2014';}
  function routeFacts(data){var decision=data.kernel_decision&&data.kernel_decision.available?data.kernel_decision:data.selected;if(!decision)return '<div class="nt-empty">'+U.esc(lx('\u041c\u0430\u0440\u0448\u0440\u0443\u0442 \u043d\u0435 \u043e\u043f\u0440\u0435\u0434\u0435\u043b\u0435\u043d.','Route decision is unavailable.'))+'</div>';var destination=decision.destination||data.target_address||data.target||'\u2014';var rows=[[lx('\u0410\u0434\u0440\u0435\u0441 \u0446\u0435\u043b\u0438','Target address'),destination],[lx('\u0421\u0435\u043c\u0435\u0439\u0441\u0442\u0432\u043e','Family'),decision.family||data.family||'\u2014'],[lx('\u0421\u043b\u0435\u0434\u0443\u044e\u0449\u0438\u0439 \u0445\u043e\u043f','Next hop'),decision.gateway||'direct'],[lx('\u0418\u043d\u0442\u0435\u0440\u0444\u0435\u0439\u0441','Interface'),decision.interface||'\u2014'],[lx('\u0418\u0441\u0445\u043e\u0434\u043d\u044b\u0439 \u0430\u0434\u0440\u0435\u0441','Source address'),decision.source||'\u2014'],[lx('\u0422\u0430\u0431\u043b\u0438\u0446\u0430','Table'),decision.table||'main'],[lx('\u041c\u0435\u0442\u0440\u0438\u043a\u0430','Metric'),Number(decision.metric||0)],[lx('\u0422\u0438\u043f','Type'),decision.type||'unicast'],['PBR',routePolicyLabel(data.policy_state)]];return '<div class="route-title">'+U.esc(lx('\u0420\u0435\u0448\u0435\u043d\u0438\u0435 \u044f\u0434\u0440\u0430','Kernel decision'))+'</div>'+rows.map(function(row){return '<div class="fact-row"><span>'+U.esc(row[0])+'</span><strong>'+U.esc(row[1])+'</strong></div>';}).join('');}
  function routeDecision(data,target){var resolution=data.resolution||{},addresses=resolution.addresses||[],decision=data.kernel_decision||{},caps=data.capabilities||{},rules=(data.family==='ipv6'?data.rules_v6:data.rules_v4)||[];var lookup=caps.route_get&&decision.available?lx('\u0422\u043e\u0447\u043d\u044b\u0439 ip route get','Exact ip route get'):decision.available?lx('\u0420\u0435\u0437\u0435\u0440\u0432\u043d\u044b\u0439 longest-prefix lookup','Fallback longest-prefix lookup'):lx('\u0420\u0435\u0448\u0435\u043d\u0438\u0435 \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u043e','Decision unavailable');var nextHop=decision.available?((decision.gateway||'direct')+' \u00b7 '+(decision.interface||'\u2014')):'\u2014';var source=decision.source||lx('\u042f\u0434\u0440\u043e \u043d\u0435 \u0441\u043e\u043e\u0431\u0449\u0438\u043b\u043e source address','Kernel did not report a source address');var steps=[[lx('\u0420\u0430\u0437\u0440\u0435\u0448\u0435\u043d\u0438\u0435 \u0446\u0435\u043b\u0438','Target resolution'),addresses.length?addresses.join(', '):('Target: '+target)],[lx('\u041f\u043e\u0438\u0441\u043a \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u0430','Route lookup'),lookup],[lx('\u041f\u043e\u043b\u0438\u0442\u0438\u043a\u0430 PBR','Policy routing'),routePolicyLabel(data.policy_state)+' \u00b7 '+rules.length+' '+lx('\u043f\u0440\u0430\u0432\u0438\u043b','rules')],[lx('\u0412\u044b\u0431\u043e\u0440 next-hop','Next-hop selection'),nextHop],[lx('\u0412\u044b\u0431\u043e\u0440 source address','Source selection'),source]];return '<div class="route-title">'+U.esc(lx('\u041f\u0443\u0442\u044c \u043f\u0440\u0438\u043d\u044f\u0442\u0438\u044f \u0440\u0435\u0448\u0435\u043d\u0438\u044f','Decision path'))+'</div>'+steps.map(function(s,i){return '<div class="decision-row"><span class="decision-number">'+(i+1)+'</span><div class="decision-copy"><strong>'+U.esc(s[0])+'</strong><small>'+U.esc(s[1])+'</small></div></div>';}).join('');}
  function capabilityBadge(label,value){return '<span class="route-cap '+(value?'ok':'off')+'"><b>'+U.esc(label)+'</b>'+U.esc(value?'OK':'N/A')+'</span>';}
  function renderRouteCapabilities(data){var c=data.capabilities||{};return capabilityBadge('ip',c.ip_command)+capabilityBadge('IPv4 tables',c.ipv4_all_tables)+capabilityBadge('IPv6 tables',c.ipv6_all_tables)+capabilityBadge('IPv4 rules',c.ipv4_rules)+capabilityBadge('IPv6 rules',c.ipv6_rules)+capabilityBadge('route get',c.route_get)+'<span class="route-source">'+U.esc((data.source||'unknown')+' \u00b7 '+(data.decision_scope||'router-originated'))+'</span>';}
  function renderRouteInventory(data){var routes=[].concat(data.routes_v4||[],data.routes_v6||[]),visible=routes.slice(0,120);q('route-routes-count').textContent=String(routes.length);if(!visible.length)return '<div class="nt-empty">'+U.esc(lx('\u041c\u0430\u0440\u0448\u0440\u0443\u0442\u044b \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b.','Routes are unavailable.'))+'</div>';return '<table><thead><tr><th>IP</th><th>'+U.esc(lx('\u041d\u0430\u0437\u043d\u0430\u0447\u0435\u043d\u0438\u0435','Destination'))+'</th><th>'+U.esc(lx('\u0422\u0430\u0431\u043b\u0438\u0446\u0430','Table'))+'</th><th>Via</th><th>Dev</th><th>Src</th><th>Metric</th><th>Proto / Scope</th></tr></thead><tbody>'+visible.map(function(r){return '<tr><td>'+U.esc(r.family||'\u2014')+'</td><td class="mono">'+U.esc(r.destination||'\u2014')+'</td><td class="mono">'+U.esc(r.table||'main')+'</td><td class="mono">'+U.esc(r.gateway||'direct')+'</td><td>'+U.esc(r.interface||'\u2014')+'</td><td class="mono">'+U.esc(r.source||'\u2014')+'</td><td>'+U.esc(r.metric||0)+'</td><td>'+U.esc([r.protocol,r.scope,r.type].filter(Boolean).join(' / ')||'\u2014')+'</td></tr>';}).join('')+'</tbody></table>';}
  function renderPolicyRules(data){var rules=[].concat(data.rules_v4||[],data.rules_v6||[]),visible=rules.slice(0,120);q('route-rules-count').textContent=String(rules.length);if(!visible.length)return '<div class="nt-empty">'+U.esc(lx('\u041f\u0440\u0430\u0432\u0438\u043b\u0430 policy routing \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b.','Policy rules are unavailable.'))+'</div>';return '<table><thead><tr><th>IP</th><th>Priority</th><th>From</th><th>To</th><th>Mark</th><th>IIF / OIF</th><th>Table / Action</th></tr></thead><tbody>'+visible.map(function(r){return '<tr><td>'+U.esc(r.family||'\u2014')+'</td><td>'+U.esc(r.priority)+'</td><td class="mono">'+U.esc(r.from||'all')+'</td><td class="mono">'+U.esc(r.to||'all')+'</td><td class="mono">'+U.esc(r.mark||'\u2014')+'</td><td>'+U.esc([r.iif,r.oif].filter(Boolean).join(' / ')||'\u2014')+'</td><td class="mono">'+U.esc(r.table||r.action||'\u2014')+'</td></tr>';}).join('')+'</tbody></table>';}
  function localizeRouteInspector(){q('route-inventory-title').textContent=lx('\u0422\u0430\u0431\u043b\u0438\u0446\u044b \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u0438\u0437\u0430\u0446\u0438\u0438','Routing tables');q('route-inventory-hint').textContent=lx('IPv4/IPv6 \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u044b \u0438\u0437 \u0432\u0441\u0435\u0445 \u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0445 kernel tables.','IPv4/IPv6 routes from every kernel table available to this runtime.');q('route-rules-title').textContent=lx('\u041f\u0440\u0430\u0432\u0438\u043b\u0430 PBR','Policy rules');q('route-rules-hint').textContent=lx('Read-only ip rule snapshot; route get \u043f\u043e\u043a\u0430\u0437\u044b\u0432\u0430\u0435\u0442 \u0440\u0435\u0448\u0435\u043d\u0438\u0435 \u0434\u043b\u044f \u0442\u0440\u0430\u0444\u0438\u043a\u0430 \u0441\u0430\u043c\u043e\u0433\u043e \u0440\u043e\u0443\u0442\u0435\u0440\u0430.','Read-only ip rule snapshot; route get describes router-originated traffic.');}
  async function inspectRoute(target){target=target||q('route-target').value.trim();localizeRouteInspector();try{var d=await U.request('/routes',{target:target});state.route=d;q('route-facts').innerHTML=routeFacts(d);q('route-decision').innerHTML=routeDecision(d,target);q('route-capabilities').innerHTML=renderRouteCapabilities(d);q('route-routes').innerHTML=renderRouteInventory(d);q('route-rules').innerHTML=renderPolicyRules(d);}catch(e){U.error(q('route-facts'),e);U.error(q('route-decision'),e);U.error(q('route-routes'),e);U.error(q('route-rules'),e);}U.notifyHeight();}  function flowRow(f){var source=(f.source||'')+(f.source_port?':'+f.source_port:''),dest=(f.destination||'')+(f.destination_port?':'+f.destination_port:''),tone=f.state==='ESTABLISHED'||f.state==='ACTIVE'?'':'neutral';return '<tr><td>'+U.esc(f.seen_at||'—')+'</td><td class="mono">'+U.esc(source)+'</td><td class="mono">'+U.esc(dest)+'</td><td>'+U.esc(f.protocol||'—')+'</td><td>'+U.esc((f.source_port||'—')+' → '+(f.destination_port||'—'))+'</td><td>'+U.esc(U.fmtBytes(f.bytes||0))+'</td><td>'+U.badge(f.state||'ACTIVE',tone)+'</td></tr>';}
  function renderFlows(){var search=q('flow-search').value.trim().toLowerCase();var list=state.flows.filter(function(f){return !search||JSON.stringify(f).toLowerCase().indexOf(search)>=0;});var visible=list.slice(0,flowLimit);q('flow-count').textContent=tr('total')+': '+list.length+' '+tr('sessionsWord');if(!visible.length){q('flow-results').innerHTML='<div class="nt-empty">'+U.esc(tr('noFlows'))+'</div>';return;}q('flow-results').innerHTML='<table><thead><tr><th>'+tr('time')+'</th><th>'+tr('source')+'</th><th>'+tr('destination')+'</th><th>'+tr('protocol')+'</th><th>'+tr('ports')+'</th><th>'+tr('volume')+'</th><th>'+tr('flowState')+'</th></tr></thead><tbody>'+visible.map(flowRow).join('')+'</tbody></table>';}
  async function refreshFlows(){try{var d=await U.request('/flows',{limit:512});state.flows=d.flows||[];state.flowSource=d.source||'';renderFlows();q('kpi-sessions').textContent=state.flows.length.toLocaleString(locale==='ru'?'ru-RU':'en-US');pushHistory('sessions',state.flows.length);updateKpis();}catch(e){U.error(q('flow-results'),e);}U.notifyHeight();}
  async function runActiveProbe(){var kind=q('probe-kind').value,target=q('probe-target').value.trim(),port=Number(q('probe-port').value||0);q('probe-output').textContent=tr('run')+' '+kind+'…';try{var d=await U.request('/probe',{kind:kind,target:target,port:port});state.activeProbe=d;q('probe-output').textContent=JSON.stringify(d,null,2);}catch(e){q('probe-output').textContent='FAIL: '+e.message;}U.notifyHeight();}
  function saveReport(){var report={generated_at:new Date().toISOString(),module:'network-tools',health:state.health,summary:state.summary,doctor:state.doctor,interfaces:state.interfaces,traceroute:state.traceroute,route:state.route,flow_source:state.flowSource,flows:state.flows,active_probe:state.activeProbe};var blob=new Blob([JSON.stringify(report,null,2)+'\n'],{type:'application/json'}),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download='network-tools-report-'+new Date().toISOString().replace(/[:.]/g,'-')+'.json';document.body.appendChild(a);a.click();a.remove();setTimeout(function(){URL.revokeObjectURL(url);},1000);}
  function copyText(value){if(navigator.clipboard&&navigator.clipboard.writeText)navigator.clipboard.writeText(value).catch(function(){});}
  // R15 UI polish: data-driven route filters, useful flow controls and safer probe UX.
  var flowAutoTimer=null;
  var flowRefreshBusy=false;
  var routeFitMode='values';

  function r15Option(value,label){return '<option value="'+U.esc(value)+'">'+U.esc(label)+'</option>';}
  function r15SetOptions(id,items,preserve){
    var node=q(id);
    if(!node)return;
    var previous=preserve?node.value:'';
    node.innerHTML=items.map(function(item){return r15Option(item[0],item[1]);}).join('');
    if(preserve&&items.some(function(item){return item[0]===previous;}))node.value=previous;
  }
  function r15Unique(values){
    var seen={};
    return values.filter(function(value){
      value=String(value||'').trim();
      if(!value||seen[value])return false;
      seen[value]=true;
      return true;
    }).sort(function(a,b){return a.localeCompare(b);});
  }
  function r15LocalizeControls(){
    r15SetOptions('route-family-filter',[
      ['',lx('\u0412\u0441\u0435 IP','All IP')],
      ['ipv4','IPv4'],
      ['ipv6','IPv6']
    ],true);
    q('route-search').setAttribute('placeholder',lx('\u041d\u0430\u0437\u043d\u0430\u0447\u0435\u043d\u0438\u0435, via, dev, src\u2026','Destination, via, dev, src\u2026'));
    q('route-fit-toggle').textContent=lx('\u041a\u043e\u043b\u043e\u043d\u043a\u0438: \u043f\u043e \u0434\u0430\u043d\u043d\u044b\u043c','Columns: fit values');

    r15SetOptions('flow-window',[
      ['manual',lx('\u0420\u0443\u0447\u043d\u043e\u0435 \u043e\u0431\u043d\u043e\u0432\u043b\u0435\u043d\u0438\u0435','Manual refresh')],
      ['5',lx('\u0410\u0432\u0442\u043e \u00b7 5 \u0441','Auto \u00b7 5 s')],
      ['15',lx('\u0410\u0432\u0442\u043e \u00b7 15 \u0441','Auto \u00b7 15 s')]
    ],true);
    r15SetOptions('flow-sort',[
      ['recent',lx('\u0421\u043d\u0430\u0447\u0430\u043b\u0430 \u043d\u043e\u0432\u044b\u0435','Newest first')],
      ['bytes',lx('\u041f\u043e \u043e\u0431\u044a\u0451\u043c\u0443','By volume')],
      ['packets',lx('\u041f\u043e \u043f\u0430\u043a\u0435\u0442\u0430\u043c','By packets')],
      ['destination',lx('\u041f\u043e \u043d\u0430\u0437\u043d\u0430\u0447\u0435\u043d\u0438\u044e','By destination')]
    ],true);
    q('flow-search').setAttribute('placeholder',lx('\u041f\u043e\u0438\u0441\u043a: IP, \u043f\u043e\u0440\u0442, \u0441\u043e\u0441\u0442\u043e\u044f\u043d\u0438\u0435\u2026','Search: IP, port, state\u2026'));
    q('probe-copy').textContent=lx('\u041a\u043e\u043f\u0438\u0440\u043e\u0432\u0430\u0442\u044c','Copy');
  }

  function r15RouteTables(routes){
    return r15Unique(routes.map(function(route){return route.table||'main';}));
  }
  function r15SyncRouteFilters(routes){
    var table=q('route-table-filter');
    var previous=table?table.value:'';
    var options=[['',lx('\u0412\u0441\u0435 \u0442\u0430\u0431\u043b\u0438\u0446\u044b','All tables')]];
    r15RouteTables(routes).forEach(function(name){options.push([name,name]);});
    r15SetOptions('route-table-filter',options,false);
    if(previous&&options.some(function(item){return item[0]===previous;}))q('route-table-filter').value=previous;
  }
  function r15RouteMatches(route){
    var family=q('route-family-filter').value;
    var table=q('route-table-filter').value;
    var search=q('route-search').value.trim().toLowerCase();
    if(family&&String(route.family||'').toLowerCase()!==family)return false;
    if(table&&String(route.table||'main')!==table)return false;
    if(search){
      var hay=[
        route.destination,route.gateway,route.interface,route.source,
        route.table,route.protocol,route.scope,route.type,route.metric
      ].join(' ').toLowerCase();
      if(hay.indexOf(search)<0)return false;
    }
    return true;
  }
  renderRouteInventory=function(data){
    var routes=[].concat(data.routes_v4||[],data.routes_v6||[]);
    r15SyncRouteFilters(routes);
    var filtered=routes.filter(r15RouteMatches);
    var visible=filtered.slice(0,200);
    q('route-routes-count').textContent=filtered.length===routes.length?String(routes.length):(filtered.length+' / '+routes.length);
    if(!visible.length)return '<div class="nt-empty">'+U.esc(lx('\u041d\u0435\u0442 \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u043e\u0432 \u043f\u043e \u0442\u0435\u043a\u0443\u0449\u0435\u043c\u0443 \u0444\u0438\u043b\u044c\u0442\u0440\u0443.','No routes match the current filter.'))+'</div>';
    var mode=routeFitMode==='grid'?'fit-grid':'fit-values';
    return '<table class="route-data-table '+mode+'"><thead><tr>'+
      '<th class="c-family">IP</th>'+
      '<th class="c-destination">'+U.esc(lx('\u041d\u0430\u0437\u043d\u0430\u0447\u0435\u043d\u0438\u0435','Destination'))+'</th>'+
      '<th class="c-table">'+U.esc(lx('\u0422\u0430\u0431\u043b\u0438\u0446\u0430','Table'))+'</th>'+
      '<th class="c-via">Via</th><th class="c-dev">Dev</th><th class="c-src">Src</th>'+
      '<th class="c-metric">Metric</th><th class="c-proto">Proto / Scope</th></tr></thead><tbody>'+
      visible.map(function(r){
        var destination=r.destination||'\u2014';
        var rowClass=(destination==='default'||destination==='0.0.0.0/0'||destination==='::/0')?' class="is-default"':'';
        return '<tr'+rowClass+'>'+
          '<td>'+U.esc(r.family||'\u2014')+'</td>'+
          '<td class="mono">'+U.esc(destination)+'</td>'+
          '<td class="mono">'+U.esc(r.table||'main')+'</td>'+
          '<td class="mono">'+U.esc(r.gateway||'direct')+'</td>'+
          '<td>'+U.esc(r.interface||'\u2014')+'</td>'+
          '<td class="mono">'+U.esc(r.source||'\u2014')+'</td>'+
          '<td class="num">'+U.esc(r.metric||0)+'</td>'+
          '<td>'+U.esc([r.protocol,r.scope,r.type].filter(Boolean).join(' / ')||'\u2014')+'</td>'+
        '</tr>';
      }).join('')+'</tbody></table>';
  };

  function r15ToggleRouteFit(){
    routeFitMode=routeFitMode==='values'?'grid':'values';
    q('route-fit-toggle').textContent=routeFitMode==='values'
      ?lx('\u041a\u043e\u043b\u043e\u043d\u043a\u0438: \u043f\u043e \u0434\u0430\u043d\u043d\u044b\u043c','Columns: fit values')
      :lx('\u041a\u043e\u043b\u043e\u043d\u043a\u0438: \u043f\u043e \u0441\u0435\u0442\u043a\u0435','Columns: fixed grid');
    if(state.route)q('route-routes').innerHTML=renderRouteInventory(state.route);
    U.notifyHeight();
  }

  function r15SyncFlowFilters(){
    var protocols=r15Unique(state.flows.map(function(flow){return String(flow.protocol||'').toUpperCase();}));
    var states=r15Unique(state.flows.map(function(flow){return String(flow.state||'ACTIVE').toUpperCase();}));
    var protocolValue=q('flow-protocol').value;
    var stateValue=q('flow-state').value;
    var protocolOptions=[['',lx('\u0412\u0441\u0435 \u043f\u0440\u043e\u0442\u043e\u043a\u043e\u043b\u044b','All protocols')]];
    var stateOptions=[['',lx('\u0412\u0441\u0435 \u0441\u043e\u0441\u0442\u043e\u044f\u043d\u0438\u044f','All states')]];
    protocols.forEach(function(value){protocolOptions.push([value,value]);});
    states.forEach(function(value){stateOptions.push([value,value]);});
    r15SetOptions('flow-protocol',protocolOptions,false);
    r15SetOptions('flow-state',stateOptions,false);
    if(protocolValue&&protocols.indexOf(protocolValue)>=0)q('flow-protocol').value=protocolValue;
    if(stateValue&&states.indexOf(stateValue)>=0)q('flow-state').value=stateValue;
  }
  function r15FlowList(){
    var search=q('flow-search').value.trim().toLowerCase();
    var protocol=q('flow-protocol').value;
    var stateName=q('flow-state').value;
    var sortMode=q('flow-sort').value;
    var list=state.flows.filter(function(flow){
      if(protocol&&String(flow.protocol||'').toUpperCase()!==protocol)return false;
      if(stateName&&String(flow.state||'ACTIVE').toUpperCase()!==stateName)return false;
      if(search&&JSON.stringify(flow).toLowerCase().indexOf(search)<0)return false;
      return true;
    }).slice();
    if(sortMode==='bytes')list.sort(function(a,b){return Number(b.bytes||0)-Number(a.bytes||0);});
    else if(sortMode==='packets')list.sort(function(a,b){return Number(b.packets||0)-Number(a.packets||0);});
    else if(sortMode==='destination')list.sort(function(a,b){return String(a.destination||'').localeCompare(String(b.destination||''));});
    return list;
  }
  function r15FlowSummary(list){
    var bytes=0,tcp=0,udp=0;
    list.forEach(function(flow){
      bytes+=Number(flow.bytes||0);
      var p=String(flow.protocol||'').toUpperCase();
      if(p==='TCP')tcp++;
      if(p==='UDP')udp++;
    });
    var source=state.flowSource||'unavailable';
    q('flow-summary').innerHTML=
      '<span><b>'+U.esc(list.length)+'</b>'+U.esc(lx(' \u043f\u043e\u0442\u043e\u043a\u043e\u0432',' flows'))+'</span>'+
      '<span>TCP <b>'+tcp+'</b></span><span>UDP <b>'+udp+'</b></span>'+
      '<span>'+U.esc(lx('\u041e\u0431\u044a\u0451\u043c','Volume'))+' <b>'+U.esc(U.fmtBytes(bytes))+'</b></span>'+
      '<span class="mono">'+U.esc(source)+'</span>';
  }
  function r15FlowRow(flow){
    var source=(flow.source||'')+(flow.source_port?':'+flow.source_port:'');
    var dest=(flow.destination||'')+(flow.destination_port?':'+flow.destination_port:'');
    var stateName=flow.state||'ACTIVE';
    var tone=stateName==='ESTABLISHED'||stateName==='ACTIVE'?'':'neutral';
    return '<tr>'+
      '<td class="mono">'+U.esc(flow.seen_at||'\u2014')+'</td>'+
      '<td class="mono">'+U.esc(source||'\u2014')+'</td>'+
      '<td class="mono">'+U.esc(dest||'\u2014')+'</td>'+
      '<td>'+U.esc(flow.protocol||'\u2014')+'</td>'+
      '<td>'+U.badge(stateName,tone)+'</td>'+
      '<td class="num">'+U.esc(flow.packets||0)+'</td>'+
      '<td class="num">'+U.esc(U.fmtBytes(flow.bytes||0))+'</td>'+
      '<td class="num">'+U.esc(flow.timeout_seconds||0)+' s</td>'+
    '</tr>';
  }
  renderFlows=function(){
    var list=r15FlowList();
    var visible=list.slice(0,flowLimit);
    q('flow-count').textContent=lx('\u041f\u043e\u043a\u0430\u0437\u0430\u043d\u043e: ','Showing: ')+visible.length+' / '+list.length;
    r15FlowSummary(list);
    if(!visible.length){
      q('flow-results').innerHTML='<div class="nt-empty">'+U.esc(tr('noFlows'))+'</div>';
      return;
    }
    q('flow-results').innerHTML='<table class="flow-data-table"><thead><tr>'+
      '<th>'+tr('time')+'</th><th>'+tr('source')+'</th><th>'+tr('destination')+'</th>'+
      '<th>'+tr('protocol')+'</th><th>'+tr('flowState')+'</th>'+
      '<th>'+U.esc(lx('\u041f\u0430\u043a\u0435\u0442\u044b','Packets'))+'</th>'+
      '<th>'+tr('volume')+'</th><th>TTL</th></tr></thead><tbody>'+
      visible.map(r15FlowRow).join('')+'</tbody></table>';
  };
  refreshFlows=async function(){
    if(flowRefreshBusy)return;
    flowRefreshBusy=true;
    q('flow-refresh').disabled=true;
    try{
      var d=await U.request('/flows',{limit:512});
      state.flows=d.flows||[];
      state.flowSource=d.source||'';
      r15SyncFlowFilters();
      renderFlows();
      q('kpi-sessions').textContent=state.flows.length.toLocaleString(locale==='ru'?'ru-RU':'en-US');
      pushHistory('sessions',state.flows.length);
      updateKpis();
    }catch(e){
      U.error(q('flow-results'),e);
    }finally{
      flowRefreshBusy=false;
      q('flow-refresh').disabled=false;
    }
    U.notifyHeight();
  };
  function r15ConfigureFlowTimer(){
    if(flowAutoTimer){clearInterval(flowAutoTimer);flowAutoTimer=null;}
    var seconds=Number(q('flow-window').value||0);
    if(seconds>0)flowAutoTimer=setInterval(function(){refreshFlows();},seconds*1000);
  }
  function r15ProbeFields(){
    var kind=q('probe-kind').value;
    var port=q('probe-port');
    var needsPort=kind==='tcp'||kind==='http';
    port.disabled=!needsPort;
    if(kind==='tcp')port.value='443';
    else if(kind==='http')port.value='80';
    else port.value='';
    port.setAttribute('placeholder',needsPort?'port':'n/a');
  }
  function bind(){
    q('doctor-profile').onchange=function(){var p=q('doctor-profile').value;q('check-ping').checked=p!=='dns';q('check-dns').checked=true;q('check-http').checked=p==='web';q('check-tcp').checked=p!=='dns';};
    q('doctor-run').onclick=runDoctor;
    q('doctor-target').onkeydown=function(e){if(e.key==='Enter')runDoctor();};
    q('doctor-copy').onclick=function(){copyText(JSON.stringify(state.doctor||{},null,2));};
    q('interfaces-refresh').onclick=loadInterfaces;
    q('trace-run').onclick=runTrace;
    q('trace-copy').onclick=function(){copyText(state.traceroute?(state.traceroute.raw||JSON.stringify(state.traceroute,null,2)):'');};
    q('route-run').onclick=function(){inspectRoute();};
    q('route-family-filter').onchange=function(){if(state.route)q('route-routes').innerHTML=renderRouteInventory(state.route);};
    q('route-table-filter').onchange=function(){if(state.route)q('route-routes').innerHTML=renderRouteInventory(state.route);};
    q('route-search').oninput=function(){if(state.route)q('route-routes').innerHTML=renderRouteInventory(state.route);};
    q('route-fit-toggle').onclick=r15ToggleRouteFit;
    q('flow-refresh').onclick=refreshFlows;
    q('flow-search').oninput=renderFlows;
    q('flow-protocol').onchange=renderFlows;
    q('flow-state').onchange=renderFlows;
    q('flow-sort').onchange=renderFlows;
    q('flow-window').onchange=function(){r15ConfigureFlowTimer();refreshFlows();};
    q('flow-more').onclick=function(){flowLimit+=80;renderFlows();U.notifyHeight();};
    q('probe-kind').onchange=r15ProbeFields;
    q('probe-target').onkeydown=function(e){if(e.key==='Enter')runActiveProbe();};
    q('probe-run').onclick=runActiveProbe;
    q('probe-copy').onclick=function(){copyText(q('probe-output').textContent||'');};
    q('save-report').onclick=saveReport;
    r15LocalizeControls();
    r15ProbeFields();
  }  async function load(){localize();bindTabs();bind();q('interfaces-title').textContent=lx('\u0418\u043d\u0442\u0435\u0440\u0444\u0435\u0439\u0441\u044b','Interfaces');q('interfaces-hint').textContent=lx('\u0421\u043e\u0441\u0442\u043e\u044f\u043d\u0438\u0435 \u043b\u0438\u043d\u043a\u0430, IP-\u0430\u0434\u0440\u0435\u0441\u0430, \u0441\u0447\u0435\u0442\u0447\u0438\u043a\u0438 \u0438 \u0432\u043b\u0430\u0434\u0435\u043d\u0438\u0435 \u043c\u0430\u0440\u0448\u0440\u0443\u0442\u043e\u043c \u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e.','Link state, IP addresses, counters and default-route ownership.');q('interfaces-refresh').textContent=lx('\u041e\u0431\u043d\u043e\u0432\u0438\u0442\u044c','Refresh');try{state.health=await U.request('/health');state.summary=await U.request('/summary');q('kpi-sessions').textContent=Number(state.summary.active_sessions||0).toLocaleString(locale==='ru'?'ru-RU':'en-US');pushHistory('sessions',Number(state.summary.active_sessions||0));updateKpis();await Promise.all([refreshFlows(),inspectRoute('8.8.8.8'),loadInterfaces()]);}catch(e){U.error('.nt-page',e);}U.notifyHeight();}  load();
}());
