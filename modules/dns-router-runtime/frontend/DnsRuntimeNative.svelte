<script>
  import { rfUi } from './rf-localization.js';
  import {onDestroy} from 'svelte';
  import {createNativeReader,parseNativeSelector} from './DnsRuntimeNativeLogic.js';
  let {value=$bindable('')}=$props();
  let choices=$state([]);
  let availability=$state(rfUi('Инвентаризация ещё не запускалась. DNS и Keenetic не изменены.',"Inventory has not been run. DNS and Keenetic are unchanged."));
  let routes=$state(rfUi('System — системный DNS. PolicyN появятся после ручной проверки.',"System is the default DNS. PolicyN entries appear after a manual check."));
  let probeStatus=$state(rfUi('DNS-запрос не выполнялся.',"No DNS request was made."));
  let bridgeStatus=$state(rfUi('Native Bridge: проверка не выполнялась.',"Native bridge: no check performed."));
  let busy=$state(false);
  let probeBusy=$state(false);
  let stopped=false;
  let sequence=0;
  let checked=$state(false);
  const get=async path=>{
    const response=await fetch('../'+path,{method:'GET',cache:'no-store',credentials:'same-origin',headers:{Accept:'application/json'}});
    if(!response.ok)throw Error('HTTP '+response.status+' · '+path);
    if(!(response.headers.get('content-type')||'').toLowerCase().includes('json'))throw Error('Ожидался JSON · '+path);
    return response.json();
  };
  const native=createNativeReader({get});
  async function scan(){
    if(stopped||busy||probeBusy)return;
    busy=true;const call=++sequence;
    checked=false;choices=[];availability=rfUi('Читаю ndnproxy / PolicyN только по запросу…',"Reading ndnproxy / PolicyN on request only…");
    probeStatus=rfUi('DNS-запрос не выполнялся.',"No DNS request was made.");
    try{
      const result=await native.scan();
      if(stopped||call!==sequence)return;
      checked=true;choices=result.choices;
      routes=result.summary;
      bridgeStatus=JSON.stringify(result.bridge,null,2);
      availability=result.available?rfUi('Найдено ',"Found")+choices.length+' уникальных селекторов. Выбор не проверяет DNS-доступность.':
        rfUi('Инвентаризация недоступна или неполная. Выбор заблокирован.',"Inventory is unavailable or incomplete. Selection is blocked.");
      if(!choices.some(x=>x.selector===value))value='';
    }catch(error){
      if(!stopped&&call===sequence){availability=rfUi('Ошибка инвентаризации: ',"Discovery error:")+String(error?.message||error);choices=[];value='';}
    }finally{if(!stopped&&call===sequence)busy=false;}
  }
  async function probe(){
    if(stopped||busy||probeBusy)return;
    const selected=value;
    const parsed=parseNativeSelector(selected);
    if(!parsed||parsed.zone!=='.'){
      probeStatus=rfUi('Выбери upstream корневой зоны «.» (System / PolicyN).',"Select the resolver for the root zone “.” (System / PolicyN).");return;
    }
    probeBusy=true;probeStatus=rfUi('Один DNS-запрос example.com к явно выбранному endpoint…',"One DNS query for example.com to the explicitly selected endpoint…");
    try{
      const result=await native.probe(selected);
      if(!stopped&&selected===value){
        probeStatus=JSON.stringify(result,null,2)+'\n'+
          (result?.dns_success?rfUi('DNS ответил; шифрование этим не подтверждено.',"DNS responded; this does not verify encryption."):rfUi('DNS-проверка неуспешна.',"DNS check failed."));
      }
    }catch(error){if(!stopped&&selected===value)probeStatus=rfUi('Ошибка DNS-проверки: ',"DNS test error:")+String(error?.message||error);}
    finally{if(!stopped)probeBusy=false;}
  }
  // There is deliberately no onMount network request or automatic probe.
  onDestroy(()=>{stopped=true;sequence++;});
</script>
<div id="p85NativePicker" class="span-all" data-native-svelte-picker="true">
  <label><span>{rfUi("Штатный DNS Keenetic — System / PolicyN","Keenetic system DNS — System / PolicyN")}</span>
    <select id="p85NativeEndpoint" bind:value disabled={busy||!checked||!choices.length}>
      <option value="">{rfUi("Выбери DNS после инвентаризации","Select DNS after discovery")}</option>
      {#if value && !choices.some(item=>item.selector===value)}
        <option value={value}>{rfUi("Селектор текущего черновика · инвентаризация не подтверждена","Current draft selection · discovery not verified")}</option>
      {/if}
      {#each choices as item (item.selector)}
        <option value={item.selector}>{item.label}</option>
      {/each}
    </select>
  </label>
  <div class="panel-actions">
    <button id="p85NativeScan" class="button" type="button" disabled={busy||probeBusy} onclick={scan}>{rfUi("Найти DNS-инстансы (RO)","Find DNS instances (read-only)")}</button>
    <button id="p87NativeProbe" class="button" type="button" disabled={busy||probeBusy||!parseNativeSelector(value)} onclick={probe}>{rfUi("Проверить выбранный DNS (один запрос)","Test selected DNS (one query)")}</button>
  </div>
  <p id="p85NativeMessage" class="boundary-copy">{availability}</p>
  <p id="p87PolicySummary" class="boundary-copy">{routes}</p>
  <pre id="p87ProbeResult" class="console">{probeStatus}</pre>
  <details><summary>Native Bridge / read-only</summary><pre id="p85NativeInventoryStatus" class="console">{bridgeStatus}</pre></details>
  <p class="boundary-copy">{rfUi("Проверка селектора не изменяет Keenetic. Окончательный выбор попадает только в черновик. Для Native Bridge необходим отдельный экспериментальный capability-флаг.","Verifying the selected DNS does not change Keenetic. The final selection remains a draft. The native bridge requires a separate experimental capability flag.")}</p>
</div>
