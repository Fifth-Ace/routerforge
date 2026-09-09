export async function withRequestTimeout(label, timeoutMs, operation) {
  if (typeof operation !== 'function') {
    throw new TypeError('request operation must be a function');
  }

  const parsedTimeout = Number(timeoutMs);
  if (!Number.isFinite(parsedTimeout) || parsedTimeout <= 0) {
    throw new TypeError('request timeout must be a positive finite number');
  }

  const controller = new AbortController();
  let timedOut = false;

  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, parsedTimeout);

  try {
    return await operation(controller.signal);
  } catch (error) {
    if (timedOut) {
      const timeoutError = new Error(`${label} timed out after ${parsedTimeout}ms`);
      timeoutError.name = 'TimeoutError';
      throw timeoutError;
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
}
