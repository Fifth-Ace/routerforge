/* DNS Runtime R04: explicit, CAS-bound configuration operations.
   No network call on module evaluation, construction or mount. */
let globalOperation = false;

function validSnapshot(readDraft) {
  const snapshot = readDraft();
  if (!snapshot || typeof snapshot.revision !== 'string' || !snapshot.revision ||
      !Number.isSafeInteger(snapshot.generation) || snapshot.generation < 0 ||
      !snapshot.config || typeof snapshot.config !== 'object' || Array.isArray(snapshot.config)) {
    throw Error('Конфигурация ещё не загружена');
  }
  return snapshot;
}

function validRevision(revision) {
  if (typeof revision !== 'string' || !revision.trim()) throw Error('Сервер не подтвердил ревизию конфигурации');
  return revision;
}

export function createConfigFlow({readDraft, request, commit, confirmAction, notify}) {
  for (const fn of [readDraft, request, commit, confirmAction, notify]) {
    if (typeof fn !== 'function') throw Error('CONFIG_FLOW_DEPENDENCY_MISSING');
  }
  async function serial(run) {
    if (globalOperation) throw Error('Другая операция DNS уже выполняется');
    globalOperation = true;
    try { return await run(); }
    finally { globalOperation = false; }
  }
  async function save() {
    return serial(async () => {
      const before = validSnapshot(readDraft);
      const response = await request('config', {
        method: 'PUT',
        headers: {'Content-Type':'application/json', 'X-RouterForge-Action':'save',
          'If-Match':'"'+before.revision+'"'},
        body: JSON.stringify(before.config)
      });
      const nextRevision = validRevision(response?.revision);
      const result = commit('saved', {
        previousRevision:before.revision, revision:nextRevision, generation:before.generation
      });
      notify(result?.stale
        ? 'Сохранена отправленная версия; новые локальные изменения остаются в черновике'
        : 'Сохранено атомарно; работающий shadow не изменён до явного применения');
      return result;
    });
  }
  async function reload() {
    return serial(async () => {
      const before = validSnapshot(readDraft);
      if (before.dirty && !confirmAction('Отменить несохранённые изменения?')) return {cancelled:true};
      const response = await request('config');
      if (!response?.config || typeof response.config !== 'object' || Array.isArray(response.config)) {
        throw Error('Неверный ответ config');
      }
      validRevision(response.revision);
      // Do not discard any edits made while the GET was in flight.
      const after = validSnapshot(readDraft);
      if (after.generation !== before.generation || after.revision !== before.revision || after.dirty !== before.dirty) {
        throw Error('Черновик изменён во время загрузки; перезапись отменена');
      }
      commit('loaded', {previousRevision:before.revision, revision:response.revision,
        generation:before.generation, dirty:before.dirty, config:response.config});
      notify('Конфигурация загружена · runtime меняется только при ручном старте/применении');
      return {loaded:true};
    });
  }
  async function previewImport(fields) {
    return serial(async () => {
      const before = validSnapshot(readDraft);
      if (before.dirty) throw Error('Сначала сохрани или отменяй текущие изменения перед импортом');
      const values = {
        text: String(fields?.text ?? ''),
        upstream: String(fields?.upstream ?? ''),
        match: String(fields?.match ?? 'suffix'),
        mode: String(fields?.mode ?? 'append'),
        priority: 100
      };
      if (!values.text.trim() || !values.upstream) throw Error('Укажи домены и DNS-пул');
      const response = await request('import-preview', {
        method:'POST', headers:{'Content-Type':'application/json','X-RouterForge-Action':'import-preview'},
        body:JSON.stringify(values)
      });
      if (response?.base_revision !== before.revision) {
        throw Error('Конфигурация изменилась; перечитай её');
      }
      const preview = response.preview;
      if (!preview?.config || typeof preview.config !== 'object' || Array.isArray(preview.config) ||
          !['added','duplicates','ignored','result_rules'].every(key => Number.isSafeInteger(preview[key]) && preview[key]>=0)) {
        throw Error('Неполный результат предпросмотра импорта');
      }
      const summary = `Добавлено: ${preview.added}, дубликатов: ${preview.duplicates}, пропущено: ${preview.ignored}, всего правил: ${preview.result_rules}`;
      notify(summary);
      if (!confirmAction('Применить импорт к НЕСОХРАНЁННОМУ проекту? DNS-трафик не изменится.')) {
        return {summary, staged:false};
      }
      const after = validSnapshot(readDraft);
      if (after.revision !== before.revision || after.generation !== before.generation || after.dirty) {
        throw Error('Черновик изменён во время импорта; действие отменено');
      }
      commit('imported', {previousRevision:before.revision, generation:before.generation,
        config:preview.config});
      notify(summary+' · черновик не сохранён, DNS-трафик не изменён');
      return {summary, staged:true};
    });
  }
  return {save, reload, previewImport};
}
