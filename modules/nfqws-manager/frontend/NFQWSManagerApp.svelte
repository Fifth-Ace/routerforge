<script>
  import {onMount} from 'svelte';
  import {setupAccordion} from './NfqwsUIActions.js';
  import NfqwsListEditor from './NfqwsListEditor.svelte';
  import NfqwsListTools from './NfqwsListTools.svelte';
  import NfqwsPreflight from './NfqwsPreflight.svelte';
  import NfqwsStrategyLibrary from './NfqwsStrategyLibrary.svelte';
  import NfqwsSandbox from './NfqwsSandbox.svelte';
  import NfqwsSelector from './NfqwsSelector.svelte';
  import NfqwsAutoSetup from './NfqwsAutoSetup.svelte';
  import NfqwsObserved from './NfqwsObserved.svelte';
  import NfqwsTargetMemory from './NfqwsTargetMemory.svelte';
  import NfqwsDomainTest from './NfqwsDomainTest.svelte';
  import NfqwsProductionLab from './NfqwsProductionLab.svelte';
  import NfqwsTCP16Memory from './NfqwsTCP16Memory.svelte';
  import NfqwsStrategyRegistry from './NfqwsStrategyRegistry.svelte';
  import NfqwsStrategyImport from './NfqwsStrategyImport.svelte';
  import NfqwsClientHello from './NfqwsClientHello.svelte';
  import NfqwsDPIStatus from './NfqwsDPIStatus.svelte';
  import NfqwsDPIWorkspace from './NfqwsDPIWorkspace.svelte';
  import NfqwsProcessingMap from './NfqwsProcessingMap.svelte';
  import {dpiStatusProjection} from './NfqwsDPIStatusModel.js';
  import {startNFQWSRuntime} from './NfqwsRuntime.js';
  import {mountNfqwsHostBridge} from './NfqwsHostBridge.js';
  import NfqwsLogs from './NfqwsLogs.svelte';
  import NfqwsScripts from './NfqwsScripts.svelte';
  import NfqwsBackupCenter from './NfqwsBackupCenter.svelte';
  import NfqwsListSources from './NfqwsListSources.svelte';
  import NfqwsSmartApply from './NfqwsSmartApply.svelte';
  import NfqwsCatalog from './NfqwsCatalog.svelte';
  import NfqwsBlobs from './NfqwsBlobs.svelte';
  import NfqwsGeoAssets from './NfqwsGeoAssets.svelte';
  import NfqwsKeenetic from './NfqwsKeenetic.svelte';
  import NfqwsRuntimeHealth from './NfqwsRuntimeHealth.svelte';
  import NfqwsRunHistory from './NfqwsRunHistory.svelte';
  import NfqwsDomainBatchCheck from './NfqwsDomainBatchCheck.svelte';
  import NfqwsSetupChips from './NfqwsSetupChips.svelte';
  import NfqwsSetupDiagnostics from './NfqwsSetupDiagnostics.svelte';
  import NfqwsSetupDiff from './NfqwsSetupDiff.svelte';
  import NfqwsCustomStrategies from './NfqwsCustomStrategies.svelte';
  import NfqwsFlowModes from './NfqwsFlowModes.svelte';
  import NfqwsArgumentField from './NfqwsArgumentField.svelte';
  import NfqwsConfigLibrary from './NfqwsConfigLibrary.svelte';
  import NfqwsRawConfig from './NfqwsRawConfig.svelte';
  import {hydrateFlowFields,editFlowField} from './NfqwsFlowFieldsModel.js';
  import {nfqwsOverview} from './NfqwsOverviewModel.js';
  import {NFQWS_PRIMARY_DEFAULTS,primaryForLeaf,resolveLeafTab,resolvePrimaryTab} from './NfqwsNavigationModel.js';
  // Svelte owns DOM creation; the extracted legacy controller retains CAS/rollback logic
  // during migration of its 491 imperative functions. Do not declare ownership complete.
  let activeTab = $state('overview');
  let activePrimary = $derived(primaryForLeaf(activeTab));
  let runtime;
  let service = $state({available:false,running:false,mutationReady:false,target:'nfqws',configPath:'target not detected',backupCount:0});
  let serviceBusy = $state(true);
  let message = $state({text:'Загрузка состояния NFQWS…',type:''});
  let logData = $state({tail:'',path:'—',truncated:false});
  let logSearchRequest = $state({term:'',seq:0});
  let scriptOpenRequest=$state({seq:0,name:''});
  let scriptCount=$state(0);
  let overview=$state(nfqwsOverview(null));
  let strategyCount=$state(null);
  let configLibraryCount=$state(0);
  let rawConfigSnapshot=$state(null);
  let strategyLibraryRefreshSeq=$state(0);
  let sandboxSeed=$state({seq:0});
  let selectorSeed=$state({seq:0,target:''});
  let autoSetupSeed=$state({seq:0,target:''});
  let domainSeed=$state({seq:0,target:''});
  let labSeed=$state({seq:0,target:''});
  let memoryCount=$state(0);
  let registryCount=$state(0);
  let registryRefreshSeq=$state(0);
  let dpiInfo=$state({snapshot:null,loading:false,error:''});
  const dpiView=$derived(dpiStatusProjection(dpiInfo.snapshot,dpiInfo.loading,dpiInfo.error));
  let selectorController=null;
  let listInventory=$state([]);
  let listToolState=$state({inspect:false,path:false,analysis:false});
  let listInspection=$state({open:false,record:null,findings:[],progress:'',error:'',busy:false});
  let listStaticPath=$state({open:false,record:null,result:null,progress:'',error:'',busy:false});
  let listAnalysis=$state({open:false,selected:'',findings:[],progress:'',error:'',busy:false,dirty:false});
  let preflight=$state({state:'BLOCKED',dirty:false,busy:false,bad:0,warn:0,good:0,groups:[]});
  let listController=null;
  const strategyRequest=(path,options)=>runtime?.strategyLibraryRequest(path,options);
  const strategySandbox=candidate=>runtime?.strategyLibrarySandbox(candidate);
  const sandboxHistory=(kind,metrics,payload)=>runtime?.sandboxHistory(kind,metrics,payload);
  const selectorHistory=(kind,metrics,payload)=>runtime?.selectorHistory(kind,metrics,payload);
  const labHistory=(kind,metrics,payload)=>runtime?.labHistory(kind,metrics,payload);
  function domainToSelector(target){selectorSeed={target,source:'domain',seq:selectorSeed.seq+1};navigateTab('selector-v2');}
  const onListDraft=snapshot=>runtime?.listDraftChanged(snapshot);
  const onListRequest=(path,options)=>runtime?.listRequest(path,options);
  const onListStatus=status=>runtime?.listStatus(status);
  const onListTool=name=>runtime?.listTool(name);
  const onListToolsAction=(name,payload)=>runtime?.listToolsAction(name,payload);
  let configOpenRequest=$state({seq:0,name:''});
  let backupView=$state({files:[],root:'',selected:'',detail:null,loading:false,error:'',safetyCount:0,detailError:'',detailLoading:false});
  const backupAction=(name,id)=>runtime?.backup?.(name,id);
  let listSourceView=$state({local:[],upstream:[],root:'',error:'',loading:false,ref:'',resolution:null});
  const listSourceAction=(name,id)=>runtime?.listSources?.(name,id);
  let smartApplyView=$state({open:false,busy:false,preparing:false,plan:null,error:''});
  let catalogView=$state({items:[],selected:'',detail:null,upstream:{state:'idle'},loading:false,selecting:false,error:''});
  const catalogAction=(name,id)=>runtime?.catalogAction?.(name,id);
  let blobView=$state({files:[],dependencies:[],root:'',loading:false,busy:false,error:''});
  const blobAction=(name,id)=>runtime?.blobAction?.(name,id);
  let historyView=$state({runs:[],count:0,latestScore:'—',sizeKb:'0.1',autoPick:[],winner:'',guard:{enabled:false,armed:false,running:false,last:null}});
  let recoveryGuardEnabled=$state(false);
  let recoveryGuardThreshold=$state(20);
  const historyAction=(name,id)=>runtime?.historyAction?.(name,id);
  let domainCheckView=$state({open:false,rows:[],total:0,ok:0,fail:0,progress:0,busy:false});
  const domainCheckAction=name=>runtime?.domainCheckAction?.(name);
  let setupView=$state({dirty:false,changes:[],overview:[],diagnostics:{items:[],bad:0,warn:0,good:0,state:'CHECK',stateClass:'warn',message:'загрузка'},chips:{},internals:{queue:'—',user:'—',version:'—'},canApply:false});
  let setupDiff=$state({open:false,lines:[]});
  const setupAction=name=>runtime?.setupAction?.(name);
  function historySettingsChanged(enabled,threshold){recoveryGuardEnabled=enabled;recoveryGuardThreshold=threshold;historyAction('settings');}
  let customStrategies=$state({blocks:[],labels:[]});
  let flowModes=$state({domain:'MODE_LIST',policy:'none'});
  let flowFields=$state(hydrateFlowFields());
  let availableFlowLists=$state([]);
  const openFlowResource=(kind,name)=>runtime?.openResource(kind,name);
  function editFlowFieldUI(key,value){flowFields=editFlowField(flowFields,key,value);runtime?.flowFieldsChanged();}
  function changeFlowMode(kind,value){runtime?.flowModeChanged(kind,value);}
  function updateCustomStrategies(snapshot){customStrategies=snapshot;runtime?.customEdited();}
  const smartApplyAction=name=>runtime?.smartApply?.(name);
  function observedNavigate(action,target) {
    if(action==='setup'){autoSetupSeed={target,source:'Observed Targets',seq:autoSetupSeed.seq+1};navigateTab('auto-setup-v1');}
    else if(action==='selector'){selectorSeed={target,source:'observed',seq:selectorSeed.seq+1};navigateTab('selector-v2');}
    else if(action==='sandbox'){sandboxSeed={kind:'target',target,seq:sandboxSeed.seq+1};navigateTab('sandbox-v2');}
    else if(action==='diagnose'){runtime?.openDomainTarget(target);}
  }
  // Sandbox Save invalidates only the Svelte-owned library inventory.
  // Never make a second POST or refresh an active nfqws2.conf from this callback.
  function strategyRefresh(){strategyLibraryRefreshSeq+=1;}
  function importedStrategiesChanged(){
    // Actual Svelte owners refresh from the server after an acknowledged import.
    registryRefreshSeq+=1;
    strategyLibraryRefreshSeq+=1;
  }
  function useKeeneticDefault(iface){
    if(!iface)return;
    flowFields={...flowFields,ISP_INTERFACE:iface};
    runtime?.flowFieldsChanged();
    navigateTab('strategies');
    message={text:`ISP_INTERFACE draft ← ${iface}. Ничего не сохранено.`,type:'good'};
  }
  let overviewFocusFrame = 0;
  function openOverviewTask(tab,focusId) {
    navigateTab(tab);
    // Focus is a UI-only affordance. A tab switch has no service side effects.
    if (overviewFocusFrame) cancelAnimationFrame(overviewFocusFrame);
    overviewFocusFrame = requestAnimationFrame(()=>{
      overviewFocusFrame = 0;
      if(activeTab!==tab || !focusId)return;
      const target=document.getElementById(focusId);
      target?.focus();
      if(target?.select && target.value)target.select();
    });
  }
  function control(action) { if(runtime && !serviceBusy) void runtime.control(action); }
  function refresh() { if(runtime && !serviceBusy) void runtime.refresh(); }
  let lastTab = $state({...NFQWS_PRIMARY_DEFAULTS});
  function setTab(tab) {
    activeTab = resolveLeafTab(tab);
    const group = primaryForLeaf(tab);
    if (group !== 'overview') lastTab[group] = tab;
  }
  function navigateTab(tab) {
    if (!runtime) return;
    runtime.navigate(resolveLeafTab(tab));
  }
  function navigatePrimary(group) {
    if (!runtime) return;
    runtime.navigate(resolvePrimaryTab(group,lastTab));
  }
  onMount(() => {
    const frame = mountNfqwsHostBridge();
    const keys = new AbortController();
    document.addEventListener('keydown', event => runtime?.handleShortcut(event, activeTab), {capture:true,signal:keys.signal});
    document.addEventListener('keydown', event => runtime?.handleEscape(event), {signal:keys.signal});
    runtime = startNFQWSRuntime({onHeight:frame.reportHeight,onNavigate:setTab,onStatus:value=>service=value,onBusy:value=>serviceBusy=value,onNotice:value=>message=value,onLogs:value=>logData=value,onLogSearch:term=>logSearchRequest={term,seq:logSearchRequest.seq+1},onScriptOpen:name=>scriptOpenRequest={name,seq:scriptOpenRequest.seq+1},onOverview:value=>overview=value,onStrategyCount:value=>strategyCount=value,onBackups:value=>backupView=value,onListSources:value=>listSourceView=value,onSmartApply:value=>smartApplyView=value,onCatalog:value=>catalogView=value,onBlobs:value=>blobView=value,onHistory:value=>historyView=value,onDomainCheck:value=>domainCheckView=value,onSetup:value=>setupView=value,onDiff:value=>setupDiff=value,getRecoverySettings:()=>({enabled:recoveryGuardEnabled,threshold:recoveryGuardThreshold}),getCustomStrategies:()=>customStrategies,onCustomHydrate:next=>{customStrategies=next;},onFlowModeHydrate:next=>{flowModes={...flowModes,...next};},getFlowFields:()=>flowFields,onFlowFieldsHydrate:next=>{flowFields={...flowFields,...next};},onFlowLists:names=>{availableFlowLists=names;},onConfigOpen:name=>{configOpenRequest={seq:configOpenRequest.seq+1,name};},onRawConfig:status=>{rawConfigSnapshot=status;},onListInventory:rows=>{listInventory=rows;},onListTools:tools=>{listToolState=tools;},onListInspection:value=>{listInspection=value;},onListStaticPath:value=>{listStaticPath=value;},onListAnalysis:value=>{listAnalysis=value;},onPreflight:value=>{preflight=value;},onDpiStatus:value=>{dpiInfo=value;},getListEditor:()=>listController,onSandboxSeed:seed=>{sandboxSeed={...seed,seq:sandboxSeed.seq+1};},onSelectorSeed:seed=>{selectorSeed={...seed,seq:selectorSeed.seq+1};},onAutoSetupSeed:seed=>{autoSetupSeed={...seed,seq:autoSetupSeed.seq+1};},onDomainSeed:seed=>{domainSeed={...seed,seq:domainSeed.seq+1};},onLabSeed:seed=>{labSeed={...seed,seq:labSeed.seq+1};}});
    return () => {
      keys.abort();
      if (overviewFocusFrame) cancelAnimationFrame(overviewFocusFrame);
      overviewFocusFrame = 0;
      runtime?.dispose();
      runtime = null;
      frame.dispose();
    };
  });
