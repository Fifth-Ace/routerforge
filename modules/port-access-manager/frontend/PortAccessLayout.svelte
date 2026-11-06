<script>
  import { onMount } from 'svelte';

  // The four main pages, header state and navigation belong to Svelte.
  // Original manager panels are MOVED once, not duplicated or re-created:
  // the underlying safety-checked operations keep their existing listeners.
  const pages = [
    { id: 'protection', title: 'Обзор', description: 'Выберите метод защиты и проверьте параметры' },
    { id: 'access', title: 'Доступ', description: 'Авторизованные IP и сохранённые черновики' },
    { id: 'engines', title: 'Движки', description: 'Установленные службы и конфигурации Entware' },
    { id: 'diagnostics', title: 'Диагностика', description: 'Проверки, предпросмотр и журнал операций' }
  ];
  let active = $state('protection');
  let health = $state('ПРОВЕРКА…');
  let healthClass = $derived(health === 'РАБОТАЕТ' ? 'good' : health === 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ' ? 'warn' : '');
  let protectionEl, accessEl, enginesEl, diagnosticsEl, summaryEl;
  let legacy = null;
  let legacyProtection = null;
  let legacyTools = null;
  let observer = null;

  function move(node, to) {
    if (node && to && node !== to && !node.contains(to)) to.appendChild(node);
  }

  function syncHealth() {
    const b = summaryEl?.querySelectorAll('.rf-pa-metric b')[1];
    const message = b?.textContent || '';
    health = /включена|подтверждено/i.test(message) ? 'РАБОТАЕТ'
      : /ожидает|watchdog/i.test(message) ? 'ОЖИДАЕТ ПОДТВЕРЖДЕНИЯ'
      : /выключена|не установлен|остановлен/i.test(message) ? 'НЕ АКТИВНА' : 'ПРОВЕРКА…';
  }

  function show(page) {
    if (!pages.some(p => p.id === page)) return;
    if (page === 'engines') {
      const buttons = document.querySelectorAll('.rf-pa-choice');
      // An Entware page needs a corresponding Entware engine selection.
      if (buttons[0]?.getAttribute('aria-pressed') === 'true') buttons[1]?.click();
    }
    // The pre-existing engine controller remains authoritative for its
    // internal working/advanced visibility until that subsystem migrates.
    if (page === 'diagnostics') legacyTools?.click();
    else legacyProtection?.click();
    if (page === 'diagnostics') {
      const advanced = diagnosticsEl?.querySelector('.rf-pa-advanced');
      if (advanced) advanced.hidden = false;
    } else if (page === 'protection') {
      const working = protectionEl?.querySelector('.rf-pa-working');
      const chooser = protectionEl?.querySelector('.rf-pa-selector');
      if (working) working.hidden = false;
      if (chooser) chooser.hidden = false;
    }
    active = page;
  }

  onMount(() => {
    const main = document.querySelector('main.pa');
    const old = main?.querySelector('.rf-pa-workspace');
    const mount = document.getElementById('pa-svelte-layout');
    if (!main || !old || !mount || !document.getElementById('odin-manager')) {
      throw new Error('PORT_ACCESS_LAYOUT_MOUNT_PREREQUISITE_MISSING');
    }
    // The old workspace was prepared by ui-rebuild and P39/P53. Relocate
    // this live Svelte root before it. No changes to routing or firewall.
    old.parentElement.insertBefore(mount, old);
    main.classList.add('p55-access');
    const header = main.querySelector('header');
    header?.classList.add('p55-hero');
    const intro = header?.querySelector('p');
    if (intro) intro.textContent = 'Защита проброшенных сервисов: odin, классический Port Knocking и SPA.';

    move(old.querySelector('.rf-pa-summary'), summaryEl);
    move(old.querySelector('.rf-pa-selector'), protectionEl);
    move(old.querySelector('.rf-pa-working'), protectionEl);
    move(document.getElementById('odin-live-access'), accessEl);
    move(document.getElementById('staging-panel'), accessEl);
    move(document.getElementById('recent-live-panel'), accessEl);
    move(old.querySelector('.rf-pa-advanced'), diagnosticsEl);
    const log = main.querySelector('.rf-pa-operation-log');
    move(log?.closest('.rf-pa-log-details') || log || document.getElementById('pa-svelte-log'), diagnosticsEl);
    move(document.getElementById('existing-engine-control'), enginesEl);
    move(old.querySelector('.rf-pa-compact-advanced'), enginesEl);
    const editor = document.getElementById('engine-config-panel');
    if (editor && !enginesEl.contains(editor)) move(editor, enginesEl);
    const oldTabs = old.querySelector('.rf-pa-view-tabs');
    legacyProtection = oldTabs?.querySelector('.rf-pa-view-tab:first-child') || null;
    legacyTools = oldTabs?.querySelector('.rf-pa-view-tab:nth-child(2)') || null;

    // Remove obsolete P53 presentation-only wrappers, not operator controls.
    old.querySelector('.rf-pa-p53-hero')?.remove();
    old.querySelector('.rf-pa-p53-workflow')?.remove();
    old.querySelector('.rf-pa-p53-context')?.remove();

    for (const panel of mount.querySelectorAll('.panel,.rf-pa-selector,.rf-pa-engine-scope,.rf-pa-advanced')) panel.classList.add('p55-panel');
    for (const grid of mount.querySelectorAll('.form-grid')) grid.classList.add('p55-grid');
    const deploy = document.getElementById('rfpa-deploy-panel');
    deploy?.classList.add('p55-deploy');
    deploy?.querySelector('.rf-pa-deploy-actions')?.classList.add('p55-actions');
    document.getElementById('rfpa-replace-existing')?.closest('label')?.classList.add('p55-confirm-row');
    document.getElementById('rfpa-deploy-status')?.classList.add('p55-compact-status');
    const chooserTitle = protectionEl.querySelector('.rf-pa-selector h2');
    if (chooserTitle) chooserTitle.textContent = 'Способ защиты';

    // Existing mode handlers are retained. The Svelte router owns the
    // visible top-level page; the old mode controller owns only its forms.
    old.hidden = true;
    if (summaryEl) {
      observer = new MutationObserver(syncHealth);
      observer.observe(summaryEl, { subtree: true, childList: true, characterData: true });
    }
    syncHealth();
    show('protection');
    const restore = () => show(active);
    window.addEventListener('pageshow', restore);
    return () => {
      observer?.disconnect();
      window.removeEventListener('pageshow', restore);
    };
  });
</script>

<div class="p54-root" aria-label="Контроль доступа">
  <div bind:this={summaryEl} class="p55-summary-slot"></div>
  <div class="p55-head-actions" role="status" aria-live="polite">
    <span class={'p55-state ' + healthClass}>{health}</span>
  </div>
  <nav class="p54-navigation" aria-label="Разделы контроля доступа">
    {#each pages as page (page.id)}
      <button type="button" class="p54-tab p55-tab" class:active={active === page.id}
        aria-current={active === page.id ? 'page' : undefined} onclick={() => show(page.id)}>
        {page.title}
      </button>
    {/each}
  </nav>
  <section id="p54-protection" class="p54-page" aria-label="Обзор" hidden={active !== 'protection'} bind:this={protectionEl}>
    <div class="p54-heading p55-section-heading"><h2>Настроить защиту</h2><p>Выберите метод, проверьте настройки, затем подтвердите применение.</p></div>
  </section>
  <section id="p54-access" class="p54-page" aria-label="Доступ" hidden={active !== 'access'} bind:this={accessEl}>
    <div class="p54-heading p55-section-heading"><h2>Доступы и правила</h2><p>Активные разрешения и сохранённые черновики — разные сущности.</p></div>
  </section>
  <section id="p54-engines" class="p54-page" aria-label="Движки" hidden={active !== 'engines'} bind:this={enginesEl}>
    <div class="p54-heading p55-section-heading"><h2>Движки Entware</h2><p>Установка, запуск, остановка и редактирование конфигураций.</p></div>
  </section>
  <section id="p54-diagnostics" class="p54-page" aria-label="Диагностика" hidden={active !== 'diagnostics'} bind:this={diagnosticsEl}>
    <div class="p54-heading p55-section-heading"><h2>Диагностика и инструменты</h2><p>Проверки, журнал и экспериментальные сценарии.</p></div>
  </section>
</div>
