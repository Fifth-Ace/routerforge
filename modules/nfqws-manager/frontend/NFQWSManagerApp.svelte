<script>
  import {onMount} from 'svelte';
  import {startNFQWSRuntime} from './NfqwsRuntime.js';
  // Svelte owns DOM creation; the extracted legacy controller retains CAS/rollback logic
  // during migration of its 491 imperative functions. Do not declare ownership complete.
  let activeTab = $state('overview');
  const primaryGroups = {
    configuration: new Set(['strategies','configs','config','lists','geo-assets']),
    strategies: new Set(['strategy-library-v2','strategy-registry-v1','strategy-import']),
    menu: new Set(['catalog','upstream-lists','blobs']),
    tools: new Set(['preflight','dpi-detector','domain-v2','prodlab-v2','clienthello']),
    selection: new Set(['auto-setup-v1','selector-v2','sandbox-v2','target-memory-v2']),
    maintenance: new Set(['runs','logs','backups','runtime-health-v2','scripts','keenetic'])
  };
  let activePrimary = $derived(
    Object.keys(primaryGroups).find(group => primaryGroups[group].has(activeTab)) || 'overview'
  );
  onMount(() => {
    const onTab = (event) => {if(event.detail?.tab) activeTab = event.detail.tab;};
    document.addEventListener('routerforge:nfqws:tab', onTab);
    startNFQWSRuntime();
    return () => document.removeEventListener('routerforge:nfqws:tab', onTab);
  });
</script>

<div class="shell">
<header class="hero">
  <div class="hero-left">
    <div class="logo">NFQ</div>
    <div>
      <div class="title-line"><h1>NFQWS</h1><span id="state" class="status">...</span><span id="targetChip" class="chip">target</span></div>
      <div id="subtitle" class="subtitle">RouterForge integration · loading…</div>
    </div>
  </div>
  <div class="actions">
    <button id="start">Запустить</button><button id="stop">Остановить</button><button id="reload">Перечитать настройки</button><button id="restart">Перезапустить</button><button id="refresh">Обновить</button>
  </div>
</header>

<div id="notice" class="notice">Загрузка состояния NFQWS…</div>

<nav class="tabs nfqws-primary-tabs" id="nfqwsPrimaryTabs" aria-label="Основная навигация NFQWS">
  <button class="tab" class:active={activeTab === 'overview'} data-tab="overview">Обзор</button>
  <button class="tab" class:active={activePrimary === 'configuration'} data-primary-tab="configuration">Настройка</button>
  <button class="tab" class:active={activePrimary === 'strategies'} data-primary-tab="strategies">Стратегии</button>
  <button class="tab" class:active={activePrimary === 'menu'} data-primary-tab="menu">NFQWS Menu</button>
  <button class="tab" class:active={activePrimary === 'tools'} data-primary-tab="tools">Диагностика</button>
  <button class="tab" class:active={activePrimary === 'selection'} data-primary-tab="selection">Подбор</button>
  <button class="tab" class:active={activePrimary === 'maintenance'} data-primary-tab="maintenance">Обслуживание</button>
</nav>

<nav class="tabs nfqws-tool-tabs nfqws-config-tabs" id="nfqwsConfigTabs" aria-label="Настройка NFQWS">
  <button class="tab" class:active={activeTab === 'strategies'} data-tab="strategies">Конфигурация <span id="strategyCount" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'configs'} data-tab="configs">Сохранённые конфиги <span id="configLibraryCount" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'config'} data-tab="config">Полный конфиг</button>
  <button class="tab" class:active={activeTab === 'lists'} data-tab="lists">Списки <span id="listCount" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'geo-assets'} data-tab="geo-assets">Геоданные <span class="nav-meta">В разработке</span></button>
</nav>

<nav class="tabs nfqws-tool-tabs nfqws-strategy-tabs" id="nfqwsStrategyTabs" aria-label="Стратегии RouterForge">
  <button class="tab" class:active={activeTab === 'strategy-library-v2'} data-tab="strategy-library-v2">Свои стратегии</button>
  <button class="tab" class:active={activeTab === 'strategy-registry-v1'} data-tab="strategy-registry-v1">База стратегий <span id="v2RegistryTabCount" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'strategy-import'} data-tab="strategy-import">Импорт стратегии</button>
</nav>

<nav class="tabs nfqws-tool-tabs" id="nfqwsMenuTabs" aria-label="NFQWS Menu">
  <button class="tab" class:active={activeTab === 'catalog'} data-tab="catalog">Каталог <span id="catalogTabState" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'upstream-lists'} data-tab="upstream-lists">Списки <span id="listSourceStateNav" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'blobs'} data-tab="blobs">Blobs <span id="blobTabState" class="nav-meta"></span></button>
</nav>

<nav class="tabs nfqws-tool-tabs" id="nfqwsToolTabs" aria-label="Диагностика NFQWS">
  <button class="tab" class:active={activeTab === 'preflight'} data-tab="preflight">Проверка системы <span id="preflightTabState" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'dpi-detector'} data-tab="dpi-detector">DPI Detector <span id="dpiDetectorTabState" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'domain-v2'} data-tab="domain-v2">Проверка домена</button>
  <button class="tab" class:active={activeTab === 'prodlab-v2'} data-tab="prodlab-v2">Тестирование</button>
  <button class="tab" class:active={activeTab === 'clienthello'} data-tab="clienthello">ClientHello</button>
</nav>

<nav class="tabs nfqws-tool-tabs" id="nfqwsSelectionTabs" aria-label="Подбор стратегий">
  <button class="tab" class:active={activeTab === 'auto-setup-v1'} data-tab="auto-setup-v1">Автонастройка</button>
  <button class="tab" class:active={activeTab === 'selector-v2'} data-tab="selector-v2">Автоподбор</button>
  <button class="tab" class:active={activeTab === 'sandbox-v2'} data-tab="sandbox-v2">Песочница</button>
  <button class="tab" class:active={activeTab === 'target-memory-v2'} data-tab="target-memory-v2">История подборов <span id="v2MemoryTabCount" class="nav-meta"></span></button>
</nav>

<nav class="tabs nfqws-tool-tabs nfqws-maintenance-tabs" id="nfqwsMaintenanceTabs" aria-label="Обслуживание NFQWS">
  <button class="tab" class:active={activeTab === 'runs'} data-tab="runs">История <span id="runHistoryTabState" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'logs'} data-tab="logs">Логи</button>
  <button class="tab" class:active={activeTab === 'backups'} data-tab="backups">Резервные копии <span id="backupCountTab" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'runtime-health-v2'} data-tab="runtime-health-v2">Состояние NFQWS <span id="runtimeHealthTabState" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'scripts'} data-tab="scripts">Скрипты <span id="scriptCount" class="nav-meta"></span></button>
  <button class="tab" class:active={activeTab === 'keenetic'} data-tab="keenetic">Система <span id="keeneticTabState" class="nav-meta"></span></button>
</nav>

<div id="nfqwsMenuCredit" class="strategy-import-credit nfqws-menu-credit">
  <div><strong>NFQWS Menu</strong><span>Интегрированный источник готовых стратегий, списков и blobs.</span></div>
  <a href="https://github.com/rndnaame/nfqws-menu/" target="_blank" rel="noopener noreferrer">Открыть GitHub ↗</a>

</div>

<main>

<section id="view-dpi-detector" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div>
        <h2>DPI Detector</h2>
        <div class="section-kicker">Официальный DPI Detector · консольный и графический режимы</div>
      </div>
      <span id="dpiModuleState" class="config-badge">IDLE</span>
    </div>

    <div class="dpi-top-grid">
      <article class="panel dpi-mode-card">
        <div class="dpi-mode-head">
          <div><h2>Режим</h2><span>один официальный binary · два представления</span></div>
          <button id="dpiModuleRefresh">Обновить</button>
        </div>
        <div class="dpi-mode-switch">
          <button id="dpiModeConsole" class="primary"><strong>Консоль</strong><span>оригинальное меню DPI Detector</span></button>
          <button id="dpiModeWeb"><strong>Графический режим</strong><span>результаты в интерфейсе RouterForge</span></button>
        </div>
        <div id="dpiModuleSummary" class="dpi-runtime-strip"><span class="dpi-runtime-pill">Загрузка состояния…</span></div>
      </article>

      <div class="strategy-import-credit dpi-upstream-credit">
        <div><strong>DPI Detector · Runnin4ik/dpi-detector</strong><span>Официальный компонент из App Center. RouterForge использует установленный /opt/bin/dpi-detector.</span></div>
        <a href="https://github.com/Runnin4ik/dpi-detector" target="_blank" rel="noopener noreferrer">Открыть GitHub ↗</a>
      </div>
    </div>

    <article id="dpiConsolePane" class="panel">
      <div class="panel-head">
        <div><h2>Консоль DPI Detector</h2><div class="section-kicker">оригинальное меню диагностики</div></div>
        <div class="toolbar"><button id="dpiConsoleFullscreen">На весь экран</button><button id="dpiConsoleRestart">Перезапустить DPI Detector</button></div>
      </div>
      <iframe id="dpiConsoleFrame" title="DPI Detector native console" style="display:block;width:100%;height:680px;border:1px solid var(--border);border-radius:8px;background:var(--bg)"></iframe>
      <div class="v2-boundary">Это оригинальное консольное меню DPI Detector. RouterForge только отображает его в веб-интерфейсе.</div>
    </article>

    <div id="dpiWebPane" style="display:none">
      <div class="v2-two">
        <article class="panel">
          <div class="panel-head">
            <div><h2>Web · тесты</h2><div class="section-kicker">выберите любые machine-readable tests 0–6</div></div>
            <span id="dpiWebState" class="config-badge">IDLE</span>
          </div>
          <div class="dpi-test-grid">
            <label class="dpi-test-card"><input id="dpiTest0" type="checkbox" checked><span><strong>0 · Network</strong><small>IP, ASN, DNS, gateway, tunnels, bypass tools</small></span></label>
            <label class="dpi-test-card"><input id="dpiTest1" type="checkbox" checked><span><strong>1 · DNS</strong><small>DoH / DoT / UDP availability and substitutions</small></span></label>
            <label class="dpi-test-card"><input id="dpiTest2" type="checkbox" checked><span><strong>2 · Domains</strong><small>DNS → HTTP → TLS 1.2 / 1.3</small></span></label>
            <label class="dpi-test-card"><input id="dpiTest3" type="checkbox"><span><strong>3 · TCP16</strong><small>16–20 KB DPI behavior matrix</small></span></label>
            <label class="dpi-test-card"><input id="dpiTest4" type="checkbox"><span><strong>4 · Whitelist SNI</strong><small>white-SNI / AS detection</small></span></label>
            <label class="dpi-test-card"><input id="dpiTest5" type="checkbox"><span><strong>5 · Telegram</strong><small>DC reachability + download / upload</small></span></label>
            <label class="dpi-test-card"><input id="dpiTest6" type="checkbox"><span><strong>6 · Fingerprint Burst</strong><small>browser shapes, TLS, ALPN, overlapping handshakes</small></span></label>
            <button id="dpiWebLegend" class="dpi-test-legend" type="button"><strong>7 · Обозначения</strong><span>расшифровка результатов</span></button>
          </div>
        </article>

        <article class="panel">
          <div class="panel-head"><div><h2>Общие параметры</h2><div class="section-kicker">основные параметры проверки</div></div></div>
          <div class="v2-two">
            <label class="v2-label">Профиль
              <select id="dpiWebProfile" class="search"><option value="ru" selected>RU</option><option value="global">Global</option><option value="ir">IR</option><option value="cn">CN</option></select>
            </label>
            <label class="v2-label">Язык результата
              <select id="dpiWebLanguage" class="search"><option value="ru" selected>Русский</option><option value="en">English</option><option value="zh">中文</option><option value="fa">فارسی</option></select>
            </label>
            <label class="v2-label">Параллельно
              <select id="dpiWebConcurrency" class="search"><option value="1">1</option><option value="5">5</option><option value="20" selected>20</option><option value="50">50</option><option value="100">100</option></select>
            </label>
            <label class="v2-label">Интерфейс
              <input id="dpiWebInterface" class="search" placeholder="routing table / например eth2.1">
            </label>
          </div>
          <label class="v2-label">SOCKS/HTTP proxy
            <input id="dpiWebProxy" class="search" placeholder="не использовать">
          </label>
          <label class="v2-label">TLS fingerprint
            <select id="dpiWebFingerprint" class="search">
              <option value="" selected>По умолчанию</option>
              <option>rustls</option><option>chrome146</option><option>chrome131</option><option>chrome131_android</option><option>chrome123</option><option>chrome116</option><option>chrome107</option><option>chrome99_android</option><option>edge101</option><option>firefox147</option><option>firefox133</option><option>safari260</option><option>safari260_ios</option><option>safari184_ios</option><option>safari180</option><option>safari172_ios</option><option>safari170</option><option>safari155</option><option>safari153</option><option>tor145</option>
            </select>
          </label>
        </article>
      </div>

      <div class="v2-two" style="margin-top:12px">
        <article class="panel">
          <div class="panel-head"><div><h2>Цели</h2><div class="section-kicker">-d / --domains / --tcp16</div></div></div>
          <label class="v2-label">Домены · по одному на строку
            <textarea id="dpiWebDomains" class="v2-textarea compact" placeholder="example.com&#10;openai.com"></textarea>
          </label>
          <label class="v2-label">Файл доменов
            <input id="dpiWebDomainsFile" class="search mono" placeholder="/opt/.../domains.txt или /tmp/...">
          </label>
          <label class="v2-label">TCP16 targets
            <input id="dpiWebTCP16File" class="search mono" placeholder="/opt/.../tcp16.json или /tmp/...">
          </label>
        </article>

        <article class="panel">
          <div class="panel-head"><div><h2>Fingerprint / Burst</h2><div class="section-kicker">используется test 6</div></div></div>
          <div class="v2-two">
            <label class="v2-label">Попытки<input id="dpiWebBurst" class="search" type="number" min="1" max="100" value="5"></label>
            <label class="v2-label">Timeout, sec<input id="dpiWebBurstTimeout" class="search" type="number" min="1" max="120" value="8"></label>
            <label class="v2-label">TLS<select id="dpiWebBurstTLS" class="search"><option value="1.3+1.2" selected>1.3 + 1.2</option><option value="1.3">1.3</option><option value="1.2">1.2</option></select></label>
            <label class="v2-label">ALPN<select id="dpiWebBurstALPN" class="search"><option value="h2" selected>h2</option><option value="http/1.1">http/1.1</option></select></label>
          </div>
          <label class="v2-label">Burst profiles
            <input id="dpiWebBurstProfiles" class="search mono" placeholder="пусто = по умолчанию · all · rustls,chrome146,...">
          </label>
          <label class="v2-check"><input id="dpiWebTrace" type="checkbox"> Сохранить подробный журнал</label>
        </article>
      </div>

      <article class="panel dpi-run-panel" style="margin-top:12px">
        <div class="panel-head">
          <div><h2>Запуск</h2><div class="section-kicker">Native Live показывает вывод DPI Detector по мере выполнения. Visual-блоки временно скрыты до следующего UX-прохода.</div></div>
          <div class="toolbar"><button id="dpiWebRun" class="primary">Запустить DPI Detector</button><button id="dpiNativeRun" style="display:none" aria-hidden="true">Native</button></div>
        </div>
        <div class="v2-boundary">nfqws2, VPN и proxy автоматически не отключаются. Результат описывает текущий router path и не объявляется raw ISP truth.</div>
        <div id="dpiRunProgress" class="dpi-run-progress" style="display:none"></div>
      </article>

      <div id="dpiWebResults" style="display:none"></div>
      <div id="dpiNativeResults" style="display:block;margin-top:12px"></div>

      <article id="dpiLegendPane" class="panel" style="display:none;margin-top:12px">
        <div class="panel-head"><div><h2>Обозначения результатов</h2><div class="section-kicker">справка DPI Detector</div></div><button id="dpiLegendClose">Закрыть</button></div>
        <pre id="dpiLegendOutput" class="v2-args">—</pre>
      </article>
    </div>
  </div>
