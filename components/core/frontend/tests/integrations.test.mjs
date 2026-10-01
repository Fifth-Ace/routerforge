import assert from 'node:assert/strict';
import test from 'node:test';

import { catalogItemServiceNeedsAttention } from '../src/lib/integrations.js';

test('does not warn when only optional service definitions are declared', () => {
  const item = {
    installed: true,
    builtin: false,
    service: '',
    service_running: false,
    detection: {
      services: [
        '/opt/etc/init.d/S99geo-split',
        '/opt/etc/init.d/S80nginx-webui'
      ]
    }
  };

  assert.equal(catalogItemServiceNeedsAttention(item), false);
});

test('warns when an actually detected service has no running runtime signal', () => {
  const item = {
    installed: true,
    builtin: false,
    service: '/opt/etc/init.d/S99geo-split',
    service_running: false
  };

  assert.equal(catalogItemServiceNeedsAttention(item), true);
});

test('does not warn when the detected service runtime is healthy', () => {
  const item = {
    installed: true,
    builtin: false,
    service: '/opt/etc/init.d/S99geo-split',
    service_running: true
  };

  assert.equal(catalogItemServiceNeedsAttention(item), false);
});
