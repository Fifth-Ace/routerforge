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

export function catalogWebPort(item = {}) {
  const port = Number(item?.web?.port || item?.web_port || 0);
  return Number.isInteger(port) && port >= 1 && port <= 65535 ? port : 0;
}

export function catalogWebURL(item = {}, legacyScheme = 'current') {
  if (typeof location === 'undefined') return '';

  const web = item?.web || {};
  const port = catalogWebPort(item);
  if (!port) return '';

  const hostName = location.hostname.includes(':')
    ? `[${location.hostname}]`
    : location.hostname;

  const scheme = web.scheme === 'https'
    ? 'https:'
    : web.scheme === 'http'
      ? 'http:'
      : legacyScheme === 'http'
        ? 'http:'
        : (location.protocol === 'https:' ? 'https:' : 'http:');

  const path = typeof web.path === 'string'
    && web.path.startsWith('/')
    && !/[\r\n]/.test(web.path)
      ? web.path
      : '';

  return `${scheme}//${hostName}:${port}${path}`;
}

export function catalogWebSecurityDecision(item = {}, auth = {}, urlOverride = '') {
  const url = typeof urlOverride === 'string' && urlOverride
    ? urlOverride
    : catalogWebURL(item);
  const decision = {
    url,
    externalAllowed: Boolean(url),
    embedAllowed: false,
    credentialRisk: false,
    mixedContent: false,
    reason: ''
  };

  if (!url || typeof location === 'undefined') {
    decision.externalAllowed = false;
    decision.reason = 'no-url';
    return decision;
  }

  let target;
  try {
    target = new URL(url, location.href);
  } catch {
    decision.externalAllowed = false;
    decision.reason = 'invalid-url';
    return decision;
  }

  const normalizeHost = (value) => String(value || '')
    .replace(/^\[/, '')
    .replace(/\]$/, '')
    .toLowerCase();

  const currentHost = normalizeHost(location.hostname);
  const targetHost = normalizeHost(target.hostname);
  const currentPort = location.port || (location.protocol === 'https:' ? '443' : '80');
  const targetPort = target.port || (target.protocol === 'https:' ? '443' : '80');
  const sameHostDifferentPort = currentHost === targetHost && currentPort !== targetPort;

  decision.credentialRisk = Boolean(auth?.required && sameHostDifferentPort);
  if (decision.credentialRisk) {
    decision.externalAllowed = false;
    decision.reason = 'session-cookie-cross-port';
    return decision;
  }

  decision.mixedContent = location.protocol === 'https:' && target.protocol !== 'https:';

  const web = item?.web || {};
  const embedRequested = ['embedded-supported', 'probe-required'].includes(web.mode) && web.embed === true;
  const trustStatus = String(item?.trust?.status || '').toLowerCase();
  const probeEligible = trustStatus === 'official'
    || trustStatus === 'verified'
    || (item?.registry_source === 'legacy-fallback' && Boolean(item?.web_port_source));

  if (embedRequested && !probeEligible) {
    decision.reason = 'untrusted-web-metadata';
    return decision;
  }

  if (embedRequested && decision.mixedContent) {
    decision.reason = 'mixed-content';
    return decision;
  }

  decision.embedAllowed = decision.externalAllowed && embedRequested;
  return decision;
}

export function catalogWebResolvedURL(item = {}, probe = {}, auth = {}) {
  const candidates = Array.isArray(probe?.browser_urls)
    ? probe.browser_urls
    : [];

  for (const candidate of candidates) {
    const decision = catalogWebSecurityDecision(item, auth, candidate);
    if (decision.embedAllowed) return decision.url;
  }

  const fallback = catalogWebSecurityDecision(item, auth);
  return fallback.embedAllowed ? fallback.url : '';
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
