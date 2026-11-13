<script>
  import {onMount} from 'svelte';
  import AntiscanSetEntries from './AntiscanSetEntries.svelte';
  import AntiscanInspectorResult from './AntiscanInspectorResult.svelte';
  import AntiscanKeyValueRows from './AntiscanKeyValueRows.svelte';
  import AntiscanWarningItems from './AntiscanWarningItems.svelte';
  import AntiscanDiagnosticChecks from './AntiscanDiagnosticChecks.svelte';
  import AntiscanHistoryRows from './AntiscanHistoryRows.svelte';
  import AntiscanRiskSummary from './AntiscanRiskSummary.svelte';
  import AntiscanIPSetRows from './AntiscanIPSetRows.svelte';
  import AntiscanConfigEditor from './AntiscanConfigEditor.svelte';
  import AntiscanScheduleEditor from './AntiscanScheduleEditor.svelte';
  import {createAntiscanReadSession, mutate} from './AntiscanApi.js';
  import {mountAntiscanHostBridge} from './AntiscanHostBridge.js';
  import {fmtDuration, boolText, fmtBytes, fmtAuditTime, localizeMessage, localizeMessages, reasonLabel, evidenceSummary} from './AntiscanPresentationModel.js';
  import {readAntiscanConfigForm, writeAntiscanConfigForm, configPayloadFromSnapshot} from './AntiscanConfigModel.js';
  import {schedulerTaskLabels, managedScheduleTasks} from './AntiscanSchedulerModel.js';
  import {CONTROL_DEFAULTS} from './AntiscanControlDefaults.js';

  // State-driven migration: every displayed value/disabled flag/class is Svelte state.
  // The original Antiscan app.js is not loaded or shipped.

  // Scope presentation controls to their feature; no shared cells/virtual DOM.
  const controls = ids => Object.fromEntries(ids.map(id => [id, structuredClone(CONTROL_DEFAULTS[id])]));
  let shellControls = $state(controls(["targetState", "refresh", "notice", "actionNotice", "version", "blockedIps", "blockedSubnets", "candidateCount", "rf-generated-1", "rf-generated-2", "rf-generated-3", "rf-generated-4", "rf-generated-5", "rf-generated-6", "rf-generated-7", "rf-generated-8"]));
  let overviewControls = $state(controls(["rf-generated-9", "ipsetBinary", "runtimeSummary", "riskSummary", "lifecycleState", "lifecycleHint", "startAntiscan", "reloadAntiscan", "restartAntiscan", "stopAntiscan", "operationState", "rf-generated-10", "rf-generated-11", "rf-generated-12", "rf-generated-13", "operationOutput", "sets"]));
  let diagnosticsControls = $state(controls(["rf-generated-14", "diagnosticsState", "reloadDiagnostics", "diagnosticsMeta", "diagnosticsChecks", "rciTokenState", "reloadRCIToken", "rciTokenSummary", "rciTokenMeta", "rciTokenInput", "setRCIToken", "checkRCIToken", "deleteRCIToken", "compatibilityState", "reloadCompatibility", "compatibilityMeta", "compatibilityGrid", "compatibilityWarnings", "diagnosticsContract"]));
  let inspectorControls = $state(controls(["rf-generated-15", "inspectIp", "inspectButton", "inspectResult"]));
  let blockedControls = $state(controls(["rf-generated-16", "blockedSet", "reloadBlocked", "blockedFilter", "blockedMeta", "blockedEntries", "flushTarget", "reloadFlushPreview", "flushState", "flushMeta", "flushWarnings", "flushSelected"]));
  let listsControls = $state(controls(["rf-generated-17", "customListName", "reloadCustomList", "customListEntry", "addCustomListEntry", "reloadCustomListRuntime", "clearCustomList", "customListMeta", "customListEntries", "runtimeListsTitle", "runtimeListsDescription", "listSet", "reloadList", "listFilter", "listMeta", "listEntries"]));
  let geoControls = $state(controls(["rf-generated-18", "rf-generated-19"]));
  let protectionControls = $state(controls(["rf-generated-20", "protection", "warnings"]));
  let schedulerControls = $state(controls(["rf-generated-21", "schedulerState", "reloadScheduler", "schedulerMeta", "schedulerTaskRows", "resetScheduler", "applyScheduler"]));
  let configControls = $state(controls(["rf-generated-22", "configForm", "configState", "configHash", "configMobileRisk", "cfgIspInterfaces", "cfgPorts", "cfgForwardedPorts", "cfgEnableIpsBan", "cfgRulesMask", "cfgRecentTime", "cfgRecentHitcount", "cfgRecentLimit", "cfgRecentBanTime", "cfgCandidateStorage", "cfgDifferentThreshold", "cfgSubnetBanTime", "cfgEnableHoneypot", "cfgHoneypotPorts", "cfgHoneypotBanTime", "cfgUseCustomExclude", "cfgCustomMode", "cfgGeoMode", "cfgGeoCountries", "cfgGeoExcludeCountries", "cfgIPSetsDirectory", "cfgSaveIPSets", "cfgSaveOnExit", "cfgReadNDM", "cfgLockoutBanTime", "configHint", "resetConfig", "applyConfig"]));
  let historyControls = $state(controls(["rf-generated-23", "reloadHistory", "historyMeta", "historyEntries"]));
  // Navigation and operation lists reference controls across feature scopes by exact ID.
  function controlById(id) {
    for (const group of [shellControls,overviewControls,diagnosticsControls,inspectorControls,blockedControls,listsControls,geoControls,protectionControls,schedulerControls,configControls,historyControls]) {
      if (Object.prototype.hasOwnProperty.call(group,id)) return group[id];
    }
    throw new Error(`Unknown Antiscan control: ${id}`);
  }
  let activeTab = $state('overview');
  let hostWorkspace;
  const readSession = createAntiscanReadSession();
  const api = path => readSession.api(path);
  let schedulerRows = $state([]);
  // Native read-only UI projections: server strings render as escaped Svelte text.
  let runtimeRows = $state([]);
  let protectionRows = $state([]);
  let riskView = $state({detected:false});
  let statusWarnings = $state([]);
  let ipsetRows = $state([]);
  let diagnosticContractRows = $state([]);
  let diagnosticChecks = $state([]);
  let diagnosticChecksEmpty = $state('Диагностика ещё не запускалась.');
  let compatibilityRows = $state([]);
  let compatibilityWarnings = $state([]);
  let compatibilityError = $state('');
  let historyRows = $state([]);
  let historyRowsEmpty = $state('Откройте вкладку, чтобы загрузить последние события.');
  let blockedRows = $state([]);
  let blockedRowsEmpty = $state('Набор ещё не загружен.');
  let listRows = $state([]);
  let listRowsEmpty = $state('Выберите набор и загрузите текущее состояние.');
  let customRows = $state([]);
  let customRowsEmpty = $state('Исходный список ещё не загружен.');
  let inspectorView = $state(null);
  let inspectorEmpty = $state('Введите IPv4-адрес. Эта проверка ничего не меняет в Antiscan.');
  let flushWarningRows = $state([]);
  let schedulerError = $state('');
  let listOptions = $state([
    ['ascn_custom_exclude','Пользовательские исключения'],
    ['ascn_custom_blacklist','Пользовательский чёрный список'],
    ['ascn_custom_whitelist','Пользовательский белый список']
  ]);


  // Svelte owns the view model directly. No fake HTMLElement, Proxy or DOM-id
  // accessor: business commands update reactive fields, not browser nodes.
  function addUIClass(target, name) {
    const classes = new Set(String(target.className || '').split(/\s+/).filter(Boolean));
    classes.add(name);
    target.className = [...classes].join(' ');
  }
  function toggleUIClass(target, name, enabled) {
    const classes = new Set(String(target.className || '').split(/\s+/).filter(Boolean));
    if (enabled) classes.add(name); else classes.delete(name);
    target.className = [...classes].join(' ');
  }
  const operationControlIds = ["rf-generated-10", "rf-generated-11", "rf-generated-12", "rf-generated-13", "rf-generated-18", "rf-generated-19"];
  const tabControlIds = ["rf-generated-1", "rf-generated-2", "rf-generated-3", "rf-generated-4", "rf-generated-5", "rf-generated-6", "rf-generated-7", "rf-generated-8"];
  const pageControlIds = ["rf-generated-9", "rf-generated-14", "rf-generated-15", "rf-generated-16", "rf-generated-17", "rf-generated-20", "rf-generated-21", "rf-generated-22", "rf-generated-23"];

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
let compatibilityLoaded = $state(false);
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

function setByName(name) {
  return (snapshot?.ipsets || []).find((item) => item.name === name);
}

