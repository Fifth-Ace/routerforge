import { esc } from '../utils.js';

const ACRONYMS={
  'awg-manager':'AWG',
  nfqws2:'NQ2',
  nfqws:'NQ1',
  'hydraroute-neo':'HRN',
  'dns-core':'DNS',
  marketplace:'MKT',
  system:'SYS',
  thermal:'TMP',
  storage:'DSK',
  network:'NET',
  admin:'ADM',
  profiling:'PRF'
};

function stateInfo(item){
  switch(item.state){
    case 'installed_external':
      return item.service_running
        ? {label:'ACTIVE', cls:'active', detail:'Установлен · служба работает'}
        : {label:'INSTALLED', cls:'warn', detail:'Установлен · служба не обнаружена'};
    case 'installed': return {label:'BUILT-IN', cls:'active', detail:'Встроено в DNS Monitor'};
    case 'planned': return {label:'PLANNED', cls:'info', detail:'Запланированный модуль'};
    case 'incompatible': return {label:'INCOMPATIBLE', cls:'error', detail:'Требования не выполнены'};
    case 'broken': return {label:'BROKEN', cls:'error', detail:'Установка обнаружена, состояние некорректно'};
    default: return {label:'AVAILABLE', cls:'neutral', detail:'Доступно для установки'};
  }
}

function sourceLabel(item){
  if(item.source==='official')return 'official';
  if(item.source==='builtin')return 'builtin';
  return item.source||'dns-monitor';
}

function localWebURL(port){
  if(!port)return '';
  const host=location.hostname.includes(':')?`[${location.hostname}]`:location.hostname;
  return `${location.protocol==='https:'?'https:':'http:'}//${host}:${port}`;
}

function acronym(item){
  return ACRONYMS[item.id]||String(item.name||'EXT').replace(/[^A-Za-z0-9]/g,'').slice(0,3).toUpperCase()||'EXT';
}

function itemVersion(item){
  return item.version?`v${esc(item.version)}`:'—';
}

function packageText(item){
  const packages=item.install?.packages||item.detection?.packages||[];
  return packages.length?packages.join(', '):'—';
}

function serviceText(item){
  return item.service||item.detection?.services?.[0]||'—';
}

function compatibilityText(item){
  const hints=item.compatibility?.hints||[];
  if(item.compatibility?.status==='built-in')return 'built-in';
  if(item.compatibility?.status==='planned')return 'planned';
  if(!hints.length)return item.compatibility?.status||'not evaluated';
  return hints.join(' · ');
}

function plannerLines(item){
  const lines=[];
  lines.push(`<div><span class="market-log-dim">[CATALOG]</span> source=${esc(sourceLabel(item))} state=${esc(item.state||'available')}</div>`);
  if(item.version)lines.push(`<div class="market-log-ok">[DETECT] version ${esc(item.version)}</div>`);
  const packages=item.detection?.packages||item.install?.packages||[];
  if(packages.length)lines.push(`<div><span class="market-log-dim">[PACKAGE]</span> ${esc(packages.join(', '))}${item.installed?' · detected':' · not installed'}</div>`);
  if(item.service)lines.push(`<div class="${item.service_running?'market-log-ok':'market-log-dim'}">[SERVICE] ${esc(item.service)} · ${item.service_running?'running':'not running'}</div>`);
  if(item.web_port)lines.push(`<div><span class="market-log-dim">[WEB]</span> port ${esc(String(item.web_port))}${item.installed?' · exposed by integration':' · expected default'}</div>`);
  if(item.install?.repository_url)lines.push(`<div><span class="market-log-info">[PLAN]</span> feed ${esc(item.install.repository_url)}</div>`);
  if(item.install?.installer_url)lines.push(`<div><span class="market-log-info">[PLAN]</span> installer ${esc(item.install.installer_url)}</div>`);
  if(item.install?.method)lines.push(`<div><span class="market-log-info">[PLAN]</span> method=${esc(item.install.method)}</div>`);
  lines.push('<div class="market-log-warn">[SAFE] preview-only: no commands will be executed</div>');
  return lines.join('');
}

