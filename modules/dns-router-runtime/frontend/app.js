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
    p88RenderRules();p88RefreshStatus();
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
  // DPR2-P7: never contact ndmc without explicit click; even live=1 reads only.
  for (const live of [false,true]) {
    el(live ? 'ingressLiveP7' : 'ingressOfflineP7').addEventListener('click', async () => {
      if (dirty) {feedback('Для проверки фактического покрытия сначала сохрани конфигурацию');return;}
      const output=el('ingressAuditStatusP7');
      output.textContent=live ? 'Чтение текущих записей Keenetic…' : 'Анализ сохранённых правил…';
      try {
        const data=await request('keenetic/ingress-audit'+(live?'?live=1':''));
        if(data.revision!==revision)throw new Error('Ревизия изменилась: повтори проверку');
        output.textContent=JSON.stringify(data.audit,null,2);
        const a=data.audit;
        feedback('P7: покрыто '+a.covered+'/'+a.rule_count+'; вне маршрутов '+a.unmapped+'; конфликты '+a.conflicts.length+'; DNS не изменён');
      }catch(err){output.textContent='Ошибка P7: '+err.message;feedback('Аудит DNS: '+err.message);}
    });
  }
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
    if (['dot','doh','doq'].includes(v.transport)) upstream.server_name=v.server_name.trim();
    if (v.transport==='doh' && v.path.trim()) upstream.path=v.path.trim();
    const reserve=v.fallbacks.split(',').map(s=>s.trim()).filter(Boolean);
    if(v.transport==='native' && (v.server_name.trim() || v.path.trim() || reserve.length || !/^native:\/\/(System|Policy(?:0|[1-9][0-9]{0,2}))\/[^/]+\/[^/]+$/.test(v.address.trim()))){
      feedback('Native: нужен селектор System/PolicyN; TLS SNI, DoH path и резервы запрещены.');return;
    }
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
  el('nativeBridgePreviewButton').addEventListener('click',async()=>{
    try {const data=await request('keenetic/native-bridge/status');el('p85NativeStatus').textContent=JSON.stringify(data,null,2);feedback('Keenetic Native: capability и привязки (RO); egress не подтверждён');}
    catch(err){feedback('Native Bridge: '+err.message);}
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
            p85NativeVisibility();
            if(['udp','tcp'].includes(item.transport)){el('p8PlainProtocols').checked=true;for(const o of form.elements.namedItem('transport').options){o.hidden=false;}}
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
    if(f.transport==='native' && (String(f.server_name).trim() || String(f.path).trim() || String(f.fallbacks).trim() || !/^native:\/\/(System|Policy(?:0|[1-9][0-9]{0,2}))\/[^/]+\/[^/]+$/.test(String(f.address).trim()))){
      feedback('Native: используй селектор из инвентаризации без TLS-полей и резервов.');return;
    }
    item.transport=f.transport;item.address=String(f.address).trim();
    item.server_name=(['dot','doh','doq'].includes(f.transport))?String(f.server_name).trim():undefined;
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
      const desc=document.createElement('span');desc.textContent=group.servers.join(' → ')+(group.encrypted_only?' · только шифрование':' · legacy/совместимость')+' · fail-closed';copy.appendChild(desc);row.appendChild(copy);
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
    const encryptedOnly=!!f.encrypted_only;
    if(encryptedOnly && ids.some(id=>{const u=config.upstreams.find(u=>u.id===id);return !u||!['dot','doh','doq'].includes(u.transport)})){feedback('Группа только с шифрованием не принимает UDP/TCP-пулы');return;}
    config.dns_groups.push({id:f.id,name:f.name,servers:ids,encrypted_only:encryptedOnly});e.currentTarget.reset();markDirty();
    feedback('Строгая DNS-группа создана в черновике. Сохрани изменения перед импортом списка.');
  });

  // P8.5: inventory-backed selector; labels are textContent, never HTML.
  const p85Form=el('upstreamForm');
  const p85NativeSelect=el('p85NativeEndpoint');
  function p85NativeVisibility(){
    const native=p85Form.elements.namedItem('transport').value==='native';
    el('p85NativePicker').hidden=!native;
    const fields=['server_name','path','fallbacks'];
    for(const field of fields){
      const node=p85Form.elements.namedItem(field);
      node.readOnly=native;
      if(native)node.value='';
    }
    if(native && !p85Form.elements.namedItem('address').value.startsWith('native://'))p85Form.elements.namedItem('address').value='';
  }
  async function p85ScanNative(){
    const status=el('p85NativeMessage');
    status.textContent='Считываю ndnproxy без изменений...';
    p85NativeSelect.replaceChildren();
    try{
      const [inventory,ready,catalog]=await Promise.all([request('keenetic/native-dns'),request('keenetic/native-bridge/status'),request('keenetic/native-policies')]);
      el('p85NativeStatus').textContent=JSON.stringify(ready,null,2);
      const policyByID=new Map((catalog.available?catalog.policies:[]).map(p=>[p.name,p]));
      const labelFor=name=>{
        if(name==='System')return 'Системная (System)';
        const p=policyByID.get(name);
        return p && p.description ? p.description+' ('+name+')' : name+' (имя неизвестно)';
      };
      const routeFor=name=>{
        if(name==='System')return 'системный DNS; маршрут не проверен';
        const p=policyByID.get(name);
        if(!p)return 'маршрут неизвестен';
        if(p.ipv4_route_status==='default_present')return 'default route есть (не DNS-проверка)';
        if(p.ipv4_route_status==='default_absent')return 'нет default route';
        return 'маршрут неизвестен';
      };
      const names=(inventory.instances||[]).filter(p=>p.valid).map(p=>labelFor(p.name)+' — '+routeFor(p.name));
      el('p87PolicySummary').textContent=names.join(' | ')+(catalog.available?' · NDMC RO':' · имена/маршруты NDMC недоступны');
      if(!inventory.available){status.textContent='Инвентаризация неполная или недоступна. Выбор заблокирован.';return;}
      const choices=[];
      for(const item of inventory.instances||[]){
        if(!item.valid || !/^(System|Policy(?:0|[1-9][0-9]{0,2}))$/.test(item.name))continue;
        for(const ep of item.endpoints||[]){
          if(!/^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$/.test(ep.upstream_hint||''))continue;
          const value='native://'+item.name+'/'+ep.domain+'/'+ep.upstream_hint;
          if(!choices.some(x=>x[0]===value))choices.push([value,labelFor(item.name)+' / '+ep.domain+' / '+ep.upstream_hint+' — '+routeFor(item.name)+' (текущий '+ep.address+')']);
        }
      }
      p5Options('p85NativeEndpoint',choices,p85Form.elements.namedItem('address').value);
      status.textContent='Найдено '+choices.length+' привязок. Только чтение: порт выбирается при запросе; реальную доступность проверь отдельной кнопкой.';
      if(!choices.length)feedback('Подходящие привязки Native отсутствуют');
    }catch(err){status.textContent='Native: '+err.message;feedback('Инвентаризация Native: '+err.message);}
  }
  el('p85NativeScan').addEventListener('click',p85ScanNative);
  // P8.7: explicit ONE-query health check. Never on page load or inventory scan.
  el('p87NativeProbe').addEventListener('click',async()=>{
    const output=el('p87ProbeResult');
    const selected=p85NativeSelect.value;
    const parts=selected.startsWith('native://')?selected.slice(9).split('/'):[];
    if(parts.length!==3 || parts[1]!=='.'){
      output.textContent='Для активной DNS-проверки выбери upstream зоны «.» (System/PolicyN).';return;
    }
    output.textContent='Один DNS-запрос example.com к выбранному локальному endpoint…';
    const qs=new URLSearchParams({instance:parts[0],hint:parts[2]});
    try{
      const result=await request('keenetic/native-dns/probe?'+qs.toString());
      output.textContent=JSON.stringify(result,null,2);
      feedback(result.dns_success?'DNS upstream ответил успешно. Доступность проверена; шифрование не доказано.':'DNS upstream не прошёл проверку: '+(result.error||'нет ответа'));
    }catch(err){output.textContent='Ошибка проверки: '+err.message;feedback('DNS probe: '+err.message);}
  });
  p85NativeSelect.addEventListener('change',()=>{
    if(p85NativeSelect.value){p85Form.elements.namedItem('address').value=p85NativeSelect.value;feedback('Селектор выбран в форме. Сохрани черновик; runtime не изменён.');}
  });
  p85Form.elements.namedItem('transport').addEventListener('change',p85NativeVisibility);
  // An edit can set transport programmatically without dispatching change.
  p85Form.addEventListener('click',()=>{if(!el('p85NativePicker').hidden && p85Form.elements.namedItem('transport').value!=='native')p85NativeVisibility();});
  const p8Plain=el('p8PlainProtocols');
  const p8Select=el('upstreamForm').elements.namedItem('transport');
  function p8PlainVisibility(){
    if(['udp','tcp'].includes(p8Select.value))p8Plain.checked=true;
    for(const opt of p8Select.options){if(['udp','tcp'].includes(opt.value))opt.hidden=!p8Plain.checked;}
  }
  p8Plain.addEventListener('change',()=>{if(!p8Plain.checked&&['udp','tcp'].includes(p8Select.value))p8Select.value='dot';p8PlainVisibility();});
  p8Select.addEventListener('change',p8PlainVisibility);
  p8PlainVisibility();
  p85NativeVisibility();
  p5SourceVisibility();

  // P8.9 NATIVE CATALOG BEGIN
