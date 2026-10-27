'use strict';
// No external libraries. All user-derived labels are inserted with textContent.
(() => {
  let config = {schema_version: 1, upstreams: [], rules: []};
  let revision = '';
  let dirty = false;
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
  function markDirty() { dirty = true; feedback('Изменения не сохранены. DNS-трафик не затронут.'); render(); }
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
    table('upstreamList',['ID','Протокол','Адрес'],config.upstreams.map(u=>[u.id,u.transport,u.address]),i=>{
      const id = config.upstreams[i].id;
      if (config.rules.some(r=>r.upstream===id)) {feedback('Нельзя удалить используемый пул. Сначала удали правила.');return;}
      config.upstreams.splice(i,1);markDirty();
    });
    table('ruleList',['ID','Домен','Совпадение','Пул','Приоритет'],config.rules.map(r=>[r.id,r.domain,r.match,r.upstream,r.priority]),i=>{config.rules.splice(i,1);markDirty();});
    el('revision').textContent = (dirty ? '● Не сохранено · ' : '') + revision.substring(0,23);
  }
  async function load() {
    if (dirty && !confirm('Отменить несохранённые изменения?')) return;
    const data = await request('config');
    config = data.config;revision = data.revision; dirty = false;render();feedback('Конфигурация загружена · DNS Router не активирован');
  }
  el('upstreamForm').addEventListener('submit',e=>{
    e.preventDefault();const v=Object.fromEntries(new FormData(e.currentTarget));
    if(config.upstreams.some(u=>u.id===v.id)){feedback('ID пула уже существует');return;}
    config.upstreams.push({id:v.id,transport:v.transport,address:v.address});e.currentTarget.reset();markDirty();
  });
  el('ruleForm').addEventListener('submit',e=>{
    e.preventDefault();const v=Object.fromEntries(new FormData(e.currentTarget));
    if(config.rules.some(r=>r.id===v.id)){feedback('ID правила уже существует');return;}
    config.rules.push({id:v.id,domain:v.domain,match:v.match,upstream:v.upstream,priority:Number(v.priority)});e.currentTarget.reset();markDirty();
  });
  el('saveButton').addEventListener('click',async()=>{
    try {
      const response = await request('config', {method:'PUT', headers:{'Content-Type':'application/json','X-RouterForge-Action':'save','If-Match':'"'+revision+'"'},body:JSON.stringify(config)});
      revision = response.revision;dirty = false;render();feedback('Сохранено атомарно; DNS Router остаётся неактивным');
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
  el('exportButton').addEventListener('click',async()=>{
    try{
      const data = await request('config');
      const blob=new Blob([JSON.stringify(data.config,null,2)+'\n'],{type:'application/json'});
      const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download='routerforge-dns-router.json';a.click();URL.revokeObjectURL(url);
      feedback('Экспортирована сохранённая конфигурация');
    }catch(err){feedback(err.message);}
  });
  load().catch(err=>feedback('Ошибка загрузки: '+err.message));
})();
