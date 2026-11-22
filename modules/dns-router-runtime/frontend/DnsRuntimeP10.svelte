<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {createP10Flow} from './DnsRuntimeP10Logic.js';
  import {p10Runtime} from './DnsRuntimeCommands.js';
  import {shell,feedback} from './DnsRuntimeStore.svelte.js';
  let busy=$state(false);
  let known=$state(false);
  let active=$state(false);
  let liteMode=$state(false);
  let failed=$state('');
  let stopped=false;
  let generation=0;
  const bridge=()=>p10Runtime;
  const emit=(message,error=false)=>{
    bridge()?.report(message,error);
    if(error)failed=message;
  };
  async function request(path,options={}){
    const response=await fetch('../'+path,{cache:'no-store',credentials:'same-origin',...options});
    let body;
    try{body=await response.json();}catch{throw Error(rfUi('Некорректный ответ ',"Invalid response")+path);}
    if(!response.ok)throw Error('HTTP '+response.status+': '+(body?.error||'unknown error'));
    return body;
  }
  const flow=createP10Flow({
    request,
    readDraft:()=>bridge()?.readDraft(),
    commit:(action,payload)=>bridge()?.commit(action,payload),
    getSimple:()=>bridge()?.simple(),
    stagePending:()=>bridge()?.stagePending(),
    confirmAction:message=>window.confirm(message),
    report:feedback,
    signal:emit
  });
  function nativeBadge(status){
    shell.liteKnown=status.active||status.ownership.state==='disabled';
  }
  async function refresh(){
    const bridgeNow=bridge();
    if(stopped||!bridgeNow)return;
    const seq=++generation;
    known=false;
    try{
      if(await bridgeNow.isLite()){
        liteMode=true;
        const lite=await bridgeNow.refreshLite();
        if(!stopped&&seq===generation){known=true;active=Boolean(lite?.active);}
      }else{
        liteMode=false;
        const state=await flow.status();
        if(stopped||seq!==generation)return;
        nativeBadge(state);
        active=state.active;
        known=state.active||state.ownership.state==='disabled';
        const message=state.active?'DNS ACTIVE · '+(state.ownership.owned_zones||[]).length+rfUi(' зон · ',"zones ·")+(state.shadow.lifecycle?.listen||''):
          state.ownership.state==='disabled'?rfUi('DNS Router не подключён к DNS Keenetic',"DNS Router is not connected to Keenetic DNS"):
          rfUi('Состояние ',"Status")+state.ownership.state+' · проверь ownership и восстановление';
        emit(message,!known);
      }
    }catch(error){if(!stopped&&seq===generation){known=false;emit('Статус DNS недоступен: '+String(error?.message||error),true);}}
  }
  async function operation(kind){
    if(busy||!known||stopped)return;
    busy=true;failed='';
    try{
      const b=bridge();
      if(!b)throw Error('DNS_RUNTIME_P10_BRIDGE_MISSING');
      if(await b.isLite()){
        if(kind==='activate')await b.applyLite();else await b.stopLite();
      }else if(kind==='activate')await flow.activate();
      else await flow.disable();
    }catch(error){emit(rfUi('Ошибка настройки DNS: ','DNS configuration error: ')+String(error?.message||error)+rfUi(' · проверь состояние перед повтором',' · check status before retrying'),true);}
    finally{if(!stopped){busy=false;await refresh();}}
  }
  onMount(()=>{void refresh();return ()=>{stopped=true;++generation;};});
</script>
<button id="drP92Apply" type="button" class="button primary" disabled={busy||!known||(active&&!liteMode)} onclick={()=>operation('activate')}>{rfUi("▶ Применить DNS","▶ Apply DNS")}</button>
<button id="drP10Disable" type="button" class="button" disabled={busy||!known||!active} onclick={()=>operation('disable')}>{rfUi("■ Отключить DNS","■ Disable DNS")}</button>
{#if failed}<span class="dr-p10-error" role="alert">{failed}</span>{/if}
