<script>
  import { onMount } from 'svelte';

  const labels = [
    { id: 'knockd', name: 'knockd' },
    { id: 'fwknopd', name: 'fwknopd / SPA' },
    { id: 'iptables-recent', name: 'iptables recent' }
  ];
  const safetyLabels = {
    recent_match: 'iptables recent', filter_table: 'Таблица filter',
    ndm_forward_chain: 'Цепочки NDM / FORWARD', management_rescue: 'Резервный SSH-доступ',
    rollback: 'Автоматический откат', ndm_chain_evidence: 'Цепочки NDM (наблюдение)',
    ssh_listener_evidence: 'SSH (слушающие порты)', wan_route_evidence: 'IPv4-маршрут (наблюдение)',
    wan_scope: 'WAN / NAT / защищаемый порт'
  };
  let engines = $state({});
  let statusMessage = $state('Читаем состояние…');
  let statusError = $state('');
  let checks = $state([]);
  let preflightMessage = $state('Запрашиваем read-only preflight…');
  let preflightBlocked = $state(false);
  let refreshingStatus = $state(false);
  let refreshingPreflight = $state(false);

  function statusURL() {
    const path = window.location.pathname;
    let m = path.match(/^(.*\/api\/modules\/port-access-manager)\/(?:v1\/)?ui(?:\/.*)?$/);
    if (m) return m[1] + '/status';
    m = path.match(/^(.*)\/v1\/ui(?:\/.*)?$/);
    if (m) return m[1] + '/v1/status';
    return '/api/modules/port-access-manager/status';
  }
  async function json(url) {
    const response = await fetch(url, {
      credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' }
    });
    if (!response.ok) throw new Error('HTTP ' + response.status);
    const type = response.headers.get('content-type') || '';
    if (!type.includes('application/json')) throw new Error('Ожидался JSON, получен ' + type);
    return response.json();
  }
  async function refreshStatus() {
    refreshingStatus = true;
    statusMessage = 'Получаем состояние…';
    statusError = '';
    try {
      const data = await json(statusURL());
      if (data.module !== 'port-access-manager' || !Array.isArray(data.engines)) {
        throw new Error('Неверный ответ API');
      }
      const next = {};
      for (const engine of data.engines) next[engine.id] = engine;
      engines = next;
      statusMessage = 'Связь с Core: OK · режим ' + (data.mode || 'неизвестен');
    } catch (e) {
      engines = {};
      statusError = 'Нет данных от службы. Настройки по-прежнему не применяются.';
      statusMessage = 'Ошибка диагностики: ' + e.message;
    } finally { refreshingStatus = false; }
  }
  function localizeSafetyDetail(check) {
    const detail = String(check.detail || 'Нет данных');
    const messages = {
      'Kernel recent match is loaded; this does not prove firewall protection': 'Модуль recent загружен; это не подтверждает действующую защиту firewall.',
      'recent match is not visible; capability has not been proven': 'Модуль recent не обнаружен; поддержка не подтверждена.',
      'Cannot read kernel table registry': 'Не удалось прочитать список таблиц ядра.',
      'Kernel filter table is visible; no rule order has been verified': 'Таблица filter обнаружена, но порядок правил не проверен.',
      'Filter table is not visible in kernel table registry': 'Таблица filter не обнаружена.',
      'Keenetic NDM forwarding chain order and persistence have not been verified': 'Порядок цепочек NDM и сохранение правил после перезапуска не проверены.',
      'SSH 22/2222, active management path and an out-of-band rescue have not been verified': 'Порты SSH, действующий канал управления и резервный доступ не проверены.',
      'No timed rollback transaction has been staged or tested': 'Транзакционный откат по таймеру ещё не подготовлен и не проверен на роутере.',
      'WAN interface, protected target and NAT forwarding have not been verified': 'WAN-интерфейс, защищаемый адрес и проброс NAT не проверены.'
    };
    if (messages[detail]) return messages[detail];
    return detail
      .replace(/^Visible filter chains: /, 'Обнаружены цепочки filter: ')
      .replace(/^TCP LISTEN on port\(s\) /, 'TCP-порты в состоянии LISTEN: ')
      .replace(/^Default IPv4 route interface: /, 'Интерфейс маршрута IPv4 по умолчанию: ')
      .replace(/; insertion order and NDM persistence NOT verified/g, '; порядок правил и сохранение NDM НЕ проверены')
      .replace(/; reachability, authenticated session and rescue NOT verified/g, '; доступность, авторизация и резервный доступ НЕ проверены')
      .replace(/; physical WAN identity, NAT target and forwarding NOT verified/g, '; физический WAN, адрес NAT и проброс НЕ проверены');
  }
  async function refreshPreflight() {
    refreshingPreflight = true;
    preflightMessage = 'Запрашиваем read-only preflight…';
    preflightBlocked = false;
    checks = [];
    try {
      const data = await json(statusURL().replace(/\/status$/, '/preflight'));
      if (data.module !== 'port-access-manager' || data.mutation_api !== false ||
          data.ready_for_apply !== false || !Array.isArray(data.checks)) {
        throw new Error('Неподтверждённый read-only контракт');
      }
      checks = data.checks;
      preflightMessage = 'ПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО';
      preflightBlocked = true;
    } catch (e) {
      preflightBlocked = true;
      preflightMessage = 'Preflight недоступен: ' + e.message + '. Применение недоступно.';
    } finally { refreshingPreflight = false; }
  }
  onMount(() => { void refreshStatus(); void refreshPreflight(); });
