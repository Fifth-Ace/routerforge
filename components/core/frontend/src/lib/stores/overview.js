import { writable } from 'svelte/store';
import {
  getPlatform, getModule, getAdminSummary, getAdminCPU, getAdminStorage, getAdminThermal,
  getPlainDNS, getAppActions
} from '$lib/api.js';
import { startSerialPolling } from '$lib/polling.js';

export const overview = writable({
  platform: null, summary: null, cpu: null, memory: null, thermal: null, storage: null,
  plainDns: null, networkRoutes: null, appActions: null, cpuSustainedHigh: null
});

let stopPolling = null;
let users = 0;
let lastCatalog = null;
let cpuHighSince = 0;
let refreshGeneration = 0;

export async function refreshOverview(nextCatalog = lastCatalog) {
  lastCatalog = nextCatalog || lastCatalog;
  const generation = ++refreshGeneration;
  const modules = lastCatalog?.modules || [];
  const installed = (id) => modules.some((item) => item.id === id && item.installed);
  const safe = async (fn) => { try { return await fn(); } catch { return null; } };
  const result = {
    platform:null, summary:null, cpu:null, memory:null, thermal:null, storage:null,
    plainDns:null, networkRoutes:null, appActions:null, cpuSustainedHigh:null
  };

  const platformPromise = safe(() => getPlatform());

  let systemPromise = Promise.resolve([null, null, null]);
  if (installed('system')) {
    systemPromise = Promise.all([
      safe(() => getModule('system','summary')),
      safe(() => getModule('system','cpu')),
      safe(() => getModule('system','memory'))
    ]);
  } else if (installed('admin')) {
    systemPromise = Promise.all([
      safe(() => getAdminSummary()),
      safe(() => getAdminCPU()),
      Promise.resolve(null)
    ]);
  }

  const thermalPromise = installed('thermal')
    ? safe(() => getModule('thermal','sensors'))
    : installed('admin')
      ? safe(() => getAdminThermal())
      : Promise.resolve(null);

  const storagePromise = installed('storage')
    ? safe(() => getModule('storage','storage'))
    : installed('admin')
      ? safe(() => getAdminStorage())
      : Promise.resolve(null);

  const plainDnsPromise = installed('dns')
    ? safe(() => getPlainDNS(500))
    : Promise.resolve(null);

  const networkPromise = installed('network')
    ? safe(() => getModule('network','routes'))
    : Promise.resolve(null);

  const actionsPromise = safe(() => getAppActions());

  const [platform, systemRows, thermal, storage] = await Promise.all([
    platformPromise,
    systemPromise,
    thermalPromise,
    storagePromise
  ]);

  result.platform = platform;
  result.summary = systemRows[0];
  result.cpu = systemRows[1];
  result.memory = installed('system') ? systemRows[2] : result.summary?.memory || null;
  result.thermal = thermal;
  result.storage = storage;
  result.cpuSustainedHigh = updateSustainedCPU(result.cpu);

  if (generation === refreshGeneration) {
    overview.set({ ...result });
  }

  [result.plainDns, result.networkRoutes, result.appActions] = await Promise.all([
    plainDnsPromise,
    networkPromise,
    actionsPromise
  ]);

  if (generation !== refreshGeneration) return result;

  overview.set(result);
  return result;
}

export function startOverviewPolling(getCatalog, intervalMs = 10000) {
  users += 1;

  const tick = () => refreshOverview(typeof getCatalog === 'function' ? getCatalog() : getCatalog);
  if (!stopPolling) {
    stopPolling = startSerialPolling(tick, intervalMs);
  }

  let stopped = false;
  return () => {
    if (stopped) return;
    stopped = true;
    users = Math.max(0, users - 1);

    if (!users && stopPolling) {
      stopPolling();
      stopPolling = null;
      cpuHighSince = 0;
    }
  };
}

export function averageCPU(cpu) {
  const rows = cpu?.cpus || [];
  if (!rows.length) return 0;
  return rows.reduce((sum, row) => sum + Number(row.usage_pct || 0), 0) / rows.length;
}

function updateSustainedCPU(cpu) {
  const rows = cpu?.cpus || [];
  if (!rows.length) {
    cpuHighSince = 0;
    return { active:false, usage_pct:0, since:'' };
  }

  const usage = averageCPU(cpu);
  const now = Date.now();
  if (usage < 90) {
    cpuHighSince = 0;
    return { active:false, usage_pct:usage, since:'' };
  }

  if (!cpuHighSince) cpuHighSince = now;
  return {
    active: now - cpuHighSince >= 30000,
    usage_pct: usage,
    since: new Date(cpuHighSince).toISOString()
  };
}

export function cpuTemperature(thermal) {
  const rows = thermal?.sensors || thermal?.thermal?.sensors || [];
  const candidates = rows.filter((sensor) => ['soc','cpu'].includes(String(sensor.role || '').toLowerCase()));
  const source = candidates.length ? candidates : rows;
  if (!source.length) return 0;
  return Math.max(...source.map((sensor) => Number(sensor.temp_c || 0)).filter(Number.isFinite));
}
