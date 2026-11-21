<script>
  import { rfUi } from './rf-localization.js';
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
  let diagnosticChecksEmpty = $state(rfUi('Диагностика ещё не запускалась.',"Diagnostics have not been run yet."));
  let compatibilityRows = $state([]);
  let compatibilityWarnings = $state([]);
  let compatibilityError = $state('');
  let historyRows = $state([]);
  let historyRowsEmpty = $state(rfUi('Откройте вкладку, чтобы загрузить последние события.',"Open the tab to load recent events."));
  let blockedRows = $state([]);
  let blockedRowsEmpty = $state(rfUi('Набор ещё не загружен.',"Set not loaded yet."));
  let listRows = $state([]);
  let listRowsEmpty = $state(rfUi('Выберите набор и загрузите текущее состояние.',"Select a set and load its current status."));
  let customRows = $state([]);
  let customRowsEmpty = $state(rfUi('Исходный список ещё не загружен.',"Source list not loaded yet."));
  let inspectorView = $state(null);
  let inspectorEmpty = $state(rfUi('Введите IPv4-адрес. Эта проверка ничего не меняет в Antiscan.',"Enter an IPv4 address. This check does not change Antiscan."));
  let flushWarningRows = $state([]);
  let schedulerError = $state('');
  let listOptions = $state([
    ['ascn_custom_exclude',rfUi('Пользовательские исключения',"User exclusions")],
    ['ascn_custom_blacklist',rfUi('Пользовательский чёрный список',"User blocklist")],
    ['ascn_custom_whitelist',rfUi('Пользовательский белый список',"User allowlist")]
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
  ascn_candidates: rfUi('Кандидаты для распределённой /24-защиты',"Candidates for distributed /24 protection"),
  ascn_ips: rfUi('Прямые IP-блокировки',"Direct IP blocks"),
  ascn_subnets: rfUi('Заблокированные подсети /24',"Blocked /24 subnets"),
  ascn_custom_exclude: rfUi('Пользовательские исключения',"User exclusions"),
  ascn_custom_blacklist: rfUi('Пользовательский чёрный список',"User blocklist"),
  ascn_custom_whitelist: rfUi('Пользовательский белый список',"User allowlist"),
  ascn_geo_blacklist: rfUi('Geo-чёрный список',"Geo blocklist"),
  ascn_geo_whitelist: rfUi('Geo-белый список',"Geo allowlist"),
  ascn_geo_exclude: rfUi('Geo-исключения',"Geo exclusions"),
  ascn_ndm_lockout: rfUi('Блокировки Keenetic',"Keenetic lockouts"),
  ascn_honeypot: rfUi('Ловушка',"Honeypot")
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
    ascn_candidates: rfUi('Кандидат сам по себе не блокирует IP; несколько адресов одной /24 могут позднее привести к блокировке всей подсети.',"A candidate does not block an IP itself; several addresses within a /24 can later trigger a whole-subnet block."),
    ascn_ips: rfUi('Прямая блокировка IP. Antiscan не сохраняет точную причину срабатывания внутри этого набора.',"Direct IP block. Antiscan does not save the exact trigger reason in this set."),
    ascn_subnets: rfUi('Блокируется вся /24 после накопления разных IP-кандидатов. Это особенно важно для мобильных и динамических пулов.',"The entire /24 is blocked after enough distinct IP candidates accumulate. This matters especially for mobile and dynamic address pools."),
    ascn_honeypot: rfUi('IP заблокирован после обращения к одному из портов ловушки HONEYPOT_PORTS.',"IP was blocked after accessing one of the HONEYPOT_PORTS decoy ports."),
    ascn_ndm_lockout: rfUi('Запись импортирована из политики блокировок Keenetic; прямое снятие такой блокировки здесь намеренно отключено.',"Entry imported from the Keenetic lockout policy; direct removal here is intentionally disabled."),
    ascn_custom_exclude: rfUi('Пользовательское исключение имеет приоритет перед блокирующими правилами.',"User exclusion takes priority over blocking rules."),
    ascn_custom_blacklist: rfUi('Активный пользовательский чёрный список при CUSTOM_LISTS_BLOCK_MODE=blacklist.',"Active user blocklist when CUSTOM_LISTS_BLOCK_MODE=blacklist."),
    ascn_custom_whitelist: rfUi('В режиме whitelist отсутствие адреса в этом наборе означает блокировку.',"In allowlist mode, an address absent from this set is blocked."),
    ascn_geo_blacklist: rfUi('Подсети стран из активного Geo-чёрного списка.',"Country subnets in the active Geo blocklist."),
    ascn_geo_whitelist: rfUi('В режиме Geo whitelist отсутствие подсети в разрешённом наборе означает блокировку.',"In Geo allowlist mode, a subnet absent from the allowed set is blocked."),
    ascn_geo_exclude: rfUi('Geo-исключение имеет приоритет перед блокирующими правилами.',"Geo exclusion takes priority over blocking rules.")
  }[name] || rfUi('Запись активного набора Antiscan.',"Entry in an active Antiscan set.");
}

function reasonExplanation(result) {
  const descriptions = {
    'custom-exclude': rfUi('IP найден в пользовательских исключениях. Antiscan пропускает такой адрес до проверки блокирующих правил.',"IP was found in user exclusions. Antiscan skips it before checking blocking rules."),
    'geo-exclude': rfUi('IP попал в Geo-исключение и не должен блокироваться последующими правилами.',"IP matched a Geo exclusion and should not be blocked by later rules."),
    'custom-blacklist': rfUi('IP найден в активном пользовательском чёрном списке.',"IP was found in the active user blocklist."),
    'custom-whitelist-miss': rfUi('Включён режим белого списка, но IP отсутствует среди разрешённых адресов.',"Allowlist mode is enabled, but the IP is not among allowed addresses."),
    'geo-blacklist': rfUi('IP относится к подсети страны из активного Geo-чёрного списка.',"IP belongs to a country subnet in the active Geo blocklist."),
    'geo-whitelist-miss': rfUi('Включён Geo-белый список, но IP не относится к разрешённым подсетям.',"Geo allowlist is enabled, but the IP does not belong to allowed subnets."),
    'ndm-lockout': rfUi('IP импортирован из политики блокировок Keenetic.',"IP was imported from the Keenetic lockout policy."),
    honeypot: rfUi('IP находится в наборе блокировок ловушки после обращения к порту-приманке.',"IP is in the honeypot block set after contacting a decoy port."),
    'distributed-subnet': rfUi('Заблокирована вся /24: Antiscan накопил заданное количество разных IP-кандидатов из одной подсети.',"Entire /24 blocked: Antiscan accumulated the configured number of distinct IP candidates from one subnet."),
    'direct-ip': rfUi('IP находится в прямом наборе блокировок. Antiscan не сохраняет, какой именно механизм был точным триггером.',"IP is in the direct block set. Antiscan does not record which mechanism triggered it."),
    'candidate-only': rfUi('IP пока только кандидат для анализа /24 и этим набором сам по себе не блокируется.',"IP is currently only a /24 analysis candidate and is not blocked by this set alone."),
    'no-active-set-match': rfUi('Совпадений с активными блокирующими наборами не найдено.',"No matches found in active blocking sets.")
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
    configControls['configState'].textContent = rfUi('НЕДОСТУПНО',"UNAVAILABLE");
    configControls['configHint'].textContent = rfUi('Antiscan или ascn.conf не обнаружен.',"Antiscan or ascn.conf was not found.");
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
    state.textContent = rfUi('ПРИМЕНЕНИЕ…',"APPLYING…");
    hint.textContent = rfUi('Создаём резервную копию → атомарно записываем → перечитываем Antiscan → проверяем результат. При ошибке выполняется откат.',"Backing up → writing atomically → reloading Antiscan → verifying. Failures trigger rollback.");
  } else if (stale) {
    addUIClass(state, 'bad');
    state.textContent = rfUi('УСТАРЕЛО',"OUTDATED");
    hint.textContent = rfUi('ascn.conf изменился после открытия формы. Сбросьте форму к свежим данным перед применением.',"ascn.conf changed after the editor was opened. Reload the latest settings before applying.");
  } else if (upstreamBusy) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ANTISCAN ЗАНЯТ',"ANTISCAN BUSY");
    hint.textContent = rfUi('Antiscan сейчас перечитывает конфигурацию или Geo-данные. Применение временно заблокировано.',"Antiscan is reloading configuration or Geo data. Applying changes is temporarily blocked.");
  } else if (configDirty) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ИЗМЕНЕНО',"MODIFIED");
    hint.textContent = snapshot?.running
      ? rfUi('Изменения будут безопасно применены через штатное перечитывание S99ascn.',"Changes will be applied safely through the standard S99ascn reload.")
      : rfUi('Antiscan остановлен: файл будет сохранён и проверен, а настройки вступят в силу при следующем запуске.',"Antiscan is stopped: the file will be saved and verified; settings take effect on next start.");
  } else {
    addUIClass(state, 'good');
    state.textContent = rfUi('СИНХРОНИЗИРОВАНО',"SYNCHRONIZED");
    hint.textContent = snapshot?.running ? rfUi('Форма соответствует активному ascn.conf.',"The form matches the active ascn.conf.") : rfUi('Форма соответствует сохранённому ascn.conf.',"The form matches the saved ascn.conf.");
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
    ? rfUi('Будет создана резервная копия, ascn.conf запишется атомарно, затем Antiscan штатно перечитает настройки. При ошибке RouterForge восстановит предыдущий файл и рабочее состояние.',"A backup will be created, ascn.conf written atomically and Antiscan reloaded normally. On failure, RouterForge will restore the previous file and running state.")
    : rfUi('Antiscan остановлен. Будет создана резервная копия и атомарно сохранён ascn.conf; настройки вступят в силу при следующем запуске.',"Antiscan is stopped. A backup will be created and ascn.conf saved atomically; settings take effect at the next start.");
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
    pass: ['good', rfUi('ГОТОВО',"READY")],
    warn: ['warn', rfUi('ВНИМАНИЕ',"WARNING")],
    fail: ['bad', rfUi('ОШИБКА',"ERROR")],
    info: ['info', rfUi('ИНФО',"INFO")]
  }[state] || ['neutral', rfUi('НЕИЗВЕСТНО',"UNKNOWN")];
}

function renderDiagnostics(payload) {
  const overall = diagnosticStateView(payload?.overall || 'info');
  const state = diagnosticsControls['diagnosticsState'];
  state.className = `state ${overall[0]}`;
  state.textContent = overall[1];

  const generated = payload?.generated_at ? fmtAuditTime(payload.generated_at) : '—';
  diagnosticsControls['diagnosticsMeta'].textContent = rfUi(`Проверено: ${generated} · Antiscan ${payload?.installed_version || 'не определён'} · ${payload?.running ? 'работает' : 'остановлен'}`,`Checked: ${generated} · Antiscan ${payload?.installed_version || 'unknown'} · ${payload?.running ? 'running' : 'stopped'}`);

  const contract = payload?.contract || {};
  const rows = [
    ['Upstream', contract.repository || '—'],
    [rfUi('Зафиксированная версия',"Pinned version"), contract.version || '—'],
    ['Upstream SHA', contract.pinned_sha || '—'],
    [rfUi('Параметры ascn.conf',"ascn.conf settings"), `${(contract.config_keys || []).length} / 25`],
    [rfUi('Известные ipset',"Known ipset sets"), `${(contract.ipsets || []).length} / 11`],
    [rfUi('CLI-команды',"CLI commands"), String((contract.commands || []).length)],
    [rfUi('Разрешённые cron-задачи',"Allowed cron tasks"), (contract.valid_tasks || []).join(' · ') || '—']
  ];
  diagnosticContractRows = rows;

  diagnosticChecks = Array.isArray(payload?.checks) ? payload.checks : [];
  diagnosticChecksEmpty = rfUi('Проверки не вернули данных.',"Checks returned no data.");
}