</section>

<section id="view-observed-v1" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Проблемные адреса</h2><div class="section-kicker">подозрительные сетевые сбои, найденные RouterForge</div></div>
      <span id="v2ObservedState" class="config-badge">READ ONLY</span>
    </div>
    <article class="panel">
      <div class="panel-head">
        <div><h2>Найденные проблемы в локальной сети</h2><div class="section-kicker">показываются только после повторного независимого появления failure signal</div></div>
        <div class="toolbar"><button id="v2ObservedLoad">Обновить</button><button id="v2ObservedScan" class="primary">Проверить сетевые соединения</button></div>
      </div>
      <div id="v2ObservedMeta" class="v2-boundary">Проверка ничего не меняет в рабочей конфигурации NFQWS.</div>
      <div id="v2ObservedList" class="v2-list"><div class="analysis-empty">Загрузка…</div></div>
    </article>
    <div class="v2-boundary">Здесь показываются только повторяющиеся подозрительные сбои. Это не означает автоматически, что адрес заблокирован.</div>
  </div>
</section>

<section id="view-domain-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Проверить сайт</h2><div class="section-kicker">проверка доступа → понятная причина → следующее действие</div></div>
      <span id="v2DomainState" class="config-badge">IDLE</span>
    </div>
    <div class="v2-input-row">
      <input id="v2DomainTarget" class="search" placeholder="youtube.com" autocomplete="off" spellcheck="false">
      <button id="v2DomainRun" class="primary">Проверить</button>
      <button id="v2DomainToSelector" style="display:none">Подобрать рабочую стратегию</button>
    </div>
    <article class="panel" id="v2DomainGuidance" role="status" aria-live="polite" style="margin-top:12px">
      <div class="analysis-empty">Введите сайт и нажмите «Проверить». RouterForge подскажет, что делать дальше.</div>
    </article>
    <article id="v2DomainContext" class="panel" role="status" aria-live="polite" style="margin-top:12px">
      <div class="analysis-empty">Состояние правил nfqws2, списков и возможные ограничения проверки появятся после диагностики.</div>
    </article>
    <div id="v2DomainSummary" class="v2-card-grid">
      <div class="panel"><div class="analysis-empty">Введите домен. Рабочие настройки не изменяются.</div></div>
    </div>
    <article class="panel">
      <div class="panel-head"><div><h2>Этапы проверки</h2><div class="section-kicker">проверка ничего не меняет в рабочей конфигурации</div></div></div>
      <div id="v2DomainStages" class="v2-stage-list"><div class="analysis-empty">Проверка ещё не запускалась.</div></div>
    </article>
    <div class="v2-two">
      <article class="panel"><div class="panel-head"><div><h2>Списки</h2><div class="section-kicker">где найден этот домен</div></div></div><div id="v2DomainLists" class="v2-list"></div></article>
      <article class="panel"><div class="panel-head"><div><h2>Активные правила</h2><div class="section-kicker">какие текущие профили могут относиться к домену</div></div></div><div id="v2DomainProfiles" class="v2-list"></div></article>
    </div>
    <div class="v2-boundary">Проверка домена только диагностирует доступ и текущую конфигурацию. Если проблема относится к стратегии NFQWS, появится отдельное действие для подбора рабочего варианта.</div>
  </div>
</section>

<section id="view-prodlab-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Проверка текущей системы</h2><div class="section-kicker">несколько доменов без переключения стратегии</div></div>
      <span id="v2LabState" class="config-badge">ОЖИДАНИЕ</span>
    </div>
    <div class="v2-two">
      <article class="panel">
        <div class="panel-head"><div><h2>Цели проверки</h2><div class="section-kicker">ручной ввод или выбранный список доменов</div></div></div>
        <label class="v2-label">Источник
          <select id="v2LabSource" class="search"><option value="manual">Ручной ввод</option><option value="list">Список</option></select>
        </label>
        <label class="v2-label">Список
          <select id="v2LabList" class="search" disabled><option>загрузка…</option></select>
        </label>
        <textarea id="v2LabTargets" class="v2-textarea" placeholder="youtube.com&#10;discord.com"></textarea>
        <div class="v2-inline">
          <label>Повторы <select id="v2LabRounds" class="search"><option value="1">1</option><option value="3" selected>3</option><option value="5">5</option></select></label>
          <label>Параллельно <select id="v2LabConcurrency" class="search"><option value="1">1</option><option value="2">2</option><option value="4" selected>4</option></select></label>
        </div>
        <div class="toolbar"><button id="v2LabResolve">Загрузить цели</button><button id="v2LabRun" class="primary">Запустить</button><button id="v2LabStop" disabled>Стоп</button></div>
      </article>
      <article class="panel">
        <div class="v2-stats">
          <div><span>Целей</span><strong id="v2LabCount">0</strong></div>
          <div><span>Успешных попыток</span><strong id="v2LabOK">0</strong></div>
          <div><span>Ошибок</span><strong id="v2LabFail">0</strong></div>
          <div><span>Прогресс</span><strong id="v2LabProgress">0%</strong></div>
        </div>
        <div id="v2LabResults" class="v2-list"><div class="analysis-empty">Сначала выберите цели.</div></div>
      </article>
    </div>
    <div class="v2-boundary">Проверка использует только текущую рабочую конфигурацию nfqws2 и не подставляет альтернативные стратегии.</div>
  </div>
</section>



<section id="view-target-memory-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>История подборов</h2><div class="section-kicker">сохранённые результаты прошлых проверок</div></div>
      <span id="v2MemoryState" class="config-badge">ОЖИДАНИЕ</span>
    </div>
    <div class="toolbar">
      <input id="v2MemoryFilter" class="search" placeholder="Фильтр по домену · example.com">
      <button id="v2MemoryRefresh">Обновить</button>
      <button id="v2MemoryClearTarget" class="danger">Очистить домен…</button>
      <button id="v2MemoryClearAll" class="danger">Очистить всю память…</button>
    </div>
    <div id="v2MemoryPolicy" class="v2-boundary">Правила хранения ещё не загружены.</div>
    <div id="v2MemoryList" class="v2-list" style="margin-top:10px"><div class="analysis-empty">Откройте вкладку — загрузим историю подборов.</div></div>
    <div class="v2-boundary">История используется только как справочная информация. Новый подбор всегда проверяет текущее соединение заново.</div>
  </div>
</section>

<section id="view-runtime-health-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Состояние NFQWS</h2><div class="section-kicker">процессы, NFQUEUE и доступные возможности</div></div>
      <span id="runtimeHealthState" class="config-badge">ОЖИДАНИЕ</span>
    </div>
    <div class="toolbar"><button id="runtimeHealthRefresh">Обновить</button></div>
    <div class="v2-two">
      <article class="panel">
        <div class="panel-head"><div><h2>NFQUEUE</h2><div class="section-kicker">процесс ↔ firewall ↔ ядро</div></div></div>
        <div id="runtimeNFQueueHealth" class="diagnostics"><div class="analysis-empty">Нет snapshot.</div></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Транспортные профили</h2><div class="section-kicker">возможности, доступные текущему nfqws2</div></div></div>
        <div id="runtimeTransportProfiles" class="diagnostics"><div class="analysis-empty">Нет snapshot.</div></div>
      </article>
    </div>
    <article class="panel" style="margin-top:12px">
      <div class="panel-head"><div><h2>Правила автоматизации</h2><div class="section-kicker">автоматические действия требуют актуальной проверки</div></div></div>
      <div id="runtimeAutomationPolicy" class="diagnostics"><div class="analysis-empty">Нет snapshot.</div></div>
    </article>
    <div class="v2-boundary">Эта вкладка только показывает состояние системы и ничего не меняет.</div>
  </div>
</section>

<section id="view-auto-setup-v1" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Автонастройка</h2><div class="section-kicker">проверить сайт → найти проблему → подобрать рабочую стратегию → показать изменения → применить</div></div>
      <span id="v2AutoSetupState" class="config-badge">ОЖИДАНИЕ</span>
    </div>

    <div class="v2-two">
      <article class="panel">
        <div class="panel-head"><div><h2>1. Что проверить</h2><div class="section-kicker">домен можно ввести вручную или выбрать из списка</div></div></div>
        <label class="v2-label">Источник
          <select id="v2AutoSetupSource" class="search">
            <option value="manual">Ручной ввод</option>
            <option value="observed">Обнаруженный домен</option>
            <option value="list">Список / домен</option>
          </select>
        </label>
        <label id="v2AutoSetupManualWrap" class="v2-label">Домен
          <input id="v2AutoSetupTarget" class="search" placeholder="youtube.com" autocomplete="off" spellcheck="false">
        </label>
        <label id="v2AutoSetupObservedWrap" class="v2-label" style="display:none">Обнаруженный домен
          <select id="v2AutoSetupObserved" class="search"><option value="">Загрузка…</option></select>
        </label>
        <div id="v2AutoSetupListWrap" style="display:none">
          <label class="v2-label">Список
            <select id="v2AutoSetupList" class="search"><option value="">Загрузка…</option></select>
          </label>
          <label class="v2-label">Домен
            <select id="v2AutoSetupListTarget" class="search"><option value="">Сначала выберите список</option></select>
          </label>
        </div>
        <div class="v2-inline">
          <label>Режим
            <select id="v2AutoSetupMode" class="search">
              <option value="fast">Быстрый</option>
              <option value="normal" selected>Обычный</option>
              <option value="thorough">Полный</option>
            </select>
          </label>
          <label>Потоки
            <select id="v2AutoSetupConcurrency" class="search">
              <option value="1">1</option>
              <option value="2" selected>2</option>
              <option value="3">3</option>
              <option value="4">4</option>
            </select>
          </label>
        </div>
        <div class="toolbar">
          <button id="v2AutoSetupRun" class="primary">Диагностировать и подобрать</button>
          <button id="v2AutoSetupReset">Сбросить</button>
        </div>
      </article>

      <article class="panel">
        <div class="panel-head"><div><h2>2. Что происходит</h2><div class="section-kicker">что проверено, что найдено и почему выбран следующий шаг</div></div></div>
        <div id="v2AutoSetupStatus" class="v2-recommendation"><div class="analysis-empty">Выберите сайт и запустите проверку.</div></div>
        <div id="v2AutoSetupTimeline" class="v2-list" style="margin-top:10px"></div>
      </article>
    </div>

    <article id="v2AutoSetupWinner" class="panel" style="display:none">
      <div class="panel-head"><div><h2>3. Найденная стратегия</h2><div class="section-kicker">результат подтверждён текущей проверкой</div></div></div>
      <div id="v2AutoSetupWinnerBody" class="v2-recommendation"></div>
    </article>

    <article id="v2AutoSetupPreviewPanel" class="panel" style="display:none">
      <div class="panel-head"><div><h2>4. Предпросмотр</h2><div class="section-kicker">точные изменения перед применением</div></div></div>
      <div id="v2AutoSetupPreviewMeta" class="v2-boundary"></div>
      <pre id="v2AutoSetupPreviewBody" class="catalog-preview">Предпросмотр ещё не построен.</pre>
      <div class="toolbar">
        <button id="v2AutoSetupRebuildPreview">Обновить предпросмотр</button>
        <button id="v2AutoSetupApply" class="primary" disabled>Применить изменения</button>
      </div>
    </article>

    <div class="v2-boundary">
      Автонастройка ничего не применяет до последней кнопки. До этого выполняются только диагностика, проверка стратегий и предпросмотр изменений.
    </div>
  </div>
