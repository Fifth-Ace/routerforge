const $ = (id) => document.getElementById(id);

const setDescriptions = {
  ascn_candidates: 'Кандидаты для /24',
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

let snapshot = null;

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

function fmtDuration(seconds) {
  const value = Number(seconds || 0);
  if (!value) return '—';
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
  $('honeypotCount').textContent = countFor('ascn_honeypot');
  $('honeypotMode').textContent = snapshot?.protection?.honeypot_enabled ? 'включена' : 'выключена';
  $('ipsetBinary').textContent = `ipset: ${snapshot?.ipset_binary || 'не найден'}`;

  const notice = $('notice');
  if (!snapshot?.detected) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan не обнаружен. RouterForge Antiscan Manager не устанавливает upstream-пакет автоматически.';
  } else if (!snapshot.running) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan обнаружен, но /tmp/ascn.run отсутствует. Runtime-данные не считаются авторитетными.';
  } else if ((snapshot.errors || []).length) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = snapshot.errors.join(' · ');
  } else {
    notice.hidden = true;
  }
}

function renderProtection() {
  const p = snapshot?.protection || {};
  const cfg = snapshot?.config || {};
  const rows = [
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Порты роутера', (cfg.ports || []).join(', ') || '—'],
    ['Forwarded ports', (cfg.forwarded_ports || []).join(', ') || '—'],
    ['IP / subnet protection', p.ips_ban_enabled ? 'Включена' : 'Выключена'],
    ['Окно новых соединений', fmtDuration(p.recent_window_seconds)],
    ['Порог NEW', p.recent_hitcount || '—'],
    ['Concurrent limit', p.concurrent_connection_limit || '—'],
    ['Direct IP ban', fmtDuration(p.direct_ip_ban_seconds)],
    ['Порог разных IP в /24', p.different_ip_threshold || '—'],
    ['Хранение кандидатов', fmtDuration(p.candidate_storage_seconds)],
    ['Subnet ban', fmtDuration(p.subnet_ban_seconds)],
    ['Custom lists', cfg.custom_lists_block_mode || '0'],
    ['Geo', cfg.geoblock_mode || '0']
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
  root.className = `inspect-result ${cls}`;
  root.innerHTML = `
    <div class="inspect-verdict">
      <div><span class="mono">${escapeHTML(result.ip || '')}</span><strong>${verdictTitle(result)}</strong></div>
      <span class="state ${cls}">${escapeHTML(result.reason || result.verdict || 'unknown')}</span>
    </div>
    ${evidence || '<div class="muted inspect-empty">Совпадения в доступных runtime sets не найдены.</div>'}
    ${warnings}
  `;
}

async function loadStatus() {
  $('refresh').disabled = true;
  try {
    snapshot = await api('status');
    renderState();
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

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>'"]/g, (char) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  })[char]);
}

$('refresh').addEventListener('click', loadStatus);
$('inspectButton').addEventListener('click', inspectIP);
$('inspectIp').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') inspectIP();
});

loadStatus();
