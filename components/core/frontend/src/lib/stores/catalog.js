import { writable } from 'svelte/store';
import { getCatalog, refreshCatalogRemote } from '$lib/api.js';

export const catalog = writable({
  modules: [],
  integrations: [],
  read_only: true,
  install_test_mode: false,
  phase: 'loading'
});

export const catalogOnline = writable(false);

let timer = null;

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
    return data;
  } catch {
    catalogOnline.set(false);
    return null;
  }
}

export async function forceRefreshCatalog() {
  try {
    const result = await refreshCatalogRemote();
    const data = result?.catalog || null;
    if (data) {
      catalog.set(data);
      catalogOnline.set(true);
      return result;
    }
    const fallback = await refreshCatalog();
    return { ok: Boolean(fallback), catalog: fallback };
  } catch (error) {
    catalogOnline.set(false);
    throw error;
  }
}
export function startCatalogPolling(intervalMs = 15000) {
  if (!timer) {
    refreshCatalog();
    timer = setInterval(refreshCatalog, intervalMs);
  }
  return () => {};
}
