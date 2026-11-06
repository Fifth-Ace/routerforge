<script>
  import { onMount } from 'svelte';

  // Svelte owns state, rendering, polling and action dispatch. Keep the
  // original DOM classes/IDs and CSS to avoid a design or layout migration.
  const API = '/api/modules/crowdsec-manager/';
  let overview = $state(null);
  let installer = $state(null);
  let overviewError = $state('');
  let installerError = $state('');
  let installerProgress = $state('Ожидаем проверки платформы.');
  let busy = $state(false);
  let refreshing = $state(false);
  let installing = $state(false);
  let removing = $state(false);
  let toast = $state('');
  let toastError = $state(false);
  let lapiUrl = $state('');
  let lapiKey = $state('');
  let installUrl = $state('');
  let installKey = $state('');
  let installKeepConfig = $state(true);
  let waiverConfirmed = $state(false);
  let installedUIReady = $state(false);

  let s = $derived(overview?.snapshot || {});
  let firewall = $derived(s.firewall || {});
  let sets = $derived(s.sets || {});
  let hook = $derived(s.ndm_hook || {});
  let cfg = $derived(overview?.lapi || {});
  let protectedOK = $derived(overview?.state === 'healthy' && !!s.protected);
  let missing = $derived.by(() => {
    if (!overview) return [];
    const reasons = [];
    if (!s.installed) reasons.push('не найден bouncer');
    if (!s.running) reasons.push('bouncer не запущен');
    if (!sets.ok) reasons.push('не найдены CrowdSec ipset');
    if (!firewall.ok) reasons.push('не подтверждён путь iptables → ipset DROP');
    if (!overview.hook_executable) reasons.push('нет исполняемого NDM hook');
    if (!overview.init_installed) reasons.push('нет init-скрипта bouncer');
    if (!cfg.exists) reasons.push('нет конфигурации LAPI');
    if (!cfg.api_key_set) reasons.push('нет ключа LAPI');
    if (!cfg.api_url) reasons.push('не задан адрес LAPI');
    if (cfg.tls_verify === false && !overview.tls_waived) reasons.push('отключена проверка TLS');
    if (cfg.api_url?.startsWith('http://') && !overview.tls_waived) reasons.push('LAPI работает без TLS');
    if (!overview.lapi_probe?.startsWith('LAPI health endpoint reachable')) reasons.push('LAPI health не подтверждён');
    if (cfg.error) reasons.push('ошибка конфигурации LAPI');
    if (!protectedOK && !reasons.length) reasons.push('не подтверждён общий статус; смотри детали');
    return reasons;
  });
  let verdict = $derived(overviewError
    ? 'Нет связи с runtime: ' + overviewError
    : !overview ? 'Получаем фактическое состояние firewall…'
      : protectedOK ? (overview.tls_waived
        ? 'Фильтрация подтверждена. ВНИМАНИЕ: TLS игнорируется по вашему согласию, соединение с LAPI не защищено или сертификат не проверяется.'
        : 'Фильтрация подтверждена по активным цепочкам и ipset DROP; NDM rebuild отдельно не проверялся')
        : 'NOT VERIFIED — ' + missing.join('; ') + '.');
  let metricKeys = $derived(Object.keys(overview?.metrics || {}).sort());

  async function api(path, method = 'GET', data) {
    const r = await fetch(API + path, {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: data ? { 'Content-Type': 'application/json' } : {},
      body: data ? JSON.stringify(data) : undefined
    });
    const d = await r.json();
    if (!r.ok || d.error) throw Error(d.error || 'HTTP ' + r.status);
    return d;
  }
  function message(value, error = false) { toast = value; toastError = error; }

  async function refresh() {
    if (busy || refreshing) return;
    refreshing = true;
    try {
      const d = await api('overview');
      overview = d;
      installedUIReady = true;
      overviewError = '';
      // Preserve active user edits, exactly as the previous module did.
      if (document.activeElement?.id !== 'lapi-url' && !lapiKey && !busy) {
        lapiUrl = d.lapi?.api_url || '';
      }
    } catch (e) {
      overviewError = e.message;
    } finally { refreshing = false; }
  }

  async function action(path, data, question) {
    if (busy || !window.confirm(question)) return;
    busy = true;
    message('Выполняем…');
    try {
      await api(path, 'POST', data);
      if (path === 'lapi') lapiKey = '';
      message('Действие выполнено. Состояние обновлено.');
    } catch (e) { message(e.message, true); }
    finally { busy = false; await refresh(); }
  }
  function changeService(operation) {
    return action('service', { action: operation, confirm: 'crowdsec-service' },
      'Выполнить ' + operation + ' для /opt/etc/init.d/S99csfirewall? Это может изменить активную фильтрацию.');
  }
  function enableWaiver() {
    if (!waiverConfirmed) {
      message('Сначала подтвердите, что принимаете риск соединения без проверки TLS.', true);
      return;
    }
    return action('tls-waiver', { action: 'enable', confirm: 'crowdsec-tls-risk-accepted' },
      'Подтверждаю: понимаю риск незашифрованного или непроверенного соединения с LAPI и хочу не учитывать только TLS при оценке состояния. Продолжить?');
  }
  function revokeWaiver() {
    return action('tls-waiver', { action: 'disable', confirm: 'crowdsec-tls-risk-accepted' },
      'Отменить согласие и снова учитывать TLS при оценке состояния?');
  }
  function submitLapi(event) {
    event.preventDefault();
    return action('lapi', { api_url: lapiUrl.trim(), api_key: lapiKey, confirm: 'crowdsec-lapi' },
      'Сохранить URL/API key в существующий config с резервной копией? После сохранения перезапусти bouncer.');
  }
  async function pollInstaller() {
    try {
      const state = await api('install/status');
      installer = state;
      installerError = '';
      installerProgress = (state.busy ? 'LIVE · ' : state.completed ? 'PASS · ' : '')
        + (state.detail || (state.supported ? 'Готов к установке' : 'Нет проверенной сборки для этого ABI'));
      if (state.error) {
        installerError = state.error;
        installerProgress = 'FAIL · ' + state.error;
      }
      if (state.completed && !state.error) await refresh();
    } catch (e) { installerProgress = 'Статус установки: ' + e.message; }
  }
  async function startInstall(event) {
    event.preventDefault();
    if (busy || installing || !window.confirm('Установить официальный проверенный Firewall Bouncer, зависимости ipset/iptables и создать службу Entware на этом роутере?')) return;
    installing = true;
    installerProgress = 'Отправляем задание установки…';
    const key = installKey;
    installKey = '';
    try { await api('install/start', 'POST', { api_url: installUrl.trim(), api_key: key, confirm: 'crowdsec-install' }); await pollInstaller(); }
    catch (e) { installerError = e.message; installerProgress = 'FAIL · ' + e.message; }
    finally { installing = false; }
  }
  async function removeInstall() {
    if (removing || !window.confirm('Удалить установленный RouterForge Firewall Bouncer? Служба будет остановлена.')) return;
    removing = true;
    installerProgress = 'Останавливаем и удаляем bouncer…';
    try {
      await api('install/remove', 'POST', { confirm: 'crowdsec-remove', keep_config: installKeepConfig ? 'true' : 'false' });
      installerProgress = 'Bouncer удалён';
      await pollInstaller();
      await refresh();
    } catch (e) {
      installerError = e.message;
      installerProgress = 'FAIL · ' + e.message;
      await pollInstaller();
    } finally { removing = false; }
  }
  function synchronizeTheme() {
    const root = document.documentElement;
    const pairs = [
      ['--bg', '--cs-bg'], ['--surface', '--cs-panel'], ['--surface-2', '--cs-panel-soft'],
      ['--border', '--cs-border'], ['--text', '--cs-text'], ['--muted', '--cs-muted'],
      ['--accent', '--cs-accent'], ['--good', '--cs-good'], ['--bad', '--cs-bad'],
      ['--warn', '--cs-warning']
    ];
    try {
      if (!window.parent || window.parent === window) return;
      const parentRoot = window.parent.document.documentElement;
      const computed = window.parent.getComputedStyle(parentRoot);
      for (const [from, to] of pairs) {
        const value = computed.getPropertyValue(from).trim();
        if (value) root.style.setProperty(to, value);
      }
      const realText = window.parent.getComputedStyle(window.parent.document.body).color;
      if (!computed.getPropertyValue('--text').trim() && realText) root.style.setProperty('--cs-text', realText);
    } catch { /* original standalone fallback */ }
  }
  onMount(() => {
    synchronizeTheme();
    window.addEventListener('pageshow', synchronizeTheme);
    void pollInstaller();
    void refresh();
    const installerTimer = window.setInterval(() => { if (!document.hidden) void pollInstaller(); }, 2500);
    const overviewTimer = window.setInterval(() => { if (!document.hidden) void refresh(); }, 15000);
    return () => {
      window.removeEventListener('pageshow', synchronizeTheme);
      window.clearInterval(installerTimer);
      window.clearInterval(overviewTimer);
    };
  });
