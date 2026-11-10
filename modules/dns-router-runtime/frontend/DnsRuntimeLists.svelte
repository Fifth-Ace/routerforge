<script>
  import {untrack} from 'svelte';
  import {draft,readDraft,commitDraft,feedback} from './DnsRuntimeStore.svelte.js';
  import {createPolicyManager} from './DnsRuntimePolicyLogic.js';
  let lists=$state([]),upstreams=$state([]),groups=$state([]),rules=$state([]);
  let editingId=$state(''),name=$state(''),kind=$state('file'),url=$state(''),domainText=$state('');
  let groupId=$state(''),upstream=$state(''),match=$state('suffix'),priority=$state(100),interval=$state('0');
  let selectedFile=$state.raw(null),pending=$state.raw(null),busy=$state(false);
  let error=$state(''),previewMessage=$state('Выбери источник и нажми «Проверить список».');
  let stale=$state(false);
  const notify=feedback;
  async function postPreview({list,text}){
    const response=await fetch('../lists/preview',{
      method:'POST',cache:'no-store',credentials:'same-origin',
      headers:{'Content-Type':'application/json','X-RouterForge-Action':'lists-preview'},
      body:JSON.stringify({list,text})
    });
    const data=await response.json();
    if(!response.ok)throw Error('HTTP '+response.status+': '+(data?.error||'unknown error'));
    return data;
  }
  const manager=createPolicyManager({
    readDraft,commit:commitDraft,postPreview,confirmAction:message=>window.confirm(message),notify,
    createId:()=>('list-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,7))
  });
  function refresh(){
    const v=readDraft();
    if(!v?.config)return;
    lists=Array.isArray(v.config.lists)?v.config.lists:[];
    rules=Array.isArray(v.config.rules)?v.config.rules:[];
    groups=Array.isArray(v.config.dns_groups)?v.config.dns_groups:[];
    upstreams=Array.isArray(v.config.upstreams)?v.config.upstreams:[];
    if(!upstreams.some(x=>x.id===upstream))upstream=upstreams[0]?.id||'';
    if(groupId && !groups.some(x=>x.id===groupId))groupId='';
    if(pending && (pending.generation!==v.generation||pending.revision!==v.revision)){
      pending=null;stale=true;previewMessage='Черновик изменился. Выполни предпросмотр заново.';
    }
  }
  function groupChange(event){
    groupId=event.currentTarget.value;
    const g=groups.find(x=>x.id===groupId);
    if(g)upstream=g.servers[0];
  }
  function reset(){
    editingId='';name='';kind='file';url='';domainText='';groupId='';match='suffix';priority=100;interval='0';selectedFile=null;pending=null;
    stale=false;error='';previewMessage='Выбери источник и нажми «Проверить список».';
  }
  function edit(list){
    error='';const draft=readDraft();
    if(!draft||draft.dirty){error='Сначала сохрани текущий черновик перед обновлением источника';notify(error);return;}
    editingId=list.id;name=list.name||'';kind=list.kind||'paste';url=list.url||'';
    groupId=list.group_id||'';upstream=list.upstream||'';match=list.match||'suffix';priority=list.priority??100;
    interval=String(list.auto_update_minutes||0);domainText='';selectedFile=null;pending=null;stale=false;
    previewMessage='Проверь источник списка, затем нажми «Проверить список».';
    document.getElementById('drListPanel')?.scrollIntoView({behavior:'smooth',block:'start'});
  }
  function remove(list){
    error='';try{manager.removeList(list.id);if(editingId===list.id)reset();}
    catch(e){error=String(e?.message||e);notify(error);}
  }
  async function preview(event){
    event.preventDefault();if(busy)return;
    busy=true;pending=null;stale=false;error='';previewMessage='Проверяем список на сервере…';
    try{
      let text=kind==='paste'?domainText:'';
      if(kind==='file'){
        if(!selectedFile)throw Error('Выбери TXT-файл');
        if(selectedFile.size>1048576||!selectedFile.size)throw Error('TXT-файл должен быть до 1 МиБ');
        text=await selectedFile.text();
      }
      const item=await manager.previewList({
        editId:editingId,name,kind,url,group_id:groupId,upstream,match,priority,auto_update_minutes:interval
      },text,kind==='file'?selectedFile.size:0);
      pending=item;
      const errors=item.errors.map(x=>'строка '+x.line+': '+x.reason).join('\n');
      previewMessage='Добавлено: '+item.added+' · дубликатов: '+item.duplicates+' · пропущено: '+item.ignored+
        ' · ошибок: '+item.invalid+' · итог правил: '+item.total_rules+(errors?'\n'+errors:'');
    }catch(e){error=String(e?.message||e);previewMessage='Ошибка: '+error;notify('Импорт: '+error);}
    finally{busy=false;}
  }
  function accept(){
    if(busy||!pending)return;
    error='';
    try{const result=manager.acceptList(pending);if(!result?.cancelled){reset();previewMessage='Список добавлен в черновик. Нажми «Сохранить изменения».';}}
    catch(e){error=String(e?.message||e);pending=null;notify(error);}
  }
  $effect(()=>{void draft.generation;untrack(refresh);});
</script>
<div data-native-svelte-dns-lists="true">
  <div class="dr-p5-content">
    <div class="dr-p5-steps" aria-label="Порядок добавления"><div><b>1</b><span>Выбери список</span></div><div><b>2</b><span>Назначь DNS-пул</span></div><div><b>3</b><span>Проверь и сохрани</span></div></div>
    <div id="drListCards" class="dr-p5-list-rows">
      {#if !lists.length}<p class="dr-p5-empty">Списков пока нет. Выбери файл, URL или вставь домены ниже.</p>{/if}
      {#each lists as list (list.id)}
        <div class="dr-p5-list-row">
          <div class="dr-p5-list-copy"><strong>{list.name}</strong><span>{rules.filter(r=>r.list_id===list.id).length} доменов · {list.group_id?'Группа '+list.group_id:list.upstream} · {list.kind==='url'?'URL':list.kind==='file'?'файл':'текст'} · {list.auto_update_minutes?'авто '+list.auto_update_minutes+' мин':'обновление вручную'}</span></div>
          <div class="dr-p5-list-actions"><button class="button" type="button" onclick={()=>edit(list)}>Изменить / обновить</button><button class="button danger" type="button" onclick={()=>remove(list)}>Удалить</button></div>
        </div>
      {/each}
    </div>
  </div>
  <div class="dr-p5-form-head"><strong id="drListMode">{editingId?'Редактирование: '+name:'Новый список'}</strong><button id="drListReset" type="button" class="button" onclick={reset}>+ Новый список</button></div>
  <form class="form-grid dr-p5-form" id="drListForm" onsubmit={preview}>
    <label><span>Название списка</span><input id="drListName" required maxlength="80" placeholder="YouTube, Discord, Работа…" bind:value={name}></label>
    <label><span>Источник</span><select id="drListKind" bind:value={kind}><option value="file">TXT-файл</option><option value="paste">Вставить текст</option><option value="url">HTTPS-ссылка</option></select></label>
    {#if kind==='file'}<label class="span-all"><span>Файл с доменами (.txt, до 1 МиБ)</span><input id="drListFile" type="file" accept=".txt,.hosts,.list,text/plain" onchange={e=>selectedFile=e.currentTarget.files?.[0]||null}></label>{/if}
    {#if kind==='paste'}<label class="span-all"><span>Домены по одному на строку</span><textarea id="drListPaste" rows="6" placeholder="youtube.com&#10;googlevideo.com" bind:value={domainText}></textarea></label>{/if}
    {#if kind==='url'}<label class="span-all"><span>Публичная HTTPS-ссылка на текстовый список</span><input id="drListURL" type="url" maxlength="2048" placeholder="https://example.org/domains.txt" bind:value={url}><small>Проверяется сервером, загрузка не меняет системный DNS.</small></label>{/if}
    <label><span>Строгая DNS-группа (необязательно)</span><select id="drListGroup" value={groupId} onchange={groupChange}><option value="">Без группы (старое правило)</option>{#each groups as g (g.id)}<option value={g.id}>{g.name} [{g.id}]</option>{/each}</select></label>
    <label><span>Отправлять запросы через DNS-пул</span><select id="drListUpstream" required bind:value={upstream}>{#each upstreams as u (u.id)}<option value={u.id}>{u.id}</option>{/each}</select></label>
    <label><span>Для каких доменов</span><select id="drListMatch" bind:value={match}><option value="suffix">Домен и поддомены</option><option value="exact">Только этот домен</option><option value="subdomains">Только поддомены</option></select></label>
    <label><span>Приоритет</span><input id="drListPriority" type="number" min="0" max="1000000" bind:value={priority}></label>
    {#if kind==='url'}<label><span>Автообновление URL</span><select id="drListInterval" bind:value={interval}><option value="0">Выключено</option><option value="60">Каждый час</option><option value="360">Каждые 6 часов</option><option value="1440">Каждые сутки</option></select></label>{/if}
    <div class="form-actions"><button class="button primary" type="submit" disabled={busy||!upstreams.length}>{busy?'Проверяем…':'Проверить список'}</button></div>
  </form>
  <div class="dr-p5-preview" aria-live="polite"><strong>Предпросмотр импорта</strong><pre class="console" id="drListPreview">{previewMessage}</pre><div class="dr-p5-bottom-actions"><button type="button" id="drListAccept" class="button primary" disabled={!pending||busy||stale} onclick={accept}>Добавить в черновик</button><span>После добавления нажми «Сохранить изменения» внизу страницы.</span></div></div>
  {#if error}<p class="notice bad" role="alert">{error}</p>{/if}
</div>
