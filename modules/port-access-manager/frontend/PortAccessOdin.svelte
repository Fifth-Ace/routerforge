<script>
  import { rfUi } from './rf-localization.js';
  import { onMount } from 'svelte';
  import { createPortAccessReadSession } from './PortAccessReadSession.js';
  const readSession = createPortAccessReadSession();
  let { onStateChanged = () => {} } = $props();

  const endpoint = '/api/modules/port-access-manager/odin';
  const labels = {
    save: rfUi('Сохранить настройки',"Save settings"),
    enable: rfUi('Включить защиту',"Enable protection"),
    confirm: rfUi('Подтвердить защиту',"Confirm protection"),
    disable: rfUi('Выключить защиту',"Disable protection")
  };
  let wan = $state('eth3');
  let ip = $state('192.168.1.2');
  let portText = $state('3389');
  let trustedText = $state('');
  let k1 = $state(7777);
  let k2 = $state(8888);
  let k3 = $state(6666);
  let windowSeconds = $state(15);
  let ttl = $state(28800);
  let data = $state(null);
  let busy = $state(false);
  let status = $state(rfUi('Проверяем…',"Checking…"));
  let lastOperationError = $state('');
  let localDeadline = $state(0);
  let now = $state(Date.now());
  let firstLoad = true;
  let deadlineRefreshRequested = false;
  let disposed = false;
  let requestSerial = 0;

  const enabled = $derived(Boolean(data?.enabled));
  const confirmed = $derived(Boolean(data?.confirmed));
  const configured = $derived(Boolean(data?.configured));
  const secondsLeft = $derived(localDeadline ? Math.max(0, Math.ceil((localDeadline - now) / 1000)) : null);
  const countdown = $derived(!enabled || confirmed ? '' : secondsLeft === null
    ? rfUi('Срок watchdog неизвестен: обнови состояние. Не подтверждай защиту без проверки.',"Watchdog deadline unknown: refresh status. Do not confirm protection without verification.")
    : secondsLeft > 0
      ? rfUi('До автоматического отката: приблизительно ',"Approximate time until automatic rollback:") + secondsLeft + rfUi(' сек.',"sec.")
      : rfUi('Срок подтверждения истёк. Проверяем состояние…',"Confirmation deadline passed. Checking status…"));

  async function request(method, body) {
    return readSession.consume(endpoint, {
      method, credentials: 'same-origin', cache: 'no-store',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: body ? JSON.stringify(body) : undefined
    }, async response => {
      const raw = await response.text();
      let result;
      try { result = raw ? JSON.parse(raw) : {}; }
      catch (_) { throw new Error(rfUi('Некорректный ответ сервера (HTTP ',"Invalid server response (HTTP") + response.status + ')'); }
      if (!response.ok) throw new Error(result.error || result.message || 'HTTP ' + response.status);
      return result;
    });
  }

  function settings() {
    const ports = portText.trim().split(',').map(value => Number(value.trim()));
    return {
      wan: wan.trim(), ip: ip.trim(), port: ports[0], ports,
      trusted: trustedText.trim() ? trustedText.split(/[,\s]+/).filter(Boolean) : [],
      knock: [Number(k1), Number(k2), Number(k3)],
      window: Number(windowSeconds), ttl: Number(ttl)
    };
  }

  function validate(s) {
    const parts = s.ip.split('.');
    if (!/^[a-zA-Z0-9_.:-]+$/.test(s.wan)) return 'Укажи корректный WAN-интерфейс';
    if (parts.length !== 4 || parts.some(part => !/^\d{1,3}$/.test(part) || Number(part) > 255)) return 'Некорректный IPv4 сервиса';
    const ports = s.knock.concat(s.ports);
    if (ports.some(p => !Number.isInteger(p) || p < 1 || p > 65535)) return rfUi('Порты должны быть от 1 до 65535',"Ports must be between 1 and 65535");
    if (s.ports.length < 1 || s.ports.length > 15) return rfUi('От 1 до 15 защищаемых портов',"1–15 protected ports");
    if (s.trusted.length > 32) return rfUi('Не более 32 доверенных IP/CIDR',"Up to 32 trusted IP/CIDR entries");
    if (s.trusted.some(x => !/^[0-9./]+$/.test(x))) return 'Доверенные адреса: только IPv4/CIDR';
    if (new Set(ports).size !== ports.length) return 'Порты стуков и сервиса должны различаться';
    if (!Number.isInteger(s.window) || s.window < 5 || s.window > 300) return rfUi('Окно стуков: 5–300 секунд',"Knock window: 5–300 seconds");
    if (!Number.isInteger(s.ttl) || s.ttl < 30 || s.ttl > 86400) return rfUi('Доступ: 30–86400 секунд',"Access: 30–86400 seconds");
    return '';
  }

  function fillSettings(s) {
    wan = s.wan || '';
    ip = s.ip || '';
    portText = (s.ports?.length ? s.ports : [s.port]).join(', ');
    trustedText = (s.trusted || []).join(', ');
    k1 = s.knock?.[0] ?? 7777;
    k2 = s.knock?.[1] ?? 8888;
    k3 = s.knock?.[2] ?? 6666;
    windowSeconds = s.window;
    ttl = s.ttl;
  }

  function applyServerState(next, fill = false) {
    if (disposed) return;
    const changed = !data || Boolean(data.enabled) !== Boolean(next.enabled) || Boolean(data.confirmed) !== Boolean(next.confirmed);
    const serverDeadline = Number(next.watchdog_deadline_unix) || 0;
    const serverNow = Number(next.server_unix) || 0;
    localDeadline = next.enabled && !next.confirmed && serverDeadline > 0 && serverNow > 0
      ? Date.now() + Math.max(0, serverDeadline - serverNow) * 1000 : 0;
    if (localDeadline > Date.now()) deadlineRefreshRequested = false;
    data = next;
    if (fill && next.configured && next.settings) fillSettings(next.settings);
    status = lastOperationError || (next.enabled
      ? (next.confirmed ? rfUi('Защита включена · подтверждена',"Protection enabled · confirmed") : rfUi('Защита включена · ожидает подтверждения',"Protection enabled · awaiting confirmation"))
      : rfUi('Защита выключена',"Protection disabled"));
    if (changed) onStateChanged();
  }

  async function refresh() {
    const serial = ++requestSerial;
    try {
      const response = await request('GET');
      if (disposed || serial !== requestSerial) return null;
      applyServerState(response, firstLoad);
      firstLoad = false;
      return response;
    } catch (error) {
      if (!disposed && serial === requestSerial) {
        status = lastOperationError || rfUi('Ошибка получения состояния: ',"Status retrieval error:") + error.message;
        data = null; // Fail closed: no enable/confirm until verified again.
        onStateChanged();
      }
      return null;
    }
  }

  async function perform(action) {
    if (busy) return;
    if (action === 'refresh') { await refresh(); return; }
    if (!data || (action === 'enable' && !configured)) return;
    const s = settings();
    if (action === 'save') {
      const reason = validate(s);
      if (reason) { status = reason; return; }
    }
    const messages = {
      enable: rfUi('Включить защиту odin? Неправильная конфигурация может закрыть доступ. Без подтверждения watchdog откатит изменения через 90 секунд.',"Enable odin protection? Incorrect settings can block access. Without confirmation, the watchdog will roll back changes after 90 seconds."),
      confirm: rfUi('Подтвердить защиту? Автооткат через 90 секунд будет отменён. Убедись, что доступ по стукам проверен.',"Confirm protection? Automatic rollback after 90 seconds will be cancelled. Make sure knock-based access is verified."),
      disable: rfUi('Выключить защиту odin и убрать её правила из firewall?',"Disable odin protection and remove its firewall rules?"),
      save: rfUi('Сохранить настройки odin?',"Save odin settings?")
    };
    if (!window.confirm(messages[action])) return;
    lastOperationError = '';
    busy = true;
    status = rfUi('Выполняется: ',"Running:") + labels[action] + '…';
    try {
      await request('POST', { action, confirm: 'APPLY', settings: s });
      const fresh = await request('GET');
      ++requestSerial;
      applyServerState(fresh, true);
    } catch (error) {
      lastOperationError = rfUi('Ошибка: ',"Error:") + error.message + rfUi('. Обнови состояние перед повторной операцией.',". Refresh the status before trying again.");
      status = lastOperationError;
      // A failed mutation can still have taken effect: reconcile from server.
      await refresh();
    } finally { if (!disposed) busy = false; }
  }

  onMount(() => {
    void refresh();
    const countdownTimer = window.setInterval(() => {
      if (disposed) return;
      now = Date.now();
      if (data?.enabled && !data.confirmed && localDeadline && secondsLeft === 0 && !busy && !deadlineRefreshRequested) {
        deadlineRefreshRequested = true;
        void refresh();
      }
    }, 1000);
    const statusTimer = window.setInterval(() => {
      if (!disposed && !busy && !document.hidden) void refresh();
    }, 10000);
    return () => {
      disposed = true;
      readSession.close();
      ++requestSerial;
      window.clearInterval(countdownTimer);
      window.clearInterval(statusTimer);
    };
  });
