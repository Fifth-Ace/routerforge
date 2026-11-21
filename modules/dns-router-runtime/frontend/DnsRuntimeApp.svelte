<script>
  import { rfUi } from './rf-localization.js';
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
    return count+rfUi(' доменов · ',"domains ·")+list.length+' DNS'+(other?' · '+other+' других политик':'');}
  function feedbackClass(s){return /Ошибка|HTTP 5|HTTP 4|FAIL|конфликт|запрещено/i.test(s)?'feedback bad':
    /не сохран|сначала|отмен|внимание/i.test(s)?'feedback warn':
    /загружена|Сохранено|подтвержден|готов/i.test(s)?'feedback good':'feedback neutral';}
  onMount(()=>{
    // All startup HTTP actions are read-only GET. No start/apply/save/prepare/trial.
    const theme=setupRuntimeTheme();
    const controller=new AbortController();
    // A response arriving after unmount must not resurrect the shared draft.
    void loadConfig(apiRequest,controller.signal).catch(e=>{
      if(!controller.signal.aborted)feedback(rfUi('Ошибка загрузки: ',"Load error:")+String(e?.message||e));
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
      <p>{rfUi("Управление DNS-пулами, доменными маршрутами и безопасным тестовым DNS-сервисом.","Manage DNS pools, domain routes, and a safe test DNS service.")}</p>
    </div>
    <div class="hero-actions">
      <div id="rf-dns-runtime-svelte-mode" class="dr-mode-toggle" aria-label={rfUi("Режим интерфейса","Interface mode")}><DnsRuntimeMode /></div>
      <span class={shadowActive()?'state good':'state neutral'} id="shadowBadge">{shell.lite?'DEV · САМОСТОЯТЕЛЬНЫЙ DNS':shadowActive()?'SHADOW · АКТИВЕН':'SHADOW · ОСТАНОВЛЕН'}</span>
      <span class="state safe" id="drP10NativeBadge">{shell.lite?'ПРОФИЛЬ KEENETIC / ПОДКЛЮЧЕНИЕ ОТДЕЛЬНО':'ШТАТНЫЙ DNS НЕ ЗАТРОНУТ'}</span>
      <div id="rf-dns-runtime-svelte-reload"><DnsRuntimeConfigActions kind="reload" /></div>
    </div>
  </header>

  <div class={feedbackClass(draft.feedback)} id="feedback" role="status" aria-live="polite">{draft.feedback||'Чтение конфигурации…'}</div>

  <div id="rf-dns-runtime-svelte-lite"><DnsRuntimeLite /></div>
  <section class="dr-simple-workspace" id="drSimpleWorkspace" hidden={shell.mode!=='simple'} aria-label={rfUi("Простое управление DNS","Basic DNS management")}>
    <div class="dr-simple-intro">
      <div><span class="section-kicker">{rfUi("ПРОСТОЕ УПРАВЛЕНИЕ","SIMPLE MANAGEMENT")}</span><h2>{rfUi("Какой DNS использовать для доменов?","Which DNS should handle these domains?")}</h2><p>{rfUi("Выбери политику Keenetic, DNS-сервер и вставь домены. Ручные DoT/DoH хранятся отдельно в DNS Router и не назначаются политике Keenetic автоматически.","Select a Keenetic policy and DNS server, then enter domains. Manually configured DoT/DoH servers are stored separately and are not assigned to Keenetic policies automatically.")}</p></div>
      <div id="rf-dns-runtime-svelte-p88-scan"><DnsRuntimeP88Actions scope="scan" /></div>
    </div>
    <div class="dr-simple-layout">
      <aside class="dr-simple-side" aria-label={rfUi("Политики Keenetic","Keenetic policies")}>
        <h3>{rfUi("Политики Keenetic","Keenetic policies")}</h3><p class="dr-simple-help">{rfUi("Выбери профиль для DNS-правил · горизонтальная прокрутка","Select a profile for DNS rules · scroll horizontally")}</p>
        <div id="rf-dns-runtime-svelte-p88-policies"><DnsRuntimeP88Views scope="policies" /></div>
      </aside>
      <div class="dr-simple-main">
        <section class="dr-simple-panel">
          <h3>{rfUi("2. DNS-сервер","2. DNS server")}</h3>
          <p id="drSimpleRoute" class="dr-simple-context">{simple.policy?policyName(simple.policy)+' · '+policyRoute(simple.policy):'Политика не выбрана'}</p>
          <label class="dr-simple-field"><span>{rfUi("DNS из выбранной политики","DNS from selected policy")}</span><select id="drSimpleDNS" aria-label={rfUi("DNS-сервер выбранной политики","DNS server for selected policy")} bind:value={simple.selected} onchange={e=>changeDNS(e.currentTarget.value)} disabled={!choices().length}>
            {#each choices() as item (item.value)}<option value={item.value}>{item.label}</option>{/each}
          </select></label>
          <p class="dr-simple-help" id="drSimpleDNSNote">{nativeDNS().length?'DNS Keenetic: '+nativeDNS().length+' · доступно для выбора: '+choices().length+'.':'В текущей политике DNS endpoints не обнаружены. Проверь настройки Keenetic.'}</p>
          <div class="dr-p89-inventory" aria-label={rfUi("Все обнаруженные DNS-серверы","All detected DNS servers")}>
            <h4>{rfUi("Все DNS-серверы выбранной политики","All DNS servers in selected policy")}</h4>
            <div id="rf-dns-runtime-svelte-p88-inventory"><DnsRuntimeP88Views scope="inventory" /></div>
            <p class="dr-simple-help">{rfUi("Серверы из Keenetic. Можно выбрать существующий или добавить свой.","Keenetic servers. Choose an existing server or add your own.")}</p>
          </div>
          <div id="rf-dns-runtime-svelte-p91"><DnsRuntimeP91 /></div>
          <div id="rf-dns-runtime-svelte-p88-probe"><DnsRuntimeP88Actions scope="probe" /></div>
        </section>
        <section class="dr-simple-panel">
          <h3>{rfUi("Редактор доменов выбранного DNS","Domain editor for selected DNS")}</h3><p class="dr-simple-help">{rfUi("По одному на строку или через запятую. Поддомены включаются автоматически.","One per line or comma-separated. Subdomains are included automatically.")}</p><p id="drP10EditingLabel" class="dr-ux-editing-hint">{simple.notice||'Добавление доменов к выбранному DNS. Другие DNS не изменяются.'}</p>
          <label class="dr-simple-field"><span>{rfUi("Какие домены направлять на этот DNS?","Which domains should use this DNS?")}</span><textarea id="drSimpleDomains" rows="12" placeholder="youtube.com&#10;googlevideo.com&#10;ytimg.com" bind:value={simple.domains}></textarea></label>
          <div id="rf-dns-runtime-svelte-p92"><DnsRuntimeP92 /></div>
          <div class="dr-p92-footer">
            <div class="dr-simple-actions"><span id="rf-dns-runtime-svelte-p88-draft" style="display:contents"><DnsRuntimeP88Actions scope="draft" /></span><div id="rf-dns-runtime-svelte-p10" class="dr-svelte-p10-actions" style="display:contents"><DnsRuntimeP10 /></div></div>
          </div>
          <p id="drP10Runtime" role="status" aria-live="polite">{simple.notice||'Проверяем фактическую активацию DNS…'}</p>
          <p class="dr-simple-state" id="drSimpleState" role="status" aria-live="polite">{simple.status||(draft.dirty?'Есть несохранённые изменения. На DNS Keenetic они не влияют.':'Конфигурация сохранена. Маршрутизация клиентского DNS автоматически НЕ включалась.')}</p>
        </section>
        <section class="dr-simple-panel">
          <div class="dr-simple-list-head"><h3>{rfUi("DNS-сервер → привязанный список доменов","DNS server → assigned domain list")}</h3><span id="drSimpleRuleCount">{ruleSummary()}</span></div>
          <div id="rf-dns-runtime-svelte-p88-rules"><DnsRuntimeP88Views scope="rules" /></div>
          <p class="dr-simple-help">{rfUi("Правила сохранены в DNS Router, но это ещё не означает, что клиенты Keenetic используют их. Активное подключение DNS — отдельный этап.","The rules have been saved in DNS Router, but Keenetic clients may not use them yet. Connecting DNS to active clients is a separate step.")}</p>
        </section>
      </div>
    </div>
    <p class="dr-simple-bottom">{rfUi("Нужны DoT/DoH вручную, списки по ссылке, приоритеты или тестовая активация?","Need manual DoT/DoH, URL-based lists, priorities, or test activation?")} <span id="rf-dns-runtime-svelte-p88-advanced"><DnsRuntimeP88Actions scope="advanced" /></span></p>
  </section>

  <div id="drAdvancedWorkspace" hidden={shell.mode!=='advanced'}>
  <section class="metrics" aria-label={rfUi("Сводка DNS Router","DNS Router overview")}>
    <article class="metric"><span>{rfUi("UPSTREAM-ПУЛЫ","UPSTREAM POOLS")}</span><strong id="metricUpstreams">{draft.config.upstreams?.length||0}</strong><small>{rfUi("DNS-серверы и резервные цепочки","DNS servers and fallback chains")}</small></article>
    <article class="metric"><span>{rfUi("ДОМЕННЫЕ ПРАВИЛА","DOMAIN RULES")}</span><strong id="metricRules">{draft.config.rules?.length||0}</strong><small>{rfUi("Точные, суффиксы, поддомены","Exact matches, suffixes, and subdomains")}</small></article>
    <article class="metric"><span>{rfUi("ТЕСТОВЫЙ DNS","TEST DNS")}</span><strong id="metricRuntime">{shell.lite?(shell.liteActive?'АКТИВЕН':'ВЫКЛ.'):(shadowActive()?'АКТИВЕН':'ВЫКЛ.')}</strong><small>{rfUi("Ручной shadow-forwarder","Manual isolated forwarder")}</small></article>
    <article class="metric"><span>{rfUi("КОНФИГУРАЦИЯ","CONFIGURATION")}</span><strong id="metricConfig">{draft.dirty?'ЧЕРНОВИК':draft.loaded?'СОХРАНЕНО':'—'}</strong><small>{rfUi("Черновик и ревизия CAS","Draft and revision check")}</small></article>
  </section>

  <div class="tabs" role="tablist" aria-label={rfUi("Разделы DNS Router","DNS Router sections")}>
    <button type="button" class={shell.tab==='overview'?'tab active':'tab'} data-tab="overview" id="dr-tab-overview" role="tab" tabindex={shell.tab==='overview'?0:-1} aria-controls="dr-panel-overview" aria-selected={shell.tab==='overview'} onclick={()=>activate('overview')} onkeydown={e=>tabKey(e,'overview')}>{rfUi("Обзор","Overview")}</button>
    <button type="button" class={shell.tab==='policies'?'tab active':'tab'} data-tab="policies" id="dr-tab-policies" role="tab" tabindex={shell.tab==='policies'?0:-1} aria-controls="dr-panel-policies" aria-selected={shell.tab==='policies'} onclick={()=>activate('policies')} onkeydown={e=>tabKey(e,'policies')}>{rfUi("Списки и правила","Lists and rules")}</button>
    <button type="button" class={shell.tab==='testing'?'tab active':'tab'} data-tab="testing" id="dr-tab-testing" role="tab" tabindex={shell.tab==='testing'?0:-1} aria-controls="dr-panel-testing" aria-selected={shell.tab==='testing'} onclick={()=>activate('testing')} onkeydown={e=>tabKey(e,'testing')}>{rfUi("Проверка","Check")}</button>
    <button type="button" class={shell.tab==='keenetic'?'tab active':'tab'} data-tab="keenetic" id="dr-tab-keenetic" role="tab" tabindex={shell.tab==='keenetic'?0:-1} aria-controls="dr-panel-keenetic" aria-selected={shell.tab==='keenetic'} onclick={()=>activate('keenetic')} onkeydown={e=>tabKey(e,'keenetic')}>Keenetic</button>
    <button type="button" class={shell.tab==='diagnostics'?'tab active':'tab'} data-tab="diagnostics" id="dr-tab-diagnostics" role="tab" tabindex={shell.tab==='diagnostics'?0:-1} aria-controls="dr-panel-diagnostics" aria-selected={shell.tab==='diagnostics'} onclick={()=>activate('diagnostics')} onkeydown={e=>tabKey(e,'diagnostics')}>{rfUi("Диагностика","Diagnostics")}</button>
  </div>

  <div class={shell.tab==='overview'?'tab-page active':'tab-page'} data-page="overview" aria-hidden={shell.tab!=='overview'} id="dr-panel-overview" role="tabpanel" aria-labelledby="dr-tab-overview" tabindex="0">
    <section class="panel">
      <div class="panel-head">
        <div><span class="section-kicker">{rfUi("УПРАВЛЕНИЕ / ТЕСТОВЫЙ РЕЖИМ","MANAGEMENT / TEST MODE")}</span><h2>Shadow DNS</h2><p>{rfUi("Ручной listener на loopback или явно разрешённом LAN. Никакой подмены системного DNS.","A manual listener on loopback or an explicitly allowed LAN. No system DNS replacement.")}</p></div>
        <span class="state safe">OPT-IN</span>
      </div>
      <div class="panel-content">
        <div id="rf-dns-runtime-svelte-shadow-controls"><DnsRuntimeShadowControls /></div>
        <div class="shadow-summary" role="note">
          <span class="shadow-summary-icon" aria-hidden="true">◎</span>
          <div><strong>{rfUi("Изолированный тестовый DNS","Isolated test DNS")}</strong><p>{rfUi("Не изменяет системный DNS Keenetic. Действия выполняются только по явной команде.","Does not change Keenetic system DNS. Actions run only when explicitly requested.")}</p></div>
        </div>
        <details class="debug-details">
          <summary>{rfUi("Техническое состояние и данные API","Advanced service diagnostics")}</summary>
          <div id="rf-dns-runtime-svelte-shadow-status"><DnsRuntimeShadowStatus /></div>
        </details>
      </div>
    </section>
    <div class="two-column">
      <section class="panel">
        <div class="panel-head"><div><span class="section-kicker">{rfUi("КОНФИГУРАЦИЯ","CONFIGURATION")}</span><h2>{rfUi("Как работают правила","How the rules work")}</h2></div></div>
        <div class="steps">
          <div class="step"><span>01</span><div><strong>{rfUi("Создай upstream-пулы","Create upstream pools")}</strong><p>{rfUi("Основные транспорты — DoT, DoH и DoQ. Открытые DNS доступны в расширенных настройках.","DoT, DoH, and DoQ are the primary transports. Unencrypted DNS is available in advanced settings.")}</p></div></div>
          <div class="step"><span>02</span><div><strong>{rfUi("Задай доменные маршруты","Define domain routes")}</strong><p>{rfUi("Правила для домена, суффикса или поддоменов с приоритетами.","Prioritized rules for domains, suffixes, and subdomains.")}</p></div></div>
          <div class="step"><span>03</span><div><strong>{rfUi("Проверь и сохрани","Check and save")}</strong><p>{rfUi("Сначала тест решения, затем явное применение к shadow.","Test the decision first, then explicitly apply it to the isolated service.")}</p></div></div>
        </div>
        <div class="panel-actions"><button type="button" class="button" data-go="policies" onclick={()=>activate('policies')}>{rfUi("Перейти к политикам","Go to policies")}</button><button type="button" class="button" data-go="testing" onclick={()=>activate('testing')}>{rfUi("Проверить домен","Test a domain")}</button></div>
      </section>
      <section class="panel">
        <div class="panel-head"><div><span class="section-kicker">{rfUi("ГРАНИЦЫ ВОЗДЕЙСТВИЯ","SCOPE OF CHANGES")}</span><h2>{rfUi("Что остаётся неизменным","What stays unchanged")}</h2></div></div>
        <div class="guard-list">
          <div><span class="guard-check">✓</span><span>{rfUi("Порт","Port")} <code>53</code> {rfUi("остаётся у системного DNS Keenetic.","remains with Keenetic system DNS.")}</span></div>
          <div><span class="guard-check">✓</span><span>{rfUi("Настройки","Settings")} <code>ndnproxy</code> {rfUi("и DNS Proxy не перехватываются автоматически.","and DNS Proxy are not intercepted automatically.")}</span></div>
          <div><span class="guard-check">✓</span><span>{rfUi("Импорт, планы и диагностика не меняют DNS-трафик сами по себе.","Import, plans, and diagnostics do not change DNS traffic by themselves.")}</span></div>
          <div><span class="guard-warn">!</span><span>{rfUi("Временная DNS-транзакция требует отдельного разрешения и подтверждения.","A temporary DNS transaction requires separate permission and confirmation.")}</span></div>
        </div>
        <div class="panel-actions"><button type="button" class="button" data-go="diagnostics" onclick={()=>activate('diagnostics')}>{rfUi("Диагностика оборудования","Device diagnostics")}</button></div>
      </section>
    </div>
  </div>

  <div class={shell.tab==='policies'?'tab-page active':'tab-page'} data-page="policies" aria-hidden={shell.tab!=='policies'} id="dr-panel-policies" role="tabpanel" aria-labelledby="dr-tab-policies" tabindex="0">
    <section class="panel dr-p6-panel" id="drGroupPanel">
      <div class="panel-head"><div><span class="section-kicker">DNS POLICY / STRICT</span><h2>{rfUi("Строгие группы DNS","Strict DNS groups")}</h2><p>{rfUi("Домен из группы использует только перечисленные DNS-серверы. При отказе всех — ошибка, без системного DNS fallback.","A domain in this group uses only the specified DNS servers. If all fail, an error is returned without fallback to system DNS.")}</p></div><span class="state safe">FAIL-CLOSED</span></div>
      <div id="rf-dns-runtime-svelte-groups"><DnsRuntimeGroups /></div>
    </section>

    <section class="panel dr-p5-panel" id="drListPanel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ПОЛИТИКИ / СПИСКИ ДОМЕНОВ","POLICIES / DOMAIN LISTS")}</span><h2>{rfUi("Мои списки доменов","My domain lists")}</h2><p>{rfUi("TXT, текст или HTTPS-ссылка. Предпросмотр и применение к черновику без запуска DNS.","TXT file, pasted text, or HTTPS URL. Preview and add to draft without starting DNS.")}</p></div><span class="state safe">{rfUi("БЕЗ ИЗМЕНЕНИЯ DNS","NO DNS CHANGES")}</span></div>
      <div id="rf-dns-runtime-svelte-lists"><DnsRuntimeLists /></div>
    </section>

    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ПОЛИТИКИ / 01","POLICIES / 01")}</span><h2>{rfUi("Upstream-пулы","Upstream pools")}</h2><p>{rfUi("DNS-серверы, транспорт и явно заданные резервные цепочки.","DNS servers, transports, and explicitly configured fallback chains.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-upstream-form"><DnsRuntimePolicyForms scope="upstream" /></div>
      <!-- R07: Native inventory and form are Svelte-owned (DnsRuntimePolicyForms). -->
      <p class="boundary-copy">{rfUi("Keenetic Native: транспорт native, адрес вида","Keenetic Native: native transport, address in the form")} <code>native://System/./9.9.9.9</code> {rfUi("или","or")} <code>native://Policy0/ru/common.dot.dns.yandex.net</code>{rfUi(". Выбирается текущий loopback-порт из ndnproxy, но только с явным флагом",". Uses the current ndnproxy loopback port, but only with an explicit flag")} <code>--native-bridge-experimental</code> {rfUi("при старте DNS Router. Шифрование за пределами локального stubby этим режимом не доказывается, поэтому encrypted-only группы его не принимают.","at DNS Router startup. This mode cannot verify encryption beyond the local stubby instance, so encrypted-only groups do not accept it.")}</p>
      <p class="boundary-copy">{rfUi("Проверка Native Bridge подтверждает только совпадение селектора и runtime-флаг. Для PolicyN может отсутствовать default route — тогда DNS-запрос завершится отказом. Пример: Policy0 без интернета корректно остаётся заблокированной; RouterForge не меняет её маршруты.","The native DNS check only verifies the selected instance and runtime flag. A Policy profile may have no default route, in which case DNS queries fail. For example, Policy0 without internet remains blocked; RouterForge never changes its routes.")}</p>
      <p class="boundary-copy">{rfUi("Новые зашифрованные группы не допускают fallback к UDP/TCP. Для уже сохранённых групп совместимость сохраняется.","New encrypted-only groups cannot fall back to UDP/TCP. Existing saved groups remain compatible.")}</p>
      <div class="table-wrap" id="upstreamList"><DnsRuntimeTables target="upstreamList" /></div>
      <div id="rf-dns-runtime-svelte-upstreams"><DnsRuntimeReadOnly kind="upstreams" /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ПОЛИТИКИ / 02","POLICIES / 02")}</span><h2>{rfUi("Доменные правила","Domain rules")}</h2><p>{rfUi("Правила применяются по приоритету к выбранным upstream-пулам.","Rules are applied by priority to the selected upstream pools.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-rule-form"><DnsRuntimePolicyForms scope="rule" /></div>
      <div class="dr-p5-content"><label><span>{rfUi("Поиск по доменам и правилам","Search domains and rules")}</span><input id="drRuleSearch" type="search" placeholder={rfUi("Найти домен, ID или DNS-пул","Find domain, ID, or DNS pool")} bind:value={draft.ruleSearch}></label></div>
      <div class="table-wrap" id="ruleList"><DnsRuntimeTables target="ruleList" /></div>
    </section>
  </div>

  <div class={shell.tab==='testing'?'tab-page active':'tab-page'} data-page="testing" aria-hidden={shell.tab!=='testing'} id="dr-panel-testing" role="tabpanel" aria-labelledby="dr-tab-testing" tabindex="0">
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("СИМУЛЯТОР","SIMULATOR")}</span><h2>{rfUi("Проверить маршрутизацию домена","Test domain routing")}</h2><p>{rfUi("Проверка использует сохранённую конфигурацию и не выполняет сетевой DNS-запрос.","The check uses the saved configuration and does not send a DNS query over the network.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-evaluate"><DnsRuntimeTools kind="evaluate" /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ИМПОРТ","IMPORT")}</span><h2>{rfUi("Простой импорт (совместимость)","Basic import (compatibility)")}</h2><p>{rfUi("Для файлов, URL и именованных списков используй вкладку «Списки и правила».","Use the Lists and Rules tab for files, URLs, and named lists.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-import"><DnsRuntimeConfigActions kind="import" /></div>
    </section>
  </div>

  <div class={shell.tab==='keenetic'?'tab-page active':'tab-page'} data-page="keenetic" aria-hidden={shell.tab!=='keenetic'} id="dr-panel-keenetic" role="tabpanel" aria-labelledby="dr-tab-keenetic" tabindex="0">
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ИНТЕГРАЦИЯ / ЧЕРНОВИК","INTEGRATION / DRAFT")}</span><h2>{rfUi("Настройки listener","Listener settings")}</h2><p>{rfUi("Сохранение адреса и ACL не запускает DNS и не меняет настройки Keenetic.","Saving the address and ACL does not start DNS or change Keenetic settings.")}</p></div><span class="state safe">{rfUi("БЕЗ АКТИВАЦИИ","NOT ACTIVE")}</span></div>
      <div id="rf-dns-runtime-svelte-listener-form"><DnsRuntimePolicyForms scope="listener" /></div>
      <div id="rf-dns-runtime-svelte-listener"><DnsRuntimeReadOnly kind="listener" /></div>
      <p class="boundary-copy">{rfUi("Показывает, какие правила доступны через штатные зоны. Чтение конфигурации не меняет DNS, не запускает сервис и не включает перехват.","Shows which rules are currently reachable through native zones. Reading the configuration does not apply DNS, start a listener, or enable interception.")}</p>
      <div id="rf-dns-runtime-svelte-ingress"><DnsRuntimeReadOnly kind="ingress" /></div>
    </section>
    <section class="panel risk-panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ВРЕМЕННОЕ ИСПЫТАНИЕ DNS","Temporary DNS test · experimental")}</span><h2>{rfUi("Временная транзакция DNS-зон","Temporary DNS zone transaction")}</h2><p>{rfUi("Требует отдельного серверного флага; изменяет тестовые зоны на ограниченное время и выполняет откат.","Requires a separate server-side flag; changes test zones for a limited time and then rolls back.")}</p></div><span class="state caution">{rfUi("ПОВЫШЕННЫЙ РИСК","HIGHER RISK")}</span></div>
      <div class="boundary-copy">{rfUi("Перед испытанием необходимы сохранённая конфигурация, LAN-listener, отсутствие конфликтов и явное подтверждение. Не использовать на production.","Testing requires a saved configuration, LAN listener, no conflicts, and explicit confirmation. Do not use in production.")}</div>
      <div id="rf-dns-runtime-svelte-managed"><DnsRuntimeManagedTrial /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ВЛАДЕНИЕ DNS-ЗОНАМИ · ТОЛЬКО ЧТЕНИЕ","Zone ownership inventory · read only")}</span><h2>{rfUi("Реестр владения DNS-зонами","DNS zone ownership registry")}</h2><p>{rfUi("Сверка собственных и чужих записей. Не удаляет и не принимает чужие настройки.","Compares own and third-party entries without deleting or adopting them.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-ownership"><DnsRuntimeReadOnly kind="ownership" /></div>
    </section>
  </div>

  <div class={shell.tab==='diagnostics'?'tab-page active':'tab-page'} data-page="diagnostics" aria-hidden={shell.tab!=='diagnostics'} id="dr-panel-diagnostics" role="tabpanel" aria-labelledby="dr-tab-diagnostics" tabindex="0">
    <div id="rf-dns-runtime-svelte-diagnostics"><DnsRuntimeDiagnostics /></div>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("ДИАГНОСТИКА / READ-ONLY","DIAGNOSTICS / READ ONLY")}</span><h2>{rfUi("Среда и ресурсы","Environment and resources")}</h2><p>{rfUi("Пассивные показатели и планы интеграции без изменения настроек Keenetic.","Passive metrics and integration plans without changing Keenetic settings.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-discovery"><DnsRuntimeReadOnly kind="discovery" /></div>
    </section>
    <section class="panel">
      <div class="panel-head"><div><span class="section-kicker">{rfUi("АППАРАТНАЯ СОВМЕСТИМОСТЬ","Hardware compatibility")}</span><h2>{rfUi("Анализ hardware evidence","Hardware diagnostics")}</h2><p>{rfUi("Вставь отчёт read-only диагностики DSTest. Данные не сохраняются на маршрутизаторе.","Paste a read-only diagnostic report. Data is not stored on the router.")}</p></div></div>
      <div id="rf-dns-runtime-svelte-hardware"><DnsRuntimeReadOnly kind="hardware" /></div>
    </section>
    <div class="boundary-note">{rfUi("Доступные экспериментальные операции запускаются только через отдельные capability-флаги, включая","Experimental operations are only available with explicit capability flags, including")} <code>--shadow-lan-experimental</code> {rfUi("для LAN-listener. DNS Router не занимает порт 53 и не подменяет ndnproxy. DoT/DoH проверяют TLS-сертификаты и не используют неявный fallback.","for the LAN listener. DNS Router does not occupy port 53 or replace ndnproxy. DoT/DoH verify TLS certificates and do not use implicit fallback.")}</div>
  </div>

  <footer class="save-bar">
    <div class="save-meta"><strong id="metricFooter">{draft.dirty?'Есть несохранённые изменения':'Настройки DNS Router'}</strong><span id="revision">{(draft.dirty?'● Не сохранено · ':'')+draft.revision.substring(0,23)}</span></div>
    <div class="save-actions"><div id="rf-dns-runtime-svelte-export"><DnsRuntimeTools kind="export" /></div><div id="rf-dns-runtime-svelte-save"><DnsRuntimeConfigActions kind="save" /></div></div>
  </footer>
  </div><!-- drAdvancedWorkspace: original controls preserved -->
</main>