async function loadDiagnostics() {
  const button = diagnosticsControls['reloadDiagnostics'];
  if (button) button.disabled = true;
  diagnosticsControls['diagnosticsState'].className = 'state info';
  diagnosticsControls['diagnosticsState'].textContent = rfUi('ПРОВЕРКА…',"CHECKING…");
  try {
    const payload = await api('diagnostics');
    diagnosticsLoaded = true;
    renderDiagnostics(payload);
  } catch (error) {
    diagnosticsLoaded = true;
    diagnosticsControls['diagnosticsState'].className = 'state bad';
    diagnosticsControls['diagnosticsState'].textContent = rfUi('ОШИБКА',"ERROR");
    diagnosticsControls['diagnosticsMeta'].textContent = localizeMessage(error.message || rfUi('Не удалось выполнить диагностику Antiscan.',"Could not run Antiscan diagnostics."));
    diagnosticChecks = [];
    diagnosticChecksEmpty = rfUi('Диагностика недоступна.',"Diagnostics unavailable.");
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
  meta.textContent = payload?.summary || rfUi('Состояние совместимости не определено.',"Compatibility status is unknown.");

  const update = payload?.update || {};
  const firmware = payload?.firmware || {};
  const hook = payload?.hook || {};
  const packageVersion = payload?.package_version || rfUi('не найден',"not found");
  const packageMatch = payload?.package_version
    ? (payload?.package_runtime_match ? rfUi('совпадает',"matching") : rfUi('расходится',"mismatch"))
    : rfUi('не проверено',"not checked");
  const updateValue = update.available
    ? `${update.available_version || '—'} · ${update.available_channel || 'upstream'}`
    : (update.eligibility_known ? rfUi('по текущему upstream cache нет',"not in current upstream cache") : rfUi('не определено',"not determined"));
  const cacheValue = !update.cache_present
    ? rfUi('нет',"no")
    : `${update.cache_fresh ? rfUi('свежий',"fresh") : rfUi('устарел',"outdated")} · ${Number(update.cache_age_seconds || 0)} сек.`;
  const netfilterValue = !firmware.netfilter_known
    ? rfUi('не определено',"not determined")
    : (firmware.netfilter_present ? rfUi('есть',"present") : 'не найден');
  const hookValue = hook.present
    ? (hook.executable ? rfUi('установлен · исполняем',"installed · executable") : rfUi('установлен · не исполняем',"installed · not executable"))
    : rfUi('не найден',"not found");

  const rows = [
    ['Runtime S99ascn', payload?.script_version || '—'],
    ['OPKG Antiscan', packageVersion],
    [rfUi('Пакет / runtime',"Package / service status"), packageMatch],
    [rfUi('Проверенный RouterForge',"Verified RouterForge"), payload?.pinned_version || '—'],
    ['Pinned contract', payload?.contract_exact ? rfUi('точное совпадение',"exact match") : rfUi('не совпадает',"mismatch")],
    [rfUi('Доступное обновление',"Available update"), updateValue],
    ['Upstream update cache', cacheValue],
    [rfUi('Прошивка Keenetic',"Keenetic firmware"), firmware.version || rfUi('не определена',"unknown")],
    ['Netfilter', netfilterValue],
    ['Netfilter hook', hookValue],
    [rfUi('Кто обновляет Antiscan',"Antiscan update source"), payload?.update_owner === 'upstream-opkg' ? rfUi('официальный upstream OPKG',"official upstream OPKG") : (payload?.update_owner || '—')]
  ];
  compatibilityRows = rows;
  compatibilityWarnings = Array.isArray(payload?.warnings) ? payload.warnings : [];
}

async function loadCompatibility() {
  const button = diagnosticsControls['reloadCompatibility'];
  if (button) button.disabled = true;
  diagnosticsControls['compatibilityState'].className = 'state info';
  diagnosticsControls['compatibilityState'].textContent = rfUi('ПРОВЕРКА…',"CHECKING…");
  try {
    const payload = await api('compatibility');
    renderCompatibility(payload);
  } catch (error) {
    compatibilityLoaded = true;
    diagnosticsControls['compatibilityState'].className = 'state bad';
    diagnosticsControls['compatibilityState'].textContent = rfUi('ОШИБКА',"ERROR");
    diagnosticsControls['compatibilityMeta'].textContent = localizeMessage(error.message || rfUi('Не удалось проверить совместимость Antiscan.',"Could not check Antiscan compatibility."));
    compatibilityRows = [];
    compatibilityWarnings = [];
    compatibilityError = localizeMessage(error.message || rfUi('Не удалось проверить совместимость Antiscan.',"Could not check Antiscan compatibility."));
  } finally {
    if (button) button.disabled = false;
  }
}

function rciAuthLabel(state) {
  return {
    required: rfUi('требуется',"required"),
    not_required: rfUi('не требуется',"not required"),
    unknown: rfUi('ещё не определено',"not determined yet")
  }[state] || rfUi('неизвестно',"unknown");
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
    state.textContent = rfUi('НЕ ПОДДЕРЖИВАЕТСЯ',"UNSUPPORTED");
    summary.textContent = rfUi('Upstream отключил token-flow для этой версии прошивки.',"Upstream does not support token-based authentication on this firmware version.");
  } else if (authState === 'required' && !complete) {
    addUIClass(state, 'bad');
    state.textContent = rfUi('НУЖЕН ТОКЕН',"TOKEN REQUIRED");
    summary.textContent = rfUi('RCI требует токен, но комплект token/key Antiscan неполный.',"RCI requires a token, but the Antiscan token/key pair is incomplete.");
  } else if (complete) {
    addUIClass(state, 'good');
    state.textContent = rfUi('НАСТРОЕН',"CONFIGURED");
    summary.textContent = authState === 'required'
      ? rfUi('RCI требует токен; зашифрованный token/key комплект Antiscan присутствует.',"RCI requires a token; the encrypted Antiscan token/key pair is present.")
      : rfUi('Зашифрованный token/key комплект Antiscan присутствует.',"Encrypted Antiscan token/key pair is present.");
  } else if (authState === 'not_required') {
    addUIClass(state, 'info');
    state.textContent = rfUi('НЕ ТРЕБУЕТСЯ',"NOT REQUIRED");
    summary.textContent = rfUi('Upstream cache сообщает, что этой прошивке RCI-токен сейчас не требуется.',"The upstream cache reports that this firmware does not currently require an RCI token.");
  } else {
    addUIClass(state, 'warn');
    state.textContent = rfUi('НЕ ОПРЕДЕЛЕНО',"UNKNOWN");
    summary.textContent = rfUi('Требование RCI ещё не закэшировано upstream.',"The upstream cache has not yet resolved whether RCI authentication is required.");
  }

  meta.textContent = rfUi(`RCI: ${rciAuthLabel(authState)} · token ${payload?.token_present ? 'есть' : 'нет'} · key ${payload?.key_present ? 'есть' : 'нет'} · Antiscan ${payload?.running ? 'работает' : 'остановлен'}`,`RCI: ${rciAuthLabel(authState)} · token ${payload?.token_present ? 'present' : 'missing'} · key ${payload?.key_present ? 'present' : 'missing'} · Antiscan ${payload?.running ? 'running' : 'stopped'}`);
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
    diagnosticsControls['rciTokenState'].textContent = rfUi('ОШИБКА',"ERROR");
    diagnosticsControls['rciTokenSummary'].textContent = localizeMessage(error.message || rfUi('Не удалось прочитать состояние RCI-токена.',"Could not read the RCI token status."));
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
    prompt = rfUi('Сохранить новый RCI-токен?\n\nТокен будет передан штатной команде Antiscan через stdin, проверен upstream и не попадёт в argv или журнал.',"Save the new RCI token?\\n\\nThe token will be passed to the standard Antiscan command through stdin, checked upstream and never written to argv or logs.");
  } else if (action === 'check') {
    confirmValue = 'CHECK_TOKEN';
    prompt = rfUi('Проверить сохранённый RCI-токен через штатную команду Antiscan?',"Check the saved RCI token using the standard Antiscan command?");
  } else if (action === 'delete') {
    confirmValue = 'DELETE_TOKEN';
    prompt = rfUi('Удалить RCI-токен Antiscan?\n\nЕсли прошивка требует RCI-аутентификацию, upstream может штатно остановить Antiscan после удаления.',"Delete the Antiscan RCI token?\\n\\nIf firmware requires RCI authentication, upstream Antiscan may stop after deletion.");
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
    state.textContent = rfUi('ОШИБКА ФАЙЛА',"FILE ERROR");
  } else if (schedulerBusy) {
    state.className = 'state info';
    state.textContent = rfUi('ПРИМЕНЕНИЕ…',"APPLYING…");
  } else if (schedulerDirty) {
    state.className = 'state warn';
    state.textContent = rfUi('ИЗМЕНЕНО',"MODIFIED");
  } else if (!schedulerSnapshot.synced) {
    state.className = 'state warn';
    state.textContent = rfUi('НЕ СИНХРОНИЗИРОВАНО',"NOT SYNCHRONIZED");
  } else {
    state.className = 'state good';
    state.textContent = rfUi('СИНХРОНИЗИРОВАНО',"SYNCHRONIZED");
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
  meta.textContent = rfUi(`ascn_crontab.conf: ${payload?.task_count ?? 0} задач · active crontab: ${payload?.active_count ?? 0} · ${payload?.synced ? 'синхронизировано' : 'есть расхождение'} · SHA256 ${(payload?.source_sha256 || '—').slice(0, 16)}…`,`ascn_crontab.conf: ${payload?.task_count ?? 0} tasks · active crontab: ${payload?.active_count ?? 0} · ${payload?.synced ? 'synchronized' : 'mismatch'} · SHA256 ${(payload?.source_sha256 || '—').slice(0, 16)}…`);
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
    schedulerControls['schedulerState'].textContent = rfUi('ОШИБКА',"ERROR");
    schedulerControls['schedulerMeta'].textContent = localizeMessage(error.message || rfUi('Не удалось прочитать расписание Antiscan.',"Could not read the Antiscan schedule."));
    schedulerControls['schedulerTaskRows'].className = 'scheduler-list empty-state bad-text';
    schedulerError = rfUi('Расписание недоступно.',"Schedule unavailable.");
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
  if (!window.confirm(rfUi('Применить расписание Antiscan?\n\nRouterForge атомарно обновит ascn_crontab.conf, вызовет штатный update_crontab и проверит активный crontab. При ошибке будет восстановлено предыдущее состояние.',"Apply the Antiscan schedule?\\n\\nRouterForge will atomically update ascn_crontab.conf, run the standard update_crontab and verify active crontab. Previous state will be restored on failure."))) return;

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
  candidates: rfUi('Кандидаты',"Candidates"),
  ips: rfUi('Прямые IP',"Direct IPs"),
  subnets: rfUi('Подсети /24',"/24 subnets"),
  custom_whitelist: rfUi('Пользовательский белый список',"User allowlist"),
  custom_blacklist: rfUi('Пользовательский чёрный список',"User blocklist"),
  custom_exclude: rfUi('Пользовательские исключения',"User exclusions"),
  geo: rfUi('Все Geo-наборы',"All Geo sets"),
  ndm_lockout: rfUi('Блокировки Keenetic',"Keenetic lockouts"),
  honeypot: rfUi('Ловушка',"Honeypot"),
  all: rfUi('Upstream «все»',"Upstream «all»")
};

