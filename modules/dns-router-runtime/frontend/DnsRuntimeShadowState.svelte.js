import {readDraft,feedback} from './DnsRuntimeStore.svelte.js';
/* Native Svelte 5 shadow-DNS action state. No automatic mutation. */
export const shadow = $state({
  busy: false,
  status: 'Загрузка состояния…',
  error: '',
  result: null,
  sequence: 0,
  destroyed: false
});

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
  const requestId = ++shadow.sequence;
  try {
    const result = await jsonRequest('shadow/status');
    if (shadow.destroyed || requestId !== shadow.sequence) return;
    shadow.result = result;
    shadow.status = JSON.stringify(result, null, 2);
    shadow.error = '';
  } catch (error) {
    if (shadow.destroyed || requestId !== shadow.sequence) return;
    shadow.error = String(error?.message || error);
    shadow.status = 'Ошибка состояния Shadow DNS: ' + shadow.error;
    feedback('Shadow: ' + shadow.error);
  }
}

export async function controlShadow(operation) {
  if (shadow.busy || !['start','apply','stop'].includes(operation)) return;
  let state;
  try { state = draftStatus(); }
  catch (error) { feedback(error.message); return; }
  if (operation === 'start' && !confirm('Запустить сохранённый shadow listener? Keenetic DNS не изменится.')) return;
  if (operation === 'apply' && !confirm('Применить сохранённую ревизию к shadow DNS?')) return;
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
    feedback('Shadow: ' + result.outcome + ' · DNS Keenetic не затронут');
  } catch (error) {
    shadow.error = String(error?.message || error);
    feedback('Shadow: ' + shadow.error);
  } finally {
    await refreshShadow();
    shadow.busy = false;
  }
}