</section>

<section id="view-selector-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Автоподбор стратегии</h2><div class="section-kicker">проверка соединения → перебор стратегий → выбор результата → предпросмотр</div></div>
      <span id="v2SelectorState" class="config-badge">ОЖИДАНИЕ</span>
    </div>
    <div class="v2-two">
      <article class="panel">
        <label class="v2-label">Цель<input id="v2SelectorTarget" class="search" placeholder="youtube.com" autocomplete="off"></label>
        <div class="v2-inline">
          <label>Режим<select id="v2SelectorMode" class="search"><option value="fast">Быстрый</option><option value="normal" selected>Обычный</option><option value="thorough">Полный</option></select></label>
          <label>Потоки<select id="v2SelectorConcurrency" class="search"><option value="1">1</option><option value="2" selected>2</option><option value="3">3</option><option value="4">4</option></select></label>
        </div>
        <label class="v2-check"><input id="v2SelectorProduction" type="checkbox" checked> Профили текущей конфигурации nfqws2</label>
        <label class="v2-check"><input id="v2SelectorAutoPool" type="checkbox" checked> Стратегии из встроенного пула RouterForge</label>
        <label class="v2-check"><input id="v2SelectorLibrary" type="checkbox" checked> Сохранённые пользовательские стратегии</label>
        <label class="v2-check"><input id="v2SelectorCommunity" type="checkbox"> Сообщество по AS провайдера (внешние запросы, по желанию)</label>
        <label class="v2-label">AS провайдера (необязательно — определится через RIPEstat)
          <input id="v2SelectorCommunityASN" type="number" class="search" min="1" max="4294967295" step="1" placeholder="Например, 12389">
        </label>
        <div id="v2SelectorPoolMeta" class="v2-boundary">RouterForge проверяет стратегии из встроенного пула реальными HTTPS-запросами.</div>
        <label class="v2-label">Дополнительная стратегия
          <input id="v2SelectorExtraName" class="search" placeholder="Название (необязательно)">
          <textarea id="v2SelectorExtraArgs" class="v2-textarea compact" placeholder="--hostlist-domains=... --filter-tcp=443 ..."></textarea>
        </label>
        <div class="toolbar"><button id="v2SelectorRun" class="primary">Запустить подбор</button><button id="v2SelectorStop" disabled>Стоп</button></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Ход проверки</h2><div id="v2SelectorSession" class="section-kicker mono">—</div></div></div>
        <div class="testlab-progress"><span id="v2SelectorBar" style="width:0%"></span></div>
        <div id="v2SelectorProgressText" class="v2-boundary">Готов к запуску.</div>
        <div class="v2-stats">
          <div><span>Этап</span><strong id="v2SelectorPhase">ОЖИДАНИЕ</strong></div>
          <div><span>Проверено</span><strong id="v2SelectorCompleted">0 / 0</strong></div>
          <div><span>Потоки</span><strong id="v2SelectorWorkers">—</strong></div>
          <div><span>Очистка</span><strong id="v2SelectorCleanup">—</strong></div>
        </div>
      </article>
    </div>
    <article class="panel">
      <div class="panel-head"><div><h2>Результаты</h2><div class="section-kicker">сначала надёжность ответа, затем задержка и скорость</div></div></div>
      <div id="v2SelectorRecommendation" class="v2-recommendation"><div class="analysis-empty">Автоподбор ещё не запускался.</div></div>

      <div id="v2SelectorResults" class="v2-list"></div>
    </article>
    <article id="v2SelectorApplyPanel" class="panel" style="display:none">
      <div class="panel-head"><div><h2>Применение</h2><div class="section-kicker">предпросмотр изменений перед применением рабочей стратегии</div></div></div>
      <div id="v2SelectorApplyReason" class="v2-boundary"></div>
      <div id="v2SelectorBindPanel" style="display:none;margin-top:10px">
        <div class="v2-bind-intro"><strong>Добавление нового профиля.</strong> Существующие блоки <code>NFQWS_ARGS_CUSTOM</code> не заменяются. Выберите одну из рабочих стратегий; RouterForge отдельно перепроверит только её и подготовит новый <code>--new</code> блок.</div>
        <label class="v2-label">Рабочая стратегия
          <select id="v2SelectorBindStrategy" class="search"></select>
        </label>
        <div id="v2SelectorBindStrategyDetail" class="v2-bind-profile-card"></div>
        <div id="v2SelectorBindConflicts" class="v2-bind-profile-card"></div>
        <div class="toolbar">
          <button id="v2SelectorPrepareProfileButton">Проверить и подготовить новый профиль</button>
        </div>
      </div>
      <div class="toolbar"><button id="v2SelectorPreview" disabled>Предпросмотр</button><button id="v2SelectorApply" class="primary" disabled>Применить стратегию</button></div>
      <pre id="v2SelectorPreviewBody" class="catalog-preview">Предпросмотр ещё не построен.</pre>
    </article>
  </div>
</section>

<section id="view-sandbox-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Песочница стратегий</h2><div class="section-kicker">проверка стратегии без изменения рабочей конфигурации</div></div>
      <span id="v2SandboxState" class="config-badge">ОЖИДАНИЕ</span>
    </div>
    <div class="v2-two">
      <article class="panel">
        <div class="panel-head"><div><h2>Цели проверки</h2><div class="section-kicker">ручной ввод или список доменов</div></div></div>
        <label class="v2-label">Источник целей<select id="v2SandboxTargetSource" class="search"><option value="manual">Ручной ввод</option><option value="list">Список</option></select></label>
        <label class="v2-label">Список<select id="v2SandboxTargetList" class="search" disabled><option>загрузка…</option></select></label>
        <textarea id="v2SandboxTargets" class="v2-textarea compact" placeholder="youtube.com"></textarea>
        <div class="panel-head" style="margin-top:12px"><div><h2>Стратегия</h2><div id="v2SandboxSeedMeta" class="section-kicker">выберите источник</div></div></div>
        <label class="v2-label">Источник стратегии<select id="v2SandboxCandidateSource" class="search"><option value="production">Профиль nfqws2</option><option value="library">Библиотека стратегий</option><option value="custom">Свои аргументы</option></select></label>
        <label class="v2-label">Профиль nfqws2<select id="v2SandboxProfile" class="search"></select></label>
        <label class="v2-label">Сохранённая стратегия<select id="v2SandboxLibrary" class="search" disabled></select></label>
        <label class="v2-label">Аргументы<textarea id="v2SandboxArgs" class="v2-textarea compact" placeholder="Аргументы nfqws2 для проверки"></textarea></label>
        <div class="v2-inline"><label>Попытки<select id="v2SandboxAttempts" class="search"><option value="1">1</option><option value="2" selected>2</option><option value="3">3</option><option value="5">5</option></select></label><label>Название<input id="v2SandboxSaveName" class="search" placeholder="Моя стратегия"></label></div>
        <div class="toolbar"><button id="v2SandboxRun" class="primary">Запустить проверку</button><button id="v2SandboxStop" disabled>Стоп</button><button id="v2SandboxSave">Сохранить стратегию</button></div>
      </article>
      <article class="panel">
        <div id="v2SandboxResults" class="v2-list"><div class="analysis-empty">Результатов пока нет.</div></div>
      </article>
    </div>
    <div class="v2-boundary">Песочница использует тот же механизм проверки, что и автоподбор, и не меняет рабочую конфигурацию.</div>
  </div>
</section>



<section id="view-tcp16-memory-v1" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>История сетевых обрывов</h2><div class="section-kicker">наблюдения по обрыву соединения после 16–20 КБ</div></div>
      <span id="v2TCP16State" class="config-badge">ОЖИДАНИЕ</span>
    </div>
    <article class="panel">
      <div class="panel-head">
        <div><h2>Сетевая память</h2><div class="section-kicker">заполняется существующим «Разбором домена», когда HTTP probe даёт пригодный результат</div></div>
        <button id="v2TCP16Refresh">Обновить</button>
      </div>
      <div class="registry-summary">
        <div><span>Сетей</span><strong id="v2TCP16Count">0</strong></div>
        <div><span>Подозрение TCP16</span><strong id="v2TCP16Suspected">0</strong></div>
        <div><span>Чистых</span><strong id="v2TCP16Clear">0</strong></div>
        <div><span>Рабочих SNI</span><strong id="v2TCP16SNI">0</strong></div>
      </div>
    </article>
    <article class="panel" style="margin-top:12px">
      <div id="v2TCP16List" class="tcp16-memory-list"><div class="analysis-empty">Память ещё не загружена.</div></div>
    </article>
    <div class="v2-boundary">Здесь хранится только история наблюдений. Рабочая конфигурация nfqws2 не изменяется.</div>
  </div>
</section>

<section id="view-strategy-registry-v1" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Реестр стратегий</h2><div class="section-kicker">встроенные и пользовательские стратегии в одном списке</div></div>
      <span id="v2RegistryState" class="config-badge">ОЖИДАНИЕ</span>
    </div>

    <article class="panel">
      <div class="panel-head">
        <div><h2>База стратегий</h2><div class="section-kicker">просмотр без изменения рабочей конфигурации</div></div>
        <button id="v2RegistryRefresh">Обновить</button>
      </div>
      <div class="registry-summary">
        <div><span>Стратегий</span><strong id="v2RegistryCount">0</strong></div>
        <div><span>Источников</span><strong id="v2RegistrySourcesCount">0</strong></div>
        <div><span>Протоколов</span><strong id="v2RegistryProtocolsCount">0</strong></div>
        <div><span>С историей</span><strong id="v2RegistryEvidenceCount">0</strong></div>
        <div><span>Готовы к проверке</span><strong id="v2RegistryAutomationReady">0</strong></div>
        <div><span>Лучшие по истории</span><strong id="v2RegistryRecommendationTop">0</strong></div>
      </div>
      <div class="registry-filters">
        <input id="v2RegistrySearch" class="search" placeholder="Поиск по имени, источнику, семейству или аргументам">
        <select id="v2RegistryProtocol" class="search"><option value="">Все протоколы</option></select>
        <select id="v2RegistrySource" class="search"><option value="">Все источники</option></select>
      </div>
    </article>

    <article class="panel" style="margin-top:12px">
      <div id="v2RegistryList" class="registry-list"><div class="analysis-empty">Реестр ещё не загружен.</div></div>
    </article>

    <div class="v2-boundary">Реестр ничего не применяет автоматически. Перед использованием стратегия всё равно проходит актуальную проверку.</div>
  </div>
</section>

<section id="view-strategy-library-v2" class="view">
  <div class="v2-shell">
    <div class="v2-hero">
      <div><h2>Свои стратегии</h2><div class="section-kicker">persistent Библиотека стратегий на роутере</div></div>
      <span id="v2LibraryState" class="config-badge">IDLE</span>
    </div>
    <div class="v2-two">
      <article class="panel">
        <div class="panel-head"><div><h2>Сохранённые candidates</h2><div id="v2LibraryCount" class="section-kicker">0</div></div><button id="v2LibraryRefresh">Обновить</button></div>
        <div id="v2LibraryList" class="v2-list"><div class="analysis-empty">Загрузка…</div></div>
      </article>
      <article class="panel">
        <label class="v2-label">Название<input id="v2LibraryName" class="search" placeholder="YouTube strategy"></label>
        <label class="v2-label">Источник<select id="v2LibrarySource" class="search"><option value="custom">Своя</option><option value="catalog">Из каталога</option><option value="import">Импортированная</option></select></label>
        <label class="v2-label">Args<textarea id="v2LibraryArgs" class="v2-textarea" placeholder="--filter-tcp=443 ..."></textarea></label>
        <input id="v2LibraryID" type="hidden">
        <div class="toolbar"><button id="v2LibrarySave" class="primary">Сохранить</button><button id="v2LibraryClear">Новая</button><button id="v2LibraryToSandbox">В Sandbox</button></div>
      </article>
    </div>
    <div class="v2-boundary">Library хранит только bench-eligible candidate args и fingerprint. Она не меняет active nfqws2.conf и не запускает сервис.</div>
  </div>
</section>

