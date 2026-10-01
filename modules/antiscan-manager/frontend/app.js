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

  start.disabled = lifecycleBusy || !detected || running || upstreamBusy;
  stop.disabled = lifecycleBusy || !detected || !running || upstreamBusy;
  reload.disabled = lifecycleBusy || !detected || !running || upstreamBusy;

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
    ${evidence || '<div class="muted inspect-empty">Совпадения в доступных runtime sets не найдены.</div>'}
    ${warnings}
    ${actions}
  `;
}

function entrySecondary(entry) {
  const bits = [];
  if (entry.timeout_known) bits.push(`timeout ${fmtDuration(entry.timeout_seconds)}`);
  if (entry.packets_known) bits.push(`${entry.packets} pkt`);
  if (entry.bytes_known) bits.push(`${entry.bytes} B`);
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
        <span class="entry-meta">${escapeHTML(entrySecondary(entry))}</span>
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
  });
});

$('refresh').addEventListener('click', async () => {
  await loadStatus();
  if (browserState.blocked) await loadSet('blocked');
  if (browserState.lists) await loadSet('lists');
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
