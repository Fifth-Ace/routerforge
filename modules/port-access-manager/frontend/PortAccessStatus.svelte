<script>
  import { rfUi } from './rf-localization.js';
  import { onMount } from 'svelte';
  import { createPortAccessReadSession } from './PortAccessReadSession.js';
  const readSession = createPortAccessReadSession();

  const labels = [
    { id: 'knockd', name: 'knockd' },
    { id: 'fwknopd', name: 'fwknopd / SPA' },
    { id: 'iptables-recent', name: 'iptables recent' }
  ];
  const safetyLabels = {
    recent_match: 'iptables recent', filter_table: rfUi('Таблица filter',"Filter table"),
    ndm_forward_chain: rfUi('Цепочки NDM / FORWARD',"NDM / FORWARD chains"), management_rescue: rfUi('Резервный SSH-доступ',"Backup SSH access"),
    rollback: rfUi('Автоматический откат',"Automatic rollback"), ndm_chain_evidence: rfUi('Цепочки NDM (наблюдение)',"NDM chains (read-only observation)"),
    ssh_listener_evidence: rfUi('SSH (слушающие порты)',"SSH (listening ports)"), wan_route_evidence: rfUi('IPv4-маршрут (наблюдение)',"IPv4 route (read-only observation)"),
    wan_scope: rfUi('WAN / NAT / защищаемый порт',"WAN / NAT / protected port")
  };
  let engines = $state({});
  let statusMessage = $state(rfUi('Читаем состояние…',"Reading status…"));
  let statusError = $state('');
  let checks = $state([]);
  let preflightMessage = $state(rfUi('Запрашиваем read-only preflight…',"Running read-only safety checks…"));
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
    return readSession.consume(url, {
      credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' }
    }, async response => {
      if (!response.ok) throw new Error('HTTP ' + response.status);
      const type = response.headers.get('content-type') || '';
      if (!type.includes('application/json')) throw new Error(rfUi('Ожидался JSON, получен ',"Expected JSON, received") + type);
      return response.json();
    });
  }
  async function refreshStatus() {
    refreshingStatus = true;
    statusMessage = rfUi('Получаем состояние…',"Loading status…");
    statusError = '';
    try {
      const data = await json(statusURL());
      if (data.module !== 'port-access-manager' || !Array.isArray(data.engines)) {
        throw new Error(rfUi('Неверный ответ API',"Invalid API response"));
      }
      const next = {};
      for (const engine of data.engines) next[engine.id] = engine;
      engines = next;
      statusMessage = rfUi('Связь с Core: OK · режим ',"Core connection: OK · mode") + (data.mode || rfUi('неизвестен',"unknown"));
    } catch (e) {
      engines = {};
      statusError = rfUi('Нет данных от службы. Настройки по-прежнему не применяются.',"No service data is available. Settings are not being applied.");
      statusMessage = rfUi('Ошибка диагностики: ',"Diagnostic error:") + e.message;
    } finally { refreshingStatus = false; }
  }
  function localizeSafetyDetail(check) {
    const detail = String(check.detail || rfUi('Нет данных',"No data"));
    const messages = {
      'Kernel recent match is loaded; this does not prove firewall protection': rfUi('Модуль recent загружен; это не подтверждает действующую защиту firewall.',"The recent module is loaded; this does not prove firewall protection is active."),
      'recent match is not visible; capability has not been proven': rfUi('Модуль recent не обнаружен; поддержка не подтверждена.',"The recent module was not found; support is unverified."),
      'Cannot read kernel table registry': rfUi('Не удалось прочитать список таблиц ядра.',"Could not read the kernel routing table list."),
      'Kernel filter table is visible; no rule order has been verified': rfUi('Таблица filter обнаружена, но порядок правил не проверен.',"Filter table found, but rule order not verified."),
      'Filter table is not visible in kernel table registry': rfUi('Таблица filter не обнаружена.',"Filter table not found."),
      'Keenetic NDM forwarding chain order and persistence have not been verified': rfUi('Порядок цепочек NDM и сохранение правил после перезапуска не проверены.',"NDM chain order and rule persistence after reboot have not been verified."),
      'SSH 22/2222, active management path and an out-of-band rescue have not been verified': rfUi('Порты SSH, действующий канал управления и резервный доступ не проверены.',"SSH ports, active management connection, and backup access have not been verified."),
      'No timed rollback transaction has been staged or tested': rfUi('Транзакционный откат по таймеру ещё не подготовлен и не проверен на роутере.',"Timed transactional rollback has not been prepared or verified on the router."),
      'WAN interface, protected target and NAT forwarding have not been verified': rfUi('WAN-интерфейс, защищаемый адрес и проброс NAT не проверены.',"WAN interface, protected address, and NAT forwarding have not been verified.")
    };
    if (messages[detail]) return messages[detail];
    return detail
      .replace(/^Visible filter chains: /, rfUi('Обнаружены цепочки filter: ',"Filter chains found:"))
      .replace(/^TCP LISTEN on port\(s\) /, rfUi('TCP-порты в состоянии LISTEN: ',"Listening TCP ports:"))
      .replace(/^Default IPv4 route interface: /, rfUi('Интерфейс маршрута IPv4 по умолчанию: ',"Default IPv4 route interface:"))
      .replace(/; insertion order and NDM persistence NOT verified/g, rfUi('; порядок правил и сохранение NDM НЕ проверены',"; rule order and NDM persistence NOT verified"))
      .replace(/; reachability, authenticated session and rescue NOT verified/g, rfUi('; доступность, авторизация и резервный доступ НЕ проверены',"; connectivity, authorization, and backup access NOT verified"))
      .replace(/; physical WAN identity, NAT target and forwarding NOT verified/g, rfUi('; физический WAN, адрес NAT и проброс НЕ проверены',"; physical WAN, NAT address, and port forwarding NOT verified"));
  }
  async function refreshPreflight() {
    refreshingPreflight = true;
    preflightMessage = rfUi('Запрашиваем read-only preflight…',"Running read-only safety checks…");
    preflightBlocked = false;
    checks = [];
    try {
      const data = await json(statusURL().replace(/\/status$/, '/preflight'));
      if (data.module !== 'port-access-manager' || data.mutation_api !== false ||
          data.ready_for_apply !== false || !Array.isArray(data.checks)) {
        throw new Error(rfUi('Неподтверждённый read-only контракт',"Read-only safety contract unverified"));
      }
      checks = data.checks;
      preflightMessage = rfUi('ПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО',"APPLYING BLOCKED");
      preflightBlocked = true;
    } catch (e) {
      preflightBlocked = true;
      preflightMessage = rfUi('Preflight недоступен: ',"Pre-apply check unavailable:") + e.message + rfUi('. Применение недоступно.',". Applying changes is unavailable.");
    } finally { refreshingPreflight = false; }
  }
  onMount(() => { void refreshStatus(); void refreshPreflight(); return () => readSession.close(); });
