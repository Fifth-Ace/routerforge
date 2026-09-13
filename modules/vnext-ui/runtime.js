(function () {
  'use strict';

  function basePath() {
    var path = window.location.pathname;
    var marker = '/ui/';
    var pos = path.indexOf(marker);
    return pos >= 0 ? path.slice(0, pos) : path.replace(/\/ui$/, '');
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

  async function get(path, params) {
    var url = new URL(basePath() + path, window.location.origin);
    Object.keys(params || {}).forEach(function (key) {
      var value = params[key];
      if (value !== '' && value != null) url.searchParams.set(key, value);
    });
    var response = await fetch(url.toString(), {
      credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' }
    });
    var text = await response.text();
    var data;
    try { data = text ? JSON.parse(text) : {}; } catch (_) { data = { error: text || 'invalid JSON response' }; }
    if (!response.ok) throw new Error(data.error || ('HTTP ' + response.status));
    return data;
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
  }

  function setHealth(health) {
    var node = document.querySelector('[data-health]');
    if (!node) return;
    node.innerHTML = '<span class="rf-dot ' + (health && health.ok ? '' : 'bad') + '"></span>' +
      esc(health && health.ok ? ('online · ' + health.version) : 'offline');
  }

  window.RFUI = { basePath: basePath, esc: esc, fmtBytes: fmtBytes, get: get, badge: badge, metric: metric, error: error, setHealth: setHealth };
}());
