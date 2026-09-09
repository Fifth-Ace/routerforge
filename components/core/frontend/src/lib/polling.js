export function startSerialPolling(task, intervalMs = 15000, options = {}) {
  if (typeof task !== 'function') {
    throw new TypeError('polling task must be a function');
  }

  const parsedInterval = Number(intervalMs);
  const interval = Number.isFinite(parsedInterval) && parsedInterval >= 0 ? parsedInterval : 0;
  const immediate = options.immediate !== false;
  const setTimer = options.setTimer || setTimeout;
  const clearTimer = options.clearTimer || clearTimeout;

  let stopped = false;
  let running = false;
  let timer = null;

  const schedule = () => {
    if (stopped || timer !== null) return;
    timer = setTimer(() => {
      timer = null;
      void run();
    }, interval);
  };

  const run = async () => {
    if (stopped || running) return;

    running = true;
    try {
      await task();
    } catch {
      // Polling tasks own their UI/backend error state. Keep the loop alive.
    } finally {
      running = false;
      if (!stopped) schedule();
    }
  };

  if (immediate) {
    void run();
  } else {
    schedule();
  }

  return () => {
    if (stopped) return;
    stopped = true;

    if (timer !== null) {
      clearTimer(timer);
      timer = null;
    }
  };
}
