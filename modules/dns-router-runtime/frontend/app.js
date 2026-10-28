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
      if (config.rules.some(r=>r.upstream===id) || config.upstreams.some(u=>(u.fallbacks||[]).includes(id))) {feedback('Нельзя удалить используемый пул. Сначала убери правила и резервные ссылки.');return;}
      config.upstreams.splice(i,1);markDirty();
    });
    table('ruleList',['ID','Домен','Совпадение','Пул','Приоритет'],config.rules.map(r=>[r.id,r.domain,r.match,r.upstream,r.priority]),i=>{config.rules.splice(i,1);markDirty();});
    const listener = config.listener || {mode:'loopback',listen:'127.0.0.1:55353',allowed_clients:[]};
    el('listenerMode').value=listener.mode;
    el('listenerListen').value=listener.listen;
    el('listenerACL').value=(listener.allowed_clients||[]).join(',');
    el('revision').textContent = (dirty ? '● Не сохранено · ' : '') + revision.substring(0,23);
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
  load().catch(err=>feedback('Ошибка загрузки: '+err.message));
  shadowRefresh().catch(()=>{});
})();
