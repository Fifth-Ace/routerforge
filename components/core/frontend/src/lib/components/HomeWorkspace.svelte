<script>
  import { onMount } from 'svelte';
  import { WIDGETS, STORAGE_KEY, getWidget, defaultLayout, profileTiles, parseLayout, normalizeLayout, updateSpace, reorder, resize, boundedPercent, widgetFor, ACCENTS, COLS, MAX_ROWS, dimensions, placeTile, fitSize, nearestSize, spatialOrder, safeExternal, recordHistory, chartSegments } from '$lib/workspace.js';
  export let locale='ru';
  export let metrics={};
  export let sampleKeys={};
  export let ready=false;
  export let online=false;
  export let issueCount=0;
  export let criticalCount=0;
  export let attention;
  export let modules;
  let layout=defaultLayout(), editing=false, adding=false, transfer=false, transferText='', error='', persisted=true, loaded=false, dragId='', profile='default', name='';
  let grid, gridWidth=0, gesture=null, preview=null, selected='', tileTitle='', tileAccent='auto', tileDetail=true, linkTitle='', linkURL='', history={}, historyKeys={};
  $: columns=gridWidth>=720?4:gridWidth>=400?2:1;
  $: ordered=spatialOrder(active.tiles);
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
    try{const raw=localStorage.getItem(STORAGE_KEY);if(raw)layout=parseLayout(raw);}catch{error='restore';}
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
  function commit(next){if(gesture)cancelPointer();layout=normalizeLayout(next);if(loaded){try{localStorage.setItem(STORAGE_KEY,JSON.stringify(layout));persisted=true;}catch{persisted=false;}}}
  function change(fn){try{commit(updateSpace(layout,active.id,fn));error='';}catch{error='invalid';}}
  function selectSpace(id){commit({...layout,active:id});name='';error='';selected='';cancelPointer();}
  function addSpace(copy=false){if(layout.spaces.length>=8)return;const id='space-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8);const label=name.trim().slice(0,64);try{commit({...layout,active:id,spaces:[...layout.spaces,{id,name:label,profile,tiles:copy?active.tiles.map(x=>({...x})):profileTiles(profile)}]});name='';selected='';error='';}catch{error='invalid';}}
  function deleteSpace(){if(layout.spaces.length<=1||!confirm(tr('Удалить рабочее пространство?','Delete workspace?',en)))return;selected='';const spaces=layout.spaces.filter(s=>s.id!==active.id);commit({...layout,spaces,active:spaces[0].id});}
  function rename(){try{change(s=>({...s,name:name.trim().slice(0,64)}));name='';error='';}catch{error='invalid';}}
  function add(id){const w=getWidget(id);if(!w||active.tiles.some(t=>t.id===id))return;change(s=>({...s,tiles:[...s.tiles,{id,size:w.size}]}));}
  function remove(id){change(s=>({...s,tiles:s.tiles.filter(t=>t.id!==id)}));}
  function move(id,d){const list=spatialOrder(active.tiles),i=list.findIndex(t=>t.id===id),other=list[i+d];if(other)change(s=>({...s,tiles:reorder(s.tiles,id,other.id)}));}
  function selectTile(tile){selected=tile.id;tileTitle=tile.title||'';tileAccent=tile.accent||'auto';tileDetail=tile.showDetail!==false;}
  function saveTile(){if(!selectedTile)return;change(s=>({...s,tiles:s.tiles.map(t=>t.id===selected?{...t,title:tileTitle,accent:tileAccent,showDetail:tileDetail}:t)}));}
  function addLink(){try{const url=safeExternal(linkURL);if(active.tiles.filter(t=>widgetFor(t).type==='external').length>=12)throw Error('LINK_LIMIT');const id='link-'+Date.now().toString(36)+Math.random().toString(36).slice(2,8);commit(updateSpace(layout,active.id,s=>({...s,tiles:[...s.tiles,{id,size:'1x1',url,title:linkTitle.trim()||new URL(url).hostname.slice(0,64)}]})));linkTitle='';linkURL='';error='';}catch{error='invalid';}}
  function positionStyle(tile,columns){if(columns!==4){const [w,h]=dimensions(tile.size);return `grid-column:span ${Math.min(w,columns)};grid-row:span ${columns===1?1:Math.min(h,2)};`;}const [w,h]=dimensions(tile.size);return `grid-column:${tile.x+1}/span ${w};grid-row:${tile.y+1}/span ${h};`;}
  function nudge(e,tile){const delta={ArrowLeft:[-1,0],ArrowRight:[1,0],ArrowUp:[0,-1],ArrowDown:[0,1]}[e.key];if(!delta)return;e.preventDefault();if(columns!==4){move(tile.id,delta[0]||delta[1]);return;}const [w,h]=dimensions(tile.size);const x=Math.max(0,Math.min(COLS-w,tile.x+delta[0])),y=Math.max(0,Math.min(MAX_ROWS-h,tile.y+delta[1]));change(s=>({...s,tiles:placeTile(s.tiles,tile.id,x,y)}));}
  function beginPointer(e,tile,kind){
    if(!editing||e.button!==0||e.isPrimary===false||gesture)return;
    e.preventDefault();
    const handle=e.currentTarget;
    gesture={id:tile.id,kind,pointer:e.pointerId,x:e.clientX,y:e.clientY,tile:{...tile},columns,handle,layout};
    dragId=tile.id;preview=columns===4?{...tile}:null;
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
    if(columns!==4)return;
    const t=gesture.tile,[w,h]=dimensions(t.size),css=getComputedStyle(grid);
    const pitchX=(grid.getBoundingClientRect().width+parseFloat(css.columnGap))/4,pitchY=parseFloat(css.gridAutoRows)+parseFloat(css.rowGap);
    if(!Number.isFinite(pitchX)||!Number.isFinite(pitchY)||pitchX<=0||pitchY<=0){cancelPointer();return;}
    const dx=Math.round((e.clientX-gesture.x)/pitchX),dy=Math.round((e.clientY-gesture.y)/pitchY);
    if(gesture.kind==='resize'){const size=nearestSize(widgetFor(t),Math.max(1,w+dx),Math.max(1,h+dy)),[nw,nh]=dimensions(size);preview={...t,size,x:Math.min(t.x,COLS-nw),y:Math.min(t.y,MAX_ROWS-nh)};}
    else preview={...t,x:Math.max(0,Math.min(COLS-w,t.x+dx)),y:Math.max(0,Math.min(MAX_ROWS-h,t.y+dy))};
  }
  function cancelPointer(){
    const g=gesture;gesture=null;preview=null;dragId='';
    if(g){try{if(g.handle.hasPointerCapture(g.pointer))g.handle.releasePointerCapture(g.pointer);}catch{/* Detached handles do not retain capture. */}}
  }
  function abortPointer(e){if(gesture?.pointer===e.pointerId)cancelPointer();}
  function escapePointer(e){if(gesture&&e.key==='Escape'){e.preventDefault();cancelPointer();}}
  function blurPointer(e){if(e.target===window)cancelPointer();}
  function visibilityPointer(){if(document.hidden)cancelPointer();}
  function endPointer(e){
    if(!validGesture(e))return;
    const g=gesture;updatePointer(e);if(!gesture)return;
    const p=preview;
    const target=columns!==4&&g.kind==='move'?document.elementFromPoint(e.clientX,e.clientY)?.closest('[data-widget]')?.getAttribute('data-widget'):null;
    cancelPointer();
    if(columns===4&&p)change(s=>({...s,tiles:placeTile(s.tiles,g.id,p.x,p.y,p.size)}));
    else if(target&&target!==g.id)change(s=>({...s,tiles:reorder(s.tiles,g.id,target)}));
  }
  function importLayout(){try{const next=parseLayout(transferText);if(!confirm(tr('Заменить все рабочие пространства?','Replace all workspaces?',en)))return;commit(next);selected='';transfer=false;error='';}catch{error='invalid';}}
  function exportLayout(){transferText=JSON.stringify(layout,null,2);transfer=true;error='';}
  function reset(){if(!confirm(tr('Заменить текущую компоновку шаблоном?','Replace current layout with template?',en)))return;selected='';change(s=>({...s,profile,tiles:profileTiles(profile)}));}
  function valueFor(id,data){const v=data[id];return v&&typeof v.value==='string'?v:{value:'—',detail:'',percent:null,state:'unknown'};}
</script>
<section class="rf-workspace" aria-label={tr('Рабочее пространство','Workspace',en)}>
  <div class="rf-ws-toolbar">
    <div class="rf-ws-heading"><strong>RouterForge Workspace</strong><small>{tr('Ваше рабочее пространство','Your workspace',en)}</small></div>
    <select aria-label={tr('Рабочее пространство','Workspace',en)} value={layout.active} onchange={e=>selectSpace(e.currentTarget.value)}>
      {#each layout.spaces as space (space.id)}<option value={space.id}>{space.name||profileLabel(space.profile,en)}</option>{/each}
    </select>
    <button type="button" class="button" onclick={()=>{editing=!editing;adding=false;transfer=false;selected='';cancelPointer();}}>{editing?tr('Готово','Done',en):tr('Настроить','Customize',en)}</button>
  </div>
  {#if !ready || !online}<div class="rf-ws-message" role="status">{!ready?tr('Получение данных Core…','Loading Core data…',en):tr('Core недоступен; значения могут быть устаревшими.','Core unavailable; values may be stale.',en)}</div>{/if}
  {#if issueCount>0}<div class="rf-ws-message" class:critical={criticalCount>0} role="status"><strong>{tr('Требует внимания','Needs attention',en)}: {issueCount}</strong>{#if !active.tiles.some(t=>t.id==='attention')}<button class="button" type="button" onclick={()=>add('attention')}>{tr('Показать предупреждения','Show warnings',en)}</button>{/if}</div>{/if}
  {#if error || !persisted}<div class="rf-ws-message" role="alert">{!persisted?tr('Браузер не сохранил компоновку.','Browser could not save layout.',en):error==='restore'?tr('Сохранённая компоновка повреждена; загружен стандартный профиль.','Saved layout is invalid; default profile loaded.',en):tr('Некорректная компоновка или имя.','Invalid layout or name.',en)}</div>{/if}
  {#if editing}
    <div class="rf-ws-editor panel">
      <div class="rf-ws-actions">
        <button type="button" class="button primary" onclick={()=>adding=!adding}>{tr('＋ Плитка','＋ Tile',en)}</button>
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
      {#if adding}<div class="rf-ws-catalog">{#each WIDGETS.filter(w=>!active.tiles.some(t=>t.id===w.id)) as w (w.id)}<button type="button" class="button" onclick={()=>add(w.id)}>＋ {title(w,en)}</button>{:else}<small>{tr('Все плитки добавлены','All tiles added',en)}</small>{/each}</div>{/if}
      {#if transfer}<div class="rf-ws-transfer"><label>{tr('JSON компоновки (без секретов)','Layout JSON (no secrets)',en)}<textarea rows="8" bind:value={transferText} maxlength="65536" spellcheck="false"></textarea></label><button class="button primary" type="button" onclick={importLayout}>{tr('Импортировать JSON','Import JSON',en)}</button></div>{/if}
      {#if adding}<div class="rf-ws-actions"><input aria-label={tr('Название ярлыка','Shortcut title',en)} placeholder={tr('Название ярлыка','Shortcut title',en)} maxlength="64" bind:value={linkTitle}/><input aria-label={tr('Адрес HTTP(S)','HTTP(S) address',en)} placeholder="https://example.net/" maxlength="512" bind:value={linkURL}/><button class="button" type="button" onclick={addLink}>{tr('Добавить внешний ярлык','Add external shortcut',en)}</button><small>{tr('Без логинов, токенов и секретных путей. Открывается только по клику.','No credentials, tokens or secret paths. Opens only when clicked.',en)}</small></div>{/if}
      {#if selectedTile}<div class="rf-ws-actions rf-ws-tile-settings"><strong>{title(widgetFor(selectedTile),en)}</strong><input aria-label={tr('Заголовок плитки','Tile title',en)} maxlength="64" placeholder={tr('Заголовок плитки','Tile title',en)} bind:value={tileTitle}/><select aria-label={tr('Акцент плитки','Tile accent',en)} bind:value={tileAccent}>{#each ACCENTS as color}<option value={color}>{({auto:tr('Тема','Theme',en),green:tr('Зелёный','Green',en),blue:tr('Синий','Blue',en),violet:tr('Фиолетовый','Violet',en),amber:tr('Янтарный','Amber',en)})[color]}</option>{/each}</select><select aria-label={tr('Размер плитки','Tile size',en)} value={selectedTile.size} onchange={e=>change(s=>({...s,tiles:fitSize(s.tiles,selectedTile.id,e.currentTarget.value)}))}>{#each widgetFor(selectedTile).sizes as size}<option value={size}>{size}</option>{/each}</select><label><input type="checkbox" bind:checked={tileDetail}/> {tr('Подробности','Details',en)}</label><button class="button primary" type="button" onclick={saveTile}>{tr('Сохранить плитку','Save tile',en)}</button><button class="button" type="button" onclick={()=>selected=''}>{tr('Закрыть','Close',en)}</button></div>{/if}
      <small>{tr('Перемещайте за ручку; стрелки клавиатуры на ручке меняют позицию. Размер — за нижний угол или через настройки плитки.','Move using the handle or its arrow keys. Resize using the bottom corner or tile settings.',en)}</small>
    </div>
  {/if}
  <div class="rf-ws-grid" class:editing bind:this={grid} style={`--ws-cols:${columns}`} class:compact={columns!==4}>
    {#each ordered as tile,i (tile.id)}
      {@const w=widgetFor(tile)}
      {@const data=valueFor(tile.id,metrics)}
      {@const label=tile.title||title(w,en)}
      {@const segments=chartSegments(history[tile.id],tile.id)}
      {@const rxSegments=tile.id==='traffic'?chartSegments(history['traffic-rx'],'traffic-rx',trafficMax):[]}
      {@const txSegments=tile.id==='traffic'?chartSegments(history['traffic-tx'],'traffic-tx',trafficMax):[]}
      <article class="rf-ws-tile" class:dragging={dragId===tile.id} class:shortcut={w.type==='shortcut'||w.type==='external'} data-widget={tile.id} data-accent={tile.accent||'auto'} style={positionStyle(tile,columns)}>
        {#if editing}
          <div class="rf-ws-controls">
            <button type="button" class="rf-ws-handle" aria-label={tr('Перетащить','Drag',en)+' '+label} onpointerdown={e=>beginPointer(e,tile,'move')} onkeydown={e=>nudge(e,tile)}>⠿</button>
            <button type="button" aria-label={tr('Настройки плитки','Tile settings',en)+' '+label} onclick={()=>selectTile(tile)}>⚙</button>
            <button type="button" aria-label={tr('Изменить размер','Resize',en)+' '+label} onclick={()=>change(s=>({...s,tiles:resize(s.tiles,tile.id)}))}>{tile.size}</button>
            <button type="button" aria-label={tr('Удалить плитку','Remove tile',en)+' '+label} onclick={()=>{if(selected===tile.id)selected='';remove(tile.id);}}>×</button>
          </div>
        {/if}
        {#if w.type==='panel'}
          <div class="rf-ws-panel-content">{#if tile.title}<strong class="rf-ws-custom-title">{tile.title}</strong>{/if}{#if w.id==='attention'}{@render attention()}{:else}{@render modules()}{/if}</div>
        {:else if w.type==='external'}
          <a class="rf-ws-link" href={tile.url} target="_blank" rel="noopener noreferrer" onclick={e=>{if(editing)e.preventDefault();}}><span class="rf-ws-icon">↗</span><strong>{label}</strong><small>{new URL(tile.url).hostname}</small></a>
        {:else if w.type==='shortcut'}
          <a class="rf-ws-link" href={w.route} onclick={e=>{if(editing)e.preventDefault();}}><span class="rf-ws-icon">{w.icon}</span><strong>{label}</strong><span class="rf-ws-arrow">↗</span></a>
        {:else}
          <a class="rf-ws-link" href={w.route} onclick={e=>{if(editing)e.preventDefault();}} data-state={data.state||'unknown'}>
            <span class="rf-ws-label"><span class="rf-ws-icon">{w.icon}</span>{label}</span>
            <strong class="rf-ws-value">{online||tile.id==='core'?data.value:'—'}</strong>
            {#if online && tile.showDetail!==false && tile.size!=='1x1' && data.detail}<small>{data.detail}</small>{/if}
            {#if online && tile.showDetail!==false && tile.size!=='1x1' && boundedPercent(data.percent)!==null}<div class="rf-ws-meter" aria-hidden="true"><span style:width={`${data.percent}%`}></span></div>{/if}
            {#if online && tile.showDetail!==false && tile.size.endsWith('2') && tile.id==='traffic' && (rxSegments.length||txSegments.length)}<svg class="rf-ws-chart" viewBox="0 0 200 42" role="img" aria-label={tr('Получение и отправка за сеанс','Session receive and transmit',en)}>{#each rxSegments as points}<polyline {points} fill="none" stroke="currentColor" stroke-width="1.5"/>{/each}{#each txSegments as points}<polyline {points} fill="none" stroke="var(--muted)" stroke-width="1.5"/>{/each}</svg><small>{tr('RX — акцент · TX — серый','RX — accent · TX — muted',en)}</small>{/if}
            {#if online && tile.showDetail!==false && tile.size.endsWith('2') && segments.length}<svg class="rf-ws-chart" viewBox="0 0 200 42" role="img" aria-label={tr('История наблюдений за текущий сеанс','Observation history for this session',en)}>{#each segments as points}<polyline {points} fill="none" stroke="currentColor" stroke-width="1.5" vector-effect="non-scaling-stroke"/>{/each}</svg><small>{tr('Отсчётов за сеанс','Session samples',en)}: {history[tile.id].length}</small>{/if}
            {#if online && tile.showDetail!==false && tile.size.endsWith('2') && Array.isArray(data.rows)}<div class="rf-ws-rows">{#each data.rows.slice(0,8) as row}<span>{row}</span>{/each}</div>{/if}
            {#if !online}<small>{tr('Нет актуальных данных','No current data',en)}</small>{/if}
          </a>
        {/if}
        {#if editing && columns===4}<button type="button" class="rf-ws-resize" aria-label={tr('Изменить размер за угол','Resize by corner',en)+' '+label} onpointerdown={e=>beginPointer(e,tile,'resize')}>◢</button>{/if}
      </article>
    {:else}<div class="rf-ws-message">{tr('Добавьте первую плитку в режиме настройки.','Add your first tile in customize mode.',en)}</div>{/each}
    {#if preview&&columns===4}<div class="rf-ws-preview" style={positionStyle(preview,columns)} aria-hidden="true"></div>{/if}
  </div>
  {#if editing}<button type="button" class="rf-ws-add" onclick={()=>adding=true}>{tr('＋ Добавить плитку','＋ Add tile',en)}</button>{/if}
</section>
<style>
  .rf-workspace{display:grid;gap:12px;min-width:0}.rf-ws-toolbar,.rf-ws-actions{display:flex;align-items:center;flex-wrap:wrap;gap:8px;min-width:0}.rf-ws-heading{display:grid;gap:3px;flex:1;min-width:150px}.rf-ws-heading strong{font-size:17px}.rf-ws-heading small,.rf-ws-editor small,.rf-ws-link small{color:var(--muted);font-size:12px}.rf-ws-toolbar select,.rf-ws-editor select,.rf-ws-editor input,.rf-ws-editor textarea{max-width:100%;min-width:0;color:var(--text);background:var(--surface-2);border:1px solid var(--border);border-radius:6px;padding:8px 10px;font:inherit}.rf-ws-editor{padding:12px;display:grid;gap:9px;min-width:0}.rf-ws-catalog{display:flex;flex-wrap:wrap;gap:7px}.rf-ws-transfer label{display:grid;gap:5px}.rf-ws-transfer textarea{width:100%;resize:vertical;font-family:var(--font-mono,monospace)}.rf-ws-transfer .button{margin-top:8px}.rf-ws-message{display:flex;gap:8px;align-items:center;flex-wrap:wrap;color:var(--warn);border:1px solid var(--border);border-radius:6px;padding:8px 12px;font-size:12px}.rf-ws-message.critical{color:var(--bad)}
  .rf-ws-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));grid-auto-rows:minmax(116px,auto);gap:9px;min-width:0}.rf-ws-tile{min-width:0;min-height:116px;display:flex;flex-direction:column;gap:6px;background:var(--surface);border:1px solid var(--border);border-radius:6px;overflow:hidden}.rf-ws-tile.shortcut{background:var(--surface-2)}.rf-ws-tile.dragging{opacity:.5}.rf-ws-tile:hover{border-color:var(--accent)}.rf-ws-link{display:flex;flex:1;flex-direction:column;justify-content:space-between;align-items:flex-start;gap:9px;padding:13px;color:var(--text);text-decoration:none;min-width:0;overflow-wrap:anywhere}.rf-ws-label{display:flex;gap:7px;align-items:center;color:var(--muted);font-size:12px;flex-wrap:wrap}.rf-ws-icon{display:inline-flex;min-width:22px;color:var(--accent);font-weight:700;font-size:12px;letter-spacing:.02em}.rf-ws-value{font-size:clamp(18px,2.2vw,28px);font-family:var(--font-mono,monospace);line-height:1.15;color:var(--accent)}.rf-ws-link[data-state='unknown'] .rf-ws-value{color:var(--muted)}.rf-ws-link[data-state='warning'] .rf-ws-value{color:var(--warn)}.rf-ws-link[data-state='critical'] .rf-ws-value{color:var(--bad)}.rf-ws-arrow{align-self:flex-end;color:var(--muted)}.rf-ws-meter{width:100%;height:5px;background:var(--surface-2);border:1px solid var(--border);border-radius:4px;overflow:hidden}.rf-ws-meter span{display:block;background:var(--accent);height:100%}.rf-ws-rows{display:grid;gap:4px;font-size:11px;color:var(--muted);width:100%}.rf-ws-panel-content{padding:8px;min-width:0;flex:1;overflow:auto;max-height:520px}.rf-ws-controls{display:flex;gap:3px;flex-wrap:wrap;padding:6px;border-bottom:1px solid var(--border);background:var(--surface-2)}.rf-ws-controls button,.rf-ws-add{font:inherit;color:var(--text);background:var(--surface-2);border:1px solid var(--border);border-radius:4px;padding:5px 7px;cursor:pointer}.rf-ws-controls button:disabled{opacity:.35;cursor:not-allowed}.rf-ws-add{border-style:dashed;min-height:116px;color:var(--accent)}.editing .rf-ws-tile{border-color:var(--muted)}.rf-workspace button:focus-visible,.rf-workspace a:focus-visible,.rf-workspace select:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}
  .rf-ws-grid{grid-template-columns:repeat(var(--ws-cols,4),minmax(0,1fr));grid-auto-rows:116px;position:relative}.rf-ws-grid.editing:not(.compact){grid-auto-rows:148px}.rf-ws-grid.compact{grid-auto-rows:minmax(116px,auto)}.rf-ws-tile{position:relative;--tile-accent:var(--accent)}.rf-ws-tile[data-accent='green']{--tile-accent:#37c65b}.rf-ws-tile[data-accent='blue']{--tile-accent:#459be5}.rf-ws-tile[data-accent='violet']{--tile-accent:#a78bfa}.rf-ws-tile[data-accent='amber']{--tile-accent:#d6a340}.rf-ws-tile .rf-ws-icon,.rf-ws-tile .rf-ws-value,.rf-ws-chart{color:var(--tile-accent)}.rf-ws-meter span{background:var(--tile-accent)}.rf-ws-link[data-state='unknown'] .rf-ws-value{color:var(--muted)}.rf-ws-link[data-state='warning'] .rf-ws-value{color:var(--warn)}.rf-ws-link[data-state='critical'] .rf-ws-value{color:var(--bad)}.rf-ws-chart{width:100%;height:46px;flex-shrink:0}.rf-ws-controls{padding:3px;gap:2px;flex-shrink:0}.rf-ws-controls button{min-height:24px;font-size:11px;padding:2px 5px}.rf-ws-handle,.rf-ws-resize{touch-action:none;user-select:none}.rf-ws-handle{cursor:grab}.rf-ws-resize{position:absolute;right:0;bottom:0;background:var(--surface-2);color:var(--accent);border:1px solid var(--border);border-radius:5px 0 0 0;padding:4px;cursor:nwse-resize;z-index:2}.rf-ws-preview{z-index:3;border:2px dashed var(--accent);background:color-mix(in srgb,var(--accent) 14%,transparent);border-radius:6px;pointer-events:none}.rf-ws-add{min-height:44px}.rf-ws-link{overflow:auto;min-height:0}.rf-ws-tile[data-widget="core"] .rf-ws-link{overflow-wrap:normal}.rf-ws-link[data-state="unknown"] .rf-ws-value{color:var(--muted)}.rf-ws-custom-title{display:block;padding:3px 6px;color:var(--tile-accent)}.rf-ws-tile-settings{border-top:1px solid var(--border);padding-top:8px}.rf-ws-tile-settings label{display:flex;align-items:center;gap:4px}.rf-ws-tile-settings input[type='checkbox']{width:auto}.rf-ws-panel-content{min-height:0;max-height:none}
  @media(max-width:420px){.rf-ws-toolbar select{width:100%}}
</style>
