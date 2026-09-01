import { getSnapshot, getHistory, getSystem, getClients, getInterfaces, getClient } from './api.js';
import { loadSettings, saveSettings, applySettings } from './state.js';
import { esc, timeOnly } from './utils.js';
import { renderOverview } from './pages/overview.js';
import { renderServers } from './pages/servers.js';
import { renderRouting } from './pages/routing.js';
import { renderMonitoring } from './pages/monitoring.js';
import { renderTools } from './pages/tools.js';
import { renderSettings } from './pages/settings.js';

const app=document.querySelector('#app');
let settings=loadSettings(); applySettings(settings);
let snapshot={upstreams:[],flow:[],errors:[],top_domains:[],fallback_edges:[]};
let historyData={points:[]};
let clientData={clients:[]};
let interfaceData={interfaces:[]};
let clientDetailData={};
let system={};
let timer=null;
let busy=false;
let renderQueued=false;
let deferredRender=null;
let lastInteraction=0;
const ui={ page: pageFromLocation(), search:'', profile:'all', activeOnly:false, serverPort:0, routingProfile:'all', routingMinutes:60, monitorTab:'traffic', historyMinutes:5, historyManual:false, flowSearch:'', flowProfile:'all', fallbackOnly:false, flowPaused:false, frozenFlow:[], clientSearch:'', selectedClientIP:'', clientPaused:false, frozenClientEvents:[], clientFlowSearch:'', clientOutcome:'all', toolTab:'journal', logKind:'all', logSearch:'', logPaused:false, frozenErrors:[], frozenBursts:[] };

function pageFromLocation(){ const p=location.pathname.replace(/^\/|\/$/g,''); return ['servers','routing','monitoring','tools','settings'].includes(p)?p:'overview'; }
function pagePath(page){ return page==='overview'?'/':`/${page}`; }

function shell(){
  return `<header class="app-header"><div class="header-workspace"><div class="brand"><span class="brand-mark">D</span><span>DNS-MONITOR</span><span id="versionBadge" class="version-badge">v0.1.0</span></div><nav class="nav">${nav('overview','Обзор')}${nav('servers','Серверы')}${nav('routing','Маршрутизация')}${nav('monitoring','Мониторинг')}${nav('tools','Инструменты')}${nav('settings','Настройки')}</nav><div class="header-status"><span id="headerLed" class="status-led"></span><span id="headerState">...</span><span id="headerTime">--:--:--</span></div></div></header><main id="pageWorkspace" class="page-workspace"></main>`;
}
function nav(p,t){return `<a href="${pagePath(p)}" data-nav="${p}" class="nav-link">${t}</a>`}
app.innerHTML=shell();
const pageRoot=document.querySelector('#pageWorkspace');

function status(){
  const st=snapshot.capture_error?'ERROR':snapshot.active_down?'DOWN':(snapshot.active_degraded||snapshot.active_quality_bad||snapshot.active_quality_warn)?'DEGRADED':'OK';
  return {st,cls:st==='OK'?'':st==='DEGRADED'?'warn':'error'};
}
function updateHeader(){
  const {st,cls}=status();
  const led=document.querySelector('#headerLed');
  if(led) led.className=`status-led ${cls}`;
  const state=document.querySelector('#headerState'); if(state) state.textContent=st;
  const tm=document.querySelector('#headerTime'); if(tm) tm.textContent=timeOnly(snapshot.server_time||new Date());
  const vb=document.querySelector('#versionBadge'); if(vb) vb.textContent=`v${snapshot.version||'0.1.0'}`;
  document.querySelectorAll('[data-nav]').forEach(a=>a.classList.toggle('active',a.dataset.nav===ui.page));
}
function pageHTML(){ if(ui.page==='servers')return renderServers(snapshot,ui); if(ui.page==='routing')return renderRouting(snapshot,ui); if(ui.page==='monitoring')return renderMonitoring(snapshot,ui,historyData,clientData,interfaceData,clientDetailData); if(ui.page==='tools')return renderTools(snapshot,ui,system); if(ui.page==='settings')return renderSettings(snapshot,ui,settings,system); return renderOverview(snapshot,ui); }
function banners(){ return `${snapshot.discovery_error?`<div class="error-banner">Discovery: ${esc(snapshot.discovery_error)}</div>`:''}${snapshot.capture_error?`<div class="error-banner">Capture: ${esc(snapshot.capture_error)}</div>`:''}${snapshot.client_registry_error?`<div class="error-banner">Clients: ${esc(snapshot.client_registry_error)}</div>`:''}${snapshot.client_capture_error?`<div class="error-banner">Client capture: ${esc(snapshot.client_capture_error)}</div>`:''}`; }