function flushTargetLabel(target) {
  return flushTargetLabels[target] || target || rfUi('Наборы Antiscan',"Antiscan sets");
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
  state.textContent = payload?.allowed ? rfUi('ГОТОВО К ОЧИСТКЕ',"READY TO CLEAR") : rfUi('ЗАБЛОКИРОВАНО',"BLOCKED");
  const sets = (payload?.affected_sets || []).map((item) => {
    const count = item.count_known ? Number(item.count || 0) : '—';
    return `${item.name}: ${count}`;
  });
  meta.textContent = rfUi(`${flushTargetLabel(payload?.target)} · записей сейчас ${Number(payload?.before_entries || 0)} · ${sets.join(' · ') || 'наборы не найдены'}`,`${flushTargetLabel(payload?.target)} · current entries ${Number(payload?.before_entries || 0)} · ${sets.join(' · ') || 'no sets found'}`);

  const messages = [...localizeMessages(payload?.warnings || [])];
  if (payload?.destructive_file) messages.unshift(rfUi('Операция затрагивает сохранённые файлы upstream, а не только runtime.',"This operation affects saved upstream files, not only the running state."));
  if (payload?.restart_required) messages.push(rfUi('После этой очистки upstream требует восстановить данные и перезапустить Antiscan перед возвратом соответствующей защиты.',"After this cleanup, upstream requires data restoration and an Antiscan restart before re-enabling the related protection."));
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
    blockedControls['flushState'].textContent = rfUi('ОШИБКА',"ERROR");
    blockedControls['flushMeta'].textContent = localizeMessage(error.message || rfUi('Не удалось построить preview очистки.',"Could not prepare the clear-operation preview."));
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
    flushPreview.destructive_file ? rfUi('Будут изменены/удалены сохранённые файлы upstream.',"Saved upstream files will be changed or deleted.") : '',
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
    state.textContent = rfUi('НЕ ОБНАРУЖЕН',"NOT DETECTED");
  } else if (snapshot.running) {
    addUIClass(state, 'good');
    state.textContent = rfUi('РАБОТАЕТ',"RUNNING");
  } else {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ОСТАНОВЛЕН',"STOPPED");
  }

  shellControls['version'].textContent = snapshot?.version || '—';
  shellControls['blockedIps'].textContent = countFor('ascn_ips');
  shellControls['blockedSubnets'].textContent = countFor('ascn_subnets');
  shellControls['candidateCount'].textContent = countFor('ascn_candidates');
  overviewControls['ipsetBinary'].textContent = rfUi(`ipset: ${snapshot?.ipset_binary || 'не найден'}`,`ipset: ${snapshot?.ipset_binary || 'not found'}`);

  const notice = shellControls['notice'];
  if (!snapshot?.detected) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = rfUi('Antiscan не обнаружен. RouterForge не устанавливает пакет Antiscan автоматически.',"Antiscan was not found. RouterForge does not install Antiscan automatically.");
  } else if (!snapshot.running) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = rfUi('Antiscan обнаружен, но /tmp/ascn.run отсутствует. Текущее состояние блокировок нельзя считать достоверным.',"Antiscan was found, but /tmp/ascn.run is missing. The current blocking status cannot be verified.");
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
    state.textContent = rfUi('ОПЕРАЦИЯ…',"RUNNING OPERATION…");
    hint.textContent = rfUi('Ждём завершения штатной команды Antiscan и проверяем итоговое состояние.',"Waiting for the Antiscan command to finish and verifying its result.");
  } else if (operationBusy) {
    addUIClass(state, 'info');
    state.textContent = rfUi('ОБСЛУЖИВАНИЕ…',"MAINTENANCE…");
    hint.textContent = rfUi('Выполняется штатная сервисная команда Antiscan.',"Running an Antiscan service command.");
  } else if (!detected) {
    addUIClass(state, 'neutral');
    state.textContent = rfUi('НЕДОСТУПНО',"UNAVAILABLE");
    hint.textContent = rfUi('Antiscan не обнаружен — команды управления недоступны.',"Antiscan is not installed; management actions are unavailable.");
  } else if (upstreamBusy) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ANTISCAN ЗАНЯТ',"ANTISCAN BUSY");
    hint.textContent = rfUi('Antiscan уже перечитывает конфигурацию или Geo-данные. Новое действие временно заблокировано.',"Antiscan is already reloading configuration or Geo data. New actions are temporarily blocked.");
  } else if (configDirty) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('НЕ СОХРАНЕНО',"NOT SAVED");
    hint.textContent = rfUi('Сначала примените или сбросьте изменения ascn.conf. Штатные lifecycle-команды всегда работают с сохранённым конфигом.',"First apply or discard the ascn.conf changes. Native service commands always use the saved configuration.");
  } else if (running) {
    addUIClass(state, 'good');
    state.textContent = rfUi('РАБОТАЕТ',"RUNNING");
    hint.textContent = rfUi('Можно перечитать ascn.conf, штатно перезапустить или остановить Antiscan.',"You can reload ascn.conf, restart, or stop Antiscan using its native commands.");
  } else {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ОСТАНОВЛЕН',"STOPPED");
    hint.textContent = rfUi('Можно штатно запустить Antiscan; restart в этом состоянии также выполнит запуск upstream-командой.',"You can start Antiscan using its native command; Restart will also start a stopped instance.");
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
    state.textContent = rfUi('ВЫПОЛНЕНИЕ…',"RUNNING…");
  } else if (!detected) {
    addUIClass(state, 'neutral');
    state.textContent = rfUi('НЕДОСТУПНО',"UNAVAILABLE");
  } else if (upstreamBusy) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ANTISCAN ЗАНЯТ',"ANTISCAN BUSY");
  } else if (configDirty) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('НЕ СОХРАНЕНО',"NOT SAVED");
  } else if (!running) {
    addUIClass(state, 'warn');
    state.textContent = rfUi('ТОЛЬКО CRON',"CRON ONLY");
  } else {
    addUIClass(state, 'good');
    state.textContent = rfUi('ГОТОВО',"READY");
  }
}

function renderRuntimeSummary() {
  const cfg = snapshot?.config || {};
  const rows = [
    ['Antiscan', snapshot?.detected ? rfUi('Обнаружен',"Detected") : rfUi('Не обнаружен',"Not detected")],
    [rfUi('Маркер работы',"Running marker"), snapshot?.running ? rfUi('Есть · /tmp/ascn.run',"Present · /tmp/ascn.run") : rfUi('Нет',"No")],
    ['Перечитывание конфигурации', snapshot?.config_reload_in_progress ? rfUi('Выполняется',"Running") : 'Нет'],
    ['Обновление Geo', snapshot?.geo_reload_in_progress ? rfUi('Выполняется',"Running") : 'Нет'],
    [rfUi('Интерфейсы',"Interfaces"), (cfg.isp_interfaces || []).join(', ') || '—'],
    [rfUi('Защищаемые порты',"Protected ports"), (cfg.ports || []).join(', ') || '—']
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
    [rfUi('Интерфейсы',"Interfaces"), (cfg.isp_interfaces || []).join(', ') || '—'],
    [rfUi('Порты роутера',"Router ports"), (cfg.ports || []).join(', ') || '—'],
    [rfUi('Проброшенные порты',"Forwarded ports"), (cfg.forwarded_ports || []).join(', ') || '—'],
    [rfUi('Защита IP и подсетей',"IP and subnet protection"), boolText(Boolean(p.ips_ban_enabled))],
    [rfUi('Окно новых соединений',"New connection window"), fmtDuration(p.recent_window_seconds)],
    [rfUi('Порог новых подключений',"New connection threshold"), p.recent_hitcount || '—'],
    [rfUi('Лимит одновременных соединений',"Concurrent connection limit"), p.concurrent_connection_limit || '—'],
    [rfUi('Блокировка IP',"IP block"), fmtDuration(p.direct_ip_ban_seconds)],
    [rfUi('Порог разных IP в /24',"Distinct IP threshold per /24"), p.different_ip_threshold || '—'],
    [rfUi('Хранение кандидатов',"Candidate retention"), fmtDuration(p.candidate_storage_seconds)],
    [rfUi('Блокировка /24',"/24 block"), fmtDuration(p.subnet_ban_seconds)],
    ['Ловушка', boolText(Boolean(p.honeypot_enabled))],
    [rfUi('Порты ловушки',"Honeypot ports"), (cfg.honeypot_ports || []).join(', ') || '—'],
    ['Пользовательские исключения', boolText(Boolean(cfg.use_custom_exclude_list))],
    [rfUi('Режим пользовательского списка',"User list mode"), cfg.custom_lists_block_mode || '0'],
    [rfUi('Режим Geo',"Geo mode"), cfg.geoblock_mode || '0'],
    [rfUi('Страны Geo',"Geo countries"), (cfg.geoblock_countries || []).join(', ') || '—'],
    [rfUi('Страны-исключения Geo',"Excluded Geo countries"), (cfg.geo_exclude_countries || []).join(', ') || '—'],
    [rfUi('Импорт блокировок Keenetic',"Import Keenetic blocks"), boolText(Boolean(cfg.read_ndm_lockout_ipsets))],
    [rfUi('Сохранение наборов ipset',"Save ipset sets"), boolText(Boolean(cfg.save_ipsets))]
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
      description: setDescriptions[item.name] || rfUi('Набор Antiscan',"Antiscan set"),
      state: item.exists ? (hasOperationalError ? 'ЧАСТИЧНО' : 'АКТИВЕН') : rfUi('НЕТ',"NO"),
      stateClass: item.exists ? (hasOperationalError ? 'warn' : 'good') : 'neutral',
      count: item.count_known ? String(item.count ?? 0) : '—',
      countUnavailable, error: hasOperationalError ? localizeMessage(item.error) : ''
    };
  });
}

function verdictTitle(result) {
  if (result.verdict === 'blocked') return rfUi('ЗАБЛОКИРОВАН',"BLOCKED");
  if (result.verdict === 'excluded') return rfUi('ИСКЛЮЧЁН',"EXCLUDED");
  if (result.verdict === 'not-blocked') return rfUi('НЕ ЗАБЛОКИРОВАН',"NOT BLOCKED");
  if (result.verdict === 'candidate') return rfUi('КАНДИДАТ',"CANDIDATE");
  if (result.verdict === 'stopped') return rfUi('ANTISCAN ОСТАНОВЛЕН',"ANTISCAN STOPPED");
  if (result.verdict === 'not-detected') return rfUi('ANTISCAN НЕ ОБНАРУЖЕН',"ANTISCAN NOT DETECTED");
  return rfUi('НЕДОСТАТОЧНО ДАННЫХ',"INSUFFICIENT DATA");
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
      bits.push(rfUi('без таймаута',"without timeout"));
    } else {
      const expires = new Date(Date.now() + seconds * 1000);
      bits.push(`осталось ${fmtDuration(seconds)} · примерно до ${expires.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`);
    }
  }
  if (entry.packets_known) bits.push(`${entry.packets} пак.`);
  if (entry.bytes_known) bits.push(fmtBytes(entry.bytes));
  return bits.join(' · ') || rfUi('запись в наборе',"entry in set");
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
        ['ascn_geo_exclude', rfUi('Geo-исключения',"Geo exclusions")],
        ['ascn_geo_blacklist', rfUi('Geo-чёрный список',"Geo blocklist")],
        ['ascn_geo_whitelist', rfUi('Geo-белый список',"Geo allowlist")]
      ]
    : [
        ['ascn_custom_exclude', rfUi('Пользовательские исключения',"User exclusions")],
        ['ascn_custom_blacklist', rfUi('Пользовательский чёрный список',"User blocklist")],
        ['ascn_custom_whitelist', rfUi('Пользовательский белый список',"User allowlist")]
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
    title.textContent = rfUi('Geo-наборы Antiscan',"Antiscan Geo sets");
    description.textContent = rfUi('Текущее содержимое Geo ipset. Для больших наборов показываются первые 500 записей.',"Current Geo ipset contents. Large sets show only the first 500 entries.");
  } else {
    title.textContent = rfUi('Активные пользовательские списки',"Active user lists");
    description.textContent = rfUi('Текущее содержимое runtime ipset для blacklist, whitelist и exclude.',"Current active ipsets for blocklists, allowlists, and exclusions.");
  }

  if (browserState[mode]) {
    renderSetPage(mode);
  } else {
    listsControls['listMeta'].textContent = '—';
    listsControls['listEntries'].className = 'entry-table empty-state';
    listRows = []; listRowsEmpty = rfUi('Выберите набор и загрузите текущее состояние.',"Select a set and load its current status.");
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
  setRows([], rfUi('Набор ещё не загружен.',"Set not loaded yet."));
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
    meta.textContent = rfUi('НЕТ',"NO");
    setRows([], `${page.name}: набор сейчас отсутствует.`);
    return;
  }
  const all = page.entries || [];
  const entries = filter ? all.filter(entry => String(entry.value || '').toLowerCase().includes(filter)) : all;
  const total = page.count_known ? page.count : all.length;
  meta.textContent = rfUi(`${page.name} · ${total} записей${page.truncated ? ` · показаны первые ${page.limit}` : ''}`,`${page.name} · ${total} entries${page.truncated ? ` · first ${page.limit} shown` : ''}`);
  if (!entries.length) {
    target.className = 'entry-table empty-state';
    setRows([], filter ? rfUi('По фильтру ничего не найдено.',"No results match the filter.") : rfUi('Набор пуст.',"Set empty."));
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
    if (blocked) { blockedRows = []; blockedRowsEmpty = rfUi('Читаем текущий набор ipset…',"Reading current ipset set…"); }
    else { listRows = []; listRowsEmpty = rfUi('Читаем текущий набор ipset…',"Reading current ipset set…"); }
  }
  try {
    const page = await read.api(`sets?name=${encodeURIComponent(selected)}&limit=500`);
    if (!read.isCurrent() || select.value !== selected) return;
    browserState[kind] = page;
    renderSetPage(kind);
  } catch (error) {
    if (!read.isCurrent() || select.value !== selected) return;
    browserState[kind] = { name: selected, error: error.message || rfUi('Не удалось прочитать ipset.',"Could not read the ipset.") };
    renderSetPage(kind);
  } finally {
    if (read.isCurrent()) button.disabled = false;
    read.finish();
  }
}