<section id="view-overview" class="view active">
  <section class="ux-task-center" aria-labelledby="uxTaskTitle">
    <div class="ux-task-head">
      <div>
        <h2 id="uxTaskTitle">Что хотите сделать?</h2>
        <p>Выберите задачу обычными словами. RouterForge откроет нужный инструмент; технические детали и ручные настройки никуда не исчезают.</p>
      </div>
      <div class="ux-task-head-note">Начните с задачи<br>детали — по необходимости</div>
    </div>
    <div class="ux-task-grid">
      <button type="button" class="ux-task-card" id="uxTaskBroken">
        <span class="ux-task-icon">01</span>
        <span class="ux-task-copy"><span class="ux-task-title">Сайт или приложение не работает</span><span class="ux-task-desc">Разберём домен: списки, активный конфиг, runtime, интернет-интерфейс и проверку доступности с роутера.</span></span>
        <span class="ux-task-action">Начать диагностику</span>
      </button>
      <button type="button" class="ux-task-card" id="uxTaskCheck">
        <span class="ux-task-icon">02</span>
        <span class="ux-task-copy"><span class="ux-task-title">Проверить текущую настройку</span><span class="ux-task-desc">Проверим сервис, конфигурацию, зависимости и доступные runtime-сигналы без изменения настроек.</span></span>
        <span class="ux-task-action">Проверить</span>
      </button>
      <button type="button" class="ux-task-card" id="uxTaskAutoV2">
        <span class="ux-task-icon">03</span>
        <span class="ux-task-copy"><span class="ux-task-title">Подобрать настройку автоматически</span><span class="ux-task-desc">Baseline, production profiles и Библиотека стратегий тестируются в изолированных NFQUEUE. Результат отделяет strategy failure от infrastructure failure.</span></span>
        <span class="ux-task-action">Открыть Автоподбор</span>
      </button>
      <button type="button" class="ux-task-card" id="uxTaskImport">
        <span class="ux-task-icon">04</span>
        <span class="ux-task-copy"><span class="ux-task-title">Импортировать готовую стратегию</span><span class="ux-task-desc">Вставьте команду или загрузите .bat/.txt. RouterForge разберёт вход, покажет все преобразования и соберёт preview без сохранения.</span></span>
        <span class="ux-task-action">Разобрать</span>
      </button>
    </div>
    <div class="ux-task-foot">Эти быстрые сценарии не скрывают экспертные возможности: полная конфигурация, списки, NFQWS Menu, инструменты и обслуживание остаются доступны в верхней навигации.</div>
  </section>
  <div class="grid cards overview-metrics">
    <article class="card"><div class="metric-label">Сервис</div><div id="mService" class="metric-value">—</div><div id="mServiceSub" class="metric-sub">—</div></article>
    <article class="card"><div class="metric-label">Конфигурация</div><div id="mConfig" class="metric-value">—</div><div id="mConfigSub" class="metric-sub">—</div></article>
    <article class="card"><div class="metric-label">Стратегии</div><div id="mStrategies" class="metric-value accent">—</div><div class="metric-sub">разобрано из активного конфига</div></article>
    <article class="card"><div class="metric-label">Списки</div><div id="mLists" class="metric-value">—</div><div id="mListsSub" class="metric-sub">—</div></article>
  </div>
  <div class="grid overview-details">
    <article class="panel overview-runtime">
      <div class="panel-head"><div><h2>Runtime</h2><div class="section-kicker">Фактическое состояние внешнего nfqws</div></div></div>
      <dl class="kv">
        <dt>Target</dt><dd id="oTarget" class="mono">—</dd>
        <dt>Init script</dt><dd id="oInit" class="mono">—</dd>
        <dt>Config</dt><dd id="oConfigPath" class="mono">—</dd>
        <dt>Config SHA256</dt><dd id="oSha" class="mono">—</dd>
        <dt>Lists root</dt><dd id="oListsRoot" class="mono">—</dd>
        <dt>Log</dt><dd id="oLogPath" class="mono">—</dd>
        <dt>Mutation boundary</dt><dd id="oMutation">—</dd>
      </dl>
    </article>
    <article class="panel"><div class="panel-head"><div><h2>Состояние</h2><div class="section-kicker">Быстрая проверка runtime и safety boundary</div></div></div><div id="healthActivity" class="activity"></div></article>
  </div>
</section>

<section id="view-strategy-import" class="view">
  <div class="strategy-import-shell">
    <div class="strategy-import-hero">
      <div>
        <div class="testlab-title-row"><h2 style="margin:0">Импорт стратегии</h2><span id="strategyImportState" class="testlab-state idle">IDLE</span><span class="config-badge">ТОЛЬКО ПРОСМОТР</span></div>
        <div class="preflight-sub">Вставьте команду NFQWS/Windows или загрузите .bat/.txt. Сначала RouterForge только проверит и покажет результат — ничего не будет применено.</div>
      </div>
      <div class="toolbar"><input id="strategyImportFile" type="file" accept=".bat,.cmd,.txt" hidden><button id="strategyImportPick">Выбрать файл</button><button id="strategyImportRun" class="primary">Разобрать</button></div>
    </div>

    <div class="strategy-import-layout">
      <article class="panel strategy-import-input-panel">
        <div class="panel-head"><div><h2>Исходник</h2><div id="strategyImportSourceMeta" class="section-kicker">Команда · .bat · .cmd · .txt · лог теста</div></div><button id="strategyImportClear">Очистить</button></div>
        <textarea id="strategyImportInput" class="strategy-import-input" spellcheck="false" placeholder="@echo off&#10;start &quot;NFQWS&quot; /min &quot;%BIN%winws.exe&quot; --wf-tcp=80,443 ^&#10;--filter-tcp=443 --hostlist=&quot;%LISTS%list-general.txt&quot; --dpi-desync=fake"></textarea>
        <div class="strategy-import-note">Файл читается только браузером этой страницы. На роутер исходник не загружается. UX2.1 не пишет nfqws2.conf, списки или blobs.</div>
      </article>

      <article class="panel">
        <div class="strategy-import-summary">
          <div><span>Тип</span><strong id="strategyImportType">—</strong></div>
          <div><span>Команд</span><strong id="strategyImportCommands">0</strong></div>
          <div><span>Профилей</span><strong id="strategyImportProfiles">0</strong></div>
          <div><span>Статус</span><strong id="strategyImportVerdict">—</strong></div>
        </div>
        <div id="strategyImportMessage" class="strategy-import-message">Добавьте исходник и нажмите «Разобрать».</div>
        <div id="strategyImportWarnings" class="strategy-import-warnings"></div>
      </article>
    </div>

    <div class="strategy-import-results">
      <article class="panel">
        <div class="panel-head"><div><h2>Что RouterForge изменит</h2><div class="section-kicker">Каждое преобразование показывается явно</div></div></div>
        <div id="strategyImportTransforms" class="strategy-import-list"><div class="analysis-empty">Преобразований пока нет.</div></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Зависимости</h2><div class="section-kicker">Списки, blobs и неразрешённые переменные</div></div></div>
        <div id="strategyImportDeps" class="strategy-import-list"><div class="analysis-empty">Зависимости пока не определены.</div></div>
      </article>
    </div>

    <article class="panel" style="margin-top:10px">
      <div class="panel-head"><div><h2>Предварительный результат</h2><div class="section-kicker">стратегия до добавления в полный nfqws2.conf</div></div><button id="strategyImportCopy" disabled>Копировать</button></div>
      <pre id="strategyImportPreview" class="strategy-import-preview strategy-import-syntax-preview">Preview ещё не построен.</pre>
      <div class="strategy-import-note">RouterForge намеренно не подставляет неизвестные Windows-переменные и не переименовывает payload-файлы «по догадке». Если вход неоднозначен, результат остаётся CHECK вместо притворного готового конфига.</div>
    </article>

    <article id="strategyImportLibraryPanel" class="panel strategy-import-library-panel">
      <div class="panel-head">
        <div><h2>Сохранить как вариант конфигурации</h2><div class="section-kicker">Config Library · inactive until separate activation</div></div>
        <span id="strategyImportLibraryState" class="config-badge">WAITING</span>
      </div>
      <div class="strategy-import-library-grid">
        <div>
          <label class="strategy-import-library-label">Имя файла
            <input id="strategyImportLibraryName" class="search" placeholder="strategy-import.conf" autocomplete="off">
          </label>
          <div id="strategyImportLibraryPlan" class="strategy-import-note">Сначала нужна успешная проверка. Результат с замечаниями не сохраняется.</div>
          <div class="strategy-import-library-actions">
            <button id="strategyImportLibrarySave" class="primary" disabled>Сохранить конфигурацию</button>
            <button id="strategyImportLibraryOpen" disabled>Открыть библиотеку</button>
          </div>
          <div class="strategy-import-sandbox-handoff">
            <label>Профиль для Sandbox
              <select id="strategyImportSandboxProfile" class="search" disabled></select>
            </label>
            <button id="strategyImportSandboxOpen" disabled>Проверить в песочнице</button>
          </div>
          <div class="strategy-import-strategy-import">
            <button id="strategyImportStrategyImport" disabled>Добавить профили в библиотеку стратегий</button>
            <span id="strategyImportStrategyImportState" class="config-badge">WAITING</span>
          </div>
          <div class="strategy-import-note">Config Library хранит полный inactive config. Strategy Library сохраняет только bench-eligible profiles с source=import и deterministic fingerprint. Active nfqws2.conf не меняется.</div>
        </div>
        <div>
          <div class="strategy-import-library-diff-title">Точный diff относительно active config</div>
          <pre id="strategyImportLibraryDiff" class="strategy-import-library-diff">Diff появится после READY preview.</pre>
        </div>
      </div>
    </article>
  </div>
</section>

<section id="view-preflight" class="view">
  <div class="preflight-shell">
    <div class="preflight-hero">
      <div>
        <div class="preflight-title-row"><h2 style="margin:0">Diagnostics Preflight</h2><span id="preflightVerdict" class="preflight-verdict check">CHECK</span><span id="preflightScopeBadge" class="config-badge">ACTIVE</span></div>
        <div id="preflightSubtitle" class="preflight-sub">Проверка runtime, активного/draft-конфига, ссылок на списки и базовой готовности перед применением.</div>
      </div>
      <div>
        <div class="preflight-counts"><div class="preflight-count"><span>Ошибки</span><strong id="preflightBad" class="bad-text">0</strong></div><div class="preflight-count"><span>Проверить</span><strong id="preflightWarn" class="warn-text">0</strong></div><div class="preflight-count"><span>PASS</span><strong id="preflightGood" class="good-text">0</strong></div></div>
        <div class="toolbar" style="justify-content:flex-end;margin-top:8px"><button id="refreshPreflight">Обновить runtime</button></div>
      </div>
    </div>
    <div id="preflightGroups" class="preflight-groups"></div>
    <div class="preflight-note">Это preflight конфигурации и доступных runtime-сигналов RouterForge. Он не доказывает фактический захват конкретного пакета NFQUEUE и не заменяет live-тест стратегии — это пойдёт отдельным слоем Test Lab.</div>
  </div>
</section>


