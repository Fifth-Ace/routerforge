<script>
  import { rfUi } from './rf-localization.js';
  import { onMount } from 'svelte';
  import { createReadSessions } from './readSessions.js';

  // Svelte owns state, rendering, polling and action dispatch. Keep the
  // original DOM classes/IDs and CSS to avoid a design or layout migration.
  const API = '/api/modules/crowdsec-manager/';
  const reads = createReadSessions();
  let overview = $state(null);
  let installer = $state(null);
  let overviewError = $state('');
  let installerError = $state('');
  let installerProgress = $state(rfUi('Ожидаем проверки платформы.',"Waiting for platform verification."));
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
    if (!s.installed) reasons.push(rfUi('не найден bouncer',"Bouncer not found"));
    if (!s.running) reasons.push(rfUi('bouncer не запущен',"Bouncer is not running"));
    if (!sets.ok) reasons.push(rfUi('не найдены CrowdSec ipset',"CrowdSec ipsets not found"));
    if (!firewall.ok) reasons.push(rfUi('не подтверждён путь iptables → ipset DROP',"iptables → ipset DROP path not verified"));
    if (!overview.hook_executable) reasons.push(rfUi('нет исполняемого NDM hook',"Executable NDM hook missing"));
    if (!overview.init_installed) reasons.push(rfUi('нет init-скрипта bouncer',"Bouncer init script missing"));
    if (!cfg.exists) reasons.push(rfUi('нет конфигурации LAPI',"LAPI configuration missing"));
    if (!cfg.api_key_set) reasons.push(rfUi('нет ключа LAPI',"LAPI key missing"));
    if (!cfg.api_url) reasons.push(rfUi('не задан адрес LAPI',"LAPI address not configured"));
    if (cfg.tls_verify === false && !overview.tls_waived) reasons.push(rfUi('отключена проверка TLS',"TLS verification disabled"));
    if (cfg.api_url?.startsWith('http://') && !overview.tls_waived) reasons.push(rfUi('LAPI работает без TLS',"LAPI is running without TLS"));
    if (!overview.lapi_probe?.startsWith('LAPI health endpoint reachable')) reasons.push(rfUi('LAPI health не подтверждён',"LAPI health not verified"));
    if (cfg.error) reasons.push(rfUi('ошибка конфигурации LAPI',"LAPI configuration error"));
    if (!protectedOK && !reasons.length) reasons.push(rfUi('не подтверждён общий статус; смотри детали',"Overall status unverified; see details"));
    return reasons;
  });
  let verdict = $derived(overviewError
    ? rfUi('Нет связи с runtime: ',"Cannot reach the service:") + overviewError
    : !overview ? rfUi('Получаем фактическое состояние firewall…',"Retrieving actual firewall status…")
      : protectedOK ? (overview.tls_waived
        ? rfUi('Фильтрация подтверждена. ВНИМАНИЕ: TLS игнорируется по вашему согласию, соединение с LAPI не защищено или сертификат не проверяется.',"Filtering verified. WARNING: You agreed to ignore TLS, so the LAPI connection may be unencrypted or its certificate unverified.")
        : rfUi('Фильтрация подтверждена по активным цепочкам и ipset DROP; NDM rebuild отдельно не проверялся',"Filtering verified from active chains and ipset DROP; NDM rebuild not verified separately"))
        : 'NOT VERIFIED — ' + missing.join('; ') + '.');
  let metricKeys = $derived(Object.keys(overview?.metrics || {}).sort());

  async function api(path, method = 'GET', data, signal) {
    const r = await fetch(API + path, {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: data ? { 'Content-Type': 'application/json' } : {},
      body: data ? JSON.stringify(data) : undefined,
      // Never abort a protected mutation after it has been dispatched.
      ...(method === 'GET' && signal ? { signal } : {})
    });
    const d = await r.json();
    if (!r.ok || d.error) throw Error(d.error || 'HTTP ' + r.status);
    return d;
  }
  function message(value, error = false) { toast = value; toastError = error; }

  async function refresh() {
    if (busy || refreshing) return;
    const read = reads.begin('overview');
    refreshing = true;
    try {
      const d = await api('overview', 'GET', undefined, read.signal);
      if (!read.isCurrent()) return;
      overview = d;
      installedUIReady = true;
      overviewError = '';
      // Preserve active user edits, exactly as the previous module did.
      if (document.activeElement?.id !== 'lapi-url' && !lapiKey && !busy) {
        lapiUrl = d.lapi?.api_url || '';
      }
    } catch (e) {
      if (read.isCurrent()) overviewError = e.message;
    } finally {
      if (read.isCurrent()) refreshing = false;
      read.finish();
    }
  }

  async function action(path, data, question) {
    if (busy || !window.confirm(question)) return;
    busy = true;
    message(rfUi('Выполняем…',"Working…"));
    try {
      await api(path, 'POST', data);
      if (path === 'lapi') lapiKey = '';
      message(rfUi('Действие выполнено. Состояние обновлено.',"Action completed. Status refreshed."));
    } catch (e) { message(e.message, true); }
    finally { busy = false; await refresh(); }
  }
  function changeService(operation) {
    return action('service', { action: operation, confirm: 'crowdsec-service' },
      rfUi('Выполнить ',"Run") + operation + rfUi(' для /opt/etc/init.d/S99csfirewall? Это может изменить активную фильтрацию.',"for /opt/etc/init.d/S99csfirewall? This may change active filtering."));
  }
  function enableWaiver() {
    if (!waiverConfirmed) {
      message(rfUi('Сначала подтвердите, что принимаете риск соединения без проверки TLS.',"First confirm that you accept the risk of a connection without TLS verification."), true);
      return;
    }
    return action('tls-waiver', { action: 'enable', confirm: 'crowdsec-tls-risk-accepted' },
      rfUi('Подтверждаю: понимаю риск незашифрованного или непроверенного соединения с LAPI и хочу не учитывать только TLS при оценке состояния. Продолжить?',"I understand the risk of an unencrypted or unverified LAPI connection and want to disregard only TLS when assessing status. Continue?"));
  }
  function revokeWaiver() {
    return action('tls-waiver', { action: 'disable', confirm: 'crowdsec-tls-risk-accepted' },
      rfUi('Отменить согласие и снова учитывать TLS при оценке состояния?',"Revoke consent and restore TLS safety checks?"));
  }
  function submitLapi(event) {
    event.preventDefault();
    return action('lapi', { api_url: lapiUrl.trim(), api_key: lapiKey, confirm: 'crowdsec-lapi' },
      rfUi('Сохранить URL/API key в существующий config с резервной копией? После сохранения перезапусти bouncer.',"Save the URL/API key to the existing configuration with a backup? Restart the bouncer after saving."));
  }
  async function pollInstaller() {
    const read = reads.begin('installer');
    try {
      const state = await api('install/status', 'GET', undefined, read.signal);
      if (!read.isCurrent()) return;
      installer = state;
      installerError = '';
      installerProgress = (state.busy ? 'LIVE · ' : state.completed ? 'PASS · ' : '')
        + (state.detail || (state.supported ? rfUi('Готов к установке',"Ready to install") : rfUi('Нет проверенной сборки для этого ABI',"No verified build for this architecture")));
      if (state.error) {
        installerError = state.error;
        installerProgress = 'FAIL · ' + state.error;
      }
      if (state.completed && !state.error) await refresh();
    } catch (e) {
      if (read.isCurrent()) installerProgress = 'Статус установки: ' + e.message;
    } finally {
      read.finish();
    }
  }
  async function startInstall(event) {
    event.preventDefault();
    if (busy || installing || !window.confirm(rfUi('Установить официальный проверенный Firewall Bouncer, зависимости ipset/iptables и создать службу Entware на этом роутере?',"Install the verified official Firewall Bouncer, ipset/iptables dependencies, and Entware service on this router?"))) return;
    installing = true;
    installerProgress = rfUi('Отправляем задание установки…',"Submitting installation task…");
    const key = installKey;
    installKey = '';
    try { await api('install/start', 'POST', { api_url: installUrl.trim(), api_key: key, confirm: 'crowdsec-install' }); await pollInstaller(); }
    catch (e) { installerError = e.message; installerProgress = 'FAIL · ' + e.message; }
    finally { installing = false; }
  }
  async function removeInstall() {
    if (removing || !window.confirm(rfUi('Удалить установленный RouterForge Firewall Bouncer? Служба будет остановлена.',"Remove the installed RouterForge Firewall Bouncer? The service will be stopped."))) return;
    removing = true;
    installerProgress = rfUi('Останавливаем и удаляем bouncer…',"Stopping and removing the bouncer…");
    try {
      await api('install/remove', 'POST', { confirm: 'crowdsec-remove', keep_config: installKeepConfig ? 'true' : 'false' });
      installerProgress = rfUi('Bouncer удалён',"Bouncer removed");
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
      reads.dispose();
    };
  });
