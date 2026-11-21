import assert from 'node:assert/strict';
import test from 'node:test';
import {
  routerForgeServerPreview,
  routerForgeServerPreviewMatches,
  routerForgeServerAnnouncementKey,
  shouldShowServerAnnouncement,
  dismissServerAnnouncement
} from '../src/lib/routerforge-server-preview.js';

test('RouterForge Server is a read-only, noninstallable preview of independent Linux CLI', () => {
  const item = routerForgeServerPreview;
  assert.equal(item.id, 'routerforge-server');
  assert.equal(item.repository, 'https://github.com/Fifth-Ace/routerforge-server');
  assert.equal(item.integrationStatus, 'in-development');
  for (const key of ['installSupported','pairingSupported','remoteAccessSupported','apiSupported']) {
    assert.equal(item[key], false);
  }
  assert.equal(routerForgeServerPreviewMatches('vps'), true);
  assert.equal(routerForgeServerPreviewMatches('сервер'), true);
  assert.equal(routerForgeServerPreviewMatches('something else'), false);
});

test('announcement appears until dismissed and does not reappear after reload', () => {
  const map = new Map();
  const storage = { getItem: (key) => map.get(key) ?? null, setItem: (key, value) => map.set(key, value) };
  assert.equal(shouldShowServerAnnouncement(storage), true);
  assert.equal(dismissServerAnnouncement(storage), true);
  assert.equal(map.get(routerForgeServerAnnouncementKey), 'dismissed');
  assert.equal(shouldShowServerAnnouncement(storage), false);
  assert.equal(shouldShowServerAnnouncement(null), true);
});

test('storage failure does not crash application', () => {
  const broken = { getItem: () => { throw new Error('storage unavailable'); }, setItem: () => { throw new Error('storage unavailable'); } };
  assert.throws(() => shouldShowServerAnnouncement(broken));
  assert.throws(() => dismissServerAnnouncement(broken));
});
