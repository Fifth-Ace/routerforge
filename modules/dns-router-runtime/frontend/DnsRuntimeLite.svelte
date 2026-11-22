<script>
  import { rfUi } from './rf-localization.js';
  import {onMount,untrack} from 'svelte';
  import {createLiteFlow} from './DnsRuntimeLiteLogic.js';
  import {p10Runtime,registerLite} from './DnsRuntimeCommands.js';
  import {draft,shell,feedback} from './DnsRuntimeStore.svelte.js';
  let available=$state(false);
  let active=$state(false);
  let loading=$state(false);
  let message=$state(rfUi('Чтение состояния…',"Reading status…"));
  let traceText=$state(rfUi('Запросов пока нет.',"No requests yet."));
  let connection=$state('disconnected');
  let connectionMAC=$state('');
  let upstreams=$state([]);
  let defaultUpstream=$state('');
  let listen=$state('192.168.10.1:55358');
  let cacheMiB=$state(1);
  let networks=$state('127.0.0.0/8,192.168.10.0/24');
  let mac=$state('');
  let persist=$state(false);
  let lastError=$state('');
  let stopped=false;
  let busy=$state(false);
  const bridge=()=>p10Runtime;
  const emit=feedback;
  import {apiRequest} from './DnsRuntimeApi.js';
  const request=apiRequest;
  function signal(data){
    if(stopped)return;
    if(typeof data.mode==='boolean'){
      available=data.mode;
      shell.lite=available;
    }
    if(typeof data.active==='boolean'){
      active=data.active;
      message=data.active?rfUi('DNS РАБОТАЕТ · ',"DNS RUNNING ·")+(data.listen||'')+' · default: '+
        (data.default_upstream||'')+rfUi(' · запросов ',"· requests")+(data.requests||0)+rfUi(' · кэш ',"· cache")+(data.cache_hits||0):rfUi('DNS остановлен',"DNS stopped");
      connection=data.connection?.state||'disconnected';
      connectionMAC=data.connection?.mac||'';
      message+=' · Keenetic: '+connection+(connectionMAC?' / '+connectionMAC:'')+
        (data.last_error?' · '+data.last_error:'');
      traceText=data.traces||rfUi('Запросов пока нет.',"No requests yet.");
      shell.liteActive=Boolean(data.active);shell.liteKnown=true;
    }
  }
  const flow=createLiteFlow({
    request,
    readDraft:()=>bridge()?.readDraft(),
    commit:(type,data)=>bridge()?.commit(type,data),
    confirmAction:message=>window.confirm(message),
    stagePending:()=>{const b=bridge();if(b?.simple()?.pending)b.stagePending();},
    notify:emit,signal
  });
  const fields=()=>({defaultUpstream,listen,cacheMiB,networks,mac,persist});
  async function execute(name){
    if(loading||stopped)return;
    loading=true;busy=true;lastError='';
    try{await flow.operate(name,fields());}
    catch(error){lastError=String(error?.message||error);emit('DNS Lite: '+lastError);}
    finally{loading=false;busy=false;}
  }
  function edit(){
    if(stopped||!available)return;
    try{flow.editPreferences(fields());lastError='';}
    catch(error){lastError=String(error?.message||error);emit('DNS Lite: '+lastError);}
  }
  function updateDraft(){
    const data=draft.config;
    upstreams=data.upstreams||[];
    const id=data.service?.default_upstream||defaultUpstream;
    defaultUpstream=upstreams.some(x=>x.id===id)?id:(upstreams[0]?.id||'');
    if(data.service){
      listen=data.service.listen||listen;
      cacheMiB=Number(data.service.cache_bytes||0)/1048576;
      networks=(data.service.allowed_networks||[]).join(',');
    }
  }
  $effect(()=>{void draft.generation;untrack(updateDraft);});
  onMount(()=>{
    const api=Object.freeze({detect:()=>flow.detect(),refresh:()=>flow.refresh(),
      apply:()=>execute('apply'),stop:()=>execute('stop')});
    const unregister=registerLite(api);
    return ()=>{stopped=true;unregister();};
  });
</script>

