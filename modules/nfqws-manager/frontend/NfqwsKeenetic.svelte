<script>
 import {untrack} from 'svelte';
 import NfqwsDPIStatus from './NfqwsDPIStatus.svelte';
 import {keeneticProjection} from './NfqwsKeeneticModel.js';
 let {active=false,onRequest=async()=>{},onDefault=()=>{},onDpiRefresh=()=>{},dpiInfo={}}=$props();
 let snapshot=$state(null),loading=$state(false),error=$state(''),epoch=0,controller=null;
 let view=$derived(keeneticProjection(snapshot));
 async function refresh(){
   if(!active||loading)return;
   const n=++epoch,c=new AbortController();controller=c;loading=true;error='';
   try{const value=await onRequest('/keenetic',{method:'GET',signal:c.signal});
     if(n===epoch&&!c.signal.aborted&&active)snapshot=value;
   }catch(e){if(n===epoch&&!c.signal.aborted&&active){snapshot=null;error=String(e?.message||e);}}
   finally{if(n===epoch){controller=null;loading=false;}}
 }
 function refreshBoth(){void refresh();onDpiRefresh();}
 $effect(()=>{
   if(active){untrack(()=>void refresh());}
   else{epoch++;controller?.abort();controller=null;loading=false;}
   return()=>{epoch++;controller?.abort();controller=null;loading=false;};
 });
</script>
{#snippet diagnostics(rows)}
 <div class="diagnostics">{#each rows as row,i (i)}<div class={'diagnostic '+row.severity}><div class="diagnostic-icon">{row.severity==='good'?'✓':row.severity==='info'?'i':'!'}</div><div><div class="diagnostic-title">{row.title}</div><div class="diagnostic-detail">{row.detail}</div></div></div>{/each}</div>
{/snippet}
<div class="backup-center-shell" data-svelte-owned="nfqws-keenetic">
 <div class="backup-center-hero"><div><div class="backup-center-title-row"><h2 style="margin:0">Keenetic Integration</h2><span class="config-badge active">READ-ONLY</span><span id="keeneticState" class="config-badge" class:active={view.state==='KEENETIC'}>{loading?'LOADING':error?'ERROR':view.state}</span></div><div class="muted small" style="margin-top:5px">Сверяем RouterForge с реальным Keenetic/Entware окружением: ndmc, RCI, default route, интерфейсы и /opt. Эта вкладка ничего не меняет в KeeneticOS.</div></div><div class="toolbar"><button id="refreshKeenetic" disabled={loading} onclick={refreshBoth}>Обновить</button></div></div>
 {#if error}<div class="notice error" role="alert">{error}</div>{/if}
 <div class="backup-center-summary"><div class="backup-center-stat"><span>Platform</span><strong id="keeneticPlatform">{view.platform}</strong></div><div class="backup-center-stat"><span>NDMC</span><strong id="keeneticNdmc">{view.ndmc}</strong></div><div class="backup-center-stat"><span>RCI</span><strong id="keeneticRci">{view.rci}</strong></div><div class="backup-center-stat"><span>Entware</span><strong id="keeneticEntware">{view.entware}</strong></div></div>
 <div class="backup-center-layout"><article class="panel"><div class="panel-head"><div><h2>Роутер и сеть</h2><div id="keeneticModelMeta" class="section-kicker mono">{view.model}</div></div></div><div id="keeneticNetwork">{@render diagnostics(view.network)}</div></article>
 <article class="panel"><div class="panel-head"><div><h2>Подключение NFQWS</h2><div class="section-kicker">ISP_INTERFACE ↔ default IPv4 route</div></div><div class="toolbar"><button id="keeneticUseDefault" disabled={!view.defaultInterface||loading} onclick={()=>onDefault(view.defaultInterface)}>Вставить default в Настройку</button></div></div><div id="keeneticBinding">{@render diagnostics(view.binding)}</div></article></div>
 <div class="backup-center-layout" style="margin-top:14px"><article class="panel"><div class="panel-head"><div><h2>Entware /opt</h2><div class="section-kicker">mount + filesystem capacity</div></div></div><div id="keeneticStorage">{@render diagnostics(view.storage)}</div></article><article class="panel"><div class="panel-head"><div><h2>Сетевая политика Keenetic</h2><div class="section-kicker">GET /rci/show/ip/policy · raw policy data is not exposed</div></div></div><div id="keeneticPolicy">{@render diagnostics(view.policy)}</div></article></div>
 <div class="backup-center-layout" style="margin-top:14px"><article class="panel"><div class="panel-head"><div><h2>DPI Detector <span class="config-badge">ДОПОЛНИТЕЛЬНО</span></h2><div class="section-kicker">проверка установленного компонента</div></div><span id="dpiDetectorState" class="config-badge">{dpiInfo.loading?'LOADING':dpiInfo.error?'ERROR':dpiInfo.snapshot?.available?'READY':'IDLE'}</span></div><NfqwsDPIStatus snapshot={dpiInfo.snapshot} loading={dpiInfo.loading} error={dpiInfo.error} mode="keenetic"/></article></div>
 <div class="backup-center-note">Этот раздел только показывает состояние системы. Изменения здесь не выполняются.</div>
</div>
