import {nfqwsOverview} from './NfqwsOverviewModel.js';
import {verifiedBlobManifest} from './NfqwsCatalogBlobModel.js';
import {measuredAutoPickEligible,recoveryRegression} from './NfqwsRunHistoryModel.js';
import {domainMode,policyMode as draftPolicyMode,updateModeDraft} from './NfqwsFlowModeModel.js';
import {hydrateFlowFields,mergeFlowFields} from './NfqwsFlowFieldsModel.js';
import {safeResourceName} from './NfqwsArgumentResourcesModel.js';
import {listAnalysisView,removeIntersectingLines} from './NfqwsListToolsModel.js';
import {preflightView} from './NfqwsPreflightModel.js';
// Exact NFQWS business/controller extraction from historical inline script.
// Svelte creates the DOM before this module starts.
// Incremental migration: actions/async CAS/rollback remain unchanged.
export function startNFQWSRuntime({onNavigate = () => {}, onStatus = () => {}, onBusy = () => {}, onNotice = () => {}, onLogs = () => {}, onLogSearch = () => {}, onScriptOpen = () => {}, onOverview = () => {}, onStrategyCount = () => {}, onBackups = () => {}, onListSources = () => {}, onSmartApply = () => {}, onCatalog = () => {}, onBlobs = () => {}, onHistory = () => {}, onDomainCheck = () => {}, onSetup = () => {}, onDiff = () => {}, getRecoverySettings = () => ({enabled:false,threshold:20}), getCustomStrategies = () => ({blocks:[],labels:[]}), onCustomHydrate = () => {}, onFlowModeHydrate = () => {}, getFlowFields = () => ({}), onFlowFieldsHydrate = () => {}, onFlowLists = () => {}, onConfigOpen = () => {}, onRawConfig = () => {}, onListInventory = () => {}, onListTools = () => {}, onListInspection = () => {}, onListStaticPath = () => {}, onListAnalysis = () => {}, onPreflight = () => {}, onDpiStatus = () => {}, getListEditor = () => null, onSandboxSeed = () => {}, onSelectorSeed = () => {}, onAutoSetupSeed = () => {}, onDomainSeed = () => {}, onLabSeed = () => {}, onHeight = () => {}} = {}) {
  // Lifecycle belongs to the mounted Svelte owner, not document globals.
  const lifecycle = new AbortController();
  let disposed = false;
  const marker='/ui/', pos=location.pathname.indexOf(marker);
  const base=pos>=0?location.pathname.slice(0,pos):'/api/modules/nfqws-manager';
  let current=null, original='', busy=false, selectedList='', listOriginal='', listMeta=null, checkGeneration=0, checkAbort=null, policyMode='none', analysisFindings=[], analysisRunning=false, analysisSources={}, analysisHashes={}, analysisSelectedDirty=false, entryInspectorRunning=false, entryInspectorRecord=null, entryInspectorFindings=[], staticPathRunning=false, staticPathRecord=null, staticPathResult=null, runHistory=[], recoveryGuardRunning=false, recoveryGuardLastResult=null, listSourceLoaded=false, listSourceLoading=false, listSourceFiles=[], listSourceUpstream=[], listSourceRef='', listSourceError='', listSourceRoot='', catalogLoaded=false, catalogLoading=false, catalogItems=[], catalogSelected='', catalogSelectedData=null, catalogUpstream={state:'idle',ref:'main',sha:'',error:''}, blobLoaded=false, blobLoading=false, blobFiles=[], blobRoot='', blobManifestCache={}, blobExpectedMap={}, smartApplyPlan=null, smartApplyBusy=false, smartApplyOpen=false, smartApplyPreparing=false, smartApplyError='', smartApplyEpoch=0, backupCenterLoaded=false, backupCenterLoading=false, backupCenterFiles=[], backupCenterSelected='', backupCenterSelectedData=null, backupCenterRoot='', backupCenterError='', backupCenterDetailError='', backupCenterSafetyCount=0, backupCenterDetailLoading=false, backupCenterActionBusy=false, backupCenterSeq=0, dpiDetectorLoaded=false, dpiDetectorLoading=false, dpiDetectorSnapshot=null, dpiDetectorError='';

  let catalogError='', catalogSelectionBusy=false, catalogSelectEpoch=0;
  let blobError='', blobActionBusy=false;
  let configLibraryLoaded=false, configLibraryLoading=false, configLibraryFiles=[];
  let inspectorOpen=false,inspectorProgress='',inspectorError='',inspectorEpoch=0;
  let analysisOpen=false,analysisProgress='',analysisError='',analysisEpoch=0,analysisIncomplete=false,analysisDraftContent='';
  let staticPathOpen=false,staticPathProgress='',staticPathError='',staticPathEpoch=0;
  let preflightBusy=false;
  function emitStaticPath(){if(!disposed)emit(onListStaticPath,{open:staticPathOpen,record:staticPathRecord,result:staticPathResult,
    busy:staticPathRunning,progress:staticPathProgress,error:staticPathError});}
  function emitInspection(){if(!disposed)emit(onListInspection,{open:inspectorOpen,record:entryInspectorRecord,
    findings:entryInspectorFindings.slice(),busy:entryInspectorRunning,progress:inspectorProgress,error:inspectorError});}
  function emitAnalysis(){if(!disposed)emit(onListAnalysis,{open:analysisOpen,selected:selectedList,
    findings:analysisFindings.slice(),busy:analysisRunning,progress:analysisProgress,error:analysisError,dirty:analysisSelectedDirty,incomplete:analysisIncomplete});}


  const esc=(s='')=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const lines=s=>String(s||'').split(/\r?\n/);
  const bytes=n=>{n=Number(n||0);if(n<1024)return `${n} B`;if(n<1048576)return `${(n/1024).toFixed(1)} KB`;return `${(n/1048576).toFixed(1)} MB`};

  async function api(path,options={}){
    if(disposed)throw Error('NFQWS_UI_UNMOUNTED');
    const method = String(options.method || 'GET').toUpperCase();
    // GETs are Svelte lifetime reads. Protected writes continue until the
    // server acknowledges them even if the iframe is unmounted. Explicit
    // signals (the stoppable /check probe) retain their own cancel semantics.
    const signal = options.signal || (method === 'GET' ? lifecycle.signal : undefined);
    const r=await fetch(base+path,{cache:'no-store',credentials:'same-origin',headers:{Accept:'application/json','Content-Type':'application/json'},...options,signal});
    const text=await r.text();let body={},validJson=true;
    try{body=text?JSON.parse(text):{};}catch{validJson=false;}
    if(!r.ok)throw new Error(body?.error||`${path} HTTP ${r.status}`);
    // Blob mutations fail closed on invalid transport; a 200/plain-text cannot be success.
    if(/^\/blobs\/(install|delete)$/.test(path)){
      if(!validJson||!text.trim()||!String(r.headers?.get?.('content-type')||'').toLowerCase().includes('application/json')||!body||typeof body!=='object')
        throw Error('NFQWS_BLOB_MUTATION_INVALID_RESPONSE');
    }
    return body;
  }
  const emit = (callback, ...args) => { if (!disposed) callback(...args); };
  function notice(text,type=''){if(!disposed)emit(onNotice,{text:String(text),type});}
  function setBusy(v){if(disposed)return;busy=v;emit(onBusy,v);updateListButtons()}

  function scanAssignment(config,key){
    const re=new RegExp(`^([\\t ]*)${key}\\s*=\\s*`,'m'),m=re.exec(config);if(!m)return null;
    const start=m.index;let i=m.index+m[0].length,quote='';
    if(config[i]==='"'||config[i]==="'"){quote=config[i];i++}
    const valueStart=i;let escaped=false;
    if(quote){
      for(;i<config.length;i++){const c=config[i];if(c===quote&&!escaped)break;escaped=c==='\\'&&!escaped;if(c!=='\\')escaped=false}
      if(i>=config.length)return null;
      return{key,start,end:i+1,quote,indent:m[1],raw:config.slice(valueStart,i)};
    }
    for(;i<config.length;i++){const c=config[i];if((c==='\n'||c==='\r')&&!escaped)break;escaped=c==='\\'&&!escaped;if(c!=='\\')escaped=false}
    return{key,start,end:i,quote:'',indent:m[1],raw:config.slice(valueStart,i)};
  }
  function normalizeAssignmentRaw(raw){return String(raw||'').replace(/\\\r?\n[ \t]*/g,'\n').split(/\r?\n/).map(line=>line.trim()).join('\n').trim()}
  function assignmentValue(config,key){const a=scanAssignment(config,key);return a?normalizeAssignmentRaw(a.raw).replace(/\\"/g,'"').replace(/\\\\/g,'\\'):''}
  function boolAssignment(config,key){return /^(1|true|yes)$/i.test(assignmentValue(config,key))}
  function escapeForQuote(value,quote){
    if(quote==='"')return String(value).replace(/\\/g,'\\\\').replace(/"/g,'\\"');
    if(quote==="'")return String(value).replace(/'/g,"'\\''");
    return String(value);
  }
  function formatAssignment(key,value,a){
    const quote=a?.quote||((String(value).includes(' ')||String(value).includes('\n')||String(value)==='')?'"':'');
    const indent=a?.indent||'',logical=escapeForQuote(value,quote).replace(/\r\n/g,'\n');
    if(!logical.includes('\n'))return `${indent}${key}=${quote}${logical}${quote}`;
    const continuation=indent+' '.repeat(key.length+2);
    const formatted=logical.split('\n').map((line,i)=>{
      const clean=line.trim();
      if(i===0)return clean;
      return clean?continuation+clean:'';
    }).join('\n');
    return `${indent}${key}=${quote}${formatted}${quote}`;
  }
  function replaceAssignment(config,key,value){
    const a=scanAssignment(config,key);if(!a)return config;
    return config.slice(0,a.start)+formatAssignment(key,value,a)+config.slice(a.end);
  }

  const flowKeys=['ISP_INTERFACE','NFQWS_BASE_ARGS','NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_EXTRA_ARGS','NFQWS_ARGS_IPSET','NFQWS_ARGS_CUSTOM','IPV6_ENABLED','TCP_PORTS','UDP_PORTS','POLICY_NAME','POLICY_EXCLUDE','LOG_LEVEL','LOG_DEBUG_PATH'];
  let flowOriginal='',flowValues={},customOriginalBlocks=[];
  const customState=()=>{
    const snapshot=getCustomStrategies();
    if(!snapshot||!Array.isArray(snapshot.blocks)||!Array.isArray(snapshot.labels)||
      snapshot.blocks.length!==snapshot.labels.length)throw Error('NFQWS_CUSTOM_STATE_INVALID');
    return snapshot;
  };
  const customBlocks=()=>customState().blocks;
  const customLabels=()=>customState().labels;

  function customLabelStoreKey(){const target=String(current?.config_path||current?.target_id||'default');return `routerforge.nfqws.custom-labels.v1:${target}`}
  function normalizeCustomLabel(value){return String(value||'').replace(/[\r\n]+/g,' ').trim().slice(0,120)}
  function parseCustomLabelMetadata(config){
    const labels=[],re=/^[ \t]*# RouterForge: NFQWS_ARGS_CUSTOM label\[(\d+)\]=(.*)$/gm;let m,count=0;
    while((m=re.exec(String(config||'')))!==null){const i=Number(m[1])-1;if(i>=0){labels[i]=normalizeCustomLabel(m[2]);count++}}
    return{labels,count};
  }
  function legacyCustomLabels(){try{const data=JSON.parse(localStorage.getItem(customLabelStoreKey())||'[]');return Array.isArray(data)?data:[]}catch(_){return[]}}
  function clearLegacyCustomLabels(){try{localStorage.removeItem(customLabelStoreKey())}catch(_){}}
  function loadCustomLabels(config,blocks){
    const stored=parseCustomLabelMetadata(config),legacy=stored.count?[]:legacyCustomLabels();
    const labels=stored.count?stored.labels.slice():legacy.slice();
    while(labels.length<blocks.length)labels.push('');
    return labels.slice(0,blocks.length).map(normalizeCustomLabel);
  }
  function replaceCustomLabelMetadata(config){
    const clean=String(config||'').replace(/^[ \t]*# RouterForge: NFQWS_ARGS_CUSTOM label\[\d+\]=.*(?:\r?\n|$)/gm,'');
    const a=scanAssignment(clean,'NFQWS_ARGS_CUSTOM');if(!a)return clean;
    const lineStart=clean.lastIndexOf('\n',Math.max(0,a.start-1))+1;
    const meta=customLabels().map((value,i)=>{const label=normalizeCustomLabel(value);return label?`# RouterForge: NFQWS_ARGS_CUSTOM label[${i+1}]=${label}`:''}).filter(Boolean);
    if(!meta.length)return clean;
    return clean.slice(0,lineStart)+meta.join('\n')+'\n'+clean.slice(lineStart);
  }
  function parseCustom(value){return String(value||'').split(/\s+--new(?:\s+|$)/).map(x=>x.trim()).filter(Boolean)}
  function buildCustom(){return customBlocks().map(x=>x.trim()).filter(Boolean).join('\n--new\n')}
  function customBlocksChanged(){return JSON.stringify(customBlocks().map(x=>x.trim()))!==JSON.stringify(customOriginalBlocks.map(x=>x.trim()))}
  function parsePorts(value){return String(value||'').split(',').map(x=>x.trim()).filter(Boolean)}
  function validPortToken(value){return /^\d{1,5}(?::\d{1,5})?$/.test(value)&&value.split(':').every(x=>Number(x)>=1&&Number(x)<=65535)}
  function currentMode(){return domainMode(flowValues)}
  function renderPolicy(){
    policyMode=draftPolicyMode(flowValues);
    emit(onFlowModeHydrate,{policy:policyMode});
  }
  function setPolicyMode(mode){
    flowValues=updateModeDraft(flowValues,'policy',mode);
    policyMode=mode;
    if(mode==='none')emit(onFlowFieldsHydrate,{POLICY_NAME:''});
    // Do not derive mode again from an empty POLICY_NAME while the user is
    // about to enter it. The diagnostics gate will reject an empty name.
    emit(onFlowModeHydrate,{policy:mode});
  }
  function openListFromSetup(name){
    activateNFQWSTab('lists');
    if((current?.lists||[]).some(x=>x.name===name))selectList(name);else notice(`Список ${name} не найден на роутере.`,'error');
  }
  function processingOverviewData(){
    const mode=currentMode(),policy=policyMode==='none'?'весь трафик':policyMode==='include'?`только ${flowValues.POLICY_NAME||'политика'}`:`кроме ${flowValues.POLICY_NAME||'политики'}`;
    return[['TCP',flowValues.TCP_PORTS||'выкл'],['UDP',flowValues.UDP_PORTS||'выкл'],['Домены',mode==='MODE_LIST'?'user.list':mode==='MODE_ALL'?'всё кроме exclude':mode==='MODE_AUTO'?'user + auto':'не определено'],['IPv6',/^(1|true|yes)$/i.test(flowValues.IPV6_ENABLED||'')?'включён':'выключен'],['Политика',policy]];
  }
  function referencedListNames(){
    const set=new Set(),source=[flowValues.NFQWS_ARGS,flowValues.NFQWS_ARGS_QUIC,flowValues.NFQWS_ARGS_UDP,flowValues.NFQWS_ARGS_IPSET,...customBlocks()].join(' ');
    const re=/--(?:hostlist|hostlist-exclude|ipset|ipset-exclude)=([^\s]+)/g;let m;
    while((m=re.exec(source))!==null)set.add(m[1].split('/').pop().replace(/["']/g,''));
    return [...set];
  }
  function diagnostics(){
    const out=[],known=new Set((current?.lists||[]).map(x=>x.name)),tcp=parsePorts(flowValues.TCP_PORTS),udp=parsePorts(flowValues.UDP_PORTS);
    if(!tcp.length)out.push(['warn','TCP_PORTS пуст — TCP-трафик не будет отправляться в NFQUEUE.']);else if(tcp.some(x=>!validPortToken(x)))out.push(['bad','В TCP_PORTS есть некорректный порт или диапазон.']);else out.push(['good',`TCP: ${tcp.join(', ')}`]);
    if(!udp.length)out.push(['warn','UDP_PORTS пуст — UDP/QUIC не будет обрабатываться.']);else if(udp.some(x=>!validPortToken(x)))out.push(['bad','В UDP_PORTS есть некорректный порт или диапазон.']);else out.push(['good',`UDP: ${udp.join(', ')}`]);
    if(udp.includes('443')&&!String(flowValues.NFQWS_ARGS_QUIC||'').trim())out.push(['warn','UDP 443 включён, но NFQWS_ARGS_QUIC пустой.']);
    for(const [key,val] of [['NFQWS_BASE_ARGS',flowValues.NFQWS_BASE_ARGS],['NFQWS_ARGS',flowValues.NFQWS_ARGS],['NFQWS_ARGS_QUIC',flowValues.NFQWS_ARGS_QUIC],['NFQWS_ARGS_UDP',flowValues.NFQWS_ARGS_UDP],['NFQWS_ARGS_IPSET',flowValues.NFQWS_ARGS_IPSET]])if(/\s--new(?:\s|$)/.test(String(val||'')))out.push(['bad',`${key}: найден --new. Upstream разрешает --new только в NFQWS_ARGS_CUSTOM.`]);
    for(const name of referencedListNames())out.push([known.has(name)?'good':'warn',known.has(name)?`Связанный список ${name} найден.`:`Стратегия ссылается на ${name}, но файл не найден.`]);
    if(policyMode!=='none'&&!String(flowValues.POLICY_NAME||'').trim())out.push(['bad','Выбран режим политики, но POLICY_NAME пуст.']);
    if(currentMode()==='MODE_LIST'&&!known.has('user.list'))out.push(['warn','MODE_LIST выбран, но user.list не найден.']);
    const seenCustom=new Map();customBlocks().forEach((value,i)=>{
      const name=normalizeCustomLabel(customLabels()[i])||`#${i+1}`,raw=String(value||'').trim(),normalized=raw.replace(/\s+/g,' ');
      if(!raw)out.push(['bad',`Custom ${name}: блок пуст.`]);
      if(/(^|\s)--new(?:\s|$)/.test(raw))out.push(['bad',`Custom ${name}: --new внутри одного UI-блока запрещён. Используй отдельные блоки.`]);
      if(normalized){if(seenCustom.has(normalized))out.push(['warn',`Custom ${name}: полный дубль блока ${seenCustom.get(normalized)}.`]);else seenCustom.set(normalized,name)}
    });
    if(!out.some(x=>x[0]==='bad'))out.push(['good','Критических противоречий перед сохранением не найдено.']);
    return out;
  }
  function diagnosticsData(){
    const items=diagnostics(),bad=items.filter(x=>x[0]==='bad').length,warn=items.filter(x=>x[0]==='warn').length,good=items.filter(x=>x[0]==='good').length;
    return {items,bad,warn,good,state:bad?'BLOCKED':warn?'CHECK':'READY',stateClass:bad?'bad':warn?'warn':'good',
      message:bad?`${bad} ошибок`:warn?`${warn} предупреждений`:'можно применять'};
  }
  function changedKeys(){const next=flowConfig(),out=[];for(const key of flowKeys)if(assignmentValue(flowOriginal,key)!==assignmentValue(next,key))out.push(key);const before=parseCustomLabelMetadata(flowOriginal).labels.map(normalizeCustomLabel),after=customLabels().map(normalizeCustomLabel);if(JSON.stringify(before)!==JSON.stringify(after))out.push('CUSTOM_LABELS');return out}
  // Changed keys are supplied directly to the Svelte setup feedback owner.
  function loadFlow(config){
    flowOriginal=config;flowValues=Object.fromEntries(flowKeys.map(k=>[k,assignmentValue(config,k)]));const blocks=parseCustom(flowValues.NFQWS_ARGS_CUSTOM);customOriginalBlocks=[...blocks];emit(onCustomHydrate,{blocks,labels:loadCustomLabels(config,blocks)});
  }
  function flowConfig(){
    let cfg=flowOriginal;
    const vals={...flowValues,NFQWS_ARGS_CUSTOM:customBlocksChanged()?buildCustom():flowValues.NFQWS_ARGS_CUSTOM};
    for(const key of flowKeys){
      const next=String(vals[key]??'');
      if(assignmentValue(flowOriginal,key)!==next)cfg=replaceAssignment(cfg,key,next);
    }
    cfg=replaceCustomLabelMetadata(cfg);
    return cfg;
  }
  function flowDirty(){return flowConfig()!==flowOriginal}
  function summaryChipItems(value){
    const tokens=String(value||'').match(/(?:[^\s"']+|"[^"]*"|'[^']*')+/g)||[],result=[];
    const take=(name,label,cls='')=>{const token=tokens.find(x=>x.startsWith(`--${name}=`));if(token)result.push({label:`${label} ${token.slice(name.length+3)}`,cls})};
    take('filter-tcp','TCP','good');take('filter-udp','UDP','good');take('filter-l7','L7');take('payload','payload');take('lua-desync','lua','warn');take('dpi-desync','desync','warn');take('hostlist','hostlist');take('ipset','ipset');
    return result.length?result:[{label:'Без краткого summary',cls:''}];
  }
  function updateModeCards(){
    emit(onFlowModeHydrate,{domain:currentMode()});
  }
  function updateFlowState(){
    // No imperative HTML or disabled-state writes; Svelte is the UI authority.
    flowValues=mergeFlowFields(flowValues,getFlowFields(),policyMode);
    const dirty=flowDirty(),diag=diagnosticsData(),changes=changedKeys();
    emit(onSetup,{dirty,changes,diagnostics:diag,overview:processingOverviewData(),
      chips:{NFQWS_ARGS:summaryChipItems(flowValues.NFQWS_ARGS),NFQWS_ARGS_QUIC:summaryChipItems(flowValues.NFQWS_ARGS_QUIC),
        NFQWS_ARGS_UDP:summaryChipItems(flowValues.NFQWS_ARGS_UDP),NFQWS_ARGS_IPSET:summaryChipItems(flowValues.NFQWS_ARGS_IPSET)},
      internals:{queue:assignmentValue(flowOriginal,'NFQUEUE_NUM')||'—',user:assignmentValue(flowOriginal,'USER')||'—',version:assignmentValue(flowOriginal,'CONFIG_VERSION')||'—'},
      canApply:dirty&&!busy&&diag.bad===0});
    emit(onStrategyCount,3+customBlocks().length);
    renderPreflight();
  }
  function renderFlow(){
    emit(onFlowFieldsHydrate,hydrateFlowFields(flowValues));
    updateModeCards();renderPolicy();updateFlowState();emit(onHeight,);
  }
  function resetFlow(){loadFlow(flowOriginal);renderFlow()}
  let diffOpen=false;
  function emitDiff(){if(!disposed)emit(onDiff,{open:diffOpen,lines:diffOpen?makeDiff(flowOriginal,flowConfig()):[]})}
  function previewFlow(){updateFlowState();if(!flowDirty()||busy)return;diffOpen=true;emitDiff()}
  function setupAction(name){
    if(disposed)return;
    if(name==='reset')return resetFlow();
    if(name==='preview')return previewFlow();
    if(name==='apply')return applyFlow();
    if(name==='close')return closeDiff();
    throw Error('NFQWS_SETUP_COMMAND_UNSUPPORTED');
  }
  async function applyFlow(){
    updateFlowState();if(!flowDirty()||busy)return;const next=flowConfig();
    if(!confirm('Сохранить настройки nfqws2.conf и выполнить безопасный reload?'))return;
    closeDiff();setBusy(true);
    try{const r=await api('/config',{method:'POST',body:JSON.stringify({content:next,confirm:'NFQWS_CONFIG'})});clearLegacyCustomLabels();render(r.status);notice(`Настройки применены. Backup: ${r.backup}`,'good')}
    catch(e){notice(e.message,'error');setBusy(false)}
  }

  function preflightItem(group,severity,title,detail,tab=''){
    return{group,severity,title,detail,tab};
  }
  function buildPreflight(){
    const out=[],known=new Set((current?.lists||[]).map(x=>String(x.name||''))),mode=currentMode(),tcp=parsePorts(flowValues.TCP_PORTS),udp=parsePorts(flowValues.UDP_PORTS),refs=referencedListNames();
    const add=(g,s,t,d,tab='')=>out.push(preflightItem(g,s,t,d,tab));
    add('Runtime',current?.available?'good':'bad',current?.available?'nfqws2 target обнаружен':'nfqws2 target не найден',current?.available?`${current?.target_name||'nfqws2'} · ${current?.init_script||'init path unknown'}`:'Нужны init script и canonical config.','overview');
    add('Runtime',current?.running?'good':'warn',current?.running?'Сервис запущен':'Сервис остановлен',current?.running?'Процесс nfqws2 найден в /proc.':'Конфиг можно анализировать, но обработка трафика сейчас не подтверждена.','overview');
    add('Runtime',current?.mutation_ready?'good':'bad',current?.mutation_ready?'Safety mutation boundary готов':'Mutation boundary не готов',current?.mutation_ready?'Init script исполняемый; доступны backup + atomic write + guarded action.':'RouterForge не будет безопасно применять изменения при текущем состоянии.','overview');
    add('Config',current?.config_exists?'good':'bad',current?.config_exists?'Canonical config читается':'Canonical config отсутствует',`${current?.config_path||'/opt/etc/nfqws2/nfqws2.conf'} · ${bytes(current?.config_size)}`,'config');
    add('Config',current?.config_truncated?'bad':'good',current?.config_truncated?'Config превышает лимит анализа':'Config загружен полностью',current?.config_truncated?'Статический анализ неполон — редактор получил усечённый config.':`SHA256 ${(current?.config_sha256||'—').slice(0,20)}…`,'config');
    const dirty=flowDirty();
    add('Config',dirty?'warn':'good',dirty?'Есть несохранённый setup draft':'Setup совпадает с active config',dirty?'Preflight ниже учитывает текущий draft из вкладки «Настройка».':'Проверяется активный canonical config.','strategies');
    if(['MODE_LIST','MODE_AUTO','MODE_ALL'].includes(mode))add('Config','good',`Режим ${mode} распознан`,mode==='MODE_LIST'?'Статическая выборка опирается на user.list.':mode==='MODE_AUTO'?'Статическая выборка использует user.list + auto.list.':'Базовый режим — всё, с исключениями по exclude.','strategies');
    else add('Config','bad','Режим доменов не распознан',`NFQWS_EXTRA_ARGS → ${mode||'пусто'}. Ожидался MODE_LIST / MODE_AUTO / MODE_ALL.`,'strategies');
    const iface=String(flowValues.ISP_INTERFACE||'').trim();
    add('Traffic',iface?'good':'warn',iface?'ISP_INTERFACE задан':'ISP_INTERFACE пуст',iface||'Проверь WAN-интерфейс перед применением.','strategies');
    const queue=assignmentValue(flowOriginal,'NFQUEUE_NUM');
    add('Traffic',queue?'good':'warn',queue?'NFQUEUE_NUM задан':'NFQUEUE_NUM не найден',queue?`queue ${queue}`:'Preflight не может подтвердить номер очереди из canonical config.','strategies');
    const user=assignmentValue(flowOriginal,'USER');
    add('Traffic',user?'good':'warn',user?'Runtime USER задан':'USER не найден',user||'Проверь пользователя запуска nfqws2.','strategies');
    if(!tcp.length)add('Traffic','warn','TCP_PORTS пуст','TCP-трафик не будет отправляться в NFQUEUE.','strategies');
    else add('Traffic',tcp.some(x=>!validPortToken(x))?'bad':'good',tcp.some(x=>!validPortToken(x))?'TCP_PORTS содержит ошибку':'TCP ports валидны',tcp.join(', '),'strategies');
    if(!udp.length)add('Traffic','warn','UDP_PORTS пуст','UDP/QUIC не будет отправляться в NFQUEUE.','strategies');
    else add('Traffic',udp.some(x=>!validPortToken(x))?'bad':'good',udp.some(x=>!validPortToken(x))?'UDP_PORTS содержит ошибку':'UDP ports валидны',udp.join(', '),'strategies');
    if(udp.includes('443')&&!String(flowValues.NFQWS_ARGS_QUIC||'').trim())add('Traffic','warn','UDP/443 есть, QUIC-стратегия пустая','UDP 443 включён, но NFQWS_ARGS_QUIC не задаёт обработку.','strategies');
    const strategyValues=[['NFQWS_BASE_ARGS',flowValues.NFQWS_BASE_ARGS],['NFQWS_ARGS',flowValues.NFQWS_ARGS],['NFQWS_ARGS_QUIC',flowValues.NFQWS_ARGS_QUIC],['NFQWS_ARGS_UDP',flowValues.NFQWS_ARGS_UDP],['NFQWS_ARGS_IPSET',flowValues.NFQWS_ARGS_IPSET]];
    for(const [key,val] of strategyValues)if(/\s--new(?:\s|$)/.test(String(val||'')))add('Traffic','bad',`${key}: запрещён --new`,'Upstream допускает --new только между NFQWS_ARGS_CUSTOM блоками.','strategies');
    const anyStrategy=[flowValues.NFQWS_ARGS,flowValues.NFQWS_ARGS_QUIC,flowValues.NFQWS_ARGS_UDP,flowValues.NFQWS_ARGS_IPSET,...customBlocks()].some(x=>String(x||'').trim());
    add('Traffic',anyStrategy?'good':'bad',anyStrategy?'Есть активные strategy args':'Все strategy args пусты',anyStrategy?`${3+customBlocks().length} UI strategy slots; пустые блоки могут быть допустимы по отдельности.`:'Нет ни одной стратегии, которую можно применить к трафику.','strategies');
    if(policyMode!=='none'&&!String(flowValues.POLICY_NAME||'').trim())add('Traffic','bad','Policy mode без POLICY_NAME','Выбран policy include/exclude, но имя политики пустое.','strategies');
    if(mode==='MODE_LIST')add('Lists',known.has('user.list')?'good':'bad',known.has('user.list')?'user.list найден':'user.list отсутствует','MODE_LIST требует user.list для основной выборки.','lists');
    if(mode==='MODE_AUTO'){
      add('Lists',known.has('user.list')?'good':'bad',known.has('user.list')?'user.list найден':'user.list отсутствует','MODE_AUTO использует пользовательский список.','lists');
      add('Lists',known.has('auto.list')?'good':'warn',known.has('auto.list')?'auto.list найден':'auto.list отсутствует','Автоматически накопленные домены не будут доступны.','lists');
    }
    if(mode==='MODE_ALL')add('Lists',known.has('exclude.list')?'good':'info',known.has('exclude.list')?'exclude.list найден':'exclude.list отсутствует','MODE_ALL работает и без exclude, но исключений не будет.','lists');
    for(const name of refs)add('Lists',known.has(name)?'good':'bad',known.has(name)?`Ссылка ${name} разрешена`:`Ссылка ${name} не найдена`,known.has(name)?'Файл присутствует в runtime inventory.':'Strategy args ссылаются на отсутствующий list-файл.','lists');
    const listDirty=!!getListEditor()?.readContext?.()?.dirty;
    if(listDirty)add('Lists','warn','Есть несохранённый list draft',`${selectedList}: preflight inventory отражает файл на роутере, а не этот draft.`,'lists');
    const logEnabled=/^(1|true|yes)$/i.test(flowValues.LOG_LEVEL||'');
    if(logEnabled)add('Observability',String(current?.log_tail||'').trim()?'good':'warn',String(current?.log_tail||'').trim()?'Логирование даёт события':'Логирование включено, tail пуст',current?.log_path||flowValues.LOG_DEBUG_PATH||'/opt/var/log/nfqws2.log','logs');
    else add('Observability','info','Debug logging выключен','Это нормально для production; live-причины будет сложнее разбирать без временного логирования.','logs');
    add('Observability',current?.log_truncated?'warn':'good',current?.log_truncated?'Log tail усечён':'Log tail в лимите',current?.log_truncated?'UI показывает только хвост файла.':'Нет признака усечения текущего tail.','logs');
    add('Safety','good','Safety backup policy активна',`${current?.backup_count||0} / 8 текущих safety backup.`,'backups');
    add('Safety','info','Preflight read-only','Эта вкладка ничего не применяет, не перезапускает и не удаляет.','');
    return out;
  }
  function renderPreflight(){
    if(disposed)return;
    const rows=buildPreflight();
    emit(onPreflight,preflightView(rows,{dirty:flowDirty(),available:!!current,busy:preflightBusy}));
  }
  async function refreshPreflightRuntime(){
    if(disposed||preflightBusy)return;preflightBusy=true;renderPreflight();
    try{const s=await api('/status');if(disposed)return;current=s;renderPreflight();notice('Preflight runtime snapshot обновлён.','good');}
    catch(e){if(!disposed)notice(e.message,'error');}
    finally{preflightBusy=false;renderPreflight();}
  }
    function renderOverview(){
      // Svelte projects read-only runtime metrics; no innerHTML or DOM mutation.
      emit(onOverview,nfqwsOverview(current));
    }
  function editableStatusLists(){
    return (current?.lists||[]).filter(item=>/\.(list|list-opkg|list-old)$/i.test(String(item?.name||'')));
  }
  // List inventory and editor are owned by NfqwsListEditor.svelte. This controller
  // keeps only a read-only projection for the existing analytic tools.
  async function selectList(name,skipPrompt=false){
    const editor=getListEditor();return editor?.select?.(name,skipPrompt)??false;
  }
  function updateListButtons(){
    emit(onListTools,{inspect:!!listMeta&&!entryInspectorRunning&&!!entryRecordAtCaret(),path:!!listMeta&&!staticPathRunning&&!!entryRecordAtCaret(),analysis:!!listMeta&&!analysisRunning});
  }
  function normalizeListDataLine(raw){
    let text=String(raw??'').trim();if(!text||text.startsWith('#')||text.startsWith('//'))return'';
    const hash=text.indexOf('#');if(hash>0)text=text.slice(0,hash).trim();return text;
  }
  function parseIPv4(value){const p=String(value).split('.');if(p.length!==4)return null;let n=0;for(const x of p){if(!/^\d+$/.test(x))return null;const v=Number(x);if(v<0||v>255)return null;n=n*256+v}return BigInt(n)}
  function parseIPv6(value){
    let text=String(value).toLowerCase();if(text.includes('%'))text=text.split('%')[0];
    if(text.includes('.')){const cut=text.lastIndexOf(':');if(cut<0)return null;const v4=parseIPv4(text.slice(cut+1));if(v4===null)return null;text=text.slice(0,cut)+':'+Number((v4>>16n)&65535n).toString(16)+':'+Number(v4&65535n).toString(16)}
    if((text.match(/::/g)||[]).length>1)return null;let parts=[];if(text.includes('::')){const [a,b]=text.split('::'),left=a?a.split(':'):[],right=b?b.split(':'):[],fill=8-left.length-right.length;if(fill<1)return null;parts=[...left,...Array(fill).fill('0'),...right]}else parts=text.split(':');if(parts.length!==8)return null;
    let out=0n;for(const part of parts){if(!/^[0-9a-f]{1,4}$/.test(part||'0'))return null;out=(out<<16n)+BigInt(parseInt(part||'0',16))}return out;
  }
  function parseAddressEntry(value){
    const raw=String(value).trim(),slash=raw.indexOf('/'),host=slash>=0?raw.slice(0,slash):raw,prefixText=slash>=0?raw.slice(slash+1):'';
    const family=host.includes(':')?6:4,bits=family===6?128:32,ip=family===6?parseIPv6(host):parseIPv4(host);if(ip===null)return null;
    const prefix=slash>=0&&/^\d+$/.test(prefixText)?Number(prefixText):bits;if(prefix<0||prefix>bits)return null;
    const shift=BigInt(bits-prefix),network=shift===0n?ip:(ip>>shift)<<shift,end=shift===0n?network:network+((1n<<shift)-1n);
    return{kind:prefix===bits?'ip':'cidr',family,bits,ip,prefix,network,end,raw};
  }
  function normalizeDomain(value){let v=String(value).trim().toLowerCase();if(v.endsWith('.'))v=v.slice(0,-1);if(/^https?:\/\//.test(v)){try{v=new URL(v).hostname.toLowerCase()}catch(_){return''}}if(v.includes('/'))return'';if(v.startsWith('*.'))v=v.slice(2);return /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9-]{2,63}$/.test(v)?v:''}
  function listRole(name){const n=String(name||'').toLowerCase();return n.includes('exclude')?'exclude':'include'}
  function conflictSeverity(a,b){return listRole(a)!==listRole(b)?'conflict':'warn'}
  function findingKey(f){return[f.kind,f.a.file,f.a.line,f.b.file,f.b.line].join('|')}
  function buildListRecords(name,content){
    const out=[];String(content||'').split(/\r?\n/).forEach((raw,i)=>{const value=normalizeListDataLine(raw);if(!value)return;const net=parseAddressEntry(value);if(net){out.push({type:net.kind,file:name,line:i+1,value,net});return}const domain=normalizeDomain(value);if(domain)out.push({type:'domain',file:name,line:i+1,value,domain})});return out;
  }
  function lowerBound(items,key,project){let lo=0,hi=items.length;while(lo<hi){const mid=(lo+hi)>>1;if(project(items[mid])<key)lo=mid+1;else hi=mid}return lo}
  function pushFinding(findings,seen,kind,detail,a,b,scope){const f={kind,detail,a,b,severity:conflictSeverity(a.file,b.file),scope},key=findingKey(f);if(seen.has(key))return;seen.add(key);findings.push(f)}
  function compareDomainLists(selected,other,findings,seen){
    const exact=new Map(),reversed=[];for(const b of other){if(b.type!=='domain')continue;if(!exact.has(b.domain))exact.set(b.domain,[]);exact.get(b.domain).push(b);reversed.push({key:b.domain.split('.').reverse().join('.'),record:b})}reversed.sort((a,b)=>a.key.localeCompare(b.key));
    for(const a of selected){if(a.type!=='domain')continue;for(const b of exact.get(a.domain)||[])pushFinding(findings,seen,'EXACT_DOMAIN',a.domain,a,b,'domain');
      const labels=a.domain.split('.');for(let i=1;i<labels.length-1;i++){const parent=labels.slice(i).join('.');for(const b of exact.get(parent)||[])pushFinding(findings,seen,'DOMAIN_CHILD',`${a.domain} ⊂ ${b.domain}`,a,b,'domain')}
      const prefix=a.domain.split('.').reverse().join('.')+'.',from=lowerBound(reversed,prefix,x=>x.key);for(let i=from;i<reversed.length&&reversed[i].key.startsWith(prefix);i++){const b=reversed[i].record;pushFinding(findings,seen,'DOMAIN_PARENT',`${a.domain} ⊃ ${b.domain}`,a,b,'domain')}
    }
  }
  function compareNetworkLists(selected,other,findings,seen){
    for(const family of [4,6]){const A=selected.filter(x=>x.type!=='domain'&&x.net.family===family).sort((a,b)=>a.net.network<b.net.network?-1:a.net.network>b.net.network?1:a.net.end<b.net.end?-1:1),B=other.filter(x=>x.type!=='domain'&&x.net.family===family).sort((a,b)=>a.net.network<b.net.network?-1:a.net.network>b.net.network?1:a.net.end<b.net.end?-1:1);let j=0,active=[];
      for(const a of A){while(j<B.length&&B[j].net.network<=a.net.end)active.push(B[j++]);active=active.filter(b=>b.net.end>=a.net.network);for(const b of active){const an=a.net,bn=b.net;if(an.network>bn.end||bn.network>an.end)continue;let kind='',detail='';
          if(an.network===bn.network&&an.end===bn.end){kind=an.kind==='ip'&&bn.kind==='ip'?'EXACT_IP':'EXACT_CIDR';detail=a.value}
          else if(an.kind==='ip'&&bn.kind==='cidr'){kind='IP_IN_CIDR';detail=`${a.value} ∈ ${b.value}`}
          else if(bn.kind==='ip'&&an.kind==='cidr'){kind='IP_IN_CIDR';detail=`${b.value} ∈ ${a.value}`}
          else if(an.network<=bn.network&&an.end>=bn.end){kind='CIDR_CONTAINS';detail=`${a.value} ⊃ ${b.value}`}
          else if(bn.network<=an.network&&bn.end>=an.end){kind='CIDR_CONTAINS';detail=`${b.value} ⊃ ${a.value}`}
          else{kind='CIDR_OVERLAP';detail=`${a.value} ↔ ${b.value}`}
          pushFinding(findings,seen,kind,detail,a,b,'network')
      }}
    }
  }
  function analyzeCrossLists(all,selectedName){
    const findings=[],seen=new Set(),selected=buildListRecords(selectedName,all[selectedName]||'');for(const[name,content]of Object.entries(all)){if(name===selectedName)continue;const other=buildListRecords(name,content);compareDomainLists(selected,other,findings,seen);compareNetworkLists(selected,other,findings,seen)}
    const rank={conflict:0,warn:1};findings.sort((a,b)=>rank[a.severity]-rank[b.severity]||a.scope.localeCompare(b.scope)||a.b.file.localeCompare(b.b.file)||a.a.line-b.a.line);return findings;
  }
  function entryRecordAtCaret(){
    const context=getListEditor()?.readContext?.();
    if(!context?.selected||!context.meta)return null;
    const line=Number(context.caretLine),raw=String(context.value||'').split(/\r?\n/)[line-1]??'';
    const value=normalizeListDataLine(raw);if(!value||!Number.isSafeInteger(line)||line<1)return null;
    const net=parseAddressEntry(value);if(net)return{type:net.kind,file:context.selected,line,value,net};
    const domain=normalizeDomain(value);return domain?{type:'domain',file:context.selected,line,value,domain}:null;
  }
  function inspectEntryAgainstAll(record,all){
    const findings=[],seen=new Set();for(const[name,content]of Object.entries(all)){if(name===record.file)continue;const other=buildListRecords(name,content);if(record.type==='domain')compareDomainLists([record],other,findings,seen);else compareNetworkLists([record],other,findings,seen)}
    const rank={conflict:0,warn:1};findings.sort((a,b)=>rank[a.severity]-rank[b.severity]||a.scope.localeCompare(b.scope)||a.b.file.localeCompare(b.b.file)||a.b.line-b.b.line);return findings;
  }
  async function runEntryInspector(){
    const record=entryRecordAtCaret();if(entryInspectorRunning||!record){updateListButtons();if(!record)notice('Поставь курсор на строку с доменом, IP или CIDR.','error');return;}
    const epoch=++inspectorEpoch;entryInspectorRunning=true;inspectorOpen=true;
    entryInspectorRecord=record;entryInspectorFindings=[];inspectorError='';inspectorProgress='Читаю соседние списки…';emitInspection();updateListButtons();
    try{const loaded=await loadNeighborListContents('inspection');if(disposed||epoch!==inspectorEpoch)return;
      entryInspectorFindings=inspectEntryAgainstAll(record,loaded.contents);
      inspectorProgress=`Проверено ${Object.keys(loaded.contents).length} списков${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;}
    catch(e){if(!disposed&&epoch===inspectorEpoch){inspectorError=String(e.message||e);inspectorProgress='Ошибка инспектора';}}
    finally{if(!disposed&&epoch===inspectorEpoch){entryInspectorRunning=false;emitInspection();updateListButtons();}}
  }
  function closeEntryInspector(){inspectorOpen=false;++inspectorEpoch;entryInspectorRunning=false;emitInspection();updateListButtons();}

  function activeStaticMode(){
    const raw=String(assignmentValue(current?.config||original,'NFQWS_EXTRA_ARGS')||'').trim();return raw.replace(/^\$/,'').replace(/[{}]/g,'').trim()||'UNKNOWN';
  }
  function activeStaticRefs(){
    const src=String(current?.config||original||''),roles=new Map(),re=/--(hostlist|hostlist-exclude|ipset|ipset-exclude)=([^\s"']+)/g;let m;
    const add=(name,role)=>{name=String(name||'').split('/').pop();if(!name)return;if(!roles.has(name))roles.set(name,new Set());roles.get(name).add(role)};
    while((m=re.exec(src))!==null)add(m[2],m[1].includes('exclude')?'exclude':m[1].startsWith('ipset')?'ipset':'include');
    return roles;
  }
  function staticRoleForFile(name,refs){
    const n=String(name||''),lower=n.toLowerCase(),set=refs.get(n)||refs.get(n.split('/').pop());
    if(set&&set.size){const hasExclude=set.has('exclude'),hasInclude=set.has('include')||set.has('ipset');if(hasExclude&&hasInclude)return{kind:'mixed',impact:'neutral',label:'MIXED REF',reason:'config references this file as include and exclude'};if(hasExclude)return{kind:'exclude',impact:'exclude',label:'EXCLUDE',reason:'referenced by --hostlist-exclude/--ipset-exclude'};if(set.has('ipset'))return{kind:'ipset',impact:'include',label:'IPSET',reason:'referenced by --ipset'};return{kind:'include',impact:'include',label:'INCLUDE',reason:'referenced by --hostlist'};}
    if(lower==='exclude.list'||lower==='ipset_exclude.list'||lower.includes('exclude'))return{kind:'exclude',impact:'exclude',label:'EXCLUDE',reason:'exclude list by filename'};
    if(lower==='user.list')return{kind:'user',impact:'include',label:'USER',reason:'upstream user.list'};
    if(lower==='auto.list')return{kind:'auto',impact:'include',label:'AUTO',reason:'upstream auto.list'};
    if(lower==='ipset.list')return{kind:'ipset',impact:'include',label:'IPSET',reason:'upstream ipset.list'};
    return{kind:'unknown',impact:'neutral',label:'OTHER',reason:'not statically classified by active config'};
  }
  function staticRoleEffective(role,mode){
    if(role.impact==='exclude')return'exclude';if(role.impact!=='include')return'neutral';
    if(role.kind==='auto'&&mode!=='MODE_AUTO')return'neutral';
    if(role.kind==='user'&&!['MODE_LIST','MODE_AUTO'].includes(mode))return'neutral';
    return'include';
  }
  function staticMatchesForRecord(record,all){
    const findings=[],seen=new Set();for(const[name,content]of Object.entries(all)){const other=buildListRecords(name,content);if(record.type==='domain')compareDomainLists([record],other,findings,seen);else compareNetworkLists([record],other,findings,seen)}
    return findings.filter(f=>!(f.b.file===record.file&&f.b.line===record.line)).sort((a,b)=>a.b.file.localeCompare(b.b.file)||a.b.line-b.b.line);
  }
  function buildStaticPath(record,all){
    const mode=activeStaticMode(),refs=activeStaticRefs(),sourceRole=staticRoleForFile(record.file,refs),hits=[{file:record.file,line:record.line,value:record.value,relation:'SOURCE',role:sourceRole,effective:staticRoleEffective(sourceRole,mode)}];
    for(const f of staticMatchesForRecord(record,all)){const role=staticRoleForFile(f.b.file,refs);hits.push({file:f.b.file,line:f.b.line,value:f.b.value,relation:f.kind,detail:f.detail,role,effective:staticRoleEffective(role,mode)})}
    const includes=hits.filter(x=>x.effective==='include'),excludes=hits.filter(x=>x.effective==='exclude');let verdict='NO STATIC MATCH',tone='neutral',reason='Активный конфиг не даёт однозначного статического include/exclude для этой записи.';
    if(includes.length&&excludes.length){verdict='AMBIGUOUS';tone='warn';reason=`Есть одновременно include (${includes.length}) и exclude (${excludes.length}) совпадения. Порядок runtime-обработки здесь не угадываем.`}
    else if(excludes.length){verdict='EXCLUDED';tone='exclude';reason=`Найдено ${excludes.length} активных статических exclude-совпадений.`}
    else if(includes.length){verdict='INCLUDED';tone='include';reason=`Найдено ${includes.length} активных статических include-совпадений.`}
    else if(mode==='MODE_ALL'){verdict='INCLUDED BY MODE';tone='include';reason='MODE_ALL включает трафик по умолчанию, активного статического exclude-совпадения не найдено.'}
    return{mode,refs,sourceRole,hits,includes,excludes,verdict,tone,reason};
  }
  function renderStaticPath(){emitStaticPath();}
  async function loadStaticPathContents(){
    const context=getListEditor()?.readContext?.();
    if(!context?.meta||context.selected!==selectedList)throw Error('NFQWS_STATIC_PATH_EDITOR_STALE');
    const items=editableStatusLists().filter(item=>item.name!==selectedList),out={[selectedList]:String(context.value||'')},queue=[...items],workers=[],errors=[];let done=0;staticPathProgress=`Читаю списки: 0/${items.length}`;emitStaticPath();
    async function worker(){while(queue.length){const item=queue.shift();if(!item)break;try{const d=await api(`/list?name=${encodeURIComponent(item.name)}`);out[item.name]=d.content||''}catch(e){errors.push(`${item.name}: ${e.message||'не удалось прочитать'}`)}done++;staticPathProgress=`Читаю списки: ${done}/${items.length}`;emitStaticPath()}}
    for(let i=0;i<Math.min(4,items.length);i++)workers.push(worker());await Promise.all(workers);return{contents:out,errors};
  }
  async function runStaticPath(){
    const record=entryRecordAtCaret();if(staticPathRunning||!record){updateListButtons();if(!record)notice('Поставь курсор на строку с доменом, IP или CIDR.','error');return;}
    const epoch=++staticPathEpoch;staticPathRunning=true;staticPathOpen=true;staticPathRecord=record;staticPathResult=null;
    staticPathError='';staticPathProgress='Сопоставляю active config и list-файлы…';emitStaticPath();updateListButtons();
    try{const loaded=await loadStaticPathContents();if(disposed||epoch!==staticPathEpoch)return;
      staticPathResult=buildStaticPath(record,loaded.contents);
      staticPathProgress=`Проверено ${Object.keys(loaded.contents).length} списков${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;
      if(loaded.errors.length){staticPathResult={...staticPathResult,verdict:'PARTIAL',tone:'warn',reason:`Не прочитаны ${loaded.errors.length} списков: полноценный static verdict недоступен.`};}
      renderStaticPath();
    }catch(e){if(!disposed&&epoch===staticPathEpoch){staticPathError=String(e.message||e);staticPathProgress='Ошибка статического анализа';}}
    finally{if(!disposed&&epoch===staticPathEpoch){staticPathRunning=false;emitStaticPath();updateListButtons();}}
  }
  function closeStaticPath(){staticPathOpen=false;++staticPathEpoch;staticPathRunning=false;emitStaticPath();updateListButtons();}
  async function loadNeighborListContents(purpose='analysis'){
    const context=getListEditor()?.readContext?.();
    if(!context?.selected||!context.meta)throw Error('NFQWS_LIST_NO_ACTIVE_EDITOR');
    const items=editableStatusLists().filter(item=>item.name!==context.selected);
    const out={[context.selected]:String(context.value||'')},hashes={[context.selected]:String(context.meta.sha256||'')};
    const queue=[...items],workers=[],errors=[];let done=0;
    const update=()=>{const text=`Загрузка соседних списков: ${done}/${items.length}`;
      if(purpose==='inspection'){inspectorProgress=text;emitInspection();}
      else{analysisProgress=text;emitAnalysis();}};
    update();
    async function worker(){while(queue.length&&!disposed){const item=queue.shift();if(!item)break;
      try{const d=await api(`/list?name=${encodeURIComponent(item.name)}`);out[item.name]=d.content||'';hashes[item.name]=String(d.sha256||'');}
      catch(e){errors.push(`${item.name}: ${e.message||'не удалось прочитать'}`);}done++;update();}}
    for(let i=0;i<Math.min(4,items.length);i++)workers.push(worker());
    await Promise.all(workers);return{contents:out,hashes,errors};
  }
  function renderAnalysis(){
    emitAnalysis();
  }
  async function openListAtLine(name,line){
    const controller=getListEditor();
    const ok=await controller?.openAtLine?.(name,line);
    if(ok){closeEntryInspector();closeStaticPath();closeAnalysis();}
    return!!ok;
  }
  async function cleanupAnalysisSide(side,peer){
    const projection=listAnalysisView({selected:selectedList,findings:analysisFindings,peer,filter:'all'});
    if(!['left','right'].includes(side)||!peer||projection.peer!==peer||!projection.pairTotal||analysisRunning)return;
    const target=side==='left'?selectedList:peer;
    const context=getListEditor()?.readContext?.();
    if(side==='left'&&(analysisSelectedDirty||context?.dirty)){notice(`Сначала сохрани или отмени изменения в ${selectedList}.`,'error');return;}
    if(context?.selected!==selectedList||!context.meta||context.value!==analysisDraftContent||analysisIncomplete){notice('Черновик или inventory изменились либо загрузка была неполной. Пересчитай анализ.','error');return;}
    const records=side==='left'?projection.allLeft:projection.allRight,source=analysisSources[target],expected=analysisHashes[target];
    if(!records.length||typeof source!=='string'||!/^[a-f0-9]{64}$/i.test(String(expected||''))){notice('Исходные данные или SHA устарели. Пересчитай анализ.','error');return;}
    let next;try{next=removeIntersectingLines(source,records.map(x=>x.line));}catch(e){notice(String(e.message||e),'error');return;}
    if(next===source)return;
    if(!confirm(`Удалить ${records.length} пересекающихся строк из ${target}? Перед записью будет создан safety backup.`))return;
    const epoch=++analysisEpoch;analysisRunning=true;analysisError='';analysisProgress=`Очищаю ${target}…`;emitAnalysis();updateListButtons();
    try{
      const r=await api('/list/save',{method:'POST',body:JSON.stringify({name:target,content:next,expected_sha256:expected,confirm:'NFQWS_LIST_SAVE'})});
      if(disposed||epoch!==analysisEpoch)return;
      if(!r?.status)throw Error('NFQWS_ANALYSIS_SAVE_MISSING_STATUS');
      render(r.status);
      if(target===selectedList)await selectList(selectedList,true);
      const loaded=await loadNeighborListContents();
      if(disposed||epoch!==analysisEpoch)return;
      analysisSources=loaded.contents;analysisHashes=loaded.hashes;analysisIncomplete=loaded.errors.length>0;analysisDraftContent=String(getListEditor()?.readContext?.()?.value||'');analysisFindings=analyzeCrossLists(analysisSources,selectedList);
      analysisSelectedDirty=!!getListEditor()?.readContext?.()?.dirty;
      analysisProgress=`Удалено ${records.length} строк из ${target} · backup: ${r.backup||'—'}${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;
      renderAnalysis();notice(`Пересечения очищены в ${target}. Backup: ${r.backup||'—'}`,'good');
    }catch(e){if(!disposed&&epoch===analysisEpoch){analysisProgress='Ошибка очистки пересечений';analysisError=String(e.message||e);notice(analysisError,'error');}}
    finally{if(!disposed&&epoch===analysisEpoch){analysisRunning=false;renderAnalysis();updateListButtons();}}
  }
  async function runListAnalysis(){
    const context=getListEditor()?.readContext?.();
    if(analysisRunning||!context?.selected||!context?.meta)return;
    const epoch=++analysisEpoch;analysisRunning=true;analysisOpen=true;
    selectedList=context.selected;analysisSelectedDirty=!!context.dirty;
    analysisFindings=[];analysisSources={};analysisHashes={};analysisIncomplete=false;analysisDraftContent=String(context.value||'');analysisError='';analysisProgress='Читаю списки и строю граф пересечений…';emitAnalysis();updateListButtons();
    try{const loaded=await loadNeighborListContents();if(disposed||epoch!==analysisEpoch)return;
      if(getListEditor()?.readContext?.()?.selected!==context.selected)throw Error('NFQWS_LIST_SELECTION_STALE');
      analysisSources=loaded.contents;analysisHashes=loaded.hashes;analysisIncomplete=loaded.errors.length>0;
      analysisFindings=analyzeCrossLists(analysisSources,context.selected);
      analysisProgress=`Проанализировано ${Object.keys(analysisSources).length} списков${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;
      renderAnalysis();}
    catch(e){if(!disposed&&epoch===analysisEpoch){analysisProgress='Ошибка анализа';analysisError=String(e.message||e);}}
    finally{if(!disposed&&epoch===analysisEpoch){analysisRunning=false;emitAnalysis();updateListButtons();}}
  }
  function closeAnalysis(){analysisOpen=false;++analysisEpoch;analysisRunning=false;emitAnalysis();updateListButtons();}

  function normalizeStoredConfigName(raw){
    let name=String(raw||'').trim();if(!name)return'';if(!/\.conf$/i.test(name))name+='.conf';return/^[A-Za-z0-9][A-Za-z0-9._-]*\.conf$/i.test(name)?name:'';
  }
  // Cache-only data for Strategy Catalog / Smart Apply. The Config Library UI,
  // editor and all its interactions are owned by NfqwsConfigLibrary.svelte.
  async function loadConfigLibrary(force=false){
    if(configLibraryLoading||configLibraryLoaded&&!force)return;
    configLibraryLoading=true;
    try{
      const response=await api('/configs');
      if(disposed)return;
      configLibraryFiles=Array.isArray(response.files)?response.files:[];
      configLibraryLoaded=true;
    }finally{configLibraryLoading=false;}
  }
  // Log filtering, statistics and presentation are now owned by NfqwsLogs.svelte.
  function renderLog(){
    if(disposed)return;
    emit(onLogs,{tail:String(current?.log_tail||''),path:String(current?.log_path||'—'),truncated:!!current?.log_truncated});
  }
  // Lua inventory and read-only viewer now belong to NfqwsScripts.svelte.
  function openScriptFromSetup(name){activateNFQWSTab('scripts');emit(onScriptOpen,name);}

  // Svelte renders Backup Center; this command layer owns API/ledger actions,
  // never renders HTML, assigns click handlers or reads search DOM.
  function renderBackupCenter(){
    if(disposed)return;
    emit(onBackups,{files:backupCenterFiles.slice(),root:backupCenterRoot,selected:backupCenterSelected,
      detail:backupCenterSelectedData,loading:backupCenterLoading,error:backupCenterError,
      safetyCount:backupCenterSafetyCount,detailError:backupCenterDetailError,
      detailLoading:backupCenterDetailLoading});
  }
  async function loadBackupCenter(force=false){
    if(backupCenterLoading || (backupCenterLoaded&&!force)){renderBackupCenter();return;}
    backupCenterLoading=true;backupCenterError='';renderBackupCenter();
    try{
      const d=await api('/backups');
      if(disposed)return;
      backupCenterFiles=Array.isArray(d.files)?d.files:[];
      backupCenterRoot=d.root||'';
      backupCenterSafetyCount=Number(d.safety_count??current?.backup_count??0);
      backupCenterLoaded=true;
      if(backupCenterSelected&&!backupCenterFiles.some(x=>x.id===backupCenterSelected)){
        backupCenterSelected='';backupCenterSelectedData=null;backupCenterDetailError='';
      }
    }catch(e){if(!disposed)backupCenterError=String(e.message||e);}
    finally{backupCenterLoading=false;renderBackupCenter();}
  }
  async function selectBackupCenter(id){
    if(!backupCenterFiles.some(x=>x.id===id))return;
    const seq=++backupCenterSeq;
    backupCenterSelected=id;backupCenterSelectedData=null;
    backupCenterDetailError='';backupCenterDetailLoading=true;renderBackupCenter();
    try{
      const d=await api(`/backups/read?id=${encodeURIComponent(id)}`);
      if(!disposed&&seq===backupCenterSeq&&backupCenterSelected===id)backupCenterSelectedData=d;
    }catch(e){
      if(!disposed&&seq===backupCenterSeq)backupCenterDetailError=String(e.message||e);
    }finally{
      if(seq===backupCenterSeq){backupCenterDetailLoading=false;renderBackupCenter();}
    }
  }

  async function restoreBackupCenter(){
    const e=backupCenterSelectedData?.entry;if(!e?.id||!e.restorable)return;
    if(!confirm(`Restore ${e.label} → ${e.target}?\n\nПеред записью RouterForge создаст persistent snapshot текущего target. Для config работающий nfqws2 будет перезапущен и проверен.`))return;
    try{const r=await api('/backups/restore',{method:'POST',body:JSON.stringify({id:e.id,confirm:'NFQWS_BACKUP_RESTORE'})});backupCenterLoaded=false;backupCenterSelected='';backupCenterSelectedData=null;++backupCenterSeq;backupCenterDetailError='';if(r.status)render(r.status);await loadBackupCenter(true);notice(`Backup restored: ${e.label}`,'good')}
    catch(err){notice(err.message,'error')}
  }
  async function deleteBackupCenter(){
    const e=backupCenterSelectedData?.entry;if(!e?.id)return;
    if(!confirm(`Удалить persistent backup ${e.label}?\n\nЭто удаляет только snapshot, runtime-файлы не меняются.`))return;
    try{await api('/backups/delete',{method:'POST',body:JSON.stringify({id:e.id,confirm:'NFQWS_BACKUP_DELETE'})});backupCenterSelected='';backupCenterSelectedData=null;++backupCenterSeq;backupCenterLoaded=false;await loadBackupCenter(true);notice('Persistent backup удалён.','good')}
    catch(err){notice(err.message,'error')}
  }

  function render(s){
    if(disposed)return;current=s;const available=!!s?.available,running=!!s?.running;
    emit(onFlowLists,(s?.lists||[]).map(x=>String(x.name||'')));
    emit(onStatus,{available,running,mutationReady:!!s?.mutation_ready,
      target:String(s?.target_name||'nfqws'),configPath:String(s?.config_path||'target not detected'),backupCount:Number(s?.backup_count||0)});
    if(!available)notice('NFQWS target не найден.','error');else if(!running)notice('NFQWS обнаружен, но сервис остановлен.','error');else notice('NFQWS runtime обнаружен. RouterForge управляет установленным внешним пакетом, не заменяя его lifecycle.','good');
    original=s?.config||'';loadFlow(original);emit(onRawConfig,s);
    renderOverview();renderFlow();emit(onListInventory,(s?.lists||[]).slice());renderLog();setBusy(false);emit(onHeight,);
  }
  async function load(){setBusy(true);try{render(await api('/status'));if(disposed)return;void loadConfigLibrary(true)}catch(e){notice(e.message,'error');setBusy(false)}}
  async function action(name){if(busy||!current?.mutation_ready)return;if(!confirm(`NFQWS: ${name}?`))return;setBusy(true);try{const r=await api('/action',{method:'POST',body:JSON.stringify({action:name,confirm:'NFQWS'})});render(r.status);notice(`${name}: успешно`,'good')}catch(e){notice(e.message,'error');setBusy(false)}}
  function makeDiff(a,b){const A=lines(a),B=lines(b),out=[];for(let i=0;i<Math.max(A.length,B.length);i++){if(A[i]===B[i]){if(A[i]!==undefined)out.push({c:'same',t:`  ${A[i]}`});continue}if(A[i]!==undefined)out.push({c:'del',t:`- ${A[i]}`});if(B[i]!==undefined)out.push({c:'add',t:`+ ${B[i]}`})}return out}
  function closeDiff(){diffOpen=false;emitDiff()}
  function extractDomains(text){const set=new Set();for(let line of lines(text)){line=line.trim();if(!line||line.startsWith('#')||line.startsWith('//'))continue;line=line.replace(/^https?:\/\//i,'').replace(/^www\./i,'').split('#')[0].split('/')[0].split(':')[0].trim();if(/^[A-Za-z0-9.-]+\.[A-Za-z]{2,}$/.test(line))set.add(line)}return [...set]}

  const listSourceRepo='rndnaame/nfqws-menu',listSourceBranch='main',listSourcePrefix='strategies/lists/';
  let nfqwsMenuResolvedRef=null,listSourceResolution=null;
  async function resolveNFQWSMenuRef(force=false){
    if(nfqwsMenuResolvedRef&&!force)return nfqwsMenuResolvedRef;
    const resolved=await api('/upstream/nfqws-menu');
    nfqwsMenuResolvedRef=resolved;
    return resolved;
  }
  function listSourceLocalByName(name){return listSourceFiles.find(x=>String(x.name)===String(name))||null}
  async function listSourceFetch(url,timeout=10000){const c=new AbortController(),abort=()=>c.abort(),timer=setTimeout(abort,timeout);lifecycle.signal.addEventListener('abort',abort,{once:true});try{const r=await fetch(url,{cache:'no-store',signal:c.signal,headers:{Accept:'application/vnd.github+json'}});if(!r.ok)throw new Error(`HTTP ${r.status}`);return r}finally{clearTimeout(timer);lifecycle.signal.removeEventListener('abort',abort)}}
  async function listSourceSha256(buffer){if(!globalThis.crypto?.subtle)throw new Error('WebCrypto SHA-256 недоступен в этом браузере.');const d=await crypto.subtle.digest('SHA-256',buffer);return Array.from(new Uint8Array(d)).map(x=>x.toString(16).padStart(2,'0')).join('')}
  function listSourceBase64(buffer){const bytes=new Uint8Array(buffer);let out='';for(let i=0;i<bytes.length;i+=0x8000)out+=String.fromCharCode(...bytes.subarray(i,Math.min(i+0x8000,bytes.length)));return btoa(out)}
  function listSourceStateFor(item){const local=listSourceLocalByName(item.name);if(!local)return'missing';return String(local.sha256||'').toLowerCase()===String(item.sha256||'').toLowerCase()?'ok':'update'}
  async function resolveListSourceUpstream(force=false){
    const resolved=await resolveNFQWSMenuRef(force);listSourceResolution=resolved;const sha=String(resolved?.sha||'').trim();
    if(!/^[0-9a-f]{40}$/i.test(sha))throw new Error(resolved?.error||'upstream exact commit не получен');
    const manifestRes=await listSourceFetch(`https://raw.githubusercontent.com/${listSourceRepo}/${sha}/SHA256SUMS`,10000),manifest=await manifestRes.text(),items=[];
    for(const line of manifest.split(/\r?\n/)){const m=line.match(/^([0-9a-fA-F]{64})\s+(strategies\/lists\/([^\s/]+\.list))$/);if(m)items.push({name:m[3],path:m[2],sha256:m[1].toLowerCase(),ref:sha})}
    if(!items.length)throw new Error('upstream list manifest пуст');listSourceRef=sha;listSourceUpstream=items.sort((a,b)=>a.name.localeCompare(b.name));
  }
  function renderListSources(){
    if(disposed)return;
    emit(onListSources,{local:listSourceFiles.slice(),upstream:listSourceUpstream.slice(),ref:listSourceRef,
      root:listSourceRoot,error:listSourceError,loading:listSourceLoading,resolution:listSourceResolution});
  }
  async function loadListSources(force=false){
    if(listSourceLoading||(listSourceLoaded&&!force)){renderListSources();return;}
    listSourceLoading=true;listSourceError='';renderListSources();
    try{
      const local=await api('/list-sources');
      if(disposed)return;
      listSourceFiles=Array.isArray(local.files)?local.files:[];
      listSourceRoot=local.root||'/opt/etc/nfqws2/lists';
      try{await resolveListSourceUpstream(force);}
      catch(e){listSourceUpstream=[];listSourceRef='';listSourceError=e?.name==='AbortError'?'timeout':String(e.message||e);}
      listSourceLoaded=true;
    }catch(e){if(!disposed){listSourceLoaded=false;listSourceError=String(e.message||e);}}
    finally{listSourceLoading=false;renderListSources();}
  }

  async function installListSource(name){
    if(listSourceLoading)return;const item=listSourceUpstream.find(x=>x.name===name);if(!item||!listSourceRef)return;const local=listSourceLocalByName(name),replace=!!local,verb=replace?'заменить':'установить';if(!confirm(`List Source: ${verb} ${name}?\n\nSHA256 ${item.sha256}\nExact ref ${listSourceRef.slice(0,12)}\n\nSafety backup: ${replace?'YES':'not needed'}\nReload/restart: NO`))return;
    listSourceLoading=true;renderListSources();try{const r=await listSourceFetch(`https://raw.githubusercontent.com/${listSourceRepo}/${listSourceRef}/${item.path}`,10000),buffer=await r.arrayBuffer(),actual=await listSourceSha256(buffer);if(actual.toLowerCase()!==item.sha256.toLowerCase())throw new Error('upstream SHA256 mismatch');const result=await api('/list-sources/install',{method:'POST',body:JSON.stringify({name:item.name,content_base64:listSourceBase64(buffer),expected_sha256:item.sha256,expected_existing_sha256:replace?local.sha256:'',replace,confirm:replace?'NFQWS_LIST_SOURCE_REPLACE':'NFQWS_LIST_SOURCE_INSTALL'})});listSourceFiles=Array.isArray(result.inventory?.files)?result.inventory.files:listSourceFiles;current=result.status||current;emit(onListInventory,(current?.lists||[]).slice());renderListSources();notice(`${name}: ${result.already_current?'уже актуален':replace?'заменён':'установлен'} · runtime restart не выполнялся.`,'good')}
    catch(e){notice(`List Source ${name}: ${e.message}`,'error')}
    finally{listSourceLoading=false;renderListSources()}
  }

  const catalogRepo='rndnaame/nfqws-menu',catalogBranch='main',catalogPrefix='strategies/nfqws2/';
  const catalogDiffKeys=['ISP_INTERFACE','NFQWS_BASE_ARGS','NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET','NFQWS_ARGS_CUSTOM','NFQWS_EXTRA_ARGS','IPV6_ENABLED','TCP_PORTS','UDP_PORTS','POLICY_NAME','POLICY_EXCLUDE','NFQUEUE_NUM','USER','CONFIG_VERSION'];
  async function catalogFetch(url,timeout=10000){const c=new AbortController(),abort=()=>c.abort(),timer=setTimeout(abort,timeout);lifecycle.signal.addEventListener('abort',abort,{once:true});try{const r=await fetch(url,{cache:'no-store',signal:c.signal,headers:{Accept:'application/vnd.github+json'}});if(!r.ok)throw new Error(`HTTP ${r.status}`);return r}finally{clearTimeout(timer);lifecycle.signal.removeEventListener('abort',abort)}}
  async function catalogSha256(text){
    const value=String(text||'');
    try{if(globalThis.crypto?.subtle){const d=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(value));return Array.from(new Uint8Array(d)).map(x=>x.toString(16).padStart(2,'0')).join('')}}catch(_){}
    function rr(v,n){return(v>>>n)|(v<<(32-n))}const K=[1116352408,1899447441,3049323471,3921009573,961987163,1508970993,2453635748,2870763221,3624381080,310598401,607225278,1426881987,1925078388,2162078206,2614888103,3248222580,3835390401,4022224774,264347078,604807628,770255983,1249150122,1555081692,1996064986,2554220882,2821834349,2952996808,3210313671,3336571891,3584528711,113926993,338241895,666307205,773529912,1294757372,1396182291,1695183700,1986661051,2177026350,2456956037,2730485921,2820302411,3259730800,3345764771,3516065817,3600352804,4094571909,275423344,430227734,506948616,659060556,883997877,958139571,1322822218,1537002063,1747873779,1955562222,2024104815,2227730452,2361852424,2428436474,2756734187,3204031479,3329325298];
    const data=Array.from(new TextEncoder().encode(value)),bitLen=data.length*8;data.push(128);while(data.length%64!==56)data.push(0);const hi=Math.floor(bitLen/4294967296),lo=bitLen>>>0;for(let i=3;i>=0;i--)data.push((hi>>>(i*8))&255);for(let i=3;i>=0;i--)data.push((lo>>>(i*8))&255);let h=[1779033703,3144134277,1013904242,2773480762,1359893119,2600822924,528734635,1541459225];
    for(let off=0;off<data.length;off+=64){const w=new Array(64);for(let i=0;i<16;i++)w[i]=((data[off+i*4]<<24)|(data[off+i*4+1]<<16)|(data[off+i*4+2]<<8)|data[off+i*4+3])>>>0;for(let i=16;i<64;i++){const s0=(rr(w[i-15],7)^rr(w[i-15],18)^(w[i-15]>>>3))>>>0,s1=(rr(w[i-2],17)^rr(w[i-2],19)^(w[i-2]>>>10))>>>0;w[i]=(w[i-16]+s0+w[i-7]+s1)>>>0}let[a,b,c,d,e,f,g,hh]=h;for(let i=0;i<64;i++){const S1=(rr(e,6)^rr(e,11)^rr(e,25))>>>0,ch=((e&f)^((~e)&g))>>>0,t1=(hh+S1+ch+K[i]+w[i])>>>0,S0=(rr(a,2)^rr(a,13)^rr(a,22))>>>0,maj=((a&b)^(a&c)^(b&c))>>>0,t2=(S0+maj)>>>0;hh=g;g=f;f=e;e=(d+t1)>>>0;d=c;c=b;b=a;a=(t1+t2)>>>0}h=[(h[0]+a)>>>0,(h[1]+b)>>>0,(h[2]+c)>>>0,(h[3]+d)>>>0,(h[4]+e)>>>0,(h[5]+f)>>>0,(h[6]+g)>>>0,(h[7]+hh)>>>0]}
    return h.map(x=>x.toString(16).padStart(8,'0')).join('');
  }
  function catalogNormalizeRefPath(path){return String(path||'').split('/').pop().replace(/["']/g,'')}
  function catalogDependencyModel(config){
    const text=String(config||''),lists=new Set(),blobs=new Map(),blobUses=new Set(),lua=new Set();let m;
    const lr=/--(?:hostlist|hostlist-exclude|hostlist-auto|ipset|ipset-exclude)=([^\s"'\\]+)/g;while((m=lr.exec(text))!==null)lists.add(catalogNormalizeRefPath(m[1]));
    const bd=/--blob=([A-Za-z0-9._-]+):@?([^\s"'\\]+)/g;while((m=bd.exec(text))!==null)blobs.set(m[1],m[2]);
    const bu=/(?:^|[:=])blob=([A-Za-z0-9._-]+)/g;while((m=bu.exec(text))!==null)blobUses.add(m[1]);
    const lu=/--lua-init=@?([^\s"'\\]+)/g;while((m=lu.exec(text))!==null)lua.add(m[1]);
    return{lists:[...lists].sort(),blobs:[...blobs.entries()].map(([name,path])=>({name,path,used:blobUses.has(name)})),lua:[...lua].sort(),tcp:assignmentValue(text,'TCP_PORTS'),udp:assignmentValue(text,'UDP_PORTS'),mode:assignmentValue(text,'NFQWS_EXTRA_ARGS')};
  }
  function catalogCompatibility(config,integrity){
    const text=String(config||''),checks=[],known=new Set((current?.lists||[]).map(x=>String(x.name||''))),deps=catalogDependencyModel(text),add=(severity,title,detail)=>checks.push({severity,title,detail});
    if(!text.trim())add('bad','Пустой config','Полный nfqws2.conf отсутствует.');else add('good','Полный config загружен',`${lines(text).length} lines · ${bytes(new Blob([text]).size)}`);
    const hasStrategy=['NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET','NFQWS_ARGS_CUSTOM'].some(k=>String(assignmentValue(text,k)||'').trim());add(hasStrategy?'good':'bad',hasStrategy?'Strategy assignments найдены':'Strategy assignments не найдены',hasStrategy?'NFQWS_ARGS* доступны для semantic analysis.':'Нет NFQWS_ARGS* — это не совместимый полный strategy config.');
    for(const key of ['NFQWS_BASE_ARGS','NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET']){const v=assignmentValue(text,key);if(/(^|\s)--new(?:\s|$)/.test(v))add('bad',`${key}: запрещён --new`,'Upstream contract RouterForge разрешает --new только внутри NFQWS_ARGS_CUSTOM.')}
    const iface=assignmentValue(text,'ISP_INTERFACE');add(iface?'good':'warn',iface?'ISP_INTERFACE задан':'ISP_INTERFACE отсутствует',iface||'Перед Smart Apply понадобится WAN interface.');
    for(const key of ['NFQUEUE_NUM','USER','CONFIG_VERSION'])if(!assignmentValue(text,key))add('warn',`${key} не найден`,'Полный config стоит проверить перед Smart Apply.');
    const missing=deps.lists.filter(x=>!known.has(x));add(missing.length?'warn':'good',missing.length?'Есть отсутствующие локально list dependencies':'List dependencies разрешены',missing.length?missing.join(', '):(deps.lists.join(', ')||'Ссылок на list-файлы нет.'));
    if(deps.blobs.length)add('info',`${deps.blobs.length} blob dependencies`,`Проверь Blob Manager: ${deps.blobs.map(x=>x.name).join(', ')}`);else add('info','Blob dependencies не обнаружены','R26 всё равно проверит локальный blob inventory.');
    if(deps.lua.length)add('info',`${deps.lua.length} Lua dependencies`,deps.lua.map(catalogNormalizeRefPath).join(', '));
    if(integrity==='verified')add('good','SHA256 integrity: VERIFIED','Файл совпадает с hash из upstream SHA256SUMS.');else if(integrity==='mismatch')add('bad','SHA256 integrity: MISMATCH','Полученный config не совпадает с upstream manifest.');else add('info','SHA256 integrity: LOCAL/UNAVAILABLE','Для локального файла используется локальный SHA; upstream manifest не применим.');
    const bad=checks.filter(x=>x.severity==='bad').length,warn=checks.filter(x=>x.severity==='warn').length;return{checks,deps,status:bad?'BLOCKED':warn?'CHECK':'COMPATIBLE'};
  }
  function catalogSemanticDiff(config){
    return catalogDiffKeys.map(key=>{const active=assignmentValue(original,key),candidate=assignmentValue(config,key);let state='same';if(active!==candidate)state=!active&&candidate?'added':active&&!candidate?'removed':'changed';return{key,state,active,candidate}}).filter(x=>x.state!=='same').concat(catalogDiffKeys.map(key=>{const active=assignmentValue(original,key),candidate=assignmentValue(config,key);return{key,state:'same',active,candidate}}).filter(x=>x.state==='same').slice(0,3));
  }
  function catalogSourceLabel(item){return item.kind==='upstream'?'UPSTREAM':item.kind==='active'?'ACTIVE':'LOCAL'}
  // Catalog UI/filters now belong solely to NfqwsCatalog.svelte.
  function emitCatalog(){
    if(disposed)return;
    const d=catalogSelectedData;
    const model=d?catalogCompatibility(d.content,d.integrity):null;
    const diff=d?catalogSemanticDiff(d.content):[];
    emit(onCatalog,{items:catalogItems.slice(),selected:catalogSelected,detail:d?{
      item:d.item,content:d.content,sha256:d.sha256,integrity:d.integrity,
      compatibility:model,diff,missingLists:model.deps.lists.filter(x=>!(current?.lists||[]).some(y=>y.name===x)).length
    }:null,upstream:{...catalogUpstream},loading:catalogLoading,selecting:catalogSelectionBusy,
      error:catalogError});
  }
  const renderCatalogList=emitCatalog;
  const renderCatalogSources=emitCatalog;
  async function loadCatalog(force=false){
    if(catalogLoading||catalogLoaded&&!force){emitCatalog();return}catalogLoading=true;catalogError='';catalogUpstream={state:'loading',ref:catalogBranch,sha:'',error:''};emitCatalog();try{
      if(!configLibraryLoaded)await loadConfigLibrary(true);const local=[{id:'active:canonical',kind:'active',name:'nfqws2.conf',path:current?.config_path||'/opt/etc/nfqws2/nfqws2.conf',source:'RouterForge active',ref:'active',sha256:current?.config_sha256||''},...configLibraryFiles.map(x=>({id:`local:${x.name}`,kind:'local',name:x.name,path:x.path,source:'Config Library',ref:'local',sha256:x.sha256||'',active:!!x.active}))];
      let upstream=[];try{
        const resolved=await resolveNFQWSMenuRef(force),sha=String(resolved?.sha||'').trim();catalogUpstream.sha=sha;catalogUpstream.ref=catalogBranch;catalogUpstream.source=String(resolved?.source||'');
        if(!/^[0-9a-f]{40}$/i.test(sha)){catalogUpstream.state=resolved?.state==='rate_limited'?'rate_limited':'error';throw new Error(resolved?.error||'upstream exact commit не получен')}
        const ref=sha,manifestRes=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${ref}/SHA256SUMS`,10000),manifest=await manifestRes.text();
        upstream=manifest.split(/\r?\n/).map(line=>{const m=line.match(/^([0-9a-fA-F]{64})\s+(strategies\/nfqws2\/[^\s]+\.conf)$/);if(!m)return null;return{id:`upstream:${m[2]}`,kind:'upstream',name:m[2].split('/').pop(),path:m[2],source:catalogRepo,ref:ref,sha256:m[1].toLowerCase()}}).filter(Boolean);catalogUpstream.state=resolved?.state==='stale_cache'?'stale':'ready';catalogUpstream.count=upstream.length;
      }catch(e){if(catalogUpstream.state!=='rate_limited')catalogUpstream.state='error';catalogUpstream.error=e?.name==='AbortError'?'timeout':String(e.message||e)}
      catalogItems=[...local,...upstream];catalogLoaded=true;renderCatalogSources();renderCatalogList();if(catalogSelected&&!catalogItems.some(x=>x.id===catalogSelected)){catalogSelected='';catalogSelectedData=null;catalogError='';emitCatalog()}
    }finally{catalogLoading=false;renderCatalogSources()}
  }
  async function catalogReadItem(item){
    if(item.kind==='active')return{content:original,sha256:current?.config_sha256||'',integrity:'local'};
    if(item.kind==='local'){const d=await api(`/configs/read?name=${encodeURIComponent(item.name)}`);const actual=await catalogSha256(d.content||'');return{content:d.content||'',sha256:actual||d.sha256||'',integrity:item.sha256&&actual&&item.sha256.toLowerCase()!==actual.toLowerCase()?'mismatch':'local'}};
    const r=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${item.ref}/${item.path}`,10000),content=await r.text(),actual=await catalogSha256(content),integrity=item.sha256&&actual?(item.sha256.toLowerCase()===actual.toLowerCase()?'verified':'mismatch'):'unknown';return{content,sha256:actual,integrity};
  }
  async function selectCatalogItem(id){
    const item=catalogItems.find(x=>x.id===id);
    if(!item||disposed)return;
    const epoch=++catalogSelectEpoch;
    catalogSelected=id;catalogSelectedData=null;catalogError='';catalogSelectionBusy=true;emitCatalog();
    try{
      const data=await catalogReadItem(item);
      if(disposed||epoch!==catalogSelectEpoch)return;
      catalogSelectedData={item,...data};
    }catch(e){
      if(!disposed&&epoch===catalogSelectEpoch)catalogError=`Источник недоступен: ${String(e?.message||e)}`;
    }finally{
      if(!disposed&&epoch===catalogSelectEpoch){catalogSelectionBusy=false;emitCatalog();}
    }
  }

  const smartApplyPreserveKeys=['ISP_INTERFACE','IPV6_ENABLED','POLICY_NAME','POLICY_EXCLUDE','NFQUEUE_NUM','USER','LOG_LEVEL','LOG_DEBUG_PATH'];
  function smartMergeAssignment(config,key,value){
    if(value==='')return config;const a=scanAssignment(config,key);if(a)return replaceAssignment(config,key,value);const suffix=config.endsWith('\n')?'':'\n';return config+suffix+`${key}=${formatAssignment(key,value,null).slice(key.length+1)}\n`;
  }
  function smartMergeConfig(candidate){
    let merged=String(candidate||''),preserved=[];for(const key of smartApplyPreserveKeys){const active=assignmentValue(original,key);if(active==='')continue;const before=assignmentValue(merged,key);merged=smartMergeAssignment(merged,key,active);if(before!==active)preserved.push({key,active,candidate:before||'—'})}return{config:merged,preserved};
  }
  async function smartApplyManifestForRef(ref){
    const r=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${ref}/SHA256SUMS`,10000),text=await r.text(),lists=new Map(),blobs=new Map();
    for(const line of text.split(/\r?\n/)){let m=line.match(/^([0-9a-fA-F]{64})\s+(strategies\/lists\/([^\s/]+\.list))$/);if(m){lists.set(m[3],{sha256:m[1].toLowerCase(),path:m[2]});continue}m=line.match(/^([0-9a-fA-F]{64})\s+(strategies\/blobs\/([^\s/]+\.bin))$/);if(m)blobs.set(m[3],{sha256:m[1].toLowerCase(),path:m[2]})}
    return{lists,blobs};
  }
  function smartApplyLocalList(name){return listSourceFiles.find(x=>String(x.name)===String(name))||null}
  function smartApplyLocalBlob(name){return blobFiles.find(x=>String(x.name)===String(name))||null}
  async function buildSmartApplyPlan(){
    if(!catalogSelectedData)throw new Error('Сначала выберите config в Каталоге.');if(!current?.mutation_ready)throw new Error('Runtime mutation boundary не готов.');if(!current?.config_sha256)throw new Error('Active config SHA256 недоступен.');
    const d=catalogSelectedData,item=d.item,compat=catalogCompatibility(d.content,d.integrity);if(compat.status==='BLOCKED')throw new Error('Candidate BLOCKED compatibility checks; Smart Apply запрещён.');if(item.kind==='upstream'&&d.integrity!=='verified')throw new Error('Upstream candidate должен иметь VERIFIED SHA256.');
    const merge=smartMergeConfig(d.content),config=merge.config,configSha=await catalogSha256(config),deps=catalogDependencyModel(config);await loadListSources(true);await loadBlobs(true);
    let manifest={lists:new Map(),blobs:new Map()};if(item.kind==='upstream'){if(!item.ref)throw new Error('Exact upstream ref отсутствует.');manifest=await smartApplyManifestForRef(item.ref)}
    const lists=deps.lists.map(name=>{const local=smartApplyLocalList(name),expected=manifest.lists.get(name)||null;let state=local?'ok':'blocked',reason=local?'локальный файл присутствует':'нет локального файла';if(expected){state=local&&String(local.sha256||'').toLowerCase()===expected.sha256?'ok':'write';reason=state==='ok'?'SHA256 совпадает с exact upstream':'будет установлен/заменён из exact upstream'}return{name,local,expected,state,reason}});
    const blobs=deps.blobs.map(dep=>{const name=blobBasename(dep.path),local=smartApplyLocalBlob(name),expected=manifest.blobs.get(name)||null;let state=local?'ok':'blocked',reason=local?'локальный blob присутствует':'нет локального blob';if(expected){state=local&&String(local.sha256||'').toLowerCase()===expected.sha256?'ok':'write';reason=state==='ok'?'SHA256 совпадает с exact upstream':'будет установлен/заменён из exact upstream'}return{name,local,expected,state,reason}});
    const blocked=[...lists,...blobs].filter(x=>x.state==='blocked');return{item,compat,merge,config,configSha,lists,blobs,blocked,expectedActiveSha:current.config_sha256,wasRunning:!!current.running,ref:item.ref||'local'};
  }
  // UI/plan presentation is entirely owned by Svelte. No HTML injection,
  // document selectors, or mutation requests during component mount.
  function renderSmartApplyPlan(){
    if(disposed)return;
    emit(onSmartApply,{open:smartApplyOpen,preparing:smartApplyPreparing,busy:smartApplyBusy,
      plan:smartApplyPlan,error:smartApplyError});
  }
  async function refreshSmartApply(){
    if(disposed||smartApplyBusy||smartApplyPreparing||!smartApplyOpen)return;
    const epoch=++smartApplyEpoch;
    smartApplyPreparing=true;smartApplyError='';smartApplyPlan=null;renderSmartApplyPlan();
    try{
      const plan=await buildSmartApplyPlan();
      if(!disposed&&epoch===smartApplyEpoch&&smartApplyOpen)smartApplyPlan=plan;
    }catch(e){
      if(!disposed&&epoch===smartApplyEpoch&&smartApplyOpen){
        smartApplyError=String(e?.message||e);notice(`Smart Apply plan: ${smartApplyError}`,'error');
      }
    }finally{
      if(epoch===smartApplyEpoch){smartApplyPreparing=false;renderSmartApplyPlan();}
    }
  }
  function openSmartApply(){
    if(disposed||smartApplyBusy)return;
    smartApplyOpen=true;renderSmartApplyPlan();return refreshSmartApply();
  }
  function closeSmartApply(){
    if(smartApplyBusy)return;
    smartApplyOpen=false;++smartApplyEpoch;smartApplyPreparing=false;
    smartApplyPlan=null;smartApplyError='';renderSmartApplyPlan();
  }

  async function smartApplyFetchDependency(kind,row,ref){
    if(row.state!=='write'||!row.expected)return null;const url=`https://raw.githubusercontent.com/${catalogRepo}/${ref}/${row.expected.path}`,r=await catalogFetch(url,15000),buffer=await r.arrayBuffer(),actual=kind==='list'?await listSourceSha256(buffer):await blobSha256Bytes(buffer);if(actual.toLowerCase()!==row.expected.sha256.toLowerCase())throw new Error(`${row.name}: upstream SHA256 mismatch before transaction`);return{name:row.name,content_base64:listSourceBase64(buffer),expected_sha256:row.expected.sha256};
  }
  async function applySmartApply(){
    const p=smartApplyPlan;const recoverySettings=getRecoverySettings(),recoveryBaseline=recoverySettings.enabled?latestRecoveryBaselineForCurrent():null,recoveryStartedAt=new Date().toISOString();if(!smartApplyOpen||!p||p.blocked.length||smartApplyBusy||smartApplyPreparing)return;if(!confirm(`Smart Apply ${p.item.name}?\nDependencies to write: ${[...p.lists,...p.blobs].filter(x=>x.state==='write').length}\nConfig: semantic merge\n${p.wasRunning?'Service restart: YES':'Service restart: NO'}\nRollback on first failure: YES`))return;smartApplyBusy=true;smartApplyError='';renderSmartApplyPlan();
    try{const lists=(await Promise.all(p.lists.map(x=>smartApplyFetchDependency('list',x,p.ref)))).filter(Boolean),blobs=(await Promise.all(p.blobs.map(x=>smartApplyFetchDependency('blob',x,p.ref)))).filter(Boolean);const latest=await api('/status');if(String(latest.config_sha256||'').toLowerCase()!==String(p.expectedActiveSha).toLowerCase())throw new Error('Active config изменился после построения плана. Пересчитайте Smart Apply.');const expected_lists=Object.fromEntries(p.lists.filter(x=>x.expected).map(x=>[x.name,x.expected.sha256])),expected_blobs=Object.fromEntries(p.blobs.filter(x=>x.expected).map(x=>[x.name,x.expected.sha256])),result=await api('/smart-apply',{method:'POST',body:JSON.stringify({config:p.config,expected_active_sha256:p.expectedActiveSha,expected_config_sha256:p.configSha,lists,blobs,expected_lists,expected_blobs,confirm:'NFQWS_SMART_APPLY'})});current=result.status||current;original=current?.config||p.config;loadFlow(original);listSourceLoaded=false;blobLoaded=false;configLibraryLoaded=false;catalogLoaded=false;smartApplyPlan=null;smartApplyOpen=false;render(current);renderSmartApplyPlan();notice(`Smart Apply: PASS · ${p.item.name} · lists ${result.applied_lists?.length||0} · blobs ${result.applied_blobs?.length||0}${result.service_restarted?' · runtime restarted':''}`,'good');if(recoveryBaseline)void runRecoveryGuard(recoveryBaseline,recoveryStartedAt,p.item.name,recoverySettings.threshold)}
    catch(e){smartApplyError='Smart Apply FAIL: '+String(e.message||e)+'. Если runtime начал mutation, серверный ответ содержит rollback state.';renderSmartApplyPlan();notice(`Smart Apply: ${e.message}`,'error')}
    finally{smartApplyBusy=false;renderSmartApplyPlan()}
  }



  // ClientHello capture, generate, forge and blob save belong to
  // NfqwsClientHello.svelte. No legacy DOM writer or click handler remains.

  function blobBasename(path){return String(path||'').split('/').pop().replace(/["']/g,'')}
  function blobInstalledByName(name){return blobFiles.find(x=>String(x.name)===String(name))}
  async function blobManifestForRef(ref){
    const key=String(ref||'');if(!key)return new Map();if(blobManifestCache[key])return blobManifestCache[key];
    const r=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${key}/SHA256SUMS`,10000),text=await r.text(),map=new Map();
    for(const line of text.split(/\r?\n/)){const m=line.match(/^([0-9a-fA-F]{64})\s+(strategies\/blobs\/([^\s/]+\.bin))$/);if(m)map.set(m[3],{sha256:m[1].toLowerCase(),path:m[2]})}
    blobManifestCache[key]=map;return map;
  }
  async function blobSha256Bytes(buffer){
    if(!globalThis.crypto?.subtle)throw new Error('WebCrypto SHA-256 недоступен в этом браузере.');
    const d=await crypto.subtle.digest('SHA-256',buffer);return Array.from(new Uint8Array(d)).map(x=>x.toString(16).padStart(2,'0')).join('');
  }
  function blobBase64(buffer){
    const bytes=new Uint8Array(buffer);let out='';for(let i=0;i<bytes.length;i+=0x8000)out+=String.fromCharCode(...bytes.subarray(i,Math.min(i+0x8000,bytes.length)));return btoa(out);
  }
  async function loadBlobs(force=false){
    if(blobLoading||blobLoaded&&!force){await refreshBlobDependencies();renderBlobs();return}blobLoading=true;blobError='';renderBlobs();
    try{const d=await api('/blobs');blobFiles=Array.isArray(d.files)?d.files:[];blobRoot=d.root||'/opt/etc/nfqws2/blobs';blobLoaded=true;await refreshBlobDependencies();renderBlobs()}
    catch(e){blobLoaded=false;notice(`Blob inventory: ${e.message}`,'error');blobError=String(e.message||e);renderBlobs()}
    finally{blobLoading=false}
  }
  async function refreshBlobDependencies(){
    blobExpectedMap={};if(!catalogSelectedData)return;
    const item=catalogSelectedData.item,deps=catalogDependencyModel(catalogSelectedData.content).blobs;
    if(item?.kind!=='upstream'||!item.ref)return;
    try{const manifest=await blobManifestForRef(item.ref);for(const dep of deps){const name=blobBasename(dep.path)||`${dep.name}.bin`,meta=manifest.get(name);if(meta)blobExpectedMap[name]=meta}}catch(e){notice(`Blob manifest: ${e.message}`,'error')}
  }
  function blobDependencyRows(){
    if(!catalogSelectedData)return[];const deps=catalogDependencyModel(catalogSelectedData.content).blobs,item=catalogSelectedData.item;
    return deps.map(dep=>{const name=blobBasename(dep.path),installed=blobInstalledByName(name),expected=blobExpectedMap[name]||null;let state='missing';
      if(installed){state=expected&&String(installed.sha256).toLowerCase()!==expected.sha256?'update':'ok'}
      return{dep,name,installed,expected,state,item};
    });
  }
  // Svelte handles HTML and clicks; the command layer emits immutable snapshots.
  function renderBlobs(){
    if(disposed)return;
    const deps=blobDependencyRows();
    emit(onBlobs,{files:blobFiles.slice(),root:blobRoot,dependencies:deps,
      selectedCatalog:catalogSelectedData?.item||null,loading:blobLoading,
      busy:blobActionBusy,error:blobError});
  }
  async function installCatalogBlob(name){
    const row=blobDependencyRows().find(x=>x.name===name);
    if(!row||row.item.kind!=='upstream'||!row.expected)return;
    verifiedBlobManifest(row,name);
    const replace=!!row.installed;if(replace&&!confirm(`Заменить ${name}? Будет создан safety backup. Running nfqws НЕ перезапускается; новый файл на диске вступит в силу только при будущем controlled restart/apply.`))return;
    if(!replace&&!confirm(`Установить ${name} из ${catalogRepo} @ ${String(row.item.ref).slice(0,12)} с проверкой SHA256?`))return;

    try{
      const r=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${row.item.ref}/${row.expected.path}`,12000),buffer=await r.arrayBuffer();if(buffer.byteLength>1048576)throw new Error('blob > 1 MiB safety limit');
      const actual=await blobSha256Bytes(buffer);if(actual!==row.expected.sha256)throw new Error(`SHA256 mismatch before install: ${actual}`);
      const d=await api('/blobs/install',{method:'POST',body:JSON.stringify({name,content_base64:blobBase64(buffer),expected_sha256:row.expected.sha256,replace,confirm:replace?'NFQWS_BLOB_REPLACE':'NFQWS_BLOB_INSTALL'})});
      if(!Array.isArray(d.inventory?.files))throw Error('NFQWS_BLOB_INSTALL_INVENTORY_MISSING');
      const verified=await api('/blobs'),files=verified?.files;
      if(!Array.isArray(files)||!files.some(x=>x.name===name&&String(x.sha256||'').toLowerCase()===row.expected.sha256))
        throw Error('NFQWS_BLOB_INSTALL_POSTVERIFY_MISMATCH');
      blobFiles=files;blobLoaded=true;blobError='';renderBlobs();
      notice(`${name}: ${replace?'заменён':'установлен'}; SHA256 verified. Runtime restart не выполнялся.`,'good')
    }catch(e){blobError=String(e.message||e);notice(`Blob ${name}: ${e.message}`,'error');renderBlobs()}
  }
  async function deleteBlob(name){
    const item=blobInstalledByName(name);if(!item||item.referenced)return;if(!confirm(`Удалить unreferenced blob ${name}? Перед удалением будет создан safety backup.`))return;
    try{
      const d=await api('/blobs/delete',{method:'POST',body:JSON.stringify({name,confirm:'NFQWS_BLOB_DELETE'})});
      if(!Array.isArray(d.inventory?.files))throw Error('NFQWS_BLOB_DELETE_INVENTORY_MISSING');
      const verified=await api('/blobs');
      if(!Array.isArray(verified?.files)||verified.files.some(x=>x.name===name))throw Error('NFQWS_BLOB_DELETE_POSTVERIFY_MISMATCH');
      blobFiles=verified.files;blobLoaded=true;blobError='';renderBlobs();notice(`${name}: удалён, backup=${d.backup||'created'}`,'good');
    }catch(e){blobError=String(e.message||e);renderBlobs();notice(`Удаление ${name}: ${e.message}`,'error')}
  }



  async function validateInstalledClientHello(name){
    try{
      const d=await api(`/clienthello/validate?name=${encodeURIComponent(name)}`);
      notice(`${name}: ${d.valid?'VALID ClientHello':'not a valid ClientHello'}${d.sni?` · SNI ${d.sni}`:''} · ${d.detail||''}`,d.valid?'good':'error');
    }catch(e){notice(`ClientHello validate ${name}: ${e.message}`,'error')}
  }

  // Target Memory read, projection and confirmed clear now belong to NfqwsTargetMemory.svelte.

  async function pruneBackupCenter(){
    if(!confirm('Очистить старые NFQWS backups по встроенной retention policy?\n\nRuntime/config не меняются. Удаляются только snapshots сверх retention limits.'))return;
    try{
      const d=await api('/backups/prune',{method:'POST',body:JSON.stringify({confirm:'NFQWS_BACKUP_PRUNE'})});
      backupCenterLoaded=false;await loadBackupCenter(true);
      notice(`Backup prune: persistent ${Number(d.persistent_before||0)} → ${Number(d.persistent_after||0)}, safety ${Number(d.safety_before||0)} → ${Number(d.safety_after||0)}.`,'good');
    }catch(e){notice(`Backup prune: ${e.message}`,'error')}
  }

  function testLabEvidenceByConfig(){
    const groups=new Map();
    for(const run of runHistory){
      if(run.kind!=='testlab'||!run.active_config_sha256||!Number.isFinite(Number(run.score)))continue;
      const sha=String(run.active_config_sha256).toLowerCase(),arr=groups.get(sha)||[];arr.push(run);groups.set(sha,arr);
    }
    const median=a=>{const x=a.slice().sort((m,n)=>m-n);if(!x.length)return null;const i=Math.floor(x.length/2);return x.length%2?x[i]:(x[i-1]+x[i])/2};
    return [...groups.entries()].map(([sha,runs])=>{
      const scores=runs.map(x=>Number(x.score)).filter(Number.isFinite);
      const reach=runs.map(x=>Number(x.metrics?.success_ratio)).filter(Number.isFinite);
      const stable=runs.map(x=>Number(x.metrics?.stable_ratio)).filter(Number.isFinite);
      const targets=runs.map(x=>Number(x.metrics?.targets)||0);
      const latest=runs.slice().sort((a,b)=>String(b.created_at).localeCompare(String(a.created_at)))[0];
      return{sha,runs:runs.length,median_score:median(scores),min_score:scores.length?Math.min(...scores):0,latest_score:Number(latest?.score)||0,reach:median(reach)??0,stable:median(stable)??0,targets:Math.max(...targets,0),latest};
    }).sort((a,b)=>(b.median_score-a.median_score)||(b.runs-a.runs)||(b.latest_score-a.latest_score));
  }
  function autoPickMapName(sha){
    if(String(current?.config_sha256||'').toLowerCase()===sha)return'ACTIVE nfqws2.conf';
    const local=configLibraryFiles.find(x=>String(x.sha256||'').toLowerCase()===sha);return local?local.name:'SHA '+sha.slice(0,12);
  }
  const autoPickEligible=measuredAutoPickEligible;
  // The Svelte History owner renders measured-only ranking; no DOM or auto-apply.
  let historyAutoPickEpoch=0;
  async function renderAutoPick(){
    const epoch=++historyAutoPickEpoch;
    if(!configLibraryLoaded){try{await loadConfigLibrary()}catch(_){}}
    if(disposed||epoch!==historyAutoPickEpoch)return;
    renderRunHistory();
  }
  function latestRecoveryBaselineForCurrent(){
    const sha=String(current?.config_sha256||'').toLowerCase();
    return runHistory.find(x=>x.kind==='testlab'&&String(x.active_config_sha256||'').toLowerCase()===sha&&Array.isArray(x.payload?.results)&&x.payload.results.length>=3)||null;
  }
  function recoveryBaselineMetrics(run){
    const rows=run?.payload?.results||[],attempts=rows.flatMap(x=>x.attempts||[]),ok=attempts.filter(x=>x.ok).length,stable=rows.filter(x=>x.attempts?.length&&x.attempts.every(a=>a.ok)).length;
    return{targets:rows.map(x=>x.target).filter(Boolean).slice(0,64),reach:attempts.length?ok/attempts.length:0,stable:rows.length?stable/rows.length:0,rounds:Math.max(1,...rows.map(x=>x.attempts?.length||1))};
  }
  async function recoveryProbeTargets(targets,rounds){
    const rows=targets.map(target=>({target,attempts:[]})),queue=[];rows.forEach((_,i)=>{for(let r=0;r<rounds;r++)queue.push({i,r})});
    async function worker(){while(queue.length){const job=queue.shift(),row=rows[job.i],start=performance.now();try{const d=await api('/check',{method:'POST',body:JSON.stringify({url:row.target})});row.attempts.push({ok:!!d.reachable,status:d.status_code||0,ms:performance.now()-start,error:d.error||''})}catch(e){row.attempts.push({ok:false,status:0,ms:performance.now()-start,error:e.message||'probe failed'})}}}
    await Promise.all(Array.from({length:Math.min(4,queue.length)},worker));
    const attempts=rows.flatMap(x=>x.attempts),ok=attempts.filter(x=>x.ok).length,stable=rows.filter(x=>x.attempts.length&&x.attempts.every(a=>a.ok)).length;
    return{rows,reach:attempts.length?ok/attempts.length:0,stable:rows.length?stable/rows.length:0};
  }
  async function recoverySnapshotSet(sinceIso){
    await loadBackupCenter(true);
    const since=Date.parse(sinceIso)||0;
    return backupCenterFiles.filter(x=>Date.parse(x.created_at||x.created||0)>=since&&/^smart-apply-(config|list-|blob-)/.test(String(x.label||''))).sort((a,b)=>String(a.created_at||a.created).localeCompare(String(b.created_at||b.created)));
  }
  async function restoreRecoverySnapshots(entries){
    const deps=entries.filter(x=>x.kind!=='config').sort((a,b)=>String(b.created_at||b.created).localeCompare(String(a.created_at||a.created)));
    const config=entries.filter(x=>x.kind==='config').sort((a,b)=>String(b.created_at||b.created).localeCompare(String(a.created_at||a.created))).slice(0,1);
    const ordered=[...deps,...config];
    if(!config.length)throw new Error('persistent pre-apply config snapshot не найден');
    for(const entry of ordered)await api('/backups/restore',{method:'POST',body:JSON.stringify({id:entry.id,confirm:'NFQWS_BACKUP_RESTORE'})});
    return ordered;
  }
  async function runRecoveryGuard(baseline,startedAt,label,thresholdPercent){
    if(recoveryGuardRunning||!baseline)return;recoveryGuardRunning=true;
    renderRunHistory();
    try{
      const b=recoveryBaselineMetrics(baseline),after=await recoveryProbeTargets(b.targets,Math.min(3,b.rounds)),regressed=recoveryRegression(b,after,thresholdPercent).regressed;
      if(!regressed){
        recoveryGuardLastResult={state:'PASS',label,reach_before:b.reach,reach_after:after.reach,stable_before:b.stable,stable_after:after.stable};
        renderRunHistory();
        notice('Recovery Guard: PASS · regression threshold не достигнут.','good');return;
      }
      renderRunHistory();
      const snapshots=await recoverySnapshotSet(startedAt),restored=await restoreRecoverySnapshots(snapshots);
      await load();backupCenterLoaded=false;await loadBackupCenter(true);
      recoveryGuardLastResult={state:'RECOVERED',label,reach_before:b.reach,reach_after:after.reach,stable_before:b.stable,stable_after:after.stable,restored:restored.map(x=>x.id)};
      renderRunHistory();
      notice(`Recovery Guard: regression detected; restored ${restored.length} persistent snapshot(s).`,'error');
    }catch(e){
      recoveryGuardLastResult={state:'ERROR',error:e.message};renderRunHistory();notice(`Recovery Guard: ${e.message}`,'error');
    }finally{
      recoveryGuardRunning=false;renderAutoPick();
    }
  }

  const runHistoryKey='routerforge.nfqws.run-history.v1',runHistoryMax=24;
  function clamp01(value){return Math.max(0,Math.min(1,Number(value)||0))}
  function runHistoryLoad(){
    try{const parsed=JSON.parse(localStorage.getItem(runHistoryKey)||'[]');runHistory=Array.isArray(parsed)?parsed.slice(0,runHistoryMax):[]}catch(_){runHistory=[]}
    renderRunHistory();renderAutoPick();
  }
  function runHistoryPersist(){
    let data=runHistory.slice(0,runHistoryMax);
    try{localStorage.setItem(runHistoryKey,JSON.stringify(data));runHistory=data;renderAutoPick()}
    catch(_){data=data.slice(0,Math.max(4,Math.floor(data.length/2)));try{localStorage.setItem(runHistoryKey,JSON.stringify(data));runHistory=data}catch(__){}}
  }
  function latencyScore(ms){
    const n=Number(ms);if(!Number.isFinite(n))return 0;if(n<=250)return 1;if(n>=2000)return 0;return 1-(n-250)/1750;
  }
  function summarizeTestLabHistory(){
    const data=testLabResultsData||[],attempts=data.flatMap(x=>x.attempts||[]),okПопытки=attempts.filter(x=>x.ok).length,stable=data.filter(x=>x.attempts?.length&&x.attempts.every(a=>a.ok)).length,successfulLat=attempts.filter(x=>x.ok).map(x=>Number(x.ms)).filter(Number.isFinite),median=medianNumber(successfulLat),decisive=data.filter(x=>['INCLUDED','EXCLUDED','INCLUDED BY MODE'].includes(String(x.static?.verdict||''))).length;
    const successRatio=attempts.length?okПопытки/attempts.length:0,stableRatio=data.length?stable/data.length:0,staticRatio=data.length?decisive/data.length:0,score=Math.round(100*(.50*successRatio+.25*stableRatio+.15*latencyScore(median)+.10*staticRatio));
    return{score,score_type:'ACTIVE SCORE',metrics:{targets:data.length,attempts:attempts.length,success_ratio:successRatio,stable_ratio:stableRatio,median_ms:median,static_decisive_ratio:staticRatio},results:data};
  }
  function summarizeSandboxHistory(){
    const data=sandboxResultsData||[],analysis=sandboxCandidateAnalysis(sandboxLoadedLists||{}),bad=analysis.checks.filter(x=>x.severity==='bad').length,warn=analysis.checks.filter(x=>x.severity==='warn').length,refs=[...analysis.refs.hostlist,...analysis.refs.hostlistExclude,...analysis.refs.ipset,...analysis.refs.ipsetExclude],known=new Set(Object.keys(sandboxLoadedLists||{})),resolved=refs.filter(x=>known.has(x)).length,refRatio=refs.length?resolved/refs.length:1,determinate=data.filter(x=>['MATCH','EXCLUDED','NO MATCH'].includes(String(x.candidate?.state||''))).length,determinateRatio=data.length?determinate/data.length:0,validation=bad?0:warn?.75:1,readiness=Math.round(100*(.50*validation+.25*refRatio+.25*determinateRatio)),baselineOk=data.filter(x=>x.baseline?.ok).length,baselineLat=data.filter(x=>x.baseline?.ok).map(x=>Number(x.baseline.ms)).filter(Number.isFinite);
    return{score:readiness,score_type:'READINESS',metrics:{targets:data.length,candidate_match:data.filter(x=>x.candidate?.state==='MATCH').length,candidate_excluded:data.filter(x=>x.candidate?.state==='EXCLUDED').length,candidate_no_match:data.filter(x=>x.candidate?.state==='NO MATCH').length,validation_warnings:warn,reference_resolution_ratio:refRatio,determinate_ratio:determinateRatio,active_baseline_success_ratio:data.length?baselineOk/data.length:0,active_baseline_median_ms:medianNumber(baselineLat)},candidate:analysis.value,results:data};
  }
  function saveCompletedRun(kind){
    let summary;if(kind==='testlab'){if(!testLabResultsData.length)return;summary=summarizeTestLabHistory()}else if(kind==='sandbox'){if(!sandboxResultsData.length)return;summary=summarizeSandboxHistory()}else return;
    const entry={id:`${Date.now()}-${kind}`,kind,created_at:new Date().toISOString(),active_config_sha256:current?.config_sha256||'',active_mode:currentMode(),runtime_running:!!current?.running,score:summary.score,score_type:summary.score_type,metrics:summary.metrics,scope:kind==='testlab'?'active-router-runtime-observation':'candidate-semantic-readiness',payload:kind==='testlab'?{results:summary.results}:{candidate:summary.candidate,results:summary.results}};
    runHistory.unshift(entry);runHistory=runHistory.slice(0,runHistoryMax);runHistoryPersist();renderRunHistory();
  }
  function percentText(value){return `${Math.round(clamp01(value)*100)}%`}
  function scoreClass(score){return score>=80?'good':score>=55?'warn':'bad'}
  function runHistoryPrevious(entry){
    const same=runHistory.filter(x=>x.kind===entry.kind),i=same.findIndex(x=>x.id===entry.id);return i>=0&&i+1<same.length?same[i+1]:null;
  }
  function runHistoryDelta(entry){
    const prev=runHistoryPrevious(entry);if(!prev)return{cls:'same',text:'первый'};const d=Number(entry.score)-Number(prev.score);return{cls:d>0?'up':d<0?'down':'same',text:d>0?`+${d}`:`${d}`};
  }
  function runHistoryKindLabel(kind){
    switch(String(kind||'')){
      case'testlab':return'TEST LAB LEGACY';
      case'sandbox':return'SANDBOX LEGACY';
      case'v2-prodlab':return'TEST LAB V2';
      case'v2-sandbox':return'SANDBOX V2';
      case'v2-selector':return'AUTOSELECT V2';
      default:return String(kind||'RUN').toUpperCase();
    }
  }
  function runHistoryChips(entry){
    const m=entry.metrics||{};
    if(entry.kind==='testlab')return[`targets ${m.targets||0}`,`reach ${percentText(m.success_ratio)}`,`stable ${percentText(m.stable_ratio)}`,`median ${m.median_ms==null?'—':Math.round(m.median_ms)+'ms'}`,`static ${percentText(m.static_decisive_ratio)}`];
    if(entry.kind==='sandbox')return[`targets ${m.targets||0}`,`match ${m.candidate_match||0}`,`refs ${percentText(m.reference_resolution_ratio)}`,`determinate ${percentText(m.determinate_ratio)}`,`baseline ${percentText(m.active_baseline_success_ratio)}`];
    if(entry.kind==='v2-selector')return[`candidates ${m.candidates||0}`,`working ${m.working||0}`,`success ${percentText(m.success_ratio)}`,`recommend ${m.recommendation?'yes':'no'}`];
    return[`targets ${m.targets||0}`,`attempts ${m.attempts||0}`,`success ${percentText(m.success_ratio)}`];
  }
  function runHistoryDetails(entry){
    const m=entry.metrics||{};
    if(entry.kind==='testlab')return [['Reachability',percentText(m.success_ratio)],['Stable targets',percentText(m.stable_ratio)],['Median latency',m.median_ms==null?'—':`${Math.round(m.median_ms)} ms`],['Static decisive',percentText(m.static_decisive_ratio)]];
    if(entry.kind==='sandbox')return [['Candidate MATCH',m.candidate_match||0],['Candidate EXCLUDED',m.candidate_excluded||0],['Candidate NO MATCH',m.candidate_no_match||0],['Validation warnings',m.validation_warnings||0],['List refs resolved',percentText(m.reference_resolution_ratio)],['Deterministic classification',percentText(m.determinate_ratio)],['Active baseline success',percentText(m.active_baseline_success_ratio)],['Active baseline median',m.active_baseline_median_ms==null?'—':`${Math.round(m.active_baseline_median_ms)} ms`]];
    if(entry.kind==='v2-selector')return [['Candidates tested',m.candidates||0],['Working candidates',m.working||0],['Candidate success ratio',percentText(m.success_ratio)],['Recommendation',m.recommendation?'AVAILABLE':'NONE']];
    return [['Targets',m.targets||0],['Попытки',m.attempts||0],['Success ratio',percentText(m.success_ratio)]];
  }
  function renderRunHistory(){
    if(disposed)return;
    const settings=getRecoverySettings(),baseline=latestRecoveryBaselineForCurrent();
    const rows=testLabEvidenceByConfig(),winner=rows.find(autoPickEligible)||null;
    const entries=runHistory.map(entry=>{
      const delta=runHistoryDelta(entry),date=new Date(entry.created_at);
      return {id:entry.id,kind:entry.kind,label:runHistoryKindLabel(entry.kind),stamp:Number.isNaN(date.getTime())?String(entry.created_at||''):date.toLocaleString(),
        sha:String(entry.active_config_sha256||''),mode:String(entry.active_mode||'—'),score:Number(entry.score)||0,scoreType:String(entry.score_type||'SCORE'),
        scoreClass:scoreClass(Number(entry.score)||0),delta,scope:String(entry.scope||'—'),running:!!entry.runtime_running,
        chips:runHistoryChips(entry),details:runHistoryDetails(entry)};
    });
    emit(onHistory,{runs:entries,count:entries.length,latestScore:entries.length?`${entries[0].score} ${entries[0].scoreType==='READINESS'?'R':'S'}`:'—',
      sizeKb:(Math.max(.1,new Blob([JSON.stringify(runHistory)]).size/1024)).toFixed(1),
      autoPick:rows.slice(0,6).map(row=>({...row,name:autoPickMapName(row.sha),eligible:autoPickEligible(row),winner:winner?.sha===row.sha})),
      winner:winner?autoPickMapName(winner.sha):'',
      guard:{enabled:!!settings.enabled,armed:!!settings.enabled&&!!baseline,running:recoveryGuardRunning,last:recoveryGuardLastResult}});
  }
  // Exact ID operations; no HTML onclick handlers or hidden server action.
  async function historyAction(name,id){
    if(disposed)return;
    if(name==='refresh')return renderAutoPick();
    if(name==='settings')return renderRunHistory();
    if(name==='clear')return clearRunHistory();
    if(name==='copy')return copyRunHistory();
    if(name==='copy-one'){
      const entry=runHistory.find(x=>x.id===id);
      if(!entry)return;
      try{if(!navigator.clipboard?.writeText)throw new Error('Clipboard API недоступен');await navigator.clipboard.writeText(JSON.stringify(entry,null,2));notice('JSON прогона скопирован.','good')}
      catch(e){notice(`История JSON: ${e.message}`,'error')}
      return;
    }
    if(name==='delete-one'){
      if(!runHistory.some(x=>x.id===id))return;
      runHistory=runHistory.filter(x=>x.id!==id);runHistoryPersist();renderRunHistory();return;
    }
    throw Error('NFQWS_HISTORY_ACTION_UNSUPPORTED');
  }
  function clearRunHistory(){
    if(!runHistory.length||!confirm(`Удалить ${runHistory.length} записей локальной истории?`))return;runHistory=[];runHistoryPersist();renderRunHistory();notice('Локальная история прогонов очищена.','good');
  }
  async function copyRunHistory(){
    if(!runHistory.length)return;
    try{if(!navigator.clipboard?.writeText)throw Error('Clipboard API недоступен');await navigator.clipboard.writeText(JSON.stringify({exported_at:new Date().toISOString(),schema:'routerforge-nfqws-run-history-v1',runs:runHistory},null,2));notice('История прогонов скопирована в JSON.','good')}
    catch(e){notice(`История JSON: ${e.message}`,'error')}
  }

  let domainCheckOpen=false,domainCheckRows=[],domainCheckBusy=false;
  function emitDomainCheck(){
    if(disposed)return;
    const ok=domainCheckRows.filter(x=>x.state==='ok').length,fail=domainCheckRows.filter(x=>x.state==='fail').length;
    const done=ok+fail,total=domainCheckRows.length;
    emit(onDomainCheck,{open:domainCheckOpen,rows:domainCheckRows.map(x=>({...x})),total,ok,fail,
      progress:total?Math.round(done/total*100):0,busy:domainCheckBusy});
  }
  async function runDomainCheck(){
    if(disposed)return;
    const domains=extractDomains(getListEditor()?.readContext?.()?.value||'');
    if(!domains.length){notice('В выбранном списке не найдено доменов.','error');return}
    stopDomainCheck();
    const gen=++checkGeneration,c=new AbortController();checkAbort=c;
    const stopOnUnmount=()=>c.abort();lifecycle.signal.addEventListener('abort',stopOnUnmount,{once:true});
    domainCheckOpen=true;domainCheckBusy=true;
    domainCheckRows=domains.map(domain=>({domain,state:'pending',code:''}));emitDomainCheck();
    let next=0;
    async function worker(){while(gen===checkGeneration&&!c.signal.aborted){
      const i=next++;if(i>=domains.length)return;
      let state='fail',code='';
      try{const result=await api('/check',{method:'POST',body:JSON.stringify({url:domains[i]}),signal:c.signal});state=result.reachable?'ok':'fail';code=String(result.status_code||'')}
      catch(e){if(c.signal.aborted)return;code=String(e?.message||'probe failed')}
      if(gen!==checkGeneration||disposed||c.signal.aborted)return;
      domainCheckRows[i]={domain:domains[i],state,code};emitDomainCheck();
    }}
    try{await Promise.all(Array.from({length:Math.min(4,domains.length)},worker))}
    finally{
      lifecycle.signal.removeEventListener('abort',stopOnUnmount);
      if(checkAbort===c)checkAbort=null;
      if(gen===checkGeneration){domainCheckBusy=false;emitDomainCheck()}
    }
  }
  function stopDomainCheck(){
    ++checkGeneration;checkAbort?.abort();checkAbort=null;domainCheckBusy=false;emitDomainCheck();
  }
  function closeCheck(){stopDomainCheck();domainCheckOpen=false;emitDomainCheck()}
  function domainCheckAction(name){
    if(disposed)return;
    if(name==='close')return closeCheck();
    if(name==='stop')return stopDomainCheck();
    if(name==='retry'&&!domainCheckBusy)return runDomainCheck();
    throw Error('NFQWS_DOMAIN_CHECK_ACTION_UNSUPPORTED');
  }

  function renderDPIDetector(){
    // Only the fetch/controller remains here. Svelte owns the status badges and diagnostics.
    if(!disposed)emit(onDpiStatus,{snapshot:dpiDetectorSnapshot,loading:dpiDetectorLoading,error:dpiDetectorError});
    emit(onHeight,);
  }

  async function loadDPIDetector(force=false){
    if(dpiDetectorLoading||dpiDetectorLoaded&&!force){renderDPIDetector();return}
    dpiDetectorLoading=true;dpiDetectorError='';renderDPIDetector();
    try{dpiDetectorSnapshot=await api('/dpi-detector');dpiDetectorLoaded=true;renderDPIDetector()}
    catch(e){dpiDetectorSnapshot=null;dpiDetectorError=String(e?.message||e);notice(`DPI Detector integration: ${dpiDetectorError}`,'error');renderDPIDetector()}
    finally{dpiDetectorLoading=false;renderDPIDetector()}
  }

  // Processing Map requests, live NFQUEUE, Guided Diagnostics, presentation and
  // controls are now exclusively Svelte-owned. Keep the existing static list
  // comparison algorithm as a read-only service until the pure matcher moves.
  // Strategy Import's parse/preview, inactive Config Library handoff, profile
  // import and Sandbox selection are owned by NfqwsStrategyImport.svelte.
  // No legacy DOM read/write or direct save handlers remain for this screen.

  // Domain Test and Production Test Lab now belong to Svelte: requests,
  // results, cancellation, concurrency and inputs do not touch legacy DOM.
  // Strategy import still needs the exact historical tokenizer; the
  // Strategy Library cache remains shared with other workflows.
  // strategy and profile data helpers; no DOM ownership remains here.
  function v2OpenSandboxFromArgs(argsText,name='Imported candidate',source='custom'){
    emit(onSandboxSeed,{kind:'candidate',args:String(argsText||''),name:String(name||''),source:String(source||'custom')});
    activateNFQWSTab('sandbox-v2');
  }

  // Native NfqwsSelector.svelte owns all Selector requests and UI. Both
  // Selector and Auto Setup use the shared Svelte controller registration.

  function v2HistoryAdd(kind,metrics,payload){
    if(!Array.isArray(runHistory))return;
    const successRatio=Number(metrics?.success_ratio||0),score=Math.round(Math.max(0,Math.min(1,successRatio))*100);
    const entry={id:`${Date.now()}-${kind}`,kind,created_at:new Date().toISOString(),active_config_sha256:current?.config_sha256||'',active_mode:currentMode(),runtime_running:!!current?.running,score,score_type:'SUCCESS%',metrics:metrics||{},scope:'nfqws-v2-verified-workflow',payload};
    runHistory.unshift(entry);runHistory=runHistory.slice(0,runHistoryMax);runHistoryPersist();renderRunHistory();
  }



  // TCP16 observation memory and five-source Strategy Registry are now
  // read-only Svelte views. The legacy controller never renders their HTML.

  // Observed Targets inventory, scan, ignore and list CAS now belong to NfqwsObserved.svelte.

  async function openV2Workspace(tab){
    if(tab==='domain-v2')return;
    if(tab==='auto-setup-v1')return; // Svelte Auto Setup owns source and observed loading
    if(tab==='prodlab-v2')return; // native Svelte Production Test Lab owns inventory
    if(tab==='selector-v2')return; // Native Svelte Selector owns its library and exact status read
    if(tab==='strategy-library-v2')return; // Native Svelte Strategy Library owns load/refresh
  }



  // Domain Test and Production Test Lab register event handlers in Svelte.
  // The runtime does not read #v2Domain* or #v2Lab* fields.
  // Native NfqwsSandbox.svelte owns form, bench run/cancel, results and save.
  // Native NfqwsSelector.svelte owns form, polling, bind, preview, apply and abort.
  // Native Strategy Library owns editor, candidate CRUD, Save/Delete/Sandbox controls.



  function handleEditorSaveShortcut(e,activeTab){
    // Active tab is the Svelte owner's state, never guessed from legacy DOM classes.
    if(disposed || !(e.ctrlKey||e.metaKey) || String(e.key||'').toLowerCase()!=='s')return;
    if(activeTab !== 'strategies')return; // Native editors own their own Ctrl+S.
    e.preventDefault();e.stopPropagation();
    updateFlowState();
    if(flowDirty())previewFlow();
  }

  const nfqwsConfigTabs=new Set(['strategies','configs','config','lists','geo-assets']);
  const nfqwsStrategyTabs=new Set(['strategy-library-v2','strategy-registry-v1','strategy-import']);
  const nfqwsMenuTabs=new Set(['catalog','upstream-lists','blobs']);
  const nfqwsSelectionTabs=new Set(['observed-v1','auto-setup-v1','selector-v2','sandbox-v2','target-memory-v2']);
  const nfqwsToolTabs=new Set(['preflight','dpi-detector','domain-v2','prodlab-v2','clienthello']);
  const nfqwsMaintenanceTabs=new Set(['runs','logs','backups','runtime-health-v2','scripts','keenetic']);
  let nfqwsActiveConfig='strategies';
  let nfqwsActiveStrategy='strategy-library-v2';
  let nfqwsActiveMenu='catalog';
  let nfqwsActiveTool='domain-v2';
  let nfqwsActiveSelection='selector-v2';
  let nfqwsActiveMaintenance='runs';

  function activateNFQWSTab(tab){
    const isConfig=nfqwsConfigTabs.has(tab),isStrategy=nfqwsStrategyTabs.has(tab),isMenu=nfqwsMenuTabs.has(tab),isTool=nfqwsToolTabs.has(tab),isSelection=nfqwsSelectionTabs.has(tab),isMaintenance=nfqwsMaintenanceTabs.has(tab);
    if(isConfig)nfqwsActiveConfig=tab;
    if(isStrategy)nfqwsActiveStrategy=tab;
    if(isMenu)nfqwsActiveMenu=tab;
    if(isTool)nfqwsActiveTool=tab;
    if(isSelection)nfqwsActiveSelection=tab;
    if(isMaintenance)nfqwsActiveMaintenance=tab;
    emit(onNavigate,tab);
    // Tab visibility and active page are exclusively owned by NFQWSManagerApp.svelte.

    if(tab==='upstream-lists')loadListSources();

    // ClientHello native owner reads capabilities only when the tab is active.
    if(tab==='preflight')renderPreflight();
    if(tab==='dpi-detector')loadDPIDetector(true);
    // Native Svelte Observed Targets owns its inventory and loads only when active.
    if(tab==='runs'){renderRunHistory();renderAutoPick()}
    if(tab==='catalog')loadCatalog();
    if(tab==='blobs')loadBlobs();
    if(tab==='backups')loadBackupCenter();
    // Target Memory Svelte owner loads only when active.
    if(tab==='keenetic')loadDPIDetector()
    if(['domain-v2','prodlab-v2','auto-setup-v1','selector-v2','sandbox-v2','strategy-library-v2'].includes(tab))void openV2Workspace(tab);


    emit(onHeight,); // Svelte-owned adapter coalesces layout refresh into one animation frame.
  }

  // Navigation controls are wired in Svelte; business side effects remain here.
  // Svelte Backup Center owns its buttons and search UI.

  // DPI v5 UI, console, legend, NDJSON, Stop and report presentation are Svelte-owned.

  // Strategy Import controls and source syntax are now Svelte-owned.


  // History/Auto-pick and Recovery Guard settings are Svelte-owned.
  // Catalog and Blobs controls are registered in their Svelte owners.
  // Smart Apply modal and controls are Svelte-owned.

  // Setup buttons, resource links and Diff modal now belong to Svelte.
  // Native NfqwsListEditor owns file rows, editor, Save/Delete/Rename/Create,
  // keyboard handling, syntax and fullscreen. Auxiliary analytical dialogs
  // continue behind an explicit read-only bridge, not DOM onclick handlers.
  // Entry Inspector and Cross-List Analysis are Svelte-owned. No legacy dialog
  // DOM writes or event assignment remain. Static Path stays in legacy until moved.
  // Domain Check modal/buttons are Svelte-owned; abort on Stop/Close/unmount.

  runHistoryLoad();
  load();
  return {navigate: activateNFQWSTab, control: action, refresh: load, historyAction, domainCheckAction, setupAction,
    handleShortcut: handleEditorSaveShortcut,
    handleEscape(e) { if (!disposed && e.key === 'Escape' && smartApplyOpen) closeSmartApply(); },
    openDomainTarget(target){
      if(disposed||typeof target!=='string'||!/^[a-z0-9.-]{1,253}$/i.test(target))throw Error('NFQWS_DOMAIN_TARGET_INVALID');
      emit(onDomainSeed,{target});activateNFQWSTab('domain-v2');
    },
    dpiRefresh(){if(!disposed)return loadDPIDetector(true);},
    rawConfigChanged(status){if(!disposed&&status?.config!==undefined)render(status);},
    blobsChanged(){blobLoaded=false;void loadBlobs(true);},
    listsChanged(){listSourceLoaded=false;void loadListSources(true);},
    configLibraryChanged({activated=false}={}){configLibraryLoaded=false;catalogLoaded=false;if(activated)void load();},
    strategyLibraryRequest(path,options){return api(path,options);},
    labHistory(kind,metrics,payload){
      if(disposed||kind!=='v2-prodlab'||!Array.isArray(payload))throw Error('NFQWS_LAB_HISTORY_INVALID');
      return v2HistoryAdd(kind,metrics,payload);
    },
    selectorHistory(kind,metrics,payload){
      if(disposed||kind!=='v2-selector'||!payload||typeof payload!=='object')throw Error('NFQWS_SELECTOR_HISTORY_INVALID');
      return v2HistoryAdd(kind,metrics,payload);
    },
    selectorApplied(status){
      if(disposed||!status||typeof status!=='object')throw Error('NFQWS_SELECTOR_STATUS_INVALID');
      current=status;original=status.config||original;render(current);
    },
    sandboxHistory(kind,metrics,payload){
      if(disposed||kind!=='v2-sandbox'||!Array.isArray(payload))throw Error('NFQWS_SANDBOX_HISTORY_INVALID');
      return v2HistoryAdd(kind,metrics,payload);
    },
    strategyLibrarySandbox(candidate){
      if(disposed||!candidate||typeof candidate.args!=='string'||!candidate.args.trim())return;
      v2OpenSandboxFromArgs(candidate.args,String(candidate.name||'Library candidate'),String(candidate.source||'custom'));
    },
    listDraftChanged(snapshot){
      if(disposed)return;
      selectedList=String(snapshot?.selected||'');
      listMeta=snapshot?.meta||null;
      listOriginal=String(snapshot?.original||'');
      updateListButtons();
    },
    processingStaticPath(domain,contents,sha){
      if(disposed||!current)throw Error('PROCESSING_RUNTIME_NOT_READY');
      const value=String(domain||'');
      if(!/^[a-z0-9.-]{1,253}$/.test(value))throw Error('PROCESSING_DOMAIN_INVALID');
      const active=String(current.config_sha256||'').toLowerCase(),expected=String(sha||'').toLowerCase();
      if(!/^[a-f0-9]{64}$/.test(active)||active!==expected)throw Error('PROCESSING_STATIC_PATH_STALE_CONFIG_SHA');
      return buildStaticPath({file:'<query>',line:0,value,domain:value,type:'domain'},contents);
    },
    listRequest(path,options){return api(path,options);},
    listStatus(status){if(status&&!disposed)render(status);},
    preflightRefresh(){if(!disposed)return refreshPreflightRuntime();},
    listToolsAction(name,payload={}){
      if(disposed)return;
      if(name==='inspectorClose')return closeEntryInspector();
      if(name==='inspectorRerun')return runEntryInspector();
      if(name==='staticPathClose')return closeStaticPath();
      if(name==='staticPathRerun')return runStaticPath();
      if(name==='analysisClose')return closeAnalysis();
      if(name==='analysisRerun')return runListAnalysis();
      if(name==='analysisCleanup')return cleanupAnalysisSide(payload.side,payload.peer);
      if(name==='openLine')return openListAtLine(payload.name,payload.line);
      throw Error('NFQWS_LIST_ANALYSIS_ACTION_UNKNOWN');
    },
    listTool(name){
      if(disposed)return;
      if(name==='cursor')return updateListButtons();
      if(name==='inspect')return runEntryInspector();
      if(name==='path')return runStaticPath();
      if(name==='analysis')return runListAnalysis();
      if(name==='check')return runDomainCheck();
      throw Error('NFQWS_LIST_TOOL_UNKNOWN');
    },
    customEdited(){if(!disposed&&current)updateFlowState();},
    flowFieldsChanged(){if(!disposed&&current)updateFlowState();},
    openResource(kind,name){
      if(disposed||!safeResourceName(name))throw Error('NFQWS_RESOURCE_NAME_INVALID');
      if(kind==='list')return openListFromSetup(name);
      if(kind==='lua')return openScriptFromSetup(name);
      throw Error('NFQWS_RESOURCE_KIND_INVALID');
    },
    flowModeChanged(kind,value){
      if(disposed||!current)throw Error('NFQWS_FLOW_NOT_READY');
      if(kind==='domain'){
        flowValues=updateModeDraft(flowValues,kind,value);
        updateModeCards();
      }else if(kind==='policy'){
        if(!['none','include','exclude'].includes(value))throw Error('NFQWS_POLICY_MODE_INVALID');
        setPolicyMode(value);
      }else throw Error('NFQWS_MODE_KIND_UNKNOWN');
      updateFlowState();
    },
    async catalogAction(name,id){
      if(disposed)return;
      if(name==='refresh'){
        if(catalogLoading)return;
        catalogLoaded=false;catalogSelected='';catalogSelectedData=null;catalogError='';
        ++catalogSelectEpoch;return loadCatalog(true);
      }
      if(name==='select')return selectCatalogItem(String(id||''));
      if(name==='sandbox'){
        if(!catalogSelectedData||catalogSelectionBusy)return;
        const d=catalogSelectedData,candidate=assignmentValue(d.content,'NFQWS_ARGS');
        if(!candidate){notice('В выбранном config NFQWS_ARGS пуст — нечего отправлять в Sandbox.','error');return;}
        v2OpenSandboxFromArgs(candidate,d.item.name,'catalog');
        notice(`Candidate ${d.item.name} загружен в V2 Sandbox. Active config не изменён.`,'good');return;
      }
      if(name==='smartApply'){
        if(!catalogSelectedData||catalogSelectionBusy)return;
        return openSmartApply();
      }
      if(name==='blobs'){activateNFQWSTab('blobs');return;}
      if(name==='local'){
        if(catalogSelectedData?.item.kind!=='local')return;
        const item=catalogSelectedData.item;
        activateNFQWSTab('configs');emit(onConfigOpen,item.name);return;
      }
      throw Error('UNKNOWN_NFQWS_CATALOG_COMMAND');
    },
    async blobAction(name,id){
      if(disposed)return;
      if(name==='refresh'){if(blobLoading||blobActionBusy)return;blobLoaded=false;return loadBlobs(true);}
      if(name==='catalog'){activateNFQWSTab('catalog');return;}
      if(blobActionBusy||blobLoading)return;
      blobActionBusy=true;renderBlobs();
      try{
        if(name==='validate')return await validateInstalledClientHello(String(id||''));
        if(name==='install')return await installCatalogBlob(String(id||''));
        if(name==='delete')return await deleteBlob(String(id||''));
        throw Error('UNKNOWN_NFQWS_BLOB_COMMAND');
      }finally{blobActionBusy=false;renderBlobs();}
    },
    async smartApply(name){
      if(disposed)return;
      if(name==='refresh')return refreshSmartApply();
      if(name==='close')return closeSmartApply();
      if(name==='apply')return applySmartApply();
      throw Error('UNKNOWN_NFQWS_SMART_APPLY_COMMAND');
    },
    async listSources(name,id){
      if(disposed)return;
      if(name==='refresh'){listSourceLoaded=false;return loadListSources(true);}
      if(name==='open')return openListFromSetup(String(id||''));
      if(name==='install')return installListSource(String(id||''));
      throw Error('UNKNOWN_NFQWS_LIST_SOURCE_COMMAND');
    },
    async backup(name,id){
      if(disposed||backupCenterActionBusy)return;
      backupCenterActionBusy=true;
      try{
        if(name==='refresh')return await loadBackupCenter(true);
        if(name==='select')return await selectBackupCenter(String(id||''));
        if(name==='prune')return await pruneBackupCenter();
        if(name==='restore')return await restoreBackupCenter();
        if(name==='delete')return await deleteBackupCenter();
        throw Error('UNKNOWN_NFQWS_BACKUP_COMMAND');
      }finally{backupCenterActionBusy=false;}
    },
    dispose(){
      if(disposed)return;
      disposed=true;
      lifecycle.abort();
    }
  };
}
