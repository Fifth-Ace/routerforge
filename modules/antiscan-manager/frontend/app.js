const $ = (id) => document.getElementById(id);

const setDescriptions = {
  ascn_candidates: 'Кандидаты для распределённой /24-защиты',
  ascn_ips: 'Прямые IP-блокировки',
  ascn_subnets: 'Заблокированные подсети',
  ascn_custom_exclude: 'Пользовательские исключения',
  ascn_custom_blacklist: 'Пользовательский blacklist',
  ascn_custom_whitelist: 'Пользовательский whitelist',
  ascn_geo_blacklist: 'Geo blacklist',
  ascn_geo_whitelist: 'Geo whitelist',
  ascn_geo_exclude: 'Geo исключения',
  ascn_ndm_lockout: 'Keenetic lockout-policy',
  ascn_honeypot: 'Honeypot'
};

const browserState = {
  blocked: null,
  lists: null
};

let snapshot = null;
let lifecycleBusy = false;
let configBusy = false;
let configDirty = false;
let configBaseline = '';
let configBaseSHA = '';
let historyLoaded = false;

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

function setOperationalExplanation(name) {
  return {
    ascn_candidates: 'Кандидат сам по себе не блокирует IP; несколько адресов одной /24 могут позже продвинуть всю подсеть в ascn_subnets.',
    ascn_ips: 'Прямой ban-set. Upstream не хранит, был ли точным trigger recent-hitcount или concurrent limit.',
    ascn_subnets: 'Блокируется вся /24 после накопления разных IP-кандидатов; особенно внимательно для мобильных и вращающихся пулов.',
    ascn_honeypot: 'IP попал в ловушку на одном из HONEYPOT_PORTS.',
    ascn_ndm_lockout: 'Запись импортирована из Keenetic ip lockout-policy; direct unban здесь намеренно не предлагается.',
    ascn_custom_exclude: 'Пользовательское исключение имеет приоритет перед blocking rules.',
    ascn_custom_blacklist: 'Активный пользовательский blacklist при CUSTOM_LISTS_BLOCK_MODE=blacklist.',
    ascn_custom_whitelist: 'В whitelist-mode отсутствие адреса в этом set означает блокировку.',
    ascn_geo_blacklist: 'Подсеть страны из активного Geo blacklist.',
    ascn_geo_whitelist: 'В Geo whitelist-mode отсутствие подсети в этом set означает блокировку.',
    ascn_geo_exclude: 'Geo-исключение имеет приоритет перед blocking rules.'
  }[name] || 'Runtime member Antiscan.';
}

function reasonExplanation(result) {
  const descriptions = {
    'custom-exclude': 'IP найден в пользовательских исключениях. Antiscan возвращает трафик раньше blocking rules.',
    'geo-exclude': 'IP попал в Geo-исключение и не должен блокироваться последующими правилами.',
    'custom-blacklist': 'IP совпал с активным пользовательским blacklist.',
    'custom-whitelist-miss': 'Включён whitelist-mode, но IP отсутствует в разрешённом пользовательском set.',
    'geo-blacklist': 'IP относится к подсети страны из активного Geo blacklist.',
    'geo-whitelist-miss': 'Включён Geo whitelist-mode, но IP не относится к разрешённым Geo-подсетям.',
    'ndm-lockout': 'IP импортирован из Keenetic ip lockout-policy.',
    honeypot: 'IP находится в honeypot ban-set после обращения к порту-ловушке.',
    'distributed-subnet': 'Заблокирована вся /24: Antiscan накопил порог разных IP-кандидатов из одной подсети.',
    'direct-ip': 'IP находится в прямом ban-set. Upstream не сохраняет, что именно сработало: recent-hitcount или concurrent limit.',
    'candidate-only': 'IP пока только кандидат для /24-анализа и этим set сам по себе не блокируется.',
    'no-active-set-match': 'Совпадений с активными blocking sets не найдено.'
  };
  return descriptions[result?.reason] || '';
}

function configNumber(id) {
  const raw = $(id).value.trim();
  return raw === '' ? 0 : Number(raw);
}

