import assert from 'node:assert/strict';
import test from 'node:test';

import { catalogWebProbeMetadataEligible } from '../src/lib/catalog-web-security.js';

test('allows probe-required Web UI for an already installed unverified package', () => {
  const item = {
    installed: true,
    package_installed: true,
    trust: { status: 'unverified' },
    web: { mode: 'probe-required', embed: true, port: 2222 }
  };

  assert.equal(catalogWebProbeMetadataEligible(item, false), true);
});

test('does not trust an unverified manifest merely because it claims Web metadata', () => {
  const item = {
    installed: false,
    package_installed: false,
    trust: { status: 'unverified' },
    web: { mode: 'probe-required', embed: true, port: 2222 }
  };

  assert.equal(catalogWebProbeMetadataEligible(item, false), false);
});

test('unverified package cannot bypass probing with embedded-supported metadata', () => {
  const item = {
    installed: true,
    package_installed: true,
    trust: { status: 'unverified' },
    web: { mode: 'embedded-supported', embed: true, port: 2222 }
  };

  assert.equal(catalogWebProbeMetadataEligible(item, false), false);
});

test('keeps verified and runtime-local Web surfaces eligible', () => {
  assert.equal(
    catalogWebProbeMetadataEligible({
      trust: { status: 'verified' },
      web: { mode: 'embedded-supported', embed: true, port: 90 }
    }, false),
    true
  );

  assert.equal(
    catalogWebProbeMetadataEligible({
      trust: { status: 'runtime-local' },
      web: { mode: 'probe-required', embed: true, port: 8080 }
    }, true),
    true
  );
});
