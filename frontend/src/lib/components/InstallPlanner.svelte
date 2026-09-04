<script>
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  export let item;
  export let onclose=()=>{};

  $: locale = $settings.locale || 'ru';
  $: install=item?.install||{};
  $: hints=item?.compatibility?.hints||[];
  $: packages=install.packages||item?.detection?.packages||[];
  $: notes=install.notes||[];
</script>

{#if item}
  <div class="modal-overlay" role="presentation" onclick={(event)=>event.currentTarget===event.target&&onclose()}>
    <section class="planner-modal" role="dialog" aria-modal="true" aria-label={t(locale, 'marketplace.planner.aria')}>
      <header class="planner-head">
        <div class="planner-title"><span class="status-dot info"></span><span class="muted mono">{t(locale, 'marketplace.planner.title')} ::</span><strong>{item.name}</strong>{#if item.version}<span class="version-badge">v{item.version}</span>{/if}</div>
        <div class="planner-mode mono"><span>{t(locale, 'marketplace.planner.mode')}</span><strong>{t(locale, 'marketplace.planner.modeValue')}</strong><button class="icon-button" type="button" aria-label={t(locale, 'common.close')} onclick={onclose}>×</button></div>
      </header>

      <div class="planner-body">
        <div class="planner-checks">
          <div>
            <div class="section-label">{t(locale, 'marketplace.planner.checklist')}</div>
            <div class="check-card">
              <div class="check-head"><strong>{t(locale, 'marketplace.planner.discovery')}</strong><span class="state-chip {item.installed?'good':'neutral'}">{item.installed?t(locale, 'common.detected').toUpperCase():t(locale, 'common.notInstalled').toUpperCase()}</span></div>
              <div class="check-row"><span>{t(locale, 'marketplace.planner.state')}</span><code>{item.state||'available'}</code></div>
              <div class="check-row"><span>{t(locale, 'marketplace.planner.version')}</span><code>{item.version?`v${item.version}`:'—'}</code></div>
              {#if item.service}<div class="check-row"><span>{t(locale, 'marketplace.planner.service')}</span><code>{item.service}</code></div>{/if}
              {#if item.web_port}<div class="check-row"><span>{t(locale, 'marketplace.planner.webUi')}</span><code>:{item.web_port}</code></div>{/if}
            </div>
            <div class="check-card">
              <div class="check-head"><strong>{t(locale, 'marketplace.planner.compatibility')}</strong><span class="state-chip info">{item.compatibility?.status||'REQUIREMENTS'}</span></div>
              {#if hints.length}{#each hints as hint}<div class="check-row"><span>{t(locale, 'marketplace.planner.requirement')}</span><code>{hint}</code></div>{/each}{:else}<div class="check-row"><span>{t(locale, 'marketplace.planner.requirements')}</span><code>{t(locale, 'marketplace.planner.notDeclared')}</code></div>{/if}
            </div>
            <div class="check-card">
              <div class="check-head"><strong>{t(locale, 'marketplace.planner.installPlan')}</strong><span class="state-chip info">{t(locale, 'marketplace.planner.staged')}</span></div>
              {#if install.method}<div class="check-row"><span>{t(locale, 'marketplace.planner.method')}</span><code>{install.method}</code></div>{/if}
              {#if packages.length}<div class="check-row"><span>{t(locale, 'marketplace.planner.packages')}</span><code>{packages.join(', ')}</code></div>{/if}
              {#if install.repository_url}<div class="check-row"><span>{t(locale, 'marketplace.planner.feed')}</span><code>{install.repository_url}</code></div>{/if}
              {#if install.installer_url}<div class="check-row"><span>{t(locale, 'marketplace.planner.installer')}</span><code>{install.installer_url}</code></div>{/if}
              {#if !install.method&&!packages.length&&!install.repository_url&&!install.installer_url}<div class="check-row"><span>{t(locale, 'marketplace.planner.plan')}</span><code>{t(locale, 'marketplace.planner.notAvailable')}</code></div>{/if}
              {#if notes.length}<div class="plan-notes">{#each notes as note}<div>• {note}</div>{/each}</div>{/if}
            </div>
          </div>
          <div class="safety-box mono"><strong>{t(locale, 'marketplace.planner.safety')}</strong><span>{t(locale, 'marketplace.planner.safetyText')}</span></div>
        </div>

        <div class="planner-console">
          <div class="console-head mono"><span><i></i>catalog-plan-stream</span><span>{t(locale, 'marketplace.planner.details')}</span></div>
          <div class="console-body mono">
            <div><span class="log-dim">[CATALOG]</span> source={item.source||'routerforge'} state={item.state||'available'}</div>
            {#if item.version}<div class="log-ok">[DETECT] version {item.version}</div>{/if}
            {#if packages.length}<div><span class="log-dim">[PACKAGE]</span> {packages.join(', ')} · {item.installed?t(locale, 'marketplace.planner.detected'):t(locale, 'marketplace.planner.notInstalled')}</div>{/if}
            {#if item.service}<div class={item.service_running ? 'item-ok' : ''}>[SERVICE] {item.service} · {item.service_running?t(locale, 'marketplace.planner.running'):t(locale, 'marketplace.planner.notRunning')}</div>{/if}
            {#if item.web_port}<div><span class="log-dim">[WEB]</span> port {item.web_port} · {item.installed?t(locale, 'marketplace.planner.integrationEndpoint'):t(locale, 'marketplace.planner.expectedDefault')}</div>{/if}
            {#if install.repository_url}<div><span class="log-info">[PLAN]</span> feed {install.repository_url}</div>{/if}
            {#if install.installer_url}<div><span class="log-info">[PLAN]</span> installer {install.installer_url}</div>{/if}
            {#if install.method}<div><span class="log-info">[PLAN]</span> method={install.method}</div>{/if}
            <div class="log-warn">[SAFE] {t(locale, 'marketplace.planner.safeLog')}</div>
          </div>
          <div class="console-foot"><div class="mono"><span>{t(locale, 'marketplace.planner.planStatus')}</span><strong>{t(locale, 'marketplace.planner.previewReady')}</strong></div><div class="progress"><span style="width:100%"></span></div></div>
        </div>
      </div>

      <footer class="planner-foot">
        <div class="mono">{t(locale, 'marketplace.planner.changesApplied')} <strong>0</strong></div>
        <div class="planner-actions">
          {#if item.project_url}<a class="button" target="_blank" rel="noopener noreferrer" href={item.project_url}>{t(locale, 'marketplace.planner.openProject')}</a>{/if}
          <button class="button" type="button" onclick={onclose}>{t(locale, 'common.close')}</button>
          <span class="muted mono">{t(locale, 'marketplace.planner.packageActions')}</span>
        </div>
      </footer>
    </section>
  </div>
{/if}
