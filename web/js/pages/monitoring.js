import { esc, fmtInt, timeOnly, groupBy, fmtAgo } from '../utils.js';
import { historyChart } from '../chart.js';

export function renderMonitoring(s,ui,history,clientData={clients:[]},interfaceData={interfaces:[]},clientDetail={}){
  const tabs=`<div class="subtabs"><button class="subtab ${ui.monitorTab==='traffic'?'active':''}" data-mtab="traffic">Трафик</button><button class="subtab ${ui.monitorTab==='flow'?'active':''}" data-mtab="flow">DNS Flow</button><button class="subtab ${ui.monitorTab==='devices'?'active':''}" data-mtab="devices">Устройства</button><button class="subtab ${ui.monitorTab==='interfaces'?'active':''}" data-mtab="interfaces">Интерфейсы</button><button class="subtab ${ui.monitorTab==='domains'?'active':''}" data-mtab="domains">Домены</button></div>`;
  let body='';
  if(ui.monitorTab==='traffic') body=traffic(history,ui);
  if(ui.monitorTab==='flow') body=flow(s,ui);
  if(ui.monitorTab==='devices') body=ui.selectedClientIP?clientView(clientDetail,ui):devices(clientData.clients||[],ui);
  if(ui.monitorTab==='interfaces') body=interfaces(interfaceData.interfaces||[]);
  if(ui.monitorTab==='domains') body=domains(s);
  return `<h1 class="page-title">Мониторинг</h1>${tabs}${body}`;
}

function traffic(history,ui){return `<div class="toolbar"><div class="toolbar-spacer"></div>${[5,60,180,1440].map(n=>`<button class="btn ${ui.historyMinutes===n?'active':''}" data-minutes="${n}">${n===5?'5 мин':n===60?'1 час':n===180?'3 часа':'24 часа'}</button>`).join('')}</div><section class="panel"><div class="panel-header"><span class="panel-title">DNS traffic</span><div class="legend"><span><i class="legend-dot" style="background:var(--color-accent)"></i>Requests</span><span><i class="legend-dot" style="background:var(--color-warning)"></i>Fallbacks</span><span><i class="legend-dot" style="background:var(--color-error)"></i>Errors</span><span><i class="legend-dot" style="background:var(--color-info)"></i>Timeouts</span></div></div><div class="chart-wrap">${historyChart(history||{})}</div></section>`}

function pauseButton(paused,id){return `<button id="${id}" class="btn ${paused?'active':''}">${paused?'▶ Продолжить':'Ⅱ Пауза'}</button>`}

function flow(s,ui){
  const source=ui.flowPaused?(ui.frozenFlow||[]):(s.flow||[]);
  let f=source.slice().reverse(); const q=(ui.flowSearch||'').toLowerCase();
  if(q)f=f.filter(x=>x.domain.toLowerCase().includes(q)||String(x.client_name||x.client_hostname||x.client_ip||'').toLowerCase().includes(q));
  if(ui.flowProfile!=='all')f=f.filter(x=>x.profile===ui.flowProfile);
  if(ui.fallbackOnly)f=f.filter(x=>x.fallback);
  const profiles=[...new Set((s.upstreams||[]).map(u=>u.profile))];
  const paused=ui.flowPaused?`<span class="live-paused">ПОТОК ЗАФИКСИРОВАН · ${source.length} событий</span>`:`<span class="live-running">LIVE</span>`;
  return `<div class="toolbar"><input class="input" id="flowSearch" placeholder="Домен или устройство..." value="${esc(ui.flowSearch||'')}"><select class="select" id="flowProfile"><option value="all">Все профили</option>${profiles.map(p=>`<option ${ui.flowProfile===p?'selected':''}>${esc(p)}</option>`).join('')}</select><button id="fallbackOnly" class="btn ${ui.fallbackOnly?'active':''}">Fallback only</button><div class="toolbar-spacer"></div>${paused}${pauseButton(ui.flowPaused,'flowPause')}</div><section class="panel table-panel live-table" data-live-flow="global"><table class="data-table"><thead><tr><th style="width:82px">Время</th><th style="width:190px">Устройство</th><th>Домен</th><th style="width:110px">Профиль</th><th style="width:150px">DNS</th><th style="width:70px">Тип</th><th style="width:80px">Fallback</th></tr></thead><tbody>${f.length?f.slice(0,500).map(x=>{const nm=x.client_name||x.client_hostname||x.client_ip||'—'; const sub=x.client_ip?`${x.client_ip}${x.client_access?' · '+x.client_access:''}`:''; return `<tr><td class="mono">${timeOnly(x.time)}</td><td>${x.client_ip?`<button class="link-button" data-client-ip="${esc(x.client_ip)}"><div class="cell-title">${esc(nm)}</div><div class="cell-sub">${esc(sub)}</div></button>`:`<div class="cell-title">${esc(nm)}</div>`}</td><td class="cell-title">${esc(x.domain)}</td><td>${esc(x.profile)}</td><td>${esc(x.upstream)}</td><td><span class="pill">${esc(x.qtype)}</span></td><td>${x.fallback?'<span class="pill warn">да</span>':'—'}</td></tr>`}).join(''):'<tr><td colspan="7" class="empty">Нет подходящих запросов</td></tr>'}</tbody></table></section>`;
}

