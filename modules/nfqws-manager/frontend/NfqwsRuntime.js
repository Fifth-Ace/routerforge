// Exact NFQWS business/controller extraction from historical inline script.
// Svelte creates the DOM before this module starts.
// Incremental migration: actions/async CAS/rollback remain unchanged.
export function startNFQWSRuntime() {
  const marker='/ui/', pos=location.pathname.indexOf(marker);
  const base=pos>=0?location.pathname.slice(0,pos):'/api/modules/nfqws-manager';
  const $=id=>document.getElementById(id);
  let current=null, original='', busy=false, selectedList='', listOriginal='', listMeta=null, logMode='all', checkGeneration=0, policyMode='none', scriptsLoaded=false, scriptsLoading=false, scriptFiles=[], selectedScript='', scriptContent='', analysisFindings=[], analysisFilter='all', analysisRunning=false, analysisSources={}, analysisHashes={}, analysisPeer='', analysisSelectedDirty=false, entryInspectorRunning=false, entryInspectorRecord=null, entryInspectorFindings=[], staticPathRunning=false, staticPathRecord=null, staticPathResult=null, runHistoryFilter='all', runHistory=[], recoveryGuardRunning=false, recoveryGuardLastResult=null, listSourceLoaded=false, listSourceLoading=false, listSourceFiles=[], listSourceUpstream=[], listSourceRef='', listSourceError='', catalogLoaded=false, catalogLoading=false, catalogItems=[], catalogFilter='all', catalogSelected='', catalogSelectedData=null, catalogUpstream={state:'idle',ref:'main',sha:'',error:''}, blobLoaded=false, blobLoading=false, blobFiles=[], blobRoot='', blobManifestCache={}, blobExpectedMap={}, smartApplyPlan=null, smartApplyBusy=false, backupCenterLoaded=false, backupCenterLoading=false, backupCenterFiles=[], backupCenterSelected='', backupCenterSelectedData=null, backupCenterRoot='', keeneticLoaded=false, keeneticLoading=false, keeneticSnapshot=null, dpiDetectorLoaded=false, dpiDetectorLoading=false, dpiDetectorSnapshot=null, processingMapRunning=false, processingMapResult=null;
  let processingMapLiveRunning=false, processingMapLiveResult=null, processingMapStrategyInventory=null;
  let dpiWorkspaceMode='console',dpiWebRunning=false,dpiWebResult=null,dpiWebProgress=null;
  let dpiNativeRunning=false,dpiNativeResult=null,dpiNativeStartedAt=0,dpiNativeTimer=0,dpiNativeVisible=true;
  let dpiLiveText='',dpiLiveMeta=null,dpiLiveRenderPending=false;
  let configLibraryLoaded=false, configLibraryLoading=false, configLibraryFiles=[], neighborConfigFiles=[], selectedStoredConfig='', storedConfigOriginal='', storedConfigMeta=null, configNameMode='', configNameSource='';
  let listGroupCollapsed={system:false,user:false,opkg:false};

  const esc=(s='')=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const lines=s=>String(s||'').split(/\r?\n/);
  const bytes=n=>{n=Number(n||0);if(n<1024)return `${n} B`;if(n<1048576)return `${(n/1024).toFixed(1)} KB`;return `${(n/1048576).toFixed(1)} MB`};
  const mix=(a,b,t)=>{const A=a.replace('#',''),B=b.replace('#','');const p=(s,i)=>parseInt(s.slice(i,i+2),16);return '#'+[0,2,4].map(i=>Math.round(p(A,i)*(1-t)+p(B,i)*t).toString(16).padStart(2,'0')).join('')};
  const lum=h=>{const x=h.replace('#','');return [0,2,4].map(i=>parseInt(x.slice(i,i+2),16)).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0)/255};

  function applyRouterForgeTheme(){
    const q=new URLSearchParams(location.search), root=document.documentElement;
    const palettes={
      forge:{background:'#0b0d10',text:'#f5f7fa',surface:'#12151a',surface2:'#171b21',hover:'#1d2229',muted:'#8d98a4',border:'#29313a',borderStrong:'#36414d'},
      midnight:{background:'#08111b',text:'#edf5ff',surface:'#0f1824',surface2:'#152131',hover:'#1b2a3c',muted:'#8ca0b5',border:'#25384a',borderStrong:'#34516a'},
      graphite:{background:'#101113',text:'#f1f2f4',surface:'#17191c',surface2:'#1d2024',hover:'#25292e',muted:'#9299a2',border:'#30353c',borderStrong:'#414850'}
    };
    const theme=q.get('theme')||'forge', accent=q.get('accent')||'#38bdf8';
    let p=palettes[theme]||palettes.forge;
    if(theme==='custom'){
      const bg=q.get('background')||'#0b0d10', text=q.get('text')||'#f5f7fa', light=lum(bg)>.55;
      p={background:bg,text,surface:mix(bg,text,light?.045:.055),surface2:mix(bg,text,light?.09:.105),hover:mix(bg,text,light?.13:.15),muted:mix(text,bg,.48),border:mix(bg,text,light?.18:.19),borderStrong:mix(bg,text,light?.27:.29)};
    }
    const st=root.style;
    for(const [k,v] of Object.entries({'--bg':p.background,'--surface':p.surface,'--surface2':p.surface2,'--hover':p.hover,'--text':p.text,'--muted':p.muted,'--border':p.border,'--border-strong':p.borderStrong,'--accent':accent,'--code':mix(p.background,p.text,lum(p.background)>.55?.015:.018)}))st.setProperty(k,v);
    root.dataset.density=q.get('density')||'normal';
    root.style.setProperty('--radius',({sharp:'3px',default:'8px',soft:'12px'})[q.get('radius')||'default']);
    try{root.lang=q.get('locale')||'ru'}catch(_){}
  }
  applyRouterForgeTheme();
  loadListGroupState();

  const syntaxEditors=new WeakMap();
  const nfqwsValueWords=new Set(['http','https','tls','quic','tcp','udp','wireguard','stun','discord','mtproto','unknown','fake','circular','multisplit','hostfakesplit','http_methodeol','tls_client_hello','quic_initial','http_req','MODE_LIST','MODE_ALL','MODE_AUTO']);
  const systemLists=new Set(['user.list','auto.list','exclude.list','ipset.list','ipset_exclude.list']);

  function syn(cls,value){return `<span class="${cls}">${esc(value)}</span>`}
  function highlightArgsLine(line){
    const text=String(line||''),trimmed=text.trimStart(),lead=text.slice(0,text.length-trimmed.length);
    if(trimmed.startsWith('#'))return esc(lead)+syn(/^#\s*(?:Strategy|Стратегия)\b/i.test(trimmed)?'syn-strategy':'syn-comment',trimmed);
    let out='',i=0;
    while(i<text.length){
      const c=text[i];
      if(/\s/.test(c)){let j=i+1;while(j<text.length&&/\s/.test(text[j]))j++;out+=esc(text.slice(i,j));i=j;continue}
      if(text.startsWith('--',i)){let j=i+2;while(j<text.length&&/[A-Za-z0-9_-]/.test(text[j]))j++;out+=syn('syn-flag',text.slice(i,j));i=j;continue}
      if(c==='$'){
        let j=i+1;if(text[j]==='{'){j++;while(j<text.length&&/[A-Za-z0-9_]/.test(text[j]))j++;if(text[j]==='}')j++}else while(j<text.length&&/[A-Za-z0-9_]/.test(text[j]))j++;
        out+=syn('syn-variable',text.slice(i,j));i=j;continue
      }
      if(c==='#'){out+=syn(/^#\s*(?:Strategy|Стратегия)\b/i.test(text.slice(i))?'syn-strategy':'syn-comment',text.slice(i));break}
      if((c==='@'&&text[i+1]==='/')||c==='/'){
        let j=i+1;while(j<text.length&&!/[\s,:]/.test(text[j]))j++;out+=syn('syn-resource',text.slice(i,j));i=j;continue
      }
      if(c==='"'||c==="'"){
        const q=c;let j=i+1,escaped=false;for(;j<text.length;j++){const x=text[j];if(x===q&&!escaped){j++;break}escaped=x==='\\'&&!escaped;if(x!=='\\')escaped=false}out+=syn('syn-string',text.slice(i,j));i=j;continue
      }
      if('=:,'.includes(c)){out+=syn('syn-operator',c);i++;continue}
      if(/[0-9]/.test(c)||((c==='-'||c==='+'||c==='<'||c==='>')&&/[A-Za-z0-9]/.test(text[i+1]||''))){
        let j=i+1;while(j<text.length&&/[A-Za-z0-9._+<>-]/.test(text[j]))j++;const v=text.slice(i,j);out+=syn(/^[-+<>]?[A-Za-z]*\d/.test(v)?'syn-number':'syn-value',v);i=j;continue
      }
      let j=i+1;while(j<text.length&&!/[\s=:,#"']/.test(text[j]))j++;const v=text.slice(i,j);
      let cls='syn-value';if(/^0x[0-9a-f]+$/i.test(v)||/^\d+(?:[-:]\d+)*(?:,\d+(?:[-:]\d+)*)*$/.test(v))cls='syn-number';else if(v.includes('/')||(/^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)+$/.test(v)&&/[A-Za-z]/.test(v)))cls='syn-resource';else if(text[j]==='=')cls='syn-param';else if(!nfqwsValueWords.has(v)&&!/^[A-Za-z_][A-Za-z0-9_.-]*$/.test(v))cls='syn-string';
      out+=syn(cls,v);i=j;
    }
    return out;
  }
  function highlightArgs(text){return lines(text).map(highlightArgsLine).join('\n')||'&#8203;'}
  function closingQuoteIndex(text,quote,start){let escaped=false;for(let i=start;i<text.length;i++){const c=text[i];if(c===quote&&!escaped)return i;escaped=c==='\\'&&!escaped;if(c!=='\\')escaped=false}return-1}
  function highlightConfig(text){
    const src=lines(text),out=[];let quoted=false,quote='"';
    for(const line of src){
      if(quoted){const end=closingQuoteIndex(line,quote,0);if(end<0){out.push(highlightArgsLine(line));continue}out.push(highlightArgsLine(line.slice(0,end))+syn('syn-string',quote)+highlightArgsLine(line.slice(end+1)));quoted=false;continue}
      const t=line.trimStart(),lead=line.slice(0,line.length-t.length);if(t.startsWith('#')){out.push(esc(lead)+syn('syn-comment',t));continue}
      const m=/^(\s*)([A-Za-z_][A-Za-z0-9_]*)(\s*)(=)(.*)$/.exec(line);if(!m){out.push(highlightArgsLine(line));continue}
      let rendered=esc(m[1])+syn('syn-key',m[2])+esc(m[3])+syn('syn-operator',m[4]),rest=m[5],prefix='';const ws=/^\s*/.exec(rest)?.[0]||'';prefix+=esc(ws);rest=rest.slice(ws.length);
      if(rest[0]==='"'||rest[0]==="'"){quote=rest[0];rendered+=prefix+syn('syn-string',quote);const end=closingQuoteIndex(rest,quote,1);if(end<0){quoted=true;rendered+=highlightArgsLine(rest.slice(1))}else rendered+=highlightArgsLine(rest.slice(1,end))+syn('syn-string',quote)+highlightArgsLine(rest.slice(end+1));}
      else rendered+=prefix+highlightArgsLine(rest);
      out.push(rendered);
    }
    return out.join('\n')||'&#8203;';
  }
  function refreshSyntaxEditor(textarea){
    const state=syntaxEditors.get(textarea);if(!state)return;state.pre.innerHTML=state.mode==='config'?highlightConfig(textarea.value):highlightArgs(textarea.value);state.pre.scrollTop=textarea.scrollTop;state.pre.scrollLeft=textarea.scrollLeft;if(state.resources)renderEditorResources(textarea,state.resources,state.resourceKind);
  }
  function insertTab(textarea){const a=textarea.selectionStart,b=textarea.selectionEnd;textarea.value=textarea.value.slice(0,a)+'  '+textarea.value.slice(b);textarea.selectionStart=textarea.selectionEnd=a+2;textarea.dispatchEvent(new Event('input',{bubbles:true}))}
  function closeSyntaxFullscreen(shell){if(!shell)return;shell.classList.remove('syntax-fullscreen');if(!document.querySelector('.syntax-fullscreen')&&!document.querySelector('.full-editor')&&!document.querySelector('.script-fullscreen')){document.body.classList.remove('syntax-fullscreen-open');document.documentElement.classList.remove('syntax-fullscreen-open')}reportHeight()}
  function toggleStrategyFullscreen(shell,button){const on=!shell.classList.contains('syntax-fullscreen');document.querySelectorAll('.syntax-fullscreen').forEach(x=>x!==shell&&closeSyntaxFullscreen(x));shell.classList.toggle('syntax-fullscreen',on);document.body.classList.toggle('syntax-fullscreen-open',on);document.documentElement.classList.toggle('syntax-fullscreen-open',on);button.textContent=on?'Закрыть fullscreen':'На весь экран';reportHeight()}
  function mountSyntaxTextarea(textarea,{mode='args',caption='NFQWS args',resources=true,resourceKind='strategy'}={}){
    if(!textarea)return null;const existing=syntaxEditors.get(textarea);if(existing){refreshSyntaxEditor(textarea);return existing}
    const shell=document.createElement('div');shell.className='syntax-editor-shell';const bar=document.createElement('div');bar.className='syntax-editor-toolbar';bar.innerHTML=`<div class="syntax-editor-caption"><strong>${esc(caption)}</strong><span class="syntax-editor-hint">NFQWS syntax · Ctrl+S → Diff</span></div>`;
    const body=document.createElement('div');body.className='syntax-editor-body';const pre=document.createElement('pre');pre.className='syntax-editor-highlight';const resourceHost=document.createElement('div');resourceHost.className='resource-chips';resourceHost.hidden=!resources;
    textarea.parentNode.insertBefore(shell,textarea);shell.append(bar,body);body.append(pre,textarea);if(resources)shell.append(resourceHost);
    const state={pre,mode,shell,resources:resources?resourceHost:null,resourceKind};syntaxEditors.set(textarea,state);textarea.dataset.syntaxReady='1';
    textarea.addEventListener('input',()=>refreshSyntaxEditor(textarea));textarea.addEventListener('scroll',()=>{pre.scrollTop=textarea.scrollTop;pre.scrollLeft=textarea.scrollLeft});textarea.addEventListener('keydown',e=>{if(e.key==='Tab'){e.preventDefault();insertTab(textarea)}if(e.key==='Escape'&&shell.classList.contains('syntax-fullscreen'))closeSyntaxFullscreen(shell)});
    refreshSyntaxEditor(textarea);return state;
  }
  function resizeRawEditor(){
    const textarea=$('config');if(!textarea)return;const panel=textarea.closest('.editor-shell');
    if(panel?.classList.contains('full-editor')){textarea.style.height='';return}
    if(!textarea.offsetParent)return;const before=textarea.offsetHeight;textarea.style.height='auto';const next=Math.max(500,textarea.scrollHeight);textarea.style.height=`${next}px`;textarea.scrollTop=0;const state=syntaxEditors.get(textarea);if(state)state.pre.scrollTop=0;if(Math.abs(before-next)>1)requestAnimationFrame(reportHeight);
  }
  function mountRawSyntaxEditor(){
    const textarea=$('config');if(!textarea)return;let state=syntaxEditors.get(textarea);if(!state){const body=document.createElement('div');body.className='syntax-editor-body';const pre=document.createElement('pre');pre.className='syntax-editor-highlight';textarea.parentNode.insertBefore(body,textarea);body.append(pre,textarea);state={pre,mode:'config',shell:body,resources:null};syntaxEditors.set(textarea,state);textarea.dataset.syntaxReady='1';textarea.addEventListener('input',()=>refreshSyntaxEditor(textarea));textarea.addEventListener('scroll',()=>{pre.scrollTop=textarea.scrollTop;pre.scrollLeft=textarea.scrollLeft})}
    const panel=textarea.closest('.editor-shell');if(panel){panel.id='configEditorPanel';const toolbar=panel.querySelector('.editor-toolbar');if(toolbar&&!$('fullscreenConfig')){const hint=toolbar.lastElementChild,actions=document.createElement('div');actions.className='editor-toolbar-actions';if(hint)actions.append(hint);const button=document.createElement('button');button.id='fullscreenConfig';button.type='button';button.textContent='На весь экран';actions.append(button);toolbar.append(actions);button.onclick=()=>{const on=!panel.classList.contains('full-editor');panel.classList.toggle('full-editor',on);document.body.classList.toggle('syntax-fullscreen-open',on);button.textContent=on?'Закрыть fullscreen':'На весь экран';resizeRawEditor();reportHeight()}}}
    refreshSyntaxEditor(textarea);resizeRawEditor();
  }
  function baseResourceDefinitions(){
    const source=String(flowValues.NFQWS_BASE_ARGS||''),blobs=new Map(),lua=[];let m;const br=/--blob=([^:\s]+):(@?\/[^\s]+)/g;while((m=br.exec(source))!==null)blobs.set(m[1],m[2]);const lr=/--lua-init=(@?\/[^\s]+)/g;while((m=lr.exec(source))!==null)lua.push(m[1]);return{blobs,lua};
  }
  function editorResources(text,kind){
    const source=String(text||''),items=[],seen=new Set(),push=(type,name,path='')=>{const key=`${type}:${name}:${path}`;if(!seen.has(key)){seen.add(key);items.push({type,name,path})}};let m;
    if(kind==='base'){const defs=baseResourceDefinitions();for(const[pathName,path]of defs.blobs)push('blobdef',pathName,path);for(const path of defs.lua)push('lua',path.split('/').pop(),path);return items}
    const lr=/--(?:hostlist|hostlist-exclude|hostlist-auto|ipset|ipset-exclude)=([^\s]+)/g;while((m=lr.exec(source))!==null){const path=m[1].replace(/["']/g,'');push('list',path.split('/').pop(),path)}
    const br=/(?:^|:)blob=([^:\s,]+)/gm;while((m=br.exec(source))!==null)push('blob',m[1]);return items;
  }
  function renderEditorResources(textarea,host,kind){
    const items=editorResources(textarea.value,kind);if(!items.length){host.hidden=true;host.innerHTML='';return}host.hidden=false;const known=new Set((current?.lists||[]).map(x=>String(x.name||''))),defs=baseResourceDefinitions();host.innerHTML='<span class="resource-label">Ресурсы</span>'+items.map(item=>{
      if(item.type==='list'){const ok=known.has(item.name);return `<button type="button" class="resource-chip ${ok?'good':'warn'}" data-resource-list="${esc(item.name)}" title="${esc(item.path)}">list ${esc(item.name)} ${ok?'✓':'?'}</button>`}
      if(item.type==='blob'){const inline=/^0x[0-9a-f]+$/i.test(item.name),ok=inline||defs.blobs.has(item.name);return `<span class="resource-chip ${ok?'good':'warn'}" title="${esc(ok?(inline?'inline blob':defs.blobs.get(item.name)||'declared'):'Нет объявления в NFQWS_BASE_ARGS')}">blob ${esc(item.name)} ${ok?'✓':'?'}</span>`}
      if(item.type==='blobdef')return `<span class="resource-chip info" title="${esc(item.path)}">blob ${esc(item.name)}</span>`;
      return `<button type="button" class="resource-chip info" data-resource-script="${esc(item.name)}" title="${esc(item.path)}">lua ${esc(item.name)}</button>`;
    }).join('');host.querySelectorAll('[data-resource-list]').forEach(button=>button.onclick=()=>openListFromSetup(button.dataset.resourceList));host.querySelectorAll('[data-resource-script]').forEach(button=>button.onclick=()=>openScriptFromSetup(button.dataset.resourceScript));
  }
  function renderAllEditorResources(){document.querySelectorAll('textarea[data-syntax-ready="1"]').forEach(ta=>{const state=syntaxEditors.get(ta);if(state?.resources)renderEditorResources(ta,state.resources,state.resourceKind)})}
  function mountStrategyEditors(){
    const specs=[['flowBaseArgs','BASE_ARGS','base'],['flowArgs','HTTPS / TCP','strategy'],['flowQuic','QUIC','strategy'],['flowUdp','UDP','strategy'],['flowIpset','IPSET','strategy']];for(const[id,caption,kind]of specs)mountSyntaxTextarea($(id),{caption,resourceKind:kind});
    document.querySelectorAll('.custom-block[data-index]').forEach(card=>{const i=Number(card.dataset.index),caption=normalizeCustomLabel(customLabels[i])||`CUSTOM #${i+1}`;mountSyntaxTextarea(card.querySelector('textarea'),{caption,resourceKind:'strategy'})});renderAllEditorResources();
  }


  const workspaceHeightKeys={listWorkspace:'routerforge.nfqws.workspace.lists.height',scriptWorkspace:'routerforge.nfqws.workspace.scripts.height'};
  function setupResizableWorkspace(id){
    const node=$(id);if(!node)return;const key=workspaceHeightKeys[id];let saved=0;try{saved=Number(localStorage.getItem(key)||0)}catch(_){}if(saved>=420&&saved<=900)node.style.height=`${Math.round(saved)}px`;
    let timer=null;const observer=new ResizeObserver(()=>{const h=Math.round(node.getBoundingClientRect().height);if(h<420||h>900)return;clearTimeout(timer);timer=setTimeout(()=>{try{localStorage.setItem(key,String(h))}catch(_){}reportHeight()},120)});observer.observe(node);
  }
  function setupResizableWorkspaces(){setupResizableWorkspace('listWorkspace');setupResizableWorkspace('scriptWorkspace')}
  setupResizableWorkspaces();

  async function api(path,options={}){
    const r=await fetch(base+path,{cache:'no-store',credentials:'same-origin',headers:{Accept:'application/json','Content-Type':'application/json'},...options});
    const text=await r.text();let body={};try{body=text?JSON.parse(text):{}}catch{}
    if(!r.ok)throw new Error(body.error||`${path} HTTP ${r.status}`);return body;
  }
  function notice(text,type=''){$('notice').textContent=text;$('notice').className='notice'+(type?` ${type}`:'')}
  function setBusy(v){busy=v;for(const id of ['start','stop','reload','restart','save'])$(id).disabled=v||(id==='save'&&(!current?.mutation_ready||$('config').value===original));updateListButtons()}
  let heightReportRaf=0;
  function reportHeight(){
    if(heightReportRaf)cancelAnimationFrame(heightReportRaf);
    heightReportRaf=requestAnimationFrame(()=>{
      heightReportRaf=0;
      try{const shell=document.querySelector('.shell'),height=Math.ceil((shell?.scrollHeight||shell?.getBoundingClientRect().height||720)+4);parent.postMessage({type:'routerforge-module-height',moduleId:'nfqws-manager',height},location.origin)}catch(_){}
    });
  }
  new ResizeObserver(reportHeight).observe(document.querySelector('.shell')||document.body);window.addEventListener('load',reportHeight);window.addEventListener('resize',reportHeight);

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
  let flowOriginal='',flowValues={},customBlocks=[],customOriginalBlocks=[],customLabels=[];

  function customLabelStoreKey(){const target=String(current?.config_path||current?.target_id||'default');return `routerforge.nfqws.custom-labels.v1:${target}`}
  function normalizeCustomLabel(value){return String(value||'').replace(/[\r\n]+/g,' ').trim().slice(0,120)}
  function parseCustomLabelMetadata(config){
    const labels=[],re=/^[ \t]*# RouterForge: NFQWS_ARGS_CUSTOM label\[(\d+)\]=(.*)$/gm;let m,count=0;
    while((m=re.exec(String(config||'')))!==null){const i=Number(m[1])-1;if(i>=0){labels[i]=normalizeCustomLabel(m[2]);count++}}
    return{labels,count};
  }
  function legacyCustomLabels(){try{const data=JSON.parse(localStorage.getItem(customLabelStoreKey())||'[]');return Array.isArray(data)?data:[]}catch(_){return[]}}
  function clearLegacyCustomLabels(){try{localStorage.removeItem(customLabelStoreKey())}catch(_){}}
  function loadCustomLabels(config){
    const stored=parseCustomLabelMetadata(config),legacy=stored.count?[]:legacyCustomLabels();customLabels=stored.count?stored.labels:legacy;
    while(customLabels.length<customBlocks.length)customLabels.push('');
    if(customLabels.length>customBlocks.length)customLabels=customLabels.slice(0,customBlocks.length);
    customLabels=customLabels.map(normalizeCustomLabel);
  }
  function replaceCustomLabelMetadata(config){
    const clean=String(config||'').replace(/^[ \t]*# RouterForge: NFQWS_ARGS_CUSTOM label\[\d+\]=.*(?:\r?\n|$)/gm,'');
    const a=scanAssignment(clean,'NFQWS_ARGS_CUSTOM');if(!a)return clean;
    const lineStart=clean.lastIndexOf('\n',Math.max(0,a.start-1))+1;
    const meta=customLabels.map((value,i)=>{const label=normalizeCustomLabel(value);return label?`# RouterForge: NFQWS_ARGS_CUSTOM label[${i+1}]=${label}`:''}).filter(Boolean);
    if(!meta.length)return clean;
    return clean.slice(0,lineStart)+meta.join('\n')+'\n'+clean.slice(lineStart);
  }
  function parseCustom(value){return String(value||'').split(/\s+--new(?:\s+|$)/).map(x=>x.trim()).filter(Boolean)}
  function buildCustom(){return customBlocks.map(x=>x.trim()).filter(Boolean).join('\n--new\n')}
  function customBlocksChanged(){return JSON.stringify(customBlocks.map(x=>x.trim()))!==JSON.stringify(customOriginalBlocks.map(x=>x.trim()))}
  function parsePorts(value){return String(value||'').split(',').map(x=>x.trim()).filter(Boolean)}
  function validPortToken(value){return /^\d{1,5}(?::\d{1,5})?$/.test(value)&&value.split(':').every(x=>Number(x)>=1&&Number(x)<=65535)}
  function renderPortEditor(kind){
    const field=kind==='tcp'?'flowTcpPorts':'flowUdpPorts',host=kind==='tcp'?'tcpPortChips':'udpPortChips';
    $(host).innerHTML=parsePorts($(field).value).map((p,i)=>`<span class="port-chip">${esc(p)}<button data-kind="${kind}" data-port-index="${i}" title="Удалить">×</button></span>`).join('')||'<span class="muted small">Порты не заданы — этот протокол не попадёт в NFQUEUE.</span>';
    document.querySelectorAll(`[data-kind="${kind}"][data-port-index]`).forEach(b=>b.onclick=()=>{const arr=parsePorts($(field).value);arr.splice(Number(b.dataset.portIndex),1);$(field).value=arr.join(',');renderPortEditor(kind);updateFlowState()});
  }
  function addPort(kind){
    const input=$(kind==='tcp'?'tcpPortAdd':'udpPortAdd'),field=$(kind==='tcp'?'flowTcpPorts':'flowUdpPorts'),v=input.value.trim();
    if(!validPortToken(v)){notice(`Некорректный ${kind.toUpperCase()} порт или диапазон: ${v}`,'error');return}
    const arr=parsePorts($(field).value);if(!arr.includes(v))arr.push(v);$(field).value=arr.join(',');input.value='';renderPortEditor(kind);updateFlowState();
  }
  function currentMode(){return String(flowValues.NFQWS_EXTRA_ARGS||'').replace(/^\$/,'').replace(/[{}]/g,'')}
  function renderPolicy(){
    const hasName=String(flowValues.POLICY_NAME||'').trim()!=='';
    policyMode=!hasName?'none':(/^(1|true|yes)$/i.test(flowValues.POLICY_EXCLUDE||'')?'exclude':'include');
    document.querySelectorAll('.policy-card').forEach(card=>{const on=card.dataset.policyMode===policyMode;card.classList.toggle('active',on);const r=card.querySelector('input');if(r)r.checked=on});
    $('policyNameRow').style.display=policyMode==='none'?'none':'grid';
  }
  function setPolicyMode(mode){
    policyMode=mode;
    if(mode==='none'){flowValues.POLICY_NAME='';flowValues.POLICY_EXCLUDE='0';$('flowPolicyName').value='';$('flowPolicyExclude').value='0'}
    else{flowValues.POLICY_EXCLUDE=mode==='exclude'?'1':'0';$('flowPolicyExclude').value=flowValues.POLICY_EXCLUDE}
    renderPolicy();updateFlowState();
  }
  function openListFromSetup(name){
    const tab=document.querySelector('.tab[data-tab="lists"]');if(tab)tab.click();
    if((current?.lists||[]).some(x=>x.name===name))selectList(name);else notice(`Список ${name} не найден на роутере.`,'error');
  }
  const setupCollapseTimers=new WeakMap();
  function clearSetupCollapseTimer(section){const timer=setupCollapseTimers.get(section);if(timer){clearTimeout(timer);setupCollapseTimers.delete(section)}}
  function addCollapseControls(){
    document.querySelectorAll('.setup-section').forEach((section,i)=>{
      const head=section.querySelector(':scope > .setup-head');if(!head||section.dataset.fixed==='true')return;
      section.classList.add('collapsible-section');
      if(section.dataset.collapseReady==='1')return;
      const body=section.querySelector(':scope > .setup-body');if(!body)return;
      const collapse=document.createElement('div'),inner=document.createElement('div');collapse.className='setup-section-collapse';inner.className='setup-section-collapse-inner';body.parentNode.insertBefore(collapse,body);collapse.append(inner);inner.append(body);
      section.dataset.collapseReady='1';
      if(i>=4){section.classList.add('collapsed');collapse.hidden=true}
      const sync=()=>head.setAttribute('aria-expanded',section.classList.contains('collapsed')||section.classList.contains('closing')?'false':'true');
      const open=()=>{clearSetupCollapseTimer(section);section.classList.remove('collapsed','closing');collapse.hidden=false;collapse.classList.remove('closing');void collapse.offsetHeight;collapse.classList.add('opening');sync();reportHeight();const timer=setTimeout(()=>{collapse.classList.remove('opening');setupCollapseTimers.delete(section);reportHeight()},230);setupCollapseTimers.set(section,timer)};
      const close=()=>{clearSetupCollapseTimer(section);collapse.hidden=false;collapse.classList.remove('opening');section.classList.add('closing');void collapse.offsetHeight;collapse.classList.add('closing');sync();reportHeight();const timer=setTimeout(()=>{section.classList.remove('closing');section.classList.add('collapsed');collapse.classList.remove('closing');collapse.hidden=true;setupCollapseTimers.delete(section);sync();reportHeight()},230);setupCollapseTimers.set(section,timer)};
      const toggle=()=>{if(section.classList.contains('collapsed')||section.classList.contains('closing'))open();else close()};
      head.setAttribute('role','button');head.tabIndex=0;sync();
      head.onclick=e=>{if(e.target.closest('button,input,textarea,select,a,label'))return;toggle()};
      head.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();toggle()}};
    });
  }
  function renderProcessingOverview(){
    const mode=currentMode(),policy=policyMode==='none'?'весь трафик':policyMode==='include'?`только ${flowValues.POLICY_NAME||'политика'}`:`кроме ${flowValues.POLICY_NAME||'политики'}`;
    const cards=[['TCP',flowValues.TCP_PORTS||'выкл'],['UDP',flowValues.UDP_PORTS||'выкл'],['Домены',mode==='MODE_LIST'?'user.list':mode==='MODE_ALL'?'всё кроме exclude':mode==='MODE_AUTO'?'user + auto':'не определено'],['IPv6',/^(1|true|yes)$/i.test(flowValues.IPV6_ENABLED||'')?'включён':'выключен'],['Политика',policy]];
    $('processingOverview').innerHTML=cards.map(([k,v])=>`<article class="card"><div class="metric-label">${esc(k)}</div><div class="metric-value">${esc(v)}</div></article>`).join('');
  }
  function referencedListNames(){
    const set=new Set(),source=[flowValues.NFQWS_ARGS,flowValues.NFQWS_ARGS_QUIC,flowValues.NFQWS_ARGS_UDP,flowValues.NFQWS_ARGS_IPSET,...customBlocks].join(' ');
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
    const seenCustom=new Map();customBlocks.forEach((value,i)=>{
      const name=normalizeCustomLabel(customLabels[i])||`#${i+1}`,raw=String(value||'').trim(),normalized=raw.replace(/\s+/g,' ');
      if(!raw)out.push(['bad',`Custom ${name}: блок пуст.`]);
      if(/(^|\s)--new(?:\s|$)/.test(raw))out.push(['bad',`Custom ${name}: --new внутри одного UI-блока запрещён. Используй отдельные блоки.`]);
      if(normalized){if(seenCustom.has(normalized))out.push(['warn',`Custom ${name}: полный дубль блока ${seenCustom.get(normalized)}.`]);else seenCustom.set(normalized,name)}
    });
    if(!out.some(x=>x[0]==='bad'))out.push(['good','Критических противоречий перед сохранением не найдено.']);
    return out;
  }
  function renderDiagnostics(){
    const items=diagnostics(),bad=items.filter(x=>x[0]==='bad').length,warn=items.filter(x=>x[0]==='warn').length,good=items.filter(x=>x[0]==='good').length;
    const state=bad?'BLOCKED':warn?'CHECK':'READY',stateClass=bad?'bad':warn?'warn':'good';
    $('diagnosticSummary').innerHTML=`<div class="diagnostic-state ${stateClass}"><span>${state}</span><strong>${bad?`${bad} ошибок`:warn?`${warn} предупреждений`:'можно применять'}</strong></div><div class="diagnostic-counts"><span class="bad">Ошибки <b>${bad}</b></span><span class="warn">Предупреждения <b>${warn}</b></span><span class="good">Проверки <b>${good}</b></span></div>`;
    $('flowDiagnostics').innerHTML=items.map(([type,msg])=>`<div class="diag ${type}"><span class="diag-mark">${type==='bad'?'!':type==='warn'?'!':'✓'}</span><span>${esc(msg)}</span></div>`).join('');
  }
  function changedKeys(){const next=flowConfig(),out=[];for(const key of flowKeys)if(assignmentValue(flowOriginal,key)!==assignmentValue(next,key))out.push(key);const before=parseCustomLabelMetadata(flowOriginal).labels.map(normalizeCustomLabel),after=customLabels.map(normalizeCustomLabel);if(JSON.stringify(before)!==JSON.stringify(after))out.push('CUSTOM_LABELS');return out}
  function renderChangeSummary(){const keys=changedKeys();$('changeSummary').innerHTML=keys.slice(0,8).map(k=>`<span class="change-chip">${esc(k)}</span>`).join('')+(keys.length>8?`<span class="change-chip">+${keys.length-8}</span>`:'')}
  function loadFlow(config){
    flowOriginal=config;flowValues=Object.fromEntries(flowKeys.map(k=>[k,assignmentValue(config,k)]));customBlocks=parseCustom(flowValues.NFQWS_ARGS_CUSTOM);customOriginalBlocks=[...customBlocks];loadCustomLabels(config);
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
  function summaryChips(value){
    const tokens=String(value||'').match(/(?:[^\s"']+|"[^"]*"|'[^']*')+/g)||[],chips=[];
    const pick=(name,label,klass='')=>{const p=`--${name}=`;const t=tokens.find(x=>x.startsWith(p));if(t)chips.push(`<span class="setup-chip ${klass}">${esc(label)} ${esc(t.slice(p.length))}</span>`)};
    pick('filter-tcp','TCP','good');pick('filter-udp','UDP','good');pick('filter-l7','L7');pick('payload','payload');pick('lua-desync','lua','warn');pick('dpi-desync','desync','warn');pick('hostlist','hostlist');pick('ipset','ipset');
    return chips.join('')||'<span class="setup-chip">Без краткого summary</span>';
  }
  function renderCollapsedSummaries(){
    document.querySelectorAll('.setup-section[data-summary-key]').forEach(section=>{
      const title=section.querySelector('.setup-head>div:nth-child(2)');if(!title)return;let host=title.querySelector('.setup-head-summary');
      if(!host){host=document.createElement('div');host.className='setup-head-summary';title.append(host)}
      const key=section.dataset.summaryKey;if(key==='CUSTOM'){
        if(!customBlocks.length){host.innerHTML='<span class="setup-chip">Custom: нет</span>';return}
        const labels=customBlocks.map((_,i)=>normalizeCustomLabel(customLabels[i])||`#${i+1}`);host.innerHTML=labels.slice(0,4).map(x=>`<span class="setup-chip good">${esc(x)}</span>`).join('')+(labels.length>4?`<span class="setup-chip">+${labels.length-4}</span>`:'');
      }else host.innerHTML=summaryChips(flowValues[key]||'');
    });
  }
  function updateModeCards(){
    const mode=String(flowValues.NFQWS_EXTRA_ARGS||'').replace(/^\$/,'').replace(/[{}]/g,'');
    document.querySelectorAll('.mode-card').forEach(card=>{const active=card.dataset.mode===mode;card.classList.toggle('active',active);const input=card.querySelector('input');if(input)input.checked=active});
  }
  function renderCustomBlocks(){
    $('customBlocks').innerHTML=customBlocks.length?customBlocks.map((value,i)=>`<div class="custom-block" data-index="${i}">
      <div class="custom-block-head"><div><div class="custom-block-title">${esc(customLabels[i]||`Custom-блок ${i+1}`)}</div><div class="muted small">NFQWS_ARGS_CUSTOM${customBlocks.length>1?` · позиция ${i+1}/${customBlocks.length}`:''}</div></div>
      <div class="custom-block-actions"><button class="custom-up" ${i===0?'disabled':''}>↑</button><button class="custom-down" ${i===customBlocks.length-1?'disabled':''}>↓</button><button class="custom-clone">Клонировать</button><button class="custom-delete danger">Удалить</button></div></div>
      <div class="custom-label-row"><label>Название блока</label><input class="custom-label-input" value="${esc(customLabels[i]||'')}" maxlength="120" placeholder="Например: YouTube / основной обход"></div>
      <div class="custom-live-summary setup-chips">${summaryChips(value)}</div>
      <textarea spellcheck="false">${esc(value)}</textarea>
      <div class="label-help">Название сохраняется в raw-конфиге как безопасный комментарий <span class="mono"># RouterForge:</span> и не передаётся nfqws2 как аргумент.</div>
    </div>`).join(''):'<div class="empty">Custom-стратегий нет. Это нормально — базовые HTTPS / QUIC / UDP выше работают независимо.</div>';
    document.querySelectorAll('.custom-block[data-index]').forEach(card=>{
      const i=Number(card.dataset.index),ta=card.querySelector('textarea'),label=card.querySelector('.custom-label-input');
      ta.oninput=e=>{customBlocks[i]=e.target.value;card.querySelector('.custom-live-summary').innerHTML=summaryChips(customBlocks[i]);updateFlowState()};
      label.oninput=e=>{customLabels[i]=normalizeCustomLabel(e.target.value);card.querySelector('.custom-block-title').textContent=customLabels[i]||`Custom-блок ${i+1}`;updateFlowState()};
      card.querySelector('.custom-up').onclick=()=>{if(i>0){[customBlocks[i-1],customBlocks[i]]=[customBlocks[i],customBlocks[i-1]];[customLabels[i-1],customLabels[i]]=[customLabels[i],customLabels[i-1]];renderFlow()}};
      card.querySelector('.custom-down').onclick=()=>{if(i<customBlocks.length-1){[customBlocks[i+1],customBlocks[i]]=[customBlocks[i],customBlocks[i+1]];[customLabels[i+1],customLabels[i]]=[customLabels[i],customLabels[i+1]];renderFlow()}};
      card.querySelector('.custom-clone').onclick=()=>{customBlocks.splice(i+1,0,customBlocks[i]);customLabels.splice(i+1,0,customLabels[i]?`${customLabels[i]} — копия`:'');renderFlow()};card.querySelector('.custom-delete').onclick=()=>{if(confirm(`Удалить custom-блок ${i+1}?`)){customBlocks.splice(i,1);customLabels.splice(i,1);renderFlow()}};
    });
    mountStrategyEditors();
  }
  function updateFlowState(){
    flowValues.ISP_INTERFACE=$('flowInterface').value;flowValues.NFQWS_BASE_ARGS=$('flowBaseArgs').value;flowValues.NFQWS_ARGS=$('flowArgs').value;flowValues.NFQWS_ARGS_QUIC=$('flowQuic').value;flowValues.NFQWS_ARGS_UDP=$('flowUdp').value;flowValues.NFQWS_ARGS_IPSET=$('flowIpset').value;
    flowValues.IPV6_ENABLED=$('flowIPv6').checked?'1':'0';flowValues.TCP_PORTS=$('flowTcpPorts').value;flowValues.UDP_PORTS=$('flowUdpPorts').value;flowValues.POLICY_NAME=policyMode==='none'?'':$('flowPolicyName').value;flowValues.POLICY_EXCLUDE=policyMode==='exclude'?'1':'0';flowValues.LOG_LEVEL=$('flowLogLevel').checked?'1':'0';flowValues.LOG_DEBUG_PATH=$('flowLogPath').value;
    const dirty=flowDirty();$('setupState').textContent=dirty?'UNSAVED':'CLEAN';$('setupState').className='setup-state '+(dirty?'dirty':'');$('resetSetup').disabled=!dirty;$('diffSetup').disabled=!dirty;$('applySetup').disabled=!dirty||busy||diagnostics().some(x=>x[0]==='bad');
    $('sumMain').innerHTML=summaryChips(flowValues.NFQWS_ARGS);$('sumQuic').innerHTML=summaryChips(flowValues.NFQWS_ARGS_QUIC);$('sumUdp').innerHTML=summaryChips(flowValues.NFQWS_ARGS_UDP);$('sumIpset').innerHTML=summaryChips(flowValues.NFQWS_ARGS_IPSET);renderCollapsedSummaries();
    $('strategyCount').textContent=customBlocks.length?`(${customBlocks.length})`:'';$('mStrategies').textContent=String(3+customBlocks.length);renderProcessingOverview();renderDiagnostics();renderChangeSummary();renderAllEditorResources();renderPreflight();
  }
  function renderFlow(){
    $('flowInterface').value=flowValues.ISP_INTERFACE||'';$('flowBaseArgs').value=flowValues.NFQWS_BASE_ARGS||'';$('flowArgs').value=flowValues.NFQWS_ARGS||'';$('flowQuic').value=flowValues.NFQWS_ARGS_QUIC||'';$('flowUdp').value=flowValues.NFQWS_ARGS_UDP||'';$('flowIpset').value=flowValues.NFQWS_ARGS_IPSET||'';
    $('flowIPv6').checked=/^(1|true|yes)$/i.test(flowValues.IPV6_ENABLED||'');$('flowTcpPorts').value=flowValues.TCP_PORTS||'';$('flowUdpPorts').value=flowValues.UDP_PORTS||'';$('flowPolicyName').value=flowValues.POLICY_NAME||'';$('flowPolicyExclude').value=flowValues.POLICY_EXCLUDE||'0';$('flowLogLevel').checked=/^(1|true|yes)$/i.test(flowValues.LOG_LEVEL||'');$('flowLogPath').value=flowValues.LOG_DEBUG_PATH||'';
    $('flowQueue').textContent=assignmentValue(flowOriginal,'NFQUEUE_NUM')||'—';$('flowUser').textContent=assignmentValue(flowOriginal,'USER')||'—';$('flowConfigVersion').textContent=assignmentValue(flowOriginal,'CONFIG_VERSION')||'—';
    updateModeCards();renderPolicy();renderCustomBlocks();mountStrategyEditors();updateFlowState();addCollapseControls();reportHeight();
  }
  function resetFlow(){loadFlow(flowOriginal);renderFlow()}
  function previewFlow(){
    updateFlowState();const next=flowConfig();$('diffBody').innerHTML=makeDiff(flowOriginal,next).map(x=>`<span class="${x.c}">${esc(x.t)}</span>`).join('\n')||'<span class="same">No changes</span>';$('diffModal').classList.add('open');$('diffApply').dataset.mode='flow';
  }
  async function applyFlow(){
    updateFlowState();if(!flowDirty()||busy)return;const next=flowConfig();
    if(!confirm('Сохранить настройки nfqws2.conf и выполнить безопасный reload?'))return;
    closeDiff();setBusy(true);
    try{const r=await api('/config',{method:'POST',body:JSON.stringify({content:next,confirm:'NFQWS_CONFIG'})});clearLegacyCustomLabels();render(r.status);notice(`Настройки применены. Backup: ${r.backup}`,'good')}
    catch(e){notice(e.message,'error');setBusy(false)}
  }

  function preflightJumpButton(tab,label='Открыть'){
    if(!tab)return'';return `<button type="button" class="preflight-jump" data-preflight-tab="${esc(tab)}">${esc(label)}</button>`;
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
    const anyStrategy=[flowValues.NFQWS_ARGS,flowValues.NFQWS_ARGS_QUIC,flowValues.NFQWS_ARGS_UDP,flowValues.NFQWS_ARGS_IPSET,...customBlocks].some(x=>String(x||'').trim());
    add('Traffic',anyStrategy?'good':'bad',anyStrategy?'Есть активные strategy args':'Все strategy args пусты',anyStrategy?`${3+customBlocks.length} UI strategy slots; пустые блоки могут быть допустимы по отдельности.`:'Нет ни одной стратегии, которую можно применить к трафику.','strategies');
    if(policyMode!=='none'&&!String(flowValues.POLICY_NAME||'').trim())add('Traffic','bad','Policy mode без POLICY_NAME','Выбран policy include/exclude, но имя политики пустое.','strategies');
    if(mode==='MODE_LIST')add('Lists',known.has('user.list')?'good':'bad',known.has('user.list')?'user.list найден':'user.list отсутствует','MODE_LIST требует user.list для основной выборки.','lists');
    if(mode==='MODE_AUTO'){
      add('Lists',known.has('user.list')?'good':'bad',known.has('user.list')?'user.list найден':'user.list отсутствует','MODE_AUTO использует пользовательский список.','lists');
      add('Lists',known.has('auto.list')?'good':'warn',known.has('auto.list')?'auto.list найден':'auto.list отсутствует','Автоматически накопленные домены не будут доступны.','lists');
    }
    if(mode==='MODE_ALL')add('Lists',known.has('exclude.list')?'good':'info',known.has('exclude.list')?'exclude.list найден':'exclude.list отсутствует','MODE_ALL работает и без exclude, но исключений не будет.','lists');
    for(const name of refs)add('Lists',known.has(name)?'good':'bad',known.has(name)?`Ссылка ${name} разрешена`:`Ссылка ${name} не найдена`,known.has(name)?'Файл присутствует в runtime inventory.':'Strategy args ссылаются на отсутствующий list-файл.','lists');
    const listDirty=Boolean(listMeta&&$('listEditor')&&$('listEditor').value!==listOriginal);
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
    if(!$('preflightGroups'))return;const items=buildPreflight(),bad=items.filter(x=>x.severity==='bad').length,warn=items.filter(x=>x.severity==='warn').length,good=items.filter(x=>x.severity==='good').length,state=bad?'BLOCKED':warn?'CHECK':'READY',klass=state.toLowerCase(),dirty=flowDirty();
    $('preflightVerdict').textContent=state;$('preflightVerdict').className=`preflight-verdict ${klass}`;$('preflightTabState').textContent=state==='READY'?'✓':state==='BLOCKED'?'!':'•';$('preflightScopeBadge').textContent=dirty?'DRAFT':'ACTIVE';$('preflightScopeBadge').className='config-badge '+(dirty?'empty':'active');$('preflightBad').textContent=bad;$('preflightWarn').textContent=warn;$('preflightGood').textContent=good;$('preflightSubtitle').textContent=dirty?'Проверяется текущий setup draft + свежий runtime snapshot. Ничего не применяется.':'Проверяется active canonical config + текущий runtime snapshot. Ничего не применяется.';
    const order=['Runtime','Config','Traffic','Lists','Observability','Safety'];$('preflightGroups').innerHTML=order.map(group=>{const rows=items.filter(x=>x.group===group);if(!rows.length)return'';const gb=rows.filter(x=>x.severity==='bad').length,gw=rows.filter(x=>x.severity==='warn').length,summary=gb?`${gb} BLOCK`:gw?`${gw} CHECK`:'PASS';return `<section class="preflight-group"><div class="preflight-group-head"><span class="preflight-group-title">${esc(group)}</span><span class="preflight-group-meta">${esc(summary)}</span></div><div class="preflight-items">${rows.map(x=>`<div class="preflight-item ${x.severity}"><span class="preflight-mark">${x.severity==='good'?'✓':x.severity==='bad'?'!':x.severity==='warn'?'!':'i'}</span><div><div class="preflight-item-title">${esc(x.title)}</div><div class="preflight-item-detail">${esc(x.detail)}</div></div>${preflightJumpButton(x.tab)}</div>`).join('')}</div></section>`}).join('');
    $('preflightGroups').querySelectorAll('[data-preflight-tab]').forEach(button=>button.onclick=()=>{const tab=document.querySelector(`.tab[data-tab="${button.dataset.preflightTab}"]`);if(tab)tab.click()});
  }
  async function refreshPreflightRuntime(){
    const button=$('refreshPreflight');if(button)button.disabled=true;try{const s=await api('/status');current=s;renderPreflight();notice('Preflight runtime snapshot обновлён.','good')}catch(e){notice(e.message,'error')}finally{if(button)button.disabled=false}
  }

    function renderOverview(){
    const ls=current?.lists||[];$('mService').textContent=current?.running?'RUNNING':'STOPPED';$('mService').className='metric-value '+(current?.running?'good-text':'bad-text');$('mServiceSub').textContent=current?.mutation_ready?'manager ready':'mutation not ready';
    $('mConfig').textContent=current?.config_exists?'OK':'MISSING';$('mConfig').className='metric-value '+(current?.config_exists?'good-text':'bad-text');$('mConfigSub').textContent=`${bytes(current?.config_size)} · ${(current?.config_sha256||'').slice(0,12)||'no SHA'}`;
    $('mLists').textContent=ls.length;$('mListsSub').textContent=`${bytes(ls.reduce((a,b)=>a+Number(b.size||0),0))} total`;
    $('oTarget').textContent=`${current?.target_name||'—'} (${current?.target_id||'—'})`;$('oInit').textContent=current?.init_script||'—';$('oConfigPath').textContent=current?.config_path||'—';$('oSha').textContent=current?.config_sha256||'—';$('oListsRoot').textContent=current?.lists_root||'—';$('oLogPath').textContent=current?.log_path||'—';$('oMutation').textContent=current?.mutation_ready?'Core-guarded · ready':'not ready';
    const rows=[[current?.available,'External target detected','required nfqws paths'],[current?.running,'Runtime running','nfqws process'],[current?.config_exists,'Config readable',`${bytes(current?.config_size)} · SHA verified`],[!current?.config_truncated,'Config fully loaded','full config available'],[current?.mutation_ready,'Safe mutation ready','confirm + atomic write + backup']];
    $('healthActivity').innerHTML=rows.map(([ok,t,s])=>`<div class="activity-row"><span class="dot ${ok?'good':'warn'}"></span><div><div>${esc(t)}</div><div class="muted small">${esc(s)}</div></div><span class="${ok?'good-text':'warn-text'} mono small">${ok?'PASS':'CHECK'}</span></div>`).join('');
  }
  function updateConfigEditor(){
    const v=$('config').value,c=lines(v).length;$('gutter').textContent=Array.from({length:c},(_,i)=>i+1).join('\n');$('editorStats').textContent=`${c} lines · ${bytes(new Blob([v]).size)}`;
    const dirty=v!==original;$('editorState').textContent=dirty?'UNSAVED':'CLEAN';$('editorState').className='mono '+(dirty?'dirty':'good-text');$('save').disabled=busy||!current?.mutation_ready||!dirty;$('undoConfig').disabled=!dirty;$('diffConfig').disabled=!dirty;resizeRawEditor();refreshSyntaxEditor($('config'));
  }
  function editableStatusLists(){
    return (current?.lists||[]).filter(item=>/\.(list|list-opkg|list-old)$/i.test(String(item?.name||'')));
  }
  const listGroupStateKey='routerforge.nfqws.list-groups.v1';
  function loadListGroupState(){
    try{const saved=JSON.parse(localStorage.getItem(listGroupStateKey)||'{}');for(const key of ['system','user','opkg'])if(typeof saved[key]==='boolean')listGroupCollapsed[key]=saved[key]}catch(_){}
  }
  function saveListGroupState(){try{localStorage.setItem(listGroupStateKey,JSON.stringify(listGroupCollapsed))}catch(_){}}
  function listGroup(item){
    const name=String(item?.name||'').toLowerCase();if(name.endsWith('.list-opkg'))return'opkg';const baseName=name.endsWith('.list-old')?name.slice(0,-4):name;if(systemLists.has(baseName))return'system';return'user';
  }
  function sortLists(items){
    const order={system:0,user:1,opkg:2};return [...items].sort((a,b)=>{const ga=listGroup(a),gb=listGroup(b);if(order[ga]!==order[gb])return order[ga]-order[gb];return String(a.name||'').localeCompare(String(b.name||''),undefined,{numeric:true,sensitivity:'base'})});
  }
  function renderListFiles(){
    const list=sortLists(editableStatusLists()),q=$('listSearch').value.trim().toLowerCase();$('listCount').textContent=list.length?`(${list.length})`:'';$('fileList').innerHTML='';const visible=list.filter(x=>!q||String(x.name).toLowerCase().includes(q));if(!visible.length){$('fileList').innerHTML='<div class="empty">Списки не найдены.</div>';return}
    const groups=[['system','Системные'],['user','Пользовательские'],['opkg','OPKG']];for(const[key,label]of groups){const items=visible.filter(item=>listGroup(item)===key);if(!items.length)continue;const collapsed=!q&&!!listGroupCollapsed[key],group=document.createElement('section');group.className=`file-group ${key}${collapsed?' collapsed':''}`;const header=document.createElement('button');header.type='button';header.className='file-group-header';header.innerHTML=`<span class="file-group-header-main"><span class="file-group-chevron">▼</span><span class="file-group-title">${label}</span></span><span class="file-group-count">${items.length}</span>`;header.onclick=()=>{listGroupCollapsed[key]=!listGroupCollapsed[key];saveListGroupState();renderListFiles()};const host=document.createElement('div');host.className='file-group-items';
      items.forEach(item=>{const row=document.createElement('div'),kind=listGroup(item),meta=kind==='system'?'служебный / upstream':kind==='opkg'?'package copy':'пользовательский';row.dataset.group=kind;row.className='file-item'+(selectedList===item.name?' active':'');row.innerHTML=`<div><div class="file-name"><span class="file-icon">${kind==='system'?'◆':kind==='opkg'?'◇':'≡'}</span>${esc(item.name)}</div><div class="file-meta">${esc(item.modified_at||'')} · ${meta}${String(item.name||'').toLowerCase().endsWith('.list-old')?' · old':''}</div></div><div class="file-meta">${bytes(item.size)}</div>`;row.onclick=()=>selectList(item.name);host.append(row)});group.append(header,host);$('fileList').append(group);
    }
  }
  async function selectList(name){
    selectedList=name;const item=editableStatusLists().find(x=>String(x.name)===String(name));if(item){listGroupCollapsed[listGroup(item)]=false;saveListGroupState()}renderListFiles();$('selectedListTitle').textContent=name;$('selectedListMeta').textContent='Loading…';$('listEditor').value='';listMeta=null;listOriginal='';updateListButtons();
    try{const d=await api(`/list?name=${encodeURIComponent(name)}`);listMeta=d;listOriginal=d.content||'';$('listEditor').value=listOriginal;$('selectedListMeta').textContent=`${d.path} · ${bytes(d.size)}${d.protected?' · protected':''}`;updateListButtons()}
    catch(e){notice(e.message,'error')}
  }
  function updateListGutter(){
    const editor=$('listEditor'),gutter=$('listGutter');if(!editor||!gutter)return;const count=Math.max(1,lines(editor.value).length);gutter.textContent=Array.from({length:count},(_,i)=>i+1).join('\n');gutter.scrollTop=editor.scrollTop;
  }
  function highlightListLine(line){
    const raw=String(line??''),match=raw.match(/^(\s*)(#|\/\/)(.*)$/);if(match)return `${esc(match[1])}<span class="list-comment">${esc(match[2]+match[3])}</span>`;return esc(raw)||'&#8203;';
  }
  function updateListHighlight(){
    const editor=$('listEditor'),highlight=$('listHighlight');if(!editor||!highlight)return;const rows=String(editor.value??'').split(/\r?\n/);highlight.innerHTML=rows.map(highlightListLine).join('\n')+(editor.value.endsWith('\n')?' ':'');highlight.scrollTop=editor.scrollTop;highlight.scrollLeft=editor.scrollLeft;
  }
  function updateListButtons(){
    const loaded=Boolean(listMeta),dirty=loaded&&$('listEditor').value!==listOriginal;
    updateListGutter();updateListHighlight();
    $('saveList').disabled=busy||!loaded||!dirty;$('undoList').disabled=!dirty;$('normalizeList').disabled=!loaded;$('inspectListEntry').disabled=!loaded||entryInspectorRunning||!entryRecordAtCaret();$('staticPathEntry').disabled=!loaded||staticPathRunning||!entryRecordAtCaret();$('analyzeLists').disabled=!loaded||analysisRunning;$('checkList').disabled=!loaded;$('fullscreenList').disabled=!loaded;$('deleteList').disabled=busy||!loaded||Boolean(listMeta?.protected);$('previewRenameList').disabled=busy||!loaded||dirty||Boolean(listMeta?.protected);
    if(!loaded){$('listStats').textContent='—';return}
    const arr=lines($('listEditor').value),data=arr.map(x=>x.trim()).filter(x=>x&&!x.startsWith('#')&&!x.startsWith('//'));$('listStats').textContent=`${arr.length} lines · ${data.length} entries · ${bytes(new Blob([$('listEditor').value]).size)}${dirty?' · UNSAVED':''}`;
  }
  function normalizeList(){
    const seen=new Set(),out=[];for(const line of lines($('listEditor').value)){const t=line.trim();if(!t||t.startsWith('#')||t.startsWith('//')){out.push(line);continue}if(seen.has(t))continue;seen.add(t);out.push(t)}$('listEditor').value=out.join('\n');updateListButtons();
  }
  async function saveList(){
    if(!listMeta||$('listEditor').value===listOriginal)return;if(!confirm(`Сохранить ${selectedList}?`))return;setBusy(true);
    try{const r=await api('/list/save',{method:'POST',body:JSON.stringify({name:selectedList,content:$('listEditor').value,expected_sha256:listMeta.sha256,confirm:'NFQWS_LIST_SAVE'})});render(r.status);await selectList(selectedList);notice(`Список ${selectedList} сохранён. Backup: ${r.backup}`,'good')}
    catch(e){notice(e.message,'error');setBusy(false)}
  }
  let nf5g2PreviewPlan=null;
  function closeListRenamePreview(){nf5g2PreviewPlan=null;$('applyRenameList').disabled=true;$('renamePreviewModal').classList.remove('open')}
  async function previewListRename(){
    nf5g2PreviewPlan=null;$('applyRenameList').disabled=true;
    if(!listMeta||listMeta.protected||$('listEditor').value!==listOriginal){notice('Выбери пользовательский список без несохранённых изменений.','error');return}
    const oldName=selectedList,oldSHA=listMeta.sha256;
    let next=prompt('Новое имя списка:',oldName.replace(/\.list$/i,'')+'-renamed.list');
    if(next===null)return;next=String(next).trim();if(!/\.list$/i.test(next))next+='.list';
    if(!/^[A-Za-z0-9][A-Za-z0-9._-]*\.list$/i.test(next)){notice('Новое имя должно оканчиваться на .list и содержать безопасные символы.','error');return}
    $('renamePreviewModal').classList.add('open');$('renamePreviewBody').textContent='Проверяю список и связанные конфигурации…';
    try{
      const params=new URLSearchParams({old_name:oldName,new_name:next,expected_sha256:listMeta.sha256});
      const p=await api('/list/rename-preview?'+params.toString());
      if(!p.ok||!p.preview_only||p.mutation_performed||p.old_list_sha256!==oldSHA)throw new Error('Контракт read-only Preview не подтверждён');
      if(selectedList!==oldName||listMeta?.sha256!==oldSHA)throw new Error('Открытый список изменился во время Preview');
      if((p.config_changes||[]).length>6)throw new Error('Больше 6 конфигураций — разбей переименование на меньшие операции.');nf5g2PreviewPlan=p;$('applyRenameList').disabled=false;
      const impacts=(p.config_changes||[]).map(x=>`${x.reference} · селекторов: ${x.selectors}\n  before ${x.before_sha256}\n  after  ${x.after_sha256}`);
      $('renamePreviewBody').textContent=[`Список: ${p.old_name} → ${p.new_name}`,`SHA списка: ${p.old_list_sha256}`,`Новое имя свободно: ${p.new_name_available?'ДА':'НЕТ'}`,`Ссылок для будущей замены: ${p.reference_count}`,`Затронутых конфигов: ${(p.config_changes||[]).length}`,'',...(impacts.length?impacts:['Нет ссылок в конфигурациях.']),'','PREVIEW ONLY: запись не выполнялась. Применение доступно только по отдельному подтверждению.'].join('\n');
    }catch(e){$('renamePreviewBody').textContent='Preview заблокирован: '+String(e.message||e);notice('Rename Preview: '+String(e.message||e),'error')}
  }
  async function applyListRename(){
    const p=nf5g2PreviewPlan;
    if(!p||!p.ok||!p.preview_only||p.mutation_performed||selectedList!==p.old_name||listMeta?.sha256!==p.old_list_sha256||$('listEditor').value!==listOriginal){notice('Preview устарел. Выполни предварительный просмотр заново.','error');return}
    if(!confirm(`Переименовать ${p.old_name} → ${p.new_name}?\n\nСелекторов: ${p.reference_count}; конфигов: ${(p.config_changes||[]).length}.\nПеред записью будут созданы backup. При ошибке — rollback.\n${(p.config_changes||[]).some(c=>c.reference==='active:nfqws2.conf')?'Для активной конфигурации может потребоваться перезапуск nfqws2.':''}`))return;
    $('applyRenameList').disabled=true;
    try{
      const result=await api('/list/rename-apply',{method:'POST',body:JSON.stringify({old_name:p.old_name,new_name:p.new_name,expected_sha256:p.old_list_sha256,expected_config_changes:p.config_changes,confirm:'ROUTERFORGE_LIST_RENAME_APPLY'})});
      if(!result.ok||result.rolled_back)throw new Error('Сервер не подтвердил успешное переименование');
      nf5g2PreviewPlan=null;closeListRenamePreview();render(result.status||await api('/status'));await selectList(p.new_name);
      notice(`Список ${p.old_name} → ${p.new_name}. Обновлено ссылок: ${result.updated_selectors}. Backup: ${(result.backups||[]).length}.`,'good');
    }catch(e){nf5g2PreviewPlan=null;$('applyRenameList').disabled=true;$('renamePreviewBody').textContent='Применение заблокировано или откатилось: '+String(e.message||e)+'\n\nПовторно открой Preview перед новым применением.';notice('Rename Apply: '+String(e.message||e),'error')}
  }
  async function deleteList(){
    if(!listMeta||listMeta.protected)return;if(!confirm(`Удалить ${selectedList}? Перед удалением будет создан backup.`))return;setBusy(true);
    try{const r=await api('/list/delete',{method:'POST',body:JSON.stringify({name:selectedList,expected_sha256:listMeta.sha256,confirm:'NFQWS_LIST_DELETE'})});selectedList='';listMeta=null;listOriginal='';$('listEditor').value='';render(r.status);if(editableStatusLists().length)await selectList(sortLists(editableStatusLists())[0].name);notice(`Список удалён. Backup: ${r.backup}`,'good')}
    catch(e){notice(e.message,'error');setBusy(false)}
  }
  async function createList(){
    const stem=$('newListName').value.trim();$('createError').textContent='';if(!/^[A-Za-z0-9_-]+$/.test(stem)){$('createError').textContent='Используй только буквы, цифры, _ и -';return}
    const name=`${stem}.list`;try{const r=await api('/list/create',{method:'POST',body:JSON.stringify({name,confirm:'NFQWS_LIST_CREATE'})});closeCreate();render(r.status);await selectList(name);notice(`Создан ${name}`,'good')}catch(e){$('createError').textContent=e.message}
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
  function listCaretLine(){
    const editor=$('listEditor'),pos=Math.max(0,Number(editor?.selectionStart||0));return String(editor?.value||'').slice(0,pos).split(/\r?\n/).length;
  }
  function entryRecordAtCaret(){
    if(!selectedList||!listMeta)return null;const row=listCaretLine(),raw=lines($('listEditor').value)[row-1]??'',value=normalizeListDataLine(raw);if(!value)return null;const net=parseAddressEntry(value);if(net)return{type:net.kind,file:selectedList,line:row,value,net};const domain=normalizeDomain(value);return domain?{type:'domain',file:selectedList,line:row,value,domain}:null;
  }
  function inspectEntryAgainstAll(record,all){
    const findings=[],seen=new Set();for(const[name,content]of Object.entries(all)){if(name===record.file)continue;const other=buildListRecords(name,content);if(record.type==='domain')compareDomainLists([record],other,findings,seen);else compareNetworkLists([record],other,findings,seen)}
    const rank={conflict:0,warn:1};findings.sort((a,b)=>rank[a.severity]-rank[b.severity]||a.scope.localeCompare(b.scope)||a.b.file.localeCompare(b.b.file)||a.b.line-b.b.line);return findings;
  }
  function renderEntryInspector(){
    const record=entryInspectorRecord,findings=entryInspectorFindings,conflicts=findings.filter(f=>f.severity==='conflict').length,files=new Set(findings.map(f=>f.b.file));$('entryInspectorTotal').textContent=findings.length;$('entryInspectorConflicts').textContent=conflicts;$('entryInspectorFiles').textContent=files.size;
    if(!record){$('entryInspectorValue').textContent='—';$('entryInspectorMeta').textContent='—';$('entryInspectorRole').textContent='—';$('entryInspectorResults').innerHTML='<div class="analysis-empty">Поставь курсор на домен, IP или CIDR.</div>';return}
    $('entryInspectorValue').textContent=record.value;$('entryInspectorMeta').textContent=`${record.file}:${record.line} · ${record.type==='domain'?'DOMAIN':record.type.toUpperCase()}`;$('entryInspectorScope').textContent=`${record.file}:${record.line} ↔ остальные списки`;$('entryInspectorRole').textContent=listRole(record.file).toUpperCase();$('entryInspectorRole').className='config-badge '+(listRole(record.file)==='exclude'?'empty':'active');
    $('entryInspectorResults').innerHTML=findings.length?findings.map(f=>`<div class="inspector-row ${f.severity}"><div class="inspector-kind">${esc(f.kind)}</div><div><div class="inspector-value">${esc(f.detail)}</div><div class="inspector-meta">${esc(f.b.file)}:${f.b.line} · ${f.severity==='conflict'?'include/exclude conflict':'overlap / redundancy'}</div></div><button class="inspector-open" data-inspector-file="${esc(f.b.file)}" data-inspector-line="${f.b.line}">Открыть</button></div>`).join(''):'<div class="analysis-empty">Связей с другими списками не найдено.</div>';
    $('entryInspectorResults').querySelectorAll('[data-inspector-file]').forEach(button=>button.onclick=()=>openListAtLine(button.dataset.inspectorFile,Number(button.dataset.inspectorLine)||1));
  }
  async function runEntryInspector(){
    const record=entryRecordAtCaret();if(entryInspectorRunning||!record){updateListButtons();if(!record)notice('Поставь курсор на строку с доменом, IP или CIDR.','error');return}entryInspectorRunning=true;entryInspectorRecord=record;entryInspectorFindings=[];$('entryInspectorModal').classList.add('open');$('entryInspectorProgress').textContent='Читаю соседние списки…';$('entryInspectorResults').innerHTML='<div class="analysis-empty">Ищу связи только для выбранной записи…</div>';renderEntryInspector();updateListButtons();
    try{const loaded=await loadNeighborListContents();entryInspectorFindings=inspectEntryAgainstAll(record,loaded.contents);$('entryInspectorProgress').textContent=`Проверено ${Object.keys(loaded.contents).length} списков${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;renderEntryInspector()}catch(e){$('entryInspectorProgress').textContent='Ошибка инспектора';$('entryInspectorResults').innerHTML=`<div class="analysis-empty bad-text">${esc(e.message)}</div>`}finally{entryInspectorRunning=false;updateListButtons()}
  }
  function closeEntryInspector(){$('entryInspectorModal').classList.remove('open')}

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
  function renderStaticPath(){
    const record=staticPathRecord,result=staticPathResult;if(!record||!result)return;$('staticPathValue').textContent=record.value;$('staticPathMeta').textContent=`${record.file}:${record.line} · ${record.type==='domain'?'DOMAIN':record.type.toUpperCase()}`;$('staticPathScope').textContent=`${record.file}:${record.line} · active config static analysis`;$('staticPathSourceRole').textContent=result.sourceRole.label;$('staticPathSourceRole').className='config-badge '+(result.sourceRole.impact==='exclude'?'empty':result.sourceRole.impact==='include'?'active':'');$('staticPathMode').textContent=result.mode;
    const verdict=$('staticPathVerdict');verdict.className=`static-path-verdict ${result.tone}`;verdict.querySelector('strong').textContent=result.verdict;verdict.querySelector('.static-path-reason').textContent=result.reason;
    $('staticPathSteps').innerHTML=result.hits.map((h,i)=>`<div class="static-path-step ${h.effective}"><div class="static-path-index">${i+1}</div><div><div class="static-path-role ${h.effective}">${esc(h.role.label)}</div><div class="inspector-meta">${esc(h.relation)}</div></div><div><div class="static-path-value">${esc(h.value)}</div><div class="static-path-meta">${esc(h.file)}:${h.line} · ${esc(h.role.reason)}${h.detail?` · ${esc(h.detail)}`:''}</div></div><button class="static-path-open" data-static-file="${esc(h.file)}" data-static-line="${h.line}">Открыть</button></div>`).join('')||'<div class="analysis-empty">Статических совпадений не найдено.</div>';
    $('staticPathSteps').querySelectorAll('[data-static-file]').forEach(button=>button.onclick=()=>{closeStaticPath();openListAtLine(button.dataset.staticFile,Number(button.dataset.staticLine)||1)});
  }
  async function loadStaticPathContents(){
    const items=editableStatusLists().filter(item=>item.name!==selectedList),out={[selectedList]:$('listEditor').value},queue=[...items],workers=[],errors=[];let done=0;$('staticPathProgress').textContent=`Читаю списки: 0/${items.length}`;
    async function worker(){while(queue.length){const item=queue.shift();if(!item)break;try{const d=await api(`/list?name=${encodeURIComponent(item.name)}`);out[item.name]=d.content||''}catch(e){errors.push(`${item.name}: ${e.message||'не удалось прочитать'}`)}done++;$('staticPathProgress').textContent=`Читаю списки: ${done}/${items.length}`}}
    for(let i=0;i<Math.min(4,items.length);i++)workers.push(worker());await Promise.all(workers);return{contents:out,errors};
  }
  async function runStaticPath(){
    const record=entryRecordAtCaret();if(staticPathRunning||!record){updateListButtons();if(!record)notice('Поставь курсор на строку с доменом, IP или CIDR.','error');return}staticPathRunning=true;staticPathRecord=record;staticPathResult=null;$('staticPathModal').classList.add('open');$('staticPathValue').textContent=record.value;$('staticPathMeta').textContent=`${record.file}:${record.line}`;$('staticPathScope').textContent='строю статический путь';$('staticPathSteps').innerHTML='<div class="analysis-empty">Сопоставляю active config и list-файлы…</div>';updateListButtons();
    try{const loaded=await loadStaticPathContents();staticPathResult=buildStaticPath(record,loaded.contents);$('staticPathProgress').textContent=`Проверено ${Object.keys(loaded.contents).length} списков${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;renderStaticPath()}catch(e){$('staticPathProgress').textContent='Ошибка статического анализа';$('staticPathSteps').innerHTML=`<div class="analysis-empty bad-text">${esc(e.message)}</div>`}finally{staticPathRunning=false;updateListButtons()}
  }
  function closeStaticPath(){$('staticPathModal').classList.remove('open')}
  async function loadNeighborListContents(){
    const items=editableStatusLists().filter(item=>item.name!==selectedList),out={[selectedList]:$('listEditor').value},hashes={[selectedList]:String(listMeta?.sha256||'')},queue=[...items],workers=[],errors=[];let done=0;$('analysisProgress').textContent=`Загрузка соседних списков: 0/${items.length}`;
    async function worker(){while(queue.length){const item=queue.shift();if(!item)break;try{const d=await api(`/list?name=${encodeURIComponent(item.name)}`);out[item.name]=d.content||'';hashes[item.name]=d.sha256}catch(e){errors.push(`${item.name}: ${e.message||'не удалось прочитать'}`)}done++;$('analysisProgress').textContent=`Загрузка соседних списков: ${done}/${items.length}`}}
    for(let i=0;i<Math.min(4,items.length);i++)workers.push(worker());await Promise.all(workers);return{contents:out,hashes,errors};
  }
  function analysisPeers(){
    const set=new Set();for(const f of analysisFindings){if(f.a.file===selectedList&&f.b.file!==selectedList)set.add(f.b.file);else if(f.b.file===selectedList&&f.a.file!==selectedList)set.add(f.a.file)}return [...set].sort((a,b)=>a.localeCompare(b));
  }
  function pairFindings(peer,applyFilter=true){
    const pair=analysisFindings.filter(f=>(f.a.file===selectedList&&f.b.file===peer)||(f.b.file===selectedList&&f.a.file===peer));if(!applyFilter)return pair;return pair.filter(f=>analysisFilter==='all'||analysisFilter==='conflict'&&f.severity==='conflict'||analysisFilter==='domain'&&f.scope==='domain'||analysisFilter==='network'&&f.scope==='network');
  }
  function pairSideRecords(findings,file){
    const byLine=new Map();for(const f of findings){for(const rec of [f.a,f.b]){if(rec.file!==file)continue;let row=byLine.get(rec.line);if(!row){row={line:rec.line,value:rec.value,kinds:new Set(),severity:'warn'};byLine.set(rec.line,row)}row.kinds.add(f.kind);if(f.severity==='conflict')row.severity='conflict'}}return [...byLine.values()].sort((a,b)=>a.line-b.line);
  }
  function renderAnalysisPane(hostId,rows){
    const host=$(hostId);host.innerHTML=rows.length?rows.map(row=>`<div class="analysis-line ${row.severity}" data-line="${row.line}"><span class="analysis-line-num">${row.line}</span><span class="analysis-line-value">${esc(row.value)}</span><span class="analysis-line-badge">${esc([...row.kinds].join(' · '))}</span></div>`).join(''):'<div class="analysis-empty">По текущему фильтру строк нет.</div>';
  }
  function renderAnalysis(){
    const selected=selectedList,scoped=analysisFindings.filter(f=>f.a.file===selected||f.b.file===selected),conflicts=scoped.filter(f=>f.severity==='conflict').length,warns=scoped.length-conflicts,peers=analysisPeers(),files=new Set(scoped.flatMap(f=>[f.a.file,f.b.file]));$('analysisTotal').textContent=scoped.length;$('analysisConflicts').textContent=conflicts;$('analysisWarnings').textContent=warns;$('analysisFiles').textContent=files.size;$('analysisScope').textContent=selected?`${selected} ↔ найденные пересечения`:'—';
    for(const[id,val]of[['analysisAll','all'],['analysisConflict','conflict'],['analysisDomain','domain'],['analysisNetwork','network']])$(id).classList.toggle('active',analysisFilter===val);
    if(!peers.includes(analysisPeer))analysisPeer=peers[0]||'';$('analysisPeer').innerHTML=peers.length?peers.map(name=>`<option value="${esc(name)}"${name===analysisPeer?' selected':''}>${esc(selected)} ↔ ${esc(name)}</option>`).join(''):'<option value="">Пересечений не найдено</option>';$('analysisPeer').disabled=!peers.length;$('openAnalysisPeer').disabled=!analysisPeer;
    $('analysisLeftTitle').textContent=selected||'—';$('analysisRightTitle').textContent=analysisPeer||'—';const visible=analysisPeer?pairFindings(analysisPeer,true):[],allPair=analysisPeer?pairFindings(analysisPeer,false):[],left=pairSideRecords(visible,selected),right=pairSideRecords(visible,analysisPeer);$('analysisLeftCount').textContent=`${left.length} строк`;$('analysisRightCount').textContent=`${right.length} строк`;renderAnalysisPane('analysisLeftLines',left);renderAnalysisPane('analysisRightLines',right);$('analysisPairMeta').textContent=analysisPeer?`${allPair.length} пересечений между файлами · ${pairSideRecords(allPair,selected).length} строк слева · ${pairSideRecords(allPair,analysisPeer).length} строк справа`:scoped.length?'Выбери пару списков.':'Пересечений не найдено.';$('cleanupAnalysis').disabled=!analysisPeer||!allPair.length;$('analysisCleanup').classList.remove('open');
  }
  async function openListAtLine(name,line){
    $('analysisModal').classList.remove('open');await selectList(name);const editor=$('listEditor'),rows=String(editor.value||'').split(/\r?\n/);let pos=0;for(let i=0;i<Math.max(0,line-1)&&i<rows.length;i++)pos+=rows[i].length+1;editor.focus();editor.setSelectionRange(pos,Math.min(editor.value.length,pos+(rows[line-1]?.length||0)));const lh=parseFloat(getComputedStyle(editor).lineHeight)||16.5;editor.scrollTop=Math.max(0,(line-3)*lh);editor.onscroll?.();
  }
  function removeListLines(content,lineNumbers){
    const newline=String(content).includes('\r\n')?'\r\n':'\n',hadTrailing=String(content).endsWith('\n'),remove=new Set(lineNumbers),rows=String(content).split(/\r?\n/);if(hadTrailing&&rows.length&&rows[rows.length-1]==='')rows.pop();const kept=rows.filter((_,i)=>!remove.has(i+1));return kept.join(newline)+(hadTrailing?newline:'');
  }
  function openAnalysisCleanup(){
    if(!analysisPeer)return;const pair=pairFindings(analysisPeer,false),leftCount=pairSideRecords(pair,selectedList).length,rightCount=pairSideRecords(pair,analysisPeer).length;$('cleanupLeft').textContent=`Удалить из ${selectedList} (${leftCount})`;$('cleanupRight').textContent=`Удалить из ${analysisPeer} (${rightCount})`;$('cleanupLeft').disabled=analysisSelectedDirty;$('analysisCleanupNote').textContent=analysisSelectedDirty?`В ${selectedList} есть несохранённые изменения: очистка слева заблокирована. Сохрани или отмени их и пересчитай. Справа можно очистить безопасно.`:'Перед изменением RouterForge создаст safety backup выбранного list-файла.';$('analysisCleanup').classList.add('open');
  }
  async function cleanupAnalysisSide(side){
    if(!analysisPeer||analysisRunning)return;const target=side==='left'?selectedList:analysisPeer;if(side==='left'&&analysisSelectedDirty){notice(`Сначала сохрани или отмени изменения в ${selectedList}.`,'error');return}const pair=pairFindings(analysisPeer,false),records=pairSideRecords(pair,target),source=analysisSources[target];if(!records.length||typeof source!=='string')return;const lineNumbers=records.map(x=>x.line),next=removeListLines(source,lineNumbers);if(next===source)return;if(!confirm(`Удалить ${records.length} пересекающихся строк из ${target}? Перед записью будет создан safety backup.`))return;
    analysisRunning=true;$('cleanupLeft').disabled=true;$('cleanupRight').disabled=true;$('cleanupAnalysis').disabled=true;$('analysisProgress').textContent=`Очищаю ${target}…`;
    try{const r=await api('/list/save',{method:'POST',body:JSON.stringify({name:target,content:next,expected_sha256:analysisHashes[target],confirm:'NFQWS_LIST_SAVE'})});render(r.status);if(target===selectedList)await selectList(selectedList);analysisSelectedDirty=Boolean(listMeta&&$('listEditor').value!==listOriginal);const loaded=await loadNeighborListContents();analysisSources=loaded.contents;analysisHashes=loaded.hashes;analysisFindings=analyzeCrossLists(analysisSources,selectedList);const oldPeer=analysisPeer;analysisPeer=analysisPeers().includes(oldPeer)?oldPeer:(analysisPeers()[0]||'');$('analysisProgress').textContent=`Удалено ${records.length} строк из ${target} · backup: ${r.backup}${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;renderAnalysis();notice(`Пересечения очищены в ${target}. Backup: ${r.backup}`,'good')}
    catch(e){$('analysisProgress').textContent='Ошибка очистки пересечений';notice(e.message,'error')}finally{analysisRunning=false;updateListButtons()}
  }
  async function runListAnalysis(){
    if(analysisRunning||!selectedList)return;analysisRunning=true;analysisSelectedDirty=Boolean(listMeta&&$('listEditor').value!==listOriginal);updateListButtons();$('analysisModal').classList.add('open');$('analysisLeftLines').innerHTML='<div class="analysis-empty">Читаю списки и строю граф пересечений…</div>';$('analysisRightLines').innerHTML='<div class="analysis-empty">Подготовка…</div>';$('analysisScope').textContent=`${selectedList} ↔ остальные списки`;
    try{const loaded=await loadNeighborListContents();analysisSources=loaded.contents;analysisHashes=loaded.hashes;analysisFindings=analyzeCrossLists(analysisSources,selectedList);analysisFilter='all';analysisPeer=analysisPeers()[0]||'';$('analysisProgress').textContent=`Проанализировано ${Object.keys(analysisSources).length} списков${loaded.errors.length?` · пропущено ${loaded.errors.length}`:''}`;renderAnalysis()}catch(e){$('analysisProgress').textContent='Ошибка анализа';$('analysisLeftLines').innerHTML=`<div class="analysis-empty bad-text">${esc(e.message)}</div>`;$('analysisRightLines').innerHTML='<div class="analysis-empty">—</div>'}finally{analysisRunning=false;updateListButtons()}
  }
  function closeAnalysis(){$('analysisModal').classList.remove('open');$('analysisCleanup').classList.remove('open')}

  function normalizeStoredConfigName(raw){
    let name=String(raw||'').trim();if(!name)return'';if(!/\.conf$/i.test(name))name+='.conf';return/^[A-Za-z0-9][A-Za-z0-9._-]*\.conf$/i.test(name)?name:'';
  }
  function storedConfigByName(name){return configLibraryFiles.find(x=>x.name===name)||null}
  function updateStoredConfigEditor(){
    const editor=$('storedConfigEditor'),has=!!selectedStoredConfig;if(!editor)return;const value=editor.value,count=lines(value).length;$('storedConfigGutter').textContent=Array.from({length:count},(_,i)=>i+1).join('\n');$('storedConfigStats').textContent=has?`${count} lines · ${bytes(new Blob([value]).size)}`:'—';refreshSyntaxEditor(editor);resizeStoredConfigEditor();
  }
  function resizeStoredConfigEditor(){
    const textarea=$('storedConfigEditor'),shell=$('storedConfigEditorShell');if(!textarea||!shell||shell.style.display==='none')return;if(shell.classList.contains('full-editor')){textarea.style.height='';return}if(!textarea.offsetParent)return;textarea.style.height='auto';textarea.style.height=`${Math.max(420,textarea.scrollHeight)}px`;
  }
  function mountStoredConfigSyntaxEditor(){
    const textarea=$('storedConfigEditor');if(!textarea)return;let state=syntaxEditors.get(textarea);if(!state){const body=document.createElement('div');body.className='syntax-editor-body';const pre=document.createElement('pre');pre.className='syntax-editor-highlight';textarea.parentNode.insertBefore(body,textarea);body.append(pre,textarea);state={pre,mode:'config',shell:body,resources:null};syntaxEditors.set(textarea,state);textarea.dataset.syntaxReady='1';textarea.addEventListener('input',()=>{refreshSyntaxEditor(textarea);updateStoredConfigEditor();updateStoredConfigButtons()});textarea.addEventListener('scroll',()=>{pre.scrollTop=textarea.scrollTop;pre.scrollLeft=textarea.scrollLeft});textarea.addEventListener('keydown',e=>{if(e.key==='Tab'){e.preventDefault();insertTab(textarea)}})}refreshSyntaxEditor(textarea);resizeStoredConfigEditor();
  }
  function updateStoredConfigButtons(){
    const has=!!selectedStoredConfig,editor=$('storedConfigEditor'),dirty=has&&editor.value!==storedConfigOriginal,meta=storedConfigByName(selectedStoredConfig)||storedConfigMeta;
    $('saveStoredConfig').disabled=!has||!dirty||configLibraryLoading;$('diffStoredConfig').disabled=!has||!dirty||configLibraryLoading;$('undoStoredConfig').disabled=!has||!dirty||configLibraryLoading;$('activateStoredConfig').disabled=!has||dirty||!String(editor.value||'').trim()||configLibraryLoading;$('renameStoredConfig').disabled=!has||configLibraryLoading;$('duplicateStoredConfig').disabled=!has||configLibraryLoading;$('deleteStoredConfig').disabled=!has||configLibraryLoading;
    $('storedConfigState').textContent=!has?'—':dirty?'UNSAVED':meta?.active?'ACTIVE':meta?.empty?'EMPTY':'CLEAN';$('storedConfigState').className='mono '+(dirty?'dirty':meta?.active?'good-text':meta?.empty?'warn-text':'good-text');updateStoredConfigEditor();
  }
  function renderConfigLibraryList(){
    const q=$('configLibrarySearch').value.trim().toLowerCase(),files=configLibraryFiles.filter(x=>!q||String(x.name||'').toLowerCase().includes(q));$('configLibraryCount').textContent=configLibraryFiles.length?`(${configLibraryFiles.length})`:'';
    $('configLibraryList').innerHTML=files.length?files.map(item=>`<div class="config-library-item${item.active?' active':''}${selectedStoredConfig===item.name?' selected':''}" data-config-name="${esc(item.name)}"><div style="min-width:0"><div class="config-library-name">${esc(item.name)}</div><div class="config-library-sub">${bytes(item.size)} · ${(item.sha256||'').slice(0,12)}…</div></div><div class="config-library-badges">${item.active?'<span class="config-badge active">ACTIVE</span>':''}${item.empty?'<span class="config-badge empty">EMPTY</span>':''}</div></div>`).join(''):'<div class="empty">Полных конфигов пока нет.</div>';
    $('configLibraryList').querySelectorAll('[data-config-name]').forEach(row=>row.onclick=()=>selectStoredConfig(row.dataset.configName));renderNeighborConfigList();
  }
  function renderNeighborConfigList(){
    const host=$('neighborConfigList');if(!host)return;const q=$('configLibrarySearch').value.trim().toLowerCase(),files=neighborConfigFiles.filter(x=>!q||String(x.name||'').toLowerCase().includes(q));host.innerHTML=files.length?files.map(item=>`<div class="config-neighbor-item"><div style="min-width:0"><div class="config-library-name">${esc(item.name)}</div><div class="config-library-sub">${bytes(item.size)} · ${(item.sha256||'').slice(0,12)}…</div></div><button type="button" data-import-neighbor="${esc(item.name)}">В библиотеку</button></div>`).join(''):'<div class="config-neighbor-empty">Подходящих .conf/.cfg рядом с основным не найдено.</div>';host.querySelectorAll('[data-import-neighbor]').forEach(button=>button.onclick=()=>openConfigName('import-neighbor',button.dataset.importNeighbor));
  }
  async function loadConfigLibrary(force=false){
    if(configLibraryLoading||configLibraryLoaded&&!force)return;configLibraryLoading=true;try{const[d,n]=await Promise.all([api('/configs'),api('/configs/neighbors')]);configLibraryFiles=Array.isArray(d.files)?d.files:[];neighborConfigFiles=Array.isArray(n.files)?n.files:[];configLibraryLoaded=true;$('configLibraryRoot').textContent=d.root||'/opt/etc/nfqws2/routerforge-configs';$('neighborConfigRoot').textContent=n.root||'/opt/etc/nfqws2';renderConfigLibraryList();if(selectedStoredConfig&&!storedConfigByName(selectedStoredConfig)){selectedStoredConfig='';storedConfigOriginal='';storedConfigMeta=null;$('storedConfigEditor').value='';$('storedConfigEditorShell').style.display='none';$('configLibraryEditorEmpty').style.display='';$('storedConfigTitle').textContent='Выберите конфиг';$('storedConfigMeta').textContent='—'}updateStoredConfigButtons()}catch(e){$('configLibraryList').innerHTML=`<div class="empty bad-text">${esc(e.message)}</div>`}finally{configLibraryLoading=false;updateStoredConfigButtons();reportHeight()}
  }
  async function selectStoredConfig(name){
    if(!name)return;if(selectedStoredConfig&&$('storedConfigEditor').value!==storedConfigOriginal&&!confirm('Есть несохранённые изменения. Переключиться без сохранения?'))return;configLibraryLoading=true;updateStoredConfigButtons();try{const d=await api(`/configs/read?name=${encodeURIComponent(name)}`);selectedStoredConfig=name;storedConfigOriginal=d.content||'';storedConfigMeta=d;$('storedConfigEditor').value=storedConfigOriginal;$('storedConfigEditorShell').style.display='';$('configLibraryEditorEmpty').style.display='none';$('storedConfigTitle').textContent=name;$('storedConfigMeta').textContent=`${d.path||name} · ${bytes(d.size)} · SHA ${(d.sha256||'').slice(0,16)}…${d.active?' · ACTIVE':''}`;mountStoredConfigSyntaxEditor();renderConfigLibraryList()}catch(e){notice(e.message,'error')}finally{configLibraryLoading=false;updateStoredConfigButtons();reportHeight()}
  }
  function openConfigName(mode,source=''){
    configNameMode=mode;configNameSource=source;$('configNameError').textContent='';let initial='';if(mode==='rename')initial=String(source||'').replace(/\.conf$/i,'');else if(mode==='duplicate')initial=String(source||'').replace(/\.conf$/i,'')+'-copy';else if(mode==='import-neighbor')initial=String(source||'').replace(/\.(?:conf|cfg)$/i,'');$('configNameInput').value=initial;$('configNameTitle').textContent=mode==='create-current'?'Сохранить текущий как':mode==='create-empty'?'Новый пустой конфиг':mode==='rename'?'Переименовать конфиг':mode==='import-neighbor'?`Скопировать ${source} в библиотеку`:'Создать копию';$('configNameModal').classList.add('open');setTimeout(()=>$('configNameInput').focus(),20)
  }
  function closeConfigName(){$('configNameModal').classList.remove('open');configNameMode='';configNameSource=''}
  async function confirmConfigName(){
    const name=normalizeStoredConfigName($('configNameInput').value);if(!name){$('configNameError').textContent='Используй буквы, цифры, точку, _ и -';return}configLibraryLoading=true;updateStoredConfigButtons();try{let r;if(configNameMode==='create-empty'||configNameMode==='create-current'){r=await api('/configs/create',{method:'POST',body:JSON.stringify({name,from_current:configNameMode==='create-current',content:'',confirm:'NFQWS_CONFIG_LIBRARY_CREATE'})})}else if(configNameMode==='rename'){r=await api('/configs/rename',{method:'POST',body:JSON.stringify({name:configNameSource,new_name:name,confirm:'NFQWS_CONFIG_LIBRARY_RENAME'})})}else if(configNameMode==='duplicate'){r=await api('/configs/duplicate',{method:'POST',body:JSON.stringify({name:configNameSource,new_name:name,confirm:'NFQWS_CONFIG_LIBRARY_DUPLICATE'})})}else if(configNameMode==='import-neighbor'){r=await api('/configs/import-neighbor',{method:'POST',body:JSON.stringify({name:configNameSource,destination:name,confirm:'NFQWS_CONFIG_LIBRARY_IMPORT'})})}else return;const finishedMode=configNameMode;closeConfigName();configLibraryLoading=false;configLibraryLoaded=false;await loadConfigLibrary(true);await selectStoredConfig(name);notice(finishedMode==='create-empty'?'Пустой полный конфиг создан. Вставь содержимое и сохрани.':finishedMode==='import-neighbor'?`Конфиг ${r.source||configNameSource} скопирован в библиотеку как ${name}.`:'Конфиг сохранён в библиотеке.','good')}catch(e){$('configNameError').textContent=e.message}finally{configLibraryLoading=false;updateStoredConfigButtons()}
  }
  function showStoredConfigDiff(){
    if(!selectedStoredConfig||$('storedConfigEditor').value===storedConfigOriginal)return;$('diffApply').dataset.mode='stored-config';$('diffBody').innerHTML=makeDiff(storedConfigOriginal,$('storedConfigEditor').value).map(x=>`<span class="${x.c}">${esc(x.t)}</span>`).join('\n')||'<span class="same">No changes</span>';$('diffModal').classList.add('open')
  }
  async function saveStoredConfig(){
    if(!selectedStoredConfig||$('storedConfigEditor').value===storedConfigOriginal)return;closeDiff();configLibraryLoading=true;updateStoredConfigButtons();try{const r=await api('/configs/save',{method:'POST',body:JSON.stringify({name:selectedStoredConfig,content:$('storedConfigEditor').value,confirm:'NFQWS_CONFIG_LIBRARY_SAVE'})});storedConfigOriginal=$('storedConfigEditor').value;configLibraryLoading=false;configLibraryLoaded=false;await loadConfigLibrary(true);await selectStoredConfig(selectedStoredConfig);notice(`Конфиг сохранён в библиотеке. Backup: ${r.backup}`,'good')}catch(e){notice(e.message,'error')}finally{configLibraryLoading=false;updateStoredConfigButtons()}
  }
  async function deleteStoredConfig(){
    if(!selectedStoredConfig)return;const name=selectedStoredConfig;if(!confirm(`Удалить ${name} из библиотеки? Перед удалением будет создан backup.`))return;configLibraryLoading=true;updateStoredConfigButtons();try{const r=await api('/configs/delete',{method:'POST',body:JSON.stringify({name,confirm:'NFQWS_CONFIG_LIBRARY_DELETE'})});selectedStoredConfig='';storedConfigOriginal='';storedConfigMeta=null;$('storedConfigEditor').value='';$('storedConfigEditorShell').style.display='none';$('configLibraryEditorEmpty').style.display='';$('storedConfigTitle').textContent='Выберите конфиг';$('storedConfigMeta').textContent='—';configLibraryLoading=false;configLibraryLoaded=false;await loadConfigLibrary(true);notice(`Конфиг удалён. Backup: ${r.backup}`,'good')}catch(e){notice(e.message,'error')}finally{configLibraryLoading=false;updateStoredConfigButtons()}
  }
  async function activateStoredConfig(){
    if(!selectedStoredConfig)return;if($('storedConfigEditor').value!==storedConfigOriginal){notice('Сначала сохрани изменения в библиотечный файл.','error');return}const name=selectedStoredConfig;if(!confirm(`Активировать полный конфиг ${name}? Текущий nfqws2.conf будет сохранён в backup.${current?.running?' Сервис будет перезапущен.':' Сервис останется остановленным.'}`))return;configLibraryLoading=true;updateStoredConfigButtons();try{const r=await api('/configs/activate',{method:'POST',body:JSON.stringify({name,confirm:'NFQWS_CONFIG_LIBRARY_ACTIVATE'})});if(r.status)render(r.status);configLibraryLoading=false;configLibraryLoaded=false;await loadConfigLibrary(true);await selectStoredConfig(name);notice(r.already_active?`${name} уже совпадает с активным nfqws2.conf.`:`${name} активирован.${r.service_restarted?' NFQWS перезапущен.':' Сервис оставлен остановленным.'} Backup: ${r.backup}`,'good')}catch(e){notice(e.message,'error')}finally{configLibraryLoading=false;updateStoredConfigButtons()}
  }


  function parseLogLine(raw){
    const text=String(raw||''),parts=text.split(/\s+:\s+/);if(parts.length>=6&&/^\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2}:\d{2}$/.test(parts[0])){return{raw:text,time:parts[0],host:parts[1],profile:parts[2].replace(/^profile\s+/i,''),client:parts[3].replace(/^client\s+/i,''),proto:parts[4].replace(/^proto\s+/i,''),event:parts.slice(5).join(' : ')}}return{raw:text,time:'',host:'',profile:'',client:'',proto:'',event:text};
  }
  function logSeverity(entry){const text=`${entry.raw} ${entry.event}`;if(/\b(?:ERROR|FATAL|CRITICAL|PANIC)\b/i.test(text)||/\bfailed\b/i.test(text))return'error';if(/\bfail counter\b/i.test(text)||/\b(?:WARN|WARNING|retry|timeout)\b/i.test(text))return'warn';if(/\b(?:udp_in|udp_out|started|running|success|recovered|added)\b/i.test(text))return'good';return'info'}
  function highlightLogEvent(value){
    const text=String(value||''),re=/(fail counter|udp_in|udp_out|retrans|timeout|retry|counter|threshold|\d+(?:\/\d+)?|<=|>=|==>|<==|<=>)/gi;let out='',last=0,m;while((m=re.exec(text))!==null){out+=esc(text.slice(last,m.index));const v=m[0],cls=/^\d/.test(v)?'log-token-number':/^(?:<=|>=|==>|<==|<=>)$/.test(v)?'log-token-op':'log-token-key';out+=`<span class="${cls}">${esc(v)}</span>`;last=m.index+v.length}return out+esc(text.slice(last));
  }
  function renderLog(){
    const rawRows=lines(current?.log_tail||'').filter(Boolean),entries=rawRows.map(parseLogLine),q=$('logSearch').value.trim().toLowerCase();let errors=0,warns=0,fails=0,quic=0,tcp=0,udp=0;for(const e of entries){const sev=logSeverity(e);if(sev==='error')errors++;if(sev==='warn')warns++;if(/fail counter/i.test(e.raw))fails++;const p=e.proto.toLowerCase();if(p==='quic')quic++;if(p==='tcp')tcp++;if(p==='udp')udp++}
    $('logSummary').innerHTML=`<span class="log-stat"><span>Строк</span><b>${entries.length}</b></span><span class="log-stat error"><span>Ошибки</span><b>${errors}</b></span><span class="log-stat warn"><span>Fail</span><b>${fails}</b></span><span class="log-stat info"><span>QUIC</span><b>${quic}</b></span><span class="log-stat good"><span>TCP / UDP</span><b>${tcp} / ${udp}</b></span>`;
    const visible=entries.filter(e=>{const sev=logSeverity(e);if(logMode==='errors'&&sev!=='error')return false;if(logMode==='warnings'&&sev!=='warn')return false;return !q||e.raw.toLowerCase().includes(q)});
    if(!visible.length){$('log').innerHTML='<div class="log-empty">По текущему фильтру записей нет.</div>'}else{$('log').innerHTML='<div class="log-columns-head"><span>Время</span><span>Хост</span><span>Профиль</span><span>Клиент</span><span>Proto</span><span>Событие</span></div>'+visible.map(e=>{const sev=logSeverity(e);if(!e.time)return `<div class="log-entry raw ${sev}"><span class="log-time">RAW</span><span class="log-raw-text">${esc(e.raw)}</span></div>`;return `<div class="log-entry ${sev}"><span class="log-time">${esc(e.time)}</span><span class="log-host" title="${esc(e.host)}">${esc(e.host)}</span><span class="log-profile">${esc(e.profile)}</span><span class="log-client">${esc(e.client)}</span><span class="log-proto">${esc(e.proto||'—')}</span><span class="log-event">${highlightLogEvent(e.event)}</span></div>`}).join('')}
    $('logMeta').textContent=`${current?.log_path||'—'}${current?.log_truncated?' · tail truncated':''}`;for(const[id,m]of[['logAll','all'],['logErrors','errors'],['logWarnings','warnings']])$(id).className=logMode===m?'primary':'';
  }

  const luaKeywords=new Set(['and','break','do','else','elseif','end','false','for','function','goto','if','in','local','nil','not','or','repeat','return','then','true','until','while']);
  function highlightLuaLine(line){
    const text=String(line||'');let out='',i=0;while(i<text.length){if(text.startsWith('--',i)){out+=syn('syn-comment',text.slice(i));break}const c=text[i];if(/\s/.test(c)){let j=i+1;while(j<text.length&&/\s/.test(text[j]))j++;out+=esc(text.slice(i,j));i=j;continue}if(c==='"'||c==="'"){const q=c;let j=i+1,escaped=false;for(;j<text.length;j++){const x=text[j];if(x===q&&!escaped){j++;break}escaped=x==='\\'&&!escaped;if(x!=='\\')escaped=false}out+=syn('syn-string',text.slice(i,j));i=j;continue}if(/[0-9]/.test(c)){let j=i+1;while(j<text.length&&/[0-9A-Fa-fxX._]/.test(text[j]))j++;out+=syn('syn-number',text.slice(i,j));i=j;continue}if(/[A-Za-z_]/.test(c)){let j=i+1;while(j<text.length&&/[A-Za-z0-9_]/.test(text[j]))j++;const v=text.slice(i,j);out+=syn(luaKeywords.has(v)?'syn-key':'syn-value',v);i=j;continue}if('=+-*/%^#<>~{}[]():,.'.includes(c)){out+=syn('syn-operator',c);i++;continue}out+=esc(c);i++}return out||'&#8203;';
  }
  function renderScriptCode(){const host=$('scriptCode');if(!selectedScript){host.className='script-reader-empty';host.innerHTML='Lua-скрипты nfqws2 доступны только для чтения.';return}host.className='script-code-shell';host.innerHTML=lines(scriptContent).map((line,i)=>`<div class="script-code-line"><span class="script-line-number">${i+1}</span><code>${highlightLuaLine(line)}</code></div>`).join('')}
  function renderScriptList(){const q=$('scriptSearch').value.trim().toLowerCase();const files=scriptFiles.filter(x=>!q||String(x.name||'').toLowerCase().includes(q));$('scriptList').innerHTML=files.length?files.map(item=>`<div class="script-item${selectedScript===item.name?' active':''}" data-script-name="${esc(item.name)}"><div><div class="script-name">${esc(item.name)}</div><div class="script-meta">${esc(item.modified_at||'')} · read-only</div></div><div class="script-meta">${bytes(item.size)}</div></div>`).join(''):'<div class="empty">Lua-скрипты не найдены.</div>';$('scriptList').querySelectorAll('[data-script-name]').forEach(row=>row.onclick=()=>selectScript(row.dataset.scriptName))}
  async function loadScripts(force=false){if(scriptsLoading||scriptsLoaded&&!force)return;scriptsLoading=true;$('scriptList').innerHTML='<div class="empty">Загрузка Lua inventory…</div>';try{const d=await api('/scripts');scriptFiles=Array.isArray(d.files)?d.files:[];scriptsLoaded=true;const roots=Array.isArray(d.roots)?d.roots:[];$('scriptRoot').textContent=d.root||roots[0]||'/opt/etc/nfqws2/lua';$('scriptCount').textContent=scriptFiles.length?`(${scriptFiles.length})`:'';renderScriptList();if(!scriptFiles.length&&roots.length)$('scriptList').innerHTML=`<div class="empty">Lua-скрипты не найдены.<br><span class="muted">Проверено: ${roots.map(esc).join(' · ')}</span></div>`}catch(e){$('scriptList').innerHTML=`<div class="empty bad-text">${esc(e.message)}</div>`}finally{scriptsLoading=false}}
  async function selectScript(name){if(!name)return;selectedScript=name;renderScriptList();$('selectedScriptTitle').textContent=name;$('selectedScriptMeta').textContent='Loading…';$('copyScript').disabled=true;$('fullscreenScript').disabled=true;try{const d=await api(`/scripts?name=${encodeURIComponent(name)}`);scriptContent=d.content||'';$('selectedScriptMeta').textContent=`${d.path||name} · ${bytes(d.size)} · read-only`;$('copyScript').disabled=false;$('fullscreenScript').disabled=false;renderScriptCode()}catch(e){scriptContent='';$('selectedScriptMeta').textContent=e.message;renderScriptCode()}}
  async function openScriptFromSetup(name){activateNFQWSTab('scripts');await loadScripts();const exact=scriptFiles.find(x=>String(x.name)===String(name));if(exact)await selectScript(exact.name);else{$('selectedScriptTitle').textContent=name;$('selectedScriptMeta').textContent='Скрипт упомянут в BASE_ARGS, но не найден в runtime inventory';selectedScript='';scriptContent='';renderScriptCode()}reportHeight()}

  function backupStateClass(state){return String(state||'').toLowerCase()}
  function backupCenterVisible(){
    const q=String($('backupCenterSearch')?.value||'').trim().toLowerCase();
    return backupCenterFiles.filter(x=>!q||[x.label,x.kind,x.target,x.sha256,x.state].join(' ').toLowerCase().includes(q));
  }
  function renderBackupCenter(){
    if(!$('backupCenterList'))return;
    const files=backupCenterFiles,visible=backupCenterVisible(),changed=files.filter(x=>x.state==='CHANGED'||x.state==='MISSING').length,restorable=files.filter(x=>x.restorable).length;
    $('backupPersistentCount').textContent=files.length;$('backupChangedCount').textContent=changed;$('backupRestorableCount').textContent=restorable;$('backupCenterRoot').textContent=backupCenterRoot||'/opt/var/lib/routerforge/nfqws-manager/backups';$('backupCountTab').textContent=files.length?`(${files.length})`:'';
    $('backupCenterList').innerHTML=visible.length?visible.map(x=>`<div class="backup-center-item${backupCenterSelected===x.id?' selected':''}" data-backup-id="${esc(x.id)}"><div><div class="backup-center-name">${esc(x.label)}</div><div class="backup-center-meta">${esc(x.kind.toUpperCase())} · ${esc(x.created_at||'—')}<br>${esc(x.target||'unbound')} · ${bytes(x.size)} · ${esc((x.sha256||'').slice(0,12))}</div></div><span class="backup-center-state ${backupStateClass(x.state)}">${esc(x.state)}</span></div>`).join(''):'<div class="empty">Persistent backups не найдены.</div>';
    $('backupCenterList').querySelectorAll('[data-backup-id]').forEach(row=>row.onclick=()=>selectBackupCenter(row.dataset.backupId));
    $('backupCenterState').textContent=backupCenterLoading?'LOADING':files.length?'READY':'EMPTY';$('backupCenterState').className='config-badge '+(files.length?'active':'');
  }
  async function loadBackupCenter(force=false){
    if(backupCenterLoading||backupCenterLoaded&&!force){renderBackupCenter();return}
    backupCenterLoading=true;renderBackupCenter();
    try{
      const d=await api('/backups');backupCenterFiles=Array.isArray(d.files)?d.files:[];backupCenterRoot=d.root||'';backupCenterLoaded=true;$('backupCount').textContent=d.safety_count||current?.backup_count||0;
      if(backupCenterSelected&&!backupCenterFiles.some(x=>x.id===backupCenterSelected)){backupCenterSelected='';backupCenterSelectedData=null;$('backupCenterDetail').className='backup-center-detail-empty';$('backupCenterDetail').textContent='Выберите backup слева.'}
      renderBackupCenter();
    }catch(e){$('backupCenterState').textContent='ERROR';$('backupCenterState').className='config-badge empty';$('backupCenterList').innerHTML=`<div class="empty bad-text">${esc(e.message)}</div>`}
    finally{backupCenterLoading=false;renderBackupCenter()}
  }
  async function selectBackupCenter(id){
    const item=backupCenterFiles.find(x=>x.id===id);if(!item)return;backupCenterSelected=id;backupCenterSelectedData=null;renderBackupCenter();$('backupCenterDetail').className='backup-center-detail-empty';$('backupCenterDetail').textContent='Загрузка backup и текущего target…';
    try{const d=await api(`/backups/read?id=${encodeURIComponent(id)}`);backupCenterSelectedData=d;renderBackupCenterDetail()}catch(e){$('backupCenterDetail').className='backup-center-detail-empty bad-text';$('backupCenterDetail').textContent=e.message}
  }
  function renderBackupCenterDetail(){
    if(!backupCenterSelectedData)return;const d=backupCenterSelectedData,e=d.entry||{},isText=typeof d.content==='string',diff=isText?makeDiff(String(d.current_content||''),String(d.content||'')):[];
    const diffHtml=isText?`<div><div class="section-kicker" style="margin-bottom:6px">Current → backup diff</div><pre class="backup-center-diff">${diff.map(x=>`<span class="${x.c}">${esc(x.t)}</span>`).join('\n')||'<span class="same">No textual differences</span>'}</pre></div>`:`<div><div class="section-kicker" style="margin-bottom:6px">Binary backup</div><div class="backup-center-preview">Binary payload · ${bytes(e.size)} · SHA256 ${esc(e.sha256||'—')}</div></div>`;
    $('backupCenterDetail').className='backup-center-detail';$('backupCenterDetail').innerHTML=`<div class="backup-center-detail-head"><div><div class="backup-center-detail-name">${esc(e.label||e.id)}</div><div class="backup-center-detail-meta">id=${esc(e.id||'—')}<br>target=${esc(e.target||'—')}</div></div><div class="toolbar"><span class="backup-center-state ${backupStateClass(e.state)}">${esc(e.state||'—')}</span><button id="restoreSelectedBackup" class="primary"${e.restorable?'':' disabled'}>Restore</button><button id="deleteSelectedBackup" class="danger">Удалить backup</button></div></div><div class="backup-center-kv"><div>Kind</div><div>${esc(e.kind||'—')}</div><div>Created</div><div>${esc(e.created_at||'—')}</div><div>Backup SHA256</div><div class="mono">${esc(e.sha256||'—')}</div><div>Current SHA256</div><div class="mono">${esc(e.current_sha256||'—')}</div><div>Size / mode</div><div>${bytes(e.size)} · ${Number(e.mode||0).toString(8)}</div></div>${diffHtml}${isText?`<div><div class="section-kicker" style="margin-bottom:6px">Backup content</div><pre class="backup-center-preview">${esc(d.content)}</pre></div>`:''}`;
    $('restoreSelectedBackup').onclick=restoreBackupCenter;$('deleteSelectedBackup').onclick=deleteBackupCenter;
  }
  async function restoreBackupCenter(){
    const e=backupCenterSelectedData?.entry;if(!e?.id||!e.restorable)return;
    if(!confirm(`Restore ${e.label} → ${e.target}?\n\nПеред записью RouterForge создаст persistent snapshot текущего target. Для config работающий nfqws2 будет перезапущен и проверен.`))return;
    try{const r=await api('/backups/restore',{method:'POST',body:JSON.stringify({id:e.id,confirm:'NFQWS_BACKUP_RESTORE'})});backupCenterLoaded=false;backupCenterSelected='';backupCenterSelectedData=null;if(r.status)render(r.status);await loadBackupCenter(true);notice(`Backup restored: ${e.label}`,'good')}
    catch(err){notice(err.message,'error')}
  }
  async function deleteBackupCenter(){
    const e=backupCenterSelectedData?.entry;if(!e?.id)return;
    if(!confirm(`Удалить persistent backup ${e.label}?\n\nЭто удаляет только snapshot, runtime-файлы не меняются.`))return;
    try{await api('/backups/delete',{method:'POST',body:JSON.stringify({id:e.id,confirm:'NFQWS_BACKUP_DELETE'})});backupCenterSelected='';backupCenterSelectedData=null;backupCenterLoaded=false;$('backupCenterDetail').className='backup-center-detail-empty';$('backupCenterDetail').textContent='Выберите backup слева.';await loadBackupCenter(true);notice('Persistent backup удалён.','good')}
    catch(err){notice(err.message,'error')}
  }

  function render(s){
    current=s;const available=!!s?.available,running=!!s?.running;$('state').textContent=!available?'NOT AVAILABLE':running?'RUNNING':'STOPPED';$('state').className='status '+(available&&running?'good':available?'warn':'bad');$('targetChip').textContent=s?.target_name||'nfqws';$('subtitle').textContent=`RouterForge extension · ${s?.config_path||'target not detected'}`;
    $('start').style.display=running?'none':'';$('stop').style.display=running?'':'none';$('reload').disabled=!running||busy;$('restart').disabled=!running||busy;
    if(!available)notice('NFQWS target не найден.','error');else if(!running)notice('NFQWS обнаружен, но сервис остановлен.','error');else notice('NFQWS runtime обнаружен. RouterForge управляет установленным внешним пакетом, не заменяя его lifecycle.','good');
    original=s?.config||'';loadFlow(original);$('config').value=original;$('configMeta').textContent=`${s?.config_path||'—'} · ${bytes(s?.config_size)} · SHA256 ${(s?.config_sha256||'—').slice(0,20)}…`;
    $('backupCount').textContent=s?.backup_count||0;
    renderOverview();renderFlow();renderListFiles();renderLog();mountRawSyntaxEditor();updateConfigEditor();refreshSyntaxEditor($('config'));setBusy(false);reportHeight();
  }
  async function load(){setBusy(true);try{render(await api('/status'));void loadConfigLibrary(true);if(!selectedList&&editableStatusLists().length)await selectList(sortLists(editableStatusLists())[0].name)}catch(e){notice(e.message,'error');setBusy(false)}}
  async function action(name){if(busy||!current?.mutation_ready)return;if(!confirm(`NFQWS: ${name}?`))return;setBusy(true);try{const r=await api('/action',{method:'POST',body:JSON.stringify({action:name,confirm:'NFQWS'})});render(r.status);notice(`${name}: успешно`,'good')}catch(e){notice(e.message,'error');setBusy(false)}}
  function makeDiff(a,b){const A=lines(a),B=lines(b),out=[];for(let i=0;i<Math.max(A.length,B.length);i++){if(A[i]===B[i]){if(A[i]!==undefined)out.push({c:'same',t:`  ${A[i]}`});continue}if(A[i]!==undefined)out.push({c:'del',t:`- ${A[i]}`});if(B[i]!==undefined)out.push({c:'add',t:`+ ${B[i]}`})}return out}
  function showDiff(){$('diffApply').dataset.mode='config';$('diffBody').innerHTML=makeDiff(original,$('config').value).map(x=>`<span class="${x.c}">${esc(x.t)}</span>`).join('\n')||'<span class="same">No changes</span>';$('diffModal').classList.add('open')}
  function closeDiff(){$('diffModal').classList.remove('open')}
  // NF1B3: installed Lua documentation is read-only; output is always textContent.
  let nf1bLuaCatalogData=null, nf1bLuaPreviewSource=null;
  function nf1bLuaRender(){
    const host=$('nf1bLuaResults'), info=$('nf1bLuaInfo'), data=nf1bLuaCatalogData;
    host.textContent='';
    if(!data){info.textContent='Нет данных о локальных Lua-функциях.';return}
    const q=$('nf1bLuaQuery').value.trim().toLowerCase();
    const rows=(Array.isArray(data.functions)?data.functions:[]).filter(f=>
      !q || [f.name,f.file,f.standard,f.legacy,...(f.description||[]),...(f.arguments||[]).map(a=>a.help)].join(' ').toLowerCase().includes(q));
    info.textContent='Lua-функций: '+(data.functions||[]).length+' · найдено: '+rows.length+
      ' · прочитано файлов: '+Number(data.files_scanned||0)+' · пропущено: '+Number(data.files_skipped||0)+
      '. Источник — файлы устройства; проверка только справочная.';
    for(const f of rows.slice(0,80)){
      const card=document.createElement('div');card.className='v2-row';
      const title=document.createElement('strong');title.textContent=String(f.name||'')+' · '+String(f.file||'');card.appendChild(title);
      const parts=[];
      if(f.standard)parts.push('Стандартные параметры: '+f.standard);
      if(f.legacy)parts.push('Аналог nfqws1: '+f.legacy);
      for(const d of (f.description||[]))parts.push(d);
      for(const a of (f.arguments||[]))parts.push('• '+(a.help||a.name));
      if(parts.length){const pre=document.createElement('div');pre.className='section-kicker';pre.style.whiteSpace='pre-wrap';pre.textContent=parts.join('\n');card.appendChild(pre)}
      host.appendChild(card);
    }
    if(rows.length>80){const more=document.createElement('div');more.className='setup-note';more.textContent='Показаны первые 80 функций — уточни поиск.';host.appendChild(more)}
    reportHeight();
  }
  async function nf1bLuaLoad(force=false){
    if(nf1bLuaCatalogData&&!force){nf1bLuaRender();return}
    $('nf1bLuaInfo').textContent='Читаем документацию установленных Lua-файлов…';
    try{
      nf1bLuaCatalogData=await api('/config/lua-catalog');nf1bLuaRender();
    }catch(e){$('nf1bLuaInfo').textContent='Lua-справочник недоступен: '+e.message}
  }
  function nf1bLuaPayloadFixSelected(){
    const editor=$('config');
    if(nf1bLuaPreviewSource!==editor.value){notice('Конфиг менялся после проверки. Сначала проверь его повторно.','error');return}
    const a=editor.selectionStart,b=editor.selectionEnd;
    const selected=editor.value.slice(a,b);
    const re=/^--lua-desync=(?:fake|multisplit|hostfakesplit|multidisorder)(?::[A-Za-z0-9_.,=+\-@]+)*$/;
    if(!re.test(selected)||selected.includes('payload=')){
      notice('Выдели один полный Lua-аргумент --lua-desync=... без payload= в редакторе.','error');return
    }
    const replacement=selected+':payload=all';
    if(!confirm('Добавить payload=all к выделенному аргументу? Изменится только текст в редакторе, без записи на роутер.'))return;
    editor.value=editor.value.slice(0,a)+replacement+editor.value.slice(b);
    editor.selectionStart=a;editor.selectionEnd=a+replacement.length;
    updateConfigEditor();refreshSyntaxEditor(editor);
    nf1bLuaPreviewSource=null;$('nf1bLuaFixNote').style.display='none';
    showDiff();notice('Изменение подготовлено в редакторе. Проверь diff и повторно запусти проверку конфигурации.','good');
  }
  async function checkConfigSyntaxPreview(){
    const before=$('config').value, result=$('configSyntaxStatus'), details=$('configSemanticIssues'), button=$('checkConfigSyntax');
    button.disabled=true;nf1bLuaPreviewSource=null;$('nf1bLuaFixNote').style.display='none';
    details.style.display='none';details.textContent='';
    result.textContent='Проверяем синтаксис, параметры и доступный native nfqws2 --dry-run…';
    result.style.color='';
    try{
      const response=await api('/config/validate',{method:'POST',body:JSON.stringify({content:before})});
      if($('config').value!==before){result.textContent='Конфиг изменился во время проверки. Запустите её ещё раз.';return}
      if(!response.syntax_ok){result.textContent='Синтаксис shell: '+(response.error||'ошибка');result.style.color='var(--bad)';return}
      const issues=Array.isArray(response.issues)?response.issues:[];
      nf1bLuaPreviewSource=before;
      if(issues.some(x=>x.code==='UNKNOWN_PAYLOAD_LUA'))$('nf1bLuaFixNote').style.display='block';
      const reportLines=issues.map(x=>{
        const level=x.severity==='error'?'ОШИБКА':x.severity==='warning'?'ВНИМАНИЕ':'ИНФО';
        return level+(x.variable?' · '+x.variable:'')+': '+(x.message||x.code||'замечание');
      });
      const state=String(response.native_dry_run||'not_performed');
      const detail=String(response.native_detail||'').trim();
      if(state==='failed')reportLines.push('NFQWS2 --dry-run: ОШИБКА'+(detail?'\n'+detail:''));
      if(state==='not_performed'&&detail)reportLines.push('NFQWS2 --dry-run: НЕ ПРОВЕДЁН · '+detail);
      details.textContent=reportLines.join('\n');details.style.display=reportLines.length?'block':'none';
      if(!response.semantic_ok){
        result.textContent='Синтаксис shell: OK, но найдены ошибки параметров. Сохранение будет отклонено.';
        result.style.color='var(--bad)';
      }else if(state==='failed'){
        result.textContent='Статический анализ: OK, но нативный nfqws2 --dry-run отклонил собранные параметры. Сначала проверьте ошибку ниже.';
        result.style.color='var(--bad)';
      }else if(state==='passed'){
        result.textContent='Синтаксис и статические параметры: OK · nfqws2 --dry-run: PASS (профилей: '+Number(response.native_profiles||0)+').';
        result.style.color=issues.some(x=>x.severity==='warning')?'var(--warn)':'var(--good)';
      }else{
        result.textContent='Статическая проверка: OK. Нативная проверка не проведена для этого конфига или на этом устройстве.';
        result.style.color='var(--warn)';
      }
    }catch(error){result.textContent='Ошибка проверки: '+error.message;result.style.color='var(--bad)'}
    finally{button.disabled=false}
  }
  async function saveConfig(){if(busy||!current?.mutation_ready||$('config').value===original)return;if(!confirm('Сохранить nfqws2.conf, создать backup и выполнить reload?'))return;closeDiff();setBusy(true);try{const r=await api('/config',{method:'POST',body:JSON.stringify({content:$('config').value,confirm:'NFQWS_CONFIG'})});clearLegacyCustomLabels();render(r.status);notice(`Конфигурация применена. Backup: ${r.backup}`,'good')}catch(e){notice(e.message,'error');setBusy(false)}}

  function openCreate(){$('newListName').value='';$('createError').textContent='';$('createModal').classList.add('open');setTimeout(()=>$('newListName').focus(),20)}function closeCreate(){$('createModal').classList.remove('open')}
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
  async function listSourceFetch(url,timeout=10000){const c=new AbortController(),timer=setTimeout(()=>c.abort(),timeout);try{const r=await fetch(url,{cache:'no-store',signal:c.signal,headers:{Accept:'application/vnd.github+json'}});if(!r.ok)throw new Error(`HTTP ${r.status}`);return r}finally{clearTimeout(timer)}}
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
  async function loadListSources(force=false){
    if(listSourceLoading||listSourceLoaded&&!force){renderListSources();return}listSourceLoading=true;listSourceError='';if($('listSourceState')){$('listSourceState').textContent='LOADING';$('listSourceState').className='config-badge'}
    try{const local=await api('/list-sources');listSourceFiles=Array.isArray(local.files)?local.files:[];$('listSourceRootMeta').textContent=local.root||'/opt/etc/nfqws2/lists';try{await resolveListSourceUpstream(force)}catch(e){listSourceUpstream=[];listSourceRef='';listSourceError=e?.name==='AbortError'?'timeout':String(e.message||e)}listSourceLoaded=true;renderListSources()}
    catch(e){listSourceLoaded=false;listSourceError=String(e.message||e);if($('listSourceLocalList'))$('listSourceLocalList').innerHTML=`<div class="empty bad-text">${esc(listSourceError)}</div>`;renderListSources()}
    finally{listSourceLoading=false}
  }
  function renderListSources(){
    if(!$('listSourceLocalList'))return;const changed=listSourceUpstream.filter(x=>listSourceStateFor(x)!=='ok').length,referenced=listSourceFiles.filter(x=>x.referenced).length;
    $('listSourceLocalCount').textContent=listSourceFiles.length;$('listSourceUpstreamCount').textContent=listSourceUpstream.length;$('listSourceMissingCount').textContent=changed;$('listSourceReferencedCount').textContent=referenced;
    const resolutionState=String(listSourceResolution?.state||''),sourceLabel=String(listSourceResolution?.source||'');
    const listState=listSourceError?(resolutionState==='rate_limited'?'RATE LIMITED':'OFFLINE'):resolutionState==='stale_cache'?'STALE CACHE':listSourceRef?'SYNCED':'LOCAL';
    $('listSourceState').textContent=listState;$('listSourceState').className='config-badge '+(listState==='SYNCED'?'active':listState==='OFFLINE'?'empty':'');
    $('listSourceUpstreamMeta').textContent=listSourceRef?`${listSourceRepo} · exact ref ${listSourceRef.slice(0,12)}${sourceLabel?` · ${sourceLabel}`:''}`:`${listSourceRepo} · ${listSourceError||resolutionState||'not synced'}`;
    $('listSourceLocalList').innerHTML=listSourceFiles.length?listSourceFiles.map(x=>`<div class="blob-row"><div><div class="blob-name">${esc(x.name)} ${x.protected?'<span class="config-badge">CORE</span>':''}</div><div class="blob-meta">${bytes(x.size)} · sha256 ${esc(String(x.sha256||'').slice(0,16))}…</div><div class="blob-refs">${x.referenced?`refs: ${esc((x.references||[]).join(', '))}`:'unreferenced'}</div></div><div class="blob-actions"><span class="blob-state ok">LOCAL</span><button data-list-source-open="${esc(x.name)}">Открыть</button></div></div>`).join(''):'<div class="empty">Локальные .list не найдены.</div>';
    $('listSourceLocalList').querySelectorAll('[data-list-source-open]').forEach(b=>b.onclick=()=>openListFromSetup(b.dataset.listSourceOpen));
    if(listSourceError){const title=String(listSourceResolution?.state||'')==='rate_limited'?'GitHub API rate limited':'Upstream unavailable';$('listSourceUpstreamList').innerHTML=`<div class="empty bad-text">${esc(title)}: ${esc(listSourceError)}<br><span class="muted">Локальные списки остаются доступны.</span></div>`;return}
    $('listSourceUpstreamList').innerHTML=listSourceUpstream.length?listSourceUpstream.map(item=>{const local=listSourceLocalByName(item.name),state=listSourceStateFor(item),action=state==='missing'?'Установить':state==='update'?'Заменить':'';return `<div class="blob-dep-row"><div><div class="blob-name">${esc(item.name)}</div><div class="blob-meta">expected ${esc(item.sha256.slice(0,16))}…${local?` · local ${esc(String(local.sha256||'').slice(0,16))}…`:''}${local?.protected?' · protected':''}</div></div><div class="blob-actions"><span class="blob-state ${state}">${state==='ok'?'VERIFIED':state==='update'?'UPDATE':'MISSING'}</span>${action?`<button data-list-source-install="${esc(item.name)}">${esc(action)}</button>`:`<button data-list-source-open="${esc(item.name)}">Открыть</button>`}</div></div>`}).join(''):'<div class="empty">Upstream manifest не содержит strategies/lists/*.list.</div>';
    $('listSourceUpstreamList').querySelectorAll('[data-list-source-install]').forEach(b=>b.onclick=()=>installListSource(b.dataset.listSourceInstall));$('listSourceUpstreamList').querySelectorAll('[data-list-source-open]').forEach(b=>b.onclick=()=>openListFromSetup(b.dataset.listSourceOpen));
  }
  async function installListSource(name){
    if(listSourceLoading)return;const item=listSourceUpstream.find(x=>x.name===name);if(!item||!listSourceRef)return;const local=listSourceLocalByName(name),replace=!!local,verb=replace?'заменить':'установить';if(!confirm(`List Source: ${verb} ${name}?\n\nSHA256 ${item.sha256}\nExact ref ${listSourceRef.slice(0,12)}\n\nSafety backup: ${replace?'YES':'not needed'}\nReload/restart: NO`))return;
    listSourceLoading=true;try{const r=await listSourceFetch(`https://raw.githubusercontent.com/${listSourceRepo}/${listSourceRef}/${item.path}`,10000),buffer=await r.arrayBuffer(),actual=await listSourceSha256(buffer);if(actual.toLowerCase()!==item.sha256.toLowerCase())throw new Error('upstream SHA256 mismatch');const result=await api('/list-sources/install',{method:'POST',body:JSON.stringify({name:item.name,content_base64:listSourceBase64(buffer),expected_sha256:item.sha256,expected_existing_sha256:replace?local.sha256:'',replace,confirm:replace?'NFQWS_LIST_SOURCE_REPLACE':'NFQWS_LIST_SOURCE_INSTALL'})});listSourceFiles=Array.isArray(result.inventory?.files)?result.inventory.files:listSourceFiles;current=result.status||current;renderListFiles();renderListSources();notice(`${name}: ${result.already_current?'уже актуален':replace?'заменён':'установлен'} · runtime restart не выполнялся.`,'good')}
    catch(e){notice(`List Source ${name}: ${e.message}`,'error')}
    finally{listSourceLoading=false}
  }

  let geoAssets=[];
  function geoAssetSelected(){return geoAssets.find(x=>x.name===$('geoAssetSelect')?.value)}
  function renderGeoAssets(){
    if(!$('geoAssets'))return;
    $('geoAssets').innerHTML=geoAssets.length?geoAssets.map(x=>`<div class="blob-row"><div><div class="blob-name">${esc(x.name)} <span class="config-badge">${esc(String(x.kind||'').toUpperCase())}</span></div><div class="blob-meta">${bytes(x.size||0)} · sha256 ${esc(String(x.sha256||'').slice(0,16))}… · ${Number(x.categories?.length||0)} категорий</div><div class="blob-refs">${esc(x.source||'локальный файл')}${x.source_ref?` · ref ${esc(x.source_ref)}`:''}</div></div></div>`).join(''):'<div class="empty">Geo assets ещё не загружены.</div>';
    $('geoAssetSelect').innerHTML=geoAssets.map(x=>`<option value="${esc(x.name)}">${esc(x.name)} · ${esc(x.kind)}</option>`).join('');
    renderGeoCategories();
  }
  function renderGeoCategories(){
    const asset=geoAssetSelected(),cats=Array.isArray(asset?.categories)?asset.categories:[];
    if(asset){$('geoSource').value=asset.source||'';$('geoSourceRef').value=asset.source_ref||''}
    $('geoCategorySelect').innerHTML=cats.map(x=>`<option value="${esc(x.name)}">${esc(x.name)} · ${Number(x.count||0)}</option>`).join('');
    if(asset&&cats.length&&!$('geoListName').value.trim())$('geoListName').value=`geo-${String(cats[0].name).replace(/[^A-Za-z0-9._-]+/g,'-')}.list`;
  }
  async function loadGeoAssets(){
    if(!$('geoAssets'))return;
    try{
      const d=await api('/v2/geo/assets');geoAssets=Array.isArray(d.assets)?d.assets:[];renderGeoAssets();
      const a=await api('/v2/geo/autohostlist');
      $('geoAutohostlist').textContent=`Autohostlist: ${a.supported?'поддерживается':'не подтверждён'} · ${a.configured?'настроен':'не настроен'}`;
    }catch(e){$('geoAssets').innerHTML=`<div class="empty bad-text">${esc(e.message)}</div>`}
  }
  async function geoSHA256(buffer){const d=await crypto.subtle.digest('SHA-256',buffer);return [...new Uint8Array(d)].map(x=>x.toString(16).padStart(2,'0')).join('')}
  function geoBase64(buffer){let out='',bytes=new Uint8Array(buffer),step=0x8000;for(let i=0;i<bytes.length;i+=step)out+=String.fromCharCode(...bytes.subarray(i,i+step));return btoa(out)}
  async function uploadGeoAsset(){
    const file=$('geoFile').files?.[0];if(!file){notice('Геоданные: выберите файл.','error');return}
    if(file.size>24*1024*1024){notice('Геоданные: файл больше 24 МиБ.','error');return}
    $('geoUpload').disabled=true;$('geoState').textContent='UPLOADING';
    try{
      const buffer=await file.arrayBuffer(),sha=await geoSHA256(buffer);
      const d=await api('/v2/geo/upload',{method:'POST',body:JSON.stringify({name:file.name,kind:$('geoKind').value,content_base64:geoBase64(buffer),expected_sha256:sha,source:$('geoSource').value.trim(),source_ref:$('geoSourceRef').value.trim(),confirm:'ROUTERFORGE_GEO_UPLOAD'})});
      $('geoState').textContent='СОХРАНЕНО';notice(`Geo ${d.asset?.name||file.name}: ${Number(d.asset?.categories?.length||0)} categories · production untouched.`,'good');await loadGeoAssets();
    }catch(e){$('geoState').textContent='ОШИБКА';notice(`Geo upload: ${e.message}`,'error')}finally{$('geoUpload').disabled=false}
  }
  async function previewGeoCategory(){
    const asset=geoAssetSelected(),category=$('geoCategorySelect').value;if(!asset||!category)return;
    try{
      const d=await api('/v2/geo/preview',{method:'POST',body:JSON.stringify({name:asset.name,category,limit:500})});
      $('geoPreviewResult').textContent=`${asset.name}:${category}\n${d.count}/${d.total}${d.truncated?' (preview truncated)':''}\n\n${(d.entries||[]).join('\n')}`;
    }catch(e){notice(`Geo preview: ${e.message}`,'error')}
  }
  async function saveGeoCategory(){
    const asset=geoAssetSelected(),category=$('geoCategorySelect').value,name=String($('geoListName').value||'').trim();if(!asset||!category||!name){notice('Geo: asset/category/list name required.','error');return}
    const existing=(listSourceFiles||[]).some(x=>x.name===name),replace=existing;
    if(!confirm(`Geo category ${asset.name}:${category} → ${name}?\n\n${replace?'Existing list will be backed up and replaced.':'New list will be created.'}\nReload/restart: NO`))return;
    try{
      const d=await api('/v2/geo/save-list',{method:'POST',body:JSON.stringify({name:asset.name,category,list_name:name,limit:20000,replace,confirm:replace?'ROUTERFORGE_GEO_LIST_REPLACE':'ROUTERFORGE_GEO_LIST_CREATE'})});
      listSourceLoaded=false;await loadListSources(true);renderListFiles();notice(`Geo list ${name}: ${d.count} entries · production config unchanged.`,'good');
    }catch(e){notice(`Geo save list: ${e.message}`,'error')}
  }
  $('geoAssetSelect')?.addEventListener('change',renderGeoCategories);
  $('geoCategorySelect')?.addEventListener('change',()=>{const c=$('geoCategorySelect').value;if(c)$('geoListName').value=`geo-${String(c).replace(/[^A-Za-z0-9._-]+/g,'-')}.list`});
  $('geoUpload')?.addEventListener('click',uploadGeoAsset);
  $('geoSaveProvenance')?.addEventListener('click',saveGeoProvenance);
  $('geoRefresh')?.addEventListener('click',loadGeoAssets);
  $('geoPreview')?.addEventListener('click',previewGeoCategory);
  $('geoSaveList')?.addEventListener('click',saveGeoCategory);

  const catalogRepo='rndnaame/nfqws-menu',catalogBranch='main',catalogPrefix='strategies/nfqws2/';
  const catalogDiffKeys=['ISP_INTERFACE','NFQWS_BASE_ARGS','NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET','NFQWS_ARGS_CUSTOM','NFQWS_EXTRA_ARGS','IPV6_ENABLED','TCP_PORTS','UDP_PORTS','POLICY_NAME','POLICY_EXCLUDE','NFQUEUE_NUM','USER','CONFIG_VERSION'];
  async function catalogFetch(url,timeout=10000){const c=new AbortController(),timer=setTimeout(()=>c.abort(),timeout);try{const r=await fetch(url,{cache:'no-store',signal:c.signal,headers:{Accept:'application/vnd.github+json'}});if(!r.ok)throw new Error(`HTTP ${r.status}`);return r}finally{clearTimeout(timer)}}
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
  function catalogVisibleItems(){const q=String($('catalogSearch')?.value||'').trim().toLowerCase();return catalogItems.filter(x=>(catalogFilter==='all'||catalogFilter==='local'&&x.kind!=='upstream'||catalogFilter==='upstream'&&x.kind==='upstream')&&(!q||[x.name,x.path,x.source,x.ref,x.sha256].join(' ').toLowerCase().includes(q)))}
  function renderCatalogList(){
    if(!$('catalogList'))return;const items=catalogVisibleItems();$('catalogTabState').textContent=catalogItems.length?`(${catalogItems.length})`:'';$('catalogList').innerHTML=items.length?items.map(x=>`<div class="catalog-item${catalogSelected===x.id?' selected':''}" data-catalog-id="${esc(x.id)}"><div><div class="catalog-item-name">${esc(x.name)}</div><div class="catalog-item-meta">${esc(catalogSourceLabel(x))} · ${esc(x.ref||'local')}${x.sha256?` · ${esc(x.sha256.slice(0,10))}`:''}</div></div><span class="config-badge${x.kind==='upstream'?'':' active'}">${esc(catalogSourceLabel(x))}</span></div>`).join(''):'<div class="empty">По текущему фильтру стратегий нет.</div>';$('catalogList').querySelectorAll('[data-catalog-id]').forEach(row=>row.onclick=()=>selectCatalogItem(row.dataset.catalogId));
  }
  function renderCatalogSources(){
    if(!$('catalogLocalSource'))return;const local=catalogItems.filter(x=>x.kind!=='upstream').length,up=catalogItems.filter(x=>x.kind==='upstream').length;$('catalogLocalSource').textContent=`${local} items · active canonical + Config Library`;
    const u=catalogUpstream,ref=esc((u.sha||u.ref||'main').slice(0,12)),source=u.source?` · ${esc(u.source)}`:'';
    $('catalogUpstreamSource').innerHTML=u.state==='ready'?`<span class="catalog-source-status good">SYNCED</span> ${esc(up)} configs · ref ${ref}${source}`:u.state==='stale'?`<span class="catalog-source-status warn">STALE CACHE</span> ${esc(up)} configs · ref ${ref}${source}`:u.state==='rate_limited'?`<span class="catalog-source-status warn">RATE LIMITED</span> ${esc(u.error||'GitHub API limit')} · локальный каталог продолжает работать`:u.state==='error'?`<span class="catalog-source-status bad">OFFLINE</span> ${esc(u.error||'upstream unavailable')} · локальный каталог продолжает работать`:`<span class="catalog-source-status warn">${esc(u.state.toUpperCase())}</span> ref ${ref}`;
  }
  async function loadCatalog(force=false){
    if(catalogLoading||catalogLoaded&&!force){renderCatalogList();return}catalogLoading=true;catalogUpstream={state:'loading',ref:catalogBranch,sha:'',error:''};renderCatalogSources();try{
      if(!configLibraryLoaded)await loadConfigLibrary(true);const local=[{id:'active:canonical',kind:'active',name:'nfqws2.conf',path:current?.config_path||'/opt/etc/nfqws2/nfqws2.conf',source:'RouterForge active',ref:'active',sha256:current?.config_sha256||''},...configLibraryFiles.map(x=>({id:`local:${x.name}`,kind:'local',name:x.name,path:x.path,source:'Config Library',ref:'local',sha256:x.sha256||'',active:!!x.active}))];
      let upstream=[];try{
        const resolved=await resolveNFQWSMenuRef(force),sha=String(resolved?.sha||'').trim();catalogUpstream.sha=sha;catalogUpstream.ref=catalogBranch;catalogUpstream.source=String(resolved?.source||'');
        if(!/^[0-9a-f]{40}$/i.test(sha)){catalogUpstream.state=resolved?.state==='rate_limited'?'rate_limited':'error';throw new Error(resolved?.error||'upstream exact commit не получен')}
        const ref=sha,manifestRes=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${ref}/SHA256SUMS`,10000),manifest=await manifestRes.text();
        upstream=manifest.split(/\r?\n/).map(line=>{const m=line.match(/^([0-9a-fA-F]{64})\s+(strategies\/nfqws2\/[^\s]+\.conf)$/);if(!m)return null;return{id:`upstream:${m[2]}`,kind:'upstream',name:m[2].split('/').pop(),path:m[2],source:catalogRepo,ref:ref,sha256:m[1].toLowerCase()}}).filter(Boolean);catalogUpstream.state=resolved?.state==='stale_cache'?'stale':'ready';catalogUpstream.count=upstream.length;
      }catch(e){if(catalogUpstream.state!=='rate_limited')catalogUpstream.state='error';catalogUpstream.error=e?.name==='AbortError'?'timeout':String(e.message||e)}
      catalogItems=[...local,...upstream];catalogLoaded=true;renderCatalogSources();renderCatalogList();if(catalogSelected&&!catalogItems.some(x=>x.id===catalogSelected)){catalogSelected='';catalogSelectedData=null;$('catalogDetail').className='catalog-empty';$('catalogDetail').textContent='Выберите стратегию слева.'}
    }finally{catalogLoading=false;renderCatalogSources()}
  }
  async function catalogReadItem(item){
    if(item.kind==='active')return{content:original,sha256:current?.config_sha256||'',integrity:'local'};
    if(item.kind==='local'){const d=await api(`/configs/read?name=${encodeURIComponent(item.name)}`);const actual=await catalogSha256(d.content||'');return{content:d.content||'',sha256:actual||d.sha256||'',integrity:item.sha256&&actual&&item.sha256.toLowerCase()!==actual.toLowerCase()?'mismatch':'local'}};
    const r=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${item.ref}/${item.path}`,10000),content=await r.text(),actual=await catalogSha256(content),integrity=item.sha256&&actual?(item.sha256.toLowerCase()===actual.toLowerCase()?'verified':'mismatch'):'unknown';return{content,sha256:actual,integrity};
  }
  async function selectCatalogItem(id){
    const item=catalogItems.find(x=>x.id===id);if(!item)return;catalogSelected=id;catalogSelectedData=null;renderCatalogList();$('catalogDetail').className='catalog-empty';$('catalogDetail').textContent='Загрузка preview и semantic analysis…';try{const data=await catalogReadItem(item);catalogSelectedData={item,...data};renderCatalogDetail()}catch(e){$('catalogDetail').className='catalog-empty bad-text';$('catalogDetail').textContent=`Источник недоступен: ${e.message}`}
  }
  function renderCatalogDetail(){
    if(!catalogSelectedData)return;const d=catalogSelectedData,item=d.item,model=catalogCompatibility(d.content,d.integrity),diff=catalogSemanticDiff(d.content),deps=model.deps,changed=diff.filter(x=>x.state!=='same').length,missing=deps.lists.filter(x=>!(current?.lists||[]).some(y=>y.name===x)).length;
    const checkHtml=model.checks.map(x=>`<div class="catalog-check ${x.severity}"><span class="catalog-check-mark">${x.severity==='good'?'✓':x.severity==='bad'?'!':x.severity==='warn'?'!':'i'}</span><div><div class="catalog-check-title">${esc(x.title)}</div><div class="catalog-check-detail">${esc(x.detail)}</div></div></div>`).join('');
    const depHtml=[...deps.lists.map(x=>`<span class="catalog-dep ${(current?.lists||[]).some(y=>y.name===x)?'present':'missing'}">LIST ${esc(x)}</span>`),...deps.blobs.map(x=>`<span class="catalog-dep">BLOB ${esc(x.name)}</span>`),...deps.lua.map(x=>`<span class="catalog-dep">LUA ${esc(catalogNormalizeRefPath(x))}</span>`),deps.tcp?`<span class="catalog-dep">TCP ${esc(deps.tcp)}</span>`:'',deps.udp?`<span class="catalog-dep">UDP ${esc(deps.udp)}</span>`:''].filter(Boolean).join('')||'<span class="muted small">Явные dependencies не найдены.</span>';
    const diffHtml=`<div class="catalog-diff"><div class="catalog-diff-row head"><span>Assignment</span><span>State</span><span>Active</span><span>Candidate</span></div>${diff.map(x=>`<div class="catalog-diff-row"><b>${esc(x.key)}</b><span class="catalog-diff-state ${x.state}">${esc(x.state.toUpperCase())}</span><span class="catalog-diff-value">${esc(x.active||'—')}</span><span class="catalog-diff-value">${esc(x.candidate||'—')}</span></div>`).join('')}</div>`;
    $('catalogDetail').className='catalog-detail';$('catalogDetail').innerHTML=`<div class="catalog-detail-head"><div><div class="catalog-detail-name">${esc(item.name)}</div><div class="catalog-detail-meta">source=${esc(item.source)} · ref=${esc(item.ref||'local')} · path=${esc(item.path||'—')}<br>sha256=${esc(d.sha256||item.sha256||'—')} · integrity=${esc(d.integrity.toUpperCase())}</div></div><div class="toolbar"><span class="catalog-verdict ${model.status.toLowerCase()}">${esc(model.status)}</span><button id="catalogToSandbox">В Sandbox · HTTPS</button><button id="catalogSmartApply" class="primary">Smart Apply</button>${deps.blobs.length?`<button id="catalogOpenBlobs">Blobs · ${deps.blobs.length}</button>`:''}${item.kind==='local'?'<button id="catalogOpenLocal">Открыть в Configs</button>':''}</div></div><div class="catalog-summary"><div class="catalog-stat"><span>Diff assignments</span><strong>${changed}</strong></div><div class="catalog-stat"><span>Lists</span><strong>${deps.lists.length}${missing?` / ${missing} missing`:''}</strong></div><div class="catalog-stat"><span>Blobs</span><strong>${deps.blobs.length}</strong></div><div class="catalog-stat"><span>Lua</span><strong>${deps.lua.length}</strong></div></div><div><div class="section-kicker" style="margin-bottom:6px">Compatibility</div><div class="catalog-checks">${checkHtml}</div></div><div><div class="section-kicker" style="margin-bottom:6px">Dependencies</div><div class="catalog-deps">${depHtml}</div></div><div><div class="section-kicker" style="margin-bottom:6px">Semantic diff vs active canonical</div>${diffHtml}</div><div><div class="section-kicker" style="margin-bottom:6px">Config preview · read-only</div><pre class="catalog-preview">${esc(d.content)}</pre></div>`;
    $('catalogToSandbox').onclick=()=>{const candidate=assignmentValue(d.content,'NFQWS_ARGS');if(!candidate){notice('В выбранном config NFQWS_ARGS пуст — нечего отправлять в Sandbox.','error');return}v2OpenSandboxFromArgs(candidate,item.name,'catalog');notice(`Candidate ${item.name} загружен в реальный V2 Sandbox. Active config не изменён.`,'good')};
    $('catalogSmartApply').onclick=()=>openSmartApply();
    if($('catalogOpenBlobs'))$('catalogOpenBlobs').onclick=async()=>{const tab=document.querySelector('.tab[data-tab="blobs"]');if(tab)tab.click();await loadBlobs(true)};
    if(item.kind==='local'&&$('catalogOpenLocal'))$('catalogOpenLocal').onclick=async()=>{const tab=document.querySelector('.tab[data-tab="configs"]');if(tab)tab.click();await loadConfigLibrary();await selectStoredConfig(item.name)};
  }
  function setCatalogFilter(value){catalogFilter=value;for(const[id,v]of[['catalogAll','all'],['catalogLocal','local'],['catalogUpstream','upstream']])$(id).classList.toggle('active',v===value);renderCatalogList()}

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
  function renderSmartApplyPlan(){
    const p=smartApplyPlan;if(!p){$('smartApplyPlan').innerHTML='<div class="empty">План ещё не построен.</div>';$('applySmartApply').disabled=true;return}const writeCount=[...p.lists,...p.blobs].filter(x=>x.state==='write').length,blocked=p.blocked.length;
    const row=x=>`<div class="smart-apply-row"><code>${esc(x.name)}</code><span>${esc(x.reason)}</span><span class="smart-apply-state ${x.state}">${esc(x.state.toUpperCase())}</span></div>`;
    const preserved=p.merge.preserved.length?p.merge.preserved.map(x=>`<div class="smart-apply-row"><code>${esc(x.key)}</code><span>candidate: ${esc(x.candidate)} → local: ${esc(x.active)}</span><span class="smart-apply-state ok">PRESERVE</span></div>`).join(''):'<div class="smart-apply-row"><span>Локальные environment assignments уже совпадают.</span><span></span><span class="smart-apply-state ok">SAME</span></div>';
    $('smartApplySource').textContent=`${p.item.source} · ${p.item.name} · ref ${String(p.ref).slice(0,12)}`;$('smartApplyPlan').innerHTML=`<div class="smart-apply-head"><div class="smart-apply-stat"><span>Config</span><strong>${p.compat.status}</strong></div><div class="smart-apply-stat"><span>Dependencies</span><strong>${p.lists.length+p.blobs.length}</strong></div><div class="smart-apply-stat"><span>Writes</span><strong>${writeCount+1}</strong></div><div class="smart-apply-stat"><span>Runtime</span><strong>${p.wasRunning?'RESTART':'STOPPED'}</strong></div></div>${blocked?`<div class="smart-apply-error">BLOCKED: ${blocked.map(x=>esc(x.name)).join(', ')}. Для local candidate RouterForge не угадывает источник отсутствующих dependency.</div>`:''}<div class="smart-apply-warning"><b>TOCTOU guard:</b> apply разрешён только если active config всё ещё имеет SHA256 ${esc(p.expectedActiveSha.slice(0,16))}… Candidate SHA256 ${esc(p.configSha.slice(0,16))}…</div><div class="smart-apply-section"><div class="smart-apply-section-head"><span>Semantic merge · сохраняем локальные runtime/environment параметры</span><span>${p.merge.preserved.length}</span></div><div class="smart-apply-rows">${preserved}</div></div><div class="smart-apply-section"><div class="smart-apply-section-head"><span>Lists</span><span>${p.lists.length}</span></div><div class="smart-apply-rows">${p.lists.length?p.lists.map(row).join(''):'<div class="smart-apply-row"><span>Нет list dependencies</span><span></span><span class="smart-apply-state ok">PASS</span></div>'}</div></div><div class="smart-apply-section"><div class="smart-apply-section-head"><span>Blobs</span><span>${p.blobs.length}</span></div><div class="smart-apply-rows">${p.blobs.length?p.blobs.map(row).join(''):'<div class="smart-apply-row"><span>Нет blob dependencies</span><span></span><span class="smart-apply-state ok">PASS</span></div>'}</div></div><div class="smart-apply-warning"><b>Commit order:</b> dependencies → canonical config → ${p.wasRunning?'controlled restart':'без запуска сервиса'} → SHA/runtime verify. Первый FAIL откатывает touched dependencies + config и возвращает предыдущий runtime.</div>`;$('applySmartApply').disabled=smartApplyBusy||blocked>0;
  }
  async function openSmartApply(){
    $('smartApplyModal').classList.add('open');$('smartApplyPlan').innerHTML='<div class="empty">Строю exact plan…</div>';$('applySmartApply').disabled=true;try{smartApplyPlan=await buildSmartApplyPlan();renderSmartApplyPlan()}catch(e){smartApplyPlan=null;$('smartApplyPlan').innerHTML=`<div class="smart-apply-error">${esc(e.message)}</div>`;notice(`Smart Apply plan: ${e.message}`,'error')}
  }
  function closeSmartApply(){if(smartApplyBusy)return;$('smartApplyModal').classList.remove('open')}
  async function smartApplyFetchDependency(kind,row,ref){
    if(row.state!=='write'||!row.expected)return null;const url=`https://raw.githubusercontent.com/${catalogRepo}/${ref}/${row.expected.path}`,r=await catalogFetch(url,15000),buffer=await r.arrayBuffer(),actual=kind==='list'?await listSourceSha256(buffer):await blobSha256Bytes(buffer);if(actual.toLowerCase()!==row.expected.sha256.toLowerCase())throw new Error(`${row.name}: upstream SHA256 mismatch before transaction`);return{name:row.name,content_base64:listSourceBase64(buffer),expected_sha256:row.expected.sha256};
  }
  async function applySmartApply(){
    const p=smartApplyPlan;const recoveryBaseline=$('recoveryGuardEnabled')?.checked?latestRecoveryBaselineForCurrent():null,recoveryStartedAt=new Date().toISOString();if(!p||p.blocked.length||smartApplyBusy)return;if(!confirm(`Smart Apply ${p.item.name}?\nDependencies to write: ${[...p.lists,...p.blobs].filter(x=>x.state==='write').length}\nConfig: semantic merge\n${p.wasRunning?'Service restart: YES':'Service restart: NO'}\nRollback on first failure: YES`))return;smartApplyBusy=true;$('applySmartApply').disabled=true;$('refreshSmartApply').disabled=true;
    try{const lists=(await Promise.all(p.lists.map(x=>smartApplyFetchDependency('list',x,p.ref)))).filter(Boolean),blobs=(await Promise.all(p.blobs.map(x=>smartApplyFetchDependency('blob',x,p.ref)))).filter(Boolean);const latest=await api('/status');if(String(latest.config_sha256||'').toLowerCase()!==String(p.expectedActiveSha).toLowerCase())throw new Error('Active config изменился после построения плана. Пересчитайте Smart Apply.');const expected_lists=Object.fromEntries(p.lists.filter(x=>x.expected).map(x=>[x.name,x.expected.sha256])),expected_blobs=Object.fromEntries(p.blobs.filter(x=>x.expected).map(x=>[x.name,x.expected.sha256])),result=await api('/smart-apply',{method:'POST',body:JSON.stringify({config:p.config,expected_active_sha256:p.expectedActiveSha,expected_config_sha256:p.configSha,lists,blobs,expected_lists,expected_blobs,confirm:'NFQWS_SMART_APPLY'})});current=result.status||current;original=current?.config||p.config;loadFlow(original);$('config').value=original;listSourceLoaded=false;blobLoaded=false;configLibraryLoaded=false;catalogLoaded=false;smartApplyPlan=null;render(current);$('smartApplyModal').classList.remove('open');notice(`Smart Apply: PASS · ${p.item.name} · lists ${result.applied_lists?.length||0} · blobs ${result.applied_blobs?.length||0}${result.service_restarted?' · runtime restarted':''}`,'good');if(recoveryBaseline)void runRecoveryGuard(recoveryBaseline,recoveryStartedAt,p.item.name)}
    catch(e){$('smartApplyPlan').insertAdjacentHTML('afterbegin',`<div class="smart-apply-error">Smart Apply FAIL: ${esc(e.message)}. Если runtime начал mutation, серверный ответ содержит rollback state.</div>`);notice(`Smart Apply: ${e.message}`,'error')}
    finally{smartApplyBusy=false;$('refreshSmartApply').disabled=false;if(smartApplyPlan)renderSmartApplyPlan()}
  }



  let clientHelloGenerated=null,clientHelloCaptured=null,clientHelloCapabilities=null;

  function clientHelloDefaultName(sni){
    return `tls_clienthello_${String(sni||'site').toLowerCase().replace(/[^a-z0-9]+/g,'_').replace(/^_+|_+$/g,'')}.bin`;
  }
  function clientHelloCapturedName(sni){
    return `tls_clienthello_from_${String(sni||'captured').toLowerCase().replace(/[^a-z0-9]+/g,'_').replace(/^_+|_+$/g,'')}.bin`;
  }
  function clientHelloBlobID(name){
    const base=String(name||'blob').toLowerCase().replace(/\.bin$/,'').replace(/[^a-z0-9_]+/g,'_').replace(/^_+|_+$/g,'');
    return `rf_${base||'clienthello'}`;
  }
  function renderClientHelloUsage(){
    const d=clientHelloGenerated,name=String($('clientHelloName').value||'').trim(),sni=String($('clientHelloSNI').value||'').trim();
    if(!d||!d.valid||!name){$('clientHelloUsage').textContent='После создания здесь появятся параметры для nfqws2.';return}
    const id=clientHelloBlobID(name),path=`/opt/etc/nfqws2/blobs/${name}`;
    const lines=[`--blob=${id}:@${path}`];
    if(clientHelloCapabilities?.dynamic_clone_available){
      lines.push(`--lua-desync=tls_client_hello_clone:payload=tls_client_hello:dir=out:blob=rf_live_clienthello:sni_del:sni_first=${sni}:sni_snt_new=0:fallback=${id}`);
      lines.push(`--lua-desync=fake:payload=tls_client_hello:dir=out:blob=rf_live_clienthello:tls_mod=rnd,dupsid`);
    }else{
      lines.push(`--lua-desync=fake:payload=tls_client_hello:dir=out:blob=${id}:tls_mod=rnd,dupsid`);
    }
    $('clientHelloUsage').textContent=lines.join('\n');
  }
  async function loadClientHelloCapabilities(){
    try{
      const d=await api('/clienthello/capabilities');clientHelloCapabilities=d;
      $('clientHelloCloneCapability').textContent=d.dynamic_clone_available?'CLONE: ЕСТЬ':'STATIC BLOB';
      $('clientHelloCloneCapability').className='config-badge '+(d.dynamic_clone_available?'active':'');
      const tcp=d.tcpdump_available?`tcpdump: ЕСТЬ · ${d.tcpdump_path||'PATH'}`:'tcpdump: НЕТ';
      const lua=d.lua_available?`zapret-antidpi.lua: НАЙДЕН${d.dynamic_clone_source?` · ${d.dynamic_clone_source}`:''}`:'zapret-antidpi.lua: НЕ НАЙДЕН';
      const clone=d.dynamic_clone_available?'tls_client_hello_clone: ЕСТЬ':`tls_client_hello_clone: НЕТ · ${d.clone_reason||'причина неизвестна'}`;
      const read=d.lua_read_error?` · ошибка чтения: ${d.lua_read_error}`:'';
      $('clientHelloCapabilitiesDetail').textContent=`${tcp} | ${lua} | ${clone}${read}`;
      $('clientHelloCapture').disabled=!d.tcpdump_available;
      renderClientHelloUsage();
    }catch(e){
      $('clientHelloCloneCapability').textContent='НЕИЗВЕСТНО';
      $('clientHelloCapabilitiesDetail').textContent=`Проверка возможностей: ${e.message}`;
    }
  }
  function renderClientHelloResult(d,title){
    const extra=[
      Array.isArray(d.alpn)&&d.alpn.length?`ALPN ${d.alpn.join(', ')}`:'',
      Array.isArray(d.supported_versions)&&d.supported_versions.length?d.supported_versions.join(', '):'',
      d.extension_count?`${d.extension_count} extensions`:''
    ].filter(Boolean).join(' · ');
    $('clientHelloResult').innerHTML=`<div class="diagnostic ${d.valid?'good':'bad'}"><div class="diagnostic-icon">${d.valid?'✓':'!'}</div><div><div class="diagnostic-title">${esc(title)} · ${esc(d.sni||'no SNI')}</div><div class="diagnostic-detail">${esc(d.detail||'')} · ${Number(d.size||0)} bytes · sha256 ${esc(String(d.sha256||'').slice(0,20))}…${extra?` · ${esc(extra)}`:''}</div></div></div>`;
  }

  async function generateClientHelloLab(){
    const sni=String($('clientHelloSNI').value||'').trim();
    $('clientHelloState').textContent='RUNNING';$('clientHelloSave').disabled=true;clientHelloGenerated=null;
    try{
      const d=await api('/clienthello/generate?sni='+encodeURIComponent(sni));
      clientHelloGenerated=d;$('clientHelloState').textContent=d.valid?'VALID':'INVALID';
      if(!$('clientHelloName').value.trim())$('clientHelloName').value=clientHelloDefaultName(d.sni||sni);
      renderClientHelloResult(d,'Синтетический ClientHello');
      $('clientHelloSave').disabled=!d.valid;renderClientHelloUsage();
    }catch(e){$('clientHelloState').textContent='ОШИБКА';$('clientHelloResult').innerHTML=`<div class="notice error">${esc(e.message)}</div>`}
  }

  async function forgeClientHello(){
    const sni=String($('clientHelloSNI').value||'').trim();
    if(!clientHelloCaptured)return;
    $('clientHelloState').textContent='FORGING';$('clientHelloSave').disabled=true;clientHelloGenerated=null;
    try{
      const d=await api('/clienthello/forge',{method:'POST',body:JSON.stringify({content_base64:clientHelloCaptured.content_base64,sni})});
      clientHelloGenerated=d;$('clientHelloState').textContent=d.valid?'VALID':'INVALID';
      if(!$('clientHelloName').value.trim())$('clientHelloName').value=clientHelloDefaultName(d.sni||sni);
      renderClientHelloResult(d,`Клон ${d.source_sni||'captured'} →`);
      $('clientHelloSave').disabled=!d.valid;renderClientHelloUsage();
    }catch(e){$('clientHelloState').textContent='ОШИБКА';$('clientHelloResult').innerHTML=`<div class="notice error">${esc(e.message)}</div>`}
  }

  async function saveClientHelloLab(){
    const d=clientHelloGenerated,name=String($('clientHelloName').value||'').trim();
    if(!d||!d.valid)return;
    const existing=blobInstalledByName(name),replace=!!existing;
    if(replace&&!confirm(`Заменить ${name}? Будет создан safety backup. Runtime не перезапускается.`))return;
    $('clientHelloSave').disabled=true;
    try{
      const result=await api('/blobs/install',{method:'POST',body:JSON.stringify({
        name,content_base64:d.content_base64,expected_sha256:d.sha256,replace,
        confirm:replace?'NFQWS_BLOB_REPLACE':'NFQWS_BLOB_INSTALL'
      })});
      blobFiles=Array.isArray(result.inventory?.files)?result.inventory.files:blobFiles;blobLoaded=true;renderBlobs();
      $('clientHelloState').textContent='СОХРАНЕНО';renderClientHelloUsage();
      notice(`${name}: рабочий ClientHello blob сохранён. Конфиг и runtime не менялись.`,'good');
    }catch(e){$('clientHelloState').textContent='ОШИБКА';notice(`ClientHello save: ${e.message}`,'error');$('clientHelloSave').disabled=false}
  }

  async function captureClientHelloDevice(){
    const deviceIP=String($('clientHelloDeviceIP').value||'').trim(),iface=String($('clientHelloInterface').value||'').trim();
    $('clientHelloCaptureState').textContent='CAPTURING';$('clientHelloCapture').disabled=true;
    try{
      const d=await api('/clienthello/capture',{method:'POST',body:JSON.stringify({device_ip:deviceIP,interface:iface,seconds:5})});
      $('clientHelloCaptureState').textContent=`${Number(d.count||0)} FOUND`;
      const rows=Array.isArray(d.candidates)?d.candidates:[];
      const stats=d.capture_stats||{},diag=`Интерфейс: ${d.interface||'?'}. TCP: ${Number(stats.tcp_packets||0)}, TCP с данными: ${Number(stats.tcp_payload_packets||0)}, UDP/443: ${Number(stats.udp_443_packets||0)}.`;
      $('clientHelloCaptureResult').innerHTML=rows.length?rows.map((x,i)=>`<div class="clienthello-candidate">
        <div><strong>${esc(x.sni||'ClientHello без SNI')}</strong><span>${esc(x.src_ip||'')} → ${esc(x.dst_ip||'')}:${Number(x.dst_port||0)} · ${Number(x.size||0)} bytes · ${esc(x.detail||'')}</span></div>
        <button data-clienthello-use="${i}" ${x.valid?'':'disabled'}>Использовать как основу</button>
      </div>`).join(''):`<div class="empty">${esc(d.hint||'За 5 секунд ClientHello не найден.')}<br>${esc(diag)}</div>`;
      $('clientHelloCaptureResult').querySelectorAll('[data-clienthello-use]').forEach(b=>b.onclick=()=>{
        const x=rows[Number(b.dataset.clienthelloUse)];if(!x)return;
        clientHelloCaptured=x;clientHelloGenerated=null;
        $('clientHelloForge').disabled=!x.valid;
        $('clientHelloState').textContent='CAPTURED';$('clientHelloSave').disabled=true;
        $('clientHelloResult').innerHTML=`<div class="diagnostic good"><div class="diagnostic-icon">✓</div><div><div class="diagnostic-title">Основа: ${esc(x.sni||'ClientHello без SNI')}</div><div class="diagnostic-detail">${Number(x.size||0)} bytes · порт ${Number(x.dst_port||0)} · sha256 ${esc(String(x.sha256||'').slice(0,20))}…</div></div></div>`;
        if(!$('clientHelloSNI').value.trim())$('clientHelloSNI').value='www.google.com';
        $('clientHelloName').value=clientHelloCapturedName(x.sni||'captured');
        renderClientHelloUsage();
      });
    }catch(e){$('clientHelloCaptureState').textContent='ОШИБКА';$('clientHelloCaptureResult').innerHTML=`<div class="notice error">${esc(e.message)}</div>`}
    finally{$('clientHelloCapture').disabled=false}
  }

  $('clientHelloForge').onclick=forgeClientHello;
  $('clientHelloGenerate').onclick=generateClientHelloLab;
  $('clientHelloSave').onclick=saveClientHelloLab;
  $('clientHelloCapture').onclick=captureClientHelloDevice;
  $('clientHelloSNI').oninput=()=>{
    if(!clientHelloCaptured&&$('clientHelloSNI').value.trim())$('clientHelloName').value=clientHelloDefaultName($('clientHelloSNI').value);
    renderClientHelloUsage();
  };
  $('clientHelloName').oninput=renderClientHelloUsage;

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
    if(blobLoading||blobLoaded&&!force){await refreshBlobDependencies();renderBlobs();return}blobLoading=true;
    try{const d=await api('/blobs');blobFiles=Array.isArray(d.files)?d.files:[];blobRoot=d.root||'/opt/etc/nfqws2/blobs';blobLoaded=true;await refreshBlobDependencies();renderBlobs()}
    catch(e){blobLoaded=false;notice(`Blob inventory: ${e.message}`,'error');if($('blobInventoryList'))$('blobInventoryList').innerHTML=`<div class="empty bad-text">${esc(e.message)}</div>`}
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
  function renderBlobs(){
    if(!$('blobInventoryList'))return;const deps=blobDependencyRows(),missing=deps.filter(x=>x.state!=='ok').length,referenced=blobFiles.filter(x=>x.referenced).length;
    $('blobInstalledCount').textContent=blobFiles.length;$('blobRequiredCount').textContent=deps.length;$('blobMissingCount').textContent=missing;$('blobReferencedCount').textContent=referenced;$('blobTabState').textContent=missing?`(${missing}!)`:blobFiles.length?`(${blobFiles.length})`:'';
    $('blobRootMeta').textContent=blobRoot||'/opt/etc/nfqws2/blobs';
    $('blobInventoryList').innerHTML=blobFiles.length?blobFiles.map(x=>`<div class="blob-row"><div><div class="blob-name">${esc(x.name)}</div><div class="blob-meta">${bytes(x.size)} · sha256 ${esc(String(x.sha256||'').slice(0,16))}…</div><div class="blob-refs">${x.referenced?`refs: ${esc((x.references||[]).join(', '))}`:'unreferenced'}</div></div><div class="blob-actions"><span class="blob-state ok">INSTALLED</span><button data-blob-validate="${esc(x.name)}">ClientHello?</button><button class="danger" data-blob-delete="${esc(x.name)}" ${x.referenced?'disabled title="Referenced blob cannot be deleted"':''}>Удалить</button></div></div>`).join(''):'<div class="empty">Blob-файлы не найдены.</div>';
    $('blobInventoryList').querySelectorAll('[data-blob-validate]').forEach(b=>b.onclick=()=>validateInstalledClientHello(b.dataset.blobValidate));
    $('blobInventoryList').querySelectorAll('[data-blob-delete]').forEach(b=>b.onclick=()=>deleteBlob(b.dataset.blobDelete));
    if(!catalogSelectedData){$('blobCatalogMeta').textContent='Выберите config в Catalog';$('blobDependencyList').innerHTML='<div class="empty">Выберите стратегию в Catalog, чтобы проверить её blobs.</div>';return}
    const item=catalogSelectedData.item;$('blobCatalogMeta').textContent=`${item.name} · ${item.kind==='upstream'?`exact ref ${String(item.ref).slice(0,12)}`:item.source}`;
    $('blobDependencyList').innerHTML=deps.length?deps.map(x=>{const canFetch=x.item.kind==='upstream'&&x.expected,action=x.state==='missing'?'Установить':x.state==='update'?'Заменить':'';return `<div class="blob-dep-row"><div><div class="blob-name">${esc(x.name||x.dep.name)}</div><div class="blob-meta">symbol=${esc(x.dep.name)}${x.expected?` · expected ${esc(x.expected.sha256.slice(0,16))}…`:' · upstream hash unavailable'}${x.installed?` · local ${esc(String(x.installed.sha256).slice(0,16))}…`:''}</div></div><div class="blob-actions"><span class="blob-state ${x.state}">${x.state==='ok'?'VERIFIED':x.state==='update'?'UPDATE':'MISSING'}</span>${action?`<button data-blob-install="${esc(x.name)}" ${canFetch?'':'disabled'}>${esc(action)}</button>`:''}</div></div>`}).join(''):'<div class="empty">У выбранного config явные --blob dependencies не найдены.</div>';
    $('blobDependencyList').querySelectorAll('[data-blob-install]').forEach(b=>b.onclick=()=>installCatalogBlob(b.dataset.blobInstall));
  }
  async function installCatalogBlob(name){
    const row=blobDependencyRows().find(x=>x.name===name);if(!row||row.item.kind!=='upstream'||!row.expected)return;
    const replace=!!row.installed;if(replace&&!confirm(`Заменить ${name}? Будет создан safety backup. Running nfqws НЕ перезапускается; новый файл на диске вступит в силу только при будущем controlled restart/apply.`))return;
    if(!replace&&!confirm(`Установить ${name} из ${catalogRepo} @ ${String(row.item.ref).slice(0,12)} с проверкой SHA256?`))return;
    const button=document.querySelector(`[data-blob-install="${CSS.escape(name)}"]`);if(button)button.disabled=true;
    try{
      const r=await catalogFetch(`https://raw.githubusercontent.com/${catalogRepo}/${row.item.ref}/${row.expected.path}`,12000),buffer=await r.arrayBuffer();if(buffer.byteLength>1048576)throw new Error('blob > 1 MiB safety limit');
      const actual=await blobSha256Bytes(buffer);if(actual!==row.expected.sha256)throw new Error(`SHA256 mismatch before install: ${actual}`);
      const d=await api('/blobs/install',{method:'POST',body:JSON.stringify({name,content_base64:blobBase64(buffer),expected_sha256:row.expected.sha256,replace,confirm:replace?'NFQWS_BLOB_REPLACE':'NFQWS_BLOB_INSTALL'})});
      blobFiles=Array.isArray(d.inventory?.files)?d.inventory.files:blobFiles;blobLoaded=true;renderBlobs();notice(`${name}: ${replace?'заменён':'установлен'}; SHA256 verified. Runtime restart не выполнялся.`,'good')
    }catch(e){notice(`Blob ${name}: ${e.message}`,'error');renderBlobs()}
  }
  async function deleteBlob(name){
    const item=blobInstalledByName(name);if(!item||item.referenced)return;if(!confirm(`Удалить unreferenced blob ${name}? Перед удалением будет создан safety backup.`))return;
    try{const d=await api('/blobs/delete',{method:'POST',body:JSON.stringify({name,confirm:'NFQWS_BLOB_DELETE'})});blobFiles=Array.isArray(d.inventory?.files)?d.inventory.files:blobFiles;renderBlobs();notice(`${name}: удалён, backup=${d.backup||'created'}`,'good')}catch(e){notice(`Удаление ${name}: ${e.message}`,'error')}
  }



  async function validateInstalledClientHello(name){
    try{
      const d=await api(`/clienthello/validate?name=${encodeURIComponent(name)}`);
      notice(`${name}: ${d.valid?'VALID ClientHello':'not a valid ClientHello'}${d.sni?` · SNI ${d.sni}`:''} · ${d.detail||''}`,d.valid?'good':'error');
    }catch(e){notice(`ClientHello validate ${name}: ${e.message}`,'error')}
  }

  let v2MemoryData=null;
  function renderV2TargetMemory(){
    const d=v2MemoryData||{},rows=Array.isArray(d.entries)?d.entries:[];
    $('v2MemoryTabCount').textContent=rows.length?`(${rows.length})`:'';
    $('v2MemoryState').textContent=`${rows.length} ENTRIES`;
    const p=d.policy||{};
    $('v2MemoryPolicy').textContent=`policy v${p.version||'—'} · working TTL ${Math.round(Number(p.working_max_age_seconds||0)/3600)}h · unstable TTL ${Math.round(Number(p.unstable_max_age_seconds||0)/3600)}h · strong verified≥${Number(p.strong_min_verified||0)} · trusted verified≥${Number(p.trusted_min_verified||0)}`;
    $('v2MemoryList').innerHTML=rows.length?rows.map(x=>{
      const confidence=String(x.confidence||'BLOCKED'),good=!!x.reuse_eligible;
      return `<div class="v2-row ${good?'good':'warn'}"><div class="v2-result-title">${esc(x.target||'—')}</div><div><div class="value">${esc(x.candidate_name||x.candidate_id||'candidate')} · ${esc(x.protocol||'https')} · ${esc(confidence)}</div><div class="meta">${esc(x.candidate_source||'')} · ${esc(x.result_class||'')} · verified=${Number(x.verified_count||0)} · streak=${Number(x.success_streak||0)} · success=${Math.round(Number(x.success_rate||0)*100)}% · age=${Number(x.age_seconds||0)}s<br>${esc(x.reuse_reason||'')}</div><details><summary>args / fingerprint</summary><pre class="v2-args">${esc((x.args||[]).join(' '))}\n\n${esc(x.fingerprint||'')}</pre></details></div>${v2Pill(good?'REUSE':'HISTORY')}</div>`;
    }).join(''):'<div class="analysis-empty">Target Memory пуста.</div>';
    reportHeight();
  }
  async function loadV2TargetMemory(){
    const filter=String($('v2MemoryFilter')?.value||'').trim();
    $('v2MemoryState').textContent='LOADING';
    try{
      v2MemoryData=await api('/v2/memory'+(filter?`?target=${encodeURIComponent(filter)}`:''));
      renderV2TargetMemory();
    }catch(e){$('v2MemoryState').textContent='ОШИБКА';$('v2MemoryList').innerHTML=`<div class="notice error">${esc(e.message)}</div>`}
  }
  async function clearV2TargetMemory(all=false){
    const target=all?'':String($('v2MemoryFilter')?.value||'').trim();
    if(!all&&!target){notice('Target Memory: для точечной очистки укажите target.','error');return}
    const label=all?'ВСЮ Target Memory':`Target Memory для ${target}`;
    if(!confirm(`Очистить ${label}?\n\nЭто удаляет только СЃРѕС…СЂР°РЅС‘РЅРЅСѓСЋ РёСЃС‚РѕСЂРёСЋ СЂРµР·СѓР»СЊС‚Р°С‚РѕРІ. Production config/runtime не меняются.`))return;
    try{
      await api('/v2/memory/clear',{method:'POST',body:JSON.stringify({target,confirm:'ROUTERFORGE_V2_MEMORY_CLEAR'})});
      await loadV2TargetMemory();notice(`${label}: очищена. Production untouched.`,'good');
    }catch(e){notice(`Target Memory clear: ${e.message}`,'error')}
  }

  async function loadRuntimeHealthV2(){
    $('runtimeHealthState').textContent='LOADING';
    try{
      const [q,t,p]=await Promise.all([
        api('/v2/nfqueue-health'),
        api('/v2/bench-profiles'),
        api('/v2/policy-automation-gate')
      ]);
      const qClass=q.state==='HEALTHY'?'good':q.state==='UNHEALTHY'?'bad':'warn';
      $('runtimeNFQueueHealth').innerHTML=`<div class="diagnostic ${qClass}"><div class="diagnostic-icon">${q.ok?'✓':'!'}</div><div><div class="diagnostic-title">${esc(q.state||'UNKNOWN')} · production q${esc((q.production_queues||[]).join(','))}</div><div class="diagnostic-detail">${esc(q.reason||'')}<br>firewall=${esc((q.firewall_queues||[]).join(','))} · kernel=${esc((q.kernel_queues||[]).join(','))} · config ${esc(String(q.config_sha256||'').slice(0,16))}… · read-only=${q.read_only?'YES':'NO'}</div></div></div>`;
      $('runtimeTransportProfiles').innerHTML=(t.profiles||[]).map(x=>`<div class="diagnostic ${x.implemented?'good':'warn'}"><div class="diagnostic-icon">${x.implemented?'✓':'i'}</div><div><div class="diagnostic-title">${esc(String(x.id||'').toUpperCase())} · ${esc(x.network||'')} / ${Number(x.remote_port||0)}</div><div class="diagnostic-detail">${esc(x.metric_scope||'')} · ${esc(x.probe_kind||'')} · timeout ${Number(x.probe_timeout_seconds||0)}s · IPv4=${x.ipv4?'YES':'NO'}</div></div></div>`).join('')||'<div class="analysis-empty">Transport profiles отсутствуют.</div>';
      $('runtimeAutomationPolicy').innerHTML=`<div class="diagnostic ${p.automation_enabled?'good':'warn'}"><div class="diagnostic-icon">${p.automation_enabled?'✓':'i'}</div><div><div class="diagnostic-title">Automation ${p.automation_enabled?'ENABLED':'GATED'}</div><div class="diagnostic-detail">${esc(p.reason||p.automation_reason||'policy gate loaded')} · historical-only evidence cannot create Apply eligibility.</div></div></div>`;
      $('runtimeHealthTabState').textContent=q.state||'';
      $('runtimeHealthState').textContent=q.state||'READY';
    }catch(e){
      $('runtimeHealthState').textContent='ОШИБКА';
      $('runtimeNFQueueHealth').innerHTML=`<div class="notice error">${esc(e.message)}</div>`;
    }
    reportHeight();
  }

  async function pruneBackupCenter(){
    if(!confirm('Очистить старые NFQWS backups по встроенной retention policy?\n\nRuntime/config не меняются. Удаляются только snapshots сверх retention limits.'))return;
    try{
      const d=await api('/backups/prune',{method:'POST',body:JSON.stringify({confirm:'NFQWS_BACKUP_PRUNE'})});
      backupCenterLoaded=false;await loadBackupCenter(true);
      notice(`Backup prune: persistent ${Number(d.persistent_before||0)} → ${Number(d.persistent_after||0)}, safety ${Number(d.safety_before||0)} → ${Number(d.safety_after||0)}.`,'good');
    }catch(e){notice(`Backup prune: ${e.message}`,'error')}
  }

  async function saveGeoProvenance(){
    const asset=geoAssetSelected();if(!asset){notice('Geo provenance: выберите asset.','error');return}
    const source=String($('geoSource').value||'').trim(),sourceRef=String($('geoSourceRef').value||'').trim();
    if(!confirm(`Обновить provenance metadata для ${asset.name}?\n\nAsset bytes, lists и production config не меняются.`))return;
    try{
      const d=await api('/v2/geo/source',{method:'POST',body:JSON.stringify({name:asset.name,source,source_ref:sourceRef,source_url:asset.source_url||'',confirm:'ROUTERFORGE_GEO_SOURCE'})});
      await loadGeoAssets();notice(`Geo provenance ${d.asset?.name||asset.name}: saved. Production untouched.`,'good');
    }catch(e){notice(`Geo provenance: ${e.message}`,'error')}
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
  function autoPickEligible(row){return row.runs>=2&&row.targets>=3&&row.reach>=.75&&row.stable>=.60}
  async function renderAutoPick(){
    if(!$('autoPickSummary'))return;
    if(!configLibraryLoaded){try{await loadConfigLibrary()}catch(_){}}
    const rows=testLabEvidenceByConfig(),eligible=rows.filter(autoPickEligible),winner=eligible[0]||null;
    const state=$('recoveryGuardState'),enabled=!!$('recoveryGuardEnabled')?.checked,baseline=latestRecoveryBaselineForCurrent();
    if(state){state.textContent=enabled&&baseline?'ARMED':enabled?'NO BASELINE':'DISARMED';state.className='config-badge '+(enabled&&baseline?'active':'')}
    if(!rows.length){$('autoPickSummary').innerHTML='<div class="analysis-empty">Нет измеренных Test Lab run. Сначала прогони реальный active runtime хотя бы два раза на ≥3 целях.</div>';return}
    const item=row=>`<div class="diagnostic ${autoPickEligible(row)?'good':'warn'}"><div class="diagnostic-icon">${winner===row?'★':'i'}</div><div><div class="diagnostic-title">${esc(autoPickMapName(row.sha))} · median ${Math.round(row.median_score)}</div><div class="diagnostic-detail">runs ${row.runs} · targets ${row.targets} · reach ${Math.round(row.reach*100)}% · stable ${Math.round(row.stable*100)}% · min ${Math.round(row.min_score)} · latest ${Math.round(row.latest_score)}${autoPickEligible(row)?' · ELIGIBLE':' · NEED MORE EVIDENCE'}</div></div></div>`;
    $('autoPickSummary').innerHTML=(winner?`<div class="setup-note" style="margin-bottom:8px"><b>Auto-pick:</b> ${esc(autoPickMapName(winner.sha))}. Это лучший из <u>измеренных active runtime</u> по median ACTIVE SCORE; выбор не применяет config автоматически.</div>`:'<div class="setup-note warn" style="margin-bottom:8px">Пока нет eligible config: нужно ≥2 Test Lab runs, ≥3 targets, reach ≥75%, stable ≥60%.</div>')+rows.slice(0,6).map(item).join('');
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
  async function runRecoveryGuard(baseline,startedAt,label){
    if(recoveryGuardRunning||!baseline)return;recoveryGuardRunning=true;
    const state=$('recoveryGuardState');if(state){state.textContent='PROBING';state.className='config-badge active'}
    try{
      const b=recoveryBaselineMetrics(baseline),after=await recoveryProbeTargets(b.targets,Math.min(3,b.rounds)),threshold=(Number($('recoveryGuardThreshold')?.value)||20)/100,reachDrop=b.reach-after.reach,stableDrop=b.stable-after.stable,regressed=reachDrop>=threshold||stableDrop>=threshold;
      if(!regressed){
        recoveryGuardLastResult={state:'PASS',label,reach_before:b.reach,reach_after:after.reach,stable_before:b.stable,stable_after:after.stable};
        if($('recoveryGuardLast'))$('recoveryGuardLast').textContent=`PASS · ${label} · reach ${Math.round(b.reach*100)}→${Math.round(after.reach*100)}% · stable ${Math.round(b.stable*100)}→${Math.round(after.stable*100)}%`;
        notice('Recovery Guard: PASS · regression threshold не достигнут.','good');return;
      }
      if(state){state.textContent='RECOVERING';state.className='config-badge'}
      const snapshots=await recoverySnapshotSet(startedAt),restored=await restoreRecoverySnapshots(snapshots);
      await load();backupCenterLoaded=false;await loadBackupCenter(true);
      recoveryGuardLastResult={state:'RECOVERED',label,reach_before:b.reach,reach_after:after.reach,stable_before:b.stable,stable_after:after.stable,restored:restored.map(x=>x.id)};
      if($('recoveryGuardLast'))$('recoveryGuardLast').textContent=`RECOVERED · regression reach ${Math.round(b.reach*100)}→${Math.round(after.reach*100)}%, stable ${Math.round(b.stable*100)}→${Math.round(after.stable*100)}% · restored ${restored.length} snapshot(s)`;
      notice(`Recovery Guard: regression detected; restored ${restored.length} persistent snapshot(s).`,'error');
    }catch(e){
      recoveryGuardLastResult={state:'ERROR',error:e.message};if($('recoveryGuardLast'))$('recoveryGuardLast').textContent=`ERROR · ${e.message}`;notice(`Recovery Guard: ${e.message}`,'error');
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
    let rows;
    if(entry.kind==='testlab')rows=[['Reachability',percentText(m.success_ratio)],['Stable targets',percentText(m.stable_ratio)],['Median latency',m.median_ms==null?'—':`${Math.round(m.median_ms)} ms`],['Static decisive',percentText(m.static_decisive_ratio)]];
    else if(entry.kind==='sandbox')rows=[['Candidate MATCH',m.candidate_match||0],['Candidate EXCLUDED',m.candidate_excluded||0],['Candidate NO MATCH',m.candidate_no_match||0],['Validation warnings',m.validation_warnings||0],['List refs resolved',percentText(m.reference_resolution_ratio)],['Deterministic classification',percentText(m.determinate_ratio)],['Active baseline success',percentText(m.active_baseline_success_ratio)],['Active baseline median',m.active_baseline_median_ms==null?'—':`${Math.round(m.active_baseline_median_ms)} ms`]];
    else if(entry.kind==='v2-selector')rows=[['Candidates tested',m.candidates||0],['Working candidates',m.working||0],['Candidate success ratio',percentText(m.success_ratio)],['Recommendation',m.recommendation?'AVAILABLE':'NONE']];
    else rows=[['Targets',m.targets||0],['Попытки',m.attempts||0],['Success ratio',percentText(m.success_ratio)]];
    return rows.map(([k,v])=>`<dt>${esc(k)}</dt><dd>${esc(v)}</dd>`).join('');
  }

    function renderRunHistory(){
    if(!$('runHistoryList'))return;const visible=runHistory.filter(x=>runHistoryFilter==='all'||x.kind===runHistoryFilter),latest=runHistory[0],json=JSON.stringify(runHistory);
    $('runHistoryCount').textContent=runHistory.length;$('runHistoryLatestScore').textContent=latest?`${latest.score} ${latest.score_type==='READINESS'?'R':'S'}`:'—';$('runHistorySize').textContent=`${Math.max(0.1,new Blob([json]).size/1024).toFixed(1)} KB`;$('runHistoryTabState').textContent=runHistory.length?String(runHistory.length):'';$('copyRunHistory').disabled=!runHistory.length;$('clearRunHistory').disabled=!runHistory.length;
    for(const[id,f]of[['runHistoryAll','all'],['runHistoryTestLab','testlab'],['runHistorySandbox','sandbox']])$(id)?.classList.toggle('active',runHistoryFilter===f);
    if(!visible.length){$('runHistoryList').innerHTML='<div class="run-history-empty">Завершённых прогонов в этом фильтре пока нет.</div>';return}
    $('runHistoryList').innerHTML=visible.map(entry=>{const delta=runHistoryDelta(entry),date=new Date(entry.created_at),stamp=Number.isNaN(date.getTime())?entry.created_at:date.toLocaleString(),chips=runHistoryChips(entry).map(x=>`<span class="run-history-chip">${esc(x)}</span>`).join('');return `<details class="run-history-item"><summary><div><div class="run-history-kind ${entry.kind}">${runHistoryKindLabel(entry.kind)}</div><div class="run-history-time">${esc(stamp)}</div></div><div class="run-history-main">${esc((entry.active_config_sha256||'').slice(0,12)||'no config SHA')} · ${esc(entry.active_mode||'—')}</div><div><div class="run-history-score ${scoreClass(entry.score)}">${entry.score}</div><div class="run-history-time">${esc(entry.score_type||'SCORE')}</div></div><div class="run-history-metrics">${chips}</div><div class="run-history-delta ${delta.cls}">${esc(delta.text)}</div></summary><div class="run-history-body"><dl class="run-history-kv"><dt>Scope</dt><dd>${esc(entry.scope||'—')}</dd><dt>Runtime</dt><dd>${entry.runtime_running?'RUNNING':'STOPPED'}</dd><dt>Config SHA</dt><dd class="mono">${esc(entry.active_config_sha256||'—')}</dd>${runHistoryDetails(entry)}</dl><div class="run-history-actions"><button type="button" data-run-copy="${esc(entry.id)}">JSON</button><button type="button" class="danger" data-run-delete="${esc(entry.id)}">Удалить</button></div></div></details>`}).join('');
    $('runHistoryList').querySelectorAll('[data-run-copy]').forEach(b=>b.onclick=e=>{e.preventDefault();const item=runHistory.find(x=>x.id===b.dataset.runCopy);if(item){navigator.clipboard?.writeText(JSON.stringify(item,null,2));notice('JSON прогона скопирован.','good')}});
    $('runHistoryList').querySelectorAll('[data-run-delete]').forEach(b=>b.onclick=e=>{e.preventDefault();runHistory=runHistory.filter(x=>x.id!==b.dataset.runDelete);runHistoryPersist();renderRunHistory()});
  }
  function clearRunHistory(){
    if(!runHistory.length||!confirm(`Удалить ${runHistory.length} записей локальной истории?`))return;runHistory=[];runHistoryPersist();renderRunHistory();notice('Локальная история прогонов очищена.','good');
  }
  function copyRunHistory(){
    if(!runHistory.length)return;navigator.clipboard?.writeText(JSON.stringify({exported_at:new Date().toISOString(),schema:'routerforge-nfqws-run-history-v1',runs:runHistory},null,2));notice('История прогонов скопирована в JSON.','good');
  }

  async function runDomainCheck(){
    const domains=extractDomains($('listEditor').value);if(!domains.length){notice('В выбранном списке не найдено доменов.','error');return}
    const gen=++checkGeneration;$('checkModal').classList.add('open');$('checkTotal').textContent=domains.length;$('checkOk').textContent=0;$('checkFail').textContent=0;$('checkProgress').textContent='0%';$('checkBar').style.width='0%';
    $('checkResults').innerHTML=domains.map(d=>`<div class="domain-row pending" data-domain="${esc(d)}"><span>${esc(d)}</span><span class="result">WAIT</span></div>`).join('');
    let next=0,done=0,ok=0,fail=0;
    async function worker(){while(gen===checkGeneration){const i=next++;if(i>=domains.length)return;const d=domains[i];let reachable=false,code='';try{const r=await api('/check',{method:'POST',body:JSON.stringify({url:d})});reachable=!!r.reachable;code=r.status_code||''}catch(_){}
      if(gen!==checkGeneration)return;done++;reachable?ok++:fail++;const row=[...document.querySelectorAll('.domain-row')].find(x=>x.dataset.domain===d);if(row){row.className='domain-row '+(reachable?'ok':'fail');row.querySelector('.result').className='result '+(reachable?'ok':'fail');row.querySelector('.result').textContent=reachable?`OK ${code}`:'FAIL'}
      const p=Math.round(done/domains.length*100);$('checkOk').textContent=ok;$('checkFail').textContent=fail;$('checkProgress').textContent=`${p}%`;$('checkBar').style.width=`${p}%`;
    }}
    await Promise.all(Array.from({length:Math.min(4,domains.length)},worker));
  }
  function stopDomainCheck(){checkGeneration++}
  function closeCheck(){stopDomainCheck();$('checkModal').classList.remove('open')}


  function keeneticBytes(n){
    const value=Number(n||0);if(!value)return'—';if(value<1048576)return`${Math.round(value/1024)} KB`;if(value<1073741824)return`${(value/1048576).toFixed(1)} MB`;return`${(value/1073741824).toFixed(1)} GB`;
  }
  function keeneticDiag(kind,title,detail){
    const icon=kind==='good'?'✓':kind==='bad'?'!':kind==='warn'?'!':'i';
    return `<div class="diagnostic ${kind}"><div class="diagnostic-icon">${icon}</div><div><div class="diagnostic-title">${esc(title)}</div><div class="diagnostic-detail">${esc(detail||'')}</div></div></div>`;
  }
  function renderKeenetic(){
    const d=keeneticSnapshot;if(!$('keeneticState'))return;
    if(!d){
      $('keeneticState').textContent=keeneticLoading?'LOADING':'IDLE';$('keeneticTabState').textContent='';
      return;
    }
    $('keeneticState').textContent=d.detected?'KEENETIC':'GENERIC';$('keeneticState').className='config-badge '+(d.detected?'active':'');
    $('keeneticTabState').textContent=d.detected?'✓':'•';
    $('keeneticPlatform').textContent=d.detected?'KEENETIC':'GENERIC';
    $('keeneticNdmc').textContent=d.ndmc_available?'READY':'NO';
    $('keeneticRci').textContent=d.rci_available?'READY':'NO';
    $('keeneticEntware').textContent=d.opkg_available?'OPKG':'CHECK';
    $('keeneticModelMeta').textContent=[d.model||'model unknown',d.version||'version unknown'].join(' · ');

    const interfaces=Array.isArray(d.interfaces)?d.interfaces:[];
    $('keeneticNetwork').innerHTML=[
      keeneticDiag(d.default_ipv4_interface?'good':'warn','Default IPv4 route',d.default_ipv4_interface||'не определён'),
      ...interfaces.map(x=>keeneticDiag(x.default_v4?'good':x.up?'info':'warn',`${x.name}${x.default_v4?' · DEFAULT':''}`,`operstate=${x.oper_state||'unknown'}`)),
      ...(d.warnings||[]).map(x=>keeneticDiag('warn','Read warning',x))
    ].join('');

    const configISP=d.config_isp_interface||'',def=d.default_ipv4_interface||'',match=!!d.config_isp_matches_default;
    $('keeneticBinding').innerHTML=[
      keeneticDiag(configISP?'info':'warn','Configured ISP_INTERFACE',configISP||'не задан'),
      keeneticDiag(def?'info':'warn','Kernel default-route interface',def||'не определён'),
      keeneticDiag(configISP&&def?(match?'good':'warn'):'warn',match?'Binding MATCH':'Binding CHECK',configISP&&def?(match?'ISP_INTERFACE совпадает с default IPv4 route.':'Значения различаются. Это может быть нормально при policy routing/VLAN/PPP, поэтому RouterForge ничего не меняет автоматически.'):'Недостаточно данных для сравнения.')
    ].join('');
    $('keeneticUseDefault').disabled=!def;

    const used=d.entware_total_bytes?Math.max(0,Number(d.entware_total_bytes)-Number(d.entware_free_bytes||0)):0;
    $('keeneticStorage').innerHTML=[
      keeneticDiag(d.entware_mounted?'good':'warn','/opt mount',d.entware_mounted?`${d.entware_device||'device unknown'} · ${d.entware_fs_type||'fs unknown'}`:'отдельный mount не подтверждён'),
      keeneticDiag(d.opkg_available?'good':'warn','Entware opkg',d.opkg_available?'/opt/bin/opkg executable':'/opt/bin/opkg не найден или не executable'),
      keeneticDiag(d.entware_total_bytes?'info':'warn','Filesystem',d.entware_total_bytes?`${keeneticBytes(used)} used · ${keeneticBytes(d.entware_free_bytes)} free · ${keeneticBytes(d.entware_total_bytes)} total`:'statfs /opt недоступен')
    ].join('');

    $('keeneticPolicy').innerHTML=[
      keeneticDiag(d.rci_available?'good':'warn','RCI read endpoint',d.rci_available?'GET /rci/show/ip/policy доступен':'endpoint недоступен'),
      keeneticDiag(d.policy_identity?'info':'warn','Policy identity',d.policy_identity||'не получен'),
      keeneticDiag(d.policy_shape?'info':'warn','JSON shape',d.policy_shape?`${d.policy_shape} · top-level items ${d.policy_items||0}`:'не определён')
    ].join('');
    reportHeight();
  }
  async function loadKeenetic(force=false){
    if(keeneticLoading||keeneticLoaded&&!force){renderKeenetic();return}
    keeneticLoading=true;renderKeenetic();
    try{keeneticSnapshot=await api('/keenetic');keeneticLoaded=true;renderKeenetic()}
    catch(e){keeneticSnapshot=null;notice(`Keenetic integration: ${e.message}`,'error');renderKeenetic()}
    finally{keeneticLoading=false;renderKeenetic()}
  }
  function useKeeneticDefaultInterface(){
    const iface=String(keeneticSnapshot?.default_ipv4_interface||'');if(!iface)return;
    const tab=document.querySelector('.tab[data-tab="strategies"]');if(tab)tab.click();
    $('flowInterface').value=iface;updateFlowState();notice(`ISP_INTERFACE draft ← ${iface}. Ничего не сохранено.`,'good');
  }

  function renderDPIDetector(){
    const d=dpiDetectorSnapshot;
    if($('dpiDetectorState')){
      if(!d){$('dpiDetectorState').textContent=dpiDetectorLoading?'LOADING':'IDLE'}
      else{
        $('dpiDetectorState').textContent=d.installed?(d.execution_ready?'READY':'INCOMPATIBLE'):'NOT INSTALLED';
        $('dpiDetectorState').className='config-badge '+(d.installed&&d.execution_ready?'active':'');
        $('dpiDetectorStatus').innerHTML=d.installed?[
          keeneticDiag(d.execution_ready?'good':'warn','Installed',`${d.path||'path unknown'} · ${keeneticBytes(d.size_bytes||0)}`),
          keeneticDiag(d.execution_ready?'good':'warn','Execution',d.execution_ready?'ready':(d.compatibility_reason||d.warning||'not ready')),
          keeneticDiag('info','Upstream',`${d.upstream||'Runnin4ik/dpi-detector'} · arch ${d.host_arch||'unknown'}`),
          keeneticDiag('info','Required interpreter',d.required_interpreter||'none / static')
        ].join(''):keeneticDiag('info','Optional module','DPI Detector v5 не установлен через App Center.');
      }
    }
    renderDPIDetectorWorkspace();
    reportHeight();
  }

  function setDPIDetectorMode(mode){
    dpiWorkspaceMode=mode==='web'?'web':'console';
    renderDPIDetectorWorkspace();
    if(dpiWorkspaceMode==='console')ensureDPIConsole();
    requestAnimationFrame(reportHeight);
    setTimeout(reportHeight,120);
  }

  function dpiConsoleURL(){
    const lang=document.documentElement.lang||'ru';
    return `/api/modules/admin/ui/index.html?locale=${encodeURIComponent(lang)}&view=terminal&embed=terminal&autorun=dpi-detector`;
  }
  function ensureDPIConsole(force=false){
    const frame=$('dpiConsoleFrame');if(!frame)return;
    if(force||!frame.getAttribute('src'))frame.src=dpiConsoleURL()+(force?`&restart=${Date.now()}`:'');
  }
  function restartDPIConsole(){ensureDPIConsole(true)}

  const dpiTestNames={'0':'Network info','1':'DNS availability','2':'Domain inspection','3':'TCP 16–20 KB','4':'Whitelist SNI','5':'Telegram','6':'Fingerprint Burst'};
  function dpiStatusKind(value){
    const v=String(value??'').trim().toLowerCase().replace(/[\s-]+/g,'_');
    if(!v)return'info';
    if(['ok','pass','available','reachable','normal'].includes(v))return'good';
    if(['no_tls13','no_ca','local_ip','dns_fail','nxdomain','stalled','slow','partial','mixed','warn','warning'].includes(v))return'warn';
    if(v==='err')return'info';
    return'bad';
  }
  function dpiChip(value){const text=String(value??'—');return `<span class="setup-chip ${dpiStatusKind(text)}">${esc(text)}</span>`}
  function dpiDetail(value){if(value===null||value===undefined||value==='')return'—';return typeof value==='object'?JSON.stringify(value):String(value)}
  function dpiTable(headers,rows){
    return `<div class="table-scroll dpi-table-scroll"><table><thead><tr>${headers.map(h=>`<th>${esc(h)}</th>`).join('')}</tr></thead><tbody>${rows.join('')}</tbody></table></div>`;
  }
  function dpiNativeMarkerIndex(lines,patterns,start=0){
    for(let i=Math.max(0,start);i<lines.length;i++)if(patterns.some(re=>re.test(lines[i])))return i;
    return -1;
  }
  function dpiNativeMarkers(report){
    const lines=String(report||'').replace(/\r\n?/g,'\n').split('\n');
    const markers=[];
    markers[0]=dpiNativeMarkerIndex(lines,[/Информация о сети и системе/i,/Network\s*&?\s*system information/i,/Network information/i],0);
    markers[1]=dpiNativeMarkerIndex(lines,[/Проверка доступности DNS-серверов/i,/DNS server availability/i,/DNS availability/i],0);
    markers[2]=dpiNativeMarkerIndex(lines,[/Проверка доступности доменов/i,/Результаты проверки доменов/i,/Website availability/i,/Domain availability/i],0);
    markers[3]=dpiNativeMarkerIndex(lines,[/Проверка TCP\s*16/i,/TCP\s*16.?20/i,/CDN.*hosting availability/i],0);
    markers[4]=dpiNativeMarkerIndex(lines,[/Поиск белых SNI/i,/Whitelist SNI/i,/white.?SNI/i],0);
    markers[5]=dpiNativeMarkerIndex(lines,[/Проверка доступности Telegram/i,/Telegram availability/i,/Telegram reachability/i],0);
    markers[6]=dpiNativeMarkerIndex(lines,[/^\s*Fingerprint(?:\/|[\s|:]|$)/i,/Сибирская блокировка/i,/Fingerprint burst/i],0);
    const summary=dpiNativeMarkerIndex(lines,[/^\s*Итог\b/i,/^\s*Summary\b/i],0);
    return{lines,markers,summary};
  }
  function dpiNativeSectionFrom(report,test){
    const parsed=dpiNativeMarkers(report),start=parsed.markers[test];
    if(start==null||start<0)return'';
    let end=parsed.lines.length;
    for(const pos of [...parsed.markers,parsed.summary].filter(x=>Number.isInteger(x)&&x>start))end=Math.min(end,pos);
    return parsed.lines.slice(start,end).join('\n').trim();
  }
  function dpiNativePreamble(report){
    const parsed=dpiNativeMarkers(report),starts=parsed.markers.filter(x=>x>=0);
    const end=starts.length?Math.min(...starts):parsed.lines.length;
    return parsed.lines.slice(0,end).join('\n').trim();
  }
  function dpiNativeSummary(report){
    const parsed=dpiNativeMarkers(report);
    if(parsed.summary<0)return'';
    return parsed.lines.slice(parsed.summary).join('\n').trim();
  }
  function dpiRichTokenHTML(text){return dpiParityTokens(String(text||''))}
  function dpiRichRowKind(text){
    const value=String(text||'');
    if(/BLOCKED|DETECTED|TIMEOUT|TLS\s*(?:DROP|RST|EOF|ALERT)|SYN\s*DROP|TCP\s*RST|REFUSED|недоступен|заблокирован|перехватывает/i.test(value))return'bad';
    if(/Частично|WARNING|ВНИМАНИЕ|SLOW|STALL|MIXED|частич/i.test(value))return'warn';
    if(/\bOK\b|AVAILABLE|reachable|доступен|Найдено белых SNI/i.test(value))return'good';
    return'';
  }
  function dpiRichTableHTML(tableLines){
    const rows=tableLines.filter(line=>line.includes('│')).map(line=>{
      let cells=line.split('│');
      if(cells.length&&cells[0].trim()==='')cells=cells.slice(1);
      if(cells.length&&cells[cells.length-1].trim()==='')cells=cells.slice(0,-1);
      return cells.map(cell=>cell.trim());
    }).filter(row=>row.length&&row.some(Boolean));
    if(!rows.length)return'';
    const oneCol=rows.every(row=>row.length===1);
    if(oneCol){
      const cards=rows.map(row=>{
        const text=row[0],m=/^([^:]{1,34}):\s*(.*)$/.exec(text);
        if(m)return `<div class="dpi-rich-kv"><span>${dpiRichTokenHTML(m[1])}</span><strong>${dpiRichTokenHTML(m[2]||'—')}</strong></div>`;
        return `<div class="dpi-rich-line ${dpiRichRowKind(text)}">${dpiRichTokenHTML(text)}</div>`;
      }).join('');
      return `<div class="dpi-rich-kv-grid">${cards}</div>`;
    }
    const headerText=rows[0].join(' ');
    const headerLike=/Провайдер|Provider|Домен|Domain|DC|IP|Fingerprint|Профиль|Profile|ID|ASN|Статус|Status|HTTP|TLS|DoH|DoT|UDP|Регион|Region|Пинг|Ping|Детал/i.test(headerText);
    const head=headerLike?rows.shift():null;
    const width=Math.max(...rows.concat(head?[head]:[]).map(row=>row.length));
    const headHTML=head?`<thead><tr>${Array.from({length:width},(_,i)=>`<th>${dpiRichTokenHTML(head[i]||'')}</th>`).join('')}</tr></thead>`:'';
    const body=rows.map(row=>`<tr class="${dpiRichRowKind(row.join(' '))}">${Array.from({length:width},(_,i)=>`<td>${dpiRichTokenHTML(row[i]||'')}</td>`).join('')}</tr>`).join('');
    return `<div class="table-scroll dpi-table-scroll dpi-rich-table-scroll"><table>${headHTML}<tbody>${body}</tbody></table></div>`;
  }
  function dpiRichTextHTML(lines){
    return lines.filter(line=>line.trim()).map(line=>{
      const text=line.trim(),kind=dpiRichRowKind(text);
      const m=/^([^:]{2,36}):\s+(.+)$/.exec(text);
      if(m)return `<div class="dpi-rich-kv dpi-rich-kv-line ${kind}"><span>${dpiRichTokenHTML(m[1])}</span><strong>${dpiRichTokenHTML(m[2])}</strong></div>`;
      return `<div class="dpi-rich-line ${kind}">${dpiRichTokenHTML(text)}</div>`;
    }).join('');
  }
  function dpiRichSectionBody(section){
    const lines=String(section||'').replace(/\r\n?/g,'\n').split('\n');
    let html='',plain=[],table=[];
    const flushPlain=()=>{if(plain.length){html+=`<div class="dpi-rich-lines">${dpiRichTextHTML(plain)}</div>`;plain=[]}};
    const flushTable=()=>{if(table.length){html+=dpiRichTableHTML(table);table=[]}};
    const tableLine=line=>/[│┌┐└┘├┤┬┴┼═─]/.test(line);
    for(const line of lines){
      if(tableLine(line)){
        flushPlain();table.push(line);
        if(/[└┘╰╯]/.test(line)){flushTable()}
      }else{
        flushTable();plain.push(line);
      }
    }
    flushTable();flushPlain();
    return html||'<div class="analysis-empty">Upstream section empty.</div>';
  }
  function dpiResultSection(title,kicker,body,layout='wide'){
    return `<details class="panel dpi-result-section dpi-result-uniform" open><summary><span><strong>${esc(title)}</strong><small>${esc(kicker||'')}</small></span><span class="dpi-result-chevron">▾</span></summary><div class="dpi-result-body">${body}</div></details>`;
  }
  function dpiRenderRichNative(envelope){
    const report=String(envelope?.console_report||'');
    if(!report)return'';
    const selected=[...String(envelope.tests||selectedDPITests())].filter(x=>/[0-6]/.test(x));
    const version=dpiDetectorSnapshot?.version||'5.x';
    const profile=String($('dpiWebProfile')?.value||'—');
    const fp=String($('dpiWebFingerprint')?.value||'upstream/config default');
    let html=`<article class="panel dpi-result-summary"><div class="v2-stats"><div><span>Upstream</span><strong>${esc(version)}</strong></div><div><span>Source</span><strong>single native run</strong></div><div><span>Profile</span><strong>${esc(profile)}</strong></div><div><span>Fingerprint</span><strong>${esc(fp)}</strong></div><div><span>Duration</span><strong>${esc(envelope.duration_ms??0)} ms</strong></div></div>${envelope.environment_warning?`<div class="v2-boundary">${esc(envelope.environment_warning)}</div>`:''}</article>`;
    const preamble=dpiNativePreamble(report);
    if(preamble)html+=`<details class="panel dpi-rich-wide" open><summary><span><strong>Runtime / environment</strong><small>upstream preamble</small></span><span class="dpi-result-chevron">▾</span></summary><div class="dpi-result-body">${dpiRichSectionBody(preamble)}</div></details>`;
    for(const test of selected){
      const section=dpiNativeSectionFrom(report,Number(test));
      const name=dpiTestNames[test]||`Test ${test}`;
      html+=dpiResultSection(`${test} · ${name}`,'single-run · full upstream detail',section?dpiRichSectionBody(section):'<div class="analysis-empty">Секция не найдена в upstream report.</div>');
    }
    const summary=dpiNativeSummary(report);
    if(summary)html+=`<details class="panel dpi-rich-wide" open><summary><span><strong>Итог</strong><small>upstream summary</small></span><span class="dpi-result-chevron">▾</span></summary><div class="dpi-result-body">${dpiRichSectionBody(summary)}</div></details>`;
    html+=`<details class="panel dpi-technical"><summary>Технические детали · run envelope</summary><pre class="v2-args" style="margin-top:10px">${esc(JSON.stringify({...envelope,console_report:`<${report.length} chars>`},null,2))}</pre></details>`;
    return html;
  }
  function dpiMetric(label,value,kind='info'){return `<div class="card"><div class="metric-label">${esc(label)}</div><div class="metric-value ${kind==='good'?'good-text':kind==='bad'?'bad-text':kind==='warn'?'warn-text':''}">${esc(value??'—')}</div></div>`}
  function dpiRatioKind(ok,total){ok=Number(ok||0);total=Number(total||0);if(total<=0)return'info';if(ok===total)return'good';if(ok===0)return'bad';return'warn'}
  function dpiHumanKey(key){return String(key||'').replace(/_/g,' ').replace(/\b\w/g,c=>c.toUpperCase())}
  function dpiHumanValue(value){
    if(value===null||value===undefined||value==='')return'—';
    if(typeof value==='boolean')return value?'yes':'no';
    if(Array.isArray(value)){
      if(!value.length)return'—';
      return value.map(v=>{
        if(v===null||v===undefined)return'—';
        if(typeof v!=='object')return String(v);
        return Object.entries(v).slice(0,6).map(([k,x])=>`${dpiHumanKey(k)}: ${dpiHumanValue(x)}`).join(' · ');
      }).join(' | ');
    }
    if(typeof value==='object')return Object.entries(value).slice(0,8).map(([k,v])=>`${dpiHumanKey(k)}: ${dpiHumanValue(v)}`).join(' · ');
    return String(value);
  }
  function dpiLiveDetails(title,entries){
    const rows=(entries||[]).filter(x=>x&&x[1]!==undefined&&x[1]!==null&&x[1]!==''&&!(Array.isArray(x[1])&&x[1].length===0));
    if(!rows.length)return'';
    return `<div class="dpi-live-block"><div class="dpi-live-title">${esc(title)}</div><div class="dpi-detail-grid">${rows.map(([label,value])=>`<div class="dpi-detail-row"><span>${esc(label)}</span><span>${esc(dpiHumanValue(value))}</span></div>`).join('')}</div></div>`;
  }
  function dpiExtraDetails(title,data,skip=[]){
    if(!data||typeof data!=='object')return'';
    const deny=new Set(skip);
    return dpiLiveDetails(title,Object.entries(data).filter(([key])=>!deny.has(key)).map(([key,value])=>[dpiHumanKey(key),value]));
  }
  function dpiBytes(value){
    const n=Number(value||0);if(!Number.isFinite(n)||n<=0)return'0 B';
    if(n>=1024*1024)return`${(n/(1024*1024)).toFixed(2)} MiB`;
    if(n>=1024)return`${(n/1024).toFixed(1)} KiB`;
    return`${Math.round(n)} B`;
  }
  function dpiRate(value){
    const n=Number(value||0);if(!Number.isFinite(n)||n<=0)return'0 B/s';
    if(n>=1024*1024)return`${(n/(1024*1024)).toFixed(2)} MiB/s`;
    if(n>=1024)return`${(n/1024).toFixed(1)} KiB/s`;
    return`${Math.round(n)} B/s`;
  }

  function dpiRenderProgress(){
    const box=$('dpiRunProgress');if(!box)return;
    const p=dpiWebProgress;
    if(!p){box.style.display='none';box.innerHTML='';return}
    const failed=String(p.error||''),elapsed=Math.max(0,Math.floor((Date.now()-Number(p.started_at||Date.now()))/1000));
    const title=failed?'Проверка остановлена':dpiWebRunning?'Идёт один полный upstream run':'Проверка завершена';
    const detail=failed?failed:dpiWebRunning?`Выбраны тесты ${p.tests.join('')} · elapsed ${elapsed} s · повторного native-прогона не будет`:`Готово за ${elapsed} s · тот же report используется для Visual Web и Native view`;
    box.className=`dpi-run-progress ${failed?'bad':dpiWebRunning?'running single-run':'good'}`;
    box.style.display='grid';
    box.innerHTML=`<div class="dpi-run-progress-head"><strong>${esc(title)}</strong><span>${dpiWebRunning?`${elapsed}s`:'DONE'}</span></div><div class="dpi-run-progress-detail">${esc(detail)}</div><div class="dpi-run-progress-track"><span style="width:${dpiWebRunning?'36':'100'}%"></span></div>`;
  }
  function syncDPIConsoleFullscreen(){
    const button=$('dpiConsoleFullscreen'),pane=$('dpiConsolePane');if(!button||!pane)return;
    button.textContent=document.fullscreenElement===pane?'Выйти из полного экрана':'На весь экран';
  }
  async function toggleDPIConsoleFullscreen(){
    const pane=$('dpiConsolePane');if(!pane)return;
    try{
      if(document.fullscreenElement===pane){await document.exitFullscreen();return}
      if(!pane.requestFullscreen)throw new Error('Fullscreen API недоступен в этом браузере');
      await pane.requestFullscreen();
    }catch(e){notice(`DPI Console: ${e.message||e}`,'error')}
  }

  function dpiRenderNetwork(data){
    if(!data)return'';
    const cards=[];
    const endpoint=(x)=>x?.ip?`${x.ip} · ${x.latency_ms??'—'} ms`:'—';
    cards.push(dpiMetric('IPv4',endpoint(data.ipv4),data.ipv4?'good':'warn'));
    cards.push(dpiMetric('IPv6',endpoint(data.ipv6),data.ipv6?'good':'info'));
    cards.push(dpiMetric('ASN',data.v4_asn||'—'));
    cards.push(dpiMetric('Организация',data.v4_org||'—'));
    cards.push(dpiMetric('Страна',data.v4_cc||'—'));
    cards.push(dpiMetric('Gateway',data.gateway||'—'));
    cards.push(dpiMetric('Upstream',data.upstream||'—'));
    cards.push(dpiMetric('System DNS',(data.system_dns||[]).join(', ')||'—'));
    let extra='';
    if((data.tun||[]).length)extra+=`<div class="v2-boundary">TUN: ${esc(data.tun.join(', '))}</div>`;
    if((data.bypass_tools||[]).length)extra+=`<div class="v2-boundary">Bypass tools: ${esc(data.bypass_tools.join(', '))}</div>`;
    return dpiResultSection('0 · Network info','public path / resolver / bypass inventory',`<div class="v2-card-grid">${cards.join('')}</div>${extra}`,'compact');
  }
  function dpiRenderDNS(data){
    if(!data)return'';
    const subst=Number(data.subst_sub||0),substTotal=Number(data.subst_total||0);
    const fake=Number(data.fakeip_sub||0),fakeTotal=Number(data.fakeip_total||substTotal||0);
    const clean=Math.max(0,substTotal-subst);
    const cards=[
      dpiMetric('DoH',`${data.doh_ok??0} / ${data.doh_total??0}`,dpiRatioKind(data.doh_ok,data.doh_total)),
      dpiMetric('DoT',`${data.dot_ok??0} / ${data.dot_total??0}`,dpiRatioKind(data.dot_ok,data.dot_total)),
      dpiMetric('UDP DNS',`${data.udp_ok??0} / ${data.udp_total??0}`,dpiRatioKind(data.udp_ok,data.udp_total)),
      dpiMetric('Resolvers',data.resolvers_total??0),
      dpiMetric('Substitution',`${data.subst_sub??0} / ${data.subst_total??0}`,subst===0?'good':substTotal>0&&subst===substTotal?'bad':'warn')
    ];
    const integrity=dpiLiveDetails('Живые DNS-сигналы',[
      ['Подменено UDP',substTotal?`${subst} / ${substTotal}`:'—'],
      ['Без подмены',substTotal?`${clean} / ${substTotal}`:'—'],
      ['FakeIP ответы',fakeTotal?`${fake} / ${fakeTotal}`:'—'],
      ['Перехваченные бренды',(data.hijacked_brands||[]).length?data.hijacked_brands:'—']
    ]);
    const hij=(data.hijacked_brands||[]).length?`<div class="v2-boundary dpi-finding-bad">Hijacked / substituted brands: ${esc(data.hijacked_brands.join(', '))}</div>`:'';
    return dpiResultSection('1 · DNS availability','DoH / DoT / UDP and substitution signals',`<div class="v2-card-grid">${cards.join('')}</div>${integrity}${hij}`,'compact');
  }
  function dpiRenderDomains(rows){
    if(!Array.isArray(rows))return'';
    const body=rows.map(r=>`<tr><td><strong>${esc(r.domain||'—')}</strong></td><td class="mono">${esc(r.resolved||'—')}</td><td>${dpiChip(r.http)}<div class="small muted">${esc(dpiDetail(r.http_detail))}</div></td><td>${dpiChip(r.tls12)}<div class="small muted">${esc(dpiDetail(r.tls12_detail))}</div></td><td>${dpiChip(r.tls13)}<div class="small muted">${esc(dpiDetail(r.tls13_detail))}</div></td></tr>`);
    return dpiResultSection('2 · Domain inspection',`${rows.length} target(s)`,dpiTable(['Домен','IP','HTTP','TLS 1.2','TLS 1.3'],body));
  }
  function dpiRenderTCP16(rows){
    if(!Array.isArray(rows))return'';
    const body=rows.map(r=>`<tr><td class="mono">${esc(r.id||'—')}</td><td>${esc(r.asn||'—')}</td><td>${esc(r.provider||'—')}</td><td>${dpiChip(r.status)}</td><td>${esc(dpiDetail(r.detail))}</td></tr>`);
    return dpiResultSection('3 · TCP 16–20 KB',`${rows.length} target(s)`,dpiTable(['ID','ASN','Provider','Status','Detail'],body));
  }
  function dpiRenderWhitelist(data){
    if(!data)return'';
    const detected=Number(data.detected_as||0),found=Number(data.found_as||0);
    const missing=Math.max(0,detected-found);
    const coverage=detected>0?Math.round(found*100/detected):100;
    const details=dpiLiveDetails('Результат white-SNI',[
      ['Заблокированных AS',detected],
      ['SNI найден для AS',found],
      ['Без найденного SNI',missing],
      ['Покрытие',`${coverage}%`]
    ]);
    const extra=dpiExtraDetails('Дополнительные поля upstream',data,['detected_as','found_as']);
    const note=extra?'':`<div class="v2-boundary">Upstream JSON schema v1 отдаёт для test 4 только detected_as/found_as. Per-AS/SNI строки остаются в native Console; RouterForge не парсит текстовый TUI вместо JSON.</div>`;
    return dpiResultSection('4 · Whitelist SNI','AS-level white-SNI detection',`<div class="v2-card-grid">${dpiMetric('Detected AS',detected,detected?'warn':'good')}${dpiMetric('Found AS',found,found?'good':detected?'warn':'info')}</div>${details}${extra}${note}`,'compact');
  }
  function dpiTransfer(label,data){
    if(!data)return'';
    const avg=data.avg_bps??data.bps??0,peak=data.peak_bps??0;
    const bytes=data.bytes_total??data.downloaded??data.sent??data.bytes??0;
    const duration=data.duration??data.elapsed??0;
    return `<div class="card dpi-transfer-card"><div class="dpi-transfer-head"><strong>${esc(label)}</strong>${dpiChip(data.status)}</div><div class="dpi-transfer-metrics"><div><span>Средняя</span><strong>${esc(dpiRate(avg))}</strong></div><div><span>Пик</span><strong>${esc(dpiRate(peak))}</strong></div><div><span>Передано</span><strong>${esc(dpiBytes(bytes))}</strong></div><div><span>Время</span><strong>${esc(`${Number(duration||0).toFixed(1)} s`)}</strong></div>${data.drop_at_sec!=null?`<div><span>Обрыв</span><strong>${esc(`${data.drop_at_sec} s`)}</strong></div>`:''}</div></div>`;
  }
  function dpiRenderTelegram(data){
    if(!data)return'';
    const dc=Array.isArray(data.dc_results)?data.dc_results:[];
    const dcRows=dc.map(d=>`<tr><td>${esc(d.label||'—')}</td><td class="mono">${esc(d.ip||'—')}</td><td class="${d.reachable?'dpi-dc-ok':'dpi-dc-bad'}">${d.reachable?'reachable':'blocked'}</td><td>${d.rtt!=null?esc(`${(Number(d.rtt)*1000).toFixed(0)} ms`):'—'}</td></tr>`);
    const dcTable=dcRows.length?`<div class="dpi-live-block"><div class="dpi-live-title">Telegram DC reachability</div>${dpiTable(['DC','IP','Status','RTT'],dcRows).replace('dpi-table-scroll','dpi-table-scroll dpi-dc-table')}</div>`:`<div class="v2-boundary">Upstream не вернул per-DC записи для этого запуска.</div>`;
    const transfers=`<div class="dpi-transfer-grid">${dpiTransfer('Download',data.download)}${dpiTransfer('Upload',data.upload)}</div>`;
    const extra=dpiExtraDetails('Дополнительные поля Telegram',data,['verdict','download','upload','dc_results','dc_reachable','dc_total']);
    return dpiResultSection('5 · Telegram',`verdict ${data.verdict||'—'}`,`<div class="v2-card-grid">${dpiMetric('Verdict',data.verdict||'—',dpiStatusKind(data.verdict))}${dpiMetric('DC reachable',`${data.dc_reachable??0} / ${data.dc_total??0}`,dpiRatioKind(data.dc_reachable,data.dc_total))}</div>${dcTable}${transfers}${extra}`,'compact');
  }
  function dpiRenderBurst(data){
    if(!data)return'';
    const rows=[];
    for(const domain of data.domains||[]){
      for(const [profile,p] of Object.entries(domain.profiles||{})){
        rows.push(`<tr><td><strong>${esc(domain.domain||'—')}</strong><div class="small muted mono">${esc(domain.resolved||'—')}</div></td><td class="mono">${esc(profile)}</td><td>${esc(`${p.answered??0} / ${p.attempts??0}`)}</td><td>${(p.statuses||[]).map(dpiChip).join(' ')||'—'}</td><td>${esc(dpiDetail(p.detail))}</td></tr>`);
      }
    }
    const head=`<div class="v2-boundary">attempts ${esc(data.attempts)} · TLS ${esc(data.tls)} · ALPN ${esc(data.alpn)} · timeout ${esc(data.timeout_secs)}s · profiles ${esc((data.profiles||[]).join(', '))}</div>`;
    return dpiResultSection('6 · Fingerprint Burst','browser/TLS shape stress',head+dpiTable(['Domain','Profile','Answered','Statuses','Detail'],rows));
  }
  function renderDPIDetectorV5Result(envelope){
    const root=$('dpiWebResults');if(!root)return;
    if(!envelope){root.innerHTML=dpiWebRunning?'':'<article class="panel"><div class="analysis-empty">Тесты ещё не запускались.</div></article>';return}
    if(envelope.console_report){root.innerHTML=dpiRenderRichNative(envelope);return}
    if(envelope.ok===false){root.innerHTML=dpiResultSection('Ошибка','DPI Detector v5',`<div class="notice error">${esc(envelope.error||'run failed')}</div>`);return}
    root.innerHTML='<article class="panel"><div class="analysis-empty">Полный upstream report не получен.</div></article>';
  }

  function selectedDPITests(){let out='';for(let i=0;i<=6;i++)if($(`dpiTest${i}`)?.checked)out+=String(i);return out}
  function dpiDomainList(){
    return String($('dpiWebDomains').value||'').split(/[\n,;]+/).map(x=>x.trim()).filter(Boolean);
  }

  function renderDPIDetectorWorkspace(){
    if(!$('dpiModuleState'))return;
    const d=dpiDetectorSnapshot;
    $('dpiConsolePane').style.display=dpiWorkspaceMode==='console'?'block':'none';
    $('dpiWebPane').style.display=dpiWorkspaceMode==='web'?'block':'none';
    $('dpiModeConsole').classList.toggle('primary',dpiWorkspaceMode==='console');
    $('dpiModeWeb').classList.toggle('primary',dpiWorkspaceMode==='web');
    const state=!d?'IDLE':!d.installed?'NOT INSTALLED':d.execution_ready?'READY':'INCOMPATIBLE';
    $('dpiModuleState').textContent=state;$('dpiModuleState').className='config-badge '+(state==='READY'?'active':'');
    if($('dpiDetectorTabState'))$('dpiDetectorTabState').textContent=d?.installed?(d.execution_ready?'READY':'CHECK'):'';
    $('dpiModuleSummary').innerHTML=!d?'<span class="dpi-runtime-pill">Состояние не загружено</span>':d.installed?[
      `<span class="dpi-runtime-pill ${d.execution_ready?'good':'warn'}">${d.execution_ready?'● READY':'● CHECK'}</span>`,
      `<span class="dpi-runtime-pill">${esc(d.version||d.warning||'unknown')}</span>`,
      `<span class="dpi-runtime-pill mono">${esc(d.path||'unknown')} · ${esc(keeneticBytes(d.size_bytes||0))}</span>`
    ].join(''):'<span class="dpi-runtime-pill warn">DPI Detector v5 не установлен</span>';
    $('dpiWebRun').disabled=dpiWebRunning||!d?.installed||!d?.execution_ready;
    const elapsed=dpiWebProgress?.started_at?Math.max(0,Math.floor((Date.now()-dpiWebProgress.started_at)/1000)):0;
    $('dpiWebRun').textContent=dpiWebRunning?`DPI Detector · ${elapsed}s…`:'Запустить DPI Detector';
    $('dpiWebState').textContent=dpiWebRunning?`LIVE ${elapsed}s`:dpiWebProgress?.error?'FAIL':dpiNativeResult?.console_report?'DONE':'IDLE';
    $('dpiWebState').className='config-badge '+(!dpiWebRunning&&dpiWebResult&&!dpiWebProgress?.error?'active':'');
    dpiRenderProgress();
    renderDPIDetectorV5Result(dpiWebResult);
    updateDPINativeState();
    if(dpiWorkspaceMode==='console'&&d?.installed&&d?.execution_ready)ensureDPIConsole();
    syncDPIConsoleFullscreen();
  }

  async function runDPIDetectorWeb(){
    if(dpiWebRunning)return;
    const selected=[...selectedDPITests()];if(!selected.length){notice('DPI Detector: выберите хотя бы один тест.','error');return}
    dpiWebRunning=true;dpiWebResult=null;dpiNativeResult=null;dpiLiveText='';dpiLiveMeta=null;dpiNativeStartedAt=Date.now();
    dpiWebProgress={tests:selected,completed:[],current:'',error:'',started_at:Date.now(),single_run:true,streaming:true};
    renderDPIDetectorWorkspace();
    if(dpiNativeTimer)clearInterval(dpiNativeTimer);
    dpiNativeTimer=setInterval(()=>renderDPIDetectorWorkspace(),1000);
    try{
      const status=await api('/status');if(!status?.config_sha256)throw new Error('active config SHA недоступен');
      const bodyReq={
        tests:selected.join(''),domains:dpiDomainList(),domains_file:String($('dpiWebDomainsFile').value||'').trim(),tcp16_file:String($('dpiWebTCP16File').value||'').trim(),
        concurrency:Number($('dpiWebConcurrency').value||20),language:String($('dpiWebLanguage').value||'ru'),profile:String($('dpiWebProfile').value||'ru'),
        interface:String($('dpiWebInterface').value||'').trim(),proxy:String($('dpiWebProxy').value||'').trim(),fingerprint:String($('dpiWebFingerprint').value||'').trim(),
        burst:Number($('dpiWebBurst').value||5),burst_timeout:Number($('dpiWebBurstTimeout').value||8),burst_profiles:String($('dpiWebBurstProfiles').value||'').trim(),
        burst_tls:String($('dpiWebBurstTLS').value||'1.3+1.2'),burst_alpn:String($('dpiWebBurstALPN').value||'h2'),trace:false,
        expected_config_sha256:String(status.config_sha256),confirm:'ROUTERFORGE_DPI_DETECTOR_V5_RUN'
      };
      const response=await fetch(base+'/dpi-detector/v5/stream',{method:'POST',cache:'no-store',credentials:'same-origin',headers:{Accept:'application/x-ndjson','Content-Type':'application/json'},body:JSON.stringify(bodyReq)});
      if(!response.ok){
        const text=await response.text();let body={};try{body=text?JSON.parse(text):{}}catch{body={error:text||`HTTP ${response.status}`}}
        throw new Error(body.error||body.detail||`HTTP ${response.status}`);
      }
      if(!response.body)throw new Error('streaming body unavailable');
      const reader=response.body.getReader(),decoder=new TextDecoder();let pending='',streamError='';
      const handleEvent=event=>{
        if(!event||!event.type)return;
        if(event.type==='chunk'){dpiLiveText+=String(event.data||'');dpiScheduleLiveRender();return}
        if(event.type==='snapshot'){dpiLiveText=String(event.report||dpiLiveText);dpiLiveMeta=event;dpiScheduleLiveRender();return}
        if(event.type==='done'){
          dpiLiveMeta=event;dpiWebProgress.completed=[...selected];
          dpiNativeResult={ok:true,tests:event.tests||selected.join(''),duration_ms:Number(event.duration_ms||0),console_report:dpiLiveText,report_bytes:Number(event.report_bytes||0),report_cut:!!event.report_cut,exit_warning:event.exit_warning||'',nfqws2_running:!!event.nfqws2_running,environment_warning:event.environment_warning||'',production_config_sha256:event.production_config_sha256||'',production_config_unchanged:event.production_config_unchanged!==false};
          return;
        }
        if(event.type==='error'){streamError=[event.error,event.detail].filter(Boolean).join(' · ')||'DPI Detector stream failed'}
      };
      for(;;){
        const part=await reader.read();
        pending+=decoder.decode(part.value||new Uint8Array(),{stream:!part.done});
        let nl;
        while((nl=pending.indexOf('\n'))>=0){
          const line=pending.slice(0,nl).trim();pending=pending.slice(nl+1);
          if(line){try{handleEvent(JSON.parse(line))}catch(_){}}
        }
        if(part.done)break;
      }
      const tail=pending.trim();if(tail){try{handleEvent(JSON.parse(tail))}catch(_){}}
      if(streamError)throw new Error(streamError);
      if(!dpiNativeResult?.console_report&&dpiLiveText)dpiNativeResult={ok:true,tests:selected.join(''),duration_ms:Date.now()-dpiNativeStartedAt,console_report:dpiLiveText,report_bytes:new TextEncoder().encode(dpiLiveText).length,report_cut:false,production_config_unchanged:true};
      dpiWebResult=dpiNativeResult;
      notice('DPI Detector: Native Live завершён','good');
    }catch(e){
      const message=String(e.message||e);dpiWebProgress.error=message;dpiWebResult={ok:false,error:message};notice(`DPI Detector: ${message}`,'error');
    }finally{
      if(dpiNativeTimer){clearInterval(dpiNativeTimer);dpiNativeTimer=0}
      dpiWebRunning=false;renderDPIDetectorWorkspace();dpiScheduleLiveRender();
    }
  }
  async function loadDPILegend(){
    const pane=$('dpiLegendPane'),out=$('dpiLegendOutput');pane.style.display='block';out.textContent='Загрузка upstream legend…';
    try{const lang=String($('dpiWebLanguage').value||'ru');const d=await api(`/dpi-detector/v5/legend?lang=${encodeURIComponent(lang)}`);out.textContent=d.legend||'—'}
    catch(e){out.textContent=String(e.message||e)}
    reportHeight();
  }
  async function loadDPIDetector(force=false){
    if(dpiDetectorLoading||dpiDetectorLoaded&&!force){renderDPIDetector();return}
    dpiDetectorLoading=true;renderDPIDetector();
    try{dpiDetectorSnapshot=await api('/dpi-detector');dpiDetectorLoaded=true;renderDPIDetector()}
    catch(e){dpiDetectorSnapshot=null;notice(`DPI Detector integration: ${e.message}`,'error');renderDPIDetector()}
    finally{dpiDetectorLoading=false;renderDPIDetector()}
  }

  function normalizeProcessingMapDomain(raw){
    let value=String(raw||'').trim().toLowerCase().replace(/\.$/,'');
    if(value.includes('://')){try{value=new URL(value).hostname.toLowerCase().replace(/\.$/,'')}catch(_){return''}}
    if(value.length>253||!checkDomainName(value))return'';
    return value;
  }
  function checkDomainName(value){
    return /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(String(value||''));
  }
  async function loadProcessingMapLists(){
    const items=editableStatusLists(),out={},queue=[...items],errors=[];let done=0;
    $('processingMapMeta').textContent=`читаю списки: 0/${items.length}`;
    async function worker(){
      while(queue.length){
        const item=queue.shift();if(!item)break;
        try{const d=await api(`/list?name=${encodeURIComponent(item.name)}`);out[item.name]=d.content||''}
        catch(e){errors.push(`${item.name}: ${e.message||'read failed'}`)}
        done++;$('processingMapMeta').textContent=`читаю списки: ${done}/${items.length}`;
      }
    }
    await Promise.all(Array.from({length:Math.min(4,Math.max(1,items.length))},worker));
    return{contents:out,errors};
  }
  function processingListModel(staticPath){
    const hits=(staticPath?.hits||[]).filter(h=>h.file&&h.file!=='<query>'),activeInclude=hits.filter(h=>h.effective==='include'),activeExclude=hits.filter(h=>h.effective==='exclude'),unknown=hits.filter(h=>h.effective==='neutral');
    return{hits,found:hits.length,activeInclude,activeExclude,active:activeInclude.length+activeExclude.length,unknown};
  }
  function processingProbeModel(probe){
    if(!probe)return{kind:'info',label:'нет данных',reachable:false,contentOK:false,detail:'HTTP probe не выполнялся.'};
    const status=Number(probe.status_code||0),networkOK=!!probe.reachable;
    if(!networkOK)return{kind:'bad',label:'ошибка',reachable:false,contentOK:false,detail:probe.error||'Соединение с сайта с роутера не установлено.'};
    if(status>=200&&status<300)return{kind:'good',label:`HTTP ${status}`,reachable:true,contentOK:true,detail:'Сайт отвечает с роутера нормальным HTTP-ответом.'};
    if(status>=300&&status<400)return{kind:'good',label:`HTTP ${status}`,reachable:true,contentOK:true,detail:'Сайт достижим; сервер ответил перенаправлением.'};
    if(status>=400&&status<500)return{kind:'warn',label:`HTTP ${status}`,reachable:true,contentOK:false,detail:'Сеть и TLS до сайта работают, но сервер ограничил HTTP-запрос.'};
    if(status>=500)return{kind:'warn',label:`HTTP ${status}`,reachable:true,contentOK:false,detail:'Сайт достижим, но сервер вернул ошибку 5xx.'};
    return{kind:'good',label:status?`HTTP ${status}`:'доступен',reachable:true,contentOK:true,detail:'Сайт достижим с роутера.'};
  }
  function processingMapSummary(domain,staticPath,probe,keenetic){
    const reasons=[],running=!!current?.running,mode=staticPath.mode,lists=processingListModel(staticPath),inc=lists.activeInclude.length,exc=lists.activeExclude.length,probeModel=processingProbeModel(probe);
    let verdict='CHECK',kind='warn',detail='Недостаточно сигналов для однозначного вывода.';
    if(!running){verdict='NFQWS2 ОСТАНОВЛЕН';kind='bad';detail='nfqws2 не запущен. Списки и конфиг можно анализировать, но обход сейчас не работает.'}
    else if(inc&&exc){verdict='КОНФЛИКТ АКТИВНЫХ СПИСКОВ';kind='warn';detail=`Есть одновременно active include (${inc}) и exclude (${exc}) совпадения. Нужно проверить правила списков.`}
    else if(exc){verdict='ДОМЕН ИСКЛЮЧЁН';kind='warn';detail='Домен попадает в активный exclude path; nfqws-стратегия для него может не применяться.'}
    else if(lists.found&&!lists.active){verdict='СОВПАДЕНИЯ НАЙДЕНЫ · АКТИВНОСТЬ НЕ ПОДТВЕРЖДЕНА';kind='warn';detail=`Домен найден в ${lists.found} list-файл(ах), но active config не доказывает, что эти файлы выбирают текущую nfqws-стратегию.`}
    else if(!lists.found&&['MODE_LIST','MODE_AUTO'].includes(mode)){verdict='ДОМЕН НЕ НАЙДЕН В АКТИВНОМ ПУТИ';kind='warn';detail=`В ${mode} домен не найден в доступных list-файлах. Он может проходить без выбранной nfqws-стратегии.`}
    else if(probeModel.contentOK){verdict='САЙТ ДОСТУПЕН';kind='good';detail=probeModel.detail}
    else if(probeModel.reachable){verdict='СЕТЬ ДО САЙТА РАБОТАЕТ';kind='warn';detail=probeModel.detail}
    else if(probe){verdict='САЙТ С РОУТЕРА НЕДОСТУПЕН';kind='bad';detail=probeModel.detail}
    if(keenetic?.config_isp_interface&&keenetic?.default_ipv4_interface&&!keenetic.config_isp_matches_default){
      reasons.push(keeneticDiag('warn','WAN / interface',`ISP_INTERFACE=${keenetic.config_isp_interface} · default=${keenetic.default_ipv4_interface}.`));
    }else if(keenetic?.default_ipv4_interface){
      reasons.push(keeneticDiag('good','WAN / interface',`default IPv4=${keenetic.default_ipv4_interface}${keenetic.config_isp_interface?` · ISP_INTERFACE=${keenetic.config_isp_interface}`:''}`));
    }
    reasons.push(keeneticDiag(kind,verdict,detail));
    reasons.push(keeneticDiag(probeModel.kind,'HTTP с роутера',`${probeModel.label} · ${probeModel.detail}`));
    return{domain,verdict,kind,detail,reasons,running,mode,lists,probeModel};
  }
  function guidedActionButton(tab,label,extra=''){
    return `<button type="button" data-guided-tab="${esc(tab)}" ${extra}>${esc(label)}</button>`;
  }
  function guidedTroubleshooterModel(result){
    const running=!!result.summary.running,inc=result.staticPath.includes.length,exc=result.staticPath.excludes.length,mode=result.staticPath.mode||'UNKNOWN',probeOK=!!result.probe?.reachable;
    const ifaceMismatch=!!(result.keenetic?.config_isp_interface&&result.keenetic?.default_ipv4_interface&&!result.keenetic.config_isp_matches_default);
    let title='Нужна дополнительная проверка',detail=result.summary.detail||'RouterForge собрал доступные сигналы.',kind='warn',next='Откройте технические детали ниже или выполните дополнительную проверку.',actions=[guidedActionButton('preflight','Проверить конфигурацию')];
    if(!running){
      title='nfqws2 сейчас остановлен';detail='Без запущенного nfqws2 нельзя подтвердить работу обхода. Списки и конфиг ниже всё ещё можно анализировать.';kind='bad';next='Сначала проверьте состояние сервиса и только потом повторите диагностику.';actions=[guidedActionButton('overview','Открыть Обзор')];
    }else if(inc&&exc){
      title='Домен одновременно включён и исключён';detail=`Найдены include (${inc}) и exclude (${exc}) совпадения. По статическому анализу нельзя честно выбрать, какое правило реально определяет путь.`;kind='warn';next='Откройте списки и проверьте конфликтующие записи.';actions=[guidedActionButton('lists','Открыть списки')];
    }else if(exc){
      title='Домен попадает в исключение';detail='Активный exclude-path может объяснять, почему nfqws-стратегия к этому домену не применяется.';kind='warn';next='Проверьте запись в списках. RouterForge покажет конкретные совпадения ниже.';actions=[guidedActionButton('lists','Проверить списки')];
    }else if(!inc&&['MODE_LIST','MODE_AUTO'].includes(mode)){
      title='Домен не найден в активном списке';detail=`Текущий режим ${mode} ожидает include-match, но RouterForge его не нашёл.`;kind='warn';next='Добавьте или исправьте домен в нужном списке, затем повторите проверку.';actions=[guidedActionButton('lists','Открыть списки')];
    }else if(ifaceMismatch){
      title='Стоит проверить интернет-интерфейс';detail=`В конфиге указан ${result.keenetic.config_isp_interface}, а default IPv4 interface сейчас ${result.keenetic.default_ipv4_interface}. Это может быть нормой при policy routing/VLAN/PPP, поэтому RouterForge не считает это доказанной ошибкой.`;kind='warn';next='Сверьте интерфейс в разделе «Система», затем повторите диагностику.';actions=[guidedActionButton('keenetic','Открыть Систему')];
    }else if(probeOK){
      title='С роутера сайт доступен';detail='Router-side probe прошёл успешно. Если проблема остаётся на телефоне или ПК, её причина может находиться дальше по клиентскому пути или в фактическом попадании трафика в NFQUEUE.';kind='good';next='Повторите несколько проверок в Test Lab. Это поможет отделить единичный успех от стабильного результата.';actions=[guidedActionButton('prodlab-v2','Проверить текущую систему','data-guided-fill-prodlab="1"')];
    }else if(result.probe&&!probeOK&&inc){
      title='Домен выбран стратегией, но проверка не прошла';detail='Статический путь включает домен, однако HTTPS probe с роутера завершился ошибкой. Это сужает область поиска, но не доказывает конкретную причину.';kind='bad';next='Сначала проверьте конфигурацию, затем повторите домен в Test Lab.';actions=[guidedActionButton('preflight','Проверить конфигурацию'),guidedActionButton('prodlab-v2','Проверить текущую систему','data-guided-fill-prodlab="1"')];
    }else if(result.probe&&!probeOK){
      title='Роутер не смог открыть сайт';detail='Проверка доступности с самого роутера завершилась ошибкой, а статический путь не дал однозначного объяснения.';kind='bad';next='Проверьте конфигурацию и системный интернет-интерфейс.';actions=[guidedActionButton('preflight','Проверить конфигурацию'),guidedActionButton('keenetic','Открыть Систему')];
    }
    if(result.listErrors.length&&!actions.some(x=>x.includes('lists'))){actions.push(guidedActionButton('lists','Проверить списки'))}
    return{title,detail,kind,next,actions,running,inc,exc,mode,probeOK,ifaceMismatch};
  }
  function guidedTimelineModel(result,model){
    const rows=[],add=(kind,label,detail,action=null)=>rows.push({kind,label,detail,action});
    add(model.running?'good':'bad','Runtime',model.running?'nfqws2 запущен.':'nfqws2 остановлен.',{kind:'tab',tab:'overview',label:'Обзор'});

    const preflight=buildPreflight(),preBad=preflight.filter(x=>x.severity==='bad').length,preWarn=preflight.filter(x=>x.severity==='warn').length;
    add(preBad?'bad':preWarn?'warn':'good','Preflight',preBad?`${preBad} блокирующих проверок.`:preWarn?`${preWarn} пунктов требуют проверки.`:'Блокирующих проблем в текущем preflight не найдено.',{kind:'tab',tab:'preflight',label:'Открыть'});

    const hits=(result.staticPath?.hits||[]).filter(h=>h.file&&h.file!=='<query>');
    const firstHit=hits[0]||null,listAction=firstHit?{kind:'list',file:firstHit.file,line:firstHit.line||1,label:'К совпадению'}:{kind:'tab',tab:'lists',label:'Списки'};
    const foundMatches=Number(model.found||0),unknownMatches=Number(model.unknown||0);
    if(model.inc&&model.exc)add('warn','Списки',`Есть одновременно active include (${model.inc}) и exclude (${model.exc}) совпадения.`,listAction);
    else if(model.exc)add('warn','Списки',`Найдено active exclude совпадений: ${model.exc}.`,listAction);
    else if(model.inc)add('good','Списки',`Найдено active include совпадений: ${model.inc}.`,listAction);
    else if(foundMatches)add('warn','Списки',`Найдено совпадений: ${foundMatches}; активность ${unknownMatches} из них не подтверждена active config.`,listAction);
    else if(['MODE_LIST','MODE_AUTO'].includes(model.mode))add('warn','Списки',`В доступных list-файлах совпадений для ${model.mode} не найдено.`,{kind:'tab',tab:'lists',label:'Списки'});
    else add('info','Списки','Явный include-match для этого режима не обязателен.',{kind:'tab',tab:'lists',label:'Списки'});

    const staticVerdict=String(result.staticPath?.verdict||'UNKNOWN'),staticReason=String(result.staticPath?.reason||'Статический анализ завершён.');
    add(model.exc||model.inc?'good':'info','Static path',`${staticVerdict}: ${staticReason} Это вывод из config/list semantics, не live packet trace.`,{kind:'scroll',target:'processingMapSteps',label:'Детали'});

    if(model.ifaceMismatch)add('warn','WAN / interface',`ISP_INTERFACE=${result.keenetic?.config_isp_interface||'—'}, default IPv4=${result.keenetic?.default_ipv4_interface||'—'}. Несовпадение само по себе не доказывает ошибку.`,{kind:'tab',tab:'keenetic',label:'Система'});
    else if(result.keenetic?.default_ipv4_interface)add('good','WAN / interface',`default IPv4=${result.keenetic.default_ipv4_interface}${result.keenetic?.config_isp_interface?` · ISP_INTERFACE=${result.keenetic.config_isp_interface}`:''}.`,{kind:'tab',tab:'keenetic',label:'Система'});
    else add('info','WAN / interface','Keenetic interface evidence недоступно.',{kind:'tab',tab:'keenetic',label:'Система'});

    if(result.probe)add(model.probeOK?'good':'bad','Router probe',model.probeOK?`Router-side HTTPS доступен${result.probe.status_code?` · HTTP ${result.probe.status_code}`:''}.`:`Router-side HTTPS probe завершился ошибкой${result.probe.error?`: ${result.probe.error}`:'.'}`,{kind:'prodlab',label:'Проверка текущей системы'});
    else add('info','Router probe','Router-side probe evidence отсутствует.',{kind:'prodlab',label:'Проверка текущей системы'});

    const tail=String(current?.log_tail||''),domain=String(result.domain||'').toLowerCase(),domainLines=tail.split(/\r?\n/).filter(x=>domain&&x.toLowerCase().includes(domain));
    const debugEnabled=/^(1|true|yes)$/i.test(String(flowValues.LOG_LEVEL||''));
    if(domainLines.length)add('good','Logs',`В текущем log tail найдено строк с ${result.domain}: ${domainLines.length}. Это наблюдаемые события, но не полный packet trace.`,{kind:'logs',label:'Показать'});
    else if(debugEnabled&&tail.trim())add('info','Logs',`Log tail доступен, но строк с ${result.domain} не найдено. Отсутствие строки не доказывает отсутствие обработки.`,{kind:'logs',label:'Искать'});
    else if(debugEnabled)add('warn','Logs','Debug logging включён, но текущий tail пуст.',{kind:'tab',tab:'logs',label:'Логи'});
    else add('info','Logs','Debug logging выключен; live evidence из nfqws2 log сейчас ограничено.',{kind:'tab',tab:'logs',label:'Логи'});

    add('info','Граница наблюдаемости','RouterForge не наблюдал конкретный пакет LAN-клиента на входе и выходе NFQUEUE. Static path и router-side probe не являются доказательством такого прохождения.');
    return rows;
  }
  async function runGuidedTimelineAction(action,result){
    if(!action)return;
    if(action.kind==='tab'){activateNFQWSTab(action.tab);return}
    if(action.kind==='scroll'){document.getElementById(action.target)?.scrollIntoView({behavior:'smooth',block:'start'});return}
    if(action.kind==='list'){
      activateNFQWSTab('lists');
      await openListAtLine(action.file,Number(action.line)||1);
      return;
    }
    if(action.kind==='prodlab'){
      if(result.domain){$('v2LabSource').value='manual';$('v2LabTargets').value=result.domain}
      activateNFQWSTab('prodlab-v2');return;
    }
    if(action.kind==='logs'){
      $('logSearch').value=result.domain||'';
      logMode='all';activateNFQWSTab('logs');renderLog();return;
    }
  }
  function renderGuidedTimeline(result,model){
    const rows=guidedTimelineModel(result,model);
    $('guidedTimeline').innerHTML=rows.map((row,i)=>`<div class="guided-timeline-row ${esc(row.kind)}${row.action?' actionable':''}"><span class="guided-timeline-mark">${row.kind==='good'?'✓':row.kind==='bad'?'!':row.kind==='warn'?'!':'i'}</span><span class="guided-timeline-label">${esc(row.label)}</span><span class="guided-timeline-detail">${esc(row.detail)}</span>${row.action?`<button type="button" class="guided-timeline-action" data-guided-evidence="${i}">${esc(row.action.label)}</button>`:''}</div>`).join('');
    $('guidedTimeline').querySelectorAll('[data-guided-evidence]').forEach(button=>button.onclick=()=>{const row=rows[Number(button.dataset.guidedEvidence)];void runGuidedTimelineAction(row?.action,result)});
  }
  function guidedSupportReport(result,model){
    const evidence=guidedTimelineModel(result,model).map(row=>({state:row.kind,label:row.label,detail:row.detail}));
    return{
      generated_at:new Date().toISOString(),
      scope:'guided-troubleshooter-read-only',
      domain:String(result.domain||''),
      verdict:{state:model.kind,title:model.title,detail:model.detail},
      next_step:model.next,
      active_config_sha256:String(current?.config_sha256||''),
      runtime_running:!!model.running,
      mode:String(model.mode||'UNKNOWN'),
      static_path:{
        verdict:String(result.staticPath?.verdict||'UNKNOWN'),
        reason:String(result.staticPath?.reason||''),
        include_matches:Number(model.inc||0),
        exclude_matches:Number(model.exc||0)
      },
      router_probe:result.probe?{
        reachable:!!result.probe.reachable,
        status_code:Number(result.probe.status_code||0),
        error:String(result.probe.error||'')
      }:null,
      evidence,
      observability_boundary:'RouterForge does not directly observe the diagnosed LAN client packet entering and leaving NFQUEUE.'
    };
  }
  function guidedSupportReportText(report){
    const lines=[
      'RouterForge Guided Diagnostics',
      `Generated: ${report.generated_at}`,
      `Domain: ${report.domain||'—'}`,
      `Verdict: ${String(report.verdict.state||'').toUpperCase()} · ${report.verdict.title}`,
      `Detail: ${report.verdict.detail}`,
      `Next: ${report.next_step}`,
      `Active config SHA256: ${report.active_config_sha256||'—'}`,
      `Runtime: ${report.runtime_running?'RUNNING':'STOPPED'}`,
      `Mode: ${report.mode}`,
      `Static path: ${report.static_path.verdict}${report.static_path.reason?` · ${report.static_path.reason}`:''}`,
      `Matches: include=${report.static_path.include_matches} exclude=${report.static_path.exclude_matches}`,
      `Router probe: ${report.router_probe?(report.router_probe.reachable?`OK${report.router_probe.status_code?` HTTP ${report.router_probe.status_code}`:''}`:`FAIL${report.router_probe.error?` · ${report.router_probe.error}`:''}`):'NO DATA'}`,
      '',
      'Evidence:'
    ];
    for(const row of report.evidence)lines.push(`[${String(row.state||'info').toUpperCase()}] ${row.label}: ${row.detail}`);
    lines.push('',`Observability boundary: ${report.observability_boundary}`);
    return lines.join('\n');
  }
  function copyGuidedSupportReport(result,model){
    const report=guidedSupportReport(result,model),text=guidedSupportReportText(report);
    navigator.clipboard?.writeText(text);
    notice('Диагностический отчёт скопирован.','good');
  }
  function processingHumanModel(result){
    const lists=processingListModel(result.staticPath),probe=processingProbeModel(result.probe),running=!!result.summary.running,mode=result.staticPath.mode||'UNKNOWN';
    const inc=lists.activeInclude.length,exc=lists.activeExclude.length,found=lists.found,unknown=lists.unknown.length;
    const ifaceMismatch=!!(result.keenetic?.config_isp_interface&&result.keenetic?.default_ipv4_interface&&!result.keenetic.config_isp_matches_default);
    let title='Нужна дополнительная проверка',detail='RouterForge собрал доступные сигналы.',kind='warn',next='Откройте технические детали или запустите live NFQWS-проверку.',actions=[guidedActionButton('preflight','Проверить конфигурацию')];
    if(!running){title='nfqws2 сейчас остановлен';detail='Сервис не работает. Статический анализ ниже остаётся справочным.';kind='bad';next='Сначала запустите nfqws2 и повторите диагностику.';actions=[guidedActionButton('overview','Открыть Обзор')]}
    else if(inc&&exc){title='Конфликт активных списков';detail=`Для ${result.domain} одновременно найдены active include (${inc}) и exclude (${exc}) совпадения.`;kind='warn';next='Проверьте конфликтующие записи в списках.';actions=[guidedActionButton('lists','Открыть списки')]}
    else if(exc){title='Домен исключён из обработки';detail='Active exclude-match может объяснять, почему nfqws-стратегия к этому домену не применяется.';kind='warn';next='Проверьте запись в exclude-списке.';actions=[guidedActionButton('lists','Открыть списки')]}
    else if(inc){title=probe.contentOK?'Домен выбран, сайт доступен':'Домен выбран активным списком';detail=`Найдено active include совпадений: ${inc}. ${probe.detail}`;kind=probe.kind==='bad'?'bad':'good';next='Если нужно доказать прохождение именно через тестовую NFQUEUE, запустите «Проверить через NFQWS».';actions=[guidedActionButton('lists','Показать совпадения')]}
    else if(found){title='Домен найден в списках, но их активность не подтверждена';detail=`Найдено ${found} совпадений, из них ${unknown} RouterForge не может связать с active strategy по текущему config. ${probe.detail}`;kind='warn';next='Это не означает «домен не найден». Для фактической проверки стратегии запустите live NFQWS-тест.';actions=[guidedActionButton('lists','Показать списки')]}
    else if(['MODE_LIST','MODE_AUTO'].includes(mode)){title='Домен не найден в доступных списках';detail=`Текущий ${mode} ожидает list-match, но RouterForge не нашёл его. ${probe.detail}`;kind=probe.kind==='bad'?'bad':'warn';next=probe.contentOK?'Сайт работает напрямую. Если обход ему не нужен, ничего менять не требуется.':'Если домен должен идти через nfqws, добавьте его в нужный active список.';actions=[guidedActionButton('lists','Открыть списки')]}
    else if(probe.contentOK){title='Сайт доступен с роутера';detail=probe.detail;kind='good';next='Если у LAN-клиента сайт всё равно не работает, запустите live NFQWS-тест или проверьте клиентский маршрут.';actions=[guidedActionButton('prodlab-v2','Проверить текущую систему','data-guided-fill-prodlab="1"')]}
    else if(probe.reachable){title='Сеть до сайта работает, HTTP ограничен';detail=probe.detail;kind='warn';next='HTTP 4xx/5xx не равен сетевому отказу. При необходимости запустите live NFQWS-тест.';actions=[guidedActionButton('prodlab-v2','Проверить текущую систему','data-guided-fill-prodlab="1"')]}
    else if(result.probe){title='Сайт с роутера недоступен';detail=probe.detail;kind='bad';next='Проверьте WAN/config, затем повторите тест.';actions=[guidedActionButton('preflight','Проверить конфигурацию'),guidedActionButton('keenetic','Открыть Систему')]}
    return{title,detail,kind,next,actions,running,inc,exc,found,unknown,mode,probeOK:probe.contentOK,probe,ifaceMismatch};
  }
  function renderGuidedTroubleshooter(result){
    const model=processingHumanModel(result),root=$('guidedTroubleshooter');
    $('guidedTitle').textContent=model.title;$('guidedDetail').textContent=model.detail;
    $('guidedBadge').textContent=model.kind==='good'?'OK':model.kind==='bad'?'ПРОБЛЕМА':'ПРОВЕРИТЬ';$('guidedBadge').className=`guided-result-badge ${model.kind}`;
    $('guidedRuntime').textContent=model.running?'Работает':'Остановлен';
    $('guidedLists').textContent=model.found?`${model.found} найдено · ${model.inc+model.exc} активных${model.unknown?` · ${model.unknown} не подтверждены`:''}`:'не найдено';
    $('guidedInterface').textContent=model.ifaceMismatch?'нужно проверить':result.keenetic?.default_ipv4_interface||'нет данных';
    $('guidedProbe').textContent=model.probe.label;
    $('guidedProbe').className=model.probe.kind==='good'?'processing-live-good':model.probe.kind==='bad'?'processing-live-bad':'processing-live-warn';
    $('processingLiveCheck').disabled=processingMapLiveRunning||!result.domain||!model.running;
    $('processingLiveCheck').classList.toggle('running',processingMapLiveRunning);
    renderGuidedTimeline(result,model);
    $('copyGuidedReport').disabled=false;$('copyGuidedReport').onclick=()=>copyGuidedSupportReport(result,model);
    if((model.kind==='bad'||model.kind==='warn')&&!model.actions.some(x=>x.includes('data-guided-tab="logs"')))model.actions.push(guidedActionButton('logs','Посмотреть логи'));
    $('guidedNextText').textContent=model.next;$('guidedActions').innerHTML=model.actions.join('');
    $('guidedActions').querySelectorAll('[data-guided-tab]').forEach(button=>button.onclick=()=>{
      const tab=button.dataset.guidedTab;
      if(button.dataset.guidedFillProdlab==='1'&&result.domain){$('v2LabSource').value='manual';$('v2LabTargets').value=result.domain}
      activateNFQWSTab(tab);
    });
    root.classList.add('visible');
  }

  function processingLiveModel(){
    const d=processingMapLiveResult;
    if(processingMapLiveRunning&&!d)return{kind:'info',label:'идёт проверка',detail:'Fast-тест выполняется в тестовой NFQUEUE.',proven:false};
    if(!d)return{kind:'info',label:'не запускался',detail:'Live-проверка NFQWS ещё не выполнялась.',proven:false};
    if(d.error)return{kind:'bad',label:'ОШИБКА',detail:d.error,proven:false};
    const samples=(d.candidates||[]).flatMap(c=>(c.attempts||[]).map(a=>({profile:c.source_profile_index,attempt:a})));
    const successful=samples.find(x=>x.attempt.ok&&Number(x.attempt.outbound_queue_packets||0)>0&&Number(x.attempt.inbound_queue_packets||0)>0);
    const packets=samples.reduce((s,x)=>s+Number(x.attempt.outbound_queue_packets||0)+Number(x.attempt.inbound_queue_packets||0),0);
    const cleanup=!!d.cleanup_baseline_after,proven=!!successful;
    if(proven&&cleanup){
      const queue=Number(successful.attempt.queue||0),profile=Number(successful.profile);
      return{kind:'good',label:`ПРОЙДЕН · ${packets} пакетов`,detail:`Профиль ${Number.isInteger(profile)?profile:'—'} · NFQUEUE ${queue||'—'} · TLS-путь через candidate доказан · очистка PASS.`,proven:true};
    }
    return{kind:'warn',label:d.ok?'ТРЕБУЕТ ПРОВЕРКИ':'ОШИБКА',detail:d.recommendation_reason||'Live-путь через candidate NFQUEUE не удалось доказать полностью.',proven:false};
  }
  function processingProbeSocket(errorText){
    const raw=String(errorText||''),m=raw.match(/(?:read|dial) tcp\s+([0-9.]+):(\d+)->([0-9.]+):(\d+)/i);
    return m?{source:`${m[1]}:${m[2]}`,destination:`${m[3]}:${m[4]}`}:{source:'процесс RouterForge на роутере',destination:''};
  }
  function processingRouteNode(label,value,tone=''){
    return `<div class="processing-route-node ${tone}"><span>${esc(label)}</span><strong>${esc(value||'—')}</strong></div>`;
  }
  function processingRouteArrow(){return '<div class="processing-route-arrow"></div>'}
  function processingStrategyTechnique(profile){
    if(!profile)return'';
    const args=Array.isArray(profile.args)?profile.args:[];
    const desync=args.filter(a=>String(a).startsWith('--lua-desync='));
    if(desync.length)return desync.join(' ');
    const relevant=args.filter(a=>/^--(?:dpi-desync|filter-l7|filter-tcp|filter-udp|payload)=/.test(String(a)));
    return relevant.join(' ');
  }
  function processingStrategyShort(profile){
    if(!profile)return'профиль недоступен';
    const parts=[];
    if(Array.isArray(profile.strategy_tags)&&profile.strategy_tags.length)parts.push(`strategy tags: ${profile.strategy_tags.join(', ')}`);
    if(Array.isArray(profile.tcp_filters)&&profile.tcp_filters.length)parts.push(`TCP ${profile.tcp_filters.join(',')}`);
    if(Array.isArray(profile.l7_filters)&&profile.l7_filters.length)parts.push(`L7 ${profile.l7_filters.join(',')}`);
    if(profile.desync_count)parts.push(`desync ×${profile.desync_count}`);
    return parts.join(' · ')||'eligible strategy profile';
  }
  function processingAttemptResult(candidate){
    const attempts=Array.isArray(candidate?.attempts)?candidate.attempts:[];
    const a=attempts[0]||null;
    if(!a)return{tone:'',label:'ОЖИДАЕТ',meta:'ещё не запускалась'};
    const out=Number(a.outbound_queue_packets||0),inc=Number(a.inbound_queue_packets||0),packets=out+inc;
    if(a.ok)return{tone:'good',label:'PASS',meta:`q${a.queue||'—'} · ${packets} pkt · ${Number(a.probe_duration_ms||0)} ms`};
    return{tone:'bad',label:'FAIL',meta:`q${a.queue||'—'} · ${packets} pkt${a.error?` · ${a.error}`:''}`};
  }
  function renderProcessingStrategyPlan(){
    const inv=processingMapStrategyInventory,live=processingMapLiveResult;
    const eligible=(inv?.profiles||[]).filter(p=>p.candidate_eligible).slice(0,3);
    const tested=new Map((live?.candidates||[]).map(c=>[Number(c.source_profile_index),c]));
    const baselineResult=live?.baseline?processingAttemptResult(live.baseline):{tone:'',label:'КОНТРОЛЬ',meta:'без desync'};
    const rows=[
      `<div class="processing-strategy-row baseline ${baselineResult.tone}"><div class="processing-strategy-id">BASELINE</div><div class="processing-strategy-main"><div class="processing-strategy-title">Контрольный TLS без стратегии обхода</div><div class="processing-strategy-summary">Только тестовая NFQUEUE и временный candidate без desync. Показывает, работает ли цель без стратегии.</div></div><div class="processing-strategy-result"><b>${esc(baselineResult.label)}</b><span>${esc(baselineResult.meta)}</span></div></div>`,
      ...eligible.map(p=>{
        const result=tested.has(Number(p.index))?processingAttemptResult(tested.get(Number(p.index))):{tone:'',label:'ОЖИДАЕТ',meta:'будет проверена'};
        const technique=processingStrategyTechnique(p);
        return `<div class="processing-strategy-row ${result.tone}"><div class="processing-strategy-id">PROFILE ${Number(p.index)}</div><div class="processing-strategy-main"><div class="processing-strategy-title">${esc(processingStrategyShort(p))}</div><div class="processing-strategy-summary">Домен для теста: ${esc(processingMapResult?.domain||'—')} · профиль взят из текущего production runtime</div>${technique?`<div class="processing-strategy-args">${esc(technique)}</div>`:''}</div><div class="processing-strategy-result"><b>${esc(result.label)}</b><span>${esc(result.meta)}</span></div></div>`;
      })
    ];
    $('processingStrategyPlan').innerHTML=rows.join('');
    const mode=live?.mode;
    $('processingStrategyMode').textContent=mode?.name?`${mode.name.toUpperCase()} · ${Number(mode.attempts||1)} попытка · максимум ${Number(mode.max_candidates||3)} профиля`:`Fast · baseline + ${eligible.length||'до 3'} профиля`;
  }
  function renderProcessingNetworkRoutes(result){
    const probe=processingProbeModel(result.probe),sock=processingProbeSocket(result.probe?.error),iface=result.keenetic?.default_ipv4_interface||result.keenetic?.config_isp_interface||'системный маршрут';
    const directTone=probe.kind==='good'?'good':probe.kind==='bad'?'bad':'warn';
    $('processingDirectBadge').textContent=probe.label.toUpperCase();$('processingDirectBadge').className=`processing-route-badge ${directTone}`;
    $('processingDirectRoute').innerHTML=[
      processingRouteNode('Источник',sock.source),
      processingRouteArrow(),
      processingRouteNode('Маршрут',iface),
      processingRouteArrow(),
      processingRouteNode('Production q300','не форсируется / не измеряется'),
      processingRouteArrow(),
      processingRouteNode('Назначение',sock.destination||`https://${result.domain}:443`),
      processingRouteArrow(),
      processingRouteNode('Результат',probe.detail,directTone)
    ].join('');

    const d=processingMapLiveResult,live=processingLiveModel();
    const badgeTone=live.kind==='good'?'good':live.kind==='bad'?'bad':'warn';
    $('processingLiveBadge').textContent=live.label.toUpperCase();$('processingLiveBadge').className=`processing-route-badge ${badgeTone}`;
    if(!d){
      $('processingLiveRoute').innerHTML=[
        processingRouteNode('Источник','тестовый socket RouterForge'),
        processingRouteArrow(),
        processingRouteNode('Отбор трафика','точный dst IPv4:443 + локальный source port'),
        processingRouteArrow(),
        processingRouteNode('NFQUEUE','30000–30127 · будет выбрана свободная'),
        processingRouteArrow(),
        processingRouteNode('Candidate','временный nfqws2 · Fast mode'),
        processingRouteArrow(),
        processingRouteNode('Результат','ещё не запускался')
      ].join('');
      $('processingCleanupProof').textContent='не запускался';
      renderProcessingStrategyPlan();
      return;
    }
    if(d.error){
      $('processingLiveRoute').innerHTML=[
        processingRouteNode('Источник','тестовый socket RouterForge'),
        processingRouteArrow(),
        processingRouteNode('Live NFQWS','не завершён','bad'),
        processingRouteArrow(),
        processingRouteNode('Ошибка',d.error,'bad')
      ].join('');
      $('processingCleanupProof').textContent='не доказана';renderProcessingStrategyPlan();return;
    }
    const samples=[d.baseline,...(d.candidates||[])].flatMap(c=>(c?.attempts||[]).map(a=>({profile:c.source_profile_index,baseline:!!c.baseline,attempt:a})));
    const successful=samples.find(x=>x.attempt.ok&&Number(x.attempt.outbound_queue_packets||0)>0&&Number(x.attempt.inbound_queue_packets||0)>0);
    const representative=successful||samples.find(x=>x.attempt)||null;
    const a=representative?.attempt||{},profile=representative?.baseline?'baseline':Number.isInteger(Number(representative?.profile))?String(representative.profile):'—';
    const out=Number(a.outbound_queue_packets||0),incoming=Number(a.inbound_queue_packets||0);
    $('processingLiveRoute').innerHTML=[
      processingRouteNode('Источник',a.local_port?`RouterForge :${a.local_port}`:'тестовый socket RouterForge'),
      processingRouteArrow(),
      processingRouteNode('Отбор трафика',`dst ${a.destination_ipv4||d.destination_ipv4||result.domain}:443 + exact source port`),
      processingRouteArrow(),
      processingRouteNode('NFQUEUE',a.queue?String(a.queue):'тестовая 30000–30127',successful?'good':'warn'),
      processingRouteArrow(),
      processingRouteNode('Candidate',`временный nfqws2 · профиль ${profile}`),
      processingRouteArrow(),
      processingRouteNode('Пакеты',`${out} исходящих · ${incoming} входящих`,successful?'good':'warn'),
      processingRouteArrow(),
      processingRouteNode('TLS / результат',successful?'путь через candidate доказан':(a.error||d.recommendation_reason||'успешный путь не доказан'),successful?'good':'warn')
    ].join('');
    $('processingCleanupProof').textContent=d.cleanup_baseline_after?'PASS · тестовые правила и процесс убраны':'НЕ ДОКАЗАНА';
    $('processingCleanupProof').style.color=d.cleanup_baseline_after?'var(--good)':'var(--bad)';
    renderProcessingStrategyPlan();
  }
  function renderProcessingMap(result){
    processingMapResult=result;
    const lists=processingListModel(result.staticPath),probe=processingProbeModel(result.probe),live=processingLiveModel();
    $('processingMapMode').textContent=result.staticPath.mode||'UNKNOWN';
    $('processingMapStatic').textContent=lists.found?`${lists.found} найдено · ${lists.active} активных`:'не найдено';
    $('processingMapProbe').textContent=probe.label;
    $('processingMapRuntime').textContent=result.summary.running?'РАБОТАЕТ':'ОСТАНОВЛЕН';
    $('processingMapVerdict').innerHTML=result.summary.reasons.join('');
    $('processingLiveState').textContent=live.label;
    $('processingLiveState').className=live.kind==='good'?'processing-live-good':live.kind==='bad'?'processing-live-bad':live.kind==='warn'?'processing-live-warn':'';
    $('processingMapHints').innerHTML=[
      keeneticDiag(lists.found?'good':'info','Найдено в list-файлах',String(lists.found)),
      keeneticDiag(lists.active?'good':'info','Подтверждено текущим конфигом',String(lists.active)),
      lists.unknown.length?keeneticDiag('warn','Роль не подтверждена',String(lists.unknown.length)):keeneticDiag('good','Неподтверждённых совпадений','0'),
      result.listErrors.length?keeneticDiag('warn','Ошибки чтения списков',result.listErrors.join(' · ')):keeneticDiag('good','Списки прочитаны','Все доступные list-файлы прочитаны')
    ].join('');
    const hits=lists.hits;
    const hitRows=hits.map((h,i)=>{
      const unknown=h.effective==='neutral',role=unknown?'РОЛЬ НЕ ПОДТВЕРЖДЕНА':h.effective==='include'?'АКТИВНОЕ INCLUDE':'АКТИВНОЕ EXCLUDE',tone=unknown?'neutral':h.effective;
      return `<div class="static-path-step ${tone}"><div class="static-path-index">${i+3}</div><div><div class="static-path-role ${tone}">${role}</div><div class="inspector-meta">${esc(h.relation)}</div></div><div><div class="static-path-value">${esc(h.file)}</div><div class="static-path-meta">${unknown?'Совпадение найдено, но текущий конфиг не подтвердил роль этого списка.':esc(h.role.reason)} · ${esc(h.value)}${h.detail?` · ${esc(h.detail)}`:''}</div></div><button class="static-path-open" data-processing-file="${esc(h.file)}" data-processing-line="${h.line}">Открыть</button></div>`;
    });
    const listState=lists.active?'активный путь подтверждён':lists.found?'совпадения найдены · роль не подтверждена':'совпадений в списках нет';
    const liveTone=live.kind==='good'?'include':live.kind==='bad'?'exclude':'neutral';
    $('processingMapSteps').innerHTML=[
      `<div class="static-path-step neutral"><div class="static-path-index">1</div><div><div class="static-path-role neutral">ДОМЕН</div><div class="inspector-meta">ВХОД</div></div><div><div class="static-path-value">${esc(result.domain)}</div><div class="static-path-meta">Запрос карты обработки</div></div></div>`,
      `<div class="static-path-step ${lists.active?'include':'neutral'}"><div class="static-path-index">2</div><div><div class="static-path-role ${lists.active?'include':'neutral'}">СПИСКИ</div><div class="inspector-meta">${esc(result.staticPath.mode||'UNKNOWN')}</div></div><div><div class="static-path-value">${lists.found} найдено · ${lists.active} активных · ${lists.unknown.length} не подтверждены</div><div class="static-path-meta">${esc(listState)}</div></div></div>`,
      ...hitRows,
      `<div class="static-path-step ${liveTone}"><div class="static-path-index">${hits.length+3}</div><div><div class="static-path-role ${liveTone}">LIVE NFQWS</div><div class="inspector-meta">ТЕСТОВАЯ NFQUEUE</div></div><div><div class="static-path-value">${esc(live.label)}</div><div class="static-path-meta">${esc(live.detail)}</div></div></div>`,
      `<div class="static-path-step ${probe.kind==='good'?'include':probe.kind==='bad'?'exclude':'neutral'}"><div class="static-path-index">${hits.length+4}</div><div><div class="static-path-role ${probe.kind==='good'?'include':probe.kind==='bad'?'exclude':'neutral'}">HTTP</div><div class="inspector-meta">С РОУТЕРА</div></div><div><div class="static-path-value">${esc(probe.label)}</div><div class="static-path-meta">${esc(probe.detail)}</div></div></div>`
    ].join('');
    $('processingMapSteps').querySelectorAll('[data-processing-file]').forEach(button=>button.onclick=()=>{document.querySelector('.tab[data-tab="lists"]')?.click();openListAtLine(button.dataset.processingFile,Number(button.dataset.processingLine)||1)});
    $('processingMapMeta').textContent=`${result.domain} · ${Object.keys(result.lists).length} list-файлов · текущий конфиг`;
    renderGuidedTroubleshooter(result);
    renderProcessingNetworkRoutes(result);
    $('copyProcessingMap').disabled=false;reportHeight();
  }
  async function runProcessingMap(){
    if(processingMapRunning)return;
    const domain=normalizeProcessingMapDomain($('processingMapDomain').value);
    if(!domain){notice('Processing Map: нужен корректный публичный DNS hostname.','error');return}
    processingMapRunning=true;processingMapResult=null;processingMapLiveResult=null;processingMapStrategyInventory=null;$('copyProcessingMap').disabled=true;$('runProcessingMap').disabled=true;$('copyGuidedReport').disabled=true;$('processingLiveCheck').disabled=true;$('processingLiveState').textContent='не запускался';
    $('guidedTroubleshooter').classList.remove('visible');$('guidedActions').innerHTML='';$('guidedTimeline').innerHTML='';
    $('processingMapState').textContent='ПРОВЕРКА';$('processingMapState').className='testlab-state running';
    $('processingMapVerdict').innerHTML='<div class="analysis-empty">Проверяю runtime, списки, путь и доступность с роутера…</div>';
    try{
      const loaded=await loadProcessingMapLists(),record={file:'<query>',line:0,value:domain,domain,type:'domain'},staticPath=buildStaticPath(record,loaded.contents);
      let probe=null,keenetic=null;
      try{probe=await api('/check',{method:'POST',body:JSON.stringify({url:`https://${domain}/`})})}catch(e){probe={reachable:false,error:e.message||'probe failed'}}
      try{keenetic=await api('/keenetic')}catch(_){}
      try{processingMapStrategyInventory=await api('/bench-strategies')}catch(_){processingMapStrategyInventory=null}
      const summary=processingMapSummary(domain,staticPath,probe,keenetic);
      renderProcessingMap({domain,staticPath,probe,keenetic,summary,lists:loaded.contents,listErrors:loaded.errors});
      $('processingMapState').textContent='ГОТОВО';$('processingMapState').className='testlab-state done';
    }catch(e){
      $('processingMapState').textContent='ОШИБКА';$('processingMapState').className='testlab-state fail';
      $('processingMapVerdict').innerHTML=keeneticDiag('bad','Processing Map failed',e.message||String(e));
    }finally{processingMapRunning=false;$('runProcessingMap').disabled=false;reportHeight()}
  }
  async function runProcessingMapLiveProof(){
    if(processingMapLiveRunning||!processingMapResult?.domain)return;
    processingMapLiveRunning=true;processingMapLiveResult=null;$('processingLiveCheck').disabled=true;$('processingLiveCheck').classList.add('running');$('processingLiveState').textContent='ПРОВЕРКА';
    try{
      const latest=await api('/status');
      if(!latest?.running||!latest?.config_sha256)throw new Error('nfqws2 runtime не готов или active config SHA недоступен.');
      try{processingMapStrategyInventory=await api('/bench-strategies')}catch(_){}
      renderProcessingStrategyPlan();
      const d=await api('/bench-autotune',{method:'POST',body:JSON.stringify({mode:'fast',server_name:processingMapResult.domain,expected_config_sha256:String(latest.config_sha256).toLowerCase(),confirm:'ROUTERFORGE_AUTOTUNE_TLS'})});
      if(d.ok!==true)throw new Error('Fast-тест NFQWS не подтвердил выполнение.');
      if(d.cleanup_baseline_after!==true)throw new Error('Очистка тестовой NFQUEUE не доказана.');
      processingMapLiveResult=d;
      const live=processingLiveModel();
      notice(live.proven?'Live NFQWS: временный candidate и тестовая NFQUEUE доказаны пакетами; q300 и рабочий конфиг не изменялись.':'Live NFQWS завершён: cleanup выполнен, но успешный TLS-путь через candidate NFQUEUE не доказан.',live.proven?'good':'error');
    }catch(e){
      processingMapLiveResult={error:e.message||String(e)};
      notice(`Live NFQWS FAIL: ${e.message||e}`,'error');
    }finally{
      processingMapLiveRunning=false;$('processingLiveCheck').classList.remove('running');
      if(processingMapResult)renderProcessingMap(processingMapResult);
    }
  }
  function copyProcessingMap(){
    if(!processingMapResult)return;
    const safe={domain:processingMapResult.domain,static_path:{mode:processingMapResult.staticPath.mode,verdict:processingMapResult.staticPath.verdict,reason:processingMapResult.staticPath.reason,includes:processingMapResult.staticPath.includes,excludes:processingMapResult.staticPath.excludes,hits:processingMapResult.staticPath.hits,list_summary:processingListModel(processingMapResult.staticPath)},probe:processingMapResult.probe,strategy_plan:(processingMapStrategyInventory?.profiles||[]).filter(p=>p.candidate_eligible).slice(0,3).map(p=>({index:p.index,strategy_tags:p.strategy_tags,args:p.args})),live_nfqws:processingMapLiveResult?{ok:processingMapLiveResult.ok||false,mode:processingMapLiveResult.mode||null,baseline:processingMapLiveResult.baseline||null,cleanup_baseline_after:processingMapLiveResult.cleanup_baseline_after||false,recommendation_available:processingMapLiveResult.recommendation_available||false,recommendation_reason:processingMapLiveResult.recommendation_reason||'',candidates:processingMapLiveResult.candidates||[],error:processingMapLiveResult.error||''}:null,keenetic:processingMapResult.keenetic,summary:{verdict:processingMapResult.summary.verdict,detail:processingMapResult.summary.detail,runtime_running:processingMapResult.summary.running},list_errors:processingMapResult.listErrors};
    navigator.clipboard?.writeText(JSON.stringify(safe,null,2));notice('Processing Map JSON скопирован.','good');
  }

  function strategyImportDetectType(content){
    const s=String(content||''),lines=s.split(/\r?\n/);
    if(lines.some(x=>/^Config:\s+.+?\s+\(Type:/i.test(x.trim()))||lines.some(x=>/^\[\d+\/\d+\]\s+.+/.test(x.trim())))return'log';
    const hasExec=/(?:winws(?:\.exe)?|nfqws(?:\.exe)?)/i.test(s);
    if(hasExec&&lines.some(x=>/^\s*(?:@echo|set\s|start\s|call\s|chcp\s|cd\s|::)/i.test(x))||hasExec&&/%(?:BIN|LISTS|~dp0)/i.test(s))return'batch';
    if(hasExec||/--(?:filter|dpi-desync|wf)-/i.test(s))return'command';
    return'unknown';
  }
  function strategyImportLogicalLines(content){
    const out=[];let buf='';
    for(const raw of String(content||'').replace(/\r/g,'').split('\n')){
      const line=raw.trim();if(!line&&!buf)continue;
      const cont=/\^\s*$/.test(line),part=line.replace(/\^\s*$/,'').trim();
      buf=buf?`${buf} ${part}`:part;
      if(!cont){out.push(buf);buf=''}
    }
    if(buf)out.push(buf);
    return out;
  }
  function strategyImportVariables(content){
    const vars={};
    for(const raw of String(content||'').replace(/\r/g,'').split('\n')){
      const m=raw.trim().match(/^set\s+"?([A-Za-z_][A-Za-z0-9_]*)=([^"]*)"?\s*$/i);
      if(m)vars[m[1].toUpperCase()]=m[2];
    }
    return vars;
  }
  function strategyImportExtractCommands(content){
    return strategyImportLogicalLines(content).filter(line=>/(?:winws(?:\.exe)?|nfqws(?:\.exe)?)/i.test(line)).map(line=>{
      const low=line.toLowerCase();let i=low.indexOf('winws.exe'),len=9;
      if(i<0){i=low.indexOf('nfqws.exe');len=9}
      if(i<0){i=low.indexOf('nfqws');len=5}
      return i>=0?line.slice(i+len).replace(/^"?\s*/,'').trim():'';
    }).filter(Boolean);
  }
  function strategyImportNormalizeCmdEscapes(text,transforms){
    let out=String(text||'');
    if(out.includes('^!')){out=out.replace(/\^!/g,'!');transforms.push(['CMD','^! → ! (Windows cmd escape)'])}
    if(out.includes('^^')){out=out.replace(/\^\^/g,'^');transforms.push(['CMD','^^ → ^ (Windows cmd escape)'])}
    return out;
  }
  function strategyImportExpandKnownVars(text,vars,transforms,warnings){
    let out=String(text||'');
    out=out.replace(/%~dp0bin\\/gi,()=>{transforms.push(['PATH','%~dp0bin\\ → /opt/etc/nfqws2/blobs/']);return'/opt/etc/nfqws2/blobs/'});
    out=out.replace(/%~dp0lists\\/gi,()=>{transforms.push(['PATH','%~dp0lists\\ → /opt/etc/nfqws2/lists/']);return'/opt/etc/nfqws2/lists/'});
    out=out.replace(/%BIN%/gi,()=>{transforms.push(['PATH','%BIN% → /opt/etc/nfqws2/blobs/']);return'/opt/etc/nfqws2/blobs/'});
    out=out.replace(/%LISTS%/gi,()=>{transforms.push(['PATH','%LISTS% → /opt/etc/nfqws2/lists/']);return'/opt/etc/nfqws2/lists/'});
    out=out.replace(/%([A-Za-z_][A-Za-z0-9_]*)%/g,(all,name)=>{
      const value=vars[String(name).toUpperCase()];
      if(value!==undefined&&!/%~dp0/i.test(value)){transforms.push(['VAR',`${all} → ${value}`]);return value}
      warnings.push(['bad',`Переменная ${all} не разрешена. RouterForge не будет подставлять значение по догадке.`]);return all;
    });
    out=strategyImportNormalizeCmdEscapes(out,transforms);
    return out.replace(/\\/g,'/').replace(/"/g,'').replace(/\s+/g,' ').trim();
  }
  function strategyImportDependencyKind(flag,value){
    const f=String(flag||'').toLowerCase(),v=String(value||'');
    if(['--hostlist','--hostlist-exclude','--ipset','--ipset-exclude'].includes(f))return'LIST';
    if(f==='--dpi-desync-split-seqovl-pattern')return'BLOB';
    if(f.startsWith('--dpi-desync-fake-')){
      if(/^0x[0-9a-f]+$/i.test(v)||v==='!'||v==='?'||/^(none|rnd)$/i.test(v))return'';
      if(/[\/]/.test(v)||/\.(?:bin|dat|raw)$/i.test(v))return'BLOB';
    }
    return'';
  }
  function strategyImportCollectDependencies(args,warnings){
    const deps=[],re=/(--[A-Za-z0-9_-]+)=([^\s]+)/g;let m;
    while((m=re.exec(String(args||'')))!==null){
      const kind=strategyImportDependencyKind(m[1],m[2]);if(!kind)continue;
      const path=m[2];deps.push([kind,path]);
      if(/^[A-Za-z]:|%|\\/.test(path))warnings.push(['warn',`Windows/path variable остался в зависимости: ${path}`]);
    }
    return[...new Map(deps.map(x=>[x.join('|'),x])).values()];
  }
  function strategyImportAnalyze(content){
    const type=strategyImportDetectType(content),vars=strategyImportVariables(content),commands=strategyImportExtractCommands(content),transforms=[],warnings=[];
    if(type==='log'&&!commands.length)return{type,commands:[],profiles:[],transforms,warnings:[['warn','Распознан лог тестирования. UX2.1 умеет определить его, но для построения candidate нужен .bat или сама команда стратегии.']],deps:[],preview:'',ready:false};
    if(!commands.length)return{type,commands,profiles:[],transforms,warnings:[['bad','Не найден запуск winws.exe/nfqws и аргументы стратегии.']],deps:[],preview:'',ready:false};
    if(commands.length>1)warnings.push(['bad',`Найдено несколько запусков (${commands.length}). UX2.1 не объединяет разные процессы автоматически.`]);
    let args=strategyImportExpandKnownVars(commands[0],vars,transforms,warnings);
    let tcp='',udp='';
    args=args.replace(/--wf-tcp=([^\s]+)/ig,(_,v)=>{tcp=v;transforms.push(['WF','--wf-tcp перенесён в TCP_PORTS preview']);return''});
    args=args.replace(/--wf-udp=([^\s]+)/ig,(_,v)=>{udp=v;transforms.push(['WF','--wf-udp перенесён в UDP_PORTS preview']);return''});
    args=args.replace(/\s+/g,' ').trim();
    const profiles=args.split(/\s+--new\s+/i).map(x=>x.trim()).filter(Boolean);
    if(profiles.length>1)transforms.push(['PROFILE',`Сохранён порядок ${profiles.length} профилей через --new`]);
    const deps=strategyImportCollectDependencies(args,warnings);
    const unresolved=[...new Set((`${tcp} ${udp} ${args}`.match(/%[A-Za-z_][A-Za-z0-9_]*%/g)||[]))];
    unresolved.forEach(v=>{if(!warnings.some(x=>x[1].includes(v)))warnings.push(['bad',`Неразрешённая переменная ${v}.`])});
    const preview=[
      '# RouterForge StrategyImport import preview',
      '# ТОЛЬКО ПРОСМОТР - not saved or applied',
      tcp?`TCP_PORTS="${tcp}"`:'# TCP_PORTS: not present in source',
      udp?`UDP_PORTS="${udp}"`:'# UDP_PORTS: not present in source',
      '',
      'NFQWS_ARGS_CUSTOM="',
      profiles.map((p,i)=>`${p}${i<profiles.length-1?' --new':''}`).join('\n'),
      '"'
    ].join('\n');
    const ready=commands.length===1&&!warnings.some(x=>x[0]==='bad');
    return{type,commands,profiles,tcp,udp,transforms:[...new Map(transforms.map(x=>[x.join('|'),x])).values()],warnings,deps,preview,ready};
  }
  let strategyImportResult=null,strategyImportLibraryCandidate='',strategyImportLibrarySavedName='';
  function strategyImportedCustom(result){return (result?.profiles||[]).map(x=>String(x||'').trim()).filter(Boolean).join('\n--new\n')}
  function strategyImportBuildLibraryCandidate(result){
    const active=String(current?.config||'');
    if(!active)return{ok:false,error:'Active canonical config недоступен в текущем runtime response.'};
    const required=['NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET','NFQWS_ARGS_CUSTOM','TCP_PORTS','UDP_PORTS'];
    const missing=required.filter(key=>!scanAssignment(active,key));
    if(missing.length)return{ok:false,error:`В active config не найдены обязательные assignments: ${missing.join(', ')}. RouterForge не будет достраивать структуру по догадке.`};
    const custom=strategyImportedCustom(result);
    if(!custom)return{ok:false,error:'Импорт не содержит strategy profiles для NFQWS_ARGS_CUSTOM.'};
    let cfg=active;
    for(const key of ['NFQWS_ARGS','NFQWS_ARGS_QUIC','NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET'])cfg=replaceAssignment(cfg,key,'');
    cfg=replaceAssignment(cfg,'NFQWS_ARGS_CUSTOM',custom);
    if(String(result.tcp||'').trim())cfg=replaceAssignment(cfg,'TCP_PORTS',String(result.tcp).trim());
    if(String(result.udp||'').trim())cfg=replaceAssignment(cfg,'UDP_PORTS',String(result.udp).trim());
    return{ok:true,content:cfg};
  }
  function defaultStrategyImportLibraryName(){
    const d=new Date(),pad=n=>String(n).padStart(2,'0');
    return `strategy-import-${d.getFullYear()}${pad(d.getMonth()+1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}.conf`;
  }
  function updateStrategyImportLibrarySave(){
    const name=normalizeStoredConfigName($('strategyImportLibraryName').value);
    $('strategyImportLibrarySave').disabled=!(strategyImportResult?.ready&&strategyImportLibraryCandidate&&name);
  }
  function renderStrategyImportSandboxProfiles(result){
    const select=$('strategyImportSandboxProfile'),profiles=Array.isArray(result?.profiles)?result.profiles:[];
    select.innerHTML=profiles.map((_,i)=>`<option value="${i}">Профиль ${i+1}${profiles.length>1?` из ${profiles.length}`:''}</option>`).join('');
    select.disabled=!result?.ready||!profiles.length;
    $('strategyImportSandboxOpen').disabled=!result?.ready||!profiles.length;
    $('strategyImportStrategyImport').disabled=!result?.ready||!profiles.length;
    $('strategyImportStrategyImportState').textContent=result?.ready&&profiles.length?'READY':'WAITING';
  }

  async function importStrategyImportProfilesToStrategyLibrary(){
    const result=strategyImportResult,profiles=Array.isArray(result?.profiles)?result.profiles:[];
    if(!result?.ready||!profiles.length){notice('Импорт стратегии: нужен READY preview.','error');return}
    const items=profiles.map((profile,i)=>({name:`Импортированный профиль ${i+1}`,args:v2ArgTokens(profile)}));
    if(items.some(x=>!x.args.length)){notice('Импорт стратегии: найден пустой profile.','error');return}
    if(!confirm(`Добавить ${items.length} profile(s) в Strategy Library? Active nfqws2.conf не изменится.`))return;
    $('strategyImportStrategyImport').disabled=true;$('strategyImportStrategyImportState').textContent='СОХРАНЕНИЕ';
    try{
      const d=await api('/v2/strategies/import',{method:'POST',body:JSON.stringify({items,confirm:'ROUTERFORGE_V2_STRATEGY_IMPORT'})});
      v2Library=[];v2RegistryData=null;
      $('strategyImportStrategyImportState').textContent='СОХРАНЕНО';
      notice(`Импорт стратегии: added ${Number(d.added||0)}, existing ${Number(d.existing||0)}. Production не изменён.`,'good');
    }catch(e){
      $('strategyImportStrategyImportState').textContent='ОШИБКА';notice(`Импорт стратегии: ${e.message}`,'error');
    }finally{$('strategyImportStrategyImport').disabled=false}
  }
  function sendStrategyImportProfileToSandbox(){
    if(!strategyImportResult?.ready){notice('Импорт стратегии: Песочница доступна только после успешной проверки.','error');return}
    const index=Number($('strategyImportSandboxProfile').value||0),profile=strategyImportResult.profiles?.[index];
    if(!profile){notice('Не удалось выбрать импортированный профиль.','error');return}
    v2OpenSandboxFromArgs(profile,`Импортированный профиль ${index+1}`,'import');
    notice(`Импорт стратегии · профиль ${index+1}/${strategyImportResult.profiles.length} загружен в песочницу. Рабочая настройка не изменялась.`,'good');
  }
  function renderStrategyImportLibrary(result){
    strategyImportResult=result;strategyImportLibraryCandidate='';strategyImportLibrarySavedName='';$('strategyImportLibraryOpen').disabled=true;
    renderStrategyImportSandboxProfiles(result);
    if(!result?.ready){
      $('strategyImportLibraryState').textContent=result?.preview?'CHECK':'WAITING';$('strategyImportLibrarySave').disabled=true;
      $('strategyImportLibraryPlan').textContent=result?.preview?'Есть блокирующие или неоднозначные места. Исправьте их до сохранения.':'Сначала нужна успешная проверка. Результат с замечаниями не сохраняется.';
      $('strategyImportLibraryDiff').textContent='Diff недоступен.';return;
    }
    const built=strategyImportBuildLibraryCandidate(result);
    if(!built.ok){
      $('strategyImportLibraryState').textContent='CHECK';$('strategyImportLibrarySave').disabled=true;$('strategyImportLibraryPlan').textContent=built.error;$('strategyImportLibraryDiff').textContent='Diff недоступен.';return;
    }
    strategyImportLibraryCandidate=built.content;
    if(!$('strategyImportLibraryName').value.trim())$('strategyImportLibraryName').value=defaultStrategyImportLibraryName();
    $('strategyImportLibraryState').textContent='READY';
    $('strategyImportLibraryPlan').textContent=`Будет создан новый library config. Environment/system settings сохраняются из active config; стандартные strategy slots очищаются; ${result.profiles.length} импортированных profiles записываются в NFQWS_ARGS_CUSTOM${result.tcp?'; TCP_PORTS берётся из --wf-tcp':''}${result.udp?'; UDP_PORTS берётся из --wf-udp':''}.`;
    $('strategyImportLibraryDiff').innerHTML=makeDiff(String(current?.config||''),built.content).map(x=>`<span class="${x.c}">${esc(x.t)}</span>`).join('\n')||'<span class="same">Нет изменений.</span>';
    updateStrategyImportLibrarySave();
  }
  async function saveStrategyImportLibraryCandidate(){
    if(!strategyImportResult?.ready||!strategyImportLibraryCandidate){notice('Импорт стратегии: сохранение доступно только после успешной проверки.','error');return}
    const name=normalizeStoredConfigName($('strategyImportLibraryName').value);
    if(!name){notice('Имя конфига: только буквы, цифры, точка, _ и -, расширение .conf.','error');return}
    if(!confirm(`Создать ${name} в Config Library? Active nfqws2.conf и runtime не изменятся.`))return;
    $('strategyImportLibrarySave').disabled=true;
    try{
      await api('/configs/create',{method:'POST',body:JSON.stringify({name,content:strategyImportLibraryCandidate,from_current:false,confirm:'NFQWS_CONFIG_LIBRARY_CREATE'})});
      strategyImportLibrarySavedName=name;configLibraryLoaded=false;$('strategyImportLibraryState').textContent='СОХРАНЕНО';$('strategyImportLibraryOpen').disabled=false;
      notice(`Импорт сохранён как ${name}. Active config не менялся, nfqws2 не перезапускался.`,'good');
    }catch(e){notice(e.message,'error')}finally{updateStrategyImportLibrarySave()}
  }
  function openSavedStrategyImportConfig(){
    if(!strategyImportLibrarySavedName)return;
    activateNFQWSTab('configs');loadConfigLibrary(true).then(()=>selectStoredConfig(strategyImportLibrarySavedName));
  }
  function renderStrategyImport(result){
    $('strategyImportType').textContent=result.type==='batch'?'Windows batch':result.type==='command'?'Команда':result.type==='log'?'Лог теста':'Неизвестно';
    $('strategyImportCommands').textContent=String(result.commands.length);$('strategyImportProfiles').textContent=String(result.profiles.length);
    $('strategyImportVerdict').textContent=result.ready?'ГОТОВО':result.preview?'ПРОВЕРЬТЕ':'НУЖНЫ ДАННЫЕ';
    $('strategyImportState').textContent=result.ready?'ГОТОВО':result.preview?'ПРОВЕРЬТЕ':'ОЖИДАНИЕ';$('strategyImportState').className=`testlab-state ${result.ready?'done':result.preview?'running':'idle'}`;
    $('strategyImportMessage').textContent=result.ready?'Preview построен. Ничего не сохранено: сначала проверьте преобразования и зависимости.':result.preview?'Preview построен с блокирующими/неоднозначными местами. Исправьте их до следующего этапа импорта.':'RouterForge пока не может собрать стратегию из этих данных.';
    $('strategyImportWarnings').innerHTML=result.warnings.map(x=>`<div class="strategy-import-warning ${x[0]}">${esc(x[1])}</div>`).join('');
    $('strategyImportTransforms').innerHTML=result.transforms.length?result.transforms.map(x=>`<div class="strategy-import-row"><b>${esc(x[0])}</b><span>${esc(x[1])}</span></div>`).join(''):'<div class="analysis-empty">Автоматических преобразований нет.</div>';
    $('strategyImportDeps').innerHTML=result.deps.length?result.deps.map(x=>`<div class="strategy-import-row"><b>${esc(x[0])}</b><span class="mono">${esc(x[1])}</span></div>`).join(''):'<div class="analysis-empty">Явные list/blob зависимости не найдены.</div>';
    $('strategyImportPreview').innerHTML=result.preview?highlightConfig(result.preview):esc('Preview не построен.');$('strategyImportCopy').disabled=!result.preview;
    renderStrategyImportLibrary(result);reportHeight();
  }
  async function runStrategyImport(){
    const content=$('strategyImportInput').value;
    if(!String(content||'').trim()){notice('Импорт стратегии: добавьте команду или файл.','error');return}
    $('strategyImportRun').disabled=true;$('strategyImportState').textContent='ПРОВЕРКА';
    try{
      const d=await api('/v2/import/parse',{method:'POST',body:JSON.stringify({content})});
      const result={
        type:d.type||'unknown',commands:Array.isArray(d.commands)?d.commands:[],profiles:Array.isArray(d.profiles)?d.profiles:[],
        tcp:d.tcp||'',udp:d.udp||'',
        transforms:(d.transforms||[]).map(x=>[x.kind||'',x.text||'']),
        warnings:(d.warnings||[]).map(x=>[x.level||'warn',x.text||'']),
        deps:(d.deps||[]).map(x=>[x.kind||'',x.path||'']),
        preview:d.preview||'',ready:!!d.ready,log:d.log||null
      };
      if(result.type==='log'&&result.log){
        result.warnings.unshift(['warn',`Log parity: format=${result.log.format||'unknown'} · strategies=${Number(result.log.strategies||0)} · targets=${Number(result.log.targets||0)} · tests=${Number(result.log.tests||0)} · passed=${Number(result.log.passed||0)} · failed=${Number(result.log.failed||0)}`]);
      }
      renderStrategyImport(result);
      notice(`Импорт стратегии: ${result.type} · ${result.profiles.length} profile(s) · ${result.ready?'READY':'CHECK'}`,result.ready?'good':'');
    }catch(e){
      $('strategyImportState').textContent='ОШИБКА';notice(`Импорт стратегии: ${e.message}`,'error');
    }finally{$('strategyImportRun').disabled=false;reportHeight()}
  }
  function clearStrategyImport(){
    $('strategyImportInput').value='';refreshSyntaxEditor($('strategyImportInput'));$('strategyImportSourceMeta').textContent='Команда · .bat · .cmd · .txt · лог теста';
    $('strategyImportLibraryName').value='';
    renderStrategyImport({type:'unknown',commands:[],profiles:[],tcp:'',udp:'',transforms:[],warnings:[],deps:[],preview:'',ready:false});
  }
  function loadStrategyImportFile(file){
    if(!file)return;if(file.size>262144){notice('Импорт стратегии: файл больше 256 KiB.','error');return}
    const reader=new FileReader();
    reader.onload=()=>{$('strategyImportInput').value=String(reader.result||'');refreshSyntaxEditor($('strategyImportInput'));$('strategyImportSourceMeta').textContent=`${file.name} · ${bytes(file.size)}`;runStrategyImport()};
    reader.onerror=()=>notice('Не удалось прочитать файл в браузере.','error');reader.readAsText(file);
  }


  /* NFQWS V2 functional workflow. Legacy workspaces stay in DOM for compatibility
     but are no longer exposed by navigation. */
  let v2TargetSources=[],v2Profiles=[],v2Library=[],v2DomainLast=null;
  let v2LabAbort=null,v2SandboxAbort=null,v2SelectorAbort=null,v2SelectorPoll=0,v2SelectorLast=null,v2SelectorPreviewData=null;
  let v2SelectorLastError='',v2SelectorPreviewError='',v2SelectorApplyError='';
  let v2AutoSetupRunning=false,v2AutoSetupTimeline=[],v2AutoSetupDetect=null,v2AutoSetupInspect=null,v2AutoSetupTarget='';

  function v2Tone(value){
    const v=String(value||'').toUpperCase();
    if(['PASS','WORKING','CLEAR','DONE','OK'].includes(v))return'good';
    if(['FAILED','FAIL','DNS_FAILURE','TCP_FAILURE','TLS_FAILURE','HTTP_FAILURE','ERROR','INCONCLUSIVE'].includes(v))return'bad';
    if(['UNSTABLE','PARTIAL','HTTP_RESTRICTED','PARTIAL_16K_SUSPECTED','CHECK'].includes(v))return'warn';
    return'info';
  }
  function v2Pill(value){const v=String(value||'—');return`<span class="v2-pill ${v2Tone(v)}">${esc(v)}</span>`}
  function v2ArgTokens(value){
    const raw=String(value||'').match(/(?:[^\s"'\\]+|"(?:\\.|[^"])*"|'(?:\\.|[^'])*')+/g)||[];
    return raw.map(x=>{
      if(x.length>=2&&((x[0]==='"'&&x[x.length-1]==='"')||(x[0]==="'"&&x[x.length-1]==="'")))return x.slice(1,-1).replace(/\\(["'\\])/g,'$1');
      return x;
    }).filter(Boolean);
  }
  function v2ArgsText(args){return(Array.isArray(args)?args:[]).join(' ')}
  function v2SessionID(){
    const a=new Uint8Array(8);if(globalThis.crypto?.getRandomValues)crypto.getRandomValues(a);else for(let i=0;i<a.length;i++)a[i]=Math.floor(Math.random()*256);
    return'ui-'+[...a].map(x=>x.toString(16).padStart(2,'0')).join('');
  }
  async function v2EnsureStatus(){if(current?.config_sha256)return current;current=await api('/status');original=current?.config||original;return current}
  async function v2LoadTargetSources(force=false){
    if(v2TargetSources.length&&!force)return v2TargetSources;
    const d=await api('/v2/target-sources');v2TargetSources=Array.isArray(d.sources)?d.sources:[];
    const options=v2TargetSources.map(x=>`<option value="${esc(x.name)}">${esc(x.name)} · ${Number(x.count||0)}</option>`).join('');
    for(const id of['v2LabList','v2SandboxTargetList','v2AutoSetupList']){const n=$(id);if(n)n.innerHTML=options||'<option value="">Нет списков</option>'}
    return v2TargetSources;
  }
  async function v2LoadProfiles(force=false){
    if(v2Profiles.length&&!force)return v2Profiles;
    const d=await api('/bench-strategies');v2Profiles=(Array.isArray(d.profiles)?d.profiles:[]).filter(x=>x.candidate_eligible);
    const sel=$('v2SandboxProfile');if(sel)sel.innerHTML=v2Profiles.map(p=>`<option value="${Number(p.index)}">Profile ${Number(p.index)}${Array.isArray(p.strategy_tags)&&p.strategy_tags.length?` · tags ${p.strategy_tags.join(',')}`:''}</option>`).join('');
    return v2Profiles;
  }
  async function v2LoadLibrary(force=false){
    if(v2Library.length&&!force)return v2Library;
    const d=await api('/v2/strategies');v2Library=Array.isArray(d.strategies)?d.strategies:[];
    const sel=$('v2SandboxLibrary');if(sel)sel.innerHTML=v2Library.map(x=>`<option value="${esc(x.id)}">${esc(x.name)} · ${esc(x.source)}</option>`).join('')||'<option value="">Library пуста</option>';
    renderV2Library();
    return v2Library;
  }
  async function v2ResolveTargets(source,listName,text){
    if(source==='list'){
      const d=await api('/v2/targets/resolve',{method:'POST',body:JSON.stringify({source:'list',list_name:listName,limit:64})});
      return d.targets||[];
    }
    const raw=String(text||'').split(/\r?\n/).map(x=>x.trim()).filter(Boolean);
    const d=await api('/v2/targets/resolve',{method:'POST',body:JSON.stringify({source:'manual',targets:raw,limit:64})});
    return d.targets||[];
  }
  function v2StageRow(name,row){
    const state=String(row?.state||'skipped'),detail=row?.detail||'',extra=[row?.status_code?`HTTP ${row.status_code}`:'',Number.isFinite(Number(row?.bytes))&&Number(row.bytes)>0?`${row.bytes} B`:'',Number.isFinite(Number(row?.latency_ms))?`${row.latency_ms} ms`:''].filter(Boolean).join(' · ');
    return`<div class="v2-row ${v2Tone(state)}"><div class="v2-result-title">${esc(name.toUpperCase())}</div><div><div class="value">${esc(detail||'—')}</div><div class="meta">${esc(extra)}</div></div>${v2Pill(state)}</div>`;
  }


  function v2AutoSetupLabel(code){
    const labels={
      IDLE:'Готово к запуску',READY:'Цель выбрана',INSPECTING:'Проверяю сайт',
      TARGET_ALREADY_HEALTHY:'Сайт уже работает',DNS_FAILURE:'Проблема с DNS',
      ROUTING_FAILURE:'Проблема с маршрутом',LIVE_VERIFY:'Подбираю стратегию',
      DEPENDENCY_MISSING:'Автоподбор не запущен',NO_CANDIDATES_COMPILE:'Нет кандидатов для проверки',
      NO_CANDIDATE_WORKS:'Рабочая стратегия не найдена',CLEANUP_FAILURE:'Безопасная проверка остановлена',
      WINNER_VERIFIED:'Рабочая стратегия найдена',PREVIEW_READY:'Изменения готовы к просмотру',
      APPLY_SUCCESS:'Изменения применены',APPLY_ROLLBACK:'Изменения отменены',
      TLS_HANDSHAKE_RESET:'TLS-соединение сбрасывается',CLEAR_END_TO_END:'Проверка доступа успешна',
      INSPECTED:'Проверена текущая конфигурация',CHECKS:'Проверена цепочка доступа',
      NO_CHANGES:'Изменения не требуются',SELECTOR_BLOCKED:'Автоподбор не запущен'
    };
    return labels[String(code||'').toUpperCase()]||String(code||'Состояние').replaceAll('_',' ');
  }
  function v2AutoSetupPush(code,title,detail='',tone='info'){
    v2AutoSetupTimeline.push({code,title,detail,tone});
    const root=$('v2AutoSetupTimeline');if(!root)return;
    root.innerHTML=v2AutoSetupTimeline.map(x=>`<div class="v2-row ${esc(x.tone||'info')}"><div class="v2-result-title">${esc(x.title||v2AutoSetupLabel(x.code))}</div><div><div class="value">${esc(x.detail||'')}</div><div class="meta">Технический код: ${esc(x.code)}</div></div>${v2Pill(v2AutoSetupLabel(x.code))}</div>`).join('');
  }
  function v2AutoSetupState(code,message,tone='info'){
    $('v2AutoSetupState').textContent=v2AutoSetupLabel(code);
    $('v2AutoSetupStatus').innerHTML=`<div class="v2-row ${esc(tone)}"><div class="v2-result-title">${esc(v2AutoSetupLabel(code))}</div><div><div class="value">${esc(message||'')}</div><div class="meta">Технический код: ${esc(code)}</div></div>${v2Pill(v2AutoSetupLabel(code))}</div>`;
    reportHeight();
  }
  function v2AutoSetupStageSummary(detect){
    return ['dns','tcp','tls','http'].map(k=>{
      const state=String(detect?.stages?.[k]?.state||'skipped').toUpperCase();
      const mark=['PASS','OK','CLEAR','WORKING'].includes(state)?'✓':state==='SKIPPED'?'—':'✕';
      return `${k.toUpperCase()} ${mark}`;
    }).join(' · ');
  }
  function v2AutoSetupBlockerText(value){
    const s=String(value||'');
    const l=s.toLowerCase();
    if(l.includes('active management ssh session'))return 'Активна SSH-сессия управления; live-тесты специально заблокированы.';
    if(l.includes('candidate nfqws2 executable'))return 'Не удалось подтвердить исполняемый файл nfqws2 для тестового процесса.';
    if(l.includes('firewall nfqueue inventory'))return 'Не удалось безопасно прочитать NFQUEUE-правила firewall.';
    if(l.includes('kernel nfqueue binding inventory'))return 'Не удалось подтвердить состояние NFQUEUE в ядре.';
    if(l.includes('no free queue'))return 'Нет свободной тестовой NFQUEUE в зарезервированном диапазоне.';
    if(l.includes('cleanup baseline'))return 'Не доказано, что тестовый NFQUEUE-диапазон чист до запуска.';
    if(l.includes('management ssh session inventory'))return 'Не удалось безопасно определить активные SSH-сессии управления.';
    return s;
  }
  async function v2AutoSetupCapabilityDetails(){
    try{
      const d=await api('/bench-capabilities');
      const blockers=(d.blockers||[]).map(v2AutoSetupBlockerText);
      return blockers.length?blockers:['Безопасность тестового стенда не подтверждена, но конкретный blocker не указан.'];
    }catch(e){
      return [`Не удалось получить детали готовности тестового стенда: ${e.message}`];
    }
  }
  function v2AutoSetupErrorCode(message){
    const m=String(message||'').toLowerCase();
    if(m.includes('cleanup'))return'CLEANUP_FAILURE';
    if(m.includes('config changed')||m.includes('active config changed'))return'CONFIG_CHANGED_WHILE_TESTING';
    if(m.includes('rollback')||m.includes('rolled back'))return'APPLY_ROLLBACK';
    if(m.includes('dependency')||m.includes('capability')||m.includes('not installed')||m.includes('missing'))return'DEPENDENCY_MISSING';
    if(m.includes('preview')&&(m.includes('stale')||m.includes('changed')||m.includes('expired')||m.includes('mismatch')))return'PREVIEW_STALE';
    return'WORKFLOW_ERROR';
  }
  function v2AutoSetupReset(){
    v2AutoSetupRunning=false;v2AutoSetupTimeline=[];v2AutoSetupDetect=null;v2AutoSetupInspect=null;v2AutoSetupTarget='';
    $('v2AutoSetupRun').disabled=false;$('v2AutoSetupApply').disabled=true;
    $('v2AutoSetupWinner').style.display='none';$('v2AutoSetupPreviewPanel').style.display='none';
    $('v2AutoSetupTimeline').innerHTML='';$('v2AutoSetupPreviewBody').textContent='Preview ещё не построен.';
    v2AutoSetupState('IDLE','Выберите target и запустите workflow.');
  }
  async function v2AutoSetupLoadObserved(){
    const d=await api('/v2/observed-targets');v2ObservedData=d||v2ObservedData||{};
    const rows=(d.observed||[]).filter(v2ObservedCanDomain);
    $('v2AutoSetupObserved').innerHTML=rows.map(x=>`<option value="${esc(v2ObservedTargetValue(x))}">${esc(v2ObservedTargetValue(x))} · ${esc(v2ObservedReasonText(x.reason))} · ${Number(x.count||0)}</option>`).join('')||'<option value="">Нет domain-like Observed targets</option>';
  }
  async function v2AutoSetupLoadListTargets(){
    const name=$('v2AutoSetupList').value;if(!name){$('v2AutoSetupListTarget').innerHTML='<option value="">Нет списка</option>';return}
    const d=await api('/v2/targets/resolve',{method:'POST',body:JSON.stringify({source:'list',list_name:name,limit:64})});
    const targets=d.targets||[];$('v2AutoSetupListTarget').innerHTML=targets.map(x=>`<option value="${esc(x)}">${esc(x)}</option>`).join('')||'<option value="">В списке нет пригодных domain targets</option>';
  }
  async function v2AutoSetupSourceChanged(){
    const source=$('v2AutoSetupSource').value;
    $('v2AutoSetupManualWrap').style.display=source==='manual'?'':'none';
    $('v2AutoSetupObservedWrap').style.display=source==='observed'?'':'none';
    $('v2AutoSetupListWrap').style.display=source==='list'?'':'none';
    try{
      if(source==='observed')await v2AutoSetupLoadObserved();
      if(source==='list'){await v2LoadTargetSources(true);await v2AutoSetupLoadListTargets()}
    }catch(e){v2AutoSetupState(v2AutoSetupErrorCode(e.message),e.message,'bad')}
  }
  function v2AutoSetupSelectedTarget(){
    const source=$('v2AutoSetupSource').value;
    if(source==='observed')return String($('v2AutoSetupObserved').value||'').trim();
    if(source==='list')return String($('v2AutoSetupListTarget').value||'').trim();
    return String($('v2AutoSetupTarget').value||'').trim();
  }
  function v2AutoSetupSeed(target,source='manual'){
    activateNFQWSTab('auto-setup-v1');
    $('v2AutoSetupSource').value='manual';$('v2AutoSetupTarget').value=String(target||'').trim();
    void v2AutoSetupSourceChanged();
    v2AutoSetupState('READY',`Target получен из ${source}: ${String(target||'').trim()}`);
  }
  function v2AutoSetupDiagnosisStop(detect){
    const diag=detect?.diagnostic||{},code=String(diag.code||'').toUpperCase(),fault=String(diag.fault_domain||'').toLowerCase();
    if(String(detect?.classification||'').toLowerCase()==='clear'||code==='CLEAR_END_TO_END')return['TARGET_ALREADY_HEALTHY','Baseline уже здоров; strategy mutation не нужна.','good'];
    if(code.startsWith('DNS_')||fault==='dns')return['DNS_FAILURE',diag.suggested_action||'DNS stage не прошёл.','bad'];
    if(fault==='routing'||code==='ROUTING_FAILURE')return['ROUTING_FAILURE',diag.suggested_action||'Проблема маршрута не должна маскироваться стратегией NFQWS.','bad'];
    return null;
  }
  async function v2AutoSetupBuildPreview(){
    v2SelectorPreviewError='';
    const p=await buildV2SelectorPreview();
    if(!p){
      const code=v2AutoSetupErrorCode(v2SelectorPreviewError);
      v2AutoSetupState(code,v2SelectorPreviewError||'Preview не построен.','bad');
      v2AutoSetupPush(code,'Exact Preview не готов',v2SelectorPreviewError,'bad');
      return null;
    }
    $('v2AutoSetupPreviewPanel').style.display='block';
    $('v2AutoSetupPreviewMeta').textContent=`active=${p.active_config_sha256} · candidate=${p.candidate_config_sha256} · source=${p.candidate_source||'production'} · fingerprint=${p.candidate_fingerprint||'—'} · resources=${(p.required_resources||[]).length}`;
    $('v2AutoSetupPreviewBody').textContent=p.candidate_config||'';
    $('v2AutoSetupApply').disabled=!p.changed;
    v2AutoSetupPush('PREVIEW_READY','Exact deterministic Preview готов',`${p.candidate_config_sha256||''} · production не изменён`,'good');
    v2AutoSetupState('PREVIEW_READY','Winner проверен и exact config готов. Следующий шаг требует явного Apply.','good');
    return p;
  }
  async function runV2AutoSetup(){
    if(v2AutoSetupRunning)return;
    const target=v2AutoSetupSelectedTarget();if(!target){v2AutoSetupState('TARGET_REQUIRED','Выберите или введите target.','bad');return}
    v2AutoSetupRunning=true;v2AutoSetupTimeline=[];v2AutoSetupTarget=target;v2SelectorLastError='';v2SelectorPreviewError='';v2SelectorApplyError='';
    $('v2AutoSetupRun').disabled=true;$('v2AutoSetupApply').disabled=true;$('v2AutoSetupWinner').style.display='none';$('v2AutoSetupPreviewPanel').style.display='none';
    try{
      await Promise.all([v2EnsureStatus(),v2LoadLibrary()]);
      v2AutoSetupState('INSPECTING',`Проверяю ${target}: production mapping + baseline DNS/TCP/TLS/HTTP.`,'info');
      const [inspect,detect]=await Promise.all([
        api('/v2/inspect-target',{method:'POST',body:JSON.stringify({target})}),
        api('/v2/detect',{method:'POST',body:JSON.stringify({target})})
      ]);
      v2AutoSetupInspect=inspect;v2AutoSetupDetect=detect;
      const diag=detect.diagnostic||{},stageSummary=v2AutoSetupStageSummary(detect);
      v2AutoSetupPush('INSPECTED','Проверена текущая конфигурация',`Подходящих активных профилей: ${(inspect.production_profile_matches||[]).length} · совпадений в списках: ${(inspect.list_matches||[]).length}`,'good');
      v2AutoSetupPush('CHECKS','Проверена цепочка доступа',stageSummary,detect.classification==='clear'?'good':'warn');
      v2AutoSetupPush(diag.code||detect.classification||'BASELINE',v2AutoSetupLabel(diag.code||detect.classification||'BASELINE'),diag.suggested_action||detect.classification_text||'Диагностика завершена.',detect.classification==='clear'?'good':'warn');
      const stop=v2AutoSetupDiagnosisStop(detect);
      if(stop){
        if(stop[0]==='TARGET_ALREADY_HEALTHY'){
          v2AutoSetupPush('NO_CHANGES','Изменения не требуются','Сайт уже доступен. Автоподбор не запускался, рабочая конфигурация не изменялась.','good');
          v2AutoSetupState(stop[0],`Сайт уже работает. ${stageSummary}. Ничего менять не требуется.`,'good');
        }else{
          v2AutoSetupPush('NO_CHANGES','Стратегии NFQWS не подбирались','Проблема находится вне этапа, который должна исправлять стратегия NFQWS. Рабочая конфигурация не изменялась.','warn');
          v2AutoSetupState(stop[0],`${stop[1]} ${stageSummary}`.trim(),stop[2]);
        }
        return
      }

      if(String(diag.code||'').toUpperCase()==='HTTP_STREAM_CUTOFF_12_20K_SUSPECTED'){
        v2AutoSetupState('TCP16_REVALIDATE','Диагностика указывает на 12–20 KiB cutoff; запускаю bounded TCP16 network probe.','warn');
        const tcp16=await api('/v2/tcp16-probe',{method:'POST',body:JSON.stringify({scan_sni:true,expected_config_sha256:current.config_sha256,confirm:'ROUTERFORGE_TCP16_PROBE_V2'})});
        v2AutoSetupPush('TCP16_DONE','TCP16 branch завершён',`detected=${Number(tcp16.detected_count||0)} · working SNI=${Number(tcp16.working_sni_count||0)} · production read-only=${!!tcp16.read_only_production}`,'good');
      }

      v2AutoSetupState('LIVE_VERIFY','Строю bounded plan и запускаю isolated live Selector.','info');
      $('v2SelectorTarget').value=target;$('v2SelectorMode').value=$('v2AutoSetupMode').value;$('v2SelectorConcurrency').value=$('v2AutoSetupConcurrency').value;
      $('v2SelectorProduction').checked=true;$('v2SelectorAutoPool').checked=true;$('v2SelectorLibrary').checked=true;
      const selected=await runV2Selector();
      if(!selected){
        const raw=v2SelectorLastError||'Автоподбор не завершился.';
        if(String(raw).toLowerCase().includes('capability gates')){
          const blockers=await v2AutoSetupCapabilityDetails();
          const detail=blockers.join(' ');
          v2AutoSetupState('DEPENDENCY_MISSING',`Проблема найдена, но безопасный Автоподбор не был запущен. ${detail}`,'bad');
          v2AutoSetupPush('SELECTOR_BLOCKED','Автоподбор не запущен',detail,'bad');
          v2AutoSetupPush('NO_CHANGES','Ничего не изменено','Production-конфигурация осталась без изменений.','good');
          return
        }
        const code=v2AutoSetupErrorCode(raw);v2AutoSetupState(code,raw,'bad');v2AutoSetupPush(code,'Автоподбор остановлен',raw,'bad');return
      }
      if(!selected.cleanup_baseline_after){v2AutoSetupState('CLEANUP_FAILURE','Isolated test не доказал cleanup baseline. Apply заблокирован.','bad');v2AutoSetupPush('CLEANUP_FAILURE','Cleanup proof failed','','bad');return}
      if(!(selected.candidates||[]).length){v2AutoSetupState('NO_CANDIDATES_COMPILE','Нет пригодных кандидатов для live verification.','bad');v2AutoSetupPush('NO_CANDIDATES_COMPILE','Candidate set пуст после compiler/gates','','bad');return}
      if(!selected.recommendation_available){v2AutoSetupState('NO_CANDIDATE_WORKS',selected.recommendation_reason||'Ни один candidate не улучшил baseline.','warn');v2AutoSetupPush('NO_CANDIDATE_WORKS','Live verification не нашёл winner',selected.recommendation_reason||'','warn');return}
      if(!selected.apply_gate_eligible){v2AutoSetupState('WINNER_VERIFIED','Winner подтверждён, но unique production slot для Safe Apply не доказан.','warn');v2AutoSetupPush('WINNER_VERIFIED','Winner live-verified',selected.apply_gate_reason||'Apply gate blocked','warn');return}

      $('v2AutoSetupWinner').style.display='block';
      $('v2AutoSetupWinnerBody').innerHTML=`<div class="v2-row good"><div class="v2-result-title">${esc(selected.recommended_candidate_name||selected.recommended_candidate_id||'winner')}</div><div><div class="value">${esc(selected.recommendation_reason||'Live verified')}</div><div class="meta">${esc(selected.recommended_candidate_source||'')} · ${esc(selected.apply_gate_reason||'')}</div></div>${v2Pill('WINNER VERIFIED')}</div>`;
      v2AutoSetupPush('WINNER_VERIFIED','Live winner повторно подтверждён',`${selected.recommended_candidate_source||''} · ${selected.recommended_candidate_id||''}`,'good');
      await v2AutoSetupBuildPreview();
    }catch(e){
      const code=v2AutoSetupErrorCode(e.message);v2AutoSetupState(code,e.message,'bad');v2AutoSetupPush(code,'Workflow остановлен',e.message,'bad');
    }finally{
      v2AutoSetupRunning=false;$('v2AutoSetupRun').disabled=false;reportHeight();
    }
  }
  async function applyV2AutoSetup(){
    if(v2AutoSetupRunning||!v2SelectorPreviewData?.changed)return;
    v2AutoSetupRunning=true;$('v2AutoSetupApply').disabled=true;v2SelectorApplyError='';
    try{
      const result=await applyV2SelectorPreview();
      if(!result){
        const code=v2AutoSetupErrorCode(v2SelectorApplyError);
        v2AutoSetupState(code,v2SelectorApplyError||'Apply не выполнен.','bad');
        v2AutoSetupPush(code,'Safe Apply не завершён',v2SelectorApplyError,'bad');
        return
      }
      if(result.rolled_back){v2AutoSetupState('APPLY_ROLLBACK','Smart Apply откатил изменение после failed verify.','bad');v2AutoSetupPush('APPLY_ROLLBACK','Rollback выполнен','','bad');return}
      const latest=await api('/status');
      if(String(latest.config_sha256||'').toLowerCase()!==String(result.config_sha256||'').toLowerCase()){
        v2AutoSetupState('APPLY_ROLLBACK','Post-Apply production SHA не совпал с verified candidate.','bad');v2AutoSetupPush('APPLY_ROLLBACK','Production verification mismatch','','bad');return
      }
      v2AutoSetupPush('APPLY_SUCCESS','Production verification PASS',`config=${latest.config_sha256} · running=${!!latest.running}`,'good');
      v2AutoSetupState('APPLY_SUCCESS','Готово: exact candidate применён через Smart Apply и production verify прошёл.','good');
    }catch(e){
      const code=v2AutoSetupErrorCode(e.message);v2AutoSetupState(code,e.message,'bad');v2AutoSetupPush(code,'Apply workflow остановлен',e.message,'bad');
    }finally{v2AutoSetupRunning=false;$('v2AutoSetupApply').disabled=false;reportHeight()}
  }

  async function runV2Domain(){
    const target=String($('v2DomainTarget').value||'').trim();if(!target){notice('Проверка домена: укажите hostname.','error');return}
    $('v2DomainState').textContent='RUNNING';$('v2DomainRun').disabled=true;$('v2DomainToSelector').style.display='none';
    $('v2DomainGuidance').textContent='Проверяем DNS, TCP, TLS и HTTPS с роутера…';
    $('v2DomainContext').textContent='Сопоставляем результат с текущими профилями и списками…';
    try{
      const site=await api('/v2/site-diagnose',{method:'POST',body:JSON.stringify({target})});
      const inspect=site.inspect||{},detect=site.detect||{};
      v2DomainLast={inspect,detect};$('v2DomainTarget').value=inspect.target||detect.target||target;
      $('v2DomainState').textContent=String(detect.classification||'DONE').toUpperCase();
      const diag=detect.diagnostic||{};
      const siteContext=site.context||{};
      const contextRows=[
        ['Связь со стратегией',String(siteContext.binding_detail||'Недостаточно сведений о профилях')],
        ['Ограничение проверки',String(siteContext.network_scope||'Сеть клиента не проверена')]
      ];
      if(siteContext.tcp16_detail)contextRows.push(['Передача данных',String(siteContext.tcp16_detail)]);
      $('v2DomainContext').innerHTML='<div class="v2-result-title">'+esc(siteContext.title||'Контекст диагностики')+'</div>'+
        contextRows.map(([label,value])=>'<div class="v2-row" style="margin-top:8px"><div class="v2-result-title">'+esc(label)+'</div><div class="value">'+esc(value)+'</div></div>').join('');
      const guide=detect.guidance||{};
      const guideTitle=String(guide.title||'Диагностика завершена');
      const guideSummary=String(guide.summary||diag.suggested_action||'');
      const guideScope=String(guide.scope||'Проверка выполняется с роутера, не с клиентского устройства.');
      const runtimeNotice=!inspect.runtime_running&&guide.next_action==='selector'
        ?'NFQWS2 сейчас не запущен. Прежде чем подбирать стратегию, проверь работу сервиса.':'';
      $('v2DomainGuidance').innerHTML=`<div class="v2-result-title">${esc(guideTitle)}</div>`+
        `<div style="margin-top:8px">${esc(guideSummary)}</div>`+
        (runtimeNotice?`<div class="setup-note warn" style="margin-top:10px">${esc(runtimeNotice)}</div>`:'')+
        `<div class="section-kicker" style="margin-top:10px">${esc(guideScope)}</div>`;
      $('v2DomainSummary').innerHTML=[
        ['Classification',detect.classification||'—'],['Diagnostic',diag.code||'—'],
        ['Fault domain',diag.fault_domain||'—'],['Confidence',diag.confidence||'—'],
        ['Strategy relevant',diag.strategy_relevant?'YES':'NO'],['Production',inspect.runtime_running?'RUNNING':'STOPPED'],
        ['Binding',inspect.binding_verdict||'—'],['IPv4',detect.destination_ipv4||'—']
      ].map(([a,b])=>`<div class="v2-card"><span>${esc(a)}</span><strong>${esc(b)}</strong></div>`).join('');
      $('v2DomainStages').innerHTML=['dns','tcp','tls','http'].map(k=>v2StageRow(k,detect.stages?.[k])).join('');
      const lists=inspect.list_matches||[];$('v2DomainLists').innerHTML=lists.length?lists.map(x=>`<div class="v2-row"><div class="v2-result-title">${esc(x.name)}</div><div class="meta">${esc(x.path||'')} · ${esc(x.relation||'present')}</div>${v2Pill('MATCH')}</div>`).join(''):'<div class="analysis-empty">Совпадений в локальных list-файлах не найдено.</div>';
      const profiles=inspect.production_profile_matches||[];$('v2DomainProfiles').innerHTML=profiles.length?profiles.map(p=>`<div class="v2-row"><div class="v2-result-title">Profile ${Number(p.index)}</div><div><div class="value">${esc(p.match_reason||'')}</div><div class="v2-args">${esc(v2ArgsText(p.args))}</div></div>${v2Pill(p.candidate_ready?'READY':'CHECK')}</div>`).join(''):'<div class="analysis-empty">Прямую production profile binding доказать не удалось.</div>';
      const action=String(detect.diagnostic?.suggested_action||'').trim();
      const needsStrategy=siteContext.next_action==='selector'&&!!inspect.runtime_running;
      $('v2DomainToSelector').textContent=String(guide.next_label||'Подобрать стратегию');
      $('v2DomainToSelector').style.display=needsStrategy?'':'none';
      notice(`Проверка ${inspect.target||target}: ${detect.classification_text||detect.classification||'готово'}${action?` · ${action}`:''}`,detect.classification==='clear'?'good':'');
    }catch(e){$('v2DomainState').textContent='ОШИБКА';$('v2DomainGuidance').textContent='Не удалось выполнить проверку: '+e.message;$('v2DomainContext').textContent='Данные о правилах недоступны: '+e.message;notice(`Проверка домена: ${e.message}`,'error')}
    finally{$('v2DomainRun').disabled=false;reportHeight()}
  }
  function v2DomainSend(tab){
    const target=String(v2DomainLast?.inspect?.target||$('v2DomainTarget').value||'').trim();if(!target)return;
    if(tab==='selector-v2')$('v2SelectorTarget').value=target;
    if(tab==='prodlab-v2'){$('v2LabSource').value='manual';$('v2LabTargets').value=target;v2TargetMode('lab')}
    activateNFQWSTab(tab);
  }

  function v2TargetMode(kind){
    const lab=kind==='lab',src=$(lab?'v2LabSource':'v2SandboxTargetSource'),list=$(lab?'v2LabList':'v2SandboxTargetList'),targets=$(lab?'v2LabTargets':'v2SandboxTargets');
    if(!src||!list)return;const fromList=src.value==='list';list.disabled=!fromList;if(targets)targets.disabled=fromList;
  }
  async function v2ResolveLabTargets(){
    await v2LoadTargetSources();const source=$('v2LabSource').value,targets=await v2ResolveTargets(source,$('v2LabList').value,$('v2LabTargets').value);
    $('v2LabTargets').value=targets.join('\n');$('v2LabCount').textContent=targets.length;return targets;
  }
  function renderV2LabResults(rows,done,total){
    let ok=0,fail=0;
    for(const row of rows)for(const a of row.attempts||[])(a.ok?ok++:fail++);
    $('v2LabOK').textContent=ok;$('v2LabFail').textContent=fail;$('v2LabProgress').textContent=total?`${Math.round(done/total*100)}%`:'0%';
    $('v2LabResults').innerHTML=rows.length?rows.map(row=>{
      const good=(row.attempts||[]).filter(x=>x.ok).length,totalA=(row.attempts||[]).length,last=row.attempts?.[row.attempts.length-1];
      return`<div class="v2-row ${good===totalA&&totalA?'good':good?'warn':'bad'}"><div class="v2-result-title">${esc(row.target)}</div><div><div class="value">${good}/${totalA} successful · ${esc(last?.classification_text||last?.classification||'')}</div><div class="meta">binding=${esc(row.inspect?.binding_verdict||'—')} · lists=${row.inspect?.list_matches?.length||0}</div></div>${v2Pill(good===totalA&&totalA?'WORKING':good?'UNSTABLE':'FAILED')}</div>`;
    }).join(''):'<div class="analysis-empty">Нет результатов.</div>';
  }
  async function runV2Lab(){
    if(v2LabAbort)return;let targets;try{targets=await v2ResolveLabTargets()}catch(e){notice(`Target Picker: ${e.message}`,'error');return}
    if(!targets.length)return;
    const rounds=Math.max(1,Math.min(5,Number($('v2LabRounds').value||1))),workers=Math.max(1,Math.min(4,Number($('v2LabConcurrency').value||1)));
    const controller=new AbortController();v2LabAbort=controller;$('v2LabRun').disabled=true;$('v2LabStop').disabled=false;$('v2LabState').textContent='RUNNING';
    const rows=targets.map(x=>({target:x,attempts:[],inspect:null}));let next=0,done=0,total=targets.length*rounds;
    async function worker(){
      while(!controller.signal.aborted){
        const i=next++;if(i>=targets.length)return;const row=rows[i];
        try{row.inspect=await api('/v2/inspect-target',{method:'POST',signal:controller.signal,body:JSON.stringify({target:row.target})})}catch(e){if(controller.signal.aborted)return}
        for(let n=0;n<rounds&&!controller.signal.aborted;n++){
          try{const d=await api('/v2/detect',{method:'POST',signal:controller.signal,body:JSON.stringify({target:row.target})});row.attempts.push({ok:!!d.ok,...d})}
          catch(e){if(controller.signal.aborted)return;row.attempts.push({ok:false,classification:'error',classification_text:e.message})}
          done++;renderV2LabResults(rows,done,total);
        }
      }
    }
    try{await Promise.all(Array.from({length:Math.min(workers,targets.length)},worker));$('v2LabState').textContent=controller.signal.aborted?'STOPPED':'DONE';if(!controller.signal.aborted){const attempts=rows.flatMap(x=>x.attempts||[]),ok=attempts.filter(x=>x.ok).length;v2HistoryAdd('v2-prodlab',{targets:targets.length,attempts:attempts.length,success_ratio:attempts.length?ok/attempts.length:0},rows);notice(`Test Lab: ${targets.length} целей · ${rounds} повторов завершены.`,'good')}}
    finally{v2LabAbort=null;$('v2LabRun').disabled=false;$('v2LabStop').disabled=true;reportHeight()}
  }

  function v2SandboxCandidateMode(){
    const source=$('v2SandboxCandidateSource').value;
    $('v2SandboxProfile').disabled=source!=='production';$('v2SandboxLibrary').disabled=source!=='library';$('v2SandboxArgs').disabled=source==='production'||source==='library';
    if(source==='library'){const item=v2Library.find(x=>x.id===$('v2SandboxLibrary').value);if(item)$('v2SandboxArgs').value=v2ArgsText(item.args)}
    if(source==='production'){const p=v2Profiles.find(x=>Number(x.index)===Number($('v2SandboxProfile').value));if(p)$('v2SandboxArgs').value=v2ArgsText(p.args)}
  }
  async function v2ResolveSandboxTargets(){
    await v2LoadTargetSources();const source=$('v2SandboxTargetSource').value,targets=await v2ResolveTargets(source,$('v2SandboxTargetList').value,$('v2SandboxTargets').value);
    $('v2SandboxTargets').value=targets.join('\n');return targets;
  }
  function renderV2CandidateResult(result,target){
    const c=result?.result||result,attempts=c?.attempts||[],packets=attempts.reduce((n,a)=>n+Number(a.outbound_queue_packets||0)+Number(a.inbound_queue_packets||0),0);
    return`<div class="v2-row ${v2Tone(c?.result_class)}"><div><div class="v2-result-title">${esc(target||result?.target||c?.candidate_name||'candidate')}</div><div class="meta">${esc(c?.candidate_source||result?.source||'')}</div></div><div><div class="value">${esc(c?.result_class||'—')} · success ${Math.round(Number(c?.success_rate||0)*100)}% · complete ${Math.round(Number(c?.complete_rate||0)*100)}%</div><div class="meta">infra=${c?.infrastructure_ok?'PASS':'FAIL'} · cleanup=${c?.cleanup_proven?'PASS':'FAIL'} · packets=${packets} · TTFB=${c?.median_ttfb_ms||'—'} ms</div><div class="v2-args">${esc(v2ArgsText(c?.args||[]))}</div></div>${v2Pill(c?.result_class||'—')}</div>`;
  }
  async function runV2Sandbox(){
    if(v2SandboxAbort)return;let targets;try{targets=await v2ResolveSandboxTargets();await Promise.all([v2LoadProfiles(),v2LoadLibrary(),v2EnsureStatus()])}catch(e){notice(`Sandbox: ${e.message}`,'error');return}
    if(!targets.length){notice('Sandbox: нет целей.','error');return}
    const source=$('v2SandboxCandidateSource').value,attempts=Math.max(1,Math.min(5,Number($('v2SandboxПопытки').value||1))),controller=new AbortController();v2SandboxAbort=controller;
    const common={attempts,expected_config_sha256:current.config_sha256,confirm:'ROUTERFORGE_V2_BENCH'};
    if(source==='production')common.profile_index=Number($('v2SandboxProfile').value);
    else{
      let args=[];
      if(source==='library'){const item=v2Library.find(x=>x.id===$('v2SandboxLibrary').value);args=item?.args||[]}
      else args=v2ArgTokens($('v2SandboxArgs').value);
      if(!args.length){notice('Sandbox: candidate args пусты.','error');v2SandboxAbort=null;return}
      common.args=args;
    }
    $('v2SandboxState').textContent='RUNNING';$('v2SandboxRun').disabled=true;$('v2SandboxStop').disabled=false;$('v2SandboxResults').innerHTML='';
    const results=[];
    try{
      for(const target of targets){
        if(controller.signal.aborted)break;
        try{const d=await api('/v2/bench',{method:'POST',signal:controller.signal,body:JSON.stringify({target,...common})});results.push({target,data:d})}
        catch(e){if(controller.signal.aborted)break;results.push({target,error:e.message})}
        $('v2SandboxResults').innerHTML=results.map(x=>x.error?`<div class="v2-row bad"><div class="v2-result-title">${esc(x.target)}</div><div class="value">${esc(x.error)}</div>${v2Pill('ERROR')}</div>`:renderV2CandidateResult(x.data,x.target)).join('');
      }
      $('v2SandboxState').textContent=controller.signal.aborted?'STOPPED':'DONE';
      if(!controller.signal.aborted){const good=results.filter(x=>x.data?.result?.success_rate>0).length;v2HistoryAdd('v2-sandbox',{targets:results.length,success_ratio:results.length?good/results.length:0},results);notice(`Sandbox: завершено ${results.length}/${targets.length}. Production не менялся.`,'good');}
    }finally{v2SandboxAbort=null;$('v2SandboxRun').disabled=false;$('v2SandboxStop').disabled=true;reportHeight()}
  }
  function v2OpenSandboxFromArgs(argsText,name='Imported candidate',source='custom'){
    activateNFQWSTab('sandbox-v2');$('v2SandboxCandidateSource').value='custom';$('v2SandboxArgs').value=String(argsText||'');$('v2SandboxSaveName').value=String(name||'');$('v2SandboxSeedMeta').textContent=`${source} · imported into real V2 Sandbox`;v2SandboxCandidateMode();requestAnimationFrame(()=>$('v2SandboxTargets')?.focus());
  }
  async function saveV2SandboxCandidate(){
    try{
      await v2LoadLibrary();let args=[],source=$('v2SandboxCandidateSource').value,name=String($('v2SandboxSaveName').value||'').trim()||'Sandbox candidate';
      if(source==='production'){const p=v2Profiles.find(x=>Number(x.index)===Number($('v2SandboxProfile').value));args=p?.args||[];source='custom'}
      else if(source==='library'){const item=v2Library.find(x=>x.id===$('v2SandboxLibrary').value);args=item?.args||[];name=String($('v2SandboxSaveName').value||'').trim()||item?.name||name;source=item?.source||'custom'}
      else args=v2ArgTokens($('v2SandboxArgs').value);
      const d=await api('/v2/strategies/save',{method:'POST',body:JSON.stringify({name,source,args,confirm:'ROUTERFORGE_V2_STRATEGY_SAVE'})});
      v2Library=[];await v2LoadLibrary(true);notice(`Candidate сохранён: ${d.strategy?.name||name}`,'good');
    }catch(e){notice(`Сохранение candidate: ${e.message}`,'error')}
  }

  function renderV2Library(){
    if(!$('v2LibraryList'))return;$('v2LibraryCount').textContent=`${v2Library.length} / 64`;
    $('v2LibraryList').innerHTML=v2Library.length?v2Library.map(x=>`<div class="v2-row"><div><div class="v2-result-title">${esc(x.name)}</div><div class="meta">${esc(x.source)} · ${esc(String(x.fingerprint||'').slice(0,16))}</div></div><div class="v2-args">${esc(v2ArgsText(x.args))}</div><div class="v2-library-actions"><button data-v2-lib-edit="${esc(x.id)}">Редактировать</button><button data-v2-lib-sandbox="${esc(x.id)}">Песочница</button><button class="danger" data-v2-lib-delete="${esc(x.id)}">Удалить</button></div></div>`).join(''):'<div class="analysis-empty">Библиотека стратегий пуста.</div>';
    $('v2LibraryList').querySelectorAll('[data-v2-lib-edit]').forEach(b=>b.onclick=()=>{const x=v2Library.find(y=>y.id===b.dataset.v2LibEdit);if(!x)return;$('v2LibraryID').value=x.id;$('v2LibraryName').value=x.name;$('v2LibrarySource').value=x.source;$('v2LibraryArgs').value=v2ArgsText(x.args)});
    $('v2LibraryList').querySelectorAll('[data-v2-lib-sandbox]').forEach(b=>b.onclick=()=>{const x=v2Library.find(y=>y.id===b.dataset.v2LibSandbox);if(x)v2OpenSandboxFromArgs(v2ArgsText(x.args),x.name,x.source)});
    $('v2LibraryList').querySelectorAll('[data-v2-lib-delete]').forEach(b=>b.onclick=async()=>{const x=v2Library.find(y=>y.id===b.dataset.v2LibDelete);if(!x||!confirm(`Удалить ${x.name} из Библиотека стратегий?`))return;try{await api('/v2/strategies/delete',{method:'POST',body:JSON.stringify({id:x.id,confirm:'ROUTERFORGE_V2_STRATEGY_DELETE'})});v2Library=[];await v2LoadLibrary(true);notice('Candidate удалён.','good')}catch(e){notice(e.message,'error')}});
  }
  async function saveV2LibraryEditor(){
    try{
      const req={id:$('v2LibraryID').value||undefined,name:$('v2LibraryName').value.trim(),source:$('v2LibrarySource').value,args:v2ArgTokens($('v2LibraryArgs').value),confirm:'ROUTERFORGE_V2_STRATEGY_SAVE'};
      const d=await api('/v2/strategies/save',{method:'POST',body:JSON.stringify(req)});v2Library=[];await v2LoadLibrary(true);$('v2LibraryID').value=d.strategy?.id||'';notice(`Библиотека стратегий: ${d.strategy?.name||req.name} сохранён.`,'good');
    }catch(e){notice(`Библиотека стратегий: ${e.message}`,'error')}
  }
  function clearV2LibraryEditor(){$('v2LibraryID').value='';$('v2LibraryName').value='';$('v2LibrarySource').value='custom';$('v2LibraryArgs').value=''}

  function v2ResultText(value){
    const v=String(value||'').toUpperCase();
    const map={WORKING:'Работает',FAILED:'Не работает',FAIL:'Не работает',INCONCLUSIVE:'Не удалось проверить',PARTIAL:'Частично',PASS:'Готово',DONE:'Готово',OK:'Готово'};
    return map[v]||String(value||'—');
  }
  function v2ResultPill(value){const raw=String(value||'—');return`<span class="v2-pill ${v2Tone(raw)}">${esc(v2ResultText(raw))}</span>`}
  function v2SourceText(value){
    const v=String(value||'').toLowerCase();
    const map={production:'Текущая конфигурация',curated:'RouterForge',other:'Встроенный пул',custom:'Пользовательская',library:'Библиотека',community:'Сообщество',baseline:'Без обхода'};
    return map[v]||String(value||'');
  }
  function v2ReasonText(value){
    const v=String(value||'');
    const map={
      'baseline completed reliably; bypass strategy is not required for this target':'Сайт работает без обхода — дополнительная стратегия не требуется.',
      'candidate improved verified HTTP reachability over baseline':'Найдена рабочая стратегия, которая открывает сайт лучше исходного соединения.',
      'no candidate proved a working HTTP response through isolated NFQUEUE':'Среди проверенных стратегий рабочая не найдена.',
      'best candidate did not improve reliability/completeness over baseline':'Проверенные стратегии не дали результата лучше исходного соединения.',
      'no selector recommendation is available':'Рекомендация пока недоступна.',
      'verified candidate matches multiple production profiles; explicit slot selection is required':'Стратегия найдена, но нужно выбрать профиль nfqws2, к которому её применить.',
      'verified candidate has no unique production profile binding for this target':'Стратегия найдена, но для этого домена не определён единственный профиль nfqws2.'
    };
    return map[v]||v;
  }
  function v2PhaseText(value){
    const v=String(value||'').toUpperCase();
    const map={PLAN:'ПОДГОТОВКА',BASELINE:'ИСХОДНОЕ СОЕДИНЕНИЕ',BENCH:'ПРОВЕРКА',RANK:'СРАВНЕНИЕ',FAILED:'ОШИБКА',DONE:'ГОТОВО',IDLE:'ОЖИДАНИЕ'};
    return map[v]||v||'—';
  }
  function v2ProgressText(value){
    const v=String(value||'');
    let m=v.match(/^completed (\d+) of (\d+) candidates(?: · working (\d+))?$/);
    if(m)return`Проверено ${m[1]} из ${m[2]}${m[3]?` · рабочих: ${m[3]}`:''}`;
    if(v==='testing baseline without desync strategy')return'Проверяю сайт без стратегии обхода';
    if(v==='testing direct worker sandboxes')return'Проверяю стратегии';
    if(v==='ranking direct worker live results')return'Сравниваю найденные результаты';
    if(v==='building direct catalog')return'Готовлю список стратегий';
    return v;
  }
  function renderV2SelectorCandidate(c,label){
    const attempts=c?.attempts||[],packets=attempts.reduce((n,a)=>n+Number(a.outbound_queue_packets||0)+Number(a.inbound_queue_packets||0),0);
    const firstError=attempts.map(a=>String(a?.error||'').trim()).find(Boolean)||'';
    const errorHTML=firstError?`<details class="v2-tech-details"><summary>Диагностика</summary><pre>${esc(firstError)}</pre></details>`:'';
    const metrics=[`успех ${Math.round(Number(c?.success_rate||0)*100)}%`,`полный ответ ${Math.round(Number(c?.complete_rate||0)*100)}%`];
    if(c?.median_ttfb_ms)metrics.push(`TTFB ${c.median_ttfb_ms} мс`);
    if(c?.median_throughput_bps)metrics.push(`скорость ${c.median_throughput_bps} Б/с`);
    return`<div class="v2-row ${v2Tone(c?.result_class)}"><div><div class="v2-result-title">${esc(label||c?.candidate_name||c?.candidate_id||'Стратегия')}</div><div class="meta">${esc(v2SourceText(c?.candidate_source||''))}</div></div><div><div class="value">${esc(metrics.join(' · '))}</div><div class="v2-args">${esc(v2ArgsText(c?.args||[]))}</div>${errorHTML}</div>${v2ResultPill(c?.result_class||'—')}</div>`;
  }
  function renderV2SelectorResult(d){
    v2SelectorLast=d;
    $('v2SelectorCleanup').textContent=d.cleanup_baseline_after?'Готово':'Ошибка';
    $('v2SelectorWorkers').textContent=String(d.concurrency||'—');
    const memoryBudget=(d.pool_warnings||[]).find(x=>String(x).startsWith('NF3B1_MEMORY_BUDGET:'));
    if(memoryBudget){
      const wanted=Number($('v2SelectorConcurrency').value||2);
      $('v2SelectorPoolMeta').textContent=`Воркеры: запрошено ${wanted}, запущено ${Number(d.concurrency||0)}. ${String(memoryBudget).replace('NF3B1_MEMORY_BUDGET: ','')}`;
    }
    const confirmNote=(d.pool_warnings||[]).find(x=>String(x).startsWith('NF3B2_CONFIRM:'));
    if(confirmNote)$('v2SelectorPoolMeta').textContent+=' · '+String(confirmNote).replace('NF3B2_CONFIRM: ','Подтверждение: ');
    const communityNote=(d.pool_warnings||[]).find(x=>String(x).startsWith('NF3C_COMMUNITY:'));
    if(communityNote)$('v2SelectorPoolMeta').textContent+=' · '+String(communityNote).replace('NF3C_COMMUNITY: ','Сообщество: ');
    const best=d.recommendation_available?`${d.recommended_candidate_name||d.recommended_candidate_id||('Профиль '+d.recommended_profile_index)} · ${v2SourceText(d.recommended_candidate_source||'production')}`:'Нет рекомендации';
    const poolText=d.auto_pool_enabled?`Проверено стратегий: ${(d.candidates||[]).length}.`:'Встроенный пул отключён.';
    $('v2SelectorRecommendation').innerHTML=`<div class="v2-row ${d.recommendation_available?'good':'warn'}"><div class="v2-result-title">${esc(best)}</div><div><div class="value">${esc(v2ReasonText(d.recommendation_reason||''))}</div><div class="meta">${esc(v2ReasonText(d.apply_gate_reason||''))}</div><div class="meta">${esc(poolText)}</div></div>${v2ResultPill(d.recommendation_available?'WORKING':'—')}</div>`;
    $('v2SelectorResults').innerHTML=renderV2SelectorCandidate(d.baseline,'Без обхода')+(d.candidates||[]).map(c=>renderV2SelectorCandidate(c)).join('');
    const panel=$('v2SelectorApplyPanel');panel.style.display=d.recommendation_available?'block':'none';$('v2SelectorApplyReason').textContent=v2ReasonText(d.apply_gate_reason||'');
    $('v2SelectorPreview').disabled=!d.apply_gate_eligible;$('v2SelectorApply').disabled=true;$('v2SelectorPreviewBody').textContent=d.apply_gate_eligible?'Рабочая стратегия готова. Сначала откройте предпросмотр изменений.':'Для этой стратегии автоматическое применение пока недоступно.';
    v2SelectorPreviewData=null;void prepareV2SelectorBinding(d);reportHeight();
  }
  function v2SelectorProfileQuality(p){
    const reason=String(p?.match_reason||'').toLowerCase();
    if(reason.includes('hostlist-domains='))return{rank:400,key:'exact',label:'ТОЧНЫЙ ДОМЕН'};
    if(/(^|;\s*)include-auto\s/.test(reason))return{rank:320,key:'list',label:'HOSTLIST AUTO'};
    if(/(^|;\s*)include\s/.test(reason))return{rank:300,key:'list',label:'HOSTLIST'};
    if(/(^|;\s*)ipset\s/.test(reason))return{rank:200,key:'ipset',label:'IPSET'};
    if(reason.includes('target is not excluded'))return{rank:50,key:'general',label:'ОБЩИЙ MATCH'};
    if(reason.includes('profile has no target selector'))return{rank:10,key:'general',label:'БЕЗ SELECTOR'};
    return{rank:100,key:'general',label:'MATCH'};
  }
  function v2SelectorProfileReasonText(reason){
    return String(reason||'совпадает с целью').split(';').map(x=>x.trim()).filter(Boolean).map(x=>{
      if(x.startsWith('hostlist-domains='))return`Домен указан прямо в профиле: ${x.slice('hostlist-domains='.length)}`;
      if(x.startsWith('include-auto '))return`Домен найден в автосписке: ${x.slice('include-auto '.length)}`;
      if(x.startsWith('include '))return`Домен найден в hostlist: ${x.slice('include '.length)}`;
      if(x.startsWith('ipset '))return`IP-адрес цели найден в IP-списке: ${x.slice('ipset '.length)}`;
      if(x==='profile has no target selector')return'Общий профиль без ограничения по hostlist/ipset';
      if(x==='target is not excluded by profile filters')return'Цель допускается текущими фильтрами профиля';
      return x;
    }).join(' · ');
  }
  function v2SelectorWorkingCandidates(d){
    return (d?.candidates||[]).filter(c=>c?.result_class==='WORKING'&&Number(c.successes||0)>=1&&c.cleanup_proven&&c.infrastructure_ok);
  }
  function v2SelectorCandidateLabel(c,d){
    const recommended=String(c.candidate_id||'')===String(d?.recommended_candidate_id||'')?' · рекомендуется':'';
    const ttfb=Number(c.median_ttfb_ms||0)>0?` · TTFB ${Number(c.median_ttfb_ms)} мс`:'';
    return`${c.candidate_name||c.candidate_id||'Стратегия'}${ttfb}${recommended}`;
  }
  function renderV2SelectorSelectedStrategy(d){
    const select=$('v2SelectorBindStrategy'),detail=$('v2SelectorBindStrategyDetail');
    const candidates=v2SelectorWorkingCandidates(d),selected=candidates.find(c=>String(c.candidate_id)===String(select.value))||candidates[0];
    if(!selected){detail.innerHTML='<div class="v2-bind-reason">Нет рабочей стратегии для подготовки профиля.</div>';return}
    detail.innerHTML=`<div class="v2-bind-profile-head"><strong>${esc(selected.candidate_name||selected.candidate_id||'Стратегия')}</strong><span class="v2-bind-chip exact">WORKING</span></div><div class="v2-bind-reason"><strong>Источник:</strong> ${esc(v2SourceText(selected.candidate_source||''))} · успех ${Math.round(Number(selected.success_rate||0)*100)}%</div><div class="v2-bind-args">${esc(v2ArgsText(selected.args||[]))}</div>`;
  }
  function renderV2SelectorConflictWarnings(inspect,d){
    const box=$('v2SelectorBindConflicts'),profiles=(inspect?.production_profile_matches||[]).slice().sort((a,b)=>v2SelectorProfileQuality(b).rank-v2SelectorProfileQuality(a).rank||Number(a.index)-Number(b.index));
    if(!profiles.length){
      box.innerHTML=`<div class="v2-bind-profile-head"><strong>Конфликты с текущей конфигурацией</strong><span class="v2-bind-chip exact">НЕ НАЙДЕНЫ</span></div><div class="v2-bind-reason">Для ${esc(d.server_name||'цели')} существующие профили назначения не обнаружены.</div>`;
      return;
    }
    box.innerHTML=`<div class="v2-bind-profile-head"><strong>Возможные пересечения с текущими профилями</strong><span class="v2-bind-chip ipset">${profiles.length}</span></div>${profiles.map(p=>{const q=v2SelectorProfileQuality(p);return`<div class="v2-bind-reason"><strong>Профиль ${Number(p.index)} · ${q.label}:</strong> ${esc(v2SelectorProfileReasonText(p.match_reason))}<div class="v2-bind-args">${esc(v2ArgsText(p.args||[]))}</div></div>`}).join('')}<div class="v2-bind-hint">Это предупреждение, а не замена профилей: новый блок будет добавлен отдельно через <code>--new</code>. Проверьте пересечения target/hostlist/ipset/портов в Preview.</div>`;
  }
  async function prepareV2SelectorBinding(d){
    const panel=$('v2SelectorBindPanel'),select=$('v2SelectorBindStrategy'),button=$('v2SelectorPrepareProfileButton');
    panel.style.display='none';
    if(!d?.recommendation_available||d.apply_gate_eligible||!d.session_id)return;
    const candidates=v2SelectorWorkingCandidates(d);
    if(!candidates.length)return;
    const recommended=String(d.recommended_candidate_id||'');
    candidates.sort((a,b)=>(String(b.candidate_id)===recommended?1:0)-(String(a.candidate_id)===recommended?1:0));
    select.innerHTML=candidates.map(c=>`<option value="${esc(c.candidate_id||'')}">${esc(v2SelectorCandidateLabel(c,d))}</option>`).join('');
    select.onchange=()=>renderV2SelectorSelectedStrategy(d);
    renderV2SelectorSelectedStrategy(d);
    button.disabled=false;panel.style.display='';
    try{
      const inspect=await api('/v2/inspect-target',{method:'POST',body:JSON.stringify({target:d.server_name})});
      renderV2SelectorConflictWarnings(inspect,d);
    }catch(e){
      $('v2SelectorBindConflicts').innerHTML=`<div class="v2-bind-reason">Конфликт-анализ недоступен: ${esc(e.message)}</div>`;
    }
  }
  async function bindV2SelectorRecommendation(){
    const d=v2SelectorLast;if(!d?.session_id||d.apply_gate_eligible)return;
    const candidateID=String($('v2SelectorBindStrategy').value||'').trim();
    if(!candidateID){notice('Выберите рабочую стратегию.','error');return}
    const body={session_id:d.session_id,expected_config_sha256:current.config_sha256,candidate_id:candidateID,confirm:'ROUTERFORGE_V2_SELECTOR_BIND'};
    const button=$('v2SelectorPrepareProfileButton');button.disabled=true;button.textContent='Перепроверяю выбранную стратегию…';
    try{
      const result=await api('/v2/selector-bind',{method:'POST',body:JSON.stringify(body)});
      d.apply_gate_eligible=!!result.apply_gate_eligible;d.apply_gate_token=result.apply_gate_token||'';d.apply_gate_expires_at=result.apply_gate_expires_at||'';d.apply_gate_reason=result.apply_gate_reason||'';
      $('v2SelectorApplyReason').textContent=v2ReasonText(d.apply_gate_reason||'');
      $('v2SelectorPreview').disabled=!d.apply_gate_eligible;$('v2SelectorBindPanel').style.display='none';
      $('v2SelectorPreviewBody').textContent=`Новый профиль подготовлен после повторной проверки (${Number(result.verification_successes||0)}/${Number(result.verification_attempts||0)} успешных). Откройте предпросмотр перед применением.`;
      notice('Выбранная стратегия подтверждена. Новый профиль готов к предпросмотру.','good');
    }catch(e){notice(`Подготовка нового профиля: ${e.message}`,'error')}
    finally{button.disabled=false;button.textContent='Проверить и подготовить новый профиль';reportHeight()}
  }
  function v2SelectorProgressView(p){
    const completed=Number(p?.completed||0),total=Number(p?.total||0),mode=String($('v2SelectorMode')?.value||'normal');
    if(mode==='normal'&&total>0){
      const batchIndex=completed>0?Math.floor((completed-1)/20):0,start=batchIndex*20,size=Math.max(1,Math.min(20,total-start)),done=Math.max(0,Math.min(size,completed-start));
      return{label:`${done} / ${size} · всего ${completed} / ${total}`,pct:Math.round(done/size*100)};
    }
    return{label:`${completed} / ${total}`,pct:total?Math.round(completed/total*100):0};
  }
  async function pollV2Selector(session,signal){
    clearInterval(v2SelectorPoll);
    const startedAt=Date.now();
    const tick=async()=>{
      if(signal.aborted)return;
      try{
        const p=await api(`/v2/selector-progress?session=${encodeURIComponent(session)}`);
        $('v2SelectorPhase').textContent=v2PhaseText(p.phase);const progressView=v2SelectorProgressView(p);$('v2SelectorCompleted').textContent=progressView.label;
        $('v2SelectorProgressText').textContent=`${v2ProgressText(p.message||'')} · ${Math.floor((Date.now()-startedAt)/1000)} с`;const pct=p.phase==='BASELINE'?5:progressView.pct;$('v2SelectorBar').style.width=`${Math.max(0,Math.min(100,pct))}%`;
        if(p.done)clearInterval(v2SelectorPoll);
      }catch(_){}
    };
    await tick();v2SelectorPoll=setInterval(tick,900);
  }
  async function runV2Selector(){
    v2SelectorLastError='';
    if(v2SelectorAbort)return null;const target=$('v2SelectorTarget').value.trim();if(!target){notice('Автоподбор: укажите target.','error');return}
    try{await Promise.all([v2EnsureStatus(),v2LoadLibrary()])}catch(e){notice(e.message,'error');return}
    let propertyVector=null;
    const candidates=[];
    if($('v2SelectorLibrary').checked)for(const x of v2Library)candidates.push({id:x.id,name:x.name,source:x.source,args:x.args});
    const extra=v2ArgTokens($('v2SelectorExtraArgs').value);if(extra.length)candidates.push({name:$('v2SelectorExtraName').value.trim()||'Manual candidate',source:'custom',args:extra});
    if(!$('v2SelectorProduction').checked&&!$('v2SelectorAutoPool').checked&&!candidates.length){notice('Автоподбор: не выбран ни один источник candidates.','error');return}
    if($('v2SelectorCommunity').checked&&!$('v2SelectorAutoPool').checked){notice('Для стратегий сообщества включите встроенный пул.','error');return}
    const asText=$('v2SelectorCommunityASN').value.trim();
    const communityASN=asText?Number(asText):0;
    if(asText&&(!/^\d{1,10}$/.test(asText)||!Number.isInteger(communityASN)||communityASN<1||communityASN>4294967295)){
      notice('Номер AS должен быть целым числом от 1 до 4294967295.','error');return;
    }
    if($('v2SelectorAutoPool').checked){
      const m=String($('v2SelectorMode').value||'normal');
      const breadth=m==='fast'?'Быстрый: 30% пула':m==='normal'?'Обычный: по 20 до двух рабочих':'Полный: весь пул';
      $('v2SelectorPoolMeta').textContent=`${breadth}. Проверка выполняется реальными HTTPS-запросами.`;
    }
    const session=v2SessionID(),controller=new AbortController();v2SelectorAbort=controller;v2SelectorPreviewData=null;$('v2SelectorSession').textContent=session;$('v2SelectorState').textContent='ВЫПОЛНЯЕТСЯ';$('v2SelectorRun').disabled=true;$('v2SelectorStop').disabled=false;$('v2SelectorBar').style.width='0%';$('v2SelectorProgressText').textContent='Подготовка запуска · 0 с';
    void pollV2Selector(session,controller.signal);
    try{
      const d=await api('/v2/selector',{method:'POST',signal:controller.signal,body:JSON.stringify({
        mode:$('v2SelectorMode').value,server_name:target,expected_config_sha256:current.config_sha256,
        concurrency:Number($('v2SelectorConcurrency').value||2),session_id:session,
        include_production:$('v2SelectorProduction').checked,auto_pool:$('v2SelectorAutoPool').checked,candidates,
        community_enabled:$('v2SelectorCommunity').checked,community_asn:communityASN,
        property_vector:propertyVector,
        confirm:'ROUTERFORGE_V2_SELECTOR'
      })});
      $('v2SelectorState').textContent='DONE';$('v2SelectorBar').style.width='100%';renderV2SelectorResult(d);{const all=d.candidates||[],working=all.filter(x=>x.result_class==='WORKING').length;v2HistoryAdd('v2-selector',{targets:1,candidates:all.length,working,success_ratio:all.length?working/all.length:0,recommendation:!!d.recommendation_available},d)}notice(`Автоподбор завершён: ${d.recommendation_reason||'готово'}`,'good');return d;
    }catch(e){
      if(controller.signal.aborted){$('v2SelectorState').textContent='ОСТАНОВЛЕНО';notice('Автоподбор остановлен.','')}
      else{v2SelectorLastError=e.message||'selector failed';$('v2SelectorState').textContent='ОШИБКА';if(!v2AutoSetupRunning)notice(`Автоподбор: ${e.message}`,'error')}
      return null;
    }finally{clearInterval(v2SelectorPoll);v2SelectorPoll=0;v2SelectorAbort=null;$('v2SelectorRun').disabled=false;$('v2SelectorStop').disabled=true;reportHeight()}
  }
  async function buildV2SelectorPreview(){
    v2SelectorPreviewError='';
    const d=v2SelectorLast;if(!d?.apply_gate_eligible||!d.apply_gate_token)return null;
    $('v2SelectorPreview').disabled=true;
    try{
      const p=await api('/bench-autotune/apply-preview',{method:'POST',body:JSON.stringify({token:d.apply_gate_token,expected_config_sha256:current.config_sha256,confirm:'ROUTERFORGE_AUTOTUNE_PREVIEW'})});
      if(!p.ok||!p.preview_only)throw new Error('preview contract mismatch');v2SelectorPreviewData=p;
      const meta=[`Источник: ${v2SourceText(p.candidate_source||'production')}`,`Транспорт: ${p.transport||'https'}`,`Сессия: ${p.session_id||'—'}`,`Отпечаток: ${p.candidate_fingerprint||'—'}`,`Результат: ${v2ResultText(p.live_result_class||'—')} / ${Number(p.live_success_rate||0)*100}%`,`Ресурсы: ${(p.required_resources||[]).join(', ')||'нет'}`,`Применение: ${p.reload_method||'Smart Apply'}`].join('\n');
      const placement=p.placement_kind==='existing_list'?`\n\nСуществующий список: ${p.placement_list}\nБыло SHA: ${p.placement_list_sha_before}\nБудет SHA: ${p.placement_list_sha_after}\nИзменение: ${p.placement_diff}\nКонфигурация nfqws2 остаётся прежней.`:'';
      $('v2SelectorPreviewBody').innerHTML=`${esc(meta+placement)}\n\n${highlightConfig(p.candidate_config||'')}`;
      $('v2SelectorApply').disabled=!p.changed;notice(`Предпросмотр готов · ${String(p.candidate_config_sha256||'').slice(0,16)}…`,'good');return p;
    }catch(e){v2SelectorPreviewError=e.message||'preview failed';v2SelectorPreviewData=null;notice(`Предпросмотр: ${e.message}`,'error');return null}
    finally{$('v2SelectorPreview').disabled=false}
  }
  async function applyV2SelectorPreview(){
    v2SelectorApplyError='';
    const d=v2SelectorLast,p=v2SelectorPreviewData;if(!d?.apply_gate_token||!p?.changed)return null;
    if(!confirm(`Применить найденную стратегию?\nИзменение: ${p.placement_kind==='existing_list'?'добавление сайта в '+p.placement_list:'профиль nfqws2'}\nДомен: ${p.server_name}\nИсточник: ${v2SourceText(p.candidate_source||'production')}\nТранспорт: ${p.transport||'https'}\nОтпечаток: ${p.candidate_fingerprint||'—'}\nТекущая конфигурация: ${p.active_config_sha256}\nНовая конфигурация: ${p.candidate_config_sha256}\n\nПеред изменением будет создана резервная копия и выполнена проверка результата.`))return null;
    $('v2SelectorApply').disabled=true;
    try{
      const latest=await api('/status');if(String(latest.config_sha256).toLowerCase()!==String(p.active_config_sha256).toLowerCase())throw new Error('active config changed after preview');
      const result=await api('/bench-autotune/apply',{method:'POST',body:JSON.stringify({receipt_token:d.apply_gate_token,expected_active_sha256:p.active_config_sha256,expected_candidate_sha256:p.candidate_config_sha256,confirm:'ROUTERFORGE_AUTOTUNE_APPLY'})});
      if(!result.post_apply_verified)throw new Error('После применения не подтверждён ответ сайта через production.');current=result.status||await api('/status');original=current?.config||original;render(current);v2SelectorLast=null;v2SelectorPreviewData=null;$('v2SelectorApply').disabled=true;notice(`Применено: сайт повторно открылся через nfqws2. Проверено других сайтов: ${Number(result.regression_guards_verified||0)}.`, 'good');return result;
    }catch(e){v2SelectorApplyError=e.message||'apply failed';v2SelectorLast=null;v2SelectorPreviewData=null;notice(`Safe Apply: ${e.message}`,'error');return null}
  }


  function v2HistoryAdd(kind,metrics,payload){
    if(!Array.isArray(runHistory))return;
    const successRatio=Number(metrics?.success_ratio||0),score=Math.round(Math.max(0,Math.min(1,successRatio))*100);
    const entry={id:`${Date.now()}-${kind}`,kind,created_at:new Date().toISOString(),active_config_sha256:current?.config_sha256||'',active_mode:currentMode(),runtime_running:!!current?.running,score,score_type:'SUCCESS%',metrics:metrics||{},scope:'nfqws-v2-verified-workflow',payload};
    runHistory.unshift(entry);runHistory=runHistory.slice(0,runHistoryMax);runHistoryPersist();renderRunHistory();
  }



  let v2TCP16MemoryData=null;

  function renderTCP16MemoryV1(){
    const root=$('v2TCP16List');if(!root)return;
    const data=v2TCP16MemoryData;
    if(!data){root.innerHTML='<div class="analysis-empty">Память ещё не загружена.</div>';return}
    const entries=data.entries||[];
    if(!entries.length){root.innerHTML='<div class="analysis-empty">Наблюдений пока нет. «Разбор домена» начнёт заполнять память автоматически.</div>';return}
    root.innerHTML=entries.map(x=>{
      const state=x.cutoff_16k_suspected?'TCP16 SUSPECTED':'CLEAR';
      const cls=x.cutoff_16k_suspected?'bad':'good';
      const sni=x.working_sni||'—';
      const network=x.asn?`AS${x.asn} · ${x.provider||''}`:(x.cidr||x.network_key||'—');
      return `<div class="tcp16-memory-row">
        <div class="tcp16-memory-main"><strong>${esc(network)}</strong><span>${esc(x.last_target||'—')} · ${esc(x.destination_ipv4||'—')}</span></div>
        <div class="tcp16-memory-state ${cls}"><strong>${esc(state)}</strong><span>${esc(x.confidence||'NEW')} · suspected ${Number(x.suspected_count||0)} · clear ${Number(x.clear_count||0)}</span></div>
        <div class="tcp16-memory-sni"><span>Working SNI</span><strong>${esc(sni)}</strong></div>
        <div class="tcp16-memory-time"><span>Последнее наблюдение</span><strong>${esc(x.last_observed||'—')}</strong></div>
      </div>`;
    }).join('');
  }

  async function loadTCP16MemoryV1(force=false){
    if(v2TCP16MemoryData&&!force){renderTCP16MemoryV1();return}
    $('v2TCP16State').textContent='LOADING';
    try{
      const data=await api('/v2/tcp16-memory');
      v2TCP16MemoryData=data;
      $('v2TCP16State').textContent='READ ONLY';
      $('v2TCP16Count').textContent=String(data.count||0);
      $('v2TCP16TabCount').textContent=String(data.count||0);
      $('v2TCP16Suspected').textContent=String(data.suspected||0);
      $('v2TCP16Clear').textContent=String(data.clear||0);
      $('v2TCP16SNI').textContent=String(data.working_sni||0);
      renderTCP16MemoryV1();
    }catch(e){
      $('v2TCP16State').textContent='ОШИБКА';
      $('v2TCP16List').innerHTML=`<div class="notice error">${esc(e.message)}</div>`;
    }
    reportHeight();
  }

  let v2RegistryData=null;

  function v2RegistryPct(value){
    const n=Number(value||0);
    return `${Math.round(Math.max(0,Math.min(1,n))*100)}%`;
  }

  function v2RegistryRender(){
    const root=$('v2RegistryList');if(!root)return;
    const data=v2RegistryData;
    if(!data){root.innerHTML='<div class="analysis-empty">Реестр ещё не загружен.</div>';return}
    const query=String($('v2RegistrySearch')?.value||'').trim().toLowerCase();
    const protocol=String($('v2RegistryProtocol')?.value||'');
    const source=String($('v2RegistrySource')?.value||'');
    const entries=(data.entries||[]).filter(item=>{
      if(protocol&&item.protocol!==protocol)return false;
      if(source&&!(item.sources||[]).includes(source))return false;
      if(!query)return true;
      return [item.name,item.family,item.protocol,(item.sources||[]).join(' '),(item.aliases||[]).join(' '),(item.args||[]).join(' ')].join(' ').toLowerCase().includes(query);
    });
    if(!entries.length){root.innerHTML='<div class="analysis-empty">По текущему фильтру стратегий нет.</div>';return}
    root.innerHTML=entries.map(item=>{
      const e=item.evidence||{};
      const confidence=e.confidence||'—';
      const evidence=e.verified_count?`${e.targets||0} target · ${e.verified_count} verify · ${v2RegistryPct(e.success_rate)} · ${confidence}`:'нет verified evidence';
      const aliases=(item.aliases||[]).length?`<div class="registry-aliases">aliases: ${esc(item.aliases.join(' · '))}</div>`:'';
      const provenance=(item.provenance||[]).map(p=>`${p.source||p.kind}${p.repository?' · '+p.repository:''}${p.ref?' @ '+String(p.ref).slice(0,12):''}`).join(' | ');
      const caps=item.capabilities||{},bench=(caps.bench_transports||[]);
      const capability=bench.length?`bench: ${bench.join(', ')} · desync ${Number(caps.desync_count||0)}`:'bench: not eligible';
      const score=item.score?.total;
      const scoreText=Number.isFinite(Number(score))?`score ${Number(score)}/100`:'score —';
      const insight=item.insight||{},insightState=insight.state||'—',insightSummary=insight.summary||'Нет insight.';
      const automation=item.automation||{},automationText=automation.eligible?'AUTOMATION GATE READY':`AUTOMATION BLOCKED${(automation.reasons||[]).length?' · '+automation.reasons.join(', '):''}`;
      const recommendation=item.recommendation||null,recommendationText=recommendation?(recommendation.primary?`HISTORICAL TOP #${recommendation.rank}`:`HISTORICAL #${recommendation.rank}`):'';
      return `<div class="registry-row">
        <div class="registry-main"><strong>${esc(item.name||item.id)}</strong>${aliases}<span class="registry-meta">${esc((item.sources||[]).join(' + '))} · ${esc(item.protocol||'—')} · ${esc(item.family||'—')}</span><span class="registry-meta">${esc(capability)} · ${esc(scoreText)}</span><span class="registry-insight ${esc(String(insightState).toLowerCase())}">${esc(insightState)} · ${esc(insightSummary)}</span><span class="registry-automation ${automation.eligible?'ready':'blocked'}">${esc(automationText)}</span>${recommendation?`<span class="registry-recommendation ${recommendation.primary?'primary':'hint'}">${esc(recommendationText)} · ${esc(recommendation.reason||'')}</span>`:''}</div>
        <div class="registry-evidence"><strong>${esc(evidence)}</strong><span>${esc(provenance||'local')}</span></div>
        <details class="registry-args"><summary>Args · ${esc(String((item.fingerprint||'').slice(0,12)))}</summary><pre>${esc((item.args||[]).join('\n'))}</pre></details>
      </div>`;
    }).join('');
  }

  function v2RegistryFillSelect(id,values,label){
    const node=$(id);if(!node)return;
    const current=node.value;
    node.innerHTML=`<option value="">${esc(label)}</option>`+(values||[]).map(v=>`<option value="${esc(v)}">${esc(v)}</option>`).join('');
    if((values||[]).includes(current))node.value=current;
  }

  async function loadStrategyRegistryV1(force=false){
    if(v2RegistryData&&!force){v2RegistryRender();return}
    $('v2RegistryState').textContent='LOADING';
    try{
      const [data,scores,insights,automation,recommendations]=await Promise.all([api('/v2/strategy-registry'),api('/v2/strategy-scores'),api('/v2/strategy-insights'),api('/v2/policy-automation-gate'),api('/v2/strategy-recommendations')]);
      const scoreMap=new Map((scores.entries||[]).map(x=>[x.fingerprint,x.score]));
      const insightMap=new Map((insights.entries||[]).map(x=>[x.fingerprint,x.insight]));
      const automationMap=new Map((automation.candidates||[]).map(x=>[x.fingerprint,x]));
      const recommendationMap=new Map((recommendations.items||[]).map(x=>[x.fingerprint,x]));
      (data.entries||[]).forEach(x=>{x.score=scoreMap.get(x.fingerprint)||null;x.insight=insightMap.get(x.fingerprint)||null;x.automation=automationMap.get(x.fingerprint)||null;x.recommendation=recommendationMap.get(x.fingerprint)||null});
      data.automation_gate=automation;
      data.recommendations=recommendations;
      v2RegistryData=data;
      $('v2RegistryState').textContent='READ ONLY';
      $('v2RegistryCount').textContent=String(data.count||0);
      $('v2RegistryTabCount').textContent=String(data.count||0);
      $('v2RegistrySourcesCount').textContent=String((data.sources||[]).length);
      $('v2RegistryProtocolsCount').textContent=String((data.protocols||[]).length);
      $('v2RegistryEvidenceCount').textContent=String((data.entries||[]).filter(x=>Number(x?.evidence?.verified_count||0)>0).length);
      $('v2RegistryAutomationReady').textContent=String(Number(data.automation_gate?.ready_count||0));
      $('v2RegistryRecommendationTop').textContent=String(Number(data.recommendations?.primary_count||0));
      v2RegistryFillSelect('v2RegistryProtocol',data.protocols||[],'Все протоколы');
      v2RegistryFillSelect('v2RegistrySource',data.sources||[],'Все источники');
      v2RegistryRender();
    }catch(e){
      $('v2RegistryState').textContent='ОШИБКА';
      $('v2RegistryList').innerHTML=`<div class="notice error">${esc(e.message)}</div>`;
    }
    reportHeight();
  }


  let v2ObservedData=null;
  function v2ObservedTargetValue(x){return String(x?.target||x?.destination||'').trim()}
  function v2ObservedCanDomain(x){const v=v2ObservedTargetValue(x);return !!v&&!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(v)}
  function v2ObservedReasonText(reason){
    const map={tcp_syn_no_reply:'TCP SYN без SYN/ACK',tcp_unreplied:'TCP без ответа',tcp_rst_or_early_close:'TCP RST / раннее закрытие',udp_no_reply:'UDP egress без ответа'};
    return map[String(reason||'')]||String(reason||'suspected failure');
  }
  function renderV2Observed(data){
    v2ObservedData=data||{};
    const rows=Array.isArray(data?.observed)?data.observed:[];
    $('v2ObservedTabCount').textContent=String(rows.length);
    $('v2ObservedMeta').textContent=`visible=${rows.length} · suppressed=${Number(data?.suppressed_count||0)} · ignored=${Number(data?.ignored_count||0)} · min events=${Number(data?.min_events||2)} · source=${data?.flow_source||'stored evidence'}`;
    $('v2ObservedList').innerHTML=rows.length?rows.map((x,i)=>{
      const target=v2ObservedTargetValue(x),domain=v2ObservedCanDomain(x);
      return `<div class="v2-row ${x.active?'warn':'info'}"><div><div class="v2-result-title">${esc(target||x.destination||'—')}</div><div class="meta">device=${esc(x.device||'—')} · ${esc(x.protocol||'')} / ${Number(x.port||0)} · ${esc(v2ObservedReasonText(x.reason))} · count=${Number(x.count||0)} · last=${esc(x.last_seen||'—')}</div></div><div class="toolbar"><button data-observed-action="setup" data-index="${i}" ${domain?'':'disabled'}>Auto Setup</button><button data-observed-action="diagnose" data-index="${i}" ${domain?'':'disabled'}>Diagnose</button><button data-observed-action="sandbox" data-index="${i}" ${domain?'':'disabled'}>Песочница</button><button data-observed-action="selector" data-index="${i}" ${domain?'':'disabled'}>Auto Select</button><button data-observed-action="list" data-index="${i}" ${domain?'':'disabled'}>В user.list</button><button data-observed-action="ignore" data-index="${i}">Ignore</button></div>${v2Pill(x.active?'ACTIVE':'SEEN')}</div>`;
    }).join(''):`<div class="analysis-empty">Повторных suspected failures пока нет. Для отсечения случайного шума первое событие скрывается; повторите scan позже, если проблема сохраняется.</div>`;
    $('v2ObservedList').querySelectorAll('[data-observed-action]').forEach(btn=>btn.onclick=()=>v2ObservedAction(btn.dataset.observedAction,Number(btn.dataset.index)));
    reportHeight();
  }
  async function loadV2Observed(){
    try{$('v2ObservedState').textContent='READ ONLY';renderV2Observed(await api('/v2/observed-targets'))}
    catch(e){$('v2ObservedState').textContent='ОШИБКА';$('v2ObservedList').innerHTML=`<div class="notice error">${esc(e.message)}</div>`}
  }
  async function scanV2Observed(){
    $('v2ObservedScan').disabled=true;$('v2ObservedState').textContent='SCANNING';
    try{const d=await api('/v2/observed-targets/scan',{method:'POST',body:JSON.stringify({confirm:'ROUTERFORGE_OBSERVED_SCAN'})});$('v2ObservedState').textContent='READY';renderV2Observed(d);notice(`Observed: ${Number(d.count||0)} visible · ${Number(d.suppressed_count||0)} suppressed.`,'good')}
    catch(e){$('v2ObservedState').textContent='ОШИБКА';notice(`Observed scan: ${e.message}`,'error')}
    finally{$('v2ObservedScan').disabled=false;reportHeight()}
  }
  async function v2ObservedAction(action,index){
    const x=v2ObservedData?.observed?.[index];if(!x)return;const target=v2ObservedTargetValue(x);
    if(action==='setup'&&v2ObservedCanDomain(x)){v2AutoSetupSeed(target,'Observed Targets');return}
    if(action==='diagnose'&&v2ObservedCanDomain(x)){$('v2DomainTarget').value=target;activateNFQWSTab('domain-v2');return}
    if(action==='selector'&&v2ObservedCanDomain(x)){$('v2SelectorTarget').value=target;activateNFQWSTab('selector-v2');return}
    if(action==='sandbox'&&v2ObservedCanDomain(x)){await v2LoadTargetSources();$('v2SandboxTargetSource').value='manual';$('v2SandboxTargets').value=target;v2TargetMode('sandbox');activateNFQWSTab('sandbox-v2');return}
    if(action==='ignore'){try{await api('/v2/observed-targets/ignore',{method:'POST',body:JSON.stringify({target,ignore:true,confirm:'ROUTERFORGE_OBSERVED_IGNORE'})});await loadV2Observed();notice(`Observed: ${target} скрыт.`,'good')}catch(e){notice(e.message,'error')}return}
    if(action==='list'&&v2ObservedCanDomain(x)){
      try{
        const f=await api('/list?name=user.list'),lines=String(f.content||'').split(/\r?\n/),normalized=target.toLowerCase();
        if(lines.some(line=>line.trim().toLowerCase()===normalized)){notice(`${target} уже есть в user.list.`,'good');return}
        const content=String(f.content||'').replace(/\s*$/,'\n')+target+'\n';
        await api('/list/save',{method:'POST',body:JSON.stringify({name:'user.list',content,expected_sha256:f.sha256,confirm:'NFQWS_LIST_SAVE'})});
        notice(`${target} добавлен в user.list через backup + atomic save.`,'good');
      }catch(e){notice(`user.list: ${e.message}`,'error')}
    }
  }

  async function openV2Workspace(tab){
    if(tab==='domain-v2')return;
    if(tab==='auto-setup-v1'){await Promise.all([v2LoadTargetSources(),v2AutoSetupLoadObserved().catch(()=>null)]);await v2AutoSetupSourceChanged();return}
    if(tab==='prodlab-v2'){await v2LoadTargetSources();v2TargetMode('lab');return}
    if(tab==='sandbox-v2'){await Promise.all([v2LoadTargetSources(),v2LoadProfiles(),v2LoadLibrary()]);v2TargetMode('sandbox');v2SandboxCandidateMode();return}
    if(tab==='selector-v2'){await Promise.all([v2LoadLibrary(),v2EnsureStatus()]);return}
    if(tab==='strategy-library-v2'){await v2LoadLibrary(true);return}
  }


  $('v2TCP16Refresh').onclick=()=>loadTCP16MemoryV1(true);
  $('v2ObservedLoad').onclick=()=>loadV2Observed();
  $('v2ObservedScan').onclick=()=>scanV2Observed();
  $('v2RegistryRefresh').onclick=()=>loadStrategyRegistryV1(true);
  $('v2RegistrySearch').oninput=v2RegistryRender;
  $('v2RegistryProtocol').onchange=v2RegistryRender;
  $('v2RegistrySource').onchange=v2RegistryRender;

  // New workflow event bindings.
  $('v2AutoSetupSource').onchange=v2AutoSetupSourceChanged;$('v2AutoSetupList').onchange=v2AutoSetupLoadListTargets;$('v2AutoSetupRun').onclick=runV2AutoSetup;$('v2AutoSetupReset').onclick=v2AutoSetupReset;$('v2AutoSetupRebuildPreview').onclick=v2AutoSetupBuildPreview;$('v2AutoSetupApply').onclick=applyV2AutoSetup;
  $('v2DomainRun').onclick=runV2Domain;$('v2DomainTarget').onkeydown=e=>{if(e.key==='Enter')runV2Domain()};$('v2DomainToSelector').onclick=()=>v2DomainSend('selector-v2');
  $('v2LabSource').onchange=()=>v2TargetMode('lab');$('v2LabResolve').onclick=async()=>{try{await v2ResolveLabTargets()}catch(e){notice(e.message,'error')}};$('v2LabRun').onclick=runV2Lab;$('v2LabStop').onclick=()=>v2LabAbort?.abort();
  $('v2SandboxTargetSource').onchange=()=>v2TargetMode('sandbox');$('v2SandboxCandidateSource').onchange=v2SandboxCandidateMode;$('v2SandboxProfile').onchange=v2SandboxCandidateMode;$('v2SandboxLibrary').onchange=v2SandboxCandidateMode;$('v2SandboxRun').onclick=runV2Sandbox;$('v2SandboxStop').onclick=()=>v2SandboxAbort?.abort();$('v2SandboxSave').onclick=saveV2SandboxCandidate;
  $('v2SelectorRun').onclick=runV2Selector;$('v2SelectorStop').onclick=()=>v2SelectorAbort?.abort();$('v2SelectorPreview').onclick=buildV2SelectorPreview;$('v2SelectorApply').onclick=applyV2SelectorPreview;$('v2SelectorPrepareProfileButton').onclick=bindV2SelectorRecommendation;
  $('v2LibraryRefresh').onclick=()=>{v2Library=[];v2LoadLibrary(true)};$('v2LibrarySave').onclick=saveV2LibraryEditor;$('v2LibraryClear').onclick=clearV2LibraryEditor;$('v2LibraryToSandbox').onclick=()=>v2OpenSandboxFromArgs($('v2LibraryArgs').value,$('v2LibraryName').value||'Library editor',$('v2LibrarySource').value);

  if($('uxTaskAutoV2'))$('uxTaskAutoV2').onclick=()=>openOverviewTask('selector-v2','v2SelectorTarget');


  function handleEditorSaveShortcut(e){
    if(!(e.ctrlKey||e.metaKey)||String(e.key||'').toLowerCase()!=='s')return;
    const active=document.querySelector('.view.active'),view=active?.id||'';
    if(!['view-strategies','view-configs','view-config','view-lists','view-scripts'].includes(view))return;
    e.preventDefault();e.stopPropagation();
    if(view==='view-strategies'){updateFlowState();if(flowDirty())previewFlow();return}
    if(view==='view-configs'){if(selectedStoredConfig&&$('storedConfigEditor').value!==storedConfigOriginal)showStoredConfigDiff();return}
    if(view==='view-config'){if($('config').value!==original)showDiff();return}
    if(view==='view-lists'){if(listMeta&&$('listEditor').value!==listOriginal)void saveList();return}
  }

  const nfqwsConfigTabs=new Set(['strategies','configs','config','lists','geo-assets']);
  const nfqwsStrategyTabs=new Set(['strategy-library-v2','strategy-registry-v1','strategy-import']);
  const nfqwsMenuTabs=new Set(['catalog','upstream-lists','blobs']);
  const nfqwsSelectionTabs=new Set(['auto-setup-v1','selector-v2','sandbox-v2','target-memory-v2']);
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
    document.dispatchEvent(new CustomEvent('routerforge:nfqws:tab',{detail:{tab}}));
    const configStrip=$('nfqwsConfigTabs'),strategyStrip=$('nfqwsStrategyTabs'),menuStrip=$('nfqwsMenuTabs'),menuCredit=$('nfqwsMenuCredit'),toolStrip=$('nfqwsToolTabs'),selectionStrip=$('nfqwsSelectionTabs'),maintenanceStrip=$('nfqwsMaintenanceTabs');
    if(configStrip)configStrip.classList.toggle('visible',isConfig);
    if(strategyStrip)strategyStrip.classList.toggle('visible',isStrategy);
    if(menuStrip)menuStrip.classList.toggle('visible',isMenu);
    if(menuCredit)menuCredit.classList.toggle('visible',isMenu);
    if(toolStrip)toolStrip.classList.toggle('visible',isTool);
    if(selectionStrip)selectionStrip.classList.toggle('visible',isSelection);

    if(maintenanceStrip)maintenanceStrip.classList.toggle('visible',isMaintenance);
    document.querySelectorAll('.view').forEach(x=>x.classList.toggle('active',x.id===`view-${tab}`));
    if(tab==='scripts')loadScripts();
    if(tab==='upstream-lists')loadListSources();

    if(tab==='clienthello'){loadBlobs();loadClientHelloCapabilities();}
    if(tab==='preflight')renderPreflight();
    if(tab==='dpi-detector')loadDPIDetector(true);
    if(tab==='observed-v1')void loadV2Observed();
    if(tab==='runs'){renderRunHistory();renderAutoPick()}
    if(tab==='catalog')loadCatalog();
    if(tab==='blobs')loadBlobs();
    if(tab==='backups')loadBackupCenter();
    if(tab==='target-memory-v2')void loadV2TargetMemory();
    if(tab==='runtime-health-v2')void loadRuntimeHealthV2();
    if(tab==='keenetic'){loadKeenetic();loadDPIDetector()}
    if(['domain-v2','prodlab-v2','auto-setup-v1','selector-v2','sandbox-v2','strategy-library-v2'].includes(tab))void openV2Workspace(tab);
    if(tab==='strategy-registry-v1')void loadStrategyRegistryV1();
    if(tab==='tcp16-memory-v1')void loadTCP16MemoryV1();


    if(tab==='configs'){loadConfigLibrary();requestAnimationFrame(()=>{if(selectedStoredConfig){mountStoredConfigSyntaxEditor();updateStoredConfigEditor()}reportHeight()})}
    if(tab==='config')requestAnimationFrame(()=>{resizeRawEditor();refreshSyntaxEditor($('config'));reportHeight()});
    else requestAnimationFrame(reportHeight);
  }

  document.querySelectorAll('#nfqwsPrimaryTabs .tab').forEach(btn=>btn.onclick=()=>{
    if(btn.dataset.primaryTab==='configuration')activateNFQWSTab(nfqwsActiveConfig);
    else if(btn.dataset.primaryTab==='strategies')activateNFQWSTab(nfqwsActiveStrategy);
    else if(btn.dataset.primaryTab==='menu')activateNFQWSTab(nfqwsActiveMenu);
    else if(btn.dataset.primaryTab==='tools')activateNFQWSTab(nfqwsActiveTool);
    else if(btn.dataset.primaryTab==='selection')activateNFQWSTab(nfqwsActiveSelection);
    else if(btn.dataset.primaryTab==='maintenance')activateNFQWSTab(nfqwsActiveMaintenance);
    else activateNFQWSTab(btn.dataset.tab);
  });
  document.querySelectorAll('#nfqwsConfigTabs .tab').forEach(btn=>btn.onclick=()=>activateNFQWSTab(btn.dataset.tab));
  document.querySelectorAll('#nfqwsStrategyTabs .tab').forEach(btn=>btn.onclick=()=>activateNFQWSTab(btn.dataset.tab));
  document.querySelectorAll('#nfqwsMenuTabs .tab').forEach(btn=>btn.onclick=()=>activateNFQWSTab(btn.dataset.tab));
  document.querySelectorAll('#nfqwsToolTabs .tab').forEach(btn=>btn.onclick=()=>activateNFQWSTab(btn.dataset.tab));
  document.querySelectorAll('#nfqwsSelectionTabs .tab').forEach(btn=>btn.onclick=()=>activateNFQWSTab(btn.dataset.tab));
  document.querySelectorAll('#nfqwsMaintenanceTabs .tab').forEach(btn=>btn.onclick=()=>activateNFQWSTab(btn.dataset.tab));
  $('v2MemoryRefresh').onclick=loadV2TargetMemory;
  $('v2MemoryFilter').onkeydown=e=>{if(e.key==='Enter')loadV2TargetMemory()};
  $('v2MemoryClearTarget').onclick=()=>clearV2TargetMemory(false);
  $('v2MemoryClearAll').onclick=()=>clearV2TargetMemory(true);
  $('runtimeHealthRefresh').onclick=loadRuntimeHealthV2;
  $('pruneBackups').onclick=pruneBackupCenter;

  function syncDPIConsoleFullscreen(){
    const button=$('dpiConsoleFullscreen'),pane=$('dpiConsolePane');if(!button||!pane)return;
    button.textContent=pane.classList.contains('dpi-console-overlay')?'Закрыть полный экран':'На весь экран';
  }
  async function toggleDPIConsoleFullscreen(){
    const pane=$('dpiConsolePane');if(!pane)return;
    const on=!pane.classList.contains('dpi-console-overlay');
    pane.classList.toggle('dpi-console-overlay',on);
    document.body.classList.toggle('dpi-console-overlay-open',on);
    syncDPIConsoleFullscreen();
  }
  function dpiParityStripANSI(value){
    return String(value||'').replace(/\x1b\[[0-?]*[ -/]*[@-~]/g,'').replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g,'');
  }
  function dpiParityTokens(line){
    let html=esc(line);
    html=html.replace(/\b((?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,})\b/gi,'<span class="dpi-tui-cyan">$1</span>');
    html=html.replace(/\b(AS\d+☆?)\b/g,'<span class="dpi-tui-yellow">$1</span>');
    html=html.replace(/\b(DC[1-5]|DoH|DoT|UDP|HTTP|TLS1\.2|TLS1\.3)\b/g,'<span class="dpi-tui-cyan">$1</span>');
    html=html.replace(/(✓|\bOK\b|\bAVAILABLE\b|\breachable\b|\bUP\b)/gi,'<span class="dpi-tui-green">$1</span>');
    html=html.replace(/(×|\bDETECTED\b|\bTIMEOUT\b|\bBLOCKED\b|\bREFUSED\b|TLS DROP|TLS RST|TLS EOF|TLS ALERT|SYN DROP|TCP RST|недоступен|заблокирован)/gi,'<span class="dpi-tui-red">$1</span>');
    html=html.replace(/(\b\d+\/\d+\b)/g,'<span class="dpi-tui-yellow">$1</span>');
    return html;
  }
  function dpiParityReportHTML(report){
    const text=dpiParityStripANSI(report).replace(/\r\n?/g,'\n');
    return text.split('\n').map(line=>{
      const trimmed=line.trim();
      const warn=/^\[!?\]|ВНИМАНИЕ|Частично|перехватывает/i.test(trimmed);
      const title=/^(Проверка|Результаты|Поиск белых SNI|Итог|Информация о сети|Fingerprint)/i.test(trimmed);
      return `<div class="dpi-console-line${warn?' warn':''}${title?' title':''}">${dpiParityTokens(line)||' '}</div>`;
    }).join('');
  }
  function dpiNativeElapsed(){
    if(!dpiNativeStartedAt)return 0;
    return Math.max(0,Math.floor((Date.now()-dpiNativeStartedAt)/1000));
  }
  function dpiScheduleLiveRender(){
    if(dpiLiveRenderPending)return;
    dpiLiveRenderPending=true;
    requestAnimationFrame(()=>{dpiLiveRenderPending=false;renderDPINativeReport(dpiNativeResult)});
  }
  function renderDPINativeReport(envelope){
    const root=$('dpiNativeResults');if(!root)return;
    root.style.display='block';
    const text=dpiLiveText||String(envelope?.console_report||'');
    const running=!!dpiWebRunning;
    const elapsed=dpiNativeElapsed();
    if(!root.querySelector('.dpi-native-live-shell')){
      root.innerHTML=`<article class="panel dpi-native-live-shell"><div class="dpi-native-live-head"><div><strong>DPI Detector · Native Live</strong><span>официальный upstream output · RouterForge presentation</span></div><div class="dpi-native-live-state"><span id="dpiNativeLiveBadge" class="config-badge">IDLE</span><span id="dpiNativeLiveMeta" class="mono">—</span></div></div><div id="dpiNativeLiveBody" class="dpi-native-live-body"><div class="analysis-empty">Выберите тесты и запустите DPI Detector.</div></div><div id="dpiNativeLiveFooter" class="dpi-native-live-footer">Ожидание запуска.</div></article>`;
    }
    const badge=$('dpiNativeLiveBadge'),meta=$('dpiNativeLiveMeta'),body=$('dpiNativeLiveBody'),footer=$('dpiNativeLiveFooter');
    if(badge){badge.textContent=running?'LIVE':dpiWebProgress?.error?'FAIL':text?'DONE':'IDLE';badge.className='config-badge '+(!running&&text&&!dpiWebProgress?.error?'active':'')}
    if(meta)meta.textContent=running?`${elapsed}s · streaming`:envelope?.duration_ms?`${envelope.duration_ms} ms · ${envelope.report_bytes||0} bytes${envelope.report_cut?' · CUT':''}`:'—';
    if(body){
      const nearBottom=body.scrollHeight-body.scrollTop-body.clientHeight<100;
      body.innerHTML=text?dpiParityReportHTML(text):'<div class="analysis-empty">Выберите тесты и запустите DPI Detector.</div>';
      if(running&&nearBottom)body.scrollTop=body.scrollHeight;
    }
    if(footer){
      footer.textContent=dpiWebProgress?.error?dpiWebProgress.error:running?'Данные приходят с роутера по мере выполнения теста.':envelope?.environment_warning||'Готов к запуску.';
    }
  }
  function updateDPINativeState(){
    const button=$('dpiNativeRun');
    if(button)button.disabled=true;
    renderDPINativeReport(dpiNativeResult);
  }
  async function runDPINativeReport(){
    if(!dpiWebRunning)await runDPIDetectorWeb();
  }
  document.addEventListener('keydown',e=>{
    if(e.key==='Escape'&&$('dpiConsolePane')?.classList.contains('dpi-console-overlay'))toggleDPIConsoleFullscreen();
  });

  $('dpiModeConsole').onclick=()=>setDPIDetectorMode('console');
  $('dpiModeWeb').onclick=()=>setDPIDetectorMode('web');
  $('dpiModuleRefresh').onclick=()=>loadDPIDetector(true);
  $('dpiConsoleRestart').onclick=restartDPIConsole;
  $('dpiConsoleFullscreen').onclick=toggleDPIConsoleFullscreen;
  document.addEventListener('fullscreenchange',syncDPIConsoleFullscreen);
  $('dpiWebRun').onclick=runDPIDetectorWeb;
  $('dpiNativeRun').onclick=runDPINativeReport;
  $('dpiWebLegend').onclick=loadDPILegend;
  $('dpiLegendClose').onclick=()=>{$('dpiLegendPane').style.display='none';reportHeight()};

  function openOverviewTask(tab,focusId){
    activateNFQWSTab(tab);
    requestAnimationFrame(()=>{
      const target=focusId?$(focusId):null;
      if(target){target.focus();if(typeof target.select==='function'&&target.value)target.select()}
      reportHeight();
    });
  }
  $('uxTaskBroken').onclick=()=>openOverviewTask('domain-v2','v2DomainTarget');
  $('uxTaskCheck').onclick=()=>openOverviewTask('preflight','');
  $('uxTaskImport').onclick=()=>openOverviewTask('strategy-import','strategyImportInput');
  const strategyImportSourceSyntax=mountSyntaxTextarea($('strategyImportInput'),{mode:'config',caption:'StrategyImport source',resources:false});
  if(strategyImportSourceSyntax){const hint=strategyImportSourceSyntax.shell.querySelector('.syntax-editor-hint');if(hint)hint.textContent='BAT / command syntax · preview only'}
  $('strategyImportRun').onclick=runStrategyImport;
  $('strategyImportClear').onclick=clearStrategyImport;
  $('strategyImportPick').onclick=()=>$('strategyImportFile').click();
  $('strategyImportFile').onchange=e=>{const file=e.target.files&&e.target.files[0];loadStrategyImportFile(file);e.target.value=''};
  $('strategyImportCopy').onclick=()=>{const text=$('strategyImportPreview').textContent;if(text&&text!=='Preview не построен.'){navigator.clipboard?.writeText(text);notice('Предпросмотр стратегии скопирован.','good')}};
  $('strategyImportLibraryName').oninput=updateStrategyImportLibrarySave;
  $('strategyImportLibrarySave').onclick=saveStrategyImportLibraryCandidate;
  $('strategyImportLibraryOpen').onclick=openSavedStrategyImportConfig;
  $('strategyImportSandboxProfile').onchange=()=>{$('strategyImportSandboxOpen').disabled=!strategyImportResult?.ready};
  $('strategyImportSandboxOpen').onclick=sendStrategyImportProfileToSandbox;
  $('strategyImportStrategyImport').onclick=importStrategyImportProfilesToStrategyLibrary;

  $('processingLiveCheck').onclick=runProcessingMapLiveProof;

  $('runProcessingMap').onclick=runProcessingMap;$('copyProcessingMap').onclick=copyProcessingMap;$('processingMapDomain').onkeydown=e=>{if(e.key==='Enter')runProcessingMap()};$('start').onclick=()=>action('start');$('stop').onclick=()=>action('stop');$('reload').onclick=()=>action('reload');$('restart').onclick=()=>action('restart');$('refresh').onclick=load;$('refreshPreflight').onclick=refreshPreflightRuntime;$('runHistoryAll').onclick=()=>{runHistoryFilter='all';renderRunHistory()};$('runHistoryTestLab').onclick=()=>{runHistoryFilter='testlab';renderRunHistory()};$('runHistorySandbox').onclick=()=>{runHistoryFilter='sandbox';renderRunHistory()};$('copyRunHistory').onclick=copyRunHistory;$('clearRunHistory').onclick=clearRunHistory;
  $('refreshKeenetic').onclick=()=>{keeneticLoaded=false;dpiDetectorLoaded=false;loadKeenetic(true);loadDPIDetector(true)};$('keeneticUseDefault').onclick=useKeeneticDefaultInterface;
  $('refreshListSources').onclick=()=>{listSourceLoaded=false;loadListSources(true)};$('refreshBackups').onclick=()=>{backupCenterLoaded=false;loadBackupCenter(true)};$('backupCenterSearch').oninput=renderBackupCenter;$('refreshAutoPick').onclick=renderAutoPick;$('recoveryGuardEnabled').onchange=renderAutoPick;$('recoveryGuardThreshold').onchange=renderAutoPick;
  $('refreshCatalog').onclick=()=>{catalogLoaded=false;catalogSelected='';catalogSelectedData=null;loadCatalog(true)};$('refreshBlobs').onclick=()=>{blobLoaded=false;loadBlobs(true)};$('openBlobCatalog').onclick=()=>{const tab=document.querySelector('.tab[data-tab="catalog"]');if(tab)tab.click()};$('catalogSearch').oninput=renderCatalogList;$('catalogAll').onclick=()=>setCatalogFilter('all');$('catalogLocal').onclick=()=>setCatalogFilter('local');$('catalogUpstream').onclick=()=>setCatalogFilter('upstream');
  $('closeSmartApply').onclick=closeSmartApply;$('refreshSmartApply').onclick=async()=>{if(smartApplyBusy)return;$('smartApplyPlan').innerHTML='<div class="empty">Пересчитываю exact plan…</div>';$('applySmartApply').disabled=true;try{smartApplyPlan=await buildSmartApplyPlan();renderSmartApplyPlan()}catch(e){smartApplyPlan=null;$('smartApplyPlan').innerHTML=`<div class="smart-apply-error">${esc(e.message)}</div>`}};$('applySmartApply').onclick=applySmartApply;$('smartApplyModal').onclick=e=>{if(e.target===$('smartApplyModal'))closeSmartApply()};
  $('config').oninput=()=>{updateConfigEditor();$('configSyntaxStatus').textContent='Конфиг изменён: проверка не актуальна.';$('configSemanticIssues').style.display='none';$('configSemanticIssues').textContent='';$('configSyntaxStatus').style.color=''};$('config').onscroll=()=>{$('gutter').scrollTop=$('config').scrollTop};$('config').onkeydown=e=>{if(e.key==='Tab'){e.preventDefault();const t=e.target,a=t.selectionStart,b=t.selectionEnd;t.value=t.value.slice(0,a)+'  '+t.value.slice(b);t.selectionStart=t.selectionEnd=a+2;updateConfigEditor()}};
  $('nf1bLuaPanel').ontoggle=()=>{if($('nf1bLuaPanel').open)void nf1bLuaLoad()};
  $('nf1bLuaRefresh').onclick=()=>{void nf1bLuaLoad(true)};
  $('nf1bLuaQuery').oninput=nf1bLuaRender;
  $('nf1bLuaPayloadFix').onclick=nf1bLuaPayloadFixSelected;
  $('checkConfigSyntax').onclick=checkConfigSyntaxPreview;$('save').onclick=showDiff;$('diffConfig').onclick=showDiff;$('undoConfig').onclick=()=>{$('config').value=original;updateConfigEditor();refreshSyntaxEditor($('config'))};  $('closeDiff').onclick=closeDiff;$('diffCancel').onclick=closeDiff;$('diffApply').onclick=()=>{$('diffApply').dataset.mode==='flow'?applyFlow():$('diffApply').dataset.mode==='stored-config'?saveStoredConfig():saveConfig()};$('resetSetup').onclick=resetFlow;$('diffSetup').onclick=previewFlow;$('applySetup').onclick=previewFlow;$('addCustomBlock').onclick=()=>{customBlocks.push('--filter-tcp=443 --dpi-desync=fake');customLabels.push('');renderFlow()};document.querySelectorAll('input[name="flowMode"]').forEach(r=>r.onchange=()=>{flowValues.NFQWS_EXTRA_ARGS=`$${r.value}`;updateModeCards();updateFlowState()});document.querySelectorAll('input[name="policyMode"]').forEach(r=>r.onchange=()=>setPolicyMode(r.value));document.querySelectorAll('[data-open-list]').forEach(b=>b.onclick=()=>openListFromSetup(b.dataset.openList));for(const id of ['flowInterface','flowBaseArgs','flowArgs','flowQuic','flowUdp','flowIpset','flowTcpPorts','flowUdpPorts','flowPolicyName','flowLogPath'])$(id).oninput=updateFlowState;for(const id of ['flowIPv6','flowLogLevel'])$(id).onchange=updateFlowState;$('diffModal').onclick=e=>{if(e.target===$('diffModal'))closeDiff()};
  $('configLibrarySearch').oninput=renderConfigLibraryList;$('newStoredConfig').onclick=()=>openConfigName('create-empty');$('saveCurrentConfig').onclick=()=>openConfigName('create-current');$('refreshNeighborConfigs').onclick=()=>{configLibraryLoaded=false;loadConfigLibrary(true)};$('storedConfigEditor').oninput=updateStoredConfigButtons;$('saveStoredConfig').onclick=showStoredConfigDiff;$('diffStoredConfig').onclick=showStoredConfigDiff;$('undoStoredConfig').onclick=()=>{$('storedConfigEditor').value=storedConfigOriginal;updateStoredConfigButtons()};$('fullscreenStoredConfig').onclick=()=>{const shell=$('storedConfigEditorShell'),on=!shell.classList.contains('full-editor');shell.classList.toggle('full-editor',on);document.body.classList.toggle('syntax-fullscreen-open',on);document.documentElement.classList.toggle('syntax-fullscreen-open',on);$('fullscreenStoredConfig').textContent=on?'Закрыть fullscreen':'На весь экран';resizeStoredConfigEditor();reportHeight()};$('activateStoredConfig').onclick=activateStoredConfig;$('deleteStoredConfig').onclick=deleteStoredConfig;$('renameStoredConfig').onclick=()=>openConfigName('rename',selectedStoredConfig);$('duplicateStoredConfig').onclick=()=>openConfigName('duplicate',selectedStoredConfig);$('closeConfigName').onclick=closeConfigName;$('configNameCancel').onclick=closeConfigName;$('configNameConfirm').onclick=confirmConfigName;$('configNameInput').onkeydown=e=>{if(e.key==='Enter')confirmConfigName()};$('configNameModal').onclick=e=>{if(e.target===$('configNameModal'))closeConfigName()};
  $('listSearch').oninput=renderListFiles;$('inspectListEntry').onclick=runEntryInspector;$('staticPathEntry').onclick=runStaticPath;$('analyzeLists').onclick=runListAnalysis;$('listEditor').oninput=updateListButtons;$('listEditor').onclick=updateListButtons;$('listEditor').onkeyup=updateListButtons;$('listEditor').onscroll=()=>{$('listGutter').scrollTop=$('listEditor').scrollTop;$('listHighlight').scrollTop=$('listEditor').scrollTop;$('listHighlight').scrollLeft=$('listEditor').scrollLeft};$('fullscreenList').onclick=()=>{const panel=$('listEditorPanel'),on=!panel.classList.contains('full-editor');panel.classList.toggle('full-editor',on);document.body.classList.toggle('syntax-fullscreen-open',on);document.documentElement.classList.toggle('syntax-fullscreen-open',on);$('fullscreenList').textContent=on?'Закрыть fullscreen':'На весь экран';reportHeight()};$('normalizeList').onclick=normalizeList;$('undoList').onclick=()=>{$('listEditor').value=listOriginal;updateListButtons()};$('saveList').onclick=saveList;$('deleteList').onclick=deleteList;$('previewRenameList').onclick=previewListRename;$('applyRenameList').onclick=applyListRename;$('closeRenamePreview').onclick=closeListRenamePreview;$('renamePreviewClose').onclick=closeListRenamePreview;$('newList').onclick=openCreate;$('closeCreate').onclick=closeCreate;$('createCancel').onclick=closeCreate;$('createConfirm').onclick=createList;$('newListName').onkeydown=e=>{if(e.key==='Enter')createList()};
  $('closeEntryInspector').onclick=closeEntryInspector;$('rerunEntryInspector').onclick=runEntryInspector;$('entryInspectorModal').onclick=e=>{if(e.target===$('entryInspectorModal'))closeEntryInspector()};$('closeStaticPath').onclick=closeStaticPath;$('rerunStaticPath').onclick=runStaticPath;$('staticPathModal').onclick=e=>{if(e.target===$('staticPathModal'))closeStaticPath()};
  $('checkList').onclick=runDomainCheck;$('closeCheck').onclick=closeCheck;$('stopCheck').onclick=stopDomainCheck;$('retryCheck').onclick=runDomainCheck;$('closeAnalysis').onclick=closeAnalysis;$('rerunAnalysis').onclick=runListAnalysis;for(const[id,val]of[['analysisAll','all'],['analysisConflict','conflict'],['analysisDomain','domain'],['analysisNetwork','network']])$(id).onclick=()=>{analysisFilter=val;renderAnalysis()};$('analysisPeer').onchange=()=>{analysisPeer=$('analysisPeer').value;renderAnalysis()};$('openAnalysisPeer').onclick=()=>{if(analysisPeer)openListAtLine(analysisPeer,pairSideRecords(pairFindings(analysisPeer,false),analysisPeer)[0]?.line||1)};$('cleanupAnalysis').onclick=openAnalysisCleanup;$('cleanupCancel').onclick=()=>$('analysisCleanup').classList.remove('open');$('cleanupLeft').onclick=()=>cleanupAnalysisSide('left');$('cleanupRight').onclick=()=>cleanupAnalysisSide('right');$('analysisModal').onclick=e=>{if(e.target===$('analysisModal'))closeAnalysis()};
  $('logSearch').oninput=renderLog;$('logAll').onclick=()=>{logMode='all';renderLog()};$('logErrors').onclick=()=>{logMode='errors';renderLog()};$('logWarnings').onclick=()=>{logMode='warnings';renderLog()};$('copyLog').onclick=()=>navigator.clipboard?.writeText(lines(current?.log_tail||'').join('\n'));$('scriptSearch').oninput=renderScriptList;$('copyScript').onclick=()=>navigator.clipboard?.writeText(scriptContent);$('fullscreenScript').onclick=()=>{const panel=$('scriptPanel'),on=!panel.classList.contains('script-fullscreen');panel.classList.toggle('script-fullscreen',on);document.body.classList.toggle('syntax-fullscreen-open',on);document.documentElement.classList.toggle('syntax-fullscreen-open',on);$('fullscreenScript').textContent=on?'Закрыть fullscreen':'На весь экран';reportHeight()};
  document.addEventListener('keydown',handleEditorSaveShortcut,true);
  document.addEventListener('keydown',e=>{if(e.key==='Escape'){if($('smartApplyModal')?.classList.contains('open')){closeSmartApply();}const panel=$('configEditorPanel');if(panel?.classList.contains('full-editor')){$('fullscreenConfig')?.click()}const storedPanel=$('storedConfigEditorShell');if(storedPanel?.classList.contains('full-editor'))$('fullscreenStoredConfig')?.click();const listPanel=$('listEditorPanel');if(listPanel?.classList.contains('full-editor'))$('fullscreenList')?.click();const scriptPanel=$('scriptPanel');if(scriptPanel?.classList.contains('script-fullscreen'))$('fullscreenScript')?.click()}});
  runHistoryLoad();
  load();
}