</script>

<div class="shell">
<header class="hero">
  <div class="hero-left">
    <div class="logo">NFQ</div>
    <div>
      <div class="title-line"><h1>NFQWS</h1><span id="state" class="status" class:good={service.available && service.running} class:warn={service.available && !service.running} class:bad={!service.available}>{!service.available?'NOT AVAILABLE':service.running?'RUNNING':'STOPPED'}</span><span id="targetChip" class="chip">{service.target}</span></div>
      <div id="subtitle" class="subtitle">RouterForge extension · {service.configPath}</div>
    </div>
  </div>
  <div class="actions">
    <button id="start" style:display={service.running ? 'none' : ''} disabled={serviceBusy || !service.mutationReady} onclick={() => control('start')}>Запустить</button><button id="stop" style:display={service.running ? '' : 'none'} disabled={serviceBusy || !service.mutationReady} onclick={() => control('stop')}>Остановить</button><button id="reload" disabled={serviceBusy || !service.running || !service.mutationReady} onclick={() => control('reload')}>Перечитать настройки</button><button id="restart" disabled={serviceBusy || !service.running || !service.mutationReady} onclick={() => control('restart')}>Перезапустить</button><button id="refresh" disabled={serviceBusy} onclick={refresh}>Обновить</button>
  </div>