function devices(items,ui){
  const q=String(ui.clientSearch||'').trim().toLowerCase();
  const filtered=items.filter(c=>!q||`${c.name||''} ${c.hostname||''} ${c.ip||''} ${c.mac||''} ${c.policy||'System'} ${c.access||''} ${c.ssid||''} ${c.ap||''}`.toLowerCase().includes(q));
  // Keep the table spatially stable while counters are changing. Traffic values
  // must never be used as the implicit sort key for a live client list.
  filtered.sort((a,b)=>{
    if(Boolean(a.active)!==Boolean(b.active))return a.active?-1:1;
    const an=String(a.name||a.hostname||a.ip||'').toLocaleLowerCase();
    const bn=String(b.name||b.hostname||b.ip||'').toLocaleLowerCase();
    const byName=an.localeCompare(bn,'ru',{numeric:true,sensitivity:'base'});
    return byName||String(a.ip||'').localeCompare(String(b.ip||''),'en',{numeric:true});
  });
  return `<div class="toolbar device-toolbar"><input class="input" id="clientSearch" placeholder="Устройство, IP, MAC, Policy, интерфейс…" value="${esc(ui.clientSearch||'')}"><span class="panel-meta">${filtered.length} из ${items.length}</span></div><section class="panel table-panel" data-preserve-scroll="devices"><div class="panel-header"><span class="panel-title">DNS по устройствам</span><span class="panel-meta">стабильная сортировка · активные сначала, затем по имени</span></div><table class="data-table"><thead><tr><th>Устройство</th><th style="width:130px">IP / Policy</th><th style="width:230px">Подключение</th><th style="width:80px">Запросы</th><th style="width:86px">Forward</th><th style="width:86px">Cache/local</th><th style="width:80px">Ошибки</th><th style="width:80px">Timeout</th><th style="width:105px">Последний</th></tr></thead><tbody>${filtered.length?filtered.map(c=>`<tr class="click-row" data-client-ip="${esc(c.ip)}"><td><div class="cell-title">${esc(c.name||c.hostname||c.ip)}</div><div class="cell-sub mono">${esc(c.mac||'')}</div></td><td><div class="mono">${esc(c.ip)}</div><div class="cell-sub">${esc(c.policy||'System')}</div></td><td><div>${esc(c.access||c.network||'Unknown')}</div><div class="cell-sub">${esc([c.ap,c.mesh_cid?`mesh ${c.mesh_cid}`:''].filter(Boolean).join(' · '))}</div></td><td class="mono">${fmtInt(c.requests)}</td><td class="mono good-text">${fmtInt(c.forwarded||0)}</td><td class="mono">${fmtInt(c.cache_local||0)}</td><td class="mono ${c.client_errors?'bad-text':''}">${fmtInt(c.client_errors||0)}</td><td class="mono ${c.client_timeouts?'bad-text':''}">${fmtInt(c.client_timeouts||0)}</td><td>${c.requests?fmtAgo(c.last_seen):'<span class="cell-sub">ожидаем DNS</span>'}</td></tr>`).join(''):'<tr><td colspan="9" class="empty">По фильтру устройств ничего не найдено</td></tr>'}</tbody></table></section>`;
}

function routePath(route={}){
  const paths=route.paths||[];
  const head=route.mode==='policy-mark'?`${esc(route.name||'Policy')} · mark 0x${Number(route.mark||0).toString(16)} · table ${fmtInt(route.table||0)}`:'System · default route';
  return `<section class="panel"><div class="panel-header"><span class="panel-title">Маршрут политики</span><span class="panel-meta">${head}</span></div>${paths.length?`<div class="route-paths">${paths.map(p=>`<div class="route-path"><div><b>${esc(p.description||p.keenetic_interface||p.linux_interface)}</b><span>${esc([p.keenetic_interface,p.linux_interface,p.type].filter(Boolean).join(' · '))}</span></div>${p.weight?`<code>weight ${fmtInt(p.weight)}</code>`:''}</div>`).join('')}</div>`:'<div class="empty">Default/nexthop для политики не определён</div>'}</section>`;
}