'use strict';
// DPR2 P8.9: a truthful, read-only view of every ndnproxy endpoint.
// The native wire contract is still zone-scoped and does not prove DoT/DoH.
(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.RFNativeCatalogP89 = api;
})(typeof window !== 'undefined' ? window : null, function () {
  const hintRE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$/;
  const policyRE = /^(System|Policy(?:0|[1-9][0-9]{0,2}))$/;
  const zoneRE = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/;
  const loopbackRE = /^127\.0\.0\.1:([0-9]{4,5})$/;
  function zoneValid(zone) {
    return zone === '.' || (typeof zone === 'string' && zone.length <= 253 && zoneRE.test(zone));
  }
  function entries(inventory, policy) {
    if (!policyRE.test(String(policy || '')) || !inventory || !Array.isArray(inventory.instances)) return [];
    const complete = inventory.available === true && inventory.parsed_instances === inventory.instances.length;
    const instance = inventory.instances.find(x => x && x.name === policy);
    if (!instance || !Array.isArray(instance.endpoints)) return [];
    const counts = new Map();
    for (const ep of instance.endpoints) {
      if (!ep) continue;
      const key = String(ep.domain) + '\u0000' + String(ep.upstream_hint || '');
      counts.set(key, (counts.get(key) || 0) + 1);
    }
    return instance.endpoints.map((ep, index) => {
      const zone = ep && ep.domain;
      const hint = ep && ep.upstream_hint || '';
      const loopback = ep && typeof ep.address === 'string' && loopbackRE.exec(ep.address);
      const numericPort = loopback ? Number(loopback[1]) : 0;
      let reason = '';
      if (!complete) reason = 'Инвентаризация Keenetic неполная';
      else if (!instance.valid) reason = 'DNS-инстанс содержит ошибки';
      else if (!zoneValid(zone)) reason = 'Неизвестная DNS-зона';
      else if (!loopback || numericPort < 1025 || numericPort > 65535) reason = 'Некорректный локальный endpoint';
      else if (!hintRE.test(hint)) reason = 'Нет устойчивого идентификатора DNS-сервера';
      else if (counts.get(String(zone)+'\u0000'+hint) !== 1) reason = 'Неоднозначный DNS-сервер';
      const usable = reason === '';
      return {
        index, zone: String(zone || ''), hint: String(hint),
        address: String(ep && ep.address || ''),
        selector: usable ? 'native://' + policy + '/' + zone + '/' + hint : '',
        usable, reason,
        // Do NOT infer transport from a stubby port or a comment hint.
        transport: 'unknown', encryptionProven: false, source: 'ndnproxy',
        label: (hint || ('DNS #' + (index+1))) + ' · ' + (zone === '.' ? 'все домены' : 'зона ' + zone)
      };
    });
  }
  function acceptsDomains(entry, domains) {
    if (!entry || !entry.usable) return {ok:false, reason:entry && entry.reason || 'DNS недоступен для выбора'};
    if (!Array.isArray(domains) || !domains.length) return {ok:false, reason:'Нет доменов'};
    const zone = entry.zone;
    if (zone === '.') return {ok:true, reason:''};
    for (const d of domains) {
      if (typeof d !== 'string' || (d !== zone && !d.endsWith('.' + zone))) {
        return {ok:false, reason:'DNS привязан в Keenetic к зоне ' + zone + '. Домен ' + d + ' вне этой зоны. Для других доменов нужен DNS зоны «.» либо отдельно настроенный DoT/DoH.'};
      }
    }
    return {ok:true, reason:''};
  }
  return Object.freeze({entries, acceptsDomains});
});

  // P8.9 NATIVE CATALOG END
  // P8.8: simple policy -> native DNS -> domain list. Does not modify Keenetic.
  const p88 = {inventory:null, catalog:null, policy:'System', loading:false};
  const p88Id = id => el(id);
  function p88ParseDomains(raw){
    const found=[];
    const seen=new Set();
    const lines=String(raw||'').split(/\r?\n/);
    if(lines.length>500)return {error:'Слишком много строк. Для больших файлов используй расширенный импорт.'};
    for(const line of lines){
      const stripped=line.split('#')[0].trim();
      if(!stripped)continue;
      for(const item of stripped.split(/[\s,;]+/)){
        if(!item)continue;
        const domain=item.toLowerCase().replace(/\.$/,'');
        if(/^\d+(?:\.\d+){3}$/.test(domain) || domain.length>253 || !/^(?=.{1,253}$)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/.test(domain)){
          return {error:'Некорректный домен: '+item+'. Указывай обычные имена, без URL, IP, масок и ||-фильтров.'};
        }
        if(!seen.has(domain)){found.push(domain);seen.add(domain);}
        if(found.length>200)return {error:'Не больше 200 доменов за один раз. Большие списки добавляй в расширенном режиме.'};
      }
    }
    return {domains:found};
  }
  function p88PolicyName(name){
    if(name==='System')return 'Системная';
    const p=(p88.catalog && p88.catalog.available ? (p88.catalog.policies||[]) : []).find(x=>x.name===name);
    return p && p.description ? p.description : name;
  }
  function p88RouteHint(name){
    if(name==='System')return 'Системный DNS';
    const p=(p88.catalog && p88.catalog.available ? (p88.catalog.policies||[]) : []).find(x=>x.name===name);
    if(!p)return 'Маршрут не проверен';
    if(p.ipv4_route_status==='default_absent')return 'Нет маршрута по умолчанию';
    if(p.ipv4_route_status==='default_present')return 'Маршрут найден · DNS не проверен';
    return 'Маршрут не проверен';
  }
  // P8.9: the read-only catalog shows ALL zones, not only the default '.'.
  function p89NativeEntries(){
    return window.RFNativeCatalogP89.entries(p88.inventory,p88.policy);
  }
  function p88NativeChoices(){
    return p89NativeEntries().filter(x=>x.usable).map(x=>({
      value:x.selector,label:x.label,zone:x.zone,hint:x.hint
    }));
  }
  function p88CreateButton(text,click,cls){
    const btn=document.createElement('button');btn.type='button';btn.className=cls||'button';btn.textContent=text;btn.addEventListener('click',click);return btn;
  }
  function p88RenderPolicies(){
    const root=p88Id('drSimplePolicies');root.replaceChildren();
    if(!p88.inventory || !p88.inventory.available){
      const p=document.createElement('p');p.textContent='Список политик пока недоступен. Нажми «Обновить».';root.appendChild(p);return;
    }
    const instances=(p88.inventory.instances||[]).filter(x=>x.valid && /^(System|Policy(?:0|[1-9][0-9]{0,2}))$/.test(x.name));
    if(!instances.some(x=>x.name===p88.policy))p88.policy=instances.some(x=>x.name==='System')?'System':(instances[0]&&instances[0].name)||'';
    for(const instance of instances){
      const button=p88CreateButton('',()=>{p88.policy=instance.name;p88RenderPolicies();p88RenderDNS();p88RenderRules();},'dr-simple-policy');
      button.setAttribute('aria-pressed',String(instance.name===p88.policy));
      if(instance.name===p88.policy)button.classList.add('selected');
      const name=document.createElement('strong');name.textContent=p88PolicyName(instance.name);button.appendChild(name);
      const sub=document.createElement('span');sub.textContent=instance.name==='System'?'Основная · System':instance.name;button.appendChild(sub);
      const hint=document.createElement('small');hint.textContent=p88RouteHint(instance.name);button.appendChild(hint);
      root.appendChild(button);
    }
  }
  function p88RenderDNS(){
    const field=p88Id('drSimpleDNS'),chosen=field.value;
    field.replaceChildren();
    const entries=p89NativeEntries();
    const choices=p88NativeChoices();
    for(const choice of choices){
      const opt=document.createElement('option');opt.value=choice.value;opt.textContent=choice.label;field.appendChild(opt);
    }
    if(choices.some(x=>x.value===chosen))field.value=chosen;
    field.disabled=!choices.length;
    p88Id('drSimpleAdd').disabled=!choices.length;
    const catalog=p88Id('drSimpleDNSInventory');catalog.replaceChildren();
    for(const entry of entries){
      const card=document.createElement('div');card.className='dr-p89-dns-item';
      const label=document.createElement('strong');label.textContent=entry.label;card.appendChild(label);
      const meta=document.createElement('span');
      meta.textContent=(entry.usable?'Можно выбрать':'Только просмотр')+' · '+
        (entry.zone==='.'?'Любые домены':'Только '+entry.zone+' и поддомены')+
        ' · транспорт DoT/DoH не подтверждён';
      card.appendChild(meta);
      const more=document.createElement('small');
      more.textContent=entry.reason || 'Локальный DNS Keenetic · '+entry.address;
      card.appendChild(more);catalog.appendChild(card);
    }
    if(!entries.length){
      const info=document.createElement('p');info.textContent='В этой политике DNS endpoints не обнаружены.';catalog.appendChild(info);
    }
    p88Id('drSimpleDNSNote').textContent=entries.length ?
      'Найдено '+entries.length+' DNS-записей; доступно для выбора: '+choices.length+'. DNS для отдельных зон видны ниже. Неизвестный DoT/DoH не помечается как обычный DNS.' :
      'В текущей политике DNS endpoints не обнаружены. Проверь настройки Keenetic.';
    p88Id('drSimpleRoute').textContent=p88.policy ? p88PolicyName(p88.policy)+' · '+p88RouteHint(p88.policy) : 'Политика не выбрана';
    p88Id('drSimpleProbeResult').textContent='DNS-запрос ещё не выполнялся.';
    p89UpdateProbeChoice();
  }
  function p89UpdateProbeChoice(){
    const selected=p88NativeChoices().find(x=>x.value===p88Id('drSimpleDNS').value);
    p88Id('drSimpleProbe').disabled=!selected || selected.zone!=='.';
    if(selected && selected.zone!=='.')
      p88Id('drSimpleProbeResult').textContent='DNS ограничен зоной '+selected.zone+'. Проверка example.com для этой зоны недопустима.';
  }
  function p88RenderRules(){
    const root=p88Id('drSimpleRuleList');root.replaceChildren();
    const current=(config.rules||[]).filter(r=>{
      const u=(config.upstreams||[]).find(x=>x.id===r.upstream);
      return u && u.transport==='native' && u.address.startsWith('native://'+p88.policy+'/');
    });
    p88Id('drSimpleRuleCount').textContent=String(current.length)+' доменов';
    if(!current.length){const empty=document.createElement('p');empty.textContent='Здесь пока нет доменных правил. Добавь домены выше.';root.appendChild(empty);return;}
    for(const rule of current){
      const row=document.createElement('div');row.className='dr-simple-rule';
      const info=document.createElement('span');info.textContent=rule.domain+' · '+((config.upstreams||[]).find(u=>u.id===rule.upstream)||{}).address.split('/').pop();row.appendChild(info);
      row.appendChild(p88CreateButton('Удалить',()=>{
        const idx=(config.rules||[]).findIndex(x=>x.id===rule.id);
        if(idx<0)return;
        config.rules.splice(idx,1);markDirty();
      },'button danger'));
      root.appendChild(row);
    }
  }
  function p88RefreshStatus(){
    p88Id('drSimpleSave').disabled=!dirty;
    p88Id('drSimpleState').textContent=dirty ?
      'Есть несохранённые изменения. На DNS Keenetic они не влияют.' :
      'Конфигурация сохранена. Маршрутизация клиентского DNS автоматически НЕ включалась.';
  }
  async function p88Scan(){
    if(p88.loading)return;
    p88.loading=true;
    p88Id('drSimpleScan').disabled=true;
    p88Id('drSimpleState').textContent='Читаю DNS и политики Keenetic (только чтение)…';
    try{
      const [inventory,catalog]=await Promise.all([request('keenetic/native-dns'),request('keenetic/native-policies')]);
      p88.inventory=inventory;p88.catalog=catalog;
      p88RenderPolicies();p88RenderDNS();p88RenderRules();
      p88RefreshStatus();
      if(!inventory.available)p88Id('drSimpleState').textContent='DNS-инвентаризация недоступна. Создание правил заблокировано.';
      else if(!catalog.available)p88Id('drSimpleState').textContent='DNS найдены, но имена пользовательских политик недоступны; проверь выбранную политику.';
    }catch(err){
      p88.inventory=null;p88.catalog=null;
      p88RenderPolicies();p88RenderDNS();
      p88Id('drSimpleState').textContent='Не удалось прочитать Keenetic: '+err.message;
    }finally{p88.loading=false;p88Id('drSimpleScan').disabled=false;}
  }
  p88Id('drModeSimple').addEventListener('click',()=>{
    p88Id('drAdvancedWorkspace').hidden=true;p88Id('drSimpleWorkspace').hidden=false;
    p88Id('dnsRouterWorkspace').classList.remove('dr-mode-advanced');
    p88Id('drModeSimple').setAttribute('aria-pressed','true');p88Id('drModeAdvanced').setAttribute('aria-pressed','false');
    p88RenderRules();p88RefreshStatus();
  });
  p88Id('drModeAdvanced').addEventListener('click',()=>{
    p88Id('drSimpleWorkspace').hidden=true;p88Id('drAdvancedWorkspace').hidden=false;
    p88Id('dnsRouterWorkspace').classList.add('dr-mode-advanced');
    p88Id('drModeSimple').setAttribute('aria-pressed','false');p88Id('drModeAdvanced').setAttribute('aria-pressed','true');
  });
  p88Id('drSimpleScan').addEventListener('click',p88Scan);
  p88Id('drSimpleDNS').addEventListener('change',p89UpdateProbeChoice);
  p88Id('drSimpleAdd').addEventListener('click',()=>{
    const selected=p88Id('drSimpleDNS').value;
    const chosenDNS=p88NativeChoices().find(x=>x.value===selected);
    if(!chosenDNS){
      p88Id('drSimpleState').textContent='Выбери DNS-сервер из текущего списка Keenetic.';return;
    }
    const parsed=p88ParseDomains(p88Id('drSimpleDomains').value);
    if(parsed.error){p88Id('drSimpleState').textContent=parsed.error;return;}
    if(!parsed.domains.length){p88Id('drSimpleState').textContent='Добавь хотя бы один домен.';return;}
    const scope=window.RFNativeCatalogP89.acceptsDomains({usable:true,zone:chosenDNS.zone},parsed.domains);
    if(!scope.ok){p88Id('drSimpleState').textContent=scope.reason;return;}
    const existing=new Set((config.rules||[]).map(r=>String(r.domain).toLowerCase()));
    const duplicate=parsed.domains.find(domain=>existing.has(domain));
    if(duplicate){p88Id('drSimpleState').textContent='Домен '+duplicate+' уже есть в правилах. Открой расширенные настройки, чтобы изменить его DNS.';return;}
    if(!Array.isArray(config.upstreams))config.upstreams=[];
    if(!Array.isArray(config.rules))config.rules=[];
    let upstream=config.upstreams.find(u=>u.transport==='native' && u.address===selected);
    if(!upstream){
      let n=1;while(config.upstreams.some(u=>u.id==='native-'+n))n++;
      upstream={id:'native-'+n,transport:'native',address:selected};
      config.upstreams.push(upstream);
    }
    for(const domain of parsed.domains){
      let n=1;while(config.rules.some(r=>r.id==='domain-'+n))n++;
      config.rules.push({id:'domain-'+n,domain,match:'suffix',upstream:upstream.id,priority:100});
    }
    p88Id('drSimpleDomains').value='';
    markDirty();
    p88Id('drSimpleState').textContent='Добавлено '+parsed.domains.length+' доменов в черновик. Нажми «Сохранить настройки». DNS Keenetic не изменён.';
  });
  p88Id('drSimpleSave').addEventListener('click',()=>p88Id('saveButton').click());
  p88Id('drSimpleProbe').addEventListener('click',async()=>{
    const selected=p88Id('drSimpleDNS').value;
    const parts=selected.startsWith('native://')?selected.slice(9).split('/'):[];
    if(parts.length!==3 || parts[1]!=='.'){
      p88Id('drSimpleProbeResult').textContent='Проверка example.com доступна только для DNS зоны «.». DNS других зон не проверяется чужим доменом.';
      return;
    }
    p88Id('drSimpleProbeResult').textContent='Проверяю DNS-запросом example.com…';
    try{
      const q=new URLSearchParams({instance:parts[0],hint:parts[2]});
      const res=await request('keenetic/native-dns/probe?'+q.toString());
      p88Id('drSimpleProbeResult').textContent=res.dns_success ?
        'DNS отвечает · '+res.latency_ms+' мс · проверка шифрования не выполнялась' :
        'DNS не отвечает корректно · '+(res.error||('RCODE '+res.rcode));
    }catch(err){p88Id('drSimpleProbeResult').textContent='Ошибка проверки: '+err.message;}
  });
  p88Id('drSimpleAdvanced').addEventListener('click',()=>p88Id('drModeAdvanced').click());
  p88RefreshStatus();
  p88Scan().catch(err=>{p88Id('drSimpleState').textContent='Ошибка загрузки: '+err.message;});

  load().catch(err=>feedback('Ошибка загрузки: '+err.message));
  shadowRefresh().catch(()=>{});
})();

/* RouterForge DNS Router unified UI bridge; presentation only.
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
