import assert from 'node:assert/strict';
import test from 'node:test';

import { catalogItemServiceNeedsAttention, railInstalledIntegrations } from '../src/lib/integrations.js';

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

test('rail presentation collapses the same upstream project into one installed integration', () => {
  const rows = railInstalledIntegrations([
    {
      id: 'nfqws-web',
      name: 'nfqws Web UI',
      project_url: 'https://github.com/nfqws/nfqws-keenetic-web',
      installed: true,
      update_available: false,
      service_running: true
    },
    {
      id: 'nfqws-keenetic-web',
      name: 'nfqws Web UI',
      project_url: 'https://github.com/nfqws/nfqws-keenetic-web/',
      installed: true,
      update_available: true,
      service_running: false
    }
  ]);

  assert.equal(rows.length, 1);
  assert.equal(rows[0].update_available, true);
  assert.equal(rows[0].service_running, true);
});

test('rail presentation does not collapse distinct projects that happen to share a name', () => {
  const rows = railInstalledIntegrations([
    {
      id: 'one',
      name: 'Same Name',
      project_url: 'https://github.com/example/one',
      installed: true
    },
    {
      id: 'two',
      name: 'Same Name',
      project_url: 'https://github.com/example/two',
      installed: true
    }
  ]);

  assert.equal(rows.length, 2);
});

test('rail presentation excludes non-installed integrations', () => {
  const rows = railInstalledIntegrations([
    { id: 'installed', name: 'Installed', installed: true },
    { id: 'preview', name: 'Preview', installed: false }
  ]);

  assert.deepEqual(rows.map((item) => item.id), ['installed']);
});
