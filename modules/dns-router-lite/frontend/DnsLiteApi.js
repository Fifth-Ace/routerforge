// DNS Lite's explicit, disposable HTTP client. Boot only calls GET endpoints;
// mutations require a user-initiated command with an action header.
export function createDnsLiteApi({fetchImpl, base = '../', timeoutMs = 15000} = {}) {
  if (typeof fetchImpl !== 'function') throw Error('DNS_LITE_FETCH_REQUIRED');
  let stopped = false;
  const controllers = new Set();
  function dispose() {
    stopped = true;
    for (const controller of controllers) controller.abort();
    controllers.clear();
  }
  async function request(path, options = {}) {
    if (stopped) throw Error('DNS_LITE_DISPOSED');
    if (typeof path !== 'string' || !/^[a-z0-9][a-z0-9/-]*$/i.test(path) || path.includes('//'))
      throw Error('DNS_LITE_INVALID_API_PATH');
    const method = String(options.method || 'GET').toUpperCase();
    if (!['GET','POST','PUT'].includes(method)) throw Error('DNS_LITE_METHOD_NOT_ALLOWED');
    const headers = {...(options.headers || {})};
    const action = headers['X-RouterForge-Action'];
    if (method !== 'GET' && (!action || typeof action !== 'string'))
      throw Error('DNS_LITE_MUTATION_ACTION_REQUIRED');
    if (method === 'PUT' && path === 'config' && !/^"[^"\r\n]+"$/.test(headers['If-Match'] || ''))
      throw Error('DNS_LITE_CAS_REVISION_REQUIRED');
    const controller = new AbortController();
    controllers.add(controller);
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      const response = await Promise.race([
        fetchImpl(base+path,{
          cache:'no-store', credentials:'same-origin', ...options,
          headers, signal:controller.signal
        }),
        new Promise((_,reject)=>controller.signal.addEventListener('abort',
          ()=>reject(Error(stopped?'DNS_LITE_DISPOSED':'DNS_LITE_API_TIMEOUT: '+path)),{once:true}))
      ]);
      if (stopped) throw Error('DNS_LITE_DISPOSED');
      const contentType = response.headers?.get?.('content-type') || '';
      if (!contentType.toLowerCase().includes('json'))
        throw Error('Ожидался JSON от RouterForge: '+path);
      const body = await response.json();
      if (stopped) throw Error('DNS_LITE_DISPOSED');
      if (!response.ok) {
        const reason = typeof body?.error === 'string' ? body.error : 'HTTP '+response.status;
        const error = new Error(reason+' · '+path);
        error.status = response.status;
        throw error;
      }
      return body;
    } catch(e) {
      if (controller.signal.aborted && !stopped) throw Error('DNS_LITE_API_TIMEOUT: '+path);
      throw e;
    } finally {
      clearTimeout(timer);
      controllers.delete(controller);
    }
  }
  return {request, dispose, get disposed(){return stopped;}};
}