function mutationSuccessMessage(context, payload) {
  if (!payload?.changed) {
    if (context === 'unban') return rfUi('Запись уже отсутствовала в активной блокировке.',"Entry was already absent from the active block set.");
    if (context === 'list-entry') return rfUi('Такая запись уже есть в выбранном списке.',"This entry already exists in the selected list.");
    if (context === 'lifecycle:start') return rfUi('Antiscan уже запущен.',"Antiscan is already running.");
    if (context === 'lifecycle:stop') return rfUi('Antiscan уже остановлен.',"Antiscan is already stopped.");
    if (context === 'schedule:apply') return rfUi('ascn_crontab.conf уже совпадал с формой; активный crontab синхронизирован.',"ascn_crontab.conf already matched the form; active crontab is synchronized.");
    if (context.startsWith('flush:')) return rfUi('Выбранные данные уже были пусты; штатная очистка проверена.',"Selected data was already empty; standard cleanup verified.");
    if (context.startsWith('operation:')) return rfUi('Команда не потребовалась: текущее состояние уже корректно.',"No command needed; current state is already correct.");
    return rfUi('Состояние уже соответствовало запросу.',"State already matched the request.");
  }
  if (context === 'unban') return rfUi('Блокировка снята, результат проверен.',"Block removed; result verified.");
  if (context === 'list-entry') return rfUi('Запись добавлена в список и результат проверен.',"Entry added to list; result verified.");
  if (context === 'lifecycle:start') return rfUi('Antiscan запущен, состояние проверено.',"Antiscan started; status verified.");
  if (context === 'lifecycle:stop') return rfUi('Antiscan остановлен. Защита отключена до следующего запуска.',"Antiscan is stopped. Protection is disabled until the next start.");
  if (context === 'lifecycle:reload') return rfUi('Antiscan перечитал конфигурацию, состояние проверено.',"Antiscan reloaded its configuration; status verified.");
  if (context === 'lifecycle:restart') return rfUi('Antiscan штатно перезапущен, состояние проверено.',"Antiscan restarted through its standard script; status verified.");
  if (context === 'rci-token:set') return rfUi('RCI-токен сохранён и проверен штатной командой Antiscan.',"RCI token saved and verified using the standard Antiscan command.");
  if (context === 'rci-token:check') return rfUi('Сохранённый RCI-токен прошёл проверку Keenetic.',"Saved RCI token passed Keenetic verification.");
  if (context === 'rci-token:delete') return rfUi('RCI-токен удалён, состояние проверено.',"RCI token deleted; status verified.");
  if (context === 'schedule:apply') return rfUi('Расписание Antiscan обновлено и синхронизировано с активным crontab.',"Antiscan schedule updated and synchronized with active crontab.");
  if (context.startsWith('flush:')) return rfUi('Штатная очистка Antiscan выполнена, результат проверен.',"Standard Antiscan cleanup completed and verified.");
  if (context.startsWith('operation:')) return rfUi('Штатная сервисная команда Antiscan выполнена и проверена.',"Standard Antiscan service command completed and verified.");
  if (context === 'config') {
    return payload?.runtime_applied
      ? rfUi('Настройки сохранены, применены и проверены.',"Settings saved, applied and verified.")
      : rfUi('Настройки сохранены и проверены. Они вступят в силу при следующем запуске Antiscan.',"Settings saved and verified. They will take effect when Antiscan next starts.");
  }
  return rfUi('Изменение применено и проверено.',"Change applied and verified.");
}