<section class="dr-simple-panel" id="drLitePanel" hidden={!available} aria-label={rfUi("Самостоятельный DNS-сервис","Standalone DNS service")}>
  <h2>{rfUi("Локальный DNS-сервис","Local DNS service")}</h2>
  <p>{rfUi("Один локальный DNS-вход, доменные списки и DNS по умолчанию. Штатные доменные зоны не расходуются. Сервис использует собственные DNS-соединения.","One local DNS endpoint, domain lists, and default DNS. No built-in domain zone slots are consumed. The service uses its own DNS connections.")}</p>
  <div class="dr-p91-form-row">
    <label class="dr-simple-field"><span>{rfUi("DNS по умолчанию","Default DNS")}</span><select id="drLiteDefault" bind:value={defaultUpstream} onchange={edit}>{#each upstreams as u (u.id)}<option value={u.id}>{u.id} · {u.transport} · {u.address}</option>{/each}</select></label>
    <label class="dr-simple-field"><span>{rfUi("Адрес сервиса","Service address")}</span><input id="drLiteListen" bind:value={listen} onchange={edit} spellcheck="false" /></label>
    <label class="dr-simple-field"><span>{rfUi("Кэш, МиБ (0 — выключен)","Cache, MiB (0 = disabled)")}</span><input id="drLiteCache" type="number" min="0" max="8" bind:value={cacheMiB} onchange={edit} /></label>
  </div>
  <label class="dr-simple-field"><span>{rfUi("Разрешённые клиентские сети, через запятую","Allowed client networks, comma-separated")}</span><input id="drLiteNetworks" bind:value={networks} onchange={edit} spellcheck="false" /></label>
  <p class="dr-simple-help">{rfUi("«Применить DNS» запускает сервис и сохраняет автозапуск. Подключение устройства выполняется отдельно. Штатный DNS Keenetic не заменяется автоматически.","Apply DNS starts the service and enables automatic startup. Connect devices separately. Keenetic's system DNS is not replaced automatically.")}</p>
  <div class="action-bar">
    <button type="button" class="button primary" id="drLiteApply" disabled={busy||!available||!upstreams.length} onclick={()=>execute('apply')}>{rfUi("▶ Применить DNS","▶ Apply DNS")}</button>
    <button type="button" class="button" id="drLiteVerify" disabled={busy||!available} onclick={()=>execute('verify')}>{rfUi("✓ Проверить сервис UDP/TCP","✓ Test UDP/TCP service")}</button>
    <button type="button" class="button" id="drLiteStop" disabled={busy||!active} onclick={()=>execute('stop')}>{rfUi("■ Остановить DNS","■ Stop DNS")}</button>
    <button type="button" class="button" id="drLiteRefresh" disabled={busy||!available} onclick={()=>{void flow.refresh().catch(e=>{lastError=String(e.message);emit(lastError);});}}>{rfUi("↻ Состояние и запросы","↻ Status and queries")}</button>
  </div>
  <p id="drLiteStatus" role="status">{message}</p>
  {#if lastError}<p role="alert" class="dr-p10-error">{lastError}</p>{/if}
  <details><summary>{rfUi("Подключить одно тестовое устройство к DNS через Keenetic","Connect one test device to DNS through Keenetic")}</summary>
    <p>{rfUi("Будет создан отдельный профиль RF_DNS_LITE для указанного MAC. Предыдущее назначение сохраняется для отключения.","A separate RF_DNS_LITE profile will be created for this MAC. The previous assignment will be saved for rollback.")}</p>
    <label class="dr-simple-field"><span>{rfUi("MAC тестового устройства","Test device MAC")}</span><input id="drLiteMAC" placeholder="aa:bb:cc:dd:ee:ff" bind:value={mac} spellcheck="false" /></label>
    <label><input type="checkbox" id="drLitePersist" bind:checked={persist} /> {rfUi("Сохранить текущую конфигурацию Keenetic, чтобы подключение пережило перезагрузку","Save the current Keenetic configuration to keep the connection after reboot")}</label>
    <div class="action-bar">
      <button type="button" class="button primary" id="drLiteConnect" disabled={busy||!active||connection!=='disconnected'} onclick={()=>execute('connect')}>{rfUi("Подключить устройство","Connect device")}</button>
      <button type="button" class="button" id="drLiteDisconnect" disabled={busy||connection==='disconnected'} onclick={()=>execute('disconnect')}>{rfUi("Вернуть прежний DNS устройства","Restore previous device DNS")}</button>
    </div>
    <p>{rfUi("Собственный DoH/Private DNS на устройстве должен быть выключен. Перехват внешнего DNS не включается.","Disable custom DoH/Private DNS on the device. Interception of external DNS is not enabled.")}</p>
  </details>
  <details><summary>{rfUi("Последние DNS-запросы · только в памяти, до 128 записей","Recent DNS queries · in memory only, up to 128 entries")}</summary><pre id="drLiteTraces">{traceText}</pre></details>
</section>
