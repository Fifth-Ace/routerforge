// Per-view GET lifecycle: own the entire response-body read, not merely headers.
// POST/DELETE are deliberately not aborted: a protected transaction may have
// committed on the server even when its response has not arrived.
export function createPortAccessReadSession() {
  let closed = false;
  const controllers = new Set();
  async function consume(url, options = {}, read) {
    const method = String(options.method || 'GET').toUpperCase();
    if (method !== 'GET') return read(await fetch(url, options));
    if (closed) throw new Error('PORT_ACCESS_READ_SESSION_CLOSED');
    const controller = new AbortController();
    controllers.add(controller);
    try {
      const response = await fetch(url, {...options, signal: controller.signal});
      return await read(response);
    } finally {
      controllers.delete(controller);
    }
  }
  return {
    consume,
    close() {
      closed = true;
      for (const controller of controllers) controller.abort();
      controllers.clear();
    }
  };
}