</script>

<main class="cs">
<header class="head"><div><div class="eyebrow">ROUTERFORGE · SECURITY</div><h1>CrowdSec Manager</h1><p class="muted">Windows CrowdSec LAPI → Keenetic Firewall Bouncer → ipset / iptables</p></div><button id="refresh" onclick={refresh} disabled={refreshing}>{rfUi("↻ Обновить","↻ Refresh")}</button></header>
<div class={'banner' + (overviewError ? ' bad' : overview ? (protectedOK ? ' good' : ' bad') : '')} id="verdict" role="status">{verdict}</div>
<section class="panel" id="bouncer-install" hidden={!installedUIReady}>
<h2>{rfUi("CrowdSec Firewall Bouncer · Установка","CrowdSec Firewall Bouncer · Installation")}</h2>
<p class="muted">{rfUi("Установка в Entware на этом роутере. CrowdSec LAPI остаётся на Windows.","Installed in Entware on this router. CrowdSec LAPI remains on Windows.")}</p>
<p class="small" id="install-platform">Entware target: {installer?.platform || 'unknown'}{installer?.supported ? ' · verified build available' : ' · нет проверенной сборки'}</p>
<form id="install-form" onsubmit={startInstall} hidden={!!installer?.installed}>
<label>Windows LAPI (HTTPS)<input id="install-url" type="url" required placeholder="https://192.168.1.10:8080" autocomplete="off" bind:value={installUrl} disabled={!!installer?.busy}></label>
<label>LAPI Bouncer API key<input id="install-key" type="password" required minlength="8" autocomplete="new-password" bind:value={installKey} disabled={!!installer?.busy}></label>
<div class="actions"><button id="install-start" type="submit" disabled={installing || !!installer?.busy || !!installer?.installed || !installer?.supported}>{rfUi("Установить Firewall Bouncer","Install Firewall Bouncer")}</button><span class="small" id="install-release">Bouncer {installer?.release || 'unknown'}</span></div>
</form><div class="actions"><button id="install-remove" type="button" class="danger" hidden={!installer?.managed} disabled={!!installer?.busy || removing} onclick={removeInstall}>{rfUi("Удалить Bouncer RouterForge","Remove RouterForge Bouncer")}</button><label class="small" id="install-preserve-wrap" hidden={!installer?.managed}><input id="install-keep-config" type="checkbox" bind:checked={installKeepConfig}> {rfUi("Сохранить настройки LAPI","Save LAPI settings")}</label></div>
<p id="install-progress" class={installerError ? 'fail' : installer?.completed ? 'ok' : 'small'} role="status" aria-live="polite">{installerProgress}</p>
<p class="small">{rfUi("Используется фиксированный официальный релиз с SHA256. Существующие файлы конфигурации не перезаписываются. Пока подтверждена сборка ARM64; MIPS/MIPSEL получают отдельный статус доступности.","An official release is pinned by SHA256. Existing configuration files are not overwritten. ARM64 support is confirmed; MIPS/MIPSEL availability is reported separately.")}</p>
<p><a href="https://github.com/siamskoi/keenetic-crowdsec-bouncer" target="_blank" rel="noopener noreferrer">{rfUi("Документация Keenetic / NDM ↗","Keenetic / NDM documentation ↗")}</a></p>
</section>
<section class="stats">
<article><span>{rfUi("Защита трафика","Traffic protection")}</span><strong id="protection" class:ok={!!overview && protectedOK} class:fail={!!overview && !protectedOK}>{overview ? (protectedOK ? (overview.tls_waived ? 'ACTIVE · TLS RISK' : 'ACTIVE') : 'NOT VERIFIED') : '—'}</strong><small id="protection-hint">{overview ? 'Installed: ' + !!s.installed + ' · running: ' + !!s.running + ' · ipset: ' + !!sets.ok : 'Проверяем…'}</small></article>
<article><span>Bouncer</span><strong id="running" class:ok={!!overview && !!s.running} class:fail={!!overview && !s.running}>{overview ? (s.running ? 'Работает' : 'Остановлен') : '—'}</strong><small id="installed">{overview ? (s.installed ? 'Бинарник обнаружен' : 'Бинарник не установлен') : '—'}</small></article>
<article><span>Firewall / FORWARD</span><strong id="firewall" class:ok={!!overview && !!firewall.ok} class:fail={!!overview && !firewall.ok}>{overview ? (firewall.ok ? 'DROP подключён' : 'Не подтверждён') : '—'}</strong><small id="firewall-hint">{firewall.detail || '—'}</small></article>
<article><span>NDM hook</span><strong id="hook" class:ok={!!overview && !!overview.hook_executable} class:fail={!!overview && !overview.hook_executable}>{overview ? (overview.hook_executable ? 'Исполняемый' : 'Не готов') : '—'}</strong><small id="hook-hint">{hook.detail || '—'}</small></article>
</section>
<section class="gridtwo">
<div class="panel"><h2>{rfUi("Служба Firewall Bouncer","Firewall Bouncer service")}</h2><p class="muted">{rfUi("Работающий процесс не равен работающей защите. Состояние сверяется с ipset и действующими цепочками DROP.","A running process does not guarantee active protection. Status is checked against ipset and active DROP chains.")}</p><div class="actions"><button data-service="start" onclick={() => changeService('start')}>{rfUi("Запустить","Start")}</button><button data-service="restart" onclick={() => changeService('restart')}>{rfUi("Перезапустить","Restart")}</button><button data-service="stop" class="danger" onclick={() => changeService('stop')}>{rfUi("Остановить","Stop")}</button></div><p class="small" id="init-status">{overview ? (overview.init_installed ? 'Скрипт /opt/etc/init.d/S99csfirewall готов' : 'Отсутствует исполняемый S99csfirewall') : '—'}</p></div>
<div class="panel"><h2>Windows LAPI</h2><p class="muted">{rfUi("Только адрес и ключ существующего bouncer-конфига. Ключ не читается через API и не отображается после сохранения.","Only the address and key of the existing bouncer configuration are changed. The key is not exposed through the API or displayed after saving.")}</p><form id="lapi-form" onsubmit={submitLapi}><label>{rfUi("Адрес LAPI","LAPI address")} <input id="lapi-url" required placeholder="https://192.168.1.10:8080" autocomplete="off" bind:value={lapiUrl}></label><label>API key <input id="lapi-key" type="password" placeholder={rfUi("Не менять сохранённый ключ","Keep existing saved key")} autocomplete="new-password" bind:value={lapiKey}></label><div class="actions"><button type="submit">{rfUi("Сохранить настройки","Save settings")}</button><span id="key-present" class="small">{overview ? (cfg.api_key_set ? 'Ключ сохранён' : 'Ключ отсутствует') : '—'}</span></div><p class="small" id="tls-state">{overview ? (cfg.api_url?.startsWith('http://') ? 'ВНИМАНИЕ: HTTP без шифрования' : cfg.tls_verify ? 'Проверка TLS сертификата включена' : 'ВНИМАНИЕ: в bouncer конфиге отключена проверка TLS') : 'TLS: —'}</p><p class="small" id="lapi-probe">{overview?.lapi_probe || 'Соединение LAPI: —'}</p></form>
<div id="tls-waiver-panel" class="panel" hidden={!overview?.tls_unsecured}>
<strong>{rfUi("⚠ TLS не защищён","⚠ TLS is unprotected")}</strong>
<p class="small" id="tls-waiver-status">{overview?.tls_waived ? 'Вы согласились игнорировать предупреждение TLS для этого адреса LAPI. Соединение остаётся небезопасным.' : 'Без TLS данные могут передаваться без шифрования или без проверки подлинности сервера.'}</p>
<label id="tls-waiver-confirm-wrap" class="small" hidden={!!overview?.tls_waived}><input id="tls-waiver-confirm" type="checkbox" bind:checked={waiverConfirmed}> {rfUi("Я понимаю риск и сам соглашаюсь игнорировать TLS при оценке статуса.","I understand the risk and explicitly agree to ignore TLS when checking status.")}</label>
<div class="actions"><button id="tls-waiver-accept" type="button" hidden={!!overview?.tls_waived} onclick={enableWaiver}>{rfUi("Игнорировать TLS","Ignore TLS")}</button><button id="tls-waiver-revoke" type="button" hidden={!overview?.tls_waived} onclick={revokeWaiver}>{rfUi("Отменить игнорирование TLS","Stop ignoring TLS")}</button></div>
<p class="small">{rfUi("Это исключение не включает шифрование и не отключает проверку сертификата в Firewall Bouncer. Остальные проверки защиты остаются обязательными.","This exception does not enable encryption or disable certificate verification in Firewall Bouncer. All other protection checks still apply.")}</p>
</div></div>
</section>
<section class="gridtwo">
<div class="panel"><h2>{rfUi("Устойчивость к NDM rebuild","Recovery after NDM rebuild")}</h2><p id="hook-hint-long" class="muted">{overview ? (overview.hook_managed ? 'Установлен управляемый RouterForge hook' : overview.hook_executable ? 'Найден внешний hook. RouterForge его не изменяет.' : 'Hook не установлен или не исполняется: после NDM rebuild защита может пропасть.') : '—'}</p><p class="small">{rfUi("Восстанавливающий hook не запускается вручную: он вызывается Keenetic после перестройки netfilter. Существующий чужой hook никогда не перезаписывается.","The recovery hook is not run manually: Keenetic invokes it after rebuilding netfilter. Hooks belonging to other components are never overwritten.")}</p><div class="actions"><button id="hook-install" disabled={!!overview?.hook_executable && !overview?.hook_managed} onclick={() => action('hook', { action: 'install', confirm: 'crowdsec-hook' }, 'Установить NDM hook? Он будет восстанавливать DROP-цепочку при rebuild netfilter Keenetic.')}>{rfUi("Установить управляемый hook","Install managed hook")}</button><button id="hook-remove" class="danger" disabled={!!overview && !overview.hook_managed} onclick={() => action('hook', { action: 'remove', confirm: 'crowdsec-hook' }, 'Удалить только управляемый RouterForge hook? После NDM rebuild защита может отключиться.')}>{rfUi("Удалить свой hook","Remove managed hook")}</button></div></div>
<div class="panel"><h2>{rfUi("Реальная цепочка","Active chain")}</h2><div class="chain"><span>FORWARD</span><b>→</b><span>CROWDSEC_CHAIN</span><b>→</b><span>ipset DROP</span></div><p id="chain-hint" class="small">{overview ? (s.chains?.join(' · ') || 'Нет подтверждённой связки.') : '—'}</p><pre id="counters" class="terminal">{overview?.drop_counters || '—'}</pre></div>
</section>
<section class="panel"><div class="panel-head"><h2>{rfUi("IP-блокировки в ipset","IP blocks in ipset")}</h2><span id="updated" class="small">{s.updated || '—'}</span></div><div id="sets" class="set-grid">{#if !overview}{rfUi("Проверяем…","Checking…")}{:else if overview.set_details?.length}{#each overview.set_details as item (item.name)}<div class="set"><strong>{item.name}</strong><p class="ok">{item.count_known ? item.members + ' IP / CIDR' : 'Число неизвестно'}</p><small>{item.samples?.join(', ') || 'Нет примеров'}</small></div>{/each}{:else}{rfUi("Нет данных","No data")}{/if}</div><p class="small">{rfUi("Число адресов берётся из реального списка Members, если Keenetic не выдаёт Number of entries. Пустой bouncer gauge сам по себе не считается нулём блокировок.","The address count is read from the actual member list when Keenetic does not report an entry count. An empty bouncer gauge does not imply zero blocks.")}</p></section>
<section class="panel"><h2>{rfUi("Prometheus / диагностика","Prometheus / diagnostics")}</h2><p id="metrics-state" class="small">{overview ? (overview.metrics_error ? 'Metrics: ' + overview.metrics_error : 'Локальный Prometheus endpoint опрошен') : '—'}</p><div id="metrics" class="metrics">{#if overview && !metricKeys.length}{rfUi("Нет данных","No data")}{:else}{#each metricKeys as key (key)}<div class="metric"><code>{key}</code><strong>{overview.metrics[key]}</strong></div>{/each}{/if}</div><div id="notices" class="notices">{#if !overview}{:else if s.notes?.length}{#each s.notes as note}<p>{note}</p>{/each}{:else}{rfUi("Дополнительных замечаний нет.","No additional findings.")}{/if}</div></section>
<p id="toast" class:fail={toastError} class:ok={!toastError} aria-live="polite">{toast}</p>
<footer>{rfUi("RouterForge реализует интерфейс управления; Firewall Bouncer — отдельный проект. Благодарность","RouterForge provides the management interface; Firewall Bouncer is an independent project. Thanks to")} <a href="https://github.com/siamskoi/keenetic-crowdsec-bouncer" target="_blank" rel="noopener noreferrer">siamskoi / keenetic-crowdsec-bouncer</a> {rfUi("и CrowdSec. Действия с брандмауэром требуют проверки на тестовом Keenetic.","and CrowdSec. Firewall operations require verification on a test Keenetic.")}</footer>
</main>