function currentConfigFormPayload() {
  return {
    isp_interfaces: $('cfgIspInterfaces').value.trim(),
    ports: $('cfgPorts').value.trim(),
    forwarded_ports: $('cfgForwardedPorts').value.trim(),
    enable_honeypot: $('cfgEnableHoneypot').checked,
    honeypot_ports: $('cfgHoneypotPorts').value.trim(),
    honeypot_bantime_seconds: configNumber('cfgHoneypotBanTime'),
    enable_ips_ban: $('cfgEnableIpsBan').checked,
    rules_mask: $('cfgRulesMask').value.trim(),
    recent_connections_time_seconds: configNumber('cfgRecentTime'),
    recent_connections_hitcount: configNumber('cfgRecentHitcount'),
    recent_connections_limit: configNumber('cfgRecentLimit'),
    recent_connections_bantime_seconds: configNumber('cfgRecentBanTime'),
    different_ip_candidates_storage_seconds: configNumber('cfgCandidateStorage'),
    different_ip_threshold: configNumber('cfgDifferentThreshold'),
    subnets_bantime_seconds: configNumber('cfgSubnetBanTime'),
    ipsets_directory: $('cfgIPSetsDirectory').value.trim(),
    save_ipsets: $('cfgSaveIPSets').checked,
    save_on_exit: $('cfgSaveOnExit').checked,
    use_custom_exclude_list: $('cfgUseCustomExclude').checked,
    custom_lists_block_mode: $('cfgCustomMode').value,
    geoblock_mode: $('cfgGeoMode').value,
    geoblock_countries: $('cfgGeoCountries').value.trim(),
    geo_exclude_countries: $('cfgGeoExcludeCountries').value.trim(),
    read_ndm_lockout_ipsets: $('cfgReadNDM').checked,
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
  $('cfgIspInterfaces').value = cfg.isp_interfaces;
  $('cfgPorts').value = cfg.ports;
  $('cfgForwardedPorts').value = cfg.forwarded_ports;
  $('cfgEnableHoneypot').checked = cfg.enable_honeypot;
  $('cfgHoneypotPorts').value = cfg.honeypot_ports;
  $('cfgHoneypotBanTime').value = cfg.honeypot_bantime_seconds;
  $('cfgEnableIpsBan').checked = cfg.enable_ips_ban;
  $('cfgRulesMask').value = cfg.rules_mask;
  $('cfgRecentTime').value = cfg.recent_connections_time_seconds;
  $('cfgRecentHitcount').value = cfg.recent_connections_hitcount;
  $('cfgRecentLimit').value = cfg.recent_connections_limit;
  $('cfgRecentBanTime').value = cfg.recent_connections_bantime_seconds;
  $('cfgCandidateStorage').value = cfg.different_ip_candidates_storage_seconds;
  $('cfgDifferentThreshold').value = cfg.different_ip_threshold;
  $('cfgSubnetBanTime').value = cfg.subnets_bantime_seconds;
  $('cfgIPSetsDirectory').value = cfg.ipsets_directory;
  $('cfgSaveIPSets').checked = cfg.save_ipsets;
  $('cfgSaveOnExit').checked = cfg.save_on_exit;
  $('cfgUseCustomExclude').checked = cfg.use_custom_exclude_list;
  $('cfgCustomMode').value = cfg.custom_lists_block_mode;
  $('cfgGeoMode').value = cfg.geoblock_mode;
  $('cfgGeoCountries').value = cfg.geoblock_countries;
  $('cfgGeoExcludeCountries').value = cfg.geo_exclude_countries;
  $('cfgReadNDM').checked = cfg.read_ndm_lockout_ipsets;
  $('cfgLockoutBanTime').value = cfg.lockout_ipset_bantime_seconds;
}

function populateConfigEditor(force = false) {
  const form = $('configForm');
  if (!form) return;
  if (!snapshot?.detected || !snapshot?.config) {
    $('configState').className = 'state neutral';
    $('configState').textContent = 'UNAVAILABLE';
    $('configHint').textContent = 'Antiscan или ascn.conf не обнаружен.';
    $('applyConfig').disabled = true;
    $('resetConfig').disabled = true;
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
  if (!$('configForm')) return;
  const current = currentConfigFormPayload();
  configDirty = Boolean(configBaseline) && JSON.stringify(current) !== configBaseline;
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const stale = Boolean(configDirty && configBaseSHA && snapshot?.config_sha256 && snapshot.config_sha256 !== configBaseSHA);

  const state = $('configState');
  const hint = $('configHint');
  state.className = 'state';
  if (configBusy) {
    state.classList.add('info');
    state.textContent = 'APPLYING…';
    hint.textContent = 'Backup → atomic write → upstream reload → verify. При ошибке выполняется rollback.';
  } else if (stale) {
    state.classList.add('bad');
    state.textContent = 'STALE';
    hint.textContent = 'ascn.conf изменился после открытия формы. Сбрось форму к свежему snapshot перед применением.';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'UPSTREAM BUSY';
    hint.textContent = 'Сейчас идёт config/Geo reload. Применение временно заблокировано.';
  } else if (configDirty) {
    state.classList.add('warn');
    state.textContent = 'CHANGED';
    hint.textContent = snapshot?.running
      ? 'Изменения будут применены транзакционно через S99ascn reload.'
      : 'Antiscan остановлен: файл будет сохранён и проверен, а настройки активируются при следующем start.';
  } else {
    state.classList.add('good');
    state.textContent = 'SYNCED';
    hint.textContent = snapshot?.running ? 'Форма соответствует активному ascn.conf.' : 'Форма соответствует сохранённому ascn.conf.';
  }

  $('configHash').textContent = configBaseSHA ? `SHA256 ${configBaseSHA.slice(0, 16)}…` : 'SHA256 —';
  $('applyConfig').disabled = configBusy || !configDirty || !configBaseSHA || upstreamBusy || stale;
  $('resetConfig').disabled = configBusy || !configDirty;

  const mobileRisk = current.enable_ips_ban && current.different_ip_threshold > 0 &&
    current.different_ip_threshold <= 5 && current.different_ip_candidates_storage_seconds >= 86400;
  const risk = $('configMobileRisk');
  risk.className = `config-risk ${mobileRisk ? 'warn' : 'good'}`;
  risk.textContent = mobileRisk
    ? `⚠ /24: порог ${current.different_ip_threshold}, кандидаты ${fmtDuration(current.different_ip_candidates_storage_seconds)} — для мобильных пулов настройка агрессивная.`
    : '✓ Текущие /24 threshold/retention не попадают под встроенный mobile-risk профиль.';
}

async function applyConfigEditor() {
  if (!configDirty || !configBaseSHA) return;
  const current = currentConfigFormPayload();
  const warning = snapshot?.running
    ? 'Будет создан backup, ascn.conf запишется атомарно, затем Antiscan выполнит штатный reload. При ошибке RouterForge вернёт предыдущий файл и попытается восстановить runtime.'
    : 'Antiscan остановлен. Будет создан backup и атомарно сохранён ascn.conf; runtime применится при следующем запуске.';
  if (!window.confirm(`Применить изменения ascn.conf?\n\n${warning}`)) return;

  configBusy = true;
  updateConfigEditorState();
  try {
    const result = await mutate('config', { ...current, base_sha256: configBaseSHA, confirm: 'APPLY_CONFIG' });
    showMutationResult(result);
    if (result.restart_required) {
      const notice = $('actionNotice');
      notice.textContent += ' · IPSETS_DIRECTORY изменён: upstream требует stop/start для полного перехода на новый каталог.';
    }
    configDirty = false;
    configBaseline = '';
    await loadStatus();
    populateConfigEditor(true);
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true);
    await loadStatus();
  } finally {
    configBusy = false;
    updateConfigEditorState();
  }
}

function activateTab(name) {
  document.querySelectorAll('.tab').forEach((button) => {
    button.classList.toggle('active', button.dataset.tab === name);
  });
  document.querySelectorAll('.tab-page').forEach((page) => {
    page.classList.toggle('active', page.dataset.page === name);
  });
}

function renderState() {
  const state = $('targetState');
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

  $('version').textContent = snapshot?.version || '—';
  $('blockedIps').textContent = countFor('ascn_ips');
  $('blockedSubnets').textContent = countFor('ascn_subnets');
  $('candidateCount').textContent = countFor('ascn_candidates');
  $('ipsetBinary').textContent = `ipset: ${snapshot?.ipset_binary || 'не найден'}`;

  const notice = $('notice');
  if (!snapshot?.detected) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan не обнаружен. Manager не устанавливает upstream-пакет автоматически.';
  } else if (!snapshot.running) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan обнаружен, но /tmp/ascn.run отсутствует. Runtime-состояние блокировок нельзя считать авторитетным.';
  } else if ((snapshot.errors || []).length) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = snapshot.errors.join(' · ');
  } else {
    notice.hidden = true;
  }

  renderLifecycleControls();
}

