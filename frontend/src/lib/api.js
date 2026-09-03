import { markUnauthorized } from '$lib/stores/auth.js';

async function readError(response, path) {
  const error = new Error(`${path} HTTP ${response.status}`);
  error.status = response.status;
  try { error.payload = await response.json(); } catch {}
  if (response.status === 401) markUnauthorized();
  return error;
}

async function request(path) {
  const response = await fetch(path, {
    cache: 'no-store',
    headers: { Accept: 'application/json' }
  });
  if (!response.ok) throw await readError(response, path);
  return response.json();
}

async function postJSON(path, body) {
  const response = await fetch(path, {
    method: 'POST',
    cache: 'no-store',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  });
  if (!response.ok) throw await readError(response, path);
  return response.json();
}

export const getSnapshot = () => request('/api/snapshot');
export const getSystem = () => request('/api/system');
export const getCatalog = () => request('/api/catalog');
export const installCatalogItem = (id) => postJSON('/api/catalog/install', { id });
export const catalogAction = (id, action, confirm = '') =>
  postJSON('/api/catalog/action', { id, action, confirm });
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

export const getModule = (moduleID, endpoint = 'health') =>
  request(`/api/modules/${encodeURIComponent(moduleID)}/${encodeURIComponent(endpoint)}`);
