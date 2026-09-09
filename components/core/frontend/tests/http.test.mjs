import test from 'node:test';
import assert from 'node:assert/strict';
import { withRequestTimeout } from '../src/lib/http.js';

test('request timeout helper returns values completed before the deadline', async () => {
  const result = await withRequestTimeout('fast-request', 100, async (signal) => {
    assert.equal(signal.aborted, false);
    return 'ok';
  });

  assert.equal(result, 'ok');
});

test('request timeout helper aborts slow operations with a TimeoutError', async () => {
  await assert.rejects(
    withRequestTimeout('slow-request', 20, (signal) => new Promise((resolve, reject) => {
      signal.addEventListener('abort', () => reject(new Error('aborted')), { once: true });
    })),
    (error) => {
      assert.equal(error.name, 'TimeoutError');
      assert.match(error.message, /slow-request timed out after 20ms/);
      return true;
    }
  );
});
