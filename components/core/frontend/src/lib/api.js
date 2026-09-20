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

    const text = await response.text();
    if (!text.trim()) {
      const error = new Error(`${path} returned an empty response`);
      error.code = 'EMPTY_JSON_RESPONSE';
      error.status = response.status;
      throw error;
    }

    try {
      return JSON.parse(text);
    } catch (cause) {
      const error = new Error(`${path} returned invalid JSON`);
      error.code = 'INVALID_JSON_RESPONSE';
      error.status = response.status;
      error.cause = cause;
      throw error;
    }
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
export const getPlatformAlerts = () => request('/api/platform/alerts');
export const getPlatformEvents = ({
  limit = 100,
  severity = '',
  component = '',
  type = '',
  object_id = '',
  transaction_id = '',
  recovery = ''
} = {}) => {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  if (severity) params.set('severity', severity);
  if (component) params.set('component', component);
  if (type) params.set('type', type);
  if (object_id) params.set('object_id', object_id);
  if (transaction_id) params.set('transaction_id', transaction_id);
  if (recovery !== '' && recovery !== null && recovery !== undefined) params.set('recovery', String(recovery));
  return request(`/api/platform/events?${params.toString()}`);
};
export const getSystem = () => request('/api/system');
export const getCatalog = () => request('/api/catalog');
export const refreshCatalogRemote = (fresh = false) =>
  postJSON(`/api/catalog/refresh${fresh ? '?fresh=1' : ''}`, {});
export const setCatalogChannel = (channel) => postJSON('/api/catalog/channel', { channel });
export const probeCatalogWeb = (id) => postJSON('/api/catalog/web-probe', { id });
export const getEntwarePackages = ({ query = '', state = '', offset = 0, limit = 100 } = {}) =>
  request(`/api/apps/entware?query=${encodeURIComponent(query)}&state=${encodeURIComponent(state)}&offset=${encodeURIComponent(offset)}&limit=${encodeURIComponent(limit)}`);
export const refreshEntwarePackages = () =>
  postJSON('/api/apps/entware/refresh', {}, PACKAGE_WRITE_TIMEOUT_MS);
export const getAppSources = () => request('/api/apps/sources');
export const previewAppSource = (body) => postJSON('/api/apps/sources/preview', body);
export const addAppSource = (body) => postJSON('/api/apps/sources', body);
export const refreshAppSource = (id) => postJSON(`/api/apps/sources/${encodeURIComponent(id)}/refresh`, {});
export const setAppSourceEnabled = (id, enabled) =>
  postJSON(`/api/apps/sources/${encodeURIComponent(id)}/toggle`, { enabled });
export const setAppSourceReleaseChannel = (id, channel) =>
  postJSON(`/api/apps/sources/${encodeURIComponent(id)}/channel`, { channel });
export const removeAppSource = (id) => deleteRequest(`/api/apps/sources/${encodeURIComponent(id)}`);
export const setAppSourceSecurity = (body) => postJSON('/api/apps/sources/security', body);
export const getAppLegal = (locale = 'ru') => request(`/api/apps/legal?locale=${encodeURIComponent(locale)}`);
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
export const getAdminFiles = (path, includeTreeMetadata = false) =>
  request(`/api/modules/admin/files/list?path=${encodeURIComponent(path)}${includeTreeMetadata ? '&tree_meta=1' : ''}`);
export const getAdminFileVolumes = () =>
  request('/api/modules/admin/files/volumes');
export const getAdminFileHash = (path, algorithm = 'sha256') =>
  request(`/api/modules/admin/files/hash?path=${encodeURIComponent(path)}&algorithm=${encodeURIComponent(algorithm)}`);
export const readAdminFile = (path) =>
  request(`/api/modules/admin/files/read?path=${encodeURIComponent(path)}`);
export const adminFileDownloadURL = (path) =>
  `/api/modules/admin/files/download?path=${encodeURIComponent(path)}`;
export const adminFileMkdir = (body) =>
  postJSON('/api/modules/admin/files/mkdir', body);
export const adminFileWrite = (body) =>
  postJSON('/api/modules/admin/files/write', body);
export const adminFileMove = (body) =>
  postJSON('/api/modules/admin/files/move', body);
export const adminFileCopy = (body) =>
  postJSON('/api/modules/admin/files/copy', body);
export const adminFileDelete = (body) =>
  postJSON('/api/modules/admin/files/delete', body);
export const adminFileChmod = (body) =>
  postJSON('/api/modules/admin/files/chmod', body);