<section id="view-processing" class="view">
  <div class="testlab-shell">
    <div class="testlab-hero">
      <div>
        <div class="testlab-title-row"><h2 style="margin:0">Диагностика сайта</h2><span id="processingMapState" class="testlab-state idle">IDLE</span><span class="config-badge">READ-ONLY</span></div>
        <div class="preflight-sub">Введите домен. RouterForge отдельно покажет обычную проверку с самого роутера и изолированный Live NFQWS-тест — это два разных сетевых пути и они не подменяют друг друга.</div>
      </div>
      <div class="toolbar"><button id="copyProcessingMap" disabled>Копировать отчёт</button><button id="runProcessingMap" class="primary">Проверить</button></div>
    </div>
    <div class="guided-shell">
      <div class="guided-steps" aria-label="Этапы диагностики">
        <div class="guided-step"><span class="guided-step-num">1</span><span><strong>Укажите сайт</strong><span>Например youtube.com</span></span></div>
        <div class="guided-step"><span class="guided-step-num">2</span><span><strong>Проверка текущего состояния</strong><span>Runtime · списки · WAN · HTTP</span></span></div>
        <div class="guided-step"><span class="guided-step-num">3</span><span><strong>Live-проверка NFQWS при необходимости</strong><span>Тестовая NFQUEUE · TLS · очистка</span></span></div>
      </div>
      <article id="guidedTroubleshooter" class="guided-result" aria-live="polite">
        <div class="guided-result-head">
          <div><div id="guidedTitle" class="guided-result-title">Результат диагностики</div><div id="guidedDetail" class="guided-result-detail"></div></div>
          <div class="guided-result-tools"><button id="processingLiveCheck" disabled>Проверить через NFQWS</button><button id="copyGuidedReport" disabled>Копировать отчёт</button><span id="guidedBadge" class="guided-result-badge warn">CHECK</span></div>
        </div>
        <div class="guided-evidence">
          <div class="guided-evidence-item"><span>nfqws2</span><strong id="guidedRuntime">—</strong></div>
          <div class="guided-evidence-item"><span>Совпадения в списках</span><strong id="guidedLists">—</strong></div>
          <div class="guided-evidence-item"><span>Интерфейс</span><strong id="guidedInterface">—</strong></div>
          <div class="guided-evidence-item"><span>HTTP с роутера</span><strong id="guidedProbe">—</strong></div>
          <div class="guided-evidence-item"><span>Live NFQWS</span><strong id="processingLiveState">не запускался</strong></div>
        </div>
        <div class="guided-next">
          <div class="guided-next-title">Что это значит</div>
          <div id="guidedNextText" class="guided-next-text"></div>
          <div id="guidedActions" class="guided-actions"></div>
        </div>
        <details id="processingTechnical" class="processing-technical">
          <summary>Технические детали <span class="muted">конфиг · списки · логи · границы наблюдаемости</span></summary>
          <section class="guided-timeline-wrap" aria-labelledby="guidedTimelineTitle">
            <div class="guided-timeline-head"><strong id="guidedTimelineTitle">Что удалось доказать</strong><span>evidence timeline</span></div>
            <div id="guidedTimeline" class="guided-timeline"></div>
          </section>
        </details>
      </article>
    </div>
    <div class="testlab-layout">
      <article class="panel testlab-targets">
        <div class="panel-head"><div><h2>Домен</h2><div class="section-kicker">public DNS hostname · один за запуск</div></div></div>
        <input id="processingMapDomain" class="search" style="width:100%;max-width:none" placeholder="youtube.com">
        <div class="testlab-note">Обычная проверка ничего не меняет. «Проверить через NFQWS» запускает изолированный Fast-тест в тестовой NFQUEUE 30000–30127; рабочий конфиг не применяется и не перезаписывается, а после теста обязательно проверяется очистка.</div>
        <div id="processingMapHints" class="diagnostics" style="margin-top:12px"></div>
      </article>
      <article class="panel">
        <div class="testlab-summary">
          <div class="testlab-stat"><span>Режим</span><strong id="processingMapMode">—</strong></div>
          <div class="testlab-stat"><span>Списки</span><strong id="processingMapStatic">—</strong></div>
          <div class="testlab-stat"><span>HTTP</span><strong id="processingMapProbe">—</strong></div>
          <div class="testlab-stat"><span>Статус</span><strong id="processingMapRuntime">—</strong></div>
        </div>
        <div id="processingMapVerdict" class="diagnostics" style="margin-top:12px"><div class="analysis-empty">Введите домен и нажмите «Проверить».</div></div>
      </article>
    </div>

    <article class="panel processing-how">
      <div class="processing-how-head">
        <div>
          <h2>Как именно выполняется проверка</h2>
          <p>Обычная проверка показывает доступность сайта с самого роутера. Live NFQWS запускает отдельный временный candidate через тестовую NFQUEUE. Рабочая production-очередь q300 не используется как тестовая и не перенастраивается.</p>
        </div>
      </div>
      <div class="processing-route-grid">
        <section class="processing-route-card direct">
          <div class="processing-route-head">
            <div><div class="processing-route-title">1 · Обычная проверка</div><div class="processing-route-sub">Router-side HTTPS probe через системный маршрут</div></div>
            <span id="processingDirectBadge" class="processing-route-badge">НЕ ЗАПУСКАЛАСЬ</span>
          </div>
          <div id="processingDirectRoute" class="processing-route-flow"><div class="analysis-empty">Сначала нажмите «Проверить».</div></div>
          <div class="processing-route-note"><strong>Важно:</strong> этот probe не форсирует трафик через production NFQUEUE q300 и не доказывает путь пакета LAN-клиента.</div>
        </section>
        <section class="processing-route-card live">
          <div class="processing-route-head">
            <div><div class="processing-route-title">2 · Live NFQWS</div><div class="processing-route-sub">Изолированный Fast-тест временного candidate</div></div>
            <span id="processingLiveBadge" class="processing-route-badge">НЕ ЗАПУСКАЛСЯ</span>
          </div>
          <div id="processingLiveRoute" class="processing-route-flow"><div class="analysis-empty">После обычной проверки нажмите «Проверить через NFQWS».</div></div>
          <div class="processing-strategy-box">
            <div class="processing-strategy-head"><strong>Какие стратегии будут проверены</strong><span id="processingStrategyMode">Fast · baseline + до 3 профилей</span></div>
            <div id="processingStrategyPlan" class="processing-strategy-list"><div class="analysis-empty">План стратегий появится после обычной проверки.</div></div>
            <div class="processing-strategy-note"><strong>Baseline</strong> — контроль без desync-стратегии. Затем RouterForge по очереди берёт первые eligible профили из текущего production nfqws2, временно подставляет введённый домен и сравнивает TLS-доступность.</div>
          </div>
          <div class="processing-route-note"><strong>Важно:</strong> создаётся узкое временное правило только для тестового socket → тестовая NFQUEUE 30000–30127 → временный nfqws2 candidate → обязательная очистка.</div>
        </section>
      </div>
      <div class="processing-safety-strip">
        <div class="processing-safety-item good"><span>Production config</span><strong>не перезаписывается</strong></div>
        <div class="processing-safety-item good"><span>Production q300</span><strong>не перенастраивается</strong></div>
        <div class="processing-safety-item good"><span>Live cleanup</span><strong id="processingCleanupProof">не запускался</strong></div>
      </div>
    </article>

    <article class="panel" style="margin-top:14px">
      <div class="panel-head"><div><h2>Подробный путь и найденные списки</h2><div id="processingMapMeta" class="section-kicker">статический анализ + live evidence</div></div></div>
      <div id="processingMapSteps" class="static-path-steps"><div class="analysis-empty">Путь ещё не построен.</div></div>
    </article>
  </div>
</section>

<section id="view-strategies" class="view">
  <div class="setup-flow">
    <div class="setup-topbar setup-topbar-full">
      <div>
        <strong>Полная настройка nfqws2 сверху вниз</strong>
        <div class="muted small" style="margin-top:3px">Все параметры upstream доступны сразу. RouterForge сохраняет исходный порядок nfqws2.conf и не прячет служебные блоки.</div>
      </div>
      <span class="setup-full-badge">Полный режим</span>
    </div>
    <div id="processingOverview" class="setup-overview"></div>

    <article class="setup-section fixed-section compact-section" data-fixed="true">
      <div class="setup-head"><div class="setup-num">01</div><div><div class="setup-title">Интернет-интерфейс</div><div class="setup-key">ISP_INTERFACE</div></div><div class="setup-help">Куда уходит интернет-трафик. Обычно eth3 / eth2.2 / ppp0. Можно указать несколько интерфейсов через пробел.</div></div>
      <div class="setup-body"><div class="form-row"><div class="setup-label"><strong>Интерфейс провайдера</strong>WAN-интерфейс, через который выходит интернет.</div><div class="form-control"><input id="flowInterface" class="setup-input" spellcheck="false"></div></div></div>
    </article>

    <article class="setup-section expert-only">
      <div class="setup-head"><div class="setup-num">02</div><div><div class="setup-title">Базовая загрузка nfqws2</div><div class="setup-key">NFQWS_BASE_ARGS</div></div><div class="setup-help">Lua-модули и blobs, которые подгружаются один раз при запуске и используются стратегиями ниже.</div></div>
      <div class="setup-body"><textarea id="flowBaseArgs" class="setup-textarea large" spellcheck="false"></textarea><div class="setup-note warn">В этом блоке upstream запрещает <span class="mono">--new</span>. Профили здесь не переставляем и не дробим.</div></div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS">
      <div class="setup-head"><div class="setup-num">03</div><div><div class="setup-title">HTTPS / TCP</div><div class="setup-key">NFQWS_ARGS</div></div><div class="setup-help">Главная стратегия для HTTPS/TCP. Применяется к доменам в зависимости от выбранного режима работы.</div></div>
      <div class="setup-body"><div id="sumMain" class="setup-chips"></div><textarea id="flowArgs" class="setup-textarea large" spellcheck="false"></textarea><div class="setup-note">Порядок внутри этого блока сохраняем как в исходном конфиге. <span class="mono">--new</span> здесь не допускается самим nfqws2-keenetic.</div></div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS_QUIC">
      <div class="setup-head"><div class="setup-num">04</div><div><div class="setup-title">QUIC</div><div class="setup-key">NFQWS_ARGS_QUIC</div></div><div class="setup-help">Отдельная стратегия для QUIC, обычно UDP/443. При запуске nfqws2 к ней автоматически добавляются режим доменов и IPSET.</div></div>
      <div class="setup-body"><div id="sumQuic" class="setup-chips"></div><textarea id="flowQuic" class="setup-textarea" spellcheck="false"></textarea></div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS_UDP">
      <div class="setup-head"><div class="setup-num">05</div><div><div class="setup-title">Прочий UDP</div><div class="setup-key">NFQWS_ARGS_UDP</div></div><div class="setup-help">WireGuard, STUN, Discord, MTProto и другие UDP-протоколы. Этот блок не использует NFQWS_EXTRA_ARGS.</div></div>
      <div class="setup-body"><div id="sumUdp" class="setup-chips"></div><textarea id="flowUdp" class="setup-textarea large" spellcheck="false"></textarea><div class="setup-note good">Это отдельный UDP-профиль: доменные MODE_LIST / MODE_AUTO / MODE_ALL на него не навешиваются.</div></div>
    </article>

    <article class="setup-section fixed-section choice-section" data-fixed="true">
      <div class="setup-head"><div class="setup-num">06</div><div><div class="setup-title">Режим обработки доменов</div><div class="setup-key">MODE_LIST / MODE_ALL / MODE_AUTO → NFQWS_EXTRA_ARGS</div></div><div class="setup-help">Сами MODE_* — служебные шаблоны upstream. Мы их не редактируем, только выбираем один из них.</div></div>
      <div class="setup-body">
        <div class="mode-grid">
          <label class="mode-card" data-mode="MODE_LIST"><input type="radio" name="flowMode" value="MODE_LIST"><strong>Только мой список</strong><span>user.list</span></label>
          <label class="mode-card" data-mode="MODE_AUTO"><input type="radio" name="flowMode" value="MODE_AUTO"><strong>Автоматически</strong><span>user.list + auto.list</span></label>
          <label class="mode-card" data-mode="MODE_ALL"><input type="radio" name="flowMode" value="MODE_ALL"><strong>Весь трафик</strong><span>всё − exclude.list</span></label>
        </div>
        <div class="setup-note">Это именно <span class="mono">NFQWS_EXTRA_ARGS</span>. Upstream помечает MODE_LIST / MODE_ALL / MODE_AUTO как «don't change» — поэтому они здесь только как понятный переключатель.</div>
        <div class="list-links"><button class="list-link" data-open-list="user.list">Открыть user.list</button><button class="list-link" data-open-list="auto.list">Открыть auto.list</button><button class="list-link" data-open-list="exclude.list">Открыть exclude.list</button></div>
      </div>
    </article>

    <article class="setup-section" data-summary-key="NFQWS_ARGS_IPSET">
      <div class="setup-head"><div class="setup-num">07</div><div><div class="setup-title">IP/CIDR-фильтры <span class="muted small">· дополнительно</span></div><div class="setup-key">NFQWS_ARGS_IPSET</div></div><div class="setup-help">Upstream NFQWS_ARGS_IPSET: отдельные IP/CIDR-правила, независимые от режима доменов. Если они не нужны — этот блок можно просто оставить как есть.</div></div>
      <div class="setup-body"><div id="sumIpset" class="setup-chips"></div><textarea id="flowIpset" class="setup-textarea" spellcheck="false"></textarea><div class="list-links"><button class="list-link" data-open-list="ipset.list">Открыть ipset.list</button><button class="list-link" data-open-list="ipset_exclude.list">Открыть ipset_exclude.list</button></div></div>
    </article>

    <article class="setup-section" data-summary-key="CUSTOM">
      <div class="setup-head"><div class="setup-num">08</div><div><div class="setup-title">Дополнительные стратегии</div><div class="setup-key">NFQWS_ARGS_CUSTOM</div></div><div class="setup-help">Единственное место, где upstream разрешает несколько профилей через --new. Только здесь можно менять блоки местами.</div></div>
      <div class="setup-body">
        <div id="customBlocks" class="custom-blocks"></div>
        <button id="addCustomBlock" class="custom-add">+ Добавить custom-блок</button>
        <div class="setup-note good">Порядок блоков влияет на итоговую строку запуска. Перемещение ↑ / ↓ доступно только здесь — остальные секции следуют исходной архитектуре nfqws2.</div>
      </div>
    </article>

    <article class="setup-section fixed-section traffic-section" data-fixed="true">
      <div class="setup-head"><div class="setup-num traffic-range">09–11</div><div><div class="setup-title">Обработка трафика</div><div class="setup-key">IPV6_ENABLED / TCP_PORTS / UDP_PORTS</div></div><div class="setup-help">Три компактных параметра, которые определяют, какой трафик попадёт в NFQUEUE.</div></div>
      <div class="setup-body">
        <div class="traffic-rows">
          <div class="traffic-line">
            <span class="traffic-card-num">09</span>
            <div class="traffic-line-copy"><strong>IPv6</strong><span>IPV6_ENABLED</span></div>
            <div class="traffic-line-control"><label class="traffic-switch"><input id="flowIPv6" type="checkbox"><span class="traffic-switch-track"></span><strong>Обрабатывать IPv6</strong></label></div>
          </div>
          <div class="traffic-line">
            <span class="traffic-card-num">10</span>
            <div class="traffic-line-copy"><strong>TCP → NFQUEUE</strong><span>TCP_PORTS</span></div>
            <div class="traffic-line-control"><input id="flowTcpPorts" class="setup-input" spellcheck="false" placeholder="80,443,1984,2053,2083,2087,2096,5222,8443"></div>
          </div>
          <div class="traffic-line">
            <span class="traffic-card-num">11</span>
            <div class="traffic-line-copy"><strong>UDP → NFQUEUE</strong><span>UDP_PORTS</span></div>
            <div class="traffic-line-control"><input id="flowUdpPorts" class="setup-input" spellcheck="false" placeholder="443,590:600,1400,3478:3481,5349"></div>
          </div>
        </div>
      </div>
    </article>

    <article class="setup-section fixed-section choice-section" data-fixed="true">
      <div class="setup-head"><div class="setup-num">12</div><div><div class="setup-title">Политика Keenetic</div><div class="setup-key">POLICY_NAME / POLICY_EXCLUDE</div></div><div class="setup-help">Позволяет ограничить обработку устройствами из политики Keenetic или, наоборот, исключить их.</div></div>
      <div class="setup-body">
        <div class="policy-grid">
          <label class="policy-card" data-policy-mode="none"><input type="radio" name="policyMode" value="none"><strong>Без политики</strong><span>все устройства</span></label>
          <label class="policy-card" data-policy-mode="include"><input type="radio" name="policyMode" value="include"><strong>Только политика</strong><span>только POLICY_NAME</span></label>
          <label class="policy-card" data-policy-mode="exclude"><input type="radio" name="policyMode" value="exclude"><strong>Исключить политику</strong><span>кроме POLICY_NAME</span></label>
        </div>
        <div id="policyNameRow" class="form-row policy-name-compact"><div class="setup-label"><strong>Имя политики</strong>Точное имя политики Keenetic.</div><div class="form-control"><input id="flowPolicyName" class="setup-input" spellcheck="false"></div></div>
        <input id="flowPolicyExclude" type="hidden">
      </div>
    </article>

    <article class="setup-section expert-only">
      <div class="setup-head"><div class="setup-num">13</div><div><div class="setup-title">Логирование</div><div class="setup-key">LOG_LEVEL / LOG_DEBUG_PATH</div></div><div class="setup-help">Debug-лог нужен только для диагностики. Постоянно держать его включённым обычно не требуется.</div></div>
      <div class="setup-body form-stack">
        <div class="form-row"><div class="setup-label"><strong>Debug-логирование</strong>Включай только на время диагностики.</div><div class="form-control"><label class="config-check"><input id="flowLogLevel" type="checkbox"> Включить LOG_LEVEL=1</label></div></div>
        <div class="form-row"><div class="setup-label"><strong>Файл debug-лога</strong>Путь используется при LOG_LEVEL=1.</div><div class="form-control"><input id="flowLogPath" class="setup-input" spellcheck="false"></div></div>
      </div>
    </article>

    <article class="setup-section expert-only">
      <div class="setup-head"><div class="setup-num">14</div><div><div class="setup-title">Служебные параметры</div><div class="setup-key">NFQUEUE_NUM / USER / CONFIG_VERSION</div></div><div class="setup-help">Показываем для понимания механизма, но в обычном интерфейсе не даём случайно ломать.</div></div>
      <div class="setup-body"><div class="internal-grid"><div class="internal-item"><span>NFQUEUE_NUM</span><strong id="flowQueue">—</strong></div><div class="internal-item"><span>USER</span><strong id="flowUser">—</strong></div><div class="internal-item"><span>CONFIG_VERSION</span><strong id="flowConfigVersion">—</strong></div></div></div>
    </article>

    <article class="setup-section fixed-section diagnostics-section" data-fixed="true">
      <div class="setup-head"><div class="setup-num">✓</div><div><div class="setup-title">Проверка перед применением</div><div class="setup-key">RouterForge diagnostics</div></div><div class="setup-help">Проверяем порты, режимы, ссылки на списки и критические противоречия до записи файла.</div></div>
      <div class="setup-body diagnostics-body">
        <div id="diagnosticSummary" class="diagnostic-summary"></div>
        <div id="flowDiagnostics" class="diagnostics"></div>
      </div>
    </article>
    <div class="setup-sticky"><div><span id="setupState" class="setup-state">CLEAN</span><div id="changeSummary" class="change-summary"></div><div class="muted small" style="margin-top:3px">Сохраняется в тот же nfqws2.conf → backup → atomic write → reload → rollback при ошибке.</div></div><div class="toolbar"><button id="resetSetup" disabled>Отменить</button><button id="diffSetup" disabled>Diff</button><button id="applySetup" class="primary" disabled>Сохранить и применить</button></div></div>
  </div>
