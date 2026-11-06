<script>
  import {onMount} from 'svelte';
  // State-driven migration: every displayed value/disabled flag/class is Svelte state.
  // The original Antiscan app.js is not loaded or shipped.
  const INITIAL = {"targetState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"refresh":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"notice":{"className":"notice","hidden":true,"disabled":false,"dataset":{},"style":{}},"actionNotice":{"className":"notice action-notice","hidden":true,"disabled":false,"dataset":{},"style":{}},"version":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"blockedIps":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"blockedSubnets":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"candidateCount":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-1":{"className":"tab active","hidden":false,"disabled":false,"dataset":{"tab":"overview"},"style":{}},"rf-generated-2":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"diagnostics"},"style":{}},"rf-generated-3":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"blocked"},"style":{}},"rf-generated-4":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"lists"},"style":{}},"rf-generated-5":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"geo"},"style":{}},"rf-generated-6":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"schedule"},"style":{}},"rf-generated-7":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"config"},"style":{}},"rf-generated-8":{"className":"tab","hidden":false,"disabled":false,"dataset":{"tab":"history"},"style":{}},"rf-generated-9":{"className":"tab-page active","hidden":false,"disabled":false,"dataset":{"page":"overview"},"style":{}},"ipsetBinary":{"className":"mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"runtimeSummary":{"className":"kv-grid","hidden":false,"disabled":false,"dataset":{},"style":{}},"riskSummary":{"className":"risk-summary","hidden":false,"disabled":false,"dataset":{},"style":{}},"lifecycleState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"lifecycleHint":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"startAntiscan":{"className":"button success","hidden":false,"disabled":true,"dataset":{},"style":{}},"reloadAntiscan":{"className":"button primary","hidden":false,"disabled":true,"dataset":{},"style":{}},"restartAntiscan":{"className":"button","hidden":false,"disabled":true,"dataset":{},"style":{}},"stopAntiscan":{"className":"button danger","hidden":false,"disabled":true,"dataset":{},"style":{}},"operationState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-10":{"className":"button","hidden":false,"disabled":false,"dataset":{"operation":"update_rules"},"style":{}},"rf-generated-11":{"className":"button","hidden":false,"disabled":false,"dataset":{"operation":"read_candidates"},"style":{}},"rf-generated-12":{"className":"button","hidden":false,"disabled":false,"dataset":{"operation":"read_ndm_ipsets"},"style":{}},"rf-generated-13":{"className":"button","hidden":false,"disabled":false,"dataset":{"operation":"save_ipsets"},"style":{}},"operationOutput":{"className":"boundary-copy mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"sets":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-14":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"diagnostics"},"style":{}},"diagnosticsState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"reloadDiagnostics":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"diagnosticsMeta":{"className":"diagnostics-meta mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"diagnosticsChecks":{"className":"diagnostics-list empty-state","hidden":false,"disabled":false,"dataset":{},"style":{}},"rciTokenState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"reloadRCIToken":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"rciTokenSummary":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"rciTokenMeta":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"rciTokenInput":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"setRCIToken":{"className":"button primary","hidden":false,"disabled":true,"dataset":{},"style":{}},"checkRCIToken":{"className":"button","hidden":false,"disabled":true,"dataset":{},"style":{}},"deleteRCIToken":{"className":"button danger","hidden":false,"disabled":true,"dataset":{},"style":{}},"compatibilityState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"reloadCompatibility":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"compatibilityMeta":{"className":"diagnostics-meta mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"compatibilityGrid":{"className":"kv-grid","hidden":false,"disabled":false,"dataset":{},"style":{}},"compatibilityWarnings":{"className":"warning-list","hidden":false,"disabled":false,"dataset":{},"style":{}},"diagnosticsContract":{"className":"kv-grid","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-15":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"inspect"},"style":{}},"inspectIp":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"inspectButton":{"className":"button primary","hidden":false,"disabled":false,"dataset":{},"style":{}},"inspectResult":{"className":"inspect-result empty","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-16":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"blocked"},"style":{}},"blockedSet":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"value":"ascn_ips"},"reloadBlocked":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"blockedFilter":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"blockedMeta":{"className":"mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"blockedEntries":{"className":"entry-table empty-state","hidden":false,"disabled":false,"dataset":{},"style":{}},"flushTarget":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"value":"ips"},"reloadFlushPreview":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"flushState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"flushMeta":{"className":"diagnostics-meta mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"flushWarnings":{"className":"warning-list","hidden":false,"disabled":false,"dataset":{},"style":{}},"flushSelected":{"className":"button danger","hidden":false,"disabled":true,"dataset":{},"style":{}},"rf-generated-17":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"lists"},"style":{}},"customListName":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"value":"blacklist"},"reloadCustomList":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"customListEntry":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"addCustomListEntry":{"className":"button primary","hidden":false,"disabled":false,"dataset":{},"style":{}},"reloadCustomListRuntime":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"clearCustomList":{"className":"button danger","hidden":false,"disabled":false,"dataset":{},"style":{}},"customListMeta":{"className":"diagnostics-meta mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"customListEntries":{"className":"entry-table empty-state","hidden":false,"disabled":false,"dataset":{},"style":{}},"runtimeListsTitle":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"runtimeListsDescription":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"listSet":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"value":"ascn_custom_exclude"},"reloadList":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"listFilter":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"listMeta":{"className":"mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"listEntries":{"className":"entry-table empty-state","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-18":{"className":"button primary","hidden":false,"disabled":false,"dataset":{"operation":"update_ipsets","scope":"geo"},"style":{}},"rf-generated-19":{"className":"button","hidden":false,"disabled":false,"dataset":{"operation":"retry_load_geo"},"style":{}},"rf-generated-20":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"protection"},"style":{}},"protection":{"className":"kv-grid","hidden":false,"disabled":false,"dataset":{},"style":{}},"warnings":{"className":"warning-list","hidden":false,"disabled":false,"dataset":{},"style":{}},"rf-generated-21":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"schedule"},"style":{}},"schedulerState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"reloadScheduler":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"schedulerMeta":{"className":"diagnostics-meta mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"schedulerTaskRows":{"className":"scheduler-list empty-state","hidden":false,"disabled":false,"dataset":{},"style":{}},"resetScheduler":{"className":"button","hidden":false,"disabled":true,"dataset":{},"style":{}},"applyScheduler":{"className":"button primary","hidden":false,"disabled":true,"dataset":{},"style":{}},"rf-generated-22":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"config"},"style":{}},"configForm":{"className":"config-workspace","hidden":false,"disabled":false,"dataset":{},"style":{}},"configState":{"className":"state neutral","hidden":false,"disabled":false,"dataset":{},"style":{}},"configHash":{"className":"mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"configMobileRisk":{"className":"config-risk good","hidden":false,"disabled":false,"dataset":{},"style":{}},"cfgIspInterfaces":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgPorts":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgForwardedPorts":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgEnableIpsBan":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgRulesMask":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgRecentTime":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgRecentHitcount":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgRecentLimit":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgRecentBanTime":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgCandidateStorage":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgDifferentThreshold":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgSubnetBanTime":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgEnableHoneypot":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgHoneypotPorts":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgHoneypotBanTime":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgUseCustomExclude":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgCustomMode":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"value":"0"},"cfgGeoMode":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"value":"0"},"cfgGeoCountries":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgGeoExcludeCountries":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgIPSetsDirectory":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgSaveIPSets":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgSaveOnExit":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgReadNDM":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"cfgLockoutBanTime":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{},"checked":false,"value":""},"configHint":{"className":"","hidden":false,"disabled":false,"dataset":{},"style":{}},"resetConfig":{"className":"button","hidden":false,"disabled":true,"dataset":{},"style":{}},"applyConfig":{"className":"button primary","hidden":false,"disabled":true,"dataset":{},"style":{}},"rf-generated-23":{"className":"tab-page","hidden":false,"disabled":false,"dataset":{"page":"history"},"style":{}},"reloadHistory":{"className":"button","hidden":false,"disabled":false,"dataset":{},"style":{}},"historyMeta":{"className":"mono muted","hidden":false,"disabled":false,"dataset":{},"style":{}},"historyEntries":{"className":"history-list empty-state","hidden":false,"disabled":false,"dataset":{},"style":{}}};
  let cells = $state(INITIAL);
  let activeTab = $state('overview');
  let schedulerRows = $state([]);
  // This compatibility state accessor bridges original *business functions* to
  // Svelte reactive values; it never writes directly to DOM nodes.
  const modelCache = new Map();
  function node(id){
    if(!cells[id])return null;
    if(modelCache.has(id))return modelCache.get(id);
    const state=cells[id];
    const classes=()=>new Set(String(state.className||'').split(/\s+/).filter(Boolean));
    const classList={
      add(...names){const c=classes();for(const name of names)c.add(name);state.className=[...c].join(' ');},
      remove(...names){const c=classes();for(const name of names)c.delete(name);state.className=[...c].join(' ');},
      toggle(name,force){const c=classes();const next=force===undefined?!c.has(name):!!force;if(next)c.add(name);else c.delete(name);state.className=[...c].join(' ');return next;},
      contains(name){return classes().has(name);}
    };
    const virtual=new Proxy(state,{
      get(target,k){return k==='classList'?classList:k==='value'?String(target.value ?? ''):target[k];},
      set(target,k,value){
        if(k==='innerHTML')target.textContent=undefined;
        if(k==='textContent')target.innerHTML=undefined;
        target[k]=value;
        return true;
      }
    });
    modelCache.set(id,virtual);return virtual;
  }
  const operationControlIds = ["rf-generated-10", "rf-generated-11", "rf-generated-12", "rf-generated-13", "rf-generated-18", "rf-generated-19"];
  const tabControlIds = ["rf-generated-1", "rf-generated-2", "rf-generated-3", "rf-generated-4", "rf-generated-5", "rf-generated-6", "rf-generated-7", "rf-generated-8"];
  const pageControlIds = ["rf-generated-9", "rf-generated-14", "rf-generated-15", "rf-generated-16", "rf-generated-17", "rf-generated-20", "rf-generated-21", "rf-generated-22", "rf-generated-23"];

const routerForgeParams = new URLSearchParams(window.location.search);
let routerForgeResizeObserver = null;
let routerForgeHeightFrame = 0;
let routerForgeHostObserver = null;

const routerForgeHostTokens = [
  '--rf-bg',
  '--rf-surface',
  '--rf-surface-2',
  '--rf-hover',
  '--rf-text',
  '--rf-muted',
  '--rf-border',
  '--rf-border-strong',
  '--rf-accent',
  '--rf-accent-soft',
  '--rf-accent-hover',
  '--rf-accent-border',
  '--rf-radius-panel',
  '--rf-radius-control',
  '--rf-radius-card',
  '--rf-panel-head-h',
  '--rf-control-gap',
  '--ui-body',
  '--ui-small',
  '--ui-xs',
  '--ui-micro',
  '--ui-panel-title',
  '--rf-type-metric'
];

function syncRouterForgeHostTheme() {
  if (window.parent === window) return false;
  try {
    const root = document.documentElement;
    const hostRoot = window.parent.document.documentElement;
    const hostStyle = window.parent.getComputedStyle(hostRoot);
    routerForgeHostTokens.forEach((token) => {
      const value = hostStyle.getPropertyValue(token).trim();
      if (value) root.style.setProperty(token, value);
    });
    ['theme', 'density', 'radius', 'uiScale'].forEach((key) => {
      const value = hostRoot.dataset[key];
      if (value) root.dataset[key] = value;
    });
    return true;
  } catch (_) {
    return false;
  }
}

const routerForgeThemes = {
  forge: {
    background: '#0b0d10', text: '#f5f7fa', surface: '#12151a', surface2: '#171b21',
    hover: '#1d2229', muted: '#8d98a4', border: '#29313a', borderStrong: '#36414d'
  },
  midnight: {
    background: '#08111b', text: '#edf5ff', surface: '#0f1824', surface2: '#152131',
    hover: '#1b2a3c', muted: '#8ca0b5', border: '#25384a', borderStrong: '#34516a'
  },
  graphite: {
    background: '#101113', text: '#f1f2f4', surface: '#17191c', surface2: '#1d2024',
    hover: '#25292e', muted: '#9299a2', border: '#30353c', borderStrong: '#414850'
  }
};

function routerForgeHex(value, fallback) {
  const normalized = String(value || '').trim().toLowerCase();
  return /^#[0-9a-f]{6}$/i.test(normalized) ? normalized : fallback;
}

function routerForgeMix(hex1, hex2, t) {
  const a = routerForgeHex(hex1, '#000000').slice(1);
  const b = routerForgeHex(hex2, '#ffffff').slice(1);
  const part = (value) => parseInt(value, 16);
  return '#' + [0, 2, 4].map((i) =>
    Math.round(
      part(a.slice(i, i + 2)) * (1 - t) +
      part(b.slice(i, i + 2)) * t
    ).toString(16).padStart(2, '0')
  ).join('');
}

function routerForgeLuminance(hex) {
  const value = routerForgeHex(hex, '#000000').slice(1);
  return [0, 2, 4].map((i) => parseInt(value.slice(i, i + 2), 16))
    .reduce((sum, channel, index) => sum + channel * [.2126, .7152, .0722][index], 0) / 255;
}

function routerForgeCustomTheme(background, text) {
  const light = routerForgeLuminance(background) > .55;
  return {
    background,
    text,
    surface: routerForgeMix(background, text, light ? .045 : .055),
    surface2: routerForgeMix(background, text, light ? .09 : .105),
    hover: routerForgeMix(background, text, light ? .13 : .15),
    muted: routerForgeMix(text, background, .48),
    border: routerForgeMix(background, text, light ? .18 : .19),
    borderStrong: routerForgeMix(background, text, light ? .27 : .29)
  };
}

function applyRouterForgeFrameSettings() {
  const root = document.documentElement;
  const theme = routerForgeParams.get('theme') || 'forge';
  const background = routerForgeHex(routerForgeParams.get('background'), '#0b0d10');
  const text = routerForgeHex(routerForgeParams.get('text'), '#f5f7fa');
  const accent = routerForgeHex(routerForgeParams.get('accent'), '#38bdf8');
  const palette = theme === 'custom'
    ? routerForgeCustomTheme(background, text)
    : (routerForgeThemes[theme] || routerForgeThemes.forge);

  root.dataset.theme = theme;
  root.style.setProperty('--rf-bg', palette.background);
  root.style.setProperty('--rf-surface', palette.surface);
  root.style.setProperty('--rf-surface-2', palette.surface2);
  root.style.setProperty('--rf-hover', palette.hover);
  root.style.setProperty('--rf-text', palette.text);
  root.style.setProperty('--rf-muted', palette.muted);
  root.style.setProperty('--rf-border', palette.border);
  root.style.setProperty('--rf-border-strong', palette.borderStrong);
  root.style.setProperty('--rf-accent', accent);

  const density = routerForgeParams.get('density');
  if (density === 'compact' || density === 'normal' || density === 'comfortable') {
    root.dataset.density = density;
  }

  const radius = routerForgeParams.get('radius');
  if (radius === 'sharp' || radius === 'default' || radius === 'soft') {
    root.dataset.radius = radius;
    root.style.setProperty('--rf-radius', radius === 'sharp' ? '2px' : radius === 'soft' ? '12px' : '8px');
  }

  syncRouterForgeHostTheme();
}

function reportRouterForgeModuleHeight() {
  if (window.parent === window) return;
  if (routerForgeHeightFrame) cancelAnimationFrame(routerForgeHeightFrame);
  routerForgeHeightFrame = requestAnimationFrame(() => {
    const workspace = document.querySelector('.as-page');
    if (!workspace) return;
    const height = Math.ceil(Math.max(workspace.scrollHeight, workspace.getBoundingClientRect().height, 360) + 2);
    window.parent.postMessage({
      type: 'routerforge-module-height',
      moduleId: 'antiscan-manager',
      height
    }, window.location.origin);
  });
}

function startRouterForgeBridge() {
  applyRouterForgeFrameSettings();
  reportRouterForgeModuleHeight();

  if (window.parent !== window && 'MutationObserver' in window) {
    try {
      const hostRoot = window.parent.document.documentElement;
      routerForgeHostObserver = new MutationObserver(() => syncRouterForgeHostTheme());
      routerForgeHostObserver.observe(hostRoot, {
        attributes: true,
        attributeFilter: ['style', 'class', 'data-theme', 'data-density', 'data-radius', 'data-ui-scale']
      });
    } catch (_) {
      routerForgeHostObserver = null;
    }
  }

  const workspace = document.querySelector('.as-page');
  if (workspace && 'ResizeObserver' in window) {
    routerForgeResizeObserver = new ResizeObserver(reportRouterForgeModuleHeight);
    routerForgeResizeObserver.observe(workspace);
  }
  window.addEventListener('resize', reportRouterForgeModuleHeight);
}

const setDescriptions = {
  ascn_candidates: 'Кандидаты для распределённой /24-защиты',
  ascn_ips: 'Прямые IP-блокировки',
  ascn_subnets: 'Заблокированные подсети /24',
  ascn_custom_exclude: 'Пользовательские исключения',
  ascn_custom_blacklist: 'Пользовательский чёрный список',
  ascn_custom_whitelist: 'Пользовательский белый список',
  ascn_geo_blacklist: 'Geo-чёрный список',
  ascn_geo_whitelist: 'Geo-белый список',
  ascn_geo_exclude: 'Geo-исключения',
  ascn_ndm_lockout: 'Блокировки Keenetic',
  ascn_honeypot: 'Ловушка'
};

const browserState = {
  blocked: null,
  lists: null,
  geo: null,
  customSource: null
};

let snapshot = $state(null);
let lifecycleBusy = false;
let operationBusy = false;
let customListBusy = false;
let configBusy = false;
let configDirty = false;
let configBaseline = '';
let configBaseSHA = '';
let historyLoaded = false;
let diagnosticsLoaded = false;
let compatibilityLoaded = false;
let rciTokenLoaded = false;
let rciTokenBusy = false;
let schedulerLoaded = $state(false);
let schedulerBusy = false;
let schedulerSnapshot = null;
let schedulerBaseline = '';
let schedulerDirty = false;
let flushPreviewLoaded = false;
let flushBusy = false;
let flushPreview = null;

function api(path) {
  return fetch(`../${path}`, {
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { Accept: 'application/json' }
  }).then(async (response) => {
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(payload.error || `${path}: HTTP ${response.status}`);
      error.payload = payload;
      throw error;
    }
    return payload;
  });
}

function mutate(path, body) {
  return fetch(`../${path}`, {
    method: 'POST',
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  }).then(async (response) => {
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(payload.error || `${path}: HTTP ${response.status}`);
      error.payload = payload;
      throw error;
    }
    return payload;
  });
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>'"]/g, (char) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  })[char]);
}

function fmtDuration(seconds) {
  if (seconds === undefined || seconds === null || seconds === '') return '—';
  const value = Number(seconds);
  if (!Number.isFinite(value) || value < 0) return '—';
  if (value === 0) return 'без таймаута';
  if (value % 86400 === 0) return `${value / 86400} дн.`;
  if (value % 3600 === 0) return `${value / 3600} ч.`;
  if (value % 60 === 0) return `${value / 60} мин.`;
  return `${value} сек.`;
}

function setByName(name) {
  return (snapshot?.ipsets || []).find((item) => item.name === name);
}

function countFor(name) {
  const item = setByName(name);
  if (!item?.exists || !item?.count_known) return '—';
  return String(item.count ?? 0);
}

function boolText(value) {
  return value ? 'Включено' : 'Выключено';
}

function fmtBytes(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number < 0) return '—';
  if (number < 1024) return `${number} B`;
  if (number < 1024 * 1024) return `${(number / 1024).toFixed(1)} KiB`;
  if (number < 1024 * 1024 * 1024) return `${(number / 1024 / 1024).toFixed(1)} MiB`;
  return `${(number / 1024 / 1024 / 1024).toFixed(1)} GiB`;
}

function fmtAuditTime(value) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return String(value || '—');
  return parsed.toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'medium' });
}

