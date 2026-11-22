import { rfUi } from './rf-localization.js';
/* DNS Runtime R04: explicit, CAS-bound configuration operations.
   No network call on module evaluation, construction or mount. */
let globalOperation = false;
let viewEpoch = 0;

// Called on real root unmount; an uncertain in-flight write must be reconciled by a new GET.
export function invalidateConfigFlows() { viewEpoch++; }
function requireCurrentView(epoch) {
  if (epoch !== viewEpoch) throw Error(rfUi('Операция завершилась после закрытия интерфейса; обнови состояние перед повтором',"The operation completed after the interface closed; refresh status before retrying"));
}

function validSnapshot(readDraft) {
  const snapshot = readDraft();
  if (!snapshot || typeof snapshot.revision !== 'string' || !snapshot.revision ||
      !Number.isSafeInteger(snapshot.generation) || snapshot.generation < 0 ||
      !snapshot.config || typeof snapshot.config !== 'object' || Array.isArray(snapshot.config)) {
    throw Error(rfUi('Конфигурация ещё не загружена',"Configuration has not loaded yet"));
  }
  return snapshot;
}

function validRevision(revision) {
  if (typeof revision !== 'string' || !revision.trim()) throw Error(rfUi('Сервер не подтвердил ревизию конфигурации',"The server did not confirm the configuration revision"));
  return revision;
}

export function createConfigFlow({readDraft, request, commit, confirmAction, notify}) {
  for (const fn of [readDraft, request, commit, confirmAction, notify]) {
    if (typeof fn !== 'function') throw Error('CONFIG_FLOW_DEPENDENCY_MISSING');
  }
  async function serial(run) {
    if (globalOperation) throw Error(rfUi('Другая операция DNS уже выполняется',"Another DNS operation is already running"));
    globalOperation = true;
    try { return await run(); }
    finally { globalOperation = false; }
  }
  async function save() {
    return serial(async () => {
      const before = validSnapshot(readDraft);
      const epoch = viewEpoch;
      const response = await request('config', {
        method: 'PUT',
        headers: {'Content-Type':'application/json', 'X-RouterForge-Action':'save',
          'If-Match':'"'+before.revision+'"'},
        body: JSON.stringify(before.config)
      });
      requireCurrentView(epoch);
      const nextRevision = validRevision(response?.revision);
      const result = commit('saved', {
        previousRevision:before.revision, revision:nextRevision, generation:before.generation
      });
      notify(result?.stale
        ? rfUi('Сохранена отправленная версия; новые локальные изменения остаются в черновике',"Submitted revision saved; newer local changes remain in draft")
        : rfUi('Сохранено атомарно; работающий shadow не изменён до явного применения',"Saved atomically; running test DNS unchanged until explicit application"));
      return result;
    });
  }
  async function reload() {
    return serial(async () => {
      const before = validSnapshot(readDraft);
      const epoch = viewEpoch;
      if (before.dirty && !confirmAction(rfUi('Отменить несохранённые изменения?',"Discard unsaved changes?"))) return {cancelled:true};
      const response = await request('config');
      requireCurrentView(epoch);
      if (!response?.config || typeof response.config !== 'object' || Array.isArray(response.config)) {
        throw Error(rfUi('Неверный ответ config',"Invalid configuration response"));
      }
      validRevision(response.revision);
      // Do not discard any edits made while the GET was in flight.
      const after = validSnapshot(readDraft);
      if (after.generation !== before.generation || after.revision !== before.revision || after.dirty !== before.dirty) {
        throw Error(rfUi('Черновик изменён во время загрузки; перезапись отменена',"Draft changed while loading; overwrite cancelled"));
      }
      commit('loaded', {previousRevision:before.revision, revision:response.revision,
        generation:before.generation, dirty:before.dirty, config:response.config});
      notify(rfUi('Конфигурация загружена · runtime меняется только при ручном старте/применении',"Configuration loaded · service changes only after manual start/apply"));
      return {loaded:true};
    });
  }
  async function previewImport(fields) {
    return serial(async () => {
      const before = validSnapshot(readDraft);
      const epoch = viewEpoch;
      if (before.dirty) throw Error(rfUi('Сначала сохрани или отменяй текущие изменения перед импортом',"Save or discard current changes before importing"));
      const values = {
        text: String(fields?.text ?? ''),
        upstream: String(fields?.upstream ?? ''),
        match: String(fields?.match ?? 'suffix'),
        mode: String(fields?.mode ?? 'append'),
        priority: 100
      };
      if (!values.text.trim() || !values.upstream) throw Error(rfUi('Укажи домены и DNS-пул',"Enter domains and select a DNS pool"));
      const response = await request('import-preview', {
        method:'POST', headers:{'Content-Type':'application/json','X-RouterForge-Action':'import-preview'},
        body:JSON.stringify(values)
      });
      requireCurrentView(epoch);
      if (response?.base_revision !== before.revision) {
        throw Error(rfUi('Конфигурация изменилась; перечитай её',"Configuration changed; reload it"));
      }
      const preview = response.preview;
      if (!preview?.config || typeof preview.config !== 'object' || Array.isArray(preview.config) ||
          !['added','duplicates','ignored','result_rules'].every(key => Number.isSafeInteger(preview[key]) && preview[key]>=0)) {
        throw Error(rfUi('Неполный результат предпросмотра импорта',"Incomplete import preview result"));
      }
      const summary = rfUi(`Добавлено: ${preview.added}, дубликатов: ${preview.duplicates}, пропущено: ${preview.ignored}, всего правил: ${preview.result_rules}`,`Added: ${preview.added}, duplicates: ${preview.duplicates}, skipped: ${preview.ignored}, total rules: ${preview.result_rules}`);
      notify(summary);
      if (!confirmAction(rfUi('Применить импорт к НЕСОХРАНЁННОМУ проекту? DNS-трафик не изменится.',"Apply imported rules to the UNSAVED draft? DNS traffic will not change."))) {
        return {summary, staged:false};
      }
      requireCurrentView(epoch);
      const after = validSnapshot(readDraft);
      if (after.revision !== before.revision || after.generation !== before.generation || after.dirty) {
        throw Error(rfUi('Черновик изменён во время импорта; действие отменено',"Draft changed during import; action cancelled"));
      }
      commit('imported', {previousRevision:before.revision, generation:before.generation,
        config:preview.config});
      notify(summary+rfUi(' · черновик не сохранён, DNS-трафик не изменён',"· draft not saved, DNS traffic unchanged"));
      return {summary, staged:true};
    });
  }
  return {save, reload, previewImport};
}