function plannerModal(item){
  const p=item.install||{};
  const hints=item.compatibility?.hints||[];
  const packages=p.packages||item.detection?.packages||[];
  const notes=p.notes||[];
  const discovery=item.installed
    ? `<span class="market-check-state ok">DETECTED</span>`
    : `<span class="market-check-state neutral">NOT INSTALLED</span>`;
  const service=item.service
    ? `<div class="market-check-row"><span>Service</span><code>${esc(item.service)}</code></div>`
    : '';
  const web=item.web_port
    ? `<div class="market-check-row"><span>Web UI</span><code>:${esc(String(item.web_port))}</code></div>`
    : '';
  const reqRows=hints.length
    ? hints.map(h=>`<div class="market-check-row"><span>Requirement</span><code>${esc(h)}</code></div>`).join('')
    : '<div class="market-check-row"><span>Requirements</span><code>not declared</code></div>';
  const planRows=[
    p.method?`<div class="market-check-row"><span>Method</span><code>${esc(p.method)}</code></div>`:'',
    packages.length?`<div class="market-check-row"><span>Packages</span><code>${esc(packages.join(', '))}</code></div>`:'',
    p.repository_url?`<div class="market-check-row"><span>Feed</span><code>${esc(p.repository_url)}</code></div>`:'',
    p.installer_url?`<div class="market-check-row"><span>Installer</span><code>${esc(p.installer_url)}</code></div>`:''
  ].join('');
  const noteRows=notes.length?`<div class="market-plan-notes">${notes.map(n=>`<div>• ${esc(n)}</div>`).join('')}</div>`:'';

  return `<details class="market-plan-details">
    <summary class="market-btn ${item.installed?'':'primary'}">${item.installed?'Подробнее':'План установки'}</summary>
    <div class="market-modal-overlay" onclick="this.closest('details').removeAttribute('open')">
      <section class="market-modal" role="dialog" aria-modal="true" aria-label="План ${esc(item.name)}" onclick="event.stopPropagation()">
        <header class="market-modal-head">
          <div class="market-modal-title-row">
            <span class="market-status-dot info"></span>
            <span class="market-modal-prefix mono">Install Planner ::</span>
            <strong>${esc(item.name)}</strong>
            ${item.version?`<span class="market-version">v${esc(item.version)}</span>`:''}
          </div>
          <div class="market-modal-target mono"><span>MODE:</span><strong>READ-ONLY PREVIEW</strong><button class="market-modal-close" type="button" onclick="this.closest('details').removeAttribute('open')">×</button></div>
        </header>
        <div class="market-modal-body">
          <div class="market-modal-checks">
            <div>
              <div class="market-section-label">Execution Stage Checklist</div>
              <div class="market-check-card">
                <div class="market-check-head"><strong>1. Discovery</strong>${discovery}</div>
                <div class="market-check-row"><span>State</span><code>${esc(item.state||'available')}</code></div>
                <div class="market-check-row"><span>Version</span><code>${itemVersion(item)}</code></div>
                ${service}${web}
              </div>
              <div class="market-check-card">
                <div class="market-check-head"><strong>2. Compatibility</strong><span class="market-check-state info">REQUIREMENTS</span></div>
                ${reqRows}
              </div>
              <div class="market-check-card">
                <div class="market-check-head"><strong>3. Install Plan</strong><span class="market-check-state info">STAGED</span></div>
                ${planRows||'<div class="market-check-row"><span>Plan</span><code>not available</code></div>'}
                ${noteRows}
              </div>
            </div>
            <div class="market-safety-box mono"><strong>Safety boundary</strong><span>На этом этапе Marketplace только читает состояние и строит план. opkg/install/update/remove не выполняются.</span></div>
          </div>
          <div class="market-modal-console">
            <div class="market-console-head mono"><span><i></i>catalog-plan-stream</span><span>NO EXEC</span></div>
            <div class="market-console-body mono">${plannerLines(item)}</div>
            <div class="market-console-foot">
              <div><span>Plan Status</span><strong>Preview ready</strong></div>
              <div class="market-progress"><span></span></div>
            </div>
          </div>
        </div>
        <footer class="market-modal-foot">
          <div class="mono">Changes applied: <strong>0</strong></div>
          <div class="market-modal-actions">
            ${item.project_url?`<a class="market-btn" target="_blank" rel="noopener" href="${esc(item.project_url)}">Открыть проект</a>`:''}
            <button class="market-btn" type="button" onclick="this.closest('details').removeAttribute('open')">Закрыть</button>
            <button class="market-btn disabled" type="button" disabled>Установка отключена</button>
          </div>
        </footer>
      </section>
    </div>
  </details>`;
}

