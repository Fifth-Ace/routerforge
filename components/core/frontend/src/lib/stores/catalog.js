import { writable } from 'svelte/store';
import { getCatalog, refreshCatalogRemote } from '$lib/api.js';
import { startSerialPolling } from '$lib/polling.js';

export const catalog = writable({
  modules: [],
  integrations: [],
  read_only: true,
  install_test_mode: false,
  phase: 'loading'
});

export const catalogOnline = writable(false);
export const catalogReady = writable(false);

let stopPolling = null;

export async function refreshCatalog() {
  try {
    const data = await getCatalog();
    catalog.set(data || {
      modules: [],
      integrations: [],
      read_only: true,
      install_test_mode: false
    });
    catalogOnline.set(true);
    catalogReady.set(true);
    return data;
  } catch {
    catalogOnline.set(false);
    catalogReady.set(true);
    return null;
  }
}

export async function forceRefreshCatalog(fresh = false) {
  try {
    const result = await refreshCatalogRemote(fresh);
    const data = result?.catalog || null;
    if (data) {
      catalog.set(data);
      catalogOnline.set(true);
      catalogReady.set(true);
      return result;
    }
    const fallback = await refreshCatalog();
    return { ok: Boolean(fallback), catalog: fallback };
  } catch (error) {
    catalogOnline.set(false);
    catalogReady.set(true);
    throw error;
  }
}

export function startCatalogPolling(intervalMs = 15000) {
  if (stopPolling) return stopPolling;

  const stop = startSerialPolling(refreshCatalog, intervalMs);
  const wrappedStop = () => {
    stop();
    if (stopPolling === wrappedStop) {
      stopPolling = null;
    }
  };

  stopPolling = wrappedStop;
  return wrappedStop;
}
