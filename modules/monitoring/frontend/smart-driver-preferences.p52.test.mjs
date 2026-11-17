import { test } from 'node:test';
import { strict as assert } from 'node:assert';
import { setSmartDriverPreference, smartDriverDiskKey } from './smart-driver-preferences.js';

test('P52 distinguishes unavailable disk identity from browser storage refusal', () => {
  const disk = { device: '/dev/sdb', size_bytes: 4096, model: 'Disk' };
  const absent = setSmartDriverPreference(null, {}, null, 'sat');
  assert.equal(absent.reason, 'identity_unavailable');
  const unavailable = setSmartDriverPreference(null, {}, disk, 'sat');
  assert.equal(unavailable.reason, 'storage_unavailable');
  assert.equal(unavailable.preferences[smartDriverDiskKey(disk)], 'sat');
  const saved = setSmartDriverPreference({ setItem() {} }, {}, disk, 'sat');
  assert.equal(saved.saved, true);
});