</header>

<div id="notice" class="notice" class:error={message.type === 'error'} class:good={message.type === 'good'}>{message.text}</div>

<nav class="tabs nfqws-primary-tabs" id="nfqwsPrimaryTabs" aria-label="Основная навигация NFQWS">
  <button class="tab" class:active={activeTab === 'overview'} data-tab="overview" onclick={() => navigateTab('overview')}>Обзор</button>
  <button class="tab" class:active={activePrimary === 'configuration'} data-primary-tab="configuration" onclick={() => navigatePrimary('configuration')}>Настройка</button>
  <button class="tab" class:active={activePrimary === 'strategies'} data-primary-tab="strategies" onclick={() => navigatePrimary('strategies')}>Стратегии</button>
  <button class="tab" class:active={activePrimary === 'menu'} data-primary-tab="menu" onclick={() => navigatePrimary('menu')}>NFQWS Menu</button>
  <button class="tab" class:active={activePrimary === 'tools'} data-primary-tab="tools" onclick={() => navigatePrimary('tools')}>Диагностика</button>
  <button class="tab" class:active={activePrimary === 'selection'} data-primary-tab="selection" onclick={() => navigatePrimary('selection')}>Подбор</button>
  <button class="tab" class:active={activePrimary === 'maintenance'} data-primary-tab="maintenance" onclick={() => navigatePrimary('maintenance')}>Обслуживание</button>
</nav>

<nav class="tabs nfqws-tool-tabs nfqws-config-tabs" id="nfqwsConfigTabs" class:visible={activePrimary === 'configuration'} aria-label="Настройка NFQWS">
  <button class="tab" class:active={activeTab === 'strategies'} data-tab="strategies" onclick={() => navigateTab('strategies')}>Конфигурация <span id="strategyCount" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'configs'} data-tab="configs" onclick={() => navigateTab('configs')}>Сохранённые конфиги <span id="configLibraryCount" class="nav-meta">{configLibraryCount?`(${configLibraryCount})`:''}</span></button>
  <button class="tab" class:active={activeTab === 'config'} data-tab="config" onclick={() => navigateTab('config')}>Полный конфиг</button>
  <button class="tab" class:active={activeTab === 'lists'} data-tab="lists" onclick={() => navigateTab('lists')}>Списки <span id="listCount" class="nav-meta">{listInventory.length?`(${listInventory.length})`:''}</span></button>
  <button class="tab" class:active={activeTab === 'geo-assets'} data-tab="geo-assets" onclick={() => navigateTab('geo-assets')}>Геоданные</button>
</nav>

<nav class="tabs nfqws-tool-tabs nfqws-strategy-tabs" id="nfqwsStrategyTabs" class:visible={activePrimary === 'strategies'} aria-label="Стратегии RouterForge">
  <button class="tab" class:active={activeTab === 'strategy-library-v2'} data-tab="strategy-library-v2" onclick={() => navigateTab('strategy-library-v2')}>Свои стратегии</button>
  <button class="tab" class:active={activeTab === 'strategy-registry-v1'} data-tab="strategy-registry-v1" onclick={() => navigateTab('strategy-registry-v1')}>База стратегий <span id="v2RegistryTabCount" class="nav-meta">{registryCount?`(${registryCount})`:''}</span></button>
  <button class="tab" class:active={activeTab === 'strategy-import'} data-tab="strategy-import" onclick={() => navigateTab('strategy-import')}>Импорт стратегии</button>
</nav>

<nav class="tabs nfqws-tool-tabs" id="nfqwsMenuTabs" class:visible={activePrimary === 'menu'} aria-label="NFQWS Menu">
  <button class="tab" class:active={activeTab === 'catalog'} data-tab="catalog" onclick={() => navigateTab('catalog')}>Каталог <span id="catalogTabState" class="nav-meta">{catalogView.items.length?`(${catalogView.items.length})`:''}</span></button>
  <button class="tab" class:active={activeTab === 'upstream-lists'} data-tab="upstream-lists" onclick={() => navigateTab('upstream-lists')}>Списки <span id="listSourceStateNav" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'blobs'} data-tab="blobs" onclick={() => navigateTab('blobs')}>Blobs <span id="blobTabState" class="nav-meta">{blobView.dependencies.filter(x=>x.state!=='ok').length?`(${blobView.dependencies.filter(x=>x.state!=='ok').length}!)`:blobView.files.length?`(${blobView.files.length})`: ''}</span></button>
</nav>

