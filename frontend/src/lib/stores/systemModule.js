import { writable } from 'svelte/store';
import { getModule } from '$lib/api.js';

export const systemModuleSummary = writable(null);
export const systemModuleOnline = writable(false);

let timer = null;

export async function refreshSystemModule() {
  try {
    const data = await getModule('system', 'summary');
    systemModuleSummary.set(data);
    systemModuleOnline.set(true);
    return data;
  } catch {
    systemModuleSummary.set(null);
    systemModuleOnline.set(false);
    return null;
  }
}

export function startSystemModulePolling(intervalMs = 10000) {
  if (!timer) {
    refreshSystemModule();
    timer = setInterval(refreshSystemModule, intervalMs);
  }
  return () => {};
}