</section>

<section id="view-configs" class="view">
  <div class="config-library-grid">
    <article class="panel">
      <div class="panel-head"><div><h2>Полные конфиги</h2><div id="configLibraryRoot" class="section-kicker mono">/opt/etc/nfqws2/routerforge-configs</div></div></div>
      <div class="setup-note config-library-note">Здесь хранятся полные <span class="mono">nfqws2.conf</span>. Профиль не влияет на сервис, пока ты явно не нажмёшь «Активировать».</div>
      <div class="toolbar"><button id="newStoredConfig" class="primary">+ Пустой</button><button id="saveCurrentConfig">Сохранить текущий как…</button></div>
      <input id="configLibrarySearch" class="search" style="width:100%;max-width:none;margin-top:9px" placeholder="Поиск конфига…">
      <div id="configLibraryList" class="config-library-list"><div class="empty">Загрузка…</div></div>
      <div class="config-neighbor-head"><div><div class="config-neighbor-title">Конфиги рядом с nfqws2.conf</div><div id="neighborConfigRoot" class="section-kicker mono">/opt/etc/nfqws2</div></div><button id="refreshNeighborConfigs">Обновить</button></div>
      <div id="neighborConfigList" class="config-neighbor-list"><div class="config-neighbor-empty">Загрузка…</div></div>
    </article>
    <article class="panel">
      <div class="panel-head">
        <div><h2 id="storedConfigTitle">Выберите конфиг</h2><div id="storedConfigMeta" class="section-kicker mono">—</div></div>
        <div class="toolbar"><button id="renameStoredConfig" disabled>Переименовать</button><button id="duplicateStoredConfig" disabled>Копия</button><button id="deleteStoredConfig" class="danger" disabled>Удалить</button><button id="undoStoredConfig" disabled>Отменить</button><button id="diffStoredConfig" disabled>Diff</button><button id="saveStoredConfig" disabled>Сохранить</button><button id="activateStoredConfig" class="primary" disabled>Активировать</button></div>
      </div>
      <div id="configLibraryEditorEmpty" class="config-library-empty">Создай пустой конфиг, сохрани текущий как отдельный файл или скопируй найденный рядом с основным.</div>
      <div id="storedConfigEditorShell" class="editor-shell" style="display:none">
        <div class="editor-toolbar"><span id="storedConfigState" class="mono">—</span><div class="editor-toolbar-actions"><span>Ctrl+S → Diff · Tab = 2 spaces</span><button id="fullscreenStoredConfig" type="button">На весь экран</button></div></div>
        <div class="editor-main"><pre id="storedConfigGutter" class="gutter">1</pre><textarea id="storedConfigEditor" class="editor" spellcheck="false" placeholder="Вставь сюда полный nfqws2.conf"></textarea></div>
        <div class="editor-foot"><span id="storedConfigStats">—</span><span>library save ≠ activation · activate → backup → atomic switch → restart/verify</span></div>
      </div>
    </article>
  </div>
</section>

<section id="view-config" class="view">
  <article id="editorPanel" class="panel">
    <div class="panel-head">
      <div><h2>Полный nfqws2.conf</h2><div id="configMeta" class="section-kicker mono">—</div></div>
      <div class="toolbar"><button id="undoConfig">Отменить</button><button id="diffConfig">Diff</button><button id="checkConfigSyntax" type="button">Проверить конфиг</button><button id="save" class="primary">Сохранить и применить</button></div>
    </div>
    <div class="setup-note warn" style="margin-bottom:10px">Это полный конфиг nfqws2.conf. Для обычной настройки используйте вкладку «Настройка».</div>
    <div id="configSyntaxStatus" class="setup-note" role="status" aria-live="polite" style="margin-bottom:10px">Проверка конфига ещё не запускалась. Это статический анализ, не nfqws2 --dry-run.</div>
    <pre id="configSemanticIssues" role="status" aria-live="polite" style="display:none;white-space:pre-wrap;overflow-wrap:anywhere;margin:0 0 10px"></pre>
    <div id="nf1bLuaFixNote" class="setup-note" style="display:none;margin:0 0 10px">
      Для предупреждения о неизвестном payload: выдели целиком один аргумент <code>--lua-desync=fake:...</code> (или multisplit/hostfakesplit/multidisorder) в редакторе.
      <button id="nf1bLuaPayloadFix" type="button">Добавить payload=all к выделенному Lua</button>
      Изменение останется только в редакторе и откроет diff.
    </div>
    <details id="nf1bLuaPanel" class="panel" style="margin:0 0 12px;padding:12px">
      <summary style="cursor:pointer">Lua-функции, доступные на этом роутере · справочник</summary>
      <div class="toolbar" style="margin:10px 0">
        <input id="nf1bLuaQuery" type="search" placeholder="Поиск функции или аргумента…" style="flex:1;min-width:180px">
        <button id="nf1bLuaRefresh" type="button">Обновить</button>
      </div>
      <div id="nf1bLuaInfo" class="section-kicker" role="status">Открой справочник, чтобы прочитать локальные Lua-файлы без их выполнения.</div>
      <div id="nf1bLuaResults" class="v2-list" style="margin-top:10px"></div>
    </details>
    <div class="editor-shell">
      <div class="editor-toolbar"><span id="editorState" class="mono">CLEAN</span><span>Ctrl+S → Diff · Tab = 2 spaces</span></div>
      <div class="editor-main"><pre id="gutter" class="gutter">1</pre><textarea id="config" class="editor" spellcheck="false"></textarea></div>
      <div class="editor-foot"><span id="editorStats">—</span><span>backup → atomic write → reload → rollback</span></div>
    </div>
  </article>
</section>

<section id="view-lists" class="view">
  <div id="listWorkspace" class="list-grid rf-workspace">
    <article class="panel">
      <div class="panel-head"><div><h2>Списки</h2><div class="section-kicker">Системные · Пользовательские · OPKG</div></div><button id="newList" class="primary">+ Новый</button></div>
      <input id="listSearch" class="search" placeholder="Поиск списка…">
      <div id="fileList" class="file-list"></div>
    </article>
    <article id="listEditorPanel" class="panel">
      <div class="panel-head">
        <div><h2 id="selectedListTitle">Выберите список</h2><div id="selectedListMeta" class="section-kicker mono">—</div></div>
        <div class="toolbar"><button id="inspectListEntry" disabled>Инспектор</button><button id="staticPathEntry" disabled>Путь</button><button id="analyzeLists" disabled>Пересечения</button><button id="checkList" disabled>Проверить</button><button id="normalizeList" disabled>Очистить дубли</button><button id="fullscreenList" disabled>На весь экран</button><button id="undoList" disabled>Отменить</button><button id="saveList" class="primary" disabled>Сохранить</button><button id="previewRenameList" disabled>Переименовать · Preview</button><button id="deleteList" class="danger" disabled>Удалить</button></div>
      </div>
      <div class="list-editor-main">
        <pre id="listGutter" class="list-line-gutter" aria-hidden="true">1</pre>
        <div class="list-editor-body">
          <pre id="listHighlight" class="list-editor-highlight" aria-hidden="true"></pre>
          <textarea id="listEditor" class="editor list-editor" spellcheck="false" placeholder="Выберите список слева."></textarea>
        </div>
      </div>
      <div class="editor-foot"><div id="listStats" class="list-stats">—</div><span>save → safety backup → atomic write</span></div>
    </article>
  </div>
</section>



<section id="view-upstream-lists" class="view">
  <div class="blob-shell">
    <div class="blob-hero">
      <div>
        <div class="blob-title-row"><h2 style="margin:0">Списки NFQWS Menu</h2><span id="listSourceState" class="config-badge">ОЖИДАНИЕ</span></div>
        <div class="muted small" style="margin-top:5px">Локальные списки и готовые списки из NFQWS Menu. Перед заменой RouterForge создаёт резервную копию.</div>
      </div>
      <div class="toolbar"><button id="refreshListSources">Обновить sources</button></div>
    </div>
    <div class="blob-summary">
      <div class="blob-stat"><span>Локально</span><strong id="listSourceLocalCount">0</strong></div>
      <div class="blob-stat"><span>NFQWS Menu</span><strong id="listSourceUpstreamCount">0</strong></div>
      <div class="blob-stat"><span>Нужно установить/обновить</span><strong id="listSourceMissingCount">0</strong></div>
      <div class="blob-stat"><span>Используются</span><strong id="listSourceReferencedCount">0</strong></div>
    </div>
    <div class="blob-layout">
      <article class="panel">
        <div class="panel-head"><div><h2>Локальные списки</h2><div id="listSourceRootMeta" class="section-kicker mono">/opt/etc/nfqws2/lists</div></div></div>
        <div id="listSourceLocalList" class="blob-list"><div class="empty">Нажмите «Обновить sources».</div></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Списки NFQWS Menu</h2><div id="listSourceUpstreamMeta" class="section-kicker">официальный источник</div></div></div>
        <div id="listSourceUpstreamList" class="blob-deps"><div class="empty">Источник ещё не синхронизирован.</div></div>
      </article>
    </div>
    <div class="blob-note">Файлы проверяются перед установкой. Эти действия не меняют рабочий nfqws2.conf автоматически.</div>
  </div>
</section>

<section id="view-geo-assets" class="view">
  <div class="blob-shell">
    <article class="panel">
      <div class="panel-head">
        <div>
          <h2>Геоданные</h2>
          <div class="section-kicker">GeoSite / GeoIP и геосписки</div>
        </div>
        <span class="config-badge">В РАЗРАБОТКЕ</span>
      </div>
      <div class="setup-note" style="margin-top:10px">
        Раздел готовится. Здесь появятся импорт GeoSite / GeoIP, работа с категориями и создание геосписков для nfqws2.
      </div>
      <div class="setup-note" style="margin-top:8px">
        Текущие списки nfqws2 остаются доступны во вкладке «Списки». Рабочая конфигурация и runtime этим разделом сейчас не изменяются.
      </div>
    </article>
  </div>