function showMutationResult(payload, failed = false, context = '') {
  const notice = shellControls['actionNotice'];
  notice.hidden = false;
  notice.className = `notice action-notice ${failed ? 'bad' : 'good'}`;
  const warnings = localizeMessages(payload?.warnings);
  if (failed) {
    const prefix = payload?.rollback_performed
      ? rfUi('Операция не выполнена. Предыдущее состояние восстановлено.',"Operation not completed. Previous state restored.")
      : rfUi('Операцию выполнить не удалось.',"Operation failed.");
    const reason = localizeMessage(payload?.error || payload?.message || '');
    notice.textContent = [prefix, reason, ...warnings].filter(Boolean).join(' · ');
    return;
  }
  const messages = [mutationSuccessMessage(context, payload), ...warnings];
  if (payload?.restart_required) {
    messages.push(context.startsWith('flush:')
      ? rfUi('После штатной очистки восстановите нужные данные и перезапустите Antiscan перед возвратом соответствующей защиты.',"After standard cleanup, restore the required data and restart Antiscan before re-enabling that protection.")
      : rfUi('Требуется остановить и снова запустить Antiscan, чтобы полностью применить новый IPSETS_DIRECTORY.',"Stop and restart Antiscan to fully apply the new IPSETS_DIRECTORY."));
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
  const target = listName === 'exclude' ? rfUi('пользовательские исключения',"user exclusions") : rfUi('пользовательский белый список',"user allowlist");
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
    blacklist: rfUi('Чёрный список',"Blocklist"),
    whitelist: rfUi('Белый список',"Allowlist"),
    exclude: rfUi('Исключения',"Exclusions")
  }[name] || name || rfUi('Пользовательский список',"User list");
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
  const state = page?.active ? rfUi('активен',"active") : page?.configured ? rfUi('включён, Antiscan остановлен',"enabled, Antiscan stopped") : rfUi('выключен',"disabled");
  const runtime = page?.active ? (page.runtime_exists ? rfUi('runtime есть',"service runtime present") : rfUi('runtime отсутствует',"service runtime missing")) : rfUi('runtime не требуется',"service runtime not required");
  const skipped = Number(page?.skipped_invalid || 0);
  meta.textContent = rfUi(`${customListLabel(page?.list)} · записей ${Number(page?.count || 0)} · ${state} · ${runtime}${skipped ? ` · пропущено строк ${skipped}` : ''}`,`${customListLabel(page?.list)} · entries ${Number(page?.count || 0)} · ${state} · ${runtime}${skipped ? ` · skipped ${skipped} lines` : ''}`);
  if (clearButton) {
    clearButton.disabled = customListBusy || (Number(page?.count || 0) === 0 && !page?.runtime_exists);
    clearButton.title = page?.active
      ? rfUi('Активный список будет очищен штатной командой Antiscan flush.',"Active set will be cleared using the standard Antiscan flush command.")
      : rfUi('Исходный файл будет очищен до нулевой длины.',"Source file will be emptied.");
  }
  if (reloadRuntimeButton) reloadRuntimeButton.disabled = customListBusy || !page?.configured || !page?.running;
  const entries = page?.entries || [];
  if (!entries.length) {
    target.className = 'entry-table empty-state';
    customRowsEmpty = rfUi('В исходном файле нет корректных IPv4/CIDR записей.',"No valid IPv4/CIDR records in the source file.");
    return;
  }
  target.className = 'entry-table';
  customRows = entries.map(entry => ({
    value: entry, meta: `исходный файл ${page?.set || ''}`,
    explanation: page?.active
      ? rfUi('Изменение будет применено штатной командой Antiscan; удаление последней записи перейдёт в upstream flush.',"Change will be applied with the standard Antiscan command; deleting the last entry will trigger upstream flush.")
      : rfUi('Изменение затронет только исходный файл.',"Change will affect the source file only."),
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
    renderCustomList({ list: selected, error: error.message || rfUi('Не удалось прочитать исходный пользовательский список.',"Could not read the original user list.") });
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
    notice.textContent = rfUi('Есть несохранённые изменения ascn.conf. Сначала примените их или сбросьте форму.',"ascn.conf has unsaved changes. Apply or discard them first.");
    return;
  }

  const prompts = {
    start: rfUi('Запустить Antiscan?\n\nБудут созданы штатные наборы ipset и правила фильтрации Antiscan.',"Start Antiscan?\\n\\nStandard Antiscan ipset sets and filtering rules will be created."),
    stop: rfUi('Остановить Antiscan?\n\nЗащита Antiscan будет отключена до следующего запуска. Сохранение состояния при остановке выполняет сам Antiscan согласно SAVE_ON_EXIT.',"Stop Antiscan?\\n\\nAntiscan protection will be disabled until next start. Antiscan itself saves state on shutdown according to SAVE_ON_EXIT."),
    reload: rfUi('Перечитать текущий ascn.conf?\n\nAntiscan штатно перестроит необходимые правила и наборы, после чего RouterForge проверит итоговое состояние.',"Reload the current ascn.conf?\\n\\nAntiscan will rebuild the required rules and sets, then RouterForge will verify the resulting state."),
    restart: rfUi('Штатно перезапустить Antiscan?\n\nБудет вызван именно S99ascn restart. Upstream выполнит свой stop 1, затем start и сохранит предусмотренную им семантику перезапуска.',"Restart Antiscan using its standard script?\\n\\nS99ascn restart will be invoked. The upstream script will perform stop 1 and then start, preserving its restart behavior.")
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
    notice.textContent = rfUi('Есть несохранённые изменения ascn.conf. Сначала примените их или сбросьте форму.',"ascn.conf has unsaved changes. Apply or discard them first.");
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
  if (output) output.textContent = rfUi('Выполняется штатная команда Antiscan…',"Running an Antiscan command…");
  try {
    const result = await mutate('operation', { action, scope, confirm: 'RUN' });
    showMutationResult(result, false, `operation:${key}`);
    if (output) output.textContent = result.output || rfUi('Команда завершена без текстового вывода.',"The command completed without output.");
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    const payload = error.payload || { error: error.message };
    showMutationResult(payload, true, `operation:${key}`);
    if (output) output.textContent = localizeMessage(payload.error || error.message || rfUi('Ошибка сервисной операции.',"Service operation error."));
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
    unban: rfUi('Снятие одной блокировки',"Remove one block"),
    'list-entry': rfUi('Изменение пользовательского списка',"User list change"),
    'custom-list:add': rfUi('Добавление в пользовательский список',"Add to user list"),
    'custom-list:delete': rfUi('Удаление из пользовательского списка',"Remove from user list"),
    'custom-list:clear': rfUi('Очистка пользовательского списка',"Clear user list"),
    'custom-list:reload': rfUi('Перечитывание пользовательского списка',"Reload user list"),
    'rci-token:set': rfUi('Установка RCI-токена',"Set RCI token"),
    'rci-token:check': rfUi('Проверка RCI-токена',"RCI token check"),
    'rci-token:delete': rfUi('Удаление RCI-токена',"Delete RCI token"),
    'lifecycle:start': rfUi('Запуск Antiscan',"Start Antiscan"),
    'lifecycle:stop': rfUi('Остановка Antiscan',"Stop Antiscan"),
    'lifecycle:reload': rfUi('Перечитывание конфигурации',"Reload configuration"),
    'lifecycle:restart': rfUi('Перезапуск Antiscan',"Restart Antiscan"),
    'operation:update_rules': rfUi('Восстановление правил Antiscan',"Restore Antiscan rules"),
    'operation:read_candidates': rfUi('Обработка кандидатов',"Process candidates"),
    'operation:read_ndm_ipsets': rfUi(rfUi('Импорт блокировок Keenetic',"Import Keenetic blocks"),"Import Keenetic blocks"),
    'operation:save_ipsets': rfUi('Сохранение runtime ipset',"Save runtime ipset"),
    'operation:update_ipsets:custom': rfUi('Перечитывание пользовательских списков',"Reload user lists"),
    'operation:update_ipsets:geo': rfUi('Обновление Geo-списков',"Update Geo lists"),
    'operation:retry_load_geo': rfUi('Повторная загрузка Geo',"Retry Geo download"),
    'operation:update_crontab': rfUi('Синхронизация cron',"Synchronize cron"),
    'schedule:apply': rfUi('Изменение расписания Antiscan',"Antiscan schedule change"),
    config: rfUi('Изменение конфигурации',"Configuration change")
  }[action] || action || rfUi('Действие Antiscan Manager',"Antiscan Manager action");
}

function renderHistory(page) {
  const events = page?.events || [];
  historyLoaded = true;
  historyControls['historyMeta'].textContent = rfUi(`${events.length} событий · новые сверху${page?.skipped_invalid_lines ? ` · пропущено повреждённых строк: ${page.skipped_invalid_lines}` : ''}`,`${events.length} events · newest first${page?.skipped_invalid_lines ? ` · damaged lines skipped: ${page.skipped_invalid_lines}` : ''}`);
  historyRowsEmpty = rfUi('В журнале пока нет действий Antiscan Manager.',"No Antiscan Manager activity has been logged yet.");
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
    historyControls['historyMeta'].textContent = rfUi('Не удалось прочитать историю',"Could not read history");
    historyRows = [];
    historyRowsEmpty = localizeMessage(error.message || rfUi('Не удалось прочитать журнал действий.',"Could not read the activity log."));
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
    notice.textContent = localizeMessage(error.message || rfUi('Не удалось получить статус Antiscan Manager.',"Could not retrieve Antiscan Manager status."));
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
  inspectorEmpty = rfUi('Проверяем текущее состояние Antiscan…',"Checking current Antiscan status…");
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
      warnings: localizeMessages(payload.warnings || [error.message || rfUi('Ошибка проверки',"Verification error")])
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
        <p>{rfUi("Диагностика и безопасное управление Antiscan: понятные причины блокировок, настройка с откатом и журнал последних действий.","Diagnose and manage Antiscan safely: clear blocking explanations, rollback-protected configuration, and an action log.")}</p>
      </div>
      <div class="hero-actions">
        <span id="targetState" class={shellControls['targetState'].className} hidden={shellControls['targetState'].hidden}>{#if shellControls['targetState'].textContent !== undefined}{shellControls['targetState'].textContent}{:else}{rfUi("ПРОВЕРКА…","CHECKING…")}{/if}</span>
        <button id="refresh" class={shellControls['refresh'].className} type="button" hidden={shellControls['refresh'].hidden} disabled={shellControls['refresh'].disabled} onclick={refreshAll}>{#if shellControls['refresh'].textContent !== undefined}{shellControls['refresh'].textContent}{:else}{rfUi("↻ Обновить","↻ Refresh")}{/if}</button>
      </div>
    </header>

    <section id="notice" class={shellControls['notice'].className} hidden={shellControls['notice'].hidden}>{#if shellControls['notice'].textContent !== undefined}{shellControls['notice'].textContent}{:else}{/if}</section>
    <section id="actionNotice" class={shellControls['actionNotice'].className} hidden={shellControls['actionNotice'].hidden}>{#if shellControls['actionNotice'].textContent !== undefined}{shellControls['actionNotice'].textContent}{:else}{/if}</section>

    <section class="metrics" aria-label="Antiscan summary">
      <article class="metric"><span>{rfUi("Версия","Version")}</span><strong id="version" class={shellControls['version'].className} hidden={shellControls['version'].hidden}>{#if shellControls['version'].textContent !== undefined}{shellControls['version'].textContent}{:else}—{/if}</strong><small>{rfUi("оригинальный Antiscan","original Antiscan")}</small></article>
      <article class="metric"><span>{rfUi("Прямые блокировки","Direct blocks")}</span><strong id="blockedIps" class={shellControls['blockedIps'].className} hidden={shellControls['blockedIps'].hidden}>{#if shellControls['blockedIps'].textContent !== undefined}{shellControls['blockedIps'].textContent}{:else}—{/if}</strong><small>ascn_ips</small></article>
      <article class="metric"><span>{rfUi("Заблокировано /24","Blocked /24")}</span><strong id="blockedSubnets" class={shellControls['blockedSubnets'].className} hidden={shellControls['blockedSubnets'].hidden}>{#if shellControls['blockedSubnets'].textContent !== undefined}{shellControls['blockedSubnets'].textContent}{:else}—{/if}</strong><small>ascn_subnets</small></article>
      <article class="metric"><span>{rfUi("Кандидаты","Candidates")}</span><strong id="candidateCount" class={shellControls['candidateCount'].className} hidden={shellControls['candidateCount'].hidden}>{#if shellControls['candidateCount'].textContent !== undefined}{shellControls['candidateCount'].textContent}{:else}—{/if}</strong><small>ascn_candidates</small></article>
    </section>

    <nav class="tabs" aria-label="Antiscan Manager sections">
      <button class={shellControls['rf-generated-1'].className} type="button" data-tab="overview" id="rf-generated-1" hidden={shellControls['rf-generated-1'].hidden} disabled={shellControls['rf-generated-1'].disabled} onclick={() => chooseTab("overview")}>{#if shellControls['rf-generated-1'].textContent !== undefined}{shellControls['rf-generated-1'].textContent}{:else}{rfUi("Обзор","Overview")}{/if}</button>
      <button class={shellControls['rf-generated-2'].className} type="button" data-tab="diagnostics" id="rf-generated-2" hidden={shellControls['rf-generated-2'].hidden} disabled={shellControls['rf-generated-2'].disabled} onclick={() => chooseTab("diagnostics")}>{#if shellControls['rf-generated-2'].textContent !== undefined}{shellControls['rf-generated-2'].textContent}{:else}{rfUi("Диагностика","Diagnostics")}{/if}</button>
      <button class={shellControls['rf-generated-3'].className} type="button" data-tab="blocked" id="rf-generated-3" hidden={shellControls['rf-generated-3'].hidden} disabled={shellControls['rf-generated-3'].disabled} onclick={() => chooseTab("blocked")}>{#if shellControls['rf-generated-3'].textContent !== undefined}{shellControls['rf-generated-3'].textContent}{:else}{rfUi("Блокировки","Blocks")}{/if}</button>
      <button class={shellControls['rf-generated-4'].className} type="button" data-tab="lists" id="rf-generated-4" hidden={shellControls['rf-generated-4'].hidden} disabled={shellControls['rf-generated-4'].disabled} onclick={() => chooseTab("lists")}>{#if shellControls['rf-generated-4'].textContent !== undefined}{shellControls['rf-generated-4'].textContent}{:else}{rfUi("Списки","Lists")}{/if}</button>
      <button class={shellControls['rf-generated-5'].className} type="button" data-tab="geo" id="rf-generated-5" hidden={shellControls['rf-generated-5'].hidden} disabled={shellControls['rf-generated-5'].disabled} onclick={() => chooseTab("geo")}>{#if shellControls['rf-generated-5'].textContent !== undefined}{shellControls['rf-generated-5'].textContent}{:else}Geo{/if}</button>
      <button class={shellControls['rf-generated-6'].className} type="button" data-tab="schedule" id="rf-generated-6" hidden={shellControls['rf-generated-6'].hidden} disabled={shellControls['rf-generated-6'].disabled} onclick={() => chooseTab("schedule")}>{#if shellControls['rf-generated-6'].textContent !== undefined}{shellControls['rf-generated-6'].textContent}{:else}{rfUi("Расписание","Schedule")}{/if}</button>
      <button class={shellControls['rf-generated-7'].className} type="button" data-tab="config" id="rf-generated-7" hidden={shellControls['rf-generated-7'].hidden} disabled={shellControls['rf-generated-7'].disabled} onclick={() => chooseTab("config")}>{#if shellControls['rf-generated-7'].textContent !== undefined}{shellControls['rf-generated-7'].textContent}{:else}{rfUi("Настройки","Settings")}{/if}</button>
      <button class={shellControls['rf-generated-8'].className} type="button" data-tab="history" id="rf-generated-8" hidden={shellControls['rf-generated-8'].hidden} disabled={shellControls['rf-generated-8'].disabled} onclick={() => chooseTab("history")}>{#if shellControls['rf-generated-8'].textContent !== undefined}{shellControls['rf-generated-8'].textContent}{:else}{rfUi("История","History")}{/if}</button>
    </nav>

    <section class={overviewControls['rf-generated-9'].className} data-page="overview" id="rf-generated-9" hidden={overviewControls['rf-generated-9'].hidden}>{#if overviewControls['rf-generated-9'].textContent !== undefined}{overviewControls['rf-generated-9'].textContent}{:else}
      <div class="two-column overview-grid">
        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">{rfUi("ТЕКУЩЕЕ СОСТОЯНИЕ","CURRENT STATUS")}</span>
              <h2>{rfUi("Что сейчас работает","What is currently active")}</h2>
              <p>{rfUi("Текущее состояние Antiscan без каких-либо изменений системы.","Current Antiscan status without making any system changes.")}</p>
            </div>
            <span id="ipsetBinary" class={overviewControls['ipsetBinary'].className} hidden={overviewControls['ipsetBinary'].hidden}>{#if overviewControls['ipsetBinary'].textContent !== undefined}{overviewControls['ipsetBinary'].textContent}{:else}ipset: —{/if}</span>
          </div>
          <div id="runtimeSummary" class={overviewControls['runtimeSummary'].className} hidden={overviewControls['runtimeSummary'].hidden}><AntiscanKeyValueRows rows={runtimeRows} /></div>
        </section>

        <section class="panel risk-panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">{rfUi("РИСК ЛОЖНЫХ БЛОКИРОВОК","FALSE-POSITIVE RISK")}</span>
              <h2>{rfUi("Мобильные сети и /24","Mobile networks and /24")}</h2>
              <p>{rfUi("Проверяем сочетания настроек, которые могут случайно зацепить динамические адреса оператора.","Check setting combinations that may unintentionally block dynamic carrier IPs.")}</p>
            </div>
          </div>
          <div id="riskSummary" class={overviewControls['riskSummary'].className} hidden={overviewControls['riskSummary'].hidden}><AntiscanRiskSummary view={riskView} /></div>
        </section>
      </div>

      <section class="panel lifecycle-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("УПРАВЛЕНИЕ","CONTROLS")}</span>
            <h2>{rfUi("Управление Antiscan","Antiscan controls")}</h2>
            <p>{rfUi("Запуск, остановка и перечитывание настроек выполняются только штатным скриптом","Start, stop, and reload use only the upstream service script")} <span class="mono">S99ascn</span>{rfUi(". Правила Antiscan RouterForge не подменяет.",". RouterForge does not replace Antiscan firewall rules.")}</p>
          </div>
          <span id="lifecycleState" class={overviewControls['lifecycleState'].className} hidden={overviewControls['lifecycleState'].hidden}>{#if overviewControls['lifecycleState'].textContent !== undefined}{overviewControls['lifecycleState'].textContent}{:else}{rfUi("ПРОВЕРКА…","CHECKING…")}{/if}</span>
        </div>
        <div class="lifecycle-body">
          <div class="lifecycle-copy">
            <strong>{rfUi("Безопасное управление","Safe controls")}</strong>
            <span id="lifecycleHint" class={overviewControls['lifecycleHint'].className} hidden={overviewControls['lifecycleHint'].hidden}>{#if overviewControls['lifecycleHint'].textContent !== undefined}{overviewControls['lifecycleHint'].textContent}{:else}{rfUi("Состояние загружается…","Loading status…")}{/if}</span>
          </div>
          <div class="lifecycle-actions">
            <button id="startAntiscan" class={overviewControls['startAntiscan'].className} type="button" disabled={overviewControls['startAntiscan'].disabled} hidden={overviewControls['startAntiscan'].hidden} onclick={() => performLifecycle('start')}>{#if overviewControls['startAntiscan'].textContent !== undefined}{overviewControls['startAntiscan'].textContent}{:else}{rfUi("▶ Запустить","▶ Start")}{/if}</button>
            <button id="reloadAntiscan" class={overviewControls['reloadAntiscan'].className} type="button" disabled={overviewControls['reloadAntiscan'].disabled} hidden={overviewControls['reloadAntiscan'].hidden} onclick={() => performLifecycle('reload')}>{#if overviewControls['reloadAntiscan'].textContent !== undefined}{overviewControls['reloadAntiscan'].textContent}{:else}{rfUi("↻ Перечитать","↻ Reload")}{/if}</button>
            <button id="restartAntiscan" class={overviewControls['restartAntiscan'].className} type="button" disabled={overviewControls['restartAntiscan'].disabled} hidden={overviewControls['restartAntiscan'].hidden} onclick={() => performLifecycle('restart')}>{#if overviewControls['restartAntiscan'].textContent !== undefined}{overviewControls['restartAntiscan'].textContent}{:else}{rfUi("↻ Перезапустить","↻ Restart")}{/if}</button>
            <button id="stopAntiscan" class={overviewControls['stopAntiscan'].className} type="button" disabled={overviewControls['stopAntiscan'].disabled} hidden={overviewControls['stopAntiscan'].hidden} onclick={() => performLifecycle('stop')}>{#if overviewControls['stopAntiscan'].textContent !== undefined}{overviewControls['stopAntiscan'].textContent}{:else}{rfUi("■ Остановить","■ Stop")}{/if}</button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("СЕРВИСНЫЕ ОПЕРАЦИИ","SERVICE OPERATIONS")}</span>
            <h2>{rfUi("Обслуживание","Maintenance")}</h2>
            <p>{rfUi("Восстановление правил, обработка кандидатов, импорт блокировок Keenetic и сохранение текущего состояния.","Restore rules, process candidates, import Keenetic lockouts, and preserve current state.")}</p>
          </div>
          <span id="operationState" class={overviewControls['operationState'].className} hidden={overviewControls['operationState'].hidden}>{#if overviewControls['operationState'].textContent !== undefined}{overviewControls['operationState'].textContent}{:else}{rfUi("ГОТОВО","READY")}{/if}</span>
        </div>
        <div class="lifecycle-body">
          <div class="lifecycle-copy">
            <strong>{rfUi("Сервисные действия","Service actions")}</strong>
            <span>{rfUi("Редкие операции, которые обычно нужны только при диагностике или восстановлении Antiscan.","Infrequent operations usually needed only when diagnosing or recovering Antiscan.")}</span>
          </div>
          <div class="lifecycle-actions">
            <button class={overviewControls['rf-generated-10'].className} type="button" data-operation="update_rules" id="rf-generated-10" hidden={overviewControls['rf-generated-10'].hidden} disabled={overviewControls['rf-generated-10'].disabled} onclick={() => performOperation("read_candidates","")}>{#if overviewControls['rf-generated-10'].textContent !== undefined}{overviewControls['rf-generated-10'].textContent}{:else}{rfUi("Восстановить правила","Restore rules")}{/if}</button>
            <button class={overviewControls['rf-generated-11'].className} type="button" data-operation="read_candidates" id="rf-generated-11" hidden={overviewControls['rf-generated-11'].hidden} disabled={overviewControls['rf-generated-11'].disabled} onclick={() => performOperation("read_ndm_ipsets","")}>{#if overviewControls['rf-generated-11'].textContent !== undefined}{overviewControls['rf-generated-11'].textContent}{:else}{rfUi("Обработать кандидатов","Process candidates")}{/if}</button>
            <button class={overviewControls['rf-generated-12'].className} type="button" data-operation="read_ndm_ipsets" id="rf-generated-12" hidden={overviewControls['rf-generated-12'].hidden} disabled={overviewControls['rf-generated-12'].disabled} onclick={() => performOperation("save_ipsets","")}>{#if overviewControls['rf-generated-12'].textContent !== undefined}{overviewControls['rf-generated-12'].textContent}{:else}{rfUi("Импорт Keenetic","Import Keenetic")}{/if}</button>
            <button class={overviewControls['rf-generated-13'].className} type="button" data-operation="save_ipsets" id="rf-generated-13" hidden={overviewControls['rf-generated-13'].hidden} disabled={overviewControls['rf-generated-13'].disabled} onclick={() => performOperation("update_ipsets","geo")}>{#if overviewControls['rf-generated-13'].textContent !== undefined}{overviewControls['rf-generated-13'].textContent}{:else}{rfUi("Сохранить ipset","Save ipset")}{/if}</button>
          </div>
        </div>
        <div id="operationOutput" class={overviewControls['operationOutput'].className} hidden={overviewControls['operationOutput'].hidden}>{#if overviewControls['operationOutput'].textContent !== undefined}{overviewControls['operationOutput'].textContent}{:else}{rfUi("Последняя сервисная операция ещё не запускалась.","No service operation has been run yet.")}{/if}</div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("НАБОРЫ IPSET","IPSET SETS")}</span>
            <h2>{rfUi("Состояние наборов","Set status")}</h2>
            <p>{rfUi("Количество записей во всех известных наборах ipset Antiscan.","Entry counts across all known Antiscan ipsets.")}</p>
          </div>
        </div>
        <div class="table-scroll">
          <table>
            <thead><tr><th>{rfUi("Набор","Set")}</th><th>{rfUi("Назначение","Purpose")}</th><th>{rfUi("Состояние","Status")}</th><th>{rfUi("Записей","Entries")}</th></tr></thead>
            <tbody id="sets" class={overviewControls['sets'].className} hidden={overviewControls['sets'].hidden}><AntiscanIPSetRows rows={ipsetRows} /></tbody>
          </table>
        </div>
      </section>
    {/if}</section>

    <section class={diagnosticsControls['rf-generated-14'].className} data-page="diagnostics" id="rf-generated-14" hidden={diagnosticsControls['rf-generated-14'].hidden}>{#if diagnosticsControls['rf-generated-14'].textContent !== undefined}{diagnosticsControls['rf-generated-14'].textContent}{:else}
      <section class="panel diagnostics-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("ГОТОВНОСТЬ ANTISCAN","ANTISCAN READINESS")}</span>
            <h2>{rfUi("Диагностика окружения","Environment diagnostics")}</h2>
            <p>{rfUi("Проверяем штатные файлы, зависимости, Netfilter, hook, cron, RCI и lock-состояние без изменения настроек.","Check upstream files, dependencies, netfilter, hooks, cron, RCI, and lock state without changing settings.")}</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="diagnosticsState" class={diagnosticsControls['diagnosticsState'].className} hidden={diagnosticsControls['diagnosticsState'].hidden}>{#if diagnosticsControls['diagnosticsState'].textContent !== undefined}{diagnosticsControls['diagnosticsState'].textContent}{:else}{rfUi("НЕ ЗАГРУЖЕНО","NOT LOADED")}{/if}</span>
            <button id="reloadDiagnostics" class={diagnosticsControls['reloadDiagnostics'].className} type="button" hidden={diagnosticsControls['reloadDiagnostics'].hidden} disabled={diagnosticsControls['reloadDiagnostics'].disabled} onclick={loadDiagnostics}>{#if diagnosticsControls['reloadDiagnostics'].textContent !== undefined}{diagnosticsControls['reloadDiagnostics'].textContent}{:else}{rfUi("↻ Проверить","↻ Check")}{/if}</button>
          </div>
        </div>
        <div id="diagnosticsMeta" class={diagnosticsControls['diagnosticsMeta'].className} hidden={diagnosticsControls['diagnosticsMeta'].hidden}>{#if diagnosticsControls['diagnosticsMeta'].textContent !== undefined}{diagnosticsControls['diagnosticsMeta'].textContent}{:else}{rfUi("Откройте вкладку, чтобы выполнить read-only проверку.","Open this tab to run a read-only check.")}{/if}</div>
        <div id="diagnosticsChecks" class={diagnosticsControls['diagnosticsChecks'].className.replace(/\bempty-state\b/g, '').trim()} hidden={diagnosticsControls['diagnosticsChecks'].hidden}><AntiscanDiagnosticChecks checks={diagnosticChecks} emptyText={diagnosticChecksEmpty} /></div>
      </section>

      <section class="panel rci-token-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">RCI / KEENETIC</span>
            <h2>{rfUi("RCI-токен","RCI token")}</h2>
            <p>{rfUi("Статус token-flow Antiscan и безопасное управление штатными командами","Antiscan token flow status and safe control through upstream commands")} <span class="mono">token set/check/delete</span>.</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="rciTokenState" class={diagnosticsControls['rciTokenState'].className} hidden={diagnosticsControls['rciTokenState'].hidden}>{#if diagnosticsControls['rciTokenState'].textContent !== undefined}{diagnosticsControls['rciTokenState'].textContent}{:else}{rfUi("НЕ ЗАГРУЖЕНО","NOT LOADED")}{/if}</span>
            <button id="reloadRCIToken" class={diagnosticsControls['reloadRCIToken'].className} type="button" hidden={diagnosticsControls['reloadRCIToken'].hidden} disabled={diagnosticsControls['reloadRCIToken'].disabled} onclick={loadRCITokenStatus}>{#if diagnosticsControls['reloadRCIToken'].textContent !== undefined}{diagnosticsControls['reloadRCIToken'].textContent}{:else}{rfUi("↻ Обновить","↻ Refresh")}{/if}</button>
          </div>
        </div>
        <div class="rci-token-body">
          <div class="rci-token-copy">
            <strong id="rciTokenSummary" class={diagnosticsControls['rciTokenSummary'].className} hidden={diagnosticsControls['rciTokenSummary'].hidden}>{#if diagnosticsControls['rciTokenSummary'].textContent !== undefined}{diagnosticsControls['rciTokenSummary'].textContent}{:else}{rfUi("Состояние RCI-токена ещё не загружено.","RCI token status has not been loaded yet.")}{/if}</strong>
            <span id="rciTokenMeta" class={diagnosticsControls['rciTokenMeta'].className} hidden={diagnosticsControls['rciTokenMeta'].hidden}>{#if diagnosticsControls['rciTokenMeta'].textContent !== undefined}{diagnosticsControls['rciTokenMeta'].textContent}{:else}{rfUi("Значение токена RouterForge не читает и не показывает.","RouterForge never reads or displays the token value.")}{/if}</span>
            <span class="rci-token-security">{rfUi("Токен передаётся upstream только через stdin. Он не попадает в argv, URL, ответ API или журнал операций.","The token is passed to the upstream process only through stdin. It never appears in argv, URLs, API responses, or operation logs.")}</span>
          </div>
          <div class="rci-token-controls">
            <input id="rciTokenInput" type="password" autocomplete="new-password" spellcheck="false" placeholder={rfUi("Новый RCI-токен","New RCI token")} aria-label={rfUi("Новый RCI-токен","New RCI token")} class={diagnosticsControls['rciTokenInput'].className} hidden={diagnosticsControls['rciTokenInput'].hidden} disabled={diagnosticsControls['rciTokenInput'].disabled} bind:value={diagnosticsControls['rciTokenInput'].value} onkeydown={(event) => {if(event.key === 'Enter') performRCITokenAction('set');}}>
            <button id="setRCIToken" class={diagnosticsControls['setRCIToken'].className} type="button" disabled={diagnosticsControls['setRCIToken'].disabled} hidden={diagnosticsControls['setRCIToken'].hidden} onclick={() => performRCITokenAction('set')}>{#if diagnosticsControls['setRCIToken'].textContent !== undefined}{diagnosticsControls['setRCIToken'].textContent}{:else}{rfUi("Сохранить токен","Save token")}{/if}</button>
            <button id="checkRCIToken" class={diagnosticsControls['checkRCIToken'].className} type="button" disabled={diagnosticsControls['checkRCIToken'].disabled} hidden={diagnosticsControls['checkRCIToken'].hidden} onclick={() => performRCITokenAction('check')}>{#if diagnosticsControls['checkRCIToken'].textContent !== undefined}{diagnosticsControls['checkRCIToken'].textContent}{:else}{rfUi("Проверить","Check")}{/if}</button>
            <button id="deleteRCIToken" class={diagnosticsControls['deleteRCIToken'].className} type="button" disabled={diagnosticsControls['deleteRCIToken'].disabled} hidden={diagnosticsControls['deleteRCIToken'].hidden} onclick={() => performRCITokenAction('delete')}>{#if diagnosticsControls['deleteRCIToken'].textContent !== undefined}{diagnosticsControls['deleteRCIToken'].textContent}{:else}{rfUi("Удалить","Delete")}{/if}</button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("СОВМЕСТИМОСТЬ","COMPATIBILITY")}</span>
            <h2>{rfUi("Совместимость и обновления","Compatibility and updates")}</h2>
            <p>{rfUi("Учтённый функционал upstream привязан к проверенной версии. RouterForge не устанавливает обновления Antiscan: здесь только read-only состояние пакета, update cache, прошивки, Netfilter и hook.","Available upstream features are based on the verified version. RouterForge does not install Antiscan updates here; this screen only shows package, update cache, firmware, Netfilter, and hook status.")}</p>
          </div>
          <div class="diagnostics-head-actions">
            <span id="compatibilityState" class={diagnosticsControls['compatibilityState'].className} hidden={diagnosticsControls['compatibilityState'].hidden}>{#if diagnosticsControls['compatibilityState'].textContent !== undefined}{diagnosticsControls['compatibilityState'].textContent}{:else}{rfUi("НЕ ЗАГРУЖЕНО","NOT LOADED")}{/if}</span>
            <button id="reloadCompatibility" class={diagnosticsControls['reloadCompatibility'].className} type="button" hidden={diagnosticsControls['reloadCompatibility'].hidden} disabled={diagnosticsControls['reloadCompatibility'].disabled} onclick={loadCompatibility}>{#if diagnosticsControls['reloadCompatibility'].textContent !== undefined}{diagnosticsControls['reloadCompatibility'].textContent}{:else}{rfUi("↻ Перечитать","↻ Reload")}{/if}</button>
          </div>
        </div>
        <div id="compatibilityMeta" class={diagnosticsControls['compatibilityMeta'].className} hidden={diagnosticsControls['compatibilityMeta'].hidden}>{#if diagnosticsControls['compatibilityMeta'].textContent !== undefined}{diagnosticsControls['compatibilityMeta'].textContent}{:else}{rfUi("Откройте вкладку, чтобы проверить совместимость без сетевых запросов и изменений системы.","Open this tab to check compatibility without network requests or system changes.")}{/if}</div>
        <div id="compatibilityGrid" class={diagnosticsControls['compatibilityGrid'].className} hidden={diagnosticsControls['compatibilityGrid'].hidden}><AntiscanKeyValueRows rows={compatibilityRows} /></div>
        <div id="compatibilityWarnings" class={diagnosticsControls['compatibilityWarnings'].className} hidden={diagnosticsControls['compatibilityWarnings'].hidden}>{#if compatibilityError}<div class="empty-state bad-text">{compatibilityError}</div>{:else if compatibilityLoaded}<AntiscanWarningItems messages={compatibilityWarnings} emptyText="Версия, package metadata, Netfilter и hook согласованы с проверенным контрактом RouterForge." />{/if}</div>
        <div id="diagnosticsContract" class={diagnosticsControls['diagnosticsContract'].className} hidden={diagnosticsControls['diagnosticsContract'].hidden}><AntiscanKeyValueRows rows={diagnosticContractRows} /></div>
      </section>
    {/if}</section>
    <section class={inspectorControls['rf-generated-15'].className} data-page="inspect" id="rf-generated-15" hidden={inspectorControls['rf-generated-15'].hidden}>{#if inspectorControls['rf-generated-15'].textContent !== undefined}{inspectorControls['rf-generated-15'].textContent}{:else}
      <section class="panel inspector-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("ДИАГНОСТИКА БЛОКИРОВКИ","BLOCK DIAGNOSTICS")}</span>
            <h2>{rfUi("Почему заблокирован этот IP?","Why is this IP blocked?")}</h2>
            <p>{rfUi("Проверяем известные наборы","Check known sets")} <span class="mono">ascn_*</span> {rfUi("в том же порядке, в котором их учитывает Antiscan.","in the same order Antiscan evaluates them.")}</p>
          </div>
        </div>
        <div class="inspect-form">
          <input id="inspectIp" autocomplete="off" inputmode="decimal" spellcheck="false" placeholder={rfUi("Например, 100.64.10.77","For example, 100.64.10.77")} class={inspectorControls['inspectIp'].className} hidden={inspectorControls['inspectIp'].hidden} disabled={inspectorControls['inspectIp'].disabled} bind:value={inspectorControls['inspectIp'].value} onkeydown={(event) => {if(event.key === "Enter") inspectIP();}}>
          <button id="inspectButton" class={inspectorControls['inspectButton'].className} type="button" hidden={inspectorControls['inspectButton'].hidden} disabled={inspectorControls['inspectButton'].disabled} onclick={inspectIP}>{#if inspectorControls['inspectButton'].textContent !== undefined}{inspectorControls['inspectButton'].textContent}{:else}{rfUi("Проверить IP","Check IP")}{/if}</button>
        </div>
        <div id="inspectResult" class={inspectorControls['inspectResult'].className} hidden={inspectorControls['inspectResult'].hidden}><AntiscanInspectorResult view={inspectorView} emptyText={inspectorEmpty} onExclude={entry => performListEntry('exclude', entry)} /></div>
      </section>

      <section class="panel mobile-help">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("МОБИЛЬНЫЕ СЕТИ","MOBILE NETWORKS")}</span>
            <h2>{rfUi("Почему мобильный адрес может попасть под раздачу","Why a mobile IP may be blocked")}</h2>
          </div>
        </div>
        <div class="explain-grid">
          <article><strong>1</strong><span>{rfUi("Antiscan запоминает адреса-кандидаты.","Antiscan remembers candidate addresses.")}</span></article>
          <article><strong>2</strong><span>{rfUi("Несколько адресов одной /24 могут накопиться за период хранения.","Multiple addresses from the same /24 may accumulate over the retention period.")}</span></article>
          <article><strong>3</strong><span>{rfUi("После порога вся /24 попадает в","Once the threshold is reached, the entire /24 enters")} <span class="mono">ascn_subnets</span>.</span></article>
          <article><strong>4</strong><span>{rfUi("Новый легитимный мобильный IP из этой /24 тоже оказывается заблокирован.","A new legitimate mobile IP within that /24 may then be blocked too.")}</span></article>
        </div>
      </section>
    {/if}</section>

    <section class={blockedControls['rf-generated-16'].className} data-page="blocked" id="rf-generated-16" hidden={blockedControls['rf-generated-16'].hidden}>{#if blockedControls['rf-generated-16'].textContent !== undefined}{blockedControls['rf-generated-16'].textContent}{:else}
      <section class="panel set-browser-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">{rfUi("ТЕКУЩИЕ БЛОКИРОВКИ","CURRENT BLOCKS")}</span>
            <h2>{rfUi("Блокировки и кандидаты","Blocks and candidates")}</h2>
            <p>{rfUi("Реальные записи выбранного набора. Для блокировок Antiscan можно снять только одну конкретную запись; остальные записи не изменяются.","Live entries in the selected set. A single Antiscan block can be removed without changing other entries.")}</p>
          </div>
          <div class="browser-controls">
            <select id="blockedSet" aria-label={rfUi("Выберите Antiscan set","Select an Antiscan set")} class={blockedControls['blockedSet'].className} hidden={blockedControls['blockedSet'].hidden} disabled={blockedControls['blockedSet'].disabled} bind:value={blockedControls['blockedSet'].value} onchange={() => loadSet('blocked')}>
              <option value="ascn_ips">{rfUi("Прямые IP · ascn_ips","Direct IPs · ascn_ips")}</option>
              <option value="ascn_subnets">{rfUi("Подсети /24 · ascn_subnets","/24 subnets · ascn_subnets")}</option>
              <option value="ascn_honeypot">{rfUi("Ловушка · ascn_honeypot","Honeypot · ascn_honeypot")}</option>
              <option value="ascn_ndm_lockout">{rfUi("Блокировка Keenetic · ascn_ndm_lockout","Keenetic lockout · ascn_ndm_lockout")}</option>
              <option value="ascn_candidates">{rfUi("Кандидаты · ascn_candidates","Candidates · ascn_candidates")}</option>
            </select>
            <button id="reloadBlocked" class={blockedControls['reloadBlocked'].className} type="button" hidden={blockedControls['reloadBlocked'].hidden} disabled={blockedControls['reloadBlocked'].disabled} onclick={() => loadSet('blocked')}>{#if blockedControls['reloadBlocked'].textContent !== undefined}{blockedControls['reloadBlocked'].textContent}{:else}{rfUi("Загрузить","Load")}{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="blockedFilter" autocomplete="off" spellcheck="false" placeholder={rfUi("Фильтр по IP или подсети","Filter by IP or subnet")} class={blockedControls['blockedFilter'].className} hidden={blockedControls['blockedFilter'].hidden} disabled={blockedControls['blockedFilter'].disabled} bind:value={blockedControls['blockedFilter'].value} oninput={() => renderSetPage('blocked')}>
          <span id="blockedMeta" class={blockedControls['blockedMeta'].className} hidden={blockedControls['blockedMeta'].hidden}>{#if blockedControls['blockedMeta'].textContent !== undefined}{blockedControls['blockedMeta'].textContent}{:else}—{/if}</span>
        </div>
        <div id="blockedEntries" class={blockedControls['blockedEntries'].className} hidden={blockedControls['blockedEntries'].hidden}><AntiscanSetEntries entries={blockedRows} emptyText={blockedRowsEmpty} onUnban={(set, entry) => performUnban(set, entry)} onExclude={entry => performListEntry('exclude', entry)} /></div>
      </section>

      <section class="panel flush-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">{rfUi("ШТАТНАЯ ОЧИСТКА / ВОССТАНОВЛЕНИЕ","UPSTREAM CLEANUP / RECOVERY")}</span>
            <h2>{rfUi("Очистить набор Antiscan","Clear Antiscan set")}</h2>
            <p>{rfUi("RouterForge вызывает только штатный","RouterForge invokes only the upstream")} <span class="mono">S99ascn flush</span>{rfUi(", заранее показывает затрагиваемые наборы и после операции проверяет фактическое состояние.",", previews affected sets, and verifies actual state after the operation.")}</p>
          </div>
          <div class="browser-controls">
            <select id="flushTarget" aria-label={rfUi("Выберите набор для очистки","Select a set to clear")} class={blockedControls['flushTarget'].className} hidden={blockedControls['flushTarget'].hidden} disabled={blockedControls['flushTarget'].disabled} bind:value={blockedControls['flushTarget'].value} onchange={loadFlushPreview}>
              <option value="ips">{rfUi("Прямые IP","Direct IPs")}</option>
              <option value="subnets">{rfUi("Подсети /24","/24 subnets")}</option>
              <option value="candidates">{rfUi("Кандидаты","Candidates")}</option>
              <option value="honeypot">{rfUi("Ловушка","Honeypot")}</option>
              <option value="ndm_lockout">{rfUi("Блокировки Keenetic","Keenetic lockouts")}</option>
              <option value="custom_blacklist">{rfUi("Пользовательский чёрный список","User blocklist")}</option>
              <option value="custom_whitelist">{rfUi("Пользовательский белый список","User allowlist")}</option>
              <option value="custom_exclude">{rfUi("Пользовательские исключения","User exclusions")}</option>
              <option value="geo">{rfUi("Все Geo-наборы","All Geo sets")}</option>
              <option value="all">{rfUi("Upstream «все»","Upstream «all»")}</option>
            </select>
            <button id="reloadFlushPreview" class={blockedControls['reloadFlushPreview'].className} type="button" hidden={blockedControls['reloadFlushPreview'].hidden} disabled={blockedControls['reloadFlushPreview'].disabled} onclick={loadFlushPreview}>{#if blockedControls['reloadFlushPreview'].textContent !== undefined}{blockedControls['reloadFlushPreview'].textContent}{:else}{rfUi("↻ Проверить","↻ Check")}{/if}</button>
          </div>
        </div>
        <div class="flush-summary">
          <span id="flushState" class={blockedControls['flushState'].className} hidden={blockedControls['flushState'].hidden}>{#if blockedControls['flushState'].textContent !== undefined}{blockedControls['flushState'].textContent}{:else}{rfUi("НЕ ЗАГРУЖЕНО","NOT LOADED")}{/if}</span>
          <div id="flushMeta" class={blockedControls['flushMeta'].className} hidden={blockedControls['flushMeta'].hidden}>{#if blockedControls['flushMeta'].textContent !== undefined}{blockedControls['flushMeta'].textContent}{:else}{rfUi("Выберите цель, чтобы увидеть точные последствия штатной команды flush.","Select a target to preview the effects of the native clear operation.")}{/if}</div>
        </div>
        <div id="flushWarnings" class={blockedControls['flushWarnings'].className} hidden={blockedControls['flushWarnings'].hidden}>{#if flushWarningRows.length}<AntiscanWarningItems messages={flushWarningRows} />{:else}<div class="empty-state">{rfUi("Дополнительных предупреждений для этой цели нет.","No additional warnings for this target.")}</div>{/if}</div>
        <div class="flush-actions">
          <button id="flushSelected" class={blockedControls['flushSelected'].className} type="button" disabled={blockedControls['flushSelected'].disabled} hidden={blockedControls['flushSelected'].hidden} onclick={performFlush}>{#if blockedControls['flushSelected'].textContent !== undefined}{blockedControls['flushSelected'].textContent}{:else}{rfUi("Очистить выбранное","Clear selected")}{/if}</button>
          <span class="muted">{rfUi("Для активных whitelist с опасной комбинацией настроек операция блокируется до изменения конфигурации.","With an active allowlist and potentially unsafe settings, the operation is blocked until the configuration is changed.")}</span>
        </div>
      </section>
    {/if}</section>

    <section class={listsControls['rf-generated-17'].className} data-page="lists" id="rf-generated-17" hidden={listsControls['rf-generated-17'].hidden}>{#if listsControls['rf-generated-17'].textContent !== undefined}{listsControls['rf-generated-17'].textContent}{:else}
      <section class="panel set-browser-panel custom-source-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">{rfUi("ПОЛЬЗОВАТЕЛЬСКИЕ СПИСКИ","USER LISTS")}</span>
            <h2>{rfUi("Исходные списки Antiscan","Antiscan source lists")}</h2>
            <p>{rfUi("Полное управление файлами blacklist, whitelist и exclude. Изменения атомарные; активный список перечитывается штатной командой Antiscan и проверяется по runtime ipset.","Manage blocklist, allowlist, and exclusion files. Changes are atomic; active lists are reloaded using upstream commands and verified against runtime ipset.")}</p>
          </div>
          <div class="browser-controls">
            <select id="customListName" aria-label={rfUi("Выберите пользовательский список","Select a user list")} class={listsControls['customListName'].className} hidden={listsControls['customListName'].hidden} disabled={listsControls['customListName'].disabled} bind:value={listsControls['customListName'].value} onchange={loadCustomList}>
              <option value="blacklist">{rfUi("Чёрный список","Blocklist")}</option>
              <option value="whitelist">{rfUi("Белый список","Allowlist")}</option>
              <option value="exclude">{rfUi("Исключения","Exclusions")}</option>
            </select>
            <button id="reloadCustomList" class={listsControls['reloadCustomList'].className} type="button" hidden={listsControls['reloadCustomList'].hidden} disabled={listsControls['reloadCustomList'].disabled} onclick={loadCustomList}>{#if listsControls['reloadCustomList'].textContent !== undefined}{listsControls['reloadCustomList'].textContent}{:else}{rfUi("Загрузить","Load")}{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="customListEntry" autocomplete="off" spellcheck="false" placeholder={rfUi("IPv4 или CIDR, например 100.64.10.0/24","IPv4 or CIDR, e.g. 100.64.10.0/24")} class={listsControls['customListEntry'].className} hidden={listsControls['customListEntry'].hidden} disabled={listsControls['customListEntry'].disabled} bind:value={listsControls['customListEntry'].value} onkeydown={(event) => {if(event.key === 'Enter') performCustomListMutation('add', listsControls['customListEntry'].value.trim());}}>
          <span class="entry-actions">
            <button id="addCustomListEntry" class={listsControls['addCustomListEntry'].className} type="button" hidden={listsControls['addCustomListEntry'].hidden} disabled={listsControls['addCustomListEntry'].disabled} onclick={() => performCustomListMutation('add', listsControls['customListEntry'].value.trim())}>{#if listsControls['addCustomListEntry'].textContent !== undefined}{listsControls['addCustomListEntry'].textContent}{:else}{rfUi("Добавить адрес","Add address")}{/if}</button>
            <button id="reloadCustomListRuntime" class={listsControls['reloadCustomListRuntime'].className} type="button" hidden={listsControls['reloadCustomListRuntime'].hidden} disabled={listsControls['reloadCustomListRuntime'].disabled} onclick={() => performCustomListMutation('reload')}>{#if listsControls['reloadCustomListRuntime'].textContent !== undefined}{listsControls['reloadCustomListRuntime'].textContent}{:else}{rfUi("Перечитать runtime","Reload service state")}{/if}</button>
            <button id="clearCustomList" class={listsControls['clearCustomList'].className} type="button" hidden={listsControls['clearCustomList'].hidden} disabled={listsControls['clearCustomList'].disabled} onclick={() => performCustomListMutation('clear')}>{#if listsControls['clearCustomList'].textContent !== undefined}{listsControls['clearCustomList'].textContent}{:else}{rfUi("Очистить","Clear")}{/if}</button>
          </span>
        </div>
        <div id="customListMeta" class={listsControls['customListMeta'].className} hidden={listsControls['customListMeta'].hidden}>{#if listsControls['customListMeta'].textContent !== undefined}{listsControls['customListMeta'].textContent}{:else}{rfUi("Выберите список и загрузите его исходный файл.","Select a list and load its source file.")}{/if}</div>
        <div id="customListEntries" class={listsControls['customListEntries'].className} hidden={listsControls['customListEntries'].hidden}><AntiscanSetEntries entries={customRows} emptyText={customRowsEmpty} onDelete={entry => performCustomListMutation('delete', entry)} /></div>
      </section>

      <section class="panel set-browser-panel runtime-lists-panel">
        <div class="panel-head set-browser-head">
          <div>
            <span class="section-kicker">RUNTIME IPSET</span>
            <h2 id="runtimeListsTitle" class={listsControls['runtimeListsTitle'].className} hidden={listsControls['runtimeListsTitle'].hidden}>{#if listsControls['runtimeListsTitle'].textContent !== undefined}{listsControls['runtimeListsTitle'].textContent}{:else}{rfUi("Активные пользовательские списки","Active user lists")}{/if}</h2>
            <p id="runtimeListsDescription" class={listsControls['runtimeListsDescription'].className} hidden={listsControls['runtimeListsDescription'].hidden}>{#if listsControls['runtimeListsDescription'].textContent !== undefined}{listsControls['runtimeListsDescription'].textContent}{:else}{rfUi("Текущее содержимое runtime ipset для blacklist, whitelist и exclude.","Current active ipsets for blocklists, allowlists, and exclusions.")}{/if}</p>
          </div>
          <div class="browser-controls">
            <select id="listSet" aria-label={rfUi("Выберите list set","Select a list set")} class={listsControls['listSet'].className} hidden={listsControls['listSet'].hidden} disabled={listsControls['listSet'].disabled} bind:value={listsControls['listSet'].value} onchange={() => loadSet(currentRuntimeListMode())}>{#each listOptions as option (option[0])}<option value={option[0]}>{option[1]}</option>{/each}</select>
            <button id="reloadList" class={listsControls['reloadList'].className} type="button" hidden={listsControls['reloadList'].hidden} disabled={listsControls['reloadList'].disabled} onclick={() => loadSet(currentRuntimeListMode())}>{#if listsControls['reloadList'].textContent !== undefined}{listsControls['reloadList'].textContent}{:else}{rfUi("Загрузить","Load")}{/if}</button>
          </div>
        </div>
        <div class="browser-toolbar">
          <input id="listFilter" autocomplete="off" spellcheck="false" placeholder={rfUi("Фильтр по IP или подсети","Filter by IP or subnet")} class={listsControls['listFilter'].className} hidden={listsControls['listFilter'].hidden} disabled={listsControls['listFilter'].disabled} bind:value={listsControls['listFilter'].value} oninput={() => renderSetPage(currentRuntimeListMode())}>
          <span id="listMeta" class={listsControls['listMeta'].className} hidden={listsControls['listMeta'].hidden}>{#if listsControls['listMeta'].textContent !== undefined}{listsControls['listMeta'].textContent}{:else}—{/if}</span>
        </div>
        <div id="listEntries" class={listsControls['listEntries'].className} hidden={listsControls['listEntries'].hidden}><AntiscanSetEntries entries={listRows} emptyText={listRowsEmpty} /></div>
      </section>

      <section class="panel geo-service-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">GEO</span>
            <h2>{rfUi("Обновление Geo-данных","Geo data updates")}</h2>
            <p>{rfUi("Штатные операции Antiscan для загрузки Geo-наборов и повторной попытки после ошибки. RouterForge не подменяет upstream-логику.","Upstream Antiscan operations to download Geo sets and retry after errors. RouterForge does not replace upstream behavior.")}</p>
          </div>
          <div class="lifecycle-actions">
            <button class={geoControls['rf-generated-18'].className} type="button" data-operation="update_ipsets" data-scope="geo" id="rf-generated-18" hidden={geoControls['rf-generated-18'].hidden} disabled={geoControls['rf-generated-18'].disabled} onclick={() => performOperation("update_ipsets","geo")}>{#if geoControls['rf-generated-18'].textContent !== undefined}{geoControls['rf-generated-18'].textContent}{:else}{rfUi("Обновить Geo","Update Geo")}{/if}</button>
            <button class={geoControls['rf-generated-19'].className} type="button" data-operation="retry_load_geo" id="rf-generated-19" hidden={geoControls['rf-generated-19'].hidden} disabled={geoControls['rf-generated-19'].disabled} onclick={() => performOperation("retry_load_geo","")}>{#if geoControls['rf-generated-19'].textContent !== undefined}{geoControls['rf-generated-19'].textContent}{:else}{rfUi("Повторить после ошибки","Retry after error")}{/if}</button>
          </div>
        </div>
      </section>
    {/if}</section>

    <section class={protectionControls['rf-generated-20'].className} data-page="protection" id="rf-generated-20" hidden={protectionControls['rf-generated-20'].hidden}>{#if protectionControls['rf-generated-20'].textContent !== undefined}{protectionControls['rf-generated-20'].textContent}{:else}
      <div class="two-column">
        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">{rfUi("ЗАЩИТА","PROTECTION")}</span>
              <h2>{rfUi("Активная политика","Active policy")}</h2>
              <p>{rfUi("Значения прочитаны из","Values read from")} <span class="mono">ascn.conf</span>.</p>
            </div>
          </div>
          <div id="protection" class={protectionControls['protection'].className} hidden={protectionControls['protection'].hidden}><AntiscanKeyValueRows rows={protectionRows} /></div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">{rfUi("РИСКИ","RISKS")}</span>
              <h2>{rfUi("Что стоит проверить","What to check")}</h2>
              <p>{rfUi("Не приговор, а подсказки по потенциально агрессивным сочетаниям параметров.","Warnings about potentially aggressive setting combinations, not definitive failures.")}</p>
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
            <span class="section-kicker">{rfUi("ИСТОРИЯ ОПЕРАЦИЙ","OPERATION HISTORY")}</span>
            <h2>{rfUi("Последние действия","Recent actions")}</h2>
            <p>{rfUi("Ограниченный журнал последних действий: что было сделано, результат, длительность, откат и необходимость перезапуска. Значения формы ascn.conf в журнал не записываются.","A bounded history of recent actions: operation, result, duration, rollback, and whether a restart is needed. Form values from ascn.conf are never logged.")}</p>
          </div>
          <button id="reloadHistory" class={historyControls['reloadHistory'].className} type="button" hidden={historyControls['reloadHistory'].hidden} disabled={historyControls['reloadHistory'].disabled} onclick={loadHistory}>{#if historyControls['reloadHistory'].textContent !== undefined}{historyControls['reloadHistory'].textContent}{:else}{rfUi("↻ Обновить историю","↻ Refresh history")}{/if}</button>
        </div>
        <div class="history-toolbar">
          <span id="historyMeta" class={historyControls['historyMeta'].className} hidden={historyControls['historyMeta'].hidden}>{#if historyControls['historyMeta'].textContent !== undefined}{historyControls['historyMeta'].textContent}{:else}{rfUi("История ещё не загружена.","History has not been loaded yet.")}{/if}</span>
          <span class="state good">{rfUi("JSONL · 0600 · ограниченный размер","JSONL · 0600 · size-limited")}</span>
        </div>
        <div id="historyEntries" class={historyControls['historyEntries'].className} hidden={historyControls['historyEntries'].hidden}><AntiscanHistoryRows entries={historyRows} emptyText={historyRowsEmpty} /></div>
      </section>

      <section class="panel compact-panel">
        <div class="panel-head">
          <div>
            <span class="section-kicker">{rfUi("СОСТАВ ЖУРНАЛА","LOG CONTENTS")}</span>
            <h2>{rfUi("Что попадает в историю","What is recorded")}</h2>
          </div>
        </div>
        <p class="boundary-copy">{rfUi("Журнал хранит только тип операции, безопасную цель, HTTP-код, длительность, факт изменения и проверки, откат, необходимость перезапуска и короткие предупреждения. Тело запроса, значения конфигурации и токены подтверждения не сохраняются.","The log records operation type, safe target, HTTP status, duration, change and verification outcomes, rollback, restart requirements, and short warnings. Request bodies, configuration values, and confirmation tokens are not stored.")}</p>
      </section>
    {/if}</section>

  </main>
