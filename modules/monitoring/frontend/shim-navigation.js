export function replaceState(url, state = {}) {
  const target = String(url || '');
  try {
    if (window.parent && window.parent !== window) {
      window.parent.history.replaceState(state, '', target);
      return;
    }
  } catch (_) {}
  window.history.replaceState(state, '', target);
}