let pendingIdleRender=false;

function isEditor(el=document.activeElement){
  return !!(el&&pageRoot.contains(el)&&(el.matches?.('input,textarea,select')||el.isContentEditable));
}
function captureViewState(){
  const el=document.activeElement;
  const state={scrollX:window.scrollX,scrollY:window.scrollY,focus:null,boxes:[]};
  if(el&&pageRoot.contains(el)&&el.id){
    state.focus={id:el.id};
    try{state.focus.start=el.selectionStart;state.focus.end=el.selectionEnd;state.focus.dir=el.selectionDirection;}catch{}
  }
  pageRoot.querySelectorAll('[data-live-flow],[data-preserve-scroll]').forEach((box,i)=>{
    state.boxes.push({key:box.dataset.liveFlow||box.dataset.preserveScroll||String(i),top:box.scrollTop,left:box.scrollLeft});
  });
  return state;
}
function restoreViewState(state){
  if(!state)return;
  for(const saved of state.boxes||[]){
    const box=pageRoot.querySelector(`[data-live-flow="${CSS.escape(saved.key)}"],[data-preserve-scroll="${CSS.escape(saved.key)}"]`);
    if(box){box.scrollTop=saved.top;box.scrollLeft=saved.left;}
  }
  if(state.focus?.id){
    const el=document.getElementById(state.focus.id);
    if(el){
      el.focus({preventScroll:true});
      if(Number.isInteger(state.focus.start)){try{el.setSelectionRange(state.focus.start,state.focus.end,state.focus.dir||'none');}catch{}}
    }
  }
  window.scrollTo(state.scrollX,state.scrollY);
}
function renderNow(){
  renderQueued=false;
  const view=captureViewState();
  updateHeader();
  pageRoot.innerHTML=banners()+pageHTML();
  restoreViewState(view);
}
function render(){
  if(renderQueued)return;
  renderQueued=true;
  requestAnimationFrame(renderNow);
}
function renderWhenIdle(){
  if(isEditor()){
    pendingIdleRender=true;
    clearTimeout(deferredRender);
    return;
  }
  const since=performance.now()-lastInteraction;
  if(since>=900){ pendingIdleRender=false; render(); return; }
  clearTimeout(deferredRender);
  deferredRender=setTimeout(renderWhenIdle,Math.max(80,950-since));
}

function navigate(page){
  if(ui.page===page)return;
  ui.page=page;
  const path=pagePath(page);
  if(location.pathname!==path) window.history.pushState({},'',path);
  // Navigation is always local and immediate; network refresh happens after paint.
  render();
  setTimeout(()=>refresh({afterNavigation:true}),0);
}
window.addEventListener('popstate',()=>{ui.page=pageFromLocation();render();setTimeout(()=>refresh({afterNavigation:true}),0)});

function markInteraction(){ lastInteraction=performance.now(); }
document.addEventListener('pointerdown',markInteraction,true);
document.addEventListener('keydown',markInteraction,true);
document.addEventListener('wheel',markInteraction,{capture:true,passive:true});
document.addEventListener('touchmove',markInteraction,{capture:true,passive:true});
app.addEventListener('focusout',()=>{if(pendingIdleRender)setTimeout(()=>{if(!isEditor()){pendingIdleRender=false;render();}},0);});