function renderLifecycleControls() {
  const start = $('startAntiscan');
  const stop = $('stopAntiscan');
  const reload = $('reloadAntiscan');
  const state = $('lifecycleState');
  const hint = $('lifecycleHint');
  if (!start || !stop || !reload || !state || !hint) return;

  const detected = Boolean(snapshot?.detected);
  const running = Boolean(snapshot?.running);
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);

  start.disabled = lifecycleBusy || configBusy || !detected || running || upstreamBusy;
  stop.disabled = lifecycleBusy || configBusy || !detected || !running || upstreamBusy;
  reload.disabled = lifecycleBusy || configBusy || !detected || !running || upstreamBusy;

  state.className = 'state';
  if (lifecycleBusy) {
    state.classList.add('info');
    state.textContent = 'ОПЕРАЦИЯ…';
    hint.textContent = 'Ждём завершения штатной команды Antiscan и post-action verification.';
  } else if (!detected) {
    state.classList.add('neutral');
    state.textContent = 'UNAVAILABLE';
    hint.textContent = 'Antiscan не обнаружен — lifecycle-команды недоступны.';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'UPSTREAM BUSY';
    hint.textContent = 'Antiscan уже выполняет config/Geo reload. Новая mutation заблокирована.';
  } else if (running) {
    state.classList.add('good');
    state.textContent = 'RUNNING';
    hint.textContent = 'Можно выполнить reload текущего ascn.conf или штатно остановить Antiscan.';
  } else {
    state.classList.add('warn');
    state.textContent = 'STOPPED';
    hint.textContent = 'Можно запустить Antiscan штатной командой S99ascn start.';
  }
}

