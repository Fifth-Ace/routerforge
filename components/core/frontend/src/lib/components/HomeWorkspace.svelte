<script>
  import { onMount } from 'svelte';
  import { WIDGETS, STORAGE_KEY, getWidget, defaultLayout, profileTiles, parseLayout, normalizeLayout, updateSpace, reorder, resize, boundedPercent } from '$lib/workspace.js';
  export let locale='ru';
  export let metrics={};
  export let ready=false;
  export let online=false;
  export let issueCount=0;
  export let criticalCount=0;
  export let attention;
  export let modules;
  let layout=defaultLayout(), editing=false, adding=false, transfer=false, transferText='', error='', persisted=true, loaded=false, dragId='', profile='default', name='';
  $: en=locale==='en';
  $: active=layout.spaces.find(s=>s.id===layout.active) || layout.spaces[0];
  const tr=(ru,enText,isEn)=>isEn?enText:ru;
  const profileLabel=(id,isEn)=>id==='network'?tr('Сеть','Network',isEn):id==='admin'?tr('Обслуживание','Maintenance',isEn):tr('Главная','Home',isEn);
  const title=(w,isEn)=>isEn?w.en:w.ru;
  onMount(()=>{
    try{const raw=localStorage.getItem(STORAGE_KEY);if(raw)layout=parseLayout(raw);}catch{error='restore';}
    loaded=true;
  });
  function commit(next){layout=normalizeLayout(next);if(loaded){try{localStorage.setItem(STORAGE_KEY,JSON.stringify(layout));persisted=true;}catch{persisted=false;}}}
  function change(fn){commit(updateSpace(layout,active.id,fn));}
  function selectSpace(id){commit({...layout,active:id});name='';error='';}
  function addSpace(copy=false){if(layout.spaces.length>=8)return;const id='space-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8);const label=name.trim().slice(0,64);try{commit({...layout,active:id,spaces:[...layout.spaces,{id,name:label,profile,tiles:copy?active.tiles.map(x=>({...x})):profileTiles(profile)}]});name='';error='';}catch{error='invalid';}}
  function deleteSpace(){if(layout.spaces.length<=1)return;const spaces=layout.spaces.filter(s=>s.id!==active.id);commit({...layout,spaces,active:spaces[0].id});}
  function rename(){try{change(s=>({...s,name:name.trim().slice(0,64)}));name='';error='';}catch{error='invalid';}}
  function add(id){const w=getWidget(id);if(!w||active.tiles.some(t=>t.id===id))return;change(s=>({...s,tiles:[...s.tiles,{id,size:w.size}]}));}
  function remove(id){change(s=>({...s,tiles:s.tiles.filter(t=>t.id!==id)}));}
  function move(id,d){const i=active.tiles.findIndex(t=>t.id===id),other=active.tiles[i+d];if(other)change(s=>({...s,tiles:reorder(s.tiles,id,other.id)}));}
  function drop(e,id){e.preventDefault();if(!editing||!dragId)return;change(s=>({...s,tiles:reorder(s.tiles,dragId,id)}));dragId='';}
  function importLayout(){try{commit(parseLayout(transferText));transfer=false;error='';}catch{error='invalid';}}
  function exportLayout(){transferText=JSON.stringify(layout,null,2);transfer=true;error='';}
  function reset(){change(s=>({...s,profile,tiles:profileTiles(profile)}));error='';}
  function valueFor(id,data){const v=data[id];return v&&typeof v.value==='string'?v:{value:'—',detail:'',percent:null,state:'unknown'};}
</script>
<section class="rf-workspace" aria-label={tr('Рабочее пространство','Workspace',en)}>
  <div class="rf-ws-toolbar">
    <div class="rf-ws-heading"><strong>RouterForge Workspace</strong><small>{tr('Ваше рабочее пространство','Your workspace',en)}</small></div>
    <select aria-label={tr('Рабочее пространство','Workspace',en)} value={layout.active} onchange={e=>selectSpace(e.currentTarget.value)}>
      {#each layout.spaces as space (space.id)}<option value={space.id}>{space.name||profileLabel(space.profile,en)}</option>{/each}
    </select>
    <button type="button" class="button" onclick={()=>{editing=!editing;adding=false;transfer=false;dragId='';}}>{editing?tr('Готово','Done',en):tr('Настроить','Customize',en)}</button>
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
      <small>{tr('Перетащите за ручку или используйте стрелки. Размер меняется кнопкой с текущим размером.','Drag the handle or use arrows. Use the size button to resize.',en)}</small>
    </div>
  {/if}
  <div class="rf-ws-grid" class:editing>
    {#each active.tiles as tile,i (tile.id)}
      {@const w=getWidget(tile.id)}
      {@const data=valueFor(tile.id,metrics)}
      <article class="rf-ws-tile" class:dragging={dragId===tile.id} class:shortcut={w.type==='shortcut'} class:wide={tile.size.startsWith('4')} class:medium={tile.size.startsWith('2')} class:tall={tile.size.endsWith('2')} class:large={tile.size.endsWith('4')} data-widget={tile.id} ondragover={e=>{if(editing&&dragId)e.preventDefault();}} ondrop={e=>drop(e,tile.id)}>
        {#if editing}
          <div class="rf-ws-controls">
            <button type="button" draggable="true" aria-label={tr('Перетащить','Drag',en)+' '+title(w,en)} ondragstart={e=>{dragId=tile.id;e.dataTransfer.setData('text/plain',tile.id);e.dataTransfer.effectAllowed='move';}} ondragend={()=>dragId=''}>⠿</button>
            <button type="button" disabled={i===0} aria-label={tr('Переместить раньше','Move earlier',en)} onclick={()=>move(tile.id,-1)}>←</button>
            <button type="button" disabled={i===active.tiles.length-1} aria-label={tr('Переместить позже','Move later',en)} onclick={()=>move(tile.id,1)}>→</button>
            <button type="button" aria-label={tr('Изменить размер','Resize',en)+' '+title(w,en)} onclick={()=>change(s=>({...s,tiles:resize(s.tiles,tile.id)}))}>{tile.size}</button>
            <button type="button" aria-label={tr('Удалить плитку','Remove tile',en)+' '+title(w,en)} onclick={()=>remove(tile.id)}>×</button>
          </div>
        {/if}
        {#if w.type==='panel'}
          <div class="rf-ws-panel-content">{#if w.id==='attention'}{@render attention()}{:else}{@render modules()}{/if}</div>
        {:else if w.type==='shortcut'}
          <a class="rf-ws-link" href={w.route}><span class="rf-ws-icon">{w.icon}</span><strong>{title(w,en)}</strong><span class="rf-ws-arrow">↗</span></a>
        {:else}
          <a class="rf-ws-link" href={w.route} data-state={data.state||'unknown'}>
            <span class="rf-ws-label"><span class="rf-ws-icon">{w.icon}</span>{title(w,en)}</span>
            <strong class="rf-ws-value">{online?data.value:'—'}</strong>
            {#if online && tile.size!=='1x1' && data.detail}<small>{data.detail}</small>{/if}
            {#if online && tile.size!=='1x1' && boundedPercent(data.percent)!==null}<div class="rf-ws-meter" aria-hidden="true"><span style:width={`${data.percent}%`}></span></div>{/if}
            {#if online && (tile.size==='2x2'||tile.size==='4x2') && Array.isArray(data.rows)}<div class="rf-ws-rows">{#each data.rows.slice(0,8) as row}<span>{row}</span>{/each}</div>{/if}
            {#if !online}<small>{tr('Нет актуальных данных','No current data',en)}</small>{/if}
          </a>
        {/if}
      </article>
    {:else}<div class="rf-ws-message">{tr('Добавьте первую плитку в режиме настройки.','Add your first tile in customize mode.',en)}</div>{/each}
    {#if editing}<button type="button" class="rf-ws-add" onclick={()=>adding=true}>{tr('＋ Добавить плитку','＋ Add tile',en)}</button>{/if}
  </div>
</section>
<style>
  .rf-workspace{display:grid;gap:12px;min-width:0}.rf-ws-toolbar,.rf-ws-actions{display:flex;align-items:center;flex-wrap:wrap;gap:8px;min-width:0}.rf-ws-heading{display:grid;gap:3px;flex:1;min-width:150px}.rf-ws-heading strong{font-size:17px}.rf-ws-heading small,.rf-ws-editor small,.rf-ws-link small{color:var(--muted);font-size:12px}.rf-ws-toolbar select,.rf-ws-editor select,.rf-ws-editor input,.rf-ws-editor textarea{max-width:100%;min-width:0;color:var(--text);background:var(--surface-2);border:1px solid var(--border);border-radius:6px;padding:8px 10px;font:inherit}.rf-ws-editor{padding:12px;display:grid;gap:9px;min-width:0}.rf-ws-catalog{display:flex;flex-wrap:wrap;gap:7px}.rf-ws-transfer label{display:grid;gap:5px}.rf-ws-transfer textarea{width:100%;resize:vertical;font-family:var(--font-mono,monospace)}.rf-ws-transfer .button{margin-top:8px}.rf-ws-message{display:flex;gap:8px;align-items:center;flex-wrap:wrap;color:var(--warn);border:1px solid var(--border);border-radius:6px;padding:8px 12px;font-size:12px}.rf-ws-message.critical{color:var(--bad)}
  .rf-ws-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));grid-auto-rows:minmax(116px,auto);gap:9px;min-width:0}.rf-ws-tile{min-width:0;min-height:116px;display:flex;flex-direction:column;gap:6px;background:var(--surface);border:1px solid var(--border);border-radius:6px;overflow:hidden}.rf-ws-tile.medium{grid-column:span 2}.rf-ws-tile.wide{grid-column:span 4}.rf-ws-tile.tall{grid-row:span 2}.rf-ws-tile.large{grid-row:span 4}.rf-ws-tile.shortcut{background:var(--surface-2)}.rf-ws-tile.dragging{opacity:.5}.rf-ws-tile:hover{border-color:var(--accent)}.rf-ws-link{display:flex;flex:1;flex-direction:column;justify-content:space-between;align-items:flex-start;gap:9px;padding:13px;color:var(--text);text-decoration:none;min-width:0;overflow-wrap:anywhere}.rf-ws-label{display:flex;gap:7px;align-items:center;color:var(--muted);font-size:12px;flex-wrap:wrap}.rf-ws-icon{display:inline-flex;min-width:22px;color:var(--accent);font-weight:700;font-size:12px;letter-spacing:.02em}.rf-ws-value{font-size:clamp(18px,2.2vw,28px);font-family:var(--font-mono,monospace);line-height:1.15;color:var(--accent)}.rf-ws-link[data-state='unknown'] .rf-ws-value{color:var(--muted)}.rf-ws-link[data-state='warning'] .rf-ws-value{color:var(--warn)}.rf-ws-link[data-state='critical'] .rf-ws-value{color:var(--bad)}.rf-ws-arrow{align-self:flex-end;color:var(--muted)}.rf-ws-meter{width:100%;height:5px;background:var(--surface-2);border:1px solid var(--border);border-radius:4px;overflow:hidden}.rf-ws-meter span{display:block;background:var(--accent);height:100%}.rf-ws-rows{display:grid;gap:4px;font-size:11px;color:var(--muted);width:100%}.rf-ws-panel-content{padding:8px;min-width:0;flex:1;overflow:auto;max-height:520px}.rf-ws-tile.tall .rf-ws-panel-content{max-height:300px}.rf-ws-controls{display:flex;gap:3px;flex-wrap:wrap;padding:6px;border-bottom:1px solid var(--border);background:var(--surface-2)}.rf-ws-controls button,.rf-ws-add{font:inherit;color:var(--text);background:var(--surface-2);border:1px solid var(--border);border-radius:4px;padding:5px 7px;cursor:pointer}.rf-ws-controls button:disabled{opacity:.35;cursor:not-allowed}.rf-ws-controls button[draggable]{cursor:grab}.rf-ws-add{border-style:dashed;min-height:116px;color:var(--accent)}.editing .rf-ws-tile{border-color:var(--muted)}.rf-workspace button:focus-visible,.rf-workspace a:focus-visible,.rf-workspace select:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}
  @media(max-width:900px){.rf-ws-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.rf-ws-tile.wide{grid-column:span 2}}
  @media(max-width:420px){.rf-ws-grid{grid-template-columns:minmax(0,1fr)}.rf-ws-tile.medium,.rf-ws-tile.wide{grid-column:span 1}.rf-ws-tile.tall,.rf-ws-tile.large{grid-row:span 1}.rf-ws-toolbar select{width:100%}}
</style>