</section><section id="view-catalog" class="view">
  <div class="catalog-shell">
    <div class="catalog-hero">
      <div>
        <div class="catalog-title-row"><h2 style="margin:0">Каталог стратегий</h2><span class="config-badge active">ТОЛЬКО ПРОСМОТР</span></div>
        <div class="muted small" style="margin-top:5px">Сохранённые и готовые конфигурации NFQWS. Просмотр и проверка не меняют рабочие настройки.</div>
      </div>
      <div class="toolbar"><button id="refreshCatalog">Обновить источники</button></div>
    </div>
    <div class="catalog-sources">
      <div class="catalog-source-card"><div class="catalog-source-name">Рабочая и сохранённые конфигурации</div><div id="catalogLocalSource" class="catalog-source-meta">Локальный источник · загрузка…</div></div>
      <div class="catalog-source-card"><div class="catalog-source-name">rndnaame/nfqws-menu</div><div id="catalogUpstreamSource" class="catalog-source-meta">Configured upstream · main · не синхронизирован</div></div>

    </div>
    <div class="catalog-layout">
      <article class="panel catalog-browser">
        <input id="catalogSearch" class="search" placeholder="Стратегия / source / ref…">
        <div class="catalog-filters"><button id="catalogAll" class="active">Все</button><button id="catalogLocal">Локальные</button><button id="catalogUpstream">NFQWS Menu</button></div>
        <div id="catalogList" class="catalog-list"><div class="empty">Открой вкладку — соберём каталог.</div></div>
      </article>
      <article class="panel catalog-detail">
        <div id="catalogDetail" class="catalog-empty">Выберите стратегию слева. Просмотр не меняет рабочую конфигурацию.</div>
      </article>
    </div>
    <div class="catalog-note">Перед использованием RouterForge проверяет целостность полученных файлов.</div>
  </div>
</section>


<section id="view-clienthello" class="view">
  <div class="blob-shell">
    <article class="panel clienthello-lab">
      <div class="panel-head">
        <div><h2>ClientHello Forge</h2><div class="section-kicker">поймать настоящий ClientHello → заменить SNI → получить рабочий blob для nfqws2</div></div>
        <span id="clientHelloState" class="config-badge">IDLE</span>
      </div>

      <div class="setup-note"><b>Основной режим:</b> RouterForge берёт настоящий ClientHello выбранного устройства и меняет только SNI. Остальной TLS fingerprint сохраняется.</div>
      <div id="clientHelloCapabilitiesDetail" class="setup-note" style="margin-top:8px">Проверяю tcpdump и поддержку tls_client_hello_clone…</div>

      <div class="clienthello-capture">
        <div class="panel-head"><div><h2>1. Поймать ClientHello устройства</h2><div class="section-kicker">во время захвата открой новый HTTPS-сайт на выбранном устройстве</div></div><span id="clientHelloCaptureState" class="config-badge">IDLE</span></div>
        <div class="clienthello-grid">
          <label class="v2-label">IP устройства<input id="clientHelloDeviceIP" class="search mono" placeholder="192.168.1.50"></label>
          <label class="v2-label">Интерфейс<input id="clientHelloInterface" class="search mono" placeholder="пусто = br0 / auto"></label>
        </div>
        <div class="toolbar"><button id="clientHelloCapture" class="primary">Снять 5 секунд</button></div>
        <div id="clientHelloCaptureResult" class="clienthello-candidates"><div class="empty">Capture ещё не запускался.</div></div>
      </div>

      <div style="margin-top:14px">
        <div class="panel-head"><div><h2>2. Создать blob</h2><div class="section-kicker">укажи SNI, который должен быть внутри fake ClientHello</div></div><span id="clientHelloCloneCapability" class="config-badge">ПРОВЕРКА</span></div>
        <div class="clienthello-grid">
          <label class="v2-label">Новый SNI
            <input id="clientHelloSNI" class="search" placeholder="www.google.com">
          </label>
          <label class="v2-label">Имя blob
            <input id="clientHelloName" class="search mono" placeholder="tls_clienthello_www_google_com.bin">
          </label>
        </div>
        <div class="toolbar">
          <button id="clientHelloForge" class="primary" disabled>Создать из захваченного</button>
          <button id="clientHelloGenerate">Синтетический fallback</button>
          <button id="clientHelloSave" disabled>Сохранить blob</button>
        </div>
        <div id="clientHelloResult" class="clienthello-result"><div class="empty">Сначала поймай ClientHello устройства или используй синтетический fallback.</div></div>
        <pre id="clientHelloUsage" class="v2-args" style="margin-top:10px;white-space:pre-wrap">После создания здесь появятся параметры для nfqws2.</pre>
      </div>
    </article>
    <div class="blob-note">Сохранение пишет только .bin через Blob Manager. Конфиг nfqws2 и runtime автоматически не меняются.</div>
  </div>
</section>

<section id="view-blobs" class="view">
  <div class="blob-shell">
    <div class="blob-hero">
      <div>
        <div class="blob-title-row"><h2 style="margin:0">TLS-заготовки</h2><span id="blobRuntimeBadge" class="config-badge">ФАЙЛЫ</span></div>
        <div class="muted small" style="margin-top:5px">Локальные заготовки и файлы, необходимые выбранной стратегии. Перед установкой RouterForge проверяет их целостность.</div>
      </div>
      <div class="toolbar"><button id="refreshBlobs">Обновить inventory</button></div>
    </div>

    <div class="blob-summary">
      <div class="blob-stat"><span>Установлено</span><strong id="blobInstalledCount">0</strong></div>
      <div class="blob-stat"><span>Требуется</span><strong id="blobRequiredCount">0</strong></div>
      <div class="blob-stat"><span>Missing / update</span><strong id="blobMissingCount">0</strong></div>
      <div class="blob-stat"><span>Referenced</span><strong id="blobReferencedCount">0</strong></div>
    </div>
    <div class="blob-layout">
      <article class="panel">
        <div class="panel-head"><div><h2>Локальные blobs</h2><div id="blobRootMeta" class="section-kicker mono">/opt/etc/nfqws2/blobs</div></div></div>
        <div id="blobInventoryList" class="blob-list"><div class="empty">Открой вкладку — прочитаем inventory.</div></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Зависимости Catalog</h2><div id="blobCatalogMeta" class="section-kicker">Выберите config в Catalog</div></div><button id="openBlobCatalog">Открыть Catalog</button></div>
        <div id="blobDependencyList" class="blob-deps"><div class="empty">Выберите стратегию в Catalog, чтобы проверить её blobs.</div></div>
      </article>
    </div>
    <div class="blob-note"><b>Boundary R26:</b> установка/замена пишет только blob-файл через atomic write и safety backup. RouterForge <b>не</b> выполняет restart/reload и не утверждает, что running nfqws уже использует новый файл. Удаление запрещено, пока blob упоминается active или stored config. Координированное применение config + dependencies + verify/rollback остаётся за R28 Smart Apply.</div>
  </div>
</section>

<section id="view-runs" class="view">
  <div class="run-history-shell">
    <div class="run-history-hero">
      <div>
        <div class="run-history-title-row"><h2 style="margin:0">Run Metrics / Scoring / History</h2><span id="runHistoryStorage" class="config-badge active">BROWSER LOCAL</span></div>
        <div class="muted small" style="margin-top:5px">История завершённых Test Lab и Strategy Sandbox прогонов на этом браузере. Active runtime score и Sandbox readiness считаются раздельно.</div>
      </div>
      <div class="run-history-summary">
        <div class="run-history-stat"><span>Прогонов</span><strong id="runHistoryCount">0</strong></div>
        <div class="run-history-stat"><span>Последний score</span><strong id="runHistoryLatestScore">—</strong></div>
        <div class="run-history-stat"><span>Хранилище</span><strong id="runHistorySize">0 KB</strong></div>
      </div>
    </div>
    <div class="run-history-controls">
      <div class="run-history-filters"><button id="runHistoryAll" class="active">Все</button><button id="runHistoryTestLab">Тестирование</button><button id="runHistorySandbox">Песочница</button></div>
      <div class="toolbar"><button id="copyRunHistory">Копировать JSON</button><button id="clearRunHistory" class="danger">Очистить историю</button></div>
    </div>
    <div class="run-history-score-note"><b>Test Lab SCORE 0–100</b>: reachability 50% + stability 25% + latency 15% + decisive static-path coverage 10%. Latency получает максимум до 250 ms и плавно снижается до нуля к 2000 ms. <b>Sandbox READINESS</b> — отдельная оценка только семантической готовности candidate; active HTTP baseline в неё не входит и не выдаётся за результат candidate.</div>
    <div id="runHistoryList" class="run-history-list"></div>
    <div class="run-history-local">Хранится в localStorage текущего браузера, максимум 24 завершённых прогона. Это не router-side archive; постоянное серверное хранилище можно вынести в отдельный этап без смешивания с safety backups.</div>

    <div class="panel" style="margin-top:14px">
      <div class="panel-head">
        <div><h2>Auto-pick &amp; Recovery</h2><div class="section-kicker">Measured evidence only · no blind strategy ranking</div></div>
        <div class="toolbar"><button id="refreshAutoPick">Пересчитать</button></div>
      </div>
      <div class="setup-note">Auto-pick использует только завершённые <b>Test Lab</b> прогоны реального active runtime. Sandbox readiness не считается performance evidence и в выбор не попадает.</div>
      <div id="autoPickSummary" class="diagnostics" style="margin-top:10px"></div>
      <div class="setup-note warn" style="margin-top:10px"><b>Recovery Guard:</b> после Smart Apply повторяет последний измеренный baseline на тех же целях. При существенной регрессии может автоматически восстановить persistent pre-apply snapshots. Guard не запускается без подходящего baseline.</div>
      <div class="form-row" style="margin-top:10px">
        <div class="setup-label"><strong>Recovery Guard</strong>Порог задаётся падением reachability/stability относительно последнего baseline текущего config SHA.</div>
        <div class="form-control">
          <label class="config-check"><input id="recoveryGuardEnabled" type="checkbox"> Включить автоматический recovery после Smart Apply</label>
          <div style="display:flex;gap:8px;align-items:center;margin-top:7px"><span class="muted small">Порог</span><select id="recoveryGuardThreshold"><option value="15">15 п.п.</option><option value="20" selected>20 п.п.</option><option value="25">25 п.п.</option><option value="30">30 п.п.</option></select><span id="recoveryGuardState" class="config-badge">DISARMED</span></div>
        </div>
      </div>
      <div id="recoveryGuardLast" class="muted small" style="margin-top:8px">Recovery ещё не запускался.</div>
    </div>
</div>
</section>

<section id="view-logs" class="view">
  <article class="panel">
    <div class="panel-head"><div><h2>Логи</h2><div id="logMeta" class="section-kicker mono">—</div></div><button id="copyLog">Копировать raw</button></div>
    <div id="logSummary" class="log-summary"></div>
    <div class="logbar"><input id="logSearch" class="search" placeholder="Хост, клиент, протокол, событие…"><div class="toolbar"><button id="logAll" class="primary">Все</button><button id="logErrors">Ошибки</button><button id="logWarnings">Fail / warnings</button></div></div>
    <div id="log" class="log-stream">—</div>
  </article>
</section>

<section id="view-scripts" class="view">
  <div id="scriptWorkspace" class="script-layout rf-workspace">
    <article class="panel">
      <div class="panel-head"><div><h2>Lua-скрипты</h2><div id="scriptRoot" class="section-kicker mono">/opt/etc/nfqws2/lua</div></div></div>
      <input id="scriptSearch" class="search" placeholder="Поиск скрипта…">
      <div id="scriptList" class="script-list"><div class="empty">Открой вкладку — загрузим inventory.</div></div>
    </article>
    <article id="scriptPanel" class="panel script-panel">
      <div class="panel-head"><div><h2 id="selectedScriptTitle">Выберите скрипт</h2><div id="selectedScriptMeta" class="section-kicker mono">только просмотр</div></div><div class="toolbar"><button id="copyScript" disabled>Копировать</button><button id="fullscreenScript" disabled>На весь экран</button></div></div>
      <div id="scriptCode" class="script-reader-empty">Lua-скрипты nfqws2 доступны только для чтения.</div>
    </article>
  </div>
</section>

