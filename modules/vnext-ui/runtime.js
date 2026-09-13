(function () {
  'use strict';

  function basePath() {
    var path = window.location.pathname;
    var marker = '/ui/';
    var pos = path.indexOf(marker);
    return pos >= 0 ? path.slice(0, pos) : path.replace(/\/ui$/, '');
  }

  function moduleId() {
    var match = window.location.pathname.match(/\/api\/modules\/([^/]+)\//);
    return match ? decodeURIComponent(match[1]) : '';
  }

  function esc(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function fmtBytes(value) {
    var n = Number(value || 0);
    if (!Number.isFinite(n) || n <= 0) return '0 B';
    var units = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
    return (n / Math.pow(1024, i)).toFixed(i ? 1 : 0) + ' ' + units[i];
  }

  async function jsonRequest(url, options) {
    var response = await fetch(url, Object.assign({
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { Accept: 'application/json' }
    }, options || {}));
    var text = await response.text();
    var data;
    try { data = text ? JSON.parse(text) : {}; }
    catch (_) { data = { error: text || 'invalid JSON response' }; }
    if (!response.ok) throw new Error(data.error || ('HTTP ' + response.status));
    return data;
  }

  async function get(path, params) {
    var url = new URL(basePath() + path, window.location.origin);
    Object.keys(params || {}).forEach(function (key) {
      var value = params[key];
      if (value !== '' && value != null) url.searchParams.set(key, value);
    });
    return jsonRequest(url.toString());
  }

  async function coreGet(path) {
    return jsonRequest(new URL(path, window.location.origin).toString());
  }

  async function corePost(path, body) {
    return jsonRequest(new URL(path, window.location.origin).toString(), {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {})
    });
  }

  function badge(text, tone) {
    return '<span class="rf-badge ' + esc(tone || '') + '">' + esc(text) + '</span>';
  }

  function metric(label, value, hint) {
    return '<div class="rf-metric"><span>' + esc(label) + '</span><strong>' + esc(value) + '</strong>' +
      (hint ? '<small>' + esc(hint) + '</small>' : '') + '</div>';
  }

  function error(target, err) {
    var node = typeof target === 'string' ? document.querySelector(target) : target;
    if (node) node.innerHTML = '<div class="rf-error">' + esc(err && err.message ? err.message : err) + '</div>';
    notifyHeight();
  }

  function setHealth(health) {
    var node = document.querySelector('[data-health]');
    if (!node) return;
    node.innerHTML = '<span class="rf-dot ' + (health && health.ok ? '' : 'bad') + '"></span>' +
      esc(health && health.ok ? ('online · ' + health.version) : 'offline');
  }

  function pretty(value) {
    return '<pre class="mono" style="white-space:pre-wrap;overflow-wrap:anywhere;margin:0">' +
      esc(JSON.stringify(value, null, 2)) + '</pre>';
  }

  function notifyHeight() {
    try {
      if (window.parent === window) return;
      var root = document.documentElement;
      var body = document.body;
      var height = Math.max(
        root ? root.scrollHeight : 0,
        body ? body.scrollHeight : 0,
        root ? root.offsetHeight : 0,
        body ? body.offsetHeight : 0
      ) + 8;
      window.parent.postMessage({
        type: 'routerforge-module-height',
        moduleId: moduleId(),
        height: height
      }, window.location.origin);
    } catch (_) {}
  }

  function observeHeight() {
    notifyHeight();
    window.addEventListener('load', notifyHeight);
    window.addEventListener('resize', notifyHeight);
    if (typeof ResizeObserver === 'function') {
      var ro = new ResizeObserver(notifyHeight);
      ro.observe(document.documentElement);
    } else if (typeof MutationObserver === 'function') {
      var mo = new MutationObserver(notifyHeight);
      mo.observe(document.documentElement, { subtree: true, childList: true, attributes: true });
    }
    window.setTimeout(notifyHeight, 100);
    window.setTimeout(notifyHeight, 500);
  }

  window.RFUI = {
    basePath: basePath,
    esc: esc,
    fmtBytes: fmtBytes,
    get: get,
    coreGet: coreGet,
    corePost: corePost,
    badge: badge,
    metric: metric,
    error: error,
    setHealth: setHealth,
    pretty: pretty,
    notifyHeight: notifyHeight
  };

  observeHeight();
}());
