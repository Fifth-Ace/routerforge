/* Native DNS Runtime managed-trial flow. No implicit network or router changes.
   CAS-bound prepare; double confirmation; token consumed before trial request. */
export function createManagedFlow({readDraft, request, confirmAction, promptAction, notify}) {
  let prepared = null;
  let running = false;
  let status = 'Применение запрещено до проверки текущего плана';

  const snapshot = () => ({prepared: !!prepared, running, status});
  function gate() {
    const draft = readDraft();
    if (!draft || typeof draft.revision !== 'string' || !draft.revision) {
      throw Error('Конфигурация ещё не загружена');
    }
    if (draft.dirty) throw Error('Сначала сохрани конфигурацию для сверки зон');
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
      if (!result || result.revision !== revision) throw Error('Ревизия изменилась');
      status = JSON.stringify(result, null, 2);
      if (!result.ready) {
        notify('Найдены конфликты с действующей конфигурацией. Изменений нет.');
        return snapshot();
      }
      if (typeof result.token !== 'string' || !result.token || !Array.isArray(result.zones)) {
        throw Error('Неполный план испытания: токен или зоны отсутствуют');
      }
      prepared = result;
      notify('План проверен. Применение ещё не выполнялось. Для временной проверки нажми «Испытать».');
      return snapshot();
    } catch (error) {
      prepared = null;
      status = 'DPR2-N prepare: ' + String(error.message || error);
      notify(status);
      throw error;
    } finally { running = false; }
  }
  async function trial() {
    if (running) return snapshot();
    const revision = gate();
    if (!prepared || !prepared.ready || prepared.revision !== revision) {
      throw Error('Нужен актуальный подтверждённый план без несохранённых изменений');
    }
    const zones = prepared.zones.map(x => String(x?.domain || '')).join(', ');
    if (!confirmAction('ВНИМАНИЕ: DSTest на короткое время зарегистрирует DNS-зоны ' + zones +
       ' через ndmc. После проверки они будут удалены. Продолжить?')) return snapshot();
    if (promptAction('Для запуска временной DNS-транзакции введи TRIAL:') !== 'TRIAL') {
      notify('Испытание отменено; изменений нет');
      return snapshot();
    }
    // Token is consumed BEFORE the network call, even when the call throws.
    const token = prepared.token;
    prepared = null;
    running = true;
    status = 'Транзакция выполняется. Ожидается подтверждение DNS и отката…';
    try {
      const result = await post('trial', revision, token);
      status = JSON.stringify(result, null, 2);
      notify(result?.baseline_restored === true && result?.state === 'pass'
        ? 'DPR2-N: временное применение и точный откат подтверждены'
        : 'DPR2-N: результат неоднозначный, проверь журнал/откат');
      return snapshot();
    } catch (error) {
      status = 'DPR2-N: ' + String(error.message || error) + '; проверь DNS-снимок перед повтором';
      notify(status);
      throw error;
    } finally { running = false; }
  }
  return {prepare, trial, snapshot};
}
