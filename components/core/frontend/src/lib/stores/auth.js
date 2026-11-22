import { writable, get } from 'svelte/store';
import { withRequestTimeout } from '$lib/http.js';

const AUTH_TIMEOUT_MS = 12000;

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
// A status GET started before logout/login must never overwrite the resulting
// authenticated state after the mutation has completed.
let authReadRevision = 0;
const invalidateAuthReads = () => { authReadRevision += 1; };

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

async function authRequest(path, options) {
  return withRequestTimeout(path, AUTH_TIMEOUT_MS, async (signal) => {
    const response = await fetch(path, { ...options, signal });
    return await parseResponse(response);
  });
}

export function markUnauthorized() {
  invalidateAuthReads();
  authState.update((current) => current.required
    ? { ...current, ready: true, authenticated: false, user: '' }
    : current);
}

export async function refreshAuth() {
  const revision = ++authReadRevision;
  try {
    const data = await authRequest('/api/auth/status', {
      cache: 'no-store',
      headers: { Accept: 'application/json' }
    });
    if (revision === authReadRevision) authState.set({ ...initial, ...data, ready: true, error: '' });
    return data;
  } catch (error) {
    if (revision !== authReadRevision) return null;
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
  invalidateAuthReads();
  await authRequest('/api/auth/login', {
    method: 'POST',
    cache: 'no-store',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  });
  return refreshAuth();
}

export async function logoutAuth() {
  invalidateAuthReads();
  try {
    await authRequest('/api/auth/logout', {
      method: 'POST',
      cache: 'no-store',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: '{}'
    });
  } finally {
    invalidateAuthReads();
    authState.update((current) => ({
      ...current,
      ready: true,
      authenticated: !current.required,
      user: ''
    }));
  }
}

export async function setAuthRequired(required, username = '', password = '') {
  invalidateAuthReads();
  const data = await authRequest('/api/auth/config', {
    method: 'POST',
    cache: 'no-store',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ required: Boolean(required), username, password })
  });
  invalidateAuthReads();
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
