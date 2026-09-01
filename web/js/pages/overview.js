import { esc, fmtInt, fmtPct, fmtMs, fmtAgo, statusFor, errorCount, quality, qualityClass, windowErrors, latencyClass, groupBy, profileOrder, total } from '../utils.js';

export function renderOverview(s, ui) {
  const ups=(s.upstreams||[]).filter(u=>{
    if(ui.profile!=='all'&&u.profile!==ui.profile)return false;
    if(ui.activeOnly&&!u.active)return false;
    const q=(ui.search||'').toLowerCase(); return !q || `${u.name} ${u.target} ${u.profile}`.toLowerCase().includes(q);
  });
  const groups=groupBy(ups,'profile');
  const profiles=Object.keys(groupBy(s.upstreams||[],'profile')).sort(profileOrder);
  const req=total(s.upstreams||[],'requests');
  const errs=(s.upstreams||[]).reduce((n,u)=>n+errorCount(u)+Number(u.timeouts||0),0);
  const active=(s.upstreams||[]).filter(u=>u.active).length;
  const healthy=(s.upstreams||[]).filter(u=>u.health_status!=='DOWN').length;
  const summary=`<div class="summary">
    <div class="summary-item"><div class="summary-value">${healthy}/${s.upstream_count||0}</div><div class="summary-label">DNS серверы</div><div class="summary-note">${active} активны · DOWN ${s.down||0}</div></div>
    <div class="summary-item"><div class="summary-value">${fmtInt(req)}</div><div class="summary-label">Запросы</div><div class="summary-note">${fmtInt(s.total_responses||0)} ответов</div></div>
    <div class="summary-item"><div class="summary-value">${fmtInt(s.total_fallbacks||0)}</div><div class="summary-label">Fallback</div><div class="summary-note">${req?fmtPct(s.total_fallbacks/req*100):'0%'}</div></div>
    <div class="summary-item"><div class="summary-value">${fmtInt(errs)}</div><div class="summary-label">Ошибки</div><div class="summary-note">${fmtInt(s.total_timeouts||0)} timeout · ${s.active_degraded||0} degraded</div></div>
  </div>`;
  const profileOptions=`<option value="all">Все профили</option>${profiles.map(p=>`<option ${ui.profile===p?'selected':''}>${esc(p)}</option>`).join('')}`;
  const toolbar=`<div class="toolbar"><input class="input" id="overviewSearch" placeholder="Поиск DNS или профиля..." value="${esc(ui.search||'')}"><select class="select" id="overviewProfile">${profileOptions}</select><button class="btn ${ui.activeOnly?'active':''}" id="toggleActive">Активные</button><button class="btn ${!ui.activeOnly?'active':''}" id="toggleAll">Все</button></div>`;
  const tables=Object.keys(groups).sort(profileOrder).map(p=>profileTable(p,groups[p],req)).join('') || '<div class="panel"><div class="empty">Ничего не найдено</div></div>';
  return `<h1 class="page-title">Обзор</h1>${toolbar}${summary}${tables}`;
}

function profileTable(profile,ups,totalReq){
  return `<section class="panel table-panel"><div class="panel-header"><span class="panel-title">${esc(profile)} · ${ups.length} DNS</span></div><table class="data-table"><thead><tr>
    <th class="col-dns">DNS</th><th class="col-status">Статус</th><th class="col-type">Тип</th><th class="col-traffic">Трафик</th><th class="col-latency">Latency 5m</th><th class="col-fallback">Fallback</th><th class="col-errors">Качество 5m</th><th class="expert-only col-port">Port</th><th class="expert-only col-interface">Interface</th>
  </tr></thead><tbody>${ups.map(u=>row(u,totalReq)).join('')}</tbody></table></section>`;
}
function row(u,totalReq){
  const st=statusFor(u), share=totalReq?u.requests/totalReq*100:0, w=u.stats_5m||{}, q=quality(u), qcls=qualityClass(w), err=windowErrors(w), p95=Number(w.p95_latency_ms||0), avg=Number(w.avg_latency_ms||0);
  return `<tr><td><div class="cell-title">${esc(u.name)}</div><div class="cell-sub">${esc(u.target||u.sni||'—')}<span class="compact-expert-meta"> · :${u.port}${u.interface?` · ${esc(u.interface)}`:''}</span></div></td>
    <td><div class="status-line ${st.cls}">${st.label}</div><div class="cell-sub">${u.active?'используется сейчас':fmtAgo(u.last_request)}</div></td>
    <td><span class="pill accent">${esc(u.protocol)}</span></td>
    <td><span class="metric good">${fmtInt(u.requests)} req</span><span class="metric" style="float:right;color:var(--color-accent)">${fmtPct(share)}</span><div class="progress"><span style="width:${Math.min(100,share)}%"></span></div></td>
    <td>${p95?`<span class="latency-badge ${latencyClass(p95)}">p95 ${fmtMs(p95)}</span><div class="cell-sub">avg ${fmtMs(avg)}</div>`:'<span class="metric">—</span>'}</td>
    <td><span class="metric ${Number(w.fallbacks||0)?'warn':''}">${fmtInt(w.fallbacks||0)}</span><div class="cell-sub">${fmtPct(Number(w.fallback_pct||0))}</div></td>
    <td><span class="metric ${qcls}">${q.toFixed(1)}%</span><div class="cell-sub">${fmtInt(err)} err · ${fmtInt(w.timeouts||0)} timeout</div></td>
    <td class="expert-only col-port mono">${u.port}</td><td class="expert-only col-interface cell-sub">${esc(u.interface||'—')}</td></tr>`;
}