function moduleTree(modules){
  const ordered=[...(modules||[])].sort((a,b)=>{
    const ai=a.state==='installed'?0:a.state==='planned'?2:1;
    const bi=b.state==='installed'?0:b.state==='planned'?2:1;
    return ai-bi||String(a.name).localeCompare(String(b.name),'ru');
  });
  if(!ordered.length)return '<div class="market-tree-empty">Registry empty</div>';
  return ordered.map((m,i)=>{
    const last=i===ordered.length-1;
    const st=stateInfo(m);
    return `<div class="market-tree-row ${m.state==='installed'?'is-loaded':''}">
      <span class="market-tree-branch">${last?'└──':'├──'}</span>
      <span class="market-tree-led ${st.cls}"></span>
      <span class="market-tree-name">${esc(m.name)}</span>
      <span class="market-tree-state">[${esc(st.label)}]</span>
    </div>`;
  }).join('');
}

function card(item){
  const st=stateInfo(item);
  const web=item.installed&&item.web_port
    ? `<a class="market-btn" target="_blank" rel="noopener" href="${esc(localWebURL(item.web_port))}">Открыть UI :${esc(String(item.web_port))}</a>`
    : '';
  const project=item.project_url
    ? `<a class="market-btn compact" target="_blank" rel="noopener" href="${esc(item.project_url)}">Проект</a>`
    : '';
  const serviceState=item.service
    ? `<span class="${item.service_running?'good':'warn'}">${item.service_running?'RUNNING':'NOT RUNNING'}</span>`
    : '<span>—</span>';
  const compat=compatibilityText(item);
  return `<article class="market-card ${item.state==='incompatible'?'is-dimmed':''}">
    <div>
      <div class="market-card-head">
        <div class="market-card-identity">
          <div class="market-card-icon">${esc(acronym(item))}</div>
          <div><h3>${esc(item.name)}</h3><div class="market-card-source mono">${esc(sourceLabel(item))} / ${esc(item.category||item.kind||'extension')}</div></div>
        </div>
        <span class="market-state ${st.cls}">${esc(st.label)}</span>
      </div>
      <p class="market-card-description">${esc(item.description||'')}</p>
      <div class="market-tech-box mono">
        <div><span>Version</span><strong>${itemVersion(item)}</strong></div>
        <div><span>Package</span><strong title="${esc(packageText(item))}">${esc(packageText(item))}</strong></div>
        <div><span>Service</span><strong>${serviceState}</strong></div>
        <div><span>Compat</span><strong title="${esc(compat)}">${esc(compat)}</strong></div>
      </div>
    </div>
    <div class="market-card-foot">
      <div class="market-card-actions">${web}${plannerModal(item)}${project}</div>
      <span class="market-card-kind mono">${esc(item.kind||'extension')}</span>
    </div>
  </article>`;
}

