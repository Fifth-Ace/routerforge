import test from 'node:test';
import assert from 'node:assert/strict';
import { startSerialPolling } from '../src/lib/polling.js';

const flush = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

test('serial polling never schedules the next tick before the current task finishes', async () => {
  let nextTimer = 1;
  const timers = new Map();
  const resolvers = [];
  let calls = 0;
  let active = 0;
  let maxActive = 0;

  const setTimer = (fn, delay) => {
    const id = nextTimer++;
    timers.set(id, { fn, delay });
    return id;
  };
  const clearTimer = (id) => timers.delete(id);

  const task = () => new Promise((resolve) => {
    calls += 1;
    active += 1;
    maxActive = Math.max(maxActive, active);
    resolvers.push(() => {
      active -= 1;
      resolve();
    });
  });

  const stop = startSerialPolling(task, 25, { setTimer, clearTimer });

  await flush();
  assert.equal(calls, 1);
  assert.equal(timers.size, 0);

  resolvers.shift()();
  await flush();

  assert.equal(timers.size, 1);
  const [timerID, scheduled] = timers.entries().next().value;
  assert.equal(scheduled.delay, 25);

  timers.delete(timerID);
  scheduled.fn();
  await flush();

  assert.equal(calls, 2);
  assert.equal(maxActive, 1);
  assert.equal(timers.size, 0);

  stop();
  resolvers.shift()();
  await flush();

  assert.equal(timers.size, 0);
  assert.equal(maxActive, 1);
});

test('serial polling cleanup cancels a pending timer and prevents future work', async () => {
  let nextTimer = 1;
  const timers = new Map();
  let calls = 0;

  const setTimer = (fn, delay) => {
    const id = nextTimer++;
    timers.set(id, { fn, delay });
    return id;
  };
  const clearTimer = (id) => timers.delete(id);

  const stop = startSerialPolling(async () => {
    calls += 1;
  }, 15, { setTimer, clearTimer });

  await flush();
  assert.equal(calls, 1);
  assert.equal(timers.size, 1);

  stop();
  assert.equal(timers.size, 0);
});

test('serial polling can schedule the first run after the interval', async () => {
  let nextTimer = 1;
  const timers = new Map();
  let calls = 0;

  const setTimer = (fn, delay) => {
    const id = nextTimer++;
    timers.set(id, { fn, delay });
    return id;
  };
  const clearTimer = (id) => timers.delete(id);

  const stop = startSerialPolling(async () => {
    calls += 1;
  }, 40, { immediate: false, setTimer, clearTimer });

  assert.equal(calls, 0);
  assert.equal(timers.size, 1);

  const [timerID, scheduled] = timers.entries().next().value;
  timers.delete(timerID);
  scheduled.fn();
  await flush();

  assert.equal(calls, 1);
  stop();
});