</script>

<section class="panel" id="odin-manager">
  <h2>Port Knocking · odin (Keenetic)</h2>
  <p class="help">{rfUi("Алгоритм odin для сервиса, уже проброшенного через KeeneticOS. Автор:","odin protection for a service already forwarded through KeeneticOS. Author:")} <a href="https://forum.keenetic.ru/topic/22137-port-knoking-%D0%B4%D0%BB%D1%8F-%D0%BF%D1%80%D0%BE%D0%B1%D1%80%D0%B0%D1%81%D1%8B%D0%B2%D0%B0%D0%B5%D0%BC%D0%BE%D0%B3%D0%BE-%D0%BF%D0%BE%D1%80%D1%82%D0%B0/" target="_blank" rel="noopener noreferrer">odin · Keenetic Community</a>{rfUi(". Авторские цепочки KNOCKING-FIRST/LAST и списки KK1/KK2/KK3/KKD. Установка пакета не включает защиту.",". Uses dedicated KNOCKING-FIRST/LAST chains and KK1/KK2/KK3/KKD lists. Installing the package does not enable protection.")}</p>
  <div class="form-grid">
    <label>{rfUi("WAN-интерфейс","WAN interface")}<input id="odin-wan" bind:value={wan} autocomplete="off" /></label>
    <label>{rfUi("Внутренний IPv4 сервиса","Service's internal IPv4")}<input id="odin-ip" bind:value={ip} autocomplete="off" /></label>
    <label>{rfUi("TCP-порты (через запятую)","TCP ports (comma-separated)")}<input id="odin-port" bind:value={portText} autocomplete="off" placeholder="3389, 22, 8443" /></label>
    <label>{rfUi("Постоянный доступ IPv4/CIDR","Permanent IPv4/CIDR access")}<input id="odin-trusted" bind:value={trustedText} autocomplete="off" placeholder="192.0.2.10/32, 198.51.100.0/24" /></label>
    <label>{rfUi("Стук 1","Knock 1")}<input id="odin-k1" type="number" min="1" max="65535" bind:value={k1} /></label>
    <label>{rfUi("Стук 2","Knock 2")}<input id="odin-k2" type="number" min="1" max="65535" bind:value={k2} /></label>
    <label>{rfUi("Стук 3","Knock 3")}<input id="odin-k3" type="number" min="1" max="65535" bind:value={k3} /></label>
    <label>{rfUi("Окно между стуками, сек","Time between knocks, sec")}<input id="odin-window" type="number" min="5" max="300" bind:value={windowSeconds} /></label>
    <label>{rfUi("Доступ, сек","Access duration, sec")}<input id="odin-ttl" type="number" min="30" max="86400" bind:value={ttl} /></label>
  </div>
  <div class="refresh-bar">
    <button type="button" data-odin="save" disabled={busy || !data || enabled} onclick={() => perform('save')}>{rfUi("Сохранить настройки","Save settings")}</button>
    <button type="button" data-odin="enable" disabled={busy || !data || enabled || !configured} onclick={() => perform('enable')}>{rfUi("Включить защиту","Enable protection")}</button>
    <button type="button" data-odin="confirm" disabled={busy || !data || !enabled || confirmed} onclick={() => perform('confirm')}>{rfUi("Подтвердить защиту","Confirm protection")}</button>
    <button type="button" data-odin="disable" disabled={busy || !data || !enabled} onclick={() => perform('disable')}>{rfUi("Выключить защиту","Disable protection")}</button>
    <button type="button" data-odin="refresh" disabled={busy} onclick={() => perform('refresh')}>{rfUi("Обновить","Refresh")}</button>
    <span id="odin-status" class={enabled ? (confirmed ? "odin-state odin-state-on" : "odin-state odin-state-pending") : "odin-state"} role="status" aria-live="polite">{status}</span>
    <span id="odin-watchdog-countdown" class="odin-countdown" role="status" aria-live="off">{countdown}</span>
  </div>
  <p class="help">{rfUi("Сначала проверь доступ после трёх стуков, затем подтверди защиту в течение 90 секунд. Подтверждение отключает автоматический откат; после него ответственность за выключение защиты остаётся у оператора. Проброс защищаемого порта настраивается в KeeneticOS, порты стуков пробрасывать не требуется.","Verify access after all three knocks, then confirm protection within 90 seconds. Confirmation disables automatic rollback; disabling protection afterward is your responsibility. Configure forwarding for the protected port in KeeneticOS; knock ports do not require forwarding.")}</p>
</section>