const exactMessageTranslations = {
  'POST required': 'Для этого действия требуется POST-запрос.',
  'authorized RouterForge Core request required': 'Действие разрешено только через авторизованный RouterForge Core.',
  'another Antiscan mutation is already running': 'Другая операция Antiscan уже выполняется. Дождитесь её завершения.',
  'multiple JSON values are not allowed': 'В запросе передано несколько JSON-значений.',
  'invalid unban request': 'Некорректный запрос на снятие блокировки.',
  'confirm must equal UNBAN': 'Не подтверждено снятие блокировки.',
  'invalid list-entry request': 'Некорректный запрос на изменение пользовательского списка.',
  'confirm must equal ADD': 'Не подтверждено добавление записи.',
  'invalid lifecycle request': 'Некорректная команда управления Antiscan.',
  'lifecycle action must be start, stop, reload or restart': 'Допустимы запуск, остановка, перечитывание конфигурации или штатный перезапуск.',
  'invalid operation request': 'Некорректная сервисная операция Antiscan.',
  'confirm must equal RUN': 'Сервисная операция не подтверждена.',
  'Antiscan must be running before this operation': 'Для этой сервисной операции Antiscan должен быть запущен.',
  'Antiscan stopped during the operation': 'Во время сервисной операции Antiscan неожиданно остановился.',
  'upstream operation returned while an Antiscan reload lock is still present': 'Штатная операция завершилась, но Antiscan всё ещё держит lock-файл.',
  'iptables binary not found': 'Исполняемый файл iptables не найден.',
  'Antiscan rule chain is already present; update_rules was not repeated to avoid duplicate jump rules.': 'Цепочка Antiscan уже на месте; повторный update_rules не запускался, чтобы не создать дублирующие переходы.',
  'ENABLE_IPS_BAN is disabled; there are no candidate sets to process.': 'ENABLE_IPS_BAN выключен — обрабатывать кандидатов сейчас нечего.',
  'READ_NDM_LOCKOUT_IPSETS is disabled; there is no active Keenetic lockout import to refresh.': 'Импорт блокировок Keenetic выключен — обновлять системный список сейчас нечего.',
  'SAVE_IPSETS is disabled; no persistent ipset export was requested.': 'SAVE_IPSETS выключен — сохранять runtime-наборы в файлы сейчас не требуется.',
  'No custom blocking or exclusion list is enabled; there is nothing to reload.': 'Пользовательские блокирующие списки и исключения выключены — перечитывать нечего.',
  'Geo blocking and Geo exclusions are disabled; there is nothing to download or reload.': 'Geo-блокировка и Geo-исключения выключены — обновлять Geo сейчас нечего.',
  'Antiscan was stopped; upstream restart will start it.': 'Antiscan был остановлен; штатная команда restart запустила его.',
  'Antiscan is already running; no lifecycle command was executed.': 'Antiscan уже запущен; дополнительных действий не выполнялось.',
  'Antiscan is already stopped; no lifecycle command was executed.': 'Antiscan уже остановлен; дополнительных действий не выполнялось.',
  'Antiscan must be running before reload': 'Для перечитывания конфигурации Antiscan должен быть запущен.',
  'Antiscan reload is already in progress': 'Antiscan уже перечитывает конфигурацию. Дождитесь завершения.',
  'Antiscan reload is in progress': 'Antiscan сейчас перечитывает конфигурацию. Дождитесь завершения.',
  'Antiscan init script is unavailable or not executable': 'Штатный скрипт запуска Antiscan отсутствует или не исполняется.',
  'Antiscan init script is unavailable': 'Штатный скрипт запуска Antiscan недоступен.',
  'Antiscan is not running': 'Antiscan не запущен.',
  'Antiscan is not detected on this device': 'Antiscan не обнаружен на этом устройстве.',
  'Antiscan runtime marker is absent; live blocking state is not authoritative': 'Маркер работы Antiscan отсутствует; текущее состояние блокировок нельзя считать достоверным.',
  'ipset is unavailable; live membership cannot be checked': 'Команда ipset недоступна; проверить текущее содержимое наборов невозможно.',
  'ipset binary not found': 'Исполняемый файл ipset не найден.',
  'custom whitelist mode is configured but the runtime ipset is absent': 'Включён пользовательский белый список, но его активный набор ipset отсутствует.',
  'geo whitelist mode is configured but the runtime ipset is absent': 'Включён Geo-белый список, но его активный набор ipset отсутствует.',
  'Mobile carrier address pools can legitimately rotate through many addresses in one /24; review candidate retention and subnet threshold before changing policy.': 'Мобильные операторы могут легитимно выдавать много адресов из одной /24. Перед изменением политики проверьте время хранения кандидатов и порог блокировки подсети.',
  'invalid config request': 'Некорректный запрос на изменение конфигурации.',
  'confirm must equal APPLY_CONFIG': 'Не подтверждено применение конфигурации.',
  'ascn.conf changed since it was loaded; refresh before applying': 'ascn.conf изменился после загрузки формы. Обновите данные перед применением.',
  'ascn.conf already matches the requested settings.': 'ascn.conf уже соответствует выбранным настройкам.',
  'IPSETS_DIRECTORY changed. Upstream Antiscan requires a stop/start cycle before the new storage directory is fully active.': 'Изменён IPSETS_DIRECTORY. Для полного перехода на новый каталог нужно остановить и снова запустить Antiscan.',
  'Antiscan is stopped. Settings were stored and will become active on the next start.': 'Antiscan остановлен. Настройки сохранены и вступят в силу при следующем запуске.',
  'PORTS and PORTS_FORWARDED cannot both be empty': 'Поля PORTS и PORTS_FORWARDED не могут быть пустыми одновременно.',
  'HONEYPOT_PORTS is required when honeypot is enabled': 'При включённой ловушке необходимо указать HONEYPOT_PORTS.',
  'RULES_MASK must be a dotted IPv4 mask': 'RULES_MASK должен быть маской IPv4 в точечной записи.',
  'GEOBLOCK_COUNTRIES is required when GEOBLOCK_MODE is enabled': 'При включённом GEOBLOCK_MODE необходимо указать GEOBLOCK_COUNTRIES.',
  'IPSETS_DIRECTORY contains unsupported characters': 'IPSETS_DIRECTORY содержит недопустимые символы.',
  'IPSETS_DIRECTORY is required for persistence or Geo lists': 'Для сохранения наборов или Geo-списков необходимо указать IPSETS_DIRECTORY.',
  'IPSETS_DIRECTORY is required by the selected persistence/Geo settings': 'Выбранные настройки хранения или Geo требуют IPSETS_DIRECTORY.',
  'IPSETS_DIRECTORY must be an absolute path without parent traversal': 'IPSETS_DIRECTORY должен быть абсолютным путём без переходов через ..',
  'IPSETS_DIRECTORY contains an invalid path segment': 'IPSETS_DIRECTORY содержит недопустимый сегмент пути.',
  'IPSETS_DIRECTORY must not be /opt/etc': 'Сам каталог /opt/etc использовать нельзя; вложенные каталоги внутри /opt/etc допустимы upstream Antiscan.',
  'IPSETS_DIRECTORY must be under /opt or /tmp': 'IPSETS_DIRECTORY должен находиться внутри /opt или /tmp.',
  'SAVE_IPSETS is enabled but IPSETS_DIRECTORY is empty': 'SAVE_IPSETS включён, но IPSETS_DIRECTORY не задан.',
  'IPSETS_DIRECTORY is unavailable': 'Каталог IPSETS_DIRECTORY недоступен.',
  'invalid custom-list request': 'Некорректный запрос изменения пользовательского списка.',
  'custom list action must be add, delete, clear or reload': 'Допустимы только добавление, удаление, очистка или перечитывание пользовательского списка.',
  'list must be blacklist, whitelist or exclude': 'Можно выбрать только чёрный список, белый список или исключения.',
  'reload does not accept an entry': 'Для перечитывания списка адрес указывать не нужно.',
  'clear does not accept an entry': 'Для очистки списка адрес указывать не нужно.',
  'empty active custom whitelist reload is blocked to prevent an empty-whitelist lockout': 'Перечитывание пустого активного whitelist заблокировано: пустой whitelist способен перекрыть доступ. Сначала добавьте хотя бы один адрес или отключите whitelist.',
  'Custom list is not configured; there is nothing to reload.': 'Этот пользовательский список сейчас выключен — перечитывать runtime нечего.',
  'Antiscan is stopped; reload was not executed.': 'Antiscan остановлен — перечитывание runtime не выполнялось.',
  'Entry already exists in the custom list.': 'Такая запись уже есть в исходном пользовательском списке.',
  'Entry was already absent from the custom list.': 'Этой записи уже нет в исходном пользовательском списке.',
  'Antiscan is stopped; source file was updated without runtime reload.': 'Исходный файл обновлён, но Antiscan остановлен — runtime не перечитывался.',
  'Custom list is not configured; source file was updated without runtime reload.': 'Исходный файл обновлён. Этот список сейчас выключен, поэтому runtime не перечитывался.',
  'custom list file verification failed; original file was restored': 'Проверка исходного файла не пройдена; предыдущий файл восстановлен.',
  'Entry was already absent from the runtime set.': 'Этой записи уже нет в активном наборе.',
  'Entry already exists in the upstream custom list file.': 'Запись уже есть в пользовательском списке Antiscan.',
  'Entry is stored, but Antiscan is stopped; it will become effective when the matching list mode is loaded.': 'Запись сохранена. Antiscan остановлен, поэтому она начнёт действовать после следующего запуска соответствующего режима списка.',
  'Entry is stored, but USE_CUSTOM_EXCLUDE_LIST is disabled in ascn.conf.': 'Запись сохранена, но USE_CUSTOM_EXCLUDE_LIST сейчас выключен в ascn.conf.',
  'Entry is stored, but CUSTOM_LISTS_BLOCK_MODE is not whitelist.': 'Запись сохранена, но CUSTOM_LISTS_BLOCK_MODE сейчас не работает в режиме whitelist.',
  'unban verification failed: entry is still present': 'Проверка снятия блокировки не пройдена: запись всё ещё присутствует.',
  'unban verification failed; runtime state was restored': 'Снять блокировку не удалось; прежнее состояние восстановлено.',
  'ipset verification unavailable; file/runtime were restored': 'Проверить ipset не удалось; файл и рабочее состояние восстановлены.',
  'custom list verification failed; file/runtime were restored': 'Проверка пользовательского списка не пройдена; файл и рабочее состояние восстановлены.',
  'unban target must be an IPv4 address': 'Для снятия блокировки нужен IPv4-адрес.',
  'ascn_subnets unban target must be an IPv4 /24 prefix': 'Для ascn_subnets нужно указать IPv4-подсеть /24.',
  'single-entry unban is allowed only for ascn_ips, ascn_subnets and ascn_honeypot': 'Точечное снятие блокировки разрешено только для ascn_ips, ascn_subnets и ascn_honeypot.',
  'custom list entry must be an IPv4 address or CIDR prefix': 'Запись списка должна быть IPv4-адресом или CIDR-подсетью.',
  'list must be exclude or whitelist': 'Можно выбрать только список исключений или белый список.',
  'custom list exceeds RouterForge safety limit': 'Пользовательский список превышает безопасный лимит RouterForge.',
  'custom list would exceed RouterForge safety limit': 'После добавления пользовательский список превысит безопасный лимит RouterForge.',
  'custom list is not a regular file': 'Файл пользовательского списка имеет неподдерживаемый тип.',
  'unknown Antiscan ipset': 'Неизвестный набор ipset Antiscan.',
  'invalid IPv4 address': 'Некорректный IPv4-адрес.',
  'history limit must be between 1 and 100': 'Количество событий истории должно быть от 1 до 100.',
  'invalid Antiscan history limit': 'Некорректный лимит истории Antiscan.',
  'invalid RCI token request': 'Некорректный запрос управления RCI-токеном.',
  'RCI token action must be set, check or delete': 'Допустимы только установка, проверка или удаление RCI-токена.',
  'confirm must equal SET_TOKEN': 'Не подтверждена установка RCI-токена.',
  'confirm must equal CHECK_TOKEN': 'Не подтверждена проверка RCI-токена.',
  'confirm must equal DELETE_TOKEN': 'Не подтверждено удаление RCI-токена.',
  'RCI token must not be empty': 'RCI-токен не может быть пустым.',
  'RCI token exceeds RouterForge safety limit': 'RCI-токен превышает безопасный лимит RouterForge.',
  'RCI token must contain only ASCII letters and digits': 'RCI-токен может содержать только латинские буквы и цифры.',
  'RCI token flow is not supported by this firmware': 'Эта прошивка не поддерживает token-flow Antiscan.',
  'RCI token/key pair is incomplete': 'Комплект token/key Antiscan неполный.',
  'RCI token check does not accept a token value': 'Для проверки используется уже сохранённый токен; новое значение передавать не нужно.',
  'RCI token delete does not accept a token value': 'Для удаления значение токена передавать не нужно.',
  'upstream RCI token check failed': 'Сохранённый RCI-токен не прошёл проверку Keenetic.',
  'upstream RCI token set failed; previous encrypted token state was restored': 'Установить RCI-токен не удалось; прежнее зашифрованное состояние восстановлено.',
  'RCI token set verification failed; previous encrypted token state was restored': 'Проверка сохранения RCI-токена не пройдена; прежнее зашифрованное состояние восстановлено.',
  'RCI token validation failed; previous encrypted token state was restored': 'Новый RCI-токен не прошёл проверку; прежнее зашифрованное состояние восстановлено.',
  'upstream RCI token delete failed; encrypted token files were restored': 'Удалить RCI-токен не удалось; зашифрованные token/key файлы восстановлены.',
  'RCI token delete verification failed; encrypted token files were restored': 'Проверка удаления RCI-токена не пройдена; зашифрованные token/key файлы восстановлены.',
  'RCI token/key files are already absent.': 'RCI token/key уже отсутствуют.',
  'Antiscan stopped because this firmware requires RCI authentication after token deletion.': 'После удаления токена Antiscan остановлен: эта прошивка требует RCI-аутентификацию.',
  'invalid scheduler request': 'Некорректный запрос изменения расписания Antiscan.',
  'confirm must equal APPLY_SCHEDULE': 'Не подтверждено применение расписания Antiscan.',
  'ascn_crontab.conf changed since it was loaded; refresh before applying': 'ascn_crontab.conf изменился после загрузки. Обновите расписание перед применением.',
  'ascn_crontab.conf contains lines outside the pinned upstream scheduler contract': 'В ascn_crontab.conf есть строки вне разрешённого upstream-контракта. Автоматическое применение заблокировано.',
  'at least one managed scheduler task must remain enabled': 'Хотя бы одна обычная задача Antiscan должна оставаться включённой.',
  'cron expression must contain exactly five fields': 'Cron-выражение должно состоять ровно из пяти полей.',
  'cron expression contains characters unsupported by upstream': 'Cron-выражение содержит символы, которые upstream Antiscan не принимает.',
  'retry_load_geo is managed automatically by upstream Antiscan and was preserved unchanged.': 'Автоматическая задача retry_load_geo сохранена без изменений.',
  'scheduler verification failed; previous scheduler state was restored': 'Проверка расписания не пройдена; предыдущее состояние восстановлено.',
  'invalid flush request': 'Некорректный запрос штатной очистки Antiscan.',
  'flush target must be candidates, ips, subnets, custom_whitelist, custom_blacklist, custom_exclude, geo, ndm_lockout, honeypot or all': 'Выбрана неподдерживаемая цель очистки Antiscan.',
  'flush confirmation does not match target': 'Подтверждение очистки не соответствует выбранной цели.',
  'Antiscan must be running before flush': 'Для штатной очистки Antiscan должен быть запущен.',
  'flush blocked because active custom whitelist with SAVE_IPSETS=0 could lock out access': 'Очистка активного пользовательского whitelist заблокирована: при SAVE_IPSETS=0 пустой whitelist может перекрыть доступ. Сначала отключите whitelist в настройках.',
  'flush blocked because active Geo whitelist with SAVE_IPSETS=0 could lock out access': 'Очистка активного Geo whitelist заблокирована: при SAVE_IPSETS=0 пустой whitelist может перекрыть доступ. Сначала отключите Geo whitelist в настройках.',
  'Dynamic protection sets can repopulate while Antiscan keeps running.': 'Динамические наборы могут снова наполниться, пока Antiscan продолжает работать.',
  'Keenetic lockout entries can be imported again by read_ndm_ipsets.': 'Блокировки Keenetic могут снова импортироваться задачей read_ndm_ipsets.',
  'This flush clears the upstream source file, not only the runtime ipset.': 'Эта очистка стирает штатный исходный файл списка, а не только runtime ipset.',
  'Custom exclusion entries will be erased from the source file; removed exceptions may expose addresses to blocking rules.': 'Записи пользовательских исключений будут удалены из исходного файла; адреса могут снова попасть под блокирующие правила.',
  'Geo flush removes downloaded Geo files and empties all three Geo runtime sets.': 'Geo-очистка удаляет скачанные Geo-файлы и очищает все три Geo runtime-набора.',
  'The upstream all flush does not clear custom lists or ascn_geo_exclude.': 'Штатный flush без цели не очищает пользовательские списки и ascn_geo_exclude.',
  'Geo files are removed because upstream all flush includes geo blacklist and whitelist sets.': 'Geo-файлы удаляются, потому что штатный общий flush включает Geo blacklist и whitelist.',
  'Active custom whitelist protection will be removed by upstream; repopulate the list or change mode before restart.': 'Upstream снимет активную защиту custom whitelist. До перезапуска заполните список заново или смените режим.',
  'Active Geo whitelist protection will be removed by upstream; reload Geo or change mode before restart.': 'Upstream снимет активную защиту Geo whitelist. До перезапуска загрузите Geo заново или смените режим.',
  'Selected Antiscan sets were already empty; upstream flush completed and verification passed.': 'Выбранные наборы уже были пусты; штатная очистка выполнена и проверена.',
  'upstream Antiscan flush command failed': 'Штатная команда очистки Antiscan завершилась ошибкой.',
  'Antiscan stopped during flush': 'Во время очистки Antiscan неожиданно остановился.',
  'upstream flush returned while an Antiscan reload lock is still present': 'Очистка завершилась, но Antiscan всё ещё держит lock-файл.'
};

const messageRules = [
  [/^line (\d+): expected KEY="VALUE"$/, (m) => `Строка ${m[1]}: ожидается формат KEY="VALUE".`],
  [/^line (\d+): invalid Antiscan config syntax$/, (m) => `Строка ${m[1]}: некорректный синтаксис конфигурации Antiscan.`],
  [/^line (\d+): unsupported config value$/, (m) => `Строка ${m[1]}: неподдерживаемое значение конфигурации.`],
  [/^line (\d+): unsupported Antiscan config key (.+)$/, (m) => `Строка ${m[1]}: неподдерживаемый параметр Antiscan ${m[2]}.`],
  [/^line (\d+): duplicate Antiscan config key (.+)$/, (m) => `Строка ${m[1]}: параметр ${m[2]} указан повторно.`],
  [/^(.+) must be between (\d+) and (\d+)$/, (m) => `${m[1]} должен быть в диапазоне ${m[2]}–${m[3]}.`],
  [/^(.+) ports must be between 1 and 65535$/, (m) => `Порты ${m[1]} должны быть в диапазоне 1–65535.`],
  [/^(.+) supports at most 8 country codes$/, (m) => `${m[1]} поддерживает не более 8 кодов стран.`],
  [/^(.+) contains invalid country code "(.+)"$/, (m) => `${m[1]} содержит некорректный код страны ${m[2]}.`],
  [/^(.+) does not exist$/, (m) => `Набор ${m[1]} отсутствует.`],
  [/^confirm must equal (ADD|DELETE|CLEAR|RELOAD)$/, (m) => `Для этой операции требуется подтверждение ${m[1]}.`],
  [/^Ignored (\d+) invalid custom-list lines\.$/, (m) => `Пропущено некорректных строк исходного списка: ${m[1]}.`],
  [/^runtime list state could not be verified: (.*)$/, (m) => `Не удалось проверить runtime-состояние списка: ${m[1]}`],
  [/^custom-list verification failed: (.*)$/, (m) => `Проверка пользовательского списка не пройдена: ${m[1]}`],
  [/^upstream (start|stop|reload|restart) failed: (.*)$/, (m) => `Штатная команда Antiscan ${m[1]} завершилась ошибкой: ${m[2]}`],
  [/^upstream (.+) failed: (.*)$/, (m) => `Штатная операция Antiscan ${m[1]} завершилась ошибкой: ${m[2]}`],
  [/^start verification failed: \/tmp\/ascn\.run is absent$/, () => 'Проверка запуска не пройдена: /tmp/ascn.run не появился.'],
  [/^stop verification failed: \/tmp\/ascn\.run is still present$/, () => 'Проверка остановки не пройдена: /tmp/ascn.run всё ещё существует.'],
  [/^reload verification failed: Antiscan stopped during reload$/, () => 'Проверка перечитывания не пройдена: Antiscan остановился во время операции.'],
  [/^restart verification failed: \/tmp\/ascn\.run is absent$/, () => 'Проверка перезапуска не пройдена: /tmp/ascn.run не появился.']
];

function localizeMessage(value) {
  const text = String(value ?? '').trim();
  if (!text) return '';
  if (exactMessageTranslations[text]) return exactMessageTranslations[text];
  for (const [pattern, formatter] of messageRules) {
    const match = text.match(pattern);
    if (match) return formatter(match);
  }
  if (/[А-Яа-яЁё]/.test(text)) return text;
  return `Техническая причина: ${text}`;
}

function localizeMessages(values) {
  return (values || []).map(localizeMessage).filter(Boolean);
}

const reasonLabels = {
  'custom-exclude': 'Пользовательское исключение',
  'geo-exclude': 'Geo-исключение',
  'custom-blacklist': 'Пользовательский чёрный список',
  'custom-whitelist': 'Пользовательский белый список',
  'custom-whitelist-miss': 'Нет в пользовательском белом списке',
  'geo-blacklist': 'Geo-чёрный список',
  'geo-whitelist': 'Geo-белый список',
  'geo-whitelist-miss': 'Нет в Geo-белом списке',
  'ndm-lockout': 'Блокировка Keenetic',
  honeypot: 'Ловушка',
  'distributed-subnet': 'Блокировка подсети /24',
  'direct-ip': 'Прямая блокировка IP',
  candidate: 'Кандидат /24',
  'candidate-only': 'Кандидат /24',
  'no-active-set-match': 'Совпадений нет',
  unknown: 'Причина не определена',
  'invalid-ip': 'Некорректный IPv4-адрес'
};

function reasonLabel(value) {
  return reasonLabels[value] || value || 'Причина не определена';
}

function evidenceSummary(item) {
  return {
    'custom-exclude': 'Адрес найден в пользовательских исключениях. Эти правила проверяются раньше блокирующих.',
    'geo-exclude': 'Адрес попал в Geo-исключение и не должен блокироваться последующими правилами.',
    'custom-blacklist': 'Адрес найден в активном пользовательском чёрном списке.',
    'custom-whitelist': 'Адрес присутствует в активном пользовательском белом списке.',
    'custom-whitelist-miss': 'Адрес отсутствует в активном пользовательском белом списке.',
    'geo-blacklist': 'Адрес относится к подсети из активного Geo-чёрного списка.',
    'geo-whitelist': 'Адрес относится к разрешённой подсети активного Geo-белого списка.',
    'geo-whitelist-miss': 'Адрес отсутствует в разрешённых подсетях активного Geo-белого списка.',
    'ndm-lockout': 'Адрес импортирован из политики блокировок Keenetic.',
    honeypot: 'Адрес находится в наборе блокировок ловушки Antiscan.',
    'distributed-subnet': 'Адрес входит в /24, заблокированную после накопления разных IP-кандидатов.',
    'direct-ip': 'Адрес находится в прямом наборе блокировок. Antiscan не сохраняет, какой именно механизм был точным триггером.',
    candidate: 'Адрес является кандидатом для анализа /24, но сам этот набор его не блокирует.'
  }[item?.kind] || localizeMessage(item?.summary || '');
}

function setOperationalExplanation(name) {
  return {
    ascn_candidates: 'Кандидат сам по себе не блокирует IP; несколько адресов одной /24 могут позднее привести к блокировке всей подсети.',
    ascn_ips: 'Прямая блокировка IP. Antiscan не сохраняет точную причину срабатывания внутри этого набора.',
    ascn_subnets: 'Блокируется вся /24 после накопления разных IP-кандидатов. Это особенно важно для мобильных и динамических пулов.',
    ascn_honeypot: 'IP заблокирован после обращения к одному из портов ловушки HONEYPOT_PORTS.',
    ascn_ndm_lockout: 'Запись импортирована из политики блокировок Keenetic; прямое снятие такой блокировки здесь намеренно отключено.',
    ascn_custom_exclude: 'Пользовательское исключение имеет приоритет перед блокирующими правилами.',
    ascn_custom_blacklist: 'Активный пользовательский чёрный список при CUSTOM_LISTS_BLOCK_MODE=blacklist.',
    ascn_custom_whitelist: 'В режиме whitelist отсутствие адреса в этом наборе означает блокировку.',
    ascn_geo_blacklist: 'Подсети стран из активного Geo-чёрного списка.',
    ascn_geo_whitelist: 'В режиме Geo whitelist отсутствие подсети в разрешённом наборе означает блокировку.',
    ascn_geo_exclude: 'Geo-исключение имеет приоритет перед блокирующими правилами.'
  }[name] || 'Запись активного набора Antiscan.';
}

function reasonExplanation(result) {
  const descriptions = {
    'custom-exclude': 'IP найден в пользовательских исключениях. Antiscan пропускает такой адрес до проверки блокирующих правил.',
    'geo-exclude': 'IP попал в Geo-исключение и не должен блокироваться последующими правилами.',
    'custom-blacklist': 'IP найден в активном пользовательском чёрном списке.',
    'custom-whitelist-miss': 'Включён режим белого списка, но IP отсутствует среди разрешённых адресов.',
    'geo-blacklist': 'IP относится к подсети страны из активного Geo-чёрного списка.',
    'geo-whitelist-miss': 'Включён Geo-белый список, но IP не относится к разрешённым подсетям.',
    'ndm-lockout': 'IP импортирован из политики блокировок Keenetic.',
    honeypot: 'IP находится в наборе блокировок ловушки после обращения к порту-приманке.',
    'distributed-subnet': 'Заблокирована вся /24: Antiscan накопил заданное количество разных IP-кандидатов из одной подсети.',
    'direct-ip': 'IP находится в прямом наборе блокировок. Antiscan не сохраняет, какой именно механизм был точным триггером.',
    'candidate-only': 'IP пока только кандидат для анализа /24 и этим набором сам по себе не блокируется.',
    'no-active-set-match': 'Совпадений с активными блокирующими наборами не найдено.'
  };
  return descriptions[result?.reason] || '';
}

