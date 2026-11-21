// P54 integration boundary: metadata only. Not an Entware/OPKG module.
// Never use this metadata to infer pairing, credentials, installation or API access.
export const routerForgeServerPreview = Object.freeze({
  id: 'routerforge-server',
  name: 'RouterForge Server',
  repository: 'https://github.com/Fifth-Ace/routerforge-server',
  independentVersion: '1.0.0',
  integrationStatus: 'in-development',
  installSupported: false,
  pairingSupported: false,
  remoteAccessSupported: false,
  apiSupported: false
});

export const routerForgeServerAnnouncementKey = 'routerforge:announcement:server-intro:v1';

export function routerForgeServerPreviewMatches(value = '') {
  const query = String(value || '').toLocaleLowerCase().trim();
  if (!query) return true;
  return 'routerforge server rf server rfs linux vps сервер роутфорж серверы мониторинг'.includes(query);
}

export function shouldShowServerAnnouncement(storage) {
  if (!storage || typeof storage.getItem !== 'function') return true;
  return storage.getItem(routerForgeServerAnnouncementKey) !== 'dismissed';
}

export function dismissServerAnnouncement(storage) {
  if (!storage || typeof storage.setItem !== 'function') return false;
  storage.setItem(routerForgeServerAnnouncementKey, 'dismissed');
  return true;
}