<nav class="tabs nfqws-tool-tabs" id="nfqwsToolTabs" class:visible={activePrimary === 'tools'} aria-label="Диагностика NFQWS">
  <button class="tab" class:active={activeTab === 'preflight'} data-tab="preflight" onclick={() => navigateTab('preflight')}>Проверка системы <span id="preflightTabState" class="nav-meta">{preflight.state==='READY'?'✓':preflight.state==='BLOCKED'?'!':'•'}</span></button>
  <button class="tab" class:active={activeTab === 'dpi-detector'} data-tab="dpi-detector" onclick={() => navigateTab('dpi-detector')}>DPI Detector <span id="dpiDetectorTabState" class="nav-meta">{dpiView.tab}</span></button>
  <button class="tab" class:active={activeTab === 'domain-v2'} data-tab="domain-v2" onclick={() => navigateTab('domain-v2')}>Проверка домена</button>
  <button class="tab" class:active={activeTab === 'prodlab-v2'} data-tab="prodlab-v2" onclick={() => navigateTab('prodlab-v2')}>Тестирование</button>
  <button class="tab" class:active={activeTab === 'clienthello'} data-tab="clienthello" onclick={() => navigateTab('clienthello')}>ClientHello</button>
</nav>

<nav class="tabs nfqws-tool-tabs" id="nfqwsSelectionTabs" class:visible={activePrimary === 'selection'} aria-label="Подбор стратегий">
  <button class="tab" class:active={activeTab === 'observed-v1'} data-tab="observed-v1" onclick={() => navigateTab('observed-v1')}>Проблемные адреса</button>
  <button class="tab" class:active={activeTab === 'auto-setup-v1'} data-tab="auto-setup-v1" onclick={() => navigateTab('auto-setup-v1')}>Автонастройка</button>
  <button class="tab" class:active={activeTab === 'selector-v2'} data-tab="selector-v2" onclick={() => navigateTab('selector-v2')}>Автоподбор</button>
  <button class="tab" class:active={activeTab === 'sandbox-v2'} data-tab="sandbox-v2" onclick={() => navigateTab('sandbox-v2')}>Песочница</button>
  <button class="tab" class:active={activeTab === 'target-memory-v2'} data-tab="target-memory-v2" onclick={() => navigateTab('target-memory-v2')}>История подборов <span id="v2MemoryTabCount" class="nav-meta">{memoryCount ? `(${memoryCount})` : ''}</span></button>
</nav>

<nav class="tabs nfqws-tool-tabs nfqws-maintenance-tabs" id="nfqwsMaintenanceTabs" class:visible={activePrimary === 'maintenance'} aria-label="Обслуживание NFQWS">
  <button class="tab" class:active={activeTab === 'runs'} data-tab="runs" onclick={() => navigateTab('runs')}>История <span id="runHistoryTabState" class="nav-meta">{historyView.count?historyView.count:""}</span></button>
  <button class="tab" class:active={activeTab === 'logs'} data-tab="logs" onclick={() => navigateTab('logs')}>Логи</button>
  <button class="tab" class:active={activeTab === 'backups'} data-tab="backups" onclick={() => navigateTab('backups')}>Резервные копии <span id="backupCountTab" class="nav-meta">{backupView.files.length ? `(${backupView.files.length})` : service.backupCount ? `(${service.backupCount})` : ''}</span></button>
  <button class="tab" class:active={activeTab === 'runtime-health-v2'} data-tab="runtime-health-v2" onclick={() => navigateTab('runtime-health-v2')}>Состояние NFQWS <span id="runtimeHealthTabState" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'scripts'} data-tab="scripts" onclick={() => navigateTab('scripts')}>Скрипты <span id="scriptCount" class="nav-meta">{scriptCount ? `(${scriptCount})` : ''}</span></button>
  <button class="tab" class:active={activeTab === 'keenetic'} data-tab="keenetic" onclick={() => navigateTab('keenetic')}>Система <span id="keeneticTabState" class="nav-meta"></span></button>
</nav>

<div id="nfqwsMenuCredit" class="strategy-import-credit nfqws-menu-credit" class:visible={activePrimary === 'menu'}>
  <div><strong>NFQWS Menu</strong><span>Интегрированный источник готовых стратегий, списков и blobs.</span></div>
  <a href="https://github.com/rndnaame/nfqws-menu/" target="_blank" rel="noopener noreferrer">Открыть GitHub ↗</a>

</div>

<main>

<section id="view-dpi-detector" class="view" class:active={activeTab === 'dpi-detector'}>
  <NfqwsDPIWorkspace active={activeTab === 'dpi-detector'} {serviceBusy} snapshot={dpiInfo.snapshot}
    loading={dpiInfo.loading} error={dpiInfo.error} onRequest={strategyRequest}
    onRefresh={()=>runtime?.dpiRefresh()} onNotice={value=>message=value}/>
</section>

<section id="view-observed-v1" class="view" class:active={activeTab === 'observed-v1'}>
  <NfqwsObserved active={activeTab === 'observed-v1'} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onTarget={observedNavigate} onNotice={value=>{message=value;}} />
</section>

<section id="view-domain-v2" class="view" class:active={activeTab === 'domain-v2'}>
  <NfqwsDomainTest active={activeTab === 'domain-v2'} seed={domainSeed} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onSelector={domainToSelector} onNotice={value=>{message=value;}} />
</section>

<section id="view-prodlab-v2" class="view" class:active={activeTab === 'prodlab-v2'}>
  <NfqwsProductionLab active={activeTab === 'prodlab-v2'} seed={labSeed} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onHistory={labHistory} onNotice={value=>{message=value;}} />
</section>

<section id="view-target-memory-v2" class="view" class:active={activeTab === 'target-memory-v2'}>
  <NfqwsTargetMemory active={activeTab === 'target-memory-v2'} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onCount={value=>{memoryCount=value;}} onNotice={value=>{message=value;}} />
</section>

<section id="view-runtime-health-v2" class="view" class:active={activeTab === 'runtime-health-v2'}>
  <NfqwsRuntimeHealth active={activeTab === 'runtime-health-v2'} onRequest={strategyRequest}/>
</section>

<section id="view-auto-setup-v1" class="view" class:active={activeTab === 'auto-setup-v1'}>
  <NfqwsAutoSetup active={activeTab === 'auto-setup-v1'} seed={autoSetupSeed} serviceBusy={serviceBusy}
    onRequest={strategyRequest} getSelector={()=>selectorController} onNotice={value=>{message=value;}} />
</section>

<section id="view-selector-v2" class="view" class:active={activeTab === 'selector-v2'}>
  <NfqwsSelector active={activeTab === 'selector-v2'} seed={selectorSeed} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onHistory={selectorHistory} onApplied={status=>runtime?.selectorApplied(status)}
    onRegister={controller=>{selectorController=controller;}} onNotice={value=>{message=value;}} />
</section>

