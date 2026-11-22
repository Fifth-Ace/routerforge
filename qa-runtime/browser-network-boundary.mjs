// CDP test runner only: deny any browser-origin HTTP request not belonging to the
// exact ephemeral localhost mock API. Never ship this in the RouterForge frontend.
import {createHash} from 'node:crypto';

export function isMockOriginRequest(url, origin) {
  if (url === 'about:blank' || url.startsWith('data:')) return true;
  try {
    const u = new URL(url);
    const expected = new URL(origin);
    if (u.protocol === 'blob:') return u.origin === expected.origin;
    return u.protocol === 'http:' && u.origin === expected.origin;
  } catch {return false;}
}

function hasAppScriptInitiator(initiator, origin) {
  let stack = initiator?.stack;
  while (stack) {
    for (const frame of stack.callFrames || []) {
      const script = String(frame.url || '');
      if (!script) continue;
      if (/(?:^|\/)svelte-mode\.js(?:[?#]|$)|(?:^|\/)component-gate\.js(?:[?#]|$)/i.test(script)) return true;
      try {
        const u = new URL(script);
        if (u.origin === new URL(origin).origin && /\.(?:js|mjs)(?:[?#]|$)/.test(u.pathname)) return true;
      } catch {}
    }
    stack = stack.parent;
  }
  return false;
}

export function isKnownAdGuardInjection(url, initiator, origin) {
  if (hasAppScriptInitiator(initiator, origin)) return false;
  try {
    const u = new URL(url);
    if (u.protocol !== 'http:' || u.hostname !== 'local.adguard.org' || (u.port && u.port !== '80')) return false;
    if (u.pathname === '/' && ['content-script','user-script'].includes(u.searchParams.get('type'))) return true;
    if (u.pathname === '/adguard-ajax-api/api' && u.searchParams.get('type') === 'gm-xml-http-request' && u.searchParams.get('unique_name') === 'Web of Trust') return true;
  } catch {}
  return false;
}

export function redactNetworkUrl(url) {
  if (url.startsWith('data:')) return 'data:[redacted-inline-resource]';
  if (url.length > 3000) return 'unparsed:[redacted-large-url]';
  try {
    const u = new URL(url);
    if (!u.search && !u.hash) return u.href;
    const fingerprint = createHash('sha256').update(url).digest('hex').slice(0,16);
    return u.origin + u.pathname + '?[query/hash redacted sha256:' + fingerprint + ']';
  } catch {return '[invalid-url-redacted]';}
}

export function auditBrowserBoundary(events, denied, origin) {
  const attempts = (events || []).filter(x=>x.method==='Network.requestWillBeSent').map(x=>({
    url:x.params?.request?.url||'', initiator:x.params?.initiator
  })).filter(x=>x.url && !isMockOriginRequest(x.url, origin));
  const blocked = denied || [];
  const bypassed = attempts.filter(a => !blocked.some(b=>b.url===a.url)).map(a=>redactNetworkUrl(a.url));
  const unknown = blocked.filter(b=>!isKnownAdGuardInjection(b.url, attempts.find(a=>a.url===b.url)?.initiator, origin))
    .map(b=>redactNetworkUrl(b.url));
  return {bypassed,unknown,environmentBlocked:blocked.length-unknown.length,
    blockedLog:blocked.map(b=>({url:redactNetworkUrl(b.url),reason:b.reason}))};
}
