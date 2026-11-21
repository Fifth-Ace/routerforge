import { rfUi } from './rf-localization.js';
/* Pure explicit-action logic; never contacts network on import/mount. */
export async function evaluateDomain(domain, request) {
  const normalized = String(domain ?? '').trim();
  if (!normalized) throw Error(rfUi('Укажи домен для проверки',"Enter a domain to test"));
  const response = await request('evaluate', {
    method: 'POST',
    headers: {'Content-Type':'application/json', 'X-RouterForge-Action':'evaluate'},
    body: JSON.stringify({domain:normalized})
  });
  if (!response || !Object.prototype.hasOwnProperty.call(response,'decision'))
    throw Error(rfUi('Результат симуляции не получен',"No simulation result received"));
  return response.decision;
}

export async function exportSavedConfig(request, {BlobConstructor, URLObject, documentObject}) {
  const response = await request('config');
  if (!response || !response.config || typeof response.config !== 'object')
    throw Error(rfUi('Сохранённая конфигурация не получена',"Saved configuration not received"));
  const blob = new BlobConstructor([JSON.stringify(response.config, null, 2) + '\n'], {type:'application/json'});
  const url = URLObject.createObjectURL(blob);
  try {
    const anchor = documentObject.createElement('a');
    anchor.href = url;
    anchor.download = 'routerforge-dns-router.json';
    anchor.click();
  } finally {
    URLObject.revokeObjectURL(url);
  }
}