export const adminProcessSignal = (pid, signal) =>
  postJSON(`/api/modules/admin/processes/${encodeURIComponent(pid)}/signal`, {
    signal,
    confirm_pid: Number(pid)
  });
export const adminServiceAction = (id, action) =>
  postJSON(`/api/modules/admin/services/${encodeURIComponent(id)}/action`, {
    action,
    confirm_id: id
  });
export const adminTerminalRun = (command, cwd = '/opt') =>
  postJSON('/api/modules/admin/terminal/run', {
    command,
    cwd,
    confirm: 'RUN'
  });
export const adminTerminalCreate = (cwd = '/opt', cols = 80, rows = 24, clientID = '') =>
  postJSON('/api/modules/admin/terminal/session', {
    cwd,
    cols,
    rows,
    client_id: clientID,
    confirm: 'CONNECT'
  });
export const adminTerminalOutput = (sessionID, cursor = 0) =>
  request(`/api/modules/admin/terminal/session/${encodeURIComponent(sessionID)}/output?cursor=${encodeURIComponent(cursor)}`);
export const adminTerminalInput = (sessionID, data) =>
  postJSON(`/api/modules/admin/terminal/session/${encodeURIComponent(sessionID)}/input`, { data });
export const adminTerminalResize = (sessionID, cols, rows) =>
  postJSON(`/api/modules/admin/terminal/session/${encodeURIComponent(sessionID)}/resize`, { cols, rows });
export const adminTerminalClose = (sessionID) =>
  postJSON(`/api/modules/admin/terminal/session/${encodeURIComponent(sessionID)}/close`, {});
export const getAdminMaintenanceLogs = () =>
  request('/api/modules/admin/maintenance/logs');
export const getAdminMaintenanceTasks = () =>
  request('/api/modules/admin/maintenance/tasks');
export const getAdminNFQWSJobs = () =>
  request('/api/modules/admin/maintenance/nfqws-jobs');
export const configureAdminNFQWSJob = (job) =>
  postJSON('/api/modules/admin/maintenance/nfqws-jobs/configure', {
    ...job,
    confirm_id: job.id
  });
export const runAdminNFQWSJob = (id) =>
  postJSON('/api/modules/admin/maintenance/nfqws-jobs/run', {
    id,
    confirm_id: id
  });
export const adminMaintenanceBackup = () =>
  postJSON('/api/modules/admin/maintenance/backup', { confirm: 'BACKUP' });
export const getAdminIntegrations = () =>
  request('/api/modules/admin/integrations');
export const getNFQWS2Manager = () =>
  request('/api/modules/admin/integrations/nfqws2');
export const nfqws2ManagerAction = (action) =>
  postJSON('/api/modules/admin/integrations/nfqws2/action', { action, confirm: 'NFQWS2' });
export const saveNFQWS2Config = (content) =>
  postJSON('/api/modules/admin/integrations/nfqws2/config', { content, confirm: 'NFQWS2_CONFIG' });
export const getAdminMaintenanceBackups = () =>
  request('/api/modules/admin/maintenance/backups');
export const adminMaintenanceRestore = (path) =>
  postJSON('/api/modules/admin/maintenance/restore', {
    path,
    confirm_path: path,
    confirm: 'RESTORE'
  });
export const getAdminWatchdogs = () =>
  request('/api/modules/admin/maintenance/watchdogs');
export const configureAdminWatchdog = (id, enabled) =>
  postJSON('/api/modules/admin/maintenance/watchdogs/configure', {
    id,
    enabled,
    confirm_id: id
  });
export const getAdminSnapshots = () =>
  request('/api/modules/admin/maintenance/snapshots');
export const createAdminSnapshot = () =>
  postJSON('/api/modules/admin/maintenance/snapshot', { confirm: 'SNAPSHOT' });
export const deleteAdminSnapshot = (path) =>
  postJSON('/api/modules/admin/maintenance/snapshot/delete', {
    path,
    confirm_path: path,
    confirm: 'DELETE'
  });
export const getAdminSupportStatus = () =>
  request('/api/modules/admin/maintenance/support/status');
export const getAdminSupportBundles = () =>
  request('/api/modules/admin/maintenance/support-bundles');
export const createAdminSupportBundle = () =>
  postJSON('/api/modules/admin/maintenance/support-bundle', { confirm: 'SUPPORT_BUNDLE' });
export const adminSupportBundleDownloadURL = (path) =>
  `/api/modules/admin/maintenance/support-bundle/download?path=${encodeURIComponent(path)}`;