function configNumber(id) {
  const raw = node(id).value.trim();
  return raw === '' ? 0 : Number(raw);
}

function currentConfigFormPayload() {
  return {
    isp_interfaces: node('cfgIspInterfaces').value.trim(),
    ports: node('cfgPorts').value.trim(),
    forwarded_ports: node('cfgForwardedPorts').value.trim(),
    enable_honeypot: node('cfgEnableHoneypot').checked,
    honeypot_ports: node('cfgHoneypotPorts').value.trim(),
    honeypot_bantime_seconds: configNumber('cfgHoneypotBanTime'),
    enable_ips_ban: node('cfgEnableIpsBan').checked,
    rules_mask: node('cfgRulesMask').value.trim(),
    recent_connections_time_seconds: configNumber('cfgRecentTime'),
    recent_connections_hitcount: configNumber('cfgRecentHitcount'),
    recent_connections_limit: configNumber('cfgRecentLimit'),
    recent_connections_bantime_seconds: configNumber('cfgRecentBanTime'),
    different_ip_candidates_storage_seconds: configNumber('cfgCandidateStorage'),
    different_ip_threshold: configNumber('cfgDifferentThreshold'),
    subnets_bantime_seconds: configNumber('cfgSubnetBanTime'),
    ipsets_directory: node('cfgIPSetsDirectory').value.trim(),
    save_ipsets: node('cfgSaveIPSets').checked,
    save_on_exit: node('cfgSaveOnExit').checked,
    use_custom_exclude_list: node('cfgUseCustomExclude').checked,
    custom_lists_block_mode: node('cfgCustomMode').value,
    geoblock_mode: node('cfgGeoMode').value,
    geoblock_countries: node('cfgGeoCountries').value.trim(),
    geo_exclude_countries: node('cfgGeoExcludeCountries').value.trim(),
    read_ndm_lockout_ipsets: node('cfgReadNDM').checked,
    lockout_ipset_bantime_seconds: configNumber('cfgLockoutBanTime')
  };
}

function configPayloadFromSnapshot(cfg) {
  return {
    isp_interfaces: (cfg.isp_interfaces || []).join(' '),
    ports: (cfg.ports || []).join(','),
    forwarded_ports: (cfg.forwarded_ports || []).join(','),
    enable_honeypot: Boolean(cfg.enable_honeypot),
    honeypot_ports: (cfg.honeypot_ports || []).join(','),
    honeypot_bantime_seconds: Number(cfg.honeypot_bantime_seconds || 0),
    enable_ips_ban: Boolean(cfg.enable_ips_ban),
    rules_mask: cfg.rules_mask || '255.255.255.255',
    recent_connections_time_seconds: Number(cfg.recent_connections_time_seconds || 0),
    recent_connections_hitcount: Number(cfg.recent_connections_hitcount || 0),
    recent_connections_limit: Number(cfg.recent_connections_limit || 0),
    recent_connections_bantime_seconds: Number(cfg.recent_connections_bantime_seconds || 0),
    different_ip_candidates_storage_seconds: Number(cfg.different_ip_candidates_storage_seconds || 0),
    different_ip_threshold: Number(cfg.different_ip_threshold || 0),
    subnets_bantime_seconds: Number(cfg.subnets_bantime_seconds || 0),
    ipsets_directory: cfg.ipsets_directory || '',
    save_ipsets: Boolean(cfg.save_ipsets),
    save_on_exit: Boolean(cfg.save_on_exit),
    use_custom_exclude_list: Boolean(cfg.use_custom_exclude_list),
    custom_lists_block_mode: cfg.custom_lists_block_mode || '0',
    geoblock_mode: cfg.geoblock_mode || '0',
    geoblock_countries: (cfg.geoblock_countries || []).join(' '),
    geo_exclude_countries: (cfg.geo_exclude_countries || []).join(' '),
    read_ndm_lockout_ipsets: Boolean(cfg.read_ndm_lockout_ipsets),
    lockout_ipset_bantime_seconds: Number(cfg.lockout_ipset_bantime_seconds || 0)
  };
}

function setConfigFormPayload(cfg) {
  node('cfgIspInterfaces').value = cfg.isp_interfaces;
  node('cfgPorts').value = cfg.ports;
  node('cfgForwardedPorts').value = cfg.forwarded_ports;
  node('cfgEnableHoneypot').checked = cfg.enable_honeypot;
  node('cfgHoneypotPorts').value = cfg.honeypot_ports;
  node('cfgHoneypotBanTime').value = cfg.honeypot_bantime_seconds;
  node('cfgEnableIpsBan').checked = cfg.enable_ips_ban;
  node('cfgRulesMask').value = cfg.rules_mask;
  node('cfgRecentTime').value = cfg.recent_connections_time_seconds;
  node('cfgRecentHitcount').value = cfg.recent_connections_hitcount;
  node('cfgRecentLimit').value = cfg.recent_connections_limit;
  node('cfgRecentBanTime').value = cfg.recent_connections_bantime_seconds;
  node('cfgCandidateStorage').value = cfg.different_ip_candidates_storage_seconds;
  node('cfgDifferentThreshold').value = cfg.different_ip_threshold;
  node('cfgSubnetBanTime').value = cfg.subnets_bantime_seconds;
  node('cfgIPSetsDirectory').value = cfg.ipsets_directory;
  node('cfgSaveIPSets').checked = cfg.save_ipsets;
  node('cfgSaveOnExit').checked = cfg.save_on_exit;
  node('cfgUseCustomExclude').checked = cfg.use_custom_exclude_list;
  node('cfgCustomMode').value = cfg.custom_lists_block_mode;
  node('cfgGeoMode').value = cfg.geoblock_mode;
  node('cfgGeoCountries').value = cfg.geoblock_countries;
  node('cfgGeoExcludeCountries').value = cfg.geo_exclude_countries;
  node('cfgReadNDM').checked = cfg.read_ndm_lockout_ipsets;
  node('cfgLockoutBanTime').value = cfg.lockout_ipset_bantime_seconds;
}

function populateConfigEditor(force = false) {
  const form = node('configForm');
  if (!form) return;
  if (!snapshot?.detected || !snapshot?.config) {
    node('configState').className = 'state neutral';
    node('configState').textContent = 'НЕДОСТУПНО';
    node('configHint').textContent = 'Antiscan или ascn.conf не обнаружен.';
    node('applyConfig').disabled = true;
    node('resetConfig').disabled = true;
    return;
  }

  if (!configDirty || force) {
    const payload = configPayloadFromSnapshot(snapshot.config);
    setConfigFormPayload(payload);
    configBaseline = JSON.stringify(payload);
    configBaseSHA = snapshot.config_sha256 || '';
    configDirty = false;
  }
  updateConfigEditorState();
}

function updateConfigEditorState() {
  if (!node('configForm')) return;
  const current = currentConfigFormPayload();
  configDirty = Boolean(configBaseline) && JSON.stringify(current) !== configBaseline;
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const stale = Boolean(configDirty && configBaseSHA && snapshot?.config_sha256 && snapshot.config_sha256 !== configBaseSHA);

  const state = node('configState');
  const hint = node('configHint');
  state.className = 'state';
  if (configBusy) {
    state.classList.add('info');
    state.textContent = 'ПРИМЕНЕНИЕ…';
    hint.textContent = 'Создаём резервную копию → атомарно записываем → перечитываем Antiscan → проверяем результат. При ошибке выполняется откат.';
  } else if (stale) {
    state.classList.add('bad');
    state.textContent = 'УСТАРЕЛО';
    hint.textContent = 'ascn.conf изменился после открытия формы. Сбросьте форму к свежим данным перед применением.';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
    hint.textContent = 'Antiscan сейчас перечитывает конфигурацию или Geo-данные. Применение временно заблокировано.';
  } else if (configDirty) {
    state.classList.add('warn');
    state.textContent = 'ИЗМЕНЕНО';
    hint.textContent = snapshot?.running
      ? 'Изменения будут безопасно применены через штатное перечитывание S99ascn.'
      : 'Antiscan остановлен: файл будет сохранён и проверен, а настройки вступят в силу при следующем запуске.';
  } else {
    state.classList.add('good');
    state.textContent = 'СИНХРОНИЗИРОВАНО';
    hint.textContent = snapshot?.running ? 'Форма соответствует активному ascn.conf.' : 'Форма соответствует сохранённому ascn.conf.';
  }

  node('configHash').textContent = configBaseSHA ? `SHA256 ${configBaseSHA.slice(0, 16)}…` : 'SHA256 —';
  node('applyConfig').disabled = configBusy || !configDirty || !configBaseSHA || upstreamBusy || stale;
  node('resetConfig').disabled = configBusy || !configDirty;

  const mobileRisk = current.enable_ips_ban && current.different_ip_threshold > 0 &&
    current.different_ip_threshold <= 5 && current.different_ip_candidates_storage_seconds >= 86400;
  const risk = node('configMobileRisk');
  risk.className = `config-risk ${mobileRisk ? 'warn' : 'good'}`;
  risk.textContent = mobileRisk
    ? `⚠ /24: порог ${current.different_ip_threshold}, кандидаты хранятся ${fmtDuration(current.different_ip_candidates_storage_seconds)} — для мобильных пулов настройка может быть слишком агрессивной.`
    : '✓ Сочетание порога /24 и времени хранения кандидатов не попадает под встроенный профиль повышенного риска.';

  renderLifecycleControls();
}

async function applyConfigEditor() {
  if (!configDirty || !configBaseSHA) return;
  const current = currentConfigFormPayload();
  const warning = snapshot?.running
    ? 'Будет создана резервная копия, ascn.conf запишется атомарно, затем Antiscan штатно перечитает настройки. При ошибке RouterForge восстановит предыдущий файл и рабочее состояние.'
    : 'Antiscan остановлен. Будет создана резервная копия и атомарно сохранён ascn.conf; настройки вступят в силу при следующем запуске.';
  if (!window.confirm(`Применить изменения ascn.conf?\n\n${warning}`)) return;

  configBusy = true;
  updateConfigEditorState();
  try {
    const result = await mutate('config', { ...current, base_sha256: configBaseSHA, confirm: 'APPLY_CONFIG' });
    showMutationResult(result, false, 'config');
    configDirty = false;
    configBaseline = '';
    await loadStatus();
    populateConfigEditor(true);
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'config');
    configDirty = false;
    configBaseline = '';
    configBaseSHA = '';
    await loadStatus();
    populateConfigEditor(true);
  } finally {
    configBusy = false;
    updateConfigEditorState();
  }
}

function diagnosticStateView(state) {
  return {
    pass: ['good', 'ГОТОВО'],
    warn: ['warn', 'ВНИМАНИЕ'],
    fail: ['bad', 'ОШИБКА'],
    info: ['info', 'ИНФО']
  }[state] || ['neutral', 'НЕИЗВЕСТНО'];
}

function renderDiagnostics(payload) {
  const overall = diagnosticStateView(payload?.overall || 'info');
  const state = node('diagnosticsState');
  state.className = `state ${overall[0]}`;
  state.textContent = overall[1];

  const generated = payload?.generated_at ? fmtAuditTime(payload.generated_at) : '—';
  node('diagnosticsMeta').textContent = `Проверено: ${generated} · Antiscan ${payload?.installed_version || 'не определён'} · ${payload?.running ? 'работает' : 'остановлен'}`;

  const contract = payload?.contract || {};
  const rows = [
    ['Upstream', contract.repository || '—'],
    ['Зафиксированная версия', contract.version || '—'],
    ['Upstream SHA', contract.pinned_sha || '—'],
    ['Параметры ascn.conf', `${(contract.config_keys || []).length} / 25`],
    ['Известные ipset', `${(contract.ipsets || []).length} / 11`],
    ['CLI-команды', String((contract.commands || []).length)],
    ['Разрешённые cron-задачи', (contract.valid_tasks || []).join(' · ') || '—']
  ];
  node('diagnosticsContract').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');

  const checks = payload?.checks || [];
  const root = node('diagnosticsChecks');
  if (!checks.length) {
    root.className = 'diagnostics-list empty-state';
    root.textContent = 'Проверки не вернули данных.';
    return;
  }

  root.className = 'diagnostics-list';
  root.innerHTML = checks.map((check) => {
    const view = diagnosticStateView(check.state);
    const details = (check.details || []).map((detail) => `<small class="mono">${escapeHTML(detail)}</small>`).join('');
    return `
      <article class="diagnostic-row">
        <div class="diagnostic-main">
          <div class="diagnostic-title"><strong>${escapeHTML(check.label || check.id || 'Проверка')}</strong><span class="state ${view[0]}">${view[1]}</span></div>
          <p>${escapeHTML(check.summary || '—')}</p>
          ${details}
        </div>
      </article>`;
  }).join('');
}

async function loadDiagnostics() {
  const button = node('reloadDiagnostics');
  if (button) button.disabled = true;
  node('diagnosticsState').className = 'state info';
  node('diagnosticsState').textContent = 'ПРОВЕРКА…';
  try {
    const payload = await api('diagnostics');
    diagnosticsLoaded = true;
    renderDiagnostics(payload);
  } catch (error) {
    diagnosticsLoaded = true;
    node('diagnosticsState').className = 'state bad';
    node('diagnosticsState').textContent = 'ОШИБКА';
    node('diagnosticsMeta').textContent = localizeMessage(error.message || 'Не удалось выполнить диагностику Antiscan.');
    node('diagnosticsChecks').className = 'diagnostics-list empty-state bad-text';
    node('diagnosticsChecks').textContent = 'Диагностика недоступна.';
  } finally {
    if (button) button.disabled = false;
  }
}

function renderCompatibility(payload) {
  compatibilityLoaded = true;
  const state = node('compatibilityState');
  const meta = node('compatibilityMeta');
  const grid = node('compatibilityGrid');
  const warnings = node('compatibilityWarnings');
  if (!state || !meta || !grid || !warnings) return;

  const view = diagnosticStateView(payload?.state || 'info');
  state.className = `state ${view[0]}`;
  state.textContent = view[1];
  meta.textContent = payload?.summary || 'Состояние совместимости не определено.';

  const update = payload?.update || {};
  const firmware = payload?.firmware || {};
  const hook = payload?.hook || {};
  const packageVersion = payload?.package_version || 'не найден';
  const packageMatch = payload?.package_version
    ? (payload?.package_runtime_match ? 'совпадает' : 'расходится')
    : 'не проверено';
  const updateValue = update.available
    ? `${update.available_version || '—'} · ${update.available_channel || 'upstream'}`
    : (update.eligibility_known ? 'по текущему upstream cache нет' : 'не определено');
  const cacheValue = !update.cache_present
    ? 'нет'
    : `${update.cache_fresh ? 'свежий' : 'устарел'} · ${Number(update.cache_age_seconds || 0)} сек.`;
  const netfilterValue = !firmware.netfilter_known
    ? 'не определено'
    : (firmware.netfilter_present ? 'есть' : 'не найден');
  const hookValue = hook.present
    ? (hook.executable ? 'установлен · исполняем' : 'установлен · не исполняем')
    : 'не найден';

  const rows = [
    ['Runtime S99ascn', payload?.script_version || '—'],
    ['OPKG Antiscan', packageVersion],
    ['Пакет / runtime', packageMatch],
    ['Проверенный RouterForge', payload?.pinned_version || '—'],
    ['Pinned contract', payload?.contract_exact ? 'точное совпадение' : 'не совпадает'],
    ['Доступное обновление', updateValue],
    ['Upstream update cache', cacheValue],
    ['Прошивка Keenetic', firmware.version || 'не определена'],
    ['Netfilter', netfilterValue],
    ['Netfilter hook', hookValue],
    ['Кто обновляет Antiscan', payload?.update_owner === 'upstream-opkg' ? 'официальный upstream OPKG' : (payload?.update_owner || '—')]
  ];
  grid.innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');

  const messages = payload?.warnings || [];
  warnings.innerHTML = messages.length
    ? messages.map((item) => `<div class="warning-item"><span>⚠</span><p>${escapeHTML(String(item))}</p></div>`).join('')
    : '<div class="ok-note">Версия, package metadata, Netfilter и hook согласованы с проверенным контрактом RouterForge.</div>';
}

async function loadCompatibility() {
  const button = node('reloadCompatibility');
  if (button) button.disabled = true;
  node('compatibilityState').className = 'state info';
  node('compatibilityState').textContent = 'ПРОВЕРКА…';
  try {
    const payload = await api('compatibility');
    renderCompatibility(payload);
  } catch (error) {
    compatibilityLoaded = true;
    node('compatibilityState').className = 'state bad';
    node('compatibilityState').textContent = 'ОШИБКА';
    node('compatibilityMeta').textContent = localizeMessage(error.message || 'Не удалось проверить совместимость Antiscan.');
    node('compatibilityGrid').innerHTML = '';
    node('compatibilityWarnings').innerHTML = '';
  } finally {
    if (button) button.disabled = false;
  }
}

function rciAuthLabel(state) {
  return {
    required: 'требуется',
    not_required: 'не требуется',
    unknown: 'ещё не определено'
  }[state] || 'неизвестно';
}

function renderRCITokenStatus(payload) {
  rciTokenLoaded = true;
  const state = node('rciTokenState');
  const summary = node('rciTokenSummary');
  const meta = node('rciTokenMeta');
  const setButton = node('setRCIToken');
  const checkButton = node('checkRCIToken');
  const deleteButton = node('deleteRCIToken');
  if (!state || !summary || !meta) return;

  const complete = Boolean(payload?.complete);
  const supported = Boolean(payload?.supported);
  const authState = payload?.auth_state || 'unknown';

  state.className = 'state';
  if (!supported) {
    state.classList.add('neutral');
    state.textContent = 'НЕ ПОДДЕРЖИВАЕТСЯ';
    summary.textContent = 'Upstream отключил token-flow для этой версии прошивки.';
  } else if (authState === 'required' && !complete) {
    state.classList.add('bad');
    state.textContent = 'НУЖЕН ТОКЕН';
    summary.textContent = 'RCI требует токен, но комплект token/key Antiscan неполный.';
  } else if (complete) {
    state.classList.add('good');
    state.textContent = 'НАСТРОЕН';
    summary.textContent = authState === 'required'
      ? 'RCI требует токен; зашифрованный token/key комплект Antiscan присутствует.'
      : 'Зашифрованный token/key комплект Antiscan присутствует.';
  } else if (authState === 'not_required') {
    state.classList.add('info');
    state.textContent = 'НЕ ТРЕБУЕТСЯ';
    summary.textContent = 'Upstream cache сообщает, что этой прошивке RCI-токен сейчас не требуется.';
  } else {
    state.classList.add('warn');
    state.textContent = 'НЕ ОПРЕДЕЛЕНО';
    summary.textContent = 'Требование RCI ещё не закэшировано upstream.';
  }

  meta.textContent = `RCI: ${rciAuthLabel(authState)} · token ${payload?.token_present ? 'есть' : 'нет'} · key ${payload?.key_present ? 'есть' : 'нет'} · Antiscan ${payload?.running ? 'работает' : 'остановлен'}`;
  setButton.disabled = rciTokenBusy || !supported;
  checkButton.disabled = rciTokenBusy || !supported || !complete;
  deleteButton.disabled = rciTokenBusy || (!payload?.token_present && !payload?.key_present);
}

async function loadRCITokenStatus() {
  const button = node('reloadRCIToken');
  if (button) button.disabled = true;
  try {
    const payload = await api('rci-token');
    renderRCITokenStatus(payload);
  } catch (error) {
    rciTokenLoaded = true;
    node('rciTokenState').className = 'state bad';
    node('rciTokenState').textContent = 'ОШИБКА';
    node('rciTokenSummary').textContent = localizeMessage(error.message || 'Не удалось прочитать состояние RCI-токена.');
  } finally {
    if (button) button.disabled = false;
  }
}

async function performRCITokenAction(action) {
  const input = node('rciTokenInput');
  let token = '';
  let confirmValue = '';
  let prompt = '';

  if (action === 'set') {
    token = input.value.trim();
    if (!token) return;
    if (!/^[A-Za-z0-9]+$/.test(token)) {
      showMutationResult({ error: 'RCI token must contain only ASCII letters and digits' }, true, 'rci-token:set');
      return;
    }
    confirmValue = 'SET_TOKEN';
    prompt = 'Сохранить новый RCI-токен?\n\nТокен будет передан штатной команде Antiscan через stdin, проверен upstream и не попадёт в argv или журнал.';
  } else if (action === 'check') {
    confirmValue = 'CHECK_TOKEN';
    prompt = 'Проверить сохранённый RCI-токен через штатную команду Antiscan?';
  } else if (action === 'delete') {
    confirmValue = 'DELETE_TOKEN';
    prompt = 'Удалить RCI-токен Antiscan?\n\nЕсли прошивка требует RCI-аутентификацию, upstream может штатно остановить Antiscan после удаления.';
  } else {
    return;
  }

  if (!window.confirm(prompt)) return;

  const body = { action, confirm: confirmValue };
  if (action === 'set') body.token = token;
  if (input) input.value = '';
  token = '';

  rciTokenBusy = true;
  try {
    const result = await mutate('rci-token-action', body);
    showMutationResult(result, false, `rci-token:${action}`);
    await loadRCITokenStatus();
    await loadStatus();
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `rci-token:${action}`);
    await loadRCITokenStatus();
    await loadStatus();
  } finally {
    rciTokenBusy = false;
    if (rciTokenLoaded) {
      try {
        const payload = await api('rci-token');
        renderRCITokenStatus(payload);
      } catch (_) {
        // Previous error state is already visible.
      }
    }
  }
}
const schedulerTaskLabels = {
  read_candidates: ['Обработка кандидатов', 'Проверка накопленных IP-кандидатов и перенос /24 при достижении порога.'],
  read_ndm_ipsets: ['Импорт блокировок Keenetic', 'Перенос системных lockout IP из Keenetic в runtime Antiscan.'],
  save_ipsets: ['Сохранение ipset', 'Периодический экспорт runtime-наборов согласно SAVE_IPSETS.'],
  'update_ipsets geo': ['Обновление Geo', 'Плановое обновление списков подсетей настроенных стран.'],
  retry_load_geo: ['Повтор Geo после ошибки', 'Временная аварийная задача, которую создаёт и удаляет сам upstream Antiscan.']
};

function schedulerManagedPayload() {
  return schedulerRows.filter(item => !item.automatic).map(item=>({
    task:item.task,enabled:Boolean(item.enabled),schedule:String(item.schedule||'').trim().replace(/\s+/g,' ')
  }));
}