function renderRuntimeSummary() {
  const cfg = snapshot?.config || {};
  const rows = [
    ['Antiscan', snapshot?.detected ? 'Обнаружен' : 'Не обнаружен'],
    ['Runtime marker', snapshot?.running ? 'Есть · /tmp/ascn.run' : 'Нет'],
    ['Config reload', snapshot?.config_reload_in_progress ? 'Выполняется' : 'Нет'],
    ['Geo reload', snapshot?.geo_reload_in_progress ? 'Выполняется' : 'Нет'],
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Защищаемые порты', (cfg.ports || []).join(', ') || '—']
  ];
  $('runtimeSummary').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');
}

function renderRiskSummary() {
  const p = snapshot?.protection || {};
  const risk = p.ips_ban_enabled && Number(p.different_ip_threshold || 0) > 0 &&
    Number(p.different_ip_threshold || 0) <= 5 && Number(p.candidate_storage_seconds || 0) >= 86400;
  const root = $('riskSummary');
  if (!snapshot?.detected) {
    root.innerHTML = '<div class="risk-empty">Antiscan не обнаружен — анализировать нечего.</div>';
    return;
  }
  root.innerHTML = `
    <div class="risk-verdict ${risk ? 'warn' : 'good'}">
      <span>${risk ? '!' : '✓'}</span>
      <div>
        <strong>${risk ? 'Есть риск ложной /24-блокировки' : 'Явного /24 risk-pattern не видно'}</strong>
        <p>${risk
          ? `Порог ${escapeHTML(String(p.different_ip_threshold))} адресов сочетается с хранением кандидатов ${escapeHTML(fmtDuration(p.candidate_storage_seconds))}. Для мобильных пулов это может быть агрессивно.`
          : 'Текущая комбинация threshold/retention не попала под встроенный предупреждающий профиль.'}</p>
      </div>
    </div>
    <div class="risk-pairs">
      <div><span>Порог /24</span><strong>${escapeHTML(String(p.different_ip_threshold || '—'))}</strong></div>
      <div><span>Кандидаты живут</span><strong>${escapeHTML(fmtDuration(p.candidate_storage_seconds))}</strong></div>
      <div><span>Бан подсети</span><strong>${escapeHTML(fmtDuration(p.subnet_ban_seconds))}</strong></div>
      <div><span>Порог NEW</span><strong>${escapeHTML(String(p.recent_hitcount || '—'))}</strong></div>
    </div>
  `;
}