export function renderCatalog(data,ui){
  const modules=data.modules||[];
  const integrations=data.integrations||[];
  const all=[...integrations,...modules];
  const categories=[...new Set(all.map(x=>x.category).filter(Boolean))].sort((a,b)=>a.localeCompare(b,'ru'));
  const q=(ui.catalogSearch||'').trim().toLowerCase();
  const kind=ui.catalogKind||'all';
  const category=ui.catalogCategory||'all';
  const items=all.filter(x=>{
    if(kind!=='all'&&x.kind!==kind)return false;
    if(category!=='all'&&x.category!==category)return false;
    if(!q)return true;
    return `${x.name} ${x.category} ${x.description} ${x.version||''} ${(x.detection?.packages||[]).join(' ')}`.toLowerCase().includes(q);
  });
  const externalInstalled=integrations.filter(x=>x.state==='installed_external').length;
  const activeServices=integrations.filter(x=>x.service_running).length;
  const available=integrations.filter(x=>x.state==='available').length;
  const generated=data.generated_at?new Date(data.generated_at):null;
  const generatedText=generated&&!Number.isNaN(generated.getTime())?generated.toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit',second:'2-digit'}):'—';
  const categoryOptions=`<option value="all">Все категории</option>${categories.map(c=>`<option value="${esc(c)}" ${category===c?'selected':''}>${esc(c)}</option>`).join('')}`;

  return `<section class="marketplace-console">
    <aside class="market-sidebar">
      <div>
        <div class="market-side-label">Core Modules Tree</div>
        <div class="market-tree mono">
          <div class="market-tree-root"><span class="market-tree-led active"></span><strong>DNS Monitor Core</strong></div>
          ${moduleTree(modules)}
        </div>

        <div class="market-side-section">
          <div class="market-side-label">Registry Summary</div>
          <div class="market-side-stat active"><span>External installed</span><strong>${externalInstalled}</strong></div>
          <div class="market-side-stat"><span>Services running</span><strong>${activeServices}</strong></div>
          <div class="market-side-stat"><span>Available</span><strong>${available}</strong></div>
          <div class="market-side-stat"><span>Catalog entries</span><strong>${all.length}</strong></div>
        </div>
      </div>

      <div class="market-telemetry mono">
        <div class="market-side-label">Marketplace Safety</div>
        <div><span>Catalog API</span><strong class="good">ONLINE</strong></div>
        <div><span>Mutation API</span><strong>DISABLED</strong></div>
        <div><span>Mode</span><strong>${data.read_only?'READ-ONLY':'UNKNOWN'}</strong></div>
      </div>
    </aside>

    <div class="market-main">
      <header class="market-toolbar">
        <div class="market-search-wrap">
          <span class="market-search-icon">⌕</span>
          <input class="market-search" id="catalogSearch" placeholder="Поиск: awg, nfqws, routing, thermal..." value="${esc(ui.catalogSearch||'')}">
        </div>
        <div class="market-toolbar-controls">
          <select class="market-select" id="catalogKind">
            <option value="all" ${kind==='all'?'selected':''}>Все расширения</option>
            <option value="integration" ${kind==='integration'?'selected':''}>Интеграции</option>
            <option value="module" ${kind==='module'?'selected':''}>Модули Core</option>
          </select>
          <select class="market-select" id="catalogCategory">${categoryOptions}</select>
          <span class="market-repo-state"><i></i>CATALOG ONLINE</span>
        </div>
      </header>

      <div class="market-content">
        <div class="market-heading">
          <div><h1>Marketplace</h1><p>Модули DNS Monitor и интеграции сторонних проектов. Состояние читается с самого роутера.</p></div>
          <span class="market-readonly-badge">READ-ONLY FOUNDATION</span>
        </div>
        <div class="market-grid">${items.map(card).join('')||'<div class="market-empty">Ничего не найдено</div>'}</div>
      </div>

      <footer class="market-footer mono">
        <div><span>Phase:</span><strong>${esc(data.phase||'catalog')}</strong><span class="market-footer-sep">|</span><span>Last refresh:</span><strong>${esc(generatedText)}</strong></div>
        <div class="market-api-ok"><i></i>Marketplace API: <strong>ONLINE</strong></div>
      </footer>
    </div>
  </section>`;
}