<section id="view-sandbox-v2" class="view" class:active={activeTab === 'sandbox-v2'}>
  <NfqwsSandbox active={activeTab === 'sandbox-v2'} seed={sandboxSeed} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onRefreshLibrary={strategyRefresh} onHistory={sandboxHistory}
    onNotice={value=>{message=value;}} />
</section>

<section id="view-tcp16-memory-v1" class="view" class:active={activeTab === 'tcp16-memory-v1'}>
  <NfqwsTCP16Memory active={activeTab === 'tcp16-memory-v1'} serviceBusy={serviceBusy} onRequest={strategyRequest} />
</section>

<section id="view-strategy-registry-v1" class="view" class:active={activeTab === 'strategy-registry-v1'}>
  <NfqwsStrategyRegistry active={activeTab === 'strategy-registry-v1'} refreshSeq={registryRefreshSeq} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onCount={value=>{registryCount=value;}} />
</section>

<section id="view-strategy-library-v2" class="view" class:active={activeTab === 'strategy-library-v2'}>
  <NfqwsStrategyLibrary active={activeTab === 'strategy-library-v2'} refreshSeq={strategyLibraryRefreshSeq} serviceBusy={serviceBusy}
    onRequest={strategyRequest} onOpenSandbox={strategySandbox} onNotice={value=>{message=value;}} />
</section>

