<script>
  import {onMount,untrack} from 'svelte';
  import {createLiteFlow} from './DnsRuntimeLiteLogic.js';
  import {p10Runtime,registerLite} from './DnsRuntimeCommands.js';
  import {draft,shell,feedback} from './DnsRuntimeStore.svelte.js';
  let available=$state(false);
  let active=$state(false);
  let loading=$state(false);
  let message=$state('Чтение состояния…');
  let traceText=$state('Запросов пока нет.');
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
      message=data.active?'DNS РАБОТАЕТ · '+(data.listen||'')+' · default: '+
        (data.default_upstream||'')+' · запросов '+(data.requests||0)+' · кэш '+(data.cache_hits||0):'DNS остановлен';
      connection=data.connection?.state||'disconnected';
      connectionMAC=data.connection?.mac||'';
      message+=' · Keenetic: '+connection+(connectionMAC?' / '+connectionMAC:'')+
        (data.last_error?' · '+data.last_error:'');
      traceText=data.traces||'Запросов пока нет.';
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

<section class="dr-simple-panel" id="drLitePanel" hidden={!available} aria-label="Самостоятельный DNS-сервис">
  <h2>DNS-сервис · DEV</h2>
  <p>Один локальный DNS-вход, доменные списки и DNS по умолчанию. Штатные доменные зоны не расходуются. Сервис использует собственные DNS-соединения.</p>
  <div class="dr-p91-form-row">
    <label class="dr-simple-field"><span>DNS по умолчанию</span><select id="drLiteDefault" bind:value={defaultUpstream} onchange={edit}>{#each upstreams as u (u.id)}<option value={u.id}>{u.id} · {u.transport} · {u.address}</option>{/each}</select></label>
    <label class="dr-simple-field"><span>Адрес сервиса</span><input id="drLiteListen" bind:value={listen} onchange={edit} spellcheck="false" /></label>
    <label class="dr-simple-field"><span>Кэш, МиБ (0 — выключен)</span><input id="drLiteCache" type="number" min="0" max="8" bind:value={cacheMiB} onchange={edit} /></label>
  </div>
  <label class="dr-simple-field"><span>Разрешённые клиентские сети, через запятую</span><input id="drLiteNetworks" bind:value={networks} onchange={edit} spellcheck="false" /></label>
  <p class="dr-simple-help">«Применить DNS» запускает сервис и сохраняет автозапуск. Подключение устройства выполняется отдельно. Штатный DNS Keenetic не заменяется автоматически.</p>
  <div class="action-bar">
    <button type="button" class="button primary" id="drLiteApply" disabled={busy||!available||!upstreams.length} onclick={()=>execute('apply')}>▶ Применить DNS</button>
    <button type="button" class="button" id="drLiteVerify" disabled={busy||!available} onclick={()=>execute('verify')}>✓ Проверить сервис UDP/TCP</button>
    <button type="button" class="button" id="drLiteStop" disabled={busy||!active} onclick={()=>execute('stop')}>■ Остановить DNS</button>
    <button type="button" class="button" id="drLiteRefresh" disabled={busy||!available} onclick={()=>{void flow.refresh().catch(e=>{lastError=String(e.message);emit(lastError);});}}>↻ Состояние и запросы</button>
  </div>
  <p id="drLiteStatus" role="status">{message}</p>
  {#if lastError}<p role="alert" class="dr-p10-error">{lastError}</p>{/if}
  <details><summary>Подключить одно тестовое устройство к DNS через Keenetic</summary>
    <p>Будет создан отдельный профиль RF_DNS_LITE для указанного MAC. Предыдущее назначение сохраняется для отключения.</p>
    <label class="dr-simple-field"><span>MAC тестового устройства</span><input id="drLiteMAC" placeholder="aa:bb:cc:dd:ee:ff" bind:value={mac} spellcheck="false" /></label>
    <label><input type="checkbox" id="drLitePersist" bind:checked={persist} /> Сохранить текущую конфигурацию Keenetic, чтобы подключение пережило перезагрузку</label>
    <div class="action-bar">
      <button type="button" class="button primary" id="drLiteConnect" disabled={busy||!active||connection!=='disconnected'} onclick={()=>execute('connect')}>Подключить устройство</button>
      <button type="button" class="button" id="drLiteDisconnect" disabled={busy||connection==='disconnected'} onclick={()=>execute('disconnect')}>Вернуть прежний DNS устройства</button>
    </div>
    <p>Собственный DoH/Private DNS на устройстве должен быть выключен. Перехват внешнего DNS не включается.</p>
  </details>
  <details><summary>Последние DNS-запросы · только в памяти, до 128 записей</summary><pre id="drLiteTraces">{traceText}</pre></details>
</section>
