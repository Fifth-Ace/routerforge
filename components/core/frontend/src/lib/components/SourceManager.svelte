<script>
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n/index.js';
  import {
    getAppSources, previewAppSource, addAppSource, refreshAppSource,
    setAppSourceEnabled, setAppSourceReleaseChannel, removeAppSource, setAppSourceSecurity, getAppLegal
  } from '$lib/api.js';

  export let locale = 'ru';
  export let onclose = () => {};
  export let onchanged = () => {};

  let state = { sources:[], allow_unverified:false, agreement_version:'', agreement_accepted_at:'', allow_local_sources:false, local_agreement_version:'', local_agreement_accepted_at:'' };
  let loading = true;
  let busy = '';
  let error = '';
  let sourceURL = '';
  let sourceKind = 'auto';
  let preview = null;
  let previewRequest = null;
  let riskOpen = false;
  let riskAccepted = false;
  let localRiskOpen = false;
  let localRiskAccepted = false;
  let legalOpen = false;
  let legalText = '';
  let legalLoading = false;


  onMount(() => { void load(); });

  async function load() {
    loading = true; error = '';
    try { state = await getAppSources(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { loading = false; }
  }

  function invalidateSourcePreview() {
    preview = null;
    previewRequest = null;
  }

  async function previewSource() {
    const url = sourceURL.trim();
    const kind = sourceKind;
    if (!url || busy) return;
    busy = 'preview'; error = ''; invalidateSourcePreview();
    try {
      preview = await previewAppSource({ url, kind });
      previewRequest = { url, kind };
    }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function addSource() {
    if (!preview || !previewRequest || busy) return;
    if (sourceURL.trim() !== previewRequest.url || sourceKind !== previewRequest.kind) {
      error = t(locale, 'sourceManager.previewChanged');
      invalidateSourcePreview();
      return;
    }
    busy = 'add'; error = '';
    try {
      await addAppSource({
        url:previewRequest.url,
        kind:previewRequest.kind,
        confirm:'ADD_SOURCE',
        fingerprint:preview.fingerprint
      });
      sourceURL = ''; invalidateSourcePreview();
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function refreshSource(source) {
    if (busy || source.read_only) return;
    busy = `refresh:${source.id}`; error = '';
    try { await refreshAppSource(source.id); await load(); await onchanged(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; await load(); }
    finally { busy = ''; }
  }

  async function toggleSource(source) {
    if (busy || source.read_only) return;
    busy = `toggle:${source.id}`; error = '';
    try { await setAppSourceEnabled(source.id, !source.enabled); await load(); await onchanged(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function changeSourceReleaseChannel(source, event) {
    if (busy || source.read_only || !source.manifestless) return;
    const select = event.currentTarget;
    const current = String(source.release_channel || 'auto').toLowerCase();
    const next = String(select.value || 'auto').toLowerCase();
    if (next === current) return;
    if (next === 'beta') {
      const question = t(locale, 'sourceManager.confirmBeta');
      if (!window.confirm(question)) {
        select.value = current;
        return;
      }
    }
    busy = `channel:${source.id}`; error = '';
    try {
      await setAppSourceReleaseChannel(source.id, next);
      await load(); await onchanged();
    } catch (e) {
      select.value = current;
      error = e?.payload?.error || e?.message || 'error';
    } finally { busy = ''; }
  }


  async function deleteSource(source) {
    if (busy || source.read_only) return;
    const question = t(locale, 'sourceManager.confirmDelete', {name:source.name});
    if (!window.confirm(question)) return;
    busy = `delete:${source.id}`; error = '';
    try { await removeAppSource(source.id); await load(); await onchanged(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function disableUnsafe() {
    if (busy) return;
    busy = 'security'; error = '';
    try {
      await setAppSourceSecurity({ scope:'unverified', allow_unverified:false, accepted:false, agreement_version:state.agreement_version });
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function enableUnsafe() {
    if (!riskAccepted || busy) return;
    busy = 'security'; error = '';
    try {
      await setAppSourceSecurity({ scope:'unverified', allow_unverified:true, accepted:true, agreement_version:state.agreement_version });
      riskOpen = false; riskAccepted = false;
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function disableLocalSources() {
    if (busy) return;
    busy = 'local-security'; error = '';
    try {
      await setAppSourceSecurity({
        scope:'local',
        allow_local_sources:false,
        local_accepted:false,
        local_agreement_version:state.local_agreement_version
      });
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function enableLocalSources() {
    if (!localRiskAccepted || busy) return;
    busy = 'local-security'; error = '';
    try {
      await setAppSourceSecurity({
        scope:'local',
        allow_local_sources:true,
        local_accepted:true,
        local_agreement_version:state.local_agreement_version
      });
      localRiskOpen = false; localRiskAccepted = false;
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function showLegal() {
    legalOpen = true;
    if (legalText || legalLoading) return;
    legalLoading = true;
    try { const result = await getAppLegal(locale); legalText = result?.text || ''; }
    catch (e) { legalText = e?.payload?.error || e?.message || 'error'; }
    finally { legalLoading = false; }
  }

  function trustLabel(source) {
    const trust = String(source?.trust || 'unsigned').toUpperCase();
    if (trust === 'OFFICIAL') return t(locale, 'sourceManager.trustOfficial');
    if (trust === 'VERIFIED') return t(locale, 'sourceManager.trustVerified');
    return t(locale, 'sourceManager.trustUnsigned');
  }

  function sourceState(source) {
    if (!source.enabled) return { cls:'neutral', text:t(locale, 'sourceManager.stateDisabled') };
    if (source.online) return { cls:'good', text:t(locale, 'sourceManager.stateOnline') };
    if (source.cached) return { cls:'warn', text:t(locale, 'sourceManager.stateCache') };
    return { cls:'error', text:t(locale, 'sourceManager.stateError') };
  }
</script>

<div class="source-backdrop" role="presentation" onclick={onclose}>
  <section class="source-modal" role="dialog" aria-modal="true" aria-label={t(locale, 'sourceManager.sourcesTitle')} onclick={(event) => event.stopPropagation()}>
    <div class="source-head">
      <div>
        <h2>{t(locale, 'sourceManager.sourcesTitle')}</h2>
        <p>{t(locale, 'sourceManager.sourcesSubtitle')}</p>
      </div>
      <button class="icon-button" aria-label={t(locale, 'sourceManager.close')} onclick={onclose}>×</button>
    </div>

    {#if error}<div class="source-error">{error}</div>{/if}

    <section class="risk-panel" class:enabled={state.allow_unverified}>
      <div>
        <strong>{t(locale, 'sourceManager.unverifiedTitle')}</strong>
        <p>{state.allow_unverified
          ? (t(locale, 'sourceManager.unverifiedEnabled'))
          : (t(locale, 'sourceManager.unverifiedBlocked'))}</p>
      </div>
      {#if state.allow_unverified}
        <button class="button danger-subtle" disabled={Boolean(busy)} onclick={disableUnsafe}>{t(locale, 'sourceManager.forbid')}</button>
      {:else}
        <button class="button danger" disabled={Boolean(busy)} onclick={() => riskOpen = true}>{t(locale, 'sourceManager.allowInstallation')}</button>
      {/if}
    </section>

    <section class="risk-panel local" class:enabled={state.allow_local_sources}>
      <div>
        <strong>{t(locale, 'sourceManager.localTitle')}</strong>
        <p>{state.allow_local_sources
          ? (t(locale, 'sourceManager.localEnabled'))
          : (t(locale, 'sourceManager.localBlocked'))}</p>
      </div>
      {#if state.allow_local_sources}
        <button class="button danger-subtle" disabled={Boolean(busy)} onclick={disableLocalSources}>{t(locale, 'sourceManager.forbid')}</button>
      {:else}
        <button class="button" disabled={Boolean(busy)} onclick={() => localRiskOpen = true}>{t(locale, 'sourceManager.allowLocal')}</button>
      {/if}
    </section>

    <div class="source-list">
      {#if loading}
        <div class="source-empty">{t(locale, 'sourceManager.loading')}</div>
      {:else}
        {#each state.sources || [] as source (source.id)}
          {@const st = sourceState(source)}
          <article class="source-card">
            <div class="source-card-main">
              <div class="source-title-row">
                <strong>{source.name}</strong>
                <span class="state-chip {source.trust === 'official' ? 'official' : 'neutral'}">{trustLabel(source)}</span>
                {#if source.manifestless}<span class="state-chip warn">{t(locale, 'sourceManager.unmanagedBadge')}</span>{/if}
                <span class="state-chip {st.cls}">{st.text}</span>
              </div>
              <span class="mono source-url">{source.url}</span>
              <div class="source-meta">
                <span>{source.kind === 'app' ? (t(locale, 'sourceManager.singleApp')) : (t(locale, 'sourceManager.repository'))}</span>
                {#if source.local}<span>{t(locale, 'sourceManager.localNetwork')}</span>{/if}
                <span>{t(locale, 'sourceManager.appCount')}: {source.entry_count || 0}</span>
                {#if source.revision}<span>rev {source.revision}</span>{/if}
                {#if source.last_sync}<span>{t(locale, 'sourceManager.lastSync')}: {source.last_sync}</span>{/if}
              </div>
              {#if source.error}<div class="source-inline-error">{source.error}</div>{/if}
            </div>
            <div class="source-actions">
              {#if !source.read_only}
                {#if source.manifestless}
                  <select
                    class="source-channel-select"
                    value={source.release_channel || 'auto'}
                    disabled={Boolean(busy)}
                    aria-label={t(locale, 'sourceManager.githubChannel')}
                    onchange={(event) => changeSourceReleaseChannel(source, event)}
                  >
                    <option value="auto">AUTO</option>
                    <option value="release">RELEASE</option>
                    <option value="beta">BETA</option>
                  </select>
                {/if}

                <button class="button compact" disabled={Boolean(busy)} onclick={() => refreshSource(source)}>{t(locale, 'sourceManager.refresh')}</button>
                <button class="button compact" disabled={Boolean(busy)} onclick={() => toggleSource(source)}>{source.enabled ? (t(locale, 'sourceManager.disable')) : (t(locale, 'sourceManager.enable'))}</button>
                <button class="button danger-subtle compact" disabled={Boolean(busy)} onclick={() => deleteSource(source)}>{t(locale, 'sourceManager.remove')}</button>
              {/if}
            </div>
          </article>
        {/each}
      {/if}
    </div>

    <section class="source-add">
      <div class="source-add-head">
        <strong>{t(locale, 'sourceManager.addTitle')}</strong>
        <p>{t(locale, 'sourceManager.addSubtitle')}</p>
      </div>
      <div class="source-form">
        <select bind:value={sourceKind} onchange={invalidateSourcePreview}>
          <option value="auto">{t(locale, 'sourceManager.autoDetect')}</option>
          <option value="repository">{t(locale, 'sourceManager.repositoryType')}</option>
          <option value="app">{t(locale, 'sourceManager.singleAppType')}</option>
        </select>
        <input bind:value={sourceURL} oninput={invalidateSourcePreview} placeholder="https://github.com/owner/repo" />
        <button class="button" disabled={!sourceURL.trim() || Boolean(busy)} onclick={previewSource}>{busy === 'preview' ? (t(locale, 'sourceManager.checking')) : (t(locale, 'sourceManager.preview'))}</button>
      </div>

      {#if preview}
        <div class="source-preview">
          <div class="source-title-row">
            <strong>{preview.name}</strong>
            <span class="state-chip warn">{t(locale, 'sourceManager.trustUnsigned')}</span>
            {#if preview.manifestless}<span class="state-chip warn">{t(locale, 'sourceManager.unmanagedBadge')}</span>{/if}
            <span class="state-chip neutral">{preview.kind === 'app' ? (t(locale, 'sourceManager.previewApp')) : (t(locale, 'sourceManager.previewRepository'))}</span>
          </div>
          <div class="source-meta">
            <span>{t(locale, 'sourceManager.appCount')}: {preview.entry_count}</span>
            {#if preview.registry_id}<span>ID: {preview.registry_id}</span>{/if}
            {#if preview.revision}<span>rev {preview.revision}</span>{/if}
          </div>
          <span class="mono source-url">{preview.resolved_url}</span>
          <div class="source-preview-apps">
            {#each (preview.entries || []).slice(0, 8) as entry (entry.id)}
              <span>
                {entry.name}{#if entry.category} · {entry.category}{/if}{#if entry.package} · <code>{entry.package}</code>{/if}
                {#if (entry.targets || []).length} · <code>{entry.targets.join(', ')}</code>{/if}
                {#if entry.target_status === 'incompatible'} · <strong>{t(locale, 'sourceManager.incompatible')}</strong>{/if}
              </span>
            {/each}
            {#if Number(preview.entry_count || 0) > 8}<span>+{Number(preview.entry_count) - 8}</span>{/if}
          </div>
          {#if preview.manifestless}
            <div class="source-preview-warning">{t(locale, 'sourceManager.unmanagedWarning')}</div>
          {:else}
            <div class="source-preview-warning">{t(locale, 'sourceManager.previewArchitectureWarning', {architecture:preview.entries?.[0]?.detected_target || t(locale, 'sourceManager.previewUnknownArchitecture'), local:preview.local ? t(locale, 'sourceManager.previewLocalHint') : ''})}</div>
          {/if}
          <div class="source-preview-warning">{t(locale, 'sourceManager.previewShaWarning')}</div>
          <button class="button primary" disabled={Boolean(busy) || !preview?.fingerprint || !previewRequest} onclick={addSource}>{busy === 'add' ? (t(locale, 'sourceManager.adding')) : (t(locale, 'sourceManager.addPreviewed'))}</button>
        </div>
      {/if}
    </section>

    <footer class="source-footer">
      <a class="button compact" href="https://github.com/Fifth-Ace/routerforge" target="_blank" rel="noopener noreferrer">{t(locale, 'sourceManager.sourceCode')}</a>
      <button class="legal-badge" onclick={showLegal}>{t(locale, 'sourceManager.legalLink')}</button>
    </footer>
  </section>
</div>

{#if riskOpen}
  <div class="risk-backdrop" role="presentation" onclick={() => riskOpen = false}>
    <section class="risk-modal" role="alertdialog" aria-modal="true" onclick={(event) => event.stopPropagation()}>
      <span class="risk-kicker">{t(locale, 'sourceManager.warning')}</span>
      <h3>{t(locale, 'sourceManager.unsafeCodeTitle')}</h3>
      <p>{t(locale, 'sourceManager.unsafeCodeBody')}</p>
      <p>{t(locale, 'sourceManager.unsafeCodeSafety')}</p>
      <label class="risk-check">
        <input type="checkbox" bind:checked={riskAccepted} />
        <span>{t(locale, 'sourceManager.acceptUnsafeCode')}</span>
      </label>
      <div class="risk-actions">
        <button class="button" onclick={() => riskOpen = false}>{t(locale, 'sourceManager.cancel')}</button>
        <button class="button danger" disabled={!riskAccepted || Boolean(busy)} onclick={enableUnsafe}>{t(locale, 'sourceManager.confirmAllowUnsafe')}</button>
      </div>
    </section>
  </div>
{/if}

{#if localRiskOpen}
  <div class="risk-backdrop" role="presentation" onclick={() => localRiskOpen = false}>
    <section class="risk-modal local-risk" role="alertdialog" aria-modal="true" onclick={(event) => event.stopPropagation()}>
      <span class="risk-kicker">{t(locale, 'sourceManager.localNetworkWarning')}</span>
      <h3>{t(locale, 'sourceManager.allowLocalTitle')}</h3>
      <p>{t(locale, 'sourceManager.allowLocalDescription')}</p>
      <p>{t(locale, 'sourceManager.localSafety')}</p>
      <label class="risk-check">
        <input type="checkbox" bind:checked={localRiskAccepted} />
        <span>{t(locale, 'sourceManager.acceptLocal')}</span>
      </label>
      <div class="risk-actions">
        <button class="button" onclick={() => localRiskOpen = false}>{t(locale, 'sourceManager.cancel')}</button>
        <button class="button danger" disabled={!localRiskAccepted || Boolean(busy)} onclick={enableLocalSources}>{t(locale, 'sourceManager.confirmAllowLocal')}</button>
      </div>
    </section>
  </div>
{/if}

{#if legalOpen}
  <div class="risk-backdrop" role="presentation" onclick={() => legalOpen = false}>
    <section class="legal-modal" role="dialog" aria-modal="true" onclick={(event) => event.stopPropagation()}>
      <div class="source-head">
        <div><h3>{t(locale, 'sourceManager.legalTitle')}</h3></div>
        <button class="icon-button" onclick={() => legalOpen = false}>×</button>
      </div>
      <pre>{legalLoading ? (t(locale, 'sourceManager.loading')) : legalText}</pre>
    </section>
  </div>
{/if}

<style>
  .source-backdrop,.risk-backdrop{position:fixed;inset:0;z-index:150;background:rgba(0,0,0,.84);display:grid;place-items:center;padding:1.2rem;backdrop-filter:none}
  .source-modal{width:min(1040px,96vw);max-height:92vh;overflow:auto;background:var(--rf-surface,var(--surface));color:var(--rf-text,var(--text));border:1px solid var(--rf-border,var(--border));border-radius:1rem;padding:1rem;box-shadow:0 24px 80px rgba(0,0,0,.62);backdrop-filter:none}
  .source-head{display:flex;justify-content:space-between;gap:1rem;align-items:flex-start;border-bottom:1px solid var(--rf-border,var(--border));padding-bottom:.8rem;margin-bottom:.9rem}
  .source-head h2,.source-head h3{margin:.15rem 0 .25rem}.source-head p{margin:0;color:var(--rf-muted,var(--muted))}
  .eyebrow{font-size:.7rem;color:var(--rf-good,#52d273)}
  .risk-panel{display:flex;justify-content:space-between;align-items:center;gap:1rem;padding:.8rem;border:1px solid rgba(247,185,85,.34);background:rgba(247,185,85,.06);border-radius:.75rem;margin-bottom:.9rem}
  .risk-panel.enabled{border-color:rgba(255,78,78,.45);background:rgba(255,78,78,.07)} .risk-panel.local{border-color:rgba(95,160,255,.34);background:rgba(95,160,255,.06)} .risk-panel.local.enabled{border-color:rgba(95,160,255,.5);background:rgba(95,160,255,.1)}
  .risk-panel p{margin:.2rem 0 0;color:var(--rf-muted,var(--muted))}
  .source-list{display:grid;gap:.55rem}
  .source-card{display:flex;justify-content:space-between;gap:1rem;align-items:center;border:1px solid var(--rf-border,var(--border));border-radius:.75rem;padding:.75rem;background:rgba(255,255,255,.018)}
  .source-card-main{min-width:0;display:grid;gap:.32rem}.source-title-row{display:flex;align-items:center;gap:.45rem;flex-wrap:wrap}
  .source-url{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--rf-muted,var(--muted));font-size:.75rem}
  .source-meta{display:flex;gap:.65rem;flex-wrap:wrap;color:var(--rf-muted,var(--muted));font-size:.75rem}
  .source-actions{display:flex;gap:.35rem;flex-wrap:wrap;justify-content:flex-end}
  .source-inline-error,.source-error{color:#ff8b8b;font-size:.78rem}.source-error{border:1px solid rgba(255,78,78,.35);padding:.65rem;border-radius:.6rem;margin-bottom:.8rem}
  .source-add{margin-top:1rem;border-top:1px solid var(--rf-border,var(--border));padding-top:1rem}.source-add-head p{margin:.2rem 0 .7rem;color:var(--rf-muted,var(--muted))}
  .source-form{display:grid;grid-template-columns:minmax(12rem,.55fr) minmax(16rem,1.5fr) auto;gap:.5rem}.source-form input,.source-form select{min-width:0}
  .source-preview{margin-top:.7rem;border:1px solid rgba(82,210,115,.25);border-radius:.75rem;padding:.8rem;display:grid;gap:.55rem}
  .source-preview-apps{display:flex;gap:.35rem;flex-wrap:wrap}.source-preview-apps span{font-size:.74rem;padding:.25rem .42rem;border:1px solid var(--rf-border,var(--border));border-radius:.45rem}
  .source-preview-warning{font-size:.78rem;color:var(--rf-muted,var(--muted))}
  .source-footer{display:flex;flex-direction:column;align-items:flex-start;gap:.32rem;margin-top:1rem;padding-top:.9rem;border-top:1px solid var(--rf-border,var(--border))}
  .legal-badge{border:0;background:transparent;color:var(--rf-muted,var(--muted));font:inherit;font-size:.7rem;padding:0;cursor:pointer;text-decoration:underline;text-decoration-style:dotted}
  .risk-modal,.legal-modal{width:min(700px,94vw);max-height:88vh;overflow:auto;background:var(--rf-surface,var(--surface));color:var(--rf-text,var(--text));border:1px solid rgba(255,78,78,.45);border-radius:1rem;padding:1rem;box-shadow:0 24px 80px rgba(0,0,0,.68);backdrop-filter:none}
  .risk-kicker{font-size:.7rem;font-weight:700;color:#ff6868;letter-spacing:.12em}.risk-modal h3{margin:.25rem 0 .7rem}.risk-modal p{color:var(--rf-muted,var(--muted));line-height:1.48}
  .risk-check{display:flex;gap:.55rem;align-items:flex-start;margin:1rem 0;padding:.7rem;border:1px solid rgba(255,78,78,.3);border-radius:.65rem}.risk-check input{margin-top:.2rem}
  .risk-actions{display:flex;justify-content:flex-end;gap:.5rem}.legal-modal{border-color:var(--rf-border,var(--border))}.legal-modal pre{white-space:pre-wrap;font:inherit;font-size:.82rem;line-height:1.5;color:var(--rf-text,var(--text))}
  .button.danger{border-color:rgba(255,78,78,.55);background:rgba(255,78,78,.15);color:#ff8585}.source-empty{padding:1rem;color:var(--rf-muted,var(--muted))}
  @media(max-width:760px){.source-card{align-items:flex-start;flex-direction:column}.source-actions{justify-content:flex-start}.source-form{grid-template-columns:1fr}.source-backdrop{padding:.4rem}.source-modal{width:100%;max-height:96vh}}
</style>
