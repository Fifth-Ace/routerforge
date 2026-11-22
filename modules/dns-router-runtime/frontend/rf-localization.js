// UI-only locale helper. Core supplies ?locale=ru/en on the module iframe.
// This does not affect API requests, configuration values, or service state.
export function rfLocale() {
  if (typeof window === 'undefined') return 'ru';
  try { return new URLSearchParams(window.location.search).get('locale') === 'en' ? 'en' : 'ru'; }
  catch { return 'ru'; }
}
export function rfUi(ru, en) { return rfLocale() === 'en' ? en : ru; }
