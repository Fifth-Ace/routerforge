<script>
  import { onMount } from 'svelte';
  import { WIDGETS, STORAGE_KEY, getWidget, defaultLayout, profileTiles, parseLayout, normalizeLayout, updateSpace, compactReorder, boundedPercent, widgetFor, ACCENTS, dimensions, nearestSize, spatialOrder, safeExternal, recordHistory, chartSegments, columnsForWidth, compactTiles, compactLayout, projectTiles, compactResize, moveToCell, metricStats, widgetCategory } from '$lib/workspace.js';
  export let locale='ru';
  export let metrics={};
  export let sampleKeys={};
  export let ready=false;
  export let online=false;
  export let issueCount=0;
  export let criticalCount=0;
  export let attention;
  export let modules;
  let layout=compactLayout(defaultLayout()), editing=false, adding=false, transfer=false, transferText='', error='', persisted=true, loaded=false, dragId='', profile='default', name='';
  let grid, gridWidth=0, gesture=null, preview=null, selected='', tileTitle='', tileAccent='auto', tileDetail=true, linkTitle='', linkURL='', history={}, historyKeys={};
  let manageOpen=false, search='', category='all', past=[], future=[], previewTiles=null, fullPanel='';
  export let alerts=[];
  export let installed=[];
  export let catalogKnown=false;
  $: columns=columnsForWidth(gridWidth);
  $: ordered=projectTiles(active.tiles,columns);
  $: available=WIDGETS.filter(w=>!active.tiles.some(t=>t.id===w.id)&&(category==='all'||widgetCategory(w)===category)&&`${w.ru} ${w.en} ${w.id}`.toLowerCase().includes(search.trim().toLowerCase()));
  const categories=['all','system','network','dns','management','events','shortcuts'];
  const categoryName=(c,en)=>({all:tr('Все','All',en),system:tr('Система','System',en),network:tr('Сеть','Network',en),dns:'DNS',management:tr('Управление','Management',en),events:tr('События','Events',en),shortcuts:tr('Ярлыки','Shortcuts',en)})[c];
  $: sampleHistory(metrics,sampleKeys,online);
  $: trafficMax=Math.max(1,...['traffic-rx','traffic-tx'].flatMap(id=>(history[id]||[]).map(s=>s.value??0)));
  $: selectedTile=active.tiles.find(t=>t.id===selected);
  function sampleHistory(m,k,o){const r=recordHistory(history,historyKeys,m,k,o);history=r.history;historyKeys=r.keys;}
  $: en=locale==='en';
  $: active=layout.spaces.find(s=>s.id===layout.active) || layout.spaces[0];
  const tr=(ru,enText,isEn)=>isEn?enText:ru;
  const profileLabel=(id,isEn)=>id==='network'?tr('Сеть','Network',isEn):id==='admin'?tr('Обслуживание','Maintenance',isEn):tr('Главная','Home',isEn);
  const title=(w,isEn)=>isEn?w.en:w.ru;
  onMount(()=>{
    try{const raw=localStorage.getItem(STORAGE_KEY);if(raw)layout=compactLayout(parseLayout(raw));}catch{error='restore';}
    loaded=true;
    const observer=typeof ResizeObserver==='undefined'?null:new ResizeObserver(entries=>{gridWidth=entries[0].contentRect.width;});
    gridWidth=grid?.clientWidth||0;
    if(grid&&observer)observer.observe(grid);
    // Pointer capture is an optimization, not the lifetime of a gesture.
    // Window listeners survive loss of capture or replacement of a tile's handle.
    const handlers=[['pointermove',updatePointer],['pointerup',endPointer],['pointercancel',abortPointer],['blur',blurPointer],['keydown',escapePointer]];
    for(const [type,handler] of handlers)window.addEventListener(type,handler,true);
    document.addEventListener('visibilitychange',visibilityPointer);
    return ()=>{cancelPointer();observer?.disconnect();for(const [type,handler] of handlers)window.removeEventListener(type,handler,true);document.removeEventListener('visibilitychange',visibilityPointer);};
  });
  function persist(next){if(gesture)cancelPointer();layout=normalizeLayout(next);if(loaded){try{localStorage.setItem(STORAGE_KEY,JSON.stringify(layout));persisted=true;}catch{persisted=false;}}}
  function commit(next,record=true){const normalized=normalizeLayout(next);if(JSON.stringify(normalized)===JSON.stringify(layout))return;if(record){past=[...past,layout].slice(-24);future=[];}persist(normalized);}
  function undo(){if(!past.length)return;const next=past.at(-1);past=past.slice(0,-1);future=[...future,layout].slice(-24);selected='';persist(next);}
  function redo(){if(!future.length)return;const next=future.at(-1);future=future.slice(0,-1);past=[...past,layout].slice(-24);selected='';persist(next);}
  function editMode(){editing=!editing;fullPanel='';adding=false;manageOpen=false;transfer=false;selected='';cancelPointer();}
  function showCatalog(){adding=!adding;manageOpen=false;transfer=false;selected='';search='';category='all';}
  function showManage(){manageOpen=!manageOpen;adding=false;transfer=false;selected='';}
  function closeSettings(){selected='';}
  function showFull(id){if(editing)return;fullPanel=fullPanel===id?'':id;if(fullPanel)requestAnimationFrame(()=>document.getElementById('rf-workspace-details')?.scrollIntoView({block:'nearest'}));}

  function change(fn){try{commit(updateSpace(layout,active.id,fn));error='';}catch{error='invalid';}}
  function selectSpace(id){fullPanel='';commit({...layout,active:id},false);name='';error='';selected='';cancelPointer();}
  function addSpace(copy=false){if(layout.spaces.length>=8)return;const id='space-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8);const label=name.trim().slice(0,64);try{commit({...layout,active:id,spaces:[...layout.spaces,{id,name:label,profile,tiles:copy?active.tiles.map(x=>({...x})):compactTiles(profileTiles(profile))}]});name='';selected='';error='';}catch{error='invalid';}}
  function deleteSpace(){if(layout.spaces.length<=1||!confirm(tr('Удалить рабочее пространство?','Delete workspace?',en)))return;selected='';const spaces=layout.spaces.filter(s=>s.id!==active.id);commit({...layout,spaces,active:spaces[0].id});}
  function rename(){try{change(s=>({...s,name:name.trim().slice(0,64)}));name='';error='';}catch{error='invalid';}}
  function add(id){const w=getWidget(id);if(!w||active.tiles.some(t=>t.id===id))return;change(s=>({...s,tiles:compactTiles([...spatialOrder(s.tiles),{id,size:w.size}])}));}
  function remove(id){change(s=>({...s,tiles:compactTiles(spatialOrder(s.tiles).filter(t=>t.id!==id))}));}
  function selectTile(tile){adding=false;manageOpen=false;transfer=false;selected=tile.id;tileTitle=tile.title||'';tileAccent=tile.accent||'auto';tileDetail=tile.showDetail!==false;}
  function saveTile(){if(!selectedTile)return;change(s=>({...s,tiles:s.tiles.map(t=>t.id===selected?{...t,title:tileTitle,accent:tileAccent,showDetail:tileDetail}:t)}));}
  function addLink(){try{const url=safeExternal(linkURL);if(active.tiles.filter(t=>widgetFor(t).type==='external').length>=12)throw Error('LINK_LIMIT');const id='link-'+Date.now().toString(36)+Math.random().toString(36).slice(2,8);commit(updateSpace(layout,active.id,s=>({...s,tiles:compactTiles([...spatialOrder(s.tiles),{id,size:'1x1',url,title:linkTitle.trim()||new URL(url).hostname.slice(0,64)}])})));linkTitle='';linkURL='';error='';}catch{error='invalid';}}
  function positionStyle(tile){return `grid-column:${tile.x+1}/span ${tile.w};grid-row:${tile.y+1}/span ${tile.h};`;}
  function nudge(e,tile){const delta={ArrowLeft:-1,ArrowRight:1,ArrowUp:-columns,ArrowDown:columns}[e.key];if(delta===undefined)return;e.preventDefault();const i=ordered.findIndex(t=>t.id===tile.id),target=ordered[Math.max(0,Math.min(ordered.length-1,i+delta))];if(target&&target.id!==tile.id)change(s=>({...s,tiles:compactReorder(s.tiles,tile.id,target.id)}));}
  function beginPointer(e,tile,kind){
    if(!editing||e.button!==0||e.isPrimary===false||gesture)return;
    e.preventDefault();
    const handle=e.currentTarget;
    gesture={id:tile.id,kind,pointer:e.pointerId,x:e.clientX,y:e.clientY,tile:{...tile},columns,handle,layout,visual:ordered,original:active.tiles,moved:false};
    dragId=tile.id;preview={...tile};previewTiles=null;
    try{handle.setPointerCapture(e.pointerId);}catch{/* Window listeners remain active if capture is unavailable. */}
  }
  function validGesture(e){
    if(!gesture||gesture.pointer!==e.pointerId)return false;
    if(!editing||gesture.layout!==layout||gesture.columns!==columns){cancelPointer();return false;}
    return true;
  }
  function updatePointer(e){
    if(!validGesture(e))return;
    if(e.pointerType==='mouse'&&e.type==='pointermove'&&!(e.buttons&1)){cancelPointer();return;}
    const g=gesture,t=g.tile,css=getComputedStyle(grid);
    const pitchX=(grid.getBoundingClientRect().width+parseFloat(css.columnGap))/columns,pitchY=parseFloat(css.gridAutoRows)+parseFloat(css.rowGap);
    if(!Number.isFinite(pitchX)||!Number.isFinite(pitchY)||pitchX<=0||pitchY<=0){cancelPointer();return;}
    const dx=Math.round((e.clientX-g.x)/pitchX),dy=Math.round((e.clientY-g.y)/pitchY);
    if(Math.hypot(e.clientX-g.x,e.clientY-g.y)>6)g.moved=true;
    if(!dx&&!dy){preview={...t};previewTiles=null;return;}
    if(g.kind==='resize'){
      const [w,h]=dimensions(t.size),size=nearestSize(widgetFor(t),Math.max(1,w+dx),Math.max(1,h+dy));
      previewTiles=compactResize(g.original,g.id,size);
    }else{
      const target=document.elementFromPoint(e.clientX,e.clientY)?.closest('[data-widget]')?.getAttribute('data-widget');
      if(target&&target!==g.id)previewTiles=compactReorder(g.original,g.id,target);
      else previewTiles=moveToCell(g.original,g.visual,g.id,Math.max(0,Math.min(columns-1,t.x+dx)),Math.max(0,t.y+dy));
    }
    preview=projectTiles(previewTiles,columns).find(x=>x.id===g.id)||{...t};
  }
  function cancelPointer(){
    const g=gesture;gesture=null;preview=null;previewTiles=null;dragId='';
    if(g){try{if(g.handle.hasPointerCapture(g.pointer))g.handle.releasePointerCapture(g.pointer);}catch{/* Detached handles do not retain capture. */}}
  }
  function abortPointer(e){if(gesture?.pointer===e.pointerId)cancelPointer();}
  function escapePointer(e){if(gesture&&e.key==='Escape'){e.preventDefault();cancelPointer();}}
  function blurPointer(e){if(e.target===window)cancelPointer();}
  function visibilityPointer(){if(document.hidden)cancelPointer();}
  function endPointer(e){
    if(!validGesture(e))return;
    const g=gesture;updatePointer(e);if(!gesture)return;
    const next=previewTiles;
    cancelPointer();
    if(next)change(s=>({...s,tiles:next}));
    else if(!g.moved&&g.kind==='move')selectTile(g.tile);
  }
  function importLayout(){try{const next=compactLayout(parseLayout(transferText));if(!confirm(tr('Заменить все рабочие пространства?','Replace all workspaces?',en)))return;commit(next);selected='';transfer=false;error='';}catch{error='invalid';}}
  function exportLayout(){transferText=JSON.stringify(layout,null,2);transfer=true;error='';}
  function reset(){if(!confirm(tr('Заменить текущую компоновку шаблоном?','Replace current layout with template?',en)))return;selected='';change(s=>({...s,profile,tiles:compactTiles(profileTiles(profile))}));}
  function valueFor(id,data){const v=data[id];return v&&typeof v.value==='string'?v:{value:'—',detail:'',percent:null,state:'unknown'};}
</script>
<section class="rf-workspace" aria-label={tr('Рабочее пространство','Workspace',en)}>
  <div class="rf-ws-toolbar">
    <div class="rf-ws-heading"><strong>RouterForge Workspace</strong><small>{tr('Ваше рабочее пространство','Your workspace',en)}</small></div>
    <select aria-label={tr('Рабочее пространство','Workspace',en)} value={layout.active} onchange={e=>selectSpace(e.currentTarget.value)}>
      {#each layout.spaces as space (space.id)}<option value={space.id}>{space.name||profileLabel(space.profile,en)}</option>{/each}
    </select>
    <button type="button" class="button" onclick={editMode}>{editing?tr('Готово','Done',en):tr('Настроить','Customize',en)}</button>
  </div>
  {#if !ready || !online}<div class="rf-ws-message" role="status">{!ready?tr('Получение данных Core…','Loading Core data…',en):tr('Core недоступен; значения могут быть устаревшими.','Core unavailable; values may be stale.',en)}</div>{/if}
  {#if issueCount>0}<div class="rf-ws-message" class:critical={criticalCount>0} role="status"><strong>{tr('Требует внимания','Needs attention',en)}: {issueCount}</strong>{#if !active.tiles.some(t=>t.id==='attention')}<button class="button" type="button" onclick={()=>add('attention')}>{tr('Показать предупреждения','Show warnings',en)}</button>{/if}</div>{/if}
  {#if error || !persisted}<div class="rf-ws-message" role="alert">{!persisted?tr('Браузер не сохранил компоновку.','Browser could not save layout.',en):error==='restore'?tr('Сохранённая компоновка повреждена; загружен стандартный профиль.','Saved layout is invalid; default profile loaded.',en):tr('Некорректная компоновка или имя.','Invalid layout or name.',en)}</div>{/if}
  {#if editing}
    <div class="rf-ws-editbar">
      <button type="button" class="button primary" onclick={showCatalog}>{tr('＋ Виджет','＋ Widget',en)}</button>
      <button type="button" class="button" aria-label={tr('Отменить','Undo',en)} title={tr('Отменить','Undo',en)} disabled={!past.length} onclick={undo}>↶</button>
      <button type="button" class="button" aria-label={tr('Повторить','Redo',en)} title={tr('Повторить','Redo',en)} disabled={!future.length} onclick={redo}>↷</button>
      <button type="button" class="button" onclick={showManage}>{tr('Пространства и шаблоны','Spaces and templates',en)}</button>
      <small>{tr('Нажми плитку для настройки или перетащи её. Размер — за угол.','Select or drag a tile. Resize using its corner.',en)}</small>
    </div>
    {#if manageOpen||adding||selectedTile}
    <div class="rf-ws-editor panel">
      {#if manageOpen}
        <div class="rf-ws-actions">
          <select aria-label={tr('Шаблон','Template',en)} bind:value={profile}><option value="default">{profileLabel('default',en)}</option><option value="network">{profileLabel('network',en)}</option><option value="admin">{profileLabel('admin',en)}</option></select>
          <button type="button" class="button" onclick={reset}>{tr('Применить шаблон','Apply template',en)}</button>
          <button type="button" class="button" onclick={exportLayout}>{tr('Экспорт / импорт','Export / import',en)}</button>
        </div>
        <div class="rf-ws-actions">
          <input aria-label={tr('Имя пространства','Workspace name',en)} placeholder={tr('Имя пространства','Workspace name',en)} maxlength="64" bind:value={name}/>
          <button type="button" class="button" disabled={layout.spaces.length>=8} onclick={()=>addSpace()}>{tr('Создать','Create',en)}</button>
          <button type="button" class="button" disabled={layout.spaces.length>=8} onclick={()=>addSpace(true)}>{tr('Копировать','Duplicate',en)}</button>
          <button type="button" class="button" onclick={rename}>{tr('Переименовать','Rename',en)}</button>
          <button type="button" class="button" disabled={layout.spaces.length<=1} onclick={deleteSpace}>{tr('Удалить пространство','Delete workspace',en)}</button>
        </div>
        {#if transfer}<div class="rf-ws-transfer"><label>{tr('JSON компоновки (без секретов)','Layout JSON (no secrets)',en)}<textarea rows="8" bind:value={transferText} maxlength="65536" spellcheck="false"></textarea></label><button class="button primary" type="button" onclick={importLayout}>{tr('Импортировать JSON','Import JSON',en)}</button></div>{/if}
      {:else if adding}
        <div class="rf-ws-actions"><input aria-label={tr('Поиск виджетов','Search widgets',en)} placeholder={tr('Поиск виджетов','Search widgets',en)} bind:value={search}/><select aria-label={tr('Категория','Category',en)} bind:value={category}>{#each categories as c}<option value={c}>{categoryName(c,en)}</option>{/each}</select><button type="button" class="button" onclick={()=>adding=false}>{tr('Закрыть','Close',en)}</button></div>
        <div class="rf-ws-catalog">{#each available as w (w.id)}<button type="button" class="rf-ws-catalog-card" data-widget-choice={w.id} onclick={()=>add(w.id)}><span class="rf-ws-icon">{w.icon}</span><strong>＋ {title(w,en)}</strong><small>{categoryName(widgetCategory(w),en)} · {w.size}</small><span class="rf-ws-catalog-value">{w.type==='metric'?(online||w.id==='core'?valueFor(w.id,metrics).value:'—'):'↗'}</span></button>{:else}<small>{tr('Нет подходящих виджетов','No matching widgets',en)}</small>{/each}</div>
        {#if category==='all'||category==='shortcuts'}<div class="rf-ws-actions"><input aria-label={tr('Название ярлыка','Shortcut title',en)} placeholder={tr('Название ярлыка','Shortcut title',en)} maxlength="64" bind:value={linkTitle}/><input aria-label={tr('Адрес HTTP(S)','HTTP(S) address',en)} placeholder="https://example.net/" maxlength="512" bind:value={linkURL}/><button class="button" type="button" onclick={addLink}>{tr('Добавить внешний ярлык','Add external shortcut',en)}</button><small>{tr('Без логинов, токенов и секретных путей.','No credentials, tokens or secret paths.',en)}</small></div>{/if}
      {:else if selectedTile}
        <div class="rf-ws-actions rf-ws-tile-settings"><strong>{title(widgetFor(selectedTile),en)}</strong><input aria-label={tr('Заголовок плитки','Tile title',en)} maxlength="64" placeholder={tr('Заголовок плитки','Tile title',en)} bind:value={tileTitle}/><select aria-label={tr('Акцент плитки','Tile accent',en)} bind:value={tileAccent}>{#each ACCENTS as color}<option value={color}>{({auto:tr('Тема','Theme',en),green:tr('Зелёный','Green',en),blue:tr('Синий','Blue',en),violet:tr('Фиолетовый','Violet',en),amber:tr('Янтарный','Amber',en)})[color]}</option>{/each}</select><select aria-label={tr('Размер плитки','Tile size',en)} value={selectedTile.size} onchange={e=>change(s=>({...s,tiles:compactResize(s.tiles,selectedTile.id,e.currentTarget.value)}))}>{#each widgetFor(selectedTile).sizes as size}<option value={size}>{size}</option>{/each}</select><label><input type="checkbox" bind:checked={tileDetail}/> {tr('Подробности','Details',en)}</label><button class="button primary" type="button" onclick={saveTile}>{tr('Сохранить плитку','Save tile',en)}</button><button class="button" type="button" onclick={()=>{const id=selected;selected='';remove(id);}}>{tr('Удалить плитку','Remove tile',en)}</button><button class="button" type="button" onclick={closeSettings}>{tr('Закрыть','Close',en)}</button></div>
      {/if}
    </div>
    {/if}
  {/if}
  {#if fullPanel&&!editing}
    <section id="rf-workspace-details" class="rf-ws-detail-section" aria-label={tr('Подробности','Details',en)}>
      <div class="rf-ws-actions"><strong>{fullPanel==='attention'?tr('Предупреждения','Attention',en):tr('Модули и интеграции','Modules and integrations',en)}</strong><button type="button" class="button" onclick={()=>fullPanel=''}>{tr('Закрыть','Close',en)}</button></div>
      {#if fullPanel==='attention'}{@render attention()}{:else}{@render modules()}{/if}
    </section>
  {/if}
  <div class="rf-ws-grid" class:editing bind:this={grid} style={`--ws-cols:${columns}`} class:compact={columns<4}>
    {#each ordered as tile,i (tile.id)}
      {@const w=widgetFor(tile)}
      {@const data=valueFor(tile.id,metrics)}
      {@const label=tile.title||title(w,en)}
      {@const segments=chartSegments(history[tile.id],tile.id)}
      {@const rxSegments=tile.id==='traffic'?chartSegments(history['traffic-rx'],'traffic-rx',trafficMax):[]}
      {@const txSegments=tile.id==='traffic'?chartSegments(history['traffic-tx'],'traffic-tx',trafficMax):[]}
      <article class="rf-ws-tile" class:dragging={dragId===tile.id} class:shortcut={w.type==='shortcut'||w.type==='external'} data-widget={tile.id} data-accent={tile.accent||'auto'} class:selected={selected===tile.id&&editing} data-size={tile.size} style={positionStyle(tile)}>
        {#if editing}
          <button type="button" class="rf-ws-handle rf-ws-controls" aria-label={tr('Настройки плитки','Tile settings',en)+' '+label} title={tr('Нажми для настройки, перетащи для перемещения','Click to configure, drag to move',en)} onpointerdown={e=>beginPointer(e,tile,'move')} onkeydown={e=>nudge(e,tile)} onclick={e=>{if(e.detail===0)selectTile(tile);}}><span aria-hidden="true">⠿</span></button>
        {/if}
        {#if w.type==='panel'}
          <div class="rf-ws-panel-content">
            <div class="rf-ws-panel-head"><strong>{label}</strong><span>{w.id==='attention'?issueCount:catalogKnown?installed.length:'—'}</span></div>
            {#if w.id==='attention'}
              {#if alerts.length}<div class="rf-ws-summary-list">{#each alerts.slice(0,tile.h>=4?8:3) as item}<a href={item.href||'/apps'} tabindex={editing?-1:0} onclick={e=>{if(editing)e.preventDefault();}}><span class="rf-ws-dot" data-state={item.severity}></span><span>{item.title}</span><span aria-hidden="true">›</span></a>{/each}</div>{#if alerts.length>(tile.h>=4?8:3)}<button type="button" class="rf-ws-more" onclick={()=>showFull('attention')}>{tr('Ещё','More',en)} {alerts.length-(tile.h>=4?8:3)} ↗</button>{/if}
              {:else}<small>{ready&&online&&catalogKnown?tr('Нет активных предупреждений','No active warnings',en):tr('Ожидание данных','Waiting for data',en)}</small>{/if}
              <button type="button" class="rf-ws-more" onclick={()=>showFull('attention')}>{tr('Все подробности','All details',en)} ↗</button>
            {:else}
              <small>{catalogKnown?tr('Установлено компонентов','Installed components',en):tr('Ожидание данных','Waiting for data',en)}</small>
              <div class="rf-ws-module-list">{#each installed.slice(0,tile.h>=4?12:4) as item}<a href={item.href||'/apps'} tabindex={editing?-1:0} onclick={e=>{if(editing)e.preventDefault();}}><span class="rf-ws-dot" data-state={item.state}></span><span>{item.name}</span></a>{/each}</div>
              <a class="rf-ws-more" href="/apps" tabindex={editing?-1:0} onclick={e=>{if(editing)e.preventDefault();}}>{tr('Центр приложений','App Center',en)} ↗</a>
              <button type="button" class="rf-ws-more" onclick={()=>showFull('modules')}>{tr('Все подробности','All details',en)} ↗</button>
            {/if}
          </div>
        {:else if w.type==='external'}
          <a class="rf-ws-link" tabindex={editing?-1:0} href={tile.url} target="_blank" rel="noopener noreferrer" onclick={e=>{if(editing)e.preventDefault();}}><span class="rf-ws-icon">↗</span><strong>{label}</strong><small>{new URL(tile.url).hostname}</small></a>
        {:else if w.type==='shortcut'}
          <a class="rf-ws-link" tabindex={editing?-1:0} href={w.route} onclick={e=>{if(editing)e.preventDefault();}}><span class="rf-ws-icon">{w.icon}</span><strong>{label}</strong><span class="rf-ws-arrow">↗</span></a>
        {:else}
          <a class="rf-ws-link" tabindex={editing?-1:0} href={w.route} onclick={e=>{if(editing)e.preventDefault();}} data-state={data.state||'unknown'}>
            <span class="rf-ws-label"><span class="rf-ws-icon">{w.icon}</span>{label}</span>
            <div class="rf-ws-metric-body">
            {#if tile.id==='traffic'&&online&&Number.isFinite(data.rx)&&Number.isFinite(data.tx)}
              <div class="rf-ws-traffic-values"><strong class="rf-ws-value">↓ {data.rx.toFixed(2)}</strong><strong class="rf-ws-tx">↑ {data.tx.toFixed(2)}</strong><small>{tr('Мбит/с','Mbps',en)}</small></div>
            {:else}<strong class="rf-ws-value">{online||tile.id==='core'?data.value:'—'}</strong>{/if}
            {#if online && tile.showDetail!==false && tile.size!=='1x1' && data.detail}<small>{tile.id==='traffic'?(data.interfaceName||data.detail):data.detail}</small>{/if}
            {#if online && tile.showDetail!==false && tile.size!=='1x1' && boundedPercent(data.percent)!==null}<div class="rf-ws-meter" aria-hidden="true"><span style:width={`${data.percent}%`}></span></div>{/if}
            {#if online && tile.showDetail!==false && tile.size!=='1x1' && tile.id==='traffic' && (rxSegments.length||txSegments.length)}<svg class="rf-ws-chart" viewBox="0 0 200 42" preserveAspectRatio="none" role="img" aria-label={tr('Получение и отправка за сеанс','Session receive and transmit',en)}>{#each [4,21,38] as y}<line x1="0" x2="200" y1={y} y2={y} stroke="var(--border)" vector-effect="non-scaling-stroke"/>{/each}{#each rxSegments as points}<polyline {points} fill="none" stroke="currentColor" stroke-width="1.5" vector-effect="non-scaling-stroke"/>{/each}{#each txSegments as points}<polyline {points} fill="none" stroke="var(--muted)" stroke-width="1.5" vector-effect="non-scaling-stroke"/>{/each}</svg>{#if tile.h>=2}<small class="rf-ws-chart-caption">0–{trafficMax.toFixed(2)} {tr('Мбит/с','Mbps',en)} · RX / TX · {tr('Отсчёты за сеанс','Session samples',en)}: {Math.max(history['traffic-rx']?.length||0,history['traffic-tx']?.length||0)}</small>{/if}{/if}
            {#if online && tile.showDetail!==false && tile.size!=='1x1' && segments.length}<svg class="rf-ws-chart" viewBox="0 0 200 42" preserveAspectRatio="none" role="img" aria-label={tr('История наблюдений за текущий сеанс','Observation history for this session',en)}>{#each segments as points}<polyline {points} fill="none" stroke="currentColor" stroke-width="1.5" vector-effect="non-scaling-stroke"/>{/each}</svg>{#if tile.h>=2&&metricStats(history[tile.id])}{@const stats=metricStats(history[tile.id])}<small>{tr('Среднее','Mean',en)}: {stats.mean.toFixed(1)}{tile.id==='thermal'?'°C':'%'} · {tr('Пик','Peak',en)}: {stats.peak.toFixed(1)}{tile.id==='thermal'?'°C':'%'}</small>{/if}{/if}
            {#if online && tile.showDetail!==false && tile.h>=2 && tile.id!=='traffic' && Array.isArray(data.rows)}<div class="rf-ws-rows">{#each data.rows.slice(0,3) as row}<span>{row}</span>{/each}</div>{/if}
            {#if !online}<small>{tr('Нет актуальных данных','No current data',en)}</small>{/if}
            </div>
          </a>
        {/if}
        {#if editing && columns>=2}<button type="button" class="rf-ws-resize" aria-label={tr('Изменить размер за угол','Resize by corner',en)+' '+label} onpointerdown={e=>beginPointer(e,tile,'resize')}>◢</button>{/if}
      </article>
    {:else}<div class="rf-ws-message">{tr('Добавьте первую плитку в режиме настройки.','Add your first tile in customize mode.',en)}</div>{/each}
    {#if preview}<div class="rf-ws-preview" style={positionStyle(preview)} aria-hidden="true"></div>{/if}
  </div>
</section>
<style>
  .rf-workspace{display:grid;gap:10px;min-width:0;color:var(--text)}
  .rf-ws-toolbar,.rf-ws-editbar,.rf-ws-actions{display:flex;align-items:center;flex-wrap:wrap;gap:7px;min-width:0}
  .rf-ws-heading{display:grid;gap:2px;flex:1;min-width:140px}.rf-ws-heading strong{font-size:17px}
  small,.rf-ws-message{color:var(--muted);font-size:12px}.rf-ws-editbar small{flex:1;min-width:160px}
  .rf-ws-toolbar select,.rf-ws-editor select,.rf-ws-editor input,.rf-ws-editor textarea{max-width:100%;min-width:0;color:var(--text);background:var(--surface-2);border:1px solid var(--border);border-radius:6px;padding:6px 9px;font:inherit}
  .rf-ws-editor{padding:10px;display:grid;gap:9px;min-width:0;margin:0}.rf-ws-editor button:disabled,.rf-ws-editbar button:disabled{opacity:.4;cursor:not-allowed}
  .rf-ws-catalog{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,150px),1fr));gap:7px}.rf-ws-catalog-card{text-align:left;display:grid;gap:5px;min-width:0;padding:10px;border:1px solid var(--border);background:var(--surface-2);color:var(--text);border-radius:6px;cursor:pointer}.rf-ws-catalog-card:hover{border-color:var(--accent)}.rf-ws-catalog-value{color:var(--accent);font-weight:700;overflow-wrap:anywhere}
  .rf-ws-transfer label{display:grid;gap:5px}.rf-ws-transfer textarea{width:100%;resize:vertical}.rf-ws-message{display:flex;gap:8px;align-items:center;flex-wrap:wrap;color:var(--warn);border:1px solid var(--border);border-radius:6px;padding:8px 10px}.rf-ws-message.critical{color:var(--bad)}
  .rf-ws-grid{display:grid;grid-template-columns:repeat(var(--ws-cols,4),minmax(0,1fr));grid-auto-rows:120px;gap:9px;position:relative;min-width:0}.rf-ws-grid.compact{grid-auto-rows:132px}
  .rf-ws-tile{container-type:inline-size;position:relative;min-width:0;min-height:0;display:flex;flex-direction:column;border:1px solid var(--border);border-radius:7px;background:var(--surface);--tile-accent:var(--accent);overflow:hidden}.rf-ws-tile.shortcut{background:var(--surface-2)}.rf-ws-tile.dragging{opacity:.55}.rf-ws-tile.selected{border-color:var(--accent)}.rf-ws-tile:hover{border-color:var(--accent)}
  .rf-ws-tile[data-accent='green']{--tile-accent:#37c65b}.rf-ws-tile[data-accent='blue']{--tile-accent:#459be5}.rf-ws-tile[data-accent='violet']{--tile-accent:#a78bfa}.rf-ws-tile[data-accent='amber']{--tile-accent:#d6a340}
  .rf-ws-link{display:flex;flex:1;flex-direction:column;justify-content:center;gap:6px;padding:11px;color:var(--text);text-decoration:none;min-width:0;min-height:0;overflow:hidden}.rf-ws-label{display:flex;gap:6px;align-items:center;color:var(--muted);font-size:12px;min-width:0;overflow-wrap:anywhere}.rf-ws-icon{color:var(--tile-accent,var(--accent));font-weight:700;font-size:12px}.rf-ws-value,.rf-ws-tx{font-size:clamp(15px,9cqi,30px);font-family:var(--font-mono,monospace);line-height:1.12;color:var(--tile-accent);overflow-wrap:anywhere}.rf-ws-tx{color:var(--muted);font-size:clamp(14px,7cqi,24px)}
  .rf-ws-link[data-state='unknown'] .rf-ws-value{color:var(--muted)}.rf-ws-link[data-state='warning'] .rf-ws-value{color:var(--warn)}.rf-ws-link[data-state='critical'] .rf-ws-value{color:var(--bad)}.rf-ws-arrow{align-self:flex-end;color:var(--muted)}
  .rf-ws-metric-body{display:flex;flex:1;min-height:0;min-width:0;flex-direction:column;justify-content:center;gap:5px}.rf-ws-traffic-values{display:flex;align-items:baseline;flex-wrap:wrap;gap:4px 12px}.rf-ws-traffic-values small{white-space:nowrap}.rf-ws-tile[data-size='1x1'] .rf-ws-traffic-values{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:2px 5px}.rf-ws-tile[data-size='1x1'] .rf-ws-traffic-values small{grid-column:1/-1}.rf-ws-tile[data-size='1x1'] .rf-ws-traffic-values strong{font-size:clamp(13px,7cqi,22px)}
  .rf-ws-meter{width:100%;height:4px;flex:none;background:var(--surface-2);border-radius:4px;overflow:hidden}.rf-ws-meter span{display:block;background:var(--tile-accent);height:100%}.rf-ws-chart{display:block;width:100%;height:0;min-height:18px;flex:1 1 0;color:var(--tile-accent);overflow:visible}.rf-ws-tile[data-size='2x1'] .rf-ws-chart{min-height:18px;flex-basis:18px}.rf-ws-tile[data-size='4x2'] .rf-ws-chart,.rf-ws-tile[data-size='2x2'] .rf-ws-chart{min-height:42px}.rf-ws-tile[data-size='2x1'] .rf-ws-metric-body:has(.rf-ws-chart){display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);grid-auto-rows:min-content;align-content:center;gap:3px 10px}.rf-ws-tile[data-size='2x1'] .rf-ws-metric-body:has(.rf-ws-chart) > :not(svg){grid-column:1}.rf-ws-tile[data-size='2x1'] .rf-ws-chart{grid-column:2;grid-row:1/span 3;align-self:center;height:46px}.rf-ws-chart-caption{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.rf-ws-tile[data-size='2x1'] .rf-ws-value{font-size:clamp(16px,6cqi,23px)}.rf-ws-tile[data-size='2x1'] .rf-ws-metric-body small{font-size:10px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .rf-ws-rows{display:grid;gap:2px;font-size:11px;color:var(--muted);min-width:0;overflow:hidden}.rf-ws-rows span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .rf-ws-panel-content{padding:10px;display:flex;flex-direction:column;gap:6px;min-height:0;min-width:0;flex:1;overflow:hidden}.rf-ws-panel-head{display:flex;justify-content:space-between;gap:8px;align-items:center;color:var(--text);font-size:13px}.rf-ws-panel-head span{color:var(--tile-accent);font-family:var(--font-mono,monospace)}.rf-ws-summary-list{display:grid;gap:4px}.rf-ws-summary-list a,.rf-ws-module-list a{display:flex;align-items:center;gap:6px;min-width:0;text-decoration:none;color:var(--text);font-size:12px;border-radius:4px;background:var(--surface-2);padding:6px}.rf-ws-summary-list a span:nth-child(2),.rf-ws-module-list a span:nth-child(2){flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.rf-ws-module-list{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,125px),1fr));gap:4px}.rf-ws-dot{width:7px;height:7px;border-radius:50%;background:var(--muted);flex:none}.rf-ws-dot[data-state='good']{background:var(--good)}.rf-ws-dot[data-state='warning'],.rf-ws-dot[data-state='warn']{background:var(--warn)}.rf-ws-dot[data-state='critical'],.rf-ws-dot[data-state='error']{background:var(--bad)}.rf-ws-more{font-size:11px;color:var(--accent);cursor:pointer;text-align:left;align-self:flex-start;background:none;border:0;padding:0}.rf-ws-detail-section{display:grid;gap:8px;min-width:0;padding:10px;border:1px solid var(--border);border-radius:7px}.rf-ws-detail-section .rf-ws-actions{justify-content:space-between}
  .rf-ws-controls.rf-ws-handle,.rf-ws-resize{touch-action:none;user-select:none;position:absolute;z-index:2;cursor:grab;color:var(--accent);border:0;background:transparent}.rf-ws-handle{inset:0;width:100%;height:100%;text-align:right;padding:4px;display:flex;align-items:flex-start;justify-content:flex-end}.rf-ws-grid.compact .rf-ws-handle{inset:4px 4px auto auto;width:36px;height:32px;padding:0}.rf-ws-handle span{background:var(--surface-2);border:1px solid var(--border);padding:2px 5px;border-radius:4px;opacity:.7}.rf-ws-handle:hover span,.rf-ws-handle:focus-visible span{opacity:1}.rf-ws-resize{right:0;bottom:0;cursor:nwse-resize;padding:5px;background:var(--surface-2);border:1px solid var(--border);border-radius:5px 0 0 0;z-index:3}.rf-ws-preview{z-index:4;border:2px dashed var(--accent);background:color-mix(in srgb,var(--accent) 12%,transparent);border-radius:7px;pointer-events:none}
  .rf-workspace button:focus-visible,.rf-workspace a:focus-visible,.rf-workspace select:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}.rf-ws-tile-settings label{display:flex;align-items:center;gap:4px}.rf-ws-tile-settings input[type='checkbox']{width:auto}
  @container (max-width:220px){.rf-ws-tile[data-size='2x1'] .rf-ws-metric-body:has(.rf-ws-chart){display:flex}.rf-ws-tile[data-size='2x1'] .rf-ws-chart{height:0;flex:1 1 18px;align-self:stretch}.rf-ws-link{padding:8px;gap:4px}.rf-ws-label{font-size:11px}.rf-ws-traffic-values{gap:3px 6px}.rf-ws-panel-content{padding:7px}.rf-ws-module-list{grid-template-columns:1fr}}
  @media(max-width:420px){.rf-ws-toolbar select{width:100%}}
</style>
