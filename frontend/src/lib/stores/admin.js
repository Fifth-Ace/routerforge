import { writable } from 'svelte/store';
import { getAdminSummary } from '$lib/api.js';

export const adminSummary = writable(null);
export const adminOnline = writable(false);

let timer = null;

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
  if (!timer) {
    refreshAdminSummary();
    timer = setInterval(refreshAdminSummary, intervalMs);
  }
  return () => {};
}