function renderProtection() {
  const p = snapshot?.protection || {};
  const cfg = snapshot?.config || {};
  const rows = [
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Порты роутера', (cfg.ports || []).join(', ') || '—'],
    ['Forwarded ports', (cfg.forwarded_ports || []).join(', ') || '—'],
    ['IP / subnet protection', boolText(Boolean(p.ips_ban_enabled))],
    ['Окно новых соединений', fmtDuration(p.recent_window_seconds)],
    ['Порог NEW', p.recent_hitcount || '—'],
    ['Concurrent limit', p.concurrent_connection_limit || '—'],
    ['Direct IP ban', fmtDuration(p.direct_ip_ban_seconds)],
    ['Порог разных IP в /24', p.different_ip_threshold || '—'],
    ['Хранение кандидатов', fmtDuration(p.candidate_storage_seconds)],
    ['Subnet ban', fmtDuration(p.subnet_ban_seconds)],
    ['Honeypot', boolText(Boolean(p.honeypot_enabled))],
    ['Honeypot ports', (cfg.honeypot_ports || []).join(', ') || '—'],
    ['Custom exclude', boolText(Boolean(cfg.use_custom_exclude_list))],
    ['Custom list mode', cfg.custom_lists_block_mode || '0'],
    ['Geo mode', cfg.geoblock_mode || '0'],
    ['Geo countries', (cfg.geoblock_countries || []).join(', ') || '—'],
    ['Geo exclude countries', (cfg.geo_exclude_countries || []).join(', ') || '—'],
    ['NDM lockout import', boolText(Boolean(cfg.read_ndm_lockout_ipsets))],
    ['Save ipsets', boolText(Boolean(cfg.save_ipsets))]
  ];
  $('protection').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');
}

function renderWarnings() {
  const warnings = [...(snapshot?.warnings || []), ...(snapshot?.errors || [])];
  if (!warnings.length) {
    $('warnings').innerHTML = '<div class="ok-note">Явных risk-сигналов по текущему read-only snapshot нет.</div>';
    return;
  }
  $('warnings').innerHTML = warnings.map((warning) => `
    <div class="warning-item"><span>!</span><p>${escapeHTML(warning)}</p></div>
  `).join('');
}

