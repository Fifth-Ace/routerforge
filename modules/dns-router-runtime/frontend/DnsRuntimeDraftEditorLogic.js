/* RouterForge R05: Svelte-owned, in-memory DNS draft mutation.
 * No HTTP, no ndmc, no system DNS changes, no mutation during import.
 * The legacy draft CAS owner commits the exact next snapshot atomically.
 */
const TRANSPORTS = new Set(['udp','tcp','dot','doh','doq','native']);
const MATCHES = new Set(['exact','suffix','subdomains']);
const NATIVE = /^native:\/\/(System|Policy(?:0|[1-9][0-9]{0,2}))\/[^/]+\/[^/]+$/;
function text(value) { return String(value ?? '').trim(); }
function fail(message) { throw Error(message); }
function snapshot(read) {
  const state = read();
  if (!state || !text(state.revision) || !Number.isSafeInteger(state.generation) ||
      state.generation < 0 || !state.config || !Array.isArray(state.config.upstreams) ||
      !Array.isArray(state.config.rules)) fail('Черновик DNS ещё не загружен');
  return state;
}
function editTarget(items,id) {
  if (!id) return null;
  const index = items.findIndex(item => item.id === id);
  if (index < 0) fail('Редактируемый объект больше не существует');
  return index;
}
function requiredId(value) {
  const id = text(value);
  if (!id || id.length > 64 || !/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(id)) fail('Недопустимый ID (1–64 символа, буквы, цифры, ., _, -)');
  return id;
}
function requiredDomain(value) {
  const domain = text(value).toLowerCase().replace(/\.$/,'');
  if (!domain || domain.length > 253 || domain.includes('://') || /[\s/*#]/.test(domain) ||
      !/^[a-z0-9_-]+(?:\.[a-z0-9_-]+)*$/.test(domain)) fail('Недопустимый домен');
  return domain;
}
function upstreamDraft(config,values,editId) {
  const id = requiredId(values.id);
  const idx = editTarget(config.upstreams,editId);
  if (idx !== null && id !== editId) fail('Нельзя менять ID существующего DNS-пула');
  if (idx === null && config.upstreams.some(x => x.id === id)) fail('ID пула уже существует');
  const transport = text(values.transport);
  if (!TRANSPORTS.has(transport)) fail('Неподдерживаемый протокол DNS');
  const address = text(values.address);
  if (!address || address.length > 1024) fail('Адрес DNS не указан или слишком длинный');
  const serverName = text(values.server_name), path = text(values.path);
  const fallbacks = text(values.fallbacks).split(',').map(x=>x.trim()).filter(Boolean);
  if (fallbacks.length !== new Set(fallbacks).size || fallbacks.includes(id)) fail('Некорректная резервная цепочка DNS');
  if (transport === 'native' && (serverName || path || fallbacks.length || !NATIVE.test(address)))
    fail('Native: нужен точный System/PolicyN селектор без TLS и резервных DNS');
  if (idx !== null && (config.upstreams[idx].transport !== transport ||
      config.upstreams[idx].address !== address ||
      text(config.upstreams[idx].server_name) !== serverName ||
      text(config.upstreams[idx].path) !== (transport === 'doh' ? path : ''))) {
    // Changing the protocol or endpoint invalidates the previous certificate identity.
    // Preserve unrelated endpoint metadata, but never stale trust pinning.
    const existing = config.upstreams[idx];
    delete existing.certificate_pem;
    delete existing.certificate_sha256;
  }
  const next = idx === null ? {id,transport,address} : {...config.upstreams[idx],id,transport,address};
  delete next.server_name; delete next.path; delete next.fallbacks;
  if (['dot','doh','doq'].includes(transport) && serverName) next.server_name=serverName;
  if (transport === 'doh' && path) next.path=path;
  if (fallbacks.length) next.fallbacks=fallbacks;
  if (idx === null) config.upstreams.push(next); else config.upstreams[idx]=next;
  return idx === null ? 'DNS-пул добавлен в черновик' : 'DNS-пул изменён в черновике';
}
function ruleDraft(config,values,editId) {
  const id = requiredId(values.id);
  const idx = editTarget(config.rules,editId);
  if (idx !== null && id !== editId) fail('Нельзя менять ID существующего правила');
  if (idx === null && config.rules.some(x=>x.id===id)) fail('ID правила уже существует');
  const domain = requiredDomain(values.domain);
  const match = text(values.match), upstream=text(values.upstream);
  if (!MATCHES.has(match)) fail('Недопустимый тип доменного совпадения');
  if (!config.upstreams.some(x=>x.id===upstream)) fail('DNS-пул не найден');
  const priority=Number(values.priority);
  if (!Number.isSafeInteger(priority) || priority<0 || priority>1000000) fail('Приоритет должен быть 0–1000000');
  if (idx !== null && config.rules[idx].list_id &&
      (match!==config.rules[idx].match || upstream!==config.rules[idx].upstream || priority!==config.rules[idx].priority))
    fail('DNS-пул, совпадение и приоритет правил списка меняются через настройки списка');
  const next = idx === null ? {id,domain,match,upstream,priority} : {...config.rules[idx],domain,match,upstream,priority};
  if (idx === null) config.rules.push(next); else config.rules[idx]=next;
  return idx === null ? 'Правило добавлено в черновик' : 'Правило изменено в черновике';
}
function listenerDraft(config,values) {
  const mode=text(values.mode);
  if (mode!=='loopback' && mode!=='lan') fail('Недопустимый режим listener');
  const listen=text(values.listen);
  if (!listen || listen.length>255 || !/^[^\s]+:[0-9]{1,5}$/.test(listen)) fail('Недопустимый адрес listener');
  const port=Number(listen.slice(listen.lastIndexOf(':')+1));
  if (!Number.isSafeInteger(port) || port<1 || port>65535 || port===53) fail('Порт listener недопустим или занят системным DNS');
  const next={...(config.listener||{}),mode,listen};
  delete next.allowed_clients;
  if (mode==='lan') {
    const allowed=text(values.allowed_clients).split(',').map(s=>s.trim()).filter(Boolean);
    if (!allowed.length || allowed.length!==new Set(allowed).size) fail('Для LAN требуется уникальный список разрешённых клиентов');
    next.allowed_clients=allowed;
  }
  config.listener=next;
  return 'Listener добавлен в черновик · ничего не запущено';
}
export function createDraftEditor({readDraft,commit,notify}) {
  if (typeof readDraft!=='function'||typeof commit!=='function'||typeof notify!=='function') fail('DNS_EDITOR_DEPENDENCY_MISSING');
  function apply(kind,values,editId='') {
    const before=snapshot(readDraft);
    const config=structuredClone(before.config);
    let message;
    if (kind==='upstream') message=upstreamDraft(config,values,editId);
    else if (kind==='rule') message=ruleDraft(config,values,editId);
    else if (kind==='listener') {
      if (editId) fail('Listener не принимает ID');
      message=listenerDraft(config,values);
    } else fail('Неизвестная операция редактора DNS');
    // CAS and generation are checked again by the owner before committing.
    const outcome=commit('edited',{previousRevision:before.revision,generation:before.generation,config});
    notify(message+' · работающий DNS не изменён');
    return outcome;
  }
  return {apply};
}