</script>

<main class="cs">
<header class="head"><div><div class="eyebrow">ROUTERFORGE · SECURITY</div><h1>CrowdSec Manager</h1><p class="muted">Windows CrowdSec LAPI → Keenetic Firewall Bouncer → ipset / iptables</p></div><button id="refresh" onclick={refresh} disabled={refreshing}>↻ Обновить</button></header>
<div class={'banner' + (overviewError ? ' bad' : overview ? (protectedOK ? ' good' : ' bad') : '')} id="verdict" role="status">{verdict}</div>
<section class="panel" id="bouncer-install" hidden={!installedUIReady}>
<h2>CrowdSec Firewall Bouncer · Установка</h2>
<p class="muted">Установка в Entware на этом роутере. CrowdSec LAPI остаётся на Windows.</p>
<p class="small" id="install-platform">Entware target: {installer?.platform || 'unknown'}{installer?.supported ? ' · verified build available' : ' · нет проверенной сборки'}</p>
<form id="install-form" onsubmit={startInstall} hidden={!!installer?.installed}>
<label>Windows LAPI (HTTPS)<input id="install-url" type="url" required placeholder="https://192.168.1.10:8080" autocomplete="off" bind:value={installUrl} disabled={!!installer?.busy}></label>
<label>LAPI Bouncer API key<input id="install-key" type="password" required minlength="8" autocomplete="new-password" bind:value={installKey} disabled={!!installer?.busy}></label>
<div class="actions"><button id="install-start" type="submit" disabled={installing || !!installer?.busy || !!installer?.installed || !installer?.supported}>Установить Firewall Bouncer</button><span class="small" id="install-release">Bouncer {installer?.release || 'unknown'}</span></div>
</form><div class="actions"><button id="install-remove" type="button" class="danger" hidden={!installer?.managed} disabled={!!installer?.busy || removing} onclick={removeInstall}>Удалить Bouncer RouterForge</button><label class="small" id="install-preserve-wrap" hidden={!installer?.managed}><input id="install-keep-config" type="checkbox" bind:checked={installKeepConfig}> Сохранить настройки LAPI</label></div>
<p id="install-progress" class={installerError ? 'fail' : installer?.completed ? 'ok' : 'small'} role="status" aria-live="polite">{installerProgress}</p>
<p class="small">Используется фиксированный официальный релиз с SHA256. Существующие файлы конфигурации не перезаписываются. Пока подтверждена сборка ARM64; MIPS/MIPSEL получают отдельный статус доступности.</p>
<p><a href="https://github.com/siamskoi/keenetic-crowdsec-bouncer" target="_blank" rel="noopener noreferrer">Документация Keenetic / NDM ↗</a></p>
</section>
<section class="stats">
<article><span>Защита трафика</span><strong id="protection" class:ok={!!overview && protectedOK} class:fail={!!overview && !protectedOK}>{overview ? (protectedOK ? (overview.tls_waived ? 'ACTIVE · TLS RISK' : 'ACTIVE') : 'NOT VERIFIED') : '—'}</strong><small id="protection-hint">{overview ? 'Installed: ' + !!s.installed + ' · running: ' + !!s.running + ' · ipset: ' + !!sets.ok : 'Проверяем…'}</small></article>
<article><span>Bouncer</span><strong id="running" class:ok={!!overview && !!s.running} class:fail={!!overview && !s.running}>{overview ? (s.running ? 'Работает' : 'Остановлен') : '—'}</strong><small id="installed">{overview ? (s.installed ? 'Бинарник обнаружен' : 'Бинарник не установлен') : '—'}</small></article>
<article><span>Firewall / FORWARD</span><strong id="firewall" class:ok={!!overview && !!firewall.ok} class:fail={!!overview && !firewall.ok}>{overview ? (firewall.ok ? 'DROP подключён' : 'Не подтверждён') : '—'}</strong><small id="firewall-hint">{firewall.detail || '—'}</small></article>
<article><span>NDM hook</span><strong id="hook" class:ok={!!overview && !!overview.hook_executable} class:fail={!!overview && !overview.hook_executable}>{overview ? (overview.hook_executable ? 'Исполняемый' : 'Не готов') : '—'}</strong><small id="hook-hint">{hook.detail || '—'}</small></article>
</section>
<section class="gridtwo">
<div class="panel"><h2>Служба Firewall Bouncer</h2><p class="muted">Работающий процесс не равен работающей защите. Состояние сверяется с ipset и действующими цепочками DROP.</p><div class="actions"><button data-service="start" onclick={() => changeService('start')}>Запустить</button><button data-service="restart" onclick={() => changeService('restart')}>Перезапустить</button><button data-service="stop" class="danger" onclick={() => changeService('stop')}>Остановить</button></div><p class="small" id="init-status">{overview ? (overview.init_installed ? 'Скрипт /opt/etc/init.d/S99csfirewall готов' : 'Отсутствует исполняемый S99csfirewall') : '—'}</p></div>
<div class="panel"><h2>Windows LAPI</h2><p class="muted">Только адрес и ключ существующего bouncer-конфига. Ключ не читается через API и не отображается после сохранения.</p><form id="lapi-form" onsubmit={submitLapi}><label>Адрес LAPI <input id="lapi-url" required placeholder="https://192.168.1.10:8080" autocomplete="off" bind:value={lapiUrl}></label><label>API key <input id="lapi-key" type="password" placeholder="Не менять сохранённый ключ" autocomplete="new-password" bind:value={lapiKey}></label><div class="actions"><button type="submit">Сохранить настройки</button><span id="key-present" class="small">{overview ? (cfg.api_key_set ? 'Ключ сохранён' : 'Ключ отсутствует') : '—'}</span></div><p class="small" id="tls-state">{overview ? (cfg.api_url?.startsWith('http://') ? 'ВНИМАНИЕ: HTTP без шифрования' : cfg.tls_verify ? 'Проверка TLS сертификата включена' : 'ВНИМАНИЕ: в bouncer конфиге отключена проверка TLS') : 'TLS: —'}</p><p class="small" id="lapi-probe">{overview?.lapi_probe || 'Соединение LAPI: —'}</p></form>
<div id="tls-waiver-panel" class="panel" hidden={!overview?.tls_unsecured}>
<strong>⚠ TLS не защищён</strong>
<p class="small" id="tls-waiver-status">{overview?.tls_waived ? 'Вы согласились игнорировать предупреждение TLS для этого адреса LAPI. Соединение остаётся небезопасным.' : 'Без TLS данные могут передаваться без шифрования или без проверки подлинности сервера.'}</p>
<label id="tls-waiver-confirm-wrap" class="small" hidden={!!overview?.tls_waived}><input id="tls-waiver-confirm" type="checkbox" bind:checked={waiverConfirmed}> Я понимаю риск и сам соглашаюсь игнорировать TLS при оценке статуса.</label>
<div class="actions"><button id="tls-waiver-accept" type="button" hidden={!!overview?.tls_waived} onclick={enableWaiver}>Игнорировать TLS</button><button id="tls-waiver-revoke" type="button" hidden={!overview?.tls_waived} onclick={revokeWaiver}>Отменить игнорирование TLS</button></div>
<p class="small">Это исключение не включает шифрование и не отключает проверку сертификата в Firewall Bouncer. Остальные проверки защиты остаются обязательными.</p>
</div></div>
</section>
<section class="gridtwo">
<div class="panel"><h2>Устойчивость к NDM rebuild</h2><p id="hook-hint-long" class="muted">{overview ? (overview.hook_managed ? 'Установлен управляемый RouterForge hook' : overview.hook_executable ? 'Найден внешний hook. RouterForge его не изменяет.' : 'Hook не установлен или не исполняется: после NDM rebuild защита может пропасть.') : '—'}</p><p class="small">Восстанавливающий hook не запускается вручную: он вызывается Keenetic после перестройки netfilter. Существующий чужой hook никогда не перезаписывается.</p><div class="actions"><button id="hook-install" disabled={!!overview?.hook_executable && !overview?.hook_managed} onclick={() => action('hook', { action: 'install', confirm: 'crowdsec-hook' }, 'Установить NDM hook? Он будет восстанавливать DROP-цепочку при rebuild netfilter Keenetic.')}>Установить управляемый hook</button><button id="hook-remove" class="danger" disabled={!!overview && !overview.hook_managed} onclick={() => action('hook', { action: 'remove', confirm: 'crowdsec-hook' }, 'Удалить только управляемый RouterForge hook? После NDM rebuild защита может отключиться.')}>Удалить свой hook</button></div></div>
<div class="panel"><h2>Реальная цепочка</h2><div class="chain"><span>FORWARD</span><b>→</b><span>CROWDSEC_CHAIN</span><b>→</b><span>ipset DROP</span></div><p id="chain-hint" class="small">{overview ? (s.chains?.join(' · ') || 'Нет подтверждённой связки.') : '—'}</p><pre id="counters" class="terminal">{overview?.drop_counters || '—'}</pre></div>
</section>
<section class="panel"><div class="panel-head"><h2>IP-блокировки в ipset</h2><span id="updated" class="small">{s.updated || '—'}</span></div><div id="sets" class="set-grid">{#if !overview}Проверяем…{:else if overview.set_details?.length}{#each overview.set_details as item (item.name)}<div class="set"><strong>{item.name}</strong><p class="ok">{item.count_known ? item.members + ' IP / CIDR' : 'Число неизвестно'}</p><small>{item.samples?.join(', ') || 'Нет примеров'}</small></div>{/each}{:else}Нет данных{/if}</div><p class="small">Число адресов берётся из реального списка Members, если Keenetic не выдаёт Number of entries. Пустой bouncer gauge сам по себе не считается нулём блокировок.</p></section>
<section class="panel"><h2>Prometheus / диагностика</h2><p id="metrics-state" class="small">{overview ? (overview.metrics_error ? 'Metrics: ' + overview.metrics_error : 'Локальный Prometheus endpoint опрошен') : '—'}</p><div id="metrics" class="metrics">{#if overview && !metricKeys.length}Нет данных{:else}{#each metricKeys as key (key)}<div class="metric"><code>{key}</code><strong>{overview.metrics[key]}</strong></div>{/each}{/if}</div><div id="notices" class="notices">{#if !overview}{:else if s.notes?.length}{#each s.notes as note}<p>{note}</p>{/each}{:else}Дополнительных замечаний нет.{/if}</div></section>
<p id="toast" class:fail={toastError} class:ok={!toastError} aria-live="polite">{toast}</p>
<footer>RouterForge реализует интерфейс управления; Firewall Bouncer — отдельный проект. Благодарность <a href="https://github.com/siamskoi/keenetic-crowdsec-bouncer" target="_blank" rel="noopener noreferrer">siamskoi / keenetic-crowdsec-bouncer</a> и CrowdSec. Действия с брандмауэром требуют проверки на тестовом Keenetic.</footer>
</main>