function renderSets() {
  const sets = snapshot?.ipsets || [];
  $('sets').innerHTML = sets.map((item) => {
    const state = item.exists ? (item.error ? 'PARTIAL' : 'ACTIVE') : 'ABSENT';
    const cls = item.exists ? (item.error ? 'warn' : 'good') : 'neutral';
    const count = item.count_known ? String(item.count ?? 0) : '—';
    return `<tr>
      <td class="mono strong">${escapeHTML(item.name)}</td>
      <td>${escapeHTML(setDescriptions[item.name] || 'Antiscan runtime set')}</td>
      <td><span class="state ${cls}">${state}</span>${item.error ? `<small class="row-error">${escapeHTML(item.error)}</small>` : ''}</td>
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
  const root = $('inspectResult');
  const cls = result.blocked ? 'bad' : result.conclusive ? 'good' : 'warn';
  const evidence = (result.evidence || []).map((item) => `
    <div class="evidence-row">
      <div><strong>${escapeHTML(item.kind)}</strong><span class="mono">${escapeHTML(item.set || '')}</span></div>
      <p>${escapeHTML(item.summary || '')}</p>
    </div>
  `).join('');
  const warnings = (result.warnings || []).map((item) => `<div class="inspect-warning">${escapeHTML(item)}</div>`).join('');
  const explanation = reasonExplanation(result);
  const actions = result.ip && (result.blocked || result.verdict === 'candidate') ? `
    <div class="inspect-actions">
      <button class="button small" type="button" data-action="exclude" data-entry="${escapeHTML(result.ip)}">Добавить IP в исключения</button>
    </div>` : '';
  root.className = `inspect-result ${cls}`;
  root.innerHTML = `
    <div class="inspect-verdict">
      <div><span class="mono">${escapeHTML(result.ip || '')}</span><strong>${verdictTitle(result)}</strong></div>
      <span class="state ${cls}">${escapeHTML(result.reason || result.verdict || 'unknown')}</span>
    </div>
    ${explanation ? `<div class="reason-explanation">${escapeHTML(explanation)}</div>` : ''}
    ${evidence || '<div class="muted inspect-empty">Совпадения в доступных runtime sets не найдены.</div>'}
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
  if (entry.packets_known) bits.push(`${entry.packets} pkt`);
  if (entry.bytes_known) bits.push(fmtBytes(entry.bytes));
  return bits.join(' · ') || 'runtime member';
}

function canUnbanSet(name) {
  return ['ascn_ips', 'ascn_subnets', 'ascn_honeypot'].includes(name);
}

function blockedEntryActions(setName, entry) {
  const buttons = [];
  if (canUnbanSet(setName)) {
    buttons.push(`<button class="button small danger" type="button" data-action="unban" data-set="${escapeHTML(setName)}" data-entry="${escapeHTML(entry.value)}">Снять бан</button>`);
  }
  if (['ascn_ips', 'ascn_subnets', 'ascn_honeypot', 'ascn_ndm_lockout', 'ascn_candidates'].includes(setName)) {
    buttons.push(`<button class="button small" type="button" data-action="exclude" data-entry="${escapeHTML(entry.value)}">В исключения</button>`);
  }
  return buttons.length ? `<span class="entry-actions">${buttons.join('')}</span>` : '';
}

function renderSetPage(kind) {
  const page = browserState[kind];
  const target = kind === 'blocked' ? $('blockedEntries') : $('listEntries');
  const meta = kind === 'blocked' ? $('blockedMeta') : $('listMeta');
  const filter = (kind === 'blocked' ? $('blockedFilter').value : $('listFilter').value).trim().toLowerCase();

  if (!page) {
    target.className = 'entry-table empty-state';
    target.textContent = 'Набор ещё не загружен.';
    meta.textContent = '—';
    return;
  }
  if (page.error) {
    target.className = 'entry-table empty-state bad-text';
    target.textContent = page.error;
    meta.textContent = page.name || '—';
    return;
  }
  if (!page.exists) {
    target.className = 'entry-table empty-state';
    target.textContent = `${page.name}: набор сейчас отсутствует.`;
    meta.textContent = 'ABSENT';
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
    const actions = kind === 'blocked' ? blockedEntryActions(page.name, entry) : '';
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
  const select = kind === 'blocked' ? $('blockedSet') : $('listSet');
  const button = kind === 'blocked' ? $('reloadBlocked') : $('reloadList');
  const target = kind === 'blocked' ? $('blockedEntries') : $('listEntries');
  button.disabled = true;
  target.className = 'entry-table empty-state';
  target.textContent = 'Читаем runtime ipset…';
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

function showMutationResult(payload, failed = false) {
  const notice = $('actionNotice');
  notice.hidden = false;
  notice.className = `notice action-notice ${failed ? 'bad' : 'good'}`;
  if (failed) {
    notice.textContent = payload?.error || payload?.message || 'Guarded action failed.';
    return;
  }
  const warnings = payload?.warnings || [];
  const state = payload?.changed ? 'Изменение применено и проверено.' : 'Состояние уже соответствовало запросу.';
  notice.textContent = [state, ...warnings].join(' · ');
}

async function performUnban(setName, entry) {
  if (!window.confirm(`Снять только эту запись из ${setName}?\n\n${entry}\n\nМассовый flush не выполняется.`)) return;
  try {
    const result = await mutate('unban', { set: setName, entry, confirm: 'UNBAN' });
    showMutationResult(result);
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true);
  }
}

async function performListEntry(listName, entry) {
  const target = listName === 'exclude' ? 'Custom exclude' : 'Custom whitelist';
  if (!entry) return;
  if (!window.confirm(`Добавить запись в ${target}?\n\n${entry}\n\nФайл будет изменён атомарно; активный список затем перечитается штатным Antiscan.`)) return;
  try {
    const result = await mutate('list-entry', { list: listName, entry, confirm: 'ADD' });
    showMutationResult(result);
    await loadStatus();
    if (browserState.lists) await loadSet('lists');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true);
  }
}

async function performLifecycle(action) {
  const prompts = {
    start: 'Запустить Antiscan штатной командой S99ascn start?\n\nUpstream создаст свои ipset и firewall rules.',
    stop: 'Остановить Antiscan штатной командой S99ascn stop?\n\nUpstream удалит свои active rules/ipset. SAVE_ON_EXIT обрабатывается самим Antiscan.',
    reload: 'Перечитать текущий ascn.conf штатной командой S99ascn reload?\n\nUpstream может перестроить rules/ipset. RouterForge проверит runtime marker после завершения.'
  };
  if (!prompts[action] || !window.confirm(prompts[action])) return;

  lifecycleBusy = true;
  renderLifecycleControls();
  try {
    const result = await mutate('lifecycle', { action, confirm: action.toUpperCase() });
    showMutationResult(result);
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true);
  } finally {
    lifecycleBusy = false;
    renderLifecycleControls();
  }
}

function auditActionTitle(action) {
  return {
    unban: 'Single-entry unban',
    'list-entry': 'Custom list update',
    'lifecycle:start': 'Start',
    'lifecycle:stop': 'Stop',
    'lifecycle:reload': 'Reload',
    config: 'Transactional config'
  }[action] || action || 'Guarded action';
}

function renderHistory(page) {
  const events = page?.events || [];
  const target = $('historyEntries');
  const meta = $('historyMeta');
  historyLoaded = true;
  meta.textContent = `${events.length} событий · последние сверху${page?.skipped_invalid_lines ? ` · пропущено повреждённых строк: ${page.skipped_invalid_lines}` : ''}`;

  if (!events.length) {
    target.className = 'history-list empty-state';
    target.textContent = 'Guarded-действий в bounded audit пока нет.';
    return;
  }

  target.className = 'history-list';
  target.innerHTML = events.map((event) => {
    const failed = event.outcome !== 'success';
    const flags = [];
    if (event.changed) flags.push('<span class="history-flag">changed</span>');
    if (event.verified) flags.push('<span class="history-flag good">verified</span>');
    if (event.rollback) flags.push('<span class="history-flag bad">rollback</span>');
    if (event.restart_required) flags.push('<span class="history-flag warn">restart required</span>');
    const warnings = (event.warnings || []).map((item) => `<small>⚠ ${escapeHTML(item)}</small>`).join('');
    return `
      <article class="history-row ${failed ? 'failed' : ''}">
        <div class="history-main">
          <div class="history-title">
            <span class="state ${failed ? 'bad' : 'good'}">${failed ? 'FAIL' : 'PASS'}</span>
            <strong>${escapeHTML(auditActionTitle(event.action))}</strong>
            <span class="mono muted">${escapeHTML(fmtAuditTime(event.timestamp))}</span>
          </div>
          <p>${escapeHTML(event.summary || '—')}</p>
          ${event.target ? `<span class="mono history-target">${escapeHTML(event.target)}</span>` : ''}
          ${warnings}
        </div>
        <div class="history-side">
          <span class="mono">${Number(event.duration_ms || 0)} ms</span>
          <span class="mono">HTTP ${escapeHTML(String(event.http_status || '—'))}</span>
          <div class="history-flags">${flags.join('')}</div>
        </div>
      </article>`;
  }).join('');
}

async function loadHistory() {
  const button = $('reloadHistory');
  if (button) button.disabled = true;
  try {
    const page = await api('history?limit=50');
    renderHistory(page);
  } catch (error) {
    historyLoaded = true;
    $('historyMeta').textContent = 'Ошибка чтения history';
    $('historyEntries').className = 'history-list empty-state bad-text';
    $('historyEntries').textContent = error.message || 'Не удалось прочитать bounded audit.';
  } finally {
    if (button) button.disabled = false;
  }
}

async function loadStatus() {
  $('refresh').disabled = true;
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
    const notice = $('notice');
    notice.hidden = false;
    notice.className = 'notice bad';
    notice.textContent = error.message || 'Не удалось получить статус Antiscan Manager.';
  } finally {
    $('refresh').disabled = false;
  }
}

