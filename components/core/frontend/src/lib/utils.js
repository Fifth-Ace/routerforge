import { t, localeOf } from '$lib/i18n/index.js';

export const clamp = (n, a, b) => Math.max(a, Math.min(b, Number(n || 0)));

function currentLocale(locale) {
  if (locale) return localeOf(locale);
  if (typeof document !== 'undefined') return localeOf(document.documentElement.lang);
  return 'ru';
}

export const fmtInt = (n, locale) => new Intl.NumberFormat(currentLocale(locale) === 'en' ? 'en-US' : 'ru-RU').format(Number(n || 0));
export const fmtPct = (n) => `${Number(n || 0).toFixed(Number(n || 0) >= 10 ? 1 : 2)}%`;
export const fmtMs = (n) => Number(n || 0) > 0 ? `${Math.round(Number(n))} ms` : '—';

export function fmtAgo(iso, locale) {
  if (!iso || String(iso).startsWith('0001-')) return '—';
  const lang = currentLocale(locale);
  const s = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (s < 5) return t(lang, 'common.justNow');
  if (s < 60) return t(lang, 'common.secondsAgo', { count: s });
  if (s < 3600) return t(lang, 'common.minutesAgo', { count: Math.floor(s / 60) });
  if (s < 86400) return t(lang, 'common.hoursAgo', { count: Math.floor(s / 3600) });
  return t(lang, 'common.daysAgo', { count: Math.floor(s / 86400) });
}

export function fmtDuration(sec, locale) {
  const lang = currentLocale(locale);
  sec = Math.max(0, Number(sec || 0));
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  return [
    d ? t(lang, 'common.durationDay', { count: d }) : null,
    h ? t(lang, 'common.durationHour', { count: h }) : null,
    m ? t(lang, 'common.durationMinute', { count: m }) : null
  ].filter(Boolean).join(' ') || t(lang, 'common.durationSecond', { count: Math.floor(sec) });
}

export function bytes(n) {
  n = Number(n || 0);
  if (n < 1024) return `${n} B`;
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)} MB`;
  return `${(n / 1073741824).toFixed(2)} GB`;
}

export function timeOnly(iso, locale) {
  try {
    return new Date(iso).toLocaleTimeString(currentLocale(locale) === 'en' ? 'en-GB' : 'ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return '—';
  }
}

export function statusFor(u = {}, locale) {
  const lang = currentLocale(locale);
  const h = String(u.health_status || '').toUpperCase();
  if (h === 'DOWN') return { cls: 'error', label: t(lang, 'dns.overview.unavailableStatus') };
  if (h === 'DEGRADED') return { cls: 'warn', label: t(lang, 'dns.overview.degradedStatus') };
  if (u.active) return { cls: 'good', label: t(lang, 'dns.overview.activeStatus') };
  return { cls: 'neutral', label: t(lang, 'common.available') };
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

export function stateInfo(item = {}, locale) {
  const lang = currentLocale(locale);
  switch (item.state) {
    case 'installed_external':
      return item.service_running
        ? { label: t(lang, 'marketplace.state.active'), cls: 'good', detail: t(lang, 'marketplace.state.externalRunning') }
        : { label: t(lang, 'marketplace.state.installed'), cls: 'warn', detail: t(lang, 'marketplace.state.externalStopped') };
    case 'installed':
      if (item.managed) {
        return item.service_running
          ? { label: t(lang, 'marketplace.state.active'), cls: 'good', detail: t(lang, 'marketplace.state.managedRunning') }
          : { label: t(lang, 'marketplace.state.installed'), cls: 'warn', detail: t(lang, 'marketplace.state.managedStopped') };
      }
      return { label: t(lang, 'marketplace.state.builtIn'), cls: 'good', detail: t(lang, 'marketplace.state.builtinDetail') };
    case 'planned': return { label: t(lang, 'marketplace.state.planned'), cls: 'info', detail: t(lang, 'marketplace.state.plannedDetail') };
    case 'incompatible': return { label: t(lang, 'marketplace.state.incompatible'), cls: 'error', detail: t(lang, 'marketplace.state.incompatibleDetail') };
    case 'broken': return { label: t(lang, 'marketplace.state.broken'), cls: 'error', detail: t(lang, 'marketplace.state.brokenDetail') };
    default: return { label: t(lang, 'marketplace.state.available'), cls: 'neutral', detail: t(lang, 'marketplace.state.availableDetail') };
  }
}
