export const clamp = (n, a, b) => Math.max(a, Math.min(b, Number(n || 0)));

export const fmtInt = (n) => new Intl.NumberFormat('ru-RU').format(Number(n || 0));
export const fmtPct = (n) => `${Number(n || 0).toFixed(Number(n || 0) >= 10 ? 1 : 2)}%`;
export const fmtMs = (n) => Number(n || 0) > 0 ? `${Math.round(Number(n))} ms` : '—';

export function fmtAgo(iso) {
  if (!iso || String(iso).startsWith('0001-')) return '—';
  const s = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (s < 5) return 'только что';
  if (s < 60) return `${s} сек назад`;
  if (s < 3600) return `${Math.floor(s / 60)} мин назад`;
  if (s < 86400) return `${Math.floor(s / 3600)} ч назад`;
  return `${Math.floor(s / 86400)} дн назад`;
}

export function fmtDuration(sec) {
  sec = Math.max(0, Number(sec || 0));
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  return [d ? `${d}д` : null, h ? `${h}ч` : null, m ? `${m}м` : null].filter(Boolean).join(' ') || `${Math.floor(sec)}с`;
}

export function bytes(n) {
  n = Number(n || 0);
  if (n < 1024) return `${n} B`;
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)} MB`;
  return `${(n / 1073741824).toFixed(2)} GB`;
}

export function timeOnly(iso) {
  try {
    return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return '—';
  }
}

export function statusFor(u = {}) {
  const h = String(u.health_status || '').toUpperCase();
  if (h === 'DOWN') return { cls: 'error', label: 'Недоступен' };
  if (h === 'DEGRADED') return { cls: 'warn', label: 'Деградация' };
  if (u.active) return { cls: 'good', label: 'Активен' };
  return { cls: 'neutral', label: 'Доступен' };
}

export function errorCount(u = {}) {
  return Number(u.servfail || 0) + Number(u.refused || 0) + Number(u.other_errors || 0);
}

export function quality(u = {}, windowKey = 'stats_5m') {
  const win = u?.[windowKey];
  if (win && (Number(win.requests || 0) || Number(win.responses || 0) || Number(win.timeouts || 0))) {
    return Number(win.quality_pct ?? 100);
  }
  const responses = Number(u.responses || 0);
  if (!responses) return 100;
  return clamp(100 - (errorCount(u) / responses) * 100, 0, 100);
}

export function qualityClass(win = {}) {
  const status = String(win?.quality_status || 'NO_DATA').toUpperCase();
  return status === 'BAD' ? 'bad' : status === 'WARN' ? 'warn' : status === 'GOOD' ? 'good' : '';
}

export function latencyClass(n) {
  n = Number(n || 0);
  return n >= 1000 ? 'bad' : n >= 300 ? 'warn' : 'good';
}

export function profileOrder(a, b) {
  if (a === 'System') return 1;
  if (b === 'System') return -1;
  const na = Number((String(a).match(/\d+/) || [999])[0]);
  const nb = Number((String(b).match(/\d+/) || [999])[0]);
  return na - nb;
}

export function groupBy(arr = [], key) {
  return arr.reduce((map, item) => {
    (map[item[key]] ??= []).push(item);
    return map;
  }, {});
}

export function total(arr = [], key) {
  return arr.reduce((sum, item) => sum + Number(item[key] || 0), 0);
}

export function localWebURL(port) {
  if (!port || typeof location === 'undefined') return '';
  const host = location.hostname.includes(':') ? `[${location.hostname}]` : location.hostname;
  return `${location.protocol === 'https:' ? 'https:' : 'http:'}//${host}:${port}`;
}

export function stateInfo(item = {}) {
  switch (item.state) {
    case 'installed_external':
      return item.service_running
        ? { label: 'ACTIVE', cls: 'good', detail: 'Установлен · служба работает' }
        : { label: 'INSTALLED', cls: 'warn', detail: 'Установлен · служба не обнаружена' };
    case 'installed':
      if (item.managed) {
        return item.service_running
          ? { label: 'ACTIVE', cls: 'good', detail: 'Модуль DNS Monitor установлен и работает' }
          : { label: 'INSTALLED', cls: 'warn', detail: 'Модуль DNS Monitor установлен, helper не обнаружен' };
      }
      return { label: 'BUILT-IN', cls: 'good', detail: 'Встроено в DNS Monitor' };
    case 'planned': return { label: 'PLANNED', cls: 'info', detail: 'Запланированный модуль' };
    case 'incompatible': return { label: 'INCOMPATIBLE', cls: 'error', detail: 'Требования не выполнены' };
    case 'broken': return { label: 'BROKEN', cls: 'error', detail: 'Установка обнаружена, состояние некорректно' };
    default: return { label: 'AVAILABLE', cls: 'neutral', detail: 'Доступно для установки' };
  }
}