function updateSchedulerState() {
  const apply = node('applyScheduler');
  const reset = node('resetScheduler');
  const state = node('schedulerState');
  if (!schedulerSnapshot || !apply || !reset || !state) return;

  const current = JSON.stringify(schedulerManagedPayload());
  schedulerDirty = Boolean(schedulerBaseline) && current !== schedulerBaseline;
  if (!schedulerSnapshot.valid) {
    state.className = 'state bad';
    state.textContent = 'ОШИБКА ФАЙЛА';
  } else if (schedulerBusy) {
    state.className = 'state info';
    state.textContent = 'ПРИМЕНЕНИЕ…';
  } else if (schedulerDirty) {
    state.className = 'state warn';
    state.textContent = 'ИЗМЕНЕНО';
  } else if (!schedulerSnapshot.synced) {
    state.className = 'state warn';
    state.textContent = 'НЕ СИНХРОНИЗИРОВАНО';
  } else {
    state.className = 'state good';
    state.textContent = 'СИНХРОНИЗИРОВАНО';
  }

  apply.disabled = schedulerBusy || !schedulerSnapshot.valid || (!schedulerDirty && schedulerSnapshot.synced);
  reset.disabled = schedulerBusy || !schedulerDirty;
}

function renderScheduler(payload, force = false) {
  schedulerLoaded = true;
  schedulerSnapshot = payload;
  const root = node('schedulerTaskRows');
  const meta = node('schedulerMeta');
  if (!root || !meta) return;

  const errors = payload?.errors || [];
  meta.textContent = `ascn_crontab.conf: ${payload?.task_count ?? 0} задач · active crontab: ${payload?.active_count ?? 0} · ${payload?.synced ? 'синхронизировано' : 'есть расхождение'} · SHA256 ${(payload?.source_sha256 || '—').slice(0, 16)}…`;
  if (errors.length) {
    root.className = 'scheduler-list';
    schedulerRows = [];
    root.innerHTML = `<div class="notice bad">${errors.map((item) => escapeHTML(localizeMessage(item))).join(' · ')}</div>`;
    schedulerBaseline = '';
    schedulerDirty = false;
    updateSchedulerState();
    return;
  }

  root.className = 'scheduler-list';
  root.innerHTML = undefined;
  schedulerRows = (payload?.tasks || []).map(item=>({...item,enabled:Boolean(item.enabled),automatic:Boolean(item.automatic)}));


  if (force || !schedulerBaseline) {
    schedulerBaseline = JSON.stringify(schedulerManagedPayload());
    schedulerDirty = false;
  }
  updateSchedulerState();
}

async function loadScheduler() {
  const button = node('reloadScheduler');
  if (button) button.disabled = true;
  try {
    const payload = await api('schedule');
    schedulerBaseline = '';
    renderScheduler(payload, true);
  } catch (error) {
    schedulerLoaded = true;
    schedulerSnapshot = null;
    node('schedulerState').className = 'state bad';
    node('schedulerState').textContent = 'ОШИБКА';
    node('schedulerMeta').textContent = localizeMessage(error.message || 'Не удалось прочитать расписание Antiscan.');
    node('schedulerTaskRows').className = 'scheduler-list empty-state bad-text';
    node('schedulerTaskRows').textContent = 'Расписание недоступно.';
    node('applyScheduler').disabled = true;
    node('resetScheduler').disabled = true;
  } finally {
    if (button) button.disabled = false;
  }
}

async function applyScheduler() {
  if (!schedulerSnapshot?.source_sha256 || !schedulerSnapshot?.valid) return;
  const tasks = schedulerManagedPayload();
  if (!window.confirm('Применить расписание Antiscan?\n\nRouterForge атомарно обновит ascn_crontab.conf, вызовет штатный update_crontab и проверит активный crontab. При ошибке будет восстановлено предыдущее состояние.')) return;

  schedulerBusy = true;
  updateSchedulerState();
  try {
    const result = await mutate('schedule-action', {
      tasks,
      base_sha256: schedulerSnapshot.source_sha256,
      confirm: 'APPLY_SCHEDULE'
    });
    showMutationResult(result, false, 'schedule:apply');
    await loadScheduler();
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'schedule:apply');
    await loadScheduler();
  } finally {
    schedulerBusy = false;
    if (schedulerSnapshot) updateSchedulerState();
  }
}

const flushTargetLabels = {
  candidates: 'Кандидаты',
  ips: 'Прямые IP',
  subnets: 'Подсети /24',
  custom_whitelist: 'Пользовательский белый список',
  custom_blacklist: 'Пользовательский чёрный список',
  custom_exclude: 'Пользовательские исключения',
  geo: 'Все Geo-наборы',
  ndm_lockout: 'Блокировки Keenetic',
  honeypot: 'Ловушка',
  all: 'Upstream «все»'
};

function flushTargetLabel(target) {
  return flushTargetLabels[target] || target || 'Наборы Antiscan';
}

function renderFlushPreview(payload) {
  flushPreviewLoaded = true;
  flushPreview = payload;
  const state = node('flushState');
  const meta = node('flushMeta');
  const warnings = node('flushWarnings');
  const button = node('flushSelected');
  if (!state || !meta || !warnings || !button) return;

  state.className = `state ${payload?.allowed ? 'warn' : 'bad'}`;
  state.textContent = payload?.allowed ? 'ГОТОВО К ОЧИСТКЕ' : 'ЗАБЛОКИРОВАНО';
  const sets = (payload?.affected_sets || []).map((item) => {
    const count = item.count_known ? Number(item.count || 0) : '—';
    return `${item.name}: ${count}`;
  });
  meta.textContent = `${flushTargetLabel(payload?.target)} · записей сейчас ${Number(payload?.before_entries || 0)} · ${sets.join(' · ') || 'наборы не найдены'}`;

  const messages = [...localizeMessages(payload?.warnings || [])];
  if (payload?.destructive_file) messages.unshift('Операция затрагивает сохранённые файлы upstream, а не только runtime.');
  if (payload?.restart_required) messages.push('После этой очистки upstream требует восстановить данные и перезапустить Antiscan перед возвратом соответствующей защиты.');
  if (payload?.block_reason) messages.unshift(localizeMessage(payload.block_reason));
  warnings.innerHTML = messages.length
    ? messages.map((item) => `<div class="warning-item"><span>⚠</span><p>${escapeHTML(item)}</p></div>`).join('')
    : '<div class="empty-state">Дополнительных предупреждений для этой цели нет.</div>';
  button.disabled = flushBusy || !payload?.allowed;
}

async function loadFlushPreview() {
  const select = node('flushTarget');
  const reload = node('reloadFlushPreview');
  if (!select || !reload) return;
  reload.disabled = true;
  try {
    const payload = await api(`flush-preview?target=${encodeURIComponent(select.value)}`);
    renderFlushPreview(payload);
  } catch (error) {
    flushPreviewLoaded = true;
    flushPreview = null;
    node('flushState').className = 'state bad';
    node('flushState').textContent = 'ОШИБКА';
    node('flushMeta').textContent = localizeMessage(error.message || 'Не удалось построить preview очистки.');
    node('flushWarnings').innerHTML = '';
    node('flushSelected').disabled = true;
  } finally {
    reload.disabled = false;
  }
}

async function performFlush() {
  const target = node('flushTarget')?.value || '';
  if (!flushPreview || flushPreview.target !== target || !flushPreview.allowed) return;
  const warnings = localizeMessages(flushPreview.warnings || []);
  const detail = [
    `Цель: ${flushTargetLabel(target)}`,
    `Записей сейчас: ${Number(flushPreview.before_entries || 0)}`,
    flushPreview.destructive_file ? 'Будут изменены/удалены сохранённые файлы upstream.' : '',
    ...warnings
  ].filter(Boolean).join('\n');
  if (!window.confirm(`Выполнить штатный Antiscan flush?\n\n${detail}\n\nОперация необратимо очищает выбранные данные.`)) return;

  flushBusy = true;
  node('flushSelected').disabled = true;
  try {
    const result = await mutate('flush', {
      target,
      confirm: flushPreview.confirm
    });
    showMutationResult(result, false, `flush:${target}`);
    await loadFlushPreview();
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (browserState.customSource) await loadCustomList();
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `flush:${target}`);
    await loadFlushPreview();
  } finally {
    flushBusy = false;
    if (flushPreview) node('flushSelected').disabled = !flushPreview.allowed;
  }
}

function activateTab(name) {
  activeTab=name;
  document.documentElement.dataset.section=name;
  for(const id of tabControlIds){
    const item=node(id);
    item.classList.toggle('active',item.dataset.tab===name);
  }
  for(const id of pageControlIds){
    const item=node(id),pageName=item.dataset.page;
    const grouped=(name==='overview'&&pageName==='protection')||
                  (name==='blocked'&&pageName==='inspect')||
                  (name==='geo'&&pageName==='lists');
    item.classList.toggle('active',pageName===name||grouped);
  }
}

function renderState() {
  const state = node('targetState');
  state.className = 'state';
  if (!snapshot?.detected) {
    state.classList.add('neutral');
    state.textContent = 'НЕ ОБНАРУЖЕН';
  } else if (snapshot.running) {
    state.classList.add('good');
    state.textContent = 'РАБОТАЕТ';
  } else {
    state.classList.add('warn');
    state.textContent = 'ОСТАНОВЛЕН';
  }

  node('version').textContent = snapshot?.version || '—';
  node('blockedIps').textContent = countFor('ascn_ips');
  node('blockedSubnets').textContent = countFor('ascn_subnets');
  node('candidateCount').textContent = countFor('ascn_candidates');
  node('ipsetBinary').textContent = `ipset: ${snapshot?.ipset_binary || 'не найден'}`;

  const notice = node('notice');
  if (!snapshot?.detected) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan не обнаружен. RouterForge не устанавливает пакет Antiscan автоматически.';
  } else if (!snapshot.running) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan обнаружен, но /tmp/ascn.run отсутствует. Текущее состояние блокировок нельзя считать достоверным.';
  } else if ((snapshot.errors || []).length) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = localizeMessages(snapshot.errors).join(' · ');
  } else {
    notice.hidden = true;
  }

  renderLifecycleControls();
}

function renderLifecycleControls() {
  const start = node('startAntiscan');
  const stop = node('stopAntiscan');
  const reload = node('reloadAntiscan');
  const restart = node('restartAntiscan');
  const state = node('lifecycleState');
  const hint = node('lifecycleHint');
  if (!start || !stop || !reload || !restart || !state || !hint) return;

  const detected = Boolean(snapshot?.detected);
  const running = Boolean(snapshot?.running);
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const busy = lifecycleBusy || operationBusy || configBusy || configDirty;

  start.disabled = busy || !detected || running || upstreamBusy;
  stop.disabled = busy || !detected || !running || upstreamBusy;
  reload.disabled = busy || !detected || !running || upstreamBusy;
  restart.disabled = busy || !detected || upstreamBusy;

  state.className = 'state';
  if (lifecycleBusy) {
    state.classList.add('info');
    state.textContent = 'ОПЕРАЦИЯ…';
    hint.textContent = 'Ждём завершения штатной команды Antiscan и проверяем итоговое состояние.';
  } else if (operationBusy) {
    state.classList.add('info');
    state.textContent = 'ОБСЛУЖИВАНИЕ…';
    hint.textContent = 'Выполняется штатная сервисная команда Antiscan.';
  } else if (!detected) {
    state.classList.add('neutral');
    state.textContent = 'НЕДОСТУПНО';
    hint.textContent = 'Antiscan не обнаружен — команды управления недоступны.';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
    hint.textContent = 'Antiscan уже перечитывает конфигурацию или Geo-данные. Новое действие временно заблокировано.';
  } else if (configDirty) {
    state.classList.add('warn');
    state.textContent = 'НЕ СОХРАНЕНО';
    hint.textContent = 'Сначала примените или сбросьте изменения ascn.conf. Штатные lifecycle-команды всегда работают с сохранённым конфигом.';
  } else if (running) {
    state.classList.add('good');
    state.textContent = 'РАБОТАЕТ';
    hint.textContent = 'Можно перечитать ascn.conf, штатно перезапустить или остановить Antiscan.';
  } else {
    state.classList.add('warn');
    state.textContent = 'ОСТАНОВЛЕН';
    hint.textContent = 'Можно штатно запустить Antiscan; restart в этом состоянии также выполнит запуск upstream-командой.';
  }

  renderOperationControls();
}

function renderOperationControls() {
  const state = node('operationState');
  const buttons = operationControlIds.map(id => node(id));
  if (!state || !buttons.length) return;

  const detected = Boolean(snapshot?.detected);
  const running = Boolean(snapshot?.running);
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const busy = lifecycleBusy || operationBusy || configBusy || configDirty;

  buttons.forEach((button) => {
    const requiresRunning = button.dataset.operation !== 'update_crontab';
    button.disabled = busy || !detected || upstreamBusy || (requiresRunning && !running);
  });

  state.className = 'state';
  if (operationBusy) {
    state.classList.add('info');
    state.textContent = 'ВЫПОЛНЕНИЕ…';
  } else if (!detected) {
    state.classList.add('neutral');
    state.textContent = 'НЕДОСТУПНО';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
  } else if (configDirty) {
    state.classList.add('warn');
    state.textContent = 'НЕ СОХРАНЕНО';
  } else if (!running) {
    state.classList.add('warn');
    state.textContent = 'ТОЛЬКО CRON';
  } else {
    state.classList.add('good');
    state.textContent = 'ГОТОВО';
  }
}

function renderRuntimeSummary() {
  const cfg = snapshot?.config || {};
  const rows = [
    ['Antiscan', snapshot?.detected ? 'Обнаружен' : 'Не обнаружен'],
    ['Маркер работы', snapshot?.running ? 'Есть · /tmp/ascn.run' : 'Нет'],
    ['Перечитывание конфигурации', snapshot?.config_reload_in_progress ? 'Выполняется' : 'Нет'],
    ['Обновление Geo', snapshot?.geo_reload_in_progress ? 'Выполняется' : 'Нет'],
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Защищаемые порты', (cfg.ports || []).join(', ') || '—']
  ];
  node('runtimeSummary').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');
}

function renderRiskSummary() {
  const p = snapshot?.protection || {};
  const risk = p.ips_ban_enabled && Number(p.different_ip_threshold || 0) > 0 &&
    Number(p.different_ip_threshold || 0) <= 5 && Number(p.candidate_storage_seconds || 0) >= 86400;
  const root = node('riskSummary');
  if (!snapshot?.detected) {
    root.innerHTML = '<div class="risk-empty">Antiscan не обнаружен — анализировать нечего.</div>';
    return;
  }
  root.innerHTML = `
    <div class="risk-verdict ${risk ? 'warn' : 'good'}">
      <span>${risk ? '!' : '✓'}</span>
      <div>
        <strong>${risk ? 'Есть риск ложной /24-блокировки' : 'Явного риска ложной /24-блокировки не видно'}</strong>
        <p>${risk
          ? `Порог ${escapeHTML(String(p.different_ip_threshold))} адресов сочетается с хранением кандидатов ${escapeHTML(fmtDuration(p.candidate_storage_seconds))}. Для мобильных пулов это может быть слишком агрессивно.`
          : 'Текущее сочетание порога и времени хранения кандидатов не попало под встроенный профиль повышенного риска.'}</p>
      </div>
    </div>
    <div class="risk-pairs">
      <div><span>Порог /24</span><strong>${escapeHTML(String(p.different_ip_threshold || '—'))}</strong></div>
      <div><span>Кандидаты хранятся</span><strong>${escapeHTML(fmtDuration(p.candidate_storage_seconds))}</strong></div>
      <div><span>Блокировка подсети</span><strong>${escapeHTML(fmtDuration(p.subnet_ban_seconds))}</strong></div>
      <div><span>Порог новых подключений</span><strong>${escapeHTML(String(p.recent_hitcount || '—'))}</strong></div>
    </div>
  `;
}

function renderProtection() {
  const p = snapshot?.protection || {};
  const cfg = snapshot?.config || {};
  const rows = [
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Порты роутера', (cfg.ports || []).join(', ') || '—'],
    ['Проброшенные порты', (cfg.forwarded_ports || []).join(', ') || '—'],
    ['Защита IP и подсетей', boolText(Boolean(p.ips_ban_enabled))],
    ['Окно новых соединений', fmtDuration(p.recent_window_seconds)],
    ['Порог новых подключений', p.recent_hitcount || '—'],
    ['Лимит одновременных соединений', p.concurrent_connection_limit || '—'],
    ['Блокировка IP', fmtDuration(p.direct_ip_ban_seconds)],
    ['Порог разных IP в /24', p.different_ip_threshold || '—'],
    ['Хранение кандидатов', fmtDuration(p.candidate_storage_seconds)],
    ['Блокировка /24', fmtDuration(p.subnet_ban_seconds)],
    ['Ловушка', boolText(Boolean(p.honeypot_enabled))],
    ['Порты ловушки', (cfg.honeypot_ports || []).join(', ') || '—'],
    ['Пользовательские исключения', boolText(Boolean(cfg.use_custom_exclude_list))],
    ['Режим пользовательского списка', cfg.custom_lists_block_mode || '0'],
    ['Режим Geo', cfg.geoblock_mode || '0'],
    ['Страны Geo', (cfg.geoblock_countries || []).join(', ') || '—'],
    ['Страны-исключения Geo', (cfg.geo_exclude_countries || []).join(', ') || '—'],
    ['Импорт блокировок Keenetic', boolText(Boolean(cfg.read_ndm_lockout_ipsets))],
    ['Сохранение наборов ipset', boolText(Boolean(cfg.save_ipsets))]
  ];
  node('protection').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');
}

function renderWarnings() {
  const warnings = localizeMessages([...(snapshot?.warnings || []), ...(snapshot?.errors || [])]);
  if (!warnings.length) {
    node('warnings').innerHTML = '<div class="ok-note">Явных признаков повышенного риска в текущем состоянии не найдено.</div>';
    return;
  }
  node('warnings').innerHTML = warnings.map((warning) => `
    <div class="warning-item"><span>!</span><p>${escapeHTML(warning)}</p></div>
  `).join('');
}

function isBenignUnknownSetCount(item) {
  return Boolean(
    item?.exists &&
    !item?.count_known &&
    item?.error === 'ipset entry count missing'
  );
}

function renderSets() {
  const sets = snapshot?.ipsets || [];
  node('sets').innerHTML = sets.map((item) => {
    const countUnavailable = isBenignUnknownSetCount(item);
    const hasOperationalError = Boolean(item.error) && !countUnavailable;
    const state = item.exists ? (hasOperationalError ? 'ЧАСТИЧНО' : 'АКТИВЕН') : 'НЕТ';
    const cls = item.exists ? (hasOperationalError ? 'warn' : 'good') : 'neutral';
    const count = item.count_known ? String(item.count ?? 0) : '—';
    const detail = countUnavailable
      ? '<small class="row-note">Число записей недоступно</small>'
      : (hasOperationalError ? `<small class="row-error">${escapeHTML(localizeMessage(item.error))}</small>` : '');
    return `<tr>
      <td class="mono strong">${escapeHTML(item.name)}</td>
      <td>${escapeHTML(setDescriptions[item.name] || 'Набор Antiscan')}</td>
      <td><span class="state ${cls}">${state}</span>${detail}</td>
      <td class="mono">${count}</td>
    </tr>`;
  }).join('');
}

function verdictTitle(result) {
  if (result.verdict === 'blocked') return 'ЗАБЛОКИРОВАН';
  if (result.verdict === 'excluded') return 'ИСКЛЮЧЁН';
  if (result.verdict === 'not-blocked') return 'НЕ ЗАБЛОКИРОВАН';
  if (result.verdict === 'candidate') return 'КАНДИДАТ';
  if (result.verdict === 'stopped') return 'ANTISCAN ОСТАНОВЛЕН';
  if (result.verdict === 'not-detected') return 'ANTISCAN НЕ ОБНАРУЖЕН';
  return 'НЕДОСТАТОЧНО ДАННЫХ';
}

function renderInspect(result) {
  const root = node('inspectResult');
  const cls = result.blocked ? 'bad' : result.conclusive ? 'good' : 'warn';
  const evidence = (result.evidence || []).map((item) => `
    <div class="evidence-row">
      <div><strong>${escapeHTML(reasonLabel(item.kind))}</strong><span class="mono">${escapeHTML(item.set || '')}</span></div>
      <p>${escapeHTML(evidenceSummary(item))}</p>
    </div>
  `).join('');
  const warnings = localizeMessages(result.warnings).map((item) => `<div class="inspect-warning">${escapeHTML(item)}</div>`).join('');
  const explanation = reasonExplanation(result);
  const actions = result.ip && (result.blocked || result.verdict === 'candidate') ? `
    <div class="inspect-actions">
      <button class="button small" type="button" data-action="exclude" data-entry="${escapeHTML(result.ip)}">Добавить IP в исключения</button>
    </div>` : '';
  root.className = `inspect-result ${cls}`;
  root.innerHTML = `
    <div class="inspect-verdict">
      <div><span class="mono">${escapeHTML(result.ip || '')}</span><strong>${verdictTitle(result)}</strong></div>
      <span class="state ${cls}">${escapeHTML(reasonLabel(result.reason || result.verdict || 'unknown'))}</span>
    </div>
    ${explanation ? `<div class="reason-explanation">${escapeHTML(explanation)}</div>` : ''}
    ${evidence || '<div class="muted inspect-empty">Совпадения в доступных наборах Antiscan не найдены.</div>'}
    ${warnings}
    ${actions}
  `;
}

function entrySecondary(entry) {
  const bits = [];
  if (entry.timeout_known) {
    const seconds = Number(entry.timeout_seconds || 0);
    if (seconds === 0) {
      bits.push('без таймаута');
    } else {
      const expires = new Date(Date.now() + seconds * 1000);
      bits.push(`осталось ${fmtDuration(seconds)} · примерно до ${expires.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`);
    }
  }
  if (entry.packets_known) bits.push(`${entry.packets} пак.`);
  if (entry.bytes_known) bits.push(fmtBytes(entry.bytes));
  return bits.join(' · ') || 'запись в наборе';
}

function canUnbanSet(name) {
  return ['ascn_ips', 'ascn_subnets', 'ascn_honeypot'].includes(name);
}

function blockedEntryActions(setName, entry) {
  const buttons = [];
  if (canUnbanSet(setName)) {
    buttons.push(`<button class="button small danger" type="button" data-action="unban" data-set="${escapeHTML(setName)}" data-entry="${escapeHTML(entry.value)}">Снять блокировку</button>`);
  }
  if (['ascn_ips', 'ascn_subnets', 'ascn_honeypot', 'ascn_ndm_lockout', 'ascn_candidates'].includes(setName)) {
    buttons.push(`<button class="button small" type="button" data-action="exclude" data-entry="${escapeHTML(entry.value)}">В исключения</button>`);
  }
  return buttons.length ? `<span class="entry-actions">${buttons.join('')}</span>` : '';
}

function currentRuntimeListMode() {
  return document.documentElement.dataset.section === 'geo' ? 'geo' : 'lists';
}