function outcomePill(x){
  if(x==='FORWARDED')return '<span class="pill good">FORWARDED</span>';
  if(x==='CACHE_LOCAL')return '<span class="pill">CACHE / LOCAL</span>';
  if(x==='ERROR')return '<span class="pill bad">ERROR</span>';
  if(x==='CLIENT_TIMEOUT')return '<span class="pill bad">CLIENT TIMEOUT</span>';
  return `<span class="pill">${esc(x||'—')}</span>`;
}

function clientView(data,ui){
  const c=data?.client;
  if(!c)return `<div class="toolbar"><button class="btn" data-client-back>← Все устройства</button></div><div class="empty">Загружаю данные устройства…</div>`;
  const source=ui.clientPaused?(ui.frozenClientEvents||[]):(data.events||[]);
  const search=(ui.clientFlowSearch||'').toLowerCase();
  let events=source.filter(e=>!search||String(e.domain||'').toLowerCase().includes(search)||String(e.resolver||'').toLowerCase().includes(search)||String(e.rcode||'').toLowerCase().includes(search));
  if(ui.clientOutcome!=='all')events=events.filter(e=>e.outcome===ui.clientOutcome);
  const complete=(c.client_responses||0)+(c.client_timeouts||0);
  const cachePct=complete?((c.cache_local||0)/complete*100):0;
  const paused=ui.clientPaused?`<span class="live-paused">ПОТОК ЗАФИКСИРОВАН · ${source.length} событий</span>`:`<span class="live-running">LIVE</span>`;
  return `<div class="toolbar"><button class="btn" data-client-back>← Все устройства</button><div class="toolbar-spacer"></div>${paused}${pauseButton(ui.clientPaused,'clientPause')}</div>
  <section class="hero-card client-hero"><div class="hero-head"><div><div class="hero-title">${esc(c.name||c.hostname||c.ip)}</div><div class="hero-sub">${esc(c.ip)} · ${esc(c.mac||'—')} · ${esc(c.access||c.network||'Unknown')}</div></div><span class="pill ${c.active?'good':''}">${c.active?'ACTIVE':'OFFLINE'}</span></div><div class="hero-metrics client-metrics"><div class="hero-metric"><b>${fmtInt(c.requests)}</b><span>Requests</span></div><div class="hero-metric"><b>${fmtInt(c.forwarded||0)}</b><span>Forwarded</span></div><div class="hero-metric"><b>${fmtInt(c.cache_local||0)}</b><span>Cache / local</span></div><div class="hero-metric"><b>${cachePct.toFixed(1)}%</b><span>Local share</span></div><div class="hero-metric"><b>${Number(c.avg_client_latency_ms||0).toFixed(1)} ms</b><span>Client avg</span></div><div class="hero-metric"><b>${Number(c.p95_client_latency_ms||0).toFixed(1)} ms</b><span>Client p95</span></div></div></section>
  <div class="two-col client-detail-grid"><section class="panel"><div class="panel-header"><span class="panel-title">DNS состояние клиента</span></div><div class="info-row"><div><div class="info-label">Policy</div><div class="info-help">Базовая политика Keenetic для этого устройства</div></div><div class="info-value">${esc(c.policy||'System')}</div></div><div class="info-row"><div><div class="info-label">Ошибки upstream</div><div class="info-help">SERVFAIL/REFUSED и другие ошибки резолвера</div></div><div class="info-value ${c.errors?'bad-text':''}">${fmtInt(c.errors||0)}</div></div><div class="info-row"><div><div class="info-label">Timeout upstream</div><div class="info-help">Таймауты локального 405xx → DoT/DoH</div></div><div class="info-value ${c.timeouts?'bad-text':''}">${fmtInt(c.timeouts||0)}</div></div><div class="info-row"><div><div class="info-label">Ошибки ответа клиенту</div></div><div class="info-value ${c.client_errors?'bad-text':''}">${fmtInt(c.client_errors||0)}</div></div><div class="info-row"><div><div class="info-label">Нет ответа клиенту</div></div><div class="info-value ${c.client_timeouts?'bad-text':''}">${fmtInt(c.client_timeouts||0)}</div></div><div class="info-row"><div><div class="info-label">Fallback</div></div><div class="info-value">${fmtInt(c.fallbacks||0)}</div></div></section>${routePath(c.route||{})}</div>
  <div class="toolbar client-flow-toolbar"><input class="input" id="clientFlowSearch" placeholder="Фильтр домен / resolver / RCODE…" value="${esc(ui.clientFlowSearch||'')}"><select class="select" id="clientOutcome"><option value="all">Все исходы</option>${['FORWARDED','CACHE_LOCAL','ERROR','CLIENT_TIMEOUT'].map(x=>`<option value="${x}" ${ui.clientOutcome===x?'selected':''}>${x}</option>`).join('')}</select><span class="panel-meta">${events.length} событий в текущем снимке</span></div>
  <section class="panel table-panel live-table" data-live-flow="client"><table class="data-table client-flow-table"><thead><tr><th style="width:82px">Время</th><th>Домен</th><th style="width:120px">Исход</th><th style="width:150px">DNS</th><th style="width:88px">RCODE</th><th style="width:95px">Клиент</th><th style="width:115px">Upstream</th><th style="width:72px">Fallback</th></tr></thead><tbody>${events.length?events.map(e=>`<tr><td class="mono">${timeOnly(e.time)}</td><td><div class="cell-title">${esc(e.domain)}</div><div class="cell-sub">${esc(e.qtype)} · ${esc(e.transport)}</div></td><td>${outcomePill(e.outcome)}</td><td><div>${esc(e.resolver||'—')}</div><div class="cell-sub">${e.resolver_port?':'+fmtInt(e.resolver_port):''}</div></td><td class="mono ${e.rcode&&e.rcode!=='NOERROR'&&e.rcode!=='NXDOMAIN'?'bad-text':''}">${esc(e.rcode||'—')}</td><td class="mono">${e.latency_ms?Number(e.latency_ms).toFixed(1)+' ms':'—'}</td><td><div class="mono">${e.upstream_latency_ms?Number(e.upstream_latency_ms).toFixed(1)+' ms':'—'}</div><div class="cell-sub">${esc(e.upstream_timeout?'TIMEOUT':(e.upstream_rcode||''))}</div></td><td>${e.fallback?'<span class="pill warn">да</span>':'—'}</td></tr>`).join(''):'<tr><td colspan="8" class="empty">Пока нет завершённых DNS-запросов в этом потоке</td></tr>'}</tbody></table></section>`;
}