</script>

<section class="overview">
  <div>
    <span class="section-label">Состояние движков</span>
    <div class="cards" aria-live="polite">
      {#each labels as item (item.id)}
        {@const data = engines[item.id]}
        <article class="engine-card">
          <strong>{item.name}</strong>
          {#if !data}
            <p class="muted">Статус неизвестен</p>
          {:else if item.id === 'iptables-recent'}
            <p class:good={data.installed} class:muted={!data.installed}>{data.installed ? 'Поддержка ядра обнаружена' : 'Поддержка ядра не обнаружена'}</p>
            <p class="muted">Наличие recent ≠ активная защита</p>
          {:else}
            <p class:good={data.installed} class:muted={!data.installed}>{data.installed ? 'Установлен' : 'Не установлен'}</p>
            <p class="muted">{data.running ? 'Служба работает' : 'Служба не запущена'}</p>
            <p class="muted">{data.configuration_present ? 'Файл конфигурации найден' : 'Файл конфигурации не найден'}</p>
          {/if}
        </article>
      {/each}
    </div>
    {#if statusError}<p class="muted" role="alert">{statusError}</p>{/if}
  </div>
  <div class="refresh-bar">
    <button type="button" onclick={refreshStatus} disabled={refreshingStatus}>↻ Обновить диагностику</button>
    <span role="status">{statusMessage}</span>
  </div>
</section>
<section class="panel safety-panel" aria-labelledby="preflight-title">
  <div class="preview-top">
    <strong id="preflight-title">Проверка безопасности перед применением</strong>
    <span class="safety-verdict" class:safety-blocked={preflightBlocked} role="status">{preflightBlocked ? 'ПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО' : 'Ожидаем данные'}</span>
  </div>
  <p class="help">Только чтение. Даже при обнаруженной поддержке ядра изменение firewall заблокировано до проверки NDM, SSH, WAN и отката.</p>
  <div class="safety-checks" aria-live="polite">
    {#if checks.length}
      {#each checks as check, index (check.id + ':' + index)}
        <div class="safety-check">
          <strong>{safetyLabels[check.id] || check.id}</strong>
          <span class={check.state === 'present' || check.state === 'observed' ? 'safety-present' : 'safety-warning'}>
            {check.state === 'present' ? 'Обнаружено' : check.state === 'observed' ? 'Наблюдается' : 'Требует проверки'}
          </span>
          <small class="muted">{localizeSafetyDetail(check)}</small>
        </div>
      {/each}
    {:else}
      <p class="muted" role="status">{preflightMessage}</p>
    {/if}
  </div>
  <button type="button" onclick={refreshPreflight} disabled={refreshingPreflight}>↻ Проверить безопасность</button>
</section>