function runtimeListOptions(mode) {
  return mode === 'geo'
    ? [
        ['ascn_geo_exclude', 'Geo-исключения'],
        ['ascn_geo_blacklist', 'Geo-чёрный список'],
        ['ascn_geo_whitelist', 'Geo-белый список']
      ]
    : [
        ['ascn_custom_exclude', 'Пользовательские исключения'],
        ['ascn_custom_blacklist', 'Пользовательский чёрный список'],
        ['ascn_custom_whitelist', 'Пользовательский белый список']
      ];
}

function configureListSetMode(mode) {
  const select = node('listSet');
  const title = node('runtimeListsTitle');
  const description = node('runtimeListsDescription');
  const filter = node('listFilter');
  if (!select || !title || !description || !filter) return;

  const options = runtimeListOptions(mode);
  const previousName = browserState[mode]?.name || '';
  const changedMode = select.dataset.mode !== mode;
  select.dataset.mode = mode;
  select.value = options[0][0];
  select.innerHTML = options.map(([value, label]) =>
    `<option value="${value}">${label}</option>`
  ).join('');
  if (previousName && options.some(([value]) => value === previousName)) {
    select.value = previousName;
  }
  if (changedMode) filter.value = '';

  if (mode === 'geo') {
    title.textContent = 'Geo-наборы Antiscan';
    description.textContent = 'Текущее содержимое Geo ipset. Для больших наборов показываются первые 500 записей.';
  } else {
    title.textContent = 'Активные пользовательские списки';
    description.textContent = 'Текущее содержимое runtime ipset для blacklist, whitelist и exclude.';
  }

  if (browserState[mode]) {
    renderSetPage(mode);
  } else {
    node('listMeta').textContent = '—';
    node('listEntries').className = 'entry-table empty-state';
    node('listEntries').textContent = 'Выберите набор и загрузите текущее состояние.';
  }
}

function renderSetPage(kind) {
  const page = browserState[kind];
  const blocked = kind === 'blocked';
  if (!blocked && kind !== currentRuntimeListMode()) return;
  const target = blocked ? node('blockedEntries') : node('listEntries');
  const meta = blocked ? node('blockedMeta') : node('listMeta');
  const filter = (blocked ? node('blockedFilter').value : node('listFilter').value).trim().toLowerCase();

  if (!page) {
    target.className = 'entry-table empty-state';
    target.textContent = 'Набор ещё не загружен.';
    meta.textContent = '—';
    return;
  }
  if (page.error) {
    target.className = 'entry-table empty-state bad-text';
    target.textContent = localizeMessage(page.error);
    meta.textContent = page.name || '—';
    return;
  }
  if (!page.exists) {
    target.className = 'entry-table empty-state';
    target.textContent = `${page.name}: набор сейчас отсутствует.`;
    meta.textContent = 'НЕТ';
    return;
  }

  const all = page.entries || [];
  const entries = filter ? all.filter((entry) => String(entry.value || '').toLowerCase().includes(filter)) : all;
  const total = page.count_known ? page.count : all.length;
  meta.textContent = `${page.name} · ${total} записей${page.truncated ? ` · показаны первые ${page.limit}` : ''}`;

  if (!entries.length) {
    target.className = 'entry-table empty-state';
    target.textContent = filter ? 'По фильтру ничего не найдено.' : 'Набор пуст.';
    return;
  }

  target.className = 'entry-table';
  target.innerHTML = entries.map((entry) => {
    const actions = blocked ? blockedEntryActions(page.name, entry) : '';
    return `
      <div class="entry-row ${actions ? 'has-actions' : ''}">
        <span class="mono entry-value">${escapeHTML(entry.value)}</span>
        <span class="entry-meta-wrap">
          <span class="entry-meta">${escapeHTML(entrySecondary(entry))}</span>
          <small>${escapeHTML(setOperationalExplanation(page.name))}</small>
        </span>
        ${actions}
      </div>`;
  }).join('');
}

async function loadSet(kind) {
  const blocked = kind === 'blocked';
  const select = blocked ? node('blockedSet') : node('listSet');
  const button = blocked ? node('reloadBlocked') : node('reloadList');
  const target = blocked ? node('blockedEntries') : node('listEntries');
  button.disabled = true;
  if (blocked || kind === currentRuntimeListMode()) {
    target.className = 'entry-table empty-state';
    target.textContent = 'Читаем текущий набор ipset…';
  }
  try {
    browserState[kind] = await api(`sets?name=${encodeURIComponent(select.value)}&limit=500`);
    renderSetPage(kind);
  } catch (error) {
    browserState[kind] = { name: select.value, error: error.message || 'Не удалось прочитать ipset.' };
    renderSetPage(kind);
  } finally {
    button.disabled = false;
  }
}

function mutationSuccessMessage(context, payload) {
  if (!payload?.changed) {
    if (context === 'unban') return 'Запись уже отсутствовала в активной блокировке.';
    if (context === 'list-entry') return 'Такая запись уже есть в выбранном списке.';
    if (context === 'lifecycle:start') return 'Antiscan уже запущен.';
    if (context === 'lifecycle:stop') return 'Antiscan уже остановлен.';
    if (context === 'schedule:apply') return 'ascn_crontab.conf уже совпадал с формой; активный crontab синхронизирован.';
    if (context.startsWith('flush:')) return 'Выбранные данные уже были пусты; штатная очистка проверена.';
    if (context.startsWith('operation:')) return 'Команда не потребовалась: текущее состояние уже корректно.';
    return 'Состояние уже соответствовало запросу.';
  }
  if (context === 'unban') return 'Блокировка снята, результат проверен.';
  if (context === 'list-entry') return 'Запись добавлена в список и результат проверен.';
  if (context === 'lifecycle:start') return 'Antiscan запущен, состояние проверено.';
  if (context === 'lifecycle:stop') return 'Antiscan остановлен. Защита отключена до следующего запуска.';
  if (context === 'lifecycle:reload') return 'Antiscan перечитал конфигурацию, состояние проверено.';
  if (context === 'lifecycle:restart') return 'Antiscan штатно перезапущен, состояние проверено.';
  if (context === 'rci-token:set') return 'RCI-токен сохранён и проверен штатной командой Antiscan.';
  if (context === 'rci-token:check') return 'Сохранённый RCI-токен прошёл проверку Keenetic.';
  if (context === 'rci-token:delete') return 'RCI-токен удалён, состояние проверено.';
  if (context === 'schedule:apply') return 'Расписание Antiscan обновлено и синхронизировано с активным crontab.';
  if (context.startsWith('flush:')) return 'Штатная очистка Antiscan выполнена, результат проверен.';
  if (context.startsWith('operation:')) return 'Штатная сервисная команда Antiscan выполнена и проверена.';
  if (context === 'config') {
    return payload?.runtime_applied
      ? 'Настройки сохранены, применены и проверены.'
      : 'Настройки сохранены и проверены. Они вступят в силу при следующем запуске Antiscan.';
  }
  return 'Изменение применено и проверено.';
}

function showMutationResult(payload, failed = false, context = '') {
  const notice = node('actionNotice');
  notice.hidden = false;
  notice.className = `notice action-notice ${failed ? 'bad' : 'good'}`;
  const warnings = localizeMessages(payload?.warnings);
  if (failed) {
    const prefix = payload?.rollback_performed
      ? 'Операция не выполнена. Предыдущее состояние восстановлено.'
      : 'Операцию выполнить не удалось.';
    const reason = localizeMessage(payload?.error || payload?.message || '');
    notice.textContent = [prefix, reason, ...warnings].filter(Boolean).join(' · ');
    return;
  }
  const messages = [mutationSuccessMessage(context, payload), ...warnings];
  if (payload?.restart_required) {
    messages.push(context.startsWith('flush:')
      ? 'После штатной очистки восстановите нужные данные и перезапустите Antiscan перед возвратом соответствующей защиты.'
      : 'Требуется остановить и снова запустить Antiscan, чтобы полностью применить новый IPSETS_DIRECTORY.');
  }
  notice.textContent = messages.filter(Boolean).join(' · ');
}

async function performUnban(setName, entry) {
  if (!window.confirm(`Снять блокировку только с этой записи?\n\n${setName}\n${entry}\n\nОстальные записи не изменятся.`)) return;
  try {
    const result = await mutate('unban', { set: setName, entry, confirm: 'UNBAN' });
    showMutationResult(result, false, 'unban');
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'unban');
  }
}

async function performListEntry(listName, entry) {
  const target = listName === 'exclude' ? 'пользовательские исключения' : 'пользовательский белый список';
  if (!entry) return;
  if (!window.confirm(`Добавить запись в ${target}?\n\n${entry}\n\nФайл изменится атомарно. Если список активен, Antiscan перечитает его и RouterForge проверит результат.`)) return;
  try {
    const result = await mutate('list-entry', { list: listName, entry, confirm: 'ADD' });
    showMutationResult(result, false, 'list-entry');
    await loadStatus();
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'list-entry');
  }
}

function customListLabel(name) {
  return {
    blacklist: 'Чёрный список',
    whitelist: 'Белый список',
    exclude: 'Исключения'
  }[name] || name || 'Пользовательский список';
}

function renderCustomList(page) {
  browserState.customSource = page;
  const target = node('customListEntries');
  const meta = node('customListMeta');
  const clearButton = node('clearCustomList');
  const reloadRuntimeButton = node('reloadCustomListRuntime');
  if (!target || !meta) return;

  if (page?.error) {
    target.className = 'entry-table empty-state bad-text';
    target.textContent = localizeMessage(page.error);
    meta.textContent = customListLabel(page.list);
    return;
  }

  const state = page?.active ? 'активен' : page?.configured ? 'включён, Antiscan остановлен' : 'выключен';
  const runtime = page?.active ? (page.runtime_exists ? 'runtime есть' : 'runtime отсутствует') : 'runtime не требуется';
  const skipped = Number(page?.skipped_invalid || 0);
  meta.textContent = `${customListLabel(page?.list)} · записей ${Number(page?.count || 0)} · ${state} · ${runtime}${skipped ? ` · пропущено строк ${skipped}` : ''}`;

  if (clearButton) {
    clearButton.disabled = customListBusy || (Number(page?.count || 0) === 0 && !page?.runtime_exists);
    clearButton.title = page?.active
      ? 'Активный список будет очищен штатной командой Antiscan flush.'
      : 'Исходный файл будет очищен до нулевой длины.';
  }
  if (reloadRuntimeButton) {
    reloadRuntimeButton.disabled = customListBusy || !page?.configured || !page?.running;
  }

  const entries = page?.entries || [];
  if (!entries.length) {
    target.className = 'entry-table empty-state';
    target.textContent = 'В исходном файле нет корректных IPv4/CIDR записей.';
    return;
  }

  target.className = 'entry-table';
  target.innerHTML = entries.map((entry) => {
    return `<div class="entry-row has-actions">
      <span class="mono entry-value">${escapeHTML(entry)}</span>
      <span class="entry-meta-wrap"><span class="entry-meta">исходный файл ${escapeHTML(page?.set || '')}</span><small>${page?.active ? 'Изменение будет применено штатной командой Antiscan; удаление последней записи перейдёт в upstream flush.' : 'Изменение затронет только исходный файл.'}</small></span>
      <span class="entry-actions"><button class="button small danger" type="button" data-custom-delete="${escapeHTML(entry)}">Удалить</button></span>
    </div>`;
  }).join('');
}

async function loadCustomList() {
  const select = node('customListName');
  const button = node('reloadCustomList');
  if (!select || !button) return;
  button.disabled = true;
  try {
    const page = await api(`custom-lists?list=${encodeURIComponent(select.value)}`);
    renderCustomList(page);
  } catch (error) {
    renderCustomList({ list: select.value, error: error.message || 'Не удалось прочитать исходный пользовательский список.' });
  } finally {
    button.disabled = false;
  }
}

async function performCustomListMutation(action, entry = '') {
  const list = node('customListName')?.value || '';
  const prompts = {
    add: `Добавить запись в ${customListLabel(list)}?\n\n${entry}\n\nФайл будет изменён атомарно. Если список активен, Antiscan штатно перечитает custom ipset и RouterForge проверит результат.`,
    delete: `Удалить запись из ${customListLabel(list)}?\n\n${entry}\n\nЕсли список активен, Antiscan перечитает custom ipset и RouterForge проверит удаление.`,
    clear: `Очистить все адреса из ${customListLabel(list)}?\n\nНеактивный исходный файл будет обнулён. Для активного списка RouterForge вызовет штатный Antiscan flush; опасный пустой whitelist будет заблокирован safety-gate.`,
    reload: `Перечитать ${customListLabel(list)} из исходного файла?\n\nБудет вызван штатный update_ipsets custom и проверен runtime ipset.`
  };
  if (!prompts[action]) return;
  if ((action === 'add' || action === 'delete') && !entry) return;
  if (!window.confirm(prompts[action])) return;

  customListBusy = true;
  try {
    const result = await mutate('custom-list', {
      action,
      list,
      entry: action === 'add' || action === 'delete' ? entry : '',
      confirm: action.toUpperCase()
    });
    showMutationResult(result, false, `custom-list:${action}`);
    if (action === 'add') node('customListEntry').value = '';
    await loadCustomList();
    await loadStatus();
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `custom-list:${action}`);
    await loadCustomList();
  } finally {
    customListBusy = false;
  }
}

async function performLifecycle(action) {
  if (configDirty) {
    const notice = node('notice');
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Есть несохранённые изменения ascn.conf. Сначала примените их или сбросьте форму.';
    return;
  }

  const prompts = {
    start: 'Запустить Antiscan?\n\nБудут созданы штатные наборы ipset и правила фильтрации Antiscan.',
    stop: 'Остановить Antiscan?\n\nЗащита Antiscan будет отключена до следующего запуска. Сохранение состояния при остановке выполняет сам Antiscan согласно SAVE_ON_EXIT.',
    reload: 'Перечитать текущий ascn.conf?\n\nAntiscan штатно перестроит необходимые правила и наборы, после чего RouterForge проверит итоговое состояние.',
    restart: 'Штатно перезапустить Antiscan?\n\nБудет вызван именно S99ascn restart. Upstream выполнит свой stop 1, затем start и сохранит предусмотренную им семантику перезапуска.'
  };
  if (!prompts[action] || !window.confirm(prompts[action])) return;

  lifecycleBusy = true;
  renderLifecycleControls();
  try {
    const result = await mutate('lifecycle', { action, confirm: action.toUpperCase() });
    showMutationResult(result, false, `lifecycle:${action}`);
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `lifecycle:${action}`);
  } finally {
    lifecycleBusy = false;
    renderLifecycleControls();
  }
}

async function performOperation(action, scope = '') {
  if (configDirty) {
    const notice = node('notice');
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Есть несохранённые изменения ascn.conf. Сначала примените их или сбросьте форму.';
    return;
  }

  const key = scope ? `${action}:${scope}` : action;
  const prompts = {
    update_rules: 'Восстановить правила Antiscan?\n\nБудет вызвана штатная команда S99ascn update_rules. Upstream сам идемпотентно проверит и восстановит недостающие правила и jump-привязки.',
    read_candidates: 'Обработать текущих кандидатов?\n\nAntiscan проверит накопленные адреса и при достижении порога перенесёт соответствующие /24 в блокировку подсетей.',
    read_ndm_ipsets: 'Импортировать текущие блокировки Keenetic?\n\nAntiscan перечитает системные lockout ipset и добавит найденные IPv4 в свой runtime-набор.',
    save_ipsets: 'Сохранить runtime ipset?\n\nAntiscan выполнит штатный save_ipsets согласно SAVE_IPSETS и IPSETS_DIRECTORY.',
    'update_ipsets:custom': 'Перечитать активные пользовательские списки?\n\nAntiscan штатно пересоздаст активные custom ipset и восстановит правила.',
    'update_ipsets:geo': 'Обновить Geo-списки сейчас?\n\nAntiscan скачает актуальные подсети настроенных стран. Операция может занять несколько минут.',
    retry_load_geo: 'Повторить неудачную загрузку Geo?\n\nAntiscan выполнит штатный retry_load_geo и при успехе уберёт временную retry-задачу.',
    update_crontab: 'Синхронизировать cron с ascn_crontab.conf?\n\nБудут изменены только строки Antiscan, штатной командой update_crontab.'
  };
  if (!prompts[key] || !window.confirm(prompts[key])) return;

  operationBusy = true;
  renderLifecycleControls();
  const output = node('operationOutput');
  if (output) output.textContent = 'Выполняется штатная команда Antiscan…';
  try {
    const result = await mutate('operation', { action, scope, confirm: 'RUN' });
    showMutationResult(result, false, `operation:${key}`);
    if (output) output.textContent = result.output || 'Команда завершена без текстового вывода.';
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    const payload = error.payload || { error: error.message };
    showMutationResult(payload, true, `operation:${key}`);
    if (output) output.textContent = localizeMessage(payload.error || error.message || 'Ошибка сервисной операции.');
  } finally {
    operationBusy = false;
    renderLifecycleControls();
  }
}

function auditActionTitle(action) {
  if (String(action || '').startsWith('flush:')) {
    return `Очистка / восстановление: ${flushTargetLabel(String(action).slice(6))}`;
  }
  return {
    unban: 'Снятие одной блокировки',
    'list-entry': 'Изменение пользовательского списка',
    'custom-list:add': 'Добавление в пользовательский список',
    'custom-list:delete': 'Удаление из пользовательского списка',
    'custom-list:clear': 'Очистка пользовательского списка',
    'custom-list:reload': 'Перечитывание пользовательского списка',
    'rci-token:set': 'Установка RCI-токена',
    'rci-token:check': 'Проверка RCI-токена',
    'rci-token:delete': 'Удаление RCI-токена',
    'lifecycle:start': 'Запуск Antiscan',
    'lifecycle:stop': 'Остановка Antiscan',
    'lifecycle:reload': 'Перечитывание конфигурации',
    'lifecycle:restart': 'Перезапуск Antiscan',
    'operation:update_rules': 'Восстановление правил Antiscan',
    'operation:read_candidates': 'Обработка кандидатов',
    'operation:read_ndm_ipsets': 'Импорт блокировок Keenetic',
    'operation:save_ipsets': 'Сохранение runtime ipset',
    'operation:update_ipsets:custom': 'Перечитывание пользовательских списков',
    'operation:update_ipsets:geo': 'Обновление Geo-списков',
    'operation:retry_load_geo': 'Повторная загрузка Geo',
    'operation:update_crontab': 'Синхронизация cron',
    'schedule:apply': 'Изменение расписания Antiscan',
    config: 'Изменение конфигурации'
  }[action] || action || 'Действие Antiscan Manager';
}

function renderHistory(page) {
  const events = page?.events || [];
  const target = node('historyEntries');
  const meta = node('historyMeta');
  historyLoaded = true;
  meta.textContent = `${events.length} событий · новые сверху${page?.skipped_invalid_lines ? ` · пропущено повреждённых строк: ${page.skipped_invalid_lines}` : ''}`;

  if (!events.length) {
    target.className = 'history-list empty-state';
    target.textContent = 'В журнале пока нет действий Antiscan Manager.';
    return;
  }

  target.className = 'history-list';
  target.innerHTML = events.map((event) => {
    const failed = event.outcome !== 'success';
    const flags = [];
    if (event.changed) flags.push('<span class="history-flag">изменено</span>');
    if (event.verified) flags.push('<span class="history-flag good">проверено</span>');
    if (event.rollback) flags.push('<span class="history-flag bad">выполнен откат</span>');
    if (event.restart_required) flags.push('<span class="history-flag warn">нужен перезапуск</span>');
    const warnings = localizeMessages(event.warnings).map((item) => `<small>⚠ ${escapeHTML(item)}</small>`).join('');
    return `
      <article class="history-row ${failed ? 'failed' : ''}">
        <div class="history-main">
          <div class="history-title">
            <span class="state ${failed ? 'bad' : 'good'}">${failed ? 'ОШИБКА' : 'УСПЕХ'}</span>
            <strong>${escapeHTML(auditActionTitle(event.action))}</strong>
            <span class="mono muted">${escapeHTML(fmtAuditTime(event.timestamp))}</span>
          </div>
          <p>${escapeHTML(localizeMessage(event.summary || '—'))}</p>
          ${event.target ? `<span class="mono history-target">${escapeHTML(event.target)}</span>` : ''}
          ${warnings}
        </div>
        <div class="history-side">
          <span class="mono">${Number(event.duration_ms || 0)} мс</span>
          <span class="mono">HTTP ${escapeHTML(String(event.http_status || '—'))}</span>
          <div class="history-flags">${flags.join('')}</div>
        </div>
      </article>`;
  }).join('');
}

async function loadHistory() {
  const button = node('reloadHistory');
  if (button) button.disabled = true;
  try {
    const page = await api('history?limit=50');
    renderHistory(page);
  } catch (error) {
    historyLoaded = true;
    node('historyMeta').textContent = 'Не удалось прочитать историю';
    node('historyEntries').className = 'history-list empty-state bad-text';
    node('historyEntries').textContent = localizeMessage(error.message || 'Не удалось прочитать журнал действий.');
  } finally {
    if (button) button.disabled = false;
  }
}

async function loadStatus() {
  node('refresh').disabled = true;
  try {
    snapshot = await api('status');
    renderState();
    renderRuntimeSummary();
    renderRiskSummary();
    renderProtection();
    renderWarnings();
    renderSets();
    populateConfigEditor(false);
  } catch (error) {
    const notice = node('notice');
    notice.hidden = false;
    notice.className = 'notice bad';
    notice.textContent = localizeMessage(error.message || 'Не удалось получить статус Antiscan Manager.');
  } finally {
    node('refresh').disabled = false;
  }
}

async function inspectIP() {
  const ip = node('inspectIp').value.trim();
  if (!ip) return;
  node('inspectButton').disabled = true;
  node('inspectResult').className = 'inspect-result empty';
  node('inspectResult').textContent = 'Проверяем текущее состояние Antiscan…';
  try {
    const result = await api(`inspect?ip=${encodeURIComponent(ip)}`);
    renderInspect(result);
  } catch (error) {
    const payload = error.payload || {};
    renderInspect({
      ip,
      conclusive: false,
      blocked: false,
      verdict: payload.verdict || 'unknown',
      warnings: localizeMessages(payload.warnings || [error.message || 'Ошибка проверки'])
    });
  } finally {
    node('inspectButton').disabled = false;
  }
}


  function chooseTab(section){
    activateTab(section);
    if(section==='blocked'&&!browserState.blocked)loadSet('blocked');
    if(section==='blocked'&&!flushPreviewLoaded)loadFlushPreview();
    if(section==='lists'){
      configureListSetMode('lists');
      if(!browserState.lists)loadSet('lists');
      if(!browserState.customSource)loadCustomList();
    }
    if(section==='geo'){
      configureListSetMode('geo');
      if(!browserState.geo)loadSet('geo');
    }
    if(section==='history'&&!historyLoaded)loadHistory();
    if(section==='diagnostics'&&!diagnosticsLoaded)loadDiagnostics();
    if(section==='diagnostics'&&!compatibilityLoaded)loadCompatibility();
    if(section==='diagnostics'&&!rciTokenLoaded)loadRCITokenStatus();
    if(section==='schedule'&&!schedulerLoaded)loadScheduler();
  }
  async function refreshAll(){
    await loadStatus();
    if(browserState.blocked)await loadSet('blocked');
    if(flushPreviewLoaded)await loadFlushPreview();
    if(browserState.lists)await loadSet('lists');
    if(browserState.customSource)await loadCustomList();
    if(historyLoaded)await loadHistory();
    if(diagnosticsLoaded)await loadDiagnostics();
    if(compatibilityLoaded)await loadCompatibility();
    if(rciTokenLoaded)await loadRCITokenStatus();
    if(schedulerLoaded)await loadScheduler();
  }
  function handleDelegation(event){
    if (!event.target?.closest?.('.as-page')) return;
    const customDelete=event.target.closest('[data-custom-delete]');
    if(customDelete){performCustomListMutation('delete',customDelete.dataset.customDelete||'');return;}
    const button=event.target.closest('[data-action]');
    if(!button)return;
    const action=button.dataset.action,entry=button.dataset.entry||'';
    if(action==='unban')performUnban(button.dataset.set||'',entry);
    if(action==='exclude')performListEntry('exclude',entry);
  }
  onMount(()=>{
    startRouterForgeBridge();
    document.addEventListener('click', handleDelegation);
    loadStatus();
    return()=>{
      document.removeEventListener('click', handleDelegation);
      routerForgeResizeObserver?.disconnect();
      routerForgeHostObserver?.disconnect();
      window.removeEventListener('resize',reportRouterForgeModuleHeight);
    };
  });
