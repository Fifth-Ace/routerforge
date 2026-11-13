<script>
  import {onMount, setContext} from 'svelte';
  import {startDnsLiteController} from './DnsLiteControllerLogic.js';
  import DnsLiteCards from './DnsLiteCards.svelte';
  import DnsLiteShellState from './DnsLiteShellState.svelte';
  import {startDnsLiteShellBridge} from './DnsLiteShellBridge.js';
  import DnsLiteDomainEditor from './DnsLiteDomainEditor.svelte';
  import DnsLiteLog from './DnsLiteLog.svelte';
  import DnsLiteStatus from './DnsLiteStatus.svelte';
  import DnsLiteControls from './DnsLiteControls.svelte';
  import DnsLiteFeedback from './DnsLiteFeedback.svelte';
  import {createLiteSettingsFields} from './DnsLiteSettingsModel.js';
  import DnsModeStatus from './DnsModeStatus.svelte';
  import DnsP21Workspace from './DnsP21Workspace.svelte';
  import DnsP22Navigation from './DnsP22Navigation.svelte';
  import DnsP25Settings from './DnsP25Settings.svelte';
  // Svelte owns domain editor presentation; the controller retains only a
  // private, in-memory draft until the user explicitly saves and applies.
  const state = $state({config:{schema_version:1,upstreams:[],rules:[]},revision:'',dirty:false,busy:false,editing:null,editingList:'__legacy__',editingDNSId:null,editorBase:null,checked:null,status:null,logs:[],frozenLogs:[],logPaused:false,health:[],page:'dns',lastLogError:''});
  const ui = $state({
    cardsView:[], defaultSelection:'', cardHealth:{},
    editorView:{opened:false,generation:0}, modeView:'basic', statusView:null,
    logView:{rows:[],upstreams:[],paused:false,updated:'',error:'',filter:'',type:'',errorsOnly:false},
    logCriteria:{filter:'',type:'',errorsOnly:false},
    dnsForm:{opened:false,title:'Добавить DNS',button:'Добавить DNS',protocol:'doh',address:'',
      ready:false,checks:{},error:'',certificateInfo:'Проверка ещё не выполнялась.',certificateText:'',certificateVisible:false},

    initialized:false, p21Rows:{devices:[],groups:[],rules:[]}, p21Section:'devices',
    p21Model:{rows:{devices:[],groups:[],rules:[]},targets:[],groups:[],inventory:[],cache:'',discoveryStatus:''},
    p21Selection:null, p21SelectionSeq:0, p21Route:null, p21ProbeMAC:'',
    controlView:{gateEnableDisabled:true,gateDisableDisabled:true,preflightDisabled:true,
      stopDisabled:true,connectDisabled:true,disconnectDisabled:true,
      preflightLabel:'Проверить готовность'}
  });
  // Reactive, read-only presentation. API/CAS/Gate operations stay in the
  // controller; status fields no longer depend on imperative DOM writes.
  let presentation = $state({
    serviceBadge:'Проверяем…',serviceTone:'',healthUpdated:'Нет измерений',
    healthGate:'—',healthServers:'—',healthRequests:'—',healthP95:'—',
    nativeDNSHealth:'Проверяем модуль DNS…',gateBadge:'Выключен',gateTone:'',gatePanelState:'stock',
    listenerSummary:'—',connectionSummary:'Для LAN-режима отдельное тестовое устройство не требуется.',
    gateAdvice:'Системный DNS выключен. Сейчас запросы обрабатывает Keenetic.',
    runStatus:'Состояние DNS-сервиса загружается…',routeAdvice:'Проверяем, доходят ли запросы до DNS Router…',
    routeTone:'',clientState:'Не подключён',liveBadge:'LIVE · ожидание',liveTone:'',
    logUpdated:'Ждём запросов',metricRequests:0,metricCache:0,metricErrors:0,metricRunning:'—',
    logCounter:0,logVisible:'0 / 0', feedbackText:'Загружаем настройки…',feedbackTone:''
  });
  const settingsFields = $state(createLiteSettingsFields());
  const settingsHooks = {fields:settingsFields, change:() => {}, render:() => {}};
  const discoveryHooks = {render:()=>{}, refresh:()=>Promise.resolve(), snapshot:()=>null};
  const navigationHooks = {navigate:()=>{}};
  setContext('routerforge:dns-lite-settings',settingsHooks);
  setContext('routerforge:dns-lite-discovery',discoveryHooks);
  setContext('routerforge:dns-lite-navigation',navigationHooks);
  function portal(node, id) {
    const target = document.getElementById(id);
    if (!target) throw Error('DNS_LITE_PORTAL_TARGET_MISSING: '+id);
    target.replaceChildren(node);
    return {destroy(){if(node.parentNode===target)node.remove();}};
  }
  function cardAction(name,id,value) {controller.commands.card?.(name,id,value);}
  function editorAction(name,value) {return controller.commands.editor?.(name,value);}
  function modeAction(mode) {controller.commands.mode?.(mode);}
  function statusAction() {controller.commands.refresh?.();}
  function p21RowAction(kind, action, key) {controller.commands.p21Row?.(kind,action,key);}
  function p21FormAction(kind, payload) {controller.commands.p21Form?.(kind,payload);}
  function p21Navigate(which) {controller.commands.p21Navigate?.(which);}
  function controlAction(which, payload) {return controller.commands.control?.(which, payload);}
  const controller = {commands:{}};
  onMount(() => {
    const shellBridge=startDnsLiteShellBridge();
    try {
      const stop=startDnsLiteController({state,ui,presentation,settingsHooks,discoveryHooks,navigationHooks,controller,reportHeight:shellBridge.reportHeight});
      // Polling lifetime belongs to this Svelte root and ends before controller disposal.
      const tick=kind=>{if(!document.hidden)controller.poll?.[kind]?.();};
      const timers=[
        setInterval(()=>tick('health'),60000),
        setInterval(()=>tick('logs'),2000),
        setInterval(()=>tick('overviewLogs'),15000),
        setInterval(()=>tick('status'),15000)
      ];
      return () => {for(const timer of timers)clearInterval(timer);stop();shellBridge.dispose();};
    } catch(error) {shellBridge.dispose();throw error;}
  });
