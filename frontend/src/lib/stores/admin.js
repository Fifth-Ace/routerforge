import { writable } from 'svelte/store';
import { getAdminSummary } from '$lib/api.js';

export const adminSummary = writable(null);
export const adminOnline = writable(false);

let timer = null;
let users = 0;

export async function refreshAdminSummary() {
  try {
    const data = await getAdminSummary();
    adminSummary.set(data);
    adminOnline.set(true);
    return data;
  } catch {
    adminSummary.set(null);
    adminOnline.set(false);
    return null;
  }
}

export function startAdminPolling(intervalMs = 10000) {
  users += 1;
  if (!timer) {
    refreshAdminSummary();
    timer = setInterval(refreshAdminSummary, intervalMs);
  }
  let stopped = false;
  return () => {
    if (stopped) return;
    stopped = true;
    users = Math.max(0, users - 1);
    if (users === 0 && timer) {
      clearInterval(timer);
      timer = null;
      adminSummary.set(null);
      adminOnline.set(false);
    }
  };
}
