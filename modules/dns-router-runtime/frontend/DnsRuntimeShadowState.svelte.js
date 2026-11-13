import {readDraft,feedback} from './DnsRuntimeStore.svelte.js';
/* Native Svelte 5 shadow-DNS action state. No automatic mutation. */
export const shadow = $state({
  busy: false,
  status: 'Загрузка состояния…',
  error: '',
  result: null,
  sequence: 0,
  destroyed: false,
  viewEpoch: 0
});

let activeRead = null;
export function beginShadowView() {
  const epoch = ++shadow.viewEpoch;
  shadow.destroyed = false;
  ++shadow.sequence;
  shadow.result = null;
  shadow.status = 'Загрузка состояния…';
  shadow.error = '';
  return () => {
    if (epoch !== shadow.viewEpoch) return;
    shadow.destroyed = true;
    ++shadow.sequence;
    if (activeRead) { activeRead.abort(); activeRead = null; }
  };
}

export function draftStatus() {
  const state = readDraft();
  if (!state || typeof state.revision !== 'string' || !state.revision) {
    throw Error('Конфигурация ещё не загружена');
  }
  if (state.dirty) {
    throw Error('Сначала сохрани настройки: есть несохранённые изменения');
  }
  return state;
}

async function jsonRequest(path, options = {}) {
  const response = await fetch('../' + path, {cache:'no-store', credentials:'same-origin', ...options});
  let payload;
  try { payload = await response.json(); }
  catch { throw Error('Некорректный JSON: ' + path); }
  if (!response.ok) throw Error('HTTP ' + response.status + ': ' + (payload?.error || 'unknown error'));
  return payload;
}

export async function refreshShadow() {
  if (shadow.destroyed) return;
  const epoch = shadow.viewEpoch;
  const requestId = ++shadow.sequence;
  if (activeRead) activeRead.abort();
  const controller = new AbortController();
  activeRead = controller;
  try {
    const result = await jsonRequest('shadow/status', {signal: controller.signal});
    if (shadow.destroyed || epoch !== shadow.viewEpoch || requestId !== shadow.sequence) return;
    shadow.result = result;
    shadow.status = JSON.stringify(result, null, 2);
    shadow.error = '';
  } catch (error) {
    if (shadow.destroyed || epoch !== shadow.viewEpoch || requestId !== shadow.sequence || controller.signal.aborted) return;
    shadow.error = String(error?.message || error);
    shadow.status = 'Ошибка состояния Shadow DNS: ' + shadow.error;
    feedback('Shadow: ' + shadow.error);
  } finally {
    if (activeRead === controller) activeRead = null;
  }
}

export async function controlShadow(operation) {
  if (shadow.busy || shadow.destroyed || !['start','apply','stop'].includes(operation)) return;
  const epoch = shadow.viewEpoch;
  let state;
  try { state = draftStatus(); }
  catch (error) { feedback(error.message); return; }
  if (operation === 'start' && !confirm('Запустить сохранённый shadow listener? Keenetic DNS не изменится.')) return;
  if (operation === 'apply' && !confirm('Применить сохранённую ревизию к shadow DNS?')) return;
  // Re-check after a synchronous confirmation callback before touching the backend.
  if (epoch !== shadow.viewEpoch || shadow.destroyed) return;
  shadow.busy = true;
  shadow.error = '';
  try {
    const result = await jsonRequest('shadow/control', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-RouterForge-Action': 'shadow-control',
        'If-Match': '"' + state.revision + '"'
      },
      body: JSON.stringify({operation})
    });
    if (epoch === shadow.viewEpoch && !shadow.destroyed)
      feedback('Shadow: ' + result.outcome + ' · DNS Keenetic не затронут');
  } catch (error) {
    if (epoch === shadow.viewEpoch && !shadow.destroyed) {
      shadow.error = String(error?.message || error);
      feedback('Shadow: ' + shadow.error);
    }
  } finally {
    // The server may have completed a POST after the UI was closed. Do not retry
    // or claim success; reconcile the current view with a fresh read-only GET.
    if (!shadow.destroyed) await refreshShadow();
    shadow.busy = false;
  }
}