function setPauseButton(id,paused){const b=document.querySelector(id);if(!b)return;b.classList.toggle('active',paused);b.textContent=paused?'▶ Продолжить':'Ⅱ Пауза';}
app.addEventListener('scroll',e=>{
  const box=e.target.closest?.('[data-live-flow]'); if(!box||box.scrollTop<4)return;
  if(box.dataset.liveFlow==='global'&&!ui.flowPaused){ui.frozenFlow=(snapshot.flow||[]).slice();ui.flowPaused=true;setPauseButton('#flowPause',true);}
  if(box.dataset.liveFlow==='client'&&!ui.clientPaused){ui.frozenClientEvents=(clientDetailData.events||[]).slice();ui.clientPaused=true;setPauseButton('#clientPause',true);}
  if(box.dataset.liveFlow==='journal'&&!ui.logPaused){ui.frozenErrors=(snapshot.errors||[]).slice();ui.frozenBursts=(snapshot.error_bursts||[]).slice();ui.logPaused=true;setPauseButton('#journalPause',true);}
},true);

app.addEventListener('click',e=>{
  const navEl=e.target.closest('[data-nav]');
  if(navEl){e.preventDefault();navigate(navEl.dataset.nav);return;}
  const port=e.target.closest('[data-port]'); if(port){ui.serverPort=Number(port.dataset.port);render();return;}
  const rm=e.target.closest('[data-rminutes]'); if(rm){ui.routingMinutes=Number(rm.dataset.rminutes);render();return;}
  const clientEl=e.target.closest('[data-client-ip]'); if(clientEl){ui.page='monitoring';ui.monitorTab='devices';ui.selectedClientIP=clientEl.dataset.clientIp;ui.clientPaused=false;ui.frozenClientEvents=[];render();refreshClientDetail();return;}
  if(e.target.closest('[data-client-back]')){ui.selectedClientIP='';ui.clientPaused=false;ui.frozenClientEvents=[];clientDetailData={};render();refreshClients();return;}
  if(e.target.closest('#flowPause')){if(!ui.flowPaused){ui.frozenFlow=(snapshot.flow||[]).slice();ui.flowPaused=true;}else{ui.flowPaused=false;ui.frozenFlow=[];}render();return;}
  if(e.target.closest('#clientPause')){if(!ui.clientPaused){ui.frozenClientEvents=(clientDetailData.events||[]).slice();ui.clientPaused=true;}else{ui.clientPaused=false;ui.frozenClientEvents=[];refreshClientDetail();}render();return;}
  if(e.target.closest('#journalPause')){if(!ui.logPaused){ui.frozenErrors=(snapshot.errors||[]).slice();ui.frozenBursts=(snapshot.error_bursts||[]).slice();ui.logPaused=true;}else{ui.logPaused=false;ui.frozenErrors=[];ui.frozenBursts=[];}render();return;}
  const mt=e.target.closest('[data-mtab]'); if(mt){ui.monitorTab=mt.dataset.mtab;render();if(ui.monitorTab==='traffic')refreshHistory();if(ui.monitorTab==='devices'){if(ui.selectedClientIP)refreshClientDetail();else refreshClients();}if(ui.monitorTab==='interfaces')refreshInterfaces();return;}
  const min=e.target.closest('[data-minutes]'); if(min){ui.historyMinutes=Number(min.dataset.minutes);ui.historyManual=true;refreshHistory();return;}
  const tt=e.target.closest('[data-ttab]'); if(tt){ui.toolTab=tt.dataset.ttab;render();if(ui.toolTab==='system')refreshSystem();return;}
  const lk=e.target.closest('[data-logkind]'); if(lk){ui.logKind=lk.dataset.logkind;render();return;}
  const theme=e.target.closest('[data-theme-card]'); if(theme){updateSetting('theme',theme.dataset.themeCard);return;}
  if(e.target.closest('#toggleActive')){ui.activeOnly=true;render();return;}
  if(e.target.closest('#toggleAll')){ui.activeOnly=false;render();return;}
  if(e.target.closest('#fallbackOnly')){ui.fallbackOnly=!ui.fallbackOnly;render();return;}
});

