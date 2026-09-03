<script>
  export let item;
  export let onclose=()=>{};

  $: install=item?.install||{};
  $: hints=item?.compatibility?.hints||[];
  $: packages=install.packages||item?.detection?.packages||[];
  $: notes=install.notes||[];
</script>

{#if item}
  <div class="modal-overlay" role="presentation" onclick={(event)=>event.currentTarget===event.target&&onclose()}>
    <section class="planner-modal" role="dialog" aria-modal="true" aria-label="Install Planner">
      <header class="planner-head">
        <div class="planner-title"><span class="status-dot info"></span><span class="muted mono">Install Planner ::</span><strong>{item.name}</strong>{#if item.version}<span class="version-badge">v{item.version}</span>{/if}</div>
        <div class="planner-mode mono"><span>MODE:</span><strong>PLAN / DETAILS</strong><button class="icon-button" type="button" onclick={onclose}>×</button></div>
      </header>

      <div class="planner-body">
        <div class="planner-checks">
          <div>
            <div class="section-label">Execution Stage Checklist</div>
            <div class="check-card"><div class="check-head"><strong>1. Discovery</strong><span class="state-chip {item.installed?'good':'neutral'}">{item.installed?'DETECTED':'NOT INSTALLED'}</span></div><div class="check-row"><span>State</span><code>{item.state||'available'}</code></div><div class="check-row"><span>Version</span><code>{item.version?`v${item.version}`:'—'}</code></div>{#if item.service}<div class="check-row"><span>Service</span><code>{item.service}</code></div>{/if}{#if item.web_port}<div class="check-row"><span>Web UI</span><code>:{item.web_port}</code></div>{/if}</div>
            <div class="check-card"><div class="check-head"><strong>2. Compatibility</strong><span class="state-chip info">{item.compatibility?.status||'REQUIREMENTS'}</span></div>{#if hints.length}{#each hints as hint}<div class="check-row"><span>Requirement</span><code>{hint}</code></div>{/each}{:else}<div class="check-row"><span>Requirements</span><code>not declared</code></div>{/if}</div>
            <div class="check-card"><div class="check-head"><strong>3. Install Plan</strong><span class="state-chip info">STAGED</span></div>{#if install.method}<div class="check-row"><span>Method</span><code>{install.method}</code></div>{/if}{#if packages.length}<div class="check-row"><span>Packages</span><code>{packages.join(', ')}</code></div>{/if}{#if install.repository_url}<div class="check-row"><span>Feed</span><code>{install.repository_url}</code></div>{/if}{#if install.installer_url}<div class="check-row"><span>Installer</span><code>{install.installer_url}</code></div>{/if}{#if !install.method&&!packages.length&&!install.repository_url&&!install.installer_url}<div class="check-row"><span>Plan</span><code>not available</code></div>{/if}{#if notes.length}<div class="plan-notes">{#each notes as note}<div>• {note}</div>{/each}</div>{/if}</div>
          </div>
          <div class="safety-box mono"><strong>Safety boundary</strong><span>Это окно ничего не выполняет. Install / update / remove запускаются только отдельными кнопками Marketplace и только для lifecycle, разрешённого RouterForge Registry.</span></div>
        </div>

        <div class="planner-console">
          <div class="console-head mono"><span><i></i>catalog-plan-stream</span><span>DETAILS</span></div>
          <div class="console-body mono"><div><span class="log-dim">[CATALOG]</span> source={item.source||'dns-monitor'} state={item.state||'available'}</div>{#if item.version}<div class="log-ok">[DETECT] version {item.version}</div>{/if}{#if packages.length}<div><span class="log-dim">[PACKAGE]</span> {packages.join(', ')} · {item.installed?'detected':'not installed'}</div>{/if}{#if item.service}<div class={item.service_running ? 'item-ok' : ''}>[SERVICE] {item.service} · {item.service_running?'running':'not running'}</div>{/if}{#if item.web_port}<div><span class="log-dim">[WEB]</span> port {item.web_port} · {item.installed?'integration endpoint':'expected default'}</div>{/if}{#if install.repository_url}<div><span class="log-info">[PLAN]</span> feed {install.repository_url}</div>{/if}{#if install.installer_url}<div><span class="log-info">[PLAN]</span> installer {install.installer_url}</div>{/if}{#if install.method}<div><span class="log-info">[PLAN]</span> method={install.method}</div>{/if}<div class="log-warn">[SAFE] package actions require explicit Marketplace action + approved registry lifecycle</div></div>
          <div class="console-foot"><div class="mono"><span>Plan Status</span><strong>Preview ready</strong></div><div class="progress"><span style="width:100%"></span></div></div>
        </div>
      </div>

      <footer class="planner-foot"><div class="mono">Changes applied: <strong>0</strong></div><div class="planner-actions">{#if item.project_url}<a class="button" target="_blank" rel="noopener noreferrer" href={item.project_url}>Открыть проект</a>{/if}<button class="button" type="button" onclick={onclose}>Закрыть</button><span class="muted mono">Package actions: Marketplace card</span></div></footer>
    </section>
  </div>
{/if}
