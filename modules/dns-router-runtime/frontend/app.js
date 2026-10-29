'use strict';
// No external libraries. All user-derived labels are inserted with textContent.
(() => {
  let config = {schema_version: 1, upstreams: [], rules: []};
  let revision = '';
  let dirty = false;
  let preparedN = null;
  const el = id => document.getElementById(id);
  const feedback = msg => { el('feedback').textContent = msg; };
  async function request(path, opts = {}) {
    const response = await fetch('../' + path, {cache: 'no-store', ...opts});
    const body = await response.json();
    if (!response.ok) throw new Error('HTTP ' + response.status + ': ' + (body.error || 'unknown error'));
    return body;
  }
  function jsonOptions(action, obj, extra = {}) {
    return { method: 'POST', headers: {'Content-Type':'application/json', 'X-RouterForge-Action':action, ...extra}, body:JSON.stringify(obj) };
  }
  function markDirty() { preparedN=null;dirty = true; feedback('Изменения не сохранены. DNS-трафик не затронут.'); render(); }
  function optionSelect(id) {
    const node = el(id); const selected = node.value;
    node.replaceChildren();
    for (const u of config.upstreams) { const o = document.createElement('option'); o.value = u.id; o.textContent = u.id; node.appendChild(o); }
    if (config.upstreams.some(u => u.id === selected)) node.value = selected;
  }
  function table(target, columns, rows, onRemove) {
    const root = el(target); root.replaceChildren();
    const t = document.createElement('table');
    const head = document.createElement('tr');
    for (const title of [...columns, '']) {const th = document.createElement('th'); th.textContent = title;head.appendChild(th);}
    t.appendChild(head);
    for (const [index, values] of rows.entries()) {
      const tr = document.createElement('tr');
      for (const val of values) {const td = document.createElement('td');td.textContent = String(val);tr.appendChild(td);}
      const td = document.createElement('td'); const b = document.createElement('button'); b.type='button';b.textContent='Удалить';
      b.addEventListener('click', () => onRemove(index));td.appendChild(b);tr.appendChild(td);t.appendChild(tr);
    }
    root.appendChild(t);
  }
  function render() {
    optionSelect('ruleUpstream'); optionSelect('importUpstream');
    table('upstreamList',['ID','Протокол','Адрес','SNI','DoH path','Резерв'],config.upstreams.map(u=>[u.id,u.transport,u.address,u.server_name||'—',u.path||'—',(u.fallbacks||[]).join(', ')||'—']),i=>{
      const id = config.upstreams[i].id;
      if (config.rules.some(r=>r.upstream===id) || config.upstreams.some(u=>(u.fallbacks||[]).includes(id)) || (config.dns_groups||[]).some(g=>g.servers.includes(id))) {feedback('Нельзя удалить используемый пул. Сначала убери правила и резервные ссылки.');return;}
      config.upstreams.splice(i,1);markDirty();
    });
    table('ruleList',['ID','Домен','Совпадение','Пул','Приоритет'],config.rules.map(r=>[r.id,r.domain,r.match,r.upstream,r.priority]),i=>{config.rules.splice(i,1);markDirty();});
    const listener = config.listener || {mode:'loopback',listen:'127.0.0.1:55353',allowed_clients:[]};
    el('listenerMode').value=listener.mode;
    el('listenerListen').value=listener.listen;
    el('listenerACL').value=(listener.allowed_clients||[]).join(',');
    el('revision').textContent = (dirty ? '● Не сохранено · ' : '') + revision.substring(0,23);
    p5RefreshLists();
  }
  async function load() {
    if (dirty && !confirm('Отменить несохранённые изменения?')) return;
    const data = await request('config');
    config = data.config;revision = data.revision; dirty = false;preparedN=null;render();feedback('Конфигурация загружена · runtime меняется только при ручном старте/применении');
  }
  el('listenerForm').addEventListener('submit',e=>{
    e.preventDefault(); const form=Object.fromEntries(new FormData(e.currentTarget));
    const allowed=form.allowed_clients.split(',').map(s=>s.trim()).filter(Boolean);
    config.listener={mode:form.mode,listen:form.listen.trim()};
    if(form.mode==='lan') config.listener.allowed_clients=allowed;
    markDirty();feedback('Listener добавлен в черновик · ничего не запущено');
  });
  el('listenerPlanButton').addEventListener('click',async()=>{
    if(dirty){feedback('Для точного плана сначала сохрани черновик');return;}
    try{const data=await request('keenetic/domain-plan');
      if(data.revision!==revision) throw new Error('Конфигурация обновлена другим клиентом');
      el('listenerPlanStatus').textContent=JSON.stringify(data.plan,null,2);
      feedback('План read-only готов · DNS Keenetic не изменён');
    }catch(err){feedback('План DNS: '+err.message);}
  });
  // DPR2-N: the server recalculates its exact ndmc snapshot and a CAS-bound
  // token. Preparing never changes DNS; a confirmed trial always rolls back.
  el('managedPrepareN').addEventListener('click', async () => {
    if (dirty) { feedback('Сначала сохрани конфигурацию для сверки зон'); return; }
    preparedN = null;
    try {
      const data = await request('keenetic/domain-managed/prepare',
        jsonOptions('managed-prepare',{operation:'prepare'},{'If-Match':'"'+revision+'"'}));
      if (data.revision !== revision) throw new Error('Ревизия изменилась');
      el('managedStatusN').textContent=JSON.stringify(data,null,2);
      if (!data.ready) { feedback('Найдены конфликты с действующей конфигурацией. Изменений нет.'); return; }
      preparedN = data;
      feedback('План проверен. Применение ещё не выполнялось. Для временной проверки нажми «Испытать».');
    } catch(err) { preparedN=null;feedback('DPR2-N prepare: '+err.message); }
  });
  el('managedTrialN').addEventListener('click', async () => {
    if (dirty || !preparedN || !preparedN.ready || preparedN.revision!==revision) {
      feedback('Нужен актуальный подтверждённый план без несохранённых изменений');return;
    }
    const domains=preparedN.zones.map(x=>x.domain).join(', ');
    if (!confirm('ВНИМАНИЕ: DSTest на короткое время зарегистрирует DNS-зоны '+domains+
      ' через ndmc. После проверки они будут удалены. Продолжить?'))return;
    if (prompt('Для запуска временной DNS-транзакции введи TRIAL:')!=='TRIAL') {
      feedback('Испытание отменено; изменений нет');return;
    }
    try {
      // Consume token exactly once even if the network fails mid-request.
      const token=preparedN.token;preparedN=null;
      el('managedStatusN').textContent='Транзакция выполняется. Ожидается подтверждение DNS и отката…';
      const data=await request('keenetic/domain-managed/trial',
        jsonOptions('managed-trial',{operation:'trial',token},{'If-Match':'"'+revision+'"'}));
      el('managedStatusN').textContent=JSON.stringify(data,null,2);
      feedback(data.baseline_restored && data.state==='pass' ?
        'DPR2-N: временное применение и точный откат подтверждены' :
        'DPR2-N: результат неоднозначный, проверь журнал/откат');
    }catch(err){el('managedStatusN').textContent=String(err);feedback('DPR2-N: '+err.message+'; проверь DNS-снимок перед повтором');}
  });
  // DPR2-O1: status only; no permanent activation action exposed.
  el('managedOwnershipStatusO').addEventListener('click', async () => {
    try {
      const status=await request('keenetic/managed-ownership/status');
      el('managedOwnershipResultO').textContent=JSON.stringify(status,null,2);
      feedback('Реестр проверен без изменения DNS Keenetic');
    } catch(err) { feedback('DPR2-O1: '+err.message); }
  });
  el('upstreamForm').addEventListener('submit',e=>{
    e.preventDefault();const v=Object.fromEntries(new FormData(e.currentTarget));
    if(config.upstreams.some(u=>u.id===v.id)){feedback('ID пула уже существует');return;}
    const upstream = {id:v.id,transport:v.transport,address:v.address};
    if (v.transport==='dot'||v.transport==='doh') upstream.server_name=v.server_name.trim();
    if (v.transport==='doh' && v.path.trim()) upstream.path=v.path.trim();
    const reserve=v.fallbacks.split(',').map(s=>s.trim()).filter(Boolean);
    if (reserve.length) upstream.fallbacks=reserve;
    config.upstreams.push(upstream);e.currentTarget.reset();markDirty();
  });
  el('ruleForm').addEventListener('submit',e=>{
    e.preventDefault();const v=Object.fromEntries(new FormData(e.currentTarget));
    if(config.rules.some(r=>r.id===v.id)){feedback('ID правила уже существует');return;}
    config.rules.push({id:v.id,domain:v.domain,match:v.match,upstream:v.upstream,priority:Number(v.priority)});e.currentTarget.reset();markDirty();
  });
  el('saveButton').addEventListener('click',async()=>{
    try {
      const response = await request('config', {method:'PUT', headers:{'Content-Type':'application/json','X-RouterForge-Action':'save','If-Match':'"'+revision+'"'},body:JSON.stringify(config)});
      revision = response.revision;dirty = false;preparedN=null;render();feedback('Сохранено атомарно; работающий shadow не изменён до явного применения');
    } catch(err){feedback('Ошибка сохранения: '+err.message);}
  });
  el('reloadButton').addEventListener('click',()=>load().catch(err=>feedback(err.message)));
  el('evaluateForm').addEventListener('submit',async e=>{
    e.preventDefault();try{
      const domain = new FormData(e.currentTarget).get('domain');
      const data = await request('evaluate',jsonOptions('evaluate',{domain}));
      el('decision').textContent=JSON.stringify(data.decision,null,2);
    }catch(err){feedback(err.message);}
  });
  el('importForm').addEventListener('submit',async e=>{
    e.preventDefault();
    if(dirty){feedback('Сначала сохрани или отменяй текущие изменения перед импортом');return;}
    try{
      const f = Object.fromEntries(new FormData(e.currentTarget));
      const data = await request('import-preview',jsonOptions('import-preview',{...f,priority:100}));
      if(data.base_revision !== revision) throw new Error('Конфигурация изменилась; перечитай её');
      const p = data.preview;
      el('importResult').textContent=`Добавлено: ${p.added}, дубликатов: ${p.duplicates}, пропущено: ${p.ignored}, всего правил: ${p.result_rules}`;
      if(confirm('Применить импорт к НЕСОХРАНЁННОМУ проекту? DNS-трафик не изменится.')) {config=p.config;markDirty();}
    }catch(err){feedback(err.message);}
  });
  async function shadowRefresh() {
    const data = await request('shadow/status');
    el('shadowStatus').textContent = JSON.stringify(data,null,2);
  }
  async function lifecycleControl(operation) {
    if (dirty) { feedback('Сначала сохрани настройки: есть несохранённые изменения'); return; }
    if (operation==='start' && !confirm('Запустить сохранённый shadow listener? Keenetic DNS не изменится.')) return;
    if (operation==='apply' && !confirm('Применить сохранённую ревизию к shadow DNS?')) return;
    try {
      const data=await request('shadow/control',jsonOptions('shadow-control',{operation},{'If-Match':'"'+revision+'"'}));
      feedback('Shadow: '+data.outcome+' · DNS Keenetic не затронут');
      await shadowRefresh();
    } catch(err) { feedback('Shadow: '+err.message); await shadowRefresh().catch(()=>{}); }
  }
  el('shadowStatusButton').addEventListener('click',()=>shadowRefresh().catch(e=>feedback(e.message)));
  el('shadowStartButton').addEventListener('click',()=>lifecycleControl('start'));
  el('shadowApplyButton').addEventListener('click',()=>lifecycleControl('apply'));
  el('shadowStopButton').addEventListener('click',()=>lifecycleControl('stop'));
  el('resourcesButton').addEventListener('click',async()=>{try{const d=await request('runtime/resources');el('discoveryStatus').textContent=JSON.stringify(d,null,2);}catch(e){feedback(e.message);}});
  el('keeneticButton').addEventListener('click',async()=>{try{const d=await request('keenetic/discovery');el('discoveryStatus').textContent=JSON.stringify(d,null,2);}catch(e){feedback(e.message);}});
  el('integrationButton').addEventListener('click',async()=>{try{const d=await request('keenetic/integration-plan');el('discoveryStatus').textContent=JSON.stringify(d,null,2);}catch(e){feedback(e.message);}});
  el('hardwareButton').addEventListener('click',async()=>{try{const d=await request('hardware/evidence');el('discoveryStatus').textContent=JSON.stringify(d,null,2);}catch(e){feedback(e.message);}});
  el('evidenceForm').addEventListener('submit', async e => {
    e.preventDefault();
    try {
      const evidence = String(new FormData(e.currentTarget).get('evidence') || '');
      const report = await request('keenetic/assess-hardware', jsonOptions('assess-hardware-readonly',{evidence}));
      el('assessmentStatus').textContent = JSON.stringify(report,null,2);
      feedback('DPR2-G: отчёт обработан только в памяти; активация DNS Proxy запрещена');
    } catch(err) { feedback('DPR2-G: '+err.message); }
  });
  el('rehearsalButton').addEventListener('click', async () => {
    try {
      const report = await request('keenetic/rehearsal?failure=verify');
      el('assessmentStatus').textContent = JSON.stringify(report,null,2);
      feedback('DPR2-G: исключительно симуляция in-memory, конфигурация Keenetic не менялась');
    } catch(err) { feedback('DPR2-G: '+err.message); }
  });
  el('upstreamStatusButton').addEventListener('click',async()=>{
    try {const data=await request('upstreams/status');el('upstreamStatus').textContent=JSON.stringify(data,null,2);}
    catch(err){feedback('Диагностика: '+err.message);}
  });
  el('exportButton').addEventListener('click',async()=>{
    try{
      const data = await request('config');
      const blob=new Blob([JSON.stringify(data.config,null,2)+'\n'],{type:'application/json'});
      const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download='routerforge-dns-router.json';a.click();URL.revokeObjectURL(url);
      feedback('Экспортирована сохранённая конфигурация');
    }catch(err){feedback(err.message);}
  });
  // DPR2-P5: named domain lists. Uses the existing CAS draft and API; never
  // touches a running shadow listener or the native Keenetic resolver.
  let p5EditingList = '';
  let p5Pending = null;
  let p5EditRule = '';
  let p5EditUpstream = '';

  function p5Text(id,value) { const n=el(id);if(n)n.textContent=value; }
  function p5Value(id,value) {const n=el(id);if(n)n.value=value;}
  function p5Options(id,items,chosen) {
    const s=el(id);if(!s)return;
    s.replaceChildren();
    for(const entry of items) {
      const opt=document.createElement('option');opt.value=entry[0];opt.textContent=entry[1];s.appendChild(opt);
    }
    if(items.some(a=>a[0]===chosen))s.value=chosen;
  }
  function p5SourceVisibility() {
    const kind=el('drListKind').value;
    el('drListUrlWrap').hidden=kind!=='url';
    el('drListFileWrap').hidden=kind!=='file';
    el('drListPasteWrap').hidden=kind!=='paste';
    el('drListIntervalWrap').hidden=kind!=='url';
  }
  function p5RefreshLists() {
    if(!Array.isArray(config.lists)) config.lists=[];
    const pool=el('drListUpstream');const previous=pool.value;
    p5Options('drListUpstream',config.upstreams.map(u=>[u.id,u.id]),previous);
    p6RenderGroups();
    const target=el('drListCards');target.replaceChildren();
    if(config.lists.length===0) {
      const p=document.createElement('p');p.className='dr-p5-empty';p.textContent='Списков пока нет. Выбери файл, URL или вставь домены ниже.';target.appendChild(p);
    }
    for(const list of config.lists) {
      const row=document.createElement('div');row.className='dr-p5-list-row';
      const copy=document.createElement('div');copy.className='dr-p5-list-copy';
      const h=document.createElement('strong');h.textContent=list.name;copy.appendChild(h);
      const count=config.rules.filter(r=>r.list_id===list.id).length;
      const detail=document.createElement('span');
      detail.textContent=count+' доменов · '+(list.group_id?'Группа '+list.group_id:list.upstream)+' · '+(list.kind==='url'?'URL':list.kind==='file'?'файл':'текст')+
        (list.auto_update_minutes?' · авто '+list.auto_update_minutes+' мин':' · обновление вручную');
      copy.appendChild(detail);row.appendChild(copy);
      const actions=document.createElement('div');actions.className='dr-p5-list-actions';
      const edit=document.createElement('button');edit.type='button';edit.className='button';edit.textContent='Изменить / обновить';
      edit.addEventListener('click',()=>{
        if(dirty) {feedback('Сначала сохрани черновик перед обновлением источника');return;}
        p5EditingList=list.id;p5Pending=null;
        p5Value('drListName',list.name);p5Value('drListKind',list.kind);
        p5Value('drListURL',list.url||'');p5Value('drListUpstream',list.upstream);p5Value('drListGroup',list.group_id||'');
        p5Value('drListMatch',list.match);p5Value('drListPriority',String(list.priority));
        p5Value('drListInterval',String(list.auto_update_minutes||0));
        p5SourceVisibility();p5Text('drListMode','Редактирование: '+list.name);
        el('drListPanel').scrollIntoView({behavior:'smooth',block:'start'});
      });actions.appendChild(edit);
      const remove=document.createElement('button');remove.type='button';remove.className='button danger';remove.textContent='Удалить';
      remove.addEventListener('click',()=>{
        if(!confirm('Удалить список «'+list.name+'» и все его правила из черновика?'))return;
        config.lists=config.lists.filter(x=>x.id!==list.id);
        config.rules=config.rules.filter(x=>x.list_id!==list.id);
        p5EditingList='';p5Pending=null;markDirty();
      });actions.appendChild(remove);row.appendChild(actions);target.appendChild(row);
    }
    p5ApplySearch();
    p5InstallEditButtons();
  }
  function p5ApplySearch() {
    const q=(el('drRuleSearch')?.value||'').trim().toLowerCase();
    const root=el('ruleList');if(!root)return;
    for(const tr of root.querySelectorAll('tr')) {
      if(tr.querySelector('th'))continue;
      tr.hidden=Boolean(q)&&!tr.textContent.toLowerCase().includes(q);
    }
  }
  function p5InstallEditButtons() {
    for(const [targetName,listName,formName,onEdit] of [
      ['ruleList','rules','ruleForm','rule'],['upstreamList','upstreams','upstreamForm','upstream']
    ]) {
      const root=el(targetName);if(!root)return;
      const trs=Array.from(root.querySelectorAll('tr')).filter(x=>!x.querySelector('th'));
      trs.forEach((tr,i)=>{
        const item=config[listName][i];if(!item || tr.querySelector('.dr-p5-edit'))return;
        const td=tr.lastElementChild;if(!td)return;
        const b=document.createElement('button');b.type='button';b.className='button dr-p5-edit';b.textContent='Изменить';
        b.addEventListener('click',()=>{
          const form=el(formName);
          for(const field of form.elements){if(field.name)field.value=item[field.name]??'';}
          if(onEdit==='rule') {
            p5EditRule=item.id;
            p5Value('ruleUpstream',item.upstream);
          } else {
            p5EditUpstream=item.id;
            p5Value('drListUpstream',item.id);
            form.elements.namedItem('fallbacks').value=(item.fallbacks||[]).join(', ');
          }
          form.elements.namedItem('id').readOnly=true;
          const submit=form.querySelector('[type=submit]');if(submit)submit.textContent='Сохранить изменения';
          form.scrollIntoView({behavior:'smooth',block:'start'});
        });td.prepend(b);
      });
    }
  }
  el('drListKind').addEventListener('change',p5SourceVisibility);
  el('drRuleSearch').addEventListener('input',p5ApplySearch);
  el('drListReset').addEventListener('click',()=>{
    p5EditingList='';p5Pending=null;el('drListForm').reset();p5SourceVisibility();
    p5Text('drListMode','Новый список');p5Text('drListPreview','Выбери источник и нажми «Проверить список».');
  });
  el('drListForm').addEventListener('submit',async e=>{
    e.preventDefault();p5Pending=null;
    if(dirty){feedback('Сначала сохрани текущие изменения, затем импортируй список');return;}
    const form=e.currentTarget;
    const f=Object.fromEntries(new FormData(form));
    const kind=f.kind;
    const list={
      id:p5EditingList || ('list-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,7)),
      name:String(f.name||'').trim(),kind,
      upstream:f.group_id ? (config.dns_groups.find(g=>g.id===f.group_id)||{servers:[]}).servers[0] : f.upstream,
      group_id:f.group_id||undefined,match:f.match,priority:Number(f.priority),
      auto_update_minutes:kind==='url'?Number(f.auto_update_minutes||0):0
    };
    if(kind==='url')list.url=String(f.url||'').trim();
    let text='';
    try {
      if(kind==='file') {
        const file=el('drListFile').files[0];
        if(!file)throw new Error('Выбери TXT-файл');
        if(file.size>1048576)throw new Error('Максимальный размер файла 1 МиБ');
        text=await file.text();
      } else if(kind==='paste') text=f.text||'';
      if(kind!=='url' && !text.trim())throw new Error('Список пуст');
      p5Text('drListPreview','Проверка строк и доменов…');
      const data=await request('lists/preview',jsonOptions('lists-preview',{list,text}));
      if(revision!==data.base_revision)throw new Error('Ревизия изменилась. Перечитай конфигурацию.');
      p5Pending=data.preview;
      const p=data.preview;
      const errors=(p.errors||[]).map(x=>'строка '+x.line+': '+x.reason).join('\n');
      p5Text('drListPreview',
        'Добавлено: '+p.added+' · дубликатов: '+p.duplicates+' · пропущено: '+p.ignored+
        ' · ошибок: '+p.invalid+' · итог правил: '+p.total_rules + (errors?'\n'+errors:''));
      el('drListAccept').disabled=false;
      feedback('Предпросмотр готов. Проверь результат и нажми «Добавить в черновик».');
    } catch(err) {
      el('drListAccept').disabled=true;p5Text('drListPreview','Ошибка: '+err.message);feedback('Импорт: '+err.message);
    }
  });
  el('drListAccept').addEventListener('click',()=>{
    if(!p5Pending)return;
    if(dirty){feedback('Черновик изменился. Сначала сохрани и повтори предпросмотр.');return;}
    if(p5Pending.invalid>0 && !confirm('Есть ошибки строк: '+p5Pending.invalid+'. Добавить только корректные домены?'))return;
    config=p5Pending.config;
    p5EditingList='';p5Pending=null;el('drListAccept').disabled=true;
    el('drListForm').reset();p5SourceVisibility();p5Text('drListMode','Новый список');
    markDirty();p5Text('drListPreview','Список добавлен в черновик. Нажми «Сохранить изменения» внизу страницы.');
  });
  // Existing forms keep their API handlers; intercept only explicit edits.
  el('ruleForm').addEventListener('submit',e=>{
    if(!p5EditRule)return;
    e.preventDefault();e.stopImmediatePropagation();
    const f=Object.fromEntries(new FormData(e.currentTarget));
    const item=config.rules.find(r=>r.id===p5EditRule);
    if(!item)return;
    if(item.list_id && (f.match!==item.match || f.upstream!==item.upstream || Number(f.priority)!==item.priority)){
      feedback('Для правил списка DNS-пул, тип совпадения и приоритет меняются через настройки самого списка.');return;
    }
    if(item.list_id)feedback('Изменение домена списка сохранится в черновике, но следующий импорт источника может его перезаписать.');
    item.domain=String(f.domain).trim().toLowerCase();item.match=f.match;
    item.upstream=f.upstream;item.priority=Number(f.priority);
    p5EditRule='';const form=e.currentTarget;form.reset();form.elements.namedItem('id').readOnly=false;
    form.querySelector('[type=submit]').textContent='+ Добавить правило';markDirty();
  },true);
  el('upstreamForm').addEventListener('submit',e=>{
    if(!p5EditUpstream)return;
    e.preventDefault();e.stopImmediatePropagation();
    const f=Object.fromEntries(new FormData(e.currentTarget));
    const item=config.upstreams.find(u=>u.id===p5EditUpstream);if(!item)return;
    item.transport=f.transport;item.address=String(f.address).trim();
    item.server_name=(f.transport==='dot'||f.transport==='doh')?String(f.server_name).trim():undefined;
    item.path=f.transport==='doh'?String(f.path).trim():undefined;
    const fallback=String(f.fallbacks).split(',').map(v=>v.trim()).filter(Boolean);
    item.fallbacks=fallback.length?fallback:undefined;
    p5EditUpstream='';const form=e.currentTarget;form.reset();form.elements.namedItem('id').readOnly=false;
    form.querySelector('[type=submit]').textContent='+ Добавить пул';markDirty();
  },true);
  // A read-only explanation rather than a misleading active-service badge.
  // P6: strict DNS groups built from existing configured upstream servers.
  function p6RenderGroups(){
    if(!Array.isArray(config.dns_groups)) config.dns_groups=[];
    const select=el('drListGroup'), prior=select.value;
    p5Options('drListGroup',[['','Без группы (старое правило)'],...config.dns_groups.map(g=>[g.id,g.name+' ['+g.id+']'])],prior);
    const root=el('drGroupCards');root.replaceChildren();
    if(!config.dns_groups.length){const p=document.createElement('p');p.className='dr-p5-empty';p.textContent='Групп пока нет. Добавь DNS-пулы выше, затем создай строгую группу.';root.appendChild(p)}
    for(const group of config.dns_groups){
      const row=document.createElement('div');row.className='dr-p5-list-row';
      const copy=document.createElement('div');copy.className='dr-p5-list-copy';
      const h=document.createElement('strong');h.textContent=group.name;copy.appendChild(h);
      const desc=document.createElement('span');desc.textContent=group.servers.join(' → ')+' · только свои DNS · fail-closed';copy.appendChild(desc);row.appendChild(copy);
      const actions=document.createElement('div');actions.className='dr-p5-list-actions';
      const remove=document.createElement('button');remove.type='button';remove.className='button danger';remove.textContent='Удалить';
      remove.addEventListener('click',()=>{
        if(config.rules.some(r=>r.group_id===group.id)||config.lists.some(l=>l.group_id===group.id)){
          feedback('Группа используется. Сначала переназначь списки и правила.');return;
        }
        if(!confirm('Удалить неиспользуемую DNS-группу «'+group.name+'»?'))return;
        config.dns_groups=config.dns_groups.filter(g=>g.id!==group.id);markDirty();
      });actions.appendChild(remove);row.appendChild(actions);root.appendChild(row);
    }
  }
  el('drListGroup').addEventListener('change',()=>{
    const g=(config.dns_groups||[]).find(x=>x.id===el('drListGroup').value);
    if(g)el('drListUpstream').value=g.servers[0];
  });
  el('drGroupForm').addEventListener('submit',e=>{
    e.preventDefault();const f=Object.fromEntries(new FormData(e.currentTarget));
    const ids=String(f.servers||'').split(',').map(s=>s.trim()).filter(Boolean);
    if(!Array.isArray(config.dns_groups))config.dns_groups=[];
    if(config.dns_groups.some(g=>g.id===f.id)){feedback('ID DNS-группы уже существует');return;}
    if(!ids.length||ids.length>8||new Set(ids).size!==ids.length||ids.some(id=>!config.upstreams.some(u=>u.id===id))){
      feedback('Нужны 1–8 разных существующих DNS-пулов в порядке приоритета');return;
    }
    config.dns_groups.push({id:f.id,name:f.name,servers:ids});e.currentTarget.reset();markDirty();
    feedback('Строгая DNS-группа создана в черновике. Сохрани изменения перед импортом списка.');
  });

  p5SourceVisibility();

  load().catch(err=>feedback('Ошибка загрузки: '+err.message));
  shadowRefresh().catch(()=>{});
})();

/* RouterForge DNS Router 2.0 unified UI bridge; presentation only.
   No API mutations, DNS changes or request interception. */
(() => {
  'use strict';
  const root = document.documentElement;
  const workspace = document.getElementById('dnsRouterWorkspace');
  const params = new URLSearchParams(window.location.search);
  const themeTokens = [
    '--rf-bg','--rf-surface','--rf-surface-2','--rf-hover','--rf-text','--rf-muted',
    '--rf-border','--rf-border-strong','--rf-accent','--rf-accent-soft',
    '--rf-accent-border','--rf-accent-hover','--rf-radius-panel','--rf-radius-card',
    '--rf-radius-control','--rf-panel-head-h','--rf-control-gap',
    '--ui-body','--ui-small','--ui-xs','--ui-micro','--ui-title',
    '--ui-panel-title','--rf-type-metric'
  ];
  const palette = {
    forge: ['#0b0d10','#12151a','#171b21','#1d2229','#f5f7fa','#8d98a4','#29313a','#36414d'],
    midnight: ['#08111b','#0f1824','#152131','#1b2a3c','#edf5ff','#8ca0b5','#25384a','#34516a'],
    graphite: ['#101113','#17191c','#1d2024','#25292e','#f1f2f4','#9299a2','#30353c','#414850']
  };
  function validColor(value, fallback) {
    return /^#[0-9a-fA-F]{6}$/.test(String(value || '')) ? value : fallback;
  }
  // Match Antiscan's frame-side custom palette when the host theme is not accessible.
  function mix(a, b, weight) {
    return '#' + [1,3,5].map(i => Math.round(
      parseInt(a.slice(i,i+2),16)*(1-weight) + parseInt(b.slice(i,i+2),16)*weight
    ).toString(16).padStart(2,'0')).join('');
  }
  function customPalette(background, text) {
    const rgb = [1,3,5].map(i => parseInt(background.slice(i,i+2),16));
    const light = (rgb[0]*.2126 + rgb[1]*.7152 + rgb[2]*.0722) / 255 > .55;
    return [background,
      mix(background,text,light?.045:.055),
      mix(background,text,light?.09:.105),
      mix(background,text,light?.13:.15),
      text,mix(text,background,.48),
      mix(background,text,light?.18:.19),
      mix(background,text,light?.27:.29)];
  }
  function fallbackTheme() {
    const id = params.get('theme') || 'forge';
    const background = validColor(params.get('background'), '#0b0d10');
    const text = validColor(params.get('text'), '#f5f7fa');
    const colors = id === 'custom' ? customPalette(background,text) : (palette[id] || palette.forge);
    const names = ['--rf-bg','--rf-surface','--rf-surface-2','--rf-hover','--rf-text','--rf-muted','--rf-border','--rf-border-strong'];
    names.forEach((key,i) => root.style.setProperty(key,colors[i]));
    root.style.setProperty('--rf-accent',validColor(params.get('accent'),'#38bdf8'));
    root.dataset.theme = id;
    if (['compact','normal','comfortable'].includes(params.get('density'))) root.dataset.density = params.get('density');
    if (['sharp','default','soft'].includes(params.get('radius'))) root.dataset.radius = params.get('radius');
    root.style.colorScheme = (id === 'custom' ? background : colors[0]).match(/^#[0-9a-f]{6}$/i)
      && (parseInt((id==='custom'?background:colors[0]).slice(1,3),16)*.2126
       + parseInt((id==='custom'?background:colors[0]).slice(3,5),16)*.7152
       + parseInt((id==='custom'?background:colors[0]).slice(5,7),16)*.0722) > 140 ? 'light' : 'dark';
  }
  function syncHostTheme() {
    if (window.parent === window) return false;
    try {
      const hostRoot = window.parent.document.documentElement;
      const hostStyle = window.parent.getComputedStyle(hostRoot);
      themeTokens.forEach(token => {
        const v = hostStyle.getPropertyValue(token).trim();
        if (v) root.style.setProperty(token, v);
      });
      ['theme','density','radius','uiScale'].forEach(key => {
        if (hostRoot.dataset[key]) root.dataset[key] = hostRoot.dataset[key];
      });
      root.style.colorScheme = hostStyle.colorScheme === 'light' ? 'light' : 'dark';
      return true;
    } catch (_) { return false; }
  }
  fallbackTheme();
  syncHostTheme();
  let heightFrame = 0;
  function reportHeight() {
    if (window.parent === window || !workspace || heightFrame) return;
    heightFrame = requestAnimationFrame(() => {
      heightFrame = 0;
      const height = Math.ceil(Math.max(360, workspace.scrollHeight, workspace.getBoundingClientRect().height) + 6);
      window.parent.postMessage({type:'routerforge-module-height',moduleId:'dns-router',height}, window.location.origin);
    });
  }
  const tabs = [...document.querySelectorAll('.tab[data-tab]')];
  const pages = [...document.querySelectorAll('.tab-page[data-page]')];
  function activate(tabId) {
    if (!pages.some(p => p.dataset.page === tabId)) return;
    tabs.forEach(btn => {
      const selected = btn.dataset.tab === tabId;
      btn.classList.toggle('active', selected);
      btn.setAttribute('aria-selected', String(selected));
      btn.tabIndex = selected ? 0 : -1;
    });
    pages.forEach(page => {
      const selected = page.dataset.page === tabId;
      page.classList.toggle('active',selected);
      page.setAttribute('aria-hidden',String(!selected));
    });
    root.dataset.section = tabId;
    reportHeight();
  }
  tabs.forEach((btn,index) => {
    btn.addEventListener('click', () => activate(btn.dataset.tab));
    btn.addEventListener('keydown', event => {
      if (!['ArrowRight','ArrowLeft','Home','End'].includes(event.key)) return;
      event.preventDefault();
      let next = index;
      if (event.key==='ArrowRight') next=(index+1)%tabs.length;
      if (event.key==='ArrowLeft') next=(index+tabs.length-1)%tabs.length;
      if (event.key==='Home') next=0;
      if (event.key==='End') next=tabs.length-1;
      activate(tabs[next].dataset.tab);
      tabs[next].focus();
    });
  });
  document.querySelectorAll('[data-go]').forEach(btn => btn.addEventListener('click', () => activate(btn.dataset.go)));
  activate(params.get('view') || 'overview');
  function updateMetrics() {
    // The API renderer creates a header-only table for an empty collection.
    // Add a readable empty state without changing the renderer or its data.
    for (const [id,message] of [['upstreamList','Пулов пока нет. Добавь первый DNS upstream выше.'],
                                ['ruleList','Правил пока нет. Добавь домен или импортируй список.']]) {
      const area=document.getElementById(id);
      const table=area?.querySelector('table');
      if (!table) continue;
      const empty=area.querySelector('.dr-empty-note');
      const count=Math.max(0,table.querySelectorAll('tr').length-1);
      if (count===0 && !empty) {
        const node=document.createElement('div');node.className='dr-empty-note';node.textContent=message;
        area.appendChild(node);
      } else if (count>0 && empty) empty.remove();
    }
    const rows = id => Math.max(0, (document.getElementById(id)?.querySelectorAll('tr').length || 0) - 1);
    const sets = [
      ['metricUpstreams', rows('upstreamList')],
      ['metricRules', rows('ruleList')]
    ];
    sets.forEach(([id, value]) => {const node = document.getElementById(id); if (node) node.textContent = String(value);});
    const revision = document.getElementById('revision')?.textContent || '';
    const config = document.getElementById('metricConfig');
    if (config) config.textContent = revision.includes('Не сохранено') ? 'ЧЕРНОВИК' : (revision ? 'СОХРАНЕНО' : '—');
    const footer = document.getElementById('metricFooter');
    if (footer) footer.textContent = revision.includes('Не сохранено') ? 'Есть несохранённые изменения' : 'Настройки DNS Router';
    const status = document.getElementById('shadowStatus')?.textContent || '';
    let active = false;
    try {
      const parsed = JSON.parse(status);
      active = Boolean(parsed?.lifecycle?.active || parsed?.active || parsed?.lifecycle?.state === 'running');
    } catch (_) {}
    const badge = document.getElementById('shadowBadge');
    const metric = document.getElementById('metricRuntime');
    const ready = status.trim() && status.trim() !== 'Загрузка состояния…';
    if (badge) {
      badge.textContent = active ? 'SHADOW · АКТИВЕН' : (ready ? 'SHADOW · ОСТАНОВЛЕН' : 'SHADOW · ПРОВЕРКА');
      badge.classList.toggle('good', active);
    }
    if (metric) metric.textContent = active ? 'АКТИВЕН' : (ready ? 'ВЫКЛ.' : '—');
    const feedback = document.getElementById('feedback');
    if (feedback) {
      const msg = feedback.textContent || '';
      feedback.classList.toggle('bad', /Ошибка|HTTP 5|HTTP 4|FAIL|конфликт|запрещено/i.test(msg));
      feedback.classList.toggle('warn', !feedback.classList.contains('bad') && /не сохран|сначала|отмен|внимание/i.test(msg));
      feedback.classList.toggle('good', !feedback.classList.contains('bad') && !feedback.classList.contains('warn') && /загружена|Сохранено|подтвержден|готов/i.test(msg));
    }
  }
  for (const id of ['upstreamList','ruleList','revision','shadowStatus','feedback']) {
    const target = document.getElementById(id);
    if (target) new MutationObserver(updateMetrics).observe(target, {childList:true,subtree:true,characterData:true});
  }
  if (workspace && 'ResizeObserver' in window) {
    new ResizeObserver(reportHeight).observe(workspace);
  }
  try {
    if (window.parent !== window && 'MutationObserver' in window) {
      const hostRoot = window.parent.document.documentElement;
      new MutationObserver(() => {syncHostTheme();reportHeight();}).observe(hostRoot, {
        attributes:true, attributeFilter:['style','class','data-theme','data-density','data-radius','data-ui-scale']
      });
    }
  } catch (_) {}
  window.addEventListener('resize', reportHeight);
  updateMetrics();
  reportHeight();
})();