<section id="view-keenetic" class="view">
  <div class="backup-center-shell">
    <div class="backup-center-hero">
      <div>
        <div class="backup-center-title-row"><h2 style="margin:0">Keenetic Integration</h2><span class="config-badge active">READ-ONLY</span><span id="keeneticState" class="config-badge">IDLE</span></div>
        <div class="muted small" style="margin-top:5px">Сверяем RouterForge с реальным Keenetic/Entware окружением: ndmc, RCI, default route, интерфейсы и /opt. Эта вкладка ничего не меняет в KeeneticOS.</div>
      </div>
      <div class="toolbar"><button id="refreshKeenetic">Обновить</button></div>
    </div>
    <div class="backup-center-summary">
      <div class="backup-center-stat"><span>Platform</span><strong id="keeneticPlatform">—</strong></div>
      <div class="backup-center-stat"><span>NDMC</span><strong id="keeneticNdmc">—</strong></div>
      <div class="backup-center-stat"><span>RCI</span><strong id="keeneticRci">—</strong></div>
      <div class="backup-center-stat"><span>Entware</span><strong id="keeneticEntware">—</strong></div>
    </div>
    <div class="backup-center-layout">
      <article class="panel">
        <div class="panel-head"><div><h2>Роутер и сеть</h2><div id="keeneticModelMeta" class="section-kicker mono">—</div></div></div>
        <div id="keeneticNetwork" class="diagnostics"><div class="empty">Открой вкладку — загрузим read-only snapshot.</div></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Подключение NFQWS</h2><div class="section-kicker">ISP_INTERFACE ↔ default IPv4 route</div></div><div class="toolbar"><button id="keeneticUseDefault" disabled>Вставить default в Настройку</button></div></div>
        <div id="keeneticBinding" class="diagnostics"><div class="empty">Нет snapshot.</div></div>
      </article>
    </div>
    <div class="backup-center-layout" style="margin-top:14px">
      <article class="panel">
        <div class="panel-head"><div><h2>Entware /opt</h2><div class="section-kicker">mount + filesystem capacity</div></div></div>
        <div id="keeneticStorage" class="diagnostics"></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Сетевая политика Keenetic</h2><div class="section-kicker">GET /rci/show/ip/policy · raw policy data is not exposed</div></div></div>
        <div id="keeneticPolicy" class="diagnostics"></div>
      </article>
    </div>
    <div class="backup-center-layout" style="margin-top:14px">
      <article class="panel">
        <div class="panel-head"><div><h2>DPI Detector <span class="config-badge">ДОПОЛНИТЕЛЬНО</span></h2><div class="section-kicker">проверка установленного компонента</div></div><span id="dpiDetectorState" class="config-badge">IDLE</span></div>
        <div id="dpiDetectorStatus" class="diagnostics"><div class="empty">Открой вкладку — проверим только наличие локального dpi-detector.</div></div>
      </article>
    </div>
    <div class="backup-center-note">Этот раздел только показывает состояние системы. Изменения здесь не выполняются.</div>
  </div>
</section>

<section id="view-backups" class="view">
  <div class="backup-center-shell">
    <div class="backup-center-hero">
      <div>
        <div class="backup-center-title-row"><h2 style="margin:0">Backup Center</h2><span class="config-badge active">PERSISTENT</span><span id="backupCenterState" class="config-badge">IDLE</span></div>
        <div class="muted small" style="margin-top:5px">Постоянные snapshots до config/list/blob/library mutation. Inventory, diff и restore живут на роутере и переживают reboot.</div>
        <div id="backupCenterRoot" class="section-kicker mono" style="margin-top:7px">/opt/var/lib/routerforge/nfqws-manager/backups</div>
      </div>
      <div class="toolbar"><button id="refreshBackups">Обновить</button><button id="pruneBackups" class="danger">Очистить старые…</button></div>
    </div>
    <div class="backup-center-summary">
      <div class="backup-center-stat"><span>Persistent</span><strong id="backupPersistentCount">0</strong></div>
      <div class="backup-center-stat"><span>Changed / missing</span><strong id="backupChangedCount">0</strong></div>
      <div class="backup-center-stat"><span>Restorable</span><strong id="backupRestorableCount">0</strong></div>
      <div class="backup-center-stat"><span>Temporary safety</span><strong id="backupCount">0</strong></div>
    </div>
    <div class="backup-center-layout">
      <article class="panel">
        <div class="panel-head"><div><h2>История</h2><div class="section-kicker">до 64 persistent snapshots</div></div></div>
        <input id="backupCenterSearch" class="search" style="width:100%;max-width:none" placeholder="label / target / kind / sha…">
        <div id="backupCenterList" class="backup-center-list"><div class="empty">Открой вкладку — загрузим persistent inventory.</div></div>
      </article>
      <article class="panel">
        <div id="backupCenterDetail" class="backup-center-detail-empty">Выберите backup слева. Restore сначала создаёт новый persistent snapshot текущего target и только потом меняет файл.</div>
      </article>
    </div>
    <div class="backup-center-note"><b>Safety boundary:</b> config restore → verify SHA → controlled restart только если nfqws2 был запущен → verify running; при первой ошибке возвращается предыдущий файл и runtime. List/blob/library restore не перезапускает сервис. Старые временные safety-копии в <span class="mono">/tmp</span> остаются быстрым rollback-слоем и по-прежнему ограничены 8 файлами.</div>
  </div>
</section>
</main>
</div>

<div id="diffModal" class="modal"><div class="dialog"><div class="dialog-head"><strong>Изменения nfqws2.conf</strong><button id="closeDiff">Закрыть</button></div><pre id="diffBody" class="diff"></pre><div class="dialog-foot"><button id="diffCancel">Отмена</button><button id="diffApply" class="primary">Сохранить и применить</button></div></div></div>

<div id="renamePreviewModal" class="modal"><div class="dialog"><div class="dialog-head"><div><strong>Переименование списка · Preview</strong><div class="section-kicker">Только просмотр · без изменения файлов</div></div><button id="closeRenamePreview">Закрыть</button></div><div class="dialog-body"><pre id="renamePreviewBody" class="diff">—</pre></div><div class="dialog-foot"><span class="muted small">Запись только после явного подтверждения и проверки SHA.</span><button id="applyRenameList" class="primary" disabled>Применить</button><button id="renamePreviewClose">Закрыть</button></div></div></div>

<div id="createModal" class="modal"><div class="dialog small-dialog"><div class="dialog-head"><strong>Новый список</strong><button id="closeCreate">Закрыть</button></div><div class="dialog-body"><label class="muted small">Имя файла</label><div style="display:flex;align-items:center;gap:7px;margin-top:6px"><input id="newListName" class="search" style="flex:1;max-width:none" placeholder="youtube"><span class="mono muted">.list</span></div><div id="createError" class="bad-text small" style="margin-top:7px"></div></div><div class="dialog-foot"><button id="createCancel">Отмена</button><button id="createConfirm" class="primary">Создать</button></div></div></div>

<div id="configNameModal" class="modal"><div class="dialog small-dialog"><div class="dialog-head"><strong id="configNameTitle">Имя конфига</strong><button id="closeConfigName">Закрыть</button></div><div class="dialog-body"><label class="muted small">Имя файла</label><div style="display:flex;align-items:center;gap:7px;margin-top:6px"><input id="configNameInput" class="search" style="flex:1;max-width:none" placeholder="youtube"><span class="mono muted">.conf</span></div><div id="configNameError" class="bad-text small" style="margin-top:7px"></div></div><div class="dialog-foot"><button id="configNameCancel">Отмена</button><button id="configNameConfirm" class="primary">Продолжить</button></div></div></div>

<div id="checkModal" class="modal"><div class="dialog"><div class="dialog-head"><strong>Проверка доступности доменов</strong><button id="closeCheck">Закрыть</button></div><div class="dialog-body">
  <div class="domain-summary"><div class="domain-stat"><span>Всего</span><strong id="checkTotal">0</strong></div><div class="domain-stat"><span>Доступны</span><strong id="checkOk" class="good-text">0</strong></div><div class="domain-stat"><span>Недоступны</span><strong id="checkFail" class="bad-text">0</strong></div><div class="domain-stat"><span>Прогресс</span><strong id="checkProgress">0%</strong></div></div>
  <div class="progress"><span id="checkBar"></span></div><div id="checkResults" class="domain-results"></div>
</div><div class="dialog-foot"><button id="stopCheck">Остановить</button><button id="retryCheck" class="primary">Проверить снова</button></div></div></div>

<div id="entryInspectorModal" class="modal"><div class="dialog"><div class="dialog-head"><div><strong>Инспектор записи</strong><div id="entryInspectorScope" class="section-kicker mono">—</div></div><button id="closeEntryInspector">Закрыть</button></div><div class="dialog-body">
  <div class="inspector-source"><div><div id="entryInspectorValue" class="inspector-entry">—</div><div id="entryInspectorMeta" class="inspector-source-meta">—</div></div><span id="entryInspectorRole" class="config-badge">—</span></div>
  <div class="inspector-summary"><div class="inspector-stat"><span>Связей</span><strong id="entryInspectorTotal">0</strong></div><div class="inspector-stat"><span>Конфликтов</span><strong id="entryInspectorConflicts" class="bad-text">0</strong></div><div class="inspector-stat"><span>Списков</span><strong id="entryInspectorFiles">0</strong></div></div>
  <div id="entryInspectorProgress" class="muted small" style="margin-bottom:8px"></div><div id="entryInspectorResults" class="inspector-results"></div>
</div><div class="dialog-foot"><span class="muted small">Read-only: анализируется только запись под курсором.</span><button id="rerunEntryInspector" class="primary">Пересчитать</button></div></div></div>

<div id="staticPathModal" class="modal"><div class="dialog"><div class="dialog-head"><div><strong>Effective Static Path</strong><div id="staticPathScope" class="section-kicker mono">—</div></div><button id="closeStaticPath">Закрыть</button></div><div class="dialog-body">
  <div class="inspector-source"><div><div id="staticPathValue" class="inspector-entry">—</div><div id="staticPathMeta" class="inspector-source-meta">—</div></div><span id="staticPathSourceRole" class="config-badge">—</span></div>
  <div id="staticPathVerdict" class="static-path-verdict"><div><strong>—</strong><div class="static-path-reason">Поставь курсор на домен, IP или CIDR.</div></div><span id="staticPathMode" class="static-path-mode">MODE ?</span></div>
  <div id="staticPathProgress" class="muted small" style="margin-bottom:8px"></div><div id="staticPathSteps" class="static-path-steps"></div>
  <div class="static-path-footnote">Статический разбор конфигурации и list-файлов. Он не утверждает фактический packet path, состояние NFQUEUE, DNS, conntrack или результат DPI-обхода.</div>
</div><div class="dialog-foot"><span class="muted small">Read-only · выбранная несохранённая строка учитывается.</span><button id="rerunStaticPath" class="primary">Пересчитать</button></div></div></div>

<div id="analysisModal" class="modal"><div class="dialog"><div class="dialog-head"><div><strong>Пересечения списков</strong><div id="analysisScope" class="section-kicker mono">—</div></div><button id="closeAnalysis">Закрыть</button></div><div class="dialog-body">
  <div class="analysis-summary"><div class="analysis-stat"><span>Найдено</span><strong id="analysisTotal">0</strong></div><div class="analysis-stat"><span>Конфликты</span><strong id="analysisConflicts" class="bad-text">0</strong></div><div class="analysis-stat"><span>Пересечения</span><strong id="analysisWarnings" class="warn-text">0</strong></div><div class="analysis-stat"><span>Списки</span><strong id="analysisFiles">0</strong></div></div>
  <div class="analysis-filters"><button id="analysisAll" class="active">Все</button><button id="analysisConflict">Конфликты</button><button id="analysisDomain">Домены</button><button id="analysisNetwork">IP / CIDR</button></div>
  <div class="analysis-pairbar"><select id="analysisPeer" class="search analysis-peer"></select><button id="openAnalysisPeer">Открыть правый список</button></div>
  <div id="analysisProgress" class="muted small" style="margin-bottom:8px"></div>
  <div id="analysisCommander" class="analysis-commander">
    <section class="analysis-pane"><div class="analysis-pane-head"><div id="analysisLeftTitle" class="analysis-pane-title">—</div><div id="analysisLeftCount" class="analysis-pane-count">0 строк</div></div><div id="analysisLeftLines" class="analysis-pane-lines"></div></section>
    <section class="analysis-pane"><div class="analysis-pane-head"><div id="analysisRightTitle" class="analysis-pane-title">—</div><div id="analysisRightCount" class="analysis-pane-count">0 строк</div></div><div id="analysisRightLines" class="analysis-pane-lines"></div></section>
  </div>
  <div id="analysisPairMeta" class="analysis-pair-meta"></div>
  <div id="analysisCleanup" class="analysis-cleanup"><div class="analysis-cleanup-title">Из какого файла удалить все найденные пересекающиеся строки в текущей паре?</div><div class="analysis-cleanup-actions"><button id="cleanupLeft" class="danger">Удалить слева</button><button id="cleanupRight" class="danger">Удалить справа</button><button id="cleanupCancel">Отмена</button></div><div id="analysisCleanupNote" class="analysis-cleanup-note">Перед изменением RouterForge создаст safety backup выбранного list-файла.</div></div>
</div><div class="dialog-foot"><span class="muted small">Показываются только строки, участвующие в пересечениях.</span><div class="toolbar"><button id="cleanupAnalysis" class="danger">Очистить пересечения</button><button id="rerunAnalysis" class="primary">Пересчитать</button></div></div></div></div>


<div id="smartApplyModal" class="modal"><div class="dialog"><div class="dialog-head"><div><strong>Smart Apply / Semantic Merge</strong><div id="smartApplySource" class="section-kicker mono">—</div></div><button id="closeSmartApply">Закрыть</button></div><div class="dialog-body">
  <div id="smartApplyPlan" class="smart-apply-plan"><div class="empty">План ещё не построен.</div></div>
</div><div class="dialog-foot"><span class="muted small">Одна транзакция: dependencies → config → restart(if running) → verify. Любой FAIL запускает rollback.</span><div class="toolbar"><button id="refreshSmartApply">Пересчитать</button><button id="applySmartApply" class="primary" disabled>Применить план</button></div></div></div></div>
