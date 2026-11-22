<script>
  import { rfUi } from './rf-localization.js';
 import {untrack} from 'svelte';
 let {active=false,onRequest=async()=>{}}=$props();
 let loading=$state(false),error=$state(''),snapshot=$state(null),controller=null,epoch=0;
 async function refresh(){
   if(!active||loading)return;
   const n=++epoch,c=new AbortController();controller=c;loading=true;error='';
   try{
     const [queue,transport,policy]=await Promise.all([
       onRequest('/v2/nfqueue-health',{method:'GET',signal:c.signal}),
       onRequest('/v2/bench-profiles',{method:'GET',signal:c.signal}),
       onRequest('/v2/policy-automation-gate',{method:'GET',signal:c.signal})
     ]);
     if(n===epoch&&!c.signal.aborted&&active)snapshot={queue,transport,policy};
   }catch(e){if(n===epoch&&!c.signal.aborted&&active){snapshot=null;error=String(e?.message||e);}}
   finally{if(n===epoch){controller=null;loading=false;}}
 }
 $effect(()=>{
   if(active)untrack(()=>void refresh());
   else{epoch++;controller?.abort();controller=null;loading=false;}
   return()=>{epoch++;controller?.abort();controller=null;loading=false;};
 });
 let queue=$derived(snapshot?.queue||null),transport=$derived(snapshot?.transport||null),policy=$derived(snapshot?.policy||null);
</script>
<div class="v2-shell" data-svelte-owned="nfqws-runtime-health">
 <div class="v2-hero"><div><h2>{rfUi("Состояние NFQWS","NFQWS status")}</h2><div class="section-kicker">{rfUi("процессы, NFQUEUE и доступные возможности","processes, NFQUEUE, and available capabilities")}</div></div><span id="runtimeHealthState" class="config-badge">{loading?'LOADING':error?'ОШИБКА':queue?.state||'ОЖИДАНИЕ'}</span></div>
 <div class="toolbar"><button id="runtimeHealthRefresh" disabled={loading} onclick={()=>refresh()}>{rfUi("Обновить","Refresh")}</button></div>
 {#if error}<div class="notice error" role="alert">{error}</div>{/if}
 <div class="v2-two"><article class="panel"><div class="panel-head"><div><h2>NFQUEUE</h2><div class="section-kicker">{rfUi("процесс ↔ firewall ↔ ядро","process ↔ firewall ↔ kernel")}</div></div></div>
 <div id="runtimeNFQueueHealth" class="diagnostics">{#if !queue}<div class="analysis-empty">{rfUi("Нет snapshot.","No status data.")}</div>{:else}<div class={'diagnostic '+(queue.state==='HEALTHY'?'good':queue.state==='UNHEALTHY'?'bad':'warn')}><div class="diagnostic-icon">{queue.ok?'✓':'!'}</div><div><div class="diagnostic-title">{queue.state||'UNKNOWN'} · production q{(queue.production_queues||[]).join(',')}</div><div class="diagnostic-detail">{queue.reason||''} · firewall={(queue.firewall_queues||[]).join(',')} · kernel={(queue.kernel_queues||[]).join(',')} · config {String(queue.config_sha256||'').slice(0,16)}… · read-only={queue.read_only?'YES':'NO'}</div></div></div>{/if}</div></article>
 <article class="panel"><div class="panel-head"><div><h2>{rfUi("Транспортные профили","Transport profiles")}</h2><div class="section-kicker">{rfUi("возможности, доступные текущему nfqws2","capabilities available in the current nfqws2")}</div></div></div><div id="runtimeTransportProfiles" class="diagnostics">{#each transport?.profiles||[] as row,i (i)}<div class={'diagnostic '+(row.implemented?'good':'warn')}><div class="diagnostic-icon">{row.implemented?'✓':'i'}</div><div><div class="diagnostic-title">{String(row.id||'').toUpperCase()} · {row.network||''} / {Number(row.remote_port||0)}</div><div class="diagnostic-detail">{row.metric_scope||''} · {row.probe_kind||''} · timeout {Number(row.probe_timeout_seconds||0)}s · IPv4={row.ipv4?'YES':'NO'}</div></div></div>{:else}<div class="analysis-empty">{rfUi("Transport profiles отсутствуют.","No transport profiles found.")}</div>{/each}</div></article></div>
 <article class="panel" style="margin-top:12px"><div class="panel-head"><div><h2>{rfUi("Правила автоматизации","Automation rules")}</h2><div class="section-kicker">{rfUi("автоматические действия требуют актуальной проверки","automatic actions require a current safety check")}</div></div></div><div id="runtimeAutomationPolicy" class="diagnostics">{#if policy}<div class={'diagnostic '+(policy.automation_enabled?'good':'warn')}><div class="diagnostic-icon">{policy.automation_enabled?'✓':'i'}</div><div><div class="diagnostic-title">Automation {policy.automation_enabled?'ENABLED':'GATED'}</div><div class="diagnostic-detail">{policy.reason||policy.automation_reason||'policy gate loaded'} · historical-only evidence cannot create Apply eligibility.</div></div></div>{:else}<div class="analysis-empty">{rfUi("Нет snapshot.","No status data.")}</div>{/if}</div></article>
 <div class="v2-boundary">{rfUi("Эта вкладка только показывает состояние системы и ничего не меняет.","This tab only displays system status and makes no changes.")}</div>
</div>