</script>

<DnsLiteControls onAction={controlAction} chosen={state.editing||''} page={state.page} logView={ui.logView} logCounter={presentation.logCounter} logVisible={presentation.logVisible} form={ui.dnsForm} cards={ui.cardsView} defaultValue={ui.defaultSelection} busy={state.busy} controls={ui.controlView} dirty={state.dirty}/>
<DnsLiteShellState page={state.page} editing={state.editing} config={state.config} mode={ui.modeView} active={Boolean(state.status?.device_policies_active)}/>
<DnsLiteFeedback text={presentation.feedbackText} tone={presentation.feedbackTone}/>
<DnsLiteStatus view={presentation}/>
<div use:portal={'rf-dns-lite-svelte-mode'} style="display:contents">
  <DnsModeStatus mode={ui.modeView} status={ui.statusView} busy={state.busy} onMode={modeAction} onRefresh={statusAction}/>
</div>
<div use:portal={'dnsCards'} style="display:contents">
  <DnsLiteCards cards={ui.cardsView} selected={state.editing||''} health={ui.cardHealth} onAction={cardAction}/>
</div>
<div use:portal={'rf-dns-lite-svelte-editor'} style="display:contents">
  <DnsLiteDomainEditor snapshot={ui.editorView} onAction={editorAction}/>
</div>
<div use:portal={'rf-dns-lite-svelte-log'} style="display:contents">
  <DnsLiteLog view={ui.logView}/>
</div>
{#if ui.initialized}
  <DnsP22Navigation mode={ui.modeView} page={state.page} section={ui.p21Section} busy={state.busy} onNavigate={p21Navigate}/>
  <DnsP21Workspace model={ui.p21Model} section={ui.p21Section} busy={state.busy} selection={ui.p21Selection} route={ui.p21Route} probeMac={ui.p21ProbeMAC} onAction={p21FormAction} onRowAction={p21RowAction}/>
  <DnsP25Settings/>
{/if}