async function inspectIP() {
  const ip = $('inspectIp').value.trim();
  if (!ip) return;
  $('inspectButton').disabled = true;
  $('inspectResult').className = 'inspect-result empty';
  $('inspectResult').textContent = 'Проверяем runtime state…';
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
      warnings: payload.warnings || [error.message || 'Ошибка проверки']
    });
  } finally {
    $('inspectButton').disabled = false;
  }
}

document.querySelectorAll('.tab').forEach((button) => {
  button.addEventListener('click', () => {
    activateTab(button.dataset.tab);
    if (button.dataset.tab === 'blocked' && !browserState.blocked) loadSet('blocked');
    if (button.dataset.tab === 'lists' && !browserState.lists) loadSet('lists');
    if (button.dataset.tab === 'history' && !historyLoaded) loadHistory();
  });
});

$('refresh').addEventListener('click', async () => {
  await loadStatus();
  if (browserState.blocked) await loadSet('blocked');
  if (browserState.lists) await loadSet('lists');
  if (historyLoaded) await loadHistory();
});
$('inspectButton').addEventListener('click', inspectIP);
$('inspectIp').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') inspectIP();
});
$('reloadBlocked').addEventListener('click', () => loadSet('blocked'));
$('reloadList').addEventListener('click', () => loadSet('lists'));
$('blockedSet').addEventListener('change', () => loadSet('blocked'));
$('listSet').addEventListener('change', () => loadSet('lists'));
$('blockedFilter').addEventListener('input', () => renderSetPage('blocked'));
$('listFilter').addEventListener('input', () => renderSetPage('lists'));
$('startAntiscan').addEventListener('click', () => performLifecycle('start'));
$('reloadAntiscan').addEventListener('click', () => performLifecycle('reload'));
$('stopAntiscan').addEventListener('click', () => performLifecycle('stop'));
$('reloadHistory').addEventListener('click', loadHistory);
$('configForm').addEventListener('submit', (event) => event.preventDefault());
$('configForm').addEventListener('input', updateConfigEditorState);
$('configForm').addEventListener('change', updateConfigEditorState);
$('resetConfig').addEventListener('click', () => {
  configDirty = false;
  populateConfigEditor(true);
});
$('applyConfig').addEventListener('click', applyConfigEditor);
$('addListEntry').addEventListener('click', () => performListEntry($('listAction').value, $('listActionEntry').value.trim()));
$('listActionEntry').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') performListEntry($('listAction').value, $('listActionEntry').value.trim());
});
document.addEventListener('click', (event) => {
  const button = event.target.closest('[data-action]');
  if (!button) return;
  const action = button.dataset.action;
  const entry = button.dataset.entry || '';
  if (action === 'unban') performUnban(button.dataset.set || '', entry);
  if (action === 'exclude') performListEntry('exclude', entry);
});

loadStatus();
