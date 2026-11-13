// A Svelte UI owner controls the lifetime of its read-only requests.
// Sessions are keyed by screen/resource; a newer read cancels its predecessor.
// Never use this scope for POST/PUT/PATCH/DELETE transactions: they may have
// committed server-side even when the UI has unmounted.
export function createReadSessions() {
  const active = new Map();
  let disposed = false;

  function cancel(key) {
    const previous = active.get(key);
    if (!previous) return;
    active.delete(key);
    previous.abort();
  }

  function begin(key) {
    cancel(key);
    const controller = new AbortController();
    if (disposed) controller.abort();
    else active.set(key, controller);
    return {
      signal: controller.signal,
      isCurrent: () => !disposed && active.get(key) === controller && !controller.signal.aborted,
      finish: () => {
        if (active.get(key) === controller) active.delete(key);
      }
    };
  }

  function cancelAll() {
    for (const key of [...active.keys()]) cancel(key);
  }

  function dispose() {
    disposed = true;
    cancelAll();
  }

  return { begin, cancel, cancelAll, dispose };
}