function interfaces(items){
  return `<section class="panel table-panel"><div class="panel-header"><span class="panel-title">DNS по интерфейсам</span></div><table class="data-table"><thead><tr><th>Подключение</th><th style="width:100px">Устройств</th><th style="width:110px">Запросы</th><th style="width:90px">Ошибки</th><th style="width:90px">Timeout</th><th style="width:90px">Fallback</th><th>Top clients</th></tr></thead><tbody>${items.length?items.map(x=>`<tr><td><div class="cell-title">${esc(x.name)}</div><div class="cell-sub">${esc([x.network,x.ap].filter(Boolean).join(' · '))}</div></td><td class="mono">${fmtInt(x.devices)}</td><td class="mono">${fmtInt(x.requests)}</td><td class="mono">${fmtInt(x.errors)}</td><td class="mono">${fmtInt(x.timeouts)}</td><td class="mono">${fmtInt(x.fallbacks)}</td><td>${(x.top_clients||[]).map(c=>`${esc(c.name)} <span class="cell-sub">${fmtInt(c.count)}</span>`).join(' · ')||'—'}</td></tr>`).join(''):'<tr><td colspan="7" class="empty">Нет данных по интерфейсам</td></tr>'}</tbody></table></section>`;
}
function domains(s){
 const tops=s.top_domains||[], types=groupBy(s.flow||[],'qtype'); const max=tops[0]?.count||1;
 return `<div class="two-col"><section class="panel"><div class="panel-header"><span class="panel-title">Top domains</span></div><ul class="domain-list">${tops.map(d=>`<li class="domain-row"><div class="domain-main"><span>${esc(d.domain)}</span><b>${fmtInt(d.count)}</b></div><div class="progress"><span style="width:${d.count/max*100}%"></span></div></li>`).join('')}</ul></section><section class="panel"><div class="panel-header"><span class="panel-title">Типы запросов в live flow</span></div>${Object.entries(types).sort((a,b)=>b[1].length-a[1].length).map(([k,v])=>`<div class="info-row"><div class="info-label">${esc(k)}</div><div class="info-value" style="color:var(--color-accent)">${fmtInt(v.length)}</div></div>`).join('')}</section></div>`;
}