</script>

  <main class="as-page">
    <header class="hero">
      <div class="hero-copy">

        <div class="hero-title-row">
          <h1>Antiscan Manager</h1>

        </div>
        <p>Диагностика и безопасное управление Antiscan: понятные причины блокировок, настройка с откатом и журнал последних действий.</p>
      </div>
      <div class="hero-actions">
        <span id="targetState" class={cells["targetState"].className} hidden={cells["targetState"].hidden}>{#if cells["targetState"].innerHTML !== undefined}{@html cells["targetState"].innerHTML}{:else if cells["targetState"].textContent !== undefined}{cells["targetState"].textContent}{:else}ПРОВЕРКА…{/if}</span>
        <button id="refresh" class={cells["refresh"].className} type="button" hidden={cells["refresh"].hidden} disabled={cells["refresh"].disabled} onclick={refreshAll}>{#if cells["refresh"].innerHTML !== undefined}{@html cells["refresh"].innerHTML}{:else if cells["refresh"].textContent !== undefined}{cells["refresh"].textContent}{:else}↻ Обновить{/if}</button>
      </div>
    </header>

    <section id="notice" class={cells["notice"].className} hidden={cells["notice"].hidden}>{#if cells["notice"].innerHTML !== undefined}{@html cells["notice"].innerHTML}{:else if cells["notice"].textContent !== undefined}{cells["notice"].textContent}{:else}{/if}</section>
    <section id="actionNotice" class={cells["actionNotice"].className} hidden={cells["actionNotice"].hidden}>{#if cells["actionNotice"].innerHTML !== undefined}{@html cells["actionNotice"].innerHTML}{:else if cells["actionNotice"].textContent !== undefined}{cells["actionNotice"].textContent}{:else}{/if}</section>

    <section class="metrics" aria-label="Antiscan summary">
      <article class="metric"><span>Версия</span><strong id="version" class={cells["version"].className} hidden={cells["version"].hidden}>{#if cells["version"].innerHTML !== undefined}{@html cells["version"].innerHTML}{:else if cells["version"].textContent !== undefined}{cells["version"].textContent}{:else}—{/if}</strong><small>оригинальный Antiscan</small></article>
      <article class="metric"><span>Прямые блокировки</span><strong id="blockedIps" class={cells["blockedIps"].className} hidden={cells["blockedIps"].hidden}>{#if cells["blockedIps"].innerHTML !== undefined}{@html cells["blockedIps"].innerHTML}{:else if cells["blockedIps"].textContent !== undefined}{cells["blockedIps"].textContent}{:else}—{/if}</strong><small>ascn_ips</small></article>
      <article class="metric"><span>Заблокировано /24</span><strong id="blockedSubnets" class={cells["blockedSubnets"].className} hidden={cells["blockedSubnets"].hidden}>{#if cells["blockedSubnets"].innerHTML !== undefined}{@html cells["blockedSubnets"].innerHTML}{:else if cells["blockedSubnets"].textContent !== undefined}{cells["blockedSubnets"].textContent}{:else}—{/if}</strong><small>ascn_subnets</small></article>
      <article class="metric"><span>Кандидаты</span><strong id="candidateCount" class={cells["candidateCount"].className} hidden={cells["candidateCount"].hidden}>{#if cells["candidateCount"].innerHTML !== undefined}{@html cells["candidateCount"].innerHTML}{:else if cells["candidateCount"].textContent !== undefined}{cells["candidateCount"].textContent}{:else}—{/if}</strong><small>ascn_candidates</small></article>
    </section>

    <nav class="tabs" aria-label="Antiscan Manager sections">
      <button class={cells["rf-generated-1"].className} type="button" data-tab="overview" id="rf-generated-1" hidden={cells["rf-generated-1"].hidden} disabled={cells["rf-generated-1"].disabled} onclick={() => chooseTab("overview")}>{#if cells["rf-generated-1"].innerHTML !== undefined}{@html cells["rf-generated-1"].innerHTML}{:else if cells["rf-generated-1"].textContent !== undefined}{cells["rf-generated-1"].textContent}{:else}Обзор{/if}</button>
      <button class={cells["rf-generated-2"].className} type="button" data-tab="diagnostics" id="rf-generated-2" hidden={cells["rf-generated-2"].hidden} disabled={cells["rf-generated-2"].disabled} onclick={() => chooseTab("diagnostics")}>{#if cells["rf-generated-2"].innerHTML !== undefined}{@html cells["rf-generated-2"].innerHTML}{:else if cells["rf-generated-2"].textContent !== undefined}{cells["rf-generated-2"].textContent}{:else}Диагностика{/if}</button>
      <button class={cells["rf-generated-3"].className} type="button" data-tab="blocked" id="rf-generated-3" hidden={cells["rf-generated-3"].hidden} disabled={cells["rf-generated-3"].disabled} onclick={() => chooseTab("blocked")}>{#if cells["rf-generated-3"].innerHTML !== undefined}{@html cells["rf-generated-3"].innerHTML}{:else if cells["rf-generated-3"].textContent !== undefined}{cells["rf-generated-3"].textContent}{:else}Блокировки{/if}</button>
      <button class={cells["rf-generated-4"].className} type="button" data-tab="lists" id="rf-generated-4" hidden={cells["rf-generated-4"].hidden} disabled={cells["rf-generated-4"].disabled} onclick={() => chooseTab("lists")}>{#if cells["rf-generated-4"].innerHTML !== undefined}{@html cells["rf-generated-4"].innerHTML}{:else if cells["rf-generated-4"].textContent !== undefined}{cells["rf-generated-4"].textContent}{:else}Списки{/if}</button>
      <button class={cells["rf-generated-5"].className} type="button" data-tab="geo" id="rf-generated-5" hidden={cells["rf-generated-5"].hidden} disabled={cells["rf-generated-5"].disabled} onclick={() => chooseTab("geo")}>{#if cells["rf-generated-5"].innerHTML !== undefined}{@html cells["rf-generated-5"].innerHTML}{:else if cells["rf-generated-5"].textContent !== undefined}{cells["rf-generated-5"].textContent}{:else}Geo{/if}</button>
      <button class={cells["rf-generated-6"].className} type="button" data-tab="schedule" id="rf-generated-6" hidden={cells["rf-generated-6"].hidden} disabled={cells["rf-generated-6"].disabled} onclick={() => chooseTab("schedule")}>{#if cells["rf-generated-6"].innerHTML !== undefined}{@html cells["rf-generated-6"].innerHTML}{:else if cells["rf-generated-6"].textContent !== undefined}{cells["rf-generated-6"].textContent}{:else}Расписание{/if}</button>
      <button class={cells["rf-generated-7"].className} type="button" data-tab="config" id="rf-generated-7" hidden={cells["rf-generated-7"].hidden} disabled={cells["rf-generated-7"].disabled} onclick={() => chooseTab("config")}>{#if cells["rf-generated-7"].innerHTML !== undefined}{@html cells["rf-generated-7"].innerHTML}{:else if cells["rf-generated-7"].textContent !== undefined}{cells["rf-generated-7"].textContent}{:else}Настройки{/if}</button>
      <button class={cells["rf-generated-8"].className} type="button" data-tab="history" id="rf-generated-8" hidden={cells["rf-generated-8"].hidden} disabled={cells["rf-generated-8"].disabled} onclick={() => chooseTab("history")}>{#if cells["rf-generated-8"].innerHTML !== undefined}{@html cells["rf-generated-8"].innerHTML}{:else if cells["rf-generated-8"].textContent !== undefined}{cells["rf-generated-8"].textContent}{:else}История{/if}</button>
    </nav>

    <section class={cells["rf-generated-9"].className} data-page="overview" id="rf-generated-9" hidden={cells["rf-generated-9"].hidden}>{#if cells["rf-generated-9"].innerHTML !== undefined}{@html cells["rf-generated-9"].innerHTML}{:else if cells["rf-generated-9"].textContent !== undefined}{cells["rf-generated-9"].textContent}{:else}
      <div class="two-column overview-grid">
        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">ТЕКУЩЕЕ СОСТОЯНИЕ</span>
              <h2>Что сейчас работает</h2>
              <p>Текущее состояние Antiscan без каких-либо изменений системы.</p>
            </div>
            <span id="ipsetBinary" class={cells["ipsetBinary"].className} hidden={cells["ipsetBinary"].hidden}>{#if cells["ipsetBinary"].innerHTML !== undefined}{@html cells["ipsetBinary"].innerHTML}{:else if cells["ipsetBinary"].textContent !== undefined}{cells["ipsetBinary"].textContent}{:else}ipset: —{/if}</span>
          </div>
          <div id="runtimeSummary" class={cells["runtimeSummary"].className} hidden={cells["runtimeSummary"].hidden}>{#if cells["runtimeSummary"].innerHTML !== undefined}{@html cells["runtimeSummary"].innerHTML}{:else if cells["runtimeSummary"].textContent !== undefined}{cells["runtimeSummary"].textContent}{:else}{/if}</div>
        </section>

        <section class="panel risk-panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">РИСК ЛОЖНЫХ БЛОКИРОВОК</span>
              <h2>Мобильные сети и /24</h2>
              <p>Проверяем сочетания настроек, которые могут случайно зацепить динамические адреса оператора.</p>
            </div>
          </div>
          <div id="riskSummary" class={cells["riskSummary"].className} hidden={cells["riskSummary"].hidden}>{#if cells["riskSummary"].innerHTML !== undefined}{@html cells["riskSummary"].innerHTML}{:else if cells["riskSummary"].textContent !== undefined}{cells["riskSummary"].textContent}{:else}{/if}</div>
        </section>
      </div>

      <section class="panel lifecycle-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">УПРАВЛЕНИЕ</span>
            <h2>Управление Antiscan</h2>
            <p>Запуск, остановка и перечитывание настроек выполняются только штатным скриптом <span class="mono">S99ascn</span>. Правила Antiscan RouterForge не подменяет.</p>
          </div>
          <span id="lifecycleState" class={cells["lifecycleState"].className} hidden={cells["lifecycleState"].hidden}>{#if cells["lifecycleState"].innerHTML !== undefined}{@html cells["lifecycleState"].innerHTML}{:else if cells["lifecycleState"].textContent !== undefined}{cells["lifecycleState"].textContent}{:else}ПРОВЕРКА…{/if}</span>
        </div>
        <div class="lifecycle-body">
          <div class="lifecycle-copy">
            <strong>Безопасное управление</strong>
            <span id="lifecycleHint" class={cells["lifecycleHint"].className} hidden={cells["lifecycleHint"].hidden}>{#if cells["lifecycleHint"].innerHTML !== undefined}{@html cells["lifecycleHint"].innerHTML}{:else if cells["lifecycleHint"].textContent !== undefined}{cells["lifecycleHint"].textContent}{:else}Состояние загружается…{/if}</span>
          </div>
          <div class="lifecycle-actions">
            <button id="startAntiscan" class={cells["startAntiscan"].className} type="button" disabled={cells["startAntiscan"].disabled} hidden={cells["startAntiscan"].hidden} onclick={() => performLifecycle('start')}>{#if cells["startAntiscan"].innerHTML !== undefined}{@html cells["startAntiscan"].innerHTML}{:else if cells["startAntiscan"].textContent !== undefined}{cells["startAntiscan"].textContent}{:else}▶ Запустить{/if}</button>
            <button id="reloadAntiscan" class={cells["reloadAntiscan"].className} type="button" disabled={cells["reloadAntiscan"].disabled} hidden={cells["reloadAntiscan"].hidden} onclick={() => performLifecycle('reload')}>{#if cells["reloadAntiscan"].innerHTML !== undefined}{@html cells["reloadAntiscan"].innerHTML}{:else if cells["reloadAntiscan"].textContent !== undefined}{cells["reloadAntiscan"].textContent}{:else}↻ Перечитать{/if}</button>
            <button id="restartAntiscan" class={cells["restartAntiscan"].className} type="button" disabled={cells["restartAntiscan"].disabled} hidden={cells["restartAntiscan"].hidden} onclick={() => performLifecycle('restart')}>{#if cells["restartAntiscan"].innerHTML !== undefined}{@html cells["restartAntiscan"].innerHTML}{:else if cells["restartAntiscan"].textContent !== undefined}{cells["restartAntiscan"].textContent}{:else}↻ Перезапустить{/if}</button>
            <button id="stopAntiscan" class={cells["stopAntiscan"].className} type="button" disabled={cells["stopAntiscan"].disabled} hidden={cells["stopAntiscan"].hidden} onclick={() => performLifecycle('stop')}>{#if cells["stopAntiscan"].innerHTML !== undefined}{@html cells["stopAntiscan"].innerHTML}{:else if cells["stopAntiscan"].textContent !== undefined}{cells["stopAntiscan"].textContent}{:else}■ Остановить{/if}</button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">СЕРВИСНЫЕ ОПЕРАЦИИ</span>
            <h2>Обслуживание</h2>
            <p>Восстановление правил, обработка кандидатов, импорт блокировок Keenetic и сохранение текущего состояния.</p>
          </div>
          <span id="operationState" class={cells["operationState"].className} hidden={cells["operationState"].hidden}>{#if cells["operationState"].innerHTML !== undefined}{@html cells["operationState"].innerHTML}{:else if cells["operationState"].textContent !== undefined}{cells["operationState"].textContent}{:else}ГОТОВО{/if}</span>
        </div>
        <div class="lifecycle-body">
          <div class="lifecycle-copy">
            <strong>Сервисные действия</strong>
            <span>Редкие операции, которые обычно нужны только при диагностике или восстановлении Antiscan.</span>
          </div>
          <div class="lifecycle-actions">
            <button class={cells["rf-generated-10"].className} type="button" data-operation="update_rules" id="rf-generated-10" hidden={cells["rf-generated-10"].hidden} disabled={cells["rf-generated-10"].disabled} onclick={() => performOperation("read_candidates","")}>{#if cells["rf-generated-10"].innerHTML !== undefined}{@html cells["rf-generated-10"].innerHTML}{:else if cells["rf-generated-10"].textContent !== undefined}{cells["rf-generated-10"].textContent}{:else}Восстановить правила{/if}</button>
            <button class={cells["rf-generated-11"].className} type="button" data-operation="read_candidates" id="rf-generated-11" hidden={cells["rf-generated-11"].hidden} disabled={cells["rf-generated-11"].disabled} onclick={() => performOperation("read_ndm_ipsets","")}>{#if cells["rf-generated-11"].innerHTML !== undefined}{@html cells["rf-generated-11"].innerHTML}{:else if cells["rf-generated-11"].textContent !== undefined}{cells["rf-generated-11"].textContent}{:else}Обработать кандидатов{/if}</button>
            <button class={cells["rf-generated-12"].className} type="button" data-operation="read_ndm_ipsets" id="rf-generated-12" hidden={cells["rf-generated-12"].hidden} disabled={cells["rf-generated-12"].disabled} onclick={() => performOperation("save_ipsets","")}>{#if cells["rf-generated-12"].innerHTML !== undefined}{@html cells["rf-generated-12"].innerHTML}{:else if cells["rf-generated-12"].textContent !== undefined}{cells["rf-generated-12"].textContent}{:else}Импорт Keenetic{/if}</button>
            <button class={cells["rf-generated-13"].className} type="button" data-operation="save_ipsets" id="rf-generated-13" hidden={cells["rf-generated-13"].hidden} disabled={cells["rf-generated-13"].disabled} onclick={() => performOperation("update_ipsets","geo")}>{#if cells["rf-generated-13"].innerHTML !== undefined}{@html cells["rf-generated-13"].innerHTML}{:else if cells["rf-generated-13"].textContent !== undefined}{cells["rf-generated-13"].textContent}{:else}Сохранить ipset{/if}</button>
          </div>
        </div>
        <div id="operationOutput" class={cells["operationOutput"].className} hidden={cells["operationOutput"].hidden}>{#if cells["operationOutput"].innerHTML !== undefined}{@html cells["operationOutput"].innerHTML}{:else if cells["operationOutput"].textContent !== undefined}{cells["operationOutput"].textContent}{:else}Последняя сервисная операция ещё не запускалась.{/if}</div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">НАБОРЫ IPSET</span>
            <h2>Состояние наборов</h2>
            <p>Количество записей во всех известных наборах ipset Antiscan.</p>
          </div>
        </div>
        <div class="table-scroll">
          <table>
            <thead><tr><th>Набор</th><th>Назначение</th><th>Состояние</th><th>Записей</th></tr></thead>
            <tbody id="sets" class={cells["sets"].className} hidden={cells["sets"].hidden}>{#if cells["sets"].innerHTML !== undefined}{@html cells["sets"].innerHTML}{:else}{/if}</tbody>
          </table>
        </div>
      </section>
    {/if}</section>

    <section class={cells["rf-generated-14"].className} data-page="diagnostics" id="rf-generated-14" hidden={cells["rf-generated-14"].hidden}>{#if cells["rf-generated-14"].innerHTML !== undefined}{@html cells["rf-generated-14"].innerHTML}{:else if cells["rf-generated-14"].textContent !== undefined}{cells["rf-generated-14"].textContent}{:else}
      <section class="panel diagnostics-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ГОТОВНОСТЬ ANTISCAN</span>
            <h2>Диагностика окружения</h2>
            <p>Проверяем штатные файлы, зависимости, Netfilter, hook, cron, RCI и lock-состояние без изменения настроек.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="diagnosticsState" class={cells["diagnosticsState"].className} hidden={cells["diagnosticsState"].hidden}>{#if cells["diagnosticsState"].innerHTML !== undefined}{@html cells["diagnosticsState"].innerHTML}{:else if cells["diagnosticsState"].textContent !== undefined}{cells["diagnosticsState"].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadDiagnostics" class={cells["reloadDiagnostics"].className} type="button" hidden={cells["reloadDiagnostics"].hidden} disabled={cells["reloadDiagnostics"].disabled} onclick={loadDiagnostics}>{#if cells["reloadDiagnostics"].innerHTML !== undefined}{@html cells["reloadDiagnostics"].innerHTML}{:else if cells["reloadDiagnostics"].textContent !== undefined}{cells["reloadDiagnostics"].textContent}{:else}↻ Проверить{/if}</button>
          </div>
        </div>
        <div id="diagnosticsMeta" class={cells["diagnosticsMeta"].className} hidden={cells["diagnosticsMeta"].hidden}>{#if cells["diagnosticsMeta"].innerHTML !== undefined}{@html cells["diagnosticsMeta"].innerHTML}{:else if cells["diagnosticsMeta"].textContent !== undefined}{cells["diagnosticsMeta"].textContent}{:else}Откройте вкладку, чтобы выполнить read-only проверку.{/if}</div>
        <div id="diagnosticsChecks" class={cells["diagnosticsChecks"].className} hidden={cells["diagnosticsChecks"].hidden}>{#if cells["diagnosticsChecks"].innerHTML !== undefined}{@html cells["diagnosticsChecks"].innerHTML}{:else if cells["diagnosticsChecks"].textContent !== undefined}{cells["diagnosticsChecks"].textContent}{:else}Диагностика ещё не запускалась.{/if}</div>
      </section>

      <section class="panel rci-token-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">RCI / KEENETIC</span>
            <h2>RCI-токен</h2>
            <p>Статус token-flow Antiscan и безопасное управление штатными командами <span class="mono">token set/check/delete</span>.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="rciTokenState" class={cells["rciTokenState"].className} hidden={cells["rciTokenState"].hidden}>{#if cells["rciTokenState"].innerHTML !== undefined}{@html cells["rciTokenState"].innerHTML}{:else if cells["rciTokenState"].textContent !== undefined}{cells["rciTokenState"].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadRCIToken" class={cells["reloadRCIToken"].className} type="button" hidden={cells["reloadRCIToken"].hidden} disabled={cells["reloadRCIToken"].disabled} onclick={loadRCITokenStatus}>{#if cells["reloadRCIToken"].innerHTML !== undefined}{@html cells["reloadRCIToken"].innerHTML}{:else if cells["reloadRCIToken"].textContent !== undefined}{cells["reloadRCIToken"].textContent}{:else}↻ Обновить{/if}</button>
          </div>
        </div>
        <div class="rci-token-body">
          <div class="rci-token-copy">
            <strong id="rciTokenSummary" class={cells["rciTokenSummary"].className} hidden={cells["rciTokenSummary"].hidden}>{#if cells["rciTokenSummary"].innerHTML !== undefined}{@html cells["rciTokenSummary"].innerHTML}{:else if cells["rciTokenSummary"].textContent !== undefined}{cells["rciTokenSummary"].textContent}{:else}Состояние RCI-токена ещё не загружено.{/if}</strong>
            <span id="rciTokenMeta" class={cells["rciTokenMeta"].className} hidden={cells["rciTokenMeta"].hidden}>{#if cells["rciTokenMeta"].innerHTML !== undefined}{@html cells["rciTokenMeta"].innerHTML}{:else if cells["rciTokenMeta"].textContent !== undefined}{cells["rciTokenMeta"].textContent}{:else}Значение токена RouterForge не читает и не показывает.{/if}</span>
            <span class="rci-token-security">Токен передаётся upstream только через stdin. Он не попадает в argv, URL, ответ API или журнал операций.</span>
          </div>
          <div class="rci-token-controls">
            <input id="rciTokenInput" type="password" autocomplete="new-password" spellcheck="false" placeholder="Новый RCI-токен" aria-label="Новый RCI-токен" class={cells["rciTokenInput"].className} hidden={cells["rciTokenInput"].hidden} disabled={cells["rciTokenInput"].disabled} bind:value={cells["rciTokenInput"].value} onkeydown={(event) => {if(event.key === 'Enter') performRCITokenAction('set');}}>
            <button id="setRCIToken" class={cells["setRCIToken"].className} type="button" disabled={cells["setRCIToken"].disabled} hidden={cells["setRCIToken"].hidden} onclick={() => performRCITokenAction('set')}>{#if cells["setRCIToken"].innerHTML !== undefined}{@html cells["setRCIToken"].innerHTML}{:else if cells["setRCIToken"].textContent !== undefined}{cells["setRCIToken"].textContent}{:else}Сохранить токен{/if}</button>
            <button id="checkRCIToken" class={cells["checkRCIToken"].className} type="button" disabled={cells["checkRCIToken"].disabled} hidden={cells["checkRCIToken"].hidden} onclick={() => performRCITokenAction('check')}>{#if cells["checkRCIToken"].innerHTML !== undefined}{@html cells["checkRCIToken"].innerHTML}{:else if cells["checkRCIToken"].textContent !== undefined}{cells["checkRCIToken"].textContent}{:else}Проверить{/if}</button>
            <button id="deleteRCIToken" class={cells["deleteRCIToken"].className} type="button" disabled={cells["deleteRCIToken"].disabled} hidden={cells["deleteRCIToken"].hidden} onclick={() => performRCITokenAction('delete')}>{#if cells["deleteRCIToken"].innerHTML !== undefined}{@html cells["deleteRCIToken"].innerHTML}{:else if cells["deleteRCIToken"].textContent !== undefined}{cells["deleteRCIToken"].textContent}{:else}Удалить{/if}</button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">СОВМЕСТИМОСТЬ</span>
            <h2>Совместимость и обновления</h2>
            <p>Учтённый функционал upstream привязан к проверенной версии. RouterForge не устанавливает обновления Antiscan: здесь только read-only состояние пакета, update cache, прошивки, Netfilter и hook.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="compatibilityState" class={cells["compatibilityState"].className} hidden={cells["compatibilityState"].hidden}>{#if cells["compatibilityState"].innerHTML !== undefined}{@html cells["compatibilityState"].innerHTML}{:else if cells["compatibilityState"].textContent !== undefined}{cells["compatibilityState"].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadCompatibility" class={cells["reloadCompatibility"].className} type="button" hidden={cells["reloadCompatibility"].hidden} disabled={cells["reloadCompatibility"].disabled} onclick={loadCompatibility}>{#if cells["reloadCompatibility"].innerHTML !== undefined}{@html cells["reloadCompatibility"].innerHTML}{:else if cells["reloadCompatibility"].textContent !== undefined}{cells["reloadCompatibility"].textContent}{:else}↻ Перечитать{/if}</button>
          </div>
        </div>
        <div id="compatibilityMeta" class={cells["compatibilityMeta"].className} hidden={cells["compatibilityMeta"].hidden}>{#if cells["compatibilityMeta"].innerHTML !== undefined}{@html cells["compatibilityMeta"].innerHTML}{:else if cells["compatibilityMeta"].textContent !== undefined}{cells["compatibilityMeta"].textContent}{:else}Откройте вкладку, чтобы проверить совместимость без сетевых запросов и изменений системы.{/if}</div>
        <div id="compatibilityGrid" class={cells["compatibilityGrid"].className} hidden={cells["compatibilityGrid"].hidden}>{#if cells["compatibilityGrid"].innerHTML !== undefined}{@html cells["compatibilityGrid"].innerHTML}{:else if cells["compatibilityGrid"].textContent !== undefined}{cells["compatibilityGrid"].textContent}{:else}{/if}</div>
        <div id="compatibilityWarnings" class={cells["compatibilityWarnings"].className} hidden={cells["compatibilityWarnings"].hidden}>{#if cells["compatibilityWarnings"].innerHTML !== undefined}{@html cells["compatibilityWarnings"].innerHTML}{:else if cells["compatibilityWarnings"].textContent !== undefined}{cells["compatibilityWarnings"].textContent}{:else}{/if}</div>
        <div id="diagnosticsContract" class={cells["diagnosticsContract"].className} hidden={cells["diagnosticsContract"].hidden}>{#if cells["diagnosticsContract"].innerHTML !== undefined}{@html cells["diagnosticsContract"].innerHTML}{:else if cells["diagnosticsContract"].textContent !== undefined}{cells["diagnosticsContract"].textContent}{:else}{/if}</div>
      </section>
    {/if}</section>
    <section class={cells["rf-generated-15"].className} data-page="inspect" id="rf-generated-15" hidden={cells["rf-generated-15"].hidden}>{#if cells["rf-generated-15"].innerHTML !== undefined}{@html cells["rf-generated-15"].innerHTML}{:else if cells["rf-generated-15"].textContent !== undefined}{cells["rf-generated-15"].textContent}{:else}
      <section class="panel inspector-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ДИАГНОСТИКА БЛОКИРОВКИ</span>
            <h2>Почему заблокирован этот IP?</h2>
            <p>Проверяем известные наборы <span class="mono">ascn_*</span> в том же порядке, в котором их учитывает Antiscan.</p>
          </div>
        </div>
        <div class="inspect-form">
          <input id="inspectIp" autocomplete="off" inputmode="decimal" spellcheck="false" placeholder="Например, 100.64.10.77" class={cells["inspectIp"].className} hidden={cells["inspectIp"].hidden} disabled={cells["inspectIp"].disabled} bind:value={cells["inspectIp"].value} onkeydown={(event) => {if(event.key === "Enter") inspectIP();}}>
          <button id="inspectButton" class={cells["inspectButton"].className} type="button" hidden={cells["inspectButton"].hidden} disabled={cells["inspectButton"].disabled} onclick={inspectIP}>{#if cells["inspectButton"].innerHTML !== undefined}{@html cells["inspectButton"].innerHTML}{:else if cells["inspectButton"].textContent !== undefined}{cells["inspectButton"].textContent}{:else}Проверить IP{/if}</button>
        </div>
        <div id="inspectResult" class={cells["inspectResult"].className} hidden={cells["inspectResult"].hidden}>{#if cells["inspectResult"].innerHTML !== undefined}{@html cells["inspectResult"].innerHTML}{:else if cells["inspectResult"].textContent !== undefined}{cells["inspectResult"].textContent}{:else}Введите IPv4-адрес. Эта проверка ничего не меняет в Antiscan.{/if}</div>
      </section>

      <section class="panel mobile-help">
        <div class="panel-head">
          <div>
            <span class="section-kicker">МОБИЛЬНЫЕ СЕТИ</span>
            <h2>Почему мобильный адрес может попасть под раздачу</h2>
          </div>
        </div>
        <div class="explain-grid">
          <article><strong>1</strong><span>Antiscan запоминает адреса-кандидаты.</span></article>
          <article><strong>2</strong><span>Несколько адресов одной /24 могут накопиться за период хранения.</span></article>
          <article><strong>3</strong><span>После порога вся /24 попадает в <span class="mono">ascn_subnets</span>.</span></article>
          <article><strong>4</strong><span>Новый легитимный мобильный IP из этой /24 тоже оказывается заблокирован.</span></article>
        </div>
      </section>
    {/if}</section>

    <section class={cells["rf-generated-16"].className} data-page="blocked" id="rf-generated-16" hidden={cells["rf-generated-16"].hidden}>{#if cells["rf-generated-16"].innerHTML !== undefined}{@html cells["rf-generated-16"].innerHTML}{:else if cells["rf-generated-16"].textContent !== undefined}{cells["rf-generated-16"].textContent}{:else}
      <section class="panel set-browser-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">ТЕКУЩИЕ БЛОКИРОВКИ</span>
            <h2>Блокировки и кандидаты</h2>
            <p>Реальные записи выбранного набора. Для блокировок Antiscan можно снять только одну конкретную запись; остальные записи не изменяются.</p>
          </div>
          <div class="browser-controls">
            <select id="blockedSet" aria-label="Выберите Antiscan set" class={cells["blockedSet"].className} hidden={cells["blockedSet"].hidden} disabled={cells["blockedSet"].disabled} bind:value={cells["blockedSet"].value} onchange={() => loadSet('blocked')}>{#if cells["blockedSet"].innerHTML !== undefined}{@html cells["blockedSet"].innerHTML}{:else}
              <option value="ascn_ips">Прямые IP · ascn_ips</option>
              <option value="ascn_subnets">Подсети /24 · ascn_subnets</option>
              <option value="ascn_honeypot">Ловушка · ascn_honeypot</option>
              <option value="ascn_ndm_lockout">Блокировка Keenetic · ascn_ndm_lockout</option>
              <option value="ascn_candidates">Кандидаты · ascn_candidates</option>
            {/if}</select>
            <button id="reloadBlocked" class={cells["reloadBlocked"].className} type="button" hidden={cells["reloadBlocked"].hidden} disabled={cells["reloadBlocked"].disabled} onclick={() => loadSet('blocked')}>{#if cells["reloadBlocked"].innerHTML !== undefined}{@html cells["reloadBlocked"].innerHTML}{:else if cells["reloadBlocked"].textContent !== undefined}{cells["reloadBlocked"].textContent}{:else}Загрузить{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="blockedFilter" autocomplete="off" spellcheck="false" placeholder="Фильтр по IP или подсети" class={cells["blockedFilter"].className} hidden={cells["blockedFilter"].hidden} disabled={cells["blockedFilter"].disabled} bind:value={cells["blockedFilter"].value} oninput={() => renderSetPage('blocked')}>
          <span id="blockedMeta" class={cells["blockedMeta"].className} hidden={cells["blockedMeta"].hidden}>{#if cells["blockedMeta"].innerHTML !== undefined}{@html cells["blockedMeta"].innerHTML}{:else if cells["blockedMeta"].textContent !== undefined}{cells["blockedMeta"].textContent}{:else}—{/if}</span>
        </div>
        <div id="blockedEntries" class={cells["blockedEntries"].className} hidden={cells["blockedEntries"].hidden}>{#if cells["blockedEntries"].innerHTML !== undefined}{@html cells["blockedEntries"].innerHTML}{:else if cells["blockedEntries"].textContent !== undefined}{cells["blockedEntries"].textContent}{:else}Выберите набор и загрузите текущее состояние.{/if}</div>
      </section>

      <section class="panel flush-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">ШТАТНАЯ ОЧИСТКА / ВОССТАНОВЛЕНИЕ</span>
            <h2>Очистить набор Antiscan</h2>
            <p>RouterForge вызывает только штатный <span class="mono">S99ascn flush</span>, заранее показывает затрагиваемые наборы и после операции проверяет фактическое состояние.</p>
          </div>
          <div class="browser-controls">
            <select id="flushTarget" aria-label="Выберите набор для очистки" class={cells["flushTarget"].className} hidden={cells["flushTarget"].hidden} disabled={cells["flushTarget"].disabled} bind:value={cells["flushTarget"].value} onchange={loadFlushPreview}>{#if cells["flushTarget"].innerHTML !== undefined}{@html cells["flushTarget"].innerHTML}{:else}
              <option value="ips">Прямые IP</option>
              <option value="subnets">Подсети /24</option>
              <option value="candidates">Кандидаты</option>
              <option value="honeypot">Ловушка</option>
              <option value="ndm_lockout">Блокировки Keenetic</option>
              <option value="custom_blacklist">Пользовательский чёрный список</option>
              <option value="custom_whitelist">Пользовательский белый список</option>
              <option value="custom_exclude">Пользовательские исключения</option>
              <option value="geo">Все Geo-наборы</option>
              <option value="all">Upstream «все»</option>
            {/if}</select>
            <button id="reloadFlushPreview" class={cells["reloadFlushPreview"].className} type="button" hidden={cells["reloadFlushPreview"].hidden} disabled={cells["reloadFlushPreview"].disabled} onclick={loadFlushPreview}>{#if cells["reloadFlushPreview"].innerHTML !== undefined}{@html cells["reloadFlushPreview"].innerHTML}{:else if cells["reloadFlushPreview"].textContent !== undefined}{cells["reloadFlushPreview"].textContent}{:else}↻ Проверить{/if}</button>
          </div>
        </div>
        <div class="flush-summary">
          <span id="flushState" class={cells["flushState"].className} hidden={cells["flushState"].hidden}>{#if cells["flushState"].innerHTML !== undefined}{@html cells["flushState"].innerHTML}{:else if cells["flushState"].textContent !== undefined}{cells["flushState"].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
          <div id="flushMeta" class={cells["flushMeta"].className} hidden={cells["flushMeta"].hidden}>{#if cells["flushMeta"].innerHTML !== undefined}{@html cells["flushMeta"].innerHTML}{:else if cells["flushMeta"].textContent !== undefined}{cells["flushMeta"].textContent}{:else}Выберите цель, чтобы увидеть точные последствия штатной команды flush.{/if}</div>
        </div>
        <div id="flushWarnings" class={cells["flushWarnings"].className} hidden={cells["flushWarnings"].hidden}>{#if cells["flushWarnings"].innerHTML !== undefined}{@html cells["flushWarnings"].innerHTML}{:else if cells["flushWarnings"].textContent !== undefined}{cells["flushWarnings"].textContent}{:else}{/if}</div>
        <div class="flush-actions">
          <button id="flushSelected" class={cells["flushSelected"].className} type="button" disabled={cells["flushSelected"].disabled} hidden={cells["flushSelected"].hidden} onclick={performFlush}>{#if cells["flushSelected"].innerHTML !== undefined}{@html cells["flushSelected"].innerHTML}{:else if cells["flushSelected"].textContent !== undefined}{cells["flushSelected"].textContent}{:else}Очистить выбранное{/if}</button>
          <span class="muted">Для активных whitelist с опасной комбинацией настроек операция блокируется до изменения конфигурации.</span>
        </div>
      </section>
    {/if}</section>

    <section class={cells["rf-generated-17"].className} data-page="lists" id="rf-generated-17" hidden={cells["rf-generated-17"].hidden}>{#if cells["rf-generated-17"].innerHTML !== undefined}{@html cells["rf-generated-17"].innerHTML}{:else if cells["rf-generated-17"].textContent !== undefined}{cells["rf-generated-17"].textContent}{:else}
      <section class="panel set-browser-panel custom-source-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">ПОЛЬЗОВАТЕЛЬСКИЕ СПИСКИ</span>
            <h2>Исходные списки Antiscan</h2>
            <p>Полное управление файлами blacklist, whitelist и exclude. Изменения атомарные; активный список перечитывается штатной командой Antiscan и проверяется по runtime ipset.</p>
          </div>
          <div class="browser-controls">
            <select id="customListName" aria-label="Выберите пользовательский список" class={cells["customListName"].className} hidden={cells["customListName"].hidden} disabled={cells["customListName"].disabled} bind:value={cells["customListName"].value} onchange={loadCustomList}>{#if cells["customListName"].innerHTML !== undefined}{@html cells["customListName"].innerHTML}{:else}
              <option value="blacklist">Чёрный список</option>
              <option value="whitelist">Белый список</option>
              <option value="exclude">Исключения</option>
            {/if}</select>
            <button id="reloadCustomList" class={cells["reloadCustomList"].className} type="button" hidden={cells["reloadCustomList"].hidden} disabled={cells["reloadCustomList"].disabled} onclick={loadCustomList}>{#if cells["reloadCustomList"].innerHTML !== undefined}{@html cells["reloadCustomList"].innerHTML}{:else if cells["reloadCustomList"].textContent !== undefined}{cells["reloadCustomList"].textContent}{:else}Загрузить{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="customListEntry" autocomplete="off" spellcheck="false" placeholder="IPv4 или CIDR, например 100.64.10.0/24" class={cells["customListEntry"].className} hidden={cells["customListEntry"].hidden} disabled={cells["customListEntry"].disabled} bind:value={cells["customListEntry"].value} onkeydown={(event) => {if(event.key === 'Enter') performCustomListMutation('add', node('customListEntry').value.trim());}}>
          <span class="entry-actions">
            <button id="addCustomListEntry" class={cells["addCustomListEntry"].className} type="button" hidden={cells["addCustomListEntry"].hidden} disabled={cells["addCustomListEntry"].disabled} onclick={() => performCustomListMutation('add', node('customListEntry').value.trim())}>{#if cells["addCustomListEntry"].innerHTML !== undefined}{@html cells["addCustomListEntry"].innerHTML}{:else if cells["addCustomListEntry"].textContent !== undefined}{cells["addCustomListEntry"].textContent}{:else}Добавить адрес{/if}</button>
            <button id="reloadCustomListRuntime" class={cells["reloadCustomListRuntime"].className} type="button" hidden={cells["reloadCustomListRuntime"].hidden} disabled={cells["reloadCustomListRuntime"].disabled} onclick={() => performCustomListMutation('reload')}>{#if cells["reloadCustomListRuntime"].innerHTML !== undefined}{@html cells["reloadCustomListRuntime"].innerHTML}{:else if cells["reloadCustomListRuntime"].textContent !== undefined}{cells["reloadCustomListRuntime"].textContent}{:else}Перечитать runtime{/if}</button>
            <button id="clearCustomList" class={cells["clearCustomList"].className} type="button" hidden={cells["clearCustomList"].hidden} disabled={cells["clearCustomList"].disabled} onclick={() => performCustomListMutation('clear')}>{#if cells["clearCustomList"].innerHTML !== undefined}{@html cells["clearCustomList"].innerHTML}{:else if cells["clearCustomList"].textContent !== undefined}{cells["clearCustomList"].textContent}{:else}Очистить{/if}</button>
          </span>
        </div>
        <div id="customListMeta" class={cells["customListMeta"].className} hidden={cells["customListMeta"].hidden}>{#if cells["customListMeta"].innerHTML !== undefined}{@html cells["customListMeta"].innerHTML}{:else if cells["customListMeta"].textContent !== undefined}{cells["customListMeta"].textContent}{:else}Выберите список и загрузите его исходный файл.{/if}</div>
        <div id="customListEntries" class={cells["customListEntries"].className} hidden={cells["customListEntries"].hidden}>{#if cells["customListEntries"].innerHTML !== undefined}{@html cells["customListEntries"].innerHTML}{:else if cells["customListEntries"].textContent !== undefined}{cells["customListEntries"].textContent}{:else}Исходный список ещё не загружен.{/if}</div>
      </section>

      <section class="panel set-browser-panel runtime-lists-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">RUNTIME IPSET</span>
            <h2 id="runtimeListsTitle" class={cells["runtimeListsTitle"].className} hidden={cells["runtimeListsTitle"].hidden}>{#if cells["runtimeListsTitle"].innerHTML !== undefined}{@html cells["runtimeListsTitle"].innerHTML}{:else if cells["runtimeListsTitle"].textContent !== undefined}{cells["runtimeListsTitle"].textContent}{:else}Активные пользовательские списки{/if}</h2>
            <p id="runtimeListsDescription" class={cells["runtimeListsDescription"].className} hidden={cells["runtimeListsDescription"].hidden}>{#if cells["runtimeListsDescription"].innerHTML !== undefined}{@html cells["runtimeListsDescription"].innerHTML}{:else if cells["runtimeListsDescription"].textContent !== undefined}{cells["runtimeListsDescription"].textContent}{:else}Текущее содержимое runtime ipset для blacklist, whitelist и exclude.{/if}</p>
          </div>
          <div class="browser-controls">
            <select id="listSet" aria-label="Выберите list set" class={cells["listSet"].className} hidden={cells["listSet"].hidden} disabled={cells["listSet"].disabled} bind:value={cells["listSet"].value} onchange={() => loadSet(currentRuntimeListMode())}>{#if cells["listSet"].innerHTML !== undefined}{@html cells["listSet"].innerHTML}{:else}
              <option value="ascn_custom_exclude">Пользовательские исключения</option>
              <option value="ascn_custom_blacklist">Пользовательский чёрный список</option>
              <option value="ascn_custom_whitelist">Пользовательский белый список</option>
              <option value="ascn_geo_exclude">Geo-исключения</option>
              <option value="ascn_geo_blacklist">Geo-чёрный список</option>
              <option value="ascn_geo_whitelist">Geo-белый список</option>
            {/if}</select>
            <button id="reloadList" class={cells["reloadList"].className} type="button" hidden={cells["reloadList"].hidden} disabled={cells["reloadList"].disabled} onclick={() => loadSet(currentRuntimeListMode())}>{#if cells["reloadList"].innerHTML !== undefined}{@html cells["reloadList"].innerHTML}{:else if cells["reloadList"].textContent !== undefined}{cells["reloadList"].textContent}{:else}Загрузить{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="listFilter" autocomplete="off" spellcheck="false" placeholder="Фильтр по IP или подсети" class={cells["listFilter"].className} hidden={cells["listFilter"].hidden} disabled={cells["listFilter"].disabled} bind:value={cells["listFilter"].value} oninput={() => renderSetPage(currentRuntimeListMode())}>
          <span id="listMeta" class={cells["listMeta"].className} hidden={cells["listMeta"].hidden}>{#if cells["listMeta"].innerHTML !== undefined}{@html cells["listMeta"].innerHTML}{:else if cells["listMeta"].textContent !== undefined}{cells["listMeta"].textContent}{:else}—{/if}</span>
        </div>
        <div id="listEntries" class={cells["listEntries"].className} hidden={cells["listEntries"].hidden}>{#if cells["listEntries"].innerHTML !== undefined}{@html cells["listEntries"].innerHTML}{:else if cells["listEntries"].textContent !== undefined}{cells["listEntries"].textContent}{:else}Выберите набор и загрузите текущее состояние.{/if}</div>
      </section>

      <section class="panel geo-service-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">GEO</span>
            <h2>Обновление Geo-данных</h2>
            <p>Штатные операции Antiscan для загрузки Geo-наборов и повторной попытки после ошибки. RouterForge не подменяет upstream-логику.</p>
          </div>
          <div class="lifecycle-actions">
            <button class={cells["rf-generated-18"].className} type="button" data-operation="update_ipsets" data-scope="geo" id="rf-generated-18" hidden={cells["rf-generated-18"].hidden} disabled={cells["rf-generated-18"].disabled} onclick={() => performOperation("update_ipsets","geo")}>{#if cells["rf-generated-18"].innerHTML !== undefined}{@html cells["rf-generated-18"].innerHTML}{:else if cells["rf-generated-18"].textContent !== undefined}{cells["rf-generated-18"].textContent}{:else}Обновить Geo{/if}</button>
            <button class={cells["rf-generated-19"].className} type="button" data-operation="retry_load_geo" id="rf-generated-19" hidden={cells["rf-generated-19"].hidden} disabled={cells["rf-generated-19"].disabled} onclick={() => performOperation("retry_load_geo","")}>{#if cells["rf-generated-19"].innerHTML !== undefined}{@html cells["rf-generated-19"].innerHTML}{:else if cells["rf-generated-19"].textContent !== undefined}{cells["rf-generated-19"].textContent}{:else}Повторить после ошибки{/if}</button>
          </div>
        </div>
      </section>
    {/if}</section>

    <section class={cells["rf-generated-20"].className} data-page="protection" id="rf-generated-20" hidden={cells["rf-generated-20"].hidden}>{#if cells["rf-generated-20"].innerHTML !== undefined}{@html cells["rf-generated-20"].innerHTML}{:else if cells["rf-generated-20"].textContent !== undefined}{cells["rf-generated-20"].textContent}{:else}
      <div class="two-column">
        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">ЗАЩИТА</span>
              <h2>Активная политика</h2>
              <p>Значения прочитаны из <span class="mono">ascn.conf</span>.</p>
            </div>
          </div>
          <div id="protection" class={cells["protection"].className} hidden={cells["protection"].hidden}>{#if cells["protection"].innerHTML !== undefined}{@html cells["protection"].innerHTML}{:else if cells["protection"].textContent !== undefined}{cells["protection"].textContent}{:else}{/if}</div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">РИСКИ</span>
              <h2>Что стоит проверить</h2>
              <p>Не приговор, а подсказки по потенциально агрессивным сочетаниям параметров.</p>
            </div>
          </div>
          <div id="warnings" class={cells["warnings"].className} hidden={cells["warnings"].hidden}>{#if cells["warnings"].innerHTML !== undefined}{@html cells["warnings"].innerHTML}{:else if cells["warnings"].textContent !== undefined}{cells["warnings"].textContent}{:else}{/if}</div>
        </section>
      </div>
    {/if}</section>

    <section class={cells["rf-generated-21"].className} data-page="schedule" id="rf-generated-21" hidden={cells["rf-generated-21"].hidden}>{#if cells["rf-generated-21"].innerHTML !== undefined}{@html cells["rf-generated-21"].innerHTML}{:else if cells["rf-generated-21"].textContent !== undefined}{cells["rf-generated-21"].textContent}{:else}
      <section class="panel scheduler-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ПЛАНИРОВЩИК</span>
            <h2>Расписание Antiscan</h2>
            <p>Редактор управляет только штатными задачами из <span class="mono">VALID_TASKS</span>. Произвольные команды и shell-текст не принимаются.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="schedulerState" class={cells["schedulerState"].className} hidden={cells["schedulerState"].hidden}>{#if cells["schedulerState"].innerHTML !== undefined}{@html cells["schedulerState"].innerHTML}{:else if cells["schedulerState"].textContent !== undefined}{cells["schedulerState"].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadScheduler" class={cells["reloadScheduler"].className} type="button" hidden={cells["reloadScheduler"].hidden} disabled={cells["reloadScheduler"].disabled} onclick={loadScheduler}>{#if cells["reloadScheduler"].innerHTML !== undefined}{@html cells["reloadScheduler"].innerHTML}{:else if cells["reloadScheduler"].textContent !== undefined}{cells["reloadScheduler"].textContent}{:else}↻ Обновить{/if}</button>
          </div>
        </div>
        <div id="schedulerMeta" class={cells["schedulerMeta"].className} hidden={cells["schedulerMeta"].hidden}>{#if cells["schedulerMeta"].innerHTML !== undefined}{@html cells["schedulerMeta"].innerHTML}{:else if cells["schedulerMeta"].textContent !== undefined}{cells["schedulerMeta"].textContent}{:else}Откройте вкладку, чтобы загрузить ascn_crontab.conf и активный crontab.{/if}</div>
        <div id="schedulerTaskRows" class={cells["schedulerTaskRows"].className} hidden={cells["schedulerTaskRows"].hidden}>
{#if cells["schedulerTaskRows"].innerHTML !== undefined}
  {@html cells["schedulerTaskRows"].innerHTML}
{:else if cells["schedulerTaskRows"].textContent !== undefined}
  {cells["schedulerTaskRows"].textContent}
{:else if schedulerRows.length}
  {#each schedulerRows as item (item.task)}
  <article class="scheduler-row" data-scheduler-task={item.task} data-automatic={String(item.automatic)}>
    <label class="scheduler-toggle">
      <input data-scheduler-enabled type="checkbox" role="switch" bind:checked={item.enabled} disabled={item.automatic} onchange={updateSchedulerState}>
      <span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span>
      <span><strong>{(schedulerTaskLabels[item.task]||[item.task])[0]}</strong><small class="mono">{item.task}</small></span>
    </label>
    <div class="scheduler-expression-wrap">
      <input data-scheduler-expression class="mono" bind:value={item.schedule} spellcheck="false" disabled={item.automatic} aria-label={'Cron '+item.task} oninput={updateSchedulerState}>
      <small>{(schedulerTaskLabels[item.task]||[item.task,'Штатная задача Antiscan.'])[1]}</small>
    </div>
    {#if item.automatic}<span class="state info">АВТОМАТИЧЕСКИ</span>{:else}<span class="state neutral">5 полей cron</span>{/if}
  </article>
  {/each}
{:else if !schedulerLoaded}Расписание ещё не загружено.{/if}
</div>
        <div class="scheduler-note">
          <strong>Автоматическая задача retry_load_geo</strong>
          <span>Этой строкой управляет сам upstream Antiscan при ошибках Geo. RouterForge показывает и сохраняет её, но не позволяет вручную подменять расписание.</span>
        </div>
        <div class="scheduler-actions">
          <button id="resetScheduler" class={cells["resetScheduler"].className} type="button" disabled={cells["resetScheduler"].disabled} hidden={cells["resetScheduler"].hidden} onclick={() => {if(schedulerSnapshot)renderScheduler(schedulerSnapshot,true);}}>{#if cells["resetScheduler"].innerHTML !== undefined}{@html cells["resetScheduler"].innerHTML}{:else if cells["resetScheduler"].textContent !== undefined}{cells["resetScheduler"].textContent}{:else}Сбросить изменения{/if}</button>
          <button id="applyScheduler" class={cells["applyScheduler"].className} type="button" disabled={cells["applyScheduler"].disabled} hidden={cells["applyScheduler"].hidden} onclick={applyScheduler}>{#if cells["applyScheduler"].innerHTML !== undefined}{@html cells["applyScheduler"].innerHTML}{:else if cells["applyScheduler"].textContent !== undefined}{cells["applyScheduler"].textContent}{:else}Применить расписание{/if}</button>
        </div>
      </section>
    {/if}</section>

    <section class={cells["rf-generated-22"].className} data-page="config" id="rf-generated-22" hidden={cells["rf-generated-22"].hidden}>{#if cells["rf-generated-22"].innerHTML !== undefined}{@html cells["rf-generated-22"].innerHTML}{:else if cells["rf-generated-22"].textContent !== undefined}{cells["rf-generated-22"].textContent}{:else}
      <form id="configForm" class={cells["configForm"].className} autocomplete="off" hidden={cells["configForm"].hidden} onsubmit={(event) => event.preventDefault()} oninput={updateConfigEditorState} onchange={updateConfigEditorState}>{#if cells["configForm"].innerHTML !== undefined}{@html cells["configForm"].innerHTML}{:else if cells["configForm"].textContent !== undefined}{cells["configForm"].textContent}{:else}
        <section class="panel config-header-panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">БЕЗОПАСНАЯ НАСТРОЙКА</span>
              <h2>Настройки ascn.conf</h2>
              <p>Только известные параметры Antiscan, без произвольного shell-текста. Комментарии существующего файла сохраняются.</p>
            </div>
            <div class="config-state-wrap">
              <span id="configState" class={cells["configState"].className} hidden={cells["configState"].hidden}>{#if cells["configState"].innerHTML !== undefined}{@html cells["configState"].innerHTML}{:else if cells["configState"].textContent !== undefined}{cells["configState"].textContent}{:else}ПРОВЕРКА…{/if}</span>
              <span id="configHash" class={cells["configHash"].className} hidden={cells["configHash"].hidden}>{#if cells["configHash"].innerHTML !== undefined}{@html cells["configHash"].innerHTML}{:else if cells["configHash"].textContent !== undefined}{cells["configHash"].textContent}{:else}SHA256 —{/if}</span>
            </div>
          </div>
          <div class="config-transaction-strip">
            <span>проверка</span><b>→</b><span>резервная копия</span><b>→</b><span>атомарная запись</span><b>→</b><span>перечитывание Antiscan</span><b>→</b><span>проверка результата</span><b>→</b><span>откат при ошибке</span>
          </div>
          <div id="configMobileRisk" class={cells["configMobileRisk"].className} hidden={cells["configMobileRisk"].hidden}>{#if cells["configMobileRisk"].innerHTML !== undefined}{@html cells["configMobileRisk"].innerHTML}{:else if cells["configMobileRisk"].textContent !== undefined}{cells["configMobileRisk"].textContent}{:else}Проверяем риск ложной блокировки /24…{/if}</div>
        </section>

        <div class="config-grid">
          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">ТРАФИК</span><h2>Интерфейсы и порты</h2></div></div>
            <div class="field-grid">
              <label class="field wide"><span>Интернет-интерфейсы</span><input id="cfgIspInterfaces" spellcheck="false" placeholder="eth3 ppp0" class={cells["cfgIspInterfaces"].className} hidden={cells["cfgIspInterfaces"].hidden} disabled={cells["cfgIspInterfaces"].disabled} bind:value={cells["cfgIspInterfaces"].value}><small><span class="mono">ISP_INTERFACES</span> · 1–10 имён интерфейсов через пробел.</small></label>
              <label class="field"><span>Порты роутера</span><input id="cfgPorts" spellcheck="false" placeholder="22,80,443" class={cells["cfgPorts"].className} hidden={cells["cfgPorts"].hidden} disabled={cells["cfgPorts"].disabled} bind:value={cells["cfgPorts"].value}><small>Порты/диапазоны через запятую.</small></label>
              <label class="field"><span>Проброшенные порты</span><input id="cfgForwardedPorts" spellcheck="false" placeholder="22,80,443" class={cells["cfgForwardedPorts"].className} hidden={cells["cfgForwardedPorts"].hidden} disabled={cells["cfgForwardedPorts"].disabled} bind:value={cells["cfgForwardedPorts"].value}><small><span class="mono">PORTS_FORWARDED</span> · порты перенаправляемых подключений.</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">IP И ПОДСЕТИ</span><h2>IP и /24 защита</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgEnableIpsBan" type="checkbox" role="switch" class={cells["cfgEnableIpsBan"].className} hidden={cells["cfgEnableIpsBan"].hidden} disabled={cells["cfgEnableIpsBan"].disabled} bind:checked={cells["cfgEnableIpsBan"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>ENABLE_IPS_BAN</strong><small>Прямые IP и распределённая /24 защита.</small></span></label>
              <label class="field"><span>RULES_MASK</span><input id="cfgRulesMask" spellcheck="false" placeholder="255.255.255.255" class={cells["cfgRulesMask"].className} hidden={cells["cfgRulesMask"].hidden} disabled={cells["cfgRulesMask"].disabled} bind:value={cells["cfgRulesMask"].value}></label>
              <label class="field"><span>Окно новых подключений, сек.</span><input id="cfgRecentTime" type="number" min="2" max="3600" class={cells["cfgRecentTime"].className} hidden={cells["cfgRecentTime"].hidden} disabled={cells["cfgRecentTime"].disabled} bind:value={cells["cfgRecentTime"].value}><small class="mono">RECENT_CONNECTIONS_TIME</small></label>
              <label class="field"><span>Порог новых подключений</span><input id="cfgRecentHitcount" type="number" min="2" max="100" class={cells["cfgRecentHitcount"].className} hidden={cells["cfgRecentHitcount"].hidden} disabled={cells["cfgRecentHitcount"].disabled} bind:value={cells["cfgRecentHitcount"].value}><small class="mono">RECENT_CONNECTIONS_HITCOUNT</small></label>
              <label class="field"><span>Лимит одновременных соединений</span><input id="cfgRecentLimit" type="number" min="3" max="1000" class={cells["cfgRecentLimit"].className} hidden={cells["cfgRecentLimit"].hidden} disabled={cells["cfgRecentLimit"].disabled} bind:value={cells["cfgRecentLimit"].value}><small class="mono">RECENT_CONNECTIONS_LIMIT</small></label>
              <label class="field"><span>Блокировка IP, сек.</span><input id="cfgRecentBanTime" type="number" min="0" max="2147483" class={cells["cfgRecentBanTime"].className} hidden={cells["cfgRecentBanTime"].hidden} disabled={cells["cfgRecentBanTime"].disabled} bind:value={cells["cfgRecentBanTime"].value}><small class="mono">RECENT_CONNECTIONS_BANTIME</small></label>
              <label class="field"><span>Кандидаты /24 живут, сек.</span><input id="cfgCandidateStorage" type="number" min="0" max="2147483" class={cells["cfgCandidateStorage"].className} hidden={cells["cfgCandidateStorage"].hidden} disabled={cells["cfgCandidateStorage"].disabled} bind:value={cells["cfgCandidateStorage"].value}></label>
              <label class="field"><span>Порог разных IP в /24</span><input id="cfgDifferentThreshold" type="number" min="2" max="254" class={cells["cfgDifferentThreshold"].className} hidden={cells["cfgDifferentThreshold"].hidden} disabled={cells["cfgDifferentThreshold"].disabled} bind:value={cells["cfgDifferentThreshold"].value}></label>
              <label class="field"><span>Блокировка /24, сек.</span><input id="cfgSubnetBanTime" type="number" min="0" max="2147483" class={cells["cfgSubnetBanTime"].className} hidden={cells["cfgSubnetBanTime"].hidden} disabled={cells["cfgSubnetBanTime"].disabled} bind:value={cells["cfgSubnetBanTime"].value}><small class="mono">SUBNETS_BANTIME</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">ЛОВУШКА</span><h2>Ловушка</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgEnableHoneypot" type="checkbox" role="switch" class={cells["cfgEnableHoneypot"].className} hidden={cells["cfgEnableHoneypot"].hidden} disabled={cells["cfgEnableHoneypot"].disabled} bind:checked={cells["cfgEnableHoneypot"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>ENABLE_HONEYPOT</strong><small>При включении список портов обязателен.</small></span></label>
              <label class="field"><span>Порты ловушки</span><input id="cfgHoneypotPorts" spellcheck="false" placeholder="23,2323,3389" class={cells["cfgHoneypotPorts"].className} hidden={cells["cfgHoneypotPorts"].hidden} disabled={cells["cfgHoneypotPorts"].disabled} bind:value={cells["cfgHoneypotPorts"].value}><small class="mono">HONEYPOT_PORTS</small></label>
              <label class="field"><span>Время блокировки, сек.</span><input id="cfgHoneypotBanTime" type="number" min="0" max="2147483" class={cells["cfgHoneypotBanTime"].className} hidden={cells["cfgHoneypotBanTime"].hidden} disabled={cells["cfgHoneypotBanTime"].disabled} bind:value={cells["cfgHoneypotBanTime"].value}><small class="mono">HONEYPOT_BANTIME</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">СПИСКИ И GEO</span><h2>Списки и страны</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgUseCustomExclude" type="checkbox" role="switch" class={cells["cfgUseCustomExclude"].className} hidden={cells["cfgUseCustomExclude"].hidden} disabled={cells["cfgUseCustomExclude"].disabled} bind:checked={cells["cfgUseCustomExclude"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Пользовательские исключения</strong><small><span class="mono">USE_CUSTOM_EXCLUDE_LIST</span> · использовать ascn_custom_exclude.</small></span></label>
              <label class="field"><span>Режим пользовательского списка</span><select id="cfgCustomMode" class={cells["cfgCustomMode"].className} hidden={cells["cfgCustomMode"].hidden} disabled={cells["cfgCustomMode"].disabled} bind:value={cells["cfgCustomMode"].value}>{#if cells["cfgCustomMode"].innerHTML !== undefined}{@html cells["cfgCustomMode"].innerHTML}{:else}<option value="0">Выключено</option><option value="blacklist">Чёрный список</option><option value="whitelist">Белый список</option>{/if}</select><small class="mono">CUSTOM_LISTS_BLOCK_MODE</small></label>
              <label class="field"><span>Режим Geo</span><select id="cfgGeoMode" class={cells["cfgGeoMode"].className} hidden={cells["cfgGeoMode"].hidden} disabled={cells["cfgGeoMode"].disabled} bind:value={cells["cfgGeoMode"].value}>{#if cells["cfgGeoMode"].innerHTML !== undefined}{@html cells["cfgGeoMode"].innerHTML}{:else}<option value="0">Выключено</option><option value="blacklist">Чёрный список</option><option value="whitelist">Белый список</option>{/if}</select><small class="mono">GEOBLOCK_MODE</small></label>
              <label class="field wide"><span>Страны Geo</span><input id="cfgGeoCountries" spellcheck="false" placeholder="RU US DE" class={cells["cfgGeoCountries"].className} hidden={cells["cfgGeoCountries"].hidden} disabled={cells["cfgGeoCountries"].disabled} bind:value={cells["cfgGeoCountries"].value}><small><span class="mono">GEOBLOCK_COUNTRIES</span> · до 8 ISO-кодов, регистр исправляется автоматически.</small></label>
              <label class="field wide"><span>Страны-исключения Geo</span><input id="cfgGeoExcludeCountries" spellcheck="false" placeholder="FI DE" class={cells["cfgGeoExcludeCountries"].className} hidden={cells["cfgGeoExcludeCountries"].hidden} disabled={cells["cfgGeoExcludeCountries"].disabled} bind:value={cells["cfgGeoExcludeCountries"].value}><small class="mono">GEO_EXCLUDE_COUNTRIES</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">ХРАНЕНИЕ / KEENETIC</span><h2>Хранение и lockout</h2></div></div>
            <div class="field-grid">
              <label class="field wide"><span>IPSETS_DIRECTORY</span><input id="cfgIPSetsDirectory" spellcheck="false" placeholder="/tmp/mnt/DISK/antiscan" class={cells["cfgIPSetsDirectory"].className} hidden={cells["cfgIPSetsDirectory"].hidden} disabled={cells["cfgIPSetsDirectory"].disabled} bind:value={cells["cfgIPSetsDirectory"].value}><small>Путь внутри /opt или /tmp. Upstream запрещает только сам /opt/etc; вложенные каталоги, например /opt/etc/antiscan/ipsets, допустимы.</small></label>
              <label class="switch-field"><input id="cfgSaveIPSets" type="checkbox" role="switch" class={cells["cfgSaveIPSets"].className} hidden={cells["cfgSaveIPSets"].hidden} disabled={cells["cfgSaveIPSets"].disabled} bind:checked={cells["cfgSaveIPSets"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Сохранять наборы</strong><small><span class="mono">SAVE_IPSETS</span> · сохранять накопленные ipset.</small></span></label>
              <label class="switch-field"><input id="cfgSaveOnExit" type="checkbox" role="switch" class={cells["cfgSaveOnExit"].className} hidden={cells["cfgSaveOnExit"].hidden} disabled={cells["cfgSaveOnExit"].disabled} bind:checked={cells["cfgSaveOnExit"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Сохранять при остановке</strong><small><span class="mono">SAVE_ON_EXIT</span> · экспортировать состояние при остановке.</small></span></label>
              <label class="switch-field"><input id="cfgReadNDM" type="checkbox" role="switch" class={cells["cfgReadNDM"].className} hidden={cells["cfgReadNDM"].hidden} disabled={cells["cfgReadNDM"].disabled} bind:checked={cells["cfgReadNDM"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Блокировки Keenetic</strong><small><span class="mono">READ_NDM_LOCKOUT_IPSETS</span> · импортировать ip lockout-policy.</small></span></label>
              <label class="field"><span>Время блокировки Keenetic, сек.</span><input id="cfgLockoutBanTime" type="number" min="0" max="2147483" class={cells["cfgLockoutBanTime"].className} hidden={cells["cfgLockoutBanTime"].hidden} disabled={cells["cfgLockoutBanTime"].disabled} bind:value={cells["cfgLockoutBanTime"].value}><small class="mono">LOCKOUT_IPSET_BANTIME</small></label>
            </div>
          </section>
        </div>

        <section class="panel config-apply-panel">
          <div class="config-apply-copy">
            <strong>Применение настроек</strong>
            <span id="configHint" class={cells["configHint"].className} hidden={cells["configHint"].hidden}>{#if cells["configHint"].innerHTML !== undefined}{@html cells["configHint"].innerHTML}{:else if cells["configHint"].textContent !== undefined}{cells["configHint"].textContent}{:else}Состояние конфигурации загружается…{/if}</span>
          </div>
          <div class="config-actions">
            <button id="resetConfig" class={cells["resetConfig"].className} type="button" disabled={cells["resetConfig"].disabled} hidden={cells["resetConfig"].hidden} onclick={() => {configDirty=false;populateConfigEditor(true);}}>{#if cells["resetConfig"].innerHTML !== undefined}{@html cells["resetConfig"].innerHTML}{:else if cells["resetConfig"].textContent !== undefined}{cells["resetConfig"].textContent}{:else}Сбросить форму{/if}</button>
            <button id="applyConfig" class={cells["applyConfig"].className} type="button" disabled={cells["applyConfig"].disabled} hidden={cells["applyConfig"].hidden} onclick={applyConfigEditor}>{#if cells["applyConfig"].innerHTML !== undefined}{@html cells["applyConfig"].innerHTML}{:else if cells["applyConfig"].textContent !== undefined}{cells["applyConfig"].textContent}{:else}Применить конфигурацию{/if}</button>
          </div>
        </section>
      {/if}</form>
    {/if}</section>

    <section class={cells["rf-generated-23"].className} data-page="history" id="rf-generated-23" hidden={cells["rf-generated-23"].hidden}>{#if cells["rf-generated-23"].innerHTML !== undefined}{@html cells["rf-generated-23"].innerHTML}{:else if cells["rf-generated-23"].textContent !== undefined}{cells["rf-generated-23"].textContent}{:else}
      <section class="panel history-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ИСТОРИЯ ОПЕРАЦИЙ</span>
            <h2>Последние действия</h2>
            <p>Ограниченный журнал последних действий: что было сделано, результат, длительность, откат и необходимость перезапуска. Значения формы ascn.conf в журнал не записываются.</p>
          </div>
          <button id="reloadHistory" class={cells["reloadHistory"].className} type="button" hidden={cells["reloadHistory"].hidden} disabled={cells["reloadHistory"].disabled} onclick={loadHistory}>{#if cells["reloadHistory"].innerHTML !== undefined}{@html cells["reloadHistory"].innerHTML}{:else if cells["reloadHistory"].textContent !== undefined}{cells["reloadHistory"].textContent}{:else}↻ Обновить историю{/if}</button>
        </div>
        <div class="history-toolbar">
          <span id="historyMeta" class={cells["historyMeta"].className} hidden={cells["historyMeta"].hidden}>{#if cells["historyMeta"].innerHTML !== undefined}{@html cells["historyMeta"].innerHTML}{:else if cells["historyMeta"].textContent !== undefined}{cells["historyMeta"].textContent}{:else}История ещё не загружена.{/if}</span>
          <span class="state good">JSONL · 0600 · ограниченный размер</span>
        </div>
        <div id="historyEntries" class={cells["historyEntries"].className} hidden={cells["historyEntries"].hidden}>{#if cells["historyEntries"].innerHTML !== undefined}{@html cells["historyEntries"].innerHTML}{:else if cells["historyEntries"].textContent !== undefined}{cells["historyEntries"].textContent}{:else}Откройте вкладку, чтобы загрузить последние события.{/if}</div>
      </section>

      <section class="panel compact-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">СОСТАВ ЖУРНАЛА</span>
            <h2>Что попадает в историю</h2>
          </div>
        </div>
        <p class="boundary-copy">Журнал хранит только тип операции, безопасную цель, HTTP-код, длительность, факт изменения и проверки, откат, необходимость перезапуска и короткие предупреждения. Тело запроса, значения конфигурации и токены подтверждения не сохраняются.</p>
      </section>
    {/if}</section>

  </main>
