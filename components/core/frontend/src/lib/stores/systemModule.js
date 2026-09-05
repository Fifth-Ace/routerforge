import { writable } from 'svelte/store';
import { getModule } from '$lib/api.js';

export const systemModuleSummary = writable(null);
export const systemModuleOnline = writable(false);

let timer = null;
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
  if (!timer) {
    refreshSystemModule();
    timer = setInterval(refreshSystemModule, intervalMs);
  }
  let stopped = false;
  return () => {
    if (stopped) return;
    stopped = true;
    users = Math.max(0, users - 1);
    if (users === 0 && timer) {
      clearInterval(timer);
      timer = null;
      systemModuleSummary.set(null);
      systemModuleOnline.set(false);
    }
  };
}
