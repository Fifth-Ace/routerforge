import { markUnauthorized } from '$lib/stores/auth.js';
import { withRequestTimeout } from '$lib/http.js';

const READ_TIMEOUT_MS = 12000;
const WRITE_TIMEOUT_MS = 30000;
const PACKAGE_WRITE_TIMEOUT_MS = 190000;

async function readError(response, path) {
  const error = new Error(`${path} HTTP ${response.status}`);
  error.status = response.status;
  try { error.payload = await response.json(); } catch {}
  if (response.status === 401) markUnauthorized();
  return error;
}

async function fetchJSON(path, options, timeoutMs) {
  return withRequestTimeout(path, timeoutMs, async (signal) => {
    const response = await fetch(path, { ...options, signal });
    if (!response.ok) throw await readError(response, path);
    return await response.json();
  });
}

async function request(path, timeoutMs = READ_TIMEOUT_MS) {
  return fetchJSON(path, {
    cache: 'no-store',
    headers: { Accept: 'application/json' }
  }, timeoutMs);
}

async function postJSON(path, body, timeoutMs = WRITE_TIMEOUT_MS) {
  return fetchJSON(path, {
    method: 'POST',
    cache: 'no-store',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  }, timeoutMs);
}

async function deleteRequest(path, timeoutMs = WRITE_TIMEOUT_MS) {
  return fetchJSON(path, {
    method: 'DELETE',
    cache: 'no-store',
    headers: { Accept: 'application/json' }
  }, timeoutMs);
}

export const getSnapshot = () => request('/api/snapshot');
export const getPlatform = () => request('/api/platform');
export const getSystem = () => request('/api/system');
export const getCatalog = () => request('/api/catalog');
export const refreshCatalogRemote = () => postJSON('/api/catalog/refresh', {});
export const setCatalogChannel = (channel) => postJSON('/api/catalog/channel', { channel });
export const probeCatalogWeb = (id) => postJSON('/api/catalog/web-probe', { id });
export const getEntwarePackages = ({ query = '', state = '', offset = 0, limit = 100 } = {}) =>
  request(`/api/apps/entware?query=${encodeURIComponent(query)}&state=${encodeURIComponent(state)}&offset=${encodeURIComponent(offset)}&limit=${encodeURIComponent(limit)}`);
export const refreshEntwarePackages = () =>
  postJSON('/api/apps/entware/refresh', {}, PACKAGE_WRITE_TIMEOUT_MS);
export const entwarePackageAction = (packageName, action, confirm = '') =>
  postJSON('/api/apps/entware/action', { package: packageName, action, confirm }, PACKAGE_WRITE_TIMEOUT_MS);
export const getEntwarePackageDetail = (packageName) =>
  request(`/api/apps/entware/detail?package=${encodeURIComponent(packageName)}`);
export const preflightAppAction = (body) => postJSON('/api/apps/preflight', body);
export const startAppAction = (body) => postJSON('/api/apps/actions', body);
export const getAppActions = () => request('/api/apps/actions');
export const getAppAction = (id) => request(`/api/apps/actions/${encodeURIComponent(id)}`);
export const cancelAppAction = (id) => deleteRequest(`/api/apps/actions/${encodeURIComponent(id)}`);
export const appActionEventsURL = (id) => `/api/apps/actions/${encodeURIComponent(id)}/events`;
export const installCatalogItem = (id) =>
  postJSON('/api/catalog/install', { id }, PACKAGE_WRITE_TIMEOUT_MS);
export const catalogAction = (id, action, confirm = '') =>
  postJSON('/api/catalog/action', { id, action, confirm }, PACKAGE_WRITE_TIMEOUT_MS);
export const getClients = () => request('/api/clients');
export const getInterfaces = () => request('/api/interfaces');
export const getHistory = (minutes = 60) => request(`/api/history?minutes=${encodeURIComponent(minutes)}`);
export const getClient = (ip, limit = 500) =>
  request(`/api/client?ip=${encodeURIComponent(ip)}&limit=${encodeURIComponent(limit)}`);

export const getAdminSummary = () => request('/api/admin/summary');
export const getAdminCPU = () => request('/api/admin/cpu');
export const getAdminProcesses = () => request('/api/admin/processes');
export const getAdminPorts = () => request('/api/admin/ports');
export const getAdminServices = () => request('/api/admin/services');
export const getAdminPackages = () => request('/api/admin/packages');
export const getAdminStorage = () => request('/api/admin/storage');
export const getAdminThermal = () => request('/api/admin/thermal');

export const getPlainDNS = (limit = 100) =>
  request(`/api/plain-dns?limit=${encodeURIComponent(limit)}`);
export const getDNSInfo = () => request('/api/dns/info');

export const getModule = (moduleID, endpoint = 'health') =>
  request(`/api/modules/${encodeURIComponent(moduleID)}/${encodeURIComponent(endpoint)}`);
