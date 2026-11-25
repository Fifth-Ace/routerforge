<script>
  import { onMount } from 'svelte';
  import FoundationBindings from '$lib/components/FoundationBindings.svelte';
  import { settings } from '$lib/stores/settings.js';
  import { markUnauthorized } from '$lib/stores/auth.js';
  const base = '/api/platform/foundation';
  const types = ['network','domain-group','port-group','device','interface','gateway'];
  const deviceKinds = ['router','linux','nas','web','other'];
  const memberKinds = {network:['ip','cidr','object'],'domain-group':['domain','list-ref','object'],'port-group':['port','object'],device:['device','object'],interface:['object'],gateway:['object']};
  let locale = 'ru', state = null, draft = null, preview = null, busy = false, error = '', notice = '', tab = 'groups', group = null, device = null, filter = '', jsonText = '', events = [], eventFilter = '', eventSeverity = '', expanded = '';
  $: locale = $settings.locale || 'ru';
  let tr;
  $: tr = locale === 'en' ? (ru,en) => en : (ru,en) => ru;
  const clone = x => JSON.parse(JSON.stringify(x));
  $: filtered = (draft?.policyObjects || []).filter(x => `${x.name} ${x.id} ${x.type} ${(x.tags||[]).join(" ")} ${x.description||""}`.toLowerCase().includes(filter.toLowerCase()));
  $: dirty = !!state && !!draft && JSON.stringify(state.current.catalog) !== JSON.stringify(draft);
  async function req(path, opts={}) {
    const response = await fetch(base+path,{cache:'no-store', ...opts});
    if(response.status===401) markUnauthorized();
    const body=await response.json(); if(!response.ok) throw new Error(body.error || `HTTP ${response.status}`); return body;
  }
  async function run(fn){if(busy)return;busy=true;error='';notice='';try{await fn()}catch(e){error=String(e.message || e)}finally{busy=false}}
  function accept(s){state=s;draft=clone(s.current.catalog);preview=null;group=null;device=null;jsonText='';expanded='';nativeCheck=null;nativeUses=null;}
  async function reload(){if(dirty&&!confirm(tr('Отменить несохранённый черновик?','Discard unsaved draft?')))return;await run(async()=>accept(await req('/catalog')));}
  function update(){draft={...draft};preview=null;notice='';}
  function editGroup(x){nativeCheck=null;group=x?{...clone(x),tagText:(x.tags||[]).join(', '),existing:true}:{id:'',type:'network',name:'',revision:1,scope:'local',members:[],description:'',tagText:'',existing:false};}
  function saveGroup(){if(!group||!draft)return;const {existing,tagText,...value}=group;value.tags=tagText.split(',').map(x=>x.trim()).filter(Boolean);if(!value.description)delete value.description;const i=draft.policyObjects.findIndex(x=>x.id===value.id);if(!existing&&i>=0){error='DUPLICATE_ID';return;}if(existing&&i<0){error='UNKNOWN_ID';return;}draft.policyObjects=i<0?[...draft.policyObjects,value]:draft.policyObjects.map((x,j)=>j===i?value:x);group=null;update();}
  function removeGroup(id){const users=draft.policyObjects.filter(g=>g.members.some(m=>m.kind==='object'&&m.value===id));if(users.length){error=tr('Сначала удалите ссылки из групп: ','Remove group references first: ')+users.map(g=>g.name).join(', ');return;}draft.policyObjects=draft.policyObjects.filter(x=>x.id!==id);if(group?.id===id)group=null;update();}
  function editDevice(x){if(x){device={...clone(x),tagText:x.tags.join(', '),existing:true};return;}try{const bytes=crypto.getRandomValues(new Uint8Array(16));bytes[6]=(bytes[6]&15)|64;bytes[8]=(bytes[8]&63)|128;const h=Array.from(bytes,b=>b.toString(16).padStart(2,'0')).join('');device={id:`${h.slice(0,8)}-${h.slice(8,12)}-${h.slice(12,16)}-${h.slice(16,20)}-${h.slice(20)}`,name:'',kind:'linux',tags:[],enabled:true,revision:1,tagText:'',existing:false};}catch{error='SECURE_RANDOM_UNAVAILABLE'}}
  function saveDevice(){if(!device||!draft)return;const {existing,tagText,...value}=device;value.tags=tagText.split(',').map(x=>x.trim()).filter(Boolean);const i=draft.devices.findIndex(x=>x.id===value.id);if(!existing&&i>=0){error='DUPLICATE_ID';return;}draft.devices=i<0?[...draft.devices,value]:draft.devices.map((x,j)=>i===j?value:x);device=null;update();}
  function removeDevice(id){if(draft.connections.some(c=>c.deviceId===id)||draft.policyObjects.some(g=>g.members.some(m=>m.kind==='device'&&m.value===id))){error=tr('Сначала удалите ссылки и подключения устройства.','Remove device references and connections first.');return;}draft.devices=draft.devices.filter(x=>x.id!==id);if(device?.id===id)device=null;update();}
  let nativeCheck=null, nativeUses=null;
  async function checkNative(member){const original=member.value;nativeCheck=null;await run(async()=>{const data=await req('/native-reference?'+new URLSearchParams({ref:original}));if(data.schemaVersion!==1||typeof data.pin!=='string'||!['unpinned','current','stale'].includes(data.status)||!data.pin.endsWith('@'+data.sha256)||!/^[a-f0-9]{64}$/.test(data.sha256))throw Error('INVALID_REFERENCE_RESPONSE');if(member.value!==original)return;nativeCheck={member,original,data};});}
  function pinNative(member){if(!nativeCheck||nativeCheck.member!==member||nativeCheck.original!==member.value)return;if(!confirm(tr('Закрепить эту редакцию списка в черновике?','Pin this list revision in the draft?')))return;member.value=nativeCheck.data.pin;group={...group};nativeCheck=null;}
  async function loadNativeUses(){nativeUses=null;await run(async()=>{const result=await req('/native-references');if(result.etag!==state?.current.etag)throw Error('REVISION_CONFLICT');nativeUses=result;});}
  const headers = () => ({'Content-Type':'application/json','X-RF-Foundation':'1','If-Match':`"${state.current.etag}"`});
  async function inspect(){await run(async()=>{preview=await req('/preview',{method:'POST',headers:headers(),body:JSON.stringify(draft)});expanded='';notice=tr('Черновик проверен. Правила сети не изменены.','Draft validated. Network rules unchanged.');});}
  async function commit(){if(!preview)return;await run(async()=>{const next=await req('/save',{method:'POST',headers:{...headers(),'X-RF-Confirm':'SAVE_DRAFT','X-RF-Preview':preview.candidate.etag},body:JSON.stringify(draft)});accept(next);notice=tr('Каталог сохранён. Правила сети не изменены.','Catalogue saved. Network rules unchanged.');});}
  function loadHistory(h){if(dirty&&!confirm(tr('Заменить несохранённый черновик?','Replace unsaved draft?')))return;draft=clone(h.catalog);update();notice=tr('Редакция загружена в черновик. Сначала проверьте и сохраните.','Revision loaded into draft. Preview and save to restore.');}
  async function importDraft(){if(dirty&&!confirm(tr('Заменить несохранённый черновик?','Replace unsaved draft?')))return;await run(async()=>{const x=JSON.parse(jsonText);const p=await req('/preview',{method:'POST',headers:headers(),body:JSON.stringify(x)});draft=clone(p.candidate.catalog);group=null;device=null;update();preview=p;notice=tr('Импорт проверен, сохранение отдельно.','Import validated; save separately.');});}
  function exportCurrent(){if(!state)return;const url=URL.createObjectURL(new Blob([JSON.stringify(state.current.catalog,null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='routerforge-policy-catalog-v1.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
  async function readEvents(){await run(async()=>{const params=new URLSearchParams({limit:'100'});if(eventFilter.trim())params.set('component',eventFilter.trim());if(eventSeverity)params.set('severity',eventSeverity);events=(await req('/events?'+params)).events;});}
  onMount(()=>{void reload()});
</script>
<svelte:head><title>{tr('Объекты политик','Policy objects')} — RouterForge</title></svelte:head>
<div class="page foundation-page">
 <div class="page-head"><h1>{tr('Объекты политик','Policy objects')}</h1><p>{tr('Общий каталог устройств и групп. Сохранение черновика не применяет сетевые правила.','Shared device and group catalogue. Saving a draft never applies network rules.')}</p></div>
 {#if draft}
 <details class="panel"><summary>{tr('Ссылки и потребители каталога','Catalogue references and consumers')}</summary>
  <p>{tr('DNS Router, NFQWS и Network Tools читают сохранённые группы. Подстановка создаёт локальный черновик, а не живую привязку к правилам.','DNS Router, NFQWS and Network Tools read saved groups. Insertion creates a local draft, not a live rule binding.')}</p>
  <button type="button" class="button" disabled={busy||!state} onclick={loadNativeUses}>{tr('Ссылки на нативные списки (сохранённые)','Native list references (saved)')}</button>
  {#if nativeUses}<p>{tr('Редакция','Revision')}: {nativeUses.etag.slice(0,12)}</p>{#each nativeUses.references as item (item.ref)}<p style="overflow-wrap:anywhere"><strong>{item.ref}</strong> · {tr('Прямые и вложенные группы:','Direct and nested groups:')} {item.groups.join(', ')}</p>{:else}<p>{tr('Нет ссылок','No references')}</p>{/each}{/if}
  {#each draft.policyObjects as item (item.id)}
    <p><strong>{item.name}</strong> · {tr('Ссылки из групп:','Referenced by groups:')} {draft.policyObjects.filter(g=>g.members.some(m=>m.kind==='object'&&m.value===item.id)).map(g=>g.name).join(', ')||'—'}</p>
  {/each}
  {#each draft.devices as item (item.id)}
    <p><strong>{item.name}</strong> · {tr('Подключения:','Connections:')} {draft.connections.filter(c=>c.deviceId===item.id).length} · {tr('Группы:','Groups:')} {draft.policyObjects.filter(g=>g.members.some(m=>m.kind==='device'&&m.value===item.id)).map(g=>g.name).join(', ')||'—'}</p>
  {/each}
 </details>
 {/if}
 {#if state}<FoundationBindings catalogueETag={state.current.etag} groups={state.current.catalog.policyObjects} {locale} disabled={busy||dirty}/>{/if}
 <section class="panel">
  <div class="rf-foundation-actions">
   {#each [['groups',tr('Группы','Groups')],['devices',tr('Устройства','Devices')],['transfer',tr('Импорт / история','Import / history')],['events',tr('События','Events')]] as item}
    <button type="button" class="button" class:active={tab===item[0]} disabled={busy} onclick={()=>{tab=item[0];if(tab==='events')void readEvents()}}>{item[1]}</button>
   {/each}
  </div>
  {#if error}<p class="rf-foundation-error" role="alert">{error}</p>{/if}
  {#if notice}<p role="status">{notice}</p>{/if}
  {#if state}<p>{tr('Редакция каталога','Catalogue revision')}: {state.current.generation} · {dirty?tr('Есть черновик','Unsaved draft'):tr('Сохранено','Saved')}</p>{/if}
  {#if !draft}<button type="button" class="button" disabled={busy} onclick={reload}>{tr('Загрузить','Load')}</button>{/if}
  {#if draft && tab==='groups'}
   <div class="rf-foundation-actions"><label>{tr('Поиск','Search')} <input bind:value={filter}/></label><button type="button" class="button" disabled={busy} onclick={()=>editGroup(null)}>{tr('Добавить группу','Add group')}</button></div>
   <div class="rf-foundation-list">{#each filtered as x (x.id)}<div class="rf-foundation-row"><div><strong>{x.name}</strong><small>{x.id} · {x.type} · r{x.revision} · {x.members.length}</small></div><button type="button" class="button" disabled={busy} onclick={()=>editGroup(x)}>{tr('Изменить','Edit')}</button><button type="button" class="button" disabled={busy} onclick={()=>removeGroup(x.id)}>{tr('Удалить из черновика','Remove from draft')}</button></div>{:else}<p>{tr('Нет групп','No groups')}</p>{/each}</div>
   {#if group}<div class="rf-foundation-editor">
    <h2>{tr('Редактор группы','Group editor')}</h2>
    <label>ID <input bind:value={group.id} readonly={group.existing} maxlength="96" /></label><label>{tr('Название','Name')} <input bind:value={group.name} maxlength="160"/></label>
    <label>{tr('Тип','Type')} <select bind:value={group.type} disabled={group.existing||group.members.length>0} onchange={()=>{group.members=[];group={...group}}}>{#each types as x}<option value={x}>{x}</option>{/each}</select></label>
    <label>{tr('Описание','Description')} <input bind:value={group.description} maxlength="512"/></label><label>{tr('Теги через запятую','Comma-separated tags')} <input bind:value={group.tagText}/></label><label>{tr('Область','Scope')} <select bind:value={group.scope}><option value="local">local</option><option value="shared">shared</option></select></label>
    {#each group.members as member,i}<div class="rf-foundation-row"><label>{tr('Тип элемента','Member type')} <select bind:value={member.kind} onchange={()=>{member.value='';group={...group}}}>{#each memberKinds[group.type] as k}<option value={k}>{k}</option>{/each}</select></label>
    {#if member.kind==='object'}<label>{tr('Группа','Group')} <select bind:value={member.value}><option value="">—</option>{#each draft.policyObjects.filter(x=>x.type===group.type&&x.id!==group.id) as x}<option value={x.id}>{x.name} ({x.id})</option>{/each}</select></label>
    {:else if member.kind==='device'}<label>{tr('Устройство','Device')} <select bind:value={member.value}><option value="">—</option>{#each draft.devices as x}<option value={x.id}>{x.name}</option>{/each}</select></label>
    {:else if member.kind==='list-ref'}
     <label>{tr('Ссылка на список','List reference')} <input bind:value={member.value} maxlength="253" placeholder="dns.list-id / nfqws.user.list" oninput={()=>nativeCheck=null}/></label>
     <button type="button" class="button" disabled={busy||!member.value} onclick={()=>checkNative(member)}>{tr('Проверить редакцию','Check revision')}</button>
     {#if nativeCheck?.member===member&&nativeCheck.original===member.value}<small style="overflow-wrap:anywhere">{nativeCheck.data.status} · {nativeCheck.data.count} · {nativeCheck.data.sha256}</small>{#if nativeCheck.data.status!=='current'}<button type="button" class="button" disabled={busy} onclick={()=>pinNative(member)}>{tr('Закрепить в черновике','Pin in draft')}</button>{/if}{/if}
     <small>{tr('Только домены из сохранённого списка; условия DNS/NFQWS не переносятся. После изменения источника потребуется новый pin.','Domain membership from a saved list only; DNS/NFQWS rule semantics are not transferred. Changes require a new pin.')}</small>
    {:else}<label>{tr('Значение','Value')} <input bind:value={member.value} placeholder={member.kind==='port'?'tcp:443':member.kind==='domain'?'*.example.org':member.kind==='cidr'?'192.0.2.0/24':''}/></label>{/if}
    <button type="button" class="button" onclick={()=>{group.members=group.members.filter((_,j)=>j!==i);group={...group}}}>{tr('Убрать','Remove')}</button></div>{/each}
    <div class="rf-foundation-actions"><button type="button" class="button" onclick={()=>{group.members=[...group.members,{kind:memberKinds[group.type][0],value:''}];group={...group}}}>{tr('Добавить элемент','Add member')}</button><button type="button" class="button" onclick={saveGroup}>{tr('В черновик','Add to draft')}</button><button type="button" class="button" onclick={()=>group=null}>{tr('Отмена','Cancel')}</button></div>
   </div>{/if}
  {:else if draft && tab==='devices'}
   <button type="button" class="button" disabled={busy} onclick={()=>editDevice(null)}>{tr('Добавить устройство','Add device')}</button>
   {#each draft.devices as x (x.id)}<div class="rf-foundation-row"><div><strong>{x.name}</strong><small>{x.kind} · {x.id} · r{x.revision}</small></div><button type="button" class="button" disabled={busy} onclick={()=>editDevice(x)}>{tr('Изменить','Edit')}</button><button type="button" class="button" disabled={busy} onclick={()=>removeDevice(x.id)}>{tr('Удалить из черновика','Remove from draft')}</button></div>{:else}<p>{tr('Нет устройств','No devices')}</p>{/each}
   {#if device}<div class="rf-foundation-editor"><h2>{tr('Устройство','Device')}</h2><label>{tr('Название','Name')} <input bind:value={device.name} maxlength="160"/></label><label>{tr('Тип','Kind')} <select bind:value={device.kind}>{#each deviceKinds as k}<option value={k}>{k}</option>{/each}</select></label><label>{tr('Теги через запятую','Comma-separated tags')} <input bind:value={device.tagText}/></label><label><input type="checkbox" bind:checked={device.enabled}/> {tr('Включено','Enabled')}</label><div class="rf-foundation-actions"><button type="button" class="button" onclick={saveDevice}>{tr('В черновик','Add to draft')}</button><button type="button" class="button" onclick={()=>device=null}>{tr('Отмена','Cancel')}</button></div></div>{/if}
  {:else if draft && tab==='transfer'}
   <p>{tr('Импорт только метаданных: без токенов, ключей, ссылок vault и приватных URL. Подключения пока не выполняются.','Metadata-only import: no tokens, keys, vault references or private URLs. Connections are not executed yet.')}</p>
   <div class="rf-foundation-actions"><button type="button" class="button" onclick={exportCurrent}>{tr('Экспорт сохранённого','Export saved catalogue')}</button><button type="button" class="button" onclick={()=>jsonText=JSON.stringify(draft,null,2)}>{tr('JSON черновика','Draft JSON')}</button></div>
   <label>JSON<textarea rows="9" bind:value={jsonText}></textarea></label><button type="button" class="button" disabled={busy||!jsonText.trim()} onclick={importDraft}>{tr('Загрузить в черновик','Import into draft')}</button>
   <h2>{tr('История редакций','Revision history')}</h2>{#each state.history as h (h.etag)}<div class="rf-foundation-row"><span>r{h.generation}</span><button type="button" class="button" disabled={busy} onclick={()=>loadHistory(h)}>{tr('Восстановить в черновик','Restore to draft')}</button></div>{:else}<p>{tr('История пуста','No history')}</p>{/each}
  {:else if tab==='events'}
   <p>{tr('Нормализованные события текущего процесса Core, без сырых сообщений и секретных деталей.','Normalized events for the current Core process, without raw messages or secret details.')}</p><div class="rf-foundation-actions"><label>{tr('Компонент','Component')} <input bind:value={eventFilter}/></label><label>{tr('Уровень','Severity')} <select bind:value={eventSeverity}><option value="">{tr('Все','All')}</option>{#each ['info','warning','critical'] as s}<option value={s}>{s}</option>{/each}</select></label><button type="button" class="button" disabled={busy} onclick={readEvents}>{tr('Обновить','Refresh')}</button></div>{#each events as e (e.id)}<div class="rf-foundation-row"><div><strong>{e.type}</strong><small>{e.component} · {e.severity} · {e.observedAt}</small></div></div>{:else}<p>{tr('Нет событий','No events')}</p>{/each}
  {/if}
 </section>
 {#if draft && tab!=='events'}<section class="panel"><div class="rf-foundation-actions"><button type="button" class="button" disabled={busy} onclick={reload}>{tr('Перечитать','Reload')}</button><button type="button" class="button" disabled={busy||!dirty||!!group||!!device} onclick={inspect}>{tr('Проверить черновик','Preview draft')}</button>{#if preview}<button type="button" class="button" disabled={busy||!!group||!!device} onclick={commit}>{tr('Сохранить каталог','Save catalogue')}</button>{/if}</div>
 {#if preview}<p>{tr('Добавлено','Added')}: {preview.added.length} · {tr('Изменено','Changed')}: {preview.changed.length} · {tr('Удалено','Removed')}: {preview.removed.length}</p>{#each [['+',preview.added],['~',preview.changed],['−',preview.removed]] as row}{#each row[1] as id}<div>{row[0]} {id}</div>{/each}{/each}<label>{tr('Разворачивание группы','Group expansion')} <select bind:value={expanded}><option value="">—</option>{#each Object.keys(preview.expanded) as id}<option value={id}>{id}</option>{/each}</select></label>{#if expanded}<pre>{JSON.stringify(preview.expanded[expanded],null,2)}</pre>{/if}<p>{tr('Ссылки list-ref не разрешаются до появления адаптера потребителя. Действующие правила не изменяются.','list-ref entries are unresolved until a consumer adapter is added. Active rules remain unchanged.')}</p>{/if}</section>{/if}
</div>
<style>
 .foundation-page {min-width:0; display:grid;gap:12px}
 .rf-foundation-actions {display:flex;gap:8px;align-items:end;flex-wrap:wrap;margin:8px 0}
 .rf-foundation-row {display:flex;gap:8px;align-items:center;flex-wrap:wrap;padding:8px 0;border-bottom:1px solid var(--rf-border,var(--border,#7775));overflow-wrap:anywhere}
 .rf-foundation-row>div:first-child {flex:1;min-width:150px}
 .rf-foundation-row small {display:block;opacity:.75}
 .rf-foundation-editor {display:grid;gap:8px;margin-top:12px;min-width:0}
 label {display:flex;flex-direction:column;gap:4px;min-width:0}
 input,select,textarea {max-width:100%;min-width:0;box-sizing:border-box;padding:7px;border:1px solid var(--rf-border,var(--border,#7775));background:var(--rf-surface,var(--panel,transparent));color:inherit;border-radius:5px}
 textarea {width:100%;font-family:monospace}
 input[type=checkbox] {width:auto;align-self:start}
 .rf-foundation-error {color:var(--rf-danger,#ef4444);overflow-wrap:anywhere}
 .active {outline:1px solid currentColor}
 pre {max-height:320px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere}
</style>