function countFor(name) {
  const item = setByName(name);
  if (!item?.exists || !item?.count_known) return '—';
  return String(item.count ?? 0);
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

function currentConfigFormPayload() {
  return readAntiscanConfigForm(configControls);
}
function setConfigFormPayload(payload) {
  writeAntiscanConfigForm(configControls, payload);
}

function populateConfigEditor(force = false) {
  const form = configControls['configForm'];
  if (!form) return;
  if (!snapshot?.detected || !snapshot?.config) {
    configControls['configState'].className = 'state neutral';
    configControls['configState'].textContent = 'НЕДОСТУПНО';
    configControls['configHint'].textContent = 'Antiscan или ascn.conf не обнаружен.';
    configControls['applyConfig'].disabled = true;
    configControls['resetConfig'].disabled = true;
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
  if (!configControls['configForm']) return;
  const current = currentConfigFormPayload();
  configDirty = Boolean(configBaseline) && JSON.stringify(current) !== configBaseline;
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const stale = Boolean(configDirty && configBaseSHA && snapshot?.config_sha256 && snapshot.config_sha256 !== configBaseSHA);

  const state = configControls['configState'];
  const hint = configControls['configHint'];
  state.className = 'state';
  if (configBusy) {
    addUIClass(state, 'info');
    state.textContent = 'ПРИМЕНЕНИЕ…';
    hint.textContent = 'Создаём резервную копию → атомарно записываем → перечитываем Antiscan → проверяем результат. При ошибке выполняется откат.';
  } else if (stale) {
    addUIClass(state, 'bad');
    state.textContent = 'УСТАРЕЛО';
    hint.textContent = 'ascn.conf изменился после открытия формы. Сбросьте форму к свежим данным перед применением.';
  } else if (upstreamBusy) {
    addUIClass(state, 'warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
    hint.textContent = 'Antiscan сейчас перечитывает конфигурацию или Geo-данные. Применение временно заблокировано.';
  } else if (configDirty) {
    addUIClass(state, 'warn');
    state.textContent = 'ИЗМЕНЕНО';
    hint.textContent = snapshot?.running
      ? 'Изменения будут безопасно применены через штатное перечитывание S99ascn.'
      : 'Antiscan остановлен: файл будет сохранён и проверен, а настройки вступят в силу при следующем запуске.';
  } else {
    addUIClass(state, 'good');
    state.textContent = 'СИНХРОНИЗИРОВАНО';
    hint.textContent = snapshot?.running ? 'Форма соответствует активному ascn.conf.' : 'Форма соответствует сохранённому ascn.conf.';
  }

  configControls['configHash'].textContent = configBaseSHA ? `SHA256 ${configBaseSHA.slice(0, 16)}…` : 'SHA256 —';
  configControls['applyConfig'].disabled = configBusy || !configDirty || !configBaseSHA || upstreamBusy || stale;
  configControls['resetConfig'].disabled = configBusy || !configDirty;

  const mobileRisk = current.enable_ips_ban && current.different_ip_threshold > 0 &&
    current.different_ip_threshold <= 5 && current.different_ip_candidates_storage_seconds >= 86400;
  const risk = configControls['configMobileRisk'];
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
  const state = diagnosticsControls['diagnosticsState'];
  state.className = `state ${overall[0]}`;
  state.textContent = overall[1];

  const generated = payload?.generated_at ? fmtAuditTime(payload.generated_at) : '—';
  diagnosticsControls['diagnosticsMeta'].textContent = `Проверено: ${generated} · Antiscan ${payload?.installed_version || 'не определён'} · ${payload?.running ? 'работает' : 'остановлен'}`;

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
  diagnosticContractRows = rows;

  diagnosticChecks = Array.isArray(payload?.checks) ? payload.checks : [];
  diagnosticChecksEmpty = 'Проверки не вернули данных.';
}

async function loadDiagnostics() {
  const button = diagnosticsControls['reloadDiagnostics'];
  if (button) button.disabled = true;
  diagnosticsControls['diagnosticsState'].className = 'state info';
  diagnosticsControls['diagnosticsState'].textContent = 'ПРОВЕРКА…';
  try {
    const payload = await api('diagnostics');
    diagnosticsLoaded = true;
    renderDiagnostics(payload);
  } catch (error) {
    diagnosticsLoaded = true;
    diagnosticsControls['diagnosticsState'].className = 'state bad';
    diagnosticsControls['diagnosticsState'].textContent = 'ОШИБКА';
    diagnosticsControls['diagnosticsMeta'].textContent = localizeMessage(error.message || 'Не удалось выполнить диагностику Antiscan.');
    diagnosticChecks = [];
    diagnosticChecksEmpty = 'Диагностика недоступна.';
  } finally {
    if (button) button.disabled = false;
  }
}

function renderCompatibility(payload) {
  compatibilityLoaded = true;
  compatibilityError = '';
  const state = diagnosticsControls['compatibilityState'];
  const meta = diagnosticsControls['compatibilityMeta'];
  if (!state || !meta) return;

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
  compatibilityRows = rows;
  compatibilityWarnings = Array.isArray(payload?.warnings) ? payload.warnings : [];
}

async function loadCompatibility() {
  const button = diagnosticsControls['reloadCompatibility'];
  if (button) button.disabled = true;
  diagnosticsControls['compatibilityState'].className = 'state info';
  diagnosticsControls['compatibilityState'].textContent = 'ПРОВЕРКА…';
  try {
    const payload = await api('compatibility');
    renderCompatibility(payload);
  } catch (error) {
    compatibilityLoaded = true;
    diagnosticsControls['compatibilityState'].className = 'state bad';
    diagnosticsControls['compatibilityState'].textContent = 'ОШИБКА';
    diagnosticsControls['compatibilityMeta'].textContent = localizeMessage(error.message || 'Не удалось проверить совместимость Antiscan.');
    compatibilityRows = [];
    compatibilityWarnings = [];
    compatibilityError = localizeMessage(error.message || 'Не удалось проверить совместимость Antiscan.');
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
  const state = diagnosticsControls['rciTokenState'];
  const summary = diagnosticsControls['rciTokenSummary'];
  const meta = diagnosticsControls['rciTokenMeta'];
  const setButton = diagnosticsControls['setRCIToken'];
  const checkButton = diagnosticsControls['checkRCIToken'];
  const deleteButton = diagnosticsControls['deleteRCIToken'];
  if (!state || !summary || !meta) return;

  const complete = Boolean(payload?.complete);
  const supported = Boolean(payload?.supported);
  const authState = payload?.auth_state || 'unknown';

  state.className = 'state';
  if (!supported) {
    addUIClass(state, 'neutral');
    state.textContent = 'НЕ ПОДДЕРЖИВАЕТСЯ';
    summary.textContent = 'Upstream отключил token-flow для этой версии прошивки.';
  } else if (authState === 'required' && !complete) {
    addUIClass(state, 'bad');
    state.textContent = 'НУЖЕН ТОКЕН';
    summary.textContent = 'RCI требует токен, но комплект token/key Antiscan неполный.';
  } else if (complete) {
    addUIClass(state, 'good');
    state.textContent = 'НАСТРОЕН';
    summary.textContent = authState === 'required'
      ? 'RCI требует токен; зашифрованный token/key комплект Antiscan присутствует.'
      : 'Зашифрованный token/key комплект Antiscan присутствует.';
  } else if (authState === 'not_required') {
    addUIClass(state, 'info');
    state.textContent = 'НЕ ТРЕБУЕТСЯ';
    summary.textContent = 'Upstream cache сообщает, что этой прошивке RCI-токен сейчас не требуется.';
  } else {
    addUIClass(state, 'warn');
    state.textContent = 'НЕ ОПРЕДЕЛЕНО';
    summary.textContent = 'Требование RCI ещё не закэшировано upstream.';
  }

  meta.textContent = `RCI: ${rciAuthLabel(authState)} · token ${payload?.token_present ? 'есть' : 'нет'} · key ${payload?.key_present ? 'есть' : 'нет'} · Antiscan ${payload?.running ? 'работает' : 'остановлен'}`;
  setButton.disabled = rciTokenBusy || !supported;
  checkButton.disabled = rciTokenBusy || !supported || !complete;
  deleteButton.disabled = rciTokenBusy || (!payload?.token_present && !payload?.key_present);
}

async function loadRCITokenStatus() {
  const button = diagnosticsControls['reloadRCIToken'];
  if (button) button.disabled = true;
  try {
    const payload = await api('rci-token');
    renderRCITokenStatus(payload);
  } catch (error) {
    rciTokenLoaded = true;
    diagnosticsControls['rciTokenState'].className = 'state bad';
    diagnosticsControls['rciTokenState'].textContent = 'ОШИБКА';
    diagnosticsControls['rciTokenSummary'].textContent = localizeMessage(error.message || 'Не удалось прочитать состояние RCI-токена.');
  } finally {
    if (button) button.disabled = false;
  }
}

async function performRCITokenAction(action) {
  const input = diagnosticsControls['rciTokenInput'];
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
function schedulerManagedPayload() {
  return managedScheduleTasks(schedulerRows);
}

function updateSchedulerState() {
  const apply = schedulerControls['applyScheduler'];
  const reset = schedulerControls['resetScheduler'];
  const state = schedulerControls['schedulerState'];
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
  const root = schedulerControls['schedulerTaskRows'];
  const meta = schedulerControls['schedulerMeta'];
  if (!root || !meta) return;

  const errors = payload?.errors || [];
  meta.textContent = `ascn_crontab.conf: ${payload?.task_count ?? 0} задач · active crontab: ${payload?.active_count ?? 0} · ${payload?.synced ? 'синхронизировано' : 'есть расхождение'} · SHA256 ${(payload?.source_sha256 || '—').slice(0, 16)}…`;
  if (errors.length) {
    root.className = 'scheduler-list';
    schedulerRows = [];
    schedulerError = errors.map(item => localizeMessage(item)).join(' · ');
    schedulerBaseline = '';
    schedulerDirty = false;
    updateSchedulerState();
    return;
  }

  root.className = 'scheduler-list';
  schedulerError = '';
  schedulerRows = (payload?.tasks || []).map(item=>({...item,enabled:Boolean(item.enabled),automatic:Boolean(item.automatic)}));


  if (force || !schedulerBaseline) {
    schedulerBaseline = JSON.stringify(schedulerManagedPayload());
    schedulerDirty = false;
  }
  updateSchedulerState();
}

async function loadScheduler() {
  const read = readSession.begin('schedule');
  const button = schedulerControls['reloadScheduler'];
  if (button) button.disabled = true;
  try {
    const payload = await read.api('schedule');
    if (!read.isCurrent()) return;
    schedulerBaseline = '';
    renderScheduler(payload, true);
  } catch (error) {
    if (!read.isCurrent()) return;
    schedulerLoaded = true;
    schedulerSnapshot = null;
    schedulerControls['schedulerState'].className = 'state bad';
    schedulerControls['schedulerState'].textContent = 'ОШИБКА';
    schedulerControls['schedulerMeta'].textContent = localizeMessage(error.message || 'Не удалось прочитать расписание Antiscan.');
    schedulerControls['schedulerTaskRows'].className = 'scheduler-list empty-state bad-text';
    schedulerError = 'Расписание недоступно.';
    schedulerControls['applyScheduler'].disabled = true;
    schedulerControls['resetScheduler'].disabled = true;
  } finally {
    if (read.isCurrent() && button) button.disabled = false;
    read.finish();
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
  const state = blockedControls['flushState'];
  const meta = blockedControls['flushMeta'];
  const warnings = blockedControls['flushWarnings'];
  const button = blockedControls['flushSelected'];
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
  flushWarningRows = messages;
  button.disabled = flushBusy || !payload?.allowed;
}

async function loadFlushPreview() {
  const select = blockedControls['flushTarget'];
  const reload = blockedControls['reloadFlushPreview'];
  if (!select || !reload) return;
  reload.disabled = true;
  try {
    const payload = await api(`flush-preview?target=${encodeURIComponent(select.value)}`);
    renderFlushPreview(payload);
  } catch (error) {
    flushPreviewLoaded = true;
    flushPreview = null;
    blockedControls['flushState'].className = 'state bad';
    blockedControls['flushState'].textContent = 'ОШИБКА';
    blockedControls['flushMeta'].textContent = localizeMessage(error.message || 'Не удалось построить preview очистки.');
    flushWarningRows = [];
    blockedControls['flushSelected'].disabled = true;
  } finally {
    reload.disabled = false;
  }
}

async function performFlush() {
  const target = blockedControls['flushTarget']?.value || '';
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
  blockedControls['flushSelected'].disabled = true;
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
    if (flushPreview) blockedControls['flushSelected'].disabled = !flushPreview.allowed;
  }
}

function activateTab(name) {
  activeTab=name;
  document.documentElement.dataset.section=name;
  for(const id of tabControlIds){
    const item=controlById(id);
    toggleUIClass(item, 'active', item.dataset.tab===name);
  }
  for(const id of pageControlIds){
    const item=controlById(id),pageName=item.dataset.page;
    const grouped=(name==='overview'&&pageName==='protection')||
                  (name==='blocked'&&pageName==='inspect')||
                  (name==='geo'&&pageName==='lists');
    toggleUIClass(item, 'active', pageName===name||grouped);
  }
}

function renderState() {
  const state = shellControls['targetState'];
  state.className = 'state';
  if (!snapshot?.detected) {
    addUIClass(state, 'neutral');
    state.textContent = 'НЕ ОБНАРУЖЕН';
  } else if (snapshot.running) {
    addUIClass(state, 'good');
    state.textContent = 'РАБОТАЕТ';
  } else {
    addUIClass(state, 'warn');
    state.textContent = 'ОСТАНОВЛЕН';
  }

  shellControls['version'].textContent = snapshot?.version || '—';
  shellControls['blockedIps'].textContent = countFor('ascn_ips');
  shellControls['blockedSubnets'].textContent = countFor('ascn_subnets');
  shellControls['candidateCount'].textContent = countFor('ascn_candidates');
  overviewControls['ipsetBinary'].textContent = `ipset: ${snapshot?.ipset_binary || 'не найден'}`;

  const notice = shellControls['notice'];
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
  const start = overviewControls['startAntiscan'];
  const stop = overviewControls['stopAntiscan'];
  const reload = overviewControls['reloadAntiscan'];
  const restart = overviewControls['restartAntiscan'];
  const state = overviewControls['lifecycleState'];
  const hint = overviewControls['lifecycleHint'];
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
    addUIClass(state, 'info');
    state.textContent = 'ОПЕРАЦИЯ…';
    hint.textContent = 'Ждём завершения штатной команды Antiscan и проверяем итоговое состояние.';
  } else if (operationBusy) {
    addUIClass(state, 'info');
    state.textContent = 'ОБСЛУЖИВАНИЕ…';
    hint.textContent = 'Выполняется штатная сервисная команда Antiscan.';
  } else if (!detected) {
    addUIClass(state, 'neutral');
    state.textContent = 'НЕДОСТУПНО';
    hint.textContent = 'Antiscan не обнаружен — команды управления недоступны.';
  } else if (upstreamBusy) {
    addUIClass(state, 'warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
    hint.textContent = 'Antiscan уже перечитывает конфигурацию или Geo-данные. Новое действие временно заблокировано.';
  } else if (configDirty) {
    addUIClass(state, 'warn');
    state.textContent = 'НЕ СОХРАНЕНО';
    hint.textContent = 'Сначала примените или сбросьте изменения ascn.conf. Штатные lifecycle-команды всегда работают с сохранённым конфигом.';
  } else if (running) {
    addUIClass(state, 'good');
    state.textContent = 'РАБОТАЕТ';
    hint.textContent = 'Можно перечитать ascn.conf, штатно перезапустить или остановить Antiscan.';
  } else {
    addUIClass(state, 'warn');
    state.textContent = 'ОСТАНОВЛЕН';
    hint.textContent = 'Можно штатно запустить Antiscan; restart в этом состоянии также выполнит запуск upstream-командой.';
  }

  renderOperationControls();
}

function renderOperationControls() {
  const state = overviewControls['operationState'];
  const buttons = operationControlIds.map(id => controlById(id));
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
    addUIClass(state, 'info');
    state.textContent = 'ВЫПОЛНЕНИЕ…';
  } else if (!detected) {
    addUIClass(state, 'neutral');
    state.textContent = 'НЕДОСТУПНО';
  } else if (upstreamBusy) {
    addUIClass(state, 'warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
  } else if (configDirty) {
    addUIClass(state, 'warn');
    state.textContent = 'НЕ СОХРАНЕНО';
  } else if (!running) {
    addUIClass(state, 'warn');
    state.textContent = 'ТОЛЬКО CRON';
  } else {
    addUIClass(state, 'good');
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
  runtimeRows = rows;
}

function renderRiskSummary() {
  const p = snapshot?.protection || {};
  const risk = p.ips_ban_enabled && Number(p.different_ip_threshold || 0) > 0 &&
    Number(p.different_ip_threshold || 0) <= 5 && Number(p.candidate_storage_seconds || 0) >= 86400;
  riskView = {
    detected: Boolean(snapshot?.detected), risk,
    threshold: p.different_ip_threshold || '—',
    storage: fmtDuration(p.candidate_storage_seconds),
    subnetBan: fmtDuration(p.subnet_ban_seconds),
    recentHitcount: p.recent_hitcount || '—'
  };
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
  protectionRows = rows;
}

function renderWarnings() {
  statusWarnings = localizeMessages([...(snapshot?.warnings || []), ...(snapshot?.errors || [])]);
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
  ipsetRows = sets.map(item => {
    const countUnavailable = isBenignUnknownSetCount(item);
    const hasOperationalError = Boolean(item.error) && !countUnavailable;
    return {
      name: item.name,
      description: setDescriptions[item.name] || 'Набор Antiscan',
      state: item.exists ? (hasOperationalError ? 'ЧАСТИЧНО' : 'АКТИВЕН') : 'НЕТ',
      stateClass: item.exists ? (hasOperationalError ? 'warn' : 'good') : 'neutral',
      count: item.count_known ? String(item.count ?? 0) : '—',
      countUnavailable, error: hasOperationalError ? localizeMessage(item.error) : ''
    };
  });
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
  const cls = result.blocked ? 'bad' : result.conclusive ? 'good' : 'warn';
  inspectorControls['inspectResult'].className = `inspect-result ${cls}`;
  inspectorView = {
    ip: result.ip || '', title: verdictTitle(result), stateClass: cls,
    reason: reasonLabel(result.reason || result.verdict || 'unknown'),
    explanation: reasonExplanation(result),
    evidence: (result.evidence || []).map(item => ({
      reason: reasonLabel(item.kind), set: item.set || '', summary: evidenceSummary(item)
    })),
    warnings: localizeMessages(result.warnings),
    canExclude: Boolean(result.ip && (result.blocked || result.verdict === 'candidate'))
  };
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
  const select = listsControls['listSet'];
  const title = listsControls['runtimeListsTitle'];
  const description = listsControls['runtimeListsDescription'];
  const filter = listsControls['listFilter'];
  if (!select || !title || !description || !filter) return;

  const options = runtimeListOptions(mode);
  const previousName = browserState[mode]?.name || '';
  const changedMode = select.dataset.mode !== mode;
  select.dataset.mode = mode;
  select.value = options[0][0];
  listOptions = options;
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
    listsControls['listMeta'].textContent = '—';
    listsControls['listEntries'].className = 'entry-table empty-state';
    listRows = []; listRowsEmpty = 'Выберите набор и загрузите текущее состояние.';
  }
}

function renderSetPage(kind) {
  const page = browserState[kind];
  const blocked = kind === 'blocked';
  if (!blocked && kind !== currentRuntimeListMode()) return;
  const target = blocked ? blockedControls['blockedEntries'] : listsControls['listEntries'];
  const meta = blocked ? blockedControls['blockedMeta'] : listsControls['listMeta'];
  const filter = (blocked ? blockedControls['blockedFilter'].value : listsControls['listFilter'].value).trim().toLowerCase();
  function setRows(rows, empty) {
    if (blocked) { blockedRows = rows; blockedRowsEmpty = empty; }
    else { listRows = rows; listRowsEmpty = empty; }
  }
  setRows([], 'Набор ещё не загружен.');
  if (!page) {
    target.className = 'entry-table empty-state';
    meta.textContent = '—';
    return;
  }
  if (page.error) {
    target.className = 'entry-table empty-state bad-text';
    meta.textContent = page.name || '—';
    setRows([], localizeMessage(page.error));
    return;
  }
  if (!page.exists) {
    target.className = 'entry-table empty-state';
    meta.textContent = 'НЕТ';
    setRows([], `${page.name}: набор сейчас отсутствует.`);
    return;
  }
  const all = page.entries || [];
  const entries = filter ? all.filter(entry => String(entry.value || '').toLowerCase().includes(filter)) : all;
  const total = page.count_known ? page.count : all.length;
  meta.textContent = `${page.name} · ${total} записей${page.truncated ? ` · показаны первые ${page.limit}` : ''}`;
  if (!entries.length) {
    target.className = 'entry-table empty-state';
    setRows([], filter ? 'По фильтру ничего не найдено.' : 'Набор пуст.');
    return;
  }
  target.className = 'entry-table';
  setRows(entries.map(entry => ({
    set: page.name,
    value: entry.value,
    meta: entrySecondary(entry),
    explanation: setOperationalExplanation(page.name),
    canUnban: blocked && canUnbanSet(page.name),
    canExclude: blocked && ['ascn_ips','ascn_subnets','ascn_honeypot','ascn_ndm_lockout','ascn_candidates'].includes(page.name),
    canDelete: false
  })), '');
}

async function loadSet(kind) {
  const blocked = kind === 'blocked';
  const select = blocked ? blockedControls['blockedSet'] : listsControls['listSet'];
  const button = blocked ? blockedControls['reloadBlocked'] : listsControls['reloadList'];
  const target = blocked ? blockedControls['blockedEntries'] : listsControls['listEntries'];
  const read = readSession.begin(`sets:${kind}`);
  const selected = select.value;
  button.disabled = true;
  if (blocked || kind === currentRuntimeListMode()) {
    target.className = 'entry-table empty-state';
    if (blocked) { blockedRows = []; blockedRowsEmpty = 'Читаем текущий набор ipset…'; }
    else { listRows = []; listRowsEmpty = 'Читаем текущий набор ipset…'; }
  }
  try {
    const page = await read.api(`sets?name=${encodeURIComponent(selected)}&limit=500`);
    if (!read.isCurrent() || select.value !== selected) return;
    browserState[kind] = page;
    renderSetPage(kind);
  } catch (error) {
    if (!read.isCurrent() || select.value !== selected) return;
    browserState[kind] = { name: selected, error: error.message || 'Не удалось прочитать ipset.' };
    renderSetPage(kind);
  } finally {
    if (read.isCurrent()) button.disabled = false;
    read.finish();
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
  const notice = shellControls['actionNotice'];
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
  const target = listsControls['customListEntries'];
  const meta = listsControls['customListMeta'];
  const clearButton = listsControls['clearCustomList'];
  const reloadRuntimeButton = listsControls['reloadCustomListRuntime'];
  if (!target || !meta) return;
  customRows = [];
  if (page?.error) {
    target.className = 'entry-table empty-state bad-text';
    customRowsEmpty = localizeMessage(page.error);
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
  if (reloadRuntimeButton) reloadRuntimeButton.disabled = customListBusy || !page?.configured || !page?.running;
  const entries = page?.entries || [];
  if (!entries.length) {
    target.className = 'entry-table empty-state';
    customRowsEmpty = 'В исходном файле нет корректных IPv4/CIDR записей.';
    return;
  }
  target.className = 'entry-table';
  customRows = entries.map(entry => ({
    value: entry, meta: `исходный файл ${page?.set || ''}`,
    explanation: page?.active
      ? 'Изменение будет применено штатной командой Antiscan; удаление последней записи перейдёт в upstream flush.'
      : 'Изменение затронет только исходный файл.',
    canDelete: true
  }));
}

async function loadCustomList() {
  const select = listsControls['customListName'];
  const button = listsControls['reloadCustomList'];
  if (!select || !button) return;
  const read = readSession.begin('custom-list');
  const selected = select.value;
  button.disabled = true;
  try {
    const page = await read.api(`custom-lists?list=${encodeURIComponent(selected)}`);
    if (!read.isCurrent() || selected !== select.value) return;
    renderCustomList(page);
  } catch (error) {
    if (!read.isCurrent() || selected !== select.value) return;
    renderCustomList({ list: selected, error: error.message || 'Не удалось прочитать исходный пользовательский список.' });
  } finally {
    if (read.isCurrent()) button.disabled = false;
    read.finish();
  }
}

async function performCustomListMutation(action, entry = '') {
  const list = listsControls['customListName']?.value || '';
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
    if (action === 'add') listsControls['customListEntry'].value = '';
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
    const notice = shellControls['notice'];
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
    const notice = shellControls['notice'];
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
  const output = overviewControls['operationOutput'];
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
  historyLoaded = true;
  historyControls['historyMeta'].textContent = `${events.length} событий · новые сверху${page?.skipped_invalid_lines ? ` · пропущено повреждённых строк: ${page.skipped_invalid_lines}` : ''}`;
  historyRowsEmpty = 'В журнале пока нет действий Antiscan Manager.';
  historyRows = events.map(event => ({
    failed: event.outcome !== 'success', title: auditActionTitle(event.action),
    time: fmtAuditTime(event.timestamp), summary: localizeMessage(event.summary || '—'),
    target: event.target || '', warnings: localizeMessages(event.warnings),
    duration: Number(event.duration_ms || 0), httpStatus: String(event.http_status || '—'),
    changed: Boolean(event.changed), verified: Boolean(event.verified),
    rollback: Boolean(event.rollback), restartRequired: Boolean(event.restart_required)
  }));
}

async function loadHistory() {
  const read = readSession.begin('history');
  const button = historyControls['reloadHistory'];
  if (button) button.disabled = true;
  try {
    const page = await read.api('history?limit=50');
    if (!read.isCurrent()) return;
    renderHistory(page);
  } catch (error) {
    if (!read.isCurrent()) return;
    historyLoaded = true;
    historyControls['historyMeta'].textContent = 'Не удалось прочитать историю';
    historyRows = [];
    historyRowsEmpty = localizeMessage(error.message || 'Не удалось прочитать журнал действий.');
  } finally {
    if (read.isCurrent() && button) button.disabled = false;
    read.finish();
  }
}

async function loadStatus() {
  const read = readSession.begin('status');
  shellControls['refresh'].disabled = true;
  try {
    const currentStatus = await read.api('status');
    if (!read.isCurrent()) return;
    snapshot = currentStatus;
    renderState();
    renderRuntimeSummary();
    renderRiskSummary();
    renderProtection();
    renderWarnings();
    renderSets();
    populateConfigEditor(false);
  } catch (error) {
    if (!read.isCurrent()) return;
    const notice = shellControls['notice'];
    notice.hidden = false;
    notice.className = 'notice bad';
    notice.textContent = localizeMessage(error.message || 'Не удалось получить статус Antiscan Manager.');
  } finally {
    if (read.isCurrent()) shellControls['refresh'].disabled = false;
    read.finish();
  }
}

async function inspectIP() {
  const ip = inspectorControls['inspectIp'].value.trim();
  if (!ip) return;
  inspectorControls['inspectButton'].disabled = true;
  inspectorControls['inspectResult'].className = 'inspect-result empty';
  inspectorView = null;
  inspectorEmpty = 'Проверяем текущее состояние Antiscan…';
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
    inspectorControls['inspectButton'].disabled = false;
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

  onMount(() => {
    const hostBridge = mountAntiscanHostBridge(hostWorkspace);
    void loadStatus();
    return () => {
      readSession.dispose();
      hostBridge.dispose();
    };
  });
</script>

  <main class="as-page" bind:this={hostWorkspace}>
    <header class="hero">
      <div class="hero-copy">

        <div class="hero-title-row">
          <h1>Antiscan Manager</h1>

        </div>
        <p>Диагностика и безопасное управление Antiscan: понятные причины блокировок, настройка с откатом и журнал последних действий.</p>
      </div>
      <div class="hero-actions">
        <span id="targetState" class={shellControls['targetState'].className} hidden={shellControls['targetState'].hidden}>{#if shellControls['targetState'].textContent !== undefined}{shellControls['targetState'].textContent}{:else}ПРОВЕРКА…{/if}</span>
        <button id="refresh" class={shellControls['refresh'].className} type="button" hidden={shellControls['refresh'].hidden} disabled={shellControls['refresh'].disabled} onclick={refreshAll}>{#if shellControls['refresh'].textContent !== undefined}{shellControls['refresh'].textContent}{:else}↻ Обновить{/if}</button>
      </div>
    </header>

    <section id="notice" class={shellControls['notice'].className} hidden={shellControls['notice'].hidden}>{#if shellControls['notice'].textContent !== undefined}{shellControls['notice'].textContent}{:else}{/if}</section>
    <section id="actionNotice" class={shellControls['actionNotice'].className} hidden={shellControls['actionNotice'].hidden}>{#if shellControls['actionNotice'].textContent !== undefined}{shellControls['actionNotice'].textContent}{:else}{/if}</section>

    <section class="metrics" aria-label="Antiscan summary">
      <article class="metric"><span>Версия</span><strong id="version" class={shellControls['version'].className} hidden={shellControls['version'].hidden}>{#if shellControls['version'].textContent !== undefined}{shellControls['version'].textContent}{:else}—{/if}</strong><small>оригинальный Antiscan</small></article>
      <article class="metric"><span>Прямые блокировки</span><strong id="blockedIps" class={shellControls['blockedIps'].className} hidden={shellControls['blockedIps'].hidden}>{#if shellControls['blockedIps'].textContent !== undefined}{shellControls['blockedIps'].textContent}{:else}—{/if}</strong><small>ascn_ips</small></article>
      <article class="metric"><span>Заблокировано /24</span><strong id="blockedSubnets" class={shellControls['blockedSubnets'].className} hidden={shellControls['blockedSubnets'].hidden}>{#if shellControls['blockedSubnets'].textContent !== undefined}{shellControls['blockedSubnets'].textContent}{:else}—{/if}</strong><small>ascn_subnets</small></article>
      <article class="metric"><span>Кандидаты</span><strong id="candidateCount" class={shellControls['candidateCount'].className} hidden={shellControls['candidateCount'].hidden}>{#if shellControls['candidateCount'].textContent !== undefined}{shellControls['candidateCount'].textContent}{:else}—{/if}</strong><small>ascn_candidates</small></article>
    </section>

    <nav class="tabs" aria-label="Antiscan Manager sections">
      <button class={shellControls['rf-generated-1'].className} type="button" data-tab="overview" id="rf-generated-1" hidden={shellControls['rf-generated-1'].hidden} disabled={shellControls['rf-generated-1'].disabled} onclick={() => chooseTab("overview")}>{#if shellControls['rf-generated-1'].textContent !== undefined}{shellControls['rf-generated-1'].textContent}{:else}Обзор{/if}</button>
      <button class={shellControls['rf-generated-2'].className} type="button" data-tab="diagnostics" id="rf-generated-2" hidden={shellControls['rf-generated-2'].hidden} disabled={shellControls['rf-generated-2'].disabled} onclick={() => chooseTab("diagnostics")}>{#if shellControls['rf-generated-2'].textContent !== undefined}{shellControls['rf-generated-2'].textContent}{:else}Диагностика{/if}</button>
      <button class={shellControls['rf-generated-3'].className} type="button" data-tab="blocked" id="rf-generated-3" hidden={shellControls['rf-generated-3'].hidden} disabled={shellControls['rf-generated-3'].disabled} onclick={() => chooseTab("blocked")}>{#if shellControls['rf-generated-3'].textContent !== undefined}{shellControls['rf-generated-3'].textContent}{:else}Блокировки{/if}</button>
      <button class={shellControls['rf-generated-4'].className} type="button" data-tab="lists" id="rf-generated-4" hidden={shellControls['rf-generated-4'].hidden} disabled={shellControls['rf-generated-4'].disabled} onclick={() => chooseTab("lists")}>{#if shellControls['rf-generated-4'].textContent !== undefined}{shellControls['rf-generated-4'].textContent}{:else}Списки{/if}</button>
      <button class={shellControls['rf-generated-5'].className} type="button" data-tab="geo" id="rf-generated-5" hidden={shellControls['rf-generated-5'].hidden} disabled={shellControls['rf-generated-5'].disabled} onclick={() => chooseTab("geo")}>{#if shellControls['rf-generated-5'].textContent !== undefined}{shellControls['rf-generated-5'].textContent}{:else}Geo{/if}</button>
      <button class={shellControls['rf-generated-6'].className} type="button" data-tab="schedule" id="rf-generated-6" hidden={shellControls['rf-generated-6'].hidden} disabled={shellControls['rf-generated-6'].disabled} onclick={() => chooseTab("schedule")}>{#if shellControls['rf-generated-6'].textContent !== undefined}{shellControls['rf-generated-6'].textContent}{:else}Расписание{/if}</button>
      <button class={shellControls['rf-generated-7'].className} type="button" data-tab="config" id="rf-generated-7" hidden={shellControls['rf-generated-7'].hidden} disabled={shellControls['rf-generated-7'].disabled} onclick={() => chooseTab("config")}>{#if shellControls['rf-generated-7'].textContent !== undefined}{shellControls['rf-generated-7'].textContent}{:else}Настройки{/if}</button>
      <button class={shellControls['rf-generated-8'].className} type="button" data-tab="history" id="rf-generated-8" hidden={shellControls['rf-generated-8'].hidden} disabled={shellControls['rf-generated-8'].disabled} onclick={() => chooseTab("history")}>{#if shellControls['rf-generated-8'].textContent !== undefined}{shellControls['rf-generated-8'].textContent}{:else}История{/if}</button>
    </nav>

    <section class={overviewControls['rf-generated-9'].className} data-page="overview" id="rf-generated-9" hidden={overviewControls['rf-generated-9'].hidden}>{#if overviewControls['rf-generated-9'].textContent !== undefined}{overviewControls['rf-generated-9'].textContent}{:else}
      <div class="two-column overview-grid">
        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">ТЕКУЩЕЕ СОСТОЯНИЕ</span>
              <h2>Что сейчас работает</h2>
              <p>Текущее состояние Antiscan без каких-либо изменений системы.</p>
            </div>
            <span id="ipsetBinary" class={overviewControls['ipsetBinary'].className} hidden={overviewControls['ipsetBinary'].hidden}>{#if overviewControls['ipsetBinary'].textContent !== undefined}{overviewControls['ipsetBinary'].textContent}{:else}ipset: —{/if}</span>
          </div>
          <div id="runtimeSummary" class={overviewControls['runtimeSummary'].className} hidden={overviewControls['runtimeSummary'].hidden}><AntiscanKeyValueRows rows={runtimeRows} /></div>
        </section>

        <section class="panel risk-panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">РИСК ЛОЖНЫХ БЛОКИРОВОК</span>
              <h2>Мобильные сети и /24</h2>
              <p>Проверяем сочетания настроек, которые могут случайно зацепить динамические адреса оператора.</p>
            </div>
          </div>
          <div id="riskSummary" class={overviewControls['riskSummary'].className} hidden={overviewControls['riskSummary'].hidden}><AntiscanRiskSummary view={riskView} /></div>
        </section>
      </div>

      <section class="panel lifecycle-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">УПРАВЛЕНИЕ</span>
            <h2>Управление Antiscan</h2>
            <p>Запуск, остановка и перечитывание настроек выполняются только штатным скриптом <span class="mono">S99ascn</span>. Правила Antiscan RouterForge не подменяет.</p>
          </div>
          <span id="lifecycleState" class={overviewControls['lifecycleState'].className} hidden={overviewControls['lifecycleState'].hidden}>{#if overviewControls['lifecycleState'].textContent !== undefined}{overviewControls['lifecycleState'].textContent}{:else}ПРОВЕРКА…{/if}</span>
        </div>
        <div class="lifecycle-body">
          <div class="lifecycle-copy">
            <strong>Безопасное управление</strong>
            <span id="lifecycleHint" class={overviewControls['lifecycleHint'].className} hidden={overviewControls['lifecycleHint'].hidden}>{#if overviewControls['lifecycleHint'].textContent !== undefined}{overviewControls['lifecycleHint'].textContent}{:else}Состояние загружается…{/if}</span>
          </div>
          <div class="lifecycle-actions">
            <button id="startAntiscan" class={overviewControls['startAntiscan'].className} type="button" disabled={overviewControls['startAntiscan'].disabled} hidden={overviewControls['startAntiscan'].hidden} onclick={() => performLifecycle('start')}>{#if overviewControls['startAntiscan'].textContent !== undefined}{overviewControls['startAntiscan'].textContent}{:else}▶ Запустить{/if}</button>
            <button id="reloadAntiscan" class={overviewControls['reloadAntiscan'].className} type="button" disabled={overviewControls['reloadAntiscan'].disabled} hidden={overviewControls['reloadAntiscan'].hidden} onclick={() => performLifecycle('reload')}>{#if overviewControls['reloadAntiscan'].textContent !== undefined}{overviewControls['reloadAntiscan'].textContent}{:else}↻ Перечитать{/if}</button>
            <button id="restartAntiscan" class={overviewControls['restartAntiscan'].className} type="button" disabled={overviewControls['restartAntiscan'].disabled} hidden={overviewControls['restartAntiscan'].hidden} onclick={() => performLifecycle('restart')}>{#if overviewControls['restartAntiscan'].textContent !== undefined}{overviewControls['restartAntiscan'].textContent}{:else}↻ Перезапустить{/if}</button>
            <button id="stopAntiscan" class={overviewControls['stopAntiscan'].className} type="button" disabled={overviewControls['stopAntiscan'].disabled} hidden={overviewControls['stopAntiscan'].hidden} onclick={() => performLifecycle('stop')}>{#if overviewControls['stopAntiscan'].textContent !== undefined}{overviewControls['stopAntiscan'].textContent}{:else}■ Остановить{/if}</button>
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
          <span id="operationState" class={overviewControls['operationState'].className} hidden={overviewControls['operationState'].hidden}>{#if overviewControls['operationState'].textContent !== undefined}{overviewControls['operationState'].textContent}{:else}ГОТОВО{/if}</span>
        </div>
        <div class="lifecycle-body">
          <div class="lifecycle-copy">
            <strong>Сервисные действия</strong>
            <span>Редкие операции, которые обычно нужны только при диагностике или восстановлении Antiscan.</span>
          </div>
          <div class="lifecycle-actions">
            <button class={overviewControls['rf-generated-10'].className} type="button" data-operation="update_rules" id="rf-generated-10" hidden={overviewControls['rf-generated-10'].hidden} disabled={overviewControls['rf-generated-10'].disabled} onclick={() => performOperation("read_candidates","")}>{#if overviewControls['rf-generated-10'].textContent !== undefined}{overviewControls['rf-generated-10'].textContent}{:else}Восстановить правила{/if}</button>
            <button class={overviewControls['rf-generated-11'].className} type="button" data-operation="read_candidates" id="rf-generated-11" hidden={overviewControls['rf-generated-11'].hidden} disabled={overviewControls['rf-generated-11'].disabled} onclick={() => performOperation("read_ndm_ipsets","")}>{#if overviewControls['rf-generated-11'].textContent !== undefined}{overviewControls['rf-generated-11'].textContent}{:else}Обработать кандидатов{/if}</button>
            <button class={overviewControls['rf-generated-12'].className} type="button" data-operation="read_ndm_ipsets" id="rf-generated-12" hidden={overviewControls['rf-generated-12'].hidden} disabled={overviewControls['rf-generated-12'].disabled} onclick={() => performOperation("save_ipsets","")}>{#if overviewControls['rf-generated-12'].textContent !== undefined}{overviewControls['rf-generated-12'].textContent}{:else}Импорт Keenetic{/if}</button>
            <button class={overviewControls['rf-generated-13'].className} type="button" data-operation="save_ipsets" id="rf-generated-13" hidden={overviewControls['rf-generated-13'].hidden} disabled={overviewControls['rf-generated-13'].disabled} onclick={() => performOperation("update_ipsets","geo")}>{#if overviewControls['rf-generated-13'].textContent !== undefined}{overviewControls['rf-generated-13'].textContent}{:else}Сохранить ipset{/if}</button>
          </div>
        </div>
        <div id="operationOutput" class={overviewControls['operationOutput'].className} hidden={overviewControls['operationOutput'].hidden}>{#if overviewControls['operationOutput'].textContent !== undefined}{overviewControls['operationOutput'].textContent}{:else}Последняя сервисная операция ещё не запускалась.{/if}</div>
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
            <tbody id="sets" class={overviewControls['sets'].className} hidden={overviewControls['sets'].hidden}><AntiscanIPSetRows rows={ipsetRows} /></tbody>
          </table>
        </div>
      </section>
    {/if}</section>

    <section class={diagnosticsControls['rf-generated-14'].className} data-page="diagnostics" id="rf-generated-14" hidden={diagnosticsControls['rf-generated-14'].hidden}>{#if diagnosticsControls['rf-generated-14'].textContent !== undefined}{diagnosticsControls['rf-generated-14'].textContent}{:else}
      <section class="panel diagnostics-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ГОТОВНОСТЬ ANTISCAN</span>
            <h2>Диагностика окружения</h2>
            <p>Проверяем штатные файлы, зависимости, Netfilter, hook, cron, RCI и lock-состояние без изменения настроек.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="diagnosticsState" class={diagnosticsControls['diagnosticsState'].className} hidden={diagnosticsControls['diagnosticsState'].hidden}>{#if diagnosticsControls['diagnosticsState'].textContent !== undefined}{diagnosticsControls['diagnosticsState'].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadDiagnostics" class={diagnosticsControls['reloadDiagnostics'].className} type="button" hidden={diagnosticsControls['reloadDiagnostics'].hidden} disabled={diagnosticsControls['reloadDiagnostics'].disabled} onclick={loadDiagnostics}>{#if diagnosticsControls['reloadDiagnostics'].textContent !== undefined}{diagnosticsControls['reloadDiagnostics'].textContent}{:else}↻ Проверить{/if}</button>
          </div>
        </div>
        <div id="diagnosticsMeta" class={diagnosticsControls['diagnosticsMeta'].className} hidden={diagnosticsControls['diagnosticsMeta'].hidden}>{#if diagnosticsControls['diagnosticsMeta'].textContent !== undefined}{diagnosticsControls['diagnosticsMeta'].textContent}{:else}Откройте вкладку, чтобы выполнить read-only проверку.{/if}</div>
        <div id="diagnosticsChecks" class={diagnosticsControls['diagnosticsChecks'].className.replace(/\bempty-state\b/g, '').trim()} hidden={diagnosticsControls['diagnosticsChecks'].hidden}><AntiscanDiagnosticChecks checks={diagnosticChecks} emptyText={diagnosticChecksEmpty} /></div>
      </section>

      <section class="panel rci-token-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">RCI / KEENETIC</span>
            <h2>RCI-токен</h2>
            <p>Статус token-flow Antiscan и безопасное управление штатными командами <span class="mono">token set/check/delete</span>.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="rciTokenState" class={diagnosticsControls['rciTokenState'].className} hidden={diagnosticsControls['rciTokenState'].hidden}>{#if diagnosticsControls['rciTokenState'].textContent !== undefined}{diagnosticsControls['rciTokenState'].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadRCIToken" class={diagnosticsControls['reloadRCIToken'].className} type="button" hidden={diagnosticsControls['reloadRCIToken'].hidden} disabled={diagnosticsControls['reloadRCIToken'].disabled} onclick={loadRCITokenStatus}>{#if diagnosticsControls['reloadRCIToken'].textContent !== undefined}{diagnosticsControls['reloadRCIToken'].textContent}{:else}↻ Обновить{/if}</button>
          </div>
        </div>
        <div class="rci-token-body">
          <div class="rci-token-copy">
            <strong id="rciTokenSummary" class={diagnosticsControls['rciTokenSummary'].className} hidden={diagnosticsControls['rciTokenSummary'].hidden}>{#if diagnosticsControls['rciTokenSummary'].textContent !== undefined}{diagnosticsControls['rciTokenSummary'].textContent}{:else}Состояние RCI-токена ещё не загружено.{/if}</strong>
            <span id="rciTokenMeta" class={diagnosticsControls['rciTokenMeta'].className} hidden={diagnosticsControls['rciTokenMeta'].hidden}>{#if diagnosticsControls['rciTokenMeta'].textContent !== undefined}{diagnosticsControls['rciTokenMeta'].textContent}{:else}Значение токена RouterForge не читает и не показывает.{/if}</span>
            <span class="rci-token-security">Токен передаётся upstream только через stdin. Он не попадает в argv, URL, ответ API или журнал операций.</span>
          </div>
          <div class="rci-token-controls">
            <input id="rciTokenInput" type="password" autocomplete="new-password" spellcheck="false" placeholder="Новый RCI-токен" aria-label="Новый RCI-токен" class={diagnosticsControls['rciTokenInput'].className} hidden={diagnosticsControls['rciTokenInput'].hidden} disabled={diagnosticsControls['rciTokenInput'].disabled} bind:value={diagnosticsControls['rciTokenInput'].value} onkeydown={(event) => {if(event.key === 'Enter') performRCITokenAction('set');}}>
            <button id="setRCIToken" class={diagnosticsControls['setRCIToken'].className} type="button" disabled={diagnosticsControls['setRCIToken'].disabled} hidden={diagnosticsControls['setRCIToken'].hidden} onclick={() => performRCITokenAction('set')}>{#if diagnosticsControls['setRCIToken'].textContent !== undefined}{diagnosticsControls['setRCIToken'].textContent}{:else}Сохранить токен{/if}</button>
            <button id="checkRCIToken" class={diagnosticsControls['checkRCIToken'].className} type="button" disabled={diagnosticsControls['checkRCIToken'].disabled} hidden={diagnosticsControls['checkRCIToken'].hidden} onclick={() => performRCITokenAction('check')}>{#if diagnosticsControls['checkRCIToken'].textContent !== undefined}{diagnosticsControls['checkRCIToken'].textContent}{:else}Проверить{/if}</button>
            <button id="deleteRCIToken" class={diagnosticsControls['deleteRCIToken'].className} type="button" disabled={diagnosticsControls['deleteRCIToken'].disabled} hidden={diagnosticsControls['deleteRCIToken'].hidden} onclick={() => performRCITokenAction('delete')}>{#if diagnosticsControls['deleteRCIToken'].textContent !== undefined}{diagnosticsControls['deleteRCIToken'].textContent}{:else}Удалить{/if}</button>
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
            <span id="compatibilityState" class={diagnosticsControls['compatibilityState'].className} hidden={diagnosticsControls['compatibilityState'].hidden}>{#if diagnosticsControls['compatibilityState'].textContent !== undefined}{diagnosticsControls['compatibilityState'].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
            <button id="reloadCompatibility" class={diagnosticsControls['reloadCompatibility'].className} type="button" hidden={diagnosticsControls['reloadCompatibility'].hidden} disabled={diagnosticsControls['reloadCompatibility'].disabled} onclick={loadCompatibility}>{#if diagnosticsControls['reloadCompatibility'].textContent !== undefined}{diagnosticsControls['reloadCompatibility'].textContent}{:else}↻ Перечитать{/if}</button>
          </div>
        </div>
        <div id="compatibilityMeta" class={diagnosticsControls['compatibilityMeta'].className} hidden={diagnosticsControls['compatibilityMeta'].hidden}>{#if diagnosticsControls['compatibilityMeta'].textContent !== undefined}{diagnosticsControls['compatibilityMeta'].textContent}{:else}Откройте вкладку, чтобы проверить совместимость без сетевых запросов и изменений системы.{/if}</div>
        <div id="compatibilityGrid" class={diagnosticsControls['compatibilityGrid'].className} hidden={diagnosticsControls['compatibilityGrid'].hidden}><AntiscanKeyValueRows rows={compatibilityRows} /></div>
        <div id="compatibilityWarnings" class={diagnosticsControls['compatibilityWarnings'].className} hidden={diagnosticsControls['compatibilityWarnings'].hidden}>{#if compatibilityError}<div class="empty-state bad-text">{compatibilityError}</div>{:else if compatibilityLoaded}<AntiscanWarningItems messages={compatibilityWarnings} emptyText="Версия, package metadata, Netfilter и hook согласованы с проверенным контрактом RouterForge." />{/if}</div>
        <div id="diagnosticsContract" class={diagnosticsControls['diagnosticsContract'].className} hidden={diagnosticsControls['diagnosticsContract'].hidden}><AntiscanKeyValueRows rows={diagnosticContractRows} /></div>
      </section>
    {/if}</section>
    <section class={inspectorControls['rf-generated-15'].className} data-page="inspect" id="rf-generated-15" hidden={inspectorControls['rf-generated-15'].hidden}>{#if inspectorControls['rf-generated-15'].textContent !== undefined}{inspectorControls['rf-generated-15'].textContent}{:else}
      <section class="panel inspector-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ДИАГНОСТИКА БЛОКИРОВКИ</span>
            <h2>Почему заблокирован этот IP?</h2>
            <p>Проверяем известные наборы <span class="mono">ascn_*</span> в том же порядке, в котором их учитывает Antiscan.</p>
          </div>
        </div>
        <div class="inspect-form">
          <input id="inspectIp" autocomplete="off" inputmode="decimal" spellcheck="false" placeholder="Например, 100.64.10.77" class={inspectorControls['inspectIp'].className} hidden={inspectorControls['inspectIp'].hidden} disabled={inspectorControls['inspectIp'].disabled} bind:value={inspectorControls['inspectIp'].value} onkeydown={(event) => {if(event.key === "Enter") inspectIP();}}>
          <button id="inspectButton" class={inspectorControls['inspectButton'].className} type="button" hidden={inspectorControls['inspectButton'].hidden} disabled={inspectorControls['inspectButton'].disabled} onclick={inspectIP}>{#if inspectorControls['inspectButton'].textContent !== undefined}{inspectorControls['inspectButton'].textContent}{:else}Проверить IP{/if}</button>
        </div>
        <div id="inspectResult" class={inspectorControls['inspectResult'].className} hidden={inspectorControls['inspectResult'].hidden}><AntiscanInspectorResult view={inspectorView} emptyText={inspectorEmpty} onExclude={entry => performListEntry('exclude', entry)} /></div>
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

    <section class={blockedControls['rf-generated-16'].className} data-page="blocked" id="rf-generated-16" hidden={blockedControls['rf-generated-16'].hidden}>{#if blockedControls['rf-generated-16'].textContent !== undefined}{blockedControls['rf-generated-16'].textContent}{:else}
      <section class="panel set-browser-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">ТЕКУЩИЕ БЛОКИРОВКИ</span>
            <h2>Блокировки и кандидаты</h2>
            <p>Реальные записи выбранного набора. Для блокировок Antiscan можно снять только одну конкретную запись; остальные записи не изменяются.</p>
          </div>
          <div class="browser-controls">
            <select id="blockedSet" aria-label="Выберите Antiscan set" class={blockedControls['blockedSet'].className} hidden={blockedControls['blockedSet'].hidden} disabled={blockedControls['blockedSet'].disabled} bind:value={blockedControls['blockedSet'].value} onchange={() => loadSet('blocked')}>
              <option value="ascn_ips">Прямые IP · ascn_ips</option>
              <option value="ascn_subnets">Подсети /24 · ascn_subnets</option>
              <option value="ascn_honeypot">Ловушка · ascn_honeypot</option>
              <option value="ascn_ndm_lockout">Блокировка Keenetic · ascn_ndm_lockout</option>
              <option value="ascn_candidates">Кандидаты · ascn_candidates</option>
            </select>
            <button id="reloadBlocked" class={blockedControls['reloadBlocked'].className} type="button" hidden={blockedControls['reloadBlocked'].hidden} disabled={blockedControls['reloadBlocked'].disabled} onclick={() => loadSet('blocked')}>{#if blockedControls['reloadBlocked'].textContent !== undefined}{blockedControls['reloadBlocked'].textContent}{:else}Загрузить{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="blockedFilter" autocomplete="off" spellcheck="false" placeholder="Фильтр по IP или подсети" class={blockedControls['blockedFilter'].className} hidden={blockedControls['blockedFilter'].hidden} disabled={blockedControls['blockedFilter'].disabled} bind:value={blockedControls['blockedFilter'].value} oninput={() => renderSetPage('blocked')}>
          <span id="blockedMeta" class={blockedControls['blockedMeta'].className} hidden={blockedControls['blockedMeta'].hidden}>{#if blockedControls['blockedMeta'].textContent !== undefined}{blockedControls['blockedMeta'].textContent}{:else}—{/if}</span>
        </div>
        <div id="blockedEntries" class={blockedControls['blockedEntries'].className} hidden={blockedControls['blockedEntries'].hidden}><AntiscanSetEntries entries={blockedRows} emptyText={blockedRowsEmpty} onUnban={(set, entry) => performUnban(set, entry)} onExclude={entry => performListEntry('exclude', entry)} /></div>
      </section>

      <section class="panel flush-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">ШТАТНАЯ ОЧИСТКА / ВОССТАНОВЛЕНИЕ</span>
            <h2>Очистить набор Antiscan</h2>
            <p>RouterForge вызывает только штатный <span class="mono">S99ascn flush</span>, заранее показывает затрагиваемые наборы и после операции проверяет фактическое состояние.</p>
          </div>
          <div class="browser-controls">
            <select id="flushTarget" aria-label="Выберите набор для очистки" class={blockedControls['flushTarget'].className} hidden={blockedControls['flushTarget'].hidden} disabled={blockedControls['flushTarget'].disabled} bind:value={blockedControls['flushTarget'].value} onchange={loadFlushPreview}>
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
            </select>
            <button id="reloadFlushPreview" class={blockedControls['reloadFlushPreview'].className} type="button" hidden={blockedControls['reloadFlushPreview'].hidden} disabled={blockedControls['reloadFlushPreview'].disabled} onclick={loadFlushPreview}>{#if blockedControls['reloadFlushPreview'].textContent !== undefined}{blockedControls['reloadFlushPreview'].textContent}{:else}↻ Проверить{/if}</button>
          </div>
        </div>
        <div class="flush-summary">
          <span id="flushState" class={blockedControls['flushState'].className} hidden={blockedControls['flushState'].hidden}>{#if blockedControls['flushState'].textContent !== undefined}{blockedControls['flushState'].textContent}{:else}НЕ ЗАГРУЖЕНО{/if}</span>
          <div id="flushMeta" class={blockedControls['flushMeta'].className} hidden={blockedControls['flushMeta'].hidden}>{#if blockedControls['flushMeta'].textContent !== undefined}{blockedControls['flushMeta'].textContent}{:else}Выберите цель, чтобы увидеть точные последствия штатной команды flush.{/if}</div>
        </div>
        <div id="flushWarnings" class={blockedControls['flushWarnings'].className} hidden={blockedControls['flushWarnings'].hidden}>{#if flushWarningRows.length}<AntiscanWarningItems messages={flushWarningRows} />{:else}<div class="empty-state">Дополнительных предупреждений для этой цели нет.</div>{/if}</div>
        <div class="flush-actions">
          <button id="flushSelected" class={blockedControls['flushSelected'].className} type="button" disabled={blockedControls['flushSelected'].disabled} hidden={blockedControls['flushSelected'].hidden} onclick={performFlush}>{#if blockedControls['flushSelected'].textContent !== undefined}{blockedControls['flushSelected'].textContent}{:else}Очистить выбранное{/if}</button>
          <span class="muted">Для активных whitelist с опасной комбинацией настроек операция блокируется до изменения конфигурации.</span>
        </div>
      </section>
    {/if}</section>

    <section class={listsControls['rf-generated-17'].className} data-page="lists" id="rf-generated-17" hidden={listsControls['rf-generated-17'].hidden}>{#if listsControls['rf-generated-17'].textContent !== undefined}{listsControls['rf-generated-17'].textContent}{:else}
      <section class="panel set-browser-panel custom-source-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">ПОЛЬЗОВАТЕЛЬСКИЕ СПИСКИ</span>
            <h2>Исходные списки Antiscan</h2>
            <p>Полное управление файлами blacklist, whitelist и exclude. Изменения атомарные; активный список перечитывается штатной командой Antiscan и проверяется по runtime ipset.</p>
          </div>
          <div class="browser-controls">
            <select id="customListName" aria-label="Выберите пользовательский список" class={listsControls['customListName'].className} hidden={listsControls['customListName'].hidden} disabled={listsControls['customListName'].disabled} bind:value={listsControls['customListName'].value} onchange={loadCustomList}>
              <option value="blacklist">Чёрный список</option>
              <option value="whitelist">Белый список</option>
              <option value="exclude">Исключения</option>
            </select>
            <button id="reloadCustomList" class={listsControls['reloadCustomList'].className} type="button" hidden={listsControls['reloadCustomList'].hidden} disabled={listsControls['reloadCustomList'].disabled} onclick={loadCustomList}>{#if listsControls['reloadCustomList'].textContent !== undefined}{listsControls['reloadCustomList'].textContent}{:else}Загрузить{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="customListEntry" autocomplete="off" spellcheck="false" placeholder="IPv4 или CIDR, например 100.64.10.0/24" class={listsControls['customListEntry'].className} hidden={listsControls['customListEntry'].hidden} disabled={listsControls['customListEntry'].disabled} bind:value={listsControls['customListEntry'].value} onkeydown={(event) => {if(event.key === 'Enter') performCustomListMutation('add', listsControls['customListEntry'].value.trim());}}>
          <span class="entry-actions">
            <button id="addCustomListEntry" class={listsControls['addCustomListEntry'].className} type="button" hidden={listsControls['addCustomListEntry'].hidden} disabled={listsControls['addCustomListEntry'].disabled} onclick={() => performCustomListMutation('add', listsControls['customListEntry'].value.trim())}>{#if listsControls['addCustomListEntry'].textContent !== undefined}{listsControls['addCustomListEntry'].textContent}{:else}Добавить адрес{/if}</button>
            <button id="reloadCustomListRuntime" class={listsControls['reloadCustomListRuntime'].className} type="button" hidden={listsControls['reloadCustomListRuntime'].hidden} disabled={listsControls['reloadCustomListRuntime'].disabled} onclick={() => performCustomListMutation('reload')}>{#if listsControls['reloadCustomListRuntime'].textContent !== undefined}{listsControls['reloadCustomListRuntime'].textContent}{:else}Перечитать runtime{/if}</button>
            <button id="clearCustomList" class={listsControls['clearCustomList'].className} type="button" hidden={listsControls['clearCustomList'].hidden} disabled={listsControls['clearCustomList'].disabled} onclick={() => performCustomListMutation('clear')}>{#if listsControls['clearCustomList'].textContent !== undefined}{listsControls['clearCustomList'].textContent}{:else}Очистить{/if}</button>
          </span>
        </div>
        <div id="customListMeta" class={listsControls['customListMeta'].className} hidden={listsControls['customListMeta'].hidden}>{#if listsControls['customListMeta'].textContent !== undefined}{listsControls['customListMeta'].textContent}{:else}Выберите список и загрузите его исходный файл.{/if}</div>
        <div id="customListEntries" class={listsControls['customListEntries'].className} hidden={listsControls['customListEntries'].hidden}><AntiscanSetEntries entries={customRows} emptyText={customRowsEmpty} onDelete={entry => performCustomListMutation('delete', entry)} /></div>
      </section>

      <section class="panel set-browser-panel runtime-lists-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">RUNTIME IPSET</span>
            <h2 id="runtimeListsTitle" class={listsControls['runtimeListsTitle'].className} hidden={listsControls['runtimeListsTitle'].hidden}>{#if listsControls['runtimeListsTitle'].textContent !== undefined}{listsControls['runtimeListsTitle'].textContent}{:else}Активные пользовательские списки{/if}</h2>
            <p id="runtimeListsDescription" class={listsControls['runtimeListsDescription'].className} hidden={listsControls['runtimeListsDescription'].hidden}>{#if listsControls['runtimeListsDescription'].textContent !== undefined}{listsControls['runtimeListsDescription'].textContent}{:else}Текущее содержимое runtime ipset для blacklist, whitelist и exclude.{/if}</p>
          </div>
          <div class="browser-controls">
            <select id="listSet" aria-label="Выберите list set" class={listsControls['listSet'].className} hidden={listsControls['listSet'].hidden} disabled={listsControls['listSet'].disabled} bind:value={listsControls['listSet'].value} onchange={() => loadSet(currentRuntimeListMode())}>{#each listOptions as option (option[0])}<option value={option[0]}>{option[1]}</option>{/each}</select>
            <button id="reloadList" class={listsControls['reloadList'].className} type="button" hidden={listsControls['reloadList'].hidden} disabled={listsControls['reloadList'].disabled} onclick={() => loadSet(currentRuntimeListMode())}>{#if listsControls['reloadList'].textContent !== undefined}{listsControls['reloadList'].textContent}{:else}Загрузить{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="listFilter" autocomplete="off" spellcheck="false" placeholder="Фильтр по IP или подсети" class={listsControls['listFilter'].className} hidden={listsControls['listFilter'].hidden} disabled={listsControls['listFilter'].disabled} bind:value={listsControls['listFilter'].value} oninput={() => renderSetPage(currentRuntimeListMode())}>
          <span id="listMeta" class={listsControls['listMeta'].className} hidden={listsControls['listMeta'].hidden}>{#if listsControls['listMeta'].textContent !== undefined}{listsControls['listMeta'].textContent}{:else}—{/if}</span>
        </div>
        <div id="listEntries" class={listsControls['listEntries'].className} hidden={listsControls['listEntries'].hidden}><AntiscanSetEntries entries={listRows} emptyText={listRowsEmpty} /></div>
      </section>

      <section class="panel geo-service-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">GEO</span>
            <h2>Обновление Geo-данных</h2>
            <p>Штатные операции Antiscan для загрузки Geo-наборов и повторной попытки после ошибки. RouterForge не подменяет upstream-логику.</p>
          </div>
          <div class="lifecycle-actions">
            <button class={geoControls['rf-generated-18'].className} type="button" data-operation="update_ipsets" data-scope="geo" id="rf-generated-18" hidden={geoControls['rf-generated-18'].hidden} disabled={geoControls['rf-generated-18'].disabled} onclick={() => performOperation("update_ipsets","geo")}>{#if geoControls['rf-generated-18'].textContent !== undefined}{geoControls['rf-generated-18'].textContent}{:else}Обновить Geo{/if}</button>
            <button class={geoControls['rf-generated-19'].className} type="button" data-operation="retry_load_geo" id="rf-generated-19" hidden={geoControls['rf-generated-19'].hidden} disabled={geoControls['rf-generated-19'].disabled} onclick={() => performOperation("retry_load_geo","")}>{#if geoControls['rf-generated-19'].textContent !== undefined}{geoControls['rf-generated-19'].textContent}{:else}Повторить после ошибки{/if}</button>
          </div>
        </div>
      </section>
    {/if}</section>

    <section class={protectionControls['rf-generated-20'].className} data-page="protection" id="rf-generated-20" hidden={protectionControls['rf-generated-20'].hidden}>{#if protectionControls['rf-generated-20'].textContent !== undefined}{protectionControls['rf-generated-20'].textContent}{:else}
      <div class="two-column">
        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">ЗАЩИТА</span>
              <h2>Активная политика</h2>
              <p>Значения прочитаны из <span class="mono">ascn.conf</span>.</p>
            </div>
          </div>
          <div id="protection" class={protectionControls['protection'].className} hidden={protectionControls['protection'].hidden}><AntiscanKeyValueRows rows={protectionRows} /></div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">РИСКИ</span>
              <h2>Что стоит проверить</h2>
              <p>Не приговор, а подсказки по потенциально агрессивным сочетаниям параметров.</p>
            </div>
          </div>
          <div id="warnings" class={protectionControls['warnings'].className} hidden={protectionControls['warnings'].hidden}><AntiscanWarningItems messages={statusWarnings} symbol="!" emptyText="Явных признаков повышенного риска в текущем состоянии не найдено." /></div>
        </section>
      </div>
    {/if}</section>

    <section class={schedulerControls['rf-generated-21'].className} data-page="schedule" id="rf-generated-21" hidden={schedulerControls['rf-generated-21'].hidden}>{#if schedulerControls['rf-generated-21'].textContent !== undefined}{schedulerControls['rf-generated-21'].textContent}{:else}
      <AntiscanScheduleEditor bind:controls={schedulerControls} bind:rows={schedulerRows}
        {schedulerError} {schedulerLoaded} labels={schedulerTaskLabels}
        onReload={loadScheduler} onChanged={updateSchedulerState}
        onReset={() => {if (schedulerSnapshot) renderScheduler(schedulerSnapshot, true);}}
        onApply={applyScheduler} />
    {/if}</section>

    <section class={configControls['rf-generated-22'].className} data-page="config" id="rf-generated-22" hidden={configControls['rf-generated-22'].hidden}>{#if configControls['rf-generated-22'].textContent !== undefined}{configControls['rf-generated-22'].textContent}{:else}
      <AntiscanConfigEditor bind:controls={configControls}
        onChanged={updateConfigEditorState}
        onReset={() => {configDirty = false; populateConfigEditor(true);}}
        onApply={applyConfigEditor} />
    {/if}</section>

    <section class={historyControls['rf-generated-23'].className} data-page="history" id="rf-generated-23" hidden={historyControls['rf-generated-23'].hidden}>{#if historyControls['rf-generated-23'].textContent !== undefined}{historyControls['rf-generated-23'].textContent}{:else}
      <section class="panel history-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">ИСТОРИЯ ОПЕРАЦИЙ</span>
            <h2>Последние действия</h2>
            <p>Ограниченный журнал последних действий: что было сделано, результат, длительность, откат и необходимость перезапуска. Значения формы ascn.conf в журнал не записываются.</p>
          </div>
          <button id="reloadHistory" class={historyControls['reloadHistory'].className} type="button" hidden={historyControls['reloadHistory'].hidden} disabled={historyControls['reloadHistory'].disabled} onclick={loadHistory}>{#if historyControls['reloadHistory'].textContent !== undefined}{historyControls['reloadHistory'].textContent}{:else}↻ Обновить историю{/if}</button>
        </div>
        <div class="history-toolbar">
          <span id="historyMeta" class={historyControls['historyMeta'].className} hidden={historyControls['historyMeta'].hidden}>{#if historyControls['historyMeta'].textContent !== undefined}{historyControls['historyMeta'].textContent}{:else}История ещё не загружена.{/if}</span>
          <span class="state good">JSONL · 0600 · ограниченный размер</span>
        </div>
        <div id="historyEntries" class={historyControls['historyEntries'].className} hidden={historyControls['historyEntries'].hidden}><AntiscanHistoryRows entries={historyRows} emptyText={historyRowsEmpty} /></div>
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
