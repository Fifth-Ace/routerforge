import { esc } from '../utils.js';

function stateInfo(item){
  switch(item.state){
    case 'installed': return {label:'Установлен', cls:'ok'};
    case 'installed_external': return {label:item.service_running?'Установлен · работает':'Установлен', cls:item.service_running?'ok':'warn'};
    case 'planned': return {label:'Запланирован', cls:'accent'};
    case 'incompatible': return {label:'Несовместим', cls:'error'};
    default: return {label:'Доступен', cls:''};
  }
}

function localWebURL(port){
  if(!port)return '';
  const host=location.hostname.includes(':')?`[${location.hostname}]`:location.hostname;
  return `http://${host}:${port}`;
}

function capabilityLabel(v){
  const map={
    detect:'обнаружение', version:'версия', 'service-status':'служба',
    'open-ui':'web UI', 'install-preview':'план установки',
    'conflict-detection':'конфликты', catalog:'каталог',
    'integration-detection':'интеграции', 'install-plan-preview':'план установки',
    planned:'в разработке'
  };
  return map[v]||v;
}

function installPlan(item){
  const p=item.install||{};
  if(!p.method)return '';
  const rows=[];
  if(p.repository_url)rows.push(`<div><span>Feed</span><code>${esc(p.repository_url)}</code></div>`);
  if(p.installer_url)rows.push(`<div><span>Installer</span><code>${esc(p.installer_url)}</code></div>`);
  if((p.packages||[]).length)rows.push(`<div><span>Пакеты</span><code>${esc(p.packages.join(', '))}</code></div>`);
  const notes=(p.notes||[]).map(n=>`<li>${esc(n)}</li>`).join('');
  return `<details class="catalog-plan"><summary>План установки</summary>
    <div class="catalog-plan-grid"><div><span>Метод</span><code>${esc(p.method)}</code></div>${rows.join('')}</div>
    ${notes?`<ul>${notes}</ul>`:''}
    <div class="catalog-readonly-note">Предпросмотр: выполнение install/update/remove на этом этапе отключено.</div>
  </details>`;
}

function card(item){
  const st=stateInfo(item);
  const caps=(item.capabilities||[]).map(c=>`<span class="pill">${esc(capabilityLabel(c))}</span>`).join('');
  const hints=(item.compatibility?.hints||[]).map(h=>`<li>${esc(h)}</li>`).join('');
  const web=item.installed&&item.web_port
    ?`<a class="btn" target="_blank" rel="noopener" href="${esc(localWebURL(item.web_port))}">Открыть UI :${item.web_port}</a>`
    :'';
  const version=item.version?`<span class="catalog-version">v${esc(item.version)}</span>`:'';
  const source=item.source==='official'?'официальный проект':item.source==='builtin'?'встроено':'DNS Monitor';
  return `<article class="catalog-card">
    <div class="catalog-card-head">
      <div><div class="catalog-card-title">${esc(item.name)} ${version}</div><div class="catalog-card-category">${esc(item.category)} · ${esc(source)}</div></div>
      <span class="pill ${st.cls}">${st.label}</span>
    </div>
    <div class="catalog-card-description">${esc(item.description||'')}</div>
    ${item.service?`<div class="catalog-service mono">${esc(item.service)}</div>`:''}
    <div class="catalog-capabilities">${caps}</div>
    ${hints?`<div class="catalog-requirements"><div>Совместимость / требования</div><ul>${hints}</ul></div>`:''}
    ${installPlan(item)}
    <div class="catalog-card-actions">
      ${web}
      ${item.project_url?`<a class="btn" target="_blank" rel="noopener" href="${esc(item.project_url)}">Проект</a>`:''}
    </div>
  </article>`;
}

export function renderCatalog(data,ui){
  const all=[...(data.modules||[]),...(data.integrations||[])];
  const categories=[...new Set(all.map(x=>x.category).filter(Boolean))].sort((a,b)=>a.localeCompare(b,'ru'));
  const q=(ui.catalogSearch||'').trim().toLowerCase();
  const kind=ui.catalogKind||'all';
  const category=ui.catalogCategory||'all';
  const items=all.filter(x=>{
    if(kind!=='all'&&x.kind!==kind)return false;
    if(category!=='all'&&x.category!==category)return false;
    if(!q)return true;
    return `${x.name} ${x.category} ${x.description} ${(x.capabilities||[]).join(' ')}`.toLowerCase().includes(q);
  });
  const categoryOptions=`<option value="all">Все категории</option>${categories.map(c=>`<option value="${esc(c)}" ${category===c?'selected':''}>${esc(c)}</option>`).join('')}`;
  return `<h1 class="page-title">Модули и интеграции</h1>
    <div class="catalog-banner">
      <strong>Marketplace foundation</strong>
      <span>Сейчас каталог работает только на чтение: обнаруживает установленные системы и показывает безопасный план будущей установки. Никаких opkg/install/remove из web пока не выполняется.</span>
    </div>
    <div class="toolbar">
      <input class="input" id="catalogSearch" placeholder="Поиск модуля или интеграции..." value="${esc(ui.catalogSearch||'')}">
      <select class="select" id="catalogKind">
        <option value="all" ${kind==='all'?'selected':''}>Всё</option>
        <option value="module" ${kind==='module'?'selected':''}>Модули DNS Monitor</option>
        <option value="integration" ${kind==='integration'?'selected':''}>Сторонние интеграции</option>
      </select>
      <select class="select" id="catalogCategory">${categoryOptions}</select>
    </div>
    <div class="catalog-summary">
      <span>${items.length} элементов</span>
      <span>${all.filter(x=>x.state==='installed_external'||x.state==='installed').length} обнаружено</span>
      <span>режим: read-only</span>
    </div>
    <div class="catalog-grid">${items.map(card).join('')||'<div class="panel"><div class="empty">Ничего не найдено</div></div>'}</div>`;
}
