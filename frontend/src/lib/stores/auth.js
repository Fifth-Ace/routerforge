import { writable, get } from 'svelte/store';

const initial = {
  ready: false,
  required: false,
  authenticated: false,
  user: '',
  backend: 'entware-root',
  session_hours: 12,
  error: ''
};

export const authState = writable({ ...initial });

async function parseResponse(response) {
  let payload = null;
  try { payload = await response.json(); } catch {}
  if (!response.ok) {
    const error = new Error(payload?.error || `HTTP ${response.status}`);
    error.status = response.status;
    error.payload = payload;
    throw error;
  }
  return payload || {};
}

export function markUnauthorized() {
  authState.update((current) => current.required
    ? { ...current, ready: true, authenticated: false, user: '' }
    : current);
}

export async function refreshAuth() {
  try {
    const response = await fetch('/api/auth/status', {
      cache: 'no-store',
      headers: { Accept: 'application/json' }
    });
    const data = await parseResponse(response);
    authState.set({ ...initial, ...data, ready: true, error: '' });
    return data;
  } catch (error) {
    authState.update((current) => ({
      ...current,
      ready: true,
      authenticated: false,
      error: error?.message || 'Не удалось получить состояние авторизации'
    }));
    throw error;
  }
}

export async function loginAuth(username, password) {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    cache: 'no-store',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  });
  await parseResponse(response);
  return refreshAuth();
}

export async function logoutAuth() {
  try {
    const response = await fetch('/api/auth/logout', {
      method: 'POST',
      cache: 'no-store',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: '{}'
    });
    await parseResponse(response);
  } finally {
    authState.update((current) => ({
      ...current,
      ready: true,
      authenticated: !current.required,
      user: ''
    }));
  }
}

export async function setAuthRequired(required, username = '', password = '') {
  const response = await fetch('/api/auth/config', {
    method: 'POST',
    cache: 'no-store',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ required: Boolean(required), username, password })
  });
  const data = await parseResponse(response);
  authState.set({ ...initial, ...data, ready: true, error: '' });
  if (required) {
    // The enabling response also creates the authenticated session cookie.
    return refreshAuth();
  }
  return data;
}

export function hasPanelAccess() {
  const current = get(authState);
  return current.ready && (!current.required || current.authenticated);
}