let searchTimer=null;
app.addEventListener('input',e=>{
  if(e.target.matches('[data-color-key]')){
    const key=e.target.dataset.colorKey; settings[key]=e.target.value; settings.theme='custom'; saveSettings(settings); applySettings(settings);
    const code=e.target.parentElement.querySelector('code'); if(code)code.textContent=e.target.value; return;
  }
  if(e.target.id==='overviewSearch'){ui.search=e.target.value; clearTimeout(searchTimer); searchTimer=setTimeout(render,80);return;}
  if(e.target.id==='flowSearch'){ui.flowSearch=e.target.value; clearTimeout(searchTimer); searchTimer=setTimeout(render,80);return;}
  if(e.target.id==='clientSearch'){ui.clientSearch=e.target.value; clearTimeout(searchTimer); searchTimer=setTimeout(render,80);return;}
  if(e.target.id==='clientFlowSearch'){ui.clientFlowSearch=e.target.value; clearTimeout(searchTimer); searchTimer=setTimeout(render,80);return;}
  if(e.target.id==='logSearch'){ui.logSearch=e.target.value; clearTimeout(searchTimer); searchTimer=setTimeout(render,80);return;}
});

app.addEventListener('change',e=>{
  if(e.target.id==='overviewProfile'){ui.profile=e.target.value;render();return;}
  if(e.target.id==='routingProfile'){ui.routingProfile=e.target.value;render();return;}
  if(e.target.id==='flowProfile'){ui.flowProfile=e.target.value;render();return;}
  if(e.target.id==='clientOutcome'){ui.clientOutcome=e.target.value;render();return;}
  if(e.target.id==='uiLevel'){updateSetting('uiLevel',e.target.value);return;}
  if(e.target.id==='widthMode'){updateSetting('compact',e.target.value==='compact');return;}
  if(e.target.id==='refreshMs'){updateSetting('refreshMs',Number(e.target.value));schedule();return;}
});

function updateSetting(k,v){ settings[k]=v; saveSettings(settings); applySettings(settings); render(); }
function needsSystem(){ return ui.page==='settings'||(ui.page==='tools'&&ui.toolTab==='system'); }

async function refresh({afterNavigation=false}={}){
  if(busy)return;
  busy=true;
  try{
    const requests=[getSnapshot()];
    if(needsSystem()) requests.push(getSystem().catch(()=>system));
    const result=await Promise.all(requests);
    snapshot=result[0];
    if(result.length>1) system=result[1];
    // History is only useful on the visible traffic tab. The old OR condition
    // fetched and rebuilt it on every Monitoring sub-tab.
    if(ui.page==='monitoring'&&ui.monitorTab==='traffic'){ if(!ui.historyManual){const m=Number(snapshot.uptime_seconds||0)/60; ui.historyMinutes=m>=720?1440:m>=90?180:m>=30?60:5;} await refreshHistory(false); }
    if(ui.page==='monitoring'&&ui.monitorTab==='devices'){ if(ui.selectedClientIP){ if(!ui.clientPaused) clientDetailData=await getClient(ui.selectedClientIP).catch(()=>clientDetailData); } else clientData=await getClients().catch(()=>clientData); }
    if(ui.page==='monitoring'&&ui.monitorTab==='interfaces') interfaceData=await getInterfaces().catch(()=>interfaceData);
    if((ui.page==='monitoring'&&ui.monitorTab==='flow'&&ui.flowPaused)||(ui.page==='monitoring'&&ui.monitorTab==='devices'&&ui.selectedClientIP&&ui.clientPaused)||(ui.page==='tools'&&ui.toolTab==='journal'&&ui.logPaused)){ updateHeader(); } else if(afterNavigation) render(); else renderWhenIdle();
  }catch(e){console.error(e)}finally{busy=false;}
}
async function refreshHistory(doRender=true){ try{historyData=await getHistory(ui.historyMinutes); if(doRender)render();}catch(e){console.error(e)} }
async function refreshSystem(){ try{system=await getSystem();render();}catch(e){console.error(e)} }
async function refreshClients(){ try{clientData=await getClients();render();}catch(e){console.error(e)} }
async function refreshClientDetail(){ if(!ui.selectedClientIP)return; try{clientDetailData=await getClient(ui.selectedClientIP); if(!ui.clientPaused)render();}catch(e){console.error(e)} }
async function refreshInterfaces(){ try{interfaceData=await getInterfaces();render();}catch(e){console.error(e)} }
function schedule(){ if(timer)clearInterval(timer); timer=setInterval(()=>refresh(),Math.max(1000,settings.refreshMs||2000)); }

await refresh();
schedule();
