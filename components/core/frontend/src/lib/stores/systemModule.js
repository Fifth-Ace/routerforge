import { writable } from 'svelte/store';
import { getModule } from '$lib/api.js';
import { startSerialPolling } from '$lib/polling.js';

export const systemModuleSummary = writable(null);
export const systemModuleOnline = writable(false);

let stopPolling = null;
let users = 0;

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
  users += 1;
  if (!stopPolling) {
    stopPolling = startSerialPolling(refreshSystemModule, intervalMs);
  }

  let stopped = false;
  return () => {
    if (stopped) return;
    stopped = true;
    users = Math.max(0, users - 1);

    if (users === 0 && stopPolling) {
      stopPolling();
      stopPolling = null;
      systemModuleSummary.set(null);
      systemModuleOnline.set(false);
    }
  };
}
