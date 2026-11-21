import { rfUi } from './rf-localization.js';
/* Native DNS Runtime managed-trial flow. No implicit network or router changes.
   CAS-bound prepare; double confirmation; token consumed before trial request. */
export function createManagedFlow({readDraft, request, confirmAction, promptAction, notify}) {
  let prepared = null;
  let running = false;
  let status = rfUi('Применение запрещено до проверки текущего плана',"Application blocked until the current plan is verified");

  const snapshot = () => ({prepared: !!prepared, running, status});
  function gate() {
    const draft = readDraft();
    if (!draft || typeof draft.revision !== 'string' || !draft.revision) {
      throw Error(rfUi('Конфигурация ещё не загружена',"Configuration has not loaded yet"));
    }
    if (draft.dirty) throw Error(rfUi('Сначала сохрани конфигурацию для сверки зон',"Save the configuration before comparing zones"));
    return draft.revision;
  }
  function post(operation, revision, token) {
    return request('keenetic/domain-managed/' + operation, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-RouterForge-Action': 'managed-' + operation,
        'If-Match': '"' + revision + '"'
      },
      body: JSON.stringify(operation === 'prepare'
        ? {operation: 'prepare'} : {operation: 'trial', token})
    });
  }
  async function prepare() {
    if (running) return snapshot();
    prepared = null;
    const revision = gate();
    running = true;
    try {
      const result = await post('prepare', revision);
      if (!result || result.revision !== revision) throw Error(rfUi('Ревизия изменилась',"Revision changed"));
      status = JSON.stringify(result, null, 2);
      if (!result.ready) {
        notify(rfUi('Найдены конфликты с действующей конфигурацией. Изменений нет.',"Conflicts found with current configuration. No changes made."));
        return snapshot();
      }
      if (typeof result.token !== 'string' || !result.token || !Array.isArray(result.zones)) {
        throw Error(rfUi('Неполный план испытания: токен или зоны отсутствуют',"Incomplete test plan: token or zones missing"));
      }
      prepared = result;
      notify(rfUi('План проверен. Применение ещё не выполнялось. Для временной проверки нажми «Испытать».',"Plan checked. Nothing has been applied. Choose “Test” to run a temporary trial."));
      return snapshot();
    } catch (error) {
      prepared = null;
      status = rfUi('Ошибка подготовки DNS-теста: ','DNS test preparation error: ') + String(error.message || error);
      notify(status);
      throw error;
    } finally { running = false; }
  }
  async function trial() {
    if (running) return snapshot();
    const revision = gate();
    if (!prepared || !prepared.ready || prepared.revision !== revision) {
      throw Error(rfUi('Нужен актуальный подтверждённый план без несохранённых изменений',"A current verified plan with no unsaved changes is required"));
    }
    const zones = prepared.zones.map(x => String(x?.domain || '')).join(', ');
    if (!confirmAction(rfUi('ВНИМАНИЕ: DSTest на короткое время зарегистрирует DNS-зоны ',"WARNING: the DNS test will temporarily register DNS zones") + zones +
       ' через ndmc. После проверки они будут удалены. Продолжить?')) return snapshot();
    if (promptAction(rfUi('Для запуска временной DNS-транзакции введи TRIAL:',"Enter TRIAL to start the temporary DNS transaction:")) !== 'TRIAL') {
      notify(rfUi('Испытание отменено; изменений нет',"Test canceled; no changes made"));
      return snapshot();
    }
    // Token is consumed BEFORE the network call, even when the call throws.
    const token = prepared.token;
    prepared = null;
    running = true;
    status = rfUi('Транзакция выполняется. Ожидается подтверждение DNS и отката…',"Transaction in progress. Waiting for DNS and rollback verification…");
    try {
      const result = await post('trial', revision, token);
      status = JSON.stringify(result, null, 2);
      notify(result?.baseline_restored === true && result?.state === 'pass'
        ? rfUi('Временное применение и восстановление исходных настроек подтверждены','Temporary application and restoration of original settings verified')
        : rfUi('Результат неоднозначный, проверь журнал и восстановление настроек','Result unclear; check the log and restoration'));
      return snapshot();
    } catch (error) {
      status = rfUi('Ошибка временного DNS-теста: ','Temporary DNS test error: ') + String(error.message || error) + '; проверь DNS-снимок перед повтором';
      notify(status);
      throw error;
    } finally { running = false; }
  }
  return {prepare, trial, snapshot};
}