<section id="view-overview" class="view" class:active={activeTab === 'overview'}>
  <section class="ux-task-center" aria-labelledby="uxTaskTitle">
    <div class="ux-task-head">
      <div>
        <h2 id="uxTaskTitle">Что хотите сделать?</h2>
        <p>Выберите задачу обычными словами. RouterForge откроет нужный инструмент; технические детали и ручные настройки никуда не исчезают.</p>
      </div>
      <div class="ux-task-head-note">Начните с задачи<br>детали — по необходимости</div>
    </div>
    <div class="ux-task-grid">
      <button type="button" class="ux-task-card" id="uxTaskBroken" onclick={()=>openOverviewTask('domain-v2','v2DomainTarget')}>
        <span class="ux-task-icon">01</span>
        <span class="ux-task-copy"><span class="ux-task-title">Сайт или приложение не работает</span><span class="ux-task-desc">Разберём домен: списки, активный конфиг, runtime, интернет-интерфейс и проверку доступности с роутера.</span></span>
        <span class="ux-task-action">Начать диагностику</span>
      </button>
      <button type="button" class="ux-task-card" id="uxTaskCheck" onclick={()=>openOverviewTask('preflight','')}>
        <span class="ux-task-icon">02</span>
        <span class="ux-task-copy"><span class="ux-task-title">Проверить текущую настройку</span><span class="ux-task-desc">Проверим сервис, конфигурацию, зависимости и доступные runtime-сигналы без изменения настроек.</span></span>
        <span class="ux-task-action">Проверить</span>
      </button>
      <button type="button" class="ux-task-card" id="uxTaskAutoV2" onclick={()=>openOverviewTask('selector-v2','v2SelectorTarget')}>
        <span class="ux-task-icon">03</span>
        <span class="ux-task-copy"><span class="ux-task-title">Подобрать настройку автоматически</span><span class="ux-task-desc">Baseline, production profiles и Библиотека стратегий тестируются в изолированных NFQUEUE. Результат отделяет strategy failure от infrastructure failure.</span></span>
        <span class="ux-task-action">Открыть Автоподбор</span>
      </button>
      <button type="button" class="ux-task-card" id="uxTaskImport" onclick={()=>openOverviewTask('strategy-import','strategyImportInput')}>
        <span class="ux-task-icon">04</span>
        <span class="ux-task-copy"><span class="ux-task-title">Импортировать готовую стратегию</span><span class="ux-task-desc">Вставьте команду или загрузите .bat/.txt. RouterForge разберёт вход, покажет все преобразования и соберёт preview без сохранения.</span></span>
        <span class="ux-task-action">Разобрать</span>
      </button>
    </div>
    <div class="ux-task-foot">Эти быстрые сценарии не скрывают экспертные возможности: полная конфигурация, списки, NFQWS Menu, инструменты и обслуживание остаются доступны в верхней навигации.</div>
  </section>
  <div class="grid cards overview-metrics">
    <article class="card"><div class="metric-label">Сервис</div><div id="mService" class="metric-value" class:good-text={overview.running} class:bad-text={!overview.running}>{overview.running ? 'RUNNING' : 'STOPPED'}</div><div id="mServiceSub" class="metric-sub">{overview.ready ? 'manager ready' : 'mutation not ready'}</div></article>
    <article class="card"><div class="metric-label">Конфигурация</div><div id="mConfig" class="metric-value" class:good-text={overview.configExists} class:bad-text={!overview.configExists}>{overview.configExists?'OK':'MISSING'}</div><div id="mConfigSub" class="metric-sub">{overview.configSummary}</div></article>
    <article class="card"><div class="metric-label">Стратегии</div><div id="mStrategies" class="metric-value accent">{strategyCount ?? '—'}</div><div class="metric-sub">разобрано из активного конфига</div></article>
    <article class="card"><div class="metric-label">Списки</div><div id="mLists" class="metric-value">{overview.listCount}</div><div id="mListsSub" class="metric-sub">{overview.listSummary}</div></article>
  </div>
  <div class="grid overview-details">
    <article class="panel overview-runtime">
      <div class="panel-head"><div><h2>Runtime</h2><div class="section-kicker">Фактическое состояние внешнего nfqws</div></div></div>
      <dl class="kv">
        <dt>Target</dt><dd id="oTarget" class="mono">{overview.target}</dd>
        <dt>Init script</dt><dd id="oInit" class="mono">{overview.initScript}</dd>
        <dt>Config</dt><dd id="oConfigPath" class="mono">{overview.configPath}</dd>
        <dt>Config SHA256</dt><dd id="oSha" class="mono">{overview.configSHA}</dd>
        <dt>Lists root</dt><dd id="oListsRoot" class="mono">{overview.listsRoot}</dd>
        <dt>Log</dt><dd id="oLogPath" class="mono">{overview.logPath}</dd>
        <dt>Mutation boundary</dt><dd id="oMutation">{overview.mutation}</dd>
      </dl>
    </article>
    <article class="panel"><div class="panel-head"><div><h2>Состояние</h2><div class="section-kicker">Быстрая проверка runtime и safety boundary</div></div></div><div id="healthActivity" class="activity">
      {#each overview.activity as item (item.title)}
        <div class="activity-row"><span class="dot" class:good={item.ok} class:warn={!item.ok}></span><div><div>{item.title}</div><div class="muted small">{item.description}</div></div><span class="mono small" class:good-text={item.ok} class:warn-text={!item.ok}>{item.ok?'PASS':'CHECK'}</span></div>
      {/each}
    </div></article>
  </div>
</section>

<section id="view-strategy-import" class="view" class:active={activeTab === 'strategy-import'}>
  <NfqwsStrategyImport active={activeTab === 'strategy-import'} status={rawConfigSnapshot} {serviceBusy} onRequest={strategyRequest} onNotice={value=>message=value} onConfigOpen={name=>{configOpenRequest={seq:configOpenRequest.seq+1,name};navigateTab('configs');}} onSandbox={seed=>{sandboxSeed={...seed,seq:sandboxSeed.seq+1};navigateTab('sandbox-v2');}} onRegistryInvalidate={importedStrategiesChanged} onConfigSaved={()=>runtime?.configLibraryChanged()}/>
</section>

<section id="view-preflight" class="view" class:active={activeTab === 'preflight'}>
  <NfqwsPreflight view={preflight} onRefresh={()=>runtime?.preflightRefresh()} onJump={tab=>runtime?.navigate(tab)}/>
</section>

<section id="view-processing" class="view" class:active={activeTab === 'processing'}>
  <NfqwsProcessingMap active={activeTab === 'processing'} {serviceBusy} inventory={listInventory}
    onRequest={strategyRequest} onStaticPath={(domain,contents,sha)=>runtime?.processingStaticPath(domain,contents,sha)}
    onNavigate={navigateTab} onOpenList={(name,line)=>runtime?.listToolsAction('openLine',{name,line})}
    onLab={seed=>{labSeed={...seed,seq:labSeed.seq+1};}}
    onNotice={value=>{message=value;}} />
</section>

<section id="view-strategies" class="view" class:active={activeTab === 'strategies'}>
  <div class="setup-flow">
    <div class="setup-topbar setup-topbar-full">
      <div>
        <strong>Полная настройка nfqws2 сверху вниз</strong>
        <div class="muted small" style="margin-top:3px">Все параметры upstream доступны сразу. RouterForge сохраняет исходный порядок nfqws2.conf и не прячет служебные блоки.</div>
      </div>
      <span class="setup-full-badge">Полный режим</span>
    </div>
    <div id="processingOverview" class="setup-overview">{#each setupView.overview as card (card[0])}<article class="card"><div class="metric-label">{card[0]}</div><div class="metric-value">{card[1]}</div></article>{/each}</div>

    <article class="setup-section fixed-section compact-section" data-fixed="true" use:setupAccordion={0}>
      <div class="setup-head"><div class="setup-num">01</div><div><div class="setup-title">Интернет-интерфейс</div><div class="setup-key">ISP_INTERFACE</div></div><div class="setup-help">Куда уходит интернет-трафик. Обычно eth3 / eth2.2 / ppp0. Можно указать несколько интерфейсов через пробел.</div></div>
      <div class="setup-body"><div class="form-row"><div class="setup-label"><strong>Интерфейс провайдера</strong>WAN-интерфейс, через который выходит интернет.</div><div class="form-control"><input id="flowInterface" class="setup-input" spellcheck="false" value={flowFields.ISP_INTERFACE} disabled={serviceBusy} oninput={e=>editFlowFieldUI('ISP_INTERFACE',e.currentTarget.value)}></div></div></div>
    </article>

    <article class="setup-section expert-only" use:setupAccordion={1}>
      <div class="setup-head"><div class="setup-num">02</div><div><div class="setup-title">Базовая загрузка nfqws2</div><div class="setup-key">NFQWS_BASE_ARGS</div></div><div class="setup-help">Lua-модули и blobs, которые подгружаются один раз при запуске и используются стратегиями ниже.</div></div>
      <div class="setup-body"><NfqwsArgumentField id="flowBaseArgs" kind="base" value={flowFields.NFQWS_BASE_ARGS} busy={serviceBusy} large baseArgs={flowFields.NFQWS_BASE_ARGS} knownLists={availableFlowLists} onOpenResource={openFlowResource} onEdit={value=>editFlowFieldUI('NFQWS_BASE_ARGS',value)}/><div class="setup-note warn">В этом блоке upstream запрещает <span class="mono">--new</span>. Профили здесь не переставляем и не дробим.</div></div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS" use:setupAccordion={2}>
      <div class="setup-head"><div class="setup-num">03</div><div><div class="setup-title">HTTPS / TCP</div><div class="setup-key">NFQWS_ARGS</div><div class="setup-head-summary"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS||[]}/></div></div><div class="setup-help">Главная стратегия для HTTPS/TCP. Применяется к доменам в зависимости от выбранного режима работы.</div></div>
      <div class="setup-body"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS||[]}/><NfqwsArgumentField id="flowArgs" value={flowFields.NFQWS_ARGS} busy={serviceBusy} large baseArgs={flowFields.NFQWS_BASE_ARGS} knownLists={availableFlowLists} onOpenResource={openFlowResource} onEdit={value=>editFlowFieldUI('NFQWS_ARGS',value)}/><div class="setup-note">Порядок внутри этого блока сохраняем как в исходном конфиге. <span class="mono">--new</span> здесь не допускается самим nfqws2-keenetic.</div></div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS_QUIC" use:setupAccordion={3}>
      <div class="setup-head"><div class="setup-num">04</div><div><div class="setup-title">QUIC</div><div class="setup-key">NFQWS_ARGS_QUIC</div><div class="setup-head-summary"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS_QUIC||[]}/></div></div><div class="setup-help">Отдельная стратегия для QUIC, обычно UDP/443. При запуске nfqws2 к ней автоматически добавляются режим доменов и IPSET.</div></div>
      <div class="setup-body"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS_QUIC||[]}/><NfqwsArgumentField id="flowQuic" value={flowFields.NFQWS_ARGS_QUIC} busy={serviceBusy} baseArgs={flowFields.NFQWS_BASE_ARGS} knownLists={availableFlowLists} onOpenResource={openFlowResource} onEdit={value=>editFlowFieldUI('NFQWS_ARGS_QUIC',value)}/></div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS_UDP" use:setupAccordion={4}>
      <div class="setup-head"><div class="setup-num">05</div><div><div class="setup-title">Прочий UDP</div><div class="setup-key">NFQWS_ARGS_UDP</div><div class="setup-head-summary"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS_UDP||[]}/></div></div><div class="setup-help">WireGuard, STUN, Discord, MTProto и другие UDP-протоколы. Этот блок не использует NFQWS_EXTRA_ARGS.</div></div>
      <div class="setup-body"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS_UDP||[]}/><NfqwsArgumentField id="flowUdp" value={flowFields.NFQWS_ARGS_UDP} busy={serviceBusy} large baseArgs={flowFields.NFQWS_BASE_ARGS} knownLists={availableFlowLists} onOpenResource={openFlowResource} onEdit={value=>editFlowFieldUI('NFQWS_ARGS_UDP',value)}/><div class="setup-note good">Это отдельный UDP-профиль: доменные MODE_LIST / MODE_AUTO / MODE_ALL на него не навешиваются.</div></div>
    </article>

    <article class="setup-section fixed-section choice-section" data-fixed="true" use:setupAccordion={5}>
      <div class="setup-head"><div class="setup-num">06</div><div><div class="setup-title">Режим обработки доменов</div><div class="setup-key">MODE_LIST / MODE_ALL / MODE_AUTO → NFQWS_EXTRA_ARGS</div></div><div class="setup-help">Сами MODE_* — служебные шаблоны upstream. Мы их не редактируем, только выбираем один из них.</div></div>
      <div class="setup-body">
        <NfqwsFlowModes kind="domain" snapshot={flowModes} onChange={changeFlowMode} busy={serviceBusy}/>
        <div class="setup-note">Это именно <span class="mono">NFQWS_EXTRA_ARGS</span>. Upstream помечает MODE_LIST / MODE_ALL / MODE_AUTO как «don't change» — поэтому они здесь только как понятный переключатель.</div>
        <div class="list-links"><button class="list-link" onclick={()=>openFlowResource('list','user.list')}>Открыть user.list</button><button class="list-link" onclick={()=>openFlowResource('list','auto.list')}>Открыть auto.list</button><button class="list-link" onclick={()=>openFlowResource('list','exclude.list')}>Открыть exclude.list</button></div>
      </div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS_IPSET" use:setupAccordion={6}>
      <div class="setup-head"><div class="setup-num">07</div><div><div class="setup-title">IP/CIDR-фильтры <span class="muted small">· дополнительно</span></div><div class="setup-key">NFQWS_ARGS_IPSET</div><div class="setup-head-summary"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS_IPSET||[]}/></div></div><div class="setup-help">Upstream NFQWS_ARGS_IPSET: отдельные IP/CIDR-правила, независимые от режима доменов. Если они не нужны — этот блок можно просто оставить как есть.</div></div>
      <div class="setup-body"><NfqwsSetupChips items={setupView.chips.NFQWS_ARGS_IPSET||[]}/><NfqwsArgumentField id="flowIpset" value={flowFields.NFQWS_ARGS_IPSET} busy={serviceBusy} baseArgs={flowFields.NFQWS_BASE_ARGS} knownLists={availableFlowLists} onOpenResource={openFlowResource} onEdit={value=>editFlowFieldUI('NFQWS_ARGS_IPSET',value)}/><div class="list-links"><button class="list-link" onclick={()=>openFlowResource('list','ipset.list')}>Открыть ipset.list</button><button class="list-link" onclick={()=>openFlowResource('list','ipset_exclude.list')}>Открыть ipset_exclude.list</button></div></div>
    </article>

    <article class="setup-section" data-summary-key="CUSTOM" use:setupAccordion={7}>
      <div class="setup-head"><div class="setup-num">08</div><div><div class="setup-title">Дополнительные стратегии</div><div class="setup-key">NFQWS_ARGS_CUSTOM</div><div class="setup-head-summary"><span class="setup-chip" class:good={customStrategies.blocks.length>0}>Custom: {customStrategies.blocks.length||'нет'}</span></div></div><div class="setup-help">Единственное место, где upstream разрешает несколько профилей через --new. Только здесь можно менять блоки местами.</div></div>
      <div class="setup-body">
        <NfqwsCustomStrategies snapshot={customStrategies} onChange={updateCustomStrategies} busy={serviceBusy}/>
        <div class="setup-note good">Порядок блоков влияет на итоговую строку запуска. Перемещение ↑ / ↓ доступно только здесь — остальные секции следуют исходной архитектуре nfqws2.</div>
      </div>
    </article>

    <article class="setup-section fixed-section traffic-section" data-fixed="true" use:setupAccordion={8}>
      <div class="setup-head"><div class="setup-num traffic-range">09–11</div><div><div class="setup-title">Обработка трафика</div><div class="setup-key">IPV6_ENABLED / TCP_PORTS / UDP_PORTS</div></div><div class="setup-help">Три компактных параметра, которые определяют, какой трафик попадёт в NFQUEUE.</div></div>
      <div class="setup-body">
        <div class="traffic-rows">
          <div class="traffic-line">
            <span class="traffic-card-num">09</span>
            <div class="traffic-line-copy"><strong>IPv6</strong><span>IPV6_ENABLED</span></div>
            <div class="traffic-line-control"><label class="traffic-switch"><input id="flowIPv6" type="checkbox" checked={/^(1|true|yes)$/i.test(flowFields.IPV6_ENABLED)} disabled={serviceBusy} onchange={e=>editFlowFieldUI('IPV6_ENABLED',e.currentTarget.checked?'1':'0')}><span class="traffic-switch-track"></span><strong>Обрабатывать IPv6</strong></label></div>
          </div>
          <div class="traffic-line">
            <span class="traffic-card-num">10</span>
            <div class="traffic-line-copy"><strong>TCP → NFQUEUE</strong><span>TCP_PORTS</span></div>
            <div class="traffic-line-control"><input id="flowTcpPorts" class="setup-input" spellcheck="false" placeholder="80,443,1984,2053,2083,2087,2096,5222,8443" value={flowFields.TCP_PORTS} disabled={serviceBusy} oninput={e=>editFlowFieldUI('TCP_PORTS',e.currentTarget.value)}></div>
          </div>
          <div class="traffic-line">
            <span class="traffic-card-num">11</span>
            <div class="traffic-line-copy"><strong>UDP → NFQUEUE</strong><span>UDP_PORTS</span></div>
            <div class="traffic-line-control"><input id="flowUdpPorts" class="setup-input" spellcheck="false" placeholder="443,590:600,1400,3478:3481,5349" value={flowFields.UDP_PORTS} disabled={serviceBusy} oninput={e=>editFlowFieldUI('UDP_PORTS',e.currentTarget.value)}></div>
          </div>
        </div>
      </div>
    </article>

    <article class="setup-section fixed-section choice-section" data-fixed="true" use:setupAccordion={9}>
      <div class="setup-head"><div class="setup-num">12</div><div><div class="setup-title">Политика Keenetic</div><div class="setup-key">POLICY_NAME / POLICY_EXCLUDE</div></div><div class="setup-help">Позволяет ограничить обработку устройствами из политики Keenetic или, наоборот, исключить их.</div></div>
      <div class="setup-body">
        <NfqwsFlowModes kind="policy" snapshot={flowModes} onChange={changeFlowMode} name={flowFields.POLICY_NAME} onNameChange={value=>editFlowFieldUI('POLICY_NAME',value)} busy={serviceBusy}/>

      </div>
    </article>

    <article class="setup-section expert-only" use:setupAccordion={10}>
      <div class="setup-head"><div class="setup-num">13</div><div><div class="setup-title">Логирование</div><div class="setup-key">LOG_LEVEL / LOG_DEBUG_PATH</div></div><div class="setup-help">Debug-лог нужен только для диагностики. Постоянно держать его включённым обычно не требуется.</div></div>
      <div class="setup-body form-stack">
        <div class="form-row"><div class="setup-label"><strong>Debug-логирование</strong>Включай только на время диагностики.</div><div class="form-control"><label class="config-check"><input id="flowLogLevel" type="checkbox" checked={/^(1|true|yes)$/i.test(flowFields.LOG_LEVEL)} disabled={serviceBusy} onchange={e=>editFlowFieldUI('LOG_LEVEL',e.currentTarget.checked?'1':'0')}> Включить LOG_LEVEL=1</label></div></div>
        <div class="form-row"><div class="setup-label"><strong>Файл debug-лога</strong>Путь используется при LOG_LEVEL=1.</div><div class="form-control"><input id="flowLogPath" class="setup-input" spellcheck="false" value={flowFields.LOG_DEBUG_PATH} disabled={serviceBusy} oninput={e=>editFlowFieldUI('LOG_DEBUG_PATH',e.currentTarget.value)}></div></div>
      </div>
    </article>

    <article class="setup-section expert-only" use:setupAccordion={11}>
      <div class="setup-head"><div class="setup-num">14</div><div><div class="setup-title">Служебные параметры</div><div class="setup-key">NFQUEUE_NUM / USER / CONFIG_VERSION</div></div><div class="setup-help">Показываем для понимания механизма, но в обычном интерфейсе не даём случайно ломать.</div></div>
      <div class="setup-body"><div class="internal-grid"><div class="internal-item"><span>NFQUEUE_NUM</span><strong>{setupView.internals.queue}</strong></div><div class="internal-item"><span>USER</span><strong>{setupView.internals.user}</strong></div><div class="internal-item"><span>CONFIG_VERSION</span><strong>{setupView.internals.version}</strong></div></div></div>
    </article>

    <article class="setup-section fixed-section diagnostics-section" data-fixed="true" use:setupAccordion={12}>
      <div class="setup-head"><div class="setup-num">✓</div><div><div class="setup-title">Проверка перед применением</div><div class="setup-key">RouterForge diagnostics</div></div><div class="setup-help">Проверяем порты, режимы, ссылки на списки и критические противоречия до записи файла.</div></div>
      <div class="setup-body diagnostics-body">
        <NfqwsSetupDiagnostics value={setupView.diagnostics}/>
      </div>
    </article>
    <div class="setup-sticky"><div><span class="setup-state" class:dirty={setupView.dirty}>{setupView.dirty?'UNSAVED':'CLEAN'}</span><div class="change-summary">
      {#each setupView.changes.slice(0,8) as key}<span class="change-chip">{key}</span>{/each}{#if setupView.changes.length>8}<span class="change-chip">+{setupView.changes.length-8}</span>{/if}
      </div><div class="muted small" style="margin-top:3px">Сохраняется в тот же nfqws2.conf → backup → atomic write → reload → rollback при ошибке.</div></div><div class="toolbar"><button disabled={!setupView.dirty||serviceBusy} onclick={()=>setupAction('reset')}>Отменить</button><button disabled={!setupView.dirty||serviceBusy} onclick={()=>setupAction('preview')}>Diff</button><button class="primary" disabled={!setupView.canApply||serviceBusy} onclick={()=>setupAction('preview')}>Сохранить и применить</button></div></div>
  </div>
</section>

<section id="view-configs" class="view" class:active={activeTab === 'configs'}>
  <NfqwsConfigLibrary active={activeTab === 'configs'} openRequest={configOpenRequest} {service} onCount={value=>configLibraryCount=value} onChanged={payload=>runtime?.configLibraryChanged(payload)} onNotice={value=>message=value}/>
</section>

<section id="view-config" class="view" class:active={activeTab === 'config'}>
  <NfqwsRawConfig active={activeTab === 'config'} snapshot={rawConfigSnapshot} {service} onChanged={status=>{if(status)runtime?.rawConfigChanged(status);else void runtime?.refresh();}} onNotice={value=>message=value}/>
</section>

<section id="view-lists" class="view" class:active={activeTab === 'lists'}>
  <NfqwsListEditor active={activeTab === 'lists'} inventory={listInventory} serviceBusy={serviceBusy} tools={listToolState}
    onRequest={onListRequest} onStatus={onListStatus} onNotice={value=>{message=value;}}
    onDraft={onListDraft} onTool={onListTool} onRegister={controller=>{listController=controller;}} />
</section>

<section id="view-upstream-lists" class="view" class:active={activeTab === 'upstream-lists'}>
  <NfqwsListSources snapshot={listSourceView} onAction={listSourceAction}/>
</section>

<section id="view-geo-assets" class="view" class:active={activeTab === 'geo-assets'}>
  <NfqwsGeoAssets active={activeTab === 'geo-assets'} onRequest={strategyRequest}
    onNotice={(text,type)=>message={text,type}} onListsChanged={()=>runtime?.listsChanged?.()}/>
</section>
<section id="view-catalog" class="view" class:active={activeTab === 'catalog'}>
  <NfqwsCatalog snapshot={catalogView} onAction={catalogAction}/>
</section>

<section id="view-clienthello" class="view" class:active={activeTab === 'clienthello'}>
 <NfqwsClientHello active={activeTab === 'clienthello'} {serviceBusy} onRequest={strategyRequest} onNotice={value=>message=value} onSaved={()=>runtime?.blobsChanged()}/>
</section>

<section id="view-blobs" class="view" class:active={activeTab === 'blobs'}>
  <NfqwsBlobs snapshot={blobView} onAction={blobAction}/>
</section>

<section id="view-runs" class="view" class:active={activeTab === 'runs'}>
  <NfqwsRunHistory snapshot={historyView} onAction={historyAction} guardEnabled={recoveryGuardEnabled}
    guardThreshold={recoveryGuardThreshold} onSettings={historySettingsChanged}/>
</section>

<NfqwsLogs active={activeTab === 'logs'} snapshot={logData} filterRequest={logSearchRequest} onNotice={(text,type) => { message={text,type}; }} />

<NfqwsScripts active={activeTab === 'scripts'} openRequest={scriptOpenRequest} onCount={n=>scriptCount=n} onNotice={(text,type)=>{message={text,type};}} />

<section id="view-keenetic" class="view" class:active={activeTab === 'keenetic'}>
  <NfqwsKeenetic active={activeTab === 'keenetic'} onRequest={strategyRequest}
    onDefault={useKeeneticDefault} dpiInfo={dpiInfo} onDpiRefresh={()=>runtime?.dpiRefresh()}/>
</section>

<section id="view-backups" class="view" class:active={activeTab === 'backups'}>
  <NfqwsBackupCenter snapshot={backupView} onAction={backupAction}/>
</section>
</main>
</div>

<NfqwsSetupDiff snapshot={setupDiff} onAction={setupAction}/>

<NfqwsDomainBatchCheck snapshot={domainCheckView} onAction={domainCheckAction}/>

<NfqwsListTools inspector={listInspection} staticPath={listStaticPath} analysis={listAnalysis} onAction={onListToolsAction}/>

<NfqwsSmartApply view={smartApplyView} onAction={smartApplyAction}/>
