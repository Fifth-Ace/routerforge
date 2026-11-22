// Existing Antiscan HTTP contract, extracted without changing authorization or error handling.
export function api(path, {signal} = {}) {
  return fetch(`../${path}`, {
    signal,
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { Accept: 'application/json' }
  }).then(async (response) => {
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(payload.error || `${path}: HTTP ${response.status}`);
      error.payload = payload;
      throw error;
    }
    return payload;
  });
}

export function mutate(path, body) {
  return fetch(`../${path}`, {
    method: 'POST',
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  }).then(async (response) => {
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(payload.error || `${path}: HTTP ${response.status}`);
      error.payload = payload;
      throw error;
    }
    return payload;
  });
}


// Svelte root's read-only lifetime. Abort only GETs when a component is
// destroyed. Protected POST mutations are intentionally NOT canceled here:
// the server may already have committed a CAS/rollback operation.
export function createAntiscanReadSession() {
  const pending = new Set();
  const active = new Map();
  let disposed = false;
  function begin(key) {
    const previous = active.get(key);
    if (previous) previous.abort();
    const controller = new AbortController();
    if (disposed) controller.abort();
    else { active.set(key, controller); pending.add(controller); }
    return {
      api(path) { return api(path, {signal: controller.signal}); },
      isCurrent() { return !disposed && active.get(key) === controller && !controller.signal.aborted; },
      finish() {
        if (active.get(key) === controller) active.delete(key);
        pending.delete(controller);
      }
    };
  }
  return {
    begin,
    api(path) {
      if (disposed) return Promise.reject(new Error('ANTISCAN_UI_UNMOUNTED'));
      const controller = new AbortController();
      pending.add(controller);
      return api(path, {signal: controller.signal}).finally(() => pending.delete(controller));
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      for (const controller of pending) controller.abort();
      pending.clear();
      active.clear();
    }
  };
}
