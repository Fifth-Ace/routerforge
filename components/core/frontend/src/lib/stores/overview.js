import { writable } from 'svelte/store';
import {
  getPlatform, getModule, getAdminSummary, getAdminCPU, getAdminStorage, getAdminThermal, getPlainDNS
} from '$lib/api.js';

export const overview = writable({
  platform: null, summary: null, cpu: null, memory: null, thermal: null, storage: null, plainDns: null
});

let timer = null;
let users = 0;
let lastCatalog = null;

export async function refreshOverview(nextCatalog = lastCatalog) {
  lastCatalog = nextCatalog || lastCatalog;
  const modules = lastCatalog?.modules || [];
  const installed = (id) => modules.some((item) => item.id === id && item.installed);
  const safe = async (fn) => { try { return await fn(); } catch { return null; } };
  const result = { platform:null, summary:null, cpu:null, memory:null, thermal:null, storage:null, plainDns:null };

  result.platform = await safe(() => getPlatform());

  if (installed('system')) {
    [result.summary, result.cpu, result.memory] = await Promise.all([
      safe(() => getModule('system','summary')),
      safe(() => getModule('system','cpu')),
      safe(() => getModule('system','memory'))
    ]);
  } else if (installed('admin')) {
    [result.summary, result.cpu] = await Promise.all([
      safe(() => getAdminSummary()),
      safe(() => getAdminCPU())
    ]);
    result.memory = result.summary?.memory || null;
  }

  if (installed('thermal')) result.thermal = await safe(() => getModule('thermal','sensors'));
  else if (installed('admin')) result.thermal = await safe(() => getAdminThermal());

  if (installed('storage')) result.storage = await safe(() => getModule('storage','storage'));
  else if (installed('admin')) result.storage = await safe(() => getAdminStorage());

  if (installed('dns')) result.plainDns = await safe(() => getPlainDNS(100));

  overview.set(result);
  return result;
}

export function startOverviewPolling(getCatalog, intervalMs = 10000) {
  users += 1;
  const tick = () => refreshOverview(typeof getCatalog === 'function' ? getCatalog() : getCatalog);
  if (!timer) {
    tick();
    timer = setInterval(tick, intervalMs);
  }
  let stopped = false;
  return () => {
    if (stopped) return;
    stopped = true;
    users = Math.max(0, users - 1);
    if (!users && timer) {
      clearInterval(timer);
      timer = null;
    }
  };
}

export function averageCPU(cpu) {
  const rows = cpu?.cpus || [];
  if (!rows.length) return 0;
  return rows.reduce((sum, row) => sum + Number(row.usage_pct || 0), 0) / rows.length;
}

export function cpuTemperature(thermal) {
  const rows = thermal?.sensors || thermal?.thermal?.sensors || [];
  const candidates = rows.filter((sensor) => ['soc','cpu'].includes(String(sensor.role || '').toLowerCase()));
  const source = candidates.length ? candidates : rows;
  if (!source.length) return 0;
  return Math.max(...source.map((sensor) => Number(sensor.temp_c || 0)).filter(Number.isFinite));
}
