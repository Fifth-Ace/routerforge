<script>
  import {onMount} from 'svelte';
  import {draft,simple,shell,loadConfig,feedback,nativeDNS,choices,groups,policyName,policyRoute,changeDNS} from './DnsRuntimeStore.svelte.js';
  import {scanNative,stopRuntimeRequests} from './DnsRuntimeCommands.js';
  import {apiRequest} from './DnsRuntimeApi.js';
  import {invalidateConfigFlows} from './DnsRuntimeConfigLogic.js';
  import {shadow} from './DnsRuntimeShadowState.svelte.js';
  import DnsRuntimeConfigActions from './DnsRuntimeConfigActions.svelte';
  import DnsRuntimeDiagnostics from './DnsRuntimeDiagnostics.svelte';
  import DnsRuntimeGroups from './DnsRuntimeGroups.svelte';
  import DnsRuntimeLists from './DnsRuntimeLists.svelte';
  import DnsRuntimeLite from './DnsRuntimeLite.svelte';
  import DnsRuntimeManagedTrial from './DnsRuntimeManagedTrial.svelte';
  import DnsRuntimeMode from './DnsRuntimeMode.svelte';
  import DnsRuntimeP10 from './DnsRuntimeP10.svelte';
  import DnsRuntimeP88Actions from './DnsRuntimeP88Actions.svelte';
  import DnsRuntimeP88Views from './DnsRuntimeP88Views.svelte';
  import DnsRuntimeP91 from './DnsRuntimeP91.svelte';
  import DnsRuntimeP92 from './DnsRuntimeP92.svelte';
  import DnsRuntimePolicyForms from './DnsRuntimePolicyForms.svelte';
  import DnsRuntimeReadOnly from './DnsRuntimeReadOnly.svelte';
  import DnsRuntimeShadowControls from './DnsRuntimeShadowControls.svelte';
  import DnsRuntimeShadowStatus from './DnsRuntimeShadowStatus.svelte';
  import DnsRuntimeTables from './DnsRuntimeTables.svelte';
  import DnsRuntimeTools from './DnsRuntimeTools.svelte';
  const tabs=['overview','policies','testing','keenetic','diagnostics'];
  const urlTab=typeof window!=='undefined'?new URLSearchParams(window.location.search).get('view'):null;
  $effect(()=>{if(typeof document!=='undefined')document.documentElement.dataset.section=shell.tab;});
  if(tabs.includes(urlTab))shell.tab=urlTab;
  function activate(tab){if(tabs.includes(tab))shell.tab=tab;}
  function tabKey(e,tab){
    if(!['ArrowRight','ArrowLeft','Home','End'].includes(e.key))return;
    e.preventDefault();const n=tabs.indexOf(tab);
    const next=e.key==='Home'?0:e.key==='End'?tabs.length-1:
      e.key==='ArrowRight'?(n+1)%tabs.length:(n+tabs.length-1)%tabs.length;
    activate(tabs[next]);document.getElementById('dr-tab-'+tabs[next])?.focus();
  }
  function shadowActive(){return Boolean(shadow.result?.lifecycle?.active||shadow.result?.active||shadow.result?.lifecycle?.state==='running');}
  function ruleSummary(){const list=groups();const count=list.reduce((n,g)=>n+g.rules.length,0);const other=(draft.config.rules||[]).length-count;
    return count+' доменов · '+list.length+' DNS'+(other?' · '+other+' других политик':'');}
  function feedbackClass(s){return /Ошибка|HTTP 5|HTTP 4|FAIL|конфликт|запрещено/i.test(s)?'feedback bad':
    /не сохран|сначала|отмен|внимание/i.test(s)?'feedback warn':
    /загружена|Сохранено|подтвержден|готов/i.test(s)?'feedback good':'feedback neutral';}
  onMount(()=>{
    // All startup HTTP actions are read-only GET. No start/apply/save/prepare/trial.
    const theme=setupRuntimeTheme();
    const controller=new AbortController();
    // A response arriving after unmount must not resurrect the shared draft.
    void loadConfig(apiRequest,controller.signal).catch(e=>{
      if(!controller.signal.aborted)feedback('Ошибка загрузки: '+String(e?.message||e));
    });
    void scanNative(controller.signal);
    return ()=>{controller.abort();stopRuntimeRequests();invalidateConfigFlows();theme();};
  });
  import {setupRuntimeTheme} from './DnsRuntimeTheme.js';