</script>

<section class="overview">
  <div>
    <span class="section-label">{rfUi("Состояние движков","Engine status")}</span>
    <div class="cards" aria-live="polite">
      {#each labels as item (item.id)}
        {@const data = engines[item.id]}
        <article class="engine-card">
          <strong>{item.name}</strong>
          {#if !data}
            <p class="muted">{rfUi("Статус неизвестен","Status unknown")}</p>
          {:else if item.id === 'iptables-recent'}
            <p class:good={data.installed} class:muted={!data.installed}>{data.installed ? 'Поддержка ядра обнаружена' : 'Поддержка ядра не обнаружена'}</p>
            <p class="muted">{rfUi("Наличие recent ≠ активная защита","A recent entry does not prove active protection")}</p>
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
    <button type="button" onclick={refreshStatus} disabled={refreshingStatus}>{rfUi("↻ Обновить диагностику","↻ Refresh diagnostics")}</button>
    <span role="status">{statusMessage}</span>
  </div>
</section>
<section class="panel safety-panel" aria-labelledby="preflight-title">
  <div class="preview-top">
    <strong id="preflight-title">{rfUi("Проверка безопасности перед применением","Pre-apply safety check")}</strong>
    <span class="safety-verdict" class:safety-blocked={preflightBlocked} role="status">{preflightBlocked ? 'ПРИМЕНЕНИЕ ЗАБЛОКИРОВАНО' : 'Ожидаем данные'}</span>
  </div>
  <p class="help">{rfUi("Только чтение. Даже при обнаруженной поддержке ядра изменение firewall заблокировано до проверки NDM, SSH, WAN и отката.","Read only. Even when kernel support is detected, firewall changes remain blocked until NDM, SSH, WAN, and rollback checks succeed.")}</p>
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
  <button type="button" onclick={refreshPreflight} disabled={refreshingPreflight}>{rfUi("↻ Проверить безопасность","↻ Check safety")}</button>
</section>
