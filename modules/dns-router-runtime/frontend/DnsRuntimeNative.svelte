<script>
  import {onDestroy} from 'svelte';
  import {createNativeReader,parseNativeSelector} from './DnsRuntimeNativeLogic.js';
  let {value=$bindable('')}=$props();
  let choices=$state([]);
  let availability=$state('Инвентаризация ещё не запускалась. DNS и Keenetic не изменены.');
  let routes=$state('System — системный DNS. PolicyN появятся после ручной проверки.');
  let probeStatus=$state('DNS-запрос не выполнялся.');
  let bridgeStatus=$state('Native Bridge: проверка не выполнялась.');
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
    checked=false;choices=[];availability='Читаю ndnproxy / PolicyN только по запросу…';
    probeStatus='DNS-запрос не выполнялся.';
    try{
      const result=await native.scan();
      if(stopped||call!==sequence)return;
      checked=true;choices=result.choices;
      routes=result.summary;
      bridgeStatus=JSON.stringify(result.bridge,null,2);
      availability=result.available?'Найдено '+choices.length+' уникальных селекторов. Выбор не проверяет DNS-доступность.':
        'Инвентаризация недоступна или неполная. Выбор заблокирован.';
      if(!choices.some(x=>x.selector===value))value='';
    }catch(error){
      if(!stopped&&call===sequence){availability='Ошибка инвентаризации: '+String(error?.message||error);choices=[];value='';}
    }finally{if(!stopped&&call===sequence)busy=false;}
  }
  async function probe(){
    if(stopped||busy||probeBusy)return;
    const selected=value;
    const parsed=parseNativeSelector(selected);
    if(!parsed||parsed.zone!=='.'){
      probeStatus='Выбери upstream корневой зоны «.» (System / PolicyN).';return;
    }
    probeBusy=true;probeStatus='Один DNS-запрос example.com к явно выбранному endpoint…';
    try{
      const result=await native.probe(selected);
      if(!stopped&&selected===value){
        probeStatus=JSON.stringify(result,null,2)+'\n'+
          (result?.dns_success?'DNS ответил; шифрование этим не подтверждено.':'DNS-проверка неуспешна.');
      }
    }catch(error){if(!stopped&&selected===value)probeStatus='Ошибка DNS-проверки: '+String(error?.message||error);}
    finally{if(!stopped)probeBusy=false;}
  }
  // There is deliberately no onMount network request or automatic probe.
  onDestroy(()=>{stopped=true;sequence++;});
</script>
<div id="p85NativePicker" class="span-all" data-native-svelte-picker="true">
  <label><span>Штатный DNS Keenetic — System / PolicyN</span>
    <select id="p85NativeEndpoint" bind:value disabled={busy||!checked||!choices.length}>
      <option value="">Выбери DNS после инвентаризации</option>
      {#if value && !choices.some(item=>item.selector===value)}
        <option value={value}>Селектор текущего черновика · инвентаризация не подтверждена</option>
      {/if}
      {#each choices as item (item.selector)}
        <option value={item.selector}>{item.label}</option>
      {/each}
    </select>
  </label>
  <div class="panel-actions">
    <button id="p85NativeScan" class="button" type="button" disabled={busy||probeBusy} onclick={scan}>Найти DNS-инстансы (RO)</button>
    <button id="p87NativeProbe" class="button" type="button" disabled={busy||probeBusy||!parseNativeSelector(value)} onclick={probe}>Проверить выбранный DNS (один запрос)</button>
  </div>
  <p id="p85NativeMessage" class="boundary-copy">{availability}</p>
  <p id="p87PolicySummary" class="boundary-copy">{routes}</p>
  <pre id="p87ProbeResult" class="console">{probeStatus}</pre>
  <details><summary>Native Bridge / read-only</summary><pre id="p85NativeInventoryStatus" class="console">{bridgeStatus}</pre></details>
  <p class="boundary-copy">Проверка селектора не изменяет Keenetic. Окончательный выбор попадает только в черновик. Для Native Bridge необходим отдельный экспериментальный capability-флаг.</p>
</div>