</script>

<main class={shell.mode==='advanced'?'dr-page dr-mode-advanced':'dr-page'} id="dnsRouterWorkspace">
  <header class="hero">
    <div class="hero-copy">
      <h1>DNS Router</h1>
      <p>Управление DNS-пулами, доменными маршрутами и безопасным тестовым DNS-сервисом.</p>
    </div>
    <div class="hero-actions">
      <div id="rf-dns-runtime-svelte-mode" class="dr-mode-toggle" aria-label="Режим интерфейса"><DnsRuntimeMode /></div>
      <span class={shadowActive()?'state good':'state neutral'} id="shadowBadge">{shell.lite?'DEV · САМОСТОЯТЕЛЬНЫЙ DNS':shadowActive()?'SHADOW · АКТИВЕН':'SHADOW · ОСТАНОВЛЕН'}</span>
      <span class="state safe" id="drP10NativeBadge">{shell.lite?'ПРОФИЛЬ KEENETIC / ПОДКЛЮЧЕНИЕ ОТДЕЛЬНО':'ШТАТНЫЙ DNS НЕ ЗАТРОНУТ'}</span>
      <div id="rf-dns-runtime-svelte-reload"><DnsRuntimeConfigActions kind="reload" /></div>
    </div>
  </header>

  <div class={feedbackClass(draft.feedback)} id="feedback" role="status" aria-live="polite">{draft.feedback||'Чтение конфигурации…'}</div>

  <div id="rf-dns-runtime-svelte-lite"><DnsRuntimeLite /></div>
  <section class="dr-simple-workspace" id="drSimpleWorkspace" hidden={shell.mode!=='simple'} aria-label="Простое управление DNS">
    <div class="dr-simple-intro">
      <div><span class="section-kicker">ПРОСТОЕ УПРАВЛЕНИЕ</span><h2>Какой DNS использовать для доменов?</h2><p>Выбери политику Keenetic, DNS-сервер и вставь домены. Ручные DoT/DoH хранятся отдельно в DNS Router и не назначаются политике Keenetic автоматически.</p></div>
      <div id="rf-dns-runtime-svelte-p88-scan"><DnsRuntimeP88Actions scope="scan" /></div>
    </div>
    <div class="dr-simple-layout">
      <aside class="dr-simple-side" aria-label="Политики Keenetic">
        <h3>Политики Keenetic</h3><p class="dr-simple-help">Выбери профиль для DNS-правил · горизонтальная прокрутка</p>
        <div id="rf-dns-runtime-svelte-p88-policies"><DnsRuntimeP88Views scope="policies" /></div>
      </aside>
      <div class="dr-simple-main">
        <section class="dr-simple-panel">
          <h3>2. DNS-сервер</h3>
          <p id="drSimpleRoute" class="dr-simple-context">{simple.policy?policyName(simple.policy)+' · '+policyRoute(simple.policy):'Политика не выбрана'}</p>
          <label class="dr-simple-field"><span>DNS из выбранной политики</span><select id="drSimpleDNS" aria-label="DNS-сервер выбранной политики" bind:value={simple.selected} onchange={e=>changeDNS(e.currentTarget.value)} disabled={!choices().length}>
            {#each choices() as item (item.value)}<option value={item.value}>{item.label}</option>{/each}
          </select></label>
          <p class="dr-simple-help" id="drSimpleDNSNote">{nativeDNS().length?'DNS Keenetic: '+nativeDNS().length+' · доступно для выбора: '+choices().length+'.':'В текущей политике DNS endpoints не обнаружены. Проверь настройки Keenetic.'}</p>
          <div class="dr-p89-inventory" aria-label="Все обнаруженные DNS-серверы">
            <h4>Все DNS-серверы выбранной политики</h4>
            <div id="rf-dns-runtime-svelte-p88-inventory"><DnsRuntimeP88Views scope="inventory" /></div>
            <p class="dr-simple-help">Серверы из Keenetic. Можно выбрать существующий или добавить свой.</p>
          </div>
          <div id="rf-dns-runtime-svelte-p91"><DnsRuntimeP91 /></div>
          <div id="rf-dns-runtime-svelte-p88-probe"><DnsRuntimeP88Actions scope="probe" /></div>
        </section>
        <section class="dr-simple-panel">
          <h3>Редактор доменов выбранного DNS</h3><p class="dr-simple-help">По одному на строку или через запятую. Поддомены включаются автоматически.</p><p id="drP10EditingLabel" class="dr-ux-editing-hint">{simple.notice||'Добавление доменов к выбранному DNS. Другие DNS не изменяются.'}</p>
          <label class="dr-simple-field"><span>Какие домены направлять на этот DNS?</span><textarea id="drSimpleDomains" rows="12" placeholder="youtube.com&#10;googlevideo.com&#10;ytimg.com" bind:value={simple.domains}></textarea></label>
          <div id="rf-dns-runtime-svelte-p92"><DnsRuntimeP92 /></div>
          <div class="dr-p92-footer">
            <div class="dr-simple-actions"><span id="rf-dns-runtime-svelte-p88-draft" style="display:contents"><DnsRuntimeP88Actions scope="draft" /></span><div id="rf-dns-runtime-svelte-p10" class="dr-svelte-p10-actions" style="display:contents"><DnsRuntimeP10 /></div></div>
          </div>
          <p id="drP10Runtime" role="status" aria-live="polite">{simple.notice||'Проверяем фактическую активацию DNS…'}</p>
          <p class="dr-simple-state" id="drSimpleState" role="status" aria-live="polite">{simple.status||(draft.dirty?'Есть несохранённые изменения. На DNS Keenetic они не влияют.':'Конфигурация сохранена. Маршрутизация клиентского DNS автоматически НЕ включалась.')}</p>
        </section>
        <section class="dr-simple-panel">
          <div class="dr-simple-list-head"><h3>DNS-сервер → привязанный список доменов</h3><span id="drSimpleRuleCount">{ruleSummary()}</span></div>
          <div id="rf-dns-runtime-svelte-p88-rules"><DnsRuntimeP88Views scope="rules" /></div>
          <p class="dr-simple-help">Правила сохранены в DNS Router, но это ещё не означает, что клиенты Keenetic используют их. Активное подключение DNS — отдельный этап.</p>
        </section>
      </div>
    </div>
    <p class="dr-simple-bottom">Нужны DoT/DoH вручную, списки по ссылке, приоритеты или тестовая активация? <span id="rf-dns-runtime-svelte-p88-advanced"><DnsRuntimeP88Actions scope="advanced" /></span></p>
  </section>

  <div id="drAdvancedWorkspace" hidden={shell.mode!=='advanced'}>
  <section class="metrics" aria-label="Сводка DNS Router">
    <article class="metric"><span>UPSTREAM-ПУЛЫ</span><strong id="metricUpstreams">{draft.config.upstreams?.length||0}</strong><small>DNS-серверы и резервные цепочки</small></article>
    <article class="metric"><span>ДОМЕННЫЕ ПРАВИЛА</span><strong id="metricRules">{draft.config.rules?.length||0}</strong><small>Точные, суффиксы, поддомены</small></article>
    <article class="metric"><span>ТЕСТОВЫЙ DNS</span><strong id="metricRuntime">{shell.lite?(shell.liteActive?'АКТИВЕН':'ВЫКЛ.'):(shadowActive()?'АКТИВЕН':'ВЫКЛ.')}</strong><small>Ручной shadow-forwarder</small></article>
    <article class="metric"><span>КОНФИГУРАЦИЯ</span><strong id="metricConfig">{draft.dirty?'ЧЕРНОВИК':draft.loaded?'СОХРАНЕНО':'—'}</strong><small>Черновик и ревизия CAS</small></article>
  </section>

  <div class="tabs" role="tablist" aria-label="Разделы DNS Router">
    <button type="button" class={shell.tab==='overview'?'tab active':'tab'} data-tab="overview" id="dr-tab-overview" role="tab" tabindex={shell.tab==='overview'?0:-1} aria-controls="dr-panel-overview" aria-selected={shell.tab==='overview'} onclick={()=>activate('overview')} onkeydown={e=>tabKey(e,'overview')}>Обзор</button>
    <button type="button" class={shell.tab==='policies'?'tab active':'tab'} data-tab="policies" id="dr-tab-policies" role="tab" tabindex={shell.tab==='policies'?0:-1} aria-controls="dr-panel-policies" aria-selected={shell.tab==='policies'} onclick={()=>activate('policies')} onkeydown={e=>tabKey(e,'policies')}>Списки и правила</button>
    <button type="button" class={shell.tab==='testing'?'tab active':'tab'} data-tab="testing" id="dr-tab-testing" role="tab" tabindex={shell.tab==='testing'?0:-1} aria-controls="dr-panel-testing" aria-selected={shell.tab==='testing'} onclick={()=>activate('testing')} onkeydown={e=>tabKey(e,'testing')}>Проверка</button>
    <button type="button" class={shell.tab==='keenetic'?'tab active':'tab'} data-tab="keenetic" id="dr-tab-keenetic" role="tab" tabindex={shell.tab==='keenetic'?0:-1} aria-controls="dr-panel-keenetic" aria-selected={shell.tab==='keenetic'} onclick={()=>activate('keenetic')} onkeydown={e=>tabKey(e,'keenetic')}>Keenetic</button>
    <button type="button" class={shell.tab==='diagnostics'?'tab active':'tab'} data-tab="diagnostics" id="dr-tab-diagnostics" role="tab" tabindex={shell.tab==='diagnostics'?0:-1} aria-controls="dr-panel-diagnostics" aria-selected={shell.tab==='diagnostics'} onclick={()=>activate('diagnostics')} onkeydown={e=>tabKey(e,'diagnostics')}>Диагностика</button>
  </div>

  <div class={shell.tab==='overview'?'tab-page active':'tab-page'} data-page="overview" aria-hidden={shell.tab!=='overview'} id="dr-panel-overview" role="tabpanel" aria-labelledby="dr-tab-overview" tabindex="0">
    <section class="panel">
      <div class="panel-head">
        <div><span class="section-kicker">УПРАВЛЕНИЕ / ТЕСТОВЫЙ РЕЖИМ</span><h2>Shadow DNS</h2><p>Ручной listener на loopback или явно разрешённом LAN. Никакой подмены системного DNS.</p></div>
        <span class="state safe">OPT-IN</span>
      </div>
      <div class="panel-content">
        <div id="rf-dns-runtime-svelte-shadow-controls"><DnsRuntimeShadowControls /></div>
        <div class="shadow-summary" role="note">
          <span class="shadow-summary-icon" aria-hidden="true">◎</span>
          <div><strong>Изолированный тестовый DNS</strong><p>Не изменяет системный DNS Keenetic. Действия выполняются только по явной команде.</p></div>
        </div>
        <details class="debug-details">
          <summary>Техническое состояние и данные API</summary>
          <div id="rf-dns-runtime-svelte-shadow-status"><DnsRuntimeShadowStatus /></div>
        </details>
      </div>
    </section>
    <div class="two-column">
      <section class="panel">
        <div class="panel-head"><div><span class="section-kicker">КОНФИГУРАЦИЯ</span><h2>Как работают правила</h2></div></div>
        <div class="steps">
          <div class="step"><span>01</span><div><strong>Создай upstream-пулы</strong><p>Основные транспорты — DoT, DoH и DoQ. Открытые DNS доступны в расширенных настройках.</p></div></div>
          <div class="step"><span>02</span><div><strong>Задай доменные маршруты</strong><p>Правила для домена, суффикса или поддоменов с приоритетами.</p></div></div>
          <div class="step"><span>03</span><div><strong>Проверь и сохрани</strong><p>Сначала тест решения, затем явное применение к shadow.</p></div></div>
        </div>
        <div class="panel-actions"><button type="button" class="button" data-go="policies" onclick={()=>activate('policies')}>Перейти к политикам</button><button type="button" class="button" data-go="testing" onclick={()=>activate('testing')}>Проверить домен</button></div>
      </section>
      <section class="panel">
        <div class="panel-head"><div><span class="section-kicker">ГРАНИЦЫ ВОЗДЕЙСТВИЯ</span><h2>Что остаётся неизменным</h2></div></div>
        <div class="guard-list">
          <div><span class="guard-check">✓</span><span>Порт <code>53</code> остаётся у системного DNS Keenetic.</span></div>
          <div><span class="guard-check">✓</span><span>Настройки <code>ndnproxy</code> и DNS Proxy не перехватываются автоматически.</span></div>
          <div><span class="guard-check">✓</span><span>Импорт, планы и диагностика не меняют DNS-трафик сами по себе.</span></div>
          <div><span class="guard-warn">!</span><span>Временная DNS-транзакция требует отдельного разрешения и подтверждения.</span></div>
        </div>
        <div class="panel-actions"><button type="button" class="button" data-go="diagnostics" onclick={()=>activate('diagnostics')}>Диагностика оборудования</button></div>
      </section>
    </div>
  </div>

  <div class={shell.tab==='policies'?'tab-page active':'tab-page'} data-page="policies" aria-hidden={shell.tab!=='policies'} id="dr-panel-policies" role="tabpanel" aria-labelledby="dr-tab-policies" tabindex="0">
    <section class="panel dr-p6-panel" id="drGroupPanel">
      <div class="panel-head"><div><span class="section-kicker">DNS POLICY / STRICT</span><h2>Строгие группы DNS</h2><p>Домен из группы использует только перечисленные DNS-серверы. При отказе всех — ошибка, без системного DNS fallback.</p></div><span class="state safe">FAIL-CLOSED</span></div>
      <div id="rf-dns-runtime-svelte-groups"><DnsRuntimeGroups /></div>
    </section>

    <section class="panel dr-p5-panel" id="drListPanel">
      <div class="panel-head"><div><span class="section-kicker">ПОЛИТИКИ / СПИСКИ ДОМЕНОВ</span><h2>Мои списки доменов</h2><p>TXT, текст или HTTPS-ссылка. Предпросмотр и применение к черновику без запуска DNS.</p></div><span class="state safe">БЕЗ ИЗМЕНЕНИЯ DNS</span></div>
      <div id="rf-dns-runtime-svelte-lists"><DnsRuntimeLists /></div>
    </section>

    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">ПОЛИТИКИ / 01</span><h2>Upstream-пулы</h2><p>DNS-серверы, транспорт и явно заданные резервные цепочки.</p></div></div>
      <div id="rf-dns-runtime-svelte-upstream-form"><DnsRuntimePolicyForms scope="upstream" /></div>
      <!-- R07: Native inventory and form are Svelte-owned (DnsRuntimePolicyForms). -->
      <p class="boundary-copy">Keenetic Native: транспорт native, адрес вида <code>native://System/./9.9.9.9</code> или <code>native://Policy0/ru/common.dot.dns.yandex.net</code>. Выбирается текущий loopback-порт из ndnproxy, но только с явным флагом <code>--native-bridge-experimental</code> при старте DNS Router. Шифрование за пределами локального stubby этим режимом не доказывается, поэтому encrypted-only группы его не принимают.</p>
      <p class="boundary-copy">Проверка Native Bridge подтверждает только совпадение селектора и runtime-флаг. Для PolicyN может отсутствовать default route — тогда DNS-запрос завершится отказом. Пример: Policy0 без интернета корректно остаётся заблокированной; RouterForge не меняет её маршруты.</p>
      <p class="boundary-copy">Новые зашифрованные группы не допускают fallback к UDP/TCP. Для уже сохранённых групп совместимость сохраняется.</p>
      <div class="table-wrap" id="upstreamList"><DnsRuntimeTables target="upstreamList" /></div>
      <div id="rf-dns-runtime-svelte-upstreams"><DnsRuntimeReadOnly kind="upstreams" /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">ПОЛИТИКИ / 02</span><h2>Доменные правила</h2><p>Правила применяются по приоритету к выбранным upstream-пулам.</p></div></div>
      <div id="rf-dns-runtime-svelte-rule-form"><DnsRuntimePolicyForms scope="rule" /></div>
      <div class="dr-p5-content"><label><span>Поиск по доменам и правилам</span><input id="drRuleSearch" type="search" placeholder="Найти домен, ID или DNS-пул" bind:value={draft.ruleSearch}></label></div>
      <div class="table-wrap" id="ruleList"><DnsRuntimeTables target="ruleList" /></div>
    </section>
  </div>

  <div class={shell.tab==='testing'?'tab-page active':'tab-page'} data-page="testing" aria-hidden={shell.tab!=='testing'} id="dr-panel-testing" role="tabpanel" aria-labelledby="dr-tab-testing" tabindex="0">
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">СИМУЛЯТОР</span><h2>Проверить маршрутизацию домена</h2><p>Проверка использует сохранённую конфигурацию и не выполняет сетевой DNS-запрос.</p></div></div>
      <div id="rf-dns-runtime-svelte-evaluate"><DnsRuntimeTools kind="evaluate" /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">ИМПОРТ</span><h2>Простой импорт (совместимость)</h2><p>Для файлов, URL и именованных списков используй вкладку «Списки и правила».</p></div></div>
      <div id="rf-dns-runtime-svelte-import"><DnsRuntimeConfigActions kind="import" /></div>
    </section>
  </div>

  <div class={shell.tab==='keenetic'?'tab-page active':'tab-page'} data-page="keenetic" aria-hidden={shell.tab!=='keenetic'} id="dr-panel-keenetic" role="tabpanel" aria-labelledby="dr-tab-keenetic" tabindex="0">
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">ИНТЕГРАЦИЯ / ЧЕРНОВИК</span><h2>Настройки listener</h2><p>Сохранение адреса и ACL не запускает DNS и не меняет настройки Keenetic.</p></div><span class="state safe">БЕЗ АКТИВАЦИИ</span></div>
      <div id="rf-dns-runtime-svelte-listener-form"><DnsRuntimePolicyForms scope="listener" /></div>
      <div id="rf-dns-runtime-svelte-listener"><DnsRuntimeReadOnly kind="listener" /></div>
      <p class="boundary-copy">P7 показывает, какие правила реально доступны через штатные зоны. Чтение конфигурации не применяет DNS, не запускает listener и не включает перехват.</p>
      <div id="rf-dns-runtime-svelte-ingress"><DnsRuntimeReadOnly kind="ingress" /></div>
    </section>
    <section class="panel risk-panel">
      <div class="panel-head"><div><span class="section-kicker">DPR2-N / ЭКСПЕРИМЕНТ ТОЛЬКО DSTEST</span><h2>Временная транзакция DNS-зон</h2><p>Требует отдельного серверного флага; изменяет тестовые зоны на ограниченное время и выполняет откат.</p></div><span class="state caution">ПОВЫШЕННЫЙ РИСК</span></div>
      <div class="boundary-copy">Перед испытанием необходимы сохранённая конфигурация, LAN-listener, отсутствие конфликтов и явное подтверждение. Не использовать на production.</div>
      <div id="rf-dns-runtime-svelte-managed"><DnsRuntimeManagedTrial /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">DPR2-O1 / ТОЛЬКО ЧТЕНИЕ</span><h2>Реестр владения DNS-зонами</h2><p>Сверка собственных и чужих записей. Не удаляет и не принимает чужие настройки.</p></div></div>
      <div id="rf-dns-runtime-svelte-ownership"><DnsRuntimeReadOnly kind="ownership" /></div>
    </section>
  </div>

  <div class={shell.tab==='diagnostics'?'tab-page active':'tab-page'} data-page="diagnostics" aria-hidden={shell.tab!=='diagnostics'} id="dr-panel-diagnostics" role="tabpanel" aria-labelledby="dr-tab-diagnostics" tabindex="0">
    <div id="rf-dns-runtime-svelte-diagnostics"><DnsRuntimeDiagnostics /></div>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">ДИАГНОСТИКА / READ-ONLY</span><h2>Среда и ресурсы</h2><p>Пассивные показатели и планы интеграции без изменения настроек Keenetic.</p></div></div>
      <div id="rf-dns-runtime-svelte-discovery"><DnsRuntimeReadOnly kind="discovery" /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">DPR2-G / АППАРАТНАЯ СОВМЕСТИМОСТЬ</span><h2>Анализ hardware evidence</h2><p>Вставь отчёт read-only диагностики DSTest. Данные не сохраняются на маршрутизаторе.</p></div></div>
      <div id="rf-dns-runtime-svelte-hardware"><DnsRuntimeReadOnly kind="hardware" /></div>
    </section>
    <div class="boundary-note">Доступные экспериментальные операции запускаются только через отдельные capability-флаги, включая <code>--shadow-lan-experimental</code> для LAN-listener. DNS Router не занимает порт 53 и не подменяет ndnproxy. DoT/DoH проверяют TLS-сертификаты и не используют неявный fallback.</div>
  </div>

  <footer class="save-bar">
    <div class="save-meta"><strong id="metricFooter">{draft.dirty?'Есть несохранённые изменения':'Настройки DNS Router'}</strong><span id="revision">{(draft.dirty?'● Не сохранено · ':'')+draft.revision.substring(0,23)}</span></div>
    <div class="save-actions"><div id="rf-dns-runtime-svelte-export"><DnsRuntimeTools kind="export" /></div><div id="rf-dns-runtime-svelte-save"><DnsRuntimeConfigActions kind="save" /></div></div>
  </footer>
  </div><!-- drAdvancedWorkspace: original controls preserved -->
</main>
