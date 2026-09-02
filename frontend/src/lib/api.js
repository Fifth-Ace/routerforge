async function request(path) {
  const response = await fetch(path, {
    cache: 'no-store',
    headers: { Accept: 'application/json' }
  });
  if (!response.ok) {
    throw new Error(`${path} HTTP ${response.status}`);
  }
  return response.json();
}

export const getSnapshot = () => request('/api/snapshot');
export const getSystem = () => request('/api/system');
export const getCatalog = () => request('/api/catalog');
export const getClients = () => request('/api/clients');
export const getInterfaces = () => request('/api/interfaces');
export const getHistory = (minutes = 60) => request(`/api/history?minutes=${encodeURIComponent(minutes)}`);
export const getClient = (ip, limit = 500) =>
  request(`/api/client?ip=${encodeURIComponent(ip)}&limit=${encodeURIComponent(limit)}`);
